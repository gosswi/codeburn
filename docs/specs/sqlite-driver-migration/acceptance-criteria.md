# Acceptance Criteria: SQLite Driver Migration

## AC-01 -- mattn driver active on darwin CGO build

**Given** CGO_ENABLED=1 (darwin default)
**When** `go build ./cmd/codeburn` is run on darwin
**Then** the resulting binary uses `mattn/go-sqlite3` (driver name `"sqlite3"`)

**Traces**: R-01, R-04

## AC-02 -- modernc fallback active on CGO-disabled build

**Given** CGO_ENABLED=0 (linux default or explicit override)
**When** `go build ./cmd/codeburn` is run
**Then** the resulting binary uses `modernc.org/sqlite` (driver name `"sqlite"`)

**Traces**: R-02, R-04

## AC-03 -- no dual-driver registration

**Given** any valid build configuration
**When** the binary starts
**Then** exactly one SQLite driver is registered in `database/sql`; no panic from duplicate registration

**Traces**: R-03

## AC-04 -- driver name consistency across files

**Given** the codebase after migration
**When** grepping for `sql.Open` calls in all `.go` files
**Then** every call uses a driver name variable or constant defined in the build-tagged driver file, not a hardcoded string

**Traces**: R-04

## AC-05 -- goreleaser produces CGO darwin binaries

**Given** the updated `.goreleaser.yaml`
**When** goreleaser runs the darwin build
**Then** `CGO_ENABLED=1` is set and the binary links against the mattn SQLite amalgamation

**Traces**: R-05

## AC-06 -- goreleaser produces static linux binaries

**Given** the updated `.goreleaser.yaml`
**When** goreleaser runs the linux build
**Then** `CGO_ENABLED=0` is set and the binary is statically linked

**Traces**: R-05, NF-02

## AC-07 -- CI workflow supports CGO cross-compilation

**Given** the updated `.github/workflows/release.yml`
**When** the release workflow runs on ubuntu-latest
**Then** a C cross-compiler for darwin targets is available and the darwin build succeeds

**Traces**: R-06

## AC-08 -- session cache round-trip under mattn

**Given** a build using mattn/go-sqlite3
**When** `TestCacheHit`, `TestCacheMissOnMtimeChange`, `TestCacheMissOnSizeChange`, `TestCacheCorruptRecovery`, `TestCacheConcurrentReads`, `TestCachePrivacyInvariant` are run
**Then** all pass

**Traces**: R-07

## AC-09 -- Cursor provider queries under mattn

**Given** a build using mattn/go-sqlite3
**When** `TestBasicBubbleParsing`, `Test35DayLookback`, `TestDedupKey`, `TestLanguageExtraction`, `TestCursorFileCache` are run
**Then** all pass (json_extract queries work identically)

**Traces**: R-07

## AC-10 -- benchmark proves mattn is faster

**Given** a Go benchmark test in `internal/parser/` or a dedicated `benchmarks/` package
**When** `go test -bench=BenchmarkSQLiteDriver -count=5` is run with CGO_ENABLED=1
**Then** the mattn-backed operations show lower ns/op than a comparable modernc baseline, with the result documented in the PR

**Traces**: R-08

## AC-11 -- darwin binary size delta within limit

**Given** a darwin binary built with `CGO_ENABLED=1` (mattn/go-sqlite3)
**When** its file size is compared against the modernc baseline binary
**Then** the delta is less than 2MB

**Traces**: NF-01

## AC-12 -- all tests pass under CGO_ENABLED=0

**Given** `CGO_ENABLED=0` (modernc.org/sqlite active via build tag)
**When** the test functions named in AC-08 and AC-09 are run
**Then** all pass without modification

**Traces**: NF-03
