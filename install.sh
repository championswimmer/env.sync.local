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
ENV_SYNC_VERSION="3.0.1"
USER_INSTALL=false
INSTALL_PREFIX="/usr/local"
BIN_DIR="$INSTALL_PREFIX/bin"
ENV_SYNC_LOAD_LINE='eval "$(env-sync load --quiet 2>/dev/null)"'
INSTALL_GUI=false
INSTALL_CLI=true
GUI_BUNDLE_NAME="env-sync.app"
GUI_BINARY_NAME="env-sync-gui"

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
        --gui)
            INSTALL_GUI=true
            shift
            ;;
        --all)
            INSTALL_GUI=true
            INSTALL_CLI=true
            shift
            ;;
        --gui-only)
            INSTALL_GUI=true
            INSTALL_CLI=false
            shift
            ;;
        --help)
            echo "Usage: install.sh [options]"
            echo "Options:"
            echo "  --user      Install to ~/.local (no sudo required)"
            echo "  --gui       Also install the GUI application"
            echo "  --gui-only  Install only the GUI application (not CLI)"
            echo "  --all       Install both CLI and GUI"
            echo "  --help      Show this help"
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

# Download artifact from GitHub releases
download_artifact() {
    local artifact_name="$1"
    local download_url="${GITHUB_RELEASES_URL}/${artifact_name}"

    echo "Downloading ${artifact_name} from ${download_url}..." >&2

    local temp_dir
    temp_dir="$(mktemp -d)"
    local temp_path="${temp_dir}/${artifact_name}"

    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL -o "$temp_path" "$download_url"; then
            echo -e "${RED}Failed to download artifact from ${download_url}${NC}" >&2
            rm -rf "$temp_dir"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -q -O "$temp_path" "$download_url"; then
            echo -e "${RED}Failed to download artifact from ${download_url}${NC}" >&2
            rm -rf "$temp_dir"
            exit 1
        fi
    else
        echo -e "${RED}Neither curl nor wget found. Please install one of them.${NC}" >&2
        rm -rf "$temp_dir"
        exit 1
    fi

    echo "$temp_path"
}

download_binary() {
    local platform="$1"
    local artifact_name="env-sync-${platform}"

    if [[ "$platform" == windows-* ]]; then
        artifact_name="${artifact_name}.exe"
    fi

    download_artifact "$artifact_name"
}

extract_deb_payload() {
    local deb_path="$1"
    local destination="$2"

    mkdir -p "$destination"

    if command -v dpkg-deb >/dev/null 2>&1; then
        dpkg-deb -x "$deb_path" "$destination"
        return
    fi

    if ! command -v ar >/dev/null 2>&1; then
        echo -e "${RED}Neither dpkg-deb nor ar is available to unpack $(basename "$deb_path").${NC}" >&2
        exit 1
    fi

    local data_member=""
    data_member="$(ar t "$deb_path" | awk '/^data\.tar(\.|$)/ { print; exit }')"

    if [[ -z "$data_member" ]]; then
        echo -e "${RED}Unable to locate Debian payload inside $(basename "$deb_path").${NC}" >&2
        exit 1
    fi

    case "$data_member" in
        *.tar.gz)
            ar p "$deb_path" "$data_member" | tar -xzf - -C "$destination"
            ;;
        *.tar.xz)
            ar p "$deb_path" "$data_member" | tar -xJf - -C "$destination"
            ;;
        *.tar.zst)
            if command -v unzstd >/dev/null 2>&1; then
                ar p "$deb_path" "$data_member" | unzstd -c | tar -xf - -C "$destination"
            elif command -v zstd >/dev/null 2>&1; then
                ar p "$deb_path" "$data_member" | zstd -d -c | tar -xf - -C "$destination"
            else
                echo -e "${RED}zstd support is required to unpack $(basename "$deb_path").${NC}" >&2
                exit 1
            fi
            ;;
        *.tar.bz2)
            ar p "$deb_path" "$data_member" | tar -xjf - -C "$destination"
            ;;
        *.tar)
            ar p "$deb_path" "$data_member" | tar -xf - -C "$destination"
            ;;
        *)
            echo -e "${RED}Unsupported Debian payload format: ${data_member}${NC}" >&2
            exit 1
            ;;
    esac
}

echo -e "${BLUE}Installing env-sync v${ENV_SYNC_VERSION}...${NC}"

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

configure_gui_paths() {
    case "$OS" in
        Darwin)
            if [[ "$USER_INSTALL" == "true" ]]; then
                GUI_INSTALL_DIR="$HOME/Applications"
            else
                GUI_INSTALL_DIR="/Applications"
            fi
            GUI_INSTALL_TARGET="$GUI_INSTALL_DIR/$GUI_BUNDLE_NAME"
            ;;
        Linux)
            if [[ "$USER_INSTALL" == "true" ]]; then
                GUI_INSTALL_DIR="$HOME/.local/lib/env-sync"
                GUI_DESKTOP_DIR="$HOME/.local/share/applications"
                GUI_ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
                GUI_LAUNCHER="$HOME/.local/bin/$GUI_BINARY_NAME"
            else
                GUI_INSTALL_DIR="/opt/env-sync"
                GUI_DESKTOP_DIR="/usr/local/share/applications"
                GUI_ICON_DIR="/usr/local/share/icons/hicolor/512x512/apps"
                GUI_LAUNCHER="$BIN_DIR/$GUI_BINARY_NAME"
            fi
            GUI_INSTALL_TARGET="$GUI_INSTALL_DIR/$GUI_BINARY_NAME"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            GUI_INSTALL_DIR=""
            GUI_INSTALL_TARGET=""
            ;;
    esac
}

write_linux_gui_desktop_file() {
    local desktop_file="$1"
    local exec_path="$2"
    local icon_name="$3"

    cat > "$desktop_file" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=env-sync
Comment=Distributed secrets synchronization for local networks
Exec=${exec_path}
Icon=${icon_name}
Terminal=false
Categories=Utility;Development;
Keywords=env;sync;secrets;
StartupWMClass=env-sync-gui
EOF
}

refresh_linux_desktop_cache() {
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database "$GUI_DESKTOP_DIR" >/dev/null 2>&1 || true
    fi
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache -q "${GUI_ICON_DIR%/512x512/apps}" >/dev/null 2>&1 || true
    fi
}

install_linux_gui_payload() {
    local binary_source="$1"
    local icon_source="$2"

    mkdir -p "$GUI_INSTALL_DIR" "$GUI_DESKTOP_DIR" "$GUI_ICON_DIR" "$(dirname "$GUI_LAUNCHER")"
    install -m 755 "$binary_source" "$GUI_INSTALL_TARGET"
    install -m 644 "$icon_source" "$GUI_ICON_DIR/env-sync-gui.png"
    ln -sf "$GUI_INSTALL_TARGET" "$GUI_LAUNCHER"
    write_linux_gui_desktop_file "$GUI_DESKTOP_DIR/env-sync-gui.desktop" "$GUI_LAUNCHER" "env-sync-gui"
    refresh_linux_desktop_cache
    echo -e "${GREEN}✓ GUI application installed to ${GUI_INSTALL_DIR}${NC}"
}

create_macos_gui_bundle() {
    local binary_source="$1"
    local bundle_target="$2"
    local contents_dir="$bundle_target/Contents"
    local resources_dir="$contents_dir/Resources"
    local icon_source="$SCRIPT_DIR/src/gui/build/bin/env-sync.app/Contents/Resources/iconfile.icns"

    rm -rf "$bundle_target"
    mkdir -p "$contents_dir/MacOS" "$resources_dir"
    install -m 755 "$binary_source" "$contents_dir/MacOS/$GUI_BINARY_NAME"

    if [[ -f "$icon_source" ]]; then
        install -m 644 "$icon_source" "$resources_dir/iconfile.icns"
    fi

    cat > "$contents_dir/Info.plist" <<EOF
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleName</key>
    <string>env-sync</string>
    <key>CFBundleExecutable</key>
    <string>${GUI_BINARY_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>com.wails.env-sync</string>
    <key>CFBundleVersion</key>
    <string>${ENV_SYNC_VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${ENV_SYNC_VERSION}</string>
    <key>CFBundleGetInfoString</key>
    <string>Distributed secrets synchronization for local networks</string>
    <key>CFBundleIconFile</key>
    <string>iconfile</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13.0</string>
    <key>NSHighResolutionCapable</key>
    <string>true</string>
    <key>NSAppTransportSecurity</key>
    <dict>
      <key>NSAllowsLocalNetworking</key>
      <true/>
    </dict>
  </dict>
</plist>
EOF
}

install_macos_gui_bundle() {
    local bundle_source="$1"

    mkdir -p "$GUI_INSTALL_DIR"
    rm -rf "$GUI_INSTALL_TARGET"
    ditto "$bundle_source" "$GUI_INSTALL_TARGET"
    xattr -rc "$GUI_INSTALL_TARGET" >/dev/null 2>&1 || true
    echo -e "${GREEN}✓ GUI application installed to ${GUI_INSTALL_TARGET}${NC}"
}

configure_gui_paths

if [[ "$INSTALL_GUI" == "true" && "$OS" =~ ^(MINGW|MSYS|CYGWIN) ]]; then
    echo -e "${RED}GUI installation via install.sh is not supported on Windows.${NC}"
    echo "Download and run the Windows installer from GitHub Releases instead."
    exit 1
fi

CLI_INSTALLED=false
GUI_INSTALLED=false

echo "Creating directories..."
if [[ "$INSTALL_CLI" == "true" ]] || [[ "$INSTALL_GUI" == "true" && "$OS" == "Linux" && "$USER_INSTALL" == "false" ]]; then
    mkdir -p "$BIN_DIR"
fi

echo "Installing files..."

SERVICE_WAS_STOPPED=false
if [[ "$INSTALL_CLI" == "true" ]] && command -v env-sync >/dev/null 2>&1; then
    echo "Checking for running service..."
    if env-sync service stop 2>&1 | grep -q "Service stopped"; then
        SERVICE_WAS_STOPPED=true
    fi
fi

if [[ "$REMOTE_MODE" == "true" ]]; then
    PLATFORM=$(detect_platform)
    PLATFORM_OS="${PLATFORM%-*}"
    PLATFORM_ARCH="${PLATFORM##*-}"
    echo "Detected platform: $PLATFORM"

    if [[ "$INSTALL_CLI" == "true" ]]; then
        TEMP_BINARY=$(download_binary "$PLATFORM")
        cp "$TEMP_BINARY" "$BIN_DIR/env-sync"
        chmod +x "$BIN_DIR/env-sync"
        rm -rf "$(dirname "$TEMP_BINARY")"
        CLI_INSTALLED=true
        echo -e "${GREEN}✓ CLI binary downloaded and installed${NC}"
    fi

    if [[ "$INSTALL_GUI" == "true" ]]; then
        case "$PLATFORM_OS" in
            macos)
                TEMP_GUI_ARCHIVE=$(download_artifact "env-sync-gui-macos-${PLATFORM_ARCH}.dmg")
                TEMP_GUI_DIR="$(mktemp -d)"
                MOUNT_POINT="$TEMP_GUI_DIR/mount"
                mkdir -p "$MOUNT_POINT"
                hdiutil attach "$TEMP_GUI_ARCHIVE" -mountpoint "$MOUNT_POINT" -nobrowse -readonly >/dev/null
                TEMP_GUI_BUNDLE="$MOUNT_POINT/$GUI_BUNDLE_NAME"

                if [[ ! -d "$TEMP_GUI_BUNDLE" ]]; then
                    echo -e "${RED}✗ Downloaded macOS GUI bundle is missing ${GUI_BUNDLE_NAME}${NC}"
                    hdiutil detach "$MOUNT_POINT" -quiet >/dev/null 2>&1 || true
                    rm -rf "$(dirname "$TEMP_GUI_ARCHIVE")" "$TEMP_GUI_DIR"
                    exit 1
                fi

                if ! install_macos_gui_bundle "$TEMP_GUI_BUNDLE"; then
                    hdiutil detach "$MOUNT_POINT" -quiet >/dev/null 2>&1 || true
                    rm -rf "$(dirname "$TEMP_GUI_ARCHIVE")" "$TEMP_GUI_DIR"
                    exit 1
                fi
                hdiutil detach "$MOUNT_POINT" -quiet >/dev/null 2>&1 || true
                rm -rf "$(dirname "$TEMP_GUI_ARCHIVE")" "$TEMP_GUI_DIR"
                GUI_INSTALLED=true
                ;;
            linux)
                TEMP_GUI_ARCHIVE=$(download_artifact "env-sync-gui-linux-${PLATFORM_ARCH}.deb")
                TEMP_GUI_DIR="$(mktemp -d)"
                extract_deb_payload "$TEMP_GUI_ARCHIVE" "$TEMP_GUI_DIR"

                if [[ ! -f "$TEMP_GUI_DIR/opt/env-sync/$GUI_BINARY_NAME" ]] || [[ ! -f "$TEMP_GUI_DIR/usr/share/icons/hicolor/512x512/apps/env-sync-gui.png" ]]; then
                    echo -e "${RED}✗ Downloaded Linux GUI package is missing required files${NC}"
                    rm -rf "$(dirname "$TEMP_GUI_ARCHIVE")" "$TEMP_GUI_DIR"
                    exit 1
                fi

                install_linux_gui_payload \
                    "$TEMP_GUI_DIR/opt/env-sync/$GUI_BINARY_NAME" \
                    "$TEMP_GUI_DIR/usr/share/icons/hicolor/512x512/apps/env-sync-gui.png"
                rm -rf "$(dirname "$TEMP_GUI_ARCHIVE")" "$TEMP_GUI_DIR"
                GUI_INSTALLED=true
                ;;
            *)
                echo -e "${RED}✗ GUI installation is not available for ${PLATFORM_OS} via install.sh${NC}"
                exit 1
                ;;
        esac
    fi
else
    echo "Building from source..."
    cd "$SCRIPT_DIR"

    if [[ "$INSTALL_CLI" == "true" ]]; then
        echo "Building CLI binary..."
        make build

        if [[ ! -f "$SCRIPT_DIR/target/env-sync" ]]; then
            echo -e "${RED}✗ CLI build failed - binary not found${NC}"
            exit 1
        fi

        cp "$SCRIPT_DIR/target/env-sync" "$BIN_DIR/env-sync"
        chmod +x "$BIN_DIR/env-sync"
        CLI_INSTALLED=true
        echo -e "${GREEN}✓ CLI binary installed${NC}"
    fi

    if [[ "$INSTALL_GUI" == "true" ]]; then
        echo "Building GUI binary..."

        if ! command -v npm >/dev/null 2>&1; then
            echo -e "${RED}✗ npm is required to build the GUI. Install Node.js first.${NC}"
            exit 1
        fi

        make build-gui

        if [[ ! -f "$SCRIPT_DIR/target/env-sync-gui" ]]; then
            echo -e "${RED}✗ GUI build failed - binary not found${NC}"
            exit 1
        fi

        case "$OS" in
            Darwin)
                TEMP_GUI_DIR="$(mktemp -d)"
                create_macos_gui_bundle "$SCRIPT_DIR/target/env-sync-gui" "$TEMP_GUI_DIR/$GUI_BUNDLE_NAME"
                install_macos_gui_bundle "$TEMP_GUI_DIR/$GUI_BUNDLE_NAME"
                rm -rf "$TEMP_GUI_DIR"
                ;;
            Linux)
                install_linux_gui_payload "$SCRIPT_DIR/target/env-sync-gui" "$SCRIPT_DIR/src/gui/build/appicon.png"
                ;;
            *)
                echo -e "${RED}✗ GUI installation is not available for ${OS} via install.sh${NC}"
                exit 1
                ;;
        esac

        GUI_INSTALLED=true
    fi
fi

# Restart service if it was stopped
if [[ "$SERVICE_WAS_STOPPED" == "true" && "$CLI_INSTALLED" == "true" ]]; then
    echo "Restarting service..."
    "$BIN_DIR/env-sync" service restart >/dev/null 2>&1 || {
        echo -e "${YELLOW}Note: Service was stopped but could not be restarted automatically${NC}"
        echo "Run 'env-sync serve -d' to start the service manually"
    }
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""

# Check if bin directory is in PATH
if [[ "$INSTALL_CLI" == "true" && ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: $BIN_DIR is not in your PATH${NC}"
    echo "Add the following to your ~/.bashrc or ~/.zshrc:"
    echo ""
    echo "  export PATH=\"$BIN_DIR:\$PATH\""
    echo ""
fi

if [[ "$CLI_INSTALLED" == "true" ]]; then
    add_shell_integration
fi

# Post-install instructions
echo "Next steps:"
echo ""
echo "env-sync v${ENV_SYNC_VERSION} has been installed!"
echo ""
if [[ "$CLI_INSTALLED" == "true" ]]; then
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
fi

if [[ "$GUI_INSTALLED" == "true" ]]; then
    case "$OS" in
        Darwin)
            echo "GUI application location:"
            echo "  $GUI_INSTALL_TARGET"
            echo "Launch it from Finder, Spotlight, or:"
            echo "  open \"$GUI_INSTALL_TARGET\""
            echo ""
            ;;
        Linux)
            echo "GUI application location:"
            echo "  $GUI_INSTALL_TARGET"
            echo "Desktop launcher:"
            echo "  $GUI_DESKTOP_DIR/env-sync-gui.desktop"
            echo ""
            ;;
    esac
fi

echo "The machines will automatically discover each other!"
echo ""

# Verify installation
echo "Verifying installation..."
if [[ "$CLI_INSTALLED" == "true" ]] && command -v env-sync >/dev/null 2>&1; then
    echo -e "${GREEN}✓ env-sync installed successfully${NC}"
    env-sync --help | head -20
elif [[ "$CLI_INSTALLED" == "true" ]]; then
    echo -e "${RED}✗ Installation verification failed${NC}"
    echo "Please ensure $BIN_DIR is in your PATH"
    exit 1
fi

if [[ "$GUI_INSTALLED" == "true" ]]; then
    case "$OS" in
        Darwin)
            if [[ -d "$GUI_INSTALL_TARGET" ]]; then
                echo -e "${GREEN}✓ env-sync GUI installed successfully${NC}"
            else
                echo -e "${RED}✗ GUI installation verification failed${NC}"
                exit 1
            fi
            ;;
        Linux)
            if [[ -x "$GUI_INSTALL_TARGET" ]] && [[ -f "$GUI_DESKTOP_DIR/env-sync-gui.desktop" ]]; then
                echo -e "${GREEN}✓ env-sync GUI installed successfully${NC}"
            else
                echo -e "${RED}✗ GUI installation verification failed${NC}"
                exit 1
            fi
            ;;
    esac
fi
