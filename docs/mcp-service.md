# NodeBridge MCP v0.46.16

本版供 Windows 内网测试。另一台 Windows 或 Mac 上的 AI 客户端通过 SSH 启动被测机的 `SyncAgent.exe mcp-stdio`。MCP 不监听 HTTP 端口，SSH 提供加密和登录身份。

一台中心服务器、N 台边缘服务器以及 Windows/Mac 调试电脑的完整部署和授权步骤见[内网部署与远程管理手册](lan-deployment-guide.md)。

## 实验室全权限模式

### v0.46.13 运行日志

`nodebridge_logs` 支持 `{"limit":100}`（1–500，默认100），返回既有 `items` 字符串数组与 `log_path`。未指定 CLI `-log` 时，自动发现配置目录下 `logs/sync-runtime.jsonl`，包含最近4份轮转；没有运行日志时兼容 `logs/sync-agent.log`。显式 `-log` 仍只读取用户指定文件。日志资源 `nodebridge://logs` 复用该读取路径。

运行日志记录真实时间、ERROR/WARN、worker、event_id、节点/版本/PID、重试、恢复与阶段耗时；慢步骤执行中也会采样。日志在写盘前脱敏配置凭据、URL/DSN凭据和引号中的SQL值，每文件8MiB、最多4份备份。MCP响应再次去除当前配置秘密。日志不是业务Apply账本，也不是已经通过性能门禁的证明。

`nodebridge_diagnostic_summary` 的 `log_path` 指向当前有效读取文件，新增 `runtime_log_path` 表示结构化日志位置；工具数量与鉴权规则不变。

启动参数 `-lab-full-access` 临时解除 MCP enable 开关、完整初始配置、管理解锁以及非敏感字段白名单限制。可以首次创建配置，修改 MySQL/RabbitMQ/Canal 密码、管理/退出密码、日志 token、MCP 开关及所有同步配置字段。

引导模式只作用于带此参数的 MCP 进程，不改变 Wails UI 的鉴权。普通模式必须启用 MCP 且已有完整配置，开启后同样开放全部管理工具及结构化配置字段，不新增申请、审批或角色系统。引导模式运行时，UI 开关不能关闭该进程，需由客户端断开连接。

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
- `nodebridge_mysql_diagnostics`（v0.46.9候选，实验室模式）：只读返回连接和逐步查询耗时、buffer pool大小、四张系统表的估计行数/字节数/索引定义。可选 `event_tables` 最多8个表名，增加精确事件总数、失败数和非执行式EXPLAIN计划；默认不扫描事件。整体默认5秒、最大10秒，响应`partial/error/skipped`不是通过，未知计数不冒充0。只访问保存的数据库，不接受任意SQL，不返回业务payload、密码或原始SQL错误。

v0.46.9安装包同时携带`app/migrations`。已有中心库需要在升级验收时显式执行安装目录内的`SyncAgent.exe migrate -config <配置路径> -scope server`，幂等补齐`sync_event_log(table_name,status)`覆盖索引。迁移优先使用可执行文件旁的schema，不依赖开发仓库；重复执行核对索引定义，遇到冲突或不支持在线DDL时失败，不自动删除现有索引。诊断接口本身不会执行DDL。

普通及引导模式还提供：

- `nodebridge_mysql_query`：在已保存的 MySQL 数据库中按结构化列、过滤和排序读取业务表；默认最多 50 行，硬上限 200 行。
- `nodebridge_mysql_mutation_plan`、`nodebridge_mysql_mutation_apply`：受控执行 `INSERT` / `UPDATE` / `DELETE`。不接受原始 SQL；`UPDATE` / `DELETE` 必须有过滤条件，apply 必须传 `confirm:true` 和与事务内复核一致的 `expected_rows`，单次最多 100 行。
- `nodebridge_mysql_schema_change_plan`、`nodebridge_mysql_schema_change_apply`：只对已存在业务表执行 `ADD COLUMN` / `DROP COLUMN`。apply 必须传当前 `plan_id` 和 `confirm:true`；禁止建表、删表、修改列、索引和组合 ALTER，禁止删除主键或索引列。
- `nodebridge_start_agent`、`nodebridge_stop_agent`、`nodebridge_restart_agent`。
- `nodebridge_retry_failed_event`、`nodebridge_retry_failed_events`。
- `nodebridge_dead_letters`：预览后 requeue，不删除消息。
- `nodebridge_managed_install_plan`、`nodebridge_apply_managed_install`：受管配置和 RabbitMQ 拓扑初始化；不等于安装系统组件的 NSIS 流程。
- `nodebridge_ensure_server_edge_user`：仅中心节点使用；按给定 `node_id` 创建或迁移该边缘节点连接中心 RabbitMQ 的账号。
- `nodebridge_export_diagnostics`：在被测机生成 ZIP，返回路径后可用 SCP 取回。
- `nodebridge_get_autostart`、`nodebridge_set_autostart`：当前 Windows 账户的 UI 自启动。

数据库治理默认使用 `mysql.database`；也可通过已保存的 `rule_id` 和 `side: source|target` 选择该规则引用的本机库表，不能指定任意库。表名/列名必须是安全 identifier，`sync_*` 内部表禁止通过治理写工具修改。写入和拒绝结果记录到 `logs/mcp-audit.log`，不记录密码。

新增 `nodebridge_capabilities`、`nodebridge_rule_preflight`、`nodebridge_event_status`、`nodebridge_queue_event_plan/apply/audit`，合计39工具及5资源。规则保存现需已有文件的 `expected_revision`，从 `nodebridge_sync_rules.saved_revision` 获取；保存后核对 `active_revision`，不视为热加载。完整当前使用步骤、删除/方向限制与升级注意事项见 [业务 AI 交接](mcp-business-ai-handoff.md)。

Canal二进制列按ISO-8859-1还原字节，SyncEvent行值以 `{"$nodebridge_binary_base64":"..."}` 持久化/传输，接收端恢复字节绑定SQL；NULL与空二进制区分。相关中心/边缘必须同包升级，旧版不支持该表示。依据：[Canal官方转换代码](https://github.com/alibaba/canal/blob/master/parse/src/main/java/com/alibaba/otter/canal/parse/inbound/mysql/dbsync/LogEventConvert.java)。

## 自动列同步

Canal 只把单列 `ALTER TABLE ... ADD COLUMN` 和 `ALTER TABLE ... DROP COLUMN` 转成 `SyncEvent`。规则通过 `schema_sync.add_columns`、`schema_sync.drop_columns` 分别授权；默认均为 `false`。列名遵循 `include_columns`、`exclude_columns` 和 `column_mappings`。

NodeBridge 不自动执行 `CREATE TABLE`、`DROP TABLE`、`MODIFY/CHANGE COLUMN`、索引或复合 ALTER。源表、中心目标表和边缘目标表必须预先存在。`DROP COLUMN` 除 `SERVER_TO_EDGE` 外必须恰好配置一个 `source_node_id`，防止同名多边缘表互相触发破坏性结构变化。

本机三节点回归：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/lab-governance-schema-e2e.ps1
```

脚本使用 Edge A `3307/5673`、模拟中心 `3309/5675`、模拟 Edge B `3308/5674` 和临时 Canal `11121`，通过真实 MCP stdio 执行数据与列治理，再验证完整 `Edge A -> Server -> Edge B` 链路。它不会使用或停止现有 `3306/5672/11111` 服务。

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
