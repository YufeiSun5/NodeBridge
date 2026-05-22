param(
    [switch]$SkipLabPrepare,
    [string]$CanalImage = "canal/canal-server:latest",
    [string]$MySQLHost = "host.docker.internal"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/lib/env.ps1") -RepoRoot $root

function Assert-LastExit {
    param([string]$Command)
    if ($LASTEXITCODE -ne 0) {
        throw "command failed with exit code ${LASTEXITCODE}: $Command"
    }
}

function Invoke-Docker {
    param([string]$Command)
    Invoke-Expression $Command
    Assert-LastExit $Command
}

function Grant-CanalPrivileges {
    param([string]$Container)
    $sql = "GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'sync_user'@'%'; FLUSH PRIVILEGES;"
    Invoke-Docker "docker exec $Container mysql -uroot -proot_password -e `"$sql`""
}

function Start-CanalContainer {
    param(
        [string]$Name,
        [string]$Destination,
        [string]$DatabaseFilter,
        [int]$MySQLPort,
        [int]$HostPort,
        [int]$SlaveID
    )
    $existing = docker ps -a --filter "name=^/$Name$" --format "{{.Names}}"
    if ($existing -contains $Name) {
        docker rm -f $Name | Out-Null
        Assert-LastExit "docker rm -f $Name"
    }
    $args = @(
        "run", "-d",
        "--name", $Name,
        "-p", "${HostPort}:11111",
        "-e", "canal.auto.scan=false",
        "-e", "canal.destinations=$Destination",
        "-e", "canal.instance.master.address=${MySQLHost}:$MySQLPort",
        "-e", "canal.instance.dbUsername=sync_user",
        "-e", "canal.instance.dbPassword=sync_password",
        "-e", "canal.instance.connectionCharset=UTF-8",
        "-e", "canal.instance.tsdb.enable=false",
        "-e", "canal.instance.gtidon=false",
        "-e", "canal.instance.mysql.slaveId=$SlaveID",
        "-e", "canal.instance.filter.regex=$DatabaseFilter",
        $CanalImage
    )
    & docker @args
    Assert-LastExit "docker run $Name"
}

function Wait-TcpEndpoint {
    param(
        [string]$HostName,
        [int]$Port,
        [string]$Name
    )
    for ($i = 0; $i -lt 60; $i++) {
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $iar = $client.BeginConnect($HostName, $Port, $null, $null)
            if ($iar.AsyncWaitHandle.WaitOne([TimeSpan]::FromSeconds(1))) {
                $client.EndConnect($iar)
                return
            }
        } catch {
            # Not ready. / 未就绪。 / 未準備。
        } finally {
            $client.Close()
        }
        Start-Sleep -Seconds 2
    }
    throw "Canal endpoint not ready: $Name $HostName`:$Port"
}

if (-not $SkipLabPrepare) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root "scripts/lab-smoke.ps1")
}

Grant-CanalPrivileges "nodebridge-mysql-edge-a"
Grant-CanalPrivileges "nodebridge-mysql-server"

Start-CanalContainer `
    -Name "nodebridge-canal-edge-a" `
    -Destination "edge-001" `
    -DatabaseFilter "scada_edge\..*" `
    -MySQLPort 3307 `
    -HostPort 11111 `
    -SlaveID 301

Start-CanalContainer `
    -Name "nodebridge-canal-server" `
    -Destination "server-001" `
    -DatabaseFilter "scada_center\..*" `
    -MySQLPort 3309 `
    -HostPort 11113 `
    -SlaveID 303

Wait-TcpEndpoint -HostName "127.0.0.1" -Port 11111 -Name "edge canal"
Wait-TcpEndpoint -HostName "127.0.0.1" -Port 11113 -Name "server canal"

Write-Host "Canal lab is ready"
Write-Host "Edge A Canal: 127.0.0.1:11111 destination=edge-001"
Write-Host "Server Canal: 127.0.0.1:11113 destination=server-001"
