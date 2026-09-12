# 双向峰值失败调查报告

身份：review-ai。首次落盘：2026-09-11 09:05 Asia/Singapore，产品修复之前。

## 结论与边界

### 09:39 更新：确认日志 UPSERT 的锁放大机制

用户确认方案后进行了有界原表续跑，未更改产品二进制。`peak-original-scene2-092608` 的原物理表、约60万行业务历史、热点实际版本和原系统日志均保留；每端新增44,560 INSERT/22,280 UPDATE，574.185秒完成新增范围50列、2000写入键及新增提交账本核验。不是旧缺失事件或七小时验收通过。

前六分钟没有持续积压。09:33:18-09:33:49对原中心Canal容器做30秒暂停/恢复，保持容器、Agent和MySQL参数不变，以制造短暂追赶。后段再次出现与旧故障同类的慢日志写入：09:35:28/31/37/39的CDC `event_log` 分别约1753/1905/1847/1876ms。入口ready峰值1031、下行2431，停止生产后均排空；没有达到125秒超时。

**关键新增证据**：1145份MySQL现场采样中4份捕获 `sync_event_log.PRIMARY` 的 `supremum pseudo-record` 锁等待：新写入请求 `X,INSERT_INTENTION`，另一事务持有 `X`。部分样本伴随另一个日志UPSERT或COMMIT；09:35:38-39 redo检查点距离约85.7MB，但仅有相关性，不能把redo容量直接认定为唯一原因。

随后在独占空库复制完全相同日志表schema，用两个事务做秒级、无负载的 UPSERT / UPDATE / UPSERT 对照：事务1修改已存在的日志，未提交时事务2插入不同event_id。两次UPSERT均出现1个锁等待，事务2必须等事务1回滚；状态UPDATE时锁等待为0，事务2可在事务1结束前完成。独占库已清理。这确认了**用INSERT ON DUPLICATE KEY UPDATE完成已有日志状态，会把状态写入的事务等待传递给独立的新日志插入**，不只是payload编码成本。

因此先修已证实的代码问题：CDC发布确认后的SUCCESS落盘改为按既有event_id执行状态UPDATE，保留首次完整PENDING、payload、提交后Canal ACK和失败重放；缺失日志必须报错，不能默默前移位点。修复前本段已落盘。该机制与旧故障阶段吻合，但旧125秒失败未完整重现，不能宣称长测已通过或所有底层I/O问题已排除。

18组探针：计划暂停前14组，上行最大609.623ms、下行832.565ms；包含暂停/追赶/后段慢写的全部样本最大分别10,487.701/20,821.791ms，不把这些慢样本隐藏或算作正式P99通过。

双端原5张表已备份：中心 `.cache/incident-20260911/scene-backup-0920/center.sql`，SHA256 `82B44450F8DFA0C711285D9F0C4D29AEA15686DDC605817373F1FE63A4723FEC`；Edge `D:/NodeBridge-Test-Backups/incident-scene-20260911-0920/edge.sql`，SHA256 `62600144B4F61775B986DDEAD9B2A86A08CE2DA48371A03FAAF219683B559D94`。原表现含诊断续写，旧失败快照由这些备份及原结果文件保存，未回滚系统日志。第一次续跑因PowerShell将正常stderr误当异常而提前终止，已修夹具并单独保留，不算产品失败。

恢复：中心15972/Edge19320，原规则CD26哈希一致，运行队列ready/unacked均0，旧11040条隔离消息保留；Canal未暂停，临时PFS开关已恢复。本轮前段统计频率低于旧脚本，约第3分钟起另外施加15秒快照/60秒统计；不是完全等价的故障时间线。

### 09:48 修复与验证

- `internal/syncstore/event_log_success.go` 新增按event_id分块的事务状态UPDATE，只修改status/applied_at/error_message，不重传或覆盖payload。重复ID去重；已经成功且值未变化可重入；缺失记录、任意分块失败和提交失败均返回错误，不静默补造日志。
- Canal批量与单事件CDC分发都在publisher confirm之后调用该能力，首次完整PENDING不变；状态提交失败不提交Canal位点，下一轮重放。兼容只实现旧EventLogStore的适配器，真实SQL Store走新路径；无Wails/SyncEvent/DTO/配置迁移变化。
- `TestEventLogSuccessAvoidsInsertGapMySQL` 直接使用产品SQL和相同MySQL表结构：UPSERT阻塞/状态UPDATE不阻塞/UPSERT阻塞，整个真实测试1.55秒。另验证跨128条边界缺失日志整批回滚、重复完成及payload保留；独占库已清理。
- 单元覆盖块边界、重复ID、空ID、取消、begin/exec/RowsAffected/query/第二块/commit失败；运行时覆盖PENDING→publish→状态UPDATE→位点提交及状态失败后的同批重放。
- 最终 `go test ./...`、`go vet ./...`、`golangci-lint run ./...` 通过，lint为0 issues。
- **修复仅在源码，未出包、未替换现场Agent，也未运行修复版双端长测。** 已消除可复现的状态UPSERT锁放大机制；不宣称完整125秒超时的所有诱因已排除。MySQL FULL binlog、redo容量和刷盘可靠性均未调整。
- FB-096锁缺陷与FB-095本轮报告/受控修复交付关闭；FB-094原长测失败与修复版整链路验收继续open。

### 初次阶段结论（09:05）

1. **两次七小时测试均未通过**。0.46.12 与 0.46.13 都在第 270 分钟切入每方向 `80 INSERT/s + 40 UPDATE/s` 后约 6 分钟，因同步延迟探针超过 125 秒失败。不能把进程恢复、已完成探针的 P99、短测通过替代完整验收。
2. **故障时的直接瓶颈在中心链路**：Edge 上传约 120 条/秒，中心入口实际处理约 80-92 条/秒，入口队列持续积压。中心 CDC 与入口 Worker 的慢步骤主要落在 `event_log`，而不是 Edge 下行 Apply；0.46.13 的下行批量优化没有解决这一现场问题。
3. **底层等待机制尚未唯一确认**。相同二进制、数据库历史与可靠性设置下，新测试表的持续峰值复现已通过；不能据此宣布原故障消失，也不能把 redo、内存、日志写放大单独归为根因。下面明确记录已排除的简单解释和下一项受控对照。
4. 恢复后剩余的 11,040 条旧测试消息曾阻塞正常规则。此次已逐条确认归属，发布确认后保留到 `nb7_wide25200s_0910_213112.quarantine`，没有 purge。正常规则与 Agent 已恢复，运行队列为零。

<!-- 待确认：完成大批次、历史索引分布及数据库等待对照后，更新具体根因和修复结论。不得将本阶段报告标为性能修复完成。 -->

## 失败证据

| 项目 | 0.46.12 | 0.46.13 |
| --- | --- | --- |
| Run | `wide7h_0910_100650` | `wide25200s_0910_213112` |
| 开始 | 09-10 10:07:37 +08 | 09-10 21:32:04 +08 |
| 失败/恢复结束 | 14:44:10 / 14:44:25 | 09-11 02:08:12 / 02:08:29 |
| 失败阶段 | 第 276 分钟附近，峰值 | 第 276 分钟附近，峰值 |
| 入口 ready | 13,585 | 10,925，另有 50 unacked |
| 错误 | `latency probe exceeded 125 seconds` | 同左 |

0.46.13 每方向生产 `605,440 INSERT / 320,720 UPDATE` 后停止。两次 15 分钟 Agent 故障、在线追平及 ADD/DROP COLUMN 已完成，但最终一致性没有验收。最后采样磁盘净下降 Edge 6.19 GB、Server 22.68 GB，不是容量底线触发。

候选 Agent SHA256：`6C07957C7802F1BCC2EC624FB5E5CE89368CCBC51A513B761DEB5011EB42C30D`。

原始证据：

- `.cache/wide-seven-hour/wide7h_0910_100650/`
- `.cache/wide-seven-hour/wide25200s_0910_213112/`
- 本轮保全的双端运行日志：`.cache/incident-20260911/center-logs/`、`edge-logs/`

## 慢步骤与数据库状态

0.46.13 的代表性中心日志：

| 时间 +08 | Worker | 总耗时 ms | 主要阶段 ms |
| --- | --- | --- | --- |
| 02:03:12.539 | server-cdc-canal | 3327 | event_log 2904、publish_confirm 631 |
| 02:05:16 附近 | server-ingress | 2049 | event_log 1912、mysql_apply 90 |
| 02:06:42 附近 | server-ingress | 2070 | event_log 1901、mysql_apply 132 |
| 02:08:00 附近 | server-cdc-canal | 3392 | event_log 3032、publish_confirm 593 |

阶段值不可直接相加：`RoutingDownlinkDispatcher.DispatchBatch -> publishCanalBatch` 与外层 `dispatchDownlinkBatch` 重复计入同名确认阶段。CDC 的 `StepResult.Count` 未填写，故 `reported_messages=0` 不代表没有处理 CDC。这两处是已确认的观测缺陷。

中心 MySQL 8.0.38：buffer pool 8 GiB，redo capacity 100 MiB，`innodb_flush_log_at_trx_commit=1`、`sync_binlog=1`。Edge buffer pool 2 GiB。此次没有扩大内存、修改刷盘可靠性或放宽门禁。

01:55-02:07 的中心 `Innodb_buffer_pool_reads`、`Innodb_data_read`、`Innodb_buffer_pool_wait_free` 均未增加。峰值时脏页和写入速率增加，但这不能单独证明 redo 是根因。历史 Performance Schema 累计耗时出现接近 uint64 上限的异常值，本报告不使用这些累计时间排序归因。

## 源码审阅

- `internal/syncruntime/canal_runtime.go`：中心 CDC 先持久化 PENDING，再发布确认，再将相同完整记录 upsert 为 SUCCESS，最后提交 Canal 位点。
- `internal/syncstore/store.go`：多行 upsert 受 128 行/估算 1 MiB 约束，整个输入批次保持同一事务；重复更新仍携带完整 payload。
- `internal/syncruntime/runtime.go`：中心入口提交 Apply 后，另行写 SUCCESS 事件日志，成功后才 ACK。
- 两条中心链路共用 `sync_event_log`，但无 Store 全局互斥锁；数据库连接池未限制最大连接数。

完整 payload 重复传输是实际开销，但只有证实它与故障机制相关，才能把优化收益视为本故障的修复证据。必须保留 PENDING 可恢复性、publisher confirm、提交后 ACK 和位点顺序。

## 受控实验

### 事件日志双写

在独占临时库复制相同 schema，两个并发写入者分别模拟 CDC 的 PENDING/SUCCESS 和入口 SUCCESS，payload 大小约 2.2/4.1 KB。保留真实持久性设置。每轮 60 秒，完整 upsert / 状态 UPDATE / 完整 upsert，临时库结束后清理。

| 历史量 | 完整 upsert CDC 条/秒 | 完整 upsert 入口条/秒 | 状态 UPDATE CDC 条/秒 |
| --- | --- | --- | --- |
| 0 | 830 / 687 | 1568 / 1321 | 831 |
| 500,000 | 750 / 651 | 1396 / 1262 | 详见原始 JSON |

第二、三轮在 redo 水位接近容量时可观察到约 1 秒的短暂同步停顿，但总体吞吐仍显著高于 120 条/秒。**该实验不支持“100 MiB redo 单独限制吞吐到 90 条/秒”的结论**；也不能外推完整历史和大 CDC 批次的表现。

证据：`.cache/incident-20260911/logprobe-fresh.jsonl`、`logprobe-history.jsonl`。程序只写独占库，复制历史没有修改原事件记录。

### 原版持续双向峰值

Run：`.cache/incident-20260911/peak-baseline-085148/`。

- 使用原 0.46.13 Agent，新独占测试表，保留现场同步系统表及 MySQL 参数。
- 原 50 列、映射、同键重复 UPDATE、事务 oracle 和在线版本观察逻辑；固定每方向 80 INSERT/s + 40 UPDATE/s。
- 每方向 557 个生产秒，`44,560 INSERT / 22,280 UPDATE`。生产及核验 565.338 秒完成，测试进程 578.20 秒 PASS。
- 完成两次生命周期、两次所有权交接、ADD/DROP COLUMN、5 组延迟探针；全 50 列和精确 Apply/SUCCESS 账本通过。
- 5 组上行 203-405 ms，下行 413-648 ms。这是诊断样本，不是正式 P99 准入。
- 后半段附加旧失败 run 的大历史量 metrics/snapshot 查询。事件统计约 3 秒，但未复现持续积压。
- 双端恢复 Agent：中心 75472，Edge 13304；原规则 hash `CD26BB056B729B5D0A3614784467C849354E9A37181BF96ABE10FD6E8EFBE2CD`，运行队列全部零。PID 仅为本次核验记录。
- 临时 SQL 等待/阶段观测在结束后恢复原配置。

**限制**：新测试表没有原 run 的单表 60 万行业务历史；历史分布、CDC 批次大小和故障后的运行轨迹尚未完全等价。无两次 15 分钟停机，不作七小时通过结论。诊断固定速率未改变正式长测默认速率或门禁。

## 修复准入

1. 继续对齐 CDC 批次大小和历史索引分布，捕获具体 SQL 等待。报告更新在产品修改之前。
2. 已确认的观测缺陷可独立修正，但不能以“日志更完整”宣称性能问题解决。
3. 实际性能修复必须附相同条件的前后对照、提交/回滚/重入队与重放测试，再运行全量 Go test/vet/lint。
4. FB-094 保持 open；FB-095/096已按09:48修复与验证交付关闭，不代表七小时验收通过。Wails API、SyncEvent、DTO、UI 不变。
