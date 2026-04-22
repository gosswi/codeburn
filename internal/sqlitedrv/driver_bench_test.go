package sqlitedrv_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/agentseal/codeburn/internal/sqlitedrv"
)

// BenchmarkSessionCacheWorkload benchmarks INSERT + SELECT operations that
// match the session cache workload (cache.go). Run under both drivers:
//
//	CGO_ENABLED=1 go test ./internal/sqlitedrv/ -bench=. -benchtime=5s  # mattn/go-sqlite3
//	CGO_ENABLED=0 go test ./internal/sqlitedrv/ -bench=. -benchtime=5s  # modernc.org/sqlite
//
// Expected result: mattn is faster than modernc for this workload (R-08).
func BenchmarkSessionCacheWorkload(b *testing.B) {
	dir := b.TempDir()
	dbPath := dir + "/bench.db"

	db, err := sql.Open(sqlitedrv.DriverName, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		b.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session_summaries (
		  file_path TEXT PRIMARY KEY,
		  mtime_ms INTEGER NOT NULL,
		  file_size INTEGER NOT NULL,
		  summary_json TEXT NOT NULL,
		  cached_at INTEGER NOT NULL
		)`); err != nil {
		b.Fatal(err)
	}

	payload := map[string]any{
		"session_id":   "abc123",
		"project":      "codeburn",
		"total_cost":   1.23,
		"input_tokens": 10000,
	}
	summaryJSON, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := os.TempDir() + "/session-" + string(rune('a'+i%26)) + ".jsonl"
		_, err := db.Exec(`
			INSERT OR REPLACE INTO session_summaries
			  (file_path, mtime_ms, file_size, summary_json, cached_at)
			VALUES (?, ?, ?, ?, ?)`,
			path, int64(i), int64(1024+i), string(summaryJSON), int64(i))
		if err != nil {
			b.Fatal(err)
		}

		var out string
		err = db.QueryRow(
			`SELECT summary_json FROM session_summaries WHERE file_path = ? AND mtime_ms = ? AND file_size = ?`,
			path, int64(i), int64(1024+i),
		).Scan(&out)
		if err != nil {
			b.Fatal(err)
		}
	}
}
