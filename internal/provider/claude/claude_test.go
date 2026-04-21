package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentseal/codeburn/internal/provider"
)

func makeSource(dir, project string) provider.SessionSource {
	return provider.SessionSource{Path: dir, Project: project, Provider: "claude"}
}

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

func TestBasicAssistantCall(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, filepath.Join(dir, "session1.jsonl"), []map[string]any{
		{
			"type":      "user",
			"timestamp": "2024-01-01T00:00:00Z",
			"sessionId": "session1",
			"message":   map[string]any{"role": "user", "content": "hello"},
		},
		{
			"type":      "assistant",
			"timestamp": "2024-01-01T00:00:01Z",
			"sessionId": "session1",
			"message": map[string]any{
				"id":    "msg_001",
				"model": "claude-sonnet-4-5",
				"usage": map[string]any{
					"input_tokens":  100,
					"output_tokens": 50,
				},
				"content": []map[string]any{
					{"type": "tool_use", "name": "Bash"},
				},
			},
		},
	})

	p := &Provider{}
	seenKeys := make(map[string]struct{})

	var calls []provider.ParsedCall
	for call, err := range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		calls = append(calls, call)
	}

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	c := calls[0]
	if c.UserMessage != "hello" {
		t.Errorf("UserMessage = %q, want %q", c.UserMessage, "hello")
	}
	if c.SessionID != "session1" {
		t.Errorf("SessionID = %q, want %q", c.SessionID, "session1")
	}
	if len(c.Tools) != 1 || c.Tools[0] != "Bash" {
		t.Errorf("Tools = %v, want [Bash]", c.Tools)
	}
	if c.InputTokens != 100 || c.OutputTokens != 50 {
		t.Errorf("tokens = %d/%d, want 100/50", c.InputTokens, c.OutputTokens)
	}
}

func TestDeduplicationByMsgID(t *testing.T) {
	dir := t.TempDir()
	entry := map[string]any{
		"type":      "assistant",
		"timestamp": "2024-01-01T00:00:01Z",
		"sessionId": "session1",
		"message": map[string]any{
			"id":      "msg_dup",
			"model":   "claude-sonnet-4-5",
			"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
			"content": []map[string]any{},
		},
	}
	writeJSONL(t, filepath.Join(dir, "s.jsonl"), []map[string]any{entry, entry})

	p := &Provider{}
	seenKeys := make(map[string]struct{})
	var count int
	for _, _ = range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 call (deduped), got %d", count)
	}
}

func TestDeduplicationFallbackKey(t *testing.T) {
	dir := t.TempDir()
	// No ID field -> fallback key is "claude:<timestamp>"
	entry := map[string]any{
		"type":      "assistant",
		"timestamp": "2024-01-01T00:00:01Z",
		"sessionId": "session1",
		"message": map[string]any{
			"model":   "claude-sonnet-4-5",
			"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
			"content": []map[string]any{},
		},
	}
	writeJSONL(t, filepath.Join(dir, "s.jsonl"), []map[string]any{entry, entry})

	p := &Provider{}
	seenKeys := make(map[string]struct{})
	var count int
	for _, _ = range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("fallback dedup: expected 1, got %d", count)
	}
}

func TestInvalidJSONLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "s.jsonl"))
	f.WriteString("not json\n")
	f.WriteString(`{"type":"assistant","timestamp":"2024-01-01T00:00:01Z","sessionId":"s","message":{"id":"m1","model":"claude-sonnet-4-5","usage":{"input_tokens":10,"output_tokens":5},"content":[]}}` + "\n")
	f.Close()

	p := &Provider{}
	seenKeys := make(map[string]struct{})
	var count int
	for _, _ = range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 valid call, got %d", count)
	}
}

func TestSubagentJSONLIncluded(t *testing.T) {
	dir := t.TempDir()
	uuid := "abc-123"
	subagentsDir := filepath.Join(dir, uuid, "subagents")
	os.MkdirAll(subagentsDir, 0o755)

	writeJSONL(t, filepath.Join(subagentsDir, "sub.jsonl"), []map[string]any{
		{
			"type":      "assistant",
			"timestamp": "2024-01-01T00:00:02Z",
			"sessionId": "sub",
			"message": map[string]any{
				"id":      "msg_sub",
				"model":   "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 20, "output_tokens": 10},
				"content": []map[string]any{},
			},
		},
	})

	p := &Provider{}
	seenKeys := make(map[string]struct{})
	var count int
	for _, _ = range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 subagent call, got %d", count)
	}
}

func TestUserMessageTextBlockArray(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, filepath.Join(dir, "s.jsonl"), []map[string]any{
		{
			"type":      "user",
			"timestamp": "2024-01-01T00:00:00Z",
			"sessionId": "s",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "first"},
					{"type": "text", "text": "second"},
				},
			},
		},
		{
			"type":      "assistant",
			"timestamp": "2024-01-01T00:00:01Z",
			"sessionId": "s",
			"message": map[string]any{
				"id":      "m1",
				"model":   "claude-sonnet-4-5",
				"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
				"content": []map[string]any{},
			},
		},
	})

	p := &Provider{}
	seenKeys := make(map[string]struct{})
	for call, _ := range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		if call.UserMessage != "first second" {
			t.Errorf("UserMessage = %q, want %q", call.UserMessage, "first second")
		}
	}
}

func TestGlobalDedupAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	entry := map[string]any{
		"type":      "assistant",
		"timestamp": "2024-01-01T00:00:01Z",
		"sessionId": "s",
		"message": map[string]any{
			"id":      "shared_id",
			"model":   "claude-sonnet-4-5",
			"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
			"content": []map[string]any{},
		},
	}
	writeJSONL(t, filepath.Join(dir, "a.jsonl"), []map[string]any{entry})
	writeJSONL(t, filepath.Join(dir, "b.jsonl"), []map[string]any{entry})

	p := &Provider{}
	seenKeys := make(map[string]struct{})
	var count int
	for _, _ = range p.ParseSession(makeSource(dir, "proj"), seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("global dedup: expected 1, got %d", count)
	}
}
