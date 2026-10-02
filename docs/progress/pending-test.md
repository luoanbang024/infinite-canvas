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


## HN P0-B R7（本地 feature 部分完成，foundation API gap 待审查）

- Candidate ensure、单独显式选择、显式 SequenceItem placement 与内部 frontend helper 已实现。晚到 Candidate 不替换选择，旧 placement 不随选择改写，重复 add 创建新 ID。
- 当前完整 Go/Bridge/50 frontend tests/独立 tsc/build 与已实现范围的生产本地 adapter/router restart/reopen smoke 通过；无 Provider 调用。尚未完成 reorder，不能声明完整 R7 PASS。
- 已复现 Workspace.Reorder 额外更新 UpdatedAt，违反本轮仅改 orderIndex 的严格契约；按执行文件第 5 节等待单行 foundation 修正授权，不静默改已审计基础。不 merge/push；详见 docs/hn/candidate_sequence_wiring.md。
