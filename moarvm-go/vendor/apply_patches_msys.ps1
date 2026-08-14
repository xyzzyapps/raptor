# MoarVM Windows MSYS2 / MinGW UCRT64 Build & Patch Script (PowerShell)
$ErrorActionPreference = "Stop"

Write-Host "=== MoarVM Windows MSYS2 / MinGW UCRT64 Build Helper ===" -ForegroundColor Cyan

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$bashExe = "C:\msys64\usr\bin\bash.exe"

if (-not (Test-Path $bashExe)) {
    Write-Error "MSYS2 bash.exe not found at $bashExe. Please ensure MSYS2 is installed at C:\msys64."
    exit 1
}

# Ensure UCRT64 and MSYS2 tools are in PATH
$env:PATH = "C:\msys64\ucrt64\bin;C:\msys64\usr\bin;$env:PATH"

# Run the bash build script
$scriptBashPath = ($scriptDir -replace '\\', '/').Replace('C:', '/c') + "/apply_patches_msys.sh"

& $bashExe -lc "export PATH=/ucrt64/bin:/usr/bin:`$PATH; bash '$scriptBashPath'"

if ($LASTEXITCODE -ne 0) {
    Write-Error "MoarVM build failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Host "=== MoarVM build and deployment successful ===" -ForegroundColor Green
