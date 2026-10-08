package handler

import (
	"errors"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
	"net/http"
)

// Reuse the audited strict boundary BEFORE any dynamic sequence validation.
func hnExportBoundary(w http.ResponseWriter, r *http.Request, sequence string, read bool) (string, bool) {
	root, ok := hnPlacementBoundary(w, r, read)
	if !ok {
		return "", false
	}
	if sequence != "main" {
		FailWithStatus(w, 400, "EXPORT_INPUT_INVALID")
		return "", false
	}
	return root, true
}
func hnExportCommandResponse(w http.ResponseWriter, data any, e error) {
	if e == nil {
		OK(w, data)
		return
	}
	status, msg := 503, "EXPORT_STORE_UNAVAILABLE"
	for _, v := range []struct {
		e      error
		status int
		msg    string
	}{{foundation.ErrExportInput, 400, "EXPORT_INPUT_INVALID"}, {foundation.ErrExportIntegrity, 409, "EXPORT_INTEGRITY_CONFLICT"}, {foundation.ErrExportIntent, 409, "EXPORT_INTENT_CONFLICT"}, {foundation.ErrExportProtocol, 409, "EXPORT_PROTOCOL_NOT_INITIALIZED"}, {foundation.ErrReorderProtocol, 409, "REORDER_PROTOCOL_NOT_INITIALIZED"}, {foundation.ErrReorderIntegrity, 409, "EXPORT_INTEGRITY_CONFLICT"}} {
		if errors.Is(e, v.e) {
			status, msg = v.status, v.msg
			break
		}
	}
	FailWithStatus(w, status, msg)
}
func HNInitializeExportProtocol(w http.ResponseWriter, r *http.Request, project, sequence string) {
	root, ok := hnExportBoundary(w, r, sequence, false)
	if !ok {
		return
	}
	var p struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if !hnReorderBody(w, r, []string{"protocolVersion"}, &p) || p.ProtocolVersion != 1 {
		FailWithStatus(w, 400, "EXPORT_INPUT_INVALID")
		return
	}
	s, e := service.InitializeLocalExport(root, project)
	hnExportCommandResponse(w, s, e)
}
func HNExportCommand(w http.ResponseWriter, r *http.Request, project, sequence string) {
	root, ok := hnExportBoundary(w, r, sequence, false)
	if !ok {
		return
	}
	var c foundation.ExportCommand
	if !hnReorderBody(w, r, []string{"protocolVersion", "exportIntentId", "expectedRevision", "orderedSequenceItemIds", "format"}, &c) || !foundation.ValidExportCommand(c) {
		FailWithStatus(w, 400, "EXPORT_INPUT_INVALID")
		return
	}
	s, e := service.ExecuteLocalExportCommand(root, project, c)
	hnExportCommandResponse(w, s, e)
}
func HNLookupExportCommand(w http.ResponseWriter, r *http.Request, project, sequence, intent string) {
	root, ok := hnExportBoundary(w, r, sequence, true)
	if !ok {
		return
	}
	s, e := service.LookupLocalExportCommand(root, project, intent)
	hnExportCommandResponse(w, s, e)
}
func HNVerifyExportBundle(w http.ResponseWriter, r *http.Request, project, sequence, intent string) {
	root, ok := hnExportBoundary(w, r, sequence, true)
	if !ok {
		return
	}
	s, e := service.VerifyLocalExportBundle(root, project, intent)
	hnExportCommandResponse(w, s, e)
}
func HNExportCommandOptions(w http.ResponseWriter, r *http.Request, read bool) {
	HNReorderOptions(w, r, read)
}
