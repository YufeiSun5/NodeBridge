---
description: "Use when: 查看从当前骨架到 V1.0 结束的版本路线、下一版目标、测试门禁、Wails 与日志 Web 服务约束"
---

# Roadmap

## 当前状态

- V0.1 已完成：Go 骨架、配置样例、迁移样例、RabbitMQ 基础、表列映射契约和测试门禁。
- V0.2 已完成默认测试范围：MySQL 连接、迁移执行、`MappedEvent` Apply Worker、sample event 和 CLI 子命令。
- V0.3 已完成默认测试范围：RabbitMQ 拓扑初始化、事件发布、单条消费、成功 Apply 后 ACK。
- V0.4 已完成默认测试范围：Edge/Server Runtime、长期 worker、状态存储、日志 Web、`sync-agent run`、Edge downlink apply 和 worker 日志 ring buffer。
- V0.5 已进入实现：CDC stub、ChangeEvent 标准化和本地上传队列发布入口。
- V0.6 已完成默认测试范围：CDC offset 模型和 Canal adapter 前置接口。
- V0.7 已进入实现：MySQL offset store 和 CDC 异常恢复策略。
- V0.8 已进入实现：ACK、dispatch、error 持久化和失败事件入口。

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
| V1.0 | 产品化收口 | Wails 管理端、Windows Service、无感安装、诊断包完成 |

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
