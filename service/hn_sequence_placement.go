package service

import "github.com/tigerowo/infinite-canvas/hn/foundation"

// These entry points intentionally bypass Open/reconciliation: only metadata of
// an already existing project is involved, including on the write protocol.
func ExecuteLocalPlacement(root, projectID string, command foundation.PlacementCommand) (foundation.PlacementReceipt, error) {
	if !hnEditorialIDs(projectID) || !foundation.ValidPlacementCommand(command) {
		return foundation.PlacementReceipt{}, foundation.ErrPlacementInput
	}
	w, err := foundation.OpenExisting(root, projectID, false)
	if err != nil {
		return foundation.PlacementReceipt{}, err
	}
	defer w.Close()
	return w.ExecutePlacementCommand(command)
}
func ReadLocalMainSequence(root, projectID string) (foundation.SequenceSnapshot, error) {
	if !hnEditorialIDs(projectID) {
		return foundation.SequenceSnapshot{}, foundation.ErrPlacementInput
	}
	w, err := foundation.OpenExisting(root, projectID, true)
	if err != nil {
		return foundation.SequenceSnapshot{}, err
	}
	defer w.Close()
	return w.ReadMainSequence()
}
func LookupLocalPlacement(root, projectID, intent string) (any, error) {
	if !hnEditorialIDs(projectID) {
		return nil, foundation.ErrPlacementInput
	}
	w, err := foundation.OpenExisting(root, projectID, true)
	if err != nil {
		return nil, err
	}
	defer w.Close()
	return w.LookupPlacementCommand(intent)
}
