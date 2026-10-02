# HN R6 stable Shot / Generation binding

EXECUTION_ID = HN_AI_IC_P0_B_R6_SHOT_GENERATION_BINDING_WIRING
STATE = AUDITED / INTEGRATED
GPT_AUDIT = PASS
R6_FEATURE_HEAD = afea14e17e57177b6fc530cc61e6f203eebe9e54
R6_COMPLETION_SHA256 = 352921726ea9cdda91389c56c686b4e9b7be6765b3af5597e7940583cfc6fa55
CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R6_AUDIT_CLOSEOUT_INTEGRATION
SHOT_IDENTITY = STABLE_PROJECT_AND_EXACT_SOURCE_NODE
GENERATION_WIRING = SHOT_AWARE_PREPARE_FREEZE_ONLY
NEW_R6_SOURCE_BASELINE = ff32dc249811130a3db69be456e295be100b6e9f
HISTORICAL_GENERATION_REWRITE = NONE; CANDIDATE_WIRING = NONE; SEQUENCE_WIRING = NONE
REAL_SUBMIT = NONE; TASKBINDING_WIRING = NONE; REAL_PROVIDER_CALLS = NONE; PAID_CALLS = NONE
SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_STABLE_BASELINE_CHANGE = YES
BASE_OUR_COMMIT = ff32dc249811130a3db69be456e295be100b6e9f
IMPLEMENTATION_COMMIT = 0a06a6451d072b2b2ae76c4e96d3e233268370df
TEST_COMMIT = eefc641e0bd0ff76eb0a02ceef86fd79bf7eb0cb
TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT = NONE

## Identity and local boundary

EnsureShotForSourceNode(root, projectId, sourceNodeId, label) uses foundation.Open/List(shots)/CreateShot under unchanged hnReferenceWriter. Identity is exact decoded string within a project, not a mutable title or a filesystem path. Source IDs are nonempty, maximum 128 UTF-8 bytes; labels maximum 256 bytes, blank -> Shot. Recognizable credentials, URL-like strings, absolute path forms and control characters are rejected. Relative punctuation/case are preserved, with no trimming, normalization, hash mapping or schema migration. Client rejects malformed Unicode before network. Duplicate nonempty SourceNodeID -> SHOT_IDENTITY_CONFLICT, no arbitrary selection/new Shot/rename. Existing label/timestamps returned unchanged; invalid matching stored metadata is treated as conflict, never echoed. List is a thin linear scan for this local scope; no global unique constraint or pagination change. Same-process concurrent ensure is serialized; independent processes remain controlled single writers.

POST /api/hn/projects/:projectId/shots/ensure accepts only sourceNodeId/label JSON (8 KiB). Unknown fields/trailing JSON/wrong types/unsafe project reject; absent config returns 503, conflict 409. Response includes shotId/projectId/sourceNodeId/label/createdAt/updatedAt only. Reuses actual loopback peer/Host, existing Origin rule, X-HN-Local-Request: 1 and HN_PROJECTS_ROOT; OPTIONS uses existing R3 guard. Direct request credentials omit, read-only discovery unchanged. No Auth/JWT/token, proxy trust or absolute path response.

## Generation and orchestration

Updated PrepareLocalGeneration keeps shotId optional for legacy callers. Nonempty Shot ID must be a safe ID readable in the same project before any Generation creation; foreign/unknown/malformed reject. Foundation receives ShotID before CreateGeneration/FreezeGeneration and response includes the exact frozen ShotID. Backend supports explicit same-project Shot binding; the R6 Canvas helper establishes the source-node association via ensure rather than guessing or inferring a historical mapping. New R6 prepareLocalShotGeneration requires a Shot ID and validates returned binding. All new successful preparations, including omitted-Shot legacy-style calls, require current SourceBaseline ff32dc249811130a3db69be456e295be100b6e9f.

prepareHNShotVideoGeneration captures the existing R4 buildHNVideoGenerationPrepareInput business projection before await, ensures source node id/title, then invokes required-Shot preparation. R3 exact local image roles/reference hash logic and R4 closed business vocabulary remain unchanged. No broad AiConfig/URLs/credentials persist. No node metadata mutation or ShotID Canvas schema field; no UI button/live callback. Title changes do not rename a Shot. Every explicit prepare produces a fresh Generation ID bound to the same Shot.

Historical R4/R5 frozen Generations retain their original SourceBaseline and empty/previous ShotID; no update, remapping or repair is attempted. Foundation hash/invariant validation unchanged. SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_STABLE_BASELINE_CHANGE: review both integration constants/tests on the next stable change; historical foundation default remains unchanged.

## Durable steps and scope

Ensure Shot, ReferenceVersion snapshot, CreateGeneration and FreezeGeneration remain independent durable steps. Later invalid/missing media/storage/transport failure can retain a Shot/reference/DRAFT; no destructive rollback, automatic Generation retry or new idempotency claim. Shot ensure itself reuses exact identity in the supported single-writer scope. Arbitrary unlabeled secrets must never be supplied; validators are not general DLP. R4 video/audio/remote/workflow/element restrictions remain.

R6 creates no TaskBinding/Result/ArchiveJob/Candidate/SequenceItem, submits/polls no Provider, performs no external free/paid generation. R3/R5 source semantics, foundation bytes, Provider/Auth/upstream schema/dependencies/locks unchanged; router only adds Shot POST/OPTIONS. The audited R6 feature was normally pushed and fast-forward integrated without rewriting its three commits; governance-only closeout preserves all reviewed implementation/test bytes.

## Verification

Backend tests exercise input/HTTP guard/config, exact identity/default/label retention/project separation, eight concurrent ensure calls -> one Shot, duplicate pre-existing conflict without count change, foreign/unknown/malformed Shot preparation, current baseline, old historical raw bytes and immutable frozen Shot, second attempt/reopen, forbidden entity counts. Seven new frontend tests plus 38 existing tests cover transport, malformed responses, async snapshots, required Shot/current baseline, unchanged R4 reference/parameter projection, no submit and unchanged node.

Isolated T2V synthetic Canvas source uses production ensure + Canvas helper + Generation adapter and production router. Read-only backend discovery is injected; no browser UI claim or Provider worker started. Ensure twice and title change retain one Shot; first Generation frozen before stop; independent process Open validates ShotID/hash/SQLite integrity. Restart and second prepare create another ID/same Shot; final independent reopen confirms first raw Generation/Shot unchanged and zero TaskBinding/Result/ArchiveJob/Candidate/SequenceItem. Services stopped. Bun emits its pre-existing nonfatal tsconfig directory diagnostic after operation (exit 0); durable independent evidence confirms success.

R6 GPT Audit PASS; full regression/smoke evidence remains in the audited execution Completion. Focused closeout checks, unchanged-source evidence and resulting stable local/remote SHA are recorded in closeout Completion. Await GPT closeout Completion Review and separate execution authorization before Candidate/Sequence work. All audit caveats above remain in effect; the audited SourceBaseline is not rewritten by this governance-only closeout.
