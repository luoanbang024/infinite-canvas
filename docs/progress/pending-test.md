---
title: 待测试
description: 当前版本已实现但仍需人工验证的变更项
---

# 待测试

## HN P0-B R16（GPT Audit PASS / ACTIVE_GOVERNANCE_POLICY，等待 audit-closeout Completion GPT Review）

- API_KEY_SETUP = DEFERRED；API_KEY_INPUT / API_KEY_DETECTION / API_KEY_VALIDATION = SKIPPED。ACCOUNT_CREDENTIAL_CHECK / ACCOUNT_BALANCE_CHECK / ACCOUNT_ENTITLEMENT_CHECK / CHANNEL_SECRET_SETUP / LIVE_PROVIDER_VALIDATION = DEFERRED；LIVE_VALIDATION_AUTHORIZATION = NOT_GRANTED。
- 既有 MiniMax readiness/one-task pre-auth = PAUSED_BY_USER_DECISION，不执行、不判失败、不删除历史。详见 [凭据延期与统一阶段策略](../hn/api_key_deferral_and_credential_strategy.md)；R10/R12/R13/R14/R15 fake/local 能力保留，无产品/test/Foundation/dependency/lockfile/Auth/schema/config 改动。
- REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0；REAL_CREDENTIAL_READ / REAL_REMOTE_MEDIA_DOWNLOAD = NONE。不是新的 implementation patch 或 audited validation；原 R16 GPT Audit PASS，已 fast-forward 集成。R16_AUDITED_COMPLETION_SHA256 = 07592d9e4f473812bbab8502921cfe76c0d19aa607f51595b5ea5ef3b856f5c7（40 ZIP members / 39 payloads）；本次仅治理 audit closeout，等待本次 closeout Completion GPT Review。
- 无凭据的 local Canvas、Candidate/Sequence、export/handoff、非 secret UI、离线 error/recovery 工作不以缺失 Key 为阻塞，具体下一实现仍需独立执行与评审；本轮未启动。未来 credential workstream 统一延期，SourceBaseline 与历史 Generation 保持。

## HN P0-B R15（GPT Audit PASS / AUDITED_VALIDATED，保留后续业务限制）

- VALIDATION_ONLY / CROSS_BOUNDARY；R14 provider archived Result 直接复用现有 R7 EnsureCandidate/显式 select/add/reorder 与 R8 export，无 production/test 修改或 implementation commit。两个同 Shot provider Results A/B；Candidate 不自动选择、不自动放置，repeat 不 rename，B 晚到不替换 A。
- 显式最终 sequence/export B,A，reorder 仅 OrderIndex；export hash/bytes 与 source provider Results 一致；close/reopen 和独立进程验证 PASS。全部下游新增 Query/media/submit 为 0，已归档 R14 repeat same Result/job 且零网络；ARCHIVE_FAILED/RECEIVED Result Candidate ensure 拒绝。
- Audited Completion SHA-256 f009105631232c3a38ab7ffd09f864f85f6c254e79c711fbd7be8f6e97ca0b36；临时 R15 test 已删除。只完成治理收口，等待本次 Completion GPT Review。synthetic MP4 boundary fixtures 不宣称完整 codec decode/playback；未重跑剪映，live Provider/CDN/account、TTL、长期恢复、multiprocess availability、proxy 与损坏 immutable artifact recovery 限制保持；无自动 Candidate、production orchestration/Canvas UI 或真实 Key/API/CDN/task，live validation 未授权。

## HN P0-B R14（GPT Audit PASS / 已集成，保留真实业务验证限制）

- MiniMax H3 exact SUBMITTED + BOUND task 的独立 provider-result/archive service boundary；严格 succeeded re-query、SSRF bounded media GET、MP4 header/prefix 校验后复用既有 Foundation 归档。仅 authenticated localhost fake TLS，无生产 route/UI/worker。
- Result 精确绑定 Generation/TaskBinding，ProviderResultID=ProviderTaskID，SourceURLRef 空；已归档重复调用零网络，失败必须显式 same-job retry，metadata gap 同 Result 修复。并发不创建重复 Result/job；流中断/reopen/hash/bytes/receipt 与瞬时 URL 保密均验证。
- R5/R10/R12/R13/Foundation 与 SourceBaseline 保持；真实 Key/API/CDN 调用为 0，不自动创建 Candidate。详细边界见 docs/hn/minimax_official_result_archive.md 与 Completion；live CDN/account、完整 codec decode、多进程 availability 和损坏 immutable final artifact 恢复均不宣称通过。本次 audit closeout 仅治理/进度收口、fast-forward 和正常非 force 推送已审计 R14 feature/our-main，实现和测试字节不变，R14 closeout 已 GPT Review PASS；R15 独立 audited validation 见上，live validation 仍未授权。

## HN P0-B R13（GPT Audit PASS / 已集成，保留真实业务验证限制）

- 已知 SUBMITTED Generation + 唯一 BOUND TaskBinding 增加 explicit MiniMax H3 Query adapter；复用 R12 owner/channel 和冻结参数映射，实际 fake server 每次显式调用仅一次 GET、0 submit POST。
- 仅 queued/running/succeeded/failed/cancelled；严格身份与已报告 request facts 校验，更新既有 LastPolledAt，失败/取消仅静态 ErrorClass；HTTP/协议失败不会进入 SUBMISSION_UNKNOWN，stale terminal conflict 不覆盖持久化事实。仅由已持久化 TaskBinding reopen/query，raw result URL 不公开/持久化/日志/下载。
- 无 Foundation/schema/R10/R12/SourceBaseline 改动，无真实 Key/API/task、Result/ArchiveJob、scheduler/route/UI；完整验证与限制见 docs/hn/minimax_official_task_polling.md 和 Completion。7 天查询窗口、长期恢复、跨进程 availability 和真实兼容性限制保持；本次 audit closeout 仅治理/进度收口、fast-forward 和正常非 force 推送已审计 R13 feature/our-main，实现和测试字节不变，R13 已 AUDITED_INTEGRATED，集成与源码保护记录保持；R14 已独立审计集成。R13_POLL_AVAILABILITY_DURING_UNRELATED_SUBMITTING = CONSERVATIVELY_BLOCKED。

## HN P0-B R12（GPT Audit PASS / 已集成，保留真实业务验证限制）

- HN prepare 仅为显式选中的 exact official global metaso/H3 channel 冻结 minimax-official-global-v2，gateway 不误标；新 Generation 的 backend/frontend SourceBaseline 经 R12 identity review 更新为 16047f46e2186373ea824e12e84ae8dfa2ccde32，历史记录不改写。
- submit-only service adapter 在 Begin 前拒绝无效冻结请求/channel；复用不变 R10，owned snapshot 再校验后仅一次 pinned official POST。仅 localhost TLS fake 验证；完整回归与并发/redirect/429/500/drop/timeout/unsafe ID/persistence/subprocess crash/reopen 的实际 server POST count 见 Completion。
- 无 production submit route、Canvas submit UI、默认启用、真实 Key/Provider/task、polling、Result/download/archive；PRIMARY_VIDEO_PROVIDER/ACCOUNT_MODE 仍 DEFERRED。详见 docs/hn/minimax_official_submit_adapter.md。业务/live 接线需后续独立授权；本次 audit closeout 仅治理/进度收口、fast-forward 和正常非 force 推送已审计 feature/our-main，实现和测试字节不变，等待 closeout Completion GPT Review。
- STRICT_SINGLE_WIRE_POST = AUDITED_WITH_LOOPBACK_FAKE；T2V_ONLY = YES；LIVE_PROVIDER_COMPATIBILITY / MINIMAX_ACCOUNT_ENTITLEMENT = NOT_YET_VALIDATED；POLLING_RECOVERY / RESULT_DOWNLOAD_ARCHIVE = DEFERRED；MULTIPROCESS_AVAILABILITY = NOT_CLAIMED；PROXY_REQUIRED_ENVIRONMENT = NOT_VALIDATED。

## HN P0-B R2 本地基础（GPT Audit PASS / 已集成本地库，待用户接线验证）

- 独立 hn/foundation 已实现 project workspace、SQLite v1、ReferenceVersion、冻结 Generation、TaskBinding/Result、可靠本地 ArchiveJob、Shot/Candidate/Sequence 和离线 JSON/CSV 交接结构；合成 fixtures 的自动验证通过，R2 当时未接线 UI/Provider；R3 已审计集成仅本地图片显式 ReferenceVersion UI，Provider 仍未接线。
- GPT Audit PASS 已确认本轮范围与验证；持续保留单 writer、文件/metadata 对账、SourceBaseline 绑定复核与真实 submit idempotency 的后续接线约束；详见 docs/hn/p0_b_foundation_implementation.md。
- 剪映素材导入/播放/codec 与正式业务工作区手工接线验证延后；本轮不启动编辑器、不调用 Provider、不修改用户数据。


## HN P0-B R3（GPT Audit PASS / 已集成，保留人工业务验证约束）

- 已有本地图片增加“冻结为生成参考”，复用原 Blob/storageKey，runtime HN_PROJECTS_ROOT 控制后端项目父目录；无自动冻结、无原节点替换、无 Provider 调用。
- 自动回归及实际浏览器选图/冻结/前后端重启 smoke 通过；不同快照 ID、相同 byte/hash、原节点身份保持，详见 docs/hn/local_reference_snapshot_wiring.md。
- 尚无该 Canvas UI 的组件测试 harness；真实用户项目与不兼容 Canvas ID 映射、不同独立进程 writer、边缘工具栏溢出、真实编码/剪映 handoff、真实 submit idempotency 仍各需后续单独评审。不得把合成 smoke 当成这些范围的验证。


## HN P0-B R4（GPT Audit PASS / 已集成，保留业务验证约束）

- 内部 Canvas 视频 intent builder 与本机 Generation prepare/freeze 端点已实现；T2V 或 R3 精确本地图片版本绑定 -> PREPARED；当前 reviewed baseline 显式记录，无旧默认值。未接 live submit/UI。
- root Go、Bridge、32 frontend tests、独立 tsc、production build、真实本机 API + production adapter 的合成 restart/reopen/二次 attempt 验证与 scope/secret scans PASS；审计 feature 正常推送并 fast-forward 集成，governance-only closeout 保持实现字节不变；收尾核验和 stable SHA 见 closeout Completion，详见 docs/hn/generation_prepare_freeze_wiring.md 和 Completion。
- workflow/element/video/audio/remote-only 参考、带 URL 文本、任意未标注秘密、跨进程 writer、安全 ID 映射、真实 browser/submit idempotency 均保留后续评审约束。Provider 选择仍 DEFERRED；Result/Archive runtime 无接线。


## HN P0-B R5（GPT Audit PASS，已集成本地 Result / ArchiveJob 接线）

- 已冻结 HN Generation + video:/file: 精确本地 Blob -> Result/ArchiveJob/generated/，显式本地附件不声明 Provider 成功；失败返回 Job ID，重试同 Job 不新建 Generation/Result。
- 完整回归及本机 API restart/reopen/hash/byteLength/receipt/failure-retry smoke PASS；已审计 feature 正常推送并 fast-forward 集成，governance-only closeout 保持 R5/R4/R3/foundation 实现和测试字节不变。focused verification 与 stable local/remote SHA 见 closeout Completion；详见 docs/hn/local_result_archive_wiring.md。
- 64 MiB 与 MP4/WebM 签名边界、非 codec 验证、CreateResult/CreateArchive 非原子步骤、未完成 copy 的期待值不持久固定、初始请求不确定响应、跨进程 writer 等限制保留后续审核。Shot/Candidate/Sequence/剪映及 live Provider 未开始。


## HN P0-B R6（GPT Audit PASS，已集成稳定 Shot / Generation 绑定）

- 同 project + exact sourceNodeId ensure 稳定 Shot；重复返回原 ID/初始 label/timestamps，多个既有同源 Shot 返回 SHOT_IDENTITY_CONFLICT。仅 in-process shared writer 序列化，不宣称跨进程唯一约束。
- Canvas 内部 helper 复用 R4 非敏感参数/本地图片 ReferenceVersion 投影，冻结前绑定 ShotID；当前新 Generation 基线 ff32dc249811130a3db69be456e295be100b6e9f，历史冻结记录保持原样。legacy API 允许省略 ShotID，新 R6 helper 必须提供有效 Shot。
- 完整 root Go、Bridge、45 frontend tests、独立 tsc、production build、生产 adapter/router 的隔离 ensure/restart/reopen/第二次 attempt/hash/零禁用实体 smoke，以及 scope/secret scans PASS；详见 docs/hn/shot_generation_binding_wiring.md 和 Completion。
- 已审计 feature 正常推送并 fast-forward 集成，governance-only closeout 保持 R6/R5/R4/R3/foundation 实现和测试字节不变；focused verification 与 stable local/remote SHA 见 closeout Completion。无 live UI/Provider/TaskBinding/Result/ArchiveJob/Candidate/Sequence 接线。Ensure/Reference/Create/Freeze 仍为独立 durable steps，后续失败保留已创建数据，无自动 retry 或清理。


## HN P0-B R7（GPT Audit PASS，已集成本地 editorial 接线）

- Candidate 稳定 ensure、单独显式选择、显式 SequenceItem placement 与完整集 deterministic reorder 已接线；晚到 B 不替换 A 选择/placement，选择切换不改旧 placement，重复 add 创建新 ID。
- 全量 Go/Bridge/53 frontend tests/独立 tsc/build、生产本地 adapter/router 的 late-arrival/select/repeated-add/reorder/no-op/restart-reopen 和 scope/secret 检查通过；无 Provider 调用、无 export。详见 docs/hn/candidate_sequence_wiring.md 和 Completion。
- 唯一 foundation 改动经用户明确授权：删除 Reorder 更新 UpdatedAt 的一行，单独记录 OUR_VERIFICATION_FIX；新增全字段不变回归，证明 OrderIndex 之外所有字段（含 UpdatedAt）保持。其余 foundation/R3–R6 保持不变。
- 审计 feature 已正常推送并 fast-forward 集成，七个审计提交保持；governance-only closeout 不改任何实现/测试字节。afc8c28867028f95ea33028ad24757566b567711 独立 OUR_VERIFICATION_FIX provenance、单行差异和全字段回归保持；focused verification 与实际 stable local/remote SHA 见 closeout Completion。无 UI/codec/剪映 handoff 验证声明；跨进程 writer、显式 add 不确定响应和既有 R5 归档限制保留。


## HN P0-B R8（GPT Audit PASS，已集成本地 offline export 接线）

- stable SequenceItem -> Workspace.Export -> 本地 API/exportLocalSequence -> 新 UUID 离线 bundle；JSON 最后完成标记，CSV/媒体 hash/byteLength/order 验证；重复 placement 保留，旧 bundle 对 selection/reorder/reopen 不变。
- 明确授权仅移除 Export 当前 selection 依赖，独立 OUR_VERIFICATION_FIX，保留 RED BEFORE/GREEN AFTER 与所有权/归档回归；R7 Reorder 修正不变。
- 无 Provider/真实免费付费调用，无新生成/归档对象，由现有 R6/R5 API 构建合成 smoke 前置数据；无 SourceBaseline 改写，无剪映/草稿/XML/EDL。
- 合成 MP4 signature 不证明 codec 可播放；安全可播放素材的人工导入/顺序/播放已由 R9 三条 H.264/MP4 fixture 的独立人工 Gate 完成（3/3 导入、B→A→C、3/3 播放 PASS）；其他 codec/音频/生产项目接线仍待验证。重复显式 export 可新建第二份 bundle，无自动 retry/删除；跨进程 writer 和用户业务工作区继续保留原限制。审计 feature 已正常推送并 fast-forward 集成，治理收口只改文档；实现/测试字节和 R7/R8 两项独立 OUR_VERIFICATION_FIX 保持。实际 stable local/remote SHA 与 focused checks 见 R8 closeout Completion；该收口已 GPT Review PASS。


## HN P0-B R9（VALIDATION_ONLY / HUMAN_GATED / AUDITED_VALIDATED）

- 已关闭三条安全 H.264/MP4 fixture 的剪映人工导入/顺序/播放 Gate：3/3 导入、B→A→C 顺序、B/A/C 3/3 播放 PASS，无错误弹窗。正式项目未修改，无手动转码或 export 文件重命名。
- R9 Handoff Ready 的 hash/bytes、source/export 全量 decode、JSON/CSV 顺序和 store reopen 已验证；人工声明与随包 GPT Review PASS 作为审核依据，详见 docs/hn/jianying_handoff_validation.md。
- 无产品源码或新功能变更，单独 auditedValidations 记录；Foundation、测试、Provider/Auth/schema/dependency/lockfile 和既有修正保持。R9 只证明该 H.264/MP4/no-audio fixture；其他 codec/音频、自动 editor/draft、正式项目流程、跨进程 writer 与 Provider/TaskBinding 继续保留未验证状态。


## HN P0-B R10（GPT Audit PASS / 已 fast-forward 集成，保持后续业务约束）

- 计划性 provider-neutral Foundation submission APIs + 内部 injected transport service；原子 Begin/accepted、异常 UNKNOWN、并发一次调用、子进程 crash/reopen、相同 ID 禁止重发与新 ID 重试均有合成自动证据。详见 docs/hn/submission_guard.md。
- 完整 Go/Bridge/frontend/typecheck/build 和 scope/secret 检查结果见 Completion；R7/R8 原修正、R9 auditedValidations、历史 Generation 与 SourceBaseline 保持。原审计实现保持；feature 444a5d7a7c7e8a8fe26953c9f1e9e34a587ebce6 及原四 commits 已正常 push / fast-forward 集成，仅本次 governance-only closeout。审计 Completion SHA-256 = 84e571250114418bef21088110c83e4434428c15e6cac3dbf5912ba2736ab242；最终 stable SHA 和 focused checks 见 closeout Completion。
- 无 production submit route/UI、真实 Provider/Key/免费或付费生成、polling、远程 Result/Archive/下载。真实 Provider/account/adapter 绑定与真实业务提交仍需后续决定；跨进程可保守牺牲 live attempt，不承诺 lease/availability。

- R10 TYPE 保持 OUR_EXTENSION / RELIABILITY_WIRING；多进程 availability 不承诺，task recovery / 真实 Provider ID compatibility 延后，真实 transport 必须先评审禁止 hidden retry。等待 closeout Completion Review，不开始真实 Provider 选择/Key/submit/polling/download。


## HN P0-B R18（GPT Audit PASS / AUDITED_INTEGRATED / 无凭据本地准备）

- TYPE = OUR_EXTENSION / PRODUCT_WIRING / LOCAL_ONLY；空 Video intent 的“本地准备”工具栏入口与本地视频准备 Dialog 已实现。明确说明“仅冻结本地业务请求，不调用模型、不生成视频。渠道未绑定。”，打开/重开只有本地检查，HTTP write = 0。
- 精确项目 ID 哈希映射、固定 label 的稳定 Shot、预先缓存本地图片 Blob、ReferenceVersion 绑定、新 frozen Generation PREPARED 和最小回执；Protocol / ProviderIdentity 为空，ConnectionID absent，SourceBaseline 保持 16047f46e2186373ea824e12e84ae8dfa2ccde32，历史不改写。
- page-scoped exact project/node guard，首次 action 一次 POST，第二次显式 action + confirmation；编辑后 STALE，同 fingerprint reload 仅 RELOADED_UNVERIFIED；不确定 POST 为 OUTCOME_UNKNOWN，无自动 resend。当前 Dialog 每两秒仅重查本地 Blob/fingerprint，关闭即停止，不进行 HN query 或 Provider polling。
- root Go、Bridge、79 frontend tests、独立 tsc、隔离 production build、原审计 focused regressions、正常 Canvas store 隔离持久化/重载与 HN 独立进程重开 PASS。现有 ReactDOM SSR + event/controller harness，不声称真实 DOM 点击/用户浏览器验收；不新增测试依赖。
- GPT Audit PASS 后仍保留后续用户 UI 验收：真实浏览器 toolbar/Modal、主题/边缘布局、用户项目/缓存丢失行为；不读取真实凭据。Canvas debounce 400ms 与 HN durable steps 非跨库原子；进程中断可能没有 Canvas receipt；不会后台查询/重试/删除历史。
- Provider-bound / real / paid calls = 0；TaskBinding/Result/ArchiveJob/Candidate/SequenceItem = 0（R18 组合 fixture）；API Key、账户、余额/权限、channel secret/live 工作仍延期。R7/R8 修正与 R9/R15 验证记录、R10–R15 原实现/测试保持。
- 已审计 feature 正常非 force 推送并 fast-forward 集成；本次 governance-only closeout 保持实现/测试字节，focused verification 与最终 our-main push 见收口 Completion，等待其 GPT Review；详见 docs/hn/canvas_local_prepare_workflow.md 与 Completion。


## HN P0-B R20（GPT Audit PASS / AUDITED_INTEGRATED / 本地视频归档 UI）

- 同一个 R18 prepared Video + 本地 video:/file: Blob -> 归档 Dialog -> R5 Result/ArchiveJob -> safe receipt；不声明 AI/Provider 成功，不改正常 status/上传/Generate。详情见 docs/hn/canvas_local_archive_workflow.md。
- root Go、Bridge、101 frontend tests、独立 tsc、隔离 production build、R5/R6/R7/R8/R10/R12/R13/R14/R18/full Foundation preservation PASS；真实 localhost fixture server observed POST=1，同 bytes 重复 fresh=0；production Go fresh A/B 不同 IDs，同 job retry，独立进程 reopen、hash/bytes/sidecar/local provenance PASS。
- journal write/read-back/strict attemptId 是前置 POST 安全屏障；失败=0 POST；首次 response loss UNKNOWN 禁止 fresh 重发；已知 422 只同 job retry；成功后改 bytes 必须再次确认新归档。reload 已知 receipt 仅 RELOADED_UNVERIFIED，pending 转 UNKNOWN；undo 不清 journal。
- GPT Audit PASS；仍待真实用户浏览器 toolbar/Modal/IndexedDB/主题/边缘布局/缓存丢失验收；自动证据为 ReactDOM SSR、injected journal/controller 与 production localhost/handler/process reopen，不声称真实 IndexedDB acceptance。合成 MP4 只证明签名/hash/bytes，不证明 codec playback。
- page lock 无跨 tab/global exactly-once；Canvas/HN/journal 无跨库原子事务；未知首次可能产生孤立 HN facts，保守停止，无 backend query/recovery/自动删除；用户删除全部 journal/receipt、跨进程 writer 和浏览器存储故障 availability 保留限制。
- API Key/account/balance/entitlement/channel secret/live 延期；Provider/paid calls=0；Candidate/Selection/Sequence/Reorder/Export 未实现。R5/Foundation/R18 helper、R7/R8 fix、R9/R15 records 与 R10–R14 原实现保持；原已审计 feature 正常非 force 推送并 fast-forward 集成；本次仅治理收口，focused verification、最终 our-main push 与 stable SHA 见 Completion，等待其 GPT Review。

## HN P0-B R22（GPT Audit PASS / AUDITED_INTEGRATED，等待 closeout Completion GPT Review）

- 复用“归档本地视频”Dialog，仅严格有效的历史 R20 ARCHIVED Result 可点击“将已归档版本加入候选”。Candidate 只是“已归档的本地版本可作为剪辑候选”，不证明 AI/Provider 成功；不自动选中、不加入序列。明确显示完整候选说明和历史版本说明；无新 toolbar action。
- RELOADED_UNVERIFIED 底层 ARCHIVED 和 SOURCE_CHANGED 历史版本允许显式后端完整性复核；FAILED/ARCHIVING/UNKNOWN 禁止 ensure。操作不读取当前 Blob、不 archive、不上传/下载。safe hnLocalCandidate 回执重载为 CANDIDATE_RELOADED_UNVERIFIED；Candidate 成功不提升 R20 reload 状态。
- page-scoped exact project/node 同步锁及 R20/R22 双向互锁；response loss=UNKNOWN，无自动重试，用户明确重新确认同 target 后恢复同 ID。真实 localhost production handler first POST=1、explicit additional POST=1、Candidate count=1、no rename、protected records unchanged；single-process writer 范围，非跨进程保证。
- root Go、Bridge、全部 frontend、独立 tsc、隔离副本 production build、全部要求的 focused preservation 自动验证 PASS；最终 scope/secret/manifest 证据见 Completion。572 个受保护 tracked files raw bytes/hash 未变；R20 controller strip 新 reader 后 raw bytes 不变。
- 待人工：真实浏览器打开/关闭/重开 Dialog、并发点击/确认取消、项目/节点切换、Canvas localforage 持久化/reload、深浅主题布局及可访问性。已完成 SSR/事件 harness、合成 adapter 的真实 Canvas store persist/rehydrate 与 production handler bridge；不宣称真实 DOM/IndexedDB 验收完成。未知首次 R20 archive 仍保持停止策略。
- API Key/account/balance/entitlement/channel secret/live 统一延期；input/detection/validation SKIPPED，pre-auth PAUSED_BY_USER_DECISION，live authorization NOT_GRANTED。Provider/paid calls=0，真实 Key/用户 DB/媒体/账号不读取。没有 Selection/Sequence/Reorder/Export UI、Foundation/R5 backend/R7 production/SourceBaseline 改动，R7/R8 fixes 与 R9/R15 records 保持。

- 本次 closeout 仅六个允许的 governance/progress paths、一个 governance-only commit；原实现/测试 bytes 不变。规定 focused verification 与 normal non-force our-main push、stable SHA 见 Completion；真实浏览器验收仍为 NOT_CLAIMED，不把 Candidate success/reload 解释为 Provider 成功。

## HN P0-B R24（GPT Audit PASS / AUDITED_INTEGRATED，等待 closeout Completion GPT Review）

- 现有 archive Dialog/Candidate 区新增明确 Selection 命令，one confirm→one local Select POST，历史 owner 严格验证，不读当前 Blob，不隐式 ensure/archive/Sequence/Export，不证明 AI/Provider 成功。
- 主锁 hnProject+Shot、辅以 Canvas project+node，六方向互斥；UNKNOWN same-Shot barrier 阻止 archive/ensure/different-target，只允许同 target 用户明确重新选择。repeat 是新 mutation，可能覆盖后来选择；reload receipt=SELECTION_RELOADED_UNVERIFIED，没有 current-selection GET。
- 所有执行要求的 full/focused checks PASS；139 frontend PASS / 1 prerequisite skip（Go handler bridge 已实际验证），独立 tsc/build PASS；578 protected non-env raw files unchanged，核心 Generate/Upload 等 10 个 AST initializer unchanged。Completion 含实际 handler/localhost计数、scope/hash、最终 member-exact 扫描证据。
- 待人工：真实浏览器 Dialog 打开/关闭/重开、六方向快速点击/取消、UNKNOWN 同 target 重新选择、替换视频但仍选择历史版本、项目/节点切换、localforage reload、深浅主题布局/无障碍。已有 SSR/controller/JSON-roundtrip 自动证据，不宣称真实 DOM/IndexedDB 验收。
- R16 deferral、R7/R8 fixes、R9/R15 validations、R20/R22 controller、R5/R7/Foundation/R10/R12/R13/R14 和 SourceBaseline 均保留；不启动 Provider、Key/account、Sequence/Export。

- 本次仅六个治理 allowlist paths、一个 governance-only commit；R24 production/test 和既有 R5–R22 实现字节保留。规定 focused verification 和 normal non-force our-main push、实际 stable SHA 见 Completion；真实 DOM/IndexedDB/主题布局人工验收仍 NOT_CLAIMED。

## HN P0-B R26（GPT Audit PASS / AUDITED_INTEGRATED，人工验收未声明）

- archive Dialog新增“本地序列”区与“加入主序列”，固定main，明确确认；成功同Candidate禁止repeat，UNKNOWN不重发，新Candidate只在已知旧success后明确新Selection才能独立append。
- durable aggregate ledger、prePOST写入/读回、strictack最终封存、page Sequence+Shot/node锁、十二方向互锁、原Shot UNKNOWN barrier、reload UNVERIFIED及零Blob/Provider边界自动验证通过。实际production handler已提交后断连接POST=1/item=1、reload追加POST=0；独立tsc/build/Go/Bridge/full frontend/focused通过，完整证据见Completion。
- 待人工：真实浏览器localforage IndexedDB写入/重载、Dialog深浅主题、关闭重开/快速点击/取消、跨Shot互锁、storage不可用、未确认结果停止、新Candidate操作。SSR/受控事件/controller与JSON roundtrip不等于真实DOM/IndexedDB人工验收。
- SourceBaseline/R7/R8 fixes/R9/R15 validations/R16 policy及冻结source范围保持；无 authoritative GET/CAS/revision、Reorder/compound/Export或Provider/API Key功能。

- R26 Closeout：只改六个治理文件；source/test bytes与已审计Completion保留，focused结果/最终main及origin身份详见Closeout Completion。人工DOM/IndexedDB/theme事项仍保留，Closeout GPT Review尚待独立审核。

## HN P0-B R28（PENDING_GPT_AUDIT）

- 完整 Go/Bridge/mod verify、R28 Foundation/service/handler/router、真实双进程同步屏障、R7/R8/R20/R22/R24/R26 preservation、全部 frontend、独立 tsc、隔离 production build 与 protected-byte checks 通过。首轮失败及修正记录、最终 source-bound logs/counters 在 Completion，不以 focused 代替 full suite。
- 待独立 GPT Audit；真实浏览器 DOM/IndexedDB fsync、深浅主题布局、跨 tab 操作人工验收 NOT_CLAIMED。readonly metadata 不证明媒体文件现存/codec 可播放；lost K、损坏 schema/records、存储故障继续 fail closed。
- 支持升级 legacy/new writer 同 SQLite（不含 concurrent legacy reorder）；mixed old binaries/直接 DB 写/跨主机 FS 不保证。v1 UNKNOWN 无自动迁移，v2 exact GET + 同 K/P 显式 continuation；无自动 POST retry，无当前 Selection GET/CAS/revision、Reorder/compound/Export/Jianying。
- API Key/account/channel/live 延期；Provider/paid=0、真实 credential/env/media NONE；R7/R8 fixes、SourceBaseline 及既有 R20/R22/R24 语义保留；无 push/merge。
