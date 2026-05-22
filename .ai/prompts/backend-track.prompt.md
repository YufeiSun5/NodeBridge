---
description: "Use when: backend AI track, Wails API contract, DTO, DataSync UI backend methods"
agent: "backend-track"
tools: [read, edit, search, run]
---

# Backend Track Prompt

Read first:

- [AGENTS.md](../../AGENTS.md)
- [.ai/docs/frontend-backend-contract.md](../docs/frontend-backend-contract.md)
- [.ai/docs/frontend-requirements.md](../docs/frontend-requirements.md)
- [.ai/docs/ai-collaboration-log.md](../docs/ai-collaboration-log.md) Active Board
- [.ai/instructions/go-syncagent.md](../instructions/go-syncagent.md)

Ownership:

- Edit `cmd/datasync-ui/**`.
- Edit `internal/uiapi/**` or backend DTO/query helpers.
- Do not change frontend visuals unless the contract requires it.

Rules:

- Before every backend dialog or task, read `ai-collaboration-log.md` Active Board and scan every `open` or `blocked` item.
- Keep Wails method names and JSON fields stable.
- Return empty/unknown/unsupported instead of fake success.
- Redact secrets before returning config.
- Record interface changes in the Active Board before changing contract.
- Proactively record frontend-facing questions, DTO changes, required UI updates, and blockers in Active Board.
- If an open item is solved, mark it `closed` in Active Board and append an answer/decision entry.
- If blocked, mark it `blocked` in Active Board and state the needed frontend or product decision.
- Final response must list Active Board items handled and still open/blocked.
