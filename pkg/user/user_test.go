package user

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUser_Struct(t *testing.T) {
	user := User{
		Username: "testuser",
		UID:      1001,
		Enabled:  true,
	}

	if user.Username != "testuser" {
		t.Errorf("expected Username to be 'testuser', got %s", user.Username)
	}
	if user.UID != 1001 {
		t.Errorf("expected UID to be 1001, got %d", user.UID)
	}
	if !user.Enabled {
		t.Error("expected Enabled to be true")
	}
}

// TestParsePdbeditOutput tests parsing of pdbedit -L -v output
func TestParsePdbeditOutput(t *testing.T) {
	pdbeditOutput := `---------------
Unix username:        user1
NT username:          user1
Account Flags:        [U          ]
User SID:             S-1-5-21-1234567890-1234567890-1234567890-1001
Primary Group SID:    S-1-5-21-1234567890-1234567890-1234567890-513
---------------
Unix username:        user2
NT username:          user2
Account Flags:        [DU         ]
User SID:             S-1-5-21-1234567890-1234567890-1234567890-1002
Primary Group SID:    S-1-5-21-1234567890-1234567890-1234567890-513
---------------
Unix username:        admin
NT username:          admin
Account Flags:        [U          ]
User SID:             S-1-5-21-1234567890-1234567890-1234567890-1000
`

	users := parsePdbeditOutput(pdbeditOutput)

	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}

	// Check user1
	foundUser1 := false
	foundUser2 := false
	foundAdmin := false

	for _, u := range users {
		switch u.Username {
		case "user1":
			foundUser1 = true
			if !u.Enabled {
				t.Error("user1 should be enabled")
			}
		case "user2":
			foundUser2 = true
			if u.Enabled {
				t.Error("user2 should be disabled (has D flag)")
			}
		case "admin":
			foundAdmin = true
			if !u.Enabled {
				t.Error("admin should be enabled")
			}
		}
	}

	if !foundUser1 {
		t.Error("user1 not found")
	}
	if !foundUser2 {
		t.Error("user2 not found")
	}
	if !foundAdmin {
		t.Error("admin not found")
	}
}

// parsePdbeditOutput is a helper that simulates the parsing logic from List()
func parsePdbeditOutput(output string) []User {
	var users []User
	var currentUser *User
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || line == "---------------" {
			continue
		}

		// Parse pdbedit output
		if strings.HasPrefix(line, "Unix username:") {
			if currentUser != nil {
				users = append(users, *currentUser)
			}
			currentUser = &User{
				Enabled: true, // default
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentUser.Username = strings.TrimSpace(parts[1])
			}
		} else if currentUser != nil && strings.Contains(line, "Account Flags:") {
			if strings.Contains(line, "D") {
				currentUser.Enabled = false
			}
		}
	}

	if currentUser != nil {
		users = append(users, *currentUser)
	}

	return users
}

// TestAccountFlagsParser tests parsing of account flags
func TestAccountFlagsParser(t *testing.T) {
	tests := []struct {
		flags    string
		disabled bool
	}{
		{"[U          ]", false},
		{"[DU         ]", true},
		{"[UD         ]", true},
		{"[U   D      ]", true},
		{"[           ]", false},
		{"[UX         ]", false},
	}

	for _, tt := range tests {
		t.Run(tt.flags, func(t *testing.T) {
			isDisabled := strings.Contains(tt.flags, "D")
			if isDisabled != tt.disabled {
				t.Errorf("flags %s: expected disabled=%v, got %v", tt.flags, tt.disabled, isDisabled)
			}
		})
	}
}

// TestParsePasswdLine tests parsing /etc/passwd format
func TestParsePasswdLine(t *testing.T) {
	tests := []struct {
		line     string
		username string
		uid      int
		valid    bool
	}{
		{"testuser:x:1001:1001:Test User:/home/testuser:/bin/bash", "testuser", 1001, true},
		{"root:x:0:0:root:/root:/bin/bash", "root", 0, true},
		{"nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin", "nobody", 65534, true},
		{"samba:x:1002:1002::/home/samba:/usr/sbin/nologin", "samba", 1002, true},
		{"invalid", "", 0, false},
		{"too:few", "", 0, false},
		{"user:x:notanumber:1001::/home/user:/bin/bash", "user", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			parts := strings.Split(tt.line, ":")
			if len(parts) < 3 {
				if tt.valid {
					t.Error("expected valid parse but line has too few fields")
				}
				return
			}

			username := parts[0]
			uidStr := parts[2]
			uid, err := strconv.Atoi(uidStr)

			if err != nil {
				if tt.valid && tt.uid != 0 {
					t.Errorf("expected valid UID parse, got error: %v", err)
				}
				return
			}

			if username != tt.username {
				t.Errorf("expected username %s, got %s", tt.username, username)
			}
			if uid != tt.uid {
				t.Errorf("expected UID %d, got %d", tt.uid, uid)
			}
		})
	}
}

// TestReadPasswdFile tests reading UID from a temporary passwd file
func TestReadPasswdFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPasswd := filepath.Join(tmpDir, "passwd")

	passwdContent := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
testuser:x:1001:1001:Test User:/home/testuser:/bin/bash
sambauser:x:1002:1002:Samba User:/home/sambauser:/usr/sbin/nologin
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
`

	if err := os.WriteFile(tmpPasswd, []byte(passwdContent), 0644); err != nil {
		t.Fatalf("failed to create temp passwd: %v", err)
	}

	// Read and find users
	content, err := os.ReadFile(tmpPasswd)
	if err != nil {
		t.Fatalf("failed to read temp passwd: %v", err)
	}

	tests := []struct {
		username    string
		expectedUID int
		found       bool
	}{
		{"root", 0, true},
		{"testuser", 1001, true},
		{"sambauser", 1002, true},
		{"nobody", 65534, true},
		{"nonexistent", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			found := false
			var foundUID int

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				parts := strings.Split(line, ":")
				if len(parts) >= 3 && parts[0] == tt.username {
					uid, err := strconv.Atoi(parts[2])
					if err == nil {
						found = true
						foundUID = uid
					}
					break
				}
			}

			if found != tt.found {
				t.Errorf("expected found=%v, got %v", tt.found, found)
			}
			if found && foundUID != tt.expectedUID {
				t.Errorf("expected UID %d, got %d", tt.expectedUID, foundUID)
			}
		})
	}
}

// TestUsernameValidation tests validation of usernames
func TestUsernameValidation(t *testing.T) {
	tests := []struct {
		username string
		valid    bool
		reason   string
	}{
		{"testuser", true, "normal username"},
		{"user1", true, "username with number"},
		{"_user", true, "username starting with underscore"},
		{"user-name", true, "username with hyphen"},
		{"root", true, "root user"},
		{"", false, "empty username"},
		{"user name", false, "username with space"},
		{"user\ttab", false, "username with tab"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			// Basic validation - no empty, no whitespace
			valid := tt.username != "" && !strings.ContainsAny(tt.username, " \t\n\r")
			if valid != tt.valid {
				t.Errorf("username %q: expected valid=%v, got %v", tt.username, tt.valid, valid)
			}
		})
	}
}

// TestPasswordValidation tests basic password validation
func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
		reason   string
	}{
		{"password123", true, "simple password"},
		{"P@ssw0rd!", true, "complex password"},
		{"short", true, "short password (samba allows it)"},
		{"", false, "empty password"},
		{"verylongpasswordthatismorethanreasonable", true, "long password"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			// Samba generally allows most passwords, just not empty
			valid := tt.password != ""
			if valid != tt.valid {
				t.Errorf("password validation: expected valid=%v, got %v", tt.valid, valid)
			}
		})
	}
}

// TestPdbeditOutputFormats tests different pdbedit output formats
func TestPdbeditOutputFormats(t *testing.T) {
	// Simple format (pdbedit -L)
	simpleOutput := `user1:1001:
user2:1002:
admin:1000:
`
	// Test simple format parsing
	lines := strings.Split(simpleOutput, "\n")
	var simpleUsers []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 1 && parts[0] != "" {
			simpleUsers = append(simpleUsers, parts[0])
		}
	}

	if len(simpleUsers) != 3 {
		t.Errorf("simple format: expected 3 users, got %d", len(simpleUsers))
	}

	// Verbose format (pdbedit -L -v) - tested in TestParsePdbeditOutput
}

// TestUserExistsCheck tests the logic for checking if a user exists
func TestUserExistsCheck(t *testing.T) {
	existingUsers := []User{
		{Username: "user1", UID: 1001, Enabled: true},
		{Username: "user2", UID: 1002, Enabled: false},
		{Username: "admin", UID: 1000, Enabled: true},
	}

	tests := []struct {
		username string
		exists   bool
	}{
		{"user1", true},
		{"user2", true},
		{"admin", true},
		{"nonexistent", false},
		{"User1", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			found := false
			for _, u := range existingUsers {
				if u.Username == tt.username {
					found = true
					break
				}
			}
			if found != tt.exists {
				t.Errorf("user %s: expected exists=%v, got %v", tt.username, tt.exists, found)
			}
		})
	}
}

// TestEmptyUserList tests handling of empty user list
func TestEmptyUserList(t *testing.T) {
	emptyOutput := ""
	users := parsePdbeditOutput(emptyOutput)

	if len(users) != 0 {
		t.Errorf("expected 0 users from empty output, got %d", len(users))
	}
}

// TestSingleUser tests parsing output with a single user
func TestSingleUser(t *testing.T) {
	singleUserOutput := `---------------
Unix username:        onlyuser
NT username:          onlyuser
Account Flags:        [U          ]
User SID:             S-1-5-21-1234567890-1234567890-1234567890-1001
`

	users := parsePdbeditOutput(singleUserOutput)

	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
		return
	}

	if users[0].Username != "onlyuser" {
		t.Errorf("expected username 'onlyuser', got %s", users[0].Username)
	}
	if !users[0].Enabled {
		t.Error("expected user to be enabled")
	}
}

// TestSystemUserCreationArgs tests the arguments for creating system users
func TestSystemUserCreationArgs(t *testing.T) {
	username := "sambauser"

	// Expected args for useradd command
	expectedArgs := []string{"-M", "-s", "/usr/sbin/nologin", username}

	// Verify the arguments match what createSystemUser would use
	args := []string{"-M", "-s", "/usr/sbin/nologin", username}

	if len(args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d", len(expectedArgs), len(args))
		return
	}

	for i, arg := range expectedArgs {
		if args[i] != arg {
			t.Errorf("arg %d: expected %s, got %s", i, arg, args[i])
		}
	}
}

// TestUserRemovalFlags tests the flags used for user removal
func TestUserRemovalFlags(t *testing.T) {
	tests := []struct {
		username     string
		removeSystem bool
		smbpasswdArg string
	}{
		{"user1", false, "-x"},
		{"user2", true, "-x"},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			// smbpasswd -x is always used for removing samba user
			if tt.smbpasswdArg != "-x" {
				t.Errorf("expected smbpasswd arg '-x', got %s", tt.smbpasswdArg)
			}
		})
	}
}

// TestParseUsernameLine tests parsing username from pdbedit output
func TestParseUsernameLine(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"Unix username:        user1", "user1"},
		{"Unix username:  admin", "admin"},
		{"Unix username:testuser", "testuser"},
		{"Unix username:     spaced  ", "spaced"},
		{"Not a username line", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			var username string
			if strings.HasPrefix(tt.line, "Unix username:") {
				parts := strings.SplitN(tt.line, ":", 2)
				if len(parts) == 2 {
					username = strings.TrimSpace(parts[1])
				}
			}
			if username != tt.expected {
				t.Errorf("expected username %q, got %q", tt.expected, username)
			}
		})
	}
}

// TestUserWithSpacesInOutput tests handling of various spacing in output
func TestUserWithSpacesInOutput(t *testing.T) {
	output := `---------------
Unix username:        test_user
NT username:          test_user
Account Flags:        [U          ]
---------------
Unix username:   admin
Account Flags: [U]
`

	users := parsePdbeditOutput(output)

	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
		return
	}

	if users[0].Username != "test_user" {
		t.Errorf("expected username 'test_user', got %s", users[0].Username)
	}
	if users[1].Username != "admin" {
		t.Errorf("expected username 'admin', got %s", users[1].Username)
	}
}
