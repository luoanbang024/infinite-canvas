package foundation

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExportPlacementIndependentOfCurrentSelection(t *testing.T) {
	w := newWorkspace(t)
	s := makeShot(t, w, "same shot")
	var candidates []Candidate
	for _, label := range []string{"A", "B"} {
		g := makeGeneration(t, w, s.ID, label)
		r := makeResult(t, w, g)
		archive(t, w, r, []byte("synthetic-"+label))
		c, e := w.CreateCandidate(s.ID, r.ID, label)
		okay(t, e)
		candidates = append(candidates, c)
	}
	okay(t, w.SelectCandidate(s.ID, candidates[0].ID))
	a, e := w.AddSequenceItem("main", candidates[0].ID)
	okay(t, e)
	okay(t, w.SelectCandidate(s.ID, candidates[1].ID))
	b, e := w.AddSequenceItem("main", candidates[1].ID)
	okay(t, e)
	repeat, e := w.AddSequenceItem("main", candidates[1].ID)
	okay(t, e)
	order := []string{repeat.ID, a.ID, b.ID}
	okay(t, w.Reorder("main", order))
	snapshot := func() map[string][]byte {
		result := map[string][]byte{}
		for _, kind := range []string{"shots", "generations", "results", "archive_jobs", "task_bindings", "candidates", "sequence_items"} {
			rows, e := w.List(kind)
			okay(t, e)
			for n, row := range rows {
				result[kind+string(rune(n))] = row
			}
		}
		return result
	}
	before := snapshot()
	first, e := w.Export("main")
	okay(t, e)
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("export mutated records")
	}
	path, e := w.Resolve("exports/" + first.ExportID + "/ordered-manifest.json")
	okay(t, e)
	old, e := os.ReadFile(path)
	okay(t, e)
	for n, i := range first.Items {
		if i.SequenceItemID != order[n] || i.SequenceIndex != n+1 {
			t.Fatal("order")
		}
		okay(t, w.verifyFile(i.RelativePath, i.SHA256, i.ByteLength))
	}
	if first.Items[1].CandidateID != candidates[0].ID || first.Items[0].ResultID != first.Items[2].ResultID {
		t.Fatal("fallback/repeated placement")
	}
	okay(t, w.SelectCandidate(s.ID, ""))
	before = snapshot()
	second, e := w.Export("main")
	okay(t, e)
	if second.ExportID == first.ExportID || !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("identity/record mutation")
	}
	for n, i := range second.Items {
		if i.CandidateID != first.Items[n].CandidateID {
			t.Fatal("selection fallback")
		}
	}
	okay(t, w.Reorder("main", []string{a.ID, b.ID, repeat.ID}))
	third, e := w.Export("main")
	okay(t, e)
	if third.Items[0].SequenceItemID != a.ID || third.ExportID == first.ExportID {
		t.Fatal("new order/identity")
	}
	now, e := os.ReadFile(path)
	okay(t, e)
	if !bytes.Equal(old, now) {
		t.Fatal("old manifest changed")
	}
	okay(t, w.Close())
	reopened, e := Open(filepath.Dir(w.Root), w.ProjectID)
	okay(t, e)
	defer reopened.Close()
	for _, i := range first.Items {
		okay(t, reopened.verifyFile(i.RelativePath, i.SHA256, i.ByteLength))
	}
	// Missing archive bytes remain a rejection, even with valid placements.
	r := get[Result](t, reopened, "results", candidates[0].ResultID)
	media, e := reopened.Resolve(r.ArchivedRelativePath)
	okay(t, e)
	okay(t, os.Remove(media))
	_, e = reopened.Export("main")
	reject(t, e)
}

func TestExportPlacementStillRequiresOwnershipAndArchiveFacts(t *testing.T) {
	for _, fault := range []string{"candidate-shot", "candidate-result", "candidate-generation", "result-status", "receipt", "job-hash", "job-bytes", "result-hash", "result-bytes", "file"} {
		t.Run(fault, func(t *testing.T) {
			w := newWorkspace(t)
			s := makeShot(t, w, "shot")
			g := makeGeneration(t, w, s.ID, "test")
			r := makeResult(t, w, g)
			archive(t, w, r, []byte("synthetic"))
			r = get[Result](t, w, "results", r.ID)
			c, e := w.CreateCandidate(s.ID, r.ID, "A")
			okay(t, e)
			okay(t, w.SelectCandidate(s.ID, c.ID))
			_, e = w.AddSequenceItem("main", c.ID)
			okay(t, e)
			okay(t, w.SelectCandidate(s.ID, ""))
			switch fault {
			case "candidate-shot":
				c.ShotID = "other"
				okay(t, put(w.db, "candidates", c.ID, c))
			case "candidate-result":
				c.ResultID = "other"
				okay(t, put(w.db, "candidates", c.ID, c))
			case "candidate-generation":
				c.GenerationID = "other"
				okay(t, put(w.db, "candidates", c.ID, c))
			case "result-status":
				r.Status = "RECEIVED"
				okay(t, put(w.db, "results", r.ID, r))
			case "result-hash":
				r.SHA256 = digest([]byte("other"))
				okay(t, put(w.db, "results", r.ID, r))
			case "result-bytes":
				r.ByteLength++
				okay(t, put(w.db, "results", r.ID, r))
			case "file":
				p, e := w.Resolve(r.ArchivedRelativePath)
				okay(t, e)
				okay(t, os.WriteFile(p, []byte("tampered"), 0600))
			case "receipt":
				p, e := w.Resolve(r.ArchivedRelativePath + ".json")
				okay(t, e)
				okay(t, os.Remove(p))
			default:
				j := get[ArchiveJob](t, w, "archive_jobs", r.ArchiveJobID)
				switch fault {
				case "job-hash":
					j.ActualSHA256 = digest([]byte("other"))
				case "job-bytes":
					j.ActualBytes++
				}
				okay(t, put(w.db, "archive_jobs", j.ID, j))
			}
			_, e = w.Export("main")
			reject(t, e)
			p, e := w.Resolve("exports")
			okay(t, e)
			entries, e := os.ReadDir(p)
			okay(t, e)
			for _, entry := range entries {
				_, e = os.Stat(filepath.Join(p, entry.Name(), "ordered-manifest.json"))
				if e == nil {
					t.Fatal("failed export has completion marker")
				}
			}
		})
	}
}
