param(
    [string]$BundleVersion = "0.46.12",
    [switch]$ExecuteInstall,
    [switch]$Uninstall,
    [switch]$VerifyOnly,
    [switch]$RemoveData,
    [switch]$RequireCanalService,
    [switch]$SkipManagedApply,
    [string]$RuntimeDirectory = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$bundleRoot = Split-Path -Parent $scriptDir
$binDir = Join-Path $bundleRoot "bin"
$configDir = Join-Path $bundleRoot "config"
$deployDir = Join-Path $bundleRoot "deploy/windows"
$runtimeDir = Join-Path $bundleRoot "runtime"
if ($RuntimeDirectory) {
    $runtimeDir = [System.IO.Path]::GetFullPath($RuntimeDirectory)
}
$summaryPath = Join-Path $runtimeDir "headless-installer-summary.json"
$rabbitMQOwnershipPath = Join-Path $env:ProgramData "NodeBridge\managed-rabbitmq.json"

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
Push-Location $bundleRoot
if (-not $RuntimeDirectory) {
    Get-ChildItem -LiteralPath $runtimeDir -File -ErrorAction SilentlyContinue | Remove-Item -Force
}
$script:rabbitMQServiceName = ""
$script:rabbitMQServiceOwned = $false
$script:rabbitMQServiceExistedBefore = $false
$script:rabbitMQServiceNamesBefore = @()

function Write-Summary {
    if (-not $steps) {
        return
    }
    $summary = [ordered]@{
        created_at = (Get-Date).ToString("o")
        bundle_root = $bundleRoot
        version = $bundleVersion
        execute_install = [bool]$ExecuteInstall
        sync_agent = $syncAgent
        config = $configPath
        catalog = $catalogPath
        manifest = $manifestPath
        steps = $steps
    }
    $summary | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
}

function Add-Step {
    param(
        [System.Collections.Generic.List[object]]$Steps,
        [string]$Name,
        [string]$Status,
        [string]$Message = ""
    )
    $Steps.Add([ordered]@{
        name = $Name
        status = $Status
        message = $Message
        time = (Get-Date).ToString("o")
    })
    Write-Summary
}

function Set-StepStatus {
    param(
        [System.Collections.Generic.List[object]]$Steps,
        [string]$Name,
        [string]$Status,
        [string]$Message = ""
    )
    for ($i = $Steps.Count - 1; $i -ge 0; $i--) {
        if ($Steps[$i].name -eq $Name -and $Steps[$i].status -eq "running") {
            $Steps[$i].status = $Status
            $Steps[$i].message = $Message
            $Steps[$i].time = (Get-Date).ToString("o")
            Write-Summary
            return
        }
    }
    Add-Step -Steps $Steps -Name $Name -Status $Status -Message $Message
}

function Test-IsAdmin {
    $principal = [Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-Checked {
    param(
        [System.Collections.Generic.List[object]]$Steps,
        [string]$Name,
        [scriptblock]$Block
    )
    Add-Step -Steps $Steps -Name $Name -Status "running"
    try {
        & $Block
        Set-StepStatus -Steps $Steps -Name $Name -Status "passed"
    } catch {
        Set-StepStatus -Steps $Steps -Name $Name -Status "failed" -Message $_.Exception.Message
        throw
    }
}

function Resolve-BundlePath {
    param([string]$Path)
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }
    return Join-Path $bundleRoot $Path
}

function Invoke-ProcessChecked {
    param(
        [string]$FilePath,
        [string[]]$ArgumentList,
        [string]$Name = "process",
        [int]$TimeoutSeconds = 900,
        [scriptblock]$SuccessProbe = $null,
        [int]$ProbeTimeoutSeconds = 300
    )
    $safeName = ($Name -replace "[^a-zA-Z0-9_.-]", "-").ToLowerInvariant()
    $evidencePath = Join-Path $runtimeDir ("process-" + $safeName + ".json")
    $startedAt = Get-Date
    $quotedArguments = @($ArgumentList | ForEach-Object { ConvertTo-NativeArgument $_ })
    $process = Start-Process -FilePath $FilePath -ArgumentList ($quotedArguments -join ' ') -PassThru -WindowStyle Hidden
    # Retain the process handle before waiting so Windows PowerShell can read ExitCode.
    $processHandle = $process.Handle
    $timedOut = $false
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        $timedOut = $true
        try {
            $process.Kill()
            $process.WaitForExit(10000) | Out-Null
        } catch {
        }
    }
    $exitCode = if ($timedOut) { $null } else { $process.ExitCode }
    $probePassed = $false
    $probeMessage = ""
    if (-not $timedOut -and ($null -eq $exitCode -or $exitCode -eq 0 -or $exitCode -eq 3010) -and $null -ne $SuccessProbe) {
        $probeDeadline = (Get-Date).AddSeconds($ProbeTimeoutSeconds)
        do {
            try {
                & $SuccessProbe
                $probePassed = $true
                $probeMessage = "post-install detection passed"
                break
            } catch {
                $probeMessage = $_.Exception.Message
                Start-Sleep -Seconds 2
            }
        } while ((Get-Date) -lt $probeDeadline)
        if (-not $probePassed) {
            $probeMessage = "success probe timed out: $probeMessage"
        }
    }
    $endedAt = Get-Date
    [ordered]@{
        name = $Name
        file = $FilePath
        args = $ArgumentList
        pid = $process.Id
        started_at = $startedAt.ToString("o")
        ended_at = $endedAt.ToString("o")
        timeout_seconds = $TimeoutSeconds
        timed_out = $timedOut
        exit_code = $exitCode
        exit_code_missing_but_detected = ($null -eq $exitCode -and $probePassed)
        is_64_bit_process = [Environment]::Is64BitProcess
        program_files = $env:ProgramFiles
        program_w6432 = $env:ProgramW6432
        success_probe = $probeMessage
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $evidencePath -Encoding UTF8
    $process.Dispose()
    if ($timedOut) {
        throw "process timed out after ${TimeoutSeconds}s file=$FilePath args=$($ArgumentList -join ' ')"
    }
    if ($null -eq $exitCode -and $probePassed) {
        return
    }
    if ($null -eq $exitCode) {
        throw "process failed with missing exit_code file=$FilePath args=$($ArgumentList -join ' ') probe=$probeMessage"
    }
    if ($exitCode -ne 0 -and $exitCode -ne 3010) {
        throw "process failed exit_code=$exitCode file=$FilePath args=$($ArgumentList -join ' ')"
    }
    if ($null -ne $SuccessProbe -and -not $probePassed) {
        throw "post-install detection failed file=$FilePath probe=$probeMessage"
    }
}

function ConvertTo-NativeArgument {
    param([AllowEmptyString()][string]$Value)
    # Windows command-line quoting must preserve spaces, quotes and trailing backslashes.
    return '"' + (($Value -replace '(\\*)"', '$1$1\"') -replace '(\\+)$', '$1$1') + '"'
}

function Get-ComponentRoots {
    param([string]$Directory)
    @($env:ProgramW6432, $env:ProgramFiles, ${env:ProgramFiles(x86)}) |
        Where-Object { $_ } | Select-Object -Unique |
        ForEach-Object { Join-Path $_ $Directory }
}

function Invoke-AssetInstall {
    param(
        [object]$Asset,
        [scriptblock]$SuccessProbe = $null
    )
    $path = Resolve-BundlePath -Path ([string]$Asset.path)
    $args = @()
    if ($Asset.install_args) {
        $args = @($Asset.install_args | ForEach-Object { [string]$_ })
    }
    $filePath = $path
    $argumentList = $args
    if ([System.IO.Path]::GetExtension($path).Equals(".msi", [System.StringComparison]::OrdinalIgnoreCase)) {
        $filePath = "msiexec.exe"
        $safeName = (([string]$Asset.component) -replace "[^a-zA-Z0-9_.-]", "-").ToLowerInvariant()
        $logPath = Join-Path $runtimeDir ("msi-" + $safeName + ".log")
        $argumentList = @("/i", $path) + $args + @("/L*v", $logPath)
    }
    Invoke-ProcessChecked -FilePath $filePath -ArgumentList $argumentList -Name ("install-" + [string]$Asset.component) -TimeoutSeconds 900 -SuccessProbe $SuccessProbe
}

function Find-ErlangExe {
    $roots = (@($env:ERLANG_HOME) + @(Get-ComponentRoots "Erlang OTP")) |
        Where-Object { $_ -and (Test-Path -LiteralPath $_) }
    foreach ($root in $roots) {
        $erl = Get-ChildItem -Path $root -Filter "erl.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($erl) {
            return $erl.FullName
        }
    }
    throw "erl.exe not found"
}

function Add-ErlangToProcessPath {
    $erl = Find-ErlangExe
    $erlDir = Split-Path -Parent $erl
    $parts = @($env:PATH -split ";" | Where-Object { $_ -ne "" })
    $hasPath = $false
    foreach ($part in $parts) {
        if ([string]::Equals($part.TrimEnd("\"), $erlDir.TrimEnd("\"), [System.StringComparison]::OrdinalIgnoreCase)) {
            $hasPath = $true
            break
        }
    }
    if (-not $hasPath) {
        $env:PATH = "$erlDir;$env:PATH"
    }
    [ordered]@{
        erl = $erl
        erl_dir = $erlDir
        path_contains_erlang = $true
        captured_at = (Get-Date).ToString("o")
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "erlang-process-path.json") -Encoding UTF8
    return $erl
}

function Ensure-ErlangInstalled {
    try {
        Add-ErlangToProcessPath | Tee-Object -FilePath (Join-Path $runtimeDir "erlang-path.txt") | Out-Null
        return
    } catch {
        $asset = $catalog.assets | Where-Object { $_.component -eq "erlang" } | Select-Object -First 1
        if (-not $asset) { throw "erlang asset missing in catalog" }
        Invoke-AssetInstall -Asset $asset -SuccessProbe { Find-ErlangExe | Out-Null }
        Add-ErlangToProcessPath | Tee-Object -FilePath (Join-Path $runtimeDir "erlang-path.txt") | Out-Null
    }
}

function Ensure-RabbitMQInstalled {
    try {
        Find-RabbitMQCtl | Tee-Object -FilePath (Join-Path $runtimeDir "rabbitmqctl-path.txt") | Out-Null
        return
    } catch {
        $asset = $catalog.assets | Where-Object { $_.component -eq "rabbitmq" } | Select-Object -First 1
        if (-not $asset) { throw "rabbitmq asset missing in catalog" }
        Invoke-AssetInstall -Asset $asset -SuccessProbe { Find-RabbitMQCtl | Out-Null }
        Find-RabbitMQCtl | Tee-Object -FilePath (Join-Path $runtimeDir "rabbitmqctl-path.txt") | Out-Null
    }
}

function Find-RabbitMQServiceBat {
    $roots = @(
        Get-ComponentRoots "RabbitMQ Server"
    ) | Where-Object { $_ -and (Test-Path $_) }
    foreach ($root in $roots) {
        $bat = Get-ChildItem -Path $root -Filter "rabbitmq-service.bat" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($bat) {
            return $bat.FullName
        }
    }
    throw "rabbitmq-service.bat not found after RabbitMQ install"
}

function Find-RabbitMQCtl {
    $roots = @(
        Get-ComponentRoots "RabbitMQ Server"
    ) | Where-Object { $_ -and (Test-Path $_) }
    foreach ($root in $roots) {
        $ctl = Get-ChildItem -Path $root -Filter "rabbitmqctl.bat" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($ctl) {
            return $ctl.FullName
        }
    }
    throw "rabbitmqctl.bat not found"
}

function Invoke-RabbitMQCtl {
    param(
        [string[]]$Arguments,
        [switch]$IgnoreAlreadyExists,
        [switch]$IgnoreNotFound
    )
    Add-ErlangToProcessPath | Out-Null
    $ctl = Find-RabbitMQCtl
    $output = & $ctl @Arguments 2>&1
    $exit = $LASTEXITCODE
    $text = ($output | Out-String)
    if ($exit -ne 0) {
        if ($IgnoreAlreadyExists -and ($text -match "already_exists|already exists|exists")) {
            return $text
        }
        if ($IgnoreNotFound -and ($text -match "not_found|not found|no_such|does not exist")) {
            return $text
        }
        throw "rabbitmqctl failed exit_code=$exit args=$($Arguments -join ' ') output=$text"
    }
    return $text
}

function Get-RabbitMQService {
    $service = Get-Service -Name "NodeBridgeRabbitMQ" -ErrorAction SilentlyContinue
    if ($service) {
        $script:rabbitMQServiceName = "NodeBridgeRabbitMQ"
        $script:rabbitMQServiceOwned = $true
        return $service
    }
    $service = Get-Service -Name "RabbitMQ" -ErrorAction SilentlyContinue
    if ($service) {
        $script:rabbitMQServiceName = "RabbitMQ"
        $script:rabbitMQServiceOwned = Test-RabbitMQOwnershipMarker -ServiceName "RabbitMQ"
        return $service
    }
    return $null
}

function Initialize-RabbitMQOwnershipState {
    $existing = @()
    foreach ($serviceName in @("NodeBridgeRabbitMQ", "RabbitMQ")) {
        if (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) {
            $existing += $serviceName
        }
    }
    $script:rabbitMQServiceNamesBefore = $existing
    $script:rabbitMQServiceExistedBefore = $existing.Count -gt 0
    [ordered]@{
        captured_at = (Get-Date).ToString("o")
        service_names_before = $existing
        ownership_marker = $rabbitMQOwnershipPath
        marker_exists = Test-Path -LiteralPath $rabbitMQOwnershipPath
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-service-before.json") -Encoding UTF8
}

function Test-RabbitMQOwnershipMarker {
    param([string]$ServiceName)
    if ($ServiceName -eq "NodeBridgeRabbitMQ") {
        return $true
    }
    if (-not (Test-Path -LiteralPath $rabbitMQOwnershipPath)) {
        return $false
    }
    try {
        $marker = Get-Content -LiteralPath $rabbitMQOwnershipPath -Raw | ConvertFrom-Json
        return ([string]$marker.service_name) -eq $ServiceName
    } catch {
        return $false
    }
}

function Write-RabbitMQOwnershipMarker {
    param([string]$ServiceName)
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $rabbitMQOwnershipPath) | Out-Null
    [ordered]@{
        created_at = (Get-Date).ToString("o")
        created_by = "NodeBridge headless installer"
        version = $bundleVersion
        service_name = $ServiceName
        service_existed_before = [bool]$script:rabbitMQServiceExistedBefore
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $rabbitMQOwnershipPath -Encoding UTF8
}

function Get-RabbitMQCookiePaths {
    return @(
        (Join-Path $env:USERPROFILE ".erlang.cookie"),
        (Join-Path $env:windir "System32\config\systemprofile\.erlang.cookie")
    )
}

function Sync-RabbitMQCliCookie {
    $paths = Get-RabbitMQCookiePaths
    $userCookie = $paths[0]
    $systemCookie = $paths[1]
    if (Test-Path -LiteralPath $systemCookie) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $userCookie) | Out-Null
        Copy-Item -LiteralPath $systemCookie -Destination $userCookie -Force
    } elseif ((Test-Path -LiteralPath $userCookie) -and (Test-IsAdmin)) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $systemCookie) | Out-Null
        Copy-Item -LiteralPath $userCookie -Destination $systemCookie -Force
    } else {
        return
    }
    Get-RabbitMQCookiePaths |
        ForEach-Object {
            [ordered]@{
                path = $_
                exists = Test-Path -LiteralPath $_
                sha256 = if (Test-Path -LiteralPath $_) { (Get-FileHash -Algorithm SHA256 -LiteralPath $_).Hash.ToLowerInvariant() } else { "" }
            }
        } |
        ConvertTo-Json -Depth 4 |
        Set-Content -LiteralPath (Join-Path $runtimeDir "erlang-cookie-sync.json") -Encoding UTF8
}

function Wait-RabbitMQCookie {
    param([int]$TimeoutSeconds = 60)
    $paths = Get-RabbitMQCookiePaths
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if ((Test-Path -LiteralPath $paths[0]) -or (Test-Path -LiteralPath $paths[1])) {
            Sync-RabbitMQCliCookie
            return
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    Get-RabbitMQCookiePaths |
        ForEach-Object {
            [ordered]@{
                path = $_
                exists = Test-Path -LiteralPath $_
            }
        } |
        ConvertTo-Json -Depth 4 |
        Set-Content -LiteralPath (Join-Path $runtimeDir "erlang-cookie-missing.json") -Encoding UTF8
    throw "RabbitMQ cookie did not appear within ${TimeoutSeconds}s"
}

function Wait-RabbitMQReady {
    param([int]$TimeoutSeconds = 60)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $attempts = [System.Collections.Generic.List[object]]::new()
    do {
        try {
            Add-ErlangToProcessPath | Out-Null
            Sync-RabbitMQCliCookie
            Invoke-RabbitMQCtl -Arguments @("status") | Out-Null
            $attempts.Add([ordered]@{
                time = (Get-Date).ToString("o")
                ok = $true
                message = "rabbitmqctl status passed"
            })
            $attempts | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-ready-attempts.json") -Encoding UTF8
            return
        } catch {
            $_.Exception.Message | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmqctl-status-last-error.txt") -Encoding UTF8
            $attempts.Add([ordered]@{
                time = (Get-Date).ToString("o")
                ok = $false
                message = $_.Exception.Message
            })
            $attempts | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-ready-attempts.json") -Encoding UTF8
            Start-Sleep -Seconds 2
        }
    } while ((Get-Date) -lt $deadline)
    throw "RabbitMQ did not become ready within ${TimeoutSeconds}s"
}

function Test-RabbitMQReady {
    $service = Get-RabbitMQService
    if (-not $service) {
        throw "RabbitMQ service not found"
    }
    if ($service.Status -ne "Running") {
        throw "$($service.Name) status is $($service.Status)"
    }
    Sync-RabbitMQCliCookie
    $vhosts = Invoke-RabbitMQCtl -Arguments @("list_vhosts")
    $vhosts | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-vhosts.txt") -Encoding UTF8
    $users = Invoke-RabbitMQCtl -Arguments @("list_users")
    $users | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-users.txt") -Encoding UTF8
    [ordered]@{
        name = $service.Name
        status = [string]$service.Status
        owned_by_nodebridge = [bool]$script:rabbitMQServiceOwned
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-service.json") -Encoding UTF8
}

function Ensure-RabbitMQService {
    $serviceBat = Find-RabbitMQServiceBat
    $service = Get-RabbitMQService
    if (-not $service) {
        $env:RABBITMQ_SERVICENAME = "NodeBridgeRabbitMQ"
        & $serviceBat "install"
        if ($LASTEXITCODE -ne 0) { throw "rabbitmq-service install failed with exit code $LASTEXITCODE" }
        $service = Get-Service -Name "NodeBridgeRabbitMQ" -ErrorAction Stop
        $script:rabbitMQServiceName = "NodeBridgeRabbitMQ"
        $script:rabbitMQServiceOwned = $true
        Write-RabbitMQOwnershipMarker -ServiceName "NodeBridgeRabbitMQ"
    } elseif (-not $script:rabbitMQServiceExistedBefore -and -not (Test-RabbitMQOwnershipMarker -ServiceName $service.Name)) {
        Write-RabbitMQOwnershipMarker -ServiceName $service.Name
        $script:rabbitMQServiceOwned = $true
    }
    if ($service.Status -ne "Running") {
        if ($script:rabbitMQServiceOwned) {
            $env:RABBITMQ_SERVICENAME = "NodeBridgeRabbitMQ"
            & $serviceBat "start"
            if ($LASTEXITCODE -ne 0) { throw "rabbitmq-service start failed with exit code $LASTEXITCODE" }
        } else {
            Start-Service -Name $service.Name
        }
    }
    $service = Get-RabbitMQService
    if ($service.Status -ne "Running") {
        throw "$($service.Name) status is $($service.Status)"
    }
    Wait-RabbitMQCookie -TimeoutSeconds 60
    [ordered]@{
        name = $service.Name
        status = [string]$service.Status
        start_type = [string]$service.StartType
        owned_by_nodebridge = [bool]$script:rabbitMQServiceOwned
        note = if ($script:rabbitMQServiceOwned) { "NodeBridge managed service" } else { "Existing RabbitMQ service reused; not modified or removed by NodeBridge" }
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "rabbitmq-service.json") -Encoding UTF8
}

function Test-CanalExtracted {
    $canalRoot = Join-Path $env:ProgramData "NodeBridge\canal"
    if (-not (Test-Path -LiteralPath $canalRoot)) {
        throw "Canal directory not found: $canalRoot"
    }
    $startup = Get-ChildItem -Path $canalRoot -Filter "startup.bat" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $startup) {
        throw "Canal startup.bat not found under $canalRoot"
    }
    Get-ChildItem -Path $canalRoot -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 100 FullName,Length |
        ConvertTo-Json -Depth 4 |
        Set-Content -LiteralPath (Join-Path $runtimeDir "canal-files.json") -Encoding UTF8
}

function Expand-CanalArchive {
    param(
        [string]$Source,
        [string]$Destination
    )
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    $lower = $Source.ToLowerInvariant()
    if ($lower.EndsWith(".zip")) {
        Expand-Archive -LiteralPath $Source -DestinationPath $Destination -Force
        return
    }
    if ($lower.EndsWith(".tar.gz") -or $lower.EndsWith(".tgz")) {
        & tar.exe -xzf $Source -C $Destination
        if ($LASTEXITCODE -ne 0) { throw "tar extract failed with exit code $LASTEXITCODE" }
        return
    }
    throw "unsupported Canal archive format: $Source"
}

function Ensure-CanalArchiveExtracted {
    param(
        [string]$Source,
        [string]$Destination
    )
    $service = Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq "Running") {
        Test-CanalExtracted
        Patch-CanalStartupJvmOptions -CanalRoot $Destination
        [ordered]@{
            action = "skipped"
            reason = "NodeBridgeCanal is running; existing Canal files are reused to avoid hot overwrite locks."
            source = $Source
            destination = $Destination
            service_status = [string]$service.Status
            captured_at = (Get-Date).ToString("o")
        } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "canal-asset-extract.json") -Encoding UTF8
        return
    }
    if (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Recurse -Force
    }
    Expand-CanalArchive -Source $Source -Destination $Destination
    Patch-CanalStartupJvmOptions -CanalRoot $Destination
    Test-CanalExtracted
    [ordered]@{
        action = "extracted"
        source = $Source
        destination = $Destination
        captured_at = (Get-Date).ToString("o")
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "canal-asset-extract.json") -Encoding UTF8
}

function Patch-CanalStartupJvmOptions {
    param([string]$CanalRoot)
    $files = Get-ChildItem -Path $CanalRoot -Include "startup.bat","startup.sh" -Recurse -ErrorAction SilentlyContinue
    $results = @()
    foreach ($file in $files) {
        $before = Get-Content -LiteralPath $file.FullName -Raw
        $after = $before
        foreach ($pattern in @("(?i)\s*-XX:PermSize=\S+", "(?i)\s*-XX:MaxPermSize=\S+")) {
            $after = [regex]::Replace($after, $pattern, "")
        }
        if ($after -ne $before) {
            Set-Content -LiteralPath $file.FullName -Value $after -Encoding UTF8
        }
        $results += [ordered]@{
            path = $file.FullName
            changed = [bool]($after -ne $before)
            removed_permgen_options = [bool]($before -match "(?i)-XX:(Max)?PermSize=")
        }
    }
    $results | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "canal-startup-patch.json") -Encoding UTF8
}

function Get-AssetDestination {
    param(
        [object]$Asset,
        [string]$DefaultDestination
    )
    $args = @()
    if ($Asset.install_args) {
        $args = @($Asset.install_args | ForEach-Object { [string]$_ })
    }
    for ($i = 0; $i -lt ($args.Count - 1); $i++) {
        if ($args[$i] -eq "-Destination") {
            return [Environment]::ExpandEnvironmentVariables($args[$i + 1].Replace("/", "\"))
        }
    }
    return $DefaultDestination
}

function Expand-JavaArchive {
    param(
        [string]$Source,
        [string]$Destination
    )
    if (-not (Test-Path -LiteralPath $Source)) {
        throw "Java archive not found: $Source"
    }
    if (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    $lower = $Source.ToLowerInvariant()
    if ($lower.EndsWith(".zip")) {
        Expand-Archive -LiteralPath $Source -DestinationPath $Destination -Force
    } elseif ($lower.EndsWith(".tar.gz") -or $lower.EndsWith(".tgz")) {
        & tar.exe -xzf $Source -C $Destination
        if ($LASTEXITCODE -ne 0) { throw "tar extract java failed with exit code $LASTEXITCODE" }
    } else {
        throw "unsupported Java archive format: $Source"
    }
    $java = Get-ChildItem -Path $Destination -Filter "java.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $java) {
        throw "java.exe not found after extracting Java archive to $Destination"
    }
    [ordered]@{
        source = $Source
        destination = $Destination
        java = $java.FullName
        captured_at = (Get-Date).ToString("o")
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "java-extract.json") -Encoding UTF8
    return $java.FullName
}

function Resolve-OptionalPackage {
    param([string[]]$Names)
    foreach ($name in $Names) {
        $path = Join-Path $bundleRoot ("packages/" + $name)
        if (Test-Path -LiteralPath $path) {
            return $path
        }
    }
    return ""
}

function Find-JavaExe {
    $candidates = @()
    if ($env:JAVA_HOME) {
        $candidates += (Join-Path $env:JAVA_HOME "bin\java.exe")
    }
    $cmd = Get-Command "java.exe" -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }
    $roots = @(
        Join-Path $env:ProgramData "NodeBridge\java"
        Get-ComponentRoots "Eclipse Adoptium"
        Get-ComponentRoots "Java"
        Get-ComponentRoots "Microsoft"
    ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }
    foreach ($root in $roots) {
        $found = Get-ChildItem -Path $root -Filter "java.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($found) {
            return $found.FullName
        }
    }
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) {
            return $candidate
        }
    }
    [ordered]@{
        searched_roots = $roots
        java_home = $env:JAVA_HOME
        path = $env:PATH
        captured_at = (Get-Date).ToString("o")
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $runtimeDir "java-search.json") -Encoding UTF8
    throw "java.exe not found"
}

function Ensure-JavaInstalled {
    try {
        $java = Find-JavaExe
        [ordered]@{
            java = $java
            installed = $true
            source = "existing"
            captured_at = (Get-Date).ToString("o")
        } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "java-path.json") -Encoding UTF8
        return $true
    } catch {
        $asset = $catalog.assets | Where-Object { $_.component -eq "java" -or $_.component -eq "jre" } | Select-Object -First 1
        if (-not $asset) {
            [ordered]@{
                installed = $false
                message = "java asset missing in catalog"
                captured_at = (Get-Date).ToString("o")
            } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "java-missing.json") -Encoding UTF8
            return $false
        }
        $javaAssetPath = Resolve-BundlePath -Path ([string]$asset.path)
        $lowerJavaAssetPath = $javaAssetPath.ToLowerInvariant()
        if ($lowerJavaAssetPath.EndsWith(".zip") -or $lowerJavaAssetPath.EndsWith(".tar.gz") -or $lowerJavaAssetPath.EndsWith(".tgz")) {
            $destination = Get-AssetDestination -Asset $asset -DefaultDestination (Join-Path $env:ProgramData "NodeBridge\java")
            Expand-JavaArchive -Source $javaAssetPath -Destination $destination | Out-Null
        } else {
            Invoke-AssetInstall -Asset $asset -SuccessProbe { Find-JavaExe | Out-Null }
        }
        $java = Find-JavaExe
        [ordered]@{
            java = $java
            installed = $true
            source = "asset"
            captured_at = (Get-Date).ToString("o")
        } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "java-path.json") -Encoding UTF8
        return $true
    }
}

function Write-CanalServiceEvidence {
    $service = Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue
    $cim = Get-CimInstance Win32_Service -Filter "Name='NodeBridgeCanal'" -ErrorAction SilentlyContinue
    [ordered]@{
        captured_at = (Get-Date).ToString("o")
        service = if ($service) {
            [ordered]@{
                name = $service.Name
                status = [string]$service.Status
                start_type = [string]$service.StartType
            }
        } else { $null }
        win32_service = if ($cim) {
            [ordered]@{
                name = $cim.Name
                state = $cim.State
                exit_code = $cim.ExitCode
                path_name = $cim.PathName
                start_name = $cim.StartName
            }
        } else { $null }
    } | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $runtimeDir "canal-service.json") -Encoding UTF8

    $logDir = Join-Path $env:ProgramData "NodeBridge\logs\canal"
    if (Test-Path -LiteralPath $logDir) {
        Get-ChildItem -Path $logDir -Filter "*.log" -ErrorAction SilentlyContinue |
            ForEach-Object {
                [ordered]@{
                    name = $_.Name
                    path = $_.FullName
                    length = $_.Length
                    content_tail = ((Get-Content -LiteralPath $_.FullName -Tail 200 -ErrorAction SilentlyContinue) -join [Environment]::NewLine)
                }
            } |
            ConvertTo-Json -Depth 5 |
            Set-Content -LiteralPath (Join-Path $runtimeDir "canal-service-logs.json") -Encoding UTF8
    }
}

function Wait-NodeBridgeCanalRunning {
    param([int]$TimeoutSeconds = 60)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $service = Get-Service -Name "NodeBridgeCanal" -ErrorAction Stop
        if ($service.Status -eq "Running") {
            return
        }
        if ($service.Status -eq "Stopped") {
            Write-CanalServiceEvidence
            throw "NodeBridgeCanal stopped during startup"
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    Write-CanalServiceEvidence
    throw "NodeBridgeCanal did not become Running within ${TimeoutSeconds}s"
}

function Wait-ServiceAbsent {
    param(
        [string]$Name,
        [int]$TimeoutSeconds = 30
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $attempts = @()
    do {
        $service = Get-Service -Name $Name -ErrorAction SilentlyContinue
        $cim = Get-CimInstance Win32_Service -Filter "Name='$Name'" -ErrorAction SilentlyContinue
        $attempts += [ordered]@{
            time = (Get-Date).ToString("o")
            exists = [bool]($service -or $cim)
            status = if ($service) { [string]$service.Status } else { "" }
            state = if ($cim) { [string]$cim.State } else { "" }
        }
        if (-not $service -and -not $cim) {
            $attempts | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $runtimeDir "$Name-delete-wait.json") -Encoding UTF8
            return
        }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)
    $attempts | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $runtimeDir "$Name-delete-wait.json") -Encoding UTF8
    throw "$Name still exists"
}

function Install-CanalServiceWithWinSW {
    $wrapper = Resolve-OptionalPackage -Names @("NodeBridgeCanal.exe", "WinSW-x64.exe", "winsw-x64.exe", "WinSW.exe")
    if ($wrapper -eq "") {
        if ($RequireCanalService) {
            throw "WinSW wrapper is required but no wrapper exe was found in packages/"
        }
        Add-Step -Steps $steps -Name "canal-service" -Status "skipped" -Message "WinSW wrapper missing; put WinSW-x64.exe in packages/ to register NodeBridgeCanal."
        return
    }
    $canalRoot = Join-Path $env:ProgramData "NodeBridge\canal"
    $startup = Get-ChildItem -Path $canalRoot -Filter "startup.bat" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $startup) {
        throw "Canal startup.bat not found under $canalRoot"
    }
    try {
        if (-not (Ensure-JavaInstalled)) {
            if ($RequireCanalService) {
                throw "Java runtime is required for NodeBridgeCanal service. Add a java asset to the catalog or install Java before using -RequireCanalService."
            }
            Add-Step -Steps $steps -Name "canal-service" -Status "skipped" -Message "Java runtime missing; Canal files extracted but service was not registered."
            return
        }
        $javaExe = Find-JavaExe
        [ordered]@{
            java = $javaExe
            captured_at = (Get-Date).ToString("o")
        } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "canal-java.json") -Encoding UTF8
    } catch {
        [ordered]@{
            missing = "java.exe"
            message = $_.Exception.Message
            captured_at = (Get-Date).ToString("o")
        } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir "canal-java-missing.json") -Encoding UTF8
        throw
    }
    $serviceExe = Join-Path $canalRoot "NodeBridgeCanal.exe"
    $existing = Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue
    if (-not ($existing -and $existing.Status -eq "Running" -and (Test-Path -LiteralPath $serviceExe))) {
        Copy-Item -LiteralPath $wrapper -Destination $serviceExe -Force
    }
    $xmlPath = Join-Path $canalRoot "NodeBridgeCanal.xml"
    $workDir = Split-Path -Parent (Split-Path -Parent $startup.FullName)
    $logDir = Join-Path $env:ProgramData "NodeBridge\logs\canal"
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $javaDir = Split-Path -Parent $javaExe
    $safePath = [System.Security.SecurityElement]::Escape("$javaDir;$env:PATH")
    $safeStartup = [System.Security.SecurityElement]::Escape($startup.FullName)
    $safeWorkDir = [System.Security.SecurityElement]::Escape($workDir)
    $safeLogDir = [System.Security.SecurityElement]::Escape($logDir)
    $xml = @"
<service>
  <id>NodeBridgeCanal</id>
  <name>NodeBridgeCanal</name>
  <description>NodeBridge managed Canal Server</description>
  <executable>cmd.exe</executable>
  <arguments>/c "$safeStartup"</arguments>
  <workingdirectory>$safeWorkDir</workingdirectory>
  <logpath>$safeLogDir</logpath>
  <env name="PATH" value="$safePath" />
  <stoptimeout>30 sec</stoptimeout>
  <log mode="roll-by-size">
    <sizeThreshold>10485760</sizeThreshold>
    <keepFiles>5</keepFiles>
  </log>
</service>
"@
    $xml | Set-Content -LiteralPath $xmlPath -Encoding UTF8
    $existing = Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue
    if (-not $existing) {
        & $serviceExe "install"
        if ($LASTEXITCODE -ne 0) { throw "NodeBridgeCanal service install failed with exit code $LASTEXITCODE" }
    }
    $existing = Get-Service -Name "NodeBridgeCanal" -ErrorAction Stop
    if ($existing.Status -ne "Running") {
        & $serviceExe "start"
        if ($LASTEXITCODE -ne 0) {
            Write-CanalServiceEvidence
            throw "NodeBridgeCanal service start failed with exit code $LASTEXITCODE"
        }
        Wait-NodeBridgeCanalRunning -TimeoutSeconds 60
    }
    Write-CanalServiceEvidence
}

function Uninstall-NodeBridgeResources {
    try {
        Sync-RabbitMQCliCookie
        $ownedUsers = @("nb-server-sync", "nb-edge-001", "nb-edge-001-local")
        $ownedVHosts = @("/nodebridge-edge", "/nodebridge-server")
        if (Test-Path -LiteralPath $manifestPath) {
            $ownedManifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
            $ownedUsers += @($ownedManifest.managed_components.rabbitmq.users)
            $ownedVHosts += @($ownedManifest.managed_components.rabbitmq.vhosts)
        }
        $ownedUsers | Where-Object { $_ } | Select-Object -Unique | ForEach-Object {
            Invoke-RabbitMQCtl -Arguments @("delete_user", [string]$_) -IgnoreNotFound | Out-Null
        }
        $ownedVHosts | Where-Object { $_ } | Select-Object -Unique | ForEach-Object {
            Invoke-RabbitMQCtl -Arguments @("delete_vhost", [string]$_) -IgnoreNotFound | Out-Null
        }
    } catch {
        Add-Step -Steps $steps -Name "rabbitmq-logical-cleanup" -Status "skipped" -Message $_.Exception.Message
    }

    $canalExe = Join-Path $env:ProgramData "NodeBridge\canal\NodeBridgeCanal.exe"
    if (Test-Path -LiteralPath $canalExe) {
        & $canalExe "stop" 2>$null
        & $canalExe "uninstall" 2>$null
    } elseif (Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue) {
        sc.exe stop NodeBridgeCanal | Out-Null
        sc.exe delete NodeBridgeCanal | Out-Null
    }
    Wait-ServiceAbsent -Name "NodeBridgeCanal" -TimeoutSeconds 45

    if (Get-Service -Name "NodeBridgeRabbitMQ" -ErrorAction SilentlyContinue) {
        try {
            $serviceBat = Find-RabbitMQServiceBat
            $env:RABBITMQ_SERVICENAME = "NodeBridgeRabbitMQ"
            & $serviceBat "stop" 2>$null
            & $serviceBat "remove" 2>$null
        } catch {
            if (Get-Service -Name "NodeBridgeRabbitMQ" -ErrorAction SilentlyContinue) {
                sc.exe stop NodeBridgeRabbitMQ | Out-Null
                sc.exe delete NodeBridgeRabbitMQ | Out-Null
            }
        }
        Wait-ServiceAbsent -Name "NodeBridgeRabbitMQ" -TimeoutSeconds 45
    }
    $rabbitService = Get-Service -Name "RabbitMQ" -ErrorAction SilentlyContinue
    if ($rabbitService -and (Test-RabbitMQOwnershipMarker -ServiceName "RabbitMQ")) {
        try {
            $serviceBat = Find-RabbitMQServiceBat
            $env:RABBITMQ_SERVICENAME = "RabbitMQ"
            & $serviceBat "stop" 2>$null
            & $serviceBat "remove" 2>$null
        } catch {
            if (Get-Service -Name "RabbitMQ" -ErrorAction SilentlyContinue) {
                sc.exe stop RabbitMQ | Out-Null
                sc.exe delete RabbitMQ | Out-Null
            }
        }
    }
    if (Test-Path -LiteralPath $rabbitMQOwnershipPath) {
        Remove-Item -LiteralPath $rabbitMQOwnershipPath -Force
    }

    if (Test-Path -LiteralPath $manifestPath) {
        Remove-Item -LiteralPath $manifestPath -Force
    }
    if ($RemoveData) {
        $dir = Join-Path $env:ProgramData "NodeBridge"
        if (Test-Path -LiteralPath $dir) {
            Remove-Item -LiteralPath $dir -Recurse -Force
        }
    }
}

function Verify-Uninstalled {
    foreach ($serviceName in @("NodeBridgeRabbitMQ", "NodeBridgeCanal")) {
        Wait-ServiceAbsent -Name $serviceName -TimeoutSeconds 45
    }
    if (Test-Path -LiteralPath $manifestPath) {
        throw "manifest still exists: $manifestPath"
    }
    if (Test-Path -LiteralPath $rabbitMQOwnershipPath) {
        throw "RabbitMQ ownership marker still exists: $rabbitMQOwnershipPath"
    }
    $service = Get-Service -Name "RabbitMQ" -ErrorAction SilentlyContinue
    if ($service) {
        Sync-RabbitMQCliCookie
        $vhosts = Invoke-RabbitMQCtl -Arguments @("list_vhosts")
        if ($vhosts -match "/nodebridge-edge" -or $vhosts -match "/nodebridge-server") {
            throw "NodeBridge RabbitMQ vhosts still exist"
        }
    }
}

$steps = [System.Collections.Generic.List[object]]::new()
$syncAgent = Join-Path $binDir "SyncAgent.exe"
$configPath = Join-Path $configDir "headless-installer-test.yaml"
$catalogPath = Join-Path $deployDir "nodebridge-assets.json"
if (-not (Test-Path -LiteralPath $catalogPath)) {
    $catalogPath = Join-Path $deployDir "nodebridge-assets.example.json"
}
$manifestPath = Join-Path $bundleRoot "runtime\install-manifest.json"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $manifestPath) | Out-Null
$commandPlanPath = Join-Path $runtimeDir "installer-command-plan.json"
$assetCheckPath = Join-Path $runtimeDir "installer-assets-check.json"
$managedPlanPath = Join-Path $runtimeDir "managed-plan.json"
$managedApplyPath = Join-Path $runtimeDir "managed-apply.json"

try {
    $isAdmin = Test-IsAdmin
    if (($ExecuteInstall -or $Uninstall -or $VerifyOnly) -and -not $isAdmin) {
        throw "Run install, uninstall, or verify mode from an elevated PowerShell session inside the VM."
    }
    if ($isAdmin) {
        Add-Step -Steps $steps -Name "admin-check" -Status "passed"
    } else {
        Add-Step -Steps $steps -Name "admin-check" -Status "skipped" -Message "Preflight can run without admin; -ExecuteInstall requires admin."
    }
    Invoke-Checked -Steps $steps -Name "required-files" -Block {
        foreach ($path in @($syncAgent, $configPath, $catalogPath)) {
            if (-not (Test-Path -LiteralPath $path)) {
                throw "missing required file: $path"
            }
        }
    }
    Initialize-RabbitMQOwnershipState
    if ($Uninstall) {
        Invoke-Checked -Steps $steps -Name "uninstall-nodebridge-resources" -Block {
            Uninstall-NodeBridgeResources
        }
        Invoke-Checked -Steps $steps -Name "verify-uninstall" -Block {
            Verify-Uninstalled
        }
        return
    }
    if ($VerifyOnly) {
        Invoke-Checked -Steps $steps -Name "verify-rabbitmq" -Block {
            Test-RabbitMQReady
        }
        Invoke-Checked -Steps $steps -Name "verify-canal-files" -Block {
            Test-CanalExtracted
        }
        if (Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue) {
            Invoke-Checked -Steps $steps -Name "verify-canal-service" -Block {
                $service = Get-Service -Name "NodeBridgeCanal" -ErrorAction Stop
                if ($service.Status -ne "Running") {
                    throw "NodeBridgeCanal status is $($service.Status)"
                }
            }
        } else {
            Add-Step -Steps $steps -Name "verify-canal-service" -Status "skipped" -Message "NodeBridgeCanal service not installed."
        }
        return
    }
    Invoke-Checked -Steps $steps -Name "sync-agent-ready" -Block {
        & $syncAgent "-config" $configPath | Tee-Object -FilePath (Join-Path $runtimeDir "sync-agent-ready.txt") | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "sync-agent ready failed with exit code $LASTEXITCODE" }
    }
    Invoke-Checked -Steps $steps -Name "canal-check" -Block {
        & $syncAgent "canal-check" "-config" $configPath | Tee-Object -FilePath (Join-Path $runtimeDir "canal-check.txt") | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "canal-check failed with exit code $LASTEXITCODE" }
    }
    Invoke-Checked -Steps $steps -Name "installer-command-plan" -Block {
        & $syncAgent "installer-command-plan" "-catalog" $catalogPath | Set-Content -LiteralPath $commandPlanPath -Encoding UTF8
        if ($LASTEXITCODE -ne 0) { throw "installer-command-plan failed with exit code $LASTEXITCODE" }
    }
    Invoke-Checked -Steps $steps -Name "installer-assets-check" -Block {
        $strictValue = if ($ExecuteInstall) { "true" } else { "false" }
        & $syncAgent "installer-assets-check" "-catalog" $catalogPath "-strict=$strictValue" | Set-Content -LiteralPath $assetCheckPath -Encoding UTF8
        if ($LASTEXITCODE -ne 0) { throw "installer-assets-check failed with exit code $LASTEXITCODE" }
    }
    Invoke-Checked -Steps $steps -Name "managed-plan" -Block {
        & $syncAgent "managed-plan" "-config" $configPath "-manifest" $manifestPath "-version" $bundleVersion | Set-Content -LiteralPath $managedPlanPath -Encoding UTF8
        if ($LASTEXITCODE -ne 0) { throw "managed-plan failed with exit code $LASTEXITCODE" }
    }
    if (-not $SkipManagedApply -and -not $ExecuteInstall) {
        Invoke-Checked -Steps $steps -Name "managed-apply-safe" -Block {
            & $syncAgent "managed-apply" "-config" $configPath "-manifest" $manifestPath "-version" $bundleVersion | Set-Content -LiteralPath $managedApplyPath -Encoding UTF8
            if ($LASTEXITCODE -ne 0) { throw "managed-apply failed with exit code $LASTEXITCODE" }
        }
    }
    if ($ExecuteInstall) {
        $catalog = Get-Content -LiteralPath $catalogPath -Raw | ConvertFrom-Json
        Invoke-Checked -Steps $steps -Name "install-erlang" -Block {
            Ensure-ErlangInstalled
        }
        Invoke-Checked -Steps $steps -Name "install-rabbitmq" -Block {
            Ensure-RabbitMQInstalled
        }
        Invoke-Checked -Steps $steps -Name "rabbitmq-service" -Block {
            Ensure-RabbitMQService
        }
        Invoke-Checked -Steps $steps -Name "rabbitmq-bootstrap" -Block {
            Wait-RabbitMQReady -TimeoutSeconds 180
            if (-not $SkipManagedApply) {
                & $syncAgent "managed-config-migrate" "-config" $configPath | Set-Content -LiteralPath (Join-Path $runtimeDir "managed-config-migration.json") -Encoding UTF8
                if ($LASTEXITCODE -ne 0) { throw "managed config migration failed with exit code $LASTEXITCODE" }
                & $syncAgent "managed-apply" "-config" $configPath "-manifest" $manifestPath "-version" $bundleVersion | Set-Content -LiteralPath $managedApplyPath -Encoding UTF8
                if ($LASTEXITCODE -ne 0) { throw "managed apply failed with exit code $LASTEXITCODE" }
            }
            Test-RabbitMQReady
        }
        Invoke-Checked -Steps $steps -Name "canal-asset-extract" -Block {
            $asset = $catalog.assets | Where-Object { $_.component -eq "canal" } | Select-Object -First 1
            if (-not $asset) { throw "canal asset missing in catalog" }
            $source = Resolve-BundlePath -Path ([string]$asset.path)
            $target = Join-Path $env:ProgramData "NodeBridge\canal"
            Ensure-CanalArchiveExtracted -Source $source -Destination $target
        }
        try {
            Install-CanalServiceWithWinSW
            $canalService = Get-Service -Name "NodeBridgeCanal" -ErrorAction SilentlyContinue
            if ($canalService -and $canalService.Status -eq "Running") {
                Add-Step -Steps $steps -Name "canal-service" -Status "passed"
            }
        } catch {
            Add-Step -Steps $steps -Name "canal-service" -Status "failed" -Message $_.Exception.Message
            throw
        }
    } else {
        Add-Step -Steps $steps -Name "real-install" -Status "skipped" -Message "Pass -ExecuteInstall inside the VM after replacing catalog hashes and package files. Use -Uninstall to clean NodeBridge resources."
    }
} finally {
    $summary = [ordered]@{
        created_at = (Get-Date).ToString("o")
        bundle_root = $bundleRoot
        version = $bundleVersion
        execute_install = [bool]$ExecuteInstall
        sync_agent = $syncAgent
        config = $configPath
        catalog = $catalogPath
        manifest = $manifestPath
        steps = $steps
    }
    $summary | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
    Write-Host "summary: $summaryPath"
    Pop-Location
}
