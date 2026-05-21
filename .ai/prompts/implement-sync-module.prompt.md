---
description: "Use when: 规划并实现一个 SyncAgent 同步核心模块、Go 后端包、CDC、RabbitMQ、Apply、Loop、Router"
agent: "default"
tools: [read, edit, search, run]
---

# Implement Sync Module

Read first:

- [AGENTS.md](../../AGENTS.md)
- [MEMORY.md](../../MEMORY.md)
- [.ai/instructions/go-syncagent.md](../instructions/go-syncagent.md)
- [.ai/instructions/sync-architecture.md](../instructions/sync-architecture.md)

## Prompt

请为当前任务实现一个 SyncAgent 同步模块。先确认该模块归属的 `internal/<domain>` 包和上下游边界，再实现最小可测试功能。

要求：

- 保持包职责单一。
- 长生命周期逻辑支持 `context.Context` 停止。
- 外部 IO 错误可诊断。
- 涉及 RabbitMQ 消费时，业务成功后才 ACK。
- 涉及 Apply 时，必须考虑幂等和事务。
- 完成后更新 `MEMORY.md`。
