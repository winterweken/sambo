package platform

import (
	"os/exec"
	"runtime"
)

// OS represents the detected operating system
type OS string

const (
	Linux   OS = "linux"
	MacOS   OS = "darwin"
	Unknown OS = "unknown"
)

// Current returns the current operating system
func Current() OS {
	switch runtime.GOOS {
	case "linux":
		return Linux
	case "darwin":
		return MacOS
	default:
		return Unknown
	}
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// HasHomebrew returns true if Homebrew is installed (macOS)
func HasHomebrew() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

// SambaConfigPath returns the path to smb.conf based on OS
func SambaConfigPath() string {
	if IsMacOS() {
		// Check Homebrew location first
		if _, err := exec.LookPath("/opt/homebrew/etc/smb.conf"); err == nil {
			return "/opt/homebrew/etc/smb.conf"
		}
		if _, err := exec.LookPath("/usr/local/etc/smb.conf"); err == nil {
			return "/usr/local/etc/smb.conf"
		}
		// Default macOS Homebrew Intel path
		return "/usr/local/etc/smb.conf"
	}
	return "/etc/samba/smb.conf"
}

// SambaConfigDir returns the directory containing smb.conf
func SambaConfigDir() string {
	if IsMacOS() {
		// Check for Apple Silicon Homebrew first
		if _, err := exec.LookPath("/opt/homebrew/bin/brew"); err == nil {
			return "/opt/homebrew/etc"
		}
		return "/usr/local/etc"
	}
	return "/etc/samba"
}

// NFSExportsPath returns the path to the exports file
func NFSExportsPath() string {
	// Same on both Linux and macOS
	return "/etc/exports"
}

// CredentialsDir returns the directory for storing credentials
func CredentialsDir() string {
	if IsMacOS() {
		return "/var/root/.sambo"
	}
	return "/root/.sambo"
}

// FstabPath returns the path to fstab (Linux only, empty on macOS)
func FstabPath() string {
	if IsMacOS() {
		// macOS doesn't use fstab for network mounts
		return ""
	}
	return "/etc/fstab"
}

// SupportsPersistentMounts returns whether the OS supports persistent mounts
func SupportsPersistentMounts() bool {
	return true // Both Linux (fstab) and macOS (auto_nfs) support persistence
}

// AutoNFSPath returns the path to auto_nfs (macOS only)
func AutoNFSPath() string {
	if IsMacOS() {
		return "/etc/auto_nfs"
	}
	return ""
}

// ServiceManager returns the service management system name
func ServiceManager() string {
	if IsMacOS() {
		return "launchctl"
	}
	return "systemctl"
}
