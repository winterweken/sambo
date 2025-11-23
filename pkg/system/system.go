package system

import (
	"fmt"
	"os/exec"
	"strings"
)

// PackageManager represents a system package manager
type PackageManager string

const (
	APT    PackageManager = "apt"
	YUM    PackageManager = "yum"
	DNF    PackageManager = "dnf"
	PACMAN PackageManager = "pacman"
	ZYPPER PackageManager = "zypper"
	UNKNOWN PackageManager = "unknown"
)

// DetectPackageManager detects which package manager is available
func DetectPackageManager() PackageManager {
	managers := []struct {
		name PackageManager
		cmd  string
	}{
		{APT, "apt-get"},
		{DNF, "dnf"},
		{YUM, "yum"},
		{PACMAN, "pacman"},
		{ZYPPER, "zypper"},
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

// IsNFSInstalled checks if NFS server is installed
func IsNFSInstalled() bool {
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

// IsCIFSInstalled checks if CIFS utils are installed (for mounting)
func IsCIFSInstalled() bool {
	if _, err := exec.LookPath("mount.cifs"); err != nil {
		return false
	}
	return true
}

// IsNFSClientInstalled checks if NFS client is installed (for mounting)
func IsNFSClientInstalled() bool {
	if _, err := exec.LookPath("mount.nfs"); err != nil {
		return false
	}
	return true
}

// GetSambaPackages returns the package names for Samba based on package manager
func GetSambaPackages(pm PackageManager) []string {
	switch pm {
	case APT:
		return []string{"samba"}
	case YUM, DNF:
		return []string{"samba"}
	case PACMAN:
		return []string{"samba"}
	case ZYPPER:
		return []string{"samba"}
	default:
		return []string{"samba"}
	}
}

// GetNFSPackages returns the package names for NFS server based on package manager
func GetNFSPackages(pm PackageManager) []string {
	switch pm {
	case APT:
		return []string{"nfs-kernel-server"}
	case YUM, DNF:
		return []string{"nfs-utils"}
	case PACMAN:
		return []string{"nfs-utils"}
	case ZYPPER:
		return []string{"nfs-kernel-server"}
	default:
		return []string{"nfs-utils"}
	}
}

// GetCIFSPackages returns the package names for CIFS client based on package manager
func GetCIFSPackages(pm PackageManager) []string {
	switch pm {
	case APT:
		return []string{"cifs-utils"}
	case YUM, DNF:
		return []string{"cifs-utils"}
	case PACMAN:
		return []string{"cifs-utils"}
	case ZYPPER:
		return []string{"cifs-utils"}
	default:
		return []string{"cifs-utils"}
	}
}

// GetNFSClientPackages returns the package names for NFS client based on package manager
func GetNFSClientPackages(pm PackageManager) []string {
	switch pm {
	case APT:
		return []string{"nfs-common"}
	case YUM, DNF:
		return []string{"nfs-utils"}
	case PACMAN:
		return []string{"nfs-utils"}
	case ZYPPER:
		return []string{"nfs-client"}
	default:
		return []string{"nfs-utils"}
	}
}

// InstallPackages installs packages using the detected package manager
func InstallPackages(packages []string) error {
	pm := DetectPackageManager()
	if pm == UNKNOWN {
		return fmt.Errorf("unable to detect package manager")
	}

	var cmd *exec.Cmd

	switch pm {
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

	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installation failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// EnableService enables and starts a systemd service
func EnableService(service string) error {
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

// GetSambaServiceName returns the appropriate Samba service name
func GetSambaServiceName() string {
	// Check which service exists
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
