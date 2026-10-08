# R33 local Sequence export recovery protocol

STATE = PENDING_GPT_AUDIT
TYPE = OUR_EXTENSION / LOCAL_ONLY / EXPORT_RECOVERY_PROTOCOL / NO_UI
BASE_SHA = 790439097de0f3f03995c9ea9c0ef010091d7b56
EXECUTION_ID = HN_AI_IC_P0_B_R33_NO_CREDENTIAL_SEQUENCE_EXPORT_RECOVERY_PROTOCOL

## Authority and identities

K = projectId + main + protocolVersion 1 + lowercase UUIDv4 exportIntentId. Canonical P has exactly expectedRevision (canonical nonnegative signed-int64 decimal string), orderedSequenceItemIds (complete unique safe vector, at most 256), format hn-offline-bundle-v1, in that JSON key order. SHA-256 binds compact UTF-8 P. Wire command adds protocolVersion/exportIntentId. First accepted command checks revision and full vector and validates indexed plus JSON Shot/Candidate/frozen Generation/ARCHIVED Result/ArchiveJob ownership under a real short SQLite write transaction. Current Selection is not an export prerequisite. It reserves one ExportID and immutable minimal snapshot (archive path/hash/bytes/duration/frozen facts), never prompts or credentials.

Existing exact K is examined before current CAS. Same K/P terminal receipts replay without files or current mutable rows; changed P conflicts. Same-key nonterminal continuation uses the original snapshot and ExportID, even after later append/reorder/delete. Different intent is a fresh explicit export. Export does not advance Sequence revision or change SequenceItem fields/Generation SourceBaseline.

## Local routes

Under /api/hn/projects/:projectId/sequences/:sequenceId:

| Method | suffix | behavior |
|---|---|---|
| POST | export-protocol/initialize | explicit closed protocolVersion=1; existing store; requires already initialized R30; exact additive export schema |
| POST | export-commands | one explicit command/continuation; no automatic retry |
| GET | export-commands/:intent | exact readonly command/receipt, or snapshot-only NOT_OBSERVED |
| GET | export-commands/:intent/bundle-verification | COMMITTED historical identity plus current sampled VERIFIED/MISSING/CORRUPT health |

Local boundary precedes main validation on every route: loopback peer/Host/Origin, X-HN-Local-Request: 1, no Cookie/Authorization/query, GET empty body, closed duplicate/unknown-field-free JSON POST <=64KiB, no-store. Dedicated restrictive OPTIONS and static sanitized logs do not weaken old POST-only OPTIONS. Lookup uses strict readonly OpenExisting, not normal Open; it cannot initialize/migrate/reconcile/mkdir/lock/access media. Verification separately reads exact bounded bundle facts without writes. Protocol absent differs from NOT_OBSERVED; the latter never proves an in-flight command cannot later complete.

## Durable state and file ownership

Additive exact export_protocol/export_jobs/export_receipts tables and retention/immutability/transition triggers are explicitly initialized. Job authority K/P/hash/revisions/ExportID/snapshot/createdAt is immutable. Job phases: RESERVED, COPYING, READY_TO_FINALIZE, RETRYABLE, RECOVERY_BLOCKED; terminal COMMITTED/REJECTED/FAILED. Attempt count is persisted before stage creation, maximum three. Terminal receipt and job transition commit in the same short SQLite transaction; terminal jobs/receipts cannot be updated/deleted/replaced.

Persistent metadata/export-locks/<canonical-K-hash>.lock has a real exclusive kernel lock, held throughout same-key continuation. It is never unlinked or TTL-expired. A competing exact-key process gets bounded IN_PROGRESS; process death releases OS ownership. Windows LockFileEx and Unix flock are platform-specific; unsupported targets fail closed. Guarantee covers upgraded same-local-workspace writers, not arbitrary DB/filesystem writers or universal network filesystems.

Stage = metadata/export-staging/<exportId>/<attemptId>; final = exports/commands-v1/<exportId>. Expected media-byte sum bounds streaming copies; JSON/CSV/marker overhead is bounded to 1MiB. Source sidecar and archive bytes must agree with frozen facts. Partials remain owned and retained; no fourth attempt or automatic cleanup. Full stage verification checks exact files, hashes, byte lengths, manifests and command marker, then READY_TO_FINALIZE is committed. Same-volume whole-directory publication never replaces a foreign final. File Sync plus platform-appropriate publication/directory barriers precede final re-verification and terminal SQLite commit. Windows directory fsync has no portable equivalent; write-through publication is used. Universal filesystem+SQLite/power-loss atomicity is not asserted.

## Recovery and receipts

Valid current stage -> finalize same attempt. Valid final with no terminal receipt -> full verification then receipt, no media recopy or new ExportID. Partial stage -> explicit next bounded attempt. Three exhausted attempts -> RECOVERY_BLOCKED. Foreign/malformed/unreadable final -> RECOVERY_BLOCKED, never overwrite or fake FAILED. Source integrity can become terminal FAILED only while owning the lock with exact final proven absent. Safe absent-final I/O may remain RETRYABLE. Historical COMMITTED is never rebuilt when files later disappear/corrupt; explicit readonly health is separate.

R8 ordered-manifest.json keeps the existing ExportManifest/ManifestItem semantics, and CSV keeps exactly its ten columns. Separate export-command.json binds K/P/hash/revisions/format/exportId/snapshot/items/manifests/currentAttempt; no raw URL, credentials or baseline changes. Receipt is immutable historical completion, not current Sequence/Selection, Provider success or perpetual disk health. Media fixtures are synthetic bytes; codec/GUI/editor/live-provider acceptance is not claimed.

## Transport and validation

New local-export-commands.ts exports initializeExportProtocol, executeExportCommand, lookupExportCommand, verifyExportBundle. Inputs are copied before await; responses have closed shapes, exact identities/canonical hash, bounded UTF-8 JSON (1MiB), canonical wire serialization, finite timeouts, credentials omit/redirect error/cache no-store. No fallback to legacy Export and automatic POST retry=0. Caller must retain original command for explicit exact lookup/same-key continuation; no Canvas action or browser journal is added in R33.

Completion retains full Go/frontend/typecheck/mod/build logs, 11 actual killed child-process boundaries, two-process same/different K and initialization, actual kernel-suspended owner/kill release, concurrent supported sequence writers, bounded attempt/unsafe-path/health facts, raw/Git source bindings and exact-member recursive disclosure scan. R7 only-OrderIndex, R8 selection independence, R28 placement, R30 revision/CAS and R31 UI are preserved. Existing optional R22/R26 browser bridge cases are also exercised by the root Go handler integration fixtures. No browser DOM/IndexedDB/manual Jianying/hardware-power-loss acceptance is implied.

API_KEY_SETUP = DEFERRED; API_KEY_INPUT/DETECTION/VALIDATION = SKIPPED; account/balance/entitlement/channel secret/live validation = DEFERRED; live authorization = NOT_GRANTED. Provider/paid/live calls=0; real credential/.env content read and real remote media download=NONE. PUSH=NONE; MERGE=NONE. Next action: independent GPT review, no integration or next-stage work.
