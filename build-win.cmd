@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
where pwsh >nul 2>nul
if %errorlevel%==0 (
  pwsh -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-win.ps1"
) else (
  powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-win.ps1"
)
echo.
echo Press any key to close this window.
pause >nul
endlocal
