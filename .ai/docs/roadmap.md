---
description: "Use when: 查看从当前骨架到 V1.0 结束的版本路线、下一版目标、测试门禁、Wails 与日志 Web 服务约束"
---

# Roadmap

## 当前状态

- V0.1-V0.14 已完成：Go 骨架、RabbitMQ/MySQL/CDC 基础、runtime、持久化、replay、Canal adapter/runtime。
- V0.15-V0.18 已完成：单机三节点 lab、独立 RabbitMQ、节点注册、配置下发、CRUD/软删/幂等/单向表/表列映射 E2E。
- V0.19 已完成：批量同步、Wails React 骨架、暗色工业终端 UI 规范。
- V0.20 已完成：前后端契约、协作文档、Wails API 骨架、前端初版页面和三语切换。
- V0.21 已完成：Wails 后端真实接口、配置落盘、密钥保护、托盘后端支撑、诊断包和压力测试脚本。
- V0.33 已新增安装器离线包预检：catalog、SHA256 校验和命令计划。下一步是 V0.34：隔离 Windows VM 真实安装验证。

## 版本路线

| Version | 目标 | 结束条件 |
| --- | --- | --- |
| V0.1 | 工程骨架 | `go test ./...`、`go vet ./...` 通过 |
| V0.2 | MySQL Apply + 模拟事件 MVP | JSON `SyncEvent` 可通过映射后写入 MySQL |
| V0.3 | RabbitMQ 真实链路 | publish/consume/ack 与 Docker RabbitMQ 集成测试通过 |
| V0.4 | Edge/Server Runtime | runtime 单元测试覆盖 Edge 上传、Edge 下发 Apply、Server Apply、防回源下发、worker 循环、dispatch 计数和日志 Web |
| V0.5 | CDC stub + Canal 前置 | 模拟 `ChangeEvent` 可生成 `SyncEvent` 并发布到 Edge 本地上传队列 |
| V0.6 | Canal prep | Canal row change 可转换为 `ChangeEvent`，offset 可保存并 ack |
| V0.7 | CDC recovery | offset 可持久化到 MySQL，CDC 临时错误可按策略重试 |
| V0.8 | Persistence + retry entry | ACK、dispatch、error 可持久化，失败事件可查询和标记重试 |
| V0.9 | Event payload + replay | Server 持久化事件 payload，失败下发可单步重放 |
| V0.10 | Auto replay worker | `sync-agent run` Server 模式自动运行 replay worker |
| V0.11 | Single machine lab | 一台电脑可启动 RabbitMQ + 3 个 MySQL 节点并初始化配置 |
| V0.12 | E2E lab smoke | Docker MySQL/RabbitMQ 下 Edge A -> Server -> Edge B 跑通 |
| V0.13 | Real CDC adapter | 真实 Canal client 路径确认并可读取 MySQL binlog |
| V0.14 | Canal runtime | Edge runtime 可从 Canal 拉取变更并发布到本地 RabbitMQ |
| V0.15 | Single PC verified | 三 MySQL + RabbitMQ 链路跑通 |
| V0.16 | Separated RabbitMQ lab | 三套 RabbitMQ 下断网缓存验证 |
| V0.17 | Node management | HTTP 注册、动态分发、配置下发 |
| V0.18 | CRUD E2E | 增删改、软删、幂等、单向表不分发、表列映射 |
| V0.19 | Batch + UI skeleton | 50 条或 500ms batch，Wails React 骨架 |
| V0.20 | Frontend/backend contract | Wails API 契约、前后端协作、页面范围 |
| V0.21 | Wails backend + stress | 配置落盘、密钥保护、托盘后端支撑、诊断包、1000 条 stress |
| V0.22 | Runtime control + benchmark | Wails `Start/Stop/Restart` 真实编排，常驻 producer 压测 |
| V0.23 | Real Canal E2E | MySQL binlog -> Canal -> RabbitMQ -> Apply 全链路 |
| V0.24 | Retry/dead-letter closure | 失败重试、死信、诊断增强闭环 |
| V0.25 | Conflict + multi-edge | 冲突策略和多 Edge 扩容长测 |
| V0.26 | Installer | RabbitMQ/Erlang 离线无感安装和卸载策略 |
| V0.27 | Runtime/package readiness | SyncAgent 进程状态、stop-file 停止、可信压力入口 |
| V0.28 | Real Canal lab | Canal Docker lab、Edge/Server Canal E2E、soak 脚本 |
| V0.29 | Package smoke | `DataSync.exe` + `SyncAgent.exe` 固定目录 smoke，试用 runbook |
| V0.30 | Retry closure | 失败重试、批量重试、死信查看闭环已通过 |
| V0.31 | Installer alpha + read-only MCP | 受管组件执行器 alpha 和 stdio MCP 只读诊断已通过 |
| V0.32 | 11-node soak | 10 Edge 汇总、多主 fanout、断网恢复和队列积压长测已具备脚本入口 |
| V0.33 | Installer beta 1 preflight | 离线包 catalog、SHA256 校验和命令计划，不真实安装 |
| V0.34 | Installer beta 2 VM | 隔离 Windows VM 内执行 Erlang/OTP 与 RabbitMQ 静默安装、服务检测和卸载验证 |
| V1.0 | 产品化收口 | Wails 管理端、托盘常驻、无感安装、诊断、真实 CDC 完成 |

## V0.2 目标

- MySQL 连接池与迁移执行。
- `mapper.MappedEvent` Apply Worker。
- `sync_apply_log` 幂等。
- `alarm_history` 和 `device_config/device_settings` MVP 表支持。
- `sync-agent migrate` 与 `sync-agent apply-event`。
- Docker MySQL 只用于集成测试；连接串通过环境变量注入。

## V0.3 目标

- RabbitMQ 真实连接管理。
- `sync-agent publish-event`。
- `sync-agent consume-once`。
- publisher confirm 与 manual ack 的真实 broker 验证。
- Docker RabbitMQ 只用于集成测试；连接串通过 `NODEBRIDGE_RABBITMQ_URL` 注入。

## V0.3 Smoke

命令说明见 `docs/v0.3-smoke.md`。

## V0.4 目标

- `internal/syncruntime` 作为长期服务核心，不把同步流程写死在 CLI 里。
- Edge upload runtime：从本地上传队列取消息，发布到 Server ingress，成功后 ACK，失败 NACK。
- Edge downlink runtime：从 Server RabbitMQ 的 `<node_id>.downlink.q` 消费，下发 Apply 成功后 ACK。
- Server ingress runtime：从 Server ingress 取消息，按规则映射，调用 Apply Worker，成功后 ACK。
- Server 下发：多向/Server-to-Edge 规则按节点列表分发，必须跳过 `origin_node_id`。
- Worker 循环：支持空队列休眠、错误重试、上下文取消和状态上报。
- 日志 Web：默认关闭，只读 `/healthz`、`/status`、`/logs`，远程访问必须配置 token。
- `sync-agent run`：按 `edge/server` 模式启动当前 worker 组，并挂接日志 Web 生命周期。
- CLI 保留开发 smoke 入口：`forward-upload-once`、`consume-once`。
- worker 日志：固定容量 ring buffer，只记录脱敏后的 worker 运行摘要。

## V0.4 Smoke

开发阶段先用单步命令验证 runtime：

```powershell
sync-agent forward-upload-once `
  -local-amqp-url $env:NODEBRIDGE_EDGE_RABBITMQ_URL `
  -server-amqp-url $env:NODEBRIDGE_RABBITMQ_URL

sync-agent consume-once `
  -config configs/server.example.yaml `
  -rules configs/sync-rules.example.yaml `
  -amqp-url $env:NODEBRIDGE_RABBITMQ_URL
```

## V0.5 目标

- `internal/normalizer`：把 CDC `ChangeEvent` 转成标准 `SyncEvent`。
- `CDCUploadRuntime`：串联 loop suppressor、normalizer 和 RabbitMQ publisher。
- `cdc.StubSource`：提供测试和开发 smoke 输入源。
- `sync-agent publish-change-once`：读取模拟 CDC JSON 并发布到本地上传队列。
- 真实 Canal client、offset 持久化和 binlog 重连留到后续增量。

## V0.5 Smoke

命令说明见 `docs/v0.5-cdc-stub.md`。

## V0.6 目标

- `cdc.Offset` 和 `OffsetStore`。
- Canal adapter 前置接口，不绑定真实第三方 client。
- Canal row change 到 `ChangeEvent` 的转换。
- `FetchOnce` 完成 fetch、convert、save offset、ack。
- `sync-agent canal-check` 校验 Canal 配置。

## V0.6 Smoke

命令说明见 `docs/v0.6-canal-prep.md`。

## V0.7 目标

- MySQL offset store 写入 `sync_upload_offset`。
- CDC recovery policy：指数退避、最大延迟和最大次数。
- fatal error 显式标记，避免无限重试不可恢复错误。
- 恢复策略文档化，为真实 Canal client 接入做准备。

## V0.7 Smoke

命令说明见 `docs/v0.7-cdc-recovery.md`。

## V0.8 目标

- `syncstore.Store`：ACK、dispatch、error 持久化。
- 失败事件查询 CLI：`failed-events`。
- 失败事件重试标记 CLI：`retry-event`。
- 真实重放暂不做，等待事件载荷持久化。

## V0.8 Smoke

命令说明见 `docs/v0.8-persistence.md`。

## V0.9 目标

- `sync_event_log.event_payload` 保存原始 `SyncEvent`。
- Server ingress 先持久化 payload，再 Apply，再更新成功状态。
- `ReplayRuntime` 从 `PENDING` ACK 读取 payload 并重新下发。
- `sync-agent replay-pending-once` 提供开发阶段单步重放入口。
- 自动后台重放、Wails 失败事件页面留到后续增量。

## V0.9 Smoke

命令说明见 `docs/v0.9-replay.md`。

## V0.10 目标

- Server `sync-agent run` 同时启动 `server-ingress` 和 `server-replay`。
- `server-replay` 独立连接 RabbitMQ，避免与 ingress 共用 publisher channel。
- replay worker 使用现有 worker 状态和日志 Web 暴露运行状态。
- 单步 CLI `replay-pending-once` 保留为开发 smoke 入口。

## V0.10 Smoke

命令说明见 `docs/v0.10-auto-replay.md`。

## V0.11 目标

- 提供单机 lab 配置：`configs/lab/edge-a.local.yaml`、`edge-b.local.yaml`、`server.local.yaml`。
- 提供开发 Docker Compose：1 个 RabbitMQ + 3 个 MySQL。
- RabbitMQ 使用 vhost 隔离 Edge A、Edge B、Server。
- 提供 `scripts/lab-smoke.ps1` 初始化 lab 服务、迁移和拓扑。
- 明确 Docker 仅用于开发测试，不进入最终交付路径。

## V0.11 Smoke

命令说明见 `docs/single-machine-lab.md`。

## V0.12 目标

- 提供 `scripts/lab-e2e.ps1` 串联当前 MVP 链路。
- 使用 stub CDC，不依赖真实 Canal。
- 跑通 Edge A -> Server -> Edge B。
- 验证 Server 和 Edge B 的 `device_settings.setting_value = ON`。
- Docker CLI 不可用时允许跳过行级验证，但脚本和 Go 门禁仍必须通过。

## V0.12 Smoke

命令说明见 `docs/v0.12-e2e-smoke.md`。

## V0.13 目标

- 引入可替换的真实 Canal client adapter。
- 将第三方 Canal Go client 限定在 `internal/cdc/canal`。
- 支持 Canal protobuf message 到 `cdc.ChangeEvent` 的转换。
- 支持 Canal batch ACK 和现有 offset 保存流程。
- 真实 Canal client 先以 adapter 形式隔离，后续接入 Edge runtime。

## V0.13 Smoke

命令说明见 `docs/v0.13-canal-client.md`。

## V0.14 目标

- Edge `sync-agent run` 在 `cdc.type: canal` 时启动 `edge-cdc-canal`。
- 新增 `canal-publish-once`，用于真实 Canal 单步测试。
- Canal batch 只有在 RabbitMQ 发布全部成功后才保存 offset 并 ACK。
- worker 停止时关闭 Canal source。
- 保留 stub CDC 单机 E2E，直到 lab compose 增加 Canal Server。

## V0.14 Smoke

命令说明见 `docs/v0.14-canal-runtime.md`。

## 交付判断

- 技术试点最早在 V0.12 后：真实 CDC 路径和 Docker E2E smoke 必须跑通。
- 客户试用最早在 V0.23 后：Wails 托盘常驻、runtime 控制、真实 CDC E2E、诊断包和安装 runbook 必须具备。
- 产品交付以 V1.0 为目标：离线安装、RabbitMQ 默认安装/外部配置、配置加密和断网恢复验收必须完成。
- 详细评估见 `docs/delivery-assessment.md`。

## Wails 与日志 Web

- Wails 前端默认不占用端口。
- 前端通过 Wails binding 直接调用 Go 后端方法。
- 管理界面不作为 Web server 暴露。
- 日志模块允许在 `SyncAgent` 内启用一个轻量 HTTP 服务，便于远程排查。
- 日志 Web 默认关闭或仅绑定 `127.0.0.1`；远程排查时通过配置开启。

```yaml
log_web:
  enable: false
  bind: 127.0.0.1
  port: 18080
  token: encrypted_token
```

## 测试门禁

- 默认必跑：`go test ./...`、`go vet ./...`。
- `golangci-lint` 已安装时必须运行。
- Docker 集成测试必须显式设置 DSN/URL 环境变量才运行。
