package claude

import (
	"bufio"
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agentseal/codeburn/internal/models"
	"github.com/agentseal/codeburn/internal/provider"
)

// rawEntry is the top-level JSONL structure.
type rawEntry struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
}

type assistantMsg struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Content []contentBlock `json:"content"`
	Usage   apiUsage       `json:"usage"`
	Speed   string         `json:"speed"`
}

type contentBlock struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type apiUsage struct {
	InputTokens              int64          `json:"input_tokens"`
	OutputTokens             int64          `json:"output_tokens"`
	CacheCreationInputTokens int64          `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64          `json:"cache_read_input_tokens"`
	ServerToolUse            *serverToolUse `json:"server_tool_use"`
}

type serverToolUse struct {
	WebSearchRequests int64 `json:"web_search_requests"`
}

type userMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func getClaudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func getDesktopSessionsDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "local-agent-mode-sessions")
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Claude", "local-agent-mode-sessions")
	default:
		return filepath.Join(home, ".config", "Claude", "local-agent-mode-sessions")
	}
}

// findDesktopProjectDirs walks the desktop sessions base dir up to depth 8,
// collecting entries within any "projects" subdirectory found.
func findDesktopProjectDirs(base string, depth int) []string {
	if depth > 8 {
		return nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var results []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "node_modules" || e.Name() == ".git" {
			continue
		}
		full := filepath.Join(base, e.Name())
		if e.Name() == "projects" {
			subEntries, err := os.ReadDir(full)
			if err != nil {
				continue
			}
			for _, pd := range subEntries {
				if pd.IsDir() {
					results = append(results, filepath.Join(full, pd.Name()))
				}
			}
		} else {
			results = append(results, findDesktopProjectDirs(full, depth+1)...)
		}
	}
	return results
}

// collectJSONLFiles returns all .jsonl files in dirPath (directly and under
// <uuid>/subagents/ subdirectories).
func collectJSONLFiles(dirPath string) []string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".jsonl") {
			files = append(files, filepath.Join(dirPath, name))
		} else if e.IsDir() {
			subagentsPath := filepath.Join(dirPath, name, "subagents")
			subEntries, err := os.ReadDir(subagentsPath)
			if err != nil {
				continue
			}
			for _, sf := range subEntries {
				if strings.HasSuffix(sf.Name(), ".jsonl") {
					files = append(files, filepath.Join(subagentsPath, sf.Name()))
				}
			}
		}
	}
	return files
}

func extractUserMessageText(raw json.RawMessage) string {
	var msg userMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Role != "user" {
		return ""
	}
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return s
	}
	var blocks []textBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, " ")
}

// parseFile scans a single JSONL file and appends ParsedCall items to calls.
func parseFile(filePath string, seenKeys map[string]struct{}, calls *[]provider.ParsedCall) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

	currentUserMsg := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}

		switch e.Type {
		case "user":
			text := extractUserMessageText(e.Message)
			if strings.TrimSpace(text) != "" {
				currentUserMsg = text
			}

		case "assistant":
			var msg assistantMsg
			if err := json.Unmarshal(e.Message, &msg); err != nil || msg.Model == "" {
				continue
			}
			if msg.Usage.InputTokens == 0 && msg.Usage.OutputTokens == 0 {
				continue
			}

			dedupKey := msg.ID
			if dedupKey == "" {
				dedupKey = "claude:" + e.Timestamp
			}
			if _, seen := seenKeys[dedupKey]; seen {
				continue
			}
			seenKeys[dedupKey] = struct{}{}

			var webSearchRequests int64
			if msg.Usage.ServerToolUse != nil {
				webSearchRequests = msg.Usage.ServerToolUse.WebSearchRequests
			}

			var tools []string
			for _, block := range msg.Content {
				if block.Type == "tool_use" {
					tools = append(tools, block.Name)
				}
			}

			speed := msg.Speed
			if speed == "" {
				speed = "standard"
			}

			costUSD := models.CalculateCost(
				msg.Model,
				msg.Usage.InputTokens,
				msg.Usage.OutputTokens,
				msg.Usage.CacheCreationInputTokens,
				msg.Usage.CacheReadInputTokens,
				webSearchRequests,
				speed,
			)

			*calls = append(*calls, provider.ParsedCall{
				Provider:                 "claude",
				Model:                    msg.Model,
				InputTokens:              msg.Usage.InputTokens,
				OutputTokens:             msg.Usage.OutputTokens,
				CacheCreationInputTokens: msg.Usage.CacheCreationInputTokens,
				CacheReadInputTokens:     msg.Usage.CacheReadInputTokens,
				WebSearchRequests:        webSearchRequests,
				CostUSD:                  costUSD,
				Tools:                    tools,
				Timestamp:                e.Timestamp,
				Speed:                    speed,
				DeduplicationKey:         dedupKey,
				UserMessage:              currentUserMsg,
				SessionID:                sessionID,
			})
		}
	}
}

// Provider implements the Claude session provider.
type Provider struct{}

func (p *Provider) Name() string { return "claude" }

func (p *Provider) DiscoverSessions() ([]provider.SessionSource, error) {
	var sources []provider.SessionSource

	projectsDir := filepath.Join(getClaudeDir(), "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				sources = append(sources, provider.SessionSource{
					Path:     filepath.Join(projectsDir, e.Name()),
					Project:  e.Name(),
					Provider: "claude",
				})
			}
		}
	}

	for _, dir := range findDesktopProjectDirs(getDesktopSessionsDir(), 0) {
		sources = append(sources, provider.SessionSource{
			Path:     dir,
			Project:  filepath.Base(dir),
			Provider: "claude",
		})
	}

	return sources, nil
}

func (p *Provider) ParseSession(source provider.SessionSource, seenKeys map[string]struct{}) iter.Seq2[provider.ParsedCall, error] {
	return func(yield func(provider.ParsedCall, error) bool) {
		for _, filePath := range collectJSONLFiles(source.Path) {
			var calls []provider.ParsedCall
			parseFile(filePath, seenKeys, &calls)
			for _, call := range calls {
				if !yield(call, nil) {
					return
				}
			}
		}
	}
}
