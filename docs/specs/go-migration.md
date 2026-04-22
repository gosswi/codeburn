# CodeBurn: TypeScript to Go Migration Spec

**Date:** 2026-04-20
**Status:** Draft
**Branch:** feature/go-migration

---

## 1. Motivation

CodeBurn is a CLI tool that shows where AI coding tokens go. It reads session data from disk for Claude Code, Codex, and Cursor, with an interactive TUI dashboard, CSV/JSON export, and a macOS menu bar widget.

The TypeScript implementation has structural performance limitations that a Go rewrite addresses: Node.js startup overhead, sequential session parsing, high memory footprint from React/Ink, native addon distribution pain (better-sqlite3), and requiring a Node.js runtime for installation.

---

## 2. Current Architecture

### Data Flow

```
CLI (cli.ts)
  -> loadCurrency(), loadPricing()           [HTTP fetch + disk cache]
  -> parseAllSessions(opts)                  [parser.ts]
       -> discoverAllSessions()              [providers/index.ts]
            -> claude.discoverSessions()     [readdir ~/.claude/projects + Desktop walk]
            -> codex.discoverSessions()      [readdir ~/.codex/sessions/YYYY/MM/DD/**]
            -> cursor.discoverSessions()     [lazy-load better-sqlite3, open state.vscdb]
       -> scanProjectDirs()                  [Claude: JSONL line-by-line via readline]
       -> parseProviderSources()             [Codex/Cursor: async generator per session]
       -> merge + sort by cost
  -> renderDashboard()                       [dashboard.tsx - Ink/React TUI]
       or format/export output
```

### Module Inventory

| Module | Responsibility |
|---|---|
| `cli.ts` | Commander subcommands, date-range construction |
| `providers/{claude,codex,cursor}.ts` | Discovery + stream parsing per provider |
| `providers/index.ts` | Provider registry, lazy Cursor load, discovery cache |
| `parser.ts` | JSONL parsing, turn grouping, session summaries, project aggregation |
| `classifier.ts` | Turn category classification via regex + tool presence |
| `models.ts` | Pricing lookup, cost calculation, LiteLLM cache |
| `session-cache.ts` | SQLite-backed parsed-session cache |
| `cursor-cache.ts` | JSON file cache for Cursor parse results |
| `sqlite.ts` | better-sqlite3 wrapper (lazy load, read-only) |
| `dashboard.tsx` | Ink/React interactive TUI (669 lines) |
| `format.ts` / `export.ts` | Cost/token formatting, CSV/JSON export |
| `menubar.ts` | SwiftBar/xbar plugin generation and install |
| `currency.ts` | Exchange rate fetch + cache, Intl.NumberFormat integration |
| `config.ts` | Simple JSON config at `~/.config/codeburn/config.json` |

### Key Invariants

- Deduplication is per-provider using a shared `Set<string>`. Claude uses `message.id`; Codex uses `path:timestamp:cumulativeTotal`; Cursor uses `conversationId:createdAt:inputTokens:outputTokens`.
- SQLite session cache stores full parsed `SessionSummary` JSON keyed by `(filePath, mtimeMs, fileSize)`.
- `userMessage` is zeroed after `classifyTurn()` and before cache write (privacy invariant, R7-R9).
- Cursor data is read-only (`readonly: true, fileMustExist: true`).
- Pricing lookup has a four-level fallback chain: exact match -> fallback table prefix -> LiteLLM fuzzy prefix -> LiteLLM reverse prefix.

---

## 3. Performance Bottleneck Analysis

### Bottleneck 1: Node.js Startup Time

For `codeburn status` or `codeburn today` in a menubar plugin context, Node.js adds ~80-150ms of startup latency before application code runs. V8 JIT compilation for the React/Ink dependency tree is significant even when the TUI is never rendered. For macOS menubar use (invoked every 5 minutes by SwiftBar), this is the dominant cost. A Go binary starts in under 5ms.

### Bottleneck 2: Sequential Session Parsing

`parseSessionFile` processes each `.jsonl` file via `readline` one line at a time in a single async loop. No parallelism across session files. In `scanProjectDirs`, files within a project are sequential. A user with 50 projects and 200 session files sees all 200 parsed serially.

### Bottleneck 3: Line-by-Line JSONL Parse Overhead

Each line undergoes `JSON.parse()` into a generic `JournalEntry` object. For large session files, this creates allocation pressure in V8's heap. The `entries: JournalEntry[]` array accumulates all entries before grouping into turns -- full in-memory buffer of each session.

### Bottleneck 4: Cursor Provider Memory

The Codex and Cursor parsers read entire files into memory as strings. Cursor materializes the full SQLite query result into an in-memory array. The cursor-cache.ts layer writes all results as a single JSON blob, re-read in entirety on each warm invocation.

### Bottleneck 5: Ink/React Memory Overhead

The React reconciler, virtual DOM, and Ink's terminal rendering layer are loaded for every invocation including non-interactive ones. For `codeburn status --format json`, the TUI is never rendered but all React code is parsed and V8-compiled. Adds ~30-40MB resident memory.

### Summary

| Concern | TS Impact | Go Impact |
|---|---|---|
| Startup time | 80-150ms runtime overhead | ~5ms binary startup |
| Parallel session parsing | Sequential, single-threaded | Goroutines, zero-overhead parallelism |
| Memory per invocation | React + V8 heap ~30-60MB | Typically 5-15MB |
| JSONL parsing throughput | V8 JSON.parse per line | encoding/json or custom scanner |
| SQLite access | Node native addon (better-sqlite3) | Pure Go: modernc.org/sqlite (CGO-free) |
| Binary distribution | Requires Node.js runtime | Single static binary, no runtime |

---

## 4. Go Architecture Design

### Package Structure

```
codeburn/
  cmd/
    codeburn/
      main.go                -- cobra CLI entrypoint
  internal/
    provider/
      provider.go            -- Provider interface + SessionSource type
      claude/
        claude.go            -- discovery + JSONL parser
      codex/
        codex.go             -- discovery + JSONL parser
      cursor/
        cursor.go            -- SQLite query parser
    parser/
      parser.go              -- session file parsing, turn grouping
      cache.go               -- SQLite session cache (same schema, Go driver)
    classifier/
      classifier.go          -- turn category classification (compiled regexp)
    models/
      models.go              -- pricing lookup, cost calculation
      litellm.go             -- LiteLLM fetch + disk cache
    export/
      csv.go
      json.go
    format/
      format.go              -- cost/token formatting
    currency/
      currency.go            -- exchange rate fetch + cache
    config/
      config.go              -- JSON config at ~/.config/codeburn/config.json
    menubar/
      menubar.go             -- SwiftBar/xbar plugin generation
    tui/
      dashboard.go           -- Bubbletea TUI
      panels.go              -- individual panel components
      model.go               -- Bubbletea Update/View/Init
```

### Core Interface Contract

```go
// internal/provider/provider.go
type SessionSource struct {
    Path     string
    Project  string
    Provider string
}

type ParsedCall struct {
    Provider                  string
    Model                     string
    InputTokens               int64
    OutputTokens              int64
    CacheCreationInputTokens  int64
    CacheReadInputTokens      int64
    CachedInputTokens         int64
    ReasoningTokens           int64
    WebSearchRequests         int64
    CostUSD                   float64
    Tools                     []string
    Timestamp                 string
    Speed                     string
    DeduplicationKey          string
    UserMessage               string
    SessionID                 string
}

type Provider interface {
    Name() string
    DisplayName() string
    ModelDisplayName(model string) string
    ToolDisplayName(rawTool string) string
    DiscoverSessions(ctx context.Context) ([]SessionSource, error)
    ParseSession(ctx context.Context, source SessionSource, seen *sync.Map) iter.Seq2[ParsedCall, error]
}
```

Go 1.23 `iter.Seq2` replaces TS `AsyncGenerator`. The caller iterates over a stream -- same conceptual contract.

### Concurrency Model

```go
// internal/parser/parser.go
func ParseAllSessions(ctx context.Context, opts ParseOptions) ([]ProjectSummary, error) {
    sources, err := discoverAll(ctx, opts.ProviderFilter)
    if err != nil {
        return nil, err
    }

    db, _ := cache.Open(DefaultCachePath)
    defer db.Close()

    type result struct {
        session *SessionSummary
        err     error
    }

    sem := make(chan struct{}, runtime.NumCPU()*2) // bounded worker pool
    results := make(chan result, len(sources))
    seen := &sync.Map{}

    var wg sync.WaitGroup
    for _, src := range sources {
        wg.Add(1)
        go func(src SessionSource) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            sess, err := parseSessionFile(ctx, src, seen, db, opts)
            results <- result{sess, err}
        }(src)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect, merge by project, sort
    ...
}
```

Deduplication uses `sync.Map.LoadOrStore` for atomic key insertion across goroutines.

### Pricing

```go
var (
    pricingOnce  sync.Once
    pricingCache map[string]ModelCosts
)

func loadPricing() map[string]ModelCosts {
    pricingOnce.Do(func() {
        pricingCache = loadOrFetch()
    })
    return pricingCache
}
```

The four-level fallback lookup chain translates directly as a helper function with early returns.

---

## 5. TUI Strategy: Ink to Bubbletea

### Mapping

| Ink Concept | Bubbletea Equivalent |
|---|---|
| `React.useState` | Fields on the Model struct |
| `useEffect` with async fetch | `tea.Cmd` returning `tea.Msg` |
| `useInput` | `Update(msg tea.KeyMsg)` |
| `useWindowSize` | `tea.WindowSizeMsg` |
| `useCallback` debounce | Timer `tea.Cmd` |
| `Box flexDirection="column"` | `lipgloss.NewStyle()` + string concatenation |
| `Box width={pw}` | `lipgloss.NewStyle().Width(pw)` |
| `Text color={GOLD}` | `lipgloss.NewStyle().Foreground(lipgloss.Color(GOLD))` |
| `HBar` gradient chars | Manual string builder with `lipgloss.Color` per char |

### Framework Stack

- **charmbracelet/bubbletea** - Elm Architecture TUI framework
- **charmbracelet/lipgloss** - Terminal styling (color, padding, borders, width)
- **charmbracelet/bubbles/table** - Table primitives for breakdown panels

### Complexity Note

The Ink/React dashboard (669 lines) uses flexbox layout, responsive columns, per-character gradient color computation, keyboard navigation, and period switching with debounce. The Bubbletea port is the most labor-intensive component. The gradient HBar rendering, responsive layout calculation, and period-switching debounce are all portable but require careful translation from declarative React to imperative string building.

---

## 6. Dependency Mapping

| TS Dependency | Purpose | Go Equivalent |
|---|---|---|
| `commander` | CLI argument parsing | `github.com/spf13/cobra` |
| `ink` + `react` | TUI framework | `github.com/charmbracelet/bubbletea` |
| (ink styling) | Terminal colors/layout | `github.com/charmbracelet/lipgloss` |
| `chalk` | ANSI color | `lipgloss` or `github.com/fatih/color` |
| `better-sqlite3` | SQLite (cache + Cursor) | `modernc.org/sqlite` (CGO-free) |
| Node `readline` | Line-by-line JSONL | `bufio.NewScanner` |
| Node `fs/promises` | Async file I/O | `os`, `io`, `path/filepath` |
| Node `path` | Path manipulation | `path/filepath` |
| Node `os` | homedir, platform | `os.UserHomeDir()`, `runtime.GOOS` |
| `fetch` (global) | HTTP for LiteLLM + FX rates | `net/http` |
| `Intl.NumberFormat` | Currency formatting | `golang.org/x/text/currency` or hardcoded table |
| `tsup` | Build bundler | `go build -ldflags="-s -w"` |
| `vitest` | Test runner | `testing` (stdlib) |
| `tsx` | Dev runner | `go run ./cmd/codeburn` |

### SQLite Decision

`modernc.org/sqlite` is a pure-Go CGO-free port. It eliminates the native dependency entirely (no build toolchain required for installation). JSON1 extension is included, required for Cursor's `json_extract` queries. This is strictly better than the current better-sqlite3 situation for distribution.

---

## 7. Migration Phases

### Phase 0: Test Harness and Validation Fixtures (2-3 days, Low risk)

Build cross-output comparison tooling. Run both TS and Go implementations against the same fixture data and assert byte-for-byte identical output for non-TUI commands. Use the existing `tests/fixtures/bench/*.jsonl` files as the baseline.

### Phase 1: Core Data Pipeline (2-3 weeks, Medium risk)

Implement in Go:
- `internal/provider/{claude,codex,cursor}`
- `internal/parser` (with concurrent session parsing)
- `internal/classifier`
- `internal/models` (pricing lookup + LiteLLM cache)
- `internal/parser/cache` (SQLite session cache, same schema)

**Performance targets:**
- Cold parse of 5 fixture sessions: < 50ms (vs TS target of 200ms)
- Warm cache hit: < 5ms (vs TS target of 20ms)

**Key decisions:**
- Unify Claude's special-cased parsing path into the standard Provider interface
- Use `sync.Map` for cross-goroutine dedup
- Preserve the privacy invariant: zero `userMessage` after classification, before cache write

### Phase 2: Status, Export, and Menubar Commands (1 week, Low risk)

Non-TUI commands that exercise the full pipeline:
- `codeburn status` (including `--format json/menubar`)
- `codeburn export` (CSV/JSON builders)
- `codeburn menubar install/uninstall`

These require only the data pipeline and string formatting. No TUI involved.

### Phase 3: TUI Dashboard (2-3 weeks, High risk)

Bubbletea port of the Ink/React dashboard:
1. Static layout first (no keyboard interaction) - validate visual output
2. State machine for period switching and provider cycling
3. Auto-refresh timer
4. Gradient bar chart (standalone function, tested in isolation)

8 view components become functions that receive the model and return strings. Layout arithmetic from `getLayout()` maps directly to Go functions.

### Phase 4: Binary Distribution (2-3 days, Low risk)

```bash
go build -ldflags="-s -w" -o codeburn ./cmd/codeburn
```

Single static binary (~10-15MB). Cross-compilation via `GOOS`/`GOARCH`. No Node.js runtime required. Consider goreleaser for automated multi-platform builds.

---

## 8. Risk Assessment

### Low Risk (direct mechanical translation)

- **Classifier**: pure regex + set membership, no framework dependency
- **Models/pricing**: pure arithmetic + map lookups
- **Export**: string building, identical logic
- **Config**: JSON read/write on a simple struct
- **Currency**: HTTP fetch + caching
- **Codex provider parser**: line-by-line JSON decode, state machine

### Medium Risk (architectural decisions required)

- **Session cache**: must preserve `mtime_ms + file_size` fingerprint semantics exactly or cache invalidation breaks
- **Cursor provider**: SQLite JSON1 extension (`json_extract`) must be verified with chosen Go driver
- **Concurrent deduplication**: `sync.Map` is correct for this workload (hundreds of keys, not millions)
- **Codex token accounting**: `prevCumulative/delta` logic is stateful and ordering-dependent; safe because each goroutine handles one file

### High Risk (significant effort or uncertain fidelity)

- **TUI port**: 669 lines of React with flexbox, responsive layout, gradient rendering, keyboard nav, debounce. The Elm Architecture requires translating declarative React to imperative string building. Visual fidelity regression is possible.
- **Currency formatting**: `Intl.NumberFormat` has no Go stdlib equivalent. Pragmatic fix: hardcoded symbol table for the ~17 currencies used in menubar plus a validation set.
- **Subagent sessions**: directory traversal for `{sessionDir}/{uuid}/subagents/*.jsonl` is implicit and under-tested. Must be reproduced exactly.

### What Does NOT Benefit from Go

- macOS menubar widget (shell script calling `codeburn status`)
- Pricing data accuracy (same LiteLLM JSON, same fallback table)
- Exchange rate fetch (network latency dominates)

---

## 9. Incremental Shipping Strategy

Phases 0-2 are independently shippable and cover the highest-value performance wins:
- Menubar startup: ~5ms vs ~100ms
- CLI commands: parallel parsing, lower memory
- Distribution: single binary, no Node.js

The Go binary handles all non-interactive use cases while the TS TUI stays active. Route `codeburn report` to the TS bundle and all other commands to Go until the TUI port (Phase 3) is complete. This maximizes delivered value per unit of risk.

---

## 10. What to Preserve from the TS Implementation

- SQLite session cache with mtime+size fingerprinting (well-designed write-once/read-many)
- `userMessage` zeroing after classification (privacy invariant R7-R9)
- Lazy Cursor load pattern (Go equivalent: `modernc.org/sqlite` as compile-time option via build tags)
- Provider plugin interface (clean and extensible, maps directly to Go)

---

## Estimated Total Effort

| Phase | Effort | Risk |
|---|---|---|
| 0 - Test harness | 2-3 days | Low |
| 1 - Core pipeline | 2-3 weeks | Medium |
| 2 - Non-TUI commands | 1 week | Low |
| 3 - TUI dashboard | 2-3 weeks | High |
| 4 - Binary distribution | 2-3 days | Low |
| **Total** | **6-10 weeks** | |
