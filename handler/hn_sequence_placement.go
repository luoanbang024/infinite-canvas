package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func hnPlacementBoundary(w http.ResponseWriter, r *http.Request, read bool) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if !hnLocalRequest(w, r) {
		return "", false
	}
	if (read && r.Method != "GET") || (!read && r.Method != "POST") {
		FailWithStatus(w, 405, "PLACEMENT_METHOD_INVALID")
		return "", false
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		FailWithStatus(w, 403, "PLACEMENT_HEADERS_INVALID")
		return "", false
	}
	if r.Header.Get("X-HN-Local-Request") != "1" {
		FailWithStatus(w, 403, "PLACEMENT_LOCAL_MARKER_REQUIRED")
		return "", false
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return "", false
	}
	if read {
		b, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(b) != 0 {
			FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
			return "", false
		}
	}
	root := strings.TrimSpace(os.Getenv("HN_PROJECTS_ROOT"))
	if root == "" {
		FailWithStatus(w, 503, "PLACEMENT_STORE_UNAVAILABLE")
		return "", false
	}
	return root, true
}
func hnPlacementResponse(w http.ResponseWriter, data any, err error) {
	if err == nil {
		OK(w, data)
		return
	}
	status, msg := 500, "PLACEMENT_STORE_UNAVAILABLE"
	for _, v := range []struct {
		e      error
		status int
		msg    string
	}{
		{foundation.ErrPlacementInput, 400, "PLACEMENT_INPUT_INVALID"},
		{foundation.ErrPlacementIntegrity, 409, "PLACEMENT_INTEGRITY_CONFLICT"},
		{foundation.ErrPlacementIntent, 409, "PLACEMENT_INTENT_CONFLICT"},
		{foundation.ErrPlacementProtocol, 409, "PLACEMENT_PROTOCOL_NOT_INITIALIZED"},
		{foundation.ErrPlacementCapacity, 409, "READ_CAPACITY_EXCEEDED"},
		{foundation.ErrPlacementUnavailable, 503, "PLACEMENT_STORE_UNAVAILABLE"},
	} {
		if errors.Is(err, v.e) {
			status, msg = v.status, v.msg
			break
		}
	}
	FailWithStatus(w, status, msg)
}
func HNPlacementCommand(w http.ResponseWriter, r *http.Request, projectID, sequenceID string) {
	root, ok := hnPlacementBoundary(w, r, false)
	if !ok {
		return
	}
	if sequenceID != "main" {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	fields := map[string]json.RawMessage{}
	allowed := map[string]bool{"protocolVersion": true, "placementIntentId": true, "candidateId": true, "shotId": true, "generationId": true, "resultId": true, "archiveJobId": true, "preparedFrozenHash": true}
	for d.More() {
		key, err := d.Token()
		name, valid := key.(string)
		if err != nil || !valid || !allowed[name] || fields[name] != nil {
			FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
			return
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
			return
		}
		fields[name] = value
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') || len(fields) != 8 || d.Decode(new(any)) != io.EOF {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	var command foundation.PlacementCommand
	b, _ := json.Marshal(fields)
	if json.Unmarshal(b, &command) != nil || !foundation.ValidPlacementCommand(command) {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	data, err := service.ExecuteLocalPlacement(root, projectID, command)
	hnPlacementResponse(w, data, err)
}
func HNReadMainSequence(w http.ResponseWriter, r *http.Request, projectID, sequenceID string) {
	root, ok := hnPlacementBoundary(w, r, true)
	if !ok {
		return
	}
	if sequenceID != "main" {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	data, err := service.ReadLocalMainSequence(root, projectID)
	hnPlacementResponse(w, data, err)
}
func HNLookupPlacement(w http.ResponseWriter, r *http.Request, projectID, sequenceID, intent string) {
	root, ok := hnPlacementBoundary(w, r, true)
	if !ok {
		return
	}
	if sequenceID != "main" {
		FailWithStatus(w, 400, "PLACEMENT_INPUT_INVALID")
		return
	}
	data, err := service.LookupLocalPlacement(root, projectID, intent)
	hnPlacementResponse(w, data, err)
}

// GET preflight is a separate helper. Existing POST-only OPTIONS is unchanged.
func HNPlacementReadOptions(w http.ResponseWriter, r *http.Request) {
	if !hnLocalRequest(w, r) {
		return
	}
	if r.Header.Get("Access-Control-Request-Method") != http.MethodGet || r.URL.RawQuery != "" {
		FailWithStatus(w, 405, "PLACEMENT_GET_ONLY")
		return
	}
	for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		if strings.TrimSpace(h) != "" && !strings.EqualFold(strings.TrimSpace(h), "X-HN-Local-Request") {
			FailWithStatus(w, 403, "PLACEMENT_HEADERS_INVALID")
			return
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "X-HN-Local-Request")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(204)
}
