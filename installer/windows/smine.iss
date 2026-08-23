; smine Windows installer (addendum M5) — mirrors peek-mcp.iss: prebuilt
; payload embedded, the wizard clones (or ff-updates) the public smine repo,
; everything else delegated to configserver.exe -install (self-elevating).
; Compiled by iscc in the public repo's release CI: iscc /DAppVersion=x.y.z
; installer\windows\smine.iss with the payload staged under dist\.

#ifndef AppVersion
  #define AppVersion "0.0.0"
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
; Repo binaries land in the checkout's bin\ (the scheduled tasks and the
; acdsl Read hook expect them there); shared payloads in the shim dir.
Source: "..\..\dist\bin\configserver.exe"; DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\routinewrap.exe";  DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\acdsl.exe";        DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\rules.exe";        DestDir: "{code:RepoBin}"
Source: "..\..\dist\bin\verifiers\*";    DestDir: "{code:RepoBin}\verifiers"
Source: "..\..\dist\jq.exe";               DestDir: "{app}"
Source: "..\..\dist\peek-mcp.exe";         DestDir: "{app}"

[Run]
Filename: "{code:RepoBin}\configserver.exe"; Parameters: "-install"; \
  WorkingDir: "{code:RepoDir}"; Flags: waituntilterminated; \
  StatusMsg: "Registering tasks and syncing settings..."
Filename: "http://127.0.0.1:6001/"; Flags: shellexec postinstall skipifsilent; \
  Description: "Open the config server"

[Code]
const RepoURL = 'https://github.com/kevinhorst/smine';
var
  RepoPage: TInputDirWizardPage;

function GitPath(): string;
begin
  // git.exe from PATH; empty when absent.
  Result := FileSearch('git.exe', GetEnv('PATH'));
end;

procedure InitializeWizard;
begin
  RepoPage := CreateInputDirPage(wpWelcome, 'smine repo',
    'Where should the smine repo live?',
    'Missing: cloned from ' + RepoURL + '. Existing clone: updated (fast-forward only).',
    False, '');
  RepoPage.Add('');
  RepoPage.Values[0] := ExpandConstant('{%USERPROFILE}\smine');
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
  Repo: string;
  Code: Integer;
begin
  Result := '';
  Repo := RepoPage.Values[0];
  // Stop a running server first - it file-locks configserver.exe.
  Exec('powershell.exe', '-NoProfile -Command "Stop-ScheduledTask -TaskPath ''\smine\'' -TaskName configserver -ErrorAction SilentlyContinue"',
       '', SW_HIDE, ewWaitUntilTerminated, Code);
  if DirExists(Repo + '\.git') then begin
    Exec(GitPath(), '-C "' + Repo + '" pull --ff-only', '', SW_HIDE, ewWaitUntilTerminated, Code);
    if Code <> 0 then
      MsgBox('git pull --ff-only failed (local changes?) - installing onto the existing tree.', mbInformation, MB_OK);
  end else begin
    Exec(GitPath(), 'clone ' + RepoURL + ' "' + Repo + '"', '', SW_SHOW, ewWaitUntilTerminated, Code);
    if Code <> 0 then
      Result := 'git clone failed (exit ' + IntToStr(Code) + ') - check network access and the target folder.';
  end;
end;

function RepoDir(Param: string): string;
begin
  Result := RepoPage.Values[0];
end;

function RepoBin(Param: string): string;
begin
  Result := RepoPage.Values[0] + '\bin';
end;
