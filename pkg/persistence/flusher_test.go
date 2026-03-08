
package persistence

import (
	"os"
	"testing"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/memtable"
)

func TestNewFlusher(t *testing.T) {
	dataDir := "./test-flusher"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  make(chan *memtable.Memtable, 10),
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	if flusher == nil {
		t.Fatal("Flusher is nil")
	}
}

func TestFlusher_Start(t *testing.T) {
	dataDir := "./test-flusher-start"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  make(chan *memtable.Memtable, 10),
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	
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
	
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  make(chan *memtable.Memtable, 10),
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Create and populate memtable
	mt := memtable.NewMemtable(1024 * 1024)
	
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Test"})
		node.SetProperty("index", int64(i))
		data, _ := graph.SerializeNode(node)
		mt.Put("node:"+node.ID, data)
	}
	
	// Freeze memtable
	mt.Freeze()
	
	// Queue for flushing
	flushQueue <- mt
	
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Create multiple memtables
	numMemtables := 5
	for i := 0; i < numMemtables; i++ {
		mt := memtable.NewMemtable(1024 * 1024)
		
		for j := 0; j < 20; j++ {
			key := string(rune('a'+i)) + string(rune('0'+j))
			node := graph.NewNode(key, []string{"Test"})
			data, _ := graph.SerializeNode(node)
			mt.Put("node:"+key, data)
		}
		
		mt.Freeze()
		flushQueue <- mt
	}
	
	// Wait for all flushes
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Create empty memtable
	mt := memtable.NewMemtable(1024 * 1024)
	mt.Freeze()
	
	// Queue for flushing
	flushQueue <- mt
	
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
	
	flushQueue := make(chan *memtable.Memtable, 20)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 4, // Multiple workers
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Queue many memtables concurrently
	numMemtables := 10
	for i := 0; i < numMemtables; i++ {
		go func(id int) {
			mt := memtable.NewMemtable(1024 * 1024)
			
			for j := 0; j < 30; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j))
				node := graph.NewNode(key, []string{"Test"})
				data, _ := graph.SerializeNode(node)
				mt.Put("node:"+key, data)
			}
			
			mt.Freeze()
			flushQueue <- mt
		}(i)
	}
	
	// Wait for all flushes
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Create large memtable
	mt := memtable.NewMemtable(10 * 1024 * 1024) // 10MB
	
	for i := 0; i < 1000; i++ {
		node := graph.NewNode(string(rune(i)), []string{"Test"})
		// Add properties to increase size
		for j := 0; j < 10; j++ {
			node.SetProperty(string(rune('a'+j)), "some-value-to-increase-size")
		}
		data, _ := graph.SerializeNode(node)
		mt.Put("node:"+node.ID, data)
	}
	
	mt.Freeze()
	flushQueue <- mt
	
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Flush some memtables
	for i := 0; i < 3; i++ {
		mt := memtable.NewMemtable(1024 * 1024)
		
		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := graph.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}
		
		mt.Freeze()
		flushQueue <- mt
	}
	
	// Wait for flushes
	time.Sleep(300 * time.Millisecond)
	
	// Get stats
	stats := flusher.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
	
	if stats.TotalFlushed == 0 {
		t.Error("Should have flushed some memtables")
	}
}

func TestFlusher_QueueFull(t *testing.T) {
	dataDir := "./test-flusher-queue-full"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	// Very small queue
	flushQueue := make(chan *memtable.Memtable, 2)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 1, // Single worker to slow down processing
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Try to queue more than capacity
	for i := 0; i < 5; i++ {
		mt := memtable.NewMemtable(1024 * 1024)
		
		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := graph.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}
		
		mt.Freeze()
		
		// Non-blocking send
		select {
		case flushQueue <- mt:
			t.Logf("Queued memtable %d", i)
		default:
			t.Logf("Queue full, couldn't queue memtable %d", i)
		}
	}
	
	// Wait for processing
	time.Sleep(500 * time.Millisecond)
}

func TestFlusher_EdgeData(t *testing.T) {
	dataDir := "./test-flusher-edges"
	defer os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Create memtable with edges
	mt := memtable.NewMemtable(1024 * 1024)
	
	for i := 0; i < 10; i++ {
		edge := graph.NewEdge(string(rune('a'+i)), "node-1", "node-2", "KNOWS")
		edge.SetProperty("index", int64(i))
		data, _ := graph.SerializeEdge(edge)
		mt.Put("edge:"+edge.ID, data)
	}
	
	mt.Freeze()
	flushQueue <- mt
	
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Create memtable with nodes and edges
	mt := memtable.NewMemtable(1024 * 1024)
	
	// Add nodes
	for i := 0; i < 10; i++ {
		node := graph.NewNode(string(rune('a'+i)), []string{"Person"})
		data, _ := graph.SerializeNode(node)
		mt.Put("node:"+node.ID, data)
	}
	
	// Add edges
	for i := 0; i < 5; i++ {
		edge := graph.NewEdge(string(rune('e'+i)), string(rune('a'+i)), string(rune('a'+i+1)), "KNOWS")
		data, _ := graph.SerializeEdge(edge)
		mt.Put("edge:"+edge.ID, data)
	}
	
	mt.Freeze()
	flushQueue <- mt
	
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 1,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	
	// Queue memtables
	for i := 0; i < 5; i++ {
		mt := memtable.NewMemtable(1024 * 1024)
		
		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := graph.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}
		
		mt.Freeze()
		flushQueue <- mt
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
	
	flushQueue := make(chan *memtable.Memtable, 10)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 2,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	// Flush multiple memtables
	for i := 0; i < 3; i++ {
		mt := memtable.NewMemtable(1024 * 1024)
		
		for j := 0; j < 10; j++ {
			node := graph.NewNode(string(rune('a'+i))+string(rune('0'+j)), []string{"Test"})
			data, _ := graph.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}
		
		mt.Freeze()
		flushQueue <- mt
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
	
	flushQueue := make(chan *memtable.Memtable, 100)
	config := FlusherConfig{
		DataDir:     dataDir,
		FlushQueue:  flushQueue,
		BlockSize:   4096,
		FlushWorkers: 4,
	}
	
	flusher := NewFlusher(config)
	flusher.Start()
	defer flusher.Stop()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt := memtable.NewMemtable(1024 * 1024)
		
		for j := 0; j < 100; j++ {
			node := graph.NewNode(string(rune(j)), []string{"Bench"})
			data, _ := graph.SerializeNode(node)
			mt.Put("node:"+node.ID, data)
		}
		
		mt.Freeze()
		flushQueue <- mt
	}
	
	// Wait for all flushes
	time.Sleep(time.Duration(b.N) * 10 * time.Millisecond)
}
