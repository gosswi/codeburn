# Acceptance Criteria: Performance Optimization v2

## Cache module (R1, R2)

**AC1** (R1) -- Given `~/.cache/codeburn/` does not exist; when `openCache` is called;
then the directory is created with `mkdirSync({ recursive: true })` and a new
`session-cache.db` is returned without throwing.

**AC2** (R1) -- Given `better-sqlite3` dynamic import fails (native addon absent); when
`openCache` is called; then it returns `null` and `parseAllSessions` completes normally
at Phase 1 performance without throwing.

**AC3** (R2) -- Given an open cache db; when inspected with sqlite3 CLI; then the table
`session_summaries` exists with columns `file_path, mtime_ms, file_size, summary_json,
cached_at` and `file_path` is the PRIMARY KEY.

## Cache hit/miss logic (R3, R4, R5)

**AC4** (R3) -- Given a cached row with `mtime_ms=T` and `file_size=S`; when the file on
disk has `mtime_ms=T` and `file_size=S`; then `getCachedSummary` returns the deserialised
`SessionSummary` and `parseSessionFile` does not open the JSONL file.

**AC5** (R3) -- Given a cached row; when the file's `mtime_ms` differs by any amount (even
1ms); then `getCachedSummary` returns `null` and the file is re-parsed and the cache row
is overwritten.

**AC6** (R3) -- Given a cached row; when the file's `file_size` differs but `mtime_ms` is
the same; then `getCachedSummary` returns `null` and the file is re-parsed.

**AC7** (R4) -- Given `db` is `null`; when `parseSessionFile` is called; then it runs the
full Phase 1 JSONL parsing path with no stat call and no cache interaction.

**AC8** (R5) -- Given a CLI command that calls `parseAllSessions`; when the command
completes; then `openCache` was called exactly once during that execution (verifiable via
spy or call count in unit test).

## userMessage zeroing (R6, R7, R8, R9)

**AC9** (R7, R9) -- Given a Claude JSONL session with `userMessage = "refactor the auth
module"`; when parsed through the Claude path; then `classifyTurn` receives the full
message (category is `refactoring`) AND the stored `ClassifiedTurn.userMessage` is `''`.

**AC10** (R8, R9) -- Given a Codex/Cursor provider call with a non-empty `userMessage`;
when parsed through `parseProviderSources`; then the resulting `ClassifiedTurn.userMessage`
stored in `sessionMap` is `''`.

**AC11** (R9) -- Given a `SessionSummary` retrieved from the cache; when inspected; then
every `turn.userMessage` field is `''` (zeroing is captured in the persisted summary, not
applied on read).

## Performance (R12, R13)

**AC12** (R12, D-PERF-1) -- Given the built bundle with warm cache (all session files
previously parsed and cached); when `node dist/cli.js status --format json` is run 3
times on the developer machine used for Phase 1 measurements; then the median wall time
is under 400ms.

**AC13** (R13) -- Given the fixture dataset `tests/fixtures/bench/` with an empty cache;
when the vitest bench "parse cold" case runs; then median iteration time is under 200ms.

**AC14** (R13) -- Given the fixture dataset with a populated cache; when the vitest bench
"parse warm" case runs; then median iteration time is under 20ms, and warm is at least
3x faster than cold.

**AC15** (R12, D-RSS-1) -- Given warm cache run of `status --format json` (built bundle);
when `process.memoryUsage().rss` is logged at exit; then the value is under 100MB
(104,857,600 bytes). Measurement taken after the main output is complete.

## Observability (R10)

**AC16** (R10) -- Given `DEBUG=1 codeburn status --format json` is run; when stderr is
captured; then exactly one line per session file appears, each prefixed with
`[session-cache] HIT` or `[session-cache] MISS`, and the total hit+miss count equals the
number of session files discovered.

**AC17** (R10) -- Given `codeburn status --format json` (no `DEBUG`); when stderr is
captured; then no `[session-cache]` lines appear.

## Schema resilience (R1, R11)

**AC18** (R11) -- Given the `session-cache.db` file is corrupted (truncated to 0 bytes);
when `openCache` is called; then it deletes the file, creates a fresh database, and
returns a working db handle without throwing.

**AC19** (R1) -- Given a valid database with a row whose `summary_json` is malformed JSON;
when `getCachedSummary` is called for that row; then it returns `null`, deletes the
corrupted row, and does not throw.

## Concurrent access (C7)

**AC20** (C7) -- Given two `codeburn` processes started within 100ms of each other; when
both attempt to write cache entries; then neither process exits with non-zero code,
neither produces garbled output, and the database passes `PRAGMA integrity_check`.

## Benchmark infrastructure (R15)

**AC21** (R15) -- Given `tests/fixtures/bench/` contains >= 5 synthetic JSONL files
totalling >= 500 lines; when `npx vitest bench` is run; then both "parse cold" and
"parse warm" bench cases execute successfully.

**AC22** (R15) -- Given the benchmark suite; when `npx vitest run` is executed; then the
`.bench.ts` files are not included in the test run.

**AC23** (R15) -- Given the benchmark suite runs; then the SQLite cache it uses is in a
temporary directory (not `~/.cache/codeburn/session-cache.db`).

## Correctness and regression (R14)

**AC24** (R14) -- Given the same dataset used for Phase 1 baseline; when
`codeburn status --format json` output (warm cache) is diffed against the Phase 1
baseline output; then the diff is empty (byte-for-byte identical).

**AC25** (R14) -- Given all spec changes committed; when `npx vitest run` is executed;
then all tests pass with zero failures.
