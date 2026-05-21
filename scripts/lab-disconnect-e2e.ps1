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

function Wait-RabbitMQ {
    param([string]$Container)
    for ($i = 0; $i -lt 30; $i++) {
        docker exec $Container rabbitmq-diagnostics -q ping | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "RabbitMQ container not ready: $Container"
}

function Get-QueueMessages {
    param(
        [string]$Container,
        [string]$Vhost,
        [string]$Queue
    )
    $rows = docker exec $Container rabbitmqctl -q list_queues -p $Vhost name messages
    foreach ($row in $rows) {
        $parts = $row -split "\s+"
        if ($parts.Length -ge 2 -and $parts[0] -eq $Queue) {
            return [int]$parts[1]
        }
    }
    throw "queue not found: $Container/$Vhost/$Queue"
}

function Invoke-NodeBridgeRetry {
    param(
        [string[]]$Arguments,
        [int]$Attempts = 10
    )
    Push-Location $root
    try {
        for ($i = 1; $i -le $Attempts; $i++) {
            & go run ./cmd/sync-agent @Arguments
            if ($LASTEXITCODE -eq 0) {
                return
            }
            if ($i -eq $Attempts) {
                throw "command failed after ${Attempts} attempts: go run ./cmd/sync-agent $($Arguments -join ' ')"
            }
            Start-Sleep -Seconds 3
        }
    } finally {
        Pop-Location
    }
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

docker exec nodebridge-rabbitmq-edge-a rabbitmqctl purge_queue -p edge-a-sync edge.upload.cdc.q | Out-Null
docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync server.cdc.ingress.q | Out-Null
docker stop nodebridge-rabbitmq-server | Out-Null

Push-Location $root
try {
    Write-Host "nodebridge> publish while server broker is down"
    & go run ./cmd/sync-agent publish-change-once -config configs/lab/edge-a.local.yaml -rules configs/sync-rules.example.yaml -file sample-events/device_config.insert.change.json
    Assert-LastExit "publish-change-once"

    Write-Host "nodebridge> forward should fail while server broker is down"
    & go run ./cmd/sync-agent forward-upload-once -local-amqp-url amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync -server-amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync
    if ($LASTEXITCODE -eq 0) {
        throw "forward unexpectedly succeeded while server broker was down"
    }
} finally {
    Pop-Location
}

$edgeDepth = Get-QueueMessages "nodebridge-rabbitmq-edge-a" "edge-a-sync" "edge.upload.cdc.q"
if ($edgeDepth -ne 1) {
    throw "expected edge upload queue depth 1 while server is down, got $edgeDepth"
}

docker start nodebridge-rabbitmq-server | Out-Null
Wait-RabbitMQ "nodebridge-rabbitmq-server"
Start-Sleep -Seconds 5

Write-Host "nodebridge> init server topology after broker recovers"
Invoke-NodeBridgeRetry @("init-rabbitmq", "-mode", "server", "-edges", "edge-001,edge-002", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")

Push-Location $root
try {
    Write-Host "nodebridge> forward after server broker recovers"
    & go run ./cmd/sync-agent forward-upload-once -local-amqp-url amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync -server-amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync
    Assert-LastExit "forward-upload-once after recover"
} finally {
    Pop-Location
}

$edgeDepth = Get-QueueMessages "nodebridge-rabbitmq-edge-a" "edge-a-sync" "edge.upload.cdc.q"
$serverDepth = Get-QueueMessages "nodebridge-rabbitmq-server" "server-sync" "server.cdc.ingress.q"
if ($edgeDepth -ne 0) {
    throw "expected edge upload queue depth 0 after recover, got $edgeDepth"
}
if ($serverDepth -ne 1) {
    throw "expected server ingress queue depth 1 after recover, got $serverDepth"
}

Write-Host "disconnect e2e passed"
