package service

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var ErrHNShotInput = errors.New("invalid HN shot identity")
var ErrHNShotIdentityConflict = errors.New("SHOT_IDENTITY_CONFLICT")
var hnShotUnsafe = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://|^(?:[a-z]:[\\/]|[\\/])|[\x00-\x1f\x7f]|^(?:data|blob):)`)

type HNShot struct {
	ShotID       string `json:"shotId"`
	ProjectID    string `json:"projectId"`
	SourceNodeID string `json:"sourceNodeId"`
	Label        string `json:"label"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

func validHNShotText(text string, max int) bool {
	return len(text) <= max && !hnUnsafeText.MatchString(text) && !hnShotUnsafe.MatchString(text)
}

// Exact source-node correlation within a project; labels never determine identity.
func EnsureShotForSourceNode(root, projectID, sourceNodeID, label string) (HNShot, error) {
	if !validHNReferenceName(projectID) || strings.TrimSpace(sourceNodeID) == "" || !validHNShotText(sourceNodeID, 128) || !validHNShotText(label, 256) {
		return HNShot{}, ErrHNShotInput
	}
	if strings.TrimSpace(label) == "" {
		label = "Shot"
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNShot{}, err
	}
	defer w.Close()
	rows, err := w.List("shots")
	if err != nil {
		return HNShot{}, err
	}
	var found *foundation.Shot
	for _, raw := range rows {
		var shot foundation.Shot
		if json.Unmarshal(raw, &shot) != nil || shot.ProjectID != projectID || !validHNReferenceName(shot.ID) {
			return HNShot{}, ErrHNShotIdentityConflict
		}
		if shot.SourceNodeID == sourceNodeID {
			if found != nil || !validHNShotText(shot.Label, 256) {
				return HNShot{}, ErrHNShotIdentityConflict
			}
			found = &shot
		}
	}
	if found == nil {
		shot, err := w.CreateShot(label, sourceNodeID)
		if err != nil {
			return HNShot{}, err
		}
		found = &shot
	}
	return HNShot{found.ID, found.ProjectID, found.SourceNodeID, found.Label, found.CreatedAt, found.UpdatedAt}, nil
}
