---
description: "Use when: SyncEvent、同步链路、Loop Suppressor、Apply Worker、Conflict Resolver、RabbitMQ topology、Edge Server 模式"
applyTo: "internal/event/**,internal/normalizer/**,internal/loop/**,internal/apply/**,internal/router/**,internal/rabbitmq/**,migrations/**,configs/**"
---

# Sync Architecture Rules

## 核心链路

```text
MySQL -> CDC -> ChangeEvent -> SyncEvent -> RabbitMQ -> Apply -> MySQL
```

- CDC 只负责读取 binlog，不负责判断回环。
- RabbitMQ 只负责缓存、转发、确认、重试和死信。
- SyncAgent 负责事件判断、回环抑制、冲突处理和路由分发。

## SyncEvent 不变量

- 所有 CDC 事件必须转换为统一 `SyncEvent`。
- `event_id` 使用全局唯一且大致有序的 ID，规划推荐 ULID。
- `origin_node_id` 表示事件最初产生节点。
- `source_node_id` 表示当前发送节点。
- `target_node_id` 仅在定向下发时设置。

## 回环抑制

- 多向表必须包含：`sync_version`、`updated_by_node`、`last_event_id`、`updated_at`。
- 每个节点必须维护 `sync_apply_log`。
- Apply Worker 写业务表和写 `sync_apply_log` 必须在同一事务。
- CDC 捕获本地变更后，如果 `last_event_id` 命中 `sync_apply_log` 且来源不是本节点，则不上传。
- Server 分发时不得把事件发回 `origin_node_id`。

## RabbitMQ 约束

- exchange、queue、message 都必须持久化。
- 使用 publisher confirm 确认 broker 接收发布。
- 使用 manual ack 确认消费者处理成功。
- 失败消息进入重试或死信，禁止无记录丢弃。

## MVP 范围

- 1 个 Server，2 个 Edge。
- `alarm_history`: 单向 Edge -> Server。
- `device_config`: 多向 Edge <-> Server <-> Edge。
- 第一版优先实现 `SERVER_WIN` 和 `LAST_WRITE_WIN` 冲突策略。
