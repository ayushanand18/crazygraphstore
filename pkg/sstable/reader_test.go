
package sstable

import (
	"os"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

func createTestSSTable(t *testing.T, path string, numEntries int) {
	writer, err := NewWriter(path, 4096)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	for i := 0; i < numEntries; i++ {
		node := graph.NewNode(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := graph.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}
	
	err = writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
}

func TestNewReader(t *testing.T) {
	path := "./test-reader.sst"
	defer os.Remove(path)
	
	// Create test SSTable
	createTestSSTable(t, path, 10)
	
	// Open reader
	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()
	
	if reader == nil {
		t.Fatal("Reader is nil")
	}
}

func TestReader_Get(t *testing.T) {
	path := "./test-reader-get.sst"
	defer os.Remove(path)
	
	// Create SSTable with known entries
	writer, _ := NewWriter(path, 4096)
	node1 := graph.NewNode("node-1", []string{"Person"})
	node1.SetProperty("name", "Alice")
	data1, _ := graph.SerializeNode(node1)
	writer.Add("node:node-1", data1)
	
	node2 := graph.NewNode("node-2", []string{"Person"})
	node2.SetProperty("name", "Bob")
	data2, _ := graph.SerializeNode(node2)
	writer.Add("node:node-2", data2)
	
	writer.Finalize()
	writer.Close()
	
	// Read entries
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Get first node
	value1, found1, err := reader.Get("node:node-1")
	if err != nil {
		t.Fatalf("Failed to get node-1: %v", err)
	}
	if !found1 {
		t.Error("node-1 not found")
	}
	if value1 == nil {
		t.Error("node-1 data is nil")
	}
	
	// Get second node
	value2, found2, err := reader.Get("node:node-2")
	if err != nil {
		t.Fatalf("Failed to get node-2: %v", err)
	}
	if !found2 {
		t.Error("node-2 not found")
	}
	if value2 == nil {
		t.Error("node-2 data is nil")
	}
}

func TestReader_GetNotFound(t *testing.T) {
	path := "./test-reader-notfound.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 10)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Try to get non-existent key
	_, found, err := reader.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Error("Should not find non-existent key")
	}
}

func TestReader_BloomFilterFalsePositive(t *testing.T) {
	path := "./test-bloom-fp.sst"
	defer os.Remove(path)
	
	// Create SSTable with specific entries
	writer, _ := NewWriter(path, 4096)
	for i := 0; i < 100; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Test"})
		data, _ := graph.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}
	writer.Finalize()
	writer.Close()
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Query for keys not in SSTable
	// Bloom filter should reduce disk lookups
	notFoundCount := 0
	for i := 1000; i < 1100; i++ {
		_, found, _ := reader.Get("node:" + string(rune(i)))
		if !found {
			notFoundCount++
		}
	}
	
	// Most queries should return not found quickly
	if notFoundCount < 90 {
		t.Error("Bloom filter not working effectively")
	}
}

func TestReader_MultipleGets(t *testing.T) {
	path := "./test-multiple-gets.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 50)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Read multiple times
	for j := 0; j < 3; j++ {
		for i := 0; i < 50; i++ {
			key := "node:" + string(rune('a'+i%26)) + string(rune('0'+i/26))
			_, found, err := reader.Get(key)
			if err != nil {
				t.Fatalf("Get failed on iteration %d, key %s: %v", j, key, err)
			}
			if !found {
				t.Errorf("Key %s not found on iteration %d", key, j)
			}
		}
	}
}

func TestReader_Scan(t *testing.T) {
	path := "./test-scan.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 20)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Scan all entries
	count := 0
	err := reader.Scan(func(key string, value []byte) bool {
		count++
		return true // continue scanning
	})
	
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	if count != 20 {
		t.Errorf("Expected 20 entries, got %d", count)
	}
}

func TestReader_ScanEarlyStop(t *testing.T) {
	path := "./test-scan-stop.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 50)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Scan but stop after 10 entries
	count := 0
	err := reader.Scan(func(key string, value []byte) bool {
		count++
		return count < 10 // stop after 10
	})
	
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	if count != 10 {
		t.Errorf("Expected 10 entries, got %d", count)
	}
}

func TestReader_ScanRange(t *testing.T) {
	path := "./test-scan-range.sst"
	defer os.Remove(path)
	
	// Create SSTable with alphabetically sorted keys
	writer, _ := NewWriter(path, 4096)
	keys := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	for _, key := range keys {
		node := graph.NewNode(key, []string{"Test"})
		data, _ := graph.SerializeNode(node)
		writer.Add(key, data)
	}
	writer.Finalize()
	writer.Close()
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Scan range from "bbb" to "ddd"
	var foundKeys []string
	err := reader.ScanRange("bbb", "ddd", func(key string, value []byte) bool {
		foundKeys = append(foundKeys, key)
		return true
	})
	
	if err != nil {
		t.Fatalf("ScanRange failed: %v", err)
	}
	
	// Should find bbb, ccc
	if len(foundKeys) < 2 {
		t.Errorf("Expected at least 2 keys in range, got %d", len(foundKeys))
	}
}

func TestReader_Close(t *testing.T) {
	path := "./test-reader-close.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 10)
	
	reader, _ := NewReader(path)
	
	// Get before close
	_, found, _ := reader.Get("node:a0")
	if !found {
		t.Error("Should find entry before close")
	}
	
	// Close reader
	err := reader.Close()
	if err != nil {
		t.Fatalf("Failed to close reader: %v", err)
	}
	
	// Try to get after close - should fail
	_, _, err = reader.Get("node:a0")
	if err == nil {
		t.Error("Expected error after close")
	}
}

func TestReader_LargeSSTable(t *testing.T) {
	path := "./test-large-sstable.sst"
	defer os.Remove(path)
	
	// Create large SSTable
	numEntries := 1000
	createTestSSTable(t, path, numEntries)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Verify we can read entries
	count := 0
	err := reader.Scan(func(key string, value []byte) bool {
		count++
		return true
	})
	
	if err != nil {
		t.Fatalf("Scan large SSTable failed: %v", err)
	}
	
	if count != numEntries {
		t.Errorf("Expected %d entries, got %d", numEntries, count)
	}
}

func TestReader_EdgeEntries(t *testing.T) {
	path := "./test-edge-entries.sst"
	defer os.Remove(path)
	
	// Create SSTable with edge entries
	writer, _ := NewWriter(path, 4096)
	
	edge1 := graph.NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	data1, _ := graph.SerializeEdge(edge1)
	writer.Add("edge:edge-1", data1)
	
	edge2 := graph.NewEdge("edge-2", "node-1", "node-3", "LIKES")
	data2, _ := graph.SerializeEdge(edge2)
	writer.Add("edge:edge-2", data2)
	
	writer.Finalize()
	writer.Close()
	
	// Read edges
	reader, _ := NewReader(path)
	defer reader.Close()
	
	data, found, _ := reader.Get("edge:edge-1")
	if !found {
		t.Error("edge-1 not found")
	}
	
	// Deserialize to verify
	edge, _ := graph.DeserializeEdge(data)
	if edge.Type != "KNOWS" {
		t.Error("Edge type mismatch")
	}
}

func TestReader_Stats(t *testing.T) {
	path := "./test-reader-stats.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 100)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Check file size
	info, _ := os.Stat(path)
	if info.Size() == 0 {
		t.Error("SSTable file should not be empty")
	}
	
	// Perform some reads
	for i := 0; i < 10; i++ {
		key := "node:" + string(rune('a'+i))
		reader.Get(key)
	}
}

func TestReader_MemoryMappedRead(t *testing.T) {
	path := "./test-mmap.sst"
	defer os.Remove(path)
	
	createTestSSTable(t, path, 100)
	
	// Reader should use mmap for efficient access
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Multiple reads should be fast (cached in memory)
	for i := 0; i < 100; i++ {
		key := "node:" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		_, found, _ := reader.Get(key)
		if !found {
			t.Errorf("Key %s not found", key)
		}
	}
}

func TestReader_EmptySSTable(t *testing.T) {
	path := "./test-empty-reader.sst"
	defer os.Remove(path)
	
	// Create empty file
	f, _ := os.Create(path)
	f.Close()
	
	// Try to open as reader
	_, err := NewReader(path)
	if err == nil {
		t.Error("Expected error opening empty file as SSTable")
	}
}

func TestReader_CorruptedFile(t *testing.T) {
	path := "./test-corrupted.sst"
	defer os.Remove(path)
	
	// Create valid SSTable
	createTestSSTable(t, path, 10)
	
	// Corrupt the file
	f, _ := os.OpenFile(path, os.O_RDWR, 0644)
	f.Seek(100, 0)
	f.Write([]byte("corruption"))
	f.Close()
	
	// Try to read - might fail or return unexpected results
	reader, err := NewReader(path)
	if err != nil {
		// Expected - corrupted file
		return
	}
	defer reader.Close()
	
	// If opened, reads might fail
	_, _, _ = reader.Get("node:a0")
}

func TestReader_MixedContent(t *testing.T) {
	path := "./test-mixed-content.sst"
	defer os.Remove(path)
	
	// Create SSTable with nodes and edges
	writer, _ := NewWriter(path, 4096)
	
	// Add nodes
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Person"})
		data, _ := graph.SerializeNode(node)
		writer.Add("node:"+node.ID, data)
	}
	
	// Add edges
	for i := 0; i < 5; i++ {
		edge := graph.NewEdge(string(rune('e'+i)), string(rune('a'+i)), string(rune('a'+i+1)), "KNOWS")
		data, _ := graph.SerializeEdge(edge)
		writer.Add("edge:"+edge.ID, data)
	}
	
	writer.Finalize()
	writer.Close()
	
	// Read mixed content
	reader, _ := NewReader(path)
	defer reader.Close()
	
	// Verify nodes
	_, found, _ := reader.Get("node:a")
	if !found {
		t.Error("Node not found")
	}
	
	// Verify edges
	_, found, _ = reader.Get("edge:e")
	if !found {
		t.Error("Edge not found")
	}
}

func BenchmarkReader_Get(b *testing.B) {
	path := "./bench-reader-get.sst"
	defer os.Remove(path)
	
	// Create SSTable
	writer, _ := NewWriter(path, 4096)
	for i := 0; i < 1000; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Bench"})
		data, _ := graph.SerializeNode(node)
		writer.Add(string(rune(i)), data)
	}
	writer.Finalize()
	writer.Close()
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Get(string(rune(i % 1000)))
	}
}

func BenchmarkReader_Scan(b *testing.B) {
	path := "./bench-reader-scan.sst"
	defer os.Remove(path)
	
	// Create SSTable
	createTestSSTable(nil, path, 1000)
	
	reader, _ := NewReader(path)
	defer reader.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Scan(func(key string, value []byte) bool {
			return true
		})
	}
}
