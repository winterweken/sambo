package mount

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sambo/pkg/platform"
	"sambo/pkg/validate"
)

const (
	credentialsMode = 0600
)

// getFstabPath returns the path to fstab
func getFstabPath() string {
	return platform.FstabPath()
}

// getFstabBackupPath returns the path to fstab backup
func getFstabBackupPath() string {
	fstab := platform.FstabPath()
	if fstab == "" {
		return ""
	}
	return fstab + ".backup"
}

// getCredentialsDir returns the path to credentials directory
func getCredentialsDir() string {
	return platform.CredentialsDir()
}

// fstabEntry represents a parsed fstab line
type fstabEntry struct {
	source     string
	mountPoint string
}

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

// loadFstabEntries reads fstab once and returns a map for O(1) lookups
func loadFstabEntries() map[string]bool {
	entries := make(map[string]bool)

	fstabPath := getFstabPath()
	if fstabPath == "" {
		return entries // macOS doesn't use fstab for network mounts
	}

	file, err := os.Open(fstabPath)
	if err != nil {
		return entries
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := fields[0] + ":" + fields[1]
			entries[key] = true
		}
	}

	return entries
}

// List returns all network mounts (CIFS and NFS)
func List() ([]Mount, error) {
	if platform.IsMacOS() {
		return listMacOS()
	}
	return listLinux()
}

// listLinux reads mounts from /proc/mounts (Linux-specific)
func listLinux() ([]Mount, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Load fstab entries once for efficient lookups
	fstabEntries := loadFstabEntries()

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

		// Use map lookup instead of re-reading fstab for each mount
		fstabKey := fields[0] + ":" + fields[1]

		mount := Mount{
			Source:     fields[0],
			MountPoint: fields[1],
			Type:       MountType(mountType),
			Persistent: fstabEntries[fstabKey],
			Active:     true, // Reading from /proc/mounts, so it's active
		}

		if len(fields) >= 4 {
			mount.Options = fields[3]
		}

		mounts = append(mounts, mount)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

// loadAutoNFSEntries reads macOS auto_nfs and returns a map for O(1) lookups
// Returns map[mountPoint]source for persistent NFS mounts
func loadAutoNFSEntries() map[string]string {
	entries := make(map[string]string)

	autoNFSPath := platform.AutoNFSPath()
	if autoNFSPath == "" {
		return entries
	}

	file, err := os.Open(autoNFSPath)
	if err != nil {
		return entries
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// auto_nfs format: /mount/point -options server:/path
		// Parse: first field is mount point, last field is server:/path
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			mountPoint := fields[0]
			source := fields[len(fields)-1] // Last field is server:/path
			entries[mountPoint] = source
		}
	}

	return entries
}

// listMacOS reads mounts using the mount command (macOS/BSD)
func listMacOS() ([]Mount, error) {
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run mount command: %w", err)
	}

	// Load auto_nfs entries for persistence detection
	autoNFSEntries := loadAutoNFSEntries()

	var mounts []Mount
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		// macOS mount output format: /dev/disk1s1 on /path (type, options)
		// SMB format: //user@server/share on /path (smbfs, options)
		// NFS format: server:/path on /local/path (nfs, options)

		if !strings.Contains(line, " on ") {
			continue
		}

		// Check if it's a network mount
		// Match patterns like "(smbfs", "(smbfs,", "(cifs", "(nfs", "(nfs)", "(nfs,"
		// Also skip autofs entries (they're mount triggers, not actual mounts)
		if strings.Contains(line, "(autofs") {
			continue
		}
		isSMB := strings.Contains(line, "(smbfs") || strings.Contains(line, "(cifs")
		isNFS := strings.Contains(line, "(nfs")

		if !isSMB && !isNFS {
			continue
		}

		parts := strings.SplitN(line, " on ", 2)
		if len(parts) != 2 {
			continue
		}

		source := parts[0]

		// Extract mount point (everything before the parenthesis)
		rest := parts[1]
		parenIdx := strings.LastIndex(rest, " (")
		if parenIdx == -1 {
			continue
		}

		mountPoint := rest[:parenIdx]

		mountType := TypeNFS
		if isSMB {
			mountType = TypeCIFS
		}

		// Check if persistent in auto_nfs (for NFS mounts)
		persistent := false
		if isNFS {
			if _, ok := autoNFSEntries[mountPoint]; ok {
				persistent = true
			}
		}

		mount := Mount{
			Source:     source,
			MountPoint: mountPoint,
			Type:       mountType,
			Active:     true,
			Persistent: persistent,
		}

		mounts = append(mounts, mount)
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

// mountShare is the internal implementation for mounting any network share
func mountShare(source, mountPoint, fsType, options string, persistent bool) error {
	// Check if already mounted
	existing, _ := Get(mountPoint)
	if existing != nil {
		return fmt.Errorf("something is already mounted at '%s'", mountPoint)
	}

	// Ensure mount point exists
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	var cmd *exec.Cmd

	if platform.IsMacOS() {
		// macOS uses different mount commands
		if fsType == "cifs" {
			// macOS uses mount_smbfs
			// Convert //server/share to //user@server/share if needed
			args := []string{}
			if options != "" {
				args = append(args, "-o", options)
			}
			args = append(args, source, mountPoint)
			cmd = exec.Command("mount_smbfs", args...)
		} else {
			// macOS uses mount_nfs
			args := []string{}
			if options != "" {
				args = append(args, "-o", options)
			}
			args = append(args, source, mountPoint)
			cmd = exec.Command("mount_nfs", args...)
		}
	} else {
		// Linux mount command
		args := []string{"-t", fsType}
		if options != "" {
			args = append(args, "-o", options)
		}
		args = append(args, source, mountPoint)
		cmd = exec.Command("mount", args...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to mount: %s: %w", string(output), err)
	}

	// Add to fstab if persistent (Linux only)
	if persistent {
		if platform.IsMacOS() {
			// macOS doesn't use fstab for network mounts in the same way
			// We could add to auto_master or use other methods, but for now just warn
			return fmt.Errorf("persistent mounts are not fully supported on macOS. Mount succeeded but will not persist after reboot")
		}
		if err := addToFstab(source, mountPoint, fsType, options); err != nil {
			return fmt.Errorf("mounted successfully but failed to add to fstab: %w", err)
		}
	}

	return nil
}

// MountCIFS mounts a CIFS/SMB share
func MountCIFS(source, mountPoint, options string, persistent bool) error {
	// Validate source
	if err := validate.CIFSSource(source); err != nil {
		return err
	}
	// Validate mount point
	if err := validate.MountPoint(mountPoint); err != nil {
		return err
	}
	return mountShare(source, mountPoint, "cifs", options, persistent)
}

// MountCIFSWithCredentials mounts a CIFS share using secure credentials handling
// Credentials are stored in a secure file instead of being passed on command line
func MountCIFSWithCredentials(source, mountPoint, username, password string, persistent bool) error {
	// Validate source
	if err := validate.CIFSSource(source); err != nil {
		return err
	}
	// Validate mount point
	if err := validate.MountPoint(mountPoint); err != nil {
		return err
	}

	// Create credentials file
	credFile, err := writeCredentialsFile(source, username, password)
	if err != nil {
		return fmt.Errorf("failed to create credentials file: %w", err)
	}

	// Build options with credentials file reference
	options := fmt.Sprintf("credentials=%s,uid=1000,gid=1000", credFile)

	return mountShare(source, mountPoint, "cifs", options, persistent)
}

// writeCredentialsFile creates a secure credentials file for CIFS mounts
func writeCredentialsFile(source, username, password string) (string, error) {
	credDir := getCredentialsDir()

	// Create credentials directory if it doesn't exist
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Generate a filename based on the source (sanitized)
	// Replace characters that aren't safe in filenames
	safeName := strings.ReplaceAll(source, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	credFile := fmt.Sprintf("%s/creds_%s", credDir, safeName)

	// Write credentials to file
	content := fmt.Sprintf("username=%s\npassword=%s\n", username, password)
	if err := os.WriteFile(credFile, []byte(content), credentialsMode); err != nil {
		return "", fmt.Errorf("failed to write credentials file: %w", err)
	}

	return credFile, nil
}

// RemoveCredentialsFile removes the credentials file for a given source
func RemoveCredentialsFile(source string) error {
	credDir := getCredentialsDir()

	safeName := strings.ReplaceAll(source, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	credFile := fmt.Sprintf("%s/creds_%s", credDir, safeName)

	if err := os.Remove(credFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove credentials file: %w", err)
	}
	return nil
}

// MountNFS mounts an NFS share
func MountNFS(source, mountPoint, options string, persistent bool) error {
	// Validate source
	if err := validate.NFSSource(source); err != nil {
		return err
	}
	// Validate mount point
	if err := validate.MountPoint(mountPoint); err != nil {
		return err
	}
	return mountShare(source, mountPoint, "nfs", options, persistent)
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

func addToFstab(source, mountPoint, fsType, options string) error {
	fstabPath := getFstabPath()
	if fstabPath == "" {
		return fmt.Errorf("fstab not supported on this platform")
	}

	// Backup fstab
	if err := backupFstab(); err != nil {
		return err
	}

	// Check if already in fstab using efficient lookup
	fstabEntries := loadFstabEntries()
	fstabKey := source + ":" + mountPoint
	if fstabEntries[fstabKey] {
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

	// Add _netdev option for network mounts to wait for network before mounting
	if !strings.Contains(options, "_netdev") {
		options += ",_netdev"
	}

	// Format: source mountpoint fstype options dump pass
	fstabLine := fmt.Sprintf("%s %s %s %s 0 0\n", source, mountPoint, fsType, options)

	if _, err := f.WriteString(fstabLine); err != nil {
		return fmt.Errorf("failed to write to fstab: %w", err)
	}

	return nil
}

func removeFromFstab(source, mountPoint string) error {
	fstabPath := getFstabPath()
	if fstabPath == "" {
		return nil // fstab not supported on this platform
	}

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
	fstabPath := getFstabPath()
	backupPath := getFstabBackupPath()

	if fstabPath == "" || backupPath == "" {
		return nil // fstab not supported on this platform
	}

	// Check if file exists
	if _, err := os.Stat(fstabPath); os.IsNotExist(err) {
		return nil
	}

	input, err := os.ReadFile(fstabPath)
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func restoreFstab() error {
	fstabPath := getFstabPath()
	backupPath := getFstabBackupPath()

	if fstabPath == "" || backupPath == "" {
		return nil // fstab not supported on this platform
	}

	input, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(fstabPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore fstab: %w", err)
	}

	return nil
}

// SetPersistent toggles the persistence of a mount
func SetPersistent(mountPoint string, persistent bool) error {
	mount, err := Get(mountPoint)
	if err != nil {
		return err
	}

	if mount.Persistent == persistent {
		return nil // Already in desired state
	}

	if platform.IsMacOS() {
		if mount.Type == TypeNFS {
			if persistent {
				return addToAutoNFS(mount.Source, mount.MountPoint)
			}
			return removeFromAutoNFS(mount.MountPoint)
		}
		// CIFS/SMB persistence on macOS not yet supported
		return fmt.Errorf("persistent CIFS/SMB mounts on macOS are not yet supported")
	}

	// Linux: use fstab
	if persistent {
		fsType := string(mount.Type)
		return addToFstab(mount.Source, mount.MountPoint, fsType, mount.Options)
	}
	return removeFromFstab(mount.Source, mount.MountPoint)
}

// macOS auto_nfs management

func getAutoNFSPath() string {
	return platform.AutoNFSPath()
}

func getAutoNFSBackupPath() string {
	autoNFS := platform.AutoNFSPath()
	if autoNFS == "" {
		return ""
	}
	return autoNFS + ".backup"
}

func backupAutoNFS() error {
	autoNFSPath := getAutoNFSPath()
	backupPath := getAutoNFSBackupPath()

	if autoNFSPath == "" || backupPath == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(autoNFSPath); os.IsNotExist(err) {
		return nil
	}

	input, err := os.ReadFile(autoNFSPath)
	if err != nil {
		return fmt.Errorf("failed to read auto_nfs: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func restoreAutoNFS() error {
	autoNFSPath := getAutoNFSPath()
	backupPath := getAutoNFSBackupPath()

	if autoNFSPath == "" || backupPath == "" {
		return nil
	}

	input, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(autoNFSPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore auto_nfs: %w", err)
	}

	return nil
}

func addToAutoNFS(source, mountPoint string) error {
	autoNFSPath := getAutoNFSPath()
	if autoNFSPath == "" {
		return fmt.Errorf("auto_nfs not supported on this platform")
	}

	// Backup auto_nfs
	if err := backupAutoNFS(); err != nil {
		return err
	}

	// Check if already in auto_nfs
	entries := loadAutoNFSEntries()
	if _, ok := entries[mountPoint]; ok {
		return nil // Already exists
	}

	// Open auto_nfs for appending
	f, err := os.OpenFile(autoNFSPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open auto_nfs: %w", err)
	}
	defer f.Close()

	// Format: /mount/point -fstype=nfs,vers=3,rw,resvport server:/path
	autoNFSLine := fmt.Sprintf("%s\t-fstype=nfs,vers=3,rw,resvport\t%s\n", mountPoint, source)

	if _, err := f.WriteString(autoNFSLine); err != nil {
		return fmt.Errorf("failed to write to auto_nfs: %w", err)
	}

	// Reload automount
	if err := reloadAutomount(); err != nil {
		restoreAutoNFS()
		return fmt.Errorf("failed to reload automount: %w", err)
	}

	return nil
}

func removeFromAutoNFS(mountPoint string) error {
	autoNFSPath := getAutoNFSPath()
	if autoNFSPath == "" {
		return nil
	}

	// Backup auto_nfs
	if err := backupAutoNFS(); err != nil {
		return err
	}

	// Read entire auto_nfs
	content, err := os.ReadFile(autoNFSPath)
	if err != nil {
		return fmt.Errorf("failed to read auto_nfs: %w", err)
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
		if len(fields) >= 1 {
			// Skip the line to remove (first field is mount point)
			if fields[0] == mountPoint {
				continue
			}
		}

		newLines = append(newLines, line)
	}

	// Write new auto_nfs
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(autoNFSPath, []byte(newContent), 0644); err != nil {
		restoreAutoNFS()
		return fmt.Errorf("failed to write auto_nfs: %w", err)
	}

	// Reload automount
	if err := reloadAutomount(); err != nil {
		restoreAutoNFS()
		return fmt.Errorf("failed to reload automount: %w", err)
	}

	return nil
}

func reloadAutomount() error {
	cmd := exec.Command("automount", "-vc")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("automount -vc failed: %s: %w", string(output), err)
	}
	return nil
}
