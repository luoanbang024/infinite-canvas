package service

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestR30ServiceExistingStoreNoMediaAndLegacyShape(t *testing.T) {
	missing := t.TempDir()
	_, e := InitializeLocalReorder(missing, "test")
	if !errors.Is(e, foundation.ErrReorderUnavailable) {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(missing)
	if e != nil || len(entries) != 0 {
		t.Fatal("created workspace")
	}
	root := t.TempDir()
	s, _, r := hnEditorialFixture(t, root, "test", "r30")
	c, e := EnsureCandidateForArchivedResult(root, "test", s.ShotID, r.ResultID, "Local Archive")
	if e != nil {
		t.Fatal(e)
	}
	_, e = SelectLocalCandidate(root, "test", s.ShotID, c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	a, e := AddLocalSequenceItem(root, "test", "main", c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	b, e := AddLocalSequenceItem(root, "test", "main", c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	w, e := foundation.OpenExisting(root, "test", true)
	if e != nil {
		t.Fatal(e)
	}
	before, e := w.Read("results", r.ResultID)
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	var record foundation.Result
	if e = json.Unmarshal(before, &record); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(root, "test", filepath.FromSlash(record.ArchivedRelativePath))); e != nil {
		t.Fatal(e)
	}
	state, e := InitializeLocalReorder(root, "test")
	if e != nil || state.SequenceRevision != "2" {
		t.Fatal(state, e)
	}
	snapshot, e := ReadLocalReorderSnapshot(root, "test")
	if e != nil || len(snapshot.Items) != 2 {
		t.Fatal(snapshot, e)
	}
	command := foundation.ReorderCommand{ProtocolVersion: 1, ReorderIntentID: uuid.NewString(), ExpectedRevision: "2", DesiredSequenceItemIDs: []string{b.SequenceItemID, a.SequenceItemID}}
	receipt, e := ExecuteLocalReorder(root, "test", command)
	if e != nil || receipt.Outcome != "COMMITTED" {
		t.Fatal(receipt, e)
	}
	items, e := ReorderLocalSequence(root, "test", "main", []string{a.SequenceItemID, b.SequenceItemID})
	if e != nil || len(items) != 2 {
		t.Fatal(items, e)
	}
	if items[0].CreatedAt != a.CreatedAt || items[0].UpdatedAt != a.UpdatedAt || items[1].UpdatedAt != b.UpdatedAt {
		t.Fatal("legacy timestamp changed")
	}
	raw, _ := json.Marshal(items)
	var shape []map[string]any
	json.Unmarshal(raw, &shape)
	for _, i := range shape {
		if len(i) != 9 {
			t.Fatal("legacy DTO changed")
		}
	}
	old, e := LookupLocalReorder(root, "test", command.ReorderIntentID)
	if e != nil || !reflect.DeepEqual(old, receipt) {
		t.Fatal(old, e)
	}
	replay, e := ExecuteLocalReorder(root, "test", command)
	if e != nil || !reflect.DeepEqual(replay, receipt) {
		t.Fatal("historic replay changed")
	}
	now, e := ReadLocalReorderSnapshot(root, "test")
	if e != nil || now.SequenceRevision != "4" || now.Items[0].SequenceItemID != a.SequenceItemID {
		t.Fatal(now, e)
	}
	w, e = foundation.OpenExisting(root, "test", true)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	after, e := w.Read("results", r.ResultID)
	if e != nil || string(after) != string(before) {
		t.Fatal("reconciled/read media/changed result")
	}
	if _, e = InitializeLocalReorder(root, "../bad"); !errors.Is(e, foundation.ErrReorderInput) {
		t.Fatal(e)
	}
	if dir := os.Getenv("HN_R30_EVIDENCE"); dir != "" {
		os.MkdirAll(dir, 0700)
		data, _ := json.Marshal(map[string]any{"existingStoreOnly": true, "mediaRemovedBeforeProtocol": true, "resultRecordUnchanged": true, "legacyNineFieldDTO": items, "snapshot": now, "originalReceipt": receipt})
		if e = os.WriteFile(filepath.Join(dir, "service-existing-no-media.json"), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
