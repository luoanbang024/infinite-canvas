package foundation

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrReorderInput       = errors.New("REORDER_INPUT_INVALID")
	ErrReorderIntegrity   = errors.New("REORDER_INTEGRITY_CONFLICT")
	ErrReorderIntent      = errors.New("REORDER_INTENT_CONFLICT")
	ErrReorderProtocol    = errors.New("REORDER_PROTOCOL_NOT_INITIALIZED")
	ErrReorderExhausted   = errors.New("SEQUENCE_REVISION_EXHAUSTED")
	ErrReorderUnavailable = errors.New("REORDER_STORE_UNAVAILABLE")
)

type ReorderCommand struct {
	ProtocolVersion        int      `json:"protocolVersion"`
	ReorderIntentID        string   `json:"reorderIntentId"`
	ExpectedRevision       string   `json:"expectedRevision"`
	DesiredSequenceItemIDs []string `json:"desiredSequenceItemIds"`
}
type reorderPayload struct {
	ExpectedRevision       string   `json:"expectedRevision"`
	DesiredSequenceItemIDs []string `json:"desiredSequenceItemIds"`
}
type ReorderReceipt struct {
	ProtocolVersion        int      `json:"protocolVersion"`
	ProjectID              string   `json:"projectId"`
	SequenceID             string   `json:"sequenceId"`
	ReorderIntentID        string   `json:"reorderIntentId"`
	CanonicalRequestHash   string   `json:"canonicalRequestHash"`
	ExpectedRevision       string   `json:"expectedRevision"`
	Outcome                string   `json:"outcome"`
	ObservedRevision       string   `json:"observedRevision"`
	AppliedRevision        *string  `json:"appliedRevision"`
	DesiredSequenceItemIDs []string `json:"desiredSequenceItemIds"`
	ErrorClass             *string  `json:"errorClass"`
}
type ReorderState struct {
	ProtocolVersion  int    `json:"protocolVersion"`
	ProjectID        string `json:"projectId"`
	SequenceID       string `json:"sequenceId"`
	SequenceRevision string `json:"sequenceRevision"`
}
type ReorderSnapshot struct {
	ProtocolVersion  int             `json:"protocolVersion"`
	ProjectID        string          `json:"projectId"`
	SequenceID       string          `json:"sequenceId"`
	ItemsComplete    bool            `json:"itemsComplete"`
	SequenceRevision string          `json:"sequenceRevision"`
	Items            []PlacementItem `json:"items"`
}
type ReorderLookup struct {
	ProtocolVersion int    `json:"protocolVersion"`
	ProjectID       string `json:"projectId"`
	SequenceID      string `json:"sequenceId"`
	ReorderIntentID string `json:"reorderIntentId"`
	Outcome         string `json:"outcome"`
}

func ParseSequenceRevision(s string) (int64, bool) {
	n, e := strconv.ParseInt(s, 10, 64)
	return n, e == nil && n >= 0 && strconv.FormatInt(n, 10) == s
}
func validReorderIDs(ids []string) bool {
	if ids == nil || len(ids) > 256 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validName(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func ValidReorderCommand(c ReorderCommand) bool {
	_, ok := ParseSequenceRevision(c.ExpectedRevision)
	return c.ProtocolVersion == 1 && placementUUID.MatchString(c.ReorderIntentID) && ok && validReorderIDs(c.DesiredSequenceItemIDs)
}
func reorderCanonical(c ReorderCommand) string {
	b, _ := json.Marshal(reorderPayload{c.ExpectedRevision, c.DesiredSequenceItemIDs})
	return string(b)
}
func reorderDigest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

var reorderSchema = []struct{ name, sql string }{
	{"reorder_protocol", `CREATE TABLE reorder_protocol (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL CHECK(version=1))`},
	{"sequence_state", `CREATE TABLE sequence_state (project_id TEXT NOT NULL REFERENCES project(id), sequence_id TEXT NOT NULL CHECK(sequence_id='main'), revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=0), PRIMARY KEY(project_id,sequence_id))`},
	{"reorder_commands", `CREATE TABLE reorder_commands (project_id TEXT NOT NULL REFERENCES project(id), sequence_id TEXT NOT NULL CHECK(sequence_id='main'), protocol_version INTEGER NOT NULL CHECK(protocol_version=1), intent_id TEXT NOT NULL, canonical_request TEXT NOT NULL, request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64), outcome TEXT NOT NULL CHECK(outcome IN ('COMMITTED','CONFLICT')), observed_revision INTEGER NOT NULL CHECK(typeof(observed_revision)='integer' AND observed_revision>=0), applied_revision INTEGER, error_class TEXT, receipt TEXT NOT NULL, PRIMARY KEY(project_id,sequence_id,protocol_version,intent_id), CHECK((outcome='COMMITTED' AND error_class IS NULL AND typeof(applied_revision)='integer' AND observed_revision<9223372036854775807 AND applied_revision=observed_revision+1) OR (outcome='CONFLICT' AND applied_revision IS NULL AND error_class IS NOT NULL AND error_class IN ('SEQUENCE_REVISION_CONFLICT','SEQUENCE_SET_CONFLICT'))))`},
	{"reorder_commands_no_update", `CREATE TRIGGER reorder_commands_no_update BEFORE UPDATE ON reorder_commands BEGIN SELECT RAISE(ABORT,'immutable reorder command'); END`},
	{"reorder_commands_no_delete", `CREATE TRIGGER reorder_commands_no_delete BEFORE DELETE ON reorder_commands BEGIN SELECT RAISE(ABORT,'retained reorder command'); END`},
	{"reorder_commands_no_replace", `CREATE TRIGGER reorder_commands_no_replace BEFORE INSERT ON reorder_commands WHEN EXISTS(SELECT 1 FROM reorder_commands WHERE project_id=NEW.project_id AND sequence_id=NEW.sequence_id AND protocol_version=NEW.protocol_version AND intent_id=NEW.intent_id) BEGIN SELECT RAISE(ABORT,'immutable reorder identity'); END`},
}

// Called only within a pinned write/read transaction. GET never installs.
func (w *Workspace) reorderExtension(q placementConnection, install bool) error {
	var count int
	if q.QueryRow("SELECT count(*) FROM sqlite_master WHERE name GLOB 'reorder_*' OR name='sequence_state'").Scan(&count) != nil {
		return ErrReorderIntegrity
	}
	if count == 0 {
		if !install {
			return ErrReorderProtocol
		}
		if _, e := w.reorderItems(q, "main", 0); e != nil {
			return e
		}
		for _, v := range reorderSchema {
			if _, e := q.Exec(v.sql); e != nil {
				return e
			}
		}
		if _, e := q.Exec("INSERT INTO reorder_protocol VALUES(1,1)"); e != nil {
			return e
		}
		if _, e := q.Exec("INSERT INTO sequence_state VALUES(?,'main',0)", w.ProjectID); e != nil {
			return e
		}
	} else if count != len(reorderSchema) {
		return ErrReorderIntegrity
	}
	for _, v := range reorderSchema {
		var text string
		if q.QueryRow("SELECT sql FROM sqlite_master WHERE name=?", v.name).Scan(&text) != nil || strings.TrimSpace(text) != v.sql {
			return ErrReorderIntegrity
		}
	}
	var version, total, id int
	if q.QueryRow("SELECT count(*),coalesce(max(id),0),coalesce(max(version),0) FROM reorder_protocol").Scan(&total, &id, &version) != nil || total != 1 || id != 1 || version != 1 {
		return ErrReorderIntegrity
	}
	_, e := w.reorderRevision(q)
	return e
}
func (w *Workspace) reorderRevision(q placementConnection) (int64, error) {
	var total int
	if q.QueryRow("SELECT count(*) FROM sequence_state").Scan(&total) != nil || total != 1 {
		return 0, ErrReorderIntegrity
	}
	var revision int64
	var kind string
	if q.QueryRow("SELECT revision,typeof(revision) FROM sequence_state WHERE project_id=? AND sequence_id='main'", w.ProjectID).Scan(&revision, &kind) != nil || kind != "integer" || revision < 0 {
		return 0, ErrReorderIntegrity
	}
	return revision, nil
}
func (w *Workspace) reorderBeforeMutation(q placementConnection, sequence string) (int64, error) {
	if sequence != "main" {
		return 0, nil
	}
	if e := w.reorderExtension(q, true); e != nil {
		return 0, e
	}
	n, e := w.reorderRevision(q)
	if e != nil {
		return 0, e
	}
	if n == math.MaxInt64 {
		return 0, ErrReorderExhausted
	}
	return n, nil
}
func (w *Workspace) reorderAdvance(q placementConnection, sequence string, previous int64) error {
	if sequence != "main" {
		return nil
	}
	r, e := q.Exec("UPDATE sequence_state SET revision=? WHERE project_id=? AND sequence_id='main' AND revision=?", previous+1, w.ProjectID, previous)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil || n != 1 {
		return ErrReorderIntegrity
	}
	return nil
}

// Validates record and indexed facts without media or mutable selection reads.
func (w *Workspace) reorderItems(q placementConnection, sequence string, capacity int) ([]SequenceItem, error) {
	var count int
	if q.QueryRow("SELECT count(*) FROM sequence_items WHERE sequence_id=?", sequence).Scan(&count) != nil {
		return nil, ErrReorderIntegrity
	}
	if capacity > 0 && count > capacity {
		return nil, ErrPlacementCapacity
	}
	rows, e := q.Query("SELECT id FROM sequence_items ORDER BY order_index,id")
	if e != nil {
		return nil, ErrReorderIntegrity
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			return nil, ErrReorderIntegrity
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, ErrReorderIntegrity
	}
	items := []SequenceItem{}
	seen := map[int]bool{}
	for _, id := range ids {
		var i SequenceItem
		if read(q, "sequence_items", id, &i) != nil || w.check(i.Identity) != nil || i.ID != id || !validName(i.SequenceID) || !validName(i.ShotID) || !validName(i.CandidateID) || !validName(i.ResultID) || i.OrderIndex < 0 || !placementTime(i.CreatedAt) || !placementTime(i.UpdatedAt) {
			return nil, ErrReorderIntegrity
		}
		if w.placementRecord(q, "sequence_items", id, &i, []string{"sequence_id", "order_index", "shot_id", "candidate_id", "result_id"}, []any{i.SequenceID, int64(i.OrderIndex), i.ShotID, i.CandidateID, i.ResultID}) != nil {
			return nil, ErrReorderIntegrity
		}
		if i.SequenceID != sequence {
			continue
		}
		if seen[i.OrderIndex] {
			return nil, ErrReorderIntegrity
		}
		seen[i.OrderIndex] = true
		items = append(items, i)
	}
	if len(items) != count {
		return nil, ErrReorderIntegrity
	}
	sort.Slice(items, func(a, b int) bool { return items[a].OrderIndex < items[b].OrderIndex })
	return items, nil
}
func (w *Workspace) reorderApply(q placementConnection, items []SequenceItem, ids []string) ([]SequenceItem, error) {
	if len(items) != len(ids) {
		return nil, ErrReorderInput
	}
	byID := map[string]SequenceItem{}
	for _, i := range items {
		byID[i.ID] = i
	}
	result := []SequenceItem{}
	for index, id := range ids {
		i, ok := byID[id]
		if !ok {
			return nil, ErrReorderInput
		}
		delete(byID, id)
		if i.OrderIndex != index {
			var raw string
			var fields map[string]json.RawMessage
			if q.QueryRow("SELECT data FROM sequence_items WHERE id=?", id).Scan(&raw) != nil || json.Unmarshal([]byte(raw), &fields) != nil {
				return nil, ErrReorderIntegrity
			}
			fields["orderIndex"], _ = json.Marshal(index)
			b, e := json.Marshal(fields)
			if e != nil {
				return nil, e
			}
			r, e := q.Exec("UPDATE sequence_items SET data=?,order_index=? WHERE id=?", string(b), index, id)
			if e != nil {
				return nil, e
			}
			n, e := r.RowsAffected()
			if e != nil || n != 1 {
				return nil, ErrReorderIntegrity
			}
		}
		i.OrderIndex = index
		result = append(result, i)
	}
	return result, nil
}
func (w *Workspace) InitializeReorderProtocol() (ReorderState, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	state := ReorderState{1, w.ProjectID, "main", ""}
	e := w.placementWrite(func(q placementConnection) error {
		if e := w.reorderExtension(q, true); e != nil {
			return e
		}
		n, e := w.reorderRevision(q)
		state.SequenceRevision = strconv.FormatInt(n, 10)
		return e
	})
	return state, e
}
func (w *Workspace) ReadReorderSnapshot() (ReorderSnapshot, error) {
	s := ReorderSnapshot{1, w.ProjectID, "main", true, "", []PlacementItem{}}
	e := w.placementRead(func(q placementConnection) error {
		if e := w.reorderExtension(q, false); e != nil {
			return e
		}
		n, e := w.reorderRevision(q)
		if e != nil {
			return e
		}
		items, e := w.reorderItems(q, "main", 256)
		if e != nil {
			return e
		}
		s.SequenceRevision = strconv.FormatInt(n, 10)
		for _, i := range items {
			s.Items = append(s.Items, PlacementItemFacts(i))
		}
		return nil
	})
	return s, e
}

// Immutable historical receipt validation does not require current order equality.
func (w *Workspace) reorderCommandRead(q placementConnection, intent string) (*ReorderReceipt, string, error) {
	var canonical, h, outcome, raw string
	var observed int64
	var applied sql.NullInt64
	var class sql.NullString
	e := q.QueryRow("SELECT canonical_request,request_sha256,outcome,observed_revision,applied_revision,error_class,receipt FROM reorder_commands WHERE project_id=? AND sequence_id='main' AND protocol_version=1 AND intent_id=?", w.ProjectID, intent).Scan(&canonical, &h, &outcome, &observed, &applied, &class, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, "", nil
	}
	if e != nil {
		return nil, "", ErrReorderIntegrity
	}
	var p reorderPayload
	var r ReorderReceipt
	if json.Unmarshal([]byte(canonical), &p) != nil || json.Unmarshal([]byte(raw), &r) != nil {
		return nil, "", ErrReorderIntegrity
	}
	c := ReorderCommand{1, intent, p.ExpectedRevision, p.DesiredSequenceItemIDs}
	b, _ := json.Marshal(r)
	revision, e := w.reorderRevision(q)
	if e != nil || !ValidReorderCommand(c) || reorderCanonical(c) != canonical || reorderDigest(canonical) != h || string(b) != raw || r.ProtocolVersion != 1 || r.ProjectID != w.ProjectID || r.SequenceID != "main" || r.ReorderIntentID != intent || r.CanonicalRequestHash != h || r.ExpectedRevision != p.ExpectedRevision || !reflect.DeepEqual(r.DesiredSequenceItemIDs, p.DesiredSequenceItemIDs) || r.Outcome != outcome || observed < 0 || observed > revision || r.ObservedRevision != strconv.FormatInt(observed, 10) {
		return nil, "", ErrReorderIntegrity
	}
	if outcome == "COMMITTED" {
		if class.Valid || r.ErrorClass != nil || !applied.Valid || observed == math.MaxInt64 || applied.Int64 != observed+1 || applied.Int64 > revision || r.AppliedRevision == nil || *r.AppliedRevision != strconv.FormatInt(applied.Int64, 10) || r.ExpectedRevision != r.ObservedRevision {
			return nil, "", ErrReorderIntegrity
		}
	} else if outcome == "CONFLICT" {
		if applied.Valid || r.AppliedRevision != nil || !class.Valid || r.ErrorClass == nil || *r.ErrorClass != class.String || (class.String != "SEQUENCE_REVISION_CONFLICT" && class.String != "SEQUENCE_SET_CONFLICT") || (class.String == "SEQUENCE_REVISION_CONFLICT" && r.ExpectedRevision == r.ObservedRevision) || (class.String == "SEQUENCE_SET_CONFLICT" && r.ExpectedRevision != r.ObservedRevision) {
			return nil, "", ErrReorderIntegrity
		}
	} else {
		return nil, "", ErrReorderIntegrity
	}
	return &r, canonical, nil
}
func (w *Workspace) ExecuteReorderCommand(c ReorderCommand) (ReorderReceipt, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var receipt ReorderReceipt
	if !ValidReorderCommand(c) {
		return receipt, ErrReorderInput
	}
	c.DesiredSequenceItemIDs = append([]string{}, c.DesiredSequenceItemIDs...)
	canonical := reorderCanonical(c)
	e := w.placementWrite(func(q placementConnection) error {
		if e := w.reorderExtension(q, true); e != nil {
			return e
		}
		old, p, e := w.reorderCommandRead(q, c.ReorderIntentID)
		if e != nil {
			return e
		}
		if old != nil {
			if p != canonical {
				return ErrReorderIntent
			}
			receipt = *old
			return nil
		}
		n, e := w.reorderRevision(q)
		if e != nil {
			return e
		}
		items, e := w.reorderItems(q, "main", 0)
		if e != nil {
			return e
		}
		receipt = ReorderReceipt{ProtocolVersion: 1, ProjectID: w.ProjectID, SequenceID: "main", ReorderIntentID: c.ReorderIntentID, CanonicalRequestHash: reorderDigest(canonical), ExpectedRevision: c.ExpectedRevision, Outcome: "CONFLICT", ObservedRevision: strconv.FormatInt(n, 10), DesiredSequenceItemIDs: c.DesiredSequenceItemIDs}
		class := ""
		byID := map[string]bool{}
		for _, i := range items {
			byID[i.ID] = true
		}
		if c.ExpectedRevision != receipt.ObservedRevision {
			class = "SEQUENCE_REVISION_CONFLICT"
		} else {
			valid := len(items) == len(c.DesiredSequenceItemIDs)
			for _, id := range c.DesiredSequenceItemIDs {
				valid = valid && byID[id]
			}
			if !valid {
				class = "SEQUENCE_SET_CONFLICT"
			}
		}
		var applied any
		var errorClass any
		if class != "" {
			receipt.ErrorClass = &class
			errorClass = class
		} else {
			if n == math.MaxInt64 {
				return ErrReorderExhausted
			}
			if _, e := w.reorderApply(q, items, c.DesiredSequenceItemIDs); e != nil {
				return e
			}
			if e := w.reorderAdvance(q, "main", n); e != nil {
				return e
			}
			next := strconv.FormatInt(n+1, 10)
			receipt.Outcome = "COMMITTED"
			receipt.AppliedRevision = &next
			applied = n + 1
		}
		raw, _ := json.Marshal(receipt)
		_, e = q.Exec("INSERT INTO reorder_commands VALUES(?,?,?,?,?,?,?,?,?,?,?)", w.ProjectID, "main", 1, c.ReorderIntentID, canonical, receipt.CanonicalRequestHash, receipt.Outcome, n, applied, errorClass, string(raw))
		return e
	})
	return receipt, e
}
func (w *Workspace) LookupReorderCommand(intent string) (any, error) {
	if !placementUUID.MatchString(intent) {
		return nil, ErrReorderInput
	}
	var data any
	e := w.placementRead(func(q placementConnection) error {
		if e := w.reorderExtension(q, false); e != nil {
			return e
		}
		r, _, e := w.reorderCommandRead(q, intent)
		if e != nil {
			return e
		}
		if r == nil {
			data = ReorderLookup{1, w.ProjectID, "main", intent, "NOT_OBSERVED"}
		} else {
			data = *r
		}
		return nil
	})
	return data, e
}

// Legacy wire shape stays unchanged; authority/DTO captured under writer lock.
func (w *Workspace) ReorderSequenceItems(sequence string, ids []string) ([]SequenceItem, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !validName(sequence) || ids == nil {
		return nil, ErrReorderInput
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validName(id) || seen[id] {
			return nil, ErrReorderInput
		}
		seen[id] = true
	}
	ids = append([]string{}, ids...)
	var captured []SequenceItem
	e := w.placementWrite(func(q placementConnection) error {
		items, e := w.reorderItems(q, sequence, 0)
		if e != nil {
			return e
		}
		if len(items) != len(ids) {
			return ErrReorderInput
		}
		byID := map[string]bool{}
		for _, i := range items {
			byID[i.ID] = true
		}
		for _, id := range ids {
			if !byID[id] {
				return ErrReorderInput
			}
		}
		previous, e := w.reorderBeforeMutation(q, sequence)
		if e != nil {
			return e
		}
		captured, e = w.reorderApply(q, items, ids)
		if e != nil {
			return e
		}
		return w.reorderAdvance(q, sequence, previous)
	})
	return captured, e
}

func (w *Workspace) deleteSequenceItem(q placementConnection, id string) error {
	var i SequenceItem
	if err := read(q, "sequence_items", id, &i); err != nil {
		return err
	}
	if w.check(i.Identity) != nil || i.ID != id {
		return ErrReorderIntegrity
	}
	if !validName(i.SequenceID) || i.OrderIndex < 0 || w.placementRecord(q, "sequence_items", id, &i, []string{"sequence_id", "order_index", "shot_id", "candidate_id", "result_id"}, []any{i.SequenceID, int64(i.OrderIndex), i.ShotID, i.CandidateID, i.ResultID}) != nil {
		return ErrReorderIntegrity
	}
	previous, err := w.reorderBeforeMutation(q, i.SequenceID)
	if err != nil {
		return err
	}
	result, err := q.Exec("DELETE FROM sequence_items WHERE id=? AND project_id=?", id, w.ProjectID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return w.reorderAdvance(q, i.SequenceID, previous)
}
