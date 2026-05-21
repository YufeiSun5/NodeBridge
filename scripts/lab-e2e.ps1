param(
    [switch]$SkipPrepare,
    [switch]$SkipVerify
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$syncAgent = "go run ./cmd/sync-agent"

$env:PATH = "$root\.vfox\sdks\golang\bin;$root\.vfox\sdks\golang\packages\bin;$root\.vfox\sdks\nodejs;$env:PATH"

function Invoke-NodeBridge {
    param([string]$Command)
    Push-Location $root
    try {
        Invoke-Expression $Command
    } finally {
        Pop-Location
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

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

# Stub CDC. / 模拟 CDC。 / CDC を模擬。
Invoke-NodeBridge "$syncAgent publish-change-once -config configs/lab/edge-a.local.yaml -rules configs/sync-rules.example.yaml -file sample-events/device_config.change.json"

# Edge A -> Server. / 上传到中心。 / Server へ送信。
Invoke-NodeBridge "$syncAgent forward-upload-once -local-amqp-url amqp://sync:sync_password@127.0.0.1:5672/edge-a-sync -server-amqp-url amqp://sync:sync_password@127.0.0.1:5672/server-sync"

# Server -> Edge B. / 中心下发。 / Edge B へ配信。
Invoke-NodeBridge "$syncAgent consume-once -config configs/lab/server.local.yaml -rules configs/sync-rules.example.yaml -amqp-url amqp://sync:sync_password@127.0.0.1:5672/server-sync -edges edge-001,edge-002"

# Edge B apply. / 边缘写入。 / Edge B に適用。
Invoke-NodeBridge "$syncAgent consume-downlink-once -config configs/lab/edge-b.local.yaml -rules configs/sync-rules.example.yaml -amqp-url amqp://sync:sync_password@127.0.0.1:5672/server-sync"

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
