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

func TestR30RouterRoutesBoundaryOptionsAndNoLogging(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	old := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = old })
	engine := New()
	logs.Reset()
	base := "http://127.0.0.1/api/hn/projects/test/sequences/"
	routes := []struct{ method, path string }{
		{"POST", "reorder-protocol/initialize"}, {"GET", "reorder-snapshot"},
		{"POST", "reorder-commands"}, {"GET", "reorder-commands/" + uuid.NewString()},
	}
	evidence := []map[string]any{}
	for _, route := range routes {
		for _, remote := range []bool{true, false} {
			r := httptest.NewRequest(route.method, base+"other/"+route.path, strings.NewReader(`{"protocolVersion":1}`))
			if route.method == "GET" {
				r = httptest.NewRequest("GET", base+"other/"+route.path, nil)
			}
			r.RemoteAddr = "127.0.0.1:9"
			if remote {
				r.RemoteAddr = "192.0.2.9:9"
			}
			r.Header.Set("X-HN-Local-Request", "1")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, r)
			expected := 400
			if remote {
				expected = 403
			}
			if w.Code != expected || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(route, w.Code, w.Body.String())
			}
			evidence = append(evidence, map[string]any{"route": route.path, "remote": remote, "status": w.Code})
		}
	}
	for _, route := range routes {
		r := httptest.NewRequest("OPTIONS", base+"main/"+route.path, nil)
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("Access-Control-Request-Method", route.method)
		r.Header.Set("Access-Control-Request-Headers", "X-HN-Local-Request")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Code != 204 || w.Header().Get("Access-Control-Allow-Methods") != route.method {
			t.Fatal("preflight", route, w.Code)
		}
		r.Header.Set("Access-Control-Request-Headers", "Authorization")
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("auth preflight")
		}
	}
	r := httptest.NewRequest("OPTIONS", "http://127.0.0.1/api/hn/projects/test/references", nil)
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("old POST OPTIONS weakened")
	}
	for _, route := range routes {
		logs.Reset()
		r := httptest.NewRequest(route.method, base+"main/"+route.path+"?value=synthetic_sensitive_text", nil)
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("X-HN-Local-Request", "1")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Code != 400 || logs.Len() != 0 || strings.Contains(w.Body.String(), "synthetic_sensitive_text") {
			t.Fatal("logging/echo", logs.String(), w.Body.String())
		}
	}
	store, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal(e)
	}
	store.Close()
	call := func(method, path, body string) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, base+"main/"+path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:9"
		r.Header.Set("X-HN-Local-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
		var response map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		return response
	}
	init := call("POST", "reorder-protocol/initialize", `{"protocolVersion":1}`)
	c := foundation.ReorderCommand{1, uuid.NewString(), "0", []string{}}
	b, _ := json.Marshal(c)
	committed := call("POST", "reorder-commands", string(b))
	snapshot := call("GET", "reorder-snapshot", "")
	if len(snapshot["data"].(map[string]any)) != 6 || snapshot["data"].(map[string]any)["sequenceRevision"] != "1" {
		t.Fatal(snapshot)
	}
	lookup := call("GET", "reorder-commands/"+c.ReorderIntentID, "")
	items := call("GET", "items", "")
	if len(items["data"].(map[string]any)) != 5 {
		t.Fatal("R28 closed shape changed")
	}
	legacy := call("POST", "reorder", `{"sequenceItemIds":[]}`)
	if len(legacy["data"].([]any)) != 0 {
		t.Fatal("legacy empty array shape")
	}
	if dir := os.Getenv("HN_R30_EVIDENCE"); dir != "" {
		os.MkdirAll(dir, 0700)
		b, _ := json.Marshal(map[string]any{"boundaryRows": evidence, "logsBytes": 0, "initialize": init, "command": committed, "snapshot": snapshot, "lookup": lookup, "r28Items": items, "legacy": legacy})
		if e := os.WriteFile(filepath.Join(dir, "router-shapes-options-logs.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
