param(
    [switch]$NoBuild,
    [switch]$SkipDataSyncLaunch,
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

function Invoke-Repo {
    param([scriptblock]$Block)
    Push-Location $root
    try {
        & $Block
    } finally {
        Pop-Location
    }
}

function Invoke-FrontendBuild {
    Push-Location (Join-Path $root "frontend")
    try {
        & ".\node_modules\.bin\tsc.cmd"
        Assert-LastExit "frontend tsc"
        & ".\node_modules\.bin\vite.cmd" "build"
        Assert-LastExit "frontend vite build"
    } finally {
        Pop-Location
    }
}

function Copy-IfMissingOrRefresh {
    param(
        [string]$Source,
        [string]$Target
    )
    if ($RefreshConfig -or -not (Test-Path -LiteralPath $Target)) {
        Copy-Item -LiteralPath $Source -Destination $Target -Force
    }
}

$binDir = Join-Path $root "build/bin"
$dataSyncExe = Join-Path $binDir "DataSync.exe"
$syncAgentExe = Join-Path $binDir "SyncAgent.exe"
$configPath = Join-Path $binDir "config.yaml"
$rulesPath = Join-Path $binDir "sync-rules.yaml"
$summaryPath = Join-Path $binDir "package-smoke-summary.json"

New-Item -ItemType Directory -Path $binDir -Force | Out-Null

if (-not $NoBuild) {
    Invoke-FrontendBuild
    Invoke-Repo {
        & go build -o $syncAgentExe .\cmd\sync-agent
        Assert-LastExit "go build SyncAgent"
        & go build -o $dataSyncExe .
        Assert-LastExit "go build DataSync"
    }
}

Copy-IfMissingOrRefresh -Source (Join-Path $root "configs/edge.example.yaml") -Target $configPath
Copy-IfMissingOrRefresh -Source (Join-Path $root "configs/sync-rules.example.yaml") -Target $rulesPath

$required = @($dataSyncExe, $syncAgentExe, $configPath, $rulesPath)
foreach ($path in $required) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "package file missing: $path"
    }
}

& $syncAgentExe "-config" $configPath
Assert-LastExit "SyncAgent ready smoke"
& $syncAgentExe "canal-check" "-config" $configPath
Assert-LastExit "SyncAgent canal-check smoke"

$dataSyncStatus = "skipped"
$dataSyncPid = 0
if (-not $SkipDataSyncLaunch) {
    $oldConfigPath = $env:NODEBRIDGE_CONFIG_PATH
    $env:NODEBRIDGE_CONFIG_PATH = $configPath
    try {
        $process = Start-Process -FilePath $dataSyncExe -WorkingDirectory $binDir -PassThru -WindowStyle Hidden
        $dataSyncPid = $process.Id
        Start-Sleep -Seconds 3
        $running = Get-Process -Id $process.Id -ErrorAction SilentlyContinue
        if (-not $running) {
            throw "DataSync exited during package smoke"
        }
        Stop-Process -Id $process.Id -Force
        $dataSyncStatus = "started"
    } finally {
        if ($null -eq $oldConfigPath) {
            Remove-Item Env:NODEBRIDGE_CONFIG_PATH -ErrorAction SilentlyContinue
        } else {
            $env:NODEBRIDGE_CONFIG_PATH = $oldConfigPath
        }
    }
}

$summary = [ordered]@{
    created_at = (Get-Date).ToString("o")
    bin_dir = $binDir
    datasync_exe = $dataSyncExe
    syncagent_exe = $syncAgentExe
    config_path = $configPath
    rules_path = $rulesPath
    datasync_launch = $dataSyncStatus
    datasync_pid = $dataSyncPid
}
$summary | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath $summaryPath -Encoding UTF8

Write-Host "package smoke passed"
Write-Host "bin: $binDir"
Write-Host "summary: $summaryPath"
