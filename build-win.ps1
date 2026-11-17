$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

New-Item -ItemType Directory -Force -Path (Join-Path $PSScriptRoot 'bin') | Out-Null
$out = Join-Path $PSScriptRoot 'bin\kaizen.exe'

go build -trimpath -ldflags "-s -w" -o $out ./cmd/kaizen
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Built: $out" -ForegroundColor Green
Write-Host "Run:   .\bin\kaizen.exe" -ForegroundColor DarkGray
Write-Host "Open:  http://127.0.0.1:8080/setup on first run" -ForegroundColor DarkGray
