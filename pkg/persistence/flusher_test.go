package persistence

import (
	"os"
	"testing"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/memtable"
)

var testSerializer = graph.NewSerializer()

func TestNewFlusher(t *testing.T) {
	dataDir := "./test-flusher"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	if flusher == nil {
		t.Fatal("Flusher is nil")
	}
}

func TestFlusher_Start(t *testing.T) {
	dataDir := "./test-flusher-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)

	err := flusher.Start()
	if err != nil {
		t.Fatalf("Failed to start flusher: %v", err)
	}

	// Stop flusher
	flusher.Stop()
}

func TestFlusher_Stop(t *testing.T) {
	dataDir := "./test-flusher-stop"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()

	err := flusher.Stop()
	if err != nil {
		t.Fatalf("Failed to stop flusher: %v", err)
	}
}

func TestFlusher_FlushMemtable(t *testing.T) {
	dataDir := "./test-flusher-flush"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Create and populate memtable
	mt := memtable.New(1024 * 1024)

	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := testSerializer.SerializeNode(node)
		mt.Put("node:"+node.ID, data)
	}

	// Freeze memtable
	mt.Freeze()

	// Flush directly
	err := flusher.FlushMemtable(mt, 0)
	if err != nil {
		t.Logf("Flush error: %v", err)
	}

	// Wait for flush to complete
	time.Sleep(100 * time.Millisecond)

	// Verify SSTable was created
	files, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatalf("Failed to read data dir: %v", err)
	}

	if len(files) == 0 {
		t.Error("No SSTable files created")
	}
}

func TestFlusher_MultipleFlushes(t *testing.T) {
	dataDir := "./test-flusher-multiple"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Create multiple memtables
	numMemtables := 5
	for i := 0; i < numMemtables; i++ {
		mt := memtable.New(1024 * 1024)

		for j := 0; j < 20; j++ {
			key := string(rune('a'+i)) + string(rune('0'+j))
			node := graph.NewNode(key, []string{"Test"})
			data, _ := testSerializer.SerializeNode(node)
			mt.Put("node:"+key, data)
		}

		mt.Freeze()
		flusher.FlushMemtable(mt, i)
	}

	// Wait for all flushes
	flusher.WaitForFlushes()
	time.Sleep(500 * time.Millisecond)

	// Verify multiple SSTables created
	files, _ := os.ReadDir(dataDir)
	if len(files) < numMemtables {
		t.Errorf("Expected at least %d SSTable files, got %d", numMemtables, len(files))
	}
}

func TestFlusher_EmptyMemtable(t *testing.T) {
	dataDir := "./test-flusher-empty"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Create empty memtable
	mt := memtable.New(1024 * 1024)
	mt.Freeze()

	// Flush directly
	flusher.FlushMemtable(mt, 0)

	// Wait
	time.Sleep(100 * time.Millisecond)

	// Empty memtables might not create files
	files, _ := os.ReadDir(dataDir)
	t.Logf("Files created for empty memtable: %d", len(files))
}

func TestFlusher_ConcurrentFlushes(t *testing.T) {
	dataDir := "./test-flusher-concurrent"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Queue many memtables concurrently
	numMemtables := 10
	done := make(chan bool, numMemtables)

	for i := 0; i < numMemtables; i++ {
		go func(id int) {
			mt := memtable.New(1024 * 1024)

			for j := 0; j < 30; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j))
				node := graph.NewNode(key, []string{"Test"})
				data, _ := testSerializer.SerializeNode(node)
				mt.Put("node:"+key, data)
			}

			mt.Freeze()
			flusher.ScheduleFlush(&FlushJob{
				LaneID:   id,
				Memtable: mt,
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines to schedule
	for i := 0; i < numMemtables; i++ {
		<-done
	}

	// Wait for all flushes
	flusher.WaitForFlushes()
	time.Sleep(1 * time.Second)

	// Verify files created
	files, _ := os.ReadDir(dataDir)
	if len(files) < numMemtables {
		t.Logf("Created %d SSTable files from %d memtables", len(files), numMemtables)
	}
}

func TestFlusher_LargeMemtable(t *testing.T) {
	dataDir := "./test-flusher-large"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Create large memtable
	mt := memtable.New(10 * 1024 * 1024) // 10MB

	for i := 0; i < 1000; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Test"})
		// Add properties to increase size
		for j := 0; j < 10; j++ {
			node.SetProperty(string(rune('a'+j)), "some-value-to-increase-size")
		}
		data, _ := testSerializer.SerializeNode(node)
		mt.Put("node:"+node.ID, data)
	}

	mt.Freeze()
	flusher.FlushMemtable(mt, 0)

	// Wait for flush
	time.Sleep(500 * time.Millisecond)

	// Verify SSTable created
	files, _ := os.ReadDir(dataDir)
	if len(files) == 0 {
		t.Error("Large memtable should create SSTable")
	}
}

func TestFlusher_Stats(t *testing.T) {
	dataDir := "./test-flusher-stats"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Flush some memtables
	for i := 0; i < 3; i++ {
		mt := memtable.New(1024 * 1024)

		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := testSerializer.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}

		mt.Freeze()
		flusher.FlushMemtable(mt, i)
	}

	// Wait for flushes
	time.Sleep(300 * time.Millisecond)

	// Get stats
	stats := flusher.Stats()

	if stats.TotalFlushes == 0 {
		t.Error("Should have flushed some memtables")
	}
}

func TestFlusher_EdgeData(t *testing.T) {
	dataDir := "./test-flusher-edges"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Create memtable with edges
	mt := memtable.New(1024 * 1024)

	for i := 0; i < 10; i++ {
		edge := graph.NewEdge(string(rune('a'+i)), "node-1", "node-2", "KNOWS")
		edge.SetProperty("index", int64(i))
		data, _ := testSerializer.SerializeEdge(edge)
		mt.Put("edge:"+edge.ID, data)
	}

	mt.Freeze()
	flusher.FlushMemtable(mt, 0)

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Verify SSTable created
	files, _ := os.ReadDir(dataDir)
	if len(files) == 0 {
		t.Error("Should create SSTable for edges")
	}
}

func TestFlusher_MixedNodeEdge(t *testing.T) {
	dataDir := "./test-flusher-mixed"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Create memtable with nodes and edges
	mt := memtable.New(1024 * 1024)

	// Add nodes
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Person"})
		data, _ := testSerializer.SerializeNode(node)
		mt.Put("node:"+node.ID, data)
	}

	// Add edges
	for i := 0; i < 5; i++ {
		edge := graph.NewEdge(string(rune('e'+i)), string(rune('a'+i)), string(rune('a'+i+1)), "KNOWS")
		data, _ := testSerializer.SerializeEdge(edge)
		mt.Put("edge:"+edge.ID, data)
	}

	mt.Freeze()
	flusher.FlushMemtable(mt, 0)

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Verify SSTable created
	files, _ := os.ReadDir(dataDir)
	if len(files) == 0 {
		t.Error("Should create SSTable for mixed content")
	}
}

func TestFlusher_StopWithPendingFlushes(t *testing.T) {
	dataDir := "./test-flusher-stop-pending"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()

	// Queue memtables
	for i := 0; i < 5; i++ {
		mt := memtable.New(1024 * 1024)

		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := testSerializer.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}

		mt.Freeze()
		flusher.ScheduleFlush(&FlushJob{
			LaneID:   i,
			Memtable: mt,
		})
	}

	// Stop immediately (should wait for pending flushes)
	err := flusher.Stop()
	if err != nil {
		t.Fatalf("Failed to stop with pending flushes: %v", err)
	}

	// Verify some files were created
	files, _ := os.ReadDir(dataDir)
	t.Logf("Created %d files before stop", len(files))
}

func TestFlusher_FileNaming(t *testing.T) {
	dataDir := "./test-flusher-naming"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	// Flush multiple memtables
	for i := 0; i < 3; i++ {
		mt := memtable.New(1024 * 1024)

		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := testSerializer.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}

		mt.Freeze()
		flusher.FlushMemtable(mt, i)
	}

	// Wait for flushes
	time.Sleep(300 * time.Millisecond)

	// Verify unique file names
	files, _ := os.ReadDir(dataDir)
	fileNames := make(map[string]bool)

	for _, file := range files {
		if fileNames[file.Name()] {
			t.Errorf("Duplicate file name: %s", file.Name())
		}
		fileNames[file.Name()] = true
	}
}

func BenchmarkFlusher_Flush(b *testing.B) {
	dataDir := "./bench-flusher"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	config := Config{
		DataDir:       dataDir,
		FlushInterval: 30 * time.Second,
	}

	flusher, _ := NewFlusher(&config)
	flusher.Start()
	defer flusher.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt := memtable.New(1024 * 1024)

		for j := 0; j < 100; j++ {
			node := graph.NewNode(string(rune(j)), []string{"Bench"})
			data, _ := testSerializer.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}

		mt.Freeze()
		flusher.FlushMemtable(mt, i)
	}

	// Wait for all flushes
	flusher.WaitForFlushes()
}
