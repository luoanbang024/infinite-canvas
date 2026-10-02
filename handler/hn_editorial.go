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

func hnEditorialRequest(w http.ResponseWriter, r *http.Request, input any) (string, bool) {
	if !hnLocalRequest(w, r) {
		return "", false
	}
	if r.Header.Get("X-HN-Local-Request") != "1" {
		FailWithStatus(w, 403, "HN 剪辑需要显式本机请求标记")
		return "", false
	}
	root := strings.TrimSpace(os.Getenv("HN_PROJECTS_ROOT"))
	if root == "" {
		FailWithStatus(w, 503, "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT")
		return "", false
	}
	content, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || content != "application/json" {
		FailWithStatus(w, 400, "HN 剪辑只接受 JSON")
		return "", false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	// Decode into a RawMessage first to reject null and non-object command shapes.
	var raw json.RawMessage
	if decoder.Decode(&raw) != nil || decoder.Decode(new(any)) != io.EOF || len(raw) == 0 || raw[0] != '{' {
		FailWithStatus(w, 400, "HN 剪辑输入无效")
		return "", false
	}
	command := json.NewDecoder(strings.NewReader(string(raw)))
	command.DisallowUnknownFields()
	if command.Decode(input) != nil {
		FailWithStatus(w, 400, "HN 剪辑输入含不支持字段")
		return "", false
	}
	return root, true
}

func hnEditorialResponse(w http.ResponseWriter, data any, err error) {
	if errors.Is(err, service.ErrHNEditorialInput) {
		FailWithStatus(w, 400, "HN 剪辑标识或输入无效")
		return
	}
	if errors.Is(err, service.ErrHNCandidateIdentity) || errors.Is(err, service.ErrHNEditorialOwnership) {
		FailWithStatus(w, 409, err.Error())
		return
	}
	if err != nil {
		FailWithStatus(w, 500, "HN 剪辑失败，请检查本地项目完整性")
		return
	}
	OK(w, data)
}

func HNEnsureCandidate(w http.ResponseWriter, r *http.Request, projectID, shotID string) {
	var input struct {
		ResultID string `json:"resultId"`
		Label    string `json:"label"`
	}
	root, ok := hnEditorialRequest(w, r, &input)
	if !ok {
		return
	}
	data, err := service.EnsureCandidateForArchivedResult(root, projectID, shotID, input.ResultID, input.Label)
	hnEditorialResponse(w, data, err)
}

func HNSelectCandidate(w http.ResponseWriter, r *http.Request, projectID, shotID, candidateID string) {
	root, ok := hnEditorialRequest(w, r, &struct{}{})
	if !ok {
		return
	}
	data, err := service.SelectLocalCandidate(root, projectID, shotID, candidateID)
	hnEditorialResponse(w, data, err)
}

func HNAddSequenceItem(w http.ResponseWriter, r *http.Request, projectID, sequenceID string) {
	var input struct {
		CandidateID string `json:"candidateId"`
	}
	root, ok := hnEditorialRequest(w, r, &input)
	if !ok {
		return
	}
	data, err := service.AddLocalSequenceItem(root, projectID, sequenceID, input.CandidateID)
	hnEditorialResponse(w, data, err)
}
