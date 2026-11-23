param(
    [Parameter(Mandatory=$true)]
    [ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$')]
    [string]$Version
)
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    # Preserve build-win.ps1 for fast local builds. This is release packaging.
    $python = Get-Command python -ErrorAction SilentlyContinue
    if (-not $python) {
        throw 'Python 3.10+ is required by skills/go-single-binary/scripts/package_go.py'
    }
    & $python.Source 'skills/go-single-binary/scripts/package_go.py' build --project-root . --package ./cmd/kaizen --name kaizen --version $Version --target windows/amd64 --cgo 0
    if ($LASTEXITCODE -ne 0) {
        throw "Kaizen release packaging failed (exit code $LASTEXITCODE)."
    }
    Write-Host "Release ZIP and checksums: dist/kaizen_$($Version)_windows_amd64.zip"
} finally {
    Pop-Location
}
