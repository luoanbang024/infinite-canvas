# Canvas 本地候选显式选择

EXECUTION_ID = HN_AI_IC_P0_B_R24_NO_CREDENTIAL_CANVAS_LOCAL_SELECTION
TYPE = OUR_EXTENSION / PRODUCT_WIRING / LOCAL_ONLY
STATE = PENDING_GPT_AUDIT

## 唯一入口与语义

复用“归档本地视频”Dialog 的“本地剪辑候选”区，严格 R22 Candidate + 有效 R20 历史 ARCHIVED owner 才能显式“选择此候选”。Selection 是修改 Shot 当前候选的命令，不是查询。不会自动选择、不创建候选、不加入序列、不 Reorder、compound 或 Export；不改正常 Generate。

固定显示：“选择会更新此镜头当前使用的候选；不会加入序列，也不会导出，不代表 AI/Provider 生成成功。” LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS。历史 reloaded Candidate 允许明确选择，由既有本地后端校验归属及归档完整性并写入；不把 R20/R22 reload 状态升级。

## Mutation 与未知结果

生产 Select 对同 Candidate ID 收敛，但每次重新采样/写入 Shot.UpdatedAt，因此 byte idempotent=NO；不保证时间戳一般严格递增。A→B→later A 可重新选择 A，没有 revision/CAS/server intent dedup。再次点击“重新选择此候选”是新的 mutation、确认和 UUID intentId，警告“再次选择会重新写入当前选择，并可能覆盖之后由其他操作设置的候选。” intentId 仅浏览器关联，不发送到后端。

POST loss/timeout/500/非严格回执为 SELECTION_OUTCOME_UNKNOWN，自动 resend=0。page-local 同 Shot barrier 保留 exact owner；禁止 Archive、Candidate ensure 和 different-target Selection，只允许同 target 用户重新确认后发一个新 Select POST。known rejection/cancel 不创建新 barrier；已存在 UNKNOWN 不被旧 success 回执消除。reload 不保存此内存 barrier，回执只能 advisory/unverified。

## 锁与历史 owner

page-owned controller/coordinator 的主锁为 JSON.stringify([hnProjectId,shotId])，辅以 exact Canvas project/node token。first await 前同步取锁，三种操作六方向互斥，UI disabled 与实际 handler gate 都生效，Dialog 关闭重开不释放锁；token/epoch 防止 stale finally 释放新锁及跨项目写回。不能防止其他 tab/process 或已经发出的迟到请求；不声称全局 stale-intent rejection。

Selection 仅调用已有历史 reader 和 Candidate validator，读取 journal/历史 projection。传入 reader 的精简节点只有 owner metadata；不读取当前 content/storageKey/Blob，不上传、下载或重新归档。每个 await/POST/merge 边界重新校验完整 owner。historical source changed/reload 底层 ARCHIVED 可选；FAILED/ARCHIVING/UNKNOWN 不可选。仅当前投影候选，无历史 picker。

## Advisory receipt 与持久化

hnLocalSelection 严格 13 keys：version、canvasProjectId、hnProjectId、sourceNodeId、shotId、generationId、preparedFrozenHash、resultId、archiveJobId、sourceMediaFingerprint、candidateId、intentId、observedAt。不含后台 timestamp/version、label、Provider、Sequence、URL/path/key/raw body。只有 exact acknowledgement 且 owner 稳定才 merge，其他 metadata/status/media 和 SourceBaseline 不变。selection 回执不触发 R20 Blob inspect revision。

复用已有 Canvas persistence：未登录浏览器本地保存，既有已登录账号/云同步流程未修改，本轮不访问真实账号或凭据。回执重载为 SELECTION_RELOADED_UNVERIFIED；缺失回执不表示后端未选中。固定“本页没有可靠的当前选择查询；再次选择是写入操作。”没有新增 current-selection GET。

## 验证与限制

root Go、go mod verify、Bridge、全部 frontend、独立 tsc、隔离 production build，以及 Foundation/R5/R6/R7/R8/R10/R12/R13/R14/R18/R20/R22/R24 focused preservation PASS。139 frontend PASS，1 个 R22 handler-bridge prerequisite 测试在独立 frontend run 条件跳过；同一个 bridge 在 Go handler fixture 实际执行 PASS。实际生产 handler 证明同 A 时间重写、A→B→A、committed-response-loss 无自动重发及实体库存不变。R24 localhost server 观察 first POST=1、explicit recovery additional POST=1；redirect follow=0、first-await lock、六方向互锁、unknown barrier/owner drift/reload、无 Blob/Provider side effect 已测。

SSR/render/受控事件与 controller harness、合成 journal 及 receipt JSON roundtrip 为自动证据；不宣称真实 DOM/IndexedDB、布局主题、人工端到端验收通过。真实浏览器需按 pending-test 验收。build 使用源码隔离副本与已安装依赖，无 env/key/user DB/media 读取，无 install/update。

API Key setup/account/secret/live 全部 DEFERRED；input/detection/validation SKIPPED；pre-auth PAUSED_BY_USER_DECISION；授权 NOT_GRANTED；Provider/real/paid calls=0，REAL_CREDENTIAL_READ/REAL_REMOTE_MEDIA_DOWNLOAD=NONE。R7/R8 独立 verification fix、R9/R15 validation、R16 policy、Foundation/schema/Auth/dependency/lockfile、R5/R7/R10/R12/R13/R14 实现与 R20/R22 controller 字节保留。HN_GENERATION_SOURCE_BASELINE 仍为 16047f46e2186373ea824e12e84ae8dfa2ccde32，没有 Generation 新建或历史改写（仅隔离测试 fixture 构造前置实体）。本地 feature 不 merge/push，等待 GPT Review / Audit。
