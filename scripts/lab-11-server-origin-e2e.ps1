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

function Sync-ServerOriginTable {
    param(
        [string]$TableName,
        [int]$ID,
        [string]$Name,
        [string]$Value,
        [string]$EventID
    )
    Invoke-DockerCommand "docker exec nodebridge-lab11-mysql-server mysql -usync_user -psync_password scada_center -e `"INSERT INTO $TableName (id, name, value, sync_version, updated_by_node, last_event_id, updated_at) VALUES ($ID, '$Name', '$Value', 1, 'server-001', '$EventID', NOW(3)) ON DUPLICATE KEY UPDATE value='$Value', updated_by_node='server-001', last_event_id='$EventID', updated_at=NOW(3);`""
    $file = New-Lab11ChangeFile -DatabaseName "scada_center" -TableName $TableName -Operation "INSERT" -ID $ID -Name $Name -Value $Value -UpdatedByNode "server-001" -LastEventID $EventID
    Invoke-NodeBridge @("server-cdc-dispatch-once", "-config", $script:Lab11ServerConfig, "-rules", $script:Lab11Rules, "-file", $file, "-event-id", $EventID, "-amqp-url", $script:Lab11ServerRabbitURL)
    for ($i = 1; $i -le 10; $i++) {
        Invoke-NodeBridge @("consume-downlink-once", "-config", (Get-Lab11EdgeConfig $i), "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL)
    }
    for ($i = 1; $i -le 10; $i++) {
        $edgeID = Get-Lab11EdgeID $i
        $mysql = Get-Lab11EdgeMySQLContainer $i
        Assert-Lab11Equal "$edgeID $TableName server-origin value" $Value (Invoke-Lab11Scalar $mysql "scada_edge" "SELECT value FROM $TableName WHERE id=$ID;")
        Assert-Lab11Equal "$edgeID $TableName server-origin apply count" "1" (Invoke-Lab11Scalar $mysql "scada_edge" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='$EventID';")
    }
    Assert-Lab11Equal "server must not apply $TableName server-origin event" "0" (Invoke-Lab11Scalar "nodebridge-lab11-mysql-server" "scada_center" "SELECT COUNT(1) FROM sync_apply_log WHERE event_id='$EventID';")
}

Write-Host "lab11 server-origin e2e: device_config"
Sync-ServerOriginTable -TableName "device_config" -ID 2501 -Name "Server Device" -Value "SERVER-ORIGIN-DEVICE" -EventID "evt-server-origin-device"

Write-Host "lab11 server-origin e2e: point_config"
Sync-ServerOriginTable -TableName "point_config" -ID 2502 -Name "Server Point" -Value "SERVER-ORIGIN-POINT" -EventID "evt-server-origin-point"

Write-Host "lab11 server-origin e2e passed"
