@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo [SECURITY] Plain HTTP on a LAN is disabled for personnel data.
echo Run start.cmd on 127.0.0.1 and publish HTTPS through a trusted reverse proxy.
echo For a Tailscale tailnet, configure tailscale serve with HTTPS to localhost:8080.
exit /b 1
