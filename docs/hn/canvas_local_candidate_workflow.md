# Canvas 本地已归档版本候选

EXECUTION_ID = HN_AI_IC_P0_B_R22_NO_CREDENTIAL_CANVAS_LOCAL_CANDIDATE
TYPE = OUR_EXTENSION / PRODUCT_WIRING / LOCAL_ONLY
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS
BASE_OUR_COMMIT = 53fdd43a7c2f849145c35cb053164afcab9dfdfb

## 操作与事实边界

R22 复用同一个“归档本地视频”Dialog，不增加工具栏动作。严格有效的历史 R20 ARCHIVED Result 可由用户点击“将已归档版本加入候选”并确认；已有历史候选或结果未确认时，必须明确点击“重新确认候选”。

“此操作只把已归档的本地版本登记为剪辑候选；不会自动选中，也不会加入序列，不代表 AI/Provider 生成成功。”

“操作的是回执中的历史已归档版本，不会上传或归档当前节点视频。”

LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS。Candidate 仅表示“已归档的本地版本可作为剪辑候选”。没有 Selection、Sequence placement、Reorder、compound helper 或 Export 接线。

## 历史读取与锁

HNLocalArchiveController 只新增 readHistoricalArchiveReceipt；复用 private history 的 Canvas/journal 仲裁。持久 journal 读取不可靠、回执矛盾、prepared owner 不一致、节点/项目/生命周期变化时停止。reader 无状态/journal 写入，无 Blob/source inspection/HTTP。原 R20 controller 其余 bytes 完全保持。

RELOADED_UNVERIFIED 底层 ARCHIVED 与 SOURCE_CHANGED 历史 ARCHIVED 可显式 ensure；FAILED、ARCHIVING、UNKNOWN 均不可。当前本地/远程/missing storageKey 不授予 Candidate 权限，Candidate 操作不读取当前 Blob、不 archive、不上传/下载。历史提示只控制已有入口可见性，不是授权。

Candidate controller 在 page useRef 中保存，exact project + node entry 在第一个 await 前同步 running。Dialog 关闭/重开不释放运行锁；R20 archive 按 Candidate running 拒绝，Candidate 按 R20 running 拒绝。候选回执更新不触发当前 Blob 的 R20 inspect。项目/节点/owner 改变、dispose 和 late response 不写回错误 owner。

## 请求与回执

只复用既有 R7 ensureCandidate：一次本地 endpoint discovery GET，最多一次 exact local ensure POST，label 固定 Local Archive。credentials omit、redirect error、静态安全错误；闭合校验 code/data/msg、Candidate 的 project/Shot/Generation/Result/ARCHIVED facts、ID、安全旧 label 和 timestamps。服务返回安全旧 label 予以接受，不 rename。

POST response loss/timeout/无法严格校验为 CANDIDATE_OUTCOME_UNKNOWN；自动 retry=0。只有用户重新确认同一历史 target 才再 ensure。当前 single-process service writer 已用真实 production handler 验证：同 Shot + Result 返回同 CandidateID，response-loss 显式恢复后 Candidate count=1。此结论不扩展为多进程唯一性保证。

hnLocalCandidate 是 13-field closed projection：version、canvasProjectId、hnProjectId、sourceNodeId、shotId、generationId、preparedFrozenHash、resultId、archiveJobId、sourceMediaFingerprint、candidateId、availabilityStatus、observedAt。它不含 provider-success/status、Selection、sequence、URL、path、key、request/response 或 error。只有通过归属与历史验证的回执 merge 到 hnLocalCandidate；其他节点与 metadata 保持。

receipt 持久化复用 Canvas store。未登录项目保存在浏览器本地；既有登录账号/云端同步行为未改动，本轮不访问账号。reload receipt 为 CANDIDATE_RELOADED_UNVERIFIED，不能自动 READY；成功候选也不把 R20 RELOADED_UNVERIFIED 提升成 ARCHIVED。缺失或非法 projection 不构成成功事实。

## 验证与限制

root Go、Bridge、全部 frontend、独立 tsc、隔离副本 production build 与 Foundation/R5/R6/R7/R8/R10/R12/R13/R14/R18/R20/R22 focused checks PASS。实际 Go production handler + Bun controller localhost bridge：first POST=1、自动 retry=0、explicit additional POST=1、Candidate count=1、same ID、protected records unchanged、forbidden calls=0、Blob reads=0。篡改 archived media/receipt/hash/bytes/ownership 和未归档 Result 拒绝。

SSR/UI handler harness、合成 journal、实际 Canvas store + 合成 localforage adapter 的 isolated persist/rehydrate 已测；不宣称真实浏览器 DOM/IndexedDB 或主题/布局人工验收通过。production build 不读取 .env，使用现有依赖副本，无依赖更新。完整证据、source diffs/hash maps、最终 ZIP 精确扫描清单在 Completion。

API Key setup/input/detection/validation、account/balance/entitlement、channel secret、live Provider validation 继续延期；input/detection/validation SKIPPED，pre-auth PAUSED_BY_USER_DECISION，授权 NOT_GRANTED。PROVIDER_BOUND_CALLS/REAL_PROVIDER_CALLS/PAID_CALLS=0；真实凭据读取、远程媒体下载 NONE。原 SourceBaseline 16047f46e2186373ea824e12e84ae8dfa2ccde32 不变；没有 Generation 重写或 Provider/Auth/schema/Foundation/dependency/lockfile 修改。R7/R8 独立 OUR_VERIFICATION_FIX 与 R9/R15 validation 记录保持。

原三个已审计 commits 已正常非 force 推送并 fast-forward 集成；本次只有一个 governance-only closeout commit。focused verification、our-main 正常推送与实际 stable SHA 见收口 Completion，等待其 GPT Review。未来 Selection/Sequence/Export 或多进程写入能力需独立执行范围与评审。


## R22 审计收口

- AUDITED_FEATURE_HEAD = ee5edecfde7b6ffe6c4fc63ccfee46c0ef749467；AUDITED_COMPLETION_SHA256 = af6795f292627947ea4353a38c1b03453cfbbce1ab230ccfd746729d1a8cf36b；110 actual ZIP members / 109 payload bytes/hash / CRC / exact 110-member secret-scan inventory 在任何 push 前重新核验 PASS。
- R22 production/test raw bytes 与 R20 archive/write 方法保持；Candidate only=true；同 Shot + Result 返回同 CandidateID，仅限 current single-process writer；response-loss auto retry=0；explicit recovery=same-target ensure；Candidate count after recovery=1；reload=CANDIDATE_RELOADED_UNVERIFIED；R20 archive state promotion=NONE；Selection/Sequence/Reorder/compound/Export=NONE；PROVIDER_BOUND_CALLS=0。
- R5/R7/R8/R10/R12/R13/R14/R18/R20/Foundation/schema/依赖/lockfile/Auth/SourceBaseline 保持；R7/R8 独立 OUR_VERIFICATION_FIX 与 R9/R15 validation records 保持；R16 API Key/account/channel/live 继续延期，不开始下一 milestone。

```text
CANDIDATE_IDEMPOTENCY_SCOPE = CURRENT_SINGLE_PROCESS_APPLICATION_WRITER_ONLY
GLOBAL_SHOT_RESULT_UNIQUE_CONSTRAINT = NONE
GLOBAL_CROSS_PROCESS_CANDIDATE_UNIQUENESS = NOT_CLAIMED
CANDIDATE_READ_LIST_STATUS_ENDPOINT = NONE
CANDIDATE_RELOAD_STATE = CANDIDATE_RELOADED_UNVERIFIED
R20_RELOAD_STATE_PROMOTION_BY_CANDIDATE = NONE
CURRENT_VIDEO_BLOB_USED_FOR_CANDIDATE = NO
SELECTION_UI = NONE
SEQUENCE_UI = NONE
REORDER_UI = NONE
COMPOUND_COMMIT_TO_SEQUENCE = NONE
EXPORT_UI = NONE
ACTUAL_BROWSER_DOM_LAYOUT_ACCEPTANCE = NOT_CLAIMED
ACTUAL_BROWSER_INDEXEDDB_DURABILITY = NOT_CLAIMED
```
