package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHNExportHTTPTrustAndExactEmptyCommand(t *testing.T) {
	for _, tc := range []struct {
		name, body, peer, host, origin, project, sequence string
		status                                            int
		disabled, noHeader                                bool
	}{
		{name: "disabled", disabled: true, status: 503}, {name: "peer", peer: "192.0.2.1:12", status: 403}, {name: "host", host: "remote.invalid", status: 403}, {name: "origin", origin: "https://remote.invalid", status: 403}, {name: "header", noHeader: true, status: 403},
		{name: "unknown", body: `{"extra":true}`, status: 400}, {name: "trailing", body: `{} {}`, status: 400}, {name: "null", body: `null`, status: 400}, {name: "array", body: `[]`, status: 400}, {name: "huge", body: `{"extra":"` + strings.Repeat("x", 64<<10) + `"}`, status: 400},
		{name: "project", project: "../outside", status: 400}, {name: "sequence", sequence: "CON", status: 400}, {name: "empty", status: 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HN_PROJECTS_ROOT", t.TempDir())
			if tc.disabled {
				t.Setenv("HN_PROJECTS_ROOT", "")
			}
			body := tc.body
			if body == "" {
				body = `{}`
			}
			r := httptest.NewRequest("POST", "http://127.0.0.1/", strings.NewReader(body))
			r.RemoteAddr = "127.0.0.1:12"
			if tc.peer != "" {
				r.RemoteAddr = tc.peer
			}
			if tc.host != "" {
				r.Host = tc.host
			}
			r.Header.Set("Origin", "http://localhost:3017")
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			r.Header.Set("Content-Type", "application/json")
			if !tc.noHeader {
				r.Header.Set("X-HN-Local-Request", "1")
			}
			p, s := tc.project, tc.sequence
			if p == "" {
				p = "test"
			}
			if s == "" {
				s = "main"
			}
			out := httptest.NewRecorder()
			HNExportSequence(out, r, p, s)
			if out.Code != tc.status {
				t.Fatalf("status %d expected %d", out.Code, tc.status)
			}
			if strings.Contains(out.Body.String(), "GPT_Work") || strings.Contains(out.Body.String(), "sqlite") {
				t.Fatal("filesystem detail exposed")
			}
		})
	}
}
