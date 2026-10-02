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

const hnPollMessageSentinel = "HN_R13_SYNTHETIC_ERROR_SENTINEL"
const hnPollURLSentinel = "HN_R13_SYNTHETIC_LOCATION_SENTINEL"

func hnPollFixture(t *testing.T, root string) (foundation.Generation, foundation.TaskBinding) {
	t.Helper()
	p := hnMiniMaxPrepare(t, root, nil)
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal("fixture Open failed")
	}
	defer w.Close()
	_, owner, err := w.BeginSubmission(p.GenerationID)
	if err != nil {
		t.Fatal("fixture Begin failed")
	}
	b, err := w.RecordSubmissionAccepted(p.GenerationID, owner.ID, "", hnMiniMaxFakeTask)
	if err != nil {
		t.Fatal("fixture acceptance failed")
	}
	var g foundation.Generation
	if hnReadRecord(w, "generations", p.GenerationID, &g) != nil {
		t.Fatal("fixture Generation read failed")
	}
	return g, b
}

func hnPollResultLocation() string {
	u := url.URL{Scheme: "https", Host: "r13-fixture.invalid", Path: "/result", RawQuery: "signature=" + hnPollURLSentinel}
	return u.String()
}
func hnPollBody(state string) map[string]any {
	task := map[string]any{"id": hnMiniMaxFakeTask, "model": "MiniMax-H3", "status": state, "task_type": "generation", "modality": "video", "resolution": "768P", "duration": 6, "ratio": "16:9"}
	if state == "succeeded" {
		task["content"] = map[string]any{"url": hnPollResultLocation()}
	}
	if state == "failed" || state == "cancelled" {
		task["error"] = map[string]any{"message": hnPollMessageSentinel, "code": "synthetic"}
	}
	return map[string]any{"task": task}
}
func hnPollTLS(t *testing.T, calls, posts *atomic.Int32, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == "POST" {
			posts.Add(1)
		}
		if r.Method != "GET" || r.Host != "api.minimax.io" || r.URL.Path != "/v2/query/video_generation/"+hnMiniMaxFakeTask || r.URL.RawQuery != "" || r.ProtoMajor != 1 || !r.Close || r.Header.Get("Authorization") != "Bearer "+hnMiniMaxSyntheticKey || r.ContentLength > 0 || r.Header.Get("Cookie") != "" {
			t.Error("poll wire policy violated")
		}
		handler(w, r)
	}))
	s.EnableHTTP2 = true
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}
func hnPollCall(ctx context.Context, root, binding string, options hnMiniMaxHTTPOptions, resolver HNMiniMaxChannelResolver) (HNProviderTaskObservation, error) {
	if options.dialContext == nil {
		options.dialContext = func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("offline fixture denies unexpected network")
		}
	}
	return pollHNMiniMax(ctx, root, "test", binding, "test-owner", resolver, options)
}
func hnPollRawRecords(t *testing.T, root string, g, b string) (foundation.Generation, foundation.TaskBinding) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
	if err != nil {
		t.Fatal("fixture record connection failed")
	}
	defer db.Close()
	var rawG, rawB string
	if db.QueryRow("SELECT data FROM generations WHERE id=?", g).Scan(&rawG) != nil || db.QueryRow("SELECT data FROM task_bindings WHERE id=?", b).Scan(&rawB) != nil {
		t.Fatal("fixture record read failed")
	}
	var generation foundation.Generation
	var binding foundation.TaskBinding
	if json.Unmarshal([]byte(rawG), &generation) != nil || json.Unmarshal([]byte(rawB), &binding) != nil {
		t.Fatal("fixture decode failed")
	}
	return generation, binding
}
func hnPollPersistFixture(t *testing.T, root string, g foundation.Generation, b foundation.TaskBinding) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
	if err != nil {
		t.Fatal("fixture mutation connection failed")
	}
	defer db.Close()
	for _, row := range []struct {
		table, id string
		value     any
	}{{"generations", g.ID, g}, {"task_bindings", b.ID, b}} {
		raw, _ := json.Marshal(row.value)
		if _, err = db.Exec("UPDATE "+row.table+" SET data=? WHERE id=?", string(raw), row.id); err != nil {
			t.Fatal("fixture mutation failed")
		}
	}
}
func hnPollAssertPrivate(t *testing.T, root string, public any, err error) {
	t.Helper()
	raw, _ := json.Marshal(public)
	if err != nil {
		raw = append(raw, err.Error()...)
	}
	secrets := []string{hnMiniMaxSyntheticKey, hnPollMessageSentinel, hnPollURLSentinel, hnPollResultLocation()}
	for _, secret := range secrets {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("public poll leak")
		}
	}
	if filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			for _, secret := range secrets {
				if bytes.Contains(data, []byte(secret)) {
					return errors.New("persisted poll leak")
				}
			}
		}
		return nil
	}) != nil {
		t.Fatal("private workspace scan failed")
	}
}

func TestHNMiniMaxPollStatesAndRecoverySmoke(t *testing.T) {
	root := t.TempDir()
	g, b := hnPollFixture(t, root)
	var gets, posts atomic.Int32
	states := []string{"queued", "running", "succeeded", "succeeded"}
	s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) {
		i := int(gets.Load()) - 1
		if i >= len(states) {
			t.Error("unexpected extra request")
			return
		}
		json.NewEncoder(w).Encode(hnPollBody(states[i]))
	})
	options := hnMiniMaxLocalOptions(t, s)
	var captured bytes.Buffer
	previousLog := log.Writer()
	log.SetOutput(&captured)
	defer log.SetOutput(previousLog)
	observations := []HNProviderTaskObservation{}
	last := ""
	for i, state := range states {
		if i == 3 {
			w, err := foundation.Open(root, "test")
			if err != nil {
				t.Fatal("reopen failed")
			}
			w.Close()
		}
		obs, err := hnPollCall(context.Background(), root, b.ID, options, hnMiniMaxSyntheticResolver)
		if err != nil || obs.State != state || obs.Terminal != (state == "succeeded") || obs.ResultLocationAvailable != (state == "succeeded") || obs.TaskBindingID != b.ID || obs.ProviderTaskID != b.ProviderTaskID || obs.SafeFailureClass != "" || obs.PolledAt == "" {
			t.Fatal("state observation failed")
		}
		when, err := time.Parse(time.RFC3339Nano, obs.PolledAt)
		if err != nil || when.Location() != time.UTC || last != "" && !when.After(mustHNPollTime(t, last)) {
			t.Fatal("poll timestamp did not advance")
		}
		last = obs.PolledAt
		afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
		if !reflect.DeepEqual(g, afterG) || afterB.LastPolledAt != obs.PolledAt || afterB.ErrorClass != "" || afterB.BindingState != "BOUND" || afterB.ProviderTaskID != b.ProviderTaskID {
			t.Fatal("poll changed submission or ownership")
		}
		compare := afterB
		compare.LastPolledAt, compare.UpdatedAt = b.LastPolledAt, b.UpdatedAt
		if !reflect.DeepEqual(compare, b) {
			t.Fatal("unplanned binding mutation")
		}
		hnPollAssertPrivate(t, root, obs, err)
		observations = append(observations, obs)
	}
	if gets.Load() != 4 || posts.Load() != 0 || captured.Len() != 0 {
		t.Fatal("wire count or logs unsafe")
	}
	// A separate known accepted task receives only static terminal failure facts.
	failedG, failedB := hnPollFixture(t, root)
	var failGets, failPosts atomic.Int32
	failed := hnPollTLS(t, &failGets, &failPosts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hnPollBody("failed")) })
	obs, err := hnPollCall(context.Background(), root, failedB.ID, hnMiniMaxLocalOptions(t, failed), hnMiniMaxSyntheticResolver)
	if err != nil || obs.SafeFailureClass != "MINIMAX_TASK_FAILED" || failGets.Load() != 1 || failPosts.Load() != 0 {
		t.Fatal("failure smoke failed")
	}
	storedG, storedB := hnPollRawRecords(t, root, failedG.ID, failedB.ID)
	if !reflect.DeepEqual(failedG, storedG) || storedB.ErrorClass != "MINIMAX_TASK_FAILED" || storedB.LastPolledAt != obs.PolledAt {
		t.Fatal("failure persistence failed")
	}
	hnPollAssertPrivate(t, root, obs, err)
	if captured.Len() != 0 {
		t.Fatal("provider failure logged private data")
	}
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal("final reopen failed")
	}
	defer w.Close()
	for _, kind := range []string{"results", "archive_jobs", "candidates", "sequence_items"} {
		rows, e := w.List(kind)
		if e != nil || len(rows) != 0 {
			t.Fatal("poll created an out-of-scope record")
		}
	}
	if path := os.Getenv("HN_R13_SMOKE_EVIDENCE_PATH"); path != "" {
		evidence := map[string]any{"status": "PASS", "observations": observations, "explicitSequenceGETs": 3, "reopenExplicitGETs": 1, "secondTaskGETs": 1, "submitPosts": 0, "results": 0, "archiveJobs": 0, "rawResultURLPersisted": false, "generationUnchanged": true, "bindingState": "BOUND", "staticFailureClass": storedB.ErrorClass, "realProviderCalls": 0, "realCredentialRead": "NONE"}
		raw, _ := json.MarshalIndent(evidence, "", "  ")
		if os.WriteFile(path, raw, 0600) != nil {
			t.Fatal("safe evidence write failed")
		}
	}
	t.Log("POLL_SMOKE=PASS SEQUENCE_GETS=3 REOPEN_GETS=1 SECOND_TASK_GETS=1 SUBMIT_POSTS=0 GENERATION_BYTES=UNCHANGED RESULT_URL_PUBLIC_PERSISTED_LOGGED=NONE RESULTS=0 ARCHIVES=0")
}
func mustHNPollTime(t *testing.T, text string) time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		t.Fatal("invalid fixture timestamp")
	}
	return when
}

func TestHNMiniMaxPollTerminalClassesAndConflicts(t *testing.T) {
	for _, first := range []string{"failed", "cancelled"} {
		for _, next := range []string{"queued", "running", "succeeded", "failed", "cancelled"} {
			t.Run(first+"-then-"+next, func(t *testing.T) {
				root := t.TempDir()
				g, b := hnPollFixture(t, root)
				var gets, posts atomic.Int32
				s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) {
					state := first
					if gets.Load() == 2 {
						state = next
					}
					json.NewEncoder(w).Encode(hnPollBody(state))
				})
				options := hnMiniMaxLocalOptions(t, s)
				obs, err := hnPollCall(context.Background(), root, b.ID, options, hnMiniMaxSyntheticResolver)
				if err != nil || !obs.Terminal || obs.SafeFailureClass == "" || obs.ResultLocationAvailable {
					t.Fatal("terminal class failed")
				}
				beforeG, beforeB := hnPollRawRecords(t, root, g.ID, b.ID)
				obs, err = hnPollCall(context.Background(), root, b.ID, options, hnMiniMaxSyntheticResolver)
				afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
				if !reflect.DeepEqual(g, afterG) || gets.Load() != 2 || posts.Load() != 0 {
					t.Fatal("terminal poll changed submission")
				}
				if first != next {
					if !errors.Is(err, ErrHNMiniMaxPollStateConflict) || !reflect.DeepEqual(beforeG, afterG) || !reflect.DeepEqual(beforeB, afterB) || obs != (HNProviderTaskObservation{}) {
						t.Fatal("terminal conflict mutated facts")
					}
				} else if err != nil || afterB.ErrorClass != beforeB.ErrorClass || !mustHNPollTime(t, afterB.LastPolledAt).After(mustHNPollTime(t, beforeB.LastPolledAt)) {
					t.Fatal("same terminal observation rejected")
				}
				hnPollAssertPrivate(t, root, obs, err)
			})
		}
	}
	t.Log("TERMINAL_FAILED_CANCELLED=SAFE_STATIC STALE_DOWNGRADE=REJECTED CONFLICT_PERSISTENCE_MUTATION=NONE")
}

func TestHNMiniMaxPollPreflight(t *testing.T) {
	cases := map[string]func(*foundation.Generation, *foundation.TaskBinding){
		"prepared": func(g *foundation.Generation, b *foundation.TaskBinding) {
			g.Status, g.SubmissionState = "PREPARED", "PREPARED"
		},
		"submitting": func(g *foundation.Generation, b *foundation.TaskBinding) {
			g.Status, g.SubmissionState = "SUBMITTING", "SUBMITTING"
		},
		"unknown": func(g *foundation.Generation, b *foundation.TaskBinding) {
			g.Status, g.SubmissionState = "SUBMISSION_UNKNOWN", "SUBMISSION_UNKNOWN"
		},
		"unfrozen":        func(g *foundation.Generation, b *foundation.TaskBinding) { g.Frozen = false },
		"unbound":         func(g *foundation.Generation, b *foundation.TaskBinding) { b.BindingState = "UNBOUND" },
		"missing-id":      func(g *foundation.Generation, b *foundation.TaskBinding) { b.ProviderTaskID = "" },
		"unsafe-id":       func(g *foundation.Generation, b *foundation.TaskBinding) { b.ProviderTaskID = "../another" },
		"credential-id":   func(g *foundation.Generation, b *foundation.TaskBinding) { b.ProviderTaskID = "sk-unsafe" },
		"binding-owner":   func(g *foundation.Generation, b *foundation.TaskBinding) { b.GenerationID = "missing" },
		"binding-project": func(g *foundation.Generation, b *foundation.TaskBinding) { b.ProjectID = "another" },
		"connection":      func(g *foundation.Generation, b *foundation.TaskBinding) { b.ConnectionID = "another" },
		"protocol":        func(g *foundation.Generation, b *foundation.TaskBinding) { g.Protocol = "ark" },
		"identity":        func(g *foundation.Generation, b *foundation.TaskBinding) { g.ProviderIdentity = "metaso" },
		"model":           func(g *foundation.Generation, b *foundation.TaskBinding) { g.Model = "MiniMax-H3-Max" },
		"frozen-hash":     func(g *foundation.Generation, b *foundation.TaskBinding) { g.FrozenHash = strings.Repeat("0", 64) },
		"frozen-prompt":   func(g *foundation.Generation, b *foundation.TaskBinding) { g.PromptSnapshot = "changed" },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			edit(&g, &b)
			hnPollPersistFixture(t, root, g, b)
			var gets, posts atomic.Int32
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hnPollBody("running")) })
			obs, err := hnPollCall(context.Background(), root, b.ID, hnMiniMaxLocalOptions(t, s), hnMiniMaxSyntheticResolver)
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, ErrHNMiniMaxPollPreflight) || gets.Load() != 0 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) || obs != (HNProviderTaskObservation{}) {
				t.Fatal("preflight wrote or queried invalid attempt")
			}
		})
	}
	t.Log("INVALID_STATE_OWNERSHIP_HASH_ID=GET_0 GENERATION_BINDING_BYTES=UNCHANGED SUBMISSION_UNKNOWN_NOT_POLLED=PASS SUBMITTING_NOT_RECONCILED_BY_POLL=PASS")
}

func TestHNMiniMaxPollChannelPreflight(t *testing.T) {
	for _, name := range []string{"owner", "connection", "disabled", "gateway", "protocol", "model", "key", "error", "panic"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			var gets, posts atomic.Int32
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hnPollBody("running")) })
			resolver := func(owner, connection string) (model.ModelChannel, error) {
				if owner != "test-owner" || connection != b.ConnectionID {
					t.Error("wrong resolver owner/connection")
				}
				c := hnMiniMaxSyntheticChannel()
				switch name {
				case "owner", "error":
					return c, errors.New(hnPollMessageSentinel)
				case "connection":
					c.ID = "another"
				case "disabled":
					c.Enabled = false
				case "gateway":
					c.BaseURL = "https://gateway.invalid"
				case "protocol":
					c.Protocol = "ark"
				case "model":
					c.Models = []string{"other"}
				case "key":
					c.APIKey = "bad\r\nheader"
				case "panic":
					panic(hnPollMessageSentinel)
				}
				return c, nil
			}
			obs, err := hnPollCall(context.Background(), root, b.ID, hnMiniMaxLocalOptions(t, s), resolver)
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, ErrHNMiniMaxPollPreflight) || gets.Load() != 0 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) {
				t.Fatal("invalid channel queried or mutated")
			}
			hnPollAssertPrivate(t, root, obs, err)
		})
	}
}

func TestHNMiniMaxPollHTTPFailures(t *testing.T) {
	for _, code := range []int{201, 301, 302, 307, 308, 400, 401, 403, 404, 429, 500, 502, 503, 504} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			var targets atomic.Int32
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targets.Add(1) }))
			defer target.Close()
			var gets, posts atomic.Int32
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(code)
				io.WriteString(w, hnPollMessageSentinel)
			})
			obs, err := hnPollCall(context.Background(), root, b.ID, hnMiniMaxLocalOptions(t, s), hnMiniMaxSyntheticResolver)
			want := ErrHNMiniMaxPollProtocol
			if code == 401 || code == 403 {
				want = ErrHNMiniMaxPollAuth
			}
			if code == 404 {
				want = ErrHNMiniMaxPollNotFound
			}
			if code == 429 || code >= 500 {
				want = ErrHNMiniMaxPollTransient
			}
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, want) || gets.Load() != 1 || posts.Load() != 0 || targets.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) {
				t.Fatal("HTTP failure changed accepted task or retried")
			}
			hnPollAssertPrivate(t, root, obs, err)
			t.Logf("HTTP_STATUS=%d OBSERVED_GETS=1 SUBMIT_POSTS=0 REDIRECT_TARGET_REQUESTS=0 SUBMITTED_BOUND_LASTPOLLED_UNCHANGED=PASS", code)
		})
	}
}

func TestHNMiniMaxPollResponseValidation(t *testing.T) {
	cases := map[string]func(map[string]any){
		"id": func(task map[string]any) { task["id"] = "another" }, "model": func(task map[string]any) { task["model"] = "MiniMax-H3-Max" }, "task-type": func(task map[string]any) { task["task_type"] = "regeneration" }, "modality": func(task map[string]any) { task["modality"] = "audio" },
		"resolution": func(task map[string]any) { task["resolution"] = "2K" }, "duration": func(task map[string]any) { task["duration"] = 7 }, "duration-string": func(task map[string]any) { task["duration"] = "6" }, "ratio": func(task map[string]any) { task["ratio"] = "adaptive" }, "null-fact": func(task map[string]any) { task["resolution"] = nil },
		"unknown-status": func(task map[string]any) { task["status"] = "completed" }, "status-case": func(task map[string]any) { task["status"] = "Succeeded" }, "id-number": func(task map[string]any) { task["id"] = 424010985738629 }, "missing-model": func(task map[string]any) { delete(task, "model") },
		"missing-url": func(task map[string]any) { delete(task, "content") }, "empty-url": func(task map[string]any) { task["content"] = map[string]any{"url": ""} }, "url-type": func(task map[string]any) { task["content"] = map[string]any{"url": 3} }, "url-relative": func(task map[string]any) { task["content"] = map[string]any{"url": "/result"} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			body := hnPollBody("succeeded")
			edit(body["task"].(map[string]any))
			var gets, posts atomic.Int32
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(body) })
			obs, err := hnPollCall(context.Background(), root, b.ID, hnMiniMaxLocalOptions(t, s), hnMiniMaxSyntheticResolver)
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, ErrHNMiniMaxPollProtocol) || gets.Load() != 1 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) {
				t.Fatal("invalid task response mutated or retried")
			}
			hnPollAssertPrivate(t, root, obs, err)
		})
	}
	for name, body := range map[string]string{"malformed": "{", "array": "[]", "missing-task": "{}", "null-task": `{"task":null}`, "duplicate-task": `{"task":{},"task":{}}`, "duplicate-id": `{"task":{"id":"other","id":"` + hnMiniMaxFakeTask + `"}}`, "trailing": `{"task":{}} {}`, "oversized": strings.Repeat("x", hnMiniMaxResponseLimit+1)} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			var gets, posts atomic.Int32
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
			_, err := hnPollCall(context.Background(), root, b.ID, hnMiniMaxLocalOptions(t, s), hnMiniMaxSyntheticResolver)
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, ErrHNMiniMaxPollProtocol) || gets.Load() != 1 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) {
				t.Fatal("malformed response unsafe")
			}
		})
	}
	t.Log("TASK_ID_MODEL_TYPE_MODALITY_FROZEN_RESOLUTION_DURATION_RATIO=STRICT UNKNOWN_MALFORMED_DUPLICATE_OVERSIZED=REJECTED POLLING_FACT_MUTATION=NONE")
}

func TestHNMiniMaxPollNetworkFailures(t *testing.T) {
	for _, name := range []string{"timeout", "drop", "body-drop", "cancel"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			var gets, posts atomic.Int32
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) {
				switch name {
				case "timeout":
					time.Sleep(150 * time.Millisecond)
				case "cancel":
					cancel()
					time.Sleep(30 * time.Millisecond)
				case "drop", "body-drop":
					if name == "body-drop" {
						w.Header().Set("Content-Length", "1000")
						w.WriteHeader(200)
						io.WriteString(w, "{")
						w.(http.Flusher).Flush()
					}
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						conn.Close()
					}
					return
				}
				json.NewEncoder(w).Encode(hnPollBody("running"))
			})
			options := hnMiniMaxLocalOptions(t, s)
			if name == "timeout" {
				options.timeout = 50 * time.Millisecond
			}
			obs, err := hnPollCall(ctx, root, b.ID, options, hnMiniMaxSyntheticResolver)
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, ErrHNMiniMaxPollTransient) || gets.Load() != 1 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) {
				t.Fatal("network failure changed known acceptance or resent")
			}
			hnPollAssertPrivate(t, root, obs, err)
			t.Log("OBSERVED_GETS=1 SUBMIT_POSTS=0 POLL_TRANSIENT=PASS SUBMITTED_BOUND_LASTPOLLED_UNCHANGED=PASS")
		})
	}
}

func TestHNMiniMaxPollConcurrentUpdatesAndStaleResponse(t *testing.T) {
	root := t.TempDir()
	g, b := hnPollFixture(t, root)
	var gets, posts atomic.Int32
	s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hnPollBody("running")) })
	options := hnMiniMaxLocalOptions(t, s)
	var wg sync.WaitGroup
	var wins atomic.Int32
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := hnPollCall(context.Background(), root, b.ID, options, hnMiniMaxSyntheticResolver); err == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
	if wins.Load() != 6 || gets.Load() != 6 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || afterB.BindingState != "BOUND" || afterB.ProviderTaskID != b.ProviderTaskID || afterB.LastPolledAt == "" {
		t.Fatal("concurrent polls corrupted known task")
	}
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal("concurrent reopen failed")
	}
	rows, err := w.List("task_bindings")
	w.Close()
	if err != nil || len(rows) != 1 {
		t.Fatal("duplicate binding")
	}
	// The first nonterminal response is held until the second terminal update commits.
	entered, release := make(chan struct{}), make(chan struct{})
	var staleGets, stalePosts atomic.Int32
	stale := hnPollTLS(t, &staleGets, &stalePosts, func(w http.ResponseWriter, r *http.Request) {
		state := "failed"
		if staleGets.Load() == 1 {
			close(entered)
			<-release
			state = "running"
		}
		json.NewEncoder(w).Encode(hnPollBody(state))
	})
	staleOptions := hnMiniMaxLocalOptions(t, stale)
	finished := make(chan error, 1)
	go func() {
		_, e := hnPollCall(context.Background(), root, b.ID, staleOptions, hnMiniMaxSyntheticResolver)
		finished <- e
	}()
	<-entered
	if _, err := hnPollCall(context.Background(), root, b.ID, staleOptions, hnMiniMaxSyntheticResolver); err != nil {
		close(release)
		t.Fatal("terminal concurrent poll failed")
	}
	beforeG, beforeB := hnPollRawRecords(t, root, g.ID, b.ID)
	close(release)
	if !errors.Is(<-finished, ErrHNMiniMaxPollStateConflict) {
		t.Fatal("stale response not rejected")
	}
	afterG, afterB = hnPollRawRecords(t, root, g.ID, b.ID)
	if !reflect.DeepEqual(beforeG, afterG) || !reflect.DeepEqual(beforeB, afterB) || afterB.ErrorClass != "MINIMAX_TASK_FAILED" || staleGets.Load() != 2 || stalePosts.Load() != 0 {
		t.Fatal("stale response erased terminal facts")
	}
	t.Log("CONCURRENT_EXPLICIT_POLLS=6 OBSERVED_GETS=6 OWNERS=1 SUBMIT_POSTS=0 STALE_RUNNING_AFTER_FAILED=STATE_CONFLICT DURABLE_FACTS_UNCHANGED=PASS")
}

func TestHNMiniMaxPollRecoveryChild(t *testing.T) {
	if os.Getenv("HN_R13_RECOVERY_CHILD") != "1" {
		return
	}
	address := os.Getenv("HN_R13_FAKE_ADDRESS")
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("not loopback recovery fixture")
	}
	der, err := base64.StdEncoding.DecodeString(os.Getenv("HN_R13_FAKE_CERT"))
	if err != nil {
		t.Fatal("invalid fake certificate")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal("invalid fake certificate")
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	options := hnMiniMaxHTTPOptions{timeout: 2 * time.Second, tlsConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}, dialContext: func(ctx context.Context, network, dest string) (net.Conn, error) {
		if dest != "api.minimax.io:443" {
			return nil, errors.New("un-pinned recovery destination")
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	obs, err := hnPollCall(context.Background(), os.Getenv("HN_R13_RECOVERY_ROOT"), os.Getenv("HN_R13_RECOVERY_BINDING"), options, hnMiniMaxSyntheticResolver)
	if err != nil || obs.State != "succeeded" || !obs.ResultLocationAvailable {
		t.Fatal("process recovery failed")
	}
}
func TestHNMiniMaxPollProcessRestart(t *testing.T) {
	root := t.TempDir()
	g, b := hnPollFixture(t, root)
	var gets, posts atomic.Int32
	s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hnPollBody("succeeded")) })
	u, _ := url.Parse(s.URL)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHNMiniMaxPollRecoveryChild$")
	cmd.Env = append(os.Environ(), "HN_R13_RECOVERY_CHILD=1", "HN_R13_RECOVERY_ROOT="+root, "HN_R13_RECOVERY_BINDING="+b.ID, "HN_R13_FAKE_ADDRESS="+u.Host, "HN_R13_FAKE_CERT="+base64.StdEncoding.EncodeToString(s.Certificate().Raw))
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = output
		t.Fatal("recovery child failed; no raw child output exposed")
	}
	afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
	if gets.Load() != 1 || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || afterB.ProviderTaskID != b.ProviderTaskID || afterB.BindingState != "BOUND" || afterB.LastPolledAt == "" {
		t.Fatal("process restart resubmitted or lost task")
	}
	hnPollAssertPrivate(t, root, afterB, nil)
	t.Log("SEPARATE_PROCESS_REOPEN=PASS PERSISTED_PROVIDER_TASK_ID=EXACT OBSERVED_GETS=1 RECOVERY_SUBMIT_POSTS=0 GENERATION_BYTES=UNCHANGED")
}

func TestHNMiniMaxPollReadGateAndPersistenceFailure(t *testing.T) {
	for _, scenario := range []string{"duplicate-owner", "unrelated-submitting", "persistence-failure"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnPollFixture(t, root)
			var pendingID string
			if scenario == "unrelated-submitting" {
				p := hnMiniMaxPrepare(t, root, nil)
				w, err := foundation.Open(root, "test")
				if err != nil {
					t.Fatal("fixture Open failed")
				}
				_, _, err = w.BeginSubmission(p.GenerationID)
				w.Close()
				if err != nil {
					t.Fatal("fixture Begin failed")
				}
				pendingID = p.GenerationID
			}
			db, err := sql.Open("sqlite", filepath.ToSlash(filepath.Join(root, "test", "metadata", "hn-extension.sqlite")))
			if err != nil {
				t.Fatal("fixture connection failed")
			}
			defer db.Close()
			var beforePending string
			if scenario == "duplicate-owner" {
				other := b
				other.ID = "duplicate-owner"
				raw, _ := json.Marshal(other)
				if _, err = db.Exec("INSERT INTO task_bindings(id,project_id,generation_id,data) VALUES(?,?,?,?)", other.ID, other.ProjectID, other.GenerationID, string(raw)); err != nil {
					t.Fatal("duplicate fixture failed")
				}
			}
			if scenario == "unrelated-submitting" && db.QueryRow("SELECT data FROM generations WHERE id=?", pendingID).Scan(&beforePending) != nil {
				t.Fatal("pending fixture read failed")
			}
			if scenario == "persistence-failure" {
				if _, err = db.Exec("CREATE TRIGGER reject_r13_poll BEFORE UPDATE ON task_bindings BEGIN SELECT RAISE(ABORT,'synthetic persistence refusal'); END"); err != nil {
					t.Fatal("fault fixture failed")
				}
			}
			var gets, posts atomic.Int32
			s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hnPollBody("running")) })
			obs, err := hnPollCall(context.Background(), root, b.ID, hnMiniMaxLocalOptions(t, s), hnMiniMaxSyntheticResolver)
			want, count := ErrHNMiniMaxPollPreflight, int32(0)
			if scenario == "persistence-failure" {
				want, count = ErrHNMiniMaxPollStateConflict, 1
			}
			afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
			if !errors.Is(err, want) || gets.Load() != count || posts.Load() != 0 || !reflect.DeepEqual(g, afterG) || !reflect.DeepEqual(b, afterB) || obs != (HNProviderTaskObservation{}) {
				t.Fatal("gate or failed persistence changed facts")
			}
			if pendingID != "" {
				var afterPending string
				if db.QueryRow("SELECT data FROM generations WHERE id=?", pendingID).Scan(&afterPending) != nil || afterPending != beforePending {
					t.Fatal("poll reconciled unrelated live submission")
				}
			}
		})
	}
	t.Log("DUPLICATE_OWNER_UNRELATED_SUBMITTING=GET_0 PERSISTENCE_FAILURE_GETS=1 ACCEPTED_FACTS_UNCHANGED=PASS")
}

func TestHNMiniMaxPollOptionalFactsAndOuterInput(t *testing.T) {
	root := t.TempDir()
	g, b := hnPollFixture(t, root)
	var gets, posts atomic.Int32
	s := hnPollTLS(t, &gets, &posts, func(w http.ResponseWriter, r *http.Request) {
		body := hnPollBody("queued")
		task := body["task"].(map[string]any)
		for _, name := range []string{"modality", "resolution", "duration", "ratio"} {
			delete(task, name)
		}
		task["created_at"], task["usage"] = 1, map[string]any{"output_seconds": 0}
		json.NewEncoder(w).Encode(body)
	})
	options := hnMiniMaxLocalOptions(t, s)
	if obs, err := hnPollCall(context.Background(), root, b.ID, options, hnMiniMaxSyntheticResolver); err != nil || obs.State != "queued" {
		t.Fatal("optional absent facts rejected")
	}
	beforeG, beforeB := hnPollRawRecords(t, root, g.ID, b.ID)
	for _, input := range []struct{ project, binding, owner string }{{"../escape", b.ID, "test-owner"}, {"test", "../binding", "test-owner"}, {"test", b.ID, "sk-unsafe"}, {"test", "missing", "test-owner"}} {
		if _, err := pollHNMiniMax(context.Background(), root, input.project, input.binding, input.owner, hnMiniMaxSyntheticResolver, options); !errors.Is(err, ErrHNMiniMaxPollPreflight) {
			t.Fatal("unsafe outer input accepted")
		}
	}
	if _, err := pollHNMiniMax(nil, root, "test", b.ID, "test-owner", hnMiniMaxSyntheticResolver, options); !errors.Is(err, ErrHNMiniMaxPollPreflight) {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hnPollCall(ctx, root, b.ID, options, hnMiniMaxSyntheticResolver); !errors.Is(err, ErrHNMiniMaxPollTransient) {
		t.Fatal("cancelled pre-query context accepted")
	}
	afterG, afterB := hnPollRawRecords(t, root, g.ID, b.ID)
	if gets.Load() != 1 || posts.Load() != 0 || !reflect.DeepEqual(beforeG, afterG) || !reflect.DeepEqual(beforeB, afterB) {
		t.Fatal("outer input rejection made a request or mutation")
	}
	t.Log("OPTIONAL_REPORTED_FACTS=STRICT_IF_PRESENT OUTER_INVALID_INPUT_GETS=0 SUBMIT_POSTS=0")
}
