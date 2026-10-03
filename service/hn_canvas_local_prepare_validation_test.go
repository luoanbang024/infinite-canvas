package service_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tigerowo/infinite-canvas/handler"
	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/service"
)

func TestHNCanvasLocalPrepareCompositionAndProcessReopen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HN_PROJECTS_ROOT", root)
	idHash := sha256.Sum256([]byte("_CaseSensitiveProject"))
	project := "canvas-" + hex.EncodeToString(idHash[:])
	var shotPosts, referencePosts, preparePosts, forbidden atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Method != http.MethodPost {
			forbidden.Add(1)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/hn/projects/" + project + "/shots/ensure":
			shotPosts.Add(1)
			handler.HNEnsureShot(w, r, project)
		case "/api/hn/projects/" + project + "/references":
			referencePosts.Add(1)
			handler.HNFreezeReference(w, r, project)
		case "/api/hn/projects/" + project + "/generations/prepare":
			preparePosts.Add(1)
			handler.HNPrepareGeneration(w, r, project)
		default:
			forbidden.Add(1)
			http.Error(w, "forbidden", http.StatusNotFound)
		}
	}))
	defer server.Close()
	post := func(path, contentType string, body io.Reader, target any) {
		t.Helper()
		r, err := http.NewRequest(http.MethodPost, server.URL+"/api/hn/projects/"+project+path, body)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", contentType)
		r.Header.Set("X-HN-Local-Request", "1")
		response, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if json.NewDecoder(response.Body).Decode(&envelope) != nil || response.StatusCode != 200 || envelope.Code != 0 || json.Unmarshal(envelope.Data, target) != nil {
			t.Fatal("local response invalid")
		}
	}
	ensure := func() service.HNShot {
		var shot service.HNShot
		post("/shots/ensure", "application/json", strings.NewReader(`{"sourceNodeId":"_video-intent","label":"Shot"}`), &shot)
		return shot
	}
	shotA := ensure()
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aBVkAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "local-reference.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(png); err != nil {
		t.Fatal(err)
	}
	if err = form.WriteField("kind", "image"); err != nil {
		t.Fatal(err)
	}
	if err = form.WriteField("logicalReferenceId", "local-image"); err != nil {
		t.Fatal(err)
	}
	if err = form.Close(); err != nil {
		t.Fatal(err)
	}
	var reference foundation.ReferenceVersion
	post("/references", form.FormDataContentType(), &body, &reference)
	bytesHash := sha256.Sum256(png)
	if reference.SHA256 != hex.EncodeToString(bytesHash[:]) || reference.ByteLength != int64(len(png)) {
		t.Fatal("reference byte facts changed")
	}
	input := service.HNGenerationPrepareInput{NodeID: "_video-intent", ShotID: shotA.ShotID, PromptSnapshot: "Synthetic local camera intent", Protocol: "", ProviderIdentity: "", Parameters: json.RawMessage(`{"size":"16:9","videoSeconds":"4","vquality":"768P"}`), SourceBaseline: service.HNGenerationSourceBaseline, ReferenceBindings: []foundation.ReferenceBinding{{ReferenceVersionID: reference.ID, SHA256: reference.SHA256, Role: "firstFrame"}, {ReferenceVersionID: reference.ID, SHA256: reference.SHA256, Role: "lastFrame"}}}
	prepare := func() service.HNPreparedGeneration {
		raw, e := json.Marshal(input)
		if e != nil {
			t.Fatal(e)
		}
		var g service.HNPreparedGeneration
		post("/generations/prepare", "application/json", bytes.NewReader(raw), &g)
		return g
	}
	a := prepare()
	if preparePosts.Load() != 1 {
		t.Fatal("one explicit prepare did not produce exactly one POST")
	}
	shotB := ensure()
	if shotA != shotB {
		t.Fatal("repeat ensure changed shot or label")
	}
	b := prepare()
	if a.GenerationID == b.GenerationID || a.ShotID != b.ShotID || a.ShotID != shotA.ShotID || !a.Frozen || !b.Frozen || a.Status != "PREPARED" || b.SubmissionState != "PREPARED" || !reflect.DeepEqual(a.ReferenceBindings, input.ReferenceBindings) {
		t.Fatal("local frozen attempts changed")
	}
	w, err := foundation.Open(root, project)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, table := range []string{"shots", "reference_versions", "generations", "task_bindings", "results", "archive_jobs", "candidates", "sequence_items"} {
		rows, e := w.List(table)
		if e != nil {
			t.Fatal(e)
		}
		counts[table] = len(rows)
	}
	if counts["shots"] != 1 || counts["reference_versions"] != 1 || counts["generations"] != 2 {
		t.Fatal("local fact counts mismatch")
	}
	for _, table := range []string{"task_bindings", "results", "archive_jobs", "candidates", "sequence_items"} {
		if counts[table] != 0 {
			t.Fatal("crossed a later boundary")
		}
	}
	var before [2]json.RawMessage
	for i, g := range []service.HNPreparedGeneration{a, b} {
		before[i], err = w.Read("generations", g.GenerationID)
		if err != nil {
			t.Fatal(err)
		}
		var stored foundation.Generation
		if json.Unmarshal(before[i], &stored) != nil || stored.Protocol != "" || stored.ProviderIdentity != "" || stored.ConnectionID != "" || stored.SourceBaseline != service.HNGenerationSourceBaseline {
			t.Fatal("unbound facts changed")
		}
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHNCanvasLocalPrepareReopenChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), "HN_R18_CHILD_ROOT="+root, "HN_R18_CHILD_PROJECT="+project, "HN_R18_CHILD_IDS="+a.GenerationID+","+b.GenerationID)
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("process reopen failed: %v %s", e, output)
	}
	w, err = foundation.Open(root, project)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for i, g := range []service.HNPreparedGeneration{a, b} {
		after, e := w.Read("generations", g.GenerationID)
		if e != nil || !bytes.Equal(before[i], after) {
			t.Fatal("reopen mutated historical generation")
		}
	}
	if preparePosts.Load() != 2 || referencePosts.Load() != 1 || forbidden.Load() != 0 {
		t.Fatal("unexpected local network counts")
	}
	evidence := map[string]any{"serverObservedShotPOST": shotPosts.Load(), "serverObservedReferencePOST": referencePosts.Load(), "serverObservedPreparePOST": preparePosts.Load(), "rowCounts": counts, "sameShot": true, "newGenerationPerExplicitAttempt": true, "processRestartReopen": "PASS", "historicalRawBytes": "IDENTICAL", "providerBoundCalls": 0, "realProviderCalls": 0}
	if path := os.Getenv("HN_R18_GO_EVIDENCE"); path != "" {
		raw, e := json.MarshalIndent(evidence, "", "  ")
		if e != nil || os.WriteFile(path, raw, 0600) != nil {
			t.Fatal("evidence write failed")
		}
	}
	t.Logf("R18 local composition PASS: prepare POST=2, Shot=1, Generation=2, Provider=0")
}

func TestHNCanvasLocalPrepareReopenChild(t *testing.T) {
	root := os.Getenv("HN_R18_CHILD_ROOT")
	if root == "" {
		t.Skip("child-only isolated fixture")
	}
	w, err := foundation.Open(root, os.Getenv("HN_R18_CHILD_PROJECT"))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, id := range strings.Split(os.Getenv("HN_R18_CHILD_IDS"), ",") {
		raw, e := w.Read("generations", id)
		var g foundation.Generation
		if e != nil || json.Unmarshal(raw, &g) != nil || !g.Frozen || g.Status != "PREPARED" || g.SubmissionState != "PREPARED" || g.ShotID == "" {
			t.Fatal(fmt.Sprint("invalid reopened local generation"))
		}
	}
}
