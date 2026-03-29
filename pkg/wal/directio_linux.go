//go:build linux

package wal

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openWALDirect opens a WAL file with O_DIRECT flag for bypassing OS page cache.
// On Linux, this uses the O_DIRECT flag which requires aligned I/O.
func openWALDirect(path string) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_APPEND | unix.O_DIRECT

	fd, err := unix.Open(path, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file with Direct I/O: %w", err)
	}

	return os.NewFile(uintptr(fd), path), nil
}
