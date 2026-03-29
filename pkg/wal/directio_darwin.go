//go:build darwin

package wal

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openWALDirect opens a WAL file with cache bypassing for Direct I/O.
// On macOS, we use F_NOCACHE via fcntl to disable caching since O_DIRECT
// is not available.
func openWALDirect(path string) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_APPEND

	fd, err := unix.Open(path, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	file := os.NewFile(uintptr(fd), path)

	// Use F_NOCACHE to disable caching on macOS
	_, err = unix.FcntlInt(uintptr(fd), unix.F_NOCACHE, 1)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to set F_NOCACHE on WAL file: %w", err)
	}

	return file, nil
}
