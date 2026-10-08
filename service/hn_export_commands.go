package service

import "github.com/tigerowo/infinite-canvas/hn/foundation"

func exportCommandStore(root, project string, read bool) (*foundation.Workspace, error) {
	if !hnEditorialIDs(project) {
		return nil, foundation.ErrExportInput
	}
	w, e := foundation.OpenExisting(root, project, read)
	if e != nil {
		return nil, foundation.ErrExportUnavailable
	}
	return w, nil
}
func InitializeLocalExport(root, project string) (foundation.ExportProtocolState, error) {
	w, e := exportCommandStore(root, project, false)
	if e != nil {
		return foundation.ExportProtocolState{}, e
	}
	defer w.Close()
	return w.InitializeExportProtocol()
}
func ExecuteLocalExportCommand(root, project string, c foundation.ExportCommand) (foundation.ExportStatus, error) {
	if !foundation.ValidExportCommand(c) {
		return foundation.ExportStatus{}, foundation.ErrExportInput
	}
	w, e := exportCommandStore(root, project, false)
	if e != nil {
		return foundation.ExportStatus{}, e
	}
	defer w.Close()
	return w.ExecuteExportCommand(c)
}
func LookupLocalExportCommand(root, project, intent string) (foundation.ExportStatus, error) {
	w, e := exportCommandStore(root, project, true)
	if e != nil {
		return foundation.ExportStatus{}, e
	}
	defer w.Close()
	return w.LookupExportCommand(intent)
}
func VerifyLocalExportBundle(root, project, intent string) (foundation.ExportBundleHealth, error) {
	w, e := exportCommandStore(root, project, true)
	if e != nil {
		return foundation.ExportBundleHealth{}, e
	}
	defer w.Close()
	return w.VerifyExportBundle(intent)
}
