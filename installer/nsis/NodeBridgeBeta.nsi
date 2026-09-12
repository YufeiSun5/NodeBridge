!include "MUI2.nsh"
!include "x64.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "nsDialogs.nsh"

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
BrandingText "NodeBridge | MySQL Synchronization"
SetFont "Segoe UI" 9
ManifestDPIAware true

Var SkipSystemComponents
Var ComponentDialog
Var ReuseComponentsRadio
Var InstallComponentsRadio
Var ComponentChoice

Function .onInit
  StrCpy $SkipSystemComponents "1"
  StrCpy $ComponentChoice ""
  ${GetParameters} $R0
  ClearErrors
  ${GetOptions} $R0 "/SkipSystemComponents" $R1
  ${IfNot} ${Errors}
    StrCpy $ComponentChoice "skip"
  ${EndIf}
  ClearErrors
  ${GetOptions} $R0 "/InstallSystemComponents" $R1
  ${IfNot} ${Errors}
    ${If} $ComponentChoice == "skip"
      MessageBox MB_ICONSTOP|MB_OK "$(ConflictingModes)" /SD IDOK
      SetErrorLevel 2
      Abort
    ${EndIf}
    StrCpy $SkipSystemComponents "0"
  ${EndIf}
!ifdef UI_TEST_LANGUAGE
  StrCpy $LANGUAGE ${UI_TEST_LANGUAGE}
!endif
FunctionEnd

!define MUI_ABORTWARNING
!define MUI_ICON "..\..\build\appicon.ico"
!define MUI_UNICON "..\..\build\appicon.ico"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "..\..\build\installer-art\header.bmp"
!define MUI_WELCOMEFINISHPAGE_BITMAP "..\..\build\installer-art\welcome.bmp"
!define MUI_WELCOMEPAGE_TITLE "NodeBridge ${VERSION}"
!define MUI_WELCOMEPAGE_TEXT "$(WelcomeText)"
!define MUI_FINISHPAGE_TITLE "$(FinishTitle)"
!define MUI_FINISHPAGE_TEXT "$(FinishText)"
!define MUI_FINISHPAGE_NOAUTOCLOSE

!insertmacro MUI_PAGE_WELCOME
Page custom ComponentModeCreate ComponentModeLeave
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Japanese"

LangString WelcomeText ${LANG_SIMPCHINESE} "安装或更新 NodeBridge。$\r$\n$\r$\n升级将保留节点身份、数据库连接与同步规则。$\r$\n$\r$\n安装期间会关闭正在运行的 NodeBridge 进程。安装完成后，会尝试在原用户桌面恢复升级前的管理窗口。"
LangString WelcomeText ${LANG_ENGLISH} "Install or update NodeBridge.$\r$\n$\r$\nUpgrades retain node identity, database connections and synchronization rules.$\r$\n$\r$\nRunning NodeBridge processes will be closed. After installation, an existing management window will be restored to its original desktop when available."
LangString WelcomeText ${LANG_JAPANESE} "NodeBridge をインストールまたは更新します。$\r$\n$\r$\n更新時はノード情報、データベース接続、同期ルールを保持します。$\r$\n$\r$\n実行中の NodeBridge を終了し、完了後に元のデスクトップで管理画面の復元を試みます。"
LangString FinishTitle ${LANG_SIMPCHINESE} "NodeBridge 安装完成"
LangString FinishTitle ${LANG_ENGLISH} "NodeBridge is installed"
LangString FinishTitle ${LANG_JAPANESE} "NodeBridge のインストールが完了しました"
LangString FinishText ${LANG_SIMPCHINESE} "可从桌面或开始菜单打开 NodeBridge。$\r$\n$\r$\n已有配置已保留。安装诊断及窗口恢复警告保存在：$\r$\nC:\ProgramData\NodeBridgeInstallerLogs"
LangString FinishText ${LANG_ENGLISH} "Open NodeBridge from the desktop or Start menu.$\r$\n$\r$\nExisting configuration is retained. Installer diagnostics and UI restoration warnings:$\r$\nC:\ProgramData\NodeBridgeInstallerLogs"
LangString FinishText ${LANG_JAPANESE} "デスクトップまたはスタートメニューから NodeBridge を開けます。$\r$\n$\r$\n既存設定は保持されます。診断と画面復元の警告：$\r$\nC:\ProgramData\NodeBridgeInstallerLogs"

LangString ComponentTitle ${LANG_SIMPCHINESE} "组件安装方式"
LangString ComponentTitle ${LANG_ENGLISH} "Component installation"
LangString ComponentTitle ${LANG_JAPANESE} "コンポーネントのインストール"
LangString ComponentSubtitle ${LANG_SIMPCHINESE} "选择复用现有环境，或安装本机系统组件。"
LangString ComponentSubtitle ${LANG_ENGLISH} "Reuse existing services or install local system components."
LangString ComponentSubtitle ${LANG_JAPANESE} "既存サービスの利用、またはローカルへの新規導入を選択します。"
LangString ReuseComponents ${LANG_SIMPCHINESE} "复用现有组件（Docker / 已有服务，默认）"
LangString ReuseComponents ${LANG_ENGLISH} "Reuse existing components (Docker / existing services, default)"
LangString ReuseComponents ${LANG_JAPANESE} "既存コンポーネントを利用（Docker / 既存サービス、既定）"
LangString ReuseDetails ${LANG_SIMPCHINESE} "仅安装或更新 NodeBridge。跳过 Erlang、RabbitMQ、Java、Canal 的安装及组件配置；保留现有连接和同步规则。"
LangString ReuseDetails ${LANG_ENGLISH} "Install or update NodeBridge only. Skip Erlang, RabbitMQ, Java, Canal and component configuration. Keep existing connections and sync rules."
LangString ReuseDetails ${LANG_JAPANESE} "NodeBridge のみ導入・更新します。Erlang、RabbitMQ、Java、Canal の導入と構成を省略し、既存の接続設定・同期ルールを保持します。"
LangString InstallComponents ${LANG_SIMPCHINESE} "安装本机系统组件（全新环境）"
LangString InstallComponents ${LANG_ENGLISH} "Install local system components (new environment)"
LangString InstallComponents ${LANG_JAPANESE} "ローカルのシステムコンポーネントを導入（新規環境）"
LangString InstallDetails ${LANG_SIMPCHINESE} "安装或复用 Windows 原生 Erlang、RabbitMQ、Java、Canal。已有 Docker 或外部服务请选择上方选项，避免端口冲突。"
LangString InstallDetails ${LANG_ENGLISH} "Install or reuse native Windows Erlang, RabbitMQ, Java and Canal. For Docker or external services, select reuse above to avoid port conflicts."
LangString InstallDetails ${LANG_JAPANESE} "Windows 版 Erlang、RabbitMQ、Java、Canal を導入・再利用します。Docker や外部サービスがある場合は上の選択肢を使用してください。"
LangString ComponentNote ${LANG_SIMPCHINESE} "MySQL 不由本安装器安装。此页为手动选择，不自动检测 Docker 或验证连接；安装后在 NodeBridge 中确认配置。"
LangString ComponentNote ${LANG_ENGLISH} "MySQL is not installed. This is a manual choice, not Docker detection or a connection test. Verify configuration in NodeBridge after installation."
LangString ComponentNote ${LANG_JAPANESE} "MySQL は導入しません。手動選択のため Docker 検出・接続確認は行いません。導入後に NodeBridge で設定を確認してください。"
LangString ConflictingModes ${LANG_SIMPCHINESE} "不能同时指定 /SkipSystemComponents 和 /InstallSystemComponents。"
LangString ConflictingModes ${LANG_ENGLISH} "Do not combine /SkipSystemComponents and /InstallSystemComponents."
LangString ConflictingModes ${LANG_JAPANESE} "/SkipSystemComponents と /InstallSystemComponents は併用できません。"

Function ComponentModeCreate
  !insertmacro MUI_HEADER_TEXT "$(ComponentTitle)" "$(ComponentSubtitle)"
  nsDialogs::Create 1018
  Pop $ComponentDialog
  ${If} $ComponentDialog == error
    Abort
  ${EndIf}
  ${NSD_CreateRadioButton} 0 0 100% 20u "$(ReuseComponents)"
  Pop $ReuseComponentsRadio
  ${NSD_CreateLabel} 12u 23u 94% 32u "$(ReuseDetails)"
  Pop $0
  ${NSD_CreateRadioButton} 0 58u 100% 20u "$(InstallComponents)"
  Pop $InstallComponentsRadio
  ${NSD_CreateLabel} 12u 81u 94% 28u "$(InstallDetails)"
  Pop $0
  ${NSD_CreateLabel} 0 115u 100% 30u "$(ComponentNote)"
  Pop $0
  ${If} $SkipSystemComponents == "1"
    ${NSD_Check} $ReuseComponentsRadio
  ${Else}
    ${NSD_Check} $InstallComponentsRadio
  ${EndIf}
  nsDialogs::Show
FunctionEnd

Function ComponentModeLeave
  ${NSD_GetState} $ReuseComponentsRadio $0
  ${If} $0 == ${BST_CHECKED}
    StrCpy $SkipSystemComponents "1"
  ${Else}
    StrCpy $SkipSystemComponents "0"
  ${EndIf}
FunctionEnd

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
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$PLUGINSDIR\upgrade-preflight.ps1" -InstallRoot "$INSTDIR" -UIStatePath "$PLUGINSDIR\ui-state.json" -TestOnlySkipAdminCheck' $0
!else
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$PLUGINSDIR\upgrade-preflight.ps1" -InstallRoot "$INSTDIR" -UIStatePath "$PLUGINSDIR\ui-state.json"' $0
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
  ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\install.ps1" -InstallRoot "$INSTDIR" -Version "${VERSION}" -UIStatePath "$PLUGINSDIR\ui-state.json" -SkipSystemComponents -TestOnlySkipAdminCheck' $0
!else
  ${If} $SkipSystemComponents == "1"
    ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\install.ps1" -InstallRoot "$INSTDIR" -Version "${VERSION}" -UIStatePath "$PLUGINSDIR\ui-state.json" -SkipSystemComponents' $0
  ${Else}
    ExecWait '"$1" -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "$INSTDIR\install.ps1" -InstallRoot "$INSTDIR" -Version "${VERSION}" -UIStatePath "$PLUGINSDIR\ui-state.json" -InstallSystemComponents' $0
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
