# Traceability Matrix: SQLite Driver Migration

| Requirement | Acceptance Criteria | Design Decisions | Constraints |
|-------------|--------------------|--------------------|-------------|
| R-01 | AC-01 | D-01, D-02 | C-07 |
| R-02 | AC-02 | D-01, D-02 | C-01 |
| R-03 | AC-03 | D-01 | C-02 |
| R-04 | AC-04 | D-01 | C-02 |
| R-05 | AC-05, AC-06 | D-02 | C-05 |
| R-06 | AC-07 | D-02 | C-05 |
| R-07 | AC-08, AC-09 | -- | C-03, C-04, C-06 |
| R-08 | AC-10 | D-02 | -- |
| NF-01 | AC-11 | D-02 | -- |
| NF-02 | AC-06 | D-02 | C-01 |
| NF-03 | AC-12 | D-01 | C-06 |

## Files Affected (D-01)

| Action | File | Notes |
|--------|------|-------|
| Create | `internal/sqlitedrv/driver_cgo.go` | `//go:build cgo`; imports mattn; exports `DriverName = "sqlite3"` |
| Create | `internal/sqlitedrv/driver_nocgo.go` | `//go:build !cgo`; imports modernc; exports `DriverName = "sqlite"` |
| Create | `internal/sqlitedrv/doc.go` | Package doc comment |
| Create | `internal/sqlitedrv/driver_bench_test.go` | R-08 benchmark: INSERT+SELECT session cache workload |
| Modify | `internal/parser/cache.go` | Replace driver import + hardcoded string with `sqlitedrv.DriverName` |
| Modify | `internal/provider/cursor/cursor.go` | Replace driver import + hardcoded string with `sqlitedrv.DriverName` |
| Modify | `internal/provider/cursor/cursor_test.go` | Replace driver import + hardcoded strings with `sqlitedrv.DriverName` |
| Modify | `.goreleaser.yaml` | Split into codeburn-darwin (CGO=1, zig cc) and codeburn-linux (CGO=0) stanzas |
| Modify | `.github/workflows/release.yml` | Add setup-zig step; remove CGO_ENABLED=0 override |

## Implementation Results

| Requirement | Status | Evidence |
|-------------|--------|---------|
| R-01, R-02, R-03 | PASS | `CGO_ENABLED=1/0 go build ./...` both succeed |
| R-04 | PASS | All `sql.Open` calls use `sqlitedrv.DriverName` constant |
| R-05 | PASS | `.goreleaser.yaml` has two stanzas with correct CGO env per OS |
| R-06 | PASS | `release.yml` installs zig 0.13.0 before goreleaser |
| R-07 | PASS | All tests pass under both drivers |
| R-08 | PASS | mattn 47,360 ns/op vs modernc 84,270 ns/op (1.78x faster) |
| NF-02 | PASS | Linux stanza retains `CGO_ENABLED=0` (static binary) |
| NF-03 | PASS | `CGO_ENABLED=0 go test ./...` all green; `CGO_ENABLED=1 go test ./...` all green |

## Coverage Gaps

None. All R-ids have at least one AC-id. All AC-ids trace to at least one R-id.
