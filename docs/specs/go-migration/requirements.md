# Requirements: CodeBurn Go Migration

## 2.1 Behavioral Compatibility

**R1** - The Go binary must accept all CLI subcommands and flags defined in the TypeScript implementation: `report`, `status`, `today`, `month`, `export`, `install-menubar`, `uninstall-menubar`, `currency`. All flags must use the same names and defaults.

**R2** - The `status --format json` output must be byte-compatible with the TypeScript implementation. The JSON schema is:

```json
{
  "currency": "<code>",
  "today": { "cost": <float>, "calls": <int> },
  "month": { "cost": <float>, "calls": <int> }
}
```

Cost values must be rounded to two decimal places. Costs are in the active currency (not USD).

**R3** - The `export --format csv` output must be structurally identical: same section headers (prefixed `#`), same column names, same sort order (cost descending within each section, dates ascending in daily), same formula-injection protection (prefix `=`, `+`, `-`, `@` with a tab character, then quote if commas or newlines exist). The cost column header must be `Cost (<currency code>)`.

**R4** - The `export --format json` output must be structurally identical: `generated` (ISO8601), `periods` map by label, `tools`, `shellCommands`, `projects`. Section schemas must match exactly (field names, types, nesting).

**R5** - The `status --format terminal` output must render the same layout: newline-padded, bold labels `Today` and `Month`, yellow cost values, dim call counts.

**R6** - The `status --format menubar` output must be pipe-compatible with SwiftBar/xbar. Title line format: `<cost> | sfimage=flame.fill color=#FF8C42`. Section structure, indentation levels (`--` prefix), `font=Menlo`, `size=` attributes, and currency submenu entries (17 currencies in the exact order from the TypeScript source) must all be preserved.

**R7** - The `install-menubar` command must write a shell script to the same path (`codeburn.5m.sh` in the detected plugin directory) with the same content, chmod it to 0755, and print the same confirmation messages. On non-macOS platforms, it must print the same "only available on macOS" message.

**R8** - The `currency` command must read and write `~/.config/codeburn/config.json` with the same schema: `{ "currency": { "code": string, "symbol"?: string } }`. Invalid ISO 4217 codes must exit with code 1 and print `"<code>" is not a valid ISO 4217 currency code.` (same wording). The `--reset` flag must remove the currency key.

**R9** - The `report`, `today`, and `month` commands must launch the TUI dashboard. In non-TTY mode (piped stdout), they must render a single static frame and exit.

## 2.2 Data Pipeline Correctness

**R10** - The Claude provider must discover sessions from all three source locations:

- `$CLAUDE_CONFIG_DIR/projects/*` (default: `~/.claude/projects/*`)
- Platform-specific local-agent-mode-sessions directory (macOS/Windows/Linux paths identical to TypeScript)
- Subagent JSONL files at `{sessionDir}/{uuid}/subagents/*.jsonl`

**R11** - The Codex provider must discover sessions by walking `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-*.jsonl` (default: `~/.codex/sessions/...`). Only files whose first line is a `session_meta` entry with `payload.originator` starting with `codex` are valid.

**R12** - The Cursor provider must read `state.vscdb` from the platform-specific path. It must use `modernc.org/sqlite` (CGO-free). The SQL queries and JSON extraction paths must exactly match the TypeScript queries (`bubbleId:` key prefix, `$.tokenCount.inputTokens`, `$.tokenCount.outputTokens`, `$.modelInfo.modelName`, `$.createdAt`, `$.conversationId`, `$.text`, `$.codeBlocks`, `$.type`).

**R13** - Deduplication must be globally scoped per `ParseAllSessions` invocation (not per-file). The three deduplication key formats must be preserved exactly:

- Claude: `msg.id` from the assistant entry's message field, fallback `claude:<timestamp>`
- Codex: `codex:<filepath>:<timestamp>:<cumulativeTotal>`
- Cursor: `cursor:<conversationId>:<createdAt>:<inputTokens>:<outputTokens>`

**R14** - The Codex token normalization must be preserved: `uncachedInputTokens = max(0, inputTokens - cachedInputTokens)`. Codex includes cached tokens inside `input_tokens`; the Go implementation must apply the same normalization before cost calculation.

**R15** - The privacy invariant must be preserved: `userMessage` must be zeroed (set to empty string) on every `ClassifiedTurn` after `classifyTurn()` completes and before the data is written to the session cache or any aggregate. The session cache on disk must never contain user message text.

**R16** - The session cache must use SQLite at `~/.cache/codeburn/session-cache.db` with the identical schema:

```sql
CREATE TABLE IF NOT EXISTS session_summaries (
  file_path TEXT PRIMARY KEY,
  mtime_ms INTEGER NOT NULL,
  file_size INTEGER NOT NULL,
  summary_json TEXT NOT NULL,
  cached_at INTEGER NOT NULL
)
```

WAL mode and busy_timeout=3000ms must be set. Cache hit requires all three of `file_path`, `mtime_ms`, and `file_size` to match. `mtime_ms` must be stored as `floor(statResult.ModTime().UnixMilli())`.

**R17** - When the session cache SQLite database is corrupt (fails to open or init), the Go implementation must delete the file and recreate it (same recovery behavior as `tryInit` in `session-cache.ts`). If recreation also fails, parsing must continue without caching.

**R18** - The Cursor cache must be preserved: a JSON file at `~/.cache/codeburn/cursor-results.json` with schema `{ "fingerprint": "<mtimeMs>:<size>", "results": [...] }`. The entire cache is invalidated when the fingerprint of `state.vscdb` changes.

**R19** - The pricing lookup must implement the same four-level fallback chain:

1. Exact match in LiteLLM pricing map (canonical model name)
2. FALLBACK_PRICING exact match or prefix match (`canonical.startsWith(key + "-")`)
3. LiteLLM fuzzy: `canonical.startsWith(key)` or `key.startsWith(canonical)`
4. FALLBACK_PRICING prefix: `canonical.startsWith(key)`

- Canonical name is computed by stripping `@.*` suffix and `-YYYYMMDD` date suffix.

**R20** - The LiteLLM pricing JSON must be fetched from `https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json` and cached at `~/.cache/codeburn/litellm-pricing.json` for 24 hours. On fetch failure, fall back to disk cache; if no disk cache exists, fall back to the hardcoded FALLBACK_PRICING table. No user-visible error in any of these fallback paths.

**R21** - The FALLBACK_PRICING table must contain all 18 model entries from `models.ts` with the exact same per-token costs (in USD). Any new model added to the TypeScript source before Go ships must be propagated.

**R22** - The exchange rate cache must be stored at `~/.cache/codeburn/exchange-rate.json` with schema `{ "timestamp": <unix-ms>, "code": "<ISO4217>", "rate": <float> }` and a 24-hour TTL. On Frankfurter API fetch failure, rate must default to 1 (identity, equivalent to USD). No user-visible error.

**R23** - The classifier must implement the same 13-category logic using the same regex patterns and tool sets (EDIT_TOOLS, READ_TOOLS, BASH_TOOLS, TASK_TOOLS, SEARCH_TOOLS). Regexes must be precompiled at init time. The Go implementation must produce identical category assignments for the same input as the TypeScript implementation.

**R24** - The `shortProject` display transformation (stripping home directory prefix, `/private/tmp/<org>/<env>/`, `/private/tmp/`, `/tmp/` from encoded project names) must be preserved in the TUI display.

**R25** - The `in-process session cache` (60-second TTL, 10-entry LRU, keyed by dateRange+provider+extractBash (extractBash is a boolean ParseOptions flag that controls whether bash command text is extracted from tool_use blocks during parsing; when false, bash extraction is skipped for performance)) must be preserved. The dashboard's period-switch path re-parses data; the in-process cache prevents redundant full parses within the TTY session.

## 2.3 TUI Dashboard

**R26** - The TUI must implement all 8 panels: overview, daily, project, model, activity, tools, mcp, bash. Panel colors must match (PANEL_COLORS hex values). The mcp panel and bash panel must be hidden when their data is empty.

**R27** - Responsive layout must be preserved: 2-column layout at terminal width >= 90 columns, capped at 160 columns total. `barWidth` must be clamped between 6 and 10: `max(6, min(10, inner - 30))` where `inner = halfWidth - 4`.

**R28** - The gradient bar (`HBar`) must implement the three-segment blue-amber-orange gradient using the same RGB interpolation: `[91,158,245] -> [245,200,91] -> [255,140,66] -> [245,91,91]`. Empty bars (max=0) must render `░` characters in dim color. Filled cells use `█`. Unfilled cells use `░` in `#333333`.

**R29** - All keyboard shortcuts must be implemented:

- `q`: quit
- `p`: cycle provider (only when multiple providers detected)
- Left arrow: previous period (wraps)
- Right arrow / Tab: next period (wraps)
- `1`/`2`/`3`/`4`: direct period selection (today/week/30days/month)

**R30** - Period switching must use a 600ms debounce for arrow/Tab keys. Direct number key selection (1-4) must bypass debounce and reload immediately.

**R31** - Auto-refresh must fire every `--refresh <seconds>` milliseconds (if specified). On each tick, `ParseAllSessions` is called with the current period and provider, replacing the displayed data.

**R32** - The TUI must detect multiple providers by checking `discoverSessions()` for each known provider at startup. The provider cycle (`p` key) must include `all` plus each detected provider name, in the order: `all`, then detected providers in registration order (claude, codex, cursor).

**R33** - When the Cursor provider is active (single-provider mode), the tools panel must show "Languages" (from `lang:` prefixed tools) instead of the standard tools/bash/mcp panels.

## 2.4 Performance Targets

**R34** - Cold parse of 5 benchmark fixture JSONL sessions (from `tests/fixtures/bench/`): complete in under 50ms wall time on a 2020+ Mac.

**R35** - Warm cache hit (all 5 sessions cached in SQLite): complete in under 5ms wall time.

**R36** - `codeburn status --format json` end-to-end (including binary startup): complete in under 100ms on warm cache. This is the menubar use case.

**R37** - Memory RSS for `codeburn status --format json`: under 30MB. The TypeScript implementation peaks at 150-185MB due to V8 + React.

## 2.5 Error Handling

**R38** - Invalid JSONL lines during Claude session parsing must be silently skipped. Parsing of the containing file must continue.

**R39** - If a Claude session directory is unreadable (permission denied, missing), it must be silently skipped. Remaining directories must continue parsing.

**R40** - If the Cursor `state.vscdb` does not exist or cannot be opened, the Cursor provider must return zero sessions silently. No panic, no user-visible error.

**R41** - If the Cursor schema is not recognized (missing `cursorDiskKV` table or `bubbleId:` keys), the binary must write `codeburn: Cursor storage format not recognized. You may need to update CodeBurn.` to stderr and skip Cursor data.

**R42** - If `better-sqlite3` was previously missing (TS behavior was silent Cursor skip), the Go implementation has no such condition: `modernc.org/sqlite` is a compile-time dependency. Cursor is always attempted if the database file exists.

**R43** - Session cache write failures (SQLite busy, full disk) must be silently swallowed. Parsing must succeed and return results even when caching fails.

**R44** - The `codeburn status --format json` command must always produce valid JSON to stdout even if all provider data is empty (all counters zero).

**R45** - If both SwiftBar and xbar plugin directories exist, SwiftBar takes priority (match TypeScript behavior: `existsSync(swiftBarDir)` checked first).

**R46** - When `CODEBURN_DEBUG=1` is set, the binary must write diagnostic output to stderr: provider discovery counts, session cache hit/miss counts, pricing lookup source (LiteLLM/fallback/disk), and any silently skipped files or errors. Debug output must never go to stdout (would corrupt JSON/menubar/CSV output).

**R47** - The `export` command must accept `-o, --output <path>` flag. When provided, output is written to the file. When omitted, output goes to stdout. Write failures (permission denied, invalid path) must print an error to stderr and exit with code 1.

**R48** - The `status` and `export` commands must accept `--provider <provider>` flag with values `all`, `claude`, `codex`, `cursor` (default: `all`). This filters which providers contribute data to the output.

**R49** - Exit codes: `0` for success, `1` for user errors (invalid currency code, invalid arguments, file write failure). All other errors (missing session data, network failures, corrupt cache) must not cause a non-zero exit.

**R50** - If `~/.config/codeburn/config.json` exists but contains invalid JSON, the `currency` command must treat it as if no config exists (use defaults). Other commands reading config must do the same.

**R51** - R37 (RSS under 30MB) must be re-verified after Phase 3 TUI dependencies (bubbletea, lipgloss) are added. The Phase 3 gate must include an RSS measurement of `codeburn status --format json` to ensure TUI imports do not regress the non-TUI path.

**R52** - The Cursor provider must apply a 35-day lookback window. Only `bubbleId:` entries with `createdAt` within the last 35 days are included in results. The time floor is computed as `now - 35 * 24 * 60 * 60 * 1000` and converted to ISO 8601 for the SQL `WHERE` clause. This matches the TypeScript implementation at `cursor.ts:138`.

**R53** - The `CalculateCost` function must support a `speed` parameter (`standard` or `fast`). When `speed` is `fast`, the total cost is multiplied by the model's `fastMultiplier` from the pricing table. Standard speed uses a multiplier of 1. The FALLBACK_PRICING table must include the `fastMultiplier` field for every model entry (e.g., `claude-opus-4-6` has `fastMultiplier: 6`; all others have `fastMultiplier: 1`).
