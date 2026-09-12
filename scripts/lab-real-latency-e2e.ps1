param(
    [string]$EdgeHost = "192.168.10.105",
    [string]$EdgeUser = "xx",
    [string]$SSHKeyPath = "$HOME\.ssh\nodebridge_ed25519",
    [string]$DockerPath = "C:\Program Files\Docker\Docker\resources\bin\docker.exe",
    [string]$ServerMySQLContainer = "mysql-8.0.38",
    [int]$Count = 20,
    [int]$IdleBetweenProbesMillis = 1200,
    [int]$TimeoutMillis = 10000
)

$ErrorActionPreference = "Stop"

function Start-RedirectedProcess {
    param(
        [Parameter(Mandatory)] [string]$FilePath,
        [Parameter(Mandatory)] [string]$Arguments
    )

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
    if (-not $process.Start()) {
        throw "failed to start $FilePath"
    }
    return $process
}

function Invoke-MySQLLine {
    param(
        [Parameter(Mandatory)] [Diagnostics.Process]$Process,
        [Parameter(Mandatory)] [string]$SQL
    )

    $Process.StandardInput.WriteLine($SQL)
    $Process.StandardInput.Flush()
    $line = $Process.StandardOutput.ReadLine()
    if ($null -eq $line) {
        $stderr = $Process.StandardError.ReadToEnd()
        throw "mysql session ended unexpectedly: $stderr"
    }
    return $line.Trim()
}

if ($Count -le 0) {
    throw "Count must be positive"
}
if (-not (Test-Path -LiteralPath $SSHKeyPath -PathType Leaf)) {
    throw "SSH key not found: $SSHKeyPath"
}
if (-not (Test-Path -LiteralPath $DockerPath -PathType Leaf)) {
    throw "Docker CLI not found: $DockerPath"
}

$remoteScript = @'
$ProgressPreference = "SilentlyContinue"
$env:MYSQL_PWD = "root"
& "C:\Program Files\MySQL\MySQL Server 8.4\bin\mysql.exe" -uroot -N -B --raw --unbuffered
'@
$encodedRemote = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($remoteScript))
$sshArgs = "-T -i `"$SSHKeyPath`" -o BatchMode=yes -o ConnectTimeout=8 $EdgeUser@$EdgeHost powershell.exe -NoProfile -EncodedCommand $encodedRemote"
$dockerArgs = "exec -i -e MYSQL_PWD=root $ServerMySQLContainer mysql -uroot -N -B --raw --unbuffered"

$edge = $null
$server = $null
$results = [Collections.Generic.List[object]]::new()
try {
    $edge = Start-RedirectedProcess -FilePath "ssh.exe" -Arguments $sshArgs
    $server = Start-RedirectedProcess -FilePath $DockerPath -Arguments $dockerArgs

    if ((Invoke-MySQLLine -Process $edge -SQL "SELECT 'EDGE_READY';") -ne "EDGE_READY") {
        throw "edge mysql readiness check failed"
    }
    if ((Invoke-MySQLLine -Process $server -SQL "SELECT 'SERVER_READY';") -ne "SERVER_READY") {
        throw "server mysql readiness check failed"
    }

    for ($index = 1; $index -le $Count; $index++) {
        Start-Sleep -Milliseconds $IdleBetweenProbesMillis
        $id = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds() * 1000L + $index
        $value = "latency-fix-$id"
        $watch = [Diagnostics.Stopwatch]::StartNew()
        $inserted = Invoke-MySQLLine -Process $edge -SQL "INSERT INTO scada_edge.nb_e2e_probe(id,probe_value,created_at) VALUES($id,'$value',NOW(3)); SELECT CONCAT('INSERTED|',$id);"
        if ($inserted -ne "INSERTED|$id") {
            throw "unexpected edge insert response for $id`: $inserted"
        }
        $insertAckMillis = $watch.Elapsed.TotalMilliseconds

        $polls = 0
        do {
            $polls++
            $found = Invoke-MySQLLine -Process $server -SQL "SELECT IF(EXISTS(SELECT 1 FROM scada_center.nb_e2e_probe WHERE id=$id),'FOUND','MISS');"
            if ($found -eq "FOUND") {
                break
            }
            if ($watch.Elapsed.TotalMilliseconds -ge $TimeoutMillis) {
                throw "probe $id timed out after $TimeoutMillis ms"
            }
            Start-Sleep -Milliseconds 10
        } while ($true)

        $watch.Stop()
        $result = [ordered]@{
            sequence = $index
            id = $id
            latency_ms = [math]::Round($watch.Elapsed.TotalMilliseconds, 3)
            insert_ack_ms = [math]::Round($insertAckMillis, 3)
            polls = $polls
        }
        $results.Add([pscustomobject]$result)
        Write-Host ("PROBE {0:D2}/{1}: {2:N3} ms" -f $index, $Count, $result.latency_ms)
    }
} finally {
    foreach ($process in @($edge, $server)) {
        if ($null -eq $process) {
            continue
        }
        try {
            $process.StandardInput.Close()
            if (-not $process.WaitForExit(2000)) {
                $process.Kill()
            }
        } catch {
        }
        $process.Dispose()
    }
}

$sorted = @($results | Sort-Object latency_ms)
$p50Index = [math]::Ceiling($sorted.Count * 0.50) - 1
$p95Index = [math]::Ceiling($sorted.Count * 0.95) - 1
$summary = [ordered]@{
    status = "passed"
    edge = "$EdgeUser@$EdgeHost"
    probes = $results.Count
    idle_between_probes_ms = $IdleBetweenProbesMillis
    min_ms = $sorted[0].latency_ms
    p50_ms = $sorted[$p50Index].latency_ms
    p95_ms = $sorted[$p95Index].latency_ms
    max_ms = $sorted[-1].latency_ms
    avg_ms = [math]::Round(($results | Measure-Object latency_ms -Average).Average, 3)
    samples = @($results)
}
$summary | ConvertTo-Json -Depth 4
