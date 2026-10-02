# Patch Ledger

PROJECT_ID: HN_AI_IC
BASE_UPSTREAM: tigerowo/infinite-canvas v0.8.0 @ edd4452cb9b0d93dbb9c1ea5acdea1ee14015800

This ledger tracks OUR changes that alter upstream-owned product behavior or source. Governance-only bootstrap files are not product patches.

## Active patches

### HN-AI-IC-P0-R1-RW-BRIDGE-001

- TYPE = `UPSTREAM_BACKPORT`
- EXECUTION_ID = `HN_AI_IC_P0_R1_REWORK_R1_VERIFICATION_FIXES`
- BOUND_BASE = `v0.8.0 / edd4452cb9b0d93dbb9c1ea5acdea1ee14015800`
- SOURCE_COMMIT = `f4e557ebf656cafb7f6a69d1d44959a5845b1b65`
- FILES/HUNKS = `canvas-agent/native/comfy-bridge/workflow_test.go`, one payload line in `TestWorkflowFieldsKeepOriginalBridgeExecution`: replace legacy `params.count` with `workflowOverrides` for node `2`, field `steps`, value `5`.
- REASON = Close the reproduced P0-R1 Bridge verification failure; v0.8.0 production code already consumes `workflowOverrides`.
- OUR_CHANGE = Test fixture only; no Bridge production behavior change.
- INTRODUCED_IN_COMMIT = `5f4fa932f3e7dbe44993c2ae481e1f66d0736113`.
- VERIFICATION = `go test -count=1 ./...` in the Bridge module passed after this backport; full regression results are recorded in the rework completion package.
- DATA_SCHEMA_IMPACT = None.
- REMOVAL_CONDITION = Remove the backport when a reviewed, adopted upstream baseline already contains this test correction.
- ROLLBACK = Revert this fixture line and its governance records; no user-data operation.

### HN-AI-IC-P0-R1-RW-TYPE-001

- TYPE = `OUR_VERIFICATION_FIX`
- EXECUTION_ID = `HN_AI_IC_P0_R1_REWORK_R1_VERIFICATION_FIXES`
- BOUND_BASE = `v0.8.0 / edd4452cb9b0d93dbb9c1ea5acdea1ee14015800`
- SOURCE_COMMIT = None; this one-line annotation is locally authored, not an upstream backport.
- FILES/HUNKS = `web/src/components/model-picker.tsx`, `currentOption` declaration: `useMemo` becomes `useMemo<PickerOption | undefined>`.
- REASON = Preserve the declared picker union through TypeScript inference and close the four reproduced type errors without changing runtime logic.
- UPSTREAM_FIRST_EVIDENCE = Inspected `f4e557ebf656cafb7f6a69d1d44959a5845b1b65` and `6571143e4f51da7494d38572c76202b752cc5e0c`; the isolated upstream change and full virtual upstream frontend both retain all four errors. No upstream-authored correction was found in those candidates.
- AUTHORIZATION = Human explicitly authorized this exact one-line local annotation in the rework chat on 2026-10-01 after reviewing its scope and provenance.
- INTRODUCED_IN_COMMIT = `effb0960bf05abb0e64389e9258fba1f118734c4`.
- VERIFICATION = Virtual compiler reports zero diagnostics; emitted JavaScript is byte-identical. Final independent typecheck and full focused regression are recorded in the completion package.
- DATA_SCHEMA_IMPACT = None.
- REMOVAL_CONDITION = Remove when a reviewed, adopted upstream baseline fixes this inference or makes the annotation unnecessary; confirm with independent typecheck.
- ROLLBACK = Remove the generic annotation and its governance records; no user-data operation.

## Audited baseline closeout

- GPT_REVIEW = `HN_AI_IC_P0_R1_REWORK_R1_GPT_AUDIT_PASS` (2026-10-01), result `PASS`.
- SOURCE_ADOPTION_DECISION = `APPROVED`.
- SOURCE_ADOPTION_STATE = `ADOPTED`: fixed v0.8.0 plus the two audited scoped patches above.
- EXECUTION_ID = `HN_AI_IC_P0_B_R1_BASELINE_ADOPTION_AND_CHOICES`.
- Audited completion SHA-256 = `942cdf50768b568da0dd35896cb83aa305aca0a54a84044fb90f8fdd7ebe9a22`.
- Automatic upstream upgrade remains disabled. Upstream main is not adopted.
- Stable pushed OUR baseline SHA is recorded externally in the Stage A completion evidence to avoid self-reference in this governance commit.

## Rules

For every future upstream patch, record at minimum:

| Field | Required |
|---|---|
| PATCH_ID | Yes |
| Upstream area/file | Yes |
| OUR change | Yes |
| Reason | Yes |
| Introduced in commit | Yes |
| Upstream replacement/removal condition | Yes |
| Data/schema impact | Yes when applicable |
| Rollback note | Yes when risk is non-trivial |

Prefer extension points over direct Canvas/Node/Agent/Provider/Workflow/Asset/Director core modifications.

## HN Extension implementation — P0-B R2

- PATCH_ID = HN-AI-IC-P0-B-R2-FOUNDATION-001
- TYPE = OUR_EXTENSION; SOURCE_COMMIT=None (locally authored, not upstream backport).
- EXECUTION_ID = HN_AI_IC_P0_B_R2_PROVIDER_NEUTRAL_LOCAL_FOUNDATION.
- BASE_OUR_COMMIT = 852fd2128770136d037f92dfef2655dd3d6ac1d5.
- INTRODUCED_IN_COMMIT = b08a968a84a1796cc2ff2dbb4bb60bcac30890a2.
- LOCATION = hn/foundation; isolated Go library, no runtime wiring.
- OUR_CHANGE = project-local workspace/SQLite v1, immutable references, frozen Generation, TaskBinding/Result ownership, file ArchiveJob recovery, explicit Shot/Candidate selection, stable SequenceItem and offline export manifests.
- UPSTREAM_FIRST = fetched tip 6571143e4f51da7494d38572c76202b752cc5e0c; no blocking equivalent local foundation; see docs/hn/upstream-watch.md.
- DATA_SCHEMA_IMPACT = new independent metadata/hn-extension.sqlite only; upstream application schema unchanged. No migration of user production data.
- PROVIDER/AUTH/CANVAS/NODE_CHANGE = NONE; REAL_EXTERNAL_GENERATION = NONE.
- VERIFICATION = 12 local foundation tests plus subprocess reopen, SQLite integrity, root Go, Bridge, independent tsc, existing 18 frontend tests and production build; details in R2 completion.
- REMOVAL_CONDITION = if a formally reviewed/adopted upstream equivalent replaces this HN layer, review identity/data mapping before removing or deprecating it; no automatic data deletion.
- ROLLBACK = revert isolated HN implementation/governance commits before integration; preserve project workspaces. Do not drop user stores.
- ADOPTION_STATE = AUDITED / INTEGRATED_LIBRARY_ONLY; feature integrated by fast-forward without rewriting the two audited commits.
- GPT_AUDIT = PASS; input record HN_AI_IC_P0_B_R2_GPT_AUDIT_PASS_20261001.md.
- R2_FEATURE_HEAD = 35b22aa65fc3b90b3134a7ec20057e6b1cc955ce.
- R2_COMPLETION_SHA256 = f7ead792f34ce9335d2dd93e2b3048cf5a62930a772cb6558a896a17f2d78c3e.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R2_AUDIT_CLOSEOUT_INTEGRATION.
- PROVIDER_WIRING = NONE; UI_WIRING = NONE; REAL_PROVIDER_CALLS = NONE; PAID_CALLS = NONE.
- SOURCE_BASELINE_BINDING = audited code remains bound to 852fd2128770136d037f92dfef2655dd3d6ac1d5; review the binding before future wiring creates new Generations. No implementation change is authorized by this closeout.
- Stable resulting OUR commit is recorded in the external closeout evidence; no self-referential SHA in this commit.


## HN local reference snapshot wiring — P0-B R3

- PATCH_ID = HN-AI-IC-P0-B-R3-REFERENCE-WIRING-001
- TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT=None (locally authored, not upstream backport).
- EXECUTION_ID = HN_AI_IC_P0_B_R3_LOCAL_REFERENCE_SNAPSHOT_WIRING.
- BASE_OUR_COMMIT = b8fe4fb164fa45caf1fffd3141c703988df94c79.
- INTRODUCED_IN_COMMIT = 1fc693be121f1a297a3097ad785dd2a3f26f7f80.
- FILES/HUNKS = handler/hn_reference.go (+tests), service/hn_reference.go; router/router.go HN POST/OPTIONS registration; web/src/services/hn/local-reference.ts (+tests); web/src/app/api/hn/local-endpoint/route.ts read-only discovery; existing catch-all API proxy HN-only refusal; canvas-client-page.tsx imports/in-flight guard/callback/one prop; canvas-node-hover-toolbar.tsx one local-image action and its visibility filtering.
- OUR_CHANGE = reuse existing local image Blob/storageKey; explicit freeze -> audited foundation Open/Snapshot/Close; deterministic logical ID and per-event version ID; no image/node mutation, no new browser store.
- UPSTREAM_FIRST = fetched 6571143e4f51da7494d38572c76202b752cc5e0c; original image-storage/hover toolbar/server storage/router have no new overlapping changes; Canvas candidate hunk is unrelated Agent channel classification. No upstream main adoption.
- LOCAL_API = actual loopback RemoteAddr/Host, explicit custom header, loopback-only Origin/CORS, direct transport and HN-only proxy exclusion. HN_PROJECTS_ROOT runtime parent config; no hardcoded workspace path in source.
- FOUNDATION_SOURCE_CHANGE = NONE, 7 files raw-byte verified against audited base.
- PROVIDER/AUTH/UPSTREAM_DB_SCHEMA/CANVAS_NODE_MODEL_CHANGE = NONE; Generation/TaskBinding/Result/Archive runtime = NONE; KEY_CONFIGURATION/REAL/FREE/PAID_GENERATION = NONE.
- VERIFICATION = go mod verify, root Go, independent Bridge, independent tsc, existing 18 + new 6 frontend tests, production build, synthetic image UI/restart/hash/store smoke, scope/lockfile and secret scans PASS; details in R3 completion.
- ID_MAPPING = no hidden mapping; actual smoke Canvas ID is safe; incompatible existing IDs require PROJECT_ID_MAPPING_REQUIRED review.
- DATA_SCHEMA_IMPACT = no schema changes; writes only existing independent HN ReferenceVersion records/files in explicitly configured project parent.
- ADOPTION_STATE = AUDITED / INTEGRATED; fast-forward preserves both audited R3 commits.
- GPT_AUDIT = PASS; HN_AI_IC_P0_B_R3_GPT_AUDIT_PASS_20261001.md.
- R3_FEATURE_HEAD = 17141d547551874676112314483f2b67d118a851.
- R3_COMPLETION_SHA256 = 6ec629faea0bff479373398bf483f447dbb458efdc09abc31da35fe1d4b31c84.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R3_AUDIT_CLOSEOUT_INTEGRATION.
- REFERENCEVERSION_WIRING = EXPLICIT_LOCAL_IMAGE_ONLY; PROVIDER_WIRING = NONE; GENERATION_WIRING = NONE; RESULT_ARCHIVE_WIRING = NONE; REAL_PROVIDER_CALLS = NONE; PAID_CALLS = NONE.
- EXTERNAL_WRITE_BOUNDARY = human explicitly authorizes only normal non-force pushes of the reviewed R3 feature and our-main to current origin; resulting stable SHA and remote verification are recorded in external closeout evidence.
- REMOVAL_CONDITION = after formal reviewed upstream equivalent adoption, review identity/data compatibility then retire this OUR wiring; never automatically delete HN stores.
- ROLLBACK = revert R3 local commits or omit HN_PROJECTS_ROOT; preserve all workspaces/references, no destructive data operation.


## HN Generation prepare/freeze wiring — P0-B R4

- PATCH_ID = HN-AI-IC-P0-B-R4-GENERATION-PREPARE-001.
- TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT=None (locally authored, not upstream backport).
- EXECUTION_ID = HN_AI_IC_P0_B_R4_GENERATION_PREPARE_FREEZE_WIRING.
- BASE_OUR_COMMIT = 418ffbde3dbea33d374356588cb672336ec38353; INTRODUCED_IN_COMMIT = 92349f02c5a06594a003ba41e49409dc490f4239.
- PURPOSE = provider-neutral Generation prepare/freeze boundary. LOCATION = handler/hn_generation.go, service/hn_generation.go, router HN POST/OPTIONS registration, web/services/hn/local-generation.ts, Canvas internal hn-video-generation-prepare.ts; focused backend/frontend tests.
- UPSTREAM_FIRST = fetched tiger-upstream tip 6571143e4f51da7494d38572c76202b752cc5e0c; no equivalent local Generation identity/frozen-request integration in the checked scope; upstream main not adopted.
- OUR_CHANGE = explicit Canvas intent -> R3 exact local image ReferenceVersion ID/hash/role -> foundation CreateGeneration -> FreezeGeneration -> PREPARED. No live submit path or UI added.
- SOURCE_BASELINE = 418ffbde3dbea33d374356588cb672336ec38353, explicitly required in every R4 input and persisted record; old foundation default never used by this adapter. SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_BASELINE_ADOPTION.
- LOCAL_BOUNDARY = existing R3 actual loopback peer/Host/Origin guard, custom header, direct transport, JSON-only 256 KiB limit, HN_PROJECTS_ROOT runtime config; credentialRef and unknown fields rejected.
- PARAMETERS = closed deterministic video business vocabulary; empty prompt rejected; URL/recognizable credential literals rejected; no broad AiConfig; workflow/element bundles/video/audio/remote-only references unsupported in R4.
- DATA_SCHEMA_IMPACT = existing independent HN Generation/reference tables only; no schema change or historical Generation rewrite. R3/foundation raw bytes unchanged.
- REAL_SUBMIT = NONE; RESULT_ARCHIVE_WIRING = NONE; PROVIDER_SELECTION = DEFERRED; PROVIDER_CALLS/PAID_CALLS = NONE; PROVIDER/PROTOCOL/POLLER/AUTH/UPSTREAM_DB_SCHEMA_CHANGE = NONE.
- VERIFICATION = go mod verify, full root Go, independent Bridge, 32 frontend tests (24 existing + 8 R4), independent tsc, production build, production local router + frontend adapter restart/reopen immutable-attempt smoke, protected-source/lockfile and secret scans PASS; evidence in R4 completion.
- ADOPTION_STATE = AUDITED / INTEGRATED; fast-forward preserves all four audited R4 commits. Test commit 4b99d0fa4e00dd4586282c4de8e5c70b474dcfbe.
- GPT_AUDIT = PASS; HN_AI_IC_P0_B_R4_GPT_AUDIT_PASS_20261001.md.
- R4_FEATURE_HEAD = f03a79522c2bc4e723502fee1297bf5b27c92269; R4_COMPLETION_SHA256 = f37292070ef501689b7a1c702d90fe096d8cd2ae516c480c8e93295a2cb1addd.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R4_AUDIT_CLOSEOUT_INTEGRATION; resulting stable SHA and exact remote refs recorded externally in closeout completion.
- GENERATION_WIRING = PREPARE_FREEZE_ONLY; TASKBINDING_WIRING = NONE; REAL_SUBMIT/RESULT_ARCHIVE_WIRING = NONE; PROVIDER_SELECTION = DEFERRED; REAL_PROVIDER_CALLS/PAID_CALLS = NONE.
- SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_BASELINE_ADOPTION = YES; audited SourceBaseline binding remains 418ffbde3dbea33d374356588cb672336ec38353, no implementation/historical record rewrite.
- EXTERNAL_WRITE_BOUNDARY = user authorized only normal non-force pushes of the audited R4 feature and governance-closed our-main to the checked current origin; no other branch/tag/PR/Release/settings action.
- R4_VALIDATION_FIX = 4d0bab236ee4fdc14aec62e0f09ed5f0b92b7e33; reject JSON null scalar/shot values so the backend preserves the client string-only business vocabulary. Full root Go and final local restart/reopen smoke reverified; no broader implementation change.
- REMOVAL_CONDITION = retire after formal adoption of an equivalent upstream implementation and explicit identity/data mapping review; no automatic user-store deletion.
- ROLLBACK = revert isolated R4 commits or omit HN_PROJECTS_ROOT; preserve project workspaces/references/generations.


## HN local Result / ArchiveJob wiring — P0-B R5

- PATCH_ID = HN-AI-IC-P0-B-R5-LOCAL-ARCHIVE-001.
- TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT=None (locally authored, not upstream backport).
- EXECUTION_ID = HN_AI_IC_P0_B_R5_LOCAL_RESULT_ARCHIVE_WIRING; BASE_OUR_COMMIT = 879d531a03cbdbb815474261cc87afb68b72e610.
- INTRODUCED_IN_COMMIT = f2d828f296ea2e8600550a89974cf04695092b2a; TEST_COMMIT = b5582e5661ce85e1ccf8d4e79bca29429fd79365.
- PURPOSE = local exact-byte Result + ArchiveJob wiring; FILES = handler/service hn_result_archive.go, router R5 registrations, HN frontend local-result-archive.ts, internal Canvas hn-local-result-archive.ts, focused tests.
- FOUNDATION_REUSE = Open/CreateResult/CreateArchive/RunArchive/reconcile plus shared HN writer lock; no duplicate runtime hashing/copy/retry logic or foundation/R3/R4 semantic change.
- PROVENANCE = LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS; TaskBindingID/ProviderResultID/SourceURLRef empty. R5 creates no Generation/TaskBinding and does not modify Generation SourceBaseline.
- LOCAL_INPUT = video:/file: durable Blob only, getMediaBlob; MP4/WebM signature/MIME; 64 MiB max, bounded multipart/disk temporary storage. Filename ignored. SHA-256 expectation supplied by client and verified by foundation. No codec validation claim.
- RETRY = existing ArchiveJob only; no new Result/Generation; completed file verified/reused. Failures preserve IDs/history with controlled 422 response. Existing foundation limitations documented in docs/hn/local_result_archive_wiring.md.
- PROVIDER_SUCCESS_ASSERTION = NONE; TASKBINDING_WIRING = NONE; REMOTE_DOWNLOAD = NONE; REAL_PROVIDER_CALLS/PAID_CALLS = NONE; PROVIDER/AUTH/UPSTREAM_DB_SCHEMA_CHANGE = NONE.
- UPSTREAM_FIRST = fetched tip 6571143e4f51da7494d38572c76202b752cc5e0c; no overlap in checked HN/local file storage/archive paths; no upstream main adoption.
- DATA_SCHEMA_IMPACT = only existing independent HN Result/ArchiveJob records and generated/ files; no schema changes or user media deletion.
- VERIFICATION = go mod verify, full root Go, Bridge, 38 frontend tests, independent tsc, production build, success/failure/same-job-retry/restart/receipt/hash smoke and scope/secret scans PASS; Completion retains evidence.
- ADOPTION_STATE = AUDITED / INTEGRATED; fast-forward preserves all three audited R5 commits.
- GPT_AUDIT = PASS; HN_AI_IC_P0_B_R5_GPT_AUDIT_PASS_20261002.md.
- R5_FEATURE_HEAD = 81795aadd070370998472722e35add7d24fbbe4f; R5_COMPLETION_SHA256 = 686bc371b7647c6366071a535ad76f883093b7c5eb3367732e4e664bfec06c47.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R5_AUDIT_CLOSEOUT_INTEGRATION; stable closeout SHA and verified remote refs recorded externally in Completion, avoiding self-reference.
- RESULT_ARCHIVE_WIRING = EXPLICIT_LOCAL_VIDEO_ONLY; GENERATION_CREATED_BY_R5 = NO; PROVIDER_SUCCESS_ASSERTION = NONE; TASKBINDING_WIRING/REMOTE_DOWNLOAD/REAL_PROVIDER_CALLS/PAID_CALLS = NONE.
- EXTERNAL_WRITE_BOUNDARY = user authorizes only normal non-force pushes of this audited feature and governance-closed our-main to checked current origin; no other branch/tag/PR/Release/settings action.
- REMOVAL_CONDITION = retire after formal adoption of an equivalent upstream implementation and explicit ownership/data compatibility review; never auto-delete stores/media.
- ROLLBACK = omit HN_PROJECTS_ROOT or revert isolated R5 commits before integration; preserve all Result/Archive history and files.


## HN stable Shot / Generation binding — P0-B R6

- PATCH_ID = HN-AI-IC-P0-B-R6-SHOT-BINDING-001; TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT=None (locally authored, not upstream backport).
- EXECUTION_ID = HN_AI_IC_P0_B_R6_SHOT_GENERATION_BINDING_WIRING; BASE_OUR_COMMIT = ff32dc249811130a3db69be456e295be100b6e9f.
- INTRODUCED_IN_COMMIT = 0a06a6451d072b2b2ae76c4e96d3e233268370df; TEST_COMMIT = eefc641e0bd0ff76eb0a02ceef86fd79bf7eb0cb.
- PURPOSE = stable project + exact sourceNodeId Shot identity before Generation freeze; no retroactive frozen ownership mapping.
- LOCATION = new handler/service hn_shot.go, HN local-shot.ts and internal hn-shot-generation-prepare.ts; router Shot ensure POST/OPTIONS; existing HN generation service/client limited to ShotID/current SourceBaseline; focused tests.
- FOUNDATION_REUSE = Open/List/Read/CreateShot/CreateGeneration/FreezeGeneration with unchanged shared HN writer lock; no foundation semantics/schema changes, uniqueness migration or Canvas node field.
- IDENTITY = repeated ensure returns same Shot/initial label/timestamps; title changes do not rename; duplicate pre-existing source identity -> SHOT_IDENTITY_CONFLICT (409), no arbitrary selection. In-process serialization only; controlled single-writer cross-process policy retained.
- GENERATION_WIRING = SHOT_AWARE_PREPARE_FREEZE_ONLY; explicit valid same-project ShotID fixed before freeze; new attempt -> new Generation ID/same Shot. Legacy HN prepare accepts omitted ShotID; new R6 helper requires one.
- SOURCE_BASELINE = ff32dc249811130a3db69be456e295be100b6e9f for all new successful updated preparations; historical R4/R5 records retain their original baseline. SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_STABLE_BASELINE_CHANGE.
- HISTORICAL_GENERATION_REWRITE = NONE; FOUNDATION/R3_REFERENCE/R5_ARCHIVE_SEMANTIC_CHANGE = NONE; DATA_SCHEMA_IMPACT = existing independent Shot/Generation tables only, no migration.
- PROVIDER_SELECTION = DEFERRED; TASKBINDING/RESULT/ARCHIVEJOB/CANDIDATE/SEQUENCEITEM_CREATED_BY_R6 = NONE; PROVIDER_CALLS/PAID_CALLS = NONE; PROVIDER/AUTH/UPSTREAM_DB_SCHEMA_CHANGE = NONE.
- UPSTREAM_FIRST = fetched tip 6571143e4f51da7494d38572c76202b752cc5e0c; no equivalent ShotID/CreateShot/FreezeGeneration in checked service/handler/HN service scope and empty narrow v0.8.0->tip diff; upstream main not adopted.
- VERIFICATION = go mod verify, root Go, Bridge, 45 frontend tests (38 existing + 7 R6), independent tsc, production build, production local router + Canvas helper ensure/freeze/restart/second-attempt/reopen count/hash smoke, protected source/lockfile and secret scans PASS; evidence in R6 Completion.
- ADOPTION_STATE = AUDITED / INTEGRATED; fast-forward preserves all three audited R6 commits.
- GPT_AUDIT = PASS; HN_AI_IC_P0_B_R6_GPT_AUDIT_PASS_20261002.md.
- R6_FEATURE_HEAD = afea14e17e57177b6fc530cc61e6f203eebe9e54; R6_COMPLETION_SHA256 = 352921726ea9cdda91389c56c686b4e9b7be6765b3af5597e7940583cfc6fa55.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R6_AUDIT_CLOSEOUT_INTEGRATION; actual stable closeout SHA and exact remote refs recorded externally in Completion, without self-reference.
- SHOT_IDENTITY = STABLE_PROJECT_AND_EXACT_SOURCE_NODE; GENERATION_WIRING = SHOT_AWARE_PREPARE_FREEZE_ONLY; HISTORICAL_GENERATION_REWRITE = NONE.
- CANDIDATE_WIRING/SEQUENCE_WIRING/REAL_SUBMIT/TASKBINDING_WIRING/REAL_PROVIDER_CALLS/PAID_CALLS = NONE.
- SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_STABLE_BASELINE_CHANGE = YES; reviewed R6 source binding remains ff32dc249811130a3db69be456e295be100b6e9f, with no implementation/historical record rewrite in closeout.
- EXTERNAL_WRITE_BOUNDARY = user authorizes only normal non-force pushes of this audited feature and governance-closed our-main to checked current origin; no other branch/tag/PR/Release/settings action.
- LIMITATIONS = separate Ensure/Reference/Create/Freeze durable steps can retain a Shot/reference/DRAFT on later failure; no automatic Generation retry; no arbitrary unlabeled secret guarantee; no live UI/submit/editor or cross-process uniqueness claim.
- REMOVAL_CONDITION = retire after formal adoption of an equivalent upstream implementation and explicit identity/data compatibility review; no automatic HN data deletion.
- ROLLBACK = omit HN_PROJECTS_ROOT or revert isolated R6 commits before integration; preserve all Shot/Generation/history/reference data.


## HN Candidate / explicit selection / SequenceItem / reorder — P0-B R7

- PATCH_ID = HN-AI-IC-P0-B-R7-EDITORIAL-001; TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT = None (locally authored, not upstream backport).
- EXECUTION_ID = HN_AI_IC_P0_B_R7_CANDIDATE_SEQUENCE_WIRING; BASE_OUR_COMMIT = f3babca6bcdf96f1f692c23bd5c145f06e534973.
- PURPOSE = shot-bound frozen Generation + durable archived Result -> Candidate -> explicit selection -> SequenceItem -> deterministic reorder.
- CANDIDATE_IDENTITY = STABLE_SHOT_AND_RESULT; repeated ensure retains original ID/label/timestamps, duplicate pre-existing match -> CANDIDATE_IDENTITY_CONFLICT. Shared in-process HN writer; no DB uniqueness migration.
- AUTO_SELECTION = NONE; late alternatives preserve current Shot selection and existing placements. Selection is a separate explicit operation, revalidating durable archived facts after Workspace.Open reconciliation.
- SEQUENCE_ADD = EXPLICIT; selected archived Candidate required, every add creates a fresh placement, no hidden dedup. SEQUENCE_REORDER = EXPLICIT_FULL_SET; at most 256 item IDs, no duplicate/missing/foreign items, exact requested response order; only OrderIndex changes.
- FOUNDATION_REUSE = Open/reconcile/Read/List/CreateCandidate/SelectCandidate/AddSequenceItem/Reorder; no duplicated copy/hash/export/generation logic. LOCATION = HN editorial service/handler/router/local-editorial adapter and focused tests.
- ADOPTION_STATE = AUDITED / INTEGRATED; supplied GPT Audit PASS, all seven audited commits preserved by fast-forward.
- GPT_AUDIT = PASS; HN_AI_IC_P0_B_R7_GPT_AUDIT_PASS_20261002.md.
- R7_FEATURE_HEAD = 1741b656e9f43aba688dab6bae73a7e1533eb2d2; R7_COMPLETION_SHA256 = bd3cf1a54fc71cb1cabebb86dff6353e48cc8767a60df8f8a48bd83f4a71e824.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R7_AUDIT_CLOSEOUT_INTEGRATION; actual resulting stable SHA and remote refs recorded externally in closeout Completion, avoiding self-reference.
- GENERATION_SOURCE_BASELINE_CHANGE = NONE; audited preparation baseline remains ff32dc249811130a3db69be456e295be100b6e9f, historical/new frozen Generation records are not rewritten by R7.
- FOUNDATION_SCOPE_EXCEPTION = only explicitly authorized HN-AI-IC-P0-B-R7-REORDER-VERIFY-001 below. Other foundation and all R3/R4/R5/R6 implementation/test bytes remain unchanged; schema/Provider/Auth/dependencies/lockfiles unchanged.
- GENERATION/RESULT/ARCHIVEJOB/TASKBINDING_CREATED_BY_R7 = NO. Smoke prerequisites use existing R6/R5 local production adapters only; independent before/after records prove no downstream prerequisite creation by editorial operations.
- EDITOR_EXPORT/JIANying_MODIFICATION/REAL_PROVIDER_CALLS/PAID_CALLS/REMOTE_DOWNLOAD = NONE; EXTERNAL_WRITE_BOUNDARY = user authorizes normal non-force pushes only of the audited R7 feature and governance-closed our-main to checked current origin; no other branch/tag/PR/Release/settings action. MERGE_TO_OUR_MAIN = FAST_FORWARD_ONLY.
- VERIFICATION = go mod verify, full root Go, Bridge, 53 frontend tests (45 existing + 8 R7), independent tsc, production build, two-alternative production local adapter/router late-arrival/select/repeated-add/reorder/no-op/restart-reopen smoke, protected-source and secret scans PASS; evidence in R7 Completion.
- UPSTREAM_FIRST = fetched tip 6571143e4f51da7494d38572c76202b752cc5e0c; narrow overlap diff empty, no inspected CreateCandidate/SelectCandidate/AddSequenceItem/SequenceItem equivalent. No upstream main adoption.
- REMOVAL_CONDITION = retire wiring after formally adopting equivalent upstream identity/selection/sequence contracts and reviewing data ownership; never auto-delete stores/media. ROLLBACK = omit HN_PROJECTS_ROOT or revert isolated R7 feature commits before integration, preserving local records/media.

## HN strict reorder field invariance — R7 authorized verification fix

- PATCH_ID = HN-AI-IC-P0-B-R7-REORDER-VERIFY-001; TYPE = OUR_VERIFICATION_FIX; SOURCE_COMMIT = None. This is a local verification fix, not an upstream backport.
- PROVEN_GAP = audited Workspace.Reorder changed UpdatedAt even on an unchanged one-item order, violating the R7 only-OrderIndex contract. Reproduction and red regression retained in Completion.
- HUMAN_AUTHORIZATION = user explicitly authorized deleting only `i.UpdatedAt = timestamp()` inside hn/foundation/records.go Reorder and adding full-field invariance regression. No other foundation edits authorized or made.
- EXACT_DELTA = one deleted implementation line; only new foundation test is hn/foundation/reorder_test.go. All other existing foundation raw bytes remain unchanged; no schema/API signature/transaction/selection/archive/generation change.
- REGRESSION = compare all persisted SequenceItem JSON fields, allowing only OrderIndex to differ; UpdatedAt/CreatedAt/identity/schemaVersion/ownership retained; exact reordered indexes, repeated no-op and reopen, all other entity lists unchanged. Red before removal, green after.
- REMOVAL_CONDITION = remove the local patch when a formally adopted upstream baseline already implements equivalent only-OrderIndex behavior; never reintroduce UpdatedAt mutation during removal.
- ADOPTION_STATE = AUDITED / INTEGRATED; GPT Audit PASS. COMMIT = afc8c28867028f95ea33028ad24757566b567711; UPSTREAM_BACKPORT = NO; OTHER_FOUNDATION_CHANGES = NONE.
- CLOSEOUT_PROVENANCE = separate OUR_VERIFICATION_FIX ledger section, standalone auditedVerificationFixes record and immutable original commit retained; no squash/rewrite/restoration/expansion. Invariant regression retained unchanged.


## HN offline sequence export wiring — P0-B R8

- PATCH_ID = HN-AI-IC-P0-B-R8-EXPORT-WIRING-001; TYPE = OUR_INTEGRATION_PATCH; SOURCE_COMMIT = None.
- EXECUTION_ID = HN_AI_IC_P0_B_R8_OFFLINE_EXPORT_WIRING; BASE_OUR_COMMIT = fdb4178d05d2937659416e42e545505fd34278ca; STATE = AUDITED / INTEGRATED.
- GPT_AUDIT = PASS; HN_AI_IC_P0_B_R8_GPT_AUDIT_PASS_20261002.md.
- R8_FEATURE_HEAD = 303904795f0cc3f0875575f3655c27759b099a49; R8_COMPLETION_SHA256 = 8159c886f155d7dcc25704934d3e4fd75103f31e811041981d2e893f5440e3b8.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R8_AUDIT_CLOSEOUT_INTEGRATION; actual stable local/remote SHA recorded in Completion, avoiding self-reference.
- PURPOSE = stable SequenceItem placements -> Workspace.Export -> local HN export API -> frontend exportLocalSequence -> immutable offline bundle.
- FOUNDATION_REUSE = Open reconciliation / Export / Resolve; shared HN writer lock. No duplicate sorting/copy/hash/CSV/JSON/export-ID generation. Service validates returned manifest, completion marker, CSV existence and media lengths; Foundation checks source/copy hash and receipts.
- API = POST /api/hn/projects/:projectId/sequences/:sequenceId/export with exact {}; existing loopback peer/Host/Origin/header/CORS boundary, runtime HN_PROJECTS_ROOT. Return only project-relative facts.
- EXPLICIT_REPEAT = fresh export ID, no automatic retry/delete/overwrite. JSON last completion marker; partial without JSON is not successful. Existing completed bundle immutable after selection/reorder/reopen.
- PROVIDER_CALLS / PAID_CALLS / EDITOR_MUTATION / XML_EDL = NONE. GENERATION / RESULT / ARCHIVEJOB / TASKBINDING_CREATED_BY_R8 = NO. Existing R6/R5 local synthetic prerequisites are attributed separately.
- GENERATION_SOURCE_BASELINE_CHANGE / HISTORICAL_GENERATION_REWRITE = NONE; current constant remains ff32dc249811130a3db69be456e295be100b6e9f. R7 fix afc8c28867028f95ea33028ad24757566b567711 preserved unchanged.
- EXTERNAL_WRITE_BOUNDARY = user-authorized normal non-force pushes only of the reviewed R8 feature and governance-closed our-main to checked origin; MERGE_TO_OUR_MAIN = FAST_FORWARD_ONLY; GPT_AUDIT = PASS. No Provider/Auth/schema/dependency/lockfile changes.
- VERIFICATION = full required Go/Bridge/frontend/typecheck/build, RED BEFORE/GREEN AFTER, exact delta, local production-router/adapter export/restart/hash/bytes/JSON/CSV/immutability evidence in Completion.
- REMOVAL_CONDITION = retire after formal adoption of equivalent upstream export wiring and ownership/data compatibility review. Preserve local bundles/data; omit HN_PROJECTS_ROOT to disable endpoint.

## HN stable placement export — R8 authorized verification fix

- PATCH_ID = HN-AI-IC-P0-B-R8-EXPORT-PLACEMENT-VERIFY-001; TYPE = OUR_VERIFICATION_FIX; COMMIT = 307dba5594e78e095f37e1a6b4bb64900db93bcf; SOURCE_COMMIT = None; UPSTREAM_BACKPORT = NO.
- HUMAN_AUTHORIZATION = current user explicitly authorized deleting only `s.SelectedCandidateID != c.ID ||` from Workspace.Export eligibility and corresponding Foundation regressions.
- EXACT_DELTA = that condition deletion only in hn/foundation/export.go. OTHER_FOUNDATION_PRODUCTION_CHANGE = NONE; R7 Reorder fix remains intact with its original independent provenance.
- RED BEFORE = old Export rejects valid A placements after selecting B and clearing selection; failing log retained. GREEN AFTER = same scenarios pass after exact deletion; old invalidation assertion superseded by persisted-placement contract.
- REGRESSION = Candidate/Shot/Result/Generation binding, ARCHIVED Result, archive receipt/file/hash/bytes still mandatory; no fallback or record mutation; order, repeated placements, new ID, old bundle integrity and reopen verified.
- REMOVAL_CONDITION = retire only after formal adoption of equivalent upstream stable-placement export semantics; do not restore current-selection invalidation.
- ADOPTION_STATE = AUDITED / INTEGRATED; GPT Audit PASS. Separate ledger section and auditedVerificationFixes record retain original commit 307dba5594e78e095f37e1a6b4bb64900db93bcf; no squash/rewrite/restoration/expansion or upstream-backport classification. R7 afc8c28867028f95ea33028ad24757566b567711 remains independently recorded and unchanged.


## R9 Jianying manual handoff — validation/audit record

- EXECUTION_ID = HN_AI_IC_P0_B_R9_JIANYING_MANUAL_HANDOFF_VALIDATION; TYPE = VALIDATION_ONLY / HUMAN_GATED; STATE = AUDITED_VALIDATED.
- BASE_OUR_COMMIT = a1e21382e3037ed10c60fc69f6192bca54b684a1; CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R9_GOVERNANCE_CLOSEOUT; R9_HANDOFF_READY_SHA256 = dec829271e24dd469f922f24b4f58828ceb8b6734653319f4699eea1e11cbe89.
- AUTOMATED_PREPARATION = PASS; three locally encoded H.264/MP4 yuv420p 1280x720/30fps/2sec/no-audio fixtures; source/archive/export hash and bytes, full decode, JSON/CSV B,A,C order and reopen passed.
- HUMAN_GATE = supplied GPT Review PASS and current user confirmation: JIANying_IMPORT = 3/3 PASS; JIANying_TIMELINE_ORDER = B,A,C PASS; JIANying_PLAYBACK = 3/3 PASS; ERROR_DIALOG = NONE.
- PRODUCTION_EDITOR_PROJECT_MODIFIED = NO; MANUAL_TRANSCODING_USED = NO; EXPORT_FILES_RENAMED = NO; disposable manual editor project only.
- CLASSIFICATION = validation evidence with no product implementation delta; separately stored in auditedValidations. SOURCE_CODE_CHANGE = NONE; no new integration patch or verification fix introduced.
- SCOPE = three R9 H.264/MP4 fixtures only; no universal codec/container/audio compatibility, automated editor/draft integration or XML/EDL claim.
- PROVIDER_CALLS / PAID_CALLS = NONE; TASKBINDING_CREATED = 0; GENERATION_SOURCE_BASELINE_CHANGE / HISTORICAL_GENERATION_REWRITE = NONE. Existing R7/R8 verification-fix records unchanged.
- EXTERNAL_WRITE_BOUNDARY = one governance-only commit on our-main and one normal non-force our-main push to checked origin; no other branch/tag/PR/Release/settings changes. Actual stable SHA is recorded in Completion.


## R10 provider-neutral durable submission guard — planned extension

- EXECUTION_ID = HN_AI_IC_P0_B_R10_PROVIDER_NEUTRAL_SUBMISSION_GUARD; TYPE = OUR_EXTENSION / RELIABILITY_WIRING; STATE = AUDITED / INTEGRATED.
- BASE_OUR_COMMIT = f4c0cd1176bf9f48a49f57281065f224e3288276; BRANCH = feature/p0-b-r10-submission-guard.
- PURPOSE = frozen Generation at-most-once transport ownership and TaskBinding acceptance/ambiguity lifecycle.
- PLANNED_FOUNDATION_SCOPE = new submission.go BeginSubmission/RecordSubmissionAccepted/reconciliation plus Open reconciliation hook; existing schema/version and MarkSubmissionUnknown unchanged. This is planned functionality, not OUR_VERIFICATION_FIX or upstream backport.
- DURABLE_GATE = PREPARED -> atomic SUBMITTING + one UNBOUND owner; accepted -> BOUND/SUBMITTED; error/panic/crash ambiguity -> SUBMISSION_UNKNOWN; same-generation resend forbidden.
- TRANSPORT = explicitly injected fake only; no default network transport, production submit route, Provider selection, credentials or retry loop. PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; REAL_PROVIDER_CALLS / PAID_CALLS = NONE; RESULT_WIRING / POLLING / REMOTE_DOWNLOAD = NONE.
- EXISTING_SEMANTICS = R3-R9 preserved, R7 afc8c28867028f95ea33028ad24757566b567711 and R8 307dba5594e78e095f37e1a6b4bb64900db93bcf retain original independent OUR_VERIFICATION_FIX provenance/bytes. R9 auditedValidations record unchanged.
- GENERATION_SOURCE_BASELINE_CHANGE / HISTORICAL_GENERATION_REWRITE = NONE; R6 ff32dc249811130a3db69be456e295be100b6e9f unchanged.
- LIMIT = existing writer held across injected call; no re-entry, lease/availability protocol or real Provider acceptance/status reconciliation. A competing process Open can conservatively sacrifice a live attempt as UNKNOWN; no resend.
- VALIDATION = focused atomic/concurrency/error/panic/subprocess crash/reopen + synthetic local smoke and complete regression evidence in Completion.
- EXTERNAL_PUSH = user-authorized normal non-force pushes only of the audited feature and governance-closed our-main to checked origin; MERGE_TO_OUR_MAIN = FAST_FORWARD_ONLY; GPT_AUDIT = PASS. Future formal upstream equivalent requires ownership/data compatibility review before removal.

- AUDIT_BINDING = FEATURE_HEAD 444a5d7a7c7e8a8fe26953c9f1e9e34a587ebce6; AUDITED_COMPLETION_SHA256 84e571250114418bef21088110c83e4434428c15e6cac3dbf5912ba2736ab242; four original reviewed commits retained, no rebase/squash/amend/rewrite.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R10_AUDIT_CLOSEOUT_INTEGRATION; six governance/progress paths only; audited production/test bytes unchanged. Actual integrated stable SHA recorded in Completion, no self-reference in this commit.
- AT_MOST_ONCE_SUBMISSION_GUARD = AUDITED; BEGIN_SUBMISSION = ATOMIC; TASKBINDING_OWNER = ONE_PER_GENERATION_ATTEMPT; SUBMITTING_REOPEN = SUBMISSION_UNKNOWN; SAME_GENERATION_AUTO_RESEND = NONE; NEW_RETRY_REQUIRES_NEW_GENERATION_ID = YES.
- AUDIT_LIMITS = MULTIPROCESS_AVAILABILITY NOT_CLAIMED; TASK_RECOVERY DEFERRED; REAL_PROVIDER_IDENTIFIER_COMPATIBILITY DEFERRED; REAL_TRANSPORT_RETRY_POLICY_REVIEW REQUIRED_BEFORE_PROVIDER_BINDING. PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE remain DEFERRED.
- NEXT_WORKSTREAM = wait closeout Completion GPT Review before any Provider/account/adapter decision gate; no real Provider, Key, submit, polling or result download authorized by this closeout.

## R12 MiniMax H3 official submit adapter — audited integration

- EXECUTION_ID = HN_AI_IC_P0_B_R12_MINIMAX_H3_OFFICIAL_SUBMIT_ADAPTER; TYPE = OUR_EXTENSION / ADAPTER_WIRING; STATE = AUDITED_INTEGRATED; GPT_AUDIT = PASS; UPSTREAM_BACKPORT = NO. Not OUR_VERIFICATION_FIX, upstream backport or live capability validation.
- BASE_OUR_COMMIT = 16047f46e2186373ea824e12e84ae8dfa2ccde32; BRANCH = feature/p0-b-r12-minimax-h3-official-submit; TARGET_PROVIDER_BINDING = MINIMAX_H3_OFFICIAL_GLOBAL_V2_DIRECT; PROTOCOL = metaso; PROVIDER_IDENTITY = minimax-official-global-v2; MODEL = MiniMax-H3; FIRST_MODE = T2V_ONLY.
- IDENTITY_FREEZE = explicit local channel/protocol/model/exact official global root only; gateway identity unchanged. SOURCE_BASELINE_REVIEW = R12 provider identity freeze review; new constants/tests -> 16047f46e2186373ea824e12e84ae8dfa2ccde32; HISTORICAL_GENERATION_REWRITE / FOUNDATION_DEFAULT_CHANGE = NONE.
- SUBMIT = pre-Begin frozen mapping/channel validation -> unchanged R10 ownership -> owned snapshot/current channel revalidation -> pinned single POST. Exact owner channel resolver reused; ephemeral credential; no second store. Fake loopback TLS tests only, real resolver never invoked.
- HTTP_POLICY = fresh HTTP/1, no proxy/keepalive/HTTP2/redirect/replay/retry/SDK/failover; 30s timeout, 64 KiB response; safe exact task_id bytes -> BOUND/SUBMITTED or UNKNOWN with no same-generation resend. Actual server-observed POST counters are the evidence.
- FOUNDATION_PRODUCTION / R10_IMPLEMENTATION_AND_TEST / UPSTREAM_PROVIDER / AUTH_SCHEMA / DEPENDENCY / LOCKFILE_CHANGE = NONE. R7 afc8c28867028f95ea33028ad24757566b567711 and R8 307dba5594e78e095f37e1a6b4bb64900db93bcf independent OUR_VERIFICATION_FIX provenance/bytes and R9 validation unchanged.
- PRODUCTION_ROUTE / CANVAS_SUBMIT_UI / DEFAULT_CALLER / POLLING / RESULT / ARCHIVE / REMOTE_DOWNLOAD = NONE; PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; LIVE_VALIDATION_AUTHORIZATION = NOT_AUTHORIZED; REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0; REAL_CREDENTIAL_READ = NONE.
- VALIDATION = focused identity/baseline/mapping/resolver/wire/persistence/subprocess/reopen + isolated smoke + full Go/Bridge/frontend/typecheck/build; final evidence in Completion and docs/hn/minimax_official_submit_adapter.md. MERGE_TO_OUR_MAIN = FAST_FORWARD_ONLY; EXTERNAL_PUSH = user-authorized normal non-force R12 feature and our-main only. Closeout focused verification is recorded in the closeout Completion; await its GPT Review.

- AUDIT_BINDING = FEATURE_HEAD c38073515e79c2496cccc28258bc3da1dac4c024; AUDITED_COMPLETION_SHA256 5821b22158345fb551edaf3055d960754a45cca6f491e4dbe7355cdc89f7c331; original commits 8ebce87585559c82aeb3657f7b7220b710ac9aa8 and c38073515e79c2496cccc28258bc3da1dac4c024 preserved without rebase/squash/amend/rewrite.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R12_AUDIT_CLOSEOUT_INTEGRATION; governance/progress only; R12 and all protected implementation/test bytes unchanged. Actual resulting stable SHA is recorded only in Completion. NEW_GENERATION_SOURCE_BASELINE remains 16047f46e2186373ea824e12e84ae8dfa2ccde32; historical Generation unchanged.
- STRICT_SINGLE_WIRE_POST = AUDITED_WITH_LOOPBACK_FAKE; T2V_ONLY = YES; LIVE_PROVIDER_COMPATIBILITY / MINIMAX_ACCOUNT_ENTITLEMENT = NOT_YET_VALIDATED; POLLING_RECOVERY / RESULT_DOWNLOAD_ARCHIVE = DEFERRED; MULTIPROCESS_AVAILABILITY = NOT_CLAIMED; PROXY_REQUIRED_ENVIRONMENT = NOT_VALIDATED.
- REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED / REAL_CREDENTIAL_READ = NONE; PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; LIVE_VALIDATION_AUTHORIZATION = NOT_AUTHORIZED. No R13 work; closeout Completion requires GPT Review.

## R13 MiniMax H3 known-task polling/recovery — audited integration

- EXECUTION_ID = HN_AI_IC_P0_B_R13_MINIMAX_H3_TASK_POLLING_RECOVERY; TYPE = OUR_EXTENSION / ADAPTER_WIRING; STATE = AUDITED_INTEGRATED; GPT_AUDIT = PASS; UPSTREAM_BACKPORT = NO. Not a verification fix or live validation.
- BASE_OUR_COMMIT = 6e8ca04b7bf20241a90a0c1b602b3bea1a23adac; BRANCH = feature/p0-b-r13-minimax-h3-task-polling; TARGET = MINIMAX_H3_OFFICIAL_GLOBAL_V2_QUERY; PROTOCOL = metaso; PROVIDER_IDENTITY = minimax-official-global-v2; MODEL = MiniMax-H3.
- KNOWN_TASK = frozen SUBMITTED/SUBMITTED Generation, one exact BOUND TaskBinding and persisted safe ProviderTaskID; frozen request hash and channel/ownership checked. Read-only pre-Open gate avoids existing Open reconciliation for rejected/other in-flight submissions; no Foundation change.
- QUERY = internally pinned /v2/query/video_generation/{task_id}; fresh HTTP/1 one explicit GET; no proxy, keepalive, HTTP2, SDK/application retry, redirect follow or failover; bounded timeout/headers/body. Synthetic resolver and authenticated loopback TLS tests only.
- OBSERVATION = exact id/model/task_type/status; optional modality/resolution/duration/ratio cross-checked if present against frozen R12 mapping; only queued/running/succeeded/failed/cancelled accepted. Succeeded requires transient valid content.url; public result-location availability only.
- PERSISTENCE = re-read under existing writer; valid observation updates existing LastPolledAt and Foundation-managed UpdatedAt; failed/cancelled only MINIMAX_TASK_FAILED/MINIMAX_TASK_CANCELLED. Conflicting/stale observation cannot replace/clear terminal class. No new status field or TaskBinding/Generation; Generation remains byte-identical SUBMITTED.
- POLL_FAILURE = auth/not-found/transient/protocol/state-conflict separately classified; no raw body/message/key/URL returned or persisted; HTTP/network/protocol errors preserve Generation and TaskBinding polling facts. No MarkSubmissionUnknown or submit call.
- RAW_RESULT_URL_PUBLIC_RETURN / PERSISTENCE / LOGGING / DOWNLOAD = NONE; RESULT / ARCHIVEJOB / CANDIDATE_CREATION = NONE; PRODUCTION_SCHEDULER / ROUTE / UI / STARTUP_WORKER = NONE; POLL_ADAPTER_ENABLED_BY_DEFAULT = NO.
- PRESERVATION = Foundation, R10/R12 implementation/tests, R12 identity/SourceBaseline and historical Generation unchanged; R7 afc8c28867028f95ea33028ad24757566b567711 and R8 307dba5594e78e095f37e1a6b4bb64900db93bcf retain independent OUR_VERIFICATION_FIX; R9 AUDITED_VALIDATED unchanged. GENERATION_SOURCE_BASELINE_CHANGE = NONE; 16047f46e2186373ea824e12e84ae8dfa2ccde32 remains current new-Generation request provenance.
- LIMITS = PROVIDER_QUERY_HISTORY_WINDOW 7_DAYS_DOCUMENTED; HN_LONG_TERM_RECOVERY_AFTER_PROVIDER_HISTORY_EXPIRY NOT_CLAIMED; no media URL TTL inference, UNKNOWN list/search/manual reconciliation, live compatibility/entitlement or cross-process availability guarantee. Unrelated durable SUBMITTING blocks poll preflight conservatively. Proxy-required deployment not validated.
- PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; LIVE_VALIDATION_AUTHORIZATION = NOT_AUTHORIZED; REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0; REAL_CREDENTIAL_READ = NONE; DEPENDENCY / LOCKFILE / AUTH_SCHEMA / UPSTREAM_DB_SCHEMA_CHANGE = NONE.
- VALIDATION = production local boundaries with synthetic accepted tasks; strict fake wire GET counters, failure/nonmutation, stale/concurrent updates, result secrecy, restart/reopen and full regression evidence in Completion. MERGE_TO_OUR_MAIN = FAST_FORWARD_ONLY; EXTERNAL_PUSH = user-authorized normal non-force R13 feature and our-main only. Await GPT Review of audit closeout Completion; no R14 work.

- AUDIT_BINDING = FEATURE_HEAD acf4bd8d5c50d12389de5eaf065cebe8320dcb01; AUDITED_COMPLETION_SHA256 03059bad0da507dcb47a3256f90019f9172ae2a18dc5bd7b55a2bf70359f414a; reviewed commits 301fa8676dd69a7933578f12497673f53a33294f and acf4bd8d5c50d12389de5eaf065cebe8320dcb01 remain exact, unrewritten ancestors.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R13_AUDIT_CLOSEOUT_INTEGRATION; governance/progress only; all protected implementation/test bytes unchanged. Actual resulting stable SHA is recorded only in Completion. NEW_GENERATION_SOURCE_BASELINE remains 16047f46e2186373ea824e12e84ae8dfa2ccde32; historical Generation unchanged.
- KNOWN_TASK_BOUNDARY = SUBMITTED_GENERATION_AND_BOUND_TASKBINDING; SUBMISSION_UNKNOWN_NOT_POLLED = YES; POLL_FAILURE_CHANGES_SUBMISSION_STATE = NO.
- PROVIDER_QUERY_HISTORY_WINDOW = 7_DAYS_DOCUMENTED.
- HN_LONG_TERM_RECOVERY_AFTER_PROVIDER_HISTORY_EXPIRY = NOT_CLAIMED.
- LIVE_PROVIDER_COMPATIBILITY = NOT_YET_VALIDATED.
- MINIMAX_ACCOUNT_ENTITLEMENT = NOT_YET_VALIDATED.
- MULTIPROCESS_AVAILABILITY = NOT_CLAIMED.
- PROXY_REQUIRED_ENVIRONMENT = NOT_VALIDATED.
- MEDIA_URL_TTL = NOT_INFERRED.
- UNKNOWN_SUBMISSION_LIST_SEARCH_RECONCILIATION = DEFERRED.
- R13_POLL_AVAILABILITY_DURING_UNRELATED_SUBMITTING = CONSERVATIVELY_BLOCKED.

## R14 MiniMax H3 successful provider result/archive — audited integration

- EXECUTION_ID = HN_AI_IC_P0_B_R14_MINIMAX_H3_RESULT_ARCHIVE; TYPE = OUR_EXTENSION / ADAPTER_WIRING; STATE = AUDITED_INTEGRATED; GPT_AUDIT = PASS; UPSTREAM_BACKPORT = NO; BASE_OUR_COMMIT = 965afe86d22d83b7cfa04f58e3a7e769d252ca87; BRANCH = feature/p0-b-r14-minimax-h3-result-archive.
- TARGET = MINIMAX_H3_OFFICIAL_SUCCESS_RESULT_TO_HN_ARCHIVE; exact frozen SUBMITTED/SUBMITTED Generation + one BOUND TaskBinding + persisted safe ProviderTaskID; Protocol metaso / ProviderIdentity minimax-official-global-v2 / MiniMax-H3. R13 read gate/snapshot and strict parser, R12 frozen mapper/owner-channel resolver reused without source changes.
- QUERY = one explicit strict succeeded GET before new media attempt/retry; no list/submit/retry/redirect/failover. Private result location discarded; TaskBinding LastPolledAt/ErrorClass and Generation bytes unchanged. Existing terminal ErrorClass rejects as POLL_STATE_CONFLICT.
- RESULT = exact GenerationID / TaskBindingID; ProviderResultID = ProviderTaskID; ResultKind video; SourceURLRef empty; known frozen duration. CreateResult -> CreateArchive -> RunArchive remains existing Foundation production. Target generated/<generationId>/<resultId>/media.mp4; Foundation owns final hash/bytes/receipt.
- MEDIA = HTTPS only, bounded valid hostname, no userinfo/fragment/IP literal/non-443/whitespace/control; DNS all answers must be public, dial one approved literal IP with original TLS ServerName, Proxy=nil, fresh HTTP/1, no redirect/retry, no Authorization/cookie forwarding. Embedded channel credential in transient location rejects. HTTP 200 / exact video/mp4 / known positive Content-Length <=64 MiB / no encoding / MP4 prefix before metadata; bounded reconstructed stream with static reader errors.
- IDEMPOTENCY = writer serialization through bounded I/O and re-read before metadata; exact provider-result/job identity checked. Verified ARCHIVED repeat returns same IDs with zero network. Failed job requires explicit same-job retry; CreateResult/CreateArchive gap repairs same Result with one missing job. Conflicting/duplicate provider-result facts reject.
- VALIDATION = authenticated localhost query/media TLS only; actual request counters, failed/not-ready/malformed query, SSRF mixed-answer rejection, invalid media, stream interruption, same-job retry, concurrency, metadata-gap repair, immutable corruption rejection, restart/reopen, raw-location/credential scan and full regression in Completion.
- PRESERVATION = R5/R10/R12/R13 source/tests and Foundation unchanged; independent R7/R8 OUR_VERIFICATION_FIX and R9 AUDITED_VALIDATED retained. GENERATION_SOURCE_BASELINE_CHANGE / HISTORICAL_GENERATION_REWRITE = NONE; NEW_GENERATION_SOURCE_BASELINE = 16047f46e2186373ea824e12e84ae8dfa2ccde32.
- LIMITS = fake-only; live Provider/CDN/account compatibility NOT_YET_VALIDATED; MP4 prefix signature is not full codec/decode verification. No redirect/chunked/no-length/WebM/>64 MiB/proxy support; MEDIA_URL_TTL NOT_INFERRED; 7-day provider history does not claim long-term recovery. Multiprocess availability NOT_CLAIMED. Unrelated durable SUBMITTING is conservatively rejected; writer held through bounded I/O. Existing corrupt final bytes/immutable receipt are never overwritten; recovery of those artifacts is not claimed.
- REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0; REAL_CREDENTIAL_READ / REAL_REMOTE_MEDIA_DOWNLOAD = NONE; PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; LIVE_VALIDATION_AUTHORIZATION = NOT_AUTHORIZED; RESULT_ARCHIVE_ADAPTER_ENABLED_BY_DEFAULT = NO; PRODUCTION_ROUTE / CANVAS_UI / BACKGROUND_WORKER / CANDIDATE_CREATION = NONE.
- DEPENDENCY / LOCKFILE / AUTH_SCHEMA / UPSTREAM_DB_SCHEMA_CHANGE = NONE; MERGE_TO_OUR_MAIN = FAST_FORWARD_ONLY; EXTERNAL_PUSH = user-authorized normal non-force R14 feature and our-main only. Await GPT Review of audit closeout Completion. No R15 or live-validation work is authorized here.

- AUDIT_BINDING = FEATURE_HEAD 9299b497007c979d35fee436d9deb1ecf280b1ce; AUDITED_COMPLETION_SHA256 bcdb9092219a016b157c8a4a58f8eac4d00f00fc41439218be8e244840cc9f4e; reviewed commits b6750dc45de29ef195ba015e65903953573651e1 and 9299b497007c979d35fee436d9deb1ecf280b1ce remain exact, unrewritten ancestors.
- CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R14_AUDIT_CLOSEOUT_INTEGRATION; governance/progress only; all protected implementation/test bytes unchanged. Actual resulting stable SHA is recorded only in Completion. NEW_GENERATION_SOURCE_BASELINE remains 16047f46e2186373ea824e12e84ae8dfa2ccde32; historical Generation unchanged.
- SSRF_GUARD = AUDITED_WITH_LOOPBACK_FAKE; MEDIA_POLICY = HTTPS / PUBLIC_DNS / PINNED_IP / NO_PROXY / NO_REDIRECT / KNOWN_LENGTH_MP4; ARCHIVE_APIS = CreateResult + CreateArchive + RunArchive; FAILED_JOB_RETRY = EXPLICIT_SAME_JOB; ALREADY_ARCHIVED_REPEAT_NETWORK = NONE.
- RAW_PROVIDER_RESULT_URL_PUBLIC_RETURN / PERSISTENCE / LOGGING = NONE; REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED / REAL_CREDENTIAL_READ / REAL_REMOTE_MEDIA_DOWNLOAD = NONE.
- LIVE_PROVIDER_COMPATIBILITY = NOT_YET_VALIDATED.
- LIVE_CDN_COMPATIBILITY = NOT_YET_VALIDATED.
- MINIMAX_ACCOUNT_ENTITLEMENT = NOT_YET_VALIDATED.
- MEDIA_URL_TTL = NOT_INFERRED.
- PROVIDER_QUERY_HISTORY_WINDOW = 7_DAYS_DOCUMENTED.
- HN_LONG_TERM_RECOVERY_AFTER_PROVIDER_HISTORY_EXPIRY = NOT_CLAIMED.
- MULTIPROCESS_AVAILABILITY = NOT_CLAIMED.
- PROXY_REQUIRED_ENVIRONMENT = NOT_VALIDATED.
- FULL_CODEC_DECODE_VALIDATION = NOT_CLAIMED.
- CORRUPT_IMMUTABLE_FINAL_ARTIFACT_OVERWRITE = REJECTED.
- CORRUPT_IMMUTABLE_FINAL_ARTIFACT_RECOVERY = NOT_CLAIMED.
