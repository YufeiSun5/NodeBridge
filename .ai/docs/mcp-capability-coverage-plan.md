# MCP 新旧功能覆盖清单

> 2026-09-11 用户纠偏：MCP保持全开放，scope申请/审批不实施，相关新增代码已撤回。下文权限列不再是批准的实施要求；测试隔离仅指本方独占测试数据，不涉及业务AI数据库与配置。

> 2026-09-11 | review-ai | 配合 [实施方案](initial-alignment-implementation-plan.md) 的拟议清单，不代表工具已新增。
> EN: Existing-tool compatibility and proposed capability coverage. JA: 既存ツール互換性と新機能の対応計画。
> 首次对齐可选、默认关闭、手动预览确认。MCP 不得在安装、注册、保存、启停或发现差异时自动发起对齐。

## 1. 覆盖定义与发布规则

1. 业务能力可经 MCP 发现、获授权调用、读取实际结果及审计；长任务可跨客户端断开后查询和控制。
2. 保留已有工具名称、输入兼容性、秘密保护、数据治理上限和 5 个资源 URI。新增授权入口不把旧非秘密 patch 工具升级成任意配置/SQL 写入工具。
3. 每一行覆盖项必须有成功、无权、参数错误、真实失败用例；写入/长任务另有重复请求、超时恢复和审计用例。返回 unsupported 或仅列出名称不算该环境功能验收通过。
4. 普通产品能力在正式模式以 scope grant 开放；测试/注入/压测原语继续限明确实验室配置，不把 -lab-full-access 当生产解锁方案。
5. 安装前引导、UAC、桌面会话、业务停写和业务 schema 是真实前置条件；允许 MCP 发起受控申请/计划/查询，不能绕过 OS 或虚报执行成功。
6. 构建阶段拟从同一工具注册表导出 catalog/schema/capability-to-handler 映射，CI 比对 Wails 产品入口和 CLI 子命令清单；新增功能无 MCP 映射或无书面内部用途分类则阻止发布。

## 2. 现有 33 工具逐项保留

来源：`internal/mcpstdio/server.go` 的 10 个 BaseTools，`internal/datasyncui/mcp.go` 普通额外 5 个和实验室额外 18 个。普通共 15，实验室共 33，不是 48。

| # | 现有工具名 | 当前可见性 | 目标授权/补充验证 |
| --- | --- | --- | --- |
| 01 | nodebridge_overview | 普通 | status.read；真实组件状态/版本；不能用概览零替代 broker 屏障 |
| 02 | nodebridge_config_summary | 普通 | config.read；密文/密码始终不回显 |
| 03 | nodebridge_queue_status | 普通 | status.read；本地/中心/下行/重试/死信口径核验 |
| 04 | nodebridge_sync_rules | 普通 | rules.read；旧规则与新可选字段兼容 |
| 05 | nodebridge_failed_events | 普通 | status.read；分页/字段脱敏/失败原因 |
| 06 | nodebridge_logs | 普通 | logs.read；轮转日志、限量、结构化值无损 |
| 07 | nodebridge_diagnostic_summary | 普通 | diagnostics.read；实际能力与限制，不只列路径 |
| 08 | nodebridge_validate_config_patch | 普通 | config.plan；旧白名单/非秘密限制不变 |
| 09 | nodebridge_save_config_patch | 普通 | config.write；原白名单不变；不得触发首次对齐 |
| 10 | nodebridge_save_sync_rules | 普通 | rules.write；版本校验、行范围；不得触发首次对齐 |
| 11 | nodebridge_node_options | 普通 | nodes.read；保持现有候选语义，完整节点另有 list |
| 12 | nodebridge_test_mysql | 普通 | connection.test；使用已保存凭据，不输出秘密 |
| 13 | nodebridge_test_rabbitmq | 普通 | connection.test；分别报告本地与中心连接 |
| 14 | nodebridge_agent_status | 普通 | status.read；FB-089 时间误脱敏回归 |
| 15 | nodebridge_mysql_schema | 普通 | schema.read；只读表列/键元数据 |
| 16 | nodebridge_mysql_diagnostics | 实验室 | diagnostics.read；正式授权后开放，5 秒默认/10 秒上限等既有边界不变 |
| 17 | nodebridge_mysql_query | 实验室 | data.read + 表/列范围；50 默认/200 最大，不作批量对齐数据通道 |
| 18 | nodebridge_mysql_mutation_plan | 实验室 | data.plan；结构化有限操作、预期影响行数 |
| 19 | nodebridge_mysql_mutation_apply | 实验室 | data.write；原 100 行边界、确认/expected_rows及审计，不因对齐需要扩容 |
| 20 | nodebridge_mysql_schema_change_plan | 实验室 | schema.plan；现有业务表 ADD/DROP COLUMN，不隐式建表 |
| 21 | nodebridge_mysql_schema_change_apply | 实验室 | schema.write；计划有效性/DDL 权限/依赖核验 |
| 22 | nodebridge_start_agent | 实验室 | agent.control；正式 scope 开放，不隐式扫描存量/首次对齐 |
| 23 | nodebridge_stop_agent | 实验室 | agent.control；优雅停止和部分作业状态可见 |
| 24 | nodebridge_restart_agent | 实验室 | agent.control；配置生效，已有对齐默认暂停，不新建作业 |
| 25 | nodebridge_retry_failed_event | 实验室 | retry.write；单事件权限、重复请求和确认传播 |
| 26 | nodebridge_retry_failed_events | 实验室 | retry.write；限量/范围/批次实际结果 |
| 27 | nodebridge_dead_letters | 实验室 | queue.preview；目前取出再重入队可能影响顺序，需副作用授权/正确 annotation；禁止删除 |
| 28 | nodebridge_managed_install_plan | 实验室 | installer.plan；所有权、默认复用与外部组件边界 |
| 29 | nodebridge_apply_managed_install | 实验室 | installer.run；已有小操作返回兼容；较长执行通过新增 job 入口，不改变旧结果形状 |
| 30 | nodebridge_ensure_server_edge_user | 实验室 | broker.accounts.write；仅 server + managed RabbitMQ，不能宣称覆盖 external Docker |
| 31 | nodebridge_export_diagnostics | 实验室 | diagnostics.export；保留远端路径结果，新增受权 artifact 读取，不任意文件下载 |
| 32 | nodebridge_get_autostart | 实验室 | desktop.read；绑定真正 OS 用户 |
| 33 | nodebridge_set_autostart | 实验室 | desktop.control；当前用户作用域、无交互用户时明确失败 |

P1 完成授权兼容主框架；P3 补齐各域适配。普通/实验室两份 0.46.15 原工具集作为回归基线，新版工具数量允许增加，但原条目不得无说明消失。工具清单可按主体权限过滤，权限变更后的发现行为与直接调用拒绝均测试。

保留资源：`nodebridge://overview`、`nodebridge://config`、`nodebridge://sync-rules`、`nodebridge://diagnostic-summary`、`nodebridge://logs`。新作业/能力资源若支持则单独声明；不要注册未实现的订阅能力。

## 3. 旧产品功能的 MCP 缺口

以下名称均为拟议；P0 冻结、P1-P3 实现，P5 逐行验收。多个工具共同完成一项能力，不要求把所有内部辅助函数直接暴露。

| 能力/现有入口 | 拟议 MCP 工具 | 权限与完成条件 |
| --- | --- | --- |
| 能力/模式/版本/安装归属 | nodebridge_capabilities | 返回版本、功能协议、授权可用项、managed/external、QUIESCED/ONLINE 实际支持；不含秘密 |
| GetAuthState / UnlockAdmin / LockAdmin | nodebridge_auth_status / nodebridge_auth_request / nodebridge_auth_revoke | 通过本机批准或预授权 OS 策略获得受限 grant；不暴露通用验密探测、不得自授权 |
| GetMCPServerStatus / SetMCPServerEnabled | nodebridge_mcp_status / nodebridge_mcp_set_enabled | 首次启用需要本地引导；关闭后拒绝后续调用，明确作业 grant 是否被同时撤销 |
| SaveConfig 未被非秘密 patch 覆盖的可写项 | nodebridge_config_plan / nodebridge_config_apply | 完整类型化配置计划、版本/差异/审批；凭据引用另走受控入口，不接收原始 YAML |
| MySQL/RabbitMQ/security/log-web 凭据设置或轮换 | nodebridge_credentials_plan / nodebridge_credentials_apply | 输入本机保护的 secret_ref 或安全交互申请；不将明文秘密放在 AI 示例或结果；不可返回已有密码 |
| TestMySQL/TestRabbitMQ 的候选配置测试 | nodebridge_connection_test | 支持 type=mysql/rabbitmq/cdc 与已批准候选配置/secret_ref；不得任意网络探测 |
| canal-check，CDC 状态/位点 | nodebridge_cdc_status / nodebridge_cdc_check | 真实连接、配置与位点信息；只读检查不消耗批次/推进位点 |
| register-node / list-nodes | nodebridge_nodes_list / nodebridge_node_register_plan / nodebridge_node_register_apply | 与 node_options 区分完整状态；node_id 唯一、无隐式首次对齐 |
| set-node-config / list-node-config | nodebridge_node_config_get / nodebridge_node_config_plan / nodebridge_node_config_apply | 节点范围、版本、命令回执；中心提出不等于边缘已保存生效 |
| init-rabbitmq，external 实例拓扑/账户 | nodebridge_topology_plan / nodebridge_topology_apply | 复用 installer planner；外部实例需显式受权管理适配，默认不改；managed-only 旧工具语义保留 |
| migrate，系统 schema 版本 | nodebridge_migration_status / nodebridge_migration_plan / nodebridge_migration_apply | 只执行发布包固定编号迁移；备份/当前 schema 校验；不接受任意 SQL |
| managed-repair/uninstall/config-migrate，安装/升级 | nodebridge_installation_plan / nodebridge_installation_apply | action 白名单 install/upgrade/repair/uninstall/config_migrate；外部 helper 返回 job_id；manifest/UAC/保配置 |
| installer-assets-check / installer-command-plan | nodebridge_installer_assets_check / nodebridge_installer_command_plan | 校验本机批准的包/清单/hash；只能展示固定操作计划，不能变成任意命令执行器 |
| mcp-client-config | nodebridge_mcp_client_config | 导出普通模式本地/SSH 客户端配置，无实验室 token/密码，不自动写外部客户端设置 |
| 诊断 ZIP 的实际取得 | nodebridge_artifact_get | 只读当前主体可访问的已生成 artifact_id，分块/大小上限/hash；不允许任意路径 |
| UI 语言/主题本地偏好 | nodebridge_ui_preferences_get / nodebridge_ui_preferences_set | 通过当前用户 UI/本机偏好适配层，保持中英日和默认中文；不混入同步配置 |
| 托盘显隐 / VerifyExitPassword / RequestExit | nodebridge_desktop_status / nodebridge_desktop_request | action 白名单 show/hide/request_exit；真实交互用户 IPC；退出单独授权，Session 0 不假装桌面操作成功 |
| serve-log-web / serve-node-api 的可选运行 | nodebridge_aux_service_status / nodebridge_aux_service_plan / nodebridge_aux_service_apply | 默认关闭；绑定/端口/token/防火墙影响显式批准；仅管理现有指定服务，不泛化进程启动 |
| sync_mode、冲突、DDL、分发/来源范围 | 复用 nodebridge_sync_rules / nodebridge_save_sync_rules | 所有已支持枚举和映射可读取/配置/验证；新增 row_scope 和手动策略按规则版本拒绝旧端误用 |
| 原始同步/诊断步骤 CLI | nodebridge_pipeline_step_plan / nodebridge_pipeline_step_apply | 见下一节；只在明确 lab/sandbox 范围，类型化输入、独占目标、数量/时间上限、审计 |

配置变更涉及重启或后台服务时，计划列出影响并需单独确认，不能让“保存配置”连带触发数据库写入、首次对齐或安装组件。

### 3.1 CLI 全入口归属

来源为 `cmd/sync-agent/main.go` 的现有命令分派。每个旧入口要么复用正式产品工具，要么通过明确受限的诊断原语覆盖；不将面向开发者的命令当正常业务自动化捷径。

| CLI 命令（完整分组） | MCP 归属 |
| --- | --- |
| run | start_agent / stop_agent / restart_agent / agent_status |
| migrate | migration_status/plan/apply |
| init-rabbitmq | topology_plan/apply |
| canal-check | cdc_check |
| failed-events / retry-event / retry-failed-batch / dead-letters | failed_events / retry_failed_event / retry_failed_events / dead_letters |
| managed-plan / managed-apply | 保留 managed_install_plan / apply_managed_install；新长作业走 installation_plan/apply |
| managed-repair / managed-uninstall / managed-config-migrate | installation_plan/apply 对应 action |
| installer-assets-check / installer-command-plan | installer_assets_check / installer_command_plan |
| mcp-client-config | mcp_client_config |
| mcp-stdio | MCP 传输宿主，不递归创建宿主工具；mcp_status/client_config 覆盖接入管理 |
| serve-log-web / serve-node-api | aux_service_status/plan/apply |
| register-node / set-node-config / list-nodes / list-node-config | node_register_plan/apply、node_config_get/plan/apply、nodes_list |
| apply-event / publish-event / publish-change-once / dispatch-event-once | pipeline_step_plan/apply 对应受限 action；不得接受任意 SQL/目标库 |
| publish-stress-batch | pipeline_step_plan/apply 的 bounded_stress，只有独占 lab 表/队列，独立容量/时限/停止清理计划 |
| canal-publish-once / server-cdc-dispatch-once / server-canal-dispatch-once | pipeline_step_plan/apply；涉及真实消费/位点，不能标 readOnly；不得与常驻 CDC owner 并行 |
| consume-once / consume-batch-once / consume-downlink-once / consume-downlink-batch-once | pipeline_step_plan/apply；保持事务/ACK/幂等，运行前独占租约 |
| forward-upload-once / forward-upload-batch-once / replay-pending-once | pipeline_step_plan/apply；confirm/重放/去重语义与原核心共用 |

LoadConfig 等路径级 helper、Wails Startup/Shutdown 生命周期、内部 private 函数不是独立产品功能，不暴露为任意文件/进程工具；它们所支撑的配置、启停、桌面能力已在上表覆盖。

## 4. 新增功能必须同时支持 MCP

| 能力 | 拟议 MCP 工具 | 阶段/限制 |
| --- | --- | --- |
| 首次对齐开关/手动策略 | config_plan/apply 或 save_sync_rules 的版本化可选字段 | P2；默认 DISABLED，仅 MANUAL 可选；保存不执行，无 AUTO |
| 节点与规则对齐预检 | nodebridge_alignment_preflight | P2；只读连接/schema/key/权限/预算/能力；不得自动进入 start |
| 只读计划扫描 | nodebridge_alignment_plan | P2；返回 planning job_id，不写目标业务行；模式 QUIESCED，P4 才加 ONLINE |
| 差异与冲突样本 | nodebridge_alignment_diff | P2；plan_id + cursor，表/列授权、限量、精确/估算分开 |
| 明确批准并开始 | nodebridge_alignment_start | P2；plan_hash + request_id + 人类批准引用，范围/数量上限不变；勾选不等于批准 |
| 作业列表/详情/日志 | nodebridge_jobs_list / nodebridge_job_get / nodebridge_job_events | P1-P2；可见各端水位/行数/字节/冲突/等待/部分提交；游标分页 |
| 暂停/恢复/取消 | nodebridge_job_pause / nodebridge_job_resume / nodebridge_job_cancel | P2；事务边界生效；重启后默认 PAUSED；取消不回滚已提交数据，不直接开门禁 |
| 数据核验 | nodebridge_alignment_verify | P2；完整核验为后台作业，结果含范围/时点/差异；不能用 count 相等代替 |
| 冲突解决 | nodebridge_alignment_conflict_plan / nodebridge_alignment_conflict_apply | P2；单独计划/审批，不能悄悄改权威；有限批次，对已变化行拒绝 |
| 失效计划重建 | nodebridge_alignment_replan | P2；新计划/代际，不沿用旧批准，需再次明确 start |
| 在线快照/增量暂存/交接 | 同一 alignment 工具 mode=ONLINE，job_get 返回 B0/C/generation/缓冲状态 | P4；能力不具备时拒绝，不静默降级为停写/改变范围 |
| 容量预算/估算/实时分项 | nodebridge_capacity_status / nodebridge_capacity_plan | P2-P4；100GB同一总预算，未实测值明确 estimated；不自动清理数据 |
| 接入流程汇总 | nodebridge_onboarding_plan / nodebridge_onboarding_status | P3；默认 skip_initial_alignment=true；各写操作仍明确审批，不提供全自动安装+对齐+启用按钮 |

首批不提供“复制整库”“自动删除目标多余行”“自动多主冲突合并”“任意 SQL”“远程任意 Shell”等工具。它们不是已支持能力；如以后成为正式功能，同样须补 MCP、权限、回滚边界与测试后才能发布。

## 5. 端到端 MCP 验收脚本路线

1. 从正式本地 stdio/SSH stdio 连接，记录 initialize、tools/list、resources/list、capabilities 与当前主体权限，不使用 lab 绕过替代正式验收。
2. 默认未选首次对齐：通过 MCP 登记节点/保存规则/Start Agent，验证没有对齐 job、没有存量扫描与对齐写入，原 CDC 仍按既有语义运行。
3. 用户明确选择首次对齐：预检 -> plan -> jobs/get -> diff -> 人类批准 -> start -> events/get -> verify，结果与独立业务账本一致。
4. MCP 连接断开后重连查询同 job；Agent 重启后 job 为 PAUSED，未经 resume 不追加业务写入；显式恢复后完整核验，不能创建重复作业。
5. 通过同一工具完成权限拒绝、plan_stale、源/schema 变化、磁盘不足、远端离线、部分提交取消；确保真实原因与下一动作可见，未完成不显示成功。
6. M2 在用户选择 ONLINE 后增加快照期间更新/删除、切换屏障和晚到旧块；完成初始核验后验证持续增量至少一次真实 INSERT/UPDATE/DELETE。
7. 对旧 33 工具、全部旧产品入口、安装与桌面外部前置条件逐项复测。产品源码测试、真实 MCP 验收和最终包 hash 绑定；直接执行底层 SQL 只能作为独立 oracle，不能替代 MCP 功能测试。

本清单完成条件是逐行功能验收，不是工具数达到某个目标。后端负责注册/实现和权限，前端负责同能力 Wails 视图，test-ai 负责真实协议与 E2E，review-ai 审阅遗漏和风险；活跃状态以 [AI_BOARD.md](../../AI_BOARD.md) 为准。
