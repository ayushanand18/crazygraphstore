// Package storage provides the core storage engine implementation.
package storage

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/memtable"
	"github.com/ayushanand18/crazygraphstore/pkg/wal"
)

// WriteLane represents a single shared-nothing write lane.
// Each lane has its own memtable and operates independently.
type WriteLane struct {
	id             int
	wal            *wal.WAL // Phase 3: Write-Ahead Log
	active         *memtable.NodeMemtable
	activeEdges    *memtable.EdgeMemtable
	activeAdj      *memtable.AdjacencyMemtable
	old            []*memtable.NodeMemtable
	oldEdges       []*memtable.EdgeMemtable
	oldAdj         []*memtable.AdjacencyMemtable
	oldMu          sync.RWMutex
	memtableSize   int64
	flushCh        chan struct{}
	stopCh         chan struct{}
	stopped        atomic.Bool
	serializer     *graph.Serializer
}

// NewWriteLane creates a new write lane.
func NewWriteLane(id int, memtableSize int64, walConfig *wal.Config) (*WriteLane, error) {
	// Create WAL for this lane (Phase 3)
	walInstance, err := wal.NewWAL(id, walConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	return &WriteLane{
		id:             id,
		wal:            walInstance,
		active:         memtable.NewNodeMemtable(memtableSize),
		activeEdges:    memtable.NewEdgeMemtable(memtableSize),
		activeAdj:      memtable.NewAdjacencyMemtable(memtableSize),
		memtableSize:   memtableSize,
		flushCh:        make(chan struct{}, 1),
		stopCh:         make(chan struct{}),
		serializer:     graph.NewSerializer(),
	}, nil
}

// WriteNode writes a node to the active memtable.
func (wl *WriteLane) WriteNode(ctx context.Context, node *graph.Node) error {
	if wl.stopped.Load() {
		return fmt.Errorf("write lane %d is stopped", wl.id)
	}

	// Phase 3: Write to WAL first for durability
	nodeData, err := wl.serializer.SerializeNode(node)
	if err != nil {
		return fmt.Errorf("failed to serialize node for WAL: %w", err)
	}

	if err := wl.wal.Append(wal.EntryTypeNode, node.ID, nodeData); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Write to active memtable (NO LOCK!)
	if err := wl.active.PutNode(node); err != nil {
		return fmt.Errorf("failed to write node: %w", err)
	}

	// Check if memtable should be rotated
	if wl.active.Size() > wl.memtableSize {
		wl.rotateMemtable()
	}

	return nil
}

// WriteEdge writes an edge to the active memtable.
func (wl *WriteLane) WriteEdge(ctx context.Context, edge *graph.Edge) error {
	if wl.stopped.Load() {
		return fmt.Errorf("write lane %d is stopped", wl.id)
	}

	// Phase 3: Write to WAL first for durability
	edgeData, err := wl.serializer.SerializeEdge(edge)
	if err != nil {
		return fmt.Errorf("failed to serialize edge for WAL: %w", err)
	}

	if err := wl.wal.Append(wal.EntryTypeEdge, edge.ID, edgeData); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Write edge
	if err := wl.activeEdges.PutEdge(edge); err != nil {
		return fmt.Errorf("failed to write edge: %w", err)
	}

	// Update adjacency lists
	if err := wl.activeAdj.AddOutgoingEdge(edge.FromNodeID, edge.ID); err != nil {
		return fmt.Errorf("failed to update outgoing adjacency: %w", err)
	}
	if err := wl.activeAdj.AddIncomingEdge(edge.ToNodeID, edge.ID); err != nil {
		return fmt.Errorf("failed to update incoming adjacency: %w", err)
	}

	// Check if memtables should be rotated
	if wl.activeEdges.Size() > wl.memtableSize || wl.activeAdj.Size() > wl.memtableSize {
		wl.rotateMemtable()
	}

	return nil
}

// ReadNode reads a node from active or old memtables.
func (wl *WriteLane) ReadNode(ctx context.Context, nodeID string) (*graph.Node, error) {
	// Check active memtable first
	node, err := wl.active.GetNode(nodeID)
	if err == nil {
		return node, nil
	}

	// Check old memtables
	wl.oldMu.RLock()
	defer wl.oldMu.RUnlock()

	for i := len(wl.old) - 1; i >= 0; i-- {
		node, err := wl.old[i].GetNode(nodeID)
		if err == nil {
			return node, nil
		}
	}

	return nil, fmt.Errorf("node not found in write lane %d", wl.id)
}

// ReadEdge reads an edge from active or old memtables.
func (wl *WriteLane) ReadEdge(ctx context.Context, edgeID string) (*graph.Edge, error) {
	// Check active memtable first
	edge, err := wl.activeEdges.GetEdge(edgeID)
	if err == nil {
		return edge, nil
	}

	// Check old memtables
	wl.oldMu.RLock()
	defer wl.oldMu.RUnlock()

	for i := len(wl.oldEdges) - 1; i >= 0; i-- {
		edge, err := wl.oldEdges[i].GetEdge(edgeID)
		if err == nil {
			return edge, nil
		}
	}

	return nil, fmt.Errorf("edge not found in write lane %d", wl.id)
}

// GetOutgoingEdges gets outgoing edge IDs for a node.
func (wl *WriteLane) GetOutgoingEdges(ctx context.Context, nodeID string) []string {
	// Check active first
	edges := wl.activeAdj.GetOutgoingEdges(nodeID)
	
	// Check old memtables
	wl.oldMu.RLock()
	defer wl.oldMu.RUnlock()
	
	for i := len(wl.oldAdj) - 1; i >= 0; i-- {
		oldEdges := wl.oldAdj[i].GetOutgoingEdges(nodeID)
		edges = append(edges, oldEdges...)
	}
	
	return edges
}

// GetIncomingEdges gets incoming edge IDs for a node.
func (wl *WriteLane) GetIncomingEdges(ctx context.Context, nodeID string) []string {
	// Check active first
	edges := wl.activeAdj.GetIncomingEdges(nodeID)
	
	// Check old memtables
	wl.oldMu.RLock()
	defer wl.oldMu.RUnlock()
	
	for i := len(wl.oldAdj) - 1; i >= 0; i-- {
		oldEdges := wl.oldAdj[i].GetIncomingEdges(nodeID)
		edges = append(edges, oldEdges...)
	}
	
	return edges
}

// rotateMemtable freezes the current active memtable and creates a new one.
func (wl *WriteLane) rotateMemtable() {
	// Only lock when moving to old list
	wl.oldMu.Lock()
	defer wl.oldMu.Unlock()

	// Freeze current active memtables
	wl.active.Freeze()
	wl.activeEdges.Freeze()
	wl.activeAdj.Freeze()

	// Add to old list
	wl.old = append(wl.old, wl.active)
	wl.oldEdges = append(wl.oldEdges, wl.activeEdges)
	wl.oldAdj = append(wl.oldAdj, wl.activeAdj)

	// Create new active memtables (no locks!)
	wl.active = memtable.NewNodeMemtable(wl.memtableSize)
	wl.activeEdges = memtable.NewEdgeMemtable(wl.memtableSize)
	wl.activeAdj = memtable.NewAdjacencyMemtable(wl.memtableSize)

	// Signal flusher (non-blocking)
	select {
	case wl.flushCh <- struct{}{}:
	default:
		// Flusher is busy, that's okay
	}
}

// GetOldMemtables returns the old memtables for flushing.
func (wl *WriteLane) GetOldMemtables() []*memtable.NodeMemtable {
	wl.oldMu.RLock()
	defer wl.oldMu.RUnlock()
	
	// Return a copy of the slice
	old := make([]*memtable.NodeMemtable, len(wl.old))
	copy(old, wl.old)
	return old
}

// RemoveOldMemtable removes a memtable from the old list after flushing.
func (wl *WriteLane) RemoveOldMemtable(mt *memtable.NodeMemtable) {
	wl.oldMu.Lock()
	defer wl.oldMu.Unlock()

	for i, old := range wl.old {
		if old == mt {
			wl.old = append(wl.old[:i], wl.old[i+1:]...)
			break
		}
	}
}

// Stop stops the write lane.
func (wl *WriteLane) Stop() {
	if wl.stopped.CompareAndSwap(false, true) {
		close(wl.stopCh)
		
		// Close WAL (Phase 3)
		if wl.wal != nil {
			wl.wal.Close()
		}
	}
}

// WaitForFlush returns a channel that signals when flush is needed.
func (wl *WriteLane) WaitForFlush() <-chan struct{} {
	return wl.flushCh
}

// WaitForStop returns a channel that signals when the lane should stop.
func (wl *WriteLane) WaitForStop() <-chan struct{} {
	return wl.stopCh
}

// Stats returns statistics about the write lane.
type WriteLaneStats struct {
	ID               int
	ActiveSize       int64
	ActiveCount      int64
	OldMemtableCount int
	TotalSize        int64
	WALStats         wal.WALStats // Phase 3
}

// Stats returns current statistics.
func (wl *WriteLane) Stats() WriteLaneStats {
	wl.oldMu.RLock()
	defer wl.oldMu.RUnlock()

	stats := WriteLaneStats{
		ID:               wl.id,
		ActiveSize:       wl.active.Size(),
		ActiveCount:      wl.active.Count(),
		OldMemtableCount: len(wl.old),
		TotalSize:        wl.active.Size(),
	}

	if wl.wal != nil {
		stats.WALStats = wl.wal.Stats()
	}

	for _, old := range wl.old {
		stats.TotalSize += old.Size()
	}

	return stats
}

// Recover replays WAL entries to rebuild the memtable (Phase 3).
func (wl *WriteLane) Recover() error {
	if wl.wal == nil {
		return nil
	}

	return wl.wal.Replay(func(entry *wal.Entry) error {
		switch entry.Type {
		case wal.EntryTypeNode:
			node, err := wl.serializer.DeserializeNode(entry.Value)
			if err != nil {
				return fmt.Errorf("failed to deserialize node: %w", err)
			}
			return wl.active.PutNode(node)

		case wal.EntryTypeEdge:
			edge, err := wl.serializer.DeserializeEdge(entry.Value)
			if err != nil {
				return fmt.Errorf("failed to deserialize edge: %w", err)
			}
			
			// Restore edge
			if err := wl.activeEdges.PutEdge(edge); err != nil {
				return err
			}
			
			// Restore adjacency lists
			wl.activeAdj.AddOutgoingEdge(edge.FromNodeID, edge.ID)
			wl.activeAdj.AddIncomingEdge(edge.ToNodeID, edge.ID)
			return nil

		case wal.EntryTypeDelete:
			// Handle deletion (tombstone)
			return wl.active.Delete(entry.Key)

		default:
			return fmt.Errorf("unknown WAL entry type: %d", entry.Type)
		}
	})
}

// TruncateWAL truncates the WAL after successful flush (Phase 3).
func (wl *WriteLane) TruncateWAL() error {
	if wl.wal == nil {
		return nil
	}
	return wl.wal.Truncate()
}
