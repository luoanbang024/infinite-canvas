package service

import "github.com/tigerowo/infinite-canvas/hn/foundation"

// Metadata only: existing-store opener never creates/reconciles/reads media.
func reorderStore(root, project string, read bool) (*foundation.Workspace, error) {
	if !hnEditorialIDs(project) {
		return nil, foundation.ErrReorderInput
	}
	w, e := foundation.OpenExisting(root, project, read)
	if e != nil {
		return nil, foundation.ErrReorderUnavailable
	}
	return w, nil
}
func InitializeLocalReorder(root, project string) (foundation.ReorderState, error) {
	w, e := reorderStore(root, project, false)
	if e != nil {
		return foundation.ReorderState{}, e
	}
	defer w.Close()
	return w.InitializeReorderProtocol()
}
func ReadLocalReorderSnapshot(root, project string) (foundation.ReorderSnapshot, error) {
	w, e := reorderStore(root, project, true)
	if e != nil {
		return foundation.ReorderSnapshot{}, e
	}
	defer w.Close()
	return w.ReadReorderSnapshot()
}
func ExecuteLocalReorder(root, project string, c foundation.ReorderCommand) (foundation.ReorderReceipt, error) {
	if !foundation.ValidReorderCommand(c) {
		return foundation.ReorderReceipt{}, foundation.ErrReorderInput
	}
	w, e := reorderStore(root, project, false)
	if e != nil {
		return foundation.ReorderReceipt{}, e
	}
	defer w.Close()
	return w.ExecuteReorderCommand(c)
}
func LookupLocalReorder(root, project, intent string) (any, error) {
	w, e := reorderStore(root, project, true)
	if e != nil {
		return nil, e
	}
	defer w.Close()
	return w.LookupReorderCommand(intent)
}
