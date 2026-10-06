package router

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestR28RouterReadPreflightAndLogClosure(t *testing.T) {
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
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("non-main read allowed")
	}
}
