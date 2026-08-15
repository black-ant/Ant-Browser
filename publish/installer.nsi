; ZwBrowser NSIS Installer Script
; Usage: makensis /DVERSION=1.4.0 /DSTAGINGDIR=C:\path\to\staging installer.nsi

Unicode True

!ifndef VERSION
  !define VERSION "1.4.0"
!endif
!ifndef STAGINGDIR
  !define STAGINGDIR "..\publish\staging"
!endif

!define PRODUCT_NAME    "ZwBrowser"
!define PRODUCT_EXE     "ant-chrome.exe"
!define PRODUCT_ICON    "AntBrowser.ico"
!define UNINSTALL_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\AntBrowser"
!define INSTALL_DIR     "$PROGRAMFILES64\ZwBrowser"
!define POWERSHELL_EXE  "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"

!include "MUI2.nsh"
!include "LogicLib.nsh"

!macro WriteCloseProcessScript HANDLE
  FileWrite ${HANDLE} "param([string]$$InstallDir, [string]$$ExcludePath)$\r$\n"
  FileWrite ${HANDLE} "$$ErrorActionPreference = 'SilentlyContinue'$\r$\n"
  FileWrite ${HANDLE} "if ([string]::IsNullOrWhiteSpace($$InstallDir) -or -not (Test-Path -LiteralPath $$InstallDir)) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$root = [System.IO.Path]::GetFullPath($$InstallDir).TrimEnd('\') + '\'$\r$\n"
  FileWrite ${HANDLE} "$$exclude = ''$\r$\n"
  FileWrite ${HANDLE} "if (-not [string]::IsNullOrWhiteSpace($$ExcludePath)) { $$exclude = [System.IO.Path]::GetFullPath($$ExcludePath) }$\r$\n"
  FileWrite ${HANDLE} "function Get-AntBrowserProcesses {$\r$\n"
  FileWrite ${HANDLE} "  @(Get-CimInstance Win32_Process | Where-Object {$\r$\n"
  FileWrite ${HANDLE} "    $$_.ExecutablePath -and $$_.ExecutablePath.StartsWith($$root, [System.StringComparison]::OrdinalIgnoreCase) -and ($$exclude -eq '' -or -not $$_.ExecutablePath.Equals($$exclude, [System.StringComparison]::OrdinalIgnoreCase))$\r$\n"
  FileWrite ${HANDLE} "  })$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$deadline = (Get-Date).AddSeconds(10)$\r$\n"
  FileWrite ${HANDLE} "do {$\r$\n"
  FileWrite ${HANDLE} "  $$procs = Get-AntBrowserProcesses$\r$\n"
  FileWrite ${HANDLE} "  if (-not $$procs -or $$procs.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "  foreach ($$p in $$procs) { try { Stop-Process -Id $$p.ProcessId -Force -ErrorAction Stop } catch {} }$\r$\n"
  FileWrite ${HANDLE} "  Start-Sleep -Milliseconds 400$\r$\n"
  FileWrite ${HANDLE} "} while ((Get-Date) -lt $$deadline)$\r$\n"
  FileWrite ${HANDLE} "$$left = Get-AntBrowserProcesses$\r$\n"
  FileWrite ${HANDLE} "if ($$left -and $$left.Count -gt 0) {$\r$\n"
  FileWrite ${HANDLE} "  $$names = ($$left | ForEach-Object { $$_.Name + '#' + $$_.ProcessId }) -join ', '$\r$\n"
  FileWrite ${HANDLE} "  Write-Host ('still running: ' + $$names)$\r$\n"
  FileWrite ${HANDLE} "  exit 1$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "exit 0$\r$\n"
!macroend

Function CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "正在关闭安装目录中的残留进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath ""' $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到旧版本仍有进程占用安装目录。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“重试”将再次尝试关闭残留进程，点击“取消”将终止本次安装。" IDRETRY retry_powershell IDCANCEL install_abort

install_abort:
  Abort "安装已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  DetailPrint "PowerShell 不可用，回退到 taskkill 清理主进程和代理进程..."
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM ${PRODUCT_EXE}' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe' $2
  Sleep 1500

done:
FunctionEnd

Function un.CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "正在关闭安装目录中的残留进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "$INSTDIR\Uninstall.exe"' $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到安装目录中仍有旧进程占用文件。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“重试”将再次尝试关闭残留进程，点击“取消”将终止本次卸载。" IDRETRY retry_powershell IDCANCEL uninstall_abort

uninstall_abort:
  Abort "卸载已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  DetailPrint "PowerShell 不可用，回退到 taskkill 清理主进程和代理进程..."
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM ${PRODUCT_EXE}' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe' $2
  Sleep 1500

done:
FunctionEnd

Function WarnInstallDir
  StrCpy $0 "$INSTDIR"

  StrCpy $1 "$0" 2
  ${If} "$1" == "C:"
    Goto warn_dir
  ${EndIf}
  ${If} "$1" == "c:"
    Goto warn_dir
  ${EndIf}

  StrLen $1 "$PROGRAMFILES64"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES64"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  StrLen $1 "$PROGRAMFILES"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  StrLen $1 "$PROGRAMFILES32"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES32"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  Return

warn_dir:
  MessageBox MB_ICONEXCLAMATION|MB_OKCANCEL|MB_DEFBUTTON2 "不建议安装到 C 盘、Program Files 或其他受权限保护的目录。$\r$\n$\r$\nZwBrowser 会在安装目录写入 data、浏览器实例和运行时文件；普通权限运行时可能写入失败，导致再次启动闪退或浏览器实例启动失败。$\r$\n$\r$\n建议改为非 C 盘可写目录，例如 D:\software\ZwBrowser 或 E:\software\ZwBrowser。$\r$\n$\r$\n点击“确定”继续安装到当前目录，点击“取消”返回修改安装路径。" IDOK continue_install IDCANCEL cancel_install

continue_install:
  Return

cancel_install:
  Abort
FunctionEnd

Name "${PRODUCT_NAME} ${VERSION}"
OutFile "..\publish\output\AntBrowser-Setup-${VERSION}.exe"
InstallDir "${INSTALL_DIR}"
InstallDirRegKey HKLM "${UNINSTALL_KEY}" "InstallLocation"
RequestExecutionLevel admin
!ifdef BESTCOMPRESSION
  SetCompressor /SOLID lzma
!else
  SetCompressor lzma
!endif

!define MUI_ICON "..\build\windows\icon.ico"
!define MUI_UNICON "..\build\windows\icon.ico"

!insertmacro MUI_PAGE_WELCOME
!define MUI_DIRECTORYPAGE_TEXT_TOP "请选择 ZwBrowser 的安装目录。建议安装到非 C 盘的可写目录，例如 D:\software\ZwBrowser 或 E:\software\ZwBrowser；不要安装到 C 盘、Program Files 或其他受权限保护的目录。"
!define MUI_DIRECTORYPAGE_TEXT_DESTINATION "安装目录（建议非 C 盘）"
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE WarnInstallDir
!insertmacro MUI_PAGE_DIRECTORY
!define MUI_COMPONENTSPAGE_SMALLDESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch ZwBrowser"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

Section "ZwBrowser (required)" SecMain
  SectionIn RO
  Call CloseInstalledProcesses
  SetOutPath "$INSTDIR"
  File "${STAGINGDIR}\${PRODUCT_EXE}"
  File /oname=${PRODUCT_ICON} "..\build\windows\icon.ico"
!if /FileExists "${STAGINGDIR}\config.yaml"
  IfFileExists "$INSTDIR\config.yaml" +2 0
    File "${STAGINGDIR}\config.yaml"
!else
  !echo "Warning: ${STAGINGDIR}\\config.yaml not found, installer will use runtime defaults."
!endif
!if /FileExists "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR\chrome"
  File /r "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR"
!endif
  CreateDirectory "$INSTDIR\data"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName"     "${PRODUCT_NAME}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "Publisher"       "ZwBrowser Team"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayIcon"     "$INSTDIR\${PRODUCT_ICON}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "NoModify"        "1"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "NoRepair"        "1"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_ICON}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk" "$INSTDIR\Uninstall.exe" "" "$INSTDIR\${PRODUCT_ICON}"
SectionEnd

Section "Proxy Runtime (xray / sing-box)" SecRuntime
  SectionIn RO
  SetOutPath "$INSTDIR\bin"
  File "${STAGINGDIR}\bin\xray.exe"
  File "${STAGINGDIR}\bin\sing-box.exe"
SectionEnd

Section /o "Desktop Shortcut" SecDesktop
  CreateShortcut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_ICON}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain}    "ZwBrowser main program and default config (required)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRuntime} "xray and sing-box proxy tools (vless/vmess/hysteria2)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a shortcut on the desktop"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  Call un.CloseInstalledProcesses

  Delete /REBOOTOK "$INSTDIR\${PRODUCT_EXE}"
  Delete /REBOOTOK "$INSTDIR\${PRODUCT_ICON}"
  Delete /REBOOTOK "$INSTDIR\config.yaml"
  Delete /REBOOTOK "$INSTDIR\proxies.yaml"
  Delete /REBOOTOK "$INSTDIR\Uninstall.exe"
  RMDir /r /REBOOTOK "$INSTDIR\bin"
  RMDir /r /REBOOTOK "$INSTDIR\chrome"
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
  RMDir /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}"
  Delete /REBOOTOK "$DESKTOP\${PRODUCT_NAME}.lnk"
  DeleteRegKey HKLM "${UNINSTALL_KEY}"
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "是否彻底清理所有用户数据？$\r$\n$\r$\n选择“是”将删除 data 目录（含数据库/实例数据）以及安装目录残留文件。$\r$\n此操作不可恢复。" IDYES un_remove_all_data IDNO un_keep_user_data

un_remove_all_data:
  RMDir /r /REBOOTOK "$INSTDIR\data"
  RMDir /r /REBOOTOK "$INSTDIR\logs"
  RMDir /r /REBOOTOK "$INSTDIR"
  Goto un_done

un_keep_user_data:
  RMDir /REBOOTOK "$INSTDIR"
  Goto un_done

un_done:
  IfFileExists "$INSTDIR\." 0 +2
    MessageBox MB_ICONEXCLAMATION|MB_OK "检测到部分文件仍被占用，已标记为重启后自动删除。"
SectionEnd
