//go:build duckdb && !pgch

package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/db"
)

type ExtensionMigration struct {
	Source embed.FS
	Path   string
	Table  string
}

var ExtensionPostgresMigrations []ExtensionMigration

//go:embed duckdb/*.sql
var migrationsDuckDBFS embed.FS

//go:embed duckdb_telemetry/*.sql
var migrationsDuckDBTelemetryFS embed.FS

func Run(dbType string) error {
	if err := runMigrationsOn(db.DB, migrationsDuckDBFS, "duckdb", "schema_migrations"); err != nil {
		return fmt.Errorf("main db migrations: %w", err)
	}

	if err := runMigrationsOn(db.TelemetryDB, migrationsDuckDBTelemetryFS, "duckdb_telemetry", "schema_migrations"); err != nil {
		return fmt.Errorf("telemetry db migrations: %w", err)
	}

	// Flush the migration DDL out of the WAL immediately. DuckDB cannot replay
	// ALTER records against tables whose columns have function defaults
	// (nextval/now) — replay dies with "GetDefaultDatabase with no default
	// database set" — and the main DB writes so little that its WAL never hits
	// the auto-checkpoint threshold, so those records would otherwise sit there
	// indefinitely. An unclean shutdown (e.g. an OOM kill under load) would then
	// brick the database: every restart panics replaying the WAL. Checkpointing
	// here leaves only row-change records in any future WAL, which replay fine.
	if _, err := db.DB.Exec("CHECKPOINT"); err != nil {
		return fmt.Errorf("main db post-migration checkpoint: %w", err)
	}
	if _, err := db.TelemetryDB.Exec("CHECKPOINT"); err != nil {
		return fmt.Errorf("telemetry db post-migration checkpoint: %w", err)
	}

	return nil
}

func runMigrationsOn(target *sql.DB, fsys embed.FS, dir string, trackingTable string) error {
	_, err := target.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version VARCHAR PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT now()
	)`, trackingTable))
	if err != nil {
		return fmt.Errorf("failed to create %s table: %w", trackingTable, err)
	}

	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		version := strings.TrimSuffix(file, ".up.sql")

		var count int
		err := target.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE version = ?", trackingTable), version).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration version %s: %w", version, err)
		}
		if count > 0 {
			continue
		}

		content, err := fsys.ReadFile(dir + "/" + file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := target.Exec(stmt); err != nil {
				return fmt.Errorf("failed to execute migration %s: %w", file, err)
			}
		}

		if _, err := target.Exec(fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", trackingTable), version); err != nil {
			return fmt.Errorf("failed to record migration version %s: %w", version, err)
		}
	}

	return nil
}
