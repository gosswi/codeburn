# Design Decisions: Performance Optimization v2

## D1 — Cache module path: src/session-cache.ts

Three exported functions: `openCache`, `getCachedSummary`, `putCachedSummary`.
This mirrors the pattern in `src/cursor-cache.ts` (three exported async functions)
and keeps cache logic isolated from the parser. The db handle type reuses the
`SqliteDatabase` interface from `src/sqlite.ts`, avoiding a new type.

**Override note**: Spec A proposed the same module path and function names. Codebase
confirms `src/cursor-cache.ts` as the precedent pattern; D5 from Spec A adopted as-is.

## D2 — Cache filename: session-cache.db

Filename is `session-cache.db` inside `~/.cache/codeburn/` (the existing cache dir used
by `cursor-cache.ts` and pricing). The name `session-cache.db` is more descriptive than
alternatives and follows the existing `cursor-results.json` naming pattern (noun-noun).

**Conflict resolved**: Spec A used `codeburn-sessions.db`; Spec B used `session-cache.db`.
Chose Spec B's name: it pairs naturally with the module name `session-cache.ts` and
matches the `cursor-cache.ts` -> `cursor-results.json` naming relationship.

## D3 — No dateRange filter at cache level

The cached `SessionSummary` stores all turns regardless of the `dateRange` passed to
`parseAllSessions`. `filterProjectsByDateRange` (Phase 1) handles date narrowing after
the summary is returned from cache. Filtering at cache key level would require a separate
cache entry per date range combination, defeating the purpose of the cache.

**Source**: Spec A's D3. Codebase confirms: `filterProjectsByDateRange` is already the
in-memory narrowing primitive. Cache stores the full summary; callers filter.

## D4 — better-sqlite3 native build failure: skip cache, not crash

If `require('better-sqlite3')` fails (native `.node` file absent, wrong Node version,
or platform mismatch), `openCache` returns `null`. `parseAllSessions` proceeds with Phase
1 performance. This matches the Cursor provider's lazy-load pattern in
`src/providers/index.ts` and `src/sqlite.ts`. The cache is an optimisation, not a
correctness dependency.

**Interview binding**: D-CACHE-1 states "SQLite cache required dependency" — this means
SQLite is the chosen mechanism, not that the build failure is fatal. The CLI must not
crash when the native addon is absent.

## D5 — Measurement method: median of 3 runs (D-PERF-1)

Performance targets (AC11, AC12, AC13) use median of 3 runs with the built bundle.
No warmup runs discarded before measurement. This matches the method recorded in
`docs/specs/performance-analysis-v2.md` (the Phase 1 post-implementation report) and
interview decision D-PERF-1.

**Conflict resolved**: Spec B proposed warmup-2/runs-10 (hyperfine defaults). Chose
median-of-3 per D-PERF-1. The Phase 1 report used median of 3; continuity matters for
comparing before/after.

## D6 — No explicit closeCache

The `openCache` db handle is not closed explicitly. For a short-lived CLI (< 10s), SQLite
will flush WAL and close cleanly on process exit. Adding a `closeCache` call requires
threading the handle through to process exit or using a `finally` block in every command,
adding complexity for no correctness benefit. Defer to Phase 3 if daemon mode is added.

**Conflict resolved**: Spec B included a `closeCache` requirement. Evaluator flagged it
as unnecessary for short-lived CLI. Dropped.

## D7 — Cache size: unbounded, accepted for Phase 2

No row eviction policy is implemented. `cached_at` is stored to enable future TTL-based
eviction in Phase 3. At ~1-5KB per serialised `SessionSummary`, even 500 sessions
consume less than 3MB on disk — acceptable for a developer tool cache.

## D8 — Schema reset on error (D-SCHEMA-1)

If the db file is corrupt, `openCache` deletes it and recreates. This sacrifices one
cold-parse cycle but guarantees the CLI never crashes due to cache corruption. No user
intervention required. All errors in the cache module are swallowed; `DEBUG` env var
exposes them to stderr.

## D9 — userMessage zeroing after classify, before cache write

`classifyTurn` reads `userMessage` for keyword-based category detection. Zeroing must
happen after classification returns and before `putCachedSummary` serialises the summary.
This ordering is an invariant enforced by the requirement ordering (R7 -> R9) and tested
by AC8/AC9.

**Source**: Spec A's three-requirement split (R6/R7/R8 in consolidated numbering) adopted
because it makes the ordering constraint explicit and independently testable.

---

## Perspectives

- **D2 (cache filename)**: Spec A proposed `codeburn-sessions.db`; Spec B proposed
  `session-cache.db`. Chose Spec B because the module name `session-cache.ts` makes the
  pairing obvious to future engineers.

- **D5 (measurement method)**: Spec A specified median-of-3 (matching the Phase 1 report);
  Spec B specified hyperfine warmup-2/runs-10. Chose Spec A's method for continuity with
  existing measurements. Spec B's approach is valid for micro-benchmarks; reserved for
  Phase 3 when targeting sub-100ms precision.

- **D6 (closeCache)**: Spec B required an explicit `closeCache`. Evaluator flagged this
  as unnecessary overhead for a short-lived CLI. Dropped; preserved as a note for Phase 3
  daemon-mode work.
