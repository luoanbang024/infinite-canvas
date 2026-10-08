package foundation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"math"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrExportInput       = errors.New("EXPORT_INPUT_INVALID")
	ErrExportIntegrity   = errors.New("EXPORT_INTEGRITY_CONFLICT")
	ErrExportIntent      = errors.New("EXPORT_INTENT_CONFLICT")
	ErrExportProtocol    = errors.New("EXPORT_PROTOCOL_NOT_INITIALIZED")
	ErrExportUnavailable = errors.New("EXPORT_STORE_UNAVAILABLE")
	ErrExportInProgress  = errors.New("EXPORT_IN_PROGRESS")
)

const ExportFormat = "hn-offline-bundle-v1"
const MaxExportAttempts = 3
const exportOverheadLimit = 1 << 20

type ExportCommand struct {
	ProtocolVersion        int      `json:"protocolVersion"`
	ExportIntentID         string   `json:"exportIntentId"`
	ExpectedRevision       string   `json:"expectedRevision"`
	OrderedSequenceItemIDs []string `json:"orderedSequenceItemIds"`
	Format                 string   `json:"format"`
}
type exportPayload struct {
	ExpectedRevision       string   `json:"expectedRevision"`
	OrderedSequenceItemIDs []string `json:"orderedSequenceItemIds"`
	Format                 string   `json:"format"`
}
type ExportProtocolState struct {
	ProtocolVersion int    `json:"protocolVersion"`
	ProjectID       string `json:"projectId"`
	SequenceID      string `json:"sequenceId"`
}
type ExportReceipt struct {
	ProtocolVersion          int      `json:"protocolVersion"`
	ProjectID                string   `json:"projectId"`
	SequenceID               string   `json:"sequenceId"`
	ExportIntentID           string   `json:"exportIntentId"`
	CanonicalRequestHash     string   `json:"canonicalRequestHash"`
	ExpectedRevision         string   `json:"expectedRevision"`
	ObservedRevision         string   `json:"observedRevision"`
	OrderedSequenceItemIDs   []string `json:"orderedSequenceItemIds"`
	Format                   string   `json:"format"`
	Outcome                  string   `json:"outcome"`
	ExportID                 *string  `json:"exportId"`
	BundleRelativePath       *string  `json:"bundleRelativePath"`
	ManifestJSONRelativePath *string  `json:"manifestJsonRelativePath"`
	ManifestCSVRelativePath  *string  `json:"manifestCsvRelativePath"`
	ItemCount                int      `json:"itemCount"`
	SnapshotHash             *string  `json:"snapshotHash"`
	ManifestJSONSHA256       *string  `json:"manifestJsonSHA256"`
	ManifestCSVSHA256        *string  `json:"manifestCsvSHA256"`
	CommandMarkerSHA256      *string  `json:"commandMarkerSHA256"`
	CompletedAt              string   `json:"completedAt"`
	ErrorClass               *string  `json:"errorClass"`
}
type ExportStatus struct {
	ProtocolVersion      int            `json:"protocolVersion"`
	ProjectID            string         `json:"projectId"`
	SequenceID           string         `json:"sequenceId"`
	ExportIntentID       string         `json:"exportIntentId"`
	Outcome              string         `json:"outcome"`
	Command              *ExportCommand `json:"command"`
	CanonicalRequestHash *string        `json:"canonicalRequestHash"`
	ExportID             *string        `json:"exportId"`
	AttemptCount         int            `json:"attemptCount"`
	ErrorClass           *string        `json:"errorClass"`
	Receipt              *ExportReceipt `json:"receipt"`
}
type exportFrozenItem struct {
	ManifestItem
	GenerationID       string `json:"generationId"`
	ArchiveJobID       string `json:"archiveJobId"`
	Frozen             bool   `json:"frozen"`
	FrozenHash         string `json:"frozenHash"`
	SourceRelativePath string `json:"sourceRelativePath"`
}
type exportSnapshot struct {
	ProjectID        string             `json:"projectId"`
	SequenceID       string             `json:"sequenceId"`
	ObservedRevision string             `json:"observedRevision"`
	Format           string             `json:"format"`
	ExportID         string             `json:"exportId"`
	Items            []exportFrozenItem `json:"items"`
	TotalMediaBytes  int64              `json:"totalMediaBytes"`
}
type exportJob struct {
	Command                                                     ExportCommand
	Canonical, Hash, Observed, Phase, Created, Updated          string
	ExportID, SnapshotJSON, SnapshotHash, AttemptID, ErrorClass *string
	AttemptCount                                                int
	Snapshot                                                    *exportSnapshot
	Receipt                                                     *ExportReceipt
}

func ValidExportCommand(c ExportCommand) bool {
	_, ok := ParseSequenceRevision(c.ExpectedRevision)
	return c.ProtocolVersion == 1 && placementUUID.MatchString(c.ExportIntentID) && ok && validReorderIDs(c.OrderedSequenceItemIDs) && c.Format == ExportFormat
}
func ExportCanonical(c ExportCommand) string {
	b, _ := json.Marshal(exportPayload{c.ExpectedRevision, c.OrderedSequenceItemIDs, c.Format})
	return string(b)
}
func ExportRequestHash(c ExportCommand) string { return reorderDigest(ExportCanonical(c)) }
func exportString(s string) *string            { return &s }
func exportTerminal(s string) bool             { return s == "COMMITTED" || s == "REJECTED" || s == "FAILED" }
func exportRelative(s string) bool {
	return s != "" && path.Clean(s) == s && !strings.HasPrefix(s, "/") && !strings.ContainsAny(s, "\\:") && !strings.Contains(s, "../") && !strings.HasSuffix(s, "/..")
}
func exportPhase(s string) bool {
	return exportTerminal(s) || s == "RESERVED" || s == "COPYING" || s == "READY_TO_FINALIZE" || s == "RETRYABLE" || s == "RECOVERY_BLOCKED"
}

var exportExtensionPattern = regexp.MustCompile(`^\.[A-Za-z0-9]{1,8}$`)

var exportSchema = []struct{ name, sql string }{
	{"export_protocol", `CREATE TABLE export_protocol (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL CHECK(version=1))`},
	{"export_jobs", `CREATE TABLE export_jobs (project_id TEXT NOT NULL REFERENCES project(id), sequence_id TEXT NOT NULL CHECK(sequence_id='main'), protocol_version INTEGER NOT NULL CHECK(protocol_version=1), intent_id TEXT NOT NULL, canonical_request TEXT NOT NULL, request_sha256 TEXT NOT NULL CHECK(length(request_sha256)=64), expected_revision INTEGER NOT NULL CHECK(expected_revision>=0), observed_revision INTEGER NOT NULL CHECK(observed_revision>=0), format TEXT NOT NULL CHECK(format='hn-offline-bundle-v1'), export_id TEXT UNIQUE, snapshot_json TEXT, snapshot_sha256 TEXT, phase TEXT NOT NULL CHECK(phase IN ('RESERVED','COPYING','READY_TO_FINALIZE','RETRYABLE','RECOVERY_BLOCKED','COMMITTED','REJECTED','FAILED')), attempt_count INTEGER NOT NULL CHECK(attempt_count BETWEEN 0 AND 3), current_attempt_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, error_class TEXT, PRIMARY KEY(project_id,sequence_id,protocol_version,intent_id), CHECK((phase='REJECTED' AND export_id IS NULL AND snapshot_json IS NULL AND snapshot_sha256 IS NULL AND attempt_count=0 AND current_attempt_id IS NULL) OR (phase!='REJECTED' AND export_id IS NOT NULL AND snapshot_json IS NOT NULL AND length(snapshot_sha256)=64)), CHECK((attempt_count=0 AND current_attempt_id IS NULL) OR (attempt_count>0 AND current_attempt_id IS NOT NULL)))`},
	{"export_receipts", `CREATE TABLE export_receipts (project_id TEXT NOT NULL, sequence_id TEXT NOT NULL CHECK(sequence_id='main'), protocol_version INTEGER NOT NULL CHECK(protocol_version=1), intent_id TEXT NOT NULL, outcome TEXT NOT NULL CHECK(outcome IN ('COMMITTED','REJECTED','FAILED')), receipt TEXT NOT NULL, PRIMARY KEY(project_id,sequence_id,protocol_version,intent_id), FOREIGN KEY(project_id,sequence_id,protocol_version,intent_id) REFERENCES export_jobs(project_id,sequence_id,protocol_version,intent_id))`},
	{"export_jobs_no_delete", `CREATE TRIGGER export_jobs_no_delete BEFORE DELETE ON export_jobs BEGIN SELECT RAISE(ABORT,'retained export job'); END`},
	{"export_jobs_no_replace", `CREATE TRIGGER export_jobs_no_replace BEFORE INSERT ON export_jobs WHEN EXISTS(SELECT 1 FROM export_jobs WHERE (project_id=NEW.project_id AND sequence_id=NEW.sequence_id AND protocol_version=NEW.protocol_version AND intent_id=NEW.intent_id) OR (NEW.export_id IS NOT NULL AND export_id=NEW.export_id)) BEGIN SELECT RAISE(ABORT,'retained export identity'); END`},
	{"export_jobs_immutable", `CREATE TRIGGER export_jobs_immutable BEFORE UPDATE ON export_jobs WHEN OLD.phase IN ('COMMITTED','REJECTED','FAILED') OR NEW.project_id IS NOT OLD.project_id OR NEW.sequence_id IS NOT OLD.sequence_id OR NEW.protocol_version IS NOT OLD.protocol_version OR NEW.intent_id IS NOT OLD.intent_id OR NEW.canonical_request IS NOT OLD.canonical_request OR NEW.request_sha256 IS NOT OLD.request_sha256 OR NEW.expected_revision IS NOT OLD.expected_revision OR NEW.observed_revision IS NOT OLD.observed_revision OR NEW.format IS NOT OLD.format OR NEW.export_id IS NOT OLD.export_id OR NEW.snapshot_json IS NOT OLD.snapshot_json OR NEW.snapshot_sha256 IS NOT OLD.snapshot_sha256 OR NEW.created_at IS NOT OLD.created_at BEGIN SELECT RAISE(ABORT,'immutable export authority'); END`},
	{"export_jobs_transition", `CREATE TRIGGER export_jobs_transition BEFORE UPDATE ON export_jobs WHEN NOT ((NEW.phase=OLD.phase AND NEW.phase IN ('COPYING','RETRYABLE','RECOVERY_BLOCKED','READY_TO_FINALIZE')) OR (OLD.phase IN ('RESERVED','COPYING','READY_TO_FINALIZE','RETRYABLE','RECOVERY_BLOCKED') AND NEW.phase IN ('COPYING','READY_TO_FINALIZE','RETRYABLE','RECOVERY_BLOCKED','FAILED')) OR (OLD.phase='READY_TO_FINALIZE' AND NEW.phase='COMMITTED')) OR NOT ((NEW.attempt_count=OLD.attempt_count AND NEW.current_attempt_id IS OLD.current_attempt_id) OR (NEW.phase='COPYING' AND NEW.attempt_count=OLD.attempt_count+1 AND NEW.current_attempt_id IS NOT NULL AND NEW.current_attempt_id IS NOT OLD.current_attempt_id)) BEGIN SELECT RAISE(ABORT,'invalid export transition'); END`},
	{"export_receipts_no_update", `CREATE TRIGGER export_receipts_no_update BEFORE UPDATE ON export_receipts BEGIN SELECT RAISE(ABORT,'immutable export receipt'); END`},
	{"export_receipts_no_delete", `CREATE TRIGGER export_receipts_no_delete BEFORE DELETE ON export_receipts BEGIN SELECT RAISE(ABORT,'retained export receipt'); END`},
	{"export_receipts_no_replace", `CREATE TRIGGER export_receipts_no_replace BEFORE INSERT ON export_receipts WHEN EXISTS(SELECT 1 FROM export_receipts WHERE project_id=NEW.project_id AND sequence_id=NEW.sequence_id AND protocol_version=NEW.protocol_version AND intent_id=NEW.intent_id) BEGIN SELECT RAISE(ABORT,'immutable export receipt identity'); END`},
}

func (w *Workspace) exportExtension(q placementConnection, install bool) error {
	var count int
	if q.QueryRow("SELECT count(*) FROM sqlite_master WHERE name GLOB 'export_*' OR (tbl_name IN ('export_protocol','export_jobs','export_receipts') AND sql IS NOT NULL)").Scan(&count) != nil {
		return ErrExportIntegrity
	}
	if count == 0 {
		if !install {
			return ErrExportProtocol
		}
		for _, v := range exportSchema {
			if _, e := q.Exec(v.sql); e != nil {
				return e
			}
		}
		if _, e := q.Exec("INSERT INTO export_protocol VALUES(1,1)"); e != nil {
			return e
		}
	} else if count != len(exportSchema) {
		return ErrExportIntegrity
	}
	for _, v := range exportSchema {
		var s string
		if q.QueryRow("SELECT sql FROM sqlite_master WHERE name=?", v.name).Scan(&s) != nil || strings.TrimSpace(s) != v.sql {
			return ErrExportIntegrity
		}
	}
	var total, id, version int
	if q.QueryRow("SELECT count(*),coalesce(max(id),0),coalesce(max(version),0) FROM export_protocol").Scan(&total, &id, &version) != nil || total != 1 || id != 1 || version != 1 {
		return ErrExportIntegrity
	}
	return nil
}
func (w *Workspace) InitializeExportProtocol() (ExportProtocolState, error) {
	s := ExportProtocolState{1, w.ProjectID, "main"}
	e := w.placementWrite(func(q placementConnection) error {
		if e := w.reorderExtension(q, false); e != nil {
			return e
		}
		return w.exportExtension(q, true)
	})
	return s, e
}
func exportCanonicalJSON(raw string, out any) bool {
	if json.Unmarshal([]byte(raw), out) != nil {
		return false
	}
	b, e := json.Marshal(out)
	return e == nil && string(b) == raw
}
func (w *Workspace) exportReadJob(q placementConnection, intent string) (*exportJob, error) {
	j := &exportJob{}
	var expected, observed int64
	var format string
	e := q.QueryRow("SELECT canonical_request,request_sha256,expected_revision,observed_revision,format,export_id,snapshot_json,snapshot_sha256,phase,attempt_count,current_attempt_id,created_at,updated_at,error_class FROM export_jobs WHERE project_id=? AND sequence_id='main' AND protocol_version=1 AND intent_id=?", w.ProjectID, intent).Scan(&j.Canonical, &j.Hash, &expected, &observed, &format, &j.ExportID, &j.SnapshotJSON, &j.SnapshotHash, &j.Phase, &j.AttemptCount, &j.AttemptID, &j.Created, &j.Updated, &j.ErrorClass)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, ErrExportIntegrity
	}
	var p exportPayload
	if !exportCanonicalJSON(j.Canonical, &p) {
		return nil, ErrExportIntegrity
	}
	j.Command = ExportCommand{1, intent, p.ExpectedRevision, p.OrderedSequenceItemIDs, p.Format}
	j.Observed = strconv.FormatInt(observed, 10)
	if !ValidExportCommand(j.Command) || j.Hash != ExportRequestHash(j.Command) || expected < 0 || strconv.FormatInt(expected, 10) != p.ExpectedRevision || observed < 0 || format != p.Format || !exportPhase(j.Phase) || j.AttemptCount < 0 || j.AttemptCount > 3 || !placementTime(j.Created) || !placementTime(j.Updated) || ((j.AttemptCount == 0) != (j.AttemptID == nil)) || j.AttemptID != nil && !placementUUID.MatchString(*j.AttemptID) {
		return nil, ErrExportIntegrity
	}
	if j.Phase == "REJECTED" {
		if j.ExportID != nil || j.SnapshotJSON != nil || j.SnapshotHash != nil || j.AttemptCount != 0 || j.ErrorClass == nil || !exportRejection(*j.ErrorClass) {
			return nil, ErrExportIntegrity
		}
	} else {
		if j.ExportID == nil || !placementUUID.MatchString(*j.ExportID) || j.SnapshotJSON == nil || j.SnapshotHash == nil || reorderDigest(*j.SnapshotJSON) != *j.SnapshotHash || j.Observed != p.ExpectedRevision {
			return nil, ErrExportIntegrity
		}
		var s exportSnapshot
		if !exportCanonicalJSON(*j.SnapshotJSON, &s) || !w.exportValidSnapshot(j, s) {
			return nil, ErrExportIntegrity
		}
		j.Snapshot = &s
	}

	if j.ErrorClass != nil {
		allowed := j.Phase == "REJECTED" && exportRejection(*j.ErrorClass) || j.Phase == "FAILED" && *j.ErrorClass == "EXPORT_SOURCE_INTEGRITY" || j.Phase == "RETRYABLE" && *j.ErrorClass == "EXPORT_IO_RETRYABLE" || j.Phase == "RECOVERY_BLOCKED" && (*j.ErrorClass == "EXPORT_RECOVERY_BLOCKED" || *j.ErrorClass == "EXPORT_ATTEMPT_EXHAUSTED")
		if !allowed {
			return nil, ErrExportIntegrity
		}
	} else if j.Phase == "REJECTED" || j.Phase == "FAILED" || j.Phase == "RETRYABLE" || j.Phase == "RECOVERY_BLOCKED" {
		return nil, ErrExportIntegrity
	}
	if j.Phase == "RESERVED" && j.AttemptCount != 0 || (j.Phase == "COPYING" || j.Phase == "READY_TO_FINALIZE" || j.Phase == "RETRYABLE") && j.AttemptCount == 0 {
		return nil, ErrExportIntegrity
	}
	var raw, outcome string
	e = q.QueryRow("SELECT receipt,outcome FROM export_receipts WHERE project_id=? AND sequence_id='main' AND protocol_version=1 AND intent_id=?", w.ProjectID, intent).Scan(&raw, &outcome)
	if exportTerminal(j.Phase) {
		var r ExportReceipt
		if e != nil || outcome != j.Phase || !exportCanonicalJSON(raw, &r) || !w.exportValidReceipt(j, r) {
			return nil, ErrExportIntegrity
		}
		j.Receipt = &r
	} else if !errors.Is(e, sql.ErrNoRows) {
		return nil, ErrExportIntegrity
	}
	return j, nil
}
func exportRejection(s string) bool {
	return s == "SEQUENCE_REVISION_CONFLICT" || s == "SEQUENCE_VECTOR_CONFLICT" || s == "EMPTY_SEQUENCE" || s == "READ_CAPACITY_EXCEEDED" || s == "EXPORT_OWNERSHIP_REJECTED"
}
func (w *Workspace) exportValidSnapshot(j *exportJob, s exportSnapshot) bool {
	if j.ExportID == nil || s.ProjectID != w.ProjectID || s.SequenceID != "main" || s.ObservedRevision != j.Observed || s.Format != ExportFormat || s.ExportID != *j.ExportID || len(s.Items) == 0 || len(s.Items) != len(j.Command.OrderedSequenceItemIDs) || len(s.Items) > 256 {
		return false
	}
	var total int64
	for n, i := range s.Items {
		ext := path.Ext(i.SourceRelativePath)
		if i.ExportID != s.ExportID || i.SequenceIndex != n+1 || i.SequenceItemID != j.Command.OrderedSequenceItemIDs[n] || !validName(i.ShotID) || !validName(i.CandidateID) || !validName(i.ResultID) || !validName(i.GenerationID) || !validName(i.ArchiveJobID) || !i.Frozen || !placementHash.MatchString(i.FrozenHash) || !exportRelative(i.SourceRelativePath) || !strings.HasPrefix(i.SourceRelativePath, "generated/"+i.GenerationID+"/") || !exportExtensionPattern.MatchString(ext) || i.RelativePath != exportMediaRelative(s.ExportID, n+1, i.ResultID, ext) || !placementHash.MatchString(i.SHA256) || i.ByteLength <= 0 || total > math.MaxInt64-i.ByteLength || i.DurationSeconds != nil && (math.IsNaN(*i.DurationSeconds) || math.IsInf(*i.DurationSeconds, 0) || *i.DurationSeconds < 0) {
			return false
		}
		total += i.ByteLength
	}
	return total == s.TotalMediaBytes
}
func (w *Workspace) exportValidReceipt(j *exportJob, r ExportReceipt) bool {
	if r.ProtocolVersion != 1 || r.ProjectID != w.ProjectID || r.SequenceID != "main" || r.ExportIntentID != j.Command.ExportIntentID || r.CanonicalRequestHash != j.Hash || r.ExpectedRevision != j.Command.ExpectedRevision || r.ObservedRevision != j.Observed || !reflect.DeepEqual(r.OrderedSequenceItemIDs, j.Command.OrderedSequenceItemIDs) || r.Format != ExportFormat || r.Outcome != j.Phase || !reflect.DeepEqual(r.ExportID, j.ExportID) || !reflect.DeepEqual(r.SnapshotHash, j.SnapshotHash) || !reflect.DeepEqual(r.ErrorClass, j.ErrorClass) || !placementTime(r.CompletedAt) {
		return false
	}
	if r.Outcome == "REJECTED" {
		return r.BundleRelativePath == nil && r.ManifestJSONRelativePath == nil && r.ManifestCSVRelativePath == nil && r.ManifestJSONSHA256 == nil && r.ManifestCSVSHA256 == nil && r.CommandMarkerSHA256 == nil && r.ItemCount == 0
	}
	b := exportFinalRelative(*j.ExportID)
	if r.BundleRelativePath == nil || *r.BundleRelativePath != b || r.ManifestJSONRelativePath == nil || *r.ManifestJSONRelativePath != b+"/ordered-manifest.json" || r.ManifestCSVRelativePath == nil || *r.ManifestCSVRelativePath != b+"/ordered-manifest.csv" || r.ItemCount != len(j.Snapshot.Items) {
		return false
	}
	if r.Outcome == "FAILED" {
		return r.ErrorClass != nil && *r.ErrorClass == "EXPORT_SOURCE_INTEGRITY" && r.ManifestJSONSHA256 == nil && r.ManifestCSVSHA256 == nil && r.CommandMarkerSHA256 == nil
	}

	if r.Outcome == "COMMITTED" {
		_, _, _, h, e := w.exportBundleBytes(j)
		if e != nil || r.ManifestJSONSHA256 == nil || *r.ManifestJSONSHA256 != h.JSON || r.ManifestCSVSHA256 == nil || *r.ManifestCSVSHA256 != h.CSV || r.CommandMarkerSHA256 == nil || *r.CommandMarkerSHA256 != h.Marker {
			return false
		}
	}
	return r.Outcome == "COMMITTED" && r.ErrorClass == nil && r.ManifestJSONSHA256 != nil && placementHash.MatchString(*r.ManifestJSONSHA256) && r.ManifestCSVSHA256 != nil && placementHash.MatchString(*r.ManifestCSVSHA256) && r.CommandMarkerSHA256 != nil && placementHash.MatchString(*r.CommandMarkerSHA256) && j.AttemptCount > 0
}
func (w *Workspace) exportStatus(intent string, j *exportJob) ExportStatus {
	s := ExportStatus{ProtocolVersion: 1, ProjectID: w.ProjectID, SequenceID: "main", ExportIntentID: intent, Outcome: "NOT_OBSERVED"}
	if j != nil {
		s.Outcome = j.Phase
		s.Command = &j.Command
		s.CanonicalRequestHash = &j.Hash
		s.ExportID = j.ExportID
		s.AttemptCount = j.AttemptCount
		s.ErrorClass = j.ErrorClass
		s.Receipt = j.Receipt
	}
	return s
}
func (w *Workspace) LookupExportCommand(intent string) (ExportStatus, error) {
	if !placementUUID.MatchString(intent) {
		return ExportStatus{}, ErrExportInput
	}
	var j *exportJob
	e := w.placementRead(func(q placementConnection) error {
		if e := w.exportExtension(q, false); e != nil {
			return e
		}
		var e error
		j, e = w.exportReadJob(q, intent)
		return e
	})
	return w.exportStatus(intent, j), e
}
func (w *Workspace) exportCapture(q placementConnection, c ExportCommand, observed string, exportID string, items []SequenceItem) (exportSnapshot, error) {
	s := exportSnapshot{w.ProjectID, "main", observed, ExportFormat, exportID, []exportFrozenItem{}, 0}
	for n, i := range items {
		var candidate Candidate
		var g Generation
		var r Result
		var a ArchiveJob
		if read(q, "candidates", i.CandidateID, &candidate) != nil || read(q, "generations", candidate.GenerationID, &g) != nil || read(q, "results", i.ResultID, &r) != nil || read(q, "archive_jobs", r.ArchiveJobID, &a) != nil {
			return s, ErrExportIntegrity
		}
		p := PlacementOwner{candidate.ID, i.ShotID, g.ID, r.ID, a.ID, g.FrozenHash}
		_, ok, e := w.placementOwner(q, p, false)
		if e != nil || !ok || candidate.ID != i.CandidateID || candidate.ResultID != i.ResultID {
			return s, ErrExportIntegrity
		}
		ext := path.Ext(r.ArchivedRelativePath)
		f := exportFrozenItem{ManifestItem{exportID, n + 1, i.ID, i.ShotID, i.CandidateID, i.ResultID, exportMediaRelative(exportID, n+1, r.ID, ext), r.SHA256, r.ByteLength, r.DurationSeconds}, g.ID, a.ID, true, g.FrozenHash, r.ArchivedRelativePath}
		if r.ByteLength <= 0 || s.TotalMediaBytes > math.MaxInt64-r.ByteLength {
			return s, ErrExportIntegrity
		}
		s.TotalMediaBytes += r.ByteLength
		s.Items = append(s.Items, f)
	}
	j := exportJob{Command: c, Observed: observed, ExportID: &exportID}
	if !w.exportValidSnapshot(&j, s) {
		return s, ErrExportIntegrity
	}
	return s, nil
}
func (w *Workspace) exportReceipt(j *exportJob, outcome string) ExportReceipt {
	r := ExportReceipt{ProtocolVersion: 1, ProjectID: w.ProjectID, SequenceID: "main", ExportIntentID: j.Command.ExportIntentID, CanonicalRequestHash: j.Hash, ExpectedRevision: j.Command.ExpectedRevision, ObservedRevision: j.Observed, OrderedSequenceItemIDs: j.Command.OrderedSequenceItemIDs, Format: ExportFormat, Outcome: outcome, ExportID: j.ExportID, SnapshotHash: j.SnapshotHash, CompletedAt: timestamp(), ErrorClass: j.ErrorClass}
	if j.ExportID != nil {
		base := exportFinalRelative(*j.ExportID)
		r.BundleRelativePath = &base
		r.ManifestJSONRelativePath = exportString(base + "/ordered-manifest.json")
		r.ManifestCSVRelativePath = exportString(base + "/ordered-manifest.csv")
		r.ItemCount = len(j.Snapshot.Items)
	}
	return r
}
func (w *Workspace) exportInsertReceipt(q placementConnection, r ExportReceipt) error {
	b, _ := json.Marshal(r)
	_, e := q.Exec("INSERT INTO export_receipts VALUES(?,'main',1,?,?,?)", w.ProjectID, r.ExportIntentID, r.Outcome, string(b))
	return e
}
func (w *Workspace) exportReserve(c ExportCommand) (*exportJob, error) {
	var result *exportJob
	e := w.placementWrite(func(q placementConnection) error {
		if e := w.exportExtension(q, false); e != nil {
			return e
		}
		old, e := w.exportReadJob(q, c.ExportIntentID)
		if e != nil {
			return e
		}
		if old != nil {
			if old.Canonical != ExportCanonical(c) {
				return ErrExportIntent
			}
			result = old
			return nil
		}
		if e := w.reorderExtension(q, false); e != nil {
			return e
		}
		revision, e := w.reorderRevision(q)
		if e != nil {
			return e
		}
		observed := strconv.FormatInt(revision, 10)
		items, e := w.reorderItems(q, "main", 256)
		reject := ""
		if errors.Is(e, ErrPlacementCapacity) {
			reject = "READ_CAPACITY_EXCEEDED"
		} else if e != nil {
			return ErrExportIntegrity
		}
		if reject == "" {
			ids := []string{}
			for _, i := range items {
				ids = append(ids, i.ID)
			}
			if c.ExpectedRevision != observed {
				reject = "SEQUENCE_REVISION_CONFLICT"
			} else if !reflect.DeepEqual(ids, c.OrderedSequenceItemIDs) {
				reject = "SEQUENCE_VECTOR_CONFLICT"
			} else if len(items) == 0 {
				reject = "EMPTY_SEQUENCE"
			}
		}
		j := &exportJob{Command: c, Canonical: ExportCanonical(c), Hash: ExportRequestHash(c), Observed: observed, Phase: "RESERVED", Created: timestamp(), Updated: timestamp()}
		if reject == "" {
			id := uuid.NewString()
			s, e := w.exportCapture(q, c, observed, id, items)
			if e != nil {
				reject = "EXPORT_OWNERSHIP_REJECTED"
			} else {
				b, _ := json.Marshal(s)
				j.ExportID = &id
				j.SnapshotJSON = exportString(string(b))
				j.SnapshotHash = exportString(reorderDigest(string(b)))
				j.Snapshot = &s
			}
		}
		if reject != "" {
			j.Phase = "REJECTED"
			j.ErrorClass = &reject
		}
		expected, _ := ParseSequenceRevision(c.ExpectedRevision)
		_, e = q.Exec("INSERT INTO export_jobs VALUES(?,'main',1,?,?,?,?,?,?,?,?,?,?,0,NULL,?,?,?)", w.ProjectID, c.ExportIntentID, j.Canonical, j.Hash, expected, revision, ExportFormat, j.ExportID, j.SnapshotJSON, j.SnapshotHash, j.Phase, j.Created, j.Updated, j.ErrorClass)
		if e != nil {
			return e
		}
		if reject != "" {
			r := w.exportReceipt(j, "REJECTED")
			if e = w.exportInsertReceipt(q, r); e != nil {
				return e
			}
		}
		result, e = w.exportReadJob(q, c.ExportIntentID)
		return e
	})
	return result, e
}

// Short conditional transactions only. File work and lock contention occur outside.
func (w *Workspace) exportUpdate(j *exportJob, phase string, class *string, attempt *string, receipt *ExportReceipt) error {
	return w.placementWrite(func(q placementConnection) error {
		if e := w.exportExtension(q, false); e != nil {
			return e
		}
		old, e := w.exportReadJob(q, j.Command.ExportIntentID)
		if e != nil || old == nil || old.Canonical != j.Canonical || old.Phase != j.Phase || old.AttemptCount != j.AttemptCount {
			return ErrExportIntegrity
		}
		count := j.AttemptCount
		current := j.AttemptID
		if attempt != nil {
			count++
			current = attempt
		}
		r, e := q.Exec("UPDATE export_jobs SET phase=?,attempt_count=?,current_attempt_id=?,updated_at=?,error_class=? WHERE project_id=? AND sequence_id='main' AND protocol_version=1 AND intent_id=? AND phase=? AND attempt_count=?", phase, count, current, timestamp(), class, w.ProjectID, j.Command.ExportIntentID, j.Phase, j.AttemptCount)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil || n != 1 {
			return ErrExportIntegrity
		}
		if receipt != nil {
			if e = w.exportInsertReceipt(q, *receipt); e != nil {
				return e
			}
		}
		updated, e := w.exportReadJob(q, j.Command.ExportIntentID)
		if e != nil {
			return e
		}
		*j = *updated
		return nil
	})
}
func (w *Workspace) ExecuteExportCommand(c ExportCommand) (ExportStatus, error) {
	return w.executeExportCommand(c, nil)
}

// barrier is package-private; never controlled by HTTP, settings or environment.
func (w *Workspace) executeExportCommand(c ExportCommand, barrier func(string)) (ExportStatus, error) {
	if !ValidExportCommand(c) {
		return ExportStatus{}, ErrExportInput
	}
	c.OrderedSequenceItemIDs = append([]string{}, c.OrderedSequenceItemIDs...)
	status, e := w.LookupExportCommand(c.ExportIntentID)
	if e != nil {
		return status, e
	}
	if status.Command != nil && ExportCanonical(*status.Command) != ExportCanonical(c) {
		return status, ErrExportIntent
	}
	if status.Receipt != nil {
		return status, nil
	}
	release, e := w.exportAcquire(c.ExportIntentID)
	if errors.Is(e, ErrExportInProgress) {
		status, e = w.LookupExportCommand(c.ExportIntentID)
		if e != nil {
			return status, e
		}
		if status.Command != nil && ExportCanonical(*status.Command) != ExportCanonical(c) {
			return status, ErrExportIntent
		}
		if status.Receipt != nil {
			return status, nil
		}
		status.Outcome = "IN_PROGRESS"
		return status, nil
	}
	if e != nil {
		return status, ErrExportUnavailable
	}
	defer release()
	j, e := w.exportReserve(c)
	if e != nil {
		return status, e
	}
	if j.Receipt != nil {
		return w.exportStatus(c.ExportIntentID, j), nil
	}
	exportBarrier(barrier, "RESERVED")
	e = w.exportRun(j, barrier)
	return w.exportStatus(c.ExportIntentID, j), e
}
func exportBarrier(f func(string), point string) {
	if f != nil {
		f(point)
	}
}
