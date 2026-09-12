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

- 本节点的冲突修复只凭 `sync_repair_replay` 的事件/节点/库/表/实际操作证明过滤；不得简单移除 `updated_by_node != local` 限制而误过滤业务自己的标记更新。UPDATE必须识别新修复标记，保留旧标记的本地UPDATE继续上传；DELETE不能借用旧INSERT/UPDATE修复证明。
- 修复任务及获胜镜像与版本同事务持久保存。捕获线程不得等待业务行锁做恢复；修复worker必须先锁业务行/间隙，再等新鲜CDC屏障，之后锁版本并重新读取当前winner。修复/来源证明/日志/清任务同事务，不用修复时间创造新的业务版本。

- 多向表必须包含：`sync_version`、`updated_by_node`、`last_event_id`、`updated_at`。
- 每个节点必须维护 `sync_apply_log`。
- Apply Worker 写业务表和写 `sync_apply_log` 必须在同一事务。
- INSERT 的远端 `last_event_id` 命中 `sync_apply_log` 时可判定回放；UPDATE 还必须检查 FULL 前后镜像中的同步标记变化。标记保持不变的后续本地 UPDATE 必须上传，不得仅因旧收据存在而过滤；缺少必要前镜像时返回错误，不猜测来源。
- HARD DELETE 的回放必须有删除事务来源证据，不能把删除前遗留的 INSERT/UPDATE 标记当作删除回放证据。双向映射的 TrackDeleteReplay 路径在同事务更新专用标记、删除、写 sync_delete_replay 和 sync_apply_log；专用收据按事件/来源/库/表查询，普通旧收据不算删除证明。该路径要求 FULL 元数据及无触发器且可核验；普通单向删除不增加标记 UPDATE。完整双向运行验收前仍保持能力门禁。
- Server 分发时不得把事件发回 `origin_node_id`。

## 首次对齐约束

- 当前快照/传输/恢复仅为内部原语，不能把复制收据当作CDC交接完成。`sync_alignment_job`的所有现有phase都阻断Agent；禁止以删账本或直接改phase解除阻断。
- 源端持表行/间隙锁到目标提交收据返回，最终提交请求前持久保存SOURCE_READY行数与摘要。目标复制行、原事务捕获标记和收据同事务；RabbitMQ数据消息只能在目标提交后ACK。
- 探针读取Canal不ACK，关闭后原业务输入必须可重读。恢复只观察已提交的原标记，不能拿新脉冲位置替代；普通过期计划禁止新复制，已提交目标的核对恢复可在过期后进行。
- 正式启用前必须完成旧事件过滤、完整来源血缘验证、全部参与节点就绪/启用握手及维护租约编排。不能清空已有行版本/墓碑，不能扩大参与节点范围或假设远端库表同名。
- context取消并不证明MySQL autocommit未执行，也不证明上游Canal socket已中止；提交结果未知须按持久结果核对，不可盲目重复写入。

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
