# Constraints: Performance Optimization v2

## C1 -- No user-visible output changes

CLI interface, flags, output formats, exit codes, and all displayed numbers must be
identical before and after. No new subcommands or flags.

## C2 -- TypeScript strict mode, no `any` types

All new code must comply with `"strict": true`. Type assertions (`as`) are permitted
only for `JSON.parse` results where the schema shape is known.

## C3 -- Synchronous better-sqlite3 API only

All cache reads and writes must use `better-sqlite3`'s synchronous API
(`db.prepare().get()`, `db.prepare().run()`). No async wrappers or worker threads.

## C4 -- Cache path follows existing convention

Cache database at `~/.cache/codeburn/session-cache.db`. Same directory used by
pricing cache and cursor-cache. No new configuration option for the path.

## C5 -- userMessage field retained in types.ts

`ClassifiedTurn` retains `userMessage: string`. The field value is cleared to `''`
at runtime; the type shape is unchanged.

## C6 -- Schema reset on error, no versioning

Single table `session_summaries`. No version column. Incompatible schema or corrupt db
triggers delete-and-recreate. Any future schema or SessionSummary shape change will
require a full cache reset -- this is acceptable because the cache is reconstructable
from source JSONL data.

## C7 -- WAL mode for concurrent access

`openCache` must set `PRAGMA journal_mode = WAL` and `PRAGMA busy_timeout = 3000`
to handle concurrent CLI + menubar widget invocations. Write failures (including
busy_timeout exhaustion) are silently swallowed; the parsed result is still returned.

## C8 -- Measurement method: median of 3 runs, built bundle

Performance targets verified with `node dist/cli.js` (built bundle), median of 3
runs. `npx tsx` numbers not used for target verification (D-PERF-1).

## C9 -- Benchmark excluded from default test run

`tests/bench/*.bench.ts` must not run during `npx vitest run`. Only via
`npx vitest bench`.

## C10 -- better-sqlite3: required npm dependency, optional at runtime

`better-sqlite3` is listed in `dependencies` (not `optionalDependencies`) in
`package.json`. However, `src/session-cache.ts` imports it dynamically
(`await import('better-sqlite3')`). If the import fails at runtime (native addon
absent, wrong platform), `openCache` returns `null` and the CLI degrades to Phase 1
performance. This means: `npm install` on supported platforms pulls the dep; on
unsupported platforms install may fail, which is acceptable per D-CACHE-1.

## C11 -- Each task independently committable

SQLite cache and userMessage zeroing must each pass `npx vitest run` when
committed independently.
