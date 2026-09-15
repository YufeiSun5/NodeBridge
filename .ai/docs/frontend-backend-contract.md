# Frontend Backend Contract

2026-09-15 NB-RECONNECT：运行状态WorkerStatus新增consecutive_errors，连续失败递增、成功处理或空闲探测成功归零；last_processed_at仍只记录实际处理成功，不以进程running/重连成功替代。RabbitMQ失败日志区分transport reset/reconnect failed及worker recovered，复用现有错误诊断与队列指标，不新增Wails方法或业务规则字段。

2026-09-14 0.48.5：SyncRule新增可选name（显示名称）；UI列表/详情优先名称，未填回退id，已保存id只读。名称随YAML及现有DTO保存，CAS仍覆盖完整文件；名称不参与对齐规则语义/计划哈希或Agent运行语义版本，因此修改名称不要求重启。旧无name规则和既有对齐证明兼容，不开放改ID/删证明。安装器两种组件模式均调用upgrade-system，使用现有配置角色与MySQL系统库；记录sync_schema_migration校验值和sync_schema_version，失败不恢复UI/报告成功，初次未配置明确跳过。不新增业务表列。

2026-09-14 0.48.4：双向下行尊重已编译规则的目标业务库，系统连接默认库不再覆盖反向/中继映射；逗号分隔 Canal 过滤追加独立系统表表达式。无新增 Wails/MCP 方法、DTO、规则字段或迁移，不要求修改业务表。沿用 0.48.3 小屏规则工作区。

2026-09-14 0.48.2：启动只检查当前启用规则涉及的本地物理源/目标表的对齐状态，无关禁用表的失败作业不再阻断；同表未完成对齐仍阻断，方向/规则ID变更不能绕过。StartAgent/MCP start仍返回OperationResult，但成功须等对应PID发布ready且短暂稳定；提前退出/超时返回ok=false、error及本次启动诊断。AgentProcessStatus可为starting，running只说明启动确认及进程存活，不等于全链路业务验收。无新方法或用户参数。

2026-09-14 0.48.1：双向首次对齐不再要求业务表含last_event_id/updated_by_node，业务同名列按原值映射，不覆盖。回放证据转为NodeBridge系统库010迁移的sync_replay_marker/sync_replay_position。Wails/MCP方法、DTO与用户参数不变，两端须同版本并完成系统迁移；缺标记订阅时捕获屏障失败，不静默放行。未改变时钟/LWW策略，不宣称现场时差问题已解决。

2026-09-12 0.48.0多端/MCP：`StartInitialAlignment`保留`rule_id/peer_node_id/confirm`；Server的peer可为逗号分隔全部参与Edge，Edge仍唯一Server。既有Rules页提供中心多选和三语，中心按选定目标表匹配每个Edge唯一保存规则；异名映射用独立rule_id和source_node_ids，同名结构可共享规则。新增MCP start/status/interrupt三个工具，重试复用start；普通stdio42工具5资源，未新增审批。running只表示受理，completed才表示完整组就绪；客户端须保持当前stdio并轮询，断线未知结果需重连核验。原始256MiB/编码512MiB/每复制15分钟/组1小时；中心源与边缘源各200MiB真实三节点短测通过。Canal需足够未ACK缓存与堆（受管默认512MiB/2GiB，外部和复用组件须另外核验重启）；不支持在线加成员或将既有冲突历史复制给新成员。以下0.47的两节点/64MiB/无MCP表述是历史记录，由本条覆盖。

本文件定义 NodeBridge Wails UI 的前后端契约。前端只调用 Wails binding，不直接访问 SyncAgent HTTP API、RabbitMQ、MySQL 或配置文件。

## 通用规则

2026-09-12 FB-107接口：`StartInitialAlignment({rule_id,peer_node_id,confirm})`异步运行明确两节点的手动对齐/已提交结果恢复；两端均须显式执行且Agent已停，复用进程锁，不自动停止Agent或连接远端MySQL。`GetInitialAlignmentStatus()`返回rule_id/peer_node_id/running/stage/plan_id/rows/message/started_at/updated_at。stage为idle/waiting_peer/checking/copying/recovering/preparing_incremental/cancelling_uncommitted/completed/failed/stopping/unknown。`InterruptInitialAlignment()`只中断本窗口当前操作，不能撤销已提交业务数据或删除作业。完成表示双方持久交接已确认，仍由用户显式启动Agent；管理方法沿用requireAdmin。两端CLI到真实Agent的三种起点短测通过后，公开initial_alignment.execute和sync.bidirectional=true；online=false，不增加MCP操作工具。

2026-09-12 FB-107启动保护：非CANCELLED作业必须有核验通过的双方ACTIVE交接证明，否则Agent以`alignment_cutover_pending`拒绝启动。TARGET_COMMITTED/TARGET_CONFIRMED收据本身不够；禁止删除账本解除阻断。重试发现目标尚未提交时，先锁定并撤销目标，再核对撤销源，保留原计划/标记/摘要并返回明确重试提示；双方再次执行才重建计划。每次最多64MiB/两分钟，两端非空拒绝覆盖。已有映射固定，规则保存的Enable变化使用当前配置；启用双向必须匹配本机持久ACTIVE证明，仍执行CAS和本机预检。

2026-09-12 FB-107事件状态：`state`/`apply_status`的`superseded`表示LWW旧事件未覆盖获胜行，或旧消息已被首次全量基线取代并留档；后者没有Apply收据，applied_at为空并返回明确warning。`recorded_outcome_unknown`表示有Apply收据但仲裁结果查询失败。`applied_at`是收据提交时间，不证明当前行值。后续转发仍失败时state可为retrying而apply_status为superseded；前端用三语区分，不显示为已写入。

2026-09-11后续整改覆盖下方首批限制（源码，未出包）：`PreflightSyncRule({rule_id,side,source_schema?})` / `nodebridge_rule_preflight` 只检查所选本机源/目标，返回结构、findings、checked_permissions与unverified。新增启用或修改已启用规则必须通过本机职责端预检；禁用草稿可离线保存。`source_schema`仅用于目标侧比较，来源新鲜度明确未核验。

`SyncRulesDTO`新增saved_revision/active_revision/activation，activation为active/stopped/restart_required/unknown。SaveSyncRules和nodebridge_save_sync_rules替换已有文件时必须提供expected_revision；过期返回revision_conflict，缺失返回expected_revision_required。不热重载；Agent启动时公布实际加载规则revision。新建文件可无revision。

`GetEventStatus({event_id?,rule_id?,limit?})` / `nodebridge_event_status`返回有限运行错误观察与Apply账本，state区分retrying/retry_pending/applied/unknown/recovered_without_apply_receipt，apply_status单独记录。complete=false，不将空日志或账本存在当业务一致性证明。失败页旧列表仅为sync_ack_log FAILED。禁用/IGNORE规则的已入队事件现在返回rule_not_active，由消费者NACK策略保留，不再静默ACK。

`nodebridge_mysql_schema/query/mutation_plan/mutation_apply`新增可选rule_id+side（source/target），table须精确匹配该端规则引用；不改默认mysql.database，不开放任意SQL，系统表写保护保留。原无选择参数调用行为不变。MCP结构化参数统一UseNumber，数值主键不经float64。

`SyncRule.delete_mode` 为 `HARD | SOFT`。旧文件缺省按 SOFT 读取，下次显式保存写出 SOFT；读取不改写文件。新建 UI 规则默认显式 HARD，物理 DELETE 不要求软删字段，不推导删除目标额外行。源端软删 UPDATE 仍为 UPDATE。SOFT 的本机结构预检要求实际软删列及兼容类型，不以配置保存代替端到端运行验收。

普通单向INSERT不再UPSERT；1062返回`unique_key_conflict`，不覆盖已有主键。同一event_id按账本幂等；不同event_id的重复主键是冲突。UPDATE 0变更行时同事务锁定检查，缺失返回`target_row_missing`，同值行成功；异常多行回滚。键必须完整非NULL；可见的主键变化和映射碰撞拒绝。实际目标主键/schema在本机预检；已对齐LWW路径按源版本仲裁后允许恢复获胜镜像，不套用普通单向缺行策略。

普通单向启用规则支持conflict_policy NONE；已完成首次对齐的单Edge/单Server映射对支持BIDIRECTIONAL+LAST_WRITE_WIN，SERVER_WIN仍不支持。LWW依据源binlog秒级时间及确定性平局排序，不以接收/处理时间替代；源时钟回退时后发指令可能成为loser，必须保持源时钟可靠。严格业务指令时间字段另行约定。保存使用CAS及saved/active revision，保存成功不代表运行中已热生效。

MCP v0.46 实验室扩展：`SyncAgent.exe mcp-stdio -lab-full-access` 支持所有 `appconfig.Config` 字段的部分更新，包括密码/token/security/mcp_server，及全部 SyncRule 字段、自启动、真实诊断、重试和同步进程控制。实验室模式绕过 MCP 开关和管理解锁，不改变 Wails 方法鉴权；保留配置/规则校验、加密与脱敏。CLI `mcp-client-config -ssh-host user@ip -lab-full-access` 生成 Windows/Mac 客户端可用的 SSH stdio 配置。详见 `docs/mcp-service.md`。本版 Agent 状态支持跨 UI/MCP 进程识别；升级前须停止旧版同步进程。

- Wails 绑定对象：`datasyncui.App`；前端服务层可保留 `main.App` 作为旧入口 fallback，但必须优先使用 `window.go.datasyncui.App`。
- 错误：Go 方法可返回 `error`；前端必须显示为 error state。
- 空能力：未接真实能力时返回空数组、`unknown` 或 `unsupported`，不得返回假成功。
- 脱敏：后端返回配置时必须隐藏 MySQL/RabbitMQ/CDC/LogWeb 密码、token、管理密码和退出密码。
- 字段名：JSON 使用 snake_case，前端 DTO 必须保持一致。
- 关闭行为：Wails 原生关闭按钮隐藏窗口到 Windows 系统托盘并阻止退出；真正退出必须先调用 `RequestExit` 再调用 `runtime.Quit`。
- 托盘菜单由 Wails 后端桌面层创建：左键双击或右键“显示 NodeBridge”恢复窗口；右键“退出...”会恢复窗口并向前端发送 `datasync:request-exit`，由前端打开退出密码弹窗。
- 管理鉴权：启动默认 locked；敏感方法未解锁时返回 error 或 `status=locked`，前端必须显示解锁弹窗。

## Methods

| Method | Request | Response | Notes |
| --- | --- | --- | --- |
| `GetOverview()` | none | `status.Overview` | 模式、节点、配置加载状态、配置路径、规则路径、服务状态、队列积压、失败数、版本。未知字段返回 `unknown`。 |
| `GetConfig()` | none | `uiapi.ConfigDTO` | 返回脱敏配置。 |
| `SaveConfig(req)` | `uiapi.SaveConfigRequest` | `uiapi.ConfigDTO` | 校验配置并保存到 `%ProgramData%\NodeBridge\config.yaml`；redacted secret 自动沿用旧值；`security.admin_password` 不能为空。首次只设置安全密码时允许写入仅含 `security` 的加密草稿，但启动同步前仍必须补齐完整同步配置。 |
| `TestMySQL(req)` | `appconfig.MySQLConfig` | `uiapi.TestResult` | 使用 3 秒超时 ping MySQL；请求密码为 `******` 时沿用当前已保存密码。 |
| `TestRabbitMQ(req)` | `appconfig.RabbitMQConfig` | `uiapi.TestResult` | 分别测试本地和中心 URL；请求密码或 URL 凭据为 `******` 时沿用当前已保存值。 |
| `GetSyncRules()` | none | `uiapi.SyncRulesDTO` | 优先读取 `%ProgramData%\NodeBridge\sync-rules.yaml`，缺失时回退 example。 |
| `SaveSyncRules(req)` | `uiapi.SaveSyncRulesRequest` | `uiapi.SyncRulesDTO` | 校验 identifier、来源节点作用域、主键映射、列映射及删列来源约束后落盘。 |
| `StartInitialAlignment(req)` | `uiapi.InitialAlignmentRequest` | `uiapi.InitialAlignmentStatus` | 明确rule_id/peer_node_id/confirm，管理解锁；异步执行手动两端对齐。 |
| `GetInitialAlignmentStatus()` | none | `uiapi.InitialAlignmentStatus` | 本窗口状态或持久最后状态；重开后未终结操作显示unknown而非成功。 |
| `InterruptInitialAlignment()` | none | `uiapi.InitialAlignmentStatus` | 管理解锁；中断不撤销已提交数据。 |
| `GetNodeOptions()` | none | `uiapi.NodeOptionsResponse` | 只读返回 ACTIVE Edge 节点候选，用于 Rules 页 `dispatch_node_ids` 选择；配置缺失返回空列表和 `unknown`。 |
| `GetQueueStatus()` | none | `uiapi.QueueStatusResponse` | 使用 RabbitMQ passive declare 查询队列深度；无法连接时返回 error 状态。 |
| `GetFailedEvents(req)` | `uiapi.FailedEventsRequest` | `uiapi.FailedEventsResponse` | 从 SyncAgent MySQL 系统表查询失败 ACK。 |
| `RetryFailedEvent(req)` | `uiapi.RetryFailedEventRequest` | `uiapi.OperationResult` | 将失败 ACK 标记为 `PENDING`。 |
| `RetryFailedEvents(req)` | `uiapi.RetryFailedEventsRequest` | `uiapi.OperationResult` | 批量将失败 ACK 标记为 `PENDING`，按 limit 限制数量。 |
| `GetDeadLetters(req)` | `uiapi.DeadLetterRequest` | `uiapi.DeadLetterResponse` | 只读预览死信队列消息，读取后立即 requeue；需要管理解锁。 |
| `GetLogs(req)` | `uiapi.LogQuery` | `uiapi.LogsResponse` | 合并 runtime ring buffer 与持久化 sync-runtime.jsonl（含轮转），按真实时间倒序，支持 level/module/limit；旧 sync-agent.log 使用文件修改时间回退。DTO 不变。 |
| `UnlockAdmin(req)` | `uiapi.UnlockAdminRequest` | `uiapi.OperationResult` | 校验 `security.admin_password`，成功后 10 分钟内允许敏感操作；旧配置缺失 admin password 时兼容 fallback 到 exit password。 |
| `LockAdmin()` | none | `uiapi.OperationResult` | 立即锁定管理操作。 |
| `GetAuthState()` | none | `uiapi.AuthState` | 返回当前 locked/unlocked 状态和过期时间。 |
| `VerifyExitPassword(req)` | `uiapi.VerifyExitPasswordRequest` | `uiapi.OperationResult` | 前端退出托盘程序前调用；密码为空时允许退出。 |
| `RequestExit(req)` | `uiapi.VerifyExitPasswordRequest` | `uiapi.OperationResult` | 校验退出密码并放行下一次 Wails `Quit`；窗口关闭按钮默认隐藏到托盘，不直接退出。 |
| `GetAutoStart()` | none | `uiapi.AutoStartStatus` | 查询当前用户登录自启动状态。 |
| `SetAutoStart(req)` | `uiapi.SetAutoStartRequest` | `uiapi.AutoStartStatus` | 写入或删除当前用户 Run 启动项，不做 Windows Service。 |
| `GetMCPServerStatus()` | none | `uiapi.MCPServerStatus` | 查询持久 MCP Service 开关；默认关闭，启用后重启仍保持启用，只能由用户手动关闭。配置无法被 `SyncAgent.exe mcp-stdio` 加载时返回 `unsupported`。 |
| `SetMCPServerEnabled(req)` | `uiapi.SetMCPServerEnabledRequest` | `uiapi.MCPServerStatus` | 启用/关闭 stdio MCP Service 开关；写入 YAML，不占端口。启用前后端会严格校验配置文件可由 MCP stdio 加载，安全草稿或 DPAPI 解密失败会返回 `unsupported`。 |
| `GetManagedInstallPlan(req)` | `uiapi.ManagedInstallRequest` | `uiapi.ManagedInstallResponse` | 只读生成 RabbitMQ/Canal 受管组件计划，不执行安装动作。 |
| `ApplyManagedInstall(req)` | `uiapi.ManagedInstallRequest` | `uiapi.ManagedInstallResponse` | 执行 V0.31 alpha 受管组件动作；需要管理解锁。 |
| `ExportDiagnosticPackage()` | none | `uiapi.DiagnosticPackageResponse` | 导出脱敏配置、规则、状态、队列、失败事件和日志 zip。 |
| `GetAgentProcessStatus()` | none | `uiapi.AgentProcessStatus` | 返回外部 SyncAgent 可执行路径、PID、状态、启动/退出时间、最后错误和日志路径。 |
| `StartAgent()` | none | `uiapi.OperationResult` | 启动外部 `SyncAgent.exe` 执行 `run -config <config> -rules <rules> -stop-file <path>`；未找到可执行文件时返回 error。 |
| `StopAgent()` | none | `uiapi.OperationResult` | 写入 stop file 并等待最多 10 秒；超时后 fallback kill，返回 `stopped` 或 `forced_stopped`。 |
| `RestartAgent()` | none | `uiapi.OperationResult` | 先停止再启动外部 SyncAgent 进程。 |
| `GetCapabilities()` | none | `uiapi.CapabilitiesResponse` | 返回实际支持能力；未接通的首次对齐执行/在线模式必须supported=false，不以有选项代替已实现。 |

## MCP Access

远程连接沿用 SSH 承载原 stdio MCP。2026-09-11 按用户决定撤回未发布的 VPN/HTTP/token、远程专用设置与 SYSTEM 后台方案；不增加 MCP 监听端口或安装 SSH 服务。配置仍使用原账户 DPAPI，SSH 账户须可解密本机配置。

按用户2026-09-11纠偏，MCP启用后全部管理工具开放，不增加会话申请、审批、scope授权或审批界面。保留原MCP开关、凭据加密和返回脱敏、结构化数据治理确认与资源所有权检查；实验室隔离是测试操作边界，不是产品权限收紧。`-lab-full-access`仅兼容原缺配置/开关未启用的引导方式。

`nodebridge_capabilities`只声明实际实现能力。死信预览会重入队，readOnlyHint=false。AgentProcessStatus可信时间字段不做秘密子串替换；自由文本错误仍脱敏，JSON数字保持精度。

SyncEvent/ChangeEvent 的 primary_key/before/after 中，二进制列使用单键对象 `{"$nodebridge_binary_base64":"..."}`；NULL 仍为 null，空二进制为标签空串，普通文本/数字不变。反序列化恢复 SQL 字节值，映射仍按源列名进行；旧版不支持，需相关双端同包升级。诊断展示不得把带标签值误称普通文本或已解码业务字符串。


规则可选字段`initial_alignment.policy`只接受DISABLED/MANUAL，缺失等于DISABLED，不提供AUTO。保存、节点注册、安装及Agent启停都不得创建首次对齐任务。MANUAL只允许后续明确提出预览/确认，不等于授权执行。

## DTO Summary

- `ConfigDTO`: `mode`、`node`、`mysql`、`rabbitmq`、`cdc`、`sync`、`log_web`、`mcp_server.enable`、`security.admin_password`、`security.exit_password`。
- `CDCConfig`: `type`、`mode`、`install`、`reader_name`、`canal_addr`、`config_dir`、`service_name`、`destination`、`username`、`password`、`filter`、`batch_size`、`use_gtid`。
- `SyncRulesDTO`: `rules[]`，规则字段沿用 `rules.SyncRule`，包含 `source_node_ids[]`、`dispatch_target`、`dispatch_node_ids[]`、`sync_mode`、`schema_sync.add_columns`、`schema_sync.drop_columns`。列同步开关默认 `false`；非 `SERVER_TO_EDGE` 的 `drop_columns=true` 必须恰好限定一个 `source_node_id`。
- `sync_mode`: 可为空或 `crud_ordered` / `append_only` / `crud_ordered_compact`。空值等同 `crud_ordered`；`append_only` 仅用于历史倾倒/采集流水表，只接受 `INSERT`，后端可用批量插入优化，收到 `UPDATE` / `DELETE` 必须失败并进入现有重试/死信链路；`crud_ordered_compact` 仅用于允许合并同一主键连续 `UPDATE` 中间状态的状态表，后端不跨 `INSERT` / `DELETE` 边界，默认不得自动选择。
- `sync.apply_lanes`: Server ingress 独立主键并行 Apply lane 数；空值或 0 由后端默认处理，前端可先只透传。
- `sync.enable_crud_compact`: Settings 里的全局性能开关，默认 `false`；只有开启后，单条规则的 `sync_mode=crud_ordered_compact` 才会执行。前端不得提供一键把全部规则改成 compact 的操作。
- `NodeOptionsResponse`: `items[]`、`status`、`message`；每项包含 `node_id`、`node_name`、`node_type`、`status`、`location`、`last_heartbeat_at`，当前只返回 ACTIVE Edge。
- `Overview`: `product_name`、`mode`、`node_id`、`node_name`、`config_loaded`、`config_path`、`rules_path`、`agent_status`、`agent_pid`、`agent_log_path`、`mysql_status`、`rabbitmq_status`、`cdc_status`、`cdc_message`、队列和计数字段。
- `QueueStatusDTO`: `name`、`role`、`messages`、`consumers`、`status`。
- `FailedEventDTO`: `event_id`、`target_node_id`、`status`、`error_message`、`created_at`，时间字段为 RFC3339 字符串。
- `DeadLetterMessageDTO`: `queue`、`content_type`、`body_preview`、`body_size`、`headers`；预览内容可能包含业务数据，前端必须按敏感信息处理。
- `LogEntry`: `time`、`level`、`module`、`message`，时间字段为 RFC3339 字符串。
- `OperationResult`: `ok`、`status`、`message`。
- `AuthState`: `unlocked`、`status`、`expires_at`、`timeout_seconds`、`message`，`expires_at` 为 RFC3339 字符串。
- `AutoStartStatus`: `enabled`、`status`、`message`。
- `MCPServerStatus`: `enabled`、`status`、`message`、`transport`、`ephemeral`、`restart_resets`；`configured` 表示 stdio MCP 开关已持久启用，`ephemeral=false` 和 `restart_resets=false` 表示 NodeBridge 重启后不会自动关闭。
- `ManagedInstallResponse`: `mode`、`manifest_path`、`operations[]`；operation 包含 `component/action/target/status/message`。
- `AgentProcessStatus`: `executable_path`、`pid`、`status`、`started_at`、`exited_at`、`last_error`、`log_path`，时间字段为 RFC3339 字符串。
- `DiagnosticPackageResponse`: `path`。

## MCP Stdio Tools

`SyncAgent.exe mcp-stdio` 是本机 stdio MCP 入口，不开放 HTTP 端口。现场目标是：边缘机无屏幕时，操作员可在另一台电脑上的 AI 客户端中受控查看并修改本机 NodeBridge 配置。

普通模式 `mcp-stdio` 启动时强制检查 `mcp_server.enable=true`；显式 `-lab-full-access` 的实验室模式按用户授权绕过此门禁，允许首次配置和全部字段写入。

| Tool | Capability | Safety |
| --- | --- | --- |
| `nodebridge_overview` | 读取模式、节点、配置路径和脱敏 RabbitMQ 摘要。 | 只读。 |
| `nodebridge_config_summary` | 读取脱敏配置和 MCP 写入策略。 | 密码、token、security 字段永不返回明文。 |
| `nodebridge_sync_rules` | 读取同步规则。 | 只读。 |
| `nodebridge_logs` | 读取配置的 SyncAgent 日志尾部。 | 只读。 |
| `nodebridge_diagnostic_summary` | 读取诊断路径、安全边界和允许动作。 | 只读。 |
| `nodebridge_validate_config_patch` | 校验非敏感 JSON 配置补丁并返回脱敏后的预期结果。 | 不落盘；建议远程 AI 在保存前先调用。 |
| `nodebridge_save_config_patch` | 保存 JSON 配置补丁。 | 普通模式只允许非敏感白名单字段；实验室全权限模式允许全部配置字段，并可传 `restart_agent:true`。托管 RabbitMQ 按 `mode/node.id` 生成账号，并先修改本机 RabbitMQ 用户，成功后才保存配置。 |
| `nodebridge_save_sync_rules` | 保存同步规则。 | 使用后端 `rules.SaveFile` 校验 identifier、主键映射、分发策略和 `sync_mode`。 |
| `nodebridge_ensure_server_edge_user` | 在中心节点按边缘 `node_id` 创建或迁移中心 RabbitMQ 用户。 | 仅实验室全权限模式；只管理 `nb-<node.id>` 和 `/nodebridge-server`。 |

实验室模式另有 5 个结构化数据库治理工具：`nodebridge_mysql_query`、`nodebridge_mysql_mutation_plan/apply`、`nodebridge_mysql_schema_change_plan/apply`。它们只访问已保存的 `mysql.database`；不接受任意 SQL，不允许修改 `sync_*` 表，数据写入限制 100 行并要求影响行数确认，结构写入仅允许既有业务表的 `ADD COLUMN` / `DROP COLUMN` 且要求当前 `plan_id`。自动同步也只支持加列和删列，绝不自动建表或删表。

MCP 写操作和被拒绝的写操作记录审计日志。实验室模式可通过现有后端管理方法执行失败事件重试、受管拓扑初始化等操作；受管资源归属边界不变。下面的有限写工具表描述普通模式，实验室完整工具清单见 `docs/mcp-service.md`。

## MySQL Diagnostics (Candidate v0.46.9)

实验室模式新增只读 `nodebridge_mysql_diagnostics`，不新增 Wails 页面或 binding。请求仅允许 `event_tables?: string[]`（最多8个不重复简单表名）、`timeout_seconds?: int`（默认5秒，最大10秒），只使用已保存的 MySQL 数据库。无表名时不扫描事件日志；指定表名后返回精确事件总数/失败数及非执行式 EXPLAIN JSON。不得传任意 SQL、数据库或连接凭据。

响应为 `{database, at, status, elapsed_ms, steps[]}`；每步包含 `name/status/elapsed_ms/error_code?/data?`。固定步骤为连接、buffer pool字节数、四张系统表估计行数/数据及索引字节数、索引列定义；可选步骤为失败计数执行计划、事件总数和失败数。`status=partial` 或步骤 `error/skipped` 不是通过；未知计数不返回0。表行数标记 `estimated_rows`，不当作精确账本。遵守调用方取消和整体期限；不返回payload、业务行、凭据或原始SQL错误。

Server标准 `migrate -scope server` 会幂等补充 `sync_event_log.idx_table_status(table_name,status)`，只接受可见完整列定义；冲突定义报错，不擅自替换。该DDL不由诊断接口执行，也不在每次Agent启动时隐式执行。已有数据库须在正式升级验收阶段明确执行迁移。

## Sensitive Methods

以下方法必须在 unlocked 状态下调用：

- `SaveConfig`
- `SaveSyncRules`
- `RetryFailedEvent`
- `RetryFailedEvents`
- `GetDeadLetters`
- `SetAutoStart`
- `SetMCPServerEnabled`
- `ApplyManagedInstall`
- `ExportDiagnosticPackage`
- `StartAgent`
- `StopAgent`
- `RestartAgent`

只读方法不要求解锁：`GetOverview`、`GetConfig`、`GetSyncRules`、`GetNodeOptions`、`GetQueueStatus`、`GetFailedEvents`、`GetLogs`、`GetAgentProcessStatus`。

## Change Discipline

- 改方法名、字段名、错误语义前，先在根级 `AI_BOARD.md` 的 Active Board 记录。
- contract 更新后，前端服务层和后端测试必须同步更新。
- `frontend-backend-contract.md` 只放稳定契约，不放对话、临时问题或任务流水。
- 前后端交流只使用根级 `AI_BOARD.md`，不再新增分散看板。
- 不确定内容使用 `<!-- 待确认 -->`。
