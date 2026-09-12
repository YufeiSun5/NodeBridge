param(
    [string]$Version = "0.46.16",
    [string]$DateStamp = "",
    [string]$MakensisPath = "",
    [switch]$NoBuild,
    [switch]$SkipHeadlessPackage,
    [switch]$SkipNSIS
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

function Copy-Tree {
    param(
        [string]$Source,
        [string]$Target
    )
    if (-not (Test-Path -LiteralPath $Source)) {
        throw "source directory missing: $Source"
    }
    if (Test-Path -LiteralPath $Target) {
        Remove-Item -LiteralPath $Target -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Target) | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Target -Recurse -Force
}

function Resolve-Makensis {
    if ($MakensisPath -ne "") {
        if (-not (Test-Path -LiteralPath $MakensisPath)) {
            throw "makensis not found: $MakensisPath"
        }
        return $MakensisPath
    }
    $cmd = Get-Command "makensis.exe" -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }
    $candidates = @(
        (Join-Path $env:ProgramFiles "NSIS\makensis.exe"),
        (Join-Path ${env:ProgramFiles(x86)} "NSIS\makensis.exe")
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) {
            return $candidate
        }
    }
    throw "makensis.exe not found. Install NSIS or rerun with -SkipNSIS to validate staging only."
}

if ($DateStamp -eq "") {
    $DateStamp = Get-Date -Format "yyyyMMdd"
}

$binDir = Join-Path $root "build/bin"
$nodeBridgeExe = Join-Path $binDir "NodeBridge.exe"
$syncAgentExe = Join-Path $binDir "SyncAgent.exe"
$binConfig = Join-Path $binDir "config.yaml"
$binRules = Join-Path $binDir "sync-rules.yaml"
$headlessRoot = Join-Path $root "build/headless-installer-test"
$buildRoot = Join-Path $root "build/nsis-beta"
$stagingRoot = Join-Path $buildRoot "NodeBridge-beta-v$Version-$DateStamp"
$outputExe = Join-Path $root "build/NodeBridge-beta-v$Version-$DateStamp.exe"
$summaryPath = Join-Path $buildRoot "package-nsis-beta-summary.json"

& (Join-Path $root "scripts/build-installer-art.ps1")

if (-not $NoBuild) {
    & (Join-Path $root "scripts/package-smoke.ps1") -SkipNodeBridgeLaunch -RefreshConfig
    Assert-LastExit "package-smoke"
}

if (-not $SkipHeadlessPackage) {
    $headlessArgs = @{
        Version = $Version
        NoBuild = $NoBuild
    }
    & (Join-Path $root "scripts/package-headless-installer-with-assets.ps1") @headlessArgs
    Assert-LastExit "package-headless-installer-with-assets"
}

foreach ($path in @($nodeBridgeExe, $syncAgentExe, $headlessRoot)) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "required package input missing: $path"
    }
}

if (Test-Path -LiteralPath $stagingRoot) {
    Remove-Item -LiteralPath $stagingRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $stagingRoot | Out-Null

$appDir = Join-Path $stagingRoot "app"
$docsDir = Join-Path $stagingRoot "docs"
$headlessTarget = Join-Path $stagingRoot "installer/headless"
New-Item -ItemType Directory -Force -Path $appDir | Out-Null
New-Item -ItemType Directory -Force -Path $docsDir | Out-Null

Copy-Required -Source $nodeBridgeExe -Target (Join-Path $appDir "NodeBridge.exe")
Copy-Required -Source $syncAgentExe -Target (Join-Path $appDir "SyncAgent.exe")
Copy-Required -Source (Join-Path $root "scripts/mcp-lab-client-config.ps1") -Target (Join-Path $appDir "mcp-lab-client-config.ps1")
Copy-Required -Source (Join-Path $root "build/appicon.ico") -Target (Join-Path $appDir "NodeBridge.ico")
Copy-Required -Source (Join-Path $root "configs/installer-bootstrap.yaml") -Target (Join-Path $appDir "config.yaml")
Copy-Required -Source (Join-Path $root "configs/installer-external-bootstrap.yaml") -Target (Join-Path $appDir "config-external.yaml")
Copy-Required -Source (Join-Path $root "configs/sync-rules.empty.yaml") -Target (Join-Path $appDir "sync-rules.yaml")
Copy-Tree -Source (Join-Path $root "migrations") -Target (Join-Path $appDir "migrations")

foreach ($doc in @(
    "docs/managed-components.md",
    "docs/headless-installer-test-bundle.md",
    "docs/installer-vm-test-runbook.md",
    "docs/trial-runbook.md",
    "docs/lan-deployment-guide.md",
    "docs/mcp-service.md",
    "docs/mcp-business-ai-handoff.md"
)) {
    $source = Join-Path $root $doc
    if (Test-Path -LiteralPath $source) {
        Copy-Required -Source $source -Target (Join-Path $docsDir (Split-Path -Leaf $source))
    }
}

Copy-Tree -Source $headlessRoot -Target $headlessTarget
Copy-Required -Source (Join-Path $root "scripts/headless-installer-test.ps1") -Target (Join-Path $headlessTarget "scripts/headless-installer-test.ps1")
Copy-Required -Source $syncAgentExe -Target (Join-Path $headlessTarget "bin/SyncAgent.exe")
$headlessRuntime = Join-Path $headlessTarget "runtime"
if (Test-Path -LiteralPath $headlessRuntime) {
    Remove-Item -LiteralPath $headlessRuntime -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $headlessRuntime | Out-Null

Copy-Required -Source (Join-Path $root "installer/nsis/scripts/install.ps1") -Target (Join-Path $stagingRoot "install.ps1")
Copy-Required -Source (Join-Path $root "installer/nsis/scripts/restore-ui.ps1") -Target (Join-Path $stagingRoot "restore-ui.ps1")
Copy-Required -Source (Join-Path $root "installer/nsis/scripts/uninstall.ps1") -Target (Join-Path $stagingRoot "uninstall.ps1")

$readme = @"
# NodeBridge Beta

Version: $Version
Date: $DateStamp

Run `NodeBridge-beta-v$Version-$DateStamp.exe` as administrator.

The component mode page defaults to reusing existing Docker/native/external services.
Reuse skips system installers, managed password migration, and component configuration.
For a new native environment, explicitly choose Install local system components.
Silent installation also defaults to reuse. Use /InstallSystemComponents to opt in;
/SkipSystemComponents remains supported. Do not combine both flags.
No automatic Docker discovery or connection validation is performed by this choice.

Installed app:

- `app/NodeBridge.exe`
- `app/SyncAgent.exe`

An incomplete bootstrap config with no MySQL credentials or node identity is copied to `%ProgramData%\NodeBridge\config.yaml` only when that file does not already exist.
An empty rules file is copied to `%ProgramData%\NodeBridge\sync-rules.yaml` only when that file does not already exist.

Install logs are retained after uninstall, in a unique directory per attempt:

- `%ProgramData%\NodeBridgeInstallerLogs\<run>\nsis-beta-install.log`
- `%ProgramData%\NodeBridgeInstallerLogs\<run>\nsis-beta-install-summary.json`
- `%ProgramData%\NodeBridgeInstallerLogs\<run>\components\headless-installer-summary.json`

Uninstall logs:

- `runtime\nsis-beta-uninstall.log`
- `runtime\nsis-beta-uninstall-summary.json`
- `runtime\component-headless-uninstall-summary.json`

Managed component boundary:

- Do not remove customer Erlang/OTP installs.
- Do not remove customer RabbitMQ program directories.
- Do not delete unknown or customer-owned RabbitMQ services.
- Only NodeBridge-owned RabbitMQ vhosts/users/services and NodeBridgeCanal are cleaned up.
"@
$readme | Set-Content -LiteralPath (Join-Path $stagingRoot "README.md") -Encoding UTF8

$nsiPath = Join-Path $root "installer/nsis/NodeBridgeBeta.nsi"
$nsisStatus = "skipped"
$nsisMessage = ""

if (-not $SkipNSIS) {
    $makensis = Resolve-Makensis
    if (Test-Path -LiteralPath $outputExe) {
        Remove-Item -LiteralPath $outputExe -Force
    }
    & $makensis /INPUTCHARSET UTF8 "/DVERSION=$Version" "/DSTAGING_DIR=$stagingRoot" "/DOUTPUT_EXE=$outputExe" $nsiPath
    Assert-LastExit "makensis"
    $nsisStatus = "built"
    $nsisMessage = $outputExe
} else {
    $nsisMessage = "SkipNSIS was set."
}

$summary = [ordered]@{
    created_at = (Get-Date).ToString("o")
    version = $Version
    date_stamp = $DateStamp
    staging_root = $stagingRoot
    output_exe = if (Test-Path -LiteralPath $outputExe) { $outputExe } else { "" }
    nsis_status = $nsisStatus
    nsis_message = $nsisMessage
    headless_root = $headlessTarget
}
$summary | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $summaryPath -Encoding UTF8

Write-Host "NSIS beta staging ready"
Write-Host "staging: $stagingRoot"
if (Test-Path -LiteralPath $outputExe) {
    $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $outputExe
    Write-Host "exe: $outputExe"
    Write-Host "sha256: $($hash.Hash)"
} elseif ($SkipNSIS) {
    Write-Host "exe: skipped"
} else {
    Write-Host "exe: not built"
}
Write-Host "summary: $summaryPath"
