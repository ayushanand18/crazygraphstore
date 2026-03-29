// Package memtable provides an in-memory sorted table for graph data.
package memtable

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

// Entry represents a key-value pair in the memtable.
type Entry struct {
	Key   string
	Value []byte
}

// Memtable is a thread-safe in-memory sorted table.
type Memtable struct {
	data    sync.Map     // map[string][]byte
	size    atomic.Int64 // Current size in bytes
	maxSize int64        // Maximum size before flush
	frozen  atomic.Bool  // Whether the memtable is frozen
	mu      sync.RWMutex // Protects entries slice
	count   atomic.Int64 // Number of entries
}

// New creates a new memtable with the specified maximum size.
func New(maxSize int64) *Memtable {
	return &Memtable{
		maxSize: maxSize,
	}
}

// Put inserts or updates a key-value pair in the memtable.
func (m *Memtable) Put(key string, value []byte) error {
	if m.frozen.Load() {
		return fmt.Errorf("cannot write to frozen memtable")
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Calculate size
	entrySize := int64(len(key) + len(value))

	// Check if adding this entry would exceed max size
	newSize := m.size.Load() + entrySize
	if newSize > m.maxSize {
		return fmt.Errorf("memtable size limit exceeded")
	}

	// Store the value
	oldValue, existed := m.data.LoadOrStore(key, value)

	if existed {
		// Replace existing value
		m.data.Store(key, value)
		// Adjust size (subtract old, add new)
		oldSize := int64(len(key) + len(oldValue.([]byte)))
		m.size.Add(entrySize - oldSize)
	} else {
		// New entry
		m.size.Add(entrySize)
		m.count.Add(1)
	}

	return nil
}

// Get retrieves a value by key.
func (m *Memtable) Get(key string) ([]byte, bool) {
	value, ok := m.data.Load(key)
	if !ok {
		return nil, false
	}
	if value == nil {
		return nil, true
	}
	return value.([]byte), true
}

// Delete marks an entry as deleted (tombstone).
func (m *Memtable) Delete(key string) error {
	if m.frozen.Load() {
		return fmt.Errorf("cannot delete from frozen memtable")
	}

	oldValue, existed := m.data.Load(key)
	// Store a nil value to represent deletion
	m.data.Store(key, nil)
	if !existed {
		m.count.Add(1)
		m.size.Add(int64(len(key)))
		return nil
	}

	if oldValue != nil {
		m.size.Add(-int64(len(oldValue.([]byte))))
	}
	return nil
}

// Freeze marks the memtable as immutable.
func (m *Memtable) Freeze() {
	m.frozen.Store(true)
}

// IsFrozen returns whether the memtable is frozen.
func (m *Memtable) IsFrozen() bool {
	return m.frozen.Load()
}

// Size returns the current size in bytes.
func (m *Memtable) Size() int64 {
	return m.size.Load()
}

// Count returns the number of entries.
func (m *Memtable) Count() int64 {
	return m.count.Load()
}

// Entries returns all entries in the memtable.
// This is used during flushing to disk.
func (m *Memtable) Entries() []Entry {
	entries := make([]Entry, 0, m.count.Load())

	m.data.Range(func(key, value interface{}) bool {
		var bytes []byte
		if value != nil {
			bytes = value.([]byte)
		}
		entries = append(entries, Entry{
			Key:   key.(string),
			Value: bytes,
		})
		return true
	})

	return entries
}

// Clear removes all entries from the memtable.
func (m *Memtable) Clear() {
	m.data.Range(func(key, value interface{}) bool {
		m.data.Delete(key)
		return true
	})
	m.size.Store(0)
	m.count.Store(0)
	m.frozen.Store(false)
}

// NodeMemtable is a specialized memtable for graph nodes.
type NodeMemtable struct {
	*Memtable
	serializer *graph.Serializer
}

// NewNodeMemtable creates a new memtable for nodes.
func NewNodeMemtable(maxSize int64) *NodeMemtable {
	return &NodeMemtable{
		Memtable:   New(maxSize),
		serializer: graph.NewSerializer(),
	}
}

// PutNode stores a node in the memtable.
func (nm *NodeMemtable) PutNode(node *graph.Node) error {
	data, err := nm.serializer.SerializeNode(node)
	if err != nil {
		return fmt.Errorf("failed to serialize node: %w", err)
	}

	key := fmt.Sprintf("node:%s", node.ID)
	return nm.Put(key, data)
}

// GetNode retrieves a node from the memtable.
func (nm *NodeMemtable) GetNode(nodeID string) (*graph.Node, error) {
	key := fmt.Sprintf("node:%s", nodeID)
	data, ok := nm.Get(key)
	if !ok {
		return nil, fmt.Errorf("node not found")
	}

	if data == nil {
		// Tombstone
		return nil, fmt.Errorf("node deleted")
	}

	return nm.serializer.DeserializeNode(data)
}

// EdgeMemtable is a specialized memtable for graph edges.
type EdgeMemtable struct {
	*Memtable
	serializer *graph.Serializer
}

// NewEdgeMemtable creates a new memtable for edges.
func NewEdgeMemtable(maxSize int64) *EdgeMemtable {
	return &EdgeMemtable{
		Memtable:   New(maxSize),
		serializer: graph.NewSerializer(),
	}
}

// PutEdge stores an edge in the memtable.
func (em *EdgeMemtable) PutEdge(edge *graph.Edge) error {
	data, err := em.serializer.SerializeEdge(edge)
	if err != nil {
		return fmt.Errorf("failed to serialize edge: %w", err)
	}

	key := fmt.Sprintf("edge:%s", edge.ID)
	return em.Put(key, data)
}

// GetEdge retrieves an edge from the memtable.
func (em *EdgeMemtable) GetEdge(edgeID string) (*graph.Edge, error) {
	key := fmt.Sprintf("edge:%s", edgeID)
	data, ok := em.Get(key)
	if !ok {
		return nil, fmt.Errorf("edge not found")
	}

	if data == nil {
		// Tombstone
		return nil, fmt.Errorf("edge deleted")
	}

	return em.serializer.DeserializeEdge(data)
}

// AdjacencyMemtable stores adjacency lists for fast traversal.
type AdjacencyMemtable struct {
	*Memtable
}

// NewAdjacencyMemtable creates a new memtable for adjacency lists.
func NewAdjacencyMemtable(maxSize int64) *AdjacencyMemtable {
	return &AdjacencyMemtable{
		Memtable: New(maxSize),
	}
}

// AddOutgoingEdge adds an edge to a node's outgoing adjacency list.
func (am *AdjacencyMemtable) AddOutgoingEdge(nodeID, edgeID string) error {
	key := fmt.Sprintf("adj:out:%s", nodeID)

	// Get existing edges
	existing, _ := am.Get(key)

	// Append new edge ID
	newValue := append(existing, []byte(edgeID+",")...)

	return am.Put(key, newValue)
}

// AddIncomingEdge adds an edge to a node's incoming adjacency list.
func (am *AdjacencyMemtable) AddIncomingEdge(nodeID, edgeID string) error {
	key := fmt.Sprintf("adj:in:%s", nodeID)

	// Get existing edges
	existing, _ := am.Get(key)

	// Append new edge ID
	newValue := append(existing, []byte(edgeID+",")...)

	return am.Put(key, newValue)
}

// GetOutgoingEdges retrieves outgoing edge IDs for a node.
func (am *AdjacencyMemtable) GetOutgoingEdges(nodeID string) []string {
	key := fmt.Sprintf("adj:out:%s", nodeID)
	data, ok := am.Get(key)
	if !ok {
		return nil
	}

	return parseEdgeList(string(data))
}

// GetIncomingEdges retrieves incoming edge IDs for a node.
func (am *AdjacencyMemtable) GetIncomingEdges(nodeID string) []string {
	key := fmt.Sprintf("adj:in:%s", nodeID)
	data, ok := am.Get(key)
	if !ok {
		return nil
	}

	return parseEdgeList(string(data))
}

// Helper function to parse comma-separated edge IDs
func parseEdgeList(data string) []string {
	if data == "" {
		return nil
	}

	edges := make([]string, 0)
	current := ""

	for _, ch := range data {
		if ch == ',' {
			if current != "" {
				edges = append(edges, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	return edges
}
