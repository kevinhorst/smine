# install.ps1 - Windows from-source launcher: Git and Go via winget when
# missing, then the Go installer (cmd/installer) with every argument
# forwarded unchanged (master C6). install.bat runs it under
# -ExecutionPolicy Bypass.
$ErrorActionPreference = "Stop"
$RepoDir = Split-Path -Parent $MyInvocation.MyCommand.Path

function Test-Cmd($name) { [bool](Get-Command $name -ErrorAction SilentlyContinue) }

$missing = @()
if (-not (Test-Cmd git.exe)) { $missing += "Git.Git" }
if (-not (Test-Cmd go.exe))  { $missing += "GoLang.Go" }
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
    throw "Prerequisites installed - open a NEW terminal, then re-run install.bat"
}

Push-Location $RepoDir
try {
    & go run ./cmd/installer @args
    if ($LASTEXITCODE -ne 0) { throw "installer failed (exit $LASTEXITCODE)" }
} finally {
    Pop-Location
}
