package foundation

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func placementFixture(t *testing.T) (*Workspace, PlacementCommand) {
	w := newWorkspace(t)
	s := makeShot(t, w, "synthetic placement")
	g := makeGeneration(t, w, s.ID, "synthetic request")
	r := makeResult(t, w, g)
	j := archive(t, w, r, []byte("synthetic local bytes"))
	c, e := w.CreateCandidate(s.ID, r.ID, "Local Archive")
	okay(t, e)
	okay(t, w.SelectCandidate(s.ID, c.ID))
	return w, PlacementCommand{1, uuid.NewString(), PlacementOwner{c.ID, s.ID, g.ID, r.ID, j.ID, g.FrozenHash}}
}
func TestR28CommandIdentityAtomicityLegacyAndReopen(t *testing.T) {
	w, p := placementFixture(t)
	a, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if a.Outcome != "COMMITTED" {
		t.Fatal(a)
	}
	b, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if !reflect.DeepEqual(a, b) || count(t, w, "sequence_items") != 1 {
		t.Fatal("same intent did not converge")
	}
	changed := p
	changed.PreparedFrozenHash = strings.Repeat("a", 64)
	_, e = w.ExecutePlacementCommand(changed)
	if !errors.Is(e, ErrPlacementIntent) {
		t.Fatal(e)
	}
	p.PlacementIntentID = uuid.NewString()
	d, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if d.OriginalItem.SequenceItemID == a.OriginalItem.SequenceItemID || d.OriginalItem.OrderIndex != 1 {
		t.Fatal("distinct intent collapsed")
	}
	i, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	k, e := w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	if i.ID == k.ID || i.OrderIndex != 2 || k.OrderIndex != 3 {
		t.Fatal("legacy fresh contract changed")
	}
	reject(t, w.Delete("sequence_items", a.OriginalItem.SequenceItemID))
	_, e = w.db.Exec("UPDATE placement_commands SET canonical_request='bad'")
	reject(t, e)
	_, e = w.db.Exec("INSERT OR REPLACE INTO placement_commands SELECT project_id,sequence_id,protocol_version,intent_id,'changed',request_sha256,outcome,item_id,error_class,receipt FROM placement_commands WHERE intent_id=?", a.PlacementIntentID)
	reject(t, e)
	_, e = w.db.Exec("INSERT OR REPLACE INTO placement_commands SELECT project_id,sequence_id,protocol_version,?,canonical_request,request_sha256,outcome,item_id,error_class,receipt FROM placement_commands WHERE intent_id=?", uuid.NewString(), a.PlacementIntentID)
	reject(t, e)
	untouched, e := w.LookupPlacementCommand(a.PlacementIntentID)
	okay(t, e)
	if !reflect.DeepEqual(untouched, a) {
		t.Fatal("REPLACE changed command")
	}
	_, e = w.db.Exec("CREATE TRIGGER synthetic_failure BEFORE INSERT ON placement_commands BEGIN SELECT RAISE(ABORT,'synthetic failure'); END")
	okay(t, e)
	p.PlacementIntentID = uuid.NewString()
	_, e = w.ExecutePlacementCommand(p)
	reject(t, e)
	if count(t, w, "sequence_items") != 4 {
		t.Fatal("item escaped rollback")
	}
	_, e = w.db.Exec("DROP TRIGGER synthetic_failure")
	okay(t, e)
	// Current Selection changes, but replay does not mutate it or reject past A.
	s := get[Shot](t, w, "shots", p.ShotID)
	g := get[Generation](t, w, "generations", p.GenerationID)
	r := makeResult(t, w, g)
	archive(t, w, r, []byte("synthetic B"))
	cb, e := w.CreateCandidate(s.ID, r.ID, "B")
	okay(t, e)
	okay(t, w.SelectCandidate(s.ID, cb.ID))
	shotBefore := get[Shot](t, w, "shots", s.ID)
	original := p
	original.PlacementIntentID = a.PlacementIntentID
	b, e = w.ExecutePlacementCommand(original)
	okay(t, e)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(shotBefore, get[Shot](t, w, "shots", s.ID)) {
		t.Fatal("replay reselected or changed receipt")
	}
	rejected, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if rejected.Outcome != "REJECTED" {
		t.Fatal(rejected)
	}
	okay(t, w.SelectCandidate(s.ID, p.CandidateID))
	again, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if !reflect.DeepEqual(rejected, again) {
		t.Fatal("rejection not terminal")
	}
	root, project := filepath.Dir(w.Root), w.ProjectID
	okay(t, w.Close())
	ro, e := OpenExisting(root, project, true)
	okay(t, e)
	defer ro.Close()
	found, e := ro.LookupPlacementCommand(a.PlacementIntentID)
	okay(t, e)
	if !reflect.DeepEqual(a, found) {
		t.Fatal(found)
	}
	snapshot, e := ro.ReadMainSequence()
	okay(t, e)
	if len(snapshot.Items) != 4 || snapshot.Items[0].OrderIndex != 0 {
		t.Fatal(snapshot)
	}
	if dst := os.Getenv("HN_R28_SCHEMA_EVIDENCE"); dst != "" {
		rows, e := ro.db.Query("SELECT type,name,sql FROM sqlite_master WHERE name LIKE 'placement_%' ORDER BY name")
		okay(t, e)
		defer rows.Close()
		schema := []map[string]string{}
		for rows.Next() {
			var typ, n, sql string
			okay(t, rows.Scan(&typ, &n, &sql))
			schema = append(schema, map[string]string{"type": typ, "name": n, "sql": sql})
		}
		data, _ := json.MarshalIndent(map[string]any{"schema": schema, "receipt": a, "legacyItems": []SequenceItem{i, k}, "terminalRejected": rejected, "pairRollback": true}, "", "  ")
		okay(t, os.WriteFile(dst, data, 0600))
	}
}
func TestR28ReadonlyAbsentPartialCorruptionAndCapacity(t *testing.T) {
	root := t.TempDir()
	_, e := OpenExisting(root, "absent", true)
	reject(t, e)
	_, e = os.Stat(filepath.Join(root, "absent"))
	if !os.IsNotExist(e) {
		t.Fatal("read created project")
	}
	w, p := placementFixture(t)
	path := filepath.Join(w.Root, "metadata", "hn-extension.sqlite")
	before, e := os.ReadFile(path)
	okay(t, e)
	ro, e := OpenExisting(filepath.Dir(w.Root), w.ProjectID, true)
	okay(t, e)
	_, e = ro.LookupPlacementCommand(p.PlacementIntentID)
	if !errors.Is(e, ErrPlacementProtocol) {
		t.Fatal("absence became NOT_OBSERVED", e)
	}
	_, e = ro.ReadMainSequence()
	okay(t, e)
	_, e = ro.db.Exec("CREATE TABLE should_never_exist(x)")
	reject(t, e)
	ro.Close()
	after, e := os.ReadFile(path)
	okay(t, e)
	if !bytes.Equal(before, after) {
		t.Fatal("readonly changed database")
	}
	// Missing media must not be consulted or reconciled on new read/write paths.
	r := get[Result](t, w, "results", p.ResultID)
	okay(t, os.Remove(filepath.Join(w.Root, filepath.FromSlash(r.ArchivedRelativePath))))
	a, e := w.ExecutePlacementCommand(p)
	okay(t, e)
	if a.Outcome != "COMMITTED" {
		t.Fatal("command read media")
	}
	_, e = w.LookupPlacementCommand(uuid.NewString())
	okay(t, e)
	for n := 1; n < 256; n++ {
		_, e = w.AddSequenceItem("main", p.CandidateID)
		okay(t, e)
	}
	s, e := w.ReadMainSequence()
	okay(t, e)
	if len(s.Items) != 256 {
		t.Fatal("boundary")
	}
	_, e = w.AddSequenceItem("main", p.CandidateID)
	okay(t, e)
	_, e = w.ReadMainSequence()
	if !errors.Is(e, ErrPlacementCapacity) {
		t.Fatal(e)
	}
	// Isolated partial schema must not be repaired by POST.
	v, pp := placementFixture(t)
	_, e = v.db.Exec(placementSchema[0].sql)
	okay(t, e)
	_, e = v.ExecutePlacementCommand(pp)
	if !errors.Is(e, ErrPlacementIntegrity) {
		t.Fatal(e)
	}
	if count(t, v, "sequence_items") != 0 {
		t.Fatal("partial schema admitted item")
	}
	u, pu := placementFixture(t)
	aa, e := u.ExecutePlacementCommand(pu)
	okay(t, e)
	_, e = u.db.Exec("UPDATE sequence_items SET order_index=7 WHERE id=?", aa.OriginalItem.SequenceItemID)
	okay(t, e)
	_, e = u.LookupPlacementCommand(pu.PlacementIntentID)
	if !errors.Is(e, ErrPlacementIntegrity) {
		t.Fatal("indexed/json corruption accepted", e)
	}
	// Simulated externally damaged store, outside supported writer guarantees.
	_, e = u.db.Exec("PRAGMA foreign_keys=OFF; DELETE FROM sequence_items WHERE id=?", aa.OriginalItem.SequenceItemID)
	okay(t, e)
	_, e = u.LookupPlacementCommand(pu.PlacementIntentID)
	if !errors.Is(e, ErrPlacementIntegrity) {
		t.Fatal("missing item recreated/acknowledged", e)
	}
}
func TestR28RealTwoProcess(t *testing.T) {
	if root := os.Getenv("HN_R28_CHILD_ROOT"); root != "" {
		var command PlacementCommand
		okay(t, json.Unmarshal([]byte(os.Getenv("HN_R28_CHILD_COMMAND")), &command))
		w, e := OpenExisting(root, "project-a", false)
		okay(t, e)
		defer w.Close()
		ready := os.Getenv("HN_R28_CHILD_READY")
		okay(t, os.WriteFile(ready, []byte("ready"), 0600))
		deadline := time.Now().Add(20 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(root, "release-processes")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("two-process barrier timed out")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if os.Getenv("HN_R28_CHILD_LEGACY") == "1" {
			_, e = w.AddSequenceItem("main", command.CandidateID)
		} else {
			_, e = w.ExecutePlacementCommand(command)
		}
		okay(t, e)
		return
	}
	for _, mode := range []string{"same-first-schema", "different-intents", "legacy-new"} {
		t.Run(mode, func(t *testing.T) {
			w, p := placementFixture(t)
			root := filepath.Dir(w.Root)
			okay(t, w.Close())
			commands := []PlacementCommand{p, p}
			if mode == "different-intents" {
				commands[1].PlacementIntentID = uuid.NewString()
			}
			children := []*exec.Cmd{}
			logs := []*bytes.Buffer{}
			for n, c := range commands {
				b, _ := json.Marshal(c)
				cmd := exec.Command(os.Args[0], "-test.run=^TestR28RealTwoProcess$", "-test.v")
				cmd.Env = append(os.Environ(), "HN_R28_CHILD_ROOT="+root, "HN_R28_CHILD_COMMAND="+string(b), "HN_R28_CHILD_READY="+filepath.Join(root, []string{"ready-a", "ready-b"}[n]))
				if mode == "legacy-new" && n == 0 {
					cmd.Env = append(cmd.Env, "HN_R28_CHILD_LEGACY=1")
				}
				log := &bytes.Buffer{}
				cmd.Stdout = log
				cmd.Stderr = log
				children = append(children, cmd)
				logs = append(logs, log)
				okay(t, cmd.Start())
			}
			deadline := time.Now().Add(20 * time.Second)
			for {
				_, a := os.Stat(filepath.Join(root, "ready-a"))
				_, b := os.Stat(filepath.Join(root, "ready-b"))
				if a == nil && b == nil {
					break
				}
				if time.Now().After(deadline) {
					for _, cmd := range children {
						cmd.Process.Kill()
					}
					t.Fatal("children did not rendezvous")
				}
				time.Sleep(10 * time.Millisecond)
			}
			okay(t, os.WriteFile(filepath.Join(root, "release-processes"), []byte("start"), 0600))
			for n, cmd := range children {
				if e := cmd.Wait(); e != nil {
					t.Fatal(e, logs[n].String())
				}
			}
			read, e := OpenExisting(root, "project-a", true)
			okay(t, e)
			defer read.Close()
			snapshot, e := read.ReadMainSequence()
			okay(t, e)
			expected := 2
			if mode == "same-first-schema" {
				expected = 1
			}
			if len(snapshot.Items) != expected {
				t.Fatal(mode, snapshot)
			}
			for n, i := range snapshot.Items {
				if i.OrderIndex != n {
					t.Fatal("duplicate order", snapshot)
				}
			}
			data, _ := json.Marshal(map[string]any{"scenario": mode, "childProcesses": 2, "processIDs": []int{children[0].Process.Pid, children[1].Process.Pid}, "bothOpenedBeforeRelease": true, "itemCount": expected, "items": snapshot.Items, "sameIntentOneEffect": mode == "same-first-schema"})
			t.Log(string(data))
			if dir := os.Getenv("HN_R28_PROCESS_EVIDENCE"); dir != "" {
				okay(t, os.MkdirAll(dir, 0700))
				okay(t, os.WriteFile(filepath.Join(dir, mode+".json"), data, 0600))
			}
		})
	}
}
func TestR28SchemaFailureRollsBackExtensionAndItem(t *testing.T) {
	w, p := placementFixture(t)
	_, e := w.db.Exec("CREATE TRIGGER synthetic_item_failure BEFORE INSERT ON sequence_items BEGIN SELECT RAISE(ABORT,'synthetic'); END")
	okay(t, e)
	before, _ := w.Read("generations", p.GenerationID)
	_, e = w.ExecutePlacementCommand(p)
	reject(t, e)
	var n int
	okay(t, w.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name LIKE 'placement_%'").Scan(&n))
	if n != 0 || count(t, w, "sequence_items") != 0 {
		t.Fatal("partial install/pair")
	}
	after, _ := w.Read("generations", p.GenerationID)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("generation changed")
	}
}

func TestR28OwnershipIndexedFactsUnknownSchemaAndDuplicateOrder(t *testing.T) {
	for _, field := range []string{"candidate", "shot", "generation", "result", "job", "hash"} {
		t.Run(field, func(t *testing.T) {
			w, p := placementFixture(t)
			switch field {
			case "candidate":
				p.CandidateID = "missing"
			case "shot":
				p.ShotID = "missing"
			case "generation":
				p.GenerationID = "missing"
			case "result":
				p.ResultID = "missing"
			case "job":
				p.ArchiveJobID = "missing"
			case "hash":
				p.PreparedFrozenHash = strings.Repeat("a", 64)
			}
			r, e := w.ExecutePlacementCommand(p)
			okay(t, e)
			if r.Outcome != "REJECTED" || count(t, w, "sequence_items") != 0 {
				t.Fatal("owner not checked", r)
			}
		})
	}
	w, p := placementFixture(t)
	_, e := w.db.Exec("UPDATE candidates SET generation_id=NULL WHERE id=?", p.CandidateID)
	reject(t, e)
	_, e = w.db.Exec("UPDATE candidates SET shot_id='missing' WHERE id=?", p.CandidateID)
	reject(t, e)
	// Damage only the indexed result FK while retaining valid record JSON.
	s := makeShot(t, w, "other shot")
	g := makeGeneration(t, w, s.ID, "other generation")
	_, e = w.db.Exec("UPDATE candidates SET generation_id=? WHERE id=?", g.ID, p.CandidateID)
	reject(t, e)
	_, e = w.db.Exec("UPDATE generations SET shot_id=? WHERE id=?", s.ID, p.GenerationID)
	okay(t, e)
	_, e = w.ExecutePlacementCommand(p)
	if !errors.Is(e, ErrPlacementIntegrity) {
		t.Fatal("SQL/JSON disagreement admitted", e)
	}
	v, pv := placementFixture(t)
	_, e = v.db.Exec("CREATE TABLE placement_unknown(x)")
	okay(t, e)
	_, e = v.ExecutePlacementCommand(pv)
	if !errors.Is(e, ErrPlacementIntegrity) {
		t.Fatal("unknown extension repaired", e)
	}
	u, pu := placementFixture(t)
	a, e := u.AddSequenceItem("main", pu.CandidateID)
	okay(t, e)
	b, e := u.AddSequenceItem("main", pu.CandidateID)
	okay(t, e)
	b.OrderIndex = a.OrderIndex
	raw, _ := json.Marshal(b)
	_, e = u.db.Exec("UPDATE sequence_items SET order_index=?,data=? WHERE id=?", b.OrderIndex, string(raw), b.ID)
	okay(t, e)
	_, e = u.ReadMainSequence()
	if !errors.Is(e, ErrPlacementIntegrity) {
		t.Fatal("duplicate positions accepted", e)
	}
	// A historical immutable receipt survives later supported reordering and
	// continues to return its ORIGINAL append position, not current order.
	x, px := placementFixture(t)
	first, e := x.ExecutePlacementCommand(px)
	okay(t, e)
	px.PlacementIntentID = uuid.NewString()
	second, e := x.ExecutePlacementCommand(px)
	okay(t, e)
	okay(t, x.Reorder("main", []string{second.OriginalItem.SequenceItemID, first.OriginalItem.SequenceItemID}))
	original := px
	original.PlacementIntentID = first.PlacementIntentID
	again, e := x.ExecutePlacementCommand(original)
	okay(t, e)
	if !reflect.DeepEqual(first, again) {
		t.Fatal("receipt changed after reorder")
	}
	snap, e := x.ReadMainSequence()
	okay(t, e)
	if snap.Items[0].SequenceItemID != second.OriginalItem.SequenceItemID {
		t.Fatal("snapshot not current metadata")
	}
}
