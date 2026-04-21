package currency

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentseal/codeburn/internal/config"
)

const (
	cacheTTL       = 24 * time.Hour
	frankfurterURL = "https://api.frankfurter.app/latest?from=USD&to="
)

// State holds the active currency for display.
type State struct {
	Code   string
	Rate   float64
	Symbol string
}

var (
	mu     sync.RWMutex
	active = State{Code: "USD", Rate: 1, Symbol: "$"}
)

// symbolTable maps ISO 4217 codes to display symbols.
var symbolTable = map[string]string{
	"USD": "$",
	"GBP": "£",
	"EUR": "€",
	"AUD": "A$",
	"CAD": "C$",
	"NZD": "NZ$",
	"JPY": "¥",
	"CHF": "Fr",
	"INR": "₹",
	"BRL": "R$",
	"SEK": "kr",
	"SGD": "S$",
	"HKD": "HK$",
	"KRW": "₩",
	"MXN": "MX$",
	"ZAR": "R",
	"DKK": "kr",
}

// fractionDigits returns the number of decimal places for a currency code.
var fractionDigits = map[string]int{
	"JPY": 0,
	"KRW": 0,
}

// validCodes is the supported set for IsValidCurrencyCode.
var validCodes = func() map[string]struct{} {
	m := make(map[string]struct{}, len(symbolTable))
	for k := range symbolTable {
		m[k] = struct{}{}
	}
	return m
}()

// IsValidCurrencyCode returns true if code is in the supported set.
func IsValidCurrencyCode(code string) bool {
	_, ok := validCodes[strings.ToUpper(code)]
	return ok
}

// ResolveSymbol returns the display symbol for code.
func ResolveSymbol(code string) string {
	if sym, ok := symbolTable[strings.ToUpper(code)]; ok {
		return sym
	}
	return code
}

// GetFractionDigits returns the number of fraction digits for a currency.
func GetFractionDigits(code string) int {
	if d, ok := fractionDigits[strings.ToUpper(code)]; ok {
		return d
	}
	return 2
}

// --- disk cache ---

type rateCache struct {
	Timestamp int64   `json:"timestamp"` // Unix ms
	Code      string  `json:"code"`
	Rate      float64 `json:"rate"`
}

func getCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "codeburn")
}

func getRateCachePath() string {
	return filepath.Join(getCacheDir(), "exchange-rate.json")
}

func loadCachedRate(code string) (float64, bool) {
	data, err := os.ReadFile(getRateCachePath())
	if err != nil {
		return 0, false
	}
	var c rateCache
	if err := json.Unmarshal(data, &c); err != nil {
		return 0, false
	}
	if !strings.EqualFold(c.Code, code) {
		return 0, false
	}
	if time.Now().UnixMilli()-c.Timestamp > int64(cacheTTL.Milliseconds()) {
		return 0, false
	}
	return c.Rate, true
}

func saveRateCache(code string, rate float64) {
	if err := os.MkdirAll(getCacheDir(), 0o755); err != nil {
		return
	}
	c := rateCache{Timestamp: time.Now().UnixMilli(), Code: code, Rate: rate}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(getRateCachePath(), data, 0o644)
}

func fetchRate(code string) (float64, error) {
	resp, err := http.Get(frankfurterURL + code) //nolint:noctx
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var body struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	r, ok := body.Rates[strings.ToUpper(code)]
	if !ok {
		return 0, fmt.Errorf("no rate for %s", code)
	}
	return r, nil
}

// GetExchangeRate returns the USD->code rate, using cache or API.
// Returns 1 on any error (per R22).
func GetExchangeRate(code string) float64 {
	upper := strings.ToUpper(code)
	if upper == "USD" {
		return 1
	}
	if r, ok := loadCachedRate(upper); ok {
		return r
	}
	r, err := fetchRate(upper)
	if err != nil {
		return 1
	}
	saveRateCache(upper, r)
	return r
}

// Load reads the config and initialises the active currency state.
func Load() {
	cfg := config.Read()
	if cfg.Currency == nil {
		return
	}
	code := strings.ToUpper(cfg.Currency.Code)
	rate := GetExchangeRate(code)
	sym := cfg.Currency.Symbol
	if sym == "" {
		sym = ResolveSymbol(code)
	}
	mu.Lock()
	active = State{Code: code, Rate: rate, Symbol: sym}
	mu.Unlock()
}

// Get returns the active currency state.
func Get() State {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

// Switch changes the active currency to code, updating state and config.
func Switch(code string) error {
	upper := strings.ToUpper(code)
	if upper == "USD" {
		mu.Lock()
		active = State{Code: "USD", Rate: 1, Symbol: "$"}
		mu.Unlock()
		return nil
	}
	rate := GetExchangeRate(upper)
	sym := ResolveSymbol(upper)
	mu.Lock()
	active = State{Code: upper, Rate: rate, Symbol: sym}
	mu.Unlock()
	return nil
}

// GetCostColumnHeader returns the column header string for cost columns.
func GetCostColumnHeader() string {
	mu.RLock()
	defer mu.RUnlock()
	return fmt.Sprintf("Cost (%s)", active.Code)
}

// ConvertCost converts a USD cost to the active currency, rounded to fraction digits.
func ConvertCost(costUSD float64) float64 {
	mu.RLock()
	s := active
	mu.RUnlock()
	digits := GetFractionDigits(s.Code)
	factor := 1.0
	for i := 0; i < digits; i++ {
		factor *= 10
	}
	if factor == 0 {
		return 0
	}
	v := costUSD * s.Rate * factor
	rounded := float64(int64(v+0.5)) / factor
	return rounded
}

// FormatCost formats a USD cost in the active currency with appropriate decimal places.
func FormatCost(costUSD float64) string {
	mu.RLock()
	s := active
	mu.RUnlock()
	cost := costUSD * s.Rate
	digits := GetFractionDigits(s.Code)

	if digits == 0 {
		return fmt.Sprintf("%s%d", s.Symbol, int64(cost+0.5))
	}
	if cost >= 1 {
		return fmt.Sprintf("%s%.2f", s.Symbol, cost)
	}
	if cost >= 0.01 {
		return fmt.Sprintf("%s%.3f", s.Symbol, cost)
	}
	return fmt.Sprintf("%s%.4f", s.Symbol, cost)
}
