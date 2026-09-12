param(
    [ValidateSet("Start", "Run", "Status", "Stop")]
    [string]$Action = "Start",
    [string]$RunId = "",
    [double]$DurationHours = 13,
    [int]$RowsPerBatch = 200,
    [int]$IntervalSeconds = 5,
    [string]$EdgeHost = "192.168.10.105",
    [string]$EdgeUser = "xx",
    [string]$SSHKeyPath = "$HOME\.ssh\nodebridge_ed25519",
    [string]$DockerPath = "C:\Program Files\Docker\Docker\resources\bin\docker.exe",
    [string]$ServerMySQLContainer = "mysql-8.0.38",
    [string]$ServerRabbitMQContainer = "rabbitmq",
    [string]$ServerAgentPath = "",
    [switch]$Smoke
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Root = Split-Path -Parent $PSScriptRoot
$CacheRoot = Join-Path $Root ".cache\real-bidirectional-overnight"
$CurrentPath = Join-Path $CacheRoot "current.json"
$ServerConfigPath = "C:\ProgramData\NodeBridge\config.yaml"
$ServerRulesPath = "C:\ProgramData\NodeBridge\sync-rules.yaml"
$ServerStopPath = "C:\ProgramData\NodeBridge\run\sync-agent.stop"
$EdgeConfigPath = "C:\ProgramData\NodeBridge\config.yaml"
$EdgeRulesPath = "C:\ProgramData\NodeBridge\sync-rules.yaml"
$EdgeStopPath = "C:\ProgramData\NodeBridge\run\sync-agent.stop"
$EdgeAgentPath = "C:\Program Files\NodeBridge\app\SyncAgent.exe"
$ServerRabbitVHost = "/nodebridge-server"
$ServerAMQPURL = "amqp://nb-server-sync:1234@127.0.0.1:5672/%2Fnodebridge-server"

if ([string]::IsNullOrWhiteSpace($ServerAgentPath)) {
    $ServerAgentPath = Join-Path $Root "build\bin\SyncAgent.exe"
}

function Write-JsonAtomic {
    param([Parameter(Mandatory)][string]$Path, [Parameter(Mandatory)][object]$Value)
    $directory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    $temporary = "$Path.$PID.tmp"
    [IO.File]::WriteAllText($temporary, ($Value | ConvertTo-Json -Depth 12), [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $temporary -Destination $Path -Force
}

function Read-JsonFile {
    param([Parameter(Mandatory)][string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $null
    }
    return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
}

function Get-EvidenceRoot {
    param([string]$RequestedRunId)
    if (-not [string]::IsNullOrWhiteSpace($RequestedRunId)) {
        return Join-Path $CacheRoot $RequestedRunId
    }
    $current = Read-JsonFile $CurrentPath
    if ($null -eq $current -or [string]::IsNullOrWhiteSpace($current.evidence_root)) {
        throw "no current overnight run"
    }
    return $current.evidence_root
}

function Test-ProcessAlive {
    param([int]$ProcessId)
    if ($ProcessId -le 0) { return $false }
    return $null -ne (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)
}

if ($Action -eq "Status") {
    $evidenceRoot = Get-EvidenceRoot $RunId
    $state = Read-JsonFile (Join-Path $evidenceRoot "state.json")
    if ($null -eq $state) { throw "state file is missing: $evidenceRoot" }
    $state | Add-Member -NotePropertyName runner_alive -NotePropertyValue (Test-ProcessAlive ([int]$state.runner_pid)) -Force
    $state | ConvertTo-Json -Depth 10
    exit 0
}

if ($Action -eq "Stop") {
    $evidenceRoot = Get-EvidenceRoot $RunId
    [IO.File]::WriteAllText((Join-Path $evidenceRoot "stop.requested"), (Get-Date).ToString("o"), [Text.UTF8Encoding]::new($false))
    [pscustomobject]@{ status = "stop_requested"; evidence_root = $evidenceRoot } | ConvertTo-Json
    exit 0
}

if ($Action -eq "Start") {
    if ($DurationHours -le 0) { throw "DurationHours must be positive" }
    if ($RowsPerBatch -le 0) { throw "RowsPerBatch must be positive" }
    if ($IntervalSeconds -le 0) { throw "IntervalSeconds must be positive" }
    if (-not (Test-Path -LiteralPath $SSHKeyPath -PathType Leaf)) { throw "SSH key not found: $SSHKeyPath" }
    if (-not (Test-Path -LiteralPath $DockerPath -PathType Leaf)) { throw "Docker CLI not found: $DockerPath" }
    if (-not (Test-Path -LiteralPath $ServerAgentPath -PathType Leaf)) { throw "Server agent not found: $ServerAgentPath" }

    New-Item -ItemType Directory -Force -Path $CacheRoot | Out-Null
    $current = Read-JsonFile $CurrentPath
    if ($null -ne $current) {
        $currentState = Read-JsonFile (Join-Path $current.evidence_root "state.json")
        if ($null -ne $currentState -and (Test-ProcessAlive ([int]$currentState.runner_pid)) -and $currentState.status -in @("starting", "running", "draining")) {
            throw "overnight run is already active: $($current.run_id) PID $($currentState.runner_pid)"
        }
    }

    if ([string]::IsNullOrWhiteSpace($RunId)) {
        $kind = if ($Smoke) { "smoke" } else { "overnight" }
        $RunId = "{0}-{1}" -f $kind, (Get-Date -Format "yyyyMMdd-HHmmss")
    }
    if ($RunId -notmatch '^[A-Za-z0-9_-]+$') { throw "RunId contains unsupported characters" }
    $evidenceRoot = Join-Path $CacheRoot $RunId
    if (Test-Path -LiteralPath $evidenceRoot) { throw "run already exists: $RunId" }
    New-Item -ItemType Directory -Force -Path $evidenceRoot | Out-Null

    $initialState = [ordered]@{
        run_id = $RunId
        status = "starting"
        phase = "launcher"
        runner_pid = 0
        started_at = (Get-Date).ToString("o")
        heartbeat_at = (Get-Date).ToString("o")
        planned_duration_hours = $DurationHours
        rows_per_batch = $RowsPerBatch
        interval_seconds = $IntervalSeconds
        smoke = [bool]$Smoke
        evidence_root = $evidenceRoot
    }
    Write-JsonAtomic (Join-Path $evidenceRoot "state.json") $initialState
    Write-JsonAtomic $CurrentPath ([ordered]@{ run_id = $RunId; evidence_root = $evidenceRoot; updated_at = (Get-Date).ToString("o") })

    $hostExe = (Get-Process -Id $PID).Path
    $argumentText = "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" -Action Run -RunId $RunId -DurationHours $($DurationHours.ToString([Globalization.CultureInfo]::InvariantCulture)) -RowsPerBatch $RowsPerBatch -IntervalSeconds $IntervalSeconds -EdgeHost $EdgeHost -EdgeUser $EdgeUser -SSHKeyPath `"$SSHKeyPath`" -DockerPath `"$DockerPath`" -ServerMySQLContainer $ServerMySQLContainer -ServerRabbitMQContainer $ServerRabbitMQContainer -ServerAgentPath `"$ServerAgentPath`""
    if ($Smoke) { $argumentText += " -Smoke" }
    $runner = Start-Process -FilePath $hostExe -ArgumentList $argumentText -RedirectStandardOutput (Join-Path $evidenceRoot "runner.stdout.log") -RedirectStandardError (Join-Path $evidenceRoot "runner.stderr.log") -WindowStyle Hidden -PassThru

    $deadline = (Get-Date).AddMinutes(3)
    do {
        Start-Sleep -Seconds 1
        $state = Read-JsonFile (Join-Path $evidenceRoot "state.json")
        if ($null -ne $state -and [int]$state.runner_pid -eq $runner.Id -and $state.phase -ne "launcher") { break }
        if ($runner.HasExited) {
            $stderr = Get-Content (Join-Path $evidenceRoot "runner.stderr.log") -Raw -ErrorAction SilentlyContinue
            throw "overnight runner exited during startup: $stderr"
        }
    } while ((Get-Date) -lt $deadline)
    if ($null -eq $state -or [int]$state.runner_pid -ne $runner.Id -or $state.phase -eq "launcher") {
        throw "overnight runner did not initialize within 3 minutes"
    }
    [pscustomobject]@{
        run_id = $RunId
        status = $state.status
        phase = $state.phase
        runner_pid = $runner.Id
        evidence_root = $evidenceRoot
        planned_end_at = $state.planned_end_at
    } | ConvertTo-Json -Depth 5
    exit 0
}

# Run action starts here.
$EvidenceRoot = Get-EvidenceRoot $RunId
$StatePath = Join-Path $EvidenceRoot "state.json"
$EventsPath = Join-Path $EvidenceRoot "events.jsonl"
$SnapshotsPath = Join-Path $EvidenceRoot "snapshots.jsonl"
$StopRequestPath = Join-Path $EvidenceRoot "stop.requested"
$script:State = [ordered]@{}
$script:EdgeSession = $null
$script:ServerSession = $null
$script:Restored = $false
$script:RunError = $null
$script:OriginalServerAgentPath = ""

function Set-State {
    param([hashtable]$Values)
    foreach ($entry in $Values.GetEnumerator()) {
        $script:State[$entry.Key] = $entry.Value
    }
    $script:State.heartbeat_at = (Get-Date).ToString("o")
    Write-JsonAtomic $StatePath $script:State
}

function Add-Event {
    param([string]$Name, [string]$Status = "ok", [object]$Data = $null)
    $record = [ordered]@{ at = (Get-Date).ToString("o"); name = $Name; status = $Status; data = $Data }
    Add-Content -LiteralPath $EventsPath -Value ($record | ConvertTo-Json -Depth 10 -Compress) -Encoding UTF8
}

function Invoke-External {
    param([Parameter(Mandatory)][string]$FilePath, [Parameter(Mandatory)][string[]]$Arguments, [string]$Label = "command")
    $output = & $FilePath @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "$Label failed with exit code ${LASTEXITCODE}: $($output | Out-String)"
    }
    return @($output)
}

function Invoke-RemotePowerShell {
    param([Parameter(Mandatory)][string]$Script)
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes("`$ProgressPreference='SilentlyContinue'; " + $Script))
    return Invoke-External "ssh.exe" @("-T", "-i", $SSHKeyPath, "-o", "BatchMode=yes", "-o", "ConnectTimeout=8", "$EdgeUser@$EdgeHost", "powershell.exe", "-NoProfile", "-EncodedCommand", $encoded) "remote PowerShell"
}

function Copy-FromEdge {
    param([string]$RemotePath, [string]$LocalPath)
    Invoke-External "scp.exe" @("-q", "-i", $SSHKeyPath, "-o", "BatchMode=yes", "$EdgeUser@${EdgeHost}:$($RemotePath.Replace('\', '/'))", $LocalPath) "scp from edge" | Out-Null
}

function Copy-ToEdge {
    param([string]$LocalPath, [string]$RemotePath)
    Invoke-External "scp.exe" @("-q", "-i", $SSHKeyPath, "-o", "BatchMode=yes", $LocalPath, "$EdgeUser@${EdgeHost}:$($RemotePath.Replace('\', '/'))") "scp to edge" | Out-Null
}

function New-RedirectedProcess {
    param([string]$FilePath, [string]$Arguments)
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $FilePath
    $start.Arguments = $Arguments
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardInput = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    if (-not $process.Start()) { throw "failed to start $FilePath" }
    return $process
}

function New-EdgeMySQLSession {
    $remote = @'
$env:MYSQL_PWD = "root"
& "C:\Program Files\MySQL\MySQL Server 8.4\bin\mysql.exe" -uroot -N -B --raw --unbuffered
'@
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($remote))
    $arguments = "-T -i `"$SSHKeyPath`" -o BatchMode=yes -o ConnectTimeout=8 $EdgeUser@$EdgeHost powershell.exe -NoProfile -EncodedCommand $encoded"
    return New-RedirectedProcess "ssh.exe" $arguments
}

function New-ServerMySQLSession {
    $arguments = "exec -i -e MYSQL_PWD=root $ServerMySQLContainer mysql -uroot -N -B --raw --unbuffered"
    return New-RedirectedProcess $DockerPath $arguments
}

function Close-Session {
    param([Diagnostics.Process]$Process)
    if ($null -eq $Process) { return }
    try {
        $Process.StandardInput.Close()
        if (-not $Process.WaitForExit(2000)) { $Process.Kill() }
    } catch {
    } finally {
        $Process.Dispose()
    }
}

function Invoke-SessionSQL {
    param([ValidateSet("Edge", "Server")][string]$Side, [string]$SQL, [switch]$Scalar)
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        try {
            if ($Side -eq "Edge") {
                if ($null -eq $script:EdgeSession -or $script:EdgeSession.HasExited) {
                    Close-Session $script:EdgeSession
                    $script:EdgeSession = New-EdgeMySQLSession
                }
                $process = $script:EdgeSession
            } else {
                if ($null -eq $script:ServerSession -or $script:ServerSession.HasExited) {
                    Close-Session $script:ServerSession
                    $script:ServerSession = New-ServerMySQLSession
                }
                $process = $script:ServerSession
            }
            $marker = "__NB_END_$([Guid]::NewGuid().ToString('N'))__"
            $process.StandardInput.WriteLine("$SQL SELECT '$marker';")
            $process.StandardInput.Flush()
            $lines = [Collections.Generic.List[string]]::new()
            while ($true) {
                $line = $process.StandardOutput.ReadLine()
                if ($null -eq $line) { throw "$Side mysql session ended unexpectedly" }
                if ($line.Trim() -eq $marker) { break }
                $lines.Add($line.Trim())
            }
            if ($Scalar) {
                if ($lines.Count -eq 0) { return "" }
                return $lines[0]
            }
            return @($lines)
        } catch {
            if ($Side -eq "Edge") { Close-Session $script:EdgeSession; $script:EdgeSession = $null }
            else { Close-Session $script:ServerSession; $script:ServerSession = $null }
            if ($attempt -eq 3) { throw }
            Start-Sleep -Seconds (2 * $attempt)
        }
    }
}

function Get-AgentProcess {
    param([ValidateSet("Edge", "Server")][string]$Side)
    if ($Side -eq "Edge") {
        $remoteScript = @'
Get-CimInstance Win32_Process -Filter "Name='SyncAgent.exe'" |
    Where-Object { $_.CommandLine -like '*C:\ProgramData\NodeBridge\config.yaml*' } |
    Select-Object -First 1 ProcessId,ExecutablePath,CommandLine |
    ConvertTo-Json -Compress
'@
        $raw = Invoke-RemotePowerShell $remoteScript
        $json = ($raw | Where-Object { $_ -match '^\s*\{' } | Select-Object -Last 1)
        if ([string]::IsNullOrWhiteSpace($json)) { return $null }
        return $json | ConvertFrom-Json
    }
    return Get-CimInstance Win32_Process -Filter "Name='SyncAgent.exe'" | Where-Object { $_.CommandLine -like "*$ServerConfigPath*" } | Select-Object -First 1 ProcessId,ExecutablePath,CommandLine
}

function Stop-Agent {
    param([ValidateSet("Edge", "Server")][string]$Side)
    if ($Side -eq "Edge") {
        $script = @'
New-Item -ItemType Directory -Force -Path 'C:\ProgramData\NodeBridge\run' | Out-Null
[IO.File]::WriteAllText('C:\ProgramData\NodeBridge\run\sync-agent.stop', (Get-Date).ToString('o'))
$deadline = (Get-Date).AddSeconds(20)
do {
    Start-Sleep -Milliseconds 200
    $p = Get-CimInstance Win32_Process -Filter "Name='SyncAgent.exe'" |
        Where-Object { $_.CommandLine -like '*C:\ProgramData\NodeBridge\config.yaml*' }
} while ($null -ne $p -and (Get-Date) -lt $deadline)
if ($null -ne $p) { throw 'edge agent did not stop' }
'STOPPED'
'@
        Invoke-RemotePowerShell $script | Out-Null
    } else {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $ServerStopPath) | Out-Null
        [IO.File]::WriteAllText($ServerStopPath, (Get-Date).ToString("o"), [Text.UTF8Encoding]::new($false))
        $deadline = (Get-Date).AddSeconds(20)
        do { Start-Sleep -Milliseconds 200; $process = Get-AgentProcess "Server" } while ($null -ne $process -and (Get-Date) -lt $deadline)
        if ($null -ne $process) { throw "server agent did not stop" }
    }
    Add-Event "agent_stopped" "ok" @{ side = $Side }
}

function Start-Agent {
    param([ValidateSet("Edge", "Server")][string]$Side, [switch]$TestMode)
    if ($null -ne (Get-AgentProcess $Side)) { return }
    if ($Side -eq "Edge") {
        $command = "`"$EdgeAgentPath`" run -config `"$EdgeConfigPath`" -rules `"$EdgeRulesPath`" -stop-file `"$EdgeStopPath`""
        $encodedCommand = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
        $script = @'
Remove-Item -LiteralPath 'C:\ProgramData\NodeBridge\run\sync-agent.stop' -Force -ErrorAction SilentlyContinue
$command = [Text.Encoding]::Unicode.GetString([Convert]::FromBase64String('__COMMAND__'))
$result = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $command }
if ($result.ReturnValue -ne 0) { throw ("edge agent create failed: {0}" -f $result.ReturnValue) }
$result.ProcessId
'@.Replace('__COMMAND__', $encodedCommand)
        $output = Invoke-RemotePowerShell $script
        $newPID = ($output | Where-Object { $_ -match '^\d+$' } | Select-Object -Last 1)
    } else {
        Remove-Item -LiteralPath $ServerStopPath -Force -ErrorAction SilentlyContinue
        $edgeArgument = if ($TestMode) { " -edges edge-001" } else { "" }
        $agentPath = if (-not $TestMode -and $script:OriginalServerAgentPath) { $script:OriginalServerAgentPath } else { $ServerAgentPath }
        $command = "`"$agentPath`" run -config `"$ServerConfigPath`" -rules `"$ServerRulesPath`"$edgeArgument -stop-file `"$ServerStopPath`""
        $result = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $command }
        if ($result.ReturnValue -ne 0) { throw "server agent create failed: $($result.ReturnValue)" }
        $newPID = $result.ProcessId
    }
    $deadline = (Get-Date).AddSeconds(20)
    do { Start-Sleep -Milliseconds 250; $process = Get-AgentProcess $Side } while ($null -eq $process -and (Get-Date) -lt $deadline)
    if ($null -eq $process) { throw "$Side agent did not become visible" }
    Add-Event "agent_started" "ok" @{ side = $Side; pid = [int]$process.ProcessId; requested_pid = $newPID; test_mode = [bool]$TestMode }
}

function Restart-Agents {
    param([switch]$TestMode)
    foreach ($side in @("Edge", "Server")) {
        if ($null -ne (Get-AgentProcess $side)) { Stop-Agent $side }
    }
    Start-Agent "Server" -TestMode:$TestMode
    Start-Agent "Edge" -TestMode:$TestMode
}

function Wait-Until {
    param([scriptblock]$Condition, [int]$TimeoutSeconds, [string]$Description)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if (& $Condition) { return }
        Set-State @{}
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    throw "timed out waiting for $Description"
}

function Get-ServerQueueSnapshot {
    $output = Invoke-External $DockerPath @("exec", $ServerRabbitMQContainer, "rabbitmqctl", "list_queues", "-p", $ServerRabbitVHost, "name", "messages_ready", "messages_unacknowledged") "server queue snapshot"
    $queues = [ordered]@{}
    foreach ($line in $output) {
        if ($line -match '^([^\s]+)\s+(\d+)\s+(\d+)$') {
            $queues[$Matches[1]] = @{ ready = [int64]$Matches[2]; unacked = [int64]$Matches[3] }
        }
    }
    return $queues
}

function Write-Snapshot {
    param([string]$Phase)
    $queue = Get-ServerQueueSnapshot
    $snapshot = [ordered]@{
        at = (Get-Date).ToString("o")
        phase = $Phase
        cycle = $script:State.cycle
        edge_rows_written = $script:State.edge_rows_written
        server_rows_written = $script:State.server_rows_written
        edge_target_rows = [int64](Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar)
        server_target_rows = [int64](Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar)
        queues = $queue
        edge_agent_pid = if ($null -eq ($p = Get-AgentProcess "Edge")) { 0 } else { [int]$p.ProcessId }
        server_agent_pid = if ($null -eq ($p = Get-AgentProcess "Server")) { 0 } else { [int]$p.ProcessId }
    }
    Add-Content -LiteralPath $SnapshotsPath -Value ($snapshot | ConvertTo-Json -Depth 8 -Compress) -Encoding UTF8
    Set-State @{ last_snapshot = $snapshot }
}

function New-RulesFile {
    param([string]$OriginalPath, [string]$OutputPath)
    $original = Get-Content -LiteralPath $OriginalPath -Raw
    $addition = @"
    - id: overnight-edge-stream
      database_name: scada_edge
      table_name: nb_overnight_edge_stream
      source_node_ids: [edge-001]
      target_database_name: scada_center
      target_table_name: nb_overnight_edge_archive
      direction: EDGE_TO_SERVER
      dispatch_target: NONE
      sync_mode: append_only
      conflict_policy: NONE
      enable: true
      primary_keys: [id]
      include_columns: []
      exclude_columns: []
      schema_sync: {add_columns: true, drop_columns: true}
    - id: overnight-server-stream
      database_name: scada_center
      table_name: nb_overnight_server_stream
      target_database_name: scada_edge
      target_table_name: nb_overnight_server_inbox
      direction: SERVER_TO_EDGE
      dispatch_target: SELECTED_EDGES
      dispatch_node_ids: [edge-001]
      sync_mode: append_only
      conflict_policy: NONE
      enable: true
      primary_keys: [id]
      include_columns: []
      exclude_columns: []
      schema_sync: {add_columns: true, drop_columns: true}
    - id: overnight-edge-state
      database_name: scada_edge
      table_name: nb_overnight_edge_state
      source_node_ids: [edge-001]
      target_database_name: scada_center
      target_table_name: nb_overnight_center_state
      direction: BIDIRECTIONAL
      dispatch_target: ACTIVE_EDGES
      sync_mode: crud_ordered
      conflict_policy: LAST_WRITE_WIN
      enable: true
      primary_keys: [item_id]
      target_primary_keys: [record_id]
      include_columns: [item_id, run_tag, edge_value, is_deleted, deleted_at, deleted_by_node, sync_version, updated_by_node, last_event_id, updated_at]
      exclude_columns: [ignored_edge]
      column_mappings:
        - {source_column: item_id, target_column: record_id}
        - {source_column: edge_value, target_column: server_value}
    - id: overnight-server-state
      database_name: scada_center
      table_name: nb_overnight_center_state
      target_database_name: scada_edge
      target_table_name: nb_overnight_edge_state
      direction: SERVER_TO_EDGE
      dispatch_target: SELECTED_EDGES
      dispatch_node_ids: [edge-001]
      sync_mode: crud_ordered
      conflict_policy: LAST_WRITE_WIN
      enable: true
      primary_keys: [record_id]
      target_primary_keys: [item_id]
      include_columns: [record_id, run_tag, server_value, is_deleted, deleted_at, deleted_by_node, sync_version, updated_by_node, last_event_id, updated_at]
      exclude_columns: [ignored_server]
      column_mappings:
        - {source_column: record_id, target_column: item_id}
        - {source_column: server_value, target_column: edge_value}
"@
    [IO.File]::WriteAllText($OutputPath, ($original.TrimEnd() + "`r`n" + $addition.TrimEnd()), [Text.UTF8Encoding]::new($false))
}

function Provision-Tables {
    $edgeSQL = @"
CREATE TABLE IF NOT EXISTS scada_edge.nb_overnight_edge_stream (id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, seq_no BIGINT NOT NULL, payload VARCHAR(255) NOT NULL, created_at DATETIME(3) NOT NULL, PRIMARY KEY(id), KEY ix_run_seq(run_id,seq_no)) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS scada_edge.nb_overnight_server_inbox (id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, seq_no BIGINT NOT NULL, payload VARCHAR(255) NOT NULL, created_at DATETIME(3) NOT NULL, PRIMARY KEY(id), KEY ix_run_seq(run_id,seq_no)) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS scada_edge.nb_overnight_edge_state (item_id BIGINT NOT NULL, run_tag VARCHAR(64) NOT NULL, edge_value VARCHAR(255) NOT NULL, ignored_edge VARCHAR(64) NULL, is_deleted TINYINT NOT NULL DEFAULT 0, deleted_at DATETIME(3) NULL, deleted_by_node VARCHAR(64) NULL, sync_version BIGINT NOT NULL DEFAULT 0, updated_by_node VARCHAR(64) NOT NULL, last_event_id VARCHAR(128) NOT NULL, updated_at DATETIME(3) NOT NULL, PRIMARY KEY(item_id), KEY ix_run_tag(run_tag)) ENGINE=InnoDB;
DELETE FROM scada_edge.nb_overnight_edge_stream WHERE run_id='$RunId';
DELETE FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';
DELETE FROM scada_edge.nb_overnight_edge_state WHERE run_tag='$RunId';
"@
    $serverSQL = @"
CREATE TABLE IF NOT EXISTS scada_center.nb_overnight_edge_archive (id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, seq_no BIGINT NOT NULL, payload VARCHAR(255) NOT NULL, created_at DATETIME(3) NOT NULL, PRIMARY KEY(id), KEY ix_run_seq(run_id,seq_no)) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS scada_center.nb_overnight_server_stream (id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, seq_no BIGINT NOT NULL, payload VARCHAR(255) NOT NULL, created_at DATETIME(3) NOT NULL, PRIMARY KEY(id), KEY ix_run_seq(run_id,seq_no)) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS scada_center.nb_overnight_center_state (record_id BIGINT NOT NULL, run_tag VARCHAR(64) NOT NULL, server_value VARCHAR(255) NOT NULL, ignored_server VARCHAR(64) NULL, is_deleted TINYINT NOT NULL DEFAULT 0, deleted_at DATETIME(3) NULL, deleted_by_node VARCHAR(64) NULL, sync_version BIGINT NOT NULL DEFAULT 0, updated_by_node VARCHAR(64) NOT NULL, last_event_id VARCHAR(128) NOT NULL, updated_at DATETIME(3) NOT NULL, PRIMARY KEY(record_id), KEY ix_run_tag(run_tag)) ENGINE=InnoDB;
DELETE FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';
DELETE FROM scada_center.nb_overnight_server_stream WHERE run_id='$RunId';
DELETE FROM scada_center.nb_overnight_center_state WHERE run_tag='$RunId';
"@
    Invoke-SessionSQL Edge $edgeSQL | Out-Null
    Invoke-SessionSQL Server $serverSQL | Out-Null
    if ((Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_edge_state' AND column_name='is_deleted';" -Scalar) -eq "0") {
        Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_edge_state ADD COLUMN is_deleted TINYINT NOT NULL DEFAULT 0;" | Out-Null
    }
    if ((Invoke-SessionSQL Server "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_center' AND table_name='nb_overnight_center_state' AND column_name='is_deleted';" -Scalar) -eq "0") {
        Invoke-SessionSQL Server "ALTER TABLE scada_center.nb_overnight_center_state ADD COLUMN is_deleted TINYINT NOT NULL DEFAULT 0;" | Out-Null
    }
    if ((Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_edge_state' AND column_name='deleted_at';" -Scalar) -eq "0") {
        Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_edge_state ADD COLUMN deleted_at DATETIME(3) NULL;" | Out-Null
    }
    if ((Invoke-SessionSQL Server "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_center' AND table_name='nb_overnight_center_state' AND column_name='deleted_at';" -Scalar) -eq "0") {
        Invoke-SessionSQL Server "ALTER TABLE scada_center.nb_overnight_center_state ADD COLUMN deleted_at DATETIME(3) NULL;" | Out-Null
    }
    if ((Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_edge_state' AND column_name='deleted_by_node';" -Scalar) -eq "0") {
        Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_edge_state ADD COLUMN deleted_by_node VARCHAR(64) NULL;" | Out-Null
    }
    if ((Invoke-SessionSQL Server "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_center' AND table_name='nb_overnight_center_state' AND column_name='deleted_by_node';" -Scalar) -eq "0") {
        Invoke-SessionSQL Server "ALTER TABLE scada_center.nb_overnight_center_state ADD COLUMN deleted_by_node VARCHAR(64) NULL;" | Out-Null
    }
    if ((Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_edge_stream' AND column_name='overnight_note';" -Scalar) -eq "1") {
        Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_edge_stream DROP COLUMN overnight_note;" | Out-Null
    }
    if ((Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_server_inbox' AND column_name='overnight_note';" -Scalar) -eq "1") {
        Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_server_inbox DROP COLUMN overnight_note;" | Out-Null
    }
    foreach ($table in @("nb_overnight_edge_archive", "nb_overnight_server_stream")) {
        if ((Invoke-SessionSQL Server "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_center' AND table_name='$table' AND column_name='overnight_note';" -Scalar) -eq "1") {
            Invoke-SessionSQL Server "ALTER TABLE scada_center.$table DROP COLUMN overnight_note;" | Out-Null
        }
    }
    Add-Event "tables_provisioned"
}

function Install-TestRules {
    $originalServer = Get-AgentProcess "Server"
    if ($null -ne $originalServer) {
        $script:OriginalServerAgentPath = $originalServer.ExecutablePath
        Set-State @{ original_server_agent_path = $script:OriginalServerAgentPath }
    }
    $serverOriginal = Join-Path $EvidenceRoot "server-rules.original.yaml"
    $edgeOriginal = Join-Path $EvidenceRoot "edge-rules.original.yaml"
    $testRules = Join-Path $EvidenceRoot "sync-rules.test.yaml"
    Copy-Item -LiteralPath $ServerRulesPath -Destination $serverOriginal -Force
    Copy-FromEdge $EdgeRulesPath $edgeOriginal
    $serverHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $serverOriginal).Hash
    $edgeHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $edgeOriginal).Hash
    if ($serverHash -ne $edgeHash) { throw "server and edge rules differ before test; refusing to replace them" }
    New-RulesFile $serverOriginal $testRules
    foreach ($side in @("Edge", "Server")) {
        if ($null -ne (Get-AgentProcess $side)) { Stop-Agent $side }
    }
    & $ServerAgentPath run -config $ServerConfigPath -rules $testRules -edges edge-001 -max-steps 1 2>&1 | Add-Content -LiteralPath (Join-Path $EvidenceRoot "rules-validation.log") -Encoding UTF8
    if ($LASTEXITCODE -ne 0) { throw "generated test rules failed validation" }
    Copy-Item -LiteralPath $testRules -Destination $ServerRulesPath -Force
    Copy-ToEdge $testRules $EdgeRulesPath
    Start-Agent "Server" -TestMode
    Start-Agent "Edge" -TestMode
    Add-Event "test_rules_installed" "ok" @{ original_sha256 = $serverHash; test_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $testRules).Hash }
}

function Restore-Site {
    if ($script:Restored) { return }
    Close-Session $script:EdgeSession; $script:EdgeSession = $null
    Close-Session $script:ServerSession; $script:ServerSession = $null
    $serverOriginal = Join-Path $EvidenceRoot "server-rules.original.yaml"
    $edgeOriginal = Join-Path $EvidenceRoot "edge-rules.original.yaml"
    if ((Test-Path -LiteralPath $serverOriginal) -and (Test-Path -LiteralPath $edgeOriginal)) {
        foreach ($side in @("Edge", "Server")) {
            if ($null -ne (Get-AgentProcess $side)) { Stop-Agent $side }
        }
        Copy-Item -LiteralPath $serverOriginal -Destination $ServerRulesPath -Force
        Copy-ToEdge $edgeOriginal $EdgeRulesPath
        Start-Agent "Server"
        Start-Agent "Edge"
    }
    $script:Restored = $true
    Add-Event "site_restored"
}

function Add-StreamBatch {
    param([ValidateSet("Edge", "Server")][string]$Side, [int64]$BaseId, [int64]$StartSequence, [int]$Count)
    $values = [Collections.Generic.List[string]]::new()
    for ($offset = 0; $offset -lt $Count; $offset++) {
        $sequence = $StartSequence + $offset
        $id = $BaseId + $sequence
        $payload = "$($Side.ToLowerInvariant())-$RunId-$sequence"
        $values.Add("($id,'$RunId',$sequence,'$payload',NOW(3))")
    }
    $table = if ($Side -eq "Edge") { "scada_edge.nb_overnight_edge_stream" } else { "scada_center.nb_overnight_server_stream" }
    Invoke-SessionSQL $Side "INSERT INTO $table(id,run_id,seq_no,payload,created_at) VALUES $($values -join ',');" | Out-Null
}

function Invoke-CRUDCycle {
    param([int]$Cycle, [int64]$BaseId)
    $version = $Cycle + 1
    $slot = $Cycle % 4
    $edgeId = $BaseId + 4000000
    $serverId = $BaseId + 4000001
    $sharedId = $BaseId + 4000002
    $deleteId = $BaseId + 4000003
    $edgeEvent = "edge-$RunId-$version"
    $serverEvent = "server-$RunId-$version"
    Invoke-SessionSQL Edge "INSERT INTO scada_edge.nb_overnight_edge_state(item_id,run_tag,edge_value,ignored_edge,sync_version,updated_by_node,last_event_id,updated_at) VALUES($edgeId,'$RunId','edge-owned-$version','must-not-map',$version,'edge-001','$edgeEvent',NOW(3)) ON DUPLICATE KEY UPDATE edge_value=VALUES(edge_value),ignored_edge=VALUES(ignored_edge),sync_version=VALUES(sync_version),updated_by_node=VALUES(updated_by_node),last_event_id=VALUES(last_event_id),updated_at=VALUES(updated_at);" | Out-Null
    Invoke-SessionSQL Server "INSERT INTO scada_center.nb_overnight_center_state(record_id,run_tag,server_value,ignored_server,sync_version,updated_by_node,last_event_id,updated_at) VALUES($serverId,'$RunId','server-owned-$version','must-not-map',$version,'server-001','$serverEvent',NOW(3)) ON DUPLICATE KEY UPDATE server_value=VALUES(server_value),ignored_server=VALUES(ignored_server),sync_version=VALUES(sync_version),updated_by_node=VALUES(updated_by_node),last_event_id=VALUES(last_event_id),updated_at=VALUES(updated_at);" | Out-Null
    if (($Cycle % 2) -eq 0) {
        Invoke-SessionSQL Edge "INSERT INTO scada_edge.nb_overnight_edge_state(item_id,run_tag,edge_value,ignored_edge,sync_version,updated_by_node,last_event_id,updated_at) VALUES($sharedId,'$RunId','shared-edge-$version','edge-ignore',$version,'edge-001','shared-edge-$RunId-$version',NOW(3)) ON DUPLICATE KEY UPDATE edge_value=VALUES(edge_value),sync_version=VALUES(sync_version),updated_by_node=VALUES(updated_by_node),last_event_id=VALUES(last_event_id),updated_at=VALUES(updated_at);" | Out-Null
    } else {
        Invoke-SessionSQL Server "INSERT INTO scada_center.nb_overnight_center_state(record_id,run_tag,server_value,ignored_server,sync_version,updated_by_node,last_event_id,updated_at) VALUES($sharedId,'$RunId','shared-server-$version','server-ignore',$version,'server-001','shared-server-$RunId-$version',NOW(3)) ON DUPLICATE KEY UPDATE server_value=VALUES(server_value),sync_version=VALUES(sync_version),updated_by_node=VALUES(updated_by_node),last_event_id=VALUES(last_event_id),updated_at=VALUES(updated_at);" | Out-Null
    }
    if ($slot -eq 0) {
        Invoke-SessionSQL Edge "INSERT INTO scada_edge.nb_overnight_edge_state(item_id,run_tag,edge_value,ignored_edge,sync_version,updated_by_node,last_event_id,updated_at) VALUES($deleteId,'$RunId','delete-cycle-$version','edge-ignore',$version,'edge-001','delete-insert-$RunId-$version',NOW(3)) ON DUPLICATE KEY UPDATE edge_value=VALUES(edge_value),sync_version=VALUES(sync_version),updated_by_node=VALUES(updated_by_node),last_event_id=VALUES(last_event_id),updated_at=VALUES(updated_at);" | Out-Null
    } elseif ($slot -eq 2) {
        Invoke-SessionSQL Edge "DELETE FROM scada_edge.nb_overnight_edge_state WHERE item_id=$deleteId AND run_tag='$RunId';" | Out-Null
        Wait-Until { (Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_center_state WHERE record_id=$deleteId AND run_tag='$RunId' AND is_deleted=1;" -Scalar) -eq "1" } 120 "mapped DELETE soft tombstone"
        $script:State.crud_delete_verified = $true
        Add-Event "crud_delete_propagated" "ok" @{ cycle = $Cycle; id = $deleteId }
    }
    $script:State.crud_cycles = [int]$script:State.crud_cycles + 1
}

function Invoke-DDLAdd {
    Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_edge_stream ADD COLUMN overnight_note VARCHAR(64) NULL;" | Out-Null
    Invoke-SessionSQL Server "ALTER TABLE scada_center.nb_overnight_server_stream ADD COLUMN overnight_note VARCHAR(64) NULL;" | Out-Null
    Wait-Until { (Invoke-SessionSQL Server "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_center' AND table_name='nb_overnight_edge_archive' AND column_name='overnight_note';" -Scalar) -eq "1" } 120 "Edge-to-Server ADD COLUMN"
    Wait-Until { (Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_server_inbox' AND column_name='overnight_note';" -Scalar) -eq "1" } 120 "Server-to-Edge ADD COLUMN"
    $edgeDDLId = [int64]$script:State.base_id + 7000000L
    $serverDDLId = [int64]$script:State.base_id + 9000000L
    Invoke-SessionSQL Edge "INSERT INTO scada_edge.nb_overnight_edge_stream(id,run_id,seq_no,payload,created_at,overnight_note) VALUES($edgeDDLId,'$RunId',7000000,'edge-ddl-row',NOW(3),'edge-ddl-$RunId');" | Out-Null
    Invoke-SessionSQL Server "INSERT INTO scada_center.nb_overnight_server_stream(id,run_id,seq_no,payload,created_at,overnight_note) VALUES($serverDDLId,'$RunId',7000000,'server-ddl-row',NOW(3),'server-ddl-$RunId');" | Out-Null
    Wait-Until { (Invoke-SessionSQL Server "SELECT overnight_note FROM scada_center.nb_overnight_edge_archive WHERE id=$edgeDDLId;" -Scalar) -eq "edge-ddl-$RunId" } 120 "Edge-to-Server ADD COLUMN value"
    Wait-Until { (Invoke-SessionSQL Edge "SELECT overnight_note FROM scada_edge.nb_overnight_server_inbox WHERE id=$serverDDLId;" -Scalar) -eq "server-ddl-$RunId" } 120 "Server-to-Edge ADD COLUMN value"
    Add-Event "ddl_add_propagated"
}

function Wait-ForStreamCatchUp {
    param([int]$TimeoutSeconds, [string]$Scenario)
    Wait-Until {
        $edgeSource = [int64](Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_edge_stream WHERE run_id='$RunId';" -Scalar)
        $edgeTarget = [int64](Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar)
        $serverSource = [int64](Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_server_stream WHERE run_id='$RunId';" -Scalar)
        $serverTarget = [int64](Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar)
        $edgeSource -eq $edgeTarget -and $serverSource -eq $serverTarget
    } $TimeoutSeconds "$Scenario stream recovery"
    Add-Event "stream_recovery_verified" "ok" @{
        scenario = $Scenario
        edge_rows = [int64](Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar)
        server_rows = [int64](Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar)
    }
}

function Invoke-DDLDrop {
    Invoke-SessionSQL Edge "ALTER TABLE scada_edge.nb_overnight_edge_stream DROP COLUMN overnight_note;" | Out-Null
    Invoke-SessionSQL Server "ALTER TABLE scada_center.nb_overnight_server_stream DROP COLUMN overnight_note;" | Out-Null
    Wait-Until { (Invoke-SessionSQL Server "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_center' AND table_name='nb_overnight_edge_archive' AND column_name='overnight_note';" -Scalar) -eq "0" } 120 "Edge-to-Server DROP COLUMN"
    Wait-Until { (Invoke-SessionSQL Edge "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='scada_edge' AND table_name='nb_overnight_server_inbox' AND column_name='overnight_note';" -Scalar) -eq "0" } 120 "Server-to-Edge DROP COLUMN"
    Add-Event "ddl_drop_propagated"
}

function Invoke-IdempotencyProbe {
    param([int64]$BaseId)
    $eventID = "duplicate-$RunId"
    $id = $BaseId + 8000000
    $now = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
    $event = [ordered]@{
        event_id = $eventID; event_type = "INSERT"; origin_node_id = "edge-001"; source_node_id = "edge-001";
        database_name = "scada_edge"; table_name = "nb_overnight_edge_stream"; primary_key = @{ id = $id };
        after = @{ id = $id; run_id = "$RunId-dup"; seq_no = 1; payload = "duplicate-once"; created_at = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss.fff") };
        schema_version = 1; sync_version = 0; created_at = $now; event_time = $now; trace_id = $eventID; headers = @{ overnight = "idempotency" }
    }
    $path = Join-Path $EvidenceRoot "duplicate-event.json"
    [IO.File]::WriteAllText($path, ($event | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
    for ($i = 0; $i -lt 2; $i++) {
        Invoke-External $ServerAgentPath @("publish-event", "-amqp-url", $ServerAMQPURL, "-file", $path) "duplicate event publish" | Out-Null
    }
    Wait-Until { (Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId-dup' AND id=$id;" -Scalar) -eq "1" } 60 "duplicate event apply"
    $applyCount = Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.sync_apply_log WHERE event_id='$eventID';" -Scalar
    if ($applyCount -ne "1") { throw "duplicate event apply log count expected 1, got $applyCount" }
    Add-Event "duplicate_event_idempotent" "ok" @{ event_id = $eventID; published = 2; business_rows = 1; apply_rows = 1 }
}

function Get-ConsistencySummary {
    $edgeSource = [int64](Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_edge_stream WHERE run_id='$RunId';" -Scalar)
    $edgeTarget = [int64](Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar)
    $serverSource = [int64](Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_server_stream WHERE run_id='$RunId';" -Scalar)
    $serverTarget = [int64](Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar)
    $edgeDigest = Invoke-SessionSQL Edge "SELECT CONCAT(COUNT(*),'|',COALESCE(SUM(CRC32(CONCAT_WS('#',id,run_id,seq_no,payload,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s.%f')))),0)) FROM scada_edge.nb_overnight_edge_stream WHERE run_id='$RunId';" -Scalar
    $edgeTargetDigest = Invoke-SessionSQL Server "SELECT CONCAT(COUNT(*),'|',COALESCE(SUM(CRC32(CONCAT_WS('#',id,run_id,seq_no,payload,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s.%f')))),0)) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar
    $serverDigest = Invoke-SessionSQL Server "SELECT CONCAT(COUNT(*),'|',COALESCE(SUM(CRC32(CONCAT_WS('#',id,run_id,seq_no,payload,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s.%f')))),0)) FROM scada_center.nb_overnight_server_stream WHERE run_id='$RunId';" -Scalar
    $serverTargetDigest = Invoke-SessionSQL Edge "SELECT CONCAT(COUNT(*),'|',COALESCE(SUM(CRC32(CONCAT_WS('#',id,run_id,seq_no,payload,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s.%f')))),0)) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar
    return [ordered]@{
        edge_to_server = @{ source_rows = $edgeSource; target_rows = $edgeTarget; source_digest = $edgeDigest; target_digest = $edgeTargetDigest; match = ($edgeSource -eq $edgeTarget -and $edgeDigest -eq $edgeTargetDigest) }
        server_to_edge = @{ source_rows = $serverSource; target_rows = $serverTarget; source_digest = $serverDigest; target_digest = $serverTargetDigest; match = ($serverSource -eq $serverTarget -and $serverDigest -eq $serverTargetDigest) }
    }
}

function Assert-FinalState {
    param([int64]$BaseId)
    foreach ($scenario in @("edge_outage_done", "server_outage_done", "ddl_add_done", "ddl_drop_done", "idempotency_done", "crud_delete_verified")) {
        if (-not $script:State[$scenario]) { throw "required scenario did not run: $scenario" }
    }
    $summary = Get-ConsistencySummary
    if (-not $summary.edge_to_server.match) { throw "Edge-to-Server stream mismatch" }
    if (-not $summary.server_to_edge.match) { throw "Server-to-Edge stream mismatch" }
    $edgeId = $BaseId + 4000000
    $serverId = $BaseId + 4000001
    $sharedId = $BaseId + 4000002
    foreach ($id in @($edgeId, $serverId, $sharedId)) {
        $edgeRow = Invoke-SessionSQL Edge "SELECT CONCAT(edge_value,'|',sync_version,'|',updated_by_node,'|',last_event_id) FROM scada_edge.nb_overnight_edge_state WHERE item_id=$id AND run_tag='$RunId';" -Scalar
        $serverRow = Invoke-SessionSQL Server "SELECT CONCAT(server_value,'|',sync_version,'|',updated_by_node,'|',last_event_id) FROM scada_center.nb_overnight_center_state WHERE record_id=$id AND run_tag='$RunId';" -Scalar
        if ([string]::IsNullOrWhiteSpace($edgeRow) -or $edgeRow -ne $serverRow) { throw "bidirectional state mismatch for id $id`: edge=$edgeRow server=$serverRow" }
    }
    $ignoredOnServer = Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_center_state WHERE record_id=$edgeId AND run_tag='$RunId' AND ignored_server IS NOT NULL;" -Scalar
    $ignoredOnEdge = Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_edge_state WHERE item_id=$serverId AND run_tag='$RunId' AND ignored_edge IS NOT NULL;" -Scalar
    if ($ignoredOnServer -ne "0" -or $ignoredOnEdge -ne "0") { throw "excluded columns crossed the mapping boundary" }
    $queues = Get-ServerQueueSnapshot
    foreach ($name in @("server.cdc.ingress.q", "edge-001.downlink.q", "server.dead.q")) {
        if ($queues.Contains($name) -and ($queues[$name].ready -ne 0 -or $queues[$name].unacked -ne 0)) { throw "queue not drained: $name" }
    }
    $serverErrors = Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.sync_error_log WHERE created_at >= '$($script:State.started_at_mysql)';" -Scalar
    $edgeErrors = Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.sync_error_log WHERE created_at >= '$($script:State.started_at_mysql)';" -Scalar
    if ($serverErrors -ne "0" -or $edgeErrors -ne "0") { throw "sync errors detected: server=$serverErrors edge=$edgeErrors" }
    $final = [ordered]@{ status = "passed"; run_id = $RunId; finished_at = (Get-Date).ToString("o"); consistency = $summary; queues = $queues; server_error_rows = [int]$serverErrors; edge_error_rows = [int]$edgeErrors; scenarios = $script:State.scenarios }
    Write-JsonAtomic (Join-Path $EvidenceRoot "final-report.json") $final
    Add-Event "final_consistency_passed" "ok" $final
    return $final
}

function Wait-ForDrain {
    param([int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $edgeSource = Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_edge_stream WHERE run_id='$RunId';" -Scalar
        $edgeTarget = Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar
        $serverSource = Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_server_stream WHERE run_id='$RunId';" -Scalar
        $serverTarget = Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar
        $queues = Get-ServerQueueSnapshot
        $queueDepth = 0
        foreach ($queue in $queues.Values) { $queueDepth += [int64]$queue.ready + [int64]$queue.unacked }
        Set-State @{ phase = "draining"; drain = @{ edge_source = $edgeSource; edge_target = $edgeTarget; server_source = $serverSource; server_target = $serverTarget; queue_depth = $queueDepth } }
        if ($edgeSource -eq $edgeTarget -and $serverSource -eq $serverTarget -and $queueDepth -eq 0) { return }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    throw "queues and target tables did not drain before timeout"
}

$startAt = Get-Date
$duration = [TimeSpan]::FromHours($DurationHours)
$drainWindow = if ($Smoke) { [TimeSpan]::FromSeconds([Math]::Min(20, [Math]::Max(5, $duration.TotalSeconds * 0.12))) } else { [TimeSpan]::FromMinutes(15) }
if ($drainWindow -ge $duration) { $drainWindow = [TimeSpan]::FromSeconds([Math]::Max(2, $duration.TotalSeconds * 0.15)) }
$workloadEnd = $startAt + $duration - $drainWindow
$plannedEnd = $startAt + $duration
$baseId = [int64]([DateTimeOffset]::UtcNow.ToUnixTimeSeconds()) * 10000000L
$faultDurationSeconds = if ($Smoke) { 5 } else { 300 }
$snapshotIntervalSeconds = if ($Smoke) { 10 } else { 60 }
$recoveryTimeoutSeconds = if ($Smoke) { 180 } else { 600 }
$crudEveryCycles = [Math]::Max(1, [Math]::Round(60 / $IntervalSeconds))
if ($Smoke) { $crudEveryCycles = [Math]::Min(10, $crudEveryCycles) }

$script:State = [ordered]@{
    run_id = $RunId; status = "running"; phase = "setup"; runner_pid = $PID;
    started_at = $startAt.ToString("o"); started_at_mysql = $startAt.ToString("yyyy-MM-dd HH:mm:ss"); heartbeat_at = $startAt.ToString("o");
    planned_end_at = $plannedEnd.ToString("o"); workload_end_at = $workloadEnd.ToString("o"); evidence_root = $EvidenceRoot;
    duration_hours = $DurationHours; rows_per_batch = $RowsPerBatch; interval_seconds = $IntervalSeconds; smoke = [bool]$Smoke;
    base_id = $baseId; cycle = 0; edge_rows_written = 0L; server_rows_written = 0L; crud_cycles = 0;
    edge_outage_done = $false; server_outage_done = $false; ddl_add_done = $false; ddl_drop_done = $false; idempotency_done = $false; crud_delete_verified = $false;
    restoration_status = "pending"; last_error = "";
    scenarios = @("edge_to_server_append", "server_to_edge_append", "mapped_crud_insert_update_delete", "paired_bidirectional_updates", "last_write_win_metadata", "include_exclude_columns", "primary_key_and_column_mapping", "edge_agent_outage_backlog_recovery", "server_agent_outage_backlog_recovery", "add_column_both_directions", "drop_column_both_directions", "duplicate_event_idempotency", "queue_drain", "exact_count_and_digest")
}
Write-JsonAtomic $StatePath $script:State
Write-JsonAtomic (Join-Path $EvidenceRoot "manifest.json") $script:State

try {
    if (-not (Test-Path -LiteralPath $SSHKeyPath -PathType Leaf)) { throw "SSH key not found: $SSHKeyPath" }
    Provision-Tables
    Install-TestRules
    Set-State @{ phase = "smoke" }

    Add-StreamBatch "Edge" $baseId 1 1
    Add-StreamBatch "Server" ($baseId + 2000000L) 1 1
    $script:State.edge_rows_written = 1L
    $script:State.server_rows_written = 1L
    Wait-Until { (Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_edge_archive WHERE run_id='$RunId';" -Scalar) -ge "1" } 120 "initial Edge-to-Server row"
    Wait-Until { (Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_server_inbox WHERE run_id='$RunId';" -Scalar) -ge "1" } 120 "initial Server-to-Edge row"
    Invoke-CRUDCycle 0 $baseId
    Wait-Until { (Invoke-SessionSQL Server "SELECT COUNT(*) FROM scada_center.nb_overnight_center_state WHERE run_tag='$RunId';" -Scalar) -ge "1" } 120 "initial mapped Edge state"
    Wait-Until { (Invoke-SessionSQL Edge "SELECT COUNT(*) FROM scada_edge.nb_overnight_edge_state WHERE run_tag='$RunId';" -Scalar) -ge "2" } 120 "initial mapped Server state"
    Add-Event "bidirectional_smoke_passed"
    Set-State @{ phase = "steady" }

    $nextCycleAt = Get-Date
    $nextSnapshotAt = Get-Date
    while ((Get-Date) -lt $workloadEnd) {
        if (Test-Path -LiteralPath $StopRequestPath) {
            $script:State.status = "stopped"
            Add-Event "stop_requested" "stopped"
            break
        }
        $now = Get-Date
        $progress = ($now - $startAt).TotalSeconds / [Math]::Max(1, ($workloadEnd - $startAt).TotalSeconds)

        if (-not $script:State.edge_outage_done -and $progress -ge 0.22) {
            Set-State @{ phase = "edge_agent_outage" }
            Stop-Agent "Edge"
            $outageEnd = (Get-Date).AddSeconds($faultDurationSeconds)
            while ((Get-Date) -lt $outageEnd) {
                Add-StreamBatch "Edge" $baseId ([int64]$script:State.edge_rows_written + 1) $RowsPerBatch
                Add-StreamBatch "Server" ($baseId + 2000000L) ([int64]$script:State.server_rows_written + 1) $RowsPerBatch
                $script:State.edge_rows_written = [int64]$script:State.edge_rows_written + $RowsPerBatch
                $script:State.server_rows_written = [int64]$script:State.server_rows_written + $RowsPerBatch
                Set-State @{ cycle = [int]$script:State.cycle + 1 }
                Start-Sleep -Seconds $IntervalSeconds
            }
            Start-Agent "Edge" -TestMode
            Wait-ForStreamCatchUp $recoveryTimeoutSeconds "edge_agent_outage"
            $script:State.edge_outage_done = $true
            Add-Event "edge_agent_outage_completed" "ok" @{ seconds = $faultDurationSeconds }
            Set-State @{ phase = "recovery_after_edge_outage" }
        }
        if (-not $script:State.server_outage_done -and $progress -ge 0.48) {
            Set-State @{ phase = "server_agent_outage" }
            Stop-Agent "Server"
            $outageEnd = (Get-Date).AddSeconds($faultDurationSeconds)
            while ((Get-Date) -lt $outageEnd) {
                Add-StreamBatch "Edge" $baseId ([int64]$script:State.edge_rows_written + 1) $RowsPerBatch
                Add-StreamBatch "Server" ($baseId + 2000000L) ([int64]$script:State.server_rows_written + 1) $RowsPerBatch
                $script:State.edge_rows_written = [int64]$script:State.edge_rows_written + $RowsPerBatch
                $script:State.server_rows_written = [int64]$script:State.server_rows_written + $RowsPerBatch
                Set-State @{ cycle = [int]$script:State.cycle + 1 }
                Start-Sleep -Seconds $IntervalSeconds
            }
            Start-Agent "Server" -TestMode
            Wait-ForStreamCatchUp $recoveryTimeoutSeconds "server_agent_outage"
            $script:State.server_outage_done = $true
            Add-Event "server_agent_outage_completed" "ok" @{ seconds = $faultDurationSeconds }
            Set-State @{ phase = "recovery_after_server_outage" }
        }
        if (-not $script:State.ddl_add_done -and $progress -ge 0.68) {
            Set-State @{ phase = "ddl_add" }
            Invoke-DDLAdd
            $script:State.ddl_add_done = $true
        }
        if (-not $script:State.ddl_drop_done -and $progress -ge 0.76) {
            Set-State @{ phase = "ddl_drop" }
            Invoke-DDLDrop
            $script:State.ddl_drop_done = $true
        }
        if (-not $script:State.idempotency_done -and $progress -ge 0.84) {
            Set-State @{ phase = "idempotency" }
            Invoke-IdempotencyProbe $baseId
            $script:State.idempotency_done = $true
        }

        if ($now -ge $nextCycleAt) {
            $edgeStart = [int64]$script:State.edge_rows_written + 1
            $serverStart = [int64]$script:State.server_rows_written + 1
            Add-StreamBatch "Edge" $baseId $edgeStart $RowsPerBatch
            Add-StreamBatch "Server" ($baseId + 2000000L) $serverStart $RowsPerBatch
            $script:State.edge_rows_written = [int64]$script:State.edge_rows_written + $RowsPerBatch
            $script:State.server_rows_written = [int64]$script:State.server_rows_written + $RowsPerBatch
            $script:State.cycle = [int]$script:State.cycle + 1
            if (([int]$script:State.cycle % $crudEveryCycles) -eq 0) { Invoke-CRUDCycle ([int]$script:State.crud_cycles) $baseId }
            $nextCycleAt = (Get-Date).AddSeconds($IntervalSeconds)
            Set-State @{ phase = "steady" }
        }
        if ($now -ge $nextSnapshotAt) {
            foreach ($side in @("Server", "Edge")) {
                if ($null -eq (Get-AgentProcess $side)) {
                    Add-Event "unexpected_agent_exit" "recovered" @{ side = $side }
                    Start-Agent $side -TestMode
                }
            }
            Write-Snapshot $script:State.phase
            $nextSnapshotAt = (Get-Date).AddSeconds($snapshotIntervalSeconds)
        }
        Start-Sleep -Milliseconds 200
    }

    if ($script:State.status -ne "stopped") {
        foreach ($side in @("Server", "Edge")) { if ($null -eq (Get-AgentProcess $side)) { Start-Agent $side -TestMode } }
        Wait-ForDrain ([int][Math]::Max(30, $drainWindow.TotalSeconds))
        $final = Assert-FinalState $baseId
        Set-State @{ status = "passed"; phase = "restoring"; final_report = (Join-Path $EvidenceRoot "final-report.json") }
    }
} catch {
    $script:RunError = $_
    Add-Event "run_failed" "failed" @{ message = $_.Exception.Message; detail = ($_ | Out-String) }
    Set-State @{ status = "failed"; phase = "restoring"; last_error = $_.Exception.Message }
} finally {
    try {
        Restore-Site
        Set-State @{ restoration_status = "passed"; phase = if ($null -eq $script:RunError -and $script:State.status -ne "stopped") { "complete" } else { $script:State.status } }
    } catch {
        Add-Event "site_restore_failed" "failed" @{ message = $_.Exception.Message }
        Set-State @{ restoration_status = "failed"; last_error = (($script:State.last_error + "; restore: " + $_.Exception.Message).Trim('; ')); phase = "restore_failed" }
        if ($null -eq $script:RunError) { $script:RunError = $_ }
    }
    Close-Session $script:EdgeSession
    Close-Session $script:ServerSession
    Set-State @{ finished_at = (Get-Date).ToString("o") }
}

if ($null -ne $script:RunError) { throw $script:RunError }
$script:State | ConvertTo-Json -Depth 10
