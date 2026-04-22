# Code Review: Go Migration (`internal/`)

**Date**: 2026-04-21
**Reviewer**: sdlc-toolkit:code-review
**Branch**: feature/go-migration
**Scope**: All files under `internal/` (~5 000 lines of logic, ~2 580 lines of tests)

---

## Summary

**Change size**: ~5 000 lines of logic | `[OVERSIZED-PR]` by the 300-line guideline, but this is a full language migration -- splitting by feature would be artificial churn. Flagged for the record only.

**Blast radius**: All data paths -- session parsing, classification, cost calculation, export, TUI, menubar. Effectively the entire product.

**All tests pass**: `go test ./internal/...` -- 14 packages, all green.

---

## CRITICAL

None.

---

## HIGH

### H1: `provider.ToApiCall` is dead code and inconsistent with the conversion functions actually used

`internal/provider/provider.go:46-68`

`ToApiCall` is exported but never called. The actual converters are `claudeCallToApiCall` (`parser.go:264`) and `providerCallToTurn` (`parser.go:231`). Both set `McpTools: extractMcpTools(tools)` -- `ToApiCall` omits that field entirely, so any caller would get an empty `McpTools` slice. Delete `ToApiCall` or align it and add a call site.

### H2: Level 3 fuzzy match iterates over a Go map -- non-deterministic pricing for rare models

`internal/models/models.go:89-93`

```go
for key, costs := range pricing {
    if strings.HasPrefix(canonical, key) || strings.HasPrefix(key, canonical) {
```

Map iteration order in Go is randomized per run. For an unknown model that could match multiple keys, the returned price is non-deterministic across runs. The `strings.HasPrefix(key, canonical)` direction also risks matching a short canonical name (`"gpt-4"`) against a longer unrelated key. Narrow to `strings.HasPrefix(canonical, key)` only, and consider sorting keys by length descending (longest-match-wins) for determinism.

---

## MEDIUM

### M1: `int64str` is a hand-rolled int-to-string function that should not exist

`internal/provider/codex/codex.go:364-384`

`strconv.FormatInt(n, 10)` does exactly this in one line. The 20-line custom implementation adds maintenance surface and is non-idiomatic Go.

### M2: `reMonth` and `reDay` are identical regexps

`internal/provider/codex/codex.go:17-18`

```go
var reMonth = regexp.MustCompile(`^\d{2}$`)
var reDay   = regexp.MustCompile(`^\d{2}$`)
```

Same pattern, two compiled regexps. Consolidate to one `reTwoDigit`.

### M3: `httpError.Error()` discards the status code

`internal/models/litellm.go:105-108`

```go
type httpError struct{ code int }
func (e *httpError) Error() string { return "HTTP error" }
```

The struct carries `code` but the error message doesn't include it. `fmt.Sprintf("HTTP %d", e.code)` would be more informative in logs.

### M4: `selectAllProjects` hardcodes `"30 Days"` as a string literal

`internal/export/export.go:291-301`

This couples the export logic to the string representation of a period label. If the label ever changes, export silently falls back to the last period. Use a period constant or a sentinel value.

---

## LOW

### L1: Empty result not cached in `ParseAllSessionsCached`

`internal/parser/parser.go:595`

```go
if err != nil || data == nil {
    return data, err
}
```

A nil result (no sessions found) is never cached, so a user with zero sessions will re-run the full discovery on every TUI refresh tick. Cache the empty slice too.

### L2: `hasMcpTools` uses a manual index check instead of `strings.HasPrefix`

`internal/classifier/classifier.go:126-130`

```go
if len(t) >= 5 && t[:5] == "mcp__" {
```

`strings.HasPrefix(t, "mcp__")` is idiomatic and handles the length guard internally.

---

## NIT

- `cursor.go:228-231`: `userMessages[convIDStr] = convMessages[1:]` copies a slice header on every row -- fine at scale, but `index + offset` would avoid the allocation.
- `parser.go:352-367`: the `// TODO: T11 in-process cache covers the hot path` comment for skipped Claude directory caching is fine to keep but could note when it would be removed.

---

## Five-Axis Summary

| Axis | Verdict | Notes |
|---|---|---|
| Correctness | PASS | Dedup, date filtering, token math, turn grouping all correct |
| Security | PASS | CSV injection protection present, SQLite WAL+busy_timeout set, no secrets in code, file perms standard |
| Performance | PASS | Bounded goroutine pool, sync.Map for cross-goroutine dedup, pre-allocated result slots, two-tier cache (SQLite + in-process 60s LRU) |
| Maintainability | 2 issues | Dead/inconsistent `ToApiCall`, duplicate regexps |
| Test coverage | PASS | All 14 packages with logic have tests; privacy invariant, dedup edge cases, scanner buffer limits, subagent JSONL -- real behaviors tested |

---

## Verdict

**APPROVE with comments** -- no blockers. H1 (`ToApiCall` dead code with missing `McpTools`) and H2 (non-deterministic fuzzy pricing) should be fixed before this ships to avoid future footguns.
