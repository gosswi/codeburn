//go:build cgo

package sqlitedrv

import _ "github.com/mattn/go-sqlite3"

// DriverName is "sqlite3" when built with CGO (mattn/go-sqlite3).
const DriverName = "sqlite3"
