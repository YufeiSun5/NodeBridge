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

function Reset-LabState {
    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-edge-a rabbitmqctl purge_queue -p edge-a-sync $queue | Out-Null"
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
    }
    Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings; DELETE FROM alarm_history;`""
    Invoke-Docker "docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_apply_log; DELETE FROM device_settings; DELETE FROM alarm_history;`""
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

function Sync-Event {
    param(
        [string]$File,
        [bool]$ConsumeDownlink = $true
    )
    Run-NodeBridge @("publish-event", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-file", $File)
    Run-NodeBridge @("forward-upload-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-server-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
    Run-NodeBridge @("consume-once", "-config", "configs/lab/server.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
    if ($ConsumeDownlink) {
        Run-NodeBridge @("consume-downlink-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
    }
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

Write-Host "crud e2e: INSERT"
Sync-Event "sample-events/device_config.insert.sync.json" $true
Assert-Equal "server insert value" "ON" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT setting_value FROM device_settings WHERE setting_id=1;")
Assert-Equal "edge-b insert value" "ON" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT setting_value FROM device_settings WHERE setting_id=1;")
Assert-Equal "server insert apply count" "1" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='evt-device-crud-insert';")
Assert-Equal "edge-b insert apply count" "1" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='evt-device-crud-insert';")

Write-Host "crud e2e: duplicate INSERT idempotency"
Sync-Event "sample-events/device_config.insert.sync.json" $true
Assert-Equal "server duplicate apply count" "1" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='evt-device-crud-insert';")
Assert-Equal "edge-b duplicate apply count" "1" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='evt-device-crud-insert';")

Write-Host "crud e2e: UPDATE"
Sync-Event "sample-events/device_config.update.sync.json" $true
Assert-Equal "server update value" "AUTO" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT setting_value FROM device_settings WHERE setting_id=1;")
Assert-Equal "edge-b update value" "AUTO" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT setting_value FROM device_settings WHERE setting_id=1;")

Write-Host "crud e2e: DELETE soft delete"
Sync-Event "sample-events/device_config.delete.sync.json" $true
Assert-Equal "server soft delete" "1	edge-001	evt-device-crud-delete" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT is_deleted, deleted_by_node, last_event_id FROM device_settings WHERE setting_id=1;")
Assert-Equal "edge-b soft delete" "1	edge-001	evt-device-crud-delete" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT is_deleted, deleted_by_node, last_event_id FROM device_settings WHERE setting_id=1;")

Write-Host "crud e2e: EDGE_TO_SERVER alarm no dispatch"
Sync-Event "sample-events/alarm_history.insert.json" $false
Assert-Equal "server alarm count" "1" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM alarm_history WHERE id=1001 AND origin_node_id='edge-001';")
Assert-Equal "edge-b alarm count" "0" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM alarm_history WHERE id=1001;")
$downlinkDepth = Get-QueueMessages "nodebridge-rabbitmq-server" "server-sync" "edge-002.downlink.q"
Assert-Equal "edge-002 downlink depth after EDGE_TO_SERVER" "0" ([string]$downlinkDepth)

Write-Host "crud e2e passed"
