package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentseal/codeburn/internal/types"
)

func makeProject(name string, cost float64, calls int) types.ProjectSummary {
	return types.ProjectSummary{
		Project:      name,
		ProjectPath:  "/" + name,
		TotalCostUSD: cost,
		TotalAPICalls: calls,
		Sessions: []types.SessionSummary{
			{
				SessionID:    "s1",
				Project:      name,
				TotalCostUSD: cost,
				APICalls:     calls,
				Turns: []types.ClassifiedTurn{
					{
						ParsedTurn: types.ParsedTurn{
							Timestamp: "2026-04-20T10:00:00Z",
							AssistantCalls: []types.ParsedApiCall{
								{
									CostUSD: cost,
									Model:   "claude-sonnet-4-5",
									Usage: types.TokenUsage{
										InputTokens:  100,
										OutputTokens: 50,
									},
								},
							},
						},
						Category: types.CategoryCoding,
					},
				},
				ModelBreakdown: map[string]types.ModelStats{
					"claude-sonnet-4-5": {Calls: calls, CostUSD: cost, Tokens: types.TokenUsage{InputTokens: 100}},
				},
				CategoryBreakdown: map[types.TaskCategory]types.CategoryStats{
					types.CategoryCoding: {Turns: 1, CostUSD: cost},
				},
				ToolBreakdown: map[string]struct{ Calls int }{"Bash": {Calls: 1}},
				BashBreakdown: map[string]struct{ Calls int }{"ls": {Calls: 1}},
			},
		},
	}
}

func TestEscCsv_FormulaInjection(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"=SUM(A1)", "'=SUM(A1)"},
		{"+cmd", "'+cmd"},
		{"-cmd", "'-cmd"},
		{"@foo", "'@foo"},
		{"normal", "normal"},
		{`has,comma`, `"has,comma"`},
		{`has"quote`, `"has""quote"`},
	}
	for _, tt := range tests {
		got := escCsv(tt.input)
		if got != tt.want {
			t.Errorf("escCsv(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExportCSV_WritesFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.csv")

	projects := []types.ProjectSummary{makeProject("myproject", 1.5, 10)}
	periods := []PeriodExport{
		{Label: "Today", Projects: projects},
		{Label: "7 Days", Projects: projects},
		{Label: "30 Days", Projects: projects},
	}

	savedPath, err := ExportCSV(periods, outPath)
	if err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}
	if savedPath != outPath {
		t.Errorf("savedPath = %q, want %q", savedPath, outPath)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("could not read output: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# Summary") {
		t.Error("expected '# Summary' section")
	}
	if !strings.Contains(content, "# Daily - Today") {
		t.Error("expected '# Daily - Today' section")
	}
	if !strings.Contains(content, "myproject") {
		t.Error("expected project name in output")
	}
}

func TestExportJSON_WritesFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.json")

	projects := []types.ProjectSummary{makeProject("myproject", 1.5, 10)}
	periods := []PeriodExport{
		{Label: "30 Days", Projects: projects},
	}

	savedPath, err := ExportJSON(periods, outPath)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	data, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("could not read output: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if _, ok := result["generated"]; !ok {
		t.Error("expected 'generated' key in JSON")
	}
	if _, ok := result["periods"]; !ok {
		t.Error("expected 'periods' key in JSON")
	}
}

func TestSelectAllProjects_Prefers30Days(t *testing.T) {
	p30 := []types.ProjectSummary{makeProject("thirty", 1.0, 5)}
	pOther := []types.ProjectSummary{makeProject("other", 2.0, 10)}
	periods := []PeriodExport{
		{Label: "Today", Projects: pOther},
		{Label: "30 Days", Projects: p30},
	}
	got := selectAllProjects(periods)
	if len(got) != 1 || got[0].Project != "thirty" {
		t.Error("expected 30 Days projects to be selected")
	}
}
