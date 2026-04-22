package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	litellmURL   = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	cacheTTL     = 24 * time.Hour
	cacheFile    = "litellm-pricing.json"
	cacheSubDir  = ".cache/codeburn"
)

// LiteLLMEntry is the raw JSON structure from the LiteLLM pricing file.
type LiteLLMEntry struct {
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	ProviderSpecificEntry       *struct {
		Fast float64 `json:"fast"`
	} `json:"provider_specific_entry"`
}

type diskCache struct {
	Timestamp int64                     `json:"timestamp"`
	Data      map[string]ModelCosts     `json:"data"`
}

func getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, cacheSubDir), nil
}

func getCachePath() (string, error) {
	dir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFile), nil
}

func parseLiteLLMEntry(entry LiteLLMEntry) (ModelCosts, bool) {
	if entry.InputCostPerToken == 0 || entry.OutputCostPerToken == 0 {
		return ModelCosts{}, false
	}
	cacheWrite := entry.CacheCreationInputTokenCost
	if cacheWrite == 0 {
		cacheWrite = entry.InputCostPerToken * 1.25
	}
	cacheRead := entry.CacheReadInputTokenCost
	if cacheRead == 0 {
		cacheRead = entry.InputCostPerToken * 0.1
	}
	fastMultiplier := 1.0
	if entry.ProviderSpecificEntry != nil && entry.ProviderSpecificEntry.Fast != 0 {
		fastMultiplier = entry.ProviderSpecificEntry.Fast
	}
	return ModelCosts{
		InputCostPerToken:       entry.InputCostPerToken,
		OutputCostPerToken:      entry.OutputCostPerToken,
		CacheWriteCostPerToken:  cacheWrite,
		CacheReadCostPerToken:   cacheRead,
		WebSearchCostPerRequest: webSearchCost,
		FastMultiplier:          fastMultiplier,
	}, true
}

func fetchFromURL(url string) (map[string]ModelCosts, error) {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{resp.StatusCode}
	}

	var raw map[string]LiteLLMEntry
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	pricing := make(map[string]ModelCosts, len(raw))
	for name, entry := range raw {
		if strings.Contains(name, "/") || strings.Contains(name, ".") {
			continue
		}
		if costs, ok := parseLiteLLMEntry(entry); ok {
			pricing[name] = costs
		}
	}
	return pricing, nil
}

type httpError struct{ code int }

func (e *httpError) Error() string { return fmt.Sprintf("HTTP %d", e.code) }

func saveToDisk(pricing map[string]ModelCosts) {
	path, err := getCachePath()
	if err != nil {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	cache := diskCache{
		Timestamp: time.Now().Unix(),
		Data:      pricing,
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func loadFromDisk() (map[string]ModelCosts, bool) {
	path, err := getCachePath()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var cache diskCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}
	age := time.Since(time.Unix(cache.Timestamp, 0))
	if age > cacheTTL {
		return nil, false
	}
	return cache.Data, true
}

// LoadPricing fetches LiteLLM pricing and sets it in memory.
// On any failure it falls back to disk cache, then to FALLBACK_PRICING.
func LoadPricing() error {
	return loadPricingFromURL(litellmURL)
}

// loadPricingFromURL is the internal implementation, accepting a URL for testability.
func loadPricingFromURL(url string) error {
	// Try disk cache first.
	if cached, ok := loadFromDisk(); ok {
		SetLiteLLMPricing(cached)
		return nil
	}

	// Try network fetch.
	pricing, err := fetchFromURL(url)
	if err == nil {
		SetLiteLLMPricing(pricing)
		saveToDisk(pricing)
		return nil
	}

	// Network failed - try expired disk cache.
	path, pathErr := getCachePath()
	if pathErr == nil {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			var cache diskCache
			if jsonErr := json.Unmarshal(data, &cache); jsonErr == nil && len(cache.Data) > 0 {
				SetLiteLLMPricing(cache.Data)
				return nil
			}
		}
	}

	// No cache at all - FALLBACK_PRICING remains active (litellmPricing stays nil).
	return nil
}
