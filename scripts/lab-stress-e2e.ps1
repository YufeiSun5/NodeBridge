param(
    [switch]$SkipPrepare,
    [int]$Count = 1000
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
    if (-not $script:NodeBridgeExe) {
        throw "NodeBridge binary is not built"
    }
    if ($Arguments.Count -gt 0 -and $Arguments[0] -eq "publish-event") {
        & $script:NodeBridgeExe @Arguments | Out-Null
    } else {
        & $script:NodeBridgeExe @Arguments
    }
    Assert-LastExit "$script:NodeBridgeExe $($Arguments -join ' ')"
}

function Build-NodeBridge {
    Push-Location $root
    try {
        $output = Join-Path $env:TEMP "nodebridge-sync-agent-stress.exe"
        & go build -o $output ./cmd/sync-agent
        Assert-LastExit "go build ./cmd/sync-agent"
        $script:NodeBridgeExe = $output
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

Build-NodeBridge
Reset-LabState

$timer = [System.Diagnostics.Stopwatch]::StartNew()
Write-Host "stress e2e: publish $Count events"
Run-NodeBridge @("publish-stress-batch", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-count", "$Count", "-batch-size", "50", "-event-id-prefix", "evt-device-stress", "-origin-node-id", "edge-001", "-database", "scada_edge", "-table", "device_config")

$batches = [Math]::Ceiling($Count / 50)
Write-Host "stress e2e: process $batches batches"
for ($i = 1; $i -le $batches; $i++) {
    Run-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-server-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "50", "-flush-interval-millis", "500")
    Run-NodeBridge @("consume-batch-once", "-config", "configs/lab/server.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "50", "-flush-interval-millis", "500")
    Run-NodeBridge @("consume-downlink-batch-once", "-config", "configs/lab/edge-b.local.yaml", "-rules", "configs/sync-rules.example.yaml", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync", "-max-batch", "50", "-flush-interval-millis", "500")
}
$timer.Stop()

$expected = [string]$Count
Assert-Equal "server device count" $expected (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM device_settings;")
Assert-Equal "edge-b device count" $expected (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM device_settings;")
Assert-Equal "server apply count" $expected (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id LIKE 'evt-device-stress-%';")
Assert-Equal "edge-b apply count" $expected (Invoke-Scalar "nodebridge-mysql-edge-b" "scada_edge" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id LIKE 'evt-device-stress-%';")
Assert-Equal "failed ack count" "0" (Invoke-Scalar "nodebridge-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED';")

$throughput = [Math]::Round($Count / [Math]::Max($timer.Elapsed.TotalSeconds, 0.001), 2)
$summary = @{
    count = $Count
    elapsed_seconds = [Math]::Round($timer.Elapsed.TotalSeconds, 2)
    throughput_per_second = $throughput
    server_apply_count = $expected
    edge_b_apply_count = $expected
    failed_ack_count = 0
}
$summaryPath = Join-Path $root "build/lab-stress-summary.json"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $summaryPath) | Out-Null
[System.IO.File]::WriteAllText($summaryPath, ($summary | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))
Write-Host "stress e2e passed"
Write-Host "count=$Count elapsed_seconds=$([Math]::Round($timer.Elapsed.TotalSeconds, 2)) throughput_per_second=$throughput"
Write-Host "summary=$summaryPath"
