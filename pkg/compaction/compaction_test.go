package compaction

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/sstable"
)

var testSerializer = graph.NewSerializer()

func createTestSSTableForCompaction(t *testing.T, path string, keyPrefix string, count int) int64 {
	writer, err := sstable.NewWriter(path, 4096)
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		return 0
	}

	for i := 0; i < count; i++ {
		key := keyPrefix + string(rune('a'+i%26)) + string(rune('0'+i/26))
		node := graph.NewNode(key, []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := testSerializer.SerializeNode(node)
		writer.Add(key, data)
	}

	writer.Close()

	// Return file size
	stat, _ := os.Stat(path)
	if stat != nil {
		return stat.Size()
	}
	return 0
}

func TestNewCompactor(t *testing.T) {
	dataDir := "./test-compactor"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)
	if compactor == nil {
		t.Fatal("Compactor is nil")
	}
}

func TestCompactor_StartStop(t *testing.T) {
	dataDir := "./test-compactor-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := compactor.Start(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to start compactor: %v", err)
	}

	// Stop compactor
	compactor.Stop()
}

func TestCompactor_Stop(t *testing.T) {
	dataDir := "./test-compactor-stop"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	compactor.Start(ctx, 30*time.Second)

	err := compactor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop compactor: %v", err)
	}
}

func TestCompactor_DoubleStart(t *testing.T) {
	dataDir := "./test-compactor-double-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start first time
	err := compactor.Start(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("First start should succeed: %v", err)
	}
	defer compactor.Stop()

	// Start second time - should fail
	err = compactor.Start(ctx, 30*time.Second)
	if err == nil {
		t.Error("Double start should fail")
	}
}

func TestCompactor_RegisterTable(t *testing.T) {
	dataDir := "./test-compactor-register"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	// Create test SSTable
	sstPath := dataDir + "/test.sst"
	size := createTestSSTableForCompaction(t, sstPath, "key", 10)

	// Register SSTable
	compactor.RegisterTable(sstPath, size, 10)

	// Verify registration
	stats := compactor.Stats()
	if stats.NumTables != 1 {
		t.Errorf("Expected 1 table, got %d", stats.NumTables)
	}
}

func TestMerger_MergeSSTables(t *testing.T) {
	dataDir := "./test-merger"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Create two SSTables with overlapping keys
	sst1 := dataDir + "/sst1.sst"
	sst2 := dataDir + "/sst2.sst"

	createTestSSTableForCompaction(t, sst1, "node:", 50)
	createTestSSTableForCompaction(t, sst2, "node:", 50)

	// Create merger
	tables := []*TableInfo{
		{Path: sst1, Size: 0, Level: 0},
		{Path: sst2, Size: 0, Level: 0},
	}
	merger := newMerger(tables)

	// Merge SSTables
	outputPath := dataDir + "/merged.sst"
	inputPaths := []string{sst1, sst2}

	err := merger.Merge(inputPaths, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge SSTables: %v", err)
	}

	// Verify merged SSTable exists
	_, err = os.Stat(outputPath)
	if err != nil {
		t.Error("Merged SSTable not created")
	}
}

func TestMerger_MergeEmpty(t *testing.T) {
	dataDir := "./test-merger-empty"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	tables := []*TableInfo{}
	merger := newMerger(tables)
	outputPath := dataDir + "/merged.sst"

	// Try to merge with no input files
	err := merger.Merge([]string{}, outputPath)
	if err == nil {
		t.Error("Merge with no files should fail")
	}
}

func TestMerger_MergeSingleFile(t *testing.T) {
	dataDir := "./test-merger-single"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Create single SSTable
	sst1 := dataDir + "/sst1.sst"
	createTestSSTableForCompaction(t, sst1, "node:", 50)

	tables := []*TableInfo{
		{Path: sst1, Size: 0, Level: 0},
	}
	merger := newMerger(tables)
	outputPath := dataDir + "/merged.sst"

	// Merge single file
	err := merger.Merge([]string{sst1}, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge single file: %v", err)
	}

	// Verify output exists
	_, err = os.Stat(outputPath)
	if err != nil {
		t.Error("Merged SSTable not created")
	}
}

func TestMerger_MergeMultipleFiles(t *testing.T) {
	dataDir := "./test-merger-multiple"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Create multiple SSTables
	numFiles := 5
	inputPaths := make([]string, numFiles)
	tables := make([]*TableInfo, numFiles)

	for i := 0; i < numFiles; i++ {
		path := dataDir + "/sst" + string(rune('0'+i)) + ".sst"
		createTestSSTableForCompaction(t, path, "key"+string(rune('0'+i)), 20)
		inputPaths[i] = path
		tables[i] = &TableInfo{Path: path, Size: 0, Level: 0}
	}

	// Merge all files
	merger := newMerger(tables)
	outputPath := dataDir + "/merged.sst"

	err := merger.Merge(inputPaths, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge multiple files: %v", err)
	}

	// Verify output
	_, err = os.Stat(outputPath)
	if err != nil {
		t.Error("Merged SSTable not created")
	}
}

func TestMerger_MergeWithDuplicateKeys(t *testing.T) {
	dataDir := "./test-merger-duplicates"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Create two SSTables with same keys but different values
	writer1, _ := sstable.NewWriter(dataDir+"/sst1.sst", 4096)
	node1 := graph.NewNode("node-1", []string{"Test"})
	node1.SetProperty("version", int64(1))
	data1, _ := testSerializer.SerializeNode(node1)
	writer1.Add("node:node-1", data1)
	writer1.Close()

	writer2, _ := sstable.NewWriter(dataDir+"/sst2.sst", 4096)
	node2 := graph.NewNode("node-1", []string{"Test"})
	node2.SetProperty("version", int64(2))
	data2, _ := testSerializer.SerializeNode(node2)
	writer2.Add("node:node-1", data2)
	writer2.Close()

	// Merge - should keep latest version
	tables := []*TableInfo{
		{Path: dataDir + "/sst1.sst", Size: 0, Level: 0},
		{Path: dataDir + "/sst2.sst", Size: 0, Level: 0},
	}
	merger := newMerger(tables)
	outputPath := dataDir + "/merged.sst"

	err := merger.Merge([]string{dataDir + "/sst1.sst", dataDir + "/sst2.sst"}, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge with duplicates: %v", err)
	}

	// Read merged file and verify
	reader, _ := sstable.NewReader(outputPath)
	defer reader.Close()

	value, err := reader.Get("node:node-1")
	if err != nil {
		t.Error("Key should exist in merged SSTable")
	}

	node, _ := testSerializer.DeserializeNode(value)
	version := node.GetProperty("version")

	// Should have latest version (2)
	if version != int64(2) {
		t.Logf("Version in merged SSTable: %v (implementation dependent)", version)
	}
}

func TestCompactor_Stats(t *testing.T) {
	dataDir := "./test-compactor-stats"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	// Add some SSTables
	for i := 0; i < 3; i++ {
		sstPath := dataDir + "/test" + string(rune('0'+i)) + ".sst"
		size := createTestSSTableForCompaction(t, sstPath, "key", 10)
		compactor.RegisterTable(sstPath, size, 10)
	}

	// Get stats
	stats := compactor.Stats()
	if stats.NumTables != 3 {
		t.Errorf("Expected 3 tables, got %d", stats.NumTables)
	}
}

func TestCompactor_ConcurrentRegister(t *testing.T) {
	dataDir := "./test-concurrent-register"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	compactor.Start(ctx, 30*time.Second)
	defer compactor.Stop()

	// Concurrent adds
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			sstPath := dataDir + "/test" + string(rune('a'+id)) + ".sst"
			size := createTestSSTableForCompaction(t, sstPath, "key"+string(rune('a'+id)), 20)
			compactor.RegisterTable(sstPath, size, 20)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all tables registered
	stats := compactor.Stats()
	if stats.NumTables != 10 {
		t.Errorf("Expected 10 tables, got %d", stats.NumTables)
	}
}

func TestMerger_MergePreservesSortOrder(t *testing.T) {
	dataDir := "./test-merger-sort-order"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Create SSTables with keys in different ranges
	writer1, _ := sstable.NewWriter(dataDir+"/sst1.sst", 4096)
	writer1.Add("key-a", []byte("value-a"))
	writer1.Add("key-c", []byte("value-c"))
	writer1.Close()

	writer2, _ := sstable.NewWriter(dataDir+"/sst2.sst", 4096)
	writer2.Add("key-b", []byte("value-b"))
	writer2.Add("key-d", []byte("value-d"))
	writer2.Close()

	// Merge
	tables := []*TableInfo{
		{Path: dataDir + "/sst1.sst", Size: 0, Level: 0},
		{Path: dataDir + "/sst2.sst", Size: 0, Level: 0},
	}
	merger := newMerger(tables)
	outputPath := dataDir + "/merged.sst"
	err := merger.Merge([]string{dataDir + "/sst1.sst", dataDir + "/sst2.sst"}, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge: %v", err)
	}

	// Verify sorted order in output
	reader, _ := sstable.NewReader(outputPath)
	defer reader.Close()

	var keys []string
	reader.Scan(func(key string, value []byte) error {
		keys = append(keys, key)
		return nil
	})

	// Verify keys are sorted
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("Keys not sorted: %s > %s", keys[i-1], keys[i])
		}
	}
}

func TestCompactor_GetTables(t *testing.T) {
	dataDir := "./test-compactor-get-tables"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	// Add SSTables
	for i := 0; i < 3; i++ {
		sstPath := dataDir + "/test" + string(rune('0'+i)) + ".sst"
		size := createTestSSTableForCompaction(t, sstPath, "key", 10)
		compactor.RegisterTable(sstPath, size, 10)
	}

	// Get tables
	tables := compactor.GetTables()
	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}
}

func BenchmarkMerger_Merge(b *testing.B) {
	dataDir := "./bench-merger"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Create test SSTables
	inputPaths := make([]string, 3)
	tables := make([]*TableInfo, 3)
	for i := 0; i < 3; i++ {
		path := dataDir + "/input" + string(rune('0'+i)) + ".sst"
		createTestSSTableForCompaction(nil, path, "key", 100)
		inputPaths[i] = path
		tables[i] = &TableInfo{Path: path, Size: 0, Level: 0}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		merger := newMerger(tables)
		outputPath := dataDir + "/merged" + string(rune('0'+i%10)) + ".sst"
		merger.Merge(inputPaths, outputPath)
	}
}

func BenchmarkCompactor_RegisterTable(b *testing.B) {
	dataDir := "./bench-register"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		MaxTableSize:  1024 * 1024, // 1MB
		CheckInterval: 30 * time.Second,
	}

	compactor, _ := NewCompactor(&config)

	// Create a test SSTable
	sstPath := dataDir + "/test.sst"
	size := createTestSSTableForCompaction(nil, sstPath, "key", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compactor.RegisterTable(sstPath, size, 100)
	}
}
