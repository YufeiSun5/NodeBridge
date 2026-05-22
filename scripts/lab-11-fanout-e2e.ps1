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

function Sync-FanoutTable {
    param(
        [string]$TableName,
        [int]$ID,
        [string]$Value
    )
    $eventID = "evt-fanout-$TableName"
    $file = New-Lab11EventFile -EventID $eventID -EventType "INSERT" -OriginNodeID "edge-001" -SourceNodeID "edge-001" -TargetNodeID "server-001" -TableName $TableName -ID $ID -Name "$TableName from edge-001" -Value $Value
    Sync-Lab11EdgeEvent -EdgeIndex 1 -EventFile $file -ConsumeDownlinks $true
    Assert-Lab11Equal "server $TableName value" $Value (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT value FROM $TableName WHERE id=$ID;")
    for ($i = 2; $i -le 10; $i++) {
        $edgeID = Get-Lab11EdgeID $i
        $mysql = Get-Lab11EdgeMySQLContainer $i
        Assert-Lab11Equal "$edgeID $TableName value" $Value (Invoke-Lab11Scalar $mysql "scada_edge" "SELECT value FROM $TableName WHERE id=$ID;")
        Assert-Lab11Equal "$edgeID $TableName apply count" "1" (Invoke-Lab11Scalar $mysql "scada_edge" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='$eventID';")
    }
    $originDepth = Get-Lab11QueueDepth "nodebridge-lab11-rabbitmq-server" "server-sync" "edge-001.downlink.q"
    Assert-Lab11Equal "edge-001 no loopback for $TableName" "0" ([string]$originDepth)
}

Write-Host "lab11 fanout e2e: device_config"
Sync-FanoutTable -TableName "device_config" -ID 2401 -Value "DEVICE-FANOUT"

Write-Host "lab11 fanout e2e: point_config"
Sync-FanoutTable -TableName "point_config" -ID 2402 -Value "POINT-FANOUT"

Write-Host "lab11 fanout e2e passed"
