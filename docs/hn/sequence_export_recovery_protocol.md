# R33 local Sequence export recovery protocol

STATE = AUDITED_INTEGRATED
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

API_KEY_SETUP = DEFERRED; API_KEY_INPUT/DETECTION/VALIDATION = SKIPPED; account/balance/entitlement/channel secret/live validation = DEFERRED; live authorization = NOT_GRANTED. Provider/paid/live calls=0; real credential/.env content read and real remote media download=NONE. PUSH=NORMAL_NON_FORCE; INTEGRATION=FAST_FORWARD_ONLY. Next action: independent GPT Review of Audit Closeout Completion; no next-stage work.

### R33 independent GPT Audit closeout binding

EXECUTION_ID = HN_AI_IC_P0_B_R33_NO_CREDENTIAL_SEQUENCE_EXPORT_RECOVERY_PROTOCOL
CLOSEOUT_EXECUTION_ID = HN_AI_IC_P0_B_R33_AUDIT_CLOSEOUT_INTEGRATION
TYPE = OUR_EXTENSION / LOCAL_ONLY / EXPORT_RECOVERY_PROTOCOL / NO_UI
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS
AUDITED_FEATURE_HEAD = 874e8adbea74f8255acd4256af07926fda7c37df
AUDITED_COMPLETION_SHA256 = 4b5324e8710f03a44bf8996e38b5499c274c0f93ec1ad882ab82f062650ca8ac
AUDITED_COMPLETION_COUNTS = 354 ZIP members / 353 payloads / CRC PASS / exact final secret inventory
REVIEWED_COMMITS = e410d1d78108a6906a01b658adecdb6e76ad2a0b, f385c7f707d5aa58f444736107ac3cec43eb1e19, 874e8adbea74f8255acd4256af07926fda7c37df
INTEGRATION_METHOD = FAST_FORWARD_ONLY
PUSH_METHOD = NORMAL_NON_FORCE
REVIEWED_COMMIT_REWRITE = NONE
FINAL_STABLE_SHA = recorded only in Closeout Completion
CLOSEOUT_REVIEW_STATUS = READY_FOR_R33_AUDIT_CLOSEOUT_GPT_REVIEW after all required gates and final remote equality

R33 independently audited production/test bytes are integrated without modification. This commit changes only six governance files. Fresh full Go/mod verify, Foundation/service/handler/router focused tests, actual two-process and 11 process-kill boundaries, bounded attempt/kernel lock/sequence race/current health, prior R7/R8/R28/R30/R31 and local UI regression, full frontend, independent TypeScript and isolated production frontend + Bridge build must PASS before final main push. Evidence and actual final stable SHA belong to Closeout Completion. Closeout GPT Review remains independently pending.

Legacy R8 Export remains unchanged/fresh/non-idempotent. R33 uses expectedRevision + exact ordered vector + exportIntentId; one intent owns one durable ExportID and the accepted snapshot freezes before filesystem effects. Same-key continuation never recaptures current Sequence. Persistent exact-K kernel lock has no TTL stealing/unlink, staging has at most three retained attempts and no destructive partial cleanup. R8 JSON/CSV semantics remain with a separate export-command.json. Final publication and full verification precede immutable COMMITTED receipt; valid final without receipt is adopted by same-key continuation. Receipt is historical; readonly bundle verification reports current sampled health separately. Export does not advance Sequence revision. R30/R31 bytes and semantics remain unchanged. Universal hardware power-loss durability and browser/editor/live acceptance are not claimed.

No Canvas Export UI/browser journal/controller, R34, Jianying, compound or drag/drop. PROVIDER_CALLS = 0; PAID_CALLS = 0; LIVE_CALLS = 0; REAL_CREDENTIAL_READ = NONE; REAL_REMOTE_MEDIA_DOWNLOAD = NONE. API Key input/detection/validation remain SKIPPED; setup/account/balance/entitlement/channel secret/live Provider validation remain DEFERRED; authorization NOT_GRANTED.
