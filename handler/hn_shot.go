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

func HNEnsureShot(w http.ResponseWriter, r *http.Request, projectID string) {
	if !hnLocalRequest(w, r) {
		return
	}
	if r.Header.Get("X-HN-Local-Request") != "1" {
		FailWithStatus(w, http.StatusForbidden, "HN Shot 需要显式本机请求标记")
		return
	}
	root := strings.TrimSpace(os.Getenv("HN_PROJECTS_ROOT"))
	if root == "" {
		FailWithStatus(w, http.StatusServiceUnavailable, "HN Shot 未启用：未配置 HN_PROJECTS_ROOT")
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		FailWithStatus(w, http.StatusBadRequest, "HN Shot 只接受 JSON")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		SourceNodeID string `json:"sourceNodeId"`
		Label        string `json:"label"`
	}
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		FailWithStatus(w, http.StatusBadRequest, "HN Shot 输入无效或含不支持的字段")
		return
	}
	shot, err := service.EnsureShotForSourceNode(root, projectID, input.SourceNodeID, input.Label)
	if errors.Is(err, service.ErrHNShotInput) {
		FailWithStatus(w, http.StatusBadRequest, "HN Shot 需要有效项目、非空来源节点及非敏感标签")
		return
	}
	if errors.Is(err, service.ErrHNShotIdentityConflict) {
		FailWithStatus(w, http.StatusConflict, "SHOT_IDENTITY_CONFLICT")
		return
	}
	if err != nil {
		FailWithStatus(w, http.StatusInternalServerError, "HN Shot 失败，请检查项目目录配置与完整性")
		return
	}
	OK(w, shot)
}
