//go:build duckdb && !pgch

package db

import (
	"database/sql"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	_ "github.com/marcboeker/go-duckdb/v2"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
)

func Init() error {
	cfg := config.Config
	if cfg.DBType == "duckdb" {
		return initDuckDB()
	}
	return initPostgres()
}

func initDuckDB() error {
	path := config.Config.DuckDBPath
	if path == "" {
		path = "./traceway.duckdb"
	}

	mainDB, err := openDuckDB(path, false)
	if err != nil {
		return err
	}
	DB = mainDB
	Driver = lit.DuckDB
	config.Logf("DuckDB database opened at %s", path)

	telemetryPath := strings.TrimSuffix(path, ".duckdb") + "_telemetry.duckdb"
	if path == ":memory:" {
		telemetryPath = ":memory:"
	}
	telDB, err := openDuckDB(telemetryPath, true)
	if err != nil {
		return err
	}
	TelemetryDB = telDB
	config.Logf("DuckDB telemetry database opened at %s", telemetryPath)

	return nil
}

// openDuckDB opens one DuckDB database. Connections from a single *sql.DB share
// one in-process DuckDB instance, so the telemetry handle keeps a pool to absorb
// concurrent ingestion (DuckDB allows concurrent appends to a table); the main
// handle stays single-connection because its writes are low-volume relational
// updates where DuckDB's optimistic concurrency would otherwise surface
// write-write conflicts.
func openDuckDB(path string, telemetry bool) (*sql.DB, error) {
	dsn := path
	if path == ":memory:" {
		dsn = ""
	}

	d, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb at %s: %w", path, err)
	}
	if err := d.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping duckdb at %s: %w", path, err)
	}

	if telemetry {
		d.SetMaxOpenConns(duckDBTelemetryMaxConns())
	} else {
		d.SetMaxOpenConns(1)
	}
	return d, nil
}

// duckDBTelemetryMaxConns sizes the telemetry connection pool. Defaults to the
// CPU count (DuckDB parallelizes a single connection internally, so a handful of
// concurrent appenders is enough to saturate ingest) and can be overridden with
// DUCKDB_TELEMETRY_MAX_CONNS for benchmarking different write concurrencies.
func duckDBTelemetryMaxConns() int {
	if v := config.Config.DuckDBTelemetryMaxConns; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := runtime.NumCPU()
	if n < 4 {
		return 4
	}
	return n
}
