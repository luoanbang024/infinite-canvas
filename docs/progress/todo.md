---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## HN 审核后窄范围后续

- R7 stable closeout 为 R8 基线；R8 stable SequenceItem -> offline export API/adapter/bundle 已 GPT Audit PASS 并完成 fast-forward 集成与治理收口，该 R8 closeout 已 GPT Review PASS。仅移除 Export 当前 selection 依赖另记 OUR_VERIFICATION_FIX；R7 Reorder 修正保持。R9 三条 H.264/MP4 fixture 的剪映人工 handoff 已 AUDITED_VALIDATED；Provider/TaskBinding 继续延后独立决定。
- 下次 stable baseline 变化必须复核当前 HN SourceBaseline 常量/adapter/service/tests；R6 新准备绑定 ff32dc249811130a3db69be456e295be100b6e9f，历史 R4/R5 绑定不改写，当前绑定不自动覆盖后续升级。真实 Provider submit/idempotency、跨进程 writer、兼容 ID 映射和 unsupported bundles 仍需独立评审。
- R9 已完成安全 H.264/MP4 fixture 的人工导入/顺序/播放验证，等待本次 governance closeout Completion Review 后再选下一阶段；其他 codec/音频、自动 editor 集成与正式项目工作流继续独立评审。
