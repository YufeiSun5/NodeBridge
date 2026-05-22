# Frontend Requirements

DataSync 前端第一阶段只做本地管理端，不实现同步核心逻辑。

## Global Rules

- 必须遵循 `.ai/docs/ui-design-spec.md`。
- 只使用 Wails bindings，禁止 `fetch` / `axios`。
- 服务层必须优先调用 `window.go.datasyncui.App`，不能只查 `window.go.main.App`。
- 禁止 Tailwind、Bootstrap 等 CSS 框架。
- 页面必须包含 loading、empty、error 状态。
- 前端不得直接操作 RabbitMQ、MySQL、配置文件路径细节。
- 前端不得实现 CDC、Apply、Loop Suppressor、Conflict Resolver 逻辑。
- 前端负责托盘控制按钮、退出密码弹窗和退出确认；Windows 原生托盘图标与菜单由 Wails 后端桌面层负责。
- 退出前必须调用 `VerifyExitPassword`；自启动开关必须调用 `GetAutoStart` / `SetAutoStart`。
- 管理界面必须有锁定/解锁状态；未解锁时只能查看只读概览和状态，不得执行敏感操作。
- 敏感操作前必须调用 `UnlockAdmin` 并确认 `GetAuthState.unlocked=true`。
- 管理密码和退出密码分离：`security.admin_password` 用于解锁敏感操作，`security.exit_password` 用于托盘退出。

## Required Pages

### Overview

必须显示：

- 当前模式：Edge / Server / unknown。
- 节点 ID、节点名称。
- Agent、MySQL、RabbitMQ、CDC 状态。
- 上传积压、下发积压、失败事件数、冲突数。
- 当前版本。

必须提供：

- Start / Stop / Restart 按钮。
- Start / Stop / Restart 必须在未解锁时禁用或触发解锁弹窗。
- `unsupported` 时显示明确状态，不显示成功。

### Config / Sync Config

必须显示和编辑：

- 节点配置：id、name、location。
- MySQL：host、port、database、username、password 输入。
- RabbitMQ：mode、install、local_url、server_url、management_url、username、password、vhost。
- CDC：type、reader_name、canal_addr、destination、filter、batch_size。
- Log Web：enable、bind、port、token。

必须提供：

- Test MySQL。
- Test RabbitMQ。
- Save Config。
- Save Config 必须要求管理解锁。
- 密码字段显示为脱敏值；用户重新输入后才覆盖。

必须禁止：

- 将软件设置混入同步配置页，包括语言、退出入口、管理密码、退出密码和登录自启动。

### Settings

必须显示和编辑：

- Language：默认跟随操作系统语言，允许用户手动选择中文、英文、日文。
- Theme：默认跟随系统，允许用户手动选择深色模式或正常模式；该偏好只保存在前端本地，不写入同步配置。
- Security：admin_password、exit_password。
- Auto Start：当前用户登录自启动开关。
- MCP Server：默认关闭，只调用 Wails 后端预留接口保存开关，不在前端启动 runtime。
- Window and Exit：说明关闭窗口进入系统托盘，真正退出走退出密码弹窗。
- MCP Server：预留开关，默认关闭；当前只表示配置开关，不表示 MCP runtime 已经运行。

必须提供：

- Admin Password 用于解锁管理操作；Exit Password 用于托盘退出。
- 保存安全设置时必须提供 Admin Password；空管理密码不能保存。
- Verify Exit Password 的 UI 不在同步配置页直接暴露密码明文。
- Set AutoStart。
- Set AutoStart 必须要求管理解锁。
- Set MCP Server 必须要求管理解锁。
- MCP Server 开关必须调用 `GetMCPServerStatus` / `SetMCPServerEnabled`。
- 启用 MCP Server 开关必须要求管理解锁，并如实显示 `configured` / `unsupported` / `error` 状态。

必须禁止：

- 默认启用 MCP Server。
- 前端自行启动 MCP runtime。
- 前端绕过 Wails 后端直接读写 MCP 配置。

### Rules

必须显示和编辑：

- database_name、table_name。
- source_node_ids：可选，逗号分隔；用于 10 个 Edge 都叫 `data_all` 但中心目标表不同的场景。
- target_database_name、target_table_name。
- direction、conflict_policy、enable。
- dispatch_target：`AUTO`、`NONE`、`ACTIVE_EDGES`、`SELECTED_EDGES`。
- dispatch_node_ids：`SELECTED_EDGES` 时必填，逗号分隔。
- primary_keys、target_primary_keys。
- include_columns、exclude_columns。
- column_mappings：source_column -> target_column。

必须提供：

- 编辑和保存规则前要求管理解锁。

必须强调：

- 不得假设源表列名等于目标表列名。
- 不得假设同名源表只有一条规则；`source_node_ids` 不为空时必须显示并保存。
- 不得假设所有多主表都固定分发给所有 Edge；分发范围必须显示并保存。
- include/exclude 使用源列名。

### Queues

必须显示：

- Local upload。
- Server ingress。
- Downlink。
- Dead letter。
- Retry。

每项显示：

- queue name。
- messages。
- consumers。
- status。

### Failures

必须显示：

- event_id。
- target_node_id。
- status。
- error_message。
- created_at。

必须提供：

- Retry 按钮占位。
- Retry 必须要求管理解锁。
- Empty state。

### Logs

必须显示：

- time。
- level。
- module。
- message。

必须提供：

- level/module 筛选。
- copy message。
- 日志 Web 入口状态说明。
- 诊断包导出入口必须要求管理解锁。

### Admin Lock

必须实现：

- 启动后默认 locked。
- locked 状态允许只读 Overview、Queues、Failures、Logs、Config、Rules。
- Config 和 Rules 在 locked 状态下必须使用只读文本/表格展示，不显示输入框、保存、新增、删除等会造成可编辑错觉的控件。
- 密码、token 等重要数据在 locked 状态下必须使用磨砂/遮罩效果；普通参数允许明文查看。
- Config、Rules、Retry、Diagnostic、AutoStart、MCP Server、Start/Stop/Restart 操作必须要求解锁。
- 解锁弹窗调用 `UnlockAdmin`，只提交管理密码。
- 顶部或状态区显示 locked/unlocked。
- 提供手动 Lock 按钮，调用 `LockAdmin`。
- 可定时调用 `GetAuthState`，过期后恢复 locked UI。

必须禁止：

- 在前端缓存明文管理密码。
- 用退出密码替代管理密码做敏感操作解锁。
- 绕过后端鉴权直接调用敏感操作后假装成功。
- locked 状态下保存配置、规则或导出诊断包。
- locked 状态下展示配置输入框、规则输入框或删除按钮。

### Tray Shell

必须实现：

- 点击窗口关闭按钮时隐藏到 Windows 系统托盘，不直接退出。
- 托盘菜单提供显示窗口和退出入口；退出入口必须回到前端退出密码弹窗。
- exit 动作弹出退出密码输入。
- 密码验证成功后才退出应用。
- 密码验证失败显示 error state。

必须禁止：

- 前端自行读取配置文件。
- 前端自行保存退出密码。
- 前端绕过 `VerifyExitPassword` 直接退出。

## Acceptance

- `npm run build` 必须通过，前提是 Node/npm 环境可用。
- 全局搜索不得出现 `fetch(`、`axios`、Tailwind、Bootstrap。
- 无后端真实数据时页面不崩溃。
- Wails 接口错误时进入 error state。
- 视觉必须保持暗色工业终端风格。
