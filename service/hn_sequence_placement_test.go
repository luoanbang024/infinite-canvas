package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

func TestR28ServiceMetadataOnlyAndSelectionReplay(t *testing.T) {
	root := t.TempDir()
	s, g, r := hnEditorialFixture(t, root, "r28", "node")
	c, e := EnsureCandidateForArchivedResult(root, "r28", s.ShotID, r.ResultID, "Local Archive")
	if e != nil {
		t.Fatal(e)
	}
	_, e = SelectLocalCandidate(root, "r28", s.ShotID, c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	w, e := foundation.OpenExisting(root, "r28", true)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := w.Read("results", r.ResultID)
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	var result foundation.Result
	json.Unmarshal(raw, &result)
	// No physical media remains. Ordinary Open would reconcile this failure.
	if e = os.Remove(filepath.Join(root, "r28", filepath.FromSlash(result.ArchivedRelativePath))); e != nil {
		t.Fatal(e)
	}
	p := foundation.PlacementCommand{ProtocolVersion: 1, PlacementIntentID: uuid.NewString(), PlacementOwner: foundation.PlacementOwner{CandidateID: c.CandidateID, ShotID: s.ShotID, GenerationID: g.GenerationID, ResultID: r.ResultID, ArchiveJobID: r.ArchiveJobID, PreparedFrozenHash: g.FrozenHash}}
	a, e := ExecuteLocalPlacement(root, "r28", p)
	if e != nil || a.Outcome != "COMMITTED" {
		t.Fatal(e, a)
	}
	snapshot, e := ReadLocalMainSequence(root, "r28")
	if e != nil || len(snapshot.Items) != 1 {
		t.Fatal(e, snapshot)
	}
	again, e := LookupLocalPlacement(root, "r28", p.PlacementIntentID)
	if e != nil || !reflect.DeepEqual(a, again) {
		t.Fatal(e, again)
	}
	w, e = foundation.OpenExisting(root, "r28", true)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	after, e := w.Read("results", r.ResultID)
	if e != nil || string(after) != string(raw) {
		t.Fatal("result was reconciled", e)
	}
}
