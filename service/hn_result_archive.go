package service

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

const HNArchiveMaxBytes int64 = 64 << 20

var ErrHNArchiveInput = errors.New("invalid local archive input")
var ErrHNArchiveFailed = errors.New("local archive failed; explicit same-job retry required")

func HNVideoExtension(mime string) string {
	switch mime {
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	}
	return ""
}

type HNArchiveFacts struct {
	ResultID             string `json:"resultId"`
	GenerationID         string `json:"generationId"`
	ArchiveJobID         string `json:"archiveJobId"`
	ResultStatus         string `json:"resultStatus"`
	ArchiveStatus        string `json:"archiveStatus"`
	ArchivedRelativePath string `json:"archivedRelativePath"`
	SHA256               string `json:"sha256"`
	ByteLength           int64  `json:"byteLength"`
	MimeType             string `json:"mimeType"`
}

func hnReadRecord(w *foundation.Workspace, kind, id string, out any) error {
	raw, err := w.Read(kind, id)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS. The supplied ID is an
// explicit attachment owner, never inferred from Canvas/provider task metadata.
// Empty retryID creates a local Result; nonempty retryID only reuses that job.
func ArchiveLocalResult(root, projectID, generationID, retryID, mimeType string, source io.Reader, byteLength int64, sha256 string) (HNArchiveFacts, error) {
	if !validHNReferenceName(projectID) || HNVideoExtension(mimeType) == "" || source == nil || byteLength <= 0 || byteLength > HNArchiveMaxBytes || !hnHash.MatchString(sha256) || (retryID == "" && !validHNReferenceName(generationID)) || (retryID != "" && (!validHNReferenceName(retryID) || generationID != "")) {
		return HNArchiveFacts{}, ErrHNArchiveInput
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNArchiveFacts{}, err
	}
	defer w.Close()
	var result foundation.Result
	var job foundation.ArchiveJob
	if retryID != "" {
		if hnReadRecord(w, "archive_jobs", retryID, &job) != nil || job.ProjectID != projectID || job.ExpectedMime != mimeType || hnReadRecord(w, "results", job.ResultID, &result) != nil || result.ProjectID != projectID || result.GenerationID != job.GenerationID || result.ArchiveJobID != job.ID || result.ResultKind != "video" || result.TaskBindingID != "" || result.ProviderResultID != "" || result.SourceURLRef != "" {
			return HNArchiveFacts{}, ErrHNArchiveInput
		}
		generationID = job.GenerationID
	}
	var generation foundation.Generation
	// Open reconciles and validates frozen hashes/reference bytes before any write.
	if hnReadRecord(w, "generations", generationID, &generation) != nil || generation.ProjectID != projectID || !generation.Frozen {
		return HNArchiveFacts{}, ErrHNArchiveInput
	}
	if retryID == "" {
		result, err = w.CreateResult(foundation.Result{GenerationID: generationID, ResultKind: "video"})
		if err != nil {
			return HNArchiveFacts{}, err
		}
		job, err = w.CreateArchive(result.ID, "generated/"+generationID+"/"+result.ID+"/media"+HNVideoExtension(mimeType), mimeType)
		if err != nil {
			return HNArchiveFacts{ResultID: result.ID, GenerationID: generationID}, err
		}
	}
	runErr := w.RunArchive(job.ID, source, byteLength, sha256)
	if err = hnReadRecord(w, "archive_jobs", job.ID, &job); err != nil {
		return HNArchiveFacts{}, err
	}
	if err = hnReadRecord(w, "results", result.ID, &result); err != nil {
		return HNArchiveFacts{}, err
	}
	facts := HNArchiveFacts{result.ID, generationID, job.ID, result.Status, job.Status, result.ArchivedRelativePath, result.SHA256, result.ByteLength, job.ExpectedMime}
	if runErr != nil {
		return facts, ErrHNArchiveFailed
	}
	return facts, nil
}
