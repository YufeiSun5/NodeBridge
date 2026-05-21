---
description: "Use when: 新增或修改 Wails2 管理端页面、React TypeScript、配置页、状态页、日志页、队列页"
agent: "default"
tools: [read, edit, search, run]
---

# Add Management Page

Read first:

- [AGENTS.md](../../AGENTS.md)
- [MEMORY.md](../../MEMORY.md)
- [.ai/instructions/frontend-wails.md](../instructions/frontend-wails.md)

## Prompt

请新增或修改一个 Wails2 管理端页面。页面只承担管理、状态、配置、日志或诊断职责，不实现同步核心逻辑。

要求：

- API 调用集中在 `frontend/src/services/`。
- DTO 字段与 Go JSON 字段保持一致。
- 补齐加载、错误和空状态。
- 涉及配置保存时提供连接测试或明确状态反馈。
- 完成后更新 `MEMORY.md`。
