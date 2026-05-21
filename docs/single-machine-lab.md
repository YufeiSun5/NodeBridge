# Single Machine Lab

一台电脑可以完成当前阶段的开发测试。

## 判断

可以测试：

- 配置加载、迁移、RabbitMQ 拓扑初始化。
- 模拟 CDC：`ChangeEvent` -> `SyncEvent` -> RabbitMQ。
- Server Apply、下发、Edge Apply。
- 表名和列名重映射。
- 失败事件标记和 replay worker。

暂时不能完整测试：

- 真实 Canal binlog 捕获。
- Windows Service 安装和开机自启。
- Wails 管理端完整交互。
- RabbitMQ 离线无感安装包。

## 单机拓扑

```text
One Windows PC
├─ RabbitMQ edge-a container 127.0.0.1:5673 / 15673
│  └─ vhost edge-a-sync
├─ RabbitMQ edge-b container 127.0.0.1:5674 / 15674
│  └─ vhost edge-b-sync
├─ RabbitMQ server container 127.0.0.1:5675 / 15675
│  └─ vhost server-sync
├─ MySQL edge-a container 127.0.0.1:3307 scada_edge
├─ MySQL edge-b container 127.0.0.1:3308 scada_edge
└─ MySQL server container 127.0.0.1:3309 scada_center
```

Docker is for development only. Final delivery still uses Windows installer and configurable external services.

## Prepare

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/lab-smoke.ps1
```

The script starts dev containers when Docker CLI is available, validates lab configs, runs migrations, and initializes RabbitMQ topology.

## Configs

- `configs/lab/edge-a.local.yaml`
- `configs/lab/edge-b.local.yaml`
- `configs/lab/server.local.yaml`

The lab uses `rabbitmq.mode: external` and `install: false` because Docker RabbitMQ is not a delivery dependency.

## E2E Smoke

V0.12 adds:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/lab-e2e.ps1
```

The script:

1. Publish Edge A sample change to `edge-a-sync`.
2. Forward Edge A upload from Edge A RabbitMQ to Server RabbitMQ.
3. Consume Server ingress and dispatch to ACTIVE Edge nodes from `sync_node_registry`.
4. Consume Edge B downlink.
5. Query Server and Edge B MySQL to verify rows.

Details: `docs/v0.12-e2e-smoke.md`.

## Config Downlink Smoke

V0.17 adds HTTP registration and RabbitMQ config delivery:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/lab-config-e2e.ps1 -SkipPrepare
```

Details: `docs/v0.17-node-management.md`.

## CRUD Semantic E2E

V0.18 adds full CRUD and one-way-table verification:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/lab-crud-e2e.ps1 -SkipPrepare
```

The script verifies `device_config -> device_settings` INSERT, duplicate idempotency, UPDATE, soft DELETE, table/column remapping, and `alarm_history` as `EDGE_TO_SERVER` without Edge B dispatch.

Details: `docs/v0.18-crud-e2e.md`.
