package service

import (
	"encoding/json"
	"errors"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var ErrHNEditorialInput = errors.New("invalid HN editorial input")
var ErrHNEditorialOwnership = errors.New("EDITORIAL_OWNERSHIP_CONFLICT")
var ErrHNCandidateIdentity = errors.New("CANDIDATE_IDENTITY_CONFLICT")

type HNCandidate struct {
	CandidateID        string `json:"candidateId"`
	ProjectID          string `json:"projectId"`
	ShotID             string `json:"shotId"`
	GenerationID       string `json:"generationId"`
	ResultID           string `json:"resultId"`
	Label              string `json:"label"`
	AvailabilityStatus string `json:"availabilityStatus"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type HNSelection struct {
	ShotID              string `json:"shotId"`
	SelectedCandidateID string `json:"selectedCandidateId"`
}

type HNSequenceItem struct {
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

func hnEditorialIDs(ids ...string) bool {
	for _, id := range ids {
		if !validHNReferenceName(id) {
			return false
		}
	}
	return true
}

// Called only after Open reconciliation and under the shared local writer lock.
func hnArchivedEditorialResult(w *foundation.Workspace, shotID, resultID string) (foundation.Shot, foundation.Result, error) {
	var s foundation.Shot
	var r foundation.Result
	var g foundation.Generation
	var j foundation.ArchiveJob
	bad := func() (foundation.Shot, foundation.Result, error) { return s, r, ErrHNEditorialOwnership }
	if !hnEditorialIDs(shotID, resultID) || hnReadRecord(w, "shots", shotID, &s) != nil || s.ID != shotID || s.ProjectID != w.ProjectID || hnReadRecord(w, "results", resultID, &r) != nil || r.ID != resultID || r.ProjectID != w.ProjectID {
		return bad()
	}
	if !hnEditorialIDs(r.GenerationID, r.ArchiveJobID) || hnReadRecord(w, "generations", r.GenerationID, &g) != nil || g.ID != r.GenerationID || g.ProjectID != w.ProjectID || !g.Frozen || g.ShotID != s.ID {
		return bad()
	}
	if r.Status != "ARCHIVED" || r.ArchivedRelativePath == "" || !hnHash.MatchString(r.SHA256) || r.ByteLength <= 0 || hnReadRecord(w, "archive_jobs", r.ArchiveJobID, &j) != nil {
		return bad()
	}
	if j.ID != r.ArchiveJobID || j.ProjectID != w.ProjectID || j.ResultID != r.ID || j.GenerationID != g.ID || j.Status != "ARCHIVED" || j.ActualSHA256 != r.SHA256 || j.ActualBytes != r.ByteLength || j.TargetRelativePath != r.ArchivedRelativePath {
		return bad()
	}
	return s, r, nil
}

func hnEditorialCandidate(w *foundation.Workspace, candidateID string) (foundation.Candidate, foundation.Shot, error) {
	var c foundation.Candidate
	if hnReadRecord(w, "candidates", candidateID, &c) != nil || c.ID != candidateID || c.ProjectID != w.ProjectID || !hnEditorialIDs(c.ShotID, c.ResultID, c.GenerationID) || c.AvailabilityStatus != "ARCHIVED" || !validHNShotText(c.Label, 256) {
		return c, foundation.Shot{}, ErrHNEditorialOwnership
	}
	s, r, err := hnArchivedEditorialResult(w, c.ShotID, c.ResultID)
	if err != nil || c.GenerationID != r.GenerationID {
		return c, s, ErrHNEditorialOwnership
	}
	return c, s, nil
}

func hnCandidateFacts(c foundation.Candidate) HNCandidate {
	return HNCandidate{c.ID, c.ProjectID, c.ShotID, c.GenerationID, c.ResultID, c.Label, c.AvailabilityStatus, c.CreatedAt, c.UpdatedAt}
}
func hnSequenceFacts(i foundation.SequenceItem) HNSequenceItem {
	return HNSequenceItem{i.ID, i.ProjectID, i.SequenceID, i.OrderIndex, i.ShotID, i.CandidateID, i.ResultID, i.CreatedAt, i.UpdatedAt}
}

// Ensure never selects; a repeated label is display metadata, not a rename command.
func EnsureCandidateForArchivedResult(root, projectID, shotID, resultID, label string) (HNCandidate, error) {
	if !hnEditorialIDs(projectID, shotID, resultID) || !validHNShotText(label, 256) {
		return HNCandidate{}, ErrHNEditorialInput
	}
	if label == "" {
		label = "Candidate"
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNCandidate{}, err
	}
	defer w.Close()
	_, r, err := hnArchivedEditorialResult(w, shotID, resultID)
	if err != nil {
		return HNCandidate{}, err
	}
	rows, err := w.List("candidates")
	if err != nil {
		return HNCandidate{}, err
	}
	var found *foundation.Candidate
	for _, raw := range rows {
		var c foundation.Candidate
		if json.Unmarshal(raw, &c) != nil || c.ProjectID != projectID || !validHNReferenceName(c.ID) {
			return HNCandidate{}, ErrHNCandidateIdentity
		}
		if c.ShotID == shotID && c.ResultID == resultID {
			if found != nil {
				return HNCandidate{}, ErrHNCandidateIdentity
			}
			copy := c
			found = &copy
		}
	}
	if found != nil {
		c, _, err := hnEditorialCandidate(w, found.ID)
		if err != nil {
			return HNCandidate{}, err
		}
		return hnCandidateFacts(c), nil
	}
	c, err := w.CreateCandidate(shotID, resultID, label)
	if err != nil {
		return HNCandidate{}, err
	}
	if c.GenerationID != r.GenerationID || c.AvailabilityStatus != "ARCHIVED" {
		return HNCandidate{}, ErrHNEditorialOwnership
	}
	return hnCandidateFacts(c), nil
}

func SelectLocalCandidate(root, projectID, shotID, candidateID string) (HNSelection, error) {
	if !hnEditorialIDs(projectID, shotID, candidateID) {
		return HNSelection{}, ErrHNEditorialInput
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNSelection{}, err
	}
	defer w.Close()
	c, s, err := hnEditorialCandidate(w, candidateID)
	if err != nil {
		return HNSelection{}, err
	}
	if s.ID != shotID {
		return HNSelection{}, ErrHNEditorialOwnership
	}
	if err = w.SelectCandidate(shotID, c.ID); err != nil {
		return HNSelection{}, err
	}
	return HNSelection{shotID, c.ID}, nil
}

// Every explicit add is a new placement; intentional repeats are never deduplicated.
func AddLocalSequenceItem(root, projectID, sequenceID, candidateID string) (HNSequenceItem, error) {
	if !hnEditorialIDs(projectID, sequenceID, candidateID) {
		return HNSequenceItem{}, ErrHNEditorialInput
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNSequenceItem{}, err
	}
	defer w.Close()
	c, s, err := hnEditorialCandidate(w, candidateID)
	if err != nil {
		return HNSequenceItem{}, err
	}
	if s.SelectedCandidateID != c.ID {
		return HNSequenceItem{}, ErrHNEditorialOwnership
	}
	i, err := w.AddSequenceItem(sequenceID, c.ID)
	return hnSequenceFacts(i), err
}
