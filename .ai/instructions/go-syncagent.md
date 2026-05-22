---
description: "Use when: Go SyncAgent 后端、cmd/sync-agent、internal 包、RabbitMQ、MySQL、CDC、Apply Worker、Windows Service"
applyTo: "**/*.go,cmd/**,internal/**,configs/**,migrations/**,deploy/**"
---

# Go SyncAgent Rules

## 进程边界

- 当前试用版采用 `DataSync.exe` 托盘常驻模型，Wails 关闭窗口不等于退出。
- 后端只提供退出鉴权、自启动、配置、状态和运行时控制接口；托盘 UI 由前端处理。
- `SyncAgent.exe` 仍保留为长期运行核心入口，后续可切换 Windows Service。
- 不要把托盘交互、退出弹窗、页面状态写入 Go 后端。

## 前后端协作

- 后端任务开始前必须读取 `.ai/docs/ai-collaboration-log.md` 的 Active Board。
- Wails 方法、DTO 字段、错误语义、页面所需数据变化，先写 Active Board，再改 contract 和代码。
- 后端发现需要前端处理的问题，必须写入 Active Board，不只写在最终回复里。
- 不新增单独的后端看板；稳定接口只维护 `.ai/docs/frontend-backend-contract.md`。

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

## 安装边界

- 默认安装器只能管理 NodeBridge 自己创建并登记在 `install-manifest.json` 的资源。
- `rabbitmq.mode=external` 或 `cdc.mode=external` 时，只读取连接配置，不创建、不删除、不改权限。
- RabbitMQ/Canal 受管资源必须带 NodeBridge 标识，例如 `NodeBridgeRabbitMQ`、`/nodebridge-server`、`nb-*`、`NodeBridgeCanal`。
- 卸载时只清理 manifest 记录的资源，客户已有 RabbitMQ/Canal/MySQL 不属于 NodeBridge 资源。

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
