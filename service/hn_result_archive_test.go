package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

func archiveHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func archiveGeneration(t *testing.T, root, project string, frozen bool) foundation.Generation {
	t.Helper()
	w, e := foundation.Open(root, project)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	g, e := w.CreateGeneration(foundation.Generation{PromptSnapshot: "synthetic archive owner", SourceBaseline: HNGenerationSourceBaseline})
	if e != nil {
		t.Fatal(e)
	}
	if frozen {
		g, e = w.FreezeGeneration(g.ID)
		if e != nil {
			t.Fatal(e)
		}
	}
	return g
}

type interruptedArchiveReader struct{}

func (interruptedArchiveReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestHNLocalArchiveOwnership(t *testing.T) {
	root := t.TempDir()
	g := archiveGeneration(t, root, "local", true)
	draft := archiveGeneration(t, root, "local", false)
	foreign := archiveGeneration(t, root, "foreign", true)
	data := []byte("synthetic local video")
	for _, id := range []string{"", "missing", draft.ID, foreign.ID} {
		if _, e := ArchiveLocalResult(root, "local", id, "", "video/mp4", bytes.NewReader(data), int64(len(data)), archiveHash(data)); !errors.Is(e, ErrHNArchiveInput) {
			t.Fatalf("generation %q accepted: %v", id, e)
		}
	}
	if _, e := ArchiveLocalResult(root, "../escape", g.ID, "", "video/mp4", bytes.NewReader(data), int64(len(data)), archiveHash(data)); e == nil {
		t.Fatal("unsafe project")
	}
	for _, mime := range []string{"audio/mpeg", "image/png", "video/unknown"} {
		if _, e := ArchiveLocalResult(root, "local", g.ID, "", mime, bytes.NewReader(data), int64(len(data)), archiveHash(data)); e == nil {
			t.Fatal("unsupported MIME")
		}
	}
	for _, size := range []int64{0, HNArchiveMaxBytes + 1} {
		if _, e := ArchiveLocalResult(root, "local", g.ID, "", "video/mp4", bytes.NewReader(data), size, archiveHash(data)); e == nil {
			t.Fatal("invalid size")
		}
	}
	if _, e := ArchiveLocalResult(root, "local", g.ID, "", "video/mp4", bytes.NewReader(data), int64(len(data)), "invalid"); e == nil {
		t.Fatal("invalid hash")
	}
}

func TestHNLocalArchiveRetryReopenReceipt(t *testing.T) {
	root := t.TempDir()
	g := archiveGeneration(t, root, "local", true)
	data := []byte("synthetic local video bytes")
	hash := archiveHash(data)
	w, e := foundation.Open(root, "local")
	if e != nil {
		t.Fatal(e)
	}
	before, e := w.Read("generations", g.ID)
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	facts, e := ArchiveLocalResult(root, "local", g.ID, "", "video/mp4", io.MultiReader(bytes.NewReader(data[:4]), interruptedArchiveReader{}), int64(len(data)), hash)
	if !errors.Is(e, ErrHNArchiveFailed) || facts.ArchiveStatus != "FAILED" || facts.ResultStatus != "ARCHIVE_FAILED" || facts.ArchiveJobID == "" {
		t.Fatalf("lost failure history: %+v %v", facts, e)
	}
	failedID, resultID := facts.ArchiveJobID, facts.ResultID
	// Same failed Job, no Result/Generation creation. Wrong actual bytes must fail.
	if _, e = ArchiveLocalResult(root, "local", "", failedID, "video/mp4", bytes.NewReader([]byte("wrong")), int64(len(data)), hash); !errors.Is(e, ErrHNArchiveFailed) {
		t.Fatal("mismatched bytes accepted")
	}
	facts, e = ArchiveLocalResult(root, "local", "", failedID, "video/mp4", bytes.NewReader(data), int64(len(data)), hash)
	if e != nil || facts.ArchiveJobID != failedID || facts.ResultID != resultID || facts.ResultStatus != "ARCHIVED" || facts.ArchiveStatus != "ARCHIVED" || facts.SHA256 != hash || facts.ByteLength != int64(len(data)) {
		t.Fatalf("retry failed %+v %v", facts, e)
	}
	path := filepath.Join(root, "local", filepath.FromSlash(facts.ArchivedRelativePath))
	fileBefore, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	if facts.ArchivedRelativePath != "generated/"+g.ID+"/"+resultID+"/media.mp4" {
		t.Fatal("unsafe target")
	}
	if _, e = ArchiveLocalResult(root, "local", "", failedID, "video/mp4", interruptedArchiveReader{}, int64(len(data)), hash); e != nil {
		t.Fatal("verified file was recopied", e)
	}
	fileAfter, e := os.Stat(path)
	if e != nil || !fileBefore.ModTime().Equal(fileAfter.ModTime()) {
		t.Fatal("duplicate byte write")
	}
	for _, bad := range []struct {
		n int64
		h string
	}{{int64(len(data) + 1), hash}, {int64(len(data)), archiveHash([]byte("wrong"))}} {
		if _, e = ArchiveLocalResult(root, "local", "", failedID, "video/mp4", bytes.NewReader(data), bad.n, bad.h); !errors.Is(e, ErrHNArchiveFailed) {
			t.Fatal("retry expectation mismatch accepted")
		}
	}
	for _, pair := range [][2]string{{"local", "missing"}, {"foreign", failedID}} {
		if _, e = ArchiveLocalResult(root, pair[0], "", pair[1], "video/mp4", bytes.NewReader(data), int64(len(data)), hash); !errors.Is(e, ErrHNArchiveInput) {
			t.Fatal("foreign/unknown job accepted")
		}
	}
	w, e = foundation.Open(root, "local")
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	for kind, count := range map[string]int{"generations": 1, "results": 1, "archive_jobs": 1, "task_bindings": 0} {
		records, e := w.List(kind)
		if e != nil || len(records) != count {
			t.Fatal("retry entity count", kind, e)
		}
	}
	after, e := w.Read("generations", g.ID)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("Generation mutated")
	}
	var result foundation.Result
	var job foundation.ArchiveJob
	if hnReadRecord(w, "results", resultID, &result) != nil || hnReadRecord(w, "archive_jobs", failedID, &job) != nil {
		t.Fatal("reopen read")
	}
	if result.Status != "ARCHIVED" || job.Status != "ARCHIVED" || result.TaskBindingID != "" || result.ProviderResultID != "" || result.SourceURLRef != "" {
		t.Fatal("provenance/state")
	}
	actual, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(actual, data) {
		t.Fatal("file bytes")
	}
	raw, e := os.ReadFile(path + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var receipt struct {
		ID, ProjectID, RelativePath, SHA256 string
		ByteLength                          int64
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.ID != job.ID || receipt.ProjectID != "local" || receipt.SHA256 != hash || receipt.ByteLength != int64(len(data)) {
		t.Fatal("receipt mismatch", string(raw))
	}
}
