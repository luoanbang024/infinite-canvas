package foundation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var tables = map[string]bool{"reference_versions": true, "generations": true, "task_bindings": true, "results": true, "archive_jobs": true, "shots": true, "candidates": true, "sequence_items": true}

const schema = `
CREATE TABLE project (id TEXT PRIMARY KEY);
CREATE TABLE reference_versions (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), data TEXT NOT NULL);
CREATE TABLE shots (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), data TEXT NOT NULL);
CREATE TABLE generations (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), shot_id TEXT REFERENCES shots(id), data TEXT NOT NULL);
CREATE TABLE generation_references (generation_id TEXT REFERENCES generations(id) ON DELETE CASCADE, reference_id TEXT REFERENCES reference_versions(id), PRIMARY KEY(generation_id,reference_id));
CREATE TABLE task_bindings (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), generation_id TEXT NOT NULL REFERENCES generations(id), data TEXT NOT NULL, UNIQUE(id,generation_id));
CREATE TABLE results (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), generation_id TEXT NOT NULL REFERENCES generations(id), task_binding_id TEXT, data TEXT NOT NULL, UNIQUE(id,generation_id), FOREIGN KEY(task_binding_id,generation_id) REFERENCES task_bindings(id,generation_id));
CREATE TABLE archive_jobs (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), generation_id TEXT NOT NULL, result_id TEXT NOT NULL UNIQUE, target_relative_path TEXT NOT NULL COLLATE NOCASE UNIQUE, data TEXT NOT NULL, FOREIGN KEY(result_id,generation_id) REFERENCES results(id,generation_id));
CREATE TABLE candidates (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), shot_id TEXT NOT NULL REFERENCES shots(id), generation_id TEXT NOT NULL REFERENCES generations(id), result_id TEXT NOT NULL, data TEXT NOT NULL, FOREIGN KEY(result_id,generation_id) REFERENCES results(id,generation_id));
CREATE TABLE sequence_items (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), sequence_id TEXT NOT NULL, order_index INTEGER NOT NULL CHECK(order_index>=0), shot_id TEXT NOT NULL REFERENCES shots(id), candidate_id TEXT NOT NULL REFERENCES candidates(id), result_id TEXT NOT NULL REFERENCES results(id), data TEXT NOT NULL);
PRAGMA user_version=1;`

func (w *Workspace) initialize() error {
	if _, err := w.db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA synchronous=FULL;"); err != nil {
		return err
	}
	v, err := w.Version()
	if err != nil {
		return err
	}
	if v != 0 && v != SchemaVersion {
		return fmt.Errorf("unsupported HN schema version %d", v)
	}
	if v == 0 {
		var count int
		if err = w.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errors.New("unversioned nonempty store rejected")
		}
		tx, e := w.db.Begin()
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if _, e = tx.Exec(schema); e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO project(id) VALUES(?)", w.ProjectID); e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	var id string
	if err = w.db.QueryRow("SELECT id FROM project").Scan(&id); err != nil {
		return err
	}
	if id != w.ProjectID {
		return errors.New("store belongs to another project")
	}
	return w.IntegrityCheck()
}

type queryer interface{ QueryRow(string, ...any) *sql.Row }
type executor interface {
	Exec(string, ...any) (sql.Result, error)
}

func read(q queryer, table, id string, out any) error {
	var raw string
	if err := q.QueryRow("SELECT data FROM "+table+" WHERE id=?", id).Scan(&raw); err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), out)
}
func put(q executor, table string, id string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	result, err := q.Exec("UPDATE "+table+" SET data=? WHERE id=?", string(b), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return sql.ErrNoRows
	}
	return err
}
func insert(q executor, table string, i Identity, value any, columns string, args ...any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cols := "id,project_id,data"
	marks := "?,?,?"
	values := []any{i.ID, i.ProjectID, string(b)}
	if columns != "" {
		cols += "," + columns
		for range args {
			marks += ",?"
		}
		values = append(values, args...)
	}
	_, err = q.Exec("INSERT INTO "+table+" ("+cols+") VALUES ("+marks+")", values...)
	return err
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Read/List expose JSON records; writes use the typed, invariant-preserving operations.
func (w *Workspace) Read(kind, id string) (json.RawMessage, error) {
	if !tables[kind] {
		return nil, errors.New("unknown entity")
	}
	var raw string
	err := w.db.QueryRow("SELECT data FROM "+kind+" WHERE id=? AND project_id=?", id, w.ProjectID).Scan(&raw)
	return json.RawMessage(raw), err
}
func (w *Workspace) List(kind string) ([]json.RawMessage, error) {
	if !tables[kind] {
		return nil, errors.New("unknown entity")
	}
	rows, err := w.db.Query("SELECT data FROM "+kind+" WHERE project_id=? ORDER BY id", w.ProjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(raw))
	}
	return out, rows.Err()
}

// Delete removes only unreferenced metadata. Files are deliberately retained; no
// destructive asset garbage collection is performed. Frozen history is retained.
func (w *Workspace) Delete(kind, id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !tables[kind] {
		return errors.New("unknown entity")
	}
	if kind == "sequence_items" {
		return w.placementWrite(func(q placementConnection) error { return w.deleteSequenceItem(q, id) })
	}
	if kind == "generations" {
		var g Generation
		if err := read(w.db, kind, id, &g); err != nil {
			return err
		}
		if g.Frozen {
			return errors.New("frozen history cannot be deleted")
		}
	}
	if kind == "archive_jobs" {
		var job ArchiveJob
		if err := read(w.db, kind, id, &job); err != nil {
			return err
		}
		if job.Status != "PENDING" {
			return errors.New("archive history cannot be deleted")
		}
		tx, err := w.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var result Result
		if err = read(tx, "results", job.ResultID, &result); err != nil {
			return err
		}
		result.ArchiveJobID = ""
		result.UpdatedAt = timestamp()
		if err = put(tx, "results", result.ID, result); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM archive_jobs WHERE id=?", id); err != nil {
			return err
		}
		return tx.Commit()
	}
	if kind == "candidates" {
		rows, err := w.List("shots")
		if err != nil {
			return err
		}
		for _, raw := range rows {
			var s Shot
			json.Unmarshal(raw, &s)
			if s.SelectedCandidateID == id {
				return errors.New("selected candidate cannot be deleted")
			}
		}
	}
	result, err := w.db.Exec("DELETE FROM "+kind+" WHERE id=? AND project_id=?", id, w.ProjectID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return sql.ErrNoRows
	}
	return err
}
