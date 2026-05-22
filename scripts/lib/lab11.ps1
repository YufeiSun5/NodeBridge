param(
    [string]$RepoRoot
)

if (-not $RepoRoot) {
    $RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
}

. (Join-Path $RepoRoot "scripts/lib/env.ps1") -RepoRoot $RepoRoot

$script:Lab11ServerRabbitURL = "amqp://sync:sync_password@127.0.0.1:5699/server-sync"
$script:Lab11ServerConfig = "configs/lab11/server.local.yaml"
$script:Lab11Rules = "configs/sync-rules.10-edge.example.yaml"

function Get-Lab11EdgeID {
    param([int]$Index)
    return "edge-{0:D3}" -f $Index
}

function Get-Lab11EdgeRabbitPort {
    param([int]$Index)
    return 5680 + $Index
}

function Get-Lab11EdgeRabbitUIPort {
    param([int]$Index)
    return 15680 + $Index
}

function Get-Lab11EdgeMySQLPort {
    param([int]$Index)
    return 3310 + $Index
}

function Get-Lab11EdgeRabbitContainer {
    param([int]$Index)
    return "nodebridge-lab11-rabbitmq-$(Get-Lab11EdgeID $Index)"
}

function Get-Lab11EdgeMySQLContainer {
    param([int]$Index)
    return "nodebridge-lab11-mysql-$(Get-Lab11EdgeID $Index)"
}

function Get-Lab11EdgeConfig {
    param([int]$Index)
    return "configs/lab11/$(Get-Lab11EdgeID $Index).local.yaml"
}

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Invoke-NodeBridge {
    param([string[]]$Arguments)
    Push-Location $RepoRoot
    try {
        & go run ./cmd/sync-agent @Arguments
        Assert-LastExit "go run ./cmd/sync-agent $($Arguments -join ' ')"
    } finally {
        Pop-Location
    }
}

function Invoke-DockerCommand {
    param([string]$Command)
    Invoke-Expression $Command
    if ($LASTEXITCODE -ne 0) {
        throw "docker command failed: $Command"
    }
}

function Invoke-Lab11Scalar {
    param(
        [string]$Container,
        [string]$Database,
        [string]$Query
    )
    docker exec $Container mysql -usync_user -psync_password -N -B $Database -e $Query
}

function Get-Lab11QueueDepth {
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

function Assert-Lab11Equal {
    param(
        [string]$Name,
        [string]$Expected,
        [string]$Actual
    )
    if ($Actual -ne $Expected) {
        throw "$Name expected '$Expected', got '$Actual'"
    }
}

function Reset-Lab11Queues {
    for ($i = 1; $i -le 10; $i++) {
        $edgeID = Get-Lab11EdgeID $i
        $rabbit = Get-Lab11EdgeRabbitContainer $i
        foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
            Invoke-DockerCommand "docker exec $rabbit rabbitmqctl purge_queue -p $edgeID-sync $queue | Out-Null"
        }
        Invoke-DockerCommand "docker exec nodebridge-lab11-rabbitmq-server rabbitmqctl purge_queue -p server-sync $edgeID.downlink.q | Out-Null"
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q")) {
        Invoke-DockerCommand "docker exec nodebridge-lab11-rabbitmq-server rabbitmqctl purge_queue -p server-sync $queue | Out-Null"
    }
}

function Reset-Lab11Data {
    $deleteSummary = (1..10 | ForEach-Object { "DELETE FROM data_all_edge_{0:D3};" -f $_ }) -join " "
    Invoke-DockerCommand "docker exec nodebridge-lab11-mysql-server mysql -usync_user -psync_password scada_center -e `"DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_config; DELETE FROM point_config; $deleteSummary`""
    for ($i = 1; $i -le 10; $i++) {
        $mysql = Get-Lab11EdgeMySQLContainer $i
        Invoke-DockerCommand "docker exec $mysql mysql -usync_user -psync_password scada_edge -e `"DELETE FROM sync_apply_log; DELETE FROM device_config; DELETE FROM point_config; DELETE FROM data_all;`""
    }
}

function New-Lab11EventFile {
    param(
        [string]$EventID,
        [string]$EventType,
        [string]$OriginNodeID,
        [string]$SourceNodeID,
        [string]$TargetNodeID,
        [string]$TableName,
        [int]$ID,
        [string]$Name,
        [string]$Value
    )
    $event = @{
        event_id = $EventID
        event_type = $EventType
        origin_node_id = $OriginNodeID
        source_node_id = $SourceNodeID
        target_node_id = $TargetNodeID
        database_name = "scada_edge"
        table_name = $TableName
        primary_key = @{ id = $ID }
        after = @{
            id = $ID
            name = $Name
            value = $Value
            sync_version = 1
            updated_by_node = $OriginNodeID
            last_event_id = $EventID
            updated_at = "2026-05-21T10:00:00+08:00"
        }
        schema_version = 1
        sync_version = 1
        created_at = "2026-05-21T10:00:01+08:00"
        event_time = "2026-05-21T10:00:00+08:00"
        trace_id = "trace-$EventID"
    }
    $path = Join-Path $env:TEMP "$EventID.json"
    [System.IO.File]::WriteAllText($path, ($event | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
    return $path
}

function New-Lab11ChangeFile {
    param(
        [string]$DatabaseName,
        [string]$TableName,
        [string]$Operation,
        [int]$ID,
        [string]$Name,
        [string]$Value,
        [string]$UpdatedByNode,
        [string]$LastEventID
    )
    $change = @{
        database_name = $DatabaseName
        table_name = $TableName
        operation = $Operation
        primary_key = @{ id = $ID }
        after = @{
            id = $ID
            name = $Name
            value = $Value
            sync_version = 1
            updated_by_node = $UpdatedByNode
            last_event_id = $LastEventID
            updated_at = "2026-05-21T10:00:00+08:00"
        }
        event_time = "2026-05-21T10:00:00+08:00"
    }
    $path = Join-Path $env:TEMP "$LastEventID.change.json"
    [System.IO.File]::WriteAllText($path, ($change | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
    return $path
}

function Sync-Lab11EdgeEvent {
    param(
        [int]$EdgeIndex,
        [string]$EventFile,
        [bool]$ConsumeDownlinks = $true
    )
    $edgeID = Get-Lab11EdgeID $EdgeIndex
    $edgePort = Get-Lab11EdgeRabbitPort $EdgeIndex
    Invoke-NodeBridge @("publish-event", "-amqp-url", "amqp://sync:sync_password@127.0.0.1:$edgePort/$edgeID-sync", "-exchange", "edge.upload.x", "-routing-key", "edge.upload.cdc", "-file", $EventFile)
    Invoke-NodeBridge @("forward-upload-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:$edgePort/$edgeID-sync", "-server-amqp-url", $script:Lab11ServerRabbitURL)
    Invoke-NodeBridge @("consume-once", "-config", $script:Lab11ServerConfig, "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL)
    if ($ConsumeDownlinks) {
        for ($i = 1; $i -le 10; $i++) {
            if ($i -eq $EdgeIndex) {
                continue
            }
            Invoke-NodeBridge @("consume-downlink-once", "-config", (Get-Lab11EdgeConfig $i), "-rules", $script:Lab11Rules, "-amqp-url", $script:Lab11ServerRabbitURL)
        }
    }
}
