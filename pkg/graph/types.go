// Package graph provides core graph data structures for the storage engine.
package graph

import (
	"fmt"
	"time"
)

func cloneProperties(properties map[string]interface{}) map[string]interface{} {
	if properties == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		switch typedValue := value.(type) {
		case []byte:
			copied := make([]byte, len(typedValue))
			copy(copied, typedValue)
			cloned[key] = copied
		default:
			cloned[key] = value
		}
	}

	return cloned
}

// Node represents a graph vertex with labels and properties.
type Node struct {
	ID         string                 // Unique identifier
	Labels     []string               // Node types/labels (e.g., ["Person", "Employee"])
	Properties map[string]interface{} // Arbitrary properties
	Version    uint64                 // For optimistic locking
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewNode creates a new node with the given ID and labels.
func NewNode(id string, labels []string) *Node {
	now := time.Now()
	return &Node{
		ID:         id,
		Labels:     labels,
		Properties: make(map[string]interface{}),
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// HasLabel checks if the node has a specific label.
func (n *Node) HasLabel(label string) bool {
	for _, l := range n.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// GetProperty retrieves a property value, returns nil if not found.
func (n *Node) GetProperty(key string) interface{} {
	return n.Properties[key]
}

// SetProperty sets a property value and updates the version.
func (n *Node) SetProperty(key string, value interface{}) {
	n.Properties[key] = value
	n.Version++
	n.UpdatedAt = time.Now()
}

// Clone returns a deep copy of the node so callers can mutate it safely.
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}

	labels := make([]string, len(n.Labels))
	copy(labels, n.Labels)

	return &Node{
		ID:         n.ID,
		Labels:     labels,
		Properties: cloneProperties(n.Properties),
		Version:    n.Version,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
	}
}

// Edge represents a directed relationship between two nodes with properties.
type Edge struct {
	ID         string                 // Unique edge identifier
	FromNodeID string                 // Source node
	ToNodeID   string                 // Target node
	Type       string                 // Edge type (e.g., "KNOWS", "WORKS_AT")
	Properties map[string]interface{} // Edge properties
	Version    uint64                 // For optimistic locking
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewEdge creates a new edge between two nodes.
func NewEdge(id, fromNodeID, toNodeID, edgeType string) *Edge {
	now := time.Now()
	return &Edge{
		ID:         id,
		FromNodeID: fromNodeID,
		ToNodeID:   toNodeID,
		Type:       edgeType,
		Properties: make(map[string]interface{}),
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// SetProperty sets a property value on the edge.
func (e *Edge) SetProperty(key string, value interface{}) {
	e.Properties[key] = value
	e.Version++
	e.UpdatedAt = time.Now()
}

// Clone returns a deep copy of the edge so callers can mutate it safely.
func (e *Edge) Clone() *Edge {
	if e == nil {
		return nil
	}

	return &Edge{
		ID:         e.ID,
		FromNodeID: e.FromNodeID,
		ToNodeID:   e.ToNodeID,
		Type:       e.Type,
		Properties: cloneProperties(e.Properties),
		Version:    e.Version,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}

// Direction represents the direction of edge traversal.
type Direction int

const (
	DirectionOut  Direction = iota // Outgoing edges
	DirectionIn                    // Incoming edges
	DirectionBoth                  // Both directions
)

// String returns a string representation of the direction.
func (d Direction) String() string {
	switch d {
	case DirectionOut:
		return "OUT"
	case DirectionIn:
		return "IN"
	case DirectionBoth:
		return "BOTH"
	default:
		return fmt.Sprintf("Unknown(%d)", d)
	}
}

// Path represents a path through the graph.
type Path struct {
	Nodes []*Node
	Edges []*Edge
	Cost  float64 // Optional path cost
}

// Length returns the number of hops in the path.
func (p *Path) Length() int {
	return len(p.Edges)
}

// AddHop adds a node and edge to the path.
func (p *Path) AddHop(node *Node, edge *Edge) {
	p.Nodes = append(p.Nodes, node)
	if edge != nil {
		p.Edges = append(p.Edges, edge)
	}
}
