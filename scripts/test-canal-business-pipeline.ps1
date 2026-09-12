param([string]$CandidatePath = '', [ValidateSet('EDGE_TO_SERVER','SERVER_TO_EDGE')][string]$Direction = 'EDGE_TO_SERVER', [switch]$Bidirectional, [switch]$AlignmentCapture)
$ErrorActionPreference='Stop'
if($Bidirectional -and $AlignmentCapture){throw 'Choose one owned pipeline scenario'}
$repo=[IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$run=[guid]::NewGuid().ToString('N')
$root=Join-Path $repo ".cache/canal-business/$run"
New-Item -ItemType Directory -Path $root -Force | Out-Null
$docker='C:/Program Files/Docker/Docker/resources/bin/docker.exe'
$containers=[Collections.Generic.List[string]]::new()
$network=''
$saved=@{}
foreach($name in @('NODEBRIDGE_OWNED_CDC_FIXTURE','NODEBRIDGE_CDC_FIXTURE_ROOT','NODEBRIDGE_CDC_TEST_DSN','NODEBRIDGE_CDC_TEST_RABBITMQ_URL','NODEBRIDGE_CDC_TEST_CANAL_ADDR','NODEBRIDGE_CDC_TEST_SECOND_CANAL_ADDR','NODEBRIDGE_CDC_DIRECTION')){$saved[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
function Docker-ID([string[]]$Arguments){
    $output=& $docker @Arguments
    if($LASTEXITCODE -ne 0){throw 'owned Docker fixture creation failed'}
    $id=([string]($output|Select-Object -Last 1)).Trim()
    if($id -notmatch '^[a-f0-9]{64}$'){throw 'unexpected Docker resource ID'}
    return $id
}
function Port([string]$ID,[int]$Internal){
    $value=& $docker port $ID "$Internal/tcp"
    if($LASTEXITCODE -ne 0 -or $value -notmatch '^127\.0\.0\.1:(\d+)$'){throw 'fixture port must bind only loopback'}
    return [int]$Matches[1]
}
try{
    if($CandidatePath){
        Copy-Item -LiteralPath $CandidatePath -Destination (Join-Path $root 'SyncAgent.exe')
    }else{
        & go build -o (Join-Path $root 'SyncAgent.exe') ./cmd/sync-agent
        if($LASTEXITCODE -ne 0){throw 'candidate build failed'}
    }
    $network=Docker-ID @('network','create',"nb-cdc-$run")
    $mysql=Docker-ID @('run','-d','--rm','--network',$network,'--network-alias','owned-mysql','--name',"nb-cdc-mysql-$run",'--cpus','1','--memory','768m','-p','127.0.0.1::3306','-e','MYSQL_ROOT_PASSWORD=owned_cdc_fixture_only','-e','MYSQL_ROOT_HOST=%','mysql:8.0.38','--server-id=98101','--log-bin=mysql-bin','--binlog-format=ROW','--binlog-row-image=FULL','--default-authentication-plugin=mysql_native_password')
    $containers.Add($mysql)
    $rabbit=Docker-ID @('run','-d','--rm','--network',$network,'--name',"nb-cdc-rabbit-$run",'--cpus','1','--memory','512m','-p','127.0.0.1::5672','-e','RABBITMQ_DEFAULT_USER=owned_cdc','-e','RABBITMQ_DEFAULT_PASS=owned_cdc_fixture_only','rabbitmq:3-management')
    $containers.Add($rabbit)
    $deadline=(Get-Date).AddSeconds(90)
    $mysqlReady=$false
    do{
        $probe=& $docker exec $mysql mysql --protocol=tcp --host=127.0.0.1 --user=root --password=owned_cdc_fixture_only --batch --skip-column-names --execute='SELECT 1' 2>&1
        $mysqlReady=$LASTEXITCODE -eq 0
        if($mysqlReady){break}
        Start-Sleep -Milliseconds 500
    }while((Get-Date)-lt $deadline)
    if(-not $mysqlReady){throw "owned MySQL readiness timed out: $probe"}
    $instance=@'
canal.instance.mysql.slaveId=98102
canal.instance.gtidon=false
canal.instance.master.address=owned-mysql:3306
canal.instance.master.journal.name=
canal.instance.master.position=
canal.instance.master.timestamp=
canal.instance.master.gtid=
canal.instance.tsdb.enable=true
canal.instance.dbUsername=root
canal.instance.dbPassword=owned_cdc_fixture_only
canal.instance.connectionCharset=UTF-8
canal.instance.enableDruid=false
canal.instance.filter.regex=nb_cdc_source\\.(source_rows|sync_capture_fence)
canal.instance.filter.black.regex=mysql\\.slave_.*
canal.mq.topic=example
canal.mq.partition=0
'@
    $instancePath=Join-Path $root 'instance.properties'
    $instance | Set-Content -LiteralPath $instancePath -Encoding ascii
    $canal=Docker-ID @('run','-d','--rm','--network',$network,'--name',"nb-cdc-canal-$run",'--cpus','1','--memory','768m','-p','127.0.0.1::11111','-e','JAVA_OPTS=-Xms128m -Xmx512m -Xmn64m','--mount',"type=bind,source=$instancePath,target=/home/admin/canal-server/conf/example/instance.properties,readonly",'canal/canal-server:latest')
    $containers.Add($canal)
    $mysqlPort=Port $mysql 3306
    $rabbitPort=Port $rabbit 5672
    $canalPort=Port $canal 11111
    $readyPorts=@(@($rabbitPort,'RabbitMQ'),@($canalPort,'Canal'))
    if($Bidirectional -or $AlignmentCapture){
        $secondPath=Join-Path $root 'second-instance.properties'
        $instance.Replace('slaveId=98102','slaveId=98103').Replace('nb_cdc_source','nb_cdc_target').Replace('source_rows','target_rows') | Set-Content -LiteralPath $secondPath -Encoding ascii
        $secondCanal=Docker-ID @('run','-d','--rm','--network',$network,'--name',"nb-cdc-second-canal-$run",'--cpus','1','--memory','768m','-p','127.0.0.1::11111','-e','JAVA_OPTS=-Xms128m -Xmx512m -Xmn64m','--mount',"type=bind,source=$secondPath,target=/home/admin/canal-server/conf/example/instance.properties,readonly",'canal/canal-server:latest')
        $containers.Add($secondCanal)
        $secondPort=Port $secondCanal 11111
        $readyPorts+=,@($secondPort,'Second Canal')
        $env:NODEBRIDGE_CDC_TEST_SECOND_CANAL_ADDR="127.0.0.1:$secondPort"
    }
    foreach($entry in $readyPorts){
        $ready=$false;$deadline=(Get-Date).AddSeconds(45)
        do{$client=[Net.Sockets.TcpClient]::new();try{$connect=$client.ConnectAsync('127.0.0.1',[int]$entry[0]);$ready=$connect.Wait(500)-and $client.Connected}catch{}finally{$client.Dispose()};if(-not $ready){Start-Sleep -Milliseconds 500}}while(-not $ready -and (Get-Date)-lt $deadline)
        if(-not $ready){throw "owned $($entry[1]) readiness timed out"}
    }
    $env:NODEBRIDGE_OWNED_CDC_FIXTURE='1'
    $env:NODEBRIDGE_CDC_DIRECTION=$Direction
    $env:NODEBRIDGE_CDC_FIXTURE_ROOT=$root
    $env:NODEBRIDGE_CDC_TEST_DSN="root:owned_cdc_fixture_only@tcp(127.0.0.1:$mysqlPort)/"
    $env:NODEBRIDGE_CDC_TEST_RABBITMQ_URL="amqp://owned_cdc:owned_cdc_fixture_only@127.0.0.1:$rabbitPort/"
    $env:NODEBRIDGE_CDC_TEST_CANAL_ADDR="127.0.0.1:$canalPort"
    $testName='^TestOwnedCanalBusinessPipeline$'
    if($Bidirectional){$testName='^TestOwnedBidirectionalPipeline$'}
    if($AlignmentCapture){$testName='^TestOwnedAlignmentCaptureProbe$'}
    & go test ./cmd/sync-agent -run $testName -count=1 -timeout=180s -v 2>&1 | Tee-Object -FilePath (Join-Path $root 'test-output.txt')
    if($LASTEXITCODE -ne 0){throw "CDC fixture failed; evidence at $root"}
    [ordered]@{passed=$true;direction=$Direction;bidirectional=[bool]$Bidirectional;candidate_sha256=(Get-FileHash -LiteralPath (Join-Path $root 'SyncAgent.exe')).Hash;mysql_port=$mysqlPort;rabbitmq_port=$rabbitPort;canal_port=$canalPort;scope='owned containers and candidate CLI; not installed product'} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $root 'evidence.json') -Encoding utf8
    Write-Host "Evidence: $root"
}finally{
    foreach($id in $containers){
        & $docker logs --tail 80 $id 2>&1 | Out-File -LiteralPath (Join-Path $root "$id.log") -Encoding utf8
        & $docker rm --force $id | Out-Null
    }
    if($network){& $docker network rm $network | Out-Null}
    foreach($name in $saved.Keys){[Environment]::SetEnvironmentVariable($name,$saved[$name],'Process')}
}
