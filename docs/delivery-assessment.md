# Delivery Assessment

Last updated: 2026-05-21

## Current Readiness

NodeBridge is not yet production-deliverable, but it is close to a technical pilot.

The current codebase has the main Go runtime shape: config, mapping, MySQL apply, RabbitMQ publish/consume, runtime workers, log web, CDC preparation, offset persistence, failure tracking, replay entry points, node registration, config downlink, and CRUD semantic E2E scripts.

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
- It does not prove Windows Service installation.
- It does not replace final offline RabbitMQ installer testing.

See `docs/single-machine-lab.md`.

V0.12 adds the executable smoke entry: `scripts/lab-e2e.ps1`.

V0.18 adds the full semantic entry: `scripts/lab-crud-e2e.ps1`.

## Earliest Customer Trial

Earliest customer trial target: after V0.19.

Customer trial means:

- Configurable external MySQL and RabbitMQ.
- Default RabbitMQ managed install path documented.
- SyncAgent can run as a Windows Service.
- Wails has minimal config/status/rule/failure/log pages.
- Diagnostic package export exists.
- CRUD and one-way table E2E pass against the separated broker lab.
- Batch publish and batch apply have a measured baseline on mechanical-disk-friendly settings.

Required before customer trial:

- V0.19: batching with `50 events or 500ms`, ordered consume/apply, and benchmark notes.
- Windows Service install/start/stop/uninstall.
- Wails management MVP without occupying a frontend port.
- Diagnostic export and installer runbook.

## Product Delivery

Production delivery target: after V1.0.

V1.0 must include:

- Offline Windows installer bundling Erlang/OTP, RabbitMQ, SyncAgent, and DataSync UI.
- External RabbitMQ override for customers who already have RabbitMQ.
- Config encryption for passwords/tokens.
- Recovery behavior tested under disconnect/reconnect.
- Failure retry and dead-letter operation documented.
- End-to-end tests covering table and column remapping.

## Remaining Critical Risks

- Real Canal client selection is still not confirmed.
- Windows Service implementation library is still not selected.
- Batch publish/apply is not implemented yet.
- Wails frontend is still only a Go shell.
- RabbitMQ installer is currently a plan model, not an executable installer.
- Integration tests depend on user-provided Docker services or DSNs.
