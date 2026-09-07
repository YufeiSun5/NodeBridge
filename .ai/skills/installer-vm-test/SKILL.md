---
name: installer-vm-test
description: "Use when: NodeBridge headless installer VM validation, Hyper-V installer lab, clean checkpoint restore, ExecuteInstall/VerifyOnly/Uninstall testing, Canal Service strong validation / 安装器隔离 VM 测试、干净快照恢复、无 GUI 安装闭环、Canal Service 强验 / インストーラー隔離 VM テスト、クリーンチェックポイント復元、ヘッドレス検証"
argument-hint: "version/package/board item, e.g. V0.40.0 FB-024"
---

# Installer VM Test

Use this project skill as `test-ai` whenever validating a NodeBridge headless installer package in the Hyper-V VM.

## Identity And Required Reads

1. Declare identity as `test-ai`.
2. Read `MEMORY.md`.
3. Read `AI_BOARD.md` Active Board and locate the installer test item.
4. Read `docs/installer-vm-test-runbook.md` for copy-paste host PowerShell commands.
5. If package contents or flags changed, read `docs/headless-installer-test-bundle.md`.

## Hard Boundaries

- Run real Erlang/RabbitMQ/Canal install tests only inside `NodeBridge-V034-InstallerLab-G1`.
- Do not install, stop, delete, or reconfigure Erlang/RabbitMQ/Canal on the host.
- Restore `Clean-Windows-Installed` before clean install validation unless the board explicitly asks to continue from a dirty state.
- Record the exact package version, VM checkpoint, commands, exit status, and evidence directory.

## Fixed Lab Values

| Item | Value |
| --- | --- |
| VM | `NodeBridge-V034-InstallerLab-G1` |
| Clean checkpoint | `Clean-Windows-Installed` |
| VM user | `Administrator` |
| VM password | See `docs/test-credentials.md`. |
| Host workspace | `D:\DEV_D\NodeBridge` |
| Package pattern | `build\NodeBridge-headless-installer-test-vX.Y.Z-with-assets.zip` |
| Evidence pattern | `.cache\vX.Y.Z-clean-validation\` |

## Standard Workflow

1. Set `$Version` from the open board item and confirm the package exists under `build\`.
2. Stop the VM, restore `Clean-Windows-Installed`, start the VM, and wait for PowerShell Direct.
3. Confirm the VM has no RabbitMQ, NodeBridge, or Canal services before testing.
4. Copy the with-assets package into the VM and expand it under `C:\NodeBridgeRealVXYZClean`.
5. Run the standard loop:
   - `.\scripts\headless-installer-test.ps1 -ExecuteInstall`
   - `.\scripts\headless-installer-test.ps1 -VerifyOnly`
   - `.\scripts\headless-installer-test.ps1 -ExecuteInstall`
   - `.\scripts\headless-installer-test.ps1 -Uninstall`
6. After uninstall, confirm no RabbitMQ, NodeBridge, or Canal services remain.
7. Collect `runtime\*`, service/process snapshots, and summary JSON into `.cache\vX.Y.Z-clean-validation\`.
8. Update `AI_BOARD.md` with pass/fail, evidence path, VM checkpoint, and whether host components were untouched.
9. Update `MEMORY.md` after meaningful validation.

## Canal Service Strong Validation

Only use `-RequireCanalService` when the package includes Java/JRE and the asset catalog declares `component=java`.

For V0.40.0 and later Java-enabled packages, run:

```powershell
.\scripts\headless-installer-test.ps1 -ExecuteInstall -RequireCanalService
.\scripts\headless-installer-test.ps1 -VerifyOnly
.\scripts\headless-installer-test.ps1 -ExecuteInstall -RequireCanalService
.\scripts\headless-installer-test.ps1 -Uninstall
```

If Java is absent, default Canal Service skip is expected; do not mark that as a failure unless the board requested strong validation.

## Board Result Format

Pass:

```text
Vx.y.z 已在 `Clean-Windows-Installed` 干净 VM 通过闭环：第一轮 `-ExecuteInstall` 通过，`-VerifyOnly` 通过，二次 `-ExecuteInstall` 通过，`-Uninstall` 通过且 `verify-uninstall` passed；卸载后确认无 RabbitMQ/NodeBridge/Canal 服务。证据在 `.cache/vx.y.z-clean-validation/`。宿主机 Erlang/RabbitMQ/Canal 未触碰。
```

Fail:

```text
Vx.y.z 在 `Clean-Windows-Installed` 干净 VM 阻塞：失败命令 `<command>`，失败步骤 `<step>`，原始错误 `<error>`。证据在 `.cache/vx.y.z-clean-validation/`。宿主机 Erlang/RabbitMQ/Canal 未触碰。
```
