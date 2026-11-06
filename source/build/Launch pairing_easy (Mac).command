#!/bin/zsh
# Double-clic : compile bbpPairings une fois (outils Xcode requis) puis lance pairing_easy.
cd "$(dirname "$0")/.." || exit 1
DATA="$HOME/Library/Application Support/pairing_easy"
ENGINE="$DATA/engine/bbpPairings"
ARCH=$(uname -m); BIN="other-systems/pairing_easy-macos-intel"; [ "$ARCH" = "arm64" ] && BIN="other-systems/pairing_easy-macos-arm64"
if [ ! -x "$ENGINE" ]; then
  command -v clang++ >/dev/null || { echo "Installez les outils Xcode : xcode-select --install"; read -k1; exit 1; }
  echo "==> Compilation de bbpPairings (une seule fois)…"
  TMP=$(mktemp -d)
  git clone --depth 1 https://github.com/BieremaBoyzProgramming/bbpPairings.git "$TMP/bbp" || { read -k1; exit 1; }
  perl -i -ne 'print unless /^\t-Werror \\$/' "$TMP/bbp/Makefile"
  make -C "$TMP/bbp" COMP=clang -j4 || { echo "Échec de la compilation"; read -k1; exit 1; }
  mkdir -p "$DATA/engine" && cp "$TMP/bbp/bbpPairings.exe" "$ENGINE"
fi
chmod +x "$BIN"; xattr -d com.apple.quarantine "$BIN" 2>/dev/null
exec "./$BIN" "$@"
