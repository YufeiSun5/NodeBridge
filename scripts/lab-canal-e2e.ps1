param(
    [switch]$SkipPrepare,
    [string]$EdgeCanalAddr = "127.0.0.1:11111",
    [string]$ServerCanalAddr = "127.0.0.1:11113",
    [switch]$SkipServerCanal
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

function Test-TcpEndpoint {
    param([string]$Address)
    $parts = $Address.Split(":")
    if ($parts.Length -ne 2) {
        throw "invalid endpoint: $Address"
    }
    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $iar = $client.BeginConnect($parts[0], [int]$parts[1], $null, $null)
        if (-not $iar.AsyncWaitHandle.WaitOne([TimeSpan]::FromSeconds(2))) {
            throw "Canal endpoint is not reachable: $Address"
        }
        $client.EndConnect($iar)
    } finally {
        $client.Close()
    }
}

function Write-TempServerCanalConfig {
    param([string]$CanalAddr)
    $path = Join-Path $env:TEMP "nodebridge-server-canal.local.yaml"
    $yaml = @"
mode: server

node:
  id: server-001
  name: Lab Server
  location: single-pc-lab

mysql:
  host: 127.0.0.1
  port: 3309
  username: sync_user
  password: sync_password
  database: scada_center

rabbitmq:
  mode: external
  install: false
  server_url: amqp://sync:sync_password@127.0.0.1:5675/server-sync
  management_url: http://127.0.0.1:15675
  username: sync
  password: sync_password
  vhost: server-sync

cdc:
  type: canal
  reader_name: server-001
  canal_addr: $CanalAddr
  destination: server-001
  username: ""
  password: ""
  filter: scada_center\..*
  batch_size: 1000
  use_gtid: false

sync:
  upload_batch_size: 50
  dispatch_batch_size: 50
  flush_interval_millis: 500
  retry_interval_seconds: 1
  node_timeout_seconds: 60

log_web:
  enable: false

mcp_server:
  enable: false

security:
  admin_password: admin-pass
  exit_password: exit-pass
"@
    [System.IO.File]::WriteAllText($path, $yaml, [System.Text.UTF8Encoding]::new($false))
    return $path
}

function Reset-LabCanalState {
    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-edge-a rabbitmqctl purge_queue -p edge-a-sync $queue | Out-Null"
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
        Invoke-Docker "docker exec nodebridge-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
    }
    Invoke-Docker "docker exec nodebridge-mysql-edge-a mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_upload_offset; DELETE FROM sync_apply_log; DELETE FROM device_config;`""
    Invoke-Docker "docker exec nodebridge-mysql-edge-b mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_upload_offset; DELETE FROM sync_apply_log; DELETE FROM device_settings; DELETE FROM device_config; DELETE FROM point_config;`""
    Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_upload_offset; DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings; DELETE FROM device_config; DELETE FROM point_config;`""
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-canal-prepare.ps1")
}

Write-Host "checking edge Canal endpoint $EdgeCanalAddr"
Test-TcpEndpoint $EdgeCanalAddr
if (-not $SkipServerCanal) {
    Write-Host "checking server Canal endpoint $ServerCanalAddr"
    Test-TcpEndpoint $ServerCanalAddr
}

Reset-LabCanalState

Write-Host "canal e2e: edge mysql -> canal -> local rabbitmq"
Invoke-Docker "docker exec nodebridge-mysql-edge-a mysql -usync_user -psync_password scada_edge -e `"INSERT INTO device_config(id, name, value, sync_version, updated_by_node, last_event_id, updated_at) VALUES (901, 'canal-device', 'ON', 1, 'edge-001', 'canal-seed-901', NOW(3));`""
Invoke-NodeBridge @("canal-publish-once", "-config", "configs/lab/edge-a.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync")
Invoke-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-server-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
Invoke-NodeBridge @("consume-batch-once", "-config", "configs/lab/server.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
Invoke-NodeBridge @("consume-downlink-batch-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")

Assert-Equal "server canal device value" "ON" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT setting_value FROM device_settings WHERE setting_id=901;")
Assert-Equal "edge-b canal device value" "ON" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT setting_value FROM device_settings WHERE setting_id=901;")
Assert-Equal "edge offset count" "1" (Invoke-Scalar "nodebridge-mysql-edge-a" "scada_edge" "SELECT COUNT(1) FROM sync_upload_offset WHERE reader_name='edge-001';")

if (-not $SkipServerCanal) {
    Write-Host "canal e2e: server mysql -> canal -> downlink"
    $serverCanalConfig = Write-TempServerCanalConfig $ServerCanalAddr
    Invoke-Docker "docker exec nodebridge-mysql-server mysql -usync_user -psync_password scada_center -e `"INSERT INTO point_config(id, name, value, sync_version, updated_by_node, last_event_id, updated_at) VALUES (902, 'server-point', 'S-ON', 1, 'server-001', 'server-canal-seed-902', NOW(3));`""
    Invoke-NodeBridge @("server-canal-dispatch-once", "-config", $serverCanalConfig, "-rules", "configs/sync-rules.example.yaml")
    Invoke-NodeBridge @("consume-downlink-batch-once", "-config", "configs/lab/edge-a.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
    Invoke-NodeBridge @("consume-downlink-batch-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")

    Assert-Equal "edge-a server canal point value" "S-ON" (Invoke-Scalar "nodebridge-mysql-edge-a" "scada_edge" "SELECT value FROM point_config WHERE id=902;")
    Assert-Equal "edge-b server canal point value" "S-ON" (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT value FROM point_config WHERE id=902;")
}

Write-Host "canal e2e passed"
Write-Host "note: this script requires external Canal instances configured to read the lab MySQL binlogs; Docker lab does not ship Canal."
