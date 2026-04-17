# Tasks: Performance Optimization v2 -- SQLite Session Cache

**Generated from**: docs/specs/performance-optimization-v2/
**Total tasks**: 10
**Parallel groups**: 3
**Estimated total effort**: 18h
**Max parallel agents**: 3

## File Conflict Matrix

| Task | Files Modified | Conflicts With |
|------|---------------|----------------|
| T1 | `src/session-cache.ts` (new) | -- |
| T2 | `tests/session-cache.test.ts` (new) | -- |
| T3 | `src/parser.ts` | T4, T5 |
| T4 | `src/parser.ts` | T3, T5 |
| T5 | `src/parser.ts`, `tests/parser.test.ts` | T3, T4 |
| T6 | `tests/parser.test.ts` | T5 |
| T7 | `tests/fixtures/bench/*.jsonl` (new, 5+ files) | -- |
| T8 | `tests/bench/parse-performance.bench.ts` (new) | -- |
| T9 | all (built bundle verification) | -- |
| T10 | all (built bundle verification) | -- |

## Task Graph

```
T1 ──→ T2 ──┐
             ├──→ T3 ──→ T4 ──→ T5 ──→ T6
T7 ──────────┘                            │
                                          ├──→ T9 ──→ T10
T8 (depends on T7, T3) ──────────────────┘
```

---

## Parallel Group 1: Foundation (no dependencies)

- [P] **T1**: Create SQLite session cache module
  - **Validates**: R1, R2, R3, R10, R11
  - **Files**: `src/session-cache.ts` (new)
  - **Complexity**: medium
  - **Effort**: 3h
  - **Details**:
    - Create `src/session-cache.ts` exporting `openCache`, `getCachedSummary`, `putCachedSummary`
    - Dynamic import of `better-sqlite3` (`await import('better-sqlite3')`)
    - `openCache`: create dir with `mkdirSync({ recursive: true })`, set `PRAGMA journal_mode = WAL` and `PRAGMA busy_timeout = 3000`, create `session_summaries` table with `CREATE TABLE IF NOT EXISTS`
    - `getCachedSummary`: lookup by `file_path + mtime_ms + file_size`, return deserialized `SessionSummary` or `null`. Delete row and return `null` if `JSON.parse` throws
    - `putCachedSummary`: `INSERT OR REPLACE` with serialized summary JSON and `cached_at` timestamp. Silently swallow write failures
    - `openCache` returns `null` if dynamic import fails, permission error, or corrupt db
    - Schema reset on error: delete corrupt db file and recreate; if recreation fails return `null`
    - `mtime_ms` stored as `Math.floor(stat.mtimeMs)` (R2)
    - DEBUG logging to `process.stderr`: `[session-cache] HIT/MISS <path>` (R10)
    - Reuse `SqliteDatabase` interface from `src/sqlite.ts` if compatible, or use `better-sqlite3` types directly

- [P] **T7**: Create synthetic benchmark fixture files
  - **Validates**: R15
  - **Files**: `tests/fixtures/bench/*.jsonl` (new, >= 5 files, >= 500 lines total)
  - **Complexity**: low
  - **Effort**: 1h
  - **Details**:
    - Create >= 5 JSONL files under `tests/fixtures/bench/`
    - Each file contains entries with `type`, `timestamp`, `message.model`, `message.usage`, and `message.content` fields
    - Must exercise the full parse path including classification
    - Files are committed to the repo

---

## Sequential Group 2: Cache Module Tests (depends on T1)

- **T2**: Write unit tests for session-cache module
  - **Validates**: R1, R2, R3, R10, R11
  - **Files**: `tests/session-cache.test.ts` (new)
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T1
  - **Details**:
    - AC1: `openCache` creates `~/.cache/codeburn/` if missing (use temp dir in test)
    - AC2: `openCache` returns `null` when `better-sqlite3` import fails
    - AC3: schema has correct columns and primary key
    - AC4: cache hit returns deserialized `SessionSummary`
    - AC5: mtime mismatch returns `null`, triggers re-parse
    - AC6: size mismatch (same mtime) returns `null`
    - AC16/AC17: DEBUG logging appears on stderr when `DEBUG` is set, absent when unset
    - AC18: corrupt db (truncated to 0 bytes) triggers delete-and-recreate
    - AC19: malformed `summary_json` returns `null` and deletes the row
    - Use isolated temp directory for all cache tests (not `~/.cache/codeburn/`)

---

## Sequential Group 3: Parser Integration (depends on T2, T7)

- **T3**: Integrate cache in `parseSessionFile`
  - **Validates**: R4, R5
  - **Files**: `src/parser.ts`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T2
  - **Details**:
    - Add optional `db: Database | null` parameter to `parseSessionFile`
    - When `db` is non-null: `stat` the file, call `getCachedSummary`; on hit return deserialized summary; on miss parse normally then call `putCachedSummary`
    - When `db` is null: behavior unchanged from Phase 1
    - In `parseAllSessions`: call `openCache` once, pass handle to `scanProjectDirs` and down to `parseSessionFile`
    - `openCache` must not be called more than once per CLI command execution (AC8)
    - Thread `db` through `scanProjectDirs` signature

- **T4**: Implement userMessage zeroing
  - **Validates**: R6, R7, R8, R9
  - **Files**: `src/parser.ts`
  - **Complexity**: low
  - **Effort**: 1h
  - **Depends on**: T3
  - **Cannot parallelize**: modifies same file as T3 (`src/parser.ts`)
  - **Details**:
    - Claude path (R7): after `turns.map(classifyTurn)` in `parseSessionFile`, iterate and set `classified.userMessage = ''` on each element
    - Provider path (R8): in `parseProviderSources`, after `classifyTurn(turn)`, set `classified.userMessage = ''` before pushing to `sessionMap`
    - Ordering invariant (R9): zeroing must happen after `classifyTurn` (which reads `userMessage`) and before `putCachedSummary`

- **T5**: Write integration tests for parser cache and zeroing
  - **Validates**: R4, R5, R6, R7, R8, R9
  - **Files**: `tests/parser.test.ts`, `src/parser.ts` (minor if needed)
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T4
  - **Cannot parallelize**: modifies same test file as T6 could depend on
  - **Details**:
    - AC7: `parseSessionFile` with `db = null` runs full Phase 1 path, no stat/cache calls
    - AC8: `openCache` called exactly once per `parseAllSessions` invocation (spy/mock)
    - AC9: Claude path -- `classifyTurn` receives full `userMessage` (category set correctly) AND stored `ClassifiedTurn.userMessage` is `''`
    - AC10: Provider path -- `ClassifiedTurn.userMessage` stored in `sessionMap` is `''`
    - AC11: `SessionSummary` from cache has all `turn.userMessage` fields as `''`

- **T6**: Write concurrent access test
  - **Validates**: C7
  - **Files**: `tests/session-cache.test.ts` or `tests/concurrent-cache.test.ts` (new)
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T5
  - **Details**:
    - AC20: two processes started within 100ms both write cache entries; neither exits non-zero, no garbled output, db passes `PRAGMA integrity_check`
    - May use `child_process.fork` or direct concurrent `putCachedSummary` calls with WAL mode

---

## Sequential Group 4: Benchmark Suite (depends on T7, T3)

- **T8**: Create automated benchmark suite
  - **Validates**: R13, R15
  - **Files**: `tests/bench/parse-performance.bench.ts` (new)
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T7, T3
  - **Details**:
    - Create `tests/bench/parse-performance.bench.ts` using vitest bench mode
    - "parse cold" case: empty cache, `parseAllSessions` under 200ms median
    - "parse warm" case: cache populated, `parseAllSessions` under 20ms median, at least 3x faster than cold
    - Use isolated cache in temporary directory (AC23)
    - Must be excluded from `npx vitest run` (AC22) -- configure vitest to exclude `.bench.ts`
    - Verify vitest config excludes bench files from default run

---

## Sequential Group 5: Verification (depends on all above)

- **T9**: End-to-end regression and output correctness
  - **Validates**: R14
  - **Files**: all (verification, no new code expected)
  - **Complexity**: low
  - **Effort**: 1h
  - **Depends on**: T5, T6, T8
  - **Details**:
    - AC24: `codeburn status --format json` output (warm cache) byte-for-byte identical to Phase 1 baseline
    - AC25: `npx vitest run` passes with zero failures
    - Run `npx tsx src/cli.ts report` and `npx tsx src/cli.ts today` to confirm interactive mode works
    - Verify no user-visible output changes (C1)

- **T10**: Performance target verification
  - **Validates**: R12, R13
  - **Files**: all (verification, minor tuning if needed)
  - **Complexity**: low
  - **Effort**: 2h
  - **Depends on**: T9
  - **Details**:
    - AC12: `node dist/cli.js status --format json` (built bundle, warm cache) under 400ms median of 3 runs
    - AC13: vitest bench "parse cold" under 200ms median
    - AC14: vitest bench "parse warm" under 20ms median, at least 3x faster than cold
    - AC15: peak RSS under 100MB (104,857,600 bytes)
    - Build with `npm run build` first
    - If targets not met: profile and optimize (may require changes to cache module or parser)

---

## Requirement Traceability

| Task | Requirements Validated |
|------|----------------------|
| T1 | R1, R2, R3, R10, R11 |
| T2 | R1, R2, R3, R10, R11 |
| T3 | R4, R5 |
| T4 | R6, R7, R8, R9 |
| T5 | R4, R5, R6, R7, R8, R9 |
| T6 | C7 |
| T7 | R15 |
| T8 | R13, R15 |
| T9 | R14 |
| T10 | R12, R13 |

All 15 requirements (R1-R15) are covered. No orphan tasks.

## Constraint Coverage

| Constraint | Covered By |
|------------|-----------|
| C1 | T9 (output identity check) |
| C2 | T1, T3, T4 (strict TS, no `any`) |
| C3 | T1 (synchronous better-sqlite3 API) |
| C4 | T1 (cache path `~/.cache/codeburn/session-cache.db`) |
| C5 | T4 (field retained, value cleared at runtime) |
| C6 | T1 (single table, reset on error) |
| C7 | T6 (WAL mode, concurrent access test) |
| C8 | T10 (median of 3 runs, built bundle) |
| C9 | T8 (bench excluded from vitest run) |
| C10 | T1 (required dep, optional at runtime) |
| C11 | T3, T4 (each independently committable) |
