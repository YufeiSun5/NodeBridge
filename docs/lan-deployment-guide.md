# NodeBridge 内网部署与远程管理手册

本文面向以下现场：一台中心服务器、N 台边缘服务器，以及若干 Windows 或 Mac 调试电脑。示例名称和地址需要替换为现场实际值。

## 1. 目标拓扑

```mermaid
flowchart LR
    subgraph Debug[调试电脑]
      W[Windows Codex]
      M[Mac Codex]
    end
    subgraph Center[中心服务器 server-001]
      SM[中心 MySQL]
      SR[中心 RabbitMQ]
      SC[中心 Canal]
      SN[NodeBridge Server]
    end
    subgraph Edges[边缘节点 x N]
      E1[edge-001<br/>MySQL + RabbitMQ + Canal + NodeBridge]
      E2[edge-002<br/>MySQL + RabbitMQ + Canal + NodeBridge]
      EN[edge-N<br/>MySQL + RabbitMQ + Canal + NodeBridge]
    end
    W -. SSH / MCP stdio .-> SN
    W -. SSH / MCP stdio .-> E1
    M -. SSH / MCP stdio .-> SN
    M -. SSH / MCP stdio .-> E2
    E1 -->|AMQP 5672| SR
    E2 -->|AMQP 5672| SR
    EN -->|AMQP 5672| SR
    SC --> SN
    SN --> SM
    SR --> SN
```

中心服务器只有一台，负责中心库写入和数据下发。每台边缘节点有自己的 MySQL、Canal 和本地 RabbitMQ；网络中断时先把事件保存在边缘队列，恢复后继续上传。

调试电脑不需要安装 NodeBridge。它只需要 SSH 客户端、自己的私钥和支持 stdio MCP 的 AI 客户端。

## 2. 地址、名称和端口规划

正式部署前先固定以下清单：

| 项目 | 示例 | 要求 |
| --- | --- | --- |
| 中心节点 ID | `server-001` | 全系统唯一，投入运行后不要随意修改 |
| 中心地址 | `192.168.10.10` | 推荐固定 IP 或 DHCP 保留地址 |
| 边缘节点 ID | `edge-001` 至 `edge-N` | 每台唯一，建议编号与现场台账一致 |
| 边缘地址 | `192.168.10.101` 起 | SSH 管理使用；可使用固定 IP |
| Windows 运行账户 | `nodebridge` 或现场账户 | 安装、UI、SSH MCP 使用同一账户 |
| RabbitMQ vhost | `/nodebridge-server`、`/nodebridge-edge` | URL 中写成 `%2Fnodebridge-server` 和 `%2Fnodebridge-edge` |

建议防火墙策略：

| 端口 | 开放位置 | 来源 | 用途 |
| ---: | --- | --- | --- |
| 22/TCP | 中心和所有边缘 | 调试电脑 IP 或可信本地子网 | SSH 与 MCP stdio |
| 5672/TCP | 中心服务器 | 所有边缘节点 IP | 边缘上传和下发队列 |
| 3306/TCP | 仅数据库跨机时 | 明确的 NodeBridge 节点 | MySQL；本机数据库无需对外开放 |
| 11111/TCP | 通常只允许本机 | 本机 NodeBridge | Canal |
| 15672/TCP | 默认只允许本机或管理网 | 管理员 | RabbitMQ 管理界面 |
| 18080/TCP | 默认关闭 | 诊断电脑 | 可选日志 Web，必须配置 token |

## 3. 安装前准备

1. 在中心和所有边缘电脑安装并配置 MySQL。NodeBridge 安装包不包含 MySQL。
2. 每台 MySQL 设置唯一 `server_id`，启用 `log_bin`，使用 `binlog_format=ROW` 和 `binlog_row_image=FULL`。
3. Canal 读取账户至少需要业务表查询权限以及 MySQL replication client/slave 权限。
4. NodeBridge Apply 账户需要目标业务表写权限，并需要创建或访问 NodeBridge 系统表。
5. 记录中心和各边缘的数据库名、用户名、密码及目标表主键。
6. 先备份现场数据库，再开始同步规则测试。

安装包自带 Erlang、RabbitMQ、Java、Canal 和 WinSW。安装器只应管理带 NodeBridge 标记的服务、vhost、用户和配置目录；外部 RabbitMQ 或 Canal 应在配置中选择 `external`。

## 4. 推荐安装顺序

1. 安装中心服务器，配置 `server-001`，确认中心 MySQL 和 RabbitMQ 正常。
2. 创建同步规则，但先不要批量启动全部边缘。
3. 安装 `edge-001`，完成一台边缘的端到端验证。
4. 验证新增、修改、删除、断网积压和恢复。
5. 复制经过验证的配置结构到其他边缘，但必须修改节点 ID、名称、位置、数据库和 RabbitMQ 账号。
6. 每增加一台边缘就单独验收，再继续下一台。

请用最终运行 NodeBridge 的 Windows 账户登录，然后右键安装包选择“以管理员身份运行”。安装后主要路径为：

```text
程序：C:\Program Files\NodeBridge\app
配置：C:\ProgramData\NodeBridge\config.yaml
规则：C:\ProgramData\NodeBridge\sync-rules.yaml
日志：C:\ProgramData\NodeBridge\logs
安装诊断：C:\ProgramData\NodeBridgeInstallerLogs
```

不要把一台电脑上已由 DPAPI 加密的 `config.yaml` 直接复制给另一台电脑或另一个 Windows 账户。可以复制脱敏模板，再在目标电脑重新输入密码。

## 5. 中心服务器配置

中心服务器使用：

```yaml
mode: server
node:
  id: server-001
  name: 中心服务器

mysql:
  host: 127.0.0.1
  port: 3306
  username: sync_user
  password: 现场密码
  database: scada_center

rabbitmq:
  mode: managed
  install: true
  server_url: amqp://nb-server-sync:现场密码@127.0.0.1:5672/%2Fnodebridge-server

cdc:
  type: canal
  mode: managed
  install: true
  reader_name: server-001
  canal_addr: 127.0.0.1:11111
  destination: server-001
  filter: scada_center\..*
```

使用 MCP 时，先调用 `nodebridge_config_summary` 读取当前脱敏配置，然后用以下结构预检：

```json
{
  "patch": {
    "mode": "server",
    "node": {"id": "server-001", "name": "中心服务器", "location": "中心机房"},
    "mysql": {
      "host": "127.0.0.1", "port": 3306,
      "username": "sync_user", "password": "现场密码", "database": "scada_center"
    },
    "rabbitmq": {
      "mode": "managed", "install": true,
      "server_url": "amqp://nb-server-sync:现场密码@127.0.0.1:5672/%2Fnodebridge-server"
    },
    "cdc": {
      "type": "canal", "mode": "managed", "install": true,
      "reader_name": "server-001", "canal_addr": "127.0.0.1:11111",
      "destination": "server-001", "filter": "scada_center\\..*"
    }
  }
}
```

调用顺序：

1. `nodebridge_validate_config_patch`
2. `nodebridge_save_config_patch`
3. `nodebridge_test_mysql`
4. `nodebridge_test_rabbitmq`
5. `nodebridge_mysql_schema`
6. `nodebridge_save_sync_rules`
7. `nodebridge_apply_managed_install`
8. `nodebridge_start_agent` 或 `nodebridge_restart_agent`
9. `nodebridge_overview`、`nodebridge_queue_status`、`nodebridge_logs`

保存配置或规则不会自动重启已经运行的同步进程。修改运行参数后必须调用重启。

## 6. 每台边缘服务器配置

每台边缘必须修改以下字段：

```yaml
mode: edge
node:
  id: edge-002
  name: 二号产线
  location: 大连工厂-二车间

mysql:
  host: 127.0.0.1
  port: 3306
  username: sync_user
  password: 现场密码
  database: scada_edge_002

rabbitmq:
  mode: managed
  install: true
  local_url: amqp://nb-edge-002-local:本地密码@127.0.0.1:5672/%2Fnodebridge-edge
  server_url: amqp://nb-edge-002:中心分配密码@192.168.10.10:5672/%2Fnodebridge-server

cdc:
  type: canal
  mode: managed
  install: true
  reader_name: edge-002
  canal_addr: 127.0.0.1:11111
  destination: edge-002
  filter: scada_edge_002\..*
```

MCP patch 示例：

```json
{
  "patch": {
    "mode": "edge",
    "node": {"id": "edge-002", "name": "二号产线", "location": "二车间"},
    "mysql": {
      "host": "127.0.0.1", "port": 3306,
      "username": "sync_user", "password": "现场密码", "database": "scada_edge_002"
    },
    "rabbitmq": {
      "local_url": "amqp://nb-edge-002-local:本地密码@127.0.0.1:5672/%2Fnodebridge-edge",
      "server_url": "amqp://nb-edge-002:中心分配密码@192.168.10.10:5672/%2Fnodebridge-server"
    },
    "cdc": {
      "reader_name": "edge-002", "destination": "edge-002",
      "filter": "scada_edge_002\\..*"
    }
  }
}
```

Beta 安装器默认只初始化 `edge-001` 的示例账号。部署 `edge-002` 及后续节点时，需要在中心 RabbitMQ 创建独立用户。以下命令在中心 RabbitMQ 所在电脑执行，密码必须替换：

```powershell
rabbitmqctl add_user nb-edge-002 '独立强密码'
rabbitmqctl set_permissions -p /nodebridge-server nb-edge-002 '^$' 'server\.ingress\..*' 'edge-002\.downlink\..*'
```

每个边缘独立账号可以单独撤销，也能限制它只写中心 ingress、只读自己的 downlink。不要让所有边缘共用 `nb-server-sync`。

## 7. 同步规则

| `direction` | 用途 | 推荐 `dispatch_target` |
| --- | --- | --- |
| `EDGE_TO_SERVER` | 采集历史、告警历史等只汇总到中心 | `NONE` |
| `SERVER_TO_EDGE` | 中心配置下发到边缘 | `ACTIVE_EDGES` 或 `SELECTED_EDGES` |
| `BIDIRECTIONAL` | 中心和边缘均可修改 | `ACTIVE_EDGES` 或 `SELECTED_EDGES` |
| `IGNORE` | 明确排除表 | 不分发 |

历史流水表可使用 `append_only`，仅接受 INSERT。需要保持增删改顺序的业务表使用 `crud_ordered`。启用 `crud_ordered_compact` 前必须确认只允许压缩同一主键的连续 UPDATE，并同时打开全局 compact 开关。

源表、目标表和列名不必相同，但必须明确填写 `target_database_name`、`target_table_name`、主键和列映射。第一轮只选择一张小表验证，确认无回环后再扩展。

## 8. MCP 与调试电脑授权

MCP 使用 SSH stdio，不开放 MCP HTTP 端口。控制关系是：

```text
调试电脑私钥 -> SSH 登录目标 Windows -> 启动目标 SyncAgent mcp-stdio
```

### 8.1 在每台受控 Windows 启用 SSH

管理员 PowerShell：

```powershell
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Start-Service sshd
Set-Service sshd -StartupType Automatic
```

只允许可信本地子网访问：

```powershell
New-NetFirewallRule `
  -Name 'NodeBridge-SSH-LocalSubnet' `
  -DisplayName 'NodeBridge SSH from local subnet' `
  -Direction Inbound -Action Allow -Protocol TCP -LocalPort 22 `
  -RemoteAddress LocalSubnet -Profile Any
```

如果现场有固定调试电脑地址，优先把 `RemoteAddress` 改为明确的 IP 列表。

### 8.2 Windows 调试电脑生成密钥

```powershell
ssh-keygen -t ed25519 -f "$env:USERPROFILE\.ssh\nodebridge_ed25519"
Get-Content "$env:USERPROFILE\.ssh\nodebridge_ed25519.pub"
```

### 8.3 Mac 调试电脑生成密钥

```bash
ssh-keygen -t ed25519 -f ~/.ssh/nodebridge_mac_ed25519
cat ~/.ssh/nodebridge_mac_ed25519.pub
chmod 600 ~/.ssh/nodebridge_mac_ed25519
```

每台电脑独立生成密钥，不要复制别人的私钥。`.pub` 公钥可以交给管理员；没有 `.pub` 后缀的私钥必须只留在生成它的电脑。

### 8.4 把公钥加入受控 Windows

当 SSH 登录账户属于本机 Administrators 组时，在受控电脑管理员 PowerShell 执行：

```powershell
$publicKey = '粘贴一整行 ssh-ed25519 公钥'
$keyFile = 'C:\ProgramData\ssh\administrators_authorized_keys'
Add-Content -LiteralPath $keyFile -Value $publicKey -Encoding ascii
icacls.exe $keyFile /inheritance:r /grant:r '*S-1-5-32-544:F' '*S-1-5-18:F'
```

每台调试电脑的公钥占一行。建议保留公钥末尾注释，例如 `alice-mac@company`，便于撤权。

非管理员 SSH 账户使用该账户自己的 `%USERPROFILE%\.ssh\authorized_keys`，不要写入 `administrators_authorized_keys`。

### 8.5 验证免密登录

Windows：

```powershell
ssh -i C:/Users/你的用户名/.ssh/nodebridge_ed25519 `
  -o IdentitiesOnly=yes -o BatchMode=yes `
  -l 目标用户名 目标IP whoami
```

Mac：

```bash
ssh -i ~/.ssh/nodebridge_mac_ed25519 \
  -o IdentitiesOnly=yes -o BatchMode=yes \
  目标用户名@目标IP whoami
```

第一次连接应去掉 `BatchMode=yes`，人工核对并接受服务器主机指纹。MCP 运行时不能等待密码或指纹确认。

### 8.6 为每个节点生成 MCP 配置

在目标 NodeBridge 电脑运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File 'C:\Program Files\NodeBridge\app\mcp-lab-client-config.ps1' `
  -SshHost '目标用户名@目标IP' `
  -ClientKeyPath '调试电脑上的私钥路径'
```

Windows 私钥示例：

```text
C:/Users/Alice/.ssh/nodebridge_ed25519
```

Mac 私钥示例：

```text
/Users/alice/.ssh/nodebridge_mac_ed25519
```

生成的 `nodebridge-mcp-lab.json` 中包含 `command: ssh` 和启动目标 `SyncAgent.exe mcp-stdio` 的参数，不包含私钥内容。

在 Codex 中为每个节点使用独立名称，例如：

```text
nodebridge-server-001
nodebridge-edge-001
nodebridge-edge-002
```

Codex CLI 注册形式为：

```text
codex mcp add <节点名称> -- ssh <生成 JSON 中的全部 args>
```

注册后使用 `codex mcp get <节点名称>` 检查，并重新打开 Codex 任务加载工具。

### 8.7 普通模式和实验室模式

`-lab-full-access` 用于受控内网调试，可以修改所有 NodeBridge 配置、密码、规则和运行状态。它仍执行字段校验、DPAPI 加密、输出脱敏和审计，不提供任意 shell 或通用 SQL。

生产运维建议去掉 `-lab-full-access`，并在 UI 中启用 MCP。普通模式要求配置已完整加载，并限制敏感配置写入。

审计日志：

```text
C:\ProgramData\NodeBridge\logs\mcp-audit.log
```

## 9. 给其他调试电脑授权和撤权

新增调试电脑时：

1. 在新电脑本机生成独立密钥对。
2. 把新电脑的 `.pub` 公钥追加到需要控制的每台 NodeBridge 电脑。
3. 从新电脑逐台执行 `ssh ... whoami`，核对主机指纹。
4. 为每个节点生成对应 MCP 配置并注册独立名称。

撤销某台调试电脑时，在每台受控 Windows 的以下文件中删除该电脑对应的完整公钥行：

```text
C:\ProgramData\ssh\administrators_authorized_keys
```

删除公钥立即阻止新的 SSH 登录。随后从调试电脑移除 MCP：

```text
codex mcp remove <节点名称>
```

私钥丢失时必须立刻撤销对应公钥并重新生成，不要让多名人员长期共用同一私钥。

## 10. 网络环境变化

SSH 公钥与 IP 无关，换网络后仍有效。需要更新的是连接地址和防火墙范围：

1. 为中心和边缘重新分配固定 IP 或 DHCP 保留地址。
2. 更新所有边缘配置中的 `rabbitmq.server_url`，使其指向中心新地址。
3. 更新调试电脑的 MCP `ssh-host`，重新注册对应节点。
4. 首次连接新地址时核对 SSH 主机指纹。
5. 检查中心 5672/TCP 和所有节点 22/TCP。

如果 SSH 防火墙使用 `RemoteAddress LocalSubnet`，切换到新的本地子网后通常无需修改规则。如果规则限制为某个旧 IP，需要更新为新的调试电脑 IP。

主机名可以减少 IP 调整，但代理软件的 Fake-IP、DNS 劫持或跨网段解析可能使 SSH 指向错误地址。现场没有可靠 DNS 时优先使用固定 IP。

## 11. 上线与验收顺序

每个节点按以下顺序验收：

1. `nodebridge_config_summary`：确认角色、节点 ID 和路径。
2. `nodebridge_test_mysql`：必须成功。
3. `nodebridge_test_rabbitmq`：本地与中心连接必须成功。
4. `nodebridge_mysql_schema`：确认源表、目标表、列和主键。
5. `nodebridge_save_sync_rules`：先保存一条测试规则。
6. `nodebridge_restart_agent`：应用新配置。
7. `nodebridge_overview`：MySQL、RabbitMQ、CDC 状态无错误。
8. `nodebridge_queue_status`：观察 ingress、upload 和 downlink。
9. 写入一条测试数据，核对目标库。
10. 断开边缘到中心网络，写入数据，再恢复网络并确认队列清空。
11. 查看 `nodebridge_failed_events`、`nodebridge_dead_letters` 和日志。

全部通过后再增加规则或接入下一台边缘。

## 12. 常见错误

| 错误 | 含义 | 处理 |
| --- | --- | --- |
| `Connection refused` | 目标 22 端口无服务 | 安装并启动 `sshd`，检查监听 |
| `Connection timed out` | 网络或防火墙阻断 | 检查 IP、子网、网络隔离和入站规则 |
| `Host key verification failed` | 未确认或主机指纹变化 | 人工连接并核对目标机指纹 |
| `Permission denied (publickey)` | 公钥位置、账户或 ACL 不正确 | 管理员账户检查 `administrators_authorized_keys` |
| `rename ... Access is denied` | 配置目录缺少替换文件权限 | 使用 v0.46.3，并给运行账户目录 `Modify` 权限 |
| MCP 显示未启用 | 普通模式开关关闭或配置不完整 | 在 UI 启用；实验室模式由连接参数控制 |
| MySQL 状态 `error` | 地址、账号、库或权限错误 | 保存配置后调用 `nodebridge_test_mysql` |
| RabbitMQ 403 | vhost 或用户权限不匹配 | 检查 `%2F` vhost、用户和 `set_permissions` |

## 13. v0.46.3 Beta 边界

- Windows x64 优先，Linux Server 尚未作为正式发行目标。
- MySQL 不随安装包提供。
- 安装包内 RabbitMQ bootstrap 默认以 `edge-001` 为示例；N 边缘账号需按本手册创建。
- `-lab-full-access` 面向受控测试，不应用作长期共享管理入口。
- 配置和规则支持 MCP 管理，但 UI 与 MCP 同时编辑时应由操作员协调，远程保存后重新打开 UI。
- 发布前测试包括 Go、Wails/前端、MCP 协议、安装器回归和离线资产校验；实际现场仍需按本手册逐节点验收。
