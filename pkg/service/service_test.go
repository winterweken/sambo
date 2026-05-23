package service

import (
	"testing"
)

func TestStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{"unknown", StatusUnknown, "unknown"},
		{"running", StatusRunning, "running"},
		{"stopped", StatusStopped, "stopped"},
		{"not installed", StatusNotInstalled, "not installed"},
		{"out of range", Status(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify iota ordering
	if StatusUnknown != 0 {
		t.Errorf("StatusUnknown = %d, want 0", StatusUnknown)
	}
	if StatusRunning != 1 {
		t.Errorf("StatusRunning = %d, want 1", StatusRunning)
	}
	if StatusStopped != 2 {
		t.Errorf("StatusStopped = %d, want 2", StatusStopped)
	}
	if StatusNotInstalled != 3 {
		t.Errorf("StatusNotInstalled = %d, want 3", StatusNotInstalled)
	}
}

func TestCheckSamba_ReturnsValidStatus(t *testing.T) {
	// On macOS CI, Samba may or may not be installed.
	// We just verify the function returns a valid status without panicking.
	status := CheckSamba()
	validStatuses := map[Status]bool{
		StatusUnknown:      true,
		StatusRunning:      true,
		StatusStopped:      true,
		StatusNotInstalled: true,
	}
	if !validStatuses[status] {
		t.Errorf("CheckSamba() returned invalid status: %d", status)
	}
}

func TestCheckNFS_ReturnsValidStatus(t *testing.T) {
	// On macOS, NFS may or may not be running.
	// We just verify the function returns a valid status without panicking.
	status := CheckNFS()
	validStatuses := map[Status]bool{
		StatusUnknown:      true,
		StatusRunning:      true,
		StatusStopped:      true,
		StatusNotInstalled: true,
	}
	if !validStatuses[status] {
		t.Errorf("CheckNFS() returned invalid status: %d", status)
	}
}

func TestEnsureSambaRunning_ReturnsErrorOrNil(t *testing.T) {
	// On macOS test environment, Samba is likely not running.
	// We verify the function doesn't panic and returns a sensible result.
	err := EnsureSambaRunning()
	status := CheckSamba()
	if status == StatusRunning && err != nil {
		t.Errorf("EnsureSambaRunning() returned error %v when Samba is running", err)
	}
	if status != StatusRunning && err == nil {
		t.Errorf("EnsureSambaRunning() returned nil when Samba status is %s", status)
	}
}

func TestEnsureNFSRunning_ReturnsErrorOrNil(t *testing.T) {
	// Similar to TestEnsureSambaRunning
	err := EnsureNFSRunning()
	status := CheckNFS()
	if status == StatusRunning && err != nil {
		t.Errorf("EnsureNFSRunning() returned error %v when NFS is running", err)
	}
	if status != StatusRunning && err == nil {
		t.Errorf("EnsureNFSRunning() returned nil when NFS status is %s", status)
	}
}

func TestWarnIfNotRunning_UnknownService(t *testing.T) {
	// WarnIfNotRunning with an unknown service name should return early
	// without panicking (no crash test)
	WarnIfNotRunning("unknown_service")
	WarnIfNotRunning("")
	WarnIfNotRunning("ftp")
	// If we get here without panic, the test passes
}

func TestWarnIfNotRunning_KnownServices(t *testing.T) {
	// Test with known service names - should not panic
	WarnIfNotRunning("samba")
	WarnIfNotRunning("nfs")
	// If we get here without panic, the test passes
}
