package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
	"github.com/tigerowo/infinite-canvas/model"
)

const hnArchiveLocationSentinel = "HN_R14_SYNTHETIC_LOCATION_SENTINEL"
const hnArchiveErrorSentinel = "HN_R14_SYNTHETIC_ERROR_SENTINEL"
const hnArchiveMediaHost = "media.fixture.invalid"

func hnArchiveLocation() string {
	u := url.URL{Scheme: "https", Host: hnArchiveMediaHost, Path: "/output.mp4", RawQuery: "signature=" + hnArchiveLocationSentinel}
	return u.String()
}
func hnArchiveMP4() []byte {
	data := make([]byte, 2048)
	data[3] = 24
	copy(data[4:], "ftyp")
	copy(data[8:], "mp42")
	copy(data[16:], "mp42")
	copy(data[20:], "isom")
	copy(data[24:], "synthetic bounded archive test bytes")
	return data
}
func hnArchiveFixture(t *testing.T, root, taskID string) (foundation.Generation, foundation.TaskBinding) {
	t.Helper()
	p := hnMiniMaxPrepare(t, root, nil)
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal("fixture Open")
	}
	defer w.Close()
	_, owner, e := w.BeginSubmission(p.GenerationID)
	if e != nil {
		t.Fatal("fixture Begin")
	}
	b, e := w.RecordSubmissionAccepted(p.GenerationID, owner.ID, "", taskID)
	if e != nil {
		t.Fatal("fixture acceptance")
	}
	var g foundation.Generation
	if hnReadRecord(w, "generations", p.GenerationID, &g) != nil {
		t.Fatal("fixture read")
	}
	return g, b
}

type hnArchiveWire struct{ query, media, posts, lookups, dials atomic.Int32 }

func hnArchiveBody(binding foundation.TaskBinding, state string) map[string]any {
	body := hnPollBody(state)
	task := body["task"].(map[string]any)
	task["id"] = binding.ProviderTaskID
	if state == "succeeded" {
		task["content"] = map[string]any{"url": hnArchiveLocation()}
	}
	return body
}
func hnArchiveTLSCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal("fixture key")
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(14), Subject: pkix.Name{CommonName: hnArchiveMediaHost}, DNSNames: []string{hnArchiveMediaHost}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	raw, e := x509.CreateCertificate(rand.Reader, cert, cert, public, private)
	if e != nil {
		t.Fatal("fixture certificate")
	}
	leaf, e := x509.ParseCertificate(raw)
	if e != nil {
		t.Fatal("fixture parse")
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}
}
func hnArchiveNetwork(t *testing.T, b foundation.TaskBinding, queryHandler, mediaHandler http.HandlerFunc) (hnMiniMaxArchiveOptions, *hnArchiveWire) {
	t.Helper()
	wire := &hnArchiveWire{}
	query := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wire.query.Add(1)
		if r.Method == "POST" {
			wire.posts.Add(1)
		}
		if r.Method != "GET" || r.Host != "api.minimax.io" || r.URL.Path != "/v2/query/video_generation/"+b.ProviderTaskID || r.URL.RawQuery != "" || r.ProtoMajor != 1 || !r.Close || r.Header.Get("Authorization") != "Bearer "+hnMiniMaxSyntheticKey || r.Header.Get("Cookie") != "" {
			t.Error("query wire violated")
		}
		if queryHandler == nil {
			json.NewEncoder(w).Encode(hnArchiveBody(b, "succeeded"))
		} else {
			queryHandler(w, r)
		}
	}))
	query.StartTLS()
	t.Cleanup(query.Close)
	certificate := hnArchiveTLSCertificate(t)
	media := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wire.media.Add(1)
		if r.Method == "POST" {
			wire.posts.Add(1)
		}
		if r.Method != "GET" || r.Host != hnArchiveMediaHost || r.URL.Path != "/output.mp4" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Accept-Encoding") != "" || r.ProtoMajor != 1 || !r.Close || r.TLS == nil || r.TLS.ServerName != hnArchiveMediaHost {
			t.Error("credential isolation / TLS / media policy violated")
		}
		if mediaHandler == nil {
			hnArchiveSend(w, hnArchiveMP4(), 2048)
		} else {
			mediaHandler(w, r)
		}
	}))
	media.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	media.StartTLS()
	t.Cleanup(media.Close)
	mediaURL, e := url.Parse(media.URL)
	if e != nil || mediaURL.Hostname() != "127.0.0.1" {
		t.Fatal("fixture nonlocal")
	}
	pool := x509.NewCertPool()
	pool.AddCert(certificate.Leaf)
	options := hnMiniMaxArchiveOptions{query: hnMiniMaxLocalOptions(t, query), mediaTimeout: 2 * time.Second, mediaRoots: pool,
		lookupIP: func(ctx context.Context, host string) ([]netip.Addr, error) {
			wire.lookups.Add(1)
			if host != hnArchiveMediaHost {
				t.Error("DNS unexpected host")
				return nil, ErrHNMiniMaxArchiveDestination
			}
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		},
		mediaDial: func(ctx context.Context, network, address string) (net.Conn, error) {
			wire.dials.Add(1)
			if address != "8.8.8.8:443" {
				t.Error("un-pinned media dial")
				return nil, ErrHNMiniMaxArchiveDestination
			}
			return (&net.Dialer{}).DialContext(ctx, network, mediaURL.Host)
		}}
	return options, wire
}
func hnArchiveSend(w http.ResponseWriter, data []byte, declared int) {
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Length", stringInt(declared))
	w.WriteHeader(200)
	w.Write(data)
}
func stringInt(n int) string { return new(big.Int).SetInt64(int64(n)).String() }
func hnArchiveCall(ctx context.Context, root, binding, retry string, options hnMiniMaxArchiveOptions) (HNArchiveFacts, error) {
	// Negative tests cannot fall through to real Provider/DNS even on a regression.
	if options.query.dialContext == nil {
		options.query.dialContext = func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New(hnArchiveErrorSentinel)
		}
	}
	if options.lookupIP == nil {
		options.lookupIP = func(context.Context, string) ([]netip.Addr, error) { return nil, errors.New(hnArchiveErrorSentinel) }
	}
	return archiveHNMiniMax(ctx, root, "test", binding, retry, "test-owner", hnMiniMaxSyntheticResolver, options)
}
func hnArchivePrivate(t *testing.T, root string, public any, e error) {
	t.Helper()
	hnPollAssertPrivate(t, root, public, e)
	raw, _ := json.Marshal(public)
	if e != nil {
		raw = append(raw, e.Error()...)
	}
	forbidden := []string{hnArchiveLocationSentinel, hnArchiveErrorSentinel, hnArchiveLocation(), hnMiniMaxSyntheticKey}
	check := func(data []byte) error {
		for _, secret := range forbidden {
			if bytes.Contains(data, []byte(secret)) {
				return errors.New("private archive data escaped")
			}
		}
		return nil
	}
	if check(raw) != nil {
		t.Fatal("public archive leak")
	}
	if filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return check(data)
	}) != nil {
		t.Fatal("durable archive leak")
	}
}
func hnArchiveRecords(t *testing.T, root string) ([]foundation.Result, []foundation.ArchiveJob) {
	t.Helper()
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal("record Open")
	}
	defer w.Close()
	results := []foundation.Result{}
	jobs := []foundation.ArchiveJob{}
	for _, kind := range []string{"results", "archive_jobs", "candidates", "sequence_items"} {
		rows, e := w.List(kind)
		if e != nil {
			t.Fatal("record list")
		}
		for _, raw := range rows {
			switch kind {
			case "results":
				var r foundation.Result
				if json.Unmarshal(raw, &r) != nil {
					t.Fatal("result parse")
				}
				results = append(results, r)
			case "archive_jobs":
				var j foundation.ArchiveJob
				if json.Unmarshal(raw, &j) != nil {
					t.Fatal("job parse")
				}
				jobs = append(jobs, j)
			default:
				t.Fatal("out-of-scope record")
			}
		}
	}
	return results, jobs
}
func hnArchiveUnchanged(t *testing.T, root string, g foundation.Generation, b foundation.TaskBinding) {
	t.Helper()
	gotG, gotB := hnPollRawRecords(t, root, g.ID, b.ID)
	if !reflect.DeepEqual(g, gotG) || !reflect.DeepEqual(b, gotB) {
		t.Fatal("Generation/TaskBinding mutation")
	}
}
func hnArchiveVerify(t *testing.T, root string, g foundation.Generation, b foundation.TaskBinding, f HNArchiveFacts) {
	t.Helper()
	w, e := foundation.Open(root, "test")
	if e != nil {
		t.Fatal("archive reopen")
	}
	defer w.Close()
	var r foundation.Result
	var j foundation.ArchiveJob
	if hnReadRecord(w, "results", f.ResultID, &r) != nil || hnReadRecord(w, "archive_jobs", f.ArchiveJobID, &j) != nil {
		t.Fatal("archive identity read")
	}
	data := hnArchiveMP4()
	hash := archiveHash(data)
	if r.Status != "ARCHIVED" || j.Status != "ARCHIVED" || r.GenerationID != g.ID || r.TaskBindingID != b.ID || r.ProviderResultID != b.ProviderTaskID || r.SourceURLRef != "" || r.ResultKind != "video" || r.DurationSeconds == nil || *r.DurationSeconds != 6 || r.SHA256 != hash || j.ActualSHA256 != hash || r.ByteLength != int64(len(data)) || j.ActualBytes != r.ByteLength || r.ArchiveJobID != j.ID || r.ArchivedRelativePath != hnMiniMaxArchiveTarget(g.ID, r.ID) || j.TargetRelativePath != r.ArchivedRelativePath || j.ExpectedMime != "video/mp4" {
		t.Fatal("provider archive provenance/bytes")
	}
	path, e := w.Resolve(r.ArchivedRelativePath)
	if e != nil {
		t.Fatal("archive path")
	}
	actual, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(actual, data) {
		t.Fatal("exact media bytes")
	}
	sidecar, e := os.ReadFile(path + ".json")
	var receipt struct {
		ProjectID  string `json:"projectId"`
		ID         string `json:"id"`
		SHA256     string `json:"sha256"`
		ByteLength int64  `json:"byteLength"`
	}
	if e != nil || json.Unmarshal(sidecar, &receipt) != nil || receipt.ProjectID != "test" || receipt.ID != j.ID || receipt.SHA256 != hash || receipt.ByteLength != int64(len(data)) {
		t.Fatal("receipt facts")
	}
	hnArchiveUnchanged(t, root, g, b)
	hnArchivePrivate(t, root, f, nil)
}

func TestHNMiniMaxArchiveSmoke(t *testing.T) {
	root := t.TempDir()
	g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
	options, wire := hnArchiveNetwork(t, b, nil, nil)
	var captured bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&captured)
	defer log.SetOutput(prior)
	facts, e := hnArchiveCall(context.Background(), root, b.ID, "", options)
	if e != nil || wire.query.Load() != 1 || wire.media.Load() != 1 || wire.posts.Load() != 0 {
		t.Fatal("success wire counts")
	}
	hnArchiveVerify(t, root, g, b, facts)
	repeat, e := hnArchiveCall(context.Background(), root, b.ID, "", options)
	if e != nil || repeat != facts || wire.query.Load() != 1 || wire.media.Load() != 1 {
		t.Fatal("archived repeat network/identity")
	}
	again, e := hnArchiveCall(context.Background(), root, "", facts.ArchiveJobID, options)
	if e != nil || again != facts || wire.query.Load() != 1 || wire.media.Load() != 1 {
		t.Fatal("archived retry network/identity")
	}
	// Second accepted task fails after the validated prefix, then reopens/retries.
	g2, b2 := hnArchiveFixture(t, root, "424010985738630")
	var complete atomic.Bool
	retryOptions, retryWire := hnArchiveNetwork(t, b2, nil, func(w http.ResponseWriter, r *http.Request) {
		data := hnArchiveMP4()
		if !complete.Load() {
			data = data[:700]
		}
		hnArchiveSend(w, data, 2048)
	})
	failed, e := hnArchiveCall(context.Background(), root, b2.ID, "", retryOptions)
	if !errors.Is(e, ErrHNMiniMaxArchiveFailed) || failed.ArchiveStatus != "FAILED" || failed.ResultStatus != "ARCHIVE_FAILED" || failed.ArchiveJobID == "" || failed.ResultID == "" {
		t.Fatal("stream failure durable owner")
	}
	hnArchivePrivate(t, root, failed, e)
	results, jobs := hnArchiveRecords(t, root)
	if len(results) != 2 || len(jobs) != 2 {
		t.Fatal("failed archive duplicates")
	}
	pending, e := hnArchiveCall(context.Background(), root, b2.ID, "", retryOptions)
	if !errors.Is(e, ErrHNMiniMaxArchiveRetryRequired) || pending.ResultID != failed.ResultID || pending.ArchiveJobID != failed.ArchiveJobID || retryWire.query.Load() != 1 || retryWire.media.Load() != 1 {
		t.Fatal("normal call retried failed archive")
	}
	complete.Store(true)
	retried, e := hnArchiveCall(context.Background(), root, "", failed.ArchiveJobID, retryOptions)
	if e != nil || retried.ResultID != failed.ResultID || retried.ArchiveJobID != failed.ArchiveJobID || retryWire.query.Load() != 2 || retryWire.media.Load() != 2 {
		t.Fatal("same-job retry counts")
	}
	hnArchiveVerify(t, root, g2, b2, retried)
	results, jobs = hnArchiveRecords(t, root)
	if len(results) != 2 || len(jobs) != 2 {
		t.Fatal("retry duplicate result/job")
	}
	if strings.Contains(captured.String(), hnArchiveLocationSentinel) || strings.Contains(captured.String(), hnMiniMaxSyntheticKey) || strings.Contains(captured.String(), hnArchiveErrorSentinel) || captured.Len() != 0 {
		t.Fatal("private logging")
	}
	if p := os.Getenv("HN_R14_SMOKE_EVIDENCE_PATH"); p != "" {
		evidence := map[string]any{"status": "PASS", "successQueryGETs": wire.query.Load(), "successMediaGETs": wire.media.Load(), "submitPOSTs": wire.posts.Load() + retryWire.posts.Load(), "alreadyArchivedAdditionalNetwork": 0, "retryQueryGETs": retryWire.query.Load(), "retryMediaGETs": retryWire.media.Load(), "sameResultID": true, "sameArchiveJobID": true, "resultCount": len(results), "archiveJobCount": len(jobs), "candidateCount": 0, "firstArchive": facts, "retriedArchive": retried, "results": results, "archiveJobs": jobs, "generationBindingBytes": "UNCHANGED", "rawResultLocationPersistence": "NONE", "realProviderCalls": 0, "realCredentialRead": "NONE"}
		raw, _ := json.MarshalIndent(evidence, "", "  ")
		if os.WriteFile(p, raw, 0600) != nil {
			t.Fatal("safe evidence")
		}
	}
	t.Log("R14_SMOKE=PASS SUCCESS_QUERY_GETS=1 SUCCESS_MEDIA_GETS=1 REPEAT_ADDITIONAL_NETWORK=0 RETRY_QUERY_GETS=2 RETRY_MEDIA_GETS=2 SAME_RESULT_JOB=PASS RESULTS=2 JOBS=2 CANDIDATES=0 EXACT_HASH_BYTES_RECEIPT_REOPEN=PASS GENERATION_BINDING_BYTES=UNCHANGED RAW_URL_PUBLIC_DURABLE_LOGS=NONE")
}

func TestHNMiniMaxArchiveQueryFailures(t *testing.T) {
	cases := []string{"queued", "running", "failed", "cancelled", "401", "403", "404", "429", "500", "503", "307", "drop", "timeout", "malformed", "id", "model", "task_type", "resolution", "duration", "ratio", "modality", "duplicate", "oversized"}
	for _, fault := range cases {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
			options, wire := hnArchiveNetwork(t, b, func(w http.ResponseWriter, r *http.Request) {
				switch fault {
				case "queued", "running", "failed", "cancelled":
					json.NewEncoder(w).Encode(hnArchiveBody(b, fault))
					return
				case "401", "403", "404", "429", "500", "503", "307":
					if fault == "307" {
						w.Header().Set("Location", hnArchiveLocation())
					}
					n := new(big.Int)
					n.SetString(fault, 10)
					w.WriteHeader(int(n.Int64()))
					io.WriteString(w, hnArchiveErrorSentinel)
					return
				case "drop":
					conn, _, e := w.(http.Hijacker).Hijack()
					if e == nil {
						conn.Close()
					}
					return
				case "timeout":
					select {
					case <-r.Context().Done():
					case <-time.After(time.Second):
					}
					return
				case "malformed":
					io.WriteString(w, "{"+hnArchiveErrorSentinel)
					return
				case "duplicate":
					io.WriteString(w, `{"task":{},"task":{}}`)
					return
				case "oversized":
					io.WriteString(w, strings.Repeat("x", hnMiniMaxResponseLimit+1))
					return
				}
				body := hnArchiveBody(b, "succeeded")
				task := body["task"].(map[string]any)
				if fault == "duration" {
					task[fault] = 7
				} else {
					task[fault] = "mismatch"
				}
				json.NewEncoder(w).Encode(body)
			}, nil)
			if fault == "timeout" {
				options.query.timeout = 40 * time.Millisecond
			}
			f, e := hnArchiveCall(context.Background(), root, b.ID, "", options)
			if e == nil || wire.query.Load() != 1 || wire.media.Load() != 0 || wire.lookups.Load() != 0 || wire.posts.Load() != 0 {
				t.Fatal("query failure boundary")
			}
			results, jobs := hnArchiveRecords(t, root)
			if len(results) != 0 || len(jobs) != 0 {
				t.Fatal("query failure metadata")
			}
			hnArchiveUnchanged(t, root, g, b)
			hnArchivePrivate(t, root, f, e)
			t.Log("QUERY_GETS=1 MEDIA_GETS=0 RESULTS=0 JOBS=0 SUBMITTED_BOUND_BYTES=UNCHANGED")
		})
	}
}

func TestHNMiniMaxArchiveMediaURL(t *testing.T) {
	for _, fault := range []string{"http", "userinfo", "fragment", "ip", "ipv6", "localhost", "private-host", "port", "empty-port", "whitespace", "control", "oversized", "empty-host", "numeric-host"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
			u := url.URL{Scheme: "https", Host: hnArchiveMediaHost, Path: "/output.mp4"}
			switch fault {
			case "http":
				u.Scheme = "http"
			case "userinfo":
				u.User = url.User("synthetic")
			case "fragment":
				u.Fragment = "fragment"
			case "ip":
				u.Host = "127.0.0.1"
			case "ipv6":
				u.Host = "[::1]"
			case "localhost":
				u.Host = "localhost"
			case "private-host":
				u.Host = "media.internal"
			case "port":
				u.Host = hnArchiveMediaHost + ":8443"
			case "empty-port":
				u.Host = hnArchiveMediaHost + ":"
			case "empty-host":
				u.Host = ""
			case "numeric-host":
				u.Host = "127.1"
			}
			location := u.String()
			switch fault {
			case "whitespace":
				location += " "
			case "control":
				location += "\x01"
			case "oversized":
				location += strings.Repeat("x", 16385)
			}
			options, wire := hnArchiveNetwork(t, b, func(w http.ResponseWriter, r *http.Request) {
				body := hnArchiveBody(b, "succeeded")
				body["task"].(map[string]any)["content"] = map[string]any{"url": location}
				json.NewEncoder(w).Encode(body)
			}, nil)
			f, e := hnArchiveCall(context.Background(), root, b.ID, "", options)
			if e == nil || wire.query.Load() != 1 || wire.lookups.Load() != 0 || wire.media.Load() != 0 {
				t.Fatal("URL validation allowed media IO")
			}
			rs, js := hnArchiveRecords(t, root)
			if len(rs) != 0 || len(js) != 0 {
				t.Fatal("URL validation metadata")
			}
			hnArchiveUnchanged(t, root, g, b)
			hnArchivePrivate(t, root, f, e)
		})
	}
}
func TestHNMiniMaxArchiveSSRF(t *testing.T) {
	denied := []string{"127.0.0.1", "::1", "10.0.0.1", "172.16.1.1", "192.168.1.1", "169.254.169.254", "fe80::1", "224.0.0.1", "ff02::1", "0.0.0.0", "::", "100.64.0.1", "192.0.2.1", "198.51.100.1", "203.0.113.1", "198.18.1.1", "240.0.0.1", "2001:db8::1", "2002:a00:1::1", "3fff::1", "fc00::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1"}
	root := t.TempDir()
	g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
	options, wire := hnArchiveNetwork(t, b, nil, nil)
	for _, value := range denied {
		t.Run(value, func(t *testing.T) {
			ip := netip.MustParseAddr(value)
			if hnMiniMaxPublicIP(ip) {
				t.Fatal("nonpublic IP approved")
			}
			o := options
			o.lookupIP = func(context.Context, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("8.8.8.8"), ip}, nil
			}
			f, e := hnArchiveCall(context.Background(), root, b.ID, "", o)
			if !errors.Is(e, ErrHNMiniMaxArchiveDestination) || wire.media.Load() != 0 || wire.dials.Load() != 0 {
				t.Fatal("mixed DNS answer reached dial")
			}
			hnArchivePrivate(t, root, f, e)
		})
	}
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888"} {
		if !hnMiniMaxPublicIP(netip.MustParseAddr(ip)) {
			t.Fatal("public IP rejected")
		}
	}
	rs, js := hnArchiveRecords(t, root)
	if len(rs) != 0 || len(js) != 0 {
		t.Fatal("SSRF metadata")
	}
	hnArchiveUnchanged(t, root, g, b)
	t.Log("SSRF_ALL_ANSWERS_CHECKED=PASS MIXED_PUBLIC_PRIVATE=REJECTED PRIVATE_DIALS=0 PUBLIC_PIN_TLS_HOST=AUTHENTICATED_IN_SMOKE")
}

func TestHNMiniMaxArchiveMediaFailures(t *testing.T) {
	for _, fault := range []string{"redirect", "404", "500", "type", "unknown-length", "zero", "oversized", "encoding", "html", "webm", "short-prefix", "timeout", "drop"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
			options, wire := hnArchiveNetwork(t, b, nil, func(w http.ResponseWriter, r *http.Request) {
				switch fault {
				case "redirect":
					w.Header().Set("Location", hnArchiveLocation())
					w.WriteHeader(307)
					return
				case "404":
					w.WriteHeader(404)
					return
				case "500":
					w.WriteHeader(500)
					return
				case "drop":
					c, _, e := w.(http.Hijacker).Hijack()
					if e == nil {
						c.Close()
					}
					return
				case "timeout":
					select {
					case <-r.Context().Done():
					case <-time.After(time.Second):
					}
					return
				case "unknown-length":
					w.Header().Set("Content-Type", "video/mp4")
					w.(http.Flusher).Flush()
					w.Write(hnArchiveMP4())
					return
				case "zero":
					hnArchiveSend(w, nil, 0)
					return
				case "oversized":
					hnArchiveSend(w, nil, int(HNArchiveMaxBytes+1))
					return
				}
				w.Header().Set("Content-Type", "video/mp4")
				data := hnArchiveMP4()
				switch fault {
				case "type":
					w.Header().Set("Content-Type", "video/webm")
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				case "html":
					data = bytes.Repeat([]byte("<html>synthetic</html>"), 32)
				case "webm":
					data = bytes.Repeat([]byte{0x1a, 0x45, 0xdf, 0xa3}, 128)
				case "short-prefix":
					data = data[:16]
				}
				w.Header().Set("Content-Length", stringInt(len(data)))
				w.WriteHeader(200)
				w.Write(data)
			})
			if fault == "timeout" {
				options.mediaTimeout = 40 * time.Millisecond
			}
			f, e := hnArchiveCall(context.Background(), root, b.ID, "", options)
			if e == nil || wire.query.Load() != 1 || wire.media.Load() != 1 || wire.posts.Load() != 0 {
				t.Fatal("media failure wire boundary")
			}
			rs, js := hnArchiveRecords(t, root)
			if len(rs) != 0 || len(js) != 0 {
				t.Fatal("invalid media created metadata")
			}
			hnArchiveUnchanged(t, root, g, b)
			hnArchivePrivate(t, root, f, e)
			t.Log("QUERY_GETS=1 MEDIA_GETS=1 REDIRECT_FOLLOW=0 RESULTS=0 JOBS=0")
		})
	}
}

func TestHNMiniMaxArchiveConcurrentAndGapRepair(t *testing.T) {
	root := t.TempDir()
	g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
	options, wire := hnArchiveNetwork(t, b, nil, nil)
	var wg sync.WaitGroup
	facts := make([]HNArchiveFacts, 2)
	errs := make([]error, 2)
	for i := range facts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			facts[i], errs[i] = hnArchiveCall(context.Background(), root, b.ID, "", options)
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || facts[0] != facts[1] || wire.query.Load() != 1 || wire.media.Load() != 1 {
		t.Fatal("concurrent archive duplicated")
	}
	rs, js := hnArchiveRecords(t, root)
	if len(rs) != 1 || len(js) != 1 {
		t.Fatal("concurrent metadata duplicated")
	}
	hnArchiveVerify(t, root, g, b, facts[0])
	// Seed the exact existing Foundation CreateResult/CreateArchive crash gap.
	gapRoot := t.TempDir()
	g2, b2 := hnArchiveFixture(t, gapRoot, hnMiniMaxFakeTask)
	w, e := foundation.Open(gapRoot, "test")
	if e != nil {
		t.Fatal("gap Open")
	}
	duration := float64(6)
	r, e := w.CreateResult(foundation.Result{GenerationID: g2.ID, TaskBindingID: b2.ID, ProviderResultID: b2.ProviderTaskID, ResultKind: "video", DurationSeconds: &duration})
	w.Close()
	if e != nil {
		t.Fatal("gap result")
	}
	o, c := hnArchiveNetwork(t, b2, nil, nil)
	f, e := hnArchiveCall(context.Background(), gapRoot, b2.ID, "", o)
	if e != nil || f.ResultID != r.ID || c.query.Load() != 1 || c.media.Load() != 1 {
		t.Fatal("gap same-result repair")
	}
	rs, js = hnArchiveRecords(t, gapRoot)
	if len(rs) != 1 || len(js) != 1 {
		t.Fatal("gap repair duplicate")
	}
	hnArchiveVerify(t, gapRoot, g2, b2, f)
	t.Log("CONCURRENT_CALLERS=2 QUERY_GETS=1 MEDIA_GETS=1 RESULTS=1 JOBS=1 GAP_REPAIR_SAME_RESULT=PASS")
}

func TestHNMiniMaxArchivePreflightAndIdentityConflict(t *testing.T) {
	for _, fault := range []string{"unknown", "submitting", "unbound", "unsafe-task", "provider", "model", "frozen-hash", "terminal-class", "owner", "channel", "duplicate-result", "wrong-result", "wrong-job", "foreign-retry", "missing-binding", "missing-project"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			stateRoot := root
			g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
			beforeG, beforeB := g, b
			binding, retry := b.ID, ""
			resolver := HNMiniMaxChannelResolver(hnMiniMaxSyntheticResolver)
			switch fault {
			case "unknown":
				g.Status = "SUBMISSION_UNKNOWN"
				g.SubmissionState = g.Status
			case "submitting":
				g.Status = "SUBMITTING"
				g.SubmissionState = g.Status
			case "unbound":
				b.BindingState = "UNBOUND"
				b.ProviderTaskID = ""
			case "unsafe-task":
				b.ProviderTaskID = "../unsafe"
			case "provider":
				b.ProviderIdentity = "metaso"
			case "model":
				g.Model = "MiniMax-H3-Max"
			case "frozen-hash":
				g.PromptSnapshot += "changed"
			case "terminal-class":
				b.ErrorClass = "MINIMAX_TASK_FAILED"
			case "owner":
				resolver = func(string, string) (model.ModelChannel, error) {
					return model.ModelChannel{}, errors.New(hnArchiveErrorSentinel)
				}
			case "channel":
				resolver = func(o, c string) (model.ModelChannel, error) {
					channel := hnMiniMaxSyntheticChannel()
					channel.BaseURL = "https://gateway.invalid"
					return channel, nil
				}
			case "missing-binding":
				binding = "missing"
			case "missing-project":
				root = filepath.Join(root, "missing")
			}
			if !reflect.DeepEqual(g, beforeG) || !reflect.DeepEqual(b, beforeB) {
				hnPollPersistFixture(t, root, g, b)
			}
			if strings.Contains(fault, "result") || fault == "wrong-job" || fault == "foreign-retry" {
				w, e := foundation.Open(root, "test")
				if e != nil {
					t.Fatal("conflict Open")
				}
				duration := float64(6)
				result := foundation.Result{GenerationID: g.ID, TaskBindingID: b.ID, ProviderResultID: b.ProviderTaskID, ResultKind: "video", DurationSeconds: &duration}
				if fault == "wrong-result" {
					result.ProviderResultID = "different-task"
				}
				r, e := w.CreateResult(result)
				if e != nil {
					t.Fatal("conflict fixture result")
				}
				if fault == "duplicate-result" {
					if _, e = w.CreateResult(result); e != nil {
						t.Fatal("duplicate fixture")
					}
				}
				if fault == "wrong-job" {
					if _, e = w.CreateArchive(r.ID, "generated/"+g.ID+"/"+r.ID+"/wrong.mp4", "video/mp4"); e != nil {
						t.Fatal("wrong target fixture")
					}
				}
				if fault == "foreign-retry" {
					j, e := w.CreateArchive(r.ID, hnMiniMaxArchiveTarget(g.ID, r.ID), "video/mp4")
					if e != nil {
						t.Fatal("retry fixture")
					}
					binding = ""
					retry = j.ID
					result.ProviderResultID = "different-task"
					raw, _ := json.Marshal(result)
					_ = raw // Retry chain corruption is exercised below by task identity.
					b.ProviderTaskID = "different-task"
					w.Close()
					hnPollPersistFixture(t, root, g, b)
					w = nil
				}
				if w != nil {
					w.Close()
				}
			}
			options, wire := hnArchiveNetwork(t, b, nil, nil)
			f, e := archiveHNMiniMax(context.Background(), root, "test", binding, retry, "test-owner", resolver, options)
			if e == nil || wire.query.Load() != 0 || wire.media.Load() != 0 {
				t.Fatal("preflight/conflict crossed network")
			}
			hnArchiveUnchanged(t, stateRoot, g, b)
			hnArchivePrivate(t, stateRoot, f, e)
		})
	}
	// Public signatures have no network overrides; invalid outer input rejects.
	if _, e := ArchiveMiniMaxH3OfficialResult(nil, "unused", "test", "binding", "test-owner", hnMiniMaxSyntheticResolver); e == nil {
		t.Fatal("nil context")
	}
	if _, e := RetryMiniMaxH3OfficialResultArchive(context.Background(), "unused", "../project", "job", "test-owner", hnMiniMaxSyntheticResolver); e == nil {
		t.Fatal("unsafe project")
	}
}

func TestHNMiniMaxArchiveRestartChild(t *testing.T) {
	if os.Getenv("HN_R14_CHILD_MODE") != "archive-reopen" {
		return
	}
	root := os.Getenv("HN_R14_CHILD_ROOT")
	binding := os.Getenv("HN_R14_CHILD_BINDING")
	f, e := hnArchiveCall(context.Background(), root, binding, "", hnMiniMaxArchiveOptions{})
	if e != nil || f.ResultStatus != "ARCHIVED" || f.ArchiveStatus != "ARCHIVED" {
		t.Fatal("child offline durable archive")
	}
	raw, _ := json.Marshal(f)
	if os.WriteFile(filepath.Join(root, "child-facts.json"), raw, 0600) != nil {
		t.Fatal("child facts")
	}
}
func TestHNMiniMaxArchiveProcessReopen(t *testing.T) {
	root := t.TempDir()
	g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
	options, wire := hnArchiveNetwork(t, b, nil, nil)
	f, e := hnArchiveCall(context.Background(), root, b.ID, "", options)
	if e != nil {
		t.Fatal("parent archive")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestHNMiniMaxArchiveRestartChild$")
	command.Env = append(os.Environ(), "HN_R14_CHILD_MODE=archive-reopen", "HN_R14_CHILD_ROOT="+root, "HN_R14_CHILD_BINDING="+b.ID)
	output, e := command.CombinedOutput()
	if e != nil || bytes.Contains(output, []byte(hnArchiveLocationSentinel)) || bytes.Contains(output, []byte(hnMiniMaxSyntheticKey)) {
		t.Fatal("child reopen failed")
	}
	data, e := os.ReadFile(filepath.Join(root, "child-facts.json"))
	var got HNArchiveFacts
	if e != nil || json.Unmarshal(data, &got) != nil || got != f || wire.query.Load() != 1 || wire.media.Load() != 1 {
		t.Fatal("separate process repeat changed")
	}
	hnArchiveVerify(t, root, g, b, f)
	t.Log("SEPARATE_PROCESS_REOPEN=PASS DURABLE_HASH_BYTES_RECEIPT=PASS ADDITIONAL_QUERY_MEDIA=0 SAME_RESULT_JOB=PASS")
}

func TestHNMiniMaxArchiveRevalidationAndSecrecy(t *testing.T) {
	for _, fault := range []string{"binding-change", "credential-in-location", "escaped-credential-in-location", "empty-dns", "dns-error", "tls-trust", "resolver-panic"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
			expectedB := b
			options, wire := hnArchiveNetwork(t, b, func(w http.ResponseWriter, r *http.Request) {
				body := hnArchiveBody(b, "succeeded")
				if strings.Contains(fault, "credential-in-location") {
					u := url.URL{Scheme: "https", Host: hnArchiveMediaHost, Path: "/output.mp4"}
					key := hnMiniMaxSyntheticKey
					if strings.HasPrefix(fault, "escaped") {
						key = strings.ReplaceAll(key, "H", "%48")
					}
					u.RawQuery = "opaque=" + key
					body["task"].(map[string]any)["content"] = map[string]any{"url": u.String()}
				}
				if fault == "binding-change" {
					expectedB.LastPolledAt = time.Now().UTC().Format(time.RFC3339Nano)
					hnPollPersistFixture(t, root, g, expectedB)
				}
				json.NewEncoder(w).Encode(body)
			}, nil)
			if fault == "empty-dns" {
				options.lookupIP = func(context.Context, string) ([]netip.Addr, error) { return nil, nil }
			}
			if fault == "dns-error" {
				options.lookupIP = func(context.Context, string) ([]netip.Addr, error) { return nil, errors.New(hnArchiveLocation()) }
			}
			if fault == "tls-trust" {
				options.mediaRoots = x509.NewCertPool()
			}
			resolver := HNMiniMaxChannelResolver(hnMiniMaxSyntheticResolver)
			if fault == "resolver-panic" {
				resolver = func(string, string) (model.ModelChannel, error) { panic(hnArchiveErrorSentinel) }
			}
			var captured bytes.Buffer
			prior := log.Writer()
			log.SetOutput(&captured)
			defer log.SetOutput(prior)
			f, e := archiveHNMiniMax(context.Background(), root, "test", b.ID, "", "test-owner", resolver, options)
			if e == nil {
				t.Fatal("security/state failure accepted")
			}
			rs, js := hnArchiveRecords(t, root)
			if len(rs) != 0 || len(js) != 0 {
				t.Fatal("state/security failure created records")
			}
			if fault == "binding-change" {
				if !errors.Is(e, ErrHNMiniMaxPollStateConflict) || wire.media.Load() != 1 {
					t.Fatal("metadata revalidation missing")
				}
			} else if wire.media.Load() != 0 {
				t.Fatal("security failure reached media handler")
			}
			if fault == "resolver-panic" && wire.query.Load() != 0 {
				t.Fatal("resolver panic queried")
			}
			hnArchiveUnchanged(t, root, g, expectedB)
			hnArchivePrivate(t, root, f, e)
			for _, secret := range []string{hnArchiveLocationSentinel, hnArchiveErrorSentinel, hnMiniMaxSyntheticKey, hnArchiveLocation()} {
				if strings.Contains(captured.String(), secret) {
					t.Fatal("security failure logged private value")
				}
			}
		})
	}
	t.Log("PRE_METADATA_OWNERSHIP_REVALIDATION=PASS URL_EMBEDDED_CREDENTIAL=REJECTED DNS_FAILURE=SAFE TLS_TRUST=REQUIRED RESOLVER_PANIC=SAFE")
}
func TestHNMiniMaxArchiveDurableMismatchRejectsOverwrite(t *testing.T) {
	for _, fault := range []string{"bytes", "receipt"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			g, b := hnArchiveFixture(t, root, hnMiniMaxFakeTask)
			o, wire := hnArchiveNetwork(t, b, nil, nil)
			f, e := hnArchiveCall(context.Background(), root, b.ID, "", o)
			if e != nil {
				t.Fatal("setup archive")
			}
			path := filepath.Join(root, "test", filepath.FromSlash(f.ArchivedRelativePath))
			if fault == "bytes" {
				data := hnArchiveMP4()
				data[30] ^= 1
				if os.WriteFile(path, data, 0600) != nil {
					t.Fatal("byte fixture")
				}
			} else {
				if os.WriteFile(path+".json", []byte(`{}`), 0600) != nil {
					t.Fatal("receipt fixture")
				}
			}
			broken, e := hnArchiveCall(context.Background(), root, b.ID, "", o)
			if !errors.Is(e, ErrHNMiniMaxArchiveRetryRequired) || broken.ArchiveStatus != "INCONSISTENT" || wire.query.Load() != 1 || wire.media.Load() != 1 {
				t.Fatal("unverified archive repeated as success")
			}
			repaired, e := hnArchiveCall(context.Background(), root, "", f.ArchiveJobID, o)
			if !errors.Is(e, ErrHNMiniMaxArchiveFailed) || repaired.ResultID != f.ResultID || repaired.ArchiveJobID != f.ArchiveJobID || wire.query.Load() != 2 || wire.media.Load() != 2 {
				t.Fatal("immutable final artifact overwrite was not safely rejected")
			}
			rs, js := hnArchiveRecords(t, root)
			if len(rs) != 1 || len(js) != 1 || rs[0].Status == "ARCHIVED" {
				t.Fatal("corrupt archive metadata duplicated or falsely completed")
			}
			hnArchiveUnchanged(t, root, g, b)
			hnArchivePrivate(t, root, repaired, e)
		})
	}
	t.Log("REOPEN_TAMPERED_BYTES_RECEIPT=INCONSISTENT NORMAL_CALL_NETWORK=0 EXPLICIT_RETRY_SAME_RESULT_JOB=RETAINED IMMUTABLE_CORRUPT_FINAL_OVERWRITE=REJECTED")
}
