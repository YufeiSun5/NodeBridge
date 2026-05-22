# Managed Components Boundary

## 目标

安装程序可以默认安装 NodeBridge 需要的组件，但必须只管理 NodeBridge 自己创建的资源。客户已有 MySQL、RabbitMQ、Canal 可以配置使用，NodeBridge 不擅自修改或删除。

## 资源归属

安装后写入：

```text
%ProgramData%\NodeBridge\install-manifest.json
```

manifest 记录：

- `product`: 固定为 `NodeBridge`。
- `install_id`: 本次安装唯一 ID。
- `managed_components.rabbitmq`: NodeBridge 创建的 RabbitMQ service、vhost、user、topology tag。
- `managed_components.canal`: NodeBridge 创建的 Canal service、config dir、destination。

卸载和修复只能处理 manifest 中登记的资源。没有登记的 RabbitMQ vhost/user、Canal destination、客户配置文件一律视为外部资源。

## RabbitMQ 默认受管资源

```text
service: NodeBridgeRabbitMQ
vhost: /nodebridge-edge
vhost: /nodebridge-server
user: nb-server-sync
user: nb-edge-001
user: nb-edge-001-local
topology_tag: nodebridge
```

队列和 exchange 当前仍使用同步协议名，例如 `edge.upload.cdc.q`、`server.cdc.ingress.q`。隔离边界依赖 NodeBridge vhost 和账号权限，不依赖全局队列名前缀。

## Canal 默认受管资源

```text
service: NodeBridgeCanal
config_dir: %ProgramData%\NodeBridge\canal
destination: nodebridge-edge-001
destination: nodebridge-server-001
```

Canal 配置只写入 NodeBridge config dir。外部 Canal 只保存 `canal_addr`、`destination`、`filter` 等连接参数，不修改客户 Canal 配置。

## 配置模式

```yaml
rabbitmq:
  mode: managed   # managed 或 external
  install: true

cdc:
  type: canal
  mode: managed   # managed 或 external
  install: true
```

- `managed`: 安装器安装/初始化 NodeBridge 自己的资源。
- `external`: 只读取用户提供的连接配置，不创建、不删除、不改权限。

## 约束

- 不使用 `guest` 账号。
- 不修改客户已有 RabbitMQ vhost/user/permission。
- 不修改客户已有 Canal config dir 或 destination。
- 不把 Docker 作为交付组件；Docker 仅用于开发测试。
- 敏感字段仍通过 DPAPI 或等价机制加密保存。

## V0.31 Alpha 执行器

当前已有后端 alpha 命令：

```powershell
SyncAgent.exe managed-plan -config config.yaml -manifest install-manifest.json
SyncAgent.exe managed-apply -config config.yaml -manifest install-manifest.json
SyncAgent.exe managed-repair -config config.yaml -manifest install-manifest.json
SyncAgent.exe managed-uninstall -config config.yaml -manifest install-manifest.json
```

当前 alpha 只执行安全动作：

- 写入 `install-manifest.json`。
- 生成 NodeBridge Canal destination 配置。
- 在配置的 AMQP URL 可连接时初始化 NodeBridge RabbitMQ topology。

真实 Erlang/RabbitMQ/Canal 离线安装包执行和 Windows Service 注册仍属于后续 beta。

## V0.33 离线包预检

V0.33 只做安全预检，不真实安装：

```powershell
SyncAgent.exe installer-assets-check -catalog deploy/windows/nodebridge-assets.example.json
SyncAgent.exe installer-command-plan -catalog deploy/windows/nodebridge-assets.example.json
```

已实现能力：

- 读取离线包 catalog。
- 校验 Erlang/RabbitMQ/Canal 安装包路径。
- 计算 SHA256 并对比期望值。
- 输出 Windows 安装命令计划。

明确不做：

- 不执行静默安装。
- 不注册或启动 Windows Service。
- 不修改客户已有 Erlang/RabbitMQ/Canal。
- 不写注册表、不改系统环境变量。

真实执行安装前必须先在 Windows VM 或明确授权的测试机里验证。
