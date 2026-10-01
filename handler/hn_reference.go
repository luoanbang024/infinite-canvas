package handler

import (
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/tigerowo/infinite-canvas/service"
)

// No common limit exists in the original local image import path. Limit the
// image to 16 MiB and the multipart request to that plus 64 KiB overhead.
const HNReferenceMaxBytes int64 = 16 << 20

func hnLoopbackHost(host string) bool {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || ip != nil && ip.IsLoopback()
}

func hnLocalRequest(w http.ResponseWriter, r *http.Request) bool {
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(remote)
	if err != nil || ip == nil || !ip.IsLoopback() || !hnLoopbackHost(r.Host) {
		FailWithStatus(w, http.StatusForbidden, "HN 参考冻结仅允许本机请求")
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		// Frontend/backend ports differ. Allow only explicit loopback origins;
		// forwarded headers never establish trust. No credentialed CORS.
		if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || !hnLoopbackHost(u.Host) || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			FailWithStatus(w, http.StatusForbidden, "HN 参考冻结不接受此页面来源")
			return false
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	return true
}

func HNReferenceOptions(w http.ResponseWriter, r *http.Request) {
	if !hnLocalRequest(w, r) {
		return
	}
	if r.Header.Get("Access-Control-Request-Method") != http.MethodPost {
		FailWithStatus(w, http.StatusForbidden, "HN 参考冻结只接受显式 POST")
		return
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-HN-Local-Request")
	w.WriteHeader(http.StatusNoContent)
}

func HNFreezeReference(w http.ResponseWriter, r *http.Request, projectID string) {
	if !hnLocalRequest(w, r) {
		return
	}
	if r.Header.Get("X-HN-Local-Request") != "1" {
		FailWithStatus(w, http.StatusForbidden, "HN 参考冻结需要显式本机请求标记")
		return
	}
	root := strings.TrimSpace(os.Getenv("HN_PROJECTS_ROOT"))
	if root == "" {
		FailWithStatus(w, http.StatusServiceUnavailable, "HN 本地参考冻结未启用：未配置 HN_PROJECTS_ROOT")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, HNReferenceMaxBytes+(64<<10))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		FailWithStatus(w, http.StatusBadRequest, "参考图片请求无效或超过 16 MiB 限制")
		return
	}
	defer r.MultipartForm.RemoveAll()
	logical, kind, files := r.MultipartForm.Value["logicalReferenceId"], r.MultipartForm.Value["kind"], r.MultipartForm.File["file"]
	if len(logical) != 1 || len(kind) != 1 || kind[0] != "image" || len(files) != 1 || len(r.MultipartForm.File) != 1 {
		FailWithStatus(w, http.StatusBadRequest, "需要一张图片、稳定参考标识及 kind=image")
		return
	}
	header := files[0]
	_, disposition, err := mime.ParseMediaType(header.Header.Get("Content-Disposition"))
	filename := disposition["filename"]
	if err != nil || filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\:\x00") || header.Size <= 0 || header.Size > HNReferenceMaxBytes {
		FailWithStatus(w, http.StatusBadRequest, "参考图片为空、文件名不安全或超过 16 MiB 限制")
		return
	}
	file, err := header.Open()
	if err != nil {
		FailWithStatus(w, http.StatusBadRequest, "无法读取所选本地图片")
		return
	}
	defer file.Close()
	var prefix [512]byte
	n, err := file.Read(prefix[:])
	if err != nil && err != io.EOF {
		FailWithStatus(w, http.StatusBadRequest, "无法读取所选本地图片")
		return
	}
	actualMIME := http.DetectContentType(prefix[:n])
	switch actualMIME {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/x-icon":
	default:
		FailWithStatus(w, http.StatusBadRequest, "参考必须是可识别的本地栅格图片")
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		FailWithStatus(w, http.StatusBadRequest, "无法读取所选本地图片")
		return
	}
	reference, err := service.SnapshotLocalReference(root, projectID, logical[0], actualMIME, file, header.Size)
	if errors.Is(err, service.ErrHNReferenceIdentity) {
		FailWithStatus(w, http.StatusBadRequest, "项目或参考标识不兼容；需审核 ID 映射")
		return
	}
	if err != nil {
		FailWithStatus(w, http.StatusInternalServerError, "本地参考冻结失败，请检查项目目录配置与完整性")
		return
	}
	OK(w, reference)
}
