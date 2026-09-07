param(
    [double]$MinCoverage = 70.0,
    [string]$OutDir = ".cache"
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$GoExe = (Get-Command go -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source)
if (-not $GoExe) {
    $goCandidates = @(
        (Join-Path $Root ".tools/go1.25.5/go/bin/go.exe"),
        "C:\Program Files\Go\bin\go.exe"
    )
    $GoExe = $goCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if (-not $GoExe) {
    throw "go CLI not found"
}

$GoRoot = Split-Path -Parent (Split-Path -Parent $GoExe)
$env:GOROOT = $GoRoot
$env:GOTOOLCHAIN = "local"

$outPath = if ([IO.Path]::IsPathRooted($OutDir)) { $OutDir } else { Join-Path $Root $OutDir }
New-Item -ItemType Directory -Force -Path $outPath | Out-Null
$profile = Join-Path $outPath "coverage-core.out"
$summary = Join-Path $outPath "coverage-core-summary.txt"
$result = Join-Path $outPath "coverage-core-result.json"

$packages = @(
    "./internal/apply",
    "./internal/cdc",
    "./internal/cdc/canal",
    "./internal/diagnostic",
    "./internal/installer/assets",
    "./internal/installer/canal",
    "./internal/installer/manifest",
    "./internal/installer/rabbitmq",
    "./internal/loop",
    "./internal/mapper",
    "./internal/mcpstdio",
    "./internal/mysqlconn",
    "./internal/normalizer",
    "./internal/rabbitmq",
    "./internal/rules",
    "./internal/status",
    "./internal/syncruntime",
    "./internal/syncstore",
    "./internal/uiapi"
)

Push-Location $Root
try {
    & $GoExe test @packages -coverprofile $profile
    if ($LASTEXITCODE -ne 0) {
        throw "go test coverage command failed with exit code $LASTEXITCODE"
    }
    $coverageOutput = & $GoExe tool cover -func $profile
    $coverageOutput | Set-Content -Path $summary -Encoding UTF8
    $totalLine = $coverageOutput | Where-Object { $_ -match "^total:" } | Select-Object -Last 1
    if (-not $totalLine -or $totalLine -notmatch "([0-9]+(?:\.[0-9]+)?)%") {
        throw "failed to parse total coverage"
    }
    $coverage = [double]$Matches[1]
    [IO.File]::WriteAllText(
        $result,
        ([pscustomobject]@{
            status = if ($coverage -ge $MinCoverage) { "passed" } else { "failed" }
            total_coverage_percent = $coverage
            min_coverage_percent = $MinCoverage
            profile = $profile
            summary = $summary
            packages = $packages
            completed_at = (Get-Date).ToString("o")
        } | ConvertTo-Json -Depth 5),
        [Text.UTF8Encoding]::new($false)
    )
    Write-Host ("core coverage={0:0.0}% min={1:0.0}%" -f $coverage, $MinCoverage)
    if ($coverage -lt $MinCoverage) {
        throw "core coverage below threshold"
    }
} finally {
    Pop-Location
}
