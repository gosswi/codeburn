# SDLC Toolkit Review

An honest review of the sdlc-toolkit plugin based on 17 real interactions across 3 days (April 15-17, 2026) while optimizing the CodeBurn CLI. This is not a synthetic evaluation -- every observation comes from shipping real code to a real repository.

---

## Context

CodeBurn is a ~2500-line TypeScript CLI tool. The work involved two performance optimization phases: Phase 1 (streaming, caching, lazy extraction) and Phase 2 (SQLite session cache). The toolkit was used for the entire lifecycle: audit, analysis, spec writing, verification, task generation, implementation, code review, and PR creation.

Total skills invoked: 17
Total artifacts produced: 12 specs/reports, 2 PRs, ~4000 lines of implementation + tests

---

## What Worked Well

### 1. The verify-spec / spec-writer feedback loop

This is the toolkit's strongest feature. In Phase 1, the spec-writer produced a 580-line spec that looked complete. verify-spec then found 2 errors and 3 warnings that would have caused bugs during implementation:

- T6 set `extractBash: false` for the `export` command, but `export.ts:buildBashRows()` reads `bashBreakdown` -- this would have silently emptied bash data in CSV/JSON exports
- AC-6b contradicted T6 Step 4 for terminal format

These are exactly the kind of cross-cutting errors that humans miss in spec review. The spec-writer didn't check downstream consumers of the flag it was setting. verify-spec did. One iteration caught everything.

### 2. Multi-agent spec generation (generate-spec)

Phase 2 used the full multi-agent pipeline: researcher + 2 parallel writers (simplicity lens vs defensive lens) + codebase-blind evaluator + consolidator. The divergence between writers was genuinely useful:

- Writer A (simplicity) had zero corruption handling, zero concurrent access protection, and no schema upgrade path
- Writer B (defensive) caught all three but deviated from interview decisions (warmup-2/runs-10 instead of the agreed median-of-3)
- The evaluator caught 6 gaps both writers missed (cache observability, size bounds, directory creation, build-failure policy)

This is not a gimmick -- the defensive writer's output was materially better (72/90 vs 57/90), and the evaluator caught the defensive writer's own deviations. Real multi-perspective value.

### 3. Code review catching implementation bugs

The code-review skill found H1: menubar provider cost over-counts when a project has mixed providers. The Phase 1 rewrite from per-provider `parseAllSessions` to `filterProjectsByDateRange` broke provider cost attribution by attributing full project cost to each provider. This was a real correctness bug that tests didn't catch because the test fixtures only had single-provider projects. It was fixed before merge.

### 4. generate-tasks producing actionable parallelism

Both Phase 1 and Phase 2 task breakdowns identified real serialization constraints (T3b and T6b both touch `src/cli.ts`; `src/parser.ts` as the Phase 2 bottleneck for T3/T4/T5). The file conflict matrix is not decorative -- it directly determined which tasks could be parallelized and which had to be sequential. Max parallel agents: 3, with concrete reasoning.

### 5. performance-analysis with concrete numbers

Both pre- and post-optimization analyses produced specific self-times, not vague assessments. "bash regex: 171ms / 21% CPU" is actionable. "JSON.parse: 138ms / 17%" is actionable. "It's slow" is not. The post-optimization analysis honestly reported two unmet targets (json still 589ms vs 400ms, RSS 185MB vs 100MB) and explained the floor.

---

## What Didn't Work

### 1. Consolidator dropping output files

The generate-spec consolidator missed 2 of 6 output files (constraints.md, traceability.md). These had to be written manually. For a multi-agent pipeline that costs ~170K tokens, dropping deliverables is a significant reliability gap. The pipeline should validate its own output completeness.

### 2. High token cost for multi-agent spec generation

5 agents, ~170K tokens for one spec. The consolidated spec scored 63/100 on first pass and required a full evaluation + fix cycle. The question is whether this cost is justified vs a single careful writer + one verification pass. For Phase 1, a single spec-writer + verify-spec produced a solid result at a fraction of the cost. The multi-agent approach caught more edge cases but the ROI is unclear for Feature-tier work.

### 3. Architect skill producing overly verbose output

The language rewrite evaluation (TypeScript vs Rust/Go/Python/Swift) was thorough but too long for the decision it supported. The answer was "don't rewrite, do 4 targeted fixes" -- this could have been a 1-page analysis instead of a detailed comparison across 10 dimensions. The skill doesn't adapt its output length to the decisiveness of the conclusion.

### 4. Implementation skill missing a critical design flaw

During Phase 2 implementation, the implementer guarded cache writes with `if (!dateRange)`, which sounded correct (don't cache partial results) but made the performance target unreachable -- `status` always passes `dateRange`, so cache hits were always 0. This was caught during T10 verification (manual DEBUG logging), not by the implementation skill itself. The skill should have verified its performance claims against the spec targets before marking the task complete.

### 5. create-pr went for original project instead of this fork everytime

The /create-pr skill always created the PR against the original repository. Although this could be right in many cases, in this particular session Claude was mandated to use the fork main branch as base. This issue could be troubling if the PR against the original repo was approved and megered (improbable yet not impossible). A more clear ruleset for PR creation would have avoided this: a default way suggestion always origin for forked repos, or explicit questions regarding where to point the PR.

### 6. create-pr not detecting existing PRs

In this session, the create-pr skill generated a `gh pr create` command for a branch that already had an open PR (#72). It should check `gh pr list --head <branch>` first and offer `gh pr edit` instead. Minor, but avoidable.

### 7. Sandbox limitations blocking npm audit

The full audit skill could not run `npm audit` due to sandbox network restrictions. This is an environment constraint, not a toolkit bug, but it's worth noting that security audit skills that can't reach package registries have a blind spot.

---

## Quantified Results

| Metric | Before toolkit | After toolkit |
|---|---|---|
| `status --format json` | 960ms | 310ms (3.1x) |
| `status --format menubar` | 1520ms | 330ms (4.6x) |
| `status --format terminal` | 710ms | 310ms (2.3x) |
| Test count | 40 | 74 |
| Bugs caught pre-merge | 0 (no review) | 3 (H1 + 2 spec errors) |
| Specs produced | 0 | 2 complete specs |

---

## Self-Correction Effectiveness

The toolkit's most valuable property is its self-correction loop. Across all interactions:

| Stage | Caught by | What was caught |
|---|---|---|
| Spec writing | verify-spec | extractBash flag breaking exports, AC contradiction |
| Spec consolidation | spec-evaluator | AC20 traceability misassignment, R5 with zero ACs, untestable AC16 |
| Implementation | code-review | Provider cost over-counting (H1), duplicate function call (M2) |
| Implementation | manual T10 | Cache guard making warm cache unreachable |

3 out of 4 catches were automated by the toolkit pipeline. The 4th (cache guard) required manual debugging. The pipeline is not a substitute for running the code, but it catches a meaningful class of errors that happen between spec and implementation.

---

## Recommendations

### For the toolkit developers

1. **Consolidator output validation**: The multi-agent pipeline should verify all expected output files exist before reporting success. A simple checklist pass at the end would have caught the missing constraints.md and traceability.md.

2. **Cost-aware tier routing**: generate-spec's 5-agent pipeline is overkill for Feature-tier work. Consider a lighter 2-agent path (1 writer + 1 evaluator) for Feature tier, reserving the full pipeline for Epic tier. The Phase 1 single-writer approach was nearly as effective at a fraction of the cost.

3. **Implementation verification against spec targets**: When a spec has quantitative acceptance criteria (e.g., "under 400ms"), the implement skill should verify those targets after implementation, not just check that tests pass. The `!dateRange` cache guard bug would have been caught immediately.

4. **Adaptive verbosity for architect**: If the conclusion is decisive ("don't rewrite"), the output should be short. The 10-dimension comparison matrix is useful when the decision is close; it's noise when it's not.

5. **create-pr should have more guidelines**: A more robust default guideline or explicitly asking the user how to write and where to point the PR could be really useful to avoid ambiguities.

6. **create-pr should check for existing PRs**: A simple `gh pr list --head <branch>` before generating the create command would avoid duplicate PR attempts and offer edit instead.

7. **Benchmark / performance-analysis integration**: The performance-analysis skill produces before/after numbers, and the benchmark script produces detailed comparisons. These could be a single workflow where the toolkit runs the benchmark and produces the analysis in one pass.

### For users of the toolkit

1. **Always run verify-spec after spec-writer**. The spec-writer produces plausible output that looks complete but may have cross-cutting errors. verify-spec is cheap and caught real bugs every time it was used.

2. **Don't skip code-review before merge**. The H1 bug (provider cost over-counting) was not caught by tests, not caught during implementation, and not visible without running the specific format that triggered it. code-review found it from static analysis alone.

3. **Run your code after implementation**. The toolkit's implementation skill can produce code that passes all tests but misses performance targets. Manual verification (especially `DEBUG=1` logging) caught the cache guard bug that automated testing missed.

4. **The multi-agent spec pipeline is worth it for complex features**. The divergence between writers is real, not theatrical. But for straightforward features, a single spec-writer + verify-spec is sufficient and much cheaper.

5. **Journal everything**. The journal format (Action / Value / Limitations / Self-correction) forces honest accounting. Without it, you lose track of what the toolkit actually contributed vs what you did manually.

---

## Verdict

The sdlc-toolkit is a genuine productivity multiplier for spec-driven development. Its strongest contribution is the self-correction loop: spec-writer makes mistakes, verify-spec catches them, code-review catches implementation bugs. This loop caught 3 bugs that would have shipped without it.

Its weakest areas are cost efficiency (the multi-agent pipeline is expensive for mid-complexity work) and implementation verification (it checks that code compiles and tests pass, but doesn't verify quantitative spec targets). The consolidator reliability issue is fixable.

For a solo developer working on a mid-size TypeScript project, the toolkit turned a vague "make it faster" goal into two structured optimization phases with specs, traceability, and verified results. The total performance gain (2.3x-4.6x across commands) is real and measured. Whether the toolkit was necessary to achieve that gain is debatable -- but it made the process structured, auditable, and less error-prone.
