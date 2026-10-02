package foundation

import (
	"errors"
	"regexp"
)

var ErrSubmissionState = errors.New("submission ownership/state rejected; same generation cannot be resent")

var submissionCredentialPrefix = regexp.MustCompile(`(?i)^(sk-|gh[pousr]_|github_pat_|AKIA|ASIA)`)

func (w *Workspace) frozenSubmission(id string) (Generation, error) {
	var g Generation
	if err := read(w.db, "generations", id, &g); err != nil {
		return g, err
	}
	if !g.Frozen || generationHash(g) != g.FrozenHash {
		return g, ErrSubmissionState
	}
	return g, w.validateGeneration(&g)
}

// BeginSubmission commits ownership before any transport may be invoked. The
// returned request is the exact frozen snapshot owned by this transaction.
func (w *Workspace) BeginSubmission(id string) (Generation, TaskBinding, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	checked, err := w.frozenSubmission(id)
	if err != nil {
		return Generation{}, TaskBinding{}, err
	}
	tx, err := w.db.Begin()
	if err != nil {
		return Generation{}, TaskBinding{}, err
	}
	defer tx.Rollback()
	var g Generation
	if err = read(tx, "generations", id, &g); err != nil {
		return g, TaskBinding{}, err
	}
	if !g.Frozen || g.SubmissionState != "PREPARED" || g.Status != "PREPARED" || generationHash(g) != checked.FrozenHash || g.FrozenHash != checked.FrozenHash {
		return g, TaskBinding{}, ErrSubmissionState
	}
	var owners int
	if err = tx.QueryRow("SELECT count(*) FROM task_bindings WHERE generation_id=?", id).Scan(&owners); err != nil {
		return g, TaskBinding{}, err
	}
	if owners != 0 {
		return g, TaskBinding{}, ErrSubmissionState
	}
	b := TaskBinding{Identity: w.identity(), GenerationID: g.ID, ConnectionID: g.ConnectionID, Protocol: g.Protocol, ProviderIdentity: g.ProviderIdentity, BindingState: "UNBOUND"}
	if err = insert(tx, "task_bindings", b.Identity, b, "generation_id", id); err != nil {
		return g, TaskBinding{}, err
	}
	g.Status, g.SubmissionState, g.UpdatedAt = "SUBMITTING", "SUBMITTING", timestamp()
	if err = put(tx, "generations", id, g); err != nil {
		return g, TaskBinding{}, err
	}
	if err = tx.Commit(); err != nil {
		return Generation{}, TaskBinding{}, err
	}
	return g, b, nil
}

// RecordSubmissionAccepted binds only the durable owner's opaque task IDs.
// Identical persisted acceptance is idempotent; replacement or late acceptance
// of a reconciled UNKNOWN attempt is rejected.
func (w *Workspace) RecordSubmissionAccepted(generationID, bindingID, localTaskID, providerTaskID string) (TaskBinding, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if localTaskID == "" && providerTaskID == "" {
		return TaskBinding{}, ErrSubmissionState
	}
	for _, id := range []string{localTaskID, providerTaskID} {
		if id != "" && (!validName(id) || nonSecret(id) != nil || submissionCredentialPrefix.MatchString(id)) {
			return TaskBinding{}, ErrSubmissionState
		}
	}
	checked, err := w.frozenSubmission(generationID)
	if err != nil {
		return TaskBinding{}, err
	}
	tx, err := w.db.Begin()
	if err != nil {
		return TaskBinding{}, err
	}
	defer tx.Rollback()
	var g Generation
	var b TaskBinding
	if err = read(tx, "generations", generationID, &g); err != nil {
		return b, err
	}
	if err = read(tx, "task_bindings", bindingID, &b); err != nil {
		return b, err
	}
	var owners int
	if err = tx.QueryRow("SELECT count(*) FROM task_bindings WHERE generation_id=?", generationID).Scan(&owners); err != nil {
		return b, err
	}
	if w.check(b.Identity) != nil || owners != 1 || b.GenerationID != g.ID || b.ConnectionID != g.ConnectionID || b.Protocol != g.Protocol || b.ProviderIdentity != g.ProviderIdentity || !g.Frozen || g.FrozenHash != checked.FrozenHash || generationHash(g) != checked.FrozenHash {
		return b, ErrSubmissionState
	}
	if g.SubmissionState == "SUBMITTED" && g.Status == "SUBMITTED" && b.BindingState == "BOUND" && b.UpstreamLocalTaskID == localTaskID && b.ProviderTaskID == providerTaskID {
		return b, nil
	}
	if g.SubmissionState != "SUBMITTING" || g.Status != "SUBMITTING" || b.BindingState != "UNBOUND" || b.UpstreamLocalTaskID != "" || b.ProviderTaskID != "" {
		return b, ErrSubmissionState
	}
	b.UpstreamLocalTaskID, b.ProviderTaskID, b.BindingState, b.UpdatedAt = localTaskID, providerTaskID, "BOUND", timestamp()
	g.Status, g.SubmissionState, g.UpdatedAt = "SUBMITTED", "SUBMITTED", b.UpdatedAt
	if err = put(tx, "task_bindings", b.ID, b); err != nil {
		return b, err
	}
	if err = put(tx, "generations", g.ID, g); err != nil {
		return b, err
	}
	return b, tx.Commit()
}

// A reopened in-flight attempt may have reached the external system. Retain its
// binding and sacrifice the attempt rather than permit a duplicate invocation.
func (w *Workspace) reconcileSubmissions() error {
	// Guard the current durable state in the write itself: an acceptance committed
	// by another handle since Open's integrity checks must never be downgraded.
	_, err := w.db.Exec(`UPDATE generations SET data=json_set(data,
		'$.submissionState','SUBMISSION_UNKNOWN','$.status','SUBMISSION_UNKNOWN','$.updatedAt',?)
		WHERE project_id=? AND json_extract(data,'$.submissionState')='SUBMITTING'`, timestamp(), w.ProjectID)
	return err
}
