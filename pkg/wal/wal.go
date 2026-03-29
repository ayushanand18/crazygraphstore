// Package wal provides Write-Ahead Logging for durability.
package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// WAL represents a Write-Ahead Log for a single write lane.
type WAL struct {
	file       *os.File
	writer     *bufio.Writer
	path       string
	laneID     int
	offset     atomic.Uint64
	syncTicker *time.Ticker
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	closed     atomic.Bool

	// Direct I/O support
	useDirectIO   bool
	alignedBuffer *alignedBuffer

	// Statistics
	totalWrites  atomic.Uint64
	totalBytes   atomic.Uint64
	totalSyncs   atomic.Uint64
	lastSyncTime atomic.Value // time.Time
}

// alignedBuffer provides 512-byte aligned memory for Direct I/O operations.
// Direct I/O requires both the memory address and the I/O size to be aligned
// to the filesystem block size (typically 512 bytes).
type alignedBuffer struct {
	data      []byte // The underlying allocated buffer (larger for alignment)
	aligned   []byte // The aligned slice within data
	pos       int    // Current write position
	blockSize int    // Alignment block size (typically 512)
}

// newAlignedBuffer creates a new aligned buffer with the given capacity.
// The actual capacity may be slightly larger to ensure alignment.
func newAlignedBuffer(capacity, blockSize int) *alignedBuffer {
	// Allocate extra space for alignment
	data := make([]byte, capacity+blockSize)

	// Find the aligned start position
	ptr := uintptr(unsafe.Pointer(&data[0]))
	alignmentOffset := int((blockSize - int(ptr%uintptr(blockSize))) % blockSize)

	return &alignedBuffer{
		data:      data,
		aligned:   data[alignmentOffset : alignmentOffset+capacity],
		pos:       0,
		blockSize: blockSize,
	}
}

// Write writes data to the aligned buffer.
func (ab *alignedBuffer) Write(p []byte) (n int, err error) {
	if ab.pos+len(p) > len(ab.aligned) {
		return 0, fmt.Errorf("aligned buffer overflow: need %d bytes, have %d", len(p), len(ab.aligned)-ab.pos)
	}
	copy(ab.aligned[ab.pos:], p)
	ab.pos += len(p)
	return len(p), nil
}

// Flush writes the buffered data to the file with proper alignment.
// For Direct I/O, writes must be in multiples of blockSize.
func (ab *alignedBuffer) Flush(file *os.File) error {
	if ab.pos == 0 {
		return nil
	}

	// Calculate the aligned write size (round up to blockSize)
	alignedSize := ((ab.pos + ab.blockSize - 1) / ab.blockSize) * ab.blockSize

	// Zero-pad the remaining space in the final block
	for i := ab.pos; i < alignedSize; i++ {
		ab.aligned[i] = 0
	}

	// Write the aligned data
	_, err := file.Write(ab.aligned[:alignedSize])
	if err != nil {
		return err
	}

	ab.pos = 0
	return nil
}

// Reset clears the buffer.
func (ab *alignedBuffer) Reset() {
	ab.pos = 0
}

// Len returns the current data length in the buffer.
func (ab *alignedBuffer) Len() int {
	return ab.pos
}

// EntryType represents the type of WAL entry.
type EntryType byte

const (
	EntryTypeNode   EntryType = 1
	EntryTypeEdge   EntryType = 2
	EntryTypeDelete EntryType = 3
)

// Entry represents a single WAL entry.
type Entry struct {
	Type      EntryType
	Key       string
	Value     []byte
	Checksum  uint32
	Timestamp int64
}

// Config holds WAL configuration.
type Config struct {
	DataDir      string
	SyncInterval time.Duration
	BufferSize   int
	UseDirectIO  bool // Enable O_DIRECT for bypassing OS page cache
}

// DefaultConfig returns default WAL configuration.
func DefaultConfig() *Config {
	return &Config{
		DataDir:      "./data/wal",
		SyncInterval: 10 * time.Millisecond, // Group commit every 10ms
		BufferSize:   256 * 1024,            // 256KB buffer
		UseDirectIO:  false,                 // Disabled by default for compatibility
	}
}

// DirectIOBlockSize is the alignment requirement for Direct I/O operations.
// Most filesystems require 512-byte alignment.
const DirectIOBlockSize = 512

// NewWAL creates a new Write-Ahead Log for a specific lane.
func NewWAL(laneID int, config *Config) (*WAL, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create WAL directory
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Generate WAL file path
	path := filepath.Join(config.DataDir, fmt.Sprintf("lane-%d.wal", laneID))

	var file *os.File
	var err error

	// Open WAL file (with or without Direct I/O)
	if config.UseDirectIO {
		file, err = openWALDirect(path)
		if err != nil {
			return nil, err
		}
	} else {
		// Standard file open (append mode)
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open WAL file: %w", err)
		}
	}

	// Get current file size (offset)
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat WAL file: %w", err)
	}

	wal := &WAL{
		file:        file,
		path:        path,
		laneID:      laneID,
		syncTicker:  time.NewTicker(config.SyncInterval),
		stopCh:      make(chan struct{}),
		useDirectIO: config.UseDirectIO,
	}

	// Setup buffering based on Direct I/O mode
	if config.UseDirectIO {
		wal.alignedBuffer = newAlignedBuffer(config.BufferSize, DirectIOBlockSize)
		wal.writer = nil // Not used with Direct I/O
	} else {
		wal.writer = bufio.NewWriterSize(file, config.BufferSize)
		wal.alignedBuffer = nil
	}

	wal.offset.Store(uint64(stat.Size()))
	wal.lastSyncTime.Store(time.Now())

	// Start background sync goroutine
	wal.wg.Add(1)
	go wal.backgroundSync()

	return wal, nil
}

// Append writes an entry to the WAL.
func (w *WAL) Append(entryType EntryType, key string, value []byte) error {
	if w.closed.Load() {
		return fmt.Errorf("wal is closed")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	entry := &Entry{
		Type:      entryType,
		Key:       key,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
	}

	// Serialize entry
	data, err := w.serializeEntry(entry)
	if err != nil {
		return fmt.Errorf("failed to serialize entry: %w", err)
	}

	// Write to appropriate buffer
	if w.useDirectIO {
		if _, err := w.alignedBuffer.Write(data); err != nil {
			return fmt.Errorf("failed to write to aligned buffer: %w", err)
		}
	} else {
		if _, err := w.writer.Write(data); err != nil {
			return fmt.Errorf("failed to write to WAL: %w", err)
		}
	}

	// Update statistics
	w.totalWrites.Add(1)
	w.totalBytes.Add(uint64(len(data)))
	w.offset.Add(uint64(len(data)))

	return nil
}

// serializeEntry converts an entry to bytes.
// Format: [Type(1)][KeyLen(4)][Key][ValueLen(4)][Value][Timestamp(8)][Checksum(4)]
func (w *WAL) serializeEntry(entry *Entry) ([]byte, error) {
	keyLen := len(entry.Key)
	valueLen := len(entry.Value)
	totalLen := 1 + 4 + keyLen + 4 + valueLen + 8 + 4

	buf := make([]byte, totalLen)
	offset := 0

	// Type
	buf[offset] = byte(entry.Type)
	offset++

	// Key length
	binary.LittleEndian.PutUint32(buf[offset:], uint32(keyLen))
	offset += 4

	// Key
	copy(buf[offset:], entry.Key)
	offset += keyLen

	// Value length
	binary.LittleEndian.PutUint32(buf[offset:], uint32(valueLen))
	offset += 4

	// Value
	copy(buf[offset:], entry.Value)
	offset += valueLen

	// Timestamp
	binary.LittleEndian.PutUint64(buf[offset:], uint64(entry.Timestamp))
	offset += 8

	// Checksum (CRC32 of all previous data)
	checksum := crc32.ChecksumIEEE(buf[:offset])
	binary.LittleEndian.PutUint32(buf[offset:], checksum)

	return buf, nil
}

// backgroundSync periodically syncs the WAL to disk.
func (w *WAL) backgroundSync() {
	defer w.wg.Done()

	for {
		select {
		case <-w.syncTicker.C:
			if err := w.Sync(); err != nil {
				// Log error (in production, use proper logging)
				fmt.Printf("WAL sync error for lane %d: %v\n", w.laneID, err)
			}

		case <-w.stopCh:
			// Final sync before exit
			w.Sync()
			return
		}
	}
}

// Sync flushes the buffer and syncs to disk.
func (w *WAL) Sync() error {
	if w.closed.Load() {
		return fmt.Errorf("wal is closed")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush buffer based on mode
	if w.useDirectIO {
		if err := w.alignedBuffer.Flush(w.file); err != nil {
			return fmt.Errorf("failed to flush aligned buffer: %w", err)
		}
	} else {
		if err := w.writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
	}

	// Sync to disk (fsync)
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	w.totalSyncs.Add(1)
	w.lastSyncTime.Store(time.Now())

	return nil
}

// Close closes the WAL.
func (w *WAL) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(w.stopCh)
	w.wg.Wait()

	// Final sync without the public closed check.
	w.mu.Lock()
	if w.useDirectIO {
		if err := w.alignedBuffer.Flush(w.file); err != nil {
			w.mu.Unlock()
			return fmt.Errorf("failed to flush aligned buffer: %w", err)
		}
	} else {
		if err := w.writer.Flush(); err != nil {
			w.mu.Unlock()
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
	}
	if err := w.file.Sync(); err != nil {
		w.mu.Unlock()
		return fmt.Errorf("failed to sync file: %w", err)
	}
	w.mu.Unlock()

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close WAL file: %w", err)
	}

	return nil
}

// Replay reads all entries from the WAL and applies them using the provided function.
func (w *WAL) Replay(fn func(entry *Entry) error) error {
	// Open file for reading
	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No WAL file yet
		}
		return fmt.Errorf("failed to open WAL for replay: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	offset := 0

	for {
		entry, bytesRead, err := w.deserializeEntry(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to deserialize entry at offset %d: %w", offset, err)
		}

		// Apply entry
		if err := fn(entry); err != nil {
			return fmt.Errorf("failed to apply entry at offset %d: %w", offset, err)
		}

		offset += bytesRead
	}

	return nil
}

// deserializeEntry reads and deserializes a single entry from the reader.
func (w *WAL) deserializeEntry(reader *bufio.Reader) (*Entry, int, error) {
	// Read type
	typeByte, err := reader.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	bytesRead := 1

	// Read key length
	keyLenBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, keyLenBuf); err != nil {
		return nil, bytesRead, err
	}
	keyLen := binary.LittleEndian.Uint32(keyLenBuf)
	bytesRead += 4

	// Read key
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, bytesRead, err
	}
	bytesRead += int(keyLen)

	// Read value length
	valueLenBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, valueLenBuf); err != nil {
		return nil, bytesRead, err
	}
	valueLen := binary.LittleEndian.Uint32(valueLenBuf)
	bytesRead += 4

	// Read value
	value := make([]byte, valueLen)
	if _, err := io.ReadFull(reader, value); err != nil {
		return nil, bytesRead, err
	}
	bytesRead += int(valueLen)

	// Read timestamp
	timestampBuf := make([]byte, 8)
	if _, err := io.ReadFull(reader, timestampBuf); err != nil {
		return nil, bytesRead, err
	}
	timestamp := int64(binary.LittleEndian.Uint64(timestampBuf))
	bytesRead += 8

	// Read checksum
	checksumBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, checksumBuf); err != nil {
		return nil, bytesRead, err
	}
	storedChecksum := binary.LittleEndian.Uint32(checksumBuf)
	bytesRead += 4

	// Verify checksum
	entry := &Entry{
		Type:      EntryType(typeByte),
		Key:       string(key),
		Value:     value,
		Timestamp: timestamp,
		Checksum:  storedChecksum,
	}

	// Recompute checksum to verify
	data, _ := w.serializeEntry(entry)
	expectedChecksum := binary.LittleEndian.Uint32(data[len(data)-4:])
	if storedChecksum != expectedChecksum {
		return nil, bytesRead, fmt.Errorf("checksum mismatch: expected %d, got %d", expectedChecksum, storedChecksum)
	}

	return entry, bytesRead, nil
}

// Truncate removes all entries from the WAL.
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Close current file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close WAL file: %w", err)
	}

	var file *os.File
	var err error

	// Reopen file (with or without Direct I/O)
	if w.useDirectIO {
		// For Direct I/O, we need to remove and recreate the file
		if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove WAL file: %w", err)
		}
		file, err = openWALDirect(w.path)
		if err != nil {
			return err
		}
		w.alignedBuffer.Reset()
	} else {
		// Standard truncate
		file, err = os.OpenFile(w.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to truncate WAL file: %w", err)
		}
		w.writer = bufio.NewWriterSize(file, 256*1024)
	}

	w.file = file
	w.offset.Store(0)

	return nil
}

// Stats returns WAL statistics.
type WALStats struct {
	LaneID       int
	TotalWrites  uint64
	TotalBytes   uint64
	TotalSyncs   uint64
	LastSyncTime time.Time
	CurrentSize  uint64
}

// Stats returns current WAL statistics.
func (w *WAL) Stats() WALStats {
	lastSync := w.lastSyncTime.Load()
	var lastSyncTime time.Time
	if lastSync != nil {
		lastSyncTime = lastSync.(time.Time)
	}

	return WALStats{
		LaneID:       w.laneID,
		TotalWrites:  w.totalWrites.Load(),
		TotalBytes:   w.totalBytes.Load(),
		TotalSyncs:   w.totalSyncs.Load(),
		LastSyncTime: lastSyncTime,
		CurrentSize:  w.offset.Load(),
	}
}
