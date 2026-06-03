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

type HealthDeepResponse struct {
	CHReachable      bool         `json:"chReachable"`
	CHUptimeSec      int64        `json:"chUptimeSec"`
	PartsCount       int64        `json:"partsCount"`
	PartsByTable     []TableParts `json:"partsByTable,omitempty"`
	ActiveMerges     int64        `json:"activeMerges"`
	LongestMergeSec  float64      `json:"longestMergeSec"`
	ErrorsRecent     []CHError    `json:"errorsRecent,omitempty"`
	MemoryUsageBytes int64        `json:"memoryUsageBytes,omitempty"`
	MemoryPeakBytes  int64        `json:"memoryPeakBytes,omitempty"`
	MemoryTotalBytes int64        `json:"memoryTotalBytes,omitempty"`

	// Embedded-mode (SQLite/DuckDB) ingestion progress, so a benchmark can see
	// whether the store is keeping up: rows accepted since startup, the
	// telemetry DB file size, and the WAL backlog (a WAL that keeps growing
	// means checkpoints are falling behind the write rate).
	IngestedRows      int64 `json:"ingestedRows,omitempty"`
	TelemetryDBBytes  int64 `json:"telemetryDbBytes,omitempty"`
	TelemetryWALBytes int64 `json:"telemetryWalBytes,omitempty"`
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
