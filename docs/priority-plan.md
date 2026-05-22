# Priority Plan

## 目标

后续主线优先客户试用闭环，MCP 稳步推进为诊断辅助能力，不抢同步链路、安装器和稳定性资源。

## P0 客户试用前必须完成

| Version | 主题 | 交付结果 |
| --- | --- | --- |
| V0.28 | 真实 Canal E2E / soak | MySQL binlog -> Canal -> SyncAgent -> RabbitMQ -> Apply 可复现，记录 offset 和重启行为。 |
| V0.29 | 打包目录与运行闭环 | `DataSync.exe` + `SyncAgent.exe` 固定目录 smoke 已跑通；前端进程状态展示仍由 FB-012 跟进。 |
| V0.30 | 失败重试最小闭环 | 失败列表、单条重试、批量重试、死信只读预览和重试 E2E 已跑通。 |

## P1 客户试用增强

| Version | 主题 | 交付结果 |
| --- | --- | --- |
| V0.31 | 安装执行器一期 + 只读 MCP Alpha | 受管 RabbitMQ/Canal 执行器 alpha 已有 dry-run/apply/repair/uninstall；stdio MCP 只读诊断工具已可 smoke。 |
| V0.32 | 11 节点长测 | 11 节点 soak 和断网恢复脚本已具备；最小参数已验证，长参数留作试用前压测。 |
| V0.33 | 安装器 Beta 1 预检 | 已完成离线包 catalog、SHA256 校验和命令计划；不触碰本机服务。 |
| V0.34 | 安装器 Beta 2 VM 验证 | 在隔离 Windows VM 执行 Erlang/OTP 与 RabbitMQ 静默安装、服务检测和卸载验证。 |

## MCP 节奏

- V0.28: 冻结 MCP 边界，不允许绕过 Wails 后端 service。
- V0.31: 只读 stdio MCP，不占端口，不需要公开发布。
- V0.33: 继续保持只读 MCP；安装器预检优先，不扩展写操作。

## 当前原则

- Canal、打包、失败重试、安装器优先级高于 MCP。
- MCP 初期只做诊断：overview、queue、rules、failed events、logs、diagnostic package。
- 不开放 `save_config`，直到权限模型和审计日志完整。
