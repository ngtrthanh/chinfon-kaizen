@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
set APP_ADDR=0.0.0.0:8080
if not defined APP_DATA_DIR set APP_DATA_DIR=%~dp0data
if not defined APP_SEED_DEMO set APP_SEED_DEMO=false
call start.cmd
endlocal
