//go:build !pgch

package controllers

import (
	"context"

	"github.com/tracewayapp/traceway/backend/app/repositories"
)

func fetchCHHealth(_ context.Context) HealthDeepResponse {
	resp := HealthDeepResponse{CHReachable: false}

	// In SQLite mode there's no ClickHouse but there may be a buffered async
	// writer. Populate its stats so the bench's per-step chSnapshot can show
	// buffer depth, enqueued/flushed/dropped counters, and overflow errors —
	// useful diagnostic when soft-cliff fires from 500s.
	stats := repositories.GetMetricBufferStats()
	resp.MetricBuffer = &MetricBufferInfo{
		Enabled:     stats.Enabled,
		Capacity:    stats.Capacity,
		Depth:       stats.Depth,
		Enqueued:    stats.Enqueued,
		Flushed:     stats.Flushed,
		Dropped:     stats.Dropped,
		FlushErrors: stats.FlushErrors,
	}
	return resp
}
