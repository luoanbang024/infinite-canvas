# EXTENSION_STORE_SCHEMA

SCHEMA_VERSION = 1
STORE = <ProjectRoot>/metadata/hn-extension.sqlite
OWNERSHIP = OUR_EXTENSION
UPSTREAM_DB_SCHEMA_CHANGE = NONE

使用 SQLite database/sql 与现有 pure-Go 驱动，单连接，foreign_keys=ON、busy_timeout=5000、synchronous=FULL。用户版号 PRAGMA user_version；0 + empty store 在事务内创建 v1/项目身份，0 + nonempty 拒绝，1 验证/重开，不支持的未来/其他版本拒绝，不自动升级或修改用户生产资料。项目身份来自 store 中单一 project.id，不匹配 caller projectId 即拒绝。

| 表 | 关系索引/约束 |
|---|---|
| project | stable project id；一个 store 对应一个 project |
| reference_versions | id PK、project_id FK、data JSON（完整 ReferenceVersion facts） |
| shots | id PK、project_id FK、data JSON（含显式 selectedCandidateId） |
| generations | id PK、project_id FK、shot_id nullable FK、data JSON（请求/冻结/status） |
| generation_references | generation_id FK ON DELETE CASCADE、reference_id FK；复合 PK，角色/hash 保留在 Generation JSON |
| task_bindings | id PK、project_id FK、generation_id FK，UNIQUE(id,generation_id)，data JSON |
| results | id PK、project_id FK、generation_id FK；optional task_binding_id 与 generation_id composite FK，UNIQUE(id,generation_id)，data JSON |
| archive_jobs | id PK、project_id FK；result_id UNIQUE，与 generation_id composite FK；target_relative_path NOCASE UNIQUE；data JSON |
| candidates | id PK、project_id/shot_id/generation_id FK；result_id+generation_id composite FK；data JSON |
| sequence_items | id PK、project_id/shot_id/candidate_id/result_id FK、sequence_id、order_index>=0、data JSON |

所有完整领域字段序列化在 data JSON，关键引用单独列受 SQL FK 约束。Candidate 的 Shot/Result/Generation 一致、Shot selection、Sequence 组合关系由 typed API 校验；不是每项 JSON 关系都以 SQL FK 表达。排序在事务内重排并同步 JSON 与 order_index。外部绕过 API 直接编辑 SQL 不受支持。

Identity: id（各类型自己的 stable ID）、projectId、schemaVersion、createdAt、updatedAt；请求身份只引用 opaque credentialRef，从不存实际 token/key。ReferenceVersion 含 logicalReferenceId/relativePath/byteLength/sha256/mimeType/kind/sourceDescription?/transformSummary?。Generation 含 nodeId?/shotId?/promptSnapshot/protocol?/providerIdentity?/model?/parameters/referenceBindings[]/connectionId?/credentialRef?/requestSnapshotRef+hash?/sourceBaseline/status/submissionState/frozen/frozenHash。

TaskBinding 含 generationId/connectionId?/upstreamLocalTaskId?/providerTaskId?/protocol?/providerIdentity?/bindingState/lastPolledAt?/errorClass?。Result 含 generationId/taskBindingId?/providerResultId?/resultKind/sourceUrlRef?/receivedAt/archiveJobId?/archivedRelativePath?/sha256?/byteLength/status/duration?（仅真实已知）。ArchiveJob 含 resultId/generationId/targetRelativePath/status/attemptCount/lastError?/startedAt?/completedAt?/expectedMime?/actualSha256?/actualBytes。

Shot 含 label/sourceNodeId?/selectedCandidateId?；Candidate 含 shotId/generationId/resultId/label?/availabilityStatus；SequenceItem 含 sequenceId/orderIndex/shotId/candidateId/resultId。各自 stable ID 独立于 Provider IDs、URL、顺序位置。

删除遵循 FK/已选定/frozen-history 保护；文件不自动删除。没有 upstream DB 连接、Auth/User FK 或 schema migration。schema 原始 SQL 见 hn/foundation/store.go，字段 Go/JSON tags 见 types.go。测试通过 fresh/reopen/future gate/cross-project/CRUD/constraint/integrity；不交付任何测试 DB。
