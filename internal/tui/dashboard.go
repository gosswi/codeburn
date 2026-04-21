package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/agentseal/codeburn/internal/currency"
	"github.com/agentseal/codeburn/internal/models"
	"github.com/agentseal/codeburn/internal/parser"
	"github.com/agentseal/codeburn/internal/types"
)

// ParsePeriod converts a period string to a Period constant.
func ParsePeriod(s string) Period {
	switch s {
	case "today":
		return PeriodToday
	case "week":
		return PeriodWeek
	case "30days":
		return Period30Days
	case "month":
		return PeriodMonth
	default:
		return PeriodWeek
	}
}

// periodDateRange returns the DateRange for a Period.
func periodDateRange(p Period) types.DateRange {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999_000_000, now.Location())
	endMs := end.UnixMilli()

	switch p {
	case PeriodToday:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return types.DateRange{Start: start.UnixMilli(), End: endMs}
	case PeriodWeek:
		d := now.AddDate(0, 0, -7)
		start := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return types.DateRange{Start: start.UnixMilli(), End: endMs}
	case Period30Days:
		d := now.AddDate(0, 0, -30)
		start := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return types.DateRange{Start: start.UnixMilli(), End: endMs}
	case PeriodMonth:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return types.DateRange{Start: start.UnixMilli(), End: endMs}
	}
	// default: week
	d := now.AddDate(0, 0, -7)
	start := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
	return types.DateRange{Start: start.UnixMilli(), End: endMs}
}

// RunDashboard starts the interactive TUI or renders a single static frame for non-TTY.
func RunDashboard(periodStr, provider string, refreshSeconds int) error {
	currency.Load()
	if err := models.LoadPricing(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load pricing: %v\n", err)
	}

	period := ParsePeriod(periodStr)
	dr := periodDateRange(period)
	pf := ""
	if provider != "all" {
		pf = provider
	}
	opts := types.ParseOptions{
		DateRange:      &dr,
		ProviderFilter: pf,
		ExtractBash:    true,
	}
	projects, err := parser.ParseAllSessionsCached(opts)
	if err != nil {
		return err
	}

	termWidth := 80
	if w := os.Getenv("COLUMNS"); w != "" {
		fmt.Sscanf(w, "%d", &termWidth)
	}

	isTTY := isatty()

	m := NewModel(projects, period, provider, termWidth, refreshSeconds)

	if !isTTY {
		// Non-TTY: render a single static frame.
		layout := GetLayout(termWidth)
		label := periodLabels[period]
		isCursor := provider == "cursor"
		fmt.Println(renderTabs(period, provider, false, layout.DashWidth))
		fmt.Println(DashboardContent(projects, label, layout, isCursor))
		return nil
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// isatty returns true when both stdin and stdout are connected to a terminal.
func isatty() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
