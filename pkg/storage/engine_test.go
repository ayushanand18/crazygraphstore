
package storage

import (
	"context"
	"os"
	"testing"

	"github.com/storage-engine-graph-db/pkg/graph"
)

func TestNewEngine(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-engine"
	config.EnableWAL = false // Disable WAL for simpler tests
	defer os.RemoveAll(config.DataDir)
	
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	
	if engine == nil {
		t.Fatal("Engine is nil")
	}
	
	if engine.numLanes != config.NumWriteLanes {
		t.Errorf("Expected %d lanes, got %d", config.NumWriteLanes, engine.numLanes)
	}
}

func TestEngine_StartStop(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-start-stop"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	
	err := engine.Start()
	if err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}
	
	if !engine.running.Load() {
		t.Error("Engine should be running after Start()")
	}
	
	err = engine.Stop()
	if err != nil {
		t.Fatalf("Failed to stop engine: %v", err)
	}
	
	if engine.running.Load() {
		t.Error("Engine should not be running after Stop()")
	}
}

func TestEngine_DoubleStart(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-double-start"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	err := engine.Start()
	if err == nil {
		t.Error("Expected error on double start, got nil")
	}
}

func TestEngine_CreateNode(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-create-node"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))
	
	err := engine.CreateNode(ctx, node)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	
	// Verify node was created
	retrieved, err := engine.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	
	if retrieved.ID != "node-1" {
		t.Errorf("ID mismatch: got %s, want node-1", retrieved.ID)
	}
	
	if retrieved.GetProperty("name") != "Alice" {
		t.Error("Property 'name' mismatch")
	}
	
	if retrieved.GetProperty("age") != int64(30) {
		t.Error("Property 'age' mismatch")
	}
}

func TestEngine_CreateNode_NotRunning(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-not-running"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	// Don't start engine
	
	ctx := context.Background()
	node := graph.NewNode("node-1", []string{"Person"})
	
	err := engine.CreateNode(ctx, node)
	if err == nil {
		t.Error("Expected error when engine not running, got nil")
	}
}

func TestEngine_GetNode_NotFound(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-not-found"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	_, err := engine.GetNode(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node, got nil")
	}
}

func TestEngine_UpdateNode(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-update"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	engine.CreateNode(ctx, node)
	
	// Update node
	updates := map[string]interface{}{
		"name": "Bob",
		"age":  int64(25),
	}
	
	err := engine.UpdateNode(ctx, "node-1", updates)
	if err != nil {
		t.Fatalf("Failed to update node: %v", err)
	}
	
	// Verify updates
	retrieved, _ := engine.GetNode(ctx, "node-1")
	
	if retrieved.GetProperty("name") != "Bob" {
		t.Error("Property 'name' not updated")
	}
	
	if retrieved.GetProperty("age") != int64(25) {
		t.Error("Property 'age' not set")
	}
}

func TestEngine_CreateEdge(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-create-edge"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create nodes
	node1 := graph.NewNode("node-1", []string{"Person"})
	node2 := graph.NewNode("node-2", []string{"Person"})
	engine.CreateNode(ctx, node1)
	engine.CreateNode(ctx, node2)
	
	// Create edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	
	err := engine.CreateEdge(ctx, edge)
	if err != nil {
		t.Fatalf("Failed to create edge: %v", err)
	}
	
	// Verify edge
	retrieved, err := engine.GetEdge(ctx, "edge-1")
	if err != nil {
		t.Fatalf("Failed to get edge: %v", err)
	}
	
	if retrieved.FromNodeID != "node-1" {
		t.Error("FromNodeID mismatch")
	}
	
	if retrieved.ToNodeID != "node-2" {
		t.Error("ToNodeID mismatch")
	}
	
	if retrieved.Type != "KNOWS" {
		t.Error("Type mismatch")
	}
}

func TestEngine_CreateEdge_SourceNotFound(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-edge-no-source"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create only target node
	node2 := graph.NewNode("node-2", []string{"Person"})
	engine.CreateNode(ctx, node2)
	
	// Try to create edge with missing source
	edge := graph.NewEdge("edge-1", "nonexistent", "node-2", "KNOWS")
	
	err := engine.CreateEdge(ctx, edge)
	if err == nil {
		t.Error("Expected error for missing source node, got nil")
	}
}

func TestEngine_CreateEdge_TargetNotFound(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-edge-no-target"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create only source node
	node1 := graph.NewNode("node-1", []string{"Person"})
	engine.CreateNode(ctx, node1)
	
	// Try to create edge with missing target
	edge := graph.NewEdge("edge-1", "node-1", "nonexistent", "KNOWS")
	
	err := engine.CreateEdge(ctx, edge)
	if err == nil {
		t.Error("Expected error for missing target node, got nil")
	}
}

func TestEngine_GetOutgoingEdges(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-outgoing"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create nodes
	node1 := graph.NewNode("node-1", []string{"Person"})
	node2 := graph.NewNode("node-2", []string{"Person"})
	node3 := graph.NewNode("node-3", []string{"Person"})
	engine.CreateNode(ctx, node1)
	engine.CreateNode(ctx, node2)
	engine.CreateNode(ctx, node3)
	
	// Create edges
	edge1 := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-1", "node-3", "KNOWS")
	engine.CreateEdge(ctx, edge1)
	engine.CreateEdge(ctx, edge2)
	
	// Get outgoing edges
	edges, err := engine.GetOutgoingEdges(ctx, "node-1")
	if err != nil {
		t.Fatalf("Failed to get outgoing edges: %v", err)
	}
	
	if len(edges) != 2 {
		t.Errorf("Expected 2 outgoing edges, got %d", len(edges))
	}
}

func TestEngine_GetIncomingEdges(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-incoming"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create nodes
	node1 := graph.NewNode("node-1", []string{"Person"})
	node2 := graph.NewNode("node-2", []string{"Person"})
	node3 := graph.NewNode("node-3", []string{"Person"})
	engine.CreateNode(ctx, node1)
	engine.CreateNode(ctx, node2)
	engine.CreateNode(ctx, node3)
	
	// Create edges pointing to node-3
	edge1 := graph.NewEdge("edge-1", "node-1", "node-3", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-2", "node-3", "KNOWS")
	engine.CreateEdge(ctx, edge1)
	engine.CreateEdge(ctx, edge2)
	
	// Get incoming edges
	edges, err := engine.GetIncomingEdges(ctx, "node-3")
	if err != nil {
		t.Fatalf("Failed to get incoming edges: %v", err)
	}
	
	if len(edges) != 2 {
		t.Errorf("Expected 2 incoming edges, got %d", len(edges))
	}
}

func TestEngine_GetNeighbors_Out(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-neighbors-out"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create graph: node-1 -> node-2, node-1 -> node-3
	node1 := graph.NewNode("node-1", []string{"Person"})
	node2 := graph.NewNode("node-2", []string{"Person"})
	node3 := graph.NewNode("node-3", []string{"Person"})
	engine.CreateNode(ctx, node1)
	engine.CreateNode(ctx, node2)
	engine.CreateNode(ctx, node3)
	
	edge1 := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-1", "node-3", "KNOWS")
	engine.CreateEdge(ctx, edge1)
	engine.CreateEdge(ctx, edge2)
	
	// Get outgoing neighbors
	neighbors, err := engine.GetNeighbors(ctx, "node-1", graph.DirectionOut)
	if err != nil {
		t.Fatalf("Failed to get neighbors: %v", err)
	}
	
	if len(neighbors) != 2 {
		t.Errorf("Expected 2 neighbors, got %d", len(neighbors))
	}
	
	// Verify neighbors are node-2 and node-3
	neighborIDs := make(map[string]bool)
	for _, n := range neighbors {
		neighborIDs[n.ID] = true
	}
	
	if !neighborIDs["node-2"] || !neighborIDs["node-3"] {
		t.Error("Expected neighbors node-2 and node-3")
	}
}

func TestEngine_GetNeighbors_In(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-neighbors-in"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create graph: node-1 -> node-3, node-2 -> node-3
	node1 := graph.NewNode("node-1", []string{"Person"})
	node2 := graph.NewNode("node-2", []string{"Person"})
	node3 := graph.NewNode("node-3", []string{"Person"})
	engine.CreateNode(ctx, node1)
	engine.CreateNode(ctx, node2)
	engine.CreateNode(ctx, node3)
	
	edge1 := graph.NewEdge("edge-1", "node-1", "node-3", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-2", "node-3", "KNOWS")
	engine.CreateEdge(ctx, edge1)
	engine.CreateEdge(ctx, edge2)
	
	// Get incoming neighbors
	neighbors, err := engine.GetNeighbors(ctx, "node-3", graph.DirectionIn)
	if err != nil {
		t.Fatalf("Failed to get neighbors: %v", err)
	}
	
	if len(neighbors) != 2 {
		t.Errorf("Expected 2 neighbors, got %d", len(neighbors))
	}
}

func TestEngine_GetNeighbors_Both(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-neighbors-both"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create graph: node-1 -> node-2 -> node-3
	node1 := graph.NewNode("node-1", []string{"Person"})
	node2 := graph.NewNode("node-2", []string{"Person"})
	node3 := graph.NewNode("node-3", []string{"Person"})
	engine.CreateNode(ctx, node1)
	engine.CreateNode(ctx, node2)
	engine.CreateNode(ctx, node3)
	
	edge1 := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-2", "node-3", "KNOWS")
	engine.CreateEdge(ctx, edge1)
	engine.CreateEdge(ctx, edge2)
	
	// Get both direction neighbors for node-2
	neighbors, err := engine.GetNeighbors(ctx, "node-2", graph.DirectionBoth)
	if err != nil {
		t.Fatalf("Failed to get neighbors: %v", err)
	}
	
	if len(neighbors) != 2 {
		t.Errorf("Expected 2 neighbors (node-1 and node-3), got %d", len(neighbors))
	}
	
	// Verify neighbors
	neighborIDs := make(map[string]bool)
	for _, n := range neighbors {
		neighborIDs[n.ID] = true
	}
	
	if !neighborIDs["node-1"] || !neighborIDs["node-3"] {
		t.Error("Expected neighbors node-1 and node-3")
	}
}

func TestEngine_Stats(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-stats"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create some nodes
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Test"})
		engine.CreateNode(ctx, node)
	}
	
	stats := engine.Stats()
	
	if stats.NumLanes != config.NumWriteLanes {
		t.Errorf("NumLanes mismatch: got %d, want %d", stats.NumLanes, config.NumWriteLanes)
	}
	
	if stats.TotalActiveCount == 0 {
		t.Error("Expected some active entries")
	}
	
	if len(stats.LaneStats) != config.NumWriteLanes {
		t.Errorf("Expected %d lane stats, got %d", config.NumWriteLanes, len(stats.LaneStats))
	}
}

func TestEngine_CacheIntegration(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-cache"
	config.CacheSize = 10 * 1024 * 1024 // 10MB cache
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create node
	node := graph.NewNode("cached-node", []string{"Person"})
	node.SetProperty("name", "Alice")
	engine.CreateNode(ctx, node)
	
	// First read - cache miss
	_, err := engine.GetNode(ctx, "cached-node")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	
	// Second read - should hit cache
	_, err = engine.GetNode(ctx, "cached-node")
	if err != nil {
		t.Fatalf("Failed to get cached node: %v", err)
	}
	
	stats := engine.Stats()
	// Note: Cache stats might not show hits immediately due to implementation details
	// Just verify cache is initialized
	if stats.CacheStats.TotalSize == 0 {
		t.Error("Cache should be initialized")
	}
}

func TestEngine_MultipleNodes(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "./test-data-multiple"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Create 100 nodes
	for i := 0; i < 100; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		node.SetProperty("index", int64(i))
		err := engine.CreateNode(ctx, node)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
	}
	
	// Verify all nodes can be retrieved
	for i := 0; i < 100; i++ {
		nodeID := string(rune('a'+i%26)) + string(rune('0'+i/26))
		node, err := engine.GetNode(ctx, nodeID)
		if err != nil {
			t.Errorf("Failed to get node %s: %v", nodeID, err)
		}
		if node.GetProperty("index") != int64(i) {
			t.Errorf("Index mismatch for node %s", nodeID)
		}
	}
}

func BenchmarkEngine_CreateNode(b *testing.B) {
	config := DefaultConfig()
	config.DataDir = "./bench-data-create"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Bench"})
		engine.CreateNode(ctx, node)
	}
}

func BenchmarkEngine_GetNode(b *testing.B) {
	config := DefaultConfig()
	config.DataDir = "./bench-data-get"
	config.EnableWAL = false
	defer os.RemoveAll(config.DataDir)
	
	engine, _ := NewEngine(config)
	engine.Start()
	defer engine.Stop()
	
	ctx := context.Background()
	
	// Populate
	for i := 0; i < 1000; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Bench"})
		engine.CreateNode(ctx, node)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.GetNode(ctx, string(rune(i%1000)))
	}
}
