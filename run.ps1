$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8

# --- Go check ---------------------------------------------------------------
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    Write-Host "Go was not found in PATH." -ForegroundColor Red
    Write-Host "Install Go 1.25+ (recommended 1.27) and reopen PowerShell." -ForegroundColor Yellow
    exit 1
}
Write-Host "Go: $((& go version))" -ForegroundColor DarkGray

# --- Address / port ---------------------------------------------------------
$explicitAddr = [bool]$env:APP_ADDR
if (-not $env:APP_ADDR) { $env:APP_ADDR = '127.0.0.1:8080' }
if (-not $env:APP_DATA_DIR) { $env:APP_DATA_DIR = Join-Path $PSScriptRoot 'data' }
if (-not $env:APP_SEED_DEMO) { $env:APP_SEED_DEMO = 'false' }

$addr = [string]$env:APP_ADDR
$idx = $addr.LastIndexOf(':')
if ($idx -lt 0) { $addr = "$addr`:8080"; $idx = $addr.LastIndexOf(':') }
$bindHost = $addr.Substring(0, $idx)
if ([string]::IsNullOrWhiteSpace($bindHost)) { $bindHost = '127.0.0.1' }
$port = 8080
[void][int]::TryParse($addr.Substring($idx + 1), [ref]$port)
if ($port -le 0) { $port = 8080 }
$checkHost = $bindHost
if ($bindHost -eq '0.0.0.0' -or $bindHost -eq '::') { $checkHost = '127.0.0.1' }
$baseUrl = "http://$($checkHost):$port"

function Test-PortFree([int]$p, [string]$h) {
    $listener = $null
    try {
        $ip = [System.Net.IPAddress]::Loopback
        if ($h -eq '0.0.0.0') { $ip = [System.Net.IPAddress]::Any }
        elseif ($h -eq '::') { $ip = [System.Net.IPAddress]::IPv6Any }
        elseif (-not [string]::IsNullOrWhiteSpace($h)) { $ip = [System.Net.IPAddress]::Parse($h) }
        $listener = New-Object System.Net.Sockets.TcpListener($ip, $p)
        $listener.Start()
        return $true
    } catch {
        return $false
    } finally {
        if ($listener) { $listener.Stop() }
    }
}

function Test-KaizenAt([string]$url) {
    try {
        $r = Invoke-WebRequest -UseBasicParsing "$url/setup" -TimeoutSec 2 -MaximumRedirection 3
        return ($r.Content -match 'Chinfon Kaizen 360')
    } catch {
        return $false
    }
}

if (Test-KaizenAt $baseUrl) {
    Write-Host "v3 is already running at $baseUrl" -ForegroundColor Green
    Write-Host "Open: $baseUrl/" -ForegroundColor Cyan
    try { Start-Process "$baseUrl/" } catch {}
    exit 0
}

if (-not (Test-PortFree $port $bindHost)) {
    if ($explicitAddr) {
        Write-Host "Port $port on $bindHost is already in use." -ForegroundColor Red
        Write-Host "Set a free port, for example:" -ForegroundColor Yellow
        Write-Host '  $env:APP_ADDR="127.0.0.1:8081"; .\start.cmd' -ForegroundColor Yellow
        exit 1
    }
    $free = $null
    for ($p = 8081; $p -le 8099; $p++) {
        if (Test-PortFree $p '127.0.0.1') { $free = $p; break }
    }
    if (-not $free) {
        Write-Host "No free port found in 8080-8099." -ForegroundColor Red
        exit 1
    }
    Write-Host "Port 8080 is in use; using $free instead." -ForegroundColor Yellow
    $port = $free
    $env:APP_ADDR = "127.0.0.1:$port"
    $bindHost = '127.0.0.1'
    $checkHost = '127.0.0.1'
    $baseUrl = "http://127.0.0.1:$port"
}

# --- Run --------------------------------------------------------------------
Write-Host ""
Write-Host "Chinfon Kaizen 360 v3" -ForegroundColor Cyan
Write-Host "Bind: $bindHost`:$port" -ForegroundColor Cyan
Write-Host "Local: $baseUrl" -ForegroundColor Cyan
if ($bindHost -eq '0.0.0.0' -or $bindHost -eq '::') {
    Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
        ForEach-Object { Write-Host "LAN:  http://$($_.IPAddress):$port  ($($_.InterfaceAlias))" -ForegroundColor Cyan }
}
Write-Host "Data: $env:APP_DATA_DIR (demo seed: $env:APP_SEED_DEMO)" -ForegroundColor DarkGray
Write-Host "Open: $baseUrl/  (first run: $baseUrl/setup)" -ForegroundColor Yellow
Write-Host "MFA is required after the first password change." -ForegroundColor Yellow
Write-Host "Stop: Ctrl+C" -ForegroundColor DarkGray
Write-Host ""

& go run ./cmd/kaizen
$code = $LASTEXITCODE
if ($code -ne 0) {
    Write-Host ""
    Write-Host "v3 stopped with exit code $code." -ForegroundColor Red
    Write-Host "Run 'go build ./...' in this folder for full error details." -ForegroundColor Yellow
}
exit $code
