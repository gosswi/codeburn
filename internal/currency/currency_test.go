package currency

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidCurrencyCode(t *testing.T) {
	if !IsValidCurrencyCode("USD") {
		t.Error("USD should be valid")
	}
	if !IsValidCurrencyCode("gbp") { // case-insensitive
		t.Error("gbp (lowercase) should be valid")
	}
	if IsValidCurrencyCode("XYZ") {
		t.Error("XYZ should not be valid")
	}
	if IsValidCurrencyCode("") {
		t.Error("empty string should not be valid")
	}
}

func TestResolveSymbol(t *testing.T) {
	tests := []struct {
		code, want string
	}{
		{"USD", "$"},
		{"GBP", "£"},
		{"EUR", "€"},
		{"JPY", "¥"},
		{"KRW", "₩"},
		{"UNKNOWN", "UNKNOWN"}, // falls back to code
	}
	for _, tt := range tests {
		got := ResolveSymbol(tt.code)
		if got != tt.want {
			t.Errorf("ResolveSymbol(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestGetFractionDigits(t *testing.T) {
	if GetFractionDigits("JPY") != 0 {
		t.Error("JPY should have 0 fraction digits")
	}
	if GetFractionDigits("KRW") != 0 {
		t.Error("KRW should have 0 fraction digits")
	}
	if GetFractionDigits("USD") != 2 {
		t.Error("USD should have 2 fraction digits")
	}
	if GetFractionDigits("EUR") != 2 {
		t.Error("EUR should have 2 fraction digits")
	}
}

func TestFormatCost_USD(t *testing.T) {
	// Default active is USD, rate=1, symbol=$
	tests := []struct {
		cost float64
		want string
	}{
		{1.50, "$1.50"},
		{0.05, "$0.050"},
		{0.001, "$0.0010"},
		{10.0, "$10.00"},
	}
	for _, tt := range tests {
		got := FormatCost(tt.cost)
		if got != tt.want {
			t.Errorf("FormatCost(%v) = %q, want %q", tt.cost, got, tt.want)
		}
	}
}

func TestCacheRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Nothing cached yet.
	_, ok := loadCachedRate("EUR")
	if ok {
		t.Error("expected cache miss initially")
	}

	// Save and reload.
	saveRateCache("EUR", 0.92)
	rate, ok := loadCachedRate("EUR")
	if !ok {
		t.Error("expected cache hit after save")
	}
	if rate != 0.92 {
		t.Errorf("cached rate = %v, want 0.92", rate)
	}
}

func TestCacheExpiry(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write an expired cache entry manually.
	cacheDir := filepath.Join(tmp, ".cache", "codeburn")
	os.MkdirAll(cacheDir, 0o755)
	expired := rateCache{
		Timestamp: 0, // epoch - definitely expired
		Code:      "GBP",
		Rate:      1.25,
	}
	data, _ := json.Marshal(expired)
	os.WriteFile(filepath.Join(cacheDir, "exchange-rate.json"), data, 0o644)

	_, ok := loadCachedRate("GBP")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
}

func TestCacheWrongCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	saveRateCache("EUR", 0.92)

	// Requesting a different code should miss.
	_, ok := loadCachedRate("GBP")
	if ok {
		t.Error("expected cache miss for different code")
	}
}

func TestGetExchangeRate_USD(t *testing.T) {
	rate := GetExchangeRate("USD")
	if rate != 1 {
		t.Errorf("USD rate should be 1, got %v", rate)
	}
}
