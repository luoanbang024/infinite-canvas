---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## HN 审核后窄范围后续

- R16_GPT_AUDIT = PASS；R16_POLICY_STATE = ACTIVE_GOVERNANCE_POLICY；已 fast-forward 集成原审计 commit，等待 audit-closeout Completion GPT Review。当前有效范围政策：API Key setup/input/detection/validation、账户凭据/余额/权限与 channel secret setup、live Provider validation 统一延期到后续独立凭据阶段；LIVE_VALIDATION_PREAUTH_STATUS = PAUSED_BY_USER_DECISION，LIVE_VALIDATION_AUTHORIZATION = NOT_GRANTED。详见 [凭据延期策略](../hn/api_key_deferral_and_credential_strategy.md)。缺少 Key 不阻塞无关 local/fake 工作；下一实现候选为不依赖真实凭据的本地产品/UI workflow，仍需单独执行文件与评审，本轮不启动。

- R7 stable closeout 为 R8 基线；R8 stable SequenceItem -> offline export API/adapter/bundle 已 GPT Audit PASS 并完成 fast-forward 集成与治理收口，该 R8 closeout 已 GPT Review PASS。仅移除 Export 当前 selection 依赖另记 OUR_VERIFICATION_FIX；R7 Reorder 修正保持。R9 三条 H.264/MP4 fixture 的剪映人工 handoff 已 AUDITED_VALIDATED；真实 Provider/TaskBinding adapter 继续延后独立决定；R10 fake TaskBinding lifecycle 已 GPT Audit PASS / fast-forward 集成，真实 adapter 仍未接线。
- 下次 stable baseline 变化必须复核当前 HN SourceBaseline 常量/adapter/service/tests；R12 provider identity freeze review 将新准备绑定更新为 16047f46e2186373ea824e12e84ae8dfa2ccde32，历史记录不改写，当前绑定不自动覆盖后续升级。真实 Provider submit/idempotency、跨进程 writer、兼容 ID 映射和 unsupported bundles 仍需独立评审。
- R9 已完成安全 H.264/MP4 fixture 的人工导入/顺序/播放验证，R9 governance closeout 已 GPT Review PASS；R10 本地 fake submission guard 已 GPT Audit PASS / 集成，等待 audit closeout Completion Review；其他 codec/音频、自动 editor 集成与正式项目工作流继续独立评审。

- R10 provider-neutral at-most-once guard 已 GPT Audit PASS / fast-forward 集成，保留 pending-test 的真实业务与多进程 availability 限制。等待其 closeout Completion Review。Provider/account/credential/live 工作按当前政策统一延期；现有 fake/local guard 保留，不阻塞单独评审的无凭据本地产品工作。

- R11 discovery 已 GPT Review PASS_WITH_REQUIRED_SCOPE_CORRECTION；R12 official H3 T2V adapter 和 identity freeze 修正已 GPT Audit PASS / fast-forward 集成，实现和测试字节保持，等待 audit closeout Completion GPT Review，详见 pending-test。R12 closeout 已 GPT Review PASS；R13/R14 已 AUDITED_INTEGRATED，R14 closeout 已 GPT Review PASS，实现和测试字节保持；R15 provider Result → 现有 R7/R8 editorial/export 已 GPT Audit PASS / AUDITED_VALIDATED，VALIDATION_ONLY / CROSS_BOUNDARY，无 implementation commit，R15 governance closeout 已 GPT Review PASS；既有 MiniMax readiness/one-task pre-auth 保留历史，现按用户决定 PAUSED_BY_USER_DECISION，不执行、不判失败。credential/channel secret、账户检查与 live Provider validation 统一延期；下一候选为无凭据本地 workflow，生产 route/UI 或其他实现仍需独立范围授权。


## R18 后续审核门

- R17 GPT Audit PASS 的唯一无凭据本地 UI 方案 R18 已 GPT Audit PASS / AUDITED_INTEGRATED，原审计 commits 正常 push / fast-forward 集成；本次仅治理收口，等待 closeout Completion GPT Review。后续 UI 人工验收仍见 pending-test.md；任何新阶段都需单独范围授权，本次不启动。
- 后续用户浏览器 UI 验收和持久化中断限制见 pending-test.md；不扩展为 Provider submit、poll、Result/archive、Candidate/export UI。
- R16 active deferral 继续：API Key setup、credential/account/balance/entitlement/channel secret/live 统一延期；input/detection/validation SKIPPED；pre-auth PAUSED_BY_USER_DECISION，live authorization NOT_GRANTED。


## R20 后续审核门

- R18 closeout GPT Review PASS / R19 GPT Audit PASS 后，本次独立授权的同节点本地 archive-only UI 已 GPT Audit PASS / AUDITED_INTEGRATED，原三 commits 正常推送并 fast-forward 集成；本轮仅治理收口，等待 R20 closeout Completion GPT Review。历史 R18 本轮范围限制保持原记录。
- Candidate/Selection/Sequence/Reorder/Export UI 不在 R20；未知首次 archive 的恢复与真实浏览器 IndexedDB/主题/布局验收见 pending-test.md，未自动开发。
- API Key、account/balance/entitlement/channel secret/live 继续延期，input/detection/validation SKIPPED，pre-auth PAUSED_BY_USER_DECISION，live authorization NOT_GRANTED。

## HN P0-B R22 后续审核门

- R20 Closeout GPT Review PASS / R21 GPT Audit PASS。R22 唯一 Candidate-only UI 已 GPT Audit PASS / AUDITED_INTEGRATED，原三个 commits 正常非 force 推送并 fast-forward 集成；本次只做治理收口，等待 closeout Completion GPT Review。已实现事项与待人工验收见 pending-test.md 和 docs/hn/canvas_local_candidate_workflow.md。
- 仅已归档历史本地版本可显式登记/重新确认候选；Selection、Sequence placement、Reorder、compound helper、Export UI 继续后移，未启动。多进程 writer、真实浏览器持久化中断与主题/布局仍需独立验证。
- API Key/account/channel secret/live 全部继续延期；input/detection/validation SKIPPED；pre-auth PAUSED_BY_USER_DECISION；live authorization NOT_GRANTED；Provider calls=0。

## HN P0-B R24 后续审核门

- R23 GPT Audit PASS；R24 Selection-only UI 已 GPT Audit PASS / AUDITED_INTEGRATED，原三个 commits 正常非 force 推送并 fast-forward 集成；本次仅治理收口，等待 closeout Completion GPT Review。实际可测项见 pending-test.md。
- Sequence placement、Reorder、compound、Export、authoritative read/CAS、跨 tab/process 保护独立后移；不把“重新选择”描述为只读复核。
- API Key/account/channel secret/live 全部延期；input/detection/validation SKIPPED，pre-auth PAUSED_BY_USER_DECISION，live authorization NOT_GRANTED，Provider calls=0。

## HN P0-B R26 后续审核门

- R25 GPT Review PASS；R26 已 GPT Audit PASS / AUDITED_INTEGRATED，三个已审计 commits 原样正常推送并 fast-forward 集成；本次仅一个治理收口 commit，等待 Closeout GPT Review，不启动下一阶段。
- authoritative Sequence read、恢复协议、跨tab/process幂等、Reorder、compound、Export 独立后移；本轮无新增backend/Foundation。
- API Key/account/channel secret/live继续延期，输入/检测/validation跳过；Provider calls=0。

## HN P0-B R28 后续审核门

- R27 Discovery GPT Review PASS 后，仅实现批准的 atomic durable placement command receipt + read-only main snapshot；原实现三 commits + 独立 local-boundary OUR_VERIFICATION_FIX 两 commits 已一起 GPT Audit PASS / AUDITED_INTEGRATED；reviewed head 1b93f9f660412e0460da6479b8bd1e273e4d6056。
- 本次已授权正常非 force feature push、ff-only 集成、一个六文件 governance-only commit；完整 closeout 验证 PASS 后才正常推送 our-main，最终 SHA 和证据写 Completion。等待独立 Closeout GPT Review；不得开始 R29。人工浏览器验收和不保证范围见 pending-test。
- 旧 v1 UNKNOWN 继续 fail closed；完全丢失 intent 不按 Candidate/order/time 推测。Reorder/compound/Export/Jianying、CAS/revision 与 credential/live Provider 不属于本轮。

- legacy Add fresh/non-idempotent；same correlated intent + same payload one-effect；different intent 为新 placement；command receipt/item 同事务。readonly snapshot/exact lookup 不写库，PROTOCOL_NOT_INITIALIZED != NOT_OBSERVED；v1 UNKNOWN 不伪恢复；v2 exact terminal lookup / explicit same-key continuation 保持，boundary 先于 sequence validation。无 Provider/paid、真实 credential/env/media；API Key/account/balance/entitlement/channel/live 继续延期，Reorder/compound/Export/Jianying 继续独立后移。

## HN P0-B R30 后续审核门

- R29 Discovery GPT Review PASS；仅实现批准的 monotonic revision CAS + durable reorder command/receipt server/typed transport 协议。STATE=AUDITED_INTEGRATED，原三个 reviewed commits 不改写，正常非 force 推送并 ff-only 集成；本轮一个治理收口 commit，等待 Closeout Completion 独立 GPT Review。
- PUSH=NORMAL_NON_FORCE，INTEGRATION=FAST_FORWARD_ONLY；不启动 R31、Canvas Reorder/drag-drop/browser journal/controller、compound/Export/Jianying。
- API Key/account/balance/entitlement/channel/live 延期；input/detection/validation skipped；Provider/paid/live calls=0，真实 credential/env/remote media read=NONE。

- Closeout：仅六个治理文件，生产/测试字节不变；完整 Go/mod verify/R30 focused/真实双进程/R7/R28/R20/R22/R24/frontend/独立 TypeScript 与审计 build-source 绑定复验见 Completion。两处已审计 EOF 空行不修。最终 main SHA 仅写 Completion，不自引用；无 R31。

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

## R33 next action

R33 GPT Audit=PASS; STATE=AUDITED_INTEGRATED. Finish required closeout verification and normal final main push, then submit sealed Closeout Completion for independent GPT Review. Final stable SHA is recorded in Completion only. Do not start R34 / Export UI, browser journal/controller, Jianying, compound, drag/drop or credential/live work. All audited R33/R8/R30/R31 semantics and bytes remain preserved.
