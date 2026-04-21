package models

import (
	"regexp"
	"strings"
	"sync"
)

// ModelCosts holds per-token and per-request pricing for a model.
type ModelCosts struct {
	InputCostPerToken       float64
	OutputCostPerToken      float64
	CacheWriteCostPerToken  float64
	CacheReadCostPerToken   float64
	WebSearchCostPerRequest float64
	FastMultiplier          float64
}

const webSearchCost = 0.01

// fallbackPricing is the hardcoded pricing table used when LiteLLM data is unavailable.
var fallbackPricing = map[string]ModelCosts{
	"claude-opus-4-6":  {5e-6, 25e-6, 6.25e-6, 0.5e-6, webSearchCost, 6},
	"claude-opus-4-5":  {5e-6, 25e-6, 6.25e-6, 0.5e-6, webSearchCost, 1},
	"claude-opus-4-1":  {15e-6, 75e-6, 18.75e-6, 1.5e-6, webSearchCost, 1},
	"claude-opus-4":    {15e-6, 75e-6, 18.75e-6, 1.5e-6, webSearchCost, 1},
	"claude-sonnet-4-6": {3e-6, 15e-6, 3.75e-6, 0.3e-6, webSearchCost, 1},
	"claude-sonnet-4-5": {3e-6, 15e-6, 3.75e-6, 0.3e-6, webSearchCost, 1},
	"claude-sonnet-4":   {3e-6, 15e-6, 3.75e-6, 0.3e-6, webSearchCost, 1},
	"claude-3-7-sonnet": {3e-6, 15e-6, 3.75e-6, 0.3e-6, webSearchCost, 1},
	"claude-3-5-sonnet": {3e-6, 15e-6, 3.75e-6, 0.3e-6, webSearchCost, 1},
	"claude-haiku-4-5":  {1e-6, 5e-6, 1.25e-6, 0.1e-6, webSearchCost, 1},
	"claude-3-5-haiku":  {0.8e-6, 4e-6, 1e-6, 0.08e-6, webSearchCost, 1},
	"gpt-4o":            {2.5e-6, 10e-6, 2.5e-6, 1.25e-6, webSearchCost, 1},
	"gpt-4o-mini":       {0.15e-6, 0.6e-6, 0.15e-6, 0.075e-6, webSearchCost, 1},
	"gemini-2.5-pro":    {1.25e-6, 10e-6, 1.25e-6, 0.315e-6, webSearchCost, 1},
	"gpt-5.3-codex":     {2.5e-6, 10e-6, 2.5e-6, 1.25e-6, webSearchCost, 1},
	"gpt-5.4":           {2.5e-6, 10e-6, 2.5e-6, 1.25e-6, webSearchCost, 1},
	"gpt-5.4-mini":      {0.4e-6, 1.6e-6, 0.4e-6, 0.2e-6, webSearchCost, 1},
	"gpt-5":             {2.5e-6, 10e-6, 2.5e-6, 1.25e-6, webSearchCost, 1},
}

// litellmPricing is loaded at startup from the LiteLLM pricing JSON.
var (
	litellmMu      sync.RWMutex
	litellmPricing map[string]ModelCosts
)

// SetLiteLLMPricing replaces the in-memory LiteLLM pricing map.
func SetLiteLLMPricing(m map[string]ModelCosts) {
	litellmMu.Lock()
	defer litellmMu.Unlock()
	litellmPricing = m
}

var reAtSuffix   = regexp.MustCompile(`@.*$`)
var reDateSuffix = regexp.MustCompile(`-\d{8}$`)

// getCanonicalName strips @-suffixes and 8-digit date suffixes.
func getCanonicalName(model string) string {
	s := reAtSuffix.ReplaceAllString(model, "")
	return reDateSuffix.ReplaceAllString(s, "")
}

// GetModelCosts returns pricing for the given model using a four-level fallback chain.
func GetModelCosts(model string) (*ModelCosts, bool) {
	canonical := getCanonicalName(model)

	litellmMu.RLock()
	pricing := litellmPricing
	litellmMu.RUnlock()

	// Level 1: exact match in LiteLLM map.
	if pricing != nil {
		if c, ok := pricing[canonical]; ok {
			return &c, true
		}
	}

	// Level 2: FALLBACK_PRICING - exact match first, then longest trailing-dash prefix.
	if c, ok := fallbackPricing[canonical]; ok {
		return &c, true
	}
	var bestFallback string
	for key := range fallbackPricing {
		if strings.HasPrefix(canonical, key+"-") && len(key) > len(bestFallback) {
			bestFallback = key
		}
	}
	if bestFallback != "" {
		c := fallbackPricing[bestFallback]
		return &c, true
	}

	// Level 3: LiteLLM forward prefix match, longest key wins for determinism.
	if pricing != nil {
		var bestKey string
		for key := range pricing {
			if strings.HasPrefix(canonical, key) && len(key) > len(bestKey) {
				bestKey = key
			}
		}
		if bestKey != "" {
			c := pricing[bestKey]
			return &c, true
		}
	}

	// Level 4: FALLBACK_PRICING prefix match.
	for key, costs := range fallbackPricing {
		if strings.HasPrefix(canonical, key) {
			c := costs
			return &c, true
		}
	}

	return nil, false
}

// CalculateCost returns the total cost for one model call.
func CalculateCost(model string, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, webSearchRequests int64, speed string) float64 {
	costs, ok := GetModelCosts(model)
	if !ok {
		return 0
	}

	multiplier := 1.0
	if speed == "fast" {
		multiplier = costs.FastMultiplier
	}

	return multiplier * (
		float64(inputTokens)*costs.InputCostPerToken +
			float64(outputTokens)*costs.OutputCostPerToken +
			float64(cacheCreationTokens)*costs.CacheWriteCostPerToken +
			float64(cacheReadTokens)*costs.CacheReadCostPerToken +
			float64(webSearchRequests)*costs.WebSearchCostPerRequest)
}

// shortNames maps canonical prefixes to display names. Order matters: more specific keys first.
var shortNames = []struct {
	key  string
	name string
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

// GetShortModelName returns a human-readable display name for a model.
func GetShortModelName(model string) string {
	canonical := getCanonicalName(model)
	for _, e := range shortNames {
		if strings.HasPrefix(canonical, e.key) {
			return e.name
		}
	}
	return canonical
}
