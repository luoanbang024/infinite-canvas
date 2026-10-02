---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## HN 审核后窄范围后续

- R6 closeout 已 GPT Review PASS；R7 Candidate/显式选择/SequenceItem/reorder 本地执行完成，等待 GPT Review / Audit，未集成/push。仅用户授权的 Reorder 单行 UpdatedAt 修正另记 OUR_VERIFICATION_FIX。后续 selected Sequence -> offline export/剪映人工 handoff 仍需单独执行与审核，不在 R7 启动；Provider/TaskBinding 继续延后决定。
- 下次 stable baseline 变化必须复核当前 HN SourceBaseline 常量/adapter/service/tests；R6 新准备绑定 ff32dc249811130a3db69be456e295be100b6e9f，历史 R4/R5 绑定不改写，当前绑定不自动覆盖后续升级。真实 Provider submit/idempotency、跨进程 writer、兼容 ID 映射和 unsupported bundles 仍需独立评审。
- 独立执行安全本地素材的剪映人工导入/顺序/播放验证，当前离线结构通过不等于 codec/handoff 实测。
