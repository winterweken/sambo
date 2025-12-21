package samba

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"sambo/pkg/avahi"
	"sambo/pkg/service"
	"sambo/pkg/system"
	"sambo/pkg/user"
	"sambo/pkg/validate"
)

// AvahiManager defines the interface for Avahi operations
type AvahiManager interface {
	AddTimeMachineShare(shareName string, existingShares []string) error
	RemoveTimeMachineShare(shareName string, remainingShares []string) error
}

// RealAvahiManager implements AvahiManager using the avahi package
type RealAvahiManager struct{}

func (RealAvahiManager) AddTimeMachineShare(name string, existing []string) error {
	return avahi.AddTimeMachineShare(name, existing)
}

func (RealAvahiManager) RemoveTimeMachineShare(name string, remaining []string) error {
	return avahi.RemoveTimeMachineShare(name, remaining)
}

// Manager handles Samba configuration and operations
type Manager struct {
	fs       system.FileSystem
	exec     system.CommandExecutor
	platform system.Platform
	avahi    AvahiManager
}

// NewManager creates a new Samba manager
func NewManager(fs system.FileSystem, exec system.CommandExecutor, platform system.Platform, avahi AvahiManager) *Manager {
	return &Manager{
		fs:       fs,
		exec:     exec,
		platform: platform,
		avahi:    avahi,
	}
}

// getSambaConfPath returns the path to smb.conf
func (m *Manager) getSambaConfPath() string {
	return m.platform.SambaConfigPath()
}

// getSambaBackupPath returns the path to the smb.conf backup
func (m *Manager) getSambaBackupPath() string {
	return m.platform.SambaConfigPath() + ".backup"
}

// getSambaConfigDir returns the directory containing smb.conf
func (m *Manager) getSambaConfigDir() string {
	return m.platform.SambaConfigDir()
}

// CheckInstalled verifies that Samba is installed and configured
func (m *Manager) CheckInstalled() error {
	return m.CheckInstalledInteractive(true)
}

// CheckInstalledInteractive verifies that Samba is installed and optionally offers to install it
func (m *Manager) CheckInstalledInteractive(interactive bool) error {
	// Check if smbd binary exists
	if _, err := m.exec.LookPath("smbd"); err != nil {
		if !interactive {
			if m.platform.IsMacOS() {
				return fmt.Errorf("Samba is not installed.\n\nPlease install Samba:\n  macOS: brew install samba")
			}
			return fmt.Errorf("Samba is not installed.\n\nPlease install Samba:\n  Debian/Ubuntu: sudo apt-get install samba\n  RHEL/CentOS:   sudo yum install samba\n  Arch:          sudo pacman -S samba")
		}

		// Offer to install
		fmt.Println("Samba is not installed.")
		fmt.Print("Would you like to install it now? (y/N): ")

		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			return fmt.Errorf("Samba installation cancelled")
		}

		// Detect package manager and install
		if err := m.installSamba(); err != nil {
			return fmt.Errorf("failed to install Samba: %w", err)
		}

		fmt.Println("✓ Samba installed successfully")
	}

	confPath := m.getSambaConfPath()
	configDir := m.getSambaConfigDir()

	// Check if config file exists, create if missing
	if _, err := m.fs.Stat(confPath); os.IsNotExist(err) {
		// Try to create directory
		if err := m.fs.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("Samba is installed but cannot create config directory: %w", err)
		}

		// Create basic smb.conf
		basicConfig := `[global]
   workgroup = WORKGROUP
   server string = Samba Server
   security = user
   map to guest = Bad User
   dns proxy = no

`
		if err := m.fs.WriteFile(confPath, []byte(basicConfig), 0644); err != nil {
			return fmt.Errorf("Samba is installed but cannot create config file: %w", err)
		}
		fmt.Println("✓ Created basic Samba configuration")
	}

	return nil
}

func (m *Manager) installSamba() error {
	// Detect package manager
	if m.platform.IsMacOS() {
		if _, err := m.exec.LookPath("brew"); err == nil {
			fmt.Println("Installing Samba using Homebrew...")
			return m.exec.Run("brew", "install", "samba")
		} else {
			return fmt.Errorf("Homebrew is not installed.\n\nPlease install Homebrew first: https://brew.sh\nThen run: brew install samba")
		}
	} else if _, err := m.exec.LookPath("apt-get"); err == nil {
		fmt.Println("Installing Samba using apt-get...")
		return m.exec.Run("apt-get", "install", "-y", "samba")
	} else if _, err := m.exec.LookPath("yum"); err == nil {
		fmt.Println("Installing Samba using yum...")
		return m.exec.Run("yum", "install", "-y", "samba")
	} else if _, err := m.exec.LookPath("dnf"); err == nil {
		fmt.Println("Installing Samba using dnf...")
		return m.exec.Run("dnf", "install", "-y", "samba")
	} else if _, err := m.exec.LookPath("pacman"); err == nil {
		fmt.Println("Installing Samba using pacman...")
		return m.exec.Run("pacman", "-S", "--noconfirm", "samba")
	} else if _, err := m.exec.LookPath("zypper"); err == nil {
		fmt.Println("Installing Samba using zypper...")
		return m.exec.Run("zypper", "install", "-y", "samba")
	} else {
		if m.platform.IsMacOS() {
			return fmt.Errorf("could not detect package manager.\n\nPlease install Homebrew first: https://brew.sh\nThen run: brew install samba")
		}
		return fmt.Errorf("could not detect package manager.\n\nPlease install Samba manually:\n  Debian/Ubuntu: sudo apt-get install samba\n  RHEL/CentOS:   sudo yum install samba\n  Arch:          sudo pacman -S samba")
	}
}

// ShareType constants for different share presets
const (
	ShareTypeGeneral      = "general"
	ShareTypeTimeMachine  = "timemachine"
	ShareTypeUnifiProtect = "unifi-protect"
	ShareTypeMedia        = "media"
)

// ShareTypeNames provides human-readable names for share types
var ShareTypeNames = map[string]string{
	ShareTypeGeneral:      "General Purpose",
	ShareTypeTimeMachine:  "Time Machine",
	ShareTypeUnifiProtect: "Ubiquiti Protect",
	ShareTypeMedia:        "Media Server",
}

// ShareTypeList returns ordered list of share types for UI
func ShareTypeList() []string {
	return []string{ShareTypeGeneral, ShareTypeTimeMachine, ShareTypeUnifiProtect, ShareTypeMedia}
}

// Share represents a Samba share configuration
type Share struct {
	Name               string
	Path               string
	Comment            string
	ReadOnly           bool
	Browseable         bool
	ValidUsers         []string
	ShareType          string // "general", "timemachine", "unifi-protect", "media"
	TimeMachine        bool   // Deprecated: use ShareType == ShareTypeTimeMachine
	TimeMachineMaxSize string // e.g., "500G", "1T", "0" for unlimited
}

// List returns all configured Samba shares
func (m *Manager) List() ([]Share, error) {
	confPath := m.getSambaConfPath()
	content, err := m.fs.ReadFile(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Share{}, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", confPath, err)
	}

	var shares []Share
	var currentShare *Share
	scanner := bufio.NewScanner(bytes.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for share section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Save previous share if exists
			if currentShare != nil {
				shares = append(shares, *currentShare)
			}

			shareName := strings.Trim(line, "[]")
			// Skip special shares
			if shareName == "global" || shareName == "homes" || shareName == "printers" {
				currentShare = nil
				continue
			}

			currentShare = &Share{
				Name:       shareName,
				Browseable: true, // default
			}
			continue
		}

		// Parse share parameters
		if currentShare != nil && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "path":
				currentShare.Path = value
			case "comment":
				currentShare.Comment = value
			case "read only":
				currentShare.ReadOnly = strings.ToLower(value) == "yes"
			case "browseable", "browsable":
				currentShare.Browseable = strings.ToLower(value) == "yes"
			case "valid users":
				if value != "" {
					currentShare.ValidUsers = strings.Fields(value)
				}
			case "fruit:time machine":
				currentShare.TimeMachine = strings.ToLower(value) == "yes"
			case "fruit:time machine max size":
				currentShare.TimeMachineMaxSize = value
			}
		}
	}

	// Add last share
	if currentShare != nil {
		shares = append(shares, *currentShare)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	return shares, nil
}

// Get retrieves a specific share by name
func (m *Manager) Get(name string) (*Share, error) {
	shares, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, share := range shares {
		if share.Name == name {
			return &share, nil
		}
	}

	return nil, fmt.Errorf("share '%s' not found", name)
}

// Create adds a new Samba share
func (m *Manager) Create(share Share) error {
	// Check if Samba service is available
	// TODO: Inject service checker too, but for now wrap in try/catch or assume it's there?
	// The original code used a helper that just printed a warning. We can leave it for now.
	service.WarnIfNotRunning("samba")

	// Validate share name
	if err := validate.ShareName(share.Name); err != nil {
		return fmt.Errorf("invalid share name: %w", err)
	}

	// Validate path
	if err := validate.Path(share.Path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if share already exists
	existing, _ := m.Get(share.Name)
	if existing != nil {
		return fmt.Errorf("share '%s' already exists", share.Name)
	}

	// Verify path exists
	if _, err := m.fs.Stat(share.Path); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist", share.Path)
	}

	// Determine effective share type (backward compat: TimeMachine bool overrides)
	effectiveType := share.GetEffectiveShareType()

	// If Time Machine is enabled, ensure global config is set up
	if effectiveType == ShareTypeTimeMachine {
		if err := m.EnsureTimeMachineGlobalConfig(); err != nil {
			return fmt.Errorf("failed to configure Time Machine global settings: %w", err)
		}
	}

	// Apply permissions based on share type
	if err := m.FixPermissions(share); err != nil {
		// Log warning but don't fail creation? User might fix it later.
		// Or fail? "Failed to set permissions".
		// Given user context "we also need to set the permissions", I should probably warn or fail.
		// I'll return error to be safe.
		return fmt.Errorf("failed to set share permissions: %w", err)
	}

	// Backup config
	if err := m.backupConfig(); err != nil {
		return err
	}

	// Append share configuration
	confPath := m.getSambaConfPath()

	// Read existing content
	existingContent, err := m.fs.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Write share configuration
	var config strings.Builder
	config.Write(existingContent)
	config.WriteString(fmt.Sprintf("\n[%s]\n", share.Name))
	config.WriteString(fmt.Sprintf("   path = %s\n", share.Path))

	if share.Comment != "" {
		config.WriteString(fmt.Sprintf("   comment = %s\n", share.Comment))
	}

	if share.ReadOnly {
		config.WriteString("   read only = yes\n")
	} else {
		config.WriteString("   read only = no\n")
	}

	if share.Browseable {
		config.WriteString("   browseable = yes\n")
	} else {
		config.WriteString("   browseable = no\n")
	}

	if len(share.ValidUsers) > 0 {
		config.WriteString(fmt.Sprintf("   valid users = %s\n", strings.Join(share.ValidUsers, " ")))
	}

	// Add share type-specific configuration
	m.writeShareTypeConfig(&config, share, effectiveType)

	// Write back
	if err := m.fs.WriteFile(confPath, []byte(config.String()), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test configuration
	if err := m.testConfig(); err != nil {
		m.restoreConfig()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Reload Samba
	if err := m.reloadSamba(); err != nil {
		return err
	}

	// Update Avahi service for Time Machine discovery
	if effectiveType == ShareTypeTimeMachine {
		existingTMShares := m.getTimeMachineShareNames()
		if err := m.avahi.AddTimeMachineShare(share.Name, existingTMShares); err != nil {
			// Log warning but don't fail - Avahi is optional
			fmt.Printf("Warning: Failed to update Avahi service for Time Machine discovery: %v\n", err)
		}
	}

	return nil
}

// GetEffectiveShareType returns the share type, with backward compatibility for TimeMachine bool
func (s *Share) GetEffectiveShareType() string {
	// Backward compat: TimeMachine bool takes precedence if set
	if s.TimeMachine {
		return ShareTypeTimeMachine
	}
	if s.ShareType == "" {
		return ShareTypeGeneral
	}
	return s.ShareType
}

// writeShareTypeConfig writes share type-specific SMB configuration
func (m *Manager) writeShareTypeConfig(config *strings.Builder, share Share, shareType string) {
	switch shareType {
	case ShareTypeTimeMachine:
		config.WriteString("   vfs objects = catia fruit streams_xattr\n")
		config.WriteString("   fruit:metadata = stream\n")
		config.WriteString("   fruit:model = MacSamba\n")
		config.WriteString("   fruit:veto_appledouble = no\n")
		config.WriteString("   fruit:posix_rename = yes\n")
		config.WriteString("   fruit:zero_file_id = yes\n")
		config.WriteString("   fruit:wipe_intentionally_left_blank_rfork = yes\n")
		config.WriteString("   fruit:delete_empty_adfiles = yes\n")
		config.WriteString("   fruit:nfs_aces = no\n")
		config.WriteString("   fruit:aapl = yes\n")
		config.WriteString("   fruit:time machine = yes\n")
		// Add max size if specified (0 or empty means unlimited)
		maxSize := share.TimeMachineMaxSize
		if maxSize == "" {
			maxSize = "0" // Default to unlimited
		}
		if maxSize != "0" {
			config.WriteString(fmt.Sprintf("   fruit:time machine max size = %s\n", maxSize))
		}
		// Additional reliability options for Time Machine
		config.WriteString("   durable handles = yes\n")
		config.WriteString("   kernel oplocks = no\n")
		config.WriteString("   kernel share modes = no\n")
		config.WriteString("   posix locking = no\n")

	case ShareTypeUnifiProtect:
		// Ubiquiti Protect NVR storage requirements
		config.WriteString("   # Ubiquiti Protect optimized settings\n")
		config.WriteString("   create mask = 0660\n")
		config.WriteString("   directory mask = 0770\n")
		config.WriteString("   force create mode = 0660\n")
		config.WriteString("   force directory mode = 0770\n")
		config.WriteString("   inherit permissions = yes\n")
		config.WriteString("   nt acl support = yes\n")
		config.WriteString("   store dos attributes = yes\n")
		config.WriteString("   map acl inherit = yes\n")
		// Extended attributes for proper file handling
		config.WriteString("   ea support = yes\n")
		config.WriteString("   vfs objects = streams_xattr\n")

	case ShareTypeMedia:
		// Media server optimized settings (read-heavy, streaming)
		config.WriteString("   # Media server optimized settings\n")
		config.WriteString("   strict locking = no\n")
		config.WriteString("   min receivefile size = 16384\n")
		config.WriteString("   use sendfile = yes\n")
		config.WriteString("   aio read size = 16384\n")
		config.WriteString("   aio write size = 16384\n")

	case ShareTypeGeneral:
		// General purpose - no additional settings needed
	}
}

// getTimeMachineShareNames returns names of all Time Machine enabled shares
func (m *Manager) getTimeMachineShareNames() []string {
	shares, err := m.List()
	if err != nil {
		return nil
	}

	var names []string
	for _, share := range shares {
		if share.TimeMachine {
			names = append(names, share.Name)
		}
	}
	return names
}

// Remove deletes a Samba share
func (m *Manager) Remove(name string) error {
	shares, err := m.List()
	if err != nil {
		return err
	}

	// Check if share exists and if it's a Time Machine share
	var foundShare *Share
	for _, share := range shares {
		if share.Name == name {
			foundShare = &share
			break
		}
	}

	if foundShare == nil {
		return fmt.Errorf("share '%s' not found", name)
	}

	// Backup config
	if err := m.backupConfig(); err != nil {
		return err
	}

	confPath := m.getSambaConfPath()

	// Read entire config
	content, err := m.fs.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Remove share section
	lines := strings.Split(string(content), "\n")
	var newLines []string
	inShare := false
	targetShare := "[" + name + "]"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if entering target share section
		if trimmed == targetShare {
			inShare = true
			continue
		}

		// Check if entering another share section
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inShare = false
		}

		// Add line if not in target share
		if !inShare {
			newLines = append(newLines, line)
		}
	}

	// Write new config
	newContent := strings.Join(newLines, "\n")
	if err := m.fs.WriteFile(confPath, []byte(newContent), 0644); err != nil {
		m.restoreConfig()
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test configuration
	if err := m.testConfig(); err != nil {
		m.restoreConfig()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Reload Samba
	// Reload Samba
	if err := m.reloadSamba(); err != nil {
		return err
	}

	// Update Avahi service if this was a Time Machine share
	if foundShare.TimeMachine {
		remainingTMShares := m.getTimeMachineShareNames()
		if err := m.avahi.RemoveTimeMachineShare(name, remainingTMShares); err != nil {
			// Log warning but don't fail - Avahi is optional
			fmt.Printf("Warning: Failed to update Avahi service: %v\n", err)
		}
	}

	return nil
}

// Modify updates an existing share using atomic replacement
func (m *Manager) Modify(name string, updates map[string]interface{}) error {
	// Get current share
	share, err := m.Get(name)
	if err != nil {
		return err
	}

	// Track if Time Machine setting changed
	oldTimeMachine := share.TimeMachine

	// Apply updates
	if comment, ok := updates["comment"].(string); ok {
		share.Comment = comment
	}
	if readonly, ok := updates["readonly"].(bool); ok {
		share.ReadOnly = readonly
	}
	if browseable, ok := updates["browseable"].(bool); ok {
		share.Browseable = browseable
	}
	if validUsers, ok := updates["validusers"].([]string); ok {
		share.ValidUsers = validUsers
	}
	if timeMachine, ok := updates["timemachine"].(bool); ok {
		share.TimeMachine = timeMachine
	}
	if timeMachineMaxSize, ok := updates["timemachinemaxsize"].(string); ok {
		share.TimeMachineMaxSize = timeMachineMaxSize
	}

	// Backup config first
	if err := m.backupConfig(); err != nil {
		return err
	}

	confPath := m.getSambaConfPath()

	// Read entire config
	content, err := m.fs.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Remove old share section from content
	lines := strings.Split(string(content), "\n")
	var newLines []string
	inShare := false
	targetShare := "[" + name + "]"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == targetShare {
			inShare = true
			continue
		}

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inShare = false
		}

		if !inShare {
			newLines = append(newLines, line)
		}
	}

	// Build new share configuration
	var config strings.Builder
	config.WriteString(fmt.Sprintf("\n[%s]\n", share.Name))
	config.WriteString(fmt.Sprintf("   path = %s\n", share.Path))

	if share.Comment != "" {
		config.WriteString(fmt.Sprintf("   comment = %s\n", share.Comment))
	}

	if share.ReadOnly {
		config.WriteString("   read only = yes\n")
	} else {
		config.WriteString("   read only = no\n")
	}

	if share.Browseable {
		config.WriteString("   browseable = yes\n")
	} else {
		config.WriteString("   browseable = no\n")
	}

	if len(share.ValidUsers) > 0 {
		config.WriteString(fmt.Sprintf("   valid users = %s\n", strings.Join(share.ValidUsers, " ")))
	}

	if share.TimeMachine {
		config.WriteString("   vfs objects = catia fruit streams_xattr\n")
		config.WriteString("   fruit:metadata = stream\n")
		config.WriteString("   fruit:model = MacSamba\n")
		config.WriteString("   fruit:veto_appledouble = no\n")
		config.WriteString("   fruit:posix_rename = yes\n")
		config.WriteString("   fruit:zero_file_id = yes\n")
		config.WriteString("   fruit:wipe_intentionally_left_blank_rfork = yes\n")
		config.WriteString("   fruit:delete_empty_adfiles = yes\n")
		config.WriteString("   fruit:aapl = yes\n")
		config.WriteString("   fruit:time machine = yes\n")
		// Add max size if specified (0 or empty means unlimited)
		maxSize := share.TimeMachineMaxSize
		if maxSize == "" {
			maxSize = "0" // Default to unlimited
		}
		if maxSize != "0" {
			config.WriteString(fmt.Sprintf("   fruit:time machine max size = %s\n", maxSize))
		}
		// Additional reliability options for Time Machine
		config.WriteString("   durable handles = yes\n")
		config.WriteString("   kernel oplocks = no\n")
		config.WriteString("   kernel share modes = no\n")
		config.WriteString("   posix locking = no\n")
	}

	// Combine: existing content (minus old share) + new share config
	newContent := strings.Join(newLines, "\n") + config.String()

	// Write to temp file first for atomic operation
	tmpFile := confPath + ".tmp"
	if err := m.fs.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Test configuration before applying
	if err := m.exec.Run("testparm", "-s", tmpFile); err != nil {
		m.fs.Remove(tmpFile)
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Atomic rename
	if err := m.fs.Rename(tmpFile, confPath); err != nil {
		m.fs.Remove(tmpFile)
		m.restoreConfig()
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Reload Samba
	// Reload Samba
	if err := m.reloadSamba(); err != nil {
		return err
	}

	// Update Avahi service if Time Machine setting changed
	if oldTimeMachine != share.TimeMachine {
		tmShares := m.getTimeMachineShareNames()
		if share.TimeMachine {
			// Time Machine was enabled
			if err := m.avahi.AddTimeMachineShare(share.Name, tmShares); err != nil {
				fmt.Printf("Warning: Failed to update Avahi service for Time Machine discovery: %v\n", err)
			}
		} else {
			// Time Machine was disabled
			if err := m.avahi.RemoveTimeMachineShare(share.Name, tmShares); err != nil {
				fmt.Printf("Warning: Failed to update Avahi service: %v\n", err)
			}
		}
	}

	return nil
}

// FixPermissions enforces correct ownership and permissions for the share directory.
func (m *Manager) FixPermissions(share Share) error {
	effectiveType := share.GetEffectiveShareType()

	// Only apply special permissions for certain share types
	switch effectiveType {
	case ShareTypeTimeMachine:
		// Time Machine requires strict single-user permissions
		if len(share.ValidUsers) == 0 {
			return fmt.Errorf("Time Machine share has no valid users to assign ownership")
		}

		// Verify path exists
		if _, err := m.fs.Stat(share.Path); os.IsNotExist(err) {
			return fmt.Errorf("path '%s' does not exist", share.Path)
		}

		// Get UID of the primary user
		owner := share.ValidUsers[0]
		uid, err := user.GetSystemUID(owner)
		if err != nil {
			return fmt.Errorf("failed to resolve UID for user '%s': %w", owner, err)
		}

		// Chown to user (preserve group)
		if err := m.fs.Chown(share.Path, uid, -1); err != nil {
			return fmt.Errorf("failed to chown directory: %w", err)
		}

		// Chmod to 0700 (User only)
		if err := m.fs.Chmod(share.Path, 0700); err != nil {
			return fmt.Errorf("failed to chmod directory: %w", err)
		}

	case ShareTypeUnifiProtect:
		// Ubiquiti Protect needs group-writable permissions
		if _, err := m.fs.Stat(share.Path); os.IsNotExist(err) {
			return fmt.Errorf("path '%s' does not exist", share.Path)
		}

		// Set permissions to 0770 (user and group read/write/execute)
		if err := m.fs.Chmod(share.Path, 0770); err != nil {
			return fmt.Errorf("failed to chmod directory: %w", err)
		}

		// If valid users specified, chown to first user
		if len(share.ValidUsers) > 0 {
			owner := share.ValidUsers[0]
			uid, err := user.GetSystemUID(owner)
			if err != nil {
				return fmt.Errorf("failed to resolve UID for user '%s': %w", owner, err)
			}
			if err := m.fs.Chown(share.Path, uid, -1); err != nil {
				return fmt.Errorf("failed to chown directory: %w", err)
			}
		}

	case ShareTypeMedia, ShareTypeGeneral:
		// No special permissions needed
	}

	return nil
}

// Helper functions

func (m *Manager) backupConfig() error {
	confPath := m.getSambaConfPath()
	backupPath := m.getSambaBackupPath()

	input, err := m.fs.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := m.fs.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func (m *Manager) restoreConfig() error {
	confPath := m.getSambaConfPath()
	backupPath := m.getSambaBackupPath()

	input, err := m.fs.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := m.fs.WriteFile(confPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	return nil
}

func (m *Manager) testConfig() error {
	confPath := m.getSambaConfPath()
	if err := m.exec.Run("testparm", "-s", confPath); err != nil {
		return fmt.Errorf("configuration test failed: %w", err)
	}
	return nil
}

func (m *Manager) reloadSamba() error {
	if m.platform.IsMacOS() {
		// macOS: Send SIGHUP to smbd or use brew services
		if err := m.exec.Run("pkill", "-HUP", "smbd"); err != nil {
			// Try restarting via brew services
			if err := m.exec.Run("brew", "services", "restart", "samba"); err != nil {
				return fmt.Errorf("failed to reload samba: %w", err)
			}
		}
		return nil
	}

	// Linux: Try systemctl first
	if err := m.exec.Run("systemctl", "reload", "smbd"); err != nil {
		// Fallback to service command
		if err := m.exec.Run("service", "smbd", "reload"); err != nil {
			return fmt.Errorf("failed to reload samba: %w", err)
		}
	}
	return nil
}

// EnsureTimeMachineGlobalConfig checks and adds necessary global configuration for Time Machine
func (m *Manager) EnsureTimeMachineGlobalConfig() error {
	confPath := m.getSambaConfPath()

	content, err := m.fs.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	configStr := string(content)
	lines := strings.Split(configStr, "\n")

	// Check if Time Machine global settings already exist
	hasMinProtocol := strings.Contains(configStr, "min protocol")
	hasEASupport := strings.Contains(configStr, "ea support")
	hasVFSObjects := strings.Contains(configStr, "vfs objects") && strings.Contains(configStr, "[global]")

	// If all settings exist, no need to modify
	if hasMinProtocol && hasEASupport && hasVFSObjects {
		return nil
	}

	// Backup config
	if err := m.backupConfig(); err != nil {
		return err
	}

	// Find [global] section and add missing settings
	var newLines []string
	inGlobal := false
	globalSettingsAdded := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're entering global section
		if trimmed == "[global]" {
			inGlobal = true
			newLines = append(newLines, line)
			continue
		}

		// Check if we're leaving global section
		if inGlobal && strings.HasPrefix(trimmed, "[") && trimmed != "[global]" {
			// Add settings before leaving global section
			if !globalSettingsAdded {
				if !hasMinProtocol {
					newLines = append(newLines, "   min protocol = SMB2")
				}
				if !hasEASupport {
					newLines = append(newLines, "   ea support = yes")
				}
				if !hasVFSObjects {
					newLines = append(newLines, "   vfs objects = catia fruit streams_xattr")
				}
				globalSettingsAdded = true
			}
			inGlobal = false
		}

		newLines = append(newLines, line)

		// If we're at the end of global section (next line is a new section or end of file)
		if inGlobal && !globalSettingsAdded {
			// Check if next line is a section header or we're at the end
			if i+1 >= len(lines) || (strings.HasPrefix(strings.TrimSpace(lines[i+1]), "[") && strings.TrimSpace(lines[i+1]) != "[global]") {
				if !hasMinProtocol {
					newLines = append(newLines, "   min protocol = SMB2")
				}
				if !hasEASupport {
					newLines = append(newLines, "   ea support = yes")
				}
				if !hasVFSObjects {
					newLines = append(newLines, "   vfs objects = catia fruit streams_xattr")
				}
				globalSettingsAdded = true
			}
		}
	}

	// Write updated config
	newContent := strings.Join(newLines, "\n")
	if err := m.fs.WriteFile(confPath, []byte(newContent), 0644); err != nil {
		m.restoreConfig()
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test configuration
	if err := m.testConfig(); err != nil {
		m.restoreConfig()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return nil
}
