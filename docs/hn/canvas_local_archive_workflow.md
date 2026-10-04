# Canvas 本地视频归档

EXECUTION_ID = HN_AI_IC_P0_B_R20_NO_CREDENTIAL_CANVAS_LOCAL_RESULT_ARCHIVE
TYPE = OUR_EXTENSION / PRODUCT_WIRING / LOCAL_ONLY
STATE = PENDING_GPT_AUDIT
UPSTREAM_BACKPORT = false

R19 GPT Audit PASS 后，仅接线同一个 R18 prepared Video 节点的当前本地视频。Hover Toolbar “归档本地视频”只打开 Dialog，HTTP GET/POST 和 journal write = 0；本地检查只读 Blob/journal。既有本地视频上传的 metadata spread 原样保留 hnLocalPrepared，归档只更新 hnLocalArchive，不改正常 metadata.status/内容/准备回执。

Dialog 与确认框始终显示：**仅把当前本地视频附加到此节点的历史冻结请求并归档；不证明 AI/Provider 生成成功，不调用模型。** LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS。入口对 Video 有内容或安全归档历史可见；写入必须有同 Canvas project/node、Shot/Generation/frozenHash 的有效历史 R18 回执。没有该回执时只展示安全历史并拒绝写入；不提供 Generation picker、另一节点附加或 backend status/read API。

只接受 video:/file: 本地持久化 Blob，0<size<=64 MiB，MIME 为 video/mp4 或 video/webm。server:/remote/cloud source 在 Blob read 前拒绝，不下载、不转换。缓存 exact Blob，实际 SHA-256/byteLength/MIME 构造 version=1 canonical JSON fingerprint；locator/name/mtime 不参与，同 bytes 改 locator 不触发新归档。

一张 active page 的 controller 以 JSON [exact Canvas project,node] pair 为 key，第一 await 前加锁。关闭/重开 Dialog 不释放进行中 lock；其他 owner 独立。发送顺序：lock -> exact owner/Blob -> explicit confirm -> journal ARCHIVING -> await localforage setItem -> getItem -> strict closed-shape/owner/attemptId/all facts equality -> R5 POST。journal 独立 store hn_local_archive_attempts；endpoint discovery 后 wire dispatch 前再次检查 pending barrier、owner 和 active page。journal 写入/读回失败或替换时 POST=0，无 auto retry/failover。

R5 fresh archive 非幂等：fresh #1=Result A/Job A，fresh #2=Result B/Job B。UI 保存 safe Canvas receipt 加 durable local journal 作为重复请求安全屏障；ARCHIVED 同 bytes 再操作 fresh POST=0。只有 known success 后 bytes 真正变化，用户明确点击“创建新的本地归档”并确认“会创建新的 Result 和 ArchiveJob，保留之前的本地归档；这不是重试。”才允许新的 Result/Job。

严格验证 422 exact Generation/安全 ResultID/ArchiveJobID/MIME，status pair 只能 ARCHIVE_FAILED+FAILED 或 RECEIVED+PENDING/COPYING/FINALIZING，未完成 hash/path 为空且 bytes=0。此时“显式重试同一归档”只使用原 Job/Result/Generation 与原 source SHA/bytes/MIME；源 bytes 不同即阻止。retry 不创建新 Result。成功 response 必须 exact generated relative path/hash/bytes/MIME，raw path 不存入 Canvas/journal。

首次 POST 的 drop/timeout/unexpected status/malformed JSON/invalid success or failure facts -> ARCHIVE_OUTCOME_UNKNOWN，不能自动/手动 fresh resend、不能以 changed bytes 新 Result override。未知首次没有可信 Job ID，停止并保留可能已有的 HN orphan facts，等待单独恢复设计。已知 job retry 的 response loss 只保留先前验证的 original Job IDs，后续明确 retry 仍同 job。成功/失败后 journal final write/read-back 不可靠同样保守 UNKNOWN；无法清除的 pending marker 在 reload 阻止 fresh。

Canvas/journal receipt 严格白名单：version、project/node/Shot/Generation/frozenHash、UUID attemptId、client observedAt、source SHA/bytes/MIME/fingerprint、outcome、按 outcome 的安全 job/status/hash/bytes/MIME。没有 raw storageKey、文件名、root/path、URL、prompt、Key、Provider response 或 Provider success。UI 只显示 hash 前 12 位；observedAt 是浏览器观察时间，不能冒充 backend archivedAt。

状态 NOT_ARCHIVED / ARCHIVING / ARCHIVED / ARCHIVE_FAILED_RETRYABLE / ARCHIVE_OUTCOME_UNKNOWN / SOURCE_CHANGED / RELOADED_UNVERIFIED。reload ARCHIVING -> UNKNOWN，已知成功/失败同 bytes -> RELOADED_UNVERIFIED，不能宣称重新查验 backend。移除/undo Canvas projection 不会清除 page/journal 历史；journal 单独没有 R18 prepared receipt 不授权写入。迟到 completion 留存原 owner journal，只有 active project 与 exact prepared owner 仍匹配才 merge receipt。

只复用 R5 archiveLocalResult/retryLocalArchive 和既有 local-endpoint。全部 fetch credentials omit、redirect error、单次 exact local POST；没有 Provider/R7/R8 调用。R18 Protocol/ProviderIdentity empty，ConnectionID absent，SourceBaseline 16047f46e2186373ea824e12e84ae8dfa2ccde32 不改写，历史 Generation 不修改。

API_KEY_SETUP / ACCOUNT_CREDENTIAL_CHECK / ACCOUNT_BALANCE_CHECK / ACCOUNT_ENTITLEMENT_CHECK / CHANNEL_SECRET_SETUP / LIVE_PROVIDER_VALIDATION = DEFERRED；API_KEY_INPUT / API_KEY_DETECTION / API_KEY_VALIDATION = SKIPPED；LIVE_VALIDATION_AUTHORIZATION = NOT_GRANTED；LIVE_VALIDATION_PREAUTH_STATUS = PAUSED_BY_USER_DECISION；PROVIDER_BOUND_CALLS / REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0；REAL_CREDENTIAL_READ / REAL_REMOTE_MEDIA_DOWNLOAD = NONE。

完整 root Go、Bridge、101 frontend tests（R20 focused 22）、独立 tsc、production build、R5/R6/R7/R8/R10/R12/R13/R14/R18 和 Foundation preservation PASS。Go production handler/service/Foundation 证明 fresh A/B 不同 IDs、同 job retry、close/reopen 与独立进程 reopen 后 exact media hash/bytes/sidecar/local provenance。测试使用合成 MP4 signature fixture，不宣称完整 codec playback。浏览器实际 IndexedDB/DOM 点击未验收；现有 ReactDOM SSR + injected async persistent journal/controller 和 localhost HTTP server 是自动证据。

限制：page lock 不是跨 tab/global exactly-once；localforage acknowledged read-back 不宣称 OS fsync 或跨 Canvas/HN store 原子事务；用户手动清除全部浏览器 journal/receipt、跨进程 writer、未知首次恢复均未解决。Canvas 仍原有 debounced persistence。保守 UNKNOWN 可能牺牲 availability，不删除 HN 历史。后续真实浏览器布局/主题/缓存丢失测试需独立验收。

仅五个允许 production files、四个新增 tests、六个 governance files。R5/Foundation/R10–R14/R18 helper 和正常 Generate/Upload/Cloud source 不改；R7/R8 独立 OUR_VERIFICATION_FIX 与 R9/R15 records 保持。撤销条件：单独评审后 revert 该 UI/helper/metadata/test/governance patch，保留历史 HN facts；将来等效 upstream 工作必须重新评审，不能称 upstream backport。

三类本地 commits；不 merge、不 push，等待 GPT Review / Audit。
