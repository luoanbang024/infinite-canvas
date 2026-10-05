package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func r24Shot(t *testing.T, f r22Fixture) foundation.Shot {
	t.Helper()
	w, err := foundation.Open(f.root, f.project)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	raw, err := w.Read("shots", f.shot)
	if err != nil {
		t.Fatal(err)
	}
	var s foundation.Shot
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func r24SelectRecorder(t *testing.T, f r22Fixture, shot, candidate, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/", strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-HN-Local-Request", "1")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	out := httptest.NewRecorder()
	HNSelectCandidate(out, r, f.project, shot, candidate)
	if bytes.Contains(out.Body.Bytes(), []byte(f.root)) {
		t.Fatal("local path disclosure")
	}
	return out
}

func TestHNR24SelectionMutationRepeatOverwriteAndResponseLoss(t *testing.T) {
	f := r22LocalFixture(t, "r24")
	a, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "A")
	if status != 200 {
		t.Fatal("fixture candidate A failed")
	}
	h := sha256.Sum256(hnSyntheticMP4)
	bArchive, err := service.ArchiveLocalResult(f.root, f.project, f.generation, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), int64(len(hnSyntheticMP4)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	b, status := r22Ensure(t, f, bArchive.ResultID, f.shot, "B")
	if status != 200 || a.CandidateID == b.CandidateID {
		t.Fatal("fixture candidate B failed")
	}
	before := r22Snapshot(t, f)
	var posts atomic.Int32
	var drop atomic.Bool
	drop.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Error("non-POST selection")
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 10 || parts[1] != "api" || parts[2] != "hn" || parts[3] != "projects" || parts[4] != f.project || parts[5] != "shots" || parts[6] != f.shot || parts[7] != "candidates" || parts[9] != "select" {
			t.Error("unexpected local route")
			w.WriteHeader(400)
			return
		}
		posts.Add(1)
		if drop.Load() {
			out := httptest.NewRecorder()
			HNSelectCandidate(out, r, f.project, f.shot, parts[8])
			if out.Code != 200 {
				t.Error("committed-loss fixture rejected")
			}
			conn, _, e := w.(http.Hijacker).Hijack()
			if e != nil {
				t.Error(e)
				return
			}
			_ = conn.Close()
			return
		}
		HNSelectCandidate(w, r, f.project, f.shot, parts[8])
	}))
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(id string, loss bool) {
		t.Helper()
		r, e := http.NewRequest(http.MethodPost, server.URL+"/api/hn/projects/"+f.project+"/shots/"+f.shot+"/candidates/"+id+"/select", strings.NewReader("{}"))
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-HN-Local-Request", "1")
		response, e := client.Do(r)
		if loss {
			if e == nil {
				response.Body.Close()
				t.Fatal("expected lost response")
			}
			return
		}
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		var envelope map[string]json.RawMessage
		if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&envelope) != nil || len(envelope) != 3 {
			t.Fatal("invalid selection envelope")
		}
		var selection map[string]string
		if json.Unmarshal(envelope["data"], &selection) != nil || len(selection) != 2 || selection["shotId"] != f.shot || selection["selectedCandidateId"] != id || string(envelope["code"]) != "0" || string(envelope["msg"]) != `"ok"` {
			t.Fatal("unsafe or mismatched selection response")
		}
	}
	call(a.CandidateID, true)
	first := r24Shot(t, f)
	if first.SelectedCandidateID != a.CandidateID || posts.Load() != 1 {
		t.Fatal("first select did not commit exactly once")
	}
	time.Sleep(10 * time.Millisecond)
	if posts.Load() != 1 {
		t.Fatal("lost response was automatically retried")
	}
	drop.Store(false)
	deadline := time.Now().Add(time.Second)
	for time.Now().UTC().Format(time.RFC3339Nano) == first.UpdatedAt && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	call(a.CandidateID, false)
	repeated := r24Shot(t, f)
	if repeated.UpdatedAt == first.UpdatedAt || repeated.SelectedCandidateID != a.CandidateID {
		t.Fatal("same identity repeat did not rewrite timestamp")
	}
	copyRepeat := repeated
	copyRepeat.UpdatedAt = first.UpdatedAt
	if !reflect.DeepEqual(first, copyRepeat) {
		t.Fatal("repeat modified other Shot fields")
	}
	call(b.CandidateID, false)
	if r24Shot(t, f).SelectedCandidateID != b.CandidateID {
		t.Fatal("B did not overwrite A")
	}
	call(a.CandidateID, false)
	if r24Shot(t, f).SelectedCandidateID != a.CandidateID || posts.Load() != 4 {
		t.Fatal("later A cannot overwrite B or extra transport call")
	}
	after := r22Snapshot(t, f)
	for table, raw := range before {
		if table != "shots" && !reflect.DeepEqual(raw, after[table]) {
			t.Fatal("Select changed protected records", table)
		}
	}
	original := before["shots"][0]
	var initial foundation.Shot
	if err = json.Unmarshal(original, &initial); err != nil {
		t.Fatal(err)
	}
	final := r24Shot(t, f)
	final.UpdatedAt = initial.UpdatedAt
	final.SelectedCandidateID = initial.SelectedCandidateID
	if !reflect.DeepEqual(initial, final) {
		t.Fatal("other Shot fields changed")
	}
	entries, err := os.ReadDir(filepath.Join(f.root, f.project, "exports"))
	if err != nil || len(entries) != 0 {
		t.Fatal("selection created export")
	}
	evidence := map[string]any{"scenario": "actual-production-select-handler", "firstCommittedLostResponsePOST": 1, "automaticAdditionalPOST": 0, "explicitSameTargetReselectPOST": 1, "totalServerObservedPOST": posts.Load(), "sameCandidateIdentity": "CONVERGENT", "byteIdempotent": "NO", "updatedAtRewritten": first.UpdatedAt != repeated.UpdatedAt, "allOtherShotFieldsUnchanged": true, "overwrite": "A->B->A", "generationResultJobCandidateSequenceChange": "NONE", "providerBoundCalls": 0, "realProviderCalls": 0}
	if path := os.Getenv("HN_R24_GO_EVIDENCE"); path != "" {
		raw, e := json.MarshalIndent(evidence, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log("R24_SELECT_PRODUCTION_CONTRACT_PASS same-A identity convergent / timestamp rewrite / A-B-A / loss no auto resend / POST=4 / protected entity inventories unchanged")
}

func TestHNR24SelectionKnownOwnershipAndInputRejections(t *testing.T) {
	f := r22LocalFixture(t, "r24-reject")
	a, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "A")
	if status != 200 {
		t.Fatal("fixture candidate failed")
	}
	for _, x := range []struct {
		shot, candidate, body string
		headers               map[string]string
		status                int
	}{
		{f.shot, a.CandidateID, "{}", nil, 200}, {"../unsafe", a.CandidateID, "{}", nil, 400}, {f.shot, "missing", "{}", nil, 409}, {"different-shot", a.CandidateID, "{}", nil, 409},
		{f.shot, a.CandidateID, `{"intentId":"client-only"}`, nil, 400}, {f.shot, a.CandidateID, "[]", nil, 400}, {f.shot, a.CandidateID, "{}", map[string]string{"X-HN-Local-Request": ""}, 403}, {f.shot, a.CandidateID, "{}", map[string]string{"Origin": "https://remote.invalid"}, 403},
	} {
		out := r24SelectRecorder(t, f, x.shot, x.candidate, x.body, x.headers)
		if out.Code != x.status {
			t.Fatalf("selection rejection want %d got %d", x.status, out.Code)
		}
	}
	// Actual different-Shot Candidate, not just an absent requested Shot.
	s, err := service.EnsureShotForSourceNode(f.root, f.project, "_other", "Other")
	if err != nil {
		t.Fatal(err)
	}
	g, err := service.PrepareLocalGeneration(f.root, f.project, service.HNGenerationPrepareInput{NodeID: "_other", ShotID: s.ShotID, PromptSnapshot: "synthetic local", Model: "local intent", Parameters: json.RawMessage(`{}`), SourceBaseline: service.HNGenerationSourceBaseline})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(hnSyntheticMP4)
	r, err := service.ArchiveLocalResult(f.root, f.project, g.GenerationID, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), int64(len(hnSyntheticMP4)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	c, err := service.EnsureCandidateForArchivedResult(f.root, f.project, s.ShotID, r.ResultID, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if out := r24SelectRecorder(t, f, f.shot, c.CandidateID, "{}", nil); out.Code != 409 {
		t.Fatal("different-Shot Candidate accepted")
	}
	// Corrupting only an isolated fixture's media makes Open revoke archive eligibility.
	if err = os.WriteFile(filepath.Join(f.root, f.project, filepath.FromSlash(f.archive.ArchivedRelativePath)), []byte("synthetic corrupted bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if out := r24SelectRecorder(t, f, f.shot, a.CandidateID, "{}", nil); out.Code != 409 {
		t.Fatal("non-archived chain accepted", out.Code)
	}
	t.Setenv("HN_PROJECTS_ROOT", "")
	if out := r24SelectRecorder(t, f, f.shot, a.CandidateID, "{}", nil); out.Code != 503 {
		t.Fatal("unconfigured backend not 503")
	}
}
