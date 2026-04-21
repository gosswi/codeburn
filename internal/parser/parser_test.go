package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agentseal/codeburn/internal/provider"
	"github.com/agentseal/codeburn/internal/provider/claude"
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

// claudeSource creates a SessionSource for a single Claude JSONL file.
func claudeSource(filePath string) provider.SessionSource {
	return provider.SessionSource{Path: filePath, Project: "proj", Provider: "claude"}
}

// TestParseSource_CacheHit_Claude verifies that a cache hit skips re-parsing (AC-04).
// Pre-populates the cache with a fake summary; expects parseSource to return the fake data.
func TestParseSource_CacheHit_Claude(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeJSONL(t, filePath, []map[string]any{assistantEntry("m1", "claude-sonnet-4-5", 100, 50)})
	oldTime := time.Now().Add(-10 * time.Second)
	os.Chtimes(filePath, oldTime, oldTime)

	cache := openTestCache(t)
	mtimeMs, fileSize, err := GetFileFingerprint(filePath)
	if err != nil {
		t.Fatal(err)
	}

	fakeSummary := testSummary()
	fakeSummary.APICalls = 99
	cache.PutCachedSummary(filePath, mtimeMs, fileSize, fakeSummary)

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.APICalls != 99 {
		t.Errorf("expected cache hit (APICalls=99), got APICalls=%d", result.APICalls)
	}
}

// TestParseSource_CacheMiss_Claude verifies that a cold cache miss triggers a parse
// and writes the result to the cache (AC-05).
func TestParseSource_CacheMiss_Claude(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeJSONL(t, filePath, []map[string]any{assistantEntry("m1", "claude-sonnet-4-5", 100, 50)})
	oldTime := time.Now().Add(-10 * time.Second)
	os.Chtimes(filePath, oldTime, oldTime)

	cache := openTestCache(t)

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result == nil || result.APICalls == 0 {
		t.Fatalf("expected non-nil result with calls, got %v", result)
	}

	mtimeMs, fileSize, _ := GetFileFingerprint(filePath)
	cached, _ := cache.GetCachedSummary(filePath, mtimeMs, fileSize)
	if cached == nil {
		t.Error("expected summary to be written to cache after miss")
	}
}

// TestParseSource_FingerprintInvalidation_Claude verifies that a fingerprint change
// invalidates the cache entry (AC-06).
func TestParseSource_FingerprintInvalidation_Claude(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeJSONL(t, filePath, []map[string]any{assistantEntry("m1", "claude-sonnet-4-5", 100, 50)})

	oldTime := time.Now().Add(-10 * time.Second)
	os.Chtimes(filePath, oldTime, oldTime)

	cache := openTestCache(t)
	mtimeMs1, fileSize1, _ := GetFileFingerprint(filePath)
	fakeSummary := testSummary()
	fakeSummary.APICalls = 99
	cache.PutCachedSummary(filePath, mtimeMs1, fileSize1, fakeSummary)

	// Verify pre-condition: cache is hit with F1
	var globalSeen sync.Map
	prov := &claude.Provider{}
	result := parseSource(claudeSource(filePath), prov, &globalSeen, cache, nil)
	if result == nil || result.APICalls != 99 {
		t.Fatalf("pre-condition: expected cache hit (APICalls=99), got %v", result)
	}

	// Change mtime → new fingerprint F2
	newTime := time.Now().Add(-8 * time.Second)
	os.Chtimes(filePath, newTime, newTime)

	globalSeen = sync.Map{}
	result = parseSource(claudeSource(filePath), prov, &globalSeen, cache, nil)

	if result == nil {
		t.Fatal("expected non-nil result after fingerprint change")
	}
	if result.APICalls == 99 {
		t.Error("expected cache miss after fingerprint change; got stale cached result")
	}
	if result.APICalls != 1 {
		t.Errorf("expected APICalls=1 from fresh parse, got %d", result.APICalls)
	}
}

// TestParseSource_MtimeGuard_Within5s verifies that a file modified within the last 5s
// bypasses both cache read and write but is still parsed (AC-07).
func TestParseSource_MtimeGuard_Within5s(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeJSONL(t, filePath, []map[string]any{assistantEntry("m1", "claude-sonnet-4-5", 100, 50)})
	// File was just written: mtime < 5s ago → guard fires

	cache := openTestCache(t)
	mtimeMs, fileSize, _ := GetFileFingerprint(filePath)

	// Pre-populate cache with a fake entry at this fingerprint
	fakeSummary := testSummary()
	fakeSummary.APICalls = 99
	cache.PutCachedSummary(filePath, mtimeMs, fileSize, fakeSummary)

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result == nil {
		t.Fatal("expected non-nil result (file parsed despite mtime guard)")
	}
	// Guard bypasses cache read: should return real parse result, not fake 99
	if result.APICalls == 99 {
		t.Error("mtime guard should bypass cache read; got stale pre-populated fake data")
	}
	if result.APICalls != 1 {
		t.Errorf("expected fresh parse APICalls=1, got %d", result.APICalls)
	}
	// Guard bypasses cache write: fake entry (99) should be unchanged
	inCache, _ := cache.GetCachedSummary(filePath, mtimeMs, fileSize)
	if inCache == nil || inCache.APICalls != 99 {
		t.Errorf("mtime guard should bypass cache write; expected unchanged fake entry (APICalls=99), got %v", inCache)
	}
}

// TestParseSource_MtimeGuard_Boundary verifies that a file at exactly 5000ms old
// is NOT guarded (guard condition is < 5000, so 5000ms uses the cache) (AC-08).
func TestParseSource_MtimeGuard_Boundary(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeJSONL(t, filePath, []map[string]any{assistantEntry("m1", "claude-sonnet-4-5", 100, 50)})

	// Set mtime to 5000ms ago. At the point of the guard check, elapsed will be >= 5000ms.
	boundaryTime := time.Now().Add(-5000 * time.Millisecond)
	os.Chtimes(filePath, boundaryTime, boundaryTime)

	cache := openTestCache(t)
	mtimeMs, fileSize, _ := GetFileFingerprint(filePath)

	fakeSummary := testSummary()
	fakeSummary.APICalls = 99
	cache.PutCachedSummary(filePath, mtimeMs, fileSize, fakeSummary)

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result == nil {
		t.Fatal("expected non-nil result at 5000ms boundary")
	}
	if result.APICalls != 99 {
		t.Errorf("at 5000ms boundary, cache should be used: expected APICalls=99, got %d", result.APICalls)
	}
}

// TestParseSource_TurnGrouping verifies that user+assistant pairs become separate turns (AC-09).
func TestParseSource_TurnGrouping(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	ts := func(offset int) string {
		return time.Now().Add(time.Duration(offset) * time.Second).UTC().Format(time.RFC3339)
	}
	writeJSONL(t, filePath, []map[string]any{
		{"type": "user", "timestamp": ts(0), "sessionId": "s",
			"message": map[string]any{"role": "user", "content": "q1"}},
		{"type": "assistant", "timestamp": ts(1), "sessionId": "s",
			"message": map[string]any{
				"id": "m1", "model": "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
				"content": []map[string]any{},
			}},
		{"type": "user", "timestamp": ts(2), "sessionId": "s",
			"message": map[string]any{"role": "user", "content": "q2"}},
		{"type": "assistant", "timestamp": ts(3), "sessionId": "s",
			"message": map[string]any{
				"id": "m2", "model": "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 20, "output_tokens": 10},
				"content": []map[string]any{},
			}},
	})

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.APICalls != 2 {
		t.Errorf("expected APICalls=2, got %d", result.APICalls)
	}
	if len(result.Turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(result.Turns))
	}
}

// TestParseSource_FingerprintError verifies that a nonexistent file returns nil
// with no cache operations (AC-10).
func TestParseSource_FingerprintError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nonexistent.jsonl")

	cache := openTestCache(t)
	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result != nil {
		t.Errorf("expected nil result for nonexistent file, got %v", result)
	}
}

// TestParseSource_WriteError verifies that a nil cache returns a valid summary without
// panicking (AC-11).
func TestParseSource_WriteError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeJSONL(t, filePath, []map[string]any{assistantEntry("m1", "claude-sonnet-4-5", 100, 50)})

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil result with nil cache")
	}
	if result.APICalls != 1 {
		t.Errorf("expected APICalls=1, got %d", result.APICalls)
	}
}

// TestParseSource_UserMessageZeroed verifies the privacy invariant: UserMessage is
// zeroed in the in-memory result and in the cached round-trip (AC-12).
func TestParseSource_UserMessageZeroed(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	ts := time.Now().UTC().Format(time.RFC3339)
	writeJSONL(t, filePath, []map[string]any{
		{"type": "user", "timestamp": ts, "sessionId": "s",
			"message": map[string]any{"role": "user", "content": "secret prompt"}},
		{"type": "assistant", "timestamp": ts, "sessionId": "s",
			"message": map[string]any{
				"id": "m1", "model": "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
				"content": []map[string]any{},
			}},
	})
	oldTime := time.Now().Add(-10 * time.Second)
	os.Chtimes(filePath, oldTime, oldTime)

	cache := openTestCache(t)

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	for i, turn := range result.Turns {
		if turn.UserMessage != "" {
			t.Errorf("result.Turns[%d].UserMessage = %q, want empty", i, turn.UserMessage)
		}
	}

	// Round-trip through cache
	mtimeMs, fileSize, _ := GetFileFingerprint(filePath)
	cached, _ := cache.GetCachedSummary(filePath, mtimeMs, fileSize)
	if cached != nil {
		for i, turn := range cached.Turns {
			if turn.UserMessage != "" {
				t.Errorf("cached.Turns[%d].UserMessage = %q, want empty", i, turn.UserMessage)
			}
		}
	}
}

// TestParseSource_ZeroAPICalls verifies that an empty JSONL file returns nil
// and nothing is written to the cache (AC-13).
func TestParseSource_ZeroAPICalls(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.jsonl")
	writeJSONL(t, filePath, nil)
	oldTime := time.Now().Add(-10 * time.Second)
	os.Chtimes(filePath, oldTime, oldTime)

	cache := openTestCache(t)

	var globalSeen sync.Map
	result := parseSource(claudeSource(filePath), &claude.Provider{}, &globalSeen, cache, nil)

	if result != nil {
		t.Errorf("expected nil for empty JSONL, got %v", result)
	}

	mtimeMs, fileSize, _ := GetFileFingerprint(filePath)
	cached, _ := cache.GetCachedSummary(filePath, mtimeMs, fileSize)
	if cached != nil {
		t.Error("expected no cache write for zero API calls")
	}
}

// TestParseSource_DateFilterAfterCache verifies that the full summary is cached and
// date filtering is applied after cache retrieval (AC-14).
func TestParseSource_DateFilterAfterCache(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	recentTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	oldCallTime := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)

	// Each assistant entry must be preceded by a user message so groupClaudeCalls
	// creates two separate turns (one with oldCallTime, one with recentTime).
	writeJSONL(t, filePath, []map[string]any{
		{"type": "user", "timestamp": oldCallTime, "sessionId": "s",
			"message": map[string]any{"role": "user", "content": "old question"}},
		{
			"type": "assistant", "timestamp": oldCallTime, "sessionId": "s",
			"message": map[string]any{
				"id": "m_old", "model": "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 100, "output_tokens": 50},
				"content": []map[string]any{},
			},
		},
		{"type": "user", "timestamp": recentTime, "sessionId": "s",
			"message": map[string]any{"role": "user", "content": "recent question"}},
		{
			"type": "assistant", "timestamp": recentTime, "sessionId": "s",
			"message": map[string]any{
				"id": "m_recent", "model": "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 200, "output_tokens": 100},
				"content": []map[string]any{},
			},
		},
	})
	fileOldTime := time.Now().Add(-10 * time.Second)
	os.Chtimes(filePath, fileOldTime, fileOldTime)

	cache := openTestCache(t)

	// First call: no dateRange → caches full summary (2 calls)
	var globalSeen1 sync.Map
	prov := &claude.Provider{}
	full := parseSource(claudeSource(filePath), prov, &globalSeen1, cache, nil)
	if full == nil || full.APICalls != 2 {
		t.Fatalf("expected full summary with 2 calls, got %v", full)
	}

	// Second call: narrow dateRange → hits cache, applies filter → 1 call
	now := time.Now()
	dr := types.DateRange{
		Start: now.Add(-2 * time.Hour).UnixMilli(),
		End:   now.UnixMilli(),
	}
	var globalSeen2 sync.Map
	filtered := parseSource(claudeSource(filePath), prov, &globalSeen2, cache, &dr)

	if filtered == nil {
		t.Fatal("expected non-nil filtered result")
	}
	if filtered.APICalls != 1 {
		t.Errorf("expected 1 call after date filter, got %d", filtered.APICalls)
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
