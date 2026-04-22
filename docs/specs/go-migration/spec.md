# Spec: CodeBurn Go Migration

**Spec ID**: go-migration-v1
**Status**: Draft
**Version**: 1.2
**Date**: 2026-04-20
**Tier**: 3 (Epic)
**Branch convention**: `feature/go-migration`
**Perspective seed**: Prioritize edge cases, failure modes, and defensive design

---

## Objective

Rewrite the CodeBurn CLI from TypeScript to Go to eliminate five structural limitations:

- Node.js startup overhead (~80-150ms) dominates menubar invocations (every 5 minutes)
- Sequential session file parsing (single-threaded async loop across hundreds of files)
- React/Ink loaded on every invocation including non-interactive ones (~30-40MB RSS)
- `better-sqlite3` native addon breaks on ARM/x86 cross-distribution and requires Node.js
- Installation requires a Node.js runtime; Go produces a single static binary

---

## Out of Scope

- Changing output formats, behavior, or user-visible semantics
- Rewriting the macOS menubar app (SwiftBar/xbar plugin remains a shell script)
- Adding new features during migration

---

## Changelog

| Version | Date | Author | Change |
| --- | --- | --- | --- |
| 1.0 | 2026-04-20 | spec-writer | Initial production specification |
| 1.1 | 2026-04-20 | evolve-spec | Added 22 ACs (AC21-AC42) for previously uncovered requirements. Added R46-R51 (observability, --output flag, --provider flag, exit codes, config error handling, Phase 3 RSS re-check). Defined extractBash in R25. Relaxed AC2 from byte-identical to semantically equivalent. Added RGB tolerance to AC15. Fixed RISK-3 AC41 reference. Linked RISK-4 and RISK-9 to new requirements. |
| 1.2 | 2026-04-20 | evolve-spec | Added 13 ACs (AC43-AC55) for all previously uncovered requirements. Added R52 (Cursor 35-day lookback) and R53 (fast-mode pricing). Fixed RISK-3 stale cross-reference (AC34 -> AC47). Reassigned AC10 from R12 to R52. Updated traceability matrix: all 53 requirements now have AC coverage. Added fast-mode unit test to testing strategy. |
| 1.2.1 | 2026-04-20 | restructure | Split monolithic spec into folder structure matching sdlc-toolkit convention. No content changes. |
