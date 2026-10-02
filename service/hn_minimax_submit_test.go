package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/model"
)

const hnMiniMaxSyntheticKey = "HN_R12_SYNTHETIC_SENTINEL"
const hnMiniMaxFakeTask = "424010985738629"

func hnMiniMaxSyntheticChannel() model.ModelChannel {
	return model.ModelChannel{ID: "official-h3", Protocol: "metaso", Models: []string{"MiniMax-H3"}, BaseURL: "https://api.minimax.io", APIKey: hnMiniMaxSyntheticKey, Enabled: true}
}
func hnMiniMaxSyntheticResolver(owner, connection string) (model.ModelChannel, error) {
	if owner != "test-owner" || connection != "official-h3" {
		return model.ModelChannel{}, errors.New(hnMiniMaxSyntheticKey)
	}
	return hnMiniMaxSyntheticChannel(), nil
}
func hnMiniMaxPrepare(t *testing.T, root string, edit func(*HNGenerationPrepareInput)) HNPreparedGeneration {
	t.Helper()
	shot, e := EnsureShotForSourceNode(root, "test", "video", "synthetic H3 shot")
	if e != nil {
		t.Fatal(e)
	}
	i := hnTestInput()
	i.NodeID = "video"
	i.ShotID = shot.ShotID
	i.Protocol = "metaso"
	i.ProviderIdentity = HNMiniMaxOfficialIdentity
	i.Model = "MiniMax-H3"
	i.ConnectionID = "official-h3"
	i.Parameters = json.RawMessage(`{"videoSeconds":"6","vquality":"720","size":"1280x720","videoMode":"std","videoNegativePrompt":"","videoMultiShot":"false","videoShotType":"intelligence","videoMultiPrompt":[],"videoGenerateAudio":"true","videoWatermark":"false","videoCharacterOrientation":"video"}`)
	if edit != nil {
		edit(&i)
	}
	p, e := PrepareLocalGeneration(root, "test", i)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func hnMiniMaxLocalOptions(t *testing.T, server *httptest.Server) hnMiniMaxHTTPOptions {
	t.Helper()
	u, e := url.Parse(server.URL)
	if e != nil || u.Hostname() != "127.0.0.1" {
		t.Fatal("not loopback fixture")
	}
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	return hnMiniMaxHTTPOptions{timeout: 2 * time.Second, tlsConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}, dialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "api.minimax.io:443" {
			return nil, errors.New("un-pinned destination")
		}
		return (&net.Dialer{}).DialContext(ctx, network, u.Host)
	}}
}
func hnMiniMaxCall(ctx context.Context, root, id string, options hnMiniMaxHTTPOptions, resolver HNMiniMaxChannelResolver) (foundation.TaskBinding, error) {
	// Even a broken preflight must never let a negative test reach the real host.
	if options.dialContext == nil {
		options.dialContext = func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("unexpected network in offline fixture")
		}
	}
	return submitHNMiniMax(ctx, root, "test", id, "test-owner", resolver, options)
}
func hnMiniMaxAssertState(t *testing.T, root, id, state string, posts *atomic.Int32, before foundation.Generation) {
	t.Helper()
	g, bs := hnSubmissionRead(t, root, id)
	if g.Status != state || g.SubmissionState != state || len(bs) != 1 {
		t.Fatal("unexpected submission state/owner")
	}
	hnSubmissionUnchanged(t, before, g)
	if state == "SUBMITTED" {
		if bs[0].BindingState != "BOUND" || bs[0].ProviderTaskID != hnMiniMaxFakeTask || bs[0].UpstreamLocalTaskID != "" {
			t.Fatal("acceptance mismatch")
		}
	} else if bs[0].BindingState != "UNBOUND" || bs[0].ProviderTaskID != "" {
		t.Fatal("unsafe acceptance persisted")
	}
	beforeRetry := posts.Load()
	if _, e := hnMiniMaxCall(context.Background(), root, id, hnMiniMaxHTTPOptions{}, hnMiniMaxSyntheticResolver); e == nil || posts.Load() != beforeRetry {
		t.Fatal("same generation was resent")
	}
	again, againBindings := hnSubmissionRead(t, root, id)
	if !reflect.DeepEqual(g, again) || !reflect.DeepEqual(bs, againBindings) {
		t.Fatal("reopen changed attempt")
	}
	if strings.Contains(string(mustHNMiniMaxJSON(t, []any{g, bs})), hnMiniMaxSyntheticKey) {
		t.Fatal("credential persisted")
	}
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	for _, kind := range []string{"results", "archive_jobs", "candidates", "sequence_items"} {
		rows, e := w.List(kind)
		if e != nil || len(rows) != 0 {
			t.Fatal("out-of-scope records created")
		}
	}
}
func mustHNMiniMaxJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestHNMiniMaxSuccessWireAndConcurrentDuplicate(t *testing.T) {
	root := t.TempDir()
	p := hnMiniMaxPrepare(t, root, nil)
	before, _ := hnSubmissionRead(t, root, p.GenerationID)
	var posts atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		if r.Method != "POST" || r.URL.Path != "/v2/video_generation" || r.Host != "api.minimax.io" || r.ProtoMajor != 1 || !r.Close || r.Header.Get("Authorization") != "Bearer "+hnMiniMaxSyntheticKey || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Idempotency-Key") != "" || r.Header.Get("X-Idempotency-Key") != "" {
			t.Error("wire policy violation")
		}
		raw, e := io.ReadAll(r.Body)
		if e != nil {
			t.Error(e)
		}
		var got any
		json.Unmarshal(raw, &got)
		var want any
		json.Unmarshal([]byte(`{"model":"MiniMax-H3","content":[{"type":"text","text":"synthetic camera move"}],"resolution":"768P","duration":6,"ratio":"16:9"}`), &want)
		if !reflect.DeepEqual(got, want) {
			t.Error("body differs from frozen mapping")
		}
		io.WriteString(w, `{"task_id":"`+hnMiniMaxFakeTask+`"}`)
	}))
	server.EnableHTTP2 = true // A capable peer must still receive HTTP/1.1.
	server.StartTLS()
	defer server.Close()
	options := hnMiniMaxLocalOptions(t, server)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, options, hnMiniMaxSyntheticResolver); e == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 || posts.Load() != 1 {
		t.Fatal("duplicate wire POST")
	}
	hnMiniMaxAssertState(t, root, p.GenerationID, "SUBMITTED", &posts, before)
	t.Log("WIRE_SUCCESS_POSTS=1 CONCURRENT_CALLERS=8 WINNERS=1 REOPEN=BOUND/SUBMITTED SAME_GENERATION_ADDITIONAL_POSTS=0 HTTP=1.1 PINNED_HOST=PASS")
}

func TestHNMiniMaxAmbiguousWire(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
	}{
		{"429", "retry later", 429}, {"500", "retry later", 500}, {"201", `{"task_id":"424010985738629"}`, 201},
		{"malformed", "not JSON", 200}, {"missing", `{}`, 200}, {"null", `null`, 200}, {"array", `[]`, 200}, {"numeric-task", `{"task_id":424010985738629}`, 200},
		{"URL-task", `{"task_id":"https://invalid.example/task"}`, 200}, {"dot-task", `{"task_id":"task.1"}`, 200}, {"colon-task", `{"task_id":"task:1"}`, 200}, {"query-task", `{"task_id":"task?key=fixture"}`, 200},
		{"secret-task", `{"task_id":"sk-synthetic-rejected"}`, 200}, {"embedded-secret", `{"task_id":"task-sk-proj-fixture"}`, 200}, {"reserved-task", `{"task_id":"CON"}`, 200}, {"oversized", strings.Repeat("x", hnMiniMaxResponseLimit+1), 200},
		{"extra-field", `{"task_id":"424010985738629","error":"fixture"}`, 200}, {"duplicate-task", `{"task_id":"other","task_id":"424010985738629"}`, 200}, {"trailing-JSON", `{"task_id":"424010985738629"}{}`, 200}, {"sentinel-response", hnMiniMaxSyntheticKey, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := hnMiniMaxPrepare(t, root, nil)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			var posts atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				io.Copy(io.Discard, r.Body)
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			var logs bytes.Buffer
			oldLog := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(oldLog)
			_, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxLocalOptions(t, server), hnMiniMaxSyntheticResolver)
			if !errors.Is(e, ErrHNSubmissionUnknown) || posts.Load() != 1 {
				t.Fatal("ambiguity policy failure")
			}
			if strings.Contains(e.Error()+logs.String(), hnMiniMaxSyntheticKey) {
				t.Fatal("credential in error/log")
			}
			hnMiniMaxAssertState(t, root, p.GenerationID, "SUBMISSION_UNKNOWN", &posts, before)
			t.Log("OBSERVED_POSTS=1 UNKNOWN=PASS TASK_ID_NOT_PERSISTED=PASS SAME_GENERATION_ADDITIONAL_POSTS=0 SENTINEL_LEAK=NONE")
		})
	}
}

func TestHNMiniMaxRedirectWire(t *testing.T) {
	for _, status := range []int{307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			root := t.TempDir()
			p := hnMiniMaxPrepare(t, root, nil)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			var first, second atomic.Int32
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				second.Add(1)
				io.WriteString(w, `{"task_id":"424010985738629"}`)
			}))
			defer target.Close()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first.Add(1)
				io.Copy(io.Discard, r.Body)
				w.Header().Set("Location", target.URL+"/v2/video_generation")
				w.WriteHeader(status)
			}))
			defer server.Close()
			_, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxLocalOptions(t, server), hnMiniMaxSyntheticResolver)
			if !errors.Is(e, ErrHNSubmissionUnknown) || first.Load() != 1 || second.Load() != 0 {
				t.Fatal("redirect replay detected")
			}
			hnMiniMaxAssertState(t, root, p.GenerationID, "SUBMISSION_UNKNOWN", &first, before)
			t.Logf("HTTP=%d FIRST_SERVER_POSTS=1 SECOND_SERVER_REQUESTS=0 UNKNOWN=PASS RESEND=NONE", status)
		})
	}
}

func TestHNMiniMaxDropTimeoutAndCancellationWire(t *testing.T) {
	for _, mode := range []string{"drop", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			p := hnMiniMaxPrepare(t, root, nil)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			var posts atomic.Int32
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				io.Copy(io.Discard, r.Body)
				if mode == "drop" {
					c, _, e := w.(http.Hijacker).Hijack()
					if e == nil {
						c.Close()
					}
					return
				}
				if mode == "cancel" {
					cancel()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			options := hnMiniMaxLocalOptions(t, server)
			options.timeout = 500 * time.Millisecond
			_, e := hnMiniMaxCall(ctx, root, p.GenerationID, options, hnMiniMaxSyntheticResolver)
			if !errors.Is(e, ErrHNSubmissionUnknown) || posts.Load() != 1 {
				t.Fatal("network ambiguity replay")
			}
			hnMiniMaxAssertState(t, root, p.GenerationID, "SUBMISSION_UNKNOWN", &posts, before)
			t.Log("BODY_RECEIVED_POSTS=1 DROP_OR_ACCEPTANCE_TIMEOUT_OR_CANCEL=UNKNOWN RETRY=NONE REOPEN=PASS")
		})
	}
}

func TestHNMiniMaxPreflightReject(t *testing.T) {
	cases := []struct {
		name string
		edit func(*HNGenerationPrepareInput)
	}{
		{"protocol", func(i *HNGenerationPrepareInput) { i.Protocol = "minimax" }}, {"provider", func(i *HNGenerationPrepareInput) { i.ProviderIdentity = "metaso" }}, {"model", func(i *HNGenerationPrepareInput) { i.Model = "MiniMax-H3-Max" }}, {"connection", func(i *HNGenerationPrepareInput) { i.ConnectionID = "" }},
	}
	for _, item := range []struct{ key, value string }{{"videoSeconds", "3"}, {"videoSeconds", "16"}, {"videoSeconds", "4.5"}, {"videoSeconds", "04"}, {"vquality", "unknown"}, {"size", "adaptive"}, {"size", "123x456"}, {"videoMode", "pro"}, {"videoNegativePrompt", "bad"}, {"videoMultiShot", "true"}, {"videoShotType", "custom"}, {"videoGenerateAudio", "false"}, {"videoWatermark", "true"}, {"videoCharacterOrientation", "image"}} {
		cases = append(cases, struct {
			name string
			edit func(*HNGenerationPrepareInput)
		}{item.key + "-" + item.value, func(i *HNGenerationPrepareInput) {
			var p map[string]any
			json.Unmarshal(i.Parameters, &p)
			p[item.key] = item.value
			i.Parameters, _ = json.Marshal(p)
		}})
	}
	cases = append(cases, struct {
		name string
		edit func(*HNGenerationPrepareInput)
	}{"multi-prompt", func(i *HNGenerationPrepareInput) {
		i.Parameters = json.RawMessage(`{"videoSeconds":"6","vquality":"720","size":"16:9","videoMultiPrompt":[{"prompt":"shot","duration":"3"}]}`)
	}})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := hnMiniMaxPrepare(t, root, tc.edit)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			var posts atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts.Add(1) }))
			defer server.Close()
			_, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxLocalOptions(t, server), hnMiniMaxSyntheticResolver)
			g, bs := hnSubmissionRead(t, root, p.GenerationID)
			if !errors.Is(e, ErrHNMiniMaxPreflight) || posts.Load() != 0 || len(bs) != 0 || !reflect.DeepEqual(g, before) {
				t.Fatal("invalid request consumed attempt")
			}
			t.Log("PREFLIGHT_REJECT=PASS SERVER_POSTS=0 TASKBINDINGS=0 PREPARED_BYTES_UNCHANGED=YES")
		})
	}
	t.Run("reference", func(t *testing.T) {
		root := t.TempDir()
		r, e := SnapshotLocalReference(root, "test", "image", "image/png", strings.NewReader("fixture"), 7)
		if e != nil {
			t.Fatal(e)
		}
		p := hnMiniMaxPrepare(t, root, func(i *HNGenerationPrepareInput) {
			i.ReferenceBindings = []foundation.ReferenceBinding{{ReferenceVersionID: r.ID, SHA256: r.SHA256, Role: "firstFrame"}}
		})
		_, e = hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxHTTPOptions{}, hnMiniMaxSyntheticResolver)
		g, bs := hnSubmissionRead(t, root, p.GenerationID)
		if !errors.Is(e, ErrHNMiniMaxPreflight) || len(bs) != 0 || g.SubmissionState != "PREPARED" {
			t.Fatal("reference entered submission")
		}
	})
}

func TestHNMiniMaxStrictChannelAndOwner(t *testing.T) {
	for _, base := range []string{"https://api.minimax.io", "https://api.minimax.io/", "https://api.minimax.io:443/", " HTTPS://API.MINIMAX.IO "} {
		c := hnMiniMaxSyntheticChannel()
		c.BaseURL = base
		if !validHNMiniMaxChannel(c, c.ID) {
			t.Fatal("valid root rejected")
		}
	}
	for _, base := range []string{"https://metaso.cn/api/minimax", "https://api.minimax.cn", "http://api.minimax.io", "https://api.minimax.io.evil.invalid", "https://evilapi.minimax.io", "https://api.minimax.io:444", "https://user@api.minimax.io", "https://@api.minimax.io", "https://api.minimax.io/v2", "https://api.minimax.io?", "https://api.minimax.io/#", "https://api.minimax.io/../", "https://api.minimax.io\\"} {
		c := hnMiniMaxSyntheticChannel()
		c.BaseURL = base
		if validHNMiniMaxChannel(c, c.ID) {
			t.Fatal("nonofficial root accepted")
		}
	}
	edits := []func(*model.ModelChannel){func(c *model.ModelChannel) { c.BaseURL = "https://metaso.cn/api/minimax" }, func(c *model.ModelChannel) { c.ID = "other" }, func(c *model.ModelChannel) { c.Protocol = "minimax" }, func(c *model.ModelChannel) { c.Models = nil }, func(c *model.ModelChannel) { c.APIKey = "" }, func(c *model.ModelChannel) { c.Enabled = false }}
	for index, edit := range edits {
		t.Run(string(rune('A'+index)), func(t *testing.T) {
			root := t.TempDir()
			p := hnMiniMaxPrepare(t, root, nil)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			var posts atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts.Add(1) }))
			defer server.Close()
			resolver := func(owner, id string) (model.ModelChannel, error) {
				c := hnMiniMaxSyntheticChannel()
				edit(&c)
				return c, nil
			}
			_, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxLocalOptions(t, server), resolver)
			g, bs := hnSubmissionRead(t, root, p.GenerationID)
			if !errors.Is(e, ErrHNMiniMaxPreflight) || posts.Load() != 0 || len(bs) != 0 || !reflect.DeepEqual(before, g) {
				t.Fatal("channel preflight failure")
			}
		})
	}
	for _, resolver := range []HNMiniMaxChannelResolver{func(string, string) (model.ModelChannel, error) {
		return model.ModelChannel{}, errors.New(hnMiniMaxSyntheticKey)
	}, func(string, string) (model.ModelChannel, error) { panic(hnMiniMaxSyntheticKey) }} {
		root := t.TempDir()
		p := hnMiniMaxPrepare(t, root, nil)
		_, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxHTTPOptions{}, resolver)
		g, bs := hnSubmissionRead(t, root, p.GenerationID)
		if !errors.Is(e, ErrHNMiniMaxPreflight) || strings.Contains(e.Error(), hnMiniMaxSyntheticKey) || len(bs) != 0 || g.SubmissionState != "PREPARED" {
			t.Fatal("resolver secret/preflight failure")
		}
	}
	root := t.TempDir()
	p := hnMiniMaxPrepare(t, root, nil)
	_, e := submitHNMiniMax(context.Background(), root, "test", p.GenerationID, "wrong-owner", hnMiniMaxSyntheticResolver, hnMiniMaxHTTPOptions{dialContext: func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("offline fixture") }})
	g, bs := hnSubmissionRead(t, root, p.GenerationID)
	if !errors.Is(e, ErrHNMiniMaxPreflight) || len(bs) != 0 || g.SubmissionState != "PREPARED" {
		t.Fatal("owner mismatch accepted")
	}
	t.Log("EXACT_OWNER_CONNECTION_MODEL_PROTOCOL_ROOT=PASS REAL_RESOLVER_INVOKED=NO KEY_STORE=NONE")
}

func TestHNMiniMaxOwnedRevalidationAndPersistenceFailure(t *testing.T) {
	for _, mode := range []string{"channel-race", "resolver-panic", "acceptance-persistence", "acceptance-and-unknown-persistence"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			p := hnMiniMaxPrepare(t, root, nil)
			before, _ := hnSubmissionRead(t, root, p.GenerationID)
			var posts, resolutions atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				io.Copy(io.Discard, r.Body)
				io.WriteString(w, `{"task_id":"424010985738629"}`)
			}))
			defer server.Close()
			if strings.Contains(mode, "persistence") {
				db, e := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
				if e != nil {
					t.Fatal(e)
				}
				_, e = db.Exec("CREATE TRIGGER fail_binding BEFORE UPDATE ON task_bindings BEGIN SELECT RAISE(ABORT,'synthetic accepted persistence failure'); END")
				if e != nil {
					t.Fatal(e)
				}
				if mode == "acceptance-and-unknown-persistence" {
					_, e = db.Exec("CREATE TRIGGER fail_unknown BEFORE UPDATE ON generations WHEN json_extract(NEW.data,'$.submissionState')='SUBMISSION_UNKNOWN' BEGIN SELECT RAISE(ABORT,'synthetic unknown persistence failure'); END")
					if e != nil {
						t.Fatal(e)
					}
				}
				db.Close()
			}
			resolver := func(owner, id string) (model.ModelChannel, error) {
				c := hnMiniMaxSyntheticChannel()
				if resolutions.Add(1) > 1 {
					if mode == "channel-race" {
						c.BaseURL = "https://metaso.cn/api/minimax"
					}
					if mode == "resolver-panic" {
						panic(hnMiniMaxSyntheticKey)
					}
				}
				return c, nil
			}
			_, e := hnMiniMaxCall(context.Background(), root, p.GenerationID, hnMiniMaxLocalOptions(t, server), resolver)
			wantPosts := int32(0)
			if strings.Contains(mode, "persistence") {
				wantPosts = 1
			}
			if !errors.Is(e, ErrHNSubmissionUnknown) || posts.Load() != wantPosts || strings.Contains(e.Error(), hnMiniMaxSyntheticKey) {
				t.Fatal("owned error replay/leak")
			}
			if mode == "acceptance-and-unknown-persistence" {
				db, e := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
				if e != nil {
					t.Fatal(e)
				}
				_, e = db.Exec("DROP TRIGGER fail_unknown")
				db.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			hnMiniMaxAssertState(t, root, p.GenerationID, "SUBMISSION_UNKNOWN", &posts, before)
			t.Logf("MODE=%s OBSERVED_POSTS=%d REOPEN=UNKNOWN ACCEPTANCE_NOT_PERSISTED=YES RESEND=NONE", mode, wantPosts)
		})
	}
}

func TestHNMiniMaxNewBaselinePreservesHistoricalGeneration(t *testing.T) {
	root := t.TempDir()
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	old, e := w.CreateGeneration(foundation.Generation{NodeID: "old", PromptSnapshot: "historical fixture", Protocol: "metaso", ProviderIdentity: "metaso", Model: "MiniMax-H3", Parameters: json.RawMessage(`{}`), SourceBaseline: "ff32dc249811130a3db69be456e295be100b6e9f"})
	if e != nil {
		t.Fatal(e)
	}
	old, e = w.FreezeGeneration(old.ID)
	if e != nil {
		t.Fatal(e)
	}
	before, e := w.Read("generations", old.ID)
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	p := hnMiniMaxPrepare(t, root, nil)
	g, _ := hnSubmissionRead(t, root, p.GenerationID)
	if g.SourceBaseline != "16047f46e2186373ea824e12e84ae8dfa2ccde32" || g.ProviderIdentity != HNMiniMaxOfficialIdentity || g.Protocol != "metaso" {
		t.Fatal("reviewed prepare identity/baseline absent")
	}
	w, e = foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	after, e := w.Read("generations", old.ID)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("historical Generation rewritten")
	}
	t.Log("NEW_SOURCE_BASELINE=16047f46e2186373ea824e12e84ae8dfa2ccde32 HISTORICAL_BASELINE_AND_PROVIDER_AND_RECORD_BYTES=UNCHANGED")
}

func TestHNMiniMaxRequestMapping(t *testing.T) {
	root := t.TempDir()
	p := hnMiniMaxPrepare(t, root, nil)
	g, _ := hnSubmissionRead(t, root, p.GenerationID)
	g.PromptSnapshot = "  exact frozen prompt\n"
	for _, quality := range []struct{ in, out string }{{"480", "768P"}, {"720p", "768P"}, {"768P", "768P"}, {"1080", "2K"}, {"2K", "2K"}, {"4k", "2K"}} {
		for _, ratio := range []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"} {
			for seconds := 4; seconds <= 15; seconds++ {
				g.Parameters = mustHNMiniMaxJSON(t, map[string]string{"vquality": quality.in, "size": ratio, "videoSeconds": stringDurationTest(seconds)})
				raw, e := hnMiniMaxRequestBody(g)
				var request hnMiniMaxRequest
				json.Unmarshal(raw, &request)
				if e != nil || request.Duration != seconds || request.Resolution != quality.out || request.Ratio != ratio || request.Content[0].Text != g.PromptSnapshot {
					t.Fatal("finite mapping failed")
				}
			}
		}
	}
	t.Log("FINITE_DURATION_RESOLUTION_RATIO_MAPPING=PASS PROMPT_EXACT=PASS DEFAULT_ONLY_UNSUPPORTED_CONTROLS=PASS")
}

func TestHNMiniMaxSyntheticSmoke(t *testing.T) {
	root := t.TempDir()
	var posts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := posts.Add(1)
		io.Copy(io.Discard, r.Body)
		if n == 1 {
			io.WriteString(w, `{"task_id":"424010985738629"}`)
		} else {
			w.WriteHeader(500)
			io.WriteString(w, "synthetic retry later")
		}
	}))
	defer server.Close()
	options := hnMiniMaxLocalOptions(t, server)
	success := hnMiniMaxPrepare(t, root, nil)
	unknown := hnMiniMaxPrepare(t, root, nil)
	if success.GenerationID == unknown.GenerationID || success.ShotID != unknown.ShotID {
		t.Fatal("attempt/Shot identity")
	}
	first, _ := hnSubmissionRead(t, root, success.GenerationID)
	second, _ := hnSubmissionRead(t, root, unknown.GenerationID)
	if _, e := hnMiniMaxCall(context.Background(), root, success.GenerationID, options, hnMiniMaxSyntheticResolver); e != nil {
		t.Fatal(e)
	}
	if _, e := hnMiniMaxCall(context.Background(), root, unknown.GenerationID, options, hnMiniMaxSyntheticResolver); !errors.Is(e, ErrHNSubmissionUnknown) {
		t.Fatal(e)
	}
	hnMiniMaxAssertState(t, root, success.GenerationID, "SUBMITTED", &posts, first)
	hnMiniMaxAssertState(t, root, unknown.GenerationID, "SUBMISSION_UNKNOWN", &posts, second)
	if posts.Load() != 2 {
		t.Fatal("smoke wire count")
	}
	a, bs := hnSubmissionRead(t, root, success.GenerationID)
	b, bu := hnSubmissionRead(t, root, unknown.GenerationID)
	if path := os.Getenv("HN_R12_SMOKE_EVIDENCE_PATH"); path != "" {
		evidence := map[string]any{"status": "PASS", "network": "loopback TLS fake only; request host pinned api.minimax.io", "generations": []foundation.Generation{a, b}, "bindings": []foundation.TaskBinding{bs[0], bu[0]}, "observedPostsTotal": posts.Load(), "perGenerationObservedPosts": []int{1, 1}, "sameGenerationAdditionalPosts": 0, "results": 0, "archiveJobs": 0, "candidates": 0, "realProviderCalls": 0, "realCredentialRead": "NONE"}
		data, e := json.MarshalIndent(evidence, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log("SYNTHETIC_SMOKE=PASS PROJECTS=1 SHOTS=1 GENERATIONS=2 PER_GENERATION_POSTS=1,1 REOPEN=BOUND/SUBMITTED,UNBOUND/UNKNOWN RESULTS=0 ARCHIVES=0 CANDIDATES=0")
}
func stringDurationTest(n int) string { b, _ := json.Marshal(n); return string(b) }

// A real process exits after local HTTP acceptance, before R10 can persist it.
func TestHNMiniMaxCrashChild(t *testing.T) {
	if os.Getenv("HN_R12_CRASH_CHILD") != "1" {
		return
	}
	address := os.Getenv("HN_R12_FAKE_ADDRESS")
	host, _, e := net.SplitHostPort(address)
	if e != nil || host != "127.0.0.1" {
		t.Fatal("not loopback")
	}
	der, e := base64.StdEncoding.DecodeString(os.Getenv("HN_R12_FAKE_CERT"))
	if e != nil {
		t.Fatal(e)
	}
	cert, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	root, id := os.Getenv("HN_R12_CRASH_ROOT"), os.Getenv("HN_R12_CRASH_GENERATION")
	before, _ := hnSubmissionRead(t, root, id)
	options := hnMiniMaxHTTPOptions{timeout: 2 * time.Second, tlsConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}, dialContext: func(ctx context.Context, network, dest string) (net.Conn, error) {
		if dest != "api.minimax.io:443" {
			return nil, errors.New("un-pinned host")
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	transport := hnMiniMaxCrashTransport{hnMiniMaxTransport{"test-owner", hnMiniMaxSyntheticResolver, before.FrozenHash, options}}
	if _, e := SubmitFrozenGeneration(context.Background(), root, "test", id, transport); e != nil {
		t.Fatal(e)
	}
	t.Fatal("child did not crash")
}

type hnMiniMaxCrashTransport struct{ hnMiniMaxTransport }

func (t hnMiniMaxCrashTransport) Submit(ctx context.Context, g foundation.Generation) (SubmissionAcceptance, error) {
	a, e := t.hnMiniMaxTransport.Submit(ctx, g)
	if e == nil {
		os.Exit(0)
	}
	return a, e
}
func TestHNMiniMaxCrashAfterWireAcceptance(t *testing.T) {
	root := t.TempDir()
	p := hnMiniMaxPrepare(t, root, nil)
	before, _ := hnSubmissionRead(t, root, p.GenerationID)
	var posts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		io.Copy(io.Discard, r.Body)
		io.WriteString(w, `{"task_id":"424010985738629"}`)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHNMiniMaxCrashChild$")
	cmd.Env = append(os.Environ(), "HN_R12_CRASH_CHILD=1", "HN_R12_CRASH_ROOT="+root, "HN_R12_CRASH_GENERATION="+p.GenerationID, "HN_R12_FAKE_ADDRESS="+u.Host, "HN_R12_FAKE_CERT="+base64.StdEncoding.EncodeToString(server.Certificate().Raw))
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("child failed: %v %s", e, b)
	}
	if posts.Load() != 1 {
		t.Fatal("child wire count")
	}
	hnMiniMaxAssertState(t, root, p.GenerationID, "SUBMISSION_UNKNOWN", &posts, before)
	t.Log("CRASH_AFTER_LOCAL_WIRE_ACCEPTANCE=PASS OBSERVED_POSTS=1 REOPEN=UNKNOWN SAME_GENERATION_ADDITIONAL_POSTS=0")
}
