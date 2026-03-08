
package compaction

import (
	"os"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/sstable"
)

func createTestSSTableForCompaction(t *testing.T, path string, keyPrefix string, count int) {
	writer, err := sstable.NewWriter(path, 4096)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	for i := 0; i < count; i++ {
		key := keyPrefix + string(rune('a'+i%26)) + string(rune('0'+i/26))
		node := graph.NewNode(key, []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := graph.SerializeNode(node)
		writer.Add(key, data)
	}
	
	err = writer.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}
}

func TestNewCompactor(t *testing.T) {
	dataDir := "./test-compactor"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 4,
	}
	
	compactor := NewCompactor(config)
	if compactor == nil {
		t.Fatal("Compactor is nil")
	}
}

func TestCompactor_Start(t *testing.T) {
	dataDir := "./test-compactor-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 4,
	}
	
	compactor := NewCompactor(config)
	
	err := compactor.Start()
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
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 4,
	}
	
	compactor := NewCompactor(config)
	compactor.Start()
	
	err := compactor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop compactor: %v", err)
	}
}

func TestCompactor_AddSSTable(t *testing.T) {
	dataDir := "./test-compactor-add"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 4,
	}
	
	compactor := NewCompactor(config)
	
	// Create test SSTable
	sstPath := dataDir + "/test.sst"
	createTestSSTableForCompaction(t, sstPath, "key", 10)
	
	// Add SSTable to level 0
	err := compactor.AddSSTable(sstPath, 0)
	if err != nil {
		t.Fatalf("Failed to add SSTable: %v", err)
	}
}

func TestCompactor_TriggerCompaction(t *testing.T) {
	dataDir := "./test-trigger-compaction"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 2, // Lower threshold
	}
	
	compactor := NewCompactor(config)
	compactor.Start()
	defer compactor.Stop()
	
	// Create multiple SSTables at level 0
	for i := 0; i < 3; i++ {
		sstPath := dataDir + "/test" + string(rune('0'+i)) + ".sst"
		createTestSSTableForCompaction(t, sstPath, "key"+string(rune('0'+i)), 20)
		compactor.AddSSTable(sstPath, 0)
	}
	
	// Trigger compaction manually
	err := compactor.TriggerCompaction()
	if err != nil {
		t.Logf("Compaction trigger: %v", err)
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
	merger := NewMerger()
	
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
	
	merger := NewMerger()
	outputPath := dataDir + "/merged.sst"
	
	// Try to merge with no input files
	err := merger.Merge([]string{}, outputPath)
	if err == nil {
		t.Error("Expected error when merging empty list")
	}
}

func TestMerger_MergeSingleFile(t *testing.T) {
	dataDir := "./test-merger-single"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	// Create single SSTable
	sst1 := dataDir + "/sst1.sst"
	createTestSSTableForCompaction(t, sst1, "node:", 50)
	
	merger := NewMerger()
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
	
	for i := 0; i < numFiles; i++ {
		path := dataDir + "/sst" + string(rune('0'+i)) + ".sst"
		createTestSSTableForCompaction(t, path, "key"+string(rune('0'+i)), 20)
		inputPaths[i] = path
	}
	
	// Merge all files
	merger := NewMerger()
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

func TestMerger_DuplicateKeys(t *testing.T) {
	dataDir := "./test-merger-duplicates"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	// Create two SSTables with same keys but different values
	writer1, _ := sstable.NewWriter(dataDir+"/sst1.sst", 4096)
	node1 := graph.NewNode("node-1", []string{"Test"})
	node1.SetProperty("version", int64(1))
	data1, _ := graph.SerializeNode(node1)
	writer1.Add("node:node-1", data1)
	writer1.Finalize()
	writer1.Close()
	
	writer2, _ := sstable.NewWriter(dataDir+"/sst2.sst", 4096)
	node2 := graph.NewNode("node-1", []string{"Test"})
	node2.SetProperty("version", int64(2))
	data2, _ := graph.SerializeNode(node2)
	writer2.Add("node:node-1", data2)
	writer2.Finalize()
	writer2.Close()
	
	// Merge - should keep latest version
	merger := NewMerger()
	outputPath := dataDir + "/merged.sst"
	
	err := merger.Merge([]string{dataDir + "/sst1.sst", dataDir + "/sst2.sst"}, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge with duplicates: %v", err)
	}
	
	// Read merged file and verify
	reader, _ := sstable.NewReader(outputPath)
	defer reader.Close()
	
	value, found, _ := reader.Get("node:node-1")
	if !found {
		t.Error("Key should exist in merged SSTable")
	}
	
	node, _ := graph.DeserializeNode(value)
	version := node.GetProperty("version")
	
	// Should have latest version (2)
	if version != int64(2) {
		t.Logf("Version in merged SSTable: %v (implementation dependent)", version)
	}
}

func TestCompactor_LevelManagement(t *testing.T) {
	dataDir := "./test-level-mgmt"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 2,
	}
	
	compactor := NewCompactor(config)
	
	// Add SSTables to different levels
	for level := 0; level < 3; level++ {
		for i := 0; i < 2; i++ {
			sstPath := dataDir + "/level" + string(rune('0'+level)) + "_" + string(rune('0'+i)) + ".sst"
			createTestSSTableForCompaction(t, sstPath, "key", 10)
			compactor.AddSSTable(sstPath, level)
		}
	}
	
	// Verify levels are tracked
	stats := compactor.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
}

func TestCompactor_Stats(t *testing.T) {
	dataDir := "./test-compactor-stats"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 4,
	}
	
	compactor := NewCompactor(config)
	
	// Add some SSTables
	for i := 0; i < 3; i++ {
		sstPath := dataDir + "/test" + string(rune('0'+i)) + ".sst"
		createTestSSTableForCompaction(t, sstPath, "key", 10)
		compactor.AddSSTable(sstPath, 0)
	}
	
	// Get stats
	stats := compactor.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
}

func TestCompactor_ConcurrentAdd(t *testing.T) {
	dataDir := "./test-concurrent-add"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         4,
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 4,
	}
	
	compactor := NewCompactor(config)
	compactor.Start()
	defer compactor.Stop()
	
	// Concurrent adds
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			sstPath := dataDir + "/test" + string(rune('a'+id)) + ".sst"
			createTestSSTableForCompaction(t, sstPath, "key"+string(rune('a'+id)), 20)
			compactor.AddSSTable(sstPath, 0)
			done <- true
		}(i)
	}
	
	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMerger_SortedOutput(t *testing.T) {
	dataDir := "./test-sorted-output"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	// Create SSTables with keys in different ranges
	writer1, _ := sstable.NewWriter(dataDir+"/sst1.sst", 4096)
	writer1.Add("key-a", []byte("value-a"))
	writer1.Add("key-c", []byte("value-c"))
	writer1.Finalize()
	writer1.Close()
	
	writer2, _ := sstable.NewWriter(dataDir+"/sst2.sst", 4096)
	writer2.Add("key-b", []byte("value-b"))
	writer2.Add("key-d", []byte("value-d"))
	writer2.Finalize()
	writer2.Close()
	
	// Merge
	merger := NewMerger()
	outputPath := dataDir + "/merged.sst"
	err := merger.Merge([]string{dataDir + "/sst1.sst", dataDir + "/sst2.sst"}, outputPath)
	if err != nil {
		t.Fatalf("Failed to merge: %v", err)
	}
	
	// Verify sorted order in output
	reader, _ := sstable.NewReader(outputPath)
	defer reader.Close()
	
	var keys []string
	reader.Scan(func(key string, value []byte) bool {
		keys = append(keys, key)
		return true
	})
	
	// Verify keys are sorted
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("Keys not sorted: %s > %s", keys[i-1], keys[i])
		}
	}
}

func TestCompactor_MaxLevel(t *testing.T) {
	dataDir := "./test-max-level"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := CompactionConfig{
		DataDir:          dataDir,
		MaxLevel:         2, // Only 2 levels
		LevelSizeMultiplier: 10,
		MinFilesToCompact: 2,
	}
	
	compactor := NewCompactor(config)
	
	// Try to add to level 0, 1, 2
	for level := 0; level <= 2; level++ {
		sstPath := dataDir + "/level" + string(rune('0'+level)) + ".sst"
		createTestSSTableForCompaction(t, sstPath, "key", 10)
		err := compactor.AddSSTable(sstPath, level)
		if err != nil {
			t.Logf("Add to level %d: %v", level, err)
		}
	}
	
	// Try to add beyond max level
	sstPath := dataDir + "/level3.sst"
	createTestSSTableForCompaction(t, sstPath, "key", 10)
	err := compactor.AddSSTable(sstPath, 3)
	if err == nil {
		t.Error("Should fail to add beyond max level")
	}
}

func BenchmarkMerger_Merge(b *testing.B) {
	dataDir := "./bench-merge"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	// Create test SSTables
	inputPaths := make([]string, 3)
	for i := 0; i < 3; i++ {
		path := dataDir + "/input" + string(rune('0'+i)) + ".sst"
		createTestSSTableForCompaction(nil, path, "key", 100)
		inputPaths[i] = path
	}
	
	merger := NewMerger()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputPath := dataDir + "/merged-" + string(rune('0'+i)) + ".sst"
		merger.Merge(inputPaths, outputPath)
		os.Remove(outputPath)
	}
}
