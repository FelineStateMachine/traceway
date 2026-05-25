//go:build !pgch && !cgo_sqlite

package repositories

import _ "modernc.org/sqlite"

const testSQLiteDriver = "sqlite"
