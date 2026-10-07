package router

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestR28RouterReadPreflightAndLogClosure(t *testing.T) {
	t.Setenv("HN_PROJECTS_ROOT", t.TempDir())
	old := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = old })
	engine := New()
	logs.Reset()
	path := "http://127.0.0.1/api/hn/projects/test/sequences/main/items"
	r := httptest.NewRequest("OPTIONS", path, nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", "X-HN-Local-Request")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Methods") != "GET" {
		t.Fatal(w.Code, w.Body.String())
	}
	r = httptest.NewRequest("OPTIONS", "http://127.0.0.1/api/hn/projects/test/references", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("Access-Control-Request-Method", "GET")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unrelated POST preflight weakened")
	}
	logs.Reset()
	r = httptest.NewRequest("GET", path+"?unexpected=synthetic_sensitive_text", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-HN-Local-Request", "1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 400 || logs.Len() != 0 || bytes.Contains(w.Body.Bytes(), []byte("synthetic_sensitive_text")) {
		t.Fatal("recovery query logged/echoed", logs.String(), w.Body.String())
	}
	r = httptest.NewRequest("GET", "http://127.0.0.1/api/hn/projects/test/sequences/other/items", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-HN-Local-Request", "1")
	r.Header.Set("Origin", "http://localhost:3000")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("non-main read allowed")
	}
}

// Invalid sequence IDs must never bypass the local security boundary.
func TestR28AuditFixRouterBoundaryBeforeSequence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	old := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = old })
	engine := New()
	logs.Reset()
	var evidence []map[string]any
	routes := []struct{ method, suffix string }{
		{"GET", "items"},
		{"POST", "placement-commands"},
		{"GET", "placement-commands/00000000-0000-4000-8000-000000000001"},
	}
	for _, route := range routes {
		for _, scenario := range []string{"remote", "host", "origin", "marker", "cookie", "authorization", "local"} {
			t.Run(route.method+"/"+route.suffix+"/"+scenario, func(t *testing.T) {
				body := ""
				if route.method == "POST" {
					body = `{"protocolVersion":1,"placementIntentId":"00000000-0000-4000-8000-000000000001","candidateId":"candidate","shotId":"shot","generationId":"generation","resultId":"result","archiveJobId":"job","preparedFrozenHash":"` + strings.Repeat("a", 64) + `"}`
				}
				r := httptest.NewRequest(route.method, "http://127.0.0.1/api/hn/projects/test/sequences/other/"+route.suffix, strings.NewReader(body))
				r.RemoteAddr = "127.0.0.1:1"
				r.Header.Set("Origin", "http://localhost:3000")
				r.Header.Set("X-HN-Local-Request", "1")
				r.Header.Set("Content-Type", "application/json")
				want := 403
				switch scenario {
				case "remote":
					r.RemoteAddr = "192.0.2.1:1"
				case "host":
					r.Host = "synthetic.invalid"
				case "origin":
					r.Header.Set("Origin", "https://synthetic.invalid")
				case "marker":
					r.Header.Del("X-HN-Local-Request")
				case "cookie":
					r.Header.Set("Cookie", "synthetic=value")
				case "authorization":
					r.Header.Set("Authorization", "synthetic")
				case "local":
					want = 400
				}
				w := httptest.NewRecorder()
				engine.ServeHTTP(w, r)
				var envelope map[string]any
				if json.Unmarshal(w.Body.Bytes(), &envelope) != nil || len(envelope) != 3 || envelope["code"] != float64(1) || envelope["data"] != nil {
					t.Fatal("open response shape", w.Body.String())
				}
				if w.Code != want {
					t.Errorf("boundary must precede sequence validation: got %d want %d", w.Code, want)
				}
				if scenario == "local" && envelope["msg"] != "PLACEMENT_INPUT_INVALID" {
					t.Error(w.Body.String())
				}
				if scenario != "local" && envelope["msg"] == "PLACEMENT_INPUT_INVALID" {
					t.Error("sequence validation preceded local boundary")
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Error("boundary no-store missing")
				}
				evidence = append(evidence, map[string]any{"method": route.method, "route": route.suffix, "scenario": scenario, "status": w.Code, "message": envelope["msg"], "cacheControl": w.Header().Get("Cache-Control")})
			})
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("non-main request mutated store", err)
	}
	if logs.Len() != 0 {
		t.Fatal("boundary requests logged", logs.String())
	}
	if path := os.Getenv("HN_R28_FIX_EVIDENCE"); path != "" {
		b, err := json.MarshalIndent(map[string]any{"rows": evidence, "rootEntries": len(entries), "logsBytes": logs.Len()}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestR28AuditFixRouterOptionsPreservation(t *testing.T) {
	old := gin.DefaultWriter
	gin.DefaultWriter = &bytes.Buffer{}
	t.Cleanup(func() { gin.DefaultWriter = old })
	engine := New()
	cases := []struct {
		suffix, method, headers, remote string
		want                            int
		allow                           string
	}{
		{"items", "GET", "X-HN-Local-Request", "127.0.0.1:1", 204, "GET"},
		{"items", "POST", "Content-Type, X-HN-Local-Request", "127.0.0.1:1", 204, "POST"},
		{"items", "GET", "Authorization", "127.0.0.1:1", 403, ""},
		{"items", "GET", "X-HN-Local-Request", "192.0.2.1:1", 403, ""},
		{"placement-commands", "POST", "Content-Type, X-HN-Local-Request", "127.0.0.1:1", 204, "POST"},
		{"placement-commands", "GET", "X-HN-Local-Request", "127.0.0.1:1", 403, ""},
		{"placement-commands/intent", "GET", "X-HN-Local-Request", "127.0.0.1:1", 204, "GET"},
		{"placement-commands/intent", "POST", "X-HN-Local-Request", "127.0.0.1:1", 405, ""},
		{"placement-commands/intent", "GET", "Cookie", "127.0.0.1:1", 403, ""},
	}
	for _, c := range cases {
		r := httptest.NewRequest("OPTIONS", "http://127.0.0.1/api/hn/projects/test/sequences/main/"+c.suffix, nil)
		r.RemoteAddr = c.remote
		r.Header.Set("Origin", "http://localhost:3000")
		r.Header.Set("Access-Control-Request-Method", c.method)
		r.Header.Set("Access-Control-Request-Headers", c.headers)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Code != c.want || w.Header().Get("Access-Control-Allow-Methods") != c.allow {
			t.Fatal(c, w.Code, w.Body.String())
		}
	}
}
