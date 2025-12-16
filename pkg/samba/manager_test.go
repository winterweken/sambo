package samba

import (
	"fmt"
	"strings"
	"testing"

	"sambo/pkg/user"
)

const tConfPath = "/etc/samba/smb.conf"

func setupManager() (*Manager, *MockFileSystem, *MockCommandExecutor, *MockPlatform, *MockAvahiManager) {
	fs := NewMockFileSystem()
	exec := NewMockCommandExecutor()
	platform := &MockPlatform{
		IsLinuxReturn:         true,
		SambaConfigPathReturn: tConfPath,
	}
	avahi := &MockAvahiManager{}
	return NewManager(fs, exec, platform, avahi), fs, exec, platform, avahi
}

func TestList(t *testing.T) {
	manager, mockFS, _, _, _ := setupManager()

	// Mock smb.conf content
	confContent := `
[global]
    workgroup = WORKGROUP

[share1]
    path = /path/to/share1
    comment = First Share
    read only = yes
    browseable = yes

[share2]
    path = /path/to/share2
    comment = Second Share
    valid users = user1, user2
    read only = no
`
	mockFS.WriteFile(tConfPath, []byte(confContent), 0644)

	shares, err := manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(shares) != 2 {
		t.Errorf("Expected 2 shares, got %d", len(shares))
	}

	// Verify Share 1
	if shares[0].Name != "share1" {
		t.Errorf("Expected share1 name, got %s", shares[0].Name)
	}
	if shares[0].Path != "/path/to/share1" {
		t.Errorf("Expected share1 path, got %s", shares[0].Path)
	}
	if !shares[0].ReadOnly {
		t.Error("Expected share1 to be read only")
	}

	// Verify Share 2
	if shares[1].Name != "share2" {
		t.Errorf("Expected share2 name, got %s", shares[1].Name)
	}
	if len(shares[1].ValidUsers) != 2 {
		t.Errorf("Expected 2 valid users for share2, got %d", len(shares[1].ValidUsers))
	}
}

func TestCreate(t *testing.T) {
	manager, mockFS, mockExec, _, _ := setupManager()

	// Setup initial config
	initialConfig := `[global]
    workgroup = WORKGROUP
`
	mockFS.WriteFile(tConfPath, []byte(initialConfig), 0644)
	// Create the directory for the new share in the mock FS so validation passes
	mockFS.MkdirAll("/new/path", 0755)

	newShare := Share{
		Name:        "newshare",
		Path:        "/new/path",
		Comment:     "New Share",
		ReadOnly:    false,
		Browseable:  true,
		ValidUsers:  []string{"alice", "bob"},
		TimeMachine: true, // Enable Time Machine to trigger permissions logic
	}

	// Mock user.GetSystemUID
	origGetUID := user.GetSystemUID
	defer func() { user.GetSystemUID = origGetUID }()
	user.GetSystemUID = func(name string) (int, error) {
		return 1000, nil
	}

	err := manager.Create(newShare)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify file content was updated
	content, _ := mockFS.ReadFile(tConfPath)
	contentStr := string(content)

	if !strings.Contains(contentStr, "[newshare]") {
		t.Error("Config does not contain [newshare]")
	}
	if !strings.Contains(contentStr, "path = /new/path") {
		t.Error("Config does not contain path")
	}
	if !strings.Contains(contentStr, "valid users = alice bob") {
		t.Errorf("Config does not contain valid users. Content:\n%s", contentStr)
	}

	// Verify restart command was called
	foundRestart := false
	for _, call := range mockExec.RunCalls {
		if call.Name == "systemctl" || call.Name == "service" || call.Name == "brew" {
			foundRestart = true
		}
	}
	if !foundRestart {
		t.Error("Expected Samba restart command but none was called")
	}
}

func TestRemove(t *testing.T) {
	manager, mockFS, _, _, _ := setupManager()

	config := `[global]
    workgroup = WORKGROUP

[toremove]
    path = /remove/me
`
	mockFS.WriteFile(tConfPath, []byte(config), 0644)

	err := manager.Remove("toremove")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	content, _ := mockFS.ReadFile(tConfPath)
	if strings.Contains(string(content), "[toremove]") {
		t.Error("Share [toremove] should have been removed")
	}
}

func TestInstall_MacOS(t *testing.T) {
	manager, _, mockExec, mockPlat, _ := setupManager()
	mockPlat.IsMacOSReturn = true
	mockPlat.IsLinuxReturn = false
	mockExec.MockLookPath = func(file string) (string, error) {
		if file == "brew" {
			return "/opt/homebrew/bin/brew", nil
		}
		return "", fmt.Errorf("not found") // Error
	}

	err := manager.installSamba()
	if err != nil {
		t.Fatalf("installSamba() error = %v", err)
	}

	// Verify brew install was called
	found := false
	for _, call := range mockExec.RunCalls {
		if call.Name == "brew" && len(call.Args) >= 2 && call.Args[0] == "install" && call.Args[1] == "samba" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'brew install samba' call on macOS")
	}
}

func TestInstall_Linux_Apt(t *testing.T) {
	manager, _, mockExec, mockPlat, _ := setupManager()
	mockPlat.IsMacOSReturn = false
	mockPlat.IsLinuxReturn = true
	mockExec.MockLookPath = func(file string) (string, error) {
		if file == "apt-get" {
			return "/usr/bin/apt-get", nil
		}
		return "", fmt.Errorf("not found")
	}

	err := manager.installSamba()
	if err != nil {
		t.Fatalf("installSamba() error = %v", err)
	}

	// Verify apt-get install was called
	found := false
	for _, call := range mockExec.RunCalls {
		if call.Name == "apt-get" && len(call.Args) >= 3 && call.Args[0] == "install" && call.Args[2] == "samba" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'apt-get install samba' call on Linux")
	}
}

func TestReload_MacOS(t *testing.T) {
	manager, _, mockExec, mockPlat, _ := setupManager()
	mockPlat.IsMacOSReturn = true

	err := manager.reloadSamba()
	if err != nil {
		t.Fatalf("reloadSamba() error = %v", err)
	}

	// Verify pkill call
	found := false
	for _, call := range mockExec.RunCalls {
		if call.Name == "pkill" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pkill call on macOS reload")
	}
}

func TestReload_Linux(t *testing.T) {
	manager, _, mockExec, mockPlat, _ := setupManager()
	mockPlat.IsLinuxReturn = true
	mockPlat.IsMacOSReturn = false

	err := manager.reloadSamba()
	if err != nil {
		t.Fatalf("reloadSamba() error = %v", err)
	}

	// Verify systemctl call
	found := false
	for _, call := range mockExec.RunCalls {
		if call.Name == "systemctl" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected systemctl call on Linux reload")
	}
}

func TestFixPermissions(t *testing.T) {
	manager, mockFS, _, _, _ := setupManager()

	// Mock user.GetSystemUID
	origGetUID := user.GetSystemUID
	defer func() { user.GetSystemUID = origGetUID }()
	user.GetSystemUID = func(name string) (int, error) {
		if name == "alice" {
			return 1001, nil
		}
		return 0, fmt.Errorf("user not found")
	}

	// Create dummy directory
	mockFS.MkdirAll("/data/tm", 0755)

	share := Share{
		Name:        "timemachine",
		Path:        "/data/tm",
		TimeMachine: true,
		ValidUsers:  []string{"alice"},
	}

	err := manager.FixPermissions(share)
	if err != nil {
		t.Fatalf("FixPermissions() error = %v", err)
	}

	// Verify Chown called
	chownFound := false
	for _, call := range mockFS.ChownCalls {
		if call.Path == "/data/tm" && call.Uid == 1001 && call.Gid == -1 {
			chownFound = true
			break
		}
	}
	if !chownFound {
		t.Error("Expected Chown(/data/tm, 1001, -1)")
	}

	// Verify Chmod called
	chmodFound := false
	for _, call := range mockFS.ChmodCalls {
		if call.Path == "/data/tm" && call.Perm == 0700 {
			chmodFound = true
			break
		}
	}
	if !chmodFound {
		t.Error("Expected Chmod(/data/tm, 0700)")
	}
}
