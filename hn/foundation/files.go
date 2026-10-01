package foundation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type receipt struct {
	SchemaVersion int    `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
	ID            string `json:"id"`
	RelativePath  string `json:"relativePath"`
	SHA256        string `json:"sha256"`
	ByteLength    int64  `json:"byteLength"`
}

func (w *Workspace) fileFacts(relative string) (int64, string, error) {
	path, err := w.Resolve(relative)
	if err != nil {
		return 0, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return 0, "", errors.New("not a regular file")
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	return n, hex.EncodeToString(h.Sum(nil)), err
}
func (w *Workspace) verifyFile(relative, hash string, n int64) error {
	actual, sha, err := w.fileFacts(relative)
	if err != nil {
		return err
	}
	if actual != n || sha != hash {
		return errors.New("completed bytes/hash mismatch")
	}
	return nil
}
func (w *Workspace) tempFile(relative string, source io.Reader, expectedBytes int64, expectedHash string) (string, int64, string, error) {
	if source == nil {
		return "", 0, "", errors.New("source reader required")
	}
	path, err := w.Resolve(relative)
	if err != nil {
		return "", 0, "", err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", 0, "", err
	}
	if _, err = w.Resolve(relative); err != nil {
		return "", 0, "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "hn-partial-*")
	if err != nil {
		return "", 0, "", err
	}
	name := f.Name()
	keep := false
	defer func() {
		f.Close()
		if !keep {
			os.Remove(name)
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), source)
	if err != nil {
		return "", n, "", errors.New("source copy interrupted")
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if expectedBytes >= 0 && n != expectedBytes || expectedHash != "" && hash != expectedHash {
		return "", n, hash, errors.New("source completed byte verification failed")
	}
	if err = f.Sync(); err != nil {
		return "", n, hash, err
	}
	if err = f.Close(); err != nil {
		return "", n, hash, err
	}
	tempRelative, err := filepath.Rel(w.Root, name)
	if err != nil {
		return "", n, hash, err
	}
	if err = w.verifyFile(filepath.ToSlash(tempRelative), hash, n); err != nil {
		return "", n, hash, err
	}
	keep = true
	return name, n, hash, nil
}
func (w *Workspace) publish(temp, relative string) error {
	path, err := w.Resolve(relative)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(path); err == nil {
		return errors.New("refuse to overwrite final file")
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temp, path) // temporary and final file are on the same volume.
}
func (w *Workspace) jsonFile(relative string, value any) error {
	if err := nonSecret(value); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path, err := w.Resolve(relative)
	if err != nil {
		return err
	}
	if old, e := os.ReadFile(path); e == nil {
		if string(old) == string(b) {
			return nil
		}
		return errors.New("immutable sidecar already differs")
	} else if !os.IsNotExist(e) {
		return e
	}
	temp, _, _, err := w.tempFile(relative, strings.NewReader(string(b)), int64(len(b)), digest(b))
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	return w.publish(temp, relative)
}
func (w *Workspace) verifyReference(r ReferenceVersion) error {
	if err := w.check(r.Identity); err != nil {
		return err
	}
	if err := w.verifyFile(r.RelativePath, r.SHA256, r.ByteLength); err != nil {
		return err
	}
	path, err := w.Resolve(r.RelativePath + ".json")
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var side ReferenceVersion
	if err = json.Unmarshal(b, &side); err != nil {
		return err
	}
	a, _ := json.Marshal(r)
	c, _ := json.Marshal(side)
	if string(a) != string(c) {
		return errors.New("reference sidecar mismatch")
	}
	return nil
}

// SnapshotFile reads only the supplied file, never a directory or a drive scan.
func (w *Workspace) SnapshotFile(source, logicalID, kind, mime string) (ReferenceVersion, error) {
	f, err := os.Open(source)
	if err != nil {
		return ReferenceVersion{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ReferenceVersion{}, errors.New("source must be a regular file")
	}
	return w.Snapshot(logicalID, kind, mime, f, info.Size())
}

// Snapshot accepts explicit submitted bytes; expectedBytes must be known.
func (w *Workspace) Snapshot(logicalID, kind, mime string, source io.Reader, expectedBytes int64) (ReferenceVersion, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !validName(logicalID) || expectedBytes < 0 {
		return ReferenceVersion{}, errors.New("logical ID/length required")
	}
	if kind != "image" && kind != "video" && kind != "audio" {
		return ReferenceVersion{}, errors.New("unsupported reference kind")
	}
	r := ReferenceVersion{Identity: w.identity(), LogicalReferenceID: logicalID, Kind: kind, MimeType: mime}
	r.RelativePath = "references/" + r.ID + "/reference.bin"
	if err := nonSecret(r); err != nil {
		return r, err
	}
	temp, n, hash, err := w.tempFile(r.RelativePath, source, expectedBytes, "")
	if err != nil {
		return ReferenceVersion{}, err
	}
	defer os.Remove(temp)
	r.ByteLength = n
	r.SHA256 = hash
	if err = w.publish(temp, r.RelativePath); err != nil {
		return ReferenceVersion{}, err
	}
	if err = w.jsonFile(r.RelativePath+".json", r); err != nil {
		return ReferenceVersion{}, err
	}
	if err = w.verifyReference(r); err != nil {
		return ReferenceVersion{}, err
	}
	return r, insert(w.db, "reference_versions", r.Identity, r, "")
}

func (w *Workspace) CreateArchive(resultID, target, mime string) (ArchiveJob, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var r Result
	if err := read(w.db, "results", resultID, &r); err != nil {
		return ArchiveJob{}, err
	}
	if r.ArchiveJobID != "" {
		return ArchiveJob{}, errors.New("result already has an archive job")
	}
	if target == "" {
		target = "generated/" + r.GenerationID + "/" + r.ID + "/media.bin"
	}
	if !strings.HasPrefix(target, "generated/"+r.GenerationID+"/") {
		return ArchiveJob{}, errors.New("archive must belong to generation tree")
	}
	if _, err := w.Resolve(target); err != nil {
		return ArchiveJob{}, err
	}
	j := ArchiveJob{Identity: w.identity(), ResultID: r.ID, GenerationID: r.GenerationID, TargetRelativePath: target, ExpectedMime: mime, Status: "PENDING"}
	if err := nonSecret(j); err != nil {
		return j, err
	}
	tx, err := w.db.Begin()
	if err != nil {
		return j, err
	}
	defer tx.Rollback()
	if err = insert(tx, "archive_jobs", j.Identity, j, "generation_id,result_id,target_relative_path", j.GenerationID, j.ResultID, j.TargetRelativePath); err != nil {
		return j, err
	}
	r.ArchiveJobID = j.ID
	r.UpdatedAt = timestamp()
	if err = put(tx, "results", r.ID, r); err != nil {
		return j, err
	}
	return j, tx.Commit()
}
func (w *Workspace) archiveReceipt(j ArchiveJob) receipt {
	return receipt{SchemaVersion, w.ProjectID, j.ID, j.TargetRelativePath, j.ActualSHA256, j.ActualBytes}
}
func (w *Workspace) verifyArchive(j ArchiveJob) error {
	if j.ActualSHA256 == "" {
		return errors.New("no completed byte evidence")
	}
	if err := w.verifyFile(j.TargetRelativePath, j.ActualSHA256, j.ActualBytes); err != nil {
		return err
	}
	path, err := w.Resolve(j.TargetRelativePath + ".json")
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var got receipt
	if err = json.Unmarshal(b, &got); err != nil {
		return err
	}
	if got != w.archiveReceipt(j) {
		return errors.New("archive sidecar mismatch")
	}
	return nil
}
func (w *Workspace) completeArchive(j ArchiveJob) error {
	if err := w.verifyArchive(j); err != nil {
		return err
	}
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var r Result
	if err = read(tx, "results", j.ResultID, &r); err != nil {
		return err
	}
	if r.GenerationID != j.GenerationID || r.ArchiveJobID != j.ID {
		return errors.New("archive ownership mismatch")
	}
	j.Status = "ARCHIVED"
	j.CompletedAt = timestamp()
	j.UpdatedAt = j.CompletedAt
	j.LastError = ""
	r.Status = "ARCHIVED"
	r.ArchivedRelativePath = j.TargetRelativePath
	r.SHA256 = j.ActualSHA256
	r.ByteLength = j.ActualBytes
	r.UpdatedAt = j.CompletedAt
	if err = put(tx, "archive_jobs", j.ID, j); err != nil {
		return err
	}
	if err = put(tx, "results", r.ID, r); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE candidates SET data=json_set(data,'$.availabilityStatus','ARCHIVED') WHERE result_id=?", r.ID); err != nil {
		return err
	}
	return tx.Commit()
}
func (w *Workspace) failArchive(j ArchiveJob, status, message string) error {
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	j.Status = status
	j.LastError = message
	j.UpdatedAt = timestamp()
	if err = put(tx, "archive_jobs", j.ID, j); err != nil {
		return err
	}
	var r Result
	if err = read(tx, "results", j.ResultID, &r); err != nil {
		return err
	}
	r.Status = "ARCHIVE_" + status
	r.UpdatedAt = j.UpdatedAt
	if err = put(tx, "results", r.ID, r); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE candidates SET data=json_set(data,'$.availabilityStatus',?) WHERE result_id=?", r.Status, r.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// RunArchive copies supplied bytes only. There is no submit or HTTP operation.
// A nil reader is permitted only when an existing verified final file can be reused.
func (w *Workspace) RunArchive(id string, source io.Reader, expectedBytes int64, expectedHash string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var j ArchiveJob
	if err := read(w.db, "archive_jobs", id, &j); err != nil {
		return err
	}
	if j.ActualSHA256 != "" {
		if err := w.verifyFile(j.TargetRelativePath, j.ActualSHA256, j.ActualBytes); err == nil {
			if expectedBytes >= 0 && expectedBytes != j.ActualBytes || expectedHash != "" && expectedHash != j.ActualSHA256 {
				return errors.New("retry expectation differs")
			}
			if err = w.jsonFile(j.TargetRelativePath+".json", w.archiveReceipt(j)); err != nil {
				return err
			}
			return w.completeArchive(j)
		}
	}
	if j.Status == "ARCHIVED" {
		if err := w.failArchive(j, "INCONSISTENT", "final bytes missing or mismatched"); err != nil {
			return err
		}
		return errors.New("archived file mismatch")
	}
	if expectedBytes < 0 {
		return errors.New("expected completed byte length required")
	}
	j.AttemptCount++
	j.Status = "COPYING"
	j.StartedAt = timestamp()
	j.UpdatedAt = j.StartedAt
	if err := put(w.db, "archive_jobs", id, j); err != nil {
		return err
	}
	temp, n, hash, err := w.tempFile(j.TargetRelativePath, source, expectedBytes, expectedHash)
	if err != nil {
		if e := w.failArchive(j, "FAILED", "copy or byte verification failed"); e != nil {
			return e
		}
		return err
	}
	defer os.Remove(temp)
	j.ActualBytes = n
	j.ActualSHA256 = hash
	j.Status = "FINALIZING"
	j.UpdatedAt = timestamp()
	if err = put(w.db, "archive_jobs", id, j); err != nil {
		return err
	}
	if err = w.publish(temp, j.TargetRelativePath); err != nil {
		if e := w.failArchive(j, "FAILED", "final publication failed"); e != nil {
			return e
		}
		return err
	}
	if err = w.jsonFile(j.TargetRelativePath+".json", w.archiveReceipt(j)); err != nil {
		return err
	}
	return w.completeArchive(j)
}

// Reopen reconciles final-file/metadata disagreement. Only matching verified final
// bytes can complete an interrupted FINALIZING operation. It never generates media.
func (w *Workspace) reconcile() error {
	refs, err := w.List("reference_versions")
	if err != nil {
		return err
	}
	for _, raw := range refs {
		var r ReferenceVersion
		if err = json.Unmarshal(raw, &r); err != nil {
			return err
		}
		if err = w.verifyReference(r); err != nil {
			return fmt.Errorf("reference %s: %w", r.ID, err)
		}
	}
	gens, err := w.List("generations")
	if err != nil {
		return err
	}
	for _, raw := range gens {
		var g Generation
		if err = json.Unmarshal(raw, &g); err != nil {
			return err
		}
		if g.Frozen {
			if err = w.validateGeneration(&g); err != nil {
				return err
			}
			if generationHash(g) != g.FrozenHash {
				return errors.New("frozen generation corrupted")
			}
		}
	}
	jobs, err := w.List("archive_jobs")
	if err != nil {
		return err
	}
	for _, raw := range jobs {
		var j ArchiveJob
		if err = json.Unmarshal(raw, &j); err != nil {
			return err
		}
		if j.Status == "ARCHIVED" {
			if e := w.verifyArchive(j); e != nil {
				if err = w.failArchive(j, "INCONSISTENT", "file/receipt verification failed on reopen"); err != nil {
					return err
				}
				continue
			}
			var r Result
			if err = read(w.db, "results", j.ResultID, &r); err != nil {
				return err
			}
			if r.Status != "ARCHIVED" || r.ArchivedRelativePath != j.TargetRelativePath || r.SHA256 != j.ActualSHA256 || r.ByteLength != j.ActualBytes {
				if err = w.completeArchive(j); err != nil {
					return err
				}
			}
		} else if j.Status == "FINALIZING" && w.verifyFile(j.TargetRelativePath, j.ActualSHA256, j.ActualBytes) == nil {
			if err = w.jsonFile(j.TargetRelativePath+".json", w.archiveReceipt(j)); err != nil {
				if err = w.failArchive(j, "INCONSISTENT", "sidecar mismatch on reopen"); err != nil {
					return err
				}
				continue
			}
			if err = w.completeArchive(j); err != nil {
				return err
			}
		} else if j.Status == "COPYING" || j.Status == "FINALIZING" {
			if err = w.failArchive(j, "FAILED", "interrupted before verified final bytes"); err != nil {
				return err
			}
		}
	}
	return nil
}
