package samba

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sambo/pkg/platform"
	"sambo/pkg/service"
	"sambo/pkg/validate"
)

// getSambaConfPath returns the path to smb.conf
func getSambaConfPath() string {
	return platform.SambaConfigPath()
}

// getSambaBackupPath returns the path to the smb.conf backup
func getSambaBackupPath() string {
	return platform.SambaConfigPath() + ".backup"
}

// getSambaConfigDir returns the directory containing smb.conf
func getSambaConfigDir() string {
	return platform.SambaConfigDir()
}

// CheckInstalled verifies that Samba is installed and configured
func CheckInstalled() error {
	return CheckInstalledInteractive(true)
}

// CheckInstalledInteractive verifies that Samba is installed and optionally offers to install it
func CheckInstalledInteractive(interactive bool) error {
	// Check if smbd binary exists
	if _, err := exec.LookPath("smbd"); err != nil {
		if !interactive {
			if platform.IsMacOS() {
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
		if err := installSamba(); err != nil {
			return fmt.Errorf("failed to install Samba: %w", err)
		}

		fmt.Println("✓ Samba installed successfully")
	}

	confPath := getSambaConfPath()
	configDir := getSambaConfigDir()

	// Check if config file exists, create if missing
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		// Try to create directory
		if err := os.MkdirAll(configDir, 0755); err != nil {
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
		if err := os.WriteFile(confPath, []byte(basicConfig), 0644); err != nil {
			return fmt.Errorf("Samba is installed but cannot create config file: %w", err)
		}
		fmt.Println("✓ Created basic Samba configuration")
	}

	return nil
}

func installSamba() error {
	var cmd *exec.Cmd

	// Detect package manager
	if platform.IsMacOS() {
		if _, err := exec.LookPath("brew"); err == nil {
			fmt.Println("Installing Samba using Homebrew...")
			cmd = exec.Command("brew", "install", "samba")
		} else {
			return fmt.Errorf("Homebrew is not installed.\n\nPlease install Homebrew first: https://brew.sh\nThen run: brew install samba")
		}
	} else if _, err := exec.LookPath("apt-get"); err == nil {
		fmt.Println("Installing Samba using apt-get...")
		cmd = exec.Command("apt-get", "install", "-y", "samba")
	} else if _, err := exec.LookPath("yum"); err == nil {
		fmt.Println("Installing Samba using yum...")
		cmd = exec.Command("yum", "install", "-y", "samba")
	} else if _, err := exec.LookPath("dnf"); err == nil {
		fmt.Println("Installing Samba using dnf...")
		cmd = exec.Command("dnf", "install", "-y", "samba")
	} else if _, err := exec.LookPath("pacman"); err == nil {
		fmt.Println("Installing Samba using pacman...")
		cmd = exec.Command("pacman", "-S", "--noconfirm", "samba")
	} else if _, err := exec.LookPath("zypper"); err == nil {
		fmt.Println("Installing Samba using zypper...")
		cmd = exec.Command("zypper", "install", "-y", "samba")
	} else {
		if platform.IsMacOS() {
			return fmt.Errorf("could not detect package manager.\n\nPlease install Homebrew first: https://brew.sh\nThen run: brew install samba")
		}
		return fmt.Errorf("could not detect package manager.\n\nPlease install Samba manually:\n  Debian/Ubuntu: sudo apt-get install samba\n  RHEL/CentOS:   sudo yum install samba\n  Arch:          sudo pacman -S samba")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// Share represents a Samba share configuration
type Share struct {
	Name        string
	Path        string
	Comment     string
	ReadOnly    bool
	Browseable  bool
	ValidUsers  []string
	TimeMachine bool
}

// List returns all configured Samba shares
func List() ([]Share, error) {
	confPath := getSambaConfPath()
	file, err := os.Open(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Share{}, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", confPath, err)
	}
	defer file.Close()

	var shares []Share
	var currentShare *Share
	scanner := bufio.NewScanner(file)

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
func Get(name string) (*Share, error) {
	shares, err := List()
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
func Create(share Share) error {
	// Check if Samba service is available
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
	existing, _ := Get(share.Name)
	if existing != nil {
		return fmt.Errorf("share '%s' already exists", share.Name)
	}

	// Verify path exists
	if _, err := os.Stat(share.Path); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist", share.Path)
	}

	// If Time Machine is enabled, ensure global config is set up
	if share.TimeMachine {
		if err := EnsureTimeMachineGlobalConfig(); err != nil {
			return fmt.Errorf("failed to configure Time Machine global settings: %w", err)
		}
	}

	// Backup config
	if err := backupConfig(); err != nil {
		return err
	}

	// Append share configuration
	confPath := getSambaConfPath()
	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	// Write share configuration
	config := fmt.Sprintf("\n[%s]\n", share.Name)
	config += fmt.Sprintf("   path = %s\n", share.Path)

	if share.Comment != "" {
		config += fmt.Sprintf("   comment = %s\n", share.Comment)
	}

	if share.ReadOnly {
		config += "   read only = yes\n"
	} else {
		config += "   read only = no\n"
	}

	if share.Browseable {
		config += "   browseable = yes\n"
	} else {
		config += "   browseable = no\n"
	}

	if len(share.ValidUsers) > 0 {
		config += fmt.Sprintf("   valid users = %s\n", strings.Join(share.ValidUsers, " "))
	}

	// Add Time Machine support if enabled
	if share.TimeMachine {
		config += "   vfs objects = catia fruit streams_xattr\n"
		config += "   fruit:metadata = stream\n"
		config += "   fruit:model = MacSamba\n"
		config += "   fruit:veto_appledouble = no\n"
		config += "   fruit:posix_rename = yes\n"
		config += "   fruit:zero_file_id = yes\n"
		config += "   fruit:wipe_intentionally_left_blank_rfork = yes\n"
		config += "   fruit:delete_empty_adfiles = yes\n"
		config += "   fruit:aapl = yes\n"
		config += "   fruit:time machine = yes\n"
		config += "   fruit:time machine max size = 500G\n"
	}

	if _, err := f.WriteString(config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test configuration
	if err := testConfig(); err != nil {
		restoreConfig()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Reload Samba
	return reloadSamba()
}

// Remove deletes a Samba share
func Remove(name string) error {
	shares, err := List()
	if err != nil {
		return err
	}

	// Check if share exists
	found := false
	for _, share := range shares {
		if share.Name == name {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("share '%s' not found", name)
	}

	// Backup config
	if err := backupConfig(); err != nil {
		return err
	}

	confPath := getSambaConfPath()

	// Read entire config
	content, err := os.ReadFile(confPath)
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
	if err := os.WriteFile(confPath, []byte(newContent), 0644); err != nil {
		restoreConfig()
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test configuration
	if err := testConfig(); err != nil {
		restoreConfig()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Reload Samba
	return reloadSamba()
}

// Modify updates an existing share using atomic replacement
func Modify(name string, updates map[string]interface{}) error {
	// Get current share
	share, err := Get(name)
	if err != nil {
		return err
	}

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

	// Backup config first
	if err := backupConfig(); err != nil {
		return err
	}

	confPath := getSambaConfPath()

	// Read entire config
	content, err := os.ReadFile(confPath)
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
	config := fmt.Sprintf("\n[%s]\n", share.Name)
	config += fmt.Sprintf("   path = %s\n", share.Path)

	if share.Comment != "" {
		config += fmt.Sprintf("   comment = %s\n", share.Comment)
	}

	if share.ReadOnly {
		config += "   read only = yes\n"
	} else {
		config += "   read only = no\n"
	}

	if share.Browseable {
		config += "   browseable = yes\n"
	} else {
		config += "   browseable = no\n"
	}

	if len(share.ValidUsers) > 0 {
		config += fmt.Sprintf("   valid users = %s\n", strings.Join(share.ValidUsers, " "))
	}

	if share.TimeMachine {
		config += "   vfs objects = catia fruit streams_xattr\n"
		config += "   fruit:aapl = yes\n"
		config += "   fruit:time machine = yes\n"
		config += "   fruit:time machine max size = 500G\n"
	}

	// Combine: existing content (minus old share) + new share config
	newContent := strings.Join(newLines, "\n") + config

	// Write to temp file first for atomic operation
	tmpFile := confPath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Test configuration before applying
	cmd := exec.Command("testparm", "-s", tmpFile)
	if err := cmd.Run(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, confPath); err != nil {
		os.Remove(tmpFile)
		restoreConfig()
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Reload Samba
	return reloadSamba()
}

// Helper functions

func backupConfig() error {
	confPath := getSambaConfPath()
	backupPath := getSambaBackupPath()

	input, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func restoreConfig() error {
	confPath := getSambaConfPath()
	backupPath := getSambaBackupPath()

	input, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(confPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	return nil
}

func testConfig() error {
	confPath := getSambaConfPath()
	cmd := exec.Command("testparm", "-s", confPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("configuration test failed: %w", err)
	}
	return nil
}

func reloadSamba() error {
	if platform.IsMacOS() {
		// macOS: Send SIGHUP to smbd or use brew services
		cmd := exec.Command("pkill", "-HUP", "smbd")
		if err := cmd.Run(); err != nil {
			// Try restarting via brew services
			cmd = exec.Command("brew", "services", "restart", "samba")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to reload samba: %w", err)
			}
		}
		return nil
	}

	// Linux: Try systemctl first
	cmd := exec.Command("systemctl", "reload", "smbd")
	if err := cmd.Run(); err != nil {
		// Fallback to service command
		cmd = exec.Command("service", "smbd", "reload")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to reload samba: %w", err)
		}
	}
	return nil
}

// EnsureTimeMachineGlobalConfig checks and adds necessary global configuration for Time Machine
func EnsureTimeMachineGlobalConfig() error {
	confPath := getSambaConfPath()

	content, err := os.ReadFile(confPath)
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
	if err := backupConfig(); err != nil {
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
	if err := os.WriteFile(confPath, []byte(newContent), 0644); err != nil {
		restoreConfig()
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test configuration
	if err := testConfig(); err != nil {
		restoreConfig()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return nil
}
