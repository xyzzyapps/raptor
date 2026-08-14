# ==============================================================================
# Raptor Multi-Platform Release & Distribution Automation Pipeline
# ==============================================================================
# Builds and packages production release bundles:
#   1. Windows x86_64 ZIP bundle with all runtime DLLs (moar, raylib, sqlite3)
#   2. Linux x86_64 Musl Static TAR.GZ bundle via WSL Alpine / Musl toolchain
#   3. WebAssembly TinyGo in-browser distribution payload
#   4. SHA256 Checksums file for release verification
# ==============================================================================

param (
    [string]$Version = "v1.0.0",
    [ValidateSet("All", "Windows", "Linux", "Wasm")]
    [string]$Target = "All",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

Set-Location $RootDir

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "  Raptor Automated Distribution Pipeline ($Version)" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# 0. Test Suite Validation
if (-not $SkipTests) {
    Write-Host "`n[0/4] Running Validation Test Suite..." -ForegroundColor Yellow
    $testResult = go test ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Tests failed! Aborting distribution build."
    }
    Write-Host "  -> All test suites passed successfully (100% OK)." -ForegroundColor Green
}

# Ensure dist and staging directories exist
$DistDir = Join-Path $RootDir "dist"
$StagingDir = Join-Path $RootDir "release"
$WinStaging = Join-Path $StagingDir "windows"
$LinuxStaging = Join-Path $StagingDir "linux"

if (-not (Test-Path $DistDir)) { New-Item -ItemType Directory -Path $DistDir | Out-Null }
if (Test-Path $StagingDir) { Remove-Item -Path $StagingDir -Recurse -Force }
New-Item -ItemType Directory -Path $WinStaging | Out-Null
New-Item -ItemType Directory -Path $LinuxStaging | Out-Null

# ------------------------------------------------------------------------------
# 1. Windows x86_64 Distribution Bundle
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Windows") {
    Write-Host "`n[1/4] Building Windows x86_64 Release Bundle..." -ForegroundColor Yellow
    
    # Compile Windows executables
    go build -ldflags="-s -w" -o (Join-Path $WinStaging "raptor.exe") .\cmd\raptor
    go build -ldflags="-s -w" -o (Join-Path $WinStaging "raptorhp.exe") .\cmd\raptorhp

    # Bundle runtime DLLs
    if (Test-Path "bin\moar.dll") { Copy-Item "bin\moar.dll" $WinStaging -Force }
    if (Test-Path "bin\libraylib.dll") { Copy-Item "bin\libraylib.dll" $WinStaging -Force }
    if (Test-Path "bin\sqlite3.dll") { Copy-Item "bin\sqlite3.dll" $WinStaging -Force }

    # Bundle standard libraries, examples, and documentation
    Copy-Item "lib" (Join-Path $WinStaging "lib") -Recurse -Force
    Copy-Item "examples" (Join-Path $WinStaging "examples") -Recurse -Force
    Copy-Item "README.md", "SPEC.md", "LICENSE" $WinStaging -Force

    # Create ZIP archive
    $WinZip = Join-Path $DistDir "raptor-$Version-windows-x86_64.zip"
    if (Test-Path $WinZip) { Remove-Item $WinZip -Force }
    Compress-Archive -Path "$WinStaging\*" -DestinationPath $WinZip -Force
    
    $winSize = (Get-Item $WinZip).Length / 1MB
    Write-Host "  -> Created: dist/raptor-$Version-windows-x86_64.zip ($([math]::Round($winSize, 2)) MB)" -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 2. Linux x86_64 Musl Static Distribution Bundle
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Linux") {
    Write-Host "`n[2/4] Building Linux x86_64 Musl Static Release Bundle..." -ForegroundColor Yellow
    
    $wslDistros = (wsl -l -q 2>$null)
    $hasAlpine = $wslDistros -match "Alpine"
    
    if ($hasAlpine) {
        Write-Host "  -> Compiling via WSL Alpine Linux (Native Musl Toolchain)..." -ForegroundColor Gray
        $wslRootDir = ($RootDir -replace "\\", "/").Replace("C:", "/mnt/c").Replace("c:", "/mnt/c")
        wsl -d Alpine sh -c "cd '$wslRootDir' && go build -ldflags='-s -w' -o release/linux/raptor ./cmd/raptor && go build -ldflags='-s -w' -o release/linux/raptorhp ./cmd/raptorhp"
    } else {
        Write-Host "  -> Cross-compiling static Linux binaries via Go toolchain..." -ForegroundColor Gray
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        go build -ldflags="-s -w -extldflags '-static'" -o (Join-Path $LinuxStaging "raptor") .\cmd\raptor
        go build -ldflags="-s -w -extldflags '-static'" -o (Join-Path $LinuxStaging "raptorhp") .\cmd\raptorhp
        Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
    }

    # Bundle standard libraries, examples, and documentation
    Copy-Item "lib" (Join-Path $LinuxStaging "lib") -Recurse -Force
    Copy-Item "examples" (Join-Path $LinuxStaging "examples") -Recurse -Force
    Copy-Item "README.md", "SPEC.md", "LICENSE" $LinuxStaging -Force

    # Create TAR.GZ archive
    $LinuxTarGz = Join-Path $DistDir "raptor-$Version-linux-x86_64-musl-static.tar.gz"
    $LinuxZip = Join-Path $DistDir "raptor-$Version-linux-x86_64-musl-static.zip"
    if (Test-Path $LinuxTarGz) { Remove-Item $LinuxTarGz -Force }
    if (Test-Path $LinuxZip) { Remove-Item $LinuxZip -Force }
    
    if (Get-Command tar -ErrorAction SilentlyContinue) {
        tar -czf $LinuxTarGz -C $LinuxStaging . | Out-Null
    } elseif ($hasAlpine) {
        $wslTarGz = ($LinuxTarGz -replace "\\", "/").Replace("C:", "/mnt/c").Replace("c:", "/mnt/c")
        $wslStaging = ($LinuxStaging -replace "\\", "/").Replace("C:", "/mnt/c").Replace("c:", "/mnt/c")
        wsl -d Alpine sh -c "tar -czf '$wslTarGz' -C '$wslStaging' ."
    }
    
    # Also create standard ZIP archive for Linux
    Compress-Archive -Path "$LinuxStaging\*" -DestinationPath $LinuxZip -Force
    
    if (Test-Path $LinuxTarGz) {
        $linuxTarSize = (Get-Item $LinuxTarGz).Length / 1MB
        Write-Host "  -> Created: dist/raptor-$Version-linux-x86_64-musl-static.tar.gz ($([math]::Round($linuxTarSize, 2)) MB)" -ForegroundColor Green
    }
    if (Test-Path $LinuxZip) {
        $linuxZipSize = (Get-Item $LinuxZip).Length / 1MB
        Write-Host "  -> Created: dist/raptor-$Version-linux-x86_64-musl-static.zip ($([math]::Round($linuxZipSize, 2)) MB)" -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# 3. WebAssembly TinyGo Payload
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Wasm") {
    Write-Host "`n[3/4] Building TinyGo WebAssembly Tour Payload..." -ForegroundColor Yellow
    if (Get-Command tinygo -ErrorAction SilentlyContinue) {
        # Find wasm-opt if available
        $opt = (Get-ChildItem -Path "$env:APPDATA\npm\node_modules\wasm-opt" -Filter "wasm-opt.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1).FullName
        if ($opt) { $env:WASMOPT = $opt }
        
        tinygo build -target=wasm -no-debug -o web\raptor.wasm .\cmd\wasm
        $wasmSize = (Get-Item web\raptor.wasm).Length / 1MB
        Write-Host "  -> Created: web/raptor.wasm ($([math]::Round($wasmSize, 2)) MB)" -ForegroundColor Green
    } else {
        Write-Host "  -> TinyGo not found in PATH, skipping WebAssembly build." -ForegroundColor DarkYellow
    }
}

# ------------------------------------------------------------------------------
# 4. Generate SHA256 Checksums
# ------------------------------------------------------------------------------
Write-Host "`n[4/4] Generating Cryptographic Checksums (dist/checksums.sha256)..." -ForegroundColor Yellow
$checksums = @()
Get-ChildItem -Path $DistDir -File | Where-Object { $_.Name -ne "checksums.sha256" } | ForEach-Object {
    $hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
    $checksums += "$hash  $($_.Name)"
}

$checksumFile = Join-Path $DistDir "checksums.sha256"
$checksums | Out-File -FilePath $checksumFile -Encoding ascii
Write-Host "  -> Written checksums.sha256" -ForegroundColor Green

Write-Host "`n==================================================================" -ForegroundColor Green
Write-Host "  Distribution Release Artifacts Built Successfully in dist/" -ForegroundColor Green
Write-Host "==================================================================" -ForegroundColor Green
Get-ChildItem -Path $DistDir | Format-Table Name, Length, LastWriteTime
