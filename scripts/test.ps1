$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$env:PATH = "$root\.vfox\sdks\golang\bin;$root\.vfox\sdks\golang\packages\bin;$root\.vfox\sdks\nodejs;$env:PATH"

go test ./...
go vet ./...

$lint = Get-Command golangci-lint -ErrorAction SilentlyContinue
if ($lint) {
    golangci-lint run ./...
} else {
    Write-Host "golangci-lint not installed; skipped."
}
