//go:build !pgch && !duckdb

package repositories

import (
	"context"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func (r *metricPointRepository) InsertAsync(ctx context.Context, points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, p := range points {
		tags := NewSQLiteJSONMap(p.Tags)
		tagsVal, _ := tags.Value()
		query, args, err := lit.ParseNamedQuery(db.Driver,
			"INSERT INTO metric_points (project_id, name, value, tags, recorded_at) VALUES (:project_id, :name, :value, :tags, :recorded_at)",
			lit.P{
				"project_id":  p.ProjectId,
				"name":        p.Name,
				"value":       p.Value,
				"tags":        tagsVal,
				"recorded_at": NewSQLiteTime(p.RecordedAt),
			})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	db.AddIngestedTelemetryRows(int64(len(points)))
	return nil
}
