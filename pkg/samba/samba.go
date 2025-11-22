package samba

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	sambaConfPath = "/etc/samba/smb.conf"
	sambaBackup   = "/etc/samba/smb.conf.backup"
)

// Share represents a Samba share configuration
type Share struct {
	Name       string
	Path       string
	Comment    string
	ReadOnly   bool
	Browseable bool
	ValidUsers []string
}

// List returns all configured Samba shares
func List() ([]Share, error) {
	file, err := os.Open(sambaConfPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Share{}, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", sambaConfPath, err)
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
	// Check if share already exists
	existing, _ := Get(share.Name)
	if existing != nil {
		return fmt.Errorf("share '%s' already exists", share.Name)
	}

	// Verify path exists
	if _, err := os.Stat(share.Path); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist", share.Path)
	}

	// Backup config
	if err := backupConfig(); err != nil {
		return err
	}

	// Append share configuration
	f, err := os.OpenFile(sambaConfPath, os.O_APPEND|os.O_WRONLY, 0644)
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

	// Read entire config
	content, err := os.ReadFile(sambaConfPath)
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
	if err := os.WriteFile(sambaConfPath, []byte(newContent), 0644); err != nil {
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

// Modify updates an existing share
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

	// Remove old share and create new one
	if err := Remove(name); err != nil {
		return err
	}

	return Create(*share)
}

// Helper functions

func backupConfig() error {
	input, err := os.ReadFile(sambaConfPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := os.WriteFile(sambaBackup, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func restoreConfig() error {
	input, err := os.ReadFile(sambaBackup)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(sambaConfPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	return nil
}

func testConfig() error {
	cmd := exec.Command("testparm", "-s", sambaConfPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("configuration test failed: %w", err)
	}
	return nil
}

func reloadSamba() error {
	// Try systemctl first
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
