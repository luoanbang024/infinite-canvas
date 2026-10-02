package foundation

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func must[T any](t *testing.T, v T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func okay(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func reject(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected rejection")
	}
}
func newWorkspace(t *testing.T) *Workspace {
	t.Helper()
	w, err := Open(t.TempDir(), "project-a")
	okay(t, err)
	t.Cleanup(func() { w.Close() })
	return w
}
func get[T any](t *testing.T, w *Workspace, kind, id string) T {
	t.Helper()
	b, err := w.Read(kind, id)
	okay(t, err)
	var v T
	okay(t, json.Unmarshal(b, &v))
	return v
}
func count(t *testing.T, w *Workspace, kind string) int {
	t.Helper()
	v, e := w.List(kind)
	okay(t, e)
	return len(v)
}
func makeShot(t *testing.T, w *Workspace, label string) Shot {
	s, e := w.CreateShot(label, "")
	return must(t, s, e)
}
func makeGeneration(t *testing.T, w *Workspace, shotID, prompt string) Generation {
	g, e := w.CreateGeneration(Generation{ShotID: shotID, PromptSnapshot: prompt})
	okay(t, e)
	g, e = w.FreezeGeneration(g.ID)
	return must(t, g, e)
}
func makeResult(t *testing.T, w *Workspace, g Generation) Result {
	r, e := w.CreateResult(Result{GenerationID: g.ID, ResultKind: "video"})
	return must(t, r, e)
}
func archive(t *testing.T, w *Workspace, r Result, data []byte) ArchiveJob {
	j, e := w.CreateArchive(r.ID, "generated/"+r.GenerationID+"/"+r.ID+"/media.mp4", "video/mp4")
	okay(t, e)
	okay(t, w.RunArchive(j.ID, bytes.NewReader(data), int64(len(data)), digest(data)))
	return get[ArchiveJob](t, w, "archive_jobs", j.ID)
}

func TestWorkspaceSchemaAndPaths(t *testing.T) {
	parent := t.TempDir()
	for _, id := range []string{"..", "../outside", "a/b", "a\\b", "C:drive", "CON", "nul", "COM1", "LPT9", "name.", "name "} {
		_, e := Open(parent, id)
		reject(t, e)
	}
	w, e := Open(parent, "")
	okay(t, e)
	defer w.Close()
	if !validName(w.ProjectID) {
		t.Fatal("generated unsafe ID")
	}
	for _, dir := range []string{"assets", "references", "generated", "exports", "metadata"} {
		p, e := w.Resolve(dir)
		okay(t, e)
		i, e := os.Stat(p)
		okay(t, e)
		if !i.IsDir() {
			t.Fatal(dir)
		}
	}
	v, e := w.Version()
	okay(t, e)
	if v != 1 {
		t.Fatal(v)
	}
	okay(t, w.IntegrityCheck())
	for _, p := range []string{"", "../outside", "assets/../../bad", "/absolute", "C:/absolute", "C:relative", "\\\\server\\share", "assets\\bad", "assets/a:stream", "assets//file", "assets/./file", "assets/CON.bin", "assets/a. ", "assets/a\x00"} {
		_, e := w.Resolve(p)
		reject(t, e)
	}
	if runtime.GOOS != "windows" {
		outside := t.TempDir()
		okay(t, os.Symlink(outside, filepath.Join(w.Root, "assets", "linked")))
		_, e = w.Resolve("assets/linked/file.bin")
		reject(t, e)
	} else {
		outside := t.TempDir()
		linked := filepath.Join(w.Root, "assets", "linked")
		output, err := exec.Command("cmd", "/c", "mklink", "/J", linked, outside).CombinedOutput()
		if err != nil {
			t.Fatalf("local test junction creation: %v %s", err, output)
		}
		_, e = w.Resolve("assets/linked/file.bin")
		reject(t, e)
		alias := filepath.Join(parent, "project-alias")
		output, err = exec.Command("cmd", "/c", "mklink", "/J", alias, outside).CombinedOutput()
		if err != nil {
			t.Fatalf("local project junction creation: %v %s", err, output)
		}
		_, e = Open(parent, "project-alias")
		reject(t, e)
	}
}
func TestUnsupportedSchemaAndWrongProject(t *testing.T) {
	parent := t.TempDir()
	w, e := Open(parent, "project-a")
	okay(t, e)
	_, e = w.db.Exec("PRAGMA user_version=2")
	okay(t, e)
	okay(t, w.Close())
	_, e = Open(parent, "project-a")
	reject(t, e)
	w, e = Open(parent, "project-b")
	okay(t, e)
	_, e = w.db.Exec("UPDATE project SET id='other-project'")
	okay(t, e)
	okay(t, w.Close())
	_, e = Open(parent, "project-b")
	reject(t, e)
	w, e = Open(parent, "project-c")
	okay(t, e)
	_, e = w.db.Exec("PRAGMA user_version=0")
	okay(t, e)
	okay(t, w.Close())
	_, e = Open(parent, "project-c")
	reject(t, e)
}

func TestReferenceSnapshotIdentityAndImmutability(t *testing.T) {
	w := newWorkspace(t)
	source := filepath.Join(t.TempDir(), "source.bin")
	input := []byte("synthetic reference version one\x00\x01")
	okay(t, os.WriteFile(source, input, 0600))
	r, e := w.SnapshotFile(source, "ref-a", "image", "image/png")
	okay(t, e)
	if r.ByteLength != int64(len(input)) || r.SHA256 != digest(input) {
		t.Fatal("snapshot facts mismatch")
	}
	path, e := w.Resolve(r.RelativePath)
	okay(t, e)
	b, e := os.ReadFile(path)
	okay(t, e)
	if !bytes.Equal(input, b) {
		t.Fatal("not submitted bytes")
	}
	okay(t, w.verifyReference(r))
	okay(t, os.WriteFile(source, []byte("changed input"), 0600))
	okay(t, w.verifyReference(r))
	r2, e := w.SnapshotFile(source, "ref-a", "image", "image/png")
	okay(t, e)
	if r2.ID == r.ID || r2.SHA256 == r.SHA256 {
		t.Fatal("new bytes must get a new version/hash")
	}
	r3, e := w.Snapshot("ref-a", "image", "image/png", bytes.NewReader(input), int64(len(input)))
	okay(t, e)
	if r3.ID == r.ID || r3.SHA256 != r.SHA256 {
		t.Fatal("duplicate content must not collapse identity")
	}
	okay(t, w.Close())
	reopened, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer reopened.Close()
	stored := get[ReferenceVersion](t, reopened, "reference_versions", r.ID)
	if !reflect.DeepEqual(stored, r) {
		t.Fatal("reference changed on reopen")
	}
	okay(t, reopened.verifyReference(stored))
	okay(t, os.WriteFile(path, []byte("tampered"), 0600))
	okay(t, reopened.Close())
	_, e = Open(filepath.Dir(w.Root), w.ProjectID)
	reject(t, e)
}

type brokenReader struct{ sent bool }

func (b *brokenReader) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, "partial"), nil
	}
	return 0, errors.New("synthetic interrupted source")
}
func TestIncompleteReferenceLeavesNoCompletedRecord(t *testing.T) {
	w := newWorkspace(t)
	_, e := w.Snapshot("ref-a", "image", "image/png", &brokenReader{}, 100)
	reject(t, e)
	_, e = w.Snapshot("ref-a", "image", "image/png", strings.NewReader("short"), 100)
	reject(t, e)
	if count(t, w, "reference_versions") != 0 {
		t.Fatal("false completed reference")
	}
	okay(t, filepath.Walk(w.Root, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !i.IsDir() {
			if strings.Contains(i.Name(), "hn-partial-") || i.Name() == "reference.bin" {
				t.Fatal("incomplete final/temp survived")
			}
		}
		return nil
	}))
}

func TestRequestSnapshotFreezeAndArchivePathOwnership(t *testing.T) {
	w := newWorkspace(t)
	path, e := w.Resolve("metadata/request.json")
	okay(t, e)
	payload := []byte(`{"prompt":"synthetic request","parameters":{"seconds":5}}`)
	okay(t, os.WriteFile(path, payload, 0600))
	g, e := w.CreateGeneration(Generation{RequestSnapshotRef: "metadata/request.json", RequestSnapshotHash: digest(payload)})
	okay(t, e)
	_, e = w.FreezeGeneration(g.ID)
	okay(t, e)
	r := makeResult(t, w, g)
	_, e = w.CreateArchive(r.ID, "generated/another-generation/media.mp4", "video/mp4")
	reject(t, e)
	j, e := w.CreateArchive(r.ID, "generated/"+g.ID+"/fixed.mp4", "video/mp4")
	okay(t, e)
	r2 := makeResult(t, w, g)
	_, e = w.CreateArchive(r2.ID, j.TargetRelativePath, "video/mp4")
	reject(t, e)
	_, e = w.CreateArchive(r2.ID, "generated/"+g.ID+"/FIXED.mp4", "video/mp4")
	reject(t, e)
	okay(t, os.WriteFile(path, []byte(`{"prompt":"tampered request"}`), 0600))
	_, e = w.FreezeGeneration(g.ID)
	reject(t, e)
	okay(t, w.Close())
	_, e = Open(filepath.Dir(w.Root), w.ProjectID)
	reject(t, e)
}

func TestGenerationFreezeAndBindings(t *testing.T) {
	w := newWorkspace(t)
	s := makeShot(t, w, "shot")
	r, e := w.Snapshot("ref-a", "image", "image/png", strings.NewReader("ref"), 3)
	okay(t, e)
	g, e := w.CreateGeneration(Generation{ShotID: s.ID, PromptSnapshot: "first", Parameters: json.RawMessage(`{"duration":5}`), ReferenceBindings: []ReferenceBinding{{r.ID, r.SHA256, "first_frame"}}, CredentialRef: "future-connection"})
	okay(t, e)
	g.PromptSnapshot = "final"
	okay(t, w.UpdateGeneration(g))
	g, e = w.FreezeGeneration(g.ID)
	okay(t, e)
	if !g.Frozen || g.Status != "PREPARED" || g.FrozenHash == "" {
		t.Fatal("not frozen durably")
	}
	changes := []func(*Generation){func(g *Generation) { g.PromptSnapshot = "bad" }, func(g *Generation) { g.Model = "bad" }, func(g *Generation) { g.ShotID = "foreign" }, func(g *Generation) { g.Parameters = json.RawMessage(`{"duration":10}`) }, func(g *Generation) { g.ReferenceBindings = nil }, func(g *Generation) { g.ConnectionID = "bad" }, func(g *Generation) { g.ProjectID = "other" }, func(g *Generation) { g.SourceBaseline = "bad" }, func(g *Generation) { g.CredentialRef = "bad" }}
	for _, change := range changes {
		copy := g
		change(&copy)
		reject(t, w.UpdateGeneration(copy))
	}
	okay(t, w.MarkSubmissionUnknown(g.ID))
	g = get[Generation](t, w, "generations", g.ID)
	if g.SubmissionState != "SUBMISSION_UNKNOWN" {
		t.Fatal(g.SubmissionState)
	}
	b, e := w.CreateTaskBinding(TaskBinding{GenerationID: g.ID})
	okay(t, e)
	if b.ProviderTaskID != "" || b.BindingState != "UNBOUND" {
		t.Fatal("fabricated provider fact")
	}
	b.ProviderTaskID = "known-test-task"
	okay(t, w.UpdateTaskBinding(b))
	b = get[TaskBinding](t, w, "task_bindings", b.ID)
	b.ProviderTaskID = "another-task"
	reject(t, w.UpdateTaskBinding(b))
	g2 := makeGeneration(t, w, s.ID, "newer")
	_, e = w.CreateResult(Result{GenerationID: g2.ID, TaskBindingID: b.ID, ResultKind: "video"})
	reject(t, e)
	foreign := g
	foreign.ProjectID = "other-project"
	reject(t, w.UpdateGeneration(foreign))
	reject(t, w.Delete("reference_versions", r.ID))
	reject(t, w.Delete("generations", g.ID))
	_, e = w.CreateGeneration(Generation{Parameters: json.RawMessage(`{"api_key":"synthetic-not-a-credential"}`)})
	reject(t, e)
	_, e = w.CreateGeneration(Generation{Parameters: json.RawMessage(`{"url":"https://example.invalid/file?sig=synthetic"}`)})
	reject(t, e)
	_, e = w.CreateResult(Result{GenerationID: g.ID, ResultKind: "video", SourceURLRef: "https://example.invalid/video"})
	reject(t, e)
	okay(t, w.Close())
	reopened, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer reopened.Close()
	old := get[Generation](t, reopened, "generations", g.ID)
	if old.FrozenHash != g.FrozenHash || old.SubmissionState != "SUBMISSION_UNKNOWN" {
		t.Fatal("freeze lost after restart")
	}
}

func TestArchiveRetryAndLateResultSelection(t *testing.T) {
	w := newWorkspace(t)
	s := makeShot(t, w, "shot")
	old := makeGeneration(t, w, s.ID, "old")
	newer := makeGeneration(t, w, s.ID, "newer")
	newResult := makeResult(t, w, newer)
	data := []byte("synthetic offline video bytes")
	j, e := w.CreateArchive(newResult.ID, "", "video/mp4")
	okay(t, e)
	reject(t, w.RunArchive(j.ID, &brokenReader{}, 100, ""))
	failed := get[ArchiveJob](t, w, "archive_jobs", j.ID)
	if failed.Status != "FAILED" || failed.AttemptCount != 1 {
		t.Fatal(failed)
	}
	okay(t, w.RunArchive(j.ID, bytes.NewReader(data), int64(len(data)), digest(data)))
	done := get[ArchiveJob](t, w, "archive_jobs", j.ID)
	if done.Status != "ARCHIVED" || done.AttemptCount != 2 {
		t.Fatal(done)
	}
	okay(t, w.verifyArchive(done))
	okay(t, w.RunArchive(j.ID, nil, int64(len(data)), digest(data)))
	again := get[ArchiveJob](t, w, "archive_jobs", j.ID)
	if again.AttemptCount != 2 {
		t.Fatal("verified retry duplicated bytes")
	}
	if count(t, w, "generations") != 2 {
		t.Fatal("archive regenerated")
	}
	c, e := w.CreateCandidate(s.ID, newResult.ID, "new")
	okay(t, e)
	shot := get[Shot](t, w, "shots", s.ID)
	if shot.SelectedCandidateID != "" {
		t.Fatal("arrival auto-selected")
	}
	okay(t, w.SelectCandidate(s.ID, c.ID))
	late := makeResult(t, w, old)
	archive(t, w, late, []byte("old late bytes"))
	lateCandidate, e := w.CreateCandidate(s.ID, late.ID, "old late")
	okay(t, e)
	shot = get[Shot](t, w, "shots", s.ID)
	if shot.SelectedCandidateID != c.ID {
		t.Fatal("late result replaced selection")
	}
	if lateCandidate.GenerationID != old.ID {
		t.Fatal("late result reassigned")
	}
	if get[Generation](t, w, "generations", newer.ID).PromptSnapshot != "newer" {
		t.Fatal("new generation overwritten")
	}
	_, e = w.CreateArchive(late.ID, "generated/../outside.mp4", "")
	reject(t, e)
	other := makeShot(t, w, "other")
	reject(t, w.SelectCandidate(other.ID, c.ID))
	_, e = w.CreateCandidate(other.ID, late.ID, "wrong")
	reject(t, e)
	okay(t, w.IntegrityCheck())
}

func TestArchiveFinalizationRecoveryAndMismatch(t *testing.T) {
	w := newWorkspace(t)
	g := makeGeneration(t, w, "", "test")
	r := makeResult(t, w, g)
	j, e := w.CreateArchive(r.ID, "", "video/mp4")
	okay(t, e)
	data := []byte("complete bytes before simulated process stop")
	temp, n, hash, e := w.tempFile(j.TargetRelativePath, bytes.NewReader(data), int64(len(data)), digest(data))
	okay(t, e)
	j.Status = "FINALIZING"
	j.ActualBytes = n
	j.ActualSHA256 = hash
	j.AttemptCount = 1
	okay(t, put(w.db, "archive_jobs", j.ID, j))
	okay(t, w.publish(temp, j.TargetRelativePath))
	// Simulate process stopping after rename but before sidecar/DB completion.
	okay(t, w.Close())
	reopened, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer reopened.Close()
	done := get[ArchiveJob](t, reopened, "archive_jobs", j.ID)
	if done.Status != "ARCHIVED" {
		t.Fatal(done.Status)
	}
	result := get[Result](t, reopened, "results", r.ID)
	if result.Status != "ARCHIVED" || result.SHA256 != hash {
		t.Fatal("file/metadata not reconciled")
	}
	okay(t, reopened.verifyArchive(done))
	path, e := reopened.Resolve(done.TargetRelativePath)
	okay(t, e)
	okay(t, os.WriteFile(path, []byte("corrupt"), 0600))
	okay(t, reopened.Close())
	reopened2, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer reopened2.Close()
	if get[ArchiveJob](t, reopened2, "archive_jobs", j.ID).Status != "INCONSISTENT" || get[Result](t, reopened2, "results", r.ID).Status == "ARCHIVED" {
		t.Fatal("false durable completion")
	}
}

func TestInterruptedAndMissingArchiveDoesNotComplete(t *testing.T) {
	w := newWorkspace(t)
	g := makeGeneration(t, w, "", "test")
	r := makeResult(t, w, g)
	j, e := w.CreateArchive(r.ID, "", "video/mp4")
	okay(t, e)
	j.Status = "COPYING"
	j.AttemptCount = 1
	okay(t, put(w.db, "archive_jobs", j.ID, j))
	okay(t, w.Close())
	reopened, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer reopened.Close()
	if get[ArchiveJob](t, reopened, "archive_jobs", j.ID).Status != "FAILED" {
		t.Fatal("interruption not recorded")
	}
	data := []byte("synthetic retry")
	okay(t, reopened.RunArchive(j.ID, bytes.NewReader(data), int64(len(data)), digest(data)))
	done := get[ArchiveJob](t, reopened, "archive_jobs", j.ID)
	path, e := reopened.Resolve(done.TargetRelativePath)
	okay(t, e)
	okay(t, os.Remove(path))
	okay(t, reopened.Close())
	w2, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer w2.Close()
	if get[Result](t, w2, "results", r.ID).Status == "ARCHIVED" {
		t.Fatal("missing file falsely completed")
	}
	if count(t, w2, "generations") != 1 {
		t.Fatal("restart regenerated")
	}
}

func TestSequenceStableReorderAndOfflineExport(t *testing.T) {
	w := newWorkspace(t)
	items := []SequenceItem{}
	original := map[string]string{}
	for index, label := range []string{"one", "two"} {
		s := makeShot(t, w, label)
		g := makeGeneration(t, w, s.ID, label)
		r := makeResult(t, w, g)
		archive(t, w, r, []byte("synthetic-media-"+label))
		c, e := w.CreateCandidate(s.ID, r.ID, label)
		okay(t, e)
		okay(t, w.SelectCandidate(s.ID, c.ID))
		i, e := w.AddSequenceItem("sequence-a", c.ID)
		okay(t, e)
		items = append(items, i)
		original[i.ID] = r.ID
		if index == 0 {
			repeat, e := w.AddSequenceItem("sequence-a", c.ID)
			okay(t, e)
			items = append(items, repeat)
			original[repeat.ID] = r.ID
		}
	}
	order := []string{items[2].ID, items[0].ID, items[1].ID}
	okay(t, w.Reorder("sequence-a", order))
	reject(t, w.Reorder("sequence-a", []string{items[0].ID, items[0].ID, items[1].ID}))
	for n, id := range order {
		i := get[SequenceItem](t, w, "sequence_items", id)
		if i.ID != id || i.OrderIndex != n || i.ResultID != original[id] {
			t.Fatal("reorder changed identity")
		}
	}
	m, e := w.Export("sequence-a")
	okay(t, e)
	if len(m.Items) != 3 {
		t.Fatal("manifest size")
	}
	for n, i := range m.Items {
		if i.SequenceItemID != order[n] || i.SequenceIndex != n+1 || i.DurationSeconds != nil {
			t.Fatal("manifest order/duration invented")
		}
		okay(t, w.verifyFile(i.RelativePath, i.SHA256, i.ByteLength))
		if filepath.IsAbs(i.RelativePath) {
			t.Fatal("absolute archive identity")
		}
	}
	jsonPath, e := w.Resolve("exports/" + m.ExportID + "/ordered-manifest.json")
	okay(t, e)
	b, e := os.ReadFile(jsonPath)
	okay(t, e)
	var stored ExportManifest
	okay(t, json.Unmarshal(b, &stored))
	if !reflect.DeepEqual(m, stored) {
		t.Fatal("JSON mismatch")
	}
	csvPath, e := w.Resolve("exports/" + m.ExportID + "/ordered-manifest.csv")
	okay(t, e)
	f, e := os.Open(csvPath)
	okay(t, e)
	rows, e := csv.NewReader(f).ReadAll()
	f.Close()
	okay(t, e)
	if len(rows) != 4 || rows[1][2] != order[0] || rows[1][7] != m.Items[0].SHA256 || rows[1][9] != "" {
		t.Fatal("CSV mismatch")
	}
	if output := os.Getenv("HN_R2_SAMPLE_DIR"); output != "" {
		okay(t, os.MkdirAll(output, 0700))
		okay(t, os.WriteFile(filepath.Join(output, "ordered-manifest.json"), b, 0600))
		csvBytes, e := os.ReadFile(csvPath)
		okay(t, e)
		okay(t, os.WriteFile(filepath.Join(output, "ordered-manifest.csv"), csvBytes, 0600))
	}
	// A persisted placement survives later selection changes; no fallback.
	okay(t, w.SelectCandidate(m.Items[0].ShotID, ""))
	next, e := w.Export("sequence-a")
	okay(t, e)
	if next.Items[0].CandidateID != m.Items[0].CandidateID {
		t.Fatal("placement replaced by current selection")
	}
}

func TestEntityCRUDAndProjectConstraints(t *testing.T) {
	w := newWorkspace(t)
	s := makeShot(t, w, "draft")
	okay(t, w.RenameShot(s.ID, "renamed"))
	if get[Shot](t, w, "shots", s.ID).Label != "renamed" {
		t.Fatal("shot update")
	}
	draft, e := w.CreateGeneration(Generation{ShotID: s.ID, PromptSnapshot: "draft"})
	okay(t, e)
	draft.PromptSnapshot = "edit"
	okay(t, w.UpdateGeneration(draft))
	okay(t, w.Delete("generations", draft.ID))
	rf, e := w.Snapshot("unbound-ref", "image", "image/png", strings.NewReader("x"), 1)
	okay(t, e)
	okay(t, w.Delete("reference_versions", rf.ID))
	_, e = w.Read("reference_versions", rf.ID)
	if !errors.Is(e, sql.ErrNoRows) {
		t.Fatal(e)
	}
	g := makeGeneration(t, w, s.ID, "frozen")
	binding, e := w.CreateTaskBinding(TaskBinding{GenerationID: g.ID})
	okay(t, e)
	binding.LastPolledAt = timestamp()
	okay(t, w.UpdateTaskBinding(binding))
	r, e := w.CreateResult(Result{GenerationID: g.ID, TaskBindingID: binding.ID, ResultKind: "video"})
	okay(t, e)
	j, e := w.CreateArchive(r.ID, "", "video/mp4")
	okay(t, e)
	c, e := w.CreateCandidate(s.ID, r.ID, "candidate")
	okay(t, e)
	okay(t, w.RenameCandidate(c.ID, "candidate-edited"))
	okay(t, w.SelectCandidate(s.ID, c.ID))
	i, e := w.AddSequenceItem("seq", c.ID)
	okay(t, e)
	for kind := range tables {
		if count(t, w, kind) < 1 && kind != "reference_versions" {
			t.Fatal("missing entity", kind)
		}
	}
	reject(t, w.Delete("candidates", c.ID))
	reject(t, w.Delete("results", r.ID))
	reject(t, w.Delete("task_bindings", binding.ID))
	reject(t, w.Delete("shots", s.ID))
	okay(t, w.Delete("sequence_items", i.ID))
	okay(t, w.SelectCandidate(s.ID, ""))
	okay(t, w.Delete("candidates", c.ID))
	okay(t, w.Delete("archive_jobs", j.ID))
	if get[Result](t, w, "results", r.ID).ArchiveJobID != "" {
		t.Fatal("dangling archive binding")
	}
	okay(t, w.Delete("results", r.ID))
	okay(t, w.Delete("task_bindings", binding.ID))
	other, e := Open(t.TempDir(), "project-b")
	okay(t, e)
	defer other.Close()
	_, e = other.CreateGeneration(Generation{Identity: Identity{ProjectID: w.ProjectID}})
	reject(t, e)
	_, e = other.CreateGeneration(Generation{ShotID: s.ID})
	reject(t, e)
	_, e = other.CreateResult(Result{GenerationID: g.ID, ResultKind: "video"})
	reject(t, e)
	_, e = w.List("not_a_table")
	reject(t, e)
	reject(t, w.Delete("results; DROP TABLE project", r.ID))
	okay(t, w.IntegrityCheck())
}

func TestWorkspaceProcessRestart(t *testing.T) {
	w := newWorkspace(t)
	r, e := w.Snapshot("ref-a", "image", "image/png", strings.NewReader("restart bytes"), 13)
	okay(t, e)
	g, e := w.CreateGeneration(Generation{ReferenceBindings: []ReferenceBinding{{r.ID, r.SHA256, "reference"}}})
	okay(t, e)
	_, e = w.FreezeGeneration(g.ID)
	okay(t, e)
	okay(t, w.Close())
	executable, e := os.Executable()
	okay(t, e)
	cmd := exec.Command(executable, "-test.run=^TestWorkspaceProcessHelper$")
	cmd.Env = append(os.Environ(), "HN_TEST_REOPEN_ROOT="+filepath.Dir(w.Root), "HN_TEST_REOPEN_PROJECT="+w.ProjectID, "HN_TEST_REFERENCE_ID="+r.ID, "HN_TEST_GENERATION_ID="+g.ID)
	output, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("subprocess reopen: %v\n%s", e, output)
	}
}
func TestWorkspaceProcessHelper(t *testing.T) {
	parent := os.Getenv("HN_TEST_REOPEN_ROOT")
	if parent == "" {
		t.Skip("subprocess helper")
	}
	w, e := Open(parent, os.Getenv("HN_TEST_REOPEN_PROJECT"))
	okay(t, e)
	defer w.Close()
	okay(t, w.IntegrityCheck())
	r := get[ReferenceVersion](t, w, "reference_versions", os.Getenv("HN_TEST_REFERENCE_ID"))
	okay(t, w.verifyReference(r))
	g := get[Generation](t, w, "generations", os.Getenv("HN_TEST_GENERATION_ID"))
	if !g.Frozen {
		t.Fatal("freeze lost across process")
	}
}

// Compile-time assertion: ingestion works with a reader, without a Provider or HTTP API.
var _ io.Reader = (*brokenReader)(nil)
