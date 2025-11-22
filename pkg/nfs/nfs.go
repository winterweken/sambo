package nfs

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	exportsPath   = "/etc/exports"
	exportsBackup = "/etc/exports.backup"
)

// Export represents an NFS export configuration
type Export struct {
	Path    string
	Clients string
	Options string
}

// List returns all configured NFS exports
func List() ([]Export, error) {
	file, err := os.Open(exportsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Export{}, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", exportsPath, err)
	}
	defer file.Close()

	var exports []Export
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse export line: /path client(options)
		export := parseExportLine(line)
		if export != nil {
			exports = append(exports, *export)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading exports: %w", err)
	}

	return exports, nil
}

// Get retrieves a specific export by path
func Get(path string) (*Export, error) {
	exports, err := List()
	if err != nil {
		return nil, err
	}

	for _, export := range exports {
		if export.Path == path {
			return &export, nil
		}
	}

	return nil, fmt.Errorf("export '%s' not found", path)
}

// Create adds a new NFS export
func Create(export Export) error {
	// Check if export already exists
	existing, _ := Get(export.Path)
	if existing != nil {
		return fmt.Errorf("export '%s' already exists", export.Path)
	}

	// Verify path exists
	if _, err := os.Stat(export.Path); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist", export.Path)
	}

	// Backup config
	if err := backupConfig(); err != nil {
		return err
	}

	// Append export
	f, err := os.OpenFile(exportsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open exports file: %w", err)
	}
	defer f.Close()

	// Format: /path client(options)
	exportLine := fmt.Sprintf("%s %s(%s)\n", export.Path, export.Clients, export.Options)

	if _, err := f.WriteString(exportLine); err != nil {
		return fmt.Errorf("failed to write export: %w", err)
	}

	// Reload NFS exports
	return reloadNFS()
}

// Remove deletes an NFS export
func Remove(path string) error {
	exports, err := List()
	if err != nil {
		return err
	}

	// Check if export exists
	found := false
	for _, export := range exports {
		if export.Path == path {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("export '%s' not found", path)
	}

	// Backup config
	if err := backupConfig(); err != nil {
		return err
	}

	// Read entire config
	content, err := os.ReadFile(exportsPath)
	if err != nil {
		return fmt.Errorf("failed to read exports: %w", err)
	}

	// Remove export line
	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}

		// Parse and check if this is the export to remove
		export := parseExportLine(trimmed)
		if export != nil && export.Path == path {
			continue // Skip this line
		}

		newLines = append(newLines, line)
	}

	// Write new config
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(exportsPath, []byte(newContent), 0644); err != nil {
		restoreConfig()
		return fmt.Errorf("failed to write exports: %w", err)
	}

	// Reload NFS exports
	return reloadNFS()
}

// Modify updates an existing export
func Modify(path string, updates map[string]interface{}) error {
	// Get current export
	export, err := Get(path)
	if err != nil {
		return err
	}

	// Apply updates
	if clients, ok := updates["clients"].(string); ok {
		export.Clients = clients
	}
	if options, ok := updates["options"].(string); ok {
		export.Options = options
	}

	// Remove old export and create new one
	if err := Remove(path); err != nil {
		return err
	}

	return Create(*export)
}

// Helper functions

func parseExportLine(line string) *Export {
	// Format: /path client(options) or /path client1(options) client2(options)
	// For simplicity, we'll handle single client specs
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	path := parts[0]

	// Parse client and options
	clientPart := parts[1]
	clientsOptions := strings.SplitN(clientPart, "(", 2)

	clients := clientsOptions[0]
	options := ""

	if len(clientsOptions) == 2 {
		options = strings.TrimSuffix(clientsOptions[1], ")")
	}

	return &Export{
		Path:    path,
		Clients: clients,
		Options: options,
	}
}

func backupConfig() error {
	// Check if file exists
	if _, err := os.Stat(exportsPath); os.IsNotExist(err) {
		// Create empty file
		f, err := os.Create(exportsPath)
		if err != nil {
			return fmt.Errorf("failed to create exports file: %w", err)
		}
		f.Close()
		return nil
	}

	input, err := os.ReadFile(exportsPath)
	if err != nil {
		return fmt.Errorf("failed to read exports: %w", err)
	}

	if err := os.WriteFile(exportsBackup, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func restoreConfig() error {
	input, err := os.ReadFile(exportsBackup)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(exportsPath, input, 0644); err != nil {
		return fmt.Errorf("failed to restore exports: %w", err)
	}

	return nil
}

func reloadNFS() error {
	// Try exportfs command
	cmd := exec.Command("exportfs", "-ra")
	if err := cmd.Run(); err != nil {
		// Fallback to systemctl restart
		cmd = exec.Command("systemctl", "restart", "nfs-server")
		if err := cmd.Run(); err != nil {
			// Try another service name
			cmd = exec.Command("systemctl", "restart", "nfs-kernel-server")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to reload NFS exports: %w", err)
			}
		}
	}
	return nil
}
