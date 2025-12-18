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
install_sambo() {
    TARBALL="sambo-${VERSION}-${PLATFORM}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/v${VERSION}/${TARBALL}"

    info "Downloading $TARBALL..."

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    if ! curl -fsSL "$URL" -o "$TMP_DIR/$TARBALL"; then
        error "Failed to download from $URL"
    fi

    info "Extracting..."
    tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR"

    # Find extracted directory
    EXTRACTED_DIR=$(find "$TMP_DIR" -maxdepth 1 -type d -name "sambo-*" | head -1)
    if [ -z "$EXTRACTED_DIR" ]; then
        error "Could not find extracted directory"
    fi

    # Install binary
    info "Installing sambo to $INSTALL_DIR (requires sudo)..."
    sudo install -m 755 "$EXTRACTED_DIR/sambo" "$INSTALL_DIR/sambo"

    # Install scripts
    info "Installing scripts to $SCRIPTS_DIR..."
    sudo mkdir -p "$SCRIPTS_DIR"
    if [ -d "$EXTRACTED_DIR/scripts" ]; then
        sudo cp -r "$EXTRACTED_DIR/scripts/"* "$SCRIPTS_DIR/"
        sudo chmod +x "$SCRIPTS_DIR/"*.sh 2>/dev/null || true
    fi

    success "Sambo $VERSION installed successfully!"
}

# Run macOS client setup if on macOS
run_client_setup() {
    if [ "$OS" = "darwin" ] && [ -f "$SCRIPTS_DIR/macos-client-setup.sh" ]; then
        echo ""
        read -p "Run macOS client setup to verify requirements? [Y/n] " -n 1 -r
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
