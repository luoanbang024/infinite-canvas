package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func r28Fixture(t *testing.T) (r22Fixture, foundation.PlacementCommand) {
	f := r22LocalFixture(t, "r28")
	c, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive")
	if status != 200 {
		t.Fatal(status)
	}
	_, e := service.SelectLocalCandidate(f.root, f.project, f.shot, c.CandidateID)
	if e != nil {
		t.Fatal(e)
	}
	return f, foundation.PlacementCommand{ProtocolVersion: 1, PlacementIntentID: uuid.NewString(), PlacementOwner: foundation.PlacementOwner{CandidateID: c.CandidateID, ShotID: f.shot, GenerationID: f.generation, ResultID: f.archive.ResultID, ArchiveJobID: f.archive.ArchiveJobID, PreparedFrozenHash: f.frozenHash}}
}
func r28Request(method, body string) *http.Request {
	r := httptest.NewRequest(method, "http://127.0.0.1/", strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("X-HN-Local-Request", "1")
	r.Header.Set("Content-Type", "application/json")
	return r
}
func TestR28HandlerClosedBoundaryLookupAndReadonly(t *testing.T) {
	f, p := r28Fixture(t)
	b, _ := json.Marshal(p)
	for _, body := range []string{`null`, `{}`, string(b) + ` {}`, strings.Replace(string(b), `"protocolVersion":1`, `"ProtocolVersion":1`, 1), strings.Replace(string(b), `"protocolVersion":1`, `"protocolVersion":1,"protocolVersion":1`, 1), strings.TrimSuffix(string(b), "}") + `,"extra":1}`} {
		out := httptest.NewRecorder()
		HNPlacementCommand(out, r28Request("POST", body), f.project)
		if out.Code != 400 {
			t.Fatal(body, out.Code)
		}
	}
	for _, scenario := range []string{"remote", "host", "origin", "marker", "query", "body", "cookie", "authorization"} {
		r := r28Request("GET", "")
		switch scenario {
		case "remote":
			r.RemoteAddr = "192.0.2.1:9"
		case "host":
			r.Host = "synthetic.invalid"
		case "origin":
			r.Header.Set("Origin", "https://synthetic.invalid")
		case "marker":
			r.Header.Del("X-HN-Local-Request")
		case "query":
			r.URL.RawQuery = "unexpected=1"
		case "body":
			r.Body = io.NopCloser(strings.NewReader("x"))
		case "cookie":
			r.Header.Set("Cookie", "synthetic=value")
		case "authorization":
			r.Header.Set("Authorization", "synthetic")
		}
		out := httptest.NewRecorder()
		HNReadMainSequence(out, r, f.project)
		if out.Code != 400 && out.Code != 403 {
			t.Fatal(scenario, out.Code)
		}
	}
	out := httptest.NewRecorder()
	HNLookupPlacement(out, r28Request("GET", ""), f.project, p.PlacementIntentID)
	if out.Code != 409 || !strings.Contains(out.Body.String(), "PLACEMENT_PROTOCOL_NOT_INITIALIZED") {
		t.Fatal(out.Body.String())
	}
	path := filepath.Join(f.root, f.project, "metadata", "hn-extension.sqlite")
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	out = httptest.NewRecorder()
	HNReadMainSequence(out, r28Request("GET", ""), f.project)
	if out.Code != 200 || out.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(out.Code, out.Body.String())
	}
	after, e := os.ReadFile(path)
	if e != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("GET wrote DB")
	}
	out = httptest.NewRecorder()
	HNPlacementCommand(out, r28Request("POST", string(b)), f.project)
	if out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	var a struct{ Data foundation.PlacementReceipt }
	json.Unmarshal(out.Body.Bytes(), &a)
	lookup := httptest.NewRecorder()
	HNLookupPlacement(lookup, r28Request("GET", ""), f.project, p.PlacementIntentID)
	if lookup.Code != 200 {
		t.Fatal(lookup.Body.String())
	}
	var found struct{ Data foundation.PlacementReceipt }
	json.Unmarshal(lookup.Body.Bytes(), &found)
	if !reflect.DeepEqual(a, found) {
		t.Fatal("receipt changed")
	}
	notObserved := httptest.NewRecorder()
	HNLookupPlacement(notObserved, r28Request("GET", ""), f.project, uuid.NewString())
	if notObserved.Code != 200 || !strings.Contains(notObserved.Body.String(), "NOT_OBSERVED") {
		t.Fatal(notObserved.Body.String())
	}
	r := r28Request("OPTIONS", "")
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", "X-HN-Local-Request")
	preflight := httptest.NewRecorder()
	HNPlacementReadOptions(preflight, r)
	if preflight.Code != 204 || preflight.Header().Get("Access-Control-Allow-Methods") != "GET" {
		t.Fatal(preflight.Code)
	}
	old := httptest.NewRecorder()
	HNReferenceOptions(old, r)
	if old.Code != 403 {
		t.Fatal("old POST preflight weakened")
	}
	if dir := os.Getenv("HN_R28_HTTP_EVIDENCE"); dir != "" {
		os.MkdirAll(dir, 0700)
		for name, record := range map[string]*httptest.ResponseRecorder{"committed": out, "lookup": lookup, "not-observed": notObserved} {
			os.WriteFile(filepath.Join(dir, name+".json"), record.Body.Bytes(), 0600)
		}
	}
}
func TestR28ActualHTTPCommittedLossContinuationAndDelayedRace(t *testing.T) {
	f, p := r28Fixture(t)
	body, _ := json.Marshal(p)
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		out := httptest.NewRecorder()
		HNPlacementCommand(out, r, f.project)
		if out.Code != 200 {
			t.Error(out.Code, out.Body.String())
		}
		conn, _, e := w.(http.Hijacker).Hijack()
		if e != nil {
			t.Error(e)
			return
		}
		conn.Close()
	}))
	defer server.Close()
	r, e := http.NewRequest("POST", server.URL, bytes.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-HN-Local-Request", "1")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	_, e = client.Do(r)
	if e == nil {
		t.Fatal("response loss not observed")
	}
	lookup := httptest.NewRecorder()
	HNLookupPlacement(lookup, r28Request("GET", ""), f.project, p.PlacementIntentID)
	if lookup.Code != 200 {
		t.Fatal(lookup.Body.String())
	}
	var ack struct{ Data foundation.PlacementReceipt }
	json.Unmarshal(lookup.Body.Bytes(), &ack)
	continued := httptest.NewRecorder()
	HNPlacementCommand(continued, r28Request("POST", string(body)), f.project)
	var same struct{ Data foundation.PlacementReceipt }
	json.Unmarshal(continued.Body.Bytes(), &same)
	if continued.Code != 200 || !reflect.DeepEqual(ack, same) {
		t.Fatal("continuation not exact")
	}
	if len(r22Snapshot(t, f)["sequence_items"]) != 1 || posts.Load() != 1 {
		t.Fatal("duplicate")
	}
	p.PlacementIntentID = uuid.NewString()
	body, _ = json.Marshal(p)
	var wg sync.WaitGroup
	start := make(chan struct{})
	outcomes := make(chan foundation.PlacementReceipt, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out := httptest.NewRecorder()
			HNPlacementCommand(out, r28Request("POST", string(body)), f.project)
			if out.Code != 200 {
				t.Error(out.Code, out.Body.String())
			}
			var receipt struct{ Data foundation.PlacementReceipt }
			json.Unmarshal(out.Body.Bytes(), &receipt)
			outcomes <- receipt.Data
		}()
	}
	close(start)
	wg.Wait()
	a, b := <-outcomes, <-outcomes
	if !reflect.DeepEqual(a, b) || len(r22Snapshot(t, f)["sequence_items"]) != 2 {
		t.Fatal("delayed continuation race")
	}
	if dir := os.Getenv("HN_R28_HTTP_EVIDENCE"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"serverObservedDroppedPOST": posts.Load(), "lookup": ack.Data, "sameKeyContinuation": same.Data, "itemCountAfterRecovery": 1, "delayedOriginalRacingContinuationSameReceipt": true, "itemCountAfterNewIntentRace": 2, "ProviderCalls": 0}, "", "  ")
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "committed-loss-race.json"), data, 0600)
	}
}

func TestR28LegacyHTTPFreshAndTerminalRejection(t *testing.T) {
	f, p := r28Fixture(t)
	items := []service.HNSequenceItem{}
	for n := 0; n < 2; n++ {
		out := httptest.NewRecorder()
		HNAddSequenceItem(out, r28Request("POST", `{"candidateId":"`+p.CandidateID+`"}`), f.project, "main")
		if out.Code != 200 {
			t.Fatal(out.Body.String())
		}
		var v struct{ Data service.HNSequenceItem }
		if json.Unmarshal(out.Body.Bytes(), &v) != nil {
			t.Fatal("legacy response")
		}
		items = append(items, v.Data)
		var shape map[string]json.RawMessage
		json.Unmarshal(out.Body.Bytes(), &shape)
		var dto map[string]json.RawMessage
		json.Unmarshal(shape["data"], &dto)
		if len(dto) != 9 {
			t.Fatal("legacy DTO changed")
		}
	}
	if items[0].SequenceItemID == items[1].SequenceItemID || items[0].OrderIndex != 0 || items[1].OrderIndex != 1 {
		t.Fatal("legacy not fresh")
	}
	out := httptest.NewRecorder()
	HNAddSequenceItem(out, r28Request("POST", `{"candidateId":"`+p.CandidateID+`","placementIntentId":"`+p.PlacementIntentID+`"}`), f.project, "main")
	if out.Code != 400 {
		t.Fatal("legacy accepted command identity", out.Code)
	}
	p.PreparedFrozenHash = strings.Repeat("a", 64)
	b, _ := json.Marshal(p)
	out = httptest.NewRecorder()
	HNPlacementCommand(out, r28Request("POST", string(b)), f.project)
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"outcome":"REJECTED"`) {
		t.Fatal(out.Body.String())
	}
	lookup := httptest.NewRecorder()
	HNLookupPlacement(lookup, r28Request("GET", ""), f.project, p.PlacementIntentID)
	if !bytes.Equal(out.Body.Bytes(), lookup.Body.Bytes()) {
		t.Fatal("terminal rejection drift")
	}
	if len(r22Snapshot(t, f)["sequence_items"]) != 2 {
		t.Fatal("rejected command created item")
	}
	if dir := os.Getenv("HN_R28_HTTP_EVIDENCE"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"legacyItems": items, "legacyExtraIdentityRejected": true, "terminalRejected": json.RawMessage(out.Body.Bytes()), "itemsAfterRejection": 2}, "", "  ")
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "legacy-rejected.json"), data, 0600)
	}
}

func TestR28ProductionHTTPBunClientRecovery(t *testing.T) {
	bin := os.Getenv("HN_R26_BUN")
	if bin == "" {
		t.Skip("execution runner supplies installed Bun")
	}
	f, p := r28Fixture(t)
	var posts, queries, forbidden atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/hn/projects/" + f.project + "/sequences/main/placement-commands"
		if r.URL.Path == "/api/hn/local-endpoint" && r.Method == "GET" {
			OK(w, map[string]string{"url": server.URL})
			return
		}
		if r.URL.Path == base && r.Method == "POST" {
			n := posts.Add(1)
			if n == 1 {
				out := httptest.NewRecorder()
				HNPlacementCommand(out, r, f.project)
				if out.Code != 200 {
					t.Error(out.Body.String())
				}
				conn, _, e := w.(http.Hijacker).Hijack()
				if e != nil {
					t.Error(e)
					return
				}
				conn.Close()
				return
			}
			HNPlacementCommand(w, r, f.project)
			return
		}
		if r.URL.Path == base+"/"+p.PlacementIntentID && r.Method == "GET" {
			queries.Add(1)
			HNLookupPlacement(w, r, f.project, p.PlacementIntentID)
			return
		}
		forbidden.Add(1)
		http.Error(w, "unexpected", 404)
	}))
	defer server.Close()
	fixture, _ := json.Marshal(map[string]any{"base": server.URL, "project": f.project, "command": p})
	script := `import assert from "node:assert/strict";import {executePlacement,lookupPlacement} from "./src/services/hn/local-sequence-placement.ts";const f=JSON.parse(process.env.HN_R28_BRIDGE_FIXTURE);const native=fetch;let posts=0;const request=async(u,o)=>{assert.equal(o.credentials,"omit");assert.equal(o.redirect,"error");if(o.method==="POST")posts++;return native(u.startsWith("/")?f.base+u:u,o)};await assert.rejects(()=>executePlacement(f.project,f.command,request));assert.equal(posts,1);const receipt=await lookupPlacement(f.project,f.command,request);assert.equal(receipt.outcome,"COMMITTED");assert.equal(posts,1);const continued=await executePlacement(f.project,f.command,request);assert.deepEqual(continued,receipt);assert.equal(posts,2);console.log(JSON.stringify({actualPOST:posts,explicitLookup:1,sameReceipt:true,automaticResend:0}));`
	cmd := exec.Command(bin, "--no-env-file", "-e", script)
	cmd.Dir = filepath.Join("..", "web")
	cmd.Env = append(os.Environ(), "HN_R28_BRIDGE_FIXTURE="+string(fixture))
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatal(e, string(out))
	}
	if posts.Load() != 2 || queries.Load() != 1 || forbidden.Load() != 0 || len(r22Snapshot(t, f)["sequence_items"]) != 1 {
		t.Fatal("client/server effects", posts.Load(), queries.Load(), forbidden.Load())
	}
	if dir := os.Getenv("HN_R28_HTTP_EVIDENCE"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"runtime": "installed Bun production TypeScript service + actual Go HTTP handlers", "serverPOST": posts.Load(), "serverLookupGET": queries.Load(), "items": 1, "forbidden": forbidden.Load(), "clientEvidence": string(out), "providerCalls": 0}, "", "  ")
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "bun-production-bridge.json"), data, 0600)
	}
}
