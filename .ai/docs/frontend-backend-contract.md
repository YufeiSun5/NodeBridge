# Frontend Backend Contract

本文件定义 DataSync Wails UI 的前后端契约。前端只调用 Wails binding，不直接访问 SyncAgent HTTP API、RabbitMQ、MySQL 或配置文件。

## 通用规则

- Wails 绑定对象：`datasyncui.App`；前端服务层可保留 `main.App` 作为旧入口 fallback，但必须优先使用 `window.go.datasyncui.App`。
- 错误：Go 方法可返回 `error`；前端必须显示为 error state。
- 空能力：未接真实能力时返回空数组、`unknown` 或 `unsupported`，不得返回假成功。
- 脱敏：后端返回配置时必须隐藏 MySQL/RabbitMQ/CDC/LogWeb 密码、token、管理密码和退出密码。
- 字段名：JSON 使用 snake_case，前端 DTO 必须保持一致。
- 关闭行为：Wails 原生关闭按钮隐藏窗口到 Windows 系统托盘并阻止退出；真正退出必须先调用 `RequestExit` 再调用 `runtime.Quit`。
- 托盘菜单由 Wails 后端桌面层创建：左键双击或右键“显示 DataSync”恢复窗口；右键“退出...”会恢复窗口并向前端发送 `datasync:request-exit`，由前端打开退出密码弹窗。
- 管理鉴权：启动默认 locked；敏感方法未解锁时返回 error 或 `status=locked`，前端必须显示解锁弹窗。

## Methods

| Method | Request | Response | Notes |
| --- | --- | --- | --- |
| `GetOverview()` | none | `status.Overview` | 模式、节点、配置加载状态、配置路径、规则路径、服务状态、队列积压、失败数、版本。未知字段返回 `unknown`。 |
| `GetConfig()` | none | `uiapi.ConfigDTO` | 返回脱敏配置。 |
| `SaveConfig(req)` | `uiapi.SaveConfigRequest` | `uiapi.ConfigDTO` | 校验配置并保存到 `%ProgramData%\NodeBridge\config.yaml`；redacted secret 自动沿用旧值；`security.admin_password` 不能为空。 |
| `TestMySQL(req)` | `appconfig.MySQLConfig` | `uiapi.TestResult` | 使用 3 秒超时 ping MySQL。 |
| `TestRabbitMQ(req)` | `appconfig.RabbitMQConfig` | `uiapi.TestResult` | 优先测试 `server_url`，为空时测试 `local_url`。 |
| `GetSyncRules()` | none | `uiapi.SyncRulesDTO` | 优先读取 `%ProgramData%\NodeBridge\sync-rules.yaml`，缺失时回退 example。 |
| `SaveSyncRules(req)` | `uiapi.SaveSyncRulesRequest` | `uiapi.SyncRulesDTO` | 校验 identifier、来源节点作用域、主键映射和列映射后落盘。 |
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
| `GetMCPServerStatus()` | none | `uiapi.MCPServerStatus` | 查询 MCP Server 预留开关；默认关闭。 |
| `SetMCPServerEnabled(req)` | `uiapi.SetMCPServerEnabledRequest` | `uiapi.MCPServerStatus` | 保存 MCP Server 预留开关；只写配置，不启动真实 MCP runtime。 |
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
- `SyncRulesDTO`: `rules[]`，规则字段沿用 `rules.SyncRule`，包含 `source_node_ids[]`、`dispatch_target`、`dispatch_node_ids[]`。
- `Overview`: `product_name`、`mode`、`node_id`、`node_name`、`config_loaded`、`config_path`、`rules_path`、`agent_status`、`agent_pid`、`agent_log_path`、`mysql_status`、`rabbitmq_status`、`cdc_status`、`cdc_message`、队列和计数字段。
- `QueueStatusDTO`: `name`、`role`、`messages`、`consumers`、`status`。
- `FailedEventDTO`: `event_id`、`target_node_id`、`status`、`error_message`、`created_at`，时间字段为 RFC3339 字符串。
- `DeadLetterMessageDTO`: `queue`、`content_type`、`body_preview`、`body_size`、`headers`；预览内容可能包含业务数据，前端必须按敏感信息处理。
- `LogEntry`: `time`、`level`、`module`、`message`，时间字段为 RFC3339 字符串。
- `OperationResult`: `ok`、`status`、`message`。
- `AuthState`: `unlocked`、`status`、`expires_at`、`timeout_seconds`、`message`，`expires_at` 为 RFC3339 字符串。
- `AutoStartStatus`: `enabled`、`status`、`message`。
- `MCPServerStatus`: `enabled`、`status`、`message`；`configured` 表示开关已启用但真实 MCP runtime 尚未开放。
- `ManagedInstallResponse`: `mode`、`manifest_path`、`operations[]`；operation 包含 `component/action/target/status/message`。
- `AgentProcessStatus`: `executable_path`、`pid`、`status`、`started_at`、`exited_at`、`last_error`、`log_path`，时间字段为 RFC3339 字符串。
- `DiagnosticPackageResponse`: `path`。

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

只读方法不要求解锁：`GetOverview`、`GetConfig`、`GetSyncRules`、`GetQueueStatus`、`GetFailedEvents`、`GetLogs`、`GetAgentProcessStatus`。

## Change Discipline

- 改方法名、字段名、错误语义前，先在 `.ai/docs/ai-collaboration-log.md` 的 Active Board 记录。
- contract 更新后，前端服务层和后端测试必须同步更新。
- `frontend-backend-contract.md` 只放稳定契约，不放对话、临时问题或任务流水。
- 前后端交流只使用 `.ai/docs/ai-collaboration-log.md`，不再新增分散看板。
- 不确定内容使用 `<!-- 待确认 -->`。
