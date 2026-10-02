---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## HN 审核后窄范围后续

- R7 stable closeout 为 R8 基线；R8 stable SequenceItem -> offline export API/adapter/bundle 已 GPT Audit PASS 并完成 fast-forward 集成与治理收口，该 R8 closeout 已 GPT Review PASS。仅移除 Export 当前 selection 依赖另记 OUR_VERIFICATION_FIX；R7 Reorder 修正保持。R9 三条 H.264/MP4 fixture 的剪映人工 handoff 已 AUDITED_VALIDATED；真实 Provider/TaskBinding adapter 继续延后独立决定；R10 fake TaskBinding lifecycle 已 GPT Audit PASS / fast-forward 集成，真实 adapter 仍未接线。
- 下次 stable baseline 变化必须复核当前 HN SourceBaseline 常量/adapter/service/tests；R12 provider identity freeze review 将新准备绑定更新为 16047f46e2186373ea824e12e84ae8dfa2ccde32，历史记录不改写，当前绑定不自动覆盖后续升级。真实 Provider submit/idempotency、跨进程 writer、兼容 ID 映射和 unsupported bundles 仍需独立评审。
- R9 已完成安全 H.264/MP4 fixture 的人工导入/顺序/播放验证，R9 governance closeout 已 GPT Review PASS；R10 本地 fake submission guard 已 GPT Audit PASS / 集成，等待 audit closeout Completion Review；其他 codec/音频、自动 editor 集成与正式项目工作流继续独立评审。

- R10 provider-neutral at-most-once guard 已 GPT Audit PASS / fast-forward 集成，保留 pending-test 的真实业务与多进程 availability 限制。等待其 closeout Completion Review。后续候选为 Provider/account/adapter binding 决策，不代表已授权 live submit。

- R11 discovery 已 GPT Review PASS_WITH_REQUIRED_SCOPE_CORRECTION；R12 official H3 T2V adapter 和 identity freeze 修正已实现待 GPT Review / Audit，详见 pending-test。下一候选仅 fake-only polling/recovery，尚未授权；真实 credential/channel 业务接线、生产 route/UI、live 调用与媒体下载各需独立 gate。
