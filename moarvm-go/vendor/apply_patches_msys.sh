#!/usr/bin/env bash
set -e

echo "=== MoarVM Windows MSYS2 / MinGW UCRT64 Build Helper ==="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOAR_DIR="$SCRIPT_DIR/MoarVM"
BUILD_DIR="$SCRIPT_DIR/../build/moarvm"

if [ ! -d "$MOAR_DIR" ]; then
    echo "Cloning MoarVM source..."
    git clone https://github.com/MoarVM/MoarVM.git "$MOAR_DIR"
fi

cd "$MOAR_DIR"

echo "Configuring MoarVM for MinGW32 / UCRT64..."
perl Configure.pl --os=mingw32 --compiler=gcc --prefix="$BUILD_DIR"

echo "Building MoarVM..."
make -j8

echo "Installing MoarVM artifacts..."
make install

echo "=== MoarVM build and install complete! ==="
