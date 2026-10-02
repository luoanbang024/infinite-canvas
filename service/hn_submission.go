package service

import (
	"context"
	"errors"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var ErrHNSubmissionInput = errors.New("invalid local submission input")
var ErrHNSubmissionUnknown = errors.New("submission outcome unknown; manual reconciliation required; retry requires a new generation ID")

type SubmissionAcceptance struct {
	UpstreamLocalTaskID string
	ProviderTaskID      string
}

// No default adapter, credential resolver, network client or production route.
type SubmissionTransport interface {
	Submit(context.Context, foundation.Generation) (SubmissionAcceptance, error)
}

func invokeHNSubmission(ctx context.Context, transport SubmissionTransport, g foundation.Generation) (a SubmissionAcceptance, err error) {
	defer func() {
		if recover() != nil {
			err = ErrHNSubmissionUnknown
		}
	}()
	return transport.Submit(ctx, g)
}

// SubmitFrozenGeneration serializes Open/Begin/transport/record/Close with the
// existing HN writer. The transaction is the durable gate; the lock prevents an
// overlapping service Open from reconciling a live same-process attempt.
// Injected transport must not re-enter HN writer operations.
func SubmitFrozenGeneration(ctx context.Context, root, projectID, generationID string, transport SubmissionTransport) (foundation.TaskBinding, error) {
	if ctx == nil || transport == nil || !validHNReferenceName(projectID) || !validHNReferenceName(generationID) {
		return foundation.TaskBinding{}, ErrHNSubmissionInput
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	if err := ctx.Err(); err != nil {
		return foundation.TaskBinding{}, err
	}
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return foundation.TaskBinding{}, err
	}
	defer w.Close()
	g, b, err := w.BeginSubmission(generationID)
	if err != nil {
		return foundation.TaskBinding{}, err
	}
	a, err := invokeHNSubmission(ctx, transport, g)
	if err == nil {
		var bound foundation.TaskBinding
		bound, err = w.RecordSubmissionAccepted(g.ID, b.ID, a.UpstreamLocalTaskID, a.ProviderTaskID)
		if err == nil {
			return bound, nil
		}
	}
	// Never persist or return arbitrary transport error/panic text or secret data.
	// If this write fails, durable SUBMITTING is reconciled on the next Open.
	if w.MarkSubmissionUnknown(g.ID) != nil {
		return b, errors.Join(ErrHNSubmissionUnknown, errors.New("unknown-state persistence failed; reopen required"))
	}
	return b, ErrHNSubmissionUnknown
}
