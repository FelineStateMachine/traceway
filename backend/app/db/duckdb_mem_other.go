//go:build duckdb && !pgch && !linux && !darwin

package db

func totalPhysicalMemory() uint64 { return 0 }
