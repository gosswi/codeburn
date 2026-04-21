package cursor

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/agentseal/codeburn/internal/provider"
)

func createTestDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.vscdb")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	return dbPath, db
}

func insertBubble(t *testing.T, db *sql.DB, key, value string) {
	t.Helper()
	_, err := db.Exec("INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)", key, value)
	if err != nil {
		t.Fatal(err)
	}
}

func recentTime() string {
	return time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339)
}

func oldTime() string {
	return time.Now().AddDate(0, 0, -40).UTC().Format(time.RFC3339)
}

func TestSchemaValidation_MissingTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.vscdb")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// No cursorDiskKV table - schema validation should print warning and return no calls
	db.Close()

	p := &Provider{dbPathOverride: dbPath}
	seenKeys := make(map[string]struct{})

	var count int
	for _, _ = range p.ParseSession(provider.SessionSource{Path: dbPath}, seenKeys) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 calls with missing table, got %d", count)
	}
}

func TestMissingDB(t *testing.T) {
	p := &Provider{dbPathOverride: "/nonexistent/state.vscdb"}
	sessions, err := p.DiscoverSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for missing DB, got %d", len(sessions))
	}
}

func TestBasicBubbleParsing(t *testing.T) {
	dbPath, db := createTestDB(t)
	createdAt := recentTime()
	insertBubble(t, db, "bubbleId:conv1:1", `{
		"tokenCount": {"inputTokens": 100, "outputTokens": 50},
		"modelInfo": {"modelName": "claude-sonnet-4-5"},
		"createdAt": "`+createdAt+`",
		"conversationId": "conv1",
		"text": "assistant response",
		"codeBlocks": null
	}`)
	db.Close()

	p := &Provider{dbPathOverride: dbPath}
	seenKeys := make(map[string]struct{})

	var calls []provider.ParsedCall
	for call, err := range p.ParseSession(provider.SessionSource{Path: dbPath}, seenKeys) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		calls = append(calls, call)
	}

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	c := calls[0]
	if c.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", c.InputTokens)
	}
	if c.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", c.OutputTokens)
	}
	if c.SessionID != "conv1" {
		t.Errorf("SessionID = %q, want %q", c.SessionID, "conv1")
	}
}

func Test35DayLookback(t *testing.T) {
	dbPath, db := createTestDB(t)
	// Recent entry - should be included
	insertBubble(t, db, "bubbleId:conv1:1", `{
		"tokenCount": {"inputTokens": 100, "outputTokens": 50},
		"modelInfo": {"modelName": "claude-sonnet-4-5"},
		"createdAt": "`+recentTime()+`",
		"conversationId": "conv1",
		"text": "recent",
		"codeBlocks": null
	}`)
	// Old entry (>35 days) - should be excluded
	insertBubble(t, db, "bubbleId:conv2:1", `{
		"tokenCount": {"inputTokens": 200, "outputTokens": 100},
		"modelInfo": {"modelName": "claude-sonnet-4-5"},
		"createdAt": "`+oldTime()+`",
		"conversationId": "conv2",
		"text": "old",
		"codeBlocks": null
	}`)
	db.Close()

	p := &Provider{dbPathOverride: dbPath}
	seenKeys := make(map[string]struct{})

	var count int
	for _, _ = range p.ParseSession(provider.SessionSource{Path: dbPath}, seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 recent call (35-day lookback), got %d", count)
	}
}

func TestDedupKey(t *testing.T) {
	dbPath, db := createTestDB(t)
	createdAt := recentTime()
	// Same bubble, same tokens -> same dedup key
	val := `{
		"tokenCount": {"inputTokens": 100, "outputTokens": 50},
		"modelInfo": {"modelName": "claude-sonnet-4-5"},
		"createdAt": "` + createdAt + `",
		"conversationId": "conv1",
		"text": "response",
		"codeBlocks": null
	}`
	insertBubble(t, db, "bubbleId:conv1:1", val)
	db.Close()

	p := &Provider{dbPathOverride: dbPath}
	// Pre-populate seenKeys with the expected dedup key
	seenKeys := map[string]struct{}{
		"cursor:conv1:" + createdAt + ":100:50": {},
	}

	var count int
	for _, _ = range p.ParseSession(provider.SessionSource{Path: dbPath}, seenKeys) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 (already seen), got %d", count)
	}
}

func TestLanguageExtraction(t *testing.T) {
	dbPath, db := createTestDB(t)
	createdAt := recentTime()
	insertBubble(t, db, "bubbleId:conv1:1", `{
		"tokenCount": {"inputTokens": 100, "outputTokens": 50},
		"modelInfo": {"modelName": "claude-sonnet-4-5"},
		"createdAt": "`+createdAt+`",
		"conversationId": "conv1",
		"text": "response",
		"codeBlocks": [{"languageId": "go"}, {"languageId": "python"}, {"languageId": "plaintext"}]
	}`)
	db.Close()

	p := &Provider{dbPathOverride: dbPath}
	seenKeys := make(map[string]struct{})
	for call, _ := range p.ParseSession(provider.SessionSource{Path: dbPath}, seenKeys) {
		found := map[string]bool{}
		for _, tool := range call.Tools {
			found[tool] = true
		}
		if !found["cursor:edit"] {
			t.Error("expected cursor:edit tool")
		}
		if !found["lang:go"] {
			t.Error("expected lang:go tool")
		}
		if !found["lang:python"] {
			t.Error("expected lang:python tool")
		}
		if found["lang:plaintext"] {
			t.Error("plaintext should be excluded")
		}
	}
}

func TestCursorFileCache(t *testing.T) {
	dbPath, db := createTestDB(t)
	createdAt := recentTime()
	insertBubble(t, db, "bubbleId:conv1:1", `{
		"tokenCount": {"inputTokens": 100, "outputTokens": 50},
		"modelInfo": {"modelName": "claude-sonnet-4-5"},
		"createdAt": "`+createdAt+`",
		"conversationId": "conv1",
		"text": "response",
		"codeBlocks": null
	}`)
	db.Close()

	// Set HOME to temp dir so cache goes there
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	p := &Provider{dbPathOverride: dbPath}
	src := provider.SessionSource{Path: dbPath}

	// First parse - hits DB and writes cache
	seenKeys := make(map[string]struct{})
	var count1 int
	for _, _ = range p.ParseSession(src, seenKeys) {
		count1++
	}
	if count1 != 1 {
		t.Fatalf("first parse: got %d, want 1", count1)
	}

	// Second parse - should hit file cache (same fingerprint)
	seenKeys2 := make(map[string]struct{})
	var count2 int
	for _, _ = range p.ParseSession(src, seenKeys2) {
		count2++
	}
	if count2 != 1 {
		t.Errorf("cached parse: got %d, want 1", count2)
	}
}

func TestDiscoverSessions_ExistingDB(t *testing.T) {
	dbPath, db := createTestDB(t)
	db.Close()

	p := &Provider{dbPathOverride: dbPath}
	sessions, err := p.DiscoverSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session source, got %d", len(sessions))
	}
	if sessions[0].Path != dbPath {
		t.Errorf("Path = %q, want %q", sessions[0].Path, dbPath)
	}
}

func TestGetCacheFilePath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	path, err := getCacheFilePath()
	if err != nil {
		t.Fatal(err)
	}

	expectedSuffix := filepath.Join(cursorCacheSubDir, cursorCacheFile)
	if !filepath.IsAbs(path) {
		t.Errorf("getCacheFilePath should return absolute path, got %q", path)
	}
	if len(path) < len(expectedSuffix) || path[len(path)-len(expectedSuffix):] != expectedSuffix {
		t.Errorf("path %q should end with %q", path, expectedSuffix)
	}
}

// Test that HOME env var is used for cache path.
func TestGetCacheFilePathUsesHomeEnv(t *testing.T) {
	if os.Getenv("HOME") == "" {
		t.Skip("HOME not set")
	}
	path, err := getCacheFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
}
