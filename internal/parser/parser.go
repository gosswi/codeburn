package parser

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agentseal/codeburn/internal/classifier"
	"github.com/agentseal/codeburn/internal/models"
	"github.com/agentseal/codeburn/internal/provider"
	"github.com/agentseal/codeburn/internal/provider/claude"
	"github.com/agentseal/codeburn/internal/provider/codex"
	"github.com/agentseal/codeburn/internal/provider/cursor"
	"github.com/agentseal/codeburn/internal/types"
)

// allProviders is the ordered list of active providers.
var allProviders = []provider.Provider{
	&claude.Provider{},
	codex.New(),
	cursor.New(),
}

// unsanitizePath converts encoded project directory names back to paths.
func unsanitizePath(dirName string) string {
	return strings.ReplaceAll(dirName, "-", "/")
}

// extractMcpTools returns tools whose names start with "mcp__".
func extractMcpTools(tools []string) []string {
	var out []string
	for _, t := range tools {
		if strings.HasPrefix(t, "mcp__") {
			out = append(out, t)
		}
	}
	return out
}

// extractCoreTools returns tools whose names do not start with "mcp__".
func extractCoreTools(tools []string) []string {
	var out []string
	for _, t := range tools {
		if !strings.HasPrefix(t, "mcp__") {
			out = append(out, t)
		}
	}
	return out
}

// mcpServer returns the server name portion of an mcp__ tool name.
func mcpServer(tool string) string {
	parts := strings.SplitN(tool, "__", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return tool
}

// buildSessionSummary aggregates classified turns into a SessionSummary.
func buildSessionSummary(sessionID, project string, turns []types.ClassifiedTurn) types.SessionSummary {
	modelBreakdown := make(map[string]types.ModelStats)
	toolBreakdown := make(map[string]struct{ Calls int })
	mcpBreakdown := make(map[string]struct{ Calls int })
	bashBreakdown := make(map[string]struct{ Calls int })
	categoryBreakdown := make(map[types.TaskCategory]types.CategoryStats)

	var totalCost float64
	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int64
	var apiCalls int
	var firstTs, lastTs string

	for _, turn := range turns {
		turnCost := 0.0
		for _, call := range turn.AssistantCalls {
			turnCost += call.CostUSD
		}

		cat := categoryBreakdown[turn.Category]
		cat.Turns++
		cat.CostUSD += turnCost
		if turn.HasEdits {
			cat.EditTurns++
			cat.Retries += turn.Retries
			if turn.Retries == 0 {
				cat.OneShotTurns++
			}
		}
		categoryBreakdown[turn.Category] = cat

		for _, call := range turn.AssistantCalls {
			totalCost += call.CostUSD
			totalInput += call.Usage.InputTokens
			totalOutput += call.Usage.OutputTokens
			totalCacheRead += call.Usage.CacheReadInputTokens
			totalCacheWrite += call.Usage.CacheCreationInputTokens
			apiCalls++

			modelKey := models.GetShortModelName(call.Model)
			ms := modelBreakdown[modelKey]
			ms.Calls++
			ms.CostUSD += call.CostUSD
			ms.Tokens.InputTokens += call.Usage.InputTokens
			ms.Tokens.OutputTokens += call.Usage.OutputTokens
			ms.Tokens.CacheReadInputTokens += call.Usage.CacheReadInputTokens
			ms.Tokens.CacheCreationInputTokens += call.Usage.CacheCreationInputTokens
			modelBreakdown[modelKey] = ms

			for _, tool := range extractCoreTools(call.Tools) {
				t := toolBreakdown[tool]
				t.Calls++
				toolBreakdown[tool] = t
			}
			for _, mcp := range extractMcpTools(call.Tools) {
				srv := mcpServer(mcp)
				m := mcpBreakdown[srv]
				m.Calls++
				mcpBreakdown[srv] = m
			}
			for _, cmd := range call.BashCommands {
				b := bashBreakdown[cmd]
				b.Calls++
				bashBreakdown[cmd] = b
			}

			if firstTs == "" || call.Timestamp < firstTs {
				firstTs = call.Timestamp
			}
			if lastTs == "" || call.Timestamp > lastTs {
				lastTs = call.Timestamp
			}
		}
	}

	if firstTs == "" && len(turns) > 0 {
		firstTs = turns[0].Timestamp
	}
	if lastTs == "" && len(turns) > 0 {
		lastTs = turns[len(turns)-1].Timestamp
	}

	return types.SessionSummary{
		SessionID:             sessionID,
		Project:               project,
		FirstTimestamp:        firstTs,
		LastTimestamp:         lastTs,
		TotalCostUSD:          totalCost,
		TotalInputTokens:      totalInput,
		TotalOutputTokens:     totalOutput,
		TotalCacheReadTokens:  totalCacheRead,
		TotalCacheWriteTokens: totalCacheWrite,
		APICalls:              apiCalls,
		Turns:                 turns,
		ModelBreakdown:        modelBreakdown,
		ToolBreakdown:         toolBreakdown,
		McpBreakdown:          mcpBreakdown,
		BashBreakdown:         bashBreakdown,
		CategoryBreakdown:     categoryBreakdown,
	}
}

// filterSessionByDateRange filters turns to those within the given range.
// Returns nil if no turns remain.
func filterSessionByDateRange(session types.SessionSummary, dr types.DateRange) *types.SessionSummary {
	var filtered []types.ClassifiedTurn
	for _, turn := range session.Turns {
		if turn.Timestamp == "" {
			filtered = append(filtered, turn)
			continue
		}
		ts, err := time.Parse(time.RFC3339, turn.Timestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339Nano, turn.Timestamp)
			if err != nil {
				filtered = append(filtered, turn)
				continue
			}
		}
		tms := ts.UnixMilli()
		if tms >= dr.Start && tms <= dr.End {
			filtered = append(filtered, turn)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	s := buildSessionSummary(session.SessionID, session.Project, filtered)
	return &s
}

// FilterProjectsByDateRange filters a pre-parsed list of projects to only include
// turns within the given date range. Projects with no matching turns are excluded.
func FilterProjectsByDateRange(projects []types.ProjectSummary, dr types.DateRange) []types.ProjectSummary {
	var result []types.ProjectSummary
	for _, p := range projects {
		var sessions []types.SessionSummary
		for _, s := range p.Sessions {
			filtered := filterSessionByDateRange(s, dr)
			if filtered != nil {
				sessions = append(sessions, *filtered)
			}
		}
		if len(sessions) == 0 {
			continue
		}
		var cost float64
		calls := 0
		for _, s := range sessions {
			cost += s.TotalCostUSD
			calls += s.APICalls
		}
		result = append(result, types.ProjectSummary{
			Project:       p.Project,
			ProjectPath:   p.ProjectPath,
			Sessions:      sessions,
			TotalCostUSD:  cost,
			TotalAPICalls: calls,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCostUSD > result[j].TotalCostUSD
	})
	return result
}

// providerCallToTurn converts a single ParsedCall from a non-Claude provider
// into a ParsedTurn with one API call.
func providerCallToTurn(c provider.ParsedCall) types.ParsedTurn {
	tools := c.Tools
	apiCall := types.ParsedApiCall{
		Provider: c.Provider,
		Model:    c.Model,
		Usage: types.TokenUsage{
			InputTokens:              c.InputTokens,
			OutputTokens:             c.OutputTokens,
			CacheCreationInputTokens: c.CacheCreationInputTokens,
			CacheReadInputTokens:     c.CacheReadInputTokens,
			CachedInputTokens:        c.CachedInputTokens,
			ReasoningTokens:          c.ReasoningTokens,
			WebSearchRequests:        c.WebSearchRequests,
		},
		CostUSD:          c.CostUSD,
		Tools:            tools,
		McpTools:         extractMcpTools(tools),
		HasAgentSpawn:    provider.ContainsTool(tools, "Agent"),
		HasPlanMode:      provider.ContainsTool(tools, "EnterPlanMode"),
		Speed:            c.Speed,
		Timestamp:        c.Timestamp,
		BashCommands:     []string{},
		DeduplicationKey: c.DeduplicationKey,
	}
	return types.ParsedTurn{
		UserMessage:    c.UserMessage,
		AssistantCalls: []types.ParsedApiCall{apiCall},
		Timestamp:      c.Timestamp,
		SessionID:      c.SessionID,
	}
}

// claudeCallToApiCall converts a Claude ParsedCall to a ParsedApiCall.
func claudeCallToApiCall(c provider.ParsedCall) types.ParsedApiCall {
	return types.ParsedApiCall{
		Provider: c.Provider,
		Model:    c.Model,
		Usage: types.TokenUsage{
			InputTokens:              c.InputTokens,
			OutputTokens:             c.OutputTokens,
			CacheCreationInputTokens: c.CacheCreationInputTokens,
			CacheReadInputTokens:     c.CacheReadInputTokens,
			WebSearchRequests:        c.WebSearchRequests,
		},
		CostUSD:          c.CostUSD,
		Tools:            c.Tools,
		McpTools:         extractMcpTools(c.Tools),
		HasAgentSpawn:    provider.ContainsTool(c.Tools, "Agent"),
		HasPlanMode:      provider.ContainsTool(c.Tools, "EnterPlanMode"),
		Speed:            c.Speed,
		Timestamp:        c.Timestamp,
		BashCommands:     []string{},
		DeduplicationKey: c.DeduplicationKey,
	}
}

// groupClaudeCalls groups Claude ParsedCalls into ParsedTurns.
// A new turn starts when a call has a non-empty UserMessage.
func groupClaudeCalls(calls []provider.ParsedCall, sessionID string) []types.ParsedTurn {
	var turns []types.ParsedTurn
	var currentUserMsg string
	var currentCalls []types.ParsedApiCall
	var currentTs, currentSID string

	flush := func() {
		if len(currentCalls) > 0 {
			turns = append(turns, types.ParsedTurn{
				UserMessage:    currentUserMsg,
				AssistantCalls: currentCalls,
				Timestamp:      currentTs,
				SessionID:      currentSID,
			})
		}
	}

	for _, c := range calls {
		if c.UserMessage != "" {
			flush()
			currentUserMsg = c.UserMessage
			currentCalls = nil
			currentTs = c.Timestamp
			currentSID = c.SessionID
		}
		currentCalls = append(currentCalls, claudeCallToApiCall(c))
		if currentTs == "" {
			currentTs = c.Timestamp
		}
		if currentSID == "" {
			currentSID = c.SessionID
		}
	}
	flush()
	return turns
}

// parseSourceResult holds the result of parsing one session source.
type parseSourceResult struct {
	source   provider.SessionSource
	summary  *types.SessionSummary
	err      error
}

// parseSource parses a single session source, using the SQLite cache when available.
func parseSource(
	src provider.SessionSource,
	prov provider.Provider,
	seenKeys *sync.Map,
	cache *SessionCache,
	dateRange *types.DateRange,
) *types.SessionSummary {
	// For Claude: collect all calls from JSONL files, group into turns, build summary.
	// For others: convert each call into a one-call turn.

	isClaudeDir := prov.Name() == "claude"

	var mtimeMs, fileSize int64
	var cacheKey string

	if cache != nil {
		var err error
		mtimeMs, fileSize, err = GetFileFingerprint(src.Path)
		if err == nil {
			cacheKey = src.Path
			if time.Now().UnixMilli()-mtimeMs < 5000 {
				cacheKey = ""
			}
			if cacheKey != "" {
				cached, _ := cache.GetCachedSummary(cacheKey, mtimeMs, fileSize)
				if cached != nil {
					if dateRange != nil {
						return filterSessionByDateRange(*cached, *dateRange)
					}
					return cached
				}
			}
		}
	}

	// Build a per-source seenKeys map for dedup within this source.
	// Global dedup is handled by the sync.Map.
	localSeen := make(map[string]struct{})

	// Drain the iterator, checking global dedup.
	var sourceCalls []provider.ParsedCall
	for call, err := range prov.ParseSession(src, localSeen) {
		if err != nil {
			continue
		}
		_, loaded := seenKeys.LoadOrStore(call.DeduplicationKey, struct{}{})
		if loaded {
			continue
		}
		sourceCalls = append(sourceCalls, call)
	}

	if len(sourceCalls) == 0 {
		return nil
	}

	// Group calls into turns and classify.
	var classifiedTurns []types.ClassifiedTurn
	if isClaudeDir {
		// Group by file (sessionID) and user message boundaries.
		byFile := make(map[string][]provider.ParsedCall)
		var fileOrder []string
		for _, c := range sourceCalls {
			if _, exists := byFile[c.SessionID]; !exists {
				fileOrder = append(fileOrder, c.SessionID)
			}
			byFile[c.SessionID] = append(byFile[c.SessionID], c)
		}
		for _, sid := range fileOrder {
			turns := groupClaudeCalls(byFile[sid], sid)
			for _, turn := range turns {
				ct := classifier.ClassifyTurn(turn)
				ct.UserMessage = "" // R15: zero before cache/aggregate
				classifiedTurns = append(classifiedTurns, ct)
			}
		}
	} else {
		for _, c := range sourceCalls {
			turn := providerCallToTurn(c)
			ct := classifier.ClassifyTurn(turn)
			ct.UserMessage = "" // R15
			classifiedTurns = append(classifiedTurns, ct)
		}
	}

	if len(classifiedTurns) == 0 {
		return nil
	}

	// Use first call's sessionID as the summary session ID.
	sessionID := sourceCalls[0].SessionID
	if sessionID == "" {
		sessionID = src.Path
	}
	summary := buildSessionSummary(sessionID, src.Project, classifiedTurns)

	if summary.APICalls == 0 {
		return nil
	}

	// Cache the full summary (before date filtering).
	if cache != nil && cacheKey != "" {
		cache.PutCachedSummary(cacheKey, mtimeMs, fileSize, &summary)
	}

	if dateRange != nil {
		return filterSessionByDateRange(summary, *dateRange)
	}
	return &summary
}

// ParseAllSessions discovers and parses all sessions from all providers using
// a bounded goroutine pool (NumCPU*2 workers). Global dedup is enforced via
// sync.Map. Results are sorted by total cost descending.
func ParseAllSessions(opts types.ParseOptions) ([]types.ProjectSummary, error) {
	cache, _ := OpenCache()
	defer cache.Close()

	// Discover all sources from all providers (respecting provider filter).
	type sourcedProvider struct {
		src  provider.SessionSource
		prov provider.Provider
	}

	var tasks []sourcedProvider
	for _, prov := range allProviders {
		if opts.ProviderFilter != "" && prov.Name() != opts.ProviderFilter {
			continue
		}
		sources, err := prov.DiscoverSessions()
		if err != nil {
			fmt.Printf("codeburn: discover error for %s: %v\n", prov.Name(), err)
			continue
		}
		for _, src := range sources {
			tasks = append(tasks, sourcedProvider{src, prov})
		}
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	workers := runtime.NumCPU() * 2
	if workers < 1 {
		workers = 1
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}

	var globalSeen sync.Map
	results := make([]*types.SessionSummary, len(tasks))

	taskCh := make(chan int, len(tasks))
	for i := range tasks {
		taskCh <- i
	}
	close(taskCh)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range taskCh {
				t := tasks[idx]
				summary := parseSource(t.src, t.prov, &globalSeen, cache, opts.DateRange)
				results[idx] = summary
			}
		}()
	}
	wg.Wait()

	// Aggregate into projects.
	projectMap := make(map[string]*types.ProjectSummary)
	for _, summary := range results {
		if summary == nil {
			continue
		}
		key := summary.Project
		existing, ok := projectMap[key]
		if !ok {
			ps := &types.ProjectSummary{
				Project:     summary.Project,
				ProjectPath: unsanitizePath(summary.Project),
			}
			projectMap[key] = ps
			existing = ps
		}
		existing.Sessions = append(existing.Sessions, *summary)
		existing.TotalCostUSD += summary.TotalCostUSD
		existing.TotalAPICalls += summary.APICalls
	}

	projects := make([]types.ProjectSummary, 0, len(projectMap))
	for _, ps := range projectMap {
		projects = append(projects, *ps)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].TotalCostUSD > projects[j].TotalCostUSD
	})

	return projects, nil
}

// ---------- In-process session cache (T11) ----------

const (
	inProcCacheTTL     = 60 * time.Second
	inProcCacheMaxSize = 10
)

type inProcEntry struct {
	data []types.ProjectSummary
	ts   time.Time
}

var (
	inProcMu    sync.Mutex
	inProcCache = make(map[string]*inProcEntry)
)

func inProcKey(opts types.ParseOptions) string {
	var sb strings.Builder
	if opts.DateRange != nil {
		sb.WriteString(fmt.Sprintf("%d:%d", opts.DateRange.Start, opts.DateRange.End))
	} else {
		sb.WriteString("none")
	}
	sb.WriteByte(':')
	if opts.ProviderFilter != "" {
		sb.WriteString(opts.ProviderFilter)
	} else {
		sb.WriteString("all")
	}
	sb.WriteByte(':')
	if opts.ExtractBash {
		sb.WriteString("bash")
	} else {
		sb.WriteString("nobash")
	}
	return sb.String()
}

// ParseAllSessionsCached wraps ParseAllSessions with an in-process 60s/10-entry LRU cache.
func ParseAllSessionsCached(opts types.ParseOptions) ([]types.ProjectSummary, error) {
	key := inProcKey(opts)
	now := time.Now()

	inProcMu.Lock()
	if entry, ok := inProcCache[key]; ok && now.Sub(entry.ts) < inProcCacheTTL {
		data := entry.data
		inProcMu.Unlock()
		return data, nil
	}
	inProcMu.Unlock()

	data, err := ParseAllSessions(opts)
	if err != nil {
		return data, err
	}
	if data == nil {
		data = []types.ProjectSummary{}
	}

	inProcMu.Lock()
	defer inProcMu.Unlock()

	// Evict expired entries.
	for k, v := range inProcCache {
		if now.Sub(v.ts) >= inProcCacheTTL {
			delete(inProcCache, k)
		}
	}
	// Evict oldest if at capacity.
	if len(inProcCache) >= inProcCacheMaxSize {
		var oldestKey string
		var oldestTs time.Time
		for k, v := range inProcCache {
			if oldestKey == "" || v.ts.Before(oldestTs) {
				oldestKey = k
				oldestTs = v.ts
			}
		}
		delete(inProcCache, oldestKey)
	}
	inProcCache[key] = &inProcEntry{data: data, ts: now}
	return data, nil
}
