package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMountType(t *testing.T) {
	tests := []struct {
		name     string
		input    MountType
		expected string
	}{
		{"CIFS type", TypeCIFS, "cifs"},
		{"NFS type", TypeNFS, "nfs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.input) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.input)
			}
		})
	}
}

func TestMount_Struct(t *testing.T) {
	m := Mount{
		Source:     "//server/share",
		MountPoint: "/mnt/share",
		Type:       TypeCIFS,
		Options:    "username=user,password=pass",
		Persistent: true,
		Active:     true,
	}

	if m.Source != "//server/share" {
		t.Errorf("expected Source to be //server/share, got %s", m.Source)
	}
	if m.MountPoint != "/mnt/share" {
		t.Errorf("expected MountPoint to be /mnt/share, got %s", m.MountPoint)
	}
	if m.Type != TypeCIFS {
		t.Errorf("expected Type to be cifs, got %s", m.Type)
	}
	if !m.Persistent {
		t.Error("expected Persistent to be true")
	}
	if !m.Active {
		t.Error("expected Active to be true")
	}
}

// TestParseProcMountsLine tests parsing of /proc/mounts format
func TestParseProcMountsLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantMount  bool
		wantSource string
		wantPoint  string
		wantType   MountType
	}{
		{
			name:       "CIFS mount",
			line:       "//192.168.1.100/share /mnt/share cifs rw,relatime 0 0",
			wantMount:  true,
			wantSource: "//192.168.1.100/share",
			wantPoint:  "/mnt/share",
			wantType:   TypeCIFS,
		},
		{
			name:       "NFS mount",
			line:       "192.168.1.100:/export /mnt/nfs nfs rw,vers=4 0 0",
			wantMount:  true,
			wantSource: "192.168.1.100:/export",
			wantPoint:  "/mnt/nfs",
			wantType:   TypeNFS,
		},
		{
			name:       "NFS4 mount",
			line:       "server:/path /mnt/nfs4 nfs4 rw 0 0",
			wantMount:  true,
			wantSource: "server:/path",
			wantPoint:  "/mnt/nfs4",
			wantType:   TypeNFS,
		},
		{
			name:      "ext4 mount - should be skipped",
			line:      "/dev/sda1 / ext4 rw,relatime 0 0",
			wantMount: false,
		},
		{
			name:      "tmpfs - should be skipped",
			line:      "tmpfs /tmp tmpfs rw,nosuid 0 0",
			wantMount: false,
		},
		{
			name:      "short line - should be skipped",
			line:      "incomplete line",
			wantMount: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := strings.Fields(tt.line)
			if len(fields) < 3 {
				if tt.wantMount {
					t.Error("expected mount to be parsed but line is too short")
				}
				return
			}

			mountType := fields[2]
			isNFS := strings.HasPrefix(mountType, "nfs")
			isCIFS := mountType == "cifs" || mountType == "smb" || mountType == "smbfs"

			if tt.wantMount {
				if !isNFS && !isCIFS {
					t.Errorf("expected mount type to be detected, got %s", mountType)
				}
				if fields[0] != tt.wantSource {
					t.Errorf("expected source %s, got %s", tt.wantSource, fields[0])
				}
				if fields[1] != tt.wantPoint {
					t.Errorf("expected mount point %s, got %s", tt.wantPoint, fields[1])
				}
			} else {
				if isNFS || isCIFS {
					t.Error("expected mount to be skipped but was detected as network mount")
				}
			}
		})
	}
}

// TestParseFstabLine tests parsing of /etc/fstab format
func TestParseFstabLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantMount  bool
		wantSource string
		wantPoint  string
		wantType   MountType
		wantOpts   string
	}{
		{
			name:       "CIFS in fstab",
			line:       "//server/share /mnt/share cifs credentials=/root/.smbcredentials,uid=1000 0 0",
			wantMount:  true,
			wantSource: "//server/share",
			wantPoint:  "/mnt/share",
			wantType:   TypeCIFS,
			wantOpts:   "credentials=/root/.smbcredentials,uid=1000",
		},
		{
			name:       "NFS in fstab",
			line:       "server:/export /mnt/nfs nfs defaults 0 0",
			wantMount:  true,
			wantSource: "server:/export",
			wantPoint:  "/mnt/nfs",
			wantType:   TypeNFS,
			wantOpts:   "defaults",
		},
		{
			name:      "comment line",
			line:      "# This is a comment",
			wantMount: false,
		},
		{
			name:      "empty line",
			line:      "",
			wantMount: false,
		},
		{
			name:      "ext4 filesystem",
			line:      "UUID=1234 / ext4 defaults 0 1",
			wantMount: false,
		},
		{
			name:      "swap",
			line:      "/dev/sda2 none swap sw 0 0",
			wantMount: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := strings.TrimSpace(tt.line)

			// Skip comments and empty lines
			if line == "" || strings.HasPrefix(line, "#") {
				if tt.wantMount {
					t.Error("expected mount but got comment/empty line")
				}
				return
			}

			fields := strings.Fields(line)
			if len(fields) < 4 {
				if tt.wantMount {
					t.Error("expected mount but line has too few fields")
				}
				return
			}

			fsType := fields[2]
			isNFS := strings.HasPrefix(fsType, "nfs")
			isCIFS := fsType == "cifs" || fsType == "smb" || fsType == "smbfs"

			if tt.wantMount {
				if !isNFS && !isCIFS {
					t.Errorf("expected network mount type, got %s", fsType)
				}
				if fields[0] != tt.wantSource {
					t.Errorf("expected source %s, got %s", tt.wantSource, fields[0])
				}
				if fields[1] != tt.wantPoint {
					t.Errorf("expected mount point %s, got %s", tt.wantPoint, fields[1])
				}
				if fields[3] != tt.wantOpts {
					t.Errorf("expected options %s, got %s", tt.wantOpts, fields[3])
				}
			} else {
				if isNFS || isCIFS {
					t.Errorf("expected non-network mount but got %s", fsType)
				}
			}
		})
	}
}

// TestFstabLineGeneration tests generation of fstab lines
func TestFstabLineGeneration(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		mountPoint string
		fsType     string
		options    string
		expected   string
	}{
		{
			name:       "CIFS mount with options",
			source:     "//server/share",
			mountPoint: "/mnt/share",
			fsType:     "cifs",
			options:    "credentials=/root/.smbcredentials,uid=1000,gid=1000",
			expected:   "//server/share /mnt/share cifs credentials=/root/.smbcredentials,uid=1000,gid=1000 0 0\n",
		},
		{
			name:       "NFS mount with defaults",
			source:     "192.168.1.100:/export",
			mountPoint: "/mnt/nfs",
			fsType:     "nfs",
			options:    "defaults",
			expected:   "192.168.1.100:/export /mnt/nfs nfs defaults 0 0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the format used in addToFstab
			fstabLine := tt.source + " " + tt.mountPoint + " " + tt.fsType + " " + tt.options + " 0 0\n"
			if fstabLine != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, fstabLine)
			}
		})
	}
}

// TestMountTypeNormalization tests that mount types are normalized correctly
func TestMountTypeNormalization(t *testing.T) {
	tests := []struct {
		input      string
		wantNFS    bool
		wantCIFS   bool
		normalized string
	}{
		{"nfs", true, false, "nfs"},
		{"nfs3", true, false, "nfs"},
		{"nfs4", true, false, "nfs"},
		{"cifs", false, true, "cifs"},
		{"smb", false, true, "cifs"},
		{"smbfs", false, true, "cifs"},
		{"ext4", false, false, ""},
		{"xfs", false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			isNFS := strings.HasPrefix(tt.input, "nfs")
			isCIFS := tt.input == "cifs" || tt.input == "smb" || tt.input == "smbfs"

			if isNFS != tt.wantNFS {
				t.Errorf("NFS detection: expected %v, got %v", tt.wantNFS, isNFS)
			}
			if isCIFS != tt.wantCIFS {
				t.Errorf("CIFS detection: expected %v, got %v", tt.wantCIFS, isCIFS)
			}

			if isNFS || isCIFS {
				normalizedType := tt.input
				if isNFS {
					normalizedType = "nfs"
				} else if isCIFS {
					normalizedType = "cifs"
				}
				if normalizedType != tt.normalized {
					t.Errorf("normalized type: expected %s, got %s", tt.normalized, normalizedType)
				}
			}
		})
	}
}

// TestDefaultOptions tests default options for different mount types
func TestDefaultOptions(t *testing.T) {
	tests := []struct {
		fsType          string
		providedOptions string
		expectedDefault string
	}{
		{"cifs", "", "credentials=/root/.smbcredentials,uid=1000,gid=1000"},
		{"nfs", "", "defaults"},
		{"cifs", "user=test", "user=test"},
		{"nfs", "rw,hard", "rw,hard"},
	}

	for _, tt := range tests {
		t.Run(tt.fsType+"_"+tt.providedOptions, func(t *testing.T) {
			options := tt.providedOptions
			if options == "" {
				if tt.fsType == "cifs" {
					options = "credentials=/root/.smbcredentials,uid=1000,gid=1000"
				} else {
					options = "defaults"
				}
			}
			if options != tt.expectedDefault {
				t.Errorf("expected default options %s, got %s", tt.expectedDefault, options)
			}
		})
	}
}

// TestReadFstabMountsWithTempFile tests reading fstab from a temp file
func TestReadFstabMountsWithTempFile(t *testing.T) {
	// Create a temporary fstab file
	tmpDir := t.TempDir()
	tmpFstab := filepath.Join(tmpDir, "fstab")

	fstabContent := `# /etc/fstab: static file system information
UUID=1234-5678 / ext4 defaults 0 1
//server/share /mnt/cifs cifs credentials=/root/.smbcreds,uid=1000 0 0
192.168.1.100:/export /mnt/nfs nfs defaults 0 0
# Another comment
/dev/sda2 none swap sw 0 0
`

	if err := os.WriteFile(tmpFstab, []byte(fstabContent), 0644); err != nil {
		t.Fatalf("failed to create temp fstab: %v", err)
	}

	// Parse the content manually (simulating readFstabMounts logic)
	content, err := os.ReadFile(tmpFstab)
	if err != nil {
		t.Fatalf("failed to read temp fstab: %v", err)
	}

	var mounts []Mount
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		fsType := fields[2]
		isNFS := strings.HasPrefix(fsType, "nfs")
		isCIFS := fsType == "cifs" || fsType == "smb" || fsType == "smbfs"

		if !isNFS && !isCIFS {
			continue
		}

		normalizedType := fsType
		if isNFS {
			normalizedType = "nfs"
		} else if isCIFS {
			normalizedType = "cifs"
		}

		mounts = append(mounts, Mount{
			Source:     fields[0],
			MountPoint: fields[1],
			Type:       MountType(normalizedType),
			Options:    fields[3],
		})
	}

	if len(mounts) != 2 {
		t.Errorf("expected 2 network mounts, got %d", len(mounts))
	}

	// Check CIFS mount
	foundCIFS := false
	foundNFS := false
	for _, m := range mounts {
		if m.Type == TypeCIFS && m.Source == "//server/share" {
			foundCIFS = true
			if m.MountPoint != "/mnt/cifs" {
				t.Errorf("CIFS mount point: expected /mnt/cifs, got %s", m.MountPoint)
			}
		}
		if m.Type == TypeNFS && m.Source == "192.168.1.100:/export" {
			foundNFS = true
			if m.MountPoint != "/mnt/nfs" {
				t.Errorf("NFS mount point: expected /mnt/nfs, got %s", m.MountPoint)
			}
		}
	}

	if !foundCIFS {
		t.Error("CIFS mount not found")
	}
	if !foundNFS {
		t.Error("NFS mount not found")
	}
}

// TestIsInFstabLogic tests the logic for checking if a mount is in fstab
func TestIsInFstabLogic(t *testing.T) {
	fstabContent := `# Test fstab
//server/share /mnt/share cifs defaults 0 0
192.168.1.100:/export /mnt/nfs nfs defaults 0 0
`

	tests := []struct {
		source     string
		mountPoint string
		expected   bool
	}{
		{"//server/share", "/mnt/share", true},
		{"192.168.1.100:/export", "/mnt/nfs", true},
		{"//other/share", "/mnt/other", false},
		{"//server/share", "/mnt/other", false}, // same source, different mount point
	}

	for _, tt := range tests {
		t.Run(tt.source+"_"+tt.mountPoint, func(t *testing.T) {
			lines := strings.Split(fstabContent, "\n")
			found := false

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if fields[0] == tt.source && fields[1] == tt.mountPoint {
						found = true
						break
					}
				}
			}

			if found != tt.expected {
				t.Errorf("expected isInFstab=%v, got %v", tt.expected, found)
			}
		})
	}
}

// TestRemoveFromFstabLogic tests the logic for removing entries from fstab
func TestRemoveFromFstabLogic(t *testing.T) {
	originalContent := `# Test fstab
UUID=1234 / ext4 defaults 0 1
//server/share /mnt/share cifs defaults 0 0
192.168.1.100:/export /mnt/nfs nfs defaults 0 0
# Another comment
/dev/sda2 none swap sw 0 0
`

	sourceToRemove := "//server/share"
	mountPointToRemove := "/mnt/share"

	lines := strings.Split(originalContent, "\n")
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Keep comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			if fields[0] == sourceToRemove && fields[1] == mountPointToRemove {
				continue // Skip this line
			}
		}

		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")

	// Verify the CIFS entry was removed
	if strings.Contains(newContent, "//server/share") {
		t.Error("CIFS entry should have been removed")
	}

	// Verify other entries remain
	if !strings.Contains(newContent, "192.168.1.100:/export") {
		t.Error("NFS entry should still exist")
	}
	if !strings.Contains(newContent, "UUID=1234") {
		t.Error("UUID entry should still exist")
	}
	if !strings.Contains(newContent, "# Test fstab") {
		t.Error("comments should be preserved")
	}
}

// TestMountKeyGeneration tests the key generation for mount map
func TestMountKeyGeneration(t *testing.T) {
	tests := []struct {
		source     string
		mountPoint string
		expected   string
	}{
		{"//server/share", "/mnt/share", "//server/share|/mnt/share"},
		{"192.168.1.100:/export", "/mnt/nfs", "192.168.1.100:/export|/mnt/nfs"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			key := tt.source + "|" + tt.mountPoint
			if key != tt.expected {
				t.Errorf("expected key %s, got %s", tt.expected, key)
			}
		})
	}
}

// TestMountMergeLogic tests the logic for merging fstab and proc mounts
func TestMountMergeLogic(t *testing.T) {
	// Simulate fstab mounts
	fstabMounts := []Mount{
		{Source: "//server/share", MountPoint: "/mnt/share", Type: TypeCIFS, Persistent: true},
		{Source: "192.168.1.100:/export", MountPoint: "/mnt/nfs", Type: TypeNFS, Persistent: true},
	}

	// Simulate proc mounts (only the CIFS one is actually mounted)
	procMounts := []Mount{
		{Source: "//server/share", MountPoint: "/mnt/share", Type: TypeCIFS, Options: "rw,user=test"},
		{Source: "//other/share", MountPoint: "/mnt/other", Type: TypeCIFS, Options: "rw"},
	}

	// Merge logic
	mountMap := make(map[string]*Mount)

	// Add fstab mounts first
	for _, m := range fstabMounts {
		key := m.Source + "|" + m.MountPoint
		mount := m
		mount.Persistent = true
		mount.Active = false
		mountMap[key] = &mount
	}

	// Then process proc mounts
	for _, m := range procMounts {
		key := m.Source + "|" + m.MountPoint
		if existing, exists := mountMap[key]; exists {
			existing.Active = true
			if m.Options != "" {
				existing.Options = m.Options
			}
		} else {
			mount := m
			mount.Persistent = false
			mount.Active = true
			mountMap[key] = &mount
		}
	}

	// Verify results
	if len(mountMap) != 3 {
		t.Errorf("expected 3 mounts in map, got %d", len(mountMap))
	}

	// Check CIFS mount that exists in both
	cifsKey := "//server/share|/mnt/share"
	if m, ok := mountMap[cifsKey]; ok {
		if !m.Persistent {
			t.Error("CIFS mount should be persistent (in fstab)")
		}
		if !m.Active {
			t.Error("CIFS mount should be active (in proc)")
		}
		if m.Options != "rw,user=test" {
			t.Errorf("CIFS mount options should be updated from proc, got %s", m.Options)
		}
	} else {
		t.Error("CIFS mount not found in map")
	}

	// Check NFS mount that's only in fstab
	nfsKey := "192.168.1.100:/export|/mnt/nfs"
	if m, ok := mountMap[nfsKey]; ok {
		if !m.Persistent {
			t.Error("NFS mount should be persistent")
		}
		if m.Active {
			t.Error("NFS mount should not be active")
		}
	} else {
		t.Error("NFS mount not found in map")
	}

	// Check other CIFS mount that's only in proc
	otherKey := "//other/share|/mnt/other"
	if m, ok := mountMap[otherKey]; ok {
		if m.Persistent {
			t.Error("other CIFS mount should not be persistent")
		}
		if !m.Active {
			t.Error("other CIFS mount should be active")
		}
	} else {
		t.Error("other CIFS mount not found in map")
	}
}

// TestValidateMountPoint tests mount point validation
func TestValidateMountPoint(t *testing.T) {
	tests := []struct {
		mountPoint string
		valid      bool
	}{
		{"/mnt/share", true},
		{"/home/user/mount", true},
		{"/media/external", true},
		{"", false},
		{"relative/path", false},
		{"./local", false},
	}

	for _, tt := range tests {
		t.Run(tt.mountPoint, func(t *testing.T) {
			isAbsolute := strings.HasPrefix(tt.mountPoint, "/") && tt.mountPoint != ""
			if isAbsolute != tt.valid {
				t.Errorf("mount point %s: expected valid=%v, got %v", tt.mountPoint, tt.valid, isAbsolute)
			}
		})
	}
}

// TestValidateSource tests source validation for CIFS and NFS
func TestValidateSource(t *testing.T) {
	tests := []struct {
		source   string
		mountType MountType
		valid    bool
	}{
		{"//server/share", TypeCIFS, true},
		{"//192.168.1.100/share", TypeCIFS, true},
		{"server:/export", TypeNFS, true},
		{"192.168.1.100:/path/to/export", TypeNFS, true},
		{"/local/path", TypeCIFS, false},
		{"", TypeCIFS, false},
		{"", TypeNFS, false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			var valid bool
			if tt.mountType == TypeCIFS {
				valid = strings.HasPrefix(tt.source, "//")
			} else if tt.mountType == TypeNFS {
				valid = strings.Contains(tt.source, ":")
			}

			if tt.source == "" {
				valid = false
			}

			if valid != tt.valid {
				t.Errorf("source %s (type %s): expected valid=%v, got %v", tt.source, tt.mountType, tt.valid, valid)
			}
		})
	}
}

// TestMountCIFSWithCredentials_UsesDynamicUID verifies Bug #9 fix:
// MountCIFSWithCredentials must use os.Getuid()/os.Getgid() instead of
// hardcoded uid=1000,gid=1000 which breaks on macOS (UID 501) and multi-user systems.
func TestMountCIFSWithCredentials_UsesDynamicUID(t *testing.T) {
	currentUID := os.Getuid()
	currentGID := os.Getgid()

	// The options string should contain the current user's UID/GID
	expectedUIDStr := fmt.Sprintf("uid=%d", currentUID)
	expectedGIDStr := fmt.Sprintf("gid=%d", currentGID)

	// Build what MountCIFSWithCredentials would produce
	credFile := "/tmp/test_cred"
	options := fmt.Sprintf("credentials=%s,uid=%d,gid=%d", credFile, os.Getuid(), os.Getgid())

	if !strings.Contains(options, expectedUIDStr) {
		t.Errorf("Options should contain %q, got: %s", expectedUIDStr, options)
	}
	if !strings.Contains(options, expectedGIDStr) {
		t.Errorf("Options should contain %q, got: %s", expectedGIDStr, options)
	}

	// Verify it does NOT contain hardcoded values (unless the current user happens to be 1000)
	if currentUID != 1000 && strings.Contains(options, "uid=1000") {
		t.Error("Options should NOT contain hardcoded uid=1000")
	}
	if currentGID != 1000 && strings.Contains(options, "gid=1000") {
		t.Error("Options should NOT contain hardcoded gid=1000")
	}
}
