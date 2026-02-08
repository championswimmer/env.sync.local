#!/bin/bash
# Installation script for env-sync (Go default, Bash legacy)

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
USER_INSTALL=false
INSTALL_PREFIX="/usr/local"
BIN_DIR="$INSTALL_PREFIX/bin"
LIB_DIR="$INSTALL_PREFIX/lib/env-sync"
LEGACY_DIR="$LIB_DIR/legacy"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --user)
            USER_INSTALL=true
            INSTALL_PREFIX="$HOME/.local"
            BIN_DIR="$INSTALL_PREFIX/bin"
            LIB_DIR="$INSTALL_PREFIX/lib/env-sync"
            LEGACY_DIR="$LIB_DIR/legacy"
            shift
            ;;
        --help)
            echo "Usage: install.sh [options]"
            echo "Options:"
            echo "  --user    Install to ~/.local (no sudo required)"
            echo "  --help    Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_BIN="$SCRIPT_DIR/target/env-sync"

echo -e "${BLUE}Installing env-sync (Go default)${NC}"

# Detect OS
OS=$(uname -s)

# Required dependency: Go
if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}Go is required to build env-sync. Please install Go 1.24 or newer.${NC}"
    exit 1
fi

echo "Go detected: $(go version)"

# Optional dependencies (legacy Bash support and troubleshooting)
echo "Checking optional dependencies..."
MISSING_DEPS=()

if ! command -v curl >/dev/null 2>&1; then
    MISSING_DEPS+=("curl")
fi

if ! command -v nc >/dev/null 2>&1 && ! command -v netcat >/dev/null 2>&1; then
    MISSING_DEPS+=("netcat (nc)")
fi

case "$OS" in
    Linux)
        if ! command -v avahi-browse >/dev/null 2>&1; then
            MISSING_DEPS+=("avahi-utils")
        fi
        ;;
    Darwin)
        if ! command -v dns-sd >/dev/null 2>&1; then
            MISSING_DEPS+=("dns-sd (built into macOS, install via Xcode CLI tools)")
        fi
        ;;
esac

if [[ ${#MISSING_DEPS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}Warning: Missing optional dependencies (needed for legacy Bash or troubleshooting):${NC}"
    printf '  - %s\n' "${MISSING_DEPS[@]}"
    echo ""
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Build Go binary
echo "Building Go binary..."
mkdir -p "$SCRIPT_DIR/target"
(
    cd "$SCRIPT_DIR/src"
    go build -o "$TARGET_BIN" ./cmd/env-sync
)

# Create directories
echo "Creating directories..."
mkdir -p "$BIN_DIR"
mkdir -p "$LIB_DIR"
mkdir -p "$LEGACY_DIR"

# Install Go binary
echo "Installing Go binary..."
cp "$TARGET_BIN" "$LIB_DIR/env-sync-go"
chmod +x "$LIB_DIR/env-sync-go"

# Install wrapper and shims
echo "Installing CLI wrappers..."
cp "$SCRIPT_DIR/bin/env-sync" "$BIN_DIR/env-sync"
chmod +x "$BIN_DIR/env-sync"
for cmd in env-sync-client env-sync-discover env-sync-serve env-sync-key env-sync-load; do
    ln -sf env-sync "$BIN_DIR/$cmd"
done
ln -sf ../lib/env-sync/env-sync-go "$BIN_DIR/env-sync-go"

# Install legacy Bash implementation (kept for compatibility/tests)
echo "Installing legacy Bash implementation to $LEGACY_DIR..."
rm -rf "$LEGACY_DIR/bin" "$LEGACY_DIR/lib"
cp -r "$SCRIPT_DIR/legacy/bin" "$LEGACY_DIR/bin"
cp -r "$SCRIPT_DIR/legacy/lib" "$LEGACY_DIR/lib"
chmod +x "$LEGACY_DIR/bin/"*

# macOS: adjust legacy scripts if GNU sed is available
if [[ "$OS" == "Darwin" ]]; then
    if command -v gsed >/dev/null 2>&1; then
        for script in "$LEGACY_DIR"/bin/env-sync*; do
            [ -f "$script" ] || continue
            gsed -i 's/sed -i /gsed -i /g' "$script" 2>/dev/null || true
        done
    fi
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""

# Check if bin directory is in PATH
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: $BIN_DIR is not in your PATH${NC}"
    echo "Add the following to your ~/.bashrc or ~/.zshrc:"
    echo ""
    echo "  export PATH=\"$BIN_DIR:\$PATH\""
    echo ""
fi

# Post-install instructions
echo "Next steps:"
echo ""
echo "1. Initialize your secrets file (Go binary by default):"
echo "   env-sync init --encrypted"
echo ""
echo "2. Edit ~/.secrets.env to add your secrets"
echo ""
echo "3. Start the server:"
echo "   env-sync serve -d"
echo ""
echo "4. Set up periodic sync (optional):"
echo "   env-sync cron --install"
echo ""
echo "5. Need the legacy Bash version? Run with ENV_SYNC_USE_BASH=true:"
echo "   ENV_SYNC_USE_BASH=true env-sync status"
echo ""

# Verify installation
echo "Verifying installation..."
if command -v env-sync >/dev/null 2>&1; then
    echo -e "${GREEN}✓ env-sync installed successfully${NC}"
    env-sync --help | head -20
else
    echo -e "${RED}✗ Installation verification failed${NC}"
    echo "Please ensure $BIN_DIR is in your PATH"
    exit 1
fi
