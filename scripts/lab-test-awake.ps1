param(
    [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9_]{1,35}$')][string]$RunId,
    [ValidateRange(10,90000)][int]$DurationSeconds=25800,
    [string]$RulesPath='C:\ProgramData\NodeBridge\sync-rules.yaml',
    [Parameter(Mandatory)][string]$EvidencePath
)
$ErrorActionPreference='Stop'
$deadline=[DateTimeOffset]::UtcNow.AddSeconds($DurationSeconds)
$reason='deadline'
$requestActive=$false
$result=@{run_id=$RunId;pid=$PID;status='starting';started_at=(Get-Date).ToString('o');deadline=$deadline.ToString('o');rules_path=$RulesPath;policy_changed=$false}
function Test-RunRules {
    $stream=[IO.File]::Open($RulesPath,[IO.FileMode]::Open,[IO.FileAccess]::Read,([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
    try {
        $reader=[IO.StreamReader]::new($stream,[Text.Encoding]::UTF8,$true)
        try { return $reader.ReadToEnd() -match ('(?m)^\s*- id: '+[regex]::Escape($RunId)+'-e-stream\s*$') }
        finally { $reader.Dispose() }
    } finally { $stream.Dispose() }
}
try {
    if(-not (Test-RunRules)){throw 'requested test rules are not active'}
    Add-Type -TypeDefinition @'
using System.Runtime.InteropServices;
public static class NodeBridgeTestAwake {
    [DllImport("kernel32.dll", SetLastError=true)]
    public static extern uint SetThreadExecutionState(uint flags);
}
'@
    if([NodeBridgeTestAwake]::SetThreadExecutionState([uint32]2147483649) -eq 0){throw 'system awake request failed'}
    $requestActive=$true
    $result.status='active'
    [IO.File]::WriteAllText($EvidencePath,($result|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
    while([DateTimeOffset]::UtcNow -lt $deadline) {
        if(-not (Test-RunRules)){$reason='rules_restored';break}
        Start-Sleep -Seconds 2
    }
} catch {
    $reason=$_.Exception.Message
    $result.error=$reason
} finally {
    if($requestActive){$null=[NodeBridgeTestAwake]::SetThreadExecutionState([uint32]2147483648)}
    $result.status=if($result.error){'failed'}else{'released'}
    $result.finished_at=(Get-Date).ToString('o')
    $result.reason=$reason
    [IO.File]::WriteAllText($EvidencePath,($result|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
}
if($result.error){throw $result.error}
