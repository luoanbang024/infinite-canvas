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
- ADOPTION_STATE = LOCAL_FEATURE_PENDING_GPT_REVIEW; not audited/integrated, not externally pushed. OUR_MAIN remains 879d531a03cbdbb815474261cc87afb68b72e610.
- REMOVAL_CONDITION = retire after formal adoption of an equivalent upstream implementation and explicit ownership/data compatibility review; never auto-delete stores/media.
- ROLLBACK = omit HN_PROJECTS_ROOT or revert isolated R5 commits before integration; preserve all Result/Archive history and files.
