param(
    [string]$OutputPath,
    [string]$Value,
    [int]$Code = 0,
    [int]$DelaySeconds = 0
)
if ($DelaySeconds) { Start-Sleep -Seconds $DelaySeconds }
if ($OutputPath) { @{ value = $Value } | ConvertTo-Json | Set-Content -LiteralPath $OutputPath -Encoding UTF8 }
exit $Code
