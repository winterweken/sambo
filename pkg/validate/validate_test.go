package validate

import (
	"strings"
	"testing"
)

func TestShareName_ValidNames(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple lowercase", "myshare"},
		{"with numbers", "share123"},
		{"with underscore", "my_share"},
		{"with hyphen", "my-share"},
		{"mixed case", "MyShare"},
		{"single char", "a"},
		{"max length 80", strings.Repeat("a", 80)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShareName(tt.input); err != nil {
				t.Errorf("ShareName(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestShareName_InvalidNames(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty string", "", "share name is required"},
		{"too long", strings.Repeat("a", 81), "80 characters or less"},
		{"with spaces", "my share", "can only contain letters"},
		{"with dot", "my.share", "can only contain letters"},
		{"with slash", "my/share", "can only contain letters"},
		{"special chars", "share@!", "can only contain letters"},
		{"reserved global", "global", "reserved by Samba"},
		{"reserved homes", "homes", "reserved by Samba"},
		{"reserved printers", "printers", "reserved by Samba"},
		{"reserved case insensitive", "GLOBAL", "reserved by Samba"},
		{"reserved mixed case", "Homes", "reserved by Samba"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ShareName(tt.input)
			if err == nil {
				t.Errorf("ShareName(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("ShareName(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestPath_ValidPaths(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"home directory", "/home/user/share"},
		{"mnt directory", "/mnt/data"},
		{"srv directory", "/srv/samba"},
		{"nested path", "/opt/some/deep/path"},
		{"tmp directory", "/tmp/share"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Path(tt.input); err != nil {
				t.Errorf("Path(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestPath_InvalidPaths(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "path is required"},
		{"relative path", "relative/path", "must be an absolute path"},
		{"root", "/", "critical system directory"},
		{"etc", "/etc", "critical system directory"},
		{"bin", "/bin", "critical system directory"},
		{"sbin", "/sbin", "critical system directory"},
		{"usr", "/usr", "critical system directory"},
		{"dev", "/dev", "critical system directory"},
		{"proc", "/proc", "critical system directory"},
		{"sys", "/sys", "critical system directory"},
		{"boot", "/boot", "critical system directory"},
		{"root home", "/root", "critical system directory"},
		{"var run", "/var/run", "critical system directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Path(tt.input)
			if err == nil {
				t.Errorf("Path(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Path(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestMountPoint_ValidPaths(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"mnt path", "/mnt/share"},
		{"media path", "/media/user/disk"},
		{"home mount", "/home/user/mount"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := MountPoint(tt.input); err != nil {
				t.Errorf("MountPoint(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestMountPoint_InvalidPaths(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "mount point is required"},
		{"relative", "relative/path", "must be an absolute path"},
		{"root", "/", "critical system directory"},
		{"etc", "/etc", "critical system directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MountPoint(tt.input)
			if err == nil {
				t.Errorf("MountPoint(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("MountPoint(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestCIFSSource_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"basic", "//server/share"},
		{"IP address", "//192.168.1.100/data"},
		{"nested path", "//server/share/subdir"},
		{"hostname with domain", "//myserver.local/backup"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CIFSSource(tt.input); err != nil {
				t.Errorf("CIFSSource(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestCIFSSource_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "source is required"},
		{"no prefix", "server/share", "must start with //"},
		{"single slash", "/server/share", "must start with //"},
		{"no share", "//server", "must be in format //server/share"},
		{"empty server", "///share", "must be in format //server/share"},
		{"empty share", "//server/", "must be in format //server/share"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CIFSSource(tt.input)
			if err == nil {
				t.Errorf("CIFSSource(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("CIFSSource(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestNFSSource_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"basic", "server:/export"},
		{"IP address", "192.168.1.100:/data"},
		{"nested path", "server:/export/subdir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NFSSource(tt.input); err != nil {
				t.Errorf("NFSSource(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestNFSSource_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "source is required"},
		{"no colon", "server/export", "must be in format server:/path"},
		{"empty server", ":/export", "must be in format server:/path"},
		{"empty path", "server:", "must be in format server:/path"},
		{"relative path", "server:relative", "must be absolute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NFSSource(tt.input)
			if err == nil {
				t.Errorf("NFSSource(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("NFSSource(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestUsername_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "alice"},
		{"with underscore prefix", "_admin"},
		{"with numbers", "user123"},
		{"with hyphen", "my-user"},
		{"max length 32", strings.Repeat("a", 32)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Username(tt.input); err != nil {
				t.Errorf("Username(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestUsername_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "username is required"},
		{"too long", strings.Repeat("a", 33), "32 characters or less"},
		{"starts with number", "1user", "must start with a letter"},
		{"starts with hyphen", "-user", "must start with a letter"},
		{"with spaces", "my user", "must start with a letter"},
		{"with special chars", "user@host", "must start with a letter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Username(tt.input)
			if err == nil {
				t.Errorf("Username(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Username(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestPassword_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"minimum length", "abcd"},
		{"long password", "averylongpasswordthatisvalid"},
		{"with special chars", "p@ss!w0rd"},
		{"with spaces", "pass word"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Password(tt.input); err != nil {
				t.Errorf("Password(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestPassword_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "password is required"},
		{"too short 1", "a", "at least 4 characters"},
		{"too short 3", "abc", "at least 4 characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Password(tt.input)
			if err == nil {
				t.Errorf("Password(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Password(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestNFSClients_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"wildcard", "*"},
		{"IP address", "192.168.1.100"},
		{"CIDR", "192.168.1.0/24"},
		{"hostname", "myhost.local"},
		{"multiple IPs comma", "192.168.1.100,192.168.1.101"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NFSClients(tt.input); err != nil {
				t.Errorf("NFSClients(%q) returned error: %v", tt.input, err)
			}
		})
	}
}

func TestNFSClients_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "clients specification is required"},
		{"brackets", "[bad]", "invalid characters"},
		{"braces", "{bad}", "invalid characters"},
		{"parens", "(bad)", "invalid characters"},
		{"semicolons", "host;rm -rf", "invalid characters"},
		{"backslash", `host\bad`, "invalid characters"},
		{"pipe", "host|bad", "invalid characters"},
		{"single quote", "host'bad", "invalid characters"},
		{"double quote", `host"bad`, "invalid characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NFSClients(tt.input)
			if err == nil {
				t.Errorf("NFSClients(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("NFSClients(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestPath_NormalizesTrailingSlash(t *testing.T) {
	// filepath.Clean removes trailing slashes, so /etc/ becomes /etc
	err := Path("/etc/")
	if err == nil {
		t.Error("Path('/etc/') should return error after normalization")
	}
	if !strings.Contains(err.Error(), "critical system directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPath_SubdirOfCriticalPathIsAllowed(t *testing.T) {
	// /etc/samba is a subdirectory of /etc and should be allowed
	if err := Path("/etc/samba"); err != nil {
		t.Errorf("Path('/etc/samba') should be valid but got: %v", err)
	}
}

// TestNFSClients_RejectsDangerousChars verifies Bug #13 fix:
// NFSClients must reject backtick, $, &, <, > and newline characters
// which could enable command injection.
func TestNFSClients_RejectsDangerousChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"backtick", "host`whoami`"},
		{"dollar sign", "host$USER"},
		{"ampersand", "host&rm"},
		{"less than", "host<file"},
		{"greater than", "host>file"},
		{"newline", "host\nrm -rf /"},
		{"carriage return", "host\revil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NFSClients(tt.input)
			if err == nil {
				t.Errorf("NFSClients(%q) should return error for dangerous character, got nil", tt.input)
			}
		})
	}
}
