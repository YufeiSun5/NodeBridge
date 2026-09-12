# NodeBridge NSIS Beta Installer

This folder contains the NSIS wrapper for the beta installer. It packages:

- `app/NodeBridge.exe` and `app/SyncAgent.exe`.
- incomplete bootstrap `config.yaml` and empty `sync-rules.yaml` for new installations.
- the current headless installer bundle under `installer/headless`.
- `install.ps1` and `uninstall.ps1` wrappers for logs, summaries, and managed-resource cleanup.

Build from the repository root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package-nsis-beta.ps1
```

If `makensis.exe` is not installed, run with `-SkipNSIS` to validate the staging tree only:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package-nsis-beta.ps1 -SkipNSIS
```

Real Erlang/RabbitMQ/Java/Canal component installation must still be tested only inside the isolated installer VM.

## v0.46.15 Component Mode

The Chinese/English/Japanese wizard now asks for a component installation mode. **Reuse existing components** is selected by default, including silent installs with no mode flag. Choose this for Docker, external services, or an existing working deployment. It skips system installers, managed password migration, and component topology/configuration. Existing config and rules are preserved byte-for-byte; an absent config is created from the external bootstrap. It does not auto-detect Docker or verify connections.

Select **Install local system components** only for an explicitly intended native component installation. Unattended callers must now pass `/InstallSystemComponents`; `/SkipSystemComponents` remains supported. Both flags together fail before installation. Direct `install.ps1` callers likewise default to reuse and must explicitly pass `-InstallSystemComponents` for component changes. This is an intentional safer default, not automatic environment discovery.

Fresh configs still use the trial admin/exit password `1234`. Reuse upgrades preserve existing passwords and do not run the password migration described in the older compatibility section below. Existing managed configurations are not silently converted to external; the selected mode governs this installer run only. No database migration or synchronization start is implied.

`scripts/test-installer-component-mode.ps1` executes the wrapper against fixture-only binaries and component scripts for default/explicit reuse, explicit installation, fresh external bootstrap, and mutually exclusive flags. No real system components are installed. `test-nsis-upgrade.ps1` also checks exact configuration hashes and skipped component steps.

## v0.46.13 Upgrade UI

The installer still uses NSIS MUI2 with the existing component and uninstall paths. Welcome/finish pages, branded bitmaps, DPI awareness, and Chinese/English/Japanese page text are added; compile UTF-8 sources with `/INPUTCHARSET UTF8`. `scripts/build-installer-art.ps1` derives the installer bitmaps from the existing app icon.

Preflight records original UI session IDs and owner SIDs in the private NSIS plugin directory. After a successful install, `restore-ui.ps1` launches the installed UI using a temporary least-privilege InteractiveToken task only when the original user's desktop is unambiguous. It never launches into Session 0, avoids an existing UI, and removes the temporary task even after failure. A missing/ambiguous desktop or launch failure is retained as a `restore-ui` warning in the installer summary; installation is not rolled back. New installs do not auto-launch, and `UPGRADE_TEST` disables restoration. Autostart settings are unchanged.

`scripts/test-ui-restore.ps1` covers selectors and mocked task dispatch/cleanup. Actual interactive desktop restoration still requires installation retesting; these mocks are not a desktop acceptance result.

## v0.46.6 Compatibility

Normal worker idle polling no longer inherits `sync.retry_interval_seconds`; it uses a 100ms interval while actual errors keep the configured retry backoff. Canal long polling is also capped at 100ms to keep idle single-row synchronization responsive on a LAN.

Fresh installations use `1234` for the NodeBridge admin unlock and exit passwords and do not preset MySQL credentials, a database, node identity, or sync rules. Upgrades preserve existing configuration and rules while migrating NodeBridge-owned admin, exit, and managed RabbitMQ passwords to `1234`. MySQL and Canal credentials are never changed.

Managed RabbitMQ usernames are generated from `node.id`; the installer no longer provisions `edge-001` for every machine. The migration changes both the encrypted NodeBridge configuration and the RabbitMQ service user before topology initialization.

Before copying files during an in-place upgrade, NSIS runs `upgrade-preflight.ps1`. It requests a graceful SyncAgent stop and then terminates only `NodeBridge.exe`, legacy `DataSync.exe`, or `SyncAgent.exe` processes whose executable path is inside the selected install directory. Existing ProgramData configuration and rules are retained.

Run `scripts/test-nsis-upgrade.ps1` to compile a non-administrative test variant and perform two real NSIS installs into an isolated workspace directory. The second pass holds an old `SyncAgent.exe` open and verifies process shutdown, binary replacement, config preservation, rules preservation, and successful installer summaries.

The UI connection tests now resolve the redacted `******` placeholder to the saved secret. Lab MCP config saves accept optional `restart_agent:true` to apply the saved configuration by restarting SyncAgent; omitting it preserves the old behavior.

## v0.46.2 Installer Fixes

NSIS launches native 64-bit Windows PowerShell through `Sysnative`. Component detection also checks `ProgramW6432` when invoked from a 32-bit shell. Native installer calls retain the process handle, quote arguments, and verify component detection for up to 300 seconds after a successful or missing exit code. A nonzero failure exit code is not overridden by detection.

Each installation writes diagnostics to `%ProgramData%\NodeBridgeInstallerLogs\<run>\`, outside the application and component data directories. This includes the transcript, wrapper summary, and `components\process-install-erlang.json`; uninstall leaves these installation diagnostics intact.

The installer grants the Windows account used for installation inherited `Modify` access to `%ProgramData%\NodeBridge`. Atomic configuration updates create a temporary file and replace the old file, which requires delete-child rights in addition to ordinary write access.

Run `scripts/test-installer-regression.ps1` with both System32 and SysWOW64 Windows PowerShell to cover directory detection, native argument quoting, exit codes, timeout, probes, and retained failure summaries. These tests do not install system components.
