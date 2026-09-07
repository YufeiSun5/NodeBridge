param(
    [string]$RunId = "staged-v044-month30-001",
    [string]$Root = "D:\DEV_D\NodeBridge"
)

$ErrorActionPreference = "Stop"

$runScript = Join-Path $Root ".cache\longtest-90d\run-$RunId.ps1"
$watchScript = Join-Path $Root "scripts\longtest-30d-watch.ps1"
$evidence = Join-Path $Root ".cache\longtest-90d\$RunId"
$supervisorLog = Join-Path $evidence "supervisor-start.txt"
New-Item -ItemType Directory -Force -Path $evidence | Out-Null

$existing = Get-CimInstance Win32_Process -Filter "Name = 'powershell.exe' OR Name = 'pwsh.exe'" |
    Where-Object { $_.CommandLine -like "*run-$RunId.ps1*" }

if ($existing) {
    "existing runner pid(s): $($existing.ProcessId -join ',')" | Set-Content -Path $supervisorLog -Encoding UTF8
} else {
    if (-not (Test-Path -LiteralPath $runScript)) {
        throw "runner script not found: $runScript"
    }
    $runnerArgs = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $runScript
    )
    $runner = Start-Process -FilePath "powershell.exe" -ArgumentList $runnerArgs -PassThru
    "started runner pid: $($runner.Id)" | Set-Content -Path $supervisorLog -Encoding UTF8
}

$watchArgs = @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-File", $watchScript,
    "-RunId", $RunId,
    "-Root", $Root,
    "-PollSeconds", "60"
)
$watcher = Start-Process -FilePath "powershell.exe" -ArgumentList $watchArgs -PassThru
"started watcher pid: $($watcher.Id)" | Add-Content -Path $supervisorLog -Encoding UTF8

Write-Host "run_id=$RunId"
Write-Host "evidence=$evidence"
Write-Host "watcher_pid=$($watcher.Id)"
