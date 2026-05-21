---
applyTo: "frontend/**,cmd/datasync-ui/**,**/*.ts,**/*.tsx,**/*.css"
---

Read `AGENTS.md`, `MEMORY.md`, and `.ai/instructions/frontend-wails.md` before editing these files.

Rules:

- UI manages configuration, status, logs, diagnostics, and service control only.
- Put API calls under `frontend/src/services/`.
- Keep DTO fields aligned with Go JSON fields.
- Update `MEMORY.md` after meaningful changes.
