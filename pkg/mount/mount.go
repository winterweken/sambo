package mount

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	fstabPath   = "/etc/fstab"
	fstabBackup = "/etc/fstab.backup"
)

// MountType represents the type of network mount
type MountType string

const (
	TypeCIFS MountType = "cifs"
	TypeNFS  MountType = "nfs"
)

// Mount represents a network mount configuration
type Mount struct {
	Source     string    // e.g., //server/share or server:/path
	MountPoint string    // e.g., /mnt/share
	Type       MountType // cifs or nfs
	Options    string    // mount options
	Persistent bool      // whether it's in fstab
}

// List returns all network mounts (CIFS and NFS)
func List() ([]Mount, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("failed to read mounts: %w", err)
	}
	defer file.Close()

	var mounts []Mount
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 3 {
			continue
		}

		mountType := fields[2]
		if mountType != "cifs" && mountType != "nfs" && mountType != "nfs4" {
			continue
		}

		// Normalize nfs4 to nfs
		if mountType == "nfs4" {
			mountType = "nfs"
		}

		mount := Mount{
			Source:     fields[0],
			MountPoint: fields[1],
			Type:       MountType(mountType),
			Persistent: isInFstab(fields[0], fields[1]),
		}

		if len(fields) >= 4 {
			mount.Options = fields[3]
		}

		mounts = append(mounts, mount)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading mounts: %w", err)
	}

	return mounts, nil
}

// Get retrieves a specific mount by mount point
func Get(mountPoint string) (*Mount, error) {
	mounts, err := List()
	if err != nil {
		return nil, err
	}

	for _, mount := range mounts {
		if mount.MountPoint == mountPoint {
			return &mount, nil
		}
	}

	return nil, fmt.Errorf("mount '%s' not found", mountPoint)
}

// MountCIFS mounts a CIFS/SMB share
func MountCIFS(source, mountPoint, options string, persistent bool) error {
	// Check if already mounted
	existing, _ := Get(mountPoint)
	if existing != nil {
		return fmt.Errorf("something is already mounted at '%s'", mountPoint)
	}

	// Ensure mount point exists
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Build mount command
	args := []string{"-t", "cifs"}
	if options != "" {
		args = append(args, "-o", options)
	}
	args = append(args, source, mountPoint)

	cmd := exec.Command("mount", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to mount: %s: %w", string(output), err)
	}

	// Add to fstab if persistent
	if persistent {
		if err := addToFstab(source, mountPoint, "cifs", options); err != nil {
			return fmt.Errorf("mounted successfully but failed to add to fstab: %w", err)
		}
	}

	return nil
}

// MountNFS mounts an NFS share
func MountNFS(source, mountPoint, options string, persistent bool) error {
	// Check if already mounted
	existing, _ := Get(mountPoint)
	if existing != nil {
		return fmt.Errorf("something is already mounted at '%s'", mountPoint)
	}

	// Ensure mount point exists
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Build mount command
	args := []string{"-t", "nfs"}
	if options != "" {
		args = append(args, "-o", options)
	}
	args = append(args, source, mountPoint)

	cmd := exec.Command("mount", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to mount: %s: %w", string(output), err)
	}

	// Add to fstab if persistent
	if persistent {
		if err := addToFstab(source, mountPoint, "nfs", options); err != nil {
			return fmt.Errorf("mounted successfully but failed to add to fstab: %w", err)
		}
	}

	return nil
}

// Unmount unmounts a share and optionally removes from fstab
func Unmount(mountPoint string, removePersistent bool) error {
	// Check if mounted
	mount, err := Get(mountPoint)
	if err != nil {
		return err
	}

	// Unmount
	cmd := exec.Command("umount", mountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount: %s: %w", string(output), err)
	}

	// Remove from fstab if requested
	if removePersistent && mount.Persistent {
		if err := removeFromFstab(mount.Source, mountPoint); err != nil {
			return fmt.Errorf("unmounted successfully but failed to remove from fstab: %w", err)
		}
	}

	return nil
}

// Helper functions

func isInFstab(source, mountPoint string) bool {
	file, err := os.Open(fstabPath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if fields[0] == source && fields[1] == mountPoint {
				return true
			}
		}
	}

	return false
}

func addToFstab(source, mountPoint, fsType, options string) error {
	// Backup fstab
	if err := backupFstab(); err != nil {
		return err
	}

	// Check if already in fstab
	if isInFstab(source, mountPoint) {
		return nil // Already exists
	}

	// Open fstab for appending
	f, err := os.OpenFile(fstabPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open fstab: %w", err)
	}
	defer f.Close()

	// Default options if not specified
	if options == "" {
		if fsType == "cifs" {
			options = "credentials=/root/.smbcredentials,uid=1000,gid=1000"
		} else {
			options = "defaults"
		}
	}

	// Format: source mountpoint fstype options dump pass
	fstabLine := fmt.Sprintf("%s %s %s %s 0 0\n", source, mountPoint, fsType, options)

	if _, err := f.WriteString(fstabLine); err != nil {
		return fmt.Errorf("failed to write to fstab: %w", err)
	}

	return nil
}

func removeFromFstab(source, mountPoint string) error {
	// Backup fstab
	if err := backupFstab(); err != nil {
		return err
	}

	// Read entire fstab
	content, err := os.ReadFile(fstabPath)
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}

	// Remove matching line
	lines := strings.Split(string(content), "\n")
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
			// Skip the line to remove
			if fields[0] == source && fields[1] == mountPoint {
				continue
			}
		}

		newLines = append(newLines, line)
	}

	// Write new fstab
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(fstabPath, []byte(newContent), 0644); err != nil {
		restoreFstab()
		return fmt.Errorf("failed to write fstab: %w", err)
	}

	return nil
}

func backupFstab() error {
	// Check if file exists
	if _, err := os.Stat(fstabPath); os.IsNotExist(err) {
		return nil
	}

	input, err := os.ReadFile(fstabPath)
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}

	if err := os.WriteFile(fstabBackup, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func restoreFstab() error {
	input, err := os.ReadFile(fstabBackup)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(fstabPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore fstab: %w", err)
	}

	return nil
}
