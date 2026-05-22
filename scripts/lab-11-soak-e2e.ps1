param(
    [switch]$SkipPrepare,
    [int]$Iterations = 3,
    [int]$CountPerEdge = 5,
    [int]$MultiCount = 10
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/lab11.ps1") -RepoRoot $root

if ($Iterations -le 0) {
    throw "Iterations must be greater than 0"
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-11-prepare.ps1")
}

$totalTimer = [System.Diagnostics.Stopwatch]::StartNew()
$rounds = New-Object System.Collections.Generic.List[object]
$maxQueueDepth = 0
$failedTotal = 0

for ($round = 1; $round -le $Iterations; $round++) {
    Write-Host "lab11 soak: round $round/$Iterations"
    $roundTimer = [System.Diagnostics.Stopwatch]::StartNew()
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-11-stress-e2e.ps1") -SkipPrepare -CountPerEdge $CountPerEdge -MultiCount $MultiCount
    $roundTimer.Stop()

    $summaryPath = Join-Path $root "build/lab-11-stress-summary.json"
    if (-not (Test-Path $summaryPath)) {
        throw "stress summary not found: $summaryPath"
    }
    $summary = Get-Content -Raw -Path $summaryPath | ConvertFrom-Json
    if ($summary.max_queue_depth -gt $maxQueueDepth) {
        $maxQueueDepth = [int]$summary.max_queue_depth
    }
    $failedTotal += [int]$summary.failed_count
    $rounds.Add([pscustomobject]@{
        round = $round
        total_events = [int]$summary.total_events
        elapsed_seconds = [Math]::Round($roundTimer.Elapsed.TotalSeconds, 2)
        throughput_per_second = [double]$summary.throughput_per_second
        max_queue_depth = [int]$summary.max_queue_depth
        failed_count = [int]$summary.failed_count
        server_apply_log_count = [int]$summary.server_apply_log_count
    }) | Out-Null
}

$totalTimer.Stop()
$totalEvents = ($rounds | Measure-Object -Property total_events -Sum).Sum
$summaryOut = [pscustomobject]@{
    iterations = $Iterations
    count_per_edge = $CountPerEdge
    multi_count = $MultiCount
    total_events = [int]$totalEvents
    elapsed_seconds = [Math]::Round($totalTimer.Elapsed.TotalSeconds, 2)
    throughput_per_second = [Math]::Round($totalEvents / [Math]::Max($totalTimer.Elapsed.TotalSeconds, 0.001), 2)
    max_queue_depth = $maxQueueDepth
    failed_count = $failedTotal
    rounds = $rounds
}

$outPath = Join-Path $root "build/lab-11-soak-summary.json"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outPath) | Out-Null
[System.IO.File]::WriteAllText($outPath, ($summaryOut | ConvertTo-Json -Depth 6), [System.Text.UTF8Encoding]::new($false))

if ($failedTotal -ne 0) {
    throw "lab11 soak failed_count=$failedTotal"
}

Write-Host "lab11 soak e2e passed"
Write-Host "iterations=$Iterations total_events=$totalEvents elapsed_seconds=$($summaryOut.elapsed_seconds) throughput_per_second=$($summaryOut.throughput_per_second) max_queue_depth=$maxQueueDepth"
Write-Host "summary=$outPath"
