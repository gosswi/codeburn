# Traceability Matrix: Claude JSONL Per-File Caching

| Requirement | Acceptance Criteria | Design Decisions | Constraints | Code | Tests |
|-------------|---------------------|------------------|-------------|------|-------|
| R-01 | AC-01, AC-02 | D-01, D-07 | C-01, C-04 | claude.go:`DiscoverSessions` | claude_test.go:`TestDiscoverSessions_PerFileEmission` |
| R-02 | AC-03 | D-01, D-07 | C-01 | claude.go:`ParseSession` | claude_test.go:`TestParseSession_SingleFile` |
| R-03 | AC-04, AC-05, AC-06 | D-01 | C-02, C-07 | parser.go:`parseSource` | parser_test.go:`TestParseSource_CacheHit_Claude`, `TestParseSource_CacheMiss_Claude`, `TestParseSource_FingerprintInvalidation_Claude` |
| R-04 | AC-07, AC-08 | D-02 | C-09 | parser.go:`parseSource` | parser_test.go:`TestParseSource_MtimeGuard_Within5s`, `TestParseSource_MtimeGuard_Boundary` |
| R-05 | AC-09 | D-01 | — | parser.go:`parseSource` (isClaudeDir turn-grouping) | parser_test.go:`TestParseSource_TurnGrouping` |
| R-06 | AC-10 | D-06 | C-05 | parser.go:`parseSource` | parser_test.go:`TestParseSource_FingerprintError` |
| R-07 | AC-11 | D-06 | C-05 | parser.go:`parseSource` | parser_test.go:`TestParseSource_WriteError` |
| R-08 | AC-12 | D-05 | C-06 | parser.go:`parseSource` (ct.UserMessage="") | parser_test.go:`TestParseSource_UserMessageZeroed` |
| R-09 | AC-13 | D-03 | — | parser.go:`parseSource` | parser_test.go:`TestParseSource_ZeroAPICalls` |
| R-10 | AC-14 | D-04 | — | parser.go:`parseSource` | parser_test.go:`TestParseSource_DateFilterAfterCache` |
| R-11 | AC-15 | D-08 | — | claude.go:`ParseSession` | claude_test.go:`TestGlobalDedupAcrossFiles` |
| NF-01 | AC-04 | D-03 | C-02 | cache.go:`GetFileFingerprint` | parser_test.go:`TestParseSource_CacheHit_Claude` |
| NF-02 | — | D-06 | C-07 | cache.go:`initSchema` (busy_timeout=3000) | — |
| NF-03 | AC-04, AC-14 | D-04 | C-07 | parser.go:`filterSessionByDateRange` | parser_test.go:`TestParseSource_DateFilterAfterCache` |

## Coverage Gaps

- **NF-02** has no direct behavioral AC. Latency is bounded by the existing `busy_timeout=3000ms`
  pragma (cache.go:73); verified structurally rather than by a timed test.
- **AC-12** (round-trip UserMessage zeroing) covers both the write and read path for R-08, making
  a separate read-back AC redundant.
