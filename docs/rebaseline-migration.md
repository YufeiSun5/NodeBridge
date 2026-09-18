# 双向计划变更与迁移接续（0.48.8）

适用：一个 Edge 与一个 Server，首次恢复以 **Edge 的当前业务数据为准**。支持重建规则 ID、源/目标库表、主键和列映射；可以同时包含 EDGE_TO_SERVER 与 BIDIRECTIONAL 规则。显示名称 `name` 仍可直接修改，不需要重建。

## 为什么不能直接修改旧计划

旧计划绑定两端实际库表、规则摘要、复制收据与 CDC 边界。直接改规则或删除账本会让旧消息、行版本与新映射混用。0.48.8 增加明确的“预览 → 备份并准备新一代 → 重新对齐”流程；旧计划完整保留，正常保存仍阻止绕过对齐。

此次现场选择：105 为权威源，108 接收新基线。安装补丁本身不会清空业务数据，也不会自动执行下面的迁移。

## 升级与准备

1. 停止两端同步 Agent，退出旧管理程序，断开旧 MCP 进程；两端安装相同版本，选择复用已有组件。安装器会执行系统库升级，包含 `011_rebaseline.sql`。重新连接 MCP，确认版本与新工具可见。
2. 在主站调用已有“配置边缘账号”MCP 工具 `nodebridge_ensure_server_edge_user`，参数 `node_id` 为实际边缘节点 ID。新权限包含首次对齐所需的默认交换机与限定队列；外部 RabbitMQ 由管理员设置等价权限。
3. 暂停目标业务表的其他写入程序，并在全部规则对齐、启用和验证完成前保持暂停。准备操作会**备份并清空新规则指定的整张目标表**，之后由边缘快照填充。边缘原业务行不由准备操作修改。
4. 确认两端 MySQL 账号可建系统库、执行迁移；主站还需目标 SELECT/DELETE 和备份建表/INSERT 权限及足够磁盘空间。目标表必须是有主键的 InnoDB，不能有触发器或引用/被引用的外键；不支持该条件时明确拒绝。
5. 外部 Canal 必须允许新业务库表和新系统库的 `sync_capture_fence`、运行期回放标记通过。工具会更新本地 `cdc.filter`；不会擅自重启或修改外部 Canal 服务。静态白名单限制需在服务侧同步调整。

## MCP 执行流程

两端分别调用 `nodebridge_plan_rebaseline`，使用相同 `migration_id`、`edge_node_id`、`server_node_id` 和完整目标规则集。例如：

```json
{
  "migration_id": "restore-105-to-108-20260917",
  "edge_node_id": "edge-001",
  "server_node_id": "server-001"
}
```

省略 `rules` 表示使用该端现有全部规则。要修改双向计划，传入完整 `rules` 数组，字段与保存同步规则一致，包括未改变的规则。工具规范化为禁用、MANUAL 对齐；不是只提交某一条增量。两端最终语义必须一致。

保存两端各自返回的完整 plan JSON，核对旧/新系统库、目标库表、`new_cdc_filter`、备份表与旧证明。两端的 `plan_id` 不同是正常的：本地身份、结构与配置修订各自绑定。

两端分别调用 `nodebridge_prepare_rebaseline`：

```json
{
  "plan": { "这里放本端完整预览结果": "不要手工修改" },
  "confirm": true,
  "target_writers_stopped": true
}
```

上面的 plan 是说明占位，不是可直接执行的请求。`target_writers_stopped` 表示操作者确认已暂停目标业务写入，不是软件已经自动停掉这些程序。

返回 `prepared: true` 后：

- 当前系统库切换到确定性的 `nb_gen_…`，业务库不改名。
- 旧系统库原样保留，包括旧作业、证明、行版本、墓碑和位点。
- 配置、规则及配对文件备份到返回的 `backup_directory`；旧配对文件从当前入口移除，新证明稍后生成。
- 主站备份表位于新系统库，名称列在 `local_tables[].backup_table`。备份行、目标 DELETE 和准备收据同事务提交。
- 所有目标规则保持禁用，Agent 不会自动启动。

随后对每一条规则，在两端调用原有 `nodebridge_start_initial_alignment`（`rule_id`、对端 `peer_node_id`、`confirm:true`），保持 MCP 会话并轮询 `nodebridge_initial_alignment_status`。两端都 `completed` 才处理下一条。所有规则完成后，读取最新规则修订，通过正常 CAS 保存启用，再启动两端 Agent。核对上下行新增、更新、删除及表数据，再恢复目标业务写入。

## 中断、重试与恢复边界

- 准备阶段断线：先保持 Agent 停止，重连后提交**原本完整的同一计划**。不能重新预览并替换成另一个计划冒充继续。SQL 准备收据避免重复清空；已复制完成后重复原准备也不会删除新基线。
- 部分文件尚未发布时，`.rebaseline-pending.json` 会阻止 Agent 和对齐启动。保留此文件，通过原计划继续；不要手工删除解锁。
- 当前配置/规则/结构变化会被拒绝；目标恢复写入后新对齐也会拒绝反向复制。修正原因后再按明确状态处理。
- 任何计划中的规则尚未完成对齐，新一代都不能启动。完成一条不代表全部完成。
- 已知旧 epoch 的消息按原证明校验身份后写入 superseded 审计再确认，未知 epoch 不因迁移而自动放行；没有清空 RabbitMQ 队列。
- 不提供一键在线回滚。若需回退，先停两端 Agent 和业务写入，核对备份与两端已提交状态，再制定恢复方案；只恢复旧配置不能撤销已发生的业务变化。
- 多边缘拓扑迁移、在线修改、自动合并双方现有数据不在此版本范围内；检测到旧拓扑存在其他成员时拒绝，不能借改 ID 绕过。

## English

Version 0.48.8 adds explicit offline rebaselining for one edge/server pair. The edge is authoritative. Preview on both endpoints with the same migration ID and full desired rules, then prepare each exact local plan with confirmation and target writers paused. Target tables are transactionally backed up and emptied; old metadata remains intact. Align every rule on both endpoints, enable via the normal revision-checked save, then start and verify synchronization. Preparation is resumable and is not proof of successful synchronization. Both endpoints must be upgraded. Triggered/FK target tables and multi-edge migrations are rejected.

## 日本語

0.48.8 は Edge 1 台と Server 1 台のオフライン再同期基準作成に対応します。Edge 側を正とし、両端で同じ移行 ID と全ルールをプレビューし、各端の計画を確認して準備します。対象表への業務書き込みを停止してください。対象データのバックアップ・削除・準備記録は同一トランザクションで確定し、旧管理 DB は保持します。全ルールを両端で初期同期後、有効化・起動・検証します。準備完了は同期成功ではありません。両端の更新が必要です。トリガー／外部キー付き対象表、多 Edge の移行は拒否されます。
