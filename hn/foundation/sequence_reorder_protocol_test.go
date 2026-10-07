package foundation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func r30Revision(t *testing.T, w *Workspace) string {
	t.Helper()
	s, e := w.ReadReorderSnapshot()
	okay(t, e)
	return s.SequenceRevision
}
func r30Command(revision string, ids ...string) ReorderCommand {
	return ReorderCommand{1, uuid.NewString(), revision, append([]string{}, ids...)}
}
func r30Items(t *testing.T, w *Workspace) map[string]map[string]json.RawMessage {
	t.Helper()
	rows, e := w.List("sequence_items")
	okay(t, e)
	out := map[string]map[string]json.RawMessage{}
	for _, raw := range rows {
		var fields map[string]json.RawMessage
		okay(t, json.Unmarshal(raw, &fields))
		var id string
		okay(t, json.Unmarshal(fields["id"], &id))
		out[id] = fields
	}
	return out
}
func r30Invariant(t *testing.T, w *Workspace, before map[string]map[string]json.RawMessage) {
	t.Helper()
	after := r30Items(t, w)
	if len(after) != len(before) {
		t.Fatal("membership changed")
	}
	for id, fields := range after {
		var index int
		okay(t, json.Unmarshal(fields["orderIndex"], &index))
		var sqlIndex int
		okay(t, w.db.QueryRow("SELECT order_index FROM sequence_items WHERE id=?", id).Scan(&sqlIndex))
		if index != sqlIndex {
			t.Fatal("JSON/index disagreement")
		}
		fields["orderIndex"] = before[id]["orderIndex"]
		if !reflect.DeepEqual(fields, before[id]) {
			t.Fatal("non-order field changed", id)
		}
		var i SequenceItem
		okay(t, json.Unmarshal(r30JSON(t, fields), &i))
		var project, sequence, shot, candidate, result string
		okay(t, w.db.QueryRow("SELECT project_id,sequence_id,shot_id,candidate_id,result_id FROM sequence_items WHERE id=?", id).Scan(&project, &sequence, &shot, &candidate, &result))
		if project != i.ProjectID || sequence != i.SequenceID || shot != i.ShotID || candidate != i.CandidateID || result != i.ResultID {
			t.Fatal("indexed non-order changed")
		}
	}
}
func r30Evidence(t *testing.T, name string, data any) {
	t.Helper()
	if dir := os.Getenv("HN_R30_EVIDENCE"); dir != "" {
		okay(t, os.MkdirAll(dir, 0700))
		okay(t, os.WriteFile(filepath.Join(dir, name+".json"), r30JSON(t, data), 0600))
	}
}
func TestR30InitReadonlySchemaAndCapacity(t *testing.T) {
	w, p := placementFixture(t)
	defer w.Close()
	raw, e := os.ReadFile(filepath.Join(w.Root, "metadata", "hn-extension.sqlite"))
	okay(t, e)
	_, e = w.ReadReorderSnapshot()
	if !errors.Is(e, ErrReorderProtocol) {
		t.Fatal(e)
	}
	_, e = w.LookupReorderCommand(uuid.NewString())
	if !errors.Is(e, ErrReorderProtocol) {
		t.Fatal(e)
	}
	after, e := os.ReadFile(filepath.Join(w.Root, "metadata", "hn-extension.sqlite"))
	okay(t, e)
	if !bytes.Equal(raw, after) {
		t.Fatal("readonly changed DB")
	}
	state, e := w.InitializeReorderProtocol()
	okay(t, e)
	if state.SequenceRevision != "0" {
		t.Fatal(state)
	}
	same, e := w.InitializeReorderProtocol()
	okay(t, e)
	if same != state {
		t.Fatal("init incremented")
	}
	not, e := w.LookupReorderCommand(uuid.NewString())
	okay(t, e)
	if not.(ReorderLookup).Outcome != "NOT_OBSERVED" {
		t.Fatal(not)
	}
	for n := 0; n < 257; n++ {
		_, e = w.AddSequenceItem("main", p.CandidateID)
		okay(t, e)
	}
	_, e = w.ReadReorderSnapshot()
	if !errors.Is(e, ErrPlacementCapacity) {
		t.Fatal(e)
	}
	state, e = w.InitializeReorderProtocol()
	okay(t, e)
	if state.SequenceRevision != "257" {
		t.Fatal(state)
	}
	for _, mode := range []string{"partial", "unknown", "missing-state", "malformed-state"} {
		t.Run(mode, func(t *testing.T) {
			x := newWorkspace(t)
			defer x.Close()
			if mode == "partial" {
				_, e = x.db.Exec(reorderSchema[0].sql)
				okay(t, e)
			} else {
				_, e = x.InitializeReorderProtocol()
				okay(t, e)
				switch mode {
				case "unknown":
					_, e = x.db.Exec("PRAGMA ignore_check_constraints=ON; UPDATE reorder_protocol SET version=2")
				case "missing-state":
					_, e = x.db.Exec("DELETE FROM sequence_state")
				case "malformed-state":
					_, e = x.db.Exec("PRAGMA ignore_check_constraints=ON; UPDATE sequence_state SET revision=-1")
				}
				okay(t, e)
			}
			_, e = x.ReadReorderSnapshot()
			if !errors.Is(e, ErrReorderIntegrity) {
				t.Fatal(mode, e)
			}
			_, e = x.InitializeReorderProtocol()
			if !errors.Is(e, ErrReorderIntegrity) {
				t.Fatal("auto repair", mode, e)
			}
		})
	}
	var schemaRows []map[string]string
	rows, e := w.db.Query("SELECT name,sql FROM sqlite_master WHERE name GLOB 'reorder_*' OR name='sequence_state' ORDER BY name")
	okay(t, e)
	for rows.Next() {
		var name, sql string
		okay(t, rows.Scan(&name, &sql))
		schemaRows = append(schemaRows, map[string]string{"name": name, "sql": sql})
	}
	okay(t, rows.Close())
	r30Evidence(t, "schema-initialization", map[string]any{"initialize": same, "schema": schemaRows, "readonlyZeroWrite": true, "capacity": 257, "extensionObjects": len(reorderSchema)})
}
func TestR30EveryWriterRevisionAndHistoricalReceipt(t *testing.T) {
	w, p := placementFixture(t)
	defer w.Close()
	_, e := w.InitializeReorderProtocol()
	okay(t, e)
	deltas := []string{"0"}
	check := func(expected string) {
		t.Helper()
		actual := r30Revision(t, w)
		if actual != expected {
			t.Fatal(actual, expected)
		}
		deltas = append(deltas, actual)
	}
	_, e = w.AddSequenceItem("main", "missing")
	reject(t, e)
	check("0")
	other, e := w.AddSequenceItem("other", p.CandidateID)
	okay(t, e)
	check("0")
	a, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	check("1")
	b, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	check("2")
	bound, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	check("3")
	same, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if !reflect.DeepEqual(same, bound) {
		t.Fatal("placement wire changed")
	}
	check("3")
	bad := p
	bad.PreparedFrozenHash = strings.Repeat("a", 64)
	_, e = w.ExecutePlacementCommand(bad)
	if !errors.Is(e, ErrPlacementIntent) {
		t.Fatal(e)
	}
	check("3")
	bad.PlacementIntentID = uuid.NewString()
	rejected, e := w.ExecutePlacementCommand(bad)
	okay(t, e)
	if rejected.Outcome != "REJECTED" {
		t.Fatal(rejected)
	}
	check("3")
	ids := []string{a.ID, b.ID, bound.OriginalItem.SequenceItemID}
	before := r30Items(t, w)
	okay(t, w.Reorder("main", ids))
	r30Invariant(t, w, before)
	check("4")
	okay(t, w.Reorder("main", ids))
	r30Invariant(t, w, before)
	check("5")
	reject(t, w.Reorder("main", ids[:2]))
	check("5")
	c := r30Command("5", ids[2], ids[0], ids[1])
	receipt, e := w.ExecuteReorderCommand(c)
	okay(t, e)
	if receipt.Outcome != "COMMITTED" {
		t.Fatal(receipt)
	}
	r30Invariant(t, w, before)
	check("6")
	duplicate, e := w.ExecuteReorderCommand(c)
	okay(t, e)
	if !reflect.DeepEqual(receipt, duplicate) {
		t.Fatal("receipt replay changed")
	}
	check("6")
	changed := c
	changed.ExpectedRevision = "6"
	_, e = w.ExecuteReorderCommand(changed)
	if !errors.Is(e, ErrReorderIntent) {
		t.Fatal(e)
	}
	check("6")
	stale := r30Command("5", ids...)
	conflict, e := w.ExecuteReorderCommand(stale)
	okay(t, e)
	if *conflict.ErrorClass != "SEQUENCE_REVISION_CONFLICT" {
		t.Fatal(conflict)
	}
	check("6")
	set, e := w.ExecuteReorderCommand(r30Command("6", ids[:2]...))
	okay(t, e)
	if *set.ErrorClass != "SEQUENCE_SET_CONFLICT" {
		t.Fatal(set)
	}
	check("6")
	noop, e := w.ExecuteReorderCommand(r30Command("6", ids[2], ids[0], ids[1]))
	okay(t, e)
	if *noop.AppliedRevision != "7" {
		t.Fatal(noop)
	}
	r30Invariant(t, w, before)
	check("7")
	okay(t, w.Reorder("main", ids))
	check("8") // ABA old desired vector, revision remains newer.
	aba, e := w.ExecuteReorderCommand(r30Command("5", ids...))
	okay(t, e)
	if aba.Outcome != "CONFLICT" {
		t.Fatal(aba)
	}
	check("8")
	lookup, e := w.LookupReorderCommand(c.ReorderIntentID)
	okay(t, e)
	if !reflect.DeepEqual(lookup, receipt) {
		t.Fatal("history invalidated")
	}
	duplicate, e = w.ExecuteReorderCommand(c)
	okay(t, e)
	if !reflect.DeepEqual(duplicate, receipt) {
		t.Fatal("history reapply")
	}
	check("8")
	reject(t, w.Delete("sequence_items", bound.OriginalItem.SequenceItemID))
	check("8")
	reject(t, w.Delete("sequence_items", "missing"))
	check("8")
	okay(t, w.Delete("sequence_items", other.ID))
	check("8")
	okay(t, w.Delete("sequence_items", a.ID))
	check("9")
	original, e := w.LookupReorderCommand(c.ReorderIntentID)
	okay(t, e)
	if !reflect.DeepEqual(original, receipt) {
		t.Fatal("historical receipt after deletion")
	}
	_, e = w.db.Exec("UPDATE reorder_commands SET receipt='bad'")
	reject(t, e)
	_, e = w.db.Exec("DELETE FROM reorder_commands")
	reject(t, e)
	_, e = w.db.Exec("INSERT OR REPLACE INTO reorder_commands SELECT * FROM reorder_commands")
	reject(t, e)
	root := filepath.Dir(w.Root)
	okay(t, w.Close())
	w, e = OpenExisting(root, "project-a", true)
	okay(t, e)
	defer w.Close()
	check("9")
	r30Evidence(t, "writer-deltas-receipts", map[string]any{"deltas": deltas, "committed": receipt, "staleConflict": conflict, "setConflict": set, "noOp": noop, "historicAfterDelete": original, "r7AllFieldsPreserved": true, "placementReceipt": bound})
}
func TestR30OverflowRollbackAndMalformed(t *testing.T) {
	for _, v := range []string{"", "00", "01", "+1", "-1", " 1", "1 ", "1e2", "9223372036854775808"} {
		if _, ok := ParseSequenceRevision(v); ok {
			t.Fatal(v)
		}
	}
	for _, v := range []string{"0", "1", "9223372036854775807"} {
		if _, ok := ParseSequenceRevision(v); !ok {
			t.Fatal(v)
		}
	}
	w, p := placementFixture(t)
	defer w.Close()
	a, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	b, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	before := r30Items(t, w)
	c := r30Command("2", b.ID, a.ID)
	for _, statement := range []string{
		"CREATE TRIGGER fault BEFORE UPDATE ON sequence_items BEGIN SELECT RAISE(ABORT,'synthetic fault'); END",
		"CREATE TRIGGER fault BEFORE UPDATE ON sequence_state BEGIN SELECT RAISE(ABORT,'synthetic fault'); END",
		"CREATE TRIGGER fault BEFORE INSERT ON reorder_commands BEGIN SELECT RAISE(ABORT,'synthetic fault'); END",
	} {
		_, e = w.db.Exec(statement)
		okay(t, e)
		_, e = w.ExecuteReorderCommand(c)
		reject(t, e)
		if !reflect.DeepEqual(before, r30Items(t, w)) || r30Revision(t, w) != "2" {
			t.Fatal("partial transaction")
		}
		absent, e := w.LookupReorderCommand(c.ReorderIntentID)
		okay(t, e)
		if absent.(ReorderLookup).Outcome != "NOT_OBSERVED" {
			t.Fatal("fabricated terminal")
		}
		_, e = w.db.Exec("DROP TRIGGER fault")
		okay(t, e)
	}
	_, e = w.db.Exec("UPDATE sequence_state SET revision=?", int64(math.MaxInt64))
	okay(t, e)
	commands := []func() error{
		func() error { _, e := w.AddSequenceItem("main", p.CandidateID); return e },
		func() error { return w.Reorder("main", []string{b.ID, a.ID}) },
		func() error { _, e := w.ExecutePlacementCommand(p); return e },
		func() error {
			_, e := w.ExecuteReorderCommand(r30Command(strconv.FormatInt(math.MaxInt64, 10), b.ID, a.ID))
			return e
		},
		func() error { return w.Delete("sequence_items", a.ID) },
	}
	for _, f := range commands {
		if e := f(); !errors.Is(e, ErrReorderExhausted) {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(before, r30Items(t, w)) || r30Revision(t, w) != strconv.FormatInt(math.MaxInt64, 10) {
			t.Fatal("overflow mutated")
		}
	}
	for _, ids := range [][]string{nil, {a.ID, a.ID}, {"../bad"}, make([]string, 257)} {
		bad := r30Command("0", ids...)
		if ids == nil {
			bad.DesiredSequenceItemIDs = nil
		}
		_, e = w.ExecuteReorderCommand(bad)
		if !errors.Is(e, ErrReorderInput) {
			t.Fatal(e)
		}
	}
	// Failed lazy append leaves the reorder extension absent.
	fresh, p2 := placementFixture(t)
	defer fresh.Close()
	_, e = fresh.db.Exec("CREATE TRIGGER fault BEFORE INSERT ON sequence_items BEGIN SELECT RAISE(ABORT,'synthetic fault'); END")
	okay(t, e)
	_, e = fresh.AddSequenceItem("main", p2.CandidateID)
	reject(t, e)
	_, e = fresh.ReadReorderSnapshot()
	if !errors.Is(e, ErrReorderProtocol) {
		t.Fatal("failed lazy install committed", e)
	}
}

func r30JSON(t *testing.T, v any) []byte { t.Helper(); b, e := json.Marshal(v); okay(t, e); return b }
func TestR30RealTwoProcess(t *testing.T) {
	if root := os.Getenv("HN_R30_CHILD_ROOT"); root != "" {
		var input struct {
			Mode      string
			Command   ReorderCommand
			Placement PlacementCommand
			IDs       []string
			DeleteID  string
		}
		okay(t, json.Unmarshal([]byte(os.Getenv("HN_R30_CHILD_INPUT")), &input))
		w, e := OpenExisting(root, "project-a", false)
		okay(t, e)
		defer w.Close()
		okay(t, os.WriteFile(os.Getenv("HN_R30_CHILD_READY"), []byte("ready"), 0600))
		deadline := time.Now().Add(30 * time.Second)
		for {
			if _, e := os.Stat(filepath.Join(root, "release")); e == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("rendezvous timeout")
			}
			time.Sleep(10 * time.Millisecond)
		}
		var data any
		switch input.Mode {
		case "initialize":
			data, e = w.InitializeReorderProtocol()
		case "reorder":
			data, e = w.ExecuteReorderCommand(input.Command)
		case "add":
			data, e = w.AddSequenceItem("main", input.Placement.CandidateID)
		case "append-many":
			values := []SequenceItem{}
			for n := 0; n < 8; n++ {
				item, err := w.AddSequenceItem("main", input.Placement.CandidateID)
				if err != nil {
					t.Fatal(err)
				}
				values = append(values, item)
			}
			data = values
		case "placement":
			data, e = w.ExecutePlacementCommand(input.Placement)
		case "legacy":
			e = w.Reorder("main", input.IDs)
		case "delete":
			e = w.Delete("sequence_items", input.DeleteID)
		default:
			t.Fatal("unknown child mode")
		}
		failure := ""
		if e != nil {
			if input.Mode != "legacy" || !errors.Is(e, ErrReorderInput) {
				t.Fatal(e)
			}
			failure = "REORDER_INPUT_INVALID"
		}
		okay(t, os.WriteFile(os.Getenv("HN_R30_CHILD_OUTPUT"), r30JSON(t, map[string]any{"pid": os.Getpid(), "mode": input.Mode, "data": data, "error": failure}), 0600))
		return
	}
	for _, scenario := range []string{"initialize-initialize", "same-key", "different-orders", "same-order", "reorder-add", "reorder-placement", "legacy-add", "delete-reorder", "monotonic-add-add"} {
		t.Run(scenario, func(t *testing.T) {
			w, p := placementFixture(t)
			root := filepath.Dir(w.Root)
			type childInput struct {
				Mode      string
				Command   ReorderCommand
				Placement PlacementCommand
				IDs       []string
				DeleteID  string
			}
			base := int64(0)
			ids := []string{}
			if scenario != "initialize-initialize" {
				for n := 0; n < 3; n++ {
					i, e := w.AddSequenceItem("main", p.CandidateID)
					okay(t, e)
					ids = append(ids, i.ID)
				}
				base = 3
			}
			command := r30Command(strconv.FormatInt(base, 10))
			if len(ids) > 0 {
				command.DesiredSequenceItemIDs = []string{ids[2], ids[0], ids[1]}
			}
			commands := []childInput{{Mode: "reorder", Command: command, Placement: p, IDs: command.DesiredSequenceItemIDs}, {Mode: "reorder", Command: command, Placement: p, IDs: command.DesiredSequenceItemIDs}}
			switch scenario {
			case "initialize-initialize":
				commands[0].Mode = "initialize"
				commands[1].Mode = "initialize"
			case "different-orders":
				commands[1].Command = r30Command("3", ids...)
			case "same-order":
				commands[1].Command.ReorderIntentID = uuid.NewString()
			case "reorder-add":
				commands[1].Mode = "add"
			case "reorder-placement":
				commands[1].Mode = "placement"
			case "legacy-add":
				commands[0].Mode = "legacy"
				commands[1].Mode = "add"
			case "monotonic-add-add":
				commands[0].Mode = "append-many"
				commands[1].Mode = "append-many"
			case "delete-reorder":
				commands[0].Mode = "delete"
				commands[0].DeleteID = ids[0]
			}
			okay(t, w.Close())
			children := []*exec.Cmd{}
			logs := []*bytes.Buffer{}
			for n, input := range commands {
				cmd := exec.Command(os.Args[0], "-test.run=^TestR30RealTwoProcess$", "-test.v")
				cmd.Env = append(os.Environ(), "HN_R30_CHILD_ROOT="+root, "HN_R30_CHILD_INPUT="+string(r30JSON(t, input)), "HN_R30_CHILD_READY="+filepath.Join(root, fmt.Sprintf("ready-%d", n)), "HN_R30_CHILD_OUTPUT="+filepath.Join(root, fmt.Sprintf("output-%d.json", n)))
				log := &bytes.Buffer{}
				cmd.Stdout = log
				cmd.Stderr = log
				children = append(children, cmd)
				logs = append(logs, log)
				okay(t, cmd.Start())
			}
			deadline := time.Now().Add(30 * time.Second)
			for {
				_, a := os.Stat(filepath.Join(root, "ready-0"))
				_, b := os.Stat(filepath.Join(root, "ready-1"))
				if a == nil && b == nil {
					break
				}
				if time.Now().After(deadline) {
					for _, c := range children {
						c.Process.Kill()
					}
					t.Fatal("children did not rendezvous")
				}
				time.Sleep(10 * time.Millisecond)
			}
			okay(t, os.WriteFile(filepath.Join(root, "release"), []byte("go"), 0600))
			observations := []string{}
			if scenario == "monotonic-add-add" {
				observer, err := OpenExisting(root, "project-a", true)
				okay(t, err)
				last := int64(3)
				for {
					view, err := observer.ReadReorderSnapshot()
					okay(t, err)
					revision, ok := ParseSequenceRevision(view.SequenceRevision)
					if !ok || revision < last || int64(len(view.Items)) != revision {
						t.Fatal("snapshot revision/vector torn", view)
					}
					last = revision
					observations = append(observations, view.SequenceRevision)
					_, a := os.Stat(filepath.Join(root, "output-0.json"))
					_, b := os.Stat(filepath.Join(root, "output-1.json"))
					if a == nil && b == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("observer deadline")
					}
					time.Sleep(2 * time.Millisecond)
				}
				okay(t, observer.Close())
			}

			for n, child := range children {
				if e := child.Wait(); e != nil {
					t.Fatal(e, logs[n].String())
				}
			}
			w, e := OpenExisting(root, "project-a", true)
			okay(t, e)
			defer w.Close()
			snapshot, e := w.ReadReorderSnapshot()
			okay(t, e)
			actual, ok := ParseSequenceRevision(snapshot.SequenceRevision)
			if !ok {
				t.Fatal(snapshot)
			}
			outputs := []map[string]any{}
			effects := int64(0)
			committed := 0
			for n, input := range commands {
				raw, e := os.ReadFile(filepath.Join(root, fmt.Sprintf("output-%d.json", n)))
				okay(t, e)
				var output map[string]any
				okay(t, json.Unmarshal(raw, &output))
				outputs = append(outputs, output)
				if output["error"] != "" {
					continue
				}
				if input.Mode == "reorder" {
					r := output["data"].(map[string]any)
					if r["outcome"] == "COMMITTED" {
						committed++
						if scenario != "same-key" || committed == 1 {
							effects++
						}
					}
				} else if input.Mode == "append-many" {
					effects += 8
				} else if input.Mode != "initialize" {
					effects++
				}
			}
			if actual != base+effects {
				t.Fatal("lost/duplicate revision transition", actual, base, effects, outputs)
			}
			if scenario == "same-key" && committed != 2 {
				t.Fatal("same-key replay not committed twice", outputs)
			}
			if (scenario == "different-orders" || scenario == "same-order") && committed != 1 {
				t.Fatal("CAS race", outputs)
			}
			var commandCount int
			okay(t, w.db.QueryRow("SELECT count(*) FROM reorder_commands").Scan(&commandCount))
			if scenario == "same-key" && commandCount != 1 {
				t.Fatal("duplicate command")
			}
			if (scenario == "different-orders" || scenario == "same-order") && commandCount != 2 {
				t.Fatal("missing terminal conflict")
			}
			seen := map[int]bool{}
			for _, item := range snapshot.Items {
				if seen[item.OrderIndex] {
					t.Fatal("duplicate order")
				}
				seen[item.OrderIndex] = true
			}
			if outputs[0]["pid"] == outputs[1]["pid"] {
				t.Fatal("not separate processes")
			}
			r30Evidence(t, "two-process-"+scenario, map[string]any{"scenario": scenario, "childProcesses": 2, "bothOpenedBeforeRelease": true, "outputs": outputs, "baseRevision": base, "successfulEffects": effects, "finalRevision": actual, "commandCount": commandCount, "items": snapshot.Items, "monotonicRevision": true, "consistentReadObservations": observations})
		})
	}
}

func TestR30AllFieldsAndOtherEntitiesPreserved(t *testing.T) {
	w, p := placementFixture(t)
	defer w.Close()
	a, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	b, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	raw, e := w.Read("sequence_items", a.ID)
	okay(t, e)
	var fields map[string]json.RawMessage
	okay(t, json.Unmarshal(raw, &fields))
	fields["futureSafeField"] = json.RawMessage(`{"marker":"future"}`)
	_, e = w.db.Exec("UPDATE sequence_items SET data=? WHERE id=?", string(r30JSON(t, fields)), a.ID)
	okay(t, e)
	before := r30Items(t, w)
	other := map[string][]json.RawMessage{}
	for kind := range tables {
		if kind != "sequence_items" {
			other[kind], e = w.List(kind)
			okay(t, e)
		}
	}
	c := r30Command("2", b.ID, a.ID)
	_, e = w.ExecuteReorderCommand(c)
	okay(t, e)
	r30Invariant(t, w, before)
	raw, e = w.Read("sequence_items", a.ID)
	okay(t, e)
	_, e = w.ExecuteReorderCommand(r30Command("3", b.ID, a.ID))
	okay(t, e)
	same, e := w.Read("sequence_items", a.ID)
	okay(t, e)
	if !bytes.Equal(raw, same) {
		t.Fatal("no-op item bytes changed")
	}
	for kind, before := range other {
		after, e := w.List(kind)
		okay(t, e)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("other entity changed", kind)
		}
	}
	okay(t, w.Reorder("main", []string{a.ID, b.ID}))
	r30Invariant(t, w, before)
	// Stored canonical/receipt corruption fails closed without reconstruction.
	corrupt := r30Command(r30Revision(t, w))
	canonical := reorderCanonical(corrupt)
	_, e = w.db.Exec("INSERT INTO reorder_commands VALUES(?,'main',1,?,?,?,'CONFLICT',5,NULL,'SEQUENCE_SET_CONFLICT','{}')", w.ProjectID, corrupt.ReorderIntentID, canonical, reorderDigest(canonical))
	okay(t, e)
	_, e = w.LookupReorderCommand(corrupt.ReorderIntentID)
	if !errors.Is(e, ErrReorderIntegrity) {
		t.Fatal("fabricated corrupt receipt", e)
	}
	r30Evidence(t, "all-field-preservation", map[string]any{"allJsonFieldsExceptOrderEqual": true, "allIndexedFieldsExceptOrderEqual": true, "updatedAtUnchanged": true, "futureFieldRetained": true, "noOpItemBytesIdentical": true, "otherEntitiesUnchanged": true, "corruptCommandRejected": true})
}

func TestR30CommandDatabaseConstraintsAndDeleteIntegrity(t *testing.T) {
	w, p := placementFixture(t)
	defer w.Close()
	item, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	cmd := r30Command("1", item.ID)
	canonical := reorderCanonical(cmd)
	// SQLite CHECK must reject NULL conflict error_class: NULL is not false.
	_, e = w.db.Exec("INSERT INTO reorder_commands VALUES(?,'main',1,?,?,?,'CONFLICT',1,NULL,NULL,'{}')", w.ProjectID, cmd.ReorderIntentID, canonical, reorderDigest(canonical))
	if e == nil {
		t.Fatal("NULL conflict error class accepted")
	}
	var count int
	okay(t, w.db.QueryRow("SELECT count(*) FROM reorder_commands").Scan(&count))
	if count != 0 {
		t.Fatal("invalid command persisted")
	}
	_, e = w.db.Exec("UPDATE sequence_items SET sequence_id='other' WHERE id=?", item.ID)
	okay(t, e)
	e = w.Delete("sequence_items", item.ID)
	if !errors.Is(e, ErrReorderIntegrity) {
		t.Fatal("delete ignored indexed ownership", e)
	}
	var revision int64
	okay(t, w.db.QueryRow("SELECT revision FROM sequence_state WHERE sequence_id='main'").Scan(&revision))
	if revision != 1 {
		t.Fatal("corrupt delete advanced revision")
	}
	_, e = w.ReadReorderSnapshot()
	if !errors.Is(e, ErrReorderIntegrity) {
		t.Fatal("corrupt snapshot accepted", e)
	}
	_, e = w.Read("sequence_items", item.ID)
	okay(t, e)
	r30Evidence(t, "database-constraints", map[string]any{"nullConflictClassRejected": true, "noInvalidCommandRow": true, "deleteIndexedMismatchRejected": true, "failedDeleteRevisionDelta": 0})
}
