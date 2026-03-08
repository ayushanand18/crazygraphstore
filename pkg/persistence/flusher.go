
// Package persistence provides background flushing of memtables to disk.
package persistence

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/storage-engine-graph-db/pkg/memtable"
	"github.com/storage-engine-graph-db/pkg/sstable"
)

// Flusher manages background flushing of memtables to SSTables.
type Flusher struct {
	dataDir       string
	flushInterval time.Duration
	running       atomic.Bool
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	
	// Statistics
	totalFlushes   atomic.Uint64
	totalBytes     atomic.Uint64
	totalEntries   atomic.Uint64
	failedFlushes  atomic.Uint64
}

// Config holds flusher configuration.
type Config struct {
	DataDir       string        // Directory to write SSTables
	FlushInterval time.Duration // How often to check for memtables to flush
}

// DefaultConfig returns default flusher configuration.
func DefaultConfig() *Config {
	return &Config{
		DataDir:       "./data/sstables",
		FlushInterval: 5 * time.Second,
	}
}

// NewFlusher creates a new background flusher.
func NewFlusher(config *Config) (*Flusher, error) {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Flusher{
		dataDir:       config.DataDir,
		flushInterval: config.FlushInterval,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// Start starts the background flusher.
func (f *Flusher) Start() error {
	if !f.running.CompareAndSwap(false, true) {
		return fmt.Errorf("flusher already running")
	}

	return nil
}

// Stop stops the background flusher.
func (f *Flusher) Stop() error {
	if !f.running.CompareAndSwap(true, false) {
		return fmt.Errorf("flusher not running")
	}

	f.cancel()
	f.wg.Wait()

	return nil
}

// FlushMemtable flushes a single memtable to an SSTable.
func (f *Flusher) FlushMemtable(mt *memtable.Memtable, laneID int) error {
	if !f.running.Load() {
		return fmt.Errorf("flusher not running")
	}

	// Generate SSTable filename
	timestamp := time.Now().UnixNano()
	filename := filepath.Join(f.dataDir, fmt.Sprintf("lane-%d-%d.sst", laneID, timestamp))

	// Get all entries from memtable
	entries := mt.Entries()
	if len(entries) == 0 {
		return nil // Nothing to flush
	}

	// Create SSTable writer
	writer, err := sstable.NewWriter(filename, uint(len(entries)))
	if err != nil {
		f.failedFlushes.Add(1)
		return fmt.Errorf("failed to create SSTable writer: %w", err)
	}
	defer writer.Close()

	// Write all entries
	for _, entry := range entries {
		if err := writer.Add(entry.Key, entry.Value); err != nil {
			f.failedFlushes.Add(1)
			return fmt.Errorf("failed to write entry: %w", err)
		}
	}

	// Close writer (writes footer)
	if err := writer.Close(); err != nil {
		f.failedFlushes.Add(1)
		return fmt.Errorf("failed to close SSTable: %w", err)
	}

	// Update statistics
	f.totalFlushes.Add(1)
	f.totalBytes.Add(uint64(writer.FileSize()))
	f.totalEntries.Add(writer.EntriesCount())

	return nil
}

// FlushNodeMemtable flushes a node memtable to an SSTable.
func (f *Flusher) FlushNodeMemtable(mt *memtable.NodeMemtable, laneID int) error {
	return f.FlushMemtable(mt.Memtable, laneID)
}

// FlushEdgeMemtable flushes an edge memtable to an SSTable.
func (f *Flusher) FlushEdgeMemtable(mt *memtable.EdgeMemtable, laneID int) error {
	return f.FlushMemtable(mt.Memtable, laneID)
}

// Stats returns flusher statistics.
type FlusherStats struct {
	TotalFlushes  uint64
	TotalBytes    uint64
	TotalEntries  uint64
	FailedFlushes uint64
	Running       bool
}

// Stats returns current flusher statistics.
func (f *Flusher) Stats() FlusherStats {
	return FlusherStats{
		TotalFlushes:  f.totalFlushes.Load(),
		TotalBytes:    f.totalBytes.Load(),
		TotalEntries:  f.totalEntries.Load(),
		FailedFlushes: f.failedFlushes.Load(),
		Running:       f.running.Load(),
	}
}

// FlushJob represents a single flush job.
type FlushJob struct {
	LaneID    int
	Memtable  *memtable.Memtable
	Timestamp time.Time
}

// ScheduleFlush schedules a memtable for flushing.
func (f *Flusher) ScheduleFlush(job *FlushJob) {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()

		if err := f.FlushMemtable(job.Memtable, job.LaneID); err != nil {
			// Log error (in production, use proper logging)
			fmt.Printf("Error flushing memtable for lane %d: %v\n", job.LaneID, err)
		}
	}()
}

// WaitForFlushes waits for all pending flush jobs to complete.
func (f *Flusher) WaitForFlushes() {
	f.wg.Wait()
}

// DataDir returns the data directory path.
func (f *Flusher) DataDir() string {
	return f.dataDir
}
