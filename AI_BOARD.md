# AI Collaboration Board

本文件是前端 AI、后端 AI、测试 AI 与整体审阅/修改 AI 的唯一活跃交流看板。稳定接口契约仍写在 `.ai/docs/frontend-backend-contract.md`，但疑问、阻塞、分工和交接只写这里。

本文件与 `MEMORY.md` 同级，属于当前工作状态，不属于归档文档。`.ai/docs/` 只保留稳定说明、闭合记录、阶段总结和归档材料；旧路径 `.ai/docs/ai-collaboration-log.md` 仅保留迁移提示，不能继续写入活跃事项。

## 文件模型

保留两个核心文件：

| 文件 | 用途 |
| --- | --- |
| `.ai/docs/frontend-backend-contract.md` | 稳定 Wails API、DTO、错误语义、脱敏规则。 |
| `AI_BOARD.md` | 前端、后端、测试、审阅 AI 的活跃看板、交接记录、当前 open/blocked 项。 |
| `.ai/docs/` | 稳定文档、闭合记录、阶段总结和归档材料；不承载活跃看板。 |

不再新增前端看板、后端看板、测试看板、审阅看板或零散交流文件。需要前端处理的问题、需要后端处理的问题、需要测试验证的问题、整体审阅结论、接口变更疑问，都进入本文件顶部的 Active Board。

## AI Identities

所有 AI 在操作前必须声明一个当前身份，并用该身份写入 Active Board 和 Activity Log。

| Identity | Owner value | Scope | Boundary |
| --- | --- | --- | --- |
| Frontend AI | `frontend-ai` | React/Wails UI、前端服务层、三语文案、视觉与交互。 | 不实现同步核心，不直接访问 RabbitMQ/MySQL/HTTP。 |
| Backend AI | `backend-ai` | Go 后端、SyncAgent、Wails API、DTO、配置、迁移、安装器、MCP stdio。 | 涉及前端的数据、DTO、页面或阻塞必须写入 Active Board。 |
| Test AI | `test-ai` | 单元测试、smoke、lab/E2E、压测、发布门禁和测试证据。 | 默认不改产品行为；如需修测试夹具或脚本，必须说明范围。 |
| Review AI | `review-ai` | 跨模块审阅、架构评估、上线评估、风险清单和受控修正。 | 先给结论和风险；跨身份修改必须记录影响范围。 |

`Owner` 只能使用 `frontend-ai`、`backend-ai`、`test-ai`、`review-ai`。跨身份任务必须在 `Item` 或 `Next Action` 写明原因、影响范围和下一个 owner。

## Active Board

2026-09-12 15:41 backend-ai版本归档：用户要求提交并推送GitHub，本次仅归档累计源码、迁移、测试脚本及文档到origin/main，不继续功能开发或发布安装包。排除output截图、缓存、安装产物及本地凭据文件变更，FB-107/FB-094保持open；不把Git提交视作功能交付，owner backend-ai。

2026-09-12 01:42 backend-ai停止交接：用户设定的累计5小时上限已到，本轮不再继续开发。未出新安装包，首次对齐到增量交接及公开入口仍未完成；FB-107保持open，FB-094旧长测问题不关闭。源码/短测/全量门禁证据见阶段报告，测试资源已清理、业务现场未动。下一步等待用户新指令，owner backend-ai。

2026-09-12 01:39 backend-ai收尾FB-107：新增过期计划只准已提交目标恢复、源SOURCE_READY缺边界/账本身份与UUID篡改拒绝、取消后迟到标记拒绝。真实捕获恢复7.64秒、双Agent短测51.36秒、双物理MySQL传输4.46秒、含写锁覆盖SQL核心6.61秒通过；全量test/vet/lint与核心shuffle两轮、前端build/binding及三语1366/390截图通过。测试资源已清理、业务现场未动。CGO/GCC缺失未跑race；底层Canal网络硬超时/旧事件过滤/启用握手/公开操作仍open。已更新稳定架构约束与阶段报告；无新包、不部署、不长测，owner backend-ai。

2026-09-12 01:20 backend-ai接续FB-107：两端本机Canal探针不ACK，源持业务锁写/观察边界，目标标记与复制/收据同事务；SOURCE_READY在发送最终提交请求前保存源摘要，目标提交后边界失败可重连观察原标记，再核对源/目标持久结果幂等确认。跨test-ai仅单元及独占故障夹具；真实双Canal/RabbitMQ复制、1644提交后故障与重连恢复10.03秒通过，证据.cache/canal-business/d53e80054a2c4413b7287d9c2122dd2f；真实SQL摘要不符拒绝/收据丢失恢复5.43秒通过，全量test/vet/lint通过。全部phase仍阻断Agent；没有完成CDC过滤/双方启用握手/多端点协同或公开维护与恢复工具，无新增Wails DTO/入口，不出包。FB-107保持open，owner backend-ai。

2026-09-12 backend-ai接续FB-107：严格区分源binlog秒级平局后，增强6轮交替来源离线删除/业务行+墓碑+修复标志及实际进程启动阻断51.34秒通过，证据.cache/canal-business/fa28cfb4b29b4acaa4c7b60402ba1f64。最早6f813a超时缺少SQL现场、暂未复现，不能标记根因已修复；c1948d重建用例确认同秒平局的测试前提错误，9754d启动用例确认仅检查stderr漏日志，均已修正。新增内部RabbitMQ逐帧传输，持久消息/confirm/进度回信，数据与回执提交前不ACK输入，无自动清队列；暂限64MiB/两分钟。跨test-ai新增两台独立MySQL与RabbitMQ的独占短测脚本；产品级维护/恢复/CDC交接未接通，仍不出包。owner backend-ai。

2026-09-12 backend-ai FB-107回归发现：新一轮真实双向短测在离线新删除收敛处25秒超时，证据.cache/canal-business/6f813aaa4c2d43f888beeaccc6f8efe5。服务器有1213死锁并显示重试恢复，但超时根因尚未确认；未到启动阻断用例。暂停新增传输实现，补失败前业务行/版本/收据状态取证后复现，不能沿用此前通过记录宣称当前双向验收完成。所有本轮独占资源已由脚本清理。owner backend-ai。

2026-09-12 backend-ai接续FB-107：新增007对齐作业账本，准备记录按本机实际库表唯一锁定计划，目标提交收据与复制业务行同事务；源确认后仍保留对齐阻断。Edge/Server Agent启动读取未完成作业并报alignment_cutover_pending，旧库仅不存在新表时兼容，权限/连接等错误不放行。无新增Wails DTO/方法，现有启动错误展示沿用；跨test-ai仅独占SQL作业/故障/启动门禁验证。CDC交接尚未完成，无解除阻断或公开执行工具；owner backend-ai。

2026-09-12 backend-ai接续FB-107：拆分仅使用本机数据库的首次全量ExportSnapshot/SnapshotReceiver，数据帧保留原始字节、限制1MiB、严格序号/计划/摘要，目标事务复读校验后才返回提交收据。跨test-ai仅单元测试及独占MySQL短测；当前仍是内部离线传输原语，尚无RabbitMQ传输适配、持久作业恢复或CDC交接，不新增UI/MCP入口、不解除正式门禁。owner backend-ai。

2026-09-11 23:56 backend-ai进展FB-107：增强双节点短测27.21秒通过，实际观察edge-downlink回执失败和edge-cdc-canal登记失败1644后恢复，后者不重启Agent；SUPERSEDED账本经GetEventStatus底层服务实查正确。证据.cache/canal-business/310de5648490438d83e897cb0f206650；单向下发11.85秒回归.cache/canal-business/f382630b45f14eb4821d9456b070e50e。离线CopySnapshot使用专用UTC连接并用后丢弃，TIMESTAMP(6)跨+09/-04/NULL/池时区隔离实测通过，SQL核心4.45秒；禁用HARD双向草稿可保存，普通启用仍受门禁。event_status新增superseded/recorded_outcome_unknown，三语已补，合同已记；test/vet/lint和前端build/binding通过，未做新UI截图。全部测试资源清理；尚未远端schema自动采集、首次传输/作业恢复/CDC交接、完整Wails/MCP操作或出包，FB-107保持open，owner backend-ai。

2026-09-11 backend-ai接续FB-107：真实双Agent/双Canal/RabbitMQ异名LWW短测26.16秒通过，证据.cache/canal-business/3f5a44f278744662894278aa54052f2d；修复YAML/JSON空集合误判、生产插值JSON参数类型与Canal断线后旧连接复用。CLI run只在显式实际schema清单、规则比对、本机实时schema复核后绑定不可序列化的配对运行规则，普通UI/MCP启用门禁及能力声明暂不变，尚无首次对齐/自动配对，不出包。下一步event_status新增superseded与recorded_outcome_unknown，应用时间字段仍表示收据时间；跨frontend-ai仅三语文案，原因是不能把loser记账显示为已写入。owner backend-ai。

2026-09-11 backend-ai接续FB-107：将Capture/Incoming分离接入Agent装配，并增加本地schema复核与配对清单读取。清单只冻结规则和双方schema观测，不是授权，不能绕过现有运行门禁；远端采集/双方就绪协议仍待接入。计划CLI run增加可选-pair-manifest，Wails暂不增加方法或按钮，正式能力开启前需补稳定契约与前端状态。跨test-ai仅编译/装配/独占夹具，owner backend-ai。

2026-09-11 23:17 backend-ai进展FB-107：新增BuildBidirectionalGraph，基于双端实际schema观测生成各节点独立Capture/Incoming集合；中心捕获保持本机名称，Edge间转发经中心列名组合映射，不重写原事件。仅合并同一中心表的兼容配对，歧义端点/矛盾schema/混用启用或删除策略拒绝，显式分发范围不扩大，不分发回来源。单测覆盖异名/同名/无损值/顺序稳定/切片隔离/拒绝歧义；现有RoutingDownlinkDispatcher普通与批量序列化覆盖三源节点CRUD并验证原始身份，非真实broker验收。全量test/vet/lint通过；尚无远端schema交换、Agent自动配对或首次对齐交接，公共运行门禁不变，无UI/API改动、出包或部署。owner backend-ai，FB-107保持open。

2026-09-11 23:08 backend-ai进展FB-107：新增实际schema驱动的BuildBidirectionalPair，分别生成Edge上行/Server反向/Edge原名转发投影，固定端点节点、不扩大来源或分发范围；完整可写列双射、复合PK顺序、类型/排序规则/NULL、生成列、保留回放列及软删除双端校验。首次对齐计划复用此检查；ValidateStructure只校验结构，不替代公共Validate运行门禁。单测覆盖同名/异名/隐式碰撞/生成列/软删/作用域/不变性，独占MySQL实际schema+三投影写入保留BIGINT/DECIMAL/二进制通过（4.41秒），全量test/vet/lint通过、资源已清理。该配对尚未接入Agent自动解析或远端schema交换，不能宣称生产双向已启用；运行路由、事件诊断、首次全量传输/交接仍open。无UI/API变化、无出包/部署；跨test-ai仅独占SQL夹具，owner backend-ai。

2026-09-11 22:55 backend-ai进展FB-107：Edge/Server生产run入口新增共享屏障构造和conflict-repair调度；只在启用双向LWW规则时组装，现有规则运行门禁未解除，单向/禁用不触发新SQL。Canal订阅保留原过滤并追加本机脉冲表；服务端过滤仍须包含脉冲，不能把客户端扩展当作服务端配置已完成。构造/调度单测及实际Canal夹具使用同一个生产attach函数验证共享屏障，上下行12.24秒/12.16秒通过，证据.cache/canal-business/4353b0b2343c4d2e8432e317696a6004、c823ae51ac7948b688d5bf97e38de8af，test/vet/lint通过。首次误用PowerShell5触发stderr异常；仅清理本轮58619d0302904d77813511ec7ad64f22资源后用pwsh完成，测试资源均已清理。完整双向规则配对/运行验收、细粒度待修复与SUPERSEDED状态、首次对齐仍open，无新包/部署。用户停止条件改为出包或累计5小时先到即结束，owner backend-ai。

2026-09-11 backend-ai接续FB-107：接通受门禁保护的Agent冲突组件构造，同一节点的CDC包装器/LocalRecorder/SQLWorker/RepairWorker共享捕获屏障，修复调度复用现有Worker重试与状态日志。新增worker名conflict-repair沿用现有状态DTO，无新Wails方法；前端细粒度待修复/失败/SUPERSEDED仍待合同冻结。跨test-ai仅构造/调度测试及独占夹具，不操作业务环境；完整双向规则配对/首次对齐验收前不解除能力门禁，owner backend-ai。

2026-09-11 22:40 backend-ai进展FB-107：新增006迁移sync_conflict_state与sync_repair_replay，LWW获胜数据和本地获胜镜像随版本事务持久化；本地loser现原子记录待修复标志并让CDC继续前进，不再在捕获线程等待业务锁。RepairWorker只处理启用的本地双向LWW规则，按行/间隙锁->新鲜屏障->重新读取当前获胜版本和镜像顺序修复，保留原业务版本，修复写入/专用来源证明/apply日志/清任务同事务；新本地winner取消旧任务。loop新增仅本节点专用repair证明识别，UPDATE保留标记仍上传、DELETE不借用旧UPDATE/INSERT证明。真实Canal上下行11.43秒/13.35秒通过持久任务、失败回滚+重建worker重试、DECIMAL/二进制恢复、修复INSERT/UPDATE/DELETE不回放及后续本地修改不误丢、删除墓碑修复；证据.cache/canal-business/55a1b4bf75014ecdb4f06b81e0644d85、8cbdf56d235d4818baf832aa0d25915b，夹具清理。SQL核心5.07秒另验证缺行软删除创建逻辑墓碑、获胜镜像失败全回滚、软删除源时间、软行恢复/物理删除后重建、新winner取消待修复；该软删除用例是串行SQL夹具，不冒充CDC验证。全量test/vet/lint通过。内部修复闭合但生产构造/后台调度/失败诊断与前端状态尚未接入，后续需向UI区分待修复/失败/SUPERSEDED；暂无新增Wails DTO。首次对齐/完整双节点运行仍open，capabilities不变，无安装包/部署，owner backend-ai。

2026-09-11 22:13 backend-ai进展FB-107：SQLWorker接入内部LWW路径，MappedEvent冻结映射前原始事件JSON并校验映射投影；强制实际schema/完整目标镜像/无触发器与外键/规范主键、RR事务锁行或间隙，再等新鲜Canal屏障，之后才取schema/版本锁。获胜INSERT/UPDATE按真实行存在性写入（不使用ON DUP覆盖），DELETE保存墓碑，loser只落回执，历史重试校验原始身份后返回Duplicate；版本/业务/身份收据/apply日志原子提交。含LWW批次逐事件事务，不持前项版本锁等待后项屏障。Result.ConflictDecision为内部字段，暂无Wails DTO变化；后续生产事件状态/UI必须区分SUPERSEDED与实际写入，不能一律展示已写入，待backend冻结契约后交frontend-ai。真实Canal/LocalRecorder/SQL Apply组合在独占夹具验证映射、新旧版本/重复、失败回滚+重试、删除防复活、更新恢复缺行、缺行墓碑、批次顺序及历史ID篡改；上行9.50秒/下行13.50秒，证据.cache/canal-business/ad54f976aa8048d6a1acdab3a203d426、1aee2dab550046ef9b20a888ea248ae4，全部资源清理。首轮夹具错误使用宿主时间判断源事件先后已修正为实际持久源版本基线。test/vet/lint通过；真实入站事件由测试直接调用禁用规则映射后的SQLWorker，生产构造/运行门禁未解，不冒充双节点已启用双向。获胜状态持久内容/本地loser恢复、首次对齐、生产路由/状态/UI仍open，无新包/部署，owner backend-ai。

2026-09-11 21:56 backend-ai进展FB-107：新增conflict.CanonicalKey，按实际PRIMARY顺序规范整数/DECIMAL/二进制键，文本使用实际MySQL collation weight及PAD属性，拒绝精度损失/前缀PK/未支持类型；PinKeySchema与005迁移固定键结构，漂移拒绝。LocalRecorder已实现无业务行锁/不重写业务的本地持久版本登记，并在独占真实Canal包装运行时接通：持有业务行锁，屏障等待版本入库后才返回。sync_conflict_event保留所有已处理事件（含loser）的原始身份/版本/规范行，旧事件ID变更内容或改行拒绝。原已见旧事件重试现返回DUPLICATE而不是SUPERSEDED。单测/全量test/vet/lint通过；真实MySQL复制+24并发/回滚/主键排序规则/本地登记4.74秒，候选上下行8.45秒/8.33秒，证据.cache/canal-business/55a5324cfc0c44c489a5be451220312f、2b0aee2bd2df49e6ae35d075ef140d69，夹具已清理。新发现必须闭合：实际本地写入若比账本获胜版本旧，不能仅登记为loser后放行，否则本地行已被改坏；当前显式conflict_local_write_superseded阻断，获胜数据恢复路径未完成。生产SQLWorker/构造、其他PK类型/PK变更、完整对齐与双向流程仍open，无UI/DTO变更，不解除capabilities、不出包/部署，owner backend-ai。

2026-09-11 21:42 backend-ai进展FB-107：新增internal/capture和004_capture_fence迁移，每节点单行随机脉冲、30秒有界等待、批次包装器在完整处理/ACK成功后解除等待；不以旧持久检查点冒充当前进度，不发布内部脉冲，不产生纯脉冲检查点反馈。Canal上下行增加可选LocalVersions登记接口，失败不发布/不ACK并重置源，回放不登记为本地变更；实际规范主键登记实现尚未接入。单测覆盖错误token/库/表/节点/操作、取消/SQL失败清理、错误批次、发布/ACK失败和重试。真实Canal在候选进程停止后单独验证持有业务行锁时跨过先前变更、纯脉冲等待和单行存储；上下行完整夹具9.59秒/9.27秒通过，证据.cache/canal-business/2181d7703ba049a29de461837eff63fd、1d992820fc18432bade7f6e242e4bf8d，所有资源已清理。全量test/vet/lint通过。此处只证明捕获处理顺序，不证明完整冲突仲裁；实际主键规范化、登记实现、SQLWorker接入、首次对齐产品链路仍open，无UI/DTO变化、未出包/部署，owner backend-ai。

2026-09-11 21:34 backend-ai进展FB-107：提取conflict.ApplyInTx，复用调用方SQL事务，业务写入/版本/回执保持同事务；接口不自行commit/rollback，错误必须由调用方整事务回滚，成功也不代表可以ACK。新增成功和回执失败后调用方仍可回滚、回调事务一致性及nil依赖测试，全量test/vet/lint通过。本轮仅内部事务接口，无UI/DTO变化，尚未接入生产SQLWorker；本地待捕获写边界、SQL规范行身份、首次对齐运行链路仍open，未出包/部署，owner backend-ai。

2026-09-11 21:26 backend-ai进展FB-107：新增双向映射TrackDeleteReplay路径，HARD DELETE前同事务标记+专用sync_delete_replay收据+删除+apply日志，普通单向删除不变；RequireNoTriggers复用到alignment和delete，缺元数据/可见性或有触发器拒绝；loop用专用收据区分删除回放与保留旧INSERT标记的本地删除。新增003迁移、SQL事务/查询/标记单测。真实候选上下行分别8.45秒/7.88秒，验证标记UPDATE和DELETE均不回放、本地DELETE正常上传、收据失败全回滚后重试；隔离复制核心回归3.18秒，全量test/vet/lint通过，资源清理。证据.cache/canal-business/519d8227382644b697d132e959830565、115a24a76e8d4727b43d17975ce77dc8。测试直接调用禁用规则映射的内部Apply路径，不把它冒充已启用同表双向；capabilities仍false，首次对齐入口/CDC交接/完整仲裁仍open，无UI DTO改变/新安装包/部署，owner backend-ai。

2026-09-11 21:15 backend-ai进展FB-107：修复loop.Suppressor仅凭旧last_event_id收据误过滤后续本地UPDATE，改为FULL前后镜像比较标记；保留旧标记的本地修改上传，实际远端标记迁移才查回放，前镜像缺失返回明确错误。新增普通/映射标记单测，候选Agent真实Canal/Rabbit/MySQL上下行各验证SQL Apply回放INSERT过滤和随后本地UPDATE唯一收据；证据.cache/canal-business/af5ea013bd654667b0036bac76a11b52（7.95秒）、a73a40ee4b9e437bb11c70d9ece0cc26（8.59秒）。全量test/vet/lint通过，夹具已清理；未部署/出包。无DTO变化，前端仅沿用错误展示；HARD DELETE回放、双向仲裁接入及首次对齐运行入口仍open，owner backend-ai。

2026-09-11 21:07 backend-ai进展FB-107：新增内部CopySnapshot离线复制原语，明确确认+禁用规则、源/目标事务锁与空表重查、schema重查、按映射流式复制及目标行数/摘要核验、失败整批回滚；独占MySQL验证正反向映射/无损精度/约束失败回滚/结构漂移与触发器拒绝。test/vet/lint通过，测试容器已清理。该原语只用于内部隔离测试，未接入远端直连或运行入口；RabbitMQ分块/作业恢复/CDC交接/生产双向Apply仍未完成，TIMESTAMP适配未完成，capabilities仍false。无新增DTO/前端操作/安装包，FB-107保持open，owner backend-ai；严格业务指令时间字段和删除时间仍待确认。

2026-09-11 20:56 backend-ai进展FB-107：新增alignment空表实测/手动配对方向/双向可逆映射/计划hash与时效，conflict源时间/确定同时间顺序/删除版本及SQL业务-版本-收据原子事务核心，新增002_row_version迁移；Normalizer标记source_binlog/source_event/processing_fallback防止拿处理时间仲裁。独占MySQL24并发/收据失败回滚/墓碑及test/vet/lint通过，夹具已清理。尚无首次复制执行器、CDC交接、生产双向Apply/本地待捕获写登记/回环接入，capabilities仍false，工具/安装包未变，FB-107保持open。时间来源选择等待用户答复；下一owner backend-ai继续真实链路，frontend-ai待DTO冻结。记录docs/alignment-bidirectional-progress-20260911.md。

2026-09-11 backend-ai接续FB-107：用户明确开始实现首次全量与同表双向；首次全量为手动确认、有数据端到空表端，两空跳过、两端非空拒绝自动覆盖，按映射后的实际表判断。用户进一步确定双向冲突采用源端指令/事件时间较晚者优先，非接收/处理时间；需固定同时间排序、删除墓碑、时钟/缺时间保护，并区分业务指令时间字段与Canal源事件时间。backend-ai负责核心/CLI/MCP/DTO，跨test-ai仅独占夹具；前端新增对齐/冲突时间来源配置在稳定接口后交frontend-ai。先实现和验证安全核心，未经真实链路验收不解除unsupported能力声明；不改现有业务配置/数据、不部署或启动长测。<!-- 待确认 -->严格业务指令时间的字段名及删除时间写入方式尚未提供。

2026-09-11 review-ai回复FB-107功能核对：源码能力声明及运行策略一致，initial_alignment.execute/online=false、sync.bidirectional=false，启用规则仅支持NONE且无冲突仲裁；MANUAL只是可保存策略。0.46.16最终Agent隔离上下行CDC证据均passed，另有真实MySQL/RabbitMQ严格CRUD/失败重试证据，不能替代当前业务表或长测验收。本轮仅只读核对源码/既有证据并记录答复，未连接现场、未改产品/配置、未重新运行测试；FB-107及FB-094保持open，后续owner backend-ai/test-ai按既有范围推进。

2026-09-11 15:55 backend-ai交付0.46.16升级包：最终SHA256 0EB2142378774225E6E23CFBF14AC945DB48B868B63BD7094D2384732A6116A7，Agent16C571C9/UI BCE48F85。43文件/5资产、隔离双覆盖/配置原文、32/64各15、真实双方向Canal CRUD/精度/重启/收据、普通stdio与真实SSH39工具5资源、日志及UI回归通过。VPN实现按用户决定撤回；用户自行安装，长测安装后安排，无业务配置/表/队列/Agent变更。详见docs/v0.46.16-upgrade-handoff-20260911.md；跨frontend/test范围已交接，未将首次对齐/历史误账/冲突仲裁或旧七小时失败关闭。

2026-09-11 backend-ai FB-107真实Canal短测发现二进制高位字节被UTF-8重编码：00017F80FF落库00017FC280C3BF，其他BIGINT/DECIMAL/NULL/time正确。接修Canal按JDBC二进制类型解码、稳定带标签JSON行值与SQL字节绑定，需单测+真实候选CRUD/重启回归。双端需同包升级；旧版不支持新增二进制标签，历史错值不自动修复。无新增UI操作/API，仅行值协议及诊断展示影响，owner backend-ai。

2026-09-11 backend-ai：用户撤销VPN远程MCP，回到SSH承载原stdio；最新目标仅交付升级安装包，长测安装后另行安排，不自动部署。FB-108改为撤回VPN实现并验证SSH兼容；跨frontend-ai删除专用设置/绑定，跨test-ai仅隔离短测与封包门禁。FB-107本包交付已实现的正确性修复；首次对齐执行及历史误账修复不纳入本包完成声明，继续open。

2026-09-11 backend-ai FB-107发现并接修Edge下行目标库口径：生产默认库override覆盖SERVER_TO_EDGE显式target_database_name，预检/治理与实际写库不一致。改为直接下发显式目标优先，空目标保留本机默认；EDGE_TO_SERVER转发沿用本机落库兼容行为。同步预检、治理、错误元数据与真实Broker/MySQL测试；不改现场配置。owner backend-ai。

2026-09-11 backend-ai FB-108进展：已实现官方SDK Streamable HTTP、每设备Bearer token、VPN/Origin/来源边界、机器DPAPI+ACL、独立SYSTEM启动任务脚本、升级预停与资源manifest、安装选项/连接复制页、三语设置/轮换/复制。真实TCP SDK与CLI生命周期/实际目录与摘要/本机探针、DPAPI/DACL、设置鉴权、五模式安装夹具、启动归属模拟测试通过；三语1366/390 UI交互/截图通过，NSIS测试编译及中文原生选项预览通过。当前非管理员，未注册真实任务/防火墙、未安装部署，真实SYSTEM无登录重启/VPN对端/完成页复制仍open，不把配置启用当运行成功。合同及docs/remote-mcp-vpn.md已同步；下个owner backend-ai继续隔离验收及原FB-107。

| ID | Owner | Type | Status | Item | Next Action |
| --- | --- | --- | --- | --- | --- |
| FB-109 | frontend-ai | risk | open | npm audit报告7项既有前端构建依赖漏洞（含Babel/browserslist/nanoid/postcss/vite等）；新增lucide不是本次报告来源，未擅自批量升级。 | 独立评估可达性和兼容版本，升级后跑构建及三语UI回归；不作为本轮功能门禁已解决。 |

2026-09-11 backend-ai新增FB-108：用户要求安装时可选远程MCP并确认首版仅受保护VPN；设备独立token、地址/客户端配置、重启无人登录恢复，保留stdio，不加角色/审批/逐项授权，中心中转不在首版。backend-ai跨frontend-ai负责三语设置/安装展示，跨test-ai仅隔离配置/loopback协议/安装夹具，不部署业务环境。需解决现有用户DPAPI与SYSTEM启动的兼容，仅选择远程功能时迁机器范围+ACL。原FB-107整改保持open，不以新需求替代旧验收。

| ID | Owner | Type | Status | Item | Next Action |
| --- | --- | --- | --- | --- | --- |
| FB-108 | backend-ai | decision | closed | 用户撤销VPN方案，相关HTTP/token/机器凭据/SYSTEM任务/防火墙/安装及UI实现已撤回，0.46.16沿用SSH+stdio。 | 最终包Agent真实边缘SSH握手/39工具/5资源/能力通过，临时目录已清理；不安装SSH、不改用户公钥或业务配置。 |

2026-09-11 backend-ai接口预告（FB-107）：新增按规则+事件的队列隔离plan/apply/audit，固定配置队列、规则revision及消息指纹，有界检查，明确确认后持久隔离副本并ACK原消息；不提供purge、不标记业务应用成功。跨frontend-ai仅DTO/binding与三语事件处理入口，后端先实现并用独占Broker验证；MCP全开放仍保留操作确认，无新增授权体系。

2026-09-11 13:38 backend-ai继续FB-107：收紧BINARY宽度无损判断、修软删除运行时误查排除列/跨规则软删映射缓存；预检新增可见触发器/入出外键警告，明确元数据可见性与副作用未核验。独占MySQL真实回归通过。修PeekMessages立即requeue导致重复取样，独占RabbitMQ随机端口45054上消息完整保留及禁用规则反复退回/启用后ACK各5次通过；后者Apply是测试替身，不冒充真实DB整链路。全量go test/vet通过；未出包/部署，历史误账/受控残留动作/首次对齐仍继续。

2026-09-11 13:27 backend-ai进展（FB-107仍open，非交付）：本机schema/主键/InnoDB/软删列/权限预检及启用前检查、生产Apply事务内真实PK防护、CAS/saved/active_revision、按规则选择库表治理、运行事件状态工具与三语UI、禁用规则残留不ACK、MCP输入UseNumber已实施。独占MySQL与实际stdio关键回归通过；全量test/lint/tsc及三语宽窄屏检查通过。剩余历史误账修复、残留审计工具、首次对齐及最终包真实链路继续，不接触业务AI环境。下一owner仍backend-ai。

2026-09-11 backend-ai：用户明确继续完成剩余需求，不在首批阶段小结停止。FB-107新增规则预检接口（按rule_id及source/target选择本机规则引用库表）、后续事件状态与revision保护；跨frontend-ai负责对应合同/规则页面入口，跨test-ai仅独占容器/库/队列及隔离配置。MCP工具全开放，不恢复审批授权体系；实际业务环境和对方残留不动。

2026-09-11 backend-ai：用户纠偏：MCP权限全开放；“隔离”仅指本方测试库/表/config，不是产品范围授权。撤销本轮FB-106申请/审批/授权存储/UI，不保留这套额外机制。保留首次对齐默认关闭/手动选择和可信时间脱敏修复。当前优先答复业务报告Q1-Q12并修复正确性，不改业务AI数据库、配置、现场进程或残留消息。

| ID | Owner | Type | Status | Item | Next Action |
| --- | --- | --- | --- | --- | --- |
| FB-107 | backend-ai | bug | open | 0.46.16为此前已交付整改包。本轮源码新增配对LWW/墓碑/loser修复，真实双Agent短测51.36秒通过；受限首次快照/RabbitMQ/持久收据/Canal边界/核对恢复实测通过，仍无新安装包。 | 完成CDC旧事件过滤及血缘验证、维护租约/全部参与端启用握手、自动配对和公开Wails/MCP执行恢复后再验收出包；所有作业仍阻断Agent，不能删账本解锁。较早6f813a超时根因未闭环；底层Canal socket deadline及大表暂存也未完成。用户限定出包或累计5小时先到即停止，不部署/长测；详见docs/alignment-bidirectional-progress-20260911.md。 |


2026-09-11 backend-ai：用户已授权实施方案并进行完备测试，随后明确只用隔离数据，不操作业务表。本轮按P0/P1起步再接M1，现场安装/配置/业务数据保持。backend-ai跨frontend-ai仅共用能力的Wails视图及三语交互，跨test-ai仅隔离夹具/协议/E2E；各阶段证据独立，不将M2或未跑门禁记为完成。

2026-09-11 backend-ai补充边界：业务开发AI已进场，用户明确试验仅限我方测试库和表，开发业务表的配置不动。所有真实测试必须使用独占测试数据库/表/队列及隔离config/rules白名单，禁止读取调整业务表数据、同步规则、连接或节点配置，不借业务表作探针。

2026-09-11 review-ai：本轮仅制定首次对齐/全功能MCP方案。用户明确首次对齐为可选项、不能全自动：默认关闭，手动范围/差异预览/确认，安装注册保存启停均不触发；未选择时保持原增量。下列新任务尚未实施，不代表批准部署、改库或启动长测。

2026-09-11 backend-ai：用户最新要求中心本机与边缘均正式安装新包，取代下方旧的中心仅工作区决定；本轮只出包，不自动安装或启动长测。

2026-09-10 test-ai：用户确认中心直接使用本机工作区运行，无须安装；FB-078 的中心安装阻塞由此解除，转 open 待完整测试。中心固定候选二进制、边缘正式安装版，不将工作区测试记为双端实装通过。

| ID | Owner | Type | Status | Item | Next Action |
| --- | --- | --- | --- | --- | --- |
| FB-106 | backend-ai | task | closed | 授权系统按用户纠偏撤回，不作为功能交付。MCP启用后全开放，删除申请/审批/存储/UI；保留DISABLED/MANUAL选项、capabilities及可信时间修复。 | 后续正确性和接口验证转FB-107；此前审批测试不再是现行产品证据。 |
| FB-105 | review-ai | decision | open | 首次对齐方案P0待冻结业务边界：维护窗口、表/行权威、跨Edge主键、范围撤回、数据量/短锁权限；用户已确认可选非全自动，不能将待确认事项当作停写或覆盖授权。 | 依据.ai/docs/initial-alignment-implementation-plan.md第11节确认；默认KEEP、不创建业务schema；M1/M2分别验收，不关闭既有FB-094/084/089等项。 |
| FB-104 | test-ai | test | open | 拟议对齐/MCP发布门禁T01-T11：旧33工具/5资源/CLI全入口兼容，中心+两Edge范围/精度/故障/在线交接/100GB预算，默认不执行和明确批准测试。 | 待实现后按覆盖清单逐行真实stdio/SSH验收；重启后对齐PAUSED、无授权start不得写入。源码/最终包hash绑定；不能自动启动长测或把旧125秒失败追认为通过。 |
| FB-103 | frontend-ai | task | open | 拟议新增三语数据对齐页及接入可选项：执行首次对齐默认未勾选，选择范围/差异预览/明确确认；未选择直接原增量。旧UI管理功能均有对应MCP能力。 | 等backend-ai冻结DTO后用Wails薄适配实现列表/详情/冲突/进度/暂停恢复取消；保存/启停不触发，重启暂停、部分提交可见，不直接访问DB/HTTP。 |
| FB-102 | backend-ai | task | open | 按方案补正式MCP授权与旧功能缺口，并新增手动QUIESCED对齐、持久作业、行范围/映射/条件写/收据核验；后续ONLINE水位/暂存/代际交接。当前仅拟议，未修改稳定接口。 | 先P0/P1再P2/P3；参见.ai/docs/mcp-capability-coverage-plan.md逐项33工具+旧CLI/UI+新增能力。默认DISABLED、无AUTO，MCP首次start需计划明确批准；记录DTO/规则/迁移影响后交frontend-ai。 |
| FB-101 | review-ai | task | closed | 已完成首次对齐具体实施方案、技术路线、阶段人日/门禁、全量MCP覆盖清单及可编辑架构图。纳入用户追加“可选、不能全自动”：默认关闭、手动预览确认、启停不触发、重启后暂停。 | 文档.ai/docs/initial-alignment-implementation-plan.md及mcp-capability-coverage-plan.md；33工具/38CLI无遗漏、链接及图结构检查通过，现有test/vet/lint通过，非新增功能验收。只关闭方案，实施/业务确认见FB-102至105；未改产品/现场/安装包或启动同步；图未导出PNG。 |
| FB-100 | test-ai | test | closed | 用户完成本机安装后只读核验：10:42:59正式0.46.15安装passed/reuse，组件安装、密码迁移、拓扑配置均skipped，config/rules kept-existing；正式Agent/UI哈希与F012包一致。 | 注册版本0.46.15、无原生RabbitMQ/Canal/MySQL相关服务；Docker原实例仍运行。原生Erlang来自09-08旧安装（旧日志及目录时间），非本次新增；离线packages仅复制未执行。安装版MCP MySQL/RabbitMQ连接ok，server-001/external保留，Agent stopped且无旧工作区进程。未启停/改配置/清理；未做同步E2E或长测，FB-094/084/089仍open（此次无原UI待恢复，不算FB-084通过）。 |
| FB-099 | backend-ai | task | closed | 0.46.15 F012F1FE新包已交付半自动NSIS组件选择页，中英日，默认复用；静默/直接脚本也默认跳过，显式InstallSystemComponents才装组件，互斥参数拒绝。复用保留配置原文/密码，跳过组件安装/迁移/拓扑；首次创建external配置。 | backend-ai跨test-ai仅模式/升级/UI验证：五模式fixture、三语真实预览、NSIS冲突返回2、双覆盖配置哈希、32/64各15、43文件/5资产、MCP33工具/5资源、test/vet/lint全通过。无Wails DTO/同步变化；记录docs/v0.46.15-component-mode-handoff-20260911.md。未实装或自动Docker检测；FB-094/084/089仍open。 |
| FB-098 | backend-ai | bug | closed | 默认安装可能重复安装Docker已有组件的问题，通过用户授权的半自动方案FB-099处理：0.46.15新增可见选择页且默认复用，组件安装需显式选择；本机Docker可保持默认，不再需要记命令参数。 | 选择复用时跳过组件及密码/拓扑配置，旧配置不改；不是自动Docker识别。原0.46.14仍须/SkipSystemComponents，新包F012替代本次交付。自动环境探测不纳入本轮；未部署现场。 |
| FB-097 | backend-ai | task | closed | 0.46.14 C580851F安装包交付，纳入FB-096日志状态UPDATE修复；同包供中心/边缘安装，内含业务AI MCP说明，另生成双端普通模式客户端JSON。不带实验室凭据清单。 | 全量test/vet/lint、Wails/CLI/契约、42文件/5资产hash、隔离双覆盖、32/64位各15项、MCP标准握手/33工具/5资源/真实ERROR读取通过。backend-ai跨test-ai仅补封包标准验证与独立证据目录，无UI/DTO变化。记录docs/v0.46.14-package-handoff-20260911.md；未部署，中心须先停工作区Agent。FB-094/084/089仍open。 |
| FB-096 | backend-ai | bug | closed | 已先报告后修复确认后的日志状态UPSERT锁放大：现按已有event_id事务UPDATE，不覆盖payload；缺失记录回滚、重复完成安全，确认后状态提交成功才提交Canal位点。 | 真实MySQL产品SQL UPSERT/UPDATE/UPSERT对照1.55秒：旧路径阻塞、新路径不阻塞；跨块缺失回滚/payload/重入及运行时失败重放、全量test/vet/lint通过。无UI/DTO/参数变化。仅源码，未出包部署；不是完整性能验收，FB-094仍open。 |
| FB-095 | backend-ai | task | closed | 用户确认方案后，review-ai→test-ai有界原表续跑→报告更新→backend-ai受控修复完成本轮交付。原表60万历史和热点版本续跑574.185秒，新范围50列/2000键/账本通过；短暂停Canal追赶后段再现约1.9秒event_log与PRIMARY尾部锁等待。 | 报告docs/performance-root-cause-20260911.md明确证据与边界，锁机制修复见FB-096；未复现完整125秒超时、不追加长测或追认七小时通过。双端原表先备份；旧11040消息隔离保留。09:36原规则/Agent15972与19320/队列0/Canal和PFS恢复，无UI影响。 |
| FB-094 | backend-ai | bug | open | 0.46.13七小时213112于09-11 02:08:12因延迟探针超过125秒失败；峰值入口ready10925/unacked50，每方向605440 INSERT/320720 UPDATE后停止，最终一致性未验收。 | FB-095/096已找到并修复可重复的日志状态UPSERT锁放大，源码真实锁/正确性门禁通过；修复版尚未出包部署或双端完整验收，原125秒失败仍未闭环。旧11040消息已confirm隔离保留，09:36运行队列0、原规则CD26及Agent15972/19320恢复；原表诊断续写前备份见报告。 |
| FB-093 | test-ai | test | closed | wide25200s_0910_213112于09-11 02:08:12延迟探针125秒超时失败，02:08:29恢复结束，实际约4小时36分，未跑满七小时。两次15分钟停机/追平及DDL通过，峰值阶段失败，最终一致性未验收。 | 记录为失败闭环，问题转FB-094；不把closed写成测试通过。每方向605440 INSERT/320720 UPDATE，最后采样净空间下降Edge6.19GB/Server22.68GB，非容量触发。原规则/Agent恢复passed、awake释放，但08:28运行队列仍11040残留待处理，未自动重启。 |
| FB-092 | test-ai | decision | open | 用户要求按100 GB设计，已形成docs/100gb-storage-test-plan.md：暂按每节点总工作存储100,000,000,000字节，已有数据计入、分项归属、持续增长与临时峰值分开，保留1.5安全系数与独立物理盘安全下限。 | 先实现分项容量测量/预算绑定及回归，再完整30分钟重验后组织原负载36000秒测试；70GB告警、80GB停止造数并排空，越界记失败不算通过。不迁移数据、不裁剪幂等/待消费记录。自动数据保留如需产品实现另交backend-ai。本轮仅设计，未改容量脚本/产品或启动长测。 |
| FB-091 | test-ai | task | closed | 按用户授权清理边缘192.168.10.105 MySQL无用测试表：依据终态run清单、当前规则、外键/触发器/视图检查，105张旧nb7表先完整mysqldump到D盘并hash，再复核行数后DROP；2379293行，释放3780857856字节。 | 最新175644验收表、nb_e2e_probe、同步系统表及spindle_edge_ac01业务库保留；MySQL查询正常，Agent18980存活，规则CD26哈希不变。备份D:/NodeBridge-Test-Backups/cleanup-20260910-205208/scada_edge-obsolete.sql，清单.cache/edge-cleanup-20260910-205208-manifest.json。C现159.6GiB，仍未满足原276GiB十小时预测；未迁移MySQL、未清binlog或重开长测，FB-090仍blocked。 |
| FB-090 | test-ai | test | blocked | 0.46.13 widesmoke_0910_175644完整1801.222秒completed；每端64220 INSERT/39850 UPDATE，50列/精确账本、13生命周期、双故障/DDL/晚期回放交接、126样本含30压力/方向全部通过。恢复passed，租约已释放。 | 十小时未启动：边缘最低空间法测得5,293,061 bytes/s，36000秒*1.5+10GiB需约276GiB，C盘现约156GiB；D盘约929GiB空闲。已请求用户批准迁移实验室MySQL数据到D盘并重新验证，未停库/迁移/清历史/放宽门禁。当前短测证明绑定36000秒，但实际启动仍必须通过实时容量检查；旧失败保持。 |
| FB-088 | test-ai | test | closed | 用户确认边缘安装后再次只读实测：192.168.10.105正式安装MCP握手0.46.13成功，普通模式15工具、overview/logs/agent_status正常；保存的MySQL、本地RabbitMQ、中心RabbitMQ测试均ok=true。中心MySQL及RabbitMQ也正常。 | 边缘同步Agent仍stopped，中心仍原0.46.12工作区PID17460 running；两端读取旧sync-agent.log各5条。未改配置、未启停同步，不视为端到端同步通过。当前对话未加载NodeBridge原生工具入口，本次通过直接stdio及SSH stdio验证。 |
| FB-089 | backend-ai | bug | closed | 0.46.16类型感知保留AgentProcessStatus可信时间/PID，自由文本错误与路径仍脱敏。数字密码2026与started_at/exited_at/PID碰撞回归通过。 | 修复已入包，普通stdio/真实SSH及日志数值脱敏验证通过；不代表旧长测已验收。 |
| FB-087 | backend-ai | task | closed | 0.46.13最终5A38候选包交付：真实Edge ApplyBatch、持久ERROR/WARN/轮转/脱敏/阶段与卡顿采样、UI/MCP日志读取、NSIS三语向导与会话恢复。GetLogs DTO/33工具不变，显式-log兼容；MCP全协议纯数字密码JSON回归及真实启动ERROR读取通过。 | 全量test/vet/lint、Wails/契约/CLI、42文件/5资产hash、双覆盖、32/64位各15回归、MCP版本/诊断通过，最终Agent6C0795与证据绑定。报告docs/v0.46.13-candidate-handoff-20260910.md；不自动安装，FB-084实际UI恢复及FB-085双端性能留复测。 |
| FB-086 | backend-ai | task | closed | 固定utf8mb4驱动插值与真实下行批量纳入0.46.13。真实隔离50列1200混合下行逐事件62–81/s→批量353–410/s，50万历史356–401/s；全列/版本/Apply数量、SQL失败整批回滚/重入队/重放通过。 | 用户最新明确交付新包后复测，候选已由FB-087交付；局部基准不作双端验收，原门禁/可靠性/现场MySQL保持。FB-085仍open，旧formal-ready撤销保留；未部署候选或启动新长测。 |
| FB-085 | test-ai | bug | open | wide7h_0910_100650于14:44:10因latency probe exceeded 125 seconds自动失败，14:44:25 finished，运行约4小时36分。失败前每方向约80 INSERT/s+40 UPDATE/s，中心入口ready13585；已完成样本P99上行9674ms/下行12461ms，超时探针不计入这些完成样本。 | 先保留现场证据并定位高负载积压/探针超时原因，不放宽门禁，不自动重启长测。restoration passed，正常Agent中心17460/边缘5836存活、边缘原规则hash一致、awake5388 released已核验。FB-083未通过；UI修复仍由FB-084独立处理。 |
| FB-084 | backend-ai | bug | open | wide7h结束恢复后，0.46.13实现preflight记录原UI的SessionId/owner SID，安装完成以临时Limited InteractiveToken任务恢复，只允许唯一原用户桌面；拒绝Session0/模糊会话，避免重复UI，失败记录warning并清理临时任务。 | 9项选择器/模拟任务回归与隔离覆盖通过；真实原用户桌面恢复仍待用户安装复测，不写成已验收。安装资源归属、自启动与NSIS框架不变；当前候选未部署。 |
| FB-083 | test-ai | test | closed | 七小时wide7h_0910_100650于14:44:10延迟探针超过125秒自动失败，14:44:25 finished，未满七小时。保留原始失败，不追认为通过。 | 恢复passed、awake5388 released，backend-ai按FB-086完成13768条本run残留confirm隔离，正常入口/下行ready及unacked均0、原Agent17460/5836仍存活；旧准入已撤销。完整最终一致性未验收，高负载问题由FB-085/086继续处理。 |
| FB-082 | test-ai | test | closed | v0.46.12包/边缘实装/中心工作区E61核验通过。093438完整1800.925秒completed，每端64220 INSERT/39850 UPDATE，全50列/oracle/精确账本、33452次版本观察、12生命周期、双故障/DDL/晚期探针交接通过。 | 135样本/方向（102稳态/33压力），Edge稳态P95=607.918ms/全部P99=1011.091ms，Server=634.690/2893.418ms通过。恢复passed、旧awake4940released退出，正式准入已生成，七小时转FB-083。 |
| FB-081 | backend-ai | task | closed | Store事件日志改为同事务有界多行SQL，128行/估算1MiB，保留顺序、冲突字段和错误传播。全量门禁、真实257事件16字段逐条对照/跨段重复/特殊大payload/后段SQL错误整批回滚通过。 | 同隔离1000混合/320回放基准4.400→2.244s，日志3.097→1.148s；已纳入v0.46.12交test-ai FB-082。SyncEvent/Wails DTO/UI不变，局部收益不是整体延迟通过证明。 |
| FB-080 | test-ai | test | closed | 083751全量账本通过但双向P99=6852.765/12428.633ms失败，原证据保留。FB-081定位并优化事件日志逐条SQL往返，未改负载/可靠性/延迟阈值或清历史。 | v0.46.12的093438完整30分钟同负载、全压力样本P99=1011.091/2893.418ms通过，闭环证据见FB-082；不宣称唯一性能瓶颈已完全证明或七小时通过。 |
| FB-079 | test-ai | task | closed | WorkspaceServer监督器/准入绑定模式、路径和hash，边缘仍要求正式安装；拒绝越界/旧进程/hash及证明误用回归通过。 | 093438完整30分钟、恢复及正式准入验证通过，100650正式Start/Run接受同一绑定模式并运行。Installed默认未改，工作区结果不写成双端实装。 |
| FB-078 | test-ai | decision | closed | 按用户决定取消中心安装前置条件，改为工作区候选，未重试UAC；v0.46.11回放/位点修复及v0.46.12日志优化不改SyncEvent/Wails DTO/UI。 | v0.46.12完整短测通过（FB-082），MySQL保持8GiB/2GiB，正式七小时已启动（FB-083）。旧083751失败不追认。 |
| FB-077 | test-ai | bug | closed | 234447及083751压力延迟失败均保留，未通过扩大内存或放宽门禁认定解决。FB-081日志多行SQL优化后，093438完整同负载验收通过。 | 135样本含33压力样本/方向，稳态P95与全部P99均满足原门禁（FB-082）。七小时长期结果仍待FB-083。 |
| FB-076 | backend-ai | bug | closed | Store回放判定已改为事务内FOR SHARE及context/error传播，查询失败不发布不ACK；真实提交/回滚/锁等待取消实验与核心回归通过。234447多出一条UPDATE证据保留。 | 083751与093438精确Apply/SUCCESS账本均通过，093438完整30分钟及33452次在线版本观察通过；未将精确账本改为容差，不追认旧失败。 |
| FB-075 | test-ai | bug | closed | 最终核验重试已统一DateTimeOffset UTC并记录原始错误；跨午夜/时区/成功不重试/期限边界回归通过。 | 083751原始延迟错误完整记录且恢复，093438完整30分钟与最终验收通过，未再被时间类型错误掩盖；234447失败保留。 |
| FB-074 | test-ai | bug | closed | File.Replace原子替换/2秒有界共享锁重试修复已通过专项及234447约28分钟完整负载阶段，无相同文件错误；错误调用栈成功定位后续FB-075。 | 原文件锁/持续失败保留/UTF8等回归及全量门禁通过；234447因独立FB-075/076未通过，不能将本项关闭等同完整准入。 |
| FB-073 | backend-ai | bug | open | v0.46.11已实现空/全抑制批次只ACK不写SQL位点，有效业务保留位点语义，单测通过并含于v0.46.12。旧空闲自写+816证据保留。 | 093438完整短测通过；双端纯空闲位点写计数专项复核仍待进行，不认定原自写是所有性能问题唯一根因。 |
| FB-072 | backend-ai | bug | open | 19:22实测概览downlink_queue_depth=0，而RabbitMQ edge-001.downlink.q实际ready=499；逐条metadata确认为184033遗留消息，随后confirm隔离。 | 核查Server概览队列统计来源/聚合口径，不能以UI/MCP概览0代替直接broker门禁。test-ai证据.cache/fb071/overview-with-backlog.json、queue-head-metadata.log；本轮未改UI/DTO。 |
| FB-071 | test-ai | bug | open | 两端8GiB/2GiB、可靠性1/1保留。234447双故障实时恢复通过，完整生成每端64220 INSERT/39850 UPDATE；最终因FB-075/076失败，压力延迟另有FB-077。两端wait_free增量0，不宣称唯一根因。 | 00:14:14自动恢复passed，原配置/规则hash核验一致，正常Agent74960/6832，awake33392已released、正常broker队列0。无新长测；先处理FB-076/077，FB-072/073仍open。 |
| FB-070 | test-ai | bug | open | v0.46.10双端正式安装及Apply覆盖索引迁移通过；同一保留数据计数不变，中心64-96ms/边缘232-266ms。184033中16组Apply统计最慢中心1831.774ms/Edge281.926ms，无metrics超时。 | 本轮因另一生命周期/恢复延迟问题FB-071失败，未满30分钟；索引改进已验证但完整负载门禁尚未完成。保留全部旧失败证据。 |
| FB-069 | test-ai | bug | closed | 测试夹具现读取32/64位安装登记并拒绝冲突；32位NSIS登记不再误报未安装。 | 32位/64位/重复/冲突/空登记及原监督器回归通过；180024实装预检确认双端注册0.46.9、正式路径、运行实例和包hash一致。不改产品或安装包。 |
| FB-068 | test-ai | test | blocked | 双端正式v0.46.10配置保留、MCP33工具/诊断、标准migrate和前置功能通过；184033生命周期超时，192354四路对比中心恢复超时，完整30分钟未通过。 | 阻塞转FB-071，所有失败不追认。原现场和中心一路配置恢复，formal-ready仍失效，新七小时未启动。 |
| FB-067 | test-ai | task | closed | 用户授权的backend-ai实现已完成：只读限时MCP MySQL诊断、非唯一覆盖索引和标准migrate升级、包内schema，不改同步/ACK语义。隔离8万条4KB payload失败计数2310.761ms -> 0.826ms，80条失败及总数不变；最终包CLI独立工作目录迁移通过。 | v0.46.9最终包SHA 6C26E6D397C36238A7DEDCBB246D0EC39368F23C40D741F04917B9F6E699514F；全量test/vet/lint、Wails/契约、41解包hash、5资产、双覆盖含schema、32/64位回归、MCP0.46.9/33工具及新诊断通过。实装交FB-068，负载归因FB-066；无Wails新binding/UI需求。 |
| FB-066 | test-ai | bug | open | 原正式长测约21分钟snapshot超时，原始失败缺逐SQL诊断，唯一根因未证明。v0.46.10已在双端正式安装并标准迁移事件/Apply覆盖索引，同一保留数据计数不变。 | 15秒采样门禁保留；后续短测未再出现同类采样超时，但受FB-071恢复积压阻塞，仍需完整30分钟实装准入。旧600秒与隔离基准不能代替；当前无长测，formal-ready失效。 |
| FB-065 | test-ai | task | closed | 临时SYSTEM_REQUIRED唤醒租约通过精确规则归属测试、真实20秒到期退出和160102恢复规则后释放；不写电源方案。 | 正式run租约PID10724已验证active及脚本hash，最长23:23:36，正常规则恢复后自动释放。证据随正式run保留；手动睡眠/关机不在保护范围。 |
| FB-064 | test-ai | decision | closed | 以源COMMIT时刻和固定峰值/更新重载阶段分组，压力样本保留；稳态P95<=2秒、全部P99<=5秒，样本下限40/25/3。160102共43组，稳态36/压力7，全部门禁通过。 | Edge/Server稳态P95=206.636/620.148ms；全部P95=403.629/1842.609ms，全部P99=808.016/2445.866ms。154525混算失败仍保留，未追认通过。 |
| FB-063 | test-ai | bug | closed | 修复PowerShell大小写变量冲突、UTF8/共享删除读取、清理错误隔离和Windows原子替换共享锁2秒有界重试；真实临时/持续锁及恢复隔离回归通过。 | 160102完整600.494秒completed，双端原配置/规则/路径自动恢复passed；151711/153921失败证据保留。 |
| FB-062 | backend-ai | task | closed | 连续不同目标主键UPDATE合并最多64键的publisher-confirm批次；同键/INSERT/DELETE/DDL保持确认屏障，选择性NACK、失败回退及严格主键约束不变。 | 全量test/vet/lint与同速率真实短测160102通过：每端21420 INSERT/13290 UPDATE，5次生命周期、两次故障、峰值及最终全50列/账本通过。长期性能留FB-055。 |
| FB-061 | backend-ai | bug | closed | Edge常驻Canal与canal-publish-once正确装配apply-log依赖，构造器拒绝nil；修复Server自写行经Edge回传的问题。 | v0.46.8最终包bootstrap、两次所有权交接和11176次在线版本观察通过；143533失败保留，最终证据160102。 |
| FB-060 | backend-ai | decision | closed | 按用户授权完成backend-ai产品修复，再切换test-ai补夹具、打包和真实准入；无新增Wails DTO/UI功能。 | v0.46.8包、完整600秒短测与formal-ready通过；正式wide7h_0909_161239已启动并经两次相隔78秒快照确认推进。正式七小时结论转FB-055；旧失败与642条隔离消息保留。 |
| FB-059 | backend-ai | bug | closed | Apply单条/批量/compact写入当前回放标记并兼容列映射，配合FB-061日志依赖修复双向回环与版本回退；业务写入和日志同事务，不改转发源事件。 | 160102六张50列表、每端13290真实UPDATE、完整事务oracle、精确apply/SUCCESS账本、两次同键交接及11176次在线观察通过；旧134654版本回退失败保留。 |
| FB-058 | backend-ai | bug | closed | SyncEvent/ChangeEvent统一无损JSON数字解码，Canal字符串sync_version正确标准化；BIGINT和DECIMAL不再经float64丢精度。 | v0.46.8包内初/晚期9007199254740993与精确DECIMAL探针、真实Canal边界和全部50列通过；原0.46.7失败证据保留。 |
| FB-057 | backend-ai | bug | closed | MCP与UI overview共用buildinfo.Version=0.46.8，删除旧硬编码版本。 | 最终包MCP initialize/overview=0.46.8、32工具、headless及NSIS版本验证通过；用户安装目录未永久升级，本轮临时运行包内Agent。 |
| FB-056 | test-ai | test | closed | 原边缘机 v0.46.7 实包 hash、注册版本和安装摘要 8 步通过；MySQL、本地/中心 RabbitMQ、MCP 32 工具可用。初始 Agent 未启动，已通过安装版 MCP 启动为 PID 1516，并确认 SSH 断开后驻留。20/20 真实上传成功，P50 94.219ms、P95 126.016ms，数据与 apply/SUCCESS 日志各 20，错误/失败 ACK 为 0。 | 原规则未改，队列为空；报告 docs/v0.46.7-installed-connectivity-20260909.md。中心仍 v0.46.6，七小时测试仍未启动（FB-055）；版本文字问题转 FB-057。 |
| FB-055 | test-ai | test | blocked | wide7h_0909_161239约21分钟因snapshot超过15秒失败，16:34:26自动恢复；每端24980 INSERT/12490 UPDATE，流水行数对齐但最终50列未核验。 | formal-ready已撤销并验证Start拒绝，退化转FB-066；原规则/配置/Agent恢复passed。报告docs/v0.46.8-seven-hour-interruption-20260909.md；不重开或宣称七小时通过。 |
| FB-054 | backend-ai | bug | closed | 批内幂等已修复，严格业务 INSERT 保留；全量 test/vet/lint 通过。v0.46.7 包内同一事件 4+4 副本真实消费后业务/apply/SUCCESS 各 1，队列为空。安装包 hash F8EB939E1E97956A122ADBA77E27CB0397A9FA6AFF711E9F941BFBCB499161D4，32/64 位各 15 回归、隔离双覆盖、39 文件解包 hash 核验通过。无 Wails DTO/UI 变化。 | test-ai 跨身份修正封包夹具的命名参数转发及版本元数据，理由为 NoBuild 被误当资产目录；作用域仅打包链路。最终包与限制见 docs/v0.46.7-package-validation-20260909.md；后续宽表长测转 FB-055。 |
| FB-053 | test-ai | test | closed | 最终短测 `fb052-final-20260909` 在重复事件探针失败，双向各写入 16,002，8 次 CRUD、两端 Agent 停机和 ADD/DROP 已执行；未到最终摘要与全场景门禁，不能宣称通过。原规则与 Agent 已恢复。 | 失败根因转 FB-054 批内幂等；review-ai 已确认选择性 NACK 保序问题修复。后续长测按用户最新要求改为 7 小时、真实 UPDATE、50 列宽表，先制作安装包再设计，暂不启动。 |
| FB-052 | backend-ai | bug | closed | Server Canal 批量日志、连续 append_only 发布、64 条 confirm 窗口及失败源回滚已实现；CRUD/DDL 逐事件确认屏障与选择性 NACK 回归已通过 review-ai 复核。最终独立追加写分发段空表约 513.21/s、百万历史约 306.73/s，对照提升约 14.5/9.0 倍；不是端到端长期指标。无 Wails DTO/UI 变化。 | 纳入 v0.46.7；短测另发现的批内幂等由 FB-054 处理，长期性能验收留待新 7 小时测试，不把旧失败长测改写为通过。 |
| FB-051 | test-ai | test | closed | 正式双向长测 `overnight-20260908-184000` 在约 3 小时 3 分后失败：Edge -> Server `388,401/388,401`，Server -> Edge `388,401/204,484`，首次 Edge Agent 停机恢复 10 分钟内未追平，后续正式场景未执行。 | 编排器自动恢复原规则和双端 Agent；检查时清理 9 条精确闭合的测试下行残留，恢复探针约 1.50 秒通过。失败结论和吞吐证据见 `docs/v0.46.6-real-bidirectional-overnight-report-20260909.md`，性能缺陷转 FB-052。 |
| FB-050 | test-ai | test | closed | `192.168.10.105` 的 v0.46.6 正式安装与异机验收完成：注册版本、8 步安装摘要、包内 SyncAgent hash、MCP 32 工具、配置/规则保留、MySQL 与本地/中心 RabbitMQ、安装路径 Agent 启动及 SSH 断开驻留均通过。 | 安装版 20 条隔离单行 Edge -> Server 探针：min 48.015ms、P50 94.728ms、P95 127.240ms、max 195.698ms、avg 98.403ms；源/目标/apply/SUCCESS event 各 20，双端队列与错误为 0。 |
| FB-049 | test-ai | bug | closed | v0.46.5 单行真实链路延迟 5.13-11.44 秒。已修复 `retry_interval_seconds=10` 被错误复用为空闲轮询，以及 Canal 1000ms 长轮询两个等待点；正常 Worker 轮询和 Canal 长轮询均为 100ms，错误退避语义不变。无 DTO/UI 变更。 | v0.46.6 正式包内二进制 20 条隔离探针：min 62.569ms、P50 79.745ms、P95 141.854ms、max 142.122ms、avg 91.896ms；源/目标/apply/SUCCESS event 各 20，双端队列与错误为 0。详见 `docs/v0.46.6-real-latency-report-20260908.md`。 |
| FB-048 | test-ai | test | closed | v0.46.5 异机 Edge -> 本机模拟 Server 高压力测试完成：在线递增 1,000/10,000/50,000 行，Server 停机积压恢复 20,000 行。 | 四档源表、中心表、apply/event log 精确 81,000；50,000 行 110.567 秒排空，20,000 停机积压 35.216 秒恢复。最终队列、失败 ACK、错误和失败事件均为 0，详见 `docs/v0.46.5-real-stress-report-20260908.md`。 |
| FB-047 | test-ai | release | closed | v0.46.5 覆盖升级及真实链路验收完成：异机安装摘要通过，配置/身份/规则保留，远端二进制 hash 与 staging 一致并暴露 32 工具；Edge 和本机模拟 Server 均运行 v0.46.5。 | 远端 Agent PID `7324`，本机 Server PID `9688`；两条探针分别在 5.13 秒和中心重启后 11.44 秒到达，最终队列、失败 ACK、近期错误和边缘失败事件均为 0。 |
| FB-046 | frontend-ai | task | open | 后端和本机三节点测试已完成数据库治理与列结构同步：MCP 32 工具含结构化查询、受控数据写入和加列/删列；Canal 仅自动同步 `ADD COLUMN` / `DROP COLUMN`，绝不自动建表/删表。跨身份原因：`SyncRule.schema_sync` 已加入稳定 DTO，需要 Rules UI 显式呈现。 | frontend-ai 后续在 Rules 页补 `schema_sync.add_columns/drop_columns` 三语开关与删列风险提示；后端门禁要求非 `SERVER_TO_EDGE` 删列规则恰好一个 `source_node_id`。 |
| FB-045 | backend-ai | bug | closed | Schema 演进 E2E 发现 `append_only INSERT IGNORE` 会把超宽值静默截断后 ACK；已改为普通 `INSERT`，重放幂等继续由 `sync_apply_log.event_id` 保证，业务约束错误不再被吞掉。 | 单元测试和真实窄列 E2E 通过：目标过窄时 row/apply log 为 0、入口队列为 1；扩列后完整原值恢复。既有 v0.46.4 安装包不含本修复，外部分发前须生成下一版本安装包。 |
| FB-044 | test-ai | test | closed | 真实业务 Schema 演进 E2E 已覆盖运行中建表、加列、删列、删表重建、源/目标结构不一致、列排除、普通列/主键列映射、CRUD、跨 DDL 混合批次和失败恢复。 | 22 项核心场景结果、生产迁移顺序与剩余覆盖见 `docs/schema-evolution-e2e-report-20260908.md`；测试规则已移除，测试表保留为只读证据。 |
| FB-038 | backend-ai | release | closed | v0.46.3 内网安装包：新装使用无节点/MySQL/同步规则预设的空白引导配置；NodeBridge 解锁、退出和自有 RabbitMQ 密码统一为 `1234`；托管 RabbitMQ 账号按 `mode/node.id` 生成并在配置落盘前同步修改服务用户；升级迁移旧自有密码但保留 MySQL、Canal 和已有规则；UI 脱敏密码测试及 MCP 保存后可选重启已修复。 | 最终包已被 FB-039 的覆盖安装修订版取代；原包不得继续分发。 |
| FB-039 | backend-ai | test | closed | v0.46.3 覆盖安装及其 v0.46.4 后续修订均通过本机真实两轮 NSIS 覆盖、运行中进程停止、二进制更新、配置/规则保留和异机安装验证。 | v0.46.4 覆盖证据 `.cache/nsis-upgrade/20260908-103134-154/upgrade-evidence.json`；15 项回归证据 `.cache/installer-regression/74fced7aa7a54c689fe31bcf6784fd2d/`。边缘机安装摘要版本、步骤和退出码均通过。 |
| FB-040 | test-ai | test | closed | 本机模拟中心 `192.168.10.103` 与边缘 `192.168.10.105` 的真实 MySQL -> Canal -> RabbitMQ -> Apply -> MySQL 链路已通过。 | 单条初测约 25.91 秒，10 条批量约 16.83 秒；v0.46.4 安装版最终探针约 8.32 秒。中心队列 ready/unacked、失败 ACK 和近 10 分钟同步错误均为 0。 |
| FB-041 | backend-ai | bug | closed | 修复受管 Canal 配置目录与 destination 不一致：统一使用规范化节点 ID，写入 `canal/conf/<destination>/instance.properties`，激活唯一 destination 并清理旧根目录实例。 | 单元测试、全量门禁、安装器回归及真实边缘自动 CDC 传输通过；纳入 v0.46.4。 |
| FB-042 | backend-ai | bug | closed | Windows 后台 SyncAgent 使用新进程组并脱离 OpenSSH job，SSH stdio MCP 会话结束后 Agent 保持运行。 | 远端临时二进制和 v0.46.4 安装版均实测通过；无前端 DTO 变化。 |
| FB-043 | backend-ai | bug | closed | Edge 迁移补齐 `sync_ack_log`，新增 schema 契约测试；`nodebridge_failed_events` 在边缘节点可正常返回空列表。 | 已在目标边缘库迁移并由 v0.46.4 全量门禁和安装版 MCP 复测通过；无前端 DTO 变化。 |
| FB-037 | backend-ai | release | closed | 发布文档：补充一主多边缘部署、Windows/Mac SSH 公钥授权与 MCP 配置手册，更新根 README，并提交、推送、创建 GitHub Release。 | 已发布 v0.46.3 prerelease：`https://github.com/YufeiSun5/NodeBridge/releases/tag/v0.46.3`。资产大小和 SHA256 digest 已通过 GitHub API 与本地文件核对。 |
| FB-036 | backend-ai | bug | closed | v0.46.1 安装后普通运行的 `admin\xx` 仅有 ProgramData/NodeBridge Write 权限，原子替换 config.yaml 因缺少 delete-child 权限返回 Access denied。已在目标机按 SID 授予 Modify 并验证替换；安装器新增当前安装账户继承 Modify ACL。 | 目标机 MCP SSH 真实握手、26 工具、overview 和 save_config_patch 通过；Windows Codex 已全局注册。最终 v0.46.2 包 `build/NodeBridge-beta-v0.46.2-20260907.exe`，SHA256 `620208E9B275EAFA1DA2C856C1BFE774F996A5A00FD402DAC6304A7EC3D6BA14`，32/64 位各 13 项回归、解包、资产和 preflight 通过。Mac 私钥仍须在 Mac 本机生成后追加公钥。 |
| FB-035 | backend-ai | bug | closed | v0.46.1 修复 NSIS 32 位 PowerShell 漏查 x64 Erlang/RabbitMQ：Sysnative 启动、ProgramW6432 检测、原生参数转义、进程句柄保留与安装后探测；日志按次写到独立 ProgramData/NodeBridgeInstallerLogs。无前端 API 变更。 | 32/64 位各 12 项安装器回归、Go test/vet/lint、五个资产 hash、包内 MCP 26 工具 smoke 通过。交付 build/NodeBridge-beta-v0.46.1-20260907.exe；真实异机安装/卸载与 SSH 仍待 FB-034 验收，未在宿主机运行系统组件安装。 |
| FB-033 | review-ai | task | closed | Windows 第二任务：按用户授权交付 MCP v0.46 全配置实验室版。26 个工具覆盖全部配置字段/凭据/规则/自启动、真实诊断、重试、拓扑和同步进程；SSH stdio 支持 Windows/Mac 客户端，保留加密/校验/脱敏。 | 已修复协议错误、日志 limit、缺失 rules 清空风险、并发 patch 覆盖、跨进程 Agent 识别/独占锁和原子落盘；go test/vet/linter、Wails 构建、包内 EXE/中文/空格路径 smoke 通过。安装包 build/NodeBridge-beta-v0.46.0-20260907.exe，SHA256 0017615A0EB455A2DD8920075775B0913077412480D868AC8FD2DC3D518BB5B8。真实异机验收交给 test-ai FB-034。 |
| FB-034 | test-ai | test | open | MCP v0.46.4 Windows 内网异机验收完成：SSH 公钥连接、initialize、27 工具、配置/规则、MySQL/RabbitMQ 探测、Agent 启停与断开后驻留、真实数据链路均通过。 | Windows 部分已闭环；仅剩在 Mac 本机生成私钥、追加公钥并验证 SSH MCP 重连。 |
| FB-001 | backend-ai | question | closed | 前端需要显式配置状态，避免长期通过空 `mode/node.id/mysql.database` 推断首次配置。 | 已在 `GetOverview` 增加 `config_loaded/config_path/rules_path/node_id/node_name`。 |
| FB-002 | backend-ai | question | closed | Wails 绑定生成 `Not found: time.Time` 警告。 | Wails UI DTO 时间字段已改为 RFC3339 string。 |
| FB-003 | backend-ai | question | closed | 安装后 `SyncAgent.exe` 固定查找路径与优雅 shutdown 协议未冻结。 | 已冻结查找顺序并实现 stop-file 优雅停止；前端只展示 `GetAgentProcessStatus()` 和操作结果。 |
| FB-004 | frontend-ai | decision | closed | Rules UI 需要展示并编辑 `dispatch_target` 和 `dispatch_node_ids`，用于配置单向汇总、多主 fanout、指定节点下发。 | 当前 Rules 页已显示/编辑这两个字段，后续只需按 Frontend V0.26 Plan 做 exe 级验收。 |
| FB-005 | backend-ai | bug | closed | Overview 缺少显式 `config_loaded/config_path/node_id/node_name`，前端只能靠 `GetConfig` 拼数据，exe 首次启动容易显示 unknown。 | 已并入 FB-001 的 `GetOverview` 字段扩展。 |
| FB-006 | backend-ai | bug | closed | CDC 状态在 `GetOverview` 中没有真实探测逻辑，配置存在时仍长期显示 `unknown`。 | 后端按 `cdc.type` 返回 `configured/running/error/unknown`，Agent 运行时返回 `running`。 |
| FB-007 | backend-ai | bug | closed | `GetSyncRules` 在 exe 双击启动时依赖相对路径 `configs/sync-rules.example.yaml`，可能回退到内置 2 条默认规则，导致现场规则不显示。 | 后端改为优先现场默认规则，并把 fallback 规则落盘到 `sync-rules.yaml`。 |
| FB-008 | backend-ai | bug | closed | Logs 页只读取 Wails UI 进程 ring buffer，不读取外部 `SyncAgent.exe` 日志；启动外部 agent 后同步记录仍可能为空。 | 后端将外部 SyncAgent stdout/stderr 写入 `logs/sync-agent.log`，`GetLogs` 合并读取。 |
| FB-009 | backend-ai | design | closed | 需要兼容 MCP/ClaudeCode 对 NodeBridge 的自动化操作，但不能让 MCP 直接绕过 Wails 管理鉴权、配置校验或同步边界。 | V0.31 已提供只读 `mcp-stdio` alpha，不开放写配置、RabbitMQ mutation 或 MySQL 写入。 |
| FB-010 | frontend-ai | task | closed | Settings 页需要预留 MCP Server 开关，默认关闭；前端不得直接启动 MCP runtime。 | 已接入 `GetMCPServerStatus` / `SetMCPServerEnabled`，未解锁时先触发管理解锁，并将 `configured` 显示为“已启用但运行时未开放”。 |
| FB-011 | backend-ai | question | closed | Rules 页 `dispatch_node_ids` 需要选择 ACTIVE Edge 目标节点，但当前 Wails contract 没有节点列表/候选项接口。 | 后端已新增只读 `GetNodeOptions()`，返回 ACTIVE Edge 候选和稳定 empty/error 状态。 |
| FB-012 | frontend-ai | task | closed | V0.27 新增 `GetAgentProcessStatus()`；前端需要展示 SyncAgent 路径、PID、日志路径、`stopped/running/exited/error/forced_stopped` 状态。 | Overview 已读取 `GetAgentProcessStatus()` 并展示路径、PID、启动/退出时间、日志路径、最后错误和状态。 |
| FB-013 | frontend-ai | task | closed | `CDCConfig` 新增 `mode/install/config_dir/service_name`，用于 Canal managed/external 安装边界配置。 | Config 页已展示并编辑这些字段；后续可优化 external 模式说明文案。 |
| FB-014 | frontend-ai | task | closed | V0.30 新增 `RetryFailedEvents` 和 `GetDeadLetters`；Failures 页需要批量重试和死信只读预览入口。 | Failures 页已接入批量重试和死信只读预览，操作前均要求管理解锁。 |
| FB-015 | frontend-ai | task | closed | V0.31 新增 `GetManagedInstallPlan` / `ApplyManagedInstall`；Settings 或 Config 页后续需要展示受管组件计划和执行结果。 | Settings 页已展示受管组件计划、manifest 路径和 operations；执行入口先管理解锁，并明确 alpha 资源边界。 |
| FB-016 | test-ai | task | closed | 测试 AI 需要在 WinServer2022 Core / 无 GUI VM 中验证后端交付的 headless installer test bundle。 | V0.38.3 已在 `Clean-Windows-Installed` 干净 VM 通过闭环：第一轮 `-ExecuteInstall` 通过，`-VerifyOnly` 通过，二次 `-ExecuteInstall` 通过，`-Uninstall` 通过且 `verify-uninstall` passed；卸载后确认无 RabbitMQ/NodeBridge/Canal 服务。缺 Java 场景按预期跳过 Canal Service。证据在 `.cache/v0.38.3-clean-validation/`。 |
| FB-017 | frontend-ai | task | closed | Rules 页需要把 `GetNodeOptions()` 接入 `dispatch_node_ids`，用 ACTIVE Edge 候选替代纯手填，并保留手填兜底。 | Rules 编辑态已接入 ACTIVE Edge 候选勾选区，显示 empty/error 状态和三语提示，同时保留手填节点 ID 兜底。 |
| FB-018 | frontend-ai | task | closed | Settings/通知设置中的 MCP Service 开关语义调整：默认关闭，只在本次 NodeBridge 会话生效，重启后自动关闭。 | 前端已更新为本次会话 stdio 临时开关；未加载配置时禁用开关并提示先保存同步配置，不再显示“保存开关”或“运行时未开放”。 |
| FB-019 | backend-ai | question | closed | 管理密码/退出密码忘记后的正式恢复流程需要产品化，不能让前端绕过 DPAPI 或管理鉴权。 | 产品决策已闭合：不提供后端找回或自动重置 CLI；用户自行退出 NodeBridge、备份 `%ProgramData%\NodeBridge\config.yaml`，再清空 `security.admin_password` 或 `security.exit_password` 的加密值后重启并重新设置。 |
| FB-020 | backend-ai | bug | closed | 首次使用只设置管理密码不会创建 `%ProgramData%\NodeBridge\config.yaml`，导致用户以为密码已设置但后端仍未加载配置，MCP/受管组件等依赖配置的功能失败。 | 后端已允许 `SaveConfig` 在仅含 `security` 的首次引导场景写入加密安全草稿；完整同步配置仍继续要求 `mode/node.id/mysql.database/security.admin_password` 校验。前端无需新增方法，继续调用 `SaveConfig`。 |
| FB-021 | backend-ai | bug | closed | 用户已在 UI 启动并开启 MCP 模式，但按生成的 MCP client config 运行 `SyncAgent.exe mcp-stdio -config %ProgramData%\NodeBridge\config.yaml -rules %ProgramData%\NodeBridge\sync-rules.yaml` 失败。test-ai 复测当前实际 `%ProgramData%` 安全草稿配置，`SyncAgent.exe mcp-stdio` 仍 exit 1，最新错误为 `decrypt admin password: Key not valid for use in specified state`；示例完整配置下 MCP stdio 协议本身通过，说明阻塞仍在 Wails MCP 开关/状态、DPAPI 安全草稿和 SyncAgent MCP 启动前置条件不一致。 | 后端已让 `GetMCPServerStatus` / `SetMCPServerEnabled` 在配置无法严格加载、配置不完整或 DPAPI 解密失败时返回 `unsupported`，并拒绝启用；完整配置下 MCP stdio 仍为只读。 |
| FB-022 | backend-ai | change | closed | 用户需求变更：MCP Server 模式不应随管理解锁过期关闭，也不应因 NodeBridge 重启自动关闭；启用后只允许用户手动关闭。 | 后端已取消启动/保存强制清零，`mcp_server.enable` 持久化到 YAML；`GetMCPServerStatus` 返回 `ephemeral=false/restart_resets=false`，启用后只由用户手动关闭。 |
| FB-023 | frontend-ai | task | closed | 后端已把 MCP 开关语义从“本次会话临时启用、重启关闭”改为“持久启用、只允许用户手动关闭”。 | 前端已更新 Settings/说明书三语文案：移除 session-only/restart reset 提示；当后端返回 `unsupported` 时展示“同步配置未完成或当前用户无法解密配置，不能启用 MCP”。 |
| FB-024 | test-ai | bug | closed | V0.40.5 已在 `Clean-Windows-Installed` 干净 VM 通过完整闭环：第一轮 `-ExecuteInstall -RequireCanalService` 通过，首次 `-VerifyOnly` 通过，二次 `-ExecuteInstall -RequireCanalService` 通过，二次 `-VerifyOnly` 通过，首次 `-Uninstall` 通过，二次 `-Uninstall` 通过且 `verify-uninstall` passed。卸载后确认无 RabbitMQ/NodeBridge/Canal 服务；已回收 `NodeBridgeCanal-delete-wait.json` / `NodeBridgeRabbitMQ-delete-wait.json`。证据在 `.cache/v0.40.5-clean-validation/`，宿主机 Erlang/RabbitMQ/Canal 未触碰。 | 无后续动作。 |
| FB-025 | test-ai | bug | open | 90 天等价长测第一轮：单机 Docker 模拟 Edge/Server，两侧 Docker MySQL/RabbitMQ，Edge Docker Canal，真实 CDC，6 张采集表 x 42 点位字段，目标 15,552,000 行，并覆盖 24h 实时、90d 等价灌入、1h/1d/7d 断线恢复、查询性能和滚动归档。 | backend-ai 已新增按表 `sync_mode`：默认 `crud_ordered` 保持增删改保序，`append_only` 用于历史倾倒/采集流水表，只接受 INSERT；Server Apply 对 append-only 批次按目标表分组多行插入，CanalUploadRuntime 整批 `PublishBatch` 后才提交 Canal offset。`appendonly-month30-004` 因 MySQL prepared statement 占位符上限失败，已修复为批量 SQL 自动切片并保留证据；新 30 天量级复测 `appendonly-month30-005` 正在后台 drain，证据目录 `.cache/longtest-90d/appendonly-month30-005/`。2026-05-31 05:54:52 快照：Edge=5,184,000，Server=1,880,000，Edge upload=1,154,000，Server ingress=2,160,000，dead/retry=0，runner/watchdog/agents 均存活，测试未完成。 |
| FB-026 | frontend-ai | task | closed | 前端需要联调 `sync_mode` 规则字段：新增规则默认 `crud_ordered`，编辑/保存不丢字段，`append_only` 需要明显危险提示和三语文案。 | Rules DTO、下拉、默认值和 Manual 已接入；本轮补强 `append_only` 危险提示，确认 Wails contract、TypeScript、Vite build 通过。 |
| FB-027 | test-ai | test | closed | 15 天混合断网压力测：5 张 `tag_state_*` 使用 `crud_ordered` 保序更新，12 张 `collect_data_*` 使用 `append_only` 历史倾倒，断网期间 Edge 软件保持运行。 | 已补测尾段清理，`mixed-15d-offline-002` `recovery-summary.json` 最终 `status=completed`，`server.cdc.ingress_depth=0`，`server_apply_log=7,349,000`，`server_event_log=7,349,000`，尾段一致通过。 |
| FB-028 | test-ai | performance | open | Server Apply 性能成倍提升计划：当前混合恢复证明 Edge upload 可清空，瓶颈集中在 Server apply；`crud_ordered` 路径仍接近“每事件业务 SQL + apply_log + savepoint”，append-only 在混合队列中被保序事件拖慢。 | backend-ai 已完成 P2/P3/P4 后端阶段：`append_only` 在 batch/segment 内按目标表批量写入；Server Apply 支持 `sync.apply_lanes` 按 `target_table + pk` 分 lane 并行，同一主键同 lane 保序；新增 `crud_ordered_compact`，只合并同一主键连续 UPDATE 的中间状态，不跨 INSERT/DELETE 边界。按用户要求 compact 需要双开关：Settings 的 `sync.enable_crud_compact=true` + 单条规则 `sync_mode=crud_ordered_compact`，否则后端拒绝执行；不允许一键开启全部。已通过 `go test ./...`、`go vet ./...`，`golangci-lint` 未安装跳过，已重建 `build/bin/SyncAgent.exe`。下一步 test-ai 用新二进制重启/重跑 `mixed-15d-offline-002` 恢复 drain，对比吞吐和顺序/幂等。 |
| FB-029 | frontend-ai | task | closed | P4 compact 前端配合：Settings 增加“启用 CRUD compact 性能模式”开关，默认关闭；Rules 单条规则增加 `crud_ordered_compact` 选项。 | Settings 已接入 `sync.enable_crud_compact` 全局开关并要求管理解锁；Rules 已增加单条 `crud_ordered_compact` 选项，未开启全局开关时禁用新选择；无一键全开入口；三语说明已写明只合并同一主键连续 UPDATE 中间状态且不跨 INSERT/DELETE。 |
| FB-030 | frontend-ai | task | closed | MCP 设置与说明书需要按新目标更新：MCP 的目的不是开放网页端口，而是让操作员在边缘机无屏幕或屏幕被拿走时，用另一台电脑上的 AI 客户端受控查看并修改本机 NodeBridge 配置。 | Settings/Manual/`docs/mcp-service.md` 已更新：MCP 默认关闭，启用需管理解锁和完整同步配置，传输为 `stdio`，用户在远程 AI 工具配置 `mcp-client-config`，不开放 HTTP 端口，不宣传自动远程控制；前端只展示后端状态和配置命令，不启动 MCP runtime。 |
| FB-031 | backend-ai | design | closed | MCP 从只读诊断升级为“受控配置修改”需要后端设计：允许远程 AI 修改配置，但必须沿用 Wails 后端同等校验、脱敏、DPAPI/密钥边界和审计记录，不能让 MCP 任意写 YAML 或绕过管理鉴权。 | 后端已实现白名单写配置能力：`nodebridge_validate_config_patch` dry-run、`nodebridge_save_config_patch` 保存非敏感 patch、`nodebridge_save_sync_rules` 保存规则；配置 patch 拒绝 `password/token/security`、原始 YAML 和带密码 AMQP URL；读写从磁盘取最新配置/规则；成功写和拒绝写均记录 `logs/mcp-audit.log`；工具 schema 已明确 `patch/rules` 输入。`mcp-stdio` 启动强制要求 `mcp_server.enable=true`。已更新 `docs/mcp-service.md` 和 contract，测试/linter 通过并重建 `build/bin/SyncAgent.exe`。 |
| FB-032 | test-ai | task | open | 制作 NSIS beta 安装器：把已通过 V0.40.5 的 headless installer with-assets 链路包装为可双击的一键安装程序，覆盖 Erlang/RabbitMQ/Java/Canal/WinSW、NodeBridge/SyncAgent、安装前检查、安装日志、失败摘要和卸载入口。跨身份原因：这是安装器/发布工程任务，完成后必须移交 test-ai 做 VM 和本机 Docker smoke。 | backend-ai 已按实机反馈继续修复：快捷方式显式使用 `app/NodeBridge.ico`；管理端仍由 `wails build -nopackage -o NodeBridge.exe` 生成；RabbitMQ 连通性拆为 local/server 探测，本地成功时远端 `192.168.1.10:5672` 不再导致整体同步配置失败；Config 页保存 MySQL/RabbitMQ/CDC/同步参数后提示重启同步进程生效，并新增 `初始化 RabbitMQ 队列` 入口调用受管拓扑初始化；默认配置改为安装器真实创建的 `nb-edge-001-local/nodebridge_test` 和 encoded vhost `/%2Fnodebridge-edge`；NSIS 安装时会备份并替换 `%ProgramData%\NodeBridge\config.yaml` 中安全草稿/不完整配置；headless component 调用已改为参数数组直接执行并落 stdout/stderr，修复 `C:\Program Files\...` 路径空格导致的 exit code `-196608`。重新生成最终 exe：`build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `947599EA3F2F2620EE0164A6F20E5918EFBD1661FB16176E05B38B5E2972ADD7`；staging `app/NodeBridge.exe`、`app/NodeBridge.ico`、`app/SyncAgent.exe` 存在且 `app/DataSync.exe` 不存在。本机仅编译 exe，未运行 NodeBridge beta 安装器，未执行 Erlang/RabbitMQ/Java/Canal/WinSW 真实系统组件安装。下一步 test-ai 对当前 hash 在隔离 VM 或测试机重跑安装、打开 UI、RabbitMQ 队列初始化、远端不可达提示、二次安装、卸载闭环，并回收 `runtime/*.json` / `*.log`。  2026-09-07 更新：后续安装验证改用 v0.46.0-20260907，hash 见 FB-033；MCP 异机验收见 FB-034。 |

## Frontend V0.26 Plan

1. Rules 页新增 `dispatch_target` 下拉框：`AUTO`、`NONE`、`ACTIVE_EDGES`、`SELECTED_EDGES`。
2. Rules 页新增 `dispatch_node_ids` 输入框；当 `dispatch_target=SELECTED_EDGES` 时显示必填提示。
3. Rules 只读表也要显示分发策略，不能只在编辑态显示。
4. Overview 直接使用 `GetOverview.node_id/node_name/config_loaded/config_path/rules_path`，不再靠 `GetConfig` 拼节点和配置状态。
5. Overview 显示 `cdc_status` 和 `cdc_message`；`configured` 表示已配置但 SyncAgent 未运行。
6. Logs 页继续调用 `GetLogs`；后端已合并 UI ring buffer 和外部 `SyncAgent.exe` 日志。
7. 时间字段按 RFC3339 string 处理，不再按 `time.Time` / `any` 推断。
8. 验收：`npm run build`、Wails contract check、搜索无 `fetch(`/`axios`，并用打包 exe 确认 Overview、Rules、Logs 不再无故空白或 unknown。

## Frontend Requirements Review Approval

后端已审阅并批准当前 `frontend-requirements.md` 作为前端 V0.26 实施范围，附带以下约束：

1. 前端只做 Wails 管理端，不实现同步核心逻辑。
2. Settings 可以加入 MCP Server 开关，但只能调用 Wails 后端接口；默认关闭，不自行启动服务。
3. 任何新字段或方法必须先落到 `frontend-backend-contract.md`，再改页面。
4. `unsupported`、`configured`、`locked`、`error` 必须如实展示，不得显示假成功。
5. Config、Rules、Retry、Diagnostic、AutoStart、Agent control、MCP Server 开关都必须要求管理解锁。

## Board Rules

1. 开工前先声明身份，再读 Active Board，只处理与本轮身份相关或已显式跨身份授权的 open 项。
2. 新问题必须加到 Active Board，分配合法 `Owner`，状态写 `open`。
3. 解决问题时，把对应行状态改为 `closed`，并在 Activity Log 追加一条 answer/decision。
4. 无法推进时，把状态改为 `blocked`，`Next Action` 写清楚缺什么。
5. API、DTO、错误语义、页面范围变化：先更新 Active Board，再同步 `.ai/docs/frontend-backend-contract.md` 或 `.ai/docs/frontend-requirements.md`。
6. 前端、后端、测试、审阅任一身份只改自己范围时，也要检查 Active Board。
7. `test-ai` 必须把验证命令、通过/失败结果和未跑原因写入最终回复；影响发布门禁时写入 Active Board。
8. `review-ai` 必须优先列出风险、缺口和阻塞；需要代码或文档修改时先声明是否临时跨身份执行。
9. 最终回复必须说明本轮身份、处理了哪些 Board 项、哪些仍 open/blocked。
10. 闭合项可保留在 Activity Log；当日志影响阅读时，由 `review-ai` 将闭合历史整理到 `.ai/docs/archive/` 或阶段总结，并保留根级 Active Board 的当前项。

## Activity Log Format

```text
- YYYY-MM-DD HH:mm | <frontend-ai/backend-ai/test-ai/review-ai> | <question/decision/answer/blocker/review/test> | <影响范围> | <open/closed/blocked>
```

## Activity Log

- 2026-09-11 11:10 | review-ai | plan | FB-101首次对齐及全功能MCP实施方案交付；用户要求可选非全自动，已定义默认关闭/手动批准/不隐式触发及T11。后端、前端、测试与业务边界确认分别登记FB-102至105。仅文档，无稳定DTO/运行代码/现场变更，不执行安装或长测。 | closed

- 2026-09-11 09:48 | backend-ai | answer | FB-095按先方案/报告/再修复交付，原表有界复现后确认日志UPSERT尾部锁放大，FB-096源码修复及真实双事务/回滚/重放/test/vet/lint通过。未出包部署、不追加长测；FB-094原超时与整链路验收open。无UI/DTO影响，现场已恢复。 | closed

- 2026-09-10 10:11 | test-ai | test | 093438完整1800.925秒及全部门禁/恢复通过，formal-ready绑定E61/WorkspaceServer及原配置。100650七小时10:07:37启动，实际runner33492/producer29224及相隔139.794秒增长快照验证；awake5388active。用户要求的稳定启动已满足，七小时最终结果仍open（FB-083），FB-072概览统计及FB-073纯空闲复核仍open。 | open

- 2026-09-10 09:36 | test-ai | test | FB-082边缘安装6348结束成功，解密全文/规则保留，新MCP启动16060、诊断2GiB通过；中心42184工作区E61/8GiB。093438前置功能通过、完整1800秒运行中，awake4940已核验active。FB-081实现关闭，压力及长测准入仍open。 | open

- 2026-09-10 09:31 | test-ai | decision | 从backend-ai接回FB-082，v0.46.12最终包门禁完成；中心不安装，边缘包已上传，配置/规则已备份。准备实际升级和完整短测，不将隔离性能收益宣称为P99通过。 | open

- 2026-09-10 09:20 | backend-ai | decision | 接手FB-081事件日志多行SQL；test-ai诊断回归及全量test/vet/lint已通过。100万历史基准在准备阶段因DSN30秒读超时失败，临时库已核验清除，不将其当产品失败；零历史完整混合/回放基准通过，证明日志往返成本，仍需完整真实负载验证。 | open

- 2026-09-10 09:12 | test-ai | test | 083751在最终延迟核验失败，账本及全部业务字段通过；两端wait_free/log_waits增量均0。09:07:09恢复passed，Agent39956/15820、规则hash及业务队列0核验，awake7644已released并退出。FB-077未解决转FB-080继续归因；无新长测。 | open

- 2026-09-10 08:40 | test-ai | decision | FB-079测试脚本新增显式WorkspaceServer模式及模式/路径绑定准入，Installed默认保留；越界/旧进程/hash/边缘注册/证明误用回归、原子写及全量test/vet/lint通过。FB-078解除安装阻塞；runner54264运行新1800秒，awake7644确认active，最长09:14:08或原规则恢复即释放。 | open

- 2026-09-09 20:50 | test-ai | diagnostic | 原一路202640恢复失败，首次取得同时间段缓存耗尽、SQL日志执行占用和Canal缓冲背压证据；全场景未完成，1570条隔离并恢复双端。等待MySQL内存对照确认。SQL观察器为只读测试代码；另修成功单元测试误用50ms期限，30次及全量test/vet/lint通过。 | blocked

- 2026-09-09 20:20 | test-ai | diagnostic | FB-071隔离50万历史宽UPDATE分发364.91/s，40 INSERT/20 UPDATE交错246.50/s，日志/顺序/50列通过；新增FB-073空闲位点反馈证据，产品与MySQL配置未改。全量test/vet/lint通过。 | running

- 2026-09-09 20:07 | test-ai | test | FB-071原一路195000完成600秒全门禁和恢复，P95 Edge404/Server625ms；不满足1800秒准入。缓存wait_free增量0；隔离驱动基准两模式均>490事件/秒，继续大历史对照，未改产品DSN或MySQL参数 | running

- 2026-09-09 19:50 | test-ai | test | FB-071四路192354中心恢复超时，36900 INSERT/18450 UPDATE每端；原现场与一路配置恢复，500条confirm隔离。补测试MySQL缓存/刷盘计数，下一轮10分钟只作诊断，不签发30分钟准入 | running

- 2026-09-09 17:32 | test-ai | answer | 接收backend-ai的FB-067只读MCP诊断、非唯一覆盖索引幂等迁移及包内schema，完成v0.46.9完整门禁；原始超时唯一根因不追认。FB-068等待双端正式安装，新硬门禁要求实装路径/版本/hash及至少30分钟准入，FB-066 open、FB-055 blocked | closed

- 2026-09-09 16:40 | test-ai | test | 检查发现正式run在16:34:10因snapshot超时失败，恢复passed；双向流水24980行、版本观察无回退但未做最终全量。撤销准入并实测Start拒绝，FB-055 blocked、FB-066 open；未改产品或重开长测 | blocked

- 2026-09-09 16:16 | test-ai | test | FB-057至065完成修复、最终v0.46.8包及真实准入：160102完整600.494秒、每端21420 INSERT/13290 UPDATE、50列/账本/版本观察/场景/延迟/恢复全部通过；43个样本的全部P95亦低于2秒，旧失败记录原样保留 | closed
- 2026-09-09 16:16 | test-ai | test | FB-055正式wide7h_0909_161239于16:13:22开始、计划23:13:22结束；runner72756/producer52756及双Agent存活，两次相隔78秒快照确认源/目标持续推进且无失败ACK/事件；临时唤醒PID10724生效，不改电源方案，不提前宣称七小时通过 | running

Activity Log 是历史流水，可能保留当时的 `open` 状态；当前真实待办以 Active Board 为准。

- 2026-05-26 16:52 | test-ai | test | FB-025 长测 harness 改为默认 time-interleaved 写入并重建 Docker volume 干净 smoke：18/18 同步、顺序/形态检查通过、队列清空 | open
- 2026-05-26 18:43 | test-ai | blocker | FB-025 1 天等价 day1-interleaved-001：Edge 172,800，Server 停止时 151,951，剩 server.cdc.ingress.q 20,850，失败 ACK 0；阻塞为 Server Apply 吞吐不足 | blocked
- 2026-05-26 22:16 | backend-ai | answer | FB-025 V0.41 完成保序批处理、无下发日志跳过大 payload，并用当前二进制跑通 6,000 行真实 CDC smoke | open
- 2026-05-27 00:42 | test-ai | blocker | FB-025 30 天等价 month30-v041-001：Edge 5,184,000 行生成成功，但 Edge SyncAgent 出现 Canal ACK panic，重启后队列清空而 Server 仅 5,780 行；apply/event log 与业务行数不一致 | blocked
- 2026-05-27 01:35 | backend-ai | answer | FB-025 修复稳定 CDC event_id、Canal ACK/offset 顺序、Server 失败重投和 longtest Canal 显式配置；6,000 行真实 CDC smoke 通过 | open
- 2026-05-27 08:17 | test-ai | test | FB-025 V0.42 `day1-v042-001` 1 天等价通过：Edge/Server 172,800/172,800，顺序 0 违规，队列清空，查询性能均达标；证据在 `.cache/longtest-90d/day1-v042-001/` | open
- 2026-05-27 08:18 | test-ai | blocker | FB-025 V0.42 `month30-v042-001` 30 天等价阻塞：Edge 5,184,000 行完整，Server 停在 69,360 行，apply/event log 均 69,360，失败 ACK 0，最终两侧队列 0；判定为大批量 CDC/offset 完整性问题，证据在 `.cache/longtest-90d/month30-v042-001/` | blocked
- 2026-05-27 09:06 | backend-ai | answer | FB-025 修复 Canal 空 batch 不提交导致 CDC 卡住的问题，补非 ROWDATA offset 保留和空 batch ACK 测试，6,000 行真实 CDC smoke 通过 | open
- 2026-05-27 09:18 | test-ai | blocker | FB-025 V0.43 `month30-v043-001` 30 天等价仍阻塞：Edge 5,184,000 行完整，Server 停在 46,240 行，apply/event log 均 46,240，失败 ACK 0，最终两侧队列 0；已回收 Canal `logs/conf/meta` 与分析文件到 `.cache/longtest-90d/month30-v043-001/` | blocked
- 2026-05-27 09:40 | backend-ai | answer | FB-025 修复 Canal fetch/commit 错误后不重连和客户端 idle timeout 过短，最终 6,000 行 smoke 通过 | open
- 2026-05-27 10:26 | test-ai | test | FB-025 V0.44 `month30-v044-001` 零/冒烟级回归通过旧失败点：Edge 5,184,000 行完整，Server 增至 116,908，越过 V0.43/V0.42 停点，失败 ACK 0；本轮主动停止，未作为 30d 全量验收 | open
- 2026-05-27 11:09 | test-ai | test | FB-025 分层阶段 0/1 通过：`staged-v044-p0-smoke-001` 12/12 通过；`staged-v044-p1-day1-001` Edge=Server=172,800、失败 ACK 0、队列清空、查询达标 | open
- 2026-05-27 11:29 | test-ai | test | FB-025 阶段 2 `staged-v044-p2-day7-001` 已启动但未验收：Edge=1,209,600，Server 暂到 57,066，失败 ACK 0，队列活跃；需无人值守跑满 7d 后才能进 30d | open
- 2026-05-28 08:52 | test-ai | blocker | FB-025 `staged-v044-month30-001` 30 天等价无人值守复测阻塞：Edge=5,184,000 已完成，Server=294,780 后超过 10 小时不增长，`edge.upload.cdc.q=4725`、`server.cdc.ingress.q=1056`，dead/retry=0，失败 ACK 0，`summary.json` 仍为 prepare；证据目录 `.cache/longtest-90d/staged-v044-month30-001/`，现场未清理，转 backend-ai 分析同步停止推进原因 | blocked
- 2026-05-28 08:39 | backend-ai | answer | FB-025 现场复核：runner/SyncAgent 已不在，RabbitMQ 两队列均 ready 且 consumers=0；手动 canal publish/forward/consume 可继续推进，判定主阻塞为长测 harness 静默退出且证据不足，需先修监督脚本再重跑 | blocked
- 2026-05-28 09:34 | test-ai | test | FB-025 已按 backend-ai 意见优化 longtest harness 并启动 30 天等价重测 `staged-v044-month30-rerun-002`：干净 Docker volume，runner PID=40072，证据目录 `.cache/longtest-90d/staged-v044-month30-rerun-002/`，新增 runner stdout/stderr/summary、agent command log、drain fail-fast 和失败证据回收 | open
- 2026-05-30 16:50 | test-ai | test | FB-025 `staged-v044-month30-rerun-002` 30 天等价 seed 通过：Edge/Server 六表均 864,000 行，总计 5,184,000/5,184,000，队列清空，`drain-final.completed=true`，runner summary passed，耗时约 36.99 小时；用户重启后容器为 Exited，但完成证据已写盘 | open
- 2026-05-30 17:07 | test-ai | test | FB-025 优化长测 drain 为 `DrainMode=agents` 常驻 SyncAgent 模式；核心覆盖率门禁 `scripts/test-coverage-core.ps1` 通过 74.9% >= 70%，全仓库原始覆盖率 53.8% 留证；优化后 smoke `smoke-agents-coverage-001` 通过 12/12；30 天积压复测 `staged-v044-month30-agents-001` 已后台启动 PID=33244 | open
- 2026-05-31 01:30 | backend-ai | answer | FB-025 针对 Edge 上传积压释放慢新增 RabbitMQ 批量 publisher confirm：整批顺序发布、整批 confirm 后 ACK，失败整批重投；已通过相关单测、全量测试、vet、核心覆盖率 75.1%，并重建 `build/bin/SyncAgent.exe`。当前后台 30d run 仍是旧进程，需新跑或重启后才体现优化 | open
- 2026-05-31 01:42 | backend-ai | test | FB-025 已在同一 30d 积压现场切换到新 SyncAgent：旧段 Server apply 约 30.9 rows/s，新段约 141.0 rows/s，Server 收到/落库综合速率约 188.0 msg/s，短窗提升约 4.56x-6.08x；新证据 `batchconfirm-samples.csv` / `batchconfirm-analysis.json`。当前新 agents PID=37132/7072 继续运行，瓶颈转到 Server MySQL apply | open
- 2026-05-31 01:54 | backend-ai | test | FB-025 已修复无人值守监控脚本 Windows/Docker 调用兼容问题并重启监控，PID=18336；采样写入 `.cache/longtest-90d/staged-v044-month30-agents-001/batchconfirm-watch.csv`，完成摘要写入 `batchconfirm-watch-summary.json`；当前 Server=1,431,885/5,184,000，agents 37132/7072 存活 | open
- 2026-05-31 01:58 | backend-ai | test | FB-025 巡检确认批量 confirm 后 30d 现场仍在推进：Server=1,451,885/5,184,000，Edge 队列约 565,874，Server ingress=10,000，agents 37132/7072 存活；证据 `manual-snapshot-20260531-015740.json`。后台监控脚本仍受 PowerShell native stderr 影响，当前以手动快照和直接行数核验为准 | open
- 2026-05-31 02:00 | backend-ai | test | FB-025 巡检：Server=1,471,885/5,184,000，Edge 队列约 565,792，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020008.json`，测试继续跑 | open
- 2026-05-31 02:01 | backend-ai | test | FB-025 巡检：Server=1,481,885/5,184,000，Edge 队列约 552,103，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020139.json`，测试继续跑 | open
- 2026-05-31 02:03 | backend-ai | test | FB-025 巡检：Server=1,491,885/5,184,000，Edge 队列约 548,734，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020319.json`，测试继续跑 | open
- 2026-05-31 02:04 | backend-ai | test | FB-025 巡检：Server=1,501,885/5,184,000，Edge 队列约 544,434，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020444.json`，测试继续跑 | open
- 2026-05-31 02:06 | backend-ai | test | FB-025 巡检：Server=1,511,885/5,184,000，Edge 队列约 540,182，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020609.json`，测试继续跑 | open
- 2026-05-31 02:08 | backend-ai | test | FB-025 巡检：Server=1,531,885/5,184,000，Edge 队列约 536,888，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020753.json`，测试继续跑 | open
- 2026-05-31 02:09 | backend-ai | test | FB-025 巡检：Server=1,541,885/5,184,000，Edge 队列约 533,249，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-020922.json`，测试继续跑 | open
- 2026-05-31 02:10 | backend-ai | test | FB-025 巡检：Server=1,551,885/5,184,000，Edge 队列约 528,780，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021043.json`，测试继续跑 | open
- 2026-05-31 02:12 | backend-ai | test | FB-025 巡检：Server=1,561,885/5,184,000，Edge 队列约 524,971，Server ingress=0，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021215.json`，测试继续跑 | open
- 2026-05-31 02:14 | backend-ai | test | FB-025 巡检：Server=1,571,885/5,184,000，Edge 队列约 511,860，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021355.json`，测试继续跑 | open
- 2026-05-31 02:15 | backend-ai | test | FB-025 巡检：Server=1,581,885/5,184,000，Edge 队列约 507,649，Server ingress=10,000，agents 37132/7072 存活，错误日志为空；证据 `manual-snapshot-20260531-021519.json`，测试继续跑 | open
- 2026-05-31 02:23 | backend-ai | test | FB-025 巡检：Server=1,641,885/5,184,000，Edge 队列约 480,817，Server ingress=10,000，agents 37132/7072 存活；新增并验证 `scripts/longtest-30d-watchdog.ps1`，后台 PID=11172，每 5 分钟写 `watchdog-progress.csv` / `watchdog-summary.json`，测试继续跑 | open
- 2026-05-31 02:25 | backend-ai | test | FB-025 巡检：Server=1,661,885/5,184,000，Edge 队列约 479,094，Server ingress=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-022540.json`，测试继续跑 | open
- 2026-05-31 02:28 | backend-ai | test | FB-025 watchdog 正常采样：Server=1,681,885/5,184,000，Edge 队列约 461,409，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；`watchdog-progress.csv` / `watchdog-summary.json` 已更新，测试继续跑 | open
- 2026-05-31 02:30 | backend-ai | test | FB-025 直连快照：Server=1,701,885/5,184,000，Edge 队列约 459,304，Server ingress=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023039.json`，测试继续跑 | open
- 2026-05-31 02:32 | backend-ai | test | FB-025 直连快照：Server=1,711,885/5,184,000，Edge 队列约 456,368，Server ingress=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023218.json`，测试继续跑 | open
- 2026-05-31 02:33 | backend-ai | test | FB-025 直连快照：Server=1,721,885/5,184,000，Edge 队列约 442,512，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023349.json`，测试继续跑 | open
- 2026-05-31 02:35 | backend-ai | test | FB-025 直连快照：Server=1,731,885/5,184,000，Edge 队列约 438,674，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023522.json`，测试继续跑 | open
- 2026-05-31 02:38 | backend-ai | test | FB-025 直连快照：Server=1,751,885/5,184,000，Edge 队列约 430,289，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023814.json`，测试继续跑 | open
- 2026-05-31 02:39 | backend-ai | test | FB-025 直连快照：Server=1,771,885/5,184,000，Edge 队列约 426,667，Server ingress=10,000，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-023953.json`，测试继续跑 | open
- 2026-05-31 02:43 | backend-ai | test | FB-025 直连快照：Server=1,791,885/5,184,000，Edge 队列约 420,100，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024303.json`，测试继续跑 | open
- 2026-05-31 02:45 | backend-ai | test | FB-025 直连快照：Server=1,811,885/5,184,000，Edge 队列约 408,274，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024510.json`，测试继续跑 | open
- 2026-05-31 02:46 | backend-ai | test | FB-025 直连快照：Server=1,821,885/5,184,000，Edge 队列约 405,220，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024649.json`，测试继续跑 | open
- 2026-05-31 02:48 | backend-ai | test | FB-025 直连快照：Server=1,831,885/5,184,000，Edge 队列约 401,939，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-024825.json`，测试继续跑 | open
- 2026-05-31 02:50 | backend-ai | test | FB-025 直连快照：Server=1,841,885/5,184,000，Edge 队列约 389,840，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025023.json`，测试继续跑 | open
- 2026-05-31 02:52 | backend-ai | test | FB-025 直连快照：Server=1,861,885/5,184,000，Edge 队列约 386,137，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025200.json`，测试继续跑 | open
- 2026-05-31 02:53 | backend-ai | test | FB-025 直连快照：Server=1,871,885/5,184,000，Edge 队列约 383,369，Server ingress=3,711，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025340.json`，测试继续跑 | open
- 2026-05-31 02:55 | backend-ai | test | FB-025 直连快照：Server=1,881,885/5,184,000，Edge 队列约 369,886，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025516.json`，测试继续跑 | open
- 2026-05-31 02:56 | backend-ai | test | FB-025 直连快照：Server=1,891,885/5,184,000，Edge 队列约 366,224，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025651.json`，测试继续跑 | open
- 2026-05-31 02:58 | backend-ai | test | FB-025 直连快照：Server=1,911,885/5,184,000，Edge 队列约 363,437，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-025837.json`，测试继续跑 | open
- 2026-05-31 03:00 | backend-ai | test | FB-025 直连快照：Server=1,921,885/5,184,000，Edge 队列约 350,704，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030017.json`，测试继续跑 | open
- 2026-05-31 03:01 | backend-ai | test | FB-025 直连快照：Server=1,928,874/5,184,000，Edge 队列约 347,334，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030154.json`，测试继续跑 | open
- 2026-05-31 03:03 | backend-ai | test | FB-025 直连快照：Server=1,947,751/5,184,000，Edge 队列约 343,964，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030333.json`，测试继续跑 | open
- 2026-05-31 03:05 | backend-ai | test | FB-025 直连快照：Server=1,957,751/5,184,000，Edge 队列约 340,999，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030514.json`，测试继续跑 | open
- 2026-05-31 03:06 | backend-ai | test | FB-025 直连快照：Server=1,967,751/5,184,000，Edge 队列约 328,529，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030659.json`，测试继续跑 | open
- 2026-05-31 03:08 | backend-ai | test | FB-025 直连快照：Server=1,977,751/5,184,000，Edge 队列约 325,152，Server ingress=10,000，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-030839.json`，测试继续跑 | open
- 2026-05-31 03:10 | backend-ai | test | FB-025 直连快照：Server=1,997,751/5,184,000，Edge 队列约 321,853，Server ingress=0，dead/retry=0，agents 37132/7072 与 watchdog PID=11172 存活；证据 `manual-snapshot-20260531-031018.json`，测试继续跑 | open
- 2026-05-21 18:00 | codex | decision | V0.20 双线协作以 Wails IPC contract 为边界，前端不直接调用 HTTP/RabbitMQ/MySQL | closed
- 2026-05-21 18:00 | codex | decision | 未接真实能力的 Wails 接口返回稳定空状态或 `unsupported`，不能返回假成功 | closed
- 2026-05-21 18:00 | codex | blocker | 当前 shell 无可用 `npm`，前端 build 需先修复 vfox Node/npm 环境 | open
- 2026-05-21 19:45 | codex | decision | 前后端 AI 开工前必须检查 open 项；解决后追加 answer/decision，未解决追加 blocker；最终回复汇报 open/closed | closed
- 2026-05-21 19:47 | codex | decision | 后端每次对话也必须读取协作日志，并主动记录需要前端处理的问题、DTO 变化和阻塞 | closed
- 2026-05-21 20:04 | frontend-ai | answer | 已通过 vfox 重新安装并启用 nodejs@24.15.0，`npm install` 和 `npm run build` 已通过；前端构建阻塞解除 | closed
- 2026-05-21 20:20 | frontend-ai | decision | DataSync 前端界面默认中文并支持中文、英文、日文切换；协议值、DTO 字段、队列名和日志级别仍保持英文 | closed
- 2026-05-21 20:19 | backend-ai | decision | V0.21 不做 Windows Service；改为 Wails 托盘常驻支撑，前端负责托盘 UI，后端提供退出鉴权和当前用户自启动接口 | closed
- 2026-05-21 20:19 | backend-ai | decision | 配置落点确定为 `%ProgramData%\NodeBridge\config.yaml`，密码/token/退出密码使用 DPAPI 加密并对前端脱敏 | closed
- 2026-05-21 20:19 | backend-ai | decision | 新增 Wails 后端方法 `VerifyExitPassword`、`GetAutoStart`、`SetAutoStart`、`ExportDiagnosticPackage`，前端需要按 contract 接入 | closed
- 2026-05-21 20:34 | frontend-ai | answer | 已接入 V0.21 前端托盘控制面、退出密码弹窗、自启动开关、诊断包导出和 `security.exit_password` 配置字段，界面文案保持中英日三语 | closed
- 2026-05-21 20:34 | frontend-ai | blocker | 原生窗口右上角关闭时自动最小化到托盘需要 Wails shell 层 close hook；当前 React 前端已提供隐藏到托盘按钮和退出鉴权弹窗，但无法单独拦截窗口关闭按钮 | open
- 2026-05-21 20:46 | backend-ai | answer | 已在 Wails `OnBeforeClose` 接入 close hook：右上角关闭隐藏到托盘并阻止退出；新增 `RequestExit` 校验退出密码并只放行下一次 `runtime.Quit` | closed
- 2026-05-21 20:46 | backend-ai | decision | 新增管理解锁契约：`UnlockAdmin`、`LockAdmin`、`GetAuthState`；敏感方法未解锁时拒绝，前端必须实现 locked/unlocked UI | closed
- 2026-05-21 20:50 | codex | answer | 已修复根目录 Wails CLI 打包入口，`wails build -clean` 成功产出 `build/bin/DataSync.exe`，并完成原生 exe 进程级启动 smoke | closed
- 2026-05-21 21:02 | backend-ai | decision | 安全字段拆分：`security.admin_password` 负责管理解锁，`security.exit_password` 只负责托盘退出；后端返回两者都脱敏 | closed
- 2026-05-21 21:08 | backend-ai | decision | `SaveConfig` 要求 `security.admin_password` 非空；前端首次配置必须引导用户设置管理密码 | closed
- 2026-05-21 21:36 | backend-ai | answer | 修复前端服务层绑定目标：Wails 生成入口是 `window.go.datasyncui.App`，不是 `window.go.main.App`；这会导致规则、失败记录等页面只能看到空 fallback | closed
- 2026-05-21 21:36 | backend-ai | decision | 根目录 Wails 入口改为 embed `frontend/dist`，避免打包 exe 从 `build/bin` 启动时找不到资源导致白屏 | closed
- 2026-05-21 21:36 | backend-ai | review | 给前端 AI 的具体审阅意见已完成并归入 Activity Log；临时 review notes 文件已删除 | closed
- 2026-05-21 21:58 | backend-ai | decision | `StartAgent`、`StopAgent`、`RestartAgent` 改为控制外部 `SyncAgent.exe` 进程；前端应把 `status=error` 的缺失 exe 提示给用户 | closed
- 2026-05-21 22:10 | frontend-ai | answer | 已按前端审阅意见接入全局 Admin Lock、Unlock 弹窗、敏感操作前置解锁、Rules 新增删除、Failures Retry 解锁和 Config 管理密码必填校验 | closed
- 2026-05-21 22:25 | frontend-ai | question | 当前前端只能通过空 `mode`、空 `node.id`、空 `mysql.database` 推断首次配置；后端是否需要新增 `GetConfigState` 或在 `GetOverview` 暴露 `config_loaded/config_path`，避免长期依赖字段推断 | open
- 2026-05-21 22:25 | frontend-ai | question | Wails 生成绑定时持续输出 `Not found: time.Time`；请后端确认当前前端按 `any/string` 处理时间字段是否可接受，或是否应将 Wails DTO 时间字段改为 RFC3339 string | open
- 2026-05-21 22:25 | frontend-ai | question | `StartAgent` 安装后固定 `SyncAgent.exe` 路径与优雅 shutdown 协议仍需后端 V0.23 决策；前端当前只展示后端返回错误，不自行推断路径或停止协议 | open
- 2026-05-21 22:35 | codex | answer | 已修复 Windows 标题栏 X 直接退出问题：Wails Windows close 事件需要 `HideWindowOnClose=true`，`OnBeforeClose` 继续只负责 `RequestExit` 后的显式退出放行 | closed
- 2026-05-21 22:45 | codex | decision | 修正“隐藏到托盘”误导行为：Wails 2.10.2 公开 `options.App` / `runtime` 未暴露稳定系统托盘创建入口，当前改为窗口最小化/恢复，避免 `WindowHide` 后无托盘入口导致窗口不可恢复 | closed
- 2026-05-21 22:45 | codex | blocker | 真正系统托盘菜单仍需桌面层实现或升级/封装可用 tray API；在此之前前端不再展示“隐藏到托盘”文案 | open
- 2026-05-21 22:55 | codex | answer | 已实现 Windows 原生托盘 helper：`X` 隐藏窗口到系统托盘，托盘双击/菜单可恢复，托盘“退出...”触发前端退出密码弹窗；前端恢复“隐藏到托盘”文案 | closed
- 2026-05-21 22:55 | codex | answer | Rules 页已接入 `source_node_ids` 显示和编辑，前端 SyncRule DTO 同步新增该字段 | closed
- 2026-05-21 22:09 | backend-ai | decision | `SyncRule` 新增 `source_node_ids`，支持多个 Edge 源表同名但中心目标表不同；前端 Rules 页需要显示和编辑该字段 | open
- 2026-05-21 23:51 | backend-ai | answer | V0.24 只新增后端 lab 脚本、迁移和 `dispatch-event-once` CLI；未修改 Wails DTO，前端无需同步改动 | closed
- 2026-05-21 23:51 | backend-ai | answer | `source_node_ids` 前端接入已由 22:55 记录完成，22:09 open 项视为已关闭 | closed
- 2026-05-22 00:01 | backend-ai | decision | 前后端交流收敛为两个核心文件：`frontend-backend-contract.md` 管稳定契约，`ai-collaboration-log.md` 管唯一活跃看板和历史流水 | closed
- 2026-05-22 00:13 | backend-ai | decision | `SyncRule` 新增 `dispatch_target` 和 `dispatch_node_ids`，分发范围改为可配置，前端 Rules 页需要接入 | open
- 2026-05-22 01:21 | backend-ai | answer | 后端已完成 `dispatch_target` / `dispatch_node_ids` 契约、校验、Server dispatch 和 E2E；FB-004 仍由前端接入 Rules UI 控件 | open
- 2026-05-22 01:35 | backend-ai | review | 前端构建和 binding 检查通过；exe 数据 unknown 主要来自后端状态 DTO/规则路径/日志源不完整，已登记 FB-005..FB-008 | open
- 2026-05-22 08:45 | codex | decision | MCP/ClaudeCode 自动化应作为本地管理适配层复用后端 application service，不能直接操作 React、配置文件、RabbitMQ 或 MySQL | open
- 2026-05-22 08:52 | frontend-ai | answer | Rules 页已接入 `dispatch_target` / `dispatch_node_ids` 的只读展示、编辑控件和三语说明，FB-004 关闭 | closed
- 2026-05-22 08:52 | backend-ai | review | 复查当前 RulesPage 代码仍未显示/编辑 `dispatch_target` / `dispatch_node_ids`，FB-004 重新打开并写入 Frontend V0.26 Plan | open
- 2026-05-22 08:52 | backend-ai | answer | V0.26 后端已补 Overview 显式状态、RFC3339 时间、规则 fallback 落盘、CDC configured 状态和外部 SyncAgent 日志读取 | closed
- 2026-05-22 08:54 | backend-ai | answer | 重新确认 RulesPage 已包含 `dispatch_target` / `dispatch_node_ids`，FB-004 保持关闭；前端剩余任务是接 Overview 新字段和 exe 级验收 | closed
- 2026-05-22 09:00 | backend-ai | decision | V0.26 后端规划已固化到 `docs/v0.26-backend-plan.md`，优先处理 FB-003、FB-009、Canal soak、11 节点性能和 exe smoke | open
- 2026-05-22 09:17 | backend-ai | review | 已批复前端需求；批准按现有页面范围推进，新增 MCP Server 开关必须走 Wails 后端接口且默认关闭 | closed
- 2026-05-22 09:17 | backend-ai | decision | 新增 MCP Server 预留接口：仅保存配置开关，不启动真实 MCP runtime，不允许前端绕过管理鉴权 | open
- 2026-05-22 09:30 | frontend-ai | answer | Settings 已接入本地主题偏好和 MCP Server 预留开关，Overview 改用后端显式配置状态，FB-010 关闭 | closed
- 2026-05-22 10:04 | frontend-ai | question | Rules 指定分发目标节点需要 ACTIVE Edge 节点候选项；当前 Wails 契约无节点列表接口，前端只能手填节点 ID | open
- 2026-05-22 10:04 | frontend-ai | answer | Rules 编辑态从宽表改为分组卡片，启用保持滑动开关，方向/分发/冲突保持下拉，降低行高参差和横向拥挤 | closed
- 2026-05-22 10:14 | backend-ai | answer | 已冻结 SyncAgent 查找顺序和 stop-file 停止协议，并新增 `GetAgentProcessStatus()`；FB-003 关闭，前端需接 FB-012 | open
- 2026-05-22 10:30 | backend-ai | decision | 新增受管组件 manifest、RabbitMQ/Canal managed/external 边界和 CDC 安装字段；前端需接 FB-013 | open
- 2026-05-22 10:50 | backend-ai | answer | Config 页已接入 CDC managed/external 安装字段，FB-013 关闭；真实安装执行器仍属后端后续 | closed
- 2026-05-22 11:21 | backend-ai | answer | V0.28 修复并跑通 Edge/Server 真实 Canal E2E 和 20 条 soak；无新增前端契约变化 | closed
- 2026-05-22 11:58 | backend-ai | answer | V0.29 固定目录 package smoke 已通过；无新增前端契约变化，FB-012 仍需前端展示进程状态 | closed
- 2026-05-22 12:14 | backend-ai | decision | V0.30 新增批量失败重试和死信预览 Wails 契约；前端需接入 Failures 页操作入口 | open
- 2026-05-22 12:28 | frontend-ai | answer | Overview 已展示 `GetAgentProcessStatus()` 返回的 SyncAgent 进程路径、PID、日志路径、时间、错误和状态，FB-012 关闭 | closed
- 2026-05-22 12:28 | frontend-ai | answer | Failures 已接入 `RetryFailedEvents` 批量重试和 `GetDeadLetters` 死信只读预览，均先走管理解锁，FB-014 关闭 | closed
- 2026-05-22 13:10 | backend-ai | decision | V0.31 新增受管组件安装计划/执行 Wails 契约和只读 stdio MCP；前端需后续展示安装计划 | open
- 2026-05-22 13:10 | backend-ai | answer | FB-009 已以只读 `mcp-stdio` alpha 收口；MCP 不占端口且不提供任何写工具 | closed
- 2026-05-22 13:31 | backend-ai | answer | V0.32 新增 11 节点 soak/disconnect 后端脚本和诊断摘要；无新增前端契约 | closed
- 2026-05-22 13:45 | backend-ai | answer | V0.33 新增安装器离线包预检和命令计划 CLI；无新增 Wails/前端契约，FB-011 仍 open | closed
- 2026-05-22 13:28 | frontend-ai | answer | Settings 已接入 `GetManagedInstallPlan` / `ApplyManagedInstall`，展示受管组件计划和执行结果，Apply 前要求管理解锁，FB-015 关闭 | closed
- 2026-05-22 13:46 | frontend-ai | answer | 前端完成页面规整与可用性优化：Config 分组编辑、危险操作确认、Settings 分组和开关统一；FB-011 仍等待后端节点候选项接口 | closed
- 2026-05-22 16:34 | review-ai | decision | 明确 AI 身份模型：frontend-ai、backend-ai、test-ai、review-ai 操作前必须声明身份，统一通过 Active Board 协作 | closed
- 2026-05-22 16:43 | backend-ai | answer | 已生成 WinServer2022 Core 用 headless installer test bundle，交给 test-ai 验证；真实 Canal Service 仍待后续实现 | open
- 2026-05-22 16:54 | backend-ai | answer | V0.35 headless installer test bundle 已增强安装、验证、卸载入口；移交 test-ai 等待真实离线包 | blocked
- 2026-05-22 16:50 | test-ai | test | FB-016 默认预检已在 `NodeBridge-V034-InstallerLab-G1` 通过：`admin-check`、`required-files`、`sync-agent-ready`、`canal-check`、`installer-command-plan`、`installer-assets-check`、`managed-plan`、`managed-apply-safe` 均 passed；`real-install` 因缺真实离线包按预期 skipped，证据在 `.cache/v0.34-headless-preflight/` | blocked
- 2026-05-22 16:56 | review-ai | decision | 活跃 AI 看板迁移到根级 `AI_BOARD.md`，与 `MEMORY.md` 同级；`.ai/docs/ai-collaboration-log.md` 改为迁移提示，`.ai/docs/` 只放稳定文档和归档材料 | closed
- 2026-05-22 17:06 | test-ai | test | FB-016 V0.35 版本验证已在 `NodeBridge-V034-InstallerLab-G1` 执行：`build/NodeBridge-headless-installer-test-v0.35.0.zip` 解压后 `package-summary.version=0.35.0`，默认预检 exit 0 且 `managed-plan/managed-apply` manifest 版本为 `0.35.0`；`-VerifyOnly` 在未安装状态下正确失败并记录 `NodeBridgeRabbitMQ` 缺失，证据在 `.cache/v0.35-headless-version-verify/` | blocked
- 2026-05-22 17:17 | backend-ai | answer | 新增只读 `GetNodeOptions()` Wails 契约，FB-011 关闭；Rules 候选多选交给 frontend-ai 处理 FB-017 | open
- 2026-05-22 17:29 | backend-ai | answer | V0.36 headless installer 包已生成并通过默认预检；真实安装闭环仍等待离线包，FB-016 保持 blocked | blocked
- 2026-05-22 17:26 | frontend-ai | answer | Rules 页已接入 `GetNodeOptions()`：SELECTED_EDGES 时显示 ACTIVE Edge 候选勾选、状态消息和手填兜底，FB-017 关闭 | closed
- 2026-05-22 17:45 | test-ai | test | FB-016 V0.36 已在 `NodeBridge-V034-InstallerLab-G1` 验证：默认预检 exit 0，summary/package/manifest 版本均为 `0.36.0`；`prepare-installer-assets-catalog.ps1` 缺包 exit 1、假包生成 4 项 catalog；未安装状态 `-VerifyOnly` 正确失败，双次 `-Uninstall` 均 passed；证据在 `.cache/v0.36-headless-preflight/` 和 `.cache/v0.36-headless-vm-validation/` | blocked
- 2026-05-22 17:55 | frontend-ai | answer | 新增 `docs/frontend-wails-dev.md`，固化 Wails dev 启动、前端门禁和 smoke 检查流程 | closed
- 2026-05-22 17:57 | backend-ai | answer | FB-016 官方 Erlang/RabbitMQ/Canal/WinSW 离线包已下载并生成带真实资产的 V0.36 测试包，移交 test-ai 做 VM 真实安装闭环 | open
- 2026-05-22 18:00 | frontend-ai | answer | 补强 `docs/frontend-wails-dev.md`：加入完整可复制启动命令、绝对路径启动、逐行解释和快速工具检查 | closed
- 2026-05-22 18:17 | frontend-ai | answer | Settings 页恢复为稳定分区布局，避免标题、普通卡片、宽表和危险操作在同一网格中混排 | closed
- 2026-05-22 18:31 | frontend-ai | review | 已完成 1366x768、1600x900、1920x1080 下 8 个前端页面视觉审阅，报告见 `docs/frontend-visual-review-16x9.md` | closed
- 2026-05-22 18:48 | frontend-ai | answer | 根据 16:9 审阅报告优化前端：Config 长值换行、空状态增强、Overview 操作分组、Rules 状态对齐、Manual 限宽、Settings 退出按钮降权，并完成 24 张复测截图 | closed
- 2026-05-22 23:25 | frontend-ai | answer | 使用用户提供图片生成透明背景 NodeBridge 图标，接入 Wails `build/appicon`、前端 favicon 和 Windows 托盘 HICON 数据 | closed
- 2026-05-22 23:43 | frontend-ai | review | 按 Wails 默认 `1100x720` 尺寸完成 8 页截图复查；设计语言适合但默认尺寸布局需优化 Rules 横向溢出、空状态和 Settings 错误区 | closed
- 2026-05-22 23:50 | frontend-ai | answer | 已按默认 `1100x720` 复查结果迭代：Rules 横向溢出修复，Queues/Failures/Logs 空状态结构化，Settings 错误提示降权，Config 默认分栏收敛 | closed
- 2026-05-23 00:07 | frontend-ai | answer | Settings 安全区已说明无初始密码、首次设置、忘记密码重置方式；已替换 `build/windows/icon.ico` 并验证 `DataSync.exe` / `DataSync-dev.exe` 图标不再是默认 W | closed
- 2026-05-23 00:14 | frontend-ai | answer | Overview 首屏移除配置/规则/Agent 路径，路径下沉到 Settings 诊断位置；密码忘记说明改为不可解密找回，只能清空加密字段或重建配置后重新设置 | closed
- 2026-05-23 00:14 | frontend-ai | question | 密码忘记后的产品化恢复流程需要后端评估本机管理员安全重置 CLI/恢复模式，已登记 FB-019 | open
- 2026-05-23 00:21 | frontend-ai | answer | MCP 开关失败原因确认为本机未加载 `%ProgramData%\NodeBridge\config.yaml`；Settings 已禁用未配置状态下的 MCP 开关并补充临时 stdio 语义；说明书新增忘记密码恢复章节 | closed
- 2026-05-23 00:23 | frontend-ai | blocker | 首次只设置管理密码不会创建配置文件，根因是后端 `SaveConfig` 要求完整同步配置校验通过才落盘；已登记 FB-020 交给 backend-ai | open
- 2026-05-23 00:41 | frontend-ai | answer | 用户可见产品名统一为英文 `NodeBridge`，移除说明书/设置/HTML title 中的 `DataSync` 和中文名，并同步 Wails 输出文件名 | closed
- 2026-05-22 23:36 | test-ai | blocker | FB-016 V0.36 真实资产 `-ExecuteInstall` 已在隔离 VM 执行但阻塞：仅发现默认 `RabbitMQ` 服务运行，未发现 `NodeBridgeRabbitMQ`；`rabbitmqctl status/list_*` 因 Erlang cookie 不一致认证失败；真实安装 summary 未覆盖预检 summary。证据在 `.cache/v0.36-real-install-failure/` | blocked
- 2026-05-23 09:10 | backend-ai | answer | FB-016 V0.37 修复安装器：复用已有 RabbitMQ 且不删除客户服务、同步 Erlang cookie、VerifyOnly 校验 NodeBridge 逻辑资源，重新移交 test-ai | open
- 2026-05-23 09:24 | backend-ai | decision | MCP Service 开关改为本次会话临时启用，默认关闭且重启自动关闭；前端需更新 Settings/通知设置文案和状态展示 | open
- 2026-05-23 09:20 | test-ai | blocker | FB-016 V0.37 真实资产 `-ExecuteInstall` 已在隔离 VM 执行；既有 RabbitMQ 复用、cookie 同步和真实 summary 均通过，但 `rabbitmq-bootstrap` 因 AMQP vhost 访问 403 失败，证据在 `.cache/v0.37-real-install-failure/` | blocked
- 2026-05-23 00:39 | test-ai | blocker | FB-016 按用户要求恢复到无 RabbitMQ 快照后重跑 V0.37 完整安装；28 分钟后中断检查发现只完成 Erlang/RabbitMQ 默认服务安装，未写 summary，未进入 cookie/bootstrap 阶段，证据在 `.cache/v0.37-clean-install-interrupted/` | blocked
- 2026-05-23 01:21 | test-ai | blocker | FB-016 V0.38 已恢复 `Clean-Windows-Installed` 干净快照验证完整安装路径；确认无 RabbitMQ/Erlang/NodeBridge 后 `-ExecuteInstall` 在 `install-erlang` 失败，安装进程未超时但 ExitCode 为 null，证据在 `.cache/v0.38-clean-install-failure/` | blocked
- 2026-05-23 09:33 | backend-ai | decision | FB-019 按产品决策闭合：忘记密码由用户备份并手动清空配置文件 security 加密字段，不做后端重置 CLI | closed
- 2026-05-23 09:33 | backend-ai | answer | FB-020 已修复：`SaveConfig` 支持仅含 security 的首次安全草稿落盘，完整同步配置校验不放松 | closed
- 2026-05-23 00:53 | backend-ai | answer | FB-016 V0.38 修复 RabbitMQ 安装器闭环并生成基础包和 with-assets 包，移交 test-ai 做 VM 验证 | open
- 2026-05-25 08:41 | test-ai | blocker | FB-016 V0.38.1 在干净 VM 首次安装确认 ExitCode null 修复生效，但 RabbitMQ ready 90s 超时；同一状态二次安装可完成 RabbitMQ bootstrap，但 Canal WinSW 服务 StartPending 后停止，ExitCode=1067，证据分别在 `.cache/v0.38.1-clean-install-failure/` 和 `.cache/v0.38.1-clean-rerun-canal-failure/` | blocked
- 2026-05-25 10:02 | test-ai | blocker | FB-016 V0.38.2 干净 VM 第一轮 `-ExecuteInstall`、`-VerifyOnly` 和 `-Uninstall` 通过；二次 `-ExecuteInstall` 在 `rabbitmq-bootstrap` 失败且 message 仅 `Error:`，实际 RabbitMQ vhost/queue 已存在，证据在 `.cache/v0.38.2-clean-second-install-failure/` 和 `.cache/v0.38.2-clean-validation/` | blocked
- 2026-05-25 10:28 | test-ai | test | FB-016 V0.38.3 已在 `Clean-Windows-Installed` 干净 VM 跑通 `-ExecuteInstall`、`-VerifyOnly`、二次 `-ExecuteInstall`、`-Uninstall`；卸载后无 RabbitMQ/NodeBridge/Canal 服务，证据在 `.cache/v0.38.3-clean-validation/` | closed
- 2026-05-25 10:35 | backend-ai | answer | FB-024 V0.39.0 新增可选 Java/JRE 离线资产支持，生成基础包与 with-assets 包；强验 Canal Service 需补 Java MSI | open
- 2026-05-25 11:27 | backend-ai | answer | FB-024 V0.40.0 下载 Java/JRE MSI，新增带资产打包脚本并生成可强验 Canal Service 的 with-assets 包 | open
- 2026-05-26 10:32 | test-ai | blocker | FB-024 V0.40.0 第一轮 `-ExecuteInstall -RequireCanalService` 已在 `Clean-Windows-Installed` 干净 VM 执行，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 Java MSI 前置安装未产生 exit code 且 `java.exe` 未安装，导致 `canal-service` 失败；证据在 `.cache/v0.40.0-clean-validation/` | blocked
- 2026-05-26 11:19 | backend-ai | answer | FB-024 V0.40.1 扩大 Java 探测范围、增强 MSI 日志和 probe 重试，生成新 with-assets 包移交 test-ai | open
- 2026-05-26 14:44 | test-ai | blocker | FB-024 V0.40.1 第一轮 `-ExecuteInstall -RequireCanalService` 已在 `Clean-Windows-Installed` 干净 VM 执行，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 Java MSI 安装失败；`msi-java.log` 返回 1620 且有 2203/-2147286960，`java.exe` 未安装，证据在 `.cache/v0.40.1-clean-validation/` | blocked
- 2026-05-26 15:02 | backend-ai | answer | FB-024 V0.40.2 改为 Java/JRE zip 解压式资产，重新生成 with-assets 包移交 test-ai 强验 Canal Service | open
- 2026-05-26 15:28 | test-ai | blocker | FB-024 V0.40.2 第一轮 `-ExecuteInstall -RequireCanalService` 已在 `Clean-Windows-Installed` 干净 VM 执行，Java zip 解压和探测通过，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 NodeBridgeCanal 启动后停止；日志显示 JRE 17 不识别 `PermSize=128m`，证据在 `.cache/v0.40.2-clean-validation/` | blocked
- 2026-05-26 15:47 | backend-ai | answer | FB-024 V0.40.3 清洗 Canal startup 旧 PermGen JVM 参数并生成 with-assets 包，移交 test-ai 复测 Canal Service 强验 | open
- 2026-05-26 16:04 | test-ai | blocker | FB-024 V0.40.3 第一轮 `-ExecuteInstall -RequireCanalService` 和 `-VerifyOnly` 已通过，二次 `-ExecuteInstall -RequireCanalService` 阻塞在 `canal-asset-extract`，运行中的 Canal 文件 `lib/canal.server-1.1.8.jar` 无法 unlink 覆盖；证据在 `.cache/v0.40.3-clean-validation/` | blocked
- 2026-05-26 16:23 | backend-ai | answer | FB-024 V0.40.4 修复 Canal 运行中二次安装热覆盖，生成新 with-assets 包移交 test-ai | open
- 2026-05-26 16:42 | test-ai | blocker | FB-024 V0.40.4 第一轮安装、首次验证、二次安装、二次验证均通过；`-Uninstall` 失败在 `verify-uninstall`，message=`NodeBridgeCanal still exists`，延迟 10 秒后服务消失；证据在 `.cache/v0.40.4-clean-validation/` | blocked
- 2026-05-26 16:47 | backend-ai | answer | FB-024 V0.40.5 修复卸载服务删除等待确认并生成新 with-assets 包移交 test-ai | open
- 2026-05-26 17:39 | test-ai | test | FB-024 V0.40.5 在 `Clean-Windows-Installed` 干净 VM 通过完整闭环：安装、验证、二次安装、二次验证、卸载、二次卸载均通过，卸载后无 RabbitMQ/NodeBridge/Canal 服务；证据在 `.cache/v0.40.5-clean-validation/` | closed
- 2026-05-31 03:24 | backend-ai | answer | FB-025 新增按表 `sync_mode`：`crud_ordered` 保持原保序 CRUD，`append_only` 用于历史倾倒/采集流水表并启用 Server 多行批量插入；长测 6 张采集表已切到 `append_only`，`appendonly-smoke-001` 18/18 通过，新 30 天复测 `appendonly-month30-001` 后台 PID=8536 已进入 seed，早期 Edge=1,040,000/5,184,000 | open
- 2026-05-31 04:09 | backend-ai | answer | FB-025 继续定位瓶颈后确认 Canal->Edge RabbitMQ 仍逐条 publish，已改为 `PublishBatch` 成功后再提交 Canal offset；Server append-only apply 同时支持 interleaved 批次按目标表分组。验证：`go test ./...`、`go vet ./...`、核心覆盖率 74.7% >= 70%；`appendonly-month30-003` Docker RabbitMQ prepare 异常已清理，最终后台复测为 `appendonly-month30-004` PID=41432，早期 Edge=290,000/5,184,000 | open
- 2026-05-31 04:32 | backend-ai | answer | FB-025 已把 `sync_mode` 补到 Rules 页面和 Manual：按表选择 `crud_ordered` 或 `append_only`，历史倾倒/采集流水表可用 append-only，少量增删改表继续保序 CRUD；验证 `go test ./internal/rules ./internal/mapper ./internal/apply ./internal/syncruntime` 与 `frontend npm run build` 通过。现场 `appendonly-month30-004` 仍未通过：Edge=5,184,000，Server=0，Server ingress 约 150,000 且无落库事务，需继续定位 Server batch apply 卡点 | open
- 2026-05-31 04:34 | backend-ai | blocker | FB-025 `appendonly-month30-004` 已停止 runner/agents，保留 Docker 现场和 `.cache/longtest-90d/appendonly-month30-004/` 证据；该轮不计通过。下一步需在 Server batch apply 增加可观测日志/批次限流后重测，避免 Server 端拿到大批量消息但不提交落库 | blocked
- 2026-05-31 04:40 | backend-ai | answer | FB-025 根因确认并修复：append-only 多行 INSERT / sync_apply_log 批量写入超过 MySQL prepared statement 占位符上限，触发 `Error 1390` 后整批 requeue；已按 `maxStatementPlaceholders=60000` 自动切片。现场验证：旧失败现场 `max-batch=2000` 耗时 2.54s applied，`max-batch=10000` 耗时 11.98s applied。门禁：`go test ./...`、`go vet ./...`、核心覆盖率 74.8%、`frontend npm run build` 通过。新 30d 复测 `appendonly-month30-005` 已从干净 Docker volume 启动，PID=17136，证据 `.cache/longtest-90d/appendonly-month30-005/`，04:39 Edge=130,000/5,184,000，Server=0，正在 seed | open
- 2026-05-31 04:42 | backend-ai | answer | FB-025 `appendonly-month30-005` seed 阶段稳定推进，runner PID=17136 存活；04:42 快照 Edge=1,030,000/5,184,000，Server=0，队列为空，尚未进入 agents drain。证据目录 `.cache/longtest-90d/appendonly-month30-005/` | open
- 2026-05-31 04:58 | backend-ai | answer | FB-025 `appendonly-month30-005` 已完成 seed 并进入 agents drain：Edge=5,184,000，Server=90,000，Edge agent PID=3816、Server agent PID=12216 均存活，Server stderr 为空；Edge upload 队列约 1,118,020，Server ingress 10,000 unacked，说明修复后的 Server apply 正在推进。已启动 watchdog PID=40084，每 5 分钟写 `watchdog-progress.csv` / `watchdog-summary.json` | open
- 2026-05-31 05:05 | backend-ai | answer | FB-025 `appendonly-month30-005` drain 稳定推进：实时快照 Edge=5,184,000、Server=290,000；watchdog 05:03 样本 Server=250,000、Edge upload=2,905,880、Server ingress=10,000、agent_count=2、status=running；dead/retry 队列为 0。预计仍需数小时追平，继续后台运行 | open
- 2026-05-31 05:10 | backend-ai | answer | FB-025 `appendonly-month30-005` 继续推进：实时 Server=480,000/5,184,000；watchdog 05:08 样本 Server=410,000、Edge upload=4,709,923、Server ingress=0、agent_count=2、status=running；当前 Edge upload=4,634,000、Server ingress=70,000、dead/retry=0。修复后无 `Error 1390` 复发，继续后台 drain | open
- 2026-05-31 05:12 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=560,000；Edge upload=4,474,000、Server ingress=150,000，dead/retry=0。当前按表 `sync_mode` 设计确认：历史倾倒/采集流水表用 `append_only` 加速，低频增删改查表继续用默认 `crud_ordered` 保序 | open
- 2026-05-31 05:13 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：runner PID=17136、watchdog PID=40084、Edge/Server agents PID=3816/12216 均存活；实时 Edge=5,184,000、Server=610,000，Edge upload=4,374,000、Server ingress=200,000，dead/retry=0，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:19 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=850,000；Edge upload=3,884,000、Server ingress=460,000，dead/retry=0。watchdog 05:18 样本 Server=790,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:21 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：runner/watchdog/agents 仍存活；实时 Edge=5,184,000、Server=890,000，Edge upload=3,784,000、Server ingress=510,000，dead/retry=0；watchdog 文件最新仍为 05:18 样本，实时 MySQL/RabbitMQ 证明同步继续增长 | open
- 2026-05-31 05:32 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,260,000，Edge upload=2,914,000、Server ingress=1,010,000，dead/retry=0。watchdog 05:28 样本 Server=1,140,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:33 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,290,000，Edge upload=2,834,000、Server ingress=1,060,000，dead/retry=0；runner/watchdog/agents 仍存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:34 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,320,000，Edge upload=2,754,000、Server ingress=1,110,000，dead/retry=0；watchdog 05:33 样本 Server=1,300,000、agent_count=2、status=running，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:35 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,350,000，Edge upload=2,674,000、Server ingress=1,160,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:36 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,380,000，Edge upload=2,604,000、Server ingress=1,210,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:37 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,400,000，Edge upload=2,534,000、Server ingress=1,260,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:38 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,430,000，Edge upload=2,454,000、Server ingress=1,300,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:39 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,460,000，Edge upload=2,384,000、Server ingress=1,353,201，dead/retry=0；watchdog 05:38 样本 Server=1,450,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:39 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,480,000，Edge upload=2,324,000、Server ingress=1,390,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:40 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,510,000，Edge upload=2,244,000、Server ingress=1,440,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:41 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,530,000，Edge upload=2,174,000、Server ingress=1,490,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:42 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,560,000，Edge upload=2,094,000、Server ingress=1,530,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:43 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,590,000，Edge upload=2,014,000、Server ingress=1,590,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:44 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,610,000，Edge upload=1,934,000、Server ingress=1,640,000，dead/retry=0；watchdog 05:43 样本 Server=1,590,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:45 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,640,000，Edge upload=1,859,986、Server ingress=1,700,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:46 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,660,000，Edge upload=1,784,000、Server ingress=1,740,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:47 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,690,000，Edge upload=1,694,000、Server ingress=1,800,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:48 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,720,000，Edge upload=1,614,000、Server ingress=1,850,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:49 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,750,000，Edge upload=1,514,000、Server ingress=1,920,000，dead/retry=0；watchdog 05:48 样本 Server=1,720,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:51 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,780,000，Edge upload=1,444,000、Server ingress=1,960,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:51 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,800,000，Edge upload=1,374,000、Server ingress=2,010,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:52 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,820,000，Edge upload=1,314,000、Server ingress=2,050,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:53 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,850,000，Edge upload=1,234,000、Server ingress=2,110,000，dead/retry=0；watchdog 05:53 样本 Server=1,850,000、agent_count=2、status=running；测试未完成，继续后台 drain | open
- 2026-05-31 05:54 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,880,000，Edge upload=1,154,000、Server ingress=2,160,000，dead/retry=0；runner/watchdog/agents 存活，agent stderr 为空；测试未完成，继续后台 drain | open
- 2026-05-31 05:55 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,900,000，Edge upload=1,074,000、Server ingress=2,210,000，dead/retry=0；按表 `sync_mode` 继续作为产品策略：历史倾倒表走 `append_only` 批量加速，增删改查表保留 `crud_ordered` 保序 | open
- 2026-05-31 05:57 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,950,000，Edge upload=934,000、Server ingress=2,300,988，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成，继续后台 drain | open
- 2026-05-31 05:58 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=1,970,000，Edge upload=864,000、Server ingress=2,350,000，dead/retry=0；agents CPU 仍增长但 watchdog 文件暂无新采样，测试未完成，继续后台 drain | open
- 2026-05-31 05:59 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,000,000，Edge upload=784,000、Server ingress=2,400,834，dead/retry=0；watchdog 已恢复采样，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:00 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,030,000，Edge upload=704,000、Server ingress=2,460,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:01 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,050,000，Edge upload=634,000、Server ingress=2,500,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:02 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,070,000，Edge upload=564,000、Server ingress=2,550,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:03 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,090,000，Edge upload=494,000、Server ingress=2,600,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:04 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,120,000，Edge upload=424,000、Server ingress=2,644,320，dead/retry=0；watchdog 06:03 样本 Server=2,110,000，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:05 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,140,000，Edge upload=344,000、Server ingress=2,700,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:05 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,170,000，Edge upload=274,000、Server ingress=2,750,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:06 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,190,000，Edge upload=204,000、Server ingress=2,790,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:07 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,210,000，Edge upload=134,000、Server ingress=2,840,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:08 | test-ai | test | FB-025 `appendonly-month30-005` 继续推进：实时 Edge=5,184,000、Server=2,240,000，Edge upload=54,000、Server ingress=2,900,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:09 | test-ai | test | FB-025 `appendonly-month30-005` 已进入 Server 单独 drain 阶段：实时 Edge=5,184,000、Server=2,260,000，Edge upload=0、Server ingress=2,934,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:10 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,280,000，Edge upload=0、Server ingress=2,904,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:11 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,310,000，Edge upload=0、Server ingress=2,874,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:12 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,340,000，Edge upload=0、Server ingress=2,854,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:13 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,360,000，Edge upload=0、Server ingress=2,824,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:14 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,380,000，Edge upload=0、Server ingress=2,804,000，dead/retry=0；watchdog 06:14 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:15 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,410,000，Edge upload=0、Server ingress=2,774,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:16 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,430,000，Edge upload=0、Server ingress=2,754,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:17 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,460,000，Edge upload=0、Server ingress=2,724,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:18 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,490,000，Edge upload=0、Server ingress=2,701,368，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:19 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,510,000，Edge upload=0、Server ingress=2,674,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:20 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,540,000，Edge upload=0、Server ingress=2,654,000，dead/retry=0；watchdog 06:19 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:20 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,560,000，Edge upload=0、Server ingress=2,624,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:21 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,590,000，Edge upload=0、Server ingress=2,604,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:22 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,610,000，Edge upload=0、Server ingress=2,574,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:23 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,640,000，Edge upload=0、Server ingress=2,554,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:24 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,660,000，Edge upload=0、Server ingress=2,524,000，dead/retry=0；watchdog 06:24 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:25 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,690,000，Edge upload=0、Server ingress=2,494,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:28 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,780,000，Edge upload=0、Server ingress=2,414,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:30 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,800,000，Edge upload=0、Server ingress=2,384,000，dead/retry=0；watchdog 06:29 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:31 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,840,000，Edge upload=0、Server ingress=2,354,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:32 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,870,000，Edge upload=0、Server ingress=2,324,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:33 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,900,000，Edge upload=0、Server ingress=2,294,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:35 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,960,000，Edge upload=0、Server ingress=2,234,000，dead/retry=0；watchdog 06:34 样本正常，runner/watchdog/agents 存活，测试未完成 | open
- 2026-05-31 06:37 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=2,980,000，Edge upload=0、Server ingress=2,204,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:39 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,030,000，Edge upload=0、Server ingress=2,154,000，dead/retry=0；`append_only` 作为历史倾倒表模式继续验证，CRUD 表仍保留 `crud_ordered` 保序链路 | open
- 2026-05-31 06:40 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,060,000，Edge upload=0、Server ingress=2,124,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，测试未完成 | open
- 2026-05-31 06:45 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,210,000，Edge upload=0、Server ingress=1,984,000，dead/retry=0；近 5.6 分钟增加约 150,000 行，约 26,800 行/分钟，按当前速度仍需约 74 分钟 | open
- 2026-05-31 06:56 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,480,000，Edge upload=0、Server ingress=1,714,000，dead/retry=0；近 10.7 分钟增加约 270,000 行，约 25,300 行/分钟，agent stderr 为空 | open
- 2026-05-31 07:12 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=3,870,000，Edge upload=0、Server ingress=1,314,000，dead/retry=0；近 15.6 分钟增加约 390,000 行，约 25,000 行/分钟，runner/watchdog/agents 存活且 agent stderr 为空 | open
- 2026-05-31 07:27 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,250,000，Edge upload=0、Server ingress=934,000，dead/retry=0；近 15.5 分钟增加约 380,000 行，约 24,500 行/分钟，agent stderr 为空 | open
- 2026-05-31 07:43 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,640,000，Edge upload=0、Server ingress=554,000，dead/retry=0；近 15.6 分钟增加约 390,000 行，约 25,000 行/分钟，进入最后 55 万后改为 5 分钟采样 | open
- 2026-05-31 07:48 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,770,000，Edge upload=0、Server ingress=414,000，dead/retry=0；近 5.5 分钟增加约 130,000 行，约 23,500 行/分钟，agent stderr 为空 | open
- 2026-05-31 07:54 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 继续推进：实时 Edge=5,184,000、Server=4,910,000，Edge upload=0、Server ingress=274,000，dead/retry=0；近 5.6 分钟增加约 140,000 行，约 25,000 行/分钟，agent stderr 为空 | open
- 2026-05-31 08:00 | test-ai | test | FB-025 `appendonly-month30-005` Server 单独 drain 接近尾段：实时 Edge=5,184,000、Server=5,040,000，Edge upload=0、Server ingress=144,000，dead/retry=0；进入最后 3% 后改为 2 分钟短轮询 | open
- 2026-05-31 08:07 | test-ai | test | FB-025 `appendonly-month30-005` 30 天量级复测通过：runner-summary `passed`，drain-final `completed=true`；Edge/Server 总数均 5,184,000，6 张表各 864,000；Edge upload/retry/dead/downlink 与 Server ingress/dead/downlink 队列全 0；`sync_apply_log=5,184,000`、`sync_event_log=5,184,000` 且 payload 全 NULL、`sync_ack_log=0`，agent stderr 与错误关键字扫描为空；已停止残留 watchdog | closed
- 2026-05-31 08:43 | test-ai | handoff | FB-026 用户暂停 15 天混合断网压测，已停止 `mixed-smoke-001` 残留 PowerShell/SyncAgent 进程；前端联调关注 `sync_mode` 规则字段：可选 `crud_ordered`（默认，增删改查保序）与 `append_only`（历史倾倒/采集流水，只接受 INSERT，UPDATE/DELETE 会由后端拒绝并走现有失败链路）。当前前端已有 Rules 下拉、Manual 文案和 Wails DTO 字段，但需要 frontend-ai 联调确认：新增规则默认 `crud_ordered`、编辑/保存不丢 `sync_mode`、`append_only` 显示危险提示、三语文案准确，并用后端真实规则保存/读取闭环验证 | closed
- 2026-05-31 08:55 | frontend-ai | answer | FB-026 已完成：Rules 新增规则默认 `crud_ordered`；编辑态和只读态展示 `sync_mode`；`append_only` 追加中英日危险提示；DTO 保存字段保持透传；前端门禁通过 | closed
- 2026-06-01 09:04 | test-ai | test | FB-027 按用户要求启动 15 天混合断网压力测，不持续盯进度：无效首轮 `mixed-15d-offline-001` 已停止，原因是先停 Server RabbitMQ 再启动 Edge agent 会导致 Edge agent 首连失败；已修正 harness 为先启动 Edge agent 再停 Server RabbitMQ。有效 run 为 `mixed-15d-offline-002`，runner PID=41444、Edge agent PID=29348，证据目录 `.cache/longtest-90d/mixed-15d-offline-002/`；测试计划为 5 张 `tag_state_*` 保序 CRUD 表 + 12 张 `collect_data_*` append-only 表，15 天，预期业务行 5,189,000、预期事件 7,349,000。启动确认：Server RabbitMQ 已停止模拟断网，Edge agent 存活且 stderr 为空，Edge 当前 append_rows=80,000、tag_rows=5,000，测试正在运行 | open
- 2026-06-01 09:10 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows 从 120,000 增至 240,000、tag_rows=5,000，Edge upload 队列从 169,604 增至 299,264，retry/dead=0；runner/agent 日志关键字扫描未见 panic/fatal/Error 1390/FAILED，当前为正常积压生成阶段 | open
- 2026-06-01 10:31 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=1,560,000/5,184,000，tag_rows=5,000，Server 业务行仍 0，Edge upload 队列=2,199,994、retry/dead=0；runner.err 仅 MySQL password warning，edge.err 为空，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，当前仍在断网积压生成阶段 | open
- 2026-06-01 14:05 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=3,480,000/5,184,000，tag_rows=5,000，Server 业务行仍 0，Edge upload 队列=4,904,324、retry/dead=0；runner.out 最新进度 `rows_each_append=200000/432000`，日志关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，仍在断网积压生成阶段 | open
- 2026-06-01 15:54 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=4,320,000/5,184,000，tag_rows=5,000，Server 业务行仍 0；Edge upload 队列=6,081,325/预期事件 7,349,000，retry/dead=0；runner.err 仍为 MySQL password warning，edge.err 为空，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，仍在断网积压生成阶段 | open
- 2026-06-01 17:04 | test-ai | test | FB-027 用户要求“灌完数据先停，明天再测恢复”：当前 `mixed-15d-offline-002` 仍在断网灌数，runner PID=41444、Edge agent PID=29348 均存活；Edge append_rows=4,800,000/5,184,000，tag_rows=5,000，Edge upload 队列约 6,793,892/7,349,000，tag 版本约 389k-400k/432k。已启动 pause monitor PID=37368，每 10 秒检查；达到 append_rows=5,184,000 且 edge_upload>=7,349,000 后会自动停止 runner 和 Edge agent，并保持 Server RabbitMQ stopped，写 `.cache/longtest-90d/mixed-15d-offline-002/pause-after-seed.json` | open
- 2026-06-01 17:18 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002` 暂停前状态：runner PID=41444、Edge agent PID=29348、pause monitor PID=37368 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=4,920,000/5,184,000，还差 264,000；Edge upload 队列=6,959,575/7,349,000，还差 389,425；pause marker 尚未生成，monitor 正常采样；retry/dead=0，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED | open
- 2026-06-01 17:59 | test-ai | test | FB-027 `mixed-15d-offline-002` 已按用户要求停在“离线积压已灌完、尚未恢复同步”状态：`pause-after-seed.json` 17:55:42 生成，append_rows=5,184,000、edge_upload_depth=7,349,000，target reached 后已停止 runner PID=41444 和 Edge agent PID=29348；Server RabbitMQ 仍 stopped，Edge upload.cdc.q=7,349,000 ready / 0 unacked，retry/dead=0，Server 业务行仍 0。明天可从该现场启动 Server RabbitMQ 和 agents 做恢复 drain | open
- 2026-06-02 14:12 | test-ai | test | FB-027 重启后复核 `mixed-15d-offline-002` 恢复测试前置条件：先只拉起 Edge/Server MySQL、Edge RabbitMQ、Edge Canal，保持 Server RabbitMQ stopped；Edge RabbitMQ 用约 107 秒完成 7,349,000 条持久消息索引重建，队列为 edge.upload.cdc.q=7,349,000 ready / 0 unacked / 0 consumers，retry/dead=0；Edge append_rows=5,184,000、tag_rows=5,000，Server 业务行仍 0；无残留 runner/agent/pause monitor，日志关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED。可以启动恢复 drain，不需要重灌数据 | open
- 2026-06-02 14:23 | test-ai | test | FB-027 已按用户要求启动 `mixed-15d-offline-002` 恢复 drain：第一次控制器因 Server RabbitMQ 未实际启动导致 init 拓扑失败，未启动 agents、未消费积压；随后手动启动 Server RabbitMQ、初始化 Server 拓扑成功，并启动修正后的恢复控制器 PID=22524、Edge/Server SyncAgent PID=23644/4228。14:23 样本显示恢复已运行：ServerRows=70,000、ApplyLog=70,000、Edge upload=7,177,118、Server ingress=97,092，当前 controller-2/edge/server stderr 均为空；日志在 `.cache/longtest-90d/mixed-15d-offline-002/agents-recovery/`，进度在 `recovery-progress.csv` / `recovery-summary.json` | open
- 2026-06-02 16:20 | review-ai | decision | FB-028 已将 Server Apply 成倍性能优化计划交给 backend-ai：优先分流 `append_only` 与 `crud_ordered`，append-only 批量业务写和批量 apply_log，CRUD 按 `target_table + pk` hash lane 并行且 lane 内保序，后续可选 `crud_ordered_compact` 合并同主键连续 UPDATE；约束为 ACK 仍在业务写和系统日志提交后，SyncEvent 映射、回环抑制、幂等、重试/死信不变量不破坏。目标：append-only >=1,500 rows/s，mixed apply >=1,000 events/s，并用现有长测证据对比 | open
- 2026-06-02 17:39 | test-ai | test | FB-027 巡检 `mixed-15d-offline-002` 恢复 drain：Edge upload 已清空，Server ingress=3,599,000，ServerRows=2,650,000/5,189,000，ApplyLog=3,750,000/7,349,000；恢复控制器 PID=22524、Edge/Server SyncAgent PID=23644/4228 均存活，controller-2/edge/server stderr 为空，retry/dead=0。最近约 20 分钟 apply 约 285 events/s，剩余约 3.6M apply events，预计还需约 3.5 小时；继续后台运行，不持续盯进度 | open
- 2026-06-03 08:30 | test-ai | blocker | FB-027 复核 `mixed-15d-offline-002` 最终证据：`recovery-summary.json` 标记 completed，`server.cdc.ingress.q=9000` 且无消费者，`server_event_log=7,340,000`（少 9000），`sync_apply_log=7,349,000`；说明还有未清尾段事件需确认并补 drain | open
- 2026-06-03 09:08 | test-ai | answer | FB-027 已补测 `mixed-15d-offline-002` 尾段：先修正规则配置后逐批 `consume-batch-once` 清理剩余 9,000，随后重核样本显示 `recovery-summary.json` `completed`、`server.cdc.ingress.q=0`、`server_rows=5,189,000`、`sync_apply_log=7,349,000`、`sync_event_log=7,349,000`，`sync_ack_log failed=0` | closed
- 2026-06-02 16:25 | backend-ai | answer | FB-028 完成 P1a mixed batch 优化：连续 append-only 段按目标表批量写入，CRUD 边界不跨越；全量 Go 测试和 vet 通过，已重建 `build/bin/SyncAgent.exe`，移交 test-ai 用新二进制对比 mixed 恢复吞吐 | open
- 2026-06-02 16:45 | backend-ai | answer | FB-028 完成 P2/P3/P4 后端：append-only 批量段、Apply lane 并行、compact 双开关门槛；全量 Go 测试和 vet 通过，已重建 `build/bin/SyncAgent.exe`。新增 FB-029 交前端做 Settings 开关和 Rules 单表选项，禁止一键全开 | open
- 2026-06-02 16:54 | backend-ai | decision | 新增 FB-030/FB-031：MCP 目标调整为远程 AI 受控查看和修改本机 NodeBridge 配置；前端更新 Settings/Manual 说明，后端设计白名单写配置能力 | open
- 2026-06-02 17:18 | frontend-ai | answer | FB-029/FB-030 已完成：Settings 增加 CRUD compact 全局开关和 MCP stdio 客户端配置提示；Rules 增加单条 `crud_ordered_compact`；Manual 与 `docs/mcp-service.md` 更新新 MCP 目标和边界；前端门禁通过 | closed
- 2026-06-02 16:54 | backend-ai | test | MCP 最小 smoke 通过：`go test -count=1 ./internal/mcpstdio ./cmd/sync-agent -run "MCP|Mcp|mcp"`；当前只读 stdio 入口可继续作为后续写配置能力基础 | open
- 2026-06-02 17:07 | backend-ai | answer | FB-031 已完成 MCP 白名单写配置：非敏感 config patch、规则保存、审计日志、`mcp_server.enable` 启动门禁、contract/runbook 更新；`go test ./...`、`go vet ./...` 通过，linter 未安装跳过，已重建 `build/bin/SyncAgent.exe` | closed
- 2026-06-02 17:28 | backend-ai | answer | FB-031 MCP 增强完成：新增 dry-run 验证工具、工具 schema、拒绝写审计和磁盘最新配置/规则读取；安装 golangci-lint 2.12.2，Go test/vet/lint 通过，已重建 SyncAgent | closed
- 2026-06-03 08:45 | backend-ai | test | FB-031 MCP 定向 Go 测试和真实 `SyncAgent.exe mcp-stdio` smoke 通过：initialize/tools/list/dry-run/save patch/reject secret/save rules/audit 均通过；关闭态 `mcp_server.enable=false` 返回非 0 且 stderr 明确原因；Go test/vet/lint 通过并重建 SyncAgent | closed
- 2026-06-03 09:05 | backend-ai | decision | 测试密码统一写入 `docs/test-credentials.md`；安装 VM 文档和 installer skill 清除散落明文，改为引用统一清单；lab 配置/脚本保留可运行测试值 | closed
- 2026-06-03 10:19 | backend-ai | test | 已生成本机解压试用包 `build/v0.45-local-trial-20260603.zip`；包内 `SyncAgent.exe -config config.yaml` 和隐藏启动 `DataSync.exe` smoke 均通过；该包不执行 RabbitMQ/Canal 系统安装 | closed
- 2026-06-03 10:31 | review-ai | decision | 新增 FB-032 委托 backend-ai 制作 NSIS beta 安装器；允许在不冲突时使用本机 Docker MySQL/RabbitMQ 做非破坏 smoke，真实 Erlang/RabbitMQ/Canal/WinSW 系统安装仍只在隔离 VM 验证 | open
- 2026-06-03 10:42 | backend-ai | answer | FB-032 NSIS beta 源码和 staging 已完成：新增 NSIS 脚本、安装/卸载 PowerShell 包装、打包脚本，`package-nsis-beta.ps1 -NoBuild -SkipHeadlessPackage -SkipNSIS` 通过；PowerShell 语法检查通过；本机缺 `makensis.exe`，最终 exe 编译待装有 NSIS 的环境 | open
- 2026-06-03 10:52 | backend-ai | answer | FB-032 已通过 winget 安装 NSIS 3.12，并用 `makensis.exe` 生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `5D2FE45EEED0C1B2AF0B0DE360797933599F97CE89C748EDEF9984CEABF64A7A`；未运行该安装器，未触碰宿主机 Erlang/RabbitMQ/Java/Canal/WinSW 系统组件；owner 转 test-ai 做隔离 VM 闭环 | open
- 2026-06-03 11:02 | backend-ai | answer | FB-032 修正管理端 exe 命名：`package-smoke.ps1`、`package-nsis-beta.ps1`、NSIS 快捷方式和安装包装脚本均改为 `NodeBridge.exe`；重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `1E3AC51C1E305BA1F15C8CBD03E99BD29137162A62831A89ABCE8851501EF49B`；未运行安装器本身 | open
- 2026-06-03 11:24 | backend-ai | answer | FB-032 根据测试截图修复两点：管理端改用 `wails build` 生成正式 Wails exe，`package-smoke` 隐藏启动通过；NSIS 默认取消 `-RequireCanalService`，避免普通测试机服务强验失败直接中断。新包 `build/NodeBridge-beta-v0.45.0-20260603.exe` SHA256 `C6A0A106C4EFA449E16869F2DB9B3AB76B2C3E009735D37100194C6DC9D90F17`，未运行安装器本身 | open
- 2026-06-03 12:01 | backend-ai | answer | FB-032 按实机反馈修复图标、保存重启提示和 RabbitMQ local/server 拆分：快捷方式显式使用 `NodeBridge.ico`，Config 保存运行时配置变更后提示重启同步进程，新增 RabbitMQ 队列初始化按钮，远端 RabbitMQ 不可达不再让本地配置整体失败。已重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `60AB33C6B71F14ABC985129FAB0D0A0072B03F1E0E418E3EEF55E0FF0A9FEAFA`；package smoke、前端 build、Go test/vet/lint 均通过；未运行安装器本身 | open
- 2026-06-03 14:06 | backend-ai | answer | FB-032 修复实机安装和 RabbitMQ 403 根因：NSIS 安装遇到安全草稿/不完整 `%ProgramData%\NodeBridge\config.yaml` 时备份并写入完整默认配置；默认 RabbitMQ URL 改为 `nodebridge_test` 和 encoded `/nodebridge-*` vhost，匹配安装器 bootstrap；NSIS 打包强制使用仓库示例配置而非可能陈旧的 `build/bin/config.yaml`。已重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `83362DFC51E56A21A56383F8639FB37E586DE7745FD6CBE037700B97C20B9CEB`；相关 Go 测试和打包预检通过，未运行安装器本身 | open
- 2026-06-03 14:20 | backend-ai | answer | FB-032 修复 NSIS 安装 `system-components` 1 秒失败 exit code `-196608`：不再用 `Start-Process -ArgumentList` 传递带空格的 `C:\Program Files\NodeBridge\installer\headless\...` 脚本路径，改为 PowerShell 参数数组直接执行并记录 `component-headless-install.out/err.txt`。已重新生成 `build/NodeBridge-beta-v0.45.0-20260603.exe`，SHA256 `947599EA3F2F2620EE0164A6F20E5918EFBD1661FB16176E05B38B5E2972ADD7`；语法检查和相关 Go 测试通过，未运行安装器本身 | open
- 2026-05-26 15:10 | test-ai | test | FB-025 90 天等价长测 harness 已落地并完成 prepare/smoke/query/archive 轻量验证；真实 Canal 抓取 12 条并同步到 Server，证据在 `.cache/longtest-90d/smoke-001/` | open
- 2026-05-25 10:11 | backend-ai | answer | FB-016 V0.38.3 修复二次安装 RabbitMQ bootstrap 幂等和 runtime 证据残留，已生成基础包与 with-assets 包移交测试 | open
- 2026-05-25 08:59 | backend-ai | answer | FB-016 V0.38.2 修复 RabbitMQ ready PATH 和 Canal Java/WinSW 证据逻辑，已生成基础包与 with-assets 包移交测试 | open
- 2026-05-23 01:14 | frontend-ai | answer | 已配合 FB-020 后端首次安全草稿约定：Settings 首次安全保存发送最小 `security` DTO；稳定契约用户可见名称同步为 `NodeBridge` | closed
- 2026-05-23 01:28 | frontend-ai | answer | 已移除顶部管理锁定横幅和“本地管理”常驻文字，只读/解锁状态改为底部右侧指示器 | closed
- 2026-05-23 10:12 | frontend-ai | answer | 解锁入口收敛到右下角胶囊；Config/Rules/Settings 不再显示页面内解锁按钮；MCP 持久启用需求登记为 FB-022 交后端 | open
- 2026-05-23 10:24 | frontend-ai | answer | 右下角权限控件改为状态/动作分段样式：左侧只读/编辑状态不可点击，右侧解锁/锁定按钮明确可操作 | closed
- 2026-05-23 01:21 | test-ai | test | MCP stdio smoke：示例完整配置下 `initialize`、`tools/list`、`nodebridge_overview`、`resources/read` 通过，写工具 `save_config` 正确拒绝；当前 `%ProgramData%\NodeBridge\config.yaml` 缺 `mode/node.id/mysql.database`，实际本机 MCP 客户端配置启动会失败 | blocked
- 2026-05-23 01:21 | test-ai | blocker | 已将 MCP 实际配置启动失败登记为 FB-021 交给 backend-ai；阻塞点是 Wails 会话开关可开启，但 `SyncAgent.exe mcp-stdio` 对同一 `%ProgramData%` 配置执行完整配置校验后失败 | open
- 2026-05-23 10:10 | test-ai | blocker | FB-021 复测实际 `%ProgramData%\NodeBridge\config.yaml`：配置为安全草稿，`SyncAgent.exe mcp-stdio` exit 1，stderr 为 `decrypt admin password: Key not valid for use in specified state`；相关 Go 测试因本机 vfox Go SDK 标准库缺失无法运行 | blocked
- 2026-05-23 10:28 | backend-ai | answer | 修复 FB-016 installer ExitCode null 兜底探测并生成 V0.38.1 测试包，移交 test-ai 复测 | open
- 2026-05-23 10:28 | backend-ai | answer | 闭合 FB-021/FB-022：MCP 配置不可用返回 unsupported，开关改为持久保存并新增前端文案任务 FB-023 | closed
- 2026-05-23 10:36 | frontend-ai | answer | FB-023 已完成：Settings 与说明书 MCP 文案改为持久启用语义，unsupported 状态提示同步配置未完成或当前用户无法解密 | closed
- 2026-05-23 10:26 | review-ai | answer | 管理解锁默认有效期从 10 分钟调整为 1 天，保留显式 `a.auth.Timeout` 覆盖能力；`GetAuthState().timeout_seconds` 已由后端测试锁定为 86400 | closed
- 2026-09-07 17:24 | backend-ai | answer | FB-038 完成 v0.46.3 功能修订：空白新装配置/空规则，节点派生 RabbitMQ 账号与服务密码原子迁移，中心边缘账号 MCP 工具，旧版自有密码升级迁移，UI 脱敏测试和 MCP 保存后重启；该阶段初始包已由 FB-039 覆盖安装修订包取代，不得分发；全量门禁及包内 27 工具 smoke 通过 | closed
- 2026-09-07 16:34 | test-ai | test | FB-034 Windows 控制端真实 SSH MCP 通过；目标 Edge MySQL 改为 `127.0.0.1:3306/scada_edge` 并创建 UTF8MB4 空库，MySQL/RabbitMQ 探测 running；当前和旧版规则文件均清空、队列为 0、Agent 保持 stopped | open
- 2026-09-07 17:52 | backend-ai | test | FB-039 最终发布包哈希校正为 `DD6852F3BA87550C8C1C708A52B8507875562E322EA3FE114DB4B8A6CAB1D6F9`；重新执行真实双 NSIS 覆盖和 15 项安装器回归均通过。目标边缘机 TCP/22 可连但在 SSH banner 前断开，真实异机覆盖待 SSH 恢复 | open
- 2026-09-08 09:11 | test-ai | test | FB-034 重装后边缘机 IP 更新为 `192.168.10.105`；SSH MCP initialize、v0.46.3/27 工具、配置摘要和实时探测通过。MySQL/本机 RabbitMQ running；中心 RabbitMQ 仍为示例 `192.168.1.10:5672` 且超时，Agent stopped | open
- 2026-09-08 09:34 | test-ai | test | FB-040 本机 `192.168.10.103` 以跳过系统组件方式安装 v0.46.3，复用 Docker MySQL/RabbitMQ并新增 Canal，配置为 `server-001`；中心全组件和 Agent running，边缘已改指中心且 RabbitMQ 双连接 running。最小业务数据链路待验收 | open
- 2026-09-08 10:44 | backend-ai | answer | FB-039 至 FB-043 收口：v0.46.4 修复 Canal destination 路径、SSH 后台进程驻留和 Edge `sync_ack_log`；Go/前端/安装器全门禁通过，远端安装摘要通过，最终安装版自动数据探针 8.32 秒到达，队列和错误均为 0 | closed
- 2026-09-08 10:44 | backend-ai | answer | FB-034 Windows 异机 MCP、Agent 驻留及数据链路均完成；保留 open 仅跟踪 Mac 本机密钥和 SSH MCP 重连 | open
- 2026-09-07 17:59 | backend-ai | release | FB-037 已完成：提交并推送 main，创建并推送 `v0.46.3` 标签，GitHub prerelease 及 Windows 安装包上传成功；API 核对资产状态、大小和 SHA256 digest 通过 | closed
- 2026-09-08 11:22 | test-ai | test | FB-040 按边缘新部署状态复测：SSH stdio MCP `initialize`/27 工具/配置/规则/依赖通过，MCP 启动边缘 Agent PID 4948；探针 `1788837621242` 在 10.58 秒后到达当前 PC 中心库，两端 Agent/CDC/MySQL/RabbitMQ running，队列与失败事件为 0。Edge v0.46.4 -> Server v0.46.3 跨版本链路通过，中心升级未执行 | closed
- 2026-09-08 12:03 | test-ai | test | FB-044 Schema 演进 E2E 完成：22 项核心场景覆盖 DDL 边界、结构不匹配保留/恢复、列排除与映射、CRUD、主键映射、删表重建和跨 DDL 批次；规则已恢复，最终探针 17.18 秒通过，报告已生成 | closed
- 2026-09-08 12:03 | backend-ai | answer | FB-045 修复 append-only `INSERT IGNORE` 静默截断：改为普通 `INSERT`，单元测试、全量 test/vet/lint 和真实窄列阻塞/扩列完整恢复通过；v0.46.4 旧包不含修复 | closed
- 2026-09-08 13:59 | backend-ai | answer | FB-046 后端与测试完成：新增 5 个结构化 MCP 数据治理工具、写入审计和行数/计划确认；Canal/SyncEvent/Mapper/Apply 支持规则授权的单列 ADD/DROP，禁止自动建表/删表。隔离 Edge A 3307/5673/Canal 11121 -> Server 3309/5675 -> Edge B 3308/5674 E2E 连续两次通过 INSERT、ADD、UPDATE 新列、DROP，最终队列为 0；Rules UI 三语开关移交 frontend-ai | open
- 2026-09-08 14:15 | backend-ai | release | FB-047 v0.46.5 安装包已生成并移交 test-ai：SHA256 `FAC49E2B0B2705C64178F14796A921FF9B4A752C79B2B80507C69293F3C07297`；包内 SyncAgent MCP 32 工具真实进程冒烟通过，NSIS 两轮隔离覆盖验证旧进程停止、二进制更新、配置和规则保留。等待用户完成异机安装后验收 Edge -> 本机模拟 Server 链路 | open
- 2026-09-08 14:50 | test-ai | test | FB-047 异机 v0.46.5 安装摘要、配置/规则保留、32 工具和 SyncAgent hash 通过；MCP 启动 Edge Agent PID 7324，本机模拟 Server 受控重启为 PID 9688。重启前后两条真实探针分别 5.13 秒、11.44 秒到达，最终队列/失败 ACK/近期错误/边缘失败事件均为 0 | closed
- 2026-09-08 14:55 | test-ai | test | FB-048 启动 v0.46.5 异机高压力测试：独立 ID 区间在线递增 61,000 行，加 Server 停机积压恢复 20,000 行；核对精确行数、apply log、队列、错误和进程恢复 | open
- 2026-09-08 15:35 | test-ai | test | FB-048 完成：在线 1k/10k/50k 与 Server 停机积压 20k 全部通过；总计 81,000 行在 Edge、中心、apply log、SUCCESS event log 精确一致，50k 有效速率 452.22 行/秒，20k 恢复速率 567.92 行/秒，最终队列/失败 ACK/错误/失败事件全为 0 | closed
- 2026-09-08 15:48 | backend-ai | bug | FB-049 启动：确认 5-12 秒并非内网 RTT，而是 `retry_interval_seconds=10` 被错误复用为空闲轮询；修复范围限于 Worker 配置语义和回归测试，随后切换 test-ai 做真实异机延迟分布验证 | open
- 2026-09-08 15:53 | test-ai | test | FB-049 后端修复已通过 `go test ./...`、`go vet ./...`、`golangci-lint`；SHA256 `436B362D82E0646882F0DB07EF4821A5849B1678ACB9BA8995DB37D5527F2289` 的临时二进制已启动为远端 Edge PID 10848、本机 Server PID 52448，开始单行延迟分布验证 | open
- 2026-09-08 16:27 | test-ai | release | FB-049 完成：先拆分 Worker 空闲/错误间隔将 20 条 P50 降至约 0.81 秒，再把 Canal 1000ms 长轮询收紧为 100ms；最终 v0.46.6 包内二进制 20 条 min/P50/P95/max 为 62.569/79.745/141.854/142.122ms，数据与日志各 20、队列和错误为 0。安装包 SHA256 `71E8D3BAD677E44ECB1A5D874DCA0FAF947B8BA21CC3C89F6B746DFDE922CAC9`，Go 门禁、package smoke、32 工具 MCP 与 NSIS 双覆盖通过 | closed
- 2026-09-08 17:10 | test-ai | test | FB-050 完成：目标 `192.168.10.105` 正式注册为 v0.46.6，安装摘要 8 步通过，安装目录 SyncAgent hash 与 staging 一致；MCP 32 工具、配置/规则/依赖、PID 5040 SSH 断开驻留通过。安装版 20 条 min/P50/P95/max 为 48.015/94.728/127.240/195.698ms，四类数据各 20、队列和错误为 0 | closed
- 2026-09-08 17:28 | test-ai | test | FB-051 启动：设计真实 Edge/Server 约 13 小时无人值守双向跨夜长测，采用隔离表与成对方向规则，后台 runner 持久记录心跳、阶段、计数、队列、故障注入和最终一致性，并在结束时自动恢复原规则和 Agent | running
- 2026-09-08 18:41 | test-ai | test | FB-051 加速 smoke `smoke-20260908-183400` 全部通过：双向各 882 行源/目标计数与 CRC 摘要一致，双侧错误和三条中心队列均为 0，两次 Agent 停机恢复、双向 ADD/DROP、重复事件幂等及现场恢复均通过。正式任务 `overnight-20260908-184000` 已启动为 PID `1780`，计划运行至 2026-09-09 07:40:17 +08:00；连续心跳、双向首批与初始累计各 1,801 行已确认，随后停止主动监控 | running
- 2026-09-09 08:46 | test-ai | test | FB-051 正式长测在约 3 小时 3 分后失败：Edge -> Server 388,401/388,401，Server -> Edge 388,401/204,484，首次 Edge 停机恢复 10 分钟未追平；双侧错误为 0，中心下行队列长期为 0/1。原规则/Agent 自动恢复，9 条测试残留已清理，恢复探针约 1.50 秒通过。报告已生成，Server CDC 逐事件日志与 confirm 吞吐问题转 FB-052 | closed
- 2026-09-09 10:00 | backend-ai | answer | FB-052/FB-054：批量分发、确认保序、失败源回滚和批内幂等修复完成；选择性 NACK 审阅闭合，全量 Go 门禁通过；无新增前端 DTO/UI 需求 | closed
- 2026-09-09 10:00 | test-ai | test | FB-053 最终短测失败如实归档并转 FB-054；v0.46.7 包内 4+4 重放、32/64 位安装回归、NSIS 双覆盖与实际解包通过；跨身份修正封包夹具 NoBuild 命名参数与版本元数据。原双端规则保留，Server 旧版从备份路径运行 | closed
- 2026-09-09 10:00 | test-ai | decision | FB-055 七小时宽表方案与 schema 设计完成，强调真实 UPDATE、50 列填充/更新/比较、独立生产时钟和提交账本；未实现完整新编排器、未升级边缘机、未启动长测 | open
- 2026-09-09 13:17 | test-ai | test | FB-056 用户安装 v0.46.7 后连通验收通过：安装 hash/注册版本/8 步摘要、SSH MCP 32 工具、MySQL 与两端 RabbitMQ、20 条真实同步及精确数据/日志核验通过；启动 Edge Agent PID 1516，SSH 断开后保持运行；Go test/vet/lint 通过 | closed
- 2026-09-09 13:17 | test-ai | question | FB-057 安装实包为 v0.46.7，overview/MCP 仍硬编码显示 v0.46.6，转 backend-ai 统一版本来源；不影响本次连通结论，未现场改代码或配置 | open
- 2026-09-09 14:10 | test-ai | test | FB-055 新50列短测各11460 INSERT+5730 UPDATE后受控停止；十分钟未完成，正式七小时未启动。已登记FB-058数字精度、FB-059真实双向版本回退，夹具缺失门禁保持WIP，正式Start已加拒绝保护 | blocked
- 2026-09-09 14:10 | test-ai | test | 原规则hash与Agent路径恢复，642条本run残留经逐条身份检查/confirm后移至持久隔离队列，正常队列0；恢复上传探针3/3、80.786-116.356ms通过。全量Go test/vet/lint（0 issues）、PS parser及拒绝门禁通过。产品修复/重打包待用户答复，不把单元门禁当作宽表验收通过 | blocked
