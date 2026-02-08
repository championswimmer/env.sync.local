#!/bin/bash
# Installation script for env-sync

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
LEGACY_BIN_DIR="$LEGACY_DIR/bin"
LEGACY_LIB_DIR="$LEGACY_DIR/lib"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --user)
            USER_INSTALL=true
            INSTALL_PREFIX="$HOME/.local"
            BIN_DIR="$INSTALL_PREFIX/bin"
            LIB_DIR="$INSTALL_PREFIX/lib/env-sync"
            LEGACY_DIR="$LIB_DIR/legacy"
            LEGACY_BIN_DIR="$LEGACY_DIR/bin"
            LEGACY_LIB_DIR="$LEGACY_DIR/lib"
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

echo -e "${BLUE}Installing env-sync...${NC}"

# Detect OS
OS=$(uname -s)

# Check dependencies
echo "Checking dependencies..."

MISSING_DEPS=()
GO_AVAILABLE=false

if ! command -v curl >/dev/null 2>&1; then
    MISSING_DEPS+=("curl")
fi

if ! command -v nc >/dev/null 2>&1 && ! command -v netcat >/dev/null 2>&1; then
    MISSING_DEPS+=("netcat (nc)")
fi

# Check for age (required for encryption support)
if ! command -v age >/dev/null 2>&1; then
    MISSING_DEPS+=("age")
fi

if ! command -v age-keygen >/dev/null 2>&1; then
    MISSING_DEPS+=("age-keygen")
fi

if command -v go >/dev/null 2>&1; then
    GO_AVAILABLE=true
else
    MISSING_DEPS+=("go (1.24+)")
fi

case "$OS" in
    Linux)
        if ! command -v avahi-browse >/dev/null 2>&1; then
            MISSING_DEPS+=("avahi-utils")
        fi
        ;;
    Darwin)
        # macOS has built-in dns-sd
        ;;
esac

if [[ ${#MISSING_DEPS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}Warning: Missing dependencies:${NC}"
    printf '  - %s\n' "${MISSING_DEPS[@]}"
    echo ""
    echo "Please install them:"
    case "$OS" in
        Linux)
            echo "  Ubuntu/Debian: sudo apt-get install avahi-daemon avahi-utils curl netcat-openbsd age"
            echo "  Fedora/RHEL:   sudo dnf install avahi avahi-tools curl nmap-ncat age"
            echo ""
            echo "  To install age manually:"
            echo "    curl -fsSL https://github.com/FiloSottile/age/releases/latest/download/age-v1.2.0-linux-amd64.tar.gz | tar -xz -C /usr/local/bin --strip-components=1"
            ;;
        Darwin)
            echo "  macOS: brew install age"
            echo ""
            echo "  To install age manually:"
            echo "    curl -fsSL https://github.com/FiloSottile/age/releases/latest/download/age-v1.2.0-darwin-amd64.tar.gz | tar -xz -C /usr/local/bin --strip-components=1"
            ;;
    esac
    echo ""
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Build Go binary unless it already exists and Go is unavailable
if $GO_AVAILABLE; then
    echo "Building Go binary..."
    mkdir -p "$SCRIPT_DIR/target"
    (cd "$SCRIPT_DIR/src" && go build -o "$SCRIPT_DIR/target/env-sync" ./cmd/env-sync)
elif [[ ! -x "$SCRIPT_DIR/target/env-sync" ]]; then
    echo -e "${RED}Go is required to build env-sync v2.0.${NC}"
    echo "Install Go 1.24+ or run 'make build' before installing."
    exit 1
fi

# Create directories
echo "Creating directories..."
mkdir -p "$BIN_DIR"
mkdir -p "$LIB_DIR"
mkdir -p "$LEGACY_BIN_DIR"
mkdir -p "$LEGACY_LIB_DIR"

# Install files
echo "Installing files..."

# Install Go binary (default)
install -m 755 "$SCRIPT_DIR/target/env-sync" "$BIN_DIR/env-sync"
for cmd in env-sync-discover env-sync-client env-sync-serve env-sync-key env-sync-load; do
    ln -sf env-sync "$BIN_DIR/$cmd"
done

# Install legacy Bash scripts (optional)
cp "$SCRIPT_DIR/legacy/bin/"* "$LEGACY_BIN_DIR/"
cp "$SCRIPT_DIR/legacy/lib/common.sh" "$LEGACY_LIB_DIR/"
chmod +x "$LEGACY_BIN_DIR"/env-sync*

# Create symlinks for older macOS compatibility
if [[ "$OS" == "Darwin" ]]; then
    # macOS uses BSD sed which has different syntax
    # Update scripts to use gsed if available
    if command -v gsed >/dev/null 2>&1; then
        for script in "$LEGACY_BIN_DIR"/env-sync*; do
            sed -i.bak 's/sed -i /gsed -i /g' "$script" 2>/dev/null || true
            rm -f "$script.bak"
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
echo "1. Initialize your secrets file:"
echo "   env-sync init"
echo ""
echo "2. Edit ~/.secrets.env to add your secrets"
echo ""
echo "3. Start the server:"
echo "   env-sync serve -d"
echo ""
echo "4. Set up periodic sync (optional):"
echo "   env-sync cron --install"
echo ""
echo "5. On other machines, repeat steps 1-4"
echo ""
echo "The machines will automatically discover each other!"
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
