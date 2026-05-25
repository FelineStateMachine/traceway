//go:build !pgch

package db

import (
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/mattn/go-sqlite3"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/lit/v2"
)

var driverSeq uint32

func Init() error {
	cfg := config.Config
	if cfg.DBType == "sqlite" {
		return initSQLite()
	}
	return initPostgres()
}

func initSQLite() error {
	path := config.Config.SQLitePath
	if path == "" {
		path = "./traceway.db"
	}

	mainDB, err := openSQLite(path, false)
	if err != nil {
		return err
	}
	DB = mainDB
	Driver = lit.SQLite
	config.Logf("SQLite database opened at %s", path)

	telemetryPath := strings.TrimSuffix(path, ".db") + "_telemetry.db"
	if path == ":memory:" {
		telemetryPath = ":memory:"
	}
	telDB, err := openSQLite(telemetryPath, true)
	if err != nil {
		return err
	}
	TelemetryDB = telDB
	config.Logf("SQLite telemetry database opened at %s", telemetryPath)

	return nil
}

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
