#!/bin/bash
# Installation script for env-sync v2.0

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
ENV_SYNC_LOAD_LINE='eval "$(env-sync load --quiet 2>/dev/null)"'

# GitHub repository
GITHUB_REPO="championswimmer/env.sync.local"
GITHUB_RELEASES_URL="https://github.com/${GITHUB_REPO}/releases/latest/download"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --user)
            USER_INSTALL=true
            INSTALL_PREFIX="$HOME/.local"
            BIN_DIR="$INSTALL_PREFIX/bin"
            shift
            ;;
        --help)
            echo "Usage: install.sh [options]"
            echo "Options:"
            echo "  --user    Install to ~/.local (no sudo required)"
            echo "  --help    Show this help"
            echo ""
            echo "Installation modes:"
            echo "  - Local mode: Run from cloned repository (builds from source)"
            echo "  - Remote mode: Run via curl (downloads pre-built binary)"
            echo ""
            echo "Examples:"
            echo "  # Quick install (web-based):"
            echo "  curl -fsSL https://envsync.arnav.tech/install.sh | sudo bash"
            echo ""
            echo "  # Install to user directory:"
            echo "  curl -fsSL https://envsync.arnav.tech/install.sh | bash -s -- --user"
            echo ""
            echo "  # Local install from source:"
            echo "  git clone https://github.com/championswimmer/env.sync.local.git"
            echo "  cd env.sync.local"
            echo "  sudo ./install.sh"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Detect if running from local repo or via curl/wget (remote mode)
REMOTE_MODE=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

# Check if we're in a git repository with the expected structure
if [[ ! -d "$SCRIPT_DIR/src" ]] || [[ ! -f "$SCRIPT_DIR/Makefile" ]]; then
    REMOTE_MODE=true
    echo -e "${BLUE}Remote installation mode detected${NC}"
else
    echo -e "${BLUE}Local installation mode detected${NC}"
fi

# Detect OS and architecture for remote mode
detect_platform() {
    local os=""
    local arch=""

    # Detect OS
    case "$(uname -s)" in
        Linux*)
            os="linux"
            ;;
        Darwin*)
            os="macos"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            os="windows"
            ;;
        *)
            echo -e "${RED}Unsupported operating system: $(uname -s)${NC}"
            exit 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            echo -e "${RED}Unsupported architecture: $(uname -m)${NC}"
            exit 1
            ;;
    esac

    echo "${os}-${arch}"
}

# Download binary from GitHub releases
download_binary() {
    local platform="$1"
    local binary_name="env-sync-${platform}"
    local download_url="${GITHUB_RELEASES_URL}/${binary_name}"

    # Add .exe extension for Windows
    if [[ "$platform" == windows-* ]]; then
        binary_name="${binary_name}.exe"
        download_url="${download_url}.exe"
    fi

    echo "Downloading env-sync from ${download_url}..."

    # Create temporary directory for download
    local temp_dir
    temp_dir="$(mktemp -d)"
    local temp_binary="${temp_dir}/env-sync"

    # Try curl first, then wget
    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL -o "$temp_binary" "$download_url"; then
            echo -e "${RED}Failed to download binary from ${download_url}${NC}"
            rm -rf "$temp_dir"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -q -O "$temp_binary" "$download_url"; then
            echo -e "${RED}Failed to download binary from ${download_url}${NC}"
            rm -rf "$temp_dir"
            exit 1
        fi
    else
        echo -e "${RED}Neither curl nor wget found. Please install one of them.${NC}"
        rm -rf "$temp_dir"
        exit 1
    fi

    echo "$temp_binary"
}

echo -e "${BLUE}Installing env-sync v2.0 (Go binary)...${NC}"

# Detect OS
OS=$(uname -s)

# Check dependencies
echo "Checking dependencies..."

MISSING_DEPS=()

# Go version dependencies (minimal - most is built-in)
# Only require Go if building from source (local mode)
if [[ "$REMOTE_MODE" == "false" ]] && ! command -v go >/dev/null 2>&1; then
    MISSING_DEPS+=("go (v1.24 or later)")
fi

case "$OS" in
    Linux)
        if ! command -v avahi-browse >/dev/null 2>&1; then
            MISSING_DEPS+=("avahi-utils (for mDNS discovery)")
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
            echo "  Ubuntu/Debian: sudo apt-get install golang-go avahi-daemon avahi-utils"
            echo "  Fedora/RHEL:   sudo dnf install golang avahi avahi-tools"
            ;;
        Darwin)
            echo "  macOS: brew install go"
            ;;
    esac
    echo ""
    echo "Note: AGE encryption is built into the Go binary, no separate age package needed."
    echo ""
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

add_shell_integration() {
    local target_file=""

    if [[ -f "$HOME/.profile" ]]; then
        target_file="$HOME/.profile"
    elif [[ -f "$HOME/.bash_profile" ]]; then
        target_file="$HOME/.bash_profile"
    else
        target_file="$HOME/.bashrc"
        if [[ ! -f "$target_file" ]]; then
            touch "$target_file"
        fi
    fi

    if grep -F "env-sync load" "$target_file" >/dev/null 2>&1; then
        echo "Shell integration already present in ${target_file/$HOME/~}"
        return
    fi

    printf '\n%s\n' "$ENV_SYNC_LOAD_LINE" >> "$target_file"
    echo "Added shell integration to ${target_file/$HOME/~}"
}

# Create directories
echo "Creating directories..."
mkdir -p "$BIN_DIR"

# Install files
echo "Installing files..."

# Stop running service if it exists
SERVICE_WAS_STOPPED=false
# Check if env-sync is already installed and try to stop the service
if command -v env-sync >/dev/null 2>&1; then
    echo "Checking for running service..."
    # Try to stop the service gracefully
    if env-sync service stop 2>&1 | grep -q "Service stopped"; then
        SERVICE_WAS_STOPPED=true
    fi
fi

# Build and install Go binary
if [[ "$REMOTE_MODE" == "true" ]]; then
    # Remote mode - download pre-built binary
    PLATFORM=$(detect_platform)
    echo "Detected platform: $PLATFORM"
    TEMP_BINARY=$(download_binary "$PLATFORM")

    # Install downloaded binary
    cp "$TEMP_BINARY" "$BIN_DIR/env-sync"
    chmod +x "$BIN_DIR/env-sync"

    # Clean up temporary files
    rm -rf "$(dirname "$TEMP_BINARY")"

    echo -e "${GREEN}✓ Binary downloaded and installed${NC}"
else
    # Local mode - build from source
    echo "Building Go binary..."
    cd "$SCRIPT_DIR"
    make build

    if [[ ! -f "$SCRIPT_DIR/target/env-sync" ]]; then
        echo -e "${RED}✗ Build failed - binary not found${NC}"
        exit 1
    fi

    # Install Go binary
    cp "$SCRIPT_DIR/target/env-sync" "$BIN_DIR/env-sync"
    chmod +x "$BIN_DIR/env-sync"
fi

# Restart service if it was stopped
if [[ "$SERVICE_WAS_STOPPED" == "true" ]]; then
    echo "Restarting service..."
    "$BIN_DIR/env-sync" service restart >/dev/null 2>&1 || {
        echo -e "${YELLOW}Note: Service was stopped but could not be restarted automatically${NC}"
        echo "Run 'env-sync serve -d' to start the service manually"
    }
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

add_shell_integration

# Post-install instructions
echo "Next steps:"
echo ""
echo "env-sync v2.0 (Go binary) has been installed!"
echo ""
echo "1. Initialize your secrets file:"
echo "   env-sync init --encrypted"
echo ""
echo "2. Add your secrets:"
echo "   env-sync add OPENAI_API_KEY=\"sk-...\""
echo ""
echo "3. Set up periodic sync (optional):"
echo "   env-sync cron --install"
echo ""
echo "4. On other machines, repeat steps 1-3"
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
