//go:build duckdb && !pgch

package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type metricPointRepository struct{}

// The DuckDB metric_points columns are native (project_id UUID, value DOUBLE,
// tags JSON, recorded_at TIMESTAMP), so these queries bind raw time.Time values
// (the driver only accepts time.Time for TIMESTAMP parameters) and scan buckets
// straight into time.Time — no RFC3339 strings anywhere on this path.

func (r *metricPointRepository) QueryTimeSeries(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time, intervalMinutes int, aggregation string, tagFilters map[string]string, groupBy string) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	aggFunc := duckdbAggregationFunc(aggregation)
	hasGroupBy := groupBy != ""

	selectClause := fmt.Sprintf("SELECT time_bucket(INTERVAL %d SECOND, recorded_at) AS bucket", secs)
	if hasGroupBy {
		selectClause += ", json_extract_string(tags, '$.' || :group_by) AS group_key"
	}
	selectClause += ", " + aggFunc + " AS agg_value FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to"

	params := lit.P{
		"project_id": projectId,
		"name":       name,
		"from":       from.UTC(),
		"to":         to.UTC(),
	}
	if hasGroupBy {
		params["group_by"] = groupBy
	}

	filterClauses := ""
	for i, k := range sortedKeys(tagFilters) {
		fk := fmt.Sprintf("fk_%d", i)
		fv := fmt.Sprintf("fv_%d", i)
		filterClauses += fmt.Sprintf(" AND json_extract_string(tags, '$.' || :%s) = :%s", fk, fv)
		params[fk] = k
		params[fv] = tagFilters[k]
	}

	query := selectClause + filterClauses + " GROUP BY bucket"
	if hasGroupBy {
		query += ", group_key"
	}
	query += " ORDER BY bucket ASC"

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucket time.Time
		var value float64
		groupKey := "__all__"

		if hasGroupBy {
			var groupKeyNullable *string
			if err := rows.Scan(&bucket, &groupKeyNullable, &value); err != nil {
				return nil, err
			}
			if groupKeyNullable != nil {
				groupKey = *groupKeyNullable
			}
		} else {
			if err := rows.Scan(&bucket, &value); err != nil {
				return nil, err
			}
		}

		if groupKey == "" {
			groupKey = "(empty)"
		}
		result[groupKey] = append(result[groupKey], models.TimeSeriesPoint{
			Timestamp: bucket.UTC(),
			Value:     value,
		})
	}
	return result, rows.Err()
}

// DiscoverMetrics mirrors the SQLite LEFT JOIN json_each shape: a metric whose
// tags are '{}' must still surface, with a NULL tag_key — the CASE substitutes a
// single-element [NULL] list so unnest always yields at least one row.
func (r *metricPointRepository) DiscoverMetrics(ctx context.Context, projectId uuid.UUID, from, to time.Time) ([]models.DiscoveredMetric, error) {
	query, args, err := lit.ParseNamedQuery(db.Driver,
		`SELECT name, k AS tag_key
		FROM (
			SELECT name,
				unnest(CASE WHEN json_keys(tags) IS NULL OR len(json_keys(tags)) = 0 THEN [NULL] ELSE json_keys(tags) END) AS k
			FROM metric_points
			WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		)
		GROUP BY name, k
		ORDER BY name ASC, k ASC NULLS FIRST`,
		lit.P{"project_id": projectId, "from": from.UTC(), "to": to.UTC()})
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectDiscoveredMetrics(rows)
}

func (r *metricPointRepository) DiscoverTagValues(ctx context.Context, projectId uuid.UUID, metricName, tagKey string, from, to time.Time) ([]string, error) {
	results, err := lit.SelectNamed[tagValueRow](db.TelemetryDB,
		`SELECT DISTINCT json_extract_string(tags, '$.' || :tag_key) AS tag_value
		FROM metric_points
		WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to
		AND json_extract_string(tags, '$.' || :tag_key) IS NOT NULL
		AND json_extract_string(tags, '$.' || :tag_key) != ''
		ORDER BY tag_value ASC`,
		lit.P{"project_id": projectId, "name": metricName, "tag_key": tagKey, "from": from.UTC(), "to": to.UTC()})
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(results))
	for _, r := range results {
		values = append(values, r.TagValue)
	}
	return values, nil
}

func (r *metricPointRepository) GetAggregateBetween(ctx context.Context, projectId uuid.UUID, name, aggregation string, start, end time.Time) (float64, error) {
	result, err := lit.SelectSingleNamed[avgResult](db.TelemetryDB,
		fmt.Sprintf("SELECT COALESCE(%s, 0) AS agg_value FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to", duckdbAggregationFunc(aggregation)),
		lit.P{"project_id": projectId, "name": name, "from": start.UTC(), "to": end.UTC()})
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Value, nil
}

func (r *metricPointRepository) GetAverageBetween(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time) (float64, error) {
	return r.GetAggregateBetween(ctx, projectId, name, "avg", start, end)
}

func (r *metricPointRepository) GetSortedValues(ctx context.Context, projectId uuid.UUID, name string, from, to time.Time) ([]float64, error) {
	query, args, err := lit.ParseNamedQuery(db.Driver,
		"SELECT value FROM metric_points WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to ORDER BY value ASC",
		lit.P{"project_id": projectId, "name": name, "from": from.UTC(), "to": to.UTC()})
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []float64{}
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (r *metricPointRepository) LatestRecordedAt(ctx context.Context, projectId uuid.UUID) (time.Time, bool, error) {
	var maxTs sql.NullTime
	if err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT MAX(recorded_at) FROM metric_points WHERE project_id = ?", projectId).Scan(&maxTs); err != nil {
		return time.Time{}, false, err
	}
	if !maxTs.Valid {
		return time.Time{}, false, nil
	}
	return maxTs.Time.UTC(), true, nil
}

func (r *metricPointRepository) GetDistinctServers(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]string, error) {
	results, err := lit.SelectNamed[distinctServerResult](db.TelemetryDB,
		`SELECT DISTINCT json_extract_string(tags, '$.server_name') AS sn
		FROM metric_points
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		AND json_extract_string(tags, '$.server_name') IS NOT NULL
		AND json_extract_string(tags, '$.server_name') != ''
		ORDER BY sn ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
	if err != nil {
		return nil, err
	}

	servers := make([]string, 0, len(results))
	for _, r := range results {
		servers = append(servers, r.ServerName)
	}
	return servers, nil
}

func (r *metricPointRepository) GetAverageByIntervalPerServer(ctx context.Context, projectId uuid.UUID, name string, start, end time.Time, intervalMinutes int, servers []string) (map[string][]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60

	params := lit.P{
		"project_id": projectId,
		"name":       name,
		"from":       start.UTC(),
		"to":         end.UTC(),
	}

	query := fmt.Sprintf(`SELECT
		time_bucket(INTERVAL %d SECOND, recorded_at) AS bucket,
		json_extract_string(tags, '$.server_name') AS sn,
		avg(value) AS avg_value
	FROM metric_points
	WHERE project_id = :project_id AND name = :name AND recorded_at >= :from AND recorded_at <= :to`, secs)

	if len(servers) > 0 {
		placeholders := make([]string, len(servers))
		for i, s := range servers {
			key := fmt.Sprintf("srv_%d", i)
			placeholders[i] = ":" + key
			params[key] = s
		}
		query += " AND json_extract_string(tags, '$.server_name') IN (" + strings.Join(placeholders, ", ") + ")"
	} else {
		query += " AND json_extract_string(tags, '$.server_name') IS NOT NULL AND json_extract_string(tags, '$.server_name') != ''"
	}

	query += " GROUP BY bucket, sn ORDER BY bucket ASC, sn ASC"

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucket time.Time
		var serverName string
		var value float64
		if err := rows.Scan(&bucket, &serverName, &value); err != nil {
			return nil, err
		}

		result[serverName] = append(result[serverName], models.TimeSeriesPoint{
			Timestamp: bucket.UTC(),
			Value:     value,
		})
	}
	return result, rows.Err()
}

func duckdbAggregationFunc(agg string) string {
	switch agg {
	case "min":
		return "min(value)"
	case "max":
		return "max(value)"
	case "sum":
		return "sum(value)"
	case "count":
		return "CAST(COUNT(*) AS DOUBLE)"
	default:
		return "avg(value)"
	}
}

var MetricPointRepository = metricPointRepository{}
