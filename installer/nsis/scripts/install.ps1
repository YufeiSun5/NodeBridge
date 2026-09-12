param(
    [string]$InstallRoot = "",
    [string]$Version = "0.46.9",
    [string]$UIStatePath = "",
    [switch]$SkipSystemComponents,
    [switch]$InstallSystemComponents,
    [switch]$RequireCanalService,
    [switch]$TestOnlySkipAdminCheck
)

$ErrorActionPreference = "Stop"

if ($SkipSystemComponents -and $InstallSystemComponents) {
    throw "SkipSystemComponents and InstallSystemComponents are mutually exclusive."
}
# Installing system services always requires an explicit selection.
$SkipSystemComponents = -not [bool]$InstallSystemComponents

if ($InstallRoot -eq "") {
    $InstallRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
}

$runId = (Get-Date).ToString("yyyyMMdd-HHmmss-fff") + "-" + [guid]::NewGuid().ToString("N").Substring(0, 8)
$runtimeDir = Join-Path $env:ProgramData "NodeBridgeInstallerLogs\$runId"
$summaryPath = Join-Path $runtimeDir "nsis-beta-install-summary.json"
$logPath = Join-Path $runtimeDir "nsis-beta-install.log"
$steps = [System.Collections.Generic.List[object]]::new()
$transcriptStarted = $false

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

function Write-Summary {
    param(
        [string]$Status,
        [string]$Message
    )
    New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
    [ordered]@{
        created_at = (Get-Date).ToString("o")
        version = $Version
        status = $Status
        message = $Message
        install_root = $InstallRoot
        diagnostic_directory = $runtimeDir
        is_64_bit_process = [Environment]::Is64BitProcess
        program_data = Join-Path $env:ProgramData "NodeBridge"
        skip_system_components = [bool]$SkipSystemComponents
        component_mode = if ($SkipSystemComponents) { "reuse" } else { "install" }
        test_only_skip_admin_check = [bool]$TestOnlySkipAdminCheck
        require_canal_service = [bool]$RequireCanalService
        steps = $steps
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
}

function Test-IsAdmin {
    $principal = [Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Grant-NodeBridgeDataAccess {
    param([string]$Path)
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $sid = $identity.User.Value
    $grant = "*$sid`:(OI)(CI)M"
    $output = & icacls.exe $Path /grant:r $grant /T /C 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "grant NodeBridge data access failed exit_code=$LASTEXITCODE output=$($output -join ' ')"
    }
    return [ordered]@{
        identity = $identity.Name
        sid = $sid
        rights = "Modify"
        path = $Path
    }
}

function Copy-DefaultIfMissing {
    param(
        [string]$Source,
        [string]$Target
    )
    if (-not (Test-Path -LiteralPath $Target)) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Target) | Out-Null
        Copy-Item -LiteralPath $Source -Destination $Target -Force
        return "created"
    }
    return "kept-existing"
}

function Test-ConfigLooksComplete {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        return $false
    }
    $text = Get-Content -LiteralPath $Path -Raw
    return (
        $text -match '(?m)^mode:\s*(edge|server)\s*$' -and
        $text -match '(?ms)^node:\s*.*?^\s+id:\s*"?[^"\s]+' -and
        $text -match '(?ms)^mysql:\s*.*?^\s+database:\s*"?[^"\s]+'
    )
}

function Copy-DefaultConfigIfMissing {
    param(
        [string]$Source,
        [string]$Target
    )
    if (-not (Test-Path -LiteralPath $Target)) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Target) | Out-Null
        Copy-Item -LiteralPath $Source -Destination $Target -Force
        return "created"
    }
    return "kept-existing"
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
        if (-not $TestOnlySkipAdminCheck -and -not (Test-IsAdmin)) {
            throw "Run NodeBridge beta installer as administrator."
        }
    }

    $appDir = Join-Path $InstallRoot "app"
    $headlessRoot = Join-Path $InstallRoot "installer\headless"
    $nodeBridgeExe = Join-Path $appDir "NodeBridge.exe"
    $syncAgentExe = Join-Path $appDir "SyncAgent.exe"
    $programDataDir = Join-Path $env:ProgramData "NodeBridge"
    $programDataConfig = Join-Path $programDataDir "config.yaml"
    $programDataRules = Join-Path $programDataDir "sync-rules.yaml"

    Invoke-Step -Name "required-files" -Block {
        foreach ($path in @(
            $nodeBridgeExe,
            $syncAgentExe,
            (Join-Path $appDir "config.yaml"),
            (Join-Path $appDir "config-external.yaml"),
            (Join-Path $appDir "sync-rules.yaml"),
            (Join-Path $headlessRoot "scripts\headless-installer-test.ps1")
        )) {
            if (-not (Test-Path -LiteralPath $path)) {
                throw "missing required file: $path"
            }
        }
    }

    Invoke-Step -Name "default-config" -Block {
        $bootstrap = if ($SkipSystemComponents) { "config-external.yaml" } else { "config.yaml" }
        $configState = Copy-DefaultConfigIfMissing -Source (Join-Path $appDir $bootstrap) -Target $programDataConfig
        $rulesState = Copy-DefaultIfMissing -Source (Join-Path $appDir "sync-rules.yaml") -Target $programDataRules
        [ordered]@{
            config = $configState
            rules = $rulesState
            config_path = $programDataConfig
            rules_path = $programDataRules
        } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "default-config.json") -Encoding UTF8
    }

    Invoke-Step -Name "config-permissions" -Block {
        Grant-NodeBridgeDataAccess -Path $programDataDir |
            ConvertTo-Json -Depth 4 |
            Set-Content -LiteralPath (Join-Path $runtimeDir "config-permissions.json") -Encoding UTF8
    }

    if ($SkipSystemComponents) {
        Add-Step -Name "managed-config-migration" -Status "skipped" -Message "Reuse mode preserves existing credentials and connection configuration."
    } else {
        Invoke-Step -Name "managed-config-migration" -Block {
            & $syncAgentExe "managed-config-migrate" "-config" $programDataConfig | Tee-Object -FilePath (Join-Path $runtimeDir "managed-config-migration.json") | Out-Null
            if ($LASTEXITCODE -ne 0) {
                throw "managed config migration failed with exit code $LASTEXITCODE"
            }
        }
    }

    Invoke-Step -Name "sync-agent-smoke" -Block {
        & $syncAgentExe "mcp-client-config" "-config" $programDataConfig "-lab-full-access" | Tee-Object -FilePath (Join-Path $runtimeDir "sync-agent-smoke.json") | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "SyncAgent smoke failed with exit code $LASTEXITCODE" }
    }

    if ($SkipSystemComponents) {
        Add-Step -Name "system-components" -Status "skipped" -Message "SkipSystemComponents was set."
    } else {
        Invoke-Step -Name "system-components" -Block {
            $headlessScript = Join-Path $headlessRoot "scripts\headless-installer-test.ps1"
            $componentRuntime = Join-Path $runtimeDir "components"
            $args = @("-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", $headlessScript, "-ExecuteInstall", "-SkipManagedApply", "-RuntimeDirectory", $componentRuntime, "-BundleVersion", $Version)
            if ($RequireCanalService) {
                $args += "-RequireCanalService"
            }
            $previousLocation = Get-Location
            try {
                Set-Location -LiteralPath $headlessRoot
                & powershell.exe @args 1> (Join-Path $runtimeDir "component-headless-install.out.txt") 2> (Join-Path $runtimeDir "component-headless-install.err.txt")
                $exitCode = $LASTEXITCODE
            } finally {
                Set-Location -LiteralPath $previousLocation
                $headlessSummary = Join-Path $componentRuntime "headless-installer-summary.json"
                if (Test-Path -LiteralPath $headlessSummary) {
                    Copy-Item -LiteralPath $headlessSummary -Destination (Join-Path $runtimeDir "component-headless-installer-summary.json") -Force
                }
            }
            if ($exitCode -ne 0) {
                throw "headless component install failed with exit code $exitCode"
            }
        }
    }

    if ($SkipSystemComponents) {
        Add-Step -Name "managed-node-configuration" -Status "skipped" -Message "Reuse mode does not change component configuration or topology."
    } elseif (Test-ConfigLooksComplete -Path $programDataConfig) {
        Invoke-Step -Name "managed-node-configuration" -Block {
            $manifestPath = Join-Path $programDataDir "install-manifest.json"
            & $syncAgentExe "managed-apply" "-config" $programDataConfig "-manifest" $manifestPath "-version" $Version |
                Tee-Object -FilePath (Join-Path $runtimeDir "managed-node-configuration.json") | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "managed node configuration failed with exit code $LASTEXITCODE" }
        }
    } else {
        Add-Step -Name "managed-node-configuration" -Status "skipped" -Message "Node identity and MySQL database will be configured through UI or MCP."
    }

    if ($TestOnlySkipAdminCheck) {
        Add-Step -Name "restore-ui" -Status "skipped" -Message "UI restore is disabled in installer test mode."
    } else {
        try {
            . (Join-Path $PSScriptRoot "restore-ui.ps1")
            $restoreMessage = Restore-NodeBridgeUI -InstallRoot $InstallRoot -StatePath $UIStatePath
            Add-Step -Name "restore-ui" -Status "passed" -Message $restoreMessage
        } catch {
            Add-Step -Name "restore-ui" -Status "warning" -Message $_.Exception.Message
            Write-Warning "Installation completed, but UI restore failed: $($_.Exception.Message)"
        }
    }
    Write-Summary -Status "passed" -Message "NodeBridge beta install completed."
    exit 0
} catch {
    Write-Summary -Status "failed" -Message $_.Exception.Message
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
} finally {
    if ($transcriptStarted) {
        try {
            Stop-Transcript | Out-Null
        } catch {
        }
    }
}
