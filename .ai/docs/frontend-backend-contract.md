# Frontend Backend Contract

本文件定义 NodeBridge Wails UI 的前后端契约。前端只调用 Wails binding，不直接访问 SyncAgent HTTP API、RabbitMQ、MySQL 或配置文件。

## 通用规则

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
| `SaveSyncRules(req)` | `uiapi.SaveSyncRulesRequest` | `uiapi.SyncRulesDTO` | 校验 identifier、来源节点作用域、主键映射和列映射后落盘。 |
| `GetNodeOptions()` | none | `uiapi.NodeOptionsResponse` | 只读返回 ACTIVE Edge 节点候选，用于 Rules 页 `dispatch_node_ids` 选择；配置缺失返回空列表和 `unknown`。 |
| `GetQueueStatus()` | none | `uiapi.QueueStatusResponse` | 使用 RabbitMQ passive declare 查询队列深度；无法连接时返回 error 状态。 |
| `GetFailedEvents(req)` | `uiapi.FailedEventsRequest` | `uiapi.FailedEventsResponse` | 从 SyncAgent MySQL 系统表查询失败 ACK。 |
| `RetryFailedEvent(req)` | `uiapi.RetryFailedEventRequest` | `uiapi.OperationResult` | 将失败 ACK 标记为 `PENDING`。 |
| `RetryFailedEvents(req)` | `uiapi.RetryFailedEventsRequest` | `uiapi.OperationResult` | 批量将失败 ACK 标记为 `PENDING`，按 limit 限制数量。 |
| `GetDeadLetters(req)` | `uiapi.DeadLetterRequest` | `uiapi.DeadLetterResponse` | 只读预览死信队列消息，读取后立即 requeue；需要管理解锁。 |
| `GetLogs(req)` | `uiapi.LogQuery` | `uiapi.LogsResponse` | 从后端 runtime ring buffer 读取日志，支持 level/module/limit。 |
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

## DTO Summary

- `ConfigDTO`: `mode`、`node`、`mysql`、`rabbitmq`、`cdc`、`sync`、`log_web`、`mcp_server.enable`、`security.admin_password`、`security.exit_password`。
- `CDCConfig`: `type`、`mode`、`install`、`reader_name`、`canal_addr`、`config_dir`、`service_name`、`destination`、`username`、`password`、`filter`、`batch_size`、`use_gtid`。
- `SyncRulesDTO`: `rules[]`，规则字段沿用 `rules.SyncRule`，包含 `source_node_ids[]`、`dispatch_target`、`dispatch_node_ids[]`、`sync_mode`。
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

MCP 写操作和被拒绝的写操作记录审计日志。实验室模式可通过现有后端管理方法执行失败事件重试、受管拓扑初始化等操作；受管资源归属边界不变。下面的有限写工具表描述普通模式，实验室完整工具清单见 `docs/mcp-service.md`。

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
