---
description: "Use when: frontend AI track, DataSync Wails React pages, UI implementation from contract and requirements"
agent: "frontend-track"
tools: [read, edit, search, run]
---

# Frontend Track Prompt

Read first:

- [AGENTS.md](../../AGENTS.md)
- [.ai/docs/frontend-requirements.md](../docs/frontend-requirements.md)
- [.ai/docs/frontend-backend-contract.md](../docs/frontend-backend-contract.md)
- [.ai/docs/ui-design-spec.md](../docs/ui-design-spec.md)
- [.ai/docs/ai-collaboration-log.md](../docs/ai-collaboration-log.md) Active Board

Ownership:

- Edit `frontend/**`.
- Only touch Wails generated binding adapters if needed.
- Do not edit SyncAgent runtime, Apply, RabbitMQ, CDC, or migrations.

Rules:

- Before work, scan `ai-collaboration-log.md` Active Board.
- Use Wails bindings only.
- No `fetch`, no `axios`.
- Keep dark terminal style.
- Record interface questions in the Active Board, not in separate files.
- Reply in Activity Log when backend asks for frontend handling or UI decisions.
- If an open item is solved, mark it `closed` in Active Board and append an answer/decision entry.
- If blocked, mark it `blocked` in Active Board and state the needed backend decision.
- Final response must list Active Board items handled and still open/blocked.
