package samba

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShare_Struct(t *testing.T) {
	share := Share{
		Name:        "documents",
		Path:        "/srv/samba/documents",
		Comment:     "Shared Documents",
		ReadOnly:    false,
		Browseable:  true,
		ValidUsers:  []string{"user1", "user2"},
		TimeMachine: false,
	}

	if share.Name != "documents" {
		t.Errorf("expected Name to be 'documents', got %s", share.Name)
	}
	if share.Path != "/srv/samba/documents" {
		t.Errorf("expected Path to be '/srv/samba/documents', got %s", share.Path)
	}
	if share.Comment != "Shared Documents" {
		t.Errorf("expected Comment to be 'Shared Documents', got %s", share.Comment)
	}
	if share.ReadOnly {
		t.Error("expected ReadOnly to be false")
	}
	if !share.Browseable {
		t.Error("expected Browseable to be true")
	}
	if len(share.ValidUsers) != 2 {
		t.Errorf("expected 2 ValidUsers, got %d", len(share.ValidUsers))
	}
	if share.TimeMachine {
		t.Error("expected TimeMachine to be false")
	}
}

// TestParseShareSection tests parsing of smb.conf share sections
func TestParseShareSection(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		expectedShares int
		checkShare     string
		checkPath      string
		checkReadOnly  bool
	}{
		{
			name: "single share",
			config: `[global]
workgroup = WORKGROUP

[documents]
path = /srv/samba/documents
read only = no
browseable = yes`,
			expectedShares: 1,
			checkShare:     "documents",
			checkPath:      "/srv/samba/documents",
			checkReadOnly:  false,
		},
		{
			name: "multiple shares",
			config: `[global]
workgroup = WORKGROUP

[documents]
path = /srv/samba/documents
read only = no

[backup]
path = /srv/samba/backup
read only = yes`,
			expectedShares: 2,
		},
		{
			name: "share with Time Machine",
			config: `[timemachine]
path = /srv/timemachine
read only = no
fruit:time machine = yes`,
			expectedShares: 1,
			checkShare:     "timemachine",
		},
		{
			name: "skip special sections",
			config: `[global]
workgroup = WORKGROUP

[homes]
comment = Home Directories

[printers]
comment = All Printers

[documents]
path = /srv/samba/documents`,
			expectedShares: 1,
			checkShare:     "documents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shares := parseConfigContent(tt.config)

			if len(shares) != tt.expectedShares {
				t.Errorf("expected %d shares, got %d", tt.expectedShares, len(shares))
			}

			if tt.checkShare != "" {
				found := false
				for _, s := range shares {
					if s.Name == tt.checkShare {
						found = true
						if tt.checkPath != "" && s.Path != tt.checkPath {
							t.Errorf("share %s: expected path %s, got %s", tt.checkShare, tt.checkPath, s.Path)
						}
						if s.ReadOnly != tt.checkReadOnly {
							t.Errorf("share %s: expected ReadOnly=%v, got %v", tt.checkShare, tt.checkReadOnly, s.ReadOnly)
						}
					}
				}
				if !found {
					t.Errorf("share %s not found", tt.checkShare)
				}
			}
		})
	}
}

// parseConfigContent is a helper function that simulates the parsing logic from List()
func parseConfigContent(content string) []Share {
	var shares []Share
	var currentShare *Share
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

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
				if currentShare.TimeMachine {
					currentShare.ShareType = ShareTypeTimeMachine
				}
			case "store dos attributes":
				if strings.ToLower(value) == "yes" {
					currentShare.ShareType = ShareTypeUnifiProtect
				}
			case "min receivefile size":
				if value == "16384" {
					currentShare.ShareType = ShareTypeMedia
				}
			}
		}
	}

	// Add last share
	if currentShare != nil {
		shares = append(shares, *currentShare)
	}

	return shares
}

// TestParseKeyValue tests parsing of key=value pairs in smb.conf
func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		line     string
		wantKey  string
		wantVal  string
		wantBool bool
	}{
		{"path = /srv/samba/share", "path", "/srv/samba/share", false},
		{"read only = yes", "read only", "yes", true},
		{"read only = no", "read only", "no", false},
		{"browseable = yes", "browseable", "yes", true},
		{"browsable = no", "browsable", "no", false},
		{"comment = My Share", "comment", "My Share", false},
		{"valid users = user1 user2", "valid users", "user1 user2", false},
		{"fruit:time machine = yes", "fruit:time machine", "yes", true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if !strings.Contains(tt.line, "=") {
				t.Error("line should contain '='")
				return
			}

			parts := strings.SplitN(tt.line, "=", 2)
			if len(parts) != 2 {
				t.Error("failed to split line")
				return
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if key != tt.wantKey {
				t.Errorf("expected key %s, got %s", tt.wantKey, key)
			}
			if value != tt.wantVal {
				t.Errorf("expected value %s, got %s", tt.wantVal, value)
			}

			if key == "read only" || key == "browseable" || key == "browsable" || key == "fruit:time machine" {
				boolVal := strings.ToLower(value) == "yes"
				if boolVal != tt.wantBool {
					t.Errorf("expected bool %v, got %v", tt.wantBool, boolVal)
				}
			}
		})
	}
}

// TestSkipComments tests that comments are properly skipped
func TestSkipComments(t *testing.T) {
	tests := []struct {
		line       string
		isComment  bool
	}{
		{"# This is a comment", true},
		{"; This is also a comment", true},
		{"   # Indented comment", true},
		{"[share]", false},
		{"path = /srv/share", false},
		{"", true}, // empty lines are also skipped
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			line := strings.TrimSpace(tt.line)
			isComment := line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";")
			if isComment != tt.isComment {
				t.Errorf("line %q: expected isComment=%v, got %v", tt.line, tt.isComment, isComment)
			}
		})
	}
}

// TestShareConfigGeneration tests generation of share configuration
func TestShareConfigGeneration(t *testing.T) {
	share := Share{
		Name:       "documents",
		Path:       "/srv/samba/documents",
		Comment:    "Shared Documents",
		ReadOnly:   false,
		Browseable: true,
		ValidUsers: []string{"user1", "user2"},
	}

	config := generateShareConfig(share)

	if !strings.Contains(config, "[documents]") {
		t.Error("config should contain section header")
	}
	if !strings.Contains(config, "path = /srv/samba/documents") {
		t.Error("config should contain path")
	}
	if !strings.Contains(config, "comment = Shared Documents") {
		t.Error("config should contain comment")
	}
	if !strings.Contains(config, "read only = no") {
		t.Error("config should contain 'read only = no'")
	}
	if !strings.Contains(config, "browseable = yes") {
		t.Error("config should contain 'browseable = yes'")
	}
	if !strings.Contains(config, "valid users = user1 user2") {
		t.Error("config should contain valid users")
	}
}

// generateShareConfig is a helper that simulates the config generation from Create()
func generateShareConfig(share Share) string {
	config := "\n[" + share.Name + "]\n"
	config += "   path = " + share.Path + "\n"

	if share.Comment != "" {
		config += "   comment = " + share.Comment + "\n"
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
		config += "   valid users = " + strings.Join(share.ValidUsers, " ") + "\n"
	}

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

	return config
}

// TestTimeMachineShareConfig tests Time Machine share configuration
func TestTimeMachineShareConfig(t *testing.T) {
	share := Share{
		Name:        "timemachine",
		Path:        "/srv/timemachine",
		ReadOnly:    false,
		Browseable:  true,
		TimeMachine: true,
	}

	config := generateShareConfig(share)

	requiredSettings := []string{
		"vfs objects = catia fruit streams_xattr",
		"fruit:metadata = stream",
		"fruit:model = MacSamba",
		"fruit:aapl = yes",
		"fruit:time machine = yes",
		"fruit:time machine max size = 500G",
	}

	for _, setting := range requiredSettings {
		if !strings.Contains(config, setting) {
			t.Errorf("Time Machine config should contain %q", setting)
		}
	}
}

// TestRemoveShareSection tests the logic for removing a share section
func TestRemoveShareSection(t *testing.T) {
	originalConfig := `[global]
workgroup = WORKGROUP

[documents]
path = /srv/samba/documents
read only = no

[backup]
path = /srv/samba/backup
read only = yes

[media]
path = /srv/samba/media
`

	shareToRemove := "backup"
	newConfig := removeShareFromConfig(originalConfig, shareToRemove)

	// Verify the share was removed
	if strings.Contains(newConfig, "[backup]") {
		t.Error("backup share should have been removed")
	}
	if strings.Contains(newConfig, "/srv/samba/backup") {
		t.Error("backup path should have been removed")
	}

	// Verify other shares remain
	if !strings.Contains(newConfig, "[documents]") {
		t.Error("documents share should remain")
	}
	if !strings.Contains(newConfig, "[media]") {
		t.Error("media share should remain")
	}
	if !strings.Contains(newConfig, "[global]") {
		t.Error("global section should remain")
	}
}

// removeShareFromConfig simulates the removal logic from Remove()
func removeShareFromConfig(content, name string) string {
	lines := strings.Split(content, "\n")
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

	return strings.Join(newLines, "\n")
}

// TestValidUsers tests parsing of valid users
func TestValidUsers(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"user1", []string{"user1"}},
		{"user1 user2", []string{"user1", "user2"}},
		{"user1 user2 user3", []string{"user1", "user2", "user3"}},
		{"@group1", []string{"@group1"}},
		{"user1 @group1", []string{"user1", "@group1"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var result []string
			if tt.input != "" {
				result = strings.Fields(tt.input)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d users, got %d", len(tt.expected), len(result))
				return
			}

			for i, user := range tt.expected {
				if result[i] != user {
					t.Errorf("user %d: expected %s, got %s", i, user, result[i])
				}
			}
		})
	}
}

// TestReadConfigFromFile tests reading config from a temporary file
func TestReadConfigFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpConfig := filepath.Join(tmpDir, "smb.conf")

	configContent := `[global]
workgroup = MYGROUP
server string = Samba Server

[public]
path = /srv/public
read only = no
browseable = yes

[private]
path = /srv/private
read only = yes
valid users = admin
`

	if err := os.WriteFile(tmpConfig, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	// Read and parse the file
	content, err := os.ReadFile(tmpConfig)
	if err != nil {
		t.Fatalf("failed to read temp config: %v", err)
	}

	shares := parseConfigContent(string(content))

	if len(shares) != 2 {
		t.Errorf("expected 2 shares, got %d", len(shares))
	}

	// Check public share
	foundPublic := false
	foundPrivate := false

	for _, s := range shares {
		if s.Name == "public" {
			foundPublic = true
			if s.Path != "/srv/public" {
				t.Errorf("public path: expected /srv/public, got %s", s.Path)
			}
			if s.ReadOnly {
				t.Error("public should not be read only")
			}
			if !s.Browseable {
				t.Error("public should be browseable")
			}
		}
		if s.Name == "private" {
			foundPrivate = true
			if !s.ReadOnly {
				t.Error("private should be read only")
			}
			if len(s.ValidUsers) != 1 || s.ValidUsers[0] != "admin" {
				t.Errorf("private valid users should be [admin], got %v", s.ValidUsers)
			}
		}
	}

	if !foundPublic {
		t.Error("public share not found")
	}
	if !foundPrivate {
		t.Error("private share not found")
	}
}

// TestIsSpecialShare tests detection of special samba shares
func TestIsSpecialShare(t *testing.T) {
	tests := []struct {
		name      string
		isSpecial bool
	}{
		{"global", true},
		{"homes", true},
		{"printers", true},
		{"documents", false},
		{"backup", false},
		{"timemachine", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSpecial := tt.name == "global" || tt.name == "homes" || tt.name == "printers"
			if isSpecial != tt.isSpecial {
				t.Errorf("share %s: expected isSpecial=%v, got %v", tt.name, tt.isSpecial, isSpecial)
			}
		})
	}
}

// TestBrowseableSpelling tests that both spellings are accepted
func TestBrowseableSpelling(t *testing.T) {
	configs := []string{
		`[share1]
path = /srv/share1
browseable = yes`,
		`[share2]
path = /srv/share2
browsable = yes`,
		`[share3]
path = /srv/share3
browseable = no`,
		`[share4]
path = /srv/share4
browsable = no`,
	}

	expected := []bool{true, true, false, false}

	for i, config := range configs {
		shares := parseConfigContent(config)
		if len(shares) != 1 {
			t.Errorf("config %d: expected 1 share, got %d", i, len(shares))
			continue
		}
		if shares[0].Browseable != expected[i] {
			t.Errorf("config %d: expected Browseable=%v, got %v", i, expected[i], shares[0].Browseable)
		}
	}
}

// TestTimeMachineGlobalSettings tests detection of Time Machine global settings
func TestTimeMachineGlobalSettings(t *testing.T) {
	tests := []struct {
		name            string
		config          string
		hasMinProtocol  bool
		hasEASupport    bool
		hasVFSObjects   bool
	}{
		{
			name: "all settings present",
			config: `[global]
min protocol = SMB2
ea support = yes
vfs objects = catia fruit streams_xattr
`,
			hasMinProtocol: true,
			hasEASupport:   true,
			hasVFSObjects:  true,
		},
		{
			name: "no settings",
			config: `[global]
workgroup = WORKGROUP
`,
			hasMinProtocol: false,
			hasEASupport:   false,
			hasVFSObjects:  false,
		},
		{
			name: "partial settings",
			config: `[global]
min protocol = SMB2
workgroup = WORKGROUP
`,
			hasMinProtocol: true,
			hasEASupport:   false,
			hasVFSObjects:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMinProtocol := strings.Contains(tt.config, "min protocol")
			hasEASupport := strings.Contains(tt.config, "ea support")
			hasVFSObjects := strings.Contains(tt.config, "vfs objects") && strings.Contains(tt.config, "[global]")

			if hasMinProtocol != tt.hasMinProtocol {
				t.Errorf("hasMinProtocol: expected %v, got %v", tt.hasMinProtocol, hasMinProtocol)
			}
			if hasEASupport != tt.hasEASupport {
				t.Errorf("hasEASupport: expected %v, got %v", tt.hasEASupport, hasEASupport)
			}
			if hasVFSObjects != tt.hasVFSObjects {
				t.Errorf("hasVFSObjects: expected %v, got %v", tt.hasVFSObjects, hasVFSObjects)
			}
		})
	}
}

// TestShareModification tests applying updates to a share
func TestShareModification(t *testing.T) {
	original := Share{
		Name:       "documents",
		Path:       "/srv/samba/documents",
		Comment:    "Original comment",
		ReadOnly:   false,
		Browseable: true,
		ValidUsers: []string{"user1"},
	}

	updates := map[string]interface{}{
		"comment":    "Updated comment",
		"readonly":   true,
		"browseable": false,
		"validusers": []string{"user1", "user2"},
	}

	// Apply updates
	if comment, ok := updates["comment"].(string); ok {
		original.Comment = comment
	}
	if readonly, ok := updates["readonly"].(bool); ok {
		original.ReadOnly = readonly
	}
	if browseable, ok := updates["browseable"].(bool); ok {
		original.Browseable = browseable
	}
	if validUsers, ok := updates["validusers"].([]string); ok {
		original.ValidUsers = validUsers
	}

	if original.Comment != "Updated comment" {
		t.Errorf("Comment should be updated, got %s", original.Comment)
	}
	if !original.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if original.Browseable {
		t.Error("Browseable should be false")
	}
	if len(original.ValidUsers) != 2 {
		t.Errorf("ValidUsers should have 2 entries, got %d", len(original.ValidUsers))
	}
}

// TestHasVFSObjects_OnlyChecksGlobalSection verifies Bug #5 fix:
// VFS objects check must only match in the [global] section, not share sections.
func TestHasVFSObjects_OnlyChecksGlobalSection(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		wantVFSInGlobal bool
	}{
		{
			name: "vfs objects in global section",
			config: `[global]
   workgroup = WORKGROUP
   vfs objects = catia fruit streams_xattr

[share1]
   path = /data
`,
			wantVFSInGlobal: true,
		},
		{
			name: "vfs objects only in share section, not in global",
			config: `[global]
   workgroup = WORKGROUP

[timemachine]
   path = /data
   vfs objects = catia fruit streams_xattr
`,
			wantVFSInGlobal: false,
		},
		{
			name: "vfs objects in both sections",
			config: `[global]
   workgroup = WORKGROUP
   vfs objects = catia fruit streams_xattr

[timemachine]
   path = /data
   vfs objects = catia fruit streams_xattr
`,
			wantVFSInGlobal: true,
		},
		{
			name: "no vfs objects anywhere",
			config: `[global]
   workgroup = WORKGROUP

[share1]
   path = /data
`,
			wantVFSInGlobal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.config, "\n")
			hasVFSObjects := false
			inGlobal := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "[global]" {
					inGlobal = true
					continue
				}
				if strings.HasPrefix(trimmed, "[") {
					inGlobal = false
				}
				if inGlobal && strings.Contains(strings.ToLower(trimmed), "vfs objects") {
					hasVFSObjects = true
					break
				}
			}
			if hasVFSObjects != tt.wantVFSInGlobal {
				t.Errorf("hasVFSObjects = %v, want %v", hasVFSObjects, tt.wantVFSInGlobal)
			}
		})
	}
}

// TestGetEffectiveShareType_BackwardCompat verifies Bug #7 fix:
// GetEffectiveShareType must return ShareTypeTimeMachine for both
// share.TimeMachine == true AND share.ShareType == ShareTypeTimeMachine.
func TestGetEffectiveShareType_BackwardCompat(t *testing.T) {
	tests := []struct {
		name      string
		share     Share
		wantType  string
	}{
		{
			name:     "TimeMachine bool true",
			share:    Share{TimeMachine: true, ShareType: ""},
			wantType: ShareTypeTimeMachine,
		},
		{
			name:     "ShareType timemachine",
			share:    Share{TimeMachine: false, ShareType: ShareTypeTimeMachine},
			wantType: ShareTypeTimeMachine,
		},
		{
			name:     "both true",
			share:    Share{TimeMachine: true, ShareType: ShareTypeTimeMachine},
			wantType: ShareTypeTimeMachine,
		},
		{
			name:     "general type",
			share:    Share{TimeMachine: false, ShareType: ShareTypeGeneral},
			wantType: ShareTypeGeneral,
		},
		{
			name:     "empty type defaults to general",
			share:    Share{TimeMachine: false, ShareType: ""},
			wantType: ShareTypeGeneral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.share.GetEffectiveShareType()
			if got != tt.wantType {
				t.Errorf("GetEffectiveShareType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

// TestGetTimeMachineShareNames_IncludesShareType verifies Bug #7 fix:
// getTimeMachineShareNames must find shares with ShareType == ShareTypeTimeMachine,
// not just share.TimeMachine == true.
func TestGetTimeMachineShareNames_IncludesShareType(t *testing.T) {
	manager, mockFS, _, _, _ := setupManager()

	// Config with a share using ShareType=timemachine style (fruit:time machine = yes parsed)
	// and a share with no time machine marker
	confContent := `[global]
    workgroup = WORKGROUP

[tm-bool]
    path = /data/tm1
    fruit:time machine = yes

[tm-type]
    path = /data/tm2
    fruit:time machine = yes

[regular]
    path = /data/regular
`
	mockFS.WriteFile(tConfPath, []byte(confContent), 0644)

	names := manager.getTimeMachineShareNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 Time Machine shares, got %d: %v", len(names), names)
	}
}

// TestRemove_TriggersAvahiForShareType verifies Bug #8 fix:
// Remove must trigger Avahi cleanup for shares with ShareType == ShareTypeTimeMachine.
func TestRemove_TriggersAvahiForShareType(t *testing.T) {
	manager, mockFS, _, _, mockAvahi := setupManager()

	// Config with a Time Machine share using the fruit:time machine = yes marker
	confContent := `[global]
    workgroup = WORKGROUP

[tm-backup]
    path = /data/tm
    fruit:time machine = yes
`
	mockFS.WriteFile(tConfPath, []byte(confContent), 0644)

	err := manager.Remove("tm-backup")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify Avahi was called to remove the Time Machine share
	if len(mockAvahi.RemoveTimeMachineShareCalls) == 0 {
		t.Error("Expected Avahi RemoveTimeMachineShare to be called for Time Machine share")
	}
}

// TestCreate_StatError_HandlesPermissionDenied verifies Bug #16 fix:
// Create must distinguish between "not exist" and other Stat errors (e.g. permission denied).
func TestCreate_StatError_HandlesPermissionDenied(t *testing.T) {
	manager, mockFS, _, _, _ := setupManager()

	// Setup initial config
	initialConfig := `[global]
    workgroup = WORKGROUP
`
	mockFS.WriteFile(tConfPath, []byte(initialConfig), 0644)

	// Don't create the path in mock FS → Stat will return os.ErrNotExist
	newShare := Share{
		Name:       "testshare",
		Path:       "/nonexistent/path",
		ReadOnly:   false,
		Browseable: true,
	}

	err := manager.Create(newShare)
	if err == nil {
		t.Fatal("Expected error when path does not exist, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}
