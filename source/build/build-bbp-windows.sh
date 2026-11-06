#!/usr/bin/env bash
# Rebuilds the copy of bbpPairings embedded in the Windows executable, by
# cross-compiling the official sources with MinGW-w64 (static, no DLL).
#   apt install g++-mingw-w64-x86-64-posix
set -euo pipefail
cd "$(dirname "$0")/.."
WORK=$(mktemp -d)
git clone --depth 1 https://github.com/BieremaBoyzProgramming/bbpPairings.git "$WORK/bbp"
make -C "$WORK/bbp" -j"$(nproc)" CXX=x86_64-w64-mingw32-g++ static=yes
cp "$WORK/bbp/bbpPairings.exe" backend/internal/bbp/assets/bbpPairings-windows-amd64.exe
REV=$(git -C "$WORK/bbp" log -1 --format=%h)
sed -i "s/^const embeddedVersion = .*/const embeddedVersion = \"master@${REV}\"/" backend/internal/bbp/version.go
echo "bbpPairings master@${REV} intégré."
