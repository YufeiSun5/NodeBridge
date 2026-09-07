# Frontend Wails Dev And Test

This note records the current Windows development path for the NodeBridge Wails UI.

## Role And Boundary

- Active identity: `frontend-ai`.
- UI scope: React, TypeScript, Wails binding wrapper, text, layout, and smoke checks.
- Boundary: do not call HTTP, RabbitMQ, or MySQL directly from the frontend. Use Wails bindings only.

## Environment

Run commands from the repository root:

```powershell
Set-Location D:\DEV_D\NodeBridge
```

Current local tools used by the frontend workflow:

| Tool | Path |
| --- | --- |
| Wails CLI | `D:\DEV_D\NodeBridge\.tools\bin\wails.exe` |
| Go | `D:\DEV_D\NodeBridge\.tools\go1.25.5\go\bin\go.exe` |
| Node | `D:\DEV_D\NodeBridge\.vfox\sdks\nodejs\node.exe` |

The local vfox Node SDK may only expose `node.exe`, so prefer direct `node` calls for TypeScript and Vite instead of assuming `npm` is available in PATH.

## Start Wails Dev

Use this exact PowerShell sequence. It is intentionally complete and does not rely on the shell already knowing `wails`, `go`, `node`, or vfox shims.

```powershell
Set-Location D:\DEV_D\NodeBridge

$root = (Get-Location).Path
$env:GOROOT = Join-Path $root '.tools\go1.25.5\go'
$env:PATH = "$root\.tools\bin;$root\.tools\go1.25.5\go\bin;$root\.vfox\sdks\nodejs;$env:PATH"

& "$root\.tools\bin\wails.exe" dev
```

One-line version for a fresh PowerShell:

```powershell
Set-Location D:\DEV_D\NodeBridge; $root = (Get-Location).Path; $env:GOROOT = Join-Path $root '.tools\go1.25.5\go'; $env:PATH = "$root\.tools\bin;$root\.tools\go1.25.5\go\bin;$root\.vfox\sdks\nodejs;$env:PATH"; & "$root\.tools\bin\wails.exe" dev
```

What each line does:

| Line | Purpose |
| --- | --- |
| `Set-Location D:\DEV_D\NodeBridge` | Move to the repository root. Wails reads `wails.json` from here. |
| `$root = (Get-Location).Path` | Store the absolute repo path so later paths are not relative guesses. |
| `$env:GOROOT = ...\.tools\go1.25.5\go` | Force Go to use the project Go tree that has a complete standard library. |
| `$env:PATH = ...` | Put project Wails, project Go, and vfox Node at the front of PATH for this terminal only. |
| `& "$root\.tools\bin\wails.exe" dev` | Start Wails dev with the project Wails CLI. |

Expected output includes:

```text
Wails CLI v2.10.2
Executing: go mod tidy
Generating bindings: Done.
Installing frontend dependencies: Done.
Compiling frontend: Done.
Vite Server URL: http://127.0.0.1:5173/
Using DevServer URL: http://localhost:<dynamic-port>
Using Frontend DevServer URL: http://127.0.0.1:5173
Serving assets from frontend DevServer URL: http://127.0.0.1:5173
WebView2 Environment created successfully
native tray icon registered
native tray ready
```

The Wails dev server port is dynamic. Use the `Using DevServer URL` value printed by Wails for browser-based Wails binding checks. Vite remains at:

```text
http://127.0.0.1:5173/
```

Stop the dev session with `Ctrl+C` in the terminal that started `wails dev`.

Do not close the PowerShell window if you still need the Wails app running. The terminal owns the dev process.

## Start Wails Dev From Any Directory

If the current shell is not in the repository, use absolute paths:

```powershell
$root = 'D:\DEV_D\NodeBridge'
Set-Location $root
$env:GOROOT = "$root\.tools\go1.25.5\go"
$env:PATH = "$root\.tools\bin;$root\.tools\go1.25.5\go\bin;$root\.vfox\sdks\nodejs;$env:PATH"
& "$root\.tools\bin\wails.exe" dev
```

This is the safest form for another AI or a clean terminal.

## Quick Tool Check

Run this when startup fails before changing code:

```powershell
$root = 'D:\DEV_D\NodeBridge'
Test-Path "$root\.tools\bin\wails.exe"
Test-Path "$root\.tools\go1.25.5\go\bin\go.exe"
Test-Path "$root\.vfox\sdks\nodejs\node.exe"
Test-Path "$root\wails.json"
Test-Path "$root\frontend\node_modules"
```

All lines should print `True`. If `frontend\node_modules` is missing, run the frontend install step from the repo's current package setup before starting Wails dev.

## Frontend Gate

Use this gate after frontend code changes:

```powershell
$root = "D:\DEV_D\NodeBridge"
$node = "$root\.vfox\sdks\nodejs\node.exe"

Set-Location $root
& $node "$root\frontend\scripts\check-wails-contract.mjs"

Push-Location "$root\frontend"
& $node "node_modules\typescript\bin\tsc"
& $node "node_modules\vite\bin\vite.js" build
Pop-Location

rg "fetch\(|axios|tailwind|bootstrap" "$root\frontend" -S
git diff --check
```

One-line frontend gate:

```powershell
$root = "D:\DEV_D\NodeBridge"; $node = "$root\.vfox\sdks\nodejs\node.exe"; Set-Location $root; & $node "$root\frontend\scripts\check-wails-contract.mjs"; Push-Location "$root\frontend"; & $node "node_modules\typescript\bin\tsc"; & $node "node_modules\vite\bin\vite.js" build; Pop-Location; rg "fetch\(|axios|tailwind|bootstrap" "$root\frontend" -S; git diff --check
```

Expected results:

- Wails contract check prints `wails contract checks passed`.
- TypeScript exits with code `0`.
- Vite build exits with code `0` and writes `frontend/dist`.
- The `rg` command returns no matches. Exit code `1` is acceptable here because it means no forbidden dependency usage was found.
- `git diff --check` has no whitespace errors. Windows CRLF conversion warnings are acceptable if no error is reported.

## Wails Smoke Checks

Run these checks in the native Wails window:

1. Start the app with `wails dev`.
2. Confirm the window opens and the terminal prints `native tray ready`.
3. Open Overview and confirm status cards render without a blank screen.
4. Open Rules and confirm existing rules render in the readonly card layout.
5. Unlock admin only when testing edit actions.
6. In Rules edit mode, confirm `SELECTED_EDGES` shows ACTIVE Edge candidates when `GetNodeOptions()` returns items, and still allows manual node ID entry when the list is empty.
7. Close the window with `X`; it should hide to the Windows tray instead of exiting.
8. Restore from the tray by left click or the tray menu.
9. Use the explicit authenticated exit flow when testing real exit behavior.

## Browser Smoke Checks

Use the Wails dev URL printed by Wails for binding-backed browser tests:

```text
http://localhost:<dynamic-port>
```

Use the Vite URL only for pure frontend layout checks:

```text
http://127.0.0.1:5173/
```

The Vite URL may not behave the same as the Wails URL for Go method calls. If a page depends on `window.go.datasyncui.App`, verify it through the Wails dev URL or the native window.

## Common Issues

- `wails` not found: use `.tools\bin\wails.exe` and inject `.tools\bin` into PATH.
- `go` standard library missing from vfox: set `GOROOT` to `.tools\go1.25.5\go`.
- `npm` not found: call `node.exe` directly with local TypeScript and Vite scripts.
- Blank page after frontend edits: run the frontend gate first, then restart `wails dev`.
- Wails binding mismatch: run `frontend\scripts\check-wails-contract.mjs` before debugging UI state.
