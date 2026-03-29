package graph

import (
	"testing"
	"time"
)

func TestNewNode(t *testing.T) {
	node := NewNode("node-1", []string{"Person", "User"})

	if node.ID != "node-1" {
		t.Errorf("Expected ID 'node-1', got '%s'", node.ID)
	}

	if len(node.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(node.Labels))
	}

	if node.Version != 1 {
		t.Errorf("Expected version 1, got %d", node.Version)
	}

	if node.Properties == nil {
		t.Error("Expected properties map to be initialized")
	}
}

func TestNode_HasLabel(t *testing.T) {
	node := NewNode("node-1", []string{"Person", "User"})

	if !node.HasLabel("Person") {
		t.Error("Expected node to have label 'Person'")
	}

	if !node.HasLabel("User") {
		t.Error("Expected node to have label 'User'")
	}

	if node.HasLabel("Admin") {
		t.Error("Expected node not to have label 'Admin'")
	}
}

func TestNode_SetProperty(t *testing.T) {
	node := NewNode("node-1", []string{"Person"})
	initialVersion := node.Version
	initialTime := node.UpdatedAt

	time.Sleep(time.Millisecond) // Ensure time difference

	node.SetProperty("name", "Alice")

	if node.GetProperty("name") != "Alice" {
		t.Errorf("Expected property 'name' to be 'Alice', got %v", node.GetProperty("name"))
	}

	if node.Version != initialVersion+1 {
		t.Errorf("Expected version to increment, got %d", node.Version)
	}

	if !node.UpdatedAt.After(initialTime) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestNode_GetProperty(t *testing.T) {
	node := NewNode("node-1", []string{"Person"})

	// Non-existent property
	if val := node.GetProperty("nonexistent"); val != nil {
		t.Errorf("Expected nil for non-existent property, got %v", val)
	}

	// Set and get property
	node.SetProperty("age", int64(30))
	if val := node.GetProperty("age"); val != int64(30) {
		t.Errorf("Expected age to be 30, got %v", val)
	}
}

func TestNewEdge(t *testing.T) {
	edge := NewEdge("edge-1", "node-1", "node-2", "KNOWS")

	if edge.ID != "edge-1" {
		t.Errorf("Expected ID 'edge-1', got '%s'", edge.ID)
	}

	if edge.FromNodeID != "node-1" {
		t.Errorf("Expected FromNodeID 'node-1', got '%s'", edge.FromNodeID)
	}

	if edge.ToNodeID != "node-2" {
		t.Errorf("Expected ToNodeID 'node-2', got '%s'", edge.ToNodeID)
	}

	if edge.Type != "KNOWS" {
		t.Errorf("Expected Type 'KNOWS', got '%s'", edge.Type)
	}

	if edge.Version != 1 {
		t.Errorf("Expected version 1, got %d", edge.Version)
	}
}

func TestEdge_SetProperty(t *testing.T) {
	edge := NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	initialVersion := edge.Version

	edge.SetProperty("weight", int64(5))

	if edge.Properties["weight"] != int64(5) {
		t.Errorf("Expected weight to be 5, got %v", edge.Properties["weight"])
	}

	if edge.Version != initialVersion+1 {
		t.Errorf("Expected version to increment, got %d", edge.Version)
	}
}

func TestDirection_String(t *testing.T) {
	tests := []struct {
		direction Direction
		expected  string
	}{
		{DirectionOut, "OUT"},
		{DirectionIn, "IN"},
		{DirectionBoth, "BOTH"},
		{Direction(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		if got := tt.direction.String(); got != tt.expected {
			t.Errorf("Direction.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestPath_Length(t *testing.T) {
	path := &Path{
		Nodes: []*Node{
			NewNode("n1", []string{"Person"}),
			NewNode("n2", []string{"Person"}),
			NewNode("n3", []string{"Person"}),
		},
		Edges: []*Edge{
			NewEdge("e1", "n1", "n2", "KNOWS"),
			NewEdge("e2", "n2", "n3", "KNOWS"),
		},
	}

	if length := path.Length(); length != 2 {
		t.Errorf("Expected path length 2, got %d", length)
	}
}

func TestPath_AddHop(t *testing.T) {
	path := &Path{
		Nodes: make([]*Node, 0),
		Edges: make([]*Edge, 0),
	}

	node := NewNode("n1", []string{"Person"})
	edge := NewEdge("e1", "n1", "n2", "KNOWS")

	path.AddHop(node, edge)

	if len(path.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(path.Nodes))
	}

	if len(path.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(path.Edges))
	}

	// Add hop without edge
	path.AddHop(NewNode("n2", []string{"Person"}), nil)

	if len(path.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(path.Nodes))
	}

	if len(path.Edges) != 1 {
		t.Errorf("Expected 1 edge (nil not added), got %d", len(path.Edges))
	}
}

func TestNode_Properties_DifferentTypes(t *testing.T) {
	node := NewNode("node-1", []string{"Mixed"})

	// Test different property types
	node.SetProperty("string", "value")
	node.SetProperty("int", int64(42))
	node.SetProperty("float", float64(3.14))
	node.SetProperty("bool", true)
	node.SetProperty("bytes", []byte("data"))

	if node.GetProperty("string") != "value" {
		t.Error("String property mismatch")
	}

	if node.GetProperty("int") != int64(42) {
		t.Error("Int property mismatch")
	}

	if node.GetProperty("float") != float64(3.14) {
		t.Error("Float property mismatch")
	}

	if node.GetProperty("bool") != true {
		t.Error("Bool property mismatch")
	}
}

func TestNode_UpdateProperty(t *testing.T) {
	node := NewNode("node-1", []string{"Person"})

	node.SetProperty("name", "Alice")
	if node.Version != 2 {
		t.Errorf("Expected version 2 after first set, got %d", node.Version)
	}

	node.SetProperty("name", "Bob")
	if node.Version != 3 {
		t.Errorf("Expected version 3 after update, got %d", node.Version)
	}

	if node.GetProperty("name") != "Bob" {
		t.Errorf("Expected name 'Bob', got %v", node.GetProperty("name"))
	}
}

func TestNode_Clone(t *testing.T) {
	node := NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	node.SetProperty("blob", []byte("data"))

	clone := node.Clone()
	clone.Labels[0] = "Admin"
	clone.SetProperty("name", "Bob")
	cloneBlob := clone.GetProperty("blob").([]byte)
	cloneBlob[0] = 'D'

	if node.Labels[0] != "Person" {
		t.Fatalf("expected labels to be copied, got %q", node.Labels[0])
	}
	if node.GetProperty("name") != "Alice" {
		t.Fatalf("expected properties to be copied, got %v", node.GetProperty("name"))
	}
	if string(node.GetProperty("blob").([]byte)) != "data" {
		t.Fatalf("expected []byte properties to be deep copied, got %q", node.GetProperty("blob").([]byte))
	}
}

func TestEdge_Clone(t *testing.T) {
	edge := NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("blob", []byte("edge-data"))

	clone := edge.Clone()
	clone.Type = "LIKES"
	cloneBlob := clone.Properties["blob"].([]byte)
	cloneBlob[0] = 'E'

	if edge.Type != "KNOWS" {
		t.Fatalf("expected edge type to remain unchanged, got %q", edge.Type)
	}
	if string(edge.Properties["blob"].([]byte)) != "edge-data" {
		t.Fatalf("expected edge blob to be deep copied, got %q", edge.Properties["blob"].([]byte))
	}
}
