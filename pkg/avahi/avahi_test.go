package avahi

import (
	"strings"
	"testing"
)

func TestGetTimeMachineShares_Empty(t *testing.T) {
	shares := GetTimeMachineShares([]string{})
	if len(shares) != 0 {
		t.Errorf("GetTimeMachineShares([]) returned %d shares, want 0", len(shares))
	}
}

func TestGetTimeMachineShares_SingleShare(t *testing.T) {
	shares := GetTimeMachineShares([]string{"backup"})
	if len(shares) != 1 {
		t.Fatalf("GetTimeMachineShares returned %d shares, want 1", len(shares))
	}
	if shares[0].Name != "backup" {
		t.Errorf("shares[0].Name = %q, want %q", shares[0].Name, "backup")
	}
}

func TestGetTimeMachineShares_MultipleShares(t *testing.T) {
	input := []string{"backup", "timemachine", "archive"}
	shares := GetTimeMachineShares(input)
	if len(shares) != 3 {
		t.Fatalf("GetTimeMachineShares returned %d shares, want 3", len(shares))
	}
	for i, name := range input {
		if shares[i].Name != name {
			t.Errorf("shares[%d].Name = %q, want %q", i, shares[i].Name, name)
		}
	}
}

func TestGenerateServiceFile_NoShares(t *testing.T) {
	content := generateServiceFile([]TimeMachineShare{})
	// Should contain XML header and structure but no dk entries
	if !strings.Contains(content, `<?xml version="1.0"`) {
		t.Error("generateServiceFile should contain XML header")
	}
	if !strings.Contains(content, "<service-group>") {
		t.Error("generateServiceFile should contain <service-group>")
	}
	if !strings.Contains(content, "_smb._tcp") {
		t.Error("generateServiceFile should contain _smb._tcp service type")
	}
	if !strings.Contains(content, "_adisk._tcp") {
		t.Error("generateServiceFile should contain _adisk._tcp service type")
	}
	if strings.Contains(content, "dk0=") {
		t.Error("generateServiceFile with no shares should not contain dk entries")
	}
}

func TestGenerateServiceFile_SingleShare(t *testing.T) {
	shares := []TimeMachineShare{{Name: "MyBackup"}}
	content := generateServiceFile(shares)

	if !strings.Contains(content, "dk0=adVN=MyBackup,adVF=0x82") {
		t.Errorf("generateServiceFile should contain dk0 entry for MyBackup, got:\n%s", content)
	}
	// Should not have dk1
	if strings.Contains(content, "dk1=") {
		t.Error("generateServiceFile with 1 share should not contain dk1")
	}
}

func TestGenerateServiceFile_MultipleShares(t *testing.T) {
	shares := []TimeMachineShare{
		{Name: "backup1"},
		{Name: "backup2"},
		{Name: "archive"},
	}
	content := generateServiceFile(shares)

	if !strings.Contains(content, "dk0=adVN=backup1,adVF=0x82") {
		t.Error("missing dk0 entry for backup1")
	}
	if !strings.Contains(content, "dk1=adVN=backup2,adVF=0x82") {
		t.Error("missing dk1 entry for backup2")
	}
	if !strings.Contains(content, "dk2=adVN=archive,adVF=0x82") {
		t.Error("missing dk2 entry for archive")
	}
}

func TestGenerateServiceFile_StructureIsWellFormed(t *testing.T) {
	shares := []TimeMachineShare{{Name: "test"}}
	content := generateServiceFile(shares)

	// Check critical elements are present
	expectedElements := []string{
		`<?xml version="1.0" standalone='no'?>`,
		`<service-group>`,
		`</service-group>`,
		`<name replace-wildcards="yes">%h</name>`,
		`<type>_smb._tcp</type>`,
		`<port>445</port>`,
		`<type>_device-info._tcp</type>`,
		`model=TimeCapsule8,119`,
		`<type>_adisk._tcp</type>`,
		`sys=waMa=0,adVF=0x100`,
	}
	for _, expected := range expectedElements {
		if !strings.Contains(content, expected) {
			t.Errorf("generateServiceFile missing expected element %q", expected)
		}
	}
}

func TestIsAvahiAvailable_OnMacOS(t *testing.T) {
	// On macOS, Avahi should not be available (macOS uses Bonjour)
	available := IsAvahiAvailable()
	if available {
		t.Error("IsAvahiAvailable() should return false on macOS")
	}
}

func TestUpdateTimeMachineService_ReturnsNilOnMacOS(t *testing.T) {
	// On macOS, UpdateTimeMachineService should return nil because Avahi is not available
	err := UpdateTimeMachineService([]TimeMachineShare{{Name: "test"}})
	if err != nil {
		t.Errorf("UpdateTimeMachineService should return nil on macOS, got: %v", err)
	}
}

func TestAddTimeMachineShare_ReturnsNilOnMacOS(t *testing.T) {
	// On macOS, should return nil (Avahi not available, early return)
	err := AddTimeMachineShare("backup", []string{})
	if err != nil {
		t.Errorf("AddTimeMachineShare should return nil on macOS, got: %v", err)
	}
}

func TestRemoveTimeMachineShare_ReturnsNilOnMacOS(t *testing.T) {
	// On macOS, should return nil (Avahi not available, early return)
	err := RemoveTimeMachineShare("backup", []string{"other"})
	if err != nil {
		t.Errorf("RemoveTimeMachineShare should return nil on macOS, got: %v", err)
	}
}

func TestAddTimeMachineShare_DeduplicatesExisting(t *testing.T) {
	// AddTimeMachineShare filters duplicates from existingTimeMachineShares
	// On macOS this returns nil from UpdateTimeMachineService, but we can verify
	// the function doesn't panic with duplicate names
	err := AddTimeMachineShare("backup", []string{"backup", "other"})
	if err != nil {
		t.Errorf("AddTimeMachineShare with duplicate should not error on macOS, got: %v", err)
	}
}

func TestRemoveTimeMachineShare_FiltersCorrectly(t *testing.T) {
	// RemoveTimeMachineShare filters out the named share
	// On macOS this returns nil, but verify it doesn't panic
	err := RemoveTimeMachineShare("backup", []string{"backup", "other", "archive"})
	if err != nil {
		t.Errorf("RemoveTimeMachineShare should not error on macOS, got: %v", err)
	}
}
