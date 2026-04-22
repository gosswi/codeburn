package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/agentseal/codeburn/internal/currency"
	"github.com/agentseal/codeburn/internal/format"
	"github.com/agentseal/codeburn/internal/types"
)

const (
	colorOrange = "#FF8C42"
	colorDim    = "#555555"
	colorGold   = "#FFD700"
)

var panelColors = map[string]string{
	"overview": "#FF8C42",
	"daily":    "#5B9EF5",
	"project":  "#5BF5A0",
	"model":    "#E05BF5",
	"activity": "#F5C85B",
	"tools":    "#5BF5E0",
	"mcp":      "#F55BE0",
	"bash":     "#F5A05B",
}

var categoryColors = map[types.TaskCategory]string{
	types.CategoryCoding:       "#5B9EF5",
	types.CategoryDebugging:    "#F55B5B",
	types.CategoryFeature:      "#5BF58C",
	types.CategoryRefactoring:  "#F5E05B",
	types.CategoryTesting:      "#E05BF5",
	types.CategoryExploration:  "#5BF5E0",
	types.CategoryPlanning:     "#7B9EF5",
	types.CategoryDelegation:   "#F5C85B",
	types.CategoryGit:          "#CCCCCC",
	types.CategoryBuildDeploy:  "#5BF5A0",
	types.CategoryConversation: "#888888",
	types.CategoryBrainstorm:   "#F55BE0",
	types.CategoryGeneral:      "#666666",
}

var langDisplayNames = map[string]string{
	"javascript": "JavaScript", "typescript": "TypeScript", "python": "Python",
	"rust": "Rust", "go": "Go", "java": "Java", "cpp": "C++", "c": "C",
	"csharp": "C#", "ruby": "Ruby", "php": "PHP", "swift": "Swift",
	"kotlin": "Kotlin", "html": "HTML", "css": "CSS", "scss": "SCSS",
	"json": "JSON", "yaml": "YAML", "sql": "SQL", "shell": "Shell",
	"shellscript": "Shell Script", "bash": "Bash", "typescriptreact": "TSX",
	"javascriptreact": "JSX", "markdown": "Markdown", "dockerfile": "Dockerfile",
	"toml": "TOML",
}

func dimStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim)) }
func goldStyle() lipgloss.Style  { return lipgloss.NewStyle().Foreground(lipgloss.Color(colorGold)) }
func boldStyle() lipgloss.Style  { return lipgloss.NewStyle().Bold(true) }

func panelStyle(color string, width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(color)).
		PaddingLeft(1).PaddingRight(1).
		Width(width)
}

func fit(s string, n int) string {
	rs := []rune(s)
	if len(rs) > n {
		return string(rs[:n])
	}
	return s + strings.Repeat(" ", n-len(rs))
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

// shortProject converts an encoded project dir name to a short display path.
var homeEncoded = func() string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return strings.ReplaceAll(home, "/", "-")
}()

func shortProject(encoded string) string {
	path := strings.TrimPrefix(encoded, "-")

	home := strings.TrimPrefix(homeEncoded, "-")
	if strings.HasPrefix(path, home) {
		path = strings.TrimPrefix(path[len(home):], "-")
	}

	// Strip temp dir prefixes
	for _, prefix := range []string{"private-tmp-"} {
		if strings.HasPrefix(path, prefix) {
			rest := path[len(prefix):]
			// Strip <org>-<env>- pattern (two more segments)
			parts := strings.SplitN(rest, "-", 3)
			if len(parts) == 3 {
				path = parts[2]
			} else {
				path = rest
			}
		}
	}
	path = strings.TrimPrefix(path, "tmp-")

	if path == "" {
		return "home"
	}
	parts := strings.Split(path, "-")
	parts = filterEmpty(parts)
	if len(parts) <= 3 {
		return strings.Join(parts, "/")
	}
	return strings.Join(parts[len(parts)-3:], "/")
}

func filterEmpty(ss []string) []string {
	out := ss[:0]
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// renderPanel wraps content in a bordered panel.
func renderPanel(title, color string, width int, lines []string) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(title)
	content := header + "\n" + strings.Join(lines, "\n")
	return panelStyle(color, width).Render(content)
}

// renderOverview renders the overview panel.
func renderOverview(projects []types.ProjectSummary, label string, width int) string {
	var totalCost float64
	var totalCalls, totalSessions int
	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int64

	for _, p := range projects {
		totalCost += p.TotalCostUSD
		totalCalls += p.TotalAPICalls
		totalSessions += len(p.Sessions)
		for _, s := range p.Sessions {
			totalInput += s.TotalInputTokens
			totalOutput += s.TotalOutputTokens
			totalCacheRead += s.TotalCacheReadTokens
			totalCacheWrite += s.TotalCacheWriteTokens
		}
	}

	var cacheHitPct float64
	if totalInput+totalCacheRead > 0 {
		cacheHitPct = float64(totalCacheRead) / float64(totalInput+totalCacheRead) * 100
	}

	line1 := boldStyle().Foreground(lipgloss.Color(colorOrange)).Render("CodeBurn") +
		dimStyle().Render("  "+label)
	line2 := goldStyle().Bold(true).Render(currency.FormatCost(totalCost)) +
		dimStyle().Render(" cost   ") +
		boldStyle().Render(fmt.Sprintf("%d", totalCalls)) +
		dimStyle().Render(" calls   ") +
		boldStyle().Render(fmt.Sprintf("%d", totalSessions)) +
		dimStyle().Render(" sessions   ") +
		boldStyle().Render(fmt.Sprintf("%.0f%%", cacheHitPct)) +
		dimStyle().Render(" cache hit")
	line3 := dimStyle().Render(fmt.Sprintf("%s in   %s out   %s cached   %s written",
		format.FormatTokens(totalInput), format.FormatTokens(totalOutput),
		format.FormatTokens(totalCacheRead), format.FormatTokens(totalCacheWrite)))

	return renderPanel("Overview", panelColors["overview"], width, []string{line1, line2, line3})
}

func renderDailyActivity(projects []types.ProjectSummary, days, pw, bw int) string {
	dailyCosts := map[string]float64{}
	dailyCalls := map[string]int{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for _, t := range s.Turns {
				if t.Timestamp == "" || len(t.Timestamp) < 10 {
					continue
				}
				day := t.Timestamp[:10]
				for _, c := range t.AssistantCalls {
					dailyCosts[day] += c.CostUSD
					dailyCalls[day]++
				}
			}
		}
	}

	allDays := make([]string, 0, len(dailyCosts))
	for d := range dailyCosts {
		allDays = append(allDays, d)
	}
	sort.Strings(allDays)
	if len(allDays) > days {
		allDays = allDays[len(allDays)-days:]
	}

	var maxCost float64
	for _, d := range allDays {
		if dailyCosts[d] > maxCost {
			maxCost = dailyCosts[d]
		}
	}
	maxCostInt := int(maxCost * 10000) // scale to avoid float truncation
	if maxCostInt == 0 {
		maxCostInt = 1
	}

	header := dimStyle().Render(strings.Repeat(" ", 6+bw) + padLeft("cost", 8) + padLeft("calls", 6))
	lines := []string{header}
	for _, day := range allDays {
		cost := dailyCosts[day]
		calls := dailyCalls[day]
		costInt := int(cost * 10000)
		bar := HBar(bw, costInt, maxCostInt)
		line := dimStyle().Render(day[5:]+" ") + bar +
			goldStyle().Render(padLeft(currency.FormatCost(cost), 8)) +
			fmt.Sprintf("%6d", calls)
		lines = append(lines, line)
	}
	return renderPanel("Daily Activity", panelColors["daily"], pw, lines)
}

func renderProjectBreakdown(projects []types.ProjectSummary, pw, bw int) string {
	var maxCost float64
	for _, p := range projects {
		if p.TotalCostUSD > maxCost {
			maxCost = p.TotalCostUSD
		}
	}
	maxCostInt := int(maxCost * 10000)
	if maxCostInt == 0 {
		maxCostInt = 1
	}

	nw := pw - bw - 23
	if nw < 8 {
		nw = 8
	}

	header := dimStyle().Render(strings.Repeat(" ", bw+1+nw) + padLeft("cost", 8) + padLeft("sess", 6))
	lines := []string{header}
	limit := projects
	if len(limit) > 8 {
		limit = limit[:8]
	}
	for _, p := range limit {
		costInt := int(p.TotalCostUSD * 10000)
		bar := HBar(bw, costInt, maxCostInt)
		line := bar + dimStyle().Render(" "+fit(shortProject(p.Project), nw)) +
			goldStyle().Render(padLeft(currency.FormatCost(p.TotalCostUSD), 8)) +
			fmt.Sprintf("%6d", len(p.Sessions))
		lines = append(lines, line)
	}
	return renderPanel("By Project", panelColors["project"], pw, lines)
}

func renderModelBreakdown(projects []types.ProjectSummary, pw, bw int) string {
	type modelData struct {
		calls   int
		costUSD float64
	}
	modelTotals := map[string]*modelData{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for m, d := range s.ModelBreakdown {
				if modelTotals[m] == nil {
					modelTotals[m] = &modelData{}
				}
				modelTotals[m].calls += d.Calls
				modelTotals[m].costUSD += d.CostUSD
			}
		}
	}
	type entry struct {
		name string
		data *modelData
	}
	sorted := make([]entry, 0, len(modelTotals))
	for k, v := range modelTotals {
		sorted = append(sorted, entry{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].data.costUSD > sorted[j].data.costUSD })

	var maxCost float64
	if len(sorted) > 0 {
		maxCost = sorted[0].data.costUSD
	}
	maxCostInt := int(maxCost * 10000)
	if maxCostInt == 0 {
		maxCostInt = 1
	}

	nw := pw - bw - 25
	if nw < 6 {
		nw = 6
	}

	header := dimStyle().Render(strings.Repeat(" ", bw+1+nw) + padLeft("cost", 8) + padLeft("calls", 7))
	lines := []string{header}
	for _, e := range sorted {
		costInt := int(e.data.costUSD * 10000)
		bar := HBar(bw, costInt, maxCostInt)
		line := bar + " " + fit(e.name, nw) +
			goldStyle().Render(padLeft(currency.FormatCost(e.data.costUSD), 8)) +
			fmt.Sprintf("%7d", e.data.calls)
		lines = append(lines, line)
	}
	return renderPanel("By Model", panelColors["model"], pw, lines)
}

func renderActivityBreakdown(projects []types.ProjectSummary, pw, bw int) string {
	type catData struct {
		turns, editTurns, oneShotTurns int
		costUSD                        float64
	}
	catTotals := map[types.TaskCategory]*catData{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for cat, d := range s.CategoryBreakdown {
				if catTotals[cat] == nil {
					catTotals[cat] = &catData{}
				}
				catTotals[cat].turns += d.Turns
				catTotals[cat].costUSD += d.CostUSD
				catTotals[cat].editTurns += d.EditTurns
				catTotals[cat].oneShotTurns += d.OneShotTurns
			}
		}
	}
	type entry struct {
		cat  types.TaskCategory
		data *catData
	}
	sorted := make([]entry, 0, len(catTotals))
	for k, v := range catTotals {
		sorted = append(sorted, entry{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].data.costUSD > sorted[j].data.costUSD })

	var maxCost float64
	if len(sorted) > 0 {
		maxCost = sorted[0].data.costUSD
	}
	maxCostInt := int(maxCost * 10000)
	if maxCostInt == 0 {
		maxCostInt = 1
	}

	header := dimStyle().Render(strings.Repeat(" ", bw+14) + padLeft("cost", 8) + padLeft("turns", 6) + padLeft("1-shot", 7))
	lines := []string{header}
	for _, e := range sorted {
		costInt := int(e.data.costUSD * 10000)
		bar := HBar(bw, costInt, maxCostInt)

		oneShotStr := "-"
		if e.data.editTurns > 0 {
			oneShotStr = fmt.Sprintf("%d%%", int(float64(e.data.oneShotTurns)/float64(e.data.editTurns)*100+0.5))
		}

		catColor := categoryColors[e.cat]
		if catColor == "" {
			catColor = "#666666"
		}
		label, ok := types.CategoryLabels[e.cat]
		if !ok {
			label = string(e.cat)
		}

		oneShotColor := colorOrange
		if e.data.editTurns == 0 {
			oneShotColor = colorDim
		} else if oneShotStr == "100%" {
			oneShotColor = "#5BF58C"
		}

		line := bar +
			lipgloss.NewStyle().Foreground(lipgloss.Color(catColor)).Render(" "+fit(label, 13)) +
			goldStyle().Render(padLeft(currency.FormatCost(e.data.costUSD), 8)) +
			fmt.Sprintf("%6d", e.data.turns) +
			lipgloss.NewStyle().Foreground(lipgloss.Color(oneShotColor)).Render(padLeft(oneShotStr, 7))
		lines = append(lines, line)
	}
	return renderPanel("By Activity", panelColors["activity"], pw, lines)
}

func renderToolBreakdown(projects []types.ProjectSummary, pw, bw int, title, filterPrefix string) string {
	toolTotals := map[string]int{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for tool, d := range s.ToolBreakdown {
				if filterPrefix != "" {
					if !strings.HasPrefix(tool, filterPrefix) {
						continue
					}
				} else {
					if strings.HasPrefix(tool, "lang:") {
						continue
					}
				}
				toolTotals[tool] += d.Calls
			}
		}
	}
	type entry struct {
		name  string
		calls int
	}
	sorted := make([]entry, 0, len(toolTotals))
	for k, v := range toolTotals {
		sorted = append(sorted, entry{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].calls > sorted[j].calls })

	var maxCalls int
	if len(sorted) > 0 {
		maxCalls = sorted[0].calls
	}
	if maxCalls == 0 {
		maxCalls = 1
	}

	nw := pw - bw - 15
	if nw < 6 {
		nw = 6
	}

	if title == "" {
		title = "Core Tools"
	}

	header := dimStyle().Render(strings.Repeat(" ", bw+1+nw) + padLeft("calls", 7))
	lines := []string{header}
	limit := sorted
	if len(limit) > 10 {
		limit = limit[:10]
	}
	for _, e := range limit {
		raw := e.name
		if filterPrefix != "" {
			raw = strings.TrimPrefix(raw, filterPrefix)
		}
		display := raw
		if filterPrefix != "" {
			if d, ok := langDisplayNames[raw]; ok {
				display = d
			}
		}
		bar := HBar(bw, e.calls, maxCalls)
		line := bar + " " + fit(display, nw) + fmt.Sprintf("%7d", e.calls)
		lines = append(lines, line)
	}
	return renderPanel(title, panelColors["tools"], pw, lines)
}

func renderMcpBreakdown(projects []types.ProjectSummary, pw, bw int) string {
	mcpTotals := map[string]int{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			for server, d := range s.McpBreakdown {
				mcpTotals[server] += d.Calls
			}
		}
	}

	type entry struct {
		name  string
		calls int
	}
	sorted := make([]entry, 0, len(mcpTotals))
	for k, v := range mcpTotals {
		sorted = append(sorted, entry{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].calls > sorted[j].calls })

	if len(sorted) == 0 {
		return renderPanel("MCP Servers", panelColors["mcp"], pw, []string{dimStyle().Render("No MCP usage")})
	}

	maxCalls := sorted[0].calls
	if maxCalls == 0 {
		maxCalls = 1
	}

	nw := pw - bw - 15
	if nw < 6 {
		nw = 6
	}

	header := dimStyle().Render(strings.Repeat(" ", bw+1+nw) + padLeft("calls", 6))
	lines := []string{header}
	limit := sorted
	if len(limit) > 8 {
		limit = limit[:8]
	}
	for _, e := range limit {
		bar := HBar(bw, e.calls, maxCalls)
		line := bar + " " + fit(e.name, nw) + fmt.Sprintf("%6d", e.calls)
		lines = append(lines, line)
	}
	return renderPanel("MCP Servers", panelColors["mcp"], pw, lines)
}

func renderBashBreakdown(projects []types.ProjectSummary, pw, bw int) string {
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
	sorted := make([]entry, 0, len(bashTotals))
	for k, v := range bashTotals {
		sorted = append(sorted, entry{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].calls > sorted[j].calls })

	if len(sorted) == 0 {
		return renderPanel("Shell Commands", panelColors["bash"], pw, []string{dimStyle().Render("No shell commands")})
	}

	maxCalls := sorted[0].calls
	if maxCalls == 0 {
		maxCalls = 1
	}

	nw := pw - bw - 15
	if nw < 6 {
		nw = 6
	}

	header := dimStyle().Render(strings.Repeat(" ", bw+1+nw) + padLeft("calls", 7))
	lines := []string{header}
	limit := sorted
	if len(limit) > 10 {
		limit = limit[:10]
	}
	for _, e := range limit {
		bar := HBar(bw, e.calls, maxCalls)
		line := bar + " " + fit(e.cmd, nw) + fmt.Sprintf("%7d", e.calls)
		lines = append(lines, line)
	}
	return renderPanel("Shell Commands", panelColors["bash"], pw, lines)
}

// DashboardContent renders all panels for the given projects and period.
func DashboardContent(projects []types.ProjectSummary, periodLabel string, layout Layout, isCursor bool) string {
	if len(projects) == 0 {
		return renderPanel("CodeBurn", panelColors["overview"], layout.DashWidth,
			[]string{dimStyle().Render("No usage data found for " + periodLabel + ".")})
	}

	pw := layout.HalfWidth
	bw := layout.BarWidth

	days := 14
	switch periodLabel {
	case "30 Days", "This Month":
		days = 31
	}

	overview := renderOverview(projects, periodLabel, layout.DashWidth)

	daily := renderDailyActivity(projects, days, pw, bw)
	project := renderProjectBreakdown(projects, pw, bw)
	activity := renderActivityBreakdown(projects, pw, bw)
	model := renderModelBreakdown(projects, pw, bw)

	var rows []string
	rows = append(rows, overview)

	if layout.Wide {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, daily, project))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, activity, model))
	} else {
		rows = append(rows, daily, project, activity, model)
	}

	if isCursor {
		rows = append(rows, renderToolBreakdown(projects, layout.DashWidth, bw, "Languages", "lang:"))
	} else {
		tools := renderToolBreakdown(projects, pw, bw, "Core Tools", "")
		bash := renderBashBreakdown(projects, pw, bw)
		if layout.Wide {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, tools, bash))
		} else {
			rows = append(rows, tools, bash)
		}
		rows = append(rows, renderMcpBreakdown(projects, layout.DashWidth, bw))
	}

	return strings.Join(rows, "\n")
}
