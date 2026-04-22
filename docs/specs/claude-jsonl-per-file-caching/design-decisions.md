# Design Decisions: Claude JSONL Per-File Caching

## D-01 — Per-file sources via DiscoverSessions (Option A)
**Status**: accepted
**Context**: Two approaches to per-file caching existed. Option A: change `DiscoverSessions` to
emit one `SessionSource` per file; each file enters the worker pool independently. Option B:
keep directory-level sources; add a per-file cache loop inside `parseSource` for Claude dirs.
**Decision**: Option A. Each JSONL file becomes a separate task in `ParseAllSessions`'s
`NumCPU*2` worker pool, giving intra-project parallelism for both cold and warm paths. Option A
also avoids a summary-merge function that Option B would require to combine per-file
`SessionSummary` objects back into a single project summary.
**Consequences**: `DiscoverSessions` and `ParseSession` in `claude.go` are modified. The number
of tasks in the worker pool grows (one per file instead of one per project dir). The `isClaudeDir`
cache skip in `parseSource` is removed; Claude file sources use the same path as Codex/Cursor.
**Alternatives considered**: Option B (loop inside parseSource) — rejected because files within
a project are processed sequentially (worse cold-parse performance) and requires a new merge
function. Directory-level fingerprinting — rejected because directory mtime is unreliable when
child files change without a rename.

## D-02 — Mtime guard for actively-written files
**Status**: accepted
**Context**: A JSONL file currently being written by Claude Code has an mtime close to
`time.Now()`. Caching a partial snapshot causes a stale cache hit on the next run until the
mtime or size changes again.
**Decision**: Skip both `GetCachedSummary` and `PutCachedSummary` when
`time.Now().UnixMilli() - mtimeMs < 5000`. Set `cacheKey = ""` so the existing write guard
`if cache != nil && cacheKey != ""` also suppresses the write without new branches. The file is
still parsed and returned.
**Consequences**: Files written within the last 5 seconds are always re-parsed. Files idle for
5+ seconds benefit from the cache.
**Alternatives considered**: Guard write path only (allow cache read) — rejected because a prior
cached entry for the same path could be a stale partial snapshot if the file was cached then
appended to without triggering a stat change within the guard window. Configurable threshold —
rejected per C-09.

## D-03 — One stat call per file
**Status**: accepted
**Context**: A naive implementation could call `os.Stat` once for the lookup fingerprint and
again before the write.
**Decision**: Call `GetFileFingerprint` once; store `(mtimeMs, fileSize)` in local variables;
reuse for both `GetCachedSummary` and `PutCachedSummary`.
**Consequences**: If the file is modified between stat and parse, the stored summary corresponds
to new content but the old fingerprint, causing a miss on the next run. This is safe (miss is
safe; a stale hit cannot occur from this race).
**Alternatives considered**: Re-stat before write — rejected because it doubles stat calls and
the race window is negligible.

## D-04 — Date filtering applied after cache retrieval
**Status**: accepted
**Context**: Users request different date ranges. Caching a date-filtered view would require one
row per (file, dateRange).
**Decision**: Cache the full summary; apply `filterSessionByDateRange` after the cache hit, as
already done for non-Claude providers.
**Consequences**: Cached rows are reusable across all date-range requests.
**Alternatives considered**: Cache per (file, dateRange) — rejected; multiplies rows and
complicates invalidation.

## D-05 — Privacy: zero UserMessage before cache write
**Status**: accepted
**Context**: `ClassifiedTurn.UserMessage` holds the raw user prompt and must not be persisted.
**Decision**: Zero `ct.UserMessage = ""` at the classification site (already done at
parser.go:408-417) before `PutCachedSummary`. No change required.
**Consequences**: Cached summaries never contain user prompts. `TestCachePrivacyInvariant`
catches accidental removal.
**Alternatives considered**: Zero inside `PutCachedSummary` — rejected; couples cache layer to
privacy logic and breaks the established zeroing pattern.

## D-06 — Error handling: fail-open on all cache errors
**Status**: accepted
**Context**: SQLite may be unavailable. Caching must never break parsing.
**Decision**: All cache errors (stat failure, read error, write error) fall back to normal
parsing. `parseSource` never propagates cache errors to callers.
**Consequences**: In degraded cache conditions, performance falls to pre-feature behavior.
**Alternatives considered**: Surface write errors as warnings — rejected; pollutes stdout and
the `PutCachedSummary` contract already swallows errors.

## D-07 — collectJSONLFiles call site moves to DiscoverSessions
**Status**: accepted
**Context**: `collectJSONLFiles` was called inside `ParseSession` to enumerate a directory. After
Option A, `ParseSession` receives a single file path.
**Decision**: Move the `collectJSONLFiles` call to `DiscoverSessions`, which iterates each
returned path and emits a `SessionSource` per file. `collectJSONLFiles` is not modified (C-01).
**Consequences**: Subagent files (`<uuid>/subagents/*.jsonl`) are naturally discovered and emitted
with the correct `Project` value (captured in the outer loop before iterating file paths).
**Alternatives considered**: Add a new discovery helper — rejected; `collectJSONLFiles` already
returns exactly the paths needed.

## D-08 — Global dedup across per-file sources
**Status**: accepted
**Context**: Each file source gets its own `localSeen` map in `parseSource`. Cross-file dedup
within a project relies on `globalSeen sync.Map` in `ParseAllSessions`.
**Decision**: Accept this trade-off; same model used by Codex. `TestGlobalDedupAcrossFiles`
is restructured to test two single-file sources sharing a `seenKeys` map.
**Consequences**: Duplicate `msg.ID` values across files in the same project are deduplicated via
`globalSeen` per run. Cache hits do not re-register dedup keys (safe: each run starts fresh).
**Alternatives considered**: Re-register cached file dedup keys into `globalSeen` — rejected;
`SessionSummary` stores aggregated counts, not raw dedup keys.
