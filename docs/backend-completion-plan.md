# Backend Completion Plan

Last updated: 2026-05-22

## 已有后端能力

| 能力 | 状态 | 验收方式 |
| --- | --- | --- |
| 配置加载、校验、落盘 | 已完成 | `internal/appconfig` 单元测试，Wails App 测试 |
| 密码、token、退出密码加密与脱敏 | 已完成 | `secure_test.go`，Wails App 脱敏测试 |
| SyncEvent、ChangeEvent、Normalizer | 已完成 | `internal/event`、`internal/normalizer` 测试 |
| 表名和列名重映射 | 已完成 | `internal/mapper`、CRUD/batch E2E |
| MySQL Apply Worker | 已完成基础 CRUD | `internal/apply` 测试，`lab-crud-e2e.ps1` |
| RabbitMQ 拓扑、发布确认、手动 ACK | 已完成 | `internal/rabbitmq` 测试，lab E2E |
| Edge 本地队列缓存和断网恢复 | 已完成基础验证 | `lab-disconnect-e2e.ps1` |
| Server 动态节点注册和分发 | 已完成 | `internal/nodeapi` 测试，`lab-config-e2e.ps1` |
| 配置下发 `CONFIG_UPDATE` | 已完成 | `lab-config-e2e.ps1` |
| 批量同步 50 条或 500ms | 已完成基础实现 | `lab-batch-e2e.ps1`，`lab-stress-e2e.ps1` |
| Wails 后端配置、规则、队列、失败、日志、诊断接口 | 已完成基础实现 | `internal/datasyncui` 测试 |
| 托盘常驻后端支撑 | 已完成后端接口 | `VerifyExitPassword`、`GetAutoStart`、`SetAutoStart` 测试 |
| 管理解锁和敏感操作保护 | 已完成后端接口 | `admin_password` 解锁、`exit_password` 退出、`UnlockAdmin`、`LockAdmin`、`GetAuthState` 和敏感方法锁定测试 |
| Wails 后端运行控制 | 已完成外部进程控制 | `StartAgent`、`StopAgent`、`RestartAgent` fake controller 测试，固定目录 package smoke |
| 真实 Canal E2E | 已完成基础验证 | `lab-canal-e2e.ps1`，`lab-canal-soak.ps1 -Count 20` |
| 失败重试最小闭环 | 已完成基础验证 | `lab-retry-e2e.ps1`，批量标记 `PENDING`、pending replay、downlink apply、死信预览 |
| 受管组件执行器 alpha | 已完成基础实现 | `managed-plan/apply/repair/uninstall`，Wails plan/apply，manifest 和 Canal config 写入 |
| 离线安装包预检 | 已完成安全预检 | `installer-assets-check`、`installer-command-plan`，只校验文件/哈希并输出命令计划 |
| 只读 MCP alpha | 已完成基础实现 | `mcp-stdio` tools/list smoke，overview/rules/logs 等只读工具 |
| 11 节点长测入口 | 已完成脚本入口 | `lab-11-soak-e2e.ps1`、`lab-11-disconnect-e2e.ps1` 最小参数验证 |

## 已补齐测试范围

- 配置示例和 lab 配置加载。
- 配置必填项、非法 mode。
- 密钥加密、解密、redacted secret 合并和落盘无明文。
- 规则加载、重复规则、非法 identifier、目标主键数量不一致、重复目标列映射。
- RabbitMQ publisher confirm、consumer ACK/NACK、拓扑、队列状态。
- Apply Worker INSERT/UPDATE/DELETE 软删、幂等、表列映射。
- Runtime Edge upload、Server ingress、Edge downlink、batch 顺序和失败边界。
- Wails App 空状态、配置保存、规则保存、退出密码、自启动、队列错误状态、日志和诊断包。
- Wails App 管理解锁、手动锁定、敏感方法 locked 拒绝和解锁后放行。
- 诊断包 zip 内容和 JSON 编码失败。
- 三 RabbitMQ + 三 MySQL lab：smoke、CRUD、batch、config、disconnect、1000 条 stress。

## 未完成后端能力

| 优先级 | 能力 | 说明 | 建议版本 |
| --- | --- | --- | --- |
| P1 | 冲突策略 | `SERVER_WIN`、`LAST_WRITE_WIN` 尚未形成完整仲裁和冲突日志闭环。 | V0.25 |
| P1 | 受管组件安装 beta | V0.33 已完成离线包路径/哈希预检和命令计划；真实 Erlang/RabbitMQ/Canal 安装与服务注册需 VM 验证后进入后续版本。 | V0.34+ |
| P1 | 诊断增强 | 诊断包已生成，但还需包含系统环境、RabbitMQ broker 信息、最近错误分类和压测摘要。 | V0.24 |
| P2 | 多节点扩容长测 | 已具备 11 节点 soak 和断网恢复脚本；仍需客户试用前长参数/过夜压测。 | V0.32 |
| P2 | 配置迁移和密码恢复 | DPAPI 已用，仍需配置版本迁移、备份恢复、忘记退出密码处理策略。 | V0.26 |

## 下一版建议

目标：推进安装器 Beta 2，在隔离 Windows VM 中开始真实离线包安装能力。

必须完成：

- 使用 V0.33 catalog 校验通过的离线包。
- 执行 Erlang/OTP 静默安装。
- 执行 RabbitMQ Server 静默安装。
- 检测 RabbitMQ Windows Service 是否存在并可启动。
- 写入安装 manifest，并保证卸载只清理 NodeBridge 登记资源。
- 不注册 Canal Service，留到后续版本。

测试门禁：

- `go test ./...`
- `go vet ./...`
- `npm run build`
- `scripts/lab-smoke.ps1`
- `scripts/lab-e2e.ps1 -SkipPrepare`
- `scripts/lab-crud-e2e.ps1 -SkipPrepare`
- `scripts/lab-batch-e2e.ps1 -SkipPrepare`
- `scripts/lab-config-e2e.ps1 -SkipPrepare`
- `scripts/lab-disconnect-e2e.ps1 -SkipPrepare`
- `scripts/lab-stress-e2e.ps1 -SkipPrepare`

## 交付判断

当前后端具备技术试点能力，但不是正式交付版本。

客户试用前至少还要完成：

- 外部 `SyncAgent.exe` 已可构建，`DataSync.exe` + `SyncAgent.exe` 固定目录 smoke 已通过。
- 仍需最终安装器把 DataSync.exe 与 SyncAgent.exe 固定放在同一目录并创建快捷方式/卸载入口。
- 前端托盘常驻交互接入。
- 安装 runbook 和 RabbitMQ 默认安装策略。

正式交付前还要完成：

- RabbitMQ/Erlang/Canal 离线无感安装器，严格按 `install-manifest.json` 只管理 NodeBridge 资源。
- 长时间稳定性测试。
- 多 Edge 扩容测试。
- 冲突策略和失败重试产品闭环。
