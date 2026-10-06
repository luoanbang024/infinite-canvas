# Canvas 本地主序列显式加入

EXECUTION_ID = HN_AI_IC_P0_B_R26_NO_CREDENTIAL_CANVAS_LOCAL_SEQUENCE_PLACEMENT
TYPE = OUR_EXTENSION / PRODUCT_WIRING / LOCAL_ONLY
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS

## 入口与事实边界

既有“归档本地视频”Dialog 中的独立“本地序列”区块，只支持 main。strict prepared owner → effective historical ARCHIVED journal → strict Candidate → strict Selection receipt → 明确确认 → existing addSequenceItem。不隐式 ensure/Select；不读当前 Blob、上传、下载或重新归档。Selection reload 可提交，但它不是当前 server authority；后端 Add 在 INSERT 前校验实际 SelectedCandidateID。没有 Sequence/current-selection GET、CAS、revision 或恢复协议。

持续显示：“此操作会在主序列末尾新增一个剪辑项；不会生成视频，也不会导出，不代表 AI/Provider 生成成功。”与“本页没有可靠的主序列查询；回执只记录一次已确认的加入操作。”LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS。

## 非幂等 append 与可靠停止

现有 Add 同 Candidate repeat 创建不同 SequenceItemID，orderIndex 取当前 max+1；没有业务幂等键。browser UUID 仅关联，不发 backend。确认展示 main/Shot/Candidate/Result 及“每次成功调用都会新增一个剪辑项；结果未确认时不能重新发送。”每个明确 command 最多一次地址发现 GET 和一次 loopback POST，credentials omit、redirect error、30秒 timeout，无 retry/failover。request wrapper 只放行 exact route/body/headers，拒绝凭据头、其他路径及 foreign/extra/malformed response。

专用 localforage name=infinite-canvas、store=hn_local_sequence_placement_attempts；key JSON.stringify(["v1",hnProjectId,"main"])。闭合 aggregate ledger 同时保存 Candidate 历史与 Sequence barrier；最多256条是本版 browser budget，不是 server Add cap。先同步 Sequence→Shot/node 取锁、严格读历史、确认、发现地址，再 PLACING setItem→getItem→strict完整值/revision/intent/owner一致校验→POST。确认取消无 journal write/POST。known rejection 必须 seal/read-back REJECTED 才允许新的明确 command；只有 REJECTED 可替代。严格 ack 必须 seal/read-back PLACED 才写 Canvas receipt。

timeout/drop/redirect/5xx/未知409/invalid response 为 PLACEMENT_OUTCOME_UNKNOWN；内存先立 barrier，durable UNKNOWN best effort，保留 PLACING 亦保护 reload。final sealing 不可靠同样 UNKNOWN，不重发、不override。已知成功同 hnProject/main/Candidate 一律禁止 repeat（包括删投影、新Selection intent、reload、其他node）；新 Candidate 在明确新Selection后可独立加入，保留旧ledger。Sequence 有未知项则所有 Shot 的新 placement 都禁止。

## Admission、互锁与 reload

page-owned controller 固定 Sequence key [hnProjectId,main]，复用 R24 Shot [hnProjectId,shotId] + node [canvasProjectId,nodeId]。first await 前同步 nonblocking Sequence first→Shot second；无排队，无FIFO/跨process承诺。保留原六方向，增加 Placement↔Archive/Candidate/Selection 六方向。optional synchronous coordinator admission gate 只查validatedcache；hydrate前或unreadable/malformed ledger时整project fail closed。UNKNOWN阻止原Shot的三个操作，包括原R24同target reselect；其他Shot的三个动作在有效barrier下可继续。Dialog关闭、target变化不能释放token/清barrier；epoch防stale merge/release。

PLACING/UNKNOWN reload → UNKNOWN；PLACED reload → PLACEMENT_RELOADED_UNVERIFIED，不查询、不append复核。Canvas receipt无matchingledger为ERROR。receipt闭合17字段，仅exact owner+main+itemID+创建时orderIndex+两个browserintent+observedAt；不是当前完整Sequence或当前顺序。ack后owner变了可封存captured历史success，不merge新owner。R20/R22/R24 reload不会被提升。inspect/mount本地只读，业务HTTP/journalwrite=0。依赖保留browserjournal的单page边界，不保证跨tab原子性、browserfsync或完整storage删除后的恢复。

## 验证、保护与后续

root Go、独立 Bridge Go、go mod verify、全部frontend、独立tsc、env-free isolated production build、Foundation及R5/R6/R7/R8/R10/R12/R13/R14/R18/R20/R22/R24/R26 focused preservation均通过；实际production handler/localhost响应丢失证明POST=1/item=1、reload追加POST=0，redirect destination=0。完整源文件、diff、raw hashes、日志、计数和最终member-exact secret scan在Completion。真实DOM/IndexedDB/主题/人工流程验收NOT_CLAIMED，见pending-test。

R24 controller/request冻结raw segments不变；R20/R22 controllers、R7/backend/router/Foundation/Auth/schema/dependency/lockfile字节保留。R7/R8独立OUR_VERIFICATION_FIX、R9/R15 audited validations、R16 deferral不变。SourceBaseline仍16047f46e2186373ea824e12e84ae8dfa2ccde32，生产不新建Generation/Result/Archive/Candidate/Selection（测试隔离fixture除外），不改历史Generation。

Provider/real/paid calls=0，REAL_CREDENTIAL_READ/REAL_REMOTE_MEDIA_DOWNLOAD=NONE。API_KEY_SETUP/ACCOUNT_CREDENTIAL_CHECK/ACCOUNT_BALANCE_CHECK/ACCOUNT_ENTITLEMENT_CHECK/CHANNEL_SECRET_SETUP/LIVE_PROVIDER_VALIDATION=DEFERRED；API_KEY_INPUT/DETECTION/VALIDATION=SKIPPED；PREAUTH=PAUSED_BY_USER_DECISION；LIVE_AUTHORIZATION=NOT_GRANTED。不Reorder/compound/Export/Jianying。本次授权normal non-force feature/main push、ff-only集成与治理收口；最终stable SHA及focused checks见Closeout Completion，等待Closeout GPT Review。

### R26 GPT Audit closeout binding

GPT_AUDIT = PASS
STATE = AUDITED_INTEGRATED
AUDITED_FEATURE_HEAD = 7718b379cf11bb2ac5ef4b5685d186230335bc99
AUDITED_COMPLETION_SHA256 = fda786657c19c292447f6fc08f1b85f06f6b313f46e4b65681123b81145b12bf
AUDITED_COMPLETION_COUNTS = 122 ZIP members / 121 payloads / CRC PASS / exact final secret inventory / empty allowlist
REVIEWED_COMMITS = 078ec1629fbd2bf2550240049881a6975d4395f0, 2a4f3937392a8458ce85b7ac5ec1ec2f34997a17, 7718b379cf11bb2ac5ef4b5685d186230335bc99
INTEGRATION_METHOD = FAST_FORWARD_ONLY
PUSH_METHOD = NORMAL_NON_FORCE
REVIEWED_COMMIT_REWRITE = NONE
CLOSEOUT_STABLE_SHA = recorded in Closeout Completion (no self-reference)
CLOSEOUT_REVIEW_STATUS = READY_FOR_R26_AUDIT_CLOSEOUT_GPT_REVIEW after focused PASS and final origin identity

当前main只集成已审计R26并新增一个治理commit。为保持原Completion混合LF/CRLF raw bytes，验证exactancestor与expectedoldref后使用等价exactfast-forward ref推进，再切换同一HEAD；无mergecommit、源码编辑或配置修改。原12个production/test文件rawSHA与Completion完全一致，Gitcanonicalblob身份亦保留；所有非治理文件不变。

non-idempotent main append / no authoritative current Sequence read / single-page retained browser journal / UNKNOWN fail closed / no automatic retry / advisory reload UNVERIFIED / twelve interlocks 继续保持。真实browser DOM/IndexedDB/theme acceptance NOT_CLAIMED。SourceBaseline16047f46e2186373ea824e12e84ae8dfa2ccde32不变，R7/R8独立fix与R9/R15validation/R16 policy不改写。API Key/account/balance/entitlement/channel/live继续DEFERRED，input/detection/validation SKIPPED，preauthPAUSED_BY_USER_DECISION，liveauthorizationNOT_GRANTED；Provider/real/paid calls=0、realcredential/media读取NONE。无Reorder/compound/Export/Jianying/GET/list/recovery/CAS/revision/R27。最终focused/byte/scope/push证据见Closeout Completion，不自行宣布Closeout GPT Review PASS。
