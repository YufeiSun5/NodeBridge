param(
    [switch]$SkipPrepare
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/env.ps1") -RepoRoot $root

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Invoke-Docker {
    param([string]$Command)
    Invoke-Expression $Command
    Assert-LastExit $Command
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

function Invoke-NodeBridge {
    param([string[]]$Arguments)
    Push-Location $root
    try {
        & go run ./cmd/sync-agent @Arguments
        Assert-LastExit "go run ./cmd/sync-agent $($Arguments -join ' ')"
    } finally {
        Pop-Location
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

function Reset-RetryLab {
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
    }
    Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings WHERE setting_id=3030;`""
    Invoke-Docker "docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_apply_log; DELETE FROM device_settings WHERE setting_id=3030;`""
}

function New-RetryEventFile {
    $path = Join-Path $env:TEMP "nodebridge-retry-event.json"
    $json = @"
{
  "event_id": "evt-retry-e2e",
  "event_type": "INSERT",
  "origin_node_id": "edge-001",
  "source_node_id": "edge-001",
  "target_node_id": "server-001",
  "database_name": "scada_edge",
  "table_name": "device_config",
  "primary_key": { "id": 3030 },
  "after": {
    "id": 3030,
    "name": "Retry Device",
    "value": "RETRY-ON",
    "sync_version": 1,
    "updated_by_node": "edge-001",
    "last_event_id": "evt-retry-e2e",
    "updated_at": "2026-05-22T11:00:00+08:00"
  },
  "schema_version": 1,
  "sync_version": 1,
  "created_at": "2026-05-22T11:00:01+08:00",
  "event_time": "2026-05-22T11:00:00+08:00",
  "trace_id": "trace-retry-e2e"
}
"@
    [System.IO.File]::WriteAllText($path, $json, [System.Text.UTF8Encoding]::new($false))
    return $path
}

function Seed-FailedRetry {
    param([string]$EventFile)
    $payload = (Get-Content -LiteralPath $EventFile -Raw).Replace("'", "''")
    $sqlPath = Join-Path $env:TEMP "nodebridge-retry-seed.sql"
    $sql = @"
INSERT INTO sync_event_log (
  event_id, origin_node_id, source_node_id, database_name, table_name,
  target_database_name, target_table_name, pk_value, op_type, direction,
  status, event_time, received_at, applied_at, error_message, event_payload
) VALUES (
  'evt-retry-e2e', 'edge-001', 'edge-001', 'scada_edge', 'device_config',
  'scada_center', 'device_settings', 'id=3030', 'INSERT', 'BIDIRECTIONAL',
  'SUCCESS', NOW(3), NOW(3), NOW(3), NULL, '$payload'
)
ON DUPLICATE KEY UPDATE status='SUCCESS', event_payload=VALUES(event_payload);
INSERT INTO sync_ack_log (event_id, target_node_id, status, ack_at, error_message, created_at)
VALUES ('evt-retry-e2e', 'edge-002', 'FAILED', NULL, 'simulated dispatch failure', NOW(3))
ON DUPLICATE KEY UPDATE status='FAILED', ack_at=NULL, error_message='simulated dispatch failure';
"@
    [System.IO.File]::WriteAllText($sqlPath, $sql, [System.Text.UTF8Encoding]::new($false))
    docker cp $sqlPath nodebridge-mysql-server:/tmp/nodebridge-retry-seed.sql
    Assert-LastExit "docker cp retry seed sql"
    docker exec nodebridge-mysql-server sh -c "mysql -usync_user -psync_password scada_center < /tmp/nodebridge-retry-seed.sql"
    Assert-LastExit "mysql retry seed sql"
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

Reset-RetryLab
$eventFile = New-RetryEventFile
Seed-FailedRetry $eventFile

Write-Host "retry e2e: failed list"
$failed = Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED' AND event_id='evt-retry-e2e';"
Assert-Equal "seed failed count" "1" $failed
Invoke-NodeBridge @("failed-events", "-config", "configs/lab/server.local.yaml", "-limit", "10")

Write-Host "retry e2e: batch mark pending and replay"
Invoke-NodeBridge @("retry-failed-batch", "-config", "configs/lab/server.local.yaml", "-limit", "10")
Assert-Equal "pending count" "1" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_ack_log WHERE status='PENDING' AND event_id='evt-retry-e2e';")
Invoke-NodeBridge @("replay-pending-once", "-config", "configs/lab/server.local.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
Assert-Equal "downlink queue count" "1" "$(Get-QueueMessages -Container "nodebridge-rabbitmq-server" -Vhost "server-sync" -Queue "edge-002.downlink.q")"
Invoke-NodeBridge @("consume-downlink-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")

Assert-Equal "edge-b retry value" "RETRY-ON" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT setting_value FROM device_settings WHERE setting_id=3030;")
Assert-Equal "retry ack success" "SUCCESS" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT status FROM sync_ack_log WHERE event_id='evt-retry-e2e' AND target_node_id='edge-002';")

Write-Host "retry e2e: dead-letter preview"
Invoke-NodeBridge @("publish-event", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-exchange", "server.dead.x", "-routing-key", "server.dead", "-file", $eventFile)
Assert-Equal "dead queue before preview" "1" "$(Get-QueueMessages -Container "nodebridge-rabbitmq-server" -Vhost "server-sync" -Queue "server.dead.q")"
Invoke-NodeBridge @("dead-letters", "-config", "configs/lab/server.local.yaml", "-queue", "server.dead.q", "-limit", "1")
Assert-Equal "dead queue after preview" "1" "$(Get-QueueMessages -Container "nodebridge-rabbitmq-server" -Vhost "server-sync" -Queue "server.dead.q")"
Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync server.dead.q | Out-Null"

Write-Host "retry e2e passed"
