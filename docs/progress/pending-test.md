---
title: 待测试
description: 当前版本已实现但仍需人工验证的变更项
---

# 待测试

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
