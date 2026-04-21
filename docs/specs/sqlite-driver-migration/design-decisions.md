# Design Decisions: SQLite Driver Migration

## D-01 -- Build-tag conditional driver selection

**Status**: accepted
**Context**: Go's `database/sql` requires exactly one driver import per driver name. `mattn/go-sqlite3` registers as `"sqlite3"`, `modernc.org/sqlite` registers as `"sqlite"`. Both can coexist in a single binary without a registration panic, but including both wastes binary size and creates ambiguity about which driver is active. Build tags provide compile-time exclusion.
**Decision**: Introduce a `driver` abstraction with two build-tagged files per package that needs SQLite:
- `driver_cgo.go` (`//go:build cgo`): imports `_ "github.com/mattn/go-sqlite3"`, exports `const DriverName = "sqlite3"`
- `driver_nocgo.go` (`//go:build !cgo`): imports `_ "modernc.org/sqlite"`, exports `const DriverName = "sqlite"`
- All `sql.Open` calls use `DriverName` instead of a hardcoded string

To avoid duplicating these files in both `internal/parser/` and `internal/provider/cursor/`, place them in a shared internal package (e.g., `internal/sqlitedrv/`) imported by both consumers.
**Consequences**: Introduces one new internal package (3 files: driver_cgo.go, driver_nocgo.go, doc.go). All SQLite consumers import this package instead of importing the driver directly.
**Alternatives considered**:
- Duplicate build-tagged files in each package: works but duplicates the driver import and name constant; rejected for DRY.
- Single init() in cmd/main.go: would work but forces all tests to also wire through main; rejected because tests need the driver too.
- Runtime detection via `sql.Drivers()`: fragile, cannot control which driver is present at runtime; rejected.

## D-02 -- Option C: CGO for darwin, pure-Go fallback for linux

**Status**: accepted
**Context**: Benchmarks showed modernc.org/sqlite is 1.8x slower than the TS implementation using native better-sqlite3. mattn/go-sqlite3 (CGO) would close this gap. However, CGO breaks static binary distribution on linux and complicates cross-compilation.
**Decision**: Use CGO_ENABLED=1 (mattn/go-sqlite3) for darwin builds and CGO_ENABLED=0 (modernc.org/sqlite) for linux builds.
**Consequences**:
- darwin: performance parity with TS; binary is dynamically linked but macOS always has libc
- linux: 1.8x slower SQLite than native but retains fully static binary; acceptable because linux use is primarily CI/server, not interactive menubar
- goreleaser must split into two build stanzas with different env settings
- CI must install a darwin cross-compiler (osxcross or zig cc) for the release workflow

**Tradeoff analysis (all three options)**:

| Approach | darwin perf | linux perf | linux static | CI complexity | Binary size |
|----------|------------|-----------|-------------|--------------|-------------|
| A: mattn everywhere | Best | Best | No (needs musl or dynamic) | Medium (CGO cross-compile) | ~12MB |
| B: modernc everywhere (status quo) | 1.8x slower | 1.8x slower | Yes | Low | ~18MB |
| C: hybrid (chosen) | Best | 1.8x slower | Yes | Medium-high (split builds) | ~12MB darwin, ~18MB linux |

Option C was chosen because darwin is the primary target (macOS menubar use case), and linux static binaries are a hard requirement for the existing Homebrew/binary download distribution.

**Alternatives considered**:
- Option A (mattn everywhere): requires musl-based static linking on linux or shipping dynamic binaries; rejected because static linux binary is a distribution requirement.
- Option B (modernc everywhere, status quo): does not address the 1.8x performance gap that motivated this spec; rejected.
