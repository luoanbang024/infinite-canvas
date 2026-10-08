package foundation

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

var exportSourceIntegrity = errors.New("EXPORT_SOURCE_INTEGRITY")

func exportFinalRelative(id string) string { return "exports/commands-v1/" + id }
func exportMediaRelative(id string, n int, result, ext string) string {
	return fmt.Sprintf("%s/media/%03d_%s%s", exportFinalRelative(id), n, result, ext)
}
func exportStageRelative(j *exportJob) string {
	return "metadata/export-staging/" + *j.ExportID + "/" + *j.AttemptID
}
func exportJSON(v any) []byte           { b, _ := json.MarshalIndent(v, "", "  "); return append(b, '\n') }
func exportDigestBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (w *Workspace) exportAcquire(intent string) (func(), error) {
	k, _ := json.Marshal([]any{w.ProjectID, "main", 1, intent})
	p, e := w.Resolve("metadata/export-locks/" + exportDigestBytes(k) + ".lock")
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return nil, e
	}
	p, e = w.Resolve("metadata/export-locks/" + exportDigestBytes(k) + ".lock")
	if e != nil {
		return nil, e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, ErrExportIntegrity
	}
	unlock, e := exportKernelLock(f)
	if e != nil {
		f.Close()
		return nil, e
	}
	return func() { unlock(); f.Close() }, nil
}

type exportMarker struct {
	ProtocolVersion           int    `json:"protocolVersion"`
	ProjectID                 string `json:"projectId"`
	SequenceID                string `json:"sequenceId"`
	ExportIntentID            string `json:"exportIntentId"`
	CanonicalRequestHash      string `json:"canonicalRequestHash"`
	ExpectedRevision          string `json:"expectedRevision"`
	ObservedRevision          string `json:"observedRevision"`
	ExportID                  string `json:"exportId"`
	Format                    string `json:"format"`
	SnapshotHash              string `json:"snapshotHash"`
	ItemCount                 int    `json:"itemCount"`
	OrderedManifestJSONSHA256 string `json:"orderedManifestJsonSHA256"`
	OrderedManifestCSVSHA256  string `json:"orderedManifestCsvSHA256"`
	AttemptID                 string `json:"attemptId"`
}
type exportBundleHashes struct{ JSON, CSV, Marker string }

func (w *Workspace) exportBundleBytes(j *exportJob) ([]byte, []byte, []byte, exportBundleHashes, error) {
	m := ExportManifest{SchemaVersion, w.ProjectID, *j.ExportID, "main", []ManifestItem{}}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	if e := cw.Write([]string{"exportId", "sequenceIndex", "sequenceItemId", "shotId", "candidateId", "resultId", "relativePath", "sha256", "byteLength", "duration"}); e != nil {
		return nil, nil, nil, exportBundleHashes{}, e
	}
	for _, item := range j.Snapshot.Items {
		i := item.ManifestItem
		m.Items = append(m.Items, i)
		duration := ""
		if i.DurationSeconds != nil {
			duration = strconv.FormatFloat(*i.DurationSeconds, 'f', -1, 64)
		}
		if e := cw.Write([]string{i.ExportID, strconv.Itoa(i.SequenceIndex), i.SequenceItemID, i.ShotID, i.CandidateID, i.ResultID, i.RelativePath, i.SHA256, strconv.FormatInt(i.ByteLength, 10), duration}); e != nil {
			return nil, nil, nil, exportBundleHashes{}, e
		}
	}
	cw.Flush()
	if e := cw.Error(); e != nil {
		return nil, nil, nil, exportBundleHashes{}, e
	}
	jb := exportJSON(m)
	cb := buf.Bytes()
	h := exportBundleHashes{JSON: exportDigestBytes(jb), CSV: exportDigestBytes(cb)}
	marker := exportMarker{1, w.ProjectID, "main", j.Command.ExportIntentID, j.Hash, j.Command.ExpectedRevision, j.Observed, *j.ExportID, ExportFormat, *j.SnapshotHash, len(j.Snapshot.Items), h.JSON, h.CSV, *j.AttemptID}
	mb := exportJSON(marker)
	h.Marker = exportDigestBytes(mb)
	if len(jb)+len(cb)+len(mb) > exportOverheadLimit {
		return nil, nil, nil, h, ErrExportIntegrity
	}
	return jb, cb, mb, h, nil
}
func (w *Workspace) exportReadFile(relative string, limit int64) ([]byte, error) {
	p, e := w.Resolve(relative)
	if e != nil {
		return nil, ErrExportIntegrity
	}
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, ErrExportIntegrity
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, ErrExportIntegrity
	}
	return b, nil
}
func (w *Workspace) exportWriteFile(relative string, b []byte) error {
	if len(b) > exportOverheadLimit {
		return ErrExportIntegrity
	}
	p, e := w.Resolve(relative)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	return f.Sync()
}
func (w *Workspace) exportCopyItem(j *exportJob, i exportFrozenItem, n int, barrier func(string)) error {
	side, e := w.exportReadFile(i.SourceRelativePath+".json", exportOverheadLimit)
	expected := exportJSON(receipt{SchemaVersion, w.ProjectID, i.ArchiveJobID, i.SourceRelativePath, i.SHA256, i.ByteLength})
	if e != nil || !bytes.Equal(side, expected) {
		return exportSourceIntegrity
	}
	p, e := w.Resolve(i.SourceRelativePath)
	if e != nil {
		return exportSourceIntegrity
	}
	info, e := os.Lstat(p)
	if e != nil || !info.Mode().IsRegular() || info.Size() != i.ByteLength {
		return exportSourceIntegrity
	}
	source, e := os.Open(p)
	if e != nil {
		return exportSourceIntegrity
	}
	defer source.Close()
	suffix := strings.TrimPrefix(i.RelativePath, exportFinalRelative(*j.ExportID)+"/")
	p, e = w.Resolve(exportStageRelative(j) + "/" + suffix)
	if e != nil {
		return e
	}
	dest, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer dest.Close()
	h := sha256.New()
	out := io.MultiWriter(dest, h)
	first := make([]byte, 1)
	if _, e = io.ReadFull(source, first); e != nil {
		return exportSourceIntegrity
	}
	if _, e = out.Write(first); e != nil {
		return e
	}
	if n == 0 {
		exportBarrier(barrier, "PARTIAL_MEDIA")
	}
	copied, e := io.CopyN(out, source, i.ByteLength-1)
	if e != nil || copied != i.ByteLength-1 {
		if e == io.EOF || e == io.ErrUnexpectedEOF {
			return exportSourceIntegrity
		}
		return e
	}
	var extra [1]byte
	k, e := source.Read(extra[:])
	if k != 0 || e != io.EOF || hex.EncodeToString(h.Sum(nil)) != i.SHA256 {
		return exportSourceIntegrity
	}
	return dest.Sync()
}
func (w *Workspace) exportBuildStage(j *exportJob, barrier func(string)) error {
	stage := exportStageRelative(j)
	p, e := w.Resolve(stage)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	p, e = w.Resolve(stage)
	if e != nil {
		return e
	}
	if e = os.Mkdir(p, 0700); e != nil {
		return e
	}
	media, e := w.Resolve(stage + "/media")
	if e != nil {
		return e
	}
	if e = os.Mkdir(media, 0700); e != nil {
		return e
	}
	for n, i := range j.Snapshot.Items {
		if e = w.exportCopyItem(j, i, n, barrier); e != nil {
			return e
		}
	}
	exportBarrier(barrier, "ALL_MEDIA")
	jb, cb, mb, _, e := w.exportBundleBytes(j)
	if e != nil {
		return e
	}
	if e = w.exportWriteFile(stage+"/ordered-manifest.csv", cb); e != nil {
		return e
	}
	exportBarrier(barrier, "CSV")
	if e = w.exportWriteFile(stage+"/ordered-manifest.json", jb); e != nil {
		return e
	}
	exportBarrier(barrier, "JSON")
	if e = w.exportWriteFile(stage+"/export-command.json", mb); e != nil {
		return e
	}
	if e = exportSyncDirectory(media); e != nil {
		return e
	}
	if e = exportSyncDirectory(p); e != nil {
		return e
	}
	exportBarrier(barrier, "STAGE_COMPLETE")
	return nil
}
func (w *Workspace) exportVerify(j *exportJob, base string) (exportBundleHashes, error) {
	if j.AttemptID == nil {
		return exportBundleHashes{}, ErrExportIntegrity
	}
	jb, cb, mb, h, e := w.exportBundleBytes(j)
	if e != nil {
		return h, e
	}
	for _, v := range []struct {
		name string
		b    []byte
	}{{"ordered-manifest.csv", cb}, {"ordered-manifest.json", jb}, {"export-command.json", mb}} {
		b, e := w.exportReadFile(base+"/"+v.name, exportOverheadLimit)
		if e != nil {
			return h, e
		}
		if !bytes.Equal(b, v.b) {
			return h, ErrExportIntegrity
		}
	}
	expected := map[string]bool{"ordered-manifest.csv": true, "ordered-manifest.json": true, "export-command.json": true}
	for _, i := range j.Snapshot.Items {
		suffix := strings.TrimPrefix(i.RelativePath, exportFinalRelative(*j.ExportID)+"/")
		expected[suffix] = true
		if e = w.verifyFile(base+"/"+suffix, i.SHA256, i.ByteLength); e != nil {
			return h, e
		}
	}
	p, e := w.Resolve(base)
	if e != nil {
		return h, e
	}
	info, e := os.Lstat(p)
	if e != nil || !info.IsDir() {
		return h, ErrExportIntegrity
	}
	e = filepath.WalkDir(p, func(current string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		suffix, _ := filepath.Rel(p, current)
		if suffix == "." {
			return nil
		}
		suffix = filepath.ToSlash(suffix)
		if _, e := w.Resolve(base + "/" + suffix); e != nil || d.Type()&os.ModeSymlink != 0 {
			return ErrExportIntegrity
		}
		if d.IsDir() {
			if suffix != "media" {
				return ErrExportIntegrity
			}
			return nil
		}
		if !expected[suffix] {
			return ErrExportIntegrity
		}
		delete(expected, suffix)
		return nil
	})
	if e != nil || len(expected) != 0 {
		return h, ErrExportIntegrity
	}
	return h, nil
}
func (w *Workspace) exportFinalAbsent(j *exportJob) bool {
	p, e := w.Resolve(exportFinalRelative(*j.ExportID))
	if e != nil {
		return false
	}
	_, e = os.Lstat(p)
	return os.IsNotExist(e)
}
func (w *Workspace) exportBlock(j *exportJob, class string) error {
	return w.exportUpdate(j, "RECOVERY_BLOCKED", &class, nil, nil)
}
func (w *Workspace) exportRun(j *exportJob, barrier func(string)) error {
	final := exportFinalRelative(*j.ExportID)
	p, e := w.Resolve(final)
	if e != nil {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	_, stat := os.Lstat(p)
	if stat == nil {
		h, e := w.exportVerify(j, final)
		if e != nil {
			return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
		}
		if j.Phase != "READY_TO_FINALIZE" {
			if e = w.exportUpdate(j, "READY_TO_FINALIZE", nil, nil, nil); e != nil {
				return e
			}
		}
		return w.exportCommit(j, h, barrier)
	}
	if !os.IsNotExist(stat) {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	validStage := false
	if j.AttemptID != nil {
		_, e = w.exportVerify(j, exportStageRelative(j))
		validStage = e == nil
	}
	if !validStage {
		if j.AttemptCount >= MaxExportAttempts {
			return w.exportBlock(j, "EXPORT_ATTEMPT_EXHAUSTED")
		}
		attempt := uuid.NewString()
		if e = w.exportUpdate(j, "COPYING", nil, &attempt, nil); e != nil {
			return e
		}
		exportBarrier(barrier, "COPYING")
		if e = w.exportBuildStage(j, barrier); e != nil {
			if !w.exportFinalAbsent(j) {
				return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
			}
			if errors.Is(e, exportSourceIntegrity) {
				class := "EXPORT_SOURCE_INTEGRITY"
				r := w.exportReceipt(j, "FAILED")
				r.ErrorClass = &class
				return w.exportUpdate(j, "FAILED", &class, nil, &r)
			}
			class := "EXPORT_IO_RETRYABLE"
			return w.exportUpdate(j, "RETRYABLE", &class, nil, nil)
		}
	}
	h, e := w.exportVerify(j, exportStageRelative(j))
	if e != nil {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	if j.Phase != "READY_TO_FINALIZE" {
		if e = w.exportUpdate(j, "READY_TO_FINALIZE", nil, nil, nil); e != nil {
			return e
		}
	}
	exportBarrier(barrier, "READY_TO_FINALIZE")
	source, e := w.Resolve(exportStageRelative(j))
	if e != nil {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		class := "EXPORT_IO_RETRYABLE"
		return w.exportUpdate(j, "RETRYABLE", &class, nil, nil)
	}
	p, e = w.Resolve(final)
	if e != nil || !w.exportFinalAbsent(j) {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	if e = exportPublishDirectory(source, p); e != nil {
		if !w.exportFinalAbsent(j) {
			return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
		}
		class := "EXPORT_IO_RETRYABLE"
		return w.exportUpdate(j, "RETRYABLE", &class, nil, nil)
	}
	exportBarrier(barrier, "FINAL_RENAMED")
	if e = exportSyncDirectory(filepath.Dir(p)); e != nil {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	h, e = w.exportVerify(j, final)
	if e != nil {
		return w.exportBlock(j, "EXPORT_RECOVERY_BLOCKED")
	}
	exportBarrier(barrier, "FINAL_VERIFIED")
	return w.exportCommit(j, h, barrier)
}
func (w *Workspace) exportCommit(j *exportJob, h exportBundleHashes, barrier func(string)) error {
	r := w.exportReceipt(j, "COMMITTED")
	r.ManifestJSONSHA256 = &h.JSON
	r.ManifestCSVSHA256 = &h.CSV
	r.CommandMarkerSHA256 = &h.Marker
	if e := w.exportUpdate(j, "COMMITTED", nil, nil, &r); e != nil {
		return e
	}
	exportBarrier(barrier, "COMMITTED")
	return nil
}

type ExportBundleHealth struct {
	ProtocolVersion int    `json:"protocolVersion"`
	ProjectID       string `json:"projectId"`
	SequenceID      string `json:"sequenceId"`
	ExportIntentID  string `json:"exportIntentId"`
	ExportID        string `json:"exportId"`
	Health          string `json:"health"`
}

func (w *Workspace) VerifyExportBundle(intent string) (ExportBundleHealth, error) {
	if !placementUUID.MatchString(intent) {
		return ExportBundleHealth{}, ErrExportInput
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
	if e != nil {
		return ExportBundleHealth{}, e
	}
	if j == nil || j.Phase != "COMMITTED" {
		return ExportBundleHealth{}, ErrExportInput
	}
	health := ExportBundleHealth{1, w.ProjectID, "main", intent, *j.ExportID, "VERIFIED"}
	h, e := w.exportVerify(j, exportFinalRelative(*j.ExportID))
	if e != nil {
		health.Health = "CORRUPT"
		if os.IsNotExist(e) {
			health.Health = "MISSING"
		}
		return health, nil
	}
	if !reflect.DeepEqual(j.Receipt.ManifestJSONSHA256, &h.JSON) || !reflect.DeepEqual(j.Receipt.ManifestCSVSHA256, &h.CSV) || !reflect.DeepEqual(j.Receipt.CommandMarkerSHA256, &h.Marker) {
		health.Health = "CORRUPT"
	}
	return health, nil
}
