package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	return tmp
}

func TestRead_MissingFile(t *testing.T) {
	withTempHome(t)
	cfg := Read()
	if cfg.Currency != nil {
		t.Error("expected nil currency for missing config")
	}
}

func TestRead_InvalidJSON(t *testing.T) {
	withTempHome(t)
	path := GetConfigFilePath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("{not json}"), 0o644)

	cfg := Read()
	if cfg.Currency != nil {
		t.Error("expected nil currency for invalid JSON (R50)")
	}
}

func TestSaveAndRead(t *testing.T) {
	withTempHome(t)
	cfg := Config{
		Currency: &CurrencyConfig{Code: "GBP", Symbol: "£"},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := Read()
	if got.Currency == nil {
		t.Fatal("expected currency after save")
	}
	if got.Currency.Code != "GBP" {
		t.Errorf("Code = %q, want GBP", got.Currency.Code)
	}
	if got.Currency.Symbol != "£" {
		t.Errorf("Symbol = %q, want £", got.Currency.Symbol)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	withTempHome(t)
	cfg := Config{}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(GetConfigFilePath()); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestSave_NilCurrencyOmitted(t *testing.T) {
	withTempHome(t)
	cfg := Config{Currency: nil}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Read()
	if got.Currency != nil {
		t.Error("expected nil currency after saving nil currency config")
	}
}
