package foundation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type r33ChildConfig struct {
	Root, Project, Mode, Point, Ready, Gate, Output, Mutation string
	Command                                                   ExportCommand
	CandidateID                                               string
}
type r33ChildResult struct {
	PID      int           `json:"pid"`
	Status   *ExportStatus `json:"status"`
	Error    string        `json:"error"`
	Revision string        `json:"revision"`
}

func r33File(t *testing.T, p string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	okay(t, e)
	okay(t, os.WriteFile(p, b, 0600))
}
func TestR33ProcessHelper(t *testing.T) {
	file := os.Getenv("HN_R33_CHILD_CONFIG")
	if file == "" {
		return
	}
	b, e := os.ReadFile(file)
	okay(t, e)
	var cfg r33ChildConfig
	okay(t, json.Unmarshal(b, &cfg))
	w, e := OpenExisting(cfg.Root, cfg.Project, false)
	okay(t, e)
	defer w.Close()
	wait := func(point string) {
		if cfg.Point != point {
			return
		}
		s, e := w.LookupExportCommand(cfg.Command.ExportIntentID)
		if cfg.Mode == "initialize" || point == "OPENED" {
			e = nil
		}
		if e != nil {
			t.Fatal(e)
		}
		r33File(t, cfg.Ready, map[string]any{"pid": os.Getpid(), "point": point, "status": s})
		until := time.Now().Add(25 * time.Second)
		for time.Now().Before(until) {
			if _, e = os.Stat(cfg.Gate); e == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("test gate timeout")
	}
	wait("OPENED")
	out := r33ChildResult{PID: os.Getpid()}
	switch cfg.Mode {
	case "initialize":
		_, e = w.InitializeExportProtocol()
	case "mutate":
		snap, err := w.ReadReorderSnapshot()
		okay(t, err)
		ids := []string{}
		for _, i := range snap.Items {
			ids = append(ids, i.SequenceItemID)
		}
		switch cfg.Mutation {
		case "reorder":
			ids[0], ids[len(ids)-1] = ids[len(ids)-1], ids[0]
			_, e = w.ExecuteReorderCommand(ReorderCommand{1, uuid.NewString(), snap.SequenceRevision, ids})
		case "append":
			_, e = w.AddSequenceItem("main", cfg.CandidateID)
		case "delete":
			e = w.Delete("sequence_items", ids[0])
		default:
			t.Fatal("bad test mutation")
		}
	default:
		s, err := w.executeExportCommand(cfg.Command, wait)
		out.Status = &s
		e = err
	}
	if e != nil {
		out.Error = e.Error()
	}
	out.Revision = r30RevisionFromDB(t, w)
	r33File(t, cfg.Output, out)
}

type r33Child struct {
	cmd  *exec.Cmd
	log  bytes.Buffer
	done chan struct{}
	err  error
	cfg  r33ChildConfig
}

func r33Start(t *testing.T, cfg r33ChildConfig) *r33Child {
	t.Helper()
	dir := t.TempDir()
	cfg.Ready = filepath.Join(dir, "ready.json")
	cfg.Gate = filepath.Join(dir, "gate")
	cfg.Output = filepath.Join(dir, "output.json")
	config := filepath.Join(dir, "config.json")
	r33File(t, config, cfg)
	c := &r33Child{done: make(chan struct{}), cfg: cfg}
	c.cmd = exec.Command(os.Args[0], "-test.run=^TestR33ProcessHelper$", "-test.v")
	c.cmd.Env = append(os.Environ(), "HN_R33_CHILD_CONFIG="+config)
	c.cmd.Stdout = &c.log
	c.cmd.Stderr = &c.log
	okay(t, c.cmd.Start())
	go func() { c.err = c.cmd.Wait(); close(c.done) }()
	t.Cleanup(func() {
		select {
		case <-c.done:
		default:
			_ = c.cmd.Process.Kill()
			select {
			case <-c.done:
			case <-time.After(3 * time.Second):
			}
		}
	})
	return c
}
func (c *r33Child) ready(t *testing.T) map[string]any {
	t.Helper()
	until := time.Now().Add(15 * time.Second)
	for time.Now().Before(until) {
		b, e := os.ReadFile(c.cfg.Ready)
		if e == nil {
			var v map[string]any
			if json.Unmarshal(b, &v) == nil {
				return v
			}
		}
		select {
		case <-c.done:
			t.Fatal("child exited before barrier", c.err, c.log.String())
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child did not reach barrier")
	return nil
}
func (c *r33Child) release(t *testing.T) {
	t.Helper()
	okay(t, os.WriteFile(c.cfg.Gate, []byte("release synthetic test gate"), 0600))
}
func (c *r33Child) result(t *testing.T) r33ChildResult {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(20 * time.Second):
		t.Fatal("child timed out")
	}
	if c.err != nil {
		t.Fatal(c.err, c.log.String())
	}
	b, e := os.ReadFile(c.cfg.Output)
	okay(t, e)
	var r r33ChildResult
	okay(t, json.Unmarshal(b, &r))
	return r
}
func (c *r33Child) kill(t *testing.T) {
	t.Helper()
	okay(t, c.cmd.Process.Kill())
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		t.Fatal("kill did not finish")
	}
	if c.err == nil {
		t.Fatal("child not terminated")
	}
}
func r33Existing(t *testing.T, root, project string) *Workspace {
	t.Helper()
	w, e := OpenExisting(root, project, false)
	okay(t, e)
	t.Cleanup(func() { w.Close() })
	return w
}
func r33FinalCount(t *testing.T, w *Workspace) int {
	t.Helper()
	p, e := w.Resolve("exports/commands-v1")
	okay(t, e)
	rows, e := os.ReadDir(p)
	if os.IsNotExist(e) {
		return 0
	}
	okay(t, e)
	return len(rows)
}
func TestR33RealProcessKillCrashMatrix(t *testing.T) {
	points := []string{"RESERVED", "COPYING", "PARTIAL_MEDIA", "ALL_MEDIA", "CSV", "JSON", "STAGE_COMPLETE", "READY_TO_FINALIZE", "FINAL_RENAMED", "FINAL_VERIFIED", "COMMITTED"}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			w, c := r33Fixture(t)
			root, project := filepath.Dir(w.Root), w.ProjectID
			okay(t, w.Close())
			child := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: "export", Point: point, Command: c})
			ready := child.ready(t)
			child.kill(t)
			reopened := r33Existing(t, root, project)
			before, e := reopened.LookupExportCommand(c.ExportIntentID)
			okay(t, e)
			j := r33Job(t, reopened, c)
			id := *j.ExportID
			attempt := j.AttemptCount
			revision := r30Revision(t, reopened)
			after, e := reopened.ExecuteExportCommand(c)
			okay(t, e)
			if after.Outcome != "COMMITTED" || *after.ExportID != id || r33FinalCount(t, reopened) != 1 || r30Revision(t, reopened) != revision || after.AttemptCount > 3 {
				t.Fatal(before, after)
			}
			h, e := reopened.VerifyExportBundle(c.ExportIntentID)
			okay(t, e)
			if h.Health != "VERIFIED" {
				t.Fatal(h)
			}
			if point == "STAGE_COMPLETE" || point == "READY_TO_FINALIZE" || point == "FINAL_RENAMED" || point == "FINAL_VERIFIED" || point == "COMMITTED" {
				if attempt != after.AttemptCount {
					t.Fatal("recopied valid stage/final")
				}
			}
			if point == "COMMITTED" && !reflect.DeepEqual(before, after) {
				t.Fatal("committed replay changed")
			}
			r33Evidence(t, "kill-"+point, map[string]any{"childPid": child.cmd.Process.Pid, "realProcessTermination": true, "exitCode": child.cmd.ProcessState.ExitCode(), "barrier": ready, "before": before, "after": after, "health": h, "finalBundleCount": 1, "originalExportID": id, "revisionBeforeAfter": revision, "log": child.log.String()})
		})
	}
}
func TestR33RealTwoProcessSameDifferentIntentsAndInitialize(t *testing.T) {
	for _, kind := range []string{"initialize", "same-key", "different-key"} {
		t.Run(kind, func(t *testing.T) {
			w, c := r33Fixture(t)
			if kind == "initialize" {
				for n := len(exportSchema) - 1; n >= 0; n-- {
					v := exportSchema[n]
					sqlText := "DROP TRIGGER " + v.name
					if n < 3 {
						sqlText = "DROP TABLE " + v.name
					}
					_, e := w.db.Exec(sqlText)
					okay(t, e)
				}
			}
			root, project := filepath.Dir(w.Root), w.ProjectID
			okay(t, w.Close())
			c2 := c
			if kind == "different-key" {
				c2.ExportIntentID = uuid.NewString()
			}
			mode := "export"
			if kind == "initialize" {
				mode = "initialize"
			}
			a := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: mode, Point: "OPENED", Command: c})
			b := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: mode, Point: "OPENED", Command: c2})
			ra, rb := a.ready(t), b.ready(t)
			if a.cmd.Process.Pid == b.cmd.Process.Pid {
				t.Fatal("not two processes")
			}
			a.release(t)
			b.release(t)
			x, y := a.result(t), b.result(t)
			reopened := r33Existing(t, root, project)
			if x.Error != "" || y.Error != "" {
				t.Fatal(x, y)
			}
			count := 0
			if kind == "initialize" {
				_, e := reopened.InitializeExportProtocol()
				okay(t, e)
			} else {
				sx, e := reopened.ExecuteExportCommand(c)
				okay(t, e)
				sy, e := reopened.ExecuteExportCommand(c2)
				okay(t, e)
				if sx.Outcome != "COMMITTED" || sy.Outcome != "COMMITTED" {
					t.Fatal(sx, sy)
				}
				count = r33FinalCount(t, reopened)
				expected := 1
				if kind == "different-key" {
					expected = 2
				}
				if count != expected {
					t.Fatal(count)
				}
				if kind == "same-key" && !reflect.DeepEqual(sx, sy) {
					t.Fatal("same key effect differs")
				}
			}
			r33Evidence(t, "two-process-"+kind, map[string]any{"childProcesses": 2, "bothOpenedBeforeRelease": true, "ready": []any{ra, rb}, "outputs": []any{x, y}, "finalBundleCount": count, "logs": []string{a.log.String(), b.log.String()}})
		})
	}
}
func TestR33SuspendedOwnerConflictAndKillUnlock(t *testing.T) {
	w, c := r33Fixture(t)
	root, project := filepath.Dir(w.Root), w.ProjectID
	okay(t, w.Close())
	owner := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: "export", Point: "RESERVED", Command: c})
	ready := owner.ready(t)
	resume, e := r33SuspendProcess(owner.cmd.Process.Pid)
	okay(t, e)
	defer resume()
	competing := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: "export", Command: c})
	same := competing.result(t)
	if same.Error != "" || same.Status == nil || same.Status.Outcome != "IN_PROGRESS" {
		t.Fatal(same)
	}
	changed := c
	changed.ExpectedRevision = "0"
	other := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: "export", Command: changed})
	conflict := other.result(t)
	if conflict.Error != "EXPORT_INTENT_CONFLICT" {
		t.Fatal(conflict)
	}
	owner.kill(t)
	reopened := r33Existing(t, root, project)
	j := r33Job(t, reopened, c)
	id := *j.ExportID
	lockDir, _ := reopened.Resolve("metadata/export-locks")
	files, e := os.ReadDir(lockDir)
	okay(t, e)
	if len(files) != 1 {
		t.Fatal("lock identity changed")
	}
	s, e := reopened.ExecuteExportCommand(c)
	okay(t, e)
	if s.Outcome != "COMMITTED" || *s.ExportID != id {
		t.Fatal(s)
	}
	after, e := os.ReadDir(lockDir)
	okay(t, e)
	if len(after) != 1 || files[0].Name() != after[0].Name() {
		t.Fatal("lock file unlinked")
	}
	r33Evidence(t, "suspended-owner", map[string]any{"ownerPid": owner.cmd.Process.Pid, "actualKernelSuspension": true, "ready": ready, "competing": same, "conflict": conflict, "ownerKilled": true, "continued": s, "persistentLockName": files[0].Name(), "ttlStealing": false})
}
func TestR33SequenceWritesDuringReservedExportTwoProcess(t *testing.T) {
	for _, kind := range []string{"reorder", "append", "delete"} {
		t.Run(kind, func(t *testing.T) {
			w, c := r33Fixture(t)
			i := get[SequenceItem](t, w, "sequence_items", c.OrderedSequenceItemIDs[0])
			okay(t, w.SelectCandidate(i.ShotID, i.CandidateID))
			root, project := filepath.Dir(w.Root), w.ProjectID
			okay(t, w.Close())
			owner := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: "export", Point: "RESERVED", Command: c})
			owner.ready(t)
			writer := r33Start(t, r33ChildConfig{Root: root, Project: project, Mode: "mutate", Mutation: kind, Command: c, CandidateID: i.CandidateID})
			mutation := writer.result(t)
			if mutation.Error != "" || mutation.Revision == c.ExpectedRevision {
				t.Fatal(mutation)
			}
			owner.release(t)
			out := owner.result(t)
			if out.Error != "" || out.Status == nil || out.Status.Outcome != "COMMITTED" || !reflect.DeepEqual(out.Status.Receipt.OrderedSequenceItemIDs, c.OrderedSequenceItemIDs) || out.Status.Receipt.ObservedRevision != c.ExpectedRevision {
				t.Fatal(out)
			}
			reopened := r33Existing(t, root, project)
			if r30Revision(t, reopened) != mutation.Revision {
				t.Fatal("export changed revision")
			}
			r33Evidence(t, "sequence-race-"+kind, map[string]any{"childProcesses": 2, "ownerPid": owner.cmd.Process.Pid, "writerPid": writer.cmd.Process.Pid, "writerCompletedBeforeCopyRelease": true, "mutation": mutation, "export": out, "historicalVector": c.OrderedSequenceItemIDs, "currentRevision": mutation.Revision})
		})
	}
}
func ExampleExportCanonical() {
	fmt.Println(ExportCanonical(ExportCommand{1, "", "0", []string{}, ExportFormat})) // Output: {"expectedRevision":"0","orderedSequenceItemIds":[],"format":"hn-offline-bundle-v1"}
}
