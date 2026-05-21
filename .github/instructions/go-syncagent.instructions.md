---
applyTo: "**/*.go,cmd/**,internal/**,configs/**,migrations/**,deploy/**"
---

Read `AGENTS.md`, `MEMORY.md`, and `.ai/instructions/go-syncagent.md` before editing these files.

Rules:

- Keep SyncAgent runtime logic out of the Wails UI process.
- Preserve focused package boundaries under `internal/`.
- ACK RabbitMQ messages only after successful business handling.
- Mark uncertain design decisions as `<!-- 待确认 -->`.
