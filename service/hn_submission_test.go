package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

type hnFakeSubmit struct {
	calls atomic.Int32
	run   func(foundation.Generation) (SubmissionAcceptance, error)
}

func (f *hnFakeSubmit) Submit(_ context.Context, g foundation.Generation) (SubmissionAcceptance, error) {
	f.calls.Add(1)
	if f.run != nil {
		return f.run(g)
	}
	return SubmissionAcceptance{"fake-local-task", "fake-provider-task"}, nil
}

func hnSubmissionFixture(t *testing.T, root string) HNPreparedGeneration {
	t.Helper()
	s, err := EnsureShotForSourceNode(root, "test", "synthetic-shot", "submit shot")
	if err != nil {
		t.Fatal(err)
	}
	i := hnTestInput()
	i.NodeID = "synthetic-shot"
	i.ShotID = s.ShotID
	i.Protocol = "fake-protocol"
	i.ProviderIdentity = "fake-provider"
	i.ConnectionID = "fake-connection"
	i.Model = "fake-model"
	g, err := PrepareLocalGeneration(root, "test", i)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func hnSubmissionRead(t *testing.T, root, id string) (foundation.Generation, []foundation.TaskBinding) {
	t.Helper()
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	var g foundation.Generation
	if e = hnReadRecord(w, "generations", id, &g); e != nil {
		t.Fatal(e)
	}
	rows, e := w.List("task_bindings")
	if e != nil {
		t.Fatal(e)
	}
	bs := []foundation.TaskBinding{}
	for _, raw := range rows {
		var b foundation.TaskBinding
		if e = json.Unmarshal(raw, &b); e != nil {
			t.Fatal(e)
		}
		if b.GenerationID == id {
			bs = append(bs, b)
		}
	}
	for _, kind := range []string{"results", "archive_jobs"} {
		rows, e = w.List(kind)
		if e != nil || len(rows) != 0 {
			t.Fatal("submission created out-of-scope entity", kind, e)
		}
	}
	if e = w.IntegrityCheck(); e != nil {
		t.Fatal(e)
	}
	return g, bs
}

func hnSubmissionUnchanged(t *testing.T, a, b foundation.Generation) {
	t.Helper()
	a.Status, b.Status = "", ""
	a.SubmissionState, b.SubmissionState = "", ""
	a.UpdatedAt, b.UpdatedAt = "", ""
	if !reflect.DeepEqual(a, b) {
		t.Fatal("frozen transport request/SourceBaseline changed")
	}
}

func TestHNSubmissionSuccessAndNewAttempt(t *testing.T) {
	root := t.TempDir()
	prepared := hnSubmissionFixture(t, root)
	original, _ := hnSubmissionRead(t, root, prepared.GenerationID)
	f := &hnFakeSubmit{run: func(g foundation.Generation) (SubmissionAcceptance, error) {
		hnSubmissionUnchanged(t, original, g)
		return SubmissionAcceptance{"fake-local-task", "fake-provider-task"}, nil
	}}
	b, e := SubmitFrozenGeneration(context.Background(), root, "test", prepared.GenerationID, f)
	if e != nil || b.BindingState != "BOUND" || b.ProviderTaskID != "fake-provider-task" || b.UpstreamLocalTaskID != "fake-local-task" {
		t.Fatal(b, e)
	}
	g, bs := hnSubmissionRead(t, root, prepared.GenerationID)
	if g.Status != "SUBMITTED" || g.SubmissionState != "SUBMITTED" || len(bs) != 1 || !reflect.DeepEqual(b, bs[0]) {
		t.Fatal(g, bs)
	}
	hnSubmissionUnchanged(t, original, g)
	_, e = SubmitFrozenGeneration(context.Background(), root, "test", g.ID, f)
	if e == nil || f.calls.Load() != 1 {
		t.Fatal("resend after success", e, f.calls.Load())
	}
	second := hnSubmissionFixture(t, root)
	if second.GenerationID == prepared.GenerationID || second.ShotID != prepared.ShotID {
		t.Fatal("new attempt changed shot or reused generation")
	}
	f.run = nil
	_, e = SubmitFrozenGeneration(context.Background(), root, "test", second.GenerationID, f)
	if e != nil || f.calls.Load() != 2 {
		t.Fatal(e, f.calls.Load())
	}
	t.Log("SUCCESS_CALLS=1 REOPEN=SUBMITTED/BOUND SAME_ID_RESEND=REJECTED NEW_ID_SAME_SHOT=PASS")
}

func TestHNSubmissionAmbiguity(t *testing.T) {
	for _, mode := range []string{"error", "panic", "empty-acceptance", "unsafe-acceptance"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			p := hnSubmissionFixture(t, root)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			f := &hnFakeSubmit{run: func(foundation.Generation) (SubmissionAcceptance, error) {
				switch mode {
				case "error":
					return SubmissionAcceptance{}, errors.New("synthetic ambiguous failure")
				case "panic":
					panic("synthetic transport panic")
				case "unsafe-acceptance":
					return SubmissionAcceptance{ProviderTaskID: "https://example.invalid/task"}, nil
				}
				return SubmissionAcceptance{}, nil
			}}
			b, e := SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f)
			if !errors.Is(e, ErrHNSubmissionUnknown) || b.ID == "" {
				t.Fatal("missing explicit ambiguity", b, e)
			}
			g, bs := hnSubmissionRead(t, root, p.GenerationID)
			if g.Status != "SUBMISSION_UNKNOWN" || g.SubmissionState != "SUBMISSION_UNKNOWN" || len(bs) != 1 || bs[0].ID != b.ID || bs[0].BindingState != "UNBOUND" {
				t.Fatal(g, bs)
			}
			hnSubmissionUnchanged(t, before, g)
			_, e = SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f)
			if e == nil || f.calls.Load() != 1 {
				t.Fatal("unknown resent", e, f.calls.Load())
			}
			retry := hnSubmissionFixture(t, root)
			if retry.GenerationID == p.GenerationID || retry.ShotID != p.ShotID {
				t.Fatal("retry identity")
			}
			_, e = SubmitFrozenGeneration(context.Background(), root, "test", retry.GenerationID, &hnFakeSubmit{})
			if e != nil {
				t.Fatal(e)
			}
			t.Log("TRANSPORT_CALLS=1 UNKNOWN=PASS HISTORICAL_UNBOUND=PASS AUTO_RESEND=NONE NEW_ID_RETRY=PASS")
		})
	}
}

func TestHNSubmissionConcurrentDuplicate(t *testing.T) {
	root := t.TempDir()
	p := hnSubmissionFixture(t, root)
	entered := make(chan struct{})
	release := make(chan struct{})
	f := &hnFakeSubmit{run: func(foundation.Generation) (SubmissionAcceptance, error) {
		close(entered)
		<-release
		return SubmissionAcceptance{ProviderTaskID: "fake-concurrent-task"}, nil
	}}
	var wins atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, e := SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f); e == nil {
			wins.Add(1)
		}
	}()
	<-entered
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, e := SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f); e == nil {
			wins.Add(1)
		}
	}()
	close(release)
	wg.Wait()
	g, bs := hnSubmissionRead(t, root, p.GenerationID)
	if f.calls.Load() != 1 || wins.Load() != 1 || len(bs) != 1 || g.SubmissionState != "SUBMITTED" {
		t.Fatal("concurrent duplicate", f.calls.Load(), wins.Load(), g, bs)
	}
	t.Log("CONCURRENT_SERVICE_CALLERS=2 TRANSPORT_CALLS=1 OWNERS=1 BOUND=1")
}

func TestHNSubmissionPreSendValidation(t *testing.T) {
	root := t.TempDir()
	p := hnSubmissionFixture(t, root)
	f := &hnFakeSubmit{}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, input := range []struct {
		ctx         context.Context
		project, id string
		transport   SubmissionTransport
	}{{nil, "test", p.GenerationID, f}, {cancelled, "test", p.GenerationID, f}, {context.Background(), "../escape", p.GenerationID, f}, {context.Background(), "test", p.GenerationID, nil}, {context.Background(), "test", "missing-generation", f}} {
		if _, e := SubmitFrozenGeneration(input.ctx, root, input.project, input.id, input.transport); e == nil {
			t.Fatal("invalid submit accepted")
		}
	}
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	draft, e := w.CreateGeneration(foundation.Generation{PromptSnapshot: "not frozen"})
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	if _, e = SubmitFrozenGeneration(context.Background(), root, "test", draft.ID, f); e == nil {
		t.Fatal("draft submitted")
	}
	g, bs := hnSubmissionRead(t, root, p.GenerationID)
	if f.calls.Load() != 0 || len(bs) != 0 || g.SubmissionState != "PREPARED" {
		t.Fatal("pre-validation consumed attempt", g, bs)
	}
	t.Log("LOCAL_PRECONDITIONS=REJECTED TRANSPORT_CALLS=0 NO_OWNER_CREATED=PASS")
}

func TestHNSubmissionAcceptancePersistenceFailure(t *testing.T) {
	root := t.TempDir()
	p := hnSubmissionFixture(t, root)
	db, e := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec("CREATE TRIGGER fail_binding BEFORE UPDATE ON task_bindings BEGIN SELECT RAISE(ABORT,'synthetic accepted persistence failure'); END")
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	f := &hnFakeSubmit{}
	_, e = SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f)
	if !errors.Is(e, ErrHNSubmissionUnknown) {
		t.Fatal(e)
	}
	g, bs := hnSubmissionRead(t, root, p.GenerationID)
	if g.SubmissionState != "SUBMISSION_UNKNOWN" || len(bs) != 1 || bs[0].BindingState != "UNBOUND" || f.calls.Load() != 1 {
		t.Fatal(g, bs, f.calls.Load())
	}
	_, e = SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f)
	if e == nil || f.calls.Load() != 1 {
		t.Fatal("persistence error resent")
	}
	t.Log("ACCEPTANCE_PERSISTENCE_FAILURE=SUBMISSION_UNKNOWN TRANSPORT_CALLS=1 RESEND=NONE")
}

func TestHNSubmissionLocalSmoke(t *testing.T) {
	root := t.TempDir()
	prepared := []HNPreparedGeneration{}
	before := []foundation.Generation{}
	for range 3 {
		p := hnSubmissionFixture(t, root)
		prepared = append(prepared, p)
		g, _ := hnSubmissionRead(t, root, p.GenerationID)
		before = append(before, g)
	}
	for _, p := range prepared {
		if p.ShotID != prepared[0].ShotID {
			t.Fatal("smoke shot changed")
		}
	}
	success := &hnFakeSubmit{}
	if _, e := SubmitFrozenGeneration(context.Background(), root, "test", prepared[0].GenerationID, success); e != nil {
		t.Fatal(e)
	}
	ambiguous := &hnFakeSubmit{run: func(foundation.Generation) (SubmissionAcceptance, error) {
		return SubmissionAcceptance{}, errors.New("fake ambiguity")
	}}
	if _, e := SubmitFrozenGeneration(context.Background(), root, "test", prepared[1].GenerationID, ambiguous); !errors.Is(e, ErrHNSubmissionUnknown) {
		t.Fatal(e)
	}
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	g, b, e := w.BeginSubmission(prepared[2].GenerationID)
	if e != nil {
		t.Fatal(e)
	}
	crashTransport := &hnFakeSubmit{}
	a, e := crashTransport.Submit(context.Background(), g)
	if e != nil || a.ProviderTaskID == "" {
		t.Fatal(e)
	}
	w.Close() // Deliberately omit acceptance recording; reopen must sacrifice it.
	after := []foundation.Generation{}
	bindings := []foundation.TaskBinding{}
	for index, p := range prepared {
		g, bs := hnSubmissionRead(t, root, p.GenerationID)
		if len(bs) != 1 {
			t.Fatal(bs)
		}
		want := "SUBMISSION_UNKNOWN"
		if index == 0 {
			want = "SUBMITTED"
		}
		if g.SubmissionState != want {
			t.Fatal(g)
		}
		hnSubmissionUnchanged(t, before[index], g)
		after = append(after, g)
		bindings = append(bindings, bs[0])
		if _, e := SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, success); e == nil {
			t.Fatal("smoke resend")
		}
	}
	if bindings[2].ID != b.ID || bindings[2].BindingState != "UNBOUND" || success.calls.Load() != 1 || ambiguous.calls.Load() != 1 || crashTransport.calls.Load() != 1 {
		t.Fatal("smoke ownership/count")
	}
	if path := os.Getenv("HN_R10_SMOKE_EVIDENCE_PATH"); path != "" {
		data, e := json.MarshalIndent(map[string]any{"status": "PASS", "before": before, "after": after, "taskBindings": bindings, "transportCalls": []int32{success.calls.Load(), ambiguous.calls.Load(), crashTransport.calls.Load()}, "resultsCreated": 0, "archiveJobsCreated": 0, "realProviderCalls": 0, "paidCalls": 0, "credentialsResolved": false, "networkTransport": false}, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log("SMOKE=PASS SHOTS=1 GENERATIONS=3 BINDINGS=3 TRANSPORT_CALLS=1,1,1 STATES=SUBMITTED,UNKNOWN,UNKNOWN RESULTS=0 ARCHIVEJOBS=0")
}

// A subprocess exits without Close/record cleanup, proving both durable crash
// windows against the actual store rather than an in-memory state simulation.
func TestHNSubmissionCrashChild(t *testing.T) {
	mode := os.Getenv("HN_R10_CRASH_MODE")
	if mode == "" {
		return
	}
	root := os.Getenv("HN_R10_CRASH_ROOT")
	id := os.Getenv("HN_R10_CRASH_GENERATION")
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	g, _, e := w.BeginSubmission(id)
	if e != nil {
		t.Fatal(e)
	}
	if mode == "after-acceptance" {
		f := &hnFakeSubmit{}
		a, e := f.Submit(context.Background(), g)
		if e != nil || a.ProviderTaskID == "" || f.calls.Load() != 1 {
			t.Fatal("fake acceptance")
		}
		if e = os.WriteFile(filepath.Join(root, "fake-transport-call.txt"), []byte("1"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	os.Exit(0)
}

func TestHNSubmissionProcessChild(t *testing.T) {
	root := os.Getenv("HN_R10_PROCESS_ROOT")
	if root == "" {
		return
	}
	f := &hnFakeSubmit{run: func(foundation.Generation) (SubmissionAcceptance, error) {
		log, e := os.OpenFile(filepath.Join(root, "process-transport.txt"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if e != nil {
			return SubmissionAcceptance{}, e
		}
		defer log.Close()
		if _, e = log.WriteString("fake-submit\n"); e != nil {
			return SubmissionAcceptance{}, e
		}
		return SubmissionAcceptance{ProviderTaskID: "fake-process-task"}, nil
	}}
	// A competing Open can conservatively reconcile the winner to UNKNOWN;
	// regardless of acceptance timing neither process may ever resend.
	_, _ = SubmitFrozenGeneration(context.Background(), root, "test", os.Getenv("HN_R10_PROCESS_GENERATION"), f)
}

func TestHNSubmissionConcurrentProcesses(t *testing.T) {
	root := t.TempDir()
	p := hnSubmissionFixture(t, root)
	commands := []*exec.Cmd{}
	for range 2 {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHNSubmissionProcessChild$")
		cmd.Env = append(os.Environ(), "HN_R10_PROCESS_ROOT="+root, "HN_R10_PROCESS_GENERATION="+p.GenerationID)
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if e := cmd.Wait(); e != nil {
			t.Fatal(e)
		}
	}
	raw, e := os.ReadFile(filepath.Join(root, "process-transport.txt"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Count(string(raw), "fake-submit\n") != 1 {
		t.Fatal("duplicate cross-process transport", string(raw))
	}
	g, bs := hnSubmissionRead(t, root, p.GenerationID)
	if len(bs) != 1 || (g.SubmissionState != "SUBMITTED" && g.SubmissionState != "SUBMISSION_UNKNOWN") {
		t.Fatal(g, bs)
	}
	f := &hnFakeSubmit{}
	if _, e := SubmitFrozenGeneration(context.Background(), root, "test", p.GenerationID, f); e == nil || f.calls.Load() != 0 {
		t.Fatal("process attempt resent")
	}
	t.Logf("SEPARATE_PROCESSES=2 TRANSPORT_CALLS=1 TASKBINDINGS=1 STATE=%s RESEND=NONE", g.SubmissionState)
}

func TestHNSubmissionCrashWindows(t *testing.T) {
	for _, mode := range []string{"before-transport", "after-acceptance"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			p := hnSubmissionFixture(t, root)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			cmd := exec.Command(os.Args[0], "-test.run=^TestHNSubmissionCrashChild$")
			cmd.Env = append(os.Environ(), "HN_R10_CRASH_MODE="+mode, "HN_R10_CRASH_ROOT="+root, "HN_R10_CRASH_GENERATION="+p.GenerationID)
			if out, e := cmd.CombinedOutput(); e != nil {
				t.Fatalf("crash child: %v %s", e, out)
			}
			g, bs := hnSubmissionRead(t, root, p.GenerationID)
			if g.SubmissionState != "SUBMISSION_UNKNOWN" || len(bs) != 1 || bs[0].BindingState != "UNBOUND" || bs[0].ProviderTaskID != "" {
				t.Fatal("crash ambiguity lost", g, bs)
			}
			hnSubmissionUnchanged(t, before, g)
			calls := 0
			if b, e := os.ReadFile(filepath.Join(root, "fake-transport-call.txt")); e == nil && string(b) == "1" {
				calls = 1
			}
			want := 0
			if mode == "after-acceptance" {
				want = 1
			}
			if calls != want {
				t.Fatal("crash transport count", calls, want)
			}
			f := &hnFakeSubmit{}
			if _, e := SubmitFrozenGeneration(context.Background(), root, "test", g.ID, f); e == nil || f.calls.Load() != 0 {
				t.Fatal("resend after crash")
			}
			again, againBindings := hnSubmissionRead(t, root, g.ID)
			if !reflect.DeepEqual(g, again) || !reflect.DeepEqual(bs, againBindings) {
				t.Fatal("reopen altered historical attempt")
			}
			t.Logf("CRASH=%s PRECRASH_TRANSPORT_CALLS=%d REOPEN=SUBMISSION_UNKNOWN OWNER=UNBOUND RETRY_TRANSPORT_CALLS=0", mode, calls)
		})
	}
}
