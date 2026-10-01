package service

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

func hnTestInput() HNGenerationPrepareInput {
	return HNGenerationPrepareInput{NodeID: "_canvas-video", PromptSnapshot: "synthetic camera move", SourceBaseline: HNGenerationSourceBaseline, Parameters: json.RawMessage(`{"videoSeconds":"6","size":"16:9"}`)}
}

func TestHNGenerationValidation(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		edit func(*HNGenerationPrepareInput)
	}{
		{"empty-prompt", func(i *HNGenerationPrepareInput) { i.PromptSnapshot = "  " }},
		{"empty-node", func(i *HNGenerationPrepareInput) { i.NodeID = "" }},
		{"missing-baseline", func(i *HNGenerationPrepareInput) { i.SourceBaseline = "" }},
		{"old-baseline", func(i *HNGenerationPrepareInput) { i.SourceBaseline = foundation.SourceBaseline }},
		{"array-parameters", func(i *HNGenerationPrepareInput) { i.Parameters = json.RawMessage(`[]`) }},
		{"null-parameters", func(i *HNGenerationPrepareInput) { i.Parameters = json.RawMessage(`null`) }},
		{"null-scalar", func(i *HNGenerationPrepareInput) { i.Parameters = json.RawMessage(`{"size":null}`) }},
		{"null-shot-value", func(i *HNGenerationPrepareInput) {
			i.Parameters = json.RawMessage(`{"videoMultiPrompt":[{"prompt":null,"duration":"1"}]}`)
		}},
		{"secret-field", func(i *HNGenerationPrepareInput) { i.Parameters = json.RawMessage(`{"apiKey":"synthetic"}`) }},
		{"nested-secret", func(i *HNGenerationPrepareInput) {
			i.Parameters = json.RawMessage(`{"videoMultiPrompt":[{"prompt":"ok","duration":"1","token":"synthetic"}]}`)
		}},
		{"credential-literal", func(i *HNGenerationPrepareInput) { i.PromptSnapshot = "Bearer synthetic-fixture" }},
		{"embedded-signed-url", func(i *HNGenerationPrepareInput) { i.PromptSnapshot = "see https://invalid.example/?sig=fixture" }},
		{"model-secret", func(i *HNGenerationPrepareInput) { i.Model = "sk-proj-synthetic-fixture" }},
		{"connection-secret", func(i *HNGenerationPrepareInput) { i.ConnectionID = "token=fixture" }},
		{"wrong-scalar-type", func(i *HNGenerationPrepareInput) { i.Parameters = json.RawMessage(`{"size":5}`) }},
		{"unsupported-role", func(i *HNGenerationPrepareInput) {
			i.ReferenceBindings = []foundation.ReferenceBinding{{ReferenceVersionID: "missing", SHA256: strings.Repeat("a", 64), Role: "video"}}
		}},
		{"missing-reference", func(i *HNGenerationPrepareInput) {
			i.ReferenceBindings = []foundation.ReferenceBinding{{ReferenceVersionID: "missing", SHA256: strings.Repeat("a", 64), Role: "reference"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := hnTestInput()
			tc.edit(&i)
			if _, err := PrepareLocalGeneration(root, "test", i); err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
	if _, err := PrepareLocalGeneration(root, "../escape", hnTestInput()); err == nil {
		t.Fatal("unsafe project accepted")
	}
}

func TestHNGenerationExactBindingAttemptsReopenAndUnknown(t *testing.T) {
	root := t.TempDir()
	r, err := SnapshotLocalReference(root, "canvas-test", "image-a", "image/png", bytes.NewReader([]byte("synthetic exact bytes")), int64(len("synthetic exact bytes")))
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := SnapshotLocalReference(root, "foreign", "image-a", "image/png", bytes.NewReader([]byte("foreign bytes")), int64(len("foreign bytes")))
	if err != nil {
		t.Fatal(err)
	}
	i := hnTestInput()
	i.ReferenceBindings = []foundation.ReferenceBinding{{ReferenceVersionID: r.ID, SHA256: r.SHA256, Role: "firstFrame"}}
	for _, binding := range []foundation.ReferenceBinding{{ReferenceVersionID: r.ID, SHA256: strings.Repeat("a", 64), Role: "reference"}, {ReferenceVersionID: foreign.ID, SHA256: foreign.SHA256, Role: "reference"}} {
		bad := i
		bad.ReferenceBindings = []foundation.ReferenceBinding{binding}
		if _, err := PrepareLocalGeneration(root, "canvas-test", bad); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	first, err := PrepareLocalGeneration(root, "canvas-test", i)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Frozen || first.Status != "PREPARED" || first.SubmissionState != "PREPARED" || first.SourceBaseline != HNGenerationSourceBaseline || !reflect.DeepEqual(first.ReferenceBindings, i.ReferenceBindings) {
		t.Fatalf("bad frozen metadata: %+v", first)
	}
	w, err := foundation.Open(root, "canvas-test")
	if err != nil {
		t.Fatal(err)
	}
	before, err := w.Read("generations", first.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	second, err := PrepareLocalGeneration(root, "canvas-test", i)
	if err != nil {
		t.Fatal(err)
	}
	if second.GenerationID == first.GenerationID {
		t.Fatal("attempt reused")
	}
	w, err = foundation.Open(root, "canvas-test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	after, err := w.Read("generations", first.GenerationID)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("old attempt changed")
	}
	g, err := w.FreezeGeneration(first.GenerationID)
	if err != nil || g.FrozenHash != first.FrozenHash {
		t.Fatal("reopen hash failed", err)
	}
	g.PromptSnapshot = "changed"
	if w.UpdateGeneration(g) == nil {
		t.Fatal("frozen request mutable")
	}
	for _, kind := range []string{"results", "archive_jobs", "task_bindings"} {
		rows, err := w.List(kind)
		if err != nil || len(rows) != 0 {
			t.Fatal("unexpected runtime entities", kind)
		}
	}
	if err = w.MarkSubmissionUnknown(second.GenerationID); err != nil {
		t.Fatal(err)
	}
	raw, err := w.Read("generations", second.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(raw, &g)
	if g.SubmissionState != "SUBMISSION_UNKNOWN" {
		t.Fatal("unknown state missing")
	}
}

func TestHNGenerationT2VAndUnsupportedReference(t *testing.T) {
	root := t.TempDir()
	prepared, err := PrepareLocalGeneration(root, "t2v", hnTestInput())
	if err != nil || len(prepared.ReferenceBindings) != 0 {
		t.Fatal("T2V failed", err)
	}
	w, err := foundation.Open(root, "t2v")
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.Snapshot("video-ref", "video", "video/mp4", bytes.NewReader([]byte("synthetic")), int64(len("synthetic")))
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	i := hnTestInput()
	i.ReferenceBindings = []foundation.ReferenceBinding{{ReferenceVersionID: r.ID, SHA256: r.SHA256, Role: "reference"}}
	if _, err = PrepareLocalGeneration(root, "t2v", i); err == nil {
		t.Fatal("video reference accepted")
	}
}
