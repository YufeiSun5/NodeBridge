param(
    [switch]$SkipPrepare
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/env.ps1") -RepoRoot $root
$env:PATH = "$root\.vfox\sdks\golang\bin;$root\.vfox\sdks\golang\packages\bin;$root\.vfox\sdks\nodejs;$env:PATH"
$env:PATH = "C:\Program Files\Docker\Docker\resources\bin;$env:PATH"

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Invoke-Docker {
    param([string]$Command)
    Invoke-Expression $Command
    if ($LASTEXITCODE -ne 0) {
        throw "docker command failed: $Command"
    }
}

function Invoke-Scalar {
    param(
        [string]$Container,
        [string]$Database,
        [string]$Query
    )
    docker exec $Container mysql -usync_user -psync_password -N -B $Database -e $Query
}

function Run-NodeBridge {
    param([string[]]$Arguments)
    Push-Location $root
    try {
        & go run ./cmd/sync-agent @Arguments
        Assert-LastExit "go run ./cmd/sync-agent $($Arguments -join ' ')"
    } finally {
        Pop-Location
    }
}

function Reset-LabState {
    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-edge-a rabbitmqctl purge_queue -p edge-a-sync $queue | Out-Null"
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
    }
    Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings;`""
    Invoke-Docker "docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_apply_log; DELETE FROM device_settings;`""
}

function New-BatchEventFile {
    param([int]$Index)
    $eventID = "evt-device-batch-{0:D3}" -f $Index
    $value = "VALUE-{0:D3}" -f $Index
    $event = @{
        event_id = $eventID
        event_type = "INSERT"
        origin_node_id = "edge-001"
        source_node_id = "edge-001"
        target_node_id = "server-001"
        database_name = "scada_edge"
        table_name = "device_config"
        primary_key = @{ id = $Index }
        after = @{
            id = $Index
            name = "Pump $Index"
            value = $value
            sync_version = 1
            updated_by_node = "edge-001"
            last_event_id = $eventID
            updated_at = "2026-05-21T10:00:00+08:00"
        }
        schema_version = 1
        sync_version = 1
        created_at = "2026-05-21T10:00:01+08:00"
        event_time = "2026-05-21T10:00:00+08:00"
        trace_id = "trace-device-batch-$Index"
    }
    $path = Join-Path $env:TEMP "$eventID.json"
    [System.IO.File]::WriteAllText($path, ($event | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
    return $path
}

function Assert-Equal {
    param(
        [string]$Name,
        [string]$Expected,
        [string]$Actual
    )
    if ($Actual -ne $Expected) {
        throw "$Name expected '$Expected', got '$Actual'"
    }
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

Reset-LabState

Write-Host "batch e2e: publish 50 events"
for ($i = 1; $i -le 50; $i++) {
    $file = New-BatchEventFile $i
    Run-NodeBridge @("publish-event", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-file", $file)
}

Write-Host "batch e2e: forward batch"
Run-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-server-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "50", "-flush-interval-millis", "500")

Write-Host "batch e2e: server ingress batch"
Run-NodeBridge @("consume-batch-once", "-config", "configs/lab/server.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "50", "-flush-interval-millis", "500")

Write-Host "batch e2e: edge downlink batch"
Run-NodeBridge @("consume-downlink-batch-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "50", "-flush-interval-millis", "500")

Assert-Equal "server device count" "50" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM device_settings;")
Assert-Equal "edge-b device count" "50" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM device_settings;")
Assert-Equal "server apply count" "50" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id LIKE 'evt-device-batch-%';")
Assert-Equal "edge-b apply count" "50" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id LIKE 'evt-device-batch-%';")
Assert-Equal "server last value" "VALUE-050" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT setting_value FROM device_settings WHERE setting_id=50;")
Assert-Equal "edge-b last value" "VALUE-050" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT setting_value FROM device_settings WHERE setting_id=50;")

$expectedOrder = (1..50 | ForEach-Object { "evt-device-batch-{0:D3}" -f $_ }) -join ","
$serverOrder = Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SET SESSION group_concat_max_len=10000; SELECT GROUP_CONCAT(event_id ORDER BY id SEPARATOR ',') FROM sync_apply_log WHERE event_id LIKE 'evt-device-batch-%';"
$edgeOrder = Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SET SESSION group_concat_max_len=10000; SELECT GROUP_CONCAT(event_id ORDER BY id SEPARATOR ',') FROM sync_apply_log WHERE event_id LIKE 'evt-device-batch-%';"
Assert-Equal "server apply order" $expectedOrder $serverOrder
Assert-Equal "edge-b apply order" $expectedOrder $edgeOrder

Write-Host "batch e2e passed"
