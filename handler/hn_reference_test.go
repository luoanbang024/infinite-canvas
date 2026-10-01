package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

func hnTestImage(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{R: 42, G: 90, B: 170, A: 255})
	if err := png.Encode(&out, im); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func hnTestRequest(t *testing.T, data []byte, filename, logical, kind string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if filename != "" {
		file, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	_ = mw.WriteField("logicalReferenceId", logical)
	_ = mw.WriteField("kind", kind)
	_ = mw.WriteField("mimeType", "text/plain") // actual bytes take precedence
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/hn/projects/canvas-test/references", &body)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-HN-Local-Request", "1")
	return r
}

func TestHNReferenceBoundary(t *testing.T) {
	imageBytes := hnTestImage(t)
	for _, tc := range []struct {
		name                    string
		status                  int
		change                  func(*http.Request)
		project, filename, kind string
		data                    []byte
		disabled                bool
	}{
		{name: "disabled", status: 503, disabled: true},
		{name: "missing-header", status: 403, change: func(r *http.Request) { r.Header.Del("X-HN-Local-Request") }},
		{name: "external-address", status: 403, change: func(r *http.Request) { r.RemoteAddr = "192.0.2.1:1234"; r.Header.Set("X-Forwarded-For", "127.0.0.1") }},
		{name: "untrusted-host", status: 403, change: func(r *http.Request) { r.Host = "evil.example" }},
		{name: "cross-site", status: 403, change: func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }},
		{name: "null-origin", status: 403, change: func(r *http.Request) { r.Header.Set("Origin", "null") }},
		{name: "unsafe-project", status: 400, project: "../escape"},
		{name: "device-project", status: 400, project: "CON"},
		{name: "empty-image", status: 400, data: []byte{}},
		{name: "missing-image", status: 400, filename: "missing"},
		{name: "not-image", status: 400, data: []byte("pretend PNG")},
		{name: "traversal-filename", status: 400, filename: "../private.png"},
		{name: "windows-path-filename", status: 400, filename: `C:\private.png`},
		{name: "non-image-kind", status: 400, kind: "video"},
		{name: "oversize-image", status: 400, data: bytes.Repeat([]byte{0}, int(HNReferenceMaxBytes+1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "projects")
			t.Setenv("HN_PROJECTS_ROOT", root)
			if tc.disabled {
				t.Setenv("HN_PROJECTS_ROOT", "")
			}
			data := imageBytes
			if tc.data != nil {
				data = tc.data
			}
			filename := tc.filename
			if filename == "" {
				filename = "image.png"
			}
			if filename == "missing" {
				filename = ""
			}
			kind := tc.kind
			if kind == "" {
				kind = "image"
			}
			project := tc.project
			if project == "" {
				project = "canvas-test"
			}
			r := hnTestRequest(t, data, filename, "ref-test", kind)
			if tc.change != nil {
				tc.change(r)
			}
			w := httptest.NewRecorder()
			HNFreezeReference(w, r, project)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), root) {
				t.Fatal("absolute path leak")
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatal("rejected request created workspace")
			}
		})
	}
}

func TestHNReferenceExactBytesVersionsAndReopen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projects")
	t.Setenv("HN_PROJECTS_ROOT", root)
	original := hnTestImage(t)
	changed := append(append([]byte{}, original...), byte(42))
	var refs []foundation.ReferenceVersion
	for _, data := range [][]byte{original, original, changed} {
		r := hnTestRequest(t, data, "image.png", "ref-same", "image")
		r.Header.Set("Origin", "http://127.0.0.1:3013")
		w := httptest.NewRecorder()
		HNFreezeReference(w, r, "canvas-test")
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:3013" {
			t.Fatal("CORS missing")
		}
		var payload struct {
			Code int                         `json:"code"`
			Data foundation.ReferenceVersion `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		ref := payload.Data
		hash := sha256.Sum256(data)
		if payload.Code != 0 || ref.ProjectID != "canvas-test" || ref.MimeType != "image/png" || ref.Kind != "image" || ref.SHA256 != hex.EncodeToString(hash[:]) || ref.ByteLength != int64(len(data)) {
			t.Fatal("wrong metadata")
		}
		if strings.Contains(w.Body.String(), root) || filepath.IsAbs(ref.RelativePath) {
			t.Fatal("absolute path leak")
		}
		actual, err := os.ReadFile(filepath.Join(root, ref.ProjectID, filepath.FromSlash(ref.RelativePath)))
		if err != nil || !bytes.Equal(actual, data) {
			t.Fatal("stored bytes changed", err)
		}
		sidecar, err := os.ReadFile(filepath.Join(root, ref.ProjectID, filepath.FromSlash(ref.RelativePath)+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var fromSidecar foundation.ReferenceVersion
		if err = json.Unmarshal(sidecar, &fromSidecar); err != nil || fromSidecar != ref {
			t.Fatal("sidecar mismatch", err)
		}
		refs = append(refs, ref)
	}
	if refs[0].ID == refs[1].ID || refs[0].SHA256 != refs[1].SHA256 || refs[2].SHA256 == refs[0].SHA256 || refs[2].LogicalReferenceID != refs[0].LogicalReferenceID {
		t.Fatal("version/source semantics")
	}
	w, err := foundation.Open(root, "canvas-test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, ref := range refs {
		raw, err := w.Read("reference_versions", ref.ID)
		if err != nil {
			t.Fatal(err)
		}
		var got foundation.ReferenceVersion
		if err = json.Unmarshal(raw, &got); err != nil || got != ref {
			t.Fatal("reopen mismatch", err)
		}
	}
}

func TestHNReferenceCORSPreflight(t *testing.T) {
	for _, remote := range []string{"127.0.0.1:1234", "[::1]:1234", "192.0.2.1:1234"} {
		r := httptest.NewRequest("OPTIONS", "http://127.0.0.1:8080/api/hn/projects/a/references", nil)
		r.RemoteAddr = remote
		r.Header.Set("Origin", "http://localhost:3013")
		r.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()
		HNReferenceOptions(w, r)
		want := 204
		if strings.HasPrefix(remote, "192.") {
			want = 403
		}
		if w.Code != want {
			t.Fatal(remote, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatal("credentialed CORS")
		}
	}
}

func TestHNReferenceConcurrentExplicitSnapshots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projects")
	t.Setenv("HN_PROJECTS_ROOT", root)
	data := hnTestImage(t)
	requests := make([]*http.Request, 6)
	for i := range requests {
		requests[i] = hnTestRequest(t, data, "image.png", "ref-shared", "image")
	}
	var wg sync.WaitGroup
	for _, request := range requests {
		wg.Add(1)
		go func(r *http.Request) {
			defer wg.Done()
			w := httptest.NewRecorder()
			HNFreezeReference(w, r, "canvas-test")
			if w.Code != 200 {
				t.Errorf("concurrent freeze failed: %d %s", w.Code, w.Body)
			}
		}(request)
	}
	wg.Wait()
	w, err := foundation.Open(root, "canvas-test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	refs, err := w.List("reference_versions")
	if err != nil || len(refs) != 6 {
		t.Fatal("concurrent versions lost", len(refs), err)
	}
}
