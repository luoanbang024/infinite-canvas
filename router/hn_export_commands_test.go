package router

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR33RouterFullLifecycleBoundaryOptionsAndNoLogging(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	old := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = old })
	engine := New()
	logs.Reset()
	intent := uuid.NewString()
	base := "http://127.0.0.1/api/hn/projects/test/sequences/"
	routes := []struct{ method, path string }{{"POST", "export-protocol/initialize"}, {"POST", "export-commands"}, {"GET", "export-commands/" + intent}, {"GET", "export-commands/" + intent + "/bundle-verification"}}
	for _, route := range routes {
		for _, remote := range []bool{true, false} {
			body := ""
			if route.method == "POST" {
				body = `{"protocolVersion":1}`
			}
			r := httptest.NewRequest(route.method, base+"other/"+route.path, strings.NewReader(body))
			r.RemoteAddr = "127.0.0.1:9"
			if remote {
				r.RemoteAddr = "192.0.2.9:9"
			}
			r.Header.Set("X-HN-Local-Request", "1")
			r.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			engine.ServeHTTP(out, r)
			expected := 400
			if remote {
				expected = 403
			}
			if out.Code != expected || out.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(route, out.Code, out.Body.String())
			}
		}
		r := httptest.NewRequest("OPTIONS", base+"main/"+route.path, nil)
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("Origin", "http://localhost:3017")
		r.Header.Set("Access-Control-Request-Method", route.method)
		r.Header.Set("Access-Control-Request-Headers", "X-HN-Local-Request")
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, r)
		if out.Code != 204 || out.Header().Get("Access-Control-Allow-Methods") != route.method {
			t.Fatal(out.Code, out.Body.String())
		}
		r = httptest.NewRequest(route.method, base+"main/"+route.path+"?untrusted=r33-do-not-log", nil)
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("X-HN-Local-Request", "1")
		out = httptest.NewRecorder()
		engine.ServeHTTP(out, r)
		if out.Code != 400 {
			t.Fatal(out.Code)
		}
	}
	if logs.Len() != 0 {
		t.Fatal("new request log leaked", logs.String())
	}
	for _, method := range []string{"GET", "POST"} {
		r := httptest.NewRequest("OPTIONS", base+"main/export", nil)
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("Access-Control-Request-Method", method)
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, r)
		expected := 204
		if method == "GET" {
			expected = 403
		}
		if out.Code != expected {
			t.Fatal("legacy options changed", out.Code)
		}
	}
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	shot, e := w.CreateShot("synthetic", "node")
	if e != nil {
		t.Fatal(e)
	}
	g, e := w.CreateGeneration(foundation.Generation{ShotID: shot.ID, PromptSnapshot: "synthetic", SourceBaseline: "790439097de0f3f03995c9ea9c0ef010091d7b56"})
	if e != nil {
		t.Fatal(e)
	}
	g, e = w.FreezeGeneration(g.ID)
	if e != nil {
		t.Fatal(e)
	}
	result, e := w.CreateResult(foundation.Result{GenerationID: g.ID, ResultKind: "video"})
	if e != nil {
		t.Fatal(e)
	}
	job, e := w.CreateArchive(result.ID, "generated/"+g.ID+"/"+result.ID+"/media.mp4", "video/mp4")
	if e != nil {
		t.Fatal(e)
	}
	data := []byte("synthetic HTTP fixture bytes")
	if e = w.RunArchive(job.ID, bytes.NewReader(data), int64(len(data)), ""); e != nil {
		t.Fatal(e)
	}
	candidate, e := w.CreateCandidate(shot.ID, result.ID, "Local Archive")
	if e != nil {
		t.Fatal(e)
	}
	if e = w.SelectCandidate(shot.ID, candidate.ID); e != nil {
		t.Fatal(e)
	}
	item, e := w.AddSequenceItem("main", candidate.ID)
	if e != nil {
		t.Fatal(e)
	}
	snapshot, e := w.ReadReorderSnapshot()
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, base+"main/"+path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("X-HN-Local-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:3017")
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, r)
		return out
	}
	missing := request("GET", "export-commands/"+intent, "")
	if missing.Code != 409 || !strings.Contains(missing.Body.String(), "EXPORT_PROTOCOL_NOT_INITIALIZED") {
		t.Fatal(missing.Code, missing.Body.String())
	}
	init := request("POST", "export-protocol/initialize", `{"protocolVersion":1}`)
	if init.Code != 200 {
		t.Fatal(init.Code, init.Body.String())
	}
	absent := request("GET", "export-commands/"+intent, "")
	if absent.Code != 200 || !strings.Contains(absent.Body.String(), "NOT_OBSERVED") {
		t.Fatal(absent.Body.String())
	}
	c := foundation.ExportCommand{ProtocolVersion: 1, ExportIntentID: intent, ExpectedRevision: snapshot.SequenceRevision, OrderedSequenceItemIDs: []string{item.ID}, Format: foundation.ExportFormat}
	b, _ := json.Marshal(c)
	post := request("POST", "export-commands", string(b))
	if post.Code != 200 {
		t.Fatal(post.Code, post.Body.String())
	}
	var envelope struct {
		Data foundation.ExportStatus `json:"data"`
	}
	if e = json.Unmarshal(post.Body.Bytes(), &envelope); e != nil || envelope.Data.Outcome != "COMMITTED" {
		t.Fatal(e, post.Body.String())
	}
	lookup := request("GET", "export-commands/"+intent, "")
	if !bytes.Equal(post.Body.Bytes(), lookup.Body.Bytes()) {
		t.Fatal("lookup receipt mismatch", lookup.Body.String())
	}
	health := request("GET", "export-commands/"+intent+"/bundle-verification", "")
	if health.Code != 200 || !strings.Contains(health.Body.String(), "VERIFIED") {
		t.Fatal(health.Body.String())
	}
	if dir := os.Getenv("HN_R33_EVIDENCE"); dir != "" {
		os.MkdirAll(dir, 0700)
		b, _ := json.MarshalIndent(map[string]any{"command": c, "post": json.RawMessage(post.Body.Bytes()), "lookup": json.RawMessage(lookup.Body.Bytes()), "health": json.RawMessage(health.Body.Bytes()), "newRequestLogging": "NONE", "boundaryBeforeMain": true, "legacyOptionsPreserved": true}, "", "  ")
		if e = os.WriteFile(filepath.Join(dir, "http-lifecycle.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
