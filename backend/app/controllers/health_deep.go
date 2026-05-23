package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type healthDeepController struct{}

type TableParts struct {
	Table string `json:"table"`
	Parts int64  `json:"parts"`
}

type CHError struct {
	Name          string `json:"name"`
	Value         int64  `json:"value"`
	LastErrorTime string `json:"lastErrorTime,omitempty"`
}

// MetricBufferInfo mirrors repositories.MetricBufferStats and reports SQLite
// async-insert buffer state. Populated only when SQLITE_BUFFERED_INSERT=1 is
// set and the build is sqlite mode. Surfaced so the bench's per-step
// chSnapshot captures backpressure (depth approaching capacity, drops > 0).
type MetricBufferInfo struct {
	Enabled     bool  `json:"enabled"`
	Capacity    int   `json:"capacity"`
	Depth       int   `json:"depth"`
	Enqueued    int64 `json:"enqueued"`
	Flushed     int64 `json:"flushed"`
	Dropped     int64 `json:"dropped"`
	FlushErrors int64 `json:"flushErrors"`
}

type HealthDeepResponse struct {
	CHReachable      bool              `json:"chReachable"`
	CHUptimeSec      int64             `json:"chUptimeSec"`
	PartsCount       int64             `json:"partsCount"`
	PartsByTable     []TableParts      `json:"partsByTable,omitempty"`
	ActiveMerges     int64             `json:"activeMerges"`
	LongestMergeSec  float64           `json:"longestMergeSec"`
	ErrorsRecent     []CHError         `json:"errorsRecent,omitempty"`
	MemoryUsageBytes int64             `json:"memoryUsageBytes,omitempty"`
	MemoryTotalBytes int64             `json:"memoryTotalBytes,omitempty"`
	MetricBuffer     *MetricBufferInfo `json:"metricBuffer,omitempty"`
}

func (h healthDeepController) Get(c *gin.Context) {
	resp := fetchCHHealth(c.Request.Context())
	if !resp.CHReachable {
		c.JSON(http.StatusServiceUnavailable, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}

var HealthDeepController = healthDeepController{}
