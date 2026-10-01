// Package foundation is HN's local, account- and provider-neutral production layer.
// It has no generation submit operation or network client.
package foundation

import "encoding/json"

const SchemaVersion = 1

type Identity struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	SchemaVersion int    `json:"schemaVersion"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}
type ReferenceVersion struct {
	Identity
	LogicalReferenceID string `json:"logicalReferenceId"`
	RelativePath       string `json:"relativePath"`
	ByteLength         int64  `json:"byteLength"`
	SHA256             string `json:"sha256"`
	MimeType           string `json:"mimeType"`
	Kind               string `json:"kind"`
	SourceDescription  string `json:"sourceDescription,omitempty"`
	TransformSummary   string `json:"transformSummary,omitempty"`
}
type ReferenceBinding struct {
	ReferenceVersionID string `json:"referenceVersionId"`
	SHA256             string `json:"sha256"`
	Role               string `json:"role"`
}
type Generation struct {
	Identity
	NodeID              string             `json:"nodeId,omitempty"`
	ShotID              string             `json:"shotId,omitempty"`
	PromptSnapshot      string             `json:"promptSnapshot"`
	Protocol            string             `json:"protocol,omitempty"`
	ProviderIdentity    string             `json:"providerIdentity,omitempty"`
	Model               string             `json:"model,omitempty"`
	Parameters          json.RawMessage    `json:"parameters"`
	ReferenceBindings   []ReferenceBinding `json:"referenceBindings"`
	ConnectionID        string             `json:"connectionId,omitempty"`
	CredentialRef       string             `json:"credentialRef,omitempty"`
	RequestSnapshotRef  string             `json:"requestSnapshotRef,omitempty"`
	RequestSnapshotHash string             `json:"requestSnapshotHash,omitempty"`
	SourceBaseline      string             `json:"sourceBaseline"`
	Status              string             `json:"status"`
	SubmissionState     string             `json:"submissionState"`
	Frozen              bool               `json:"frozen"`
	FrozenHash          string             `json:"frozenHash,omitempty"`
}
type TaskBinding struct {
	Identity
	GenerationID        string `json:"generationId"`
	ConnectionID        string `json:"connectionId,omitempty"`
	UpstreamLocalTaskID string `json:"upstreamLocalTaskId,omitempty"`
	ProviderTaskID      string `json:"providerTaskId,omitempty"`
	Protocol            string `json:"protocol,omitempty"`
	ProviderIdentity    string `json:"providerIdentity,omitempty"`
	BindingState        string `json:"bindingState"`
	LastPolledAt        string `json:"lastPolledAt,omitempty"`
	ErrorClass          string `json:"errorClass,omitempty"`
}
type Result struct {
	Identity
	GenerationID         string   `json:"generationId"`
	TaskBindingID        string   `json:"taskBindingId,omitempty"`
	ProviderResultID     string   `json:"providerResultId,omitempty"`
	ResultKind           string   `json:"resultKind"`
	SourceURLRef         string   `json:"sourceUrlRef,omitempty"`
	ReceivedAt           string   `json:"receivedAt"`
	ArchiveJobID         string   `json:"archiveJobId,omitempty"`
	ArchivedRelativePath string   `json:"archivedRelativePath,omitempty"`
	SHA256               string   `json:"sha256,omitempty"`
	ByteLength           int64    `json:"byteLength"`
	DurationSeconds      *float64 `json:"duration,omitempty"`
	Status               string   `json:"status"`
}
type ArchiveJob struct {
	Identity
	ResultID           string `json:"resultId"`
	GenerationID       string `json:"generationId"`
	TargetRelativePath string `json:"targetRelativePath"`
	Status             string `json:"status"`
	AttemptCount       int    `json:"attemptCount"`
	LastError          string `json:"lastError,omitempty"`
	StartedAt          string `json:"startedAt,omitempty"`
	CompletedAt        string `json:"completedAt,omitempty"`
	ExpectedMime       string `json:"expectedMime,omitempty"`
	ActualSHA256       string `json:"actualSha256,omitempty"`
	ActualBytes        int64  `json:"actualBytes"`
}
type Shot struct {
	Identity
	Label               string `json:"label"`
	SourceNodeID        string `json:"sourceNodeId,omitempty"`
	SelectedCandidateID string `json:"selectedCandidateId,omitempty"`
}
type Candidate struct {
	Identity
	ShotID             string `json:"shotId"`
	GenerationID       string `json:"generationId"`
	ResultID           string `json:"resultId"`
	Label              string `json:"label,omitempty"`
	AvailabilityStatus string `json:"availabilityStatus"`
}
type SequenceItem struct {
	Identity
	SequenceID  string `json:"sequenceId"`
	OrderIndex  int    `json:"orderIndex"`
	ShotID      string `json:"shotId"`
	CandidateID string `json:"candidateId"`
	ResultID    string `json:"resultId"`
}
type ManifestItem struct {
	ExportID        string   `json:"exportId"`
	SequenceIndex   int      `json:"sequenceIndex"`
	SequenceItemID  string   `json:"sequenceItemId"`
	ShotID          string   `json:"shotId"`
	CandidateID     string   `json:"candidateId"`
	ResultID        string   `json:"resultId"`
	RelativePath    string   `json:"relativePath"`
	SHA256          string   `json:"sha256"`
	ByteLength      int64    `json:"byteLength"`
	DurationSeconds *float64 `json:"duration,omitempty"`
}
type ExportManifest struct {
	SchemaVersion int            `json:"schemaVersion"`
	ProjectID     string         `json:"projectId"`
	ExportID      string         `json:"exportId"`
	SequenceID    string         `json:"sequenceId"`
	Items         []ManifestItem `json:"items"`
}
