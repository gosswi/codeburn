package codex

import (
	"bufio"
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agentseal/codeburn/internal/models"
	"github.com/agentseal/codeburn/internal/provider"
)

var reYear     = regexp.MustCompile(`^\d{4}$`)
var reTwoDigit = regexp.MustCompile(`^\d{2}$`)

// toolNameMap normalizes Codex tool names to CodeBurn canonical names.
var toolNameMap = map[string]string{
	"exec_command": "Bash",
	"read_file":    "Read",
	"write_file":   "Edit",
	"apply_diff":   "Edit",
	"apply_patch":  "Edit",
	"spawn_agent":  "Agent",
	"close_agent":  "Agent",
	"wait_agent":   "Agent",
	"read_dir":     "Glob",
}

type codexEntry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	SessionID  string `json:"session_id"`
	Originator string `json:"originator"`
	Model      string `json:"model"`
	Cwd        string `json:"cwd"`
}

type functionCallPayload struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type messagePayload struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type tokenCountPayload struct {
	Type string          `json:"type"`
	Info json.RawMessage `json:"info"`
}

type tokenInfo struct {
	LastTokenUsage  *tokenUsage `json:"last_token_usage"`
	TotalTokenUsage *tokenUsage `json:"total_token_usage"`
}

type tokenUsage struct {
	InputTokens          int64 `json:"input_tokens"`
	CachedInputTokens    int64 `json:"cached_input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens          int64 `json:"total_tokens"`
}

func getCodexDir() string {
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func sanitizeProject(cwd string) string {
	s := strings.TrimPrefix(cwd, "/")
	return strings.ReplaceAll(s, "/", "-")
}

// readFirstLine reads and decodes the first JSONL line as a session_meta entry.
// Returns (sessionID, model, cwd, ok).
func readFirstLine(filePath string) (sessionID, model, cwd string, ok bool) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 512*1024), 512*1024)
	if !scanner.Scan() {
		return
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return
	}

	var e codexEntry
	if err := json.Unmarshal([]byte(line), &e); err != nil || e.Type != "session_meta" {
		return
	}
	var meta sessionMetaPayload
	if err := json.Unmarshal(e.Payload, &meta); err != nil {
		return
	}
	if !strings.HasPrefix(meta.Originator, "codex") {
		return
	}

	sessionID = meta.SessionID
	model = meta.Model
	cwd = meta.Cwd
	ok = true
	return
}

// Provider implements the Codex session provider.
type Provider struct {
	codexDir string
}

// New returns a Codex provider reading from the default CODEX_HOME directory.
func New() *Provider {
	return &Provider{codexDir: getCodexDir()}
}

func (p *Provider) Name() string { return "codex" }

func (p *Provider) DiscoverSessions() ([]provider.SessionSource, error) {
	sessionsDir := filepath.Join(p.codexDir, "sessions")
	years, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, nil
	}

	var sources []provider.SessionSource
	for _, year := range years {
		if !year.IsDir() || !reYear.MatchString(year.Name()) {
			continue
		}
		yearDir := filepath.Join(sessionsDir, year.Name())
		months, _ := os.ReadDir(yearDir)
		for _, month := range months {
			if !month.IsDir() || !reTwoDigit.MatchString(month.Name()) {
				continue
			}
			monthDir := filepath.Join(yearDir, month.Name())
			days, _ := os.ReadDir(monthDir)
			for _, day := range days {
				if !day.IsDir() || !reTwoDigit.MatchString(day.Name()) {
					continue
				}
				dayDir := filepath.Join(monthDir, day.Name())
				files, _ := os.ReadDir(dayDir)
				for _, file := range files {
					name := file.Name()
					if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
						continue
					}
					if file.IsDir() {
						continue
					}
					filePath := filepath.Join(dayDir, name)
					sessionID, _, cwd, ok := readFirstLine(filePath)
					if !ok {
						continue
					}
					project := sanitizeProject(cwd)
					if project == "" {
						project = sessionID
					}
					sources = append(sources, provider.SessionSource{
						Path:     filePath,
						Project:  project,
						Provider: "codex",
					})
				}
			}
		}
	}
	return sources, nil
}

func (p *Provider) ParseSession(source provider.SessionSource, seenKeys map[string]struct{}) iter.Seq2[provider.ParsedCall, error] {
	return func(yield func(provider.ParsedCall, error) bool) {
		f, err := os.Open(source.Path)
		if err != nil {
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

		var sessionModel string
		var sessionID string
		var prevCumulativeTotal int64
		var prevInput, prevCached, prevOutput, prevReasoning int64
		var pendingTools []string
		var pendingUserMsg string

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var e codexEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				continue
			}

			switch e.Type {
			case "session_meta":
				var meta sessionMetaPayload
				if err := json.Unmarshal(e.Payload, &meta); err == nil {
					sessionID = meta.SessionID
					sessionModel = meta.Model
				}

			case "response_item":
				var fcPayload functionCallPayload
				if err := json.Unmarshal(e.Payload, &fcPayload); err != nil {
					continue
				}
				if fcPayload.Type == "function_call" && fcPayload.Name != "" {
					normalized := toolNameMap[fcPayload.Name]
					if normalized == "" {
						normalized = fcPayload.Name
					}
					pendingTools = append(pendingTools, normalized)
				} else if fcPayload.Type == "message" {
					var msgPayload messagePayload
					if err := json.Unmarshal(e.Payload, &msgPayload); err == nil && msgPayload.Role == "user" {
						var items []contentItem
						if err := json.Unmarshal(msgPayload.Content, &items); err == nil {
							var texts []string
							for _, item := range items {
								if item.Type == "input_text" && item.Text != "" {
									texts = append(texts, item.Text)
								}
							}
							if len(texts) > 0 {
								pendingUserMsg = strings.Join(texts, " ")
							}
						}
					}
				}

			case "event_msg":
				var tcPayload tokenCountPayload
				if err := json.Unmarshal(e.Payload, &tcPayload); err != nil || tcPayload.Type != "token_count" {
					continue
				}
				var info tokenInfo
				if err := json.Unmarshal(tcPayload.Info, &info); err != nil || info.TotalTokenUsage == nil {
					continue
				}

				cumulativeTotal := info.TotalTokenUsage.TotalTokens
				if cumulativeTotal > 0 && cumulativeTotal == prevCumulativeTotal {
					continue
				}
				prevCumulativeTotal = cumulativeTotal

				var inputTokens, cachedInputTokens, outputTokens, reasoningTokens int64
				if info.LastTokenUsage != nil {
					inputTokens = info.LastTokenUsage.InputTokens
					cachedInputTokens = info.LastTokenUsage.CachedInputTokens
					outputTokens = info.LastTokenUsage.OutputTokens
					reasoningTokens = info.LastTokenUsage.ReasoningOutputTokens
				} else if cumulativeTotal > 0 {
					tot := info.TotalTokenUsage
					inputTokens = tot.InputTokens - prevInput
					cachedInputTokens = tot.CachedInputTokens - prevCached
					outputTokens = tot.OutputTokens - prevOutput
					reasoningTokens = tot.ReasoningOutputTokens - prevReasoning

					prevInput = tot.InputTokens
					prevCached = tot.CachedInputTokens
					prevOutput = tot.OutputTokens
					prevReasoning = tot.ReasoningOutputTokens
				}

				totalTokens := inputTokens + cachedInputTokens + outputTokens + reasoningTokens
				if totalTokens == 0 {
					continue
				}

				// Normalize: Codex includes cached inside input_tokens; strip them out.
				uncachedInput := inputTokens - cachedInputTokens
				if uncachedInput < 0 {
					uncachedInput = 0
				}

				dedupKey := "codex:" + source.Path + ":" + e.Timestamp + ":" + strconv.FormatInt(cumulativeTotal, 10)
				if _, seen := seenKeys[dedupKey]; seen {
					continue
				}
				seenKeys[dedupKey] = struct{}{}

				callModel := sessionModel
				if info.LastTokenUsage != nil {
					// model may be in the event; session model is the fallback
				}
				if callModel == "" {
					callModel = "gpt-5"
				}

				costUSD := models.CalculateCost(
					callModel,
					uncachedInput,
					outputTokens+reasoningTokens,
					0,
					cachedInputTokens,
					0,
					"standard",
				)

				if sessionID == "" {
					sessionID = strings.TrimSuffix(filepath.Base(source.Path), ".jsonl")
				}

				call := provider.ParsedCall{
					Provider:             "codex",
					Model:                callModel,
					InputTokens:          uncachedInput,
					OutputTokens:         outputTokens,
					CachedInputTokens:    cachedInputTokens,
					ReasoningTokens:      reasoningTokens,
					CostUSD:              costUSD,
					Tools:                pendingTools,
					Timestamp:            e.Timestamp,
					Speed:                "standard",
					DeduplicationKey:     dedupKey,
					UserMessage:          pendingUserMsg,
					SessionID:            sessionID,
				}

				pendingTools = nil
				pendingUserMsg = ""

				if !yield(call, nil) {
					return
				}
			}
		}
	}
}
