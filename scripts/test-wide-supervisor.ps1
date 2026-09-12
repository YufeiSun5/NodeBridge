$ErrorActionPreference='Stop'
$tokens=$null;$errors=$null
$path=Join-Path $PSScriptRoot 'lab-real-wide-seven-hour.ps1'
$ast=[Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors)
if($errors.Count){throw ($errors | Out-String)}
foreach($name in @('Get-WideExpectedCounts','ConvertTo-WideArgument','Read-JsonFile','Restore-WideSite','Get-WideLatencySummary','Assert-WideInstalledRuntime','Select-WideInstalledRegistration','Assert-WideSiteFingerprint','Get-WideRecoveryProgress','Test-WideVerificationRetry')){
    $f=@($ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)|Where-Object Name -eq $name)
    if($f.Count -ne 1){throw "ambiguous function $name"}
    . ([scriptblock]::Create($f[0].Extent.Text))
}
$f=$ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)|Where-Object Name -eq 'Assert-WideReadinessRuntime'
. ([scriptblock]::Create($f.Extent.Text))
$durationFunction=$ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)|Where-Object Name -eq 'Assert-WideReadinessDuration'
. ([scriptblock]::Create($durationFunction.Extent.Text))
Assert-WideReadinessDuration @{formal_duration_seconds=36000} 36000
foreach($proofDuration in @(0,25200,86400)) {
    $rejected=$false
    try { Assert-WideReadinessDuration @{formal_duration_seconds=$proofDuration} 36000 } catch { $rejected=$true }
    if(-not $rejected){throw 'mismatched long-test duration accepted'}
}
if($ast.Extent.Text -notmatch '-FormalDurationSeconds \$FormalDurationSeconds'){throw 'launcher omits formal duration'}
if($ast.Extent.Text -notmatch '-BestEffort -DiskBudgetGB \$DiskBudgetGB'){throw 'launcher omits best-effort budget'}
if($ast.Extent.Text -notmatch 'best-effort disk growth budget reached'){throw 'best-effort storage stop missing'}
if($ast.Extent.Text -notmatch 'data disk below 10GiB safety floor'){throw 'physical disk floor missing'}
$formal=Get-WideExpectedCounts 25200 25200
$tenHour=Get-WideExpectedCounts 36000 36000
if($tenHour.inserts -ne 1285720 -or $tenHour.updates -ne 797120){throw 'ten-hour totals changed'}
$deadline=[DateTimeOffset]::Parse('2026-09-09T16:14:58Z')
$beforeDeadline=[DateTimeOffset]::Parse('2026-09-10T00:14:02+08:00')
if(-not (Test-WideVerificationRetry $false $deadline $beforeDeadline)){throw 'cross-midnight retry was rejected'}
if(Test-WideVerificationRetry $true $deadline $beforeDeadline){throw 'successful verification retried'}
if(Test-WideVerificationRetry $false $deadline $deadline){throw 'deadline boundary retried'}
if(Test-WideVerificationRetry $false $deadline $deadline.AddSeconds(1)){throw 'expired verification retried'}
if(-not (Test-WideVerificationRetry $false ([DateTimeOffset]::UtcNow.AddMinutes(1)))){throw 'default UTC clock retry failed'}
if($formal.inserts -ne 900000 -or $formal.updates -ne 558000){throw 'formal totals changed'}
$smoke=Get-WideExpectedCounts 600 600
if($smoke.inserts -ne 21420 -or $smoke.updates -ne 13290){throw 'smoke totals changed'}
$extended=Get-WideExpectedCounts 1800 1800
if($extended.inserts -ne 64220 -or $extended.updates -ne 39850){throw 'extended smoke totals changed'}
$hash='A'*64
$site=@{server_config=$hash;edge_config=$hash;server_rules=$hash;edge_rules=$hash}
Assert-WideSiteFingerprint $site $site
foreach($key in $site.Keys){$changed=$site.Clone();$changed[$key]='B'*64;$rejected=$false;try{Assert-WideSiteFingerprint $site $changed}catch{$rejected=$true};if(-not $rejected){throw "configuration drift accepted: $key"}}
$rejected=$false;try{Assert-WideSiteFingerprint @{} $site}catch{$rejected=$true};if(-not $rejected){throw 'missing configuration proof accepted'}
$snap=@{'edge-001'=@{source=@{count=2000};incoming=@{count=1920}};'server-001'=@{source=@{count=2000};incoming=@{count=1920}}}
$recovery=@{edge=1000;server=1000}
if(-not (Get-WideRecoveryProgress $snap $recovery).ready){throw 'two-second live lag boundary rejected'}
$snap.'server-001'.incoming.count=1919
if((Get-WideRecoveryProgress $snap $recovery).ready){throw 'stale restart watermark falsely marked recovered'}
$snap.'server-001'.incoming.count=2100
if(-not (Get-WideRecoveryProgress $snap $recovery).ready){throw 'sampling-time target advance rejected'}
$recovery.edge=2200
if((Get-WideRecoveryProgress $snap $recovery).ready){throw 'unreached restart watermark accepted'}
$snap.'edge-001'.source.count=$null
$rejected=$false;try{Get-WideRecoveryProgress $snap $recovery|Out-Null}catch{$rejected=$true};if(-not $rejected){throw 'missing recovery counts accepted'}
$registration32=@{install_root='C:\Program Files\NodeBridge';version='0.46.9';registry_view='Registry32'}
$registration64=@{install_root='C:\Program Files\NodeBridge';version='0.46.9';registry_view='Registry64'}
foreach($items in @(@($registration32),@($registration64),@($registration32,$registration64))){
    if((Select-WideInstalledRegistration $items).version -ne '0.46.9'){throw 'valid registry view rejected'}
}
foreach($items in @(@(),@($registration32,@{install_root='D:\Other';version='0.46.9'}),@($registration32,@{install_root='C:\Program Files\NodeBridge';version='0.46.8'}))){
    $rejected=$false
    try{Select-WideInstalledRegistration $items|Out-Null}catch{$rejected=$true}
    if(-not $rejected){throw 'missing/conflicting registry views accepted'}
}
$registrations=@{Server=@{install_root='C:\Program Files\NodeBridge';version='0.46.9'};Edge=@{install_root='D:\Apps\NodeBridge';version='0.46.9'}}
$processes=@{Server=@{ExecutablePath='C:\Program Files\NodeBridge\app\SyncAgent.exe';ProcessId=1};Edge=@{ExecutablePath='D:\Apps\NodeBridge\app\SyncAgent.exe';ProcessId=2}}
$candidates=@{Server=@{path=$processes.Server.ExecutablePath;hash=$hash};Edge=@{path=$processes.Edge.ExecutablePath;hash=$hash}}
$installed=Assert-WideInstalledRuntime $registrations $processes $candidates '0.46.9' $hash
if(-not $installed.passed){throw 'matching installed runtimes were rejected'}
foreach($kind in @('temporary','old_version','wrong_hash','wrong_process','missing_registration')){
    $r=$registrations | ConvertTo-Json -Depth 5 | ConvertFrom-Json
    $p=$processes | ConvertTo-Json -Depth 5 | ConvertFrom-Json
    $c=$candidates | ConvertTo-Json -Depth 5 | ConvertFrom-Json
    switch($kind){
        temporary {$c.Edge.path='C:\ProgramData\NodeBridge\lab-v0469\SyncAgent.exe'}
        old_version {$r.Edge.version='0.46.8'}
        wrong_hash {$c.Server.hash='B'*64}
        wrong_process {$p.Server.ExecutablePath='D:\DEV_D\NodeBridge\.cache\original\SyncAgent.exe'}
        missing_registration {$r.Edge.install_root=''}
    }
    $rejected=$false
    try{Assert-WideInstalledRuntime @{Server=$r.Server;Edge=$r.Edge} @{Server=$p.Server;Edge=$p.Edge} @{Server=$c.Server;Edge=$c.Edge} '0.46.9' $hash | Out-Null}catch{$rejected=$true}
    if(-not $rejected){throw "installation gate accepted $kind"}
}
$workspace='D:\DEV_D\NodeBridge'
$wc=@{Server=@{path="$workspace\build\candidate\SyncAgent.exe";hash=$hash};Edge=$candidates.Edge}
$wp=@{Server=@{ExecutablePath=$wc.Server.path;ProcessId=3};Edge=$processes.Edge}
$wr=@{Edge=$registrations.Edge}
$proof=Assert-WideInstalledRuntime $wr $wp $wc '0.46.9' $hash -Mode WorkspaceServer -WorkspaceRoot $workspace
if($proof.Server.kind -ne 'workspace' -or $proof.Edge.kind -ne 'installed' -or $proof.Server.registered_version){throw 'workspace mislabeled as installed'}
$ready=@{runtime_verified=$true;runtime_verification=$proof}
Assert-WideReadinessRuntime $ready WorkspaceServer $wc.Server.path $wc.Edge.path
foreach($kind in @('escape','prefix','wrong_hash','old_process','edge_version','edge_temporary')){
    $c=@{Server=$wc.Server.Clone();Edge=$wc.Edge.Clone()};$p=@{Server=$wp.Server.Clone();Edge=$wp.Edge.Clone()};$r=@{Edge=$wr.Edge.Clone()}
    switch($kind){
        escape {$c.Server.path="$workspace\..\outside\SyncAgent.exe";$p.Server.ExecutablePath=$c.Server.path}
        prefix {$c.Server.path="${workspace}-other\SyncAgent.exe";$p.Server.ExecutablePath=$c.Server.path}
        wrong_hash {$c.Server.hash='B'*64}
        old_process {$p.Server.ExecutablePath=$processes.Server.ExecutablePath}
        edge_version {$r.Edge.version='0.46.8'}
        edge_temporary {$c.Edge.path="$workspace\SyncAgent.exe";$p.Edge.ExecutablePath=$c.Edge.path}
    }
    $rejected=$false
    try{Assert-WideInstalledRuntime $r $p $c '0.46.9' $hash -Mode WorkspaceServer -WorkspaceRoot $workspace|Out-Null}catch{$rejected=$true}
    if(-not $rejected){throw "workspace gate accepted $kind"}
}
foreach($kind in @('mode','path','missing','kind')){
    $r=$ready|ConvertTo-Json -Depth 8|ConvertFrom-Json
    switch($kind){mode {$r.runtime_verification.mode='Installed'};path {$r.runtime_verification.Server.path="$workspace\other.exe"};missing {$r.runtime_verified=$false};kind {$r.runtime_verification.Server.kind='installed'}}
    $rejected=$false
    try{Assert-WideReadinessRuntime $r WorkspaceServer $wc.Server.path $wc.Edge.path}catch{$rejected=$true}
    if(-not $rejected){throw "readiness gate accepted $kind"}
}
$before=Get-WideExpectedCounts -1 600
if($before.inserts -ne 0 -or $before.updates -ne 0){throw 'pre-start work counted'}
foreach($case in @(@('plain','"plain"'),@('a b','"a b"'),@('a"b','"a\"b"'),@('C:\folder\','"C:\folder\\"'),@('','""'))){
    if((ConvertTo-WideArgument $case[0]) -cne $case[1]){throw "argument quoting failed: $($case[0])"}
}
$temp=Join-Path ([IO.Path]::GetTempPath()) ('wide-test-'+[guid]::NewGuid().ToString('N')+'.json')
try {
    $message=[string][char]0x6062+[char]0x590D+[char]0x5931+[char]0x8D25
    [IO.File]::WriteAllText($temp,(@{error=$message}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
    if((Read-JsonFile $temp).error -cne $message){throw 'UTF8 JSON changed'}
} finally { Remove-Item -LiteralPath $temp -Force }
if($null -ne (Read-JsonFile $temp)){throw 'missing JSON must return null'}
$samples=@(1..36|ForEach-Object { @{node=400;phases=@{node='steady'}} })
$samples+=@(1..4|ForEach-Object { @{node=2458;phases=@{node='peak'}} })
$summary=Get-WideLatencySummary $samples node
if($summary.all.count -ne 40 -or $summary.stress.count -ne 4 -or $summary.stress.max_ms -ne 2458 -or $summary.steady.p95_ms -ne 400){throw 'latency phase report discarded pressure samples'}
$samples[0].node=3000;$samples[1].node=3000
$rejected=$false
try{Get-WideLatencySummary $samples node|Out-Null}catch{$rejected=$true}
if(-not $rejected){throw 'steady latency violation accepted'}
$samples[0].node=400;$samples[1].node=400;$samples[39].node=6000
$rejected=$false
try{Get-WideLatencySummary $samples node|Out-Null}catch{$rejected=$true}
if(-not $rejected){throw 'stress all-sample P99 violation accepted'}
$samples[39].node=2458;$samples[0].phases.node='unknown'
$rejected=$false
try{Get-WideLatencySummary $samples node|Out-Null}catch{$rejected=$true}
if(-not $rejected){throw 'unclassified latency sample accepted'}
$assignments=$ast.FindAll({param($n)$n -is [Management.Automation.Language.AssignmentStatementAst]},$true)
foreach($assignment in $assignments){
    if($assignment.Left.Extent.Text -ieq '$latency'){throw 'unscoped latency assignment collides with background job'}
}
# A malformed job must not skip either node restoration or the final evidence write.
$script:Scenario=@{invalid=$true};$script:Metrics=$null;$script:Latency=$null
$script:Producer=$null;$script:Tunnel=$null;$script:SiteChanged=$true
$script:Manifest=@{original=@{Edge=@{ExecutablePath='original'};Server=@{ExecutablePath='original'}}}
$EvidenceRoot='TestDrive';$ServerRulesPath='rules';$EdgeRulesPath='rules';$ServerConfigPath='config';$EdgeConfigPath='config'
$script:started=@();$script:proof=$null
function Get-AgentProcess($Side){return @{ExecutablePath='original';ProcessId=1}}
function Stop-Agent($Side){}
function Start-Agent($Side){$script:started+=$Side}
function Copy-ToEdge($From,$To){}
function Copy-FromEdge($From,$To){}
function Copy-Item {}
function Get-FileHash {return @{Hash='same'}}
function Write-JsonAtomic($Path,$Data){$script:proof=$Data}
$caught=$false
try { Restore-WideSite } catch { $caught=$true }
if(-not $caught -or $script:started.Count -ne 2 -or -not $script:proof -or $script:proof.passed -or $script:proof.errors.Count -ne 1){throw 'cleanup failure bypassed site recovery or evidence'}
'Wide supervisor arithmetic, quoting, UTF8, cleanup isolation and parser passed'
