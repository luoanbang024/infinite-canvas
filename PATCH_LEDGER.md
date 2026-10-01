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
