package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentseal/codeburn/internal/provider"
)

func makeSource(path string) provider.SessionSource {
	return provider.SessionSource{Path: path, Project: "test", Provider: "codex"}
}

func writeLines(t *testing.T, path string, entries []any) {
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

func TestSessionMetaValidation(t *testing.T) {
	dir := t.TempDir()
	// invalid: originator does not start with "codex"
	badPath := filepath.Join(dir, "rollout-bad.jsonl")
	writeLines(t, badPath, []any{
		map[string]any{
			"type": "session_meta",
			"payload": map[string]any{
				"originator": "openai",
				"session_id": "s1",
				"cwd":        "/home/user/proj",
			},
		},
	})
	_, _, _, ok := readFirstLine(badPath)
	if ok {
		t.Error("expected invalid for non-codex originator")
	}

	// valid
	goodPath := filepath.Join(dir, "rollout-good.jsonl")
	writeLines(t, goodPath, []any{
		map[string]any{
			"type": "session_meta",
			"payload": map[string]any{
				"originator": "codex-cli",
				"session_id": "s1",
				"cwd":        "/home/user/proj",
			},
		},
	})
	sessionID, _, cwd, ok := readFirstLine(goodPath)
	if !ok {
		t.Error("expected valid codex session")
	}
	if sessionID != "s1" {
		t.Errorf("sessionID = %q, want %q", sessionID, "s1")
	}
	if cwd != "/home/user/proj" {
		t.Errorf("cwd = %q, want %q", cwd, "/home/user/proj")
	}
}

func TestTokenNormalization(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "rollout-session.jsonl")
	writeLines(t, filePath, []any{
		map[string]any{
			"type": "session_meta",
			"payload": map[string]any{
				"originator": "codex",
				"session_id": "s1",
				"model":      "gpt-4o",
				"cwd":        "/project",
			},
		},
		map[string]any{
			"type":      "event_msg",
			"timestamp": "2024-01-01T00:00:01Z",
			"payload": map[string]any{
				"type": "token_count",
				"info": map[string]any{
					"last_token_usage": map[string]any{
						"input_tokens":        int64(150),
						"cached_input_tokens": int64(50),
						"output_tokens":       int64(75),
					},
					"total_token_usage": map[string]any{
						"total_tokens": int64(275),
					},
				},
			},
		},
	})

	p := &Provider{codexDir: dir}
	// Manually parse a single file
	src := makeSource(filePath)
	seenKeys := make(map[string]struct{})

	var calls []provider.ParsedCall
	for call, err := range p.ParseSession(src, seenKeys) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		calls = append(calls, call)
	}

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	c := calls[0]
	// uncachedInput = max(0, 150 - 50) = 100
	if c.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (uncached only)", c.InputTokens)
	}
	if c.CachedInputTokens != 50 {
		t.Errorf("CachedInputTokens = %d, want 50", c.CachedInputTokens)
	}
	if c.OutputTokens != 75 {
		t.Errorf("OutputTokens = %d, want 75", c.OutputTokens)
	}
}

func TestDedupKey(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "rollout-x.jsonl")
	tokenEvent := map[string]any{
		"type":      "event_msg",
		"timestamp": "2024-01-01T00:01:00Z",
		"payload": map[string]any{
			"type": "token_count",
			"info": map[string]any{
				"last_token_usage": map[string]any{
					"input_tokens":  int64(10),
					"output_tokens": int64(5),
				},
				"total_token_usage": map[string]any{"total_tokens": int64(15)},
			},
		},
	}
	writeLines(t, filePath, []any{
		map[string]any{
			"type":    "session_meta",
			"payload": map[string]any{"originator": "codex", "session_id": "s1", "cwd": "/p"},
		},
		tokenEvent,
		tokenEvent, // duplicate
	})

	p := &Provider{codexDir: dir}
	src := makeSource(filePath)
	seenKeys := make(map[string]struct{})
	var count int
	for _, _ = range p.ParseSession(src, seenKeys) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 (deduped), got %d", count)
	}
}

func TestCumulativeDeltaAccounting(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "rollout-delta.jsonl")
	writeLines(t, filePath, []any{
		map[string]any{
			"type":    "session_meta",
			"payload": map[string]any{"originator": "codex", "session_id": "s1", "model": "gpt-4o", "cwd": "/p"},
		},
		// First event: cumulative only (no last_token_usage)
		map[string]any{
			"type":      "event_msg",
			"timestamp": "2024-01-01T00:01:00Z",
			"payload": map[string]any{
				"type": "token_count",
				"info": map[string]any{
					"total_token_usage": map[string]any{
						"input_tokens":  int64(100),
						"output_tokens": int64(50),
						"total_tokens":  int64(150),
					},
				},
			},
		},
		// Second event: cumulative increases
		map[string]any{
			"type":      "event_msg",
			"timestamp": "2024-01-01T00:02:00Z",
			"payload": map[string]any{
				"type": "token_count",
				"info": map[string]any{
					"total_token_usage": map[string]any{
						"input_tokens":  int64(200),
						"output_tokens": int64(100),
						"total_tokens":  int64(300),
					},
				},
			},
		},
	})

	p := &Provider{codexDir: dir}
	src := makeSource(filePath)
	seenKeys := make(map[string]struct{})
	var calls []provider.ParsedCall
	for call, _ := range p.ParseSession(src, seenKeys) {
		calls = append(calls, call)
	}

	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	// First call: delta from 0 -> 150
	if calls[0].InputTokens != 100 {
		t.Errorf("call[0].InputTokens = %d, want 100", calls[0].InputTokens)
	}
	// Second call: delta from 150 -> 300
	if calls[1].InputTokens != 100 {
		t.Errorf("call[1].InputTokens = %d, want 100", calls[1].InputTokens)
	}
}

func TestToolNormalization(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "rollout-tools.jsonl")
	writeLines(t, filePath, []any{
		map[string]any{
			"type":    "session_meta",
			"payload": map[string]any{"originator": "codex", "session_id": "s1", "model": "gpt-4o", "cwd": "/p"},
		},
		map[string]any{
			"type":      "response_item",
			"timestamp": "2024-01-01T00:00:01Z",
			"payload":   map[string]any{"type": "function_call", "name": "exec_command"},
		},
		map[string]any{
			"type":      "event_msg",
			"timestamp": "2024-01-01T00:01:00Z",
			"payload": map[string]any{
				"type": "token_count",
				"info": map[string]any{
					"last_token_usage": map[string]any{
						"input_tokens":  int64(10),
						"output_tokens": int64(5),
					},
					"total_token_usage": map[string]any{"total_tokens": int64(15)},
				},
			},
		},
	})

	p := &Provider{codexDir: dir}
	seenKeys := make(map[string]struct{})
	for call, _ := range p.ParseSession(makeSource(filePath), seenKeys) {
		if len(call.Tools) != 1 || call.Tools[0] != "Bash" {
			t.Errorf("Tools = %v, want [Bash]", call.Tools)
		}
	}
}
