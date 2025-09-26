#!/usr/bin/env bash
set -euo pipefail

# DeeSpec installer for Linux/macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.sh | bash

REPO="YoshitsuguKoike/deespec"
BIN="deespec"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print colored messages
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# Detect OS and Architecture
info "Detecting platform..."
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Normalize architecture names
case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        error "Unsupported architecture: $ARCH"
        ;;
esac

# Validate OS
case "$OS" in
    linux|darwin)
        info "Platform: ${OS}_${ARCH}"
        ;;
    *)
        error "Unsupported operating system: $OS"
        ;;
esac

# Get latest release tag
info "Fetching latest release..."
TAG="$(curl -fsSL https://api.github.com/repos/${REPO}/releases/latest 2>/dev/null | grep -oE '"tag_name":\s*"[^"]+"' | cut -d'"' -f4)" || true

if [ -z "$TAG" ]; then
    error "Failed to fetch latest release. Please check your internet connection."
fi

info "Latest version: $TAG"

# Construct asset name
ASSET="${BIN}_${OS}_${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

# Create temp directory
TMP="$(mktemp -d)"
trap "rm -rf ${TMP}" EXIT

# Download binary
info "Downloading ${ASSET}..."
if ! curl -fsSL -o "${TMP}/${BIN}" "${DOWNLOAD_URL}"; then
    error "Failed to download from ${DOWNLOAD_URL}"
fi

chmod +x "${TMP}/${BIN}"

# Determine installation directory
DEST="/usr/local/bin/${BIN}"
if [ -w "/usr/local/bin" ]; then
    info "Installing to /usr/local/bin..."
    mv "${TMP}/${BIN}" "${DEST}"
else
    # Fallback to ~/.local/bin
    DEST="${HOME}/.local/bin/${BIN}"
    mkdir -p "${HOME}/.local/bin"
    info "Installing to ~/.local/bin..."
    mv "${TMP}/${BIN}" "${DEST}"

    # Update PATH in shell rc files
    PATH_LINE='export PATH="$HOME/.local/bin:$PATH"'
    PATH_UPDATED=false

    # Update .bashrc if exists
    if [ -f "$HOME/.bashrc" ]; then
        if ! grep -q "\$HOME/.local/bin" "$HOME/.bashrc"; then
            info "Adding ~/.local/bin to PATH in .bashrc"
            echo "" >> "$HOME/.bashrc"
            echo "# Added by deespec installer" >> "$HOME/.bashrc"
            echo "$PATH_LINE" >> "$HOME/.bashrc"
            PATH_UPDATED=true
        else
            info "PATH already configured in .bashrc"
        fi
    fi

    # Update .zshrc if exists
    if [ -f "$HOME/.zshrc" ]; then
        if ! grep -q "\$HOME/.local/bin" "$HOME/.zshrc"; then
            info "Adding ~/.local/bin to PATH in .zshrc"
            echo "" >> "$HOME/.zshrc"
            echo "# Added by deespec installer" >> "$HOME/.zshrc"
            echo "$PATH_LINE" >> "$HOME/.zshrc"
            PATH_UPDATED=true
        else
            info "PATH already configured in .zshrc"
        fi
    fi

    # Update current session
    export PATH="$HOME/.local/bin:$PATH"

    # Inform user about PATH updates
    if [ "$PATH_UPDATED" = true ]; then
        info "✅ PATH has been automatically configured in your shell config files"
    fi
fi

echo ""
info "✅ Successfully installed ${BIN} to ${DEST}"
echo ""

# Verify installation
if command -v "${BIN}" &> /dev/null; then
    info "Running '${BIN} --help' to verify installation:"
    echo ""
    "${BIN}" --help || true
else
    warn "${BIN} is installed but not in PATH yet."
    echo ""

    # Detect current shell and provide specific instructions
    CURRENT_SHELL="$(basename "$SHELL")"
    echo "To use ${BIN} immediately, run one of these commands:"
    echo ""

    case "$CURRENT_SHELL" in
        zsh)
            echo "  source ~/.zshrc"
            ;;
        bash)
            echo "  source ~/.bashrc"
            ;;
        *)
            echo "  source ~/.bashrc  # for bash"
            echo "  source ~/.zshrc   # for zsh"
            ;;
    esac

    echo ""
    echo "Or simply open a new terminal window."
    echo ""
    echo "After that, you can run:"
    echo "  ${BIN} init"
fi