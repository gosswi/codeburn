# Spec: Performance Optimization v2 — SQLite Session Cache

**Status**: Draft
**Version**: 1.1.0
**Date**: 2026-04-17
**Tier**: 2 (Feature)
**Branch convention**: `feat/session-cache`

---

## Objective

Bring `codeburn status --format json` (built bundle) under 400ms median wall time
and reduce peak RSS below 100MB. Phase 1 optimizations (streaming, widen-then-filter,
discovery cache, bash extraction) delivered 589ms and 185MB — short of both targets.
The remaining bottleneck is the ~280ms single-pass parse of all JSONL data on every
invocation. This spec introduces a persistent SQLite session cache that skips re-parsing
unchanged files, targeting a further 200-250ms reduction.

---

## Out of Scope

- Node.js startup time (D-SCOPE-1): fixed ~50-70ms, out of scope
- Cursor SQLite query optimization: already cached in `cursor-cache.ts`
- Parallelising JSONL reads with `Promise.all`
- Changes to any user-visible output format or numeric value
- TTL-based cache expiry (deferred to Phase 3; `cached_at` column enables it)
- Implementing cache close/cleanup (process exit handles the db handle for a short-lived CLI)

---

## Architecture

```
parseAllSessions()
  └─ parseSessionFile(filePath)
       ├─ [CACHE HIT]  getCachedSummary(db, filePath, mtime, size) -> SessionSummary
       └─ [CACHE MISS] full JSONL parse -> putCachedSummary(db, filePath, mtime, size, summary)
```

Cache module: `src/session-cache.ts`
Cache location: `~/.cache/codeburn/session-cache.db`
Invalidation: file mtime (ms) + file size (bytes) — same strategy as `cursor-cache.ts`

---

## Risks

- RSS may increase slightly: cached summary objects are smaller than raw entries, but the
  db handle adds overhead. Expected net reduction. Verified post-implementation.
- better-sqlite3 native build failure: if the build fails, the cache is silently skipped.
  Acceptable; CLI degrades to Phase 1 performance, not a crash.
- Schema corruption: reset-on-error policy (D-SCHEMA-1) prevents crashes at the cost of
  one cold-parse cycle.

---

## Changelog

| Version | Date | Change |
|---------|------|--------|
| 1.1.0 | 2026-04-17 | Post-evaluation revision: fixed 6 blocking issues (AC20 misassignment, R5 missing AC, perf AC protocols, D-BENCH-1 coverage, C7 AC coverage, dep model clarification). Added R15, AC8, AC19-AC25. |
| 1.0.0 | 2026-04-17 | Consolidated from 2 independent drafts -- multi-agent spec generation |
