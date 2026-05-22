param(
    [switch]$SkipPrepare,
    [int]$Count = 10
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/lab11.ps1") -RepoRoot $root

if ($Count -le 0) {
    throw "Count must be greater than 0"
}

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-11-prepare.ps1")
}

function Wait-Lab11Rabbit {
    param([string]$Container)
    for ($i = 0; $i -lt 45; $i++) {
        docker exec $Container rabbitmq-diagnostics -q ping 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            docker exec $Container rabbitmqctl await_startup 2>$null | Out-Null
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "RabbitMQ container not ready: $Container"
}

function Invoke-NodeBridgeExpectFail {
    param([string[]]$Arguments)
    Push-Location $root
    try {
        & go run ./cmd/sync-agent @Arguments
        if ($LASTEXITCODE -eq 0) {
            throw "command unexpectedly succeeded: go run ./cmd/sync-agent $($Arguments -join ' ')"
        }
    } finally {
        Pop-Location
    }
}

function Invoke-Lab11ServerRecovery {
    Invoke-NodeBridge @("init-rabbitmq", "-mode", "server", "-config", $script:Lab11ServerConfig, "-amqp-url", $script:Lab11ServerRabbitURL)
}

Reset-Lab11Queues
Reset-Lab11Data

$edgeID = Get-Lab11EdgeID 1
$edgeURL = "amqp://sync:sync_password@127.0.0.1:$(Get-Lab11EdgeRabbitPort 1)/$edgeID-sync"

Write-Host "lab11 disconnect: server rabbitmq down, edge local queue must retain messages"
docker stop nodebridge-lab11-rabbitmq-server | Out-Null

Invoke-NodeBridge @("publish-stress-batch", "-amqp-url", $edgeURL, "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-count", "$Count", "-batch-size", "50", "-event-id-prefix", "evt-lab11-server-down", "-origin-node-id", $edgeID, "-database", "scada_edge", "-table", "device_config")
Invoke-NodeBridgeExpectFail @("forward-upload-batch-once", "-local-amqp-url", $edgeURL, "-server-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")

$edgeDepth = Get-Lab11QueueDepth (Get-Lab11EdgeRabbitContainer 1) "$edgeID-sync" "edge.upload.cdc.q"
Assert-Lab11Equal "edge upload retained while server down" "$Count" ([string]$edgeDepth)

docker start nodebridge-lab11-rabbitmq-server | Out-Null
Wait-Lab11Rabbit "nodebridge-lab11-rabbitmq-server"
Invoke-Lab11ServerRecovery

Invoke-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", $edgeURL, "-server-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
Invoke-NodeBridge @("consume-batch-once", "-config", $script:Lab11ServerConfig, "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
for ($i = 2; $i -le 10; $i++) {
    Invoke-NodeBridge @("consume-downlink-batch-once", "-config", (Get-Lab11EdgeConfig $i), "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
}

Assert-Lab11Equal "edge upload drained after server recovery" "0" ([string](Get-Lab11QueueDepth (Get-Lab11EdgeRabbitContainer 1) "$edgeID-sync" "edge.upload.cdc.q"))
Assert-Lab11Equal "server device count after server recovery" "$Count" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM device_config;")
for ($i = 2; $i -le 10; $i++) {
    Assert-Lab11Equal "edge $i device count after server recovery" "$Count" (Invoke-Lab11Scalar (Get-Lab11EdgeMySQLContainer $i) "scada_edge" "SELECT COUNT(1) FROM device_config;")
}

Write-Host "lab11 disconnect: edge local rabbitmq restart must preserve durable upload queue"
Reset-Lab11Queues
Reset-Lab11Data
Invoke-NodeBridge @("publish-stress-batch", "-amqp-url", $edgeURL, "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-count", "$Count", "-batch-size", "50", "-event-id-prefix", "evt-lab11-edge-restart", "-origin-node-id", $edgeID, "-database", "scada_edge", "-table", "point_config")
docker stop (Get-Lab11EdgeRabbitContainer 1) | Out-Null
docker start (Get-Lab11EdgeRabbitContainer 1) | Out-Null
Wait-Lab11Rabbit (Get-Lab11EdgeRabbitContainer 1)
Invoke-NodeBridge @("init-rabbitmq", "-mode", "edge", "-amqp-url", $edgeURL)

Assert-Lab11Equal "edge upload retained after local rabbit restart" "$Count" ([string](Get-Lab11QueueDepth (Get-Lab11EdgeRabbitContainer 1) "$edgeID-sync" "edge.upload.cdc.q"))
Invoke-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", $edgeURL, "-server-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
Invoke-NodeBridge @("consume-batch-once", "-config", $script:Lab11ServerConfig, "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
for ($i = 2; $i -le 10; $i++) {
    Invoke-NodeBridge @("consume-downlink-batch-once", "-config", (Get-Lab11EdgeConfig $i), "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
}

Assert-Lab11Equal "server point count after edge restart" "$Count" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM point_config;")
Assert-Lab11Equal "failed ack count" "0" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED';")

$summary = [pscustomobject]@{
    count = $Count
    server_disconnect_recovered = $true
    edge_local_restart_recovered = $true
    failed_count = 0
}
$summaryPath = Join-Path $root "build/lab-11-disconnect-summary.json"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $summaryPath) | Out-Null
[System.IO.File]::WriteAllText($summaryPath, ($summary | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))

Write-Host "lab11 disconnect e2e passed"
Write-Host "summary=$summaryPath"
