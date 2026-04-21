# Tasks: SQLite Driver Migration (modernc -> mattn/go-sqlite3)

**Generated from**: docs/specs/sqlite-driver-migration/
**Total tasks**: 9
**Parallel groups**: 3
**Estimated total effort**: 13h
**Max parallel agents**: 3

## File Conflict Matrix

| Task | Files Modified | Conflicts With |
|------|---------------|----------------|
| T1 | internal/sqlitedrv/doc.go, internal/sqlitedrv/driver_cgo.go, internal/sqlitedrv/driver_nocgo.go | — |
| T2 | go.mod, go.sum | — |
| T3 | internal/parser/cache.go | — |
| T4 | internal/provider/cursor/cursor.go | — |
| T5 | internal/provider/cursor/cursor_test.go | T4 (same package) |
| T6 | internal/sqlitedrv/driver_bench_test.go | T1 (same package) |
| T7 | .goreleaser.yaml | — |
| T8 | .github/workflows/release.yml | — |
| T9 | internal/parser/cache_test.go, internal/provider/cursor/cursor_test.go | T5 (cursor_test.go) |

## Task Graph

```
T1 ──┬──→ T3 ──┐
     ├──→ T4 ──┤
     └──→ T6   ├──→ T9
T2 ──────→ T3  │
          T5 ──┘
T7 (independent)
T8 (independent, depends on T7 context)
```

Detailed dependency chain:

```
T1 ──→ T3 ──→ T9
T1 ──→ T4 ──→ T5 ──→ T9
T1 ──→ T6
T2 ──→ T3
T2 ──→ T4
T7 ──→ T8
```

## Parallel Group 1: Foundation (no dependencies)

- [P] **T1**: Create `internal/sqlitedrv` package with build-tagged driver files
  - **Validates**: R-01, R-02, R-03, R-04, C-02
  - **Files**: internal/sqlitedrv/doc.go, internal/sqlitedrv/driver_cgo.go, internal/sqlitedrv/driver_nocgo.go
  - **Complexity**: low
  - **Effort**: 1h
  - **Details**: Create three files. `driver_cgo.go` has `//go:build cgo`, imports `_ "github.com/mattn/go-sqlite3"`, exports `const DriverName = "sqlite3"`. `driver_nocgo.go` has `//go:build !cgo`, imports `_ "modernc.org/sqlite"`, exports `const DriverName = "sqlite"`. `doc.go` is the package doc comment only.

- [P] **T2**: Add `mattn/go-sqlite3` to go.mod
  - **Validates**: R-01, NF-01
  - **Files**: go.mod, go.sum
  - **Complexity**: low
  - **Effort**: 1h
  - **Details**: Run `go get github.com/mattn/go-sqlite3` (CGO_ENABLED=1 required on the dev machine). Verify both `modernc.org/sqlite` and `mattn/go-sqlite3` remain in go.mod (C-01). Run `go mod tidy` after.

- [P] **T7**: Split goreleaser build stanzas for darwin (CGO) and linux (pure-Go)
  - **Validates**: R-05, NF-02, D-02
  - **Files**: .goreleaser.yaml
  - **Complexity**: medium
  - **Effort**: 2h
  - **Details**: Replace the single `builds` entry with two stanzas: one for darwin with `CGO_ENABLED=1` and `goos: [darwin]`, one for linux with `CGO_ENABLED=0` and `goos: [linux]`. Preserve `-s -w -trimpath` flags in both. Verify the archives block names remain correct.

## Parallel Group 2: Consumer Updates (depends on T1, T2)

- [P] **T3**: Migrate `internal/parser/cache.go` to use `sqlitedrv.DriverName`
  - **Validates**: R-04, C-02, C-03
  - **Files**: internal/parser/cache.go
  - **Complexity**: low
  - **Effort**: 1h
  - **Depends on**: T1, T2
  - **Details**: Remove `_ "modernc.org/sqlite"` import. Add `"github.com/agentseal/codeburn/internal/sqlitedrv"` import. Change `sql.Open("sqlite", dbPath)` to `sql.Open(sqlitedrv.DriverName, dbPath)`.

- [P] **T4**: Migrate `internal/provider/cursor/cursor.go` to use `sqlitedrv.DriverName`
  - **Validates**: R-04, R-07, C-02, C-04
  - **Files**: internal/provider/cursor/cursor.go
  - **Complexity**: low
  - **Effort**: 1h
  - **Depends on**: T1, T2
  - **Details**: Remove `_ "modernc.org/sqlite"` import. Add `"github.com/agentseal/codeburn/internal/sqlitedrv"` import. Change `sql.Open("sqlite", source.Path)` to `sql.Open(sqlitedrv.DriverName, source.Path)`.

## Sequential: Test Updates (depends on Group 2)

- **T5**: Migrate `internal/provider/cursor/cursor_test.go` to use `sqlitedrv.DriverName`
  - **Validates**: R-07, C-06
  - **Files**: internal/provider/cursor/cursor_test.go
  - **Complexity**: low
  - **Effort**: 1h
  - **Depends on**: T4
  - **Cannot parallelize**: modifies same package as T4; merge conflict risk if run alongside T4

- **T6**: Write benchmark test comparing mattn vs modernc for session cache workload
  - **Validates**: R-08
  - **Files**: internal/sqlitedrv/driver_bench_test.go
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T1
  - **Details**: Create a benchmark that opens a DB with each driver name, runs representative CRUD ops matching the session cache workload (INSERT, SELECT with json column), and uses `testing.B`. The test file needs a build constraint that activates only when both drivers are available, or use two separate bench functions with build tags. See R-08 for the expected result (mattn faster than modernc).

## Parallel Group 3: CI Update (depends on T7, independent of code changes)

- **T8**: Update GitHub Actions release workflow to install a C toolchain for darwin cross-compilation
  - **Validates**: R-06, C-05
  - **Files**: .github/workflows/release.yml
  - **Complexity**: medium
  - **Effort**: 2h
  - **Depends on**: T7
  - **Details**: Add a step before `Run GoReleaser` to install `zig` (via apt or action) or `osxcross`. Configure the darwin build stanza in .goreleaser.yaml to use `CC: zig cc` or the osxcross wrapper. Remove the `CGO_ENABLED: 0` env override from the goreleaser action env block (the per-stanza env now controls it). The linux stanza must retain `CGO_ENABLED=0`.

## Sequential: Verification (depends on all prior tasks)

- **T9**: Verify correctness under both drivers with `go test ./...`
  - **Validates**: R-07, NF-03, C-03, C-04, C-06
  - **Files**: internal/parser/cache_test.go, internal/provider/cursor/cursor_test.go
  - **Complexity**: low
  - **Effort**: 2h
  - **Depends on**: T3, T4, T5
  - **Details**: Run `CGO_ENABLED=1 go test ./...` - must pass (uses mattn). Run `CGO_ENABLED=0 go test ./...` - must pass (uses modernc). Confirm `json_extract` queries in cursor tests pass under both (C-04). Confirm privacy invariant test in cache_test.go passes under both (C-03).
