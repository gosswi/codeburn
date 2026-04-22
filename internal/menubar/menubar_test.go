package menubar

import (
	"runtime"
	"strings"
	"testing"
)

func testPeriodData(label string, cost float64, calls int) PeriodData {
	return PeriodData{
		Label:        label,
		Cost:         cost,
		Calls:        calls,
		InputTokens:  1000,
		OutputTokens: 500,
		Categories: []CategoryData{
			{Name: "Coding", Cost: cost * 0.8, Turns: 5},
		},
		Models: []ModelData{
			{Name: "claude-sonnet-4-5", Cost: cost, Calls: calls},
		},
	}
}

func TestRenderMenubarFormat_Structure(t *testing.T) {
	today := testPeriodData("Today", 1.23, 10)
	week := testPeriodData("7 Days", 5.0, 50)
	month := testPeriodData("Month", 15.0, 150)

	out := RenderMenubarFormat(today, week, month, nil)

	if !strings.Contains(out, "---") {
		t.Error("expected separator lines")
	}
	if !strings.Contains(out, "CodeBurn") {
		t.Error("expected CodeBurn title")
	}
	if !strings.Contains(out, "Today") {
		t.Error("expected Today section")
	}
	if !strings.Contains(out, "7 Days") {
		t.Error("expected 7 Days section")
	}
	if !strings.Contains(out, "Month") {
		t.Error("expected Month section")
	}
	if !strings.Contains(out, "Currency:") {
		t.Error("expected currency submenu")
	}
}

func TestRenderMenubarFormat_ProviderList(t *testing.T) {
	today := testPeriodData("Today", 1.0, 5)
	week := testPeriodData("7 Days", 3.0, 15)
	month := testPeriodData("Month", 10.0, 50)
	providers := []ProviderCost{
		{Name: "Claude", Cost: 0.8},
		{Name: "Cursor", Cost: 0.2},
	}

	out := RenderMenubarFormat(today, week, month, providers)
	if !strings.Contains(out, "Claude") {
		t.Error("expected Claude provider in output")
	}
	if !strings.Contains(out, "Cursor") {
		t.Error("expected Cursor provider in output")
	}
}

func TestMiniBar(t *testing.T) {
	tests := []struct {
		value, max float64
		width      int
		wantPrefix string
	}{
		{0, 0, 5, "·····"},
		{10, 10, 5, "█████"},
		{5, 10, 10, "█████·····"},
	}
	for _, tt := range tests {
		got := miniBar(tt.value, tt.max, tt.width)
		if got != tt.wantPrefix {
			t.Errorf("miniBar(%v, %v, %d) = %q, want %q", tt.value, tt.max, tt.width, got, tt.wantPrefix)
		}
	}
}

func TestInstallMenubar_NonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping non-darwin check on darwin")
	}
	result, err := InstallMenubar()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "only available on macOS") {
		t.Errorf("expected macOS-only message, got: %s", result)
	}
}

func TestUninstallMenubar_NoneInstalled(t *testing.T) {
	// Use a temp HOME so no plugin exists.
	t.Setenv("HOME", t.TempDir())
	result, err := UninstallMenubar()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No menu bar plugin found") {
		t.Errorf("expected 'No menu bar plugin found', got: %s", result)
	}
}
