package avahi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sambo/pkg/platform"
)

const (
	avahiServicesDir = "/etc/avahi/services"
	sambaServiceFile = "samba.service"
)

// TimeMachineShare represents a Time Machine share for Avahi advertisement
type TimeMachineShare struct {
	Name string
}

// IsAvahiAvailable checks if Avahi is available on the system
func IsAvahiAvailable() bool {
	if platform.IsMacOS() {
		// macOS uses built-in Bonjour, no Avahi needed
		return false
	}
	_, err := exec.LookPath("avahi-daemon")
	return err == nil
}

// GetTimeMachineShares scans the list of share names and returns them as TimeMachineShare objects
func GetTimeMachineShares(shareNames []string) []TimeMachineShare {
	shares := make([]TimeMachineShare, len(shareNames))
	for i, name := range shareNames {
		shares[i] = TimeMachineShare{Name: name}
	}
	return shares
}

// UpdateTimeMachineService updates the Avahi service file to advertise Time Machine shares
// If no shares are provided, it removes the service file
func UpdateTimeMachineService(shares []TimeMachineShare) error {
	if !IsAvahiAvailable() {
		// Avahi not available, skip silently
		return nil
	}

	serviceFile := filepath.Join(avahiServicesDir, sambaServiceFile)

	// If no Time Machine shares, remove the service file
	if len(shares) == 0 {
		if _, err := os.Stat(serviceFile); err == nil {
			if err := os.Remove(serviceFile); err != nil {
				return fmt.Errorf("failed to remove Avahi service file: %w", err)
			}
		}
		return nil
	}

	// Check if Avahi services directory exists
	if _, err := os.Stat(avahiServicesDir); os.IsNotExist(err) {
		return fmt.Errorf("Avahi services directory does not exist: %s (is avahi-daemon installed?)", avahiServicesDir)
	}

	// Generate the service file content
	content := generateServiceFile(shares)

	// Write the service file
	if err := os.WriteFile(serviceFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write Avahi service file: %w", err)
	}

	return nil
}

// generateServiceFile creates the XML content for the Avahi service file
func generateServiceFile(shares []TimeMachineShare) string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name replace-wildcards="yes">%h</name>
  <service>
    <type>_smb._tcp</type>
    <port>445</port>
  </service>
  <service>
    <type>_device-info._tcp</type>
    <port>0</port>
    <txt-record>model=TimeCapsule8,119</txt-record>
  </service>
  <service>
    <type>_adisk._tcp</type>
    <port>9</port>
    <txt-record>sys=waMa=0,adVF=0x100</txt-record>
`)

	// Add each Time Machine share as a disk entry
	for i, share := range shares {
		// adVF=0x82 flags: 0x02 = Time Machine, 0x80 = mounted
		sb.WriteString(fmt.Sprintf("    <txt-record>dk%d=adVN=%s,adVF=0x82</txt-record>\n", i, share.Name))
	}

	sb.WriteString(`  </service>
</service-group>
`)

	return sb.String()
}

// AddTimeMachineShare adds a single Time Machine share to the Avahi advertisement
// It reads existing shares and adds the new one
func AddTimeMachineShare(shareName string, existingTimeMachineShares []string) error {
	// Build the complete list of shares
	allShares := make([]TimeMachineShare, 0, len(existingTimeMachineShares)+1)

	// Add existing shares
	for _, name := range existingTimeMachineShares {
		if name != shareName { // Avoid duplicates
			allShares = append(allShares, TimeMachineShare{Name: name})
		}
	}

	// Add the new share
	allShares = append(allShares, TimeMachineShare{Name: shareName})

	return UpdateTimeMachineService(allShares)
}

// RemoveTimeMachineShare removes a Time Machine share from the Avahi advertisement
func RemoveTimeMachineShare(shareName string, remainingTimeMachineShares []string) error {
	// Filter out the removed share
	shares := make([]TimeMachineShare, 0, len(remainingTimeMachineShares))
	for _, name := range remainingTimeMachineShares {
		if name != shareName {
			shares = append(shares, TimeMachineShare{Name: name})
		}
	}

	return UpdateTimeMachineService(shares)
}
