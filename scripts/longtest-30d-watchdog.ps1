param(
    [string]$RunId = "staged-v044-month30-agents-001",
    [int]$IntervalSeconds = 300,
    [int64]$TargetRows = 5184000,
    [int[]]$AgentPids = @(37132, 7072),
    [string]$DockerPath = "C:\Program Files\Docker\Docker\resources\bin\docker.exe",
    [switch]$Once
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$runDir = Join-Path $repoRoot ".cache\longtest-90d\$RunId"
New-Item -ItemType Directory -Path $runDir -Force | Out-Null

$progressPath = Join-Path $runDir "watchdog-progress.csv"
$summaryPath = Join-Path $runDir "watchdog-summary.json"
$errorPath = Join-Path $runDir "watchdog-errors.log"

if (-not (Test-Path $progressPath)) {
    "at,edge_rows,server_rows,remaining_rows,edge_upload_messages,edge_upload_ready,edge_upload_unacked,server_ingress_messages,server_ingress_ready,server_ingress_unacked,agent_count,status" |
        Set-Content -Path $progressPath -Encoding UTF8
}

function Invoke-DockerLines {
    param([string[]]$DockerArgs)
    & $DockerPath @DockerArgs 2>$null
}

function Get-TotalRows {
    param([string]$Container, [string]$Database)
    $sql = "SELECT (SELECT COUNT(*) FROM collect_data_01)+(SELECT COUNT(*) FROM collect_data_02)+(SELECT COUNT(*) FROM collect_data_03)+(SELECT COUNT(*) FROM collect_data_04)+(SELECT COUNT(*) FROM collect_data_05)+(SELECT COUNT(*) FROM collect_data_06);"
    $value = Invoke-DockerLines -DockerArgs @("exec", $Container, "env", "MYSQL_PWD=root_password", "mysql", "-uroot", "-N", "-B", $Database, "-e", $sql) | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($value)) { return 0 }
    return [int64]$value
}

function Get-QueueStats {
    param([string]$Container, [string]$Vhost, [string]$QueueName)
    $lines = Invoke-DockerLines -DockerArgs @("exec", $Container, "rabbitmqctl", "-q", "list_queues", "-p", $Vhost, "name", "messages", "messages_ready", "messages_unacknowledged")
    foreach ($line in $lines) {
        $parts = $line -split "`t"
        if ($parts.Count -ge 4 -and $parts[0] -eq $QueueName) {
            return [pscustomobject]@{
                messages = [int64]$parts[1]
                ready = [int64]$parts[2]
                unacked = [int64]$parts[3]
            }
        }
    }
    return [pscustomobject]@{ messages = 0; ready = 0; unacked = 0 }
}

function Write-Summary {
    param(
        [string]$Status,
        [string]$Message,
        [int64]$EdgeRows,
        [int64]$ServerRows,
        [object]$EdgeUpload,
        [object]$ServerIngress,
        [object[]]$Agents
    )
    [pscustomobject]@{
        run_id = $RunId
        at = (Get-Date).ToString("o")
        status = $Status
        message = $Message
        target_rows = $TargetRows
        edge_rows = $EdgeRows
        server_rows = $ServerRows
        remaining_rows = $TargetRows - $ServerRows
        edge_upload_queue = $EdgeUpload
        server_ingress_queue = $ServerIngress
        agents = $Agents
        progress_path = $progressPath
    } | ConvertTo-Json -Depth 6 | Set-Content -Path $summaryPath -Encoding UTF8
}

while ($true) {
    try {
        $edgeRows = Get-TotalRows "nodebridge-longtest-mysql-edge" "scada_edge_longtest"
        $serverRows = Get-TotalRows "nodebridge-longtest-mysql-server" "scada_center_longtest"
        $edgeUpload = Get-QueueStats "nodebridge-longtest-rabbitmq-edge" "edge-longtest-sync" "edge.upload.cdc.q"
        $serverIngress = Get-QueueStats "nodebridge-longtest-rabbitmq-server" "server-longtest-sync" "server.cdc.ingress.q"
        $agents = @(Get-Process -Id $AgentPids -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,StartTime,CPU,WorkingSet64,Path)
        $agentCount = $agents.Count

        $status = "running"
        if ($serverRows -ge $TargetRows -and $edgeUpload.messages -eq 0 -and $serverIngress.messages -eq 0) {
            $status = "completed"
        } elseif ($agentCount -lt $AgentPids.Count) {
            $status = "agent_exit"
        }

        $line = "{0},{1},{2},{3},{4},{5},{6},{7},{8},{9},{10},{11}" -f `
            (Get-Date).ToString("o"), $edgeRows, $serverRows, ($TargetRows - $serverRows), `
            $edgeUpload.messages, $edgeUpload.ready, $edgeUpload.unacked, `
            $serverIngress.messages, $serverIngress.ready, $serverIngress.unacked, `
            $agentCount, $status
        Add-Content -Path $progressPath -Value $line -Encoding UTF8

        Write-Summary $status "watchdog sample recorded" $edgeRows $serverRows $edgeUpload $serverIngress $agents

        if ($Once -or $status -ne "running") {
            break
        }
    } catch {
        $message = "{0} {1}" -f (Get-Date).ToString("o"), $_.Exception.Message
        Add-Content -Path $errorPath -Value $message -Encoding UTF8
        Write-Summary "error" $_.Exception.Message 0 0 `
            ([pscustomobject]@{ messages = 0; ready = 0; unacked = 0 }) `
            ([pscustomobject]@{ messages = 0; ready = 0; unacked = 0 }) `
            @()
        break
    }

    Start-Sleep -Seconds $IntervalSeconds
}
