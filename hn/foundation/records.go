package foundation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"
)

const SourceBaseline = "852fd2128770136d037f92dfef2655dd3d6ac1d5"

func timestamp() string      { return time.Now().UTC().Format(time.RFC3339Nano) }
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Reject secret-bearing field names and recognizable credential/URL literals.
// Callers must still supply non-secret text; this is not a general DLP engine.
func nonSecret(value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var v any
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	var check func(any) error
	check = func(x any) error {
		switch x := x.(type) {
		case map[string]any:
			for k, v := range x {
				n := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(k))
				switch n {
				case "apikey", "token", "accesstoken", "authorization", "password", "secret", "clientsecret", "cookie", "signature":
					return errors.New("secret field rejected")
				}
				if e := check(v); e != nil {
					return e
				}
			}
		case []any:
			for _, v := range x {
				if e := check(v); e != nil {
					return e
				}
			}
		case string:
			for _, s := range []string{"-----BEGIN", "sk-proj-", "ghp_", "github_pat_"} {
				if strings.Contains(x, s) {
					return errors.New("credential literal rejected")
				}
			}
			if u, e := url.Parse(x); e == nil && u.Scheme != "" {
				if u.User != nil {
					return errors.New("URL credentials rejected")
				}
				for k := range u.Query() {
					n := strings.ToLower(k)
					if strings.Contains(n, "token") || strings.Contains(n, "signature") || strings.Contains(n, "credential") || n == "key" || n == "sig" || n == "apikey" {
						return errors.New("signed/credential URL rejected")
					}
				}
			}
		}
		return nil
	}
	return check(v)
}

func (w *Workspace) CreateShot(label, nodeID string) (Shot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := Shot{Identity: w.identity(), Label: label, SourceNodeID: nodeID}
	if err := nonSecret(s); err != nil {
		return Shot{}, err
	}
	return s, insert(w.db, "shots", s.Identity, s, "")
}
func (w *Workspace) RenameShot(id, label string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var s Shot
	if err := read(w.db, "shots", id, &s); err != nil {
		return err
	}
	s.Label = label
	s.UpdatedAt = timestamp()
	if err := nonSecret(s); err != nil {
		return err
	}
	return put(w.db, "shots", id, s)
}

func (w *Workspace) validateGeneration(g *Generation) error {
	if err := w.check(g.Identity); err != nil {
		return err
	}
	if g.CredentialRef != "" && !validName(g.CredentialRef) {
		return errors.New("credentialRef must be an opaque ID")
	}
	if g.SourceBaseline == "" {
		return errors.New("source baseline required")
	}
	if len(g.Parameters) == 0 {
		g.Parameters = json.RawMessage(`{}`)
	}
	var params map[string]any
	if err := json.Unmarshal(g.Parameters, &params); err != nil || params == nil {
		return errors.New("parameters must be a JSON object")
	}
	g.Parameters, _ = json.Marshal(params)
	if err := nonSecret(g); err != nil {
		return err
	}
	if g.ShotID != "" {
		var s Shot
		if err := read(w.db, "shots", g.ShotID, &s); err != nil {
			return err
		}
	}
	for _, b := range g.ReferenceBindings {
		var r ReferenceVersion
		if err := read(w.db, "reference_versions", b.ReferenceVersionID, &r); err != nil {
			return err
		}
		if b.SHA256 != r.SHA256 || b.Role == "" {
			return errors.New("reference binding hash/role mismatch")
		}
		if err := w.verifyReference(r); err != nil {
			return err
		}
	}
	if (g.RequestSnapshotRef == "") != (g.RequestSnapshotHash == "") {
		return errors.New("request snapshot path/hash must be paired")
	}
	if g.RequestSnapshotRef != "" {
		if !strings.HasPrefix(g.RequestSnapshotRef, "metadata/") {
			return errors.New("request snapshot must be project metadata")
		}
		path, err := w.Resolve(g.RequestSnapshotRef)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if digest(b) != g.RequestSnapshotHash {
			return errors.New("request snapshot hash mismatch")
		}
		var v any
		if err = json.Unmarshal(b, &v); err != nil {
			return err
		}
		if err = nonSecret(v); err != nil {
			return err
		}
	}
	return nil
}
func generationHash(g Generation) string {
	g.UpdatedAt = ""
	g.Status = ""
	g.SubmissionState = ""
	g.Frozen = false
	g.FrozenHash = ""
	b, _ := json.Marshal(g)
	return digest(b)
}
func (w *Workspace) saveGeneration(g Generation, create bool) error {
	if err := w.validateGeneration(&g); err != nil {
		return err
	}
	if !create {
		var old Generation
		if err := read(w.db, "generations", g.ID, &old); err != nil {
			return err
		}
		if old.CreatedAt != g.CreatedAt {
			return errors.New("identity facts immutable")
		}
		if old.Frozen && generationHash(g) != old.FrozenHash {
			return errors.New("frozen request immutable")
		}
		g.Frozen = old.Frozen
		g.FrozenHash = old.FrozenHash
		g.Status = old.Status
		g.SubmissionState = old.SubmissionState
		g.UpdatedAt = timestamp()
	}
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if create {
		err = insert(tx, "generations", g.Identity, g, "shot_id", nullable(g.ShotID))
	} else {
		err = put(tx, "generations", g.ID, g)
		if err == nil {
			_, err = tx.Exec("UPDATE generations SET shot_id=? WHERE id=?", nullable(g.ShotID), g.ID)
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM generation_references WHERE generation_id=?", g.ID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, r := range g.ReferenceBindings {
		if seen[r.ReferenceVersionID] {
			continue
		}
		seen[r.ReferenceVersionID] = true
		if _, err = tx.Exec("INSERT INTO generation_references VALUES(?,?)", g.ID, r.ReferenceVersionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (w *Workspace) CreateGeneration(g Generation) (Generation, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.checkNew(g.Identity); err != nil {
		return Generation{}, err
	}
	g.Identity = w.identity()
	g.Frozen = false
	g.FrozenHash = ""
	g.Status = "DRAFT"
	g.SubmissionState = "NOT_SUBMITTED"
	if g.SourceBaseline == "" {
		g.SourceBaseline = SourceBaseline
	}
	if len(g.Parameters) == 0 {
		g.Parameters = json.RawMessage(`{}`)
	}
	err := w.saveGeneration(g, true)
	if err == nil {
		err = read(w.db, "generations", g.ID, &g)
	}
	return g, err
}
func (w *Workspace) UpdateGeneration(g Generation) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.saveGeneration(g, false)
}
func (w *Workspace) FreezeGeneration(id string) (Generation, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var g Generation
	if err := read(w.db, "generations", id, &g); err != nil {
		return g, err
	}
	if err := w.validateGeneration(&g); err != nil {
		return g, err
	}
	if g.Frozen {
		if generationHash(g) != g.FrozenHash {
			return g, errors.New("frozen record corruption")
		}
		return g, nil
	}
	g.FrozenHash = generationHash(g)
	g.Frozen = true
	g.Status = "PREPARED"
	g.SubmissionState = "PREPARED"
	g.UpdatedAt = timestamp()
	return g, put(w.db, "generations", id, g)
}

// MarkSubmissionUnknown records uncertainty only. It does not send or resend anything.
func (w *Workspace) MarkSubmissionUnknown(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var g Generation
	if err := read(w.db, "generations", id, &g); err != nil {
		return err
	}
	if !g.Frozen {
		return errors.New("generation not frozen")
	}
	g.SubmissionState = "SUBMISSION_UNKNOWN"
	g.Status = "SUBMISSION_UNKNOWN"
	g.UpdatedAt = timestamp()
	return put(w.db, "generations", id, g)
}

func (w *Workspace) CreateTaskBinding(b TaskBinding) (TaskBinding, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.checkNew(b.Identity); err != nil {
		return TaskBinding{}, err
	}
	b.Identity = w.identity()
	var g Generation
	if err := read(w.db, "generations", b.GenerationID, &g); err != nil {
		return b, err
	}
	if !g.Frozen {
		return b, errors.New("generation not frozen")
	}
	if b.ConnectionID != g.ConnectionID || b.Protocol != g.Protocol || b.ProviderIdentity != g.ProviderIdentity {
		return b, errors.New("connection identity mismatch")
	}
	b.BindingState = "UNBOUND"
	if b.ProviderTaskID != "" || b.UpstreamLocalTaskID != "" {
		b.BindingState = "BOUND"
	}
	if err := nonSecret(b); err != nil {
		return b, err
	}
	return b, insert(w.db, "task_bindings", b.Identity, b, "generation_id", b.GenerationID)
}
func (w *Workspace) UpdateTaskBinding(b TaskBinding) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.check(b.Identity); err != nil {
		return err
	}
	var old TaskBinding
	if err := read(w.db, "task_bindings", b.ID, &old); err != nil {
		return err
	}
	if b.GenerationID != old.GenerationID || b.CreatedAt != old.CreatedAt || b.ConnectionID != old.ConnectionID || b.Protocol != old.Protocol || b.ProviderIdentity != old.ProviderIdentity || old.ProviderTaskID != "" && b.ProviderTaskID != old.ProviderTaskID || old.UpstreamLocalTaskID != "" && b.UpstreamLocalTaskID != old.UpstreamLocalTaskID {
		return errors.New("task ownership/bound facts immutable")
	}
	b.BindingState = "UNBOUND"
	if b.ProviderTaskID != "" || b.UpstreamLocalTaskID != "" {
		b.BindingState = "BOUND"
	}
	b.UpdatedAt = timestamp()
	if err := nonSecret(b); err != nil {
		return err
	}
	return put(w.db, "task_bindings", b.ID, b)
}
func (w *Workspace) CreateResult(r Result) (Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.checkNew(r.Identity); err != nil {
		return Result{}, err
	}
	r.Identity = w.identity()
	r.ReceivedAt = r.CreatedAt
	r.Status = "RECEIVED"
	r.ArchiveJobID = ""
	r.ArchivedRelativePath = ""
	r.SHA256 = ""
	r.ByteLength = 0
	var g Generation
	if err := read(w.db, "generations", r.GenerationID, &g); err != nil {
		return r, err
	}
	if !g.Frozen {
		return r, errors.New("generation not frozen")
	}
	if r.SourceURLRef != "" && !validName(r.SourceURLRef) {
		return r, errors.New("sourceUrlRef must be opaque, not a URL")
	}
	if r.ResultKind != "video" && r.ResultKind != "image" && r.ResultKind != "audio" {
		return r, errors.New("unsupported media kind")
	}
	if r.DurationSeconds != nil && (*r.DurationSeconds <= 0) {
		return r, errors.New("invalid known duration")
	}
	if err := nonSecret(r); err != nil {
		return r, err
	}
	return r, insert(w.db, "results", r.Identity, r, "generation_id,task_binding_id", r.GenerationID, nullable(r.TaskBindingID))
}
func (w *Workspace) CreateCandidate(shotID, resultID, label string) (Candidate, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var r Result
	var g Generation
	var s Shot
	if err := read(w.db, "results", resultID, &r); err != nil {
		return Candidate{}, err
	}
	if err := read(w.db, "generations", r.GenerationID, &g); err != nil {
		return Candidate{}, err
	}
	if err := read(w.db, "shots", shotID, &s); err != nil {
		return Candidate{}, err
	}
	if g.ShotID != shotID {
		return Candidate{}, errors.New("result belongs to another shot")
	}
	c := Candidate{Identity: w.identity(), ShotID: shotID, GenerationID: g.ID, ResultID: r.ID, Label: label, AvailabilityStatus: r.Status}
	if err := nonSecret(c); err != nil {
		return c, err
	}
	return c, insert(w.db, "candidates", c.Identity, c, "shot_id,generation_id,result_id", shotID, g.ID, r.ID)
}
func (w *Workspace) RenameCandidate(id, label string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var c Candidate
	if err := read(w.db, "candidates", id, &c); err != nil {
		return err
	}
	c.Label = label
	c.UpdatedAt = timestamp()
	if err := nonSecret(c); err != nil {
		return err
	}
	return put(w.db, "candidates", id, c)
}
func (w *Workspace) SelectCandidate(shotID, candidateID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var s Shot
	if err := read(w.db, "shots", shotID, &s); err != nil {
		return err
	}
	if candidateID != "" {
		var c Candidate
		if err := read(w.db, "candidates", candidateID, &c); err != nil {
			return err
		}
		if c.ShotID != shotID {
			return errors.New("candidate belongs to another shot")
		}
	}
	s.SelectedCandidateID = candidateID
	s.UpdatedAt = timestamp()
	return put(w.db, "shots", shotID, s)
}
func (w *Workspace) AddSequenceItem(sequenceID, candidateID string) (SequenceItem, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !validName(sequenceID) {
		return SequenceItem{}, errors.New("unsafe sequence ID")
	}
	var c Candidate
	var s Shot
	if err := read(w.db, "candidates", candidateID, &c); err != nil {
		return SequenceItem{}, err
	}
	if err := read(w.db, "shots", c.ShotID, &s); err != nil {
		return SequenceItem{}, err
	}
	if s.SelectedCandidateID != c.ID {
		return SequenceItem{}, errors.New("candidate not explicitly selected")
	}
	var index int
	if err := w.db.QueryRow("SELECT coalesce(max(order_index)+1,0) FROM sequence_items WHERE sequence_id=?", sequenceID).Scan(&index); err != nil {
		return SequenceItem{}, err
	}
	i := SequenceItem{Identity: w.identity(), SequenceID: sequenceID, OrderIndex: index, ShotID: c.ShotID, CandidateID: c.ID, ResultID: c.ResultID}
	return i, insert(w.db, "sequence_items", i.Identity, i, "sequence_id,order_index,shot_id,candidate_id,result_id", sequenceID, index, c.ShotID, c.ID, c.ResultID)
}
func (w *Workspace) Reorder(sequenceID string, ids []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var count int
	if err := w.db.QueryRow("SELECT count(*) FROM sequence_items WHERE sequence_id=?", sequenceID).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return errors.New("reorder must name every sequence item")
	}
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seen := map[string]bool{}
	for index, id := range ids {
		var i SequenceItem
		if err = read(tx, "sequence_items", id, &i); err != nil {
			return err
		}
		if seen[id] || i.SequenceID != sequenceID {
			return errors.New("duplicate/foreign sequence item")
		}
		seen[id] = true
		i.OrderIndex = index
		i.UpdatedAt = timestamp()
		if err = put(tx, "sequence_items", id, i); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE sequence_items SET order_index=? WHERE id=?", index, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
