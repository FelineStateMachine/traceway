//go:build duckdb && !pgch && darwin

package db

import "golang.org/x/sys/unix"

func totalPhysicalMemory() uint64 {
	v, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return v
}
