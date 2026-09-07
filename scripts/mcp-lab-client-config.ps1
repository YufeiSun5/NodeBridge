param(
    [Parameter(Mandatory = $true)][string]$SshHost,
    [int]$SshPort = 22,
    [string]$ClientKeyPath = "",
    [string]$ConfigPath = "$env:ProgramData\NodeBridge\config.yaml",
    [string]$OutputPath = "$env:USERPROFILE\Desktop\nodebridge-mcp-lab.json"
)

$ErrorActionPreference = "Stop"
$agent = Join-Path $PSScriptRoot "SyncAgent.exe"
if (-not (Test-Path -LiteralPath $agent)) {
    $agent = Join-Path (Split-Path -Parent $PSScriptRoot) "build\bin\SyncAgent.exe"
}
if (-not (Test-Path -LiteralPath $agent)) { throw "SyncAgent.exe not found beside script" }
$mcpArguments = @("mcp-client-config", "-lab-full-access", "-ssh-host", $SshHost, "-ssh-port", "$SshPort", "-exe", $agent, "-config", $ConfigPath)
if ($ClientKeyPath -ne "") { $mcpArguments += @("-ssh-key", $ClientKeyPath) }
$json = & $agent @mcpArguments
if ($LASTEXITCODE -ne 0) { throw "MCP client configuration generation failed" }
$null = ($json -join "`n") | ConvertFrom-Json
[System.IO.File]::WriteAllText($OutputPath, ($json -join "`n"), [System.Text.UTF8Encoding]::new($false))
Write-Output "MCP lab client config: $OutputPath"
