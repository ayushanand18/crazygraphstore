// Package storage provides the storage engine implementation.
package storage

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/cache"
	"github.com/ayushanand18/crazygraphstore/pkg/compaction"
	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/manifest"
	"github.com/ayushanand18/crazygraphstore/pkg/persistence"
	"github.com/ayushanand18/crazygraphstore/pkg/wal"
)

// Engine is the main storage engine with shared-nothing write lanes.
type Engine struct {
	// Write lanes (shared-nothing architecture)
	writeLanes []*WriteLane
	numLanes   int

	// Configuration
	memtableSize int64

	// Cache layer (Phase 2)
	cache *cache.MultiLevelCache

	// Persistence layer (Phase 2)
	flusher *persistence.Flusher

	// Phase 3: Durability and compaction
	compactor *compaction.Compactor
	manifest  *manifest.Manifest

	// State
	running atomic.Bool
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// Config holds engine configuration.
type Config struct {
	NumWriteLanes int    // Number of parallel write lanes
	MemtableSize  int64  // Size per memtable in bytes
	CacheSize     int64  // Total cache size in bytes (Phase 2)
	DataDir       string // Data directory for SSTables (Phase 2)
	WALDir        string // WAL directory (Phase 3)
	EnableWAL     bool   // Enable Write-Ahead Logging (Phase 3)
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		NumWriteLanes: 16,                     // 16 lanes for high concurrency
		MemtableSize:  64 * 1024 * 1024,       // 64MB per memtable
		CacheSize:     4 * 1024 * 1024 * 1024, // 4GB cache (Phase 2)
		DataDir:       "./data",               // Data directory (Phase 2)
		WALDir:        "./data/wal",           // WAL directory (Phase 3)
		EnableWAL:     true,                   // Enable WAL (Phase 3)
	}
}

// NewEngine creates a new storage engine.
func NewEngine(config *Config) (*Engine, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.NumWriteLanes <= 0 {
		return nil, fmt.Errorf("number of write lanes must be positive")
	}

	if config.MemtableSize <= 0 {
		return nil, fmt.Errorf("memtable size must be positive")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create cache (Phase 2)
	mlCache := cache.NewMultiLevelCache(config.CacheSize)

	// Create flusher (Phase 2)
	flusherConfig := &persistence.Config{
		DataDir: config.DataDir,
	}
	flusher, err := persistence.NewFlusher(flusherConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create flusher: %w", err)
	}

	// Create compactor (Phase 3)
	compactorConfig := compaction.DefaultConfig()
	compactorConfig.DataDir = config.DataDir + "/sstables"
	compactor, err := compaction.NewCompactor(compactorConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create compactor: %w", err)
	}

	// Create/load manifest (Phase 3)
	manifestConfig := &manifest.Config{
		DataDir: config.DataDir,
	}
	mani, err := manifest.NewManifest(manifestConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create manifest: %w", err)
	}

	engine := &Engine{
		numLanes:     config.NumWriteLanes,
		memtableSize: config.MemtableSize,
		cache:        mlCache,
		flusher:      flusher,
		compactor:    compactor,
		manifest:     mani,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Create WAL config (Phase 3)
	var walConfig *wal.Config
	if config.EnableWAL {
		walConfig = wal.DefaultConfig()
		walConfig.DataDir = config.WALDir
	}

	// Create write lanes
	engine.writeLanes = make([]*WriteLane, config.NumWriteLanes)
	for i := 0; i < config.NumWriteLanes; i++ {
		lane, err := NewWriteLane(i, config.MemtableSize, walConfig)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create write lane %d: %w", i, err)
		}
		engine.writeLanes[i] = lane
	}

	return engine, nil
}

// Start starts the storage engine.
func (e *Engine) Start() error {
	if !e.running.CompareAndSwap(false, true) {
		return fmt.Errorf("engine already running")
	}

	// Recover from WAL (Phase 3)
	if err := e.recover(); err != nil {
		e.running.Store(false)
		return fmt.Errorf("failed to recover from WAL: %w", err)
	}

	// Start flusher (Phase 2)
	if err := e.flusher.Start(); err != nil {
		e.running.Store(false)
		return fmt.Errorf("failed to start flusher: %w", err)
	}

	// Start compactor (Phase 3)
	if err := e.compactor.Start(e.ctx, 30 * time.Second); err != nil {
		e.running.Store(false)
		return fmt.Errorf("failed to start compactor: %w", err)
	}

	// Start background flusher workers for each lane
	for _, lane := range e.writeLanes {
		e.wg.Add(1)
		go e.runLaneFlusher(lane)
	}
	
	return nil
}

// Stop stops the storage engine gracefully.
func (e *Engine) Stop() error {
	if !e.running.CompareAndSwap(true, false) {
		return fmt.Errorf("engine not running")
	}

	// Signal all lanes to stop
	for _, lane := range e.writeLanes {
		lane.Stop()
	}

	// Cancel context
	e.cancel()

	// Wait for background workers
	e.wg.Wait()

	// Stop compactor (Phase 3)
	if err := e.compactor.Stop(); err != nil {
		return fmt.Errorf("failed to stop compactor: %w", err)
	}

	// Stop flusher (Phase 2)
	if err := e.flusher.Stop(); err != nil {
		return fmt.Errorf("failed to stop flusher: %w", err)
	}

	// Wait for any remaining flushes
	e.flusher.WaitForFlushes()

	// Save manifest (Phase 3)
	if err := e.manifest.Save(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

// CreateNode creates a new node in the graph.
func (e *Engine) CreateNode(ctx context.Context, node *graph.Node) error {
	if !e.running.Load() {
		return fmt.Errorf("engine not running")
	}

	// Get the write lane for this node (consistent hashing)
	lane := e.getWriteLaneForKey(node.ID)

	// Write to the lane (no locks!)
	return lane.WriteNode(ctx, node)
}

// GetNode retrieves a node by ID.
func (e *Engine) GetNode(ctx context.Context, nodeID string) (*graph.Node, error) {
	if !e.running.Load() {
		return nil, fmt.Errorf("engine not running")
	}

	// Check cache first (Phase 2)
	if node, ok := e.cache.GetNode(nodeID); ok {
		return node, nil
	}

	// Try the primary lane first
	lane := e.getWriteLaneForKey(nodeID)
	node, err := lane.ReadNode(ctx, nodeID)
	if err == nil {
		// Cache the node (Phase 2)
		e.cache.PutNode(node)
		return node, nil
	}

	// Node might be in a different lane due to hash collisions
	// Check all lanes (this is a fallback)
	for _, l := range e.writeLanes {
		if l == lane {
			continue
		}
		node, err := l.ReadNode(ctx, nodeID)
		if err == nil {
			// Cache the node (Phase 2)
			e.cache.PutNode(node)
			return node, nil
		}
	}

	return nil, fmt.Errorf("node not found: %s", nodeID)
}

// UpdateNode updates a node's properties.
func (e *Engine) UpdateNode(ctx context.Context, nodeID string, properties map[string]interface{}) error {
	if !e.running.Load() {
		return fmt.Errorf("engine not running")
	}

	// Get existing node
	node, err := e.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Update properties
	for key, value := range properties {
		node.SetProperty(key, value)
	}

	// Invalidate cache (Phase 2)
	e.cache.InvalidateNode(nodeID)

	// Write updated node
	lane := e.getWriteLaneForKey(nodeID)
	return lane.WriteNode(ctx, node)
}

// DeleteNode deletes a node from the graph.
func (e *Engine) DeleteNode(ctx context.Context, nodeID string) error {
	if !e.running.Load() {
		return fmt.Errorf("engine not running")
	}

	// TODO: Delete all edges connected to this node
	// TODO: Write tombstone to memtable

	return fmt.Errorf("not implemented")
}

// CreateEdge creates a new edge in the graph.
func (e *Engine) CreateEdge(ctx context.Context, edge *graph.Edge) error {
	if !e.running.Load() {
		return fmt.Errorf("engine not running")
	}

	// Verify that both nodes exist
	if _, err := e.GetNode(ctx, edge.FromNodeID); err != nil {
		return fmt.Errorf("source node not found: %w", err)
	}
	if _, err := e.GetNode(ctx, edge.ToNodeID); err != nil {
		return fmt.Errorf("target node not found: %w", err)
	}

	// Get the write lane for this edge
	lane := e.getWriteLaneForKey(edge.ID)

	// Write edge
	return lane.WriteEdge(ctx, edge)
}

// GetEdge retrieves an edge by ID.
func (e *Engine) GetEdge(ctx context.Context, edgeID string) (*graph.Edge, error) {
	if !e.running.Load() {
		return nil, fmt.Errorf("engine not running")
	}

	// Check cache first (Phase 2)
	if edge, ok := e.cache.GetEdge(edgeID); ok {
		return edge, nil
	}

	// Try the primary lane first
	lane := e.getWriteLaneForKey(edgeID)
	edge, err := lane.ReadEdge(ctx, edgeID)
	if err == nil {
		// Cache the edge (Phase 2)
		e.cache.PutEdge(edge)
		return edge, nil
	}

	// Check all lanes as fallback
	for _, l := range e.writeLanes {
		if l == lane {
			continue
		}
		edge, err := l.ReadEdge(ctx, edgeID)
		if err == nil {
			// Cache the edge (Phase 2)
			e.cache.PutEdge(edge)
			return edge, nil
		}
	}

	return nil, fmt.Errorf("edge not found: %s", edgeID)
}

// GetOutgoingEdges retrieves all outgoing edges from a node.
func (e *Engine) GetOutgoingEdges(ctx context.Context, nodeID string) ([]*graph.Edge, error) {
	if !e.running.Load() {
		return nil, fmt.Errorf("engine not running")
	}

	// Collect edge IDs from all lanes
	edgeIDs := make(map[string]bool)
	for _, lane := range e.writeLanes {
		ids := lane.GetOutgoingEdges(ctx, nodeID)
		for _, id := range ids {
			edgeIDs[id] = true
		}
	}

	// Retrieve edges
	edges := make([]*graph.Edge, 0, len(edgeIDs))
	for edgeID := range edgeIDs {
		edge, err := e.GetEdge(ctx, edgeID)
		if err == nil {
			edges = append(edges, edge)
		}
	}

	return edges, nil
}

// GetIncomingEdges retrieves all incoming edges to a node.
func (e *Engine) GetIncomingEdges(ctx context.Context, nodeID string) ([]*graph.Edge, error) {
	if !e.running.Load() {
		return nil, fmt.Errorf("engine not running")
	}

	// Collect edge IDs from all lanes
	edgeIDs := make(map[string]bool)
	for _, lane := range e.writeLanes {
		ids := lane.GetIncomingEdges(ctx, nodeID)
		for _, id := range ids {
			edgeIDs[id] = true
		}
	}

	// Retrieve edges
	edges := make([]*graph.Edge, 0, len(edgeIDs))
	for edgeID := range edgeIDs {
		edge, err := e.GetEdge(ctx, edgeID)
		if err == nil {
			edges = append(edges, edge)
		}
	}

	return edges, nil
}

// GetNeighbors retrieves neighboring nodes in the specified direction.
func (e *Engine) GetNeighbors(ctx context.Context, nodeID string, direction graph.Direction) ([]*graph.Node, error) {
	if !e.running.Load() {
		return nil, fmt.Errorf("engine not running")
	}

	nodeIDs := make(map[string]bool)

	switch direction {
	case graph.DirectionOut:
		edges, err := e.GetOutgoingEdges(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		for _, edge := range edges {
			nodeIDs[edge.ToNodeID] = true
		}

	case graph.DirectionIn:
		edges, err := e.GetIncomingEdges(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		for _, edge := range edges {
			nodeIDs[edge.FromNodeID] = true
		}

	case graph.DirectionBoth:
		outgoing, err := e.GetOutgoingEdges(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		for _, edge := range outgoing {
			nodeIDs[edge.ToNodeID] = true
		}

		incoming, err := e.GetIncomingEdges(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		for _, edge := range incoming {
			nodeIDs[edge.FromNodeID] = true
		}
	}

	// Retrieve nodes
	nodes := make([]*graph.Node, 0, len(nodeIDs))
	for id := range nodeIDs {
		node, err := e.GetNode(ctx, id)
		if err == nil {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// Stats returns engine statistics.
type EngineStats struct {
	NumLanes          int
	TotalActiveSize   int64
	TotalActiveCount  int64
	TotalOldMemtables int
	TotalSize         int64
	LaneStats         []WriteLaneStats
	CacheStats        cache.MultiLevelCacheStats     // Phase 2
	FlusherStats      persistence.FlusherStats       // Phase 2
	CompactionStats   compaction.CompactionStats     // Phase 3
	ManifestStats     manifest.ManifestStats         // Phase 3
}

// Stats returns current statistics.
func (e *Engine) Stats() EngineStats {
	stats := EngineStats{
		NumLanes:        e.numLanes,
		LaneStats:       make([]WriteLaneStats, e.numLanes),
		CacheStats:      e.cache.Stats(),      // Phase 2
		FlusherStats:    e.flusher.Stats(),    // Phase 2
		CompactionStats: e.compactor.Stats(),  // Phase 3
		ManifestStats:   e.manifest.Stats(),   // Phase 3
	}

	for i, lane := range e.writeLanes {
		laneStats := lane.Stats()
		stats.LaneStats[i] = laneStats
		stats.TotalActiveSize += laneStats.ActiveSize
		stats.TotalActiveCount += laneStats.ActiveCount
		stats.TotalOldMemtables += laneStats.OldMemtableCount
		stats.TotalSize += laneStats.TotalSize
	}

	return stats
}

// getWriteLaneForKey returns the write lane for a given key using consistent hashing.
func (e *Engine) getWriteLaneForKey(key string) *WriteLane {
	hash := fnv.New32a()
	hash.Write([]byte(key))
	idx := hash.Sum32() % uint32(e.numLanes)
	return e.writeLanes[idx]
}

// runLaneFlusher runs the background flusher for a write lane.
func (e *Engine) runLaneFlusher(lane *WriteLane) {
	defer e.wg.Done()

	for {
		select {
		case <-lane.WaitForFlush():
			// Get old memtables to flush
			oldMemtables := lane.GetOldMemtables()
			
			for _, mt := range oldMemtables {
				if mt.IsFrozen() {
					// Flush to SSTable
					if err := e.flusher.FlushNodeMemtable(mt, lane.id); err != nil {
						// Log error (in production, use proper logging)
						fmt.Printf("Error flushing memtable for lane %d: %v\n", lane.id, err)
						continue
					}
					
					// Remove flushed memtable
					lane.RemoveOldMemtable(mt)
				}
			}

		case <-lane.WaitForStop():
			// Flush any remaining memtables before stopping
			oldMemtables := lane.GetOldMemtables()
			for _, mt := range oldMemtables {
				e.flusher.FlushNodeMemtable(mt, lane.id)
			}
			return

		case <-e.ctx.Done():
			return
		}
	}
}

// recover replays WAL entries for all lanes (Phase 3).
func (e *Engine) recover() error {
	for _, lane := range e.writeLanes {
		if err := lane.Recover(); err != nil {
			return fmt.Errorf("failed to recover lane %d: %w", lane.id, err)
		}
	}
	return nil
}
