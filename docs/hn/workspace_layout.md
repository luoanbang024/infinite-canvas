# WORKSPACE_LAYOUT

PROJECTS_ROOT = E:\GPT_Work\HN_AI_Infinite_Canvas\workspace\projects
PROJECT_ROOT = <PROJECTS_ROOT>/<projectId>

```text
<ProjectRoot>/
├── assets/
├── references/<referenceVersionId>/reference.bin[.json]
├── generated/<generationId>/<resultId>/media.<ext>[.json]
├── exports/<exportId>/media/001_<resultId>.<ext>
│                    ordered-manifest.json
│                    ordered-manifest.csv
└── metadata/hn-extension.sqlite
             request.json  # optional explicitly supplied frozen snapshot
```

projectId 安全 ASCII stable ID，空值 UUID；拒绝 traversal/Windows device/drive/UNC/ADS。持久路径为 canonical forward-slash project-relative；Resolve 校验各现有 parent 与 Go os.Root guard，拒绝 escaping junction，包括项目根 junction。只创建显式项目的五目录，不扫描用户盘。

输入源文件路径仅供 SnapshotFile 明确读取，不持久化外部 absolute path。Snapshot Reader 要求 expected byteLength；临时文件和最终文件在同目录/卷，以最终内容计算 SHA-256/byteLength，Sync 完成后 rename，不覆盖已存在 final/sidecar。Snapshot 原文件后来变化只产生新版本。

RunArchive 的 source reader 不内置网络；测试全部本地合成 bytes，不访问 Provider。生成树路径必须归属于原 generationId；target unique（NOCASE），防止两个结果复用同一 final identity。ARCHIVED 仅当最终文件/sidecar/metadata 同意。已有已验证 bytes 复用；失败或重开不触发 Generation。

R2 生产函数尚未接线应用，因此没有创建/打开真实用户业务项目。测试目录固定为 E:\GPT_Work\HN_AI_Infinite_Canvas\temp\r2-tests；testing.TempDir 下数据自动清理。无 DB/媒体/临时二进制交付。单 writer/受控目录是当前调用约束；不支持敌对目录替换或并发多进程 writer。备份仍需停写整个项目并另行协调上游/浏览器状态，不宣称复制单一 DB 是全应用备份。
