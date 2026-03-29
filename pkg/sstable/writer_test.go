package sstable

import (
	"os"
	"sort"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

var testSerializer = graph.NewSerializer()

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
	data, _ := testSerializer.SerializeNode(node)
	err := writer.Add("node:node-1", data)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	// Close writes the footer and finalizes the file.
	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
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
	data, _ := testSerializer.SerializeEdge(edge)
	err := writer.Add("edge:edge-1", data)
	if err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestWriter_AddMultipleEntries(t *testing.T) {
	path := "./test-multiple.sst"
	defer os.Remove(path)

	writer, _ := NewWriter(path, 4096)
	defer writer.Close()
	type testEntry struct {
		key  string
		data []byte
	}
	entries := make([]testEntry, 0, 100)

	// Add multiple nodes
	for i := 0; i < 100; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := testSerializer.SerializeNode(node)
		entries = append(entries, testEntry{key: "node:" + node.ID, data: data})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	for i, entry := range entries {
		err := writer.Add(entry.key, entry.data)
		if err != nil {
			t.Fatalf("Failed to add node %d: %v", i, err)
		}
	}

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
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
		data, _ := testSerializer.SerializeNode(node)
		writer.Add(key, data)
	}

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close sorted keys: %v", err)
	}
}

func TestWriter_BlockSize(t *testing.T) {
	path := "./test-blocksize.sst"
	defer os.Remove(path)

	// Small block size to force multiple blocks
	writer, _ := NewWriter(path, 256)
	defer writer.Close()
	type testEntry struct {
		key  string
		data []byte
	}
	entries := make([]testEntry, 0, 50)

	// Add enough data to span multiple blocks
	for i := 0; i < 50; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		// Add properties to increase size
		for j := 0; j < 10; j++ {
			node.SetProperty(string(rune('x'+j)), "some-value-here")
		}
		data, _ := testSerializer.SerializeNode(node)
		entries = append(entries, testEntry{key: "node:" + node.ID, data: data})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	for _, entry := range entries {
		if err := writer.Add(entry.key, entry.data); err != nil {
			t.Fatalf("Failed to add entry %s: %v", entry.key, err)
		}
	}

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close with multiple blocks: %v", err)
	}
}

func TestWriter_EmptySSTable(t *testing.T) {
	path := "./test-empty.sst"
	defer os.Remove(path)

	writer, _ := NewWriter(path, 4096)
	defer writer.Close()

	// Close without adding anything. Empty SSTables are valid in this implementation.
	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close empty SSTable: %v", err)
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

	data, _ := testSerializer.SerializeNode(node)

	err := writer.Add("node:large-node", data)
	if err != nil {
		t.Fatalf("Failed to add large entry: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close large entry: %v", err)
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
		data, _ := testSerializer.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Bloom filter should be built (internal check would require reader)
}

func TestWriter_AddAfterClose(t *testing.T) {
	path := "./test-add-after-close.sst"
	defer os.Remove(path)

	writer, _ := NewWriter(path, 4096)

	// Add and close
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)
	writer.Add("node:node-1", data)
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Try to add after close
	node2 := graph.NewNode("node-2", []string{"Test"})
	data2, _ := testSerializer.SerializeNode(node2)
	err := writer.Add("node:node-2", data2)
	if err == nil {
		t.Error("Expected error when adding after close")
	}
}

func TestWriter_Close(t *testing.T) {
	path := "./test-close-writer.sst"
	defer os.Remove(path)

	writer, _ := NewWriter(path, 4096)

	// Add entry and close
	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)
	writer.Add("node:node-1", data)

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
	data, _ := testSerializer.SerializeEdge(edge)

	// Add primary edge key
	writer.Add("edge:edge-1", data)

	// Add adjacency keys (outgoing and incoming)
	writer.Add("in:node-2:edge-1", data)
	writer.Add("out:node-1:edge-1", data)

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close edge keys: %v", err)
	}
}

func TestWriter_MixedNodeAndEdge(t *testing.T) {
	path := "./test-mixed.sst"
	defer os.Remove(path)

	writer, _ := NewWriter(path, 4096)
	defer writer.Close()

	// Add entries in lexicographic order.
	edge := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edgeData, _ := testSerializer.SerializeEdge(edge)
	writer.Add("edge:edge-1", edgeData)

	node1 := graph.NewNode("node-1", []string{"Person"})
	nodeData1, _ := testSerializer.SerializeNode(node1)
	writer.Add("node:node-1", nodeData1)

	node2 := graph.NewNode("node-2", []string{"Person"})
	nodeData2, _ := testSerializer.SerializeNode(node2)
	writer.Add("node:node-2", nodeData2)

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close mixed entries: %v", err)
	}
}

func TestWriter_FilePermissions(t *testing.T) {
	path := "./test-permissions.sst"
	defer os.Remove(path)

	writer, _ := NewWriter(path, 4096)
	defer writer.Close()

	node := graph.NewNode("node-1", []string{"Test"})
	data, _ := testSerializer.SerializeNode(node)
	writer.Add("node:node-1", data)
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

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
	type testEntry struct {
		key  string
		data []byte
	}
	entries := make([]testEntry, 0, numEntries)
	for i := 0; i < numEntries; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		data, _ := testSerializer.SerializeNode(node)
		entries = append(entries, testEntry{key: "node:" + node.ID, data: data})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	for _, entry := range entries {
		if err := writer.Add(entry.key, entry.data); err != nil {
			t.Fatalf("Failed to add stats entry %s: %v", entry.key, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

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
	data, _ := testSerializer.SerializeNode(node)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Add(string(rune(i)), data)
	}
}

func BenchmarkWriter_Close(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		path := "./bench-writer-finalize.sst"
		writer, _ := NewWriter(path, 4096)

		// Add entries
		for j := 0; j < 100; j++ {
			node := graph.NewNode(string(rune(j)), []string{"Bench"})
			data, _ := testSerializer.SerializeNode(node)
			writer.Add(string(rune(j)), data)
		}

		b.StartTimer()
		writer.Close()
		b.StopTimer()

		os.Remove(path)
	}
}
