package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func TestHNShotHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		status                            int
		body, peer, host, origin, content string
		noHeader, disabled                bool
	}{
		{name: "valid", status: 200}, {name: "disabled", status: 503, disabled: true},
		{name: "peer", status: 403, peer: "192.0.2.1:12"}, {name: "host", status: 403, host: "remote.invalid"},
		{name: "origin", status: 403, origin: "https://remote.invalid"}, {name: "header", status: 403, noHeader: true},
		{name: "content", status: 400, content: "text/plain"}, {name: "unknown", status: 400, body: `{"sourceNodeId":"a","apiKey":"fixture"}`},
		{name: "trailing", status: 400, body: `{"sourceNodeId":"a"} {}`}, {name: "empty", status: 400, body: `{}`},
		{name: "wrong-type", status: 400, body: `{"sourceNodeId":5}`}, {name: "url", status: 400, body: `{"sourceNodeId":"https://invalid.example/"}`},
		{name: "too-large", status: 400, body: `{"sourceNodeId":"` + strings.Repeat("a", 8<<10) + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HN_PROJECTS_ROOT", t.TempDir())
			if tc.disabled {
				t.Setenv("HN_PROJECTS_ROOT", "")
			}
			body := tc.body
			if body == "" {
				body = `{"sourceNodeId":"node-a","label":"Initial"}`
			}
			r := httptest.NewRequest("POST", "http://127.0.0.1/api/hn/projects/test/shots/ensure", strings.NewReader(body))
			r.RemoteAddr = "127.0.0.1:12"
			if tc.peer != "" {
				r.RemoteAddr = tc.peer
			}
			if tc.host != "" {
				r.Host = tc.host
			}
			r.Header.Set("Origin", "http://localhost:3016")
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			r.Header.Set("Content-Type", "application/json")
			if tc.content != "" {
				r.Header.Set("Content-Type", tc.content)
			}
			if !tc.noHeader {
				r.Header.Set("X-HN-Local-Request", "1")
			}
			w := httptest.NewRecorder()
			HNEnsureShot(w, r, "test")
			if w.Code != tc.status {
				t.Fatalf("unexpected status=%d", w.Code)
			}
			if tc.status == 200 {
				var response struct {
					Code int
					Data service.HNShot
				}
				if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code != 0 || response.Data.SourceNodeID != "node-a" || response.Data.ShotID == "" {
					t.Fatal("invalid metadata")
				}
				if strings.Contains(w.Body.String(), "relativePath") || strings.Contains(w.Body.String(), "selectedCandidate") {
					t.Fatal("unexpected response field")
				}
			}
		})
	}
}

func TestHNShotHTTPConflictAndGenerationOwnership(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	w, err := foundation.Open(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"A", "B"} {
		if _, err = w.CreateShot(label, "duplicate"); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	call := func(body, project string, prepare bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1/", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:12"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-HN-Local-Request", "1")
		out := httptest.NewRecorder()
		if prepare {
			HNPrepareGeneration(out, r, project)
		} else {
			HNEnsureShot(out, r, project)
		}
		return out
	}
	out := call(`{"sourceNodeId":"duplicate"}`, "test", false)
	if out.Code != 409 || !strings.Contains(out.Body.String(), "SHOT_IDENTITY_CONFLICT") {
		t.Fatal("conflict response", out.Code)
	}
	if call(`{"sourceNodeId":"a"}`, "../escape", false).Code != 400 {
		t.Fatal("unsafe project accepted")
	}
	s, err := service.EnsureShotForSourceNode(root, "test", "video", "A")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := service.EnsureShotForSourceNode(root, "foreign", "video", "B")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{s.ShotID, "missing", "../invalid", foreign.ShotID} {
		body := `{"nodeId":"video","shotId":"` + id + `","promptSnapshot":"synthetic","sourceBaseline":"` + service.HNGenerationSourceBaseline + `","parameters":{},"referenceBindings":[]}`
		out = call(body, "test", true)
		if id == s.ShotID {
			var response struct{ Data service.HNPreparedGeneration }
			if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &response) != nil || response.Data.ShotID != id || !response.Data.Frozen {
				t.Fatal("valid Shot preparation")
			}
		} else if out.Code != 400 {
			t.Fatal("invalid Shot accepted", out.Code)
		}
	}
}
