; k10s Windows installer (user-mode, offline-friendly bundle).
;
; Build with:
;   makensis -DVERSION=x.y.z scripts/k10s-installer.nsi
;
; Expects these files relative to the working directory (the script is run
; from the repo root by scripts/build-windows-installer.sh):
;   dist/k10s_windows_amd64_v1/k10s.exe       (from goreleaser)
;   dist/windows-deps/kubectl.exe
;   dist/windows-deps/k9s.exe
;   dist/windows-deps/kubelogin.exe
;   dist/windows-deps/kubectl-oidc_login.exe
;   dist/windows-deps/LICENSES/*

Unicode True
SetCompressor /SOLID lzma

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WinMessages.nsh"
!include "StrFunc.nsh"
${StrStr}

!ifndef VERSION
  !define VERSION "0.0.0-dev"
!endif

; Absolute path to the repo root (where dist/ lives). Caller should pass
; this via `-DROOT=...` so the File commands resolve regardless of where
; makensis is invoked from.
!ifndef ROOT
  !define ROOT ".."
!endif

!define APP_NAME          "k10s"
!define APP_PUBLISHER     "benly50s"
!define APP_URL           "https://github.com/benly50s/k10s"
!define INSTDIR_NAME      "k10s"
!define UNINST_KEY        "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"
!define START_MENU_DIR    "$SMPROGRAMS\${APP_NAME}"

Name "${APP_NAME} ${VERSION}"
OutFile "${ROOT}/dist/k10s-setup-${VERSION}.exe"
InstallDir "$LOCALAPPDATA\Programs\${INSTDIR_NAME}"
InstallDirRegKey HKCU "Software\${APP_NAME}" "InstallDir"
RequestExecutionLevel user

VIProductVersion "0.0.0.0"
VIAddVersionKey "ProductName"     "${APP_NAME}"
VIAddVersionKey "CompanyName"     "${APP_PUBLISHER}"
VIAddVersionKey "FileDescription" "k10s Kubernetes TUI installer"
VIAddVersionKey "FileVersion"     "${VERSION}"
VIAddVersionKey "ProductVersion"  "${VERSION}"

!define MUI_ABORTWARNING
!define MUI_ICON   "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Korean"

; ---------- install ----------
Section "Install"
  SetOutPath "$INSTDIR"

  File "${ROOT}/dist/k10s_windows_amd64_v1/k10s.exe"
  File "${ROOT}/dist/windows-deps/kubectl.exe"
  File "${ROOT}/dist/windows-deps/k9s.exe"
  File "${ROOT}/dist/windows-deps/kubelogin.exe"
  File "${ROOT}/dist/windows-deps/kubectl-oidc_login.exe"

  SetOutPath "$INSTDIR\LICENSES"
  File /r "${ROOT}/dist/windows-deps/LICENSES/*.*"

  SetOutPath "$INSTDIR"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Start Menu
  CreateDirectory "${START_MENU_DIR}"
  CreateShortCut  "${START_MENU_DIR}\${APP_NAME}.lnk"           "cmd.exe" '/K "$INSTDIR\k10s.exe --help"' "$INSTDIR\k10s.exe"
  CreateShortCut  "${START_MENU_DIR}\Uninstall ${APP_NAME}.lnk" "$INSTDIR\uninstall.exe"

  ; Add install dir to user PATH
  Push "$INSTDIR"
  Call AddToUserPath

  ; Add/Remove Programs entry (per-user)
  WriteRegStr   HKCU "${UNINST_KEY}" "DisplayName"          "${APP_NAME}"
  WriteRegStr   HKCU "${UNINST_KEY}" "DisplayVersion"       "${VERSION}"
  WriteRegStr   HKCU "${UNINST_KEY}" "Publisher"            "${APP_PUBLISHER}"
  WriteRegStr   HKCU "${UNINST_KEY}" "URLInfoAbout"         "${APP_URL}"
  WriteRegStr   HKCU "${UNINST_KEY}" "InstallLocation"      "$INSTDIR"
  WriteRegStr   HKCU "${UNINST_KEY}" "UninstallString"      "$\"$INSTDIR\uninstall.exe$\""
  WriteRegStr   HKCU "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
  WriteRegStr   HKCU "${UNINST_KEY}" "DisplayIcon"          "$INSTDIR\k10s.exe"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify"             1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair"             1

  WriteRegStr HKCU "Software\${APP_NAME}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "Software\${APP_NAME}" "Version"    "${VERSION}"
SectionEnd

; ---------- uninstall ----------
Section "Uninstall"
  Push "$INSTDIR"
  Call un.RemoveFromUserPath

  Delete "$INSTDIR\k10s.exe"
  Delete "$INSTDIR\kubectl.exe"
  Delete "$INSTDIR\k9s.exe"
  Delete "$INSTDIR\kubelogin.exe"
  Delete "$INSTDIR\kubectl-oidc_login.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir /r "$INSTDIR\LICENSES"
  RMDir "$INSTDIR"

  Delete "${START_MENU_DIR}\${APP_NAME}.lnk"
  Delete "${START_MENU_DIR}\Uninstall ${APP_NAME}.lnk"
  RMDir  "${START_MENU_DIR}"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\${APP_NAME}"
SectionEnd

; ---------- PATH helpers ----------
; Adds the top-of-stack value to HKCU\Environment\PATH without duplicates,
; then broadcasts WM_SETTINGCHANGE so new shells pick up the change.
Function AddToUserPath
  Exch $0
  Push $1
  Push $2

  ReadRegStr $1 HKCU "Environment" "PATH"
  ${If} $1 == ""
    WriteRegExpandStr HKCU "Environment" "PATH" "$0"
  ${Else}
    ${StrStr} $2 ";$1;" ";$0;"
    ${If} $2 == ""
      WriteRegExpandStr HKCU "Environment" "PATH" "$1;$0"
    ${EndIf}
  ${EndIf}

  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000

  Pop $2
  Pop $1
  Pop $0
FunctionEnd

; Removes a single exact-match entry from HKCU\Environment\PATH by splitting
; on ';' and filtering. Empty tokens are dropped too.
Function un.RemoveFromUserPath
  Exch $0                       ; $0 = entry to remove
  Push $1                       ; $1 = remaining PATH being consumed
  Push $2                       ; $2 = new PATH being built
  Push $3                       ; $3 = current token
  Push $4                       ; $4 = char at offset
  Push $5                       ; $5 = offset

  ReadRegStr $1 HKCU "Environment" "PATH"
  ${If} $1 == ""
    Goto _rm_broadcast
  ${EndIf}

  StrCpy $2 ""
  StrCpy $1 "$1;"               ; sentinel so trailing token is emitted

_rm_loop:
  StrLen $5 $1
  ${If} $5 == 0
    Goto _rm_write
  ${EndIf}

  StrCpy $5 0
_rm_scan:
  StrCpy $4 $1 1 $5
  ${If} $4 == ";"
    StrCpy $3 $1 $5
    IntOp $5 $5 + 1
    StrCpy $1 $1 "" $5
    Goto _rm_emit
  ${EndIf}
  IntOp $5 $5 + 1
  Goto _rm_scan

_rm_emit:
  ${If} $3 != ""
  ${AndIf} $3 != $0
    ${If} $2 == ""
      StrCpy $2 $3
    ${Else}
      StrCpy $2 "$2;$3"
    ${EndIf}
  ${EndIf}
  Goto _rm_loop

_rm_write:
  WriteRegExpandStr HKCU "Environment" "PATH" $2

_rm_broadcast:
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000

  Pop $5
  Pop $4
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd
