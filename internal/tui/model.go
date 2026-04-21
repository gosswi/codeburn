package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/agentseal/codeburn/internal/parser"
	"github.com/agentseal/codeburn/internal/types"
)

// Period represents one of the four time windows.
type Period int

const (
	PeriodToday Period = iota
	PeriodWeek
	Period30Days
	PeriodMonth
)

var periodLabels = map[Period]string{
	PeriodToday:  "Today",
	PeriodWeek:   "7 Days",
	Period30Days:  "30 Days",
	PeriodMonth:  "This Month",
}

var periodKeys = []Period{PeriodToday, PeriodWeek, Period30Days, PeriodMonth}

// Model is the Bubbletea model for the interactive dashboard.
type Model struct {
	period            Period
	projects          []types.ProjectSummary
	loading           bool
	err               error
	activeProvider    string
	detectedProviders []string
	termWidth         int
	refreshSeconds    int
	reloadPending     bool
}

type (
	projectsLoaded struct {
		projects []types.ProjectSummary
		err      error
	}
	providersDetected []string
	debounceElapsed   struct{}
	refreshTick       struct{}
)

// NewModel creates a new dashboard Model.
func NewModel(initialProjects []types.ProjectSummary, initialPeriod Period, provider string, termWidth, refreshSeconds int) Model {
	if termWidth < 40 {
		termWidth = 80
	}
	return Model{
		period:         initialPeriod,
		projects:       initialProjects,
		activeProvider: provider,
		termWidth:      termWidth,
		refreshSeconds: refreshSeconds,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		detectProvidersCmd(),
	}
	if m.refreshSeconds > 0 {
		cmds = append(cmds, scheduleRefresh(m.refreshSeconds))
	}
	return tea.Batch(cmds...)
}

func detectProvidersCmd() tea.Cmd {
	return func() tea.Msg {
		// Discover which providers have data.
		var found []string
		providerNames := []string{"claude", "codex", "cursor"}
		for _, name := range providerNames {
			opts := types.ParseOptions{ProviderFilter: name}
			projects, err := parser.ParseAllSessions(opts)
			if err == nil && len(projects) > 0 {
				found = append(found, name)
			}
		}
		return providersDetected(found)
	}
}

func loadDataCmd(period Period, provider string) tea.Cmd {
	return func() tea.Msg {
		dr := periodToDateRange(period)
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
		return projectsLoaded{projects: projects, err: err}
	}
}

func scheduleDebounce() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg {
		return debounceElapsed{}
	})
}

func scheduleRefresh(seconds int) tea.Cmd {
	return tea.Tick(time.Duration(seconds)*time.Second, func(time.Time) tea.Msg {
		return refreshTick{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "left", "right", "tab":
			idx := periodIndex(m.period)
			if msg.String() == "left" {
				idx = (idx - 1 + len(periodKeys)) % len(periodKeys)
			} else {
				idx = (idx + 1) % len(periodKeys)
			}
			m.period = periodKeys[idx]
			m.reloadPending = true
			m.loading = true
			return m, scheduleDebounce()

		case "1":
			return m.switchPeriodImmediate(PeriodToday)
		case "2":
			return m.switchPeriodImmediate(PeriodWeek)
		case "3":
			return m.switchPeriodImmediate(Period30Days)
		case "4":
			return m.switchPeriodImmediate(PeriodMonth)

		case "p":
			if len(m.detectedProviders) > 1 {
				options := append([]string{"all"}, m.detectedProviders...)
				idx := 0
				for i, p := range options {
					if p == m.activeProvider {
						idx = i
						break
					}
				}
				m.activeProvider = options[(idx+1)%len(options)]
				m.loading = true
				return m, loadDataCmd(m.period, m.activeProvider)
			}
		}

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width

	case debounceElapsed:
		if m.reloadPending {
			m.reloadPending = false
			return m, loadDataCmd(m.period, m.activeProvider)
		}

	case projectsLoaded:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.projects = msg.projects
		}
		if m.refreshSeconds > 0 {
			return m, scheduleRefresh(m.refreshSeconds)
		}

	case providersDetected:
		m.detectedProviders = []string(msg)

	case refreshTick:
		return m, loadDataCmd(m.period, m.activeProvider)
	}

	return m, nil
}

func (m Model) switchPeriodImmediate(p Period) (tea.Model, tea.Cmd) {
	m.period = p
	m.reloadPending = false
	m.loading = true
	return m, loadDataCmd(p, m.activeProvider)
}

func periodIndex(p Period) int {
	for i, k := range periodKeys {
		if k == p {
			return i
		}
	}
	return 0
}

func (m Model) View() string {
	layout := GetLayout(m.termWidth)
	label := periodLabels[m.period]
	isCursor := m.activeProvider == "cursor"
	multiProvider := len(m.detectedProviders) > 1

	tabs := renderTabs(m.period, m.activeProvider, multiProvider, layout.DashWidth)
	statusBar := renderStatusBar(layout.DashWidth, multiProvider)

	if m.loading {
		loadingPanel := renderPanel("CodeBurn", panelColors["overview"], layout.DashWidth,
			[]string{dimStyle().Render("Loading " + label + "...")})
		return tabs + "\n" + loadingPanel + "\n" + statusBar
	}

	content := DashboardContent(m.projects, label, layout, isCursor)
	return tabs + "\n" + content + "\n" + statusBar
}

func renderTabs(active Period, provider string, multiProvider bool, width int) string {
	var parts []string
	for _, p := range periodKeys {
		label := periodLabels[p]
		if p == active {
			parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("[ "+label+" ]"))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim)).Render("  "+label+"  "))
		}
	}

	left := strings.Join(parts, " ")
	if !multiProvider {
		return lipgloss.NewStyle().PaddingLeft(1).Render(left)
	}

	providerColors := map[string]string{
		"all": "#FF8C42", "claude": "#FF8C42", "codex": "#5BF5A0", "cursor": "#00B4D8",
	}
	providerDisplay := map[string]string{
		"all": "All", "claude": "Claude", "codex": "Codex", "cursor": "Cursor",
	}
	col := providerColors[provider]
	if col == "" {
		col = colorOrange
	}
	disp := providerDisplay[provider]
	if disp == "" {
		disp = provider
	}

	right := dimStyle().Render("|  ") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("[p]") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(col)).Render(" " + disp)

	// Pad left to full width
	leftWidth := lipgloss.Width(left) + 2 // +2 for paddingLeft=1
	rightWidth := lipgloss.Width(right)
	padding := width - leftWidth - rightWidth - 2
	if padding < 0 {
		padding = 0
	}
	return lipgloss.NewStyle().PaddingLeft(1).Render(left) +
		strings.Repeat(" ", padding) + right
}

func renderStatusBar(width int, showProvider bool) string {
	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("<") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Render(">") +
		dimStyle().Render(" switch   ") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("q") +
		dimStyle().Render(" quit   ") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("1") +
		dimStyle().Render(" today   ") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("2") +
		dimStyle().Render(" week   ") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("3") +
		dimStyle().Render(" 30 days   ") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("4") +
		dimStyle().Render(" month")

	if showProvider {
		content += dimStyle().Render("   ") +
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrange)).Render("p") +
			dimStyle().Render(" provider")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorDim)).
		Width(width).
		Align(lipgloss.Center).
		PaddingLeft(1).PaddingRight(1).
		Render(content)
}

// periodToDateRange converts a Period to a types.DateRange.
func periodToDateRange(p Period) types.DateRange {
	dr := periodDateRange(p)
	return types.DateRange{Start: dr.Start, End: dr.End}
}
