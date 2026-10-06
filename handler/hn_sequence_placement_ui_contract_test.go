package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func r26Add(t *testing.T, f r22Fixture, candidate string) (service.HNSequenceItem, int) {
	t.Helper()
	r := httptest.NewRequest("POST", "http://127.0.0.1/", strings.NewReader(`{"candidateId":"`+candidate+`"}`))
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-HN-Local-Request", "1")
	out := httptest.NewRecorder()
	HNAddSequenceItem(out, r, f.project, "main")
	var envelope struct{ Data service.HNSequenceItem }
	if json.Unmarshal(out.Body.Bytes(), &envelope) != nil {
		t.Fatal("invalid envelope")
	}
	return envelope.Data, out.Code
}
func TestHNR26AppendIsNonIdempotentSelectedPrerequisiteAndReopen(t *testing.T) {
	f := r22LocalFixture(t, "r26")
	a, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "A")
	if status != 200 {
		t.Fatal(status)
	}
	before := r22Snapshot(t, f)
	if _, status = r26Add(t, f, a.CandidateID); status != 409 {
		t.Fatal("unselected accepted")
	}
	if len(r22Snapshot(t, f)["sequence_items"]) != 0 {
		t.Fatal("unselected inserted")
	}
	if _, err := service.SelectLocalCandidate(f.root, f.project, f.shot, a.CandidateID); err != nil {
		t.Fatal(err)
	}
	selected := r22Snapshot(t, f)
	first, status := r26Add(t, f, a.CandidateID)
	if status != 200 {
		t.Fatal(status)
	}
	second, status := r26Add(t, f, a.CandidateID)
	if status != 200 || first.SequenceItemID == second.SequenceItemID || first.OrderIndex != 0 || second.OrderIndex != 1 {
		t.Fatal("append unexpectedly converged", first, second)
	}
	after := r22Snapshot(t, f)
	for _, table := range []string{"shots", "generations", "results", "archive_jobs", "task_bindings", "candidates"} {
		if !reflect.DeepEqual(selected[table], after[table]) {
			t.Fatal("Add changed protected healthy records", table)
		}
	}
	if len(after["sequence_items"]) != 2 || len(before["sequence_items"]) != 0 {
		t.Fatal("reopen row count")
	}
	h := sha256.Sum256(hnSyntheticMP4)
	bArchive, err := service.ArchiveLocalResult(f.root, f.project, f.generation, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), int64(len(hnSyntheticMP4)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	b, status := r22Ensure(t, f, bArchive.ResultID, f.shot, "B")
	if status != 200 {
		t.Fatal(status)
	}
	if _, err = service.SelectLocalCandidate(f.root, f.project, f.shot, b.CandidateID); err != nil {
		t.Fatal(err)
	}
	if _, status = r26Add(t, f, a.CandidateID); status != 409 || len(r22Snapshot(t, f)["sequence_items"]) != 2 {
		t.Fatal("stale A bypassed server B selection")
	}
	files, err := os.ReadDir(filepath.Join(f.root, f.project, "exports"))
	if err != nil || len(files) != 0 {
		t.Fatal("unexpected export")
	}
	t.Log("R26 actual Add: repeat distinct IDs, orderIndex 0/1; server B/Add A=409 zero new items; reopen PASS; protected entities unchanged")
}
func TestHNR26TwoShotsMainConcurrentSingleProcessWriter(t *testing.T) {
	f := r22LocalFixture(t, "r26")
	a, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "A")
	if status != 200 {
		t.Fatal(status)
	}
	if _, err := service.SelectLocalCandidate(f.root, f.project, f.shot, a.CandidateID); err != nil {
		t.Fatal(err)
	}
	shot, err := service.EnsureShotForSourceNode(f.root, f.project, "other_video", "Other")
	if err != nil {
		t.Fatal(err)
	}
	g, err := service.PrepareLocalGeneration(f.root, f.project, service.HNGenerationPrepareInput{NodeID: "other_video", ShotID: shot.ShotID, PromptSnapshot: "synthetic", Model: "local intent", Parameters: json.RawMessage(`{}`), SourceBaseline: service.HNGenerationSourceBaseline})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(hnSyntheticMP4)
	r, err := service.ArchiveLocalResult(f.root, f.project, g.GenerationID, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), int64(len(hnSyntheticMP4)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.EnsureCandidateForArchivedResult(f.root, f.project, shot.ShotID, r.ResultID, "B")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SelectLocalCandidate(f.root, f.project, shot.ShotID, b.CandidateID); err != nil {
		t.Fatal(err)
	}
	before := r22Snapshot(t, f)
	var wg sync.WaitGroup
	items := make(chan service.HNSequenceItem, 2)
	errs := make(chan error, 2)
	for _, id := range []string{a.CandidateID, b.CandidateID} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			item, e := service.AddLocalSequenceItem(f.root, f.project, "main", id)
			items <- item
			errs <- e
		}(id)
	}
	wg.Wait()
	close(items)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	ids := map[string]bool{}
	indices := map[int]bool{}
	for item := range items {
		ids[item.SequenceItemID] = true
		indices[item.OrderIndex] = true
	}
	if len(ids) != 2 || !indices[0] || !indices[1] {
		t.Fatal("writer ordering failed")
	}
	after := r22Snapshot(t, f)
	for table, rows := range before {
		if table != "sequence_items" && !reflect.DeepEqual(rows, after[table]) {
			t.Fatal("side effect", table)
		}
	}
	t.Log("R26 two Shots/main concurrent service writer: distinct IDs/indices, healthy protected rows unchanged; no multi-process/FIFO claim")
}
func TestHNR26ProductionHTTPFrontendCommittedLossBarrier(t *testing.T) {
	bin := os.Getenv("HN_R26_BUN")
	if bin == "" {
		t.Skip("existing Bun required; execution runner provides it")
	}
	h := sha256.Sum256([]byte("_Project"))
	project := "canvas-" + hex.EncodeToString(h[:])
	f := r22LocalFixture(t, project)
	c, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive")
	if status != 200 {
		t.Fatal(status)
	}
	if _, err := service.SelectLocalCandidate(f.root, project, f.shot, c.CandidateID); err != nil {
		t.Fatal(err)
	}
	before := r22Snapshot(t, f)
	var posts, gets, forbidden atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/hn/local-endpoint" {
			gets.Add(1)
			OK(w, map[string]string{"url": server.URL})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/api/hn/projects/"+project+"/sequences/main/items" {
			posts.Add(1)
			out := httptest.NewRecorder()
			HNAddSequenceItem(out, r, project, "main")
			if out.Code != 200 {
				t.Error("Add rejected", out.Code)
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		forbidden.Add(1)
		http.Error(w, "unexpected", 404)
	}))
	defer server.Close()
	prepared := map[string]any{"version": 1, "canvasProjectId": "_Project", "hnProjectId": project, "sourceNodeId": "_video", "shotId": f.shot, "generationId": f.generation, "frozenHash": f.frozenHash, "preparedAt": "2026-10-04T00:00:00.000Z", "sourceBaseline": service.HNGenerationSourceBaseline, "requestFingerprint": strings.Repeat("b", 64), "snapshot": map[string]any{"model": "local intent", "seconds": "4", "vquality": "768P", "size": "16:9", "referenceCount": 0}}
	fpInput, _ := json.Marshal(struct {
		Version    int    `json:"version"`
		SHA256     string `json:"sha256"`
		ByteLength int64  `json:"byteLength"`
		MimeType   string `json:"mimeType"`
	}{1, f.archive.SHA256, f.archive.ByteLength, "video/mp4"})
	fp := sha256.Sum256(fpInput)
	archive := map[string]any{"version": 1, "canvasProjectId": "_Project", "hnProjectId": project, "sourceNodeId": "_video", "shotId": f.shot, "generationId": f.generation, "preparedFrozenHash": f.frozenHash, "attemptId": "11111111-1111-4111-8111-111111111111", "observedAt": "2026-10-04T01:00:00.000Z", "sourceSHA256": f.archive.SHA256, "sourceByteLength": f.archive.ByteLength, "sourceMimeType": "video/mp4", "sourceMediaFingerprint": hex.EncodeToString(fp[:]), "outcome": "ARCHIVED", "resultId": f.archive.ResultID, "archiveJobId": f.archive.ArchiveJobID, "resultStatus": "ARCHIVED", "archiveStatus": "ARCHIVED", "sha256": f.archive.SHA256, "byteLength": f.archive.ByteLength, "mimeType": "video/mp4"}
	owner := map[string]any{"canvasProjectId": "_Project", "hnProjectId": project, "sourceNodeId": "_video", "shotId": f.shot, "generationId": f.generation, "preparedFrozenHash": f.frozenHash, "resultId": f.archive.ResultID, "archiveJobId": f.archive.ArchiveJobID, "sourceMediaFingerprint": hex.EncodeToString(fp[:]), "candidateId": c.CandidateID}
	candidate := map[string]any{}
	selection := map[string]any{}
	for k, v := range owner {
		candidate[k] = v
		selection[k] = v
	}
	candidate["version"] = 1
	candidate["availabilityStatus"] = "ARCHIVED"
	candidate["observedAt"] = "2026-10-05T00:00:00.000Z"
	selection["version"] = 1
	selection["intentId"] = "22222222-2222-4222-8222-222222222222"
	selection["observedAt"] = "2026-10-05T00:00:00.000Z"
	fixture, _ := json.Marshal(map[string]any{"base": server.URL, "prepared": prepared, "archive": archive, "candidate": candidate, "selection": selection})
	outputPath := filepath.Join(t.TempDir(), "frontend-proof.json")
	cmd := exec.Command(bin, "--no-env-file", "test", "src/app/(user)/canvas/components/hn-local-canvas-sequence-placement.test.ts", "--test-name-pattern", "R26 actual production handler bridge")
	cmd.Dir = filepath.Join("..", "web")
	cmd.Env = append(os.Environ(), "HN_R26_BRIDGE_FIXTURE="+string(fixture), "HN_R26_BRIDGE_OUTPUT="+outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge failed: %v\n%s", err, output)
	}
	after := r22Snapshot(t, f)
	if posts.Load() != 1 || gets.Load() != 1 || forbidden.Load() != 0 || len(after["sequence_items"]) != 1 {
		t.Fatal("network/row count", posts.Load(), gets.Load(), forbidden.Load(), len(after["sequence_items"]))
	}
	for table, rows := range before {
		if table != "sequence_items" && !reflect.DeepEqual(rows, after[table]) {
			t.Fatal("unexpected side effect", table)
		}
	}
	var item foundation.SequenceItem
	if json.Unmarshal(after["sequence_items"][0], &item) != nil || item.CandidateID != c.CandidateID {
		t.Fatal("item owner")
	}
	proof, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if destination := os.Getenv("HN_R26_GO_EVIDENCE"); destination != "" {
		data, _ := json.MarshalIndent(map[string]any{"serverObservedPOST": posts.Load(), "serverObservedGET": gets.Load(), "itemCount": 1, "additionalReloadRetryPOST": 0, "forbidden": 0, "protectedEntitiesUnchanged": true, "frontendProof": json.RawMessage(proof), "providerCalls": 0}, "", "  ")
		if err = os.WriteFile(destination, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("R26 ACTUAL_PRODUCTION_COMMITTED_LOSS: POST=1/item=1; UNKNOWN/reload additional POST=0; protected rows unchanged")
}
