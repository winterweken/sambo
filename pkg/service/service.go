package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sambo/pkg/platform"
)

// Status represents the status of a service
type Status int

const (
	StatusUnknown Status = iota
	StatusRunning
	StatusStopped
	StatusNotInstalled
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusNotInstalled:
		return "not installed"
	default:
		return "unknown"
	}
}

// CheckSamba checks if the Samba service is running
func CheckSamba() Status {
	if platform.IsMacOS() {
		return checkSambaMacOS()
	}
	return checkSambaLinux()
}

// checkSambaLinux checks Samba status on Linux using systemd
func checkSambaLinux() Status {
	// Try smbd first (common on most distros)
	if checkSystemdService("smbd") == StatusRunning {
		return StatusRunning
	}
	// Try samba (some distros use this name)
	if checkSystemdService("samba") == StatusRunning {
		return StatusRunning
	}
	// Try smb (RHEL/CentOS)
	if checkSystemdService("smb") == StatusRunning {
		return StatusRunning
	}

	// Check if samba is installed at all
	if _, err := exec.LookPath("smbd"); err != nil {
		return StatusNotInstalled
	}

	return StatusStopped
}

// checkSambaMacOS checks Samba status on macOS
func checkSambaMacOS() Status {
	// Check if smbd binary exists (Homebrew installation)
	if _, err := exec.LookPath("smbd"); err != nil {
		// Check for macOS built-in file sharing
		cmd := exec.Command("launchctl", "list", "com.apple.smbd")
		if err := cmd.Run(); err == nil {
			return StatusRunning
		}
		return StatusNotInstalled
	}

	// Check if Homebrew samba is running
	cmd := exec.Command("pgrep", "-x", "smbd")
	if err := cmd.Run(); err == nil {
		return StatusRunning
	}

	// Check launchctl for samba service
	cmd = exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err == nil {
		if strings.Contains(string(output), "smbd") || strings.Contains(string(output), "samba") {
			return StatusRunning
		}
	}

	return StatusStopped
}

// CheckNFS checks if the NFS service is running
func CheckNFS() Status {
	if platform.IsMacOS() {
		return checkNFSMacOS()
	}
	return checkNFSLinux()
}

// checkNFSLinux checks NFS status on Linux using systemd
func checkNFSLinux() Status {
	// Try nfs-server (most common)
	if checkSystemdService("nfs-server") == StatusRunning {
		return StatusRunning
	}
	// Try nfs-kernel-server (Debian/Ubuntu)
	if checkSystemdService("nfs-kernel-server") == StatusRunning {
		return StatusRunning
	}
	// Try nfs (older systems)
	if checkSystemdService("nfs") == StatusRunning {
		return StatusRunning
	}

	// Check if NFS is installed
	if _, err := exec.LookPath("exportfs"); err != nil {
		return StatusNotInstalled
	}

	return StatusStopped
}

// checkNFSMacOS checks NFS status on macOS
func checkNFSMacOS() Status {
	// macOS has built-in NFS server via nfsd
	cmd := exec.Command("nfsd", "status")
	output, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(output), "is running") {
		return StatusRunning
	}

	// Check if nfsd exists
	if _, err := exec.LookPath("nfsd"); err != nil {
		return StatusNotInstalled
	}

	// Check if exports file exists and has content
	if _, err := os.Stat("/etc/exports"); err != nil {
		return StatusNotInstalled
	}

	return StatusStopped
}

// checkSystemdService checks if a systemd service is running (Linux only)
func checkSystemdService(name string) Status {
	cmd := exec.Command("systemctl", "is-active", "--quiet", name)
	err := cmd.Run()
	if err == nil {
		return StatusRunning
	}

	// Check if service exists
	cmd = exec.Command("systemctl", "list-unit-files", name+".service")
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return StatusNotInstalled
	}

	return StatusStopped
}

// EnsureSambaRunning returns an error if Samba is not running
func EnsureSambaRunning() error {
	status := CheckSamba()
	switch status {
	case StatusNotInstalled:
		if platform.IsMacOS() {
			return fmt.Errorf("Samba is not installed. Install with: brew install samba")
		}
		return fmt.Errorf("Samba is not installed. Please install samba package first")
	case StatusStopped:
		if platform.IsMacOS() {
			return fmt.Errorf("Samba service is not running. Start it with: brew services start samba")
		}
		return fmt.Errorf("Samba service is not running. Start it with: sudo systemctl start smbd")
	case StatusUnknown:
		return fmt.Errorf("unable to determine Samba service status")
	}
	return nil
}

// EnsureNFSRunning returns an error if NFS is not running
func EnsureNFSRunning() error {
	status := CheckNFS()
	switch status {
	case StatusNotInstalled:
		if platform.IsMacOS() {
			return fmt.Errorf("NFS server is not available. macOS has built-in NFS support via nfsd")
		}
		return fmt.Errorf("NFS server is not installed. Please install nfs-kernel-server package first")
	case StatusStopped:
		if platform.IsMacOS() {
			return fmt.Errorf("NFS service is not running. Start it with: sudo nfsd enable && sudo nfsd start")
		}
		return fmt.Errorf("NFS service is not running. Start it with: sudo systemctl start nfs-server")
	case StatusUnknown:
		return fmt.Errorf("unable to determine NFS service status")
	}
	return nil
}

// WarnIfNotRunning prints a warning if a service is not running (non-blocking)
func WarnIfNotRunning(serviceName string) {
	var status Status
	switch serviceName {
	case "samba":
		status = CheckSamba()
	case "nfs":
		status = CheckNFS()
	default:
		return
	}

	if status == StatusStopped {
		fmt.Printf("⚠️  Warning: %s service is not running. Changes may not take effect until service is started.\n", serviceName)
	} else if status == StatusNotInstalled {
		fmt.Printf("⚠️  Warning: %s is not installed.\n", serviceName)
	}
}

// StartSamba attempts to start the Samba service
func StartSamba() error {
	if platform.IsMacOS() {
		// Try brew services first
		cmd := exec.Command("brew", "services", "start", "samba")
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fall back to direct smbd start
		cmd = exec.Command("smbd", "-D")
		return cmd.Run()
	}

	// Linux: try systemctl
	cmd := exec.Command("systemctl", "start", "smbd")
	if err := cmd.Run(); err != nil {
		// Try alternative service names
		cmd = exec.Command("systemctl", "start", "smb")
		return cmd.Run()
	}
	return nil
}

// StartNFS attempts to start the NFS service
func StartNFS() error {
	if platform.IsMacOS() {
		cmd := exec.Command("nfsd", "enable")
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("nfsd", "start")
		return cmd.Run()
	}

	// Linux: try systemctl
	cmd := exec.Command("systemctl", "start", "nfs-server")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("systemctl", "start", "nfs-kernel-server")
		return cmd.Run()
	}
	return nil
}

// ReloadSamba reloads the Samba configuration
func ReloadSamba() error {
	if platform.IsMacOS() {
		// Send SIGHUP to smbd to reload config
		cmd := exec.Command("pkill", "-HUP", "smbd")
		return cmd.Run()
	}

	// Linux: use systemctl
	cmd := exec.Command("systemctl", "reload", "smbd")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("systemctl", "reload", "smb")
		if err := cmd.Run(); err != nil {
			// Fallback to service command
			cmd = exec.Command("service", "smbd", "reload")
			return cmd.Run()
		}
	}
	return nil
}

// ReloadNFS reloads the NFS exports
func ReloadNFS() error {
	if platform.IsMacOS() {
		// macOS: use nfsd update
		cmd := exec.Command("nfsd", "update")
		return cmd.Run()
	}

	// Linux: use exportfs
	cmd := exec.Command("exportfs", "-ra")
	if err := cmd.Run(); err != nil {
		// Fallback to systemctl restart
		cmd = exec.Command("systemctl", "restart", "nfs-server")
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("systemctl", "restart", "nfs-kernel-server")
			return cmd.Run()
		}
	}
	return nil
}
