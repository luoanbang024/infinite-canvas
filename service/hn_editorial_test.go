package service

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

func hnEditorialFixture(t *testing.T, root, project, node string) (HNShot, HNPreparedGeneration, HNArchiveFacts) {
	t.Helper()
	s, err := EnsureShotForSourceNode(root, project, node, "Shot")
	if err != nil {
		t.Fatal(err)
	}
	i := hnTestInput()
	i.NodeID = node
	i.ShotID = s.ShotID
	g, err := PrepareLocalGeneration(root, project, i)
	if err != nil {
		t.Fatal(err)
	}
	b := []byte("synthetic archive " + g.GenerationID)
	h := sha256.Sum256(b)
	r, err := ArchiveLocalResult(root, project, g.GenerationID, "", "video/mp4", bytes.NewReader(b), int64(len(b)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	return s, g, r
}

func hnEditorialSnapshot(t *testing.T, root, project string) map[string][]json.RawMessage {
	t.Helper()
	w, err := foundation.Open(root, project)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	out := map[string][]json.RawMessage{}
	for _, kind := range []string{"shots", "generations", "results", "archive_jobs", "task_bindings", "candidates", "sequence_items"} {
		out[kind], err = w.List(kind)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func hnEditorialTamper(t *testing.T, root, kind, id string, edit func(map[string]any)) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err = db.QueryRow("SELECT data FROM "+kind+" WHERE id=?", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	edit(data)
	b, _ := json.Marshal(data)
	if _, err = db.Exec("UPDATE "+kind+" SET data=? WHERE id=?", string(b), id); err != nil {
		t.Fatal(err)
	}
}

func TestHNEditorialStableCandidateLateSelectionAndReopen(t *testing.T) {
	root := t.TempDir()
	s, _, r := hnEditorialFixture(t, root, "test", "node")
	before := hnEditorialSnapshot(t, root, "test")
	a, err := EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "")
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "Changed")
	if err != nil || !reflect.DeepEqual(a, repeat) || a.Label != "Candidate" {
		t.Fatal("ensure changed existing metadata", err)
	}
	after := hnEditorialSnapshot(t, root, "test")
	if !reflect.DeepEqual(before["shots"], after["shots"]) {
		t.Fatal("auto selection")
	}
	if _, err = AddLocalSequenceItem(root, "test", "main", a.CandidateID); !errors.Is(err, ErrHNEditorialOwnership) {
		t.Fatal("unselected add accepted", err)
	}
	selected, err := SelectLocalCandidate(root, "test", s.ShotID, a.CandidateID)
	if err != nil || selected.SelectedCandidateID != a.CandidateID {
		t.Fatal(err)
	}
	i, err := AddLocalSequenceItem(root, "test", "main", a.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, rb := hnEditorialFixture(t, root, "test", "node")
	state := hnEditorialSnapshot(t, root, "test")
	b, err := EnsureCandidateForArchivedResult(root, "test", s.ShotID, rb.ResultID, "B")
	if err != nil {
		t.Fatal(err)
	}
	now := hnEditorialSnapshot(t, root, "test")
	if !reflect.DeepEqual(state["shots"], now["shots"]) || !reflect.DeepEqual(state["sequence_items"], now["sequence_items"]) {
		t.Fatal("late arrival changed selection/items")
	}
	if _, err = SelectLocalCandidate(root, "test", s.ShotID, b.CandidateID); err != nil {
		t.Fatal(err)
	}
	now = hnEditorialSnapshot(t, root, "test")
	if !reflect.DeepEqual(state["sequence_items"], now["sequence_items"]) {
		t.Fatal("selection rewrote existing sequence")
	}
	i2, err := AddLocalSequenceItem(root, "test", "main", b.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	i3, err := AddLocalSequenceItem(root, "test", "main", b.CandidateID)
	if err != nil || i3.SequenceItemID == i2.SequenceItemID || i.ResultID != r.ResultID || i2.ResultID != rb.ResultID {
		t.Fatal("placement identity", err)
	}
	now = hnEditorialSnapshot(t, root, "test")
	reopened := hnEditorialSnapshot(t, root, "test")
	if !reflect.DeepEqual(now, reopened) {
		t.Fatal("reopen mutated stable facts")
	}
	for _, kind := range []string{"generations", "results", "archive_jobs", "task_bindings"} {
		if !reflect.DeepEqual(state[kind], now[kind]) {
			t.Fatal("R7 mutated prerequisites", kind)
		}
	}
	var shot foundation.Shot
	json.Unmarshal(now["shots"][0], &shot)
	if shot.SelectedCandidateID != b.CandidateID {
		t.Fatal("selection not durable")
	}
	if len(now["candidates"]) != 2 || len(now["sequence_items"]) != 3 {
		t.Fatal("unexpected counts")
	}
}

func TestHNEditorialInputOwnershipAndArchiveEligibility(t *testing.T) {
	root := t.TempDir()
	s, _, r := hnEditorialFixture(t, root, "test", "node")
	for _, bad := range []string{"", "../escape", "CON", "https://invalid.example", strings.Repeat("x", 129)} {
		for position := 0; position < 3; position++ {
			ids := []string{"test", s.ShotID, r.ResultID}
			ids[position] = bad
			if _, err := EnsureCandidateForArchivedResult(root, ids[0], ids[1], ids[2], ""); !errors.Is(err, ErrHNEditorialInput) {
				t.Fatal("unsafe candidate ID", err)
			}
		}
		if _, err := SelectLocalCandidate(root, "test", s.ShotID, bad); !errors.Is(err, ErrHNEditorialInput) {
			t.Fatal("unsafe selection")
		}
		if _, err := AddLocalSequenceItem(root, "test", bad, "missing"); !errors.Is(err, ErrHNEditorialInput) {
			t.Fatal("unsafe sequence")
		}
	}
	for _, label := range []string{"C:\\fixture", "/fixture", "https://invalid.example", "token=fixture", "-----BEGIN fixture", strings.Repeat("镜", 86)} {
		if _, err := EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, label); err == nil {
			t.Fatal("unsafe label")
		}
	}
	foreign, _, fr := hnEditorialFixture(t, root, "foreign", "node")
	for _, ids := range [][2]string{{"missing", r.ResultID}, {s.ShotID, "missing"}, {foreign.ShotID, r.ResultID}, {s.ShotID, fr.ResultID}} {
		if _, err := EnsureCandidateForArchivedResult(root, "test", ids[0], ids[1], ""); err == nil {
			t.Fatal("foreign/missing ownership")
		}
	}
	other, _, otherResult := hnEditorialFixture(t, root, "test", "other")
	if _, err := EnsureCandidateForArchivedResult(root, "test", other.ShotID, r.ResultID, ""); err == nil {
		t.Fatal("wrong Shot Generation")
	}
	c, err := EnsureCandidateForArchivedResult(root, "test", other.ShotID, otherResult.ResultID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SelectLocalCandidate(root, "test", s.ShotID, c.CandidateID); err == nil {
		t.Fatal("foreign Candidate select")
	}
	for _, tc := range []struct {
		name, kind string
		edit       func(map[string]any)
	}{
		{"pending-job", "archive_jobs", func(d map[string]any) { d["status"] = "PENDING" }},
		{"hash-job", "archive_jobs", func(d map[string]any) { d["actualSha256"] = strings.Repeat("0", 64) }},
		{"bytes-job", "archive_jobs", func(d map[string]any) { d["actualBytes"] = float64(999) }},
		{"generation-unfrozen", "generations", func(d map[string]any) { d["frozen"] = false }},
		{"result-job-mismatch", "results", func(d map[string]any) { d["archiveJobId"] = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := t.TempDir()
			shot, g, res := hnEditorialFixture(t, rt, "test", "node")
			id := res.ArchiveJobID
			if tc.kind == "generations" {
				id = g.GenerationID
			}
			if tc.kind == "results" {
				id = res.ResultID
			}
			hnEditorialTamper(t, rt, tc.kind, id, tc.edit)
			if _, err := EnsureCandidateForArchivedResult(rt, "test", shot.ShotID, res.ResultID, ""); err == nil {
				t.Fatal("ineligible Result accepted")
			}
		})
	}
	// Open reconciliation must reject a formerly eligible Candidate after bytes disappear.
	c, err = EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "test", filepath.FromSlash(r.ArchivedRelativePath))); err != nil {
		t.Fatal(err)
	}
	if _, err = SelectLocalCandidate(root, "test", s.ShotID, c.CandidateID); err == nil {
		t.Fatal("unavailable select")
	}
	if _, err = AddLocalSequenceItem(root, "test", "main", c.CandidateID); err == nil {
		t.Fatal("unavailable add")
	}
}

func TestHNEditorialCandidateConflictAndSerializedEnsure(t *testing.T) {
	root := t.TempDir()
	s, _, r := hnEditorialFixture(t, root, "test", "node")
	var wg sync.WaitGroup
	out := make(chan HNCandidate, 6)
	fail := make(chan error, 6)
	for n := 0; n < 6; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "A")
			out <- c
			fail <- e
		}()
	}
	wg.Wait()
	close(out)
	close(fail)
	for err := range fail {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for c := range out {
		if id != "" && id != c.CandidateID {
			t.Fatal("concurrent duplicate")
		}
		id = c.CandidateID
	}
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.CreateCandidate(s.ShotID, r.ResultID, "duplicate")
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "A"); !errors.Is(err, ErrHNCandidateIdentity) {
		t.Fatal("duplicate identity not detected", err)
	}
}
