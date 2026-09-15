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

- 规则name仅为显示元数据；对齐计划/证书匹配及运行语义版本必须忽略name，保留id和所有业务映射约束。CAS仍校验完整配置文件，不能借显示改名替换规则ID或删除对齐账本。
- 双向下行使用已编译反向/中继规则的显式目标业务库；不得再由本机 MySQL 系统库覆盖。旧 EDGE_TO_SERVER 下发中继仍保留 localDefault 语义。Canal 订阅过滤按逗号拆分编译，追加系统捕获/回放表须作为独立表达式，不对整个逗号列表加括号。
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

- 0.48.1双向LWW使用NodeBridge系统表事务标记，不要求或覆盖业务列last_event_id/updated_by_node。Apply及repair在同一事务以sync_replay_marker BEGIN/END包围业务写入；CDC将边界按MySQL UUID/binlog文件/位置持久写入sync_replay_position，按目标库表识别回放。正常业务写入即使行值相同或包含同名字段，也不能按值去重。
- 新协议捕获屏障同样包围在标记中，必须观察到完整BEGIN/pulse/END后才放行写入，防止Canal过滤掉系统标记却静默运行。跨批次和重启依靠持久边界，不靠系统时间。两端必须升级并执行各自010系统迁移；不得删除仍可能被CDC重读的边界/标记。以下业务行标记条款仅描述旧协议及未切换的旧单向路径，不是新双向业务表要求。

- 本节点的冲突修复只凭 `sync_repair_replay` 的事件/节点/库/表/实际操作证明过滤；不得简单移除 `updated_by_node != local` 限制而误过滤业务自己的标记更新。UPDATE必须识别新修复标记，保留旧标记的本地UPDATE继续上传；DELETE不能借用旧INSERT/UPDATE修复证明。
- 修复任务及获胜镜像与版本同事务持久保存。捕获线程不得等待业务行锁做恢复；修复worker必须先锁业务行/间隙，再等新鲜CDC屏障，之后锁版本并重新读取当前winner。修复/来源证明/日志/清任务同事务，不用修复时间创造新的业务版本。

- 新双向业务表不必包含同步专用字段；SOFT删除仍要求用户选择的软删除语义字段，普通无软删除字段表使用HARD策略，不自动加列。
- 每个节点必须维护 `sync_apply_log`。
- Apply Worker 写业务表和写 `sync_apply_log` 必须在同一事务。
- INSERT 的远端 `last_event_id` 命中 `sync_apply_log` 时可判定回放；UPDATE 还必须检查 FULL 前后镜像中的同步标记变化。标记保持不变的后续本地 UPDATE 必须上传，不得仅因旧收据存在而过滤；缺少必要前镜像时返回错误，不猜测来源。
- HARD DELETE 的回放必须有删除事务来源证据，不能把删除前遗留的 INSERT/UPDATE 标记当作删除回放证据。双向映射的 TrackDeleteReplay 路径在同事务更新专用标记、删除、写 sync_delete_replay 和 sync_apply_log；专用收据按事件/来源/库/表查询，普通旧收据不算删除证明。该路径要求 FULL 元数据及无触发器且可核验；普通单向删除不增加标记 UPDATE。完整双向运行验收前仍保持能力门禁。
- Server 分发时不得把事件发回 `origin_node_id`。

## 首次对齐约束

- 0.48.2启动保护以启用规则实际读写的本地物理表为范围，使用MySQL大小写模式；无关禁用表的失败job/PENDING组不能全局阻断单向链路。同表未完成作业仍阻断，无论规则ID或方向如何变化。ACTIVE复制释放scope_hash唯一占位后，按持久plan的本端物理表归属加载证明，不能漏掉已完成复制证明。所有旧账本保持原样。
- 启用本机双向规则时保留其中央表对应的完整拓扑观察，包括其他边缘的异名映射；不能按本机rule_id裁掉同组远端成员。完整性与本地持久ACTIVE证明仍须验证，不从manifest单独授予运行权限。
- Agent只有启动门禁与worker构造完成后发布进程ready；管理端等待匹配PID的ready及短暂稳定，不能把cmd.Start成功当运行成功。ready不是MySQL/Canal/RabbitMQ持续健康或业务同步成功的保证。

- 手动Edge1-Server-EdgeN首次对齐通过CLI/Wails/MCP接通；不能把复制收据当作CDC交接完成。非CANCELLED作业须核验本机复制ACTIVE与完整组ACTIVE证明才能启动Agent；PENDING/READY组继续阻断。禁止删账本或直接改phase解除阻断。全空仍建立边界；单一非空源经中心依次复制；多个原有非空源拒绝合并。原始256MiB/编码512MiB/每复制15分钟，已短测200MiB；不支持在线全量、超大单行分片或已有冲突历史向新成员转移。
- 中心使用全部明确成员及其来源范围匹配唯一规则，所有成员保留完整拓扑证明。中心来源事件命名空间采用全组最早边界；后复制接收端也必须重放该边界后的版本与墓碑，不能仅凭快照行值丢弃历史。每条SELECTED_EDGES规则须覆盖全部组成员，不得静默扩大配置。
- 捕获探针不ACK，因此Canal未ACK缓存须容纳快照及既有积压。受管配置默认缓存至少512MiB、Java堆至少2GiB，保留更大值；外部/复用组件由操作员核验并重启，不能只提高Go传输上限便宣称支持大快照。
- 源端持表行/间隙锁到目标提交收据返回，最终提交请求前持久保存SOURCE_READY行数与摘要。目标复制行、原事务捕获标记和收据同事务；RabbitMQ数据消息只能在目标提交后ACK。
- 探针读取Canal不ACK，关闭后原业务输入必须可重读。恢复只观察已提交的原标记，不能拿新脉冲位置替代；普通过期计划禁止新复制，已提交目标的核对恢复可在过期后进行。
- 正式启用前完成旧事件过滤、来源流身份验证、双方就绪/启用握手，并在各端持有现有Agent进程锁。不能清空已有版本/墓碑，不能扩大节点范围或假设远端库表同名。未提交重试先锁定目标收据并持久撤销，再核对撤销源；CANCELLED保留原计划/标记/摘要，后续明确执行才重新规划。
- context取消并不证明MySQL autocommit未执行，也不证明上游Canal socket已中止；提交结果未知须按持久结果核对，不可盲目重复写入。

## RabbitMQ 约束

- 0.48.7运行期Session在连接/通道关闭、发布失败或确认超时后销毁旧连接及Publisher，下一worker重试重建。消费批次固定同一通道实例，ACK/NACK只能作用于原delivery通道；不能用新通道确认旧tag。确认未知仍返回失败并保留上游重投/Canal位点，不在底层把未知发布当成功。进程running/重连成功不等于业务已追平，诊断保留最后实际成功时间及连续错误。

- exchange、queue、message 都必须持久化。
- 使用 publisher confirm 确认 broker 接收发布。
- 使用 manual ack 确认消费者处理成功。
- 失败消息进入重试或死信，禁止无记录丢弃。

## MVP 范围

- 1 个 Server，2 个 Edge。
- `alarm_history`: 单向 Edge -> Server。
- `device_config`: 多向 Edge <-> Server <-> Edge。
- 第一版优先实现 `SERVER_WIN` 和 `LAST_WRITE_WIN` 冲突策略。
