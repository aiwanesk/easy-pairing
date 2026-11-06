#!/usr/bin/env bash
# Builds pairing_easy for Windows, macOS and Linux into dist/.
# Requirements: Go >= 1.27, Node >= 20. The Windows copy of bbpPairings is
# already in backend/internal/bbp/assets (see build-bbp-windows.sh).
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-$(date +%Y.%m.%d)}"

echo "==> Front-end"
(cd frontend && npm ci --no-audit --no-fund && npm run build)
rm -rf backend/internal/web/dist && cp -r frontend/dist backend/internal/web/dist

echo "==> Back-end (tests)"
(cd backend && go vet -mod=vendor ./... && go test -mod=vendor ./...)

mkdir -p dist
LDFLAGS="-s -w -X pairingeasy/internal/api.Version=${VERSION}"
build() { # GOOS GOARCH output
  echo "==> $1/$2"
  (cd backend && CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -mod=vendor -trimpath -ldflags "$LDFLAGS" -o "../dist/$3" ./cmd/pairing_easy)
}
build windows amd64 pairing_easy.exe
build darwin arm64 pairing_easy-macos-arm64
build darwin amd64 pairing_easy-macos-intel
build linux amd64 pairing_easy-linux-amd64
echo "Fait : dist/"
