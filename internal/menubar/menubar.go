package menubar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agentseal/codeburn/internal/currency"
	"github.com/agentseal/codeburn/internal/format"
)

const pluginRefresh = "5m"

// PeriodData contains aggregated data for one reporting period.
type PeriodData struct {
	Label           string
	Cost            float64
	Calls           int
	InputTokens     int64
	OutputTokens    int64
	CacheReadTokens int64
	CacheWriteTokens int64
	Categories      []CategoryData
	Models          []ModelData
}

// CategoryData holds per-category stats.
type CategoryData struct {
	Name      string
	Cost      float64
	Turns     int
	EditTurns int
	OneShotTurns int
}

// ModelData holds per-model stats.
type ModelData struct {
	Name  string
	Cost  float64
	Calls int
}

// ProviderCost holds cost for a single provider.
type ProviderCost struct {
	Name string
	Cost float64
}

func miniBar(value, max float64, width int) string {
	if max == 0 {
		return strings.Repeat("·", width)
	}
	filled := int(float64(width) * value / max)
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("·", width-filled)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

func getCodeburnBin() string {
	out, err := exec.Command("which", "codeburn").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return "npx --yes codeburn"
	}
	return strings.TrimSpace(string(out))
}

// RenderMenubarFormat renders the xbar/SwiftBar plugin output.
func RenderMenubarFormat(today, week, month PeriodData, todayProviders []ProviderCost) string {
	bin := getCodeburnBin()
	home := os.Getenv("HOME")
	if home == "" {
		home = "~"
	}
	activeCurrency := currency.Get().Code

	var lines []string
	add := func(s string) { lines = append(lines, s) }

	add(fmt.Sprintf("%s | sfimage=flame.fill color=#FF8C42", currency.FormatCost(today.Cost)))
	add("---")
	add("CodeBurn | size=15 color=#FF8C42")
	add("AI Coding Cost Tracker | size=11")
	if len(todayProviders) > 1 {
		for _, p := range todayProviders {
			add(fmt.Sprintf("  %s %s | font=Menlo size=11", padRight(p.Name, 10), padLeft(currency.FormatCost(p.Cost), 10)))
		}
	}
	add("---")

	add(fmt.Sprintf("Today      %s      %s calls | size=14", currency.FormatCost(today.Cost), formatInt(today.Calls)))
	add("---")

	maxCat := maxCostCat(today.Categories)
	add("Activity - Today | size=12 color=#FF8C42")
	for _, cat := range limitN(today.Categories, 8) {
		bar := miniBar(cat.Cost, maxCat, 10)
		add(fmt.Sprintf("%s  %s %s  %s turns | font=Menlo size=11",
			bar, padRight(cat.Name, 14), padLeft(currency.FormatCost(cat.Cost), 8), padLeft(fmt.Sprintf("%d", cat.Turns), 4)))
	}
	add("---")

	maxModel := maxCostModel(today.Models)
	add("Models - Today | size=12 color=#FF8C42")
	for _, m := range limitNModel(today.Models, 5) {
		if m.Name == "<synthetic>" {
			continue
		}
		bar := miniBar(m.Cost, maxModel, 10)
		add(fmt.Sprintf("%s  %s %s  %s calls | font=Menlo size=11",
			bar, padRight(m.Name, 14), padLeft(currency.FormatCost(m.Cost), 8), padLeft(fmt.Sprintf("%d", m.Calls), 5)))
	}

	cacheHit := "0"
	total := today.InputTokens + today.CacheReadTokens
	if total > 0 {
		pct := float64(today.CacheReadTokens) / float64(total) * 100
		cacheHit = fmt.Sprintf("%.0f", pct)
	}
	add(fmt.Sprintf("Tokens: %s in · %s out · %s%% cache hit | font=Menlo size=10",
		format.FormatTokens(today.InputTokens), format.FormatTokens(today.OutputTokens), cacheHit))
	add("---")

	// Week
	add(fmt.Sprintf("7 Days     %s    %s calls | size=14", currency.FormatCost(week.Cost), formatInt(week.Calls)))
	wMaxCat := maxCostCat(week.Categories)
	wMaxModel := maxCostModel(week.Models)
	add("--Activity | size=12 color=#FF8C42")
	for _, cat := range limitN(week.Categories, 8) {
		bar := miniBar(cat.Cost, wMaxCat, 10)
		add(fmt.Sprintf("--%s  %s %s  %s turns | font=Menlo size=11",
			bar, padRight(cat.Name, 14), padLeft(currency.FormatCost(cat.Cost), 8), padLeft(fmt.Sprintf("%d", cat.Turns), 4)))
	}
	add("-----")
	add("--Models | size=12 color=#FF8C42")
	for _, m := range limitNModel(week.Models, 5) {
		if m.Name == "<synthetic>" {
			continue
		}
		bar := miniBar(m.Cost, wMaxModel, 10)
		add(fmt.Sprintf("--%s  %s %s  %s calls | font=Menlo size=11",
			bar, padRight(m.Name, 14), padLeft(currency.FormatCost(m.Cost), 8), padLeft(fmt.Sprintf("%d", m.Calls), 5)))
	}

	// Month
	add(fmt.Sprintf("Month      %s    %s calls | size=14", currency.FormatCost(month.Cost), formatInt(month.Calls)))
	mMaxCat := maxCostCat(month.Categories)
	mMaxModel := maxCostModel(month.Models)
	add("--Activity | size=12 color=#FF8C42")
	for _, cat := range limitN(month.Categories, 8) {
		bar := miniBar(cat.Cost, mMaxCat, 10)
		add(fmt.Sprintf("--%s  %s %s  %s turns | font=Menlo size=11",
			bar, padRight(cat.Name, 14), padLeft(currency.FormatCost(cat.Cost), 8), padLeft(fmt.Sprintf("%d", cat.Turns), 4)))
	}
	add("-----")
	add("--Models | size=12 color=#FF8C42")
	for _, m := range limitNModel(month.Models, 5) {
		if m.Name == "<synthetic>" {
			continue
		}
		bar := miniBar(m.Cost, mMaxModel, 10)
		add(fmt.Sprintf("--%s  %s %s  %s calls | font=Menlo size=11",
			bar, padRight(m.Name, 14), padLeft(currency.FormatCost(m.Cost), 8), padLeft(fmt.Sprintf("%d", m.Calls), 5)))
	}

	add("---")
	add(fmt.Sprintf("Open Full Report | terminal=true shell=%s param1=report", bin))
	add(fmt.Sprintf("Export CSV to Desktop | terminal=false shell=%s param1=export param2=-o param3=%s/Desktop/codeburn-report.csv", bin, home))

	// Currency submenu
	currencies := []struct{ Code, Name string }{
		{"USD", "US Dollar"},
		{"GBP", "British Pound"},
		{"EUR", "Euro"},
		{"AUD", "Australian Dollar"},
		{"CAD", "Canadian Dollar"},
		{"NZD", "New Zealand Dollar"},
		{"JPY", "Japanese Yen"},
		{"CHF", "Swiss Franc"},
		{"INR", "Indian Rupee"},
		{"BRL", "Brazilian Real"},
		{"SEK", "Swedish Krona"},
		{"SGD", "Singapore Dollar"},
		{"HKD", "Hong Kong Dollar"},
		{"KRW", "South Korean Won"},
		{"MXN", "Mexican Peso"},
		{"ZAR", "South African Rand"},
		{"DKK", "Danish Krone"},
	}
	add(fmt.Sprintf("Currency: %s | size=14", activeCurrency))
	for _, c := range currencies {
		check := ""
		if c.Code == activeCurrency {
			check = " *"
		}
		if c.Code == "USD" {
			add(fmt.Sprintf("--%s (%s)%s | terminal=false refresh=true shell=%s param1=currency param2=--reset",
				c.Name, c.Code, check, bin))
		} else {
			add(fmt.Sprintf("--%s (%s)%s | terminal=false refresh=true shell=%s param1=currency param2=%s",
				c.Name, c.Code, check, bin, c.Code))
		}
	}
	add("Refresh | refresh=true")

	return strings.Join(lines, "\n")
}

// InstallMenubar writes the xbar/SwiftBar plugin script.
func InstallMenubar() (string, error) {
	if runtime.GOOS != "darwin" {
		return "Menu bar integration is only available on macOS. Use `codeburn watch` or `codeburn status` instead.", nil
	}

	bin := getCodeburnBin()
	home, _ := os.UserHomeDir()
	pluginContent := generatePlugin(bin, home)

	swiftbarDir := filepath.Join(home, "Library", "Application Support", "SwiftBar", "plugins")
	xbarDir := filepath.Join(home, "Library", "Application Support", "xbar", "plugins")

	pluginDir := swiftbarDir
	appName := "SwiftBar"

	if _, err := os.Stat(swiftbarDir); err == nil {
		pluginDir = swiftbarDir
		appName = "SwiftBar"
	} else if _, err := os.Stat(xbarDir); err == nil {
		pluginDir = xbarDir
		appName = "xbar"
	} else {
		if err := os.MkdirAll(swiftbarDir, 0o755); err != nil {
			return "", err
		}
	}

	pluginPath := filepath.Join(pluginDir, fmt.Sprintf("codeburn.%s.sh", pluginRefresh))
	if err := os.WriteFile(pluginPath, []byte(pluginContent), 0o755); err != nil {
		return "", err
	}

	swiftbarApp := existsAny(
		"/Applications/SwiftBar.app",
		filepath.Join(home, "Applications", "SwiftBar.app"),
	)
	xbarApp := existsAny(
		"/Applications/xbar.app",
		filepath.Join(home, "Applications", "xbar.app"),
	)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  Plugin installed to: %s\n", pluginPath))
	if swiftbarApp || xbarApp {
		sb.WriteString(fmt.Sprintf("  %s detected - plugin should appear in your menu bar shortly.\n", appName))
		sb.WriteString(fmt.Sprintf("  If not, open %s and refresh plugins.\n\n", appName))
	} else {
		sb.WriteString("\n  To see CodeBurn in your menu bar, install SwiftBar:\n")
		sb.WriteString("    brew install --cask swiftbar\n")
		sb.WriteString("\n  Then launch SwiftBar - the plugin will load automatically.\n\n")
	}
	return sb.String(), nil
}

// UninstallMenubar removes the xbar/SwiftBar plugin script.
func UninstallMenubar() (string, error) {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, "Library", "Application Support", "SwiftBar", "plugins", fmt.Sprintf("codeburn.%s.sh", pluginRefresh)),
		filepath.Join(home, "Library", "Application Support", "xbar", "plugins", fmt.Sprintf("codeburn.%s.sh", pluginRefresh)),
	}
	removed := false
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err != nil {
				return "", err
			}
			removed = true
		}
	}
	if removed {
		return "\n  Menu bar plugin removed.\n", nil
	}
	return "\n  No menu bar plugin found.\n", nil
}

func generatePlugin(bin, home string) string {
	return fmt.Sprintf(`#!/bin/bash
# <xbar.title>CodeBurn</xbar.title>
# <xbar.version>v0.1.0</xbar.version>
# <xbar.author>AgentSeal</xbar.author>
# <xbar.author.github>agentseal</xbar.author.github>
# <xbar.desc>See where your AI coding tokens burn. Tracks cost, activity, and model usage across Claude Code, Cursor, and Codex by task type, tool, MCP server, and project.</xbar.desc>
# <xbar.image>file://%s/codeburn/assets/logo.png</xbar.image>
# <xbar.abouturl>https://github.com/agentseal/codeburn</xbar.abouturl>
# <xbar.dependencies>node</xbar.dependencies>

export PATH="/usr/local/bin:/opt/homebrew/bin:$HOME/.local/bin:$HOME/.npm-global/bin:$PATH"

%s status --format menubar 2>/dev/null || echo "-- | sfimage=flame.fill"
`, home, bin)
}

func existsAny(paths ...string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func formatInt(n int) string { return fmt.Sprintf("%d", n) }

func maxCostCat(cats []CategoryData) float64 {
	max := 0.01
	for _, c := range cats {
		if c.Cost > max {
			max = c.Cost
		}
	}
	return max
}

func maxCostModel(models []ModelData) float64 {
	max := 0.01
	for _, m := range models {
		if m.Name != "<synthetic>" && m.Cost > max {
			max = m.Cost
		}
	}
	return max
}

func limitN(cats []CategoryData, n int) []CategoryData {
	if len(cats) <= n {
		return cats
	}
	return cats[:n]
}

func limitNModel(models []ModelData, n int) []ModelData {
	if len(models) <= n {
		return models
	}
	return models[:n]
}
