# AI Collaboration Board

本文件是前端 AI、后端 AI、测试 AI 与整体审阅/修改 AI 的唯一活跃交流看板。稳定接口契约仍写在 `.ai/docs/frontend-backend-contract.md`，但疑问、阻塞、分工和交接只写这里。

本文件与 `MEMORY.md` 同级，属于当前工作状态，不属于归档文档。`.ai/docs/` 只保留稳定说明、闭合记录、阶段总结和归档材料；旧路径 `.ai/docs/ai-collaboration-log.md` 仅保留迁移提示，不能继续写入活跃事项。

## 文件模型

保留两个核心文件：

| 文件 | 用途 |
| --- | --- |
| `.ai/docs/frontend-backend-contract.md` | 稳定 Wails API、DTO、错误语义、脱敏规则。 |
| `AI_BOARD.md` | 前端、后端、测试、审阅 AI 的活跃看板、交接记录、当前 open/blocked 项。 |
| `.ai/docs/` | 稳定文档、闭合记录、阶段总结和归档材料；不承载活跃看板。 |

不再新增前端看板、后端看板、测试看板、审阅看板或零散交流文件。需要前端处理的问题、需要后端处理的问题、需要测试验证的问题、整体审阅结论、接口变更疑问，都进入本文件顶部的 Active Board。

## AI Identities

所有 AI 在操作前必须声明一个当前身份，并用该身份写入 Active Board 和 Activity Log。

| Identity | Owner value | Scope | Boundary |
| --- | --- | --- | --- |
| Frontend AI | `frontend-ai` | React/Wails UI、前端服务层、三语文案、视觉与交互。 | 不实现同步核心，不直接访问 RabbitMQ/MySQL/HTTP。 |
| Backend AI | `backend-ai` | Go 后端、SyncAgent、Wails API、DTO、配置、迁移、安装器、MCP stdio。 | 涉及前端的数据、DTO、页面或阻塞必须写入 Active Board。 |
| Test AI | `test-ai` | 单元测试、smoke、lab/E2E、压测、发布门禁和测试证据。 | 默认不改产品行为；如需修测试夹具或脚本，必须说明范围。 |
| Review AI | `review-ai` | 跨模块审阅、架构评估、上线评估、风险清单和受控修正。 | 先给结论和风险；跨身份修改必须记录影响范围。 |

`Owner` 只能使用 `frontend-ai`、`backend-ai`、`test-ai`、`review-ai`。跨身份任务必须在 `Item` 或 `Next Action` 写明原因、影响范围和下一个 owner。

## Active Board

| ID | Owner | Type | Status | Item | Next Action |
| --- | --- | --- | --- | --- | --- |
| FB-038 | backend-ai | release | closed | v0.46.3 内网安装包：新装使用无节点/MySQL/同步规则预设的空白引导配置；NodeBridge 解锁、退出和自有 RabbitMQ 密码统一为 `1234`；托管 RabbitMQ 账号按 `mode/node.id` 生成并在配置落盘前同步修改服务用户；升级迁移旧自有密码但保留 MySQL、Canal 和已有规则；UI 脱敏密码测试及 MCP 保存后可选重启已修复。 | 最终包已被 FB-039 的覆盖安装修订版取代；原包不得继续分发。 |
| FB-039 | backend-ai | test | open | v0.46.3 覆盖安装：NSIS 复制前精确停止安装目录内旧 NodeBridge/SyncAgent；本机真实两轮 NSIS 覆盖测试已通过，运行中旧 SyncAgent 被停止，二进制更新且配置/规则保留。 | 最终发布包 `build/NodeBridge-beta-v0.46.3-20260907.exe`，SHA256 `DD6852F3BA87550C8C1C708A52B8507875562E322EA3FE114DB4B8A6CAB1D6F9`；证据 `.cache/nsis-upgrade/20260907-174024-917/`。目标边缘机 TCP/22 在线但在 SSH banner 前主动断开，恢复后上传执行真实覆盖升级并核对 RabbitMQ/MCP。 |
| FB-037 | backend-ai | release | open | 发布文档：补充一主多边缘部署、Windows/Mac SSH 公钥授权与 MCP 配置手册，更新根 README，并提交、推送、创建 GitHub Release。 | 发布版本已升级为 FB-039 的 v0.46.3 修订包；提交 main、打 v0.46.3 标签并上传安装包后关闭。 |
| FB-036 | backend-ai | bug | closed | v0.46.1 安装后普通运行的 `admin\xx` 仅有 ProgramData/NodeBridge Write 权限，原子替换 config.yaml 因缺少 delete-child 权限返回 Access denied。已在目标机按 SID 授予 Modify 并验证替换；安装器新增当前安装账户继承 Modify ACL。 | 目标机 MCP SSH 真实握手、26 工具、overview 和 save_config_patch 通过；Windows Codex 已全局注册。最终 v0.46.2 包 `build/NodeBridge-beta-v0.46.2-20260907.exe`，SHA256 `620208E9B275EAFA1DA2C856C1BFE774F996A5A00FD402DAC6304A7EC3D6BA14`，32/64 位各 13 项回归、解包、资产和 preflight 通过。Mac 私钥仍须在 Mac 本机生成后追加公钥。 |
| FB-035 | backend-ai | bug | closed | v0.46.1 修复 NSIS 32 位 PowerShell 漏查 x64 Erlang/RabbitMQ：Sysnative 启动、ProgramW6432 检测、原生参数转义、进程句柄保留与安装后探测；日志按次写到独立 ProgramData/NodeBridgeInstallerLogs。无前端 API 变更。 | 32/64 位各 12 项安装器回归、Go test/vet/lint、五个资产 hash、包内 MCP 26 工具 smoke 通过。交付 build/NodeBridge-beta-v0.46.1-20260907.exe；真实异机安装/卸载与 SSH 仍待 FB-034 验收，未在宿主机运行系统组件安装。 |
| FB-033 | review-ai | task | closed | Windows 第二任务：按用户授权交付 MCP v0.46 全配置实验室版。26 个工具覆盖全部配置字段/凭据/规则/自启动、真实诊断、重试、拓扑和同步进程；SSH stdio 支持 Windows/Mac 客户端，保留加密/校验/脱敏。 | 已修复协议错误、日志 limit、缺失 rules 清空风险、并发 patch 覆盖、跨进程 Agent 识别/独占锁和原子落盘；go test/vet/linter、Wails 构建、包内 EXE/中文/空格路径 smoke 通过。安装包 build/NodeBridge-beta-v0.46.0-20260907.exe，SHA256 0017615A0EB455A2DD8920075775B0913077412480D868AC8FD2DC3D518BB5B8。真实异机验收交给 test-ai FB-034。 |
| FB-034 | test-ai | test | open | MCP v0.46 真实内网异机验收：Windows 控制端已通过 SSH 公钥连接边缘机 `192.168.10.102`；真实 initialize、26 工具、配置保存、MySQL/RabbitMQ 探测和规则清空通过。 | 已按现场账号配置 `127.0.0.1:3306/scada_edge` 并创建 UTF8MB4 空库，MySQL/RabbitMQ 均 running；当前 `sync-rule.yaml` 与旧 `sync-rules.yaml` 均清空，队列为 0，Agent 保持 stopped。下一步验证 Mac 公钥、Agent 启停/重连、首次无 UI 配置及安装器二次安装/卸载（关联 FB-032）。 |
| FB-001 | backend-ai | question | closed | 前端需要显式配置状态，避免长期通过空 `mode/node.id/mysql.database` 推断首次配置。 | 已在 `GetOverview` 增加 `config_loaded/config_path/rules_path/node_id/node_name`。 |
| FB-002 | backend-ai | question | closed | Wails 绑定生成 `Not found: time.Time` 警告。 | Wails UI DTO 时间字段已改为 RFC3339 string。 |
| FB-003 | backend-ai | question | closed | 安装后 `SyncAgent.exe` 固定查找路径与优雅 shutdown 协议未冻结。 | 已冻结查找顺序并实现 stop-file 优雅停止；前端只展示 `GetAgentProcessStatus()` 和操作结果。 |
| FB-004 | frontend-ai | decision | closed | Rules UI 需要展示并编辑 `dispatch_target` 和 `dispatch_node_ids`，用于配置单向汇总、多主 fanout、指定节点下发。 | 当前 Rules 页已显示/编辑这两个字段，后续只需按 Frontend V0.26 Plan 做 exe 级验收。 |
| FB-005 | backend-ai | bug | closed | Overview 缺少显式 `config_loaded/config_path/node_id/node_name`，前端只能靠 `GetConfig` 拼数据，exe 首次启动容易显示 unknown。 | 已并入 FB-001 的 `GetOverview` 字段扩展。 |
| FB-006 | backend-ai | bug | closed | CDC 状态在 `GetOverview` 中没有真实探测逻辑，配置存在时仍长期显示 `unknown`。 | 后端按 `cdc.type` 返回 `configured/running/error/unknown`，Agent 运行时返回 `running`。 |
| FB-007 | backend-ai | bug | closed | `GetSyncRules` 在 exe 双击启动时依赖相对路径 `configs/sync-rules.example.yaml`，可能回退到内置 2 条默认规则，导致现场规则不显示。 | 后端改为优先现场默认规则，并把 fallback 规则落盘到 `sync-rules.yaml`。 |
| FB-008 | backend-ai | bug | closed | Logs 页只读取 Wails UI 进程 ring buffer，不读取外部 `SyncAgent.exe` 日志；启动外部 agent 后同步记录仍可能为空。 | 后端将外部 SyncAgent stdout/stderr 写入 `logs/sync-agent.log`，`GetLogs` 合并读取。 |
| FB-009 | backend-ai | design | closed | 需要兼容 MCP/ClaudeCode 对 NodeBridge 的自动化操作，但不能让 MCP 直接绕过 Wails 管理鉴权、配置校验或同步边界。 | V0.31 已提供只读 `mcp-stdio` alpha，不开放写配置、RabbitMQ mutation 或 MySQL 写入。 |
| FB-010 | frontend-ai | task | closed | Settings 页需要预留 MCP Server 开关，默认关闭；前端不得直接启动 MCP runtime。 | 已接入 `GetMCPServerStatus` / `SetMCPServerEnabled`，未解锁时先触发管理解锁，并将 `configured` 显示为“已启用但运行时未开放”。 |
| FB-011 | backend-ai | question | closed | Rules 页 `dispatch_node_ids` 需要选择 ACTIVE Edge 目标节点，但当前 Wails contract 没有节点列表/候选项接口。 | 后端已新增只读 `GetNodeOptions()`，返回 ACTIVE Edge 候选和稳定 empty/error 状态。 |
| FB-012 | frontend-ai | task | closed | V0.27 新增 `GetAgentProcessStatus()`；前端需要展示 SyncAgent 路径、PID、日志路径、`stopped/running/exited/error/forced_stopped` 状态。 | Overview 已读取 `GetAgentProcessStatus()` 并展示路径、PID、启动/退出时间、日志路径、最后错误和状态。 |
| FB-013 | frontend-ai | task | closed | `CDCConfig` 新增 `mode/install/config_dir/service_name`，用于 Canal managed/external 安装边界配置。 | Config 页已展示并编辑这些字段；后续可优化 external 模式说明文案。 |
| FB-014 | frontend-ai | task | closed | V0.30 新增 `RetryFailedEvents` 和 `GetDeadLetters`；Failures 页需要批量重试和死信只读预览入口。 | Failures 页已接入批量重试和死信只读预览，操作前均要求管理解锁。 |
| FB-015 | frontend-ai | task | closed | V0.31 新增 `GetManagedInstallPlan` / `ApplyManagedInstall`；Settings 或 Config 页后续需要展示受管组件计划和执行结果。 | Settings 页已展示受管组件计划、manifest 路径和 operations；执行入口先管理解锁，并明确 alpha 资源边界。 |
| FB-016 | test-ai | task | closed | 测试 AI 需要在 WinServer2022 Core / 无 GUI VM 中验证后端交付的 headless installer test bundle。 | V0.38.3 已在 `Clean-Windows-Installed` 干净 VM 通过闭环：第一轮 `-ExecuteInstall` 通过，`-VerifyOnly` 通过，二次 `-ExecuteInstall` 通过，`-Uninstall` 通过且 `verify-uninstall` passed；卸载后确认无 RabbitMQ/NodeBridge/Canal 服务。缺 Java 场景按预期跳过 Canal Service。证据在 `.cache/v0.38.3-clean-validation/`。 |
| FB-017 | frontend-ai | task | closed | Rules 页需要把 `GetNodeOptions()` 接入 `dispatch_node_ids`，用 ACTIVE Edge 候选替代纯手填，并保留手填兜底。 | Rules 编辑态已接入 ACTIVE Edge 候选勾选区，显示 empty/error 状态和三语提示，同时保留手填节点 ID 兜底。 |
| FB-018 | frontend-ai | task | closed | Settings/通知设置中的 MCP Service 开关语义调整：默认关闭，只在本次 NodeBridge 会话生效，重启后自动关闭。 | 前端已更新为本次会话 stdio 临时开关；未加载配置时禁用开关并提示先保存同步配置，不再显示“保存开关”或“运行时未开放”。 |
| FB-019 | backend-ai | question | closed | 管理密码/退出密码忘记后的正式恢复流程需要产品化，不能让前端绕过 DPAPI 或管理鉴权。 | 产品决策已闭合：不提供后端找回或自动重置 CLI；用户自行退出 NodeBridge、备份 `%ProgramData%\NodeBridge\config.yaml`，再清空 `security.admin_password` 或 `security.exit_password` 的加密值后重启并重新设置。 |
| FB-020 | backend-ai | bug | closed | 首次使用只设置管理密码不会创建 `%ProgramData%\NodeBridge\config.yaml`，导致用户以为密码已设置但后端仍未加载配置，MCP/受管组件等依赖配置的功能失败。 | 后端已允许 `SaveConfig` 在仅含 `security` 的首次引导场景写入加密安全草稿；完整同步配置仍继续要求 `mode/node.id/mysql.database/security.admin_password` 校验。前端无需新增方法，继续调用 `SaveConfig`。 |
| FB-021 | backend-ai | bug | closed | 用户已在 UI 启动并开启 MCP 模式，但按生成的 MCP client config 运行 `SyncAgent.exe mcp-stdio -config %ProgramData%\NodeBridge\config.yaml -rules %ProgramData%\NodeBridge\sync-rules.yaml` 失败。test-ai 复测当前实际 `%ProgramData%` 安全草稿配置，`SyncAgent.exe mcp-stdio` 仍 exit 1，最新错误为 `decrypt admin password: Key not valid for use in specified state`；示例完整配置下 MCP stdio 协议本身通过，说明阻塞仍在 Wails MCP 开关/状态、DPAPI 安全草稿和 SyncAgent MCP 启动前置条件不一致。 | 后端已让 `GetMCPServerStatus` / `SetMCPServerEnabled` 在配置无法严格加载、配置不完整或 DPAPI 解密失败时返回 `unsupported`，并拒绝启用；完整配置下 MCP stdio 仍为只读。 |
| FB-022 | backend-ai | change | closed | 用户需求变更：MCP Server 模式不应随管理解锁过期关闭，也不应因 NodeBridge 重启自动关闭；启用后只允许用户手动关闭。 | 后端已取消启动/保存强制清零，`mcp_server.enable` 持久化到 YAML；`GetMCPServerStatus` 返回 `ephemeral=false/restart_resets=false`，启用后只由用户手动关闭。 |
| FB-023 | frontend-ai | task | closed | 后端已把 MCP 开关语义从“本次会话临时启用、重启关闭”改为“持久启用、只允许用户手动关闭”。 | 前端已更新 Settings/说明书三语文案：移除 session-only/restart reset 提示；当后端返回 `unsupported` 时展示“同步配置未完成或当前用户无法解密配置，不能启用 MCP”。 |
| FB-024 | test-ai | bug | closed | V0.40.5 已在 `Clean-Windows-Installed` 干净 VM 通过完整闭环：第一轮 `-ExecuteInstall -RequireCanalService` 通过，首次 `-VerifyOnly` 通过，二次 `-ExecuteInstall -RequireCanalService` 通过，二次 `-VerifyOnly` 通过，首次 `-Uninstall` 通过，二次 `-Uninstall` 通过且 `verify-uninstall` passed。卸载后确认无 RabbitMQ/NodeBridge/Canal 服务；已回收 `NodeBridgeCanal-delete-wait.json` / `NodeBridgeRabbitMQ-delete-wait.json`。证据在 `.cache/v0.40.5-clean-validation/`，宿主机 Erlang/RabbitMQ/Canal 未触碰。 | 无后续动作。 |
| FB-025 | test-ai | bug | open | 90 天等价长测第一轮：单机 Docker 模拟 Edge/Server，两侧 Docker MySQL/RabbitMQ，Edge Docker Canal，真实 CDC，6 张采集表 x 42 点位字段，目标 15,552,000 行，并覆盖 24h 实时、90d 等价灌入、1h/1d/7d 断线恢复、查询性能和滚动归档。 | backend-ai 已新增按表 `sync_mode`：默认 `crud_ordered` 保持增删改保序，`append_only` 用于历史倾倒/采集流水表，只接受 INSERT；Server Apply 对 append-only 批次按目标表分组多行插入，CanalUploadRuntime 整批 `PublishBatch` 后才提交 Canal offset。`appendonly-month30-004` 因 MySQL prepared statement 占位符上限失败，已修复为批量 SQL 自动切片并保留证据；新 30 天量级复测 `appendonly-month30-005` 正在后台 drain，证据目录 `.cache/longtest-90d/appendonly-month30-005/`。2026-05-31 05:54:52 快照：Edge=5,184,000，Server=1,880,000，Edge upload=1,154,000，Server ingress=2,160,000，dead/retry=0，runner/watchdog/agents 均存活，测试未完成。 |
| FB-026 | frontend-ai | task | closed | 前端需要联调 `sync_mode` 规则字段：新增规则默认 `crud_ordered`，编辑/保存不丢字段，`append_only` 需要明显危险提示和三语文案。 | Rules DTO、下拉、默认值和 Manual 已接入；本轮补强 `append_only` 危险提示，确认 Wails contract、TypeScript、Vite build 通过。 |
| FB-027 | test-ai | test | closed | 15 天混合断网压力测：5 张 `tag_state_*` 使用 `crud_ordered` 保序更新，12 张 `collect_data_*` 使用 `append_only` 历史倾倒，断网期间 Edge 软件保持运行。 | 已补测尾段清理，`mixed-15d-offline-002` `recovery-summary.json` 最终 `status=completed`，`server.cdc.ingress_depth=0`，`server_apply_log=7,349,000`，`server_event_log=7,349,000`，尾段一致通过。 |
| FB-028 | test-ai | performance | open | Server Apply 性能成倍提升计划：当前混合恢复证明 Edge upload 可清空，瓶颈集中在 Server apply；`crud_ordered` 路径仍接近“每事件业务 SQL + apply_log + savepoint”，append-only 在混合队列中被保序事件拖慢。 | backend-ai 已完成 P2/P3/P4 后端阶段：`append_only` 在 batch/segment 内按目标表批量写入；Server Apply 支持 `sync.apply_lanes` 按 `target_table + pk` 分 lane 并行，同一主键同 lane 保序；新增 `crud_ordered_compact`，只合并同一主键连续 UPDATE 的中间状态，不跨 INSERT/DELETE 边界。按用户要求 compact 需要双开关：Settings 的 `sync.enable_crud_compact=true` + 单条规则 `sync_mode=crud_ordered_compact`，否则后端拒绝执行；不允许一键开启全部。已通过 `go test ./...`、`go vet ./...`，`golangci-lint` 未安装跳过，已重建 `build/bin/SyncAgent.exe`。下一步 test-ai 用新二进制重启/重跑 `mixed-15d-offline-002` 恢复 drain，对比吞吐和顺序/幂等。 |
| FB-029 | frontend-ai | task | closed | P4 compact 前端配合：Settings 增加“启用 CRUD compact 性能模式”开关，默认关闭；Rules 单条规则增加 `crud_ordered_compact` 选项。 | Settings 已接入 `sync.enable_crud_compact` 全局开关并要求管理解锁；Rules 已增加单条 `crud_ordered_compact` 选项，未开启全局开关时禁用新选择；无一键全开入口；三语说明已写明只合并同一主键连续 UPDATE 中间状态且不跨 INSERT/DELETE。 |
| FB-030 | frontend-ai | task | closed | MCP 设置与说明书需要按新目标更新：MCP 的目的不是开放网页端口，而是让操作员在边缘机无屏幕或屏幕被拿走时，用另一台电脑上的 AI 客户端受控查看并修改本机 NodeBridge 配置。 | Settings/Manual/`docs/mcp-service.md` 已更新：MCP 默认关闭，启用需管理解锁和完整同步配置，传输为 `stdio`，用户在远程 AI 工具配置 `mcp-client-config`，不开放 HTTP 端口，不宣传自动远程控制；前端只展示后端状态和配置命令，不启动 MCP runtime。 |
| FB-031 | backend-ai | design | closed | MCP 从只读诊断升级为“受控配置修改”需要后端设计：允许远程 AI 修改配置，但必须沿用 Wails 后端同等校验、脱敏、DPAPI/密钥边界和审计记录，不能让 MCP 任意写 YAML 或绕过管理鉴权。 | 后端已实现白名单写配置能力：`nodebridge_validate_config_patch` dry-run、`nodebridge_save_config_patch` 保存非敏感 patch、`nodebridge_save_sync_rules` 保存规则；配置 patch 拒绝 `password/token/security`、原始 YAML 和带密码 AMQP URL；读写从磁盘取最新配置/规则；成功写和拒绝写均记录 `logs/mcp-audit.log`；工具 schema 已明确 `patch/rules` 输入。`mcp-stdio` 启动强制要求 `mcp_server.enable=true`。已更新 `docs/mcp-service.md` 和 contract，测试/linter 通过并重建 `build/bin/SyncAgent.exe`。 |
| FB-032 | test-ai | task | open | 制作 NSIS beta 安装器：把已通过 V0.40.5 的 headless installer with-assets 链路包装为可双击的一键安装程序，覆盖 Erlang/RabbitMQ/Java/Canal/WinSW、NodeBridge/SyncAgent、安装前检查、安装日志、失败摘要和卸载入口。跨身份原因：这是安装器/发布工程任务，完成后必须移交 test-ai 做 VM 和本机 Docker smoke。 | backend-ai 已按实机反馈继续修复：快捷方式显式使用 `app/NodeBridge.ico`；管理端仍由 `wails build -nopackage -o NodeBridge.exe` 生成；RabbitMQ 连通性拆为 local/server 探测，本地成功时远端 `192.168.1.10:5672` 不再导致整体同步配置失败；Config 页保存 MySQL/RabbitMQ/CDC/同步参数后提示重启同步进程生效，并新增 `初始化 RabbitMQ 队列` 入口调用受管拓扑初始化；默认配置改为安装器真实创建的 `nb-edge-001-local/nodebridge_test` 和 encoded vhost `/%2Fnodebridge-edge`；NSIS 安装时会备份并替换 `%ProgramData%\NodeBridge\config.yaml` 中安全草稿/不完整配置；headless component 调用已改为参数数组直接执行并落 stdout/stderr，修复 `C:\Program Files\...` 路径空格导致的 exit code `-196608`。重新生成最终 exe：`build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `947599EA3F2F2620EE0164A6F20E5918EFBD1661FB16176E05B38B5E2972ADD7`；staging `app/NodeBridge.exe`、`app/NodeBridge.ico`、`app/SyncAgent.exe` 存在且 `app/DataSync.exe` 不存在。本机仅编译 exe，未运行 NodeBridge beta 安装器，未执行 Erlang/RabbitMQ/Java/Canal/WinSW 真实系统组件安装。下一步 test-ai 对当前 hash 在隔离 VM 或测试机重跑安装、打开 UI、RabbitMQ 队列初始化、远端不可达提示、二次安装、卸载闭环，并回收 `runtime/*.json` / `*.log`。  2026-09-07 更新：后续安装验证改用 v0.46.0-20260907，hash 见 FB-033；MCP 异机验收见 FB-034。 |

## Frontend V0.26 Plan

1. Rules 页新增 `dispatch_target` 下拉框：`AUTO`、`NONE`、`ACTIVE_EDGES`、`SELECTED_EDGES`。
2. Rules 页新增 `dispatch_node_ids` 输入框；当 `dispatch_target=SELECTED_EDGES` 时显示必填提示。
3. Rules 只读表也要显示分发策略，不能只在编辑态显示。
4. Overview 直接使用 `GetOverview.node_id/node_name/config_loaded/config_path/rules_path`，不再靠 `GetConfig` 拼节点和配置状态。
5. Overview 显示 `cdc_status` 和 `cdc_message`；`configured` 表示已配置但 SyncAgent 未运行。
6. Logs 页继续调用 `GetLogs`；后端已合并 UI ring buffer 和外部 `SyncAgent.exe` 日志。
7. 时间字段按 RFC3339 string 处理，不再按 `time.Time` / `any` 推断。
8. 验收：`npm run build`、Wails contract check、搜索无 `fetch(`/`axios`，并用打包 exe 确认 Overview、Rules、Logs 不再无故空白或 unknown。

## Frontend Requirements Review Approval

后端已审阅并批准当前 `frontend-requirements.md` 作为前端 V0.26 实施范围，附带以下约束：

1. 前端只做 Wails 管理端，不实现同步核心逻辑。
2. Settings 可以加入 MCP Server 开关，但只能调用 Wails 后端接口；默认关闭，不自行启动服务。
3. 任何新字段或方法必须先落到 `frontend-backend-contract.md`，再改页面。
4. `unsupported`、`configured`、`locked`、`error` 必须如实展示，不得显示假成功。
5. Config、Rules、Retry、Diagnostic、AutoStart、Agent control、MCP Server 开关都必须要求管理解锁。

## Board Rules

1. 开工前先声明身份，再读 Active Board，只处理与本轮身份相关或已显式跨身份授权的 open 项。
2. 新问题必须加到 Active Board，分配合法 `Owner`，状态写 `open`。
3. 解决问题时，把对应行状态改为 `closed`，并在 Activity Log 追加一条 answer/decision。
4. 无法推进时，把状态改为 `blocked`，`Next Action` 写清楚缺什么。
5. API、DTO、错误语义、页面范围变化：先更新 Active Board，再同步 `.ai/docs/frontend-backend-contract.md` 或 `.ai/docs/frontend-requirements.md`。
6. 前端、后端、测试、审阅任一身份只改自己范围时，也要检查 Active Board。
7. `test-ai` 必须把验证命令、通过/失败结果和未跑原因写入最终回复；影响发布门禁时写入 Active Board。
8. `review-ai` 必须优先列出风险、缺口和阻塞；需要代码或文档修改时先声明是否临时跨身份执行。
9. 最终回复必须说明本轮身份、处理了哪些 Board 项、哪些仍 open/blocked。
10. 闭合项可保留在 Activity Log；当日志影响阅读时，由 `review-ai` 将闭合历史整理到 `.ai/docs/archive/` 或阶段总结，并保留根级 Active Board 的当前项。

## Activity Log Format

```text
- YYYY-MM-DD HH:mm | <frontend-ai/backend-ai/test-ai/review-ai> | <question/decision/answer/blocker/review/test> | <影响范围> | <open/closed/blocked>
```

## Activity Log

Activity Log 是历史流水，可能保留当时的 `open` 状态；当前真实待办以 Active Board 为准。

- 2026-05-26 16:52 | test-ai | test | FB-025 长测 harness 改为默认 time-interleaved 写入并重建 Docker volume 干净 smoke：18/18 同步、顺序/形态检查通过、队列清空 | open
- 2026-05-26 18:43 | test-ai | blocker | FB-025 1 天等价 day1-interleaved-001：Edge 172,800，Server 停止时 151,951，剩 server.cdc.ingress.q 20,850，失败 ACK 0；阻塞为 Server Apply 吞吐不足 | blocked
- 2026-05-26 22:16 | backend-ai | answer | FB-025 V0.41 完成保序批处理、无下发日志跳过大 payload，并用当前二进制跑通 6,000 行真实 CDC smoke | open
- 2026-05-27 00:42 | test-ai | blocker | FB-025 30 天等价 month30-v041-001：Edge 5,184,000 行生成成功，但 Edge SyncAgent 出现 Canal ACK panic，重启后队列清空而 Server 仅 5,780 行；apply/event log 与业务行数不一致 | blocked
- 2026-05-27 01:35 | backend-ai | answer | FB-025 修复稳定 CDC event_id、Canal ACK/offset 顺序、Server 失败重投和 longtest Canal 显式配置；6,000 行真实 CDC smoke 通过 | open
- 2026-05-27 08:17 | test-ai | test | FB-025 V0.42 `day1-v042-001` 1 天等价通过：Edge/Server 172,800/172,800，顺序 0 违规，队列清空，查询性能均达标；证据在 `.cache/longtest-90d/day1-v042-001/` | open
- 2026-05-27 08:18 | test-ai | blocker | FB-025 V0.42 `month30-v042-001` 30 天等价阻塞：Edge 5,184,000 行完整，Server 停在 69,360 行，apply/event log 均 69,360，失败 ACK 0，最终两侧队列 0；判定为大批量 CDC/offset 完整性问题，证据在 `.cache/longtest-90d/month30-v042-001/` | blocked
- 2026-05-27 09:06 | backend-ai | answer | FB-025 修复 Canal 空 batch 不提交导致 CDC 卡住的问题，补非 ROWDATA offset 保留和空 batch ACK 测试，6,000 行真实 CDC smoke 通过 | open
- 2026-05-27 09:18 | test-ai | blocker | FB-025 V0.43 `month30-v043-001` 30 天等价仍阻塞：Edge 5,184,000 行完整，Server 停在 46,240 行，apply/event log 均 46,240，失败 ACK 0，最终两侧队列 0；已回收 Canal `logs/conf/meta` 与分析文件到 `.cache/longtest-90d/month30-v043-001/` | blocked
- 2026-05-27 09:40 | backend-ai | answer | FB-025 修复 Canal fetch/commit 错误后不重连和客户端 idle timeout 过短，最终 6,000 行 smoke 通过 | open
- 2026-05-27 10:26 | test-ai | test | FB-025 V0.44 `month30-v044-001` 零/冒烟级回归通过旧失败点：Edge 5,184,000 行完整，Server 增至 116,908，越过 V0.43/V0.42 停点，失败 ACK 0；本轮主动停止，未作为 30d 全量验收 | open
- 2026-05-27 11:09 | test-ai | test | FB-025 分层阶段 0/1 通过：`staged-v044-p0-smoke-001` 12/12 通过；`staged-v044-p1-day1-001` Edge=Server=172,800、失败 ACK 0、队列清空、查询达标 | open
- 2026-05-27 11:29 | test-ai | test | FB-025 阶段 2 `staged-v044-p2-day7-001` 已启动但未验收：Edge=1,209,600，Server 暂到 57,066，失败 ACK 0，队列活跃；需无人值守跑满 7d 后才能进 30d | open
- 2026-05-28 08:52 | test-ai | blocker | FB-025 `staged-v044-month30-001` 30 天等价无人值守复测阻塞：Edge=5,184,000 已完成，Server=294,780 后超过 10 小时不增长，`edge.upload.cdc.q=4725`、`server.cdc.ingress.q=1056`，dead/retry=0，失败 ACK 0，`summary.json` 仍为 prepare；证据目录 `.cache/longtest-90d/staged-v044-month30-001/`，现场未清理，转 backend-ai 分析同步停止推进原因 | blocked
- 2026-05-28 08:39 | backend-ai | answer | FB-025 现场复核：runner/SyncAgent 已不在，RabbitMQ 两队列均 ready 且 consumers=0；手动 canal publish/forward/consume 可继续推进，判定主阻塞为长测 harness 静默退出且证据不足，需先修监督脚本再重跑 | blocked
- 2026-05-28 09:34 | test-ai | test | FB-025 已按 backend-ai 意见优化 longtest harness 并启动 30 天等价重测 `staged-v044-month30-rerun-002`：干净 Docker volume，runner PID=40072，证据目录 `.cache/longtest-90d/staged-v044-month30-rerun-002/`，新增 runner stdout/stderr/summary、agent command log、drain fail-fast 和失败证据回收 | open
- 2026-05-30 16:50 | test-ai | test | FB-025 `staged-v044-month30-rerun-002` 30 天等价 seed 通过：Edge/Server 六表均 864,000 行，总计 5,184,000/5,184,000，队列清空，`drain-final.completed=true`，runner summary passed，耗时约 36.99 小时；用户重启后容器为 Exited，但完成证据已写盘 | open
- 2026-05-30 17:07 | test-ai | test | FB-025 优化长测 drain 为 `DrainMode=agents` 常驻 SyncAgent 模式；核心覆盖率门禁 `scripts/test-coverage-core.ps1` 通过 74.9% >= 70%，全仓库原始覆盖率 53.8% 留证；优化后 smoke `smoke-agents-coverage-001` 通过 12/12；30 天积压复测 `staged-v044-month30-agents-001` 已后台启动 PID=33244 | open
- 2026-05-31 01:30 | backend-ai | answer | FB-025 针对 Edge 上传积压释放慢新增 RabbitMQ 批量 publisher confirm：整批顺序发布、整批 confirm 后 ACK，失败整批重投；已通过相关单测、全量测试、vet、核心覆盖率 75.1%，并重建 `build/bin/SyncAgent.exe`。当前后台 30d run 仍是旧进程，需新跑或重启后才体现优化 | open
- 2026-05-31 01:42 | backend-ai | test | FB-025 已在同一 30d 积压现场切换到新 SyncAgent：旧段 Server apply 约 30.9 rows/s，新段约 141.0 rows/s，Server 收到/落库综合速率约 188.0 msg/s，短窗提升约 4.56x-6.08x；新证据 `batchconfirm-samples.csv` / `batchconfirm-analysis.json`。当前新 agents PID=37132/7072 继续运行，瓶颈转到 Server MySQL apply | open
- 2026-05-31 01:54 | backend-ai | test | FB-025 已修复无人值守监控脚本 Windows/Docker 调用兼容问题并重启监控，PID=18336；采样写入 `.cache/longtest-90d/staged-v044-month30-agents-001/batchconfirm-watch.csv`，完成摘要写入 `batchconfirm-watch-summary.json`；当前 Server=1,431,885/5,184,000，agents 37132/7072 存活 | open
- 2026-05-31 01:58 | backend-ai | test | FB-025 巡检确认批量 confirm 后 30d 现场仍在推进：Server=1,451,885/5,184,000，Edge 队列约 565,874，Server ingress=10,000，agents 37132/7072 存活；证据 `manual-snapshot-20260531-015740.json`。后台监控脚本仍受 PowerShell native stderr 影响，当前以手动快照和直接行数核验为准 | open
- 2026-05-31 02:00 | backend-ai | test | FB-025 巡检：Server=1,471,885/5,184,000，Edge 队列约 565,792，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020008.json`，测试继续跑 | open
- 2026-05-31 02:01 | backend-ai | test | FB-025 巡检：Server=1,481,885/5,184,000，Edge 队列约 552,103，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020139.json`，测试继续跑 | open
- 2026-05-31 02:03 | backend-ai | test | FB-025 巡检：Server=1,491,885/5,184,000，Edge 队列约 548,734，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020319.json`，测试继续跑 | open
- 2026-05-31 02:04 | backend-ai | test | FB-025 巡检：Server=1,501,885/5,184,000，Edge 队列约 544,434，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020444.json`，测试继续跑 | open
- 2026-05-31 02:06 | backend-ai | test | FB-025 巡检：Server=1,511,885/5,184,000，Edge 队列约 540,182，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020609.json`，测试继续跑 | open
- 2026-05-31 02:08 | backend-ai | test | FB-025 巡检：Server=1,531,885/5,184,000，Edge 队列约 536,888，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020753.json`，测试继续跑 | open
- 2026-05-31 02:09 | backend-ai | test | FB-025 巡检：Server=1,541,885/5,184,000，Edge 队列约 533,249，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020922.json`，测试继续跑 | open
- 2026-05-31 02:10 | backend-ai | test | FB-025 巡检：Server=1,551,885/5,184,000，Edge 队列约 528,780，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021043.json`，测试继续跑 | open
- 2026-05-31 02:12 | backend-ai | test | FB-025 巡检：Server=1,561,885/5,184,000，Edge 队列约 524,971，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021215.json`，测试继续跑 | open
- 2026-05-31 02:14 | backend-ai | test | FB-025 巡检：Server=1,571,885/5,184,000，Edge 队列约 511,860，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021355.json`，测试继续跑 | open
- 2026-05-31 02:15 | backend-ai | test | FB-025 巡检：Server=1,581,885/5,184,000，Edge 队列约 507,649，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021519.json`，测试继续跑 | open
- 2026-05-31 02:23 | backend-ai | test | FB-025 巡检：Server=1,641,885/5,184,000，Edge 队列约 480,817，Server ingress=10,000，agents 37132/7072 存活；新增并验证 `scripts/longtest-30d-watchdog.ps1`，后台 PID=11172，每 5 分钟写 `watchdog-progress.csv` / `watchdog-summary.json`，测试继续跑 | open
- 2026-05-31 02:25 | backend-ai | test | FB-025 巡检：Server=1,661,885/5,184,000，Edge 队列约 479,094，Server ingress=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-022540.json`，测试继续跑 | open
- 2026-05-31 02:28 | backend-ai | test | FB-025 watchdog 正常采样：Server=1,681,885/5,184,000，Edge 队列约 461,409，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；`watchdog-progress.csv` / `watchdog-summary.json` 已更新，测试继续跑 | open
- 2026-05-31 02:30 | backend-ai | test | FB-025 直连快照：Server=1,701,885/5,184,000，Edge 队列约 459,304，Server ingress=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023039.json`，测试继续跑 | open
- 2026-05-31 02:32 | backend-ai | test | FB-025 直连快照：Server=1,711,885/5,184,000，Edge 队列约 456,368，Server ingress=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023218.json`，测试继续跑 | open
- 2026-05-31 02:33 | backend-ai | test | FB-025 直连快照：Server=1,721,885/5,184,000，Edge 队列约 442,512，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023349.json`，测试继续跑 | open
- 2026-05-31 02:35 | backend-ai | test | FB-025 直连快照：Server=1,731,885/5,184,000，Edge 队列约 438,674，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023522.json`，测试继续跑 | open
- 2026-05-31 02:38 | backend-ai | test | FB-025 直连快照：Server=1,751,885/5,184,000，Edge 队列约 430,289，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023814.json`，测试继续跑 | open
- 2026-05-31 02:39 | backend-ai | test | FB-025 直连快照：Server=1,771,885/5,184,000，Edge 队列约 426,667，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023953.json`，测试继续跑 | open
- 2026-05-31 02:43 | backend-ai | test | FB-025 直连快照：Server=1,791,885/5,184,000，Edge 队列约 420,100，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024303.json`，测试继续跑 | open
- 2026-05-31 02:45 | backend-ai | test | FB-025 直连快照：Server=1,811,885/5,184,000，Edge 队列约 408,274，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024510.json`，测试继续跑 | open
- 2026-05-31 02:46 | backend-ai | test | FB-025 直连快照：Server=1,821,885/5,184,000，Edge 队列约 405,220，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024649.json`，测试继续跑 | open
- 2026-05-31 02:48 | backend-ai | test | FB-025 直连快照：Server=1,831,885/5,184,000，Edge 队列约 401,939，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024825.json`，测试继续跑 | open
- 2026-05-31 02:50 | backend-ai | test | FB-025 直连快照：Server=1,841,885/5,184,000，Edge 队列约 389,840，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025023.json`，测试继续跑 | open
- 2026-05-31 02:52 | backend-ai | test | FB-025 直连快照：Server=1,861,885/5,184,000，Edge 队列约 386,137，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025200.json`，测试继续跑 | open
- 2026-05-31 02:53 | backend-ai | test | FB-025 直连快照：Server=1,871,885/5,184,000，Edge 队列约 383,369，Server ingress=3,711，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025340.json`，测试继续跑 | open
- 2026-05-31 02:55 | backend-ai | test | FB-025 直连快照：Server=1,881,885/5,184,000，Edge 队列约 369,886，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025516.json`，测试继续跑 | open
- 2026-05-31 02:56 | backend-ai | test | FB-025 直连快照：Server=1,891,885/5,184,000，Edge 队列约 366,224，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025651.json`，测试继续跑 | open
- 2026-05-31 02:58 | backend-ai | test | FB-025 直连快照：Server=1,911,885/5,184,000，Edge 队列约 363,437，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025837.json`，测试继续跑 | open
- 2026-05-31 03:00 | backend-ai | test | FB-025 直连快照：Server=1,921,885/5,184,000，Edge 队列约 350,704，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030017.json`，测试继续跑 | open
- 2026-05-31 03:01 | backend-ai | test | FB-025 直连快照：Server=1,928,874/5,184,000，Edge 队列约 347,334，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030154.json`，测试继续跑 | open
- 2026-05-31 03:03 | backend-ai | test | FB-025 直连快照：Server=1,947,751/5,184,000，Edge 队列约 343,964，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030333.json`，测试继续跑 | open
- 2026-05-31 03:05 | backend-ai | test | FB-025 直连快照：Server=1,957,751/5,184,000，Edge 队列约 340,999，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030514.json`，测试继续跑 | open
- 2026-05-31 03:06 | backend-ai | test | FB-025 直连快照：Server=1,967,751/5,184,000，Edge 队列约 328,529，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030659.json`，测试继续跑 | open
- 2026-05-31 03:08 | backend-ai | test | FB-025 直连快照：Server=1,977,751/5,184,000，Edge 队列约 325,152，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030839.json`，测试继续跑 | open
- 2026-05-31 03:10 | backend-ai | test | FB-025 直连快照：Server=1,997,751/5,184,000，Edge 队列约 321,853，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-031018.json`，测试继续跑 | open
- 2026-05-21 18:00 | codex | decision | V0.20 双线协作以 Wails IPC contract 为边界，前端不直接调用 HTTP/RabbitMQ/MySQL | closed
- 2026-05-21 18:00 | codex | decision | 未接真实能力的 Wails 接口返回稳定空状态或 `unsupported`，不能返回假成功 | closed
- 2026-05-21 18:00 | codex | blocker | 当前 shell 无可用 `npm`，前端 build 需先修复 vfox Node/npm 环境 | open
- 2026-05-21 19:45 | codex | decision | 前后端 AI 开工前必须检查 open 项；解决后追加 answer/decision，未解决追加 blocker；最终回复汇报 open/closed | closed
- 2026-05-21 19:47 | codex | decision | 后端每次对话也必须读取协作日志，并主动记录需要前端处理的问题、DTO 变化和阻塞 | closed
- 2026-05-21 20:04 | frontend-ai | answer | 已通过 vfox 重新安装并启用 nodejs@24.15.0，`npm install` 和 `npm run build` 已通过；前端构建阻塞解除 | closed
- 2026-05-21 20:20 | frontend-ai | decision | DataSync 前端界面默认中文并支持中文、英文、日文切换；协议值、DTO 字段、队列名和日志级别仍保持英文 | closed
- 2026-05-21 20:19 | backend-ai | decision | V0.21 不做 Windows Service；改为 Wails 托盘常驻支撑，前端负责托盘 UI，后端提供退出鉴权和当前用户自启动接口 | closed
- 2026-05-21 20:19 | backend-ai | decision | 配置落点确定为 `%ProgramData%\NodeBridge\config.yaml`，密码/token/退出密码使用 DPAPI 加密并对前端脱敏 | closed
- 2026-05-21 20:19 | backend-ai | decision | 新增 Wails 后端方法 `VerifyExitPassword`、`GetAutoStart`、`SetAutoStart`、`ExportDiagnosticPackage`，前端需要按 contract 接入 | closed
- 2026-05-21 20:34 | frontend-ai | answer | 已接入 V0.21 前端托盘控制面、退出密码弹窗、自启动开关、诊断包导出和 `security.exit_password` 配置字段，界面文案保持中英日三语 | closed
- 2026-05-21 20:34 | frontend-ai | blocker | 原生窗口右上角关闭时自动最小化到托盘需要 Wails shell 层 close hook；当前 React 前端已提供隐藏到托盘按钮和退出鉴权弹窗，但无法单独拦截窗口关闭按钮 | open
- 2026-05-21 20:46 | backend-ai | answer | 已在 Wails `OnBeforeClose` 接入 close hook：右上角关闭隐藏到托盘并阻止退出；新增 `RequestExit` 校验退出密码并只放行下一次 `runtime.Quit` | closed
- 2026-05-21 20:46 | backend-ai | decision | 新增管理解锁契约：`UnlockAdmin`、`LockAdmin`、`GetAuthState`；敏感方法未解锁时拒绝，前端必须实现 locked/unlocked UI | closed
- 2026-05-21 20:50 | codex | answer | 已修复根目录 Wails CLI 打包入口，`wails build -clean` 成功产出 `build/bin/DataSync.exe`，并完成原生 exe 进程级启动 smoke | closed
- 2026-05-21 21:02 | backend-ai | decision | 安全字段拆分：`security.admin_password` 负责管理解锁，`security.exit_password` 只负责托盘退出；后端返回两者都脱敏 | closed
- 2026-05-21 21:08 | backend-ai | decision | `SaveConfig` 要求 `security.admin_password` 非空；前端首次配置必须引导用户设置管理密码 | closed
- 2026-05-21 21:36 | backend-ai | answer | 修复前端服务层绑定目标：Wails 生成入口是 `window.go.datasyncui.App`，不是 `window.go.main.App`；这会导致规则、失败记录等页面只能看到空 fallback | closed
- 2026-05-21 21:36 | backend-ai | decision | 根目录 Wails 入口改为 embed `frontend/dist`，避免打包 exe 从 `build/bin` 启动时找不到资源导致白屏 | closed
- 2026-05-21 21:36 | backend-ai | review | 给前端 AI 的具体审阅意见已完成并归入 Activity Log；临时 review notes 文件已删除 | closed
- 2026-05-21 21:58 | backend-ai | decision | `StartAgent`、`StopAgent`、`RestartAgent` 改为控制外部 `SyncAgent.exe` 进程；前端应把 `status=error` 的缺失 exe 提示给用户 | closed
- 2026-05-21 22:10 | frontend-ai | answer | 已按前端审阅意见接入全局 Admin Lock、Unlock 弹窗、敏感操作前置解锁、Rules 新增删除、Failures Retry 解锁和 Config 管理密码必填校验 | closed
- 2026-05-21 22:25 | frontend-ai | question | 当前前端只能通过空 `mode`、空 `node.id`、空 `mysql.database` 推断首次配置；后端是否需要新增 `GetConfigState` 或在 `GetOverview` 暴露 `config_loaded/config_path`，避免长期依赖字段推断 | open
- 2026-05-21 22:25 | frontend-ai | question | Wails 生成绑定时持续输出 `Not found: time.Time`；请后端确认当前前端按 `any/string` 处理时间字段是否可接受，或是否应将 Wails DTO 时间字段改为 RFC3339 string | open
- 2026-05-21 22:25 | frontend-ai | question | `StartAgent` 安装后固定 `SyncAgent.exe` 路径与优雅 shutdown 协议仍需后端 V0.23 决策；前端当前只展示后端返回错误，不自行推断路径或停止协议 | open
- 2026-05-21 22:35 | codex | answer | 已修复 Windows 标题栏 X 直接退出问题：Wails Windows close 事件需要 `HideWindowOnClose=true`，`OnBeforeClose` 继续只负责 `RequestExit` 后的显式退出放行 | closed
- 2026-05-21 22:45 | codex | decision | 修正“隐藏到托盘”误导行为：Wails 2.10.2 公开 `options.App` / `runtime` 未暴露稳定系统托盘创建入口，当前改为窗口最小化/恢复，避免 `WindowHide` 后无托盘入口导致窗口不可恢复 | closed
- 2026-05-21 22:45 | codex | blocker | 真正系统托盘菜单仍需桌面层实现或升级/封装可用 tray API；在此之前前端不再展示“隐藏到托盘”文案 | open
- 2026-05-21 22:55 | codex | answer | 已实现 Windows 原生托盘 helper：`X` 隐藏窗口到系统托盘，托盘双击/菜单可恢复，托盘“退出...”触发前端退出密码弹窗；前端恢复“隐藏到托盘”文案 | closed
- 2026-05-21 22:55 | codex | answer | Rules 页已接入 `source_node_ids` 显示和编辑，前端 SyncRule DTO 同步新增该字段 | closed
- 2026-05-21 22:09 | backend-ai | decision | `SyncRule` 新增 `source_node_ids`，支持多个 Edge 源表同名但中心目标表不同；前端 Rules 页需要显示和编辑该字段 | open
- 2026-05-21 23:51 | backend-ai | answer | V0.24 只新增后端 lab 脚本、迁移和 `dispatch-event-once` CLI；未修改 Wails DTO，前端无需同步改动 | closed
- 2026-05-21 23:51 | backend-ai | answer | `source_node_ids` 前端接入已由 22:55 记录完成，22:09 open 项视为已关闭 | closed
- 2026-05-22 00:01 | backend-ai | decision | 前后端交流收敛为两个核心文件：`frontend-backend-contract.md` 管稳定契约，`ai-collaboration-log.md` 管唯一活跃看板和历史流水 | closed
- 2026-05-22 00:13 | backend-ai | decision | `SyncRule` 新增 `dispatch_target` 和 `dispatch_node_ids`，分发范围改为可配置，前端 Rules 页需要接入 | open
- 2026-05-22 01:21 | backend-ai | answer | 后端已完成 `dispatch_target` / `dispatch_node_ids` 契约、校验、Server dispatch 和 E2E；FB-004 仍由前端接入 Rules UI 控件 | open
- 2026-05-22 01:35 | backend-ai | review | 前端构建和 binding 检查通过；exe 数据 unknown 主要来自后端状态 DTO/规则路径/日志源不完整，已登记 FB-005..FB-008 | open
- 2026-05-22 08:45 | codex | decision | MCP/ClaudeCode 自动化应作为本地管理适配层复用后端 application service，不能直接操作 React、配置文件、RabbitMQ 或 MySQL | open
- 2026-05-22 08:52 | frontend-ai | answer | Rules 页已接入 `dispatch_target` / `dispatch_node_ids` 的只读展示、编辑控件和三语说明，FB-004 关闭 | closed
- 2026-05-22 08:52 | backend-ai | review | 复查当前 RulesPage 代码仍未显示/编辑 `dispatch_target` / `dispatch_node_ids`，FB-004 重新打开并写入 Frontend V0.26 Plan | open
- 2026-05-22 08:52 | backend-ai | answer | V0.26 后端已补 Overview 显式状态、RFC3339 时间、规则 fallback 落盘、CDC configured 状态和外部 SyncAgent 日志读取 | closed
- 2026-05-22 08:54 | backend-ai | answer | 重新确认 RulesPage 已包含 `dispatch_target` / `dispatch_node_ids`，FB-004 保持关闭；前端剩余任务是接 Overview 新字段和 exe 级验收 | closed
- 2026-05-22 09:00 | backend-ai | decision | V0.26 后端规划已固化到 `docs/v0.26-backend-plan.md`，优先处理 FB-003、FB-009、Canal soak、11 节点性能和 exe smoke | open
- 2026-05-22 09:17 | backend-ai | review | 已批复前端需求；批准按现有页面范围推进，新增 MCP Server 开关必须走 Wails 后端接口且默认关闭 | closed
- 2026-05-22 09:17 | backend-ai | decision | 新增 MCP Server 预留接口：仅保存配置开关，不启动真实 MCP runtime，不允许前端绕过管理鉴权 | open
- 2026-05-22 09:30 | frontend-ai | answer | Settings 已接入本地主题偏好和 MCP Server 预留开关，Overview 改用后端显式配置状态，FB-010 关闭 | closed
- 2026-05-22 10:04 | frontend-ai | question | Rules 指定分发目标节点需要 ACTIVE Edge 节点候选项；当前 Wails 契约无节点列表接口，前端只能手填节点 ID | open
- 2026-05-22 10:04 | frontend-ai | answer | Rules 编辑态从宽表改为分组卡片，启用保持滑动开关，方向/分发/冲突保持下拉，降低行高参差和横向拥挤 | closed
- 2026-05-22 10:14 | backend-ai | answer | 已冻结 SyncAgent 查找顺序和 stop-file 停止协议，并新增 `GetAgentProcessStatus()`；FB-003 关闭，前端需接 FB-012 | open
- 2026-05-22 10:30 | backend-ai | decision | 新增受管组件 manifest、RabbitMQ/Canal managed/external 边界和 CDC 安装字段；前端需接 FB-013 | open
- 2026-05-22 10:50 | backend-ai | answer | Config 页已接入 CDC managed/external 安装字段，FB-013 关闭；真实安装执行器仍属后端后续 | closed
- 2026-05-22 11:21 | backend-ai | answer | V0.28 修复并跑通 Edge/Server 真实 Canal E2E 和 20 条 soak；无新增前端契约变化 | closed
- 2026-05-22 11:58 | backend-ai | answer | V0.29 固定目录 package smoke 已通过；无新增前端契约变化，FB-012 仍需前端展示进程状态 | closed
- 2026-05-22 12:14 | backend-ai | decision | V0.30 新增批量失败重试和死信预览 Wails 契约；前端需接入 Failures 页操作入口 | open
- 2026-05-22 12:28 | frontend-ai | answer | Overview 已展示 `GetAgentProcessStatus()` 返回的 SyncAgent 进程路径、PID、日志路径、时间、错误和状态，FB-012 关闭 | closed
- 2026-05-22 12:28 | frontend-ai | answer | Failures 已接入 `RetryFailedEvents` 批量重试和 `GetDeadLetters` 死信只读预览，均先走管理解锁，FB-014 关闭 | closed
- 2026-05-22 13:10 | backend-ai | decision | V0.31 新增受管组件安装计划/执行 Wails 契约和只读 stdio MCP；前端需后续展示安装计划 | open
- 2026-05-22 13:10 | backend-ai | answer | FB-009 已以只读 `mcp-stdio` alpha 收口；MCP 不占端口且不提供任何写工具 | closed
- 2026-05-22 13:31 | backend-ai | answer | V0.32 新增 11 节点 soak/disconnect 后端脚本和诊断摘要；无新增前端契约 | closed
- 2026-05-22 13:45 | backend-ai | answer | V0.33 新增安装器离线包预检和命令计划 CLI；无新增 Wails/前端契约，FB-011 仍 open | closed
- 2026-05-22 13:28 | frontend-ai | answer | Settings 已接入 `GetManagedInstallPlan` / `ApplyManagedInstall`，展示受管组件计划和执行结果，Apply 前要求管理解锁，FB-015 关闭 | closed
- 2026-05-22 13:46 | frontend-ai | answer | 前端完成页面规整与可用性优化：Config 分组编辑、危险操作确认、Settings 分组和开关统一；FB-011 仍等待后端节点候选项接口 | closed
- 2026-05-22 16:34 | review-ai | decision | 明确 AI 身份模型：frontend-ai、backend-ai、test-ai、review-ai 操作前必须声明身份，统一通过 Active Board 协作 | closed
- 2026-05-22 16:43 | backend-ai | answer | 已生成 WinServer2022 Core 用 headless installer test bundle，交给 test-ai 验证；真实 Canal Service 仍待后续实现 | open
- 2026-05-22 16:54 | backend-ai | answer | V0.35 headless installer test bundle 已增强安装、验证、卸载入口；移交 test-ai 等待真实离线包 | blocked
- 2026-05-22 16:50 | test-ai | test | FB-016 默认预检已在 `NodeBridge-V034-InstallerLab-G1` 通过：`admin-check`、`required-files`、`sync-agent-ready`、`canal-check`、`installer-command-plan`、`installer-assets-check`、`managed-plan`、`managed-apply-safe` 均 passed；`real-install` 因缺真实离线包按预期 skipped，证据在 `.cache/v0.34-headless-preflight/` | blocked
- 2026-05-22 16:56 | review-ai | decision | 活跃 AI 看板迁移到根级 `AI_BOARD.md`，与 `MEMORY.md` 同级；`.ai/docs/ai-collaboration-log.md` 改为迁移提示，`.ai/docs/` 只放稳定文档和归档材料 | closed
- 2026-05-22 17:06 | test-ai | test | FB-016 V0.35 版本验证已在 `NodeBridge-V034-InstallerLab-G1` 执行：`build/NodeBridge-headless-installer-test-v0.35.0.zip` 解压后 `package-summary.version=0.35.0`，默认预检 exit 0 且 `managed-plan/managed-apply` manifest 版本为 `0.35.0`；`-VerifyOnly` 在未安装状态下正确失败并记录 `NodeBridgeRabbitMQ` 缺失，证据在 `.cache/v0.35-headless-version-verify/` | blocked
- 2026-05-22 17:17 | backend-ai | answer | 新增只读 `GetNodeOptions()` Wails 契约，FB-011 关闭；Rules 候选多选交给 frontend-ai 处理 FB-017 | open
- 2026-05-22 17:29 | backend-ai | answer | V0.36 headless installer 包已生成并通过默认预检；真实安装闭环仍等待离线包，FB-016 保持 blocked | blocked
- 2026-05-22 17:26 | frontend-ai | answer | Rules 页已接入 `GetNodeOptions()`：SELECTED_EDGES 时显示 ACTIVE Edge 候选勾选、状态消息和手填兜底，FB-017 关闭 | closed
- 2026-05-22 17:45 | test-ai | test | FB-016 V0.36 已在 `NodeBridge-V034-InstallerLab-G1` 验证：默认预检 exit 0，summary/package/manifest 版本均为 `0.36.0`；`prepare-installer-assets-catalog.ps1` 缺包 exit 1、假包生成 4 项 catalog；未安装状态 `-VerifyOnly` 正确失败，双次 `-Uninstall` 均 passed；证据在 `.cache/v0.36-headless-preflight/` 和 `.cache/v0.36-headless-vm-validation/` | blocked
- 2026-05-22 17:55 | frontend-ai | answer | 新增 `docs/frontend-wails-dev.md`，固化 Wails dev 启动、前端门禁和 smoke 检查流程 | closed
- 2026-05-22 17:57 | backend-ai | answer | FB-016 官方 Erlang/RabbitMQ/Canal/WinSW 离线包已下载并生成带真实资产的 V0.36 测试包，移交 test-ai 做 VM 真实安装闭环 | open
- 2026-05-22 18:00 | frontend-ai | answer | 补强 `docs/frontend-wails-dev.md`：加入完整可复制启动命令、绝对路径启动、逐行解释和快速工具检查 | closed
- 2026-05-22 18:17 | frontend-ai | answer | Settings 页恢复为稳定分区布局，避免标题、普通卡片、宽表和危险操作在同一网格中混排 | closed
- 2026-05-22 18:31 | frontend-ai | review | 已完成 1366x768、1600x900、1920x1080 下 8 个前端页面视觉审阅，报告见 `docs/frontend-visual-review-16x9.md` | closed
- 2026-05-22 18:48 | frontend-ai | answer | 根据 16:9 审阅报告优化前端：Config 长值换行、空状态增强、Overview 操作分组、Rules 状态对齐、Manual 限宽、Settings 退出按钮降权，并完成 24 张复测截图 | closed
- 2026-05-22 23:25 | frontend-ai | answer | 使用用户提供图片生成透明背景 NodeBridge 图标，接入 Wails `build/appicon`、前端 favicon 和 Windows 托盘 HICON 数据 | closed
- 2026-05-22 23:43 | frontend-ai | review | 按 Wails 默认 `1100x720` 尺寸完成 8 页截图复查；设计语言适合但默认尺寸布局需优化 Rules 横向溢出、空状态和 Settings 错误区 | closed
- 2026-05-22 23:50 | frontend-ai | answer | 已按默认 `1100x720` 复查结果迭代：Rules 横向溢出修复，Queues/Failures/Logs 空状态结构化，Settings 错误提示降权，Config 默认分栏收敛 | closed
- 2026-05-23 00:07 | frontend-ai | answer | Settings 安全区已说明无初始密码、首次设置、忘记密码重置方式；已替换 `build/windows/icon.ico` 并验证 `DataSync.exe` / `DataSync-dev.exe` 图标不再是默认 W | closed
- 2026-05-23 00:14 | frontend-ai | answer | Overview 首屏移除配置/规则/Agent 路径，路径下沉到 Settings 诊断位置；密码忘记说明改为不可解密找回，只能清空加密字段或重建配置后重新设置 | closed
- 2026-05-23 00:14 | frontend-ai | question | 密码忘记后的产品化恢复流程需要后端评估本机管理员安全重置 CLI/恢复模式，已登记 FB-019 | open
- 2026-05-23 00:21 | frontend-ai | answer | MCP 开关失败原因确认为本机未加载 `%ProgramData%\NodeBridge\config.yaml`；Settings 已禁用未配置状态下的 MCP 开关并补充临时 stdio 语义；说明书新增忘记密码恢复章节 | closed
- 2026-05-23 00:23 | frontend-ai | blocker | 首次只设置管理密码不会创建配置文件，根因是后端 `SaveConfig` 要求完整同步配置校验通过才落盘；已登记 FB-020 交给 backend-ai | open
- 2026-05-23 00:41 | frontend-ai | answer | 用户可见产品名统一为英文 `NodeBridge`，移除说明书/设置/HTML title 中的 `DataSync` 和中文名，并同步 Wails 输出文件名 | closed
- 2026-05-22 23:36 | test-ai | blocker | FB-016 V0.36 真实资产 `-ExecuteInstall` 已在隔离 VM 执行但阻塞：仅发现默认 `RabbitMQ` 服务运行，未发现 `NodeBridgeRabbitMQ`；`rabbitmqctl status/list_*` 因 Erlang cookie 不一致认证失败；真实安装 summary 未覆盖预检 summary。证据在 `.cache/v0.36-real-install-failure/` | blocked
- 2026-05-23 09:10 | backend-ai | answer | FB-016 V0.37 修复安装器：复用已有 RabbitMQ 且不删除客户服务、同步 Erlang cookie、VerifyOnly 校验 NodeBridge 逻辑资源，重新移交 test-ai | open
- 2026-05-23 09:24 | backend-ai | decision | MCP Service 开关改为本次会话临时启用，默认关闭且重启自动关闭；前端需更新 Settings/通知设置文案和状态展示 | open
- 2026-05-23 09:20 | test-ai | blocker | FB-016 V0.37 真实资产 `-ExecuteInstall` 已在隔离 VM 执行；既有 RabbitMQ 复用、cookie 同步和真实 summary 均通过，但 `rabbitmq-bootstrap` 因 AMQP vhost 访问 403 失败，证据在 `.cache/v0.37-real-install-failure/` | blocked
- 2026-05-23 00:39 | test-ai | blocker | FB-016 按用户要求恢复到无 RabbitMQ 快照后重跑 V0.37 完整安装；28 分钟后中断检查发现只完成 Erlang/RabbitMQ 默认服务安装，未写 summary，未进入 cookie/bootstrap 阶段，证据在 `.cache/v0.37-clean-install-interrupted/` | blocked
- 2026-05-23 01:21 | test-ai | blocker | FB-016 V0.38 已恢复 `Clean-Windows-Installed` 干净快照验证完整安装路径；确认无 RabbitMQ/Erlang/NodeBridge 后 `-ExecuteInstall` 在 `install-erlang` 失败，安装进程未超时但 ExitCode 为 null，证据在 `.cache/v0.38-clean-install-failure/` | blocked
- 2026-05-23 09:33 | backend-ai | decision | FB-019 按产品决策闭合：忘记密码由用户备份并手动清空配置文件 security 加密字段，不做后端重置 CLI | closed
- 2026-05-23 09:33 | backend-ai | answer | FB-020 已修复：`SaveConfig` 支持仅含 security 的首次安全草稿落盘，完整同步配置校验不放松 | closed
- 2026-05-23 00:53 | backend-ai | answer | FB-016 V0.38 修复 RabbitMQ 安装器闭环并生成基础包和 with-assets 包，移交 test-ai 做 VM 验证 | open
- 2026-05-25 08:41 | test-ai | blocker | FB-016 V0.38.1 在干净 VM 首次安装确认 ExitCode null 修复生效，但 RabbitMQ ready 90s 超时；同一状态二次安装可完成 RabbitMQ bootstrap，但 Canal WinSW 服务 StartPending 后停止，ExitCode=1067，证据分别在 `.cache/v0.38.1-clean-install-failure/` 和 `.cache/v0.38.1-clean-rerun-canal-failure/` | blocked
- 2026-05-25 10:02 | test-ai | blocker | FB-016 V0.38.2 干净 VM 第一轮 `-ExecuteInstall`、`-VerifyOnly` 和 `-Uninstall` 通过；二次 `-ExecuteInstall` 在 `rabbitmq-bootstrap` 失败且 message 仅 `Error:`，实际 RabbitMQ vhost/queue 已存在，证据在 `.cache/v0.38.2-clean-second-install-failure/` 和 `.cache/v0.38.2-clean-validation/` | blocked
- 2026-05-25 10:28 | test-ai | test | FB-016 V0.38.3 已在 `Clean-Windows-Installed` 干净 VM 跑通 `-ExecuteInstall`、`-VerifyOnly`、二次 `-ExecuteInstall`、`-Uninstall`；卸载后无 RabbitMQ/NodeBridge/Canal 服务，证据在 `.cache/v0.38.3-clean-validation/` | closed
- 2026-05-25 10:35 | backend-ai | answer | FB-024 V0.39.0 新增可选 Java/JRE 离线资产支持，生成基础包与 with-assets 包；强验 Canal Service 需补 Java MSI | open
- 2026-05-25 11:27 | backend-ai | answer | FB-024 V0.40.0 下载 Java/JRE MSI，新增带资产打包脚本并生成可强验 Canal Service 的 with-assets 包 | open
- 2026-05-26 10:32 | test-ai | blocker | FB-024 V0.40.0 第一轮 `-ExecuteInstall -RequireCanalService` 已在 `Clean-Windows-Installed` 干净 VM 执行，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 Java MSI 前置安装未产生 exit code 且 `java.exe` 未安装，导致 `canal-service` 失败；证据在 `.cache/v0.40.0-clean-validation/` | blocked
- 2026-05-26 11:19 | backend-ai | answer | FB-024 V0.40.1 扩大 Java 探测范围、增强 MSI 日志和 probe 重试，生成新 with-assets 包移交 test-ai | open
- 2026-05-26 14:44 | test-ai | blocker | FB-024 V0.40.1 第一轮 `-ExecuteInstall -RequireCanalService` 已在 `Clean-Windows-Installed` 干净 VM 执行，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 Java MSI 安装失败；`msi-java.log` 返回 1620 且有 2203/-2147286960，`java.exe` 未安装，证据在 `.cache/v0.40.1-clean-validation/` | blocked
- 2026-05-26 15:02 | backend-ai | answer | FB-024 V0.40.2 改为 Java/JRE zip 解压式资产，重新生成 with-assets 包移交 test-ai 强验 Canal Service | open
- 2026-05-26 15:28 | test-ai | blocker | FB-024 V0.40.2 第一轮 `-ExecuteInstall -RequireCanalService` 已在 `Clean-Windows-Installed` 干净 VM 执行，Java zip 解压和探测通过，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 NodeBridgeCanal 启动后停止；日志显示 JRE 17 不识别 `PermSize=128m`，证据在 `.cache/v0.40.2-clean-validation/` | blocked
- 2026-05-26 15:47 | backend-ai | answer | FB-024 V0.40.3 清洗 Canal startup 旧 PermGen JVM 参数并生成 with-assets 包，移交 test-ai 复测 Canal Service 强验 | open
- 2026-05-26 16:04 | test-ai | blocker | FB-024 V0.40.3 第一轮 `-ExecuteInstall -RequireCanalService` 和 `-VerifyOnly` 已通过，二次 `-ExecuteInstall -RequireCanalService` 阻塞在 `canal-asset-extract`，运行中的 Canal 文件 `lib/canal.server-1.1.8.jar` 无法 unlink 覆盖；证据在 `.cache/v0.40.3-clean-validation/` | blocked
- 2026-05-26 16:23 | backend-ai | answer | FB-024 V0.40.4 修复 Canal 运行中二次安装热覆盖，生成新 with-assets 包移交 test-ai | open
- 2026-05-26 16:42 | test-ai | blocker | FB-024 V0.40.4 第一轮安装、首次验证、二次安装、二次验证均通过；`-Uninstall` 失败在 `verify-uninstall`，message=`NodeBridgeCanal still exists`，延迟 10 秒后服务消失；证据在 `.cache/v0.40.4-clean-validation/` | blocked
- 2026-05-26 16:47 | backend-ai | answer | FB-024 V0.40.5 修复卸载服务删除等待确认并生成新 with-assets 包移交 test-ai | open
- 2026-05-26 17:39 | test-ai | test | FB-024 V0.40.5 在 `Clean-Windows-Installed` 干净 VM 通过完整闭环：安装、验证、二次安装、二次验证、卸载、二次卸载均通过，卸载后无 RabbitMQ/NodeBridge/Canal 服务；证据在 `.cache/v0.40.5-clean-validation/` | closed
- 2026-05-31 03:24 | backend-ai | answer | FB-025 新增按表 `sync_mode`：`crud_ordered` 保持原保序 CRUD，`append_only` 用于历史倾倒/采集流水表并启用 Server 多行批量插入；长测 6 张采集表已切到 `append_only`，`appendonly-smoke-001` 18/18 通过，新 30 天复测 `appendonly-month30-001` 后台 PID=8536 已进入 seed，早期 Edge=1,040,000/5,184,000 | open
- 2026-05-31 04:09 | backend-ai | answer | FB-025 继续定位瓶颈后确认 Canal->Edge RabbitMQ 仍逐条 publish，已改为 `PublishBatch` 成功后再提交 Canal offset；Server append-only apply 同时支持 interleaved 批次按目标表分组。验证：`go test ./...`、`go vet ./...`、核心覆盖率 74.7% >= 70%；`appendonly-month30-003` Docker RabbitMQ prepare 异常已清理，最终后台复测为 `appendonly-month30-004` PID=41432，早期 Edge=290,000/5,184,000 | open
- 2026-05-31 04:32 | backend-ai | answer | FB-025 已把 `sync_mode` 补到 Rules 页面和 Manual：按表选择 `crud_ordered` 或 `append_only`，历史倾倒/采集流水表可用 append-only，少量增删改表继续保序 CRUD；验证 `go test ./internal/rules ./internal/mapper ./internal/apply ./internal/syncruntime` 与 `frontend npm run build` 通过。现场 `appendonly-month30-004` 仍未通过：Edge=5,184,000，Server=0，Server ingress 约 150,000 且无落库事务，需继续定位 Server batch apply 卡点 | open
- 2026-05-31 04:34 | backend-ai | blocker | FB-025 `appendonly-month30-004` 已停止 runner/agents，保留 Docker 现场和 `.cache/longtest-90d/appendonly-month30-004/` 证据；该轮不计通过。下一步需在 Server batch apply 增加可观测日志/批次限流后重测，避免 Server 端拿到大批量消息但不提交落库 | blocked
- 2026-05-31 04:40 | backend-ai | answer | FB-025 根因确认并修复：append-only 多行 INSERT / sync_apply_log 批量写入超过 MySQL prepared statement 占位符上限，触发 `Error 1390` 后整批 requeue；已按 `maxStatementPlaceholders=60000` 自动切片。现场验证：旧失败现场 `max-batch=2000` 耗时 2.54s applied，`max-batch=10000` 耗时 11.98s applied。门禁：`go test ./...`、`go vet ./...`、核心覆盖率 74.8%、`frontend npm run build` 通过。新 30d 复测 `appendonly-month30-005` 已从干净 Docker volume 启动，PID=17136，证据 `.cache/longtest-90d/appendonly-month30-005/`，04:39 Edge=130,000/5,184,000，Server=0，正在 seed | open
- 2026-05-31 04:42 | backend-ai | answer | FB-025 `appendonly-month30-005` seed 阶段稳定推进，runner PID=17136 存活；04:42 快照 Edge=1,030,000/5,184,000，Server=0，队列为空，尚未进入 agents drain。证据目录 `.cache/longtest-90d/appendonly-month30-005/` | open
- 2026-05-31 04:58 | backend-ai | answer | FB-025 `appendonly-month30-005` 已完成 seed 并进入 agents drain：Edge=5,184,000，Server=90,000，Edge agent PID=3816、Server agent PID=12216 均存活，Server stderr 为空；Edge upload 队列约 1,118,020，Server ingress 10,000 unacked，说明修复后的 Server apply 正在推进。已启动 watchdog PID=40084，每 5 分钟写 `watchdog-progress.csv` / `watchdog-summary.json` | open
- 2026-05-31 05:05 | backend-ai | answer | FB-025 `appendonly-month30-005` drain 稳定推进：实时快照 Edge=5,184,000、Server=290,000；watchdog 05:03 样本 Server=250,000、Edge upload=2,905,880、Server ingress=10,000、agent_count=2、status=running；dead/retry 队列为 0。预计仍需数小时追平，继续后台运行 | open
- 2026-05-31 05:10 | backend-ai | answer | FB-025 `appendonly-month30-005` 继续推进：实时 Server=480,000/5,184,000；watchdog 05:08 样本 Server=410,000、Edge upload=4,709,923、Server ingress=0、agent_count=2、status=running；当前 Edge upload=4,634,000、Server ingress=70,000、dead/retry=0。修复后无 `Error 1390` 复发，继续后台 drain | open
- 2026-05-31 05:12 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=560,000；Edge upload=4,474,000、Server ingress=150,000，dead/retry=0。当前按表 `sync_mode` 设计确认：历史倾倒/采集流水表用 `append_only` 加速，低频增删改查表继续用默认 `crud_ordered` 保序 | open
- 2026-05-31 05:13 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：runner PID=17136、watchdog PID=40084、Edge/Server agents PID=3816/12216 均存活；实时 Edge=5,184,000、Server=610,000，Edge upload=4,374,000、Server ingress=200,000，dead/retry=0，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:19 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=850,000；Edge upload=3,884,000、Server ingress=460,000，dead/retry=0。watchdog 05:18 样本 Server=790,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:21 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：runner/watchdog/agents 仍存活；实时 Edge=5,184,000、Server=890,000，Edge upload=3,784,000、Server ingress=510,000，dead/retry=0；watchdog 文件最新仍为 05:18 样本，实时 MySQL/RabbitMQ 证明同步继续增长 | open
- 2026-05-31 05:32 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,260,000，Edge upload=2,914,000、Server ingress=1,010,000，dead/retry=0。watchdog 05:28 样本 Server=1,140,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:33 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,290,000，Edge upload=2,834,000、Server ingress=1,060,000，dead/retry=0；runner/watchdog/agents 仍存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:34 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,320,000，Edge upload=2,754,000、Server ingress=1,110,000，dead/retry=0；watchdog 05:33 样本 Server=1,300,000、agent_count=2、status=running，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:35 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,350,000，Edge upload=2,674,000、Server ingress=1,160,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:36 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,380,000，Edge upload=2,604,000、Server ingress=1,210,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:37 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,400,000，Edge upload=2,534,000、Server ingress=1,260,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:38 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,430,000，Edge upload=2,454,000、Server ingress=1,300,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:39 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,460,000，Edge upload=2,384,000、Server ingress=1,353,201，dead/retry=0；watchdog 05:38 样本 Server=1,450,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:39 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,480,000，Edge upload=2,324,000、Server ingress=1,390,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:40 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,510,000，Edge upload=2,244,000、Server ingress=1,440,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:41 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,530,000，Edge upload=2,174,000、Server ingress=1,490,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:42 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,560,000，Edge upload=2,094,000、Server ingress=1,530,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:43 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,590,000，Edge upload=2,014,000、Server ingress=1,590,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:44 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,610,000，Edge upload=1,934,000、Server ingress=1,640,000，dead/retry=0；watchdog 05:43 样本 Server=1,590,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:45 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,640,000，Edge upload=1,859,986、Server ingress=1,700,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:46 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,660,000，Edge upload=1,784,000、Server ingress=1,740,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:47 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,690,000，Edge upload=1,694,000、Server ingress=1,800,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:48 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,720,000，Edge upload=1,614,000、Server ingress=1,850,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:49 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,750,000，Edge upload=1,514,000、Server ingress=1,920,000，dead/retry=0；watchdog 05:48 样本 Server=1,720,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:51 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,780,000，Edge upload=1,444,000、Server ingress=1,960,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:51 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,800,000，Edge upload=1,374,000、Server ingress=2,010,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:52 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,820,000，Edge upload=1,314,000、Server ingress=2,050,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:53 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,850,000，Edge upload=1,234,000、Server ingress=2,110,000，dead/retry=0；watchdog 05:53 样本 Server=1,850,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:54 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,880,000，Edge upload=1,154,000、Server ingress=2,160,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:55 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,900,000，Edge upload=1,074,000、Server ingress=2,210,000，dead/retry=0；按表 `sync_mode` 继续作为产品策略：历史倾倒表走 `append_only` 批量加速，增删改查表保留 `crud_ordered` 保序 | open
- 2026-05-31 05:57 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,950,000，Edge upload=934,000、Server ingress=2,300,988，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成，继续后台 drain | open
- 2026-05-31 05:58 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,970,000，Edge upload=864,000、Server ingress=2,350,000，dead/retry=0；agents CPU 仍增长但 watchdog 文件暂无新采样，测试未完成，继续后台 drain | open
- 2026-05-31 05:59 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,000,000，Edge upload=784,000、Server ingress=2,400,834，dead/retry=0；watchdog 已恢复采样，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:00 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,030,000，Edge upload=704,000、Server ingress=2,460,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:01 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,050,000，Edge upload=634,000、Server ingress=2,500,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:02 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,070,000，Edge upload=564,000、Server ingress=2,550,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:03 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,090,000，Edge upload=494,000、Server ingress=2,600,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:04 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,120,000，Edge upload=424,000、Server ingress=2,644,320，dead/retry=0；watchdog 06:03 样本 Server=2,110,000，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:05 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,140,000，Edge upload=344,000、Server ingress=2,700,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:05 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,170,000，Edge upload=274,000、Server ingress=2,750,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:06 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,190,000，Edge upload=204,000、Server ingress=2,790,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:07 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,210,000，Edge upload=134,000、Server ingress=2,840,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:08 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,240,000，Edge upload=54,000、Server ingress=2,900,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:09 | test-ai | test | FB-025 `appendonly-month30-005` 已进入 Server 单独 drain 阶段：实时 Edge=5,184,000、Server=2,260,000，Edge upload=0、Server ingress=2,934,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:10 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,280,000，Edge upload=0、Server ingress=2,904,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:11 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,310,000，Edge upload=0、Server ingress=2,874,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:12 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,340,000，Edge upload=0、Server ingress=2,854,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:13 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,360,000，Edge upload=0、Server ingress=2,824,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:14 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,380,000，Edge upload=0、Server ingress=2,804,000，dead/retry=0；watchdog 06:14 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:15 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,410,000，Edge upload=0、Server ingress=2,774,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:16 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,430,000，Edge upload=0、Server ingress=2,754,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:17 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,460,000，Edge upload=0、Server ingress=2,724,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:18 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,490,000，Edge upload=0、Server ingress=2,701,368，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:19 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,510,000，Edge upload=0、Server ingress=2,674,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:20 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,540,000，Edge upload=0、Server ingress=2,654,000，dead/retry=0；watchdog 06:19 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:20 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,560,000，Edge upload=0、Server ingress=2,624,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:21 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,590,000，Edge upload=0、Server ingress=2,604,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:22 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,610,000，Edge upload=0、Server ingress=2,574,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:23 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,640,000，Edge upload=0、Server ingress=2,554,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:24 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,660,000，Edge upload=0、Server ingress=2,524,000，dead/retry=0；watchdog 06:24 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:25 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,690,000，Edge upload=0、Server ingress=2,494,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:28 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,780,000，Edge upload=0、Server ingress=2,414,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:30 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,800,000，Edge upload=0、Server ingress=2,384,000，dead/retry=0；watchdog 06:29 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:31 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,840,000，Edge upload=0、Server ingress=2,354,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:32 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,870,000，Edge upload=0、Server ingress=2,324,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:33 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,900,000，Edge upload=0、Server ingress=2,294,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:35 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,960,000，Edge upload=0、Server ingress=2,234,000，dead/retry=0；watchdog 06:34 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:37 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,980,000，Edge upload=0、Server ingress=2,204,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:39 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,030,000，Edge upload=0、Server ingress=2,154,000，dead/retry=0；`append_only` 作为历史倾倒表模式继续验证，CRUD 表仍保留 `crud_ordered` 保序链路 | open
- 2026-05-31 06:40 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,060,000，Edge upload=0、Server ingress=2,124,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:45 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,210,000，Edge upload=0、Server ingress=1,984,000，dead/retry=0；近 5.6 分钟增加约 150,000 行，约 26,800 行/分钟，按当前速度仍需约 74 分钟 | open
- 2026-05-31 06:56 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,480,000，Edge upload=0、Server ingress=1,714,000，dead/retry=0；近 10.7 分钟增加约 270,000 行，约 25,300 行/分钟，agent stderr 为空 | open
- 2026-05-31 07:12 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,870,000，Edge upload=0、Server ingress=1,314,000，dead/retry=0；近 15.6 分钟增加约 390,000 行，约 25,000 行/分钟，runner/watchdog/agents 存活且 agent stderr 为空 | open
- 2026-05-31 07:27 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,250,000，Edge upload=0、Server ingress=934,000，dead/retry=0；近 15.5 分钟增加约 380,000 行，约 24,500 行/分钟，agent stderr 为空 | open
- 2026-05-31 07:43 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,640,000，Edge upload=0、Server ingress=554,000，dead/retry=0；近 15.6 分钟增加约 390,000 行，约 25,000 行/分钟，进入最后 55 万后改为 5 分钟采样 | open
- 2026-05-31 07:48 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,770,000，Edge upload=0、Server ingress=414,000，dead/retry=0；近 5.5 分钟增加约 130,000 行，约 23,500 行/分钟，agent stderr 为空 | open
- 2026-05-31 07:54 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,910,000，Edge upload=0、Server ingress=274,000，dead/retry=0；近 5.6 分钟增加约 140,000 行，约 25,000 行/分钟，agent stderr 为空 | open
- 2026-05-31 08:00 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 接近尾段：实时 Edge=5,184,000、Server=5,040,000，Edge upload=0、Server ingress=144,000，dead/retry=0；进入最后 3% 后改为 2 分钟短轮询 | open
- 2026-05-31 08:07 | test-ai | test | FB-025 `appendonly-month30-005` 30 天量级复测通过：runner-summary `passed`，drain-final `completed=true`；Edge/Server 总数均 5,184,000，6 张表各 864,000；Edge upload/retry/dead/downlink 与 Server ingress/dead/downlink 队列全 0；`sync_apply_log=5,184,000`、`sync_event_log=5,184,000` 且 payload 全 NULL、`sync_ack_log=0`，agent stderr 与错误关键字扫描为空；已停止残留 watchdog | closed
- 2026-05-31 08:43 | test-ai | handoff | FB-026 用户暂停 15 天混合断网压测，已停止 `mixed-smoke-001` 残留 PowerShell/SyncAgent 进程；前端联调关注 `sync_mode` 规则字段：可选 `crud_ordered`（默认，增删改查保序）与 `append_only`（历史倾倒/采集流水，只接受 INSERT，UPDATE/DELETE 会由后端拒绝并走现有失败链路）。当前前端已有 Rules 下拉、Manual 文案和 Wails DTO 字段，但需要 frontend-ai 联调确认：新增规则默认 `crud_ordered`、编辑/保存不丢 `sync_mode`、`append_only` 显示危险提示、三语文案准确，并用后端真实规则保存/读取闭环验证 | closed
- 2026-05-31 08:55 | frontend-ai | answer | FB-026 已完成：Rules 新增规则默认 `crud_ordered`；编辑态和只读态展示 `sync_mode`；`append_only` 追加中英日危险提示；DTO 保存字段保持透传；前端门禁通过 | closed
- 2026-06-01 09:04 | test-ai | test | FB-027 按用户要求启动 15 天混合断网压力测，不持续盯进度：无效首轮 `mixed-15d-offline-001` 已停止，原因是先停 Server RabbitMQ 再启动 Edge agent 会导致 Edge agent 首连失败；已修正 harness 为先启动 Edge agent 再停 Server RabbitMQ。有效 run 为 `mixed-15d-offline-002`，runner PID=41444、Edge agent PID=29348，证据目录 `.cache/longtest-90d/mixed-15d-offline-002/`；测试计划为 5 张 `tag_state_*` 保序 CRUD 表 + 12 张 `collect_data_*` append-only 表，15 天，预期业务行 5,189,000、预期事件 7,349,000。启动确认：Server RabbitMQ 已停止模拟断网，Edge agent 存活且 stderr 为空，Edge 当前 append_rows=80,000、tag_rows=5,000，测试正在运行 | open
- 2026-06-01 09:10 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows 从 120,000 增至 240,000、tag_rows=5,000，Edge upload 队列从 169,604 增至 299,264，retry/dead=0；runner/agent 日志关键字扫描未见 panic/fatal/Error 1390/FAILED，当前为正常积压生成阶段 | open
- 2026-06-01 10:31 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=1,560,000/5,184,000，tag_rows=5,000，Server 业务行仍 0，Edge upload 队列=2,199,994、retry/dead=0；runner.err 仅 MySQL password warning，edge.err 为空，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，当前仍在断网积压生成阶段 | open
- 2026-06-01 14:05 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=3,480,000/5,184,000，tag_rows=5,000，Server 业务行仍 0，Edge upload 队列=4,904,324、retry/dead=0；runner.out 最新进度 `rows_each_append=200000/432000`，日志关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，仍在断网积压生成阶段 | open
- 2026-06-01 15:54 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=4,320,000/5,184,000，tag_rows=5,000，Server 业务行仍 0；Edge upload 队列=6,081,325/预期事件 7,349,000，retry/dead=0；runner.err 仍为 MySQL password warning，edge.err 为空，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，仍在断网积压生成阶段 | open
- 2026-06-01 17:04 | test-ai | test | FB-027 用户要求“灌完数据先停，明天再测恢复”：当前 `mixed-15d-offline-002` 仍在断网灌数，runner PID=41444、Edge agent PID=29348 均存活；Edge append_rows=4,800,000/5,184,000，tag_rows=5,000，Edge upload 队列约 6,793,892/7,349,000，tag 版本约 389k-400k/432k。已启动 pause monitor PID=37368，每 10 秒检查；达到 append_rows=5,184,000 且 edge_upload>=7,349,000 后会自动停止 runner 和 Edge agent，并保持 Server RabbitMQ stopped，写 `.cache/longtest-90d/mixed-15d-offline-002/pause-after-seed.json` | open
- 2026-06-01 17:18 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002` 暂停前状态：runner PID=41444、Edge agent PID=29348、pause monitor PID=37368 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=4,920,000/5,184,000，还差 264,000；Edge upload 队列=6,959,575/7,349,000，还差 389,425；pause marker 尚未生成，monitor 正常采样；retry/dead=0，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED | open
- 2026-06-01 17:59 | test-ai | test | FB-027 `mixed-15d-offline-002` 已按用户要求停在“离线积压已灌完、尚未恢复同步”状态：`pause-after-seed.json` 17:55:42 生成，append_rows=5,184,000、edge_upload_depth=7,349,000，target reached 后已停止 runner PID=41444 和 Edge agent PID=29348；Server RabbitMQ 仍 stopped，Edge upload.cdc.q=7,349,000 ready / 0 unacked，retry/dead=0，Server 业务行仍 0。明天可从该现场启动 Server RabbitMQ 和 agents 做恢复 drain | open
- 2026-06-02 14:12 | test-ai | test | FB-027 重启后复核 `mixed-15d-offline-002` 恢复测试前置条件：先只拉起 Edge/Server MySQL、Edge RabbitMQ、Edge Canal，保持 Server RabbitMQ stopped；Edge RabbitMQ 用约 107 秒完成 7,349,000 条持久消息索引重建，队列为 edge.upload.cdc.q=7,349,000 ready / 0 unacked / 0 consumers，retry/dead=0；Edge append_rows=5,184,000、tag_rows=5,000，Server 业务行仍 0；无残留 runner/agent/pause monitor，日志关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED。可以启动恢复 drain，不需要重灌数据 | open
- 2026-06-02 14:23 | test-ai | test | FB-027 已按用户要求启动 `mixed-15d-offline-002` 恢复 drain：第一次控制器因 Server RabbitMQ 未实际启动导致 init 拓扑失败，未启动 agents、未消费积压；随后手动启动 Server RabbitMQ、初始化 Server 拓扑成功，并启动修正后的恢复控制器 PID=22524、Edge/Server SyncAgent PID=23644/4228。14:23 样本显示恢复已运行：ServerRows=70,000、ApplyLog=70,000、Edge upload=7,177,118、Server ingress=97,092，当前 controller-2/edge/server stderr 均为空；日志在 `.cache/longtest-90d/mixed-15d-offline-002/agents-recovery/`，进度在 `recovery-progress.csv` / `recovery-summary.json` | open
- 2026-06-02 16:20 | review-ai | decision | FB-028 已将 Server Apply 成倍性能优化计划交给 backend-ai：优先分流 `append_only` 与 `crud_ordered`，append-only 批量业务写和批量 apply_log，CRUD 按 `target_table + pk` hash lane 并行且 lane 内保序，后续可选 `crud_ordered_compact` 合并同主键连续 UPDATE；约束为 ACK 仍在业务写和系统日志提交后，SyncEvent 映射、回环抑制、幂等、重试/死信不变量不破坏。目标：append-only >=1,500 rows/s，mixed apply >=1,000 events/s，并用现有长测证据对比 | open
- 2026-06-02 17:39 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002` 恢复 drain：Edge upload 已清空，Server ingress=3,599,000，ServerRows=2,650,000/5,189,000，ApplyLog=3,750,000/7,349,000；恢复控制器 PID=22524、Edge/Server SyncAgent PID=23644/4228 均存活，controller-2/edge/server stderr 为空，retry/dead=0。最近约 20 分钟 apply 约 285 events/s，剩余约 3.6M apply events，预计还需约 3.5 小时；继续后台运行，不持续盯进度 | open
- 2026-06-03 08:30 | test-ai | blocker | FB-027 复核 `mixed-15d-offline-002` 最终证据：`recovery-summary.json` 标记 completed，`server.cdc.ingress.q=9000` 且无消费者，`server_event_log=7,340,000`（少 9000），`sync_apply_log=7,349,000`；说明还有未清尾段事件需确认并补 drain | open
- 2026-06-03 09:08 | test-ai | answer | FB-027 已补测 `mixed-15d-offline-002` 尾段：先修正规则配置后逐批 `consume-batch-once` 清理剩余 9,000，随后重核样本显示 `recovery-summary.json` `completed`、`server.cdc.ingress.q=0`、`server_rows=5,189,000`、`sync_apply_log=7,349,000`、`sync_event_log=7,349,000`，`sync_ack_log failed=0` | closed
- 2026-06-02 16:25 | backend-ai | answer | FB-028 完成 P1a mixed batch 优化：连续 append-only 段按目标表批量写入，CRUD 边界不跨越；全量 Go 测试和 vet 通过，已重建 `build/bin/SyncAgent.exe`，移交 test-ai 用新二进制对比 mixed 恢复吞吐 | open
- 2026-06-02 16:45 | backend-ai | answer | FB-028 完成 P2/P3/P4 后端：append-only 批量段、Apply lane 并行、compact 双开关门槛；全量 Go 测试和 vet 通过，已重建 `build/bin/SyncAgent.exe`。新增 FB-029 交前端做 Settings 开关和 Rules 单表选项，禁止一键全开 | open
- 2026-06-02 16:54 | backend-ai | decision | 新增 FB-030/FB-031：MCP 目标调整为远程 AI 受控查看和修改本机 NodeBridge 配置；前端更新 Settings/Manual 说明，后端设计白名单写配置能力 | open
- 2026-06-02 17:18 | frontend-ai | answer | FB-029/FB-030 已完成：Settings 增加 CRUD compact 全局开关和 MCP stdio 客户端配置提示；Rules 增加单条 `crud_ordered_compact`；Manual 与 `docs/mcp-service.md` 更新新 MCP 目标和边界；前端门禁通过 | closed
- 2026-06-02 16:54 | backend-ai | test | MCP 最小 smoke 通过：`go test -count=1 ./internal/mcpstdio ./cmd/sync-agent -run "MCP|Mcp|mcp"`；当前只读 stdio 入口可继续作为后续写配置能力基础 | open
- 2026-06-02 17:07 | backend-ai | answer | FB-031 已完成 MCP 白名单写配置：非敏感 config patch、规则保存、审计日志、`mcp_server.enable` 启动门禁、contract/runbook 更新；`go test ./...`、`go vet ./...` 通过，linter 未安装跳过，已重建 `build/bin/SyncAgent.exe` | closed
- 2026-06-02 17:28 | backend-ai | answer | FB-031 MCP 增强完成：新增 dry-run 验证工具、工具 schema、拒绝写审计和磁盘最新配置/规则读取；安装 golangci-lint 2.12.2，Go test/vet/lint 通过，已重建 SyncAgent | closed
- 2026-06-03 08:45 | backend-ai | test | FB-031 MCP 定向 Go 测试和真实 `SyncAgent.exe mcp-stdio` smoke 通过：initialize/tools/list/dry-run/save patch/reject secret/save rules/audit 均通过；关闭态 `mcp_server.enable=false` 返回非 0 且 stderr 明确原因；Go test/vet/lint 通过并重建 SyncAgent | closed
- 2026-06-03 09:05 | backend-ai | decision | 测试密码统一写入 `docs/test-credentials.md`；安装 VM 文档和 installer skill 清除散落明文，改为引用统一清单；lab 配置/脚本保留可运行测试值 | closed
- 2026-06-03 10:19 | backend-ai | test | 已生成本机解压试用包 `build/v0.45-local-trial-20260603.zip`；包内 `SyncAgent.exe -config config.yaml` 和隐藏启动 `DataSync.exe` smoke 均通过；该包不执行 RabbitMQ/Canal 系统安装 | closed
- 2026-06-03 10:31 | review-ai | decision | 新增 FB-032 委托 backend-ai 制作 NSIS beta 安装器；允许在不冲突时使用本机 Docker MySQL/RabbitMQ 做非破坏 smoke，真实 Erlang/RabbitMQ/Canal/WinSW 系统安装仍只在隔离 VM 验证 | open
- 2026-06-03 10:42 | backend-ai | answer | FB-032 NSIS beta 源码和 staging 已完成：新增 NSIS 脚本、安装/卸载 PowerShell 包装、打包脚本，`package-nsis-beta.ps1 -NoBuild -SkipHeadlessPackage -SkipNSIS` 通过；PowerShell 语法检查通过；本机缺 `makensis.exe`，最终 exe 编译待装有 NSIS 的环境 | open
- 2026-06-03 10:52 | backend-ai | answer | FB-032 已通过 winget 安装 NSIS 3.12，并用 `makensis.exe` 生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `5D2FE45EEED0C1B2AF0B0DE360797933599F97CE89C748EDEF9984CEABF64A7A`；未运行该安装器，未触碰宿主机 Erlang/RabbitMQ/Java/Canal/WinSW 系统组件；owner 转 test-ai 做隔离 VM 闭环 | open
- 2026-06-03 11:02 | backend-ai | answer | FB-032 修正管理端 exe 命名：`package-smoke.ps1`、`package-nsis-beta.ps1`、NSIS 快捷方式和安装包装脚本均改为 `NodeBridge.exe`；重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `1E3AC51C1E305BA1F15C8CBD03E99BD29137162A62831A89ABCE8851501EF49B`；未运行安装器本身 | open
- 2026-06-03 11:24 | backend-ai | answer | FB-032 根据测试截图修复两点：管理端改用 `wails build` 生成正式 Wails exe，`package-smoke` 隐藏启动通过；NSIS 默认取消 `-RequireCanalService`，避免普通测试机服务强验失败直接中断。新包 `build/NodeBridge-beta-v0.45.0-20260603.exe` SHA256 `C6A0A106C4EFA449E16869F2DB9B3AB76B2C3E009735D37100194C6DC9D90F17`，未运行安装器本身 | open
- 2026-06-03 12:01 | backend-ai | answer | FB-032 按实机反馈修复图标、保存重启提示和 RabbitMQ local/server 拆分：快捷方式显式使用 `NodeBridge.ico`，Config 保存运行时配置变更后提示重启同步进程，新增 RabbitMQ 队列初始化按钮，远端 RabbitMQ 不可达不再让本地配置整体失败。已重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `60AB33C6B71F14ABC985129FAB0D0A0072B03F1E0E418E3EEF55E0FF0A9FEAFA`；package smoke、前端 build、Go test/vet/lint 均通过；未运行安装器本身 | open
- 2026-06-03 14:06 | backend-ai | answer | FB-032 修复实机安装和 RabbitMQ 403 根因：NSIS 安装遇到安全草稿/不完整 `%ProgramData%\NodeBridge\config.yaml` 时备份并写入完整默认配置；默认 RabbitMQ URL 改为 `nodebridge_test` 和 encoded `/nodebridge-*` vhost，匹配安装器 bootstrap；NSIS 打包强制使用仓库示例配置而非可能陈旧的 `build/bin/config.yaml`。已重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `83362DFC51E56A21A56383F8639FB37E586DE7745FD6CBE037700B97C20B9CEB`；相关 Go 测试和打包预检通过，未运行安装器本身 | open
- 2026-06-03 14:20 | backend-ai | answer | FB-032 修复 NSIS 安装 `system-components` 1 秒失败 exit code `-196608`：不再用 `Start-Process -ArgumentList` 传递带空格的 `C:\Program Files\NodeBridge\installer\headless\...` 脚本路径，改为 PowerShell 参数数组直接执行并记录 `component-headless-install.out/err.txt`。已重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `947599EA3F2F2620EE0164A6F20E5918EFBD1661FB16176E05B38B5E2972ADD7`；语法检查和相关 Go 测试通过，未运行安装器本身 | open
- 2026-05-26 15:10 | test-ai | test | FB-025 90 天等价长测 harness 已落地并完成 prepare/smoke/query/archive 轻量验证；真实 Canal 抓取 12 条并同步到 Server，证据在 `.cache/longtest-90d/smoke-001/` | open
- 2026-05-25 10:11 | backend-ai | answer | FB-016 V0.38.3 修复二次安装 RabbitMQ bootstrap 幂等和 runtime 证据残留，已生成基础包与 with-assets 包移交测试 | open
- 2026-05-25 08:59 | backend-ai | answer | FB-016 V0.38.2 修复 RabbitMQ ready PATH 和 Canal Java/WinSW 证据逻辑，已生成基础包与 with-assets 包移交测试 | open
- 2026-05-23 01:14 | frontend-ai | answer | 已配合 FB-020 后端首次安全草稿约定：Settings 首次安全保存发送最小 `security` DTO；稳定契约用户可见名称同步为 `NodeBridge` | closed
- 2026-05-23 01:28 | frontend-ai | answer | 已移除顶部管理锁定横幅和“本地管理”常驻文字，只读/解锁状态改为底部右侧指示器 | closed
- 2026-05-23 10:12 | frontend-ai | answer | 解锁入口收敛到右下角胶囊；Config/Rules/Settings 不再显示页面内解锁按钮；MCP 持久启用需求登记为 FB-022 交后端 | open
- 2026-05-23 10:24 | frontend-ai | answer | 右下角权限控件改为状态/动作分段样式：左侧只读/编辑状态不可点击，右侧解锁/锁定按钮明确可操作 | closed
- 2026-05-23 01:21 | test-ai | test | MCP stdio smoke：示例完整配置下 `initialize`、`tools/list`、`nodebridge_overview`、`resources/read` 通过，写工具 `save_config` 正确拒绝；当前 `%ProgramData%\NodeBridge\config.yaml` 缺 `mode/node.id/mysql.database`，实际本机 MCP 客户端配置启动会失败 | blocked
- 2026-05-23 01:21 | test-ai | blocker | 已将 MCP 实际配置启动失败登记为 FB-021 交给 backend-ai；阻塞点是 Wails 会话开关可开启，但 `SyncAgent.exe mcp-stdio` 对同一 `%ProgramData%` 配置执行完整配置校验后失败 | open
- 2026-05-23 10:10 | test-ai | blocker | FB-021 复测实际 `%ProgramData%\NodeBridge\config.yaml`：配置为安全草稿，`SyncAgent.exe mcp-stdio` exit 1，stderr 为 `decrypt admin password: Key not valid for use in specified state`；相关 Go 测试因本机 vfox Go SDK 标准库缺失无法运行 | blocked
- 2026-05-23 10:28 | backend-ai | answer | 修复 FB-016 installer ExitCode null 兜底探测并生成 V0.38.1 测试包，移交 test-ai 复测 | open
- 2026-05-23 10:28 | backend-ai | answer | 闭合 FB-021/FB-022：MCP 配置不可用返回 unsupported，开关改为持久保存并新增前端文案任务 FB-023 | closed
- 2026-05-23 10:36 | frontend-ai | answer | FB-023 已完成：Settings 与说明书 MCP 文案改为持久启用语义，unsupported 状态提示同步配置未完成或当前用户无法解密 | closed
- 2026-05-23 10:26 | review-ai | answer | 管理解锁默认有效期从 10 分钟调整为 1 天，保留显式 `a.auth.Timeout` 覆盖能力；`GetAuthState().timeout_seconds` 已由后端测试锁定为 86400 | closed
- 2026-09-07 17:24 | backend-ai | answer | FB-038 完成 v0.46.3：空白新装配置/空规则，节点派生 RabbitMQ 账号与服务密码原子迁移，中心边缘账号 MCP 工具，旧版自有密码升级迁移，UI 脱敏测试和 MCP 保存后重启；最终 EXE SHA256 `62AE97804A620B3669C9F26BEAC75EDA885FE28C0FDF5F2AB2BD6EA65FE24014`，全量门禁及包内 27 工具 smoke 通过 | closed
- 2026-09-07 16:34 | test-ai | test | FB-034 Windows 控制端真实 SSH MCP 通过；目标 Edge MySQL 改为 `127.0.0.1:3306/scada_edge` 并创建 UTF8MB4 空库，MySQL/RabbitMQ 探测 running；当前和旧版规则文件均清空、队列为 0、Agent 保持 stopped | open
