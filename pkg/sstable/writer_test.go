
package sstable

import (
	"os"
	"testing"

	"github.com/storage-engine-graph-db/pkg/graph"
)

func TestNewWriter(t *testing.T) {
	path := "./test-sstable-writer.sst"
	defer os.Remove(path)
	
	writer, err := NewWriter(path, 4096)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	if writer == nil {
		t.Fatal("Writer is nil")
	}
}

func TestWriter_AddNode(t *testing.T) {
	path := "./test-add-node.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Create node
	node := graph.NewNode("node-1", []string{"Person"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))
	
	// Serialize and add
	data, _ := graph.SerializeNode(node)
	err := writer.Add("node:node-1", data)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}
	
	// Finalize
	err = writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
}

func TestWriter_AddEdge(t *testing.T) {
	path := "./test-add-edge.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Create edge
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	
	// Serialize and add
	data, _ := graph.SerializeEdge(edge)
	err := writer.Add("edge:edge-1", data)
	if err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}
	
	writer.Finalize()
}

func TestWriter_AddMultipleEntries(t *testing.T) {
	path := "./test-multiple.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add multiple nodes
	for i := 0; i < 100; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := graph.SerializeNode(node)
		
		key := "node:" + node.ID
		err := writer.Add(key, data)
		if err != nil {
			t.Fatalf("Failed to add node %d: %v", i, err)
		}
	}
	
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
	
	// Verify file was created
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("SSTable file not created: %v", err)
	}
	
	if info.Size() == 0 {
		t.Error("SSTable file is empty")
	}
}

func TestWriter_SortedKeys(t *testing.T) {
	path := "./test-sorted.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add entries in sorted order (required for SSTable)
	keys := []string{"key-a", "key-b", "key-c", "key-d", "key-e"}
	
	for _, key := range keys {
		node := graph.NewNode(key, []string{"Test"})
		data, _ := graph.SerializeNode(node)
		writer.Add(key, data)
	}
	
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize sorted keys: %v", err)
	}
}

func TestWriter_BlockSize(t *testing.T) {
	path := "./test-blocksize.sst"
	defer os.Remove(path)
	
	// Small block size to force multiple blocks
	writer, _ := NewWriter(path, 256)
	defer writer.Close()
	
	// Add enough data to span multiple blocks
	for i := 0; i < 50; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		// Add properties to increase size
		for j := 0; j < 10; j++ {
			node.SetProperty(string(rune('x'+j)), "some-value-here")
		}
		data, _ := graph.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}
	
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize with multiple blocks: %v", err)
	}
}

func TestWriter_EmptySSTable(t *testing.T) {
	path := "./test-empty.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Finalize without adding anything
	err := writer.Finalize()
	if err == nil {
		t.Error("Expected error when finalizing empty SSTable")
	}
}

func TestWriter_LargeEntry(t *testing.T) {
	path := "./test-large-entry.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Create node with many properties
	node := graph.NewNode("large-node", []string{"Test"})
	for i := 0; i < 1000; i++ {
		node.SetProperty(string(rune('a'+i%26))+string(rune('0'+i/26)), "value")
	}
	
	data, _ := graph.SerializeNode(node)
	
	err := writer.Add("node:large-node", data)
	if err != nil {
		t.Fatalf("Failed to add large entry: %v", err)
	}
	
	err = writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize large entry: %v", err)
	}
}

func TestWriter_BloomFilter(t *testing.T) {
	path := "./test-bloom.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add some entries
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Test"})
		data, _ := graph.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}
	
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
	
	// Bloom filter should be built (internal check would require reader)
}

func TestWriter_DoubleFinalize(t *testing.T) {
	path := "./test-double-finalize.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add entry
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	writer.Add("node:node-1", data)
	
	// First finalize
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("First finalize failed: %v", err)
	}
	
	// Second finalize should fail
	err = writer.Finalize()
	if err == nil {
		t.Error("Expected error on double finalize")
	}
}

func TestWriter_AddAfterFinalize(t *testing.T) {
	path := "./test-add-after-finalize.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add and finalize
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	writer.Add("node:node-1", data)
	writer.Finalize()
	
	// Try to add after finalize
	node2 := graph.NewNode("node-2", []string{"Test"})
	data2, _ := graph.SerializeNode(node2)
	err := writer.Add("node:node-2", data2)
	if err == nil {
		t.Error("Expected error when adding after finalize")
	}
}

func TestWriter_Close(t *testing.T) {
	path := "./test-close-writer.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	
	// Add entry and finalize
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	writer.Add("node:node-1", data)
	writer.Finalize()
	
	// Close
	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	
	// Verify file exists
	_, err = os.Stat(path)
	if err != nil {
		t.Error("SSTable file should exist after close")
	}
}

func TestWriter_EdgeKeys(t *testing.T) {
	path := "./test-edge-keys.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add edge with adjacency keys
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	data, _ := graph.SerializeEdge(edge)
	
	// Add primary edge key
	writer.Add("edge:edge-1", data)
	
	// Add adjacency keys (outgoing and incoming)
	writer.Add("out:node-1:edge-1", data)
	writer.Add("in:node-2:edge-1", data)
	
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize edge keys: %v", err)
	}
}

func TestWriter_MixedNodeAndEdge(t *testing.T) {
	path := "./test-mixed.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add nodes and edges in sorted order
	node1 := graph.NewNode("node-1", []string{"Person"})
	nodeData1, _ := graph.SerializeNode(node1)
	writer.Add("node:node-1", nodeData1)
	
	node2 := graph.NewNode("node-2", []string{"Person"})
	nodeData2, _ := graph.SerializeNode(node2)
	writer.Add("node:node-2", nodeData2)
	
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edgeData, _ := graph.SerializeEdge(edge)
	writer.Add("edge:edge-1", edgeData)
	
	err := writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize mixed entries: %v", err)
	}
}

func TestWriter_FilePermissions(t *testing.T) {
	path := "./test-permissions.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := graph.SerializeNode(node)
	writer.Add("node:node-1", data)
	writer.Finalize()
	
	// Check file permissions
	info, _ := os.Stat(path)
	mode := info.Mode()
	
	if mode&0400 == 0 {
		t.Error("File should be readable")
	}
}

func TestWriter_Stats(t *testing.T) {
	path := "./test-stats.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	// Add entries
	numEntries := 50
	for i := 0; i < numEntries; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		data, _ := graph.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}
	
	writer.Finalize()
	
	// Verify file size
	info, _ := os.Stat(path)
	if info.Size() == 0 {
		t.Error("SSTable should have non-zero size")
	}
}

func BenchmarkWriter_Add(b *testing.B) {
	path := "./bench-writer-add.sst"
	defer os.Remove(path)
	
	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	
	node := graph.NewNode("bench-node", []string{"Bench"})
	data, _ := graph.SerializeNode(node)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Add(string(rune(i)), data)
	}
}

func BenchmarkWriter_Finalize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		path := "./bench-writer-finalize.sst"
		writer, _ := NewWriter(path, 4096)
		
		// Add entries
		for j := 0; j < 100; j++ {
			node := graph.NewNode(string(rune(j)), []string{"Bench"})
			data, _ := graph.SerializeNode(node)
			writer.Add(string(rune(j)), data)
		}
		
		b.StartTimer()
		writer.Finalize()
		b.StopTimer()
		
		writer.Close()
		os.Remove(path)
	}
}
