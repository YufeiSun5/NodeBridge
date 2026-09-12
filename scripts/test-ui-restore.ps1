param()
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../installer/nsis/scripts/restore-ui.ps1')
$root = Join-Path $PSScriptRoot '../.cache/ui-restore-test'
$root = [IO.Path]::GetFullPath($root)
$sid = 'S-1-5-21-1-2-3-1001'
$state = [pscustomobject]@{ install_root=$root; sessions=@(
    [pscustomobject]@{session_id=2; user_sid=$sid},
    [pscustomobject]@{session_id=2; user_sid=$sid}
) }
function Assert-True([bool]$Value, [string]$Name) {
    if (-not $Value) { throw "FAIL: $Name" }
    Write-Output "PASS: $Name"
}
Assert-True (@(Get-NodeBridgeUIRestoreTargets $root $state).Count -eq 1) 'deduplicates original session'
foreach ($bad in @(
    [pscustomobject]@{install_root=$root+'-other';sessions=$state.sessions},
    [pscustomobject]@{install_root=$root;sessions=@([pscustomobject]@{session_id=0;user_sid=$sid})},
    [pscustomobject]@{install_root=$root;sessions=@([pscustomobject]@{session_id=2;user_sid='invalid'})}
)) {
    $failed=$false
    try { Get-NodeBridgeUIRestoreTargets $root $bad | Out-Null } catch { $failed=$true }
    Assert-True $failed 'rejects invalid root/session/owner'
}
Assert-True ((Restore-NodeBridgeUI $root '') -eq 'No prior UI to restore.') 'fresh install does not launch UI'

# Only the pure selector is real; all Windows and file operations below are fixtures.
$script:launched=$false
$script:registered=$false
$script:removed=$false
$script:desktopSessions=@(2)
$script:failStart=$false
function Test-Path { param($LiteralPath,$PathType) return $true }
function Get-Content { param($LiteralPath,[switch]$Raw) return ($state | ConvertTo-Json -Depth 5) }
function Get-CimInstance {
    param($ClassName,$Filter)
    if ($Filter -eq "Name='explorer.exe'") {
        foreach ($id in $script:desktopSessions) { [pscustomobject]@{SessionId=$id} }
    } elseif ($script:launched) {
        [pscustomobject]@{SessionId=2;ExecutablePath=(Join-Path $root 'app\NodeBridge.exe')}
    }
}
function Invoke-CimMethod { param($InputObject,$MethodName) return [pscustomobject]@{ReturnValue=0;Sid=$sid} }
function New-ScheduledTaskAction { param($Execute,$WorkingDirectory) return [pscustomobject]@{Execute=$Execute} }
function New-ScheduledTaskPrincipal {
    param($UserId,$LogonType,$RunLevel)
    if ($UserId -ne $sid -or $LogonType -ne 'Interactive' -or $RunLevel -ne 'Limited') { throw 'unsafe principal' }
    return [pscustomobject]@{UserId=$UserId}
}
function New-ScheduledTaskSettingsSet { param([switch]$AllowStartIfOnBatteries,[switch]$DontStopIfGoingOnBatteries,$ExecutionTimeLimit) return @{} }
function Register-ScheduledTask { param($TaskName,$Action,$Principal,$Settings,[switch]$Force) $script:registered=$true }
function Start-ScheduledTask { param($TaskName) if ($script:failStart) { throw 'injected launch failure' }; $script:launched=$true }
function Unregister-ScheduledTask { param($TaskName,[switch]$Confirm,$ErrorAction) $script:removed=$true }
function Start-Sleep { param($Milliseconds) }
$message=Restore-NodeBridgeUI $root 'fixture.json'
Assert-True ($script:registered -and $script:launched -and $script:removed -and $message -like 'Restored*') 'launches limited interactive task and removes task'
$script:registered=$false
Restore-NodeBridgeUI $root 'fixture.json' | Out-Null
Assert-True (-not $script:registered) 'does not duplicate an existing UI'
$script:launched=$false
$script:desktopSessions=@(2,3)
$failed=$false
try { Restore-NodeBridgeUI $root 'fixture.json' | Out-Null } catch { $failed=$true }
Assert-True ($failed -and -not $script:registered) 'ambiguous user session fails closed'
$script:desktopSessions=@(2)
$script:failStart=$true
$script:removed=$false
$failed=$false
try { Restore-NodeBridgeUI $root 'fixture.json' | Out-Null } catch { $failed=$true }
Assert-True ($failed -and $script:removed) 'launch failure still removes temporary task'
