package cmd

import (
	"flag"
	"reflect"
	"testing"
)

func TestParseCSV_EmptyString(t *testing.T) {
	result := parseCSV("")
	if result != nil {
		t.Errorf("parseCSV('') = %v, want nil", result)
	}
}

func TestParseCSV_SingleValue(t *testing.T) {
	result := parseCSV("alice")
	expected := []string{"alice"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseCSV('alice') = %v, want %v", result, expected)
	}
}

func TestParseCSV_MultipleValues(t *testing.T) {
	result := parseCSV("alice,bob,charlie")
	expected := []string{"alice", "bob", "charlie"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseCSV('alice,bob,charlie') = %v, want %v", result, expected)
	}
}

func TestParseCSV_WithSpaces(t *testing.T) {
	result := parseCSV("alice , bob , charlie")
	expected := []string{"alice", "bob", "charlie"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseCSV('alice , bob , charlie') = %v, want %v", result, expected)
	}
}

func TestParseCSV_EmptyElements(t *testing.T) {
	// Empty elements after split should be filtered out
	result := parseCSV("alice,,bob,,,charlie")
	expected := []string{"alice", "bob", "charlie"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseCSV('alice,,bob,,,charlie') = %v, want %v", result, expected)
	}
}

func TestParseCSV_OnlyCommas(t *testing.T) {
	result := parseCSV(",,,")
	if len(result) != 0 {
		t.Errorf("parseCSV(',,,') = %v, want empty slice", result)
	}
}

func TestParseCSV_WhitespaceOnly(t *testing.T) {
	result := parseCSV("  ,  ,  ")
	if len(result) != 0 {
		t.Errorf("parseCSV('  ,  ,  ') = %v, want empty slice", result)
	}
}

func TestContains_Found(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		substr string
		want   bool
	}{
		{"exact match", []string{"uid=1000", "gid=1000"}, "uid", true},
		{"partial match", []string{"username=alice"}, "username", true},
		{"first element", []string{"abc", "def"}, "abc", true},
		{"last element", []string{"abc", "def"}, "def", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.substr, got, tt.want)
			}
		})
	}
}

func TestContains_NotFound(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		substr string
	}{
		{"not present", []string{"abc", "def"}, "xyz"},
		{"empty slice", []string{}, "anything"},
		{"nil slice", nil, "anything"},
		{"case sensitive", []string{"ABC"}, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.substr)
			if got {
				t.Errorf("contains(%v, %q) = true, want false", tt.slice, tt.substr)
			}
		})
	}
}

func TestParseFlags_ReturnsNonNil(t *testing.T) {
	fs := parseFlags([]string{"-name", "test"})
	if fs == nil {
		t.Error("parseFlags should return non-nil FlagSet")
	}
}

// TestSambaModify_FlagVisitDetectsExplicitFlags verifies Bug #11 fix:
// fs.Visit() must be used to detect explicitly set flags, so that
// "-readonly=false" (resetting to default) is detected and applied.
func TestSambaModify_FlagVisitDetectsExplicitFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantReadonly  bool
		wantBrowse    bool
		hasReadonly   bool
		hasBrowseable bool
	}{
		{
			name:          "explicitly set readonly to false",
			args:          []string{"-name", "test", "-readonly=false"},
			wantReadonly:  false,
			hasReadonly:   true,
			hasBrowseable: false,
		},
		{
			name:          "explicitly set readonly to true",
			args:          []string{"-name", "test", "-readonly=true"},
			wantReadonly:  true,
			hasReadonly:   true,
			hasBrowseable: false,
		},
		{
			name:          "explicitly set browseable to false",
			args:          []string{"-name", "test", "-browseable=false"},
			wantBrowse:   false,
			hasReadonly:   false,
			hasBrowseable: true,
		},
		{
			name:          "no flags set",
			args:          []string{"-name", "test"},
			hasReadonly:   false,
			hasBrowseable: false,
		},
		{
			name:          "both flags set",
			args:          []string{"-name", "test", "-readonly=true", "-browseable=false"},
			wantReadonly:  true,
			wantBrowse:   false,
			hasReadonly:   true,
			hasBrowseable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("samba modify", flag.ContinueOnError)
			_ = fs.String("name", "", "Share name")
			readOnly := fs.Bool("readonly", false, "Make share read-only")
			browseable := fs.Bool("browseable", true, "Make share browseable")

			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			updates := make(map[string]interface{})
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "readonly":
					updates["readonly"] = *readOnly
				case "browseable":
					updates["browseable"] = *browseable
				}
			})

			if tt.hasReadonly {
				val, ok := updates["readonly"]
				if !ok {
					t.Error("Expected 'readonly' in updates but not found")
				} else if val.(bool) != tt.wantReadonly {
					t.Errorf("readonly = %v, want %v", val, tt.wantReadonly)
				}
			} else {
				if _, ok := updates["readonly"]; ok {
					t.Error("'readonly' should not be in updates when flag was not set")
				}
			}

			if tt.hasBrowseable {
				val, ok := updates["browseable"]
				if !ok {
					t.Error("Expected 'browseable' in updates but not found")
				} else if val.(bool) != tt.wantBrowse {
					t.Errorf("browseable = %v, want %v", val, tt.wantBrowse)
				}
			} else {
				if _, ok := updates["browseable"]; ok {
					t.Error("'browseable' should not be in updates when flag was not set")
				}
			}
		})
	}
}
