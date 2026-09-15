param([ValidateRange(1,25200)][int]$DurationSeconds=25200,[ValidateRange(1,16)][int]$BatchSize=16)
$ErrorActionPreference='Stop'
$repo=Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $repo
. "$PSScriptRoot/lib/env.ps1" -RepoRoot $repo
$run=Get-Date -Format yyyyMMdd-HHmmss
$root=Join-Path $repo ".cache/owned-soak/$run"
New-Item -ItemType Directory -Path $root -Force | Out-Null
$state=@{status='preparing';pid=$PID;started_at=(Get-Date).ToString('o');duration_seconds=$DurationSeconds;batch_size=$BatchSize;evidence_root=$root}
function Save-State { $state | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath "$root/supervisor.json" -Encoding utf8 }
Add-Type -TypeDefinition @'
using System.Runtime.InteropServices;
public static class OwnedSoakAwake {
 [DllImport("kernel32.dll")] public static extern uint SetThreadExecutionState(uint flags);
}
'@
$paths=@('C:/ProgramData/NodeBridge/config.yaml','C:/ProgramData/NodeBridge/sync-rules.yaml')
$before=@{}
foreach($path in $paths){if(Test-Path -LiteralPath $path){$before[$path]=(Get-FileHash -LiteralPath $path).Hash}}
$state.original_hashes=$before
Save-State
try {
 if([OwnedSoakAwake]::SetThreadExecutionState([uint32]2147483649) -eq 0){throw 'Awake request failed'}
 $candidate="$repo/build/bin/SyncAgent.exe"
 if((Get-FileHash -LiteralPath $candidate).Hash -ne 'BD3ECC8ACC1C217484D2BD7E5C001FE9615A749F79196C091196C6B727DD580A'){throw 'Expected verified 0.48.5 candidate'}
 $state.status='running';Save-State
 & "$PSScriptRoot/test-multi-node-sync.ps1" -CandidatePath $candidate -Source server -InterCopyWrites -SoakSeconds $DurationSeconds -BatchSize $BatchSize *>&1 | Tee-Object -FilePath "$root/output.log"
 $state.status='passed'
}catch{
 $state.status='failed';$state.error=($_|Out-String)
 $_ | Out-File -LiteralPath "$root/error.log"
}finally{
 $state.originals_unchanged=$true
 foreach($path in $before.Keys){if((Get-FileHash -LiteralPath $path).Hash -ne $before[$path]){$state.originals_unchanged=$false;$state.status='failed'}}
 $null=[OwnedSoakAwake]::SetThreadExecutionState([uint32]2147483648)
 $state.finished_at=(Get-Date).ToString('o');Save-State
}
if($state.status -ne 'passed'){exit 1}
