!include "MUI2.nsh"
!include "x64.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"

!ifndef VERSION
  !define VERSION "0.45.0"
!endif
!ifndef STAGING_DIR
  !error "STAGING_DIR is required"
!endif
!ifndef OUTPUT_EXE
  !define OUTPUT_EXE "NodeBridge-beta-${VERSION}.exe"
!endif

Name "NodeBridge Beta"
OutFile "${OUTPUT_EXE}"
InstallDir "$PROGRAMFILES64\NodeBridge"
InstallDirRegKey HKLM "Software\NodeBridge" "InstallDir"
!ifdef UPGRADE_TEST
RequestExecutionLevel user
!else
RequestExecutionLevel admin
!endif
Unicode true
SetCompressor /SOLID lzma

Var SkipSystemComponents

Function .onInit
  ${GetParameters} $R0
  ${GetOptions} $R0 "/SkipSystemComponents" $R1
  ${IfNot} ${Errors}
    StrCpy $SkipSystemComponents "1"
  ${EndIf}
!ifdef UPGRADE_TEST
  StrCpy $SkipSystemComponents "1"
!endif
FunctionEnd

!define MUI_ABORTWARNING
!define MUI_ICON "..\..\build\appicon.ico"
!define MUI_UNICON "..\..\build\appicon.ico"

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "NodeBridge Beta" SEC_MAIN
  SetShellVarContext all

  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File /oname=upgrade-preflight.ps1 "scripts\upgrade-preflight.ps1"
  ${If} ${RunningX64}
    StrCpy $1 "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  ${Else}
    MessageBox MB_ICONSTOP|MB_OK "NodeBridge requires 64-bit Windows."
    Abort
  ${EndIf}
!ifdef UPGRADE_TEST
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$PLUGINSDIR\upgrade-preflight.ps1" -InstallRoot "$INSTDIR" -TestOnlySkipAdminCheck' $0
!else
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$PLUGINSDIR\upgrade-preflight.ps1" -InstallRoot "$INSTDIR"' $0
!endif
  ${If} $0 != 0
    MessageBox MB_ICONSTOP|MB_OK "NodeBridge upgrade preflight failed. Close NodeBridge and retry."
    Abort
  ${EndIf}

  SetOutPath "$INSTDIR"
  File /r "${STAGING_DIR}\*.*"

!ifndef UPGRADE_TEST
  CreateDirectory "$SMPROGRAMS\NodeBridge"
  CreateShortCut "$SMPROGRAMS\NodeBridge\NodeBridge.lnk" "$INSTDIR\app\NodeBridge.exe" "" "$INSTDIR\app\NodeBridge.ico"
  CreateShortCut "$SMPROGRAMS\NodeBridge\Uninstall NodeBridge.lnk" "$INSTDIR\Uninstall.exe"
  CreateShortCut "$DESKTOP\NodeBridge.lnk" "$INSTDIR\app\NodeBridge.exe" "" "$INSTDIR\app\NodeBridge.ico"

  WriteUninstaller "$INSTDIR\Uninstall.exe"
  WriteRegStr HKLM "Software\NodeBridge" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "DisplayName" "NodeBridge Beta"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "Publisher" "NodeBridge"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge" "NoRepair" 1
!endif

  DetailPrint "Running NodeBridge beta installer script..."
!ifdef UPGRADE_TEST
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\install.ps1" -InstallRoot "$INSTDIR" -SkipSystemComponents -TestOnlySkipAdminCheck' $0
!else
  ${If} $SkipSystemComponents == "1"
    ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\install.ps1" -InstallRoot "$INSTDIR" -SkipSystemComponents' $0
  ${Else}
    ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\install.ps1" -InstallRoot "$INSTDIR"' $0
  ${EndIf}
!endif
  ${If} $0 != 0
    MessageBox MB_ICONSTOP|MB_OK "NodeBridge installation failed. Diagnostic logs are retained in C:\ProgramData\NodeBridgeInstallerLogs, including after uninstall."
    Abort
  ${EndIf}
SectionEnd

Section "Uninstall"
  SetShellVarContext all
  DetailPrint "Running NodeBridge beta uninstall script..."
  StrCpy $1 "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\uninstall.ps1" -InstallRoot "$INSTDIR"' $0
  ${If} $0 != 0
    MessageBox MB_ICONEXCLAMATION|MB_OK "NodeBridge managed-resource cleanup reported an error. See $INSTDIR\runtime\nsis-beta-uninstall.log and $INSTDIR\runtime\nsis-beta-uninstall-summary.json."
  ${EndIf}

  Delete "$DESKTOP\NodeBridge.lnk"
  Delete "$SMPROGRAMS\NodeBridge\NodeBridge.lnk"
  Delete "$SMPROGRAMS\NodeBridge\Uninstall NodeBridge.lnk"
  RMDir "$SMPROGRAMS\NodeBridge"

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NodeBridge"
  DeleteRegKey HKLM "Software\NodeBridge"
  RMDir /r "$INSTDIR"
SectionEnd
