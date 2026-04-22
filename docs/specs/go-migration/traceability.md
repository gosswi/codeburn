# Traceability Matrix: CodeBurn Go Migration

| Requirement | Design Decision | Acceptance Criteria | Phase | Implementation |
| --- | --- | --- | --- | --- |
| R1 | D6 | AC1 | P2 | `cmd/codeburn/main.go`, `internal/config/config.go` |
| R2 | - | AC2 | P2 | `internal/format/format.go`, `internal/currency/currency.go` |
| R3 | - | AC3, AC4 | P2 | `internal/models/models.go`, `internal/models/litellm.go` |
| R4 | - | AC21 | P2 | `internal/models/models.go` |
| R5 | - | AC22 | P2 | `internal/format/format.go` |
| R6 | D4 | AC23 | P2 | `internal/models/litellm.go` |
| R7 | D6 | AC24, AC25 | P2 | `internal/provider/codex/codex.go` |
| R8 | D4 | AC19 | P2 | `cmd/codeburn/main.go` |
| R9 | D10 | AC17 | P3 | `internal/tui/model.go`, `internal/tui/dashboard.go` |
| R10 | D5, D9 | AC16 | P1 | `internal/parser/parser.go` (SQLite cache) |
| R11 | D5 | AC26 | P1 | `internal/parser/parser.go` (in-proc cache) |
| R12 | D1 | AC10 | P1 | `internal/provider/claude/claude.go` |
| R13 | D2 | AC8 | P1 | `internal/provider/cursor/cursor.go` |
| R14 | D2 | AC9 | P1 | `internal/provider/codex/codex.go` (token normalization) |
| R15 | D8 | AC7 | P1 | `internal/parser/parser.go` (UserMessage zeroing) |
| R16 | D7 | AC5, AC6 | P1 | `internal/classifier/classifier.go` |
| R17 | D7 | AC12 | P1 | `internal/classifier/classifier.go` |
| R18 | D7 | AC27 | P1 | `internal/classifier/classifier.go` |
| R19 | - | AC11 | P1 | `internal/models/models.go` |
| R20 | - | AC28 | P1 | `internal/provider/codex/codex.go` |
| R21 | - | AC29 | P1 | `internal/provider/cursor/cursor.go` |
| R22 | - | AC30 | P2 | `internal/currency/currency.go` (Frankfurter + fallback rate=1) |
| R23 | D5 | AC18 | P1 | `internal/parser/parser.go` (worker pool) |
| R24 | - | AC31 | P3 | `internal/tui/panels.go` (shortProject) |
| R25 | D7 | AC32 | P1 | `internal/classifier/classifier.go` |
| R26 | D10 | AC43 | P3 | `internal/tui/panels.go` (8 panels) |
| R27 | D10 | AC44 | P3 | `internal/tui/layout.go` (GetLayout) |
| R28 | D10 | AC15 | P3 | `internal/tui/gradient.go` (HBar, gradientColor) |
| R29 | D10 | AC33 | P3 | `internal/tui/model.go` (keyboard: q quit) |
| R30 | D10 | AC34 | P3 | `internal/tui/model.go` (keyboard: arrows) |
| R31 | D10 | AC35 | P3 | `internal/tui/model.go` (keyboard: 1-4) |
| R32 | D10 | AC36 | P3 | `internal/tui/model.go` (debounce 600ms) |
| R33 | D10 | AC37 | P3 | `internal/tui/model.go` (provider cycling) |
| R34 | D2 | AC45 | P1 | `internal/provider/cursor/cursor.go` (35-day lookback) |
| R35 | D7 | AC5 | P1 | `internal/classifier/classifier.go` |
| R36 | D2 | AC14 | P2 | `internal/provider/cursor/cursor.go` (file cache) |
| R37 | D1, D2 | AC46 | P2 | `internal/parser/parser.go` (in-proc LRU) |
| R38 | - | AC38 | P1 | `internal/provider/codex/codex.go` (session meta validation) |
| R39 | - | AC39 | P1 | `internal/provider/codex/codex.go` (delta accounting) |
| R40 | D1 | AC13 | P1 | `internal/provider/claude/claude.go` (dedup by msg.id) |
| R41 | D1 | AC47 | P1 | `internal/parser/parser.go` (sync.Map global dedup) |
| R42 | D1 | AC40 | P1 | `internal/provider/cursor/cursor.go` (dedup key) |
| R43 | - | AC41 | P1 | `internal/provider/codex/codex.go` (tool normalization) |
| R44 | - | AC20 | P2 | `internal/export/export.go` (CSV formula injection) |
| R45 | - | AC42 | P2 | `internal/export/export.go` (JSON export) |
| R46 | - | AC48 | P2 | `internal/menubar/menubar.go` (RenderMenubarFormat) |
| R47 | - | AC49 | P2 | `internal/menubar/menubar.go` (InstallMenubar) |
| R48 | - | AC50 | P2 | `internal/menubar/menubar.go` (UninstallMenubar) |
| R49 | - | AC51 | P2 | `cmd/codeburn/main.go` (exit codes) |
| R50 | - | AC52 | P2 | `internal/config/config.go` (invalid JSON -> empty config) |
| R51 | - | AC53 | P3 | `internal/tui/dashboard.go` (non-TTY static frame) |
| R52 | D1 | AC54 | P1 | `internal/provider/claude/claude.go` (subagent JSONL) |
| R53 | - | AC55 | P1 | `internal/provider/cursor/cursor.go` (language extraction) |

## Phase 4: Distribution

| Artifact | Purpose |
|----------|---------|
| `.goreleaser.yaml` | Multi-platform builds: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 with CGO_ENABLED=0 and -ldflags="-s -w" |
| `.github/workflows/release.yml` | GitHub Actions: trigger on v* tags, attach binaries to GitHub Releases |
| `codeburn.rb` | Homebrew formula for darwin (amd64 + arm64) with SHA256 placeholders to update after first release |
| `scripts/postinstall.js` | npm postinstall: detects installed Go binary and prints informational notice |

## Code Coverage by Package

| Package | Files | Tests |
|---------|-------|-------|
| `internal/types` | `types.go` | (shared types, no logic) |
| `internal/classifier` | `classifier.go` | `classifier_test.go` |
| `internal/models` | `models.go`, `litellm.go` | `models_test.go`, `litellm_test.go` |
| `internal/provider` | `provider.go` | (interface only) |
| `internal/provider/claude` | `claude.go` | `claude_test.go` |
| `internal/provider/codex` | `codex.go` | `codex_test.go` |
| `internal/provider/cursor` | `cursor.go` | `cursor_test.go` |
| `internal/parser` | `parser.go` | `parser_test.go` |
| `internal/config` | `config.go` | `config_test.go` |
| `internal/currency` | `currency.go` | `currency_test.go` |
| `internal/format` | `format.go` | `format_test.go` |
| `internal/export` | `export.go` | `export_test.go` |
| `internal/menubar` | `menubar.go` | `menubar_test.go` |
| `internal/tui` | `gradient.go`, `layout.go`, `model.go`, `panels.go`, `dashboard.go` | `gradient_test.go`, `layout_test.go` |
| `cmd/codeburn` | `main.go` | (integration via `go run`) |
