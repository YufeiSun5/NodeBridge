---
description: "Use when: Wails React TypeScript frontend implementation, management page scaffold, dark terminal UI, DataSync UI"
agent: "frontend-implementation"
tools: [read, edit, search, run]
---

# Frontend Implementation Prompt

Before editing, read:

- [AGENTS.md](../../AGENTS.md)
- [.ai/instructions/frontend-wails.md](../instructions/frontend-wails.md)
- [.ai/docs/ui-design-spec.md](../docs/ui-design-spec.md)
- [MEMORY.md](../../MEMORY.md)

Task:

Implement or refine the Wails React frontend under `frontend/` and only touch `cmd/datasync-ui` when Wails bindings require it.

Rules:

- Use Wails IPC bindings only; no `fetch`, no `axios`.
- Keep the dark industrial terminal style.
- Keep comments short and strong.
- Do not change SyncAgent synchronization logic.
- Run `npm run build` and relevant Go tests before handoff.
