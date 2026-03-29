package cache

import (
	"sync"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

func TestNewMultiLevelCache(t *testing.T) {
	totalSize := int64(10 * 1024 * 1024) // 10MB

	cache := NewMultiLevelCache(totalSize)
	if cache == nil {
		t.Fatal("Cache is nil")
	}
}

func TestMultiLevelCache_PutGetNode(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024) // 1MB

	// Create a node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))

	// Put node
	err := cache.PutNode(node)
	if err != nil {
		t.Fatalf("Failed to put node: %v", err)
	}

	// Get node
	retrieved, found := cache.GetNode("node-1")
	if !found {
		t.Error("Node not found")
	}

	if retrieved.ID != "node-1" {
		t.Errorf("Expected node ID 'node-1', got '%s'", retrieved.ID)
	}
}

func TestMultiLevelCache_PutGetEdge(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024) // 1MB

	// Create an edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))

	// Put edge
	err := cache.PutEdge(edge)
	if err != nil {
		t.Fatalf("Failed to put edge: %v", err)
	}

	// Get edge
	retrieved, found := cache.GetEdge("edge-1")
	if !found {
		t.Error("Edge not found")
	}

	if retrieved.ID != "edge-1" {
		t.Errorf("Expected edge ID 'edge-1', got '%s'", retrieved.ID)
	}
}

func TestMultiLevelCache_PutGetConnectionList(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024) // 1MB

	// Put connection list
	edgeIDs := []string{"edge-1", "edge-2", "edge-3"}
	err := cache.PutConnectionList("node-1", graph.DirectionOut, edgeIDs)
	if err != nil {
		t.Fatalf("Failed to put connection list: %v", err)
	}

	// Get connection list
	retrieved, found := cache.GetConnectionList("node-1", graph.DirectionOut)
	if !found {
		t.Error("Connection list not found")
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(retrieved))
	}
}

func TestMultiLevelCache_GetNotFound(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024)

	_, found := cache.GetNode("nonexistent")
	if found {
		t.Error("Should not find non-existent node")
	}

	_, found = cache.GetEdge("nonexistent")
	if found {
		t.Error("Should not find non-existent edge")
	}

	_, found = cache.GetConnectionList("nonexistent", graph.DirectionOut)
	if found {
		t.Error("Should not find non-existent connection list")
	}
}

func TestMultiLevelCache_InvalidateNode(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024)

	// Add node and connections
	node := graph.NewNode("node-1", []string{"Person"})
	cache.PutNode(node)
	cache.PutConnectionList("node-1", graph.DirectionOut, []string{"edge-1"})

	// Invalidate node
	cache.InvalidateNode("node-1")

	// Verify node is removed
	_, found := cache.GetNode("node-1")
	if found {
		t.Error("Node should be invalidated")
	}

	// Connection list should also be invalidated
	_, found = cache.GetConnectionList("node-1", graph.DirectionOut)
	if found {
		t.Error("Connection list should be invalidated")
	}
}

func TestMultiLevelCache_InvalidateEdge(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024)

	// Add edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	cache.PutEdge(edge)

	// Invalidate edge
	cache.InvalidateEdge("edge-1")

	// Verify edge is removed
	_, found := cache.GetEdge("edge-1")
	if found {
		t.Error("Edge should be invalidated")
	}
}

func TestMultiLevelCache_Clear(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024)

	// Add entries
	node := graph.NewNode("node-1", []string{"Person"})
	cache.PutNode(node)

	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	cache.PutEdge(edge)

	cache.PutConnectionList("node-1", graph.DirectionOut, []string{"edge-1"})

	// Clear cache
	cache.Clear()

	// Verify all entries are gone
	_, found1 := cache.GetNode("node-1")
	_, found2 := cache.GetEdge("edge-1")
	_, found3 := cache.GetConnectionList("node-1", graph.DirectionOut)

	if found1 || found2 || found3 {
		t.Error("Cache should be empty after clear")
	}
}

func TestMultiLevelCache_Stats(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024)

	// Add entries and access them
	node := graph.NewNode("node-1", []string{"Person"})
	cache.PutNode(node)
	cache.GetNode("node-1")
	cache.GetNode("nonexistent") // Miss

	// Get stats
	stats := cache.Stats()

	if stats.TotalSize == 0 {
		t.Error("Total size should be non-zero")
	}

	// Should have hits and misses recorded
	t.Logf("L0 Hit Rate: %.2f", stats.L0HitRate)
	t.Logf("Overall Hit Rate: %.2f", stats.OverallHitRate)
}

func TestMultiLevelCache_MultipleNodes(t *testing.T) {
	cache := NewMultiLevelCache(1024 * 1024)

	// Add multiple nodes
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Person"})
		cache.PutNode(node)
	}

	// Verify all nodes exist
	for i := 0; i < 10; i++ {
		_, found := cache.GetNode(string(rune('a' + i)))
		if !found {
			t.Errorf("Node %c not found", 'a'+i)
		}
	}
}

func TestMultiLevelCache_ConcurrentAccess(t *testing.T) {
	cache := NewMultiLevelCache(10 * 1024 * 1024)

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				node := graph.NewNode(string(rune('a'+id)), []string{"Person"})
				cache.PutNode(node)
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cache.GetNode(string(rune('a' + j%10)))
			}
		}()
	}

	wg.Wait()

	// Verify cache is still functional
	node := graph.NewNode("test", []string{"Test"})
	cache.PutNode(node)
	_, found := cache.GetNode("test")
	if !found {
		t.Error("Cache should work after concurrent access")
	}
}

func BenchmarkMultiLevelCache_PutNode(b *testing.B) {
	cache := NewMultiLevelCache(100 * 1024 * 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := graph.NewNode(string(rune(i%1000)), []string{"Person"})
		cache.PutNode(node)
	}
}

func BenchmarkMultiLevelCache_GetNode(b *testing.B) {
	cache := NewMultiLevelCache(100 * 1024 * 1024)

	// Populate
	for i := 0; i < 10000; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Person"})
		cache.PutNode(node)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetNode(string(rune(i % 10000)))
	}
}
