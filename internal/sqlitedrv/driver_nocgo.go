//go:build !cgo

package sqlitedrv

import _ "modernc.org/sqlite"

// DriverName is "sqlite" when built without CGO (modernc.org/sqlite).
const DriverName = "sqlite"
