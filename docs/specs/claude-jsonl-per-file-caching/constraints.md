# Constraints: Claude JSONL Per-File Caching

| ID | Constraint | Type | Source |
|----|-----------|------|--------|
| C-01 | `collectJSONLFiles` signature and walk logic must not be modified. Its call site moves from `ParseSession` to `DiscoverSessions`. | Technical | Interview |
| C-02 | No second `os.Stat` call per file beyond the single call inside `GetFileFingerprint`. The fingerprint result must be reused for both cache lookup and cache write. | Performance | Interview |
| C-03 | `GetFileFingerprint` returns mtime as Unix milliseconds (`info.ModTime().UnixMilli()`). All comparisons must use millisecond precision. | Technical | Codebase (cache.go:145) |
| C-04 | File paths stored as cache keys are the absolute, clean paths produced by `filepath.Join` inside `collectJSONLFiles`. No additional normalization is required. | Technical | Codebase (claude.go:114-138) |
| C-05 | `PutCachedSummary` silently swallows write errors (always returns nil). Callers must not retry or surface these errors. | Technical | Codebase (cache.go:122-137) |
| C-06 | `UserMessage` on every `ClassifiedTurn` must be zeroed before `PutCachedSummary` is called (privacy invariant). | Privacy | Codebase (parser.go:408-417) |
| C-07 | The SQLite schema (`session_summaries` table) must not be altered. | Technical | Codebase (cache.go:17-25) |
| C-08 | The existing in-process 60s LRU (`ParseAllSessionsCached`) must not be removed. | Technical | Interview |
| C-09 | The mtime guard threshold is exactly 5000 milliseconds and must not be configurable via flags, env vars, or config fields. | Technical | Interview (D-EDGE-1) |
