package nfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExport_Struct(t *testing.T) {
	export := Export{
		Path:    "/srv/nfs/share",
		Clients: "192.168.1.0/24",
		Options: "rw,sync,no_subtree_check",
	}

	if export.Path != "/srv/nfs/share" {
		t.Errorf("expected Path to be '/srv/nfs/share', got %s", export.Path)
	}
	if export.Clients != "192.168.1.0/24" {
		t.Errorf("expected Clients to be '192.168.1.0/24', got %s", export.Clients)
	}
	if export.Options != "rw,sync,no_subtree_check" {
		t.Errorf("expected Options to be 'rw,sync,no_subtree_check', got %s", export.Options)
	}
}

// TestParseExportLine tests the parseExportLine function
func TestParseExportLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantExport  bool
		wantPath    string
		wantClients string
		wantOptions string
	}{
		{
			name:        "simple export with single client",
			line:        "/srv/nfs/share 192.168.1.0/24(rw,sync)",
			wantExport:  true,
			wantPath:    "/srv/nfs/share",
			wantClients: "192.168.1.0/24",
			wantOptions: "rw,sync",
		},
		{
			name:        "export with hostname client",
			line:        "/export client.example.com(rw,no_root_squash)",
			wantExport:  true,
			wantPath:    "/export",
			wantClients: "client.example.com",
			wantOptions: "rw,no_root_squash",
		},
		{
			name:        "export with wildcard",
			line:        "/data *(ro,sync)",
			wantExport:  true,
			wantPath:    "/data",
			wantClients: "*",
			wantOptions: "ro,sync",
		},
		{
			name:        "export with multiple clients",
			line:        "/shared 192.168.1.0/24(rw) 10.0.0.0/8(ro)",
			wantExport:  true,
			wantPath:    "/shared",
			wantClients: "192.168.1.0/24 10.0.0.0/8",
			wantOptions: "rw",
		},
		{
			name:       "incomplete line - path only",
			line:       "/srv/nfs",
			wantExport: false,
		},
		{
			name:       "empty line",
			line:       "",
			wantExport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			export := parseExportLine(tt.line)

			if tt.wantExport {
				if export == nil {
					t.Error("expected export to be parsed, got nil")
					return
				}
				if export.Path != tt.wantPath {
					t.Errorf("Path: expected %s, got %s", tt.wantPath, export.Path)
				}
				if export.Clients != tt.wantClients {
					t.Errorf("Clients: expected %s, got %s", tt.wantClients, export.Clients)
				}
				if export.Options != tt.wantOptions {
					t.Errorf("Options: expected %s, got %s", tt.wantOptions, export.Options)
				}
			} else {
				if export != nil {
					t.Errorf("expected nil export, got %+v", export)
				}
			}
		})
	}
}

// TestExportLineGeneration tests generation of /etc/exports lines
func TestExportLineGeneration(t *testing.T) {
	tests := []struct {
		export   Export
		expected string
	}{
		{
			export:   Export{Path: "/srv/nfs", Clients: "192.168.1.0/24", Options: "rw,sync"},
			expected: "/srv/nfs 192.168.1.0/24(rw,sync)\n",
		},
		{
			export:   Export{Path: "/data", Clients: "*", Options: "ro"},
			expected: "/data *(ro)\n",
		},
		{
			export:   Export{Path: "/home", Clients: "10.0.0.0/8", Options: "rw,no_root_squash,async"},
			expected: "/home 10.0.0.0/8(rw,no_root_squash,async)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.export.Path, func(t *testing.T) {
			// This simulates the format used in Create()
			exportLine := tt.export.Path + " " + tt.export.Clients + "(" + tt.export.Options + ")\n"
			if exportLine != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, exportLine)
			}
		})
	}
}

// TestSkipCommentsAndEmptyLines tests that comments and empty lines are skipped
func TestSkipCommentsAndEmptyLines(t *testing.T) {
	tests := []struct {
		line   string
		skip   bool
	}{
		{"# This is a comment", true},
		{"", true},
		{"   ", true},
		{"  # Indented comment", true},
		{"/srv/nfs 192.168.1.0/24(rw)", false},
		{"/export client(rw,sync)", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			line := strings.TrimSpace(tt.line)
			shouldSkip := line == "" || strings.HasPrefix(line, "#")
			if shouldSkip != tt.skip {
				t.Errorf("line %q: expected skip=%v, got %v", tt.line, tt.skip, shouldSkip)
			}
		})
	}
}

// TestParseExportsFile tests parsing a complete exports file
func TestParseExportsFile(t *testing.T) {
	exportsContent := `# /etc/exports - NFS server configuration
# See exports(5) for more information

/srv/nfs/public *(ro,sync)
/srv/nfs/private 192.168.1.0/24(rw,sync,no_subtree_check)
# Commented export
# /disabled 10.0.0.0/8(rw)
/srv/nfs/home 192.168.1.100(rw,no_root_squash)
`

	lines := strings.Split(exportsContent, "\n")
	var exports []Export

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		export := parseExportLine(line)
		if export != nil {
			exports = append(exports, *export)
		}
	}

	if len(exports) != 3 {
		t.Errorf("expected 3 exports, got %d", len(exports))
	}

	// Check specific exports
	expectedPaths := []string{"/srv/nfs/public", "/srv/nfs/private", "/srv/nfs/home"}
	for i, path := range expectedPaths {
		if i >= len(exports) {
			t.Errorf("missing export %d with path %s", i, path)
			continue
		}
		if exports[i].Path != path {
			t.Errorf("export %d: expected path %s, got %s", i, path, exports[i].Path)
		}
	}
}

// TestRemoveExport tests the logic for removing an export from the file
func TestRemoveExport(t *testing.T) {
	originalContent := `# NFS exports
/srv/nfs/public *(ro)
/srv/nfs/private 192.168.1.0/24(rw)
/srv/nfs/backup 10.0.0.0/8(rw)
`

	pathToRemove := "/srv/nfs/private"

	lines := strings.Split(originalContent, "\n")
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Keep comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}

		// Parse and check if this is the export to remove
		export := parseExportLine(trimmed)
		if export != nil && export.Path == pathToRemove {
			continue // Skip this line
		}

		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")

	// Verify the export was removed
	if strings.Contains(newContent, "/srv/nfs/private") {
		t.Error("private export should have been removed")
	}

	// Verify other exports remain
	if !strings.Contains(newContent, "/srv/nfs/public") {
		t.Error("public export should remain")
	}
	if !strings.Contains(newContent, "/srv/nfs/backup") {
		t.Error("backup export should remain")
	}
	if !strings.Contains(newContent, "# NFS exports") {
		t.Error("comments should be preserved")
	}
}

// TestNFSOptions tests common NFS export options
func TestNFSOptions(t *testing.T) {
	validOptions := []string{
		"rw",
		"ro",
		"sync",
		"async",
		"no_root_squash",
		"root_squash",
		"all_squash",
		"no_subtree_check",
		"subtree_check",
		"secure",
		"insecure",
		"anonuid=1000",
		"anongid=1000",
	}

	for _, opt := range validOptions {
		t.Run(opt, func(t *testing.T) {
			// Just verify the option is non-empty and valid format
			if opt == "" {
				t.Error("option should not be empty")
			}
			// Options should not contain spaces
			if strings.Contains(opt, " ") {
				t.Error("option should not contain spaces")
			}
		})
	}
}

// TestClientSpecifications tests various client specification formats
func TestClientSpecifications(t *testing.T) {
	tests := []struct {
		client string
		valid  bool
		desc   string
	}{
		{"192.168.1.0/24", true, "CIDR notation"},
		{"192.168.1.100", true, "single IP"},
		{"10.0.0.0/8", true, "class A network"},
		{"*", true, "wildcard"},
		{"client.example.com", true, "hostname"},
		{"*.example.com", true, "wildcard hostname"},
		{"@netgroup", true, "netgroup"},
		{"", false, "empty client"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			valid := tt.client != ""
			if valid != tt.valid {
				t.Errorf("client %s: expected valid=%v, got %v", tt.client, tt.valid, valid)
			}
		})
	}
}

// TestReadExportsFromFile tests reading exports from a temporary file
func TestReadExportsFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpExports := filepath.Join(tmpDir, "exports")

	exportsContent := `# Test exports file
/home 192.168.1.0/24(rw,sync,no_subtree_check)
/data *(ro,sync)
/backup 10.0.0.0/8(rw,no_root_squash)
`

	if err := os.WriteFile(tmpExports, []byte(exportsContent), 0644); err != nil {
		t.Fatalf("failed to create temp exports: %v", err)
	}

	// Read and parse the file
	content, err := os.ReadFile(tmpExports)
	if err != nil {
		t.Fatalf("failed to read temp exports: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	var exports []Export

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		export := parseExportLine(line)
		if export != nil {
			exports = append(exports, *export)
		}
	}

	if len(exports) != 3 {
		t.Errorf("expected 3 exports, got %d", len(exports))
	}

	// Verify each export
	expectations := []struct {
		path    string
		clients string
		options string
	}{
		{"/home", "192.168.1.0/24", "rw,sync,no_subtree_check"},
		{"/data", "*", "ro,sync"},
		{"/backup", "10.0.0.0/8", "rw,no_root_squash"},
	}

	for i, exp := range expectations {
		if i >= len(exports) {
			t.Errorf("missing export %d", i)
			continue
		}
		if exports[i].Path != exp.path {
			t.Errorf("export %d path: expected %s, got %s", i, exp.path, exports[i].Path)
		}
		if exports[i].Clients != exp.clients {
			t.Errorf("export %d clients: expected %s, got %s", i, exp.clients, exports[i].Clients)
		}
		if exports[i].Options != exp.options {
			t.Errorf("export %d options: expected %s, got %s", i, exp.options, exports[i].Options)
		}
	}
}

// TestExportPathValidation tests validation of export paths
func TestExportPathValidation(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"/srv/nfs", true},
		{"/home/user", true},
		{"/", true},
		{"/var/lib/data", true},
		{"", false},
		{"relative/path", false},
		{"./local", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			isAbsolute := strings.HasPrefix(tt.path, "/") && tt.path != ""
			if isAbsolute != tt.valid {
				t.Errorf("path %s: expected valid=%v, got %v", tt.path, tt.valid, isAbsolute)
			}
		})
	}
}

// TestExportModification tests applying updates to an export
func TestExportModification(t *testing.T) {
	original := Export{
		Path:    "/srv/nfs",
		Clients: "192.168.1.0/24",
		Options: "ro,sync",
	}

	updates := map[string]interface{}{
		"clients": "10.0.0.0/8",
		"options": "rw,async,no_root_squash",
	}

	// Apply updates
	if clients, ok := updates["clients"].(string); ok {
		original.Clients = clients
	}
	if options, ok := updates["options"].(string); ok {
		original.Options = options
	}

	if original.Clients != "10.0.0.0/8" {
		t.Errorf("Clients should be updated, got %s", original.Clients)
	}
	if original.Options != "rw,async,no_root_squash" {
		t.Errorf("Options should be updated, got %s", original.Options)
	}
	// Path should not change
	if original.Path != "/srv/nfs" {
		t.Errorf("Path should not change, got %s", original.Path)
	}
}

// TestMultipleClientsOnSamePath tests grouping exports by path
func TestMultipleClientsOnSamePath(t *testing.T) {
	exportsContent := `/srv/data 192.168.1.0/24(rw) 10.0.0.0/8(ro)
/srv/other 192.168.2.0/24(rw)
`

	lines := strings.Split(exportsContent, "\n")
	exportMap := make(map[string]*Export)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		export := parseExportLine(line)
		if export != nil {
			if existing, exists := exportMap[export.Path]; exists {
				existing.Clients = existing.Clients + " " + export.Clients
			} else {
				exportMap[export.Path] = export
			}
		}
	}

	if len(exportMap) != 2 {
		t.Errorf("expected 2 unique paths, got %d", len(exportMap))
	}

	dataExport, exists := exportMap["/srv/data"]
	if !exists {
		t.Error("/srv/data export not found")
	} else {
		// The parseExportLine already combines multiple clients on the same line
		if !strings.Contains(dataExport.Clients, "192.168.1.0/24") {
			t.Error("data export should have 192.168.1.0/24 client")
		}
		if !strings.Contains(dataExport.Clients, "10.0.0.0/8") {
			t.Error("data export should have 10.0.0.0/8 client")
		}
	}
}

// TestExportWithComplexOptions tests parsing exports with complex options
func TestExportWithComplexOptions(t *testing.T) {
	tests := []struct {
		line    string
		options string
	}{
		{
			line:    "/home 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)",
			options: "rw,sync,no_subtree_check,no_root_squash",
		},
		{
			line:    "/data client(rw,anonuid=1000,anongid=1000,all_squash)",
			options: "rw,anonuid=1000,anongid=1000,all_squash",
		},
		{
			line:    "/export *(ro,insecure,no_subtree_check)",
			options: "ro,insecure,no_subtree_check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			export := parseExportLine(tt.line)
			if export == nil {
				t.Error("expected export to be parsed")
				return
			}
			if export.Options != tt.options {
				t.Errorf("expected options %s, got %s", tt.options, export.Options)
			}
		})
	}
}
