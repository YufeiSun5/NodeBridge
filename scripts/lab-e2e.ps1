param(
    [switch]$SkipPrepare,
    [switch]$SkipVerify,
    [switch]$SkipReset
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

function Invoke-MySQLScalar {
    param(
        [string]$Container,
        [string]$Database,
        [string]$Query
    )
    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $docker) {
        throw "docker command not found; cannot verify MySQL rows."
    }
    docker exec $Container mysql -usync_user -psync_password -N -B $Database -e $Query
}

function Invoke-Docker {
    param([string]$Command)
    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $docker) {
        throw "docker command not found."
    }
    Invoke-Expression $Command
}

function Reset-LabState {
    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-edge-a rabbitmqctl purge_queue -p edge-a-sync $queue | Out-Null"
    }
    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-edge-b rabbitmqctl purge_queue -p edge-b-sync $queue | Out-Null"
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
    }
    Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings;`""
    Invoke-Docker "docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_apply_log; DELETE FROM device_settings;`""
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

if (-not $SkipReset -and -not $SkipVerify) {
    Reset-LabState
}

Write-Host "single-pc e2e chain starting"

# Stub CDC. / 模拟 CDC。 / CDC を模擬。
Push-Location $root
try {
    Write-Host "nodebridge> go run ./cmd/sync-agent publish-change-once"
    & go run ./cmd/sync-agent publish-change-once -config configs/lab/edge-a.local.yaml -rules configs/sync-rules.example.yaml -file sample-events/device_config.insert.change.json
    Assert-LastExit "publish-change-once"

    # Edge A -> Server. / 上传到中心。 / Server へ送信。
    Write-Host "nodebridge> go run ./cmd/sync-agent forward-upload-once"
    & go run ./cmd/sync-agent forward-upload-once -local-amqp-url amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync -server-amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync
    Assert-LastExit "forward-upload-once"

    # Server -> Edge B. / 中心下发。 / Edge B へ配信。
    Write-Host "nodebridge> go run ./cmd/sync-agent consume-once"
    & go run ./cmd/sync-agent consume-once -config configs/lab/server.local.yaml -rules configs/sync-rules.example.yaml -amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync
    Assert-LastExit "consume-once"

    # Edge B apply. / 边缘写入。 / Edge B に適用。
    Write-Host "nodebridge> go run ./cmd/sync-agent consume-downlink-once"
    & go run ./cmd/sync-agent consume-downlink-once -config configs/lab/edge-b.local.yaml -rules configs/sync-rules.example.yaml -amqp-url amqp://sync:sync_password@127.0.0.1:5675/server-sync
    Assert-LastExit "consume-downlink-once"
} finally {
    Pop-Location
}

if (-not $SkipVerify) {
    $serverValue = Invoke-MySQLScalar "nodebridge-mysql-server" "scada_center" "SELECT setting_value FROM device_settings WHERE setting_id = 1;"
    $edgeValue = Invoke-MySQLScalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT setting_value FROM device_settings WHERE setting_id = 1;"
    if ($serverValue -ne "ON") {
        throw "server verification failed: expected ON, got $serverValue"
    }
    if ($edgeValue -ne "ON") {
        throw "edge-b verification failed: expected ON, got $edgeValue"
    }
}

Write-Host "single-pc e2e passed"
