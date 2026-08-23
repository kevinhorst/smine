@echo off
rem install.bat - Windows entry point: run install.ps1 under an ExecutionPolicy
rem bypass so the default Restricted policy does not block it. Args forwarded.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
exit /b %ERRORLEVEL%
