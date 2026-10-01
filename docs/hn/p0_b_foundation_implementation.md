# P0_B_FOUNDATION_IMPLEMENTATION

EXECUTION_ID = HN_AI_IC_P0_B_R2_PROVIDER_NEUTRAL_LOCAL_FOUNDATION
BOUND_BASELINE = 852fd2128770136d037f92dfef2655dd3d6ac1d5
LOCATION = hn/foundation
PROVENANCE = OUR_EXTENSION / SOURCE_COMMIT=None
AUDIT_STATE = GPT_AUDIT_PASS
GPT_AUDIT = PASS
R2_FEATURE_HEAD = 35b22aa65fc3b90b3134a7ec20057e6b1cc955ce
R2_COMPLETION_SHA256 = f7ead792f34ce9335d2dd93e2b3048cf5a62930a772cb6558a896a17f2d78c3e
R2_IMPLEMENTATION_STATE = INTEGRATED_LIBRARY_ONLY
PROVIDER_WIRING = NONE
UI_WIRING = NONE
REAL_PROVIDER_CALLS = NONE
PAID_CALLS = NONE
CLOSEOUT_EXECUTION = HN_AI_IC_P0_B_R2_AUDIT_CLOSEOUT_INTEGRATION

独立 Go library，未接入应用启动、HTTP router、UI、Provider 或 upstream DB。复用已锁定的 github.com/glebarez/go-sqlite 和 github.com/google/uuid，不新增版本/lock。单一 package 内按类型、workspace、store、records、file/archive、export 分文件，不引入新框架。

| 文件 | 责任 |
|---|---|
| types.go | 8 个领域对象、稳定 Identity、引用绑定、JSON/CSV manifest 类型 |
| workspace.go | 明确 projectId / UUID、五目录、canonical relative path 与 Windows junction 验证、Open/Close/版本/integrity |
| store.go | 独立 SQLite v1 创建/版本 gate，关系 FK 和 typed write 的底层 SQL，Read/List/Delete |
| records.go | Generation draft/update/freeze、SUBMISSION_UNKNOWN、TaskBinding facts、Result ownership、显式候选选择/序列重排 |
| files.go | explicit source snapshot、hash/bytes/sidecar、temp+Sync+同卷 rename、ArchiveJob retry/reopen reconciliation |
| export.go | 选定的已校验本地档案按序复制、JSON/CSV manifest |
| foundation_test.go | 本地合成 fixtures、Windows junction、schema、失败/恢复、真实子进程 reopen、导出结构测试 |

## 调用边界

`Open(projectsRoot, projectId)` 只管理显式命名项目；空 projectId 生成 UUID。正常生产 projectsRoot 为固定 Windows workspace/projects；测试仅使用工作区内 TEMP 下的合成项目。不扫描用户目录、不打开 upstream application DB。不需要账号或 Key。

`SnapshotFile` 只读明确供应的 regular file；`Snapshot` 供应明确 reader/expectedBytes。每次 snapshot 新 ID，即使内容相同也不合并身份。文件/sidecar 不覆盖；SHA-256 从最终字节计算。原始文件变化不影响快照；API 没有 ReferenceVersion update 操作。操作系统拥有者仍可外部篡改文件，verify/reopen 会拒绝字节或 sidecar 不一致，不能宣称 OS 层禁止一切修改。

`CreateGeneration/UpdateGeneration` 只允许新身份/未冻结请求调整。`FreezeGeneration` 先验证 refs/optional metadata request snapshot，并持久化 frozenHash/PREPARED。冻结之后 request-defining facts 不可改写；`MarkSubmissionUnknown` 只记录未知状态，没有真实 submit/resubmit 方法。协议/Provider/model 可为空。

`CreateTaskBinding/UpdateTaskBinding` 仅记录 caller 明确已知的 task facts，空 taskId 保持 UNBOUND；既有 task ID 与归属不可替换。`CreateResult` 的 Gen/TaskBinding FK 约束归属，sourceUrlRef 只能是 opaque non-secret ID，不能放原始/签名 URL。

`CreateArchive/RunArchive` ingest reader，不内置 HTTP client。copy 长度/hash/Sync 验证后先写 FINALIZING，再同卷 rename、sidecar、SQLite事务完成 job/result/candidate 可用性。Retry 只重试 copy，已验证最终文件直接复用，不新增 Generation。reopen 可补全 rename 后 metadata 未完成的档案；文件/sidecar坏或缺失标 INCONSISTENT，不能保持虚假 ARCHIVED。失败临时文件清理；模拟进程终止可能留下不引用的临时/孤儿文件，不扫描/删除它们。没有自动 garbage collection。

`CreateCandidate` 不自动选中；`SelectCandidate` 是唯一显式选择入口。旧结果仍指向原 Generation，不改当前选择。SequenceItem 的 stable ID 与顺序独立，可重复引用同一选定结果。`Reorder` 必须覆盖该序列全部 ID，无重复/跨序列。Export 要求当前显式选择仍匹配 SequenceItem 且归档字节/元数据校验通过，否则拒绝。

## Store CRUD 与保护

Read/List 支持所有 8 类 JSON records。写入只通过 typed invariant-preserving API；ReferenceVersion 不可 update；Generation freeze 后不允许任意 update/delete；Result 的 archive fields 由归档事务更新；Shot/Candidate label、selection，TaskBinding append-known facts 和 Sequence reorder 为受控更新。Delete 只删除可解除关系的 metadata：selected/被引用记录与 frozen Generation 受保护；PENDING ArchiveJob 可删除并清 Result binding，完成/尝试过的归档历史不可删除。所有文件保留，避免隐式删除用户素材。

## 验证及实际限制

12 个主测试 PASS（另一个 helper 在 standalone runner 中 Skip，但子进程测试明确调用并完成它）；覆盖率 76.6%。SQLite integrity_check=ok；真实子进程重开、同进程断点模拟和损坏/缺失档案均已验证。Windows junction 的首个专项测试捕获旧 EvalSymlinks 校验缺口，现用 Go 1.25 os.Root.Stat/parent guard 校验，最终测试通过。

审计保留事项：SourceBaseline 常量仍绑定原 adopted baseline 852fd2128770136d037f92dfef2655dd3d6ac1d5；未来正式 baseline/wiring 在创建新 Generations 前须复核绑定，不通过本 governance closeout 改实现。后续真实 submit wiring 仍须落实每 generationId 最多主动提交一次，不能把 TaskBinding primitives 当成已实现 paid idempotency gate。

本 package 按一个受控 writer owning workspace 使用；同一 Workspace 的写操作 mutex 串行。尚未支持多独立进程同时打开/写同一项目，也不宣称可抵御敌对进程在验证与 path-based IO 之间替换目录。后续接线必须保持单 writer、受控目录权限。SQLite 事务与文件 rename 是两个持久化步骤，reopen 对账覆盖该窗口；没有承诺跨 store 事务或 Windows 断电情况下全局原子持久性。atomic-finalization PASS 指测试中完整 temp→final 发布与 metadata 对账协议。

合成 media bytes 仅验证字节/哈希/顺序，不是编码或剪映兼容性实测。未启动编辑器、未写其用户目录。未选择 Provider/账号模式、无网络生成/免费/付费调用，未读取或配置 Key。文档化非秘密校验拒绝常见秘密字段/URL；调用者仍必须供应 non-secret text，校验不是通用 DLP。
