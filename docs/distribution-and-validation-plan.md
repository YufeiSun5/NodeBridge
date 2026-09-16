# 分发与验证计划 / Distribution plan / 配布計画

更新：2026-09-16。状态：待实施，不代表已上架、已签名或新增测试已通过。

English: Implement public CI, then a reproducible demo; evaluate WinGet and trusted signing afterward. Homebrew requires platform validation first.

日本語：公開 CI、再現可能なデモ、WinGet と署名の順に進めます。Homebrew は対応 OS の検証後です。

## 公开 CI

- [ ] 固定工具版本，运行 Go test/vet/lint、前端检查、Windows 编译。
- [ ] 整理安装包依赖，在干净 Runner 构建并保存日志与产物。
- [ ] README 添加状态徽章，验证错误提交会失败，再配置合并门禁。
- [ ] 快速检查随提交运行，较重集成与打包检查在发行前或手动运行。

## Demo 与短演示

- [ ] 先验证 Linux 容器内 SyncAgent 与 CDC 兼容性。
- [ ] Compose 提供主站、两个从站及 MySQL/RabbitMQ/Canal，固定版本。
- [ ] 提供独立示例数据、健康检查、启动、验证和仅清理 Demo 资源的命令。
- [ ] 验证新增、更新、删除和断线恢复，核对最终数据。
- [ ] 三语说明、约两分钟短演示，另展示 Windows GUI。

实测前不承诺三分钟启动；首次镜像下载单独计时。不得使用业务凭据或将三逻辑节点写成三台物理机。

## WinGet / Chocolatey 与签名

- [ ] 干净 Windows 验证静默安装、升级、卸载、识别和返回码。
- [ ] 验证默认复用与显式安装 RabbitMQ/Canal/Erlang/Java；保留外部配置及资源归属。
- [ ] 核对第三方许可证、组件版本、校验和与卸载边界。
- [ ] 准备 WinGet 清单与固定版本下载地址，本地通过后提交审核。
- [ ] 单独评估 Chocolatey 规则与维护成本。
- [ ] 确定可信签名证书或服务、身份验证、费用和密钥管理，再接入发布流程。

平台审核与签名资质属于外部条件。Release 下载计数无需上架即可查看，不等于用户数或部署量。

## Homebrew 与社区

- [ ] 先验证 macOS/Linux Agent 和服务管理，再评估自有 Tap；不直接分发 Windows EXE。
- [ ] 官方收录另核对稳定版本、关注度与平台要求；macOS GUI 另评估 Cask。
- [ ] 整理可公开测试证据、Issue 模板、贡献指南与 Discussions。
- [ ] 分类归档历史交接报告，保留有效开发规范。
- [ ] 社区发帖须明确授权目标和内容，不制造使用指标或外部贡献。

真实部署只能依据可核对且允许公开的记录。11 节点计划不是完成结果；隔离测试、现场验证和生产使用分别描述。
