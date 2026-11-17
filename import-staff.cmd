@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
if exist "bin\kaizen.exe" (
  .\bin\kaizen.exe import-staff %*
) else (
  go run ./cmd/kaizen import-staff %*
)
echo.
echo Import finished. Press any key to close this window.
pause >nul
endlocal
