#!/usr/bin/env bash
set -e

echo "=== MoarVM Windows MSYS2 / MinGW UCRT64 Build Helper ==="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOAR_DIR="$SCRIPT_DIR/MoarVM"
PATCH_FILE="$SCRIPT_DIR/patches/changes.diff"
BUILD_DIR="$SCRIPT_DIR/../build/moarvm"

if [ ! -d "$MOAR_DIR" ]; then
    echo "Cloning MoarVM source..."
    git clone https://github.com/MoarVM/MoarVM.git "$MOAR_DIR"
fi

cd "$MOAR_DIR"

echo "Initializing and updating MoarVM git submodules..."
git submodule update --init --recursive

if [ -f "$PATCH_FILE" ]; then
    echo "Checking patch status..."
    if git apply --check "$PATCH_FILE" >/dev/null 2>&1; then
        echo "Applying patches from $PATCH_FILE..."
        git apply "$PATCH_FILE"
        echo "Patches applied successfully."
    elif git apply --reverse --check "$PATCH_FILE" >/dev/null 2>&1; then
        echo "Patches from $PATCH_FILE are already applied."
    else
        echo "Warning: Patch check failed. Attempting git apply with whitespace ignore..."
        git apply --ignore-whitespace "$PATCH_FILE" || true
    fi
fi

echo "Configuring MoarVM for MinGW32 / UCRT64 (POSIX shell + MinGW32 toolchain)..."
perl Configure.pl --os=mingw32 --toolchain=mingw32 --shell=posix --prefix="$BUILD_DIR"

echo "Building MoarVM..."
make -j8

echo "Installing MoarVM artifacts..."
make install

echo "Deploying shared library to project bin directories..."
mkdir -p "$SCRIPT_DIR/../bin"
mkdir -p "$SCRIPT_DIR/../../raptor/bin"

if [ -f "$BUILD_DIR/bin/moar.dll" ]; then
    cp -v "$BUILD_DIR/bin/moar.dll" "$SCRIPT_DIR/../bin/moar.dll"
    cp -v "$BUILD_DIR/bin/moar.dll" "$SCRIPT_DIR/../../raptor/bin/moar.dll"
elif [ -f "$MOAR_DIR/moar.dll" ]; then
    cp -v "$MOAR_DIR/moar.dll" "$SCRIPT_DIR/../bin/moar.dll"
    cp -v "$MOAR_DIR/moar.dll" "$SCRIPT_DIR/../../raptor/bin/moar.dll"
fi

echo "=== MoarVM build, install, and deployment complete! ==="
