package format

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentseal/codeburn/internal/currency"
	"github.com/agentseal/codeburn/internal/types"
)

// FormatCost delegates to currency.FormatCost.
func FormatCost(costUSD float64) string {
	return currency.FormatCost(costUSD)
}

// FormatTokens formats a token count with K/M suffix.
func FormatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// RenderStatusBar renders a two-line status summary (today and month costs/calls).
func RenderStatusBar(projects []types.ProjectSummary) string {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	monthStart := now.Format("2006-01") + "-01"

	var todayCost, monthCost float64
	var todayCalls, monthCalls int

	for _, p := range projects {
		for _, s := range p.Sessions {
			for _, t := range s.Turns {
				if t.Timestamp == "" {
					continue
				}
				day := ""
				if len(t.Timestamp) >= 10 {
					day = t.Timestamp[:10]
				}
				var turnCost float64
				for _, c := range t.AssistantCalls {
					turnCost += c.CostUSD
				}
				turnCalls := len(t.AssistantCalls)
				if day == today {
					todayCost += turnCost
					todayCalls += turnCalls
				}
				if day >= monthStart {
					monthCost += turnCost
					monthCalls += turnCalls
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Today  %s  %d calls    Month  %s  %d calls",
		FormatCost(todayCost), todayCalls,
		FormatCost(monthCost), monthCalls,
	))
	sb.WriteString("\n\n")
	return sb.String()
}
