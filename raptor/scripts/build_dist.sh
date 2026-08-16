#!/usr/bin/env sh
# ==============================================================================
# Raptor Linux / Musl Static Release Distribution Script
# ==============================================================================

set -e

VERSION="${1:-v1.0.0}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

echo "=================================================================="
echo "  Raptor Linux Release Automation Pipeline ($VERSION)"
echo "=================================================================="

# 1. Run Tests
echo "\n[1/3] Running Test Suite..."
go test -mod=mod ./...

# 2. Build Static Binaries
DIST_DIR="$ROOT_DIR/dist"
STAGING_DIR="$ROOT_DIR/release/linux"

mkdir -p "$DIST_DIR"
rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"

echo "\n[2/3] Compiling Static Musl Binaries..."
CGO_ENABLED=0 go build -mod=mod -ldflags="-s -w -extldflags '-static'" -o "$STAGING_DIR/raptor" ./cmd/raptor
CGO_ENABLED=0 go build -mod=mod -ldflags="-s -w -extldflags '-static'" -o "$STAGING_DIR/raptorhp" ./cmd/raptorhp

cp -r lib "$STAGING_DIR/lib"
cp -r examples "$STAGING_DIR/examples"
cp README.md SPEC.md LICENSE "$STAGING_DIR/"

TAR_FILE="$DIST_DIR/raptor-$VERSION-linux-x86_64-musl-static.tar.gz"
ZIP_FILE="$DIST_DIR/raptor-$VERSION-linux-x86_64-musl-static.zip"

tar -czf "$TAR_FILE" -C "$STAGING_DIR" .
if command -v zip >/dev/null 2>&1; then
    (cd "$STAGING_DIR" && zip -r "$ZIP_FILE" .)
fi

# 3. Checksums
echo "\n[3/3] Generating Checksums..."
cd "$DIST_DIR"
sha256sum "$(basename "$TAR_FILE")" > "checksums.sha256"
if [ -f "$ZIP_FILE" ]; then
    sha256sum "$(basename "$ZIP_FILE")" >> "checksums.sha256"
fi

echo "=================================================================="
echo "  Linux Release Artifacts Created in $DIST_DIR"
echo "=================================================================="
