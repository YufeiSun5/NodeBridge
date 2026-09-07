param(
    [string]$OfflineAssetsDir = "",
    [string]$ErlangFile = "otp_win64_27.3.4.11.exe",
    [string]$JavaFile = "OpenJDK-jre.zip",
    [string]$RabbitMQFile = "rabbitmq-server-4.1.8.exe",
    [string]$CanalFile = "canal.deployer-1.1.8.tar.gz",
    [string]$WinSWSourceFile = "WinSW-x64-v2.12.0.exe",
    [string]$WinSWFile = "WinSW-x64.exe",
    [string]$ErlangVersion = "27.3.4.11",
    [string]$JavaVersion = "temurin-17-jre",
    [string]$RabbitMQVersion = "4.1.8",
    [string]$CanalVersion = "1.1.8",
    [string]$WinSWVersion = "2.12.0",
    [switch]$NoBuild
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$version = "0.46.3"

if ($OfflineAssetsDir -eq "") {
    $OfflineAssetsDir = Join-Path $root ".cache/offline-assets"
}

$bundleRoot = Join-Path $root "build/headless-installer-test"
$zipPath = Join-Path $root "build/NodeBridge-headless-installer-test-v$version-with-assets.zip"
$packagesDir = Join-Path $bundleRoot "packages"

function Copy-RequiredAsset {
    param([string]$FileName)
    $source = Join-Path $OfflineAssetsDir $FileName
    if (-not (Test-Path -LiteralPath $source)) {
        throw "missing required offline asset: $source"
    }
    Copy-Item -LiteralPath $source -Destination (Join-Path $packagesDir $FileName) -Force
}

function Copy-OptionalAsset {
    param(
        [string]$SourceFile,
        [string]$TargetFile
    )
    $source = Join-Path $OfflineAssetsDir $SourceFile
    if (Test-Path -LiteralPath $source) {
        Copy-Item -LiteralPath $source -Destination (Join-Path $packagesDir $TargetFile) -Force
        return $true
    }
    return $false
}

$packageArgs = @("-NoZip", "-RefreshConfig")
if ($NoBuild) {
    $packageArgs += "-NoBuild"
}
& (Join-Path $root "scripts/package-headless-installer-test.ps1") @packageArgs
if ($LASTEXITCODE -ne 0) {
    throw "package-headless-installer-test failed with exit code $LASTEXITCODE"
}

New-Item -ItemType Directory -Force -Path $packagesDir | Out-Null
Copy-RequiredAsset -FileName $ErlangFile
Copy-RequiredAsset -FileName $RabbitMQFile
Copy-RequiredAsset -FileName $CanalFile
$hasJava = Copy-OptionalAsset -SourceFile $JavaFile -TargetFile $JavaFile
$hasWinSW = Copy-OptionalAsset -SourceFile $WinSWSourceFile -TargetFile $WinSWFile

$catalogArgs = @{
    BundleRoot = $bundleRoot
    ErlangFile = $ErlangFile
    RabbitMQFile = $RabbitMQFile
    CanalFile = $CanalFile
    ErlangVersion = $ErlangVersion
    RabbitMQVersion = $RabbitMQVersion
    CanalVersion = $CanalVersion
}
if ($hasJava) {
    $catalogArgs["JavaFile"] = $JavaFile
    $catalogArgs["JavaVersion"] = $JavaVersion
}
if ($hasWinSW) {
    $catalogArgs["WinSWFile"] = $WinSWFile
    $catalogArgs["WinSWVersion"] = $WinSWVersion
}
& (Join-Path $bundleRoot "scripts/prepare-installer-assets-catalog.ps1") @catalogArgs
if ($LASTEXITCODE -ne 0) {
    throw "prepare-installer-assets-catalog failed with exit code $LASTEXITCODE"
}

& (Join-Path $bundleRoot "scripts/headless-installer-test.ps1")
if ($LASTEXITCODE -ne 0) {
    throw "headless installer preflight failed with exit code $LASTEXITCODE"
}

if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}
Compress-Archive -Path (Join-Path $bundleRoot "*") -DestinationPath $zipPath -Force

$hash = Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath
Write-Host "headless installer with assets ready"
Write-Host "bundle: $bundleRoot"
Write-Host "zip: $zipPath"
Write-Host "sha256: $($hash.Hash)"
Write-Host "java_asset: $hasJava"
Write-Host "winsw_asset: $hasWinSW"
