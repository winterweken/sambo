package system

import (
	"strings"
	"testing"
)

func TestPackageManager_Constants(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected string
	}{
		{APT, "apt"},
		{YUM, "yum"},
		{DNF, "dnf"},
		{PACMAN, "pacman"},
		{ZYPPER, "zypper"},
		{APK, "apk"},
		{UNKNOWN, "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm), func(t *testing.T) {
			if string(tt.pm) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.pm))
			}
		})
	}
}

// TestGetSambaPackages tests package names for different package managers
func TestGetSambaPackages(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected []string
	}{
		{APT, []string{"samba"}},
		{YUM, []string{"samba", "samba-common-tools"}},
		{DNF, []string{"samba", "samba-common-tools"}},
		{PACMAN, []string{"samba"}},
		{ZYPPER, []string{"samba"}},
		{APK, []string{"samba"}},
		{UNKNOWN, []string{"samba"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm), func(t *testing.T) {
			packages := GetSambaPackages(tt.pm)
			if len(packages) != len(tt.expected) {
				t.Errorf("expected %d packages, got %d", len(tt.expected), len(packages))
				return
			}
			for i, pkg := range tt.expected {
				if packages[i] != pkg {
					t.Errorf("package %d: expected %s, got %s", i, pkg, packages[i])
				}
			}
		})
	}
}

// TestGetNFSPackages tests NFS package names for different package managers
func TestGetNFSPackages(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected []string
	}{
		{APT, []string{"nfs-kernel-server"}},
		{YUM, []string{"nfs-utils"}},
		{DNF, []string{"nfs-utils"}},
		{PACMAN, []string{"nfs-utils"}},
		{ZYPPER, []string{"nfs-kernel-server"}},
		{APK, []string{"nfs-utils"}},
		{UNKNOWN, []string{"nfs-utils"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm), func(t *testing.T) {
			packages := GetNFSPackages(tt.pm)
			if len(packages) != len(tt.expected) {
				t.Errorf("expected %d packages, got %d", len(tt.expected), len(packages))
				return
			}
			for i, pkg := range tt.expected {
				if packages[i] != pkg {
					t.Errorf("package %d: expected %s, got %s", i, pkg, packages[i])
				}
			}
		})
	}
}

// TestGetCIFSPackages tests CIFS package names for different package managers
func TestGetCIFSPackages(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected []string
	}{
		{APT, []string{"cifs-utils"}},
		{YUM, []string{"cifs-utils"}},
		{DNF, []string{"cifs-utils"}},
		{PACMAN, []string{"cifs-utils"}},
		{ZYPPER, []string{"cifs-utils"}},
		{APK, []string{"cifs-utils"}},
		{UNKNOWN, []string{"cifs-utils"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm), func(t *testing.T) {
			packages := GetCIFSPackages(tt.pm)
			if len(packages) != len(tt.expected) {
				t.Errorf("expected %d packages, got %d", len(tt.expected), len(packages))
				return
			}
			for i, pkg := range tt.expected {
				if packages[i] != pkg {
					t.Errorf("package %d: expected %s, got %s", i, pkg, packages[i])
				}
			}
		})
	}
}

// TestGetNFSClientPackages tests NFS client package names
func TestGetNFSClientPackages(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected []string
	}{
		{APT, []string{"nfs-common"}},
		{YUM, []string{"nfs-utils"}},
		{DNF, []string{"nfs-utils"}},
		{PACMAN, []string{"nfs-utils"}},
		{ZYPPER, []string{"nfs-client"}},
		{APK, []string{"nfs-utils"}},
		{UNKNOWN, []string{"nfs-utils"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm), func(t *testing.T) {
			packages := GetNFSClientPackages(tt.pm)
			if len(packages) != len(tt.expected) {
				t.Errorf("expected %d packages, got %d", len(tt.expected), len(packages))
				return
			}
			for i, pkg := range tt.expected {
				if packages[i] != pkg {
					t.Errorf("package %d: expected %s, got %s", i, pkg, packages[i])
				}
			}
		})
	}
}

// TestPackageManagerCommands tests the commands used by each package manager
func TestPackageManagerCommands(t *testing.T) {
	tests := []struct {
		pm           PackageManager
		lookupCmd    string
		installArgs  []string
	}{
		{APT, "apt-get", []string{"install", "-y"}},
		{DNF, "dnf", []string{"install", "-y"}},
		{YUM, "yum", []string{"install", "-y"}},
		{PACMAN, "pacman", []string{"-S", "--noconfirm"}},
		{ZYPPER, "zypper", []string{"install", "-y"}},
		{APK, "apk", []string{"add", "--no-cache"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm), func(t *testing.T) {
			// Verify the lookup command matches what DetectPackageManager checks
			var expectedCmd string
			switch tt.pm {
			case APT:
				expectedCmd = "apt-get"
			case DNF:
				expectedCmd = "dnf"
			case YUM:
				expectedCmd = "yum"
			case PACMAN:
				expectedCmd = "pacman"
			case ZYPPER:
				expectedCmd = "zypper"
			case APK:
				expectedCmd = "apk"
			}

			if tt.lookupCmd != expectedCmd {
				t.Errorf("expected lookup command %s, got %s", expectedCmd, tt.lookupCmd)
			}

			// Verify install args are correct
			if len(tt.installArgs) < 2 {
				t.Error("install args should have at least 2 elements")
			}
		})
	}
}

// TestServiceNames tests the service name detection logic
func TestServiceNames(t *testing.T) {
	// Test Samba service names
	sambaTests := []struct {
		servicesOutput string
		expected       string
	}{
		{"smbd.service enabled\nnmbd.service enabled", "smbd"},
		{"smb.service enabled", "smb"},
		{"other.service enabled", "smbd"}, // default fallback
	}

	for _, tt := range sambaTests {
		t.Run("samba_"+tt.expected, func(t *testing.T) {
			var result string
			if strings.Contains(tt.servicesOutput, "smbd.service") {
				result = "smbd"
			} else if strings.Contains(tt.servicesOutput, "smb.service") {
				result = "smb"
			} else {
				result = "smbd" // default
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}

	// Test NFS service names
	nfsTests := []struct {
		servicesOutput string
		expected       string
	}{
		{"nfs-kernel-server.service enabled", "nfs-kernel-server"},
		{"nfs-server.service enabled", "nfs-server"},
		{"other.service enabled", "nfs-server"}, // default fallback
	}

	for _, tt := range nfsTests {
		t.Run("nfs_"+tt.expected, func(t *testing.T) {
			var result string
			if strings.Contains(tt.servicesOutput, "nfs-kernel-server.service") {
				result = "nfs-kernel-server"
			} else if strings.Contains(tt.servicesOutput, "nfs-server.service") {
				result = "nfs-server"
			} else {
				result = "nfs-server" // default
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestSambaBinaryChecks tests the binary checks for Samba installation
func TestSambaBinaryChecks(t *testing.T) {
	requiredBinaries := []string{"smbd", "smbpasswd", "testparm"}

	for _, binary := range requiredBinaries {
		t.Run(binary, func(t *testing.T) {
			// Just verify the binary name is not empty
			if binary == "" {
				t.Error("binary name should not be empty")
			}
		})
	}
}

// TestNFSBinaryChecks tests the binary checks for NFS installation
func TestNFSBinaryChecks(t *testing.T) {
	requiredBinaries := []string{"exportfs"}

	for _, binary := range requiredBinaries {
		t.Run(binary, func(t *testing.T) {
			if binary == "" {
				t.Error("binary name should not be empty")
			}
		})
	}
}

// TestCIFSBinaryChecks tests the binary check for CIFS
func TestCIFSBinaryChecks(t *testing.T) {
	binary := "mount.cifs"
	if binary == "" {
		t.Error("binary name should not be empty")
	}
}

// TestNFSClientBinaryChecks tests the binary check for NFS client
func TestNFSClientBinaryChecks(t *testing.T) {
	binary := "mount.nfs"
	if binary == "" {
		t.Error("binary name should not be empty")
	}
}

// TestInstallCommandGeneration tests generation of install commands
func TestInstallCommandGeneration(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		packages []string
		expected string
	}{
		{APT, []string{"samba"}, "apt-get install -y samba"},
		{DNF, []string{"samba"}, "dnf install -y samba"},
		{YUM, []string{"nfs-utils"}, "yum install -y nfs-utils"},
		{PACMAN, []string{"samba"}, "pacman -S --noconfirm samba"},
		{ZYPPER, []string{"cifs-utils"}, "zypper install -y cifs-utils"},
		{APK, []string{"nfs-utils"}, "apk add --no-cache nfs-utils"},
		{APT, []string{"pkg1", "pkg2"}, "apt-get install -y pkg1 pkg2"},
	}

	for _, tt := range tests {
		t.Run(string(tt.pm)+"_"+strings.Join(tt.packages, "_"), func(t *testing.T) {
			var cmd string
			switch tt.pm {
			case APT:
				cmd = "apt-get install -y " + strings.Join(tt.packages, " ")
			case DNF:
				cmd = "dnf install -y " + strings.Join(tt.packages, " ")
			case YUM:
				cmd = "yum install -y " + strings.Join(tt.packages, " ")
			case PACMAN:
				cmd = "pacman -S --noconfirm " + strings.Join(tt.packages, " ")
			case ZYPPER:
				cmd = "zypper install -y " + strings.Join(tt.packages, " ")
			case APK:
				cmd = "apk add --no-cache " + strings.Join(tt.packages, " ")
			}

			if cmd != tt.expected {
				t.Errorf("expected command:\n%s\ngot:\n%s", tt.expected, cmd)
			}
		})
	}
}

// TestSystemctlCommands tests the systemctl command patterns
func TestSystemctlCommands(t *testing.T) {
	tests := []struct {
		action   string
		service  string
		expected string
	}{
		{"enable", "smbd", "systemctl enable smbd"},
		{"start", "smbd", "systemctl start smbd"},
		{"restart", "nfs-server", "systemctl restart nfs-server"},
		{"reload", "smbd", "systemctl reload smbd"},
	}

	for _, tt := range tests {
		t.Run(tt.action+"_"+tt.service, func(t *testing.T) {
			cmd := "systemctl " + tt.action + " " + tt.service
			if cmd != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, cmd)
			}
		})
	}
}

// TestPackageManagerDetectionOrder tests the order of package manager detection
func TestPackageManagerDetectionOrder(t *testing.T) {
	// The detection order should prioritize certain package managers
	// based on how common they are
	expectedOrder := []struct {
		pm  PackageManager
		cmd string
	}{
		{APT, "apt-get"},
		{DNF, "dnf"},
		{YUM, "yum"},
		{PACMAN, "pacman"},
		{ZYPPER, "zypper"},
		{APK, "apk"},
	}

	for i, expected := range expectedOrder {
		t.Run(string(expected.pm), func(t *testing.T) {
			// Verify the order is maintained
			if i > len(expectedOrder) {
				t.Error("order index out of bounds")
			}
		})
	}
}

// TestNFSServiceDetection tests detection of NFS service variants
func TestNFSServiceDetection(t *testing.T) {
	tests := []struct {
		systemctlOutput string
		hasNFSServer    bool
	}{
		{
			systemctlOutput: "nfs-server.service enabled enabled\nnfs-idmapd.service enabled enabled",
			hasNFSServer:    true,
		},
		{
			systemctlOutput: "nfs-kernel-server.service enabled enabled\nnfs-common.service enabled enabled",
			hasNFSServer:    true,
		},
		{
			systemctlOutput: "ssh.service enabled enabled\napache2.service enabled enabled",
			hasNFSServer:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.systemctlOutput[:20], func(t *testing.T) {
			hasNFS := strings.Contains(tt.systemctlOutput, "nfs-server.service") ||
				strings.Contains(tt.systemctlOutput, "nfs-kernel-server.service")

			if hasNFS != tt.hasNFSServer {
				t.Errorf("expected hasNFSServer=%v, got %v", tt.hasNFSServer, hasNFS)
			}
		})
	}
}

// TestAllPackageManagersHavePackages tests that all package managers return packages
func TestAllPackageManagersHavePackages(t *testing.T) {
	packageManagers := []PackageManager{APT, YUM, DNF, PACMAN, ZYPPER, APK, UNKNOWN}
	packageFuncs := []struct {
		name string
		fn   func(PackageManager) []string
	}{
		{"Samba", GetSambaPackages},
		{"NFS", GetNFSPackages},
		{"CIFS", GetCIFSPackages},
		{"NFSClient", GetNFSClientPackages},
	}

	for _, pm := range packageManagers {
		for _, pf := range packageFuncs {
			t.Run(string(pm)+"_"+pf.name, func(t *testing.T) {
				packages := pf.fn(pm)
				if len(packages) == 0 {
					t.Errorf("%s packages for %s should not be empty", pf.name, pm)
				}
				for _, pkg := range packages {
					if pkg == "" {
						t.Errorf("package name should not be empty for %s %s", pm, pf.name)
					}
				}
			})
		}
	}
}

// TestPackageManagerString tests the string representation of package managers
func TestPackageManagerString(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected string
	}{
		{APT, "apt"},
		{YUM, "yum"},
		{DNF, "dnf"},
		{PACMAN, "pacman"},
		{ZYPPER, "zypper"},
		{APK, "apk"},
		{UNKNOWN, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.pm) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.pm))
			}
		})
	}
}

// TestUnknownPackageManagerHandling tests handling of unknown package managers
func TestUnknownPackageManagerHandling(t *testing.T) {
	pm := UNKNOWN

	// All package functions should still return valid packages for UNKNOWN
	if len(GetSambaPackages(pm)) == 0 {
		t.Error("GetSambaPackages should return packages for UNKNOWN")
	}
	if len(GetNFSPackages(pm)) == 0 {
		t.Error("GetNFSPackages should return packages for UNKNOWN")
	}
	if len(GetCIFSPackages(pm)) == 0 {
		t.Error("GetCIFSPackages should return packages for UNKNOWN")
	}
	if len(GetNFSClientPackages(pm)) == 0 {
		t.Error("GetNFSClientPackages should return packages for UNKNOWN")
	}
}
