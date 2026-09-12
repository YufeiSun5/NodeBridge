# 首次数据对齐与全功能 MCP 实施方案

> 2026-09-11 实施纠偏：用户明确MCP权限全开放，撤销下文会话scope申请/审批设计，测试隔离只约束本方测试数据。当前优先修业务报告的数据正确性问题；首次对齐执行尚未实现，下文其他内容仍是方案，不是已交付能力。

> 2026-09-11 | review-ai | 方案草案，待确认后实施。
> 基线：0.46.15 发布源码；当前没有首次对齐引擎。本文的新增包、字段、接口和工具均为拟议，不是已实现的稳定契约。
> EN: Rule-scoped initial alignment, continuous CDC, and MCP coverage for existing and new capabilities.
> JA: ルール範囲の初期整合、継続 CDC、既存・新機能の MCP 対応計画。

## 1. 结论与交付目标

首次对齐是**可选功能，默认关闭，必须手动触发**。只有用户明确选择时才走“先按规则对齐，再持续增量”；不选择则保持现有增量链路，不主动扫描或补齐存量。正常 CDC 仍按既有位点消费可见事件，关闭首次对齐不等于丢弃原有待消费事件。

保留 Go、Canal、RabbitMQ、MySQL、Wails2/React 和 stdio MCP，不引入必须常驻的新 HTTP 服务，也不默认替换 Canal。安装、升级、节点注册、保存配置/规则、Start/Restart Agent 均不得隐式创建首次对齐作业。

分两次功能交付：

1. **M1：维护窗口对齐 + 旧功能 MCP 补齐。** 支持边缘历史上传、指定配置下发、已有目标补缺/受控更新、差异预览、可恢复作业、核验；旧管理功能不再只能通过 UI 或实验室绕过鉴权调用。
2. **M2：在线对齐 + CDC 无缺口交接。** 在明确单一数据权威的范围内，支持持续采集、快照期间增量暂存、顺序追平和切换；完成跨节点故障与容量验收。

“所有功能支持 MCP”的含义：每项产品能力都有可发现、带权限、可验证结果的 MCP 路径；不是每个内部函数一个工具，也不是允许 AI 任意执行 SQL/Shell。安装前引导、Windows UAC、管理员批准、业务维护窗口仍有真实外部前置条件。

配套资料：[MCP 全量覆盖清单](mcp-capability-coverage-plan.md)、[可编辑架构图](assets/initial-alignment-architecture.drawio)。活跃问题与分工只进入根级 [AI_BOARD.md](../../AI_BOARD.md)，本文不另建活跃看板。

## 2. 现状与明确缺口

| 能力 | 现有基础 | 本次需要补充 |
| --- | --- | --- |
| 持续同步 | CDC -> SyncEvent -> RabbitMQ -> Apply；确认、幂等、回环、映射、重试、部分 DDL 已有 | 首次基线与增量之间的边界和逐目标就绪门禁 |
| 规则 | 表列映射、来源节点、分发节点、CRUD/append/compact、冲突策略 | 业务行范围、初始数据权威、计划版本固定；不能拿 source_node_ids 当行过滤 |
| MCP | 普通模式 15 工具，实验室模式共 33 工具，5 个资源 | 正式授权、旧 CLI/UI 缺口、作业 API、能力清单和全面兼容回归 |
| 数据治理 | 结构化查询和有限 DML、已有业务表 ADD/DROP COLUMN | 复用校验与无损编解码；不可用 AI 逐页查询/写入充当批量同步引擎 |
| 安装 | 0.46.15 默认复用，显式选择安装系统组件 | 安装/修复/升级/卸载的受控 MCP 作业入口；保留外部资源所有权 |
| 现场验证 | 本机已正式安装 0.46.15，连接检查通过 | 不等于新能力存在，也不等于 FB-094 长测问题已验收 |

核对入口：`internal/rules/rules.go`、`internal/cdc/offset.go`、`internal/datasyncui/app.go`、`internal/datasyncui/mcp.go`、`internal/mcpstdio/server.go`、`internal/dbgovernance/`、`cmd/sync-agent/main.go`。

## 3. 业务场景与安全默认值

### 3.1 可选与手动触发规则

- 接入向导提供“执行首次对齐”复选框，默认不勾选；也可日后从“数据对齐”页手动创建作业。勾选/保存只表达选择，不执行数据写入。
- 必须经过“选择范围 -> 预检/生成差异 -> 查看影响 -> 明确确认开始”。确认前不写目标业务数据，不自动批准任何计划。
- 规则中如需保存偏好，拟议 `initial_alignment.policy=DISABLED|MANUAL`，默认 DISABLED；不提供 AUTO。MANUAL 只允许提出执行请求，不是后台触发条件。字段最终在 P0 契约评审固定。
- 每个新作业的 start 都须具备用户对该计划的明确批准；AI/MCP 不得因发现缺数据、新节点或规则变化自行启动。普通运维 grant 不自动包含首次对齐执行权限。
- 一个已批准作业内部可自动分块/重试，客户端断开可继续原已批准范围；不得扩大范围、自动重规划后重跑或创建下一批作业。Agent/主机重启后默认恢复为 PAUSED，需显式 resume；保留 CDC 安全暂存和门禁，不静默恢复业务写入。
- 关闭该选项阻止新作业；已有作业须单独显示并明确暂停/取消，不因取消勾选直接丢弃暂存或回滚数据。失败/部分完成不可隐藏。

以下场景中的“首次对齐”列均仅在用户选择并确认后执行。

| 场景 | 数据权威与范围 | 首次对齐 | 后续增量 |
| --- | --- | --- | --- |
| 边缘先运行，中心后部署 | 各 Edge 自有历史 -> 中心指定目标表/分区 | 上传本节点规则内历史；中心已有相同记录跳过 | 原上行 CDC 链路继续 |
| 新增 Edge | 中心配置 -> 明确分配给该 Edge 的行 | 只下发授权配置，不下载全中心历史或其他 Edge 数据 | 相同范围的配置变更下发 |
| 目标已有部分数据 | 计划内指定源为权威，目标本地字段保留 | 补缺；差异行按批准策略更新；冲突单列 | 按既定 CRUD 与冲突策略运行 |
| 多 Edge 主键可能重复 | 必须先有可证明不冲突的目标键或独立目标表 | 冲突则阻塞，不把两个节点的 id=1 静默合并 | 同一身份规则用于 CDC |
| 双方同一行均可独立修改 | 尚无明确业务归属 | 先生成冲突报告，不自动覆盖 | 任意多主在线合并不属于 M1/M2 首批支持 |

默认配置：`insert_missing=true`、`update_existing=REQUIRE_AUTHORITY`、`extra_target_rows=KEEP`。初次对齐不自动删除目标多余行，不 DROP/TRUNCATE 表，不重建业务库。

区分两种删除：首次清理目标多余行默认禁止；已批准持续同步规则内的真实源 DELETE 仍应正确传播。业务行离开某 Edge 授权范围时是否撤回，也是单独策略，不能把“保留目标多余行”错误套到所有后续 DELETE。

配置表与业务历史表分别建规则/作业；不依据表名猜归属。`append_only` 初始只补缺，相同主键不同内容记冲突，不自动改成可覆盖模式。局部字段排除用于保护本地运行字段；缺行 INSERT 时仍须满足目标 NOT NULL/默认值约束。

## 4. 技术架构与代码落点

| 组件/路径（拟议） | 职责与复用边界 |
| --- | --- |
| `internal/management/` | 逐项提取无 UI 依赖的管理用例；MCP、Wails、CLI 调同一服务，不整体重写现有 App |
| `internal/authorization/` | OS 身份、能力范围、节点/表约束、计划批准、撤销、审计；适配已有管理鉴权 |
| `internal/job/` | 作业状态机、幂等请求、租约、暂停/取消、事件游标、结果分页、期限 |
| `internal/alignment/` | 预检、范围固化、差异计划、扫描、条件写入、核验与交接编排 |
| `internal/alignment/mysql/` | 复用 mysqlconn、mapper、无损值处理；主键游标扫描和目标事务写入 |
| `internal/cdc/` + `internal/cdc/canal/` | 新增可验证源位点/事务边界能力；M2 快照与增量水位，不把 Canal batch_id 当持久全局位点 |
| `internal/syncruntime/` + `internal/rabbitmq/` | 每目标范围门禁、持久暂存、控制消息、confirm/manual ACK、屏障、代际 fencing |
| `internal/syncstore/` + `migrations/` | 新增作业、分块、收据、冲突、路由代际记录；走下一编号迁移，不重写历史迁移 |
| `internal/datasyncui/` + `internal/uiapi/` | Wails 薄适配和 DTO；先评审契约后生成 bindings |
| `internal/mcpstdio/` + MCP 注册入口 | 工具定义、权限映射、输入/输出 schema、资源和兼容层 |
| `frontend/src/` | 对齐任务列表、差异、详情、批准操作，三语；不直接连接数据库/消息队列 |

`SyncEvent` 继续保持现有源库表列语义。对齐另用版本化 `AlignmentEnvelope` 包装行数据、`job_id/generation/chunk_id/row_id/rule_hash/source_position/payload_hash`；需保序的 CDC 附加信息放内部传输元数据，不无故破坏已有 SyncEvent JSON。

目标对齐写入器与现有 Apply 共用映射、回环标记、日志事务辅助逻辑，但有独立的“缺失插入/有条件更新”策略。不把现有所有 INSERT 全局改成 UPSERT，不使用会隐式删除再插入的 REPLACE。

### 4.1 跨节点执行与持久化

- 中心协调作业 DAG，各节点 Agent 只用自己保存的 MySQL/RabbitMQ 凭据读写本机数据库；MCP 客户端不转运业务行、不持有远端数据库密码。
- 沿已有 RabbitMQ 网络路径传递版本化控制请求、分块及回执；复用连接与确认实现，新增独立命名空间。首版每节点固定数量队列、按 job_id 区分，避免每个块/作业无限建队列。
- 中心保存协调日志；每端 MySQL 保存本地执行状态及收据。本地业务行、回环日志与应用收据必须同一事务，中心状态通过持久回执最终更新；消息出站采用持久 outbox，confirm 后更新状态；故障只能产生可去重的重发。
- ACK 必须在目标业务写入、sync_apply_log 与分块收据提交后；中心确认“所有目标完成”须持有各端持久回执，不能只看发布成功。
- 作业由 SyncAgent 承担。新增 management-only 运行方式服务初次安装与业务同步尚未就绪的节点；同一配置只能有一个作业租约持有者和一个 CDC owner。启动对齐不暗中启动业务生产流量。
- UI/MCP 退出不终止已批准作业；跨进程配置锁只包住短控制操作，不持有几小时。Agent 恢复先读持久状态并默认暂停，显式恢复后再竞争租约和递增 fencing_epoch；旧进程不得继续写入。普通恢复保持 generation 和幂等 ID 不变；只有重新规划/新快照才开启新 generation，避免重启制造新的业务事件身份。
- 安装器在 MySQL 尚不可用时，用已有原子文件机制保存本机 helper 作业 journal。数据库依赖型作业则返回明确前置条件失败，不伪造可用状态。
- 节点控制权限绑定 node_id、job_id、generation、规则 hash 和目标范围；RabbitMQ vhost 不是业务授权。控制/数据消息校验来源身份、完整性及重放范围，拒绝伪造目标和越权分块。

### 4.2 规则与行范围

拟增规则版本及结构化 `row_scope`，只允许白名单列、类型化常量、有限比较/集合操作；不得接收 SQL 片段。源 SELECT 与 CDC before/after 必须调用同一谓词实现，统一 NULL、大小写、排序规则和时间语义。

行范围转移按 before/after 是否命中处理：未命中->命中为进入；命中->命中为更新；命中->未命中为撤回候选；都未命中为忽略。撤回须显式授权，不能悄悄改成保留或删除。缺少充分 before image 时预检拒绝相应模式，不能仅依据 after image 猜测。

仅当用户选择首次对齐时，新节点对所选范围先保持目标就绪门禁关闭，避免现有 ACTIVE_EDGES 广播抢先下发。规划时解析并固定所有目标 node_id；后续新注册节点不自动进入已批准计划。未选择对齐的节点/规则走原有接入和增量流程，不因为缺少 alignment 作业而被强制拦截。

## 5. M1：维护窗口对齐算法

维护窗口必须覆盖**所选源和目标范围的业务写入及 DDL**。停止 SyncAgent 不等于停止业务写库。M1 可以从小配置表开始，不承诺任意大表仅需短暂停写。

1. **预检。** 校验节点能力、连接、MySQL InnoDB/主键、映射、目标唯一约束、触发器/外键、副作用、schema hash、磁盘预算、权限和当前路由。目标缺业务表时报告先决条件，不擅自建库表。
2. **估算与批准。** 先只读估算行数/字节/时间。创建维护计划，固化权威源、范围、字段、最大写入量和备份证明；业务方批准并停写，排尽已开启的相关事务，记录维护租约/证据。
3. **排尽旧增量。** 让 CDC 继续捕获至事务提交水位 F；逐目标确认 F 之前的旧消息已 Apply/ACK，重试与失败消息亦须处置。用屏障与收据证明，不用“队列深度为 0”替代。
4. **固定基线。** 冻结规则版本并开启目标门禁，读取源/目标结构；按主键分块扫描和比较投影后的业务列。维护租约失效、发现业务写入或 DDL 即停止，转 NEEDS_REPLAN。
5. **生成精确差异。** 输出 `missing/changed/equal/conflict/extra_target` 数量及受限样本，计划记录每块主键范围、源 hash、预期目标 hash。执行批准必须绑定本次差异 plan_hash，不能复用先前粗估算的确认。
6. **有界写入。** 每块双上限行数/字节，主键 keyset 分页，不使用大 OFFSET。短事务内核对目标当前值；缺行插入，差异行只在权威策略和 compare-and-set 条件成立时更新。目标变化为冲突，不盲写覆盖。
7. **可靠提交。** 同事务落业务数据、回环字段、sync_apply_log、作业行/块收据；确认后 ACK。幂等 ID 派生自作业代际、规则、块和行身份，重发不重复产生业务效果。
8. **精确核验。** 对同一范围重新分块比较主键及全部同步列；默认完整核验，不仅行数/抽样。保留目标额外行与冲突数量；未解决冲突不能标为成功。
9. **交接。** 原子写入此目标/规则的 ready generation；保留已提交水位和回环记录，再解除业务维护。后续真实 INSERT/UPDATE/DELETE 至少各验证一次，确认持续 CDC 接管，无缺失/重复效果/回环。

无损比较覆盖 BIGINT、DECIMAL、NULL/空串、二进制、时区与时间精度；复用现有 JSON number 修复，不经 float64 计算主键/金额。复合键使用类型化稳定编码，检查不同源键是否映射到同一目标唯一键。

数据量较大时先完整物化源块再应用，记录每块 checksum 和 manifest sealed 状态。若读取快照/维护条件中断，未封口的扫描不能跨新快照续接；重新规划。已提交的块不会因取消而自动回滚，恢复需重新核验。

## 6. M2：在线对齐与增量衔接

首批在线范围限制为：所选行有单一权威源，目标禁止同范围独立业务写入；跨表以依赖组控制 ready，不承诺整库分布式原子可见。任意双方同时改同一行、自动冲突合并、无短锁权限的场景另立里程碑。

### 6.1 先做能力验证，再编码全链路

Canal 适配验证必须证明：能取得可持久的 file/position 或 GTID/事务序号，保留事务边界，精确识别快照边界后的事件，重连/轮转后连续恢复；batch_id 只属于连接批次，不能用于此证明。现有 Offset 字段存在不代表这些能力已完整实现。

首选算法为“配对的一致快照 S0 与位点 B0 + 持久 CDC 缓冲 + 屏障切换”。建立 S0/B0 的短锁阶段需要 DBA 批准；不宣传严格零阻塞。仅先 BEGIN 再无锁读取 SHOW MASTER STATUS 不能证明二者配对。

MySQL 一致读快照与 DDL 有边界，部分 ALTER/DROP 会使读取失败，因此全程监测 schema hash 并禁止所选范围 DDL。[MySQL 8.0 一致读文档](https://dev.mysql.com/doc/refman/8.0/en/innodb-consistent-read.html)

成熟快照实现的参考顺序是在锁保护下建立可重复读事务、取得日志位点和结构，再释放短锁、扫描，之后从该位点接续日志。本项目借鉴这一边界，不引入 Kafka，也不把 Debezium 描述成当前已接入。[Debezium MySQL 快照流程](https://debezium.io/documentation/reference/stable/connectors/mysql.html)

能力验证不通过时：M1 仍可交付，M2 明确阻塞；评审修正 Canal 适配或新增 Debezium 适配的成本，不自动切换现场 CDC。

### 6.2 在线时序

1. 协调端先建立新 generation 的目标门禁和持久 CDC 暂存；目标上已经应用的旧事件必须排到可证明边界。源采集不得中断，未持久化消息不得提前 ACK。
2. 在批准的短锁窗口内建立 S0/B0 配对，记录 schema/规则 hash，验证 CDC 能覆盖 B0 后全量事务。已有暂存中 B0 以前的范围事件由基线覆盖，不再重复写入目标；丢弃决策也须持久可审计。
3. 从同一 S0 读取有界块到持久 staging；同时将 B0 之后的源事件连续暂存。所有扫描块封口后，才允许关闭快照事务并按块应用基线。
4. 基线应用过程中，此目标范围的新增量只能暂存，不能抢先写目标，否则旧快照可能覆盖较新更新或复活已删行。其他规则/目标继续原有链路。
5. 基线完成后记录事务边界 C；按源提交顺序重放 B0 之后直到 C 的事件，保持同键 CRUD、DELETE tombstone、规则/DDL 屏障；快照数据不得走 compact 丢失核验信息。
6. 在 C 暂停该范围目标应用，依据快照加完整 CDC 重放形成的预期状态核验目标。此时源可继续写，C 之后的增量继续缓冲；不能拿持续变化的源现值与 C 时目标直接比较。
7. 全块完成、无未解决冲突、C 收据齐全且核验通过后，原子 seal ready generation，再按序排出 C 后缓冲并衔接实时流。切换期间只能有一个范围 owner；晚到旧代块拒绝，重复 CDC 仍由现有事件幂等处理。

多来源同一目标范围首批不并行初始化；先按各自不重叠的业务归属拆开。无法拆开时，不用跨节点墙钟时间伪造全局顺序。

### 6.3 在线失败与容量边界

- 快照事务在全部源块封口前丢失：该 generation 转 NEEDS_REPLAN，不从主键游标假装恢复原快照。已封口持久快照可以续传/续 Apply。
- 源 binlog 在未持久捕获前过期、源发生不可兼容 DDL、规则变更、权限撤销：停止后续写入并报告明确原因；不跳到最新位点。
- 目标提交成功但 ACK/回执丢失：重投并由本地收据去重。中心掉线由节点保留状态，过期租约禁止新提交；不会自动解除目标门禁。
- 取消不回滚已提交数据。在线取消仍须安全处理被门禁阻挡的 CDC；进入 DRAIN_REQUIRED/NEEDS_REPLAN，由已授权的继续追平或重新初始化路径解除，不将半成品目标标为 ready。
- 快照、staging、CDC spool、RabbitMQ、日志、binlog/undo 都计入容量；暂停不等于停止增长。持续测量写入速率和剩余时间，达阈值暂停新扫描/请求业务干预，不自动丢弃数据或更改 binlog 保留。

## 7. 作业模型、接口与权限

### 7.1 持久模型（拟议）

| 表/记录 | 最少字段及约束 |
| --- | --- |
| `sync_alignment_job` | job_id、parent_job_id、request_id、state、generation、fencing_epoch、scope/rule/schema/plan hash、authority、mode、owner、grant_ref、B0/C、期限、计数；request_id 唯一并绑定参数 hash |
| `sync_alignment_chunk` | job_id/generation/chunk_id、类型化键界、行数/字节、payload_hash、阶段、cursor；复合唯一键，禁止覆盖已封口内容 |
| `sync_alignment_receipt` | job/generation/chunk/target、应用计数、目标 hash、commit 水位；同业务写入提交 |
| `sync_alignment_conflict` | 冲突键、原因、源/目标摘要、解决计划引用；业务值只对受权主体可见 |
| 路由/控制记录 | ready generation、fencing token、lease、outbox/inbox、已处理控制请求；复用现有存储组件，名称在迁移评审时固定 |

主状态：`PLANNING -> AWAITING_APPROVAL -> PREPARING -> SNAPSHOTTING -> APPLYING -> CATCHING_UP -> VERIFYING -> SUCCEEDED`。M1 无需追赶时直接进 VERIFYING；旁路状态 `PAUSED/FAILED/NEEDS_REPLAN/DRAIN_REQUIRED/CANCELLED_PARTIAL` 均保留已提交计数和恢复动作，不能折叠成成功。

控制接口在有界时间内返回 job_id，目标本机 2 秒内提交控制记录作为初始性能目标，超时返回可查询的 request_id，不持锁扫描全表。计划扫描和完整核验本身也是作业。百分比未知时返回已完成行/字节及估算依据，不显示虚假 100%。

### 7.2 API 草案，不立即写入稳定契约

| 用例 | Wails 拟议入口 | MCP 拟议入口 |
| --- | --- | --- |
| 能力与条件 | GetCapabilities / PreflightAlignment | nodebridge_capabilities / nodebridge_alignment_preflight |
| 生成计划/差异 | PlanAlignment / GetAlignmentDiff | nodebridge_alignment_plan / nodebridge_alignment_diff |
| 批准后开始 | StartAlignment | nodebridge_alignment_start |
| 作业控制/观察 | ListJobs / GetJob / GetJobEvents / PauseJob / ResumeJob / CancelJob | nodebridge_jobs_list / nodebridge_job_get / nodebridge_job_events / nodebridge_job_pause / nodebridge_job_resume / nodebridge_job_cancel |
| 核验/解决冲突/重规划 | VerifyAlignment / PlanAlignmentConflictResolution / ApplyAlignmentConflictResolution / ReplanAlignment | nodebridge_alignment_verify / nodebridge_alignment_conflict_plan / nodebridge_alignment_conflict_apply / nodebridge_alignment_replan |

`plan` 请求至少包含规则 ID/版本、源/目标 node_id、QUIESCED/ONLINE、权威策略、目标额外行策略、行/字节预算；`start` 只接受已批准 plan_id/plan_hash、grant_ref 和 request_id，不允许换目标后沿用确认。

所有新控制请求带幂等 request_id；重复同参数返回同结果，不同参数复用返回 REQUEST_CONFLICT。作业类新结果统一包含 `status/job_id/request_id/generation/next_cursor/warnings/error`，业务错误类型如 AUTH_REQUIRED、UNSUPPORTED_CAPABILITY、PLAN_STALE、SCHEMA_MISMATCH、KEY_COLLISION、SOURCE_CHANGED、CAPACITY_LIMIT、CDC_GAP。旧工具保持原结果结构，不能强套新 envelope 破坏客户端。

### 7.3 正式 MCP 授权与兼容

- 先接 OS 进程身份与受控主体，再发本机保护的短时 scope grant；分开 read、config.write、rules.write、agent.control、alignment.plan/run、data.read/write、schema.write、installer.run、desktop.control。授予者只能授予自己的权限。
- 普通模式开放获授权的旧工具，不再依赖 `-lab-full-access` 才能运维。实验室兼容入口保留，但明确警示/审计，不作为生产接入教程。
- `confirm=true` 不是鉴权；高风险操作须明确人类批准或此前授权的受限自动化策略，绑定计划 hash、目标、写入预算和期限。密码不写入提示词示例、返回值、日志或批准摘要。
- MCP 启用默认仍为 false。首次启动需本地 UI/受控引导；已连接会话不能靠自授权扩大范围。关闭 MCP 后拒绝后续调用；已批准后台作业按独立作业 grant 继续或按显式撤销在事务边界停下，不能含糊处理。
- 返回 schema、工具 annotations、分页和 typed errors；业务失败用 isError，协议失败用 JSON-RPC 错误。新协议响应可同时提供 structuredContent 与兼容 JSON 文本，按协商版本测试，旧解析路径保留。[MCP Tools 规范](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)
- 保留现有 33 名称、旧必填参数与 5 个资源 URI；新增工具按实际权限发现，初始化只声明真实实现能力。MCP 的版本协商不等于节点支持在线对齐，另做产品能力协商。[MCP Lifecycle 规范](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)
- 名称/参数/结果兼容不等于永久保留旧授权方式。正式权限收紧需迁移说明和升级前检查；旧客户端缺少 grant 时明确 AUTH_REQUIRED，不能静默转实验室模式或假装旧自动化仍可无条件写入。
- 长作业采用 NodeBridge 自有 job tools；不将其宣传为已实现 MCP 原生 Tasks。notifications/prompts 可后续增强，但查询工具必须独立可用。
- 本地 stdio 和 SSH stdio 都验收；绑定远端 OS 用户的数据/DPAPI/自启动上下文。诊断包返回远端 artifact_id/受权读取接口，不假装远端路径是客户端本地文件。
- 修正结构化脱敏，包含 FB-089 的时间元数据；业务样本/差异/错误也按表/列权限脱敏。死信预览目前会取出再重入队，必须准确标注副作用，不能当作绝对只读查询。

## 8. UI、安装与接入流程

新增“数据对齐”工作页：任务列表、差异表、详情日志/水位、冲突、批准/暂停/恢复/取消。沿用现有紧凑终端样式和 13px 上限；不用营销首页。状态区分估算/精确结果、等待远端/已失败、部分提交/完整成功。中文默认，提供英日切换；所有动作调用同一 Wails 服务和权限检查。

新 Edge 接入在结构/键校验后分支：**不勾选首次对齐（默认）**，直接按原规则进入增量；**勾选首次对齐**，选择配置下发/历史上传的具体范围，预览差异并确认，完成核验后对所选范围开放增量。两条路径 UI 与 MCP 均完整支持，不把首次对齐做成安装/接入强制步骤。

安装/升级/修复/卸载采用 plan/apply 和外部 helper。只操作 manifest 标识的自有资源，默认复用 Docker/外部实例，不安装或改造客户业务 MySQL；受控迁移仅 NodeBridge 系统表。业务 schema 由业务部署流程提供，缺失时报出依赖，而不是执行任意建表 SQL。

安装前尚无 MCP 可执行文件属于引导问题：发布包提供受校验的 portable bootstrap/helper 与客户端配置生成方式，由用户先启动并完成信任/UAC；之后 MCP 可提交批准的安装作业并在主进程升级/退出后查询 helper journal。不提供绕过 UAC、任意下载 URL/命令行或卸载外部组件的工具。

语言/主题、窗口显隐/显式退出同样纳入 MCP 能力清单；它们绑定当前交互用户并通过本机 IPC 桥接真实 UI。SSH Session 0 无可用桌面时返回 unsupported/interaction_required，不伪造成功；退出仍需退出授权。MCP 不要求把桌面端改成 HTTP 服务。

## 9. 实施阶段与工作量

以下为当前范围的工程人日估算，含实现、评审和针对性验证，不是 AI 连续运行时长承诺。完整实测性能、维护审批和远端环境等待另列；P0 后据数据量/权限实测修订。此前 3-5 天仅适用于窄范围停写补齐，不包含此次全功能 MCP、权限体系、跨节点恢复和在线交接。

| 阶段 | 主 owner | 工作与完成条件 | 估算人日 |
| --- | --- | --- | --- |
| P0 设计冻结/能力验证 | review-ai + backend-ai | 确定权威/行范围/维护条件；33 工具及全部产品入口清单；S0/B0/Canal 边界验证；合同草案评审 | 2-3 |
| P1 管理服务与 MCP 基础 | backend-ai | 共享服务、正式 scope grant、旧 33 工具兼容、作业框架/幂等/审计、FB-089 回归 | 4-6 |
| P2 QUIESCED 对齐 | backend-ai | 规则范围、跨节点控制、扫描/差异/条件写/核验/恢复、全部对应 MCP 工具，真实异名映射与幂等验收 | 7-10 |
| P3 旧入口补齐与界面 | backend-ai + frontend-ai | 下表 MCP 缺口含节点配置/安装/桌面适配，Wails 三语对齐页；无 UI 也能完成接入，M1 候选 | 4-6 |
| P4 ONLINE 对齐 | backend-ai | 快照边界、持久缓冲、事务屏障、故障恢复、ready generation，M2 功能候选 | 8-12 |
| P5 发布门禁 | test-ai + review-ai | 双边缘/中心 E2E、故障矩阵、100GB 预算、旧功能 MCP 回归、升级/正式包验证和证据审阅 | 6-9 |

M1 开发阶段 P0-P3 合计约 **17-25 人日**，发布前仍需对应 P5 验收；完整 M1+M2+门禁约 **31-46 人日**。P3 UI 可在 DTO 冻结后与 P2 后段并行，不能通过并行跳过跨端真实测试。若需任意多主、零短锁在线方案、自动业务 schema 创建或复杂转换，单独重新估算，不挤进上述范围。

每阶段只允许通过门禁的能力出现在 capabilities 支持列表。M1 出包可以明确 online=false；不得将分阶段交付说成“新旧功能已全部完成”。不在本计划中承诺未经批准的版本号或发包日期。

## 10. 验收矩阵与发布门禁

| 编号 | 必须验证的行为 | 合格证据 |
| --- | --- | --- |
| T01 兼容 | 普通 15/实验室 33 旧工具基线、5 资源、4 个现有协议版本；新权限发现；旧参数/错误与非秘密 patch 限制 | 原请求重放及 schema golden；不以工具数量替代业务成功 |
| T02 业务范围 | 中心 + 两 Edge；边缘历史上传、新 Edge 只得所分配配置、旧目标补缺/保留本地字段/额外行 | 独立预期数据账本，未授权表/节点/字段零变化 |
| T03 映射/值 | 异库表列名、复合键、跨 Edge 同键、BIGINT/DECIMAL/NULL/BLOB/时间精度、50 列 | 全同步列逐值核对；冲突不覆写；数值无精度损失 |
| T04 过滤/删除 | before/after 进出范围、节点分配变更、真实 DELETE、目标额外行保留 | 每种转移的源事件、目标行与 apply 收据精确对应 |
| T05 恢复 | 分块发送/confirm/目标 COMMIT/ACK/检查点前后各点中断；客户端关闭、Agent/中心/边缘重启 | 无重复业务效果、无遗漏；重复 start 同 job；部分提交如实报告 |
| T06 在线交接 | 扫描时持续 INSERT/UPDATE/DELETE，同键多次变更、旧块晚到、快照丢失、日志轮转/缺口 | C 时点精确账本和随后增量；无版本回退/删除复活/跳位点 |
| T07 授权/审计 | 未授权、过期/撤销 grant、跨节点越权、伪造块、旧代重放、密码与时间字段脱敏 | 明确拒绝、零越权写入；审计可关联计划/主体/作业，不泄露秘密 |
| T08 资源/压力 | 每节点 100GB 总工作预算分项，快照/暂存/binlog/undo峰值，网络慢于写入，暂停增长 | 真实路径/预算绑定；超限安全停止，不删消息，不放宽原可靠性 |
| T09 全入口 | 每项覆盖清单至少一个成功与拒绝用例；正式 stdio/SSH，MCP-only 完整接入和恢复 | 通过标准 MCP 客户端验证，不由直接 SQL/Shell 代替被测入口 |
| T10 安装/回归 | 默认复用/显式组件、升级保配置、helper/UAC/退出恢复、外部 Docker不变；原有同步/DDL/回环/失败重试 | 同一最终包 hash 的功能与隔离安装证据，真实桌面验收单列 |
| T11 可选/手动 | 默认未勾选；安装/升级/注册/保存规则/启停 Agent 不创建对齐；勾选但未确认不写目标；MCP 缺批准不得 start；重启后暂停 | 零隐式作业/存量扫描/对齐写入；原增量仍运行；显式开始/恢复只执行已批准范围 |

容量策略沿用 [100GB 方案](../../docs/100gb-storage-test-plan.md)：暂按每节点总工作存储 100,000,000,000 字节，已有业务数据计入，70GB 告警/80GB 停止新扫描或造数并安全处置积压，另保留物理盘底线。新暂存预算必须从同一总额分配，不能额外赠送 100GB。

代码交付执行 `go test ./...`、`go vet ./...`、已安装的 `golangci-lint run ./...`；前端执行构建、binding/契约检查和桌面视口测试。并发核心需要 race 检查，环境无 CGO 编译器时记录未通过条件并在合适 CI 补齐，不写成通过。

既有 FB-094 的 125 秒延迟失败、FB-084 实际 UI 恢复、FB-072 队列口径、FB-073 空闲位点等独立保留；本方案不能关闭旧缺陷。长测仅在用户授权、容量与短测通过后运行，不自动启动。

## 11. 待确认决策与本轮边界

用户已确认产品要求：首次对齐是选项，不能全自动；默认关闭、手动预览确认的约束已纳入 T11。以下均未得到本轮实施授权：

- <!-- 待确认 --> 是否允许首批范围的业务停写；单次维护窗口最长多久，谁能提供/解除维护证明？建议先选择小配置表与一张代表性历史表试点。
- <!-- 待确认 --> 各表/行的数据权威、节点归属字段、目标键策略；是否存在双方都能修改的同一行？没有答案时只能差异预览。
- <!-- 待确认 --> 首批表清单、数据量、外键/触发器、已有业务 schema 交付方；在线短锁权限与 binlog 保留预算。
- <!-- 待确认 --> 超出默认 KEEP 的删除/范围撤回策略、自动化授权有效期和可接受写入预算。
- <!-- 待确认 --> 人员并行度、M1/M2 发包安排；不能把“31-46 人日”当成现场停机时长。

本轮只产出计划、覆盖清单与可编辑结构图，并登记后续分工；不修改运行代码/稳定 DTO/现场配置，不安装、启动同步或运行长测。架构图经 XML 结构检查；当前未找到 draw.io CLI，未作 PNG 导出与视觉验收。

本轮验证：覆盖清单与源码注册的 33 个旧工具、38 个 CLI 命令核对无遗漏；相对文档链接均存在，draw.io 结构检查 0 errors/0 warnings，MEMORY 未超 100 行。现有源码 `go test ./...`、`go vet ./...`、`golangci-lint run ./...`（0 issues）通过；仅本轮文档范围 diff 检查通过。这些是现有基线回归和文档检查，不是新增对齐/MCP能力已实现的证明。
