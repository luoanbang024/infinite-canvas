package foundation

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func r33Fixture(t *testing.T) (*Workspace, ExportCommand) {
	t.Helper()
	w, p := placementFixture(t)
	for n := 0; n < 3; n++ {
		_, e := w.AddSequenceItem("main", p.CandidateID)
		okay(t, e)
	}
	_, e := w.InitializeExportProtocol()
	okay(t, e)
	okay(t, w.SelectCandidate(p.ShotID, ""))
	return w, r33Command(t, w)
}
func r33Command(t *testing.T, w *Workspace) ExportCommand {
	t.Helper()
	s, e := w.ReadReorderSnapshot()
	okay(t, e)
	ids := []string{}
	for _, i := range s.Items {
		ids = append(ids, i.SequenceItemID)
	}
	return ExportCommand{1, uuid.NewString(), s.SequenceRevision, ids, ExportFormat}
}
func r33Evidence(t *testing.T, name string, v any) {
	t.Helper()
	b, e := json.MarshalIndent(v, "", "  ")
	okay(t, e)
	if root := os.Getenv("HN_R33_EVIDENCE"); root != "" {
		okay(t, os.MkdirAll(root, 0700))
		okay(t, os.WriteFile(filepath.Join(root, name+".json"), b, 0600))
	}
}
func r33Job(t *testing.T, w *Workspace, c ExportCommand) *exportJob {
	t.Helper()
	var j *exportJob
	okay(t, w.placementRead(func(q placementConnection) error { var e error; j, e = w.exportReadJob(q, c.ExportIntentID); return e }))
	return j
}
func r33Stop(t *testing.T, w *Workspace, c ExportCommand, point string) {
	t.Helper()
	stopped := false
	func() {
		defer func() {
			if recover() != nil {
				stopped = true
			}
		}()
		_, e := w.executeExportCommand(c, func(p string) {
			if p == point {
				panic("synthetic stop")
			}
		})
		okay(t, e)
	}()
	if !stopped {
		t.Fatal("barrier not reached", point)
	}
}
func TestR33InitializationReadonlyAndExactSchema(t *testing.T) {
	w := newWorkspace(t)
	intent := uuid.NewString()
	_, e := w.LookupExportCommand(intent)
	if !errors.Is(e, ErrExportProtocol) {
		t.Fatal(e)
	}
	_, e = w.InitializeExportProtocol()
	if !errors.Is(e, ErrReorderProtocol) {
		t.Fatal(e)
	}
	var count int
	okay(t, w.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name GLOB 'export_*'").Scan(&count))
	if count != 0 {
		t.Fatal("lazy schema install")
	}
	_, e = w.InitializeReorderProtocol()
	okay(t, e)
	state, e := w.InitializeExportProtocol()
	okay(t, e)
	again, e := w.InitializeExportProtocol()
	okay(t, e)
	if !reflect.DeepEqual(state, again) {
		t.Fatal("initialize changed")
	}
	for _, n := range []string{"metadata/export-locks", "metadata/export-staging", "exports/commands-v1"} {
		p, _ := w.Resolve(n)
		if _, e = os.Stat(p); !os.IsNotExist(e) {
			t.Fatal("init touched bundle dirs", n, e)
		}
	}
	rows, e := w.db.Query("SELECT name,sql FROM sqlite_master WHERE name GLOB 'export_*' ORDER BY name")
	okay(t, e)
	schema := []map[string]string{}
	for rows.Next() {
		var n, s string
		okay(t, rows.Scan(&n, &s))
		schema = append(schema, map[string]string{"name": n, "sql": s})
	}
	okay(t, rows.Close())
	if len(schema) != len(exportSchema) {
		t.Fatal(schema)
	}
	ro, e := OpenExisting(filepath.Dir(w.Root), w.ProjectID, true)
	okay(t, e)
	defer ro.Close()
	s, e := ro.LookupExportCommand(intent)
	okay(t, e)
	if s.Outcome != "NOT_OBSERVED" {
		t.Fatal(s)
	}
	_, e = ro.InitializeExportProtocol()
	reject(t, e)
	r33Evidence(t, "schema-initialization", map[string]any{"schema": schema, "repeat": again, "readonly": s, "noMediaOrDirs": true})
	for _, sqlText := range []string{"CREATE TABLE export_unknown(x)", "DROP TRIGGER export_receipts_no_delete"} {
		t.Run(sqlText, func(t *testing.T) {
			x, _ := r33Fixture(t)
			_, e := x.db.Exec(sqlText)
			okay(t, e)
			_, e = x.InitializeExportProtocol()
			if !errors.Is(e, ErrExportIntegrity) {
				t.Fatal(e)
			}
			_, e = x.LookupExportCommand(intent)
			if !errors.Is(e, ErrExportIntegrity) {
				t.Fatal(e)
			}
		})
	}
}
func TestR33CommitReplayHistoricalAndR8Compatibility(t *testing.T) {
	w, c := r33Fixture(t)
	revision := r30Revision(t, w)
	before := r30Items(t, w)
	s, e := w.ExecuteExportCommand(c)
	okay(t, e)
	if s.Outcome != "COMMITTED" || s.AttemptCount != 1 || s.Receipt == nil {
		t.Fatal(s)
	}
	r30Invariant(t, w, before)
	if r30Revision(t, w) != revision {
		t.Fatal("export advanced revision")
	}
	j := r33Job(t, w, c)
	r33CaptureBundle(t, w, j, exportFinalRelative(*j.ExportID), "final")
	jb, cb, mb, h, e := w.exportBundleBytes(j)
	okay(t, e)
	var m ExportManifest
	okay(t, json.Unmarshal(jb, &m))
	if len(m.Items) != 3 || m.SchemaVersion != SchemaVersion || m.Items[0].ResultID != m.Items[1].ResultID || m.Items[0].RelativePath == m.Items[1].RelativePath {
		t.Fatal(m)
	}
	if !strings.HasPrefix(string(cb), "exportId,sequenceIndex,sequenceItemId,shotId,candidateId,resultId,relativePath,sha256,byteLength,duration\n") {
		t.Fatal("CSV format")
	}
	health, e := w.VerifyExportBundle(c.ExportIntentID)
	okay(t, e)
	if health.Health != "VERIFIED" {
		t.Fatal(health)
	}
	repeated, e := w.ExecuteExportCommand(c)
	okay(t, e)
	if !reflect.DeepEqual(s, repeated) {
		t.Fatal("terminal repeat drift")
	}
	changed := c
	changed.ExpectedRevision = "0"
	_, e = w.ExecuteExportCommand(changed)
	if !errors.Is(e, ErrExportIntent) {
		t.Fatal(e)
	}
	ids := append([]string{}, c.OrderedSequenceItemIDs...)
	ids[0], ids[2] = ids[2], ids[0]
	_, e = w.ExecuteReorderCommand(r30Command(revision, ids...))
	okay(t, e)
	repeated, e = w.ExecuteExportCommand(c)
	okay(t, e)
	if !reflect.DeepEqual(s, repeated) {
		t.Fatal("current CAS precedes replay")
	}
	fresh := r33Command(t, w)
	second, e := w.ExecuteExportCommand(fresh)
	okay(t, e)
	if *second.ExportID == *s.ExportID {
		t.Fatal("different intent dedup")
	}
	source, _ := w.Resolve(j.Snapshot.Items[0].SourceRelativePath)
	okay(t, os.Remove(source))
	repeated, e = w.ExecuteExportCommand(c)
	okay(t, e)
	if !reflect.DeepEqual(s, repeated) {
		t.Fatal("terminal read media")
	}
	for _, sqlText := range []string{"UPDATE export_jobs SET canonical_request='{}'", "DELETE FROM export_jobs", "UPDATE export_receipts SET receipt='{}'", "DELETE FROM export_receipts", "UPDATE export_jobs SET phase='COPYING'"} {
		_, e = w.db.Exec(sqlText)
		reject(t, e)
	}
	r33Evidence(t, "commit-replay-format", map[string]any{"receipt": s, "second": second, "health": health, "json": string(jb), "csv": string(cb), "marker": string(mb), "hashes": h, "historicalReplayAfterReorder": repeated, "revisionUnchangedByExport": revision})
}
func TestR33DeterministicRejectionsAndMalformedNoJob(t *testing.T) {
	for _, kind := range []string{"revision", "vector", "empty", "capacity", "owner"} {
		t.Run(kind, func(t *testing.T) {
			w, c := r33Fixture(t)
			class := ""
			switch kind {
			case "revision":
				c.ExpectedRevision = "0"
				class = "SEQUENCE_REVISION_CONFLICT"
			case "vector":
				c.OrderedSequenceItemIDs = []string{}
				class = "SEQUENCE_VECTOR_CONFLICT"
			case "empty":
				for _, id := range c.OrderedSequenceItemIDs {
					okay(t, w.Delete("sequence_items", id))
				}
				c = r33Command(t, w)
				class = "EMPTY_SEQUENCE"
			case "capacity":
				j := get[Candidate](t, w, "candidates", get[SequenceItem](t, w, "sequence_items", c.OrderedSequenceItemIDs[0]).CandidateID)
				okay(t, w.SelectCandidate(j.ShotID, j.ID))
				for n := 3; n < 257; n++ {
					_, e := w.AddSequenceItem("main", j.ID)
					okay(t, e)
				}
				c.ExpectedRevision = r30RevisionFromDB(t, w)
				class = "READ_CAPACITY_EXCEEDED"
			case "owner":
				i := get[SequenceItem](t, w, "sequence_items", c.OrderedSequenceItemIDs[0])
				_, e := w.db.Exec("UPDATE candidates SET data=json_set(data,'$.generationId','wrong') WHERE id=?", i.CandidateID)
				okay(t, e)
				class = "EXPORT_OWNERSHIP_REJECTED"
			}
			s, e := w.ExecuteExportCommand(c)
			okay(t, e)
			if s.Outcome != "REJECTED" || s.Receipt == nil || *s.ErrorClass != class || s.ExportID != nil || s.AttemptCount != 0 {
				t.Fatal(s)
			}
			repeat, e := w.ExecuteExportCommand(c)
			okay(t, e)
			if !reflect.DeepEqual(s, repeat) {
				t.Fatal("reject replay")
			}
			p, _ := w.Resolve("metadata/export-staging")
			if _, e = os.Stat(p); !os.IsNotExist(e) {
				t.Fatal("reject staged")
			}
			r33Evidence(t, "reject-"+kind, s)
		})
	}
	w, c := r33Fixture(t)
	for _, change := range []func(*ExportCommand){func(c *ExportCommand) { c.ExpectedRevision = "01" }, func(c *ExportCommand) { c.OrderedSequenceItemIDs = nil }, func(c *ExportCommand) {
		c.OrderedSequenceItemIDs = append(c.OrderedSequenceItemIDs, c.OrderedSequenceItemIDs[0])
	}, func(c *ExportCommand) { c.ProtocolVersion = 2 }, func(c *ExportCommand) { c.Format = "wrong" }, func(c *ExportCommand) { c.ExportIntentID = "bad" }} {
		x := c
		x.OrderedSequenceItemIDs = append([]string{}, c.OrderedSequenceItemIDs...)
		change(&x)
		_, e := w.ExecuteExportCommand(x)
		if !errors.Is(e, ErrExportInput) {
			t.Fatal(x, e)
		}
	}
	var count int
	okay(t, w.db.QueryRow("SELECT count(*) FROM export_jobs").Scan(&count))
	if count != 0 {
		t.Fatal("malformed job")
	}
}
func r30RevisionFromDB(t *testing.T, w *Workspace) string {
	t.Helper()
	var n string
	okay(t, w.db.QueryRow("SELECT CAST(revision AS TEXT) FROM sequence_state").Scan(&n))
	return n
}
func TestR33BoundedRetainedAttemptsAndValidStageRecovery(t *testing.T) {
	w, c := r33Fixture(t)
	var id string
	attempts := []string{}
	for n := 1; n <= 3; n++ {
		r33Stop(t, w, c, "PARTIAL_MEDIA")
		j := r33Job(t, w, c)
		if j.AttemptCount != n {
			t.Fatal(j)
		}
		if n == 1 {
			id = *j.ExportID
		} else if *j.ExportID != id {
			t.Fatal("new export ID")
		}
		attempts = append(attempts, *j.AttemptID)
		p, _ := w.Resolve(exportStageRelative(j) + "/media")
		files, e := os.ReadDir(p)
		okay(t, e)
		if len(files) != 1 {
			t.Fatal(files)
		}
		info, e := files[0].Info()
		okay(t, e)
		if info.Size() != 1 {
			t.Fatal("partial not bounded")
		}
	}
	s, e := w.ExecuteExportCommand(c)
	okay(t, e)
	if s.Outcome != "RECOVERY_BLOCKED" || s.AttemptCount != 3 || *s.ErrorClass != "EXPORT_ATTEMPT_EXHAUSTED" {
		t.Fatal(s)
	}
	for _, a := range attempts {
		p, _ := w.Resolve("metadata/export-staging/" + id + "/" + a)
		if _, e = os.Stat(p); e != nil {
			t.Fatal("partial deleted", e)
		}
	}
	r33Evidence(t, "attempt-bound", map[string]any{"status": s, "retainedAttempts": attempts, "partialMediaBytesEach": 1, "maximum": 3})
	t.Run("complete-stage", func(t *testing.T) {
		w, c := r33Fixture(t)
		r33Stop(t, w, c, "STAGE_COMPLETE")
		j := r33Job(t, w, c)
		r33CaptureBundle(t, w, j, exportStageRelative(j), "stage")
		id := *j.ExportID
		a := *j.AttemptID
		s, e := w.ExecuteExportCommand(c)
		okay(t, e)
		if s.Outcome != "COMMITTED" || s.AttemptCount != 1 || *s.ExportID != id || *r33Job(t, w, c).AttemptID != a {
			t.Fatal("complete stage not recovered", s)
		}
	})
}
func TestR33SourceIntegrityFinalConflictAndReadonlyHealth(t *testing.T) {
	for _, kind := range []string{"missing", "short", "long", "hash", "sidecar"} {
		t.Run(kind, func(t *testing.T) {
			w, c := r33Fixture(t)
			r33Stop(t, w, c, "RESERVED")
			j := r33Job(t, w, c)
			i := j.Snapshot.Items[0]
			p, _ := w.Resolve(i.SourceRelativePath)
			switch kind {
			case "missing":
				okay(t, os.Remove(p))
			case "short":
				okay(t, os.WriteFile(p, []byte("x"), 0600))
			case "long":
				okay(t, os.WriteFile(p, bytes.Repeat([]byte("x"), int(i.ByteLength)+1), 0600))
			case "hash":
				okay(t, os.WriteFile(p, bytes.Repeat([]byte("x"), int(i.ByteLength)), 0600))
			case "sidecar":
				okay(t, os.WriteFile(p+".json", []byte(`{}`), 0600))
			}
			s, e := w.ExecuteExportCommand(c)
			okay(t, e)
			if s.Outcome != "FAILED" || s.Receipt == nil || *s.ErrorClass != "EXPORT_SOURCE_INTEGRITY" || !w.exportFinalAbsent(r33Job(t, w, c)) {
				t.Fatal(s)
			}
			r33Evidence(t, "source-"+kind, s)
		})
	}
	t.Run("foreign-final", func(t *testing.T) {
		w, c := r33Fixture(t)
		r33Stop(t, w, c, "RESERVED")
		j := r33Job(t, w, c)
		p, _ := w.Resolve(exportFinalRelative(*j.ExportID))
		okay(t, os.MkdirAll(p, 0700))
		okay(t, os.WriteFile(filepath.Join(p, "foreign"), []byte("synthetic foreign"), 0600))
		s, e := w.ExecuteExportCommand(c)
		okay(t, e)
		if s.Outcome != "RECOVERY_BLOCKED" {
			t.Fatal(s)
		}
		b, e := os.ReadFile(filepath.Join(p, "foreign"))
		okay(t, e)
		if string(b) != "synthetic foreign" {
			t.Fatal("overwritten")
		}
	})
	for _, kind := range []string{"media-missing", "media-corrupt", "json", "csv", "marker"} {
		t.Run(kind, func(t *testing.T) {
			w, c := r33Fixture(t)
			s, e := w.ExecuteExportCommand(c)
			okay(t, e)
			j := r33Job(t, w, c)
			relative := j.Snapshot.Items[0].RelativePath
			switch kind {
			case "json":
				relative = *s.Receipt.ManifestJSONRelativePath
			case "csv":
				relative = *s.Receipt.ManifestCSVRelativePath
			case "marker":
				relative = *s.Receipt.BundleRelativePath + "/export-command.json"
			}
			p, _ := w.Resolve(relative)
			if kind == "media-missing" {
				okay(t, os.Remove(p))
			} else {
				okay(t, os.WriteFile(p, []byte("synthetic corrupt"), 0600))
			}
			before := *r33Job(t, w, c)
			h, e := w.VerifyExportBundle(c.ExportIntentID)
			okay(t, e)
			expected := "CORRUPT"
			if kind == "media-missing" {
				expected = "MISSING"
			}
			if h.Health != expected {
				t.Fatal(h)
			}
			after := *r33Job(t, w, c)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("verify mutated job")
			}
			same, e := w.ExecuteExportCommand(c)
			okay(t, e)
			if !reflect.DeepEqual(s, same) {
				t.Fatal("historical receipt rebuilt")
			}
			r33Evidence(t, "health-"+kind, map[string]any{"health": h, "receiptUnchanged": true, "noRebuild": true})
		})
	}
}

func r33CaptureBundle(t *testing.T, w *Workspace, j *exportJob, base, name string) {
	t.Helper()
	root := os.Getenv("HN_R33_EVIDENCE")
	if root == "" {
		return
	}
	source, e := w.Resolve(base)
	okay(t, e)
	target := filepath.Join(root, "bundle-fixtures", name)
	okay(t, filepath.WalkDir(source, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0700)
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		return os.WriteFile(dst, b, 0600)
	}))
	r33Evidence(t, "bundle-"+name+"-job", j)
}
func TestR33DelayedObservationAndAcceptedFrozenOwners(t *testing.T) {
	w, c := r33Fixture(t)
	before, e := w.LookupExportCommand(c.ExportIntentID)
	okay(t, e)
	if before.Outcome != "NOT_OBSERVED" {
		t.Fatal(before)
	}
	r33Stop(t, w, c, "RESERVED")
	j := r33Job(t, w, c)
	// An accepted export owns its minimal immutable snapshot, not later mutable rows.
	_, e = w.db.Exec("DELETE FROM sequence_items")
	okay(t, e)
	for _, table := range []string{"candidates", "shots"} {
		_, e = w.db.Exec("UPDATE " + table + " SET data='{}'")
		okay(t, e)
	}
	s, e := w.ExecuteExportCommand(c)
	okay(t, e)
	if s.Outcome != "COMMITTED" || !reflect.DeepEqual(s.Receipt.OrderedSequenceItemIDs, c.OrderedSequenceItemIDs) || *s.ExportID != *j.ExportID {
		t.Fatal(s)
	}
	r33Evidence(t, "delayed-observation-frozen-owners", map[string]any{"before": before, "after": s, "notObservedIsSnapshotOnly": true, "acceptedSnapshotSurvivesLaterOwnerRecordDamage": true})
}
func TestR33UnsafeReparseAndFinalAmbiguity(t *testing.T) {
	t.Run("source-directory-link", func(t *testing.T) {
		w, c := r33Fixture(t)
		r33Stop(t, w, c, "RESERVED")
		j := r33Job(t, w, c)
		source, e := w.Resolve(j.Snapshot.Items[0].SourceRelativePath)
		okay(t, e)
		external := t.TempDir()
		parent := filepath.Dir(source)
		for _, p := range []string{source, source + ".json"} {
			b, e := os.ReadFile(p)
			okay(t, e)
			okay(t, os.WriteFile(filepath.Join(external, filepath.Base(p)), b, 0600))
			okay(t, os.Remove(p))
		}
		okay(t, os.Remove(parent))
		e = os.Symlink(external, parent)
		linkKind := "symlink"
		if e != nil && runtime.GOOS == "windows" {
			output, x := exec.Command("cmd", "/c", "mklink", "/J", parent, external).CombinedOutput()
			if x != nil {
				t.Fatalf("disposable junction: %v %s", x, output)
			}
			linkKind = "Windows directory junction"
		} else {
			okay(t, e)
		}
		t.Cleanup(func() { _ = os.Remove(parent) })
		if _, e = w.Resolve(j.Snapshot.Items[0].SourceRelativePath); e == nil {
			t.Fatal("linked source accepted")
		}
		s, e := w.ExecuteExportCommand(c)
		okay(t, e)
		if s.Outcome != "FAILED" || *s.ErrorClass != "EXPORT_SOURCE_INTEGRITY" {
			t.Fatal(s)
		}
		r33Evidence(t, "source-reparse", map[string]any{"linkKind": linkKind, "actualPathEscapeRejected": true, "status": s})
	})
	t.Run("published-marker-corrupt-before-receipt", func(t *testing.T) {
		w, c := r33Fixture(t)
		r33Stop(t, w, c, "FINAL_RENAMED")
		j := r33Job(t, w, c)
		p, e := w.Resolve(exportFinalRelative(*j.ExportID) + "/export-command.json")
		okay(t, e)
		okay(t, os.WriteFile(p, []byte("synthetic foreign marker"), 0600))
		s, e := w.ExecuteExportCommand(c)
		okay(t, e)
		if s.Outcome != "RECOVERY_BLOCKED" || s.Receipt != nil {
			t.Fatal(s)
		}
		b, e := os.ReadFile(p)
		okay(t, e)
		if string(b) != "synthetic foreign marker" {
			t.Fatal("unexpected final overwritten")
		}
		r33Evidence(t, "published-invalid-marker", s)
	})
	t.Run("committed-whole-bundle-missing", func(t *testing.T) {
		w, c := r33Fixture(t)
		s, e := w.ExecuteExportCommand(c)
		okay(t, e)
		p, e := w.Resolve(*s.Receipt.BundleRelativePath)
		okay(t, e)
		// Both paths are fixed descendants of this disposable workspace.
		if !strings.HasPrefix(p, w.Root+string(filepath.Separator)) {
			t.Fatal("unsafe fixture move")
		}
		okay(t, os.Rename(p, p+"-removed"))
		health, e := w.VerifyExportBundle(c.ExportIntentID)
		okay(t, e)
		if health.Health != "MISSING" {
			t.Fatal(health)
		}
		same, e := w.ExecuteExportCommand(c)
		okay(t, e)
		if !reflect.DeepEqual(s, same) {
			t.Fatal("historical drift")
		}
		if _, e = os.Stat(p); !os.IsNotExist(e) {
			t.Fatal("historical rebuilt")
		}
		r33Evidence(t, "health-whole-bundle-missing", map[string]any{"health": health, "historicalReceipt": same, "rebuild": false})
	})
}
