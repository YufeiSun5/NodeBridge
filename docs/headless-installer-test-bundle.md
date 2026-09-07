# Headless Installer Test Bundle

本文件交给 `test-ai` 使用。目标环境是 Windows Server 2022 Core / 无 GUI VM。

在 Hyper-V 安装器 VM 中执行真实安装测试时，先使用项目技能 `.ai/skills/installer-vm-test/SKILL.md`，完整宿主机 PowerShell 命令见 [Installer VM Test Runbook](installer-vm-test-runbook.md)。

## 产物

生成命令：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package-headless-installer-test.ps1
```

输出：

```text
build/headless-installer-test/
build/NodeBridge-headless-installer-test-v0.40.5.zip
```

包内关键文件：

| Path | Purpose |
| --- | --- |
| `bin/SyncAgent.exe` | 后端 CLI。 |
| `config/headless-installer-test.yaml` | 无 GUI 安装测试配置。 |
| `deploy/windows/nodebridge-assets.example.json` | 离线包 catalog 模板。 |
| `scripts/headless-installer-test.ps1` | 测试 AI 主入口。 |
| `scripts/prepare-installer-assets-catalog.ps1` | 根据 `packages/` 真实离线包生成 SHA256 catalog。 |
| `runtime/headless-installer-summary.json` | 测试输出。 |

## 默认预检

默认命令不安装任何系统组件：

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\headless-installer-test.ps1
```

覆盖：

- `SyncAgent.exe` 可启动并读取配置。
- `canal-check` 配置校验通过。
- 离线包 catalog 可读取。
- 安装命令计划可输出。
- `managed-plan` 可输出受管组件计划。
- `managed-apply` 只写 manifest 和 Canal 配置。

## 真实安装测试

只有在隔离 VM 内执行：

```powershell
.\scripts\headless-installer-test.ps1 -ExecuteInstall
```

执行前必须：

1. 把真实 Erlang/RabbitMQ/Canal 离线包放到包内 `packages/`。
2. 执行 `.\scripts\prepare-installer-assets-catalog.ps1` 生成 `deploy/windows/nodebridge-assets.json`。
3. 如文件名或版本不同，通过脚本参数传入真实文件名和版本。
4. 用管理员 PowerShell 运行。

V0.40.5 可验证：

- Erlang installer 幂等执行或跳过已安装状态。
- Java/JRE zip/tar 离线资产可解压到 `%ProgramData%\NodeBridge\java`；无 Java 时默认不阻塞普通安装。
- RabbitMQ installer 幂等执行或跳过已安装状态。
- 如果不存在 RabbitMQ 服务，则注册并启动 `NodeBridgeRabbitMQ`。
- 如果已存在客户或官方安装器创建的 `RabbitMQ` 服务，则复用该服务，不停止、不删除、不重命名。
- 如果无 RabbitMQ 的干净环境由 NodeBridge 安装出默认 `RabbitMQ` 服务，则写入 `%ProgramData%\NodeBridge\managed-rabbitmq.json` 作为归属标识；只有存在该标识时，卸载才允许移除该服务。
- 同步 Administrator 与 LocalSystem 的 Erlang cookie，保证 `rabbitmqctl` 可管理当前 broker。
- NodeBridge RabbitMQ vhost/user/permission/topology 初始化；AMQP URL 使用 encoded vhost，例如 `/%2Fnodebridge-edge`。
- 每个安装步骤都会即时刷新 `runtime/headless-installer-summary.json`，人工中断或超时后也能看到最后 running/failed 的步骤。
- 每次运行开始会清理旧 `runtime/` 文件，避免二次安装证据混入上一轮残留。
- Erlang/RabbitMQ installer 如果结束后缺失 ExitCode，只有对应安装后探测通过时才判定成功，并写入 `exit_code_missing_but_detected=true` 证据。
- 每次 `rabbitmqctl` 调用前把 Erlang `bin` 加入当前进程 PATH，避免干净 VM 首次安装后 CLI 找不到 `erl.exe`。
- RabbitMQ ready 等待会输出 `rabbitmq-ready-attempts.json` 和最后一次错误，方便定位卡在服务、cookie 还是 CLI 环境。
- RabbitMQ bootstrap 先查询 vhost/user 是否存在再创建，二次安装不依赖重复创建的错误文本。
- RabbitMQ bootstrap 过程输出 `rabbitmq-bootstrap-steps.json`，方便定位失败命令。
- 卸载时等待 NodeBridge 自有服务从 SCM 消失，并输出 `NodeBridgeCanal-delete-wait.json` 等证据，避免服务删除异步延迟造成误报。
- Canal zip 解压到 `%ProgramData%\NodeBridge\canal`。
- Canal 启动脚本会被幂等清洗，删除 Java 17 不再支持的 `PermSize` / `MaxPermSize` 参数，并输出 `runtime/canal-startup-patch.json`。
- 如果 `NodeBridgeCanal` 已在运行，二次安装会复用现有 Canal 目录并跳过热覆盖，避免运行中的 jar 被锁定。
- 如果包内有 `packages/WinSW-x64.exe` 或 `packages/NodeBridgeCanal.exe` 且 `java.exe` 可用，注册并启动 `NodeBridgeCanal`。
- 如果缺 Java 且 catalog 内有 `component=java` 离线资产，安装器会先解压 Java/JRE archive，或在 MSI 场景下执行 installer，再注册 Canal Service。
- Java 探测会检查 `%ProgramData%\NodeBridge\java`、`PATH`、`JAVA_HOME`、`Program Files\Eclipse Adoptium`、`Program Files\Java` 和常见 Microsoft JRE 目录；如果仍找不到，会输出 `runtime/java-search.json`。
- 如果缺 Java 且未指定 `-RequireCanalService`，只验证 Canal 文件解压并跳过 service 注册，同时写入 `java-missing.json` / `canal-java-missing.json`。

生成带真实资产的测试包：

```powershell
.\scripts\package-headless-installer-with-assets.ps1
```

默认读取 `.cache\offline-assets\`，要求存在 Erlang、RabbitMQ、Canal 离线包；Java/JRE zip 和 WinSW 可选。存在 Java/JRE zip 时，输出包可直接用于 `-RequireCanalService` 强验。

强制 Canal Service：

```powershell
.\scripts\headless-installer-test.ps1 -ExecuteInstall -RequireCanalService
```

## 验证和卸载

验证已安装资源：

```powershell
.\scripts\headless-installer-test.ps1 -VerifyOnly
```

卸载 NodeBridge 受管资源：

```powershell
.\scripts\headless-installer-test.ps1 -Uninstall
```

删除 NodeBridge 数据目录：

```powershell
.\scripts\headless-installer-test.ps1 -Uninstall -RemoveData
```

V0.40.5 明确边界：

- 不卸载 Erlang/OTP 全局安装。
- 不卸载 RabbitMQ Server 程序目录。
- 不删除未知或客户已有的 `RabbitMQ` 服务。
- 只删除 `NodeBridgeRabbitMQ` service、带 `%ProgramData%\NodeBridge\managed-rabbitmq.json` 归属标识的 NodeBridge-created `RabbitMQ` service，以及 NodeBridge vhost/user；若复用客户已有默认 `RabbitMQ` 服务，卸载时只移除 NodeBridge 逻辑资源。
- Canal service 依赖 WinSW wrapper 和 Java；缺任一项时默认 `skipped`，不是失败。使用 `-RequireCanalService` 可把缺失依赖变成失败。

## 测试 AI 需要回传

- `runtime/headless-installer-summary.json`
- `runtime/installer-assets-check.json`
- `runtime/installer-command-plan.json`
- `runtime/managed-plan.json`
- `runtime/managed-apply.json`
- 如果 `-ExecuteInstall` 失败，回传完整 PowerShell 错误和 VM checkpoint 名称。
- Canal service 失败时，额外回传 `runtime/canal-service.json`、`runtime/canal-service-logs.json`、`runtime/canal-java*.json`。
