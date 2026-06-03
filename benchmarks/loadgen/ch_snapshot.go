package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type tableParts struct {
	Table string `json:"table"`
	Parts int64  `json:"parts"`
}

type chError struct {
	Name          string `json:"name"`
	Value         int64  `json:"value"`
	LastErrorTime string `json:"lastErrorTime,omitempty"`
}

type chSnapshot struct {
	Reachable        bool         `json:"reachable"`
	UptimeSec        int64        `json:"uptimeSec"`
	PartsCount       int64        `json:"partsCount"`
	PartsByTable     []tableParts `json:"partsByTable,omitempty"`
	ActiveMerges     int64        `json:"activeMerges"`
	LongestMergeSec  float64      `json:"longestMergeSec"`
	ErrorsRecent     []chError    `json:"errorsRecent,omitempty"`
	MemoryUsageBytes int64        `json:"memoryUsageBytes,omitempty"`
	MemoryPeakBytes  int64        `json:"memoryPeakBytes,omitempty"`
	MemoryTotalBytes int64        `json:"memoryTotalBytes,omitempty"`
	IngestedRows      int64       `json:"ingestedRows,omitempty"`
	TelemetryDBBytes  int64       `json:"telemetryDbBytes,omitempty"`
	TelemetryWALBytes int64       `json:"telemetryWalBytes,omitempty"`
}

// healthDeepBody mirrors the backend HealthDeepResponse with its JSON tags. The
// backend uses `chReachable`/`chUptimeSec`; we expose those as `reachable`/
// `uptimeSec` in our embedded snapshot so the bench JSON stays consistent with
// other loadgen fields.
type healthDeepBody struct {
	CHReachable      bool         `json:"chReachable"`
	CHUptimeSec      int64        `json:"chUptimeSec"`
	PartsCount       int64        `json:"partsCount"`
	PartsByTable     []tableParts `json:"partsByTable"`
	ActiveMerges     int64        `json:"activeMerges"`
	LongestMergeSec  float64      `json:"longestMergeSec"`
	ErrorsRecent     []chError    `json:"errorsRecent"`
	MemoryUsageBytes int64        `json:"memoryUsageBytes"`
	MemoryPeakBytes  int64        `json:"memoryPeakBytes"`
	MemoryTotalBytes int64        `json:"memoryTotalBytes"`
	IngestedRows      int64       `json:"ingestedRows"`
	TelemetryDBBytes  int64       `json:"telemetryDbBytes"`
	TelemetryWALBytes int64       `json:"telemetryWalBytes"`
}

// fetchCHSnapshot pings the backend's /health/deep endpoint and translates the
// payload into a chSnapshot. On any error (timeout, transport, non-2xx, decode)
// it returns a snapshot with Reachable=false and logs to stderrPrefix() — the
// caller does NOT fail the step on a missing snapshot. A 503 from the backend
// (CH unreachable but backend alive) is still parsed and embedded so the JSON
// captures the chReachable=false signal.
func fetchCHSnapshot(ctx context.Context, cfg config, client *http.Client) chSnapshot {
	if cfg.jwt == "" {
		return chSnapshot{}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cfg.target+"/api/health/deep", nil)
	if err != nil {
		fmt.Fprintf(stderrPrefix(), "fetchCHSnapshot: build request failed: %v\n", err)
		return chSnapshot{}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.jwt)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderrPrefix(), "fetchCHSnapshot: http error: %v\n", err)
		return chSnapshot{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		fmt.Fprintf(stderrPrefix(), "fetchCHSnapshot: unexpected status %d\n", resp.StatusCode)
		return chSnapshot{}
	}

	var body healthDeepBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		fmt.Fprintf(stderrPrefix(), "fetchCHSnapshot: decode failed: %v\n", err)
		return chSnapshot{}
	}

	return chSnapshot{
		Reachable:        body.CHReachable,
		UptimeSec:        body.CHUptimeSec,
		PartsCount:       body.PartsCount,
		PartsByTable:     body.PartsByTable,
		ActiveMerges:     body.ActiveMerges,
		LongestMergeSec:  body.LongestMergeSec,
		ErrorsRecent:     body.ErrorsRecent,
		MemoryUsageBytes: body.MemoryUsageBytes,
		MemoryPeakBytes:  body.MemoryPeakBytes,
		MemoryTotalBytes: body.MemoryTotalBytes,
		IngestedRows:      body.IngestedRows,
		TelemetryDBBytes:  body.TelemetryDBBytes,
		TelemetryWALBytes: body.TelemetryWALBytes,
	}
}

// bytesStr renders a byte count compactly (e.g. "120MiB", "1.4GiB"); "-" for 0.
func bytesStr(n int64) string {
	if n <= 0 {
		return "-"
	}
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0fMiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0fKiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// countStr renders a row count compactly (e.g. "1.2M", "950K"); "0" for 0.
func countStr(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// memStr renders the SUT memory snapshot compactly for the per-step log as
// current→peak/total, e.g. "1.0→5.9/7.6GiB". The peak (VmHWM high-water mark)
// is the important bit: a burst backlog can spike RSS mid-step and trip the
// OOM-killer, then drain before the end-of-step sample — so current can read
// low while peak shows how close it actually came. Returns "?" when the backend
// reported no memory (older build, or the snapshot failed because it was down).
func memStr(ch chSnapshot) string {
	if ch.MemoryUsageBytes <= 0 {
		return "?"
	}
	g := func(b int64) float64 { return float64(b) / (1 << 30) }
	used := g(ch.MemoryUsageBytes)

	if ch.MemoryTotalBytes > 0 {
		if ch.MemoryPeakBytes > ch.MemoryUsageBytes {
			return fmt.Sprintf("%.1f→%.1f/%.1fGiB", used, g(ch.MemoryPeakBytes), g(ch.MemoryTotalBytes))
		}
		return fmt.Sprintf("%.1f/%.1fGiB", used, g(ch.MemoryTotalBytes))
	}
	if used < 1 {
		return fmt.Sprintf("%.0fMiB", float64(ch.MemoryUsageBytes)/(1<<20))
	}
	return fmt.Sprintf("%.1fGiB", used)
}
