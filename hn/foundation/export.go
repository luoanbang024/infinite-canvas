package foundation

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/google/uuid"
)

// Export copies only explicitly selected, verified archived media. JSON is the
// completion marker; consumers require both manifests. No editor folders are used.
func (w *Workspace) Export(sequenceID string) (ExportManifest, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !validName(sequenceID) {
		return ExportManifest{}, errors.New("unsafe sequence ID")
	}
	raw, err := w.List("sequence_items")
	if err != nil {
		return ExportManifest{}, err
	}
	items := []SequenceItem{}
	for _, b := range raw {
		var i SequenceItem
		if err = json.Unmarshal(b, &i); err != nil {
			return ExportManifest{}, err
		}
		if i.SequenceID == sequenceID {
			items = append(items, i)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OrderIndex == items[j].OrderIndex {
			return items[i].ID < items[j].ID
		}
		return items[i].OrderIndex < items[j].OrderIndex
	})
	if len(items) == 0 {
		return ExportManifest{}, errors.New("empty sequence")
	}
	m := ExportManifest{SchemaVersion: SchemaVersion, ProjectID: w.ProjectID, ExportID: uuid.NewString(), SequenceID: sequenceID, Items: []ManifestItem{}}
	for index, i := range items {
		var s Shot
		var c Candidate
		var r Result
		var j ArchiveJob
		if err = read(w.db, "shots", i.ShotID, &s); err != nil {
			return ExportManifest{}, err
		}
		if err = read(w.db, "candidates", i.CandidateID, &c); err != nil {
			return ExportManifest{}, err
		}
		if err = read(w.db, "results", i.ResultID, &r); err != nil {
			return ExportManifest{}, err
		}
		if s.SelectedCandidateID != c.ID || c.ShotID != s.ID || c.ResultID != r.ID || c.GenerationID != r.GenerationID || r.Status != "ARCHIVED" {
			return ExportManifest{}, errors.New("sequence media not selected/archived")
		}
		if err = read(w.db, "archive_jobs", r.ArchiveJobID, &j); err != nil {
			return ExportManifest{}, err
		}
		if err = w.verifyArchive(j); err != nil {
			return ExportManifest{}, err
		}
		if r.ArchivedRelativePath != j.TargetRelativePath || r.SHA256 != j.ActualSHA256 || r.ByteLength != j.ActualBytes {
			return ExportManifest{}, errors.New("archive metadata mismatch")
		}
		source, err := w.Resolve(r.ArchivedRelativePath)
		if err != nil {
			return ExportManifest{}, err
		}
		f, err := os.Open(source)
		if err != nil {
			return ExportManifest{}, err
		}
		relative := fmt.Sprintf("exports/%s/media/%03d_%s%s", m.ExportID, index+1, r.ID, filepath.Ext(source))
		temp, _, _, err := w.tempFile(relative, f, r.ByteLength, r.SHA256)
		f.Close()
		if err != nil {
			return ExportManifest{}, err
		}
		err = w.publish(temp, relative)
		os.Remove(temp)
		if err != nil {
			return ExportManifest{}, err
		}
		if err = w.verifyFile(relative, r.SHA256, r.ByteLength); err != nil {
			return ExportManifest{}, err
		}
		m.Items = append(m.Items, ManifestItem{m.ExportID, index + 1, i.ID, s.ID, c.ID, r.ID, relative, r.SHA256, r.ByteLength, r.DurationSeconds})
	}
	var buf bytes.Buffer
	csvWriter := csv.NewWriter(&buf)
	if err = csvWriter.Write([]string{"exportId", "sequenceIndex", "sequenceItemId", "shotId", "candidateId", "resultId", "relativePath", "sha256", "byteLength", "duration"}); err != nil {
		return ExportManifest{}, err
	}
	for _, i := range m.Items {
		duration := ""
		if i.DurationSeconds != nil {
			duration = strconv.FormatFloat(*i.DurationSeconds, 'f', -1, 64)
		}
		if err = csvWriter.Write([]string{i.ExportID, strconv.Itoa(i.SequenceIndex), i.SequenceItemID, i.ShotID, i.CandidateID, i.ResultID, i.RelativePath, i.SHA256, strconv.FormatInt(i.ByteLength, 10), duration}); err != nil {
			return ExportManifest{}, err
		}
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		return ExportManifest{}, err
	}
	relative := "exports/" + m.ExportID + "/ordered-manifest.csv"
	temp, _, _, err := w.tempFile(relative, &buf, int64(buf.Len()), "")
	if err != nil {
		return ExportManifest{}, err
	}
	defer os.Remove(temp)
	if err = w.publish(temp, relative); err != nil {
		return ExportManifest{}, err
	}
	if err = w.jsonFile("exports/"+m.ExportID+"/ordered-manifest.json", m); err != nil {
		return ExportManifest{}, err
	}
	return m, nil
}
