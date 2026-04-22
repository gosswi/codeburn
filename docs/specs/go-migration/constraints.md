# Constraints: CodeBurn Go Migration

## Technical Constraints

**C1** - The Go binary must target Go 1.23 minimum (required for `iter.Seq2`).

**C2** - The binary must be a single static executable with no shared libraries (no CGO). Build command: `go build -ldflags="-s -w" -o codeburn ./cmd/codeburn`. This enables cross-platform distribution. **[Superseded for darwin by `docs/specs/sqlite-driver-migration`: darwin builds use CGO_ENABLED=1 with mattn/go-sqlite3. This constraint remains binding for linux targets only.]**

**C3** - The binary must compile for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`. `windows/amd64` is a stretch goal (menubar integration is macOS-only).

**C4** - The package structure must follow the layout in go-migration.md section 4. `internal/` packages are not importable by external Go modules.

**C5** - `modernc.org/sqlite` must be used for all SQLite access (session cache and Cursor provider). No CGO SQLite bindings. **[Superseded for darwin by `docs/specs/sqlite-driver-migration`: darwin uses mattn/go-sqlite3 (CGO). This constraint remains binding for linux targets only.]**

**C6** - All regexes in `internal/classifier/classifier.go` must be compiled once at `var` initialization time (not inside functions called per-turn). This preserves the TypeScript behavior of module-level regex compilation.

**C7** - The Go binary must not read or write any file outside of: `$CLAUDE_CONFIG_DIR` (or `~/.claude`), `$CODEX_HOME` (or `~/.codex`), the Cursor platform path, `~/.config/codeburn/`, `~/.cache/codeburn/`, and the SwiftBar/xbar plugin directories. No telemetry, no home directory writes outside these paths.

**C8** - No external network requests other than those made by the TypeScript implementation: LiteLLM pricing URL and Frankfurter exchange rate URL. No analytics, no crash reporting.

## Business Constraints

**C9** - The Go binary must maintain the same user-facing CLI interface during the entire migration. Users who have automated `codeburn status --format json` in scripts must not see breakage.

**C10** - The npm package (`codeburn`) must continue to ship and work. The Go binary is a separate distribution artifact (GitHub Releases, Homebrew tap). Co-existence period lasts until Phase 3 ships.

**C11** - The SQLite session cache format (schema + fingerprint semantics) must not change between TypeScript and Go versions. A user who has a populated cache from the TypeScript version must see cache hits immediately in the Go version.

**C12** - The Cursor cache file format (`~/.cache/codeburn/cursor-results.json`) must not change. The fingerprint field (`mtimeMs:size`) must be computed identically.

**C13** - Git commit authorship follows the project convention: `AgentSeal <hello@agentseal.org>`. No `Co-Authored-By` lines.
