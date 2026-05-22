param(
    [string]$RepoRoot
)

if (-not $RepoRoot) {
    $RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
}

function Add-NodeBridgePath {
    param([string]$Path)
    if ($Path -and (Test-Path $Path)) {
        $env:PATH = "$Path;$env:PATH"
    }
}

$toolGo = Join-Path $RepoRoot ".tools/go1.25.5/go/bin/go.exe"
$vfoxGo = Join-Path $RepoRoot ".vfox/sdks/golang/bin/go.exe"
if (Test-Path $toolGo) {
    $goRoot = Join-Path $RepoRoot ".tools/go1.25.5/go"
    $env:GOROOT = $goRoot
    Add-NodeBridgePath (Split-Path -Parent $toolGo)
} elseif (Test-Path $vfoxGo) {
    Add-NodeBridgePath (Split-Path -Parent $vfoxGo)
} elseif (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "go command not found; install Go or restore .tools/go1.25.5/go."
}

$goCacheRoot = Join-Path $RepoRoot ".cache/go"
New-Item -ItemType Directory -Force -Path (Join-Path $goCacheRoot "build") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $goCacheRoot "mod") | Out-Null
$env:GOCACHE = Join-Path $goCacheRoot "build"
$env:GOMODCACHE = Join-Path $goCacheRoot "mod"

Add-NodeBridgePath (Join-Path $RepoRoot ".vfox/sdks/golang/packages/bin")
Add-NodeBridgePath (Join-Path $RepoRoot ".vfox/sdks/nodejs")
Add-NodeBridgePath "C:\Program Files\Docker\Docker\resources\bin"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Host "docker command not found; Docker-based lab scripts will fail until Docker CLI is available."
}
