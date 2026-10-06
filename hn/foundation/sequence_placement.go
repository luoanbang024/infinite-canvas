package foundation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrPlacementInput       = errors.New("PLACEMENT_INPUT_INVALID")
	ErrPlacementIntegrity   = errors.New("PLACEMENT_INTEGRITY_CONFLICT")
	ErrPlacementIntent      = errors.New("PLACEMENT_INTENT_CONFLICT")
	ErrPlacementProtocol    = errors.New("PLACEMENT_PROTOCOL_NOT_INITIALIZED")
	ErrPlacementUnavailable = errors.New("PLACEMENT_STORE_UNAVAILABLE")
	ErrPlacementCapacity    = errors.New("READ_CAPACITY_EXCEEDED")
)
var placementUUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
var placementHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

type PlacementOwner struct {
	CandidateID        string `json:"candidateId"`
	ShotID             string `json:"shotId"`
	GenerationID       string `json:"generationId"`
	ResultID           string `json:"resultId"`
	ArchiveJobID       string `json:"archiveJobId"`
	PreparedFrozenHash string `json:"preparedFrozenHash"`
}
type PlacementCommand struct {
	ProtocolVersion   int    `json:"protocolVersion"`
	PlacementIntentID string `json:"placementIntentId"`
	PlacementOwner
}

// PlacementItem is the existing safe HTTP shape, independent of record JSON.
type PlacementItem struct {
	SequenceItemID string `json:"sequenceItemId"`
	ProjectID      string `json:"projectId"`
	SequenceID     string `json:"sequenceId"`
	OrderIndex     int    `json:"orderIndex"`
	ShotID         string `json:"shotId"`
	CandidateID    string `json:"candidateId"`
	ResultID       string `json:"resultId"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}
type PlacementReceipt struct {
	ProtocolVersion   int            `json:"protocolVersion"`
	ProjectID         string         `json:"projectId"`
	SequenceID        string         `json:"sequenceId"`
	PlacementIntentID string         `json:"placementIntentId"`
	Owner             PlacementOwner `json:"owner"`
	Outcome           string         `json:"outcome"`
	OriginalItem      *PlacementItem `json:"originalItem"`
	ErrorClass        *string        `json:"errorClass"`
}
type PlacementLookup struct {
	ProtocolVersion   int    `json:"protocolVersion"`
	ProjectID         string `json:"projectId"`
	SequenceID        string `json:"sequenceId"`
	PlacementIntentID string `json:"placementIntentId"`
	Outcome           string `json:"outcome"`
}
type SequenceSnapshot struct {
	ProtocolVersion int             `json:"protocolVersion"`
	ProjectID       string          `json:"projectId"`
	SequenceID      string          `json:"sequenceId"`
	ItemsComplete   bool            `json:"itemsComplete"`
	Items           []PlacementItem `json:"items"`
}

type placementConnection struct{ *sql.Conn }

func (q placementConnection) QueryRow(s string, a ...any) *sql.Row {
	return q.QueryRowContext(context.Background(), s, a...)
}
func (q placementConnection) Exec(s string, a ...any) (sql.Result, error) {
	return q.ExecContext(context.Background(), s, a...)
}
func (q placementConnection) Query(s string, a ...any) (*sql.Rows, error) {
	return q.QueryContext(context.Background(), s, a...)
}

func (w *Workspace) placementWrite(f func(placementConnection) error) error {
	c, err := w.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer c.Close()
	q := placementConnection{c}
	if _, err = q.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA synchronous=FULL;"); err != nil {
		return err
	}
	if _, err = q.Exec("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer q.Exec("ROLLBACK")
	if err = f(q); err != nil {
		return err
	}
	_, err = q.Exec("COMMIT")
	return err
}
func (w *Workspace) placementRead(f func(placementConnection) error) error {
	c, err := w.db.Conn(context.Background())
	if err != nil {
		return ErrPlacementUnavailable
	}
	defer c.Close()
	q := placementConnection{c}
	if _, err = q.Exec("BEGIN"); err != nil {
		return ErrPlacementUnavailable
	}
	defer q.Exec("ROLLBACK")
	return f(q)
}

var placementSchema = []struct{ name, sql string }{
	{"placement_protocol", `CREATE TABLE placement_protocol (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL CHECK(version=1))`},
	{"placement_commands", `CREATE TABLE placement_commands (project_id TEXT NOT NULL REFERENCES project(id), sequence_id TEXT NOT NULL CHECK(sequence_id='main'), protocol_version INTEGER NOT NULL CHECK(protocol_version=1), intent_id TEXT NOT NULL, canonical_request TEXT NOT NULL, request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64), outcome TEXT NOT NULL CHECK(outcome IN ('COMMITTED','REJECTED')), item_id TEXT UNIQUE REFERENCES sequence_items(id) ON DELETE RESTRICT, error_class TEXT, receipt TEXT NOT NULL, PRIMARY KEY(project_id,sequence_id,protocol_version,intent_id), CHECK((outcome='COMMITTED' AND item_id IS NOT NULL AND error_class IS NULL) OR (outcome='REJECTED' AND item_id IS NULL AND error_class='OWNERSHIP_REJECTED')))`},
	{"placement_commands_no_update", `CREATE TRIGGER placement_commands_no_update BEFORE UPDATE ON placement_commands BEGIN SELECT RAISE(ABORT,'immutable placement command'); END`},
	{"placement_commands_no_delete", `CREATE TRIGGER placement_commands_no_delete BEFORE DELETE ON placement_commands BEGIN SELECT RAISE(ABORT,'retained placement command'); END`},
	{"placement_commands_no_replace", `CREATE TRIGGER placement_commands_no_replace BEFORE INSERT ON placement_commands WHEN EXISTS(SELECT 1 FROM placement_commands WHERE (project_id=NEW.project_id AND sequence_id=NEW.sequence_id AND protocol_version=NEW.protocol_version AND intent_id=NEW.intent_id) OR (NEW.item_id IS NOT NULL AND item_id=NEW.item_id)) BEGIN SELECT RAISE(ABORT,'immutable placement identity'); END`},
}

func placementExtension(q placementConnection, install bool) error {
	var count int
	if q.QueryRow("SELECT count(*) FROM sqlite_master WHERE name GLOB 'placement_*'").Scan(&count) != nil {
		return ErrPlacementIntegrity
	}
	if count == 0 {
		if !install {
			return ErrPlacementProtocol
		}
		for _, v := range placementSchema {
			if _, err := q.Exec(v.sql); err != nil {
				return err
			}
		}
		if _, err := q.Exec("INSERT INTO placement_protocol VALUES(1,1)"); err != nil {
			return err
		}
	} else if count != len(placementSchema) {
		return ErrPlacementIntegrity
	}
	for _, v := range placementSchema {
		var sqlText string
		if q.QueryRow("SELECT sql FROM sqlite_master WHERE name=?", v.name).Scan(&sqlText) != nil || strings.TrimSpace(sqlText) != v.sql {
			return ErrPlacementIntegrity
		}
	}
	var version, total int
	if q.QueryRow("SELECT count(*),coalesce(max(version),0) FROM placement_protocol").Scan(&total, &version) != nil || total != 1 || version != 1 {
		return ErrPlacementIntegrity
	}
	return nil
}
func ValidPlacementCommand(c PlacementCommand) bool {
	return c.ProtocolVersion == 1 && placementUUID.MatchString(c.PlacementIntentID) && validName(c.CandidateID) && validName(c.ShotID) && validName(c.GenerationID) && validName(c.ResultID) && validName(c.ArchiveJobID) && placementHash.MatchString(c.PreparedFrozenHash)
}
func PlacementItemFacts(i SequenceItem) PlacementItem {
	return PlacementItem{i.ID, i.ProjectID, i.SequenceID, i.OrderIndex, i.ShotID, i.CandidateID, i.ResultID, i.CreatedAt, i.UpdatedAt}
}
func placementTime(t string) bool {
	_, err := time.Parse(time.RFC3339Nano, t)
	return err == nil && regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$`).MatchString(t)
}

func (w *Workspace) appendPlacement(q placementConnection, sequence string, c Candidate) (SequenceItem, error) {
	var index int
	if err := q.QueryRow("SELECT coalesce(max(order_index)+1,0) FROM sequence_items WHERE project_id=? AND sequence_id=?", w.ProjectID, sequence).Scan(&index); err != nil {
		return SequenceItem{}, err
	}
	i := SequenceItem{Identity: w.identity(), SequenceID: sequence, OrderIndex: index, ShotID: c.ShotID, CandidateID: c.ID, ResultID: c.ResultID}
	return i, insert(q, "sequence_items", i.Identity, i, "sequence_id,order_index,shot_id,candidate_id,result_id", sequence, index, c.ShotID, c.ID, c.ResultID)
}

// Parallel indexed columns must agree with the project-owned record JSON.
func (w *Workspace) placementRecord(q placementConnection, table, id string, out any, cols []string, expected []any) error {
	var project string
	if read(q, table, id, out) != nil || q.QueryRow("SELECT project_id FROM "+table+" WHERE id=?", id).Scan(&project) != nil || project != w.ProjectID {
		return ErrPlacementIntegrity
	}
	if len(cols) > 0 {
		values := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		if q.QueryRow("SELECT "+strings.Join(cols, ",")+" FROM "+table+" WHERE id=?", id).Scan(dest...) != nil || !reflect.DeepEqual(values, expected) {
			return ErrPlacementIntegrity
		}
	}
	return nil
}
func (w *Workspace) placementOwner(q placementConnection, p PlacementOwner, requireSelection bool) (Candidate, bool, error) {
	var c Candidate
	var s Shot
	var g Generation
	var r Result
	var j ArchiveJob
	// Missing expected business entities are a stable ownership rejection. SQL
	// failures and malformed existing records remain integrity/storage failures.
	for _, v := range []struct {
		table, id string
		out       any
	}{{"candidates", p.CandidateID, &c}, {"shots", p.ShotID, &s}, {"generations", p.GenerationID, &g}, {"results", p.ResultID, &r}, {"archive_jobs", p.ArchiveJobID, &j}} {
		err := read(q, v.table, v.id, v.out)
		if errors.Is(err, sql.ErrNoRows) {
			return c, false, nil
		}
		if err != nil {
			return c, false, ErrPlacementIntegrity
		}
	}
	for _, i := range []Identity{c.Identity, s.Identity, g.Identity, r.Identity, j.Identity} {
		if w.check(i) != nil {
			return c, false, ErrPlacementIntegrity
		}
	}
	if c.ID != p.CandidateID || s.ID != p.ShotID || g.ID != p.GenerationID || r.ID != p.ResultID || j.ID != p.ArchiveJobID {
		return c, false, ErrPlacementIntegrity
	}
	checks := []struct {
		table, id string
		out       any
		cols      []string
		values    []any
	}{
		{"candidates", c.ID, &c, []string{"shot_id", "generation_id", "result_id"}, []any{c.ShotID, c.GenerationID, c.ResultID}},
		{"shots", s.ID, &s, nil, nil},
		{"generations", g.ID, &g, []string{"shot_id"}, []any{nullable(g.ShotID)}},
		{"results", r.ID, &r, []string{"generation_id", "task_binding_id"}, []any{r.GenerationID, nullable(r.TaskBindingID)}},
		{"archive_jobs", j.ID, &j, []string{"generation_id", "result_id", "target_relative_path"}, []any{j.GenerationID, j.ResultID, j.TargetRelativePath}},
	}
	for _, v := range checks {
		if err := w.placementRecord(q, v.table, v.id, v.out, v.cols, v.values); err != nil {
			return c, false, err
		}
	}
	ok := c.ShotID == s.ID && c.GenerationID == g.ID && c.ResultID == r.ID && c.AvailabilityStatus == "ARCHIVED" && g.Frozen && g.ShotID == s.ID && g.FrozenHash == p.PreparedFrozenHash && generationHash(g) == g.FrozenHash && r.GenerationID == g.ID && r.Status == "ARCHIVED" && r.ArchiveJobID == j.ID && r.ArchivedRelativePath != "" && placementHash.MatchString(r.SHA256) && r.ByteLength > 0 && j.ResultID == r.ID && j.GenerationID == g.ID && j.Status == "ARCHIVED" && j.ActualSHA256 == r.SHA256 && j.ActualBytes == r.ByteLength && j.TargetRelativePath == r.ArchivedRelativePath && (!requireSelection || s.SelectedCandidateID == c.ID)
	return c, ok, nil
}
func (w *Workspace) placementItem(q placementConnection, id string) (SequenceItem, error) {
	var i SequenceItem
	if read(q, "sequence_items", id, &i) != nil || w.check(i.Identity) != nil || i.ID != id || !validName(i.SequenceID) || !validName(i.ShotID) || !validName(i.CandidateID) || !validName(i.ResultID) || i.OrderIndex < 0 || !placementTime(i.CreatedAt) || !placementTime(i.UpdatedAt) {
		return i, ErrPlacementIntegrity
	}
	err := w.placementRecord(q, "sequence_items", id, &i, []string{"sequence_id", "order_index", "shot_id", "candidate_id", "result_id"}, []any{i.SequenceID, int64(i.OrderIndex), i.ShotID, i.CandidateID, i.ResultID})
	var c Candidate
	var s Shot
	var r Result
	var g Generation
	if err != nil || read(q, "candidates", i.CandidateID, &c) != nil || read(q, "shots", i.ShotID, &s) != nil || read(q, "results", i.ResultID, &r) != nil || read(q, "generations", c.GenerationID, &g) != nil {
		return i, ErrPlacementIntegrity
	}
	for _, v := range []Identity{c.Identity, s.Identity, r.Identity, g.Identity} {
		if w.check(v) != nil {
			return i, ErrPlacementIntegrity
		}
	}
	if c.ID != i.CandidateID || s.ID != i.ShotID || r.ID != i.ResultID || g.ID != c.GenerationID || c.ShotID != i.ShotID || c.ResultID != i.ResultID || r.GenerationID != g.ID || g.ShotID != s.ID {
		return i, ErrPlacementIntegrity
	}
	for _, v := range []struct {
		table, id string
		out       any
		cols      []string
		values    []any
	}{
		{"candidates", c.ID, &c, []string{"shot_id", "generation_id", "result_id"}, []any{c.ShotID, c.GenerationID, c.ResultID}},
		{"shots", s.ID, &s, nil, nil}, {"generations", g.ID, &g, []string{"shot_id"}, []any{nullable(g.ShotID)}},
		{"results", r.ID, &r, []string{"generation_id", "task_binding_id"}, []any{r.GenerationID, nullable(r.TaskBindingID)}},
	} {
		if w.placementRecord(q, v.table, v.id, v.out, v.cols, v.values) != nil {
			return i, ErrPlacementIntegrity
		}
	}
	return i, nil
}
func (w *Workspace) commandRead(q placementConnection, intent string) (*PlacementReceipt, string, error) {
	var canonical, h, outcome, raw string
	var item, errorClass sql.NullString
	err := q.QueryRow("SELECT canonical_request,request_sha256,outcome,item_id,error_class,receipt FROM placement_commands WHERE project_id=? AND sequence_id='main' AND protocol_version=1 AND intent_id=?", w.ProjectID, intent).Scan(&canonical, &h, &outcome, &item, &errorClass, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", ErrPlacementIntegrity
	}
	var c PlacementCommand
	var receipt PlacementReceipt
	if json.Unmarshal([]byte(canonical), &c) != nil || !ValidPlacementCommand(c) || c.PlacementIntentID != intent || json.Unmarshal([]byte(raw), &receipt) != nil {
		return nil, "", ErrPlacementIntegrity
	}
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(b)
	rbytes, _ := json.Marshal(receipt)
	if string(b) != canonical || hex.EncodeToString(sum[:]) != h || string(rbytes) != raw || receipt.ProtocolVersion != 1 || receipt.ProjectID != w.ProjectID || receipt.SequenceID != "main" || receipt.PlacementIntentID != intent || receipt.Owner != c.PlacementOwner || receipt.Outcome != outcome {
		return nil, "", ErrPlacementIntegrity
	}
	if outcome == "COMMITTED" {
		if !item.Valid || errorClass.Valid || receipt.ErrorClass != nil || receipt.OriginalItem == nil {
			return nil, "", ErrPlacementIntegrity
		}
		i, e := w.placementItem(q, item.String)
		if e != nil {
			return nil, "", e
		}
		original := *receipt.OriginalItem
		current := PlacementItemFacts(i)
		current.OrderIndex = original.OrderIndex
		if current != original || i.SequenceID != "main" || i.CandidateID != c.CandidateID || i.ShotID != c.ShotID || i.ResultID != c.ResultID || original.OrderIndex < 0 {
			return nil, "", ErrPlacementIntegrity
		}
		if _, valid, err := w.placementOwner(q, c.PlacementOwner, false); err != nil || !valid {
			return nil, "", ErrPlacementIntegrity
		}
	} else if outcome != "REJECTED" || item.Valid || !errorClass.Valid || errorClass.String != "OWNERSHIP_REJECTED" || receipt.OriginalItem != nil || receipt.ErrorClass == nil || *receipt.ErrorClass != errorClass.String {
		return nil, "", ErrPlacementIntegrity
	}
	return &receipt, canonical, nil
}
func (w *Workspace) ExecutePlacementCommand(command PlacementCommand) (PlacementReceipt, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var receipt PlacementReceipt
	if !ValidPlacementCommand(command) {
		return receipt, ErrPlacementInput
	}
	err := w.placementWrite(func(q placementConnection) error {
		if err := placementExtension(q, true); err != nil {
			return err
		}
		canonical, _ := json.Marshal(command)
		old, stored, err := w.commandRead(q, command.PlacementIntentID)
		if err != nil {
			return err
		}
		if old != nil {
			if stored != string(canonical) {
				return ErrPlacementIntent
			}
			receipt = *old
			return nil
		}
		c, valid, err := w.placementOwner(q, command.PlacementOwner, true)
		if err != nil {
			return err
		}
		receipt = PlacementReceipt{ProtocolVersion: 1, ProjectID: w.ProjectID, SequenceID: "main", PlacementIntentID: command.PlacementIntentID, Owner: command.PlacementOwner, Outcome: "REJECTED"}
		var item any
		var errorClass any = "OWNERSHIP_REJECTED"
		rejected := "OWNERSHIP_REJECTED"
		receipt.ErrorClass = &rejected
		if valid {
			i, err := w.appendPlacement(q, "main", c)
			if err != nil {
				return err
			}
			facts := PlacementItemFacts(i)
			receipt.Outcome = "COMMITTED"
			receipt.OriginalItem = &facts
			receipt.ErrorClass = nil
			item = i.ID
			errorClass = nil
		}
		raw, _ := json.Marshal(receipt)
		sum := sha256.Sum256(canonical)
		_, err = q.Exec("INSERT INTO placement_commands VALUES(?,?,?,?,?,?,?,?,?,?)", w.ProjectID, "main", 1, command.PlacementIntentID, string(canonical), hex.EncodeToString(sum[:]), receipt.Outcome, item, errorClass, string(raw))
		return err
	})
	return receipt, err
}
func (w *Workspace) LookupPlacementCommand(intent string) (any, error) {
	if !placementUUID.MatchString(intent) {
		return nil, ErrPlacementInput
	}
	var result any
	err := w.placementRead(func(q placementConnection) error {
		if err := placementExtension(q, false); err != nil {
			return err
		}
		r, _, err := w.commandRead(q, intent)
		if err != nil {
			return err
		}
		if r == nil {
			result = PlacementLookup{1, w.ProjectID, "main", intent, "NOT_OBSERVED"}
		} else {
			result = *r
		}
		return nil
	})
	return result, err
}
func (w *Workspace) ReadMainSequence() (SequenceSnapshot, error) {
	result := SequenceSnapshot{1, w.ProjectID, "main", true, []PlacementItem{}}
	err := w.placementRead(func(q placementConnection) error {
		var count int
		if q.QueryRow("SELECT count(*) FROM sequence_items WHERE project_id=? AND sequence_id='main'", w.ProjectID).Scan(&count) != nil {
			return ErrPlacementIntegrity
		}
		if count > 256 {
			return ErrPlacementCapacity
		}
		rows, err := q.Query("SELECT id FROM sequence_items ORDER BY order_index,id")
		if err != nil {
			return ErrPlacementIntegrity
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if rows.Scan(&id) != nil {
				rows.Close()
				return ErrPlacementIntegrity
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return ErrPlacementIntegrity
		}
		seen := map[int]bool{}
		for _, id := range ids {
			i, e := w.placementItem(q, id)
			if e != nil {
				return ErrPlacementIntegrity
			}
			if i.SequenceID != "main" {
				continue
			}
			if seen[i.OrderIndex] {
				return ErrPlacementIntegrity
			}
			seen[i.OrderIndex] = true
			result.Items = append(result.Items, PlacementItemFacts(i))
		}
		if len(result.Items) != count {
			return ErrPlacementIntegrity
		}
		return nil
	})
	sort.Slice(result.Items, func(i, j int) bool {
		a, b := result.Items[i], result.Items[j]
		if a.OrderIndex == b.OrderIndex {
			return a.SequenceItemID < b.SequenceItemID
		}
		return a.OrderIndex < b.OrderIndex
	})
	return result, err
}
