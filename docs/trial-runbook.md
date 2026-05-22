# Trial Runbook

## Scope

本手册用于工程人员准备客户试用或现场技术试点。当前版本不是最终安装器，RabbitMQ、MySQL、Canal 可以由工程人员预先准备，也可以使用 Docker lab 验证。

## 1. Build Package Directory

```powershell
cd D:\DEV_D\NodeBridge
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package-smoke.ps1 -RefreshConfig
```

输出目录：

```text
build/bin/
  DataSync.exe
  SyncAgent.exe
  config.yaml
  sync-rules.yaml
```

## 2. Edit Configuration

编辑 `build/bin/config.yaml`：

- `mode`: `edge` 或 `server`。
- `node.id`: 节点唯一编号。
- `mysql.*`: 客户 MySQL 连接信息。
- `rabbitmq.local_url`: Edge 本地 RabbitMQ，仅 Edge 需要。
- `rabbitmq.server_url`: 中心 RabbitMQ。
- `cdc.*`: Canal 地址、destination、filter。
- `security.admin_password`: 管理解锁密码。
- `security.exit_password`: 退出托盘常驻程序的密码。

同步规则编辑 `build/bin/sync-rules.yaml`。

## 3. Initialize Database Tables

Edge：

```powershell
.\build\bin\SyncAgent.exe migrate -config .\build\bin\config.yaml -scope edge
```

Server：

```powershell
.\build\bin\SyncAgent.exe migrate -config .\build\bin\config.yaml -scope server
```

## 4. Initialize RabbitMQ Topology

Edge 本地 RabbitMQ：

```powershell
.\build\bin\SyncAgent.exe init-rabbitmq -mode edge -amqp-url <edge-local-amqp-url>
```

Server RabbitMQ：

```powershell
.\build\bin\SyncAgent.exe init-rabbitmq -mode server -config .\build\bin\config.yaml -amqp-url <server-amqp-url>
```

## 5. Verify CDC

```powershell
.\build\bin\SyncAgent.exe canal-check -config .\build\bin\config.yaml
```

开发实验室可运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-canal-prepare.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-canal-e2e.ps1 -SkipPrepare
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-canal-soak.ps1 -SkipPrepare -Count 20
```

## 6. Start UI

```powershell
.\build\bin\DataSync.exe
```

当前试用版采用 Wails 托盘常驻模型：

- 点窗口关闭应隐藏到托盘。
- 显式退出需要退出密码。
- 开机自启动是当前用户登录启动，不是 Windows Service。

## 7. Retry And Dead Letters

查看失败事件：

```powershell
.\build\bin\SyncAgent.exe failed-events -config .\build\bin\config.yaml -limit 50
```

批量标记失败事件为待重试：

```powershell
.\build\bin\SyncAgent.exe retry-failed-batch -config .\build\bin\config.yaml -limit 50
```

Server 侧重放待处理事件：

```powershell
.\build\bin\SyncAgent.exe replay-pending-once -config .\build\bin\config.yaml -rules .\build\bin\sync-rules.yaml
```

只读预览死信队列：

```powershell
.\build\bin\SyncAgent.exe dead-letters -config .\build\bin\config.yaml -limit 20
```

开发实验室可运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-retry-e2e.ps1 -SkipPrepare
```

## 8. Managed Components And MCP Alpha

查看受管组件计划：

```powershell
.\build\bin\SyncAgent.exe managed-plan -config .\build\bin\config.yaml -manifest .\build\bin\install-manifest.json
```

执行 V0.31 alpha 安全动作：

```powershell
.\build\bin\SyncAgent.exe managed-apply -config .\build\bin\config.yaml -manifest .\build\bin\install-manifest.json
```

校验离线安装包 catalog，不执行安装：

```powershell
.\build\bin\SyncAgent.exe installer-assets-check -catalog .\deploy\windows\nodebridge-assets.example.json
.\build\bin\SyncAgent.exe installer-command-plan -catalog .\deploy\windows\nodebridge-assets.example.json
```

启动只读 stdio MCP：

```powershell
.\build\bin\SyncAgent.exe mcp-stdio -config .\build\bin\config.yaml -rules .\build\bin\sync-rules.yaml
```

## 9. 11 Node Soak

最小验证：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-11-soak-e2e.ps1 -SkipPrepare -Iterations 1 -CountPerEdge 1 -MultiCount 2
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-11-disconnect-e2e.ps1 -SkipPrepare -Count 2
```

试用前建议长测：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-11-soak-e2e.ps1 -SkipPrepare -Iterations 10 -CountPerEdge 20 -MultiCount 50
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\lab-11-disconnect-e2e.ps1 -SkipPrepare -Count 50
```

## 10. Known Trial Limits

- 离线安装器尚未完成。
- RabbitMQ/Canal 无感安装执行器已有 alpha；真实 Erlang/RabbitMQ/Canal 离线安装包执行仍未完成。
- 失败重试/死信已有最小闭环；前端批量操作入口仍需接入。
- 冲突策略仍需增强。
- 11 节点最小 soak 已可执行；过夜长测仍需单独安排。
