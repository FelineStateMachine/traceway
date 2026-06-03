//go:build !pgch

package controllers

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/db"
)

// fetchCHHealth is the embedded-mode (SQLite/DuckDB) /health/deep payload. There
// is no ClickHouse, so CHReachable stays false, but it reports this process's
// memory plus telemetry-store progress (rows ingested, DB + WAL file sizes) so a
// benchmark can watch the backend climb toward OOM or fall behind on ingestion.
func fetchCHHealth(_ context.Context) HealthDeepResponse {
	dbBytes, walBytes := telemetryFileSizes()
	return HealthDeepResponse{
		CHReachable:       false,
		MemoryUsageBytes:  processRSSBytes(),
		MemoryPeakBytes:   processPeakRSSBytes(),
		MemoryTotalBytes:  systemTotalMemoryBytes(),
		IngestedRows:      db.IngestedTelemetryRows(),
		TelemetryDBBytes:  dbBytes,
		TelemetryWALBytes: walBytes,
	}
}

// processPeakRSSBytes returns the peak resident memory the process has ever hit
// (Linux VmHWM high-water mark). This is the figure that matters for OOM: a
// burst backlog can spike RSS mid-step and trip the OOM-killer, then drain
// before the next snapshot — so current RSS reads low while the peak reveals how
// close it came. Falls back to current RSS where VmHWM is unavailable.
func processPeakRSSBytes() int64 {
	if kb := scanKVFileKB("/proc/self/status", "VmHWM:"); kb > 0 {
		return kb * 1024
	}
	return processRSSBytes()
}

// telemetryFileSizes returns the on-disk size of the telemetry DB file and its
// WAL sidecar (0 each for in-memory). The WAL is what DuckDB/SQLite checkpoint
// into the main file; a WAL that keeps growing signals checkpoints lagging the
// write rate.
func telemetryFileSizes() (dbBytes, walBytes int64) {
	path := db.TelemetryFilePath
	if path == "" {
		return 0, 0
	}
	if fi, err := os.Stat(path); err == nil {
		dbBytes = fi.Size()
	}
	if fi, err := os.Stat(path + ".wal"); err == nil {
		walBytes = fi.Size()
	}
	return dbBytes, walBytes
}

// processRSSBytes returns this process's resident memory. On Linux it reads the
// VmRSS the kernel OOM-killer acts on; elsewhere it falls back to the Go
// runtime's reserved bytes, a high proxy that still tracks growth.
func processRSSBytes() int64 {
	if kb := scanKVFileKB("/proc/self/status", "VmRSS:"); kb > 0 {
		return kb * 1024
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}

func systemTotalMemoryBytes() int64 {
	if kb := scanKVFileKB("/proc/meminfo", "MemTotal:"); kb > 0 {
		return kb * 1024
	}
	return 0
}

// scanKVFileKB reads a /proc-style file and returns the kB integer on the line
// starting with key (e.g. "VmRSS:\t  12345 kB"); 0 if absent or unreadable.
func scanKVFileKB(path, key string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, key) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}
