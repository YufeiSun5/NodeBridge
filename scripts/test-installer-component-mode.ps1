param()
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root 'scripts/lib/env.ps1') -RepoRoot $root
$testRoot = Join-Path $root ('.cache/component-mode/' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot -Force | Out-Null
$stub = Join-Path $testRoot 'SyncAgent.exe'
& go build -o $stub (Join-Path $root 'scripts/tests/installer-agent-stub.go')
if ($LASTEXITCODE -ne 0) { throw 'build fixture agent failed' }
$savedData = $env:ProgramData
$savedTrace = $env:NODEBRIDGE_INSTALLER_TEST_TRACE
$savedComponents = $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE
$results = @()
try {
    foreach ($case in @('default-existing','skip-existing','install-existing','default-fresh','conflicting')) {
        $caseRoot = Join-Path $testRoot $case
        $installRoot = Join-Path $caseRoot 'installed'
        $app = Join-Path $installRoot 'app'
        $headless = Join-Path $installRoot 'installer/headless/scripts'
        $env:ProgramData = Join-Path $caseRoot 'data'
        $configDir = Join-Path $env:ProgramData 'NodeBridge'
        New-Item -ItemType Directory -Path $app,$headless,$configDir -Force | Out-Null
        $env:NODEBRIDGE_INSTALLER_TEST_TRACE = Join-Path $caseRoot 'agent-calls.jsonl'
        $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE = Join-Path $caseRoot 'component-calls.txt'
        Copy-Item -LiteralPath $stub -Destination (Join-Path $app 'SyncAgent.exe')
        Copy-Item -LiteralPath $stub -Destination (Join-Path $app 'NodeBridge.exe')
        Copy-Item -LiteralPath (Join-Path $root 'configs/installer-bootstrap.yaml') -Destination (Join-Path $app 'config.yaml')
        Copy-Item -LiteralPath (Join-Path $root 'configs/installer-external-bootstrap.yaml') -Destination (Join-Path $app 'config-external.yaml')
        Copy-Item -LiteralPath (Join-Path $root 'configs/sync-rules.empty.yaml') -Destination (Join-Path $app 'sync-rules.yaml')
        Copy-Item -LiteralPath (Join-Path $root 'scripts/fixtures/installer-components-stub.ps1') -Destination (Join-Path $headless 'headless-installer-test.ps1')
        $config = Join-Path $configDir 'config.yaml'
        $rules = Join-Path $configDir 'sync-rules.yaml'
        if ($case -ne 'default-fresh') {
            "mode: server`nnode:`n  id: existing-node`nmysql:`n  database: existing_db`nrabbitmq:`n  mode: managed`nsecurity:`n  admin_password: keep-existing-password`n" | Set-Content -LiteralPath $config
            "rules: []`n# keep-existing" | Set-Content -LiteralPath $rules
        }
        $hashBefore = if (Test-Path $config) { (Get-FileHash $config).Hash } else { '' }
        $rulesBefore = if (Test-Path $rules) { (Get-FileHash $rules).Hash } else { '' }
        $arguments = @('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',(Join-Path $root 'installer/nsis/scripts/install.ps1'),'-InstallRoot',$installRoot,'-Version','component-test','-TestOnlySkipAdminCheck')
        if ($case -in @('skip-existing','conflicting')) { $arguments += '-SkipSystemComponents' }
        if ($case -in @('install-existing','conflicting')) { $arguments += '-InstallSystemComponents' }
        & powershell.exe @arguments *> (Join-Path $caseRoot 'output.txt')
        $code = $LASTEXITCODE
        if ($case -eq 'conflicting') {
            if ($code -eq 0 -or (Test-Path $env:NODEBRIDGE_INSTALLER_TEST_TRACE) -or (Test-Path (Join-Path $env:ProgramData 'NodeBridgeInstallerLogs'))) { throw 'conflicting flags did not fail before effects' }
        } else {
            if ($code -ne 0) { throw "case $case failed; see $caseRoot/output.txt" }
            $summaryFile = Get-ChildItem (Join-Path $env:ProgramData 'NodeBridgeInstallerLogs') -Recurse -Filter nsis-beta-install-summary.json | Select-Object -First 1
            $summary = Get-Content $summaryFile.FullName -Raw | ConvertFrom-Json
            $expectedMode = if ($case -eq 'install-existing') { 'install' } else { 'reuse' }
            if ($summary.status -ne 'passed' -or $summary.component_mode -ne $expectedMode) { throw "wrong summary: $case" }
            $calls = @(Get-Content $env:NODEBRIDGE_INSTALLER_TEST_TRACE | ForEach-Object { ,(ConvertFrom-Json $_) })
            $names = @($calls | ForEach-Object { $_[0] })
            if ($expectedMode -eq 'reuse') {
                if ((Test-Path $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE) -or ($names -contains 'managed-config-migrate') -or ($names -contains 'managed-apply')) { throw "reuse changed components: $case" }
                foreach ($step in @('system-components','managed-config-migration','managed-node-configuration')) {
                    if (($summary.steps | Where-Object name -eq $step).status -ne 'skipped') { throw "$case did not skip $step" }
                }
            } else {
                if (-not (Test-Path $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE) -or $names -notcontains 'managed-config-migrate' -or $names -notcontains 'managed-apply') { throw 'explicit install did not execute fixture paths' }
            }
            if ($hashBefore -and (Get-FileHash $config).Hash -ne $hashBefore) { throw "config changed: $case" }
            if ($rulesBefore -and (Get-FileHash $rules).Hash -ne $rulesBefore) { throw "rules changed: $case" }
            if ($case -eq 'default-fresh' -and (Get-FileHash $config).Hash -ne (Get-FileHash (Join-Path $app 'config-external.yaml')).Hash) { throw 'fresh reuse did not select external bootstrap' }
        }
        $results += @{case=$case;passed=$true}
        Write-Host "PASS: $case"
    }
    @{passed=$true;cases=$results;fixture_only=$true} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $testRoot 'evidence.json')
    Write-Host "evidence: $testRoot"
} finally {
    $env:ProgramData = $savedData
    $env:NODEBRIDGE_INSTALLER_TEST_TRACE = $savedTrace
    $env:NODEBRIDGE_INSTALLER_TEST_COMPONENT_TRACE = $savedComponents
}
