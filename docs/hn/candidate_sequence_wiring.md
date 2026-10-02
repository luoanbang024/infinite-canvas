# HN local Candidate / selection / SequenceItem / reorder wiring (R7)

Base: f3babca6bcdf96f1f692c23bd5c145f06e534973. Feature: feature/p0-b-r7-candidate-sequence. Complete local execution, pending GPT Review / Audit; not audited or integrated.

## Ownership and arrival

Candidate ensure opens/reconciles the Workspace and validates project-owned Shot/Result/frozen same-Shot Generation/ArchiveJob. Result and Job must both be ARCHIVED with matching owner IDs, SHA-256, positive byteLength and nonempty matching project-relative path. Existing reconcile verifies archived file/receipt bytes. Failed, missing, inconsistent and unbound inputs are rejected.

Candidate identity is project + exact ShotID + ResultID. One match returns all original metadata unchanged; more than one match rejects CANDIDATE_IDENTITY_CONFLICT (409). Label is bounded non-secret display metadata (empty defaults to Candidate), never identity. Shared HN in-process writer serialization is retained; no DB uniqueness migration or cross-process guarantee.

Candidate arrival never selects. Select is a separate explicit command and revalidates durable facts. A late B preserves current A selection and A placement. Selecting B does not alter A Candidate or any placement. Add requires the current selected archived Candidate; every explicit add creates a fresh stable SequenceItem, including intentional repeated Candidate placements.

## Reorder and narrowly authorized foundation fix

Reorder accepts exactly the full current sequence set in desired order. Missing, duplicate, unsafe, foreign and oversized lists reject before writes. Max 256 items; no hidden mapping. An explicit empty list for an empty safe sequence is supported. Atomic foundation Reorder performs the write; response items follow the requested order.

The original audited Reorder also assigned UpdatedAt. An isolated reproduction plus failing full-field regression established FOUNDATION_API_GAP_REVIEW_REQUIRED. After explicit user authorization, exactly that assignment was removed from records.go, attributed separately as OUR_VERIFICATION_FIX; no other existing foundation line/file changed. The new regression compares every persisted JSON field except OrderIndex, including UpdatedAt and SchemaVersion, across reordered, no-op and reopened states. No timestamp hiding or direct DB repair occurs in production code.

Remove the local verification patch only when a formally adopted upstream baseline contains equivalent behavior; retain the only-OrderIndex contract.

## Local API

- POST /api/hn/projects/:projectId/shots/:shotId/candidates/ensure — {resultId,label} -> Candidate facts.
- POST /api/hn/projects/:projectId/shots/:shotId/candidates/:candidateId/select — {} -> {shotId,selectedCandidateId}.
- POST /api/hn/projects/:projectId/sequences/:sequenceId/items — {candidateId} -> placement facts.
- POST /api/hn/projects/:projectId/sequences/:sequenceId/reorder — {sequenceItemIds:[...]} -> ordered placement facts.

All four reuse actual loopback peer + loopback Host/Origin, X-HN-Local-Request: 1, HN_PROJECTS_ROOT and POST-only local CORS options. Body: JSON object, 64 KiB limit, unknown/trailing/non-object JSON rejected. IDs use existing safe-name validation. No new Auth token. Error envelope remains code/data/msg with controlled non-secret messages.

## Frontend

local-editorial exposes ensureCandidate/selectCandidate/addSequenceItem/reorderSequence. Each uses validated loopback discovery and credentials omit. Returned identity/ownership/status/timestamps are validated. reorderSequence accepts all current items in desired order, snapshots them before discovery, sends only sequenceItemIds and rejects any response field change except orderIndex. Thus callers retain ownership/timestamp facts as evidence, rather than trusting a changed response.

commitArchivedResultToSequence snapshots input, validates before writes and executes ensure -> explicit select -> explicit add. Default sequence main is explicit and caller-overridable. Ensure response may not claim implicit selection. Individual calls remain available. No Canvas state, media read, Generation, Result/archive creation, Provider or editor dependency is accepted by this adapter. No new UI is claimed.

## Verification and practical limits

Root Go, Bridge, all 53 frontend tests, independent typecheck, production build and final production-router/local-adapter smoke pass. Isolated smoke uses existing R6 prepare/freeze and R5 exact local archive to construct two same-Shot alternatives, then proves selection separation, late-arrival preservation, repeat placement identity, reordered/full-field facts, no-op, backend restart/reopen and unchanged prerequisite lists. Counts: Shot 1, Generation 2, Result 2, ArchiveJob 2, Candidate 2, SequenceItem 3, TaskBinding 0. All services stopped.

Fixture: 24-byte synthetic MP4 signature, not a codec/playback/handoff claim. Explicit add intentionally creates a fresh placement and is not idempotent after an uncertain response. Cross-process writers, real user UI/codec/handoff and later export/Provider submit remain separate review work. Existing R5 reconciliation/archival limitations are retained.

Generation SourceBaseline remains ff32dc249811130a3db69be456e295be100b6e9f; no historical rewrite. Other foundation/R3–R6/Provider/Auth/upstream DB/Canvas Core/Node Core/dependencies/lockfiles unchanged. No editor export/import, Jianying changes, Provider calls, remote media download, external push or our-main merge.
