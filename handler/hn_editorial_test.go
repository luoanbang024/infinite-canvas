package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/service"
)

func TestHNEditorialHTTPTrustAndCommandShapes(t *testing.T) {
	for _, operation := range []string{"ensure", "select", "add", "reorder"} {
		for _, tc := range []struct {
			name, body, peer, host, origin, content string
			status                                  int
			disabled, noHeader                      bool
		}{
			{name: "disabled", status: 503, disabled: true}, {name: "peer", peer: "192.0.2.1:12", status: 403}, {name: "host", host: "remote.invalid", status: 403}, {name: "origin", origin: "https://remote.invalid", status: 403}, {name: "header", noHeader: true, status: 403},
			{name: "unknown", body: `{"extra":true}`, status: 400}, {name: "trailing", body: `{} {}`, status: 400}, {name: "null", body: `null`, status: 400}, {name: "array", body: `[]`, status: 400}, {name: "content", content: "text/plain", status: 400}, {name: "too-large", body: `{"label":"` + strings.Repeat("x", 64<<10) + `"}`, status: 400},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
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
				if tc.content != "" {
					r.Header.Set("Content-Type", tc.content)
				}
				if !tc.noHeader {
					r.Header.Set("X-HN-Local-Request", "1")
				}
				out := httptest.NewRecorder()
				switch operation {
				case "ensure":
					HNEnsureCandidate(out, r, "test", "shot")
				case "select":
					HNSelectCandidate(out, r, "test", "shot", "candidate")
				case "add":
					HNAddSequenceItem(out, r, "test", "main")
				case "reorder":
					HNReorderSequence(out, r, "test", "main")
				}
				if out.Code != tc.status {
					t.Fatalf("status %d wanted %d", out.Code, tc.status)
				}
			})
		}
	}
}

func TestHNEditorialHTTPExplicitProductionPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	s, err := service.EnsureShotForSourceNode(root, "test", "node", "Shot")
	if err != nil {
		t.Fatal(err)
	}
	g, err := service.PrepareLocalGeneration(root, "test", service.HNGenerationPrepareInput{NodeID: "node", ShotID: s.ShotID, PromptSnapshot: "synthetic", Parameters: json.RawMessage(`{}`), SourceBaseline: service.HNGenerationSourceBaseline})
	if err != nil {
		t.Fatal(err)
	}
	b := []byte("synthetic local video")
	h := sha256.Sum256(b)
	res, err := service.ArchiveLocalResult(root, "test", g.GenerationID, "", "video/mp4", bytes.NewReader(b), int64(len(b)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	call := func(operation, body, project, shot, candidate, sequence string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1/", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:12"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-HN-Local-Request", "1")
		out := httptest.NewRecorder()
		switch operation {
		case "ensure":
			HNEnsureCandidate(out, r, project, shot)
		case "select":
			HNSelectCandidate(out, r, project, shot, candidate)
		case "add":
			HNAddSequenceItem(out, r, project, sequence)
		case "reorder":
			HNReorderSequence(out, r, project, sequence)
		}
		return out
	}
	out := call("ensure", `{"resultId":"`+res.ResultID+`"}`, "test", s.ShotID, "", "")
	var candidate struct{ Data service.HNCandidate }
	if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &candidate) != nil {
		t.Fatal("ensure", out.Code)
	}
	body := `{"candidateId":"` + candidate.Data.CandidateID + `"}`
	if call("add", body, "test", "", "", "main").Code != 409 {
		t.Fatal("add before select")
	}
	if call("select", `{}`, "test", s.ShotID, candidate.Data.CandidateID, "").Code != 200 {
		t.Fatal("select")
	}
	for _, op := range []string{"ensure", "select", "add"} {
		if call(op, `{}`, "../unsafe", s.ShotID, candidate.Data.CandidateID, "main").Code != 400 {
			t.Fatal("unsafe project")
		}
	}
	if call("select", `{"selected":true}`, "test", s.ShotID, candidate.Data.CandidateID, "").Code != 400 {
		t.Fatal("select nonempty shape")
	}
	out = call("add", body, "test", "", "", "main")
	var item struct{ Data service.HNSequenceItem }
	if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &item) != nil || item.Data.CandidateID != candidate.Data.CandidateID || item.Data.ResultID != res.ResultID {
		t.Fatal("add ownership", out.Code)
	}
	for _, body := range []string{`{}`, `{"sequenceItemIds":null}`, `{"sequenceItemIds":["../bad"]}`, `{"sequenceItemIds":["` + item.Data.SequenceItemID + `","` + item.Data.SequenceItemID + `"]}`} {
		if call("reorder", body, "test", "", "", "main").Code != 400 {
			t.Fatal("malformed reorder accepted")
		}
	}
	if call("reorder", `{"sequenceItemIds":[]}`, "test", "", "", "main").Code != 409 {
		t.Fatal("missing item accepted")
	}
	out = call("reorder", `{"sequenceItemIds":["`+item.Data.SequenceItemID+`"]}`, "test", "", "", "main")
	var ordered struct{ Data []service.HNSequenceItem }
	if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &ordered) != nil || len(ordered.Data) != 1 || ordered.Data[0] != item.Data {
		t.Fatal("reorder changed fields")
	}
}
