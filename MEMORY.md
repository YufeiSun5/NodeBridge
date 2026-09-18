# MEMORY

Last updated: 2026-09-18 Asia/Singapore

## 当前阶段
- 2026-09-18 backend-ai：v0.48.10 Beta Release已发布，tag b9c213c，安装包/校验文件服务端验证通过；用户反馈现场测试无问题，按用户验收反馈记录，历史偶发超时保留跟踪。
- 2026-09-18 backend-ai：0.48.10补新机系统库初始化入口与upgrade-system -create-database，显式建库/系统表，升级失败输出脱敏原因。两角色缺库及旧库重复升级、Go全测/vet/lint、Wails三语交互通过；65文件5资产及覆盖安装核验通过，已出包，现场未部署。docs/v0.48.10-fresh-database-handoff-20260918.md。
- 2026-09-18 backend-ai：0.48.9修复Windows批处理正则传参exit255并出包；原版Windows RabbitMQ实链、包内MCP配置账号/权限读回、44工具、65文件5资产/覆盖升级、Go全测/vet/lint通过。源码/包内修复closed，现场重试与迁移验收仍open；docs/v0.48.9-windows-rabbitmq-handoff-20260918.md。
- 2026-09-18 review-ai：Windows批处理权限正则传参缺陷已复现，同现场exit255；NB-ALIGNMENT-PERMISSIONS reopened，先前Docker权限测试不覆盖bat。产品未改、现场未连接；修复与Windows实际链路验收待backend-ai。详见docs/v0.48.8-windows-rabbitmq-review-20260918.md。
- 2026-09-17 backend-ai：0.48.8已出包，补对齐RabbitMQ权限、旧计划诊断、双向计划MCP预览/受控重建，105权威源；旧系统库保留、目标事务备份清空、新代际重对齐、旧epoch审计。Go全测/vet/lint、包内双端E2E连续两轮、系统库升级/覆盖安装、44工具及65文件22迁移5资产通过。一次中间update-1超时原因未定，现场未部署未恢复，NB-MIGRATION-105-108仍open；详见docs/v0.48.8-rebaseline-handoff-20260917.md。

- 0.48.7自动重连补丁交付：NB-RECONNECT固定旧RabbitMQ对象无限重试已修，Session失效重建且保留原ACK/确认/Canal位点边界；两轮三节点批量16服务停止恢复与通信暂停20秒自动补齐，1.058–1.465秒（测试重试1秒），确认未知/旧ACK隔离通过。全测/vet/lint、Wails/CLI、安装/系统库升级及62文件20迁移5资产哈希通过。包build/NodeBridge-beta-v0.48.7-20260915.exe（5A1FDF8F…），报告docs/v0.48.7-reconnect-handoff-20260915.md。backend-ai跨test-ai，仅独占测试，资源清理，业务配置规则未变；现场复验/初次断线诱因/整机重启与其他历史项待验。

- 0.48.6批量漏转发补丁已交付：修复已提交前缀跳过dispatch却ACK，转发失败重入队；旧分支确定性失败、新分支通过。三节点批量16连续10批1056CRUD通过，53个死锁错误批次重试后最终三端30行摘要一致；test/vet/lint、安装/旧库升级及62文件20迁移5资产解包校验通过。包build/NodeBridge-beta-v0.48.6-20260915.exe（B1BC08DD…），交接docs/v0.48.6-batch-relay-handoff-20260915.md。backend-ai跨test-ai，仅隔离测试；历史缺行不自动补回、死锁竞争/其他历史项仍open，业务未部署。

- test-ai七小时长测批量1已通过：741批/80004次CRUD，三端摘要一致，两次10分钟离线恢复与结束门禁通过。run dcee70b6467447519b6ee636420eefee，22:07:32至05:07:38；test-output/evidence/supervisor全部通过，原配置/规则哈希未变，独占进程/容器/网络已清理。首轮批量16缺行未修、三库SQL保留，FB-094/107仍open，FB-084/109不变。同Windows三逻辑节点及递增测试时间限制见docs/soak-7h-20260914.md。

- 最新0.48.5交付：可选规则name突出显示，原id只读且兼容旧对齐计划/证明，名称修改不要求重启，CAS仍覆盖完整文件。安装器复用/安装组件均执行系统库upgrade-system，记录迁移checksum和版本，失败阻断完成，初次未配置明确跳过。两角色独立MySQL旧库重复升级保留ACTIVE/原数据、Go全测/vet/lint、Wails/绑定/MCP、6安装场景+17回归、三语三宽度与62文件20迁移5资产解包哈希通过。安装build/NodeBridge-beta-v0.48.5-20260914.exe（4C8695C2…），交接docs/v0.48.5-upgrade-handoff-20260914.md。backend-ai跨frontend/test，未现场升级，FB-094/107/084/109历史项不关闭。

- 最新0.48.4修复双向下行目标业务库被系统默认库覆盖及Canal逗号过滤括号错误，保留单向中继回退和0.48.3小屏UI。test/vet/lint、Wails/绑定、CLI/MCP、解包62文件/20迁移/5资产与便携ZIP21文件哈希通过；无新增迁移/业务字段，现场未部署未复验。安装/便携包build/*v0.48.4*，交接docs/v0.48.4-downlink-handoff-20260914.md，FB-107本轮代码修复closed，现场与历史风险pending。

- 最新0.48.3规则小屏优化：搜索列表+单条编辑/只读四页签，窄屏选择框、顶部保存和按需说明，保留鉴权/CAS/首次对齐/预检。zh/en/ja四视口48组及草稿/增删保存/只读/冲突检查通过；test/vet/lint、Wails构建/绑定通过。安装包build/NodeBridge-beta-v0.48.3-20260914.exe，交接docs/v0.48.3-ui-handoff-20260914.md。frontend-ai，跨backend仅版本/封包，未现场安装，同步内核沿用0.48.2，历史FB-094/107/084/109不关闭。

- 最新0.48.2已交付：修复无关失败对齐阻塞单向启动、MCP/UI进程ready确认和三节点远端映射误裁；同表未完成账本与完整ACTIVE保护保留。最终Agent1EE3945E…，三节点27.97秒、单向上/下18.23/16.57秒、无标记双节点49.14秒通过；test/vet/lint、Wails/绑定、17安装回归、MCP及解包62文件/20迁移/5资产通过。
- 安装包build/NodeBridge-beta-v0.48.2-20260914.exe（D8212312…，340882572字节），便携包build/NodeBridge-agent-v0.48.2-20260914.zip；替换同日10:33未验收候选。交接docs/v0.48.2-startup-handoff-20260914.md。backend-ai，本轮FB-107启动修复closed，现场验收pending；FB-094/107时钟与历史长测、FB-084/109仍open。未现场安装/迁移/业务写入、未恢复长测或Git提交，本轮测试资源已退出。

- 最新0.48.1已出包：双向不再要求或覆盖业务last_event_id/updated_by_node，改同事务系统标记和持久CDC边界；两端须新版Agent与010系统库迁移。最终包内程序无标记表双节点50.89秒/三节点28.05秒通过，test/vet/lint、MCP与封包门禁通过。安装包build/NodeBridge-beta-v0.48.1-20260914.exe，便携包build/NodeBridge-agent-v0.48.1-20260914.zip。未现场安装或业务写入；业务AI接续说明docs/v0.48.1-replay-handoff-20260914.md。backend-ai，字段兼容性实现closed，现场验收pending，时钟与FB-094/107历史项仍open；下方“尚未修复”由本条覆盖。

- backend-ai最新定位：普通业务表sys_detection_standards被首次对齐硬性要求last_event_id/updated_by_node阻断，0行复制。回放写入、删除及回环识别均依赖业务行标记，属于NodeBridge无侵入兼容性缺口；不改业务表，不仅删除门禁掩盖问题，产品整改尚未完成，FB-107 open。
- test-ai本次垃圾库清理完成：17个已废弃cr_lite迁移集成测试库备份1675277字节后删除，查询确认剩余0；远端只剩系统/业务及同步元数据库，未删业务库。共享历史压测表和日志仍待定向清理，不能称全部清完。证据.cache/cleanup-20260914/stale-databases*。

- 最新状态覆盖下方运行中记录：用户要求停止测试、让业务AI先使用；自动复查已删除，不自动恢复。长测实际仅08:47:03至08:55:22，58批/最后记录2860次操作；连接中断后两端各690行但摘要不同，离线场景未执行，不能报通过。用户确认主站Docker由其启动；业务组件/MCP保持运行，时钟问题仍open。
- 已清理4个本轮隔离库（先导出，约38.55MiB）、2个测试vhost、2个独占Canal容器及远端测试运行副本；11个重复解包目录释放4259105445字节。证据.cache/cleanup-20260914。随后直接核查发现历史测试表仍占两端合计6731.12MiB（227张），尚未清理；主站共享事件日志19509.45MiB，主站/边缘共享应用日志1714.63/1573.58MiB，不能将这些整表视为全部可删。test-ai负责后续定向清理，详见AI_BOARD顶部；FB-094/107仍open。

- 用户要求先实测，不先改产品/系统时间。安装版七小时长测已于2026-09-14 08:47:03启动，计划15:47:03结束写入并最多10分钟收敛；run `.cache/soak048/20260914a`，远端 `C:/NodeBridgeSoak/20260914a`。100行首次全量与基线摘要一致；supervisor38252/writer28032/本机Agent42856/远端Agent12388已核验存活，写入110→460→710递增。第2/4小时各暂停一端Agent600秒；heartbeat自动化`nodebridge`已ACTIVE，约15:48复查并在终态后删除。
- 本轮只交换独立测试逻辑角色（本机Edge、远端Server），保留现场约30秒时差，原业务配置与安装二进制不变。原角色`alignment_plan_expired`失败保留，不能以本轮运行关闭时钟不准缺陷，更未验证±10分钟。写入中两次摘要抽查有1至3行瞬时差异，等待停止写入后的最终收敛；FB-094/107长测进行中，时钟缺陷open。Go test/vet/lint通过。旧20260913a失败资源已停并留证据，不再等待校时批准。

- test-ai正在准备用户授权的两端安装版0.48七小时长测；run `.cache/soak048/20260913a`，边缘 `C:/NodeBridgeSoak/20260913a`。两端Agent哈希确认25B18D...；独立数据库/消息空间/Canal，不借原Canal固定client1001推进业务游标。测试表排序规则差异已修，首次对齐仍被约30秒系统时差拒绝，已询问是否允许同步边缘全局时钟，未获答复不改时钟。长测尚未计时、无7小时复查自动化；FB-094/107待验证。新增测试夹具Go test/vet/lint通过，不改产品行为。

- 0.48.0已交付：`build/NodeBridge-beta-v0.48.0-20260912.exe`，340838916字节，未签名；SHA256 `775BD96C1CF41E4ABDBB47959A879C40C343EADDCAE3A9B9543FACFBD198BE23`。60个解包文件、18迁移和5组件哈希通过。当前backend-ai，按出包即停止条件结束本轮（19:39开始，约1小时半）。不部署业务环境、不使用E:/F:、不自动Git提交。
- 多成员串行复制与完整PENDING/READY/ACTIVE证书、稳定来源epoch、所有接收端重放首次边界后的版本/墓碑已接通；已有配置范围不静默扩大。三独立MySQL/Canal/Agent的异名、共享规则同名、全空、中心源/边缘源200MiB、部分已提交中断恢复、复制间隙写入/删除、离线/重启/回执失败等短测通过。最终候选200MiB+间隙写入93.24秒：.cache/multi-node-sync/ad4dcb64a97b4121b9321160398b97b4；双节点兼容55.46秒：.cache/canal-business/dbdc73483f344112b987da5e736a750e。
- 普通MCP42工具5资源；新增start/status/interrupt，重试复用start，所有节点须保持stdio并轮询完整组completed。规则页提供中心多选与三语，12组角色/语言/宽窄屏通过；全量Go test/vet/lint、Wails构建/绑定、五安装模式、32/64位各17回归通过。
- 原始值上限256MiB/编码512MiB/每复制15分钟/整组1小时；实际短测三节点和200MiB，不承诺任意N规模。Canal默认512MiB堆OOM与16MiB未ACK缓存堵塞已取证，受管容量补丁提高为至少2GiB堆/512MiB缓存；复用/外部组件须另外核对和维护重启。不支持在线扩容、已有冲突版本/墓碑向新成员迁移或超大单行分片，源binlog秒级LWW仍依赖可靠时钟。
- 包内Agent SHA256 25B18D6040574CE1CC7BA4D15CC2D8CDE68CEC104C2C119B755E1CEF66324445；UI B5BBC7721AF6B0EBC63BDFC4AC85696286799C17F8D4310003F87C02D7218DF1。旧二进制已备份，旧安装包保留，测试容器/Agent/5173服务已清理。FB-107本轮交付完成，旧历史6f813a/FB-094/084/109不关闭。交接见`docs/v0.48.0-upgrade-handoff-20260912.md`。

## 上一交付：0.47.0

- 0.47.0已交付：`build/NodeBridge-beta-v0.47.0-20260912.exe`，340732348字节，未签名，SHA256 `FA23D8A9CAF8626FD06F1FD11CC8EAE59B6AF7520F5332F1ABB82BC20F00F1DF`。双节点手动首次全量、持久安全接续/恢复、双向LWW和既有规则页三语入口已接通；公开能力true，online=false，无新MCP工具。
- 两独立MySQL+双Canal/双Agent三种起点短测全部通过（server57.05秒、edge58.56秒、empty54.11秒）；全量Go test/vet/lint、前端build/binding、三语1366/390交互、五安装模式及32/64位各15回归通过。58解包文件、16迁移和5组件资产哈希一致。普通stdio39工具5资源，隔离不可用连接诊断预期partial，不是业务连接验收。
- 范围仍限一Edge一Server、64MiB/两分钟、两端停Agent；源binlog秒级LWW依赖可靠时钟。双方非空拒绝覆盖。无在线/多节点/大表续传。按出包即停止条件结束，无业务安装部署、无长测、无USB盘访问、无新Git提交。测试容器及本轮5173服务已清理。
- 当前身份backend-ai，FB-107本轮功能已交付但历史数据/早期6f813a超时残项仍open；FB-094/084/109保留。交付证据与安装步骤：`docs/v0.47.0-upgrade-handoff-20260912.md`，操作指南：`docs/initial-alignment.md`。

## 本轮中间记录（历史，不代表最终状态）

- 最新封包进展：0.47.0候选已构建，CLI强制每条启用双向规则匹配ACTIVE证明；Server预检使用实际本机目标表。两个独立MySQL实例+双Canal/双Agent的server/edge源首次全量与后续双向短测57.05/58.56秒通过，含真实stdio规则启停保存与旧消息状态；三语1366/390交互/截图、全量Go测试、安装暂存及五模式保留配置/pair/status侧文件通过。最终空起点双MySQL短测、安装回归与NSIS压包仍待完成，尚未交付。下方旧“未接UI/能力false”等状态由本条及AI_BOARD最新记录覆盖。

- 用户新指令启动第二轮实施，2026-09-12 16:33开始，交付安装包或21:33到达5小时即停止。精简为双节点明确映射、受限首次全量与安全增量接续、LWW、必要Wails操作；多节点编排/大表续传/新MCP工具后置。仅短测、不部署业务环境，FB-107继续open；上一轮未出包的事实不变。

- 第二轮增量进展（覆盖下方上一轮状态）：已补实际Canal网络期限/取消和ACK写失败；008交接证明与历史消息审计、两端READY/ACTIVE、捕获边界过滤、新事件命名空间、仅已知配对旧消息持久留档后ACK。临时RabbitMQ控制会话串起复制/已提交恢复，复用本机Agent锁，新增initial-alignment CLI和待前端接入的Wails异步操作/状态/中断接口。捕获/提交后恢复/就绪/留档14.16秒通过；首次全量CLI接真实双Agent短测进行中，不能以中间断言当整轮通过。提交前失败的安全重规划、正式UI、全量门禁及新安装包仍待完成，capabilities尚未开放。
- 用户禁止使用移动硬盘：USB盘E:/F:不访问、不作缓存/临时/测试存储；仅使用内置C:/D:。已确认Go缓存/临时目录及Docker WSL配置在C:/D:。

- 当前身份 backend-ai；跨 test-ai 仅单元、脚本、独占数据库/队列/候选进程及 UI 测试夹具；此前跨 frontend-ai 仅事件状态三语与绑定。AI_BOARD.md 是唯一活跃看板。
- 用户要求实施首次全量/同表双向，功能测试通过后出包；用户自行安装，长测安装后再说。停止条件是首次出包或本目标累计5小时，先到即结束。不得自动部署、启停业务Agent、改业务配置/表或处置其他AI残留。
- 首次全量规则：手动，有数据端复制到空表端；两空跳过，两端非空拒绝合并/覆盖；按实际表/列/主键映射。双向以源事件时间优先，不能用接收/处理时间。严格业务指令时间字段与删除时间来源仍待确认，当前内部实现使用源binlog事件时间及确定性平局排序。
- FB-107内部LWW已接入配对清单CLI运行：实际schema复核、分离Capture/Incoming、完整可写列双射、来源/分发范围约束、共享Canal屏障、本地版本登记、获胜镜像/墓碑与loser修复、硬删除专用回放证明、幂等历史收据。三节点仅映射/序列化回归，不等于三节点实际运行验收。
- 双Agent/双Canal/RabbitMQ真实异名精度CRUD、保留标记本地写入、离线交错源时间获胜、6轮交替来源删除/墓碑/repair_pending清空、重启、1644回执/本地登记失败重试、实际对齐启动门禁通过51.36秒；证据.cache/canal-business/3cc9ef70f1f44eba958feddad402df3f。
- 首次对齐已有本机UTC快照、1MiB数据帧、64MiB受限RabbitMQ传输、007持久作业、SOURCE_READY提交前摘要与目标原子收据。源锁保持到目标收据返回，目标未提交行不外泄；NULL/空串/BIGINT/DECIMAL/二进制/DATETIME/TIMESTAMP(6)无损。
- 两端探针不ACK，源表锁内写/观察源标记，目标标记与复制行/收据同事务。提交后边界失败可重连观察原标记，不能用新脉冲替代；核对结果幂等确认不解除Agent阻断。普通过期计划禁止新复制，已提交目标允许恢复探针。所有作业phase仍阻断Agent，没有COMPLETE。
- 真实双Canal/RabbitMQ捕获联动和提交后1644故障恢复7.64秒通过：.cache/canal-business/df37b685afe24bb7b7e6c3d988b5f684（同一物理MySQL两个逻辑端点）。另两台物理MySQL的传输正反向/回执失败/中断4.46秒通过：.cache/alignment-transport/afdef4425008412f8f627aff967f6c09。SQL核心含源/目标锁覆盖、源锁保持到收据、收据丢失/摘要篡改/幂等恢复6.61秒通过。
- 当前全量go test/vet/lint、前端build/binding通过；三语1366/390状态与溢出回归通过，已检查英文桌面与日文窄屏截图，output/playwright/business-remediation/。UI状态区分superseded和recorded_outcome_unknown，不把loser记账当写入获胜行。
- 尚未闭合：CDC旧事件过滤/完整血缘与重置验证、全部参与节点启用握手、产品维护租约编排、自动schema配对、公开Wails/MCP执行/恢复流程、大表持久暂存及队列审计清理。已有版本/墓碑不能清空；不得删作业账本来解除启动阻断。无新包、无部署、无长测；initial_alignment.execute/sync.bidirectional仍false。
- 风险：较早.cache/canal-business/6f813aaa4c2d43f888beeaccc6f8efe5收敛超时缺SQL现场，未复现不等于根因已修；后续已补失败快照。withlin/canal-go v1.1.2底层socket没有实际deadline，context超时不能宣称硬终止网络I/O；公开维护入口前仍需处理。SQL锁探针已改显式未提交事务，避免把客户端超时误当作autocommit未执行。
- 详细实施与证据：docs/alignment-bidirectional-progress-20260911.md。FB-107保持open；FB-094旧长测问题另行验收。

## 上一已交付包：0.46.16

- 不是本次首次对齐/双向功能包。build/NodeBridge-beta-v0.46.16-20260911.exe，339896994字节，未签名；SHA256 0EB2142378774225E6E23CFBF14AC945DB48B868B63BD7094D2384732A6116A7。记录docs/v0.46.16-upgrade-handoff-20260911.md。
- 已交付严格INSERT、缺行/同值UPDATE、HARD/SOFT删除、实际PK/结构/权限检查、规则CAS/saved/active revision、事件状态、禁用规则残留不ACK、受控队列隔离plan/apply/audit、跨规则库治理、MCP无损数字/时间脱敏。
- SERVER_TO_EDGE明确target_database_name优先，转发保留源事件语义，不假设同名库表列。Canal二进制高位字节以rowvalue.Binary无损传输，中心和关联边缘须同包升级，旧数据不自动修复。
- 普通MCP 39工具/5资源，不增加申请/角色/范围授权；写入与队列隔离仍需明确确认。撤销未发布VPN/HTTP/token/SYSTEM入口，沿用SSH+stdio；用户账户DPAPI和SSH服务不改。
- 上一包Agent 16C571C9BA73D629C4D74AAD29844307DF22B2B90A4A1252F696FDFBE5AD39AB；UI BCE48F851F1A8E77C6DA69BEAB8135A40B6B620B6874DC41E8D1AE8B391F6E72。
- 上一包真实上下行CDC、SSH公钥stdio、错误日志、五安装模式、32/64位各15项、UI恢复9项模拟、两次覆盖升级/配置哈希与43文件/5资产解包验证通过；.cache/v04616-release/package-gates.json。不是长测、真实原桌面恢复或本次首次对齐验收。
- 上一包测试资源已清理，未操作业务现场。0.46.15此前用户已本机安装；不据此推断0.46.16当前安装状态。

## 安装与后续

- 中心与边缘同一Windows x64包；默认复用组件、保留连接/规则/密码，不安装MySQL，不自动Docker探测。
- 升级前备份并停同步、显式退出旧UI；安装器只停所选安装目录进程，工作区副本由操作员在原入口停止。DPAPI密文不可跨机器/账户搬运，SSH保持配置所属账户。
- docs/mcp-business-ai-handoff.md；上一包客户端配置build/NodeBridge-mcp-client-v0.46.16.json，地址须现场核对；凭据仅查docs/test-credentials.md，不打入安装包。
- FB-108撤回VPN/SSH验证closed、FB-089可信时间修复入旧包closed；FB-107整体open，FB-094旧七小时125秒失败未完整复验，FB-084真实UI恢复待安装后验证，FB-109既有npm依赖风险未处理。
- 不将旧七小时失败、容量/延迟风险或失效formal-ready改写为通过。历史地址192.168.10.103/105、进程ID及规则状态均非实时依据，操作前核对。
- 历史MEMORY及阶段流水归档.ai/docs/changelog.md；稳定合同.ai/docs/frontend-backend-contract.md，活跃问题只写AI_BOARD.md。

## 改动记录
- 2026-09-18 | Codex | backend-ai：新机缺库初始化及错误输出修复，跨frontend/test完成入口与隔离回归。
- 2026-09-18 | Codex | backend-ai：0.48.9 Windows批处理权限修复，原生实链与包内MCP验证通过并封包。
- 2026-09-18 | Codex | review-ai：本地复现权限配置Windows批处理失败，重开缺陷并明确目标备份不等于活动库合并。
- 2026-09-17 | Codex | backend-ai：完成0.48.8双向计划迁移补丁和隔离验证出包，保留现场验收及偶发更新超时跟踪。
- 2026-09-16 11:48 | Codex | review-ai：三语README增加多节点拓扑和路由示例；NONE/ACTIVE_EDGES/SELECTED_EDGES及节点映射说明，Go门禁通过。
- 2026-09-16 11:42 | Codex | review-ai：三语README突出列映射/双向/MCP，增加架构与真实Windows入门；CI/Demo/软件源/签名列为待实施计划。
