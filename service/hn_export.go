package service

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var ErrHNExportInput = errors.New("invalid HN export input")
var ErrHNExportIncomplete = errors.New("HN_EXPORT_INCOMPLETE")

type HNExport struct {
	foundation.ExportManifest
	BundleRelativePath       string `json:"bundleRelativePath"`
	ManifestJSONRelativePath string `json:"manifestJsonRelativePath"`
	ManifestCSVRelativePath  string `json:"manifestCsvRelativePath"`
	ItemCount                int    `json:"itemCount"`
}

// Export owns the copy, hash checks and completion marker. This boundary only
// validates its returned facts and never resolves a different Candidate.
func ExportLocalSequence(root, projectID, sequenceID string) (HNExport, error) {
	if !hnEditorialIDs(projectID, sequenceID) || strings.TrimSpace(root) == "" {
		return HNExport{}, ErrHNExportInput
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNExport{}, ErrHNExportIncomplete
	}
	defer w.Close()
	m, err := w.Export(sequenceID)
	if err != nil {
		return HNExport{}, ErrHNExportIncomplete
	}
	if m.SchemaVersion != 1 || m.ProjectID != projectID || m.SequenceID != sequenceID || !hnEditorialIDs(m.ExportID) || len(m.Items) == 0 {
		return HNExport{}, ErrHNExportIncomplete
	}
	bundle := "exports/" + m.ExportID
	seen := map[string]bool{}
	for n, i := range m.Items {
		if i.ExportID != m.ExportID || i.SequenceIndex != n+1 || !hnEditorialIDs(i.SequenceItemID, i.ShotID, i.CandidateID, i.ResultID) || seen[i.SequenceItemID] || !strings.HasPrefix(i.RelativePath, bundle+"/media/") || !hnHash.MatchString(i.SHA256) || i.ByteLength <= 0 {
			return HNExport{}, ErrHNExportIncomplete
		}
		if i.DurationSeconds != nil && (math.IsNaN(*i.DurationSeconds) || math.IsInf(*i.DurationSeconds, 0) || *i.DurationSeconds < 0) {
			return HNExport{}, ErrHNExportIncomplete
		}
		seen[i.SequenceItemID] = true
		p, e := w.Resolve(i.RelativePath)
		if e != nil {
			return HNExport{}, ErrHNExportIncomplete
		}
		info, e := os.Stat(p)
		if e != nil || !info.Mode().IsRegular() || info.Size() != i.ByteLength {
			return HNExport{}, ErrHNExportIncomplete
		}
	}
	out := HNExport{m, bundle, bundle + "/ordered-manifest.json", bundle + "/ordered-manifest.csv", len(m.Items)}
	p, err := w.Resolve(out.ManifestJSONRelativePath)
	if err != nil {
		return HNExport{}, ErrHNExportIncomplete
	}
	b, err := os.ReadFile(p)
	var stored foundation.ExportManifest
	if err != nil || json.Unmarshal(b, &stored) != nil || !reflect.DeepEqual(m, stored) {
		return HNExport{}, ErrHNExportIncomplete
	}
	p, err = w.Resolve(out.ManifestCSVRelativePath)
	if err != nil {
		return HNExport{}, ErrHNExportIncomplete
	}
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return HNExport{}, ErrHNExportIncomplete
	}
	return out, nil
}
