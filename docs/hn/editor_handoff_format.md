# EDITOR_HANDOFF_FORMAT

TARGET_EDITOR = 剪映专业版
IMPLEMENTATION = OFFLINE_STRUCTURE_ONLY
LIVE_EDITOR_IMPORT_TEST = DEFERRED
JIANying_USER_DATA_MODIFIED = NO

`Workspace.Export(sequenceId)` 只复制该序列的显式 selected、ARCHIVED 且 hash/byteLength/sidecar/metadata 一致的本地文件。排序 orderIndex，然后稳定 ID；序列内允许相同 Result 重复引用。导出新的 exportId，媒体名为 001_<resultId>.<原档案扩展名>，JSON/CSV manifest 都出现后才视为完整 bundle；JSON 最后写为完成 marker，失败的 partial export 可能保留文件但没有完成 JSON，不自动删除。

位置 exports/<exportId>/media/，同目录 ordered-manifest.json/csv。Manifest 包含 schemaVersion/projectId/exportId/sequenceId/items；每项 exportId/sequenceIndex（1 起）/sequenceItemId/shotId/candidateId/resultId/relativePath/sha256/byteLength/duration?。relativePath 明确相对项目根，不是绝对 Windows path；若单独复制 export directory，消费者须从 exports/<exportId>/ 前缀计算 bundle-relative media/...。CSV UTF-8、标准 encoding/csv quoting，header 同上述 item 字段；未知 duration 空格，JSON 省略，不推测。

复制前核对原档案，temp copy 后重新核对 bytes/hash，再发布 final；清单引用的顺序/ID/hash 冻结成导出快照，后续重排不改已导出清单。若某 Shot 当前 selected 已不同于 SequenceItem，Export 拒绝；不会按到达顺序或任意 Candidate 替代。

R2 示例清单由实际离线测试生成（synthetic-media-one/two 的合成字节）；不含 media 文件，不证明文件 codec 可播放。国际/国内剪映兼容性、本机导入/顺序/播放留待独立后续验证；没有原生 manifest importer 承诺，没有 XML/EDL，没有编辑器草稿目录写入。
