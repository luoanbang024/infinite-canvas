# Canvas 本地视频准备

EXECUTION_ID = HN_AI_IC_P0_B_R18_NO_CREDENTIAL_CANVAS_LOCAL_PREPARE
TYPE = OUR_EXTENSION / PRODUCT_WIRING / LOCAL_ONLY
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS
UPSTREAM_BACKPORT = false

只在 clean empty Video intent 的 Hover Toolbar 显示“本地准备”。已有内容、工作流、任务或非 idle 状态不可用。原生成、重试、快捷键和 Agent 路径保持。

点击只打开“本地视频准备” Dialog，HTTP write = 0。始终显示：**仅冻结本地业务请求，不调用模型、不生成视频。渠道未绑定。** 首次点击“冻结本地准备”创建一个 attempt；再次点击“创建新的本地准备”须确认“这会创建新的 Generation，保留已有准备记录。是否继续？”

Canvas ID 只接受 `^[A-Za-z0-9_-]{1,128}$`，HN ID 固定 `canvas-<SHA256(exact UTF-8 ID)>`；不 trim/lowercase/raw fallback/migrate。sourceNodeId 是 exact Video ID，Shot 固定 label 为 Shot。同 project/node 复用原 Shot，不 rename。

页面只投影已经存在的非敏感视频业务标量和 prompt/duration 对，不传 AiConfig/channel/Key；纯 Canvas context + camera prompt。只接受 exact image: Blob，0<size<=16MiB，所有 Blob 在 Shot 写入前读取、验证、缓存并计算 hash。参考顺序、角色和重复绑定保持；不支持 remote/server/data-only/video/audio/workflow/element 或找不到的选中 ID。

复用现有 ensureLocalShot / prepareLocalShotGeneration / R3 reference local API。Protocol 和 ProviderIdentity 为空，ConnectionID absent；Model 可以为空。SourceBaseline 保持 `16047f46e2186373ea824e12e84ae8dfa2ccde32`，与本轮 UI Git base `c47cc05960bea70a2f69d12aca28160e37a14ccd` 区分，历史 Generation 不改写。未来绑定 Provider 需要新 Generation，不能修改已有 frozen local intent。

receipt 仅 `hnLocalPrepared` 的严格白名单：version、Canvas/HN project、node/Shot/Generation ID、frozenHash、server preparedAt、sourceBaseline、fingerprint 和安全摘要 model/seconds/vquality/size/referenceCount。没有完整 prompt、params、storage-key 数组、URL、Key、channel、provider response、root path 或 provider status。显示 hash 仅前 12 字符。

fingerprint 为 schemaVersion/project/node/applied prompt/model/empty binding/canonical parameters/baseline/ordered reference role、key、source image identity、SHA、bytes、MIME 的 deterministic JSON SHA-256。额外 image identity 确保相同 key/bytes 的 frame 节点切换可辨别；标题、位置、timestamps 不参与。

页面 controller 的 Map 以 exact project+node JSON pair 为 key，第一 await 前加锁。Dialog 关闭/重开不会清锁；项目切换、删除/重建或 page dispose 禁止旧 completion 回写其他节点。单 attempt 最多一个 prepare POST；不会自动 retry/failover。只有已知 400/403 rejection 证明 prepare 未成功；drop/timeout/5xx/malformed 等可能提交为 OUTCOME_UNKNOWN，用户需再次显式确认新 attempt。

状态：UNPREPARED / PREPARING / CURRENT / STALE / RELOADED_UNVERIFIED / ERROR / OUTCOME_UNKNOWN。CURRENT 只限当前 session 严格验证成功 + 当前 fingerprint 一致；变化显示 STALE 并保留历史。reload 相同 fingerprint 仍是“历史准备回执，未复核”，没有新增 backend query。可见 Dialog 每 2 秒重查本地 fingerprint/Blob，不 HTTP、不 Provider polling，关闭清理 timer。错误只显示静态安全类。

正常 Canvas project persistence 保持既有 400ms debounce，无新增 store/schema 字段。隔离 storage adapter 的两进程 smoke 验证实际 useCanvasStore updateProject/persist/rehydrate，reload 同 fingerprint 未验证、改 prompt stale、额外网络 0；不声称真实 IndexedDB/DOM 浏览器测试。Go production handler composition 验证 isolated HN durable facts 和 child-process reopen。Canvas 与 HN DB 无跨库事务，失败/退出可能保留 Shot/Reference/Generation 而无 receipt；不自动清理、查询或重发。

API_KEY_SETUP、ACCOUNT_CREDENTIAL_CHECK、ACCOUNT_BALANCE_CHECK、ACCOUNT_ENTITLEMENT_CHECK、CHANNEL_SECRET_SETUP、LIVE_PROVIDER_VALIDATION = DEFERRED；API_KEY_INPUT / API_KEY_DETECTION / API_KEY_VALIDATION = SKIPPED；LIVE_VALIDATION_AUTHORIZATION = NOT_GRANTED；LIVE_VALIDATION_PREAUTH_STATUS = PAUSED_BY_USER_DECISION。PROVIDER_BOUND_CALLS / REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0；REAL_CREDENTIAL_READ / REAL_REMOTE_MEDIA_DOWNLOAD = NONE。

Root Go、Bridge、79 frontend tests、独立 tsc、隔离 production build 和 focused preservation PASS。当前已有 ReactDOM rendering + direct event/controller tests；没有新增 DOM 测试依赖。真实用户浏览器布局、IndexedDB、跨进程唯一性、live/provider/codec/editor 能力均不在本次验收声明中。R7/R8 independent verification fix、R9/R15 validations、Foundation 和 R10/R12/R13/R14 原实现字节保持。已审计 feature 正常非 force 推送并 fast-forward 集成；本次只做 governance-only closeout，focused verification 与最终 our-main push/stable SHA 见收口 Completion，等待其 GPT Review。


## R18 审计收口

- FEATURE_HEAD = 7073b6e7b6ffb9672671f9f787916db7c745b2de；AUDITED_COMPLETION_SHA256 = a9291465c035c71cccff29f2f6fab22856894abe66c83d85f565c87c5b777c42；80 ZIP members、79 payload bytes/hash、CRC 和 exact final scan inventory 在 push 前重新核验通过。原三个 commits 不重写；R18 实现/测试保持字节不变。
- CANVAS_RECEIPT_BACKEND_AUTHORITATIVE_AFTER_RELOAD = NO；RELOAD_STATE = RELOADED_UNVERIFIED；CANVAS_HN_CROSS_STORE_ATOMICITY = NOT_CLAIMED；BACKEND_READ_STATUS_ENDPOINT = NONE。
- PROVIDER_BOUND_GENERATION = NONE；FUTURE_DIRECT_SUBMIT_OF_R18_GENERATION = NOT_ALLOWED；ACTUAL_BROWSER_INDEXEDDB_ACCEPTANCE = NOT_CLAIMED；RESULT_CANDIDATE_SEQUENCE_EXPORT_UI = NOT_IN_R18；LIVE_PROVIDER_VALIDATION = DEFERRED。
- 本次恰好一个 governance-only closeout commit；仅 normal non-force 已审计 feature 与 our-main push。focused checks、remote-ref 验证和 stable SHA 只记录 Completion。R7/R8 两项 verification fix、R9/R15 validations、R16 deferral、当前 SourceBaseline 与所有既有审计源码保持。没有 PR/Release/tag/settings 或后续阶段开发。
