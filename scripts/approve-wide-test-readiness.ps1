param(
    [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9_]{1,35}$')][string]$RunId,
    [ValidateRange(25200,86400)][int]$FormalDurationSeconds = 25200
)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$cache=Join-Path $root '.cache\wide-seven-hour'
$run=Join-Path $cache $RunId
function Read-Proof([string]$Path){return Get-Content -LiteralPath $Path -Encoding UTF8 -Raw | ConvertFrom-Json}
function Require([bool]$Condition,[string]$Message){if(-not $Condition){throw $Message}}
$state=Read-Proof (Join-Path $run 'state.json')
$manifest=Read-Proof (Join-Path $run 'manifest.json')
$gates=Read-Proof (Join-Path $run 'gates.json')
$restored=Read-Proof (Join-Path $run 'restoration.json')
$package=Read-Proof (Join-Path $cache 'package-gates.json')
$current=Read-Proof (Join-Path $cache 'current.json')
Require ($current.run_id -eq $RunId) 'Only the latest run may certify readiness'
Require ($state.status -eq 'completed' -and $state.smoke -and $manifest.duration_seconds -ge 1800) 'A completed smoke of at least thirty minutes is required'
$runtime=$manifest.runtime_verification
Require ($runtime.passed -and $runtime.agent_sha256 -eq $manifest.package_agent_sha256 -and $runtime.version -eq $package.version -and $runtime.mode -in @('Installed','WorkspaceServer')) 'Smoke candidate runtime proof missing or mismatched'
foreach($side in @('Server','Edge')){
    $kind=if($side -eq 'Server' -and $runtime.mode -eq 'WorkspaceServer'){'workspace'}else{'installed'}
    Require ($runtime.$side.kind -eq $kind -and $runtime.$side.path -eq $manifest.original.$side.ExecutablePath -and $restored.$side.path -eq $runtime.$side.path) "Runtime kind, original or restored path mismatch: $side"
    if($kind -eq 'installed'){Require ($runtime.$side.registered_version -eq $package.version) "Registered version mismatch: $side"}
    else{Require ([IO.Path]::GetFullPath($runtime.$side.path).StartsWith(([IO.Path]::GetFullPath($root).TrimEnd('\')+'\'),[StringComparison]::OrdinalIgnoreCase)) 'Server runtime outside workspace'}
}
Require (([DateTimeOffset]$state.finished_at-[DateTimeOffset]$state.started_at).TotalSeconds -ge $manifest.duration_seconds) 'Smoke ended before its required duration'
Require (([DateTimeOffset]$state.finished_at-[DateTimeOffset]$state.planned_end_at).TotalSeconds -le 10) 'Smoke exceeded its deadline'
Require ($gates.passed -and $gates.columns -eq 50 -and $gates.life_cycles -ge 2 -and $gates.queue_quiet_samples -ge 3 -and $gates.exact_producer_ledger -and $gates.all_business_columns_updated -and $gates.replay_metadata_proven) 'Wide-table gates incomplete'
Require ($restored.passed -and @($restored.errors).Count -eq 0) 'Site restoration failed'
Require ($package.passed -and $package.agent_sha256 -eq $manifest.package_agent_sha256) 'Smoke/package agent mismatch'
Require ((Get-FileHash $package.installer_path).Hash -eq $package.installer_sha256) 'Installer changed after verification'
Require ((Get-FileHash (Join-Path $cache 'labwide.exe')).Hash -eq $manifest.workload_sha256) 'Workload changed after smoke'
Require ((Get-FileHash (Join-Path $root 'scripts\lab-real-wide-seven-hour.ps1')).Hash -eq $manifest.script_sha256) 'Supervisor changed after smoke'
Require ((Get-FileHash (Join-Path $root 'scripts\fixtures\longtest-wide50.schema.json')).Hash -eq $manifest.schema_sha256) 'Schema changed after smoke'
foreach($file in @('consistency.json','event-ledger-consistency.json','version-observations.json','idempotency.json','late\idempotency.json')){Require ([bool](Read-Proof (Join-Path $run $file)).passed) "Proof not passed: $file"}
foreach($flag in @('edgeStopped','edgeStarted','serverStopped','serverStarted','ddlAdd','ddlDrop','lateProbe','lateHandoff')){Require ([bool]$gates.scenario_flags.$flag) "Missing scenario $flag"}
foreach($node in @('edge-001','server-001')){Require ($gates.latency.$node.all.count -ge 40 -and $gates.latency.$node.steady.count -ge 25 -and $gates.latency.$node.stress.count -ge 3 -and $gates.latency.$node.steady.p95_ms -le 2000 -and $gates.latency.$node.all.p99_ms -le 5000) "Latency gate failed: $node"}
$history=@($gates.disk_history | Sort-Object at)
$span=$history[-1].at-$history[0].at
Require ($span -ge $manifest.duration_seconds*0.9) 'Measured disk observation shorter than 90 percent of the smoke duration'
$growth=@{}
foreach($disk in @('server_data_free','edge_data_free')){
    $minimum=($history | ForEach-Object {$_.$disk} | Measure-Object -Minimum).Minimum
    $growth[$disk]=[Math]::Max(1,($history[0].$disk-$minimum)/$span)
}
$ready=@{passed=$true;at=(Get-Date).ToString('o');run_id=$RunId;evidence_root=$run;agent_sha256=$manifest.package_agent_sha256;helper_sha256=$manifest.workload_sha256;script_sha256=$manifest.script_sha256;schema_sha256=$manifest.schema_sha256;installer_sha256=$package.installer_sha256;disk_growth_bytes_per_second=$growth;measured_disk_seconds=$span;scope='Ready to begin the formal test, not a seven-hour pass'}
$ready.installed_runtime_verified=$runtime.mode -eq 'Installed'
$ready.formal_duration_seconds=$FormalDurationSeconds
$ready.scope='Ready to begin the duration-bound formal test, not a long-test pass'
$ready.runtime_verified=$true
$ready.runtime_verification=$runtime
$ready.smoke_duration_seconds=$manifest.duration_seconds
$ready.site_sha256=@{}
foreach($side in @('server','edge')){foreach($kind in @('config','rules')){$ready.site_sha256["${side}_${kind}"]=(Get-FileHash -LiteralPath (Join-Path $run "$side-$kind.original.yaml")).Hash}}
$path=Join-Path $cache 'formal-ready.json'
[IO.File]::WriteAllText($path+'.tmp',($ready|ConvertTo-Json -Depth 8),[Text.UTF8Encoding]::new($false))
Move-Item -LiteralPath ($path+'.tmp') -Destination $path -Force
$ready | ConvertTo-Json -Depth 8
