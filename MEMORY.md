# MEMORY

Last updated: 2026-09-12 01:42 Asia/Singapore

## 当前阶段

- 本轮已于累计5小时上限停止，未生成新安装包；停止不代表功能完成。FB-107及列明缺口保持open，未部署/长测；后续开发等待用户新指令。

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

- 2026-09-12 15:41 | Codex / backend-ai | 按用户新指令提交并推送累计开发进度到GitHub；仅版本归档，测试临时产物留本地，不继续功能开发或出包，FB-107/FB-094保持open。

- 2026-09-12 01:39 | Codex / backend-ai | 补取消后迟到Canal标记拒绝及账本损坏校验；全量test/vet/lint与核心shuffle两轮通过，本轮测试容器/候选Agent/UI端口已清理，正式交接/新安装包仍未完成。

- 2026-09-12 01:30 | Codex / backend-ai | FB-107补源提交前摘要、目标原事务边界恢复、过期计划恢复区分及锁覆盖；真实短测/三语状态回归与全量门禁通过，首次全量到增量交接仍未闭合，未出包。
