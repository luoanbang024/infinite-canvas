# HN R5 local Result / ArchiveJob wiring

EXECUTION_ID = HN_AI_IC_P0_B_R5_LOCAL_RESULT_ARCHIVE_WIRING
STATE = AUDITED / INTEGRATED
GPT_AUDIT = PASS
R5_FEATURE_HEAD = 81795aadd070370998472722e35add7d24fbbe4f
R5_COMPLETION_SHA256 = 686bc371b7647c6366071a535ad76f883093b7c5eb3367732e4e664bfec06c47
CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R5_AUDIT_CLOSEOUT_INTEGRATION
RESULT_ARCHIVE_WIRING = EXPLICIT_LOCAL_VIDEO_ONLY
PROVIDER_SUCCESS_ASSERTION = NONE; GENERATION_CREATED_BY_R5 = NO
TASKBINDING_WIRING = NONE; REMOTE_DOWNLOAD = NONE; REAL_PROVIDER_CALLS = NONE; PAID_CALLS = NONE
BASE_OUR_COMMIT = 879d531a03cbdbb815474261cc87afb68b72e610
IMPLEMENTATION_COMMIT = f2d828f296ea2e8600550a89974cf04695092b2a
TEST_COMMIT = b5582e5661ce85e1ccf8d4e79bca29429fd79365
LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS

## Boundary

An explicit existing frozen HN Generation and exact local video Blob create a video Result with empty TaskBindingID, ProviderResultID and SourceURLRef. A Result then owns an ArchiveJob under generated/<generationId>/<resultId>/media.mp4 or media.webm. The existing foundation owns copying, hash/length verification, publication, receipt, state transition and reopen reconciliation. R5 creates no Generation or TaskBinding, does not change the Generation's PREPARED status or SourceBaseline, and does not infer Provider success. No retrospective Generation or live submission is introduced.

R5 reuses hnReferenceWriter across R3/R4/R5 and foundation.Open/CreateResult/CreateArchive/RunArchive. Open validates frozen Generation facts and reconciles files before the service checks target ownership. All calls operate on a single project and reject unknown/foreign/unfrozen Generation or unknown/foreign ArchiveJob. Retry additionally requires video Result ownership and empty Provider/TaskBinding/URL facts. Existing R3/R4/foundation semantics and bytes are unchanged; router only gains R5 POST/OPTIONS registrations.

## Local HTTP

- POST /api/hn/projects/:projectId/generations/:generationId/results/local-archive
- POST /api/hn/projects/:projectId/archive-jobs/:archiveJobId/retry

Use the unchanged R3 local request guard and OPTIONS handler: actual loopback peer, loopback Host, supplied loopback Origin, explicit X-HN-Local-Request: 1, credentials omitted, HN_PROJECTS_ROOT required. Forwarded headers do not establish trust. Existing Next generic HN proxy refusal remains unchanged. Auth/JWT and upstream DB are unchanged.

Multipart has exactly one file and one each of resultKind=video, mimeType, sha256. The additional sha256 is the known local Blob expectation computed by the browser; foundation performs authoritative streamed verification. Unknown fields including providerResultId/taskBindingId/sourceUrl/credential/token are rejected. No absolute file paths are returned. Failures after RunArchive return HTTP 422, code=1 and non-secret archive facts including existing Result/Job IDs; the frontend LocalArchiveFailure exposes those IDs for explicit same-job retry. Other validation/configuration errors use controlled responses without echoing private paths/inputs.

No common existing browser video-upload maximum was found. R5 chooses a conservative 64 MiB video maximum plus 64 KiB multipart envelope. ParseMultipartForm uses a 1 MiB file memory threshold and Go-managed temporary files, removed after use; Go's multipart parser also has its standard bounded form-value allowance. Video is streamed to foundation from the temporary file, never loaded wholesale into Go memory. Client SHA-256 currently reads the bounded Blob in browser memory. Initial container signature must match the declared MP4 or WebM MIME. Original filename is ignored and never controls durable paths. Signature/MIME checks are not codec, playability, full-container or video-track validation.

## Frontend / Canvas

archiveLocalResult and retryLocalArchive use getMediaBlob from the existing media_files store by default, with an injectable exact Blob lookup for controlled tests. Keys must have video: or file: prefix and a nonempty local identifier. server:, HTTP(S), signed URL, blob-only URL and image/audio keys are rejected before lookup or network. Missing/empty/oversized/non-video Blob is rejected before writes. The adapter neither deletes nor replaces the source Blob and does not invoke any remote downloader/upload/Provider function. The direct multipart response is checked for identity, state, path, source SHA-256 and byteLength.

buildHNLocalResultArchiveInput accepts an existing local video node with media content and either success or absent status (imported local media), plus an explicitly supplied HN Generation ID. Loading/error nodes are rejected. The helper returns a new input object and never adds Generation ID to node metadata. Its meaning is “explicitly attach these exact local bytes to this Generation”, not “the Generation or Provider produced this node”. No new button, UI, Canvas data model or submit callback is added.

## Retry and failure facts

Failed or interrupted copies retain Result + ArchiveJob failure history. Retry reopens and uses the same Job and Result; it never calls CreateResult/CreateGeneration/CreateTaskBinding. Completed bytes are verified/reused by foundation and are not copied again. Supplied expected length/hash mismatch is rejected. Before any completed byte evidence exists, foundation checks each retry against that request's supplied expectations; it does not durably pin the original failed request's expected hash. R5 preserves that audited foundation behavior rather than inventing a second manifest/schema. Callers must explicitly choose the same intended local bytes for a retry. A completed Job's recorded byte evidence pins later retry expectations.

CreateResult and CreateArchive remain separate foundation operations. A storage failure between them may retain an unarchived Result; no destructive cleanup or implicit recreation occurs. An uncertain initial HTTP response is not an idempotent create contract: do not automatically repeat initial ingestion; use the returned Job ID for explicit retry when available. Cross-process writers remain controlled single writers. Browser MIME is not codec validation. These limitations do not authorize live submit or archive redesign.

## Verification and next boundary

Synthetic tests cover local guard/config/input/size/MIME/ownership, failure history, exact hashes/lengths/receipt, same-job retry, no duplicate byte writes, immutable Generation and no Provider facts. Full root Go, independent Bridge, 38 frontend tests (32 existing + 6 new), independent TypeScript and production build passed. Local smoke invokes the integrated R4 prepare path only to create prerequisite frozen Generations, then the real R5 frontend adapter and production router/handlers. Isolated success and failure/retry projects each retain one Generation, one Result, one ArchiveJob and zero TaskBindings. Stops/restarts and independent Go-process reopen validate metadata, files and receipts; retry keeps identical IDs/Generation facts. All services stopped.

Smoke uses a 24-byte synthetic MP4 signature fixture, with controlled production adapter Blob lookup/read-only endpoint discovery. It tests exact-byte reliability, not playable media or browser picker UI. The failure fixture injects an incorrect expected hash into a real local multipart request; the backend persists FAILED, then correct-byte retry after restart succeeds. Bun's previously observed nonfatal tsconfig directory-handle diagnostic is retained in logs; operation exits 0 and independent Go verification confirms the durable facts.

GPT Audit PASS; the audited feature was normally pushed and fast-forward integrated without rewriting its three commits. Governance-only closeout preserves R5/R4/R3/foundation implementation and test bytes; focused verification and resulting stable local/remote SHA are recorded in closeout Completion. Await GPT closeout Completion Review and separate execution authorization before archived Result -> Shot/Candidate/explicit selection/SequenceItem -> offline handoff. Provider choice/account/API validation remain deferred. All archive and provenance limitations above remain in effect.
