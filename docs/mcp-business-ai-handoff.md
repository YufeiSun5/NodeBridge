# NodeBridge 0.46.16：安装与业务 AI MCP 交接

本说明可直接交给负责业务配置的 AI。中心与边缘使用同一个 Windows x64 安装包，角色由每台机器的配置决定。当前为内网测试版，不是七小时性能验收通过版。

## 1. 本次部署边界

**0.46.16新增半自动组件选择页，默认“复用现有组件（Docker / 已有服务）”。** 本机Docker中心保持默认即可，已有组件的边缘升级也可选择复用；只有要安装Windows原生组件的全新环境才选“安装本机系统组件”。不自动识别Docker或测试连接。停止旧工作区Agent、备份配置后，双击新版EXE；也可显式指定：

```powershell
& 'D:\DEV_D\NodeBridge\build\NodeBridge-beta-v0.46.16-20260911.exe' /SkipSystemComponents
```

复用模式跳过Erlang/RabbitMQ/Java/Canal安装、受管密码迁移和组件拓扑配置，保留已有配置/规则原文及密码；首次安装则生成external配置。不会把已有managed配置偷偷改成external。静默安装默认也是复用；显式安装组件用 `/InstallSystemComponents`，两参数不能同时使用。MySQL不由安装器安装。

- 中心：用户当前电脑，暂任主服务器；现有实验室地址 `192.168.10.103`，数据库 `scada_center`。
- 边缘：当前实验室 `192.168.10.105`，数据库 `scada_edge`。部署前核实地址、数据库和业务用途，不将实验室名称直接套用到新业务。
- 默认程序：`C:\Program Files\NodeBridge\app\NodeBridge.exe` 与 `SyncAgent.exe`。
- 默认配置：`C:\ProgramData\NodeBridge\config.yaml`；规则：同目录 `sync-rules.yaml`。
- 用户自行安装。本说明不授权 AI 自动安装、停业务、清队列、改业务数据或启动压力测试。

**中心安装前必须停止旧工作区 Agent。** 当前中心曾从开发目录 `build\nsis-beta\...\app\SyncAgent.exe` 运行。安装器只处理所选安装目录中的进程，不会替你停止工作区副本。由操作员在原管理入口停止同步、显式退出旧 UI，确认该副本已退出后再安装；不能按进程名批量强杀。

先备份本机配置、规则和安装清单。升级保留配置/规则，新装生成不完整配置和空规则，不会凭空具备业务同步能力。配置有 Windows DPAPI 加密，不能直接跨机器或跨账户复制密文。

中心现有 Docker MySQL/RabbitMQ/Canal 必须按现状复用为 external，不创建冲突的本机受管服务，不卸载或修改外部组件权限。MySQL 不由本包安装。边缘已有受管组件按原清单复用。安装前确认 `rabbitmq.mode`、`cdc.mode` 及端口归属。

试用版首次安装/受管配置存在 `1234` 默认管理、退出及受管 RabbitMQ 密码约定；选择安装系统组件时仍执行相关迁移，复用升级不改已有密码。不得作为公网生产部署。实际连接凭据由用户另行安全提供，交接文件不携带实验室密码清单。

## 2. MCP 是什么连接

NodeBridge 实现 JSON-RPC 2.0、换行分隔的 **stdio MCP**。客户端启动一个 `SyncAgent.exe mcp-stdio` 子进程；这不等于启动同步 Agent。没有 MCP HTTP/SSE URL、端口或 HTTP token。远程连接是 SSH 转发该进程的 stdin/stdout。

服务支持协议版本 `2024-11-05`、`2025-03-26`、`2025-06-18`、`2025-11-25`。客户端按 initialize 返回值协商，顺序为：

1. `initialize`，带 `protocolVersion`、`capabilities`、`clientInfo`。
2. 确认 `serverInfo.name=nodebridge`、`serverInfo.version=0.46.16`。
3. 发无 id 的 `notifications/initialized`。
4. `tools/list`、`resources/list`，再按发现结果执行 `tools/call`、`resources/read`；`ping` 可用。

只声明已实现的 `tools` 和 `resources`。本版不提供 prompts、资源订阅、HTTP/OAuth、任意 shell 或原始 SQL 执行。它们不是每个 MCP 服务都必须实现的能力，不要调用未声明的方法。工具结果为标准 `content` 中的 JSON 文本，须解析 `content[].text`，不能假设存在 `structuredContent`。同时检查协议 `error`、工具 `isError` 及内容中的 `ok/status`；`partial/error/locked` 不能算成功。

标准参考：[生命周期](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)、[工具](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)。

## 3. 本机连接与远程连接

### 普通模式

先在 UI 中保存完整配置并开启 MCP。使用当前配置所属 Windows 账户运行客户端：

```json
{
  "mcpServers": {
    "nodebridge-server": {
      "command": "C:\\Program Files\\NodeBridge\\app\\SyncAgent.exe",
      "args": ["mcp-stdio", "-config", "C:\\ProgramData\\NodeBridge\\config.yaml", "-rules", "C:\\ProgramData\\NodeBridge\\sync-rules.yaml"]
    }
  }
}
```

`mcpServers` 是常见客户端配置形式，不是 MCP 线协议；客户端格式不同则转换相同 command/args。每个节点单独一个服务条目，不把多个节点配置指向同一文件。

### 经明确授权的业务配置模式

MCP 开启且配置完整时，普通模式已开放全部管理工具，不需要新增申请或审批。`-lab-full-access` 只用于初次配置/缺配置引导，将它加入上面的 args；该模式绕过 MCP 开关和初始完整配置要求，但保留配置校验、DPAPI、输出脱敏、写操作审计和治理约束。UI 关闭 MCP **不能关闭这种进程**，结束时须由客户端断开。只给受信任 AI/用户使用，不默认永久授权。

### 边缘 SSH

被控 Windows 需 OpenSSH Server、公钥登录，账户须能读取自己所属配置。先手动连接并核对主机指纹，不能关闭 host key 校验。生成命令采用非交互 BatchMode，不能在 MCP 内输入 SSH 密码。

一台控制电脑可把同一份公钥加入多台被控机对应账户的授权公钥列表，私钥只留控制电脑，不复制给被控机。多台控制电脑建议各自独立密钥。SSH 需网络地址/端口可达，本安装包不部署 VPN、内网穿透或额外 MCP 监听。

在已经安装的中心电脑 PowerShell 中，生成远程条目（以下用户名沿用当前实验室，密钥路径属于运行 AI 的电脑）：

```powershell
& 'C:\Program Files\NodeBridge\app\SyncAgent.exe' mcp-client-config `
  -name nodebridge-edge-001 `
  -exe 'C:\Program Files\NodeBridge\app\SyncAgent.exe' `
  -config 'C:\ProgramData\NodeBridge\config.yaml' `
  -rules 'C:\ProgramData\NodeBridge\sync-rules.yaml' `
  -ssh-host 'xx@192.168.10.105' `
  -ssh-key 'C:\Users\SunYufei\.ssh\nodebridge_ed25519'
```

输出 JSON 中的 `nodebridge-edge-001` 与本机 `nodebridge-server` 合并到同一 `mcpServers`。此生成命令不连接或修改边缘。需要授权配置模式时为生成命令加 `-lab-full-access`。非默认 SSH 端口用 `-ssh-port`；AI 在其他电脑运行时改用户名、地址、客户端密钥路径，程序与配置路径则必须属于被控机。不要手工拼接 Windows SSH 引号，使用生成器。

若 AI 不在中心电脑，也用相同生成器为中心生成 SSH 条目并命名 `nodebridge-server`。私钥内容不得发给 AI 或写进 JSON。

## 4. 业务 AI 的执行顺序

1. 每端握手并发现工具；读取 `nodebridge_config_summary`、`nodebridge_sync_rules`、`nodebridge_agent_status`、`nodebridge_diagnostic_summary`。记录实际角色、唯一 node.id、库、路径和版本；先解决旧版 Agent 混跑问题。
2. 向用户确认源节点/库/表、目标节点/库/表、主键、同步方向、列映射、是否向其他边缘转发，以及已有数据如何初始化。不要自行推断同名表列，不把 CDC 当作已完成历史全量导入。
3. 配置修改先 `nodebridge_validate_config_patch`，成功后以同一 patch 调 `nodebridge_save_config_patch`；未传字段保留。不要把脱敏返回值当成真实凭据。保存时 `restart_agent:true` 仅在明确授权后使用。
4. 使用 `nodebridge_test_mysql`、`nodebridge_test_rabbitmq` 检查已保存连接；从 `tools/list` 获取每个接口当前的必填项及枚举。受管拓扑按 plan/apply；中心为边缘创建账号用 `nodebridge_ensure_server_edge_user`，不要修改 external broker 的归属约定。
5. 两端分别用 `nodebridge_mysql_schema` 检查真实表、列、类型和主键。可传已保存的 `rule_id` 和 `side: source|target`，选择该规则引用的本机库表；未传时使用本机 `mysql.database`，不是任意数据库浏览器。业务表须预先存在。
6. 对已有库，先安排 DBA 备份并审阅系统迁移，再在各自机器使用安装版 `SyncAgent.exe migrate -config <路径> -scope server` 或 `-scope edge`；schema 位于安装目录 `app\migrations`。MCP 没有任意 SQL/migrate 工具，不得发明一个。
7. 先读取全部规则及 `saved_revision`，合并本次变更，再以 `expected_revision: saved_revision` 调用 `nodebridge_save_sync_rules`。出现 `revision_conflict` 必须重读合并，不盲目覆盖。它保存整个 `rules` 数组，不是局部追加；`[]` 会清空全部规则。保留无关规则，避免 UI 与 AI 并发编辑。保存后重新读取核对；`saved_revision` 与 `active_revision` 不同表示需要明确重启以激活，不等于当前运行已换规则。先用 `nodebridge_rule_preflight` 检查两端对应本机结构/权限；预检不证明存量一致或远端结构兼容。
8. 仅在配置、表结构、迁移、规则及用户授权齐全后启动/重启 Agent。以 `agent_status`、连接、真实 `queue_status`、失败事件及日志联合判断，不以 overview 的 0 队列作为唯一证据。
9. 经用户授权，用隔离测试键做 INSERT/UPDATE/DELETE 和需要的边缘间转发；核对全列、目标主键、重复回放和最终收敛。不得修改既有业务行来试通，不自动清失败/死信或压力造数。记录测试范围，不将短测称为长期验收。

## 5. 同步规则与回环约束

- 先读取返回规则和工具 inputSchema；字段参考随包 `mcp-service.md`、`lan-deployment-guide.md`。
- `primary_keys`、include/exclude 使用源列名；表映射为 `target_database_name` / `target_table_name`，列映射为 `column_mappings`，目标主键须与映射一致。
- 多边缘同名源表必须用 `source_node_ids` 区分；每个节点有唯一 `node.id`。
- 仅汇总中心：`EDGE_TO_SERVER` 配合 `dispatch_target: NONE`。中心继续发其他边缘：按业务明确授权设置 `ACTIVE_EDGES` 或 `SELECTED_EDGES`。不要凭“上传”推断必须广播。
- Server 分发跳过 `origin_node_id`；Apply 日志和 `last_event_id` / `updated_by_node` 用于抑制 CDC 回放。不得把这些字段排除或错误映射。多向表需 `sync_version`、`updated_by_node`、`last_event_id`、`updated_at`，系统 `sync_apply_log` 必须存在。
- `append_only` 仅适合 INSERT-only 业务；不要为提速套到有 UPDATE/DELETE 的表。默认不启用未讨论的 compact/DDL 功能。
- DDL 默认关闭；仅支持受控单列 ADD/DROP，需要规则分别授权。CREATE/DROP TABLE、MODIFY COLUMN、索引及复合 ALTER 不自动同步。

## 6. 工具和资源速查

普通模式与引导模式均提供 39 个管理工具（含旧 33 个），不增加角色/申请/审批。新增 `nodebridge_capabilities`、`nodebridge_rule_preflight`、`nodebridge_event_status`、`nodebridge_queue_event_plan`、`nodebridge_queue_event_apply`、`nodebridge_queue_event_audit`。完整名称及参数以每次 `tools/list` 为准，不凭旧版数量推断可用性。

队列隔离必须先停止对应 Agent、确保队列无消费者，再按规则和事件 plan/confirm/apply；有界扫描最多 100 条/2 MiB，保留原消息的持久副本后才 ACK。它不是业务应用成功，不是 purge，也不能用来掩盖失败或处理其他 AI 的残留。

治理工具不接受原始 SQL，不能写 `sync_*` 内部表。数据 apply 需 `confirm:true` 和精确 `expected_rows`，UPDATE/DELETE 必须过滤，最多100行；结构 apply 需当前 plan_id 和确认，不能创建/删除表。plan 不是用户已授权执行。

资源 URI：`nodebridge://overview`、`nodebridge://config`、`nodebridge://sync-rules`、`nodebridge://diagnostic-summary`、`nodebridge://logs`。

日志工具 `nodebridge_logs` 可传 `{"limit":100}`，上限500，读取最近最多1MiB。默认读取配置目录 `logs\sync-runtime.jsonl` 及轮转，兼容旧日志。审计在 `logs\mcp-audit.log`。错误先检查日志、失败事件、实际队列和连接，不删除队列/账本掩盖错误。

## 7. 本次版本已知边界

0.46.16 包含严格 INSERT、缺行 UPDATE 拒绝、HARD/SOFT 删除、真实主键检查、规则预检/CAS/运行版本、事件状态、受控队列隔离、MCP 数字及可信时间脱敏修复；保留半自动组件选择与复用时配置原文。

启用规则只支持 `conflict_policy: NONE`，不支持的 `BIDIRECTIONAL`、`SERVER_WIN`、`LAST_WRITE_WIN` 会明确拒绝，不能因旧版曾接受就认为已实现。旧规则原文保留，升级前检查；不要为了通过校验擅自改变业务方向。未写 `delete_mode` 的旧规则仍按 SOFT，需要对应软删列；HARD 仅响应源 DELETE，不清理目标多余行。

中心和所有关联边缘应停同步后升级同一包，再按规则核验后启动。二进制列在事件行值中采用 `{"$nodebridge_binary_base64":"..."}` 保留字节，普通文本/数字/NULL不变；旧节点不支持此格式。历史已经错写的二进制数据和旧残留不自动修复。

首次对齐执行、在线交接、冲突仲裁及历史误账修复引擎未交付；MANUAL选项不表示可执行，也不会自动执行。七小时压力与实际升级后 UI 自动恢复仍待安装后现场验收，本包不自动部署或启动长测。

English: Install the same build on server and edges. Discover the standard MCP tools/resources over local or SSH stdio; obtain explicit authorization before configuration or business writes. Full-duration performance acceptance remains open.

日本語: サーバーとエッジには同じビルドを使用します。ローカルまたは SSH stdio で MCP を接続し、設定・業務データ変更前に承認を得てください。長時間性能検証は未完了です。
