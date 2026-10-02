package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var (
	ErrHNMiniMaxPollPreflight     = errors.New("MiniMax known-task poll preflight rejected")
	ErrHNMiniMaxPollAuth          = errors.New("MiniMax task query authentication unavailable")
	ErrHNMiniMaxPollNotFound      = errors.New("MiniMax known task not found or no longer queryable")
	ErrHNMiniMaxPollTransient     = errors.New("MiniMax task query temporarily unavailable")
	ErrHNMiniMaxPollProtocol      = errors.New("MiniMax task query protocol rejected")
	ErrHNMiniMaxPollStateConflict = errors.New("POLL_STATE_CONFLICT")
)

// An observation, not a durable provider status or a result/download capability.
type HNProviderTaskObservation struct {
	TaskBindingID           string `json:"taskBindingId"`
	ProviderTaskID          string `json:"providerTaskId"`
	State                   string `json:"state"`
	Terminal                bool   `json:"terminal"`
	ResultLocationAvailable bool   `json:"resultLocationAvailable"`
	PolledAt                string `json:"polledAt"`
	SafeFailureClass        string `json:"safeFailureClass,omitempty"`
}

func hnMiniMaxSafeTaskID(id string) bool {
	return validHNReferenceName(id) && !hnMiniMaxSecretPrefix.MatchString(id) && !strings.Contains(id, "sk-proj-") && !strings.Contains(id, "ghp_") && !strings.Contains(id, "github_pat_")
}

func hnMiniMaxKnownTask(g foundation.Generation, b foundation.TaskBinding, projectID, bindingID string) bool {
	return b.ID == bindingID && b.ProjectID == projectID && g.ProjectID == projectID && b.SchemaVersion == foundation.SchemaVersion && g.SchemaVersion == foundation.SchemaVersion && validHNReferenceName(g.ID) && g.Frozen && g.Status == "SUBMITTED" && g.SubmissionState == "SUBMITTED" && b.GenerationID == g.ID && b.BindingState == "BOUND" && hnMiniMaxSafeTaskID(b.ProviderTaskID) && b.ConnectionID == g.ConnectionID && b.Protocol == g.Protocol && b.ProviderIdentity == g.ProviderIdentity && g.Protocol == "metaso" && g.ProviderIdentity == HNMiniMaxOfficialIdentity && g.Model == "MiniMax-H3"
}

// Open performs existing submission reconciliation. A read-only gate rejects
// nonaccepted tasks before Open, so polling cannot reconcile an ambiguous submit.
// No schema or Foundation production behavior is changed. The normal writer is
// held; cross-process workspace availability remains outside this boundary.
func hnMiniMaxPollReadGate(root, projectID, bindingID string) error {
	parent, err := filepath.Abs(root)
	if err != nil {
		return ErrHNMiniMaxPollPreflight
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return ErrHNMiniMaxPollPreflight
	}
	guard := foundation.Workspace{Root: filepath.Join(parent, projectID), ProjectID: projectID}
	path, err := guard.Resolve("metadata/hn-extension.sqlite")
	if err != nil {
		return ErrHNMiniMaxPollPreflight
	}
	uriPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return ErrHNMiniMaxPollPreflight
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var rawB, rawG string
	var g foundation.Generation
	var b foundation.TaskBinding
	var inFlight int
	if db.QueryRow("SELECT data FROM task_bindings WHERE id=? AND project_id=?", bindingID, projectID).Scan(&rawB) != nil || json.Unmarshal([]byte(rawB), &b) != nil || db.QueryRow("SELECT data FROM generations WHERE id=? AND project_id=?", b.GenerationID, projectID).Scan(&rawG) != nil || json.Unmarshal([]byte(rawG), &g) != nil || !hnMiniMaxKnownTask(g, b, projectID, bindingID) || db.QueryRow("SELECT count(*) FROM generations WHERE project_id=? AND json_extract(data,'$.submissionState')='SUBMITTING'", projectID).Scan(&inFlight) != nil || inFlight != 0 {
		return ErrHNMiniMaxPollPreflight
	}
	return nil
}

// Caller holds hnReferenceWriter. Open/Freeze on a frozen record verifies its
// immutable request hash and reference/snapshot integrity without rewriting it.
func hnMiniMaxPollSnapshot(root, projectID, bindingID string) (g foundation.Generation, b foundation.TaskBinding, err error) {
	if hnMiniMaxPollReadGate(root, projectID, bindingID) != nil {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	defer w.Close()
	if hnReadRecord(w, "task_bindings", bindingID, &b) != nil || hnReadRecord(w, "generations", b.GenerationID, &g) != nil || !hnMiniMaxKnownTask(g, b, projectID, bindingID) {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	rows, err := w.List("task_bindings")
	if err != nil {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	owners := 0
	for _, raw := range rows {
		var owner foundation.TaskBinding
		if json.Unmarshal(raw, &owner) != nil {
			return g, b, ErrHNMiniMaxPollPreflight
		}
		if owner.GenerationID == g.ID {
			owners++
		}
	}
	if owners != 1 {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	if _, err = w.FreezeGeneration(g.ID); err != nil {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	if _, err = hnMiniMaxRequestBody(g); err != nil {
		return g, b, ErrHNMiniMaxPollPreflight
	}
	return g, b, nil
}

// Reject duplicate object keys and trailing JSON instead of accepting whichever
// identity/status appears last. Unknown provider metadata is never propagated.
func hnMiniMaxPollObject(raw []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, ErrHNMiniMaxPollProtocol
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		key, err := d.Token()
		name, okay := key.(string)
		if err != nil || !okay || fields[name] != nil {
			return nil, ErrHNMiniMaxPollProtocol
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, ErrHNMiniMaxPollProtocol
		}
		fields[name] = value
	}
	closing, err := d.Token()
	if err != nil || closing != json.Delim('}') || d.Decode(&struct{}{}) != io.EOF {
		return nil, ErrHNMiniMaxPollProtocol
	}
	return fields, nil
}

func hnMiniMaxPollString(fields map[string]json.RawMessage, name string) (string, bool) {
	var value *string
	err := json.Unmarshal(fields[name], &value)
	if err != nil || value == nil {
		return "", false
	}
	return *value, true
}

func hnMiniMaxPollParse(raw []byte, b foundation.TaskBinding, expected hnMiniMaxRequest) (HNProviderTaskObservation, error) {
	zero := HNProviderTaskObservation{}
	root, err := hnMiniMaxPollObject(raw)
	if err != nil {
		return zero, err
	}
	task, err := hnMiniMaxPollObject(root["task"])
	if err != nil {
		return zero, err
	}
	for name, want := range map[string]string{"id": b.ProviderTaskID, "model": "MiniMax-H3", "task_type": "generation"} {
		if got, okay := hnMiniMaxPollString(task, name); !okay || got != want {
			return zero, ErrHNMiniMaxPollProtocol
		}
	}
	for name, want := range map[string]string{"modality": "video", "resolution": expected.Resolution, "ratio": expected.Ratio} {
		if _, exists := task[name]; exists {
			if got, okay := hnMiniMaxPollString(task, name); !okay || got != want {
				return zero, ErrHNMiniMaxPollProtocol
			}
		}
	}
	if value, exists := task["duration"]; exists {
		var duration *float64
		if json.Unmarshal(value, &duration) != nil || duration == nil || *duration != float64(expected.Duration) {
			return zero, ErrHNMiniMaxPollProtocol
		}
	}
	state, okay := hnMiniMaxPollString(task, "status")
	if !okay {
		return zero, ErrHNMiniMaxPollProtocol
	}
	obs := HNProviderTaskObservation{TaskBindingID: b.ID, ProviderTaskID: b.ProviderTaskID, State: state}
	switch state {
	case "queued", "running":
	case "succeeded":
		content, err := hnMiniMaxPollObject(task["content"])
		if err != nil {
			return zero, err
		}
		location, okay := hnMiniMaxPollString(content, "url")
		u, err := url.Parse(location)
		if !okay || err != nil || len(location) > 16384 || strings.TrimSpace(location) != location || strings.ContainsAny(location, "\r\n\t ") || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
			return zero, ErrHNMiniMaxPollProtocol
		}
		obs.Terminal, obs.ResultLocationAvailable = true, true
	case "failed":
		obs.Terminal, obs.SafeFailureClass = true, "MINIMAX_TASK_FAILED"
	case "cancelled":
		obs.Terminal, obs.SafeFailureClass = true, "MINIMAX_TASK_CANCELLED"
	default:
		return zero, ErrHNMiniMaxPollProtocol
	}
	return obs, nil // No raw URL/error/body crosses this return boundary.
}

func hnMiniMaxPollQuery(ctx context.Context, channelKey string, b foundation.TaskBinding, expected hnMiniMaxRequest, options hnMiniMaxHTTPOptions) (HNProviderTaskObservation, error) {
	timeout := options.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := &http.Transport{Proxy: nil, DisableKeepAlives: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, DialContext: options.dialContext, TLSClientConfig: options.tlsConfig, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 16 << 10}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.minimax.io/v2/query/video_generation/"+url.PathEscape(b.ProviderTaskID), nil)
	if err != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollPreflight
	}
	req.Header.Set("Authorization", "Bearer "+channelKey)
	response, err := client.Do(req) // Exactly one GET; fresh connection, no retry/failover.
	if err != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollTransient
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case 401, 403:
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollAuth
	case 404:
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollNotFound
	case 429:
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollTransient
	default:
		if response.StatusCode >= 500 && response.StatusCode <= 599 {
			return HNProviderTaskObservation{}, ErrHNMiniMaxPollTransient
		}
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollProtocol
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, hnMiniMaxResponseLimit+1))
	if err != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollTransient
	}
	if len(raw) > hnMiniMaxResponseLimit {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollProtocol
	}
	return hnMiniMaxPollParse(raw, b, expected)
}

// Compiled but not enabled: no route, scheduler, startup worker or live caller.
// Tests exclusively inject synthetic channel resolution and loopback TLS dial.
func PollMiniMaxH3OfficialTask(ctx context.Context, root, projectID, taskBindingID, authenticatedOwnerID string, resolver HNMiniMaxChannelResolver) (HNProviderTaskObservation, error) {
	return pollHNMiniMax(ctx, root, projectID, taskBindingID, authenticatedOwnerID, resolver, hnMiniMaxHTTPOptions{})
}

func pollHNMiniMax(ctx context.Context, root, projectID, bindingID, ownerID string, resolver HNMiniMaxChannelResolver, options hnMiniMaxHTTPOptions) (obs HNProviderTaskObservation, err error) {
	defer func() {
		if recover() != nil {
			obs, err = HNProviderTaskObservation{}, ErrHNMiniMaxPollTransient
		}
	}()
	if ctx == nil || resolver == nil || !hnMiniMaxSafeTaskID(ownerID) || !validHNReferenceName(projectID) || !validHNReferenceName(bindingID) {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollPreflight
	}
	if ctx.Err() != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollTransient
	}
	var g foundation.Generation
	var b foundation.TaskBinding
	err = func() error {
		hnReferenceWriter.Lock()
		defer hnReferenceWriter.Unlock()
		var e error
		g, b, e = hnMiniMaxPollSnapshot(root, projectID, bindingID)
		return e
	}()
	if err != nil {
		return HNProviderTaskObservation{}, err
	}
	channel, err := resolveHNMiniMax(ownerID, g.ConnectionID, resolver)
	if err != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollPreflight
	}
	body, err := hnMiniMaxRequestBody(g)
	var expected hnMiniMaxRequest
	if err != nil || json.Unmarshal(body, &expected) != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollPreflight
	}
	obs, err = hnMiniMaxPollQuery(ctx, channel.APIKey, b, expected, options)
	if err != nil {
		return HNProviderTaskObservation{}, err
	}
	// Network is outside the writer. Re-read current facts, preserve concurrent
	// terminal classes and merge only polling timestamp/safe failure class.
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	currentG, currentB, err := hnMiniMaxPollSnapshot(root, projectID, bindingID)
	if err != nil || !reflect.DeepEqual(currentG, g) || currentB.GenerationID != b.GenerationID || currentB.ProviderTaskID != b.ProviderTaskID || currentB.ConnectionID != b.ConnectionID || currentB.UpstreamLocalTaskID != b.UpstreamLocalTaskID || currentB.CreatedAt != b.CreatedAt || currentB.ErrorClass != "" && currentB.ErrorClass != obs.SafeFailureClass {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollStateConflict
	}
	polledAt := time.Now().UTC().Format(time.RFC3339Nano)
	currentB.LastPolledAt = polledAt
	if obs.SafeFailureClass != "" {
		currentB.ErrorClass = obs.SafeFailureClass
	}
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollStateConflict
	}
	defer w.Close()
	if w.UpdateTaskBinding(currentB) != nil {
		return HNProviderTaskObservation{}, ErrHNMiniMaxPollStateConflict
	}
	obs.PolledAt = polledAt
	return obs, nil
}
