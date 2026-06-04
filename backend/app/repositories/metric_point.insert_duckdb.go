//go:build duckdb && !pgch

package repositories

import (
	"context"
	"database/sql/driver"

	duckdb "github.com/marcboeker/go-duckdb/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// InsertAsync bulk-loads metric points through DuckDB's native Appender, which
// writes columnar chunks straight into storage and skips the SQL
// parse/bind/execute path that the row-by-row SQLite insert pays per row. The
// values go in natively — UUID bytes, time.Time, and the tags map (the JSON
// column marshals it in the driver) — so no per-point string formatting
// remains. This is the high-throughput ingestion path the metric benchmarks
// pressure-test.
func (r *metricPointRepository) InsertAsync(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	conn, err := db.TelemetryDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		appender, err := duckdb.NewAppenderFromConn(driverConn.(driver.Conn), "", "metric_points")
		if err != nil {
			return err
		}

		for i := range points {
			p := &points[i]
			tags := p.Tags
			if tags == nil {
				tags = map[string]string{}
			}
			if err := appender.AppendRow(
				duckdb.UUID(p.ProjectId),
				p.Name,
				p.Value,
				tags,
				p.RecordedAt.UTC(),
			); err != nil {
				appender.Close()
				return err
			}
		}

		if err := appender.Close(); err != nil {
			return err
		}
		db.AddIngestedTelemetryRows(int64(len(points)))
		return nil
	})
}
