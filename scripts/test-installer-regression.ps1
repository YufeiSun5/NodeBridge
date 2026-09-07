param()
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$tokens = $null
$parseErrors = $null
$source = Join-Path $PSScriptRoot "headless-installer-test.ps1"
$ast = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors) { throw ($parseErrors | Out-String) }
# Load function definitions without executing component installation or cleanup.
$ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $false) |
    ForEach-Object { . ([scriptblock]::Create($_.Extent.Text)) }
$wrapperTokens = $null
$wrapperParseErrors = $null
$wrapperSource = Join-Path $repoRoot 'installer/nsis/scripts/install.ps1'
$wrapperAst = [System.Management.Automation.Language.Parser]::ParseFile($wrapperSource, [ref]$wrapperTokens, [ref]$wrapperParseErrors)
if ($wrapperParseErrors) { throw ($wrapperParseErrors | Out-String) }
$wrapperAst.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -in @('Grant-NodeBridgeDataAccess', 'Copy-DefaultConfigIfMissing') }, $false) |
    ForEach-Object { . ([scriptblock]::Create($_.Extent.Text)) }
$runtimeDir = Join-Path $repoRoot (".cache/installer-regression/" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $runtimeDir -Force | Out-Null
$passed = [System.Collections.Generic.List[string]]::new()
function Assert-True {
    param([bool]$Condition, [string]$Name)
    if (-not $Condition) { throw "FAIL: $Name" }
    $passed.Add($Name)
}
function Assert-Fails {
    param([scriptblock]$Block, [string]$Pattern)
    $message = ""
    try { & $Block } catch { $message = $_.Exception.Message }
    Assert-True ($message -like "*$Pattern*") $Pattern
}
$saved = @{}
foreach ($name in @('ProgramFiles', 'ProgramFiles(x86)', 'ProgramW6432', 'ERLANG_HOME', 'ProgramData')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:ProgramFiles = Join-Path $runtimeDir 'Program Files (x86)'
    ${env:ProgramFiles(x86)} = $env:ProgramFiles
    $env:ProgramW6432 = Join-Path $runtimeDir 'Program Files'
    $env:ERLANG_HOME = ''
    $erlPath = Join-Path $env:ProgramW6432 'Erlang OTP/bin/erl.exe'
    $rabbitPath = Join-Path $env:ProgramW6432 'RabbitMQ Server/rabbitmq_server-4.1.8/sbin/rabbitmqctl.bat'
    foreach ($path in @($erlPath, $rabbitPath)) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $path) -Force | Out-Null
        New-Item -ItemType File -Path $path -Force | Out-Null
    }
    Assert-True ((Find-ErlangExe) -eq $erlPath) 'x86 environment detects x64 Erlang'
    Assert-True ((Find-RabbitMQCtl) -eq $rabbitPath) 'x86 environment detects x64 RabbitMQ'
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}
try {
    $permissionDir = Join-Path $runtimeDir 'permission fixture'
    New-Item -ItemType Directory -Path $permissionDir -Force | Out-Null
    Grant-NodeBridgeDataAccess -Path $permissionDir | Out-Null
    $currentSid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    $modifyRule = (Get-Acl -LiteralPath $permissionDir).Access | Where-Object {
        $_.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value -eq $currentSid -and
        $_.AccessControlType -eq 'Allow' -and
        ($_.FileSystemRights -band [Security.AccessControl.FileSystemRights]::Modify)
    }
    Assert-True ($null -ne $modifyRule) 'installer grants current user modify access'
} catch {
    throw "config permission regression: $($_.Exception.Message)"
}
$defaultConfig = Join-Path $runtimeDir 'bootstrap.yaml'
$existingConfig = Join-Path $runtimeDir 'existing.yaml'
Set-Content -LiteralPath $defaultConfig -Value 'mode: ""' -Encoding UTF8
Set-Content -LiteralPath $existingConfig -Value "security:`n  admin_password: existing" -Encoding UTF8
$copyState = Copy-DefaultConfigIfMissing -Source $defaultConfig -Target $existingConfig
Assert-True ($copyState -eq 'kept-existing') 'incomplete existing config is preserved'
Assert-True ((Get-Content -LiteralPath $existingConfig -Raw) -match 'existing') 'existing config content is unchanged'
$powershell = Join-Path $PSHOME 'powershell.exe'
if (-not (Test-Path -LiteralPath $powershell)) { $powershell = Join-Path $PSHOME 'pwsh.exe' }
$fixtureDir = Join-Path $runtimeDir 'space path'
New-Item -ItemType Directory -Path $fixtureDir | Out-Null
$fixture = Join-Path $fixtureDir 'native fixture.ps1'
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'tests/native-process-fixture.ps1') -Destination $fixture
$outputPath = Join-Path $fixtureDir 'argument output.json'
$value = 'path with spaces\and "quotes"\'
$nativeArgs = @('-NoProfile', '-File', $fixture, '-OutputPath', $outputPath, '-Value', $value)
Invoke-ProcessChecked -FilePath $powershell -ArgumentList $nativeArgs -Name 'quoted'
Assert-True (((Get-Content -LiteralPath $outputPath -Raw | ConvertFrom-Json).value) -eq $value) 'native argument roundtrip'
Assert-True ((Get-Content (Join-Path $runtimeDir 'process-quoted.json') -Raw | ConvertFrom-Json).exit_code -eq 0) 'real exit code retained'
Assert-Fails { Invoke-ProcessChecked -FilePath $powershell -ArgumentList ($nativeArgs + @('-Code', '7')) -Name 'nonzero' -SuccessProbe { } } 'exit_code=7'
Assert-Fails { Invoke-ProcessChecked -FilePath $powershell -ArgumentList $nativeArgs -Name 'probe-failed' -SuccessProbe { throw 'not ready' } -ProbeTimeoutSeconds 0 } 'post-install detection failed'
Assert-Fails { Invoke-ProcessChecked -FilePath $powershell -ArgumentList ($nativeArgs + @('-DelaySeconds', '10')) -Name 'timeout' -TimeoutSeconds 1 } 'process timed out'
# Simulate the Windows PowerShell missing-ExitCode behavior deterministically.
function Start-Process {
    param($FilePath, $ArgumentList, [switch]$PassThru, $WindowStyle)
    $mock = [pscustomobject]@{ Handle = 1; Id = 123; ExitCode = $null }
    $mock | Add-Member ScriptMethod WaitForExit { param($Milliseconds) return $true }
    $mock | Add-Member ScriptMethod Dispose { }
    return $mock
}
try {
    Invoke-ProcessChecked -FilePath 'mock' -ArgumentList @('/S') -Name 'missing-detected' -SuccessProbe { }
    Assert-True ((Get-Content (Join-Path $runtimeDir 'process-missing-detected.json') -Raw | ConvertFrom-Json).exit_code_missing_but_detected) 'missing exit code requires detection'
    Assert-Fails { Invoke-ProcessChecked -FilePath 'mock' -ArgumentList @('/S') -Name 'missing-undetected' -SuccessProbe { throw 'missing' } -ProbeTimeoutSeconds 0 } 'missing exit_code'
} finally { Remove-Item Function:\Start-Process }
$installRoot = Join-Path $runtimeDir 'incomplete install'
New-Item -ItemType Directory -Path $installRoot | Out-Null
try {
    $env:ProgramData = Join-Path $runtimeDir 'data'
    $wrapper = Join-Path $repoRoot 'installer/nsis/scripts/install.ps1'
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    & $powershell -NoProfile -File $wrapper -InstallRoot $installRoot *> (Join-Path $runtimeDir 'wrapper-output.txt')
    $wrapperExit = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference
    Assert-True ($wrapperExit -ne 0) 'incomplete install fails'
    $logsRoot = Join-Path $env:ProgramData 'NodeBridgeInstallerLogs'
    $summaryFile = Get-ChildItem -LiteralPath $logsRoot -Recurse -Filter 'nsis-beta-install-summary.json' | Select-Object -First 1
    Assert-True ($null -ne $summaryFile) 'failure summary stored outside install directory'
    Assert-True ((Get-Content -LiteralPath $summaryFile.FullName -Raw | ConvertFrom-Json).status -eq 'failed') 'failure summary status retained'
} finally {
    $ErrorActionPreference = 'Stop'
    $env:ProgramData = $saved['ProgramData']
}
@{ passed = $passed; is_64_bit_process = [Environment]::Is64BitProcess } |
    ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $runtimeDir 'regression-summary.json') -Encoding UTF8
Write-Host "PASS: $($passed.Count) installer regressions; evidence: $runtimeDir"
