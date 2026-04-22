package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

// parseRGB parses a "#rrggbb" hex string to (r, g, b) ints.
func parseRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return int(r), int(g), int(b)
}

func withinTolerance(got, want, tol int) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func TestGradientColor_Segments(t *testing.T) {
	const tol = 2 // +/-2 RGB tolerance per spec

	tests := []struct {
		pct  float64
		desc string
		// Expected reference colors at these breakpoints
		wantR, wantG, wantB int
	}{
		{0.0, "start (blue)", 91, 158, 245},
		{0.33, "mid1 (amber start)", 245, 200, 91},
		{0.66, "mid2 (orange mid)", 255, 140, 66},
		{1.0, "end (dark orange)", 245, 91, 91},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("pct=%.2f %s", tt.pct, tt.desc), func(t *testing.T) {
			got := gradientColor(tt.pct)
			r, g, b := parseRGB(got)
			if !withinTolerance(r, tt.wantR, tol) || !withinTolerance(g, tt.wantG, tol) || !withinTolerance(b, tt.wantB, tol) {
				t.Errorf("gradientColor(%.2f) = %s (rgb=%d,%d,%d), want ~(rgb=%d,%d,%d) within ±%d",
					tt.pct, got, r, g, b, tt.wantR, tt.wantG, tt.wantB, tol)
			}
		})
	}
}

func TestGradientColor_Monotonic(t *testing.T) {
	// The gradient should vary smoothly. Ensure no wild jumps between adjacent samples.
	const maxJump = 60
	prev := gradientColor(0)
	pr, pg, pb := parseRGB(prev)
	for i := 1; i <= 20; i++ {
		pct := float64(i) / 20.0
		cur := gradientColor(pct)
		cr, cg, cb := parseRGB(cur)
		jump := math.Abs(float64(cr-pr)) + math.Abs(float64(cg-pg)) + math.Abs(float64(cb-pb))
		if jump > maxJump {
			t.Errorf("large jump at pct=%.2f: %s -> %s (sum diff=%.0f)", pct, prev, cur, jump)
		}
		prev = cur
		pr, pg, pb = cr, cg, cb
	}
}

func TestHBar_Empty(t *testing.T) {
	result := HBar(10, 0, 0)
	// With max=0, all characters should be '░'.
	plain := stripANSI(result)
	if !strings.Contains(plain, "░") {
		t.Errorf("empty bar should contain '░', got %q", plain)
	}
	if strings.Contains(plain, "█") {
		t.Error("empty bar should not contain filled blocks")
	}
}

func TestHBar_Full(t *testing.T) {
	result := HBar(8, 10, 10)
	plain := stripANSI(result)
	if strings.Contains(plain, "░") {
		t.Errorf("full bar should not contain '░', got %q", plain)
	}
	if len([]rune(plain)) != 8 {
		t.Errorf("full bar should have %d chars, got %d", 8, len([]rune(plain)))
	}
}

func TestHBar_Half(t *testing.T) {
	result := HBar(10, 5, 10)
	plain := stripANSI(result)
	count := func(r rune) int {
		n := 0
		for _, c := range plain {
			if c == r {
				n++
			}
		}
		return n
	}
	filled := count('█')
	empty := count('░')
	if filled != 5 {
		t.Errorf("half bar: want 5 filled, got %d (plain=%q)", filled, plain)
	}
	if empty != 5 {
		t.Errorf("half bar: want 5 empty, got %d (plain=%q)", empty, plain)
	}
}

// stripANSI removes ANSI escape sequences for easier string comparison.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
