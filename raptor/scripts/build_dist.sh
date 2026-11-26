#!/usr/bin/env sh
# ==============================================================================
# Raptor Linux / Multi-Arch Static Release Distribution Script (`zig cc`)
# ==============================================================================

set -e

VERSION="${1:-v1.3}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

echo "=================================================================="
echo "  Raptor Linux Release Automation Pipeline ($VERSION)"
echo "=================================================================="

echo "[1/3] Running Test Suite..."
go test -mod=mod ./...

DIST_DIR="$ROOT_DIR/dist"
STAGING_DIR="$ROOT_DIR/release/linux"

mkdir -p "$DIST_DIR"
rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"

echo "[2/3] Compiling Static Musl Binaries..."
if command -v zig >/dev/null 2>&1; then
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" \
        go build -mod=mod -ldflags="-s -w -linkmode external -extldflags '-static'" -o "$STAGING_DIR/raptor" ./cmd/raptor
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" \
        go build -mod=mod -ldflags="-s -w -linkmode external -extldflags '-static'" -o "$STAGING_DIR/raptorhp" ./cmd/raptorhp
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" \
        go build -mod=mod -ldflags="-s -w -linkmode external -extldflags '-static'" -o "$STAGING_DIR/esp32_raptor" ./cmd/esp32
else
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -ldflags="-s -w" -o "$STAGING_DIR/raptor" ./cmd/raptor
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -ldflags="-s -w" -o "$STAGING_DIR/raptorhp" ./cmd/raptorhp
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -ldflags="-s -w" -o "$STAGING_DIR/esp32_raptor" ./cmd/esp32
fi

cp -r lib "$STAGING_DIR/lib"
cp -r examples "$STAGING_DIR/examples"
cp -r docs "$STAGING_DIR/docs"
cp README.md SPEC.md BENCH.md RAKU_PERL5_SUBSET.md LICENSE "$STAGING_DIR/"

TAR_FILE="$DIST_DIR/raptor-$VERSION-linux-x86_64-musl-static.tar.gz"
ZIP_FILE="$DIST_DIR/raptor-$VERSION-linux-x86_64-musl-static.zip"

tar -czf "$TAR_FILE" -C "$STAGING_DIR" .
if command -v zip >/dev/null 2>&1; then
    (cd "$STAGING_DIR" && zip -r "$ZIP_FILE" .)
fi
rm -rf "$ROOT_DIR/release"

echo "[3/3] Generating Checksums..."
cd "$DIST_DIR"
sha256sum raptor-"$VERSION"-* > "checksums.sha256"

echo "=================================================================="
echo "  Linux Release Artifacts Created in $DIST_DIR"
echo "=================================================================="
