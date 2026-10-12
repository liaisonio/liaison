@echo off
REM Compatibility entry point; all installation logic lives in install.ps1.
if not exist "%~dp0install.ps1" (
  echo Download install.ps1 beside this file from your trusted Liaison server.
  exit /b 1
)
powershell.exe -NoProfile -File "%~dp0install.ps1" %*
exit /b %ERRORLEVEL%
