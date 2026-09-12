param()
$ErrorActionPreference = 'Stop'
$docker = 'C:/Program Files/Docker/Docker/resources/bin/docker.exe'
$id = [Guid]::NewGuid().ToString('N')
$name = "nb-alignment-core-$id"
$container = $null
$previousDSN = $env:NODEBRIDGE_ALIGNMENT_TEST_DSN
try {
    $container = (& $docker run -d --name $name --label "nodebridge.test.owner=$id" --cpus=1 -e MYSQL_ROOT_PASSWORD=owned_alignment_fixture_only -e MYSQL_ROOT_HOST=% -p 127.0.0.1::3306 mysql:8.4 --binlog-format=ROW --binlog-row-image=FULL).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Failed to create owned MySQL fixture' }
    $ready = $false
    for ($i = 0; $i -lt 60; $i++) {
        # Windows PowerShell treats redirected native stderr as an ErrorRecord.
        $savedPreference = $ErrorActionPreference
        try {
            $ErrorActionPreference = 'Continue'
            $null = & $docker exec $container mysql --protocol=tcp --host=127.0.0.1 --user=root --password=owned_alignment_fixture_only --batch --skip-column-names --execute='SELECT 1' 2>&1
        } finally {
            $ErrorActionPreference = $savedPreference
        }
        if ($LASTEXITCODE -eq 0) { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw 'Owned MySQL fixture did not become ready' }
    $info = (& $docker inspect $container | ConvertFrom-Json)[0]
    $port = $info.NetworkSettings.Ports.'3306/tcp'[0].HostPort
    $env:NODEBRIDGE_ALIGNMENT_TEST_DSN = "root:owned_alignment_fixture_only@tcp(127.0.0.1:$port)/"
    & go test ./internal/conflict -run '^TestRealMySQLVersionTransactionsAndEmptyDirection$' -count=1 -v -timeout 60s
    if ($LASTEXITCODE -ne 0) { throw 'Alignment/conflict component integration failed' }
} finally {
    $env:NODEBRIDGE_ALIGNMENT_TEST_DSN = $previousDSN
    if ($container) {
        $owned = (& $docker inspect $container | ConvertFrom-Json)[0]
        if ($owned.Id -ne $container -or $owned.Config.Labels.'nodebridge.test.owner' -ne $id -or $owned.Name -ne "/$name") {
            throw 'Fixture ownership mismatch; cleanup refused'
        }
        $null = & $docker rm -f $container
        if ($LASTEXITCODE -ne 0) { throw 'Owned fixture cleanup failed' }
    }
}
