param(
    [string]$RunId = "staged-v044-month30-agents-001",
    [int64]$ExpectedRows = 5184000,
    [int]$IntervalSeconds = 60,
    [int]$MaxSamples = 2400,
    [int]$CommandTimeoutSeconds = 30,
    [int[]]$AgentPids = @(37132, 7072)
)

$ErrorActionPreference = "Stop"

function Resolve-DockerCli {
    $cmd = Get-Command docker -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        "C:\Program Files\Docker\Docker\resources\bin\docker.exe",
        "C:\ProgramData\DockerDesktop\version-bin\docker.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate) { return $candidate }
    }
    throw "docker CLI not found"
}

function Get-MySqlSum {
    param([string]$Container, [string]$Database)
    $sql = "SELECT (SELECT COUNT(*) FROM collect_data_01)+(SELECT COUNT(*) FROM collect_data_02)+(SELECT COUNT(*) FROM collect_data_03)+(SELECT COUNT(*) FROM collect_data_04)+(SELECT COUNT(*) FROM collect_data_05)+(SELECT COUNT(*) FROM collect_data_06);"
    $output = Invoke-DockerWithTimeout @("exec", $Container, "mysql", "-uroot", "-proot_password", "-N", "-B", $Database, "-e", $sql)
    $value = $output | Where-Object { $_ -match "^\d+$" } | Select-Object -First 1
    if ($null -eq $value) {
        throw "mysql row count failed for $Container/$Database output=$($output -join ' ')"
    }
    return [int64]$value
}

function Get-Queue {
    param([string]$Container, [string]$VHost, [string]$Queue)
    $lines = Invoke-DockerWithTimeout @("exec", $Container, "rabbitmqctl", "-q", "list_queues", "-p", $VHost, "name", "messages", "messages_ready", "messages_unacknowledged")
    foreach ($line in $lines) {
        if ($line -match "^$([regex]::Escape($Queue))\s+(\d+)\s+(\d+)\s+(\d+)") {
            return [pscustomobject]@{
                total = [int64]$Matches[1]
                ready = [int64]$Matches[2]
                unacked = [int64]$Matches[3]
            }
        }
    }
    return [pscustomobject]@{ total = 0; ready = 0; unacked = 0 }
}

function Invoke-DockerWithTimeout {
    param([string[]]$Arguments)
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $script:Docker
    $psi.Arguments = ($Arguments | ForEach-Object { Quote-Argument $_ }) -join " "
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $proc = [System.Diagnostics.Process]::Start($psi)
    if (-not $proc.WaitForExit($CommandTimeoutSeconds * 1000)) {
        try { $proc.Kill() } catch {}
        throw "docker command timed out after ${CommandTimeoutSeconds}s: $($Arguments -join ' ')"
    }
    $stdout = $proc.StandardOutput.ReadToEnd()
    $stderr = $proc.StandardError.ReadToEnd()
    if ($proc.ExitCode -ne 0) {
        throw "docker command failed exit=$($proc.ExitCode): $($Arguments -join ' ') stdout=$stdout stderr=$stderr"
    }
    return (($stdout + "`n" + $stderr) -split "`r?`n" | Where-Object { $_ -ne "" })
}

function Quote-Argument {
    param([string]$Value)
    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    return '"' + ($Value -replace '\\', '\\' -replace '"', '\"') + '"'
}

$repo = Split-Path -Parent $PSScriptRoot
$evidence = Join-Path $repo ".cache\longtest-90d\$RunId"
New-Item -ItemType Directory -Force -Path $evidence | Out-Null
$csv = Join-Path $evidence "batchconfirm-watch.csv"
$summary = Join-Path $evidence "batchconfirm-watch-summary.json"
$script:Docker = Resolve-DockerCli

"at,sample,edge_rows,server_rows,remaining_rows,edge_upload_depth,edge_ready,edge_unacked,server_ingress_depth,server_ready,server_unacked,edge_pid,server_pid,agent_alive" | Set-Content -Path $csv -Encoding UTF8

$startedAt = Get-Date
$completed = $false
$last = $null
for ($i = 1; $i -le $MaxSamples; $i++) {
    $edgeRows = Get-MySqlSum "nodebridge-longtest-mysql-edge" "scada_edge_longtest"
    $serverRows = Get-MySqlSum "nodebridge-longtest-mysql-server" "scada_center_longtest"
    $edgeQ = Get-Queue "nodebridge-longtest-rabbitmq-edge" "edge-longtest-sync" "edge.upload.cdc.q"
    $serverQ = Get-Queue "nodebridge-longtest-rabbitmq-server" "server-longtest-sync" "server.cdc.ingress.q"
    $agentAlive = $true
    foreach ($agentPid in $AgentPids) {
        if (-not (Get-Process -Id $agentPid -ErrorAction SilentlyContinue)) {
            $agentAlive = $false
        }
    }
    $remaining = [Math]::Max(0, $ExpectedRows - $serverRows)
    $line = "{0},{1},{2},{3},{4},{5},{6},{7},{8},{9},{10},{11},{12},{13}" -f (Get-Date).ToString("o"), $i, $edgeRows, $serverRows, $remaining, $edgeQ.total, $edgeQ.ready, $edgeQ.unacked, $serverQ.total, $serverQ.ready, $serverQ.unacked, $AgentPids[0], $AgentPids[1], $agentAlive
    Add-Content -Path $csv -Value $line -Encoding UTF8
    $last = [pscustomobject]@{
        at = (Get-Date).ToString("o")
        sample = $i
        edge_rows = $edgeRows
        server_rows = $serverRows
        remaining_rows = $remaining
        edge_upload_depth = $edgeQ.total
        server_ingress_depth = $serverQ.total
        agent_alive = $agentAlive
    }
    $completed = ($serverRows -ge $ExpectedRows -and $edgeQ.total -eq 0 -and $serverQ.total -eq 0)
    if ($completed -or -not $agentAlive) { break }
    Start-Sleep -Seconds $IntervalSeconds
}

$endedAt = Get-Date
[pscustomobject]@{
    run_id = $RunId
    expected_rows = $ExpectedRows
    completed = $completed
    started_at = $startedAt.ToString("o")
    ended_at = $endedAt.ToString("o")
    elapsed_seconds = [Math]::Round(($endedAt - $startedAt).TotalSeconds, 3)
    samples = if ($last) { $last.sample } else { 0 }
    last_sample = $last
    csv = $csv
} | ConvertTo-Json -Depth 6 | Set-Content -Path $summary -Encoding UTF8

if (-not $completed) {
    exit 2
}
