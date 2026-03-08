
package memtable

import (
	"sync"
	"testing"

	"github.com/storage-engine-graph-db/pkg/graph"
)

func TestMemtable_New(t *testing.T) {
	maxSize := int64(1024 * 1024)
	mt := New(maxSize)
	
	if mt.maxSize != maxSize {
		t.Errorf("Expected maxSize %d, got %d", maxSize, mt.maxSize)
	}
	
	if mt.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", mt.Size())
	}
	
	if mt.Count() != 0 {
		t.Errorf("Expected initial count 0, got %d", mt.Count())
	}
}

func TestMemtable_Put_Get(t *testing.T) {
	mt := New(1024 * 1024)
	
	key := "test-key"
	value := []byte("test-value")
	
	err := mt.Put(key, value)
	if err != nil {
		t.Fatalf("Failed to put: %v", err)
	}
	
	retrieved, ok := mt.Get(key)
	if !ok {
		t.Fatal("Failed to get inserted key")
	}
	
	if string(retrieved) != string(value) {
		t.Errorf("Value mismatch: got %s, want %s", retrieved, value)
	}
}

func TestMemtable_Put_EmptyKey(t *testing.T) {
	mt := New(1024 * 1024)
	
	err := mt.Put("", []byte("value"))
	if err == nil {
		t.Error("Expected error for empty key, got nil")
	}
}

func TestMemtable_Put_SizeTracking(t *testing.T) {
	mt := New(1024 * 1024)
	
	key := "key"
	value := []byte("value")
	expectedSize := int64(len(key) + len(value))
	
	mt.Put(key, value)
	
	if mt.Size() != expectedSize {
		t.Errorf("Size mismatch: got %d, want %d", mt.Size(), expectedSize)
	}
	
	if mt.Count() != 1 {
		t.Errorf("Count mismatch: got %d, want 1", mt.Count())
	}
}

func TestMemtable_Put_Update(t *testing.T) {
	mt := New(1024 * 1024)
	
	key := "key"
	value1 := []byte("value1")
	value2 := []byte("value2222") // Longer value
	
	// First insert
	mt.Put(key, value1)
	size1 := mt.Size()
	
	// Update
	mt.Put(key, value2)
	size2 := mt.Size()
	
	// Count should remain 1
	if mt.Count() != 1 {
		t.Errorf("Count should be 1 after update, got %d", mt.Count())
	}
	
	// Size should reflect the new value
	expectedDelta := int64(len(value2) - len(value1))
	if size2-size1 != expectedDelta {
		t.Errorf("Size delta mismatch: got %d, want %d", size2-size1, expectedDelta)
	}
	
	// Verify value
	retrieved, _ := mt.Get(key)
	if string(retrieved) != string(value2) {
		t.Error("Updated value not retrieved correctly")
	}
}

func TestMemtable_Put_MaxSizeExceeded(t *testing.T) {
	maxSize := int64(100)
	mt := New(maxSize)
	
	key := "key"
	value := make([]byte, 200) // Exceeds max size
	
	err := mt.Put(key, value)
	if err == nil {
		t.Error("Expected error when exceeding max size, got nil")
	}
}

func TestMemtable_Delete(t *testing.T) {
	mt := New(1024 * 1024)
	
	key := "key"
	value := []byte("value")
	
	mt.Put(key, value)
	
	err := mt.Delete(key)
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}
	
	// Get should return tombstone (nil value)
	retrieved, ok := mt.Get(key)
	if !ok {
		t.Error("Expected key to exist (with tombstone)")
	}
	
	if retrieved != nil {
		t.Errorf("Expected nil (tombstone), got %v", retrieved)
	}
}

func TestMemtable_Freeze(t *testing.T) {
	mt := New(1024 * 1024)
	
	key := "key"
	value := []byte("value")
	
	mt.Put(key, value)
	
	if mt.IsFrozen() {
		t.Error("Memtable should not be frozen initially")
	}
	
	mt.Freeze()
	
	if !mt.IsFrozen() {
		t.Error("Memtable should be frozen after Freeze()")
	}
	
	// Should not allow writes after freeze
	err := mt.Put("new-key", []byte("new-value"))
	if err == nil {
		t.Error("Expected error when writing to frozen memtable, got nil")
	}
	
	// Should not allow deletes after freeze
	err = mt.Delete(key)
	if err == nil {
		t.Error("Expected error when deleting from frozen memtable, got nil")
	}
	
	// Should still allow reads
	_, ok := mt.Get(key)
	if !ok {
		t.Error("Should still be able to read from frozen memtable")
	}
}

func TestMemtable_Entries(t *testing.T) {
	mt := New(1024 * 1024)
	
	// Add multiple entries
	entries := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}
	
	for k, v := range entries {
		mt.Put(k, v)
	}
	
	// Get all entries
	allEntries := mt.Entries()
	
	if len(allEntries) != len(entries) {
		t.Errorf("Expected %d entries, got %d", len(entries), len(allEntries))
	}
	
	// Verify all entries are present
	found := make(map[string]bool)
	for _, entry := range allEntries {
		found[entry.Key] = true
		expectedValue := entries[entry.Key]
		if string(entry.Value) != string(expectedValue) {
			t.Errorf("Value mismatch for key %s", entry.Key)
		}
	}
	
	for key := range entries {
		if !found[key] {
			t.Errorf("Key %s not found in entries", key)
		}
	}
}

func TestMemtable_Clear(t *testing.T) {
	mt := New(1024 * 1024)
	
	mt.Put("key1", []byte("value1"))
	mt.Put("key2", []byte("value2"))
	mt.Freeze()
	
	mt.Clear()
	
	if mt.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", mt.Size())
	}
	
	if mt.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", mt.Count())
	}
	
	if mt.IsFrozen() {
		t.Error("Memtable should not be frozen after clear")
	}
	
	// Should be able to write after clear
	err := mt.Put("new-key", []byte("new-value"))
	if err != nil {
		t.Errorf("Should be able to write after clear: %v", err)
	}
}

func TestMemtable_ConcurrentReads(t *testing.T) {
	mt := New(1024 * 1024)
	
	// Populate memtable
	for i := 0; i < 100; i++ {
		key := string(rune('a' + i))
		mt.Put(key, []byte(key))
	}
	
	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + j))
				_, ok := mt.Get(key)
				if !ok {
					t.Errorf("Failed to get key %s", key)
				}
			}
		}()
	}
	
	wg.Wait()
}

func TestMemtable_ConcurrentWrites(t *testing.T) {
	mt := New(10 * 1024 * 1024) // Large enough for concurrent writes
	
	// Concurrent writes
	var wg sync.WaitGroup
	numWriters := 10
	writesPerWriter := 100
	
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j))
				mt.Put(key, []byte(key))
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all writes
	if mt.Count() != int64(numWriters*writesPerWriter) {
		t.Errorf("Expected %d entries, got %d", numWriters*writesPerWriter, mt.Count())
	}
}

func TestNodeMemtable_PutNode_GetNode(t *testing.T) {
	nm := NewNodeMemtable(1024 * 1024)
	
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	
	err := nm.PutNode(node)
	if err != nil {
		t.Fatalf("Failed to put node: %v", err)
	}
	
	retrieved, err := nm.GetNode("node-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	
	if retrieved.ID != node.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, node.ID)
	}
	
	if retrieved.GetProperty("name") != "Alice" {
		t.Error("Property mismatch")
	}
}

func TestNodeMemtable_GetNode_NotFound(t *testing.T) {
	nm := NewNodeMemtable(1024 * 1024)
	
	_, err := nm.GetNode("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node, got nil")
	}
}

func TestEdgeMemtable_PutEdge_GetEdge(t *testing.T) {
	em := NewEdgeMemtable(1024 * 1024)
	
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	
	err := em.PutEdge(edge)
	if err != nil {
		t.Fatalf("Failed to put edge: %v", err)
	}
	
	retrieved, err := em.GetEdge("edge-1")
	if err != nil {
		t.Fatalf("Failed to get edge: %v", err)
	}
	
	if retrieved.ID != edge.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, edge.ID)
	}
	
	if retrieved.FromNodeID != edge.FromNodeID {
		t.Error("FromNodeID mismatch")
	}
	
	if retrieved.Properties["since"] != int64(2020) {
		t.Error("Property mismatch")
	}
}

func TestAdjacencyMemtable_AddOutgoingEdge(t *testing.T) {
	am := NewAdjacencyMemtable(1024 * 1024)
	
	err := am.AddOutgoingEdge("node-1", "edge-1")
	if err != nil {
		t.Fatalf("Failed to add outgoing edge: %v", err)
	}
	
	err = am.AddOutgoingEdge("node-1", "edge-2")
	if err != nil {
		t.Fatalf("Failed to add second outgoing edge: %v", err)
	}
	
	edges := am.GetOutgoingEdges("node-1")
	if len(edges) != 2 {
		t.Errorf("Expected 2 outgoing edges, got %d", len(edges))
	}
}

func TestAdjacencyMemtable_AddIncomingEdge(t *testing.T) {
	am := NewAdjacencyMemtable(1024 * 1024)
	
	am.AddIncomingEdge("node-1", "edge-1")
	am.AddIncomingEdge("node-1", "edge-2")
	
	edges := am.GetIncomingEdges("node-1")
	if len(edges) != 2 {
		t.Errorf("Expected 2 incoming edges, got %d", len(edges))
	}
}

func TestAdjacencyMemtable_GetEdges_NoEdges(t *testing.T) {
	am := NewAdjacencyMemtable(1024 * 1024)
	
	outgoing := am.GetOutgoingEdges("nonexistent")
	if outgoing != nil {
		t.Error("Expected nil for nonexistent node outgoing edges")
	}
	
	incoming := am.GetIncomingEdges("nonexistent")
	if incoming != nil {
		t.Error("Expected nil for nonexistent node incoming edges")
	}
}

func BenchmarkMemtable_Put(b *testing.B) {
	mt := New(100 * 1024 * 1024)
	value := []byte("benchmark value")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i % 256))
		mt.Put(key, value)
	}
}

func BenchmarkMemtable_Get(b *testing.B) {
	mt := New(100 * 1024 * 1024)
	
	// Populate
	for i := 0; i < 1000; i++ {
		key := string(rune(i % 256))
		mt.Put(key, []byte("value"))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i % 256))
		mt.Get(key)
	}
}

func BenchmarkNodeMemtable_PutNode(b *testing.B) {
	nm := NewNodeMemtable(100 * 1024 * 1024)
	node := graph.NewNode("bench-node", []string{"Person"})
	node.SetProperty("name", "Alice")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nm.PutNode(node)
	}
}
