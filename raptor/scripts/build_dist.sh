#!/usr/bin/env sh
# ==============================================================================
# Raptor Linux Release Distribution Script (Go + `zig cc` `-trimpath` Builds)
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
mkdir -p "$DIST_DIR"
rm -rf "$ROOT_DIR/release"

bundle_common() {
    dest="$1"
    cp -r lib "$dest/lib"
    cp -r examples "$dest/examples"
    cp -r docs "$dest/docs"
    cp README.md SPEC.md BENCH.md RAKU_PERL5_SUBSET.md LICENSE "$dest/"
}

echo "[2/3] Compiling Pure Go Static Binaries (-trimpath)..."
GO_STAGE="$ROOT_DIR/release/linux-go"
mkdir -p "$GO_STAGE"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -trimpath -ldflags="-s -w" -o "$GO_STAGE/raptor" ./cmd/raptor
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -trimpath -ldflags="-s -w" -o "$GO_STAGE/raptorhp" ./cmd/raptorhp
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -trimpath -ldflags="-s -w" -o "$GO_STAGE/esp32_raptor" ./cmd/esp32
bundle_common "$GO_STAGE"
tar -czf "$DIST_DIR/raptor-$VERSION-linux-x86_64-go-static.tar.gz" -C "$GO_STAGE" .

if command -v zig >/dev/null 2>&1; then
    echo "Compiling zig cc Musl Static Binaries (-trimpath)..."
    ZIG_STAGE="$ROOT_DIR/release/linux-zigcc"
    mkdir -p "$ZIG_STAGE"
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" \
        go build -mod=mod -trimpath -ldflags="-s -w -linkmode external -extldflags '-static'" -o "$ZIG_STAGE/raptor" ./cmd/raptor
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" \
        go build -mod=mod -trimpath -ldflags="-s -w -linkmode external -extldflags '-static'" -o "$ZIG_STAGE/raptorhp" ./cmd/raptorhp
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" \
        go build -mod=mod -trimpath -ldflags="-s -w -linkmode external -extldflags '-static'" -o "$ZIG_STAGE/esp32_raptor" ./cmd/esp32
    bundle_common "$ZIG_STAGE"
    tar -czf "$DIST_DIR/raptor-$VERSION-linux-x86_64-musl-static-zigcc.tar.gz" -C "$ZIG_STAGE" .
fi

rm -rf "$ROOT_DIR/release"

echo "[3/3] Generating Checksums..."
cd "$DIST_DIR"
sha256sum raptor-"$VERSION"-* > "checksums.sha256"

echo "=================================================================="
echo "  Linux Release Artifacts Created in $DIST_DIR"
echo "=================================================================="
