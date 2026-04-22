package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	dimColor   = "#333333"
	emptyColor = "#555555"
)

// gradientColor returns a hex color string for position pct (0..1) across a
// blue -> amber -> orange gradient.
// Segment breakpoints: [91,158,245] -> [245,200,91] -> [255,140,66] -> [245,91,91]
func gradientColor(pct float64) string {
	lerp := func(a, b, t float64) float64 { return a + t*(b-a) }
	round := func(v float64) int { return int(math.Round(v)) }
	toHex := func(r, g, b int) string {
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	if pct <= 0.33 {
		t := pct / 0.33
		return toHex(round(lerp(91, 245, t)), round(lerp(158, 200, t)), round(lerp(245, 91, t)))
	}
	if pct <= 0.66 {
		t := (pct - 0.33) / 0.33
		return toHex(round(lerp(245, 255, t)), round(lerp(200, 140, t)), round(lerp(91, 66, t)))
	}
	t := (pct - 0.66) / 0.34
	return toHex(round(lerp(255, 245, t)), round(lerp(140, 91, t)), round(lerp(66, 91, t)))
}

// HBar renders a horizontal bar with gradient fill, dim unfilled segments,
// and all-dim for empty bars (max == 0).
func HBar(width, value, max int) string {
	if max == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render(strings.Repeat("░", width))
	}
	filled := int(math.Round(float64(value) / float64(max) * float64(width)))
	if filled > width {
		filled = width
	}
	var sb strings.Builder
	for i := 0; i < filled; i++ {
		pct := float64(i) / float64(width)
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(gradientColor(pct))).Render("█"))
	}
	if width-filled > 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(dimColor)).Render(strings.Repeat("░", width-filled)))
	}
	return sb.String()
}
