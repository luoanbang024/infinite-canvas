package service

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func hnExportFixture(t *testing.T, root string) (HNShot, []HNSequenceItem) {
	t.Helper()
	s, _, r := hnEditorialFixture(t, root, "test", "node")
	c, e := EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "A")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = SelectLocalCandidate(root, "test", s.ShotID, c.CandidateID); e != nil {
		t.Fatal(e)
	}
	a, e := AddLocalSequenceItem(root, "test", "main", c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	_, _, r = hnEditorialFixture(t, root, "test", "node")
	c, e = EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "B")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = SelectLocalCandidate(root, "test", s.ShotID, c.CandidateID); e != nil {
		t.Fatal(e)
	}
	b, e := AddLocalSequenceItem(root, "test", "main", c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	repeat, e := AddLocalSequenceItem(root, "test", "main", c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	items, e := ReorderLocalSequence(root, "test", "main", []string{repeat.SequenceItemID, a.SequenceItemID, b.SequenceItemID})
	if e != nil {
		t.Fatal(e)
	}
	return s, items
}
func hnExportFiles(t *testing.T, root string, m HNExport) map[string]string {
	t.Helper()
	out := map[string]string{}
	if e := filepath.Walk(filepath.Join(root, m.ProjectID, filepath.FromSlash(m.BundleRelativePath)), func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !i.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			h := sha256.Sum256(b)
			out[p] = hex.EncodeToString(h[:])
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	for _, i := range m.Items {
		b, e := os.ReadFile(filepath.Join(root, m.ProjectID, filepath.FromSlash(i.RelativePath)))
		if e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != i.SHA256 || int64(len(b)) != i.ByteLength || filepath.IsAbs(i.RelativePath) {
			t.Fatal("media facts")
		}
	}
	b, e := os.ReadFile(filepath.Join(root, m.ProjectID, filepath.FromSlash(m.ManifestCSVRelativePath)))
	if e != nil {
		t.Fatal(e)
	}
	rows, e := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil || len(rows) != m.ItemCount+1 {
		t.Fatal("CSV", e)
	}
	for n, i := range m.Items {
		if rows[n+1][0] != m.ExportID || rows[n+1][2] != i.SequenceItemID || rows[n+1][6] != i.RelativePath || rows[n+1][7] != i.SHA256 {
			t.Fatal("CSV order/facts")
		}
	}
	return out
}

func TestHNExportStablePlacementsFreshIdentityImmutableReopen(t *testing.T) {
	root := t.TempDir()
	s, items := hnExportFixture(t, root)
	before := hnEditorialSnapshot(t, root, "test")
	first, e := ExportLocalSequence(root, "test", "main")
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, hnEditorialSnapshot(t, root, "test")) {
		t.Fatal("record mutation")
	}
	for n, i := range first.Items {
		if i.SequenceItemID != items[n].SequenceItemID || i.CandidateID != items[n].CandidateID || i.ResultID != items[n].ResultID {
			t.Fatal("fallback/order")
		}
	}
	original := hnExportFiles(t, root, first)
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	e = w.SelectCandidate(s.ShotID, "")
	w.Close()
	if e != nil {
		t.Fatal(e)
	}
	order := []string{items[1].SequenceItemID, items[2].SequenceItemID, items[0].SequenceItemID}
	if _, e = ReorderLocalSequence(root, "test", "main", order); e != nil {
		t.Fatal(e)
	}
	before = hnEditorialSnapshot(t, root, "test")
	second, e := ExportLocalSequence(root, "test", "main")
	if e != nil {
		t.Fatal(e)
	}
	if second.ExportID == first.ExportID || second.ItemCount != 3 {
		t.Fatal("fresh identity")
	}
	for n, i := range second.Items {
		if i.SequenceItemID != order[n] {
			t.Fatal("new order")
		}
	}
	if !reflect.DeepEqual(original, hnExportFiles(t, root, first)) || !reflect.DeepEqual(before, hnEditorialSnapshot(t, root, "test")) {
		t.Fatal("old bundle/record changed")
	}
	hnExportFiles(t, root, second)
	raw, e := json.Marshal(second)
	if e != nil || strings.Contains(string(raw), root) || strings.Contains(string(raw), "generated/") {
		t.Fatal("absolute/source paths")
	}
}

func TestHNExportRejectsUnsafeEmptyOwnershipAndBrokenArchive(t *testing.T) {
	for _, id := range []string{"", "../outside", "CON", "C:drive", "a/b"} {
		for _, p := range [][2]string{{id, "main"}, {"test", id}} {
			if _, e := ExportLocalSequence(t.TempDir(), p[0], p[1]); !errors.Is(e, ErrHNExportInput) {
				t.Fatal("unsafe", e)
			}
		}
	}
	if _, e := ExportLocalSequence("", "test", "main"); !errors.Is(e, ErrHNExportInput) {
		t.Fatal("disabled")
	}
	if _, e := ExportLocalSequence(t.TempDir(), "test", "empty"); !errors.Is(e, ErrHNExportIncomplete) {
		t.Fatal("empty")
	}
	for _, fault := range []string{"shot", "result", "generation", "unarchived", "job", "missing-file", "tampered-file"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			_, items := hnExportFixture(t, root)
			i := items[0]
			switch fault {
			case "shot":
				hnEditorialTamper(t, root, "candidates", i.CandidateID, func(d map[string]any) { d["shotId"] = "missing" })
			case "result":
				hnEditorialTamper(t, root, "candidates", i.CandidateID, func(d map[string]any) { d["resultId"] = "missing" })
			case "generation":
				hnEditorialTamper(t, root, "candidates", i.CandidateID, func(d map[string]any) { d["generationId"] = "missing" })
			case "unarchived":
				// A completed receipt is authoritative on Open; use a genuinely
				// unarchived Result instead of relabelling a completed archive.
				w, e := foundation.Open(root, "test")
				if e != nil {
					t.Fatal(e)
				}
				var c foundation.Candidate
				if e = hnReadRecord(w, "candidates", i.CandidateID, &c); e != nil {
					t.Fatal(e)
				}
				r, e := w.CreateResult(foundation.Result{GenerationID: c.GenerationID, ResultKind: "video"})
				w.Close()
				if e != nil {
					t.Fatal(e)
				}
				hnEditorialTamper(t, root, "candidates", i.CandidateID, func(d map[string]any) { d["resultId"] = r.ID })
				hnEditorialTamper(t, root, "sequence_items", i.SequenceItemID, func(d map[string]any) { d["resultId"] = r.ID })
			case "job":
				hnEditorialTamper(t, root, "results", i.ResultID, func(d map[string]any) { d["archiveJobId"] = "missing" })
			default:
				w, e := foundation.Open(root, "test")
				if e != nil {
					t.Fatal(e)
				}
				var r foundation.Result
				e = hnReadRecord(w, "results", i.ResultID, &r)
				w.Close()
				if e != nil {
					t.Fatal(e)
				}
				p := filepath.Join(root, "test", filepath.FromSlash(r.ArchivedRelativePath))
				if fault == "missing-file" {
					e = os.Remove(p)
				} else {
					e = os.WriteFile(p, []byte("bad"), 0600)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
			if _, e := ExportLocalSequence(root, "test", "main"); !errors.Is(e, ErrHNExportIncomplete) {
				t.Fatal("ineligible export accepted", e)
			}
		})
	}
}
