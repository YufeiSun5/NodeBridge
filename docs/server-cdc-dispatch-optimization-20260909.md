# Server CDC Dispatch Optimization / Server CDC 分发优化

- Date: 2026-09-09
- Owners: `backend-ai` implementation, `test-ai` validation, `review-ai` focused read-only review.
- Board: FB-052 / FB-053 / FB-054.
- Scope: Server Canal dispatch, publisher confirm safety, failed Canal batch retry, batch apply idempotency, validation harness.
- No Wails API/DTO, SyncEvent wire format, configuration or database schema changes.

## Failure Evidence

The original `overnight-20260908-184000` stopped after about 3 hours 3 minutes:

| Direction | Source Rows | Target Rows |
| --- | ---: | ---: |
| Edge -> Server | 388,401 | 388,401 |
| Server -> Edge | 388,401 | 204,484 |

Server dispatch success logs were 204,493: 204,484 applied rows plus 9 remaining downlink messages. Both error logs were empty and downlink queue depth stayed near zero. This places the observed backlog before the Edge apply queue, not in its consumer. See `v0.46.6-real-bidirectional-overnight-report-20260909.md` for the original evidence.

原路径对每条 Server change 执行一次 PENDING 日志自动提交、一次同步 publisher confirm、一次 SUCCESS 日志自动提交。1000 条需要 2000 次日志提交和 1000 次串行确认，空表实测速率已经低于原长测约 40 条/秒的输入。

“历史日志增长直接导致后半程降至约 14 条/秒”仍不是已证明的因果结论。独立百万历史日志对照仅把旧路径从约 35.46 降至 34.03 条/秒；后半程进一步下降可能涉及整机 IO、CDC 读取/系统表 binlog 放大等，需要长期分段剖析。<!-- 待确认 -->

## Implementation

1. `ServerCanalDispatchRuntime` 先按原顺序完成回环判断、标准化、规则选择、映射和 DDL 门禁，再集中写入 PENDING 日志。
2. 下发通过可选 `BatchDownlinkDispatcher` 和 `RoutingDownlinkDispatcher.DispatchBatch` 发布；只有连续 `append_only INSERT` 可合并确认。CRUD/DDL 每个事件构成独立确认屏障，前一组成功后才发下一组；保留单条 dispatcher/publisher/store 的兼容回退。
3. 同一事件按目标节点顺序 fanout，跳过空节点和 `origin_node_id`；ACTIVE 节点最多查询一次，下个批次重新查询，包括空结果。
4. 所有发布确认成功后才批量写 SUCCESS 日志，之后才提交 Canal offset。既有 `UpsertEventLogs` 仍用事务内 prepared per-row SQL，并未改为多值 INSERT；主要减少事务提交次数。
5. publisher 最多保留 64 条未消费确认，发满窗口就排空。NACK 排空当前窗口后返回失败；取消后保留 pending 计数，下次发送前先处理旧确认，避免旧 ACK 被误认成新消息成功。
6. 不确定的底层发送错误或确认通道关闭后禁止复用 publisher，需要重建 channel/publisher。当前长运行 Agent 没有在这里新增自动 AMQP 重连。
7. Edge upload 和 Server dispatch 在任一处理错误后 reset Canal source。实际 connector 重连回滚未确认批次，不再让下一轮 Fetch 跳过失败批次；稳定 CDC event ID 和目标 apply log 保证重复发送幂等。

审阅发现并修复初稿的保序缺口：如果同键旧更新被 NACK，而后续新更新已经发送成功，整批重试可能让旧值生效、新值因已存在 apply log 而跳过。最终版禁止 CRUD/DDL 越过未确认前序事件，回归测试模拟选择性拒绝和接收端幂等，核对最终值仍为最新值。选择性拒绝并非纯理论假设，RabbitMQ 的 `reject-publish` 溢出策略会产生 NACK，多队列路由还可能部分接收。[RabbitMQ Queue Overflow Behaviour](https://www.rabbitmq.com/docs/maxlength#queue-overflow-behaviour)

## Measured Results

`TestIntegrationServerDispatchPerformance` creates a unique temporary MySQL database and durable RabbitMQ exchange/queue, dispatches 1000 normalized changes through the actual runtime, and checks 1000 SUCCESS logs plus unique FIFO messages. It removes only its own test resources. The Canal source is a deterministic fixture; these numbers do **not** measure full end-to-end throughput.

| Historical Log Rows | Implementation | 1000 Events | Events/s | Log API Calls / Time | Publish API Calls / Time |
| ---: | --- | ---: | ---: | --- | --- |
| 0 | Before | 28.204 s | 35.46 | 2000 / 21.776 s | 1000 / 6.274 s |
| 0 | Final append_only | 1.949 s | 513.21 | 2 / 1.692 s | 1 / 0.202 s |
| 1,000,000 | Before | 29.382 s | 34.03 | 2000 / 23.000 s | 1000 / 6.222 s |
| 1,000,000 | Final append_only | 3.260 s | 306.73 | 2 / 2.809 s | 1 / 0.327 s |
| 0 | Final crud_ordered | 7.205 s | 138.80 | 2 / 1.897 s | 1000 / 5.222 s |

在本机 MySQL 8.0.38 与 RabbitMQ 实测中，最终代码的追加写分发段空表约提升 14.5 倍，百万历史日志约提升 9.0 倍。保序 CRUD 仅合并日志事务，不合并不同事件的确认。这是各条件单次受控样本，不是多轮统计保证；绝对值会受存储负载、缓存、网络和 fanout 数量影响。历史数据每行带 512 字节 payload，测试事件带 128 字节业务 payload。初稿追加写样本为 599.10 / 358.37 条每秒；以上表格使用保序修正版的重测值。

运行入口需要设置 `NODEBRIDGE_DISPATCH_MYSQL_DSN` 和 `NODEBRIDGE_RABBITMQ_URL`，可选 `NODEBRIDGE_DISPATCH_HISTORY_ROWS=1000000`、`NODEBRIDGE_DISPATCH_SYNC_MODE=crud_ordered`（默认 append_only）：

```powershell
go test ./internal/syncruntime -run '^TestIntegrationServerDispatchPerformance$' -count=1 -v -timeout 12m
go test ./internal/rabbitmq -run '^TestIntegrationPublishBatchFIFO$' -count=1 -v
```

不要指向未授权数据库服务器。测试需要创建/删除独立库和 RabbitMQ 测试资源的权限；报告不包含凭据。

## Reliability Coverage

- 两次日志写入、发布确认和 Canal commit 的顺序断言。
- 标准化、JSON 编码、映射、节点查询、PENDING 日志、发布、部分发布、SUCCESS 日志、取消和 commit 失败后，不提交或越过原批次。
- 有前后两个批次的回滚源夹具，验证失败后重试第一批、稳定 ID 不变，成功后才处理第二批。
- ACTIVE/SELECTED 目标、防回源、空目标刷新、禁用/未匹配/IGNORE/NONE 规则。
- 同一批次 INSERT -> ADD_COLUMN -> UPDATE -> DROP_COLUMN -> DELETE 顺序和 DDL 开关。
- 选择性 NACK + 同键更新 + 幂等重放后最新值保持正确；连续追加写不得越过 CRUD/DDL 确认屏障。
- publisher 65/128/1000 条边界、NACK、取消后迟到确认、部分发送、关闭及并发批次保序。
- 真实 RabbitMQ 1000 条 FIFO 集成测试通过。

## Real Bidirectional Short Test

- Initial run: `fb052-smoke-20260909`; final corrected run: `fb052-final-20260909`.
- Lab: `192.168.10.105 Edge <-> 192.168.10.103 Server`.
- Final Server test binary: `.cache/fb052-final/SyncAgent.exe`, SHA256 `8C3F77BEC94B5B1E6D14094288DB70F275CE828368C30CD3643B7D87EDB2B127`.
- Edge retains installed v0.46.6; original Server binary is untouched, SHA256 `6B8F2B931941A41E6D61FB4B086D12FA36218E4B5A4BE392174EC3E4549D21D7`.
- Requested load: 200 rows per batch in each direction, 1 second between batches; actual cadence also includes SQL, snapshots and fault orchestration.
- Evidence: `.cache/real-bidirectional-overnight/fb052-smoke-20260909/` and `.cache/real-bidirectional-overnight/fb052-final-20260909/`.
- Initial 6-minute run: both directions 40,402 / 40,402 with equal CRC summaries, 20 CRUD cycles, both Agent outages, ADD/DROP, idempotency and drain passed; both error logs zero. Original rules and binaries restored. This run preceded the selective-NACK correction and does not replace final-binary validation.
- Final corrected run failed at the duplicate-event probe (09:28:34), after 16,002 stream rows per direction, eight CRUD cycles, both Agent outages and ADD/DROP. The last periodic snapshot was 15,602 rows per direction. Final checksums and all-scenario acceptance were not reached; this run is **failed**, not passed. Original rules and Agent paths were restored at 09:28:45.

测试夹具同步修正：CRUD 使用独立递增轮次，避免总写入轮次总是偶数而漏测部分分支；实际等待映射 DELETE 软删除；最终门禁拒绝缺失场景、双端都不存在的状态行及非零错误日志；恢复时使用原 Server 二进制路径。

## FB-054 Batch Idempotency

根因：`append_only` 分组只检查已提交的 `sync_apply_log`，没有检查当前批次内再次出现的同一 `event_id`。重复消息被加入同一多行 INSERT，触发 MySQL 1062，整批回滚并反复重试。独立 `consume-batch-once` 已在真实残留队列复现。

修复：追加写分组使用局部已见 ID 集合，只有首次出现的事件参与业务 SQL 和 apply log；重复消息仍按原位置返回 `AlreadyApplied`，保证消费者 ACK 前缀与投递位置一致。局部集合不提前修改已提交状态。compact 遇到本组重复 ID 结束合并，先提交前面的新更新，再跳过重复旧更新，避免 E1/E2/E1 恢复旧值。批量 apply-log 查询同时移除重复参数。

没有恢复 `INSERT IGNORE`：不同 `event_id` 的业务主键冲突和数据超长仍必须失败，不允许静默吞错。

新增 `internal/apply/batch_idempotency_test.go` 覆盖纯追加、混合段、已提交重放、跨段重复、每条消息结果位置、compact 最新值、不同事件同业务键失败，以及日志失败时只返回已提交前缀。

修复后用 `.cache/fb054/SyncAgent.exe` 和失败 run 的原测试规则消费 4 条残留消息成功。现场 SQL 核验 `duplicate-fb052-final-20260909` 对应业务主键 `17889170738000000`：业务行 1、apply log 1、SUCCESS event log 1；入口队列预览为空。该定点恢复验证不把此前失败的完整短测改写成通过。

## Gates And Limitations

- `go test ./... -count=1`: passed.
- `go vet ./...`: passed.
- `golangci-lint run ./...`: passed, 0 issues.
- PowerShell parser: passed.
- `go test -race ./internal/rabbitmq ./internal/syncruntime`: unavailable because CGO is disabled and the local C toolchain is absent.
- Full long-duration repeat has not run. The user's next requested duration is seven hours with explicit UPDATE and approximately 50-column tables; design follows installer production, with no automatic start. Short tests cannot establish long-term IO stability or replace the failed overnight acceptance.
- Partial publish can duplicate an already delivered prefix; delivery remains at least once and depends on target idempotency.
- Broker connection loss still requires the existing restart/rebuild path; no claim of automatic reconnect or broker outage acceptance.
- v0.46.7 installer production and isolated upgrade verification passed. Final artifact/hash and packaged-binary 4+4 duplicate regression are recorded in `v0.46.7-package-validation-20260909.md`. No release, remote permanent upgrade or source commit has been performed. Seven-hour wide-table design is complete; execution is not started.
