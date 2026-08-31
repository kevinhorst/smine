# install.ps1 - Windows from-source install: prereqs, build, then delegate
# everything else to configserver.exe -install (macOS: use ./install.sh).
param(
    [string]$Addr = "127.0.0.1:6001",
    [switch]$InitWelcome,
    [int]$PeekPort = 4242,
    [int]$PeekControlPort = 42442,
    [string]$PresentationProfile = ""
)
$ErrorActionPreference = "Stop"
$RepoDir = Split-Path -Parent $MyInvocation.MyCommand.Path

function Test-Cmd($name) { [bool](Get-Command $name -ErrorAction SilentlyContinue) }

# 1. Prerequisites: detect, then one consent prompt for winget installs.
$missing = @()
if (-not (Test-Cmd git.exe)) { $missing += "Git.Git" }
if (-not (Test-Cmd go.exe))  { $missing += "GoLang.Go" }
if (-not (Test-Cmd jq.exe))  { $missing += "jqlang.jq" }
if (-not (Test-Cmd shellcheck.exe)) { $missing += "koalaman.shellcheck" }
if ($missing.Count -gt 0) {
    if (-not (Test-Cmd winget.exe)) {
        throw "winget missing (install 'App Installer' from the Microsoft Store), and prerequisites are missing: $($missing -join ', ')"
    }
    Write-Host "Missing prerequisites: $($missing -join ', ')"
    $answer = Read-Host "Install via winget? [y/N]"
    if ($answer -ne "y") { throw "prerequisites missing; aborting" }
    foreach ($id in $missing) {
        winget install --id $id -e --accept-source-agreements --accept-package-agreements
    }
    Write-Host "Prerequisites installed - open a NEW terminal if commands are not found, then re-run install.ps1"
}

# 2. Routine OAuth token: interactive convenience prompt (the Go installer
#    only prints guidance). Whitespace stripped - pasted tokens wrap.
$tokenPath = Join-Path $env:USERPROFILE ".config\claude-routine\token"
if (-not ((Test-Path $tokenPath) -and ((Get-Item $tokenPath).Length -gt 0))) {
    $token = Read-Host "Routine OAuth token (run 'claude setup-token' to get one; Enter to skip)"
    $token = $token -replace '\s', ''
    if ($token) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $tokenPath) | Out-Null
        Set-Content -NoNewline -Path $tokenPath -Value $token
        Write-Host "-> Token written: $tokenPath"
    }
}

Push-Location $RepoDir
try {
    # 3. Build. A running server file-locks its exe - stop the task first.
    Stop-ScheduledTask -TaskPath '\smine\' -TaskName configserver -ErrorAction SilentlyContinue
    Write-Host "-> Building binaries ..."
    go build -ldflags "-H=windowsgui" -o bin\configserver.exe ./cmd/configserver
    go build -ldflags "-H=windowsgui" -o bin\routinewrap.exe ./cmd/routinewrap
    go build -o bin\acdsl.exe ./cmd/acdsl
    go build -o bin\rules.exe ./cmd/rules

    # 4. Delegate: bash/claude/peek/PATH/syncs/tasks/verify all live in Go;
    #    -install self-elevates (UAC prompt) when needed. The pipe makes PS
    #    wait for the windowsgui exe and surfaces its output; EAP=Continue
    #    keeps PS 5.1 from throwing on native stderr lines.
    $eap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $initWelcomeArg = if ($InitWelcome) { "true" } else { "false" }
    & bin\configserver.exe -install -addr $Addr -init-welcome=$initWelcomeArg -peek-port $PeekPort -peek-control-port $PeekControlPort -presentation-profile $PresentationProfile 2>&1 | ForEach-Object { "$_" }
    $ErrorActionPreference = $eap
    if ($LASTEXITCODE -ne 0) { throw "configserver -install failed (exit $LASTEXITCODE)" }
} finally {
    Pop-Location
}
