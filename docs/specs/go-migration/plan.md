# Plan: CodeBurn Go Migration

## Phase 0: Test Harness and Validation Fixtures (2-3 days)

**Goal:** Build cross-implementation comparison tooling before any Go code exists.

**Deliverables:**

P0.1 - A shell script `scripts/compare-outputs.sh` that runs both the TypeScript CLI and the Go binary against the same fixture data and diffs their output for non-TUI commands.

P0.2 - Extend `tests/fixtures/bench/` with at least one fixture per provider (Claude JSONL, Codex JSONL, Cursor SQLite snapshot). The Cursor fixture must be a minimal valid `state.vscdb` with at least 3 `bubbleId:` entries.

P0.3 - Document the expected output for each fixture (costs, call counts, session counts) in a `tests/fixtures/expected-output.json`. This becomes the ground truth for Go regression tests.

**Acceptance:** Running `scripts/compare-outputs.sh` against the TypeScript implementation produces zero diff for `status --format json`, `export --format csv`, and `export --format json`.

---

## Phase 1: Core Data Pipeline (2-3 weeks)

**Goal:** Go implementations of providers, parser, classifier, models, and session cache. No TUI.

**Deliverables:**

P1.1 - `internal/provider/claude/claude.go`: `discoverSessions` (all three source paths) + `parseSession` (JSONL line scanner, `groupIntoTurns`, `buildSessionSummary`).

P1.2 - `internal/provider/codex/codex.go`: `discoverSessions` (YYYY/MM/DD walk, `session_meta` validation) + `parseSession` (cumulative delta token accounting, tool name normalization).

P1.3 - `internal/provider/cursor/cursor.go`: `discoverSessions` (platform path, existence check) + `parseSession` (SQL query via modernc.org/sqlite, 35-day lookback, user message map, dedup).

P1.4 - `internal/parser/parser.go`: `ParseAllSessions` with bounded worker pool, `sync.Map` dedup, project merging, sort by cost descending. In-process 60s/10-entry cache.

P1.5 - `internal/parser/cache.go`: SQLite session cache (open, get, put, fingerprint, corrupt-recovery).

P1.6 - `internal/classifier/classifier.go`: All 13 categories, precompiled regexes, tool sets, `ClassifyTurn`.

P1.7 - `internal/models/models.go`: FALLBACK_PRICING table, canonical name computation, four-level lookup chain, `CalculateCost`, `GetShortModelName`.

P1.8 - `internal/models/litellm.go`: LiteLLM JSON fetch, 24h disk cache, parse filter (skip entries with `/` or `.` in name).

P1.9 - Go unit tests for: classifier category parity (all 5 bench fixtures), Codex token normalization, dedup key format, session cache hit/miss/invalidation/recovery, privacy invariant (userMessage zeroed in cache), pricing fallback chain, `getCanonicalName`.

**Performance gate (must pass before Phase 2):**

- Cold parse of 5 bench fixtures: < 50ms (measured with `go test -bench`).
- Warm cache hit for same 5 fixtures: < 5ms.

---

## Phase 2: Non-TUI Commands (1 week)

**Goal:** All CLI commands except `report`, `today`, `month`.

**Deliverables:**

P2.1 - `cmd/codeburn/main.go`: cobra CLI entrypoint. Subcommands: `status`, `export`, `install-menubar`, `uninstall-menubar`, `currency`.

P2.2 - `internal/format/format.go`: `FormatCost`, `FormatTokens`, `RenderStatusBar`.

P2.3 - `internal/export/csv.go` + `internal/export/json.go`: All section builders, `escCsv` formula injection protection.

P2.4 - `internal/menubar/menubar.go`: `RenderMenubarFormat`, `InstallMenubar`, `UninstallMenubar`. Plugin script content must be byte-identical to TypeScript output.

P2.5 - `internal/currency/currency.go`: Exchange rate fetch, 24h disk cache, `IsValidCurrencyCode` (hardcoded ISO 4217 set), symbol table, fraction digits table.

P2.6 - `internal/config/config.go`: JSON read/write at `~/.config/codeburn/config.json`.

P2.7 - Integration tests: run `codeburn status --format json` and `codeburn status --format terminal` against fixture data. Assert output matches expected (from P0.3).

P2.8 - Run `scripts/compare-outputs.sh`. Zero diff for all non-TUI commands is the phase gate.

**Performance gate:** `codeburn status --format json` completes in < 100ms wall time on warm cache.

---

## Phase 3: TUI Dashboard (2-3 weeks)

**Goal:** Full Bubbletea port of the Ink dashboard.

**Deliverables:**

P3.1 - `internal/tui/model.go`: Bubbletea Model struct (period, projects, loading, activeProvider, detectedProviders, termWidth, debounceTimer, refreshTimer). Init, Update, View functions.

P3.2 - `internal/tui/dashboard.go`: `DashboardContent` function returning a string. Calls panel functions.

P3.3 - `internal/tui/panels.go`: All 8 panel functions: `renderOverview`, `renderDailyActivity`, `renderProjectBreakdown`, `renderModelBreakdown`, `renderActivityBreakdown`, `renderToolBreakdown`, `renderMcpBreakdown`, `renderBashBreakdown`.

P3.4 - `internal/tui/gradient.go`: `gradientColor(pct float64) string` - the three-segment RGB interpolation. Must be independently unit-tested against the TypeScript reference.

P3.5 - `internal/tui/layout.go`: `GetLayout(termWidth int) Layout` - responsive layout computation.

P3.6 - Period switching debounce: 600ms timer implemented as a `tea.Cmd` that fires a custom `periodLoadMsg` after delay. Arrow/Tab keys set the pending period and restart the timer. Number keys cancel any pending timer and load immediately.

P3.7 - Auto-refresh: `tea.Tick` with the configured interval, firing a `refreshMsg` that triggers `ParseAllSessions`.

P3.8 - Static render path: when not a TTY, `render()` is called once and the model's View is written to stdout, then the program exits.

P3.9 - Manual visual validation checklist (must be completed before phase gate):
  
- Narrow terminal (< 90 cols): single-column layout
- Wide terminal (>= 90 cols): two-column layout
- 8 panels present with correct colors
- Gradient bar renders correctly at various fill ratios
- Period switching via arrows and number keys
- Provider cycling with `p` key (with multiple providers active)
- Auto-refresh updates display

**Phase gate:** All visual validation items checked. `scripts/compare-outputs.sh` still passes. All unit tests pass.

---

## Phase 4: Binary Distribution (2-3 days)

**Goal:** Ship Go binary as the primary artifact. Retire the routing wrapper.

**Deliverables:**

P4.1 - `goreleaser.yaml` configuration for automated multi-platform builds: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`.

P4.2 - GitHub Actions workflow: build and publish binaries on `v*` tags. Attach binaries to GitHub Releases.

P4.3 - Homebrew tap formula: `codeburn.rb` targeting the darwin/arm64 and darwin/amd64 builds.

P4.4 - Update `package.json` postinstall to detect the Go binary and use it as the executable. Retain Node.js bundle as fallback for platforms without a prebuilt Go binary.

P4.5 - Remove the shell wrapper from Phase 0 routing. The Go binary handles all commands.

P4.6 - Documentation update (README): remove "requires Node.js" note. Add "install via Homebrew" instructions.

---

## Testing Strategy

### 7.1 Cross-Implementation Validation (Primary)

The most important tests are those that run identical input through both the TypeScript and Go implementations and assert identical output.

**Test fixture corpus:**

- `tests/fixtures/bench/session-coding.jsonl` (Claude JSONL)
- `tests/fixtures/bench/session-debugging.jsonl` (Claude JSONL)
- `tests/fixtures/bench/session-feature.jsonl` (Claude JSONL)
- `tests/fixtures/bench/session-refactoring.jsonl` (Claude JSONL)
- `tests/fixtures/bench/session-testing.jsonl` (Claude JSONL)
- (To be created) `tests/fixtures/bench/codex-sample.jsonl` - minimal Codex JSONL with `session_meta` + 3 token events
- (To be created) `tests/fixtures/bench/cursor-sample.vscdb` - minimal SQLite with `cursorDiskKV` table and 3 `bubbleId:` entries

**Cross-implementation test targets:**

- `status --format json`: byte-identical JSON
- `export --format csv`: structurally identical (ignore ISO timestamp in generated field)
- `export --format json`: structurally identical
- `status --format terminal`: structurally identical (strip ANSI codes before comparison)
- Category classification: identical `TaskCategory` for every turn in the bench fixtures

### 7.2 Unit Tests (Go, stdlib testing package)

| Area | Test Cases |
| --- | --- |
| `classifier` | All 13 categories; tool-pattern path; keyword refinement path; tool-less conversation path; retryCount; hasEdits |
| `models` | Canonical name computation (strip `@suffix`, strip `-YYYYMMDD`); four-level fallback; `CalculateCost` with all 7 dimensions; `GetShortModelName` |
| `parser/cache` | Hit (exact match); miss (mtime changed); miss (size changed); corrupt recovery; concurrent reads; userMessage zeroed in stored JSON |
| `provider/claude` | `groupIntoTurns`; dedup by message.id; dedup fallback to timestamp; subagent file inclusion; invalid JSON line skipped |
| `provider/codex` | `session_meta` validation; cumulative delta accounting; `last_token_usage` vs delta path; tool name normalization; dedupKey format |
| `provider/cursor` | SQL query correctness; 35-day time floor; dedupKey format; missing DB returns empty; schema validation error message |
| `currency` | Valid ISO 4217 codes accepted; invalid codes rejected; symbol table correctness; fraction digits (JPY=0, USD=2) |
| `export/csv` | Formula injection protection (`=`, `+`, `-`, `@`); comma in cell value; newline in cell value; section headers |
| `format` | `FormatTokens` boundaries (0, 999, 1000, 999999, 1000000); `FormatCost` tiers (>1, 0.01-1, <0.01) |
| `models` (fast) | `CalculateCost` with `speed: fast` applies `fastMultiplier`; standard speed uses multiplier 1; model without explicit fast field defaults to 1 |

### 7.3 Benchmark Tests

```go
// go test -bench=BenchmarkParseAllSessions -benchmem ./internal/parser/...
// Target: < 50ms for 5 fixture files (cold), < 5ms (warm cache)
```

### 7.4 Integration Tests

Run as part of CI, require no network access:

- `codeburn status --format json` (mocked pricing, fixture data)
- `codeburn export --format csv` (fixture data, output to temp file)
- `codeburn export --format json` (fixture data, output to temp file)
- `codeburn currency NOTACODE` (expect exit code 1)
- `codeburn currency --reset` (expect config file has no currency key)

### 7.5 Privacy Invariant Test

```go
func TestUserMessageZeroedInCache(t *testing.T) {
    // Parse a fixture JSONL file that contains user messages
    // Write to a temp session cache
    // Read back the summary_json from SQLite
    // Unmarshal and assert every ClassifiedTurn.UserMessage == ""
}
```

This test must run in CI. Failure of this test is a hard block on shipping.

---

## Risk Register

| ID | Risk | Likelihood | Impact | Mitigation |
| --- | --- | --- | --- | --- |
| RISK-1 | TUI visual regression (gradient colors, layout) | High | Medium | Manual visual checklist (P3.9); automated pixel-level comparison is impractical for terminal output |
| RISK-2 | Codex cumulative delta logic breaks with a new Codex JSONL format | Medium | High | Validate against real Codex sessions during Phase 1; add a fixture for each Codex JSONL format variant |
| RISK-3 | Cursor DB schema changes (new Cursor version) | Medium | High | Schema validation (R41, AC47) emits actionable error; version the query in a const; document upgrade path |
| RISK-4 | LiteLLM JSON format change breaks pricing parse | Low | Medium | FALLBACK_PRICING table ensures cost calculation continues; alert path via `CODEBURN_DEBUG=1` (R46) |
| RISK-5 | `modernc.org/sqlite` JSON1 behavior divergence from SQLite reference | Low | High | Integration test Cursor SQL queries against a reference `.vscdb` fixture; compare row counts and values |
| RISK-6 | Privacy invariant broken by new code path | Low | Critical | Unit test `TestUserMessageZeroedInCache` (section 7.5) gates every PR |
| RISK-7 | Session cache schema collision between TS and Go versions | Low | Medium | Schema is identical (constraint C11); no migration needed; any schema change must be coordinated |
| RISK-8 | `sync.Map` contention on very large session histories (1000+ files) | Low | Low | `sync.Map` is designed for this pattern; benchmark with 1000 synthetic sources if needed |
| RISK-9 | Binary startup regression after adding TUI dependencies | Low | Medium | Benchmark binary startup time in CI (R51); measure RSS after Phase 3 to verify R37 |
| RISK-10 | Desktop walk (depth 8) misses sessions for unusual Claude Desktop install paths | Low | Low | Document limitation; the TypeScript implementation has the same limit |
