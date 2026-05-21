---
description: "Use when: Go SyncAgent 后端、cmd/sync-agent、internal 包、RabbitMQ、MySQL、CDC、Apply Worker、Windows Service"
applyTo: "**/*.go,cmd/**,internal/**,configs/**,migrations/**,deploy/**"
---

# Go SyncAgent Rules

## 进程边界

- `DataSync.exe` 负责管理界面和控制操作。
- `SyncAgent.exe` 负责长期运行的同步任务。
- 不要把 CDC、MQ 消费、Apply Worker 等长期运行逻辑塞进 Wails UI 进程。

## 包职责

- `internal/appconfig`: 配置加载、保存、校验、脱敏和密码加密。
- `internal/cdc`: CDCReader 接口和 Canal/Debezium 适配。
- `internal/event`: 标准 `SyncEvent` 协议模型。
- `internal/normalizer`: `ChangeEvent -> SyncEvent` 转换。
- `internal/rabbitmq`: 连接、拓扑初始化、发布确认、手动 ACK。
- `internal/apply`: 幂等、事务、业务表写入、`sync_apply_log`。
- `internal/loop`: 回环抑制判断。
- `internal/router`: Server 分发和防回源。
- `internal/status`: 运行状态、队列深度、健康检查。

## Go 约定

- 包名短小且以职责命名，不使用 `utils`、`common` 承载核心逻辑。
- 跨模块数据结构优先放在清晰的领域包中，例如 `event.SyncEvent`。
- 长生命周期组件接受 `context.Context`，支持停止和资源释放。
- 外部 IO 必须返回可诊断错误，日志中带 `event_id`、`node_id`、`table`、`pk` 等关键字段。

## 可靠性约束

- RabbitMQ 发布必须使用 publisher confirm。
- 消费者必须在业务写入和系统日志提交成功后才 ACK。
- Apply Worker 必须先查 `sync_apply_log.event_id` 实现幂等。
- MySQL Apply 必须使用事务包裹业务写入和系统表写入。

## 示例接口

```go
type CDCReader interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Events() <-chan ChangeEvent
    Errors() <-chan error
    SaveOffset(ctx context.Context) error
    LoadOffset(ctx context.Context) error
}
```
