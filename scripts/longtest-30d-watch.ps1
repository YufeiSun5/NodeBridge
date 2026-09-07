param(
    [string]$RunId = "staged-v044-month30-001",
    [string]$Root = "D:\DEV_D\NodeBridge",
    [int]$PollSeconds = 60,
    [int]$TimeoutHours = 12
)

$ErrorActionPreference = "Continue"

$docker = "C:\Program Files\Docker\Docker\resources\bin\docker.exe"
if (-not (Test-Path -LiteralPath $docker)) {
    $docker = (Get-Command docker -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source)
}
if (-not $docker) {
    throw "docker CLI not found"
}

$evidence = Join-Path $Root ".cache\longtest-90d\$RunId"
$progress = Join-Path $evidence "supervisor-progress.csv"
$final = Join-Path $evidence "supervisor-final.txt"
New-Item -ItemType Directory -Force -Path $evidence | Out-Null

function Invoke-DbScalar {
    param([string]$Container, [string]$Database, [string]$Sql)
    $out = & $docker exec $Container mysql -usync_user -psync_password -N -B $Database -e $Sql 2>$null
    if ($LASTEXITCODE -ne 0) { return "" }
    return (($out | Select-Object -First 1) -as [string]).Trim()
}

function Get-QueueDepth {
    param([string]$Container, [string]$VHost, [string]$Queue)
    $rows = & $docker exec $Container rabbitmqctl -q list_queues -p $VHost name messages 2>$null
    if ($LASTEXITCODE -ne 0) { return "" }
    foreach ($row in $rows) {
        $parts = (($row -as [string]) -split "\s+")
        if ($parts.Length -ge 2 -and $parts[0] -eq $Queue) {
            return $parts[1]
        }
    }
    return "0"
}

function Get-TotalRowsSql {
    $parts = 1..6 | ForEach-Object { "(SELECT COUNT(1) FROM collect_data_{0:D2})" -f $_ }
    return "SELECT " + ($parts -join " + ") + ";"
}

if (-not (Test-Path -LiteralPath $progress)) {
    "at,edge_total,server_total,edge_collect_01,server_collect_01,edge_upload_depth,server_ingress_depth,summary_action" |
        Set-Content -Path $progress -Encoding UTF8
}

$deadline = (Get-Date).AddHours($TimeoutHours)
Write-Host "[watch] run_id=$RunId"
Write-Host "[watch] progress=$progress"

while ((Get-Date) -lt $deadline) {
    $edgeTotal = Invoke-DbScalar "nodebridge-longtest-mysql-edge" "scada_edge_longtest" (Get-TotalRowsSql)
    $serverTotal = Invoke-DbScalar "nodebridge-longtest-mysql-server" "scada_center_longtest" (Get-TotalRowsSql)
    $edge01 = Invoke-DbScalar "nodebridge-longtest-mysql-edge" "scada_edge_longtest" "SELECT COUNT(1) FROM collect_data_01;"
    $server01 = Invoke-DbScalar "nodebridge-longtest-mysql-server" "scada_center_longtest" "SELECT COUNT(1) FROM collect_data_01;"
    $edgeDepth = Get-QueueDepth "nodebridge-longtest-rabbitmq-edge" "edge-longtest-sync" "edge.upload.cdc.q"
    $serverDepth = Get-QueueDepth "nodebridge-longtest-rabbitmq-server" "server-longtest-sync" "server.cdc.ingress.q"

    $summaryAction = ""
    $summaryPath = Join-Path $evidence "summary.json"
    if (Test-Path -LiteralPath $summaryPath) {
        try {
            $summaryAction = (Get-Content -Raw -Path $summaryPath | ConvertFrom-Json).action
        } catch {
            $summaryAction = "unreadable"
        }
    }

    $at = (Get-Date).ToString("o")
    "$at,$edgeTotal,$serverTotal,$edge01,$server01,$edgeDepth,$serverDepth,$summaryAction" |
        Add-Content -Path $progress -Encoding UTF8
    Write-Host "[watch] $at edge=$edgeTotal server=$serverTotal edge01=$edge01 server01=$server01 queues=$edgeDepth/$serverDepth summary=$summaryAction"

    if ($summaryAction -eq "seed-90d") {
        "completed at $at" | Set-Content -Path $final -Encoding UTF8
        Write-Host "[watch] completed"
        Read-Host "Press Enter to close"
        exit 0
    }

    Start-Sleep -Seconds $PollSeconds
}

"timeout at $((Get-Date).ToString('o'))" | Set-Content -Path $final -Encoding UTF8
Write-Host "[watch] timeout"
Read-Host "Press Enter to close"
exit 2
