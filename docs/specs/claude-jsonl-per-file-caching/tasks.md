# Tasks: Claude JSONL Per-File Caching

**Generated from**: docs/specs/claude-jsonl-per-file-caching/
**Total tasks**: 4
**Parallel groups**: 2
**Estimated total effort**: 6-8h (wall-clock ~3-4h with parallel execution)
**Max parallel agents**: 2

## File Conflict Matrix

| Task | Files Modified | Conflicts With |
|------|---------------|----------------|
| T1 | internal/provider/claude/claude.go | — |
| T2 | internal/parser/parser.go | — |
| T3 | internal/provider/claude/claude_test.go | — |
| T4 | internal/parser/parser_test.go, internal/parser/cache_test.go | — |

No file conflicts. T1/T2 can run in parallel. T3/T4 can run in parallel after their dependencies.

## Task Graph

```
T1 ──┬──→ T3
     │
T2 ──┴──→ T4
```

T3 depends on T1 only (tests `claude.go` directly, no `parseSource` involvement).
T4 depends on T1 AND T2 (tests `parseSource` which calls `prov.ParseSession`).

## Parallel Group 1: Provider + Parser changes

- [P] **T1**: Refactor `claude.Provider` for per-file sources
  - **What**: In `DiscoverSessions`, call `collectJSONLFiles` for each project directory
    and emit one `SessionSource` per file (`Path` = absolute file path,
    `Project` = `filepath.Base(projectDir)`). In `ParseSession`, remove the
    `collectJSONLFiles` call and call `parseFile(source.Path, seenKeys, &calls)` directly.
  - **Validates**: R-01, R-02
  - **Files**: `internal/provider/claude/claude.go`
  - **Complexity**: low
  - **Effort**: 1h
  - **Key constraint**: Subagent files (from `<uuid>/subagents/`) must receive the parent
    project directory name, not a path derived from the subagent path (C-04). The project
    name is in scope in the outer `DiscoverSessions` loop before `collectJSONLFiles` is called.

- [P] **T2**: Remove `isClaudeDir` cache skip and add mtime guard in `parseSource`
  - **What**: Delete the `if isClaudeDir { // skip }` branch (lines 350-368 of `parser.go`)
    so Claude file sources call `GetFileFingerprint` + `GetCachedSummary` + `PutCachedSummary`
    like all other providers. After `GetFileFingerprint` succeeds, add the mtime guard:
    `if time.Now().UnixMilli()-mtimeMs < 5000 { cacheKey = "" }` — this leaves `cacheKey`
    empty so the existing write guard `if cache != nil && cacheKey != ""` suppresses the
    write without new branches. Retain `isClaudeDir` for the turn-grouping block (lines
    394-419); its value is still correct when `src.Path` is a single file path.
  - **Validates**: R-03, R-04, R-05, R-06, R-07, R-09, R-10
  - **Files**: `internal/parser/parser.go`
  - **Complexity**: low
  - **Effort**: 1h
  - **Key constraint**: Guard threshold is exactly 5000ms, not configurable (C-09).
    One `os.Stat` call per file — reuse `mtimeMs`/`fileSize` from `GetFileFingerprint`
    for both lookup and write (C-02).

## Parallel Group 2: Tests (depends on Group 1)

- [P] **T3**: Tests for `claude.go` changes
  - **What**: Add/update tests in `claude_test.go`:
    - `TestDiscoverSessions_PerFileEmission` — two project dirs × two files → 4 sources,
      each `Path` ends in `.jsonl`, `Project` = dir name (AC-01)
    - `TestDiscoverSessions_SubagentProject` — top-level + subagent file → same `Project`
      value (AC-02)
    - `TestDiscoverSessions_EmptyDir` — zero files → zero sources, no error (AC-02 edge)
    - `TestParseSession_SingleFile` — single `.jsonl` with 2 entries → 2 calls, no dir
      walk (AC-03)
    - Restructure `TestGlobalDedupAcrossFiles` — two separate single-file `ParseSession`
      calls with shared `seenKeys` map; second call yields 0 on duplicate `msg.ID` (AC-15)
  - **Validates**: R-01, R-02, R-11
  - **Files**: `internal/provider/claude/claude_test.go`
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T1

- [P] **T4**: Tests for `parseSource` cache changes
  - **What**: Add tests in `parser_test.go` (integration, uses `CLAUDE_CONFIG_DIR` redirect
    and `openCacheAt` with `t.TempDir()`):
    - `TestParseSource_CacheHit_Claude` — unchanged file >5s old → no re-parse (AC-04)
    - `TestParseSource_CacheMiss_Claude` — cold cache, `APICalls > 0` → `PutCachedSummary`
      called (AC-05)
    - `TestParseSource_FingerprintInvalidation_Claude` — mtime change → miss, re-parse,
      re-cache (AC-06)
    - `TestParseSource_MtimeGuard_Within5s` — file <5s old → no cache read/write, still
      parsed (AC-07)
    - `TestParseSource_MtimeGuard_Boundary` — file at exactly 5000ms → cache used (AC-08)
    - `TestParseSource_TurnGrouping` — user+assistant+user+assistant → 2 turns, APICalls=2
      (AC-09)
    - `TestParseSource_FingerprintError` — deleted file → parsed (returns nil), no cache
      ops (AC-10)
    - `TestParseSource_WriteError` — nil cache → summary returned, no panic (AC-11)
    - `TestParseSource_UserMessageZeroed` — round-trip: stored and retrieved summary have
      empty `UserMessage` (AC-12)
    - `TestParseSource_ZeroAPICalls` — empty JSONL → nil, no cache write (AC-13)
    - `TestParseSource_DateFilterAfterCache` — cached full summary + dateRange → filtered
      result (AC-14)
  - **Validates**: R-03, R-04, R-05, R-06, R-07, R-08, R-09, R-10
  - **Files**: `internal/parser/parser_test.go`, `internal/parser/cache_test.go`
  - **Complexity**: high
  - **Effort**: 3h
  - **Depends on**: T1, T2
  - **Note**: Use `openCacheAt(t.TempDir()+"/test.db", false)` for a test-scoped cache.
    Control mtime guard timing by writing the file and manipulating `os.Chtimes` to a
    timestamp >5s in the past for cache-hit tests.
