# Design Decisions: CodeBurn Go Migration

### D1 - CGO-Free SQLite: modernc.org/sqlite

**Decision:** Use `modernc.org/sqlite` (pure-Go transpilation of SQLite, includes JSON1) instead of any CGO binding.
**Rationale:** `better-sqlite3` is the source of significant distribution pain in the TypeScript version - it requires a build toolchain, breaks on ARM/x86 cross-distribution, and was the root cause of session cache unavailability when the native addon was absent. `modernc.org/sqlite` ships as a Go package with no CGO requirement, producing a static binary with zero native dependencies. JSON1 extension is included, required for Cursor's `json_extract` queries.
**Alternatives considered:**

- `mattn/go-sqlite3` (CGO): eliminates the binary distribution advantage; rejected.
- `zombiezen/go-sqlite3` (also CGO-free): less adoption, less JSON1 testing; rejected in favor of modernc.
**D-RISK:** `modernc.org/sqlite` has slightly different performance characteristics from native SQLite. Acceptable because the session cache is a write-once/read-many workload with small result sets.

### D2 - Concurrent Session Parsing with Bounded Worker Pool

**Decision:** Parse session files concurrently using a bounded goroutine pool sized `runtime.NumCPU() * 2`. Use `sync.Map` for the cross-goroutine deduplication set.
**Rationale:** The dominant bottleneck in the TypeScript implementation is sequential JSONL parsing. A user with 200 session files sees all 200 parsed one-at-a-time. Goroutines with a bounded pool give parallelism without uncontrolled resource use (open file descriptor exhaustion, excessive memory). `sync.Map` is appropriate for this workload: hundreds of keys, high read ratio after initial population, no contention hotspots.
**Alternatives considered:**

- Unbounded goroutines: risks hitting OS file descriptor limits on large session histories; rejected.
- `sync.Mutex`-wrapped `map[string]struct{}`: simpler but introduces a single contention point; `sync.Map` is better for concurrent reads; rejected in favor of `sync.Map`.
- Channel-based fan-out/fan-in: more code, same semantics; rejected for simplicity.
**Edge case:** Codex parser is stateful (cumulative token deltas are ordering-dependent). This is safe because each goroutine handles exactly one file; ordering dependency is within a file, not across files.

### D3 - Go 1.23 iter.Seq2 for Provider Streaming

**Decision:** Provider `ParseSession` returns `iter.Seq2[ParsedCall, error]` (push iterator from Go 1.23 range-over-func). This replaces the TypeScript `AsyncGenerator<ParsedProviderCall>`.
**Rationale:** The iterator model preserves the streaming/lazy semantics of the TypeScript async generators without requiring goroutines inside providers. The caller controls iteration. This is a direct semantic match. Go 1.23 is the minimum version required.
**Alternatives considered:**

- Channel-based streaming: requires goroutine management inside each provider, complicates error propagation; rejected.
- Materializing all results into a slice: breaks the streaming invariant, increases peak memory; rejected for the Cursor provider which may have thousands of entries.

### D4 - Currency Symbol Validation via Hardcoded Table

**Decision:** Implement `isValidCurrencyCode` using a hardcoded set of valid ISO 4217 alphabetic codes instead of relying on a platform-provided locale/currency library.
**Rationale:** `Intl.NumberFormat` (JavaScript) throws on invalid codes, serving as a validator. Go's `golang.org/x/text/currency` package provides programmatic validation but has limited symbol resolution. The TypeScript implementation uses `Intl.NumberFormat` both to validate and to resolve the currency symbol. In Go, the symbol table can be hardcoded for the 17 currencies in the menubar submenu, plus a broader validation set for the `currency` command. This is simpler and eliminates a non-trivial dependency.
**Hardcoded symbol table** must contain at minimum: USD ($), GBP (£), EUR (€), AUD (A$), CAD (CA$), NZD (NZ$), JPY (¥), CHF (CHF), INR (₹), BRL (R$), SEK (kr), SGD (S$), HKD (HK$), KRW (₩), MXN (MX$), ZAR (R), DKK (kr). Fraction digits must also be hardcoded per currency (JPY and KRW: 0 digits; all others: 2 digits).
**Alternatives considered:**

- `golang.org/x/text/currency`: does not expose fraction digits or symbols in a straightforward API; rejected.
- `shopspring/decimal` + a currency library: over-engineered for this use case; rejected.

### D5 - Unified Provider Interface (No Claude Special-Case)

**Decision:** Claude session parsing is migrated into the same `Provider` interface as Codex and Cursor. The legacy `scanProjectDirs` path in the TypeScript `parser.ts` is eliminated.
**Rationale:** The TypeScript implementation has an architectural inconsistency: Claude uses a special-cased `scanProjectDirs` function in `parser.ts` that handles JSONL parsing internally, while Codex and Cursor use `Provider.createSessionParser()`. This bifurcation means Claude cannot benefit from the concurrent parser pool without special treatment. Unifying under one interface enables full parallelism for all providers and removes 150+ lines of duplicated parsing logic.
**Impact:** Claude's `createSessionParser` must implement JSONL parsing (currently a no-op stub returning an empty generator). The file discovery remains in `discoverSessions`.
**Edge case:** Subagent JSONL files must be surfaced as separate `SessionSource` entries from `discoverSessions`, not handled specially inside the parser.

### D6 - Incremental Shipping: Go Handles All Non-TUI Commands First

**Decision:** Ship the Go binary for all non-TUI commands (Phases 0-2) while retaining the TypeScript bundle for `report`, `today`, `month`. Route via a thin shell wrapper that inspects the subcommand.
**Rationale:** Phases 0-2 deliver the highest-value performance win (menubar latency) with the lowest risk. The TUI port (Phase 3) is the highest-risk component. Decoupling delivery allows the menubar, status, export, and currency commands to benefit from Go's startup time immediately without waiting for TUI parity.
**Routing mechanism:** A shell script `codeburn` wrapper installed alongside both binaries checks the first argument. `report`, `today`, `month` invoke `codeburn-ts`; all others invoke `codeburn-go`. This is removed when Phase 3 ships.
**Alternatives considered:**

- Wait until Phase 3 is complete before shipping any Go code: delays the primary benefit (menubar startup) by 4-6 weeks; rejected.
- Compile-time flag to disable TUI: makes the binary incomplete and confusing; rejected.

### D7 - Session Cache Caches Full Sessions (No Date Filter)

**Decision:** The session cache stores the full `SessionSummary` for each file (all turns, unfiltered by date range). Date filtering is applied after cache retrieval.
**Rationale:** This is the existing TypeScript behavior (from `parseSessionFile`: "cache full session no date filter so cached result serves all ranges"). If the cache stored date-filtered summaries, a summary cached for a `week` query would be invalid for a `today` query. Storing the full session means one cache entry serves all date range queries for that file.
**Edge case:** The in-process 60s/10-entry cache is keyed by `(dateRange, provider, extractBash)`. The SQLite disk cache is keyed by `(filePath, mtime_ms, file_size)`. These are complementary: the disk cache avoids JSONL re-parsing; the in-process cache avoids SQLite re-reads during dashboard period switching.

### D8 - Privacy Invariant as Explicit Zero Pass

**Decision:** After `classifyTurn()` returns, the implementation must explicitly zero `ClassifiedTurn.UserMessage` before the turn is stored in the session cache or any aggregate. This must be a named, commented step.
**Rationale:** This is a documented invariant in the TypeScript source (R7, R8, R9 comments in `parser.ts`). The classification function is the only consumer of `userMessage`. If it is stored beyond classification - in the SQLite cache or in the in-process cache - user prompts persist on disk and in memory far longer than necessary. Making this an explicit zero pass (not relying on zeroing-on-write) makes the privacy guarantee auditable.
**Failure mode:** If classification is refactored to be async or if new code paths are added that skip the zero pass, the invariant breaks silently. The Go implementation must include a test that asserts `userMessage` is empty in all `ClassifiedTurn` entries after `parseSessionFile` returns.

### D9 - Desktop Claude Sessions: Walk to Depth 8

**Decision:** The `findDesktopProjectDirs` walk is reproduced with the same depth limit (8), same exclusion list (`node_modules`, `.git`), and same termination condition (stop descending when a `projects/` directory is found).
**Rationale:** The TypeScript implementation uses this specific walk to locate Claude Desktop session directories without knowing their exact location. Depth 8 is an arbitrary but sufficient bound for typical macOS Application Support directory structures. Deviating from this bound risks missing sessions (too shallow) or excessive I/O (too deep).

### D10 - Bubbletea for TUI (Phase 3)

**Decision:** Use `github.com/charmbracelet/bubbletea` + `github.com/charmbracelet/lipgloss` for the TUI. No other TUI frameworks are evaluated.
**Rationale:** Bubbletea's Elm Architecture maps directly to the Ink/React conceptual model: Model (state) + Update (keyboard/timer messages) + View (render to string). Lipgloss provides the terminal styling primitives (width, color, border) needed to replicate the Ink `Box`/`Text` layout. The Charm ecosystem is the de-facto Go TUI standard.
**Mapping notes documented in go-migration.md section 5.** The gradient `HBar` rendering must be a standalone function tested in isolation before the full TUI is assembled.
**High-risk note:** The TUI is the most labor-intensive component (668 lines of React). Visual regression testing must be done manually before each release candidate.
