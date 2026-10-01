# HN P0-B R3 本地图片显式快照接线（待 GPT Review / Audit）

EXECUTION_ID = HN_AI_IC_P0_B_R3_LOCAL_REFERENCE_SNAPSHOT_WIRING
TYPE = OUR_INTEGRATION_PATCH
STATE = PENDING_REVIEW / NOT_ADOPTED
BASE_OUR_COMMIT = b8fe4fb164fa45caf1fffd3141c703988df94c79
INTRODUCED_IN_COMMIT = 1fc693be121f1a297a3097ad785dd2a3f26f7f80

## 实际接线

复用原 Canvas 上传/插入、image_files localforage store 和未修改的 getImageBlob。仅已有 image: storageKey 的图片节点显示“冻结为生成参考”，不另建 picker，不复制浏览器存储。按钮回调只读取 Blob 并提交，不 setNodes、不改原 ID/storageKey/content；失败仍保留图片。in-flight UI guard 仅阻止同一页面重入，完成后再次显式点击仍创建新版本。

logicalReferenceId = ref- + SHA-256(UTF-8 JSON.stringify([projectId, storageKey]))。项目与来源身份分离，正文改变不改变逻辑来源，filename/path 不参与 identity。Canvas 项目 ID 原样传递，须满足 foundation 安全名契约；未作隐藏映射。原 nanoid ID 可能以 _/- 开头；实际 smoke 第一次创建的 aRqdAXa5B_3dayfodbC5x 合法。尚未遇实际不兼容的已有项目；若遇到，停止该项目接线并报告 PROJECT_ID_MAPPING_REQUIRED，不能据本轮 PASS 宣称所有 Canvas ID 都兼容。

## 配置与写入边界

后端从当前进程读取 HN_PROJECTS_ROOT，值为项目目录的父目录；未设置返回 503 和非秘密禁用消息，其他原应用功能保持可用。源码不含本机 E 盘路径。运行 smoke 仅对子进程设置指定父目录，未改系统环境或 Auth 配置。

POST /api/hn/projects/:projectId/references：一份 file、一份 logicalReferenceId、一份 kind=image；mimeType 提示不信任，按实际字节签名识别 PNG/JPEG/GIF/WebP/BMP/ICO。仅验证 MIME 签名，未做解码、转码或安全内容审查。原本地上传路径没有统一限额，本 adapter 保守限制文件 16 MiB、整个 multipart 请求 16 MiB + 64 KiB，ParseMultipartForm 内存阈值 1 MiB 并清理临时 multipart 文件。校验空文件、多个文件、kind、项目/来源 ID、原 Content-Disposition filename 的 traversal/路径概念。文件名不入持久 metadata，也不选择最终路径。

service 对 Open -> Snapshot -> Close 整个操作持进程 mutex，避免不同 handle 并发恢复/写同项目。Snapshot 使用可回读、已知长度的 multipart reader，字节未变换，foundation 产生 UUID、relativePath、SHA-256、sidecar 和独立 ExtensionStore row。返回标准 {code,data,msg} envelope，data 为 ReferenceVersion metadata，无绝对路径。底层路径/driver 错误转换为固定非秘密错误。

## 本机 HTTP transport

后端实际 RemoteAddr 必须是 loopback，Host 必须是 loopback，POST 必须有 X-HN-Local-Request: 1。忽略所有 forwarded 头。Origin 如存在，仅允许显式 http/https loopback 来源，不允许公网、null、user-info、query/path/fragment 来源。因本机前端/后端端口不同，允许不同本机端口和 localhost/loopback IP；这是本机信任边界，不是 Auth/JWT 或秘密鉴权。OPTIONS 同样检查实际 loopback 和 Origin，仅提供 POST/custom header CORS，不允许 credentials。

原 Next 通用代理会将外部请求转成后端 loopback；其 x-forwarded-for 在已装 Next base-server 中为 ??=，不能当可信来源。因而增加只读 GET /api/hn/local-endpoint，验证原 API_BASE_URL 为无凭据的 loopback HTTP 根地址，且通用代理对 hn namespace 固定 403。前端经此只读发现后直接连接 loopback 后端，credentials=omit，不发送 Auth/cookie；不允许远端 backend URL。未修改其他 API 代理语义或 Provider。

## 验证

4 个 HN 后端主测试和 15 个拒绝/禁用子例通过；含 IPv4/IPv6、Origin、伪造 forwarded、尺寸与文件名、hash/sidecar/SQLite reopen、同/异字节版本语义、6 次并发显式冻结。24 前端 tests（原 18 + 新 6）通过。go mod verify、root Go、独立 Bridge、独立 tsc --noEmit --incremental false、production build 全部通过。build 自身跳过类型检查，因此独立 tsc 必须单独记录，不能用 build 替代。

缺少该 Canvas UI 的现成组件测试 harness；adapter tests 加实际浏览器 smoke 验证 UI。全新隔离 localhost:3013 origin 原画布库为空，使用未登录状态和合成 PNG。通过原上传素材入口选择/插入、预览、显式冻结两次；文件 351 bytes，SHA-256=02d178a157ed58b272bc4dec47ec16c371a76d009df07e9a864cb55ec0e2f18b。后端和前端进程均重启，原节点 ID/storageKey 保持，两个预览 img 的 complete=true/naturalWidth=160/naturalHeight=100；第三次显式冻结成功，旧两份 metadata/文件 hash 仍完整，新旧三版相同 logical ID/字节/hash、不同 version ID。SQLite integrity_check=ok，其他 runtime entity counts=0。

浏览器首次标签页归属错误经实际 tab inventory 解决；选图工具等待明显偏长。一次 offscreen toolbar locator 误触“反推提示词”，仅产生隔离画布本地文本/配置节点，立即撤销、移回完整可见工具栏后验证；实际服务 request 日志无 Provider endpoint call。现有边缘节点工具栏可能超出视口的布局未在本轮重构。原图、metadata 与所有最终证据均来自合成素材。未修改生产项目。测试服务已按已核实的 PID/executable 停止，合成工作区保留供审查，不随 ZIP 打包 DB。

## 范围与后续约束

hn/foundation 7 个文件的 raw bytes 与审计基线一致；原 image-storage、Provider、Auth、upstream DB schema、Canvas/Node 数据模型、依赖与锁文件不变。无 Generation/TaskBinding/Result/Archive runtime，未配置 Key，无真实/免费/付费生成、无 external push/PR/tag/Release、无 merge。保持一个受控后端 writer；不同独立进程对同项目并发写入仍不在 foundation 支持范围。Generation SourceBaseline 复核与 paid idempotency 保留旧审计约束，本轮不开展。

## 退役与回滚

审查并正式采用的上游若包含等效显式 immutable reference/local guard 接线，再复核原来源 ID、项目 ID、持久数据映射后替换或移除这份 OUR integration；禁止自动删除旧 HN workspaces。回滚本 R3 implementation/governance commits，或停止配置 HN_PROJECTS_ROOT；保留所有已冻结文件/store，无 schema migration。下一步仅等待 GPT Review / Audit，后续 Archive 候选需要另行授权。
