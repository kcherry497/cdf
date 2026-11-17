; CDF Viewer — Windows installer
; Builds a single-file installer that:
;   - places cdf-gui.exe under Program Files
;   - creates Start Menu shortcut + Desktop shortcut (optional)
;   - registers .cdf file association pointing at cdf-gui.exe
;   - writes uninstaller in Control Panel > Programs

!include "MUI2.nsh"
!include "FileAssociation.nsh"
!include "LogicLib.nsh"

!ifndef VERSION
  !define VERSION "1.0.1"
!endif

Name "CDF Viewer"
OutFile "cdf-viewer-${VERSION}-setup.exe"
InstallDir "$PROGRAMFILES64\CDF Viewer"
InstallDirRegKey HKCU "Software\CDF Viewer" ""
RequestExecutionLevel admin
Unicode true
BrandingText "CDF Viewer"

!define MUI_ABORTWARNING
!define MUI_ICON "cdf-viewer.ico"
!define MUI_UNICON "cdf-viewer.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_WELCOME
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

!insertmacro MUI_LANGUAGE "English"

Section "CDF Viewer (required)" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"

  File "cdf-gui.exe"
  File "cdf-viewer.ico"

  ; Start Menu shortcut
  CreateDirectory "$SMPROGRAMS\CDF Viewer"
  CreateShortcut "$SMPROGRAMS\CDF Viewer\CDF Viewer.lnk" "$INSTDIR\cdf-gui.exe" "" "$INSTDIR\cdf-viewer.ico"
  CreateShortcut "$SMPROGRAMS\CDF Viewer\Uninstall.lnk" "$INSTDIR\uninstall.exe"

  ; Register .cdf file association
  WriteRegStr HKCR ".cdf" "" "CDF.Document"
  WriteRegStr HKCR ".cdf" "Content Type" "application/x-cdf"
  WriteRegStr HKCR "CDF.Document" "" "ClaudeDocFormat Document"
  WriteRegStr HKCR "CDF.Document\DefaultIcon" "" "$INSTDIR\cdf-viewer.ico,0"
  WriteRegStr HKCR "CDF.Document\shell\open\command" "" '"$INSTDIR\cdf-gui.exe" "%1"'

  ; Uninstall info in Control Panel
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "DisplayName" "CDF Viewer"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "DisplayIcon" "$INSTDIR\cdf-viewer.ico"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "Publisher" "Kevin Cherry"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer" "NoRepair" 1

  WriteRegStr HKCU "Software\CDF Viewer" "" "$INSTDIR"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Tell Explorer to refresh the icon cache so the new file association shows up
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, i 0, i 0)'
SectionEnd

Section "Desktop Shortcut" SecDesktop
  CreateShortcut "$DESKTOP\CDF Viewer.lnk" "$INSTDIR\cdf-gui.exe" "" "$INSTDIR\cdf-viewer.ico"
SectionEnd

Section "Uninstall"
  ; Remove files
  Delete "$INSTDIR\cdf-gui.exe"
  Delete "$INSTDIR\cdf-viewer.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  ; Shortcuts
  Delete "$SMPROGRAMS\CDF Viewer\CDF Viewer.lnk"
  Delete "$SMPROGRAMS\CDF Viewer\Uninstall.lnk"
  RMDir  "$SMPROGRAMS\CDF Viewer"
  Delete "$DESKTOP\CDF Viewer.lnk"

  ; Registry
  DeleteRegKey HKCR ".cdf"
  DeleteRegKey HKCR "CDF.Document"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CDFViewer"
  DeleteRegKey HKCU "Software\CDF Viewer"

  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, i 0, i 0)'
SectionEnd
