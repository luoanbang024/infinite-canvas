# 无限画布文档索引

## 项目介绍

- [快速开始](overview/quick-start.md)
- [功能介绍](overview/features.md)
- [Docker 部署](overview/docker.md)
- [第三方 GitHub 提示词仓库](overview/third-party-prompt-repositories.md)

## 操作手册

- [画布节点操作手册](canvas/canvas-node-manual.md)
- [画布快捷键](canvas/canvas-shortcuts.md)

## 开发文档

- [本地开发](backend/local-development.md)
- [接口响应约定](backend/api-response.md)
- [系统配置数据结构](backend/system-settings.md)
- [后端数据库说明](backend/backend-database.md)
- [画布数据结构](backend/canvas-data-structure.md)

## 商务合作

- [开源协议](business/license.md)
- [商务合作](business/business.md)

## 赞助支持

- [打赏支持](support/donate.md)

## 项目进度

- [待测试](progress/pending-test.md)
- [TODO](progress/todo.md)

## 说明

- 未登录时画布项目和“我的素材”保存在浏览器本地；登录且账号同步可用时，会同步保存到账号/云端。
- 本地直连模式下，AI API Key 保存在浏览器本地，并由前端直接请求 OpenAI 兼容接口。

## HN P0-B R24

- [Canvas 本地候选显式选择（GPT Audit PASS / AUDITED_INTEGRATED）](hn/canvas_local_selection_workflow.md)：strict historical Candidate → mutable Selection → advisory receipt；同 Shot 六方向互锁及 UNKNOWN 停止边界。

## HN P0-B R26

- [Canvas 本地主序列显式加入（GPT Audit PASS / 已集成）](hn/canvas_local_sequence_placement_workflow.md)：strict historical Candidate/Selection → non-idempotent append → durable browser ledger / advisory receipt；main固定、UNKNOWN停止。

## HN P0-B R28

- [Canvas 本地主序列命令与 exact-key 恢复（GPT Audit PASS / AUDITED_INTEGRATED）](hn/canvas_local_sequence_recovery_workflow.md)：same-intent immutable receipt、SQLite cross-process append、readonly main snapshot；v1 UNKNOWN 不迁移。

- R28 原实现与 local-boundary OUR_VERIFICATION_FIX 已一起通过独立 GPT Audit；五个 reviewed commits 原样正常推送、ff-only 集成，六文件治理收口。boundary 先于 sequence validation；等待 Closeout Completion GPT Review，真实 browser 验收 NOT_CLAIMED，未启动 R29。

## HN P0-B R30

- [本地主序列 revision/CAS/command 协议（GPT Audit PASS / AUDITED_INTEGRATED / NO_UI）](hn/sequence_reorder_protocol.md)：upgraded main writers 同事务 revision、immutable historical receipt、readonly full snapshot；无 Canvas Reorder 接线。

- R30 三个 reviewed commits 原样正常推送、ff-only 集成；六文件一个治理收口 commit，production/test 和 EOF 字节保留；等待 Closeout Completion GPT Review。无 R31/Reorder UI/Provider。

## HN P0-B R31 — Canvas main Sequence safe reorder UI

PROJECT_ID = HN_AI_IC
EXECUTION_ID = HN_AI_IC_P0_B_R31_NO_CREDENTIAL_CANVAS_SEQUENCE_REORDER_UI
TYPE = OUR_EXTENSION / LOCAL_ONLY / CANVAS_UI_WORKFLOW / NO_PROVIDER
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS
BASE_SHA = 5df83b19e661b8677d070b43379f7c4a818b0207
FEATURE_BRANCH = feature/p0-b-r31-no-credential-canvas-sequence-reorder-ui
PUSH = NORMAL_NON_FORCE
INTEGRATION = FAST_FORWARD_ONLY

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
