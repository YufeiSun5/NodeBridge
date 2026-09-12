param(
    [string]$Version='0.46.13',
    [string]$DateStamp='20260910',
    [Parameter(Mandatory)][string]$ExtractedPath,
    [Parameter(Mandatory)][string]$UpgradeEvidence,
    [Parameter(Mandatory)][string]$Regression64,
    [Parameter(Mandatory)][string]$Regression32,
    [string]$EvidenceDirectory = '.cache\wide-seven-hour'
)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$stage=Join-Path $root "build\nsis-beta\NodeBridge-beta-v$Version-$DateStamp"
$exe=Join-Path $root "build\NodeBridge-beta-v$Version-$DateStamp.exe"
$checked=@()
foreach($file in Get-ChildItem -LiteralPath $stage -File -Recurse){
    $relative=$file.FullName.Substring($stage.Length+1)
    $actual=Join-Path $ExtractedPath $relative
    if(-not (Test-Path -LiteralPath $actual)){throw "extracted file missing: $relative"}
    $hash=(Get-FileHash -LiteralPath $file.FullName).Hash
    if((Get-FileHash -LiteralPath $actual).Hash -ne $hash){throw "extracted hash mismatch: $relative"}
    $checked+=@{path=$relative;sha256=$hash}
}
$headless=Join-Path $stage 'installer\headless'
$catalog=Get-Content (Join-Path $headless 'deploy\windows\nodebridge-assets.json') -Raw | ConvertFrom-Json
if($catalog.version -ne $Version -or $catalog.assets.Count -ne 5){throw 'asset catalog version/count mismatch'}
foreach($asset in $catalog.assets){if((Get-FileHash (Join-Path $headless $asset.path)).Hash -ne $asset.sha256){throw "asset checksum mismatch: $($asset.name)"}}
$preflight=Get-Content (Join-Path $root 'build\headless-installer-test\runtime\headless-installer-summary.json') -Raw | ConvertFrom-Json
if($preflight.version -ne $Version -or $preflight.execute_install -or @($preflight.steps|Where-Object status -eq 'failed').Count){throw 'headless preflight failed'}
$upgrade=Get-Content $UpgradeEvidence -Raw | ConvertFrom-Json
if($upgrade.status -ne 'passed' -or $upgrade.version -ne $Version -or -not $upgrade.running_agent_stopped -or -not $upgrade.binaries_overwritten -or -not $upgrade.existing_config_preserved -or -not $upgrade.existing_rules_preserved){throw 'isolated upgrade gate failed'}
$r64=Get-Content $Regression64 -Raw | ConvertFrom-Json;$r32=Get-Content $Regression32 -Raw | ConvertFrom-Json
if(-not $r64.is_64_bit_process -or $r32.is_64_bit_process -or $r64.passed.Count -ne 15 -or $r32.passed.Count -ne 15){throw 'installer bitness regression gate failed'}
$agentHash=(Get-FileHash (Join-Path $stage 'app\SyncAgent.exe')).Hash
if($upgrade.agent_sha256 -ne $agentHash -or $upgrade.ui_sha256 -ne (Get-FileHash (Join-Path $stage 'app\NodeBridge.exe')).Hash){throw 'upgrade evidence belongs to different binaries'}
foreach($scope in @('edge','server')){if($upgrade.migration_sha256.$scope -ne (Get-FileHash (Join-Path $stage "app\migrations\$scope\001_mvp_tables.sql")).Hash){throw "upgrade evidence has wrong $scope schema"}}
$evidenceRoot=Join-Path $root $EvidenceDirectory
$mcp=Get-Content (Join-Path $evidenceRoot 'package-mcp-version.json') -Raw | ConvertFrom-Json
if(-not $mcp.passed -or $mcp.agent_sha256 -ne $agentHash -or $mcp.initialize.serverInfo.version -ne $Version -or $mcp.overview_version -ne $Version -or $mcp.tools -ne 39 -or $mcp.diagnostics.status -ne 'ok'){throw 'MCP/UI package version gate failed'}
$report=@{passed=$true;at=(Get-Date).ToString('o');version=$Version;agent_sha256=$agentHash;ui_sha256=(Get-FileHash (Join-Path $stage 'app\NodeBridge.exe')).Hash;installer_path=$exe;installer_sha256=(Get-FileHash $exe).Hash;installer_bytes=(Get-Item $exe).Length;signature=(Get-AuthenticodeSignature $exe).Status.ToString();extracted_files=$checked;assets_verified=5;headless_preflight=$true;upgrade_evidence=$UpgradeEvidence;regressions=@($Regression64,$Regression32);mcp_evidence=$mcp}
$path=Join-Path $evidenceRoot 'package-gates.json'
[IO.File]::WriteAllText($path,($report|ConvertTo-Json -Depth 12),[Text.UTF8Encoding]::new($false))
"Package artifacts passed: $($checked.Count) extracted files, 5 assets, two upgrade installs, 32/64-bit regressions, MCP/UI $Version"
