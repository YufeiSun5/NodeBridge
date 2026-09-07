# NodeBridge MCP v0.46.3 Lab

本版供 Windows 内网测试。另一台 Windows 或 Mac 上的 AI 客户端通过 SSH 启动被测机的 `SyncAgent.exe mcp-stdio`。MCP 不监听 HTTP 端口，SSH 提供加密和登录身份。

一台中心服务器、N 台边缘服务器以及 Windows/Mac 调试电脑的完整部署和授权步骤见[内网部署与远程管理手册](lan-deployment-guide.md)。

## 实验室全权限模式

启动参数 `-lab-full-access` 临时解除 MCP enable 开关、完整初始配置、管理解锁以及非敏感字段白名单限制。可以首次创建配置，修改 MySQL/RabbitMQ/Canal 密码、管理/退出密码、日志 token、MCP 开关及所有同步配置字段。

模式只作用于带此参数的 MCP 进程，不改变 Wails UI 的鉴权。省略参数即恢复普通模式：必须启用 MCP 且已有完整配置，配置 patch 仍禁止敏感字段。实验室模式运行时，UI 开关不能关闭该进程，需由客户端断开连接。

仍执行配置和规则校验、DPAPI 加密、响应脱敏及写操作审计。不提供任意系统 shell 或通用 SQL 执行工具。全权限指 NodeBridge 管理能力，不是突破 Windows 文件权限或解密其他账户的配置。

## 在被测 Windows 上准备

1. 安装本版 EXE。默认程序路径为 `C:\Program Files\NodeBridge\app`，配置在 `C:\ProgramData\NodeBridge`。
2. 开启 Windows OpenSSH Server，确认控制电脑能执行 `ssh 用户名@被测机IP whoami`。
3. SSH 登录账户使用被测机配置文件的拥有者。同一账户可读取其 DPAPI 配置；不要把另一台电脑加密过的 `config.yaml` 直接复制过来。
4. 配置 SSH 公钥登录。先在控制电脑手工连接一次，核对并接受主机指纹。生成的 MCP 命令使用 `BatchMode=yes`，不能交互输入密码或首次确认指纹。
5. 在被测机 PowerShell 中生成客户端配置：

```powershell
& 'C:\Program Files\NodeBridge\app\mcp-lab-client-config.ps1' -SshHost 'labuser@192.168.1.25'
```

默认输出到被测机当前用户桌面 `nodebridge-mcp-lab.json`。将其中 `mcpServers.nodebridge` 的配置导入控制电脑的 MCP 客户端。支持 JSON 的客户端可直接导入；其他客户端按其配置格式使用相同的 `command` 和 `args`。

Mac 使用非默认密钥时，生成配置时加 `-ClientKeyPath '/Users/你的用户名/.ssh/id_ed25519'`。此路径属于控制电脑。脚本必须在被测 Windows 上运行，保证 EXE/配置路径属于被测机。

也可直接调用 CLI：

```powershell
& 'C:\Program Files\NodeBridge\app\SyncAgent.exe' mcp-client-config -lab-full-access -ssh-host 'labuser@192.168.1.25' -config 'C:\ProgramData\NodeBridge\config.yaml'
```

本机 stdio 测试：

```powershell
& 'C:\Program Files\NodeBridge\app\SyncAgent.exe' mcp-stdio -lab-full-access -config 'C:\ProgramData\NodeBridge\config.yaml'
```

首次无配置时默认规则路径与配置同目录。先保存有效配置，再保存规则，最后启动同步。实验室模式下，`nodebridge_save_config_patch` 可传 `restart_agent:true`，保存成功后立即重新加载配置并重启 SyncAgent；省略时保持旧版行为，不自动启动或重启。

## 工具

普通和实验室模式均有：

- `nodebridge_overview`：真实健康探测和节点概览。
- `nodebridge_config_summary`：脱敏配置和当前权限模式。
- `nodebridge_queue_status`：RabbitMQ 实际队列深度和消费者。
- `nodebridge_sync_rules`、`nodebridge_save_sync_rules`：读取/保存规则。
- `nodebridge_failed_events`：MySQL 失败事件。
- `nodebridge_logs`：最多 500 行、末尾最多 1 MiB，按 limit 返回。
- `nodebridge_diagnostic_summary`：路径、传输和能力说明。
- `nodebridge_validate_config_patch`、`nodebridge_save_config_patch`：预检/保存部分配置；实验室模式保存时可选 `restart_agent:true`。
- `nodebridge_node_options`：读取已注册节点。
- `nodebridge_test_mysql`、`nodebridge_test_rabbitmq`：测试已保存的连接。
- `nodebridge_agent_status`：识别本版 UI、CLI 或其他 MCP 会话启动的进程。
- `nodebridge_mysql_schema`：查询库内表、列、类型和主键；可传 `table`。

实验室模式额外提供：

- `nodebridge_start_agent`、`nodebridge_stop_agent`、`nodebridge_restart_agent`。
- `nodebridge_retry_failed_event`、`nodebridge_retry_failed_events`。
- `nodebridge_dead_letters`：预览后 requeue，不删除消息。
- `nodebridge_managed_install_plan`、`nodebridge_apply_managed_install`：受管配置和 RabbitMQ 拓扑初始化；不等于安装系统组件的 NSIS 流程。
- `nodebridge_ensure_server_edge_user`：仅中心节点使用；按给定 `node_id` 创建或迁移该边缘节点连接中心 RabbitMQ 的账号。
- `nodebridge_export_diagnostics`：在被测机生成 ZIP，返回路径后可用 SCP 取回。
- `nodebridge_get_autostart`、`nodebridge_set_autostart`：当前 Windows 账户的 UI 自启动。

资源：`nodebridge://overview`、`nodebridge://config`、`nodebridge://sync-rules`、`nodebridge://diagnostic-summary`、`nodebridge://logs`。

## 首轮远程配置示例

先调用 `nodebridge_validate_config_patch`，再以相同参数调用 `nodebridge_save_config_patch`：

```json
{
  "patch": {
    "mode": "edge",
    "node": {"id": "edge-002", "name": "Lab Edge 2"},
    "mysql": {
      "host": "127.0.0.1", "port": 3306,
      "username": "nodebridge", "password": "your-lab-password", "database": "scada_edge"
    },
    "rabbitmq": {"mode": "managed", "install": true, "server_url": "amqp://nb-edge-002:1234@192.168.1.10:5672/%2Fnodebridge-server"}
  }
}
```

省略字段保留原值。`******` 保留现有密码；空字符串清空对应密码。实验室模式可在最外层传 `"restart_agent": true`，保存成功后立即重启 SyncAgent。保存规则必须提供 `rules` 数组；明确传 `[]` 才表示清空规则。

托管 RabbitMQ 的用户名和密码不接受各机器随意指定：保存时按 `mode`/`node.id` 规范化，并先修改本机 RabbitMQ 服务用户，成功后才写配置。中心服务器还需为每台边缘机调用一次 `nodebridge_ensure_server_edge_user`，例如 `{"node_id":"edge-002"}`。

建议顺序：读取配置 -> 预检/保存 -> 测试 MySQL/RabbitMQ -> 查询表结构 -> 保存规则 -> 初始化受管拓扑 -> 启动/重启 -> 查看状态/队列/日志。

## 并发和边界

同一配置路径的 MCP 请求通过跨进程文件锁串行执行，避免两个会话覆盖彼此的 patch。同步运行进程有独占锁，退出/崩溃后由 OS 释放。旧版运行进程没有状态文件：升级时先在旧版 UI 停止同步并退出，再安装新版。

MCP 与 Wails 同时编辑配置仍应由操作员协调；UI 中已经打开的旧编辑内容不会自动合并远端更改。远程修改后重新打开 UI，再继续本地编辑。

审计路径：配置目录的 `logs/mcp-audit.log`。未提供另一台机器的连接资料前，不能把本机协议测试称为已完成真实跨机验证。

English: Windows lab server with explicit full-management mode over SSH stdio. Keep configuration validation, encrypted storage and redacted output. No HTTP listener.

日本語: Windows 実験用の全管理モードです。SSH 経由の stdio 接続を使用し、設定検証・暗号化・マスキングを維持します。

协议错误和工具执行错误采用不同返回形式，依据 [MCP tools specification](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)。
