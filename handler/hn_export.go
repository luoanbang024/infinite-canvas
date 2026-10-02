package handler

import (
	"errors"
	"github.com/tigerowo/infinite-canvas/service"
	"net/http"
)

func HNExportSequence(w http.ResponseWriter, r *http.Request, projectID, sequenceID string) {
	root, ok := hnEditorialRequest(w, r, &struct{}{})
	if !ok {
		return
	}
	data, err := service.ExportLocalSequence(root, projectID, sequenceID)
	if errors.Is(err, service.ErrHNExportInput) {
		FailWithStatus(w, 400, "HN 导出标识无效")
		return
	}
	if err != nil {
		FailWithStatus(w, 409, "HN 导出未完成，请检查序列及本地归档完整性")
		return
	}
	OK(w, data)
}
