; tail-split installer — Inno Setup
; Builds tail-split-setup.exe: installs to Program Files, registers the
; app in Apps & Features (uninstall entry), Start Menu shortcut,
; optional "run at Windows startup". Re-running a newer installer
; upgrades in place.

#define AppVersion GetEnv("APP_VERSION")
#if AppVersion == ""
  #define AppVersion "0.0.0"
#endif

[Setup]
AppId={{7E4A1C2B-5F3D-4E8A-9B6C-1D2E3F4A5B6C}
AppName=tail-split
AppVersion={#AppVersion}
AppPublisher=mohammadraufzahed
DefaultDirName={autopf}\tail-split
DefaultGroupName=tail-split
OutputDir=installer-out
OutputBaseFilename=tail-split-setup
SetupIconFile=icon.ico
UninstallDisplayIcon={app}\tail-split.exe
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
CloseApplications=force
RestartApplications=no
WizardStyle=modern

[Tasks]
Name: "startup"; Description: "Run tail-split at Windows startup"; GroupDescription: "Additional options:"

[Files]
Source: "tail-split.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\tail-split"; Filename: "{app}\tail-split.exe"; IconFilename: "{app}\tail-split.exe"
Name: "{group}\Uninstall tail-split"; Filename: "{uninstallexe}"
Name: "{userstartup}\tail-split"; Filename: "{app}\tail-split.exe"; Tasks: startup

[Run]
Filename: "{app}\tail-split.exe"; Description: "Launch tail-split"; Flags: nowait postinstall skipifsilent unchecked
