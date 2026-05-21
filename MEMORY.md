# MEMORY

Last updated: 2026-05-21 13:12 Asia/Shanghai

## 当前阶段

- 项目处于 V0.5 CDC stub / Canal 前置完成阶段。
- 当前仓库已有 Go MVP 骨架、配置样例、迁移样例、RabbitMQ 核心接口、表列映射、MySQL Apply Worker 和无感安装计划模型。
- 当前尚未实现真实 Canal client、CDC offset 持久化、ACK 持久化、失败事件重放和完整 Wails 前端。

## 已完成事项

- 已绑定 Git remote：`https://github.com/YufeiSun5/NodeBridge.git`。
- 已创建根级 `AGENTS.md`，作为 Codex/Copilot/Cursor 的项目路引入口。
- 已创建 `.ai/` 母本文档目录，包括 instructions、docs、agents、prompts。
- 已创建 Copilot 与 Cursor 的薄适配层。
- 已将 `AGENTS.md` 和技能使用说明调整为中英日三语。
- 已初始化一期 MVP 骨架：Go module、vfox 工具链锁定、双命令入口、核心模型、配置样例、迁移样例和基础测试。
- 已补强一期测试门禁，并新增 RabbitMQ 拓扑、publisher、consumer、队列状态和 Windows 无感安装计划模型。
- 已新增表名和列名重映射能力：规则模型、mapper、映射样例和迁移追踪字段。
- 已实现 V0.2 MySQL Apply：连接池、迁移执行、`MappedEvent` 事务 Apply、幂等 `sync_apply_log`、CLI 子命令和 sample events。
- 已持久化版本路线、Wails 不占端口和日志 Web 服务要求。
- 已实现 V0.3 RabbitMQ CLI：拓扑初始化、事件发布、单条消费并成功 Apply 后 ACK。
- 已实现 V0.4 Runtime 初版：Edge 上传转发、Server ingress Apply、ACK/NACK 和防回源下发。
- 已实现 V0.4 Worker 与诊断骨架：长期循环、状态快照、只读日志 Web 入口。
- 已实现 `sync-agent run`：按 Edge/Server 模式启动当前 worker 组并挂接日志 Web 生命周期。
- 已实现 Edge downlink apply：Edge 从 Server RabbitMQ 的 `<node_id>.downlink.q` 消费下发事件，映射后写入本地 MySQL，成功后 ACK。
- 已新增 `consume-downlink-once`，用于开发阶段单步验证 Server -> Edge 下发 Apply。
- 已新增 `WorkerGroup`，Edge 模式可同时运行上传转发和下发 Apply 两个 worker。
- 已按约定补充短而有力的中英日三语注释，用于 ACK 与下发队列关键边界。
- 已完成 worker 级日志 ring buffer：日志 Web 暴露只读 `/logs`，仅输出脱敏后的 worker 运行摘要。
- 已完成 Server dispatch 计数：`server-ingress` 状态记录 dispatch 累计数量。
- 已修复 vfox Go 1.25.5 SDK 缺失问题，重新安装并设为当前项目版本。
- 已推送 V0.4 到 GitHub：`origin/main`。
- 已实现 V0.5 CDC stub：`ChangeEvent` JSON、`normalizer`、`cdc.StubSource`、`CDCUploadRuntime`。
- 已新增 `publish-change-once`，模拟 CDC 事件可经回环抑制和标准化后发布到 Edge 本地上传队列。
- 已新增 `sample-events/device_config.change.json` 和 `docs/v0.5-cdc-stub.md`。

## AI 工程化状态清单

- [x] 根级项目路引：`AGENTS.md`
- [x] AI 工作流规范：`.ai/instructions/ai-workflow.md`
- [x] Go SyncAgent 规范：`.ai/instructions/go-syncagent.md`
- [x] Wails 前端规范：`.ai/instructions/frontend-wails.md`
- [x] 同步架构约束：`.ai/instructions/sync-architecture.md`
- [x] 设计摘要文档：`.ai/docs/product-design.md`
- [x] 只读架构审查 Agent：`.ai/agents/architecture-review.agent.md`
- [x] 常用 Prompt 模板：`.ai/prompts/`
- [x] Copilot 适配入口：`.github/`
- [x] Cursor 适配入口：`.cursor/`
- [x] 一期 MVP 骨架：`cmd/`、`internal/`、`configs/`、`migrations/`
- [x] 一期补强测试：CLI、配置、规则、状态、回环抑制
- [x] 二期 RabbitMQ 基础：`internal/rabbitmq`
- [x] 表列映射基础：`internal/mapper`
- [x] V0.2 MySQL Apply：`internal/mysqlconn`、`internal/apply`
- [x] V0.3 RabbitMQ CLI：`init-rabbitmq`、`publish-event`、`consume-once`
- [x] V0.4 Runtime 初版：`internal/syncruntime`、`forward-upload-once`
- [x] V0.4 Worker/诊断：worker loop、`internal/status` runtime store、`internal/logweb`
- [x] V0.4 启动入口：`sync-agent run`
- [x] V0.4 Edge 下发：`EdgeDownlinkRuntime`、`consume-downlink-once`、Edge 双 worker
- [x] V0.4 日志与调度状态：`/logs`、ring buffer、dispatch count
- [x] V0.5 CDC stub：`internal/normalizer`、`cdc.StubSource`、`CDCUploadRuntime`
- [x] V0.5 CLI smoke：`publish-change-once`
- [x] RabbitMQ 无感安装计划：`internal/installer/rabbitmq`

## 后续建议

- 使用正确 `NODEBRIDGE_RABBITMQ_URL` 和 `NODEBRIDGE_SERVER_MYSQL_DSN` 跑 `docs/v0.3-smoke.md`。
- 下一步进入真实 Canal 前置：CDC offset 模型、Canal adapter 接口、异常恢复策略。
- 对接真实 MySQL 容器：设置 `NODEBRIDGE_APPLY_MYSQL_DSN` 后运行集成测试。
- 进入 CDC 阶段：Canal Go client 选型、offset 保存、异常恢复。

## 待确认

- 项目最终名称是 `NodeBridge` 还是面向用户的 `DataSync`。
- Canal Go client 具体库选型尚未确认。
- Windows Service 实现库、日志库、配置加密实现方式尚未确认。

## 改动记录

- 2026-05-21 08:43 | gpt-5 | 初始化 AI 协作文档体系和编辑器适配入口。
- 2026-05-21 09:05 | gpt-5 | 将 AGENTS.md 和技能说明调整为中英日三语。
- 2026-05-21 10:20 | gpt-5 | 初始化一期 MVP 骨架并通过 Go 测试和 CLI smoke test。
- 2026-05-21 10:44 | gpt-5 | 补强一期测试并实现 RabbitMQ 基础和安装计划模型。
- 2026-05-21 10:55 | gpt-5 | 增加表列重映射契约、mapper 测试和配置样例。
- 2026-05-21 11:24 | gpt-5 | 持久化路线图并实现 V0.2 MySQL Apply 和模拟事件 CLI。
- 2026-05-21 11:34 | gpt-5 | 实现 V0.3 RabbitMQ CLI 与 smoke 文档。
- 2026-05-21 11:40 | gpt-5 | 实现 V0.4 Runtime 初版和 Edge 上传转发入口。
- 2026-05-21 11:44 | gpt-5 | 增加长期 worker 循环、状态快照和日志 Web 入口。
- 2026-05-21 11:48 | gpt-5 | 增加 sync-agent run 并接入 Edge/Server worker 组。
- 2026-05-21 11:58 | gpt-5 | 增加 Edge 下发 Apply、双 worker 编排和三语短注释。
- 2026-05-21 12:10 | gpt-5 | 完成 V0.4 日志 ring buffer、dispatch 计数和测试验收。
- 2026-05-21 13:12 | gpt-5 | 推送 V0.4 并完成 V0.5 CDC stub、normalizer 和测试。
