package service

import (
	"fmt"
	"os/exec"
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

// CheckNFS checks if the NFS service is running
func CheckNFS() Status {
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

// checkSystemdService checks if a systemd service is running
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
		return fmt.Errorf("Samba is not installed. Please install samba package first")
	case StatusStopped:
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
		return fmt.Errorf("NFS server is not installed. Please install nfs-kernel-server package first")
	case StatusStopped:
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
