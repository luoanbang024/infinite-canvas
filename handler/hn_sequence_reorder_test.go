package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestR30HandlerBoundaryAndClosedBodies(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	routes := []struct {
		name, method string
		call         func(http.ResponseWriter, *http.Request, string)
	}{
		{"initialize", "POST", func(w http.ResponseWriter, r *http.Request, s string) { HNInitializeReorder(w, r, "test", s) }},
		{"snapshot", "GET", func(w http.ResponseWriter, r *http.Request, s string) { HNReorderSnapshot(w, r, "test", s) }},
		{"command", "POST", func(w http.ResponseWriter, r *http.Request, s string) { HNReorderCommand(w, r, "test", s) }},
		{"lookup", "GET", func(w http.ResponseWriter, r *http.Request, s string) {
			HNLookupReorder(w, r, "test", s, uuid.NewString())
		}},
	}
	evidence := []map[string]any{}
	for _, route := range routes {
		for _, scenario := range []string{"remote", "host", "origin", "marker", "authorization", "cookie", "query", "valid-non-main"} {
			r := r28Request(route.method, `{"protocolVersion":1}`)
			if route.method == "GET" {
				r.Body = io.NopCloser(strings.NewReader(""))
			}
			switch scenario {
			case "remote":
				r.RemoteAddr = "192.0.2.9:1"
			case "host":
				r.Host = "synthetic.invalid"
			case "origin":
				r.Header.Set("Origin", "https://synthetic.invalid")
			case "marker":
				r.Header.Del("X-HN-Local-Request")
			case "authorization":
				r.Header.Set("Authorization", "synthetic")
			case "cookie":
				r.Header.Set("Cookie", "synthetic=value")
			case "query":
				r.URL.RawQuery = "synthetic=private_probe"
			}
			out := httptest.NewRecorder()
			route.call(out, r, "other")
			status := 403
			if scenario == "query" || scenario == "valid-non-main" {
				status = 400
			}
			if out.Code != status || out.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(route.name, scenario, out.Code, out.Body.String())
			}
			if scenario == "valid-non-main" && !strings.Contains(out.Body.String(), "REORDER_INPUT_INVALID") {
				t.Fatal("main validated before boundary")
			}
			if strings.Contains(out.Body.String(), "private_probe") {
				t.Fatal("query reflected")
			}
			evidence = append(evidence, map[string]any{"route": route.name, "scenario": scenario, "status": status, "noStore": true})
		}
	}
	c := foundation.ReorderCommand{1, uuid.NewString(), "0", []string{}}
	raw, _ := json.Marshal(c)
	invalid := []string{`null`, `{}`, string(raw) + ` {}`, strings.Replace(string(raw), `"protocolVersion":1`, `"protocolVersion":1,"protocolVersion":1`, 1), strings.TrimSuffix(string(raw), "}") + `,"extra":1}`, strings.Replace(string(raw), `"expectedRevision":"0"`, `"expectedRevision":0`, 1), strings.Replace(string(raw), `"expectedRevision":"0"`, `"expectedRevision":"00"`, 1)}
	for _, body := range invalid {
		out := httptest.NewRecorder()
		HNReorderCommand(out, r28Request("POST", body), "test", "main")
		if out.Code != 400 {
			t.Fatal(body, out.Code)
		}
	}
	for _, body := range []string{`null`, `{}`, `{"protocolVersion":1,"protocolVersion":1}`, `{"protocolVersion":1,"extra":0}`} {
		out := httptest.NewRecorder()
		HNInitializeReorder(out, r28Request("POST", body), "test", "main")
		if out.Code != 400 {
			t.Fatal(body, out.Code)
		}
	}
	badMedia := r28Request("POST", string(raw))
	badMedia.Header.Set("Content-Type", "text/plain")
	out := httptest.NewRecorder()
	HNReorderCommand(out, badMedia, "test", "main")
	if out.Code != 400 {
		t.Fatal(out.Code)
	}
	oversize := r28Request("POST", strings.Repeat(" ", 65537)+string(raw))
	out = httptest.NewRecorder()
	HNReorderCommand(out, oversize, "test", "main")
	if out.Code != 400 {
		t.Fatal(out.Code)
	}
	r := r28Request("GET", "x")
	out = httptest.NewRecorder()
	HNReorderSnapshot(out, r, "test", "main")
	if out.Code != 400 {
		t.Fatal("GET body allowed")
	}
	files, e := os.ReadDir(root)
	if e != nil || len(files) != 0 {
		t.Fatal("boundary/input created store")
	}
	r30HandlerEvidence(t, "local-boundary", map[string]any{"rows": evidence, "rootEntries": 0, "closedBodies": len(invalid) + 4})
}
func r30HandlerEvidence(t *testing.T, name string, v any) {
	t.Helper()
	if dir := os.Getenv("HN_R30_EVIDENCE"); dir != "" {
		os.MkdirAll(dir, 0700)
		b, _ := json.Marshal(v)
		if e := os.WriteFile(filepath.Join(dir, name+".json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func TestR30HandlerReadonlyAndResponseLoss(t *testing.T) {
	f, p := r28Fixture(t)
	t.Setenv("HN_PROJECTS_ROOT", f.root)
	db := filepath.Join(f.root, f.project, "metadata", "hn-extension.sqlite")
	before, e := os.ReadFile(db)
	if e != nil {
		t.Fatal(e)
	}
	out := httptest.NewRecorder()
	HNReorderSnapshot(out, r28Request("GET", ""), f.project, "main")
	if out.Code != 409 || !strings.Contains(out.Body.String(), "REORDER_PROTOCOL_NOT_INITIALIZED") {
		t.Fatal(out.Code, out.Body.String())
	}
	after, e := os.ReadFile(db)
	if e != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("GET wrote DB")
	}
	out = httptest.NewRecorder()
	HNInitializeReorder(out, r28Request("POST", `{"protocolVersion":1}`), f.project, "main")
	if out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	var initBody struct{ Data foundation.ReorderState }
	json.Unmarshal(out.Body.Bytes(), &initBody)
	if initBody.Data.SequenceRevision != "0" {
		t.Fatal(initBody)
	}
	ids := []string{}
	for n := 0; n < 3; n++ {
		i, e := service.AddLocalSequenceItem(f.root, f.project, "main", p.CandidateID)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, i.SequenceItemID)
	}
	var posts atomic.Int32
	var mu sync.Mutex
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts.Add(1)
			raw, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var c foundation.ReorderCommand
			json.Unmarshal(raw, &c)
			mu.Lock()
			drop := !seen[c.ReorderIntentID]
			seen[c.ReorderIntentID] = true
			mu.Unlock()
			if drop {
				discard := httptest.NewRecorder()
				HNReorderCommand(discard, r, f.project, "main")
				conn, _, e := w.(http.Hijacker).Hijack()
				if e != nil {
					t.Error(e)
					return
				}
				conn.Close()
				return
			}
			HNReorderCommand(w, r, f.project, "main")
			return
		}
		intent := strings.TrimPrefix(r.URL.Path, "/")
		HNLookupReorder(w, r, f.project, "main", intent)
	}))
	defer server.Close()
	request := func(method, path string, c *foundation.ReorderCommand) (*http.Response, error) {
		var body io.Reader
		if c != nil {
			b, _ := json.Marshal(c)
			body = bytes.NewReader(b)
		}
		r, e := http.NewRequest(method, server.URL+"/"+path, body)
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("X-HN-Local-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		return server.Client().Do(r)
	}
	c := foundation.ReorderCommand{1, uuid.NewString(), "3", []string{ids[2], ids[0], ids[1]}}
	response, e := request("POST", "", &c)
	if e == nil {
		response.Body.Close()
		t.Fatal("expected committed response drop")
	}
	if posts.Load() != 1 {
		t.Fatal("automatic resend")
	}
	response, e = request("GET", c.ReorderIntentID, nil)
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct{ Data foundation.ReorderReceipt }
	json.NewDecoder(response.Body).Decode(&envelope)
	response.Body.Close()
	original := envelope.Data
	if original.Outcome != "COMMITTED" || *original.AppliedRevision != "4" {
		t.Fatal(original)
	}
	if _, e = service.ReorderLocalSequence(f.root, f.project, "main", ids); e != nil {
		t.Fatal(e)
	}
	response, e = request("POST", "", &c)
	if e != nil {
		t.Fatal(e)
	}
	var continued struct{ Data foundation.ReorderReceipt }
	json.NewDecoder(response.Body).Decode(&continued)
	response.Body.Close()
	if !reflect.DeepEqual(original, continued.Data) {
		t.Fatal("changed historic receipt")
	}
	current, e := service.ReadLocalReorderSnapshot(f.root, f.project)
	if e != nil || current.SequenceRevision != "5" || current.Items[0].SequenceItemID != ids[0] {
		t.Fatal("continuation reapplied", current, e)
	}
	stale := c
	stale.ReorderIntentID = uuid.NewString()
	response, e = request("POST", "", &stale)
	if e == nil {
		response.Body.Close()
		t.Fatal("expected conflict response drop")
	}
	response, e = request("GET", stale.ReorderIntentID, nil)
	if e != nil {
		t.Fatal(e)
	}
	var rejected struct{ Data foundation.ReorderReceipt }
	json.NewDecoder(response.Body).Decode(&rejected)
	response.Body.Close()
	if rejected.Data.Outcome != "CONFLICT" || *rejected.Data.ErrorClass != "SEQUENCE_REVISION_CONFLICT" {
		t.Fatal(rejected)
	}
	// An observed absent exact key says nothing about a later pending dispatch.
	pending := stale
	pending.ReorderIntentID = uuid.NewString()
	pending.ExpectedRevision = "5"
	absent, e := service.LookupLocalReorder(f.root, f.project, pending.ReorderIntentID)
	if e != nil || absent.(foundation.ReorderLookup).Outcome != "NOT_OBSERVED" {
		t.Fatal(absent, e)
	}
	var wg sync.WaitGroup
	receipts := make([]foundation.ReorderReceipt, 2)
	errs := make([]error, 2)
	for n := range receipts {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			receipts[n], errs[n] = service.ExecuteLocalReorder(f.root, f.project, pending)
		}(n)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(receipts[0], receipts[1]) || *receipts[0].AppliedRevision != "6" {
		t.Fatal("delayed same-key effect")
	}
	r30HandlerEvidence(t, "response-loss-recovery", map[string]any{"serverObservedPOSTs": posts.Load(), "automaticPOSTRetry": 0, "committed": original, "continued": continued.Data, "conflict": rejected.Data, "notObservedBeforeDelayed": absent, "delayedSameKeyReceipts": receipts, "currentBeforeDelayed": current, "readonlyZeroWrite": true})
}
