package compaction

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/sstable"
)

func createTestSSTableForCompaction(t *testing.T, path string, keyPrefix string, count int) int64 {
	writer, err := sstable.NewWriter(path, 4096)
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		return 0
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
		if t != nil {
			t.Fatalf("Failed to finalize: %v", err)
		}
		return 0
	}

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

	config := &Config{
		DataDir:      dataDir,
		Strategy:     StrategySizeTiered,
		MaxTableSize: 64 * 1024 * 1024,
	}

	compactor, err := NewCompactor(config)
	if err != nil {
		t.Fatalf("Failed to create compactor: %v", err)
	}
	if compactor == nil {
		t.Fatal("Compactor is nil")
	}
}

func TestNewCompactor_DefaultConfig(t *testing.T) {
	compactor, err := NewCompactor(nil)
	if err != nil {
		t.Fatalf("Failed to create compactor with nil config: %v", err)
	}
	if compactor == nil {
		t.Fatal("Compactor is nil")
	}
}

func TestCompactor_StartStop(t *testing.T) {
	dataDir := "./test-compactor-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir:       dataDir,
		Strategy:      StrategySizeTiered,
		MaxTableSize:  64 * 1024 * 1024,
		CheckInterval: 100 * time.Millisecond,
	}

	compactor, err := NewCompactor(config)
	if err != nil {
		t.Fatalf("Failed to create compactor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = compactor.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start compactor: %v", err)
	}

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop compactor
	err = compactor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop compactor: %v", err)
	}
}

func TestCompactor_DoubleStart(t *testing.T) {
	dataDir := "./test-compactor-double-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)
	ctx := context.Background()

	err := compactor.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start compactor: %v", err)
	}
	defer compactor.Stop()

	// Try to start again - should fail
	err = compactor.Start(ctx, 100*time.Millisecond)
	if err == nil {
		t.Error("Double start should fail")
	}
}

func TestCompactor_RegisterTable(t *testing.T) {
	dataDir := "./test-compactor-register"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)

	// Create test SSTable
	sstPath := dataDir + "/test.sst"
	size := createTestSSTableForCompaction(t, sstPath, "key", 10)

	// Register SSTable
	compactor.RegisterTable(sstPath, size, 10)

	// Verify it was registered
	stats := compactor.Stats()
	if stats.TotalTables == 0 {
		t.Error("Table should be registered")
	}
}

func TestCompactor_RegisterMultipleTables(t *testing.T) {
	dataDir := "./test-compactor-register-multi"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)

	// Create and register multiple SSTables
	for i := 0; i < 5; i++ {
		sstPath := dataDir + "/test" + string(rune('0'+i)) + ".sst"
		size := createTestSSTableForCompaction(t, sstPath, "key"+string(rune('a'+i)), 20)
		compactor.RegisterTable(sstPath, size, 20)
	}

	// Verify all were registered
	stats := compactor.Stats()
	if stats.TotalTables != 5 {
		t.Errorf("Expected 5 tables, got %d", stats.TotalTables)
	}
}

func TestCompactor_Stats(t *testing.T) {
	dataDir := "./test-compactor-stats"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)

	// Add some SSTables
	for i := 0; i < 3; i++ {
		sstPath := dataDir + "/test" + string(rune('0'+i)) + ".sst"
		size := createTestSSTableForCompaction(t, sstPath, "key", 10)
		compactor.RegisterTable(sstPath, size, 10)
	}

	// Get stats
	stats := compactor.Stats()
	if stats.TotalTables != 3 {
		t.Errorf("Expected 3 tables, got %d", stats.TotalTables)
	}
}

func TestCompactor_ConcurrentRegister(t *testing.T) {
	dataDir := "./test-concurrent-register"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)
	ctx := context.Background()
	compactor.Start(ctx, 1*time.Second)
	defer compactor.Stop()

	// Concurrent registrations
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
	if stats.TotalTables != 10 {
		t.Errorf("Expected 10 tables, got %d", stats.TotalTables)
	}
}

func TestCompactor_Strategies(t *testing.T) {
	dataDir := "./test-compactor-strategies"
	defer os.RemoveAll(dataDir)

	strategies := []Strategy{StrategySizeTiered, StrategyLeveled}

	for _, strategy := range strategies {
		os.MkdirAll(dataDir, 0755)

		config := &Config{
			DataDir:  dataDir,
			Strategy: strategy,
		}

		compactor, err := NewCompactor(config)
		if err != nil {
			t.Fatalf("Failed to create compactor with strategy %d: %v", strategy, err)
		}

		if compactor == nil {
			t.Errorf("Compactor nil for strategy %d", strategy)
		}

		os.RemoveAll(dataDir)
	}
}

func TestCompactor_ContextCancellation(t *testing.T) {
	dataDir := "./test-compactor-ctx"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)

	ctx, cancel := context.WithCancel(context.Background())

	err := compactor.Start(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Cancel context - should stop compactor
	cancel()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Stop should still work (might return error since already stopped via context)
	compactor.Stop()
}

func BenchmarkCompactor_RegisterTable(b *testing.B) {
	dataDir := "./bench-register"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := &Config{
		DataDir: dataDir,
	}

	compactor, _ := NewCompactor(config)

	// Create a test SSTable
	sstPath := dataDir + "/test.sst"
	size := createTestSSTableForCompaction(nil, sstPath, "key", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compactor.RegisterTable(sstPath, size, 100)
	}
}
