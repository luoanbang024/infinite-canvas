# HN R4 Generation prepare/freeze

EXECUTION_ID = HN_AI_IC_P0_B_R4_GENERATION_PREPARE_FREEZE_WIRING
STATE = AUDITED / INTEGRATED; GPT_AUDIT = PASS.
R4_FEATURE_HEAD = f03a79522c2bc4e723502fee1297bf5b27c92269; R4_COMPLETION_SHA256 = f37292070ef501689b7a1c702d90fe096d8cd2ae516c480c8e93295a2cb1addd.
CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R4_AUDIT_CLOSEOUT_INTEGRATION; stable resulting SHA is recorded in external closeout evidence.
GENERATION_WIRING = PREPARE_FREEZE_ONLY; REAL_SUBMIT/TASKBINDING_WIRING/RESULT_ARCHIVE_WIRING = NONE.
PROVIDER_SELECTION = DEFERRED; REAL_PROVIDER_CALLS/PAID_CALLS = NONE.
SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_BASELINE_ADOPTION = YES.
BASE_OUR_COMMIT = 418ffbde3dbea33d374356588cb672336ec38353; IMPLEMENTATION_COMMIT = 92349f02c5a06594a003ba41e49409dc490f4239.

Canvas video intent -> exact local image ReferenceVersion bindings -> Generation -> FreezeGeneration -> PREPARED / not submitted. This is the audited internal prepare boundary; it is not inserted into the live Provider path and has no new button or user workflow. Call the builder with the same effective AiConfig and NodeGenerationContext as Canvas video generation and its optional cameraControl. The existing camera prompt helper supplies the prompt snapshot. Caller project identity must already be compatible; unsafe project IDs require PROJECT_ID_MAPPING_REQUIRED, with no hidden mapping. Node ID is nonempty, bounded correlation text, never the Generation primary key.

## Source and persistence

The audited R4 client/service required SourceBaseline=418ffbde3dbea33d374356588cb672336ec38353. R6 pending feature extends only ShotID and the current preparation baseline: all new preparations through the updated integration require ff32dc249811130a3db69be456e295be100b6e9f; existing frozen R4/R5 records are not rewritten. See shot_generation_binding_wiring.md. SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_STABLE_BASELINE_CHANGE. Old foundation.SourceBaseline remains unchanged for historical records/tests; the R4 path never defaults to it. SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_BASELINE_ADOPTION: review/update both integration constants and validation/tests on the next baseline adoption. This value describes the reviewed source the integration was based on, not a promise covering later commits.

Reuse foundation.Open/CreateGeneration/FreezeGeneration without semantic/schema changes. Reuse R3 hnReferenceWriter mutex for Open/write/close serialization with snapshots inside one backend process. Independent processes remain controlled single writers; the local-peer guard is not OS-user isolation. Every explicit event generates a fresh ID. Foundation's frozen hash includes identity/CreatedAt facts; repeated equal business inputs need not have equal frozen hashes. Older records remain unchanged and cannot have frozen facts updated. SUBMISSION_UNKNOWN is tested directly through foundation after a synthetic frozen record, never set by ordinary prepare.

Create and Freeze are existing separate durable operations. A process crash/storage failure between them can leave a DRAFT; the endpoint returns no success unless Freeze completed. No automatic retry, submit, historical repair or deletion is introduced. Explicit prepare after a transport error creates another attempt, and callers must not treat it as a generation retry or submission authorization.

## Local HTTP boundary

POST /api/hn/projects/:projectId/generations/prepare; OPTIONS reuses HNReferenceOptions. The unchanged R3 guard requires actual loopback RemoteAddr and Host, accepts only its existing loopback Origin rule, ignores forwarded headers for trust, and mutation requires X-HN-Local-Request: 1. Direct browser transport omits credentials/cookies; existing catch-all Next HN refusal remains unchanged. Read-only discovery is reused. HN_PROJECTS_ROOT is required (503 when absent), configured at runtime only. JSON only, maximum 256 KiB, unknown top-level/binding fields and trailing JSON rejected; errors never echo sensitive input or absolute workspace paths. Response exposes IDs/bindings/hash/status/createdAt only.

credentialRef is unsupported (even an empty caller field is rejected); connectionId is descriptive non-secret channel identity only. No token/key or account config is accepted. Auth/JWT and the upstream DB schema remain unchanged. This is not a generic secret scanner: arbitrary unlabeled secrets must not be supplied as prompt/identity text.

## Normalization and limitations

Nonempty trimmed prompt is mandatory for T2V and image references. Scalar business parameters remain upstream string values: size, videoSeconds, vquality, videoMode, videoNegativePrompt, videoMultiShot, videoShotType, videoGenerateAudio, videoWatermark, videoCharacterOrientation. videoMultiPrompt has only prompt/duration strings, at most 64 items. Client sorts object keys; foundation canonicalizes JSON on persistence; array order is preserved. Numbers/nulls, broad config, unknown fields and media URLs are rejected. Final validation fix 4d0bab236ee4fdc14aec62e0f09ed5f0b92b7e33 closes Go's null-to-zero-string decode case with negative tests. Recognizable credential literals/labels and all HTTP(S)/data URLs, including embedded ones, are conservatively rejected in persisted text. Valid URL-bearing creative prompts are consequently unsupported in R4.

Supported references: zero references (T2V), local image Blob/storageKey image:, roles reference/firstFrame/lastFrame in that order. The adapter preflights every Blob before writes, snapshots input before awaits, and calls unchanged R3 freezeLocalReference for each explicit reference. Only the returned ReferenceVersion ID and exact SHA-256 are bound; filenames/URLs never infer identity. Backend checks project ownership/image kind/role/hash, and foundation verifies file bytes during Create/Freeze. Reference versions successfully created before a later failure are retained; there is no destructive cleanup.

Unsupported: video/audio references, missing/empty/oversized local Blob, server/remote-only images, workflow execution, videoElementList bundles. These are R4 prepare limitations; existing Provider behavior is unchanged. No Provider adapter/protocol/poller import or change, submit/poll/retry, TaskBinding, Result/ArchiveJob, editor handoff or retrospective Generation is wired.

## Verification

Automated tests cover local trust/config/input/secret boundaries, T2V, exact/hash/foreign/missing/unsupported references, new attempt ownership, immutability, reopen/hash validation and SUBMISSION_UNKNOWN. Frontend tests cover canonicalization, broad-config exclusion, role order, R3 ID/hash binding, unsupported inputs before writes, transport and unchanged source, and input snapshots across await.

Smoke uses isolated synthetic Canvas nodes through the existing buildNodeGenerationContext, new production builder/prepare adapter, real production router/R3+R4 handlers and real HN ExtensionStore. Blob lookup and read-only backend discovery are injected by a controlled dev invocation; no new UI or repeated R3 browser-picker validation is claimed. The controlled server starts no Provider/background scheduler/account initializer. Stop, independent process Open/frozen-hash check, restart, second prepare, stop/reopen confirms old raw Generation unchanged, two IDs, matching reference bytes/hash and zero TaskBinding/Result/ArchiveJob. Services stopped. Existing R3 browser ingestion was already audited; actual user browser/submit wiring is deferred.

Full go mod verify/root Go/Bridge, 24 existing + 8 R4 frontend tests, independent TypeScript and production build passed. Lockfiles, dependency versions, foundation/R3 source and Provider/Auth/upstream schema remained unchanged. Initial fixture byte-length and negative-test type assertion were fixed in tests only and verified again. Bun smoke emitted a nonfatal tsconfig directory-handle diagnostic after the successful operation (exit 0); persisted state was independently verified. Completion retains these logs.

R4 GPT Audit PASS approved this scope; closeout preserves all implementation/test bytes and documented caveats. Next workstream remains only a candidate after closeout Completion Review and separate execution approval: prepared Generation + successful local media -> Result/ArchiveJob. No such work starts here.
