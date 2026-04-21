# Acceptance Criteria: CodeBurn Go Migration

### AC1 - CLI Subcommand Parity

**Requirement:** R1
**Given** the Go binary is built and installed,
**When** the user runs `codeburn --help`,
**Then** all 8 subcommands are listed: `report`, `status`, `today`, `month`, `export`, `install-menubar`, `uninstall-menubar`, `currency`.

### AC2 - Status JSON Format Compatibility

**Requirement:** R2
**Given** fixture session data with known costs,
**When** both `npx tsx src/cli.ts status --format json` and `codeburn status --format json` are run against the same data,
**Then** the JSON outputs are semantically equivalent: same keys, same values after parsing. Cost values must match when rounded to 2 decimal places. Field order within objects is not required to match.

### AC3 - CSV Export Format

**Requirement:** R3
**Given** fixture session data containing entries with cost, daily, activity, model, tool, bash, and project breakdowns,
**When** `codeburn export --format csv` is run,
**Then** the output file contains section headers in this order: `# Summary`, `# Daily - Today`, `# Activity - Today`, `# Daily - 7 Days`, `# Activity - 7 Days`, `# Models - 7 Days`, `# Daily - 30 Days`, `# Activity - 30 Days`, `# Models - 30 Days`, `# Tools - All`, `# Shell Commands - All`, `# Projects - All`.

### AC4 - CSV Formula Injection Protection

**Requirement:** R3
**Given** a project path or tool name beginning with `=`,
**When** `codeburn export --format csv` is run,
**Then** the cell value is prefixed with a tab character in the CSV output.

### AC5 - Session Cache Hit

**Requirement:** R16, R35
**Given** a JSONL session file has been parsed once and its result is stored in the SQLite cache,
**When** the same file is parsed again without modification,
**Then** the SQLite query returns the cached result, `ParseSession` does not re-read the file, and the parse completes in under 5ms.

### AC6 - Cache Invalidation on File Change

**Requirement:** R16
**Given** a cached session file,
**When** the file is modified (mtime_ms changes),
**Then** `getCachedSummary` returns nil/null (cache miss), and the file is re-parsed and re-cached.

### AC7 - Privacy Invariant

**Requirement:** R15
**Given** any session JSONL file containing user messages,
**When** parsing completes and the session summary is written to the SQLite cache,
**Then** `summary_json` in the database contains no non-empty `userMessage` values in any `ClassifiedTurn`.

### AC8 - Deduplication Global Scope

**Requirement:** R13
**Given** the same Claude `message.id` appears in two different JSONL files (session file + subagent file),
**When** `ParseAllSessions` processes both files,
**Then** the API call is counted exactly once in the output.

### AC9 - Codex Token Normalization

**Requirement:** R14
**Given** a Codex JSONL entry with `input_tokens: 1000` and `cached_input_tokens: 200`,
**When** the entry is parsed,
**Then** the resulting `ParsedCall.InputTokens` is 800 (not 1000) and `CacheReadInputTokens` is 200.

### AC10 - Cursor 35-day Lookback

**Requirement:** R52
**Given** a `state.vscdb` containing `bubbleId:` entries older than 35 days,
**When** the Cursor provider parses the database,
**Then** entries with `createdAt` more than 35 days before the current time are excluded from results.

### AC11 - Pricing Fallback Chain

**Requirement:** R19
**Given** a model name `claude-sonnet-4-5-20250613` (with a date suffix) is NOT in the LiteLLM cache,
**When** cost is calculated,
**Then** the canonical name `claude-sonnet-4-5` is derived, and the FALLBACK_PRICING entry for `claude-sonnet-4-5` is used.

### AC12 - Corrupt Cache Recovery

**Requirement:** R17
**Given** `~/.cache/codeburn/session-cache.db` contains invalid SQLite data (corrupt file),
**When** the Go binary starts and opens the cache,
**Then** the corrupt file is deleted, a new empty database is created, and parsing proceeds normally.

### AC13 - Cursor Provider Silent Skip

**Requirement:** R40
**Given** the `state.vscdb` file does not exist at the expected platform path,
**When** `discoverSessions` is called for the Cursor provider,
**Then** the function returns zero session sources, no error is written to stderr, and the binary continues with Claude and Codex data only.

### AC14 - Menubar Performance

**Requirement:** R36
**Given** the session cache is warm (all current session files already cached),
**When** `codeburn status --format json` is executed (including process startup),
**Then** the process exits with output in under 100ms wall time.

### AC15 - TUI Gradient Rendering

**Requirement:** R28
**Given** a horizontal bar of width 8 at 100% fill (value == max),
**When** the bar is rendered,
**Then** all 8 characters are `█` with colors progressing from `#5B9EF5` (first) through the amber midpoint to `#F55B5B` (tolerance: +/-2 per RGB channel) (last), matching the three-segment gradient function.

### AC16 - Subagent Session Discovery

**Requirement:** R10
**Given** a Claude session directory contains a subdirectory `<uuid>/subagents/session.jsonl`,
**When** the Claude provider discovers sessions,
**Then** `session.jsonl` is included in the session sources.

### AC17 - Non-TTY Static Render

**Requirement:** R9
**Given** stdout is not a TTY (pipe or redirect),
**When** `codeburn report` is run,
**Then** the dashboard renders a single static frame and the process exits (no blocking on keyboard input).

### AC18 - Classifier Category Parity

**Requirement:** R23
**Given** the 5 benchmark fixture JSONL sessions in `tests/fixtures/bench/`,
**When** both the TypeScript and Go classifiers process them,
**Then** every `ClassifiedTurn` receives the identical `category` value.

### AC19 - Currency Command Validation

**Requirement:** R8
**Given** the user runs `codeburn currency NOTACODE`,
**When** the command executes,
**Then** it prints `"NOTACODE" is not a valid ISO 4217 currency code.` to stderr and exits with code 1.

### AC20 - Empty Data Output

**Requirement:** R44
**Given** no session files exist (fresh install, no data),
**When** `codeburn status --format json` is run,
**Then** the output is valid JSON with `today.cost: 0`, `today.calls: 0`, `month.cost: 0`, `month.calls: 0`.

### AC21 - JSON Export Format

**Requirement:** R4
**Given** fixture session data with known costs,
**When** `codeburn export --format json` is run,
**Then** the output has `generated` (ISO8601), `periods` map by label, `tools`, `shellCommands`, `projects` with schemas matching the TypeScript implementation.

### AC22 - Terminal Status Format

**Requirement:** R5
**Given** fixture session data with known costs,
**When** `codeburn status --format terminal` is run,
**Then** the output contains bold "Today" and "Month" labels, yellow cost values, dim call counts, separated by newlines.

### AC23 - Menubar Status Format

**Requirement:** R6
**Given** fixture session data with known costs,
**When** `codeburn status --format menubar` is run,
**Then** the title line has format `<cost> | sfimage=flame.fill color=#FF8C42`, sections use `--` prefix, `font=Menlo`, and the currency submenu has 17 entries in order: USD, GBP, EUR, AUD, CAD, NZD, JPY, CHF, INR, BRL, SEK, SGD, HKD, KRW, MXN, ZAR, DKK.

### AC24 - Install Menubar on macOS

**Requirement:** R7
**Given** the platform is macOS,
**When** `codeburn install-menubar` is run,
**Then** a script is written to `codeburn.5m.sh` in the detected plugin directory with chmod 0755 and a confirmation message is printed.

### AC25 - Install Menubar on Non-macOS

**Requirement:** R7
**Given** the platform is not macOS,
**When** `codeburn install-menubar` is run,
**Then** it prints "only available on macOS".

### AC26 - Codex Session Validation

**Requirement:** R11
**Given** Codex JSONL files at `$CODEX_HOME/sessions/YYYY/MM/DD/`,
**When** `discoverSessions` runs,
**Then** only files whose first line has `session_meta` with `payload.originator` starting with `codex` are included.

### AC27 - Cursor Cache Invalidation

**Requirement:** R18
**Given** Cursor results are cached with fingerprint `<mtimeMs>:<size>`,
**When** `state.vscdb` mtime or size changes,
**Then** the entire cache is invalidated and re-parsed.

### AC28 - LiteLLM Fetch Fallback

**Requirement:** R20
**Given** the network is unreachable,
**When** pricing is needed,
**Then** the disk cache at `~/.cache/codeburn/litellm-pricing.json` is used. Given no disk cache either, then FALLBACK_PRICING is used. No user-visible error in any case.

### AC29 - FALLBACK_PRICING Accuracy

**Requirement:** R21
**Given** a model present in the hardcoded FALLBACK_PRICING table,
**When** cost is calculated,
**Then** the per-token costs match the 18 entries from `models.ts` exactly.

### AC30 - Exchange Rate Fallback

**Requirement:** R22
**Given** the Frankfurter API is unreachable,
**When** an exchange rate is needed,
**Then** the rate defaults to 1 (identity). No user-visible error.

### AC31 - Short Project Display

**Requirement:** R24
**Given** a project path `/Users/alice/dev/myproject`,
**When** displayed in the TUI,
**Then** the home directory prefix is stripped to show `dev/myproject`.

### AC32 - In-Process Cache Hit

**Requirement:** R25
**Given** `parseAllSessions` is called twice within 60s with the same dateRange+provider+extractBash,
**When** the second call executes,
**Then** the cached result is returned without SQLite or JSONL re-read.

### AC33 - Keyboard Shortcuts

**Requirement:** R29
**Given** the TUI is running,
**When** the user presses `q`,
**Then** the TUI exits.
**When** the user presses left/right arrows,
**Then** the period cycles.
**When** the user presses 1-4,
**Then** the period jumps directly.

### AC34 - Period Debounce

**Requirement:** R30
**Given** the user presses right arrow twice within 600ms,
**When** the debounce fires,
**Then** only the final period is loaded (not both intermediate periods).

### AC35 - Auto-Refresh

**Requirement:** R31
**Given** `--refresh 30` is specified,
**When** 30 seconds elapse,
**Then** `ParseAllSessions` is called and the display updates.

### AC36 - Provider Detection

**Requirement:** R32
**Given** Claude and Codex have session data but Cursor does not,
**When** the TUI starts,
**Then** the provider cycle includes `all`, `claude`, `codex` (not `cursor`).

### AC37 - Cursor Languages Panel

**Requirement:** R33
**Given** Cursor is the active provider,
**When** the tools panel renders,
**Then** it shows "Languages" from `lang:` prefixed tools instead of standard tools/bash/mcp panels.

### AC38 - Invalid JSONL Resilience

**Requirement:** R38
**Given** a JSONL file containing 10 lines where line 5 is invalid JSON,
**When** the file is parsed,
**Then** lines 1-4 and 6-10 are processed normally.

### AC39 - Unreadable Directory Resilience

**Requirement:** R39
**Given** one Claude project directory returns EACCES,
**When** `discoverSessions` runs,
**Then** other directories are still discovered and parsed.

### AC40 - Cursor Always Attempted

**Requirement:** R42
**Given** the Go binary is built,
**When** a Cursor `state.vscdb` exists,
**Then** the Cursor provider attempts parsing (no build-tag gate).

### AC41 - Cache Write Failure Resilience

**Requirement:** R43
**Given** the SQLite disk is full,
**When** a session is parsed,
**Then** the parse result is returned normally, only caching is skipped.

### AC42 - SwiftBar Priority

**Requirement:** R45
**Given** both `~/Library/Application Support/SwiftBar/Plugins/` and `~/Library/Application Support/xbar/plugins/` exist,
**When** `codeburn install-menubar` runs,
**Then** the script is written to the SwiftBar directory.

### AC43 - TUI Panel Presence and Colors

**Requirement:** R26
**Given** session data containing tool usage, bash commands, and MCP calls,
**When** the TUI dashboard renders,
**Then** all 8 panels are present (overview, daily, project, model, activity, tools, mcp, bash) with hex colors matching PANEL_COLORS from the TypeScript source. The mcp and bash panels are hidden when their data is empty.

### AC44 - Responsive Layout Breakpoint

**Requirement:** R27
**Given** a terminal width of 89 columns,
**When** the TUI renders,
**Then** a single-column layout is used.
**Given** a terminal width of 90 columns,
**When** the TUI renders,
**Then** a two-column layout is used, capped at 160 columns total. `barWidth` is clamped between 6 and 10 using `max(6, min(10, inner - 30))` where `inner = halfWidth - 4`.

### AC45 - Cold Parse Benchmark

**Requirement:** R34
**Given** the 5 benchmark fixture JSONL sessions in `tests/fixtures/bench/` and no session cache,
**When** `ParseAllSessions` is benchmarked via `go test -bench`,
**Then** parsing completes in under 50ms wall time on a 2020+ Mac.

### AC46 - Memory RSS Budget

**Requirement:** R37
**Given** the Go binary with all dependencies compiled in,
**When** `codeburn status --format json` is run against real session data,
**Then** peak RSS is under 30MB.

### AC47 - Cursor Schema Not Recognized Warning

**Requirement:** R41
**Given** a `state.vscdb` that exists but lacks the `cursorDiskKV` table or has no `bubbleId:` keys,
**When** the Cursor provider parses it,
**Then** `codeburn: Cursor storage format not recognized. You may need to update CodeBurn.` is written to stderr, Cursor data is skipped, and the process continues with other providers.

### AC48 - Debug Output

**Requirement:** R46
**Given** `CODEBURN_DEBUG=1` is set,
**When** any command runs,
**Then** stderr receives diagnostic output including: provider discovery counts, session cache hit/miss counts, pricing lookup source (LiteLLM/fallback/disk), and any silently skipped files or errors. Stdout is never polluted with debug output.

### AC49 - Export Output Flag

**Requirement:** R47
**Given** `codeburn export --format csv -o /tmp/out.csv`,
**When** the command completes,
**Then** the CSV is written to `/tmp/out.csv` (not stdout). On write failure (permission denied), the process prints an error to stderr and exits with code 1.

### AC50 - Provider Filter Flag

**Requirement:** R48
**Given** `codeburn status --format json --provider claude`,
**When** the command runs,
**Then** only Claude provider data contributes to the output. Codex and Cursor data are excluded.

### AC51 - Exit Code Contract

**Requirement:** R49
**Given** various error conditions,
**When** the binary runs,
**Then** exit code 0 is returned for success, exit code 1 for user errors (invalid currency code, invalid arguments, file write failure). Missing session data, network failures, and corrupt cache do not cause non-zero exit.

### AC52 - Invalid Config Resilience

**Requirement:** R50
**Given** `~/.config/codeburn/config.json` contains `{invalid json`,
**When** any command reads the config,
**Then** it behaves as if no config exists (uses defaults). No crash, no user-visible error.

### AC53 - Phase 3 RSS Re-check

**Requirement:** R51
**Given** the Go binary after Phase 3 TUI dependencies (bubbletea, lipgloss) are added,
**When** `codeburn status --format json` is run (non-TUI path),
**Then** peak RSS remains under 30MB. TUI imports must not regress the non-TUI memory budget.

### AC54 - Cursor 35-Day Lookback

**Requirement:** R52
**Given** a `state.vscdb` containing `bubbleId:` entries with `createdAt` timestamps older than 35 days,
**When** the Cursor provider parses the database,
**Then** entries with `createdAt` more than 35 days before the current time are excluded from results.

### AC55 - Fast-Mode Cost Multiplier

**Requirement:** R53
**Given** a model with `fastMultiplier: 6` (e.g., `claude-opus-4-6`) and `speed: 'fast'`,
**When** `CalculateCost` is invoked,
**Then** the returned cost is 6x the standard-speed cost for the same token counts.
