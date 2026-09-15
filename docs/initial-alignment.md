# 首次全量与双向同步

0.48.1修正：双向同步不要求业务表新增`last_event_id`或`updated_by_node`，同名业务字段也不被同步软件覆盖。两端升级后执行各自NodeBridge系统库的010迁移，新增系统回放标记及binlog边界表。普通表选择HARD删除；SOFT删除仍需业务自身的软删除字段。首次对齐/停止/重试方法不变。

Canal必须能捕获系统库`sync_capture_fence`与`sync_replay_marker`；Agent会补入客户端订阅并以完整事务探针校验，服务端过滤不可排除它们。标记和位置账本在CDC仍可重读期间不得清空。旧版本已运行链路升级前须处理旧协议积压，不将新表短测当作历史积压升级验证。

适用版本：0.48.0。支持同一逻辑表的 `Edge1 <-> Server <-> EdgeN`，由中心协调全部明确成员。已通过三套独立 MySQL/Canal/Agent 的短功能测试，以及中心源、边缘源各 200 MiB 首次全量；不代表长测或任意规模容量承诺。

## 执行前

1. 全部参与节点升级到同一版本，备份配置与数据库，停止全部参与节点的 SyncAgent；操作不会自动停止业务 Agent。
2. 每个节点配置自己的 MySQL、Canal 和本地 RabbitMQ。各节点 `rabbitmq.server_url` 指向中心 broker/vhost，`local_url` 指向本节点队列；无需提供对端 MySQL 密码。
3. 中心与对应 Edge 保存相同规则 ID 和映射定义。源表/源列表示 Edge，目标表/目标列表示 Server；不要在 Server 配置中交换名称。异名边缘使用独立规则及明确 `source_node_ids`；同结构边缘可共用一条规则，来源范围包含全部成员。中心必须能为每个参与 Edge 唯一匹配一条规则。
4. 双向草稿选择 `BIDIRECTIONAL`、`LAST_WRITE_WIN`、`crud_ordered`、`initial_alignment.policy=MANUAL`，首次执行前保持禁用。分发选择 `AUTO`，或每条规则的 `SELECTED_EDGES` 都包含全部组成员；不得选择 `NONE` 或仅允许部分成员。
5. 实际表须为带完整主键的 InnoDB 表，表列映射可逆且类型、排序规则和可空性兼容。双向需要可写的 `updated_by_node` 与 `last_event_id`；软删还需通过实际软删列检查。触发器、外键、生成列及不支持的主键类型会被明确拒绝。保持所有源时钟可靠。
6. 大快照必须为 Canal 配足缓存和堆。受管安装默认至少 `canal.instance.memory.buffer.size=16384`、`canal.instance.memory.buffer.memunit=32768`（512 MiB 缓存）和 `-Xmx2g`，保留已有更大值。外部 Canal 由操作员配置并重启；默认 16 MiB 缓存或 512 MiB 堆不足以完成本次 200 MiB 场景。已有积压、行编码和并发事务可能需要更多容量，不能仅按表文件大小估算。缓存参数依据 [Canal 官方文档](https://github.com/alibaba/canal/wiki/AdminGuide)。

容量默认值通过受管组件安装/配置修复路径写入；“仅复用组件”模式不会改写组件配置。已运行的 Canal 不会因文件变化自动获得新堆/缓存，升级时须在维护窗口核对配置并显式重启 Canal，再执行首次全量。不要将外部或复用组件的旧参数当作已经升级。

## 界面操作

在各节点现有“规则”页选择已保存的 MANUAL 规则。中心输入或勾选本逻辑表的全部参与 Edge ID（逗号分隔）；各 Edge 只输入中心 ID。每个节点确认后执行，等待完整组完成。中心选择该目标表任一成员规则即可，其他成员按保存的来源范围匹配。

- 中心有数据、所有 Edge 为空：中心依次复制到各 Edge。
- 中心为空、仅一个 Edge 有数据：先复制该 Edge 到中心，再从中心复制其他 Edge，不依赖输入顺序。
- 全部为空：不复制业务行，但仍建立完整增量接续边界。
- 多端原有数据：拒绝合并或覆盖；已有已提交对齐的恢复按原证明核验，不当作新复制。
- 每次复制原始值总量上限 256 MiB、编码传输上限 512 MiB、15 分钟；完整组协调上限一小时，协议最多 256 个 Edge，实际仅三节点短测验收。单行数据帧还须不超过 1 MiB（含 JSON/base64），不支持超大单行分片。
- 新计划须在创建后五分钟内开始传输，计划过期须按持久结果核验重试；十五分钟是已开始复制的上限，不保证任意积压都能在期限内处理。
- 复制持有事务行/间隙锁，可能阻塞该表业务写入。建议维护期间暂停业务写入；两个顺序快照之间的合法新写入将在增量启动后按源版本收敛。初始快照行值一致不等于已处理全部后续写入。

全部节点显示完成后，规则已保存为启用；由操作员明确启动全部 Agent。启停规则仍需保存并重启生效，不热重载。组证据必须完整 ACTIVE；一个复制对完成或目标收到数据都不等于组完成。已对齐映射不可静默修改，不能手写清单或删除账本绕过证明。

本版不是在线全量、任意大表续传或在线扩容功能。已有增量冲突版本/删除墓碑不能直接复制给新成员，系统会拒绝这种首次复制，不能靠清空版本表绕过。已完成组重试必须包含原有全部成员。

## 失败与重试

“中断等待”只停止当前调用，不回滚已提交的业务数据。窗口重开后，未保存终态的旧操作显示“上次结果待核对”，不是成功。

全部参与节点停止 Agent 后，使用相同规则和完整成员集合再次确认并重试：

- 目标已提交：核验原收据、摘要及捕获边界，不再次复制；保留复制后新写入。
- 目标未提交：先锁定目标收据并撤销旧尝试，再核对撤销源。返回 `alignment_uncommitted_attempt_cancelled` 后，双方再次执行以生成新计划。
- 配置、映射、实际 schema、MySQL 流身份改变或结果无法核验：明确报错并继续阻断启动，不能删除系统账本解除保护。

原计划、标记、摘要以及已存在的行版本、删除墓碑均保留。旧队列消息在持久留档后才 ACK；诊断中显示 `superseded`，不冒充业务增量应用成功。

## CLI

在各自节点执行，替换规则 ID、对端 ID 和本机路径：

```powershell
.\SyncAgent.exe initial-alignment -config .\config.yaml -rules .\sync-rules.yaml -rule device-pair -peer peer-node-id -confirm
```

中心的 `-peer` 可写为 `"edge-001,edge-002"`；各 Edge 的 `-peer` 为中心 ID。不要并行发起多个独立的中心对齐进程，由一个中心调用协调全组。

## MCP

普通 SSH + stdio MCP 新增三个工具，不需要 `-lab-full-access`，沿用现有管理鉴权与明确确认；普通目录共 42 个工具、5 个资源。

```json
{"name":"nodebridge_start_initial_alignment","arguments":{"rule_id":"device-pair","peer_node_id":"edge-001,edge-002","confirm":true}}
```

上例发给中心；每个 Edge 也需调用同一工具，使用与中心保存一致的本地成员规则 ID，并将 `peer_node_id` 设为中心 ID。`running=true` 只表示已受理；保持各 stdio 进程存活，轮询 `nodebridge_initial_alignment_status`，直到每个节点 `running=false,stage=completed`。

`nodebridge_interrupt_initial_alignment` 无参数，只中断当前 MCP 会话拥有的操作，不撤销提交。不在当前会话的未知操作返回 `alignment_operation_not_owned_by_this_session`，不会谎报已中断。重试复用 `nodebridge_start_initial_alignment`。断线、进程退出或 unknown 不能视作成功，应在各端重新启动同一完整组操作核验持久结果。没有新增后台控制服务或 HTTP 传输。

## 时间与删除

`LAST_WRITE_WIN`采用源 binlog 事件时间，不是接收时间、处理时间或数据库 Apply 时间。Canal 时间精度为秒，同秒按固定来源/事件顺序裁决。删除版本与获胜镜像持久保存，防止旧事件复活数据。

源时钟回退时，后发指令可能因时间更早而成为 loser；不要把时钟回退解释为“后到必胜”。严格业务指令时间字段及删除指令时间源尚未配置，不能宣称支持任意业务时间字段。

English: Manual Edge1-Server-EdgeN alignment into empty targets, with every Agent stopped. All participants explicitly join through Wails, CLI or MCP. Three-node and 200 MiB functional tests passed; adequate Canal capacity is required. No online membership changes or conflict-history transfer to new members. LWW requires reliable source clocks.

日本語：全参加 Agent を停止し、Edge1-Server-EdgeN の空テーブルへ手動初期整合を実行します。Wails・CLI・MCP で全参加ノードを明示します。3 ノードと 200 MiB の短期機能検証済みで、十分な Canal メモリー設定が必要です。オンライン拡張や新規メンバーへの競合履歴転送は対象外です。
