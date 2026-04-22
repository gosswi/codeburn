package parser

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/agentseal/codeburn/internal/sqlitedrv"
	"github.com/agentseal/codeburn/internal/types"
)

const (
	defaultDBPath = ".cache/codeburn/session-cache.db"

	createTable = `
CREATE TABLE IF NOT EXISTS session_summaries (
  file_path TEXT PRIMARY KEY,
  mtime_ms INTEGER NOT NULL,
  file_size INTEGER NOT NULL,
  summary_json TEXT NOT NULL,
  cached_at INTEGER NOT NULL
)`
)

// SessionCache wraps a SQLite database for session summary caching.
type SessionCache struct {
	db *sql.DB
}

// OpenCache opens (or creates) the SQLite cache at ~/.cache/codeburn/session-cache.db.
// On open or schema failure, it deletes the file and retries once.
// If the retry also fails, it returns nil so callers proceed without caching.
func OpenCache() (*SessionCache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	dbPath := filepath.Join(home, defaultDBPath)
	return openCacheAt(dbPath, true)
}

func openCacheAt(dbPath string, allowReset bool) (*SessionCache, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, nil
	}

	db, err := sql.Open(sqlitedrv.DriverName, dbPath)
	if err != nil {
		return tryRecover(dbPath, allowReset)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return tryRecover(dbPath, allowReset)
	}

	return &SessionCache{db: db}, nil
}

func tryRecover(dbPath string, allowReset bool) (*SessionCache, error) {
	if !allowReset {
		return nil, nil
	}
	os.Remove(dbPath)
	return openCacheAt(dbPath, false)
}

func initSchema(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=3000"); err != nil {
		return err
	}
	if _, err := db.Exec(createTable); err != nil {
		return err
	}
	return nil
}

// Close releases the database connection.
func (c *SessionCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// GetCachedSummary returns the cached summary for a file if the fingerprint matches.
// Returns nil (not an error) on a cache miss.
func (c *SessionCache) GetCachedSummary(filePath string, mtimeMs int64, fileSize int64) (*types.SessionSummary, error) {
	if c == nil || c.db == nil {
		return nil, nil
	}

	var summaryJSON string
	err := c.db.QueryRow(
		`SELECT summary_json FROM session_summaries WHERE file_path = ? AND mtime_ms = ? AND file_size = ?`,
		filePath, mtimeMs, fileSize,
	).Scan(&summaryJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}

	var summary types.SessionSummary
	if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
		// Malformed row - delete it and treat as a miss.
		c.db.Exec(`DELETE FROM session_summaries WHERE file_path = ?`, filePath)
		return nil, nil
	}
	return &summary, nil
}

// PutCachedSummary writes a summary to the cache with the given file fingerprint.
// userMessage fields must be zeroed before calling this (privacy invariant R15).
// Write errors are silently swallowed (R43/AC41).
func (c *SessionCache) PutCachedSummary(filePath string, mtimeMs int64, fileSize int64, summary *types.SessionSummary) error {
	if c == nil || c.db == nil {
		return nil
	}

	data, err := json.Marshal(summary)
	if err != nil {
		return nil
	}

	c.db.Exec(
		`INSERT OR REPLACE INTO session_summaries (file_path, mtime_ms, file_size, summary_json, cached_at) VALUES (?, ?, ?, ?, ?)`,
		filePath, mtimeMs, fileSize, string(data), time.Now().UnixMilli(),
	)
	return nil
}

// GetFileFingerprint stats a file and returns its mtime in milliseconds and size in bytes.
func GetFileFingerprint(filePath string) (mtimeMs int64, fileSize int64, err error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, 0, err
	}
	return info.ModTime().UnixMilli(), info.Size(), nil
}
