#!/bin/bash
set -e

# Configuration
APP_NAME="sambo"
IDENTIFIER="com.winterweken.sambo"
VERSION="${1:-1.5.0}" # Default to 1.5.0 if not provided
BUILD_DIR="build"
PKG_DIR="${BUILD_DIR}/pkg"
OUTPUT_DIR="${BUILD_DIR}/release"

# Colors
GREEN='\033[0;32m'
NC='\033[0m'

echo -e "${GREEN}Building macOS packages for version ${VERSION}...${NC}"

# function to build package for a specific architecture
build_pkg() {
    local ARCH=$1
    local GOARCH=$2
    
    echo "Creating package for ${ARCH}..."
    
    # Create directory structure
    local PAYLOAD_DIR="${PKG_DIR}/${ARCH}/root"
    local BIN_DIR="${PAYLOAD_DIR}/usr/local/bin"
    local SHARE_DIR="${PAYLOAD_DIR}/usr/local/share/${APP_NAME}"
    
    mkdir -p "${BIN_DIR}"
    mkdir -p "${SHARE_DIR}"
    
    # Check if binary exists
    local BINARY="${BUILD_DIR}/${APP_NAME}-darwin-${GOARCH}"
    if [ ! -f "${BINARY}" ]; then
        echo "Error: Binary ${BINARY} not found. Run 'make build-all' first."
        exit 1
    fi
    
    # Copy binary
    cp "${BINARY}" "${BIN_DIR}/${APP_NAME}"
    chmod 755 "${BIN_DIR}/${APP_NAME}"
    
    # Copy scripts
    cp scripts/macos-client-setup.sh "${SHARE_DIR}/"
    cp scripts/install.sh "${SHARE_DIR}/"
    chmod 755 "${SHARE_DIR}/"*.sh
    
    # Build package
    local PKG_NAME="${APP_NAME}-${VERSION}-darwin-${GOARCH}.pkg"
    mkdir -p "${OUTPUT_DIR}"
    
    pkgbuild --root "${PAYLOAD_DIR}" \
             --identifier "${IDENTIFIER}" \
             --version "${VERSION}" \
             --install-location "/" \
             "${OUTPUT_DIR}/${PKG_NAME}"
             
    echo -e "${GREEN}Created ${OUTPUT_DIR}/${PKG_NAME}${NC}"
}

# Clean previous build
rm -rf "${PKG_DIR}"

# Build for AMD64 (Intel)
build_pkg "amd64" "amd64"

# Build for ARM64 (Apple Silicon)
build_pkg "arm64" "arm64"

echo -e "${GREEN}Package creation complete! Check ${OUTPUT_DIR}${NC}"
