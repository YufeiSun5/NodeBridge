param(
    [switch]$SkipPrepare,
    [int]$CountPerEdge = 10,
    [int]$MultiCount = 20
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/lab11.ps1") -RepoRoot $root

if (-not $SkipPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-11-prepare.ps1")
}

Reset-Lab11Queues
Reset-Lab11Data

$timer = [System.Diagnostics.Stopwatch]::StartNew()
$maxQueueDepth = 0

function Update-MaxDepth {
    param([int]$Value)
    if ($Value -gt $script:maxQueueDepth) {
        $script:maxQueueDepth = $Value
    }
}

function Invoke-EdgeStress {
    param(
        [int]$EdgeIndex,
        [string]$TableName,
        [int]$Count,
        [string]$Prefix
    )
    $edgeID = Get-Lab11EdgeID $EdgeIndex
    $edgePort = Get-Lab11EdgeRabbitPort $EdgeIndex
    $edgeURL = "amqp://sync:sync_password@127.0.0.1:$edgePort/$edgeID-sync"
    Invoke-NodeBridge @("publish-stress-batch", "-amqp-url", $edgeURL, "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-count", "$Count", "-batch-size", "50", "-event-id-prefix", $Prefix, "-origin-node-id", $edgeID, "-database", "scada_edge", "-table", $TableName)
    Update-MaxDepth (Get-Lab11QueueDepth (Get-Lab11EdgeRabbitContainer $EdgeIndex) "$edgeID-sync" "edge.upload.cdc.q")
    Invoke-NodeBridge @("forward-upload-batch-once", "-local-amqp-url", $edgeURL, "-server-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
    Update-MaxDepth (Get-Lab11QueueDepth "nodebridge-lab11-rabbitmq-server" "server-sync" "server.cdc.ingress.q")
    Invoke-NodeBridge @("consume-batch-once", "-config", $script:Lab11ServerConfig, "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
}

Write-Host "lab11 stress: publish data_all from 10 edges"
for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    Invoke-EdgeStress -EdgeIndex $i -TableName "data_all" -Count $CountPerEdge -Prefix "evt-data-all-$edgeID-stress"
}

Write-Host "lab11 stress: publish multi-master tables from edge-001"
Invoke-EdgeStress -EdgeIndex 1 -TableName "device_config" -Count $MultiCount -Prefix "evt-device-lab11-stress"
Invoke-EdgeStress -EdgeIndex 1 -TableName "point_config" -Count $MultiCount -Prefix "evt-point-lab11-stress"

Write-Host "lab11 stress: consume fanout downlinks"
for ($i = 2; $i -le 10; $i++) {
    Invoke-NodeBridge @("consume-downlink-batch-once", "-config", (Get-Lab11EdgeConfig $i), "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL, "-max-batch", "50", "-flush-interval-millis", "500")
}

$timer.Stop()

for ($i = 1; $i -le 10; $i++) {
    $table = "data_all_edge_{0:D3}" -f $i
    Assert-Lab11Equal "$table count" "$CountPerEdge" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM $table;")
}
Assert-Lab11Equal "server device count" "$MultiCount" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM device_config;")
Assert-Lab11Equal "server point count" "$MultiCount" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM point_config;")
for ($i = 2; $i -le 10; $i++) {
    $mysql = Get-Lab11EdgeMySQLContainer $i
    Assert-Lab11Equal "edge $i device count" "$MultiCount" (Invoke-Lab11Scalar $mysql "scada_edge" "SELECT COUNT(1) FROM device_config;")
    Assert-Lab11Equal "edge $i point count" "$MultiCount" (Invoke-Lab11Scalar $mysql "scada_edge" "SELECT COUNT(1) FROM point_config;")
}
Assert-Lab11Equal "failed ack count" "0" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED';")

$totalEvents = (10 * $CountPerEdge) + (2 * $MultiCount)
$throughput = [Math]::Round($totalEvents / [Math]::Max($timer.Elapsed.TotalSeconds, 0.001), 2)
$summary = @{
    total_events = $totalEvents
    elapsed_seconds = [Math]::Round($timer.Elapsed.TotalSeconds, 2)
    throughput_per_second = $throughput
    max_queue_depth = $maxQueueDepth
    failed_count = 0
    server_apply_log_count = Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_apply_log;"
}
$summaryPath = Join-Path $root "build/lab-11-stress-summary.json"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $summaryPath) | Out-Null
[System.IO.File]::WriteAllText($summaryPath, ($summary | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))

Write-Host "lab11 stress e2e passed"
Write-Host "total_events=$totalEvents elapsed_seconds=$($summary.elapsed_seconds) throughput_per_second=$throughput max_queue_depth=$maxQueueDepth"
Write-Host "summary=$summaryPath"
