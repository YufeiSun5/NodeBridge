# Delivery Assessment

Last updated: 2026-05-22

## Current Readiness

NodeBridge is not yet production-deliverable, but it is close to a technical pilot.

The current codebase has the main Go runtime shape: config persistence, secret protection, mapping, MySQL apply, RabbitMQ publish/consume, batch runtime workers, log web, CDC preparation, offset persistence, failure tracking, replay entry points, node registration, config downlink, CRUD semantic E2E scripts, Wails backend APIs, tray-support backend methods, and a Wails React UI skeleton.

## Earliest Pilot

Earliest technical pilot target: after V0.12.

Pilot means:

- One Server and two Edge nodes.
- Docker or manually prepared MySQL/RabbitMQ allowed for lab validation.
- Canal can still be a controlled adapter path.
- Wails UI can be minimal or replaced by CLI plus log web for engineering validation.

Required before pilot:

- V0.11: single-machine lab topology and repeatable preparation script.
- V0.12: full Docker MySQL/RabbitMQ smoke script for Edge A -> Server -> Edge B.
- V0.15: separate RabbitMQ brokers for Edge A, Edge B, and Server in single-PC lab.
- V0.13: real Canal client integration path or a confirmed production CDC adapter.
- V0.14: Canal runtime path can publish to Edge local RabbitMQ after successful fetch.
- V0.17: Server-managed node registration, dynamic dispatch, and config downlink.
- V0.18: CRUD, soft delete, idempotency, one-way table, and remapping E2E against separated brokers.
- V0.19: batch publish/apply with 50-message lab verification and Wails React skeleton.
- Basic runbook for config, migration, topology init, run, retry, and log web.

## Single Machine Testing

One PC is enough for development and lab verification.

Recommended layout:

- Three RabbitMQ containers: Edge A `5673`, Edge B `5674`, Server `5675`.
- Three MySQL containers on ports `3307`, `3308`, and `3309`.
- Three SyncAgent processes using `configs/lab/*.local.yaml`.

Limits:

- It validates local broker buffering and Server broker disconnect/reconnect on one PC.
- It does not fully replace physical network partition testing between machines.
- It does not prove final installer behavior.
- It does not replace final offline RabbitMQ installer testing.

See `docs/single-machine-lab.md`.

V0.12 adds the executable smoke entry: `scripts/lab-e2e.ps1`.

V0.18 adds the full semantic entry: `scripts/lab-crud-e2e.ps1`.

V0.19 adds the batch entry: `scripts/lab-batch-e2e.ps1`.

## Earliest Customer Trial

Earliest customer trial target: after V0.23.

Customer trial means:

- Configurable external MySQL and RabbitMQ.
- Default RabbitMQ managed install path documented.
- DataSync can run as a Wails tray-persistent app after user login.
- Wails has minimal config/status/rule/failure/log pages.
- Diagnostic package export exists.
- CRUD and one-way table E2E pass against the separated broker lab.
- Batch publish and batch apply have a measured baseline on mechanical-disk-friendly settings.
- Wails frontend build is verified with a working npm runtime.

Required before customer trial:

- V0.22: Wails backend runtime start/stop/restart works safely.
- V0.23: real Canal CDC E2E is verified.
- Frontend tray UI calls `VerifyExitPassword`, `GetAutoStart`, and `SetAutoStart`.
- Diagnostic export and installer runbook.

## Product Delivery

Production delivery target: after V1.0.

V1.0 must include:

- Offline Windows installer bundling Erlang/OTP, RabbitMQ, SyncAgent/DataSync runtime, and DataSync UI.
- External RabbitMQ override for customers who already have RabbitMQ.
- Managed component manifest so install/repair/uninstall only touches NodeBridge-owned RabbitMQ and Canal resources.
- Config encryption for passwords/tokens.
- Recovery behavior tested under disconnect/reconnect.
- Failure retry, batch retry, and dead-letter preview documented.
- End-to-end tests covering table and column remapping.

## Remaining Critical Risks

- Real Canal E2E and a 20-row soak have passed; longer restart/overnight soak is still needed.
- Wails tray behavior is still a frontend open item.
- `StartAgent`, `StopAgent`, and `RestartAgent` have backend process control; fixed-directory package smoke now passes, but final installer verification is still needed.
- Failure retry minimum closure has passed; frontend still needs to expose batch retry and dead-letter preview.
- Stress test has a batch producer, but long-running soak and multi-edge concurrency baselines are still needed.
- 11-node soak and disconnect scripts now exist and passed minimal runs; overnight and larger-count baselines are still needed before production.
- Wails frontend is not yet a complete management UI.
- RabbitMQ/Canal installers have an alpha executor for manifest, Canal config, and topology init; V0.33 adds offline package hash preflight and command planning, but real offline package execution is still pending.
- Integration tests depend on user-provided Docker services or DSNs.

See `docs/backend-completion-plan.md` for the detailed backend completion plan.
