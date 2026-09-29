#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#ifndef RepoRoot
  #define RepoRoot "..\.."
#endif

[Setup]
AppId={{D7F795CE-DC1D-49A7-BD3C-F89EC8A4E1C4}
AppName=Ducky One X Configurator
AppVersion={#AppVersion}
AppPublisher=Ducky One X Configurator contributors
DefaultDirName={autopf}\Ducky One X Configurator
DefaultGroupName=Ducky One X Configurator
DisableProgramGroupPage=yes
OutputDir={#RepoRoot}\dist
OutputBaseFilename=Ducky-One-X-Configurator-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
UninstallDisplayIcon={app}\ducky-config.exe
CloseApplications=yes
RestartApplications=no

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "{#RepoRoot}\dist\ducky-config.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\Ducky One X Configurator"; Filename: "{app}\ducky-config.exe"
Name: "{autodesktop}\Ducky One X Configurator"; Filename: "{app}\ducky-config.exe"; Tasks: desktopicon

[Registry]
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "DuckyOneXConfigurator"; ValueData: """{app}\ducky-config.exe"" --minimized"; Flags: uninsdeletevalue

[Run]
Filename: "{app}\ducky-config.exe"; Description: "Open Ducky One X Configurator"; Flags: nowait postinstall skipifsilent
