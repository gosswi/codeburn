# Evolution Diff: CodeBurn Go Migration Spec

## v1.0 -> v1.1 (2026-04-20)

**Classification**: Structural
**Trigger**: Codebase-blind spec evaluation at `docs/specs/go-migration-spec-evaluation.md` identified gaps. Source-verified findings applied; incorrect findings (R2 schema, R30 debounce, R23 tool sets, R5 terminal format) dismissed after codebase verification.

### Evaluation Findings Dismissed (Spec Was Correct)

| Finding | Evaluator Claim | Source Verification |
|---------|----------------|---------------------|
| R2 schema wrong | Research doc shows 4 periods with `apiCalls`/`sessions` | `cli.ts:151-155` outputs `{ currency, today: {cost, calls}, month: {cost, calls} }` - R2 is correct |
| R30 debounce 600ms vs 100ms | Research doc says 100ms | `dashboard.tsx:580` shows `600` - R30 is correct |
| R23 tool set count 5 vs 6 | Research doc says 6 | `classifier.ts:18-22` defines exactly 5 sets - R23 is correct |
| R5 only Today/Month | Research doc shows 4 periods | `format.ts:37` renders only Today and Month - R5 is correct |

### Requirements Added

| R-id | Summary | Rationale |
|------|---------|-----------|
| R46 | `CODEBURN_DEBUG=1` diagnostic output to stderr | 8+ silent failure modes (R38-R45) with no diagnostic path; RISK-4 and RISK-9 referenced undefined debug mechanism |
| R47 | `export --output <path>` flag | Flag exists in TypeScript CLI (`-o, --output`) but was unspecified |
| R48 | `--provider` flag for status/export | Flag exists in TypeScript CLI but only specified for TUI (R32) |
| R49 | Exit code contract (0 success, 1 user errors) | Only one exit code was documented (currency command); Go CLI needs explicit contract |
| R50 | Invalid config.json treated as missing | No error handling for corrupt config file |
| R51 | Phase 3 RSS re-verification for R37 | R37 assigned to P2 but TUI deps in P3 could regress RSS |

### Acceptance Criteria Added (AC21-AC42)

| AC-id | Parent R-id | Summary |
|-------|-------------|---------|
| AC21 | R4 | JSON export structural match |
| AC22 | R5 | Terminal format layout verification |
| AC23 | R6 | Menubar pipe format with 17 currencies |
| AC24 | R7 | install-menubar on macOS |
| AC25 | R7 | install-menubar on non-macOS |
| AC26 | R11 | Codex session_meta validation |
| AC27 | R18 | Cursor cache fingerprint invalidation |
| AC28 | R20 | LiteLLM disk cache fallback |
| AC29 | R21 | FALLBACK_PRICING exact match |
| AC30 | R22 | Exchange rate defaults to 1 on failure |
| AC31 | R24 | shortProject home dir stripping |
| AC32 | R25 | In-process cache reuse within 60s |
| AC33 | R29 | Keyboard shortcuts functional |
| AC34 | R30 | Debounce coalesces rapid period switches |
| AC35 | R31 | Auto-refresh fires at interval |
| AC36 | R32 | Provider detection excludes empty providers |
| AC37 | R33 | Cursor "Languages" panel |
| AC38 | R38 | Invalid JSONL line skipped, rest parsed |
| AC39 | R39 | Unreadable dir skipped silently |
| AC40 | R42 | Cursor always attempted (no build tag) |
| AC41 | R43 | Cache write failure swallowed |
| AC42 | R45 | SwiftBar takes priority over xbar |

### Requirements Modified

| R-id | Change | Before | After |
|------|--------|--------|-------|
| R25 | Clarified | `extractBash` referenced without definition | Added definition: boolean ParseOptions flag controlling bash command text extraction |

### Acceptance Criteria Modified

| AC-id | Change | Before | After |
|-------|--------|--------|-------|
| AC2 | Relaxed | "byte-identical" | "semantically equivalent" with 2 decimal place tolerance, field order not required |
| AC15 | Tightened | "approximately #F55B5B" | "#F55B5B (tolerance: +/-2 per RGB channel)" |

### Risk Register Fixed

| Risk | Change |
|------|--------|
| RISK-3 | Fixed reference from nonexistent "AC41" to "R41, AC34" |
| RISK-4 | Linked undefined "DEBUG env var" to R46 (`CODEBURN_DEBUG=1`) |
| RISK-9 | Linked to R51 (Phase 3 RSS re-verification) |

### Coverage

| Metric | Before | After |
|--------|--------|-------|
| Requirements | 45 | 51 |
| Requirements with ACs | 24 (53%) | 51 (100%) |
| Acceptance Criteria | 20 | 42 |

---

## v1.1 -> v1.2 (2026-04-20)

**Classification**: Clarification
**Trigger**: verify-spec found 3 ERRORs, 8 WARNINGs in v1.1. The v1.1 evolution expanded AC coverage but left gaps for newly added requirements and introduced a stale RISK-3 cross-reference.

### Requirements Added

| R-id | Summary | Rationale |
|------|---------|-----------|
| R52 | Cursor 35-day lookback window | AC10 tested this behavior but no requirement defined it. Sourced from `cursor.ts:138` (`DEFAULT_LOOKBACK_DAYS = 35`) |
| R53 | Fast-mode cost multiplier (`fastMultiplier`) | `calculateCost` in `models.ts:141-162` supports `speed: fast` with per-model multiplier, but no requirement or AC covered it. `claude-opus-4-6` has `fastMultiplier: 6` |

### Acceptance Criteria Added (AC43-AC55)

| AC-id | Parent R-id | Summary |
|-------|-------------|---------|
| AC43 | R26 | TUI 8 panels present with correct PANEL_COLORS; mcp/bash hidden when empty |
| AC44 | R27 | Single-column at <90 cols, two-column at >=90, barWidth clamped 6-10 |
| AC45 | R34 | Cold parse of 5 bench fixtures <50ms via `go test -bench` |
| AC46 | R37 | RSS under 30MB for `status --format json` |
| AC47 | R41 | Cursor schema warning to stderr, skip data, continue with other providers |
| AC48 | R46 | Debug output to stderr: discovery counts, cache stats, pricing source, skipped files |
| AC49 | R47 | Export writes to file with `-o`, exit 1 on write failure |
| AC50 | R48 | `--provider claude` filters to Claude-only data |
| AC51 | R49 | Exit 0 for success, exit 1 for user errors, no non-zero for infra failures |
| AC52 | R50 | Invalid config.json treated as missing, no crash |
| AC53 | R51 | RSS re-check after Phase 3 TUI deps, must remain <30MB |
| AC54 | R52 | Cursor entries older than 35 days excluded |
| AC55 | R53 | Fast-mode cost is `fastMultiplier` x standard cost |

### Acceptance Criteria Modified

| AC-id | Change | Before | After |
|-------|--------|--------|-------|
| AC10 | Reassigned | Requirement: R12 | Requirement: R52 (35-day lookback is now its own requirement) |

### Risk Register Fixed

| Risk | Change |
|------|--------|
| RISK-3 | Fixed stale reference: AC34 (Period Debounce) -> AC47 (Cursor Schema Warning) |

### Traceability Matrix Updated

All 53 requirements now have AC coverage. New rows added for R52 and R53.

### Testing Strategy Updated

Added fast-mode unit test row to section 7.2 table.

### Coverage

| Metric | v1.1 | v1.2 |
|--------|------|------|
| Requirements | 51 | 53 |
| Requirements with ACs | 40 (78%) | 53 (100%) |
| Acceptance Criteria | 42 | 55 |
| Stale cross-references | 1 (RISK-3) | 0 |
