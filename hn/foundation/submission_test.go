package foundation

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func submissionFixture(t *testing.T, w *Workspace) Generation {
	t.Helper()
	s := makeShot(t, w, "fake submit shot")
	g, e := w.CreateGeneration(Generation{ShotID: s.ID, NodeID: "synthetic-node", PromptSnapshot: "synthetic frozen request", Protocol: "fake-protocol", ProviderIdentity: "fake-provider", Model: "fake-model", ConnectionID: "fake-connection", CredentialRef: "opaque-unresolved", Parameters: json.RawMessage(`{"duration":2}`), SourceBaseline: "ff32dc249811130a3db69be456e295be100b6e9f"})
	okay(t, e)
	g, e = w.FreezeGeneration(g.ID)
	return must(t, g, e)
}

func unchangedSubmissionRequest(t *testing.T, before, after Generation) {
	t.Helper()
	before.Status, after.Status = "", ""
	before.SubmissionState, after.SubmissionState = "", ""
	before.UpdatedAt, after.UpdatedAt = "", ""
	if !reflect.DeepEqual(before, after) {
		t.Fatal("request-defining fields/frozen hash changed")
	}
}

func TestSubmissionBeginAtomicAndPreconditions(t *testing.T) {
	w := newWorkspace(t)
	draft, e := w.CreateGeneration(Generation{PromptSnapshot: "draft"})
	okay(t, e)
	_, _, e = w.BeginSubmission(draft.ID)
	reject(t, e)
	if count(t, w, "task_bindings") != 0 {
		t.Fatal("draft created owner")
	}
	g := submissionFixture(t, w)
	// Force a failure between the binding insert and generation update.
	_, e = w.db.Exec("CREATE TRIGGER fail_submission BEFORE UPDATE ON generations BEGIN SELECT RAISE(ABORT,'synthetic transaction failure'); END")
	okay(t, e)
	_, _, e = w.BeginSubmission(g.ID)
	reject(t, e)
	if count(t, w, "task_bindings") != 0 || get[Generation](t, w, "generations", g.ID).SubmissionState != "PREPARED" {
		t.Fatal("partial Begin transaction")
	}
	_, e = w.db.Exec("DROP TRIGGER fail_submission")
	okay(t, e)
	request, b, e := w.BeginSubmission(g.ID)
	okay(t, e)
	unchangedSubmissionRequest(t, g, request)
	if request.Status != "SUBMITTING" || request.SubmissionState != "SUBMITTING" || b.BindingState != "UNBOUND" || b.GenerationID != g.ID || b.ConnectionID != g.ConnectionID || b.Protocol != g.Protocol || b.ProviderIdentity != g.ProviderIdentity || count(t, w, "task_bindings") != 1 {
		t.Fatal(request, b)
	}
	_, _, e = w.BeginSubmission(g.ID)
	reject(t, e)
	if count(t, w, "task_bindings") != 1 {
		t.Fatal("duplicate owner")
	}
	g2 := submissionFixture(t, w)
	_, e = w.CreateTaskBinding(TaskBinding{GenerationID: g2.ID, ConnectionID: g2.ConnectionID, Protocol: g2.Protocol, ProviderIdentity: g2.ProviderIdentity})
	okay(t, e)
	_, _, e = w.BeginSubmission(g2.ID)
	reject(t, e)
	if get[Generation](t, w, "generations", g2.ID).SubmissionState != "PREPARED" {
		t.Fatal("existing owner mutated generation")
	}
	t.Log("ATOMIC_ROLLBACK=PASS PRECONDITIONS=PASS ONE_UNBOUND_OWNER=PASS")
}

func TestSubmissionBeginRejectsCorruption(t *testing.T) {
	for _, field := range []string{"hash", "project"} {
		t.Run(field, func(t *testing.T) {
			w := newWorkspace(t)
			g := submissionFixture(t, w)
			if field == "hash" {
				g.PromptSnapshot = "corrupt"
			} else {
				g.ProjectID = "other-project"
			}
			okay(t, put(w.db, "generations", g.ID, g))
			_, _, e := w.BeginSubmission(g.ID)
			reject(t, e)
			if count(t, w, "task_bindings") != 0 {
				t.Fatal("corrupt generation obtained owner")
			}
		})
	}
}

func TestSubmissionConcurrentBegin(t *testing.T) {
	w := newWorkspace(t)
	g := submissionFixture(t, w)
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, _, e := w.BeginSubmission(g.ID); e == nil {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if wins.Load() != 1 || count(t, w, "task_bindings") != 1 {
		t.Fatal("multiple durable owners", wins.Load())
	}
	t.Log("CONCURRENT_BEGIN_CALLERS=16 OWNERS=1 TASKBINDINGS=1")
}

func TestSubmissionConcurrentHandles(t *testing.T) {
	w := newWorkspace(t)
	g := submissionFixture(t, w)
	other, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer other.Close()
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, handle := range []*Workspace{w, other} {
		wg.Add(1)
		go func(handle *Workspace) {
			defer wg.Done()
			<-start
			if _, _, e := handle.BeginSubmission(g.ID); e == nil {
				wins.Add(1)
			}
		}(handle)
	}
	close(start)
	wg.Wait()
	if wins.Load() != 1 || count(t, w, "task_bindings") != 1 {
		t.Fatal("cross-handle durable ownership", wins.Load())
	}
	t.Log("SEPARATE_WORKSPACE_HANDLES=2 DURABLE_OWNERS=1 TASKBINDINGS=1")
}

func TestSubmissionAcceptedOwnershipAndAtomicity(t *testing.T) {
	w := newWorkspace(t)
	g := submissionFixture(t, w)
	_, b, e := w.BeginSubmission(g.ID)
	okay(t, e)
	g2 := submissionFixture(t, w)
	_, b2, e := w.BeginSubmission(g2.ID)
	okay(t, e)
	for _, ids := range [][4]string{{g2.ID, b.ID, "local", "provider"}, {g.ID, b2.ID, "local", "provider"}, {g.ID, b.ID, "", ""}, {g.ID, b.ID, "https://example.invalid/task", ""}, {g.ID, b.ID, "", "unsafe task"}, {g.ID, b.ID, "sk-synthetic-rejected", ""}, {g.ID, b.ID, "", "AKIAsynthetic-rejected"}} {
		_, e = w.RecordSubmissionAccepted(ids[0], ids[1], ids[2], ids[3])
		reject(t, e)
	}
	if get[TaskBinding](t, w, "task_bindings", b.ID).BindingState != "UNBOUND" {
		t.Fatal("invalid acceptance mutated binding")
	}
	_, e = w.db.Exec("CREATE TRIGGER fail_accepted BEFORE UPDATE ON generations BEGIN SELECT RAISE(ABORT,'synthetic acceptance failure'); END")
	okay(t, e)
	_, e = w.RecordSubmissionAccepted(g.ID, b.ID, "fake-local", "fake-provider-task")
	reject(t, e)
	if get[TaskBinding](t, w, "task_bindings", b.ID).BindingState != "UNBOUND" || get[Generation](t, w, "generations", g.ID).SubmissionState != "SUBMITTING" {
		t.Fatal("partial acceptance transaction")
	}
	_, e = w.db.Exec("DROP TRIGGER fail_accepted")
	okay(t, e)
	bound, e := w.RecordSubmissionAccepted(g.ID, b.ID, "fake-local", "fake-provider-task")
	okay(t, e)
	accepted := get[Generation](t, w, "generations", g.ID)
	if accepted.Status != "SUBMITTED" || accepted.SubmissionState != "SUBMITTED" || bound.BindingState != "BOUND" || bound.ProviderTaskID != "fake-provider-task" || bound.UpstreamLocalTaskID != "fake-local" {
		t.Fatal(accepted, bound)
	}
	unchangedSubmissionRequest(t, g, accepted)
	again, e := w.RecordSubmissionAccepted(g.ID, b.ID, "fake-local", "fake-provider-task")
	okay(t, e)
	if !reflect.DeepEqual(bound, again) {
		t.Fatal("idempotent accepted record changed")
	}
	_, e = w.RecordSubmissionAccepted(g.ID, b.ID, "fake-local", "replacement")
	reject(t, e)
	_, _, e = w.BeginSubmission(g.ID)
	reject(t, e)
	okay(t, w.MarkSubmissionUnknown(g2.ID))
	_, _, e = w.BeginSubmission(g2.ID)
	reject(t, e)
	_, e = w.RecordSubmissionAccepted(g2.ID, b2.ID, "late", "late")
	reject(t, e)
	t.Log("ACCEPTED=SUBMITTED/BOUND EXACT_IDS=PASS REPLACEMENT=REJECTED UNKNOWN_RESEND=REJECTED")
}

func TestSubmissionReopenKeepsOwnerAndRequest(t *testing.T) {
	w := newWorkspace(t)
	g := submissionFixture(t, w)
	_, b, e := w.BeginSubmission(g.ID)
	okay(t, e)
	okay(t, w.Close())
	w, e = Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer w.Close()
	reopened := get[Generation](t, w, "generations", g.ID)
	if reopened.Status != "SUBMISSION_UNKNOWN" || reopened.SubmissionState != "SUBMISSION_UNKNOWN" || !reflect.DeepEqual(b, get[TaskBinding](t, w, "task_bindings", b.ID)) {
		t.Fatal("reopen lost ambiguous owner")
	}
	unchangedSubmissionRequest(t, g, reopened)
	_, _, e = w.BeginSubmission(g.ID)
	reject(t, e)
	okay(t, w.Close())
	w, e = Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer w.Close()
	if !reflect.DeepEqual(reopened, get[Generation](t, w, "generations", g.ID)) {
		t.Fatal("unknown reconciliation not deterministic")
	}
	v, e := w.Version()
	okay(t, e)
	if v != 1 {
		t.Fatal("schema changed")
	}
	t.Log("STALE_SUBMITTING_REOPEN=SUBMISSION_UNKNOWN BINDING_PRESERVED=PASS SECOND_REOPEN_UNCHANGED=PASS SCHEMA=1")
}

func TestSubmissionReconciliationKeepsAcceptedState(t *testing.T) {
	w := newWorkspace(t)
	g := submissionFixture(t, w)
	other, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer other.Close()
	_, b, e := w.BeginSubmission(g.ID)
	okay(t, e)
	_, e = w.RecordSubmissionAccepted(g.ID, b.ID, "", "fake-accepted-before-reconcile")
	okay(t, e)
	before := get[Generation](t, w, "generations", g.ID)
	bound := get[TaskBinding](t, w, "task_bindings", b.ID)
	// Represents another Open whose integrity phase preceded the accepted commit.
	okay(t, other.reconcileSubmissions())
	if !reflect.DeepEqual(before, get[Generation](t, w, "generations", g.ID)) || !reflect.DeepEqual(bound, get[TaskBinding](t, w, "task_bindings", b.ID)) {
		t.Fatal("reconciliation downgraded accepted facts")
	}
	t.Log("ACCEPTANCE_BEFORE_RECONCILE=SUBMITTED/BOUND CURRENT_STATE_GUARD=PASS")
}
