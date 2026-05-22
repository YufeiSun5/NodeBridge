# Sync Routing Policy

本文件定义同步规则如何表达“写中心、下发边缘、指定节点分发、表列重映射”。

## 核心判断

中心节点不是普通 downlink 目标。

- Edge 事件上传到 Server 后，Server ingress 先 Apply 到中心 MySQL。
- 之后是否下发给 Edge，由 `dispatch_target` 决定。
- Server 本地数据库变更已经发生在中心库，不需要再“分发给中心”，只需要下发给 Edge。
- 回源节点默认跳过，避免同步回环。

## Rule Fields

| 字段 | 作用 |
| --- | --- |
| `direction` | 默认业务流向：`EDGE_TO_SERVER`、`SERVER_TO_EDGE`、`BIDIRECTIONAL`、`IGNORE`。 |
| `dispatch_target` | 覆盖分发范围：`AUTO`、`NONE`、`ACTIVE_EDGES`、`SELECTED_EDGES`。 |
| `dispatch_node_ids` | `SELECTED_EDGES` 时指定目标 Edge。 |
| `source_node_ids` | 同名源表按来源节点匹配不同规则。 |
| `target_database_name` / `target_table_name` | Apply 前的目标库表映射。 |
| `column_mappings` | Apply 前的目标列名映射。 |

默认兼容：

- `BIDIRECTIONAL`、`SERVER_TO_EDGE` 默认 `dispatch_target: ACTIVE_EDGES`。
- `EDGE_TO_SERVER` 默认 `dispatch_target: NONE`。

## 场景 1：Edge 汇总到 Server，表名或列名重映射

适合 `data_all` 这类从节点同名表，中心按节点拆表。

```yaml
- id: data-all-edge-001
  database_name: scada_edge
  table_name: data_all
  source_node_ids: [edge-001]
  target_database_name: scada_center
  target_table_name: data_all_edge_001
  direction: EDGE_TO_SERVER
  dispatch_target: NONE
  primary_keys: [id]
```

结果：

- Edge 写 `data_all`。
- Server 写 `data_all_edge_001`。
- 不下发其他 Edge。

## 场景 2：Edge 多主变更，写 Server 后分发其他 Edge

适合 `device_config`、`point_config` 这类多主同构表。

```yaml
- id: device-config-multimaster
  database_name: scada_edge
  table_name: device_config
  target_database_name: scada_center
  target_table_name: device_config
  direction: BIDIRECTIONAL
  dispatch_target: ACTIVE_EDGES
  primary_keys: [id]
```

结果：

- Edge A 写本地表。
- Edge A 上传到 Server。
- Server Apply 到中心库。
- Server 下发 Edge B..N。
- Edge A 默认不回源。

如果某张表虽然是 `EDGE_TO_SERVER`，但也要下发其他 Edge，可以显式打开：

```yaml
direction: EDGE_TO_SERVER
dispatch_target: ACTIVE_EDGES
```

## 场景 3：Server 数据库变更，下发所有 Edge

Server-side CDC 产生的事件应使用中心库表作为源规则。

```yaml
- id: server-device-config-downlink
  database_name: scada_center
  table_name: device_config
  target_table_name: device_config
  direction: SERVER_TO_EDGE
  dispatch_target: ACTIVE_EDGES
  primary_keys: [id]
```

结果：

- Server 本地表已变化。
- Server CDC 生成 server-origin `SyncEvent`。
- SyncAgent 不重复 Apply 中心库。
- Server 下发所有 ACTIVE Edge。

如果中心表名和边缘表名不同，使用 `target_table_name` 和 `column_mappings` 映射。

## 场景 4：只下发指定 Edge

```yaml
dispatch_target: SELECTED_EDGES
dispatch_node_ids: [edge-002, edge-005]
```

结果：

- 只给指定节点创建 downlink 消息。
- 如果列表里包含 `origin_node_id`，仍默认跳过。

## V0.25 验收状态

- `data_all` 汇总规则已在 11 节点 lab 验证：10 个 Edge 同名源表写到中心 `data_all_edge_001..010`，不下发 Edge。
- `device_config` / `point_config` Edge-origin 多主 fanout 已验证：来源 Edge 写中心，再下发其他 9 个 Edge，跳过来源节点。
- Server-origin 下发已验证：中心库变更生成 server-origin event，下发 10 个 Edge，不重复 Apply 中心库。
- `SELECTED_EDGES` 已验证：只下发指定 Edge，其他 Edge 不收到 downlink。

## 下一步

V0.26 重点是把 Server-side CDC 从单步/worker 接口推进到真实 Canal 长时间运行 soak，并补充 11 节点延迟、吞吐和冲突策略测试。
