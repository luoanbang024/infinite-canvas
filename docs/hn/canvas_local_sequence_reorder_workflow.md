# Canvas 本地主序列安全排序

## HN P0-B R31 — Canvas main Sequence safe reorder UI

PROJECT_ID = HN_AI_IC
EXECUTION_ID = HN_AI_IC_P0_B_R31_NO_CREDENTIAL_CANVAS_SEQUENCE_REORDER_UI
TYPE = OUR_EXTENSION / LOCAL_ONLY / CANVAS_UI_WORKFLOW / NO_PROVIDER
STATE = PENDING_GPT_AUDIT
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

NEXT_ACTION = Independent GPT Review of R31 Completion. No R32, compound, Export, Jianying or Provider work.

Manual acceptance remains pending, not claimed: real browser IndexedDB/localforage persistence; dialog close/reopen and full reload; two real tabs; theme/layout/accessibility; rapid click/cancel. Browser journal provides identity retention, not browser-wide exactly-once. Server guarantees remain those of audited R30 upgraded writers on the same SQLite file; mixed old binaries/raw DB writers/distributed copies excluded.


R31 verification history: initial test-literal typecheck failure and nested React-array controlled-click harness failure were corrected in new tests only. Initial full Go run hit the default 10m timeout during concurrent disk-intensive build-copy work, with no business assertion failure observed; final full suite reruns all packages with a 30m harness ceiling. Initial failures and final actual gate results are retained separately in Completion. Protected source never changed to address these harness issues.
