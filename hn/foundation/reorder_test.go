package foundation

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReorderChangesOnlyOrderIndexIncludingUpdatedAt(t *testing.T) {
	w := newWorkspace(t)
	s := makeShot(t, w, "reorder")
	g := makeGeneration(t, w, s.ID, "reorder")
	r := makeResult(t, w, g)
	archive(t, w, r, []byte("synthetic reorder fixture"))
	c, err := w.CreateCandidate(s.ID, r.ID, "Candidate")
	okay(t, err)
	okay(t, w.SelectCandidate(s.ID, c.ID))
	ids := []string{}
	original := map[string]map[string]json.RawMessage{}
	for n := 0; n < 3; n++ {
		i, e := w.AddSequenceItem("main", c.ID)
		okay(t, e)
		ids = append(ids, i.ID)
		raw, e := w.Read("sequence_items", i.ID)
		okay(t, e)
		var fields map[string]json.RawMessage
		okay(t, json.Unmarshal(raw, &fields))
		original[i.ID] = fields
	}
	prerequisites := map[string][]json.RawMessage{}
	for _, kind := range []string{"shots", "generations", "results", "archive_jobs", "candidates", "task_bindings"} {
		prerequisites[kind], err = w.List(kind)
		okay(t, err)
	}
	order := []string{ids[2], ids[0], ids[1]}
	check := func() {
		t.Helper()
		for index, id := range order {
			raw, e := w.Read("sequence_items", id)
			okay(t, e)
			var fields map[string]json.RawMessage
			okay(t, json.Unmarshal(raw, &fields))
			var actualIndex int
			okay(t, json.Unmarshal(fields["orderIndex"], &actualIndex))
			if actualIndex != index {
				t.Fatal("orderIndex does not match requested order")
			}
			fields["orderIndex"] = original[id]["orderIndex"]
			// Compare every persisted JSON field, including UpdatedAt and future fields.
			if !reflect.DeepEqual(fields, original[id]) {
				t.Fatal("reorder changed a field other than OrderIndex (including UpdatedAt)")
			}
		}
		for kind, before := range prerequisites {
			after, e := w.List(kind)
			okay(t, e)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("reorder changed another entity", kind)
			}
		}
	}
	okay(t, w.Reorder("main", order))
	check()
	okay(t, w.Reorder("main", order)) // A no-op reorder must retain all timestamps too.
	check()
	root, project := w.Root, w.ProjectID
	okay(t, w.Close())
	w, err = Open(filepath.Dir(root), project)
	okay(t, err)
	defer w.Close()
	check()
}
