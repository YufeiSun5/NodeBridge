# MEMORY

Last updated: 2026-05-21 10:44 Asia/Shanghai

## 当前阶段

- 项目处于一期补强与二期 RabbitMQ 初始迭代阶段。
- 当前仓库已有 Go MVP 骨架、配置样例、迁移样例、RabbitMQ 核心接口和无感安装计划模型。
- 当前尚未实现真实 MySQL Apply、Canal CDC 和完整 Wails 前端。

## 已完成事项

- 已绑定 Git remote：`https://github.com/YufeiSun5/NodeBridge.git`。
- 已创建根级 `AGENTS.md`，作为 Codex/Copilot/Cursor 的项目路引入口。
- 已创建 `.ai/` 母本文档目录，包括 instructions、docs、agents、prompts。
- 已创建 Copilot 与 Cursor 的薄适配层。
- 已将 `AGENTS.md` 和技能使用说明调整为中英日三语。
- 已初始化一期 MVP 骨架：Go module、vfox 工具链锁定、双命令入口、核心模型、配置样例、迁移样例和基础测试。
- 已补强一期测试门禁，并新增 RabbitMQ 拓扑、publisher、consumer、队列状态和 Windows 无感安装计划模型。

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
- [x] RabbitMQ 无感安装计划：`internal/installer/rabbitmq`

## 后续建议

- 对接真实 RabbitMQ 容器：设置 `NODEBRIDGE_RABBITMQ_URL` 后运行集成测试。
- 继续 RabbitMQ 阶段：连接管理、重试/死信策略、管理 API 队列状态。
- 进入 MySQL Apply 阶段：连接池、动态 SQL、事务、`sync_apply_log` 幂等处理。
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
