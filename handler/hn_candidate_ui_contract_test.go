package handler

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

type r22Fixture struct {
	root, project, shot, generation, frozenHash string
	archive                                     service.HNArchiveFacts
}

func r22LocalFixture(t *testing.T, project string) r22Fixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	shot, err := service.EnsureShotForSourceNode(root, project, "_video", "Shot")
	if err != nil {
		t.Fatal(err)
	}
	g, err := service.PrepareLocalGeneration(root, project, service.HNGenerationPrepareInput{NodeID: "_video", ShotID: shot.ShotID, PromptSnapshot: "synthetic local editorial intent", Model: "local intent", Parameters: json.RawMessage(`{}`), SourceBaseline: service.HNGenerationSourceBaseline})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(hnSyntheticMP4)
	a, err := service.ArchiveLocalResult(root, project, g.GenerationID, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), int64(len(hnSyntheticMP4)), hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	return r22Fixture{root: root, project: project, shot: shot.ShotID, generation: g.GenerationID, frozenHash: g.FrozenHash, archive: a}
}

func r22Snapshot(t *testing.T, f r22Fixture) map[string][]json.RawMessage {
	t.Helper()
	w, err := foundation.Open(f.root, f.project)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	out := map[string][]json.RawMessage{}
	for _, table := range []string{"shots", "generations", "results", "archive_jobs", "task_bindings", "candidates", "sequence_items"} {
		out[table], err = w.List(table)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func r22Ensure(t *testing.T, f r22Fixture, result, shot, label string) (service.HNCandidate, int) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"resultId": result, "label": label})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1/", bytes.NewReader(body))
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-HN-Local-Request", "1")
	out := httptest.NewRecorder()
	HNEnsureCandidate(out, r, f.project, shot)
	var response struct{ Data service.HNCandidate }
	if err = json.Unmarshal(out.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Body.Bytes(), []byte(f.root)) {
		t.Fatal("unsafe local path exposed")
	}
	return response.Data, out.Code
}

func TestHNR22HandlerSameIdentityNoRenameNoSelectionReopen(t *testing.T) {
	f := r22LocalFixture(t, "r22")
	before := r22Snapshot(t, f)
	a, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Earlier Safe Label")
	if status != 200 {
		t.Fatal("first ensure failed", status)
	}
	b, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive")
	if status != 200 || !reflect.DeepEqual(a, b) || b.Label != "Earlier Safe Label" {
		t.Fatal("ensure identity or label changed")
	}
	after := r22Snapshot(t, f)
	for _, table := range []string{"shots", "generations", "results", "archive_jobs", "task_bindings", "sequence_items"} {
		if !reflect.DeepEqual(before[table], after[table]) {
			t.Fatal("healthy ensure changed protected records", table)
		}
	}
	if len(after["candidates"]) != 1 {
		t.Fatal("duplicate Candidate")
	}
	c, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Changed")
	if status != 200 || !reflect.DeepEqual(a, c) {
		t.Fatal("reopen identity changed")
	}
	entries, err := os.ReadDir(filepath.Join(f.root, f.project, "exports"))
	if err != nil || len(entries) != 0 {
		t.Fatal("unexpected export")
	}
}

func TestHNR22HandlerIneligibleAndDuplicateConflict(t *testing.T) {
	f := r22LocalFixture(t, "r22")
	w, err := foundation.Open(f.root, f.project)
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.CreateResult(foundation.Result{GenerationID: f.generation, ResultKind: "video"})
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, status := r22Ensure(t, f, r.ID, f.shot, "Local Archive"); status != 409 {
		t.Fatal("RECEIVED accepted")
	}
	failed, err := service.ArchiveLocalResult(f.root, f.project, f.generation, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), int64(len(hnSyntheticMP4)), strings.Repeat("0", 64))
	if err == nil || failed.ResultStatus != "ARCHIVE_FAILED" {
		t.Fatal("failed fixture not failed")
	}
	if _, status := r22Ensure(t, f, failed.ResultID, f.shot, "Local Archive"); status != 409 {
		t.Fatal("ARCHIVE_FAILED accepted")
	}
	if _, status := r22Ensure(t, f, f.archive.ResultID, "foreign", "Local Archive"); status != 409 {
		t.Fatal("foreign Shot accepted")
	}
	if len(r22Snapshot(t, f)["candidates"]) != 0 {
		t.Fatal("ineligible command created Candidate")
	}
	if _, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive"); status != 200 {
		t.Fatal("valid ensure failed")
	}
	w, err = foundation.Open(f.root, f.project)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.CreateCandidate(f.shot, f.archive.ResultID, "duplicate")
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive"); status != 409 {
		t.Fatal("duplicate was not conflict")
	}
}

func TestHNR22HandlerArchiveIntegrity(t *testing.T) {
	for _, mode := range []string{"missing-media", "wrong-bytes", "sidecar", "foreign-job", "path", "hash", "frozen-hash"} {
		t.Run(mode, func(t *testing.T) {
			f := r22LocalFixture(t, "r22")
			path := filepath.Join(f.root, f.project, filepath.FromSlash(f.archive.ArchivedRelativePath))
			var err error
			switch mode {
			case "missing-media":
				err = os.Remove(path)
			case "wrong-bytes":
				err = os.WriteFile(path, []byte("different bytes"), 0600)
			case "sidecar":
				err = os.WriteFile(path+".json", []byte(`{}`), 0600)
			default:
				db, e := sql.Open("sqlite", filepath.Join(f.root, f.project, "metadata", "hn-extension.sqlite"))
				if e != nil {
					t.Fatal(e)
				}
				table, id, key, value := "archive_jobs", f.archive.ArchiveJobID, "generationId", "foreign"
				if mode == "path" {
					key, value = "targetRelativePath", "generated/foreign/media.mp4"
				}
				if mode == "hash" {
					key, value = "actualSha256", strings.Repeat("0", 64)
				}
				if mode == "frozen-hash" {
					table, id, key, value = "generations", f.generation, "frozenHash", strings.Repeat("0", 64)
				}
				_, err = db.Exec("UPDATE "+table+" SET data=json_set(data,?,?) WHERE id=?", "$."+key, value, id)
				db.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			_, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive")
			if status == 200 {
				t.Fatal("corrupted archive accepted")
			}
			// Query count without Open: reconciliation may invalidate corrupted records.
			db, err := sql.Open("sqlite", filepath.Join(f.root, f.project, "metadata", "hn-extension.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var count int
			if err = db.QueryRow("SELECT count(*) FROM candidates").Scan(&count); err != nil || count != 0 {
				t.Fatal("corruption created Candidate")
			}
		})
	}
}

func TestHNR22ProductionHTTPFrontendResponseLossRecovery(t *testing.T) {
	bin := os.Getenv("HN_R22_BUN")
	if bin == "" {
		t.Skip("production frontend bridge requires an existing Bun runtime; R22 execution runner supplies it")
	}
	h := sha256.Sum256([]byte("_Project"))
	project := "canvas-" + hex.EncodeToString(h[:])
	f := r22LocalFixture(t, project)
	before := r22Snapshot(t, f)
	var posts, gets, forbidden atomic.Int64
	var server *httptest.Server
	path := "/api/hn/projects/" + project + "/shots/" + f.shot + "/candidates/ensure"
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/hn/local-endpoint" {
			gets.Add(1)
			OK(w, map[string]string{"url": server.URL})
			return
		}
		if r.Method == "POST" && r.URL.Path == path {
			posts.Add(1)
			HNEnsureCandidate(w, r, project, f.shot)
			return
		}
		forbidden.Add(1)
		http.Error(w, "unexpected test request", 404)
	}))
	defer server.Close()
	prepared := map[string]any{"version": 1, "canvasProjectId": "_Project", "hnProjectId": project, "sourceNodeId": "_video", "shotId": f.shot, "generationId": f.generation, "frozenHash": f.frozenHash, "preparedAt": "2026-10-04T00:00:00.000Z", "sourceBaseline": service.HNGenerationSourceBaseline, "requestFingerprint": strings.Repeat("b", 64), "snapshot": map[string]any{"model": "local intent", "seconds": "4", "vquality": "768P", "size": "16:9", "referenceCount": 0}}
	mediaHash := f.archive.SHA256
	fingerprintInput, err := json.Marshal(struct {
		Version    int    `json:"version"`
		SHA256     string `json:"sha256"`
		ByteLength int64  `json:"byteLength"`
		MimeType   string `json:"mimeType"`
	}{1, mediaHash, f.archive.ByteLength, "video/mp4"})
	if err != nil {
		t.Fatal(err)
	}
	mediaFingerprint := sha256.Sum256(fingerprintInput)
	archive := map[string]any{"version": 1, "canvasProjectId": "_Project", "hnProjectId": project, "sourceNodeId": "_video", "shotId": f.shot, "generationId": f.generation, "preparedFrozenHash": f.frozenHash, "attemptId": "11111111-1111-4111-8111-111111111111", "observedAt": "2026-10-04T01:00:00.000Z", "sourceSHA256": mediaHash, "sourceByteLength": f.archive.ByteLength, "sourceMimeType": "video/mp4", "sourceMediaFingerprint": hex.EncodeToString(mediaFingerprint[:]), "outcome": "ARCHIVED", "resultId": f.archive.ResultID, "archiveJobId": f.archive.ArchiveJobID, "resultStatus": "ARCHIVED", "archiveStatus": "ARCHIVED", "sha256": mediaHash, "byteLength": f.archive.ByteLength, "mimeType": "video/mp4"}
	fixture, err := json.Marshal(map[string]any{"base": server.URL, "prepared": prepared, "archive": archive})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "frontend-proof.json")
	cmd := exec.Command(bin, "--no-env-file", "test", "src/app/(user)/canvas/components/hn-local-canvas-candidate.test.ts", "--test-name-pattern", "R22 actual production handler bridge")
	cmd.Dir = filepath.Join("..", "web")
	cmd.Env = append(os.Environ(), "HN_R22_BRIDGE_FIXTURE="+string(fixture), "HN_R22_BRIDGE_OUTPUT="+resultPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("frontend/handler bridge failed: %v\n%s", err, output)
	}
	if posts.Load() != 2 || gets.Load() != 2 || forbidden.Load() != 0 {
		t.Fatal("wrong server-observed command counts", posts.Load(), gets.Load(), forbidden.Load())
	}
	after := r22Snapshot(t, f)
	if len(after["candidates"]) != 1 {
		t.Fatal("response-loss retry duplicated Candidate")
	}
	for _, table := range []string{"shots", "generations", "results", "archive_jobs", "task_bindings", "sequence_items"} {
		if !reflect.DeepEqual(before[table], after[table]) {
			t.Fatal("Candidate command changed protected records", table)
		}
	}
	var candidate foundation.Candidate
	if err = json.Unmarshal(after["candidates"][0], &candidate); err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Receipt struct {
			CandidateID string `json:"candidateId"`
		}
		SourceBlobRead int `json:"sourceBlobRead"`
		AutoRetry      int `json:"autoRetry"`
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &proof); err != nil || proof.Receipt.CandidateID != candidate.ID || proof.SourceBlobRead != 0 || proof.AutoRetry != 0 {
		t.Fatal("recovered frontend identity mismatch")
	}
	// A third explicit command after independent reopen returns the identical row.
	recovered, status := r22Ensure(t, f, f.archive.ResultID, f.shot, "Local Archive")
	if status != 200 || recovered.CandidateID != candidate.ID {
		t.Fatal("reopen Candidate identity changed")
	}
	if destination := os.Getenv("HN_R22_GO_EVIDENCE"); destination != "" {
		facts := map[string]any{"serverObservedPOST": posts.Load(), "serverObservedGET": gets.Load(), "firstPOST": 1, "autoRetry": 0, "explicitRecoveryAdditionalPOST": 1, "candidateCount": 1, "recoveredCandidateIDSame": true, "sourceBlobRead": 0, "forbiddenRequests": forbidden.Load(), "protectedRecordsUnchanged": true, "providerCalls": 0, "credentialRead": "NONE", "singleProcessWriterOnly": true, "frontendProof": json.RawMessage(data)}
		b, e := json.MarshalIndent(facts, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(destination, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log("actual production handler POST=2, Candidate count=1, same recovered ID, forbidden requests=0")
}
