---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## HN 审核后窄范围后续

- 当前有效范围政策：API Key setup/input/detection/validation、账户凭据/余额/权限与 channel secret setup、live Provider validation 统一延期到后续独立凭据阶段；LIVE_VALIDATION_PREAUTH_STATUS = PAUSED_BY_USER_DECISION，LIVE_VALIDATION_AUTHORIZATION = NOT_GRANTED。详见 [凭据延期策略](../hn/api_key_deferral_and_credential_strategy.md)。缺少 Key 不阻塞无关 local/fake 工作；下一实现候选为不依赖真实凭据的本地产品/UI workflow，仍需单独执行文件与评审，本轮不启动。

- R7 stable closeout 为 R8 基线；R8 stable SequenceItem -> offline export API/adapter/bundle 已 GPT Audit PASS 并完成 fast-forward 集成与治理收口，该 R8 closeout 已 GPT Review PASS。仅移除 Export 当前 selection 依赖另记 OUR_VERIFICATION_FIX；R7 Reorder 修正保持。R9 三条 H.264/MP4 fixture 的剪映人工 handoff 已 AUDITED_VALIDATED；真实 Provider/TaskBinding adapter 继续延后独立决定；R10 fake TaskBinding lifecycle 已 GPT Audit PASS / fast-forward 集成，真实 adapter 仍未接线。
- 下次 stable baseline 变化必须复核当前 HN SourceBaseline 常量/adapter/service/tests；R12 provider identity freeze review 将新准备绑定更新为 16047f46e2186373ea824e12e84ae8dfa2ccde32，历史记录不改写，当前绑定不自动覆盖后续升级。真实 Provider submit/idempotency、跨进程 writer、兼容 ID 映射和 unsupported bundles 仍需独立评审。
- R9 已完成安全 H.264/MP4 fixture 的人工导入/顺序/播放验证，R9 governance closeout 已 GPT Review PASS；R10 本地 fake submission guard 已 GPT Audit PASS / 集成，等待 audit closeout Completion Review；其他 codec/音频、自动 editor 集成与正式项目工作流继续独立评审。

- R10 provider-neutral at-most-once guard 已 GPT Audit PASS / fast-forward 集成，保留 pending-test 的真实业务与多进程 availability 限制。等待其 closeout Completion Review。Provider/account/credential/live 工作按当前政策统一延期；现有 fake/local guard 保留，不阻塞单独评审的无凭据本地产品工作。

- R11 discovery 已 GPT Review PASS_WITH_REQUIRED_SCOPE_CORRECTION；R12 official H3 T2V adapter 和 identity freeze 修正已 GPT Audit PASS / fast-forward 集成，实现和测试字节保持，等待 audit closeout Completion GPT Review，详见 pending-test。R12 closeout 已 GPT Review PASS；R13/R14 已 AUDITED_INTEGRATED，R14 closeout 已 GPT Review PASS，实现和测试字节保持；R15 provider Result → 现有 R7/R8 editorial/export 已 GPT Audit PASS / AUDITED_VALIDATED，VALIDATION_ONLY / CROSS_BOUNDARY，无 implementation commit，R15 governance closeout 已 GPT Review PASS；既有 MiniMax readiness/one-task pre-auth 保留历史，现按用户决定 PAUSED_BY_USER_DECISION，不执行、不判失败。credential/channel secret、账户检查与 live Provider validation 统一延期；下一候选为无凭据本地 workflow，生产 route/UI 或其他实现仍需独立范围授权。
