# 首次全量与双向同步：内部核心进度

**停止交接：2026-09-12 01:42，达到用户指定累计5小时上限。本轮没有新安装包，以下为源码进度而非功能交付；不再继续开发，等待用户新指令。**

2026-09-12 01:33，backend-ai。此记录不是功能发布、安装包或现场验收；按时间倒序，后条记录中的旧限制以最新进展为准。

## 用户确定的业务规则

- 手动首次全量：有数据的一端复制到空表一端，两端都空不复制，两端都有数据拒绝自动合并/覆盖。按规则实际映射的表判断。
- 同表双向：源端指令/事件时间较晚者优先，不用接收、落库、重试或处理时间。
- 严格业务指令时间与 MySQL binlog 执行事件时间不是同一概念。<!-- 待确认 -->业务指令时间字段及删除时间如何产生，或是否接受源 MySQL 事件时间，尚待用户确认。

## 已实现及验证

- 01:39：补取消后迟到Canal结果不能成为就绪标记；源SOURCE_READY/CONFIRMED带捕获标记却缺边界时拒绝，损坏phase/role/摘要/标记/库表及UUID变化单测通过。最终全量test/vet/lint、核心六包shuffle两轮通过；中心/边缘007迁移一致。本轮容器/候选Agent/4972测试端口已退出，未操作既有业务资源。全文diff检查仅Wails生成models.ts既有尾空白未过，排除该生成文件后通过，本次新增文件无尾空白。

- 01:33：最终恢复入口区分计划有效期：过期禁止新复制，只有已提交目标作业可以启动恢复探针；伪造摘要/phase/role/标记/库表、源就绪缺边界、MySQL UUID变化均拒绝。最新真实双Canal/RabbitMQ复制及提交后故障重连恢复7.64秒通过，`.cache/canal-business/df37b685afe24bb7b7e6c3d988b5f684`；真实双Agent完整短测51.36秒通过，`.cache/canal-business/3cc9ef70f1f44eba958feddad402df3f`。
- SQL核心增加写锁覆盖：源UPDATE/DELETE/间隙INSERT与目标空表INSERT阻塞，目标未提交数据不可见，目标提交后直到收据返回源锁仍保持；完整核心6.61秒通过。测试首次使用autocommit且把context超时误当SQL未执行，导致等待写入在锁释放后生效；已改为显式不提交的探针事务并重跑，不据此误报产品部分提交。
- 三语事件状态/规则预检/隔离/CAS的1366/390截图与溢出检查通过，已检查英文桌面、日文窄屏结果，`output/playwright/business-remediation/`。`superseded`与`recorded_outcome_unknown`没有显示成业务写入成功；前端build/binding通过。全量go test/vet/lint通过；本机CGO=0且无GCC，未做-race检测。

- 01:20：受限RabbitMQ快照传输使用双端各自本机DB与独立channel；消息持久、publisher confirm、逐帧进度回复，目标业务/收据提交前不ACK数据，失败关闭channel保留输入。不清队列，64MiB限制/两分钟context期限是内部原型约束，并非大表产品能力或网络硬超时保证。两台物理独立MySQL的正反向/精度/收据失败/中断短测4.46秒通过，`.cache/alignment-transport/afdef4425008412f8f627aff967f6c09`。
- 两端Canal探针启动后先证明活跃，所有探针读取不ACK，关闭回滚订阅位置。源表锁持有期间写入并观察源标记；目标标记与复制行/收据原子提交，提交后再观察并持久化边界。SOURCE_READY在最终提交请求前持久保存源行数/摘要，收据丢失后只能核对目标持久结果，不能盲目复制。目标提交后边界写入失败，RecoverTargetCapture重连观察原事务标记，不产生替代边界；ReconcileSnapshotReceipt校验身份/摘要/捕获完整性，可幂等确认但不解除Agent阻断。包含UUID变化拒绝，binlog重置/更换订阅等完整血缘验证仍属后续交接。
- 真实双Canal/RabbitMQ复制及目标提交后1644故障、重连恢复、两端旧/新业务行仍可读回且正确分列边界10.03秒通过，`.cache/canal-business/d53e80054a2c4413b7287d9c2122dd2f`。这是同一独占MySQL上的两个逻辑端点与独立Canal；不冒充该组合的跨物理MySQL验收。SQL核心5.43秒覆盖源摘要、收据丢失核对/摘要篡改拒绝/幂等恢复；全量test/vet/lint通过。新夹具第二轮首次误用第一轮相同后写标签而失败，已区分两轮标签并重新通过，不是产品故障修复。
- 双Agent真实双向增强6轮交替来源离线删除51.34秒通过，核验两端实际业务行、获胜来源/墓碑与repair_pending清空，且实际Agent因未完成对齐作业拒绝启动。证据`.cache/canal-business/fa28cfb4b29b4acaa4c7b60402ba1f64`。此前`.cache/canal-business/6f813aaa4c2d43f888beeaccc6f8efe5`25秒收敛超时缺SQL现场，根因仍未确认；后续已补失败快照，且另一次重建用例确认属于源binlog秒级平局的测试前提错误并修正，不能将前者无证据关闭。

- 00:20：新增各端仅使用本机DB的ExportSnapshot/SnapshotReceiver。帧传输保留字节/NULL/空串，1MiB上限、固定计划/严格序号、源摘要和目标事务复读摘要；乱序/重复/损坏/中断/SQL失败整批回滚，最终收据丢失返回commit_unknown。当前测试经过实际编解码和MySQL事务，尚未经过RabbitMQ传输。新增007_alignment_jobs：本机库表范围按实际lower_case_table_names归一化，不能换节点ID或换计划绕过未解决作业；目标收据与业务行原子提交，源确认另行持久记录。真实收据触发器1644失败、源确认失败、回执丢失后读取账本与启动阻断通过，SQL核心5.43秒。账本没有COMPLETE或解除阻断接口，后续须证明CDC交接；不增加公开执行入口。

- 23:56：两个真实候选Agent进程分别使用独立Canal读取器，RabbitMQ普通队列与实际MySQL，显式配对清单经规则比对和本机schema实时复核后装配独立Capture/Incoming；配对运行绑定不序列化，普通UI/MCP启用门禁不变。异名PK/列、uint64最大值/DECIMAL/二进制、CRUD、保留同步标记的本地更新、离线交错修改双向获胜、较新删除墓碑、重启、真正触发的回执回滚重试与本地登记失败后不重启Agent重连通过27.21秒；SUPERSEDED实际收据状态和待修复清空通过。证据`.cache/canal-business/310de5648490438d83e897cb0f206650`。先前两次失败发现并修复YAML/JSON空集合误判、生产插值JSON字节参数与Canal复用关闭连接，失败不计通过。
- 单向下发短测11.85秒回归，`.cache/canal-business/f382630b45f14eb4821d9456b070e50e`；全量test/vet/lint、前端build/binding通过。事件状态查询使用索引历史收据区分`superseded`和`recorded_outcome_unknown`，三语文案同步，不把loser记账当作写入获胜行；尚未新UI截图验收。
- 离线快照通过专用UTC会话支持TIMESTAMP(6)，使用后丢弃专用连接防止会话状态泄漏；真实+09/-04会话复制时间点/微秒/NULL且池默认时区不变，SQL核心4.45秒。禁用HARD双向草稿允许保存与手动规划，不等于启用。所有测试独占资源清理，无新包/业务部署。

- 23:17：新增BuildBidirectionalGraph，分离各端Capture/Incoming投影，中心捕获本机名称，Edge间经中心列名组合双射；同一中心表只合并兼容schema/启用/删除策略的明确配对，拒绝本机同表多目标等歧义。顺序稳定、数据不别名、不扩大显式分发或回源。普通/批量RoutingDownlinkDispatcher序列化覆盖三个源节点CRUD，保留BIGINT/二进制/事件身份并正确映射；这是序列化夹具，不是broker/生产三节点验收。全量test/vet/lint通过。实际schema交换、生产自动解析、首次对齐交接仍未接通，门禁不变。

- 23:08：新增`rulecheck.BuildBidirectionalPair`，实际schema完整可写列双射、实际复合PK顺序、同类型/排序规则/NULL、生成列排除、保留回放列及软删除双端检查；输出单个明确Edge/Server配对的Forward/Reverse/Relay投影。Relay保持边缘原始库表列，不能用正向映射再次改名；节点作用域不扩大，输出切片互相独立且不改保存规则。对齐计划复用配对检查。新增ValidateStructure仅供内部构造，不授权启用，公共Validate/MapEvent运行门禁保持原状。单测及真实SQL三投影精度（核心4.41秒）、全量test/vet/lint通过，夹具清理。这不是远端schema交换或Agent自动配对，更不是生产双向验收，后续仍须接通运行路由。

- 22:55：受规则门禁保护的Edge/Server Agent构造接入共享捕获屏障、本地登记与后台修复，复用现有Worker的重试/运行状态；客户端订阅扩展脉冲表，服务端Canal过滤仍需部署前验证。实际上下行夹具使用生产attach函数通过12.24秒/12.16秒，构造/调度单测及test/vet/lint通过；未冒充完整双节点双向运行验收。详细证据见AI_BOARD，所有独占资源清理，未出包/部署。

- `006_conflict_repair.sql` 持久保存实际获胜镜像、规范行与待修复标志，另有专用修复来源证明。入站获胜数据在同一事务内复读为无损镜像；本地winner直接保存捕获的完整可写列镜像，不读业务行锁。读取镜像时保留NULL/二进制/数字字符串，文本显式转换UTF8，避免原字符集字节被错误JSON编码。
- 本地loser现在原子保存待修复标志并允许CDC前进；RepairWorker从启用的本地LWW规则取任务，锁业务行/间隙并等待新鲜屏障，然后重读当前winner（不使用队列里过时版本），恢复数据且不改变原业务版本。修复、专用证明、apply日志及清任务原子提交。新本地winner清理过期任务。专用证明支持本节点修复的INSERT/UPDATE/DELETE，后续保留标记的实际业务更新或删除仍上传。
- 最新独占Canal上下行11.43秒/13.35秒覆盖上述修复全链路、回执失败回滚和重建worker重试、DECIMAL/二进制恢复、后续本地写入、硬墓碑及无重复任务；证据 `.cache/canal-business/55a1b4bf75014ecdb4f06b81e0644d85`、`8cbdf56d235d4818baf832aa0d25915b`。SQL核心5.07秒另验证缺行软删除逻辑墓碑、镜像保存失败全回滚、源删除时间、软行修复和物理缺行重建；软删除使用无并发写入的串行SQL夹具，不宣称CDC覆盖。全量test/vet/lint通过，资源清理。

- SQLWorker内部LWW路径已接通：MapEvent冻结映射前事件JSON；Apply校验原始身份和目标投影，强制实际schema、完整目标镜像、无触发器/外键及支持的真实PK；RR行/间隙锁之后等待捕获，之后才进入版本/schema锁。业务、版本、历史身份收据和apply日志同事务，较旧事件不写业务，重复事件先核对历史身份。含LWW的批次逐事件提交，避免跨事件持版本锁等待CDC。普通NONE写入保持原行为。
- 独占真实Canal+LocalRecorder+SQL Apply组合验证上行9.50秒/下行13.50秒：映射后的winner/loser/duplicate、回执失败回滚后重试、删除后旧事件不复活、新UPDATE恢复缺行、缺行DELETE墓碑、批次顺序及历史ID篡改。证据 `.cache/canal-business/ad54f976aa8048d6a1acdab3a203d426`、`1aee2dab550046ef9b20a888ea248ae4`。入站事件由夹具直接调用内部SQLWorker，生产构造和BIDIRECTIONAL门禁仍关闭，不是完整双节点双向验收；SOFT DELETE和更多约束组合仍需专门验证。首轮测试以宿主时钟构造新旧关系不可靠，已改用实际持久源版本的时间基线。

- `conflict.CanonicalKey` 按实际主键列顺序处理整数、DECIMAL、CHAR/VARCHAR、BINARY/VARBINARY；数值不经过浮点，文本使用实际 MySQL collation weight、字符集往返及 PAD 属性。拒绝前缀主键、范围/精度损失、未支持的类型（包括时间及浮点主键）；这是内部实现范围，不代表所有类型已支持。`PinKeySchema` 固定主键名称/类型/排序规则，变更时要求显式账本迁移。
- `005_conflict_receipts.sql` 保存键结构及所有已处理事件的原始身份/版本/规范行（含未获胜事件），防止旧事件被更新版本替代后逃过身份复用检查。已处理旧事件的重试返回 DUPLICATE；未见旧事件仍返回 SUPERSEDED，并原子保存收据。
- `LocalRecorder` 已实现匹配双向LWW规则的本地原始事件登记，过滤回放后、发布前执行，不锁业务行，不重写业务值；主键改变、结构漂移和非本地来源拒绝。真实 Canal 包装运行时已在独占夹具接通登记器，持有业务行锁时，屏障须等本地版本持久化后返回；生产构造尚未配置它。
- 最新真实 MySQL 核心回归4.74秒，覆盖整数/大小写/重音/组合字符/PAD与实际SQL等价、无业务锁登记、删除登记、键结构漂移、24并发、历史身份篡改及失败回滚；候选上下行8.45秒/8.33秒及登记屏障通过，证据 `.cache/canal-business/55a5324cfc0c44c489a5be451220312f`、`2b0aee2bd2df49e6ae35d075ef140d69`。test/vet/lint通过，所有夹具已清理。

- `internal/capture` 新鲜随机脉冲屏障及 server/edge `004_capture_fence.sql`：独立连接写入每节点固定单行，内存等待最长30秒，不使用重启前检查点；Source包装器剔除内部脉冲，完整批次经运行时处理并成功确认后才唤醒。SQL失败/取消/错误标识不误放行；发布/ACK失败后重试仍需实际处理成功。调用方必须先持有业务行/间隙锁、尚未取得版本锁，CDC必须先持久登记本地版本；这不是独立的冲突仲裁器。
- Canal上行/中心下行新增可选 `LocalVersions.RecordLocal` 接口，正常化及回放过滤之后、发布/确认之前执行；登记失败停止该批并重连重试，登记器必须幂等且不锁业务行。生产构造尚未配置登记器或屏障，不能据此解除双向门禁。
- 独占上下行候选链路9.59秒/9.27秒及独立真实Canal屏障回归通过：候选进程停止后复用专有订阅，持有业务行锁时确认此前变更已处理，随后纯脉冲无检查点反馈、内部事件不发布、数据库始终一行。证据 `.cache/canal-business/2181d7703ba049a29de461837eff63fd`、`1d992820fc18432bade7f6e242e4bf8d`。这是捕获顺序验收，不是本地版本持久登记或完整双向验收；所有夹具已清理。
- 冲突事务核心提取 `conflict.ApplyInTx`：由调用方管理事务提交/回滚，便于业务写入、版本/墓碑、消费回执原子接入；成功返回不代表可以ACK，失败必须整事务回滚。成功及回执失败后调用方仍可回滚的测试通过。

- `internal/alignment`：本机 MySQL `EXISTS` 实测空表，不用估算行数；配对计划、表/列/主键映射及反向映射、InnoDB/无损可逆类型检查、计划 hash/规则 hash/时效/明确确认验证。计划不是执行授权。
- 新增内部 `CopySnapshot` 离线复制原语：明确确认且规则禁用才执行，两个事务锁定源数据及目标空表的插入间隙，锁内重读 schema，流式按映射插入并复读校验行数/SHA256，出错目标整批回滚，最长两分钟。NULL/空值使用不同摘要编码，不经 float64；有触发器、不可证明触发器可见性、目标外键或 TIMESTAMP 列则拒绝。此原语仅由隔离测试调用，不提供远端数据库凭据方案，不替代后续各节点本机执行及 RabbitMQ 传输。
- `internal/conflict`：源端时间版本比较，来源时间类型不能混用；同源同时间按 binlog 序号/位置排序，跨节点相同时间按固定节点/事件 ID 排序；重复事件内容变化拒绝，删除也参与版本排序。
- SQL 事务组件将获胜业务写入、版本/删除墓碑和调用方收据同事务提交。旧事件不执行业务写入但必须落收据；任一步失败回滚，commit 结果不确定时返回错误，不宣称成功。
- 新增 server/edge `002_row_version.sql`，不重写历史迁移，不在现场执行迁移。
- 新增 server/edge `003_delete_replay.sql`。双向映射携带内部 `TrackDeleteReplay`，HARD DELETE 对有 FULL 元数据且确认无触发器的表，同事务标记、写专用删除收据、删除、写 apply 日志；普通单向删除保持原行为。loop 仅凭专用收据识别删除回放，不把旧 INSERT/UPDATE 收据作为删除证据。共享 `rulecheck.RequireNoTriggers` 同时供离线复制使用。
- 真实候选Agent上行8.45秒/下行7.88秒验证：跟踪硬删除产生的标记 UPDATE 和 DELETE 均不回传（后续业务插入已越过该 binlog 段）；收据失败整事务回滚后重试成功；本地保留旧远端 INSERT 标记的 DELETE 正常上传且唯一收据。证据 `.cache/canal-business/519d8227382644b697d132e959830565`、`115a24a76e8d4727b43d17975ce77dc8`。夹具直接调用禁用双向规则映射的内部 SQLWorker，不解除产品 BIDIRECTIONAL 门禁，不冒充完整双向冲突测试。
- Normalizer 在既有 `headers` 中增加 `event_time_source`：`source_binlog`、`source_event`、`processing_fallback`。保留旧单向缺时间的兼容行为，但冲突核心拒绝把 fallback 时间当成指令时间。
- 修复生产 `loop.Suppressor` 对保留旧同步标记的本地 UPDATE 的误过滤：FULL 前后镜像标记相同则上传，变更才核对收据，缺必要前镜像失败。真实候选Agent上下行均验证远端 SQL Apply INSERT 被过滤、随后未清空标记的本地 UPDATE 到达对端且只有一条收据；证据 `.cache/canal-business/af5ea013bd654667b0036bac76a11b52`、`a73a40ee4b9e437bb11c70d9ece0cc26`。这是回放/后续修改的真实链路回归，不是完整双向冲突验收。
- 单测覆盖空表四组合、反向主键/列映射、失效/篡改/未确认计划、乱序版本、同时间确定排序、重试/转发/JSON 无损、删除墓碑、原子事务失败分支。
- `scripts/test-alignment-core.ps1` 独占 loopback MySQL 8.4 组件测试通过：实际空表查询、正反向映射复制、BIGINT UNSIGNED/DECIMAL/二进制/NULL/空串/DATETIME(6)精度、第二行约束失败回滚、目标不再为空/结构漂移/触发器拒绝、两空跳过；24 并发版本、收据失败完整回滚、删除后旧事件不复活。最新执行2.79秒，夹具数据库和容器已清理；不是生产 Apply/CDC 接入测试。脚本兼容 Windows PowerShell 的原生命令 stderr 就绪探测。
- 全量 `go test ./...`、`go vet ./...`、`golangci-lint run ./...` 通过。

## 尚未接通，不能启用

1. 首次全量的CDC水位交接与产品级维护隔离。已有内部UTC快照、受限RabbitMQ传输、原子持久收据、实际Canal边界及受限结果核对恢复，只访问各端本机DB，不要求中转端持有远端DB凭据；没有公开执行器或维护租约编排。必须补捕获/入站旧事件过滤、血缘重置验证、全部参与节点就绪与启用握手、故障补偿/队列清理的可审计流程；已有行版本/墓碑不能擅自清理。所有作业phase仍阻断Agent，不能通过删账本解除。另源码复核确认withlin/canal-go v1.1.2底层net.Dial/read没有socket deadline，不能将context期限冒充不响应网络的硬终止保证，公开入口前需闭合。
2. 双向实际schema自动交换、配对维护及正式Wails/MCP启用流程未接通；当前需显式清单，真实两节点HARD/LWW链路短测已通过，三节点仅映射/序列化单测。其他主键类型、主键改变、局部列复制与更多约束组合也未支持。`Store.RowKey`禁止直接把源JSON当作行身份；聚合worker action仍需进一步区分混合批次实际写入/被取代数量。
3. 获胜镜像/本地loser修复/回放证明及生产后台调度已接通，GetEventStatus可区分SUPERSEDED；专门待修复列表/详细失败UI未完成。SOFT DELETE还需完整生产CDC组合回归。
4. 源时间/时钟校验、业务指令时间输入与删除时间来源、同表多节点真实并发故障回归。
5. 正式 Wails/MCP 操作、前端三语流程、capabilities 切换、双端安装包和链路验收。

`initial_alignment.execute=false`、`sync.bidirectional=false`保持不变，普通UI/MCP仍不能直接启用LAST_WRITE_WIN；显式配对清单的CLI运行路径已在独占两节点验收。MCP工具数量未变，没有新的安装包，不把该路径当作完整配置/首次全量产品流程。

## 后续交接验收条件

- 维护编排须在每个节点取得实际Agent独占租约，确认对应消费者停止后准备作业；单纯将规则保存为禁用或写入账本，不能停止已经运行的Agent。每端仅使用本机DB凭据，控制消息与业务帧分离。
- CDC接续必须同时处理本地尚未ACK的Canal输入和RabbitMQ中已发布的旧来源事件。只按计划参与库表/来源过滤，验证原始生产者血缘；缺少可信血缘的旧事件不能猜测归属，也不能用purge或ACK无关消息代替交接。
- 两空只跳过业务复制，不代表旧队列为空或历史DELETE已经处理；仍须证明后续不会被旧INSERT复活。多边缘同一同步组必须覆盖所有参与来源，不能只拿单对端点水位开启整个组。
- 恢复须覆盖源SOURCE_READY但最终收据丢失、目标提交但边界缺失、任一端重启及计划过期。已有目标结果只能核对，不能重复插入；源已写边界但尚未完成复制的取消/重规划目前没有安全公开操作，须保留审计且不能直接删除旧作业。
- 正式解除Agent阻断只能发生在持久交接证明、配对规则与全部参与端状态一致之后；重启须复查证明，不能只新增COMPLETE字符串或把现有所有phase视作完成。随后验证复制后到启用前的新写入、离线旧消息、第三节点转发、删除/复活、重复消息与断网恢复。
- 网络故障需包含对端不响应，而不仅是干净断开；确认socket读写可被期限实际中止、事务锁最终释放、未知提交保留可查询结果。超过64MiB的数据需要有界持久暂存方案与实际大表验收，当前逐帧未ACK缓冲不能冒充大表实现。

看板 FB-107 保持 open，FB-094 长测保持 open；实施继续沿该项，不创建第二活跃看板。
