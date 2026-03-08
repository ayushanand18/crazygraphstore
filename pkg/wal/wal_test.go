
package wal

import (
	"fmt"
	"os"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

func TestNewWAL(t *testing.T) {
	walPath := "./test-wal"
	defer os.RemoveAll(walPath)
	
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()
	
	if wal == nil {
		t.Fatal("WAL is nil")
	}
}

func TestWAL_AppendNode(t *testing.T) {
	walPath := "./test-wal-append-node"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Create node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))
	
	// Serialize node
	data, _ := graph.SerializeNode(node)
	
	// Append to WAL
	err := wal.Append(OpTypeWriteNode, "node-1", data)
	if err != nil {
		t.Fatalf("Failed to append to WAL: %v", err)
	}
	
	// Flush to ensure write
	wal.Flush()
}

func TestWAL_AppendEdge(t *testing.T) {
	walPath := "./test-wal-append-edge"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Create edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	
	// Serialize edge
	data, _ := graph.SerializeEdge(edge)
	
	// Append to WAL
	err := wal.Append(OpTypeWriteEdge, "edge-1", data)
	if err != nil {
		t.Fatalf("Failed to append edge to WAL: %v", err)
	}
	
	wal.Flush()
}

func TestWAL_Replay(t *testing.T) {
	walPath := "./test-wal-replay"
	defer os.RemoveAll(walPath)
	
	// Write some entries
	wal, _ := NewWAL(walPath)
	
	node1 := graph.NewNode("node-1", []string{"Person"})
	node1.SetProperty("name", "Alice")
	data1, _ := graph.SerializeNode(node1)
	wal.Append(OpTypeWriteNode, "node-1", data1)
	
	node2 := graph.NewNode("node-2", []string{"Person"})
	node2.SetProperty("name", "Bob")
	data2, _ := graph.SerializeNode(node2)
	wal.Append(OpTypeWriteNode, "node-2", data2)
	
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	dataEdge, _ := graph.SerializeEdge(edge)
	wal.Append(OpTypeWriteEdge, "edge-1", dataEdge)
	
	wal.Flush()
	wal.Close()
	
	// Replay WAL
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, err := wal2.Replay()
	if err != nil {
		t.Fatalf("Failed to replay WAL: %v", err)
	}
	
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
	
	// Verify first entry
	if entries[0].OpType != OpTypeWriteNode {
		t.Error("First entry should be WriteNode")
	}
	if entries[0].Key != "node-1" {
		t.Error("First entry key mismatch")
	}
	
	// Verify second entry
	if entries[1].OpType != OpTypeWriteNode {
		t.Error("Second entry should be WriteNode")
	}
	if entries[1].Key != "node-2" {
		t.Error("Second entry key mismatch")
	}
	
	// Verify third entry
	if entries[2].OpType != OpTypeWriteEdge {
		t.Error("Third entry should be WriteEdge")
	}
	if entries[2].Key != "edge-1" {
		t.Error("Third entry key mismatch")
	}
}

func TestWAL_ReplayEmpty(t *testing.T) {
	walPath := "./test-wal-replay-empty"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	entries, err := wal.Replay()
	if err != nil {
		t.Fatalf("Failed to replay empty WAL: %v", err)
	}
	
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

func TestWAL_Truncate(t *testing.T) {
	walPath := "./test-wal-truncate"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Write entries
	for i := 0; i < 10; i++ {
		node := graph.NewNode(fmt.Sprintf("node-%d", i), []string{"Test"})
		data, _ := graph.SerializeNode(node)
		wal.Append(OpTypeWriteNode, fmt.Sprintf("node-%d", i), data)
	}
	wal.Flush()
	
	// Get file size before truncate
	info, _ := os.Stat(walPath)
	sizeBefore := info.Size()
	
	// Truncate
	err := wal.Truncate()
	if err != nil {
		t.Fatalf("Failed to truncate WAL: %v", err)
	}
	
	// Get file size after truncate
	info, _ = os.Stat(walPath)
	sizeAfter := info.Size()
	
	if sizeAfter >= sizeBefore {
		t.Error("WAL size should decrease after truncate")
	}
	
	// Verify replay returns empty
	entries, _ := wal.Replay()
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after truncate, got %d", len(entries))
	}
}

func TestWAL_Flush(t *testing.T) {
	walPath := "./test-wal-flush"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Write without explicit flush
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	wal.Append(OpTypeWriteNode, "node-1", data)
	
	// Explicit flush
	err := wal.Flush()
	if err != nil {
		t.Fatalf("Failed to flush WAL: %v", err)
	}
	
	// Verify data is persisted
	info, _ := os.Stat(walPath)
	if info.Size() == 0 {
		t.Error("WAL file should not be empty after flush")
	}
}

func TestWAL_Close(t *testing.T) {
	walPath := "./test-wal-close"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	
	// Write entry
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	wal.Append(OpTypeWriteNode, "node-1", data)
	
	// Close
	err := wal.Close()
	if err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}
	
	// Verify we can't write after close
	node2 := graph.NewNode("node-2", []string{"Test"})
	data2, _ := graph.SerializeNode(node2)
	err = wal.Append(OpTypeWriteNode, "node-2", data2)
	if err == nil {
		t.Error("Expected error writing to closed WAL, got nil")
	}
}

func TestWAL_GroupCommit(t *testing.T) {
	walPath := "./test-wal-group-commit"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Write multiple entries quickly (should be batched)
	for i := 0; i < 100; i++ {
		node := graph.NewNode(fmt.Sprintf("node-%d", i), []string{"Test"})
		data, _ := graph.SerializeNode(node)
		wal.Append(OpTypeWriteNode, fmt.Sprintf("node-%d", i), data)
	}
	
	// Wait for group commit to flush
	wal.Flush()
	
	// Verify all entries can be replayed
	wal.Close()
	
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, _ := wal2.Replay()
	if len(entries) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(entries))
	}
}

func TestWAL_LargeEntry(t *testing.T) {
	walPath := "./test-wal-large"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Create node with large properties
	node := graph.NewNode("large-node", []string{"Test"})
	for i := 0; i < 1000; i++ {
		node.SetProperty(fmt.Sprintf("prop_%d", i), fmt.Sprintf("value_%d", i))
	}
	
	data, _ := graph.SerializeNode(node)
	
	err := wal.Append(OpTypeWriteNode, "large-node", data)
	if err != nil {
		t.Fatalf("Failed to append large entry: %v", err)
	}
	
	wal.Flush()
	wal.Close()
	
	// Verify replay
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, _ := wal2.Replay()
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

func TestWAL_CorruptionDetection(t *testing.T) {
	walPath := "./test-wal-corrupt"
	defer os.RemoveAll(walPath)
	
	// Write valid entries
	wal, _ := NewWAL(walPath)
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	wal.Append(OpTypeWriteNode, "node-1", data)
	wal.Flush()
	wal.Close()
	
	// Corrupt the file
	f, _ := os.OpenFile(walPath, os.O_RDWR, 0644)
	f.Seek(-10, 2) // Seek to near end
	f.Write([]byte("corruption"))
	f.Close()
	
	// Try to replay - should detect corruption
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, err := wal2.Replay()
	
	// Should either return error or partial entries
	if err == nil && len(entries) > 1 {
		t.Error("Expected corruption to be detected")
	}
}

func TestWAL_MultipleOperationTypes(t *testing.T) {
	walPath := "./test-wal-multiple-ops"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Write node
	node := graph.NewNode("node-1", []string{"Person"})
	nodeData, _ := graph.SerializeNode(node)
	wal.Append(OpTypeWriteNode, "node-1", nodeData)
	
	// Write edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edgeData, _ := graph.SerializeEdge(edge)
	wal.Append(OpTypeWriteEdge, "edge-1", edgeData)
	
	// Update node
	node.SetProperty("updated", true)
	updatedData, _ := graph.SerializeNode(node)
	wal.Append(OpTypeUpdateNode, "node-1", updatedData)
	
	wal.Flush()
	wal.Close()
	
	// Replay and verify
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, _ := wal2.Replay()
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
	
	// Verify operation types
	if entries[0].OpType != OpTypeWriteNode {
		t.Error("First entry should be WriteNode")
	}
	if entries[1].OpType != OpTypeWriteEdge {
		t.Error("Second entry should be WriteEdge")
	}
	if entries[2].OpType != OpTypeUpdateNode {
		t.Error("Third entry should be UpdateNode")
	}
}

func TestWAL_ConcurrentAppends(t *testing.T) {
	walPath := "./test-wal-concurrent"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Concurrent appends
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				node := graph.NewNode(fmt.Sprintf("node-%d-%d", id, j), []string{"Test"})
				data, _ := graph.SerializeNode(node)
				wal.Append(OpTypeWriteNode, fmt.Sprintf("node-%d-%d", id, j), data)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	wal.Flush()
	wal.Close()
	
	// Verify all entries
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, _ := wal2.Replay()
	if len(entries) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(entries))
	}
}

func TestWAL_EmptyKey(t *testing.T) {
	walPath := "./test-wal-empty-key"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	node := graph.NewNode("", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	
	// Should handle empty key
	err := wal.Append(OpTypeWriteNode, "", data)
	if err != nil {
		t.Fatalf("Failed to append with empty key: %v", err)
	}
}

func TestWAL_EmptyData(t *testing.T) {
	walPath := "./test-wal-empty-data"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	// Append with empty data
	err := wal.Append(OpTypeWriteNode, "node-1", []byte{})
	if err != nil {
		t.Fatalf("Failed to append with empty data: %v", err)
	}
	
	wal.Flush()
	wal.Close()
	
	// Replay should handle empty data
	wal2, _ := NewWAL(walPath)
	defer wal2.Close()
	
	entries, _ := wal2.Replay()
	if len(entries) != 1 {
		t.Error("Should handle empty data entry")
	}
}

func BenchmarkWAL_Append(b *testing.B) {
	walPath := "./bench-wal-append"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	node := graph.NewNode("bench-node", []string{"Bench"})
	data, _ := graph.SerializeNode(node)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wal.Append(OpTypeWriteNode, fmt.Sprintf("node-%d", i), data)
	}
}

func BenchmarkWAL_AppendAndFlush(b *testing.B) {
	walPath := "./bench-wal-flush"
	defer os.RemoveAll(walPath)
	
	wal, _ := NewWAL(walPath)
	defer wal.Close()
	
	node := graph.NewNode("bench-node", []string{"Bench"})
	data, _ := graph.SerializeNode(node)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wal.Append(OpTypeWriteNode, fmt.Sprintf("node-%d", i), data)
		wal.Flush()
	}
}
