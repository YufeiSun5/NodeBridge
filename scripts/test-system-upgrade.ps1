param([string]$Agent = "", [string]$OutputDirectory = "")
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root 'scripts/lib/env.ps1') -RepoRoot $root
if (-not $Agent) { $Agent = Join-Path $root 'build/bin/SyncAgent.exe' }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root ('.cache/system-upgrade/' + [guid]::NewGuid().ToString('N')) }
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$container = ''
$results = @()
function Invoke-OwnedSQL([string]$database, [string]$sql) {
    $result = $sql | & docker exec -i -e 'MYSQL_PWD=owned-upgrade-fixture-only' $container mysql -uroot --batch --skip-column-names $database
    if ($LASTEXITCODE -ne 0) { throw 'Owned fixture SQL failed' }
    return $result
}
try {
    $container = (& docker run -d --rm --label nodebridge.test=system-upgrade -e MYSQL_ROOT_PASSWORD=owned-upgrade-fixture-only -e MYSQL_ROOT_HOST=% -p '127.0.0.1::3306' mysql:8.4).Trim()
    if ($LASTEXITCODE -ne 0 -or $container -notmatch '^[a-f0-9]{64}$') { throw 'Owned MySQL creation failed' }
    $ready = $false
    for ($i=0; $i -lt 90; $i++) {
        & docker exec -e 'MYSQL_PWD=owned-upgrade-fixture-only' $container mysqladmin -uroot '--host=127.0.0.1' ping *> $null
        if ($LASTEXITCODE -eq 0) { $ready=$true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw 'Owned MySQL startup timed out' }
    $endpoint = (& docker port $container 3306).Trim()
    if ($endpoint -notmatch '^127\.0\.0\.1:(\d+)$') { throw 'Unexpected fixture endpoint' }
    $port = [int]$Matches[1]
    foreach ($scope in @('edge','server')) {
        $database = "owned_upgrade_$scope"
        Invoke-OwnedSQL 'mysql' "CREATE DATABASE $database CHARACTER SET utf8mb4;" | Out-Null
        foreach ($file in Get-ChildItem -LiteralPath (Join-Path $root "migrations/$scope") -Filter '*.sql' | Where-Object Name -lt '010') {
            Invoke-OwnedSQL $database (Get-Content -LiteralPath $file.FullName -Raw) | Out-Null
        }
        Invoke-OwnedSQL $database "CREATE TABLE business_guard (id INT PRIMARY KEY, payload VARCHAR(32)); INSERT INTO business_guard VALUES (1,'keep-original'); INSERT INTO sync_alignment_cutover VALUES ('owned-job','owned-proof',JSON_OBJECT('keep','original'),'ACTIVE','peer',NOW(6));" | Out-Null
        $config = Join-Path $OutputDirectory "$scope.yaml"
        "mode: $scope`nnode:`n  id: owned-upgrade-$scope`nmysql:`n  host: 127.0.0.1`n  port: $port`n  username: root`n  password: owned-upgrade-fixture-only`n  database: $database`n" | Set-Content -LiteralPath $config -Encoding utf8
        foreach ($attempt in 1,2) {
            $output = & $Agent upgrade-system -config $config
            if ($LASTEXITCODE -ne 0) { throw "Upgrade failed: $scope/$attempt" }
            $report = ($output -join "`n") | ConvertFrom-Json
            if ($report.status -ne 'upgraded' -or $report.scope -ne $scope) { throw 'Wrong upgrade result' }
            $output | Set-Content -LiteralPath (Join-Path $OutputDirectory "$scope-$attempt.json")
        }
        $actual = @(Invoke-OwnedSQL $database "SELECT payload FROM business_guard; SELECT phase FROM sync_alignment_cutover WHERE job_id='owned-job'; SELECT JSON_UNQUOTE(JSON_EXTRACT(proof_json,'$.keep')) FROM sync_alignment_cutover WHERE job_id='owned-job'; SELECT COUNT(*) FROM sync_schema_migration; SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('sync_replay_marker','sync_replay_position'); SELECT version FROM sync_schema_version;")
        $expectedCount = @(Get-ChildItem -LiteralPath (Join-Path $root "migrations/$scope") -Filter '*.sql').Count
        if ($actual[0] -ne 'keep-original' -or $actual[1] -ne 'ACTIVE' -or $actual[2] -ne 'original' -or [int]$actual[3] -ne $expectedCount -or [int]$actual[4] -ne 2 -or $actual[5] -ne $report.version) { throw "Upgrade preservation check failed: $scope" }
        $results += @{scope=$scope;repeated=$true;business_and_active_proof_preserved=$true;migrations=$expectedCount;version=$report.version}
        $freshDatabase = "owned_fresh_$scope"
        $freshConfig = Join-Path $OutputDirectory "$scope-fresh.yaml"
        (Get-Content -LiteralPath $config -Raw).Replace("database: $database", "database: $freshDatabase") | Set-Content -LiteralPath $freshConfig -Encoding utf8
        $missingOutput = & $Agent upgrade-system -config $freshConfig 2>&1
        if ($LASTEXITCODE -eq 0 -or ($missingOutput -join "`n") -notmatch 'Unknown database') { throw "Missing database was not rejected without explicit initialization: $scope" }
        foreach ($attempt in 1,2) {
            $freshOutput = & $Agent upgrade-system -config $freshConfig -create-database
            if ($LASTEXITCODE -ne 0) { throw "Fresh database initialization failed: $scope/$attempt" }
            $freshOutput | Set-Content -LiteralPath (Join-Path $OutputDirectory "$scope-fresh-$attempt.json")
        }
        $freshActual = @(Invoke-OwnedSQL $freshDatabase "SELECT COUNT(*) FROM sync_schema_migration; SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name NOT LIKE 'sync\\_%'; SELECT COUNT(*) FROM sync_alignment_cutover;")
        if ([int]$freshActual[0] -ne $expectedCount -or [int]$freshActual[1] -ne 0 -or [int]$freshActual[2] -ne 0) { throw "Fresh database system-only check failed: $scope" }
        $results += @{scope=$scope;fresh_database=$true;repeated=$true;system_tables_only=$true;migrations=$expectedCount}
    }
    @{passed=$true;owned_container=$container;results=$results} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $OutputDirectory 'evidence.json')
    Write-Output "PASS: edge/server old schemas upgraded twice, original rows and ACTIVE proofs preserved. Evidence: $OutputDirectory"
} finally {
    if ($container -match '^[a-f0-9]{64}$') {
        & docker logs $container *> (Join-Path $OutputDirectory 'mysql.log')
        $label = & docker inspect --format '{{ index .Config.Labels "nodebridge.test" }}' $container
        if ($LASTEXITCODE -eq 0 -and $label.Trim() -eq 'system-upgrade') { & docker rm -f $container | Out-Null }
    }
}
