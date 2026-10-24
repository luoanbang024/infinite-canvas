# Canvas 本地主序列安全排序

## HN P0-B R31 — Canvas main Sequence safe reorder UI

PROJECT_ID = HN_AI_IC
EXECUTION_ID = HN_AI_IC_P0_B_R31_NO_CREDENTIAL_CANVAS_SEQUENCE_REORDER_UI
TYPE = OUR_EXTENSION / LOCAL_ONLY / CANVAS_UI_WORKFLOW / NO_PROVIDER
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS
BASE_SHA = 5df83b19e661b8677d070b43379f7c4a818b0207
FEATURE_BRANCH = feature/p0-b-r31-no-credential-canvas-sequence-reorder-ui
PUSH = NONE
MERGE = NONE

Only audited R30 revision/CAS + immutable reorderIntentId/receipt. No legacy /reorder, backend/Foundation/schema or drag/drop changes. The existing archive dialog contains a sequence-global 主序列安全排序 subsection; one Canvas-page controller owns each HN project/main state. Explicit load may initialize metadata then read a complete snapshot. Opening/inspection only reads the journal; no HTTP or journal write. Up/down edits an ephemeral draft only; confirmation binds exact loaded revision and full unique item set to one new UUID.

Dedicated localforage store, one closed versioned record per intent. PREPARED write/readback before discovery; immediately before the real typed POST, DISPATCHING write/readback must succeed. Any pre-dispatch storage failure yields zero reorder POST. Opaque dispatched response or final receipt seal failure preserves UNKNOWN recovery barrier; automatic POST retry=0. All unresolved identities are retained and prevent any new R31 reorder. Explicit exact lookup only; NOT_OBSERVED/network/integrity failure cannot clear UNKNOWN. Explicit same-key continuation warns it is a mutation, uses the identical intent/revision/desired IDs, and never generates a new UUID. COMMITTED/CONFLICT need exact validated durable receipts before projection. Historical receipts never prove current order; readonly refresh may show later state and never reapply an old draft.

R20/R22/R24/R26/R28 journals/controllers, R30 typed transport, all Go/backend/Foundation/schema/Generate/Upload/Provider/Auth/dependencies/lockfiles are byte-preserved. Existing placement assertions are preserved; affected UI test receives only additive R31 assertions. Validation includes failure injection, JSON reopen, multiple unresolved identities, SSR/controlled clicks, and ephemeral localhost fake HTTP server observed POST count. Full frontend/independent tsc/root Go/mod verify/isolated production build and source-scope/member-exact secret scan results are in Completion. Corrected initial typecheck test typo is retained as initial failure evidence.

REAL_CREDENTIAL_READ = NONE
REAL_REMOTE_MEDIA_DOWNLOAD = NONE
PROVIDER_CALLS = 0
PAID_CALLS = 0
LIVE_CALLS = 0
API_KEY_SETUP = DEFERRED
API_KEY_INPUT = SKIPPED
API_KEY_DETECTION = SKIPPED
API_KEY_VALIDATION = SKIPPED
ACCOUNT_CREDENTIAL_CHECK = DEFERRED
ACCOUNT_BALANCE_CHECK = DEFERRED
ACCOUNT_ENTITLEMENT_CHECK = DEFERRED
CHANNEL_SECRET_SETUP = DEFERRED
LIVE_PROVIDER_VALIDATION = DEFERRED
LIVE_AUTHORIZATION = NOT_GRANTED

NEXT_ACTION = Independent GPT Review of R31 Audit Closeout Completion. No R32, compound, Export, Jianying or Provider work.

Manual acceptance remains pending, not claimed: real browser IndexedDB/localforage persistence; dialog close/reopen and full reload; two real tabs; theme/layout/accessibility; rapid click/cancel. Browser journal provides identity retention, not browser-wide exactly-once. Server guarantees remain those of audited R30 upgraded writers on the same SQLite file; mixed old binaries/raw DB writers/distributed copies excluded.


R31 verification history: initial test-literal typecheck failure and nested React-array controlled-click harness failure were corrected in new tests only. Initial full Go run hit the default 10m timeout during concurrent disk-intensive build-copy work, with no business assertion failure observed; final full suite reruns all packages with a 30m harness ceiling. Initial failures and final actual gate results are retained separately in Completion. Protected source never changed to address these harness issues.

### R31 GPT Audit closeout binding

GPT_AUDIT = PASS
STATE = AUDITED_INTEGRATED
AUDITED_FEATURE_HEAD = 8a0801b5559a4240ca5ddf58351473f012679a3d
AUDITED_COMPLETION_SHA256 = 9ae3290be25a2878d8d0f00d0dd3d3e52c82f8ef7db768be67169be363bbf090
AUDITED_COMPLETION_COUNTS = 229 ZIP members / 228 payloads / CRC PASS / exact final secret inventory
REVIEWED_COMMITS = 8f4f81a50eecc3e84dcc347c1115a28a1d34c53a, 2023362fd37c5b35878dbb399a115f95e084f9e0, 8a0801b5559a4240ca5ddf58351473f012679a3d
INTEGRATION_METHOD = FAST_FORWARD_ONLY
PUSH_METHOD = NORMAL_NON_FORCE
REVIEWED_COMMIT_REWRITE = NONE
FINAL_STABLE_SHA = recorded in Closeout Completion (no self-reference)
CLOSEOUT_REVIEW_STATUS = READY_FOR_R31_AUDIT_CLOSEOUT_GPT_REVIEW after required checks and final origin identity

R31 independently reviewed implementation is integrated unchanged. This closeout changes only the six authorized governance paths in one commit. All eight R31 production/test files and every non-governance file retain the audited raw bytes; the three reviewed commits remain unchanged. Required fresh full Go/mod verify, R31/R30 focused, prior UI/full frontend, independent TypeScript and actual production frontend + Bridge build evidence are in the Closeout Completion. Real browser acceptance remains NOT_CLAIMED. Closeout GPT Review is still pending independently.

Audited R30 revision/CAS + reorder command receipt only; no legacy /reorder fallback. Per-intent PREPARED -> DISPATCHING durable readback precedes the actual POST; dispatched ambiguity is UNKNOWN, auto POST retry=0. NOT_OBSERVED retains the barrier; exact lookup and explicit identical-key/payload continuation preserve separate unresolved identities. Receipts prove historical commands only; conflict does not reapply an old draft. Controller remains page-owned and sequence-global, with no Video metadata receipt or drag/drop. R20/R22/R24/R26/R28 and R30 transport/backend/Foundation/schema remain byte-identical. Provider/paid/live calls=0; real credential/env content read and remote media download=NONE. API Key input/detection/validation remain SKIPPED; account/balance/entitlement/channel secret/live validation remain DEFERRED, live authorization NOT_GRANTED. No R32/compound/Export/Jianying work.

## R36 Safe Reorder browser-native dispatch correction — audited integration closeout

STATE = AUDITED_INTEGRATED; GPT_AUDIT = PASS_WITH_RETAINED_QUALIFICATIONS; TYPE = OUR_VERIFICATION_FIX / FRONTEND_ONLY. Reviewed feature H is `d99c36c4806e973897a7c111cf1f2061bdb460f1` and is integrated by fast-forward only; reviewed commits are preserved unchanged.

Only controller request construction is corrected so the original browser-native fetch is invoked as a plain function instead of with a foreign controller receiver. R30 revision/CAS, R31 durable journal/recovery semantics and automatic POST retry=0 remain unchanged.

Human-assisted browser revalidation observed reorder COMMITTED revision 3->4 and the linked export/reverify/two-tab checks. Retained evidence limits: no standalone R36_06_REVERIFIED screenshot, second export IndexedDB record not independently expanded, and no full HAR/host packet capture.

Exact-H product verification passed at R36 closeout: the full/focused frontend suites ran on the fixed audit-only Bun 1.4.0 compatibility binary (historical execution provenance — that is the binary the successful H-bound V02/V03 runs actually executed, and the project Bun 1.3.14 pin was not upgraded) and produced 239 PASS / 2 SKIP / 0 FAIL and 221 PASS / 2 SKIP / 0 FAIL.

Subsequent R37 diagnosis (R37-01, recorded by R37-02/R37-03). This is a later finding about capability; it does not describe, and must not be read as describing, the binary used by the historical run above. The same product/test bytes are reproducible on canonical Bun 1.3.14, so the audit-only Bun 1.4.0 binary remains historical execution and diagnostic evidence only and is not a required future verification prerequisite. The verification process must have `git` resolvable and the repository Git object store/history available, because tests assert historical byte preservation through `git show <sha>:<path>`. When `git` is unresolvable those assertions fail first, and Bun 1.3.14 can then cascade into `node:test` nested-test `NotImplementedError` messages while later files register; that cascade alone does not prove Bun 1.3.14 is incompatible.

R22/R26 opt-in cases remain SKIP. Exact-F Gate G must repeat the verification under the current prerequisite (canonical Bun 1.3.14 with `git` resolvable and the object store present) before final main push. Network isolation remains NOT_AVAILABLE. No Provider/credential/live work. This section records the R36 closeout boundary, at which R37 had not started; the current R37 state is recorded in PATCH_LEDGER.md and UPSTREAM_BASELINE.json — the R37-01 diagnosis and the R37-02 local governance commit exist locally, while publication and integration remain pending.
