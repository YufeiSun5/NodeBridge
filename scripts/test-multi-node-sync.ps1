param(
    [string]$CandidatePath = '',
    [ValidateSet('edge1','server','edge2','empty')][string]$Source = 'server',
    [switch]$LargeSnapshot,
    [switch]$InterCopyWrites,
	[switch]$Reconnect,
    [switch]$SharedRule,
	[switch]$Rebaseline,
    [switch]$InterruptAfterFirstCopy,
    [ValidateRange(0,25200)][int]$SoakSeconds = 0,
    [ValidateRange(1,16)][int]$BatchSize = 16
)
$ErrorActionPreference = 'Stop'
if ($InterCopyWrites -and $InterruptAfterFirstCopy) { throw 'Run inter-copy writes and partial interruption as separate scenarios' }
$repo = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$run = [guid]::NewGuid().ToString('N')
$root = Join-Path $repo ".cache/multi-node-sync/$run"
New-Item -ItemType Directory -Path $root -Force | Out-Null
$docker = 'C:/Program Files/Docker/Docker/resources/bin/docker.exe'
$containers = [Collections.Generic.List[string]]::new()
$canals = [Collections.Generic.List[string]]::new()
$network = ''
$names = @('NODEBRIDGE_OWNED_MULTI_FIXTURE','NODEBRIDGE_MULTI_FIXTURE_ROOT','NODEBRIDGE_MULTI_ENDPOINTS','NODEBRIDGE_MULTI_SOURCE','NODEBRIDGE_MULTI_LARGE','NODEBRIDGE_MULTI_INTER_COPY','NODEBRIDGE_MULTI_SHARED_RULE','NODEBRIDGE_MULTI_PARTIAL_RETRY','NODEBRIDGE_MULTI_SOAK_SECONDS')
$names += 'NODEBRIDGE_MULTI_BATCH_SIZE'
$names += 'NODEBRIDGE_MULTI_RECONNECT'
$saved = @{}
$canalMemory = if ($LargeSnapshot) { '3g' } else { '768m' }
$canalJava = if ($LargeSnapshot) { 'JAVA_OPTS=-Xms256m -Xmx2g -Xmn256m' } else { 'JAVA_OPTS=-Xms128m -Xmx512m -Xmn64m' }
$canalMemUnit = if ($LargeSnapshot) { 32768 } else { 1024 }
foreach ($name in $names) { $saved[$name] = [Environment]::GetEnvironmentVariable($name,'Process') }
function Docker-ID([string[]]$Arguments) {
    $output = & $docker @Arguments
    if ($LASTEXITCODE -ne 0) { throw 'Owned Docker resource creation failed' }
    $id = ([string]($output | Select-Object -Last 1)).Trim()
    if ($id -notmatch '^[a-f0-9]{64}$') { throw 'Unexpected Docker resource ID' }
    return $id
}
function Port([string]$ID, [int]$Internal) {
    $value = & $docker port $ID "$Internal/tcp"
    if ($LASTEXITCODE -ne 0 -or $value -notmatch '^127\.0\.0\.1:(\d+)$') { throw 'Owned port must bind loopback' }
    return [int]$Matches[1]
}
try {
    if ($CandidatePath) {
        Copy-Item -LiteralPath $CandidatePath -Destination (Join-Path $root 'SyncAgent.exe')
    } else {
        & go build -o (Join-Path $root 'SyncAgent.exe') ./cmd/sync-agent
        if ($LASTEXITCODE -ne 0) { throw 'Candidate build failed' }
    }
    Copy-Item -LiteralPath (Join-Path $repo 'migrations') -Destination (Join-Path $root 'migrations') -Recurse
    $network = Docker-ID @('network','create',"nb-multi-$run")
    $mysqls = @()
    $endpoints = @()
    foreach ($i in 0..2) {
        $id = Docker-ID @('run','-d','--rm','--network',$network,'--network-alias',"owned-mysql-$i",'--name',"nb-multi-mysql-$i-$run",'--cpus','1','--memory','768m','-p','127.0.0.1::3306','-e','MYSQL_ROOT_PASSWORD=owned_multi_only','-e','MYSQL_ROOT_HOST=%','mysql:8.0.38',"--server-id=$(98200+$i)",'--log-bin=mysql-bin','--binlog-format=ROW','--binlog-row-image=FULL','--default-authentication-plugin=mysql_native_password')
        $containers.Add($id)
        $mysqls += $id
        $endpoints += [ordered]@{ mysql_port = (Port $id 3306); canal_addr = ''; local_url = '' }
    }
    $rabbit = Docker-ID @('run','-d','--rm','--network',$network,'--name',"nb-multi-rabbit-$run",'--cpus','1','--memory','1536m','-p','127.0.0.1::5672','-e','RABBITMQ_DEFAULT_USER=owned_multi','-e','RABBITMQ_DEFAULT_PASS=owned_multi_only','rabbitmq:3-management')
    $containers.Add($rabbit)
    foreach ($database in $mysqls) {
        $deadline = (Get-Date).AddSeconds(90)
        do {
            $null = & $docker exec $database mysql --protocol=tcp --host=127.0.0.1 --user=root --password=owned_multi_only --batch --skip-column-names --execute='SELECT 1' 2>&1
            $ready = $LASTEXITCODE -eq 0
            if (-not $ready) { Start-Sleep -Milliseconds 400 }
        } while (-not $ready -and (Get-Date) -lt $deadline)
        if (-not $ready) { throw 'Owned MySQL readiness timed out' }
    }
    $deadline = (Get-Date).AddSeconds(60)
    do {
        $null = & $docker exec $rabbit rabbitmq-diagnostics -q ping 2>&1
        $ready = $LASTEXITCODE -eq 0
        if (-not $ready) { Start-Sleep -Milliseconds 500 }
    } while (-not $ready -and (Get-Date) -lt $deadline)
    if (-not $ready) { throw 'Owned RabbitMQ readiness timed out' }
    $rabbitPort = Port $rabbit 5672
    $broker = "amqp://owned_multi:owned_multi_only@127.0.0.1:$rabbitPort/"
    foreach ($i in 0..2) {
        $vhost = "owned-node-$i"
        $null = & $docker exec $rabbit rabbitmqctl add_vhost $vhost
        if ($LASTEXITCODE -ne 0) { throw 'Owned vhost creation failed' }
        $null = & $docker exec $rabbit rabbitmqctl set_permissions -p $vhost owned_multi '.*' '.*' '.*'
        if ($LASTEXITCODE -ne 0) { throw 'Owned vhost permission failed' }
        $endpoints[$i].local_url = $broker + $vhost
        $database = @('nb_multi_edge1','nb_multi_server','nb_multi_edge2')[$i]
        $table = @('edge_rows','central_rows','branch_rows')[$i]
        if ($SharedRule) { $database = 'nb_multi_shared'; $table = 'shared_rows' }
        $instance = @"
canal.instance.mysql.slaveId=$(98300+$i)
canal.instance.gtidon=false
canal.instance.master.address=owned-mysql-${i}:3306
canal.instance.master.journal.name=
canal.instance.master.position=
canal.instance.master.timestamp=
canal.instance.master.gtid=
canal.instance.tsdb.enable=true
canal.instance.dbUsername=root
canal.instance.dbPassword=owned_multi_only
canal.instance.connectionCharset=UTF-8
canal.instance.enableDruid=false
canal.instance.memory.buffer.size=16384
canal.instance.memory.buffer.memunit=$canalMemUnit
canal.instance.memory.batch.mode=MEMSIZE
canal.instance.filter.regex=$database\\.($table|sync_capture_fence)
canal.instance.filter.black.regex=mysql\\.slave_.*
canal.mq.topic=example
canal.mq.partition=0
"@
		if ($Rebaseline) { $instance = $instance -replace '(?m)^canal.instance.filter.regex=.*$', 'canal.instance.filter.regex=.*\\.(edge_rows|central_rows|sync_capture_fence)' }
        $path = Join-Path $root "instance-$i.properties"
        $instance | Set-Content -LiteralPath $path -Encoding ascii
        $canal = Docker-ID @('run','-d','--rm','--network',$network,'--name',"nb-multi-canal-$i-$run",'--cpus','1','--memory',$canalMemory,'-p','127.0.0.1::11111','-e',$canalJava,'--mount',"type=bind,source=$path,target=/home/admin/canal-server/conf/example/instance.properties,readonly",'canal/canal-server:latest')
        $containers.Add($canal)
        $canals.Add($canal)
        $port = Port $canal 11111
        $endpoints[$i].canal_addr = "127.0.0.1:$port"
    }
    foreach ($endpoint in $endpoints) {
        $port = [int]($endpoint.canal_addr.Split(':')[-1])
        $deadline = (Get-Date).AddSeconds(60)
        do {
            $client = [Net.Sockets.TcpClient]::new()
            $ready = $false
            try { $task = $client.ConnectAsync('127.0.0.1',$port); $ready = $task.Wait(500) -and $client.Connected } catch {} finally { $client.Dispose() }
            if (-not $ready) { Start-Sleep -Milliseconds 500 }
        } while (-not $ready -and (Get-Date) -lt $deadline)
        if (-not $ready) { throw 'Owned Canal readiness timed out' }
    }
    $env:NODEBRIDGE_OWNED_MULTI_FIXTURE = '1'
    $env:NODEBRIDGE_MULTI_FIXTURE_ROOT = $root
    $env:NODEBRIDGE_MULTI_ENDPOINTS = ConvertTo-Json -InputObject $endpoints -Compress
    $env:NODEBRIDGE_MULTI_SOURCE = $Source
    $env:NODEBRIDGE_MULTI_LARGE = [string][int][bool]$LargeSnapshot
    $env:NODEBRIDGE_MULTI_INTER_COPY = [string][int][bool]$InterCopyWrites
    $env:NODEBRIDGE_MULTI_SHARED_RULE = [string][int][bool]$SharedRule
    $env:NODEBRIDGE_MULTI_PARTIAL_RETRY = [string][int][bool]$InterruptAfterFirstCopy
    $env:NODEBRIDGE_MULTI_SOAK_SECONDS = [string]$SoakSeconds
    $env:NODEBRIDGE_MULTI_BATCH_SIZE = [string]$BatchSize
	$env:NODEBRIDGE_MULTI_RECONNECT = [string][int][bool]$Reconnect
    [ordered]@{supervisor_pid=$PID;root=$root;containers=@($containers);network=$network;endpoints=$endpoints;soak_seconds=$SoakSeconds;candidate_sha256=(Get-FileHash (Join-Path $root 'SyncAgent.exe')).Hash} | ConvertTo-Json -Depth 8 | Set-Content (Join-Path $root 'resources.json') -Encoding utf8
    $testTimeout = [string]($SoakSeconds + 900) + 's'
    Write-Host "Fixture root: $root"
	if ($Reconnect) {
		$reconnectPreviousURL = $env:NODEBRIDGE_RABBITMQ_URL
		try {
			$env:NODEBRIDGE_RABBITMQ_URL = $endpoints[1].local_url
			& go test ./internal/rabbitmq -run '^TestIntegrationSessionRecreatesTransportAndUncertainConfirm$' -count=1 -timeout=60s -v 2>&1 | Tee-Object -FilePath (Join-Path $root 'session-reconnect-test.txt')
			if ($LASTEXITCODE -ne 0) { throw 'Session recovery integration failed' }
		} finally { $env:NODEBRIDGE_RABBITMQ_URL = $reconnectPreviousURL }
	}
	$testName = if ($Rebaseline) { '^TestOwnedRebaselinePipeline$' } else { '^TestOwnedMultiNodePipeline$' }
	if ($Rebaseline) {
		& go test ./internal/alignment -run '^TestOwnedRebaselineTransactionRollback$' -count=1 -timeout=90s -v 2>&1 | Tee-Object -FilePath (Join-Path $root 'rebaseline-transaction-test.txt')
		if ($LASTEXITCODE -ne 0) { throw 'Rebaseline transaction rollback integration failed' }
	}
    & go test ./cmd/sync-agent -run $testName -count=1 "-timeout=$testTimeout" -v 2>&1 | Tee-Object -FilePath (Join-Path $root 'test-output.txt')
    if ($LASTEXITCODE -ne 0) { throw "Multi-node fixture failed: $root" }
    $testedNodes = if ($Rebaseline) { 2 } else { 3 }
    $testedSource = if ($Rebaseline) { 'edge' } else { $Source }
    [ordered]@{passed=$true;source=$testedSource;rebaseline=[bool]$Rebaseline;large_snapshot=[bool]$LargeSnapshot;inter_copy_writes=[bool]$InterCopyWrites;shared_rule=[bool]$SharedRule;partial_retry=[bool]$InterruptAfterFirstCopy;allocated_mysql_instances=3;allocated_canal_readers=3;mysql_instances=$testedNodes;canal_readers=$testedNodes;agents=$testedNodes;candidate_sha256=(Get-FileHash (Join-Path $root 'SyncAgent.exe')).Hash;scope='owned loopback containers and candidate MCP/CLI on one Windows host; no business deployment'} | ConvertTo-Json | Set-Content (Join-Path $root 'evidence.json') -Encoding utf8
    Write-Host "Evidence: $root"
} finally {
    foreach ($id in $canals) {
        & $docker exec $id tail -n 100 /home/admin/canal-server/logs/example/example.log 2>&1 | Out-File (Join-Path $root "$id-canal.log") -Encoding utf8
    }
    foreach ($id in $containers) {
        & $docker logs --tail 80 $id 2>&1 | Out-File (Join-Path $root "$id.log") -Encoding utf8
        & $docker rm --force $id | Out-Null
    }
    if ($network) { & $docker network rm $network | Out-Null }
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name,$saved[$name],'Process') }
}
