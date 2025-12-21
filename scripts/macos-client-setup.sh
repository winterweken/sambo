#!/bin/bash
#
# Sambo macOS Client Setup Script
# Ensures all requirements for network mounts (CIFS/SMB and NFS) are installed
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNED=0

print_header() {
    echo ""
    echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  Sambo macOS Client Setup${NC}"
    echo -e "${BLUE}  Network Mount Requirements Checker${NC}"
    echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
    echo ""
}

print_section() {
    echo ""
    echo -e "${BLUE}── $1 ──${NC}"
}

check_pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    CHECKS_PASSED=$((CHECKS_PASSED + 1))
}

check_fail() {
    echo -e "  ${RED}✗${NC} $1"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
}

check_warn() {
    echo -e "  ${YELLOW}!${NC} $1"
    CHECKS_WARNED=$((CHECKS_WARNED + 1))
}

check_info() {
    echo -e "  ${BLUE}ℹ${NC} $1"
}

# Check if running on macOS
check_macos() {
    print_section "Operating System"

    if [[ "$(uname)" != "Darwin" ]]; then
        check_fail "This script is designed for macOS only"
        echo -e "      For Linux, install: ${YELLOW}cifs-utils nfs-common${NC}"
        exit 1
    fi

    MACOS_VERSION=$(sw_vers -productVersion)
    MACOS_BUILD=$(sw_vers -buildVersion)
    check_pass "macOS $MACOS_VERSION ($MACOS_BUILD)"

    # Check architecture
    ARCH=$(uname -m)
    check_info "Architecture: $ARCH"
}

# Check for required SMB/CIFS tools
check_smb_requirements() {
    print_section "SMB/CIFS Requirements"

    # mount_smbfs is built into macOS
    if command -v mount_smbfs &> /dev/null; then
        check_pass "mount_smbfs found (built-in)"
    else
        check_fail "mount_smbfs not found"
        echo "      This is unusual - mount_smbfs should be built into macOS"
    fi

    # smbutil for diagnostics
    if command -v smbutil &> /dev/null; then
        check_pass "smbutil found (diagnostics/browsing)"
    else
        check_warn "smbutil not found"
    fi

    # Check SMB configuration
    if [[ -f /etc/nsmb.conf ]]; then
        check_info "Custom SMB config exists at /etc/nsmb.conf"
    else
        check_info "Using default SMB configuration"
    fi
}

# Check for required NFS tools
check_nfs_requirements() {
    print_section "NFS Requirements"

    # mount_nfs is built into macOS
    if command -v mount_nfs &> /dev/null; then
        check_pass "mount_nfs found (built-in)"
    else
        check_fail "mount_nfs not found"
        echo "      This is unusual - mount_nfs should be built into macOS"
    fi

    # showmount for NFS discovery
    if command -v showmount &> /dev/null; then
        check_pass "showmount found (NFS discovery)"
    else
        check_fail "showmount not found"
        echo -e "      Install with: ${YELLOW}brew install nfs-utils${NC}"
        echo -e "      Or use Xcode Command Line Tools: ${YELLOW}xcode-select --install${NC}"
    fi

    # Check if NFS client is enabled
    if launchctl list 2>/dev/null | grep -q "com.apple.nfsd"; then
        check_info "NFS daemon detected (server mode available)"
    fi

    # Check automount configuration
    if [[ -f /etc/auto_master ]]; then
        check_pass "Automount system configured"
        if grep -q "auto_nfs" /etc/auto_master 2>/dev/null; then
            check_info "auto_nfs already in auto_master"
        fi
    else
        check_warn "Automount configuration not found"
    fi
}

# Check general mount requirements
check_general_requirements() {
    print_section "General Requirements"

    # umount command
    if command -v umount &> /dev/null; then
        check_pass "umount found"
    else
        check_fail "umount not found"
    fi

    # mount command
    if command -v mount &> /dev/null; then
        check_pass "mount found"
    else
        check_fail "mount not found"
    fi

    # diskutil for disk management
    if command -v diskutil &> /dev/null; then
        check_pass "diskutil found"
    else
        check_warn "diskutil not found"
    fi
}

# Check network requirements
check_network_requirements() {
    print_section "Network Requirements"

    # Check for active network interface
    ACTIVE_IF=$(route get default 2>/dev/null | grep interface | awk '{print $2}')
    if [[ -n "$ACTIVE_IF" ]]; then
        check_pass "Active network interface: $ACTIVE_IF"

        # Get IP address
        IP_ADDR=$(ipconfig getifaddr "$ACTIVE_IF" 2>/dev/null)
        if [[ -n "$IP_ADDR" ]]; then
            check_info "IP Address: $IP_ADDR"
        fi
    else
        check_warn "No active network interface detected"
    fi

    # Check DNS resolution
    if nslookup google.com &> /dev/null; then
        check_pass "DNS resolution working"
    else
        check_warn "DNS resolution may have issues"
    fi

    # Check for firewall that might block NFS/SMB
    FIREWALL_STATE=$(defaults read /Library/Preferences/com.apple.alf globalstate 2>/dev/null || echo "unknown")
    case "$FIREWALL_STATE" in
        0)
            check_info "Firewall: Disabled"
            ;;
        1)
            check_info "Firewall: Enabled (allow signed apps)"
            ;;
        2)
            check_warn "Firewall: Enabled (block all incoming)"
            echo "      May need to allow NFS/SMB connections"
            ;;
        *)
            check_info "Firewall: Unknown state"
            ;;
    esac
}

# Check and create required directories
check_directories() {
    print_section "Directory Setup"

    # Check for common mount point locations
    MOUNT_DIRS=("/Volumes" "/mnt" "/Users/Shared/Mounts")

    for dir in "${MOUNT_DIRS[@]}"; do
        if [[ -d "$dir" ]]; then
            check_pass "$dir exists"
        else
            check_info "$dir does not exist (can be created as needed)"
        fi
    done

    # Sambo credentials directory (requires sudo)
    CREDS_DIR="/var/root/.sambo"
    if [[ -d "$CREDS_DIR" ]]; then
        check_pass "Credentials directory exists: $CREDS_DIR"
    else
        check_info "Credentials directory will be created on first CIFS mount"
    fi
}

# Check for optional but useful tools
check_optional_tools() {
    print_section "Optional Tools"

    # Homebrew
    if command -v brew &> /dev/null; then
        check_pass "Homebrew installed"
        BREW_VERSION=$(brew --version | head -n1)
        check_info "$BREW_VERSION"
    else
        check_info "Homebrew not installed (optional)"
        echo -e "      Install from: ${YELLOW}https://brew.sh${NC}"
    fi

    # nmap for network discovery
    if command -v nmap &> /dev/null; then
        check_pass "nmap installed (network scanning)"
    else
        check_info "nmap not installed (optional, for advanced discovery)"
        echo -e "      Install with: ${YELLOW}brew install nmap${NC}"
    fi

    # smbclient for SMB browsing
    if command -v smbclient &> /dev/null; then
        check_pass "smbclient installed (SMB browsing)"
    else
        check_info "smbclient not installed (optional)"
        echo -e "      Install with: ${YELLOW}brew install samba${NC}"
    fi
}

# Test SMB connectivity (optional)
test_smb_server() {
    local server="$1"
    print_section "Testing SMB Server: $server"

    # Try to list shares
    if smbutil view "//$server" 2>/dev/null; then
        check_pass "Successfully connected to $server"
    else
        check_warn "Could not list shares on $server"
        echo "      This may be normal if authentication is required"
    fi
}

# Test NFS connectivity (optional)
test_nfs_server() {
    local server="$1"
    print_section "Testing NFS Server: $server"

    if command -v showmount &> /dev/null; then
        echo "  Available exports:"
        if showmount -e "$server" 2>/dev/null; then
            check_pass "Successfully queried NFS exports"
        else
            check_fail "Could not query NFS exports on $server"
            echo "      Check if the server is running and accessible"
        fi
    else
        check_warn "showmount not available for NFS testing"
    fi
}

# Setup automount for persistent NFS mounts
setup_automount() {
    print_section "Automount Setup"

    if [[ "$EUID" -ne 0 ]]; then
        check_warn "Run with sudo to configure automount"
        echo -e "      ${YELLOW}sudo $0 --setup-automount${NC}"
        return
    fi

    # Create auto_nfs file if it doesn't exist
    if [[ ! -f /etc/auto_nfs ]]; then
        touch /etc/auto_nfs
        chmod 644 /etc/auto_nfs
        check_pass "Created /etc/auto_nfs"
    else
        check_info "/etc/auto_nfs already exists"
    fi

    # Add to auto_master if not present
    if ! grep -q "auto_nfs" /etc/auto_master 2>/dev/null; then
        # Backup auto_master
        cp /etc/auto_master /etc/auto_master.backup
        check_info "Backed up /etc/auto_master"

        # Add auto_nfs entry
        echo "/- auto_nfs -nosuid,nodev" >> /etc/auto_master
        check_pass "Added auto_nfs to auto_master"

        # Reload automount
        automount -vc 2>/dev/null || true
        check_pass "Reloaded automount configuration"
    else
        check_info "auto_nfs already in auto_master"
    fi
}

# Print summary
print_summary() {
    echo ""
    echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  Summary${NC}"
    echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${GREEN}Passed:${NC}  $CHECKS_PASSED"
    echo -e "  ${YELLOW}Warnings:${NC} $CHECKS_WARNED"
    echo -e "  ${RED}Failed:${NC}  $CHECKS_FAILED"
    echo ""

    if [[ $CHECKS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}Your macOS system is ready for network mounts!${NC}"
    else
        echo -e "${YELLOW}Some requirements are missing. Please address the issues above.${NC}"
    fi
    echo ""
}

# Print usage
print_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --help              Show this help message"
    echo "  --test-smb SERVER   Test SMB connectivity to a server"
    echo "  --test-nfs SERVER   Test NFS connectivity to a server"
    echo "  --setup-automount   Configure automount for persistent NFS (requires sudo)"
    echo "  --install-optional  Install optional tools via Homebrew"
    echo ""
    echo "Examples:"
    echo "  $0                          # Run all checks"
    echo "  $0 --test-smb 192.168.1.10  # Test SMB server"
    echo "  $0 --test-nfs 192.168.1.10  # Test NFS server"
    echo "  sudo $0 --setup-automount   # Setup automount"
    echo ""
}

# Install optional tools
install_optional() {
    print_section "Installing Optional Tools"

    if ! command -v brew &> /dev/null; then
        check_fail "Homebrew is required for installing optional tools"
        echo -e "      Install from: ${YELLOW}https://brew.sh${NC}"
        return 1
    fi

    echo "  Installing nmap..."
    brew install nmap 2>/dev/null && check_pass "nmap installed" || check_warn "nmap installation failed"

    echo "  Installing samba (for smbclient)..."
    brew install samba 2>/dev/null && check_pass "samba installed" || check_warn "samba installation failed"
}

# Main
main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                print_usage
                exit 0
                ;;
            --test-smb)
                shift
                if [[ -z "$1" ]]; then
                    echo "Error: --test-smb requires a server address"
                    exit 1
                fi
                print_header
                check_macos
                test_smb_server "$1"
                print_summary
                exit 0
                ;;
            --test-nfs)
                shift
                if [[ -z "$1" ]]; then
                    echo "Error: --test-nfs requires a server address"
                    exit 1
                fi
                print_header
                check_macos
                test_nfs_server "$1"
                print_summary
                exit 0
                ;;
            --setup-automount)
                print_header
                check_macos
                setup_automount
                print_summary
                exit 0
                ;;
            --install-optional)
                print_header
                check_macos
                install_optional
                print_summary
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                print_usage
                exit 1
                ;;
        esac
    done

    # Default: run all checks
    print_header
    check_macos
    check_smb_requirements
    check_nfs_requirements
    check_general_requirements
    check_network_requirements
    check_directories
    check_optional_tools
    print_summary
}

main "$@"
