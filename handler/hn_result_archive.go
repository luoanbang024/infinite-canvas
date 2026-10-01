package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tigerowo/infinite-canvas/service"
)

// Bounded 64 MiB video, 64 KiB multipart overhead, 1 MiB file memory threshold.
// Filename is ignored; MIME determines only a service-owned constant extension.
func HNLocalResultArchive(w http.ResponseWriter, r *http.Request, projectID, generationID, retryID string) {
	if !hnLocalRequest(w, r) {
		return
	}
	if r.Header.Get("X-HN-Local-Request") != "1" {
		FailWithStatus(w, 403, "HN 归档需要显式本机请求标记")
		return
	}
	root := strings.TrimSpace(os.Getenv("HN_PROJECTS_ROOT"))
	if root == "" {
		FailWithStatus(w, 503, "HN 归档未启用：未配置 HN_PROJECTS_ROOT")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.HNArchiveMaxBytes+(64<<10))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		FailWithStatus(w, 400, "归档请求无效或超过 64 MiB 限制")
		return
	}
	defer r.MultipartForm.RemoveAll()
	values := r.MultipartForm.Value
	files := r.MultipartForm.File["file"]
	if len(values) != 3 || len(values["resultKind"]) != 1 || values["resultKind"][0] != "video" || len(values["mimeType"]) != 1 || len(values["sha256"]) != 1 || len(r.MultipartForm.File) != 1 || len(files) != 1 || files[0].Size <= 0 || files[0].Size > service.HNArchiveMaxBytes {
		FailWithStatus(w, 400, "需要单个非空本地视频及非敏感归档字段")
		return
	}
	mimeType := values["mimeType"][0]
	if service.HNVideoExtension(mimeType) == "" {
		FailWithStatus(w, 400, "R5 仅支持 MP4/WebM 视频")
		return
	}
	file, err := files[0].Open()
	if err != nil {
		FailWithStatus(w, 400, "无法读取本地视频")
		return
	}
	defer file.Close()
	var prefix [512]byte
	n, err := file.Read(prefix[:])
	if err != nil && err != io.EOF {
		FailWithStatus(w, 400, "无法读取本地视频")
		return
	}
	if http.DetectContentType(prefix[:n]) != mimeType {
		FailWithStatus(w, 400, "视频签名与 MIME 不一致")
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		FailWithStatus(w, 400, "无法读取本地视频")
		return
	}
	facts, err := service.ArchiveLocalResult(root, projectID, generationID, retryID, mimeType, file, files[0].Size, values["sha256"][0])
	if errors.Is(err, service.ErrHNArchiveInput) {
		FailWithStatus(w, 400, "需要同项目已冻结 Generation 或有效本地 ArchiveJob 及准确字节声明")
		return
	}
	if errors.Is(err, service.ErrHNArchiveFailed) {
		writeJSONWithStatus(w, 422, response{Code: 1, Data: facts, Msg: "归档未完成；可使用返回的 ArchiveJob ID 显式重试"})
		return
	}
	if err != nil {
		FailWithStatus(w, 500, "本地归档失败，请检查项目目录配置与完整性")
		return
	}
	OK(w, facts)
}
