; codec desktop installer for Windows (NSIS 3). Per user, no admin
; rights (decision 82): everything goes to %LOCALAPPDATA%\Programs\codec
; and HKCU. Built by scripts/desktop.sh, which passes:
;   /DVERSION=3.0.0  /DVERSION4=3.0.0.0  /DEXE=<path to codec-desktop.exe>
;   /DOUTFILE=<installer path>

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user

!macro Require NAME
  !ifndef ${NAME}
    !error "${NAME} is not set; build with scripts/desktop.sh"
  !endif
!macroend
!insertmacro Require VERSION
!insertmacro Require VERSION4
!insertmacro Require EXE
!insertmacro Require OUTFILE

!define APP      "codec"
!define APPEXE   "codec-desktop.exe"
!define PUBLISHER "Mahasen Abheetha"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP}"
!define PROGID   "codec.yaml"

Name "${APP}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\${APP}"
InstallDirRegKey HKCU "${UNINSTKEY}" "InstallLocation"

VIProductVersion "${VERSION4}"
VIAddVersionKey "ProductName" "${APP}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "${APP} installer"
VIAddVersionKey "CompanyName" "${PUBLISHER}"
VIAddVersionKey "LegalCopyright" "MIT License"

!include "MUI2.nsh"
!define MUI_ICON "icon.ico"
!define MUI_UNICON "icon.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APPEXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Start codec"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

!include "StrFunc.nsh"
${UnStrStr}

; A running codec holds its exe open. It is read-only, so nothing is
; lost by ending it (closing the window would only hide it in the tray).
; Only the copy in DIR is stopped; a portable copy elsewhere keeps running.
; CIM, not Get-Process: the installer is 32-bit, so its PowerShell can't
; read a 64-bit process's path, but Win32_Process reports it.
!macro StopCodec DIR
  nsExec::Exec `powershell -NoProfile -NonInteractive -Command "Get-CimInstance Win32_Process -Filter \"Name='${APPEXE}'\" | Where-Object { $$_.ExecutablePath -eq '${DIR}\${APPEXE}' } | ForEach-Object { Stop-Process -Id $$_.ProcessId -Force }"`
  Pop $0
  Sleep 500
!macroend

; Shortcuts are created only where no file of that name exists (the
; user may already have a "codec" shortcut, e.g. a browser app), and the
; uninstaller removes only the ones recorded here.
!macro Shortcut LNK VALUE
  IfFileExists "${LNK}" +3
    CreateShortcut "${LNK}" "$INSTDIR\${APPEXE}"
    WriteRegDWORD HKCU "${UNINSTKEY}" "${VALUE}" 1
!macroend

!macro UnShortcut LNK VALUE
  ReadRegDWORD $0 HKCU "${UNINSTKEY}" "${VALUE}"
  StrCmp $0 1 0 +2
    Delete "${LNK}"
!macroend

Section "codec" SecApp
  SectionIn RO
  SetShellVarContext current

  ; Installed before in another folder: remove that copy first, so it
  ; doesn't linger (or start at login) beside the new one.
  ReadRegStr $1 HKCU "${UNINSTKEY}" "InstallLocation"
  StrCmp $1 "" old_done
  StrCmp $1 $INSTDIR old_done
  IfFileExists "$1\uninstall.exe" 0 old_done
    ExecWait '"$1\uninstall.exe" /S _?=$1'
    Delete "$1\uninstall.exe"
    RMDir "$1"
  old_done:

  !insertmacro StopCodec "$INSTDIR"
  SetOutPath "$INSTDIR"
  File "/oname=${APPEXE}" "${EXE}"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Apps & features entry (per user)
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "${APP}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\${APPEXE}"
  WriteRegStr HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINSTKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegStr HKCU "${UNINSTKEY}" "URLInfoAbout" "https://github.com/mahasenabheetha/codec"
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "EstimatedSize" 80000 ; KB

  ; After the uninstall key exists, so the shortcut can be recorded in it.
  !insertmacro Shortcut "$SMPROGRAMS\${APP}.lnk" "StartMenuShortcut"
SectionEnd

Section "Open YAML files with codec" SecOpenWith
  ; Adds codec to "Open with" for .yaml and .yml. It does not take over
  ; the default app; Windows lets the user choose that.
  WriteRegStr HKCU "Software\Classes\${PROGID}" "" "YAML file"
  WriteRegStr HKCU "Software\Classes\${PROGID}\DefaultIcon" "" "$INSTDIR\${APPEXE},0"
  WriteRegStr HKCU "Software\Classes\${PROGID}\shell\open\command" "" '"$INSTDIR\${APPEXE}" "%1"'
  WriteRegStr HKCU "Software\Classes\.yaml\OpenWithProgids" "${PROGID}" ""
  WriteRegStr HKCU "Software\Classes\.yml\OpenWithProgids" "${PROGID}" ""
  WriteRegStr HKCU "Software\Classes\Applications\${APPEXE}" "FriendlyAppName" "${APP}"
  WriteRegStr HKCU "Software\Classes\Applications\${APPEXE}\shell\open\command" "" '"$INSTDIR\${APPEXE}" "%1"'
  WriteRegStr HKCU "Software\Classes\Applications\${APPEXE}\SupportedTypes" ".yaml" ""
  WriteRegStr HKCU "Software\Classes\Applications\${APPEXE}\SupportedTypes" ".yml" ""
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
SectionEnd

Section /o "Desktop shortcut" SecDesktop
  SetShellVarContext current
  !insertmacro Shortcut "$DESKTOP\${APP}.lnk" "DesktopShortcut"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecApp} "codec itself, with a Start menu entry."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecOpenWith} "Offer codec in Explorer's Open with menu for .yaml and .yml files."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "A codec shortcut on the desktop."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  SetShellVarContext current
  !insertmacro StopCodec "$INSTDIR"
  Delete "$INSTDIR\${APPEXE}"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  !insertmacro UnShortcut "$SMPROGRAMS\${APP}.lnk" "StartMenuShortcut"
  !insertmacro UnShortcut "$DESKTOP\${APP}.lnk" "DesktopShortcut"

  DeleteRegKey HKCU "${UNINSTKEY}"
  DeleteRegKey HKCU "Software\Classes\${PROGID}"
  DeleteRegValue HKCU "Software\Classes\.yaml\OpenWithProgids" "${PROGID}"
  DeleteRegValue HKCU "Software\Classes\.yml\OpenWithProgids" "${PROGID}"
  DeleteRegKey HKCU "Software\Classes\Applications\${APPEXE}"
  ; Start at login (Settings → Desktop), if it was on for this copy; a
  ; portable copy's entry stays.
  ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP}"
  ${UnStrStr} $1 $0 "$INSTDIR\${APPEXE}"
  StrCmp $1 "" +2
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP}"
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'

  ; The window's own storage (display preferences, cache). Settings and
  ; recent folders in %APPDATA%\codec are shared with the codec CLI and
  ; stay. WebView2's helper processes may hold it for a moment after
  ; codec stops, so try for a few seconds.
  StrCpy $2 0
  webview_loop:
    RMDir /r "$LOCALAPPDATA\codec\webview"
    IfFileExists "$LOCALAPPDATA\codec\webview\*.*" 0 webview_done
    IntOp $2 $2 + 1
    IntCmp $2 10 webview_done
    Sleep 500
    Goto webview_loop
  webview_done:
SectionEnd
