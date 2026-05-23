//go:build !pgch

package repositories

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// Buffered async insert path for SQLite metrics ingest. Enabled with
// SQLITE_BUFFERED_INSERT=1. Modeled on Prom/VM: HTTP returns 200 in
// microseconds after enqueueing to RAM; a background goroutine drains the
// buffer in batched-transaction multi-row INSERTs. Trade-off: up to
// flushInterval of metric points lost on process crash, and HTTP 500
// backpressure when SQLite can't keep up with the drain rate.
//
// Backpressure: on buffer overflow we return ErrMetricBufferFull from
// InsertAsync. The OTel metrics controller surfaces this as 500 via
// AbortWithError, the bench's evaluateStep treats 500s as errors, and the
// soft-cliff/error-threshold logic stops the failing step. The per-step
// /api/health/deep snapshot also reports buffer depth so the JSON makes the
// cliff visible even before errors fire.

const (
	metricBufferCapacity = 1_000_000
	metricFlushBatchSize = 5_000
	metricFlushInterval  = 1 * time.Second
)

var (
	sqliteBufferedMetricsEnabled = os.Getenv("SQLITE_BUFFERED_INSERT") == "1"

	metricBuffer chan models.MetricPoint

	metricsEnqueued atomic.Int64
	metricsFlushed  atomic.Int64
	metricsDropped  atomic.Int64
	metricFlushErrs atomic.Int64

	metricFlusherOnce sync.Once
)

// ErrMetricBufferFull is returned by InsertAsync when the in-memory buffer is
// saturated. Senders see HTTP 500 and retry with backoff.
var ErrMetricBufferFull = errors.New("sqlite metric buffer full")

func init() {
	if sqliteBufferedMetricsEnabled {
		metricBuffer = make(chan models.MetricPoint, metricBufferCapacity)
	}
}

// startMetricFlusher launches the drain goroutine. Idempotent via sync.Once.
// The goroutine outlives the process; the SUT in our bench is killed by
// docker-compose down between matrix entries so leak-on-shutdown is fine.
func startMetricFlusher() {
	metricFlusherOnce.Do(func() {
		go runMetricFlusher()
	})
}

func runMetricFlusher() {
	ticker := time.NewTicker(metricFlushInterval)
	defer ticker.Stop()

	batch := make([]models.MetricPoint, 0, metricFlushBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := flushMetricBatch(batch); err != nil {
			metricFlushErrs.Add(1)
		} else {
			metricsFlushed.Add(int64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case p := <-metricBuffer:
			batch = append(batch, p)
			if len(batch) >= metricFlushBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// flushMetricBatch writes the entire batch as a single multi-row INSERT inside
// one transaction. One statement parse, one Exec, one fsync — the amortisation
// that makes the buffered path worthwhile.
func flushMetricBatch(points []models.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}
	tx, err := db.TelemetryDB.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var sb strings.Builder
	sb.Grow(len("INSERT INTO metric_points (project_id, name, value, tags, recorded_at) VALUES ") + len(points)*len("(?, ?, ?, ?, ?), "))
	sb.WriteString("INSERT INTO metric_points (project_id, name, value, tags, recorded_at) VALUES ")
	args := make([]any, 0, len(points)*5)
	for i, p := range points {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?, ?)")
		args = append(args,
			p.ProjectId,
			p.Name,
			p.Value,
			NewSQLiteJSONMap(p.Tags),
			NewSQLiteTime(p.RecordedAt),
		)
	}
	if _, err := tx.ExecContext(context.Background(), sb.String(), args...); err != nil {
		return err
	}
	return tx.Commit()
}

// MetricBufferStats reports the in-memory buffer state for observability.
type MetricBufferStats struct {
	Enabled     bool  `json:"enabled"`
	Capacity    int   `json:"capacity"`
	Depth       int   `json:"depth"`
	Enqueued    int64 `json:"enqueued"`
	Flushed     int64 `json:"flushed"`
	Dropped     int64 `json:"dropped"`
	FlushErrors int64 `json:"flushErrors"`
}

// GetMetricBufferStats is read by /api/health/deep so the loadgen can embed
// per-step buffer depth + counters in its JSON. Returns Enabled=false when
// SQLITE_BUFFERED_INSERT is not set, so callers can branch on that.
func GetMetricBufferStats() MetricBufferStats {
	if !sqliteBufferedMetricsEnabled {
		return MetricBufferStats{Enabled: false}
	}
	return MetricBufferStats{
		Enabled:     true,
		Capacity:    cap(metricBuffer),
		Depth:       len(metricBuffer),
		Enqueued:    metricsEnqueued.Load(),
		Flushed:     metricsFlushed.Load(),
		Dropped:     metricsDropped.Load(),
		FlushErrors: metricFlushErrs.Load(),
	}
}

// enqueueMetricPoints is called by the buffered path of InsertAsync. Returns
// ErrMetricBufferFull when the channel can't accept the entire batch — the
// metrics that did succeed are still in the buffer, but the caller's request
// fails so the OTLP sender retries the full batch (small duplicate risk on
// retry; acceptable for benchmark visibility, documented in the package
// comment).
func enqueueMetricPoints(points []models.MetricPoint) error {
	startMetricFlusher()
	for i, p := range points {
		select {
		case metricBuffer <- p:
			metricsEnqueued.Add(1)
		default:
			metricsDropped.Add(int64(len(points) - i))
			return ErrMetricBufferFull
		}
	}
	return nil
}
