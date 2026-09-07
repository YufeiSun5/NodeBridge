# NodeBridge

NodeBridge 是面向 Windows 内网环境的 MySQL 数据同步程序。它在一个中心节点和多个边缘节点之间，通过 Canal CDC、RabbitMQ 和 `SyncAgent` 同步业务数据，并提供 Wails 管理界面、失败重试、诊断和基于 SSH stdio 的 MCP 管理入口。

```text
边缘 MySQL -> Canal -> 边缘 RabbitMQ -> 中心 RabbitMQ -> 中心 MySQL
中心 MySQL -> Canal -> 中心 RabbitMQ -> 指定边缘 MySQL
```

## 当前发行版

当前 Beta：[`v0.46.3`](https://github.com/YufeiSun5/NodeBridge/releases/tag/v0.46.3)

Windows x64 安装包包含 NodeBridge、SyncAgent、Erlang/OTP、RabbitMQ、Java Runtime、Canal 和 WinSW。MySQL 不包含在安装包中，需要在中心服务器和各边缘节点独立准备。

安装目录和数据目录：

```text
C:\Program Files\NodeBridge\app
C:\ProgramData\NodeBridge
```

安装时请使用计划运行 NodeBridge 的 Windows 账户，并以管理员身份启动安装包。管理界面关闭后会驻留系统托盘；退出需要配置中的退出密码。

## 部署入口

- [中心服务器、N 个边缘节点和调试电脑部署手册](docs/lan-deployment-guide.md)
- [MCP 工具和字段说明](docs/mcp-service.md)
- [同步方向和分发策略](docs/sync-routing-policy.md)
- [受管组件边界](docs/managed-components.md)
- [现场试运行手册](docs/trial-runbook.md)

## 节点角色

| 角色 | 数量 | 主要职责 |
| --- | ---: | --- |
| 中心服务器 | 1 | 中心 MySQL、中心 RabbitMQ、汇总 Apply、节点注册和下发 |
| 边缘节点 | N | 本地 MySQL、Canal CDC、本地断网缓冲、中心上传和下发 Apply |
| 调试电脑 | 1-N | 通过 SSH 公钥启动远程 MCP stdio，不部署同步运行时 |

每台 NodeBridge 电脑必须使用唯一 `node.id`。推荐中心使用 `server-001`，边缘依次使用 `edge-001`、`edge-002`。

## MCP 管理

MCP 不监听 HTTP 端口。调试电脑通过 SSH 登录目标 Windows，再启动目标机上的：

```text
SyncAgent.exe mcp-stdio
```

每位调试人员、每台调试电脑应使用独立 SSH 密钥。Windows 与 Mac 的授权、Codex 注册和撤权步骤见[部署手册](docs/lan-deployment-guide.md#mcp-与调试电脑授权)。

## 开发验证

```powershell
go test ./...
go vet ./...
golangci-lint run ./...
```

前端验证：

```powershell
cd frontend
npm run test
npm run build
```

许可证：[MIT](LICENSE)。
