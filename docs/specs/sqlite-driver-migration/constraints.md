# Constraints: SQLite Driver Migration

| ID | Constraint | Type | Source |
|----|-----------|------|--------|
| C-01 | Both `mattn/go-sqlite3` and `modernc.org/sqlite` must remain in `go.mod` since linux builds use the fallback. | Technical | D-02 |
| C-02 | The driver name constant must be the single source of truth for `sql.Open` calls. No hardcoded `"sqlite"` or `"sqlite3"` strings in consumer code. | Technical | D-01 |
| C-03 | The privacy invariant (UserMessage zeroing before cache write) is unaffected by this change but must continue to pass under both drivers. See `docs/specs/go-migration/design-decisions.md` (D-08: Privacy Invariant as Explicit Zero Pass). | Behavioral | go-migration design-decisions.md D-08 |
| C-04 | `json_extract` in Cursor queries must work under both drivers. mattn/go-sqlite3 includes JSON1 by default. modernc.org/sqlite also includes JSON1. | Technical | Cursor provider |
| C-05 | goreleaser darwin builds require a C compiler in the CI environment. The release workflow must install one. | CI/Infra | D-02 |
| C-06 | `go test ./...` with CGO_ENABLED=0 must pass all tests using modernc. `go test ./...` with CGO_ENABLED=1 must pass all tests using mattn. | Testing | NF-03 |
| C-07 | On macOS, `go build` and `go test` default to CGO_ENABLED=1 when a C compiler is available. The build-tag wiring must produce correct behavior with this default. | Technical | Go toolchain |
