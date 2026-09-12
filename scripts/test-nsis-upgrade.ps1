param(
    [string]$Version = "0.46.13",
    [string]$ProductionStaging = ""
)

$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "build-installer-art.ps1")
$repoRoot = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$cacheRoot = [System.IO.Path]::GetFullPath((Join-Path $repoRoot ".cache\nsis-upgrade"))
$testRoot = [System.IO.Path]::GetFullPath((Join-Path $cacheRoot (Get-Date -Format "yyyyMMdd-HHmmss-fff")))
if (-not $testRoot.StartsWith($cacheRoot + '\', [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "unsafe NSIS upgrade test path: $testRoot"
}
if ($ProductionStaging -eq "") {
    $ProductionStaging = Join-Path $repoRoot "build\nsis-beta\NodeBridge-beta-v$Version-20260907"
}
$ProductionStaging = [System.IO.Path]::GetFullPath($ProductionStaging)
if (-not (Test-Path -LiteralPath $ProductionStaging)) {
    throw "production staging is missing: $ProductionStaging"
}

function Copy-Required {
    param([string]$Source, [string]$Target)
    if (-not (Test-Path -LiteralPath $Source)) { throw "missing source: $Source" }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Target) | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Target -Force
}

function Assert-Equal {
    param($Actual, $Expected, [string]$Name)
    if ($Actual -ne $Expected) { throw "$Name expected=$Expected actual=$Actual" }
}

function Invoke-TestInstaller {
    param([string]$Installer, [string]$Target)
    $process = Start-Process -FilePath $Installer -ArgumentList @('/S', "/D=$Target") -WindowStyle Hidden -Wait -PassThru
    try {
        if ($null -eq $process.ExitCode -or $process.ExitCode -ne 0) {
            throw "NSIS installer failed: exit_code=$($process.ExitCode)"
        }
    } finally {
        $process.Dispose()
    }
}

$staging = Join-Path $testRoot "staging"
$installRoot = Join-Path $testRoot "installed\NodeBridge"
$programData = Join-Path $testRoot "program-data"
$testInstaller = Join-Path $testRoot "NodeBridge-upgrade-test.exe"
$evidencePath = Join-Path $testRoot "upgrade-evidence.json"
$sleeper = $null
$previousProgramData = $env:ProgramData

try {
    New-Item -ItemType Directory -Force -Path $staging,$programData | Out-Null
    foreach ($relative in @(
        "app\NodeBridge.exe",
        "app\SyncAgent.exe",
        "app\NodeBridge.ico",
        "app\config.yaml",
        "app\config-external.yaml",
        "app\sync-rules.yaml",
        "app\migrations\edge\001_mvp_tables.sql",
        "app\migrations\server\001_mvp_tables.sql",
        "installer\headless\scripts\headless-installer-test.ps1"
    )) {
        Copy-Required -Source (Join-Path $ProductionStaging $relative) -Target (Join-Path $staging $relative)
    }
    Copy-Required -Source (Join-Path $repoRoot "installer\nsis\scripts\install.ps1") -Target (Join-Path $staging "install.ps1")
    Copy-Required -Source (Join-Path $repoRoot "installer\nsis\scripts\uninstall.ps1") -Target (Join-Path $staging "uninstall.ps1")

    $makensis = @(
        (Get-Command makensis.exe -ErrorAction SilentlyContinue).Source,
        (Join-Path ${env:ProgramFiles(x86)} "NSIS\makensis.exe"),
        (Join-Path $env:ProgramFiles "NSIS\makensis.exe")
    ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -First 1
    if (-not $makensis) { throw "makensis.exe not found" }

    & $makensis /INPUTCHARSET UTF8 "/DVERSION=$Version" "/DSTAGING_DIR=$staging" "/DOUTPUT_EXE=$testInstaller" "/DUPGRADE_TEST=1" (Join-Path $repoRoot "installer\nsis\NodeBridgeBeta.nsi") | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "build upgrade test installer failed: $LASTEXITCODE" }

    $env:ProgramData = $programData
    Invoke-TestInstaller -Installer $testInstaller -Target $installRoot

    $configPath = Join-Path $programData "NodeBridge\config.yaml"
    $rulesPath = Join-Path $programData "NodeBridge\sync-rules.yaml"
    if (-not (Test-Path -LiteralPath $configPath) -or -not (Test-Path -LiteralPath $rulesPath)) {
        throw "first install did not create ProgramData configuration"
    }
    $configText = Get-Content -LiteralPath $configPath -Raw
    $configText = $configText -replace '(?m)^(\s+)name:.*$', '${1}name: preserve-upgrade'
    Set-Content -LiteralPath $configPath -Value $configText -Encoding UTF8
    if ((Get-Content -LiteralPath $configPath -Raw) -notmatch 'name:\s+preserve-upgrade') {
        throw "upgrade fixture could not set the config preservation marker"
    }
    Set-Content -LiteralPath $rulesPath -Value "rules: []`n# preserve-upgrade" -Encoding UTF8
    $rulesHashBefore = (Get-FileHash -LiteralPath $rulesPath -Algorithm SHA256).Hash
    $configHashBefore = (Get-FileHash -LiteralPath $configPath -Algorithm SHA256).Hash

    $installedAgent = Join-Path $installRoot "app\SyncAgent.exe"
    & go build -o $installedAgent (Join-Path $repoRoot "scripts\tests\upgrade-sleeper.go")
    if ($LASTEXITCODE -ne 0) { throw "build upgrade sleeper failed: $LASTEXITCODE" }
    $sleeper = Start-Process -FilePath $installedAgent -WindowStyle Hidden -PassThru
    Start-Sleep -Milliseconds 500
    if ($sleeper.HasExited) { throw "upgrade sleeper exited before second install" }

    Invoke-TestInstaller -Installer $testInstaller -Target $installRoot
    $sleeper.Refresh()
    if (-not $sleeper.HasExited) { throw "second install did not stop the old SyncAgent process" }

    $agentHash = (Get-FileHash -LiteralPath $installedAgent -Algorithm SHA256).Hash
    $expectedAgentHash = (Get-FileHash -LiteralPath (Join-Path $staging "app\SyncAgent.exe") -Algorithm SHA256).Hash
    $appHash = (Get-FileHash -LiteralPath (Join-Path $installRoot "app\NodeBridge.exe") -Algorithm SHA256).Hash
    $expectedAppHash = (Get-FileHash -LiteralPath (Join-Path $staging "app\NodeBridge.exe") -Algorithm SHA256).Hash
    Assert-Equal $agentHash $expectedAgentHash "SyncAgent overwrite"
    Assert-Equal $appHash $expectedAppHash "NodeBridge overwrite"
    $migrationHashes=@{}
    foreach($scope in @('edge','server')){
        $relative="app\migrations\$scope\001_mvp_tables.sql"
        $actual=(Get-FileHash -LiteralPath (Join-Path $installRoot $relative)).Hash
        Assert-Equal $actual (Get-FileHash -LiteralPath (Join-Path $staging $relative)).Hash "$scope packaged migration"
        $migrationHashes[$scope]=$actual
    }
    if ((Get-Content -LiteralPath $configPath -Raw) -notmatch 'name:\s+preserve-upgrade') {
        throw "second install did not preserve existing config"
    }
    Assert-Equal (Get-FileHash -LiteralPath $rulesPath -Algorithm SHA256).Hash $rulesHashBefore "rules preservation"
    Assert-Equal (Get-FileHash -LiteralPath $configPath -Algorithm SHA256).Hash $configHashBefore "config byte preservation"

    $latestSummary = Get-ChildItem -LiteralPath (Join-Path $programData "NodeBridgeInstallerLogs") -Recurse -Filter "nsis-beta-install-summary.json" |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $latestSummary) { throw "installer summary is missing" }
    $summary = Get-Content -LiteralPath $latestSummary.FullName -Raw | ConvertFrom-Json
    Assert-Equal $summary.status "passed" "second install summary"
    Assert-Equal $summary.component_mode "reuse" "upgrade component mode"
    foreach ($step in @('system-components','managed-config-migration','managed-node-configuration')) {
        Assert-Equal ($summary.steps | Where-Object name -eq $step).status 'skipped' $step
    }

    [ordered]@{
        status = "passed"
        version = $Version
        first_install = "passed"
        second_install = "passed"
        running_agent_stopped = $true
        binaries_overwritten = $true
        agent_sha256 = $agentHash
        ui_sha256 = $appHash
        migration_sha256 = $migrationHashes
        existing_config_preserved = $true
        existing_rules_preserved = $true
        install_root = $installRoot
        program_data = $programData
        summary = $latestSummary.FullName
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $evidencePath -Encoding UTF8
    Write-Host "PASS: NSIS in-place upgrade"
    Write-Host "evidence: $evidencePath"
} finally {
    $env:ProgramData = $previousProgramData
    if ($sleeper) {
        $sleeper.Refresh()
        if (-not $sleeper.HasExited) { Stop-Process -Id $sleeper.Id -Force -ErrorAction SilentlyContinue }
        $sleeper.Dispose()
    }
}
