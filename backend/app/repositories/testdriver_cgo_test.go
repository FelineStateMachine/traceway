//go:build !pgch && cgo_sqlite

package repositories

import _ "github.com/mattn/go-sqlite3"

const testSQLiteDriver = "sqlite3"
