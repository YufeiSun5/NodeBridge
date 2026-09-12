param([switch]$LegacyOnly)
$ErrorActionPreference='Stop'
function Import-Writer([string]$File) {
    $tokens=$null;$errors=$null
    $ast=[Management.Automation.Language.Parser]::ParseFile($File,[ref]$tokens,[ref]$errors)
    if($errors.Count){throw 'writer source parse failed'}
    $f=@($ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)|Where-Object Name -eq 'Write-JsonAtomic')
    if($f.Count -ne 1){throw 'writer definition missing or ambiguous'}
    return [scriptblock]::Create($f[0].Extent.Text)
}
$file=Join-Path ([IO.Path]::GetTempPath()) ('wide-atomic-'+[guid]::NewGuid().ToString('N')+'.json')
$reader=$null
try {
    . (Import-Writer (Join-Path $PSScriptRoot 'lab-real-bidirectional-overnight.ps1'))
    Write-JsonAtomic $file @{tick=1}
    $reader=[IO.File]::Open($file,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
    $legacyError=$null
    try { Write-JsonAtomic $file @{tick=2} } catch { $legacyError=$_ }
    if(-not $legacyError){throw 'legacy writer unexpectedly replaced locked target'}
    [pscustomobject]@{case='legacy_locked_reader';message=$legacyError.Exception.Message;error_id=$legacyError.FullyQualifiedErrorId}|ConvertTo-Json -Compress
    $reader.Dispose();$reader=$null
    if($LegacyOnly){return}
    . (Import-Writer (Join-Path $PSScriptRoot 'lab-real-wide-seven-hour.ps1'))
    Write-JsonAtomic $file @{tick=3}
    if((Get-Content $file -Raw|ConvertFrom-Json).tick -ne 3){throw 'replacement failed'}
    Add-Type -TypeDefinition @'
using System.IO;
using System.Threading.Tasks;
public static class WideAtomicReader {
    public static async Task ReleaseAfter(FileStream stream, int delay) {
        await Task.Delay(delay).ConfigureAwait(false);
        stream.Dispose();
    }
}
'@
    $reader=[IO.File]::Open($file,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
    $release=[WideAtomicReader]::ReleaseAfter($reader,150)
    Write-JsonAtomic $file @{tick=4}
    $null=$release.GetAwaiter().GetResult();$reader=$null
    if((Get-Content $file -Raw|ConvertFrom-Json).tick -ne 4){throw 'transient lock not recovered'}
    $reader=[IO.File]::Open($file,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
    $timer=[Diagnostics.Stopwatch]::StartNew();$failure=$null
    try { Write-JsonAtomic $file @{tick=5} } catch { $failure=$_ }
    if(-not $failure -or $timer.ElapsedMilliseconds -lt 1900){throw 'persistent lock not bounded and reported'}
    $reader.Dispose();$reader=$null
    if((Get-Content $file -Raw|ConvertFrom-Json).tick -ne 4){throw 'failed replacement damaged original'}
    if(@(Get-ChildItem -LiteralPath (Split-Path -Parent $file) -Filter "$(Split-Path -Leaf $file).$PID.*.tmp").Count){throw 'temporary replacement leaked'}
    $reader=[IO.File]::Open($file,[IO.FileMode]::Open,[IO.FileAccess]::Read,([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
    Write-JsonAtomic $file @{tick=6;text=([string][char]0x6062+[char]0x590D)}
    $reader.Dispose();$reader=$null
    $value=Get-Content $file -Raw -Encoding UTF8|ConvertFrom-Json
    if($value.tick -ne 6 -or $value.text -cne ([string][char]0x6062+[char]0x590D)){throw 'shared reader or UTF8 replacement failed'}
    Remove-Item -LiteralPath $file -Force
    Write-JsonAtomic $file @{tick=7}
    if((Get-Content $file -Raw|ConvertFrom-Json).tick -ne 7){throw 'first atomic creation failed'}
    'Atomic JSON creation, overwrite, transient/persistent locks, preservation, cleanup and shared readers passed.'
} finally {
    if($reader){$reader.Dispose()}
    foreach($p in @($file,"$file.$PID.tmp")){if(Test-Path -LiteralPath $p){Remove-Item -LiteralPath $p -Force}}
}
