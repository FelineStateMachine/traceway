//go:build duckdb && !pgch && linux

package db

import "golang.org/x/sys/unix"

func totalPhysicalMemory() uint64 {
	var si unix.Sysinfo_t
	if err := unix.Sysinfo(&si); err != nil {
		return 0
	}
	return uint64(si.Totalram) * uint64(si.Unit)
}
