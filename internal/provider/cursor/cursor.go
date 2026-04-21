package cursor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/agentseal/codeburn/internal/models"
	"github.com/agentseal/codeburn/internal/provider"
	"github.com/agentseal/codeburn/internal/sqlitedrv"
)

const (
	defaultModel       = "claude-sonnet-4-5"
	lookbackDays       = 35
	cursorCacheSubDir  = ".cache/codeburn"
	cursorCacheFile    = "cursor-results.json"
)

var bubbleQuerySince = `
  SELECT
    json_extract(value, '$.tokenCount.inputTokens') as input_tokens,
    json_extract(value, '$.tokenCount.outputTokens') as output_tokens,
    json_extract(value, '$.modelInfo.modelName') as model,
    json_extract(value, '$.createdAt') as created_at,
    json_extract(value, '$.conversationId') as conversation_id,
    substr(json_extract(value, '$.text'), 1, 500) as user_text,
    json_extract(value, '$.codeBlocks') as code_blocks
  FROM cursorDiskKV
  WHERE key LIKE 'bubbleId:%'
    AND json_extract(value, '$.tokenCount.inputTokens') > 0
    AND json_extract(value, '$.createdAt') > ?
  ORDER BY json_extract(value, '$.createdAt') ASC
`

var userMessagesQuery = `
  SELECT
    json_extract(value, '$.conversationId') as conversation_id,
    json_extract(value, '$.createdAt') as created_at,
    substr(json_extract(value, '$.text'), 1, 500) as text
  FROM cursorDiskKV
  WHERE key LIKE 'bubbleId:%'
    AND json_extract(value, '$.type') = 1
    AND json_extract(value, '$.createdAt') > ?
  ORDER BY json_extract(value, '$.createdAt') ASC
`

type cursorCache struct {
	Fingerprint string               `json:"fingerprint"`
	Results     []provider.ParsedCall `json:"results"`
}

type codeBlock struct {
	LanguageID string `json:"languageId"`
}

func getDbPath() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb")
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Cursor", "User", "globalStorage", "state.vscdb")
	default:
		return filepath.Join(home, ".config", "Cursor", "User", "globalStorage", "state.vscdb")
	}
}

func getCacheFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, cursorCacheSubDir, cursorCacheFile), nil
}

func fingerprint(dbPath string) string {
	info, err := os.Stat(dbPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", info.ModTime().UnixMilli(), info.Size())
}

func readCache(dbPath string) ([]provider.ParsedCall, bool) {
	cachePath, err := getCacheFilePath()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var c cursorCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, false
	}
	if c.Fingerprint != fingerprint(dbPath) {
		return nil, false
	}
	return c.Results, true
}

func writeCache(dbPath string, results []provider.ParsedCall) {
	cachePath, err := getCacheFilePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return
	}
	c := cursorCache{
		Fingerprint: fingerprint(dbPath),
		Results:     results,
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(cachePath, data, 0o644)
}

func extractLanguages(codeBlocksJSON string) []string {
	if codeBlocksJSON == "" {
		return nil
	}
	var blocks []codeBlock
	if err := json.Unmarshal([]byte(codeBlocksJSON), &blocks); err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var langs []string
	for _, b := range blocks {
		if b.LanguageID != "" && b.LanguageID != "plaintext" {
			if _, ok := seen[b.LanguageID]; !ok {
				seen[b.LanguageID] = struct{}{}
				langs = append(langs, b.LanguageID)
			}
		}
	}
	return langs
}

func resolveModel(raw string) string {
	if raw == "" || raw == "default" {
		return defaultModel
	}
	return raw
}

func buildUserMessageMap(db *sql.DB, timeFloor string) map[string][]string {
	result := make(map[string][]string)
	rows, err := db.Query(userMessagesQuery, timeFloor)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var convID, createdAt, text string
		if err := rows.Scan(&convID, &createdAt, &text); err != nil {
			continue
		}
		if convID == "" || text == "" {
			continue
		}
		result[convID] = append(result[convID], text)
	}
	return result
}

func parseBubbles(db *sql.DB, dbPath string, seenKeys map[string]struct{}) ([]provider.ParsedCall, error) {
	timeFloor := time.Now().AddDate(0, 0, -lookbackDays).UTC().Format(time.RFC3339)

	userMessages := buildUserMessageMap(db, timeFloor)

	rows, err := db.Query(bubbleQuerySince, timeFloor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []provider.ParsedCall
	var skipped int
	for rows.Next() {
		var inputTokens, outputTokens sql.NullInt64
		var model, createdAt, convID, userText, codeBlocks sql.NullString

		if err := rows.Scan(&inputTokens, &outputTokens, &model, &createdAt, &convID, &userText, &codeBlocks); err != nil {
			skipped++
			continue
		}

		inToks := inputTokens.Int64
		outToks := outputTokens.Int64
		if inToks == 0 && outToks == 0 {
			continue
		}

		convIDStr := convID.String
		if !convID.Valid {
			convIDStr = "unknown"
		}
		createdAtStr := createdAt.String

		dedupKey := fmt.Sprintf("cursor:%s:%s:%d:%d", convIDStr, createdAtStr, inToks, outToks)
		if _, seen := seenKeys[dedupKey]; seen {
			continue
		}
		seenKeys[dedupKey] = struct{}{}

		rawModel := model.String
		pricingModel := resolveModel(rawModel)
		displayModel := rawModel
		if displayModel == "" {
			displayModel = "default"
		}

		costUSD := models.CalculateCost(pricingModel, inToks, outToks, 0, 0, 0, "standard")

		convMessages := userMessages[convIDStr]
		var userQuestion string
		if len(convMessages) > 0 {
			userQuestion = convMessages[0]
			userMessages[convIDStr] = convMessages[1:]
		}
		assistantText := userText.String
		combinedText := strings.TrimSpace(userQuestion + " " + assistantText)

		langs := extractLanguages(codeBlocks.String)
		var tools []string
		if len(langs) > 0 {
			tools = append(tools, "cursor:edit")
			for _, l := range langs {
				tools = append(tools, "lang:"+l)
			}
		}

		results = append(results, provider.ParsedCall{
			Provider:         "cursor",
			Model:            displayModel,
			InputTokens:      inToks,
			OutputTokens:     outToks,
			CostUSD:          costUSD,
			Tools:            tools,
			Timestamp:        createdAtStr,
			Speed:            "standard",
			DeduplicationKey: dedupKey,
			UserMessage:      combinedText,
			SessionID:        convIDStr,
		})
	}

	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "codeburn: skipped %d unreadable Cursor entries\n", skipped)
	}

	return results, nil
}

// Provider implements the Cursor session provider.
type Provider struct {
	dbPathOverride string
}

// New returns a Cursor provider using the default platform-specific path.
func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string { return "cursor" }

func (p *Provider) DiscoverSessions() ([]provider.SessionSource, error) {
	dbPath := p.dbPathOverride
	if dbPath == "" {
		dbPath = getDbPath()
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, nil
	}
	return []provider.SessionSource{
		{Path: dbPath, Project: "cursor", Provider: "cursor"},
	}, nil
}

func (p *Provider) ParseSession(source provider.SessionSource, seenKeys map[string]struct{}) iter.Seq2[provider.ParsedCall, error] {
	return func(yield func(provider.ParsedCall, error) bool) {
		// Serve from file cache if fingerprint matches.
		if cached, ok := readCache(source.Path); ok {
			for _, call := range cached {
				if _, seen := seenKeys[call.DeduplicationKey]; seen {
					continue
				}
				seenKeys[call.DeduplicationKey] = struct{}{}
				if !yield(call, nil) {
					return
				}
			}
			return
		}

		db, err := sql.Open(sqlitedrv.DriverName, source.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "codeburn: cannot open Cursor database: %v\n", err)
			return
		}
		defer db.Close()

		// Validate schema.
		var cnt int
		err = db.QueryRow("SELECT COUNT(*) FROM cursorDiskKV WHERE key LIKE 'bubbleId:%' LIMIT 1").Scan(&cnt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "codeburn: Cursor storage format not recognized. You may need to update CodeBurn.")
			return
		}

		results, err := parseBubbles(db, source.Path, seenKeys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "codeburn: error reading Cursor database: %v\n", err)
			return
		}

		writeCache(source.Path, results)

		for _, call := range results {
			if !yield(call, nil) {
				return
			}
		}
	}
}
