# Spec: Claude JSONL Per-File Caching

**Version**: 0.2.0  **Status**: approved  **Tier**: feature
**Created**: 2026-04-21  **Last updated**: 2026-04-21

## Objective

Extend the SQLite session cache to individual Claude JSONL files. Today `parseSource`
skips SQLite caching when `isClaudeDir == true`, relying on the 60s in-process cache
only. This feature changes `claude.Provider.DiscoverSessions()` to emit one
`SessionSource` per JSONL file (instead of per directory) and updates
`claude.Provider.ParseSession()` to handle a single file path, so Claude files flow
through the existing fingerprint-and-cache path already used by Codex and Cursor. A
cache hit skips re-parsing entirely; a mtime guard prevents caching files actively
being written.

## Out of Scope

- Caching for Codex or Cursor providers (each already caches per-file)
- Changes to the `collectJSONLFiles` signature or walk depth
- Cache eviction, TTL expiry, or garbage collection of stale rows
- Changes to the in-process 60s LRU (`ParseAllSessionsCached`)
- Metrics or logging for cache hit rates

## Changelog

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-04-21 | team | Initial draft (multi-agent pipeline) |
| 0.2.0 | 2026-04-21 | team | Switch to Option A (per-file DiscoverSessions); add mtime guard |
