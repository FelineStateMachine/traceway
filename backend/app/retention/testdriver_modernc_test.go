//go:build !pgch && !cgo_sqlite

package retention

import _ "modernc.org/sqlite"

const testSQLiteDriver = "sqlite"
