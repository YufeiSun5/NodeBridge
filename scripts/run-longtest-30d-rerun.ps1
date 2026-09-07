param(
    [string]$RunId = "staged-v044-month30-rerun-001",
    [string]$Root = "D:\DEV_D\NodeBridge",
    [int]$Days = 30,
    [int]$BatchSize = 10000,
    [int]$DrainIterations = 7200,
    [ValidateSet("once", "agents")]
    [string]$DrainMode = "agents",
    [int]$AgentPollSeconds = 30,
    [int]$AgentStallMinutes = 30
)

$ErrorActionPreference = "Stop"

$docker = "C:\Program Files\Docker\Docker\resources\bin\docker.exe"
if (-not (Test-Path -LiteralPath $docker)) {
    $docker = (Get-Command docker -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source)
}
if (-not $docker) {
    throw "docker CLI not found"
}

$composeFile = Join-Path $Root "deploy\docker-compose.longtest.yml"
$longtest = Join-Path $Root "scripts\longtest-90d.ps1"
$evidence = Join-Path $Root ".cache\longtest-90d\$RunId"
$runnerSummary = Join-Path $evidence "runner-summary.json"
New-Item -ItemType Directory -Force -Path $evidence | Out-Null

function Write-RunnerSummary {
    param([string]$Status, [string]$ErrorMessage = "")
    [IO.File]::WriteAllText(
        $runnerSummary,
        ([pscustomobject]@{
            run_id = $RunId
            status = $Status
            days = $Days
            batch_size = $BatchSize
            drain_iterations = $DrainIterations
            drain_mode = $DrainMode
            agent_poll_seconds = $AgentPollSeconds
            agent_stall_minutes = $AgentStallMinutes
            completed_at = (Get-Date).ToString("o")
            error = $ErrorMessage
        } | ConvertTo-Json -Depth 5),
        [Text.UTF8Encoding]::new($false)
    )
}

Set-Location $Root
try {
    Write-Host "[runner] run_id=$RunId"
    Write-Host "[runner] evidence=$evidence"
    Write-Host "[runner] docker compose down -v"
    & $docker compose -f $composeFile down -v --remove-orphans
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose down failed with exit code $LASTEXITCODE"
    }

    Write-Host "[runner] prepare"
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $longtest -Action prepare -RunId $RunId
    if ($LASTEXITCODE -ne 0) {
        throw "prepare failed with exit code $LASTEXITCODE"
    }

    Write-Host "[runner] seed-90d days=$Days batch=$BatchSize drain_mode=$DrainMode drain_iterations=$DrainIterations"
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $longtest -Action seed-90d -ConfirmLongRun -RunId $RunId -Days $Days -BatchSize $BatchSize -DrainIterations $DrainIterations -DrainMode $DrainMode -AgentPollSeconds $AgentPollSeconds -AgentStallMinutes $AgentStallMinutes
    if ($LASTEXITCODE -ne 0) {
        throw "seed-90d failed with exit code $LASTEXITCODE"
    }

    Write-RunnerSummary -Status "passed"
    Write-Host "[runner] passed"
    exit 0
} catch {
    Write-RunnerSummary -Status "failed" -ErrorMessage $_.Exception.Message
    Write-Host "[runner] failed: $($_.Exception.Message)"
    exit 1
}
