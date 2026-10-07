# 无限画布文档索引

## 项目介绍

- [快速开始](overview/quick-start.md)
- [功能介绍](overview/features.md)
- [Docker 部署](overview/docker.md)
- [第三方 GitHub 提示词仓库](overview/third-party-prompt-repositories.md)

## 操作手册

- [画布节点操作手册](canvas/canvas-node-manual.md)
- [画布快捷键](canvas/canvas-shortcuts.md)

## 开发文档

- [本地开发](backend/local-development.md)
- [接口响应约定](backend/api-response.md)
- [系统配置数据结构](backend/system-settings.md)
- [后端数据库说明](backend/backend-database.md)
- [画布数据结构](backend/canvas-data-structure.md)

## 商务合作

- [开源协议](business/license.md)
- [商务合作](business/business.md)

## 赞助支持

- [打赏支持](support/donate.md)

## 项目进度

- [待测试](progress/pending-test.md)
- [TODO](progress/todo.md)

## 说明

- 未登录时画布项目和“我的素材”保存在浏览器本地；登录且账号同步可用时，会同步保存到账号/云端。
- 本地直连模式下，AI API Key 保存在浏览器本地，并由前端直接请求 OpenAI 兼容接口。

## HN P0-B R24

- [Canvas 本地候选显式选择（GPT Audit PASS / AUDITED_INTEGRATED）](hn/canvas_local_selection_workflow.md)：strict historical Candidate → mutable Selection → advisory receipt；同 Shot 六方向互锁及 UNKNOWN 停止边界。

## HN P0-B R26

- [Canvas 本地主序列显式加入（GPT Audit PASS / 已集成）](hn/canvas_local_sequence_placement_workflow.md)：strict historical Candidate/Selection → non-idempotent append → durable browser ledger / advisory receipt；main固定、UNKNOWN停止。

## HN P0-B R28

- [Canvas 本地主序列命令与 exact-key 恢复（GPT Audit PASS / AUDITED_INTEGRATED）](hn/canvas_local_sequence_recovery_workflow.md)：same-intent immutable receipt、SQLite cross-process append、readonly main snapshot；v1 UNKNOWN 不迁移。

- R28 原实现与 local-boundary OUR_VERIFICATION_FIX 已一起通过独立 GPT Audit；五个 reviewed commits 原样正常推送、ff-only 集成，六文件治理收口。boundary 先于 sequence validation；等待 Closeout Completion GPT Review，真实 browser 验收 NOT_CLAIMED，未启动 R29。

## HN P0-B R30

- [本地主序列 revision/CAS/command 协议（GPT Audit PASS / AUDITED_INTEGRATED / NO_UI）](hn/sequence_reorder_protocol.md)：upgraded main writers 同事务 revision、immutable historical receipt、readonly full snapshot；无 Canvas Reorder 接线。

- R30 三个 reviewed commits 原样正常推送、ff-only 集成；六文件一个治理收口 commit，production/test 和 EOF 字节保留；等待 Closeout Completion GPT Review。无 R31/Reorder UI/Provider。
