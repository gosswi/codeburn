package parser

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agentseal/codeburn/internal/types"
)

func testSummary() *types.SessionSummary {
	return &types.SessionSummary{
		SessionID:         "test-session",
		Project:           "codeburn",
		FirstTimestamp:    "2024-01-01T00:00:00Z",
		LastTimestamp:     "2024-01-01T01:00:00Z",
		TotalCostUSD:      1.23,
		TotalInputTokens:  100,
		TotalOutputTokens: 50,
		APICalls:          2,
		Turns: []types.ClassifiedTurn{
			{
				ParsedTurn: types.ParsedTurn{
					UserMessage: "", // must be zeroed before caching
					SessionID:   "test-session",
					Timestamp:   "2024-01-01T00:00:00Z",
					AssistantCalls: []types.ParsedApiCall{
						{
							Provider: "claude",
							Model:    "claude-sonnet-4-5",
							Usage:    types.TokenUsage{InputTokens: 100, OutputTokens: 50},
							CostUSD:  1.23,
						},
					},
				},
				Category: types.CategoryCoding,
			},
		},
		ModelBreakdown:    map[string]types.ModelStats{"claude-sonnet-4-5": {Calls: 2, CostUSD: 1.23}},
		ToolBreakdown:     map[string]struct{ Calls int }{},
		McpBreakdown:      map[string]struct{ Calls int }{},
		BashBreakdown:     map[string]struct{ Calls int }{},
		CategoryBreakdown: map[types.TaskCategory]types.CategoryStats{},
	}
}

func openTestCache(t *testing.T) *SessionCache {
	t.Helper()
	cache, err := openCacheAt(filepath.Join(t.TempDir(), "test.db"), true)
	if err != nil || cache == nil {
		t.Fatalf("openCacheAt: err=%v, cache=%v", err, cache)
	}
	t.Cleanup(func() { cache.Close() })
	return cache
}

func TestCacheHit(t *testing.T) {
	c := openTestCache(t)
	s := testSummary()
	c.PutCachedSummary("/a/b.jsonl", 1000, 2048, s)

	got, err := c.GetCachedSummary("/a/b.jsonl", 1000, 2048)
	if err != nil || got == nil {
		t.Fatalf("expected cache hit, err=%v got=%v", err, got)
	}
	if got.SessionID != s.SessionID {
		t.Errorf("SessionID: got %q want %q", got.SessionID, s.SessionID)
	}
}

func TestCacheMissOnMtimeChange(t *testing.T) {
	c := openTestCache(t)
	c.PutCachedSummary("/a/b.jsonl", 1000, 2048, testSummary())

	got, err := c.GetCachedSummary("/a/b.jsonl", 9999, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected miss on mtime change, got hit")
	}
}

func TestCacheMissOnSizeChange(t *testing.T) {
	c := openTestCache(t)
	c.PutCachedSummary("/a/b.jsonl", 1000, 2048, testSummary())

	got, err := c.GetCachedSummary("/a/b.jsonl", 1000, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected miss on size change, got hit")
	}
}

func TestCacheCorruptRecovery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "corrupt.db")
	os.WriteFile(dbPath, []byte("not a sqlite file"), 0o644)

	c, err := openCacheAt(dbPath, true)
	if err != nil || c == nil {
		t.Fatalf("expected recovery, got err=%v cache=%v", err, c)
	}
	defer c.Close()

	// Empty after recovery.
	got, _ := c.GetCachedSummary("/any", 1, 1)
	if got != nil {
		t.Error("expected empty cache after recovery")
	}

	// Write and read back to confirm DB is functional.
	c.PutCachedSummary("/any", 1, 1, testSummary())
	got, _ = c.GetCachedSummary("/any", 1, 1)
	if got == nil {
		t.Error("expected hit after write on recovered cache")
	}
}

func TestCacheConcurrentReads(t *testing.T) {
	c := openTestCache(t)
	c.PutCachedSummary("/a/b.jsonl", 1000, 2048, testSummary())

	var wg sync.WaitGroup
	errs := make(chan string, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.GetCachedSummary("/a/b.jsonl", 1000, 2048)
			if err != nil {
				errs <- err.Error()
			} else if got == nil {
				errs <- "expected hit, got miss"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
}

func TestCachePrivacyInvariant(t *testing.T) {
	c := openTestCache(t)
	s := testSummary()
	// R15: UserMessage must be empty before caching; verify it survives round-trip.
	c.PutCachedSummary("/a/b.jsonl", 1000, 2048, s)

	got, err := c.GetCachedSummary("/a/b.jsonl", 1000, 2048)
	if err != nil || got == nil {
		t.Fatalf("expected hit, err=%v", err)
	}
	for i, turn := range got.Turns {
		if turn.UserMessage != "" {
			t.Errorf("turn[%d].UserMessage = %q after cache round-trip, want empty", i, turn.UserMessage)
		}
	}
}
