package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var (
	ErrHNMiniMaxArchivePreflight     = errors.New("MiniMax provider result archive preflight rejected")
	ErrHNMiniMaxArchiveConflict      = errors.New("PROVIDER_RESULT_IDENTITY_CONFLICT")
	ErrHNMiniMaxArchiveNotReady      = errors.New("MiniMax provider task result not ready")
	ErrHNMiniMaxArchiveDestination   = errors.New("provider media destination rejected")
	ErrHNMiniMaxArchiveMedia         = errors.New("provider MP4 response rejected")
	ErrHNMiniMaxArchiveFailed        = errors.New("provider result archive failed; explicit same-job retry required")
	ErrHNMiniMaxArchiveRetryRequired = errors.New("existing provider archive requires explicit same-job retry")
)

// Private seams are only used by authenticated loopback tests. Public callers
// cannot replace destinations, resolution, TLS trust, or dial behavior.
type hnMiniMaxArchiveOptions struct {
	query        hnMiniMaxHTTPOptions
	mediaTimeout time.Duration
	lookupIP     func(context.Context, string) ([]netip.Addr, error)
	mediaDial    func(context.Context, string, string) (net.Conn, error)
	mediaRoots   *x509.CertPool
}

// Reuse the audited R13 validator; retain the location only in private R14 scope.
func hnMiniMaxArchiveQuery(ctx context.Context, key string, b foundation.TaskBinding, expected hnMiniMaxRequest, options hnMiniMaxHTTPOptions) (string, error) {
	timeout := options.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := &http.Transport{Proxy: nil, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, DialContext: options.dialContext, TLSClientConfig: options.tlsConfig, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 16 << 10}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.minimax.io/v2/query/video_generation/"+url.PathEscape(b.ProviderTaskID), nil)
	if e != nil {
		return "", ErrHNMiniMaxArchivePreflight
	}
	req.Header.Set("Authorization", "Bearer "+key)
	response, e := client.Do(req)
	if e != nil {
		return "", ErrHNMiniMaxPollTransient
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200:
	case 401, 403:
		return "", ErrHNMiniMaxPollAuth
	case 404:
		return "", ErrHNMiniMaxPollNotFound
	case 429:
		return "", ErrHNMiniMaxPollTransient
	default:
		if response.StatusCode >= 500 && response.StatusCode <= 599 {
			return "", ErrHNMiniMaxPollTransient
		}
		return "", ErrHNMiniMaxPollProtocol
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, hnMiniMaxResponseLimit+1))
	if e != nil {
		return "", ErrHNMiniMaxPollTransient
	}
	if len(raw) > hnMiniMaxResponseLimit {
		return "", ErrHNMiniMaxPollProtocol
	}
	obs, e := hnMiniMaxPollParse(raw, b, expected)
	if e != nil {
		return "", e
	}
	if obs.State != "succeeded" {
		return "", ErrHNMiniMaxArchiveNotReady
	}
	object, _ := hnMiniMaxPollObject(raw)
	task, _ := hnMiniMaxPollObject(object["task"])
	content, _ := hnMiniMaxPollObject(task["content"])
	location, _ := hnMiniMaxPollString(content, "url")
	return location, nil
}

func hnMiniMaxMediaURL(location string) (*url.URL, error) {
	if location == "" || len(location) > 16384 || strings.Contains(location, "#") || strings.IndexFunc(location, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return nil, ErrHNMiniMaxArchiveDestination
	}
	u, e := url.Parse(location)
	if e != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Fragment != "" || u.Hostname() == "" || u.Port() != "" && u.Port() != "443" || strings.HasSuffix(u.Host, ":") || net.ParseIP(u.Hostname()) != nil {
		return nil, ErrHNMiniMaxArchiveDestination
	}
	host := strings.ToLower(u.Hostname())
	if len(host) > 253 || !strings.Contains(host, ".") {
		return nil, ErrHNMiniMaxArchiveDestination
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".home", ".lan"} {
		if strings.HasSuffix(host, suffix) {
			return nil, ErrHNMiniMaxArchiveDestination
		}
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label == "localhost" || label[0] == '-' || label[len(label)-1] == '-' {
			return nil, ErrHNMiniMaxArchiveDestination
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return nil, ErrHNMiniMaxArchiveDestination
			}
		}
	}
	if strings.IndexFunc(labels[len(labels)-1], func(r rune) bool { return r >= 'a' && r <= 'z' }) < 0 {
		return nil, ErrHNMiniMaxArchiveDestination
	}
	return u, nil
}

var hnMiniMaxNonpublic = func() []netip.Prefix {
	ranges := []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"}
	out := make([]netip.Prefix, 0, len(ranges))
	for _, r := range ranges {
		out = append(out, netip.MustParsePrefix(r))
	}
	return out
}()

func hnMiniMaxPublicIP(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	// Only currently allocated IPv6 global-unicast space; special-use ranges deny.
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, p := range hnMiniMaxNonpublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// Foundation sees only bounded bytes and static reader errors, never URL errors.
type hnMiniMaxMediaReader struct {
	ctx    context.Context
	source io.Reader
}

func (r hnMiniMaxMediaReader) Read(p []byte) (int, error) {
	if r.ctx.Err() != nil {
		return 0, ErrHNMiniMaxArchiveMedia
	}
	n, e := r.source.Read(p)
	if e != nil && e != io.EOF {
		e = ErrHNMiniMaxArchiveMedia
	}
	return n, e
}
func hnMiniMaxArchiveMedia(ctx context.Context, location string, options hnMiniMaxArchiveOptions) (io.Reader, int64, func(), error) {
	u, e := hnMiniMaxMediaURL(location)
	if e != nil {
		return nil, 0, nil, e
	}
	timeout := options.mediaTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	lookup := options.lookupIP

	addresses, e := func() ([]netip.Addr, error) {
		if options.lookupIP != nil {
			return lookup(ctx, u.Hostname())
		}
		return net.DefaultResolver.LookupNetIP(ctx, "ip", u.Hostname())
	}()
	if e != nil || len(addresses) == 0 {
		cancel()
		return nil, 0, nil, ErrHNMiniMaxArchiveDestination
	}
	for _, ip := range addresses {
		if !hnMiniMaxPublicIP(ip) {
			cancel()
			return nil, 0, nil, ErrHNMiniMaxArchiveDestination
		}
	}
	pinned := net.JoinHostPort(addresses[0].Unmap().String(), "443")
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != net.JoinHostPort(u.Hostname(), "443") {
			return nil, ErrHNMiniMaxArchiveDestination
		}
		if options.mediaDial != nil {
			return options.mediaDial(ctx, network, pinned)
		}
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, pinned)
	}
	tr := &http.Transport{Proxy: nil, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, DialContext: dial, TLSClientConfig: &tls.Config{ServerName: u.Hostname(), RootCAs: options.mediaRoots, MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 16 << 10}
	client := &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		cancel()
		tr.CloseIdleConnections()
		return nil, 0, nil, ErrHNMiniMaxArchiveMedia
	}
	response, e := client.Do(req) // No API Authorization/cookies, redirect, retry or proxy.
	if e != nil {
		cancel()
		tr.CloseIdleConnections()
		return nil, 0, nil, ErrHNMiniMaxArchiveMedia
	}
	cleanup := func() { response.Body.Close(); tr.CloseIdleConnections(); cancel() }
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "video/mp4" || response.ContentLength <= 0 || response.ContentLength > HNArchiveMaxBytes || len(response.TransferEncoding) != 0 || response.Header.Get("Content-Encoding") != "" || response.Header.Get("Content-Length") == "" {
		cleanup()
		return nil, 0, nil, ErrHNMiniMaxArchiveMedia
	}
	size := response.ContentLength
	prefix := make([]byte, min(size, 512))
	if _, e = io.ReadFull(response.Body, prefix); e != nil || http.DetectContentType(prefix) != "video/mp4" {
		cleanup()
		return nil, 0, nil, ErrHNMiniMaxArchiveMedia
	}
	source := io.MultiReader(bytes.NewReader(prefix), io.LimitReader(response.Body, size-int64(len(prefix))+1))
	return hnMiniMaxMediaReader{ctx, source}, size, cleanup, nil
}

// The retry lookup is read-only before the audited accepted-task gate; Open must
// not reconcile unknown/nonaccepted tasks or unrelated SUBMITTING records.
func hnMiniMaxArchiveRetryBinding(root, projectID, jobID string) (string, error) {
	parent, e := filepath.Abs(root)
	if e != nil {
		return "", ErrHNMiniMaxArchivePreflight
	}
	parent, e = filepath.EvalSymlinks(parent)
	if e != nil {
		return "", ErrHNMiniMaxArchivePreflight
	}
	guard := foundation.Workspace{Root: filepath.Join(parent, projectID), ProjectID: projectID}
	path, e := guard.Resolve("metadata/hn-extension.sqlite")
	if e != nil {
		return "", ErrHNMiniMaxArchivePreflight
	}
	path = filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return "", ErrHNMiniMaxArchivePreflight
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var rawJob, rawResult string
	var job foundation.ArchiveJob
	var result foundation.Result
	if db.QueryRow("SELECT data FROM archive_jobs WHERE id=? AND project_id=?", jobID, projectID).Scan(&rawJob) != nil || json.Unmarshal([]byte(rawJob), &job) != nil || db.QueryRow("SELECT data FROM results WHERE id=? AND project_id=?", job.ResultID, projectID).Scan(&rawResult) != nil || json.Unmarshal([]byte(rawResult), &result) != nil || job.ID != jobID || job.ProjectID != projectID || result.ArchiveJobID != job.ID || result.ProjectID != projectID || result.GenerationID != job.GenerationID || !validHNReferenceName(result.TaskBindingID) {
		return "", ErrHNMiniMaxArchivePreflight
	}
	return result.TaskBindingID, nil
}
func hnMiniMaxArchiveTarget(generation, result string) string {
	return "generated/" + generation + "/" + result + "/media" + HNVideoExtension("video/mp4")
}
func hnMiniMaxProviderArchive(w *foundation.Workspace, g foundation.Generation, b foundation.TaskBinding, duration int) (foundation.Result, foundation.ArchiveJob, error) {
	var result foundation.Result
	var job foundation.ArchiveJob
	rows, e := w.List("results")
	if e != nil {
		return result, job, ErrHNMiniMaxArchiveConflict
	}
	count := 0
	for _, raw := range rows {
		var r foundation.Result
		if json.Unmarshal(raw, &r) != nil {
			return result, job, ErrHNMiniMaxArchiveConflict
		}
		if r.TaskBindingID != b.ID && !(r.GenerationID == g.ID && (r.TaskBindingID != "" || r.ProviderResultID != "")) {
			continue
		}
		count++
		if count > 1 || !validHNReferenceName(r.ID) || r.SchemaVersion != foundation.SchemaVersion || r.ProjectID != g.ProjectID || r.GenerationID != g.ID || r.TaskBindingID != b.ID || r.ProviderResultID != b.ProviderTaskID || r.ResultKind != "video" || r.SourceURLRef != "" || r.DurationSeconds == nil || *r.DurationSeconds != float64(duration) {
			return result, job, ErrHNMiniMaxArchiveConflict
		}
		result = r
	}
	if count == 0 {
		return result, job, nil
	}
	jobs, e := w.List("archive_jobs")
	if e != nil {
		return result, job, ErrHNMiniMaxArchiveConflict
	}
	count = 0
	for _, raw := range jobs {
		var j foundation.ArchiveJob
		if json.Unmarshal(raw, &j) != nil {
			return result, job, ErrHNMiniMaxArchiveConflict
		}
		if j.ResultID != result.ID && j.ID != result.ArchiveJobID {
			continue
		}
		count++
		if count > 1 || j.ID != result.ArchiveJobID || !validHNReferenceName(j.ID) || j.ProjectID != g.ProjectID || j.SchemaVersion != foundation.SchemaVersion || j.GenerationID != g.ID || j.ResultID != result.ID || j.TargetRelativePath != hnMiniMaxArchiveTarget(g.ID, result.ID) || j.ExpectedMime != "video/mp4" {
			return result, job, ErrHNMiniMaxArchiveConflict
		}
		job = j
	}
	if result.ArchiveJobID != "" && count != 1 || result.ArchiveJobID == "" && count != 0 {
		return result, job, ErrHNMiniMaxArchiveConflict
	}
	if job.ID == "" && result.Status != "RECEIVED" {
		return result, job, ErrHNMiniMaxArchiveConflict
	}
	if job.ID != "" {
		switch job.Status {
		case "ARCHIVED":
			if result.Status != "ARCHIVED" || result.ArchivedRelativePath != job.TargetRelativePath || result.SHA256 != job.ActualSHA256 || !hnHash.MatchString(result.SHA256) || result.ByteLength != job.ActualBytes || result.ByteLength <= 0 || result.ByteLength > HNArchiveMaxBytes {
				return result, job, ErrHNMiniMaxArchiveConflict
			}
		case "PENDING":
			if result.Status != "RECEIVED" {
				return result, job, ErrHNMiniMaxArchiveConflict
			}
		case "FAILED", "INCONSISTENT":
			if result.Status != "ARCHIVE_"+job.Status {
				return result, job, ErrHNMiniMaxArchiveConflict
			}
		default:
			return result, job, ErrHNMiniMaxArchiveConflict
		}
	}
	return result, job, nil
}
func hnMiniMaxProviderFacts(r foundation.Result, j foundation.ArchiveJob) HNArchiveFacts {
	return HNArchiveFacts{ResultID: r.ID, GenerationID: r.GenerationID, ArchiveJobID: j.ID, ResultStatus: r.Status, ArchiveStatus: j.Status, ArchivedRelativePath: r.ArchivedRelativePath, SHA256: r.SHA256, ByteLength: r.ByteLength, MimeType: j.ExpectedMime}
}

// Compiled but disabled: no production caller, route/UI or background worker.
func ArchiveMiniMaxH3OfficialResult(ctx context.Context, root, projectID, bindingID, ownerID string, resolver HNMiniMaxChannelResolver) (HNArchiveFacts, error) {
	return archiveHNMiniMax(ctx, root, projectID, bindingID, "", ownerID, resolver, hnMiniMaxArchiveOptions{})
}
func RetryMiniMaxH3OfficialResultArchive(ctx context.Context, root, projectID, jobID, ownerID string, resolver HNMiniMaxChannelResolver) (HNArchiveFacts, error) {
	return archiveHNMiniMax(ctx, root, projectID, "", jobID, ownerID, resolver, hnMiniMaxArchiveOptions{})
}
func archiveHNMiniMax(ctx context.Context, root, projectID, bindingID, retryID, ownerID string, resolver HNMiniMaxChannelResolver, options hnMiniMaxArchiveOptions) (facts HNArchiveFacts, err error) {
	defer func() {
		if recover() != nil {
			err = ErrHNMiniMaxArchiveFailed
		}
	}()
	if ctx == nil || resolver == nil || !validHNReferenceName(projectID) || !hnMiniMaxSafeTaskID(ownerID) || (retryID == "" && !validHNReferenceName(bindingID)) || (retryID != "" && (!validHNReferenceName(retryID) || bindingID != "")) {
		return facts, ErrHNMiniMaxArchivePreflight
	}
	if ctx.Err() != nil {
		return facts, ErrHNMiniMaxArchiveMedia
	}
	// Conservative per-process serialization includes bounded I/O. This ensures
	// repeated/concurrent create calls inspect the completed job before any GET.
	// Multiprocess availability and untrusted concurrent store writes are not claimed.
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	if retryID != "" {
		bindingID, err = hnMiniMaxArchiveRetryBinding(root, projectID, retryID)
		if err != nil {
			return facts, err
		}
	}
	g, b, e := hnMiniMaxPollSnapshot(root, projectID, bindingID)
	if e != nil {
		return facts, ErrHNMiniMaxArchivePreflight
	}
	if b.ErrorClass != "" {
		return facts, ErrHNMiniMaxPollStateConflict
	}
	channel, e := resolveHNMiniMax(ownerID, g.ConnectionID, resolver)
	if e != nil {
		return facts, ErrHNMiniMaxArchivePreflight
	}
	body, e := hnMiniMaxRequestBody(g)
	var expected hnMiniMaxRequest
	if e != nil || json.Unmarshal(body, &expected) != nil {
		return facts, ErrHNMiniMaxArchivePreflight
	}
	w, e := foundation.Open(root, projectID)
	if e != nil {
		return facts, ErrHNMiniMaxArchiveConflict
	}
	defer w.Close()
	result, job, e := hnMiniMaxProviderArchive(w, g, b, expected.Duration)
	if e != nil {
		return facts, e
	}
	if retryID != "" && job.ID != retryID {
		return facts, ErrHNMiniMaxArchiveConflict
	}
	if job.ID != "" {
		facts = hnMiniMaxProviderFacts(result, job)
		if job.Status == "ARCHIVED" {
			return facts, nil
		} // Open verified final bytes/receipt.
		if retryID == "" {
			return facts, ErrHNMiniMaxArchiveRetryRequired
		}
	}
	location, e := hnMiniMaxArchiveQuery(ctx, channel.APIKey, b, expected, options.query)
	if e != nil {
		return facts, e
	}
	decoded, decodeErr := url.QueryUnescape(location)
	if decodeErr != nil || strings.Contains(location, channel.APIKey) || strings.Contains(decoded, channel.APIKey) {
		return facts, ErrHNMiniMaxArchiveDestination
	}
	source, size, cleanup, e := hnMiniMaxArchiveMedia(ctx, location, options)
	location = "" // No durable URL/ref, logging or public return.
	if e != nil {
		return facts, e
	}
	defer cleanup()
	currentG, currentB, e := hnMiniMaxPollSnapshot(root, projectID, bindingID)
	if e != nil || !reflect.DeepEqual(g, currentG) || !reflect.DeepEqual(b, currentB) {
		return facts, ErrHNMiniMaxPollStateConflict
	}
	currentR, currentJ, e := hnMiniMaxProviderArchive(w, g, b, expected.Duration)
	if e != nil || !reflect.DeepEqual(currentR, result) || !reflect.DeepEqual(currentJ, job) {
		return facts, ErrHNMiniMaxArchiveConflict
	}
	if result.ID == "" {
		duration := float64(expected.Duration)
		result, e = w.CreateResult(foundation.Result{GenerationID: g.ID, TaskBindingID: b.ID, ProviderResultID: b.ProviderTaskID, ResultKind: "video", DurationSeconds: &duration})
		if e != nil {
			return facts, ErrHNMiniMaxArchiveFailed
		}
		facts = hnMiniMaxProviderFacts(result, job)
	}
	if job.ID == "" {
		job, e = w.CreateArchive(result.ID, hnMiniMaxArchiveTarget(g.ID, result.ID), "video/mp4")
		if e != nil {
			return facts, ErrHNMiniMaxArchiveFailed
		}
		facts = hnMiniMaxProviderFacts(result, job)
	}
	runErr := w.RunArchive(job.ID, source, size, "")
	if hnReadRecord(w, "results", result.ID, &result) != nil || hnReadRecord(w, "archive_jobs", job.ID, &job) != nil {
		return facts, ErrHNMiniMaxArchiveFailed
	}
	facts = hnMiniMaxProviderFacts(result, job)
	if runErr != nil {
		return facts, ErrHNMiniMaxArchiveFailed
	}
	if _, _, e = hnMiniMaxProviderArchive(w, g, b, expected.Duration); e != nil || job.Status != "ARCHIVED" {
		return facts, ErrHNMiniMaxArchiveConflict
	}
	return facts, nil
}
