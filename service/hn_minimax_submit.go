package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/model"
)

const HNMiniMaxOfficialIdentity = "minimax-official-global-v2"
const hnMiniMaxSubmitURL = "https://api.minimax.io/v2/video_generation"
const hnMiniMaxResponseLimit = 64 << 10

var ErrHNMiniMaxPreflight = errors.New("MiniMax official T2V preflight rejected")
var ErrHNMiniMaxSubmit = errors.New("MiniMax official acceptance unavailable")
var hnMiniMaxOfficialRoot = regexp.MustCompile(`(?i)^https://api\.minimax\.io(?::443)?/?$`)
var hnMiniMaxSecretPrefix = regexp.MustCompile(`(?i)^(sk-|gh[pousr]_|github_pat_|AKIA|ASIA)`)

// The authenticated caller supplies ownerID; there is no route/default caller.
// Tests inject synthetic channels and never invoke the real resolver.
type HNMiniMaxChannelResolver func(ownerID, connectionID string) (model.ModelChannel, error)

func ResolveHNMiniMaxOfficialChannel(ownerID, connectionID string) (model.ModelChannel, error) {
	channel, err := SelectUserLocalModelChannelForModel(ownerID, "MiniMax-H3", connectionID)
	if err != nil {
		return model.ModelChannel{}, ErrHNMiniMaxPreflight
	}
	if !validHNMiniMaxChannel(channel, connectionID) {
		return model.ModelChannel{}, ErrHNMiniMaxPreflight
	}
	return channel, nil
}

func validHNMiniMaxChannel(c model.ModelChannel, connectionID string) bool {
	for _, ch := range c.APIKey {
		if ch <= 32 || ch >= 127 {
			return false
		}
	}
	supports := false
	for _, m := range c.Models {
		if m == "MiniMax-H3" {
			supports = true
		}
	}
	return c.ID == connectionID && c.Protocol == "metaso" && supports && c.Enabled && hnMiniMaxOfficialRoot.MatchString(strings.TrimSpace(c.BaseURL)) && strings.TrimSpace(c.APIKey) != "" && !strings.ContainsAny(c.APIKey, "\r\n")
}

func resolveHNMiniMax(ownerID, connectionID string, resolver HNMiniMaxChannelResolver) (channel model.ModelChannel, err error) {
	// Arbitrary resolver text/panic must never escape, including before Begin.
	defer func() {
		if recover() != nil {
			channel = model.ModelChannel{}
			err = ErrHNMiniMaxPreflight
		}
	}()
	channel, err = resolver(ownerID, connectionID)
	if err != nil || !validHNMiniMaxChannel(channel, connectionID) {
		return model.ModelChannel{}, ErrHNMiniMaxPreflight
	}
	return channel, nil
}

type hnMiniMaxText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type hnMiniMaxRequest struct {
	Model      string          `json:"model"`
	Content    []hnMiniMaxText `json:"content"`
	Resolution string          `json:"resolution"`
	Duration   int             `json:"duration"`
	Ratio      string          `json:"ratio"`
}

func hnMiniMaxRequestBody(g foundation.Generation) ([]byte, error) {
	if !g.Frozen || g.Protocol != "metaso" || g.ProviderIdentity != HNMiniMaxOfficialIdentity || g.Model != "MiniMax-H3" || !validHNReferenceName(g.ConnectionID) || hnMiniMaxSecretPrefix.MatchString(g.ConnectionID) || len(g.ReferenceBindings) != 0 || strings.TrimSpace(g.PromptSnapshot) == "" || len(g.PromptSnapshot) > 32768 || hnUnsafeText.MatchString(g.PromptSnapshot) || (g.CredentialRef != "" && g.CredentialRef != g.ConnectionID) {
		return nil, ErrHNMiniMaxPreflight
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(g.Parameters, &fields) != nil || fields == nil {
		return nil, ErrHNMiniMaxPreflight
	}
	values := map[string]string{}
	defaults := map[string]string{"videoMode": "std", "videoNegativePrompt": "", "videoMultiShot": "false", "videoShotType": "intelligence", "videoGenerateAudio": "true", "videoWatermark": "false", "videoCharacterOrientation": "video"}
	for k, raw := range fields {
		if k == "videoMultiPrompt" {
			var shots []json.RawMessage
			if json.Unmarshal(raw, &shots) != nil || shots == nil || len(shots) != 0 {
				return nil, ErrHNMiniMaxPreflight
			}
			continue
		}
		var v *string
		if json.Unmarshal(raw, &v) != nil || v == nil {
			return nil, ErrHNMiniMaxPreflight
		}
		switch k {
		case "videoSeconds", "vquality", "size":
			values[k] = *v
		default:
			def, okay := defaults[k]
			if !okay || (*v != "" && *v != def) {
				return nil, ErrHNMiniMaxPreflight
			}
		}
	}
	// Finite mappings: no arbitrary fallback, rounding or silent clamp.
	duration, durationErr := strconv.Atoi(values["videoSeconds"])
	resolutions := map[string]string{"480": "768P", "480p": "768P", "720": "768P", "720p": "768P", "768": "768P", "768p": "768P", "1080": "2K", "1080p": "2K", "2k": "2K", "4k": "2K"}
	resolution := resolutions[strings.ToLower(strings.TrimSpace(values["vquality"]))]
	ratios := map[string][]string{
		"16:9": {"864x496", "1280x720", "1920x1080", "2560x1440", "3840x2160"},
		"4:3":  {"752x560", "1112x834", "1664x1248", "2224x1668", "3328x2496"},
		"1:1":  {"640x640", "960x960", "1440x1440", "1920x1920", "2880x2880", "1024x1024"},
		"3:4":  {"560x752", "834x1112", "1248x1664", "1668x2224", "2496x3328"},
		"9:16": {"496x864", "720x1280", "1080x1920", "1440x2560", "2160x3840"},
		"21:9": {"992x432", "1470x630", "2206x946", "2940x1260", "4412x1892"},
	}
	ratio := ""
	for name, pixels := range ratios {
		if values["size"] == name {
			ratio = name
		}
		for _, size := range pixels {
			if values["size"] == size {
				ratio = name
			}
		}
	}
	if durationErr != nil || duration < 4 || duration > 15 || strconv.Itoa(duration) != values["videoSeconds"] || resolution == "" || ratio == "" {
		return nil, ErrHNMiniMaxPreflight
	}
	return json.Marshal(hnMiniMaxRequest{g.Model, []hnMiniMaxText{{"text", g.PromptSnapshot}}, resolution, duration, ratio})
}

// Private test seam redirects only the TCP/TLS connection to a loopback fake;
// the public entrypoint accepts no destination/client/transport override.
type hnMiniMaxHTTPOptions struct {
	timeout     time.Duration
	dialContext func(context.Context, string, string) (net.Conn, error)
	tlsConfig   *tls.Config
}
type hnMiniMaxTransport struct {
	ownerID    string
	resolver   HNMiniMaxChannelResolver
	frozenHash string
	options    hnMiniMaxHTTPOptions
}

func (t hnMiniMaxTransport) Submit(ctx context.Context, g foundation.Generation) (SubmissionAcceptance, error) {
	// R10 supplies its exact owned snapshot. Remap it, never cached mutable config.
	if g.FrozenHash != t.frozenHash || g.Status != "SUBMITTING" || g.SubmissionState != "SUBMITTING" {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	body, err := hnMiniMaxRequestBody(g)
	if err != nil {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	channel, err := resolveHNMiniMax(t.ownerID, g.ConnectionID, t.resolver)
	if err != nil {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	timeout := t.options.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := &http.Transport{Proxy: nil, DisableKeepAlives: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, DialContext: t.options.dialContext, TLSClientConfig: t.options.tlsConfig, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 16 << 10}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hnMiniMaxSubmitURL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	req.ContentLength = int64(len(body))
	req.GetBody = nil
	req.Header.Set("Authorization", "Bearer "+channel.APIKey)
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req) // Exactly one call; no status/network/redirect retry.
	if err != nil {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, hnMiniMaxResponseLimit+1))
	if err != nil || len(raw) > hnMiniMaxResponseLimit {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, e := decoder.Token()
	if e != nil || opening != json.Delim('{') {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	key, e := decoder.Token()
	if e != nil || key != "task_id" {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	var taskID string
	if decoder.Decode(&taskID) != nil {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	closing, e := decoder.Token()
	if e != nil || closing != json.Delim('}') || decoder.Decode(&struct{}{}) != io.EOF || !validHNReferenceName(taskID) || hnMiniMaxSecretPrefix.MatchString(taskID) || strings.Contains(taskID, "sk-proj-") || strings.Contains(taskID, "ghp_") || strings.Contains(taskID, "github_pat_") {
		return SubmissionAcceptance{}, ErrHNMiniMaxSubmit
	}
	return SubmissionAcceptance{ProviderTaskID: taskID}, nil
}

func SubmitMiniMaxH3OfficialGeneration(ctx context.Context, root, projectID, generationID, authenticatedOwnerID string, resolver HNMiniMaxChannelResolver) (foundation.TaskBinding, error) {
	return submitHNMiniMax(ctx, root, projectID, generationID, authenticatedOwnerID, resolver, hnMiniMaxHTTPOptions{})
}

func submitHNMiniMax(ctx context.Context, root, projectID, generationID, ownerID string, resolver HNMiniMaxChannelResolver, options hnMiniMaxHTTPOptions) (foundation.TaskBinding, error) {
	if ctx == nil || ctx.Err() != nil || resolver == nil || !validHNReferenceName(ownerID) || !validHNReferenceName(projectID) || !validHNReferenceName(generationID) {
		return foundation.TaskBinding{}, ErrHNMiniMaxPreflight
	}
	// Serialize preflight Open with R10's live attempt, so Open cannot reconcile it.
	hnReferenceWriter.Lock()
	var g foundation.Generation
	err := func() error {
		w, e := foundation.Open(root, projectID)
		if e != nil {
			return ErrHNMiniMaxPreflight
		}
		defer w.Close()
		if hnReadRecord(w, "generations", generationID, &g) != nil || g.Status != "PREPARED" || g.SubmissionState != "PREPARED" {
			return ErrHNMiniMaxPreflight
		}
		if _, e = hnMiniMaxRequestBody(g); e != nil {
			return e
		}
		if _, e = w.FreezeGeneration(g.ID); e != nil {
			return ErrHNMiniMaxPreflight
		}
		return nil
	}()
	hnReferenceWriter.Unlock()
	if err != nil {
		return foundation.TaskBinding{}, ErrHNMiniMaxPreflight
	}
	if _, err = resolveHNMiniMax(ownerID, g.ConnectionID, resolver); err != nil {
		return foundation.TaskBinding{}, ErrHNMiniMaxPreflight
	}
	return SubmitFrozenGeneration(ctx, root, projectID, generationID, hnMiniMaxTransport{ownerID, resolver, g.FrozenHash, options})
}
