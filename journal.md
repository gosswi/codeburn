# SDLC Toolkit Journal

Tracks every sdlc-toolkit interaction: value delivered, limitations found, self-corrections observed.

---

### 2026-04-15 -- security-scan (Claude Cowork, Opus 4.6)

**Action**: Full security audit of the codebase. Produced `docs/specs/security-audit-report.md`.
**Value**: Confirmed zero user data leakage. Identified 2 outbound read-only HTTP requests (LiteLLM pricing, Frankfurter exchange rates) and verified neither transmits user data. Flagged 5 low-risk items (symlink following, JSONL prototype pollution, export path traversal) with clear reasoning for why each is acceptable.
**Limitations**: None.
**Self-correction**: N/A.

---

### 2026-04-15 -- learn-codebase (Claude Cowork, Opus 4.6)

**Action**: Generated `docs/specs/codebase-overview.md` -- full module map, architecture description, provider system analysis, caching strategy, and test coverage gaps.
**Value**: Mapped all 18 source files with line counts and roles. Identified the key architectural asymmetry: Claude sessions use a legacy JSONL codepath while Codex/Cursor use the async generator `SessionParser` interface. Documented deduplication strategies per provider.
**Limitations**: None.
**Self-correction**: N/A.

---

### 2026-04-15 -- performance-analysis

**Action**: Profiled CLI startup with CPU profiling and filesystem instrumentation. Produced `docs/specs/performance-analysis.md`.
**Value**: Identified 4 ranked bottlenecks with exact self-times: bash regex (171ms/21% CPU), JSON.parse all lines (138ms/17%), redundant discovery (37ms x N), overlapping date range re-parse (200-400ms). Set concrete targets: `status --format json` under 500ms, `report` under 2s.
**Limitations**: None.
**Self-correction**: N/A.

---

### 2026-04-15 -- full audit (Claude Cowork, Opus 4.6)

**Action**: Comprehensive project audit covering architecture, implementation, security, testing, strengths, and weaknesses. Produced `docs/specs/codeburn-full-audit.md` (415 lines).
**Value**: Rated 8 areas (architecture: Strong, privacy: Excellent, test coverage: Weak). Identified 8 specific weak points with actionable recommendations. Produced a complete architecture diagram in Mermaid and a module-level responsibility map.
**Limitations**: Could not run `npm audit` due to sandbox network restrictions.
**Self-correction**: N/A.

---

### 2026-04-16 14:00 -- architect

**Action**: Evaluated whether CodeBurn should be rewritten in Rust, Go, Python, or Swift instead of TypeScript.
**Value**: Produced a structured comparison across 10 dimensions (startup, memory, TUI ecosystem, SQLite, migration cost, etc.). Concluded: don't rewrite -- 4 targeted TypeScript fixes (~100-200 lines) can achieve 80% of the performance gains. Identified Go as the strongest rewrite candidate if TypeScript optimizations fail.
**Limitations**: Analysis was thorough but long. The agent produced a very detailed response that could have been more concise for decision-making.
**Self-correction**: N/A.

---

### 2026-04-16 15:00 -- spec-writer

**Action**: Wrote `docs/specs/performance-optimization.md` (v1.0.0) -- a 580-line spec defining 6 tasks (T1-T6) to fix all identified performance bottlenecks.
**Value**: Produced production-grade spec with 8 requirements, 15 acceptance criteria, 5 design decisions, risk analysis, and traceability matrix. Each task is independently shippable with its own branch, testing strategy, and invariants. Defensive design lens caught Risk-1 (filterProjectsByDateRange invariant) proactively.
**Limitations**: None apparent at writing time -- issues found later by verify-spec.
**Self-correction**: N/A.

---

### 2026-04-16 16:00 -- verify-spec

**Action**: Structural and alignment verification of performance-optimization.md against the actual codebase.
**Value**: Caught 2 errors and 3 warnings that would have caused bugs during implementation. Key find: T6 set `extractBash: false` for the `export` command, but `export.ts:buildBashRows()` reads `bashBreakdown` -- this would have silently emptied bash data in CSV/JSON exports. Also caught AC-6b contradicting T6 Step 4 for terminal format.
**Limitations**: Pre-implementation verification only (Level 1-2 depth). Cannot verify code alignment since no code exists yet.
**Self-correction**: verify-spec caught spec-writer's mistakes. The spec-writer did not check whether `export.ts` uses `bashBreakdown` before setting `extractBash: false` for exports.

---

### 2026-04-16 16:30 -- spec fixes (follow-up to verify-spec)

**Action**: Applied all 6 findings from verify-spec to the spec. Updated to v1.1.0.
**Value**: Fixed 2 errors (extractBash for export, AC-6b contradiction), 3 warnings (missing ACs for currency loading, ProjectSummary recompute, clearDiscoveryCache), and 1 info (added widen-then-filter for `status --format json`). Traceability matrix and changelog updated.
**Limitations**: None.
**Self-correction**: This is the self-correction cycle: spec-writer produced a spec with latent bugs -> verify-spec found them -> fixes applied. The plugin caught its own output within one iteration.

---
