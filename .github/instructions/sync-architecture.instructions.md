---
applyTo: "internal/event/**,internal/normalizer/**,internal/loop/**,internal/apply/**,internal/router/**,internal/rabbitmq/**,migrations/**,configs/**"
---

Read `AGENTS.md`, `MEMORY.md`, and `.ai/instructions/sync-architecture.md` before editing these files.

Rules:

- Treat `SyncEvent` as the stable cross-module contract.
- Do not rely on CDC or RabbitMQ for loop suppression.
- Preserve Apply Worker idempotency and transaction boundaries.
- Server must not dispatch events back to `origin_node_id`.
