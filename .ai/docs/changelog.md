# Changelog Archive

## 2026-09-12 01:30 MEMORY 历史快照

以下为本次阶段收敛前的流水。旧限制和进行中状态以当前MEMORY/AI_BOARD为准，不表示功能已发布。

# MEMORY

Last updated: 2026-09-12 01:20 Asia/Singapore

## 当前阶段

- FB-107新增受限RabbitMQ快照传输、两端真实Canal标记与持久边界、SOURCE_READY提交前摘要、收据核对恢复；目标标记/收据/业务行同事务，重连恢复只能观察原标记，不能以新脉冲替代。真实双Canal/RabbitMQ复制及提交后1644边界失败恢复10.03秒通过（.cache/canal-business/d53e80054a2c4413b7287d9c2122dd2f），SQL核心5.43秒通过，全量test/vet/lint通过。所有作业仍阻断Agent，无COMPLETE/CDC切换/产品维护入口，未出包/部署/长测。
- 真实双向增强6轮交替来源离线删除/墓碑/修复清空及Agent对齐门禁51.34秒通过（.cache/canal-business/fa28cfb4b29b4acaa4c7b60402ba1f64）。较早6f813a超时未保留SQL现场，不能宣称根因修复；后续已补故障现场，并修正同秒跨节点平局导致的测试错误前提。截止条件仍为首次出包或累计5小时。

- FB-107新增各端本机DB的受限快照数据帧及接收事务，实际编解码/异名正反向/损坏乱序重复中断回滚通过；007对齐作业账本将目标收据与业务行同事务，源确认另行持久化，作用域按MySQL实际大小写模式锁定。真实SQL故障/回执丢失查询/启动阻断核心5.43秒通过，仍无RabbitMQ传输适配、恢复操作、CDC交接或公开执行入口。两类Agent新增未完成作业启动门禁，实际进程回归正在验证；未出包/部署/长测。

- FB-107真实双Agent/双Canal/RabbitMQ配对LWW短测27.21秒通过：异名精度CRUD、双方离线交错写/删除、重启、已观察回执失败重试、本地登记失败后不重启Agent重连、修复清空及SUPERSEDED实查。CLI run基于显式配对清单、准确规则比较和本机实际schema复核后绑定不可序列化运行规则；UI/MCP普通启用仍关闭，未自动采集远端schema，首次对齐传输/CDC交接未接通。未出包/部署/长测。
- 修复生产参数插值下冲突JSON被当二进制、Canal关闭连接复用、YAML/JSON空集合规则误判。事件状态/三语显示superseded与recorded_outcome_unknown；禁用HARD双向草稿可保存但不能直接启用。离线复制UTC专用会话支持TIMESTAMP(6)，+09/-04时区/NULL/池隔离真实SQL通过（核心4.45秒）。全量test/vet/lint与前端build/binding测试通过；未进行新的UI截图验收。

- FB-107新增多端点双向图编译，分离本机Capture与入站Incoming，组合异名Edge转发映射，歧义/矛盾schema/策略拒绝且不扩大分发；三节点CRUD普通/批量序列化与test/vet/lint通过。远端schema交换、Agent自动解析和首次全量交接仍open，门禁未解，无新包/部署。

- FB-107新增实际schema驱动的双向三投影（上行/反向/边缘原名转发）和节点范围检查，对齐计划复用；完整列双射/PK顺序/生成列/软删除及真实SQL精度短测通过（4.41秒），test/vet/lint通过。公共运行门禁不变，Agent配对解析/首次全量传输交接尚未接通，无新包/部署。

- 用户更新停止条件：出包或本目标累计5小时，先到即结束；当前约2小时，未达到停止条件。仍不部署/长测。
- FB-107已接通受现有门禁保护的Edge/Server Agent冲突组件构造：CDC/LocalRecorder/SQLWorker/RepairWorker共享屏障，订阅追加本机脉冲表，后台修复复用Worker重试/状态。构造/调度单测、真实Canal上下行12.24秒/12.16秒及全量test/vet/lint通过；完整双向规则配对、细粒度诊断和首次对齐仍未完成，不出包。

- FB-107内部本地冲突恢复已实现：006持久获胜镜像/待修复任务/专用回放证明，行锁+新鲜屏障后恢复当前winner且不生成新业务版本。真实Canal上下行、串行SQL软删除与全量test/vet/lint通过；生产调度/诊断状态/首次对齐未接通，不出包/部署。

- FB-107内部SQLWorker已接入LWW+实际Canal屏障，冻结原始事件/验证投影、锁行后等捕获、版本/业务/回执同事务；真实上下行夹具覆盖新旧/重复、回滚、墓碑/复活、缺行和批次，test/vet/lint通过。生产构造/门禁、获胜状态与本地loser恢复、首次对齐仍未完成，不出包/部署。

- FB-107新增实际SQL主键规范化/键结构固定、历史事件身份收据及LocalRecorder；真实Canal验证业务行锁下版本持久化先于屏障返回，真实MySQL与test/vet/lint通过。旧本地写入输给账本版本时现显式阻断，获胜数据恢复、生产双向Apply/首次对齐链路仍未完成，不解除门禁、不出包/部署。

- FB-107新增capture随机脉冲/完整批次确认屏障及004迁移，Canal上下行增加发布前LocalVersions登记接口；真实Canal验证持有业务行锁仍能确认此前变更、纯脉冲无检查点反馈，test/vet/lint通过。登记器/规范SQL行身份/生产双向Apply仍未接入，capabilities不变，未出包/部署。

- FB-107新增双向硬删除内部回放证明路径（003_delete_replay迁移）：标记/专用收据/删除/apply日志同事务，专用锁定收据查询；普通单向删除不加UPDATE。上下行真实Canal分别验证删除回放抑制、本地保留旧INSERT标记删除不误丢、收据失败全回滚；复制回归及test/vet/lint通过。双向运行门禁仍关闭，未出包/部署。

- FB-107新增回环过滤修复：UPDATE保留旧同步标记时仍上传，标记改变才查回放，缺必要FULL前镜像返回错误。候选Agent真实Canal/Rabbit/MySQL上下行均验证远端回放INSERT过滤及随后本地UPDATE唯一收据；test/vet/lint通过，夹具清理，未出包/部署。同表双向/HARD DELETE来源和首次对齐运行入口仍未完成。

- 用户已要求开始首次全量/同表双向实施：手动有数据端到空表端、两空跳过、两端非空拒绝；双向按源端指令/事件时间较晚优先。新增alignment计划和内部离线CopySnapshot复制原语、conflict版本/事务核心及002_row_version迁移；独占MySQL正反向映射复制/精度/失败回滚/24并发/墓碑测试和全量test/vet/lint通过。详见docs/alignment-bidirectional-progress-20260911.md。尚未接通跨节点传输/产品执行入口/CDC交接或生产双向Apply，TIMESTAMP适配未完成，capabilities保持false、MCP工具不变，未重新出包/部署。严格指令时间来源仍待确认。
- 当前身份 backend-ai；跨 frontend-ai 仅 Wails 三语界面/绑定，跨 test-ai 仅独占配置、数据库、队列、候选进程及安装器夹具。
- 用户最新目标：交付升级后的安装包；用户自行安装，长测安装后另行安排。不得自动部署、启停业务 Agent、改业务配置/表或处置其他 AI 的残留。
- 用户撤销未发布的 VPN/HTTP/token/SYSTEM 远程入口，沿用 SSH + stdio MCP。代码、配置字段、加密范围迁移、安装选项/任务、防火墙及专用 UI 已撤回；现有账户 DPAPI 与 SSH 服务不改。
- 0.46.16升级包已交付：build/NodeBridge-beta-v0.46.16-20260911.exe，339896994字节，未签名。SHA256 0EB2142378774225E6E23CFBF14AC945DB48B868B63BD7094D2384732A6116A7；UI BCE48F851F1A8E77C6DA69BEAB8135A40B6B620B6874DC41E8D1AE8B391F6E72。记录docs/v0.46.16-upgrade-handoff-20260911.md。
- 已实现：严格 INSERT、缺行/同值 UPDATE、HARD/SOFT 删除、真实目标 PK/结构/权限检查、规则 CAS/saved/active revision、事件状态、禁用规则残留不 ACK、受控队列隔离 plan/apply/audit、跨规则库治理、MCP 无损数字和可信时间脱敏。
- SERVER_TO_EDGE 明确 target_database_name 优先于 Edge 默认库；EDGE_TO_SERVER 转发仍保留本机落库兼容规则。映射保留源事件语义，不假设同名库表列。
- 真实 Canal 测试发现并修复二进制高位字节 UTF-8 重编码。新增 rowvalue.Binary，按 JDBC 二进制类型从 Latin-1 还原字节，行值以 {"$nodebridge_binary_base64":"..."} 经过 JSON/队列/回放，SQL 使用原字节。中心与关联边缘必须同包升级，旧数据不自动修复。
- 普通模式 MCP 已开放39工具/5资源，不增设申请、审批、角色或范围授权。原 -lab-full-access 仅兼容未配置引导；结构化数据写入与队列隔离仍要求明确确认。
- 首次对齐执行/在线交接、双向冲突仲裁和历史误账修复引擎未交付，capabilities 如实标记不支持。默认对齐 DISABLED，MANUAL 不等于执行或授权。
- 已启用的 BIDIRECTIONAL/SERVER_WIN/LAST_WRITE_WIN 明确拒绝，不将此前接受配置误称能力已实现。旧规则原文保留；升级前必须检查，不能擅自改成其他业务方向。

## 本包验证

- go test ./...、go vet ./...、golangci-lint 0 issues 通过；Wails生产构建、前端tsc/vite、binding契约及三语1366/390 UI回归通过。
- 独占 MySQL/RabbitMQ：严格写入/软硬删除/缺行失败重入队/精度/回滚/实际预检/MCP治理/队列隔离/禁用规则保留通过。
- 最终 Agent 16C571C9BA73D629C4D74AAD29844307DF22B2B90A4A1252F696FDFBE5AD39AB 在独占容器实际运行：上行与下行分别验证映射 BIGINT/DECIMAL/NULL/空串/二进制/微秒时间 CRUD、Edge/Server 重启及精确3条 apply 收据；不是长测或同表双向冲突验收。
- CDC证据：.cache/canal-business/19f8199c38d34e278d525c47e6f4d84b（上行）、27d9f49611f943ce858ea5ac77b8f6a0（下行）。真实普通stdio MCP版本/39工具/5资源/能力/4步MySQL诊断通过。
- 同一 Agent 经已有 SSH 公钥登录在边缘独占临时目录完成握手、ping、39工具/5资源/能力读取，远端目录已删除；未替换安装版或读取业务配置。证据 .cache/v04616-release/ssh-mcp.json。
- 最终Agent启动ERROR持久化、MCP工具/资源读日志及数字密码不破坏PID通过，证据 .cache/v04616-release/runtime-logs/evidence.json。
- 五模式安装夹具通过 .cache/component-mode/d74027bee62941f6a58d03d691104dcd；32/64位安装回归各15项，UI恢复9项模拟通过，真实原桌面恢复未验收。
- 新UI的隔离两次覆盖升级通过 .cache/nsis-upgrade/20260911-155048-406/upgrade-evidence.json，配置/规则原文保留、旧测试进程停止与二进制覆盖通过。
- 最终重封EXE已重新解包，43打包文件/5离线资产、最终Agent/UI、升级证据及普通MCP版本全部匹配，.cache/v04616-release/package-gates.json passed。
- 真实测试容器、网络和候选进程均清理；此前独占13367 MySQL、45054 RabbitMQ已删除，4971 UI测试服务器已停止。业务现场未动。

## 安装与后续

- 中心与边缘使用同一个 Windows x64 包；默认复用现有组件，保留连接/规则/密码，MySQL不由安装器安装，不自动Docker探测。
- 升级前备份配置/规则并停止同步，显式退出旧UI；安装器只停止所选安装目录进程，工作区副本须由操作员在原入口停止。
- 配置用户DPAPI不可跨机器/跨账户搬密文。SSH保持配置所属账户；公钥可分发到多台目标，私钥留控制电脑。
- 交接 docs/mcp-business-ai-handoff.md；客户端配置 build/NodeBridge-mcp-client-v0.46.16.json（本机中心+既有边缘SSH地址，需要按实际部署核对）。
- FB-108撤回VPN/SSH验证closed，FB-089可信时间修复入包closed；FB-107整体仍open（首次对齐/历史误账等未交付）。FB-094旧七小时125秒故障仍未完整复验，FB-084真实UI恢复待安装后验证，FB-109既有npm依赖风险未处理。
- 不将旧七小时失败、曾有的容量/延迟问题或旧 formal-ready 失效改写为通过；安装后再安排长测。

## 历史与现场

- 0.46.15 F012F1FE此前用户已本机安装，组件复用/配置保留只读核验通过；不代表当前0.46.16已安装。
- 现有实验室中心192.168.10.103、边缘192.168.10.105仅为历史地址，操作前需核对。凭据仅查 docs/test-credentials.md，不打入安装包。
- 完整旧MEMORY已归档 .ai/docs/changelog.md；历史进程ID与状态不是实时依据。AI_BOARD.md 是唯一活跃协作看板。

## 改动记录

- 2026-09-12 00:20 | Codex | FB-107新增本机快照数据帧/事务接收及007作业账本，目标收据原子提交、源确认持久化与未完成作业启动保护，独占SQL短测通过；跨节点传输/CDC交接仍未完成。

- 2026-09-11 23:56 | GPT-5 | FB-107接通配对Capture/Incoming Agent装配和双节点真实LWW短测，修复插值JSON与Canal重连，补准确事件状态/三语和UTC TIMESTAMP快照；首次全量产品链路仍open，无新包/部署。

- 2026-09-11 23:17 | GPT-5 | backend-ai：新增基于实际schema观测的多端点双向图编译与三源CRUD序列化回归；test/vet/lint通过，运行门禁和首次对齐未接通，无新包。

- 2026-09-11 23:08 | Codex | FB-107实现双向实际schema配对与三向投影；结构校验不解除运行门禁，独占SQL及全量test/vet/lint通过。

- 2026-09-11 22:40 | Codex / backend-ai | 本地loser持久修复和自身修复回放抑制实现，真实上下行恢复/回滚及SQL软墓碑回归通过；生产装配与首次对齐仍open，未出包/部署。

- 2026-09-11 22:13 | Codex / backend-ai | SQLWorker内部LWW与真实Canal屏障联通，双方向短测及test/vet/lint通过；生产装配、本地冲突恢复和首次对齐仍open，未出包/部署。

- 2026-09-11 21:56 | Codex / backend-ai | 实现SQL主键规范化、005历史事件/键结构表及本地版本登记，真实Canal屏障/上下行和MySQL核心回归通过；本地loser修复与完整双向/首次对齐仍open。

- 2026-09-11 21:42 | Codex / backend-ai | 增加CDC新鲜脉冲屏障和发布前版本登记接口，真实上下行短测及test/vet/lint通过；仅证明捕获顺序，完整双向/首次对齐仍未交付，FB-107保持open。

- 2026-09-11 21:34 | Codex / backend-ai | 提取conflict.ApplyInTx供后续生产Apply复用事务，验证回调同事务和成功/失败后调用方回滚；test/vet/lint通过，生产双向接入仍未完成，FB-107保持open。

- 2026-09-11 21:26 | Codex / backend-ai | 双向硬删除内部原子标记/专用收据和回放检查实现，真实上下行短测及复制回归通过；完整双向仲裁/首次对齐运行入口未交付，FB-107仍open。

- 2026-09-11 21:15 | Codex / backend-ai | 修复保留旧同步标记的本地UPDATE被误过滤；真实候选上下行Canal短测通过，仅关闭该缺陷，FB-107整体保持open。

- 2026-09-11 21:07 | Codex / backend-ai | FB-107增加内部离线快照复制及空表/结构重查、映射精度核验和失败回滚；独占MySQL及test/vet/lint通过，跨节点运行入口/CDC交接/双向Apply仍open，未改现场。

- 2026-09-11 20:56 | Codex / backend-ai | FB-107首次空表方向/映射计划和源时间冲突事务核心开始实施；独占MySQL组件验证通过，完整执行链路仍未交付，未改现场。

- 2026-09-11 15:55 | Codex / backend-ai | 0.46.16最终0EB21423安装包交付，最终Agent上下行CDC/SSH/39工具及全部本轮安装门禁通过；用户自行安装，长测与未实现能力不冒充完成。

- 2026-09-11 15:52 | Codex / backend-ai | 收敛0.46.16修复升级包、撤回VPN恢复SSH，补二进制无损协议与真实双方向CDC/SSH/安装验证；最终重封收尾，不部署或启动长测。

## 2026-09-11 15:52 MEMORY 历史快照

以下保留收敛到升级安装包前的记录。VPN 方案已按用户决定撤回；历史进行中状态不覆盖当前 MEMORY 与 AI_BOARD。

# MEMORY

Last updated: 2026-09-11 14:41 Asia/Singapore

## 当前阶段

- 2026-09-11 backend-ai FB-108远程MCP源码实施：受保护VPN Streamable HTTP+独立token，机器DPAPI/ACL，安装默认关闭/升级保留token/独立SYSTEM任务及防火墙manifest，本机鉴权探针、三语设置轮换/复制。真实SDK/CLI启停轮换进程重建/实际工具目录摘要、DPAPI/设置鉴权通过；五安装模式和启动归属夹具、三语宽窄屏UI通过，NSIS测试编译与中文选项预览通过。全量go test/vet通过，尚未正式出包部署。当前非管理员，SYSTEM实际启动/无登录重启/VPN对端/完成页复制实测仍open，见docs/remote-mcp-vpn.md。不关闭FB-108；原FB-107首次对齐及历史误账继续，FB-109记录既有npm依赖风险。

- 2026-09-11 13:38 backend-ai继续FB-107：BINARY扩宽拒绝、软删schema检查过滤无关映射并避免跨规则缓存漏查；规则预检提示触发器和入出外键且显式保留可见性/副作用未验证。独占MySQL真实SOFT/结构/权限测试通过。PeekMessages改取完整有界批次后逐条退回，错误路径同样退回；独占RabbitMQ6293fc06随机端口45054上取样/消息保留和禁用规则退回各5次通过（运行时Apply测试替身）。全量test/vet通过；FB-107及首次对齐未完成，未部署或操作业务环境。

- 2026-09-11 13:27 backend-ai继续FB-107（用户明确不在阶段小结停止）：新增rulecheck本机源/目标真实schema/PK/InnoDB/软删列/EXPLAIN权限预检及新增启用/已启用规则修改拦截；可选源schema比较类型范围/精度/NULL/必填列，远端新鲜度/CDC/存量冲突仍未核验。生产Apply入口改CheckedSQLWorker，同事务持有目标MDL后核验真实PK。规则保存CAS、saved/active_revision、运行启动revision；跨规则库schema/query/mutation；事件状态工具+失败页分开运行重试与ACK FAILED；禁用规则入队消息改失败退回而非静默ACK。MCP输入大整数丢精度在真实测试中发现并修为UseNumber。
- 本轮独立MySQL8.0.38容器nb-remediation-20260911-b（127.0.0.1:13367）仍供后续测试，测试用例各自创建/删除独占库/用户；真实Apply、预检只读账号1142、MCP stdio启用拦截/跨库DML/9007199254740993均通过。全量go test、lint0 issues、tsc通过；前端三语桌面/390px、CAS错误、预检/重试截图通过并修窄屏裁切，最后ACK空状态文案仍需重截。未部署/出包；历史误账修复、受控残留工具、首次对齐、完整最终包CDC链路仍继续实施。文档首批报告是历史阶段状态，不能视作最终完成。

- 2026-09-11 backend-ai：FB-107首批正确性整改通过全量go test/vet/lint及前端tsc/build/契约检查，Wails binding已清除撤回授权接口。独立MySQL8.0.38容器/新库验证严格INSERT、FK与超2^53整数PK、缺行/同值UPDATE、HARD删除、账本失败回滚、批次前缀重试通过（0.32秒）；库与临时容器已清理，不操作业务AI实例数据/config/Agent/残留。答复docs/business-sync-remediation-20260911.md覆盖Q1-Q12。FB-106撤回closed；FB-107仍open：真实结构/权限预检、重试统一查询、CAS/active_revision、历史误账修复和残留处置未完成；未出包/部署/UI视觉复测/最终包CDC端到端验收。首次对齐执行与双向仍不支持。

- 2026-09-11 backend-ai：用户纠偏MCP权限全开放，隔离仅指本方测试数据；已撤掉误加的授权申请/审批/存储/UI，保留原开关、脱敏和数据治理确认。当前优先处理业务AI报告，已先答复Q1-Q12，FB-107进行中：严格INSERT、缺行UPDATE检测、HARD/SOFT删除、完整键和映射碰撞检查、未实现冲突/双向模式拒绝启用。仅本方源码与隔离测试，不改业务AI数据库/config/Agent/队列残留；未出包、未部署、首次对齐引擎未完成。
- 2026-09-11 review-ai：按用户要求编制.ai/docs/initial-alignment-implementation-plan.md、mcp-capability-coverage-plan.md及可编辑架构图，FB-101方案完成。用户追加明确“首次对齐是选项，不能全自动”：默认关闭，手动范围/预览/确认，安装注册保存启停不触发；MCP同限，重启后作业默认暂停；不选则原CDC位点/增量语义保持。M1维护窗口对齐+旧MCP补齐、M2在线交接，完整范围初估31-46工程人日，非实施承诺。
- FB-102后端/103前端/104测试/105业务决策均open待实施或确认；新工具/字段/DTO只是拟议，未写入稳定契约。当前只改文档，不改产品、安装包、现场配置或运行状态，不启动长测；维护窗口/数据权威/主键/删除等仍待确认，旧FB-094/084/089等不关闭。
- 本轮方案检查：33旧工具/38 CLI清单无遗漏，链接存在、drawio结构0错误0警告；现有go test/vet/lint（0 issues）通过、文档范围diff检查通过，不作为新增功能已实现或现场E2E证据。

- 2026-09-11 test-ai：用户本机实装后FB-100只读核验closed：10:42:59正式0.46.15 reuse安装passed，三组件相关步骤skipped、配置规则kept-existing、正式Agent F001/UI D580与发布包一致。注册0.46.15，无原生RabbitMQ/Canal/MySQL服务；Docker原容器运行，原生Erlang属09-08旧安装（日志可证），本次未安装。安装版MCP MySQL/RabbitMQ连接ok，server-001/external正常；当前Agent stopped、旧工作区15972已不在，未替用户启动。此次无原UI恢复对象，不关闭FB-084；FB-094/089等仍open。

- 2026-09-11 backend-ai：用户授权半自动或自动Docker模式，0.46.15以半自动NSIS三语组件选择页交付，默认复用（含静默及直接PS脚本），只有显式InstallSystemComponents才安装。复用保留配置/规则原文及密码，跳过组件安装、密码迁移、拓扑配置；首次创建external bootstrap，不自动更改已有managed归属。FB-098/099closed，无Wails DTO/同步核心变化。
- 新包build/NodeBridge-beta-v0.46.15-20260911.exe，339713325字节，未签名，SHA256 F012F1FECBD58A85B2D43F2C6A70E19836BBF3B38C34C7F359A7EF1F17FC8419；Agent F001902B。五模式fixture/三语真实向导预览/冲突参数/双隔离覆盖及配置hash/32和64各15/43文件和5资产/MCP33工具和5资源/test-vet-lint通过。记录docs/v0.46.15-component-mode-handoff-20260911.md，证据.cache/v04615-release。未安装本机或边缘、未停Docker或旧Agent15972、未自动环境检测/重跑长测；FB-094/084/089仍open。以下0.46.14说明为历史，0.46.15复用升级不再重设密码。

- 2026-09-11 backend-ai出包后只读核验：本机Docker MySQL3306/RabbitMQ5672/Canal11111及server-001 external配置正确，但NSIS系统组件阶段不会自动识别Docker或按external跳过；本机必须用EXE /SkipSystemComponents，先停旧工作区Agent15972。已补docs/mcp-business-ai-handoff.md出包后说明，原包hash不变且内文档无补段。FB-098open记录检测不足；未安装/停服务/改现场配置，试用版安装仍会将NodeBridge管理/退出密码迁移为1234。

- 2026-09-11 backend-ai：按用户最新要求，中心本机与边缘均由用户安装同一个0.46.14包，取代此前中心仅工作区的决定。FB-097 closed；安装包build/NodeBridge-beta-v0.46.14-20260911.exe，339728542字节、未签名，SHA256 C580851F014FA3672DDD12C0F3BA1FA58E1AF182075DD3DEDC84E7B0BD79BA2D。纳入FB-096日志状态UPDATE修复，Agent368D3932；随包docs/mcp-business-ai-handoff.md，另生成build/NodeBridge-mcp-client-v0.46.14.json普通模式连接两端。不打包实验室凭据清单。
- test/vet/lint、Wails/CLI/契约、42解包文件/5资产hash、隔离两次覆盖安装、32/64位各15项、MCP0.46.14标准握手/33工具/5资源及ERROR读取全部通过。证据.cache/v04614-release/package-gates.json，记录docs/v0.46.14-package-handoff-20260911.md。未自动安装或追加长测；中心仍旧工作区Agent15972，用户安装前必须停旧副本，Docker组件按external复用。FB-094完整性能/FB-084实际UI恢复/FB-089时间误脱敏仍open。

- 2026-09-11 09:48 backend-ai：按用户先方案/报告再实施且有界等待，原表峰值续跑574.185秒完成新增50列/2000键/账本核验，后段复现约1.9秒event_log慢写及PRIMARY尾部锁等待，未达到125秒失败。双事务UPSERT/UPDATE/UPSERT确认状态UPSERT阻塞独立INSERT；报告docs/performance-root-cause-20260911.md先更新，再修为已有日志事务状态UPDATE，保留PENDING/payload/confirm/位点顺序，缺失日志回滚，重入安全。真实SQL对照1.55秒、全量test/vet/lint通过；FB-095/096交付closed，FB-094完整故障与修复版整链路验收open。仅源码修复，未出包/部署/追加长测。
- 09:36现场恢复中心15972/Edge19320、原规则CD26、运行队列均0；旧11040消息已confirm隔离保留，Canal30秒诊断暂停已恢复、PFS临时开关已还原。双端原5张表备份及hash在调查报告；原表含诊断续写，原失败状态由备份和旧结果文件保存，不追认七小时通过。

- 2026-09-11 08:28 test-ai：七小时wide25200s_0910_213112在02:08:12因延迟探针超过125秒失败、02:08:29结束恢复，约4小时36分，未跑满；FB-093记录失败关闭，FB-094 backend-ai open定位峰值积压及残留。每方向605440 INSERT/320720 UPDATE，两次15分钟故障恢复/DDL通过但最终一致性未核验。最后磁盘净增长Edge6.19GB/Server22.68GB，非空间不足；失败前入口ready10925/unacked50。08:28实际入口9705+下行1335残留，未删除或重启。正常Agent36244/10828实测存活、规则CD26一致，防休眠19124已释放；恢复passed仅配置/进程恢复，不代表队列已清空。

- 2026-09-10 21:32 test-ai：用户最新改为七小时且明确立即开始、能跑多少算多少，FB-093执行。wide25200s_0910_213112已于21:32:04开始25200秒BestEffort，计划09-11 04:32:04结束；初始50列/回放/交接通过，双向真实INSERT/UPDATE推进。监督器21916、生产16092、防休眠19124。此模式按用户授权不要求旧容量预测准入，保留原负载/正确性/延迟/故障检查、每盘100GB净空间下降上限及10GiB物理底线；错误或超限自动恢复记失败，不追认七小时完成。原正式模式未变。十小时设计被本请求取代，当前不是100GB全生命周期容量验收。

- 用户最新要求按100 GB设计，不再将迁移D盘作为唯一前置条件。test-ai方案docs/100gb-storage-test-plan.md、FB-092 open：每节点总工作预算100GB，分类测量与实际路径、持续增长/临时峰值区分；保留原负载/正确性/延迟/1.5安全系数，预测工作占用<=80GB才启动，另保留物理盘10GiB底线。此前276GiB整盘外推不是已证明产品容量需求。当前仅设计，容量实现及完整短测待进行，FB-090未完成，十小时未启动。

- 2026-09-10 20:55 test-ai：用户授权清理无用MySQL表，FB-091 closed。边缘105张已结束run旧nb7表先备份D:/NodeBridge-Test-Backups/cleanup-20260910-205208/scada_edge-obsolete.sql（SHA256 0B60F356E220050AA156079A296DFBEAB2510F84ABC794F485BD3561CA9D569B），再核对行数/依赖/规则并DROP，2379293行、释放3.52GiB。最新验收表、业务库和同步系统表保留，Agent18980/规则hash/查询正常；C现159.6GiB，未满足十小时容量预测，未迁移/清binlog/新长测。旧失败原始表已按本次授权清理，但D盘SQL备份和全部原始测试报告保留，不改写失败结果。

- 2026-09-10 18:32 test-ai：widesmoke_0910_175644于18:27:49 completed，实际1801.222秒；每端64220 INSERT/39850 UPDATE，全50列/精确账本、13生命周期、双故障/DDL/晚期回放交接通过。126样本含30压力/方向，Edge稳态P95 605.036ms/全部P99 806.922ms，Server 619.572/1030.881ms。原规则/配置恢复passed，租约71972 released；中心46132/边缘18980为恢复核验PID。
- FB-090暂blocked：十小时未启动。边缘最低空间法实测5,293,061 bytes/s，按36000秒*1.5+10GiB需约276GiB，C现约156GiB；D约929GiB。已询问用户是否批准将实验室MySQL数据由C迁移到D并重验，未停库/迁移/清历史/放宽门禁。短测证明绑定36000秒，不能代替实际启动容量检查。test/vet/lint、监督器时长/计数/准入绑定、租约回归通过；同一6C0795产品二进制不变。

- backend-ai 已完成 0.46.13 候选交付，FB-087/086 closed。最终包 build/NodeBridge-beta-v0.46.13-20260910.exe，339704992字节，未签名；SHA256 5A38F8DA4D04CD7747C2322E73CF5F5F3796026EA72F161D6701ADAB91319DB7。Agent SHA256 6C07957C7802F1BCC2EC624FB5E5CE89368CCBC51A513B761DEB5011EB42C30D。没有自动安装或新长测。
- 真实 Edge 下行从逐事件事务改为 ApplyBatch，保留映射、配置/DDL 顺序、compact 开关与已提交前缀 ACK。固定 UTF-8 驱动参数插值一并纳入。真实隔离 50 列 1200 混合下行：逐事件 62–81/s、批量 353–410/s；50 万历史批量 356–401/s，全列/版本/Apply 数量通过。
- 真实 SQL CHECK 故障整批回滚、3条全部重入队、解除约束后提交及重复回放幂等通过。单元覆盖部分提交前缀、解析失败、配置屏障、列映射及 DDL。不以局部基准证明双端压力已验收。
- 新运行日志 logs/sync-runtime.jsonl：ERROR/WARN、worker、node/version/pid/event_id、重试/恢复、阶段耗时与执行中卡顿采样；8MiB + 4份轮转，凭据/SQL值脱敏。Wails GetLogs DTO 不变；MCP工具/资源默认发现当前及轮转日志，显式-log兼容，解析结构后脱敏避免破坏数值 JSON。
- NSIS MUI2 框架不换：三语欢迎/完成页、品牌位图、DPI；原UI用户会话通过临时 Limited InteractiveToken 任务恢复，拒绝 Session0/模糊桌面、避免重复，失败记 warning；不改自启动。9项模拟恢复回归、中文实际窗口布局检查通过。真实升级后桌面恢复留 FB-084 复测。
- 用户最新顺序：继续优化到可出包，错误日志必须做好，MCP能读日志；交付新包后一起复测。没有自动部署候选或启动新长测。

## 现场与证据

- 用户再次确认192.168.10.105已安装后，test-ai复核普通MCP0.46.13、日志及状态正常；边缘MySQL、本地RabbitMQ、中心RabbitMQ连接均ok=true，中心MySQL/RabbitMQ亦正常。边缘Agent仍stopped，未启动或更改配置；FB-088补充闭环，不视为同步验收，FB-089仍open。

- test-ai按用户“已开启MCP”只读实测：边缘正式安装MCP已0.46.13，普通模式15工具、握手/概览/日志/Agent状态成功，边缘Agent当前stopped；中心MCP工作区0.46.13正常，但同步仍0.46.12/PID17460 running。两端读到旧sync-agent.log各5条，未启停同步。FB-088 closed；MCP时间元数据误脱敏另记FB-089 open。当前对话原生工具未加载，本次为直接stdio/SSH协议验证。

- 中心实验室 192.168.10.103，MySQL scada_center；边缘 192.168.10.105，scada_edge。凭据只查 docs/test-credentials.md。
- 用户已明确中心使用工作区运行，不要求本机正式安装；边缘仍正式安装。最近恢复版均0.46.12，中心PID17460、边缘PID5836为15:08核验值，不视为实时PID。
- MySQL buffer pool 8GiB/2GiB与原可靠性保持，本轮未改现场Agent/配置/业务历史。测试基准只创建并清理其独占库/队列。
- wide7h_0910_100650 在14:44:10因125秒延迟探针超时失败，14:44:25完成恢复，未满七小时；峰值入口ready13585，不能判定性能无问题。旧准入已撤销，不追认完成样本或恢复为完整一致性通过。
- 13768条本run残留经confirm保留到专属quarantine，15:08时实际入口/下行ready及unacked=0，原隔离队列未删除。概览0不能替代实际Broker核验（FB-072）。
- 0.46.12此前093438完整1800.925秒、全50列/oracle/精确账本、故障/DDL/交接及延迟门禁通过；后续七小时失败不改写该阶段或反向外推长期通过。
- 性能细节见 docs/performance-investigation-20260910.md，交付报告 docs/v0.46.13-candidate-handoff-20260910.md。最终日志证据 .cache/v04613-final-wire-gate/evidence.json；包门禁 .cache/wide-seven-hour/package-gates.json，已验证Agent哈希一致。

## 协作与边界

- 每次任务声明 frontend-ai/backend-ai/test-ai/review-ai，先读 AI_BOARD.md；稳定接口只维护 .ai/docs/frontend-backend-contract.md。
- Wails UI 只调用 Go bindings，不占端口，关闭隐藏托盘，显式退出鉴权。
- SyncEvent 保留源库表列语义；业务与系统日志提交后ACK，event_id与sync_apply_log幂等；同键CRUD、DDL与配置屏障保序。
- MCP目前33工具，默认关闭、启用后持久化，SSH stdio无HTTP端口；实验室权限不突破配置加密/数据治理/审计。
- 安装器仅管理 manifest 登记的自有资源，保留已有配置/规则与客户MySQL。新安装不自动启动UI，测试变体禁止UI恢复。

## 待完成

- FB-087/086已关闭交付；同版本07C4/A246中间候选因MCP二次脱敏缺陷被替换，只有5A38最终包有效。
- FB-085：新包双端同等高负载、故障恢复、精确账本和完整延迟复测；旧七小时失败保持。
- FB-084：实际升级后原交互会话UI恢复验收，不能用模拟任务测试替代。
- FB-072概览队列统计、FB-073纯空闲位点专项、FB-046前端schema_sync三语开关、Mac MCP等既有项仍以Active Board为准，不擅自关闭。

## 验证门禁

- 最终代码 go test ./...、go vet ./...、golangci-lint run ./...（0 issues）通过。
- 最终二进制真实启动ERROR落盘及MCP工具/资源读取、纯数字密码不损坏JSON数值通过。
- 最终源码Wails生产构建、binding/前端编译、CLI smoke通过。
- 最终压缩版42文件/5资产逐一hash、两次隔离覆盖、32/64位各15项安装回归、MCP33工具/版本/只读诊断通过。覆盖证据 .cache/nsis-upgrade/20260910-161450-491/upgrade-evidence.json，绑定最终Agent与UI哈希。
- go test -race此前因CGO关闭且未发现gcc/clang未成功；未将其计为通过。

## 历史记录

- 先前完整MEMORY快照已原样归档至 .ai/docs/changelog.md，含2026-09-09和2026-09-10快照；历史进行中状态不覆盖本文件与Active Board。
- 保留所有历史失败、准入撤销和隔离消息证据。

## 改动记录

- 2026-09-11 14:41 | Codex | backend-ai实施FB-108 VPN远程MCP、机器DPAPI/ACL、安装后台脚本和三语设置，test/vet/lint及隔离协议/UI通过；SYSTEM重启/VPN实机与FB-107仍open，未部署。

- 2026-09-11 13:27 | Codex | backend-ai继续FB-107：规则预检与启用拦截、CAS及active revision、事件重试查询、禁用消息保留、跨规则库治理及真实MCP精度修复；全量test/lint通过，剩余对齐与修复流程继续。

- 2026-09-11 11:10 | Codex / review-ai | 首次对齐可选手动实施方案、技术路线与新旧功能MCP覆盖清单；登记FB-102至105分工，FB-101只关闭方案，未实施或部署。

- 2026-09-11 10:40 | Codex / backend-ai | 0.46.15半自动组件模式及默认复用出包，全量门禁与实际三语预览/隔离升级通过，FB-098/099关闭；不执行现场安装或自动Docker探测。

- 2026-09-11 10:13 | Codex / backend-ai | 0.46.14含日志状态UPDATE修复完成封包与全部门禁，交付业务AI MCP说明和双端客户端JSON；用户自行双端安装，FB-097closed，未将旧长测故障追认为通过。

- 2026-09-11 09:48 | Codex / backend-ai | 原表短复现与双事务实验确认日志状态UPSERT锁放大，先报告后改为事务状态UPDATE；真实锁/幂等/缺失回滚及test/vet/lint通过，未部署或宣称长测通过。

- 2026-09-10 16:13 | Codex / backend-ai | 下行真实批量、持久诊断与MCP结构化读取、NSIS交互会话恢复和三语向导完成，隔离正确性/性能与最终代码门禁通过；最终包门禁收尾，未部署或重跑双端长测。
- 2026-09-10 16:18 | Codex / backend-ai | 0.46.13最终5A38安装包全门禁通过，MCP真实ERROR及数字密码不破坏JSON已绑定同一Agent；FB-086/087交付关闭，FB-084/085留实际升级与双端复测，旧长测失败不追认。


## 2026-09-10 16:13 MEMORY 历史快照

以下完整保留阶段记录；旧安装版本与进行中状态不是当前结论。

### MEMORY

Last updated: 2026-09-10 15:58 Asia/Singapore

#### 当前阶段

- 2026-09-10 15:58 backend-ai：FB-087候选0.46.13实现真实Edge下行ApplyBatch、持久化结构化诊断/轮转/脱敏/阶段采样、UI与MCP读取。真实隔离50列1200混合下行，逐事件62–81/s→批量353–410/s；50万历史批量356–401/s，逐行/版本/Apply数量通过。全量test/vet及去除旧私有函数后lint通过（最终版还需重跑）。NSIS保留框架新增三语欢迎/完成页、品牌位图及原用户交互会话恢复；9项模拟恢复回归和旧二进制隔离双覆盖通过，新包尚未构建。原Agent/正式配置未改，双端性能及真实UI恢复留用户新包复测，不追认长测通过。

- backend-ai收尾：13768条失败长测残留按run前缀确认后保存至专属quarantine队列，未删除消息或旧隔离队列；实际broker入口/下行ready及unacked均0。原Agent中心17460/边缘5836存活、未停止或替换；新候选未部署。

- 2026-09-10 15:08 backend-ai：按用户要求先性能后UI/NSIS出包。候选DSN启用固定utf8mb4的驱动参数插值，事务/ACK/回放语义不变；真实精度/转义/回滚四组合通过。真实隔离入口1200混合事件、50万历史原模式209-284/s、优化288-383/s，逐行50列/版本/精确日志通过；全量test/vet/lint通过。仍不是双端高负载验收，FB-085/086保持open、FB-084尚未实施；无新包或部署。旧formal-ready已保留并撤销，FB-083以失败终态关闭；临时双端候选验证方式待用户答复。详见docs/performance-investigation-20260910.md。

- 2026-09-10 14:46 test-ai：wide7h_0910_100650在14:44:10因延迟探针超过125秒自动失败，14:44:25 finished，未满七小时。最后快照每端606640 INSERT/321320 UPDATE，失败ACK/事件0但尚有积压，不能认定最终一致性通过。完成样本P99上行9674ms/下行12461ms，31生命周期、DDL加删及331896在线版本观察通过；故障根因留FB-085。自动恢复passed，现场正常Agent17460/5836存活、边缘原规则hash一致、awake released；本次只读排查未停止/重启测试，未改配置。

- 2026-09-10 test-ai：只读核验边缘UI进程NodeBridge.exe/DataSync.exe均不存在，SyncAgent正常，未启动/停止程序或改配置。用户授权在wide7h_0910_100650结束且现场恢复后修复升级UI恢复问题，登记FB-084交backend-ai；长测中禁止实施。历史退出原因尚未完全确认。

- 10:14 test-ai：用户要求的“功能/短测通过后七小时稳定启动”已得到证据。093438完成1800.925秒、状态completed，每端64220 INSERT/39850 UPDATE，全50列/oracle/精确账本、33452次版本观察、12生命周期、双故障/DDL/晚期探针与交接及恢复全部通过。135样本/方向（102稳态/33压力），Edge稳态P95=607.918ms、全部P99=1011.091ms；Server=634.690/2893.418ms，原阈值通过。awake4940已released退出。
- `formal-ready.json`于10:06生成，绑定E61候选、WorkspaceServer、原配置hash、helper/script/schema及安装包；实际磁盘增长观察1674.24秒，七小时空间门禁通过。正式`wide7h_0910_100650`于10:07:37.956启动，计划17:07:37.956结束，runner33492/producer29224；10:11核验相隔139.794秒快照，双向新增约900→3680、UPDATE及目标落库递增、失败0，见run目录`stable-start.json`。这不是七小时最终通过证明，后续结果留FB-083。
- 正式运行中Agent中心69920工作区v0.46.12、Edge7832安装版v0.46.12，hash均E61F24D8DA79504268928F272EC0F56694060905BDA6FC40F6A3145C13F3B003；配置hash与准入一致，双侧测试规则hash28FD103E5BF67CEF30B8C2CF858D0B323E64B9D17B8A0F10E738210D5787606C匹配本run。awake5388已确认active，最长17:19:21或恢复规则后提前释放；不改电源方案。MySQL保持8GiB/2GiB，无中心安装/UAC重试。
- FB-075/076/077/078/079/080/081/082闭环；FB-083长测结果仍open，FB-072概览队列统计、FB-073纯空闲位点专项复核等既有项仍open。测试始终直接核对Broker，不以概览0作为门禁。旧失败全部保留。

- 09:36 test-ai：v0.46.12最终全量test/vet/lint、41文件hash/5资产/双覆盖/32与64位各15回归/MCP33工具通过。安装包SHA F3A9E39834F8D7A537A5BAC0C4FAECB484EA85046200FEB665050E6474CD0952，Agent E61F24D8DA79504268928F272EC0F56694060905BDA6FC40F6A3145C13F3B003。边缘安装PID6348成功、解密全文/规则保留、MCP启动16060及2GiB诊断通过；中心旧39956优雅退出，42184工作区候选及配置/规则/8GiB核验通过。备份与安装证据`.cache/v04612-installed-20260910`。
- 新`widesmoke_0910_093438` runner54528，前置50列/重复旧UPDATE/同键交接通过，09:35:29开始完整1800秒、计划10:05:29结束。awake4940 active，最长10:11:14或原规则恢复即释放；不改电源方案。FB-081实现交付关闭，FB-082完整准入仍open，尚无新七小时。

- backend-ai接手FB-081：Store.UpsertEventLogs改为同事务内最多128行/估算1MiB的多行SQL，保留顺序、原冲突字段、payload与整批回滚；单个超大行仍由原驱动/服务器上限处理。边界/失败/取消单测及真实MySQL257事件全部16字段逐条对照、跨分块重复ID、特殊大payload、第二分块SQL失败整批回滚通过。隔离1000混合/320回放基准4.400→2.244秒，日志3.097→1.148秒；不是整体延迟通过。全量test/vet/lint通过（版本更新前），版本与打包默认升0.46.12，准备候选包。现场仍0.46.11，原配置与8GiB/2GiB不变。

- 09:12 test-ai：`widesmoke_0910_083751`于09:06:56最终延迟门禁失败、09:07:09自动恢复，runner54264已结束。每端64220 INSERT/39850真实UPDATE全部生成；全50列/oracle、精确Apply/SUCCESS账本（双向state UPDATE均39888）、33456次版本观察、12生命周期、双故障恢复、DDL和晚期探针/交接通过。每方向130延迟样本（107稳态/23压力），Edge稳态P95=406.852ms/全部P99=6852.765ms，Server稳态P95=639.559ms/全部P99=12428.633ms，均未满足全部P99<=5000ms。高延迟集中08:59峰值及09:00 UPDATE重载；两端wait_free/log_waits增量均0，不再以内存不足归因。FB-080归因中，FB-077仍未解决。
- 正常Agent中心39956仍工作区C983候选、Edge15820正式安装C983，原规则/配置恢复passed，业务broker队列0。awake7644于09:07:14随原规则恢复released、进程退出；MySQL8GiB/2GiB未改。完整30分钟准入未通过、无新七小时测试；不能把数据/账本通过当整体通过。

- 08:40 test-ai：按用户最新决定，中心无需安装，改为固定工作区候选运行，取代旧“双端实装才能测试”要求；不重试UAC。中心74960优雅退出，新候选SHA C983C2C182EE11B74072D6C5284BEE7C1FB4F683C1FA04A815181AE4B245114F，配置/规则保留于`.cache/workspace-runtime-20260910-083730`。边缘正式0.46.11。监督器显式WorkspaceServer模式绑定实际路径/hash/运行实例，证明区分workspace/installed，默认Installed不变；专项及全量go test/vet/lint通过。FB-079 open，FB-078安装阻塞解除。
- 新`widesmoke_0910_083751` runner54264，前置50列/重复旧UPDATE/同键交接通过，08:38:39开始完整1800秒、计划09:08:39结束。短测期间Agent中心22020/边缘12976；唤醒租约7644 active，最长09:14:08或原规则恢复即释放，不改电源方案。MySQL保留8GiB/2GiB。完整账本/延迟尚待验收，不追认历史失败，无新七小时长测。

- 00:50 test-ai：边缘v0.46.11正式实装passed（安装PID6692结束0），配置解密全文及规则hash保留，Agent/hash正确，MCP启动PID11316，MySQL诊断13.897ms、2GiB。证据`.cache/v04611-installed-20260910-0045/edge-install-summary.json`。中心UAC返回“操作已被用户取消”（安装session62099结束失败）；中心仍v0.46.10/PID74960，旧Agent hash及配置/规则hash核验未变。不重试授权，FB-078 blocked等用户完成中心安装或明确继续；无新短测/长测，两端MySQL设置未修改。全部有限安装/验证进程已结束。

- 00:45 test-ai：backend-ai修复交接（FB-078），v0.46.11正式安装包构建及全部包门禁通过，最终版本全量go test/vet/lint通过。41解包文件、5资产、两次覆盖安装、32/64位各15回归及MCP/UI 33工具核验。安装包SHA=418899EDEB87DDD3DD92D81AB5B6293D9042FFD1AAD60E7F549B8D2BF2F4031B；Agent SHA=C983C2C182EE11B74072D6C5284BEE7C1FB4F683C1FA04A815181AE4B245114F。开始双端备份/实装，尚未开始新短测或长测，延迟与精确账本仍待真实完整验收，MySQL保留8GiB/2GiB。

- 00:38 backend-ai：按FB-078显式接手FB-076/073。回放判定增加context及error，Store事务内FOR SHARE等待Apply提交，SQL/提交/取消/缺失依赖错误阻止发布和ACK；四类CDC入口及Canal重连重试已覆盖，真实Store提交/回滚/锁等待取消隔离实验通过。新增Offset.SkipCheckpoint仅运行时字段（不序列化），空批次/全部抑制只ACK、不写SQL位点，有效业务即便无活动Edge仍保存位点；对应回归通过。全量test/vet/lint通过（版本更新前），现版本与打包默认升0.46.11，Wails/CLI/离线预检通过、NSIS打包中，尚未安装或重启新短测。实际现场仍v0.46.10及8GiB/2GiB；压力延迟FB-077未证明解决。

- 00:22 test-ai：234447于00:14:02最终验收失败（runner33288已结束），每端完整生成64220 INSERT/39850 UPDATE；12生命周期、双故障恢复、DDL、晚期probe/handoff通过，但精确Apply账本Server UPDATE多1条。随后混用DateTime/DateTimeOffset重试条件抛错，已修测试UTC比较并补跨午夜等回归（FB-075）。多出事件cdc4593773b4f60a36533fa02fe86fba对应中心setting_id=591/v4，携带edge-001及真实Apply回放ID cdc07f1e8ae43a373144f4dd03e4afd8（FB-076）。隔离MySQL锁等待实验通过，确认普通查询可在未提交记录前返回0，事务锁定读等提交后1/回滚后0；核心尚未修改。Store.Exists还吞查询错误，需backend-ai修可见性及失败闭合，不放宽账本。
- 234447独立延迟核验每方向131样本（105稳态/26压力）：Edge稳态P95=404.984ms、全部P99=3826.417ms；Server稳态P95=821.292ms但全部P99=8976.930ms失败（FB-077）。两端buffer_pool_wait_free自23:45:38至00:13:27增量0，8GiB/2GiB保留。00:14:14原现场恢复passed，00:21配置/规则hash一致、正常Agent74960/6832，awake33392released，正常队列0，无新长测；formal-ready仍失效。FB-074文件修复运行验证通过关闭，FB-075/076/077开放。专项/监督器/全量test/vet/lint通过，隔离实验库自动清理。

- 23:47 test-ai：修复后的新widesmoke_0909_234447已启动，前置50列/重复旧UPDATE/同键交接通过；23:45:41开始完整1800秒，预计2026-09-10 00:15:41结束。runner33288/awake33392均确认存活，防休眠最长00:20:51或随规则恢复释放。43.7秒快照每端约880 INSERT/440 UPDATE、失败0，仍待完整门禁；无新七小时长测。

- 23:44 test-ai：231556在23:39:05约22分钟发生文件已存在错误，已failed/finished且runner74636结束；每端52560 INSERT/30470 UPDATE。此前双故障实时恢复、DDL、晚期精度/回放/同键交接通过，不能代替最终门禁。原配置/规则/hash恢复核验通过，正常Agent69948/15576存活，awake65968随规则恢复released，76条残留confirm隔离，正常broker队列0。新FB-074：受控共享读锁在旧Move-Item写状态复现同一错误；已仅在宽表监督器改File.Replace+2秒有界重试并补异常调用栈。原子写专项/原监督器及全量test/vet/lint通过，待从零完整1800秒。中心8GiB、边缘2GiB持久设置保留。

- 23:18 test-ai：中心Agent缺失已通过正式安装版MCP start恢复，边缘原PID13980正常。新widesmoke_0909_231556前置50列/重复旧UPDATE/同键交接通过，23:16:42进入完整1800秒负载，计划23:46:42结束；runner74636及awake65968已验证存活（租约最长23:51:52、规则恢复即释放）。89秒快照每端1800 INSERT/900 UPDATE，双向incoming对齐、failed_events/failed_acks均0；不能提前判定短测通过。两端8GiB/2GiB保留，全量go test/vet/golangci-lint通过；当前仍无新七小时长测。

- 23:14 test-ai：用户明确要求中心8GiB、边缘2GiB（约40张表），已分别SET PERSIST为8589934592/2147483648；两端运行值、持久值、persisted_globals_load=1及resize Completed均核验，可靠性参数仍1/1，不自动回退。此设置取代23:10的512MiB。短测231352于23:14:02因both original agents must be running在预检失败，尚未产生负载；须恢复原Agent运行后重新完整短测，未启动新长测。

- 23:10 test-ai：用户授权直接修改MySQL并明确保留、不自动改回。中心Docker mysql-8.0.38（mysql_data持久卷）、边缘192.168.10.105 MySQL 8.4.11均执行SET PERSIST innodb_buffer_pool_size=536870912；两端运行值/持久值512MiB、persisted_globals_load=1、resize Completed已核验，innodb_flush_log_at_trx_commit=1及sync_binlog=1保持不变。FB-071授权阻塞解除、转open待完整1800秒复测；两端调整不是中心单变量实验，不证明根因或负载门禁通过。当前无新长测。未改产品代码或测试脚本，本次仅SQL配置及文档更新，未重跑Go门禁。

- 最新`widesmoke_0909_202640`原一路于20:43:38中心恢复期限失败，实时上传/下行差值1280/1125；每端36660 INSERT/18330 UPDATE，前置功能及6次生命周期通过，最终门禁未完成。20:43:52自动恢复passed，1570条残留confirm隔离；20:49双端配置/规则hash均与原备份相同，安装版v0.46.10/hash不变，正常Agent运行、队列0，唤醒PID12724已released。无长测。
- FB-071首次现场证据：恢复期间中心buffer pool空闲页0、wait_free从204295→205999、脏页6476；SQL观察器同事件持续至少800ms，事件日志执行占用突出，Canal确认延迟约30秒且缓冲近满。23:10已获授权并完成两端128→512MiB持久调整，待同负载复测。FB-073仍独立open；不能宣称唯一根因已证明。
- 测试专用SQL观察器每200ms读取中心sync_user当前语句类别，不记录SQL/参数、不启用原本关闭的等待采集；v1/v2证据分别保留。收尾发现成功测试误用50ms期限而偶发失败，已把短期限限定于timeout用例并明确核验取消；重复30次及最终全量test/vet/lint通过，实际门禁不变。

- 20:20 test-ai：隔离50万历史的宽UPDATE分发364.91/s、40 INSERT/20 UPDATE交错246.50/s，顺序/50列/1000条SUCCESS验证通过，仍未复现FB-071。新增FB-073：空闲时位点表捕获自身写入形成循环，47秒写计数+816而事件/Apply计数不变；尚未修产品，也不认定它为恢复超时唯一根因。全量test/vet/lint通过。

- 追加诊断：原一路配置`195000`完成600秒，50列/oracle/账本/4生命周期/双故障/DDL/晚期探针/恢复全部通过；每端21420 INSERT/13290 UPDATE，稳态P95 Edge403.789/Server625.108ms。它不满足1800秒正式准入，不能推翻192354失败。550.7秒双端buffer pool wait_free增量0，MySQL参数未改。正进行隔离连接/大历史日志基准，仅测试文件、不改产品。

- 当前身份test-ai。四路Apply的`widesmoke_0909_192354`于19:41:05中心恢复期限失败，实时积压Edge560/Server1640；每端36900 INSERT/18450 UPDATE。19:41:20原现场恢复passed，中心通道数已另恢复1，500条本run残留confirm隔离，双端Agent已重启。FB-071继续诊断；无长测，formal-ready仍失效。
- 历史184033约17.5分钟lifecycle180秒超时、每端39000 INSERT/19710 UPDATE，恢复后延迟21-30秒；499条遗留消息已confirm隔离。191842被队列预检拦截未产生负载，均不追认通过。FB-072记录概览下行0而broker499的差异；准入始终直接查broker。
- v0.46.10包SHA256 `FF17F3DC28A409D028FA044C16D4F9FFC358BB9EF3162D5ADB79C60412DAA915`，Agent `0A0E3175F40CC15648D42A9B5C265E7D3524DFDAF5E8D51269A47E8F7DE8FEF3`。双端Apply非唯一(table_name,op_type)覆盖索引标准迁移通过，同一失败run计数不变，中心64-96ms、Edge232-266ms。全量test/vet/lint、8万行基准、包内双角色CLI、覆盖/32/64位/41文件门禁通过。
- 用户明确愿意正式安装更新，不以免安装为约束。后续定位修复后交付候选包，核验双端正式安装路径/版本/hash再从零验收；临时包内Agent验证不等于正式安装全软件验收，不降低负载或跳过检查。
- 正式run `wide7h_0909_161239` 16:13:22开始，16:34:10因snapshot 15秒超时失败，16:34:26自动恢复结束。每端24980 INSERT/12490 UPDATE；流水行数对齐，最终全50列/oracle未核验。Edge恢复安装路径PID7872、Server原备份路径PID17896，规则/配置校验通过。
- 正式实装包`build/NodeBridge-beta-v0.46.9-20260909.exe`，339524533 bytes，SHA256 `6C26E6D397C36238A7DEDCBB246D0EC39368F23C40D741F04917B9F6E699514F`；Agent `9DF8C21E8959E98FE957AF58F31434F6E1BDD847FC6AA7266221795E11C50CDD`。未签名/提交/发布，双端32位NSIS注册视图均0.46.9。旧包与失败证据保留。
- v0.46.9新增限时只读MCP MySQL诊断、非唯一(table_name,status)覆盖索引标准迁移及包内schema。中心现库标准迁移成功，原77072条事件保留，失败计数0耗时1.311ms，EXPLAIN using_index=true；全诊断104.852ms。仍不宣称原始15秒超时唯一根因已证明。
- 监督器保留15秒外层门禁，新增14秒共享SQL期限/逐步trace；测试须两端注册安装路径/版本/hash一致。正式准入要求至少1800秒实装smoke，预期每端64220 INSERT/39850 UPDATE；旧600秒结果不再可签发新准入。
- v0.46.8统一SyncEvent/ChangeEvent无损数字解码、Canal字符串版本标准化、Apply回放标记/列映射、Edge apply-log依赖和MCP/UI版本来源。不同目标主键UPDATE最多64键确认批次，同键/CRUD/DDL屏障及严格主键约束不变；不把分段性能基准当长期保证。
- 最新短测`widesmoke_0909_160102`于16:01:44.696至16:11:45.190完成600.494秒。每端21420 INSERT/13290真实UPDATE，全50列/oracle、精确apply/SUCCESS账本、11176次版本观察、5生命周期、双故障、DDL、精度/重复/交接及自动恢复全部通过。
- 延迟43组/方向，稳态36、压力7。Edge/Server稳态P95=206.636/620.148ms；全部P95=403.629/1842.609ms，全部P99=808.016/2445.866ms。按固定负载阶段分类，压力样本保留；短测5秒/正式30秒采样。
- `formal-ready.json`在完整短测后于16:12:29生成，核对安装包/Agent/helper/脚本/schema及恢复证据，实测数据盘变化556.888秒，七小时空间预测通过；不是七小时通过证明。
- FB-063修复PS变量大小写冲突、UTF8/共享删除JSON读取、清理失败隔离、Windows原子替换短暂锁2秒有界重试；真实临时/持续锁和自动恢复回归通过。
- Edge临时唤醒PID10724于16:34:27随原规则恢复释放，不改电源方案；当前不再有本轮额外唤醒请求。
- 历次失败143533/144049/151711/153921/154525及0.46.7宽表失败均保留，不用恢复或重算冒充通过；详细原因见最终报告。FB-052/054分发/幂等优化历史见`docs/server-cdc-dispatch-optimization-20260909.md`；v0.46.7安装连通验收见其独立报告。

#### 当前实验室

- 本机模拟 Server：`192.168.10.103`，Docker MySQL `3306/scada_center`、RabbitMQ `5672` / `/nodebridge-server`、Canal destination `server-001`。
- 异机 Edge：`192.168.10.105`，`edge-001/scada_edge`，已正式安装v0.46.10，运行 `C:\Program Files\NodeBridge\app\SyncAgent.exe`。PID以最新run/现场核验为准；实验室凭据只查 `docs/test-credentials.md`。
- Server 也已正式安装v0.46.10并运行同一安装路径，不再使用旧备份或临时Agent；双端Agent hash为本页v0.46.10记录。中心apply_lanes已恢复1。
- 原双侧规则 SHA256：`CD26BB056B729B5D0A3614784467C849354E9A37181BF96ABE10FD6E8EFBE2CD`。短测使用独立 run-id/表和临时规则，结束后恢复原规则及原 Agent 路径。
- 隔离队列按run保留642/3189/499/500/1570条，不删除失败证据；正常队列深度以直接broker为准（FB-072概览统计差异待修）。没有本轮新长测或Codex定时任务。

#### 已验证基线

- v0.46.6 安装版真实单行延迟 20 次：P50 94.728ms、P95 127.240ms；安装、SSH stdio MCP 32 工具和断开后驻留已验证（FB-050）。
- v0.46.5 Edge -> Server 高压 1k/10k/50k 行及 Server 停机 20k 行恢复通过，源/目标/apply/event 精确 81,000，队列与错误为 0（FB-048）。
- 原 13 小时双向测试 `overnight-20260908-184000` 在约 3 小时 3 分失败：Edge -> Server 388,401/388,401，Server -> Edge 388,401/204,484。结果见 `docs/v0.46.6-real-bidirectional-overnight-report-20260909.md`，不能宣称长测通过。
- 历史日志大小导致后半程降速的因果关系未证实；百万历史对照的旧路径仍约 34 条/秒。长期 IO 和 CDC 系统表放大需要另测。<!-- 待确认 -->

#### 协作与边界

- 所有任务先声明 `frontend-ai/backend-ai/test-ai/review-ai`，读取根 Active Board；跨身份原因、范围和 owner 只登记 `AI_BOARD.md`。
- 稳定接口只查 `.ai/docs/frontend-backend-contract.md`；Wails UI 仅调用 Go bindings，默认不占端口，关闭隐藏到托盘，显式退出需鉴权。
- `SyncEvent` 保留源语义；不得假设源/目标库表列同名。业务写入和系统日志事务提交后才 ACK；重放靠稳定 `event_id` 与 `sync_apply_log` 幂等。
- `append_only` 仅用于 INSERT 流水；CRUD compact 仍需全局与单规则双开关。同键 CRUD 和 DDL 不得越过未确认前序事件。
- Canal 自动结构同步仅限显式规则授权的单列 ADD/DROP，不自动建表/删表。非 SERVER_TO_EDGE 的 DROP 要求恰好一个来源节点。
- MCP v0.46 实验室模式提供 32 个受控工具；默认关闭，启用后持久保存；SSH stdio 不开 HTTP 端口。配置加密、结构化数据治理与审计边界保留。
- 安装器只能管理 manifest 登记的自有资源，保留已有配置与用户 MySQL；升级前停止对应路径的 Agent。

#### 待完成

- FB-055 blocked、FB-066/068 open：等待双端正式安装v0.46.9，受控切换旧进程并执行中心标准索引迁移，再做至少30分钟实装准入及负载归因；未重新启动长测。
- FB-046：frontend-ai 补 Rules 页 `schema_sync.add_columns/drop_columns` 三语开关及删列风险提示。
- FB-034：Windows 异机验收已闭合，仅 Mac 本机密钥生成和 SSH MCP 重连仍待验证。
- FB-025/028/032 为既有 open 项，历史记录已归档；状态以 Active Board 为准，本次未擅自关闭。

#### 验证门禁

- v0.46.9最终Go test/vet/lint（0 issues）、完整Wails构建/契约、41解包文件/5资产hash、MCP/UI0.46.9/33工具及新诊断、两次隔离覆盖含schema、32/64位各15回归通过。最终包CLI独立库迁移通过；实装长测尚未执行。
- 最终保序代码已通过 `go test ./... -count=1`、`go vet ./...`、`golangci-lint run ./...`（0 issues）。
- RabbitMQ 真实 1000 条 FIFO、PowerShell parser 与故障/选择性 NACK 回归通过。
- v0.46.7 Wails build/contract、包内幂等、五组件资产 hash、32/64 位各 15 项安装器回归、隔离 NSIS 两次覆盖和最终 EXE 的 39 文件逐个 hash 验证通过。修正 NoBuild/NoZip 参数误按位置传递及 headless 版本元数据。
- `go test -race` 因 CGO 关闭且本机未发现 gcc/clang 而未运行成功。
- v0.46.8最终Go test/vet/lint（0 issues）、Wails build/contract、39解包文件/5资产hash、32/64位各15安装回归、双覆盖、MCP/UI0.46.8/32工具、完整600秒短测和PS恢复/锁/唤醒回归通过。旧失败不被新结果改写。
- AMQP 不确定写失败会禁止复用 publisher；自动 broker 重连不在本次范围，部分送达仍依赖目标幂等。

#### 历史记录

截至本次收口前的完整 793 行 MEMORY 已原样保留在 `.ai/docs/changelog.md` 的历史快照中；历史状态不覆盖本文件和 Active Board。

#### 改动记录

- 2026-09-10 15:08 | GPT-5 / backend-ai | 固定UTF-8驱动参数插值候选及真实精度/转义/回滚、历史宽表入口基准通过，记录未完成的双端性能验收与用户顺序；保留并撤销失败长测旧准入，FB-083失败闭合、FB-085/086持续open。

- 2026-09-10 10:14 | Codex test-ai | v0.46.12完整1800秒准入与恢复通过，七小时100650启动并验证持续推进；记录长测后续验收边界。

- 2026-09-10 09:36 | Codex test-ai | v0.46.12包与边缘实装、中心工作区候选验证通过，093438启动完整1800秒及有界防休眠，等待最终压力延迟门禁。

- 2026-09-10 09:30 | Codex backend-ai | FB-081事件日志有界多行SQL及真实回滚/字段对照验证通过，版本升0.46.12准备完整候选验证。

- 2026-09-10 09:12 | Codex test-ai | 记录083751全量一致性/账本通过但双向压力P99失败，自动恢复核验完成，FB-080继续隔离性能归因。

- 2026-09-10 08:40 | Codex test-ai | FB-079增加中心工作区模式及准入绑定回归，切换候选并启动完整1800秒测试；FB-078不再被中心安装阻塞。

- 2026-09-10 00:45 | GPT-5 / backend-ai -> test-ai | 修复事务提交可见性及回放查询吞错、ACK-only位点自写，补真实锁等待与运行时重试覆盖；v0.46.11全量test/vet/lint和包门禁通过，交接双端实装与完整短测，未降低FB-077门禁。

- 2026-09-10 00:22 | GPT-5 / test-ai | 完整负载234447暴露精确账本回放泄漏及中心压力P99超限；修复验收重试时区类型缺陷，新增隔离MySQL提交可见性实验并通过，更新FB-075/076/077，保留失败/恢复证据，无新长测。

- 2026-09-09 23:44 | GPT-5 / test-ai | 保留231556文件错误失败证据，复现并修复监督器原子状态替换的共享锁缺陷、补有界重试及错误栈；专项/监督器/全量门禁通过，恢复并confirm隔离76条残留，不改产品与负载门禁。

- 2026-09-09 23:15 | GPT-5 / test-ai | 按用户明确要求将中心/边缘buffer pool持久提高至8GiB/2GiB并核验完成，记录231352无负载预检失败，下一步恢复Agent后重测。

- 2026-09-09 23:11 | GPT-5 / test-ai | 按用户授权保留两端MySQL 512MiB buffer pool持久配置，核验扩容完成及可靠性参数不变；FB-071解除授权阻塞，完整负载复测仍待执行。

- 2026-09-09 20:50 | GPT-5 | test-ai完成原一路1800秒尝试并定位到恢复时缓存压力/SQL日志执行/Canal背压相关证据，失败保留并恢复；新增只读观察器，修测试专用50ms调度误失败，等待内存对照确认。

- 2026-09-09 20:20 | GPT-5 | test-ai扩展隔离分发基准的40 INSERT/20 UPDATE交错及50列/顺序核验；记录空闲位点反馈FB-073，未改产品配置或准入门禁。

- 2026-09-09 20:07 | GPT-5 / test-ai | 原一路600秒诊断195000完整通过但不放行长测；记录缓存增量与空库连接基准，增加50列混合Apply隔离对照测试，后续大历史日志基准进行中。

- 2026-09-09 19:50 | GPT-5 / test-ai | 记录v0.46.10四路短测恢复超时，恢复一路并confirm隔离500条；补MySQL缓存/刷盘计数采样用于增量归因，更新双端正式安装现场与FB-066/068/071状态。

- 2026-09-09 18:07 | gpt-5 | test-ai完成双端v0.46.9正式实装/配置保留/MCP/标准索引迁移；修复并关闭FB-069注册表检测夹具，全量test/vet/lint通过，180024短测运行中，尚不放行长测。

- 2026-09-09 17:32 | gpt-5 | backend-ai实现MCP诊断/幂等索引升级及包内schema；test-ai完成v0.46.9包门禁、实库对照和安装身份/30分钟准入保护，FB-067 closed、FB-068 open，FB-066 open/055 blocked不变。
- 2026-09-09 16:43 | gpt-5 | test-ai记录用户正式安装验收要求，修正旧方案运行状态和故障归因边界；FB-055 blocked、FB-066 open，未重启测试。
- 2026-09-09 16:40 | gpt-5 | test-ai确认正式测试snapshot超时失败及自动恢复；只读核查有限数据证据，撤销准入并验证拒绝重跑，FB-055 blocked、FB-066 open，未改产品行为。

- 2026-09-09 09:25 | gpt-5 | backend-ai/test-ai 实施 FB-052 批量下发、确认窗口和 CRUD/DDL 保序修复，追加分段基准及故障测试，修正短测夹具，归档历史 MEMORY；最终真实短测进行中。
- 2026-09-09 10:00 | gpt-5 | backend-ai/test-ai 修复 FB-054 批内幂等，完成 v0.46.7 安装包与门禁，按顺序交付含真实 UPDATE / 50 列的七小时方案；未启动新长测。
- 2026-09-09 13:17 | gpt-5 | test-ai 验收用户安装的 v0.46.7：启动同步、真实 20 条数据及日志核验通过；登记版本文字问题 FB-057，保留原规则，未启动长测。
- 2026-09-09 14:10 | gpt-5 | test-ai 实施50列真实UPDATE短测，登记FB-058 JSON精度和FB-059双向版本回退；阻止正式七小时，恢复原现场并隔离642条测试残留，恢复探针3/3通过；夹具仍WIP，报告与门禁已记录。
- 2026-09-09 15:10 | gpt-5 | backend-ai/test-ai按FB-060修复无损数字、回放标记、Edge日志装配和独立主键UPDATE确认批次；两轮候选失败如实保留，第二轮停载恢复后全50列/oracle通过；继续新包及完整夹具准入。
- 2026-09-09 16:16 | gpt-5 | test-ai完成v0.46.8最终包、恢复/文件锁/延迟分组/临时唤醒门禁，160102完整600秒通过并核发准入；正式wide7h_0909_161239已启动且双快照证实推进，FB-055保持running。

`MEMORY.md` 超过 100 行时，把旧的改动记录移动到这里。

## 2026-09-09 MEMORY 历史快照

以下内容完整保留归档前记录，不代表当前状态；当前状态以根 `MEMORY.md` 和 `AI_BOARD.md` 为准。

## MEMORY

Last updated: 2026-09-09 08:46 Asia/Singapore

### 当前阶段

- 2026-09-09 test-ai 完成 FB-051 正式双向跨夜长测检查并记录失败结果，报告见 `docs/v0.46.6-real-bidirectional-overnight-report-20260909.md`。`overnight-20260908-184000` 计划 13 小时，实际在约 3 小时 3 分时因首次 Edge Agent 停机恢复门禁超时结束：Edge -> Server 源/目标精确 `388,401/388,401`，Server -> Edge 为 `388,401/204,484`，缺口 `183,917`；双侧错误为 0，中心下行队列长期仅 0/1 条，说明瓶颈在 Server Canal CDC 分发而非 Edge apply。吞吐随事件日志增长由约 34 行/秒降至约 14 行/秒；代码证据显示 `ServerCanalDispatchRuntime` 对每条 change 串行执行 pending event-log upsert、单条 RabbitMQ publisher confirm、success event-log upsert，未使用批量发布。正式任务未执行后续 Server outage、DDL、幂等和最终 CRC，不能宣称长测通过。编排器已自动恢复原规则和双端 Agent；检查时清理 9 条精确闭合的测试下行残留，中心/Edge 相关队列归零，恢复探针 `1788914766156` 约 1.498 秒通过。FB-051 已关闭，产品性能优化转 backend-ai 的 FB-052。

- 2026-09-08 test-ai 启动 FB-051 真实双向 13 小时无人值守跨夜长测。新增 `scripts/lab-real-bidirectional-overnight.ps1`，在真实 `192.168.10.105 Edge <-> 192.168.10.103 Server` 上以隔离表和临时成对规则运行，覆盖 Edge->Server / Server->Edge 持续批量写入、映射 CRUD、冲突元数据、include/exclude、主键/列映射、双侧 Agent 停机积压恢复、自动单列 ADD/DROP、重复事件幂等、最终排空及精确计数/CRC 摘要，并持久记录 run-id、心跳、阶段、快照、事件和最终报告，结束时自动恢复原规则与 Agent。加速 smoke `smoke-20260908-183400` 已全部通过：双向各 882 行源/目标和摘要一致，三条中心队列与双侧错误均为 0。正式任务 `overnight-20260908-184000` PID `1780`，起止 `2026-09-08 18:40:17` 至 `2026-09-09 07:40:17 +08:00`，证据在 `.cache/real-bidirectional-overnight/overnight-20260908-184000/`；启动后连续心跳、双向首批和初始各 1,801 行写入已确认，按用户要求不再主动监控，FB-051 保持 running。

- 2026-09-08 test-ai 完成 FB-050 v0.46.6 真实安装后验收。目标 Edge `192.168.10.105` 的卸载注册版本和最新安装摘要均为 0.46.6，8 个安装步骤全通过；`C:\Program Files\NodeBridge\app\SyncAgent.exe` SHA256 `6B8F2B931941A41E6D61FB4B086D12FA36218E4B5A4BE392174EC3E4549D21D7` 与正式 staging 一致。MCP 0.46.6/32 工具、`edge-001` 身份、MySQL/CDC/RabbitMQ 配置、单条探针规则、MySQL 与本地/中心 RabbitMQ 均保留且健康。通过安装路径启动 Agent PID `5040`，SSH 断开 3 秒后仍驻留；本机 Server PID `43124`。安装版执行 20 条间隔 1200ms 的单行探针，min `48.015ms`、P50 `94.728ms`、P95 `127.240ms`、max `195.698ms`、avg `98.403ms`；源/目标/apply/SUCCESS event 各 20，双端队列、失败 ACK、近期错误和边缘失败事件均为 0。

- 2026-09-08 backend-ai/test-ai 完成 FB-049 内网单行延迟修复与 v0.46.6 交付。根因有两处：`workerConfig` 将 `retry_interval_seconds=10` 错用于正常空闲轮询，Edge CDC/upload 与 Server ingress 可叠加等待；Canal 客户端空闲长轮询为 1000ms。现将 Worker 正常空闲轮询固定为 100ms、错误退避仍遵守配置，并将 Canal 长轮询收紧为 100ms。最终包内二进制在真实 `192.168.10.105 -> 192.168.10.103` 链路执行 20 条间隔 1200ms 的单行探针，得到 min `62.569ms`、P50 `79.745ms`、P95 `141.854ms`、max `142.122ms`、avg `91.896ms`；源/目标/apply/SUCCESS event 均精确 20，队列、失败 ACK、近期错误和边缘失败事件均为 0。报告见 `docs/v0.46.6-real-latency-report-20260908.md`。安装包 `build/NodeBridge-beta-v0.46.6-20260908.exe`，大小 `339476224`，SHA256 `71E8D3BAD677E44ECB1A5D874DCA0FAF947B8BA21CC3C89F6B746DFDE922CAC9`；包内 MCP 0.46.6/32 工具、完整 Go 门禁、package smoke、NSIS 双覆盖均通过。当前 Server PID `43124`、Edge PID `12000` 已运行同哈希最终二进制；安装包也已复制到远端 `C:\Users\xx\Downloads`，远端安装目录尚未永久升级，需运行该安装包。

- 2026-09-08 test-ai 完成 FB-048 v0.46.5 异机高压力与断流恢复测试，报告见 `docs/v0.46.5-real-stress-report-20260908.md`。真实 Edge `192.168.10.105` 向本机模拟 Server 递增同步 1,000/10,000/50,000 行，分别在 21.248/41.803/110.567 秒精确排空，50,000 行档有效速率 452.22 行/秒、中心入口峰值 37,100。Server 停止时写入 20,000 行，中心业务表保持 0 且入口队列精确积压 20,000；重启后 35.216 秒排空，恢复速率 567.92 行/秒。四档源表、中心业务表、apply log 和 SUCCESS event log 均精确 81,000，所有队列回零，双端失败 ACK、中心本轮错误和边缘失败事件均为 0；Edge PID `7324`、Server PID `51540` 保持 running。测试数据保留在独立 ID 区间作为证据。

- 2026-09-08 test-ai 完成 FB-047 v0.46.5 异机覆盖安装与真实联通验收。边缘 `192.168.10.105` 安装摘要全部步骤通过，配置、`edge-001` 身份和探针规则保留，远端 `SyncAgent.exe` SHA256 `2ECA4B5E55F0EAD6E4218644318BA4D603F591A1B4709AB37D9EE199562FBD22` 与安装包 staging 一致，MCP 暴露 32 工具；安装后通过 MCP 启动 Agent PID `7324`，SSH 会话断开后仍驻留。当前 PC 模拟中心受控重启为 v0.46.5 PID `9688`。探针 `1788850057111` 在 5.13 秒到达；中心重启后探针 `1788850140994` 在 11.44 秒到达。两端 Agent/CDC/MySQL/RabbitMQ 均为 running，中心 `edge-001.downlink.q`、`server.cdc.ingress.q`、`server.dead.q` ready/unacked 全为 0，失败 ACK、近十分钟错误和边缘失败事件均为 0。FB-047 已关闭。

- 2026-09-08 backend-ai 完成 v0.46.5 覆盖升级包交付准备，包含 FB-045 长值截断修复与 FB-046 MCP 数据治理/自动单列 ADD/DROP。安装包 `build/NodeBridge-beta-v0.46.5-20260908.exe`，大小 `339478880` 字节，SHA256 `FAC49E2B0B2705C64178F14796A921FF9B4A752C79B2B80507C69293F3C07297`。包内 `SyncAgent.exe` MCP 真实进程冒烟通过 32 工具、配置加密、规则校验及 stdio 重连；NSIS 两轮隔离覆盖通过，运行中旧 Agent 被停止、二进制覆盖且配置/规则保留，证据 `.cache/nsis-upgrade/20260908-141314-394/upgrade-evidence.json`。Go test/vet/lint 与前端 test/build 已通过，随后移交 test-ai 做异机与真实链路验收。

- 2026-09-08 backend-ai 完成 MCP 数据库治理与自动列同步。实验室 MCP 从 27 增至 32 工具：新增结构化限量查询、`INSERT/UPDATE/DELETE` plan/apply、`ADD_COLUMN/DROP_COLUMN` plan/apply；不接受原始 SQL，只作用于配置库，`sync_*` 写保护，数据写入最多 100 行且事务内复核 `expected_rows`，结构写入要求当前 `plan_id`，禁止建表/删表/改列/索引/复合 ALTER。Canal、`SyncEvent`、Normalizer、Mapper、Loop、Runtime、Apply 已贯通规则授权的单列 ADD/DROP，规则新增 `schema_sync.add_columns/drop_columns`；删列除 `SERVER_TO_EDGE` 外必须恰好限定一个来源节点。`scripts/lab-governance-schema-e2e.ps1` 使用隔离 Edge A `3307/5673/11121`、Server `3309/5675`、模拟 Edge B `3308/5674`，连续两次真实 MCP + Canal + RabbitMQ + Apply 验证 INSERT、ADD、新列 UPDATE、DROP 全部通过，三端结果一致且最终队列为 0；报告在 `.cache/governance-schema-e2e/report.json`。未触碰现有 `3306/5672/11111` 拓扑，临时 Canal 已删除。Rules UI 三语开关仍由 frontend-ai 跟进 FB-046。

- 2026-09-08 test-ai/backend-ai 完成真实 Schema 演进 E2E，报告见 `docs/schema-evolution-e2e-report-20260908.md`。覆盖运行中源表新建且目标缺失、补建恢复、双方加列/删列、源/目标单边列变化、NOT NULL、列排除、普通列和主键列映射、跨 DDL 混合批次、CRUD INSERT/UPDATE/软删除、源表删后重建。DDL 明确不自动复制，结构不匹配时消息保留在入口队列，修复结构或规则后自动恢复。发现 `append_only INSERT IGNORE` 会把长值静默截断并 ACK；FB-045 已改为普通 `INSERT`，继续由 `sync_apply_log.event_id` 保证重放幂等。单元测试及真实窄列复测通过：目标过窄时 row/apply log 为 0、队列为 1，扩列后完整 `fixed-long-value` 在 5.5 秒恢复。`go test ./...`、`go vet ./...`、`golangci-lint run ./...` 全部通过。两端已恢复原单条探针规则并保持 running；最终探针 `1788840146648` 在 17.18 秒到达且队列为 0。测试表保留为只读证据。既有 v0.46.4 安装包不含 FB-045，不得宣称包含该修复。

- 2026-09-08 test-ai 按用户新部署状态复测 MCP 与真实数据链路：通过 SSH stdio MCP 连接边缘 `192.168.10.105`，`initialize` 协议 `2025-11-25`、27 个工具、配置/规则/表结构、MySQL/RabbitMQ 探测均通过；用 `nodebridge_start_agent` 启动边缘 Agent PID `4948`。随后经 SSH+MySQL 写入探针 `1788837621242 / mcp-live-1788837621242`，当前 PC 中心库在 10.58 秒后收到一致数据。两端 MCP 复核 Agent/CDC/MySQL/RabbitMQ 均为 `running`，所有相关队列为 0，失败事件为空。边缘版本 `0.46.4`，当前 PC 中心版本 `0.46.3`，跨版本同步通过；本轮未擅自升级中心。

- 2026-09-08 backend-ai 完成 v0.46.4 实机数据链路收口：修复受管 Canal 实例误写到根目录而未被 `canal/conf/<destination>` 扫描的问题，统一 destination 为规范化节点 ID，补齐实例参数、激活配置和旧路径清理；Windows 后台 SyncAgent 改为新进程组并脱离 OpenSSH job，SSH MCP 会话断开后进程继续运行；Edge 迁移补齐 `sync_ack_log` 并新增 schema 契约测试。`go test ./...`、`go vet ./...`、`golangci-lint run ./...`、前端 test/build、PowerShell 解析、NSIS 双覆盖和 15 项安装器回归全部通过。最终包 `build/NodeBridge-beta-v0.46.4-20260908.exe`，SHA256 `19B761CB9F68C936ADB17FBDFEC0C634AE3D6E393BECB8AABDE4F591C5417F9D`；边缘机安装摘要版本和步骤均通过。

- 2026-09-08 backend-ai/test-ai 在中心 `192.168.10.103` 与边缘 `192.168.10.105` 完成真实 `MySQL -> Canal -> RabbitMQ -> Apply -> MySQL` 自动传输验证。单条初测约 25.91 秒，10 条批量约 16.83 秒；安装 v0.46.4 后最终探针约 8.32 秒到达中心。中心 `edge-001.downlink`、`server.cdc.ingress`、`server.dead` 队列 ready/unacked 均为 0，失败 ACK 为 0，近 10 分钟 `sync_error_log` 为 0；中心探针业务表和 `sync_apply_log` 各 14 条。Windows SSH MCP、Agent 断线驻留和重连已闭环，仅剩 Mac 本机生成密钥并验收连接。

- 2026-09-08 test-ai 在开发电脑 `192.168.10.103` 完成本机模拟中心：v0.46.3 以 `/SkipSystemComponents` 安装成功，避免与 Docker 重复安装 RabbitMQ/MySQL；五月遗留配置已改名备份。复用 Docker `mysql:8.0.38` 和 `rabbitmq:3-management`，新增 `nodebridge-canal-main`，配置 `server-001/scada_center`、`/nodebridge-server`、`nb-server-sync` 与 `nb-edge-001`，防火墙仅允许 LocalSubnet 访问 5672。MCP 验证中心 Canal/MySQL/RabbitMQ/Agent 全部 `running`；边缘 `192.168.10.105` 已改指中心 `192.168.10.103`，本机和中心 RabbitMQ 均 `running`。同步规则仍为空，业务数据链路尚未验收。

- 2026-09-08 test-ai 复测重装后的第一台边缘机：目标 IP 更新为 `192.168.10.105`，SSH 公钥登录恢复；通过远端 `SyncAgent.exe mcp-stdio -lab-full-access` 完成 MCP `initialize`、27 工具列表、overview、配置摘要及 MySQL/RabbitMQ 实时探测。远端版本 `0.46.3`、节点 `edge-001`，MySQL 与本机 RabbitMQ 均 `running`，Log Web 关闭，Agent `stopped`；中心 RabbitMQ 仍配置为示例地址 `192.168.1.10:5672` 并连接超时，待取得中心服务器真实 IP 后修改。

- 2026-09-07 backend-ai 推进 FB-039 覆盖安装：发现并修复 NSIS 在复制文件前未停止运行中 NodeBridge/SyncAgent 的缺口，新增路径限定的 `upgrade-preflight.ps1` 和真实双安装测试。`scripts/test-nsis-upgrade.ps1` 已验证第一次安装、运行中旧 `SyncAgent.exe` 占用、第二次覆盖、二进制替换、配置字段/规则保留及两轮摘要通过，最终证据 `.cache/nsis-upgrade/20260907-174932-107/`；15 项安装器回归通过，证据 `.cache/installer-regression/d1f5e525f97a40f5a461b26205344d03/`。最终发布包 SHA256 `DD6852F3BA87550C8C1C708A52B8507875562E322EA3FE114DB4B8A6CAB1D6F9`。第一台边缘机 `192.168.10.102` TCP/22 在线但 SSH 在 banner 前主动断开，远端真实覆盖升级待连接恢复。

- 2026-09-07 backend-ai 完成 FB-038 v0.46.3 功能修订：新装配置不再预设节点身份、MySQL 凭据/数据库或同步规则，Log Web 默认关闭；解锁/退出及 NodeBridge 自有 RabbitMQ 密码统一为 `1234`。托管 RabbitMQ 用户按 `mode/node.id` 推导，边缘本机 `nb-<node>-local`、中心连接 `nb-<node>`、中心本机 `nb-server-sync`；MCP 保存先迁移本机 RabbitMQ 服务用户再落盘，并新增中心端 `nodebridge_ensure_server_edge_user`。升级保留 MySQL/Canal/规则并迁移旧 NodeBridge 密码。修复 UI 脱敏密码连接测试和 MCP `restart_agent:true`。该阶段生成的初始安装包已由 FB-039 覆盖安装修订包取代，不得继续分发；Go test/vet/lint、前端 test/build、15 项安装器回归、NSIS 列表和包内 27 工具 smoke 通过。

- 2026-09-07 test-ai 推进 FB-034 真实内网 MCP 验收：Windows 控制端通过 SSH 公钥连接第一台边缘机 `192.168.10.102`，MCP `initialize`、26 工具列表、配置读写和实时探测通过。已将 MySQL 配置改为 `127.0.0.1:3306/scada_edge`（凭据加密保存且回读脱敏），在目标机创建 `scada_edge` UTF8MB4 空库，`nodebridge_test_mysql` 与 overview 均为 running；当前/旧版两个规则文件均通过 MCP 清空，队列均为 0，SyncAgent 保持 stopped。完整 Agent 启停/重连和安装器二次安装/卸载仍待验证。

- 2026-09-07 backend-ai 完成发布文档与 GitHub 发行（FB-037）：新增根 README 和一台中心服务器、N 台边缘节点、Windows/Mac 调试电脑的内网部署与 SSH/MCP 授权手册。源码提交 `01d48ed` 已推送，`v0.46.3` 标签和 prerelease 已创建；Windows 安装包大小 `339408073` 字节，SHA256 `DD6852F3BA87550C8C1C708A52B8507875562E322EA3FE114DB4B8A6CAB1D6F9`，GitHub API 资产核对通过。

- 2026-09-07 backend-ai 修复目标机配置保存 Access denied（FB-036）：ProgramData/NodeBridge 原 ACL 只有 Users Write，缺少原子 rename 所需 delete-child。已远程按 `admin\xx` SID 授予继承 Modify，替换测试通过；SSH 防火墙改为 LocalSubnet，当前 Windows Codex 已注册 nodebridge MCP，真实 initialize/tools/list/overview/save_config_patch 通过。安装器加入安装账户 ACL 设置并生成 v0.46.2，SHA256 `620208E9B275EAFA1DA2C856C1BFE774F996A5A00FD402DAC6304A7EC3D6BA14`；32/64 位各 13 项回归、解包/资产/preflight 通过。

- 2026-09-07 backend-ai 修复异机 NSIS 安装失败（FB-035）：复现 32 位 PowerShell 的 ProgramFiles 与 ProgramFiles(x86) 同指 x86 目录，漏查用户已成功安装的 x64 Erlang。v0.46.1 改用 Sysnative PowerShell、ProgramW6432 检测、原生参数转义/进程句柄保留/300 秒安装后探测；安装日志按次写到 ProgramData/NodeBridgeInstallerLogs，卸载后保留。32/64 位各 12 项回归、Go test/vet/lint、五个离线资产 hash 和包内 MCP 26 工具 smoke 通过；真实目标机重装仍待 FB-034 验收。

- 2026-09-07 review-ai（Windows 第二任务）完成 MCP v0.46 实验室全配置管理：26 个工具，覆盖全部 Config/SyncRule 字段、密码/token/security、自启动、真实队列/失败事件/表结构、同步进程和受管拓扑；`-lab-full-access` 显式绕过 MCP 开关/管理解锁，SSH stdio 支持 Windows/Mac 控制端。新增 MCP 跨进程请求锁、Agent 独占锁/状态发现、原子配置/规则写入及协议错误处理。`go test ./...`、`go vet ./...`、golangci-lint、Wails/前端构建和真实 EXE smoke 通过；异机 SSH 与安装卸载尚未执行，交接 FB-033/FB-034。使用 `docs/mcp-service.md` 和安装目录 `app/mcp-lab-client-config.ps1`。

- 项目处于 V0.34 安装器隔离 VM 准备阶段，已在 Hyper-V 中准备私有交换机和关机状态测试 VM，后续只在 VM 内验证 Erlang/RabbitMQ/Canal 安装器，不触碰宿主机组件。
- 当前仓库已有 Go MVP 骨架、配置样例、迁移样例、RabbitMQ 核心接口、批量同步、表列映射、MySQL Apply Worker、Wails React UI 骨架、Wails UI API 契约和无感安装计划模型。
- 当前已跑通节点注册、动态分发、HTTP 配置下发、CRUD/软删/幂等/单向表语义、50 条批量同步、三套独立 RabbitMQ 单机 Docker 联调，以及 1 Server + 10 Edge 的 11 MySQL / 11 RabbitMQ 现场拓扑验证；V0.21 改为优先支持 Wails 托盘常驻，不做 Windows Service。
- AI 协作已收敛为根级活跃看板模型：`AI_BOARD.md` 与 `MEMORY.md` 同级，承载 frontend/backend/test/review 的 open/blocked/交接；`.ai/docs/` 只放稳定文档、闭合记录和归档材料。
- 同步分发策略已从固定方向改为规则可配置：`dispatch_target` 控制是否下发、下发给所有 ACTIVE Edge 或指定 Edge；Server-side CDC 已可把中心库变更直接下发 Edge，不重复 Apply 中心库。
- Wails 后端已开始消除 UI unknown 根因：Overview 暴露显式配置/节点/规则路径，时间 DTO 改为 RFC3339 字符串，规则 fallback 会落盘，外部 SyncAgent 输出进入日志文件并由 `GetLogs` 合并读取。
- 前端需求已由后端批复；MCP Server 先做预留配置开关，默认关闭，前端只可通过 Wails 后端接口切换，不启动真实 MCP runtime。
- Rules 编辑态已从超宽表格改为分组卡片；指定分发目标节点暂只能手填节点 ID，已在协作看板登记后端节点列表接口需求。
- Rules 空字段语义已在 UI 中显式化：空源节点表示全部源节点，空列映射表示同名列映射，非 SELECTED_EDGES 时目标节点由分发策略自动决定。
- Rules 编辑态已将空值默认语义改为醒目的提示条，并补齐目标库、目标表、包含列、排除列等空值说明。
- Rules 已接入后端 `sync_mode` 规则字段：新增规则默认 `crud_ordered`，`append_only` 仅用于历史倾倒/采集流水等 INSERT-only 表，UI 已补中英日危险提示。
- V0.45 性能优化 P1a 已完成：mixed batch 内连续 `append_only INSERT` 安全段按目标表批量写入，遇到 `crud_ordered` 立即停段，不跨越 CRUD 边界；已重建 `build/bin/SyncAgent.exe`，待 test-ai 用新二进制重跑 mixed 恢复压测。
- V0.45 性能优化 P2/P3/P4 后端已完成：Server Apply 支持 `sync.apply_lanes` 独立主键 lane 并行，`crud_ordered_compact` 只合并同一主键连续 UPDATE；compact 必须 Settings 全局开关和单条规则同时开启，禁止一键全开全部规则。
- 前端已配合 P4 compact：Settings 增加 `sync.enable_crud_compact` 全局开关，Rules 增加单条 `crud_ordered_compact` 选项；未开全局开关时不能新增选择 compact，且无一键批量开启入口。
- 2026-06-03 08:30 test-ai 复核 `mixed-15d-offline-002`：`recovery-summary.json` 显示 `drain completed`，但当前 `server.cdc.ingress.q=9000`、`server_event_log=7,340,000`（少 9000），`sync_apply_log=7,349,000`，且相关进程已退出。当前判定为尾段未完全清空，需要复核/补跑尾段后再定性通过。
- 2026-06-03 09:08 test-ai 补测复核 `mixed-15d-offline-002`：修正规则文件后补 `consume-batch-once` 手工 drain 9,000 条，最终核验 `server.cdc.ingress.q=0`、`server_rows=5,189,000`、`sync_apply_log=7,349,000`、`sync_event_log=7,349,000`、`sync_ack_log failed=0`，长测尾段闭环通过。
- MCP 目标已调整为远程 AI 受控查看和修改本机 NodeBridge 配置：前端需补 Settings/Manual 说明，后端需设计白名单写配置能力，当前有效交接见 `AI_BOARD.md` 的 FB-030/FB-031。
- 前端已完成 MCP 新目标说明：Settings、说明书和 `docs/mcp-service.md` 明确 MCP 默认关闭、stdio、不开放 HTTP 端口、不自动远程控制，前端只展示后端状态和 `mcp-client-config` 使用提示。
- MCP 后端已实现白名单写配置能力：`nodebridge_validate_config_patch` 先验证不落盘，`nodebridge_save_config_patch` 只接受非敏感字段，`nodebridge_save_sync_rules` 走规则校验，读写从磁盘取最新配置/规则，成功写和拒绝写均记录 `logs/mcp-audit.log`；`mcp-stdio` 强制要求 `mcp_server.enable=true`；已重建 `build/bin/SyncAgent.exe`。
- 2026-06-03 MCP 真实 stdio smoke 已通过：initialize、tools/list、dry-run、保存非敏感配置、拒绝敏感字段、保存规则、审计日志均通过；关闭态 `mcp_server.enable=false` 返回非 0 且输出明确 stderr 诊断。
- 前端状态栏低调展示“遵循 MIT 协议”，Settings 页说明 MIT 允许范围和使用者注意事项。
- V0.27 后端已冻结 SyncAgent 查找顺序和 stop-file 优雅停止协议，并新增 `GetAgentProcessStatus()` 供前端展示真实进程状态。
- 压力测试已改为单进程批量发布入口 `publish-stress-batch`，避免逐条启动 CLI 造成吞吐数据失真。
- 真实 Canal E2E 已通过：Edge A MySQL -> Canal -> 本地 RabbitMQ -> Server -> Edge B，以及 Server MySQL -> Canal -> Edge A / Edge B。
- V0.27 门禁已通过：Go 全量测试、`go vet`、Wails contract check、前端 TypeScript/Vite build、`SyncAgent.exe` 构建。
- 受管组件边界已落地为 `install-manifest.json` 模型；RabbitMQ/Canal 默认只管理带 NodeBridge 标识的 service、vhost、user、destination 和 config dir。
- Config 页已接入 CDC `managed/external`、本机安装、配置目录和服务名字段，避免安装边界只停留在后端配置里。
- 说明书页已新增独立“协议字段与关键字”章节，方向、分发、冲突、规则字段、空值默认和安全字段均提供三语解释。
- V0.28 Canal soak 已通过 20 条批量验证，覆盖 Canal 批量捕获、RabbitMQ 批量转发、Apply、offset 和失败 ACK 基础校验。
- V0.29 固定目录 package smoke 已通过：`build/bin/DataSync.exe`、`build/bin/SyncAgent.exe`、`config.yaml`、`sync-rules.yaml` 同目录可启动和校验。
- 前端已接入 `GetAgentProcessStatus()`、批量失败重试和死信只读预览；`FB-012`、`FB-014` 已关闭。
- V0.30 失败重试最小闭环已通过：失败 ACK 批量标记 `PENDING`、pending replay、Edge downlink apply 和死信只读预览均已跑通。
- V0.31 已新增受管组件执行器 alpha 和只读 stdio MCP alpha；当前不执行真实离线安装包，只写 manifest/Canal 配置并可初始化 NodeBridge RabbitMQ topology。
- Settings 页已接入受管组件安装计划与执行入口，展示 manifest 路径和操作结果，执行前要求管理解锁。
- 前端页面规整已推进：Config 改为分组编辑，Overview/Failures 危险操作增加确认，Settings 开关和分组统一，Logs 筛选按钮语义明确。
- V0.32 已新增 11 节点 soak 和断网恢复脚本，最小参数验证已通过，长参数可用于客户试用前压测。
- V0.33 已新增离线安装包预检：`installer-assets-check` 校验路径和 SHA256，`installer-command-plan` 输出 Erlang/RabbitMQ/Canal Windows 命令计划；本版不真实安装、不注册服务。
- V0.34 已下载 Microsoft Windows Server 2022 Evaluation ISO 到 `.cache/iso/`，创建 Hyper-V 私有交换机 `NodeBridge-V034-Isolated`，并改用 Gen1 VM `NodeBridge-V034-InstallerLab-G1` 继续安装测试；PowerShell Direct 已验证可操作 VM，`Clean-Windows-Installed` 快照已创建。
- V0.34 已生成交给测试 AI 的无 GUI 安装测试包：`build/NodeBridge-headless-installer-test-v0.34.0.zip`；默认预检不安装组件，`-ExecuteInstall` 可在 VM 内验证 Erlang/RabbitMQ 安装，Canal Service 当前仍为待实现。
- V0.34 headless installer 默认预检已在 `NodeBridge-V034-InstallerLab-G1` 通过，证据回收到 `.cache/v0.34-headless-preflight/`；真实 `-ExecuteInstall` 因缺真实 Erlang/RabbitMQ/Canal 离线包和 SHA256 暂阻塞。
- V0.35 已生成交给测试 AI 的无 GUI 安装测试包：`build/NodeBridge-headless-installer-test-v0.35.0.zip`；默认预检不安装组件，`-ExecuteInstall` 可验证 Erlang/RabbitMQ 安装和 RabbitMQ bootstrap，提供 WinSW 后可验证 Canal Service，`-VerifyOnly` / `-Uninstall` 用于闭环验证。
- V0.35 headless installer 版本验证已在 `NodeBridge-V034-InstallerLab-G1` 通过：package summary 和 managed manifest 均为 `0.35.0`；`-VerifyOnly` 在未安装状态下正确失败并记录 `NodeBridgeRabbitMQ` 缺失，证据在 `.cache/v0.35-headless-version-verify/`。
- AI 协作身份模型已扩展为 `frontend-ai`、`backend-ai`、`test-ai`、`review-ai`，操作前必须声明身份并通过根级 `AI_BOARD.md` Active Board 交流。
- 后端已新增只读 `GetNodeOptions()` Wails 契约，Rules 页 ACTIVE Edge 候选列表的后端阻塞已关闭，前端接入任务登记为 `FB-017`。
- V0.36 headless installer 包已生成：`build/NodeBridge-headless-installer-test-v0.36.0.zip`；新增真实离线包 catalog 生成脚本、RabbitMQ/Canal service 幂等支撑和 Canal 解压验证证据。
- V0.36 headless installer 已由 test-ai 在 `NodeBridge-V034-InstallerLab-G1` 验证默认预检、catalog 生成脚本、未安装 `-VerifyOnly` 失败和空环境双次 `-Uninstall`；真实 `-ExecuteInstall` 仍等待 Erlang/RabbitMQ/Canal/可选 WinSW 离线包。
- V0.36 官方离线包已下载到 `.cache/offline-assets/`，并生成带真实资产和 SHA256 catalog 的测试包 `build/NodeBridge-headless-installer-test-v0.36.0-with-assets.zip`，下一步交给 test-ai 在隔离 VM 内执行真实安装、验证、幂等和卸载闭环。
- V0.36 真实资产安装测试已在隔离 VM `NodeBridge-V034-InstallerLab-G1` 执行到 Erlang/RabbitMQ 阶段，但被 RabbitMQ 服务隔离和 CLI cookie 问题阻塞：VM 内运行的是默认 `RabbitMQ` 服务而非 `NodeBridgeRabbitMQ`，`rabbitmqctl` 认证失败，证据在 `.cache/v0.36-real-install-failure/`；宿主机 Erlang/RabbitMQ/Canal 未触碰。
- V0.37 已修复安装器真实 RabbitMQ 闭环：不删除未知或客户已有 `RabbitMQ` 服务，优先复用现有 broker；无 RabbitMQ 服务时才创建 `NodeBridgeRabbitMQ`；安装/验证前同步 Erlang cookie；VerifyOnly 改为校验 NodeBridge vhost/user/queue。
- V0.37 真实资产安装测试已在隔离 VM 继续执行，确认既有 `RabbitMQ` 复用、Erlang cookie 同步和真实 summary 修复生效；当前新阻塞为 `rabbitmq-bootstrap` 连接 `/nodebridge-edge` vhost 返回 403 no access，证据在 `.cache/v0.37-real-install-failure/`。
- 按用户要求补测无 RabbitMQ 干净系统完整安装路径：VM 已恢复到 `Before-V036-Real-Install` 并确认无 RabbitMQ/NodeBridge/Canal 服务；V0.37 完整安装 28 分钟后中断检查发现仅安装出默认 `RabbitMQ` 服务，未写 summary、未进入 V0.37 cookie/bootstrap 阶段，证据在 `.cache/v0.37-clean-install-interrupted/`。
- V0.38 已按看板和用户要求恢复 `Clean-Windows-Installed` 干净快照验证完整安装路径；确认无 Erlang/RabbitMQ/NodeBridge 目录和服务后执行 `-ExecuteInstall`，当前阻塞在 `install-erlang`：Erlang 安装进程未超时但 ExitCode 为 null，被脚本判为失败，证据在 `.cache/v0.38-clean-install-failure/`。
- V0.38.1 已在干净 VM 执行：Erlang/RabbitMQ installer ExitCode null 修复生效并记录 `exit_code_missing_but_detected=true`；首次安装仍因 RabbitMQ ready 90s 超时失败，二次安装可完成 RabbitMQ bootstrap，但 Canal WinSW 服务 StartPending 后停止且 ExitCode=1067，证据在 `.cache/v0.38.1-clean-install-failure/` 和 `.cache/v0.38.1-clean-rerun-canal-failure/`。
- V0.38.2 已按 test-ai 反馈修复安装器：RabbitMQ CLI 调用前注入 Erlang PATH 并延长 ready 等待；Canal Service 注册前检查 Java，缺 Java 默认跳过并写证据，强验时明确失败；已生成基础包和 with-assets 测试包交回 test-ai。
- V0.38.2 已在干净 VM 复测：第一轮 `-ExecuteInstall`、`-VerifyOnly` 和 `-Uninstall` 通过，缺 Java 场景正确跳过 Canal Service；二次 `-ExecuteInstall` 仍在 `rabbitmq-bootstrap` 幂等路径失败且 message 仅 `Error:`，证据在 `.cache/v0.38.2-clean-second-install-failure/` 和 `.cache/v0.38.2-clean-validation/`。
- V0.38.3 已按 test-ai 二次安装反馈修复安装器：每次运行清理旧 runtime 证据，RabbitMQ bootstrap 改为先查 vhost/user 再创建，避免重复创建错误文本导致幂等失败；已生成新基础包和 with-assets 测试包。
- V0.38.3 已在 `Clean-Windows-Installed` 干净 VM 跑通安装器闭环：第一轮 `-ExecuteInstall`、`-VerifyOnly`、二次 `-ExecuteInstall`、`-Uninstall` 均通过，卸载后无 RabbitMQ/NodeBridge/Canal 服务，证据在 `.cache/v0.38.3-clean-validation/`。
- V0.39.0 已在安装器中加入可选 Java/JRE 离线资产支持：catalog 可声明 `component=java`，脚本支持 MSI 安装与 Java 探测；当前 with-assets 包未包含 Java 文件，强验 Canal Service 需后续补 Java MSI。
- V0.40.0 已下载 Windows x64 JRE MSI 到离线资产缓存，新增 `package-headless-installer-with-assets.ps1`，并生成包含 Erlang/RabbitMQ/Canal/WinSW/Java 的 with-assets 测试包，可交给 test-ai 强验 `NodeBridgeCanal` Windows Service。
- V0.40.0 Canal Service 强验已由 test-ai 在 `Clean-Windows-Installed` 干净 VM 执行到第一轮 `-ExecuteInstall -RequireCanalService`，Erlang/RabbitMQ/bootstrap/Canal 解压通过，但 Java MSI 前置安装未产生 exit code 且 `java.exe` 未安装，导致 `canal-service` 失败；证据在 `.cache/v0.40.0-clean-validation/`，已移交 backend-ai 修复。
- V0.40.1 已修复 Java MSI 安装检测逻辑：扩大 `java.exe` 搜索范围到 `Program Files\Eclipse Adoptium` / `Program Files\Java` / Microsoft JRE，MSI 参数加入 `/norestart ADDLOCAL=FeatureMain,FeatureEnvironment`，缺 exit code 时 probe 重试 90 秒，并输出 `msi-java.log` / `java-search.json`；已生成新的 with-assets 包交回 test-ai。
- V0.40.1 Canal Service 强验已由 test-ai 在 `Clean-Windows-Installed` 干净 VM 复测，第一轮 `-ExecuteInstall -RequireCanalService` 仍阻塞在 Java MSI 前置安装：`msi-java.log` 返回 1620 且有 2203/-2147286960，`java.exe` 未安装；Erlang/RabbitMQ/bootstrap/Canal 解压通过，证据在 `.cache/v0.40.1-clean-validation/`，已移交 backend-ai 继续修复。
- V0.40.2 已将 Java 资产切换为解压式 JRE zip：`packages/OpenJDK-jre.zip` 解压到 `%ProgramData%\NodeBridge\java`，安装器直接从该目录探测 `java.exe`，不再依赖 `msiexec`；已生成新的 with-assets 包交回 test-ai。
- V0.40.2 Canal Service 强验已由 test-ai 在 `Clean-Windows-Installed` 干净 VM 复测，Java zip 解压和 `java.exe` 探测通过，Erlang/RabbitMQ/bootstrap/Canal 解压通过；当前阻塞为 `NodeBridgeCanal` 启动后停止，Win32 ExitCode=1067，日志显示 JRE 17 不识别 `PermSize=128m`，证据在 `.cache/v0.40.2-clean-validation/`，已移交 backend-ai 继续修复。
- V0.40.3 已确认 V0.40.2 Canal Service 阻塞根因：Canal 1.1.8 `startup.bat/startup.sh` 带 Java 17 已删除的 `PermSize/MaxPermSize` 参数；安装器现会在 Canal 解压后清洗这些旧 JVM 参数并写出 `runtime/canal-startup-patch.json`，已生成 `build/NodeBridge-headless-installer-test-v0.40.3-with-assets.zip` 交给 test-ai 复测。
- V0.40.3 Canal Service 强验已由 test-ai 在 `Clean-Windows-Installed` 干净 VM 复测：第一轮 `-ExecuteInstall -RequireCanalService` 通过，`VerifyOnly` 通过，说明 Java 17 参数清洗和 Canal 服务启动已生效；二次 `-ExecuteInstall -RequireCanalService` 阻塞在 `canal-asset-extract`，运行中的 `lib/canal.server-1.1.8.jar` 无法 unlink 覆盖，证据在 `.cache/v0.40.3-clean-validation/`，已移交 backend-ai 修复 Canal 解压幂等。
- V0.40.4 已修复 Canal 二次安装热覆盖：当 `NodeBridgeCanal` 已 Running 且 Canal 目录完整时，安装器跳过重解压，复用现有目录并写出 `runtime/canal-asset-extract.json`；已生成 `build/NodeBridge-headless-installer-test-v0.40.4-with-assets.zip` 交给 test-ai 复测完整安装/验证/二次安装/卸载闭环。
- V0.40.4 Canal Service 强验已由 test-ai 在 `Clean-Windows-Installed` 干净 VM 复测：第一轮 `-ExecuteInstall -RequireCanalService`、`VerifyOnly`、二次 `-ExecuteInstall -RequireCanalService`、二次 `VerifyOnly` 均通过；`-Uninstall` 阻塞在 `verify-uninstall`，message 为 `NodeBridgeCanal still exists`，延迟 10 秒后服务消失，证据在 `.cache/v0.40.4-clean-validation/`，已移交 backend-ai 修复卸载等待/确认逻辑。
- V0.40.5 已修复卸载服务删除确认延迟：`Uninstall` 和 `verify-uninstall` 会等待 NodeBridge 自有服务从 SCM 消失，并输出 `NodeBridgeCanal-delete-wait.json` / `NodeBridgeRabbitMQ-delete-wait.json`；已生成 `build/NodeBridge-headless-installer-test-v0.40.5-with-assets.zip` 交给 test-ai 复测完整闭环。
- V0.40.5 Canal Service 强验已由 test-ai 在 `Clean-Windows-Installed` 干净 VM 通过完整闭环：第一轮 `-ExecuteInstall -RequireCanalService`、首次 `VerifyOnly`、二次 `-ExecuteInstall -RequireCanalService`、二次 `VerifyOnly`、首次 `Uninstall`、二次 `Uninstall` 均通过；卸载后无 RabbitMQ/NodeBridge/Canal 服务，证据在 `.cache/v0.40.5-clean-validation/`，`FB-024` 已关闭。
- NSIS beta 安装器制作已委托给 backend-ai，活跃任务为 `AI_BOARD.md` 的 `FB-032`：复用 V0.40.5 headless with-assets 链路，真实系统组件安装继续只在隔离 VM 验证；本机仅允许在不冲突时用 Docker MySQL/RabbitMQ 做非破坏 smoke。
- 2026-06-03 12:01 backend-ai 已按实机反馈修复 NSIS beta 后续问题：快捷方式显式使用 `NodeBridge.ico`，MySQL/RabbitMQ/CDC/同步参数保存后提示需重启同步进程，RabbitMQ 连通性拆分本地与远端探测，Config 页新增 RabbitMQ 队列初始化入口；新包 `build/NodeBridge-beta-v0.45.0-20260603.exe` SHA256 `60AB33C6B71F14ABC985129FAB0D0A0072B03F1E0E418E3EEF55E0FF0A9FEAFA`。
- 2026-06-03 14:06 backend-ai 已修复实机安装和 RabbitMQ 403 根因：NSIS 安装遇到安全草稿/不完整 `%ProgramData%\NodeBridge\config.yaml` 时备份并写入完整默认配置；默认 RabbitMQ URL 改为 `nb-edge-001-local:nodebridge_test@127.0.0.1:5672/%2Fnodebridge-edge`，匹配安装器创建的账号和 vhost；NSIS 打包强制使用仓库示例配置。新包 `build/NodeBridge-beta-v0.45.0-20260603.exe` SHA256 `83362DFC51E56A21A56383F8639FB37E586DE7745FD6CBE037700B97C20B9CEB`。
- 2026-06-03 14:20 backend-ai 已修复 NSIS 安装 `system-components` 1 秒失败 exit code `-196608`：headless component 调用不再用 `Start-Process -ArgumentList` 传递带空格脚本路径，改为 PowerShell 参数数组直接执行并记录 stdout/stderr。新包 `build/NodeBridge-beta-v0.45.0-20260603.exe` SHA256 `947599EA3F2F2620EE0164A6F20E5918EFBD1661FB16176E05B38B5E2972ADD7`。
- NSIS beta 安装器源码、staging 和最终 exe 已生成：已通过 winget 安装 NSIS 3.12，管理端 exe 已从旧 `DataSync.exe` 修正为 `NodeBridge.exe`，且发布脚本改为 `wails build` 正式构建，修复测试机 Wails build tags 弹窗；NSIS 默认不再强制 `-RequireCanalService`，避免普通测试机因 Canal Service 强验失败直接中断 UI 安装；当前包 `build/NodeBridge-beta-v0.45.0-20260603.exe` SHA256 `C6A0A106C4EFA449E16869F2DB9B3AB76B2C3E009735D37100194C6DC9D90F17`，真实系统组件安装仍未在宿主机执行。
- 已新增并验证 90 天等价长测 harness：单机 Docker 模拟 Edge/Server，真实 MySQL binlog + Canal CDC，6 张采集表 x 42 点位字段，脚本入口 `scripts/longtest-90d.ps1`，文档 `docs/longtest-90d.md`；`prepare`、`smoke -RowsPerTable 2`、轻量 `query` 和 `archive` 已通过，证据在 `.cache/longtest-90d/smoke-001/`。
- 90 天等价长测 harness 已按用户要求改为默认 `time-interleaved` 写入：按 `collected_at` 时间窗口推进，每个窗口轮转 6 表，单表内时间顺序递增，并输出 `insert-plan.json`、`ordering-check.json`、`arrival-shape.json`；已删除旧 longtest Docker 容器和 volume 后重建干净环境，`clean-interleaved-smoke-001` 通过 18/18 同步，证据在 `.cache/longtest-90d/clean-interleaved-smoke-001/`。
- 90 天等价长测第一轮 1 天等价 `day1-interleaved-001` 已执行并判为性能阻塞：Edge 6 表写满 172,800 行，顺序/形态检查通过，Edge 队列清空，失败 ACK 0；Server Apply 停止时 151,951/172,800，`server.cdc.ingress.q` 剩 20,850，后段吞吐约 17 行/s，不满足 1 天恢复追平验收；证据在 `.cache/longtest-90d/day1-interleaved-001/`，看板 `FB-025` 已标记 blocked，需 backend-ai 优化 Apply/ACK 批处理后再跑 7d/90d。
- V0.41 已完成 Server Apply 保序批处理修复：RabbitMQ batch 改为数据库提交后 ACK，SQL `ApplyBatch` 使用单事务、savepoint 和成功前缀提交，`EDGE_TO_SERVER + dispatch_target=NONE` 成功日志跳过大 `event_payload`；同时修复 longtest smoke/seed/realtime/recovery 未重新 build `SyncAgent.exe` 的测试夹具问题。
- V0.41 已用当前二进制跑通真实 CDC 6,000 行 smoke：`v041-current-binary-6k` 中 6 张表 Edge/Server 均为 1,000 行，队列清空、失败 ACK 0、顺序违规 0，Server `sync_event_log.event_payload` 全部为 NULL；下一步交 test-ai 重跑 1 天等价、7 天和 90 天长测。
- V0.41 30 天等价长测 `month30-v041-001` 已由 test-ai 执行并阻塞：Edge 6 表总计 5,184,000 行生成成功，`time-interleaved` 顺序/形态检查通过；Edge SyncAgent 首次运行出现 Canal ACK panic（`batchId:4 is not exist`），受控重启后队列清空但 Server 仅 5,780 行，`sync_apply_log`/`sync_event_log` 对 `collect_data_01` 记录 28,900 条且业务表仅 `collect_data_01=5,780`、其余 5 表为 0；证据在 `.cache/longtest-90d/month30-v041-001/`，看板 `FB-025` 已重新标记 blocked，需 backend-ai 修复 Canal ACK/offset 恢复和 apply log 与业务写入一致性。
- V0.42 已针对 `month30-v041-001` 修复 CDC 恢复一致性：CDC `event_id` 改为稳定 ID，Canal ACK 成功后才保存 offset，`batchId:* is not exist` 不再导致 agent 崩溃，Server CLI 消费默认失败重投，longtest Canal 配置改为显式挂载并校验；`v042-fix-smoke-600` 和 `v042-fix-smoke-6000` 真实 CDC smoke 已通过。
- V0.42 长测复测由 test-ai 执行：`day1-v042-001` 1 天等价通过，Edge/Server 6 表总计均为 172,800 行，顺序检查 0 违规，队列清空，查询性能达标；证据在 `.cache/longtest-90d/day1-v042-001/`。
- V0.42 30 天等价 `month30-v042-001` 仍阻塞：Edge 6 表总计 5,184,000 行完整写入，但 Server 最终仅 69,360 行；`sync_apply_log`/`sync_event_log` 均为 69,360，失败 ACK 为 0，最终 Edge/Server RabbitMQ 队列均为 0，未复现 V0.41 的 ACK panic。判定为大批量 CDC 抓取或 Canal offset 完整性问题，证据与分析在 `.cache/longtest-90d/month30-v042-001/analysis-results.json`，看板 `FB-025` 已转交 backend-ai blocked。
- V0.43 已修复 V0.42 30 天阻塞的高概率根因：Canal `GetWithoutAck` 返回带 batchId 但无 ROWDATA 的 batch 时，runtime 现在也会 ACK 并推进 offset；`ConvertWithlinMessage` 会从非 ROWDATA entry 保留 binlog offset；已通过 `v043-empty-batch-smoke-6000` 真实 CDC smoke。
- V0.43 30 天等价 `month30-v043-001` 已由 test-ai 复测并仍阻塞：Edge 6 表总计 5,184,000 行完整写入，顺序检查 0 违规，arrival shape 0 坏窗口；Server 最终仅 46,240 行，`sync_apply_log`/`sync_event_log` 均为 46,240，失败 ACK 为 0，最终 Edge/Server RabbitMQ 队列均为 0。Canal offset 停在 `mysql-bin.000003:18740991`，已回收 Canal server `logs/conf/meta` 到 `.cache/longtest-90d/month30-v043-001/canal-internal/`，分析在 `.cache/longtest-90d/month30-v043-001/analysis-results.json`；`FB-025` 继续 blocked 并转回 backend-ai。
- V0.44 已定位 V0.43 30 天阻塞根因：Canal server 在长测中按 idle timeout 关闭 TCP client 后，SyncAgent Canal runtime 没有重建 connector，且大 batch 发布期间 Canal client 60s idle timeout 过短；现已在 fetch/commit 错误后 reset Source，下次 worker tick 重新 Connect/Subscribe，并把 Canal client net idle timeout 提到 1h；最终通过 `v044-reconnect-idle-smoke-6000` 真实 CDC 6,000 行 smoke。
- V0.44 已由 test-ai 执行零/冒烟级 30 天等价回归 `month30-v044-001`：Edge 6 表总计 5,184,000 行完整写入，Server 从 0 增至 116,908 行，越过 V0.43 的 46,240 和 V0.42 的 69,360 旧失败点，失败 ACK 为 0，说明重连/idle-timeout 修复方向有效；本轮为节省时间主动停止，停止时 Edge 队列 61,859、Server 队列 198，不能作为 30d 全量通过，证据在 `.cache/longtest-90d/month30-v044-001/analysis-results.json`。
- V0.44 分层长测已由 test-ai 开始实施并持续留证：阶段 0 `staged-v044-p0-smoke-001` 通过，12/12 同步、失败 ACK 0、队列清空；阶段 1 `staged-v044-p1-day1-001` 通过，Edge=Server=172,800、失败 ACK 0、队列清空，查询性能达标，证据目录内均有 `test-report.json`。
- V0.44 阶段 2 7 天全量 `staged-v044-p2-day7-001` 已启动但未验收：Edge 1,209,600 行完整写入，采样期间 Server 从 0 增至 57,066，失败 ACK 为 0，队列仍活跃，未观察到正确性失败；因未跑到 Edge=Server=1,209,600 且队列清空，阶段 2 状态为 paused/not accepted，需无人值守跑满后才能进入 30d。
- 90 天等价长测 harness 已新增 `DrainMode=agents`：使用常驻 Edge/Server `SyncAgent.exe run` 进行 Canal CDC、Edge upload forward 和 Server apply，替代 30 天复测里的反复 one-shot CLI drain；`scripts/test-coverage-core.ps1` 固化核心包覆盖率门禁，当前核心覆盖率 74.9% >= 70%，全仓库原始覆盖率 53.8% 留证于 `.cache/coverage-summary.txt`；优化后 smoke `smoke-agents-coverage-001` 通过 12/12、重复 event_id=0、失败 ACK=0；30 天积压复测 `staged-v044-month30-agents-001` 已后台启动，PID=33244，证据目录 `.cache/longtest-90d/staged-v044-month30-agents-001/`。
- V0.44 backend-ai 已针对 30 天积压释放瓶颈优化 Edge 上传：RabbitMQ publisher 新增 `PublishBatch`，Edge upload batch 在保持单队列顺序的前提下先顺序发布整批，再等待整批 publisher confirm，成功后 ACK 源队列，失败时整批 requeue；验证已通过 `go test ./internal/rabbitmq ./internal/syncruntime`、`go test ./...`、`go vet ./...`、核心覆盖率 75.1% >= 70%，并重建 `build/bin/SyncAgent.exe`。当前 `staged-v044-month30-agents-001` 后台长测仍是旧进程，需新跑或重启 agents 才能观察优化收益。
- 已在 `staged-v044-month30-agents-001` 同一 30 天积压现场切换到批量 confirm 新 SyncAgent：旧 agents PID=29060/17272 停止，新 agents PID=37132/7072 使用 `.cache/longtest-90d/bin/SyncAgent.exe` 继续释放同一批积压；短窗 Server apply 从约 30.9 rows/s 提升到约 141.0 rows/s，Server 收到/落库综合速率约 188.0 msg/s，提升约 4.56x-6.08x。新证据在 `.cache/longtest-90d/staged-v044-month30-agents-001/batchconfirm-samples.csv` 和 `batchconfirm-analysis.json`；当前瓶颈转向 Server MySQL apply。
- 已为批量 confirm 后的 30 天量级测试启动无人值守完成监控：`scripts/watch-longtest-batchconfirm.ps1` 以 PID=18336 后台运行，每 60 秒采样 Edge/Server 行数、Edge/Server RabbitMQ 队列和 agent 存活状态，直到 Server=5,184,000 且队列清空或 agent 退出；证据写入 `.cache/longtest-90d/staged-v044-month30-agents-001/batchconfirm-watch.csv`，完成摘要写入 `batchconfirm-watch-summary.json`。监控脚本已修复 Windows PowerShell/Docker stderr 和路径空格兼容问题；最新首条采样 Server=1,431,885/5,184,000、Edge 队列=583,111、agents 37132/7072 存活。
- 2026-05-31 01:58 巡检确认批量 confirm 后 30 天现场仍在推进：agents 37132/7072 存活，Server=1,451,885/5,184,000，Edge 队列约 565,874，Server ingress=10,000；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-015740.json`。无人值守监控脚本仍受 Windows PowerShell native stderr 行为影响，当前以直接 Docker/MySQL/RabbitMQ 快照作为权威进度证据。
- 2026-05-31 02:00 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,471,885/5,184,000，Edge 队列约 565,792，Server ingress=0，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020008.json`。
- 2026-05-31 02:01 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,481,885/5,184,000，Edge 队列约 552,103，Server ingress=10,000，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020139.json`。
- 2026-05-31 02:03 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,491,885/5,184,000，Edge 队列约 548,734，Server ingress=10,000，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020319.json`。
- 2026-05-31 02:04 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,501,885/5,184,000，Edge 队列约 544,434，Server ingress=10,000，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020444.json`。
- 2026-05-31 02:06 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,511,885/5,184,000，Edge 队列约 540,182，Server ingress=10,000，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020609.json`。
- 2026-05-31 02:08 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,531,885/5,184,000，Edge 队列约 536,888，Server ingress=0，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020753.json`。
- 2026-05-31 02:09 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,541,885/5,184,000，Edge 队列约 533,249，Server ingress=0，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-020922.json`。
- 2026-05-31 02:10 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,551,885/5,184,000，Edge 队列约 528,780，Server ingress=0，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-021043.json`。
- 2026-05-31 02:12 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,561,885/5,184,000，Edge 队列约 524,971，Server ingress=0，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-021215.json`。
- 2026-05-31 02:14 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,571,885/5,184,000，Edge 队列约 511,860，Server ingress=10,000，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-021355.json`。
- 2026-05-31 02:15 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,581,885/5,184,000，Edge 队列约 507,649，Server ingress=10,000，错误日志为空；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-021519.json`。
- 2026-05-31 02:23 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 存活，Server=1,641,885/5,184,000，Edge 队列约 480,817，Server ingress=10,000；新增并验证可靠 watchdog `scripts/longtest-30d-watchdog.ps1`，后台 PID=11172，每 5 分钟写 `.cache/longtest-90d/staged-v044-month30-agents-001/watchdog-progress.csv` 和 `watchdog-summary.json`。
- 2026-05-31 02:25 巡检确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,661,885/5,184,000，Edge 队列约 479,094，Server ingress=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-022540.json`。
- 2026-05-31 02:28 watchdog 正常采样批量 confirm 后 30 天现场：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,681,885/5,184,000，Edge 队列约 461,409，Server ingress=10,000；`watchdog-progress.csv` 和 `watchdog-summary.json` 已更新。
- 2026-05-31 02:30 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,701,885/5,184,000，Edge 队列约 459,304，Server ingress=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-023039.json`。
- 2026-05-31 02:32 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,711,885/5,184,000，Edge 队列约 456,368，Server ingress=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-023218.json`。
- 2026-05-31 02:33 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,721,885/5,184,000，Edge 队列约 442,512，Server ingress=10,000；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-023349.json`。
- 2026-05-31 02:35 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,731,885/5,184,000，Edge 队列约 438,674，Server ingress=10,000；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-023522.json`。
- 2026-05-31 02:38 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,751,885/5,184,000，Edge 队列约 430,289，Server ingress=10,000；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-023814.json`。
- 2026-05-31 02:39 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,771,885/5,184,000，Edge 队列约 426,667，Server ingress=10,000；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-023953.json`。
- 2026-05-31 02:43 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,791,885/5,184,000，Edge 队列约 420,100，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-024303.json`。
- 2026-05-31 02:45 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,811,885/5,184,000，Edge 队列约 408,274，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-024510.json`。
- 2026-05-31 02:46 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,821,885/5,184,000，Edge 队列约 405,220，Server ingress=0，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-024649.json`。
- 2026-05-31 02:48 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,831,885/5,184,000，Edge 队列约 401,939，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-024825.json`。
- 2026-05-31 02:50 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,841,885/5,184,000，Edge 队列约 389,840，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-025023.json`。
- 2026-05-31 02:52 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,861,885/5,184,000，Edge 队列约 386,137，Server ingress=0，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-025200.json`。
- 2026-05-31 02:53 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,871,885/5,184,000，Edge 队列约 383,369，Server ingress=3,711，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-025340.json`。
- 2026-05-31 02:55 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,881,885/5,184,000，Edge 队列约 369,886，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-025516.json`。
- 2026-05-31 02:56 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,891,885/5,184,000，Edge 队列约 366,224，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-025651.json`。
- 2026-05-31 02:58 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,911,885/5,184,000，Edge 队列约 363,437，Server ingress=0，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-025837.json`。
- 2026-05-31 03:00 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,921,885/5,184,000，Edge 队列约 350,704，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-030017.json`。
- 2026-05-31 03:01 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,928,874/5,184,000，Edge 队列约 347,334，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-030154.json`。
- 2026-05-31 03:03 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,947,751/5,184,000，Edge 队列约 343,964，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-030333.json`。
- 2026-05-31 03:05 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,957,751/5,184,000，Edge 队列约 340,999，Server ingress=0，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-030514.json`。
- 2026-05-31 03:06 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,967,751/5,184,000，Edge 队列约 328,529，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-030659.json`。
- 2026-05-31 03:08 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,977,751/5,184,000，Edge 队列约 325,152，Server ingress=10,000，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-030839.json`。
- 2026-05-31 03:10 直连快照确认批量 confirm 后 30 天现场继续推进：agents 37132/7072 与 watchdog PID=11172 存活，Server=1,997,751/5,184,000，Edge 队列约 321,853，Server ingress=0，dead/retry=0；手动快照写入 `.cache/longtest-90d/staged-v044-month30-agents-001/manual-snapshot-20260531-031018.json`。
- 2026-05-31 03:24 已新增按表 `sync_mode` 同步语义：默认 `crud_ordered` 保持原增删改保序路径，`append_only` 用于历史倾倒/采集流水表，只接受 INSERT，并在 Server Apply 中使用多行批量插入与批量 apply log；`configs/longtest/sync-rules.yaml` 的 6 张采集表已切到 `append_only`。
- 2026-05-31 03:24 `append_only` 优化验证通过：`go test ./internal/rules ./internal/mapper ./internal/apply`、`go test ./...`、`go vet ./...`、核心覆盖率 75.1% >= 70%、`npm run build` 均通过；`appendonly-smoke-001` 18/18 同步通过，新的 30 天量级后台复测 `appendonly-month30-001` 已启动，PID=8536，证据目录 `.cache/longtest-90d/appendonly-month30-001/`，早期快照 Edge=1,040,000/5,184,000、Server=0，仍在 seed 阶段。
- 2026-05-31 04:09 继续拆解后确认 Edge 端 Canal->RabbitMQ 仍是逐条 publish confirm，已将 `CanalUploadRuntime` 改为优先 `PublishBatch`，整批成功后才提交 Canal offset；同时 Server append-only apply 对 time-interleaved 批次按目标表分组多行插入。验证：`go test ./...`、`go vet ./...`、核心覆盖率 74.7% >= 70%。`appendonly-month30-003` 因 Edge RabbitMQ Docker cookie 权限异常在 prepare 阶段失败并清理；最终后台复测为 `appendonly-month30-004`，PID=41432，证据目录 `.cache/longtest-90d/appendonly-month30-004/`，04:09 早期快照 Edge=290,000/5,184,000、Server=0，仍在 seed 阶段。
- 已新增项目技能 `.ai/skills/installer-vm-test/SKILL.md` 和详细命令 runbook `docs/installer-vm-test-runbook.md`，固化隔离 VM 安装器测试闭环、干净快照恢复、证据回收和看板回写规则，方便后续 test-ai 接手。
- MCP Service 开关已改为持久开关：默认关闭，启用后写入 YAML，NodeBridge 重启后保持启用，只能由用户手动关闭；stdio MCP 仍不占端口，且只开放只读诊断工具。
- 管理解锁默认有效期已从 10 分钟调整为 1 天；前端继续显示 `GetAuthState().expires_at`，手动锁定仍可立即回到只读模式。
- Rules 页已接入 `GetNodeOptions()`，`SELECTED_EDGES` 可勾选 ACTIVE Edge 候选并保留手填节点 ID 兜底，`FB-017` 已关闭。
- 已新增并补强 `docs/frontend-wails-dev.md`，记录可直接复制执行的完整 Wails dev 启动命令、vfox/项目内工具路径、前端门禁和 Wails/native smoke 检查点。
- 测试凭据已集中到 `docs/test-credentials.md`：Docker lab、longtest、headless installer 和隔离 VM 密码只作为测试值记录；旧 VM 文档和测试技能改为引用该清单，避免说明文中散落明文。
- 已生成本机解压试用包 `build/v0.45-local-trial-20260603.zip`，内含 `DataSync.exe`、`SyncAgent.exe`、默认配置、规则和试用文档；包内 `SyncAgent.exe` 和 `DataSync.exe` smoke 通过，不执行系统安装。
- Settings 页已恢复为独立分区布局：外观与语言、窗口与启动、集成、受管组件、安全与关于不再混在同一个自适应网格里。
- 已完成前端 16:9 多分辨率视觉审阅，截图证据保存在 `.cache/frontend-visual-review-16x9/`，报告见 `docs/frontend-visual-review-16x9.md`。
- 已将用户提供的 NodeBridge 图处理为透明背景图标，接入 Wails app icon、前端 favicon 和 Windows 托盘 HICON。
- 已按 Wails 默认 `1100x720` 尺寸完成全页面视觉复查，确认设计语言可保留但默认尺寸布局需优先优化 Rules、空状态和 Settings 错误区。
- 已按 `1100x720` 复查结果迭代前端：Rules 默认宽度取消横向溢出，Queues/Failures/Logs 空状态改为操作型面板，Settings 错误提示降权，Config 默认宽度分栏更均衡。
- 已补充 Settings 安全区密码说明：无初始密码、首次设置管理密码、退出密码可为空、忘记密码无法找回原文且需管理员重置配置；同时修复 Wails `build/windows/icon.ico` 仍为默认 W 图标导致窗口/任务栏图标不生效的问题。
- 已将 Overview 首屏的配置路径、规则路径、Agent 可执行文件和日志路径下沉到 Settings 诊断位置；说明书和空配置提示已区分“软件首次必设管理密码”和“启动同步所需同步配置”。
- Settings MCP Service 已按后端 V0.37 语义改为本次会话 stdio 临时开关；未加载配置文件时禁用开关并提示先保存同步配置。说明书新增忘记密码恢复章节，写明 `%ProgramData%\NodeBridge\config.yaml` 与 `security.admin_password` / `security.exit_password` 重置位置。
- 用户可见产品名已统一为英文 `NodeBridge`：说明书、设置文案、HTML title、Wails 应用名和输出文件名不再使用 `DataSync` 或中文名。
- 忘记密码恢复已按产品决策闭合为用户手动备份并清空配置文件 security 加密字段；后端不提供自动找回或重置 CLI。
- 首次只设置管理密码不落盘的问题已修复：`SaveConfig` 允许仅含 `security` 的首次安全草稿写入 `%ProgramData%\NodeBridge\config.yaml`，完整同步配置校验不放松。
- MCP 实际 `%ProgramData%` 安全草稿配置阻塞已在后端收口：`GetMCPServerStatus` / `SetMCPServerEnabled` 在配置不完整、当前用户无法解密或 `mcp-stdio` 严格加载失败时返回 `unsupported` 并拒绝启用。
- V0.38 已修复 RabbitMQ 安装器闭环：AMQP vhost 使用 `%2Fnodebridge-*` 编码，安装步骤即时写 summary，installer 增加超时证据，干净安装会写 RabbitMQ ownership marker，客户已有 RabbitMQ 仍不删除。
- 前端已配合首次安全草稿约定：Settings 在未填写同步配置时只向 `SaveConfig` 发送最小 `security` DTO，避免默认端口和批量参数让后端误判为部分同步配置；稳定契约文档中的用户可见名称同步为 `NodeBridge`。
- 前端已移除顶部管理锁定横幅，将只读/解锁状态收敛到底部右侧指示器，避免占用主工作区上方位置。
- 前端已将解锁入口集中到右下角胶囊：锁定时点击胶囊弹出管理密码；Config、Rules、Settings 不再显示页面内解锁按钮。
- 右下角权限控件已从“可点击状态胶囊”改为分段控件：左侧只显示只读/编辑状态，右侧明确显示解锁/锁定动作。
- MCP Server 持久启用需求已完成前端配合：Settings 和说明书已改为“启用后写入本机配置、重启保持启用、只由用户手动关闭”，并在 unsupported 状态提示同步配置未完成或当前用户无法解密配置。
- AI 工程化初始化提示词已收敛为一段完整可复制提示词，覆盖根级协作文件、`.ai/` 母本、身份模型、深度审阅和编辑器适配。

### 已完成事项

- 已绑定 Git remote：`https://github.com/YufeiSun5/NodeBridge.git`。
- 已创建根级 `AGENTS.md`，作为 Codex/Copilot/Cursor 的项目路引入口。
- 已创建 `.ai/` 母本文档目录，包括 instructions、docs、agents、prompts。
- 已创建 Copilot 与 Cursor 的薄适配层。
- 已将 `AGENTS.md` 和技能使用说明调整为中英日三语。
- 已初始化一期 MVP 骨架：Go module、vfox 工具链锁定、双命令入口、核心模型、配置样例、迁移样例和基础测试。
- 已补强一期测试门禁，并新增 RabbitMQ 拓扑、publisher、consumer、队列状态和 Windows 无感安装计划模型。
- 已新增表名和列名重映射能力：规则模型、mapper、映射样例和迁移追踪字段。
- 已实现 V0.2 MySQL Apply：连接池、迁移执行、`MappedEvent` 事务 Apply、幂等 `sync_apply_log`、CLI 子命令和 sample events。
- 已持久化版本路线、Wails 不占端口和日志 Web 服务要求。
- 已实现 V0.3 RabbitMQ CLI：拓扑初始化、事件发布、单条消费并成功 Apply 后 ACK。
- 已实现 V0.4 Runtime 初版：Edge 上传转发、Server ingress Apply、ACK/NACK 和防回源下发。
- 已实现 V0.4 Worker 与诊断骨架：长期循环、状态快照、只读日志 Web 入口。
- 已实现 `sync-agent run`：按 Edge/Server 模式启动当前 worker 组并挂接日志 Web 生命周期。
- 已实现 Edge downlink apply：Edge 从 Server RabbitMQ 的 `<node_id>.downlink.q` 消费下发事件，映射后写入本地 MySQL，成功后 ACK。
- 已新增 `consume-downlink-once`，用于开发阶段单步验证 Server -> Edge 下发 Apply。
- 已新增 `WorkerGroup`，Edge 模式可同时运行上传转发和下发 Apply 两个 worker。
- 已按约定补充短而有力的中英日三语注释，用于 ACK 与下发队列关键边界。
- 已完成 worker 级日志 ring buffer：日志 Web 暴露只读 `/logs`，仅输出脱敏后的 worker 运行摘要。
- 已完成 Server dispatch 计数：`server-ingress` 状态记录 dispatch 累计数量。
- 已修复 vfox Go 1.25.5 SDK 缺失问题，重新安装并设为当前项目版本。
- 已推送 V0.4 到 GitHub：`origin/main`。
- 已实现 V0.5 CDC stub：`ChangeEvent` JSON、`normalizer`、`cdc.StubSource`、`CDCUploadRuntime`。
- 已新增 `publish-change-once`，模拟 CDC 事件可经回环抑制和标准化后发布到 Edge 本地上传队列。
- 已新增 `sample-events/device_config.change.json` 和 `docs/v0.5-cdc-stub.md`。
- 已实现 V0.6 Canal prep：`cdc.Offset`、`OffsetStore`、内存 offset store、Canal adapter 前置接口。
- 已实现 Canal row change 到 `cdc.ChangeEvent` 的转换，以及 `FetchOnce` 的 fetch、convert、save offset、ack 流程。
- 已新增 `sync-agent canal-check` 用于 Canal 配置 smoke 校验。
- 已评估当前缺口：真实 Canal client、ACK/失败事件持久化、Windows Service、Wails 管理端、安装器仍是后续主线。
- 已实现 V0.7 CDC recovery：MySQL offset store 写入 `sync_upload_offset`，CDC 恢复策略支持指数退避、最大延迟和最大次数。
- 已新增 `docs/v0.7-cdc-recovery.md`，记录 offset 恢复和 fatal error 策略。
- 已测试当前主干后继续迭代，默认门禁通过。
- 已实现 V0.8 Persistence：`syncstore.Store` 支持 ACK、dispatch、error 持久化。
- 已新增失败事件入口：`failed-events` 查询失败 ACK，`retry-event` 将失败事件标记为 `PENDING`。
- 已新增 `docs/v0.8-persistence.md`，记录当前持久化和重试入口范围。
- 已实现 V0.9 replay 基础：Server ingress 持久化 `SyncEvent` payload。
- 已新增 `ReplayRuntime` 和 `replay-pending-once`，支持单步重放 `PENDING` 下发事件。
- 已更新 Server migration：`sync_event_log.event_payload` 和 Server 侧 `sync_error_log`。
- 已新增 `docs/v0.9-replay.md`，记录事件重放入口和限制。
- 已实现 V0.10 auto replay：Server `sync-agent run` 同时启动 `server-ingress` 和 `server-replay`。
- 已让 `server-replay` 使用独立 RabbitMQ 连接和 publisher，避免与 ingress 共用 channel。
- 已新增 `docs/v0.10-auto-replay.md` 和 `docs/delivery-assessment.md`，固化交付判断。
- 已实现 V0.11 single-machine lab：新增 Edge A、Edge B、Server 三份本机配置。
- 已新增开发 Docker Compose：1 个 RabbitMQ、3 个 MySQL，RabbitMQ 用 vhost 隔离节点。
- 已新增 `scripts/lab-smoke.ps1` 和 `docs/single-machine-lab.md`，用于单机测试准备。
- 已实现 V0.12 E2E smoke：新增 `scripts/lab-e2e.ps1` 串联 Edge A -> Server -> Edge B。
- 已让 `consume-once` 支持 `-edges` 参数，可在单步 Server ingress 后执行防回源下发。
- 已补 Edge 侧 `device_settings` 迁移，支持当前映射规则在 Edge B 写入目标表。
- 已新增 `docs/v0.12-e2e-smoke.md`，记录单机 E2E 执行和验证范围。
- 已实现 V0.13 Canal client adapter：新增 `internal/cdc/canal/withlin_client.go`。
- 已引入 `github.com/withlin/canal-go`，并将第三方依赖隔离在 Canal adapter 层。
- 已支持 Canal protobuf message 到 `cdc.ChangeEvent` 的转换、batch ack 和 offset 保存路径。
- 已新增 `docs/v0.13-canal-client.md`，记录真实 Canal client 适配策略和限制。
- 已实现 V0.14 Canal runtime：Edge `sync-agent run` 在 `cdc.type=canal` 时启动 `edge-cdc-canal`。
- 已新增 `canal-publish-once`，用于真实 Canal -> 本地 RabbitMQ 的单步测试。
- 已调整 Canal batch 可靠性顺序：发布成功或安全抑制后才保存 offset 并 ACK Canal。
- 已让 worker 停止时调用支持 `Stop` 的 stepper，确保 Canal source 可关闭。
- 已新增 `docs/v0.14-canal-runtime.md`，记录运行命令和可靠性边界。
- 已修复单机 E2E 验收缺口：Server migration 增加 `sync_apply_log`，避免中心 Apply 因系统表缺失失败。
- 已新增 `sample-events/device_config.insert.change.json`，用 INSERT 样例覆盖空目标表首次同步路径。
- 已修复 Edge 下发目标库选择：Server 仍按规则写中心库，Edge Downlink 按本节点 MySQL 配置写本地库。
- 已增强 `scripts/lab-e2e.ps1`：显式加载 Docker CLI 路径、重置队列和表、逐步执行三节点链路、失败即停。
- 已在本机 Docker 环境跑通 Edge A -> Server -> Edge B E2E，中心库和 Edge B 均验证 `device_settings.setting_value=ON`。
- 已修正 lab 拓扑：从单 RabbitMQ 多 vhost 改为 Edge A、Edge B、Server 三个 RabbitMQ 容器。
- 已调整 lab 端口：Edge A RabbitMQ `5673/15673`，Edge B `5674/15674`，Server `5675/15675`。
- 已验证 Server RabbitMQ 停止时 Edge A 本地 `edge.upload.cdc.q` 保留消息，恢复后可继续转发到 Server。
- 已更新单机测试文档，明确 Docker 仅作开发测试，最终交付仍允许客户自有 RabbitMQ 或默认安装 RabbitMQ。
- 已实现 V0.17 节点管理后端：`sync_node_registry` 仓储、节点注册 HTTP API、节点列表 API 和节点配置 API。
- 已新增 `sync_node_config`，保存中心管理的 Edge 非敏感读取配置，不保存密码和 token。
- 已新增 `CONFIG_UPDATE` 下发事件，Server 通过 RabbitMQ 下发配置，Edge 消费后写入本地配置快照。
- 已让 Server ingress 支持从 `sync_node_registry` 动态加载 ACTIVE Edge 节点，不再依赖 `-edges`。
- 已新增 `register-node`、`set-node-config`、`list-nodes`、`list-node-config` 和 `serve-node-api` CLI。
- 已新增 `scripts/lab-config-e2e.ps1`，验证 HTTP 注册、RabbitMQ 配置下发和 Edge 侧 ACK/落库。
- 已实现 V0.18 CRUD 语义验收：`scripts/lab-crud-e2e.ps1` 覆盖 INSERT、UPDATE、DELETE 软删和重复 `event_id` 幂等。
- 已新增固定 `SyncEvent` 样例：`device_config.insert/update/delete.sync.json`，用于稳定复现 CRUD 链路。
- 已新增 `ChangeEvent` UPDATE/DELETE 样例，为后续 CDC stub 和 Canal 验证保留输入。
- 已修正 `alarm_history` 规则目标库为 `scada_center`，验证 `EDGE_TO_SERVER` 只写中心不分发。
- 已修复 Node API 测试脚本进程清理，避免 `go run` 子进程残留占用 18090。
- 已让 Node API 使用独立 RabbitMQ topology channel 和 publisher channel，避免配置下发时通道状态互相影响。
- 已新增 `docs/v0.18-crud-e2e.md`，记录 CRUD E2E 范围、命令和 V0.19 批量方向。
- 已实现 V0.19 批量同步：默认 50 条或 500ms flush，Edge upload、Server ingress、Edge downlink 均支持 batch once。
- 已新增批量 CLI：`forward-upload-batch-once`、`consume-batch-once`、`consume-downlink-batch-once`。
- 已修复 RabbitMQ publisher confirm：确认通道改为初始化时注册并复用，避免批量发布时丢 confirm。
- 已让同步事件解析兼容 Windows UTF-8 BOM，避免 PowerShell 生成 JSON 后解析失败。
- 已新增 `scripts/lab-batch-e2e.ps1`，验证 50 条事件批量转发、批量应用、apply log 计数和顺序。
- 已持久化暗色工业终端 UI 规范：`.ai/docs/ui-design-spec.md`，并接入 `AGENTS.md` 和前端 instruction。
- 已创建 Wails React TypeScript 前端骨架：`frontend/`、设计 token、基础页面、Wails IPC service wrapper。
- 已将 `cmd/datasync-ui` 接入 Wails2 app 入口，绑定现有 `App` 后端方法。
- 已新增 `.ai/prompts/frontend-implementation.prompt.md`，便于把前端工作交给另一个 AI。
- 已实现 V0.20 双线协作文档：`frontend-backend-contract.md`、`frontend-requirements.md`、`ai-collaboration-log.md`。
- 已新增前端/后端轨道 Prompt：`frontend-track.prompt.md`、`backend-track.prompt.md`。
- 已补强双线协作纪律：开工前查 open 项、解决后追加回复、阻塞时追加 blocker、交付时汇报协作项。
- 已进一步约束后端：每次后端对话也必须读取协作日志，并主动记录需要前端处理的问题、DTO 变化和阻塞。
- 已将前后端交流收敛为 Active Board：不再新增前端/后端分散看板，稳定接口只维护 contract。
- 已将活跃 AI 看板迁移到根级 `AI_BOARD.md`，旧 `.ai/docs/ai-collaboration-log.md` 仅保留迁移提示，避免 docs 目录与根级状态发生冲突。
- 已新增 `internal/uiapi`，定义 Wails UI DTO、脱敏规则、空状态和操作结果结构。
- 已扩展 `cmd/datasync-ui.App` 的稳定 Wails 方法，覆盖 Overview、Config、Rules、Queues、Failures、Logs 和 Agent control。
- 已为 Wails UI 后端补充测试，覆盖配置脱敏、配置校验、规则读写和空状态。
- 已实现前端初版页面交互：Overview、Config、Rules、Queues、Failures、Logs 均接入 Wails service wrapper，覆盖 loading、empty、error 和 unsupported 状态。
- 已修复 vfox Node/npm 环境：重新安装并启用 nodejs@24.15.0，前端 `npm install` 和 `npm run build` 通过。
- 已按 vfox 恢复 Go 1.25.5 环境，并完成 `go test ./...`、`go vet ./...` 验证。
- 已实现 V0.21 Wails 后端真实接口基础：配置落盘、DPAPI 密钥保护、规则落盘、队列状态、失败事件、日志读取和诊断包导出。
- 已实现托盘常驻后端支撑：退出密码校验、当前用户自启动查询和设置；托盘 UI 仍由前端负责。
- 已新增 `scripts/lab-stress-e2e.ps1`，用于三 RabbitMQ + 三 MySQL 环境下 1,000 条默认压力测试。
- 已明确 DataSync 前端三语要求：默认中文，支持中文、英文、日文切换，协议值和技术标识保持英文。
- 已实现前端三语界面初版：新增 i18n provider、语言切换控件，并覆盖 Overview、Config、Rules、Queues、Failures、Logs 页面主要文案。
- 已完成 V0.21 前端接口接入：托盘控制面、退出密码弹窗、自启动开关、诊断包导出和 `security.exit_password` 配置字段均已接入三语界面。
- 已完成 V0.22 Wails 关闭行为：原生窗口关闭按钮隐藏到 Windows 系统托盘，显式退出通过 `RequestExit` 校验密码后只放行下一次 `runtime.Quit`。
- 已修复 Wails 标准打包路径：根目录新增 Wails CLI 入口，后端实现抽到 `internal/datasyncui`，`cmd/datasync-ui` 保留薄入口；`wails build -clean` 已产出 `build/bin/DataSync.exe`。
- 已补齐已有后端功能测试：密钥加密/合并、规则校验、诊断包、Wails 队列错误状态和退出密码空配置。
- 已新增 `docs/backend-completion-plan.md`，整理后端已完成能力、测试覆盖和未完成计划。
- 已实现管理解锁后端：`security.admin_password` 用于 `UnlockAdmin`，`security.exit_password` 用于托盘退出，敏感方法未解锁时拒绝执行，保存配置时管理密码不能为空。
- 已更新前端要求：进入管理操作前必须解锁，锁定状态只允许只读概览和状态查看，前端需区分管理密码和退出密码。
- 已修复 Wails 前端数据为空的直接原因：服务层改为优先调用 `window.go.datasyncui.App`，并保留旧 `main.App` fallback。
- 已修复 Wails 打包资源风险：根目录入口改为 embed `frontend/dist`，避免 exe 启动时找不到前端资源。
- 已删除临时前端审阅文档和过期 V0.20 交互说明，前后端交流统一回到 Active Board。
- 已补齐 example/lab 配置中的 `security.admin_password` 与 `security.exit_password`。
- 已实现 Wails 后端运行控制初版：`StartAgent`、`StopAgent`、`RestartAgent` 控制外部 `SyncAgent.exe` 进程，并支持重复启动保护。
- 已构建 `build/bin/SyncAgent.exe`，并完成 Edge/Server 示例配置 CLI smoke；`DataSync.exe` 原生启动 smoke 通过。
- 已按后端审阅意见完成前端 Admin Lock 接入：全局锁状态、解锁弹窗、敏感操作前置解锁、Rules 新增删除、Failures Retry 解锁和 Config 管理密码必填校验。
- 已实现前端首次配置引导：空配置时 Overview、Queues、Failures、Logs 显示明确未配置提示，Start/Restart 先提示保存配置，Logs 增加诊断包导出入口。
- 已修复 Wails Windows 标题栏关闭行为：点击 X 隐藏到系统托盘而不是退出；托盘双击或右键菜单可恢复窗口。
- 已调整 Wails Windows 关闭路径：关闭按钮统一走 `OnBeforeClose` 拦截并隐藏窗口，只有退出密码通过后的 `RequestExit` 才允许真正退出，规避 Win11 下内置 `HideWindowOnClose` 行为不稳定。
- 已优化 Windows 托盘交互：托盘图标左键单击或双击恢复窗口，右键弹出显示/退出菜单。
- 已增强 Windows 托盘图标注册：通知区图标改为稳定 GUID 注册并启用 tooltip 显示，Explorer/任务栏重启后自动重新添加图标，同时记录 native tray ready 日志。
- 已将 Windows 托盘图标改为代码生成的高对比度 HICON，并记录 `Shell_NotifyIcon` add/setversion/delete 结果，排查 Win11 通知区图标不可见问题。
- 已修复托盘右键菜单兼容性：补充 NotifyIcon v4 / Win11 常用的 `WM_CONTEXTMENU` 处理，并优化前端顶部菜单和用户文案，移除不适合用户界面的 Wails/HTTP/后端调试提示。
- 已按用户软件交互收敛顶部栏：窗口 X 默认隐藏到托盘，不再显示“隐藏到托盘”按钮；退出和语言选择移入软件设置，语言默认跟随操作系统并记住用户选择。
- 已移除应用内重复品牌顶栏，原生窗口标题、总览产品名、托盘 tooltip 和托盘菜单统一为 `NodeBridge`；托盘事件改为按 NotifyIcon v4 低位事件码解析，修复右键菜单漏触发。
- 已拆分 Settings 与 Sync Config：语言、窗口退出、安全密码和登录自启动归入软件设置；同步配置页只保留 Node/MySQL/RabbitMQ/CDC/Sync/Log Web 参数。
- 已按后端协作意见补齐 Rules 分发策略 UI：`dispatch_target` 与 `dispatch_node_ids` 支持只读展示、编辑和三语说明。
- 已实现 Windows 原生托盘 helper：托盘菜单提供显示窗口和退出入口，退出入口会触发前端退出密码弹窗，不绕过 `RequestExit`。
- 已补强前端三语界面：托盘文案、状态文案、队列角色和 Rules `source_node_ids` 编辑均接入中英日。
- 已优化 Rules 页使用引导：方向枚举保持英文协议值，右上角新增三语规则填写说明。
- 已新增软件内说明书功能：独立 Manual 页面按首次配置、总览、配置、规则、队列、失败事件、日志诊断、托盘退出和常见状态分章展示三语说明。
- 已优化前端锁定态：Config 和 Rules 未解锁时仅只读展示，不显示输入框或编辑/删除按钮；首次配置也需先解锁才进入编辑态，密码、token 等重要数据使用磨砂遮罩。
- 已调整 Wails dev 前端命令：当前 vfox Node 缺少 `npm` shim，`wails.json` 改为直接调用本地 `tsc.cmd` 和 `vite.cmd`。
- 已新增 `SyncRule.source_node_ids`，支持多个 Edge 源表同名但中心目标表不同的节点作用域规则。
- 已让 Server ingress、Edge downlink、batch runtime 和 `apply-event` 使用事件来源节点匹配规则。
- 已新增 `configs/sync-rules.10-edge.example.yaml`，覆盖 2 张多主同构表和 10 个 `data_all` 汇总表映射。
- 已新增 `docs/v0.23-11-node-lab-plan.md`，明确 11 套 MySQL/RabbitMQ 实验范围和当前未测链路。
- 已完成 V0.24 前置可用性测试盘点：现有三节点 lab、配置下发、断网缓存和 1,000 条压力测试通过。
- 已新增 `docs/v0.24-backend-field-test-plan.md`，记录现场 11 节点缺口和后端修改计划。
- 已实现 V0.24 现场拓扑验证：统一 lab 环境探测、补齐 `point_config` / `data_all` 迁移、生成 11 节点 Docker lab。
- 已新增 11 节点 E2E：`data_all` 汇总到 `data_all_edge_001..010`、`device_config` / `point_config` 多主 fanout、Server-origin 模拟下发到 10 个 Edge。
- 已新增 `dispatch-event-once`，用于 Server-origin 模拟事件直接走 Server dispatch，不经过 Server ingress apply。
- 已通过 V0.24 门禁：`go test -count=1 ./...`、`go vet ./...`、Wails contract check、TypeScript + Vite build、3 节点全量回归和 11 节点新增 E2E。
- 已新增可配置分发策略：`dispatch_target`、`dispatch_node_ids`，并记录到 `docs/sync-routing-policy.md`。
- 已实现 V0.25 现场 MVP 同步策略：10 个 Edge `data_all` 汇总到中心 10 张目标表，两张多主表支持 Edge-origin fanout、Server-origin 下发和 SELECTED_EDGES 指定分发。
- 已新增 Server-side CDC dispatch runtime、`server-cdc-dispatch-once` 和 Canal server dispatch worker 接入口，中心库变更可直接走下发链路。
- 已写入 Frontend V0.26 Plan，要求前端使用 Overview 新字段、RFC3339 时间字段，并完成 exe 级验收。
- 已实现 V0.26 后端 UI 状态补强：`GetOverview` 返回 `config_loaded/config_path/rules_path/node_id/node_name/cdc_message`，CDC 状态支持 `configured`。
- 已修复 Wails `time.Time` 绑定警告方向：UI DTO 中失败事件、日志和 Auth 过期时间改为 RFC3339 string。
- 已修复打包 exe 规则显示不稳定：`GetSyncRules` fallback 使用现场默认规则并自动落盘 `sync-rules.yaml`。
- 已让外部 `SyncAgent.exe` stdout/stderr 写入 `logs/sync-agent.log`，`GetLogs` 合并读取 UI ring buffer 和 agent 日志。
- 已新增 `docs/v0.26-backend-plan.md`，固化 V0.26 后端计划、前端输入、测试矩阵和退出条件。
- 已批复前端 V0.26 需求，并新增 MCP Server 默认关闭的 Wails 预留接口与配置开关。
- 已实现前端 V0.26 设置与状态接入：本地主题偏好、Overview 显式配置状态、CDC 详情、配置/规则路径和 Settings MCP Server 预留开关。
- 已优化 Rules 页面：移除重复说明区，枚举字段保留下拉控件，启用字段改为滑动开关，复杂文本输入增加紧凑字段标签。
- 已实现 V0.29 固定目录 package smoke：构建前端、`DataSync.exe`、`SyncAgent.exe`，复制配置/规则并验证 DataSync 进程启动。
- 已新增试用运行手册，明确当前是工程试点 runbook，不是最终离线安装器。
- 已实现 V0.30 失败重试闭环：批量重试 Wails/CLI、死信只读预览、pending replay 和三节点 lab retry E2E。
- 已实现 V0.31 后端 alpha：`managed-plan/apply/repair/uninstall`、Wails 安装计划接口、诊断包安装摘要、只读 `mcp-stdio`。
- 已接入 Settings 受管安装计划 UI：读取 `GetManagedInstallPlan`，执行 `ApplyManagedInstall` 前要求管理解锁，并展示 alpha 资源边界。
- 已完成前端可用性优化：同步配置分组编辑、布尔项滑动开关、关键数字字段单位提示、失败批量重试确认、Agent 停止/重启确认和受管组件表格横向滚动。
- 已完成 Rules ACTIVE Edge 候选接入：读取 `GetNodeOptions()`，展示候选勾选、状态消息和 empty/error 提示，同时保留 `dispatch_node_ids` 手填兜底。
- 已实现 V0.32 11 节点长测入口：`lab-11-soak-e2e.ps1` 和 `lab-11-disconnect-e2e.ps1`，并将汇总文件纳入诊断包候选。
- 已实现 V0.33 安装器安全预检：离线包 catalog 模型、SHA256 校验、命令计划 CLI 和 fake asset 单元测试。

### AI 工程化状态清单

- [x] 根级项目路引：`AGENTS.md`
- [x] AI 工作流规范：`.ai/instructions/ai-workflow.md`
- [x] Go SyncAgent 规范：`.ai/instructions/go-syncagent.md`
- [x] Wails 前端规范：`.ai/instructions/frontend-wails.md`
- [x] 同步架构约束：`.ai/instructions/sync-architecture.md`
- [x] 设计摘要文档：`.ai/docs/product-design.md`
- [x] 只读架构审查 Agent：`.ai/agents/architecture-review.agent.md`
- [x] 常用 Prompt 模板：`.ai/prompts/`
- [x] Copilot 适配入口：`.github/`
- [x] Cursor 适配入口：`.cursor/`
- [x] 一期 MVP 骨架：`cmd/`、`internal/`、`configs/`、`migrations/`
- [x] 一期补强测试：CLI、配置、规则、状态、回环抑制
- [x] 二期 RabbitMQ 基础：`internal/rabbitmq`
- [x] 表列映射基础：`internal/mapper`
- [x] V0.2 MySQL Apply：`internal/mysqlconn`、`internal/apply`
- [x] V0.3 RabbitMQ CLI：`init-rabbitmq`、`publish-event`、`consume-once`
- [x] V0.4 Runtime 初版：`internal/syncruntime`、`forward-upload-once`
- [x] V0.4 Worker/诊断：worker loop、`internal/status` runtime store、`internal/logweb`
- [x] V0.4 启动入口：`sync-agent run`
- [x] V0.4 Edge 下发：`EdgeDownlinkRuntime`、`consume-downlink-once`、Edge 双 worker
- [x] V0.4 日志与调度状态：`/logs`、ring buffer、dispatch count
- [x] V0.5 CDC stub：`internal/normalizer`、`cdc.StubSource`、`CDCUploadRuntime`
- [x] V0.5 CLI smoke：`publish-change-once`
- [x] V0.6 Canal prep：`cdc.Offset`、`internal/cdc/canal`、`canal-check`
- [x] V0.7 CDC recovery：MySQL offset store、recovery policy、fatal recovery marker
- [x] V0.8 Persistence：`internal/syncstore`、`failed-events`、`retry-event`
- [x] V0.9 Replay：`event_payload`、`ReplayRuntime`、`replay-pending-once`
- [x] V0.10 Auto replay worker：Server run 挂接 `server-replay`
- [x] V0.11 Single machine lab：lab configs、dev compose、lab smoke script
- [x] V0.12 E2E smoke：`scripts/lab-e2e.ps1`、Server dispatch CLI、Edge B verify path
- [x] V0.13 Canal client adapter：`withlin/canal-go` wrapper、protobuf conversion、ACK path
- [x] V0.14 Canal runtime：`edge-cdc-canal` worker、`canal-publish-once`、publish-before-ack
- [x] V0.15 Single-pc E2E verified：Docker MySQL x3 + RabbitMQ，Edge A -> Server -> Edge B 验证通过
- [x] V0.16 Separated RabbitMQ lab：Edge/Server 独立 broker，断开 Server broker 时 Edge 本地队列保留
- [x] V0.17 Node management：HTTP 注册、非敏感配置管理、CONFIG_UPDATE 下发、动态 ACTIVE 节点分发
- [x] V0.18 CRUD E2E：增删改、软删、幂等、单向表不分发、表列映射验收
- [x] V0.19 Batch sync：50 条或 500ms flush、batch CLI、batch E2E
- [x] V0.19 Wails UI skeleton：React TypeScript、暗色工业终端风格、Wails IPC 服务层
- [x] V0.20 Frontend/backend contract：Wails API、DTO、前端需求和协作记录
- [x] V0.21 Wails backend：配置持久化、密钥保护、托盘支撑接口、诊断包、压力测试脚本
- [x] V0.22 Tray exit loop：窗口关闭隐藏到托盘、退出密码单次放行、前端退出流接入
- [x] Wails native build：根目录 `wails build`、原生 exe 启动 smoke
- [x] V0.22 Admin lock：管理解锁、手动锁定、敏感方法鉴权
- [x] V0.22 Frontend Admin lock：全局锁状态、解锁弹窗、敏感操作拦截、规则新增删除
- [x] V0.22 Frontend first-run guidance：配置缺失识别、unknown 文案分流、Logs 诊断导出
- [x] V0.22 Wails tray close fix：Windows 标题栏 X 隐藏到系统托盘
- [x] Wails dev command：绕过缺失 npm shim，直接调用本地 Vite/TypeScript
- [x] RabbitMQ 无感安装计划：`internal/installer/rabbitmq`
- [x] V0.24 11-node lab：1 Server + 10 Edge，11 MySQL / 11 RabbitMQ，汇总表、多主 fanout、Server-origin 模拟下发
- [x] V0.28 Canal lab：Edge/Server 真实 Canal E2E 和 20 条 soak
- [x] V0.29 Package smoke：固定目录 `DataSync.exe` + `SyncAgent.exe` 启动验证
- [x] V0.30 Retry closure：失败 ACK 批量重试、pending replay、死信预览
- [x] V0.31 Installer/MCP alpha：受管组件执行器 alpha、只读 stdio MCP
- [x] V0.32 11-node soak：循环 stress、Server broker 恢复、Edge local broker 重启恢复
- [x] V0.33 Installer preflight：离线包 catalog、SHA256 校验、命令计划，不触碰本机服务
- [x] V0.34 Headless installer test bundle：WinServer2022 Core 测试包、默认预检脚本、真实安装显式开关
- [x] V0.35 Installer closure beta：headless 包支持安装、验证、卸载三入口，等待真实离线包做 VM 侧执行
- [x] V0.36 Installer real-closure prep：真实 catalog 生成、幂等安装/卸载支撑、默认预检通过

### 后续建议

- 使用正确 `NODEBRIDGE_RABBITMQ_URL` 和 `NODEBRIDGE_SERVER_MYSQL_DSN` 跑 `docs/v0.3-smoke.md`。
- 下一步由 test-ai 使用真实离线包在隔离 VM 中执行 V0.36 `-ExecuteInstall`、`-VerifyOnly`、二次 `-ExecuteInstall`、二次 `-Uninstall`。
- 前端待办：使用 `GetOverview` 新字段替代配置状态猜测，并用打包 exe 验收 Overview、Rules、Logs。
- 后端未完成清单见 `docs/backend-completion-plan.md`。
- 对接真实 MySQL 容器：设置 `NODEBRIDGE_APPLY_MYSQL_DSN` 后运行集成测试。
- 进入 CDC 阶段：Canal Go client 选型、offset 保存、异常恢复。

### 待确认

- 项目最终用户可见名称统一为 `NodeBridge`。
- Canal Go client 当前使用 `github.com/withlin/canal-go`，后续可替换，依赖已隔离。
- V0.21 不做 Windows Service；后续如需无人登录运行，再单独评估服务化版本。
- vfox Node/npm 已可用；Go/Node shell 仍建议显式注入 vfox cache PATH 后执行自动化命令。
- 当前 vfox Go 1.25.5 SDK/cache 缺少标准库 `src/` 和 `vet.exe`；已确认可用项目内 `.tools/go1.25.5/go` 作为 GOROOT 跑通 `go test ./...` 和 `go vet ./...`。
- 前端当前通过空 `mode`、空 `node.id`、空 `mysql.database` 推断首次配置的问题已由后端 `GetOverview.config_loaded/config_path` 解决，前端仍需接入。
- Wails 绑定生成 `Not found: time.Time` 警告已通过 UI DTO 时间字段 RFC3339 string 化解决。
- 交付节奏：当前已具备后端技术试点基础；补齐 V0.20 前端构建/绑定、Windows Service 和最小管理端后可做客户试用，V1.0 才是产品交付。

### 改动记录

- 2026-09-07 17:59 | GPT-5 / backend-ai | FB-037：新增一主多边缘部署与 Windows/Mac SSH/MCP 授权手册，发布 v0.46.3 GitHub prerelease；标签、安装包大小和 SHA256 digest 已复核。

- 2026-09-07 15:30 | GPT-5 / backend-ai | FB-036：修复安装后 config.yaml 原子替换 Access denied，远程完成目标 ACL/LocalSubnet 防火墙和 Windows Codex MCP 注册，继续发布 v0.46.2。

- 2026-09-07 14:58 | GPT-6 / backend-ai | FB-035：修复 NSIS x86/x64 组件检测、安装退出码/参数转义与持久失败日志，生成 v0.46.1 安装修复版；32/64 位回归和 Go/MCP/资产校验通过，异机重装待 FB-034。

- 2026-09-07 14:26 | GPT-6 | review-ai 实现 MCP v0.46 全配置实验室模式、26 个真实管理工具和 SSH 客户端生成器，修复协议/空规则/日志 limit/跨会话覆盖/重复进程问题，增加原子文件替换和回归测试；全量测试、vet、linter、Wails 构建和包内 EXE smoke 通过，异机安装及 SSH 留待现场验证。

- 2026-05-21 08:43 | gpt-5 | 初始化 AI 协作文档体系和编辑器适配入口。
- 2026-05-21 09:05 | gpt-5 | 将 AGENTS.md 和技能说明调整为中英日三语。
- 2026-05-21 10:20 | gpt-5 | 初始化一期 MVP 骨架并通过 Go 测试和 CLI smoke test。
- 2026-05-21 10:44 | gpt-5 | 补强一期测试并实现 RabbitMQ 基础和安装计划模型。
- 2026-05-21 10:55 | gpt-5 | 增加表列重映射契约、mapper 测试和配置样例。
- 2026-05-21 11:24 | gpt-5 | 持久化路线图并实现 V0.2 MySQL Apply 和模拟事件 CLI。
- 2026-05-21 11:34 | gpt-5 | 实现 V0.3 RabbitMQ CLI 与 smoke 文档。
- 2026-05-21 11:40 | gpt-5 | 实现 V0.4 Runtime 初版和 Edge 上传转发入口。
- 2026-05-21 11:44 | gpt-5 | 增加长期 worker 循环、状态快照和日志 Web 入口。
- 2026-05-21 11:48 | gpt-5 | 增加 sync-agent run 并接入 Edge/Server worker 组。
- 2026-05-21 11:58 | gpt-5 | 增加 Edge 下发 Apply、双 worker 编排和三语短注释。
- 2026-05-21 12:10 | gpt-5 | 完成 V0.4 日志 ring buffer、dispatch 计数和测试验收。
- 2026-05-21 13:12 | gpt-5 | 推送 V0.4 并完成 V0.5 CDC stub、normalizer 和测试。
- 2026-05-21 13:19 | gpt-5 | 实施 V0.6 Canal prep、offset 模型和 canal-check。
- 2026-05-21 14:05 | gpt-5 | 评估缺口并完成 V0.7 MySQL offset store 与恢复策略。
- 2026-05-21 14:38 | gpt-5 | 测试后完成 V0.8 持久化仓储和失败事件入口。
- 2026-05-21 15:08 | gpt-5 | 完成 V0.9 事件载荷持久化和单步重放入口。
- 2026-05-21 15:27 | gpt-5 | 完成 V0.10 后台重放 worker 并固化交付评估。
- 2026-05-21 15:49 | gpt-5 | 完成 V0.11 单机 lab 配置、Compose 和准备脚本。
- 2026-05-21 16:14 | gpt-5 | 完成 V0.12 单机 E2E smoke 脚本和 Server 下发入口。
- 2026-05-21 16:43 | gpt-5 | 完成 V0.13 Canal client 适配层和 protobuf 转换。
- 2026-05-21 17:11 | gpt-5 | 完成 V0.14 Canal runtime 接入和单步发布入口。
- 2026-05-21 16:25 | gpt-5 | 完成 V0.15 单机三节点 E2E 修复、脚本验收和测试。
- 2026-05-21 16:41 | gpt-5 | 改为三套 RabbitMQ lab 并验证 Server 断开时 Edge 本地缓存。
- 2026-05-21 17:10 | gpt-5 | 完成 V0.17 节点注册、动态分发和配置下发后端验收。
- 2026-05-21 17:29 | gpt-5 | 完成 V0.18 CRUD/单向表 E2E、配置脚本修复和验收文档。
- 2026-05-21 18:00 | gpt-5 | 完成 V0.19 批量同步、Wails React 骨架和 UI 规范持久化。
- 2026-05-21 19:41 | gpt-5 | 完成 V0.20 前后端契约、协作文档和 Wails UI API 骨架。
- 2026-05-21 19:45 | gpt-5 | 补强前后端 AI 协作日志检查、回复和交付汇报纪律。
- 2026-05-21 19:47 | gpt-5 | 强化后端每次对话读取协作日志并主动记录前端待处理事项。
- 2026-05-21 20:04 | gpt-5 | 实现前端初版 Wails 页面交互并恢复 vfox Node/Go 验证环境。
- 2026-05-21 20:19 | gpt-5 | 完成 V0.21 Wails 后端真实接口、托盘支撑和压力测试脚本。
- 2026-05-21 20:20 | gpt-5 | 更新 AGENTS 前端三语约束并实现 DataSync UI 中英日切换。
- 2026-05-21 20:34 | gpt-5 | 接入 V0.21 前端托盘控制、退出鉴权、自启动、诊断包和安全配置界面。
- 2026-05-21 20:46 | gpt-5 | 完成 V0.22 Wails 关闭隐藏到托盘和退出鉴权单次放行。
- 2026-05-21 20:50 | gpt-5 | 修复 Wails 标准打包入口并完成原生 exe 构建和启动 smoke。
- 2026-05-21 20:33 | gpt-5 | 补齐已有后端功能测试并整理后端未完成计划。
- 2026-05-21 20:46 | gpt-5 | 增加管理解锁契约并保护敏感 Wails 后端方法。
- 2026-05-21 21:02 | gpt-5 | 拆分管理密码和退出密码并更新前端协作契约。
- 2026-05-21 21:08 | gpt-5 | 要求保存配置时必须设置管理密码并补充验证测试。
- 2026-05-21 21:36 | gpt-5 | 修复 Wails 绑定入口和前端资源嵌入并补充契约检查。
- 2026-05-21 21:36 | gpt-5 | 输出前端审阅意见并记录到协作日志。
- 2026-05-21 21:36 | gpt-5 | 补齐示例配置安全字段，避免 UI 保存配置缺少管理密码。
- 2026-05-21 21:58 | gpt-5 | 完成 Wails 后端外部 SyncAgent 进程控制和测试。
- 2026-05-21 21:58 | gpt-5 | 构建 DataSync/SyncAgent 双 exe 并完成基础启动 smoke。
- 2026-05-21 22:10 | gpt-5 | 按后端审阅意见接入前端 Admin Lock、敏感操作解锁和规则新增删除。
- 2026-05-21 22:25 | gpt-5 | 实现前端首次配置引导、unknown 状态分流和后端待确认协作问题。
- 2026-05-21 22:35 | gpt-5 | 修复 Wails Windows 标题栏关闭直接退出，改为隐藏到托盘。
- 2026-05-21 22:55 | gpt-5 | 实现 Windows 原生托盘 helper，恢复“隐藏到托盘”行为，并补齐 Rules `source_node_ids` 与部分三语文案。
- 2026-05-21 23:22 | gpt-5 | 优化 Rules 页说明区，补充 direction、source_node_ids、include/exclude 和 column mapping 的三语填写提示。
- 2026-05-21 23:29 | gpt-5 | 新增软件内说明书页面，按功能章节提供中英日操作说明和协议字段速查。
- 2026-05-21 23:49 | gpt-5 | 调整 Config/Rules 锁定态为只读展示，隐藏编辑控件并对重要数据做磨砂遮罩。
- 2026-05-21 22:38 | gpt-5 | 修复当前 vfox Node 缺少 npm 时 Wails dev 无法启动的问题。
- 2026-05-21 22:09 | gpt-5 | 增加节点作用域规则并记录 11 节点实验室计划。
- 2026-05-21 22:39 | gpt-5 | 跑完整三节点可用性测试并制定 V0.24 后端现场测试计划。
- 2026-05-21 23:51 | gpt-5 | 完成 V0.24 11 节点现场拓扑脚本、迁移、E2E 与门禁测试。
- 2026-05-22 00:01 | gpt-5 | 收敛前后端交流为 contract 加 Active Board 两文件模型。
- 2026-05-22 00:05 | gpt-5 | 删除临时前端审阅文档并开始收敛同步分发配置模型。
- 2026-05-22 00:10 | gpt-5 | 增加可配置分发策略字段和同步路由策略文档。
- 2026-05-22 00:10 | gpt-5 | 调整 Wails X 关闭处理为统一拦截隐藏，修复 Win11 下关闭未进托盘的兼容性问题。
- 2026-05-22 00:16 | gpt-5 | 增加托盘图标左键单击恢复窗口，保留右键显示/退出菜单。
- 2026-05-22 00:24 | gpt-5 | 增强 Windows 托盘图标注册稳定性，补 GUID、tooltip 和任务栏重建后自动恢复。
- 2026-05-22 00:31 | gpt-5 | 将托盘图标替换为代码生成 HICON，并补充 Shell_NotifyIcon 结果日志。
- 2026-05-22 00:42 | gpt-5 | 修复托盘右键菜单事件并收敛前端菜单、状态栏和用户文案。
- 2026-05-22 00:52 | gpt-5 | 移除顶部隐藏/退出/语言控件，将语言和退出移入配置页应用设置，并默认使用系统语言。
- 2026-05-22 01:02 | gpt-5 | 移除应用内顶部品牌行，统一 NodeBridge 可见名称，并修复托盘右键菜单事件解析。
- 2026-05-22 01:20 | gpt-5 | 拆分 Settings 与 Sync Config 页面，更新前端契约测试以匹配新的软件设置边界。
- 2026-05-22 01:21 | gpt-5 | 完成 V0.25 可配置分发、Server-origin 下发和 11 节点现场 E2E。
- 2026-05-22 08:39 | gpt-5 | 审阅前端 unknown 数据问题并登记 V0.26 状态接口、规则路径和日志源缺口。
- 2026-05-22 08:52 | gpt-5 | 接入 Rules 分发策略控件并关闭前端协作项 FB-004。
- 2026-05-22 08:54 | gpt-5 | 写入前端 V0.26 计划并实现后端 UI 状态、规则落盘和 agent 日志补强。
- 2026-05-22 09:00 | gpt-5 | 固化 V0.26 后端规划，覆盖 SyncAgent 控制、Canal soak、性能和 exe 验收。
- 2026-05-22 09:17 | gpt-5 | 批复前端需求并新增 MCP Server 默认关闭的预留接口。
- 2026-05-22 09:30 | gpt-5 | 实现前端主题切换、Overview V0.26 状态接入和 Settings MCP Server 开关。
- 2026-05-22 09:42 | gpt-5 | 优化 Rules 页面输入形态，移除重复说明并将启用改为滑动开关。
- 2026-05-22 10:04 | gpt-5 | 将 Rules 编辑态改为分组卡片布局，并登记 ACTIVE Edge 节点候选项接口需求。
- 2026-05-22 10:13 | gpt-5 | 明确 Rules 空字段默认语义，避免源节点、目标节点和列映射空白被误认为未配置。
- 2026-05-22 10:22 | gpt-5 | 增强 Rules 编辑态空值说明可读性，补齐缺失的默认语义提示。
- 2026-05-22 10:14 | gpt-5 | 完成 V0.27 后端进程控制、可信压力入口和 Canal E2E 验证脚本。
- 2026-05-22 10:20 | gpt-5 | 跑通 V0.27 Go、vet、Wails contract、前端 build 和 SyncAgent 构建门禁。
- 2026-05-22 10:39 | gpt-5 | 新增 MIT License 并同步前端包许可元数据。
- 2026-05-22 10:45 | gpt-5 | 在前端底部状态栏新增 MIT License 低调展示文案，并在 Settings 补充许可范围说明。
- 2026-05-22 10:47 | gpt-5 | 实现受管组件 manifest、RabbitMQ/Canal 安装计划和资源归属边界。
- 2026-05-22 10:51 | gpt-5 | 接入 Config 页 CDC 安装边界字段并跑通 Go/vet/contract/前端构建。
- 2026-05-22 11:03 | gpt-5 | 写入优先级计划并实现 V0.28 Canal lab/soak 脚本，记录镜像拉取阻塞。
- 2026-05-22 11:18 | gpt-5 | 只改说明书页，将协议字段从常驻参考区移入独立章节并补充规则字段细节。
- 2026-05-22 11:21 | gpt-5 | 修复 Canal E2E/soak 脚本并跑通 Edge/Server 真实 CDC 验证。
- 2026-05-22 11:58 | gpt-5 | 推进 V0.29 固定目录 package smoke、试用 runbook 和版本状态更新。
- 2026-05-22 12:14 | gpt-5 | 完成 V0.30 失败重试、批量重试、死信预览和 lab 验证。
- 2026-05-22 13:10 | gpt-5 | 完成 V0.31 受管安装执行器 alpha 和只读 MCP stdio alpha。
- 2026-05-22 13:28 | gpt-5 | Settings 接入受管组件安装计划和执行入口，关闭 FB-015。
- 2026-05-22 13:46 | gpt-5 | 完成前端页面规整和危险操作确认，保持 FB-011 由后端继续处理。
- 2026-05-22 13:31 | gpt-5 | 完成 V0.32 11 节点 soak 和断网恢复验证入口。
- 2026-05-22 13:45 | gpt-5 | 完成 V0.33 安装器离线包预检和命令计划。
- 2026-05-22 12:28 | gpt-5 | 前端接入 SyncAgent 真实进程状态、Failures 批量重试和死信只读预览。
- 2026-05-22 14:32 | gpt-5 | 准备 V0.34 Hyper-V 私有隔离 VM 环境，VM 保持关机且未触碰宿主机 Erlang/RabbitMQ/Canal。
- 2026-05-22 15:14 | gpt-5 | 新增 V0.34 安装器 VM lab 记录，写入隔离 VM 管理员测试密码、Gen1 VM 和快照计划。
- 2026-05-22 16:28 | gpt-5 | 验证可通过 PowerShell Direct 操作 V0.34 VM，并创建 `Clean-Windows-Installed` 快照。
- 2026-05-22 16:34 | gpt-5 | 强化 AI 身份约束，新增 test-ai 和 review-ai 并要求操作前声明身份、统一走 Active Board。
- 2026-05-22 16:43 | gpt-5 | 生成 WinServer2022 Core 用 headless installer test bundle 并登记测试 AI 任务。
- 2026-05-22 16:50 | gpt-5 | 以 test-ai 身份完成 V0.34 headless installer 默认预检，真实安装因缺离线包暂阻塞。
- 2026-05-22 16:54 | gpt-5 | 增强 V0.35 headless installer test bundle，加入安装、验证和卸载闭环入口。
- 2026-05-22 16:56 | gpt-5 | 将活跃 AI 协作看板迁移到根级 AI_BOARD.md，并更新 AGENTS、workflow 和身份 prompt 路引。
- 2026-05-22 17:06 | gpt-5 | 以 test-ai 身份完成 V0.35 headless installer 版本查看与 `-VerifyOnly` 未安装失败验证。
- 2026-05-22 17:17 | gpt-5 | 新增 `GetNodeOptions` Wails 契约并关闭 FB-011 后端阻塞。
- 2026-05-22 17:29 | gpt-5 | 完成 V0.36 headless installer 幂等闭环准备并生成测试包。
- 2026-05-22 17:26 | gpt-5 | 以 frontend-ai 身份接入 Rules ACTIVE Edge 候选多选并关闭 FB-017。
- 2026-05-22 17:45 | gpt-5 | 以 test-ai 身份完成 V0.36 headless installer VM 侧默认预检、catalog 脚本和未安装清理验证。
- 2026-05-22 17:55 | gpt-5 | 以 frontend-ai 身份新增 Wails 前端启动与测试文档。
- 2026-05-23 09:10 | gpt-5 | 以 backend-ai 身份修复 V0.37 RabbitMQ 复用、cookie 同步和真实安装 summary。
- 2026-05-22 17:57 | gpt-5 | 以 backend-ai 身份下载官方离线包并生成 V0.36 真实资产测试包。
- 2026-05-22 18:00 | gpt-5 | 补强 Wails 前端启动文档，加入完整可复制命令、逐行说明和快速工具检查。
- 2026-05-22 18:17 | gpt-5 | 以 frontend-ai 身份恢复 Settings 页分区布局并完成前端门禁验证。
- 2026-05-22 18:31 | gpt-5 | 以 frontend-ai 身份完成 16:9 多分辨率全页面视觉审阅并输出报告。
- 2026-05-22 18:48 | gpt-5 | 以 frontend-ai 身份根据 16:9 审阅报告优化 Config 长值、空状态、Overview 操作区、Rules 只读节奏、Manual 宽屏阅读和 Settings 退出按钮权重。
- 2026-05-22 23:36 | gpt-5 | 以 test-ai 身份执行 V0.36 真实资产安装测试，记录 RabbitMQ 服务隔离和 Erlang cookie 阻塞证据。
- 2026-05-23 09:20 | gpt-5 | 以 test-ai 身份执行 V0.37 真实资产安装测试，记录 RabbitMQ vhost 403 新阻塞证据。
- 2026-05-23 00:39 | gpt-5 | 以 test-ai 身份补测 V0.37 无 RabbitMQ 干净系统完整安装路径，记录安装中断态证据。
- 2026-05-23 01:21 | gpt-5 | 以 test-ai 身份恢复 Clean-Windows-Installed 干净快照测试 V0.38 完整安装，记录 Erlang 安装 ExitCode null 阻塞。
- 2026-05-23 10:10 | gpt-5 | 以 test-ai 身份复测 FB-021 实际 ProgramData MCP 配置，记录 DPAPI 解密失败和本机 Go SDK 缺标准库阻塞。
- 2026-05-25 08:41 | gpt-5 | 以 test-ai 身份验证 V0.38.1 干净 VM 安装，记录 RabbitMQ ready 超时和 Canal WinSW 1067 阻塞。
- 2026-05-25 08:59 | gpt-5 | 以 backend-ai 身份修复 V0.38.2 RabbitMQ ready PATH 和 Canal Service Java/证据逻辑。
- 2026-05-25 10:02 | gpt-5 | 以 test-ai 身份验证 V0.38.2 干净 VM 安装闭环，记录二次安装 RabbitMQ bootstrap 幂等阻塞。
- 2026-05-25 10:11 | gpt-5 | 以 backend-ai 身份修复 V0.38.3 RabbitMQ bootstrap 二次安装幂等和 runtime 证据清理。
- 2026-05-25 10:28 | gpt-5 | 以 test-ai 身份验证 V0.38.3 干净 VM 安装器完整闭环并关闭 FB-016。
- 2026-05-25 10:35 | gpt-5 | 以 backend-ai 身份实现 V0.39.0 可选 Java/JRE 离线资产支持并生成测试包。
- 2026-05-25 11:27 | gpt-5 | 以 backend-ai 身份实现 V0.40.0 带 Java 离线资产打包并移交 Canal Service 强验。
- 2026-05-25 11:35 | gpt-5 | 新增安装器隔离 VM 测试项目技能和命令 runbook，固化后续 test-ai 接手流程。
- 2026-05-26 10:32 | gpt-5 | 以 test-ai 身份执行 V0.40.0 Canal Service 强验，记录 Java MSI 安装检测阻塞并移交 backend-ai。
- 2026-05-26 11:19 | gpt-5 | 以 backend-ai 身份修复 V0.40.1 Java MSI 探测与日志并重新移交 Canal Service 强验包。
- 2026-05-26 14:44 | gpt-5 | 以 test-ai 身份复测 V0.40.1 Canal Service 强验，记录 Java MSI 1620/2203 阻塞并移交 backend-ai。
- 2026-05-26 15:02 | gpt-5 | 以 backend-ai 身份将 Java 离线资产改为 zip 解压式并生成 V0.40.2 强验包。
- 2026-05-26 15:10 | gpt-5 | 以 test-ai 身份新增 90 天等价长测 Docker/Canal harness，并跑通 prepare、smoke、query 和 archive 轻量验证。
- 2026-05-26 15:28 | gpt-5 | 以 test-ai 身份复测 V0.40.2 Canal Service 强验，记录 JRE 17 不兼容 `PermSize=128m` 阻塞并移交 backend-ai。
- 2026-05-26 15:47 | gpt-5 | 以 backend-ai 身份修复 V0.40.3 Canal startup 旧 JVM 参数并重新移交强验包。
- 2026-05-26 16:04 | gpt-5 | 以 test-ai 身份复测 V0.40.3 Canal Service 强验，记录二次安装 Canal 解压覆盖幂等阻塞并移交 backend-ai。
- 2026-05-26 16:23 | gpt-5 | 以 backend-ai 身份修复 V0.40.4 Canal 运行中二次安装热覆盖并移交测试包。
- 2026-05-26 16:42 | gpt-5 | 以 test-ai 身份复测 V0.40.4 Canal Service 强验，记录卸载 `NodeBridgeCanal` 删除确认延迟阻塞并移交 backend-ai。
- 2026-05-26 16:47 | gpt-5 | 以 backend-ai 身份修复 V0.40.5 卸载服务删除等待并移交测试包。
- 2026-05-26 17:39 | gpt-5 | 以 test-ai 身份复测 V0.40.5 Canal Service 强验并关闭 FB-024，完整安装/验证/二次安装/二次验证/卸载/二次卸载闭环通过。
- 2026-05-23 10:36 | gpt-5 | 以 frontend-ai 身份完成 FB-023，Settings 与说明书 MCP 文案切换为持久启用和 unsupported 配置阻塞语义。
- 2026-05-23 09:24 | gpt-5 | 以 backend-ai 身份补强 stdio MCP 并将 MCP Service 开关改为重启自动关闭。
- 2026-05-23 09:33 | gpt-5 | 以 backend-ai 身份闭合忘记密码决策并修复首次安全草稿配置落盘。
- 2026-05-23 00:53 | gpt-5 | 以 backend-ai 身份完成 V0.38 RabbitMQ 安装器闭环修复并移交测试包。
- 2026-05-23 00:14 | gpt-5 | 以 frontend-ai 身份下沉 Overview 路径诊断信息，并补清密码忘记后的加密字段重置说明。
- 2026-05-23 00:21 | gpt-5 | 以 frontend-ai 身份修正 MCP 会话开关文案和未配置禁用态，并在说明书增加忘记密码恢复流程。
- 2026-05-23 00:41 | gpt-5 | 以 frontend-ai 身份统一用户可见产品名为 `NodeBridge`，同步 Wails 应用名、输出文件名和相关测试引用。
- 2026-05-23 01:14 | gpt-5 | 以 frontend-ai 身份配合后端首次安全草稿约定，Settings 改为发送最小 security DTO，并同步契约文档产品名。
- 2026-05-23 01:28 | gpt-5 | 以 frontend-ai 身份移除顶部管理锁定横幅，将只读/解锁状态改为底部右侧指示器。
- 2026-05-23 10:12 | gpt-5 | 以 frontend-ai 身份将解锁入口集中到右下角胶囊，并把 MCP 仅手动关闭需求登记给后端。
- 2026-05-23 10:28 | gpt-5 | 以 backend-ai 身份修复安装器 ExitCode 兜底探测并持久化 MCP 开关。
- 2026-05-23 10:24 | gpt-5 | 以 frontend-ai 身份将右下角权限胶囊改为状态/动作分段控件，避免状态标签承担点击动作。
- 2026-05-23 10:26 | gpt-5 | 以 review-ai 身份将管理解锁默认有效期从 10 分钟调整为 1 天，并补充后端测试断言。
- 2026-05-26 16:14 | gpt-5 | 以 review-ai 身份升级 AI 工程化初始化提示词为三段式。
- 2026-05-26 16:21 | gpt-5 | 以 review-ai 身份将 AI 初始化提示词收敛为单段完整版本。
- 2026-05-26 22:16 | gpt-5 | 以 backend-ai 身份完成 V0.41 保序批处理和长测夹具修复。
- 2026-05-27 01:35 | gpt-5 | 以 backend-ai 身份修复 Canal ACK 恢复和 CDC 重放幂等。
- 2026-05-27 08:18 | gpt-5 | 以 test-ai 身份复测 V0.42 长测：1 天等价通过，30 天等价因 CDC/offset 未完整抓取阻塞。
- 2026-05-27 09:06 | gpt-5 | 以 backend-ai 身份修复 Canal 空 batch 提交与 offset 推进。
- 2026-05-27 09:18 | gpt-5 | 以 test-ai 身份复测 V0.43 30 天等价，确认队列清空但 Server 仅 46,240/5,184,000 行，回收 Canal 内部日志后继续阻塞。
- 2026-05-27 09:40 | gpt-5 | 以 backend-ai 身份修复 Canal 长测断连不重连和 idle timeout 过短。
- 2026-05-27 10:26 | gpt-5 | 以 test-ai 身份执行 V0.44 零/冒烟级长测回归，确认 Server 越过旧失败点到 116,908 行但未跑满 30d 全量。
- 2026-05-27 11:29 | gpt-5 | 以 test-ai 身份执行 V0.44 分层长测：阶段 0/1 通过，阶段 2 已采样但未跑满 7d 全量。
- 2026-05-28 08:52 | gpt-5 | 以 test-ai 身份记录 FB-025 `staged-v044-month30-001` 30 天等价长测阻塞：Edge=5,184,000，Server=294,780 后超过 10 小时不增长，RabbitMQ 仍有 `edge.upload.cdc.q=4725`、`server.cdc.ingress.q=1056`，失败 ACK 0；已更新 Active Board 转 backend-ai 分析，现场未清理。
- 2026-05-28 09:34 | gpt-5 | 以 test-ai 身份按 backend-ai 诊断优化 30 天长测 harness：补 runner stdout/stderr/exit 证据、SyncAgent 命令日志、drain 未追平 fail-fast、失败 summary 和证据回收；已从干净 Docker volume 启动 `staged-v044-month30-rerun-002`，PID=40072，证据目录 `.cache/longtest-90d/staged-v044-month30-rerun-002/`。
- 2026-05-30 16:50 | gpt-5 | 以 test-ai 身份确认 `staged-v044-month30-rerun-002` 30 天等价 seed 已在用户重启前完成并通过：Edge/Server 六表均为 864,000 行，总计 5,184,000/5,184,000，队列清空，`drain-final.completed=true`，runner summary passed，耗时约 36.99 小时；下一步继续查询性能、归档和 1h/1d/7d 恢复测试。
- 2026-05-30 17:07 | gpt-5 | 以 test-ai 身份优化 90 天等价长测 drain 为常驻 SyncAgent agents 模式，新增核心覆盖率门禁脚本并通过 74.9% >= 70%，优化后 smoke 通过，已后台启动 30 天积压复测 `staged-v044-month30-agents-001`。
- 2026-05-31 01:30 | gpt-5 | 以 backend-ai 身份优化 Edge 上传积压释放：新增 RabbitMQ 批量 publisher confirm 和 Edge upload 批量发布路径，保留顺序与整批失败重投语义；全量测试、vet、核心覆盖率 75.1% 和 `SyncAgent.exe` 构建通过。
- 2026-05-31 01:42 | gpt-5 | 在同一 30d 积压现场热切到批量 confirm 新 SyncAgent，短窗观测 Server apply 提升约 4.56x，Server 收到/落库综合速率提升约 6.08x；瓶颈由 Edge RabbitMQ 上传释放转向 Server MySQL apply。
- 2026-05-31 01:54 | gpt-5 | 修复并重启批量 confirm 优化后的 30d 无人值守完成监控 PID=18336，采样文件 `batchconfirm-watch.csv`，摘要文件 `batchconfirm-watch-summary.json`，等待 Server 追平 5,184,000 行和队列清空。
- 2026-05-31 01:58 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,451,885/5,184,000，Edge 队列约 565,874，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:00 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,471,885/5,184,000，Edge 队列约 565,792，Server ingress=0；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:01 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,481,885/5,184,000，Edge 队列约 552,103，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:03 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,491,885/5,184,000，Edge 队列约 548,734，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:04 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,501,885/5,184,000，Edge 队列约 544,434，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:06 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,511,885/5,184,000，Edge 队列约 540,182，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:08 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,531,885/5,184,000，Edge 队列约 536,888，Server ingress=0；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:09 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,541,885/5,184,000，Edge 队列约 533,249，Server ingress=0；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:10 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,551,885/5,184,000，Edge 队列约 528,780，Server ingress=0；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:12 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,561,885/5,184,000，Edge 队列约 524,971，Server ingress=0；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:14 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,571,885/5,184,000，Edge 队列约 511,860，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:15 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,581,885/5,184,000，Edge 队列约 507,649，Server ingress=10,000；写入手动快照并更新看板，测试尚未完成。
- 2026-05-31 02:23 | gpt-5 | 新增并启动 30d 长测 watchdog，确认 Server=1,641,885/5,184,000 且继续推进；后台 PID=11172，每 5 分钟写进度与摘要。
- 2026-05-31 02:25 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,661,885/5,184,000，Edge 队列约 479,094，watchdog 与 agents 存活；写入手动快照并更新看板。
- 2026-05-31 02:28 | gpt-5 | 确认 30d watchdog 正常连续采样，Server=1,681,885/5,184,000，Edge 队列约 461,409；更新看板和记忆，测试继续跑。
- 2026-05-31 02:30 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,701,885/5,184,000，Edge 队列约 459,304；写入手动快照并更新看板。
- 2026-05-31 02:32 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,711,885/5,184,000，Edge 队列约 456,368；写入手动快照并更新看板。
- 2026-05-31 02:33 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,721,885/5,184,000，Edge 队列约 442,512；写入手动快照并更新看板。
- 2026-05-31 02:35 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,731,885/5,184,000，Edge 队列约 438,674；写入手动快照并更新看板。
- 2026-05-31 02:38 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,751,885/5,184,000，Edge 队列约 430,289；写入手动快照并更新看板。
- 2026-05-31 02:39 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,771,885/5,184,000，Edge 队列约 426,667；写入手动快照并更新看板。
- 2026-05-31 02:43 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,791,885/5,184,000，Edge 队列约 420,100，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:45 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,811,885/5,184,000，Edge 队列约 408,274，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:46 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,821,885/5,184,000，Edge 队列约 405,220，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:48 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,831,885/5,184,000，Edge 队列约 401,939，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:50 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,841,885/5,184,000，Edge 队列约 389,840，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:52 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,861,885/5,184,000，Edge 队列约 386,137，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:53 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,871,885/5,184,000，Edge 队列约 383,369，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:55 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,881,885/5,184,000，Edge 队列约 369,886，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:56 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,891,885/5,184,000，Edge 队列约 366,224，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 02:58 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,911,885/5,184,000，Edge 队列约 363,437，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:00 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,921,885/5,184,000，Edge 队列约 350,704，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:01 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,928,874/5,184,000，Edge 队列约 347,334，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:03 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,947,751/5,184,000，Edge 队列约 343,964，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:05 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,957,751/5,184,000，Edge 队列约 340,999，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:06 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,967,751/5,184,000，Edge 队列约 328,529，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:08 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,977,751/5,184,000，Edge 队列约 325,152，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 03:10 | gpt-5 | 巡检 30d 测试仍在推进，Server=1,997,751/5,184,000，Edge 队列约 321,853，dead/retry=0；写入手动快照并更新看板。
- 2026-05-31 04:32 | gpt-5 | 以 backend-ai 身份将按表 `sync_mode` 补到 Rules 页面和 Manual：`crud_ordered` 用于增删改保序表，`append_only` 用于历史倾倒/采集流水表；相关 Go 包测试和前端构建通过。当前 30d 复测 `appendonly-month30-004` 未通过，Edge=5,184,000 但 Server=0，需继续定位 Server batch apply 卡点。
- 2026-05-31 04:34 | gpt-5 | 停止无效 30d 复测 `appendonly-month30-004` 的 runner/agents，保留 Docker 现场和 `.cache/longtest-90d/appendonly-month30-004/` 证据；该轮不计通过，下一步先补 Server batch apply 可观测性和批次限流再重测。
- 2026-05-31 04:40 | gpt-5 | 修复 append-only 批量 SQL 超过 MySQL prepared statement 占位符上限的问题：多行 INSERT 和 `sync_apply_log` 批量写入按 60000 占位符自动切片；旧失败现场验证 `max-batch=10000` 可 applied，耗时约 11.98s。完整 Go/vet/覆盖率/前端构建门禁通过，已启动新 30d 复测 `appendonly-month30-005` PID=17136。
- 2026-05-31 04:42 | gpt-5 | 巡检新 30d 复测 `appendonly-month30-005`：runner PID=17136 存活，seed 阶段 Edge=1,030,000/5,184,000，Server=0，队列为空，尚未进入 agents drain。
- 2026-05-31 04:58 | gpt-5 | `appendonly-month30-005` 已完成 seed 并进入 agents drain；Edge=5,184,000，Server=90,000，Edge/Server agents 存活且 Server stderr 为空。已启动 watchdog PID=40084，每 5 分钟记录进度到该 run 证据目录。
- 2026-05-31 05:05 | gpt-5 | `appendonly-month30-005` drain 持续推进：实时 Server=290,000/5,184,000，watchdog 05:03 样本 Server=250,000、Edge upload=2,905,880、Server ingress=10,000、agent_count=2、status=running；dead/retry 队列为 0。
- 2026-05-31 05:10 | gpt-5 | `appendonly-month30-005` drain 继续推进：实时 Server=480,000/5,184,000，watchdog 05:08 样本 Server=410,000、Edge upload=4,709,923、Server ingress=0、agent_count=2、status=running；当前 dead/retry 仍为 0。
- 2026-05-31 05:12 | gpt-5 | 以 test-ai 身份确认按表 `sync_mode` 是当前正确产品方向：历史倾倒/采集流水表使用 `append_only` 批量加速，低频增删改查表继续用默认 `crud_ordered` 保序；`appendonly-month30-005` 实时快照 Edge=5,184,000、Server=560,000、dead/retry=0，测试仍在后台推进。
- 2026-05-31 05:13 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：runner/watchdog/Edge agent/Server agent 均存活，实时 Edge=5,184,000、Server=610,000，Edge upload=4,374,000、Server ingress=200,000，dead/retry=0，agent stderr 为空；30 天量级复测未完成，继续后台 drain。
- 2026-05-31 05:19 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=850,000，Edge upload=3,884,000、Server ingress=460,000，dead/retry=0；watchdog 05:18 样本 Server=790,000、agent_count=2、status=running，30 天量级复测继续推进但尚未完成。
- 2026-05-31 05:21 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：runner/watchdog/agents 存活，实时 Edge=5,184,000、Server=890,000，Edge upload=3,784,000、Server ingress=510,000，dead/retry=0；仍未达到完成审计条件。
- 2026-05-31 05:32 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,260,000，Edge upload=2,914,000、Server ingress=1,010,000，dead/retry=0；watchdog 05:28 样本 Server=1,140,000、agent_count=2、status=running，30 天量级复测继续推进但尚未完成。
- 2026-05-31 05:33 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：runner/watchdog/agents 存活且 agent stderr 为空，实时 Edge=5,184,000、Server=1,290,000，Edge upload=2,834,000、Server ingress=1,060,000，dead/retry=0；仍未达到完成审计条件。
- 2026-05-31 05:34 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,320,000，Edge upload=2,754,000、Server ingress=1,110,000，dead/retry=0；watchdog 05:33 样本 Server=1,300,000、agent_count=2、status=running，仍未达到完成审计条件。
- 2026-05-31 05:35 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,350,000，Edge upload=2,674,000、Server ingress=1,160,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:36 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,380,000，Edge upload=2,604,000、Server ingress=1,210,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:37 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,400,000，Edge upload=2,534,000、Server ingress=1,260,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:38 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,430,000，Edge upload=2,454,000、Server ingress=1,300,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:39 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,460,000，Edge upload=2,384,000、Server ingress=1,353,201，dead/retry=0；watchdog 05:38 样本 Server=1,450,000、agent_count=2、status=running，仍未达到完成审计条件。
- 2026-05-31 05:39 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,480,000，Edge upload=2,324,000、Server ingress=1,390,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:40 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,510,000，Edge upload=2,244,000、Server ingress=1,440,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:41 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,530,000，Edge upload=2,174,000、Server ingress=1,490,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:42 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,560,000，Edge upload=2,094,000、Server ingress=1,530,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:43 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,590,000，Edge upload=2,014,000、Server ingress=1,590,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:44 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,610,000，Edge upload=1,934,000、Server ingress=1,640,000，dead/retry=0；watchdog 05:43 样本 Server=1,590,000、agent_count=2、status=running，仍未达到完成审计条件。
- 2026-05-31 05:45 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,640,000，Edge upload=1,859,986、Server ingress=1,700,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:46 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,660,000，Edge upload=1,784,000、Server ingress=1,740,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:47 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,690,000，Edge upload=1,694,000、Server ingress=1,800,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:48 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,720,000，Edge upload=1,614,000、Server ingress=1,850,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:49 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,750,000，Edge upload=1,514,000、Server ingress=1,920,000，dead/retry=0；watchdog 05:48 样本 Server=1,720,000、agent_count=2、status=running，仍未达到完成审计条件。
- 2026-05-31 05:51 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,780,000，Edge upload=1,444,000、Server ingress=1,960,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:51 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,800,000，Edge upload=1,374,000、Server ingress=2,010,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:52 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,820,000，Edge upload=1,314,000、Server ingress=2,050,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:53 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,850,000，Edge upload=1,234,000、Server ingress=2,110,000，dead/retry=0；watchdog 05:53 样本 Server=1,850,000、agent_count=2、status=running，仍未达到完成审计条件。
- 2026-05-31 05:54 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,880,000，Edge upload=1,154,000、Server ingress=2,160,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:55 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,900,000，Edge upload=1,074,000、Server ingress=2,210,000，dead/retry=0；确认按表 `sync_mode` 适合区分历史倾倒表和增删改查表，测试仍未达到完成审计条件。
- 2026-05-31 05:57 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,950,000，Edge upload=934,000、Server ingress=2,300,988，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 05:58 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=1,970,000，Edge upload=864,000、Server ingress=2,350,000，dead/retry=0；agents CPU 仍增长但 watchdog 文件暂无新采样，仍未达到完成审计条件。
- 2026-05-31 05:59 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,000,000，Edge upload=784,000、Server ingress=2,400,834，dead/retry=0；watchdog 已恢复采样，runner/watchdog/agents 存活，仍未达到完成审计条件。
- 2026-05-31 06:00 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,030,000，Edge upload=704,000、Server ingress=2,460,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:01 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,050,000，Edge upload=634,000、Server ingress=2,500,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:02 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,070,000，Edge upload=564,000、Server ingress=2,550,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:03 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,090,000，Edge upload=494,000、Server ingress=2,600,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:04 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,120,000，Edge upload=424,000、Server ingress=2,644,320，dead/retry=0；watchdog 06:03 样本 Server=2,110,000、agent_count=2、status=running，仍未达到完成审计条件。
- 2026-05-31 06:05 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,140,000，Edge upload=344,000、Server ingress=2,700,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:05 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,170,000，Edge upload=274,000、Server ingress=2,750,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:06 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,190,000，Edge upload=204,000、Server ingress=2,790,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:07 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,210,000，Edge upload=134,000、Server ingress=2,840,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:08 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,240,000，Edge upload=54,000、Server ingress=2,900,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:09 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,260,000，Edge upload=0、Server ingress=2,934,000，dead/retry=0；Edge upload 已清空，进入 Server ingress 单独 drain 阶段，仍未达到完成审计条件。
- 2026-05-31 06:10 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,280,000，Edge upload=0、Server ingress=2,904,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:11 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,310,000，Edge upload=0、Server ingress=2,874,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:12 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,340,000，Edge upload=0、Server ingress=2,854,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:13 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,360,000，Edge upload=0、Server ingress=2,824,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:14 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,380,000，Edge upload=0、Server ingress=2,804,000，dead/retry=0；watchdog 06:14 样本正常，仍未达到完成审计条件。
- 2026-05-31 06:15 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,410,000，Edge upload=0、Server ingress=2,774,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:16 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,430,000，Edge upload=0、Server ingress=2,754,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:17 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,460,000，Edge upload=0、Server ingress=2,724,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:18 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,490,000，Edge upload=0、Server ingress=2,701,368，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:19 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,510,000，Edge upload=0、Server ingress=2,674,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:20 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,540,000，Edge upload=0、Server ingress=2,654,000，dead/retry=0；watchdog 06:19 样本正常，仍未达到完成审计条件。
- 2026-05-31 06:20 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,560,000，Edge upload=0、Server ingress=2,624,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:21 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,590,000，Edge upload=0、Server ingress=2,604,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:22 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,610,000，Edge upload=0、Server ingress=2,574,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:23 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,640,000，Edge upload=0、Server ingress=2,554,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:24 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,660,000，Edge upload=0、Server ingress=2,524,000，dead/retry=0；watchdog 06:24 样本正常，仍未达到完成审计条件。
- 2026-05-31 06:25 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,690,000，Edge upload=0、Server ingress=2,494,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:28 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,780,000，Edge upload=0、Server ingress=2,414,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:30 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,800,000，Edge upload=0、Server ingress=2,384,000，dead/retry=0；watchdog 06:29 样本正常，仍未达到完成审计条件。
- 2026-05-31 06:31 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,840,000，Edge upload=0、Server ingress=2,354,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:32 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,870,000，Edge upload=0、Server ingress=2,324,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:33 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,900,000，Edge upload=0、Server ingress=2,294,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:35 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,960,000，Edge upload=0、Server ingress=2,234,000，dead/retry=0；watchdog 06:34 样本正常，仍未达到完成审计条件。
- 2026-05-31 06:37 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=2,980,000，Edge upload=0、Server ingress=2,204,000，dead/retry=0；Server ingress 单独 drain 持续推进，仍未达到完成审计条件。
- 2026-05-31 06:39 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=3,030,000，Edge upload=0、Server ingress=2,154,000，dead/retry=0；继续验证按表 `sync_mode` 区分历史倾倒表 `append_only` 与 CRUD 保序表 `crud_ordered`，仍未达到完成审计条件。
- 2026-05-31 06:40 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=3,060,000，Edge upload=0、Server ingress=2,124,000，dead/retry=0；runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 06:45 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=3,210,000，Edge upload=0、Server ingress=1,984,000，dead/retry=0；近 5.6 分钟增加约 150,000 行，约 26,800 行/分钟，仍未达到完成审计条件。
- 2026-05-31 06:56 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=3,480,000，Edge upload=0、Server ingress=1,714,000，dead/retry=0；近 10.7 分钟增加约 270,000 行，约 25,300 行/分钟，agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 07:12 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=3,870,000，Edge upload=0、Server ingress=1,314,000，dead/retry=0；近 15.6 分钟增加约 390,000 行，约 25,000 行/分钟，runner/watchdog/agents 存活且 agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 07:27 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=4,250,000，Edge upload=0、Server ingress=934,000，dead/retry=0；近 15.5 分钟增加约 380,000 行，约 24,500 行/分钟，agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 07:43 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=4,640,000，Edge upload=0、Server ingress=554,000，dead/retry=0；近 15.6 分钟增加约 390,000 行，约 25,000 行/分钟，进入最后 55 万后改为 5 分钟采样，仍未达到完成审计条件。
- 2026-05-31 07:48 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=4,770,000，Edge upload=0、Server ingress=414,000，dead/retry=0；近 5.5 分钟增加约 130,000 行，约 23,500 行/分钟，agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 07:54 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=4,910,000，Edge upload=0、Server ingress=274,000，dead/retry=0；近 5.6 分钟增加约 140,000 行，约 25,000 行/分钟，agent stderr 为空，仍未达到完成审计条件。
- 2026-05-31 08:00 | gpt-5 | 以 test-ai 身份巡检 `appendonly-month30-005`：实时 Edge=5,184,000、Server=5,040,000，Edge upload=0、Server ingress=144,000，dead/retry=0；进入最后 3% 后改为 2 分钟短轮询，仍未达到完成审计条件。
- 2026-05-31 08:07 | gpt-5 | 以 test-ai 身份完成 `appendonly-month30-005` 30 天量级复测审计：runner-summary `passed`，drain-final `completed=true`；Edge/Server 总数均 5,184,000，6 张表各 864,000；Edge upload/retry/dead/downlink 与 Server ingress/dead/downlink 队列全 0；`sync_apply_log=5,184,000`、`sync_event_log=5,184,000` 且 payload 全 NULL、`sync_ack_log=0`，agent stderr 与错误关键字扫描为空；已停止残留 watchdog。
- 2026-05-31 08:43 | gpt-5 | 用户要求暂停 5 张保序标签表 + 12 张 append-only 倾倒表的 15 天断网压测；已停止 `mixed-smoke-001` 残留脚本进程。已在 `AI_BOARD.md` 登记 FB-026 交给 frontend-ai 联调 `sync_mode` 规则选项：默认 `crud_ordered`，历史倾倒表可选 `append_only`，需确认保存/读取闭环、危险提示和三语文案。
- 2026-06-01 09:04 | gpt-5 | 以 test-ai 身份启动 15 天混合断网压力测：有效 run `mixed-15d-offline-002`，runner PID=41444、Edge agent PID=29348，证据目录 `.cache/longtest-90d/mixed-15d-offline-002/`；测试覆盖 5 张 `tag_state_*` 保序 CRUD 更新表和 12 张 `collect_data_*` append-only 倾倒表，预期业务行 5,189,000、预期事件 7,349,000。无效首轮 `mixed-15d-offline-001` 已停止并修正 harness 断网顺序；启动确认 Server RabbitMQ 已停止模拟断网，Edge agent 存活且 stderr 为空，Edge append_rows=80,000、tag_rows=5,000。按用户要求不持续盯进度。
- 2026-06-01 09:10 | gpt-5 | 以 test-ai 身份巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows 从 120,000 增至 240,000、tag_rows=5,000，Edge upload 队列从 169,604 增至 299,264，retry/dead=0；runner/agent 日志关键字扫描未见 panic/fatal/Error 1390/FAILED，当前为正常积压生成阶段。
- 2026-06-01 10:31 | gpt-5 | 以 test-ai 身份巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=1,560,000/5,184,000，tag_rows=5,000，Server 业务行仍 0，Edge upload 队列=2,199,994、retry/dead=0；runner.err 仅 MySQL password warning，edge.err 为空，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，当前仍在断网积压生成阶段。
- 2026-06-01 14:05 | gpt-5 | 以 test-ai 身份巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=3,480,000/5,184,000，tag_rows=5,000，Server 业务行仍 0，Edge upload 队列=4,904,324、retry/dead=0；runner.out 最新进度 `rows_each_append=200000/432000`，日志关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，仍在断网积压生成阶段。
- 2026-06-01 15:54 | gpt-5 | 以 test-ai 身份巡检 `mixed-15d-offline-002`：runner PID=41444、Edge agent PID=29348 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=4,320,000/5,184,000，tag_rows=5,000，Server 业务行仍 0；Edge upload 队列=6,081,325/预期事件 7,349,000，retry/dead=0；runner.err 仍为 MySQL password warning，edge.err 为空，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED，仍在断网积压生成阶段。
- 2026-06-01 17:04 | gpt-5 | 用户要求 `mixed-15d-offline-002` “灌完数据先停，明天再测恢复”：当前仍在断网灌数，runner PID=41444、Edge agent PID=29348 均存活；Edge append_rows=4,800,000/5,184,000，tag_rows=5,000，Edge upload 队列约 6,793,892/7,349,000，tag 版本约 389k-400k/432k。已启动 pause monitor PID=37368，每 10 秒检查；达到 append_rows=5,184,000 且 edge_upload>=7,349,000 后会自动停止 runner 和 Edge agent，并保持 Server RabbitMQ stopped，写 `.cache/longtest-90d/mixed-15d-offline-002/pause-after-seed.json`。
- 2026-06-01 17:18 | gpt-5 | 以 test-ai 身份巡检 `mixed-15d-offline-002` 暂停前状态：runner PID=41444、Edge agent PID=29348、pause monitor PID=37368 均存活；Server RabbitMQ 仍停止，符合断网模拟；Edge append_rows=4,920,000/5,184,000，还差 264,000；Edge upload 队列=6,959,575/7,349,000，还差 389,425；pause marker 尚未生成，monitor 正常采样；retry/dead=0，关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED。
- 2026-06-01 17:59 | gpt-5 | `mixed-15d-offline-002` 已按用户要求停在“离线积压已灌完、尚未恢复同步”状态：`pause-after-seed.json` 17:55:42 生成，append_rows=5,184,000、edge_upload_depth=7,349,000，target reached 后已停止 runner PID=41444 和 Edge agent PID=29348；Server RabbitMQ 仍 stopped，Edge upload.cdc.q=7,349,000 ready / 0 unacked，retry/dead=0，Server 业务行仍 0。明天可从该现场启动 Server RabbitMQ 和 agents 做恢复 drain。
- 2026-06-02 14:12 | gpt-5 | 以 test-ai 身份复核 `mixed-15d-offline-002` 重启后的恢复测试前置条件：先只拉起 Edge/Server MySQL、Edge RabbitMQ、Edge Canal，保持 Server RabbitMQ stopped；Edge RabbitMQ 用约 107 秒完成 7,349,000 条持久消息索引重建，edge.upload.cdc.q=7,349,000 ready / 0 unacked / 0 consumers，retry/dead=0；Edge append_rows=5,184,000、tag_rows=5,000，Server 业务行仍 0；无残留 runner/agent/pause monitor，日志关键字扫描未见 panic/fatal/Error 1390/Error 1062/FAILED。可以从该现场启动 Server RabbitMQ 和 agents 做恢复 drain，不需要重灌数据。
- 2026-06-02 14:23 | gpt-5 | 以 test-ai 身份启动 `mixed-15d-offline-002` 恢复 drain：第一次控制器因 Server RabbitMQ 未实际启动导致 init 拓扑失败，未启动 agents、未消费积压；随后手动启动 Server RabbitMQ、初始化 Server 拓扑成功，并启动修正后的恢复控制器 PID=22524、Edge/Server SyncAgent PID=23644/4228。14:23 样本显示恢复已运行：ServerRows=70,000、ApplyLog=70,000、Edge upload=7,177,118、Server ingress=97,092，当前 controller-2/edge/server stderr 均为空；日志在 `.cache/longtest-90d/mixed-15d-offline-002/agents-recovery/`，进度在 `recovery-progress.csv` / `recovery-summary.json`。
- 2026-06-02 16:20 | gpt-5 | 以 review-ai 身份把 Server Apply 成倍性能优化计划写入 `AI_BOARD.md` 的 FB-028，移交 backend-ai 实施：当前性能瓶颈明确在 Server apply，计划按 P0-P5 推进，优先分流 `append_only` 与 `crud_ordered`，append-only 独立高速 worker/队列并批量业务 INSERT + 批量 `sync_apply_log`，CRUD 按 `target_table + pk` hash lane 并行且 lane 内保序，后续评估 `crud_ordered_compact` 合并同主键连续 UPDATE 和 append-only `LOAD DATA`/staging table。硬约束：ACK 仍在业务写和系统日志提交后，SyncEvent 映射、回环抑制、幂等、失败重试/死信不变量不破坏；目标 append-only >=1,500 rows/s、mixed apply >=1,000 events/s。
- 2026-06-02 17:39 | gpt-5 | 以 test-ai 身份巡检 `mixed-15d-offline-002` 恢复 drain：Edge upload 已清空，Server ingress=3,599,000，ServerRows=2,650,000/5,189,000，ApplyLog=3,750,000/7,349,000；恢复控制器 PID=22524、Edge/Server SyncAgent PID=23644/4228 均存活，controller-2/edge/server stderr 为空，retry/dead=0。最近约 20 分钟 apply 约 285 events/s，剩余约 3.6M apply events，预计还需约 3.5 小时；继续后台运行，不持续盯进度。
- 2026-06-02 16:25 | gpt-5 | 以 backend-ai 身份完成 FB-028 P1a：mixed batch 连续 append-only 段按目标表批量写入，CRUD 边界不跨越；全量 Go 测试和 vet 通过，`golangci-lint` 未安装跳过，已重建 `build/bin/SyncAgent.exe` 并移交 test-ai 复测吞吐。
- 2026-06-02 16:45 | gpt-5 | 以 backend-ai 身份完成 FB-028 P2/P3/P4 后端：新增 `sync.apply_lanes`、`sync.enable_crud_compact` 和 `crud_ordered_compact` 双门槛；全量 Go 测试和 vet 通过，重建 `build/bin/SyncAgent.exe`，新增 FB-029 交前端补 Settings 开关和单规则选项。
- 2026-06-02 16:54 | gpt-5 | 以 backend-ai 身份登记 MCP 远程 AI 受控改配置目标，新增 FB-030/FB-031 交前后端分工。
- 2026-06-02 17:07 | gpt-5 | 以 backend-ai 身份完成 MCP 白名单写配置、规则保存和启用门禁，测试通过并重建 SyncAgent。
- 2026-06-02 17:28 | gpt-5 | 以 backend-ai 身份安装 golangci-lint 2.12.2，增强 MCP dry-run/schema/拒绝审计，Go test/vet/lint 通过并重建 SyncAgent。
- 2026-06-03 08:45 | gpt-5 | 以 backend-ai 身份完成 MCP 真实 stdio smoke，补禁用态 stderr 诊断，Go test/vet/lint 通过并重建 SyncAgent。
- 2026-06-03 09:05 | gpt-5 | 以 backend-ai 身份集中测试凭据文档，清理旧 VM 文档散落密码引用。
- 2026-06-03 10:19 | gpt-5 | 以 backend-ai 身份生成本机解压试用包，包内 SyncAgent/DataSync smoke 通过。
- 2026-06-03 10:31 | gpt-5 | 以 review-ai 身份新增 FB-032，委托 backend-ai 制作 NSIS beta 安装器并明确本机 Docker 非破坏 smoke 边界。
- 2026-06-03 10:42 | gpt-5 | 以 backend-ai 身份完成 NSIS beta 安装器源码、安装/卸载包装脚本和 staging 打包验证；最终 exe 编译待 `makensis.exe`。
- 2026-06-03 10:52 | gpt-5 | 以 backend-ai 身份安装 NSIS 3.12 并生成 `NodeBridge-beta-v0.45.0-20260603.exe`，未执行安装器本身或真实系统组件安装。
- 2026-06-03 11:02 | gpt-5 | 以 backend-ai 身份将管理端交付 exe 从旧 `DataSync.exe` 修正为 `NodeBridge.exe`，重新生成 NSIS beta 包并更新试用手册。
- 2026-06-03 11:24 | gpt-5 | 以 backend-ai 身份根据测试截图修复管理端 Wails 构建方式和 NSIS 默认强验策略，重新生成 beta 安装包。
- 2026-06-03 14:06 | gpt-5 | 以 backend-ai 身份修复 NSIS beta 安全草稿配置导致安装尾段失败、默认 RabbitMQ URL 与 bootstrap 账号/vhost 不一致导致 403 的问题，并重新生成安装包。
- 2026-06-03 14:20 | gpt-5 | 以 backend-ai 身份修复 NSIS beta 调用 headless 安装脚本时 `Program Files` 路径空格导致 exit code -196608 的问题，并重新生成安装包。
