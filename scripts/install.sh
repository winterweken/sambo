#!/bin/bash
#
# Sambo Installer
# One-liner: curl -fsSL https://raw.githubusercontent.com/winterweken/sambo/main/scripts/install.sh | bash
#

set -e

REPO="winterweken/sambo"
INSTALL_DIR="/usr/local/bin"
SCRIPTS_DIR="/usr/local/share/sambo"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${BLUE}==>${NC} $1"; }
success() { echo -e "${GREEN}==>${NC} $1"; }
warn() { echo -e "${YELLOW}==>${NC} $1"; }
error() { echo -e "${RED}==>${NC} $1"; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) OS="darwin" ;;
        linux) OS="linux" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        armv7l|armv6l) ARCH="arm" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get latest release version
get_latest_version() {
    info "Fetching latest version..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//')

    if [ -z "$VERSION" ]; then
        # Fallback: try to get from tags
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/tags" | grep '"name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//')
    fi

    if [ -z "$VERSION" ]; then
        error "Could not determine latest version"
    fi

    info "Latest version: $VERSION"
}

# Download and install
#
# Releases carry a bare binary per Linux architecture (with checksums.txt) and
# an installer package per macOS architecture.
install_sambo() {
    BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    if [ "$OS" = "darwin" ]; then
        install_macos_package
    else
        install_linux_binary
    fi

    success "Sambo $VERSION installed successfully!"
}

download() {
    info "Downloading $1..."
    if ! curl -fsSL "$BASE_URL/$1" -o "$TMP_DIR/$1"; then
        error "Failed to download from $BASE_URL/$1"
    fi
}

install_macos_package() {
    [ "$ARCH" = "arm" ] && error "No macOS package for architecture: $ARCH"
    PKG="sambo-${VERSION}-${PLATFORM}.pkg"
    download "$PKG"

    info "Installing $PKG (requires sudo)..."
    sudo installer -pkg "$TMP_DIR/$PKG" -target /
}

install_linux_binary() {
    BINARY="sambo-${PLATFORM}"
    download "$BINARY"
    download "checksums.txt"

    if command -v sha256sum >/dev/null 2>&1; then
        info "Verifying checksum..."
        if ! (cd "$TMP_DIR" && grep " ${BINARY}\$" checksums.txt | sha256sum -c - >/dev/null); then
            error "Checksum mismatch for $BINARY"
        fi
    else
        warn "sha256sum not found; skipping checksum verification"
    fi

    info "Installing sambo to $INSTALL_DIR (requires sudo)..."
    sudo install -m 755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/sambo"
}

# Run macOS client setup if on macOS
run_client_setup() {
    # Piped from curl, stdin is the script itself, so the answer comes from the terminal.
    if [ "$OS" = "darwin" ] && [ -f "$SCRIPTS_DIR/macos-client-setup.sh" ] && [ -t 1 ] && [ -r /dev/tty ]; then
        echo ""
        read -p "Run macOS client setup to verify requirements? [Y/n] " -n 1 -r </dev/tty
        echo ""
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            "$SCRIPTS_DIR/macos-client-setup.sh"
        fi
    fi
}

# Print usage info
print_usage() {
    echo ""
    success "Installation complete!"
    echo ""
    echo "Usage:"
    echo "  sudo sambo              # Launch interactive TUI"
    echo "  sudo sambo samba list   # List Samba shares"
    echo "  sudo sambo nfs list     # List NFS exports"
    echo "  sudo sambo mount list   # List network mounts"
    echo "  sambo help              # Show all commands"
    echo ""
    if [ "$OS" = "darwin" ]; then
        echo "Client setup script:"
        echo "  $SCRIPTS_DIR/macos-client-setup.sh"
        echo ""
    fi
}

main() {
    echo ""
    echo -e "${BLUE}Sambo Installer${NC}"
    echo ""

    detect_platform
    get_latest_version
    install_sambo
    print_usage
    run_client_setup
}

main "$@"
