package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentseal/codeburn/internal/currency"
	"github.com/agentseal/codeburn/internal/types"
)

// PeriodExport groups projects under a named reporting period.
type PeriodExport struct {
	Label    string
	Projects []types.ProjectSummary
}

// --- CSV helpers ---

func escCsv(s string) string {
	// Formula injection protection
	sanitized := s
	if len(sanitized) > 0 && strings.ContainsAny(string(sanitized[0]), "=+-@") {
		sanitized = "'" + sanitized
	}
	if strings.ContainsAny(sanitized, ",\"\n") {
		sanitized = `"` + strings.ReplaceAll(sanitized, `"`, `""`) + `"`
	}
	return sanitized
}

func rowsToCsv(rows []map[string]string, headers []string) string {
	if len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, len(rows)+1)
	hEsc := make([]string, len(headers))
	for i, h := range headers {
		hEsc[i] = escCsv(h)
	}
	lines = append(lines, strings.Join(hEsc, ","))
	for _, row := range rows {
		cols := make([]string, len(headers))
		for i, h := range headers {
			cols[i] = escCsv(row[h])
		}
		lines = append(lines, strings.Join(cols, ","))
	}
	return strings.Join(lines, "\n")
}

func fmtF(f float64) string { return fmt.Sprintf("%g", f) }
func fmtI(i int) string     { return fmt.Sprintf("%d", i) }

// --- row builders ---

func buildDailyRows(projects []types.ProjectSummary) ([]map[string]string, []string) {
	type day struct {
		cost, input, output, cacheRead, cacheWrite float64
		calls                                       int
	}
	daily := map[string]*day{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for _, t := range s.Turns {
				if t.Timestamp == "" {
					continue
				}
				d := ""
				if len(t.Timestamp) >= 10 {
					d = t.Timestamp[:10]
				}
				if _, ok := daily[d]; !ok {
					daily[d] = &day{}
				}
				for _, c := range t.AssistantCalls {
					daily[d].cost += c.CostUSD
					daily[d].calls++
					daily[d].input += float64(c.Usage.InputTokens)
					daily[d].output += float64(c.Usage.OutputTokens)
					daily[d].cacheRead += float64(c.Usage.CacheReadInputTokens)
					daily[d].cacheWrite += float64(c.Usage.CacheCreationInputTokens)
				}
			}
		}
	}
	keys := make([]string, 0, len(daily))
	for k := range daily {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	costHeader := currency.GetCostColumnHeader()
	headers := []string{"Date", costHeader, "API Calls", "Input Tokens", "Output Tokens", "Cache Read Tokens", "Cache Write Tokens"}
	rows := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		v := daily[k]
		rows = append(rows, map[string]string{
			"Date":               k,
			costHeader:           fmtF(currency.ConvertCost(v.cost)),
			"API Calls":          fmtI(v.calls),
			"Input Tokens":       fmtF(v.input),
			"Output Tokens":      fmtF(v.output),
			"Cache Read Tokens":  fmtF(v.cacheRead),
			"Cache Write Tokens": fmtF(v.cacheWrite),
		})
	}
	return rows, headers
}

func buildActivityRows(projects []types.ProjectSummary) ([]map[string]string, []string) {
	type cat struct {
		turns int
		cost  float64
	}
	catTotals := map[types.TaskCategory]*cat{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for c, d := range s.CategoryBreakdown {
				if _, ok := catTotals[c]; !ok {
					catTotals[c] = &cat{}
				}
				catTotals[c].turns += d.Turns
				catTotals[c].cost += d.CostUSD
			}
		}
	}
	type entry struct {
		cat  types.TaskCategory
		data *cat
	}
	entries := make([]entry, 0, len(catTotals))
	for k, v := range catTotals {
		entries = append(entries, entry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].data.cost > entries[j].data.cost })

	costHeader := currency.GetCostColumnHeader()
	headers := []string{"Activity", costHeader, "Turns"}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		label, ok := types.CategoryLabels[e.cat]
		if !ok {
			label = string(e.cat)
		}
		rows = append(rows, map[string]string{
			"Activity": label,
			costHeader: fmtF(currency.ConvertCost(e.data.cost)),
			"Turns":    fmtI(e.data.turns),
		})
	}
	return rows, headers
}

func buildModelRows(projects []types.ProjectSummary) ([]map[string]string, []string) {
	type model struct {
		calls  int
		cost   float64
		input  int64
		output int64
	}
	modelTotals := map[string]*model{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for m, d := range s.ModelBreakdown {
				if _, ok := modelTotals[m]; !ok {
					modelTotals[m] = &model{}
				}
				modelTotals[m].calls += d.Calls
				modelTotals[m].cost += d.CostUSD
				modelTotals[m].input += d.Tokens.InputTokens
				modelTotals[m].output += d.Tokens.OutputTokens
			}
		}
	}
	type entry struct {
		name string
		data *model
	}
	entries := make([]entry, 0, len(modelTotals))
	for k, v := range modelTotals {
		entries = append(entries, entry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].data.cost > entries[j].data.cost })

	costHeader := currency.GetCostColumnHeader()
	headers := []string{"Model", costHeader, "API Calls", "Input Tokens", "Output Tokens"}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]string{
			"Model":         e.name,
			costHeader:      fmtF(currency.ConvertCost(e.data.cost)),
			"API Calls":     fmtI(e.data.calls),
			"Input Tokens":  fmt.Sprintf("%d", e.data.input),
			"Output Tokens": fmt.Sprintf("%d", e.data.output),
		})
	}
	return rows, headers
}

func buildToolRows(projects []types.ProjectSummary) ([]map[string]string, []string) {
	toolTotals := map[string]int{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for t, d := range s.ToolBreakdown {
				toolTotals[t] += d.Calls
			}
		}
	}
	type entry struct {
		name  string
		calls int
	}
	entries := make([]entry, 0, len(toolTotals))
	for k, v := range toolTotals {
		entries = append(entries, entry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].calls > entries[j].calls })

	headers := []string{"Tool", "Calls"}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]string{"Tool": e.name, "Calls": fmtI(e.calls)})
	}
	return rows, headers
}

func buildBashRows(projects []types.ProjectSummary) ([]map[string]string, []string) {
	bashTotals := map[string]int{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for cmd, d := range s.BashBreakdown {
				bashTotals[cmd] += d.Calls
			}
		}
	}
	type entry struct {
		cmd   string
		calls int
	}
	entries := make([]entry, 0, len(bashTotals))
	for k, v := range bashTotals {
		entries = append(entries, entry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].calls > entries[j].calls })

	headers := []string{"Command", "Calls"}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]string{"Command": e.cmd, "Calls": fmtI(e.calls)})
	}
	return rows, headers
}

func buildProjectRows(projects []types.ProjectSummary) ([]map[string]string, []string) {
	costHeader := currency.GetCostColumnHeader()
	headers := []string{"Project", costHeader, "API Calls", "Sessions"}
	rows := make([]map[string]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, map[string]string{
			"Project":   p.ProjectPath,
			costHeader:  fmtF(currency.ConvertCost(p.TotalCostUSD)),
			"API Calls": fmtI(p.TotalAPICalls),
			"Sessions":  fmtI(len(p.Sessions)),
		})
	}
	return rows, headers
}

func buildSummaryRow(period PeriodExport) map[string]string {
	var cost float64
	calls, sessions := 0, 0
	for _, p := range period.Projects {
		cost += p.TotalCostUSD
		calls += p.TotalAPICalls
		sessions += len(p.Sessions)
	}
	costHeader := currency.GetCostColumnHeader()
	return map[string]string{
		"Period":    period.Label,
		costHeader:  fmtF(currency.ConvertCost(cost)),
		"API Calls": fmtI(calls),
		"Sessions":  fmtI(sessions),
	}
}

func selectAllProjects(periods []PeriodExport) []types.ProjectSummary {
	if len(periods) == 0 {
		return nil
	}
	return periods[len(periods)-1].Projects
}

// ExportCSV writes a multi-section CSV report to outputPath and returns the resolved path.
func ExportCSV(periods []PeriodExport, outputPath string) (string, error) {
	allProjects := selectAllProjects(periods)

	var parts []string

	// Summary section
	costHeader := currency.GetCostColumnHeader()
	summaryHeaders := []string{"Period", costHeader, "API Calls", "Sessions"}
	summaryRows := make([]map[string]string, 0, len(periods))
	for _, p := range periods {
		summaryRows = append(summaryRows, buildSummaryRow(p))
	}
	parts = append(parts, "# Summary")
	parts = append(parts, rowsToCsv(summaryRows, summaryHeaders))
	parts = append(parts, "")

	for _, period := range periods {
		rows, headers := buildDailyRows(period.Projects)
		parts = append(parts, "# Daily - "+period.Label)
		parts = append(parts, rowsToCsv(rows, headers))
		parts = append(parts, "")

		rows, headers = buildActivityRows(period.Projects)
		parts = append(parts, "# Activity - "+period.Label)
		parts = append(parts, rowsToCsv(rows, headers))
		parts = append(parts, "")

		rows, headers = buildModelRows(period.Projects)
		parts = append(parts, "# Models - "+period.Label)
		parts = append(parts, rowsToCsv(rows, headers))
		parts = append(parts, "")
	}

	rows, headers := buildToolRows(allProjects)
	parts = append(parts, "# Tools - All")
	parts = append(parts, rowsToCsv(rows, headers))
	parts = append(parts, "")

	rows, headers = buildBashRows(allProjects)
	parts = append(parts, "# Shell Commands - All")
	parts = append(parts, rowsToCsv(rows, headers))
	parts = append(parts, "")

	rows, headers = buildProjectRows(allProjects)
	parts = append(parts, "# Projects - All")
	parts = append(parts, rowsToCsv(rows, headers))
	parts = append(parts, "")

	fullPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", err
	}
	content := strings.Join(parts, "\n")
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fullPath, nil
}

// ExportJSON writes a structured JSON report to outputPath and returns the resolved path.
func ExportJSON(periods []PeriodExport, outputPath string) (string, error) {
	allProjects := selectAllProjects(periods)

	periodData := map[string]any{}
	for _, period := range periods {
		summaryRows, _ := buildDailyRows(period.Projects)
		actRows, _ := buildActivityRows(period.Projects)
		modelRows, _ := buildModelRows(period.Projects)
		periodData[period.Label] = map[string]any{
			"summary":  buildSummaryRow(period),
			"daily":    summaryRows,
			"activity": actRows,
			"models":   modelRows,
		}
	}

	toolRows, _ := buildToolRows(allProjects)
	bashRows, _ := buildBashRows(allProjects)
	projectRows, _ := buildProjectRows(allProjects)

	data := map[string]any{
		"generated":     time.Now().UTC().Format(time.RFC3339),
		"periods":       periodData,
		"tools":         toolRows,
		"shellCommands": bashRows,
		"projects":      projectRows,
	}

	fullPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, out, 0o644); err != nil {
		return "", err
	}
	return fullPath, nil
}
