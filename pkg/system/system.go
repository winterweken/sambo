package system

import (
	"fmt"
	"os/exec"
	"strings"

	"sambo/pkg/platform"
)

// PackageManager represents a system package manager
type PackageManager string

const (
	APT      PackageManager = "apt"
	YUM      PackageManager = "yum"
	DNF      PackageManager = "dnf"
	PACMAN   PackageManager = "pacman"
	ZYPPER   PackageManager = "zypper"
	APK      PackageManager = "apk"
	HOMEBREW PackageManager = "homebrew"
	UNKNOWN  PackageManager = "unknown"
)

// DetectPackageManager detects which package manager is available
func DetectPackageManager() PackageManager {
	// Check for macOS first
	if platform.IsMacOS() {
		if _, err := exec.LookPath("brew"); err == nil {
			return HOMEBREW
		}
		return UNKNOWN
	}

	// Linux package managers
	managers := []struct {
		name PackageManager
		cmd  string
	}{
		{APT, "apt-get"},
		{DNF, "dnf"},
		{YUM, "yum"},
		{PACMAN, "pacman"},
		{ZYPPER, "zypper"},
		{APK, "apk"},
	}

	for _, mgr := range managers {
		if _, err := exec.LookPath(mgr.cmd); err == nil {
			return mgr.name
		}
	}

	return UNKNOWN
}

// IsSambaInstalled checks if Samba is installed
func IsSambaInstalled() bool {
	if platform.IsMacOS() {
		return isSambaInstalledMacOS()
	}
	return isSambaInstalledLinux()
}

// isSambaInstalledLinux checks if Samba is installed on Linux
func isSambaInstalledLinux() bool {
	// Check for smbd daemon
	if _, err := exec.LookPath("smbd"); err != nil {
		return false
	}

	// Check for smbpasswd
	if _, err := exec.LookPath("smbpasswd"); err != nil {
		return false
	}

	// Check for testparm
	if _, err := exec.LookPath("testparm"); err != nil {
		return false
	}

	return true
}

// isSambaInstalledMacOS checks if Samba is installed on macOS
func isSambaInstalledMacOS() bool {
	// Check for Homebrew samba installation
	if _, err := exec.LookPath("smbd"); err == nil {
		return true
	}

	// Check if macOS built-in file sharing is available
	cmd := exec.Command("launchctl", "list", "com.apple.smbd")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// IsNFSInstalled checks if NFS server is installed
func IsNFSInstalled() bool {
	if platform.IsMacOS() {
		return isNFSInstalledMacOS()
	}
	return isNFSInstalledLinux()
}

// isNFSInstalledLinux checks if NFS server is installed on Linux
func isNFSInstalledLinux() bool {
	// Check for exportfs
	if _, err := exec.LookPath("exportfs"); err != nil {
		return false
	}

	// Check if nfs-kernel-server or nfs-server service exists
	cmd := exec.Command("systemctl", "list-unit-files", "--type=service")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	services := string(output)
	return strings.Contains(services, "nfs-server.service") ||
		strings.Contains(services, "nfs-kernel-server.service")
}

// isNFSInstalledMacOS checks if NFS server is available on macOS
func isNFSInstalledMacOS() bool {
	// macOS has built-in NFS server via nfsd
	if _, err := exec.LookPath("nfsd"); err == nil {
		return true
	}
	return false
}

// IsCIFSInstalled checks if CIFS utils are installed (for mounting)
func IsCIFSInstalled() bool {
	if platform.IsMacOS() {
		// macOS has built-in SMB/CIFS support via mount_smbfs
		if _, err := exec.LookPath("mount_smbfs"); err == nil {
			return true
		}
		return false
	}
	// Linux uses mount.cifs
	if _, err := exec.LookPath("mount.cifs"); err != nil {
		return false
	}
	return true
}

// IsNFSClientInstalled checks if NFS client is installed (for mounting)
func IsNFSClientInstalled() bool {
	if platform.IsMacOS() {
		// macOS has built-in NFS client support
		if _, err := exec.LookPath("mount_nfs"); err == nil {
			return true
		}
		return false
	}
	// Linux uses mount.nfs
	if _, err := exec.LookPath("mount.nfs"); err != nil {
		return false
	}
	return true
}

// IsAvahiInstalled checks if Avahi (mDNS/Bonjour) is installed
// Avahi is required for Time Machine to auto-discover Samba shares
func IsAvahiInstalled() bool {
	if platform.IsMacOS() {
		// macOS has built-in Bonjour (mDNSResponder)
		return true
	}
	// Linux uses avahi-daemon
	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		return false
	}
	return true
}

// GetSambaPackages returns the package names for Samba based on package manager
func GetSambaPackages(pm PackageManager) []string {
	switch pm {
	case HOMEBREW:
		return []string{"samba"}
	case APT:
		return []string{"samba"}
	case YUM, DNF:
		return []string{"samba", "samba-common-tools"}
	case PACMAN:
		return []string{"samba"}
	case ZYPPER:
		return []string{"samba"}
	case APK:
		return []string{"samba"}
	default:
		return []string{"samba"}
	}
}

// GetNFSPackages returns the package names for NFS server based on package manager
func GetNFSPackages(pm PackageManager) []string {
	switch pm {
	case HOMEBREW:
		// macOS has built-in NFS server, no package needed
		return []string{}
	case APT:
		return []string{"nfs-kernel-server"}
	case YUM, DNF:
		return []string{"nfs-utils"}
	case PACMAN:
		return []string{"nfs-utils"}
	case ZYPPER:
		return []string{"nfs-kernel-server"}
	case APK:
		return []string{"nfs-utils"}
	default:
		return []string{"nfs-utils"}
	}
}

// GetCIFSPackages returns the package names for CIFS client based on package manager
func GetCIFSPackages(pm PackageManager) []string {
	switch pm {
	case HOMEBREW:
		// macOS has built-in SMB/CIFS client support, no package needed
		return []string{}
	case APT:
		return []string{"cifs-utils"}
	case YUM, DNF:
		return []string{"cifs-utils"}
	case PACMAN:
		return []string{"cifs-utils"}
	case ZYPPER:
		return []string{"cifs-utils"}
	case APK:
		return []string{"cifs-utils"}
	default:
		return []string{"cifs-utils"}
	}
}

// GetNFSClientPackages returns the package names for NFS client based on package manager
func GetNFSClientPackages(pm PackageManager) []string {
	switch pm {
	case HOMEBREW:
		// macOS has built-in NFS client support, no package needed
		return []string{}
	case APT:
		return []string{"nfs-common"}
	case YUM, DNF:
		return []string{"nfs-utils"}
	case PACMAN:
		return []string{"nfs-utils"}
	case ZYPPER:
		return []string{"nfs-client"}
	case APK:
		return []string{"nfs-utils"}
	default:
		return []string{"nfs-utils"}
	}
}

// GetAvahiPackages returns the package names for Avahi based on package manager
// Avahi provides mDNS/Bonjour support for Time Machine discovery
func GetAvahiPackages(pm PackageManager) []string {
	switch pm {
	case HOMEBREW:
		// macOS has built-in Bonjour, no package needed
		return []string{}
	case APT:
		return []string{"avahi-daemon"}
	case YUM, DNF:
		return []string{"avahi"}
	case PACMAN:
		return []string{"avahi"}
	case ZYPPER:
		return []string{"avahi"}
	case APK:
		return []string{"avahi", "dbus"}
	default:
		return []string{"avahi"}
	}
}

// GetAvahiServiceName returns the appropriate Avahi service name
func GetAvahiServiceName() string {
	if platform.IsMacOS() {
		return "" // macOS uses built-in mDNSResponder
	}
	return "avahi-daemon"
}

// InstallPackages installs packages using the detected package manager
func InstallPackages(packages []string) error {
	if len(packages) == 0 {
		return nil // Nothing to install (e.g., built-in macOS features)
	}

	pm := DetectPackageManager()
	if pm == UNKNOWN {
		return fmt.Errorf("unable to detect package manager")
	}

	var cmd *exec.Cmd

	switch pm {
	case HOMEBREW:
		args := append([]string{"install"}, packages...)
		cmd = exec.Command("brew", args...)

	case APT:
		// Update package list first
		updateCmd := exec.Command("apt-get", "update")
		if err := updateCmd.Run(); err != nil {
			return fmt.Errorf("failed to update package list: %w", err)
		}
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("apt-get", args...)

	case DNF:
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("dnf", args...)

	case YUM:
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("yum", args...)

	case PACMAN:
		args := append([]string{"-S", "--noconfirm"}, packages...)
		cmd = exec.Command("pacman", args...)

	case ZYPPER:
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("zypper", args...)

	case APK:
		args := append([]string{"add", "--no-cache"}, packages...)
		cmd = exec.Command("apk", args...)

	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installation failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// EnableService enables and starts a service
func EnableService(service string) error {
	if platform.IsMacOS() {
		return enableServiceMacOS(service)
	}
	return enableServiceLinux(service)
}

// enableServiceLinux enables and starts a systemd service on Linux
func enableServiceLinux(service string) error {
	// Enable the service
	enableCmd := exec.Command("systemctl", "enable", service)
	if err := enableCmd.Run(); err != nil {
		return fmt.Errorf("failed to enable %s: %w", service, err)
	}

	// Start the service
	startCmd := exec.Command("systemctl", "start", service)
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to start %s: %w", service, err)
	}

	return nil
}

// enableServiceMacOS enables and starts a service on macOS
func enableServiceMacOS(service string) error {
	// Handle common service names
	switch service {
	case "smbd", "smb", "samba":
		// Try Homebrew services first
		cmd := exec.Command("brew", "services", "start", "samba")
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fall back to direct start
		cmd = exec.Command("smbd", "-D")
		return cmd.Run()

	case "nfsd", "nfs-server", "nfs":
		// Enable and start nfsd
		enableCmd := exec.Command("nfsd", "enable")
		if err := enableCmd.Run(); err != nil {
			return fmt.Errorf("failed to enable nfsd: %w", err)
		}
		startCmd := exec.Command("nfsd", "start")
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("failed to start nfsd: %w", err)
		}
		return nil

	default:
		// Try launchctl for other services
		cmd := exec.Command("launchctl", "load", "-w", fmt.Sprintf("/System/Library/LaunchDaemons/%s.plist", service))
		if err := cmd.Run(); err != nil {
			// Try homebrew services
			cmd = exec.Command("brew", "services", "start", service)
			return cmd.Run()
		}
		return nil
	}
}

// GetSambaServiceName returns the appropriate Samba service name
func GetSambaServiceName() string {
	if platform.IsMacOS() {
		return "samba" // Homebrew service name
	}

	// Linux: Check which service exists
	cmd := exec.Command("systemctl", "list-unit-files", "--type=service")
	output, err := cmd.Output()
	if err != nil {
		return "smbd" // default
	}

	services := string(output)
	if strings.Contains(services, "smbd.service") {
		return "smbd"
	}
	if strings.Contains(services, "smb.service") {
		return "smb"
	}
	return "smbd"
}

// GetNFSServiceName returns the appropriate NFS service name
func GetNFSServiceName() string {
	if platform.IsMacOS() {
		return "nfsd" // macOS built-in NFS server
	}

	// Linux: Check which service exists
	cmd := exec.Command("systemctl", "list-unit-files", "--type=service")
	output, err := cmd.Output()
	if err != nil {
		return "nfs-server" // default
	}

	services := string(output)
	if strings.Contains(services, "nfs-kernel-server.service") {
		return "nfs-kernel-server"
	}
	if strings.Contains(services, "nfs-server.service") {
		return "nfs-server"
	}
	return "nfs-server"
}
