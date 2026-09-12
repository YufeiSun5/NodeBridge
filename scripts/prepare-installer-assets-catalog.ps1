param(
    [string]$BundleRoot = "",
    [string]$BundleVersion = "0.46.12",
    [string]$ErlangFile = "otp_win64.exe",
    [string]$JavaFile = "OpenJDK-jre.zip",
    [string]$RabbitMQFile = "rabbitmq-server.exe",
    [string]$CanalFile = "canal.deployer-1.1.8.tar.gz",
    [string]$WinSWFile = "WinSW-x64.exe",
    [string]$ErlangVersion = "27.x",
    [string]$JavaVersion = "optional",
    [string]$RabbitMQVersion = "4.x",
    [string]$CanalVersion = "1.1.8",
    [string]$WinSWVersion = "3.x"
)

$ErrorActionPreference = "Stop"

if ($BundleRoot -eq "") {
    $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $BundleRoot = Split-Path -Parent $scriptDir
}

$packagesDir = Join-Path $BundleRoot "packages"
$deployDir = Join-Path $BundleRoot "deploy/windows"
$outputPath = Join-Path $deployDir "nodebridge-assets.json"

function Get-AssetHash {
    param([string]$Path)
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function New-Asset {
    param(
        [string]$Name,
        [string]$Component,
        [string]$FileName,
        [string]$Version,
        [string[]]$InstallArgs,
        [switch]$Optional
    )
    $path = Join-Path $packagesDir $FileName
    if (-not (Test-Path -LiteralPath $path)) {
        if ($Optional) {
            return $null
        }
        throw "missing package file: $path"
    }
    return [ordered]@{
        name = $Name
        component = $Component
        path = "packages/$FileName"
        version = $Version
        sha256 = Get-AssetHash -Path $path
        install_args = $InstallArgs
    }
}

New-Item -ItemType Directory -Force -Path $deployDir | Out-Null

$assets = @()
$assets += New-Asset -Name "erlang-otp-windows-x64" -Component "erlang" -FileName $ErlangFile -Version $ErlangVersion -InstallArgs @("/S")
$javaArgs = @("/qn", "/norestart", "ADDLOCAL=FeatureMain,FeatureEnvironment")
if ($JavaFile.ToLowerInvariant().EndsWith(".zip") -or $JavaFile.ToLowerInvariant().EndsWith(".tar.gz") -or $JavaFile.ToLowerInvariant().EndsWith(".tgz")) {
    $javaArgs = @("-Destination", "%ProgramData%/NodeBridge/java")
}
$java = New-Asset -Name "java-runtime-windows-x64" -Component "java" -FileName $JavaFile -Version $JavaVersion -InstallArgs $javaArgs -Optional
if ($java) {
    $java["optional"] = $true
    $assets += $java
}
$assets += New-Asset -Name "rabbitmq-server-windows" -Component "rabbitmq" -FileName $RabbitMQFile -Version $RabbitMQVersion -InstallArgs @("/S")
$assets += New-Asset -Name "canal-server" -Component "canal" -FileName $CanalFile -Version $CanalVersion -InstallArgs @("-Destination", "%ProgramData%/NodeBridge/canal")
$winsw = New-Asset -Name "winsw-x64" -Component "winsw" -FileName $WinSWFile -Version $WinSWVersion -InstallArgs @() -Optional
if ($winsw) {
    $winsw["optional"] = $true
    $assets += $winsw
}

$catalog = [ordered]@{
    version = $BundleVersion
    assets = $assets
}

[System.IO.File]::WriteAllText($outputPath, ($catalog | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
Write-Host "catalog ready: $outputPath"
