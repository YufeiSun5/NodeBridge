param(
    [string]$Version = "0.46.15",
    [switch]$NoBuild,
    [switch]$NoZip,
    [switch]$RefreshConfig
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/env.ps1") -RepoRoot $root

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Copy-Required {
    param(
        [string]$Source,
        [string]$Target
    )
    if (-not (Test-Path -LiteralPath $Source)) {
        throw "source missing: $Source"
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Target) | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Target -Force
}

function Copy-IfMissingOrRefresh {
    param(
        [string]$Source,
        [string]$Target
    )
    if ($RefreshConfig -or -not (Test-Path -LiteralPath $Target)) {
        Copy-Required -Source $Source -Target $Target
    }
}

$bundleRoot = Join-Path $root "build/headless-installer-test"
$zipPath = Join-Path $root "build/NodeBridge-headless-installer-test-v$version.zip"
$syncAgentSource = Join-Path $root "build/bin/SyncAgent.exe"
$syncAgentTarget = Join-Path $bundleRoot "bin/SyncAgent.exe"
$summaryPath = Join-Path $bundleRoot "package-summary.json"

if (Test-Path -LiteralPath $bundleRoot) {
    Remove-Item -LiteralPath $bundleRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $bundleRoot | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $bundleRoot "packages") | Out-Null

if (-not $NoBuild) {
    & go build -o $syncAgentSource .\cmd\sync-agent
    Assert-LastExit "go build SyncAgent"
}

Copy-Required -Source $syncAgentSource -Target $syncAgentTarget
Copy-IfMissingOrRefresh -Source (Join-Path $root "configs/headless-installer-test.yaml") -Target (Join-Path $bundleRoot "config/headless-installer-test.yaml")
Copy-Required -Source (Join-Path $root "configs/sync-rules.example.yaml") -Target (Join-Path $bundleRoot "config/sync-rules.yaml")
Copy-Required -Source (Join-Path $root "deploy/windows/nodebridge-assets.example.json") -Target (Join-Path $bundleRoot "deploy/windows/nodebridge-assets.example.json")
Copy-Required -Source (Join-Path $root "scripts/headless-installer-test.ps1") -Target (Join-Path $bundleRoot "scripts/headless-installer-test.ps1")
Copy-Required -Source (Join-Path $root "scripts/prepare-installer-assets-catalog.ps1") -Target (Join-Path $bundleRoot "scripts/prepare-installer-assets-catalog.ps1")
Copy-Required -Source (Join-Path $root "scripts/package-headless-installer-with-assets.ps1") -Target (Join-Path $bundleRoot "scripts/package-headless-installer-with-assets.ps1")
Copy-Required -Source (Join-Path $root "docs/v0.33-installer-assets.md") -Target (Join-Path $bundleRoot "docs/v0.33-installer-assets.md")
Copy-Required -Source (Join-Path $root "docs/v0.34-installer-vm-lab.md") -Target (Join-Path $bundleRoot "docs/v0.34-installer-vm-lab.md")
Copy-Required -Source (Join-Path $root "docs/v0.35-installer-closure.md") -Target (Join-Path $bundleRoot "docs/v0.35-installer-closure.md")
Copy-Required -Source (Join-Path $root "docs/v0.36-installer-real-closure.md") -Target (Join-Path $bundleRoot "docs/v0.36-installer-real-closure.md")
Copy-Required -Source (Join-Path $root "docs/headless-installer-test-bundle.md") -Target (Join-Path $bundleRoot "docs/headless-installer-test-bundle.md")
Copy-Required -Source (Join-Path $root "docs/managed-components.md") -Target (Join-Path $bundleRoot "docs/managed-components.md")

$readme = @"
# NodeBridge Headless Installer Test Bundle

Version: $version

This bundle is for Test AI on Windows Server 2022 Core or no-GUI VM.

## Default Safe Test

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\headless-installer-test.ps1
```

This validates:

- SyncAgent CLI starts.
- Canal config validates.
- Offline asset catalog can be read.
- Installer command plan can be printed.
- Managed manifest and Canal config can be written.

It does not install Erlang/RabbitMQ/Canal.

## Real VM Install Test

1. Put real offline packages under `packages/`.
2. Copy `deploy/windows/nodebridge-assets.example.json` to `deploy/windows/nodebridge-assets.json`.
3. Replace every `path`, `version`, and `sha256`.
4. Run from an elevated PowerShell session inside the VM:

```powershell
.\scripts\headless-installer-test.ps1 -ExecuteInstall
```

Generate the real catalog:

```powershell
.\scripts\prepare-installer-assets-catalog.ps1
```

V0.40.5 can execute Erlang/RabbitMQ/Java installer commands idempotently, reuse an existing `RabbitMQ` service without deleting it, mark only NodeBridge-created RabbitMQ services for uninstall, initialize NodeBridge vhost/user/topology with encoded AMQP vhosts, tolerate missing installer exit codes when post-install probes pass, sync Erlang into the current process PATH before RabbitMQ CLI calls, clear stale runtime evidence at the start of each run, extract Canal, patch obsolete `PermSize` JVM options from Canal startup scripts for Java 17 compatibility, skip Canal hot overwrite while `NodeBridgeCanal` is running, wait for NodeBridge service deletion during uninstall, and register `NodeBridgeCanal` only when WinSW and Java are available. Java archive assets are extracted to `%ProgramData%\NodeBridge\java`, avoiding MSI on Server Core. The script writes `runtime/headless-installer-summary.json` after each step so interrupted installs still show the last running step.

Optional Canal service wrapper:

- Put `WinSW-x64.exe` or `NodeBridgeCanal.exe` under `packages/`.
- Install Java, or put a Java/JRE offline installer in `packages/` and add a `java` asset to the catalog.
- Use `-RequireCanalService` if missing wrapper or Java should fail the test.

Uninstall verification:

```powershell
.\scripts\headless-installer-test.ps1 -Uninstall
```

## Output

Runtime files are written to `runtime/`.
Main result: `runtime/headless-installer-summary.json`.
"@
$readme | Set-Content -LiteralPath (Join-Path $bundleRoot "README.md") -Encoding UTF8

& $syncAgentTarget "-config" (Join-Path $bundleRoot "config/headless-installer-test.yaml") | Out-Null
Assert-LastExit "bundle SyncAgent ready"
& $syncAgentTarget "installer-command-plan" "-catalog" (Join-Path $bundleRoot "deploy/windows/nodebridge-assets.example.json") | Out-Null
Assert-LastExit "bundle installer-command-plan"
& $syncAgentTarget "installer-assets-check" "-catalog" (Join-Path $bundleRoot "deploy/windows/nodebridge-assets.example.json") "-strict=false" | Out-Null
Assert-LastExit "bundle installer-assets-check"

$summary = [ordered]@{
    created_at = (Get-Date).ToString("o")
    version = $version
    bundle_root = $bundleRoot
    zip_path = if ($NoZip) { "" } else { $zipPath }
    sync_agent = $syncAgentTarget
    config = Join-Path $bundleRoot "config/headless-installer-test.yaml"
    catalog = Join-Path $bundleRoot "deploy/windows/nodebridge-assets.example.json"
    test_script = Join-Path $bundleRoot "scripts/headless-installer-test.ps1"
}
$summary | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $summaryPath -Encoding UTF8

if (-not $NoZip) {
    if (Test-Path -LiteralPath $zipPath) {
        Remove-Item -LiteralPath $zipPath -Force
    }
    Compress-Archive -Path (Join-Path $bundleRoot "*") -DestinationPath $zipPath -Force
}

Write-Host "headless installer test bundle ready"
Write-Host "bundle: $bundleRoot"
if (-not $NoZip) {
    Write-Host "zip: $zipPath"
}
Write-Host "summary: $summaryPath"
