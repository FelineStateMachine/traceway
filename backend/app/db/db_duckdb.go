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
	if telemetryPath != ":memory:" {
		TelemetryFilePath = telemetryPath
	}
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

	// memory_limit is a global, instance-wide setting; applying it on the lone
	// post-Ping connection caps the database before the pool grows. A percentage
	// (e.g. "70%") is resolved by DuckDB against the box's physical RAM, so it
	// bounds memory and errors gracefully under pressure instead of letting the
	// OS OOM-kill the process.
	if limit := duckDBMemoryLimit(); limit != "" {
		if _, err := d.Exec("SET memory_limit = '" + limit + "'"); err != nil {
			return nil, fmt.Errorf("failed to set duckdb memory_limit=%q at %s: %w", limit, path, err)
		}
		var resolved string
		if err := d.QueryRow("SELECT current_setting('memory_limit')").Scan(&resolved); err == nil {
			config.Logf("DuckDB memory_limit for %s = %s (configured %s)", path, resolved, limit)
		}
	}

	if telemetry {
		d.SetMaxOpenConns(duckDBTelemetryMaxConns())
	} else {
		d.SetMaxOpenConns(1)
	}
	return d, nil
}

// duckDBMemoryLimit returns the value passed to DuckDB's memory_limit setting.
// Accepts an absolute size ("4GB", passed through) or a percentage of physical
// RAM ("70%"). DuckDB's own parser rejects "%", so a percentage is resolved here
// against the box's total RAM and handed to DuckDB as MiB — this auto-scales
// across benchmark tiers. Defaults to 70%; override with DUCKDB_MEMORY_LIMIT.
// Returns "" (skip the SET, leaving DuckDB's built-in default) when a percentage
// cannot be resolved on this platform.
func duckDBMemoryLimit() string {
	v := strings.TrimSpace(config.Config.DuckDBMemoryLimit)
	if v == "" {
		v = "70%"
	}

	pctStr, isPct := strings.CutSuffix(v, "%")
	if !isPct {
		return v
	}

	pct, err := strconv.ParseFloat(strings.TrimSpace(pctStr), 64)
	if err != nil || pct <= 0 || pct > 100 {
		return ""
	}
	total := totalPhysicalMemory()
	if total == 0 {
		return ""
	}
	mib := uint64(float64(total) * pct / 100.0 / (1024 * 1024))
	if mib == 0 {
		return ""
	}
	return strconv.FormatUint(mib, 10) + "MiB"
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
