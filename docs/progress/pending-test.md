---
title: 待测试
description: 当前版本已实现但仍需人工验证的变更项
---

# 待测试

## HN P0-B R2 本地基础（待 GPT Review / 用户接线验证）

- 独立 hn/foundation 已实现 project workspace、SQLite v1、ReferenceVersion、冻结 Generation、TaskBinding/Result、可靠本地 ArchiveJob、Shot/Candidate/Sequence 和离线 JSON/CSV 交接结构；合成 fixtures 的自动验证通过，尚未接线 UI/Provider。
- 审查重点：项目/关系隔离、Windows 路径、单 writer 调用约束、文件/metadata 对账、旧结果不覆盖选择；详见 docs/hn/p0_b_foundation_implementation.md。
- 剪映素材导入/播放/codec 与正式业务工作区手工接线验证延后；本轮不启动编辑器、不调用 Provider、不修改用户数据。
