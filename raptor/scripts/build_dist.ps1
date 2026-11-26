# ==============================================================================
# Raptor Multi-Platform Release & Distribution Pipeline (Powered by `zig cc`)
# ==============================================================================
# Cross-compiles and packages production release bundles using `zig cc`:
#   1. Windows x86_64 ZIP (`zig cc -target x86_64-windows-gnu`, CGO_ENABLED=1 + AVX2 + DLLs)
#   2. Windows arm64 ZIP  (`zig cc -target aarch64-windows-gnu`, CGO_ENABLED=1)
#   3. Linux x86_64 Musl Static TAR.GZ & ZIP (`zig cc -target x86_64-linux-musl`, CGO_ENABLED=1 + AVX2)
#   4. Linux arm64 Musl Static TAR.GZ (`zig cc -target aarch64-linux-musl`, CGO_ENABLED=1)
#   5. macOS arm64 & x86_64 TAR.GZ bundles
#   6. WebAssembly Interactive Tour bundle (`raptor.wasm` + `wasm_exec.js` + `web/`)
#   7. SHA256 Checksums (`dist/checksums.sha256`)
# ==============================================================================

param (
    [string]$Version = "v1.3",
    [ValidateSet("All", "Windows", "Linux", "MacOS", "Wasm")]
    [string]$Target = "All",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

Set-Location $RootDir

$HasZig = [bool](Get-Command zig -ErrorAction SilentlyContinue)
if ($HasZig) {
    $ZigVer = (& zig version)
    Write-Host "==================================================================" -ForegroundColor Cyan
    Write-Host "  Raptor Distribution Pipeline ($Version) — zig cc ($ZigVer)" -ForegroundColor Cyan
    Write-Host "==================================================================" -ForegroundColor Cyan
} else {
    Write-Warning "zig not found on PATH; falling back to pure-Go cross-compilation where needed."
}

# 0. Test Suite Validation
if (-not $SkipTests) {
    Write-Host "`n[0/6] Running Validation Test Suite..." -ForegroundColor Yellow
    go test -mod=mod ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Tests failed! Aborting distribution build."
    }
    Write-Host "  -> All test suites passed." -ForegroundColor Green
}

$DistDir = Join-Path $RootDir "dist"
$StagingDir = Join-Path $RootDir "release"

if (-not (Test-Path $DistDir)) { New-Item -ItemType Directory -Path $DistDir | Out-Null }
if ($Target -eq "All") {
    Get-ChildItem -Path $DistDir -File -ErrorAction SilentlyContinue | Remove-Item -Force
}
if (Test-Path $StagingDir) { Remove-Item -Path $StagingDir -Recurse -Force }
New-Item -ItemType Directory -Path $StagingDir | Out-Null

function Copy-CommonBundleAssets {
    param([string]$Dest)
    Copy-Item "lib" (Join-Path $Dest "lib") -Recurse -Force
    Copy-Item "examples" (Join-Path $Dest "examples") -Recurse -Force
    Copy-Item "docs" (Join-Path $Dest "docs") -Recurse -Force
    foreach ($doc in @("README.md", "SPEC.md", "BENCH.md", "RAKU_PERL5_SUBSET.md", "LICENSE")) {
        if (Test-Path $doc) { Copy-Item $doc $Dest -Force }
    }
}

function Clear-BuildEnv {
    foreach ($k in @("GOOS", "GOARCH", "CGO_ENABLED", "CC", "CXX", "CGO_LDFLAGS")) {
        Remove-Item "Env:$k" -ErrorAction SilentlyContinue
    }
}

# ------------------------------------------------------------------------------
# 1. Windows x86_64 & arm64 Distribution Bundles (`zig cc`)
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Windows") {
    Write-Host "`n[1/6] Building Windows x86_64 & arm64 Release Bundles with zig cc..." -ForegroundColor Yellow

    # --- Windows x86_64 ---
    $WinX64 = Join-Path $StagingDir "windows-x86_64"
    New-Item -ItemType Directory -Path $WinX64 | Out-Null
    try {
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        if ($HasZig) {
            $env:CGO_ENABLED = "1"
            $env:CC = "zig cc -target x86_64-windows-gnu"
            $env:CXX = "zig c++ -target x86_64-windows-gnu"
        }
        go build -mod=mod -ldflags="-s -w" -o (Join-Path $WinX64 "raptor.exe") .\cmd\raptor
        if ($LASTEXITCODE -ne 0) { throw "windows/amd64 raptor build failed" }
        go build -mod=mod -ldflags="-s -w" -o (Join-Path $WinX64 "raptorhp.exe") .\cmd\raptorhp
        if ($LASTEXITCODE -ne 0) { throw "windows/amd64 raptorhp build failed" }
        go build -mod=mod -ldflags="-s -w" -o (Join-Path $WinX64 "esp32_raptor.exe") .\cmd\esp32
        if ($LASTEXITCODE -ne 0) { throw "windows/amd64 esp32_raptor build failed" }
    } finally {
        Clear-BuildEnv
    }

    foreach ($dll in @("bin\moar.dll", "bin\libraylib.dll", "bin\sqlite3.dll")) {
        if (Test-Path $dll) { Copy-Item $dll $WinX64 -Force }
    }
    Copy-CommonBundleAssets -Dest $WinX64

    $WinX64Zip = Join-Path $DistDir "raptor-$Version-windows-x86_64.zip"
    if (Test-Path $WinX64Zip) { Remove-Item $WinX64Zip -Force }
    Compress-Archive -Path "$WinX64\*" -DestinationPath $WinX64Zip -Force
    Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $WinX64Zip -Leaf), ((Get-Item $WinX64Zip).Length / 1MB)) -ForegroundColor Green

    # --- Windows arm64 ---
    if ($HasZig) {
        $WinArm64 = Join-Path $StagingDir "windows-arm64"
        New-Item -ItemType Directory -Path $WinArm64 | Out-Null
        try {
            $env:GOOS = "windows"
            $env:GOARCH = "arm64"
            $env:CGO_ENABLED = "1"
            $env:CC = "zig cc -target aarch64-windows-gnu"
            $env:CXX = "zig c++ -target aarch64-windows-gnu"
            go build -mod=mod -ldflags="-s -w" -o (Join-Path $WinArm64 "raptor.exe") .\cmd\raptor
            if ($LASTEXITCODE -ne 0) { throw "windows/arm64 raptor build failed" }
            go build -mod=mod -ldflags="-s -w" -o (Join-Path $WinArm64 "raptorhp.exe") .\cmd\raptorhp
            if ($LASTEXITCODE -ne 0) { throw "windows/arm64 raptorhp build failed" }
            go build -mod=mod -ldflags="-s -w" -o (Join-Path $WinArm64 "esp32_raptor.exe") .\cmd\esp32
            if ($LASTEXITCODE -ne 0) { throw "windows/arm64 esp32_raptor build failed" }
        } finally {
            Clear-BuildEnv
        }
        Copy-CommonBundleAssets -Dest $WinArm64
        $WinArm64Zip = Join-Path $DistDir "raptor-$Version-windows-arm64.zip"
        if (Test-Path $WinArm64Zip) { Remove-Item $WinArm64Zip -Force }
        Compress-Archive -Path "$WinArm64\*" -DestinationPath $WinArm64Zip -Force
        Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $WinArm64Zip -Leaf), ((Get-Item $WinArm64Zip).Length / 1MB)) -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# 2. Linux x86_64 & arm64 Musl Static Bundles (`zig cc`)
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Linux") {
    Write-Host "`n[2/6] Building Linux x86_64 & arm64 Musl Static Bundles with zig cc..." -ForegroundColor Yellow

    # --- Linux x86_64 musl static ---
    $LinX64 = Join-Path $StagingDir "linux-x86_64"
    New-Item -ItemType Directory -Path $LinX64 | Out-Null
    try {
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        if ($HasZig) {
            $env:CGO_ENABLED = "1"
            $env:CC = "zig cc -target x86_64-linux-musl"
            $env:CXX = "zig c++ -target x86_64-linux-musl"
            $ldFlags = '-s -w -linkmode external -extldflags "-static"'
        } else {
            $env:CGO_ENABLED = "0"
            $ldFlags = '-s -w'
        }
        go build -mod=mod -ldflags="$ldFlags" -o (Join-Path $LinX64 "raptor") .\cmd\raptor
        if ($LASTEXITCODE -ne 0) { throw "linux/amd64 raptor build failed" }
        go build -mod=mod -ldflags="$ldFlags" -o (Join-Path $LinX64 "raptorhp") .\cmd\raptorhp
        if ($LASTEXITCODE -ne 0) { throw "linux/amd64 raptorhp build failed" }
        go build -mod=mod -ldflags="$ldFlags" -o (Join-Path $LinX64 "esp32_raptor") .\cmd\esp32
        if ($LASTEXITCODE -ne 0) { throw "linux/amd64 esp32_raptor build failed" }
    } finally {
        Clear-BuildEnv
    }
    Copy-CommonBundleAssets -Dest $LinX64

    $LinX64Tar = Join-Path $DistDir "raptor-$Version-linux-x86_64-musl-static.tar.gz"
    $LinX64Zip = Join-Path $DistDir "raptor-$Version-linux-x86_64-musl-static.zip"
    if (Test-Path $LinX64Tar) { Remove-Item $LinX64Tar -Force }
    if (Test-Path $LinX64Zip) { Remove-Item $LinX64Zip -Force }
    tar -czf $LinX64Tar -C $LinX64 .
    Compress-Archive -Path "$LinX64\*" -DestinationPath $LinX64Zip -Force
    Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $LinX64Tar -Leaf), ((Get-Item $LinX64Tar).Length / 1MB)) -ForegroundColor Green
    Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $LinX64Zip -Leaf), ((Get-Item $LinX64Zip).Length / 1MB)) -ForegroundColor Green

    # --- Linux arm64 musl static ---
    if ($HasZig) {
        $LinArm64 = Join-Path $StagingDir "linux-arm64"
        New-Item -ItemType Directory -Path $LinArm64 | Out-Null
        try {
            $env:GOOS = "linux"
            $env:GOARCH = "arm64"
            $env:CGO_ENABLED = "1"
            $env:CC = "zig cc -target aarch64-linux-musl"
            $env:CXX = "zig c++ -target aarch64-linux-musl"
            $ldFlags = '-s -w -linkmode external -extldflags "-static"'
            go build -mod=mod -ldflags="$ldFlags" -o (Join-Path $LinArm64 "raptor") .\cmd\raptor
            if ($LASTEXITCODE -ne 0) { throw "linux/arm64 raptor build failed" }
            go build -mod=mod -ldflags="$ldFlags" -o (Join-Path $LinArm64 "raptorhp") .\cmd\raptorhp
            if ($LASTEXITCODE -ne 0) { throw "linux/arm64 raptorhp build failed" }
            go build -mod=mod -ldflags="$ldFlags" -o (Join-Path $LinArm64 "esp32_raptor") .\cmd\esp32
            if ($LASTEXITCODE -ne 0) { throw "linux/arm64 esp32_raptor build failed" }
        } finally {
            Clear-BuildEnv
        }
        Copy-CommonBundleAssets -Dest $LinArm64
        $LinArm64Tar = Join-Path $DistDir "raptor-$Version-linux-arm64-musl-static.tar.gz"
        if (Test-Path $LinArm64Tar) { Remove-Item $LinArm64Tar -Force }
        tar -czf $LinArm64Tar -C $LinArm64 .
        Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $LinArm64Tar -Leaf), ((Get-Item $LinArm64Tar).Length / 1MB)) -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# 3. macOS arm64 & x86_64 Bundles
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "MacOS") {
    Write-Host "`n[3/6] Building macOS arm64 & x86_64 Release Bundles..." -ForegroundColor Yellow
    foreach ($archPair in @(@("arm64", "arm64"), @("amd64", "x86_64"))) {
        $goArch = $archPair[0]
        $labelArch = $archPair[1]
        $MacDir = Join-Path $StagingDir "macos-$labelArch"
        New-Item -ItemType Directory -Path $MacDir | Out-Null
        try {
            $env:GOOS = "darwin"
            $env:GOARCH = $goArch
            $env:CGO_ENABLED = "0"
            go build -mod=mod -ldflags="-s -w" -o (Join-Path $MacDir "raptor") .\cmd\raptor
            if ($LASTEXITCODE -ne 0) { throw "darwin/$goArch raptor build failed" }
            go build -mod=mod -ldflags="-s -w" -o (Join-Path $MacDir "raptorhp") .\cmd\raptorhp
            if ($LASTEXITCODE -ne 0) { throw "darwin/$goArch raptorhp build failed" }
            go build -mod=mod -ldflags="-s -w" -o (Join-Path $MacDir "esp32_raptor") .\cmd\esp32
            if ($LASTEXITCODE -ne 0) { throw "darwin/$goArch esp32_raptor build failed" }
        } finally {
            Clear-BuildEnv
        }
        Copy-CommonBundleAssets -Dest $MacDir
        $MacTar = Join-Path $DistDir "raptor-$Version-macos-$labelArch.tar.gz"
        if (Test-Path $MacTar) { Remove-Item $MacTar -Force }
        tar -czf $MacTar -C $MacDir .
        Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $MacTar -Leaf), ((Get-Item $MacTar).Length / 1MB)) -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# 4. WebAssembly Tour Payload (`web/raptor.wasm` + `dist/raptor-$Version-wasm-tour.zip`)
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Wasm") {
    Write-Host "`n[4/6] Building WebAssembly Tour Payload..." -ForegroundColor Yellow
    try {
        $env:GOOS = "js"
        $env:GOARCH = "wasm"
        $env:CGO_ENABLED = "0"
        go build -mod=mod -ldflags="-s -w" -o "web\raptor.wasm" .\cmd\wasm
        if ($LASTEXITCODE -ne 0) { throw "js/wasm build failed" }
    } finally {
        Clear-BuildEnv
    }

    $goRoot = (& go env GOROOT).Trim()
    foreach ($cand in @(
        (Join-Path $goRoot "lib\wasm\wasm_exec.js"),
        (Join-Path $goRoot "misc\wasm\wasm_exec.js")
    )) {
        if (Test-Path $cand) {
            Copy-Item $cand "web\wasm_exec.js" -Force
            break
        }
    }

    $WasmZip = Join-Path $DistDir "raptor-$Version-wasm-tour.zip"
    if (Test-Path $WasmZip) { Remove-Item $WasmZip -Force }
    Compress-Archive -Path "web\*" -DestinationPath $WasmZip -Force
    Write-Host ("  -> Updated web/raptor.wasm & created dist/{0} ({1:N2} MB)" -f (Split-Path $WasmZip -Leaf), ((Get-Item $WasmZip).Length / 1MB)) -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 5. Cleanup Staging & Generate SHA256 Checksums
# ------------------------------------------------------------------------------
if (Test-Path $StagingDir) { Remove-Item -Path $StagingDir -Recurse -Force }

Write-Host "`n[5/6] Generating Cryptographic Checksums (dist/checksums.sha256)..." -ForegroundColor Yellow
$checksums = @()
Get-ChildItem -Path $DistDir -File | Where-Object { $_.Name -ne "checksums.sha256" } | Sort-Object Name | ForEach-Object {
    $hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
    $checksums += "$hash  $($_.Name)"
}

$checksumFile = Join-Path $DistDir "checksums.sha256"
[System.IO.File]::WriteAllLines($checksumFile, $checksums, [System.Text.Encoding]::ASCII)
Write-Host "  -> Written dist/checksums.sha256" -ForegroundColor Green

Write-Host "`n==================================================================" -ForegroundColor Green
Write-Host "  Distribution Release Artifacts Ready in dist/" -ForegroundColor Green
Write-Host "==================================================================" -ForegroundColor Green
Get-ChildItem -Path $DistDir | Format-Table Name, Length, LastWriteTime
