param(
    [switch]$SkipPrepare
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/lab11.ps1") -RepoRoot $root

if (-not $SkipPrepare) {
    & (Join-Path $root "scripts/lab-11-prepare.ps1")
}

Reset-Lab11Queues
Reset-Lab11Data

$selectedRules = Join-Path $env:TEMP "nodebridge-lab11-selected-rules.yaml"
$rulesText = Get-Content (Join-Path $root $script:Lab11Rules) -Raw
$rulesText = $rulesText -replace "id: device-config-bidirectional([\s\S]*?)dispatch_target: ACTIVE_EDGES", "id: device-config-bidirectional`$1dispatch_target: SELECTED_EDGES`n    dispatch_node_ids: [edge-002, edge-005]"
[System.IO.File]::WriteAllText($selectedRules, $rulesText, [System.Text.UTF8Encoding]::new($false))

$eventID = "evt-selected-device"
$file = New-Lab11EventFile -EventID $eventID -EventType "INSERT" -OriginNodeID "edge-001" -SourceNodeID "edge-001" -TargetNodeID "server-001" -TableName "device_config" -ID 2601 -Name "Selected Device" -Value "SELECTED"

Write-Host "lab11 selected e2e: publish edge-001 event"
Invoke-NodeBridge @("publish-event", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5681/edge-001-sync", "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-file", $file)
Invoke-NodeBridge @("forward-upload-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5681/edge-001-sync", "-server-amqp-url", $script:Lab11ServerRabbitURL)
Invoke-NodeBridge @("consume-once", "-config", $script:Lab11ServerConfig, "-rules", $selectedRules, "-amqp-url", $script:Lab11ServerRabbitURL)

Write-Host "lab11 selected e2e: consume selected edge downlinks"
foreach ($i in @(2, 5)) {
    Invoke-NodeBridge @("consume-downlink-once", "-config", (Get-Lab11EdgeConfig $i), "-rules", $selectedRules, "-amqp-url", $script:Lab11ServerRabbitURL)
}

Assert-Lab11Equal "edge-002 selected value" "SELECTED" (Invoke-Lab11Scalar (Get-Lab11EdgeMySQLContainer 2) "scada_edge" "SELECT value FROM device_config WHERE id=2601;")
Assert-Lab11Equal "edge-005 selected value" "SELECTED" (Invoke-Lab11Scalar (Get-Lab11EdgeMySQLContainer 5) "scada_edge" "SELECT value FROM device_config WHERE id=2601;")

foreach ($i in @(3, 4, 6, 7, 8, 9, 10)) {
    $edgeID = Get-Lab11EdgeID $i
    Assert-Lab11Equal "$edgeID must not receive selected event" "0" (Invoke-Lab11Scalar (Get-Lab11EdgeMySQLContainer $i) "scada_edge" "SELECT COUNT(1) FROM device_config WHERE id=2601;")
    Assert-Lab11Equal "$edgeID selected queue depth" "0" (Get-Lab11QueueDepth "nodebridge-lab11-rabbitmq-server" "server-sync" "$edgeID.downlink.q")
}
Assert-Lab11Equal "edge-001 must not receive origin event" "0" (Get-Lab11QueueDepth "nodebridge-lab11-rabbitmq-server" "server-sync" "edge-001.downlink.q")

Write-Host "lab11 selected e2e passed"
