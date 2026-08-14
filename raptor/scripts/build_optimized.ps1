# ==============================================================================
# Raptor Binary Size Optimization & Build Automation Pipeline
# ==============================================================================

param (
    [switch]$WasmOnly,
    [switch]$NativeOnly,
    [switch]$Esp32Only
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

Set-Location $RootDir

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "  Raptor Binary Size Optimization & Compilation Suite" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# 1. Native CLI Binary Optimization (Stripped DWARF & Symbols)
if (-not $WasmOnly -and -not $Esp32Only) {
    Write-Host "`n[1/3] Building Optimized Native CLI (raptor.exe)..." -ForegroundColor Yellow
    if (-not (Test-Path "bin")) { New-Item -ItemType Directory -Path "bin" | Out-Null }
    
    go build -ldflags="-s -w" -o bin\raptor.exe .\cmd\raptor
    $nativeSize = (Get-Item bin\raptor.exe).Length / 1MB
    Write-Host "  -> bin\raptor.exe built successfully ($([math]::Round($nativeSize, 2)) MB)" -ForegroundColor Green
}

# 2. WebAssembly Optimization & Compression
if (-not $NativeOnly -and -not $Esp32Only) {
    Write-Host "`n[2/3] Building Optimized WebAssembly Payload (web\raptor.wasm)..." -ForegroundColor Yellow
    $env:GOOS = "js"
    $env:GOARCH = "wasm"
    go build -ldflags="-s -w" -o web\raptor.wasm .\cmd\wasm
    Remove-Item Env:GOOS, Env:GOARCH
    
    $wasmSize = (Get-Item web\raptor.wasm).Length / 1MB
    Write-Host "  -> web\raptor.wasm built successfully ($([math]::Round($wasmSize, 2)) MB)" -ForegroundColor Green

    # Generate Gzip compressed asset if gzip is available
    if (Get-Command gzip -ErrorAction SilentlyContinue) {
        Write-Host "  -> Generating Gzip compressed asset (web\raptor.wasm.gz)..." -ForegroundColor Gray
        gzip -9 -k -f web\raptor.wasm
        $gzSize = (Get-Item web\raptor.wasm.gz).Length / 1KB
        Write-Host "  -> web\raptor.wasm.gz created ($([math]::Round($gzSize, 1)) KB)" -ForegroundColor Green
    }
}

# 3. ESP32 Microcontroller Firmware Target
if (-not $WasmOnly -and -not $NativeOnly) {
    Write-Host "`n[3/3] Building ESP32 Microcontroller Target..." -ForegroundColor Yellow
    go build -ldflags="-s -w" -tags="embedded" -o bin\esp32_raptor.exe .\cmd\esp32
    $espSize = (Get-Item bin\esp32_raptor.exe).Length / 1MB
    Write-Host "  -> bin\esp32_raptor.exe built successfully ($([math]::Round($espSize, 2)) MB)" -ForegroundColor Green

    Write-Host "`n  TinyGo Flash Command for ESP32 (when connected via USB/Serial):" -ForegroundColor Cyan
    Write-Host "    tinygo flash -target=esp32 --port=/dev/ttyUSB0 ./cmd/esp32" -ForegroundColor Gray
    Write-Host "    tinygo flash -target=esp32s3 ./cmd/esp32" -ForegroundColor Gray
}

Write-Host "`n==================================================================" -ForegroundColor Green
Write-Host "  All Optimized Builds Completed Successfully!" -ForegroundColor Green
Write-Host "==================================================================" -ForegroundColor Green
