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


## HN P0-B R4（本地 feature 完成，等待 GPT Review / Audit）

- 内部 Canvas 视频 intent builder 与本机 Generation prepare/freeze 端点已实现；T2V 或 R3 精确本地图片版本绑定 -> PREPARED；当前 reviewed baseline 显式记录，无旧默认值。未接 live submit/UI。
- root Go、Bridge、32 frontend tests、独立 tsc、production build、真实本机 API + production adapter 的合成 restart/reopen/二次 attempt 验证与 scope/secret scans PASS；未 push、未 merge our-main，详见 docs/hn/generation_prepare_freeze_wiring.md 和 Completion。
- workflow/element/video/audio/remote-only 参考、带 URL 文本、任意未标注秘密、跨进程 writer、安全 ID 映射、真实 browser/submit idempotency 均保留后续评审约束。Provider 选择仍 DEFERRED；Result/Archive runtime 无接线。
