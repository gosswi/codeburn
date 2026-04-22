# Spec: SQLite Driver Migration (modernc -> mattn/go-sqlite3)

**Version**: 0.1.0  **Status**: draft  **Tier**: feature
**Created**: 2026-04-21  **Last updated**: 2026-04-21

## Objective

Replace `modernc.org/sqlite` with `mattn/go-sqlite3` (CGO) as the SQLite driver on darwin to close the 1.8x performance gap identified in the Go migration benchmarks. Retain `modernc.org/sqlite` as a build-tag fallback for linux to preserve static binary distribution. Update all driver name strings, build configuration, and CI pipelines consistently.

## Out of Scope

- Homebrew formula changes (mattn/go-sqlite3 statically embeds the SQLite amalgamation)
- TUI, menubar, export, or any non-SQLite module changes
- SQLite schema changes or data migration (relies on SQLite format stability)
- No changes to Cursor query logic or deduplication -- only the driver import and `sql.Open` name in `cursor.go`
- Session cache logic changes (only the driver import and open call change)

## Dependencies

- **Implementation prerequisite**: go-migration T-06 (`internal/parser/cache.go`) must exist before this spec can be implemented
- **Amends**: go-migration T-27 (goreleaser stanzas). This spec supersedes go-migration C-02 ("no CGO") and C-05 ("modernc.org/sqlite must be used") for darwin targets. Those constraints remain binding for linux targets only.

## Changelog

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-04-21 | spec-pipeline | Initial draft |
| 0.1.1 | 2026-04-21 | spec-pipeline | Fix verify-spec errors: add AC-11/AC-12, files affected table, dependency notes, correct out-of-scope wording, expand C-03 cross-spec reference |
