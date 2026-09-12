param(
    [switch]$SkipPrepare,
    [switch]$KeepCanal,
    [int]$CanalPort = 11121,
    [string]$CanalImage = "canal/canal-server:latest"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$work = Join-Path $root ".cache/governance-schema-e2e"
$rulesPath = Join-Path $work "sync-rules.yaml"
$edgeAConfig = Join-Path $work "edge-a.yaml"
$edgeBConfig = Join-Path $work "edge-b.yaml"
$serverConfig = Join-Path $work "server.yaml"
$canalConfig = Join-Path $work "edge-a-canal.yaml"
$reportPath = Join-Path $work "report.json"
$canalContainer = "nodebridge-canal-governance-e2e"

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
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    & docker @Arguments
    Assert-LastExit "docker $($Arguments -join ' ')"
}

function Invoke-MySQL {
    param(
        [string]$Container,
        [string]$Database,
        [string]$SQL,
        [switch]$Scalar
    )
    $arguments = @("exec", $Container, "mysql", "-usync_user", "-psync_password")
    if ($Scalar) {
        $arguments += @("-N", "-B")
    }
    $arguments += @($Database, "-e", $SQL)
    $output = & docker @arguments
    Assert-LastExit "docker exec $Container mysql"
    if ($Scalar) {
        return (($output | Out-String).Trim())
    }
}

function Invoke-Agent {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    Push-Location $root
    try {
        & go run ./cmd/sync-agent @Arguments
        Assert-LastExit "go run ./cmd/sync-agent $($Arguments -join ' ')"
    } finally {
        Pop-Location
    }
}

function Invoke-MCP {
    param(
        [string]$Config,
        [string]$Tool,
        [hashtable]$Arguments = @{}
    )
    $json = $Arguments | ConvertTo-Json -Depth 20 -Compress
    $encoded = "base64:" + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($json))
    Push-Location $root
    try {
        $raw = & node scripts/mcp-governance-call.mjs $Config $rulesPath $Tool $encoded
        Assert-LastExit "MCP $Tool"
        return ($raw | ConvertFrom-Json)
    } finally {
        Pop-Location
    }
}

function Assert-Equal {
    param([string]$Name, [string]$Expected, [string]$Actual)
    if ($Actual -ne $Expected) {
        throw "$Name expected '$Expected', got '$Actual'"
    }
}

function Remove-TestColumn {
    param([string]$Container, [string]$Database, [string]$Table)
    $count = Invoke-MySQL $Container "information_schema" "SELECT COUNT(*) FROM COLUMNS WHERE TABLE_SCHEMA='$Database' AND TABLE_NAME='$Table' AND COLUMN_NAME='governed_note';" -Scalar
    if ($count -eq "1") {
        Invoke-MySQL $Container $Database "ALTER TABLE $Table DROP COLUMN governed_note;"
    }
}

function Wait-TcpEndpoint {
    param([string]$HostName, [int]$Port)
    for ($i = 0; $i -lt 60; $i++) {
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $pending = $client.BeginConnect($HostName, $Port, $null, $null)
            if ($pending.AsyncWaitHandle.WaitOne([TimeSpan]::FromSeconds(1))) {
                $client.EndConnect($pending)
                return
            }
        } catch {
            # Retry until Canal finishes booting.
        } finally {
            $client.Close()
        }
        Start-Sleep -Seconds 2
    }
    throw "Canal endpoint not ready: ${HostName}:$Port"
}

function Publish-CanalChange {
    for ($i = 1; $i -le 15; $i++) {
        Push-Location $root
        try {
            $output = & go run ./cmd/sync-agent canal-publish-once -config $canalConfig -rules $rulesPath -amqp-url "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync"
            Assert-LastExit "canal-publish-once"
        } finally {
            Pop-Location
        }
        if (($output | Out-String) -match "action=published") {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "Canal did not publish a supported change after 15 attempts"
}

function Transfer-EdgeAToEdgeB {
    Publish-CanalChange
    Invoke-Agent @("forward-upload-batch-once", "-local-amqp-url", "amqp://sync:sync_password@127.0.0.1:5673/edge-a-sync", "-server-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
    Invoke-Agent @("consume-batch-once", "-config", $serverConfig, "-rules", $rulesPath, "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
    Invoke-Agent @("consume-downlink-batch-once", "-config", $edgeBConfig, "-rules", $rulesPath, "-amqp-url", "amqp://sync:sync_password@127.0.0.1:5675/server-sync")
}

function Write-TestFiles {
    New-Item -ItemType Directory -Path $work -Force | Out-Null
    Copy-Item (Join-Path $root "configs/lab/edge-a.local.yaml") $edgeAConfig -Force
    Copy-Item (Join-Path $root "configs/lab/edge-b.local.yaml") $edgeBConfig -Force
    Copy-Item (Join-Path $root "configs/lab/server.local.yaml") $serverConfig -Force
    $canalYaml = (Get-Content -Raw $edgeAConfig).Replace("type: stub", "type: canal").Replace("canal_addr: 127.0.0.1:11111", "canal_addr: 127.0.0.1:$CanalPort")
    [System.IO.File]::WriteAllText($canalConfig, $canalYaml, [System.Text.UTF8Encoding]::new($false))
    $rulesYaml = @"
rules:
  - id: governance-device-config
    database_name: scada_edge
    table_name: device_config
    source_node_ids: [edge-001]
    target_database_name: scada_center
    target_table_name: device_settings
    direction: BIDIRECTIONAL
    dispatch_target: ACTIVE_EDGES
    conflict_policy: LAST_WRITE_WIN
    enable: true
    primary_keys: [id]
    target_primary_keys: [setting_id]
    include_columns: [id, name, value, governed_note, sync_version, updated_by_node, last_event_id, updated_at]
    column_mappings:
      - {source_column: id, target_column: setting_id}
      - {source_column: name, target_column: display_name}
      - {source_column: value, target_column: setting_value}
    schema_sync:
      add_columns: true
      drop_columns: true
"@
    [System.IO.File]::WriteAllText($rulesPath, $rulesYaml, [System.Text.UTF8Encoding]::new($false))
}

function Reset-TestState {
    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        Invoke-Docker @("exec", "nodebridge-rabbitmq-edge-a", "rabbitmqctl", "purge_queue", "-p", "edge-a-sync", $queue) | Out-Null
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "edge-001.downlink.q", "edge-002.downlink.q")) {
        Invoke-Docker @("exec", "nodebridge-rabbitmq-server", "rabbitmqctl", "purge_queue", "-p", "server-sync", $queue) | Out-Null
    }
    Remove-TestColumn "nodebridge-mysql-edge-a" "scada_edge" "device_config"
    Remove-TestColumn "nodebridge-mysql-server" "scada_center" "device_settings"
    Remove-TestColumn "nodebridge-mysql-edge-b" "scada_edge" "device_settings"
    Invoke-MySQL "nodebridge-mysql-edge-a" "scada_edge" "DELETE FROM sync_upload_offset; DELETE FROM sync_apply_log; DELETE FROM device_config WHERE id=46001;"
    Invoke-MySQL "nodebridge-mysql-server" "scada_center" "DELETE FROM sync_ack_log; DELETE FROM sync_dispatch_log; DELETE FROM sync_event_log; DELETE FROM sync_apply_log; DELETE FROM device_settings WHERE setting_id=46001;"
    Invoke-MySQL "nodebridge-mysql-edge-b" "scada_edge" "DELETE FROM sync_apply_log; DELETE FROM device_settings WHERE setting_id=46001;"
}

function Start-TestCanal {
    Invoke-Docker @("exec", "nodebridge-mysql-edge-a", "mysql", "-uroot", "-proot_password", "-e", "GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'sync_user'@'%'; FLUSH PRIVILEGES;")
    $existing = & docker ps -a --filter "name=^/$canalContainer$" --format "{{.Names}}"
    if ($existing -contains $canalContainer) {
        Invoke-Docker @("rm", "-f", $canalContainer) | Out-Null
    }
    Invoke-Docker @(
        "run", "-d", "--name", $canalContainer, "-p", "${CanalPort}:11111",
        "-e", "canal.auto.scan=false", "-e", "canal.destinations=edge-001",
        "-e", "canal.instance.master.address=host.docker.internal:3307",
        "-e", "canal.instance.dbUsername=sync_user", "-e", "canal.instance.dbPassword=sync_password",
        "-e", "canal.instance.connectionCharset=UTF-8", "-e", "canal.instance.tsdb.enable=false",
        "-e", "canal.instance.gtidon=false", "-e", "canal.instance.mysql.slaveId=34601",
        "-e", "canal.instance.filter.regex=scada_edge\..*", $CanalImage
    ) | Out-Null
    Wait-TcpEndpoint "127.0.0.1" $CanalPort
    Start-Sleep -Seconds 5
}

if (-not $SkipPrepare) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
    Assert-LastExit "lab-smoke.ps1"
}

Write-TestFiles
Reset-TestState
Start-TestCanal

$checks = [System.Collections.Generic.List[object]]::new()
try {
    Write-Host "MCP governance E2E: INSERT on Edge A"
    $insert = @{ operation = "INSERT"; table = "device_config"; values = @{
        id = 46001; name = "governance-e2e"; value = "created"; sync_version = 1;
        updated_by_node = "edge-001"; last_event_id = "mcp-insert-46001"; updated_at = "2026-09-08 13:00:00.000"
    }}
    $insertPlan = Invoke-MCP $edgeAConfig "nodebridge_mysql_mutation_plan" $insert
    Assert-Equal "insert plan rows" "1" "$($insertPlan.matched_rows)"
    $insert.confirm = $true
    $insert.expected_rows = 1
    $insertResult = Invoke-MCP $edgeAConfig "nodebridge_mysql_mutation_apply" $insert
    Assert-Equal "insert status" "applied" $insertResult.status
    Transfer-EdgeAToEdgeB
    $edgeBRow = Invoke-MCP $edgeBConfig "nodebridge_mysql_query" @{ table = "device_settings"; columns = @("setting_id", "setting_value"); filters = @(@{ column = "setting_id"; operator = "="; value = 46001 }); limit = 1 }
    Assert-Equal "edge-b inserted value" "created" "$($edgeBRow.rows[0].setting_value)"
    $checks.Add([ordered]@{ name = "mcp_insert_edge_a_to_server_to_edge_b"; passed = $true })

    Write-Host "MCP governance E2E: ADD COLUMN on Edge A"
    $add = @{ operation = "ADD_COLUMN"; table = "device_config"; column = @{ name = "governed_note"; type = "varchar(64)"; nullable = $true; default = @{ mode = "null" }; comment = "NodeBridge governance E2E" } }
    $addPlan = Invoke-MCP $edgeAConfig "nodebridge_mysql_schema_change_plan" $add
    $addResult = Invoke-MCP $edgeAConfig "nodebridge_mysql_schema_change_apply" @{ change = $add; plan_id = $addPlan.plan_id; confirm = $true }
    Assert-Equal "add column status" "applied" $addResult.status
    Transfer-EdgeAToEdgeB
    foreach ($target in @(
        @{ Container = "nodebridge-mysql-edge-a"; Database = "scada_edge"; Table = "device_config" },
        @{ Container = "nodebridge-mysql-server"; Database = "scada_center"; Table = "device_settings" },
        @{ Container = "nodebridge-mysql-edge-b"; Database = "scada_edge"; Table = "device_settings" }
    )) {
        $count = Invoke-MySQL $target.Container "information_schema" "SELECT COUNT(*) FROM COLUMNS WHERE TABLE_SCHEMA='$($target.Database)' AND TABLE_NAME='$($target.Table)' AND COLUMN_NAME='governed_note';" -Scalar
        Assert-Equal "$($target.Container) add column" "1" $count
    }
    $checks.Add([ordered]@{ name = "mcp_add_column_edge_a_to_server_to_edge_b"; passed = $true })

    Write-Host "MCP governance E2E: UPDATE new column on Edge A"
    $update = @{ operation = "UPDATE"; table = "device_config"; values = @{ governed_note = "propagated-by-mcp"; value = "updated"; last_event_id = "mcp-update-46001" }; filters = @(@{ column = "id"; operator = "="; value = 46001 }) }
    $updatePlan = Invoke-MCP $edgeAConfig "nodebridge_mysql_mutation_plan" $update
    Assert-Equal "update plan rows" "1" "$($updatePlan.matched_rows)"
    $update.confirm = $true
    $update.expected_rows = 1
    $updateResult = Invoke-MCP $edgeAConfig "nodebridge_mysql_mutation_apply" $update
    Assert-Equal "update status" "applied" $updateResult.status
    Transfer-EdgeAToEdgeB
    $serverRow = Invoke-MCP $serverConfig "nodebridge_mysql_query" @{ table = "device_settings"; columns = @("setting_value", "governed_note"); filters = @(@{ column = "setting_id"; operator = "="; value = 46001 }); limit = 1 }
    $edgeBRow = Invoke-MCP $edgeBConfig "nodebridge_mysql_query" @{ table = "device_settings"; columns = @("setting_value", "governed_note"); filters = @(@{ column = "setting_id"; operator = "="; value = 46001 }); limit = 1 }
    Assert-Equal "server updated note" "propagated-by-mcp" "$($serverRow.rows[0].governed_note)"
    Assert-Equal "edge-b updated note" "propagated-by-mcp" "$($edgeBRow.rows[0].governed_note)"
    $checks.Add([ordered]@{ name = "mcp_update_new_column_edge_a_to_server_to_edge_b"; passed = $true })

    Write-Host "MCP governance E2E: DROP COLUMN on Edge A"
    $drop = @{ operation = "DROP_COLUMN"; table = "device_config"; column = @{ name = "governed_note"; type = "varchar(64)"; nullable = $true } }
    $dropPlan = Invoke-MCP $edgeAConfig "nodebridge_mysql_schema_change_plan" $drop
    $dropResult = Invoke-MCP $edgeAConfig "nodebridge_mysql_schema_change_apply" @{ change = $drop; plan_id = $dropPlan.plan_id; confirm = $true }
    Assert-Equal "drop column status" "applied" $dropResult.status
    Transfer-EdgeAToEdgeB
    foreach ($target in @(
        @{ Container = "nodebridge-mysql-edge-a"; Database = "scada_edge"; Table = "device_config" },
        @{ Container = "nodebridge-mysql-server"; Database = "scada_center"; Table = "device_settings" },
        @{ Container = "nodebridge-mysql-edge-b"; Database = "scada_edge"; Table = "device_settings" }
    )) {
        $count = Invoke-MySQL $target.Container "information_schema" "SELECT COUNT(*) FROM COLUMNS WHERE TABLE_SCHEMA='$($target.Database)' AND TABLE_NAME='$($target.Table)' AND COLUMN_NAME='governed_note';" -Scalar
        Assert-Equal "$($target.Container) drop column" "0" $count
    }
    $checks.Add([ordered]@{ name = "mcp_drop_column_edge_a_to_server_to_edge_b"; passed = $true })

    $auditPath = Join-Path $work "logs/mcp-audit.log"
    $auditActions = if (Test-Path $auditPath) { (Select-String -Path $auditPath -Pattern 'nodebridge_mysql_(mutation|schema_change)_apply').Count } else { 0 }
    if ($auditActions -lt 4) {
        throw "expected at least four governed write audit records, got $auditActions"
    }
    $checks.Add([ordered]@{ name = "governance_write_audit"; passed = $true; records = $auditActions })

    $report = [ordered]@{
        passed = $true
        topology = [ordered]@{ edge_a = "MySQL 3307 / RabbitMQ 5673 / Canal $CanalPort"; server = "MySQL 3309 / RabbitMQ 5675"; edge_b = "MySQL 3308 / RabbitMQ 5674" }
        automatic_schema_scope = @("ADD_COLUMN", "DROP_COLUMN")
        automatic_table_create = $false
        checks = $checks
        created_at = [DateTimeOffset]::Now.ToString("o")
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host "governance schema E2E passed: $reportPath"
} finally {
    if (-not $KeepCanal) {
        $existing = & docker ps -a --filter "name=^/$canalContainer$" --format "{{.Names}}"
        if ($existing -contains $canalContainer) {
            & docker rm -f $canalContainer | Out-Null
        }
    }
}
