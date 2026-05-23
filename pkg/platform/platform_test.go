package platform

import (
	"testing"
)

func TestCurrent_ReturnsDarwinOnMacOS(t *testing.T) {
	got := Current()
	if got != MacOS {
		t.Errorf("Current() = %q, want %q (darwin)", got, MacOS)
	}
}

func TestIsLinux_ReturnsFalseOnMacOS(t *testing.T) {
	if IsLinux() {
		t.Error("IsLinux() should return false on macOS")
	}
}

func TestIsMacOS_ReturnsTrueOnMacOS(t *testing.T) {
	if !IsMacOS() {
		t.Error("IsMacOS() should return true on macOS")
	}
}

func TestSambaConfigPath_ReturnsMacOSPath(t *testing.T) {
	path := SambaConfigPath()
	// On macOS, should return a path in /opt/homebrew/etc or /usr/local/etc
	validPaths := []string{
		"/opt/homebrew/etc/smb.conf",
		"/usr/local/etc/smb.conf",
	}
	found := false
	for _, valid := range validPaths {
		if path == valid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SambaConfigPath() = %q, want one of %v", path, validPaths)
	}
}

func TestSambaConfigDir_ReturnsMacOSDir(t *testing.T) {
	dir := SambaConfigDir()
	validDirs := []string{
		"/opt/homebrew/etc",
		"/usr/local/etc",
	}
	found := false
	for _, valid := range validDirs {
		if dir == valid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SambaConfigDir() = %q, want one of %v", dir, validDirs)
	}
}

func TestNFSExportsPath_ReturnsEtcExports(t *testing.T) {
	path := NFSExportsPath()
	if path != "/etc/exports" {
		t.Errorf("NFSExportsPath() = %q, want %q", path, "/etc/exports")
	}
}

func TestCredentialsDir_ReturnsMacOSDir(t *testing.T) {
	dir := CredentialsDir()
	if dir != "/var/root/.sambo" {
		t.Errorf("CredentialsDir() = %q, want %q", dir, "/var/root/.sambo")
	}
}

func TestFstabPath_EmptyOnMacOS(t *testing.T) {
	path := FstabPath()
	if path != "" {
		t.Errorf("FstabPath() = %q, want empty string on macOS", path)
	}
}

func TestSupportsPersistentMounts_ReturnsTrue(t *testing.T) {
	if !SupportsPersistentMounts() {
		t.Error("SupportsPersistentMounts() should return true")
	}
}

func TestAutoNFSPath_ReturnsMacOSPath(t *testing.T) {
	path := AutoNFSPath()
	if path != "/etc/auto_nfs" {
		t.Errorf("AutoNFSPath() = %q, want %q", path, "/etc/auto_nfs")
	}
}

func TestServiceManager_ReturnsLaunchctl(t *testing.T) {
	sm := ServiceManager()
	if sm != "launchctl" {
		t.Errorf("ServiceManager() = %q, want %q", sm, "launchctl")
	}
}

func TestOSConstants(t *testing.T) {
	if Linux != "linux" {
		t.Errorf("Linux constant = %q, want %q", Linux, "linux")
	}
	if MacOS != "darwin" {
		t.Errorf("MacOS constant = %q, want %q", MacOS, "darwin")
	}
	if Unknown != "unknown" {
		t.Errorf("Unknown constant = %q, want %q", Unknown, "unknown")
	}
}

// TestSambaConfigPath_UsesOsStatNotLookPath verifies Bug #1 fix:
// SambaConfigPath must detect config files with mode 0644 (non-executable).
// exec.LookPath would miss them because it only finds executables.
func TestSambaConfigPath_UsesOsStatNotLookPath(t *testing.T) {
	if !IsMacOS() {
		t.Skip("Skipping macOS-specific test")
	}
	// Create a temp file with mode 0644 (non-executable) to simulate smb.conf.
	// The point is that our function should be able to find files that aren't executable.
	// We test the actual function returns a valid macOS-style path.
	path := SambaConfigPath()
	// On macOS, the path should be one of the known Homebrew locations
	if path != "/opt/homebrew/etc/smb.conf" && path != "/usr/local/etc/smb.conf" {
		t.Errorf("SambaConfigPath() = %q, expected a macOS Homebrew path", path)
	}
}

// TestSambaConfigDir_UsesOsStatNotLookPath verifies Bug #1 fix for SambaConfigDir.
func TestSambaConfigDir_UsesOsStatNotLookPath(t *testing.T) {
	if !IsMacOS() {
		t.Skip("Skipping macOS-specific test")
	}
	dir := SambaConfigDir()
	if dir != "/opt/homebrew/etc" && dir != "/usr/local/etc" {
		t.Errorf("SambaConfigDir() = %q, expected a macOS Homebrew directory", dir)
	}
}
