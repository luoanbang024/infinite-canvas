package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func r20Post(t *testing.T, root, generation, retry, mime, hash string) (service.HNArchiveFacts, int) {
	t.Helper()
	t.Setenv("HN_PROJECTS_ROOT", root)
	var body bytes.Buffer
	f := multipart.NewWriter(&body)
	for k, v := range map[string]string{"resultKind": "video", "mimeType": mime, "sha256": hash} {
		if e := f.WriteField(k, v); e != nil {
			t.Fatal(e)
		}
	}
	file, e := f.CreateFormFile("file", "ignored.bin")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = file.Write(hnSyntheticMP4); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1/", &body)
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("Content-Type", f.FormDataContentType())
	r.Header.Set("X-HN-Local-Request", "1")
	out := httptest.NewRecorder()
	HNLocalResultArchive(out, r, "r20", generation, retry)
	var envelope struct{ Data service.HNArchiveFacts }
	if e = json.Unmarshal(out.Body.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(out.Body.Bytes(), []byte(root)) {
		t.Fatal("unsafe path in response")
	}
	return envelope.Data, out.Code
}
func r20Counts(t *testing.T, root string) map[string]int {
	t.Helper()
	w, e := foundation.Open(root, "r20")
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	counts := map[string]int{}
	for _, kind := range []string{"generations", "shots", "results", "archive_jobs", "task_bindings", "candidates", "sequence_items"} {
		rows, e := w.List(kind)
		if e != nil {
			t.Fatal(e)
		}
		counts[kind] = len(rows)
	}
	return counts
}
func TestHNR20LocalArchiveUIContract(t *testing.T) {
	root := t.TempDir()
	shot, e := service.EnsureShotForSourceNode(root, "r20", "video", "Shot")
	if e != nil {
		t.Fatal(e)
	}
	g, e := service.PrepareLocalGeneration(root, "r20", service.HNGenerationPrepareInput{NodeID: "video", ShotID: shot.ShotID, PromptSnapshot: "synthetic local attachment", Parameters: json.RawMessage(`{}`), SourceBaseline: service.HNGenerationSourceBaseline})
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(hnSyntheticMP4)
	hash := hex.EncodeToString(h[:])
	for _, size := range []int64{0, service.HNArchiveMaxBytes + 1} {
		_, e := service.ArchiveLocalResult(root, "r20", g.GenerationID, "", "video/mp4", bytes.NewReader(hnSyntheticMP4), size, hash)
		if e != service.ErrHNArchiveInput {
			t.Fatal("invalid size accepted", size)
		}
		counts := r20Counts(t, root)
		if counts["results"] != 0 || counts["archive_jobs"] != 0 {
			t.Fatal("invalid size created records")
		}
	}
	for _, bad := range []struct{ gen, mime string }{{"missing", "video/mp4"}, {g.GenerationID, "audio/mpeg"}} {
		_, status := r20Post(t, root, bad.gen, "", bad.mime, hash)
		if status != 400 {
			t.Fatal("input accepted", status)
		}
		counts := r20Counts(t, root)
		if counts["results"] != 0 || counts["archive_jobs"] != 0 {
			t.Fatal("invalid input created records")
		}
	}
	a, status := r20Post(t, root, g.GenerationID, "", "video/mp4", hash)
	if status != 200 {
		t.Fatal("archive A failed", status)
	}
	b, status := r20Post(t, root, g.GenerationID, "", "video/mp4", hash)
	if status != 200 || a.ResultID == b.ResultID || a.ArchiveJobID == b.ArchiveJobID {
		t.Fatal("fresh incorrectly deduplicated")
	}
	failed, status := r20Post(t, root, g.GenerationID, "", "video/mp4", "0000000000000000000000000000000000000000000000000000000000000000")
	if status != 422 || failed.ResultStatus != "ARCHIVE_FAILED" || failed.ArchiveStatus != "FAILED" {
		t.Fatal("missing failed IDs")
	}
	retried, status := r20Post(t, root, "", failed.ArchiveJobID, "video/mp4", hash)
	if status != 200 || retried.ResultID != failed.ResultID || retried.GenerationID != g.GenerationID || retried.ArchiveJobID != failed.ArchiveJobID {
		t.Fatal("retry changed owner")
	}
	counts := r20Counts(t, root)
	if counts["results"] != 3 || counts["archive_jobs"] != 3 || counts["generations"] != 1 || counts["shots"] != 1 || counts["task_bindings"] != 0 || counts["candidates"] != 0 || counts["sequence_items"] != 0 {
		t.Fatal("unexpected creation", counts)
	}
	w, e := foundation.Open(root, "r20")
	if e != nil {
		t.Fatal(e)
	}
	for _, result := range []service.HNArchiveFacts{a, b, retried} {
		raw, e := w.Read("results", result.ResultID)
		if e != nil {
			t.Fatal(e)
		}
		var r foundation.Result
		if json.Unmarshal(raw, &r) != nil || r.TaskBindingID != "" || r.ProviderResultID != "" || r.SourceURLRef != "" || r.Status != "ARCHIVED" || r.SHA256 != hash || r.ByteLength != int64(len(hnSyntheticMP4)) {
			t.Fatal("provenance/bytes")
		}
		raw, e = os.ReadFile(filepath.Join(w.Root, filepath.FromSlash(r.ArchivedRelativePath)))
		if e != nil || !bytes.Equal(raw, hnSyntheticMP4) {
			t.Fatal("archive bytes")
		}
		if _, e = os.Stat(filepath.Join(w.Root, filepath.FromSlash(r.ArchivedRelativePath+".json"))); e != nil {
			t.Fatal("sidecar")
		}
	}
	w.Close()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	child := exec.Command(exe, "-test.run=^TestHNR20LocalArchiveReopenChild$")
	child.Env = append(os.Environ(), "HN_R20_CHILD_ROOT="+root)
	if output, e := child.CombinedOutput(); e != nil {
		t.Fatalf("process reopen %v %s", e, output)
	}
	if path := os.Getenv("HN_R20_GO_EVIDENCE"); path != "" {
		data, _ := json.MarshalIndent(map[string]any{"freshA": a.ResultID, "freshB": b.ResultID, "jobA": a.ArchiveJobID, "jobB": b.ArchiveJobID, "retryResult": retried.ResultID, "retryJob": retried.ArchiveJobID, "sameJob": true, "hash": hash, "bytes": len(hnSyntheticMP4), "counts": counts, "processReopen": "PASS", "providerCalls": 0}, "", "  ")
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func TestHNR20LocalArchiveReopenChild(t *testing.T) {
	root := os.Getenv("HN_R20_CHILD_ROOT")
	if root == "" {
		t.Skip("child-only")
	}
	counts := r20Counts(t, root)
	if counts["results"] != 3 || counts["archive_jobs"] != 3 {
		t.Fatal("reopen records", counts)
	}
}
