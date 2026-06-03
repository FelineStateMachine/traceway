//go:build duckdb && !pgch

package repositories

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"

	duckdb "github.com/marcboeker/go-duckdb/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// InsertAsync bulk-loads metric points through DuckDB's native Appender, which
// writes columnar chunks straight into storage and skips the SQL
// parse/bind/execute path that the row-by-row SQLite insert pays per row. This
// is the high-throughput ingestion path the metric benchmarks pressure-test.
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
			tags := "{}"
			if len(p.Tags) > 0 {
				if b, marshalErr := json.Marshal(p.Tags); marshalErr == nil {
					tags = string(b)
				}
			}
			if err := appender.AppendRow(
				p.ProjectId.String(),
				p.Name,
				p.Value,
				tags,
				p.RecordedAt.UTC().Format(time.RFC3339Nano),
			); err != nil {
				appender.Close()
				return err
			}
		}

		return appender.Close()
	})
}
