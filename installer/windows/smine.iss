; smine Windows installer (addendum M5) — mirrors peek-mcp.iss: prebuilt
; payload plus the full source tree embedded; the install dir is a standalone
; git repo (created by -install, never a clone of the smine repo);
; everything else delegated to configserver.exe -install (self-elevating).
; Compiled by iscc in the public repo's release CI: iscc /DAppVersion=x.y.z
; installer\windows\smine.iss with the payload (incl. srctree\) staged under dist\.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef SrcTree
  #define SrcTree "..\..\dist\srctree"
#endif

[Setup]
AppId={{C14B0594-78A2-40A4-A027-7D6A3F677C12}
AppName=smine
AppVersion={#AppVersion}
DefaultDirName={localappdata}\smine\bin
DisableDirPage=yes
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\..\dist
OutputBaseFilename=smine-setup
SolidCompression=yes

[Files]
; Full source tree — the install dir is the user's own repo, never a clone.
Source: "{#SrcTree}\*"; DestDir: "{code:RepoDir}"; Flags: recursesubdirs createallsubdirs
; Repo binaries land in the checkout's bin\ (the scheduled tasks and the
; acdsl Read hook expect them there); shared payloads in the shim dir.
Source: "..\..\dist\bin\PAYLOAD";          DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\configserver.exe"; DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\routine.exe";      DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\acdsl.exe";        DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\rules.exe";        DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\agentwt.exe";      DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\heartbeat.exe";    DestDir: "{code:RepoBin}"
; Hook binaries: configserver -install copies them to ~/.claude/hooks/bin when Go is absent.
Source: "..\..\dist\bin\toolpolicy.exe";     DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\skillhelp.exe";      DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\worktreecreate.exe"; DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\sessiontitle.exe";   DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\readgate.exe";       DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\globalcontext.exe";  DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\skillcontext.exe";   DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\planmodecontext.exe"; DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\acdslhook.exe";      DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\skillinvokegate.exe"; DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\worktreewriteguard.exe"; DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\skillenv.exe";       DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\playsound.exe";      DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\heartbeatfeed.exe";  DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\verifiers\*";    DestDir: "{code:RepoBin}\verifiers"
Source: "..\..\dist\jq.exe";               DestDir: "{app}"
Source: "..\..\dist\peek-mcp.exe";         DestDir: "{app}"

; configserver -install runs from CurStepChanged, not [Run]: a [Run] entry ignores the exit code, and
; a failed install then ends in a green wizard with no server. The browser entry waits on its result.
[Run]
Filename: "http://127.0.0.1:6001/"; Flags: shellexec postinstall skipifsilent; \
  Description: "Open the config server"; Check: InstallSucceeded

[Code]
var
  RepoPage: TInputDirWizardPage;
  ProfilePage: TInputOptionWizardPage;
  PresetPage: TInputOptionWizardPage;
  ComponentsPage: TInputOptionWizardPage;
  SkillsPage: TInputOptionWizardPage;
  ChosenPage: TInputQueryWizardPage;
  ComponentIds: TArrayOfString;
  IsRecordFound: Boolean;
  InstallCode: Integer;

// AddComponent appends one catalog row; cmd/installer's iss test pins id,
// caption and enabled state to internal/components (one text source).
procedure AddComponent(Id, Caption: String; Enabled: Boolean);
var
  Index: Integer;
begin
  Index := ComponentsPage.Add(Caption);
  ComponentsPage.CheckListBox.ItemEnabled[Index] := Enabled;
  SetArrayLength(ComponentIds, Index + 1);
  ComponentIds[Index] := Id;
end;

function GitPath(): string;
begin
  // git.exe from PATH; empty when absent.
  Result := FileSearch('git.exe', GetEnv('PATH'));
end;

procedure InitializeWizard;
begin
  RepoPage := CreateInputDirPage(wpWelcome, 'smine repo',
    'Where should the smine repo live?',
    'Fresh folder: files are installed and a local git repository is created (no remote). Existing install: files are updated in place.',
    False, '');
  RepoPage.Add('');
  RepoPage.Values[0] := ExpandConstant('{%USERPROFILE}\smine');
  // Presentation profile: hard-coded selection; the id maps to a repo
  // template (settings\claude_code\presentation-profile.<id>.md) that
  // configserver -install copies to %USERPROFILE%\.claude\context\global\.
  ProfilePage := CreateInputOptionPage(RepoPage.ID,
    'Presentation profile', 'Who is this machine set up for?',
    'Select how smine presents itself on this machine. This can be changed later by replacing the profile file.',
    True, False);
  ProfilePage.Add('Default - English, developer');
  ProfilePage.Add('Deutsch - nicht-technisch (German, casual)');
  ProfilePage.Values[0] := True;
  // Components: a machine with a selection record keeps it unless the user
  // picks a preset (concept D18); a fresh machine starts at Minimal (D17).
  IsRecordFound := FileExists(ExpandConstant('{%USERPROFILE}\.claude\smine\install.json'));
  PresetPage := CreateInputOptionPage(ProfilePage.ID,
    'Components', 'What should smine install?',
    'Minimal installs the config server and the skills and nothing that intercepts a session. Full installs every component available on Windows. Custom lets you pick.',
    True, False);
  PresetPage.Add('Minimal');
  PresetPage.Add('Full');
  PresetPage.Add('Custom');
  if IsRecordFound then
    PresetPage.Add('Keep the current selection');
  PresetPage.Values[0] := True;
  if IsRecordFound then
    PresetPage.Values[3] := True;
  ComponentsPage := CreateInputOptionPage(PresetPage.ID,
    'Components', 'Pick the components',
    'A ticked component also installs what it needs. Greyed rows have no Windows backend.',
    False, False);
  AddComponent('config-server', 'Config server - The web UI and the smine-rules tool, always on.', False);
  AddComponent('skills', 'Skills - The repo''s skills as slash commands in Claude Code and Codex.', True);
  AddComponent('session-hooks', 'Session hooks - The master switch for everything that intercepts a session.', True);
  AddComponent('skill-environment', 'Skill environment - Each repo''s skill values exported into its sessions.', True);
  AddComponent('context', 'Context system - Context docs injected into sessions, skills and plan mode, plus the read gate.', True);
  AddComponent('gate', 'Permission gate - Closed-world permissions without prompts.', True);
  AddComponent('worktrees', 'Session worktrees - Every session in its own worktree.', True);
  AddComponent('session-helpers', 'Session helpers - Session titles, skill help, sounds.', True);
  AddComponent('global-instructions', 'Global instructions - The opinionated global instruction file.', True);
  AddComponent('agentwt', 'agentwt - Worktree status, merge order, safe removal.', True);
  AddComponent('peek', 'peek - Session inspection and telemetry.', True);
  AddComponent('ccusage', 'ccusage - Cost figures from ccusage.', True);
  AddComponent('heartbeat', 'heartbeat - Fleet state, orchestration, runtime checks.', True);
  AddComponent('routines', 'Routines - Scheduled unattended runs.', True);
  AddComponent('token', 'Headless token - Unattended claude -p calls.', True);
  AddComponent('mining', 'Mining pipeline - Session retrospectives and proposals.', True);
  ComponentsPage.Values[0] := True;
  ComponentsPage.Values[1] := True;
  SkillsPage := CreateInputOptionPage(ComponentsPage.ID,
    'Skills', 'Which skills should sync?',
    'Skills sync per selection; the Skills page of the config server deploys and removes single skills later.',
    True, False);
  SkillsPage.Add('All available skills');
  SkillsPage.Add('Chosen skills');
  SkillsPage.Add('None - deployed smine skills are removed');
  SkillsPage.Values[0] := True;
  ChosenPage := CreateInputQueryPage(SkillsPage.ID,
    'Chosen skills', 'Which skills?', 'Skill names, comma-separated (for example concept,fdesign).');
  ChosenPage.Add('Skills:', False);
end;

function IsKeep(): Boolean;
begin
  Result := IsRecordFound and PresetPage.Values[3];
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := False;
  if (ComponentsPage <> nil) and (PageID = ComponentsPage.ID) then
    Result := not PresetPage.Values[2];
  if (SkillsPage <> nil) and (PageID = SkillsPage.ID) then
    Result := IsKeep();
  if (ChosenPage <> nil) and (PageID = ChosenPage.ID) then
    Result := IsKeep() or not SkillsPage.Values[1];
end;

function ComponentsArg(Param: String): String;
var
  Index: Integer;
  Ids: String;
begin
  Result := '';
  if IsKeep() then
    Exit;
  if PresetPage.Values[0] then
    Result := ' -components=minimal'
  else if PresetPage.Values[1] then
    Result := ' -components=full'
  else begin
    Ids := '';
    for Index := 0 to GetArrayLength(ComponentIds) - 1 do
      if ComponentsPage.Values[Index] then begin
        if Ids <> '' then
          Ids := Ids + ',';
        Ids := Ids + ComponentIds[Index];
      end;
    Result := ' -components=' + Ids;
  end;
end;

function SkillsArg(Param: String): String;
var
  Chosen: String;
begin
  Result := '';
  if IsKeep() then
    Exit;
  if SkillsPage.Values[0] then
    Result := ' -skills=all'
  else if SkillsPage.Values[2] then
    Result := ' -skills=none'
  else begin
    Chosen := ChosenPage.Values[0];
    StringChangeEx(Chosen, ' ', '', True);
    Result := ' -skills=chosen:' + Chosen;
  end;
end;

function ProfileArg(Param: string): string;
begin
  Result := '';
  if (ProfilePage <> nil) and ProfilePage.Values[1] then
    Result := ' -presentation-profile=de';
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  Code: Integer;
begin
  Result := True;
  if (RepoPage <> nil) and (CurPageID = RepoPage.ID) then
    if GitPath() = '' then begin
      // Git for Windows is the one hard prerequisite; offer winget.
      if MsgBox('Git for Windows is required. Install it via winget now?',
                mbConfirmation, MB_YESNO) = IDYES then begin
        Exec('winget.exe', 'install --id Git.Git -e --accept-source-agreements --accept-package-agreements',
             '', SW_SHOW, ewWaitUntilTerminated, Code);
        if GitPath() = '' then begin
          MsgBox('git.exe still not found - finish the Git install, then re-run smine-setup.', mbError, MB_OK);
          Result := False;
        end;
      end else
        Result := False;
    end;
end;

function PrepareToInstall(var NeedsRestart: Boolean): string;
var
  Code: Integer;
begin
  Result := '';
  // Stop a running server first - it file-locks configserver.exe.
  Exec('powershell.exe', '-NoProfile -Command "Stop-ScheduledTask -TaskPath ''\smine\'' -TaskName configserver -ErrorAction SilentlyContinue"',
       '', SW_HIDE, ewWaitUntilTerminated, Code);
end;

function RepoDir(Param: string): string;
begin
  Result := RepoPage.Values[0];
end;

function RepoBin(Param: string): string;
begin
  Result := RepoPage.Values[0] + '\bin';
end;

function InstallSucceeded(): Boolean;
begin
  Result := InstallCode = 0;
end;

// LogTail is the last lines of the self-elevated install's log, which the
// non-elevated configserver -install also prints to its own console.
function LogTail(LogPath: string): string;
var
  Lines: TArrayOfString;
  Index, First: Integer;
begin
  Result := '';
  if not LoadStringsFromFile(LogPath, Lines) then
    Exit;
  First := GetArrayLength(Lines) - 12;
  if First < 0 then
    First := 0;
  for Index := First to GetArrayLength(Lines) - 1 do
    Result := Result + Lines[Index] + #13#10;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Params, LogPath: string;
begin
  if CurStep <> ssPostInstall then
    Exit;
  WizardForm.StatusLabel.Caption := 'Registering tasks and syncing settings...';
  Params := '-install' + ProfileArg('') + ComponentsArg('') + SkillsArg('');
  if not Exec(RepoBin('') + '\configserver.exe', Params, RepoDir(''), SW_SHOW, ewWaitUntilTerminated, InstallCode) then
    InstallCode := -1;
  if InstallCode = 0 then
    Exit;
  LogPath := ExpandConstant('{%TEMP}\smine-install.log');
  MsgBox('configserver -install failed (exit ' + IntToStr(InstallCode) + '): the config server is not running.'
    + #13#10 + 'Fix the cause and re-run smine-setup, or run bin\configserver.exe -install in the install folder.'
    + #13#10#13#10 + 'Log: ' + LogPath + #13#10#13#10 + LogTail(LogPath), mbError, MB_OK);
end;
