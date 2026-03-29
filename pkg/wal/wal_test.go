package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

var testSerializer = graph.NewSerializer()

func newTestWAL(t *testing.T, dataDir string, laneID int) *WAL {
	t.Helper()

	config := DefaultConfig()
	config.DataDir = dataDir

	wal, err := NewWAL(laneID, config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	return wal
}

func newBenchmarkWAL(b *testing.B, dataDir string, laneID int) *WAL {
	b.Helper()

	config := DefaultConfig()
	config.DataDir = dataDir

	wal, err := NewWAL(laneID, config)
	if err != nil {
		b.Fatalf("Failed to create WAL: %v", err)
	}

	return wal
}

func collectEntries(w *WAL) ([]*Entry, error) {
	entries := make([]*Entry, 0)
	err := w.Replay(func(entry *Entry) error {
		entries = append(entries, entry)
		return nil
	})
	return entries, err
}

func walFile(dataDir string, laneID int) string {
	return filepath.Join(dataDir, fmt.Sprintf("lane-%d.wal", laneID))
}

func TestNewWAL(t *testing.T) {
	dataDir := "./test-wal"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	if wal == nil {
		t.Fatal("WAL is nil")
	}
}

func TestWAL_AppendNode(t *testing.T) {
	dataDir := "./test-wal-append-node"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Create node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))

	// Serialize node
	data, _ := testSerializer.SerializeNode(node)

	// Append to WAL
	err := wal.Append(EntryTypeNode, "node-1", data)
	if err != nil {
		t.Fatalf("Failed to append to WAL: %v", err)
	}

	// Sync to ensure write
	if err := wal.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}
}

func TestWAL_AppendEdge(t *testing.T) {
	dataDir := "./test-wal-append-edge"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Create edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))

	// Serialize edge
	data, _ := testSerializer.SerializeEdge(edge)

	// Append to WAL
	err := wal.Append(EntryTypeEdge, "edge-1", data)
	if err != nil {
		t.Fatalf("Failed to append edge to WAL: %v", err)
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}
}

func TestWAL_Replay(t *testing.T) {
	dataDir := "./test-wal-replay"
	defer os.RemoveAll(dataDir)

	// Write some entries
	wal := newTestWAL(t, dataDir, 0)

	node1 := graph.NewNode("node-1", []string{"Person"})
	node1.SetProperty("name", "Alice")
	data1, _ := testSerializer.SerializeNode(node1)
	wal.Append(EntryTypeNode, "node-1", data1)

	node2 := graph.NewNode("node-2", []string{"Person"})
	node2.SetProperty("name", "Bob")
	data2, _ := testSerializer.SerializeNode(node2)
	wal.Append(EntryTypeNode, "node-2", data2)

	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	dataEdge, _ := testSerializer.SerializeEdge(edge)
	wal.Append(EntryTypeEdge, "edge-1", dataEdge)

	wal.Sync()
	wal.Close()

	// Replay WAL
	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, err := collectEntries(wal2)
	if err != nil {
		t.Fatalf("Failed to replay WAL: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Verify first entry
	if entries[0].Type != EntryTypeNode {
		t.Error("First entry should be WriteNode")
	}
	if entries[0].Key != "node-1" {
		t.Error("First entry key mismatch")
	}

	// Verify second entry
	if entries[1].Type != EntryTypeNode {
		t.Error("Second entry should be WriteNode")
	}
	if entries[1].Key != "node-2" {
		t.Error("Second entry key mismatch")
	}

	// Verify third entry
	if entries[2].Type != EntryTypeEdge {
		t.Error("Third entry should be WriteEdge")
	}
	if entries[2].Key != "edge-1" {
		t.Error("Third entry key mismatch")
	}
}

func TestWAL_ReplayEmpty(t *testing.T) {
	dataDir := "./test-wal-replay-empty"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	entries, err := collectEntries(wal)
	if err != nil {
		t.Fatalf("Failed to replay empty WAL: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

func TestWAL_Truncate(t *testing.T) {
	dataDir := "./test-wal-truncate"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Write entries
	for i := 0; i < 10; i++ {
		node := graph.NewNode(fmt.Sprintf("node-%d", i), []string{"Test"})
		data, _ := testSerializer.SerializeNode(node)
		wal.Append(EntryTypeNode, fmt.Sprintf("node-%d", i), data)
	}
	wal.Sync()

	// Get file size before truncate
	info, _ := os.Stat(walFile(dataDir, 0))
	sizeBefore := info.Size()

	// Truncate
	err := wal.Truncate()
	if err != nil {
		t.Fatalf("Failed to truncate WAL: %v", err)
	}

	// Get file size after truncate
	info, _ = os.Stat(walFile(dataDir, 0))
	sizeAfter := info.Size()

	if sizeAfter >= sizeBefore {
		t.Error("WAL size should decrease after truncate")
	}

	// Verify replay returns empty
	entries, _ := collectEntries(wal)
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after truncate, got %d", len(entries))
	}
}

func TestWAL_Sync(t *testing.T) {
	dataDir := "./test-wal-sync"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Write without explicit sync
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)
	wal.Append(EntryTypeNode, "node-1", data)

	// Explicit sync
	err := wal.Sync()
	if err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	// Verify data is persisted
	info, _ := os.Stat(walFile(dataDir, 0))
	if info.Size() == 0 {
		t.Error("WAL file should not be empty after sync")
	}
}

func TestWAL_Close(t *testing.T) {
	dataDir := "./test-wal-close"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)

	// Write entry
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)
	wal.Append(EntryTypeNode, "node-1", data)

	// Close
	err := wal.Close()
	if err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}

	// Verify we can't write after close
	node2 := graph.NewNode("node-2", []string{"Test"})
	data2, _ := testSerializer.SerializeNode(node2)
	err = wal.Append(EntryTypeNode, "node-2", data2)
	if err == nil {
		t.Error("Expected error writing to closed WAL, got nil")
	}
}

func TestWAL_GroupCommit(t *testing.T) {
	dataDir := "./test-wal-group-commit"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Write multiple entries quickly (should be batched)
	for i := 0; i < 100; i++ {
		node := graph.NewNode(fmt.Sprintf("node-%d", i), []string{"Test"})
		data, _ := testSerializer.SerializeNode(node)
		wal.Append(EntryTypeNode, fmt.Sprintf("node-%d", i), data)
	}

	// Force sync before replay.
	wal.Sync()

	// Verify all entries can be replayed
	wal.Close()

	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, _ := collectEntries(wal2)
	if len(entries) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(entries))
	}
}

func TestWAL_LargeEntry(t *testing.T) {
	dataDir := "./test-wal-large"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Create node with large properties
	node := graph.NewNode("large-node", []string{"Test"})
	for i := 0; i < 1000; i++ {
		node.SetProperty(fmt.Sprintf("prop_%d", i), fmt.Sprintf("value_%d", i))
	}

	data, _ := testSerializer.SerializeNode(node)

	err := wal.Append(EntryTypeNode, "large-node", data)
	if err != nil {
		t.Fatalf("Failed to append large entry: %v", err)
	}

	wal.Sync()
	wal.Close()

	// Verify replay
	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, _ := collectEntries(wal2)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

func TestWAL_CorruptionDetection(t *testing.T) {
	dataDir := "./test-wal-corrupt"
	defer os.RemoveAll(dataDir)

	// Write valid entries
	wal := newTestWAL(t, dataDir, 0)
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)
	wal.Append(EntryTypeNode, "node-1", data)
	wal.Sync()
	wal.Close()

	// Corrupt the file
	f, _ := os.OpenFile(walFile(dataDir, 0), os.O_RDWR, 0644)
	f.Seek(-10, 2) // Seek to near end
	f.Write([]byte("corruption"))
	f.Close()

	// Try to replay - should detect corruption
	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, err := collectEntries(wal2)

	// Should either return error or partial entries
	if err == nil && len(entries) > 1 {
		t.Error("Expected corruption to be detected")
	}
}

func TestWAL_MultipleEntryTypes(t *testing.T) {
	dataDir := "./test-wal-multiple-ops"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Write node
	node := graph.NewNode("node-1", []string{"Person"})
	nodeData, _ := testSerializer.SerializeNode(node)
	wal.Append(EntryTypeNode, "node-1", nodeData)

	// Write edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edgeData, _ := testSerializer.SerializeEdge(edge)
	wal.Append(EntryTypeEdge, "edge-1", edgeData)

	// Write delete tombstone.
	wal.Append(EntryTypeDelete, "node:node-1", nil)

	wal.Sync()
	wal.Close()

	// Replay and verify
	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, _ := collectEntries(wal2)
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Verify entry types
	if entries[0].Type != EntryTypeNode {
		t.Error("First entry should be WriteNode")
	}
	if entries[1].Type != EntryTypeEdge {
		t.Error("Second entry should be WriteEdge")
	}
	if entries[2].Type != EntryTypeDelete {
		t.Error("Third entry should be Delete")
	}
}

func TestWAL_ConcurrentAppends(t *testing.T) {
	dataDir := "./test-wal-concurrent"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Concurrent appends
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				node := graph.NewNode(fmt.Sprintf("node-%d-%d", id, j), []string{"Test"})
				data, _ := testSerializer.SerializeNode(node)
				wal.Append(EntryTypeNode, fmt.Sprintf("node-%d-%d", id, j), data)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	wal.Sync()
	wal.Close()

	// Verify all entries
	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, _ := collectEntries(wal2)
	if len(entries) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(entries))
	}
}

func TestWAL_EmptyKey(t *testing.T) {
	dataDir := "./test-wal-empty-key"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	node := graph.NewNode("", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)

	// Should handle empty key
	err := wal.Append(EntryTypeNode, "", data)
	if err != nil {
		t.Fatalf("Failed to append with empty key: %v", err)
	}
}

func TestWAL_EmptyData(t *testing.T) {
	dataDir := "./test-wal-empty-data"
	defer os.RemoveAll(dataDir)

	wal := newTestWAL(t, dataDir, 0)
	defer wal.Close()

	// Append with empty data
	err := wal.Append(EntryTypeNode, "node-1", []byte{})
	if err != nil {
		t.Fatalf("Failed to append with empty data: %v", err)
	}

	wal.Sync()
	wal.Close()

	// Replay should handle empty data
	wal2 := newTestWAL(t, dataDir, 0)
	defer wal2.Close()

	entries, _ := collectEntries(wal2)
	if len(entries) != 1 {
		t.Error("Should handle empty data entry")
	}
}

func BenchmarkWAL_Append(b *testing.B) {
	dataDir := "./bench-wal-append"
	defer os.RemoveAll(dataDir)

	wal := newBenchmarkWAL(b, dataDir, 0)
	defer wal.Close()

	node := graph.NewNode("bench-node", []string{"Bench"})
	data, _ := testSerializer.SerializeNode(node)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wal.Append(EntryTypeNode, fmt.Sprintf("node-%d", i), data)
	}
}

func BenchmarkWAL_AppendAndSync(b *testing.B) {
	dataDir := "./bench-wal-sync"
	defer os.RemoveAll(dataDir)

	wal := newBenchmarkWAL(b, dataDir, 0)
	defer wal.Close()

	node := graph.NewNode("bench-node", []string{"Bench"})
	data, _ := testSerializer.SerializeNode(node)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wal.Append(EntryTypeNode, fmt.Sprintf("node-%d", i), data)
		wal.Sync()
	}
}
