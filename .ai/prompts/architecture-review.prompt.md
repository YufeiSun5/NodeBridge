---
description: "Use when: 对同步架构和实现做只读一致性审查、检查回环抑制、ACK、Apply 幂等、模块边界"
agent: "architecture-review"
tools: [read, search]
---

# Architecture Review

Use agent:

- [.ai/agents/architecture-review.agent.md](../agents/architecture-review.agent.md)

Read first:

- [AGENTS.md](../../AGENTS.md)
- [.ai/instructions/go-syncagent.md](../instructions/go-syncagent.md)
- [.ai/instructions/sync-architecture.md](../instructions/sync-architecture.md)

## Prompt

请对当前改动做只读架构审查，按严重程度列出问题。优先检查：

- SyncAgent 与 Wails UI 进程边界。
- `SyncEvent`、Loop Suppressor、Apply Worker 的关键不变量。
- RabbitMQ 可靠投递和可靠消费。
- Server 防回源与 Edge 本地回环抑制。
- 配置、迁移和实现之间的不一致。
