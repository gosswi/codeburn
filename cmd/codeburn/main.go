package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentseal/codeburn/internal/config"
	"github.com/agentseal/codeburn/internal/currency"
	"github.com/agentseal/codeburn/internal/export"
	"github.com/agentseal/codeburn/internal/format"
	"github.com/agentseal/codeburn/internal/menubar"
	"github.com/agentseal/codeburn/internal/models"
	"github.com/agentseal/codeburn/internal/parser"
	"github.com/agentseal/codeburn/internal/tui"
	"github.com/agentseal/codeburn/internal/types"
)

const version = "0.6.0"

// --- date range helpers ---

type dateRangeResult struct {
	Range types.DateRange
	Label string
}

func getDateRange(period string) dateRangeResult {
	now := time.Now()
	// End: today at 23:59:59.999
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999_000_000, now.Location())
	endMs := end.UnixMilli()

	switch period {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return dateRangeResult{
			Range: types.DateRange{Start: start.UnixMilli(), End: endMs},
			Label: fmt.Sprintf("Today (%s)", start.Format("2006-01-02")),
		}
	case "yesterday":
		y := now.AddDate(0, 0, -1)
		start := time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, now.Location())
		yEnd := time.Date(y.Year(), y.Month(), y.Day(), 23, 59, 59, 999_000_000, now.Location())
		return dateRangeResult{
			Range: types.DateRange{Start: start.UnixMilli(), End: yEnd.UnixMilli()},
			Label: fmt.Sprintf("Yesterday (%s)", start.Format("2006-01-02")),
		}
	case "week":
		start := now.AddDate(0, 0, -7)
		startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		return dateRangeResult{
			Range: types.DateRange{Start: startDay.UnixMilli(), End: endMs},
			Label: "Last 7 Days",
		}
	case "month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return dateRangeResult{
			Range: types.DateRange{Start: start.UnixMilli(), End: endMs},
			Label: now.Format("January 2006"),
		}
	case "30days":
		start := now.AddDate(0, 0, -30)
		startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		return dateRangeResult{
			Range: types.DateRange{Start: startDay.UnixMilli(), End: endMs},
			Label: "Last 30 Days",
		}
	case "all":
		return dateRangeResult{
			Range: types.DateRange{Start: 0, End: endMs},
			Label: "All Time",
		}
	default:
		start := now.AddDate(0, 0, -7)
		startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		return dateRangeResult{
			Range: types.DateRange{Start: startDay.UnixMilli(), End: endMs},
			Label: "Last 7 Days",
		}
	}
}

// --- period data builder (for status/menubar) ---

func buildPeriodData(label string, projects []types.ProjectSummary) menubar.PeriodData {
	type catAccum struct {
		turns, editTurns, oneShotTurns int
		cost                           float64
	}
	type modelAccum struct {
		calls int
		cost  float64
	}

	catTotals := map[types.TaskCategory]*catAccum{}
	modelTotals := map[string]*modelAccum{}
	var totalCost float64
	var totalCalls int
	var inputTokens, outputTokens, cacheRead, cacheWrite int64

	for _, p := range projects {
		totalCost += p.TotalCostUSD
		totalCalls += p.TotalAPICalls
		for _, s := range p.Sessions {
			inputTokens += s.TotalInputTokens
			outputTokens += s.TotalOutputTokens
			cacheRead += s.TotalCacheReadTokens
			cacheWrite += s.TotalCacheWriteTokens
			for cat, d := range s.CategoryBreakdown {
				if catTotals[cat] == nil {
					catTotals[cat] = &catAccum{}
				}
				catTotals[cat].turns += d.Turns
				catTotals[cat].cost += d.CostUSD
				catTotals[cat].editTurns += d.EditTurns
				catTotals[cat].oneShotTurns += d.OneShotTurns
			}
			for m, d := range s.ModelBreakdown {
				if modelTotals[m] == nil {
					modelTotals[m] = &modelAccum{}
				}
				modelTotals[m].calls += d.Calls
				modelTotals[m].cost += d.CostUSD
			}
		}
	}

	// Sort categories by cost desc.
	type catEntry struct {
		cat  types.TaskCategory
		data *catAccum
	}
	catEntries := make([]catEntry, 0, len(catTotals))
	for k, v := range catTotals {
		catEntries = append(catEntries, catEntry{k, v})
	}
	sort.Slice(catEntries, func(i, j int) bool { return catEntries[i].data.cost > catEntries[j].data.cost })

	cats := make([]menubar.CategoryData, 0, len(catEntries))
	for _, e := range catEntries {
		name, ok := types.CategoryLabels[e.cat]
		if !ok {
			name = string(e.cat)
		}
		cats = append(cats, menubar.CategoryData{
			Name:         name,
			Cost:         e.data.cost,
			Turns:        e.data.turns,
			EditTurns:    e.data.editTurns,
			OneShotTurns: e.data.oneShotTurns,
		})
	}

	// Sort models by cost desc.
	type modelEntry struct {
		name string
		data *modelAccum
	}
	modelEntries := make([]modelEntry, 0, len(modelTotals))
	for k, v := range modelTotals {
		modelEntries = append(modelEntries, modelEntry{k, v})
	}
	sort.Slice(modelEntries, func(i, j int) bool { return modelEntries[i].data.cost > modelEntries[j].data.cost })

	mods := make([]menubar.ModelData, 0, len(modelEntries))
	for _, e := range modelEntries {
		mods = append(mods, menubar.ModelData{Name: e.name, Cost: e.data.cost, Calls: e.data.calls})
	}

	return menubar.PeriodData{
		Label:            label,
		Cost:             totalCost,
		Calls:            totalCalls,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
		Categories:       cats,
		Models:           mods,
	}
}

// --- commands ---

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "codeburn",
		Short:   "See where your AI coding tokens go - by task, tool, model, and project",
		Version: version,
	}
	root.AddCommand(
		newReportCmd(),
		newStatusCmd(),
		newTodayCmd(),
		newMonthCmd(),
		newExportCmd(),
		newInstallMenubarCmd(),
		newUninstallMenubarCmd(),
		newCurrencyCmd(),
	)
	return root
}

func newReportCmd() *cobra.Command {
	var period, providerFilter string
	var refresh int

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Interactive usage dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunDashboard(period, providerFilter, refresh)
		},
	}
	cmd.Flags().StringVarP(&period, "period", "p", "week", "Starting period: today, week, 30days, month")
	cmd.Flags().StringVar(&providerFilter, "provider", "all", "Filter by provider: all, claude, codex, cursor")
	cmd.Flags().IntVar(&refresh, "refresh", 0, "Auto-refresh interval in seconds")
	return cmd
}

func newTodayCmd() *cobra.Command {
	var providerFilter string
	var refresh int

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Today's usage dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunDashboard("today", providerFilter, refresh)
		},
	}
	cmd.Flags().StringVar(&providerFilter, "provider", "all", "Filter by provider: all, claude, codex, cursor")
	cmd.Flags().IntVar(&refresh, "refresh", 0, "Auto-refresh interval in seconds")
	return cmd
}

func newMonthCmd() *cobra.Command {
	var providerFilter string
	var refresh int

	cmd := &cobra.Command{
		Use:   "month",
		Short: "This month's usage dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunDashboard("month", providerFilter, refresh)
		},
	}
	cmd.Flags().StringVar(&providerFilter, "provider", "all", "Filter by provider: all, claude, codex, cursor")
	cmd.Flags().IntVar(&refresh, "refresh", 0, "Auto-refresh interval in seconds")
	return cmd
}

func newStatusCmd() *cobra.Command {
	var outputFormat, providerFilter string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Compact status output (today + week + month)",
		RunE: func(cmd *cobra.Command, args []string) error {
			currency.Load()
			if err := models.LoadPricing(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not load pricing: %v\n", err)
			}
			pf := ""
			if providerFilter != "all" {
				pf = providerFilter
			}
			monthDR := getDateRange("month")
			opts := types.ParseOptions{
				DateRange:      &monthDR.Range,
				ProviderFilter: pf,
				ExtractBash:    false,
			}
			monthProjects, err := parser.ParseAllSessions(opts)
			if err != nil {
				return err
			}

			switch outputFormat {
			case "menubar":
				todayDR := getDateRange("today")
				weekDR := getDateRange("week")
				todayProjects := parser.FilterProjectsByDateRange(monthProjects, todayDR.Range)
				weekProjects := parser.FilterProjectsByDateRange(monthProjects, weekDR.Range)

				todayData := buildPeriodData("Today", todayProjects)
				weekData := buildPeriodData("7 Days", weekProjects)
				monthData := buildPeriodData("Month", monthProjects)

				// Per-provider costs for today.
				providerNames := []struct{ id, display string }{
					{"claude", "Claude"},
					{"codex", "Codex"},
					{"cursor", "Cursor"},
				}
				var todayProviders []menubar.ProviderCost
				for _, pn := range providerNames {
					var cost float64
					for _, p := range todayProjects {
						for _, s := range p.Sessions {
							for _, t := range s.Turns {
								for _, c := range t.AssistantCalls {
									if strings.EqualFold(c.Provider, pn.id) {
										cost += c.CostUSD
									}
								}
							}
						}
					}
					if cost > 0 {
						todayProviders = append(todayProviders, menubar.ProviderCost{Name: pn.display, Cost: cost})
					}
				}
				fmt.Println(menubar.RenderMenubarFormat(todayData, weekData, monthData, todayProviders))

			case "json":
				todayDR := getDateRange("today")
				todayProjects := parser.FilterProjectsByDateRange(monthProjects, todayDR.Range)
				var todayCost, monthCost float64
				var todayCalls, monthCalls int
				for _, p := range todayProjects {
					todayCost += p.TotalCostUSD
					todayCalls += p.TotalAPICalls
				}
				for _, p := range monthProjects {
					monthCost += p.TotalCostUSD
					monthCalls += p.TotalAPICalls
				}
				curr := currency.Get()
				round2 := func(v float64) float64 {
					return math.Round(v*curr.Rate*100) / 100
				}
				out, _ := json.Marshal(map[string]any{
					"currency": curr.Code,
					"today":    map[string]any{"cost": round2(todayCost), "calls": todayCalls},
					"month":    map[string]any{"cost": round2(monthCost), "calls": monthCalls},
				})
				fmt.Println(string(out))

			default: // terminal
				fmt.Print(format.RenderStatusBar(monthProjects))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outputFormat, "format", "terminal", "Output format: terminal, menubar, json")
	cmd.Flags().StringVar(&providerFilter, "provider", "all", "Filter by provider: all, claude, codex, cursor")
	return cmd
}

func newExportCmd() *cobra.Command {
	var outputFormat, outputPath, providerFilter string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export usage data to CSV or JSON (includes 1 day, 7 days, 30 days)",
		RunE: func(cmd *cobra.Command, args []string) error {
			currency.Load()
			if err := models.LoadPricing(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not load pricing: %v\n", err)
			}
			pf := ""
			if providerFilter != "all" {
				pf = providerFilter
			}
			thirtyDR := getDateRange("30days")
			opts := types.ParseOptions{
				DateRange:      &thirtyDR.Range,
				ProviderFilter: pf,
				ExtractBash:    true,
			}
			allData, err := parser.ParseAllSessions(opts)
			if err != nil {
				return err
			}

			todayDR := getDateRange("today")
			weekDR := getDateRange("week")
			periods := []export.PeriodExport{
				{Label: "Today", Projects: parser.FilterProjectsByDateRange(allData, todayDR.Range)},
				{Label: "7 Days", Projects: parser.FilterProjectsByDateRange(allData, weekDR.Range)},
				{Label: "30 Days", Projects: allData},
			}

			empty := true
			for _, p := range periods {
				if len(p.Projects) > 0 {
					empty = false
					break
				}
			}
			if empty {
				fmt.Print("\n  No usage data found.\n\n")
				return nil
			}

			today := time.Now().Format("2006-01-02")
			if outputPath == "" {
				outputPath = fmt.Sprintf("codeburn-%s.%s", today, outputFormat)
			}

			var savedPath string
			if outputFormat == "json" {
				savedPath, err = export.ExportJSON(periods, outputPath)
			} else {
				savedPath, err = export.ExportCSV(periods, outputPath)
			}
			if err != nil {
				return err
			}
			fmt.Printf("\n  Exported (Today + 7 Days + 30 Days) to: %s\n\n", savedPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "csv", "Export format: csv, json")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path")
	cmd.Flags().StringVar(&providerFilter, "provider", "all", "Filter by provider: all, claude, codex, cursor")
	return cmd
}

func newInstallMenubarCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install-menubar",
		Short: "Install macOS menu bar plugin (SwiftBar/xbar)",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := menubar.InstallMenubar()
			if err != nil {
				return err
			}
			fmt.Print(result)
			return nil
		},
	}
}

func newUninstallMenubarCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-menubar",
		Short: "Remove macOS menu bar plugin",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := menubar.UninstallMenubar()
			if err != nil {
				return err
			}
			fmt.Print(result)
			return nil
		},
	}
}

func newCurrencyCmd() *cobra.Command {
	var symbol string
	var reset bool

	cmd := &cobra.Command{
		Use:   "currency [code]",
		Short: "Set display currency (e.g. codeburn currency GBP)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if reset {
				cfg := config.Read()
				cfg.Currency = nil
				if err := config.Save(cfg); err != nil {
					return err
				}
				fmt.Print("\n  Currency reset to USD.\n\n")
				return nil
			}

			if len(args) == 0 {
				// Show current currency.
				currency.Load()
				curr := currency.Get()
				if curr.Code == "USD" && curr.Rate == 1 {
					fmt.Println("\n  Currency: USD (default)")
					fmt.Printf("  Config: %s\n\n", config.GetConfigFilePath())
				} else {
					fmt.Printf("\n  Currency: %s\n", curr.Code)
					fmt.Printf("  Symbol: %s\n", curr.Symbol)
					fmt.Printf("  Rate: 1 USD = %g %s\n", curr.Rate, curr.Code)
					fmt.Printf("  Config: %s\n\n", config.GetConfigFilePath())
				}
				return nil
			}

			upperCode := strings.ToUpper(args[0])
			if !currency.IsValidCurrencyCode(upperCode) {
				fmt.Fprintf(os.Stderr, "\n  %q is not a valid ISO 4217 currency code.\n\n", args[0])
				os.Exit(1)
			}

			cfg := config.Read()
			cfg.Currency = &config.CurrencyConfig{Code: upperCode}
			if symbol != "" {
				cfg.Currency.Symbol = symbol
			}
			if err := config.Save(cfg); err != nil {
				return err
			}

			currency.Load()
			curr := currency.Get()
			fmt.Printf("\n  Currency set to %s.\n", upperCode)
			fmt.Printf("  Symbol: %s\n", curr.Symbol)
			fmt.Printf("  Rate: 1 USD = %g %s\n", curr.Rate, upperCode)
			fmt.Printf("  Config saved to %s\n\n", config.GetConfigFilePath())
			return nil
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Override the currency symbol")
	cmd.Flags().BoolVar(&reset, "reset", false, "Reset to USD (removes currency config)")
	return cmd
}

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
