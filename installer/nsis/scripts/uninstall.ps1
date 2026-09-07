param(
    [string]$InstallRoot = "",
    [switch]$SkipSystemComponents,
    [switch]$RemoveData
)

$ErrorActionPreference = "Stop"

if ($InstallRoot -eq "") {
    $InstallRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
}

$runtimeDir = Join-Path $InstallRoot "runtime"
$summaryPath = Join-Path $runtimeDir "nsis-beta-uninstall-summary.json"
$logPath = Join-Path $runtimeDir "nsis-beta-uninstall.log"
$steps = [System.Collections.Generic.List[object]]::new()
$transcriptStarted = $false

function Write-Summary {
    param(
        [string]$Status,
        [string]$Message
    )
    New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
    [ordered]@{
        created_at = (Get-Date).ToString("o")
        status = $Status
        message = $Message
        install_root = $InstallRoot
        remove_data = [bool]$RemoveData
        skip_system_components = [bool]$SkipSystemComponents
        steps = $steps
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
}

function Add-Step {
    param(
        [string]$Name,
        [string]$Status,
        [string]$Message = ""
    )
    $steps.Add([ordered]@{
        name = $Name
        status = $Status
        message = $Message
        time = (Get-Date).ToString("o")
    })
    Write-Summary -Status "running" -Message ""
}

function Set-Step {
    param(
        [string]$Name,
        [string]$Status,
        [string]$Message = ""
    )
    for ($i = $steps.Count - 1; $i -ge 0; $i--) {
        if ($steps[$i].name -eq $Name -and $steps[$i].status -eq "running") {
            $steps[$i].status = $Status
            $steps[$i].message = $Message
            $steps[$i].time = (Get-Date).ToString("o")
            Write-Summary -Status "running" -Message ""
            return
        }
    }
    Add-Step -Name $Name -Status $Status -Message $Message
}

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Block
    )
    Add-Step -Name $Name -Status "running"
    try {
        & $Block
        Set-Step -Name $Name -Status "passed"
    } catch {
        Set-Step -Name $Name -Status "failed" -Message $_.Exception.Message
        throw
    }
}

function Test-IsAdmin {
    $principal = [Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Stop-InstalledProcess {
    param([string]$ExeName)
    $root = [System.IO.Path]::GetFullPath($InstallRoot)
    $processes = Get-CimInstance Win32_Process -Filter "Name = '$ExeName'" -ErrorAction SilentlyContinue
    foreach ($process in $processes) {
        $exePath = [string]$process.ExecutablePath
        if ($exePath -and [System.IO.Path]::GetFullPath($exePath).StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) {
            Stop-Process -Id $process.ProcessId -Force -ErrorAction SilentlyContinue
        }
    }
}

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
try {
    Start-Transcript -Path $logPath -Force | Out-Null
    $transcriptStarted = $true
} catch {
    $transcriptStarted = $false
}

try {
    Write-Summary -Status "running" -Message ""

    Invoke-Step -Name "admin-check" -Block {
        if (-not (Test-IsAdmin)) {
            throw "Run NodeBridge beta uninstaller as administrator."
        }
    }

    Invoke-Step -Name "stop-installed-processes" -Block {
        Stop-InstalledProcess -ExeName "NodeBridge.exe"
        Stop-InstalledProcess -ExeName "DataSync.exe"
        Stop-InstalledProcess -ExeName "SyncAgent.exe"
    }

    if ($SkipSystemComponents) {
        Add-Step -Name "system-components" -Status "skipped" -Message "SkipSystemComponents was set."
    } else {
        Invoke-Step -Name "system-components" -Block {
            $headlessRoot = Join-Path $InstallRoot "installer\headless"
            $headlessScript = Join-Path $headlessRoot "scripts\headless-installer-test.ps1"
            if (-not (Test-Path -LiteralPath $headlessScript)) {
                throw "missing headless uninstall script: $headlessScript"
            }
            $args = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $headlessScript, "-Uninstall")
            if ($RemoveData) {
                $args += "-RemoveData"
            }
            $previousLocation = Get-Location
            try {
                Set-Location -LiteralPath $headlessRoot
                & powershell.exe @args 1> (Join-Path $runtimeDir "component-headless-uninstall.out.txt") 2> (Join-Path $runtimeDir "component-headless-uninstall.err.txt")
                $exitCode = $LASTEXITCODE
            } finally {
                Set-Location -LiteralPath $previousLocation
            }
            if ($exitCode -ne 0) {
                throw "headless component uninstall failed with exit code $exitCode"
            }
            $headlessSummary = Join-Path $headlessRoot "runtime\headless-installer-summary.json"
            if (Test-Path -LiteralPath $headlessSummary) {
                Copy-Item -LiteralPath $headlessSummary -Destination (Join-Path $runtimeDir "component-headless-uninstall-summary.json") -Force
            }
        }
    }

    Write-Summary -Status "passed" -Message "NodeBridge beta uninstall completed."
    exit 0
} catch {
    Write-Summary -Status "failed" -Message $_.Exception.Message
    Write-Error $_.Exception.Message
    exit 1
} finally {
    if ($transcriptStarted) {
        try {
            Stop-Transcript | Out-Null
        } catch {
        }
    }
}
