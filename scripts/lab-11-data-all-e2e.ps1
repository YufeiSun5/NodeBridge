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

Write-Host "lab11 data_all e2e: publish one event per edge"
for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    $eventID = "evt-data-all-$edgeID"
    $file = New-Lab11EventFile -EventID $eventID -EventType "INSERT" -OriginNodeID $edgeID -SourceNodeID $edgeID -TargetNodeID "server-001" -TableName "data_all" -ID $i -Name "Data $edgeID" -Value "VALUE-$edgeID"
    Sync-Lab11EdgeEvent -EdgeIndex $i -EventFile $file -ConsumeDownlinks $false
}

Write-Host "lab11 data_all e2e: verify server summary tables"
for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    $table = "data_all_edge_{0:D3}" -f $i
    Assert-Lab11Equal "$table count" "1" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM $table WHERE id=$i AND value='VALUE-$edgeID';")
}

Write-Host "lab11 data_all e2e: verify no downlink dispatch"
for ($i = 1; $i -le 10; $i++) {
    $edgeID = Get-Lab11EdgeID $i
    $depth = Get-Lab11QueueDepth "nodebridge-lab11-rabbitmq-server" "server-sync" "$edgeID.downlink.q"
    Assert-Lab11Equal "$edgeID downlink depth" "0" ([string]$depth)
}

Write-Host "lab11 data_all e2e passed"
