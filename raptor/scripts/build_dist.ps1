# ==============================================================================
# Raptor Multi-Platform Release & Distribution Pipeline (Go + `zig cc` Builds)
# ==============================================================================
# Produces clean `-trimpath` release archives for BOTH toolchains:
#   A. Pure Go Toolchain (`CGO_ENABLED=0`, zero external C dependencies):
#      - raptor-$Version-windows-x86_64-go.zip
#      - raptor-$Version-windows-arm64-go.zip
#      - raptor-$Version-linux-x86_64-go-static.tar.gz & .zip
#      - raptor-$Version-linux-arm64-go-static.tar.gz
#      - raptor-$Version-macos-arm64-go.tar.gz
#      - raptor-$Version-macos-x86_64-go.tar.gz
#   B. `zig cc` CGO Toolchain (`CGO_ENABLED=1`, native C FFI + AVX2):
#      - raptor-$Version-windows-x86_64-zigcc.zip
#      - raptor-$Version-windows-arm64-zigcc.zip
#      - raptor-$Version-linux-x86_64-musl-static-zigcc.tar.gz & .zip
#      - raptor-$Version-linux-arm64-musl-static-zigcc.tar.gz
#   C. WebAssembly Interactive Tour (`-trimpath`):
#      - raptor-$Version-wasm-tour.zip (+ updates web/raptor.wasm & web/wasm_exec.js)
#   D. Cryptographic Checksums:
#      - checksums.sha256
# ==============================================================================

param (
    [string]$Version = "v1.3",
    [ValidateSet("All", "Go", "ZigCC", "Wasm")]
    [string]$Target = "All",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

Set-Location $RootDir

$HasZig = [bool](Get-Command zig -ErrorAction SilentlyContinue)
$ZigVer = if ($HasZig) { (& zig version) } else { "unavailable" }

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "  Raptor Distribution Pipeline ($Version) — Go + zig cc ($ZigVer)" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# 0. Test Suite Validation
if (-not $SkipTests) {
    Write-Host "`n[0/4] Running Validation Test Suite..." -ForegroundColor Yellow
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
    param([string]$Dest, [bool]$IncludeWinDLLs = $false)
    if ($IncludeWinDLLs) {
        foreach ($dll in @("bin\moar.dll", "bin\libraylib.dll", "bin\sqlite3.dll")) {
            if (Test-Path $dll) { Copy-Item $dll $Dest -Force }
        }
    }
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

function Build-PlatformBinaries {
    param(
        [string]$OutDir,
        [string]$GoOS,
        [string]$GoArch,
        [string]$CgoEnabled,
        [string]$CCCmd = "",
        [string]$CXXCmd = "",
        [string]$LdFlags = "-s -w"
    )
    $ext = if ($GoOS -eq "windows") { ".exe" } else { "" }
    New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
    try {
        $env:GOOS = $GoOS
        $env:GOARCH = $GoArch
        $env:CGO_ENABLED = $CgoEnabled
        if ($CCCmd -ne "") { $env:CC = $CCCmd }
        if ($CXXCmd -ne "") { $env:CXX = $CXXCmd }

        foreach ($pair in @(@("raptor", ".\cmd\raptor"), @("raptorhp", ".\cmd\raptorhp"), @("esp32_raptor", ".\cmd\esp32"))) {
            $binName = $pair[0] + $ext
            $pkgPath = $pair[1]
            go build -mod=mod -trimpath -ldflags="$LdFlags" -o (Join-Path $OutDir $binName) $pkgPath
            if ($LASTEXITCODE -ne 0) {
                throw "Build failed for $GoOS/$GoArch (cgo=$CgoEnabled): $binName"
            }
        }
    } finally {
        Clear-BuildEnv
    }
}

function Package-Archive {
    param(
        [string]$SourceDir,
        [string]$BaseName,
        [bool]$MakeZip = $false,
        [bool]$MakeTarGz = $false
    )
    if ($MakeTarGz) {
        $tarPath = Join-Path $DistDir "$BaseName.tar.gz"
        if (Test-Path $tarPath) { Remove-Item $tarPath -Force }
        tar -czf $tarPath -C $SourceDir .
        Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $tarPath -Leaf), ((Get-Item $tarPath).Length / 1MB)) -ForegroundColor Green
    }
    if ($MakeZip) {
        $zipPath = Join-Path $DistDir "$BaseName.zip"
        if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
        Compress-Archive -Path "$SourceDir\*" -DestinationPath $zipPath -Force
        Write-Host ("  -> Created: dist/{0} ({1:N2} MB)" -f (Split-Path $zipPath -Leaf), ((Get-Item $zipPath).Length / 1MB)) -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# 1. Pure Go Toolchain Builds (`CGO_ENABLED=0 -trimpath`)
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Go") {
    Write-Host "`n[1/4] Building Pure Go Release Bundles (CGO_ENABLED=0, -trimpath)..." -ForegroundColor Yellow

    $goMatrix = @(
        @{ OS = "windows"; Arch = "amd64"; Label = "windows-x86_64-go"; WinDLL = $true;  Zip = $true;  Tar = $false },
        @{ OS = "windows"; Arch = "arm64"; Label = "windows-arm64-go";  WinDLL = $false; Zip = $true;  Tar = $false },
        @{ OS = "linux";   Arch = "amd64"; Label = "linux-x86_64-go-static"; WinDLL = $false; Zip = $true; Tar = $true },
        @{ OS = "linux";   Arch = "arm64"; Label = "linux-arm64-go-static";  WinDLL = $false; Zip = $false; Tar = $true },
        @{ OS = "darwin";  Arch = "arm64"; Label = "macos-arm64-go";    WinDLL = $false; Zip = $false; Tar = $true },
        @{ OS = "darwin";  Arch = "amd64"; Label = "macos-x86_64-go";   WinDLL = $false; Zip = $false; Tar = $true }
    )

    foreach ($t in $goMatrix) {
        $stage = Join-Path $StagingDir $t.Label
        Build-PlatformBinaries -OutDir $stage -GoOS $t.OS -GoArch $t.Arch -CgoEnabled "0"
        Copy-CommonBundleAssets -Dest $stage -IncludeWinDLLs $t.WinDLL
        Package-Archive -SourceDir $stage -BaseName "raptor-$Version-$($t.Label)" -MakeZip $t.Zip -MakeTarGz $t.Tar
    }
}

# ------------------------------------------------------------------------------
# 2. `zig cc` CGO Toolchain Builds (`CGO_ENABLED=1 -trimpath`)
# ------------------------------------------------------------------------------
if (($Target -eq "All" -or $Target -eq "ZigCC") -and $HasZig) {
    Write-Host "`n[2/4] Building zig cc CGO Release Bundles (CGO_ENABLED=1, -trimpath)..." -ForegroundColor Yellow

    $zigMatrix = @(
        @{ OS = "windows"; Arch = "amd64"; ZigTarget = "x86_64-windows-gnu";  Label = "windows-x86_64-zigcc"; Ld = "-s -w"; WinDLL = $true;  Zip = $true;  Tar = $false },
        @{ OS = "windows"; Arch = "arm64"; ZigTarget = "aarch64-windows-gnu"; Label = "windows-arm64-zigcc";  Ld = "-s -w"; WinDLL = $false; Zip = $true;  Tar = $false },
        @{ OS = "linux";   Arch = "amd64"; ZigTarget = "x86_64-linux-musl";   Label = "linux-x86_64-musl-static-zigcc"; Ld = '-s -w -linkmode external -extldflags "-static"'; WinDLL = $false; Zip = $true; Tar = $true },
        @{ OS = "linux";   Arch = "arm64"; ZigTarget = "aarch64-linux-musl";  Label = "linux-arm64-musl-static-zigcc";  Ld = '-s -w -linkmode external -extldflags "-static"'; WinDLL = $false; Zip = $false; Tar = $true }
    )

    foreach ($t in $zigMatrix) {
        $stage = Join-Path $StagingDir $t.Label
        Build-PlatformBinaries -OutDir $stage -GoOS $t.OS -GoArch $t.Arch -CgoEnabled "1" `
            -CCCmd "zig cc -target $($t.ZigTarget)" -CXXCmd "zig c++ -target $($t.ZigTarget)" -LdFlags $t.Ld
        Copy-CommonBundleAssets -Dest $stage -IncludeWinDLLs $t.WinDLL
        Package-Archive -SourceDir $stage -BaseName "raptor-$Version-$($t.Label)" -MakeZip $t.Zip -MakeTarGz $t.Tar
    }
}

# ------------------------------------------------------------------------------
# 3. WebAssembly Tour Payload (`-trimpath`)
# ------------------------------------------------------------------------------
if ($Target -eq "All" -or $Target -eq "Wasm") {
    Write-Host "`n[3/4] Building WebAssembly Tour Payload (-trimpath)..." -ForegroundColor Yellow
    try {
        $env:GOOS = "js"
        $env:GOARCH = "wasm"
        $env:CGO_ENABLED = "0"
        go build -mod=mod -trimpath -ldflags="-s -w" -o "web\raptor.wasm" .\cmd\wasm
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
# 4. Cleanup Staging & Generate SHA256 Checksums
# ------------------------------------------------------------------------------
if (Test-Path $StagingDir) { Remove-Item -Path $StagingDir -Recurse -Force }

Write-Host "`n[4/4] Generating Cryptographic Checksums (dist/checksums.sha256)..." -ForegroundColor Yellow
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
