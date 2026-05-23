package confirm

import (
	"strings"
	"testing"
)

func TestActionWithReader_SkipConfirmReturnsTrue(t *testing.T) {
	// When skipConfirm is true, should return true regardless of reader content
	reader := strings.NewReader("")
	result := ActionWithReader("Delete everything?", true, reader)
	if !result {
		t.Error("ActionWithReader with skipConfirm=true should return true")
	}
}

func TestActionWithReader_YesInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"lowercase y", "y\n", true},
		{"lowercase yes", "yes\n", true},
		{"uppercase Y", "Y\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed case Yes", "Yes\n", true},
		{"y with whitespace", "  y  \n", true},
		{"yes with whitespace", "  yes  \n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			got := ActionWithReader("Confirm?", false, reader)
			if got != tt.want {
				t.Errorf("ActionWithReader(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestActionWithReader_NoInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"lowercase n", "n\n", false},
		{"lowercase no", "no\n", false},
		{"empty line", "\n", false},
		{"random text", "maybe\n", false},
		{"just spaces", "   \n", false},
		{"ye (partial)", "ye\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			got := ActionWithReader("Confirm?", false, reader)
			if got != tt.want {
				t.Errorf("ActionWithReader(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestActionWithReader_EmptyReader(t *testing.T) {
	// An empty reader (no newline) should cause ReadString to return an error
	reader := strings.NewReader("")
	result := ActionWithReader("Confirm?", false, reader)
	if result {
		t.Error("ActionWithReader with empty reader should return false")
	}
}

func TestDangerousWithReader_SkipConfirmReturnsTrue(t *testing.T) {
	reader := strings.NewReader("")
	result := DangerousWithReader("Destroy data?", true, reader)
	if !result {
		t.Error("DangerousWithReader with skipConfirm=true should return true")
	}
}

func TestDangerousWithReader_ConfirmBehavior(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"y confirms", "y\n", true},
		{"yes confirms", "yes\n", true},
		{"n denies", "n\n", false},
		{"empty denies", "\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			got := DangerousWithReader("Destroy data?", false, reader)
			if got != tt.want {
				t.Errorf("DangerousWithReader(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestActionWithReader_NoNewlineInInput(t *testing.T) {
	// Input without newline - ReadString('\n') will read until EOF and return io.EOF error
	// but the data read should still be "y", however since err != nil, it returns false
	reader := strings.NewReader("y")
	result := ActionWithReader("Confirm?", false, reader)
	// This should return false because ReadString returns EOF error
	// Even though "y" was read, err is not nil
	if result {
		t.Error("ActionWithReader with no newline should return false due to io.EOF")
	}
}
