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

$runner = Join-Path $Root "scripts\run-longtest-30d-rerun.ps1"
$evidence = Join-Path $Root ".cache\longtest-90d\$RunId"
New-Item -ItemType Directory -Force -Path $evidence | Out-Null

$outLog = Join-Path $evidence "runner.out.log"
$errLog = Join-Path $evidence "runner.err.log"
$startLog = Join-Path $evidence "runner-start.json"

$args = @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-File", $runner,
    "-RunId", $RunId,
    "-Root", $Root,
    "-Days", "$Days",
    "-BatchSize", "$BatchSize",
    "-DrainIterations", "$DrainIterations",
    "-DrainMode", $DrainMode,
    "-AgentPollSeconds", "$AgentPollSeconds",
    "-AgentStallMinutes", "$AgentStallMinutes"
)

$process = Start-Process -FilePath "powershell.exe" -ArgumentList $args -RedirectStandardOutput $outLog -RedirectStandardError $errLog -PassThru -WindowStyle Hidden
[IO.File]::WriteAllText(
    $startLog,
    ([pscustomobject]@{
        run_id = $RunId
        pid = $process.Id
        started_at = (Get-Date).ToString("o")
        evidence = $evidence
        stdout = $outLog
        stderr = $errLog
        days = $Days
        batch_size = $BatchSize
        drain_iterations = $DrainIterations
        drain_mode = $DrainMode
        agent_poll_seconds = $AgentPollSeconds
        agent_stall_minutes = $AgentStallMinutes
    } | ConvertTo-Json -Depth 5),
    [Text.UTF8Encoding]::new($false)
)

Write-Host "run_id=$RunId"
Write-Host "pid=$($process.Id)"
Write-Host "evidence=$evidence"
Write-Host "stdout=$outLog"
Write-Host "stderr=$errLog"
