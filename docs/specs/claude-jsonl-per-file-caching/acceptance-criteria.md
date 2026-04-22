# Acceptance Criteria: Claude JSONL Per-File Caching

## AC-01 — DiscoverSessions emits one source per file
**Given** a Claude projects dir with two subdirs each containing two `.jsonl` files
**When** `DiscoverSessions()` is called
**Then** it returns 4 `SessionSource` entries; each `Path` ends in `.jsonl`; each `Project` equals its parent directory name
**Traces**: R-01

## AC-02 — Subagent files get parent project name
**Given** a project dir with `session.jsonl` and `<uuid>/subagents/sub.jsonl`
**When** `DiscoverSessions()` is called
**Then** both sources carry the same `Project` value (the project directory name, not a subagent path component)
**Traces**: R-01

## AC-03 — ParseSession handles single file path
**Given** a `SessionSource` with `Path` pointing to a single `.jsonl` with two assistant entries
**When** `ParseSession(source, seenKeys)` is called
**Then** it yields 2 `ParsedCall` values without calling `collectJSONLFiles`
**Traces**: R-02

## AC-04 — Cache hit skips file parse
**Given** a JSONL file older than 5s with mtime T and size S already in cache
**When** `parseSource` is called for the same file
**Then** `GetCachedSummary` returns the cached summary and the file is not re-opened
**Traces**: R-03

## AC-05 — Cache miss triggers parse and write
**Given** no cache entry exists for a JSONL file older than 5s
**When** `parseSource` processes that file with APICalls > 0
**Then** the file is parsed and `PutCachedSummary` is called with path, mtimeMs, fileSize
**Traces**: R-03

## AC-06 — Stale fingerprint causes cache miss
**Given** a cached entry for path P with mtime T0
**When** `GetCachedSummary` is called with mtime T1 (T1 != T0)
**Then** the result is nil and the file is re-parsed
**Traces**: R-03

## AC-07 — Mtime guard: file within 5s skips cache entirely
**Given** a JSONL file where `time.Now().UnixMilli() - mtimeMs < 5000`
**When** `parseSource` processes that file
**Then** neither `GetCachedSummary` nor `PutCachedSummary` is called; the file is still parsed and returned
**Traces**: R-04

## AC-08 — Mtime guard boundary: exactly 5000ms uses cache
**Given** a JSONL file where `time.Now().UnixMilli() - mtimeMs == 5000`
**When** `parseSource` processes that file on a cold cache
**Then** `GetCachedSummary` is called (guard condition `< 5000` does not fire) and on miss `PutCachedSummary` is called
**Traces**: R-04

## AC-09 — Turn grouping preserved
**Given** a JSONL file with user message U1, assistant A1, user message U2, assistant A2
**When** `parseSource` processes that file
**Then** `groupClaudeCalls` produces 2 turns and `SessionSummary.APICalls == 2`
**Traces**: R-05

## AC-10 — Fingerprint error falls back to parse
**Given** a JSONL file whose `os.Stat` call returns an error
**When** `parseSource` processes that file
**Then** the file is parsed without any cache read or write
**Traces**: R-06

## AC-11 — Write error does not suppress summary
**Given** the SQLite DB is unavailable at write time
**When** `PutCachedSummary` is called after a successful parse
**Then** `parseSource` returns the fully-built `SessionSummary`
**Traces**: R-07

## AC-12 — UserMessage zeroed before cache write and on round-trip
**Given** a JSONL file with non-empty user messages
**When** `parseSource` calls `PutCachedSummary` and the summary is later read back via `GetCachedSummary`
**Then** every `ClassifiedTurn.UserMessage` is `""` in both the written and retrieved summary
**Traces**: R-08

## AC-13 — Zero-call file skips cache write
**Given** a JSONL file containing only entries with zero token counts
**When** `parseSource` processes that file
**Then** `PutCachedSummary` is not called and nil is returned
**Traces**: R-09

## AC-14 — Date filtering applied after cache retrieval
**Given** a cached full summary spanning 30 days and a dateRange for the last 7 days
**When** `parseSource` retrieves the cached summary
**Then** `filterSessionByDateRange` is applied and only turns within 7 days are returned
**Traces**: R-10

## AC-15 — Cross-file dedup via shared seenKeys
**Given** two JSONL files in the same project each containing an assistant entry with the same `msg.ID`
**When** `ParseSession` is called for each file in sequence with a shared `seenKeys` map
**Then** the second call yields 0 `ParsedCall` entries (global dedup suppresses the duplicate)
**Traces**: R-11
