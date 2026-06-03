//go:build !pgch

package controllers

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// fetchCHHealth is the embedded-mode (SQLite/DuckDB) /health/deep payload. There
// is no ClickHouse, so CHReachable stays false, but it reports this process's
// memory and the box total so a benchmark can watch the backend climb toward
// OOM (or just back up) during ingestion.
func fetchCHHealth(_ context.Context) HealthDeepResponse {
	return HealthDeepResponse{
		CHReachable:      false,
		MemoryUsageBytes: processRSSBytes(),
		MemoryTotalBytes: systemTotalMemoryBytes(),
	}
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
