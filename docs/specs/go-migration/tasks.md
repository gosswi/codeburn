# Tasks: CodeBurn Go Migration

**Generated from**: docs/specs/go-migration-spec.md (v1.2)
**Total tasks**: 28
**Parallel groups**: 8 (plus 4 sequential tasks)
**Estimated total effort**: 72h
**Max parallel agents**: 4

---

## File Conflict Matrix

| Task | Files Created/Modified | Conflicts With |
|------|----------------------|----------------|
| T1 | go.mod, go.sum | T2 (go.mod deps) |
| T2 | internal/provider/provider.go, internal/types/types.go | — |
| T3 | internal/classifier/classifier.go, classifier_test.go | — |
| T4 | internal/models/models.go, models_test.go | — |
| T5 | internal/models/litellm.go, litellm_test.go | — |
| T6 | internal/parser/cache.go, cache_test.go | — |
| T7 | internal/provider/claude/claude.go, claude_test.go | — |
| T8 | internal/provider/codex/codex.go, codex_test.go | — |
| T9 | internal/provider/cursor/cursor.go, cursor_test.go | — |
| T10 | internal/parser/parser.go, parser_test.go | — |
| T11 | internal/parser/parser.go (in-process cache) | T10 (shared file) |
| T12 | tests/fixtures/bench/codex-sample.jsonl, cursor-sample.vscdb | — |
| T13 | internal/parser/parser_bench_test.go | T10 (reads parser.go) |
| T14 | cmd/codeburn/main.go, internal/config/config.go | — |
| T15 | internal/currency/currency.go, currency_test.go | — |
| T16 | internal/format/format.go, format_test.go | — |
| T17 | internal/export/csv.go, csv_test.go | — |
| T18 | internal/export/json.go, json_test.go | — |
| T19 | internal/menubar/menubar.go, menubar_test.go | — |
| T20 | cmd/codeburn/main.go (status/export/currency wiring) | T14 (shared file) |
| T21 | scripts/compare-outputs.sh | — |
| T22 | internal/tui/gradient.go, gradient_test.go | — |
| T23 | internal/tui/layout.go, layout_test.go | — |
| T24 | internal/tui/model.go, internal/tui/dashboard.go | — |
| T25 | internal/tui/panels.go | — |
| T26 | cmd/codeburn/main.go (report/today/month wiring) | T20 (shared file) |
| T27 | .goreleaser.yaml, .github/workflows/release.yml | — |
| T28 | README.md, Homebrew formula | — |

---

## Task Graph

```
T1 ──→ T2 ──┬──→ T3  ─────────────────────────────────────────┐
             ├──→ T4 ──→ T5                                    │
             ├──→ T6                                            │
             ├──→ T7  ──┐                                       │
             ├──→ T8  ──┤                                       │
             └──→ T9  ──┤                                       │
                        ├──→ T10 ──→ T11 ──→ T13               │
                        │                                       │
                        └──→ T12                                │
                                                                │
              T10 ──┬──→ T14 ──→ T20 ──→ T21                   │
                    │                                           │
                    ├──→ T15                                    │
                    ├──→ T16 ──┬──→ T17                         │
                    │          └──→ T18                         │
                    └──→ T19                                    │
                                                                │
                         T20 ──→ T22 ──┐                        │
                                T23 ──┤                         │
                                      ├──→ T24 ──→ T25 ──→ T26 │
                                T3  ──┘                         │
                                                                │
                                       T26 ──→ T27 ──→ T28     │
```

---

## Phase 0: Test Harness and Validation Fixtures

### Group 0: Project Init (sequential)

- **T1**: Initialize Go module and install dependencies
  - **Validates**: C1, C2, C5
  - **Files**: `go.mod`, `go.sum`
  - **Complexity**: low
  - **Effort**: 1h
  - **Details**: `go mod init github.com/user/codeburn`, add deps: `modernc.org/sqlite`, `github.com/spf13/cobra`. Set Go 1.23 minimum. Verify `CGO_ENABLED=0 go build` succeeds.

- **T2**: Define shared types and Provider interface
  - **Validates**: R1, D3, D5
  - **Files**: `internal/provider/provider.go`, `internal/types/types.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T1
  - **Details**: Port `ParsedCall`, `SessionSource`, `ClassifiedTurn`, `SessionSummary`, `ProjectSummary` structs. Define `Provider` interface with `iter.Seq2[ParsedCall, error]` return. Port `DateRange`, `ParseOptions`, `TaskCategory` enum.

---

## Phase 1: Core Data Pipeline

### Group 1: Independent Core Modules (depends on T2)

- [P] **T3**: Implement classifier with 13 categories
  - **Validates**: R23, AC18
  - **Files**: `internal/classifier/classifier.go`, `internal/classifier/classifier_test.go`
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T2
  - **Details**: Port all 13 categories, 5 tool sets (EDIT_TOOLS, READ_TOOLS, BASH_TOOLS, TASK_TOOLS, SEARCH_TOOLS), 12 regex patterns. Regexes must be compiled at `var` init time (C6). Port `classifyTurn()`. Write unit tests for all 13 categories, tool-pattern path, keyword refinement, retryCount, hasEdits.

- [P] **T4**: Implement FALLBACK_PRICING and cost calculation
  - **Validates**: R19, R21, R53, AC11, AC29, AC55
  - **Files**: `internal/models/models.go`, `internal/models/models_test.go`
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T2
  - **Details**: Port all 18 FALLBACK_PRICING entries with exact per-token costs. Implement `getCanonicalName` (strip `@suffix`, `-YYYYMMDD`). Implement four-level fallback chain. Implement `CalculateCost` with 7 dimensions including `speed`/`fastMultiplier`. Implement `GetShortModelName`. Write unit tests for canonical name, fallback chain, cost calculation with fast mode, short names.

- [P] **T5**: Implement LiteLLM pricing fetch and disk cache
  - **Validates**: R20, AC28
  - **Files**: `internal/models/litellm.go`, `internal/models/litellm_test.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T4
  - **Details**: Fetch from LiteLLM URL, parse JSON, filter entries with `/` or `.` in name. Cache at `~/.cache/codeburn/litellm-pricing.json` with 24h TTL. On fetch failure, fall back to disk cache, then FALLBACK_PRICING. No user-visible error. Write tests for parse, cache TTL, fallback chain.

- [P] **T6**: Implement SQLite session cache
  - **Validates**: R16, R17, AC5, AC6, AC12
  - **Files**: `internal/parser/cache.go`, `internal/parser/cache_test.go`
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T2
  - **Details**: Use `modernc.org/sqlite`. Create table with exact schema from spec. WAL mode, busy_timeout=3000ms. Implement `Open`, `getCachedSummary` (match filePath+mtimeMs+fileSize), `putCachedSummary`. Corrupt recovery: delete and recreate on open failure. Write tests for hit, miss (mtime changed), miss (size changed), corrupt recovery, concurrent reads.

### Group 2: Provider Implementations (depends on T2)

- [P] **T7**: Implement Claude provider
  - **Validates**: R10, R13, R38, R39, AC8, AC16, AC38, AC39
  - **Files**: `internal/provider/claude/claude.go`, `internal/provider/claude/claude_test.go`
  - **Complexity**: high
  - **Effort**: 4h
  - **Depends on**: T2
  - **Details**: Implement `discoverSessions` for all 3 source locations: `$CLAUDE_CONFIG_DIR/projects/*`, platform-specific desktop sessions, subagent JSONL at `{sessionDir}/{uuid}/subagents/*.jsonl`. Implement desktop walk with depth-8 limit (D9). Implement `parseSession` as `iter.Seq2`: JSONL line scanner, `groupIntoTurns`, `buildSessionSummary`. Dedup key: `msg.id` with fallback `claude:<timestamp>`. Skip invalid JSONL lines silently. Skip unreadable directories silently. Write tests for groupIntoTurns, dedup, subagent inclusion, invalid JSON resilience.

- [P] **T8**: Implement Codex provider
  - **Validates**: R11, R13, R14, AC9, AC26
  - **Files**: `internal/provider/codex/codex.go`, `internal/provider/codex/codex_test.go`
  - **Complexity**: high
  - **Effort**: 4h
  - **Depends on**: T2
  - **Details**: Implement `discoverSessions`: walk `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-*.jsonl`, validate first line is `session_meta` with `payload.originator` starting with `codex`. Implement `parseSession`: cumulative delta token accounting, `uncachedInputTokens = max(0, inputTokens - cachedInputTokens)` normalization, tool name normalization. Dedup key: `codex:<filepath>:<timestamp>:<cumulativeTotal>`. Write tests for session_meta validation, delta accounting, token normalization, dedup key format.

- [P] **T9**: Implement Cursor provider
  - **Validates**: R12, R40, R41, R52, AC10, AC13, AC27, AC40, AC47, AC54
  - **Files**: `internal/provider/cursor/cursor.go`, `internal/provider/cursor/cursor_test.go`, `internal/cursor-cache.go`
  - **Complexity**: high
  - **Effort**: 4h
  - **Depends on**: T2
  - **Details**: Use `modernc.org/sqlite` (D1). Implement `discoverSessions`: platform-specific path, existence check, return zero sessions silently if missing. Implement `parseSession`: SQL queries matching TS exactly (`bubbleId:` key prefix, `json_extract` paths). 35-day lookback (R52). User message map. Dedup key: `cursor:<conversationId>:<createdAt>:<inputTokens>:<outputTokens>`. Schema validation: if `cursorDiskKV` table or `bubbleId:` keys missing, write warning to stderr. Cursor cache: JSON file at `~/.cache/codeburn/cursor-results.json` with `mtimeMs:size` fingerprint. Write tests for SQL correctness, 35-day floor, missing DB, schema warning.

### Group 3: Parser Assembly (depends on Groups 1+2)

- **T10**: Implement ParseAllSessions with concurrent worker pool
  - **Validates**: R13, R15, D2, D5, D7, D8, AC7, AC8
  - **Files**: `internal/parser/parser.go`, `internal/parser/parser_test.go`
  - **Complexity**: high
  - **Effort**: 4h
  - **Depends on**: T3, T4, T6, T7, T8, T9
  - **Cannot parallelize**: T11 modifies same file
  - **Details**: Implement `ParseAllSessions` with bounded goroutine pool (`runtime.NumCPU() * 2`), `sync.Map` for dedup. Wire providers via unified interface. For each session source: check cache -> parse if miss -> classify turns -> zero `userMessage` (privacy invariant, R15) -> cache result -> aggregate. Merge projects, sort by cost descending. Date filtering applied after cache retrieval (D7). Write tests for global dedup scope, privacy invariant (userMessage zeroed), project merging.

- **T11**: Implement in-process session cache (60s/10-entry LRU)
  - **Validates**: R25, AC32
  - **Files**: `internal/parser/parser.go` (additions to T10's file)
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T10
  - **Details**: Add 60-second TTL, 10-entry LRU keyed by `dateRange+provider+extractBash`. Cache stores full `[]ProjectSummary`. On hit within TTL, return cached result without SQLite or JSONL re-read. Write test for cache hit within 60s, cache miss after TTL.

### Group 4: Fixtures and Benchmarks (depends on Group 3)

- [P] **T12**: Create Codex and Cursor test fixtures
  - **Validates**: P0.2, AC26, AC10
  - **Files**: `tests/fixtures/bench/codex-sample.jsonl`, `tests/fixtures/bench/cursor-sample.vscdb`
  - **Complexity**: low
  - **Effort**: 2h
  - **Depends on**: T8, T9
  - **Details**: Create minimal Codex JSONL with `session_meta` + 3 token events demonstrating cumulative delta accounting. Create minimal SQLite `state.vscdb` with `cursorDiskKV` table and 3 `bubbleId:` entries (one outside 35-day window for lookback test). Document expected output in `tests/fixtures/expected-output.json`.

- [P] **T13**: Implement benchmark tests and verify Phase 1 performance gates
  - **Validates**: R34, R35, AC45, AC5
  - **Files**: `internal/parser/parser_bench_test.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T10, T12
  - **Details**: Write `BenchmarkParseAllSessions` targeting 5 bench fixtures. Cold parse: <50ms. Warm cache: <5ms. Use `go test -bench -benchmem`. This is the Phase 1 performance gate.

---

## Phase 2: Non-TUI Commands

### Group 5: Command Infrastructure (depends on Phase 1)

- [P] **T14**: Implement cobra CLI entrypoint and config module
  - **Validates**: R1, R8, R49, R50, AC1, AC19, AC51, AC52
  - **Files**: `cmd/codeburn/main.go`, `internal/config/config.go`, `internal/config/config_test.go`
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T10
  - **Details**: Cobra CLI with all 8 subcommands registered. `config.go`: read/write `~/.config/codeburn/config.json`. Invalid JSON treated as missing (R50). `currency` subcommand: validate ISO 4217 via hardcoded set, `--reset` flag. Exit codes: 0 success, 1 user errors (R49). Write config tests.

- [P] **T15**: Implement currency exchange rate module
  - **Validates**: R22, AC30
  - **Files**: `internal/currency/currency.go`, `internal/currency/currency_test.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T10
  - **Details**: Fetch from Frankfurter API. Cache at `~/.cache/codeburn/exchange-rate.json` with 24h TTL and schema `{ timestamp, code, rate }`. On failure, default rate to 1 (identity). Hardcoded symbol table for 17 currencies with fraction digits (JPY/KRW: 0, others: 2). `IsValidCurrencyCode` from hardcoded ISO 4217 set.

- [P] **T16**: Implement format utilities
  - **Validates**: R2, R5, AC2, AC22
  - **Files**: `internal/format/format.go`, `internal/format/format_test.go`
  - **Complexity**: low
  - **Effort**: 2h
  - **Depends on**: T10
  - **Details**: `FormatCost` with tiers (>1, 0.01-1, <0.01). `FormatTokens` with K/M suffixes. `RenderStatusBar` for menubar. Terminal status format: bold Today/Month, yellow costs, dim counts. JSON status format. Write tests for boundaries.

### Group 6: Export and Menubar (depends on Group 5)

- [P] **T17**: Implement CSV export
  - **Validates**: R3, R47, AC3, AC4, AC49
  - **Files**: `internal/export/csv.go`, `internal/export/csv_test.go`
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T16
  - **Details**: All section builders matching TS section order exactly. `escCsv` formula injection: prefix `=`, `+`, `-`, `@` with tab. Cost column header: `Cost (<currency code>)`. Sort: cost descending within sections, dates ascending in daily. `--output` flag support. Write tests for injection protection, comma/newline in cells, section headers.

- [P] **T18**: Implement JSON export
  - **Validates**: R4, R47, AC21, AC49
  - **Files**: `internal/export/json.go`, `internal/export/json_test.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T16
  - **Details**: Schema: `generated` (ISO8601), `periods` map by label, `tools`, `shellCommands`, `projects`. Field names, types, nesting must match TS exactly. `--output` flag support. Write tests for structural match.

- [P] **T19**: Implement menubar install/uninstall and format
  - **Validates**: R6, R7, R45, AC23, AC24, AC25, AC42
  - **Files**: `internal/menubar/menubar.go`, `internal/menubar/menubar_test.go`
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T16
  - **Details**: `RenderMenubarFormat`: title line `<cost> | sfimage=flame.fill color=#FF8C42`, `--` prefix, `font=Menlo`, currency submenu with 17 entries in exact order. `InstallMenubar`: write `codeburn.5m.sh` to detected plugin dir, chmod 0755. SwiftBar priority over xbar (R45). Non-macOS: print "only available on macOS". Write tests for format structure, SwiftBar priority.

### Group 7: CLI Wiring and Cross-Implementation Validation (sequential)

- **T20**: Wire status, export, currency commands into cobra CLI
  - **Validates**: R2, R44, R46, R48, AC2, AC20, AC48, AC50
  - **Files**: `cmd/codeburn/main.go` (additions to T14)
  - **Complexity**: medium
  - **Effort**: 3h
  - **Depends on**: T14, T15, T16, T17, T18, T19
  - **Cannot parallelize**: modifies shared `main.go`
  - **Details**: Wire `status` (json/terminal/menubar formats), `export` (csv/json with `-o`), `currency` (set/reset/list). Add `--provider` filter flag to status/export. Add `CODEBURN_DEBUG=1` stderr diagnostics (R46): discovery counts, cache hit/miss, pricing source. Empty data must produce valid JSON with zero counters (R44). Write integration tests.

- **T21**: Create cross-implementation comparison script and validate
  - **Validates**: P0.1, P0.3, AC2, AC3, AC21
  - **Files**: `scripts/compare-outputs.sh`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T20
  - **Details**: Shell script that runs both `npx tsx src/cli.ts` and `./codeburn` against fixture data, diffs output for `status --format json`, `export --format csv`, `export --format json`. Zero diff is the Phase 2 gate. Measure `codeburn status --format json` wall time (<100ms on warm cache, R36). Measure RSS (<30MB, R37).

---

## Phase 3: TUI Dashboard

### Group 8: TUI Building Blocks (depends on Phase 2)

- [P] **T22**: Implement gradient bar rendering
  - **Validates**: R28, AC15
  - **Files**: `internal/tui/gradient.go`, `internal/tui/gradient_test.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T20
  - **Details**: `gradientColor(pct float64) string`: three-segment RGB interpolation `[91,158,245] -> [245,200,91] -> [255,140,66] -> [245,91,91]`. `HBar(width, value, max int) string`: filled `█`, unfilled `░` in `#333333`, empty bars (max=0) all dim `░`. Must be independently unit-tested against TS reference with +/-2 RGB tolerance.

- [P] **T23**: Implement responsive layout computation
  - **Validates**: R27, AC44
  - **Files**: `internal/tui/layout.go`, `internal/tui/layout_test.go`
  - **Complexity**: low
  - **Effort**: 1h
  - **Depends on**: T20
  - **Details**: `GetLayout(termWidth int) Layout`: 2-column at >=90 cols, single-column below. Cap at 160 cols. `barWidth = max(6, min(10, inner - 30))` where `inner = halfWidth - 4`. Write tests for breakpoints.

### Group 9: TUI Assembly (depends on Group 8)

- **T24**: Implement Bubbletea model with keyboard handling and timers
  - **Validates**: R9, R29, R30, R31, R32, AC17, AC33, AC34, AC35, AC36
  - **Files**: `internal/tui/model.go`, `internal/tui/dashboard.go`
  - **Complexity**: high
  - **Effort**: 4h
  - **Depends on**: T3, T22, T23
  - **Details**: Bubbletea Model: period, projects, loading, activeProvider, detectedProviders, termWidth, debounceTimer, refreshTimer. Init: detect providers via `discoverSessions()` for each. Update: `q` quit, `p` cycle provider, arrows cycle period with 600ms debounce, `1-4` direct selection bypasses debounce, auto-refresh via `tea.Tick`. View: call `DashboardContent`. Non-TTY: render single static frame and exit. `shortProject` display transformation (R24).

- **T25**: Implement all 8 TUI panels
  - **Validates**: R26, R33, AC43, AC37
  - **Files**: `internal/tui/panels.go`
  - **Complexity**: high
  - **Effort**: 4h
  - **Depends on**: T24
  - **Details**: Port all 8 panels: `renderOverview`, `renderDailyActivity`, `renderProjectBreakdown`, `renderModelBreakdown`, `renderActivityBreakdown`, `renderToolBreakdown`, `renderMcpBreakdown`, `renderBashBreakdown`. Match PANEL_COLORS hex values. Hide mcp/bash when empty. Cursor "Languages" panel from `lang:` prefixed tools when Cursor is active provider.

### Group 10: TUI CLI Wiring (sequential)

- **T26**: Wire report/today/month commands and remove TS routing wrapper
  - **Validates**: R9, R51, AC17, AC46, AC53
  - **Files**: `cmd/codeburn/main.go` (additions)
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T25
  - **Details**: Wire `report`, `today`, `month` subcommands to TUI. Add `--refresh` flag. Remove the Phase 0 shell routing wrapper. Run Phase 3 visual validation checklist (P3.9). Re-measure RSS for `status --format json` to verify R37/R51 (<30MB after TUI deps).

---

## Phase 4: Binary Distribution

### Group 11: Release Infrastructure (depends on Phase 3)

- [P] **T27**: Configure goreleaser and GitHub Actions
  - **Validates**: P4.1, P4.2, C2, C3
  - **Files**: `.goreleaser.yaml`, `.github/workflows/release.yml`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T26
  - **Details**: goreleaser config for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`. `CGO_ENABLED=0` with `-ldflags="-s -w"`. GitHub Actions: build on `v*` tags, attach binaries to Releases.

- [P] **T28**: Homebrew tap and documentation update
  - **Validates**: P4.3, P4.4, P4.6
  - **Files**: Homebrew formula, `README.md`, `package.json`
  - **Complexity**: low
  - **Effort**: 2h
  - **Depends on**: T27
  - **Details**: `codeburn.rb` formula targeting darwin builds. Update `package.json` postinstall to detect Go binary. Update README: remove "requires Node.js", add Homebrew install instructions.

---

## Execution Summary

| Phase | Tasks | Parallel Groups | Max Agents | Effort |
|-------|-------|----------------|------------|--------|
| 0 (Init) | T1-T2 | 0 (sequential) | 1 | 3h |
| 1 (Data Pipeline) | T3-T13 | 3 (Groups 1-4) | 4 | 33h |
| 2 (Non-TUI Commands) | T14-T21 | 2 (Groups 5-6) | 4 | 20h |
| 3 (TUI Dashboard) | T22-T26 | 1 (Group 8) | 2 | 13h |
| 4 (Distribution) | T27-T28 | 1 (Group 11) | 2 | 4h |
| **Total** | **28** | **8** | **4** | **73h** |

## Critical Path

T1 -> T2 -> T7 -> T10 -> T11 -> T13 (Phase 1 gate) -> T14 -> T20 -> T21 (Phase 2 gate) -> T24 -> T25 -> T26 (Phase 3 gate) -> T27 -> T28

**Critical path effort**: ~38h (about half of total, rest is parallelizable)

## Phase Gates

| Gate | Tasks Required | Criteria |
|------|---------------|----------|
| Phase 1 | T3-T13 all complete | Cold parse <50ms, warm <5ms, all unit tests pass |
| Phase 2 | T14-T21 all complete | `compare-outputs.sh` zero diff, `status --format json` <100ms, RSS <30MB |
| Phase 3 | T22-T26 all complete | Visual checklist (P3.9) passes, `compare-outputs.sh` still passes, RSS <30MB after TUI deps |
| Phase 4 | T27-T28 all complete | Binaries build for all 4 platforms, Homebrew formula installs correctly |
