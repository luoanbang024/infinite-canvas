# UPSTREAM_WATCH

PRECHECK = PASS / NO_BLOCKING_OVERLAP
FETCHED_TIP = 6571143e4f51da7494d38572c76202b752cc5e0c
BOUND_SOURCE = edd4452cb9b0d93dbb9c1ea5acdea1ee14015800
ADOPTED_OUR_BASE = 852fd2128770136d037f92dfef2655dd3d6ac1d5

已实际 fetch origin 和 tiger-upstream，our-main/origin refs 精确匹配。检查绑定 v0.8.0 后上游两个 commit：f4e557ebf656cafb7f6a69d1d44959a5845b1b65（TokenDance/Agent/settings）、6571143e4f51da7494d38572c76202b752cc5e0c（model classification/Agent）。检查 log、54 changed paths、workspace/reference/archive/sequence/handoff tree 名称及 video.ts/direct-ai.ts diff。

窄重叠事实：video.ts 的 cacheProtectedVideo 把受保护 content URL 缓存范围泛化，是 Provider/media 辅助已有功能，并非项目 ExtensionStore、immutable reference、冻结 Generation、ArchiveJob final hash/reopen 对账、Shot/Candidate/Sequence 或离线 manifest 实现；direct-ai.ts 主要新增 TokenDance protocol/task recovery 字段。没有能替代本次大部分 OUR 层的等效完整实现。

因此不触发 UPSTREAM_OVERLAP_REVIEW_REQUIRED。没有采用/merge upstream main；集成分支 v0.8.0 不动，Provider 差异不 backport。此结论仅针对检查到的确切 tip 与相关范围，不宣称整个上游没有相似功能。精确 source diff 存 artifacts/non-secret-evidence/upstream-watch.diff；preflight.json 含 refs/log/hashes。


## R3 narrow wiring watch

R3_BASE = b8fe4fb164fa45caf1fffd3141c703988df94c79; FETCHED_TIP = 6571143e4f51da7494d38572c76202b752cc5e0c; NO_BLOCKING_OVERLAP.

本轮重新实际 fetch origin/tiger-upstream。精确检查原 image-storage、hover toolbar、handler/storage、router 与 Canvas page 的 v0.8.0→tip diff；前四处无差异，Canvas page 仅 Agent get_generation_config 的 audio/video channel 路由变更。没有当前范围等价的显式本地图片 -> immutable ReferenceVersion 接线，故不触发重叠停止门；未采用该 Agent/Provider hunk。新增路径属于 OUR integration，详见 local_reference_snapshot_wiring.md 与 R3 scoped evidence。


## R4 narrow Generation identity/freeze watch

R4_BASE = 418ffbde3dbea33d374356588cb672336ec38353; FETCHED_TIP = 6571143e4f51da7494d38572c76202b752cc5e0c; NO_BLOCKING_OVERLAP.

Fetched origin/tiger-upstream again; local/origin our-main match the supplied stable baseline. Checked post-v0.8.0 hn/handler/service/router, Canvas generation context/helper and generation service trees. Relevant upstream changes remain TokenDance/Agent/model-channel/provider work; no equivalent local HN identity + immutable ReferenceVersion binding + pre-submit frozen request endpoint. Existing upstream task/cache records do not establish this boundary. No upstream main adoption, Provider backport or protocol/poller changes. Scope and exact diff are in R4 completion; conclusion is limited to this tip/scope.
