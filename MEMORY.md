# MEMORY

Last updated: 2026-05-21 16:25 Asia/Shanghai

## 当前阶段

- 项目处于 V0.15 single-pc E2E verified 阶段。
- 当前仓库已有 Go MVP 骨架、配置样例、迁移样例、RabbitMQ 核心接口、表列映射、MySQL Apply Worker 和无感安装计划模型。
- 当前已跑通 Edge A、Server、Edge B 单机 Docker 联调；尚未实现 Canal lab compose、Windows Service 和完整 Wails 前端。

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
- 已实现 V0.6 Canal prep：`cdc.Offset`、`OffsetStore`、内存 offset store、Canal adapter 前置接口。
- 已实现 Canal row change 到 `cdc.ChangeEvent` 的转换，以及 `FetchOnce` 的 fetch、convert、save offset、ack 流程。
- 已新增 `sync-agent canal-check` 用于 Canal 配置 smoke 校验。
- 已评估当前缺口：真实 Canal client、ACK/失败事件持久化、Windows Service、Wails 管理端、安装器仍是后续主线。
- 已实现 V0.7 CDC recovery：MySQL offset store 写入 `sync_upload_offset`，CDC 恢复策略支持指数退避、最大延迟和最大次数。
- 已新增 `docs/v0.7-cdc-recovery.md`，记录 offset 恢复和 fatal error 策略。
- 已测试当前主干后继续迭代，默认门禁通过。
- 已实现 V0.8 Persistence：`syncstore.Store` 支持 ACK、dispatch、error 持久化。
- 已新增失败事件入口：`failed-events` 查询失败 ACK，`retry-event` 将失败事件标记为 `PENDING`。
- 已新增 `docs/v0.8-persistence.md`，记录当前持久化和重试入口范围。
- 已实现 V0.9 replay 基础：Server ingress 持久化 `SyncEvent` payload。
- 已新增 `ReplayRuntime` 和 `replay-pending-once`，支持单步重放 `PENDING` 下发事件。
- 已更新 Server migration：`sync_event_log.event_payload` 和 Server 侧 `sync_error_log`。
- 已新增 `docs/v0.9-replay.md`，记录事件重放入口和限制。
- 已实现 V0.10 auto replay：Server `sync-agent run` 同时启动 `server-ingress` 和 `server-replay`。
- 已让 `server-replay` 使用独立 RabbitMQ 连接和 publisher，避免与 ingress 共用 channel。
- 已新增 `docs/v0.10-auto-replay.md` 和 `docs/delivery-assessment.md`，固化交付判断。
- 已实现 V0.11 single-machine lab：新增 Edge A、Edge B、Server 三份本机配置。
- 已新增开发 Docker Compose：1 个 RabbitMQ、3 个 MySQL，RabbitMQ 用 vhost 隔离节点。
- 已新增 `scripts/lab-smoke.ps1` 和 `docs/single-machine-lab.md`，用于单机测试准备。
- 已实现 V0.12 E2E smoke：新增 `scripts/lab-e2e.ps1` 串联 Edge A -> Server -> Edge B。
- 已让 `consume-once` 支持 `-edges` 参数，可在单步 Server ingress 后执行防回源下发。
- 已补 Edge 侧 `device_settings` 迁移，支持当前映射规则在 Edge B 写入目标表。
- 已新增 `docs/v0.12-e2e-smoke.md`，记录单机 E2E 执行和验证范围。
- 已实现 V0.13 Canal client adapter：新增 `internal/cdc/canal/withlin_client.go`。
- 已引入 `github.com/withlin/canal-go`，并将第三方依赖隔离在 Canal adapter 层。
- 已支持 Canal protobuf message 到 `cdc.ChangeEvent` 的转换、batch ack 和 offset 保存路径。
- 已新增 `docs/v0.13-canal-client.md`，记录真实 Canal client 适配策略和限制。
- 已实现 V0.14 Canal runtime：Edge `sync-agent run` 在 `cdc.type=canal` 时启动 `edge-cdc-canal`。
- 已新增 `canal-publish-once`，用于真实 Canal -> 本地 RabbitMQ 的单步测试。
- 已调整 Canal batch 可靠性顺序：发布成功或安全抑制后才保存 offset 并 ACK Canal。
- 已让 worker 停止时调用支持 `Stop` 的 stepper，确保 Canal source 可关闭。
- 已新增 `docs/v0.14-canal-runtime.md`，记录运行命令和可靠性边界。
- 已修复单机 E2E 验收缺口：Server migration 增加 `sync_apply_log`，避免中心 Apply 因系统表缺失失败。
- 已新增 `sample-events/device_config.insert.change.json`，用 INSERT 样例覆盖空目标表首次同步路径。
- 已修复 Edge 下发目标库选择：Server 仍按规则写中心库，Edge Downlink 按本节点 MySQL 配置写本地库。
- 已增强 `scripts/lab-e2e.ps1`：显式加载 Docker CLI 路径、重置队列和表、逐步执行三节点链路、失败即停。
- 已在本机 Docker 环境跑通 Edge A -> Server -> Edge B E2E，中心库和 Edge B 均验证 `device_settings.setting_value=ON`。

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
- [x] V0.6 Canal prep：`cdc.Offset`、`internal/cdc/canal`、`canal-check`
- [x] V0.7 CDC recovery：MySQL offset store、recovery policy、fatal recovery marker
- [x] V0.8 Persistence：`internal/syncstore`、`failed-events`、`retry-event`
- [x] V0.9 Replay：`event_payload`、`ReplayRuntime`、`replay-pending-once`
- [x] V0.10 Auto replay worker：Server run 挂接 `server-replay`
- [x] V0.11 Single machine lab：lab configs、dev compose、lab smoke script
- [x] V0.12 E2E smoke：`scripts/lab-e2e.ps1`、Server dispatch CLI、Edge B verify path
- [x] V0.13 Canal client adapter：`withlin/canal-go` wrapper、protobuf conversion、ACK path
- [x] V0.14 Canal runtime：`edge-cdc-canal` worker、`canal-publish-once`、publish-before-ack
- [x] V0.15 Single-pc E2E verified：Docker MySQL x3 + RabbitMQ，Edge A -> Server -> Edge B 验证通过
- [x] RabbitMQ 无感安装计划：`internal/installer/rabbitmq`

## 后续建议

- 使用正确 `NODEBRIDGE_RABBITMQ_URL` 和 `NODEBRIDGE_SERVER_MYSQL_DSN` 跑 `docs/v0.3-smoke.md`。
- 下一步进入 V0.16：Windows Service 安装、启动、停止、卸载入口。
- 对接真实 MySQL 容器：设置 `NODEBRIDGE_APPLY_MYSQL_DSN` 后运行集成测试。
- 进入 CDC 阶段：Canal Go client 选型、offset 保存、异常恢复。

## 待确认

- 项目最终名称是 `NodeBridge` 还是面向用户的 `DataSync`。
- Canal Go client 当前使用 `github.com/withlin/canal-go`，后续可替换，依赖已隔离。
- Windows Service 实现库、日志库、配置加密实现方式尚未确认。
- 交付节奏：当前已具备技术试点脚本基础；V0.17 后可做客户试用，V1.0 才是产品交付。

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
- 2026-05-21 13:19 | gpt-5 | 实施 V0.6 Canal prep、offset 模型和 canal-check。
- 2026-05-21 14:05 | gpt-5 | 评估缺口并完成 V0.7 MySQL offset store 与恢复策略。
- 2026-05-21 14:38 | gpt-5 | 测试后完成 V0.8 持久化仓储和失败事件入口。
- 2026-05-21 15:08 | gpt-5 | 完成 V0.9 事件载荷持久化和单步重放入口。
- 2026-05-21 15:27 | gpt-5 | 完成 V0.10 后台重放 worker 并固化交付评估。
- 2026-05-21 15:49 | gpt-5 | 完成 V0.11 单机 lab 配置、Compose 和准备脚本。
- 2026-05-21 16:14 | gpt-5 | 完成 V0.12 单机 E2E smoke 脚本和 Server 下发入口。
- 2026-05-21 16:43 | gpt-5 | 完成 V0.13 Canal client 适配层和 protobuf 转换。
- 2026-05-21 17:11 | gpt-5 | 完成 V0.14 Canal runtime 接入和单步发布入口。
- 2026-05-21 16:25 | gpt-5 | 完成 V0.15 单机三节点 E2E 修复、脚本验收和测试。
