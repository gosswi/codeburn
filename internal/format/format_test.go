package format

import (
	"testing"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{999_999, "1000.0K"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}
	for _, tt := range tests {
		got := FormatTokens(tt.n)
		if got != tt.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatCost_Delegates(t *testing.T) {
	// FormatCost delegates to currency.FormatCost; just verify it doesn't panic
	// and returns a non-empty string for a positive amount.
	got := FormatCost(1.23)
	if got == "" {
		t.Error("FormatCost returned empty string")
	}
}
