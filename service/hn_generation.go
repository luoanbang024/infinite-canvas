package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

// SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_BASELINE_ADOPTION.
// This reviewed integration baseline never replaces the foundation's historical default.
const HNGenerationSourceBaseline = "418ffbde3dbea33d374356588cb672336ec38353"

var ErrHNGenerationInput = errors.New("invalid HN generation prepare input")

type HNGenerationPrepareInput struct {
	NodeID            string                        `json:"nodeId"`
	PromptSnapshot    string                        `json:"promptSnapshot"`
	Protocol          string                        `json:"protocol,omitempty"`
	ProviderIdentity  string                        `json:"providerIdentity,omitempty"`
	Model             string                        `json:"model,omitempty"`
	Parameters        json.RawMessage               `json:"parameters"`
	ReferenceBindings []foundation.ReferenceBinding `json:"referenceBindings"`
	ConnectionID      string                        `json:"connectionId,omitempty"`
	SourceBaseline    string                        `json:"sourceBaseline"`
}

type HNPreparedGeneration struct {
	GenerationID      string                        `json:"generationId"`
	ProjectID         string                        `json:"projectId"`
	NodeID            string                        `json:"nodeId"`
	SourceBaseline    string                        `json:"sourceBaseline"`
	Frozen            bool                          `json:"frozen"`
	FrozenHash        string                        `json:"frozenHash"`
	Status            string                        `json:"status"`
	SubmissionState   string                        `json:"submissionState"`
	ReferenceBindings []foundation.ReferenceBinding `json:"referenceBindings"`
	CreatedAt         string                        `json:"createdAt"`
}

var hnUnsafeText = regexp.MustCompile(`(?i)(https?://|data:|bearer\s|-----BEGIN|sk-proj-|ghp_|github_pat_|api[_-]?key\s*[:=]|(?:token|password|secret|authorization|cookie|credential|signature)\s*[:=])`)
var hnHash = regexp.MustCompile(`^[a-f0-9]{64}$`)
var hnVideoParameterNames = map[string]bool{
	"size": true, "videoSeconds": true, "vquality": true, "videoMode": true,
	"videoNegativePrompt": true, "videoMultiShot": true, "videoShotType": true,
	"videoGenerateAudio": true, "videoWatermark": true, "videoCharacterOrientation": true,
}

// R4 accepts a closed business vocabulary, never an arbitrary AiConfig or URL bundle.
func validateHNPrepare(input HNGenerationPrepareInput) error {
	bad := func() error { return ErrHNGenerationInput }
	if strings.TrimSpace(input.NodeID) == "" || len(input.NodeID) > 128 || strings.TrimSpace(input.PromptSnapshot) == "" || len(input.PromptSnapshot) > 32768 || input.SourceBaseline != HNGenerationSourceBaseline {
		return bad()
	}
	for _, text := range []string{input.NodeID, input.PromptSnapshot, input.Protocol, input.ProviderIdentity, input.Model, input.ConnectionID} {
		if hnUnsafeText.MatchString(text) {
			return bad()
		}
	}
	for _, text := range []string{input.Protocol, input.ProviderIdentity, input.Model, input.ConnectionID} {
		if len(text) > 256 {
			return bad()
		}
	}
	var params map[string]json.RawMessage
	if json.Unmarshal(input.Parameters, &params) != nil || params == nil {
		return bad()
	}
	for name, raw := range params {
		if hnVideoParameterNames[name] {
			var value *string
			if json.Unmarshal(raw, &value) != nil || value == nil || len(*value) > 32768 || hnUnsafeText.MatchString(*value) {
				return bad()
			}
		} else if name == "videoMultiPrompt" {
			var shots []map[string]json.RawMessage
			if json.Unmarshal(raw, &shots) != nil || shots == nil || len(shots) > 64 {
				return bad()
			}
			for _, shot := range shots {
				if len(shot) != 2 {
					return bad()
				}
				for _, key := range []string{"prompt", "duration"} {
					var value *string
					if json.Unmarshal(shot[key], &value) != nil || value == nil || len(*value) > 32768 || hnUnsafeText.MatchString(*value) {
						return bad()
					}
				}
			}
		} else {
			return bad()
		}
	}
	if len(input.ReferenceBindings) > 32 {
		return bad()
	}
	for _, binding := range input.ReferenceBindings {
		if !validHNReferenceName(binding.ReferenceVersionID) || !hnHash.MatchString(binding.SHA256) || (binding.Role != "reference" && binding.Role != "firstFrame" && binding.Role != "lastFrame") {
			return bad()
		}
	}
	return nil
}

func PrepareLocalGeneration(root, projectID string, input HNGenerationPrepareInput) (HNPreparedGeneration, error) {
	if !validHNReferenceName(projectID) {
		return HNPreparedGeneration{}, ErrHNGenerationInput
	}
	if err := validateHNPrepare(input); err != nil {
		return HNPreparedGeneration{}, err
	}
	// Reuse R3's writer lock: separate Open handles must not concurrently recover/write.
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return HNPreparedGeneration{}, err
	}
	defer w.Close()
	for _, binding := range input.ReferenceBindings {
		raw, e := w.Read("reference_versions", binding.ReferenceVersionID)
		var reference foundation.ReferenceVersion
		if e != nil || json.Unmarshal(raw, &reference) != nil || reference.ProjectID != projectID || reference.Kind != "image" || reference.SHA256 != binding.SHA256 {
			return HNPreparedGeneration{}, ErrHNGenerationInput
		}
	}
	bindings := input.ReferenceBindings
	if bindings == nil {
		bindings = []foundation.ReferenceBinding{}
	}
	g, err := w.CreateGeneration(foundation.Generation{
		NodeID: input.NodeID, PromptSnapshot: input.PromptSnapshot, Protocol: input.Protocol,
		ProviderIdentity: input.ProviderIdentity, Model: input.Model, Parameters: input.Parameters,
		ReferenceBindings: bindings, ConnectionID: input.ConnectionID, SourceBaseline: input.SourceBaseline,
	})
	if err != nil {
		return HNPreparedGeneration{}, fmt.Errorf("%w: request facts rejected", ErrHNGenerationInput)
	}
	g, err = w.FreezeGeneration(g.ID)
	if err != nil {
		return HNPreparedGeneration{}, err
	}
	return HNPreparedGeneration{g.ID, g.ProjectID, g.NodeID, g.SourceBaseline, g.Frozen, g.FrozenHash, g.Status, g.SubmissionState, g.ReferenceBindings, g.CreatedAt}, nil
}
