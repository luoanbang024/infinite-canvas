# HN local Candidate / selection / SequenceItem wiring (R7 incomplete)

Base: f3babca6bcdf96f1f692c23bd5c145f06e534973. Feature: feature/p0-b-r7-candidate-sequence. OUR_INTEGRATION_PATCH, pending review; not audited or integrated.

## Implemented boundary

Candidate ensure opens/reconciles the workspace, validates project/Shot/Result/frozen same-Shot Generation and archived ArchiveJob, including SHA-256, byteLength and relative path agreement. It uses the shared HN writer lock and audited foundation creation/selection/placement methods.

Candidate identity is project + exact ShotID + ResultID. One match returns original metadata unchanged, multiple matches reject CANDIDATE_IDENTITY_CONFLICT. Candidate label is bounded non-secret display metadata (empty defaults to Candidate); it never determines identity.

Candidate arrival never selects. Select is a separate explicit command and revalidates durable facts; changing selection never changes existing placements. Add requires the Candidate to be the Shot's current selected Candidate and creates a fresh SequenceItem on every explicit call. The frontend compound helper explicitly performs ensure -> select -> add; sequence defaults to main and is caller-overridable.

## Local API

- POST /api/hn/projects/:projectId/shots/:shotId/candidates/ensure — {resultId,label}.
- POST /api/hn/projects/:projectId/shots/:shotId/candidates/:candidateId/select — exact empty command object {}.
- POST /api/hn/projects/:projectId/sequences/:sequenceId/items — {candidateId}.

Actual loopback peer, loopback Host/Origin and X-HN-Local-Request: 1 are required. HN_PROJECTS_ROOT configures the runtime directory. JSON object body is limited to 64 KiB; unknown/trailing/non-object JSON is rejected. All identity inputs use existing safe-name validation. Error responses preserve the existing code/data/msg format and avoid filesystem detail. No new Auth token or remote media route is added.

## Proven foundation API gap

Execution section 5 requires affected work to stop for FOUNDATION_API_GAP_REVIEW_REQUIRED. Workspace.Reorder writes UpdatedAt as well as OrderIndex. An isolated one-item reorder of the same order reproduced only UpdatedAt changing. No public foundation operation safely restores the original timestamp. Returning a hidden old timestamp would misrepresent persisted data; direct SQLite writes would bypass audited invariants.

Reorder service/route/frontend adapter and reorder/full end-to-end proof are therefore pending review authorization. The external proposal removes only `i.UpdatedAt = timestamp()` inside Reorder and adds a field-invariance regression test. It is unapplied. Foundation bytes/semantics remain unchanged.

## Verification and limits

Root Go, Bridge, all 50 frontend tests, independent typecheck and production build pass. A production-router/local-adapter smoke builds two same-Shot frozen Generations and exact synthetic local MP4-signature archives via existing R6/R5 adapters, then checks arrival/selection separation, late alternative preservation, duplicate placement and backend restart/reopen. Counts: Shot 1, Generation 2, Result 2, ArchiveJob 2, Candidate 2, SequenceItem 3, TaskBinding 0. R7 operations do not create prerequisites or rewrite them.

The fixture proves local identity/archive boundaries, not video codec/playback or editor handoff. No browser UI is introduced. In-process serialization is not a cross-process uniqueness guarantee. Initial uncertain write response may still require reopening; explicit sequence add intentionally is not idempotent. Existing archive reconciliation policy is retained.

SourceBaseline remains ff32dc249811130a3db69be456e295be100b6e9f. R3–R6/foundation/Provider/Auth/upstream schema/dependencies/lockfiles remain unchanged. No Provider calls, remote downloads, editor export/import, Jianying changes, external push or our-main merge.
