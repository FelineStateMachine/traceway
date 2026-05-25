//go:build !pgch && cgo_sqlite

package retention

import _ "github.com/mattn/go-sqlite3"

const testSQLiteDriver = "sqlite3"
