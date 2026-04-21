package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user preferences.
type Config struct {
	Currency *CurrencyConfig `json:"currency,omitempty"`
}

// CurrencyConfig stores the active currency settings.
type CurrencyConfig struct {
	Code   string `json:"code"`
	Symbol string `json:"symbol,omitempty"`
}

func getConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "codeburn")
}

// GetConfigFilePath returns the path to the config file.
func GetConfigFilePath() string {
	return filepath.Join(getConfigDir(), "config.json")
}

// Read loads the config file, returning an empty Config on any error.
func Read() Config {
	data, err := os.ReadFile(GetConfigFilePath())
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

// Save writes the config to disk.
func Save(cfg Config) error {
	path := GetConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
