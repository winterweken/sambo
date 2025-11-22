package mount

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
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
	Active     bool      // whether it's currently mounted
}

// List returns all network mounts from both /proc/mounts (active) and /etc/fstab (configured)
func List() ([]Mount, error) {
	mountMap := make(map[string]*Mount) // key: source + mountpoint

	// First, read from /etc/fstab to get configured mounts
	fstabMounts, err := readFstabMounts()
	if err != nil {
		// Don't fail if fstab doesn't exist or can't be read, just log and continue
		fmt.Fprintf(os.Stderr, "Warning: failed to read fstab: %v\n", err)
	} else {
		for _, mount := range fstabMounts {
			key := mount.Source + "|" + mount.MountPoint
			mount.Persistent = true
			mount.Active = false // Will be set to true if found in /proc/mounts
			mountMap[key] = &mount
		}
	}

	// Then, read from /proc/mounts to get currently active mounts
	procMounts, err := readProcMounts()
	if err != nil {
		return nil, fmt.Errorf("failed to read proc mounts: %w", err)
	}

	for _, mount := range procMounts {
		key := mount.Source + "|" + mount.MountPoint
		existing, exists := mountMap[key]
		if exists {
			// Mount exists in fstab, mark it as active
			existing.Active = true
			// Update options from actual mount
			if mount.Options != "" {
				existing.Options = mount.Options
			}
		} else {
			// Mount is active but not in fstab
			mount.Persistent = false
			mount.Active = true
			mountMap[key] = &mount
		}
	}

	// Convert map to slice
	var mounts []Mount
	for _, mount := range mountMap {
		mounts = append(mounts, *mount)
	}

	return mounts, nil
}

// readProcMounts reads currently active mounts from /proc/mounts
func readProcMounts() ([]Mount, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/mounts: %w", err)
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

		// Check if this is a network mount (CIFS or NFS)
		isNFS := strings.HasPrefix(mountType, "nfs") // Catches nfs, nfs3, nfs4, etc.
		isCIFS := mountType == "cifs" || mountType == "smb" || mountType == "smbfs"

		if !isNFS && !isCIFS {
			continue
		}

		// Normalize mount types
		normalizedType := mountType
		if isNFS {
			normalizedType = "nfs"
		} else if isCIFS {
			normalizedType = "cifs"
		}

		mount := Mount{
			Source:     fields[0],
			MountPoint: fields[1],
			Type:       MountType(normalizedType),
		}

		if len(fields) >= 4 {
			mount.Options = fields[3]
		}

		mounts = append(mounts, mount)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading proc mounts: %w", err)
	}

	return mounts, nil
}

// readFstabMounts reads configured mounts from /etc/fstab
func readFstabMounts() ([]Mount, error) {
	file, err := os.Open(fstabPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mounts []Mount
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		source := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]
		options := fields[3]

		// Check if this is a network mount
		isNFS := strings.HasPrefix(fsType, "nfs")
		isCIFS := fsType == "cifs" || fsType == "smb" || fsType == "smbfs"

		if !isNFS && !isCIFS {
			continue
		}

		// Normalize mount types
		normalizedType := fsType
		if isNFS {
			normalizedType = "nfs"
		} else if isCIFS {
			normalizedType = "cifs"
		}

		mount := Mount{
			Source:     source,
			MountPoint: mountPoint,
			Type:       MountType(normalizedType),
			Options:    options,
		}

		mounts = append(mounts, mount)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
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
	mountPointCreated := false
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create mount point: %w", err)
		}
		mountPointCreated = true
	}

	// Build mount command
	args := []string{"-t", "cifs"}
	if options != "" {
		args = append(args, "-o", options)
	}
	args = append(args, source, mountPoint)

	cmd := exec.Command("mount", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Cleanup mount point if we created it and mount failed
		if mountPointCreated {
			os.Remove(mountPoint)
		}
		return fmt.Errorf("failed to mount: %s: %w", string(output), err)
	}

	// Verify mount succeeded
	if !isMountActive(mountPoint) {
		if mountPointCreated {
			os.Remove(mountPoint)
		}
		return fmt.Errorf("mount command succeeded but mount point is not active")
	}

	// Add to fstab if persistent
	if persistent {
		if err := addToFstab(source, mountPoint, "cifs", options); err != nil {
			// Mount is active but couldn't persist - warn but don't fail
			fmt.Fprintf(os.Stderr, "Warning: mounted successfully but failed to add to fstab: %v\n", err)
			fmt.Fprintf(os.Stderr, "Mount is active but will not persist across reboots.\n")
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
	mountPointCreated := false
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create mount point: %w", err)
		}
		mountPointCreated = true
	}

	// Build mount command
	args := []string{"-t", "nfs"}
	if options != "" {
		args = append(args, "-o", options)
	}
	args = append(args, source, mountPoint)

	cmd := exec.Command("mount", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Cleanup mount point if we created it and mount failed
		if mountPointCreated {
			os.Remove(mountPoint)
		}
		return fmt.Errorf("failed to mount: %s: %w", string(output), err)
	}

	// Verify mount succeeded
	if !isMountActive(mountPoint) {
		if mountPointCreated {
			os.Remove(mountPoint)
		}
		return fmt.Errorf("mount command succeeded but mount point is not active")
	}

	// Add to fstab if persistent
	if persistent {
		if err := addToFstab(source, mountPoint, "nfs", options); err != nil {
			// Mount is active but couldn't persist - warn but don't fail
			fmt.Fprintf(os.Stderr, "Warning: mounted successfully but failed to add to fstab: %v\n", err)
			fmt.Fprintf(os.Stderr, "Mount is active but will not persist across reboots.\n")
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

	// Only try to unmount if currently active
	if !mount.Active {
		// If not active but still in fstab, just remove from fstab
		if removePersistent && mount.Persistent {
			if err := removeFromFstab(mount.Source, mountPoint); err != nil {
				return fmt.Errorf("failed to remove from fstab: %w", err)
			}
		}
		return nil
	}

	// Unmount
	cmd := exec.Command("umount", mountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount: %s: %w", string(output), err)
	}

	// Verify unmount succeeded
	if isMountActive(mountPoint) {
		return fmt.Errorf("umount command succeeded but mount is still active")
	}

	// Remove from fstab if requested
	if removePersistent && mount.Persistent {
		if err := removeFromFstab(mount.Source, mountPoint); err != nil {
			// Unmount succeeded but couldn't remove from fstab - warn but don't fail
			fmt.Fprintf(os.Stderr, "Warning: unmounted successfully but failed to remove from fstab: %v\n", err)
			fmt.Fprintf(os.Stderr, "Mount will reappear after reboot unless manually removed from /etc/fstab.\n")
		}
	}

	// Try to remove mount point directory if empty
	if entries, err := os.ReadDir(mountPoint); err == nil && len(entries) == 0 {
		os.Remove(mountPoint)
	}

	return nil
}

// Helper functions

func getCurrentUser() (uid, gid string, err error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current user: %w", err)
	}
	return currentUser.Uid, currentUser.Gid, nil
}

func isMountActive(mountPoint string) bool {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == mountPoint {
			return true
		}
	}
	return false
}

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
			uid, gid, err := getCurrentUser()
			if err != nil {
				// Fallback to root if we can't determine current user
				uid, gid = "0", "0"
			}
			options = fmt.Sprintf("credentials=/root/.smbcredentials,uid=%s,gid=%s", uid, gid)
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
