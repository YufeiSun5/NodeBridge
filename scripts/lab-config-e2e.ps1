param(
    [switch]$SkipPrepare
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$env:PATH = "$root\.vfox\sdks\golang\bin;$root\.vfox\sdks\golang\packages\bin;$root\.vfox\sdks\nodejs;$env:PATH"
$env:PATH = "C:\Program Files\Docker\Docker\resources\bin;$env:PATH"

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Stop-NodeApiPort {
    $connections = Get-NetTCPConnection -LocalPort 18090 -ErrorAction SilentlyContinue
    foreach ($connection in $connections) {
        if ($connection.OwningProcess -and $connection.OwningProcess -gt 0) {
            Stop-Process -Id $connection.OwningProcess -Force -ErrorAction SilentlyContinue
        }
    }
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

Stop-NodeApiPort
docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync edge-002.downlink.q | Out-Null
docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e "DELETE FROM sync_node_config WHERE node_id='edge-002';" | Out-Null

$out = Join-Path $env:TEMP "nodebridge-node-api.out.log"
$err = Join-Path $env:TEMP "nodebridge-node-api.err.log"
$process = Start-Process -FilePath "powershell" -ArgumentList @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-Command",
    "Set-Location '$root'; `$env:PATH='$env:PATH'; go run ./cmd/sync-agent serve-node-api -config configs/lab/server.local.yaml -bind 127.0.0.1 -port 18090"
) -RedirectStandardOutput $out -RedirectStandardError $err -PassThru -WindowStyle Hidden

try {
    for ($i = 0; $i -lt 30; $i++) {
        try {
            Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18090/api/nodes" | Out-Null
            break
        } catch {
            Start-Sleep -Seconds 1
        }
    }

    $registerBody = @{
        node_id = "edge-002"
        node_name = "Lab Edge B"
        location = "single-pc-lab"
        version = "0.17.0"
    } | ConvertTo-Json
    Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18090/api/nodes/register" -ContentType "application/json" -Body $registerBody | Out-Null

    $configBody = @{
        mysql_host = "127.0.0.1"
        mysql_port = 3308
        mysql_database = "scada_edge"
        mysql_username = "sync_user"
        cdc_type = "canal"
        cdc_filter = "scada_edge\..*"
        cdc_batch_size = 1000
        cdc_destination = "edge-002"
        rule_version = 17
    } | ConvertTo-Json
    Invoke-RestMethod -Method Put -Uri "http://127.0.0.1:18090/api/nodes/edge-002/config" -ContentType "application/json" -Body $configBody | Out-Null

    Push-Location $root
    try {
        & go run ./cmd/sync-agent consume-downlink-once -config configs/lab/edge-b.local.yaml -rules configs/sync-rules.example.yaml -amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync
        Assert-LastExit "consume-downlink-once config"
    } finally {
        Pop-Location
    }

    $ruleVersion = docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password -N -B scada_edge -e "SELECT rule_version FROM sync_node_config WHERE node_id='edge-002';"
    if ($ruleVersion -ne "17") {
        throw "expected applied rule_version 17, got $ruleVersion"
    }
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
    }
    Stop-NodeApiPort
}

Write-Host "config e2e passed"
