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
├─ RabbitMQ container
│  ├─ vhost edge-a-sync
│  ├─ vhost edge-b-sync
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

## Next E2E Step

After V0.11, V0.12 should add an end-to-end lab script:

1. Publish Edge A sample change to `edge-a-sync`.
2. Forward Edge A upload to `server-sync`.
3. Consume Server ingress and dispatch to Edge B.
4. Consume Edge B downlink.
5. Query Server and Edge B MySQL to verify rows.
