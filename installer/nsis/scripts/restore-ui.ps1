function Get-NodeBridgeUIRestoreTargets {
    param([string]$InstallRoot, [object]$State)
    $root = [IO.Path]::GetFullPath($InstallRoot).TrimEnd('\')
    if (-not [string]::Equals($root, [IO.Path]::GetFullPath([string]$State.install_root).TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
        throw "UI restore state belongs to a different install root."
    }
    $seen = @{}
    foreach ($session in @($State.sessions)) {
        $id = [int]$session.session_id
        $sid = [string]$session.user_sid
        if ($id -le 0 -or $sid -notmatch '^S-1-5-\d+(?:-\d+)+$') { throw "Invalid interactive UI restore target." }
        $key = "$id/$sid"
        if (-not $seen.ContainsKey($key)) {
            $seen[$key] = $true
            [pscustomobject]@{ SessionId = $id; UserSid = $sid }
        }
    }
}

function Restore-NodeBridgeUI {
    param([string]$InstallRoot, [string]$StatePath)
    if ($StatePath -eq "" -or -not (Test-Path -LiteralPath $StatePath)) { return "No prior UI to restore." }
    $state = Get-Content -LiteralPath $StatePath -Raw | ConvertFrom-Json
    $targets = @(Get-NodeBridgeUIRestoreTargets -InstallRoot $InstallRoot -State $state)
    $exe = [IO.Path]::GetFullPath((Join-Path $InstallRoot 'app\NodeBridge.exe'))
    if ($targets.Count -eq 0) { return "No prior interactive UI to restore." }
    if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { throw "Installed UI executable is missing." }
    foreach ($target in $targets) {
        # InteractiveToken must resolve to exactly the original user's session.
        $desktops = @(Get-CimInstance Win32_Process -Filter "Name='explorer.exe'" | Where-Object {
            $owner = Invoke-CimMethod -InputObject $_ -MethodName GetOwnerSid
            $owner.ReturnValue -eq 0 -and $owner.Sid -eq $target.UserSid
        } | Select-Object -ExpandProperty SessionId -Unique)
        if ($desktops.Count -ne 1 -or [int]$desktops[0] -ne $target.SessionId) { throw "Original interactive desktop is unavailable or ambiguous; UI was not launched." }
        $existing = @(Get-CimInstance Win32_Process -Filter "Name='NodeBridge.exe'" | Where-Object {
            $_.SessionId -eq $target.SessionId -and [string]::Equals([string]$_.ExecutablePath, $exe, [StringComparison]::OrdinalIgnoreCase)
        })
        if ($existing.Count -gt 0) { continue }
        $taskName = 'NodeBridge-UpgradeUI-' + [guid]::NewGuid().ToString('N')
        try {
            $action = New-ScheduledTaskAction -Execute $exe -WorkingDirectory (Split-Path -Parent $exe)
            $principal = New-ScheduledTaskPrincipal -UserId $target.UserSid -LogonType Interactive -RunLevel Limited
            $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit ([TimeSpan]::Zero)
            Register-ScheduledTask -TaskName $taskName -Action $action -Principal $principal -Settings $settings -Force | Out-Null
            Start-ScheduledTask -TaskName $taskName
            $deadline = [DateTime]::UtcNow.AddSeconds(20)
            do {
                Start-Sleep -Milliseconds 250
                $running = @(Get-CimInstance Win32_Process -Filter "Name='NodeBridge.exe'" | Where-Object {
                    $_.SessionId -eq $target.SessionId -and [string]::Equals([string]$_.ExecutablePath, $exe, [StringComparison]::OrdinalIgnoreCase)
                })
            } while ($running.Count -eq 0 -and [DateTime]::UtcNow -lt $deadline)
            if ($running.Count -eq 0) { throw "UI did not start in its original interactive session within 20 seconds." }
        } finally {
            Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
        }
    }
    return "Restored UI in $($targets.Count) original interactive session(s)."
}
