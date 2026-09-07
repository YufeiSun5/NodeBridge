param(
    [string]$InstallRoot,
    [switch]$TestOnlySkipAdminCheck
)

$ErrorActionPreference = "Stop"

function Test-IsAdmin {
    $principal = [Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Test-PathInsideRoot {
    param([string]$Path, [string]$Root)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $false
    }
    $resolvedPath = [System.IO.Path]::GetFullPath($Path)
    $resolvedRoot = [System.IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
    return $resolvedPath.StartsWith($resolvedRoot, [System.StringComparison]::OrdinalIgnoreCase)
}

if (-not $TestOnlySkipAdminCheck -and -not (Test-IsAdmin)) {
    throw "NodeBridge upgrade preflight requires administrator privileges."
}

if ([string]::IsNullOrWhiteSpace($InstallRoot) -or -not (Test-Path -LiteralPath $InstallRoot)) {
    exit 0
}

$programDataRoot = Join-Path $env:ProgramData "NodeBridge"
$stopFile = Join-Path $programDataRoot "run\sync-agent.stop"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $stopFile) | Out-Null
New-Item -ItemType File -Force -Path $stopFile | Out-Null

$targets = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
    $_.Name -in @("NodeBridge.exe", "DataSync.exe", "SyncAgent.exe") -and
    (Test-PathInsideRoot -Path ([string]$_.ExecutablePath) -Root $InstallRoot)
}

$deadline = (Get-Date).AddSeconds(10)
while ($targets -and (Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 250
    $targetIds = @($targets.ProcessId)
    $targets = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $targetIds -contains $_.ProcessId }
}

foreach ($process in @($targets)) {
    Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
}

$remaining = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
    $_.Name -in @("NodeBridge.exe", "DataSync.exe", "SyncAgent.exe") -and
    (Test-PathInsideRoot -Path ([string]$_.ExecutablePath) -Root $InstallRoot)
}
if ($remaining) {
    throw "NodeBridge processes are still running under $InstallRoot"
}
