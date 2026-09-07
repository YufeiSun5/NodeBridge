# 90 天等价长测 Harness

本 harness 给 `test-ai` 使用，用单机 Docker 模拟 Edge/Server 两侧，并强制走真实 CDC：Docker MySQL binlog -> Canal -> SyncAgent -> RabbitMQ -> Server Apply -> MySQL。

## 拓扑

| Side | Component | Container | Host Port |
| --- | --- | --- | --- |
| Edge | MySQL | `nodebridge-longtest-mysql-edge` | `3341` |
| Edge | RabbitMQ | `nodebridge-longtest-rabbitmq-edge` | `5701`, `15701` |
| Edge | Canal | `nodebridge-longtest-canal-edge` | `11121` |
| Server | MySQL | `nodebridge-longtest-mysql-server` | `3342` |
| Server | RabbitMQ | `nodebridge-longtest-rabbitmq-server` | `5702`, `15702` |

配置文件：

- `deploy/docker-compose.longtest.yml`
- `deploy/rabbitmq/definitions.longtest.json`
- `configs/longtest/edge.local.yaml`
- `configs/longtest/server.local.yaml`
- `configs/longtest/sync-rules.yaml`

## 数据模型

`collect_data_01` 到 `collect_data_06` 均使用同构表：

- `id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY`
- `device_id VARCHAR(64)`
- `collected_at DATETIME(3)`
- `p001` 到 `p042`，类型为 `DOUBLE NULL`
- `updated_by_node`
- `last_event_id`
- `created_at`
- `updated_at`
- `idx_device_time(device_id, collected_at)`
- `idx_time(collected_at)`

恢复、查询和归档边界只按 `collected_at` 判断，不使用 `id`。

长测规则将 6 张采集表配置为 `sync_mode: append_only`。该模式用于历史倾倒/采集流水表，只接受 `INSERT`，Server Apply 可以使用多行批量插入；普通增删改表仍应使用默认 `crud_ordered`，保持现有保序、幂等、软删除和冲突策略。

## 写入形态

默认写入模式为 `time-interleaved`：

- 以 `collected_at` 时间窗口向前推进。
- 每个窗口内轮转写入 `collect_data_01` 到 `collect_data_06`，避免“先写完一张表再写下一张表”的假负载。
- 每张表内部仍按 `collected_at` 单调递增写入，顺序检查写入 `ordering-check.json`。
- 每个 `collected_at` 窗口应覆盖 6 张表，形态检查写入 `arrival-shape.json`。

如需复现旧的按表顺序灌入，可显式传 `-InsertMode table-sequential`；正式长测默认不使用该模式。

## 命令

准备环境：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action prepare
```

轻量 smoke：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action smoke -RowsPerTable 2 -DrainIterations 60
```

90 天等价灌入，默认 90 天、每表 2,592,000 行、总计 15,552,000 行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action seed-90d -ConfirmLongRun -BatchSize 500 -DrainIterations 30000
```

24 小时实时写入：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action realtime-24h -ConfirmLongRun -RealtimeHours 24
```

断线恢复：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action recovery -ConfirmLongRun -RecoveryScenario 1h -DrainIterations 3000
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action recovery -ConfirmLongRun -RecoveryScenario 1d -DrainIterations 12000
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action recovery -ConfirmLongRun -RecoveryScenario 7d -DrainIterations 60000
```

30 天断线恢复保留为第二轮压力项：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action recovery -ConfirmLongRun -RecoveryScenario 30d -DrainIterations 240000
```

查询性能：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action query
```

滚动归档验证：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action archive
```

证据回收：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action collect
```

可选长期 SyncAgent 进程：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action start-agents
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\longtest-90d.ps1 -Action stop-agents -RunId <same-run-id>
```

## 证据

每次运行写入：

```text
.cache/longtest-90d/<run-id>/
```

关键文件：

- `summary.json`
- `insert-plan.json`
- `ordering-check.json`
- `arrival-shape.json`
- `row-counts.json`
- `queue-depth.json`
- `queue-depth-samples.csv`
- `latency-samples.csv`
- `query-results.json`
- `explain-results.json`
- `archive-results.json`
- `nodebridge-longtest-*.log`

## 验收口径

- 24 小时实时同步：每 3 秒 6 行，正常延迟 `< 5-10s`。
- 90 天等价数据：总计约 15,552,000 行，Server 与 Edge 行数一致。
- 离线恢复：第一轮必须通过 `1h`、`1d`、`7d`；`30d` 为第二轮压力项。
- 查询：单设备单天 `< 1s`，最近 1 小时 `< 1s`，最近 7 天 `< 3s`。
- 归档：验证 `DROP PARTITION` 路径；当前主业务表仍保留 `PRIMARY KEY(id)`，归档动作使用 `_archive` 分区验证表，避免伪装产品已内置归档能力。

## 注意

- `seed-90d`、`realtime-24h`、`recovery` 都需要 `-ConfirmLongRun`。
- `recovery 1h` 使用计划给出的 43,200 行作为积压目标；`1d`、`7d` 分别为 172,800 和 1,209,600 行。
- 如果 Docker CLI、`canal/canal-server:latest` 镜像或网络不可用，先记录为环境阻塞，不修改产品代码判断结果。
