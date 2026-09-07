# Installer VM Test Runbook

本文件给后续 `test-ai` 接手使用。所有真实 Erlang/RabbitMQ/Canal 安装测试只能在 Hyper-V 隔离 VM 内执行，不触碰宿主机组件。

## 固定资源

| Item | Value |
| --- | --- |
| VM | `NodeBridge-V034-InstallerLab-G1` |
| Clean checkpoint | `Clean-Windows-Installed` |
| VM user | `Administrator` |
| VM password | See `docs/test-credentials.md`. |
| Host workspace | `D:\DEV_D\NodeBridge` |
| Test package pattern | `build\NodeBridge-headless-installer-test-vX.Y.Z-with-assets.zip` |
| VM test root pattern | `C:\NodeBridgeRealVXYZClean` |

## 0. 选择版本

在宿主机 PowerShell 中执行，按看板版本改 `$Version`：

```powershell
$Repo = 'D:\DEV_D\NodeBridge'
$Version = '0.40.0'
$VmName = 'NodeBridge-V034-InstallerLab-G1'
$Checkpoint = 'Clean-Windows-Installed'
$VmRoot = 'C:\NodeBridgeRealV' + ($Version -replace '\.', '') + 'Clean'
$ZipName = "NodeBridge-headless-installer-test-v$Version-with-assets.zip"
$HostZip = Join-Path $Repo "build\$ZipName"
$EvidenceRoot = Join-Path $Repo ".cache\v$Version-clean-validation"
$VmPassword = '<copy from docs/test-credentials.md>'
$Password = ConvertTo-SecureString $VmPassword -AsPlainText -Force
$Cred = [pscredential]::new('Administrator', $Password)
```

确认包存在：

```powershell
Get-Item -LiteralPath $HostZip | Select-Object FullName,Length,LastWriteTime | Format-List
```

## 1. 恢复干净 VM

```powershell
Stop-VM -Name $VmName -Force -TurnOff -ErrorAction SilentlyContinue
Restore-VMSnapshot -VMName $VmName -Name $Checkpoint -Confirm:$false
Start-VM -Name $VmName
Get-VM -Name $VmName | Select-Object Name,State,Uptime,Status | Format-List
```

等待 PowerShell Direct：

```powershell
$Ready = $false
for ($i = 0; $i -lt 45; $i++) {
    try {
        Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock { 'ready' } -ErrorAction Stop | Out-Null
        $Ready = $true
        break
    } catch {
        Start-Sleep -Seconds 2
    }
}
if (-not $Ready) { throw 'PowerShell Direct not ready' }
```

确认干净度：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    $services = Get-Service | Where-Object {
        $_.Name -match 'Rabbit|NodeBridge|Canal' -or $_.DisplayName -match 'Rabbit|NodeBridge|Canal'
    }
    if ($services) {
        $services | Select-Object Name,DisplayName,Status,StartType | Format-Table -AutoSize
        throw 'VM is not clean'
    }
    foreach ($p in @('C:\Program Files\RabbitMQ Server','C:\Program Files\Erlang OTP','C:\ProgramData\NodeBridge')) {
        [pscustomobject]@{ Path = $p; Exists = Test-Path $p }
    }
}
```

## 2. 复制并解压测试包

```powershell
$Session = New-PSSession -VMName $VmName -Credential $Cred
Invoke-Command -Session $Session -ScriptBlock {
    param($VmRoot)
    New-Item -ItemType Directory -Force -Path $VmRoot | Out-Null
    Remove-Item -Path (Join-Path $VmRoot '*') -Recurse -Force -ErrorAction SilentlyContinue
} -ArgumentList $VmRoot
Copy-Item -ToSession $Session -Path $HostZip -Destination (Join-Path $VmRoot $ZipName) -Force
Invoke-Command -Session $Session -ScriptBlock {
    param($VmRoot, $ZipName)
    Expand-Archive -Path (Join-Path $VmRoot $ZipName) -DestinationPath $VmRoot -Force
    Get-Content -Path (Join-Path $VmRoot 'package-summary.json')
} -ArgumentList $VmRoot, $ZipName
Remove-PSSession $Session
```

## 3. 标准闭环测试

第一轮真实安装：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -ExecuteInstall
} -ArgumentList $VmRoot
```

验证：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -VerifyOnly
} -ArgumentList $VmRoot
```

二次安装幂等验证：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -ExecuteInstall
} -ArgumentList $VmRoot
```

卸载：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -Uninstall
} -ArgumentList $VmRoot
```

卸载后确认无服务：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    $services = Get-Service | Where-Object {
        $_.Name -match 'Rabbit|NodeBridge|Canal' -or $_.DisplayName -match 'Rabbit|NodeBridge|Canal'
    }
    if ($services) {
        $services | Select-Object Name,DisplayName,Status,StartType | Format-Table -AutoSize
        throw 'services remain after uninstall'
    }
    'NO_RABBIT_NODEBRIDGE_CANAL_SERVICES'
}
```

## 4. 强验 Canal Service

普通 `with-assets` 包可能不含 Java。缺 Java 时，默认应跳过 Canal Service 并写 `runtime\canal-java-missing.json`。

只有在包内已有 Java/JRE MSI 并且 catalog 声明 `component=java` 后才跑强验：

第一轮真实安装：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -ExecuteInstall -RequireCanalService
} -ArgumentList $VmRoot
```

验证：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -VerifyOnly
} -ArgumentList $VmRoot
```

二次安装幂等强验：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -ExecuteInstall -RequireCanalService
} -ArgumentList $VmRoot
```

卸载：

```powershell
Invoke-Command -VMName $VmName -Credential $Cred -ScriptBlock {
    param($VmRoot)
    Set-Location $VmRoot
    Set-ExecutionPolicy -Scope Process Bypass -Force
    .\scripts\headless-installer-test.ps1 -Uninstall
} -ArgumentList $VmRoot
```

## 5. 回收证据

每轮测试结束都执行。失败时也执行，保留 `runtime` 和服务/进程状态。

```powershell
New-Item -ItemType Directory -Force -Path $EvidenceRoot | Out-Null
$Session = New-PSSession -VMName $VmName -Credential $Cred
Invoke-Command -Session $Session -ScriptBlock {
    param($VmRoot)
    $out = Join-Path $VmRoot 'evidence-final'
    New-Item -ItemType Directory -Force -Path $out | Out-Null
    Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz' | Set-Content -Path (Join-Path $out 'collected-at.txt') -Encoding UTF8
    Get-Service | Where-Object {
        $_.Name -match 'Rabbit|NodeBridge|Canal' -or $_.DisplayName -match 'Rabbit|NodeBridge|Canal'
    } | Select-Object Name,DisplayName,Status,StartType |
        ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $out 'services.filtered.json') -Encoding UTF8
    Get-CimInstance Win32_Service | Where-Object {
        $_.Name -match 'Rabbit|NodeBridge|Canal' -or $_.DisplayName -match 'Rabbit|NodeBridge|Canal'
    } | Select-Object Name,DisplayName,State,StartMode,PathName,StartName,ExitCode,ServiceSpecificExitCode |
        ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $out 'services.cim.filtered.json') -Encoding UTF8
    Get-CimInstance Win32_Process | Where-Object {
        $_.Name -match 'rabbit|erl|epmd|java|NodeBridgeCanal|WinSW'
    } | Select-Object ProcessId,ParentProcessId,Name,CommandLine |
        ConvertTo-Json -Depth 5 | Set-Content -Path (Join-Path $out 'processes.filtered.json') -Encoding UTF8
    Get-ChildItem -Path (Join-Path $VmRoot 'runtime') -Force -ErrorAction SilentlyContinue |
        Select-Object Name,Length,LastWriteTime,FullName |
        ConvertTo-Json -Depth 5 | Set-Content -Path (Join-Path $out 'runtime.dir.json') -Encoding UTF8
    foreach ($p in Get-ChildItem -Path (Join-Path $VmRoot 'runtime') -File -ErrorAction SilentlyContinue) {
        Copy-Item -LiteralPath $p.FullName -Destination $out -Force
    }
    $zip = Join-Path $VmRoot 'installer-test-evidence.zip'
    Compress-Archive -Path (Join-Path $out '*') -DestinationPath $zip -Force
} -ArgumentList $VmRoot
Copy-Item -FromSession $Session -Path (Join-Path $VmRoot 'installer-test-evidence.zip') -Destination (Join-Path $EvidenceRoot 'installer-test-evidence.zip') -Force
Copy-Item -FromSession $Session -Path (Join-Path $VmRoot 'runtime\*') -Destination $EvidenceRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-PSSession $Session
Expand-Archive -Path (Join-Path $EvidenceRoot 'installer-test-evidence.zip') -DestinationPath (Join-Path $EvidenceRoot 'evidence') -Force
Get-ChildItem -Path $EvidenceRoot -Force | Select-Object Name,Length,LastWriteTime | Format-Table -AutoSize
```

## 6. 看板回写格式

通过时把 `AI_BOARD.md` 中对应项写成：

```text
<version> 已在 `Clean-Windows-Installed` 干净 VM 通过闭环：第一轮 `-ExecuteInstall` 通过，`-VerifyOnly` 通过，二次 `-ExecuteInstall` 通过，`-Uninstall` 通过且 `verify-uninstall` passed；卸载后确认无 RabbitMQ/NodeBridge/Canal 服务。证据在 `.cache/<evidence-dir>/`。
```

失败时必须写：

- 失败命令。
- 失败步骤名和原始错误。
- VM checkpoint 名称。
- 证据目录。
- 是否触碰宿主机组件，正常应写“未触碰”。

## 7. 边界

- 不在宿主机安装、停止、删除 Erlang/RabbitMQ/Canal。
- 恢复快照会丢弃 VM 当前状态；恢复前确认当前状态不需要保留。
- 普通卸载只验证 NodeBridge 管理资源和服务边界；Erlang/RabbitMQ 程序目录可能保留，这是当前 installer 边界。
- 缺 Java 时 Canal Service 默认跳过，不算失败；`-RequireCanalService` 才把缺 Java 视为失败。
