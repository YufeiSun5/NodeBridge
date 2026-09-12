param(
    [ValidateSet('Start','Run','Status','Stop')][string]$Action = 'Start',
    [string]$RunId = '',
    [switch]$Smoke,
    [ValidateRange(600,3600)][int]$SmokeDurationSeconds = 1800,
    [ValidateRange(25200,86400)][int]$FormalDurationSeconds = 25200,
    [switch]$BestEffort,
    [ValidateRange(1,100)][int]$DiskBudgetGB = 100,
    [string]$EdgeHost = '192.168.10.105',
    [string]$EdgeUser = 'xx',
    [string]$SSHKeyPath = "$HOME\.ssh\nodebridge_ed25519",
    [string]$VerifiedServerAgentPath = '',
    [ValidateSet('Installed','WorkspaceServer')][string]$RuntimeMode = 'Installed',
    [string]$VerifiedEdgeAgentPath = 'C:\Program Files\NodeBridge\app\SyncAgent.exe',
    [string]$ExpectedAgentHash = ''
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$Root = Split-Path -Parent $PSScriptRoot
$CacheRoot = Join-Path $Root '.cache\wide-seven-hour'
$CurrentPath = Join-Path $CacheRoot 'current.json'
$programFiles64=if($env:ProgramW6432){$env:ProgramW6432}else{$env:ProgramFiles}
$ServerAgentPath = Join-Path $programFiles64 'NodeBridge\app\SyncAgent.exe'
if ($VerifiedServerAgentPath) { $ServerAgentPath = $VerifiedServerAgentPath }
$EdgeAgentPath = $VerifiedEdgeAgentPath
$ServerConfigPath = 'C:\ProgramData\NodeBridge\config.yaml'
$ServerRulesPath = 'C:\ProgramData\NodeBridge\sync-rules.yaml'
$ServerStopPath = 'C:\ProgramData\NodeBridge\run\sync-agent.stop'
$EdgeConfigPath = $ServerConfigPath
$EdgeRulesPath = $ServerRulesPath
$EdgeStopPath = $ServerStopPath
$DockerPath = 'C:\Program Files\Docker\Docker\resources\bin\docker.exe'
$ServerRabbitMQContainer = 'rabbitmq'
$ServerRabbitVHost = '/nodebridge-server'
$Helper = Join-Path $CacheRoot 'labwide.exe'

# Import only vetted helper functions, never the old workload or its top-level actions.
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'lab-real-bidirectional-overnight.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw 'legacy helper parse failed' }
$imports = @('Write-JsonAtomic','Read-JsonFile','Get-EvidenceRoot','Test-ProcessAlive','Set-State','Add-Event','Invoke-External','Invoke-RemotePowerShell','Copy-FromEdge','Copy-ToEdge','Get-AgentProcess','Stop-Agent','Start-Agent','Get-ServerQueueSnapshot')
foreach ($name in $imports) {
    $function = $ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false) | Where-Object Name -eq $name
    if (@($function).Count -ne 1) { throw "helper not unique: $name" }
    . ([scriptblock]::Create($function.Extent.Text))
}
function Write-JsonAtomic {
    param([Parameter(Mandatory)][string]$Path,[Parameter(Mandatory)][object]$Value)
    $directory=Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    $temporary="$Path.$PID.$([guid]::NewGuid().ToString('N')).tmp"
    try {
        [IO.File]::WriteAllText($temporary,($Value|ConvertTo-Json -Depth 12),[Text.UTF8Encoding]::new($false))
        $timer=[Diagnostics.Stopwatch]::StartNew()
        while($true) {
            try {
                if([IO.File]::Exists($Path)){[IO.File]::Replace($temporary,$Path,[System.Management.Automation.Language.NullString]::Value)}
                else{[IO.File]::Move($temporary,$Path)}
                break
            } catch [IO.IOException] {
                $cause=$_.Exception.GetBaseException()
                $code=$cause.HResult -band 65535
                if($code -notin @(32,33,80,183) -or $timer.ElapsedMilliseconds -ge 2000){
                    throw [IO.IOException]::new("Atomic JSON replacement failed: $Path (code $code)",$cause)
                }
                Start-Sleep -Milliseconds 25
            }
        }
    } finally {
        if([IO.File]::Exists($temporary)){[IO.File]::Delete($temporary)}
    }
}
function Read-JsonFile([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    $stream=[IO.File]::Open($Path,[IO.FileMode]::Open,[IO.FileAccess]::Read,([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
    try {
        $reader=[IO.StreamReader]::new($stream,[Text.Encoding]::UTF8,$true)
        try { return $reader.ReadToEnd() | ConvertFrom-Json } finally { $reader.Dispose() }
    } finally { $stream.Dispose() }
}
function Assert-WideSiteFingerprint($Expected,$Actual) {
    foreach($name in @('server_config','edge_config','server_rules','edge_rules')) {
        if($Expected.$name -notmatch '^[A-Fa-f0-9]{64}$' -or $Actual.$name -ne $Expected.$name){throw "readiness site configuration changed or missing: $name"}
    }
}
function Assert-WideReadinessDuration($Ready,[int]$Duration) {
    if ($Ready.formal_duration_seconds -ne $Duration) { throw 'Readiness formal duration mismatch' }
}
function Assert-WideReadinessRuntime($Ready,[ValidateSet('Installed','WorkspaceServer')][string]$Mode,[string]$ServerPath,[string]$EdgePath) {
    $runtime=$Ready.runtime_verification
    if(-not $Ready.runtime_verified -or -not $runtime.passed -or $runtime.mode -ne $Mode){throw 'Readiness runtime mode mismatch'}
    foreach($side in @('Server','Edge')){
        $path=if($side -eq 'Server'){$ServerPath}else{$EdgePath}
        $kind=if($side -eq 'Server' -and $Mode -eq 'WorkspaceServer'){'workspace'}else{'installed'}
        if(-not $path -or -not $runtime.$side.path -or $runtime.$side.kind -ne $kind -or [IO.Path]::GetFullPath($runtime.$side.path) -ne [IO.Path]::GetFullPath($path)){throw "Readiness runtime path or kind mismatch: $side"}
    }
}
function Get-WideSiteFingerprint {
    $raw=Invoke-RemotePowerShell "@{edge_config=(Get-FileHash -LiteralPath '$EdgeConfigPath').Hash;edge_rules=(Get-FileHash -LiteralPath '$EdgeRulesPath').Hash}|ConvertTo-Json -Compress"
    $line=$raw|Where-Object {$_ -match '^\s*\{'}|Select-Object -Last 1
    if(-not $line){throw 'edge configuration fingerprint unavailable'}
    $remote=$line|ConvertFrom-Json
    return @{server_config=(Get-FileHash -LiteralPath $ServerConfigPath).Hash;server_rules=(Get-FileHash -LiteralPath $ServerRulesPath).Hash;edge_config=$remote.edge_config;edge_rules=$remote.edge_rules}
}
function Test-WideVerificationRetry([bool]$Verified,[DateTimeOffset]$Deadline,[DateTimeOffset]$Now=[DateTimeOffset]::UtcNow) {
    return -not $Verified -and $Now -lt $Deadline
}
function Get-WideRecoveryProgress($Snapshot,$Recovery,[ValidateRange(0,10000)][int]$MaxLagRows=80) {
    $lags=@{}
    foreach($node in @('edge-001','server-001')){
        if($null -eq $Snapshot.$node.source.count -or $null -eq $Snapshot.$node.incoming.count){throw "recovery snapshot missing counts: $node"}
    }
    $lags.Edge=[Math]::Max(0,[long]$Snapshot.'edge-001'.source.count-[long]$Snapshot.'server-001'.incoming.count)
    $lags.Server=[Math]::Max(0,[long]$Snapshot.'server-001'.source.count-[long]$Snapshot.'edge-001'.incoming.count)
    $watermark=$null -ne $Recovery.edge -and $null -ne $Recovery.server -and $Snapshot.'server-001'.incoming.count -ge $Recovery.edge -and $Snapshot.'edge-001'.incoming.count -ge $Recovery.server
    return @{ready=($watermark -and $lags.Edge -le $MaxLagRows -and $lags.Server -le $MaxLagRows);lags=$lags;max_lag_rows=$MaxLagRows}
}
if ($Action -eq 'Status') {
    $EvidenceRoot = Get-EvidenceRoot $RunId
    $state = Read-JsonFile (Join-Path $EvidenceRoot 'state.json')
    $state | Add-Member -NotePropertyName runner_alive -NotePropertyValue (Test-ProcessAlive $state.runner_pid) -Force
    $state | ConvertTo-Json -Depth 12
    exit 0
}
if ($Action -eq 'Stop') {
    $EvidenceRoot = Get-EvidenceRoot $RunId
    [IO.File]::WriteAllText((Join-Path $EvidenceRoot 'stop.requested'), (Get-Date).ToString('o'))
    'Controlled stop requested.'
    exit 0
}
if ($Action -eq 'Start') {
    if ($ExpectedAgentHash -notmatch '^[A-Fa-f0-9]{64}$') { throw 'An independently verified package agent hash is required' }
    New-Item -ItemType Directory -Force -Path $CacheRoot | Out-Null
    if (-not $Smoke -and -not $BestEffort) {
        $ready = Read-JsonFile (Join-Path $CacheRoot 'formal-ready.json')
        Assert-WideReadinessDuration $ready $FormalDurationSeconds
        Assert-WideReadinessRuntime $ready $RuntimeMode $ServerAgentPath $EdgeAgentPath
        if (-not $ready -or -not $ready.passed -or $ready.smoke_duration_seconds -lt 1800 -or $ready.agent_sha256 -ne $ExpectedAgentHash -or $ready.helper_sha256 -ne (Get-FileHash $Helper).Hash -or $ready.script_sha256 -ne (Get-FileHash $PSCommandPath).Hash) {
            throw 'Formal start requires new readiness proof from at least thirty minutes on the verified candidate runtimes.'
        }
        $smokeState=Read-JsonFile (Join-Path $ready.evidence_root 'state.json')
        $smokeRestore=Read-JsonFile (Join-Path $ready.evidence_root 'restoration.json')
        $smokeGates=Read-JsonFile (Join-Path $ready.evidence_root 'gates.json')
        if ($smokeState.status -ne 'completed' -or -not $smokeState.smoke -or -not $smokeRestore.passed -or -not $smokeGates.passed) { throw 'Formal readiness does not reference a completed restored smoke' }
        Assert-WideSiteFingerprint $ready.site_sha256 (Get-WideSiteFingerprint)
    }
    $current = Read-JsonFile $CurrentPath
    if ($current) {
        $previous = Read-JsonFile (Join-Path $current.evidence_root 'state.json')
        if ($previous -and (Test-ProcessAlive $previous.runner_pid) -and $previous.status -in @('starting','running','draining','restoring')) { throw 'wide test is already active' }
    }
    if (-not (Test-Path -LiteralPath $Helper)) { throw 'Build first: go build -o .cache/wide-seven-hour/labwide.exe ./scripts/labwide' }
    if (-not $RunId) { $RunId = ('wide{0}_{1}' -f $(if ($Smoke) {'smoke'} else {"${FormalDurationSeconds}s"}),(Get-Date -Format 'MMdd_HHmmss')) }
    if ($RunId -notmatch '^[A-Za-z0-9_]{1,35}$') { throw 'invalid RunId' }
    $EvidenceRoot = Join-Path $CacheRoot $RunId
    if (Test-Path -LiteralPath $EvidenceRoot) { throw 'run already exists' }
    New-Item -ItemType Directory -Path $EvidenceRoot | Out-Null
    Copy-Item -LiteralPath $Helper -Destination (Join-Path $EvidenceRoot 'labwide.exe')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'fixtures\longtest-wide50.schema.json') -Destination (Join-Path $EvidenceRoot 'schema.json')
    Write-JsonAtomic $CurrentPath @{ run_id=$RunId; evidence_root=$EvidenceRoot }
    Write-JsonAtomic (Join-Path $EvidenceRoot 'state.json') @{ run_id=$RunId; runner_pid=0; status='starting'; phase='launcher' }
    $hostExe = (Get-Process -Id $PID).Path
    $arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" -Action Run -RunId $RunId -EdgeHost $EdgeHost -EdgeUser $EdgeUser -SSHKeyPath `"$SSHKeyPath`""
    $arguments += " -VerifiedServerAgentPath `"$ServerAgentPath`" -VerifiedEdgeAgentPath `"$EdgeAgentPath`" -ExpectedAgentHash $ExpectedAgentHash -RuntimeMode $RuntimeMode"
    $arguments += " -FormalDurationSeconds $FormalDurationSeconds"
    if ($BestEffort) { $arguments += " -BestEffort -DiskBudgetGB $DiskBudgetGB" }
    if ($Smoke) { $arguments += " -Smoke -SmokeDurationSeconds $SmokeDurationSeconds" }
    $runner = Start-Process -FilePath $hostExe -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $EvidenceRoot 'runner.stdout.log') -RedirectStandardError (Join-Path $EvidenceRoot 'runner.stderr.log')
    Start-Sleep -Seconds 2
    if ($runner.HasExited) { throw 'runner exited; inspect runner.stderr.log' }
    @{ run_id=$RunId; runner_pid=$runner.Id; evidence_root=$EvidenceRoot; status='starting'; smoke=[bool]$Smoke } | ConvertTo-Json
    exit 0
}

$EvidenceRoot = Get-EvidenceRoot $RunId
$Helper = Join-Path $EvidenceRoot 'labwide.exe'
$StatePath = Join-Path $EvidenceRoot 'state.json'
$EventsPath = Join-Path $EvidenceRoot 'events.jsonl'
$SnapshotsPath = Join-Path $EvidenceRoot 'snapshots.jsonl'
$ConfigPath = Join-Path $EvidenceRoot 'workload.json'
$StopRequestPath = Join-Path $EvidenceRoot 'stop.requested'
$script:State = [ordered]@{ run_id=$RunId; runner_pid=$PID; status='starting'; phase='preflight'; smoke=[bool]$Smoke; evidence_root=$EvidenceRoot }
$script:OriginalServerAgentPath = ''
$script:Tunnel = $null
$script:Producer = $null
$script:SiteChanged = $false
$script:Manifest = $null
$script:Counter = 0
$script:Scenario = $null
$script:Metrics = $null
$script:Latency = $null
$script:AgentInstances=@{}
$script:LegacyStopAgent=${function:Stop-Agent}
$script:LegacyStartAgent=${function:Start-Agent}

function Get-AgentProcess {
    param([ValidateSet('Edge','Server')][string]$Side)
    if($Side -eq 'Edge'){
        $raw=Invoke-RemotePowerShell @'
$items=@(Get-CimInstance Win32_Process -Filter "Name='SyncAgent.exe'" | Where-Object {$_.CommandLine -like '*C:\ProgramData\NodeBridge\config.yaml*'} | Select-Object ProcessId,ExecutablePath,CommandLine,CreationDate)
if($items.Count -gt 1){throw 'multiple agents use the lab config'}
if($items.Count -eq 1){$items[0] | ConvertTo-Json -Compress}
'@
        $line=$raw | Where-Object {$_ -match '^\s*\{'} | Select-Object -Last 1
        if($line){return $line | ConvertFrom-Json};return $null
    }
    $items=@(Get-CimInstance Win32_Process -Filter "Name='SyncAgent.exe'" | Where-Object {$_.CommandLine -like "*$ServerConfigPath*"} | Select-Object ProcessId,ExecutablePath,CommandLine,CreationDate)
    if($items.Count -gt 1){throw 'multiple agents use the lab config'}
    if($items.Count -eq 1){return $items[0]};return $null
}
function Stop-Agent {
    param([ValidateSet('Edge','Server')][string]$Side)
    $actual=Get-AgentProcess $Side;$expected=$script:AgentInstances[$Side]
    if($actual -and $expected -and ($actual.ProcessId -ne $expected.ProcessId -or $actual.ExecutablePath -ne $expected.ExecutablePath -or "$($actual.CreationDate)" -ne "$($expected.CreationDate)")){throw "$Side process identity changed; refusing stop"}
    & $script:LegacyStopAgent $Side
}
function Start-Agent {
    param([ValidateSet('Edge','Server')][string]$Side,[switch]$TestMode)
    & $script:LegacyStartAgent $Side -TestMode:$TestMode
    $actual=Get-AgentProcess $Side
    $wanted=if($Side -eq 'Edge'){$EdgeAgentPath}elseif($TestMode){$ServerAgentPath}else{$script:OriginalServerAgentPath}
    if(-not $actual -or $actual.ExecutablePath -ne $wanted){throw "$Side started from unexpected path"}
    $script:AgentInstances[$Side]=$actual
}

function ConvertTo-WideArgument {
    param([string]$Value)
    return '"'+([regex]::Replace([regex]::Replace($Value,'(\\*)"','$1$1\"'),'(\\+)$','$1$1'))+'"'
}
function Select-WideInstalledRegistration {
    param([object[]]$Items)
    if(-not $Items -or $Items.Count -eq 0){throw 'NodeBridge installation registration missing'}
    $first=$Items[0]
    if(-not $first.install_root -or -not $first.version){throw 'Incomplete NodeBridge registration'}
    foreach($item in $Items){
        if(-not $item.install_root -or $item.version -ne $first.version -or [IO.Path]::GetFullPath($item.install_root) -ne [IO.Path]::GetFullPath($first.install_root)){throw 'Conflicting NodeBridge registry views'}
    }
    return $first
}
function Get-WideInstalledRegistration {
    param([ValidateSet('Edge','Server')][string]$Side)
    $read=@'
$items=@()
foreach($view in @([Microsoft.Win32.RegistryView]::Registry32,[Microsoft.Win32.RegistryView]::Registry64)){
    $base=[Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine,$view)
    try {
        $key=$base.OpenSubKey('SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge')
        if($key){try {$items+=@{install_root=$key.GetValue('InstallLocation');version=$key.GetValue('DisplayVersion');registry_view=$view.ToString()}}finally{$key.Dispose()}}
    } finally {$base.Dispose()}
}
ConvertTo-Json -InputObject @($items) -Compress
'@
    $raw=if($Side -eq 'Server'){& ([scriptblock]::Create($read))}else{Invoke-RemotePowerShell $read}
    $line=$raw | Where-Object {$_ -match '^\s*\['} | Select-Object -Last 1
    if(-not $line){throw "$Side installation registration unavailable"}
    return Select-WideInstalledRegistration @($line | ConvertFrom-Json)
}
function Assert-WideInstalledRuntime {
    param($Registrations,$Processes,$Candidates,[string]$ExpectedVersion,[string]$ExpectedHash,
        [ValidateSet('Installed','WorkspaceServer')][string]$Mode='Installed',[string]$WorkspaceRoot='')
    if($ExpectedHash -notmatch '^[A-Fa-f0-9]{64}$' -or -not $ExpectedVersion){throw 'Missing candidate identity'}
    $proof=@{passed=$true;version=$ExpectedVersion;agent_sha256=$ExpectedHash;mode=$Mode}
    foreach($side in @('Server','Edge')){
        $reg=$Registrations[$side];$process=$Processes[$side];$candidate=$Candidates[$side]
        if($side -eq 'Server' -and $Mode -eq 'WorkspaceServer'){
            if(-not $WorkspaceRoot -or -not $candidate.path -or -not [IO.Path]::IsPathRooted($candidate.path)){throw 'Workspace candidate path required'}
            $workspace=[IO.Path]::GetFullPath($WorkspaceRoot).TrimEnd('\')+'\'
            $path=[IO.Path]::GetFullPath($candidate.path)
            if(-not $path.StartsWith($workspace,[StringComparison]::OrdinalIgnoreCase) -or $candidate.hash -ne $ExpectedHash){throw 'Workspace candidate outside workspace or hash mismatch'}
            if(-not $process.ExecutablePath -or [IO.Path]::GetFullPath($process.ExecutablePath) -ne $path){throw 'Server must first run the verified workspace Agent'}
            $proof[$side]=@{path=$path;kind='workspace';original_pid=$process.ProcessId}
            continue
        }
        if(-not $reg.install_root -or $reg.version -ne $ExpectedVersion){throw "$side registered installation version mismatch"}
        $installed=[IO.Path]::GetFullPath((Join-Path $reg.install_root 'app\SyncAgent.exe'))
        if(-not $candidate.path -or [IO.Path]::GetFullPath($candidate.path) -ne $installed -or $candidate.hash -ne $ExpectedHash){throw "$side candidate is not the verified installed Agent"}
        if(-not $process -or -not $process.ExecutablePath -or [IO.Path]::GetFullPath($process.ExecutablePath) -ne $installed){throw "$side must first run from the normal installed Agent path"}
        $proof[$side]=@{path=$installed;kind='installed';registered_version=$reg.version;original_pid=$process.ProcessId}
    }
    return $proof
}
function Invoke-External {
    param([Parameter(Mandatory)][string]$FilePath,[Parameter(Mandatory)][string[]]$Arguments,[string]$Label='command')
    $info=[Diagnostics.ProcessStartInfo]::new()
    $info.FileName=$FilePath;$info.Arguments=($Arguments | ForEach-Object {ConvertTo-WideArgument $_}) -join ' '
    $info.UseShellExecute=$false;$info.CreateNoWindow=$true;$info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    $process=[Diagnostics.Process]::new();$process.StartInfo=$info
    try {
        $null=$process.Start();$out=$process.StandardOutput.ReadToEndAsync();$err=$process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(45000)) { $process.Kill();$process.WaitForExit();throw "$Label exceeded 45 seconds" }
        $stdout=$out.Result;$stderr=$err.Result
        if ($process.ExitCode -ne 0) { throw "$Label failed: $stdout $stderr" }
        return @($stdout -split '\r?\n' | Where-Object {$_ -ne ''})
    } finally { $process.Dispose() }
}
function Update-WideLatency {
    param([double]$Elapsed,[bool]$Allowed)
    if ($script:Latency) {
        $job=$script:Latency
        if ($job.process.HasExited) {
            if ($job.process.ExitCode -ne 0) { throw "latency probe failed: $(Get-Content $job.err -Raw)" }
            $script:Latency=$null
        } elseif (([DateTimeOffset]::UtcNow-$job.started).TotalSeconds -gt 125) { $job.process.Kill();throw 'latency probe exceeded 125 seconds' }
    }
    $interval=if($Smoke){5}else{30}
    if ($Allowed -and -not $script:Latency -and $Elapsed-$script:LastLatency -ge $interval) {
        $script:Counter++;$out=Join-Path $EvidenceRoot "latency-task-$($script:Counter).out";$err=Join-Path $EvidenceRoot "latency-task-$($script:Counter).err"
        $p=Start-Process -FilePath $Helper -ArgumentList "-config `"$ConfigPath`" -action latency" -WindowStyle Hidden -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
        $null=$p.Handle;$script:Latency=@{process=$p;err=$err;started=[DateTimeOffset]::UtcNow};$script:LastLatency=$Elapsed
    }
}
function Get-WideLatencySummary($Samples,[string]$Node) {
    $groups=@{all=@();steady=@();stress=@()}
    foreach($sample in $Samples) {
        $phase=$sample.phases.$Node
        if($phase -notin @('steady','peak','update_heavy')){throw "missing latency phase: $Node"}
        if($null -eq $sample.$Node -or [double]$sample.$Node -lt 0){throw "invalid latency sample: $Node"}
        $value=[double]$sample.$Node
        $groups.all+=$value
        if($phase -eq 'steady'){$groups.steady+=$value}else{$groups.stress+=$value}
    }
    $result=@{}
    foreach($group in @('all','steady','stress')) {
        $values=@($groups[$group]|Sort-Object)
        $minimum=if($group -eq 'all'){40}elseif($group -eq 'steady'){25}else{3}
        if($values.Count -lt $minimum){throw "not enough latency samples: $Node/$group count=$($values.Count) minimum=$minimum"}
        $result[$group]=@{count=$values.Count;p95_ms=$values[[Math]::Ceiling($values.Count*0.95)-1];p99_ms=$values[[Math]::Ceiling($values.Count*0.99)-1];max_ms=$values[-1]}
    }
    if($result.steady.p95_ms -gt 2000 -or $result.all.p99_ms -gt 5000){throw "online latency objective failed: $Node steady_p95=$($result.steady.p95_ms) all_p99=$($result.all.p99_ms)"}
    return $result
}
function Get-WideExpectedCounts {
    param([double]$Seconds,[int]$Duration)
    $ticks=[Math]::Max(0,[Math]::Min([Math]::Floor($Seconds)+1,[Math]::Floor($Duration*390/420)))
    $totalI=0.0;$totalU=0.0;$from=0
    foreach($stage in @(@(30,20,10),@(240,40,20),@(270,20,20),@(300,80,40),@(330,20,60),@(390,40,20))) {
        $end=[Math]::Min([Math]::Ceiling($stage[0]*$Duration/420),[Math]::Floor($Duration*390/420))
        $count=[Math]::Max(0,[Math]::Min($ticks,$end)-$from)
        $totalI+=$count*$stage[1];$totalU+=$count*$stage[2];$from=$end
    }
    return @{inserts=$totalI;updates=$totalU}
}
function Add-WideRateWindow {
    param([object]$Record)
    $script:RateRecords+=@($Record)
    $script:RateRecords=@($script:RateRecords | Where-Object {$Record.elapsed_seconds-$_.elapsed_seconds -le 300})
    $first=$script:RateRecords[0];$seconds=$Record.elapsed_seconds-$first.elapsed_seconds
    if ($seconds -lt 15 -or $Record.elapsed_seconds-$script:LastRateWindow -lt 30) { return }
    $expectedA=Get-WideExpectedCounts $first.elapsed_seconds $duration;$expectedB=Get-WideExpectedCounts $Record.elapsed_seconds $duration
    $window=@{at=$Record.at;window_seconds=$seconds;logical_minute=$Record.logical_minute}
    foreach($node in @('edge-001','server-001')) {
        $inserts=$Record.data.$node.committed_inserts-$first.data.$node.committed_inserts
        $updates=$Record.data.$node.committed_updates-$first.data.$node.committed_updates
        $window[$node]=@{insert_per_second=$inserts/$seconds;update_per_second=$updates/$seconds;inserts=$inserts;updates=$updates}
        if ($seconds -ge 270 -and ($inserts -lt ($expectedB.inserts-$expectedA.inserts)*0.9 -or $updates -lt ($expectedB.updates-$expectedA.updates)*0.9)) { throw "five-minute production rate below 90%: $node" }
    }
    Add-Content -LiteralPath (Join-Path $EvidenceRoot 'rate-windows.jsonl') -Value ($window | ConvertTo-Json -Depth 6 -Compress) -Encoding UTF8
    $script:LastRateWindow=$Record.elapsed_seconds
}

function Update-WideMetrics {
    param([double]$Elapsed)
    if ($script:Metrics) {
        $m=$script:Metrics
        if ($m.process.HasExited) {
            if ($m.process.ExitCode -ne 0) { throw "metrics failed: $(Get-Content $m.err -Raw)" }
            $data=Read-JsonFile $m.out
            Add-Content -LiteralPath (Join-Path $EvidenceRoot 'mysql-metrics.jsonl') -Value ($data | ConvertTo-Json -Depth 15 -Compress) -Encoding UTF8
            $script:Metrics=$null
        } elseif (([DateTimeOffset]::UtcNow-$m.started).TotalSeconds -gt 60) { $m.process.Kill();throw 'metrics exceeded 60 seconds' }
    }
    if (-not $script:Metrics -and ($Elapsed-$script:LastMetrics) -ge 60) {
        $script:Counter++;$out=Join-Path $EvidenceRoot "metrics-$($script:Counter).out";$err=Join-Path $EvidenceRoot "metrics-$($script:Counter).err"
        $p=Start-Process -FilePath $Helper -ArgumentList "-config `"$ConfigPath`" -action metrics" -WindowStyle Hidden -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
        $null=$p.Handle;$script:Metrics=@{process=$p;out=$out;err=$err;started=[DateTimeOffset]::UtcNow};$script:LastMetrics=$Elapsed
    }
}

function Start-WideScenario {
    param([string]$Command)
    $script:Counter++
    $out=Join-Path $EvidenceRoot "scenario-$($script:Counter)-$Command.out"
    $err=Join-Path $EvidenceRoot "scenario-$($script:Counter)-$Command.err"
    $p=Start-Process -FilePath $Helper -ArgumentList "-config `"$ConfigPath`" -action $Command" -WindowStyle Hidden -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
    $null=$p.Handle
    $script:Scenario=@{ command=$Command; process=$p; error_path=$err; started=[DateTimeOffset]::UtcNow }
    Add-Event 'scenario_started' 'ok' @{ command=$Command; pid=$p.Id }
}
function Complete-WideScenario {
    if (-not $script:Scenario) { return '' }
    $s=$script:Scenario
    if (-not $s.process.HasExited) {
        if (([DateTimeOffset]::UtcNow-$s.started).TotalSeconds -gt 180) { $s.process.Kill();throw "scenario timeout: $($s.command)" }
        return ''
    }
    if ($s.process.ExitCode -ne 0) { throw "scenario $($s.command) failed: $(Get-Content $s.error_path -Raw)" }
    $script:Scenario=$null
    Add-Event 'scenario_verified' 'ok' @{command=$s.command}
    return $s.command
}

function Invoke-Wide {
    param([string]$Command, [int]$TimeoutSeconds=180)
    $script:Counter++
    $out = Join-Path $EvidenceRoot ("command-{0}-{1}.out" -f $script:Counter,$Command)
    $err = Join-Path $EvidenceRoot ("command-{0}-{1}.err" -f $script:Counter,$Command)
    $process = Start-Process -FilePath $Helper -ArgumentList "-config `"$ConfigPath`" -action $Command" -WindowStyle Hidden -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
    $null = $process.Handle
    if (-not $process.WaitForExit($TimeoutSeconds*1000)) {
        $process.Kill()
        throw "wide command timeout: $Command"
    }
    if ($process.ExitCode -ne 0) { throw "wide $Command failed: $(Get-Content -LiteralPath $err -Raw)" }
    return Get-Content -LiteralPath $out -Encoding UTF8 -Raw
}
function Get-FreeSpace {
    $local = Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='D:'"
    $edge = Invoke-RemotePowerShell '(Get-CimInstance Win32_LogicalDisk -Filter "DeviceID=''C:''").FreeSpace'
    $edgeFree = [int64]($edge | Where-Object { $_ -match '^\d+$' } | Select-Object -Last 1)
    return @{ server_data_free=[int64]$local.FreeSpace; edge_data_free=$edgeFree }
}
function Get-WideResources {
    $filter="Name='SyncAgent.exe' OR Name='mysqld.exe' OR Name='java.exe' OR Name='erl.exe' OR Name='com.docker.backend.exe'"
    $fields=@('ProcessId','Name','WorkingSetSize','KernelModeTime','UserModeTime','ReadTransferCount','WriteTransferCount')
    $local=@(Get-CimInstance Win32_Process -Filter $filter | Select-Object $fields)
    $remote=Invoke-RemotePowerShell "@(Get-CimInstance Win32_Process -Filter `"$filter`" | Select-Object ProcessId,Name,WorkingSetSize,KernelModeTime,UserModeTime,ReadTransferCount,WriteTransferCount) | ConvertTo-Json -Compress"
    return @{server=$local;edge=(($remote -join "`n") | ConvertFrom-Json)}
}
function Get-WideRuntimeQueues {
    $output=Invoke-External $DockerPath @('exec',$ServerRabbitMQContainer,'rabbitmqctl','list_queues','-p',$ServerRabbitVHost,'name','messages_ready','messages_unacknowledged','consumers') 'server queues'
    $all=[ordered]@{}
    foreach($line in $output){if($line -match '^([^\s]+)\s+(\d+)\s+(\d+)\s+(\d+)$'){$all[$Matches[1]]=@{ready=[int64]$Matches[2];unacked=[int64]$Matches[3];consumers=[int64]$Matches[4]}}}
    $runtime=[ordered]@{}
    foreach ($name in @('server.cdc.ingress.q','edge-001.downlink.q','server.dead.q')) {
        if (-not $all.Contains($name)) { throw "runtime queue is missing: $name" }
        $runtime[$name]=$all[$name]
    }
    $edge=Invoke-RemotePowerShell "& 'C:\Program Files\RabbitMQ Server\rabbitmq_server-4.1.8\sbin\rabbitmqctl.bat' list_queues -p /nodebridge-edge name messages_ready messages_unacknowledged consumers; if (`$LASTEXITCODE -ne 0) {throw 'edge queue snapshot failed'}"
    $edgeQueues=@{}
    foreach($line in $edge){if($line -match '^([^\s]+)\s+(\d+)\s+(\d+)\s+(\d+)$'){$edgeQueues[$Matches[1]]=@{ready=[int64]$Matches[2];unacked=[int64]$Matches[3];consumers=[int64]$Matches[4]}}}
    foreach($name in @('edge.upload.cdc.q','edge.upload.retry.q','edge.downlink.q','edge.dead.q')){if(-not $edgeQueues.ContainsKey($name)){throw "missing edge queue $name"};$runtime['edge-local:'+$name]=$edgeQueues[$name]}
    return $runtime
}
function Add-WideRule {
    param([string]$ID,[string]$SourceDB,[string]$SourceTable,[string]$TargetDB,[string]$TargetTable,[string]$Direction,[string]$Mode,[hashtable]$Mappings,[string]$SourcePK='id',[string]$TargetPK='record_id')
    $origin = if ($SourceDB -eq 'scada_edge') { 'edge-001' } else { 'server-001' }
    $dispatch = if ($Direction -eq 'EDGE_TO_SERVER') { 'NONE' } else { 'SELECTED_EDGES' }
    $conflict = if ($Mode -eq 'append_only') { 'NONE' } else { 'LAST_WRITE_WIN' }
    $text = @"
    - id: $ID
      database_name: $SourceDB
      table_name: $SourceTable
      source_node_ids: [$origin]
      target_database_name: $TargetDB
      target_table_name: $TargetTable
      direction: $Direction
      dispatch_target: $dispatch
      dispatch_node_ids: [edge-001]
      sync_mode: $Mode
      conflict_policy: $conflict
      enable: true
      primary_keys: [$SourcePK]
      target_primary_keys: [$TargetPK]
      include_columns: []
      exclude_columns: []
      schema_sync: {add_columns: true, drop_columns: true}
      column_mappings:
"@
    foreach ($key in ($Mappings.Keys | Sort-Object)) { $text += "`n        - {source_column: $key, target_column: $($Mappings[$key])}" }
    return $text
}
function Restore-WideSite {
    $restoration = [ordered]@{ at=(Get-Date).ToString('o'); passed=$false; errors=@() }
    foreach ($job in @($script:Scenario,$script:Metrics,$script:Latency)) {
        try {
            if ($job) {
                if (-not $job.process) { throw 'background job has no process handle' }
                if (-not $job.process.HasExited) { $job.process.Kill();$job.process.WaitForExit() }
            }
        } catch { $restoration.errors += $_.Exception.Message }
    }
    try { if ($script:Producer -and -not $script:Producer.HasExited) {
        [IO.File]::WriteAllText($StopRequestPath,'restoration')
        if (-not $script:Producer.WaitForExit(45000)) { $script:Producer.Kill() }
    } } catch { $restoration.errors += $_.Exception.Message }
    if ($script:SiteChanged) {
        foreach ($side in @('Edge','Server')) {
            try {
                if (Get-AgentProcess $side) { Stop-Agent $side }
                $backup = Join-Path $EvidenceRoot ("{0}-rules.original.yaml" -f $side.ToLowerInvariant())
                if ($side -eq 'Server') { Copy-Item -LiteralPath $backup -Destination $ServerRulesPath -Force }
                else { Copy-ToEdge $backup $EdgeRulesPath }
                $original = $script:Manifest.original.$side
                if ($side -eq 'Edge' -and $original) { $script:EdgeAgentPath=$original.ExecutablePath }
                if ($original) { Start-Agent $side }
                if ($side -eq 'Edge') { Copy-FromEdge $EdgeRulesPath (Join-Path $EvidenceRoot 'edge-rules.restored.yaml'); $actual=Join-Path $EvidenceRoot 'edge-rules.restored.yaml' }
                else { $actual=$ServerRulesPath }
                if ((Get-FileHash $actual).Hash -ne (Get-FileHash $backup).Hash) { throw "$side rule restoration hash mismatch" }
                $configBackup=Join-Path $EvidenceRoot ($side.ToLowerInvariant()+'-config.original.yaml')
                $actualConfig=$ServerConfigPath
                if($side -eq 'Edge'){$actualConfig=Join-Path $EvidenceRoot 'edge-config.restored.yaml';Copy-FromEdge $EdgeConfigPath $actualConfig}
                if((Get-FileHash $actualConfig).Hash -ne (Get-FileHash $configBackup).Hash){throw "$side config changed during test"}
                $p=Get-AgentProcess $side
                if ($original -and ($null -eq $p -or $p.ExecutablePath -ne $original.ExecutablePath)) { throw "$side original process path not restored" }
                $restoration[$side]=@{ pid=if ($p){$p.ProcessId}else{0}; rules_sha256=(Get-FileHash $actual).Hash; path=if ($p){$p.ExecutablePath}else{''} }
            } catch { $restoration.errors += $_.Exception.Message }
        }
    }
    try { if ($script:Tunnel -and -not $script:Tunnel.HasExited) { $script:Tunnel.Kill(); $script:Tunnel.WaitForExit() } }
    catch { $restoration.errors += $_.Exception.Message }
    $restoration.passed = $restoration.errors.Count -eq 0
    Write-JsonAtomic (Join-Path $EvidenceRoot 'restoration.json') $restoration
    if (-not $restoration.passed) { throw ($restoration.errors -join '; ') }
}

$runError = $null
try {
    Set-State @{}
    if ((Get-FileHash $ServerAgentPath).Hash -ne $ExpectedAgentHash) { throw 'center binary differs from verified package' }
    $edgeHash = Invoke-RemotePowerShell "(Get-FileHash -LiteralPath '$EdgeAgentPath').Hash"
    if ($ExpectedAgentHash -notin $edgeHash) { throw 'edge binary differs from verified package' }
    $space=Get-FreeSpace
    if ($space.server_data_free -lt 40GB -or $space.edge_data_free -lt 40GB) { throw "less than 40GiB on a data disk: $($space | ConvertTo-Json -Compress)" }
    if (-not $Smoke -and -not $BestEffort) {
        $approved=Read-JsonFile (Join-Path $CacheRoot 'formal-ready.json')
        Assert-WideReadinessDuration $approved $FormalDurationSeconds
        Assert-WideReadinessRuntime $approved $RuntimeMode $ServerAgentPath $EdgeAgentPath
        if (-not $approved -or -not $approved.passed -or $approved.smoke_duration_seconds -lt 1800 -or $approved.agent_sha256 -ne $ExpectedAgentHash -or $approved.helper_sha256 -ne (Get-FileHash $Helper).Hash -or $approved.script_sha256 -ne (Get-FileHash $PSCommandPath).Hash) { throw 'Formal Run readiness mismatch' }
        foreach($disk in @('server_data_free','edge_data_free')) {
            $rate=$approved.disk_growth_bytes_per_second.$disk
            if ($null -eq $rate -or $rate -le 0) { throw "missing measured disk growth: $disk" }
            $required=[Math]::Max(40GB,$rate*$FormalDurationSeconds*1.5+10GB)
            if ($space[$disk] -lt $required) { throw "formal disk forecast exceeds free space: $disk required=$required available=$($space[$disk])" }
        }
    }
    $queues=Get-WideRuntimeQueues
    foreach ($name in $queues.Keys) { if ($queues[$name].ready+$queues[$name].unacked -gt 0) { throw "pre-existing queue backlog: $name" } }
    $original=@{ Server=(Get-AgentProcess Server); Edge=(Get-AgentProcess Edge) }
    if ($null -eq $original.Server -or $null -eq $original.Edge) { throw 'both original agents must be running' }
    $package=Read-JsonFile (Join-Path $CacheRoot 'package-gates.json')
    if(-not $package.passed -or $package.agent_sha256 -ne $ExpectedAgentHash){throw 'Verified package evidence is required before installed-runtime testing'}
    $registrations=@{Edge=(Get-WideInstalledRegistration Edge)}
    if($RuntimeMode -eq 'Installed'){$registrations.Server=Get-WideInstalledRegistration Server}
    $candidates=@{Server=@{path=$ServerAgentPath;hash=(Get-FileHash $ServerAgentPath).Hash};Edge=@{path=$EdgeAgentPath;hash=($edgeHash | Where-Object {$_ -match '^[A-Fa-f0-9]{64}$'} | Select-Object -First 1)}}
    $installation=Assert-WideInstalledRuntime $registrations $original $candidates $package.version $ExpectedAgentHash -Mode $RuntimeMode -WorkspaceRoot $Root
    $script:OriginalServerAgentPath=$original.Server.ExecutablePath
    $script:AgentInstances.Server=$original.Server;$script:AgentInstances.Edge=$original.Edge
    Copy-Item -LiteralPath $ServerRulesPath -Destination (Join-Path $EvidenceRoot 'server-rules.original.yaml')
    Copy-FromEdge $EdgeRulesPath (Join-Path $EvidenceRoot 'edge-rules.original.yaml')
    Copy-Item -LiteralPath $ServerConfigPath -Destination (Join-Path $EvidenceRoot 'server-config.original.yaml')
    Copy-FromEdge $EdgeConfigPath (Join-Path $EvidenceRoot 'edge-config.original.yaml')
    if(-not $Smoke -and -not $BestEffort){Assert-WideSiteFingerprint $approved.site_sha256 (Get-WideSiteFingerprint)}
    if ((Get-FileHash (Join-Path $EvidenceRoot 'server-rules.original.yaml')).Hash -ne (Get-FileHash (Join-Path $EvidenceRoot 'edge-rules.original.yaml')).Hash) { throw 'original rules differ' }
    $prefix='nb7_'+$RunId
    $duration=if ($Smoke){$SmokeDurationSeconds}else{$FormalDurationSeconds}
    $script:Manifest=@{ run_id=$RunId; original=$original; package_agent_sha256=$ExpectedAgentHash; duration_seconds=$duration; preflight_space=$space; workload_sha256=(Get-FileHash $Helper).Hash; script_sha256=(Get-FileHash $PSCommandPath).Hash; schema_sha256=(Get-FileHash (Join-Path $EvidenceRoot 'schema.json')).Hash; created_at=(Get-Date).ToString('o'); smoke=[bool]$Smoke }
    $script:Manifest.runtime_verification=$installation
    $script:Manifest.best_effort=[bool]$BestEffort
    $script:Manifest.disk_budget_bytes=[long]$DiskBudgetGB*1000000000
    $script:Manifest.capacity_forecast_required=-not $BestEffort
    if($RuntimeMode -eq 'Installed'){$script:Manifest.installed_runtime=$installation}
    Write-JsonAtomic (Join-Path $EvidenceRoot 'manifest.json') $script:Manifest
    $listener=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
    $listener.Start();$port=$listener.LocalEndpoint.Port;$listener.Stop()
    $script:Tunnel=Start-Process -FilePath 'ssh.exe' -ArgumentList "-N -T -i `"$SSHKeyPath`" -o BatchMode=yes -o ExitOnForwardFailure=yes -o ServerAliveInterval=15 -o ServerAliveCountMax=3 -L 127.0.0.1:${port}:127.0.0.1:3306 $EdgeUser@$EdgeHost" -WindowStyle Hidden -PassThru -RedirectStandardError (Join-Path $EvidenceRoot 'tunnel.stderr.log')
    Start-Sleep -Seconds 2
    if ($script:Tunnel.HasExited) { throw 'SSH tunnel failed' }
    $config=@{ run_id=$RunId; prefix=$prefix; created=[DateTime]::UtcNow.ToString('yyyy-MM-dd HH:mm:ss.fff'); start=''; duration_seconds=$duration; schema_path=(Join-Path $EvidenceRoot 'schema.json'); edge_dsn="root:root@tcp(127.0.0.1:$port)/scada_edge?charset=utf8mb4&interpolateParams=true&timeout=8s&readTimeout=30s&writeTimeout=30s&time_zone=%27%2B00%3A00%27"; server_dsn='root:root@tcp(127.0.0.1:3306)/scada_center?charset=utf8mb4&interpolateParams=true&timeout=8s&readTimeout=30s&writeTimeout=30s&time_zone=%27%2B00%3A00%27' }
    Write-JsonAtomic $ConfigPath $config
    Invoke-Wide init | Out-Null
    $rules=Get-Content -LiteralPath $ServerRulesPath -Encoding UTF8 -Raw
    $rules += "`n"+(Add-WideRule "$RunId-e-stream" scada_edge "${prefix}_e_stream" scada_center "${prefix}_e_archive" EDGE_TO_SERVER append_only @{id='record_id';seq_no='source_seq'})
    $rules += "`n"+(Add-WideRule "$RunId-s-stream" scada_center "${prefix}_s_stream" scada_edge "${prefix}_s_inbox" SERVER_TO_EDGE append_only @{id='record_id';n01='value01'})
    $rules += "`n"+(Add-WideRule "$RunId-e-state" scada_edge "${prefix}_e_state" scada_center "${prefix}_s_state" BIDIRECTIONAL crud_ordered @{id='setting_id';t01='label01';n01='value01'} -TargetPK setting_id)
    $rules += "`n"+(Add-WideRule "$RunId-s-state" scada_center "${prefix}_s_state" scada_edge "${prefix}_e_state" SERVER_TO_EDGE crud_ordered @{setting_id='id';label01='t01';value01='n01'} -SourcePK setting_id -TargetPK id)
    $testRules=Join-Path $EvidenceRoot 'sync-rules.test.yaml'
    [IO.File]::WriteAllText($testRules,$rules,[Text.UTF8Encoding]::new($false))
    $script:SiteChanged=$true
    Stop-Agent Edge
    Stop-Agent Server
    & $ServerAgentPath run -config $ServerConfigPath -rules $testRules -edges edge-001 -max-steps 1 *> (Join-Path $EvidenceRoot 'rules-validation.log')
    if ($LASTEXITCODE -ne 0) { throw 'generated rules did not validate' }
    Copy-Item -LiteralPath $testRules -Destination $ServerRulesPath -Force
    Copy-ToEdge $testRules $EdgeRulesPath
    Start-Agent Server -TestMode
    Start-Agent Edge -TestMode
    Set-State @{ phase='bootstrap'; tunnel_pid=$script:Tunnel.Id }
    Invoke-Wide bootstrap | Out-Null
    $bootstrapDeadline=(Get-Date).AddMinutes(3)
    do {
        if (Test-Path -LiteralPath $StopRequestPath) { throw 'controlled stop during bootstrap' }
        Start-Sleep -Seconds 3
        Set-State @{}
        try { Invoke-Wide verify | Out-Null; $bootstrapOK=$true } catch { $bootstrapOK=$false; $bootstrapError=$_.Exception.Message }
    } while (-not $bootstrapOK -and (Get-Date) -lt $bootstrapDeadline)
    if (-not $bootstrapOK) { throw "bootstrap did not converge: $bootstrapError" }
    Add-Event 'bootstrap_50_columns_verified'
    if (Test-Path -LiteralPath $StopRequestPath) { throw 'controlled stop requested before production' }
    $env:NODEBRIDGE_WIDE_AGENT=$ServerAgentPath
    $env:NODEBRIDGE_WIDE_AGENT_CONFIG=$ServerConfigPath
    $env:NODEBRIDGE_WIDE_AMQP='amqp://nb-server-sync:1234@127.0.0.1:5672/%2Fnodebridge-server'
    Invoke-Wide probe | Out-Null
    Add-Event 'isolated_same_batch_and_old_update_replay_verified'
    Invoke-Wide handoff | Out-Null
    Add-Event 'barrier_controlled_same_key_ownership_verified'
    $start=[DateTimeOffset]::UtcNow.AddSeconds(3)
    $config.start=$start.ToString('o')
    Write-JsonAtomic $ConfigPath $config
    $script:Producer=Start-Process -FilePath $Helper -ArgumentList "-config `"$ConfigPath`" -action run" -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $EvidenceRoot 'producer.stdout.log') -RedirectStandardError (Join-Path $EvidenceRoot 'producer.stderr.log')
    $null = $script:Producer.Handle
    $plannedEnd=$start.AddSeconds($duration)
    Set-State @{ status='running'; phase='baseline'; started_at=$start.ToString('o'); planned_end_at=$plannedEnd.ToString('o'); producer_pid=$script:Producer.Id; duration_seconds=$duration }
    $flags=@{};$lastLife=0.0;$lastSnapshot=-100.0;$edgeOff=$false;$serverOff=$false
    $lifeCount=0;$recoveries=@{};$baselineSpace=$space;$spaceHistory=@();$lastLatency=-100.0;$lastResources=-100.0;$resources=@{}
    $script:LastMetrics=-100.0;$script:LastLatency=-100.0;$outageSeconds=900*$duration/25200
    $script:RateRecords=@();$script:LastRateWindow=-100.0
    while ([DateTimeOffset]::UtcNow -lt $plannedEnd) {
        $elapsed=([DateTimeOffset]::UtcNow-$start).TotalSeconds
        $minute=$elapsed*420/$duration
        if (Test-Path -LiteralPath $StopRequestPath) { throw 'controlled stop requested' }
        if ($script:Tunnel.HasExited) { throw 'SSH tunnel exited' }
        foreach ($i in 0..1) {
            $p=Read-JsonFile (Join-Path $EvidenceRoot "producer-$i.json")
            if ($p -and $p.error) { throw "producer $i failed: $($p.error)" }
            if ($p -and -not $p.done -and ([DateTimeOffset]::UtcNow-[DateTimeOffset]$p.at).TotalSeconds -gt 45) { throw "producer $i heartbeat stale" }
        }
        if ($script:Producer.HasExited -and $minute -lt 389) { throw 'producer exited early' }
        if ($minute -ge 390) { Set-State @{status='draining';phase='drain'};break }
        Update-WideMetrics $elapsed
        $completed=Complete-WideScenario
        if ($completed -eq 'lifecycle') { $lifeCount++ }
        if ($completed -eq 'ddl-add') { $flags.ddlAdd=$true }
        if ($completed -eq 'ddl-drop') { $flags.ddlDrop=$true }
        if ($completed -eq 'late-probe') { $flags.lateProbe=$true }
        if ($completed -eq 'late-handoff') { $flags.lateHandoff=$true }
        if ($minute -ge 120 -and -not $flags.edgeStopped) { Stop-Agent Edge;$edgeOff=$true;$flags.edgeStopped=$true;$edgeOffAt=[DateTimeOffset]::UtcNow;Add-Event 'edge_outage_started';Set-State @{phase='edge_outage'} }
        if ($minute -ge 135 -and -not $flags.edgeStarted -and [DateTimeOffset]::UtcNow -ge $edgeOffAt.AddSeconds($outageSeconds)) { Start-Agent Edge -TestMode;$edgeOff=$false;$flags.edgeStarted=$true;$recoveries.Edge=@{deadline=$elapsed+[Math]::Max(30,1800*$duration/25200);edge=(Read-JsonFile (Join-Path $EvidenceRoot 'producer-0.json')).inserts;server=(Read-JsonFile (Join-Path $EvidenceRoot 'producer-1.json')).inserts};Add-Event 'edge_outage_ended' 'ok' @{duration_seconds=([DateTimeOffset]::UtcNow-$edgeOffAt).TotalSeconds};Set-State @{phase='edge_recovery'} }
        if ($minute -ge 180 -and -not $flags.serverStopped) { Stop-Agent Server;$serverOff=$true;$flags.serverStopped=$true;$serverOffAt=[DateTimeOffset]::UtcNow;Add-Event 'server_outage_started';Set-State @{phase='server_outage'} }
        if ($minute -ge 195 -and -not $flags.serverStarted -and [DateTimeOffset]::UtcNow -ge $serverOffAt.AddSeconds($outageSeconds)) { Start-Agent Server -TestMode;$serverOff=$false;$flags.serverStarted=$true;$recoveries.Server=@{deadline=$elapsed+[Math]::Max(30,1800*$duration/25200);edge=(Read-JsonFile (Join-Path $EvidenceRoot 'producer-0.json')).inserts;server=(Read-JsonFile (Join-Path $EvidenceRoot 'producer-1.json')).inserts};Add-Event 'server_outage_ended' 'ok' @{duration_seconds=([DateTimeOffset]::UtcNow-$serverOffAt).TotalSeconds};Set-State @{phase='server_recovery'} }
        if ($minute -ge 240 -and -not $flags.ddlAddStarted -and -not $script:Scenario) { Start-WideScenario ddl-add;$flags.ddlAddStarted=$true;Set-State @{phase='ddl'} }
        if ($minute -ge 255 -and $flags.ddlAdd -and -not $flags.ddlDropStarted -and -not $script:Scenario) { Start-WideScenario ddl-drop;$flags.ddlDropStarted=$true }
        if ($minute -ge 300 -and -not $flags.lateProbeStarted -and -not $script:Scenario) { Start-WideScenario late-probe;$flags.lateProbeStarted=$true }
        if ($minute -ge 305 -and $flags.lateProbe -and -not $flags.lateHandoffStarted -and -not $script:Scenario) { Start-WideScenario late-handoff;$flags.lateHandoffStarted=$true }
        $lifeInterval=if ($Smoke){90}else{300}
        $nearFault=($minute -ge 110 -and $minute -lt 140) -or ($minute -ge 170 -and $minute -lt 200) -or ($minute -ge 235 -and $minute -lt 270)
        if ($elapsed-$lastLife -ge $lifeInterval -and $minute -lt 375 -and -not $edgeOff -and -not $serverOff -and -not $script:Scenario -and -not $nearFault -and $recoveries.Count -eq 0) {
            Start-WideScenario lifecycle;$lastLife=$elapsed
        }
        Update-WideLatency $elapsed ($minute -lt 375 -and -not $edgeOff -and -not $serverOff -and -not $nearFault -and $recoveries.Count -eq 0)
        if ($elapsed-$lastSnapshot -ge 15) {
            $snapshot=Invoke-Wide snapshot -TimeoutSeconds 15 | ConvertFrom-Json
            $queue=Get-WideRuntimeQueues
            $space=Get-FreeSpace
            $spaceHistory+=@{at=$elapsed;server_data_free=$space.server_data_free;edge_data_free=$space.edge_data_free}
            if ($elapsed -ge 60) {
                foreach ($disk in @('server_data_free','edge_data_free')) {
                    if($BestEffort -and $baselineSpace[$disk]-$space[$disk] -ge [long]$DiskBudgetGB*1000000000){throw "best-effort disk growth budget reached: $disk"}
                    $growth=[Math]::Max(0,($baselineSpace[$disk]-$space[$disk])/$elapsed)
                    $required=[Math]::Max(40GB,$growth*[Math]::Max(0,$duration-$elapsed)*1.5+10GB)
                    if (-not $BestEffort -and $space[$disk] -lt $required) { throw "projected disk budget insufficient: $disk required=$required free=$($space[$disk])" }
                }
            }
            if ($space.server_data_free -lt 10GB -or $space.edge_data_free -lt 10GB) { throw 'data disk below 10GiB safety floor' }
            foreach ($node in @('edge-001','server-001')) { if ($snapshot.$node.failed_events -gt 0) { throw "unexpected failed events: $node" } }
            foreach ($node in @('edge-001','server-001')) { if ($snapshot.$node.failed_acks -gt 0) { throw "unexpected failed ACKs: $node" } }
            foreach ($name in $queue.Keys) { if ($name -match 'dead|retry' -and $queue[$name].ready+$queue[$name].unacked -gt 0) { throw "unexpected retry/dead messages: $name" } }
            foreach ($key in @($recoveries.Keys)) {
                $r=$recoveries[$key]
                # Recovery stages produce 40 INSERT/s; require live lag within two seconds twice.
                $progress=Get-WideRecoveryProgress $snapshot $r
                $r.quiet_samples=if($progress.ready){[int]$r.quiet_samples+1}else{0}
                if ($elapsed -gt $r.deadline) { throw "$key backlog recovery deadline exceeded; live lags=$($progress.lags|ConvertTo-Json -Compress)" }
                if ($r.quiet_samples -ge 2) { Add-Event 'outage_backlog_recovered' 'ok' @{side=$key;live_lags=$progress.lags;quiet_samples=$r.quiet_samples};$recoveries.Remove($key) }
            }
            foreach($side in @('Edge','Server')){
                if(($side -eq 'Edge' -and $edgeOff) -or ($side -eq 'Server' -and $serverOff)){continue}
                $p=Get-AgentProcess $side;$expected=$script:AgentInstances[$side]
                if(-not $p -or $p.ProcessId -ne $expected.ProcessId -or $p.ExecutablePath -ne $expected.ExecutablePath){throw "$side agent exited or was replaced unexpectedly"}
            }
            if ($elapsed-$lastResources -ge 60) { $resources=Get-WideResources;$lastResources=$elapsed }
            $record=@{ at=(Get-Date).ToString('o'); elapsed_seconds=$elapsed; logical_minute=$minute; data=$snapshot; queues=$queue; disk=$space; resources=$resources }
            Add-Content -LiteralPath $SnapshotsPath -Value ($record | ConvertTo-Json -Depth 12 -Compress) -Encoding UTF8
            Add-WideRateWindow $record
            Set-State @{ last_snapshot=$record }
            $lastSnapshot=$elapsed
        }
        Set-State @{}
        Start-Sleep -Seconds 1
    }
    if (-not $script:Producer.WaitForExit(45000)) { throw 'producer did not finish on schedule' }
    if ($script:Producer.ExitCode -ne 0) { throw 'producer returned failure' }
    while($script:Latency){Update-WideLatency 0 $false;Set-State @{};Start-Sleep -Seconds 1}
    while ($script:Scenario) { $completed=Complete-WideScenario;if ($completed -eq 'lifecycle'){$lifeCount++};Set-State @{};Start-Sleep -Seconds 1 }
    $drainDeadline=$start.AddSeconds($duration*410/420)
    $quiet=0
    do {
        $queue=Get-WideRuntimeQueues
        $pending=0;foreach($name in $queue.Keys){$pending+=$queue[$name].ready+$queue[$name].unacked}
        if ($pending -eq 0) {$quiet++}else{$quiet=0}
        Set-State @{quiet_samples=$quiet};if($quiet -lt 3){Start-Sleep -Seconds 1}
    } while ($quiet -lt 3 -and [DateTimeOffset]::UtcNow -lt $drainDeadline)
    if ($quiet -lt 3) { throw 'queue drain did not stay quiet before deadline' }
    do {
        Set-State @{}
        try { Invoke-Wide verify -TimeoutSeconds $(if($Smoke){180}else{600}) | Out-Null; $verified=$true } catch { $verified=$false;$verifyError=$_.Exception.Message;Add-Event 'final_consistency_retry' 'failed' @{error=$verifyError} }
        if (-not $verified) { Start-Sleep -Seconds 5 }
    } while (Test-WideVerificationRetry $verified $drainDeadline)
    if (-not $verified) { throw "final consistency failed: $verifyError" }
    foreach ($flag in @('edgeStopped','edgeStarted','serverStopped','serverStarted','ddlAdd','ddlDrop','lateProbe','lateHandoff')) { if (-not $flags[$flag]) { throw "missing scenario: $flag" } }
    if ($lifeCount -lt 2 -or $recoveries.Count -gt 0) { throw 'lifecycle or recovery gates incomplete' }
    foreach ($i in 0..1) {
        $p=Read-JsonFile (Join-Path $EvidenceRoot "producer-$i.json")
        if (-not $p.done -or $p.error -or $p.tick -ne [Math]::Floor($duration*390/420)) { throw "producer $i incomplete" }
        foreach ($column in 12..49) { if ($p.update_coverage[$column] -le 0) { throw "business column not updated: $column" } }
    }
    Add-Event 'full_50_column_consistency_verified'
    $observations=Read-JsonFile (Join-Path $EvidenceRoot 'version-observations.json')
    if (-not $observations.passed -or $observations.observations -lt 100) { throw 'online version observations missing or failed' }
    foreach($i in 0..1){$p=Read-JsonFile (Join-Path $EvidenceRoot "producer-$i.json");foreach($column in @(12,24,36,49)){if($p.field_coverage.to_null[$column] -le 0 -or $p.field_coverage.from_null[$column] -le 0){throw "NULL roundtrip coverage missing side=$i column=$column"}}}
    $latencySummary=@{}
    $samples=@(Get-ChildItem -LiteralPath $EvidenceRoot -Filter 'latency-*.json' | ForEach-Object { Read-JsonFile $_.FullName })
    foreach($node in @('edge-001','server-001')) {
        $latencySummary[$node]=Get-WideLatencySummary $samples $node
    }
    Write-JsonAtomic (Join-Path $EvidenceRoot 'gates.json') @{passed=$true;columns=50;life_cycles=$lifeCount;scenario_flags=$flags;latency=$latencySummary;queue_quiet_samples=$quiet;exact_producer_ledger=$true;all_business_columns_updated=$true;replay_metadata_proven=$true;ownership_handoff=$true;json_numeric_boundary=$true;disk_history=$spaceHistory}
    if (-not $Smoke -and [DateTimeOffset]::UtcNow -gt $plannedEnd) { throw 'verification exceeded seven-hour deadline' }
} catch {
    $runError=$_.Exception.Message
    Add-Event 'run_failed' 'failed' @{ error=$runError;error_id=$_.FullyQualifiedErrorId;exception_type=$_.Exception.GetType().FullName;hresult=$_.Exception.GetBaseException().HResult;position=$_.InvocationInfo.PositionMessage;script_stack=$_.ScriptStackTrace }
} finally {
    Set-State @{status='restoring';phase='restoration';error=$runError}
    try { Restore-WideSite } catch { $runError="$runError; restoration: $($_.Exception.Message)" }
    if (-not $runError -and [DateTimeOffset]::UtcNow -gt $plannedEnd) { $runError='restoration exceeded test deadline' }
    while (-not $runError -and [DateTimeOffset]::UtcNow -lt $plannedEnd) {
        Set-State @{status='running';phase='restored_observation'}
        if (Test-Path -LiteralPath $StopRequestPath) { $runError='controlled stop during final observation';break }
        Start-Sleep -Seconds 2
    }
    $status=if ($runError){'failed'}else{'completed'}
    Set-State @{status=$status;phase='finished';finished_at=(Get-Date).ToString('o');error=$runError}
    $report="# Wide-table test result`r`n`r`nRun: $RunId`r`nStatus: $status`r`nSmoke: $Smoke`r`nFinished: $((Get-Date).ToString('o'))`r`nError: $runError`r`n`r`nSee manifest.json, state.json, events.jsonl, snapshots.jsonl, consistency.json and restoration.json. A smoke result is not a seven-hour pass.`r`n"
    [IO.File]::WriteAllText((Join-Path $EvidenceRoot 'final-report.md'),$report,[Text.UTF8Encoding]::new($false))
}
if ($runError) { throw $runError }
