package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/tigerowo/infinite-canvas/service"
)

func HNPrepareGeneration(w http.ResponseWriter, r *http.Request, projectID string) {
	if !hnLocalRequest(w, r) {
		return
	}
	if r.Header.Get("X-HN-Local-Request") != "1" {
		FailWithStatus(w, http.StatusForbidden, "HN 准备需要显式本机请求标记")
		return
	}
	root := strings.TrimSpace(os.Getenv("HN_PROJECTS_ROOT"))
	if root == "" {
		FailWithStatus(w, http.StatusServiceUnavailable, "HN 准备未启用：未配置 HN_PROJECTS_ROOT")
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		FailWithStatus(w, http.StatusBadRequest, "HN 准备只接受 JSON")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input service.HNGenerationPrepareInput
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		FailWithStatus(w, http.StatusBadRequest, "HN 准备输入无效或含不支持的字段")
		return
	}
	generation, err := service.PrepareLocalGeneration(root, projectID, input)
	if errors.Is(err, service.ErrHNGenerationInput) {
		FailWithStatus(w, http.StatusBadRequest, "HN 准备需要非空提示词、已审核基线、非敏感业务参数及有效本地图片绑定")
		return
	}
	if err != nil {
		FailWithStatus(w, http.StatusInternalServerError, "HN 准备失败，请检查项目目录配置与完整性")
		return
	}
	OK(w, generation)
}
