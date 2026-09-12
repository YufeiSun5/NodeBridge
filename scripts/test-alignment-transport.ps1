param()
$ErrorActionPreference = 'Stop'
$docker = 'C:/Program Files/Docker/Docker/resources/bin/docker.exe'
$repo = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$run = [Guid]::NewGuid().ToString('N')
$root = Join-Path $repo ".cache/alignment-transport/$run"
New-Item -ItemType Directory -Path $root -Force | Out-Null
$containers = [Collections.Generic.List[object]]::new()
$saved = @{}
foreach ($key in @('NODEBRIDGE_OWNED_ALIGNMENT_TRANSPORT', 'NODEBRIDGE_ALIGNMENT_SOURCE_DSN', 'NODEBRIDGE_ALIGNMENT_TARGET_DSN', 'NODEBRIDGE_ALIGNMENT_BROKER_URL')) {
    $saved[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
}
function New-OwnedContainer([string]$Name, [string[]]$Arguments) {
    $output = & $docker run -d --name $Name --label "nodebridge.test.owner=$run" @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Failed to create $Name" }
    $id = ([string]($output | Select-Object -Last 1)).Trim()
    if ($id -notmatch '^[a-f0-9]{64}$') { throw 'Unexpected owned container id' }
    $containers.Add([pscustomobject]@{ ID = $id; Name = $Name })
    return $id
}
function Get-OwnedPort([string]$ID, [int]$Port) {
    $output = & $docker port $ID "$Port/tcp"
    if ($LASTEXITCODE -ne 0 -or $output -notmatch '^127\.0\.0\.1:(\d+)$') { throw 'Expected loopback-only fixture port' }
    return [int]$Matches[1]
}
try {
    $mysql = @()
    $passwords = @('owned_snapshot_source_only', 'owned_snapshot_target_only')
    for ($i = 0; $i -lt 2; $i++) {
        $mysql += New-OwnedContainer "nb-alignment-mysql-$i-$run" @('--cpus', '1', '--memory', '768m', '-p', '127.0.0.1::3306', '-e', "MYSQL_ROOT_PASSWORD=$($passwords[$i])", '-e', 'MYSQL_ROOT_HOST=%', 'mysql:8.4', '--binlog-format=ROW', '--binlog-row-image=FULL')
    }
    $rabbit = New-OwnedContainer "nb-alignment-rabbit-$run" @('--cpus', '1', '--memory', '512m', '-p', '127.0.0.1::5672', '-e', 'RABBITMQ_DEFAULT_USER=owned_alignment', '-e', 'RABBITMQ_DEFAULT_PASS=owned_alignment_transport_only', 'rabbitmq:3-management')
    $ports = @()
    for ($i = 0; $i -lt 2; $i++) {
        $ready = $false
        $deadline = (Get-Date).AddSeconds(90)
        do {
            $savedPreference = $ErrorActionPreference
            try {
                $ErrorActionPreference = 'Continue'
                $null = & $docker exec $mysql[$i] mysql --protocol=tcp --host=127.0.0.1 --connect-timeout=2 --user=root "--password=$($passwords[$i])" --batch --skip-column-names --execute='SELECT 1' 2>&1
                $ready = $LASTEXITCODE -eq 0
            } finally {
                $ErrorActionPreference = $savedPreference
            }
            if (-not $ready) { Start-Sleep -Milliseconds 500 }
        } while (-not $ready -and (Get-Date) -lt $deadline)
        if (-not $ready) { throw "Owned MySQL $i did not become ready" }
        $ports += Get-OwnedPort $mysql[$i] 3306
    }
    $rabbitPort = Get-OwnedPort $rabbit 5672
    $ready = $false
    $deadline = (Get-Date).AddSeconds(45)
    do {
        $client = [Net.Sockets.TcpClient]::new()
        try { $ready = $client.ConnectAsync('127.0.0.1', $rabbitPort).Wait(500) -and $client.Connected } catch {} finally { $client.Dispose() }
        if (-not $ready) { Start-Sleep -Milliseconds 500 }
    } while (-not $ready -and (Get-Date) -lt $deadline)
    if (-not $ready) { throw 'Owned RabbitMQ did not become ready' }
    $env:NODEBRIDGE_OWNED_ALIGNMENT_TRANSPORT = '1'
    $env:NODEBRIDGE_ALIGNMENT_SOURCE_DSN = "root:$($passwords[0])@tcp(127.0.0.1:$($ports[0]))/"
    $env:NODEBRIDGE_ALIGNMENT_TARGET_DSN = "root:$($passwords[1])@tcp(127.0.0.1:$($ports[1]))/"
    $env:NODEBRIDGE_ALIGNMENT_BROKER_URL = "amqp://owned_alignment:owned_alignment_transport_only@127.0.0.1:$rabbitPort/"
    & go test ./internal/alignment -run '^TestOwnedSnapshotAMQPTransport$' -count=1 -v -timeout=60s 2>&1 | Tee-Object -FilePath (Join-Path $root 'test-output.txt')
    if ($LASTEXITCODE -ne 0) { throw "Owned snapshot transport failed: $root" }
    [ordered]@{ passed = $true; scope = 'two distinct owned MySQL servers and RabbitMQ; no CDC cutover or installed product'; completed_at = (Get-Date).ToString('o') } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $root 'evidence.json') -Encoding utf8
    Write-Host "Evidence: $root"
} finally {
    foreach ($container in $containers) {
        $info = (& $docker inspect $container.ID | ConvertFrom-Json)[0]
        if ($LASTEXITCODE -ne 0 -or $info.Id -ne $container.ID -or $info.Name -ne "/$($container.Name)" -or $info.Config.Labels.'nodebridge.test.owner' -ne $run) {
            throw 'Fixture ownership verification failed; cleanup refused'
        }
        & $docker logs --tail 60 $container.ID 2>&1 | Out-File -LiteralPath (Join-Path $root "$($container.ID).log") -Encoding utf8
        $null = & $docker rm -f $container.ID
        if ($LASTEXITCODE -ne 0) { throw 'Owned fixture cleanup failed' }
    }
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key, $saved[$key], 'Process') }
}
