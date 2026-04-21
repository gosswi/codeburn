# Requirements: SQLite Driver Migration

## Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| R-01 | When building on darwin (CGO_ENABLED=1), the binary shall use `mattn/go-sqlite3` as the SQLite driver. | Must |
| R-02 | When building with CGO_ENABLED=0 (linux default, or explicit override), the binary shall use `modernc.org/sqlite` as the SQLite driver. | Must |
| R-03 | The build-tag mechanism shall prevent both drivers from being imported into the same binary. | Must |
| R-04 | All `sql.Open` calls shall use the driver name matching the active driver (`"sqlite3"` for mattn, `"sqlite"` for modernc). | Must |
| R-05 | The goreleaser configuration shall produce darwin binaries with CGO_ENABLED=1 and linux binaries with CGO_ENABLED=0. | Must |
| R-06 | The GitHub Actions release workflow shall install a C toolchain for darwin cross-compilation. | Must |
| R-07 | All existing SQLite functionality (session cache CRUD, Cursor provider queries including json_extract) shall behave identically under both drivers. | Must |
| R-08 | A benchmark test shall demonstrate that mattn/go-sqlite3 is faster than modernc.org/sqlite for the session cache workload. | Must |

## Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NF-01 | No increase in darwin binary size beyond 2MB from the driver change. | < 2MB delta |
| NF-02 | Linux binaries remain fully static (no dynamic linking to libsqlite3). | Static binary |
| NF-03 | `go test ./...` passes under both CGO_ENABLED=1 and CGO_ENABLED=0 on the CI runner. | Both pass |
