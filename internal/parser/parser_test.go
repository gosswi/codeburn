package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentseal/codeburn/internal/types"
)

// writeJSONL writes JSONL entries to a file.
func writeJSONL(t *testing.T, path string, entries []map[string]any) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
}

func assistantEntry(id, model string, inputTokens, outputTokens int) map[string]any {
	return map[string]any{
		"type":      "assistant",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"sessionId": "test-session",
		"message": map[string]any{
			"id":    id,
			"model": model,
			"usage": map[string]any{
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
			},
			"content": []map[string]any{},
		},
	}
}

func TestPrivacyInvariant_UserMessageZeroed(t *testing.T) {
	// Set up a temp dir with a JSONL file.
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	os.MkdirAll(filepath.Join(dir, "claude", "projects", "test-project"), 0o755)

	writeJSONL(t, filepath.Join(dir, "claude", "projects", "test-project", "session1.jsonl"), []map[string]any{
		{
			"type":      "user",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"sessionId": "session1",
			"message":   map[string]any{"role": "user", "content": "this is my secret question"},
		},
		assistantEntry("msg_001", "claude-sonnet-4-5", 100, 50),
	})

	projects, err := ParseAllSessions(types.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range projects {
		for _, session := range p.Sessions {
			for i, turn := range session.Turns {
				if turn.UserMessage != "" {
					t.Errorf("turn[%d].UserMessage = %q, want empty (R15 privacy invariant)", i, turn.UserMessage)
				}
			}
		}
	}
}

func TestProjectMerging(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	projDir := filepath.Join(dir, "claude", "projects", "my-project")
	os.MkdirAll(projDir, 0o755)

	// Two session files in the same project.
	writeJSONL(t, filepath.Join(projDir, "session1.jsonl"), []map[string]any{
		assistantEntry("msg_001", "claude-sonnet-4-5", 100, 50),
	})
	writeJSONL(t, filepath.Join(projDir, "session2.jsonl"), []map[string]any{
		assistantEntry("msg_002", "claude-sonnet-4-5", 200, 100),
	})

	projects, err := ParseAllSessions(types.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range projects {
		if p.Project == "my-project" {
			found = true
			if len(p.Sessions) < 1 {
				t.Errorf("expected at least 1 session, got %d", len(p.Sessions))
			}
		}
	}
	if !found {
		t.Error("expected to find my-project in results")
	}
}

func TestSortByCostDescending(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	os.MkdirAll(filepath.Join(dir, "claude", "projects", "expensive"), 0o755)
	os.MkdirAll(filepath.Join(dir, "claude", "projects", "cheap"), 0o755)

	// expensive: 10000 input tokens
	writeJSONL(t, filepath.Join(dir, "claude", "projects", "expensive", "s.jsonl"), []map[string]any{
		assistantEntry("e1", "claude-sonnet-4-5", 10000, 5000),
	})
	// cheap: 10 input tokens
	writeJSONL(t, filepath.Join(dir, "claude", "projects", "cheap", "s.jsonl"), []map[string]any{
		assistantEntry("c1", "claude-sonnet-4-5", 10, 5),
	})

	projects, err := ParseAllSessions(types.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(projects) < 2 {
		t.Fatalf("expected at least 2 projects, got %d", len(projects))
	}
	if projects[0].TotalCostUSD < projects[1].TotalCostUSD {
		t.Errorf("projects not sorted by cost desc: %v < %v", projects[0].TotalCostUSD, projects[1].TotalCostUSD)
	}
}

func TestInProcCache_HitWithinTTL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	os.MkdirAll(filepath.Join(dir, "claude", "projects", "proj"), 0o755)
	writeJSONL(t, filepath.Join(dir, "claude", "projects", "proj", "s.jsonl"), []map[string]any{
		assistantEntry("m1", "claude-sonnet-4-5", 100, 50),
	})

	opts := types.ParseOptions{}

	// First call: populates cache.
	r1, err := ParseAllSessionsCached(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Second call within TTL: should return same data.
	r2, err := ParseAllSessionsCached(opts)
	if err != nil {
		t.Fatal(err)
	}

	if len(r1) != len(r2) {
		t.Errorf("cache miss: r1=%d r2=%d projects", len(r1), len(r2))
	}
}

func TestInProcCache_MissAfterTTL(t *testing.T) {
	// Clear cache, set an entry with expired ts, verify miss.
	key := "test-ttl-key"
	inProcMu.Lock()
	inProcCache[key] = &inProcEntry{
		data: []types.ProjectSummary{{Project: "old"}},
		ts:   time.Now().Add(-2 * inProcCacheTTL),
	}
	inProcMu.Unlock()

	inProcMu.Lock()
	entry, ok := inProcCache[key]
	isExpired := ok && time.Since(entry.ts) >= inProcCacheTTL
	inProcMu.Unlock()

	if !isExpired {
		t.Error("expected entry to be expired")
	}
}

func TestBuildSessionSummary_ToolBreakdown(t *testing.T) {
	turns := []types.ClassifiedTurn{
		{
			ParsedTurn: types.ParsedTurn{
				AssistantCalls: []types.ParsedApiCall{
					{
						Model:    "claude-sonnet-4-5",
						Tools:    []string{"Bash", "Read", "mcp__atlassian__search"},
						Timestamp: time.Now().UTC().Format(time.RFC3339),
					},
				},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
			Category: types.CategoryCoding,
		},
	}
	summary := buildSessionSummary("s1", "proj", turns)

	if _, ok := summary.ToolBreakdown["Bash"]; !ok {
		t.Error("expected Bash in ToolBreakdown")
	}
	if _, ok := summary.ToolBreakdown["Read"]; !ok {
		t.Error("expected Read in ToolBreakdown")
	}
	if _, ok := summary.ToolBreakdown["mcp__atlassian__search"]; ok {
		t.Error("mcp tool should not appear in ToolBreakdown")
	}
	if _, ok := summary.McpBreakdown["atlassian"]; !ok {
		t.Error("expected atlassian in McpBreakdown")
	}
}

func TestFilterSessionByDateRange(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)

	session := types.SessionSummary{
		SessionID: "s1",
		Project:   "proj",
		Turns: []types.ClassifiedTurn{
			{
				ParsedTurn: types.ParsedTurn{
					Timestamp:      now.Format(time.RFC3339),
					AssistantCalls: []types.ParsedApiCall{{Model: "m", Timestamp: now.Format(time.RFC3339)}},
				},
				Category: types.CategoryCoding,
			},
			{
				ParsedTurn: types.ParsedTurn{
					Timestamp:      old.Format(time.RFC3339),
					AssistantCalls: []types.ParsedApiCall{{Model: "m", Timestamp: old.Format(time.RFC3339)}},
				},
				Category: types.CategoryCoding,
			},
		},
	}

	dr := types.DateRange{
		Start: now.Add(-1 * time.Hour).UnixMilli(),
		End:   now.Add(1 * time.Hour).UnixMilli(),
	}

	filtered := filterSessionByDateRange(session, dr)
	if filtered == nil {
		t.Fatal("expected filtered session, got nil")
	}
	if len(filtered.Turns) != 1 {
		t.Errorf("expected 1 turn after filter, got %d", len(filtered.Turns))
	}
}
