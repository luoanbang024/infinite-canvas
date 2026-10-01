package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/service"
)

func TestHNGenerationHTTPBoundary(t *testing.T) {
	body := `{"nodeId":"video","promptSnapshot":"synthetic","sourceBaseline":"` + service.HNGenerationSourceBaseline + `","parameters":{},"referenceBindings":[]}`
	for _, tc := range []struct {
		name                                                     string
		status                                                   int
		peer, host, origin, header, project, content, root, body string
	}{
		{name: "valid", status: 200},
		{name: "disabled", status: 503, root: "disabled"},
		{name: "remote-peer", status: 403, peer: "192.0.2.10:12"},
		{name: "remote-host", status: 403, host: "remote.example"},
		{name: "remote-origin", status: 403, origin: "https://remote.example"},
		{name: "origin-credentials", status: 403, origin: "http://user:fixture@localhost"},
		{name: "origin-path", status: 403, origin: "http://localhost/path"},
		{name: "missing-header", status: 403, header: "missing"},
		{name: "unsafe-project", status: 400, project: "../escape"},
		{name: "wrong-content", status: 400, content: "text/plain"},
		{name: "unknown-field", status: 400, body: strings.TrimSuffix(body, "}") + `,"credentialRef":"opaque"}`},
		{name: "trailing-json", status: 400, body: body + ` {}`},
		{name: "oversized", status: 400, body: `{"promptSnapshot":"` + strings.Repeat("a", 256<<10) + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HN_PROJECTS_ROOT", t.TempDir())
			if tc.root == "disabled" {
				t.Setenv("HN_PROJECTS_ROOT", "")
			}
			payload := body
			if tc.body != "" {
				payload = tc.body
			}
			r := httptest.NewRequest("POST", "http://127.0.0.1/api/hn/projects/test/generations/prepare", strings.NewReader(payload))
			r.RemoteAddr = "127.0.0.1:12"
			if tc.peer != "" {
				r.RemoteAddr = tc.peer
			}
			if tc.host != "" {
				r.Host = tc.host
			}
			r.Header.Set("Content-Type", "application/json")
			if tc.content != "" {
				r.Header.Set("Content-Type", tc.content)
			}
			r.Header.Set("X-HN-Local-Request", "1")
			if tc.header != "" {
				r.Header.Del("X-HN-Local-Request")
			}
			r.Header.Set("Origin", "http://localhost:3014")
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			project := "test"
			if tc.project != "" {
				project = tc.project
			}
			w := httptest.NewRecorder()
			HNPrepareGeneration(w, r, project)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status == 200 {
				var response struct {
					Code int
					Data service.HNPreparedGeneration
				}
				if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code != 0 || !response.Data.Frozen {
					t.Fatal("bad response")
				}
				if strings.Contains(w.Body.String(), "relativePath") {
					t.Fatal("path exposed")
				}
			}
		})
	}
}
