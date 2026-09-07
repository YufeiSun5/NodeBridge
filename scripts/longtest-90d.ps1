param(
    [ValidateSet("prepare", "reset", "smoke", "seed-90d", "realtime-24h", "recovery", "mixed-offline", "query", "archive", "collect", "start-agents", "stop-agents", "status")]
    [string]$Action = "smoke",
    [string]$RunId = "",
    [int]$RowsPerTable = 2,
    [int]$Days = 90,
    [int]$BatchSize = 500,
    [ValidateSet("time-interleaved", "table-sequential")]
    [string]$InsertMode = "time-interleaved",
    [int]$DrainIterations = 120,
    [ValidateSet("once", "agents")]
    [string]$DrainMode = "once",
    [int]$AgentPollSeconds = 30,
    [int]$AgentStallMinutes = 30,
    [ValidateSet("1h", "1d", "7d", "30d")]
    [string]$RecoveryScenario = "1h",
    [int]$RealtimeHours = 24,
    [switch]$ConfirmLongRun
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$ComposeFile = Join-Path $Root "deploy/docker-compose.longtest.yml"
$EdgeConfig = Join-Path $Root "configs/longtest/edge.local.yaml"
$ServerConfig = Join-Path $Root "configs/longtest/server.local.yaml"
$Rules = Join-Path $Root "configs/longtest/sync-rules.yaml"
$AgentPath = Join-Path $Root ".cache/longtest-90d/bin/SyncAgent.exe"
$EdgeRabbitURL = "amqp://sync:sync_password@127.0.0.1:5701/edge-longtest-sync"
$ServerRabbitURL = "amqp://sync:sync_password@127.0.0.1:5702/server-longtest-sync"
$EdgeContainer = "nodebridge-longtest-mysql-edge"
$ServerContainer = "nodebridge-longtest-mysql-server"
$EdgeRabbitContainer = "nodebridge-longtest-rabbitmq-edge"
$ServerRabbitContainer = "nodebridge-longtest-rabbitmq-server"
$CanalContainer = "nodebridge-longtest-canal-edge"
$EdgeDB = "scada_edge_longtest"
$ServerDB = "scada_center_longtest"
$EdgeNodeID = "edge-longtest-001"
$TableNames = 1..6 | ForEach-Object { "collect_data_{0:D2}" -f $_ }
$MixedAppendTableNames = 1..12 | ForEach-Object { "collect_data_{0:D2}" -f $_ }
$MixedTagTableNames = 1..5 | ForEach-Object { "tag_state_{0:D2}" -f $_ }
$MixedTagRowsPerTable = 1000
$Culture = [Globalization.CultureInfo]::InvariantCulture
$DockerExe = (Get-Command docker -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source)
if (-not $DockerExe) {
    $dockerCandidates = @(
        "C:\Program Files\Docker\Docker\resources\bin\docker.exe",
        "C:\ProgramData\DockerDesktop\version-bin\docker.exe"
    )
    $DockerExe = $dockerCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if (-not $DockerExe) {
    throw "docker CLI not found. Add Docker to PATH or install Docker Desktop."
}
$GoExe = (Get-Command go -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source)
if (-not $GoExe) {
    $goCandidates = @(
        (Join-Path $Root ".tools/go1.25.5/go/bin/go.exe"),
        "C:\Program Files\Go\bin\go.exe"
    )
    $GoExe = $goCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if (-not $GoExe) {
    throw "go CLI not found. Add Go to PATH or install project toolchain under .tools."
}
$GoRoot = Split-Path -Parent (Split-Path -Parent $GoExe)
if (-not (Test-Path -LiteralPath (Join-Path $GoRoot "src/context"))) {
    throw "Go standard library not found under GOROOT candidate: $GoRoot"
}
$env:GOROOT = $GoRoot
$env:GOTOOLCHAIN = "local"

if ($RunId -eq "") {
    $RunId = Get-Date -Format "yyyyMMdd-HHmmss"
}
$EvidenceRoot = Join-Path $Root ".cache/longtest-90d/$RunId"
New-Item -ItemType Directory -Force -Path $EvidenceRoot | Out-Null
$AgentCommandLog = Join-Path $EvidenceRoot "agent-commands.log"

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Invoke-Repo {
    param([scriptblock]$Block)
    Push-Location $Root
    try {
        & $Block
    } finally {
        Pop-Location
    }
}

function Invoke-Docker {
    param([string[]]$Arguments)
    & $DockerExe @Arguments
    Assert-LastExit "docker $($Arguments -join ' ')"
}

function Invoke-Compose {
    param([string[]]$Arguments)
    $full = @("compose", "-f", $ComposeFile) + $Arguments
    Invoke-Docker $full
}

function Build-Agent {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentPath) | Out-Null
    Invoke-Repo { & $GoExe build -o $AgentPath ./cmd/sync-agent }
    if ($LASTEXITCODE -ne 0) {
        if (Test-Path -LiteralPath $AgentPath) {
            $warning = "go build failed; reusing existing SyncAgent binary at $AgentPath"
            Write-Warning $warning
            Write-JsonEvidence "build-warning.json" ([pscustomobject]@{
                warning = $warning
                agent_path = $AgentPath
                agent_last_write_time = (Get-Item -LiteralPath $AgentPath).LastWriteTime.ToString("o")
            })
            return
        }
        Assert-LastExit "go build ./cmd/sync-agent"
    }
}

function Invoke-Agent {
    param([string[]]$Arguments)
    $startedAt = Get-Date
    $timer = [Diagnostics.Stopwatch]::StartNew()
    Add-Content -Path $AgentCommandLog -Encoding UTF8 -Value ("`n[{0}] sync-agent {1}" -f $startedAt.ToString("o"), ($Arguments -join " "))
    Invoke-Repo {
        if (Test-Path -LiteralPath $AgentPath) {
            & $AgentPath @Arguments 2>&1 | Tee-Object -FilePath $AgentCommandLog -Append
        } else {
            & $GoExe run ./cmd/sync-agent @Arguments 2>&1 | Tee-Object -FilePath $AgentCommandLog -Append
        }
        $exitCode = $LASTEXITCODE
        $timer.Stop()
        Add-Content -Path $AgentCommandLog -Encoding UTF8 -Value ("[{0}] exit={1} elapsed_ms={2}" -f (Get-Date).ToString("o"), $exitCode, [Math]::Round($timer.Elapsed.TotalMilliseconds, 0))
        if ($exitCode -ne 0) {
            throw "sync-agent failed with exit code ${exitCode}: $($Arguments -join ' ')"
        }
    }
}

function Wait-Tcp {
    param([string]$HostName, [int]$Port, [string]$Name)
    for ($i = 0; $i -lt 90; $i++) {
        $client = [Net.Sockets.TcpClient]::new()
        try {
            $iar = $client.BeginConnect($HostName, $Port, $null, $null)
            if ($iar.AsyncWaitHandle.WaitOne([TimeSpan]::FromSeconds(2))) {
                $client.EndConnect($iar)
                return
            }
        } catch {
        } finally {
            $client.Close()
        }
        Start-Sleep -Seconds 2
    }
    throw "$Name endpoint not ready: ${HostName}:$Port"
}

function Wait-MySQL {
    param([string]$Container)
    for ($i = 0; $i -lt 90; $i++) {
        & $DockerExe exec $Container sh -c "mysqladmin ping -usync_user -psync_password --silent 2>/dev/null" | Out-Null
        if ($LASTEXITCODE -eq 0) { return }
        Start-Sleep -Seconds 2
    }
    throw "MySQL container not ready: $Container"
}

function Wait-RabbitMQ {
    param([string]$Container)
    for ($i = 0; $i -lt 90; $i++) {
        & $DockerExe exec $Container rabbitmq-diagnostics -q ping 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            & $DockerExe exec $Container rabbitmqctl await_startup 2>$null | Out-Null
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "RabbitMQ container not ready: $Container"
}

function Invoke-MySQL {
    param([string]$Container, [string]$Database, [string]$Sql)
    $tmp = Join-Path $env:TEMP ("nodebridge-longtest-" + [Guid]::NewGuid().ToString("N") + ".sql")
    [IO.File]::WriteAllText($tmp, $Sql, [Text.UTF8Encoding]::new($false))
    try {
        Invoke-Docker @("cp", $tmp, "${Container}:/tmp/nodebridge-longtest.sql")
        Invoke-Docker @("exec", $Container, "sh", "-c", "mysql -usync_user -psync_password $Database < /tmp/nodebridge-longtest.sql")
    } finally {
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-Scalar {
    param([string]$Container, [string]$Database, [string]$Query)
    $out = & $DockerExe exec $Container mysql -usync_user -psync_password -N -B $Database -e $Query
    Assert-LastExit "mysql scalar $Container $Database"
    return (($out | Select-Object -First 1) -as [string]).Trim()
}

function Write-JsonEvidence {
    param([string]$Name, [object]$Value)
    $path = Join-Path $EvidenceRoot $Name
    [IO.File]::WriteAllText($path, ($Value | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
}

function Write-TextEvidence {
    param([string]$Name, [scriptblock]$Block)
    $path = Join-Path $EvidenceRoot $Name
    try {
        & $Block 2>&1 | Set-Content -Path $path -Encoding UTF8
    } catch {
        $_ | Out-String | Set-Content -Path $path -Encoding UTF8
    }
}

function Get-PointColumns {
    1..42 | ForEach-Object { "p{0:D3}" -f $_ }
}

function New-CollectTableSql {
    param([string]$Table, [switch]$Archive)
    $pointColumns = (Get-PointColumns | ForEach-Object { "  $_ DOUBLE NULL" }) -join ",`n"
    $primaryKey = if ($Archive) { "PRIMARY KEY (id, collected_at)" } else { "PRIMARY KEY (id)" }
    return @"
CREATE TABLE IF NOT EXISTS $Table (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  device_id VARCHAR(64) NOT NULL,
  collected_at DATETIME(3) NOT NULL,
$pointColumns,
  updated_by_node VARCHAR(64) NULL,
  last_event_id VARCHAR(128) NULL,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  $primaryKey,
  KEY idx_device_time (device_id, collected_at),
  KEY idx_time (collected_at)
);
"@
}

function New-TagStateTableSql {
    param([string]$Table)
    return @"
CREATE TABLE IF NOT EXISTS $Table (
  id BIGINT UNSIGNED NOT NULL,
  tag_key VARCHAR(64) NOT NULL,
  tag_value DOUBLE NULL,
  quality VARCHAR(16) NOT NULL,
  version BIGINT UNSIGNED NOT NULL,
  updated_by_node VARCHAR(64) NULL,
  last_event_id VARCHAR(128) NULL,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_tag_key (tag_key),
  KEY idx_updated_at (updated_at)
);
"@
}

function Initialize-CollectTables {
    $sql = New-Object Text.StringBuilder
    foreach ($table in $TableNames) {
        [void]$sql.AppendLine((New-CollectTableSql -Table $table))
    }
    Invoke-MySQL $EdgeContainer $EdgeDB $sql.ToString()
    Invoke-MySQL $ServerContainer $ServerDB $sql.ToString()
}

function Initialize-MixedTables {
    $sql = New-Object Text.StringBuilder
    foreach ($table in $MixedAppendTableNames) {
        [void]$sql.AppendLine((New-CollectTableSql -Table $table))
    }
    foreach ($table in $MixedTagTableNames) {
        [void]$sql.AppendLine((New-TagStateTableSql -Table $table))
    }
    Invoke-MySQL $EdgeContainer $EdgeDB $sql.ToString()
    Invoke-MySQL $ServerContainer $ServerDB $sql.ToString()
}

function Write-MixedRules {
    $path = Join-Path $EvidenceRoot "mixed-sync-rules.yaml"
    $yaml = New-Object Text.StringBuilder
    [void]$yaml.AppendLine("rules:")
    foreach ($table in $MixedAppendTableNames) {
        [void]$yaml.AppendLine("  - id: $table-upload")
        [void]$yaml.AppendLine("    database_name: $EdgeDB")
        [void]$yaml.AppendLine("    table_name: $table")
        [void]$yaml.AppendLine("    target_database_name: $ServerDB")
        [void]$yaml.AppendLine("    target_table_name: $table")
        [void]$yaml.AppendLine("    direction: EDGE_TO_SERVER")
        [void]$yaml.AppendLine("    dispatch_target: NONE")
        [void]$yaml.AppendLine("    sync_mode: append_only")
        [void]$yaml.AppendLine("    conflict_policy: NONE")
        [void]$yaml.AppendLine("    enable: true")
        [void]$yaml.AppendLine("    primary_keys: [id]")
        [void]$yaml.AppendLine("")
    }
    foreach ($table in $MixedTagTableNames) {
        [void]$yaml.AppendLine("  - id: $table-upload")
        [void]$yaml.AppendLine("    database_name: $EdgeDB")
        [void]$yaml.AppendLine("    table_name: $table")
        [void]$yaml.AppendLine("    target_database_name: $ServerDB")
        [void]$yaml.AppendLine("    target_table_name: $table")
        [void]$yaml.AppendLine("    direction: EDGE_TO_SERVER")
        [void]$yaml.AppendLine("    dispatch_target: NONE")
        [void]$yaml.AppendLine("    sync_mode: crud_ordered")
        [void]$yaml.AppendLine("    conflict_policy: LAST_WRITE_WIN")
        [void]$yaml.AppendLine("    enable: true")
        [void]$yaml.AppendLine("    primary_keys: [id]")
        [void]$yaml.AppendLine("")
    }
    [IO.File]::WriteAllText($path, $yaml.ToString(), [Text.UTF8Encoding]::new($false))
    Set-Variable -Name Rules -Scope Script -Value $path
    return $path
}

function Reset-LongtestData {
    & $DockerExe rm -f $CanalContainer 2>$null | Out-Null

    $truncate = New-Object Text.StringBuilder
    foreach ($table in $TableNames) {
        [void]$truncate.AppendLine("TRUNCATE TABLE $table;")
    }
    [void]$truncate.AppendLine("TRUNCATE TABLE sync_upload_offset;")
    [void]$truncate.AppendLine("TRUNCATE TABLE sync_apply_log;")
    Invoke-MySQL $EdgeContainer $EdgeDB $truncate.ToString()

    $server = New-Object Text.StringBuilder
    foreach ($table in $TableNames) {
        [void]$server.AppendLine("TRUNCATE TABLE $table;")
    }
    [void]$server.AppendLine("TRUNCATE TABLE sync_upload_offset;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_ack_log;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_dispatch_log;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_event_log;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_apply_log;")
    Invoke-MySQL $ServerContainer $ServerDB $server.ToString()

    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q")) {
        & $DockerExe exec $EdgeRabbitContainer rabbitmqctl purge_queue -p edge-longtest-sync $queue 2>$null | Out-Null
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "$EdgeNodeID.downlink.q")) {
        & $DockerExe exec $ServerRabbitContainer rabbitmqctl purge_queue -p server-longtest-sync $queue 2>$null | Out-Null
    }

    Invoke-Compose @("up", "-d", "--force-recreate", "canal-edge-longtest")
    Wait-Tcp "127.0.0.1" 11121 "edge canal"
    Assert-LongtestCanalConfig
    Start-Sleep -Seconds 10
}

function Reset-MixedData {
    & $DockerExe rm -f $CanalContainer 2>$null | Out-Null

    Initialize-MixedTables
    $edge = New-Object Text.StringBuilder
    foreach ($table in $MixedAppendTableNames + $MixedTagTableNames) {
        [void]$edge.AppendLine("TRUNCATE TABLE $table;")
    }
    [void]$edge.AppendLine("TRUNCATE TABLE sync_upload_offset;")
    [void]$edge.AppendLine("TRUNCATE TABLE sync_apply_log;")
    Invoke-MySQL $EdgeContainer $EdgeDB $edge.ToString()

    $server = New-Object Text.StringBuilder
    foreach ($table in $MixedAppendTableNames + $MixedTagTableNames) {
        [void]$server.AppendLine("TRUNCATE TABLE $table;")
    }
    [void]$server.AppendLine("TRUNCATE TABLE sync_upload_offset;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_ack_log;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_dispatch_log;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_event_log;")
    [void]$server.AppendLine("TRUNCATE TABLE sync_apply_log;")
    Invoke-MySQL $ServerContainer $ServerDB $server.ToString()

    foreach ($queue in @("edge.upload.cdc.q", "edge.upload.retry.q", "edge.dead.q", "edge.downlink.q")) {
        & $DockerExe exec $EdgeRabbitContainer rabbitmqctl purge_queue -p edge-longtest-sync $queue 2>$null | Out-Null
    }
    foreach ($queue in @("server.cdc.ingress.q", "server.dead.q", "$EdgeNodeID.downlink.q")) {
        & $DockerExe exec $ServerRabbitContainer rabbitmqctl purge_queue -p server-longtest-sync $queue 2>$null | Out-Null
    }

    Invoke-Compose @("up", "-d", "--force-recreate", "canal-edge-longtest")
    Wait-Tcp "127.0.0.1" 11121 "edge canal"
    Assert-LongtestCanalConfig
    Start-Sleep -Seconds 10
}

function Grant-CanalPrivileges {
    $grant = "GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'sync_user'@'%'; FLUSH PRIVILEGES;"
    Invoke-Docker @("exec", $EdgeContainer, "mysql", "-uroot", "-proot_password", "-e", $grant)
}

function Assert-LongtestCanalConfig {
    $config = (& $DockerExe exec $CanalContainer sh -c "cat /home/admin/canal-server/conf/edge-longtest-001/instance.properties" 2>$null) -join "`n"
    Write-TextEvidence "canal-instance.properties.txt" { $config }
    if ($config -notmatch "canal\.instance\.master\.address=mysql-edge-longtest:3306") {
        throw "longtest Canal config mismatch: master address is not mysql-edge-longtest:3306"
    }
    if ($config -notmatch "canal\.instance\.filter\.regex=scada_edge_longtest\\\.\(collect_data_\.\*\|tag_state_\.\*\)") {
        throw "longtest Canal config mismatch: mixed collect/tag filter not applied"
    }
}

function Initialize-Topology {
    Invoke-Agent @("migrate", "-config", $EdgeConfig, "-scope", "edge")
    Invoke-Agent @("migrate", "-config", $ServerConfig, "-scope", "server")
    Initialize-CollectTables
    Invoke-Agent @("init-rabbitmq", "-mode", "edge", "-amqp-url", $EdgeRabbitURL)
    Invoke-Agent @("init-rabbitmq", "-mode", "server", "-amqp-url", $ServerRabbitURL, "-edges", $EdgeNodeID)
}

function Get-QueueDepth {
    param([string]$Container, [string]$VHost, [string]$Queue)
    $rows = & $DockerExe exec $Container rabbitmqctl -q list_queues -p $VHost name messages 2>$null
    if ($LASTEXITCODE -ne 0) { return -1 }
    foreach ($row in $rows) {
        $parts = (($row -as [string]) -split "\s+")
        if ($parts.Length -ge 2 -and $parts[0] -eq $Queue) {
            return [int]$parts[1]
        }
    }
    return 0
}

function Get-TotalRows {
    param([string]$Container, [string]$Database)
    $parts = $TableNames | ForEach-Object { "(SELECT COUNT(1) FROM $_)" }
    return [int64](Invoke-Scalar $Container $Database ("SELECT " + ($parts -join " + ") + ";"))
}

function Get-RowCounts {
    $rows = foreach ($table in $TableNames) {
        [pscustomobject]@{
            table = $table
            edge = [int64](Invoke-Scalar $EdgeContainer $EdgeDB "SELECT COUNT(1) FROM $table;")
            server = [int64](Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM $table;")
        }
    }
    Write-JsonEvidence "row-counts.json" $rows
    return $rows
}

function Get-MixedRowCounts {
    $rows = foreach ($table in $MixedAppendTableNames + $MixedTagTableNames) {
        [pscustomobject]@{
            table = $table
            edge = [int64](Invoke-Scalar $EdgeContainer $EdgeDB "SELECT COUNT(1) FROM $table;")
            server = [int64](Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM $table;")
        }
    }
    Write-JsonEvidence "mixed-row-counts.json" $rows
    return $rows
}

function Get-MixedTotalRows {
    param([string]$Container, [string]$Database)
    $parts = ($MixedAppendTableNames + $MixedTagTableNames) | ForEach-Object { "(SELECT COUNT(1) FROM $_)" }
    return [int64](Invoke-Scalar $Container $Database ("SELECT " + ($parts -join " + ") + ";"))
}

function New-InsertSql {
    param([string]$Table, [int]$Count, [int64]$StartIndex, [datetime]$StartTime)
    $pointColumns = Get-PointColumns
    $columns = @("device_id", "collected_at") + $pointColumns + @("updated_by_node", "last_event_id", "created_at", "updated_at")
    $values = New-Object Collections.Generic.List[string]
    for ($i = 0; $i -lt $Count; $i++) {
        $rowIndex = $StartIndex + $i
        $device = "device-{0:D3}" -f (($rowIndex % 100) + 1)
        $dt = $StartTime.AddSeconds(3 * $rowIndex).ToString("yyyy-MM-dd HH:mm:ss.fff", $Culture)
        $points = foreach ($p in 1..42) {
            [string]::Format($Culture, "{0:0.0000}", (($rowIndex % 1000) * 0.01 + $p))
        }
        $values.Add("('$device','$dt'," + ($points -join ",") + ",'$EdgeNodeID',NULL,'$dt','$dt')")
    }
    return "INSERT INTO $Table (" + ($columns -join ",") + ") VALUES " + ($values -join ",") + ";"
}

function Insert-CollectRows {
    param([int]$RowsEachTable, [int64]$StartIndex = 0)
    $start = [datetime]"2026-01-01T00:00:00"
    $plan = [pscustomobject]@{
        mode = $InsertMode
        rows_each_table = $RowsEachTable
        start_index = $StartIndex
        batch_size = $BatchSize
        tables = $TableNames
        ordering = if ($InsertMode -eq "time-interleaved") {
            "advance by collected_at window; write all tables for the same window before moving to the next window"
        } else {
            "complete one table before moving to the next table"
        }
    }
    Write-JsonEvidence "insert-plan.json" $plan

    if ($InsertMode -eq "table-sequential") {
        foreach ($table in $TableNames) {
            $done = 0
            while ($done -lt $RowsEachTable) {
                $take = [Math]::Min($BatchSize, $RowsEachTable - $done)
                $sql = New-InsertSql -Table $table -Count $take -StartIndex ($StartIndex + $done) -StartTime $start
                Invoke-MySQL $EdgeContainer $EdgeDB $sql
                $done += $take
                if ($done % ([Math]::Max($BatchSize * 20, 1)) -eq 0) {
                    Write-Host "inserted mode=table-sequential table=$table rows=$done/$RowsEachTable"
                }
            }
        }
        return
    }

    $done = 0
    while ($done -lt $RowsEachTable) {
        $take = [Math]::Min($BatchSize, $RowsEachTable - $done)
        $windowStart = $StartIndex + $done
        foreach ($table in $TableNames) {
            $sql = New-InsertSql -Table $table -Count $take -StartIndex $windowStart -StartTime $start
            Invoke-MySQL $EdgeContainer $EdgeDB $sql
        }
        $done += $take
        if ($done % ([Math]::Max($BatchSize * 20, 1)) -eq 0 -or $done -ge $RowsEachTable) {
            Write-Host "inserted mode=time-interleaved tables=$($TableNames.Count) rows_each_table=$done/$RowsEachTable"
        }
    }
}

function New-TagInitialSql {
    param([string]$Table)
    $values = New-Object Collections.Generic.List[string]
    $dt = ([datetime]"2026-01-01T00:00:00").ToString("yyyy-MM-dd HH:mm:ss.fff", $Culture)
    for ($i = 1; $i -le $MixedTagRowsPerTable; $i++) {
        $key = "$Table-tag-{0:D4}" -f $i
        $values.Add("($i,'$key',0.0000,'GOOD',0,'$EdgeNodeID',NULL,'$dt','$dt')")
    }
    return "INSERT INTO $Table (id,tag_key,tag_value,quality,version,updated_by_node,last_event_id,created_at,updated_at) VALUES " + ($values -join ",") + ";"
}

function Insert-MixedOfflineRows {
    param([int]$DaysForRun)
    $appendRowsEach = $DaysForRun * 24 * 60 * 20
    $tagUpdatesEach = $appendRowsEach
    $start = [datetime]"2026-01-01T00:00:00"
    $plan = [pscustomobject]@{
        mode = "mixed-offline"
        days = $DaysForRun
        append_tables = $MixedAppendTableNames
        append_rows_each_table = $appendRowsEach
        tag_tables = $MixedTagTableNames
        tag_rows_each_table = $MixedTagRowsPerTable
        tag_updates_each_table = $tagUpdatesEach
        batch_size = $BatchSize
        expected_business_rows = [int64]($appendRowsEach * $MixedAppendTableNames.Count + $MixedTagRowsPerTable * $MixedTagTableNames.Count)
        expected_events = [int64]($appendRowsEach * $MixedAppendTableNames.Count + ($MixedTagRowsPerTable + $tagUpdatesEach) * $MixedTagTableNames.Count)
        offline_boundary = "server RabbitMQ stopped before inserts and updates; Edge agent stays running"
    }
    Write-JsonEvidence "mixed-insert-plan.json" $plan

    foreach ($table in $MixedTagTableNames) {
        Invoke-MySQL $EdgeContainer $EdgeDB (New-TagInitialSql -Table $table)
    }

    $done = 0
    while ($done -lt $appendRowsEach) {
        $take = [Math]::Min($BatchSize, $appendRowsEach - $done)
        $windowStart = $done
        foreach ($table in $MixedAppendTableNames) {
            $sql = New-InsertSql -Table $table -Count $take -StartIndex $windowStart -StartTime $start
            Invoke-MySQL $EdgeContainer $EdgeDB $sql
        }
        foreach ($table in $MixedTagTableNames) {
            $updates = New-Object Text.StringBuilder
            for ($i = 0; $i -lt $take; $i++) {
                $global = $windowStart + $i
                $id = [int](($global % $MixedTagRowsPerTable) + 1)
                $version = [int64]($global + 1)
                $dt = $start.AddSeconds(3 * $global).ToString("yyyy-MM-dd HH:mm:ss.fff", $Culture)
                $value = [string]::Format($Culture, "{0:0.0000}", (($global % 100000) * 0.001))
                [void]$updates.AppendLine("UPDATE $table SET tag_value=$value, quality='GOOD', version=$version, updated_by_node='$EdgeNodeID', updated_at='$dt' WHERE id=$id;")
            }
            Invoke-MySQL $EdgeContainer $EdgeDB $updates.ToString()
        }
        $done += $take
        if ($done % ([Math]::Max($BatchSize * 20, 1)) -eq 0 -or $done -ge $appendRowsEach) {
            Write-Host "inserted mixed-offline append_tables=$($MixedAppendTableNames.Count) tag_tables=$($MixedTagTableNames.Count) rows_each_append=$done/$appendRowsEach tag_updates_each=$done/$tagUpdatesEach"
        }
    }
    return $plan
}

function Test-CollectOrdering {
    param([string]$Container = $EdgeContainer, [string]$Database = $EdgeDB)
    $results = foreach ($table in $TableNames) {
        $violations = Invoke-Scalar $Container $Database @"
SELECT COUNT(1)
FROM (
  SELECT collected_at, LAG(collected_at) OVER (ORDER BY id) AS previous_collected_at
  FROM $table
) ordered_rows
WHERE previous_collected_at IS NOT NULL AND collected_at < previous_collected_at;
"@
        [pscustomobject]@{
            table = $table
            collected_at_order_violations = [int64]$violations
        }
    }
    Write-JsonEvidence "ordering-check.json" $results
    $bad = $results | Where-Object { $_.collected_at_order_violations -ne 0 }
    if ($bad) {
        throw "collect_data ordering check failed"
    }
    return $results
}

function Test-CollectArrivalShape {
    param([string]$Container = $EdgeContainer, [string]$Database = $EdgeDB)
    $union = ($TableNames | ForEach-Object {
        "SELECT collected_at, '$($_)' AS table_name FROM $_"
    }) -join " UNION ALL "
    $missing = Invoke-Scalar $Container $Database @"
SELECT COUNT(1)
FROM (
  SELECT collected_at, COUNT(DISTINCT table_name) AS table_count
  FROM ($union) all_rows
  GROUP BY collected_at
  HAVING table_count <> $($TableNames.Count)
) bad_windows;
"@
    $result = [pscustomobject]@{
        mode = $InsertMode
        expected_tables_per_collected_at = $TableNames.Count
        bad_collected_at_windows = [int64]$missing
    }
    Write-JsonEvidence "arrival-shape.json" $result
    if ($InsertMode -eq "time-interleaved" -and [int64]$missing -ne 0) {
        throw "time-interleaved arrival shape check failed"
    }
    return $result
}

function Test-CollectDataShape {
    Test-CollectOrdering | Out-Null
    if ($InsertMode -eq "time-interleaved") {
        Test-CollectArrivalShape | Out-Null
    }
}

function Test-MixedTagVersions {
    param([int]$DaysForRun, [string]$Container = $ServerContainer, [string]$Database = $ServerDB)
    $updates = $DaysForRun * 24 * 60 * 20
    $results = foreach ($table in $MixedTagTableNames) {
        $bad = Invoke-Scalar $Container $Database @"
SELECT COUNT(1)
FROM $table
WHERE version <> CASE
  WHEN $updates >= id THEN id + FLOOR(($updates - id) / $MixedTagRowsPerTable) * $MixedTagRowsPerTable
  ELSE 0
END;
"@
        $minVersion = Invoke-Scalar $Container $Database "SELECT COALESCE(MIN(version),0) FROM $table;"
        $maxVersion = Invoke-Scalar $Container $Database "SELECT COALESCE(MAX(version),0) FROM $table;"
        [pscustomobject]@{
            table = $table
            expected_updates = $updates
            version_mismatches = [int64]$bad
            min_version = [int64]$minVersion
            max_version = [int64]$maxVersion
        }
    }
    Write-JsonEvidence "mixed-tag-version-check.json" $results
    $badRows = $results | Where-Object { $_.version_mismatches -ne 0 }
    if ($badRows) {
        throw "mixed tag version check failed"
    }
    return $results
}

function Test-MixedAppendOrdering {
    param([string]$Container = $ServerContainer, [string]$Database = $ServerDB)
    $results = foreach ($table in $MixedAppendTableNames) {
        $violations = Invoke-Scalar $Container $Database @"
SELECT COUNT(1)
FROM (
  SELECT collected_at, LAG(collected_at) OVER (ORDER BY id) AS previous_collected_at
  FROM $table
) ordered_rows
WHERE previous_collected_at IS NOT NULL AND collected_at < previous_collected_at;
"@
        [pscustomobject]@{
            table = $table
            collected_at_order_violations = [int64]$violations
        }
    }
    Write-JsonEvidence "mixed-append-ordering-check.json" $results
    $bad = $results | Where-Object { $_.collected_at_order_violations -ne 0 }
    if ($bad) {
        throw "mixed append ordering check failed"
    }
    return $results
}

function Drain-Once {
    param([int]$MaxBatch = 1000)
    Invoke-Agent @("canal-publish-once", "-config", $EdgeConfig, "-rules", $Rules, "-amqp-url", $EdgeRabbitURL)
    Invoke-Agent @("forward-upload-batch-once", "-local-amqp-url", $EdgeRabbitURL, "-server-amqp-url", $ServerRabbitURL, "-max-batch", "$MaxBatch", "-flush-interval-millis", "500")
    Invoke-Agent @("consume-batch-once", "-config", $ServerConfig, "-rules", $Rules, "-amqp-url", $ServerRabbitURL, "-edges", $EdgeNodeID, "-max-batch", "$MaxBatch", "-flush-interval-millis", "500", "-requeue-on-error=true")
}

function Drain-Until {
    param([int64]$ExpectedServerRows, [int]$MaxIterations)
    $samples = New-Object Collections.Generic.List[object]
    $lastServerRows = -1
    $stalledIterations = 0
    $maxStalledIterations = 60
    $completed = $false
    for ($i = 1; $i -le $MaxIterations; $i++) {
        Drain-Once -MaxBatch ([Math]::Max($BatchSize, 1000))
        $edgeDepth = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
        $serverDepth = Get-QueueDepth $ServerRabbitContainer "server-longtest-sync" "server.cdc.ingress.q"
        $serverRows = Get-TotalRows $ServerContainer $ServerDB
        $samples.Add([pscustomobject]@{
            at = (Get-Date).ToString("o")
            iteration = $i
            edge_upload_depth = $edgeDepth
            server_ingress_depth = $serverDepth
            server_rows = $serverRows
        })
        if ($serverRows -gt $lastServerRows) {
            $lastServerRows = $serverRows
            $stalledIterations = 0
        } else {
            $stalledIterations++
        }
        if ($serverRows -ge $ExpectedServerRows -and $edgeDepth -eq 0 -and $serverDepth -eq 0) {
            $completed = $true
            break
        }
        if ($stalledIterations -ge $maxStalledIterations -and ($edgeDepth -gt 0 -or $serverDepth -gt 0)) {
            break
        }
        Start-Sleep -Milliseconds 300
    }
    $csv = Join-Path $EvidenceRoot "queue-depth-samples.csv"
    $samples | Export-Csv -Path $csv -NoTypeInformation -Encoding UTF8
    $lastSample = $samples | Select-Object -Last 1
    Write-JsonEvidence "drain-final.json" ([pscustomobject]@{
        expected_server_rows = $ExpectedServerRows
        completed = $completed
        max_iterations = $MaxIterations
        samples = $samples.Count
        last_sample = $lastSample
        stalled_iterations = $stalledIterations
    })
    if (-not $completed) {
        throw "drain did not complete expected_server_rows=$ExpectedServerRows last_server_rows=$($lastSample.server_rows) edge_depth=$($lastSample.edge_upload_depth) server_depth=$($lastSample.server_ingress_depth) iterations=$($samples.Count)"
    }
}

function Start-AgentProcesses {
    $agentDir = Join-Path $EvidenceRoot "agents"
    New-Item -ItemType Directory -Force -Path $agentDir | Out-Null
    $edgeStop = Join-Path $agentDir "edge.stop"
    $serverStop = Join-Path $agentDir "server.stop"
    Remove-Item -LiteralPath $edgeStop, $serverStop -Force -ErrorAction SilentlyContinue

    $edgeOut = Join-Path $agentDir "edge.out.log"
    $edgeErr = Join-Path $agentDir "edge.err.log"
    $serverOut = Join-Path $agentDir "server.out.log"
    $serverErr = Join-Path $agentDir "server.err.log"

    $edge = Start-Process -FilePath $AgentPath -ArgumentList @("run", "-config", $EdgeConfig, "-rules", $Rules, "-stop-file", $edgeStop) -RedirectStandardOutput $edgeOut -RedirectStandardError $edgeErr -PassThru -WindowStyle Hidden
    $server = Start-Process -FilePath $AgentPath -ArgumentList @("run", "-config", $ServerConfig, "-rules", $Rules, "-edges", $EdgeNodeID, "-stop-file", $serverStop) -RedirectStandardOutput $serverOut -RedirectStandardError $serverErr -PassThru -WindowStyle Hidden

    $agents = [pscustomobject]@{
        edge_pid = $edge.Id
        server_pid = $server.Id
        edge_stop_file = $edgeStop
        server_stop_file = $serverStop
        edge_stdout = $edgeOut
        edge_stderr = $edgeErr
        server_stdout = $serverOut
        server_stderr = $serverErr
        started_at = (Get-Date).ToString("o")
    }
    Write-JsonEvidence "agents.json" $agents
    return [pscustomobject]@{ edge = $edge; server = $server; info = $agents }
}

function Start-EdgeAgentProcess {
    $agentDir = Join-Path $EvidenceRoot "agents"
    New-Item -ItemType Directory -Force -Path $agentDir | Out-Null
    $edgeStop = Join-Path $agentDir "edge.stop"
    Remove-Item -LiteralPath $edgeStop -Force -ErrorAction SilentlyContinue
    $edgeOut = Join-Path $agentDir "edge.out.log"
    $edgeErr = Join-Path $agentDir "edge.err.log"
    $edge = Start-Process -FilePath $AgentPath -ArgumentList @("run", "-config", $EdgeConfig, "-rules", $Rules, "-stop-file", $edgeStop) -RedirectStandardOutput $edgeOut -RedirectStandardError $edgeErr -PassThru -WindowStyle Hidden
    $info = [pscustomobject]@{
        edge_pid = $edge.Id
        edge_stop_file = $edgeStop
        edge_stdout = $edgeOut
        edge_stderr = $edgeErr
        started_at = (Get-Date).ToString("o")
    }
    Write-JsonEvidence "edge-agent.json" $info
    return [pscustomobject]@{ edge = $edge; info = $info }
}

function Stop-EdgeAgentProcess {
    param([object]$Agent)
    if (-not $Agent) { return }
    New-Item -ItemType File -Force -Path $Agent.info.edge_stop_file | Out-Null
    if ($Agent.edge -and -not $Agent.edge.HasExited) {
        try { $Agent.edge.WaitForExit(15000) | Out-Null } catch {}
        if (-not $Agent.edge.HasExited) {
            try { Stop-Process -Id $Agent.edge.Id -Force -ErrorAction Stop } catch {}
        }
    }
}

function Stop-AgentProcesses {
    param([object]$Agents)
    if (-not $Agents) { return }
    New-Item -ItemType File -Force -Path $Agents.info.edge_stop_file | Out-Null
    New-Item -ItemType File -Force -Path $Agents.info.server_stop_file | Out-Null
    foreach ($proc in @($Agents.edge, $Agents.server)) {
        if ($proc -and -not $proc.HasExited) {
            try { $proc.WaitForExit(15000) | Out-Null } catch {}
            if (-not $proc.HasExited) {
                try { Stop-Process -Id $proc.Id -Force -ErrorAction Stop } catch {}
            }
        }
    }
}

function Drain-WithAgents {
    param([int64]$ExpectedServerRows, [int]$MaxSamples)
    $agents = $null
    $csv = Join-Path $EvidenceRoot "queue-depth-samples.csv"
    "at,sample,edge_rows,server_rows,edge_upload_depth,server_ingress_depth,edge_pid,server_pid" | Set-Content -Path $csv -Encoding UTF8
    $lastServerRows = -1
    $lastProgressAt = Get-Date
    $completed = $false
    $lastSample = $null
    try {
        $agents = Start-AgentProcesses
        for ($i = 1; $i -le $MaxSamples; $i++) {
            Start-Sleep -Seconds $AgentPollSeconds
            $edgeDepth = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
            $serverDepth = Get-QueueDepth $ServerRabbitContainer "server-longtest-sync" "server.cdc.ingress.q"
            $edgeRows = Get-TotalRows $EdgeContainer $EdgeDB
            $serverRows = Get-TotalRows $ServerContainer $ServerDB
            $now = Get-Date
            $lastSample = [pscustomobject]@{
                at = $now.ToString("o")
                sample = $i
                edge_rows = $edgeRows
                server_rows = $serverRows
                edge_upload_depth = $edgeDepth
                server_ingress_depth = $serverDepth
                edge_pid = $agents.edge.Id
                server_pid = $agents.server.Id
            }
            Add-Content -Path $csv -Encoding UTF8 -Value ("{0},{1},{2},{3},{4},{5},{6},{7}" -f $lastSample.at, $i, $edgeRows, $serverRows, $edgeDepth, $serverDepth, $agents.edge.Id, $agents.server.Id)

            if ($serverRows -gt $lastServerRows) {
                $lastServerRows = $serverRows
                $lastProgressAt = $now
            }
            if ($serverRows -ge $ExpectedServerRows -and $edgeDepth -eq 0 -and $serverDepth -eq 0) {
                $completed = $true
                break
            }
            if ($agents.edge.HasExited -or $agents.server.HasExited) {
                throw "agent exited before drain completed edge_exited=$($agents.edge.HasExited) server_exited=$($agents.server.HasExited)"
            }
            if (($now - $lastProgressAt).TotalMinutes -ge $AgentStallMinutes -and ($edgeDepth -gt 0 -or $serverDepth -gt 0 -or $serverRows -lt $ExpectedServerRows)) {
                throw "agent drain stalled for $AgentStallMinutes minutes server_rows=$serverRows expected=$ExpectedServerRows edge_depth=$edgeDepth server_depth=$serverDepth"
            }
        }
    } finally {
        Stop-AgentProcesses -Agents $agents
    }

    Write-JsonEvidence "drain-final.json" ([pscustomobject]@{
        mode = "agents"
        expected_server_rows = $ExpectedServerRows
        completed = $completed
        max_samples = $MaxSamples
        poll_seconds = $AgentPollSeconds
        stall_minutes = $AgentStallMinutes
        last_sample = $lastSample
    })
    if (-not $completed) {
        throw "agent drain did not complete expected_server_rows=$ExpectedServerRows last_server_rows=$($lastSample.server_rows) edge_depth=$($lastSample.edge_upload_depth) server_depth=$($lastSample.server_ingress_depth) samples=$($lastSample.sample)"
    }
}

function Drain-MixedWithAgents {
    param([int64]$ExpectedServerRows, [int64]$ExpectedApplyLogRows, [int]$MaxSamples)
    $agents = $null
    $csv = Join-Path $EvidenceRoot "mixed-drain-samples.csv"
    "at,sample,edge_rows,server_rows,apply_log_rows,edge_upload_depth,server_ingress_depth,edge_pid,server_pid" | Set-Content -Path $csv -Encoding UTF8
    $lastApplyRows = -1
    $lastProgressAt = Get-Date
    $completed = $false
    $lastSample = $null
    try {
        $agents = Start-AgentProcesses
        for ($i = 1; $i -le $MaxSamples; $i++) {
            Start-Sleep -Seconds $AgentPollSeconds
            $edgeDepth = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
            $serverDepth = Get-QueueDepth $ServerRabbitContainer "server-longtest-sync" "server.cdc.ingress.q"
            $edgeRows = Get-MixedTotalRows $EdgeContainer $EdgeDB
            $serverRows = Get-MixedTotalRows $ServerContainer $ServerDB
            $applyRows = [int64](Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM sync_apply_log;")
            $now = Get-Date
            $lastSample = [pscustomobject]@{
                at = $now.ToString("o")
                sample = $i
                edge_rows = $edgeRows
                server_rows = $serverRows
                apply_log_rows = $applyRows
                edge_upload_depth = $edgeDepth
                server_ingress_depth = $serverDepth
                edge_pid = $agents.edge.Id
                server_pid = $agents.server.Id
            }
            Add-Content -Path $csv -Encoding UTF8 -Value ("{0},{1},{2},{3},{4},{5},{6},{7},{8}" -f $lastSample.at, $i, $edgeRows, $serverRows, $applyRows, $edgeDepth, $serverDepth, $agents.edge.Id, $agents.server.Id)

            if ($applyRows -gt $lastApplyRows) {
                $lastApplyRows = $applyRows
                $lastProgressAt = $now
            }
            if ($serverRows -ge $ExpectedServerRows -and $applyRows -ge $ExpectedApplyLogRows -and $edgeDepth -eq 0 -and $serverDepth -eq 0) {
                $completed = $true
                break
            }
            if ($agents.edge.HasExited -or $agents.server.HasExited) {
                throw "agent exited before mixed drain completed edge_exited=$($agents.edge.HasExited) server_exited=$($agents.server.HasExited)"
            }
            if (($now - $lastProgressAt).TotalMinutes -ge $AgentStallMinutes -and ($edgeDepth -gt 0 -or $serverDepth -gt 0 -or $applyRows -lt $ExpectedApplyLogRows)) {
                throw "mixed agent drain stalled for $AgentStallMinutes minutes apply_rows=$applyRows expected_apply=$ExpectedApplyLogRows edge_depth=$edgeDepth server_depth=$serverDepth"
            }
        }
    } finally {
        Stop-AgentProcesses -Agents $agents
    }

    Write-JsonEvidence "mixed-drain-final.json" ([pscustomobject]@{
        mode = "agents"
        expected_server_rows = $ExpectedServerRows
        expected_apply_log_rows = $ExpectedApplyLogRows
        completed = $completed
        max_samples = $MaxSamples
        poll_seconds = $AgentPollSeconds
        stall_minutes = $AgentStallMinutes
        last_sample = $lastSample
    })
    if (-not $completed) {
        throw "mixed agent drain did not complete expected_server_rows=$ExpectedServerRows expected_apply_log_rows=$ExpectedApplyLogRows last_server_rows=$($lastSample.server_rows) last_apply_rows=$($lastSample.apply_log_rows) edge_depth=$($lastSample.edge_upload_depth) server_depth=$($lastSample.server_ingress_depth) samples=$($lastSample.sample)"
    }
}

function Drain-ExpectedRows {
    param([int64]$ExpectedServerRows, [int]$MaxIterations)
    if ($DrainMode -eq "agents") {
        Drain-WithAgents -ExpectedServerRows $ExpectedServerRows -MaxSamples $MaxIterations
    } else {
        Drain-Until -ExpectedServerRows $ExpectedServerRows -MaxIterations $MaxIterations
    }
}

function Collect-Evidence {
    Copy-Item -LiteralPath $ComposeFile -Destination (Join-Path $EvidenceRoot "docker-compose.longtest.yml") -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $EdgeConfig -Destination (Join-Path $EvidenceRoot "edge.local.yaml") -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $ServerConfig -Destination (Join-Path $EvidenceRoot "server.local.yaml") -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $Rules -Destination (Join-Path $EvidenceRoot "sync-rules.yaml") -Force -ErrorAction SilentlyContinue

    Write-TextEvidence "git-status.txt" { git status --short }
    Write-TextEvidence "docker-version.txt" { & $DockerExe version }
    Write-TextEvidence "docker-info.txt" { & $DockerExe info }
    Write-TextEvidence "docker-compose-config.txt" { & $DockerExe compose -f $ComposeFile config }
    Write-TextEvidence "docker-ps.txt" { & $DockerExe ps --filter "name=nodebridge-longtest" }
    Write-TextEvidence "docker-stats.txt" { & $DockerExe stats --no-stream $EdgeContainer $ServerContainer $EdgeRabbitContainer $ServerRabbitContainer $CanalContainer }
    Write-TextEvidence "systeminfo.txt" { systeminfo }
    Write-JsonEvidence "hardware.json" ([pscustomobject]@{
        collected_at = (Get-Date).ToString("o")
        cpu = Get-CimInstance Win32_Processor | Select-Object Name,NumberOfCores,NumberOfLogicalProcessors,MaxClockSpeed
        computer = Get-CimInstance Win32_ComputerSystem | Select-Object Manufacturer,Model,TotalPhysicalMemory
        os = Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,BuildNumber,OSArchitecture,FreePhysicalMemory,TotalVisibleMemorySize
        disks = Get-CimInstance Win32_LogicalDisk | Select-Object DeviceID,DriveType,Size,FreeSpace
    })
    Write-JsonEvidence "mysql-status.json" ([pscustomobject]@{
        edge_data_dir = (& $DockerExe exec $EdgeContainer sh -c "du -sh /var/lib/mysql 2>/dev/null" 2>$null) -join "`n"
        server_data_dir = (& $DockerExe exec $ServerContainer sh -c "du -sh /var/lib/mysql 2>/dev/null" 2>$null) -join "`n"
        edge_binlogs = (& $DockerExe exec $EdgeContainer sh -c "ls -lh /var/lib/mysql/mysql-bin.* 2>/dev/null | tail -20" 2>$null) -join "`n"
        server_binlogs = (& $DockerExe exec $ServerContainer sh -c "ls -lh /var/lib/mysql/mysql-bin.* 2>/dev/null | tail -20" 2>$null) -join "`n"
    })
    Get-RowCounts | Out-Null
    $queues = [pscustomobject]@{
        edge_upload_depth = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
        server_ingress_depth = Get-QueueDepth $ServerRabbitContainer "server-longtest-sync" "server.cdc.ingress.q"
        collected_at = (Get-Date).ToString("o")
    }
    Write-JsonEvidence "queue-depth.json" $queues
    foreach ($container in @($EdgeContainer, $ServerContainer, $EdgeRabbitContainer, $ServerRabbitContainer, $CanalContainer)) {
        $logPath = Join-Path $EvidenceRoot "$container.log"
        & cmd.exe /c "`"$DockerExe`" logs $container 2>&1" | Set-Content -Path $logPath -Encoding UTF8
    }
}

function Prepare-Longtest {
    Build-Agent
    Invoke-Compose @("up", "-d", "--remove-orphans")
    Wait-MySQL $EdgeContainer
    Wait-MySQL $ServerContainer
    Wait-RabbitMQ $EdgeRabbitContainer
    Wait-RabbitMQ $ServerRabbitContainer
    Grant-CanalPrivileges
    Invoke-Docker @("restart", $CanalContainer)
    Wait-Tcp "127.0.0.1" 11121 "edge canal"
    Assert-LongtestCanalConfig
    Initialize-Topology
    Write-JsonEvidence "summary.json" ([pscustomobject]@{
        action = "prepare"
        run_id = $RunId
        status = "prepared"
        compose = $ComposeFile
    })
}

function Invoke-Smoke {
    Build-Agent
    Reset-LongtestData
    Insert-CollectRows -RowsEachTable $RowsPerTable
    Test-CollectDataShape
    $expected = [int64]($RowsPerTable * $TableNames.Count)
    Drain-ExpectedRows -ExpectedServerRows $expected -MaxIterations $DrainIterations
    $counts = Get-RowCounts
    $duplicates = Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM (SELECT event_id FROM sync_event_log GROUP BY event_id HAVING COUNT(1) > 1) d;"
    $failed = Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED';"
    $mismatched = @($counts | Where-Object { $_.edge -ne $_.server })
    Write-JsonEvidence "summary.json" ([pscustomobject]@{
        action = "smoke"
        insert_mode = $InsertMode
        drain_mode = $DrainMode
        rows_per_table = $RowsPerTable
        expected_total = $expected
        duplicate_event_ids = [int]$duplicates
        failed_ack_count = [int]$failed
        mismatched_table_count = $mismatched.Count
        counts = $counts
    })
    if ([int]$duplicates -ne 0 -or [int]$failed -ne 0 -or $mismatched.Count -ne 0) {
        throw "smoke failed duplicate_event_ids=$duplicates failed_ack_count=$failed mismatched_table_count=$($mismatched.Count)"
    }
}

function Invoke-Seed90d {
    if (-not $ConfirmLongRun) { throw "seed-90d requires -ConfirmLongRun" }
    $rowsPerTableForDays = $Days * 24 * 60 * 20
    $total = [int64]$rowsPerTableForDays * $TableNames.Count
    $timer = [Diagnostics.Stopwatch]::StartNew()
    try {
        Build-Agent
        Reset-LongtestData
        Insert-CollectRows -RowsEachTable $rowsPerTableForDays
        Test-CollectDataShape
        Drain-ExpectedRows -ExpectedServerRows $total -MaxIterations $DrainIterations
        $timer.Stop()
        Collect-Evidence
        Write-JsonEvidence "summary.json" ([pscustomobject]@{
            action = "seed-90d"
            status = "passed"
            insert_mode = $InsertMode
            drain_mode = $DrainMode
            days = $Days
            rows_per_table = $rowsPerTableForDays
            expected_total = $total
            elapsed_seconds = [Math]::Round($timer.Elapsed.TotalSeconds, 2)
        })
    } catch {
        $timer.Stop()
        $errorMessage = $_.Exception.Message
        Write-TextEvidence "failure.txt" { $errorMessage }
        Collect-Evidence
        Write-JsonEvidence "summary.json" ([pscustomobject]@{
            action = "seed-90d"
            status = "failed"
            insert_mode = $InsertMode
            drain_mode = $DrainMode
            days = $Days
            rows_per_table = $rowsPerTableForDays
            expected_total = $total
            elapsed_seconds = [Math]::Round($timer.Elapsed.TotalSeconds, 2)
            error = $errorMessage
        })
        throw
    }
}

function Invoke-Realtime24h {
    if (-not $ConfirmLongRun) { throw "realtime-24h requires -ConfirmLongRun" }
    Build-Agent
    Reset-LongtestData
    $ticks = $RealtimeHours * 60 * 20
    $latency = New-Object Collections.Generic.List[object]
    for ($i = 0; $i -lt $ticks; $i++) {
        Insert-CollectRows -RowsEachTable 1 -StartIndex $i
        Drain-ExpectedRows -ExpectedServerRows (($i + 1) * $TableNames.Count) -MaxIterations 10
        $maxLag = Invoke-Scalar $ServerContainer $ServerDB "SELECT COALESCE(MAX(TIMESTAMPDIFF(SECOND, collected_at, NOW(3))),0) FROM collect_data_01;"
        $latency.Add([pscustomobject]@{ at = (Get-Date).ToString("o"); tick = $i + 1; max_lag_seconds = [int]$maxLag })
        Start-Sleep -Seconds 3
    }
    $latency | Export-Csv -Path (Join-Path $EvidenceRoot "latency-samples.csv") -NoTypeInformation -Encoding UTF8
    Collect-Evidence
}

function Invoke-Recovery {
    if (-not $ConfirmLongRun) { throw "recovery requires -ConfirmLongRun" }
    Build-Agent
    $totals = @{ "1h" = 43200; "1d" = 172800; "7d" = 1209600; "30d" = 5184000 }
    $totalRows = [int]$totals[$RecoveryScenario]
    $rowsEach = [int]($totalRows / $TableNames.Count)
    Reset-LongtestData
    Invoke-Docker @("stop", $ServerRabbitContainer)
    Insert-CollectRows -RowsEachTable $rowsEach
    Test-CollectDataShape
    for ($i = 0; $i -lt [Math]::Ceiling($totalRows / 1000); $i++) {
        Invoke-Agent @("canal-publish-once", "-config", $EdgeConfig, "-rules", $Rules, "-amqp-url", $EdgeRabbitURL)
    }
    $edgeDepthBefore = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
    Invoke-Docker @("start", $ServerRabbitContainer)
    Wait-RabbitMQ $ServerRabbitContainer
    Invoke-Agent @("init-rabbitmq", "-mode", "server", "-amqp-url", $ServerRabbitURL, "-edges", $EdgeNodeID)
    Drain-ExpectedRows -ExpectedServerRows $totalRows -MaxIterations $DrainIterations
    Collect-Evidence
    Write-JsonEvidence "summary.json" ([pscustomobject]@{
        action = "recovery"
        insert_mode = $InsertMode
        drain_mode = $DrainMode
        scenario = $RecoveryScenario
        expected_total = $totalRows
        rows_per_table = $rowsEach
        edge_depth_before_recovery = $edgeDepthBefore
    })
}

function Invoke-MixedOffline {
    if (-not $ConfirmLongRun) { throw "mixed-offline requires -ConfirmLongRun" }
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $edgeAgent = $null
    $serverWasStopped = $false
    try {
        Build-Agent
        Write-MixedRules | Out-Null
        Invoke-Compose @("up", "-d", "--remove-orphans")
        Wait-MySQL $EdgeContainer
        Wait-MySQL $ServerContainer
        Wait-RabbitMQ $EdgeRabbitContainer
        Wait-RabbitMQ $ServerRabbitContainer
        Grant-CanalPrivileges
        Invoke-Docker @("restart", $CanalContainer)
        Wait-Tcp "127.0.0.1" 11121 "edge canal"
        Assert-LongtestCanalConfig
        Invoke-Agent @("migrate", "-config", $EdgeConfig, "-scope", "edge")
        Invoke-Agent @("migrate", "-config", $ServerConfig, "-scope", "server")
        Initialize-MixedTables
        Invoke-Agent @("init-rabbitmq", "-mode", "edge", "-amqp-url", $EdgeRabbitURL)
        Invoke-Agent @("init-rabbitmq", "-mode", "server", "-amqp-url", $ServerRabbitURL, "-edges", $EdgeNodeID)
        Reset-MixedData

        $appendRowsEach = $Days * 24 * 60 * 20
        $expectedBusinessRows = [int64]($appendRowsEach * $MixedAppendTableNames.Count + $MixedTagRowsPerTable * $MixedTagTableNames.Count)
        $expectedEvents = [int64]($appendRowsEach * $MixedAppendTableNames.Count + ($MixedTagRowsPerTable + $appendRowsEach) * $MixedTagTableNames.Count)

        $edgeAgent = Start-EdgeAgentProcess
        Start-Sleep -Seconds 5
        if ($edgeAgent.edge.HasExited) {
            throw "edge agent exited before offline simulation"
        }
        Invoke-Docker @("stop", $ServerRabbitContainer)
        $serverWasStopped = $true
        Insert-MixedOfflineRows -DaysForRun $Days | Out-Null
        Test-MixedAppendOrdering -Container $EdgeContainer -Database $EdgeDB | Out-Null
        Test-MixedTagVersions -DaysForRun $Days -Container $EdgeContainer -Database $EdgeDB | Out-Null

        $offlineWaitSamples = New-Object Collections.Generic.List[object]
        $lastUploadDepth = -1
        $lastProgressAt = Get-Date
        for ($i = 1; $i -le $DrainIterations; $i++) {
            Start-Sleep -Seconds $AgentPollSeconds
            if ($edgeAgent.edge.HasExited) {
                throw "edge agent exited while server RabbitMQ was offline"
            }
            $edgeDepth = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
            $edgeApplyRows = [int64](Invoke-Scalar $EdgeContainer $EdgeDB "SELECT COUNT(1) FROM sync_apply_log;")
            $sample = [pscustomobject]@{
                at = (Get-Date).ToString("o")
                sample = $i
                edge_upload_depth = $edgeDepth
                edge_apply_log_rows = $edgeApplyRows
                expected_events = $expectedEvents
            }
            $offlineWaitSamples.Add($sample)
            if ($edgeDepth -gt $lastUploadDepth -or $edgeApplyRows -gt 0) {
                $lastUploadDepth = $edgeDepth
                $lastProgressAt = Get-Date
            }
            if ($edgeDepth -ge $expectedEvents) {
                break
            }
            if (((Get-Date) - $lastProgressAt).TotalMinutes -ge $AgentStallMinutes) {
                throw "edge offline backlog did not grow for $AgentStallMinutes minutes edge_depth=$edgeDepth expected_events=$expectedEvents"
            }
        }
        $offlineWaitSamples | Export-Csv -Path (Join-Path $EvidenceRoot "mixed-offline-backlog-samples.csv") -NoTypeInformation -Encoding UTF8
        $lastOffline = $offlineWaitSamples | Select-Object -Last 1
        if (-not $lastOffline -or [int64]$lastOffline.edge_upload_depth -lt $expectedEvents) {
            throw "edge offline backlog incomplete edge_depth=$($lastOffline.edge_upload_depth) expected_events=$expectedEvents"
        }

        Stop-EdgeAgentProcess -Agent $edgeAgent
        $edgeAgent = $null
        Invoke-Docker @("start", $ServerRabbitContainer)
        $serverWasStopped = $false
        Wait-RabbitMQ $ServerRabbitContainer
        Invoke-Agent @("init-rabbitmq", "-mode", "server", "-amqp-url", $ServerRabbitURL, "-edges", $EdgeNodeID)
        Drain-MixedWithAgents -ExpectedServerRows $expectedBusinessRows -ExpectedApplyLogRows $expectedEvents -MaxSamples $DrainIterations

        $counts = Get-MixedRowCounts
        Test-MixedAppendOrdering -Container $ServerContainer -Database $ServerDB | Out-Null
        Test-MixedTagVersions -DaysForRun $Days -Container $ServerContainer -Database $ServerDB | Out-Null
        $failed = Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM sync_ack_log WHERE status='FAILED';"
        $applyRows = Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM sync_apply_log;"
        $eventRows = Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM sync_event_log;"
        $timer.Stop()
        Collect-Evidence
        Write-JsonEvidence "mixed-summary.json" ([pscustomobject]@{
            action = "mixed-offline"
            status = "passed"
            days = $Days
            append_table_count = $MixedAppendTableNames.Count
            append_rows_each_table = $appendRowsEach
            tag_table_count = $MixedTagTableNames.Count
            tag_rows_each_table = $MixedTagRowsPerTable
            tag_updates_each_table = $appendRowsEach
            expected_business_rows = $expectedBusinessRows
            expected_events = $expectedEvents
            apply_log_rows = [int64]$applyRows
            event_log_rows = [int64]$eventRows
            failed_ack_count = [int64]$failed
            elapsed_seconds = [Math]::Round($timer.Elapsed.TotalSeconds, 2)
            counts = $counts
        })
        if ([int64]$failed -ne 0 -or [int64]$applyRows -ne $expectedEvents -or [int64]$eventRows -ne $expectedEvents) {
            throw "mixed audit failed failed_ack_count=$failed apply_log_rows=$applyRows event_log_rows=$eventRows expected_events=$expectedEvents"
        }
    } catch {
        $timer.Stop()
        $errorMessage = $_.Exception.Message
        Write-TextEvidence "mixed-failure.txt" { $errorMessage }
        Collect-Evidence
        Write-JsonEvidence "mixed-summary.json" ([pscustomobject]@{
            action = "mixed-offline"
            status = "failed"
            days = $Days
            elapsed_seconds = [Math]::Round($timer.Elapsed.TotalSeconds, 2)
            error = $errorMessage
        })
        throw
    } finally {
        Stop-EdgeAgentProcess -Agent $edgeAgent
        if ($serverWasStopped) {
            try {
                Invoke-Docker @("start", $ServerRabbitContainer)
                Wait-RabbitMQ $ServerRabbitContainer
            } catch {}
        }
    }
}

function Invoke-QueryTests {
    $results = New-Object Collections.Generic.List[object]
    $explains = New-Object Collections.Generic.List[object]
    $queries = @(
        @{ name = "single_device_single_day"; sql = "SELECT COUNT(1) FROM collect_data_01 WHERE device_id='device-001' AND collected_at >= '2026-01-01' AND collected_at < '2026-01-02';"; target_ms = 1000 },
        @{ name = "single_table_last_hour"; sql = "SELECT COUNT(1) FROM collect_data_01 WHERE collected_at >= NOW(3) - INTERVAL 1 HOUR;"; target_ms = 1000 },
        @{ name = "single_table_last_7_days"; sql = "SELECT COUNT(1) FROM collect_data_01 WHERE collected_at >= NOW(3) - INTERVAL 7 DAY;"; target_ms = 3000 },
        @{ name = "six_table_range_aggregate"; sql = (($TableNames | ForEach-Object { "SELECT COUNT(1) c FROM $_ WHERE collected_at >= '2026-01-01' AND collected_at < '2026-01-08'" }) -join " UNION ALL "); target_ms = 3000 }
    )
    foreach ($q in $queries) {
        $timer = [Diagnostics.Stopwatch]::StartNew()
        $value = Invoke-Scalar $ServerContainer $ServerDB $q.sql
        $timer.Stop()
        $results.Add([pscustomobject]@{ name = $q.name; elapsed_ms = [Math]::Round($timer.Elapsed.TotalMilliseconds, 2); target_ms = $q.target_ms; result = $value })
        $explain = & $DockerExe exec $ServerContainer mysql -usync_user -psync_password -N -B $ServerDB -e ("EXPLAIN " + $q.sql)
        $explains.Add([pscustomobject]@{ name = $q.name; explain = ($explain -join "`n") })
    }
    Write-JsonEvidence "query-results.json" $results
    Write-JsonEvidence "explain-results.json" $explains
}

function Invoke-ArchiveTests {
    $now = [datetime]"2026-04-01T00:00:00"
    $old = $now.AddDays(-91).ToString("yyyy-MM-dd HH:mm:ss.fff", $Culture)
    $warm = $now.AddDays(-45).ToString("yyyy-MM-dd HH:mm:ss.fff", $Culture)
    $hot = $now.AddDays(-5).ToString("yyyy-MM-dd HH:mm:ss.fff", $Culture)
    $pWarm = $now.AddDays(-60).ToString("yyyy-MM-dd", $Culture)
    $pHot = $now.AddDays(-30).ToString("yyyy-MM-dd", $Culture)
    $results = New-Object Collections.Generic.List[object]
    foreach ($table in $TableNames) {
        $archive = "${table}_archive"
        $create = New-CollectTableSql -Table $archive -Archive
        $sql = @"
DROP TABLE IF EXISTS $archive;
$create
ALTER TABLE $archive
PARTITION BY RANGE COLUMNS(collected_at) (
  PARTITION p_old VALUES LESS THAN ('$pWarm'),
  PARTITION p_warm VALUES LESS THAN ('$pHot'),
  PARTITION p_hot VALUES LESS THAN (MAXVALUE)
);
INSERT INTO $archive (id, device_id, collected_at, p001, updated_by_node, created_at, updated_at)
VALUES
  (1, 'archive-device', '$old', 1.0, '$EdgeNodeID', '$old', '$old'),
  (2, 'archive-device', '$warm', 2.0, '$EdgeNodeID', '$warm', '$warm'),
  (3, 'archive-device', '$hot', 3.0, '$EdgeNodeID', '$hot', '$hot');
ALTER TABLE $archive DROP PARTITION p_old;
"@
        Invoke-MySQL $ServerContainer $ServerDB $sql
        $remaining = Invoke-Scalar $ServerContainer $ServerDB "SELECT COUNT(1) FROM $archive;"
        $results.Add([pscustomobject]@{ table = $archive; drop_partition = "p_old"; remaining_rows = [int]$remaining })
    }
    Write-JsonEvidence "archive-results.json" $results
}

function Start-Agents {
    Build-Agent
    Start-AgentProcesses | Out-Null
}

function Stop-Agents {
    $agentFile = Join-Path $EvidenceRoot "agents.json"
    if (Test-Path -LiteralPath $agentFile) {
        $agents = Get-Content -Raw -Path $agentFile | ConvertFrom-Json
        New-Item -ItemType File -Force -Path $agents.edge_stop_file | Out-Null
        New-Item -ItemType File -Force -Path $agents.server_stop_file | Out-Null
    } else {
        Write-Host "agents.json not found for run_id=$RunId"
    }
}

switch ($Action) {
    "prepare" { Prepare-Longtest; Collect-Evidence }
    "reset" { Reset-LongtestData; Collect-Evidence }
    "smoke" { Invoke-Smoke; Collect-Evidence }
    "seed-90d" { Invoke-Seed90d }
    "realtime-24h" { Invoke-Realtime24h }
    "recovery" { Invoke-Recovery }
    "mixed-offline" { Invoke-MixedOffline }
    "query" { Invoke-QueryTests; Collect-Evidence }
    "archive" { Invoke-ArchiveTests; Collect-Evidence }
    "collect" { Collect-Evidence }
    "start-agents" { Start-Agents }
    "stop-agents" { Stop-Agents }
    "status" {
        Write-Host "run_id=$RunId evidence=$EvidenceRoot"
        Get-RowCounts | Format-Table -AutoSize
        [pscustomobject]@{
            edge_upload_depth = Get-QueueDepth $EdgeRabbitContainer "edge-longtest-sync" "edge.upload.cdc.q"
            server_ingress_depth = Get-QueueDepth $ServerRabbitContainer "server-longtest-sync" "server.cdc.ingress.q"
        } | Format-List
    }
}
