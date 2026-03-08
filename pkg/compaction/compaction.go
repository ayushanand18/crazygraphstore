
// Package compaction provides SSTable compaction to reduce read amplification.
package compaction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/sstable"
)

// Strategy represents the compaction strategy.
type Strategy int

const (
	StrategySizeTiered Strategy = iota // Merge SSTables of similar size
	StrategyLeveled                     // Leveled compaction (RocksDB-style)
)

// Compactor manages SSTable compaction.
type Compactor struct {
	dataDir       string
	strategy      Strategy
	maxTableSize  int64
	running       atomic.Bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
	
	// Tables being managed
	tables     []*TableInfo
	tablesMu   sync.RWMutex
	
	// Statistics
	totalCompactions atomic.Uint64
	totalBytesRead   atomic.Uint64
	totalBytesWritten atomic.Uint64
	lastCompactionTime atomic.Value // time.Time
}

// TableInfo holds metadata about an SSTable.
type TableInfo struct {
	Path      string
	Size      int64
	Level     int
	CreatedAt time.Time
	NumEntries uint64
}

// Config holds compaction configuration.
type Config struct {
	DataDir       string
	Strategy      Strategy
	MaxTableSize  int64
	CheckInterval time.Duration
}

// DefaultConfig returns default compaction configuration.
func DefaultConfig() *Config {
	return &Config{
		DataDir:       "./data/sstables",
		Strategy:      StrategySizeTiered,
		MaxTableSize:  64 * 1024 * 1024, // 64MB
		CheckInterval: 30 * time.Second,
	}
}

// NewCompactor creates a new compaction manager.
func NewCompactor(config *Config) (*Compactor, error) {
	if config == nil {
		config = DefaultConfig()
	}

	return &Compactor{
		dataDir:      config.DataDir,
		strategy:     config.Strategy,
		maxTableSize: config.MaxTableSize,
		stopCh:       make(chan struct{}),
		tables:       make([]*TableInfo, 0),
	}, nil
}

// Start starts the background compaction process.
func (c *Compactor) Start(ctx context.Context, interval time.Duration) error {
	if !c.running.CompareAndSwap(false, true) {
		return fmt.Errorf("compactor already running")
	}

	c.wg.Add(1)
	go c.backgroundCompaction(ctx, interval)

	return nil
}

// Stop stops the compaction process.
func (c *Compactor) Stop() error {
	if !c.running.CompareAndSwap(true, false) {
		return fmt.Errorf("compactor not running")
	}

	close(c.stopCh)
	c.wg.Wait()

	return nil
}

// RegisterTable adds a new SSTable to be managed by the compactor.
func (c *Compactor) RegisterTable(path string, size int64, numEntries uint64) {
	c.tablesMu.Lock()
	defer c.tablesMu.Unlock()

	info := &TableInfo{
		Path:       path,
		Size:       size,
		Level:      0, // New tables start at level 0
		CreatedAt:  time.Now(),
		NumEntries: numEntries,
	}

	c.tables = append(c.tables, info)
}

// backgroundCompaction runs the compaction loop.
func (c *Compactor) backgroundCompaction(ctx context.Context, interval time.Duration) {
	defer c.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.runCompaction(); err != nil {
				// Log error (in production, use proper logging)
				fmt.Printf("Compaction error: %v\n", err)
			}

		case <-c.stopCh:
			return

		case <-ctx.Done():
			return
		}
	}
}

// runCompaction performs a single compaction cycle.
func (c *Compactor) runCompaction() error {
	c.tablesMu.Lock()
	defer c.tablesMu.Unlock()

	// Check if compaction is needed
	if !c.shouldCompact() {
		return nil
	}

	// Select tables to compact based on strategy
	var tablesToCompact []*TableInfo
	switch c.strategy {
	case StrategySizeTiered:
		tablesToCompact = c.selectTablesForSizeTieredCompaction()
	case StrategyLeveled:
		tablesToCompact = c.selectTablesForLeveledCompaction()
	default:
		return fmt.Errorf("unknown compaction strategy: %d", c.strategy)
	}

	if len(tablesToCompact) < 2 {
		return nil // Need at least 2 tables to compact
	}

	// Perform compaction
	newTable, err := c.compactTables(tablesToCompact)
	if err != nil {
		return fmt.Errorf("failed to compact tables: %w", err)
	}

	// Update table list
	c.removeTables(tablesToCompact)
	c.tables = append(c.tables, newTable)

	// Update statistics
	c.totalCompactions.Add(1)
	c.lastCompactionTime.Store(time.Now())

	return nil
}

// shouldCompact checks if compaction is needed.
func (c *Compactor) shouldCompact() bool {
	// Compact if we have more than 4 tables at level 0
	level0Count := 0
	for _, table := range c.tables {
		if table.Level == 0 {
			level0Count++
		}
	}

	return level0Count > 4
}

// selectTablesForSizeTieredCompaction selects tables of similar size.
func (c *Compactor) selectTablesForSizeTieredCompaction() []*TableInfo {
	if len(c.tables) < 2 {
		return nil
	}

	// Sort tables by size
	sorted := make([]*TableInfo, len(c.tables))
	copy(sorted, c.tables)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Size < sorted[j].Size
	})

	// Find a group of tables with similar size (within 2x)
	var group []*TableInfo
	for i := 0; i < len(sorted)-1; i++ {
		group = []*TableInfo{sorted[i]}
		
		for j := i + 1; j < len(sorted) && len(group) < 4; j++ {
			// Check if size is within 2x
			if sorted[j].Size <= sorted[i].Size*2 {
				group = append(group, sorted[j])
			}
		}

		if len(group) >= 2 {
			return group
		}
	}

	return nil
}

// selectTablesForLeveledCompaction selects tables for leveled compaction.
func (c *Compactor) selectTablesForLeveledCompaction() []*TableInfo {
	// Find all level 0 tables
	var level0Tables []*TableInfo
	for _, table := range c.tables {
		if table.Level == 0 {
			level0Tables = append(level0Tables, table)
		}
	}

	if len(level0Tables) >= 4 {
		return level0Tables[:4] // Compact first 4 level 0 tables
	}

	return nil
}

// compactTables merges multiple SSTables into a new one.
func (c *Compactor) compactTables(tables []*TableInfo) (*TableInfo, error) {
	// Generate output filename
	outputPath := filepath.Join(c.dataDir, fmt.Sprintf("compacted-%d.sst", time.Now().UnixNano()))

	// Count total entries for bloom filter sizing
	totalEntries := uint64(0)
	for _, table := range tables {
		totalEntries += table.NumEntries
	}

	// Create writer
	writer, err := sstable.NewWriter(outputPath, uint(totalEntries))
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}
	defer writer.Close()

	// Merge tables using a min-heap (k-way merge)
	merger := newMerger(tables)
	
	for {
		key, value, err := merger.Next()
		if err != nil {
			if err == ErrMergerDone {
				break
			}
			return nil, fmt.Errorf("merger error: %w", err)
		}

		// Write to new table
		if err := writer.Add(key, value); err != nil {
			return nil, fmt.Errorf("failed to write entry: %w", err)
		}

		c.totalBytesRead.Add(uint64(len(key) + len(value)))
	}

	// Close writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Get file size
	stat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	c.totalBytesWritten.Add(uint64(stat.Size()))

	// Create table info
	newTable := &TableInfo{
		Path:       outputPath,
		Size:       stat.Size(),
		Level:      c.calculateNewLevel(tables),
		CreatedAt:  time.Now(),
		NumEntries: writer.EntriesCount(),
	}

	return newTable, nil
}

// calculateNewLevel determines the level for the compacted table.
func (c *Compactor) calculateNewLevel(tables []*TableInfo) int {
	maxLevel := 0
	for _, table := range tables {
		if table.Level > maxLevel {
			maxLevel = table.Level
		}
	}

	// New table goes to next level
	return maxLevel + 1
}

// removeTables removes tables from the managed list and deletes files.
func (c *Compactor) removeTables(toRemove []*TableInfo) {
	// Create a set of paths to remove
	removeSet := make(map[string]bool)
	for _, table := range toRemove {
		removeSet[table.Path] = true
	}

	// Filter tables
	newTables := make([]*TableInfo, 0, len(c.tables)-len(toRemove))
	for _, table := range c.tables {
		if !removeSet[table.Path] {
			newTables = append(newTables, table)
		} else {
			// Delete file
			os.Remove(table.Path)
		}
	}

	c.tables = newTables
}

// Stats returns compaction statistics.
type CompactionStats struct {
	TotalCompactions   uint64
	TotalBytesRead     uint64
	TotalBytesWritten  uint64
	LastCompactionTime time.Time
	NumTables          int
	TotalTableSize     int64
}

// Stats returns current compaction statistics.
func (c *Compactor) Stats() CompactionStats {
	c.tablesMu.RLock()
	defer c.tablesMu.RUnlock()

	totalSize := int64(0)
	for _, table := range c.tables {
		totalSize += table.Size
	}

	lastCompaction := c.lastCompactionTime.Load()
	var lastCompactionTime time.Time
	if lastCompaction != nil {
		lastCompactionTime = lastCompaction.(time.Time)
	}

	return CompactionStats{
		TotalCompactions:   c.totalCompactions.Load(),
		TotalBytesRead:     c.totalBytesRead.Load(),
		TotalBytesWritten:  c.totalBytesWritten.Load(),
		LastCompactionTime: lastCompactionTime,
		NumTables:          len(c.tables),
		TotalTableSize:     totalSize,
	}
}

// GetTables returns a copy of the current table list.
func (c *Compactor) GetTables() []*TableInfo {
	c.tablesMu.RLock()
	defer c.tablesMu.RUnlock()

	tables := make([]*TableInfo, len(c.tables))
	copy(tables, c.tables)
	return tables
}
