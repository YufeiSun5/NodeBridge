param(
    [Parameter(Mandatory=$true)][string]$RunId,
    [ValidateRange(1,30)][int]$Samples=6,
    [string]$EdgeHost='192.168.10.105',
    [string]$EdgeUser='xx'
)
$ErrorActionPreference='Stop'
if ($RunId -notmatch '^[a-zA-Z0-9_]+$') { throw 'Invalid run ID' }
$root=Split-Path -Parent $PSScriptRoot
$cache=Join-Path $root '.cache/wide-seven-hour'
$config=Join-Path (Join-Path $cache $RunId) 'workload.json'
$settings=Get-Content -LiteralPath $config -Raw -Encoding UTF8 | ConvertFrom-Json
if ($settings.edge_dsn -notmatch '@tcp\(127\.0\.0\.1:(?<port>\d+)\)/') { throw 'Expected loopback SSH tunnel DSN' }
$port=[int]$Matches.port
if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { throw 'Diagnostic tunnel port is already occupied' }
$helper=Join-Path $cache 'fb066-diagnostic.exe'
if (-not (Test-Path -LiteralPath $helper)) { throw 'Build fb066-diagnostic.exe first' }
$evidence=Join-Path $cache ('fb066-diagnostic-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))
$null=New-Item -Path $evidence -ItemType Directory
$key=Join-Path $HOME '.ssh/nodebridge_ed25519'
$tunnel=$null
$results=@()
try {
    $tunnel=Start-Process -FilePath ssh.exe -ArgumentList "-N -T -i `"$key`" -o BatchMode=yes -o ExitOnForwardFailure=yes -o ServerAliveInterval=15 -o ServerAliveCountMax=3 -L 127.0.0.1:${port}:127.0.0.1:3306 $EdgeUser@$EdgeHost" -WindowStyle Hidden -PassThru -RedirectStandardError (Join-Path $evidence 'tunnel.err')
    Start-Sleep -Seconds 2
    if ($tunnel.HasExited) { throw 'Diagnostic tunnel failed' }
    for($i=1;$i -le $Samples;$i++) {
        $out=Join-Path $evidence "$i.out"
        $err=Join-Path $evidence "$i.err"
        $started=[DateTimeOffset]::UtcNow
        $p=Start-Process -FilePath $helper -ArgumentList "-config `"$config`" -action snapshot" -WindowStyle Hidden -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
        $null=$p.Handle
        $timedOut=-not $p.WaitForExit(15000)
        if($timedOut){$p.Kill();$p.WaitForExit()}
        $record=[ordered]@{sample=$i;at=$started.ToString('o');elapsed_ms=([DateTimeOffset]::UtcNow-$started).TotalMilliseconds;exit_code=$p.ExitCode;outer_timeout=$timedOut}
        $results+=$record
        $record|ConvertTo-Json -Compress
        if($i -lt $Samples){Start-Sleep -Seconds 2}
    }
} finally {
    if($tunnel -and -not $tunnel.HasExited){$tunnel.Kill();$tunnel.WaitForExit()}
    [ordered]@{run_id=$RunId;helper_sha256=(Get-FileHash -LiteralPath $helper -Algorithm SHA256).Hash;read_only=$true;samples=$results;tunnel_released=($null -eq $tunnel -or $tunnel.HasExited)} | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $evidence 'summary.json') -Encoding UTF8
    Write-Output "Evidence: $evidence"
}
