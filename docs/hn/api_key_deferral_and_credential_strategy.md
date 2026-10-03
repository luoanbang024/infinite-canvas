# API Key 延期与统一凭据阶段策略

EXECUTION_ID = HN_AI_IC_P0_B_R16_API_KEY_DEFERRAL_GOVERNANCE
EXECUTION_TYPE = GOVERNANCE_ONLY / SCOPE_POLICY_UPDATE
BASE_OUR_COMMIT = 730602992a748cc332d7b61c0cbefeb0f68ecb06
DECISION_SOURCE = 用户最新明确决定

API Key 相关工作现在统一延期，后面集中处理。本次仅记录调度与范围政策，不改变产品行为，不撤销已完成的 fake/local 能力。此前 MiniMax readiness / one-task live validation / pre-auth 保留历史，当前暂停，不执行、不判失败；Readiness PASS 从来不等于 live 付费调用授权。

```text
API_KEY_SETUP = DEFERRED
API_KEY_INPUT = SKIPPED
API_KEY_DETECTION = SKIPPED
API_KEY_VALIDATION = SKIPPED
ACCOUNT_CREDENTIAL_CHECK = DEFERRED
ACCOUNT_BALANCE_CHECK = DEFERRED
ACCOUNT_ENTITLEMENT_CHECK = DEFERRED
CHANNEL_SECRET_SETUP = DEFERRED
LIVE_PROVIDER_VALIDATION = DEFERRED
LIVE_VALIDATION_AUTHORIZATION = NOT_GRANTED
REAL_PROVIDER_CALLS = 0
PAID_CALLS = 0
REAL_TASK_CREATED = 0
REAL_CREDENTIAL_READ = NONE
REAL_REMOTE_MEDIA_DOWNLOAD = NONE
LIVE_VALIDATION_PREAUTH_STATUS = PAUSED_BY_USER_DECISION
```

## 既有审计边界保持

R10 = PRESERVED（at-most-once submission guard）
R12 = PRESERVED（official H3 submit adapter / identity freeze）
R13 = PRESERVED（known-task polling/recovery）
R14 = PRESERVED（provider result/archive）
R15 = PRESERVED（provider Result → Candidate/Sequence/Export fake/local audited validation）

不回滚、不重构，不改变 R7/R8 独立 OUR_VERIFICATION_FIX 或 R9/R15 validation record。UPSTREAM_BASELINE.json 保持字节一致；这是用户范围政策，不能放入 auditedIntegrations/auditedValidations 或伪装成 integration patch、verification fix、upstream backport。

```text
NEW_GENERATION_SOURCE_BASELINE = 16047f46e2186373ea824e12e84ae8dfa2ccde32
GENERATION_SOURCE_BASELINE_CHANGE = NONE
HISTORICAL_GENERATION_REWRITE = NONE
```

## 不以真实凭据为无关工作的阻塞

缺少 API Key 不应阻塞 local/fake testing、local Canvas workflow、Candidate/Sequence、本地 export/handoff、非 secret UI、以及不联系 Provider 的 error/recovery UX。后续候选为 NO_CREDENTIAL_LOCAL_PRODUCT_WORKFLOW_EXECUTION；每项实现仍需独立范围与评审。本次不启动这些实现，不选择 Canvas UI 重设计，不把政策变为自动放行其他开发。

## 当前禁止事项

未来独立凭据执行明确授权前，不为 HN live setup 新增 API Key 输入框、key-presence detector、真实 Key validation request、account balance/entitlement API；不 live submit/poll/download，不 secret migration、不第二个 credential store、不 credential logging。不得在无关 UI 工作中隐含带入 live route、secret 检测或 Provider background worker。现有源码保留，政策更新不修改既有实现运行逻辑。

## 统一延期的未来 workstream

account model → credential type → credential storage/runtime boundary → ConnectionID ownership → input/setup UX → presence/detection behavior → validation behavior → entitlement/balance policy → live Provider validation。

该链条本轮全部不执行；未来须统一独立审查与明确授权，不散落到多个无关阶段。此前 live request 的提示词、旧价格、预算和授权模板只是保留历史；重新开启时必须重核最新官方合同、价格、账户与安全边界，并取得新的明确 live 调用授权。

## 本次交付范围

只创建策略文档并更新 CHANGELOG、PATCH_LEDGER、todo、pending-test。service/handler/hn/foundation/web/src、test/dependency/lockfile/Auth/upstream DB schema/config/settings 均保持。只在 feature/p0-b-r16-api-key-deferral 上创建一个 governance-only commit；不 merge，不 push，等待 GPT Review。
