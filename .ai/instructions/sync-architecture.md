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
- `SyncEvent` 保留源库、源表、源列语义；Apply 前必须通过规则映射为目标库、目标表、目标列。
- 任何同步链路不得假设源表名等于目标表名，也不得假设源列名等于目标列名。
- 表名、库名、列名用于 SQL 前必须做 identifier 校验，禁止拼接任意 SQL 片段。

## 表列映射

- 表映射由 `target_database_name` 和 `target_table_name` 描述，缺省时等于源库表。
- 当多个 Edge 的源库表同名但中心目标表不同，必须使用 `source_node_ids` 做节点作用域匹配。
- 列映射由 `column_mappings` 描述，未配置的列默认同名。
- `include_columns` 和 `exclude_columns` 使用源列名，过滤必须发生在列名映射之前。
- `primary_keys` 使用源列名；`target_primary_keys` 为空时按列映射自动推导。
- MVP 只做名称映射，不做类型转换、表达式计算、字段拆分或字段合并。
- CDC 和 RabbitMQ 不处理映射语义；映射必须在 Apply Worker 前完成。

## 分发策略

- `direction` 表示默认流向，不应写死所有场景。
- `dispatch_target` 可覆盖默认分发：`AUTO`、`NONE`、`ACTIVE_EDGES`、`SELECTED_EDGES`。
- `dispatch_node_ids` 只在 `SELECTED_EDGES` 下使用。
- 默认兼容旧规则：`BIDIRECTIONAL` 和 `SERVER_TO_EDGE` 分发到 ACTIVE Edge；`EDGE_TO_SERVER` 不分发。
- 需要“从节点上传到主节点后再分发给其他从节点”时，可配置 `direction: EDGE_TO_SERVER` 加 `dispatch_target: ACTIVE_EDGES`。
- 需要“只汇总到中心”时，配置 `dispatch_target: NONE`。
- Server 分发仍默认跳过 `origin_node_id`，避免回源。

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
