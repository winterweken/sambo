package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Braille character reference:
// Unicode block U+2800–U+28FF encodes 8-dot Braille patterns.
// Each character is a 2×4 grid of dots.
//
// Standard Braille dot numbering (ISO/TR 11548-1):
//   Dot 1 (0x01)  Dot 4 (0x08)
//   Dot 2 (0x02)  Dot 5 (0x10)
//   Dot 3 (0x04)  Dot 6 (0x20)
//   Dot 7 (0x40)  Dot 8 (0x80)
//
// Braille char = 0x2800 + (dot bits)
// Full block ⣿ = 0x28FF, empty ⠀ = 0x2800

const (
	brailleBase = 0x2800
	brailleFull = 0x28FF
)

// --- Braille Sparkline ---
// Renders a horizontal sparkline using Braille vertical bar heights.
// Values are normalized to 0.0–1.0 range.
// Each Braille character encodes two columns (left/right dots).

// brailleBarLeft maps height 0–4 to left-column dot patterns
var brailleBarLeft = [5]rune{
	0x2800,        // 0 dots: ⠀
	0x2800 | 0x40, // 1 dot (bottom): ⡀
	0x2800 | 0x44, // 2 dots: ⡄
	0x2800 | 0x46, // 3 dots: ⡆
	0x2800 | 0x47, // 4 dots (full left): ⡇
}

// brailleBarRight maps height 0–4 to right-column dot patterns
var brailleBarRight = [5]rune{
	0x2800,        // 0 dots: ⠀
	0x2800 | 0x80, // 1 dot (bottom): ⢀
	0x2800 | 0xA0, // 2 dots: ⢠
	0x2800 | 0xB0, // 3 dots: ⢰
	0x2800 | 0xB8, // 4 dots (full right): ⢸
}

// BrailleSparkline renders a series of values as a compact sparkline.
// Each Braille character encodes two data points (left and right columns).
// Values should be in 0.0–1.0 range; they're clamped automatically.
func BrailleSparkline(values []float64, width int) string {
	if len(values) == 0 {
		return strings.Repeat("⠀", width)
	}

	// Resample values to fit exactly width*2 data points (2 columns per char)
	targetPoints := width * 2
	resampled := resample(values, targetPoints)

	var sb strings.Builder
	for i := 0; i < len(resampled)-1; i += 2 {
		left := clamp(resampled[i], 0, 1)
		right := clamp(resampled[i+1], 0, 1)

		// Map 0.0–1.0 to 0–4 dot height
		lh := int(math.Round(left * 4))
		rh := int(math.Round(right * 4))

		// Combine left and right dot patterns
		ch := rune(int(brailleBarLeft[lh]) | int(brailleBarRight[rh]))
		sb.WriteRune(ch)
	}

	// Handle odd number of points
	if len(resampled)%2 == 1 {
		left := clamp(resampled[len(resampled)-1], 0, 1)
		lh := int(math.Round(left * 4))
		sb.WriteRune(brailleBarLeft[lh])
	}

	return sb.String()
}

// --- Braille Status Bar ---
// Visual density indicator using graduated Braille fill.

// BrailleStatusBar creates a density bar showing a ratio from 0.0 to 1.0
// using graduated Braille characters from sparse to dense.
func BrailleStatusBar(ratio float64, width int) string {
	ratio = clamp(ratio, 0, 1)
	filled := int(math.Round(ratio * float64(width)))
	empty := width - filled

	// Dense fill character and empty character
	fill := string(rune(brailleFull)) // ⣿
	gap := "⠀"                        // empty Braille space

	return strings.Repeat(fill, filled) + strings.Repeat(gap, empty)
}

// --- Braille Wave Divider ---
// Decorative section dividers using sine-wave Braille patterns.

// BrailleWaveDivider creates an abstract wave pattern divider.
// The wave uses a sine function to modulate dot density across the width.
func BrailleWaveDivider(width int) string {
	var sb strings.Builder
	for i := 0; i < width; i++ {
		// Two overlapping sine waves for visual interest
		t := float64(i) / float64(width) * math.Pi * 4
		v1 := (math.Sin(t) + 1) / 2          // Primary wave 0–1
		v2 := (math.Sin(t*1.7+0.5) + 1) / 2  // Secondary wave, phase-shifted

		// Blend the two waves
		blend := (v1*0.6 + v2*0.4)

		// Map to a subset of Braille characters by density
		ch := densityBraille(blend)
		sb.WriteRune(ch)
	}
	return sb.String()
}

// --- Braille Service Pulse ---
// Animated-style pulse indicator for service health.

// BrailleServiceIndicator creates a compact service health indicator.
// active: whether the service is running
// label: service name to display
func BrailleServiceIndicator(active bool, label string) string {
	if active {
		// Dense pulsing pattern: ⣿⣿⣿⣿
		bar := "⣿⣷⣯⣟⣿"
		return lipgloss.NewStyle().
			Foreground(successColor).
			Render(bar+" "+label)
	}
	// Sparse dim pattern: ⡀⠄⠂⠁⠀
	bar := "⠀⠁⠀⠁⠀"
	return lipgloss.NewStyle().
		Foreground(mutedColor).
		Render(bar+" "+label)
}

// --- Braille Banner ---
// Abstract network topology art for the main menu header.

// BrailleBanner generates the decorative header art for the main menu.
// It creates an abstract representation of networked nodes using Braille.
func BrailleBanner() string {
	// Abstract "network topology" art using Braille characters.
	// Represents interconnected shares/nodes — deliberately abstract
	// but evocative of network infrastructure.

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#A5B4FC", Dark: "#4338CA"})
	midStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#818CF8", Dark: "#6366F1"})
	brightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"})

	// Each line represents a layer of the abstract network visualization
	lines := []struct {
		text  string
		style lipgloss.Style
	}{
		{"    ⠀⠀⠀⠀⣠⣤⣶⣶⣶⣤⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣤⣶⣶⣶⣤⣀⠀⠀⠀⠀", dimStyle},
		{"    ⠀⠀⣠⣾⣿⣿⣿⣿⣿⣿⣿⣷⣄⠀⠀⣀⣤⣤⣤⣀⠀⣠⣾⣿⣿⣿⣿⣿⣿⣿⣷⣄⠀⠀", midStyle},
		{"    ⠀⣼⣿⣿⡿⠛⠉⠀⠉⠛⢿⣿⣿⣷⣿⣿⣿⣿⣿⣿⣷⣿⣿⡿⠛⠉⠀⠉⠛⢿⣿⣿⡆⠀", brightStyle},
		{"    ⠀⣿⣿⡟⠀⠀⣶⣶⠀⠀⠀⢻⣿⣿⣿⡿⠋⠀⠙⢿⣿⣿⡟⠀⠀⣶⣶⠀⠀⠀⢻⣿⣿⠀", brightStyle},
		{"    ⠀⢿⣿⣧⠀⠀⠛⠛⠀⠀⣠⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣧⠀⠀⠛⠛⠀⠀⣠⣿⣿⡿⠀", brightStyle},
		{"    ⠀⠀⠻⣿⣷⣤⣀⣀⣤⣾⣿⣿⠟⠻⣿⣷⣤⣀⣤⣾⣿⠟⠻⣿⣷⣤⣀⣀⣤⣾⣿⠟⠀⠀", midStyle},
		{"    ⠀⠀⠀⠀⠙⠿⣿⣿⡿⠟⠁⠀⠀⠀⠀⠙⠿⣿⡿⠟⠁⠀⠀⠀⠀⠙⠿⣿⣿⡿⠟⠀⠀⠀", dimStyle},
	}

	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l.style.Render(l.text))
		sb.WriteRune('\n')
	}

	return sb.String()
}

// --- Status Dashboard Strip ---
// Compact service overview for the main menu footer area.

// ServiceStatus holds the state of a system service for dashboard display.
type ServiceStatus struct {
	Name   string
	Active bool
	Count  int // Number of items (shares, exports, mounts, users)
}

// BrailleStatusDashboard renders a compact one-line status overview
// showing service health and item counts using Braille density.
func BrailleStatusDashboard(services []ServiceStatus) string {
	var parts []string

	for _, svc := range services {
		var indicator string
		var style lipgloss.Style

		if svc.Active {
			indicator = "⣿⣿"
			style = lipgloss.NewStyle().Foreground(successColor)
		} else {
			indicator = "⠁⠁"
			style = lipgloss.NewStyle().Foreground(mutedColor)
		}

		label := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#D1D5DB"}).
			Render(svc.Name)

		countStr := ""
		if svc.Count >= 0 {
			countStyle := lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#9CA3AF"})
			countStr = countStyle.Render(strings.Repeat("⠿", min(svc.Count, 8)))
		}

		parts = append(parts, style.Render(indicator)+" "+label+" "+countStr)
	}

	return strings.Join(parts, "  │  ")
}

// --- Utility Functions ---

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func resample(values []float64, targetLen int) []float64 {
	if len(values) == 0 || targetLen == 0 {
		return nil
	}
	if len(values) == targetLen {
		return values
	}

	result := make([]float64, targetLen)
	ratio := float64(len(values)-1) / float64(targetLen-1)

	for i := 0; i < targetLen; i++ {
		pos := float64(i) * ratio
		low := int(math.Floor(pos))
		high := int(math.Ceil(pos))

		if high >= len(values) {
			high = len(values) - 1
		}
		if low == high {
			result[i] = values[low]
		} else {
			frac := pos - float64(low)
			result[i] = values[low]*(1-frac) + values[high]*frac
		}
	}
	return result
}

// densityBraille maps a 0.0–1.0 value to a Braille character of corresponding density.
func densityBraille(v float64) rune {
	// Ordered by visual density (number of raised dots)
	densityMap := []rune{
		0x2800, // ⠀  (0 dots)
		0x2800 | 0x01, // ⠁  (1 dot)
		0x2800 | 0x09, // ⠉  (2 dots)
		0x2800 | 0x49, // ⡉  (3 dots)
		0x2800 | 0x4B, // ⡋  (4 dots)
		0x2800 | 0x5B, // ⡛  (5 dots)
		0x2800 | 0xDB, // ⣛  (6 dots)
		0x2800 | 0xFB, // ⣻  (7 dots)
		0x2800 | 0xFF, // ⣿  (8 dots - full)
	}

	idx := int(math.Round(clamp(v, 0, 1) * float64(len(densityMap)-1)))
	return densityMap[idx]
}
