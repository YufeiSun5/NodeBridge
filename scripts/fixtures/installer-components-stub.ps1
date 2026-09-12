$ErrorActionPreference = 'Stop'
if (-not $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE) { throw 'fixture trace path required' }
'fixture-only: components selected' | Set-Content -LiteralPath $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE
