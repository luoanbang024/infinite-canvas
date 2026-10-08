package service

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestR33ServiceExistingOnlyReplayAndLegacyPreservation(t *testing.T) {
	root := t.TempDir()
	if _, e := LookupLocalExportCommand(root, "test", uuid.NewString()); e == nil {
		t.Fatal("missing store accepted")
	}
	if _, e := os.Stat(filepath.Join(root, "test")); !os.IsNotExist(e) {
		t.Fatal("read created project")
	}
	_, items := hnExportFixture(t, root)
	state, e := InitializeLocalExport(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	snap, e := ReadLocalReorderSnapshot(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for _, i := range items {
		ids = append(ids, i.SequenceItemID)
	}
	c := foundation.ExportCommand{ProtocolVersion: 1, ExportIntentID: uuid.NewString(), ExpectedRevision: snap.SequenceRevision, OrderedSequenceItemIDs: ids, Format: foundation.ExportFormat}
	legacy, e := ExportLocalSequence(root, "test", "main")
	if e != nil {
		t.Fatal(e)
	}
	old, e := os.ReadFile(filepath.Join(root, "test", filepath.FromSlash(legacy.ManifestJSONRelativePath)))
	if e != nil {
		t.Fatal(e)
	}
	s, e := ExecuteLocalExportCommand(root, "test", c)
	if e != nil || s.Outcome != "COMMITTED" {
		t.Fatal(s, e)
	}
	again, e := LookupLocalExportCommand(root, "test", c.ExportIntentID)
	if e != nil || !reflect.DeepEqual(s, again) {
		t.Fatal(again, e)
	}
	before, _ := os.ReadFile(filepath.Join(root, "test", "metadata", "hn-extension.sqlite"))
	h, e := VerifyLocalExportBundle(root, "test", c.ExportIntentID)
	if e != nil || h.Health != "VERIFIED" {
		t.Fatal(h, e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "test", "metadata", "hn-extension.sqlite"))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("verify wrote SQLite")
	}
	later, e := ExportLocalSequence(root, "test", "main")
	if e != nil || later.ExportID == legacy.ExportID {
		t.Fatal("legacy repeat changed", e)
	}
	same, e := os.ReadFile(filepath.Join(root, "test", filepath.FromSlash(legacy.ManifestJSONRelativePath)))
	if e != nil || !reflect.DeepEqual(old, same) {
		t.Fatal("legacy bundle changed", e)
	}
	final, e := ReadLocalReorderSnapshot(root, "test")
	if e != nil || !reflect.DeepEqual(snap, final) {
		t.Fatal("export changed sequence", e)
	}
	if dir := os.Getenv("HN_R33_EVIDENCE"); dir != "" {
		os.MkdirAll(dir, 0700)
		b, _ := json.MarshalIndent(map[string]any{"initialize": state, "new": s, "health": h, "legacyFirst": legacy, "legacySecond": later, "readonlySQLiteBytesUnchanged": true, "sequenceUnchanged": true}, "", "  ")
		if e = os.WriteFile(filepath.Join(dir, "service-composition.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
