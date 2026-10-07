package handler

import (
	"encoding/json"
	"errors"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
	"io"
	"mime"
	"net/http"
	"strings"
)

func hnReorderBoundary(w http.ResponseWriter, r *http.Request, sequence string, read bool) (string, bool) {
	root, ok := hnPlacementBoundary(w, r, read)
	if !ok {
		return "", false
	}
	if sequence != "main" {
		FailWithStatus(w, 400, "REORDER_INPUT_INVALID")
		return "", false
	}
	return root, true
}
func hnReorderBody(w http.ResponseWriter, r *http.Request, keys []string, out any) bool {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return false
	}
	allowed := map[string]bool{}
	for _, k := range keys {
		allowed[k] = true
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, e := d.Token()
		k, ok := t.(string)
		if e != nil || !ok || !allowed[k] || fields[k] != nil {
			return false
		}
		var b json.RawMessage
		if d.Decode(&b) != nil {
			return false
		}
		fields[k] = b
	}
	t, e = d.Token()
	if e != nil || t != json.Delim('}') || len(fields) != len(keys) || d.Decode(new(any)) != io.EOF {
		return false
	}
	b, _ := json.Marshal(fields)
	return json.Unmarshal(b, out) == nil
}
func hnReorderResponse(w http.ResponseWriter, data any, err error) {
	if err == nil {
		OK(w, data)
		return
	}
	status, msg := 500, "REORDER_STORE_UNAVAILABLE"
	for _, v := range []struct {
		e      error
		status int
		msg    string
	}{
		{foundation.ErrReorderInput, 400, "REORDER_INPUT_INVALID"},
		{foundation.ErrReorderIntegrity, 409, "REORDER_INTEGRITY_CONFLICT"},
		{foundation.ErrReorderIntent, 409, "REORDER_INTENT_CONFLICT"},
		{foundation.ErrReorderProtocol, 409, "REORDER_PROTOCOL_NOT_INITIALIZED"},
		{foundation.ErrReorderExhausted, 409, "SEQUENCE_REVISION_EXHAUSTED"},
		{foundation.ErrReorderUnavailable, 503, "REORDER_STORE_UNAVAILABLE"},
		{foundation.ErrPlacementCapacity, 409, "READ_CAPACITY_EXCEEDED"},
		{foundation.ErrPlacementUnavailable, 503, "REORDER_STORE_UNAVAILABLE"},
	} {
		if errors.Is(err, v.e) {
			status, msg = v.status, v.msg
			break
		}
	}
	FailWithStatus(w, status, msg)
}
func HNInitializeReorder(w http.ResponseWriter, r *http.Request, project, sequence string) {
	root, ok := hnReorderBoundary(w, r, sequence, false)
	if !ok {
		return
	}
	var input struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if !hnReorderBody(w, r, []string{"protocolVersion"}, &input) || input.ProtocolVersion != 1 {
		FailWithStatus(w, 400, "REORDER_INPUT_INVALID")
		return
	}
	data, e := service.InitializeLocalReorder(root, project)
	hnReorderResponse(w, data, e)
}
func HNReorderSnapshot(w http.ResponseWriter, r *http.Request, project, sequence string) {
	root, ok := hnReorderBoundary(w, r, sequence, true)
	if !ok {
		return
	}
	data, e := service.ReadLocalReorderSnapshot(root, project)
	hnReorderResponse(w, data, e)
}
func HNReorderCommand(w http.ResponseWriter, r *http.Request, project, sequence string) {
	root, ok := hnReorderBoundary(w, r, sequence, false)
	if !ok {
		return
	}
	var c foundation.ReorderCommand
	if !hnReorderBody(w, r, []string{"protocolVersion", "reorderIntentId", "expectedRevision", "desiredSequenceItemIds"}, &c) || !foundation.ValidReorderCommand(c) {
		FailWithStatus(w, 400, "REORDER_INPUT_INVALID")
		return
	}
	data, e := service.ExecuteLocalReorder(root, project, c)
	if e == nil && data.Outcome == "CONFLICT" {
		// Terminal conflict includes exact immutable receipt, unlike storage errors.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]any{"code": 1, "data": data, "msg": *data.ErrorClass})
		return
	}
	hnReorderResponse(w, data, e)
}
func HNLookupReorder(w http.ResponseWriter, r *http.Request, project, sequence, intent string) {
	root, ok := hnReorderBoundary(w, r, sequence, true)
	if !ok {
		return
	}
	data, e := service.LookupLocalReorder(root, project, intent)
	hnReorderResponse(w, data, e)
}

// Separate OPTIONS never relaxes any existing POST-only preflight.
func HNReorderOptions(w http.ResponseWriter, r *http.Request, read bool) {
	w.Header().Set("Cache-Control", "no-store")
	if !hnLocalRequest(w, r) {
		return
	}
	method := "POST"
	if read {
		method = "GET"
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		FailWithStatus(w, 403, "REORDER_HEADERS_INVALID")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.Header.Get("Access-Control-Request-Method") != method {
		FailWithStatus(w, 405, "REORDER_METHOD_INVALID")
		return
	}
	for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		h = strings.TrimSpace(h)
		if h != "" && !strings.EqualFold(h, "X-HN-Local-Request") && (read || !strings.EqualFold(h, "Content-Type")) {
			FailWithStatus(w, 403, "REORDER_HEADERS_INVALID")
			return
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", method)
	headers := "X-HN-Local-Request"
	if !read {
		headers = "Content-Type, X-HN-Local-Request"
	}
	w.Header().Set("Access-Control-Allow-Headers", headers)
	w.WriteHeader(204)
}
