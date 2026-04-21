# Requirements: Claude JSONL Per-File Caching

## Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| R-01 | `claude.Provider.DiscoverSessions()` must call `collectJSONLFiles` for each project directory and emit one `SessionSource` per `.jsonl` file. `Path` = absolute file path. `Project` = `filepath.Base(projectDir)`. `Provider` = `"claude"`. | Must |
| R-02 | `claude.Provider.ParseSession()` must treat `source.Path` as a single `.jsonl` file and call `parseFile(source.Path, seenKeys, &calls)` directly. Must not call `collectJSONLFiles`. | Must |
| R-03 | `parseSource` must remove the `isClaudeDir` branch from the cache block. Claude file sources must call `GetFileFingerprint(src.Path)`, then `GetCachedSummary`, using the same code path as non-Claude providers. No new provider-name branching in the cache block. | Must |
| R-04 | After `GetFileFingerprint` succeeds, if `time.Now().UnixMilli() - mtimeMs < 5000`, set `cacheKey = ""` and skip both `GetCachedSummary` and `PutCachedSummary`. The file must still be parsed and returned normally. | Must |
| R-05 | The `isClaudeDir` variable must be retained in `parseSource` to gate `groupClaudeCalls` / `byFile` turn-grouping. Its value (`prov.Name() == "claude"`) remains correct when `src.Path` is a file path. | Must |
| R-06 | When `GetFileFingerprint` returns an error, `parseSource` must skip the cache and parse the file without calling `PutCachedSummary`. | Must |
| R-07 | When `PutCachedSummary` is called and fails, `parseSource` must still return the successfully-built `SessionSummary`. | Must |
| R-08 | All `ClassifiedTurn.UserMessage` fields must be zeroed before `PutCachedSummary` is called. | Must |
| R-09 | A file whose parsed result has `APICalls == 0` must not be cached. The existing `if summary.APICalls == 0 { return nil }` guard handles this. | Must |
| R-10 | Full summaries (all turns) must be stored in cache. `filterSessionByDateRange` is applied after cache retrieval, not before caching. | Must |
| R-11 | `TestGlobalDedupAcrossFiles` must be updated: test two separate single-file `ParseSession` calls sharing a `seenKeys` map, not a single directory-level call. | Must |

## Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NF-01 | One `os.Stat` per file per invocation (inside `GetFileFingerprint`). Fingerprint reused for both lookup and write. | 1 stat/file |
| NF-02 | Cache reads and writes must not block goroutines beyond the existing SQLite `busy_timeout`. | 3000ms max |
| NF-03 | Observable output (cost totals, token counts, project names) must be unchanged for projects with unmodified JSONL files. | 0 delta |
