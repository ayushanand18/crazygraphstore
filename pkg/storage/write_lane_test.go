
package storage

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

func TestWriteLane_WriteNode(t *testing.T) {
	dataDir := "./test-data-write-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	config.MemtableSize = 1024 * 1024 // 1MB
	
	wl, err := newWriteLane(0, config, nil)
	if err != nil {
		t.Fatalf("Failed to create write lane: %v", err)
	}
	defer wl.close()
	
	ctx := context.Background()
	
	// Create node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	
	err = wl.WriteNode(ctx, node)
	if err != nil {
		t.Fatalf("Failed to write node: %v", err)
	}
	
	// Verify node was written
	retrieved, err := wl.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	
	if retrieved.ID != "node-1" {
		t.Errorf("ID mismatch: got %s, want node-1", retrieved.ID)
	}
	
	if retrieved.GetProperty("name") != "Alice" {
		t.Error("Property mismatch")
	}
}

func TestWriteLane_WriteEdge(t *testing.T) {
	dataDir := "./test-data-write-edge"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Create edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	
	err := wl.WriteEdge(ctx, edge)
	if err != nil {
		t.Fatalf("Failed to write edge: %v", err)
	}
	
	// Verify edge was written
	retrieved, err := wl.GetEdge(ctx, "edge-1")
	if err != nil {
		t.Fatalf("Failed to get edge: %v", err)
	}
	
	if retrieved.ID != "edge-1" {
		t.Error("Edge ID mismatch")
	}
	
	if retrieved.FromNodeID != "node-1" || retrieved.ToNodeID != "node-2" {
		t.Error("Edge endpoints mismatch")
	}
}

func TestWriteLane_UpdateNode(t *testing.T) {
	dataDir := "./test-data-update-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Write initial node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	wl.WriteNode(ctx, node)
	
	// Update node
	updates := map[string]interface{}{
		"name": "Bob",
		"age":  int64(30),
	}
	
	err := wl.UpdateNode(ctx, "node-1", updates)
	if err != nil {
		t.Fatalf("Failed to update node: %v", err)
	}
	
	// Verify updates
	retrieved, _ := wl.GetNode(ctx, "node-1")
	
	if retrieved.GetProperty("name") != "Bob" {
		t.Error("Property 'name' not updated")
	}
	
	if retrieved.GetProperty("age") != int64(30) {
		t.Error("Property 'age' not set")
	}
	
	// Verify version was incremented
	if retrieved.Version != 2 {
		t.Errorf("Expected version 2, got %d", retrieved.Version)
	}
}

func TestWriteLane_GetNode_NotFound(t *testing.T) {
	dataDir := "./test-data-get-notfound"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	_, err := wl.GetNode(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node, got nil")
	}
}

func TestWriteLane_GetOutgoingEdges(t *testing.T) {
	dataDir := "./test-data-outgoing-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Create edges
	edge1 := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-1", "node-3", "LIKES")
	wl.WriteEdge(ctx, edge1)
	wl.WriteEdge(ctx, edge2)
	
	// Get outgoing edges
	edges, err := wl.GetOutgoingEdges(ctx, "node-1")
	if err != nil {
		t.Fatalf("Failed to get outgoing edges: %v", err)
	}
	
	if len(edges) != 2 {
		t.Errorf("Expected 2 outgoing edges, got %d", len(edges))
	}
}

func TestWriteLane_GetIncomingEdges(t *testing.T) {
	dataDir := "./test-data-incoming-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Create edges pointing to node-3
	edge1 := graph.NewEdge("edge-1", "node-1", "node-3", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-2", "node-3", "KNOWS")
	wl.WriteEdge(ctx, edge1)
	wl.WriteEdge(ctx, edge2)
	
	// Get incoming edges
	edges, err := wl.GetIncomingEdges(ctx, "node-3")
	if err != nil {
		t.Fatalf("Failed to get incoming edges: %v", err)
	}
	
	if len(edges) != 2 {
		t.Errorf("Expected 2 incoming edges, got %d", len(edges))
	}
}

func TestWriteLane_ConcurrentWrites(t *testing.T) {
	dataDir := "./test-data-concurrent-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Concurrent writes
	numGoroutines := 10
	writesPerGoroutine := 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < writesPerGoroutine; j++ {
				nodeID := string(rune('a'+goroutineID)) + string(rune('0'+j))
				node := graph.NewNode(nodeID, []string{"Test"})
				node.SetProperty("goroutine", int64(goroutineID))
				node.SetProperty("index", int64(j))
				
				err := wl.WriteNode(ctx, node)
				if err != nil {
					t.Errorf("Failed to write node: %v", err)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all nodes were written
	totalExpected := numGoroutines * writesPerGoroutine
	stats := wl.Stats()
	
	if stats.ActiveCount < totalExpected {
		t.Errorf("Expected at least %d entries, got %d", totalExpected, stats.ActiveCount)
	}
}

func TestWriteLane_MemtableRotation(t *testing.T) {
	dataDir := "./test-data-rotation"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	config.MemtableSize = 1024 // Very small to force rotation
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Write enough data to trigger rotation
	for i := 0; i < 100; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Test"})
		// Add properties to increase size
		for j := 0; j < 10; j++ {
			node.SetProperty(string(rune('a'+j)), "some-value-to-increase-size")
		}
		wl.WriteNode(ctx, node)
	}
	
	// Check if frozen memtables were created
	wl.mu.RLock()
	frozenCount := len(wl.frozenMemtables)
	wl.mu.RUnlock()
	
	if frozenCount == 0 {
		t.Error("Expected frozen memtables after writing large amount of data")
	}
}

func TestWriteLane_ReadFromFrozenMemtables(t *testing.T) {
	dataDir := "./test-data-frozen-read"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	config.MemtableSize = 512 // Small to force rotation
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Write nodes
	node1 := graph.NewNode("node-1", []string{"Test"})
	for i := 0; i < 20; i++ {
		node1.SetProperty(string(rune('a'+i)), "value")
	}
	wl.WriteNode(ctx, node1)
	
	// Force rotation by writing more
	for i := 0; i < 50; i++ {
		node := graph.NewNode(string(rune(i+10)), []string{"Filler"})
		for j := 0; j < 10; j++ {
			node.SetProperty(string(rune('a'+j)), "filler-value")
		}
		wl.WriteNode(ctx, node)
	}
	
	// Try to read node-1, which should now be in frozen memtable
	retrieved, err := wl.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("Failed to get node from frozen memtable: %v", err)
	}
	
	if retrieved.ID != "node-1" {
		t.Error("Failed to retrieve correct node from frozen memtable")
	}
}

func TestWriteLane_Stats(t *testing.T) {
	dataDir := "./test-data-stats-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Write some data
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Test"})
		wl.WriteNode(ctx, node)
	}
	
	stats := wl.Stats()
	
	if stats.LaneID != 0 {
		t.Errorf("Expected lane ID 0, got %d", stats.LaneID)
	}
	
	if stats.ActiveCount == 0 {
		t.Error("Expected non-zero active count")
	}
	
	if stats.ActiveSize == 0 {
		t.Error("Expected non-zero active size")
	}
}

func TestWriteLane_Close(t *testing.T) {
	dataDir := "./test-data-close"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	
	ctx := context.Background()
	
	// Write some data
	node := graph.NewNode("node-1", []string{"Test"})
	wl.WriteNode(ctx, node)
	
	// Close lane
	err := wl.close()
	if err != nil {
		t.Fatalf("Failed to close write lane: %v", err)
	}
	
	// Verify we can't write after close
	node2 := graph.NewNode("node-2", []string{"Test"})
	err = wl.WriteNode(ctx, node2)
	if err == nil {
		t.Error("Expected error writing to closed lane, got nil")
	}
}

func TestWriteLane_MultipleEdgeTypes(t *testing.T) {
	dataDir := "./test-data-edge-types"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Create edges of different types
	edge1 := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge2 := graph.NewEdge("edge-2", "node-1", "node-3", "LIKES")
	edge3 := graph.NewEdge("edge-3", "node-1", "node-4", "WORKS_WITH")
	
	wl.WriteEdge(ctx, edge1)
	wl.WriteEdge(ctx, edge2)
	wl.WriteEdge(ctx, edge3)
	
	// Get all outgoing edges
	edges, _ := wl.GetOutgoingEdges(ctx, "node-1")
	
	if len(edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(edges))
	}
	
	// Verify edge types
	types := make(map[string]bool)
	for _, e := range edges {
		types[e.Type] = true
	}
	
	if !types["KNOWS"] || !types["LIKES"] || !types["WORKS_WITH"] {
		t.Error("Not all edge types found")
	}
}

func TestWriteLane_EdgeProperties(t *testing.T) {
	dataDir := "./test-data-edge-props"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Create edge with properties
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	edge.SetProperty("weight", 0.75)
	edge.SetProperty("verified", true)
	
	wl.WriteEdge(ctx, edge)
	
	// Retrieve and verify properties
	retrieved, _ := wl.GetEdge(ctx, "edge-1")
	
	if retrieved.GetProperty("since") != int64(2020) {
		t.Error("Property 'since' mismatch")
	}
	
	if retrieved.GetProperty("weight") != 0.75 {
		t.Error("Property 'weight' mismatch")
	}
	
	if retrieved.GetProperty("verified") != true {
		t.Error("Property 'verified' mismatch")
	}
}

func TestWriteLane_NodeLabels(t *testing.T) {
	dataDir := "./test-data-labels"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Create node with multiple labels
	node := graph.NewNode("node-1", []string{"Person", "Employee", "Manager"})
	node.SetProperty("name", "Alice")
	
	wl.WriteNode(ctx, node)
	
	// Retrieve and verify labels
	retrieved, _ := wl.GetNode(ctx, "node-1")
	
	if len(retrieved.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(retrieved.Labels))
	}
	
	labelMap := make(map[string]bool)
	for _, label := range retrieved.Labels {
		labelMap[label] = true
	}
	
	if !labelMap["Person"] || !labelMap["Employee"] || !labelMap["Manager"] {
		t.Error("Not all labels preserved")
	}
}

func BenchmarkWriteLane_WriteNode(b *testing.B) {
	dataDir := "./bench-data-write"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Bench"})
		wl.WriteNode(ctx, node)
	}
}

func BenchmarkWriteLane_GetNode(b *testing.B) {
	dataDir := "./bench-data-get-lane"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := DefaultConfig()
	config.DataDir = dataDir
	config.EnableWAL = false
	
	wl, _ := newWriteLane(0, config, nil)
	defer wl.close()
	
	ctx := context.Background()
	
	// Populate
	for i := 0; i < 1000; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Bench"})
		wl.WriteNode(ctx, node)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wl.GetNode(ctx, string(rune(i%1000)))
	}
}
