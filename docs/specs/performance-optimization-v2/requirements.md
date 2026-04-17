# Requirements: Performance Optimization v2

## R1 -- SQLite session cache module

Create `src/session-cache.ts` exporting three functions:
- `openCache(): Database | null` -- opens or creates
  `~/.cache/codeburn/session-cache.db`; creates the directory with `mkdirSync` recursive;
  sets `PRAGMA journal_mode = WAL` and `PRAGMA busy_timeout = 3000`;
  returns `null` on any failure (dynamic import failure, permission error, corrupt db)
- `getCachedSummary(db: Database, filePath: string, mtimeMs: number, fileSize: number): SessionSummary | null`
  -- returns a cached `SessionSummary` if `file_path + mtime_ms + file_size` match; otherwise `null`.
  If `JSON.parse(summary_json)` throws, deletes the row and returns `null`.
- `putCachedSummary(db: Database, filePath: string, mtimeMs: number, fileSize: number, summary: SessionSummary): void`
  -- upserts a row (`INSERT OR REPLACE`); serialises `summary` as JSON; records `cached_at`.
  Write failures are silently swallowed.

The module must import `better-sqlite3` dynamically (`await import('better-sqlite3')`).
If the import fails (native addon absent), `openCache` returns `null`.

## R2 -- SQLite schema

The cache database must have a single table `session_summaries` with columns:
`file_path TEXT PRIMARY KEY, mtime_ms INTEGER NOT NULL, file_size INTEGER NOT NULL,
summary_json TEXT NOT NULL, cached_at INTEGER NOT NULL`.
The schema must be created with `CREATE TABLE IF NOT EXISTS` on every `openCache` call.
`mtime_ms` must be stored as `Math.floor(stat.mtimeMs)` to avoid float/integer comparison
mismatches across platforms.

## R3 -- Invalidation by mtime and size

Cache lookup must compare `mtime_ms` AND `file_size` of the session file on disk against
the stored row. If either differs, return `null` (treat as cache miss). This matches the
strategy in `src/cursor-cache.ts` (D-CACHE-2).

## R4 -- Integration in parseSessionFile

`parseSessionFile` in `src/parser.ts` must accept an optional `db: Database | null`
parameter. When `db` is non-null: stat the file, call `getCachedSummary`; on hit, return
the deserialized summary; on miss, parse normally then call `putCachedSummary` before
returning. When `db` is null, behaviour is unchanged from Phase 1.

## R5 -- Cache opened once per parseAllSessions invocation

`parseAllSessions` must call `openCache` once and pass the handle down to all
`parseSessionFile` calls within that invocation. `openCache` must not be called more than
once per CLI command execution.

## R6 -- userMessage zeroed before cache write

After `classifyTurn` runs, `classified.userMessage` must be set to `''` before the turn
is passed to `buildSessionSummary`. This ensures the cached `SessionSummary` never contains
user message content. Applies to both the Claude JSONL path and the provider path.

## R7 -- userMessage zeroing: Claude path

In `parseSessionFile`, after the `turns.map(classifyTurn)` call produces a
`ClassifiedTurn[]` array, iterate the array and set each element's `userMessage` to `''`.
`classifyTurn` receives the full `userMessage` (reads it for keyword matching).
The zeroing mutates the returned `ClassifiedTurn`, not the source `ParsedTurn`.

## R8 -- userMessage zeroing: provider path

In `parseProviderSources`, after `classifyTurn(turn)` returns a `ClassifiedTurn`, set
`classified.userMessage = ''` before pushing to `sessionMap`.

## R9 -- Ordering invariant: zero after classify, before cache write

`userMessage` must be zeroed after `classifyTurn` (which reads `userMessage` for keyword
matching) and before `putCachedSummary` (which serialises the `SessionSummary`). This
ordering is an invariant; test coverage must verify both that classification used the
original message AND that the stored result has an empty message.

## R10 -- DEBUG log for cache hit/miss

When `process.env.DEBUG` is truthy (any non-empty string), `parseSessionFile` must emit
one line to `process.stderr` per file: `[session-cache] HIT <file_path>` or
`[session-cache] MISS <file_path>`. No output when `DEBUG` is unset or empty.

## R11 -- Schema reset on error

If `openCache` encounters a corrupt database (any error from `CREATE TABLE IF NOT EXISTS`,
`PRAGMA integrity_check` not returning `ok`, or any unhandled SQLite exception), it must
delete the database file and recreate it. If recreation also fails, return `null`. The
cache module must never throw; errors are logged to `process.stderr` when `DEBUG` is set.

## R12 -- Performance targets

`codeburn status --format json` (built bundle) must complete in under 400ms median
(warm cache, median of 3 runs) on the benchmark dataset. Peak RSS during this command
must be under 100MB (D-PERF-1, D-RSS-1).

## R13 -- Cold and warm benchmark thresholds

Using the fixture dataset under `tests/fixtures/bench/` (>= 500 JSONL lines across >= 5
files):
- **Cold** (empty cache): `parseAllSessions` must complete under 200ms median
- **Warm** (cache populated): `parseAllSessions` must complete under 20ms median
- Warm must be at least 3x faster than cold

These are measured via vitest bench, not wall-clock CLI timing.

## R14 -- No regression

`npx vitest run` must pass with zero failures. Output of `status --format json` must be
byte-for-byte identical to Phase 1 baseline for the same dataset.

## R15 -- Automated benchmark suite

Create `tests/bench/parse-performance.bench.ts` using vitest bench mode. Create synthetic
fixture files under `tests/fixtures/bench/` (>= 5 JSONL files, >= 500 lines total,
committed to the repo). Each fixture must contain entries with `type`, `timestamp`,
`message.model`, `message.usage`, and `message.content` fields sufficient to exercise the
full parse path including classification. The benchmark must use an isolated cache
(temporary directory, not `~/.cache/codeburn/`). Must be excluded from `npx vitest run`
(only runs via `npx vitest bench`).
