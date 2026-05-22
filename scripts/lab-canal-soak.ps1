param(
    [switch]$SkipPrepare,
    [int]$Count = 20
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

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-canal-prepare.ps1")
}

if ($Count -lt 1) {
    throw "Count must be positive"
}

foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
    Invoke-Docker "docker exec nodebridge-rabbitmq-edge-a rabbitmqctl purge_queue -p edge-a-sync $queue | Out-Null"
}
foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
    Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
}
Invoke-Docker "docker exec nodebridge-mysql-edge-a mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_upload_offset; DELETE FROM sync_apply_log; DELETE FROM device_config;`""
Invoke-Docker "docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_apply_log; DELETE FROM device_settings;`""
Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_upload_offset; DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings;`""

$values = New-Object System.Collections.Generic.List[string]
for ($i = 1; $i -le $Count; $i++) {
    $id = 20000 + $i
    $values.Add("($id, 'canal-soak-$i', 'V-$i', 1, 'edge-001', 'canal-soak-$i', NOW(3))")
}
$insertSql = "INSERT INTO device_config(id, name, value, sync_version, updated_by_node, last_event_id, updated_at) VALUES " + ($values -join ",") + ";"
Invoke-Docker "docker exec nodebridge-mysql-edge-a mysql -usync_user -psync_password scada_edge -e `"$insertSql`""

$maxBatch = $Count + 100
Invoke-NodeBridge @("canal-publish-once", "-config", "configs/lab/edge-a.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync")
Invoke-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-server-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "$maxBatch", "-flush-interval-millis", "500")
Invoke-NodeBridge @("consume-batch-once", "-config", "configs/lab/server.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "$maxBatch", "-flush-interval-millis", "500")
Invoke-NodeBridge @("consume-downlink-batch-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "$maxBatch", "-flush-interval-millis", "500")

Assert-Equal "server canal soak count" "$Count" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM device_settings WHERE setting_id BETWEEN 20001 AND $(20000 + $Count);")
Assert-Equal "edge-b canal soak count" "$Count" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM device_settings WHERE setting_id BETWEEN 20001 AND $(20000 + $Count);")
Assert-Equal "edge offset count" "1" (Invoke-Scalar "nodebridge-mysql-edge-a" "scada_edge" "SELECT COUNT(1) FROM sync_upload_offset WHERE reader_name='edge-001';")
Assert-Equal "failed ack count" "0" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED';")

Write-Host "canal soak passed count=$Count"
