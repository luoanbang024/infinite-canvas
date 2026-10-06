# Canvas 本地主序列命令与恢复

EXECUTION_ID = HN_AI_IC_P0_B_R28_NO_CREDENTIAL_SEQUENCE_AUTHORITATIVE_RECOVERY_PROTOCOL
TYPE = OUR_EXTENSION / LOCAL_ONLY / AUTHORITATIVE_RECOVERY_PROTOCOL
STATE = PENDING_GPT_AUDIT
BASE_SHA = fa73edd13b9a4e3d7d24ed503c80d1c9c0c5e49a

## 一次命令的身份

采用 R27 已审核方案 ATOMIC_DURABLE_PLACEMENT_COMMAND_RECEIPT_PLUS_READ_ONLY_MAIN_SNAPSHOT。main 固定；新的 UUIDv4 placementIntentId 与 protocolVersion=1、exact Candidate/Shot/Generation/Result/ArchiveJob/frozenHash 构成 closed command。同一 project/main/version/intent + 相同 canonical payload 只产生一个 item，返回不可变的历史 COMMITTED/REJECTED；同 intent 改 payload 冲突。不同 intent 是新的明确 placement，不按 Candidate 去重。本版 UI 保留已知成功同 Candidate 禁止重复加入，不提供随意 repeat。

旧 POST items 仍只接收 candidateId，显式两次可产生两个 IDs；改为 SQLite pinned BEGIN IMMEDIATE，使 current Selection、max+1、insert 在同一写事务中。新 command 的 owner 校验、extension 初始化、item 与 terminal receipt 也在同一事务中；HTTP success 只在 COMMIT 后。已提交 A 命令在改选 B 后 replay 仍返回原 A 回执，不重选 A、不新增 item。

## 独立扩展与严格读取

独立 placement_protocol / placement_commands，加不可更新/不可删除 trigger；scoped PK、canonical bytes+SHA、terminal outcome/nullability CHECK、unique item、restrictive item FK。只由新 POST 原子安装并严格验证形状/版本；部分或未知 schema fail closed。不修改 base user_version=1、Identity、SequenceItem 或历史 JSON、不做 destructive rollback/TTL。

专用 OpenExisting 使用 existing-only mode=ro/query_only、原有 Resolve 路径保护、project/base version 校验。新 GET 不走普通 Open，不 mkdir/init/migrate/reconcile/media read。main items GET 在同一 read transaction 读 count/rows，metadata 完整、orderIndex/ID 排序、最多256；超过返回 READ_CAPACITY_EXCEEDED，非 append cap。重复 order 或 JSON/index disagreement 为 integrity conflict。

exact-key command GET 区分 COMMITTED、REJECTED、NOT_OBSERVED、protocol 未初始化、integrity conflict。NOT_OBSERVED 仅表示这个快照未观察到 key，不能证明旧请求不会稍后提交。快照只展示读取时的 metadata，不用于按 Candidate/order/time 猜测归因。

## Canvas v1/v2 与显式恢复

复用 archive Dialog 的本地序列区。v1 browser-only UUID 不迁移、不发送、UNKNOWN 不删除；其屏障和已知成功保护继续有效。v2 独立 localforage store hn_local_sequence_placement_commands_v2，key=[v2,hnProjectId,main]。page main→Shot/node 同步取锁，strict historical owner/Selection，明确确认，新 intent + payload，PLACING setItem/read-back 完整一致后才一次 POST。ack 需 durable terminal seal/read-back 后才 projection。

response loss、opaque/invalid、timeout、terminal-seal ambiguity → UNKNOWN；auto POST retry=0。新增“检查服务器结果”只 exact-key GET；只有严格匹配的 COMMITTED/REJECTED 被 durably sealed 才解除该 v2 UNKNOWN。NOT_OBSERVED、网络/存储/完整性错误继续 fail closed。较早的 durable PLACING/UNKNOWN 与 page 内相同 K/P 可在显式 recovery 重新封存 pending，但不得丢掉其他 cached command 的 unknown/duplicate protection。

“继续同一次加入操作”是新的显式 mutation，确认显示：“若此前未提交，此操作可能现在新增一个剪辑项；若已经提交，只返回原回执，不会再次加入。”严格复用同一个 intent/payload，不生成新 intent；一个明确确认最多一次 POST；再次丢响应仍 UNKNOWN。丢失 journal 但保留 strict v2 Canvas K/P 时先重建 UNKNOWN，再 exact lookup；丢失 K 不按 Candidate/snapshot 合成恢复。lookup 可绕过 mutation barrier 做安全读取，仍锁住本页相关操作；continuation 复用 main/Shot/node mutation locks，不能绕过 v1 UNKNOWN。

服务器 command 回执只证明该 exact 命令已提交及 original item/append position，不证明当前 Selection、未来 reorder 后的顺序、当前媒体完整性或 AI/Provider 成功。reload 仍 PLACEMENT_RELOADED_UNVERIFIED；恢复不提升 R20/R22/R24。LOCAL_RESULT_ARCHIVE_DOES_NOT_ASSERT_PROVIDER_SUCCESS。

## 验证与限制

完整 root Go、mod verify、独立 Bridge、focused preservation、全部 frontend、独立 tsc、隔离无 env production build，以及真实两进程 rendezvous tests 均通过，日志/源码/hash/差异/计数见 Completion。两进程同 K/P 一个 item；不同 intents 两个 unique positions；升级 legacy+new writer 并发 unique positions；first-use schema race 收敛。真实 Go HTTP 提交后断响应 + Bun production client exact GET/continuation 收敛，零自动重发。R7/R8 独立 OUR_VERIFICATION_FIX、R20/R22/R24 controller raw bytes、正常 Generate/Upload/Provider 与 SourceBaseline 不变。

保证范围仅为所有升级 writer / 同一个本地 SQLite 文件 / 无 concurrent legacy reorder。mixed old binaries、未经授权直接 DB writes、跨主机 FS 故障、永久丢失全部 intent、不可靠浏览器 fsync/全局 journal CAS、真实 DOM/IndexedDB/theme 人工验收均不声明。rollback 只停用 v2，保留扩展/receipts/items；回滚到旧 writer 将失去本轮并发保证。没有 automatic resend、Sequence CAS/revision、Reorder/compound/Export/Jianying 工作。

API Key/account/balance/entitlement/channel secret/live 全部继续 DEFERRED，input/detection/validation SKIPPED，preauth PAUSED_BY_USER_DECISION，live authorization NOT_GRANTED。Provider/real/paid calls=0；真实 credential/env/remote media read=NONE；无依赖安装/升级、merge/push。下一步仅独立 GPT Audit。
