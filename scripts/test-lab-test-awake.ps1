$ErrorActionPreference='Stop'
$tokens=$null;$errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'lab-test-awake.ps1'),[ref]$tokens,[ref]$errors)
if($errors.Count){throw ($errors|Out-String)}
$f=$ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)|Where-Object Name -eq 'Test-RunRules'
. ([scriptblock]::Create($f.Extent.Text))
$RulesPath=Join-Path ([IO.Path]::GetTempPath()) ('awake-rules-'+[guid]::NewGuid().ToString('N')+'.yaml')
$RunId='wide_test_123'
try {
    foreach($case in @(
        @{text="    - id: wide_test_123-e-stream`r`n";expected=$true},
        @{text="    - id: other_wide_test_123-e-stream`r`n";expected=$false},
        @{text="    - id: wide_test_123-e-stream_extra`r`n";expected=$false},
        @{text="    - id: original-rule`r`n";expected=$false}
    )) {
        [IO.File]::WriteAllText($RulesPath,$case.text,[Text.UTF8Encoding]::new($false))
        if((Test-RunRules) -ne $case.expected){throw 'awake lease rule ownership mismatch'}
    }
} finally {Remove-Item -LiteralPath $RulesPath -Force}
'Wake lease parser and exact rule ownership passed'
