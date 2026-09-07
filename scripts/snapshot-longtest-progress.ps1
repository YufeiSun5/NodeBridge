param(
    [string]$RunId = "staged-v044-month30-agents-001",
    [int64]$ExpectedRows = 5184000,
    [int]$IntervalSeconds = 300,
    [int]$MaxSamples = 288
)

$ErrorActionPreference = "Stop"

$docker = Get-Command docker -ErrorAction SilentlyContinue
if ($docker) {
    $dockerPath = $docker.Source
} elseif (Test-Path "C:\Program Files\Docker\Docker\resources\bin\docker.exe") {
    $dockerPath = "C:\Program Files\Docker\Docker\resources\bin\docker.exe"
} else {
    throw "docker CLI not found"
}

$repo = Split-Path -Parent $PSScriptRoot
$evidence = Join-Path $repo ".cache\longtest-90d\$RunId"
New-Item -ItemType Directory -Force -Path $evidence | Out-Null
$csv = Join-Path $evidence "batchconfirm-progress-snapshots.csv"
$summary = Join-Path $evidence "batchconfirm-progress-summary.json"
"at,sample,edge_rows,server_rows,remaining_rows,edge_upload_depth,server_ingress_depth,edge_pid_alive,server_pid_alive" | Set-Content -Path $csv -Encoding UTF8

function Get-RowSum {
    param([string]$Container, [string]$Database)
    $sql = "SELECT (SELECT COUNT(*) FROM collect_data_01)+(SELECT COUNT(*) FROM collect_data_02)+(SELECT COUNT(*) FROM collect_data_03)+(SELECT COUNT(*) FROM collect_data_04)+(SELECT COUNT(*) FROM collect_data_05)+(SELECT COUNT(*) FROM collect_data_06);"
    $value = & $dockerPath exec $Container mysql -uroot -proot_password -N -B $Database -e $sql 2>$null | Select-Object -First 1
    return [int64]$value
}

function Get-QueueDepth {
    param([string]$Container, [string]$VHost, [string]$Queue)
    $lines = & $dockerPath exec $Container rabbitmqctl -q list_queues -p $VHost name messages 2>$null
    foreach ($line in $lines) {
        if ($line -match "^$([regex]::Escape($Queue))\s+(\d+)") {
            return [int64]$Matches[1]
        }
    }
    return 0
}

$startedAt = Get-Date
$last = $null
$completed = $false
for ($i = 1; $i -le $MaxSamples; $i++) {
    $edgeRows = Get-RowSum "nodebridge-longtest-mysql-edge" "scada_edge_longtest"
    $serverRows = Get-RowSum "nodebridge-longtest-mysql-server" "scada_center_longtest"
    $edgeDepth = Get-QueueDepth "nodebridge-longtest-rabbitmq-edge" "edge-longtest-sync" "edge.upload.cdc.q"
    $serverDepth = Get-QueueDepth "nodebridge-longtest-rabbitmq-server" "server-longtest-sync" "server.cdc.ingress.q"
    $edgeAlive = [bool](Get-Process -Id 37132 -ErrorAction SilentlyContinue)
    $serverAlive = [bool](Get-Process -Id 7072 -ErrorAction SilentlyContinue)
    $remaining = [Math]::Max(0, $ExpectedRows - $serverRows)
    $line = "{0},{1},{2},{3},{4},{5},{6},{7},{8}" -f (Get-Date).ToString("o"), $i, $edgeRows, $serverRows, $remaining, $edgeDepth, $serverDepth, $edgeAlive, $serverAlive
    Add-Content -Path $csv -Value $line -Encoding UTF8
    $last = [pscustomobject]@{
        at = (Get-Date).ToString("o")
        sample = $i
        edge_rows = $edgeRows
        server_rows = $serverRows
        remaining_rows = $remaining
        edge_upload_depth = $edgeDepth
        server_ingress_depth = $serverDepth
        edge_pid_alive = $edgeAlive
        server_pid_alive = $serverAlive
    }
    $completed = ($serverRows -ge $ExpectedRows -and $edgeDepth -eq 0 -and $serverDepth -eq 0)
    if ($completed -or -not ($edgeAlive -and $serverAlive)) {
        break
    }
    Start-Sleep -Seconds $IntervalSeconds
}

$endedAt = Get-Date
[pscustomobject]@{
    run_id = $RunId
    expected_rows = $ExpectedRows
    completed = $completed
    started_at = $startedAt.ToString("o")
    ended_at = $endedAt.ToString("o")
    elapsed_seconds = [Math]::Round(($endedAt - $startedAt).TotalSeconds, 3)
    last_sample = $last
    csv = $csv
} | ConvertTo-Json -Depth 5 | Set-Content -Path $summary -Encoding UTF8

if (-not $completed) {
    exit 2
}
