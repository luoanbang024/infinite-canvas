---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## HN 审核后窄范围后续

- R3 与 closeout 已 GPT Review PASS / 集成；R4 provider-neutral Generation prepare/freeze 已 GPT Audit PASS 并集成；先 Review closeout Completion，再单独批准 prepared Generation + successful local media 的 Result/ArchiveJob 执行，不伪造历史提交 provenance；本 closeout 不开始新接线。
- 下次 stable baseline adoption 必须复核 R4 SourceBaseline 常量/adapter/service/tests；当前绑定不自动覆盖后续升级。真实 Provider submit/idempotency、跨进程 writer、兼容 ID 映射和 unsupported bundles 仍需独立评审。
- 独立执行安全本地素材的剪映人工导入/顺序/播放验证，当前离线结构通过不等于 codec/handoff 实测。
