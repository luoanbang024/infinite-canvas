package handler

import (
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestR33HandlerStrictBoundaryBeforeSequenceAndClosedBody(t *testing.T) {
	t.Setenv("HN_PROJECTS_ROOT", t.TempDir())
	intent := uuid.NewString()
	routes := []struct {
		name string
		read bool
		call func(http.ResponseWriter, *http.Request, string)
	}{
		{"initialize", false, func(w http.ResponseWriter, r *http.Request, s string) { HNInitializeExportProtocol(w, r, "test", s) }},
		{"command", false, func(w http.ResponseWriter, r *http.Request, s string) { HNExportCommand(w, r, "test", s) }},
		{"lookup", true, func(w http.ResponseWriter, r *http.Request, s string) { HNLookupExportCommand(w, r, "test", s, intent) }},
		{"verify", true, func(w http.ResponseWriter, r *http.Request, s string) { HNVerifyExportBundle(w, r, "test", s, intent) }}}
	for _, route := range routes {
		for _, kind := range []string{"peer", "host", "origin", "marker", "authorization", "cookie", "query", "get-body", "valid-other"} {
			t.Run(route.name+"-"+kind, func(t *testing.T) {
				method := "POST"
				body := `{"protocolVersion":1}`
				if route.read {
					method = "GET"
					body = ""
				}
				r := httptest.NewRequest(method, "http://127.0.0.1/api/hn/", strings.NewReader(body))
				r.RemoteAddr = "127.0.0.1:7"
				r.Header.Set("Origin", "http://localhost:3017")
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-HN-Local-Request", "1")
				status := 403
				switch kind {
				case "peer":
					r.RemoteAddr = "192.0.2.2:7"
				case "host":
					r.Host = "remote.invalid"
				case "origin":
					r.Header.Set("Origin", "https://remote.invalid")
				case "marker":
					r.Header.Del("X-HN-Local-Request")
				case "authorization":
					r.Header.Set("Authorization", "synthetic")
				case "cookie":
					r.Header.Set("Cookie", "synthetic=value")
				case "query":
					r.URL.RawQuery = "untrusted=synthetic"
					status = 400
				case "get-body":
					if !route.read {
						return
					}
					r.Body = http.NoBody
					r.Body = io.NopCloser(strings.NewReader("x"))
					status = 400
				case "valid-other":
					status = 400
				}
				out := httptest.NewRecorder()
				route.call(out, r, "other")
				if out.Code != status || out.Header().Get("Cache-Control") != "no-store" {
					t.Fatal(kind, out.Code, out.Body.String())
				}
				if kind == "valid-other" && !strings.Contains(out.Body.String(), "EXPORT_INPUT_INVALID") {
					t.Fatal(out.Body.String())
				}
			})
		}
	}
	valid := `{"protocolVersion":1,"exportIntentId":"` + intent + `","expectedRevision":"0","orderedSequenceItemIds":[],"format":"hn-offline-bundle-v1"}`
	for _, body := range []string{`null`, `[]`, `{}`, valid + ` {}`, strings.Replace(valid, `"0"`, `0`, 1), strings.Replace(valid, `"protocolVersion":1`, `"protocolVersion":1,"protocolVersion":1`, 1), strings.Replace(valid, `"format":`, `"unknown":1,"format":`, 1), strings.Replace(valid, `[]`, `null`, 1), `{"x":"` + strings.Repeat("x", 65_536) + `"}`} {
		r := httptest.NewRequest("POST", "http://127.0.0.1/", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:7"
		r.Header.Set("X-HN-Local-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		HNExportCommand(out, r, "test", "main")
		if out.Code != 400 {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1/", strings.NewReader(valid))
	r.RemoteAddr = "127.0.0.1:7"
	r.Header.Set("X-HN-Local-Request", "1")
	r.Header.Set("Content-Type", "text/plain")
	out := httptest.NewRecorder()
	HNExportCommand(out, r, "test", "main")
	if out.Code != 400 {
		t.Fatal(out.Code)
	}
}
