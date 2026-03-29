//go:build !linux && !darwin

package wal

import (
	"fmt"
	"os"
)

// openWALDirect is a fallback implementation for unsupported platforms.
// It opens the file normally without Direct I/O support.
func openWALDirect(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file (Direct I/O not supported on this platform): %w", err)
	}
	return file, nil
}
