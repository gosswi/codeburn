package models

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchFromURL_ParsesValidEntry(t *testing.T) {
	payload := map[string]LiteLLMEntry{
		"test-model": {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	pricing, err := fetchFromURL(srv.URL)
	if err != nil {
		t.Fatalf("fetchFromURL returned error: %v", err)
	}
	costs, ok := pricing["test-model"]
	if !ok {
		t.Fatal("expected test-model in pricing map")
	}
	if costs.InputCostPerToken != 1e-6 {
		t.Errorf("InputCostPerToken = %v, want 1e-6", costs.InputCostPerToken)
	}
	if costs.OutputCostPerToken != 2e-6 {
		t.Errorf("OutputCostPerToken = %v, want 2e-6", costs.OutputCostPerToken)
	}
}

func TestFetchFromURL_SkipsSlashAndDotNames(t *testing.T) {
	payload := map[string]LiteLLMEntry{
		"good-model":     {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
		"bad/model":      {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
		"bad.model":      {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
		"also/bad.model": {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	pricing, err := fetchFromURL(srv.URL)
	if err != nil {
		t.Fatalf("fetchFromURL returned error: %v", err)
	}
	if _, ok := pricing["good-model"]; !ok {
		t.Error("expected good-model in pricing map")
	}
	for _, bad := range []string{"bad/model", "bad.model", "also/bad.model"} {
		if _, ok := pricing[bad]; ok {
			t.Errorf("expected %q to be skipped", bad)
		}
	}
}

func TestLoadPricingFromURL_HTTPErrorFallsBackToFallbackPricing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	SetLiteLLMPricing(nil)
	_ = loadPricingFromURL(srv.URL)

	// Fallback pricing must still resolve costs.
	cost := CalculateCost("claude-sonnet-4-5", 1000, 0, 0, 0, 0, "standard")
	if cost <= 0 {
		t.Errorf("expected cost > 0 from fallback pricing, got %v", cost)
	}
}

func TestLoadPricingFromURL_DiskCacheSkipsFetch(t *testing.T) {
	fetchCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCalled = true
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cacheDir := filepath.Join(tmpDir, cacheSubDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cache := diskCache{
		Timestamp: time.Now().Unix(),
		Data: map[string]ModelCosts{
			"cached-model": {InputCostPerToken: 5e-6, OutputCostPerToken: 10e-6},
		},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(filepath.Join(cacheDir, cacheFile), data, 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	SetLiteLLMPricing(nil)
	_ = loadPricingFromURL(srv.URL)

	if fetchCalled {
		t.Error("expected fetch to be skipped when valid disk cache exists")
	}
	costs, ok := GetModelCosts("cached-model")
	if !ok {
		t.Fatal("expected cached-model in pricing after loading disk cache")
	}
	if costs.InputCostPerToken != 5e-6 {
		t.Errorf("InputCostPerToken = %v, want 5e-6", costs.InputCostPerToken)
	}
}
