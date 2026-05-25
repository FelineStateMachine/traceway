//go:build !pgch && cgo_sqlite

package db

import (
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/mattn/go-sqlite3"
)

var driverSeq uint32

func openSQLite(path string, telemetry bool) (*sql.DB, error) {
	name := fmt.Sprintf("sqlite3_%d", atomic.AddUint32(&driverSeq, 1))
	sql.Register(name, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			pragmas := []string{
				"PRAGMA journal_mode = WAL",
				"PRAGMA busy_timeout = 5000",
			}
			if telemetry {
				pragmas = append(pragmas,
					"PRAGMA synchronous = NORMAL",
					"PRAGMA cache_size = -524288",
					"PRAGMA temp_store = MEMORY",
					"PRAGMA mmap_size = 1073741824",
					"PRAGMA wal_autocheckpoint = 50000",
				)
			} else {
				pragmas = append(pragmas, "PRAGMA foreign_keys = ON")
			}
			for _, p := range pragmas {
				if _, err := conn.Exec(p, nil); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
			}
			return nil
		},
	})

	dsn := path
	if path != ":memory:" {
		dsn = "file:" + path
	}

	d, err := sql.Open(name, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite at %s: %w", path, err)
	}
	if err := d.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite at %s: %w", path, err)
	}

	if telemetry {
		d.SetMaxOpenConns(4)
	} else {
		d.SetMaxOpenConns(1)
	}
	return d, nil
}
