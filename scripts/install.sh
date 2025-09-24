#!/usr/bin/env bash
set -euo pipefail
REPO="<ORG>/<REPO>"
BIN="deespec"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
[ "$ARCH" = "x86_64" ] && ARCH="amd64"
[ "$ARCH" = "aarch64" ] && ARCH="arm64"

TAG="$(curl -fsSL https://api.github.com/repos/${REPO}/releases/latest | grep -oE '"tag_name":\s*"[^"]+"' | cut -d'"' -f4)"
ASSET="${BIN}_${OS}_${ARCH}"

TMP="$(mktemp -d)"
curl -fsSL -o "${TMP}/${ASSET}" "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
chmod +x "${TMP}/${ASSET}"

# /usr/local/bin に置けない環境もあるのでフォールバック
DEST="/usr/local/bin/${BIN}"
if [ -w "/usr/local/bin" ]; then
  mv "${TMP}/${ASSET}" "${DEST}"
else
  mkdir -p "${HOME}/.local/bin"
  mv "${TMP}/${ASSET}" "${HOME}/.local/bin/${BIN}"
  DEST="${HOME}/.local/bin/${BIN}"
  case ":$PATH:" in
    *":${HOME}/.local/bin:"*) :;;
    *) echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc" 2>/dev/null || true ;;
  esac
fi
echo "Installed: ${DEST}"
"${DEST}" --help || true
