# Canvas 主序列安全导出

本地归档 Dialog 内的“主序列安全导出”作用于 HN project/main，独立于当前视频节点。只使用 R30 authoritative snapshot 与 R33 export command/job/receipt。

加载安全导出：显式 R30 init → R33 init → readonly snapshot；重新读取只做 readonly snapshot。确认导出绑定 revision 字符串和完整有序项，不编辑导出顺序。需要更改顺序时先使用已审计的安全排序，再重新读取。

每个 intent 独立记录。PREPARED → DISPATCHING 均需 await 写入与严格读回；真实 POST 仅在 DISPATCHING 可靠读回之后发送。响应丢失不能当作失败；UNKNOWN 阻止新命令，不自动 retry。检查服务器状态仅 exact lookup；NOT_OBSERVED 仍不能解除。继续同一次导出是需确认的 mutation，保留 exact intent/revision/vector/format，不创建新 UUID。已知非终态保持 unresolved，多条 intent 分别恢复，不合并、不清理。

COMMITTED/REJECTED/FAILED 必须 durable seal 后显示。历史 receipt 与当前快照分开，后来顺序改变不改历史导出。当前包健康只在显式点击验证时读取，VERIFIED/MISSING/CORRUPT + observedAt 仅存在内存，reload 清除；缺失或损坏不自动重建。导出不生成媒体，不代表 AI/Provider 成功。


## R34 Canvas main Sequence safe export UI — pending independent audit

PROJECT_ID = HN_AI_IC
EXECUTION_ID = HN_AI_IC_P0_B_R34_NO_CREDENTIAL_CANVAS_SEQUENCE_EXPORT_UI
BASE_SHA = 150c0bd345804c078b93a20ebb910ff0c413f4a9
TYPE = OUR_EXTENSION / LOCAL_ONLY / CANVAS_UI_WORKFLOW / NO_PROVIDER
STATE = PENDING_GPT_AUDIT
IMPLEMENTATION_COMMITS = b1565397bc8adc74a9826f258d20713e8453fab5, f0c9221388317638c7be82b0af5c2858f46d37f3

Page-owned project/main controller and dedicated per-intent localforage journal use the unchanged R30 snapshot and R33 typed command/job/receipt transport. Opening the dialog only inspects the local journal. Explicit load initializes R30 then R33 metadata and reads a complete <=256 main snapshot. Exact revision/vector are confirmed before a new UUID. PREPARED and immediately pre-POST DISPATCHING both require strict durable readback; persistence failure forwards zero export POSTs. Ambiguity remains UNKNOWN; automatic POST retry=0. Known nonterminal status remains unresolved. Exact lookup and explicitly confirmed same-key continuation preserve every unresolved identity. No current snapshot is rebound during continuation. Terminal authority is sealed before UI projection. Historical COMMITTED receipt, current Sequence preview and explicit ephemeral bundle health remain separate. MISSING/CORRUPT never rebuild.

Only the five authorized production paths, three new tests and six governance paths change. All backend/Foundation/schema, R33/R30 typed transports, legacy export, R31/R28 controllers/journals, archive/candidate/selection controllers, Generate/Upload/Provider/Auth, dependencies and lockfiles remain byte-identical. Existing placement/reorder UI remains. No node metadata receipt, browser media read, editor/folder launch, compound or drag/drop.

Final actual Go/mod verify, full frontend, focused R34/R33 transport, independent TypeScript and isolated frontend + Bridge production build evidence are retained in Completion. No installation/upgrade. Controlled render/click, injected localforage/JSON reopen and localhost HTTP evidence are automated; real IndexedDB, real dialog/browser reload, two real tabs, long export UX, themes/layout/accessibility, rapid clicks/cancel and large-bundle verification responsiveness require manual acceptance and are NOT_CLAIMED.

PROVIDER_CALLS = 0
PAID_CALLS = 0
LIVE_CALLS = 0
REAL_CREDENTIAL_READ = NONE
REAL_ENV_CONTENT_READ = NONE
REAL_REMOTE_MEDIA_DOWNLOAD = NONE
API_KEY_SETUP = DEFERRED
API_KEY_INPUT = SKIPPED
API_KEY_DETECTION = SKIPPED
API_KEY_VALIDATION = SKIPPED
ACCOUNT_CREDENTIAL_CHECK = DEFERRED
ACCOUNT_BALANCE_CHECK = DEFERRED
ACCOUNT_ENTITLEMENT_CHECK = DEFERRED
CHANNEL_SECRET_SETUP = DEFERRED
LIVE_PROVIDER_VALIDATION = DEFERRED
LIVE_AUTHORIZATION = NOT_GRANTED
PUSH = NONE
MERGE = NONE
NEXT_ACTION = Independent GPT Review of R34 Completion; no R35.
