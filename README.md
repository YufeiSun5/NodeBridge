# NodeBridge

**English** · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [Welcome](https://yufeisun5.github.io/NodeBridge/)

MySQL synchronization for Windows edge networks. Connect a central server and multiple edge nodes through Canal CDC, RabbitMQ and SyncAgent, with a local Wails management application.

## Download and upgrade

[Windows x64 — v0.48.7 Beta](https://github.com/YufeiSun5/NodeBridge/releases/tag/v0.48.7)

Fixes RabbitMQ recovery after disconnection, forwarding of partially committed batches, and bidirectional target database selection. Includes a compact rule editor, prominent display names and automatic NodeBridge system database upgrades during installation.

The installer includes NodeBridge, SyncAgent, Erlang/OTP, RabbitMQ, Java, Canal and WinSW. **Install MySQL separately.** Back up configuration, rules and databases before upgrading. Run the installer as administrator using the intended Windows account. Migration failure blocks installation completion. These fixes require no new business table columns; rename display names without replacing rule IDs or deleting alignment records.

Application files: `C:\Program Files\NodeBridge\app`. Configuration and runtime data: `C:\ProgramData\NodeBridge`. Closing the window keeps the app in the tray; explicit exit requires the configured password.

This is a beta. Isolated tests covered three Windows Agent processes, CRUD convergence, RabbitMQ service interruption and communication pauses. They do not establish performance on three physical computers or a production SLA. [Verification report (Chinese)](docs/v0.48.7-reconnect-handoff-20260915.md).

## Operation

Assign each node a unique `node.id`, such as `server-001` or `edge-001`. Configure source/target database, table and column mappings explicitly. Messages are acknowledged after committed writes; uncertain delivery is retried with event idempotency.

- [LAN deployment guide (Chinese)](docs/lan-deployment-guide.md)
- [Initial alignment](docs/initial-alignment.md)
- [Routing policies](docs/sync-routing-policy.md)
- [Managed components](docs/managed-components.md)
- [Trial runbook](docs/trial-runbook.md)
- [MCP management](docs/mcp-service.md): SSH stdio using `SyncAgent.exe mcp-stdio`; no HTTP listener.

## Development

```powershell
go test ./...
go vet ./...
golangci-lint run ./...
cd frontend
npm ci
npm test
npm run build
```

The welcome page detects browser language (Chinese, Japanese, otherwise English) and remembers manual selection. GitHub README language is selected through the links above.

License: [MIT](LICENSE).
