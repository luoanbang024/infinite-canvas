package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/tigerowo/infinite-canvas/service"
)

var hnSyntheticMP4 = []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'}

func TestHNLocalArchiveHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		status                           int
		field, value, peer, host, origin string
		files                            int
		data                             []byte
		disabled, noHeader               bool
	}{
		{name: "valid", status: 200, files: 1}, {name: "disabled", status: 503, files: 1, disabled: true}, {name: "header", status: 403, files: 1, noHeader: true},
		{name: "peer", status: 403, files: 1, peer: "192.0.2.1:9"}, {name: "host", status: 403, files: 1, host: "remote.example"}, {name: "origin", status: 403, files: 1, origin: "https://remote.example"},
		{name: "missing", status: 400, files: 0}, {name: "multiple", status: 400, files: 2}, {name: "empty", status: 400, files: 1, data: []byte{}},
		{name: "kind", status: 400, files: 1, field: "resultKind", value: "image"}, {name: "mime", status: 400, files: 1, field: "mimeType", value: "audio/mpeg"},
		{name: "secret-field", status: 400, files: 1, field: "token", value: "synthetic"}, {name: "provider-fact", status: 400, files: 1, field: "providerResultId", value: "synthetic"},
		{name: "signature", status: 400, files: 1, data: []byte("not video")}, {name: "hash-failure-with-retry-id", status: 422, files: 1, field: "sha256", value: hex.EncodeToString(make([]byte, 32))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HN_PROJECTS_ROOT", root)
			if tc.disabled {
				t.Setenv("HN_PROJECTS_ROOT", "")
			}
			g, e := service.PrepareLocalGeneration(root, "local", service.HNGenerationPrepareInput{NodeID: "video", PromptSnapshot: "synthetic", Parameters: json.RawMessage(`{}`), SourceBaseline: service.HNGenerationSourceBaseline})
			if e != nil {
				t.Fatal(e)
			}
			data := hnSyntheticMP4
			if tc.data != nil {
				data = tc.data
			}
			hash := sha256.Sum256(data)
			values := map[string]string{"resultKind": "video", "mimeType": "video/mp4", "sha256": hex.EncodeToString(hash[:])}
			if tc.field != "" {
				values[tc.field] = tc.value
			}
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			for k, v := range values {
				form.WriteField(k, v)
			}
			for n := 0; n < tc.files; n++ {
				f, _ := form.CreateFormFile("file", "../../untrusted-name.mp4")
				f.Write(data)
			}
			form.Close()
			r := httptest.NewRequest("POST", "http://127.0.0.1/api/hn/projects/local/generations/"+g.GenerationID+"/results/local-archive", &body)
			r.RemoteAddr = "127.0.0.1:9"
			r.Header.Set("Content-Type", form.FormDataContentType())
			r.Header.Set("X-HN-Local-Request", "1")
			if tc.noHeader {
				r.Header.Del("X-HN-Local-Request")
			}
			if tc.peer != "" {
				r.RemoteAddr = tc.peer
			}
			if tc.host != "" {
				r.Host = tc.host
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			out := httptest.NewRecorder()
			HNLocalResultArchive(out, r, "local", g.GenerationID, "")
			if out.Code != tc.status {
				t.Fatalf("status %d %s", out.Code, out.Body.String())
			}
			if tc.status == 200 || tc.status == 422 {
				var envelope struct{ Data service.HNArchiveFacts }
				json.Unmarshal(out.Body.Bytes(), &envelope)
				if envelope.Data.ArchiveJobID == "" || bytes.Contains(out.Body.Bytes(), []byte(root)) {
					t.Fatal("missing job or absolute path")
				}
			}
		})
	}
}

func TestHNLocalArchiveOversizeStreaming(t *testing.T) {
	t.Setenv("HN_PROJECTS_ROOT", t.TempDir())
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer writer.Close()
		f, e := form.CreateFormFile("file", "large.mp4")
		if e != nil {
			return
		}
		block := make([]byte, 64<<10)
		for n := int64(0); n < service.HNArchiveMaxBytes+(128<<10); n += int64(len(block)) {
			if _, e = f.Write(block); e != nil {
				return
			}
		}
		form.Close()
	}()
	r := httptest.NewRequest("POST", "http://localhost/", reader)
	r.RemoteAddr = "127.0.0.1:9"
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("X-HN-Local-Request", "1")
	out := httptest.NewRecorder()
	HNLocalResultArchive(out, r, "local", "generation", "")
	reader.Close()
	<-done
	if out.Code != 400 {
		t.Fatal("oversize accepted")
	}
}
