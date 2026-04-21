package models

import (
	"math"
	"testing"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-15
}

func TestGetCanonicalName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"claude-sonnet-4-5@20250613", "claude-sonnet-4-5"},
		{"claude-sonnet-4-5-20250613", "claude-sonnet-4-5"},
		{"claude-opus-4-6@latest", "claude-opus-4-6"},
		{"gpt-4o-20241105", "gpt-4o"},
		{"claude-sonnet-4", "claude-sonnet-4"},
		{"gpt-4o", "gpt-4o"},
	}
	for _, c := range cases {
		got := getCanonicalName(c.in)
		if got != c.want {
			t.Errorf("getCanonicalName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCalculateCostFastMultiplier(t *testing.T) {
	// claude-opus-4-6 has fastMultiplier=6
	// 1000 input tokens * 5e-6 * 6 = 0.03
	cost := CalculateCost("claude-opus-4-6", 1000, 0, 0, 0, 0, "fast")
	want := 6 * 1000 * 5e-6
	if !approxEqual(cost, want) {
		t.Errorf("fast cost = %v, want %v", cost, want)
	}
}

func TestCalculateCostStandardMultiplier(t *testing.T) {
	// standard speed: multiplier=1
	cost := CalculateCost("claude-opus-4-6", 1000, 0, 0, 0, 0, "standard")
	want := 1000 * 5e-6
	if !approxEqual(cost, want) {
		t.Errorf("standard cost = %v, want %v", cost, want)
	}
}

func TestCalculateCostDefaultMultiplier(t *testing.T) {
	// empty speed string: multiplier=1
	cost := CalculateCost("claude-opus-4-6", 1000, 0, 0, 0, 0, "")
	want := 1000 * 5e-6
	if !approxEqual(cost, want) {
		t.Errorf("default speed cost = %v, want %v", cost, want)
	}
}

func TestCalculateCostAllComponents(t *testing.T) {
	// claude-sonnet-4-5: input=3e-6, output=15e-6, cacheWrite=3.75e-6, cacheRead=0.3e-6, webSearch=0.01
	cost := CalculateCost("claude-sonnet-4-5", 100, 200, 50, 30, 2, "standard")
	want := 100*3e-6 + 200*15e-6 + 50*3.75e-6 + 30*0.3e-6 + 2*0.01
	if !approxEqual(cost, want) {
		t.Errorf("all components cost = %v, want %v", cost, want)
	}
}

func TestFallbackChainDateSuffix(t *testing.T) {
	// Model with date suffix should resolve via fallback to FALLBACK_PRICING.
	// Clear litellm pricing to force fallback path.
	SetLiteLLMPricing(nil)
	costs, ok := GetModelCosts("claude-sonnet-4-5-20250613")
	if !ok {
		t.Fatal("expected costs for claude-sonnet-4-5-20250613")
	}
	if !approxEqual(costs.InputCostPerToken, 3e-6) {
		t.Errorf("InputCostPerToken = %v, want 3e-6", costs.InputCostPerToken)
	}
}

func TestUnknownModelReturnsZeroCost(t *testing.T) {
	SetLiteLLMPricing(nil)
	cost := CalculateCost("completely-unknown-model-xyz", 1000, 1000, 0, 0, 0, "standard")
	if cost != 0 {
		t.Errorf("unknown model cost = %v, want 0", cost)
	}
}

func TestGetShortModelName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"claude-opus-4-6", "Opus 4.6"},
		{"claude-opus-4-5", "Opus 4.5"},
		{"claude-opus-4-1", "Opus 4.1"},
		{"claude-opus-4", "Opus 4"},
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-sonnet-4-5", "Sonnet 4.5"},
		{"claude-sonnet-4", "Sonnet 4"},
		{"claude-3-7-sonnet", "Sonnet 3.7"},
		{"claude-3-5-sonnet", "Sonnet 3.5"},
		{"claude-haiku-4-5", "Haiku 4.5"},
		{"claude-3-5-haiku", "Haiku 3.5"},
		{"gpt-4o-mini", "GPT-4o Mini"},
		{"gpt-4o", "GPT-4o"},
		{"gpt-5.4-mini", "GPT-5.4 Mini"},
		{"gpt-5.4", "GPT-5.4"},
		{"gpt-5.3-codex", "GPT-5.3 Codex"},
		{"gpt-5", "GPT-5"},
		{"gemini-2.5-pro", "Gemini 2.5 Pro"},
	}
	for _, c := range cases {
		got := GetShortModelName(c.in)
		if got != c.want {
			t.Errorf("GetShortModelName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGetShortModelNameWithDateSuffix(t *testing.T) {
	got := GetShortModelName("claude-sonnet-4-5-20250613")
	if got != "Sonnet 4.5" {
		t.Errorf("GetShortModelName with date suffix = %q, want %q", got, "Sonnet 4.5")
	}
}

func TestGetShortModelNameUnknown(t *testing.T) {
	got := GetShortModelName("unknown-model-xyz")
	if got != "unknown-model-xyz" {
		t.Errorf("GetShortModelName unknown = %q, want %q", got, "unknown-model-xyz")
	}
}

func TestFuzzyMatchLongestPrefixWins(t *testing.T) {
	SetLiteLLMPricing(map[string]ModelCosts{
		"claude-test":       {InputCostPerToken: 1e-6},
		"claude-test-model": {InputCostPerToken: 2e-6},
	})
	defer SetLiteLLMPricing(nil)

	costs, ok := GetModelCosts("claude-test-model-extra")
	if !ok {
		t.Fatal("expected costs for claude-test-model-extra via fuzzy match")
	}
	if !approxEqual(costs.InputCostPerToken, 2e-6) {
		t.Errorf("longest prefix match: got InputCostPerToken=%v, want 2e-6", costs.InputCostPerToken)
	}
}

func TestFuzzyMatchNoReverseDirection(t *testing.T) {
	// "short-model" should NOT match key "short-model-expensive" (reverse direction).
	SetLiteLLMPricing(map[string]ModelCosts{
		"short-model-expensive": {InputCostPerToken: 999e-6},
	})
	defer SetLiteLLMPricing(nil)

	_, ok := GetModelCosts("short-model")
	if ok {
		t.Error("reverse prefix should not match: short-model should not match short-model-expensive")
	}
}

func TestFallbackPricingAllEntries(t *testing.T) {
	SetLiteLLMPricing(nil)
	entries := []string{
		"claude-opus-4-6", "claude-opus-4-5", "claude-opus-4-1", "claude-opus-4",
		"claude-sonnet-4-6", "claude-sonnet-4-5", "claude-sonnet-4",
		"claude-3-7-sonnet", "claude-3-5-sonnet",
		"claude-haiku-4-5", "claude-3-5-haiku",
		"gpt-4o", "gpt-4o-mini",
		"gemini-2.5-pro",
		"gpt-5.3-codex", "gpt-5.4", "gpt-5.4-mini", "gpt-5",
	}
	for _, name := range entries {
		_, ok := GetModelCosts(name)
		if !ok {
			t.Errorf("expected costs for fallback entry %q", name)
		}
	}
}
