# SDLC Toolkit Review: Go Migration (2026-04-22)

*Based on 20+ journal entries covering the full migration of codeburn from TypeScript to Go*

---

## Pros

**Structured pipeline discipline.** The skill sequence (architect -> spec-researcher -> spec-writer x N -> spec-evaluator -> spec-consolidator -> generate-tasks -> implement -> code-review) forces you to think before you build. On a migration of this scope - parser, formatter, provider system, SQLite cache, TUI, benchmark harness - having a written spec before touching a single file prevented at least two architecture rework cycles. The "no code before spec approval" gate is genuinely valuable.

**Spec consolidation quality.** The multi-writer + evaluator pattern produced noticeably better specs than any single pass. When three spec-writers approached the session cache design from different angles, the consolidator synthesized the strongest elements from each. The final spec for the per-file SQLite cache was precise enough that the implementation had no structural surprises.

**Research grounding.** spec-researcher reads the actual codebase before writing anything. This meant the Go specs correctly identified the TypeScript deduplication strategies (API message ID for Claude, cumulative token cross-check for Codex, conversation+timestamp for Cursor) rather than inventing a generic approach. The specs matched the existing behavior rather than describing a clean-room reimagining.

**Incremental verification.** verify-spec catching logical gaps before implement ran saved at least one implementation cycle on the formatter. The spec had claimed all three output formats (json, menubar, terminal) shared a single render path - verify-spec flagged that menubar has different truncation rules than terminal. That was a real bug caught at spec time.

**Code review catch rate.** The code-review skill found the `Array.isArray(summary.turns)` vulnerability class (wrong assumption about cache entry format across writers) before it hit production. It also flagged the missing error handling on `session-cache.db` schema migrations, which would have silently corrupted data on upgrades.

---

## Cons

**Token cost is steep.** A full generate-spec cycle (researcher + 3 writers + evaluator + consolidator) on a mid-complexity feature costs more context than a senior engineer reviewing a PR. For a one-person project, this ratio is hard to justify for every feature. The toolkit provides no lightweight mode.

**Spec quality is only as good as the researcher.** spec-researcher does a surface read - it finds files and reads key functions, but it doesn't trace execution paths deeply. The session cache spec initially missed that LiteLLM pricing is fetched over HTTP and cached separately - the researcher found the cache directory but didn't connect pricing fetch to the same cache mechanism. That gap required an evolve-spec cycle.

**generate-tasks granularity is inconsistent.** On the parser implementation, tasks were at the right level (one task per provider). On the formatter, tasks were too coarse ("implement all output formats" as one task). There's no way to control granularity via the skill invocation - you get what the skill decides.

**Implement doesn't verify against spec.** The implement skill executes the tasks it's given, but it doesn't check back against the original spec document during execution. When the benchmark harness implementation diverged from the spec (spec said 5 runs, implementation did 3), there was no automatic detection. The divergence was only caught during manual testing.

**No rollback or checkpoint mechanism.** If an implement run fails partway through a multi-task sequence, there's no built-in state to resume from. You get partial code changes with no clear indication of what completed and what didn't. On longer implement sessions, this meant manually inspecting git diff to figure out where things stopped.

---

## Challenges

**Spec drift during evolve-spec.** The migration required evolve-spec twice - once when the Go SQLite driver changed (modernc to mattn/CGO) and once when the benchmark scope expanded to cold/warm conditions. Each evolve-spec cycle produced a revised spec, but the old spec remained in the docs directory. By the end, there were three versions of the session-cache spec with no clear canonical pointer. The toolkit has no concept of spec versioning or deprecation.

**Cross-skill context loss.** Each skill invocation is stateless relative to previous skill invocations. The implement skill doesn't inherently know what the spec-evaluator said about the architecture. This means the evaluator's warnings (e.g., "the Go binary should not write PascalCase JSON to a shared cache that TS reads as camelCase") had to be manually carried into the implementation tasks. If you didn't transcribe the warning into a task, it was lost.

**Parallelism is under-specified.** The migration had multiple providers (Claude, Codex, Cursor) that could have been implemented in parallel. The toolkit's generate-tasks skill creates a sequential task list by default. There's no mechanism for expressing "these three tasks have no dependencies and can run concurrently." You can manually parallelize using worktrees, but the skill gives you no guidance on when that's safe.

**Spec completeness vs. pragmatics.** The spec-writer agents optimize for specification completeness, not implementation pragmatics. The Go parser spec was technically complete but didn't account for the fact that Claude JSONL files can be several hundred MB. The spec described reading entire files before parsing; the implementation had to introduce streaming - a non-trivial deviation that the spec should have mandated from the start.

**Benchmark spec lived outside the main spec tree.** The benchmark design never got a proper spec-researcher + spec-writer pass. It started as an ad-hoc shell script and grew into a multi-condition HTML report. By the end, the benchmark methodology was more complex than some features that had full spec coverage. The toolkit's pipeline works well for feature development but has no lightweight path for tooling/scripting work.

---

## Suggested Improvements

**1. Spec versioning with explicit supersession.** When evolve-spec runs, it should move the old spec to `docs/superpowers/specs/archive/` and write a `SUPERSEDED_BY` frontmatter field. The toolkit should enforce that only one spec per feature is "active" at any time.

**2. Lightweight mode for low-risk tasks.** Add a `sketch` skill that skips the multi-writer pattern and goes straight to a single spec draft with researcher + writer + user review. For tasks like "add a flag" or "add a new output format," three writers and an evaluator is overkill. The toolkit should let the user signal complexity level upfront.

**3. Implementation-time spec binding.** The implement skill should read the spec document at the start and surface any deviation as a warning. If the spec says "streaming parser" and the implementation reads the full file into memory, the skill should flag it rather than silently proceed.

**4. Task granularity controls.** generate-tasks should accept a `granularity` parameter: `coarse` (one task per major component), `medium` (one task per function), `fine` (one task per code path). The current one-size behavior produces tasks that are either too coarse to be actionable or too fine to be motivating depending on feature complexity.

**5. Cross-skill context transfer.** The spec-evaluator's warnings should be automatically materialized as tagged tasks in generate-tasks. If the evaluator says "risk: cache format incompatibility," generate-tasks should produce a task "verify cache format compatibility between Go and TS writers" without requiring manual transcription.

**6. Dependency graph in task output.** generate-tasks should express inter-task dependencies explicitly (e.g., as a simple adjacency list or a Mermaid diagram) so that the user can identify safe parallelism opportunities without having to reason about it from scratch.

**7. Benchmark/tooling skill.** Add a lightweight `tooling-spec` skill for scripts, benchmarks, and test harnesses - something that runs researcher + single writer + user review, with a template optimized for "inputs, methodology, outputs" rather than "architecture, components, data flow." Tooling work is a real part of any project and the current pipeline is mismatched for it.

---

## Summary

The sdlc-toolkit delivered real value on this migration: the specs were grounded in actual code, the review cycles caught real bugs, and the structured pipeline prevented the drift that typically happens when a migration is done incrementally without a written plan. The main friction points are statelessness across skill invocations, lack of spec lifecycle management, and the absence of a lightweight mode for lower-stakes work. These are fixable problems - the core pipeline design is sound.

---
