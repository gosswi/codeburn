// Package sqlitedrv selects the SQLite driver at compile time.
// On CGO-enabled builds (darwin), mattn/go-sqlite3 is used (driver name "sqlite3").
// On pure-Go builds (linux, CGO_ENABLED=0), modernc.org/sqlite is used (driver name "sqlite").
// All sql.Open callers must use DriverName instead of a hardcoded string.
package sqlitedrv
