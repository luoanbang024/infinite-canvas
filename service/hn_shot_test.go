package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

func TestHNShotInputAndIdentity(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ project, node, label string }{
		{"../escape", "node", ""}, {"test", "", ""}, {"test", "  ", ""},
		{"test", strings.Repeat("a", 129), ""}, {"test", strings.Repeat("镜", 43), ""},
		{"test", "https://invalid.example/?sig=fixture", ""}, {"test", "data:fixture", ""},
		{"test", "token=fixture", ""}, {"test", "C:\\fixture", ""}, {"test", "/fixture", ""},
		{"test", "node\n", ""}, {"test", "node", "Bearer fixture"},
		{"test", "node", "-----BEGIN synthetic private-key fixture"}, {"test", "node", strings.Repeat("a", 257)},
		{"test", "node", "C:\\fixture"},
	} {
		if _, err := EnsureShotForSourceNode(root, tc.project, tc.node, tc.label); !errors.Is(err, ErrHNShotInput) {
			t.Fatal("invalid input accepted", err)
		}
	}
	a, err := EnsureShotForSourceNode(root, "test", "node/relative:1", "Initial title")
	if err != nil || a.Label != "Initial title" || a.SourceNodeID != "node/relative:1" {
		t.Fatal("first ensure", err, a)
	}
	again, err := EnsureShotForSourceNode(root, "test", a.SourceNodeID, "Changed title")
	if err != nil || !reflect.DeepEqual(a, again) {
		t.Fatal("repeat changed identity/label/timestamps", err)
	}
	b, err := EnsureShotForSourceNode(root, "test", "Node/relative:1", "")
	if err != nil || b.ShotID == a.ShotID || b.Label != "Shot" {
		t.Fatal("exact case-sensitive identity or default", err)
	}
	foreign, err := EnsureShotForSourceNode(root, "foreign", a.SourceNodeID, "Other project")
	if err != nil || foreign.ShotID == a.ShotID {
		t.Fatal("project separation", err)
	}
}

func TestHNShotConcurrentEnsureAndConflict(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := EnsureShotForSourceNode(root, "test", "same-node", "Initial")
			if err != nil {
				t.Error(err)
			}
			ids <- s.ShotID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id == "" || id != first {
			t.Fatal("duplicate identity")
		}
	}
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := w.List("shots")
	if err != nil || len(rows) != 1 {
		t.Fatal("concurrent duplicate", err)
	}
	if _, err = w.CreateShot("A", "duplicate"); err != nil {
		t.Fatal(err)
	}
	if _, err = w.CreateShot("B", "duplicate"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	if _, err = EnsureShotForSourceNode(root, "test", "duplicate", "C"); !errors.Is(err, ErrHNShotIdentityConflict) {
		t.Fatal("conflict not detected", err)
	}
	w, err = foundation.Open(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	rows, err = w.List("shots")
	if err != nil || len(rows) != 3 {
		t.Fatal("conflict mutated store", err)
	}
}

func TestHNShotGenerationBindingHistoricalReopen(t *testing.T) {
	root := t.TempDir()
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	old, err := w.CreateGeneration(foundation.Generation{NodeID: "legacy", PromptSnapshot: "old synthetic", SourceBaseline: "418ffbde3dbea33d374356588cb672336ec38353"})
	if err != nil {
		t.Fatal(err)
	}
	old, err = w.FreezeGeneration(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := w.Read("generations", old.ID)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	s, err := EnsureShotForSourceNode(root, "test", "video", "Shot A")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := EnsureShotForSourceNode(root, "foreign", "video", "Shot B")
	if err != nil {
		t.Fatal(err)
	}
	i := hnTestInput()
	i.NodeID = s.SourceNodeID
	i.ShotID = s.ShotID
	for _, id := range []string{"missing", "../invalid", foreign.ShotID} {
		bad := i
		bad.ShotID = id
		if _, err = PrepareLocalGeneration(root, "test", bad); !errors.Is(err, ErrHNGenerationInput) {
			t.Fatal("invalid Shot accepted", err)
		}
	}
	bad := i
	bad.SourceBaseline = old.SourceBaseline
	if _, err = PrepareLocalGeneration(root, "test", bad); !errors.Is(err, ErrHNGenerationInput) {
		t.Fatal("old baseline accepted for new prepare", err)
	}
	a, err := PrepareLocalGeneration(root, "test", i)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PrepareLocalGeneration(root, "test", i)
	if err != nil || a.GenerationID == b.GenerationID || a.ShotID != s.ShotID || b.ShotID != s.ShotID || !a.Frozen || a.SourceBaseline != "16047f46e2186373ea824e12e84ae8dfa2ccde32" {
		t.Fatal("Shot-aware attempts", err)
	}
	w, err = foundation.Open(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	after, err := w.Read("generations", old.ID)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("historical frozen record rewritten", err)
	}
	old.ShotID = s.ShotID
	if w.UpdateGeneration(old) == nil {
		t.Fatal("retroactive Shot binding accepted")
	}
	after, err = w.Read("generations", old.ID)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed update altered historical record", err)
	}
	for _, prepared := range []HNPreparedGeneration{a, b} {
		raw, err := w.Read("generations", prepared.GenerationID)
		var g foundation.Generation
		if err != nil || json.Unmarshal(raw, &g) != nil || g.ShotID != s.ShotID || g.FrozenHash != prepared.FrozenHash || g.Status != "PREPARED" {
			t.Fatal("reopen binding/hash", err)
		}
		g.ShotID = ""
		if w.UpdateGeneration(g) == nil {
			t.Fatal("frozen Shot mutable")
		}
	}
	for _, kind := range []string{"results", "archive_jobs", "task_bindings", "candidates", "sequence_items"} {
		rows, err := w.List(kind)
		if err != nil || len(rows) != 0 {
			t.Fatal("unexpected runtime entity", kind, err)
		}
	}
}
