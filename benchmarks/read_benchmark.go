// Package benchmarks provides read performance testing for the graph database.
package benchmarks

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ReadBenchmark performs intensive read operations with various access patterns.
type ReadBenchmark struct {
	config        *BenchmarkConfig
	engine        *BenchmarkEngine
	metrics       *Metrics
	nodeIDs       []string
	edgeIDs       []string
	accessPattern AccessPattern
}

// AccessPattern defines different read access patterns.
type AccessPattern int

const (
	AccessPatternRandom     AccessPattern = iota // Random access
	AccessPatternSequential                      // Sequential access
	AccessPatternHotspot                         // 80/20 hotspot access
	AccessPatternZipfian                         // Zipfian distribution
)

// String returns string representation of access pattern.
func (ap AccessPattern) String() string {
	switch ap {
	case AccessPatternRandom:
		return "Random"
	case AccessPatternSequential:
		return "Sequential"
	case AccessPatternHotspot:
		return "Hotspot (80/20)"
	case AccessPatternZipfian:
		return "Zipfian"
	default:
		return "Unknown"
	}
}

// ReadBenchmarkConfig holds specific read benchmark configuration.
type ReadBenchmarkConfig struct {
	*BenchmarkConfig
	AccessPattern      AccessPattern
	ReadRatio          float64 // Ratio of node reads vs edge reads
	CacheWarmupOps     int     // Number of operations to warm cache
	EnableCacheMetrics bool    // Track cache hit/miss rates
}

// DefaultReadBenchmarkConfig returns defaults for read benchmarks.
func DefaultReadBenchmarkConfig() *ReadBenchmarkConfig {
	return &ReadBenchmarkConfig{
		BenchmarkConfig:    DefaultBenchmarkConfig(),
		AccessPattern:      AccessPatternRandom,
		ReadRatio:          0.7, // 70% node reads, 30% edge reads
		CacheWarmupOps:     5000,
		EnableCacheMetrics: true,
	}
}

// NewReadBenchmark creates a new read benchmark.
func NewReadBenchmark(config *ReadBenchmarkConfig) *ReadBenchmark {
	if config == nil {
		config = DefaultReadBenchmarkConfig()
	}

	return &ReadBenchmark{
		config:        config.BenchmarkConfig,
		accessPattern: config.AccessPattern,
		metrics:       NewMetrics(),
	}
}

// Run executes the read benchmark.
func (rb *ReadBenchmark) Run() error {
	fmt.Printf("=== Read Benchmark ===\n")
	fmt.Printf("Virtual Users: %d\n", rb.config.NumVirtualUsers)
	fmt.Printf("Total Operations: %d\n", rb.config.TotalOperations)
	fmt.Printf("Access Pattern: %s\n", rb.accessPattern.String())
	fmt.Printf("Node/Edge Read Ratio: %.1f/%.1f\n", 0.7, 0.3)

	// Initialize engine
	var err error
	rb.engine, err = NewBenchmarkEngine(rb.config)
	if err != nil {
		return fmt.Errorf("failed to create benchmark engine: %w", err)
	}

	if err := rb.engine.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer rb.engine.Stop()

	// Pre-populate with test data
	fmt.Printf("\nPre-populating database with test data...\n")
	if err := rb.prePopulateData(); err != nil {
		return fmt.Errorf("failed to pre-populate data: %w", err)
	}
	fmt.Printf("Pre-populated %d nodes and %d edges\n", len(rb.nodeIDs), len(rb.edgeIDs))

	// Warm up cache
	if rb.config.WarmupOps > 0 {
		fmt.Printf("\nWarming up cache (%d operations)...\n", rb.config.WarmupOps)
		if err := rb.warmupCache(); err != nil {
			return fmt.Errorf("cache warmup failed: %w", err)
		}
		fmt.Printf("Cache warmup completed.\n")
	}

	// Reset metrics after warmup
	rb.metrics = NewMetrics()

	// Run the actual benchmark
	fmt.Printf("\nStarting read benchmark...\n")
	startTime := time.Now()

	if err := rb.runReadTest(); err != nil {
		return fmt.Errorf("read test failed: %w", err)
	}

	duration := time.Since(startTime)
	engineStats := rb.engine.GetEngine().Stats()
	rb.metrics.Finalize(duration, engineStats)

	// Print results
	rb.metrics.PrintSummary()

	return nil
}

// prePopulateData creates test data for read operations.
func (rb *ReadBenchmark) prePopulateData() error {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, rb.config.NumVirtualUsers)

	// Calculate data size (3x the operations for variety)
	dataSize := rb.config.TotalOperations * 3
	opsPerWorker := dataSize / rb.config.NumVirtualUsers
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	rb.nodeIDs = make([]string, 0, dataSize)
	rb.edgeIDs = make([]string, 0, dataSize)

	var mu sync.Mutex
	var failedOps int64

	for worker := 0; worker < rb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				// Create nodes
				nodeID := fmt.Sprintf("read-node-%d-%d", workerID, i)
				node := rb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

				if err := rb.engine.GetEngine().CreateNode(ctx, node); err != nil {
					atomic.AddInt64(&failedOps, 1)
					continue
				}

				mu.Lock()
				rb.nodeIDs = append(rb.nodeIDs, nodeID)
				mu.Unlock()

				// Create edges occasionally
				if i > 0 && i%5 == 0 {
					edgeID := fmt.Sprintf("read-edge-%d-%d", workerID, i)
					fromNodeID := fmt.Sprintf("read-node-%d-%d", workerID, i-1)
					toNodeID := fmt.Sprintf("read-node-%d-%d",
						(workerID+1)%rb.config.NumVirtualUsers, i)

					edge := rb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, fromNodeID, toNodeID)

					if err := rb.engine.GetEngine().CreateEdge(ctx, edge); err != nil {
						atomic.AddInt64(&failedOps, 1)
						continue
					}

					mu.Lock()
					rb.edgeIDs = append(rb.edgeIDs, edgeID)
					mu.Unlock()
				}
			}
		}(worker)
	}

	wg.Wait()

	if failedOps > 0 {
		return fmt.Errorf("pre-population had %d failed operations", failedOps)
	}

	if len(rb.nodeIDs) == 0 {
		return fmt.Errorf("no nodes were created during pre-population")
	}

	return nil
}

// warmupCache performs cache warmup operations.
func (rb *ReadBenchmark) warmupCache() error {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, rb.config.NumVirtualUsers)

	opsPerWorker := rb.config.WarmupOps / rb.config.NumVirtualUsers
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	var failedOps int64

	for worker := 0; worker < rb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				var err error

				if rb.engine.GetWorkloadGenerator().rng.Float64() < 0.7 {
					// Read node
					nodeID := rb.nodeIDs[rb.engine.GetWorkloadGenerator().rng.Intn(len(rb.nodeIDs))]
					_, err = rb.engine.GetEngine().GetNode(ctx, nodeID)
				} else {
					// Read edge
					if len(rb.edgeIDs) > 0 {
						edgeID := rb.edgeIDs[rb.engine.GetWorkloadGenerator().rng.Intn(len(rb.edgeIDs))]
						_, err = rb.engine.GetEngine().GetEdge(ctx, edgeID)
					}
				}

				if err != nil {
					atomic.AddInt64(&failedOps, 1)
					log.Printf("Warmup read failed: %v", err)
				}
			}
		}(worker)
	}

	wg.Wait()

	if failedOps > 0 {
		return fmt.Errorf("cache warmup had %d failed operations", failedOps)
	}

	return nil
}

// runReadTest executes the main read benchmark.
func (rb *ReadBenchmark) runReadTest() error {
	ctx := context.Background()

	var completedOps int64
	var failedOps int64

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, rb.config.NumVirtualUsers)

	opsPerWorker := rb.config.TotalOperations / rb.config.NumVirtualUsers
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	// Progress reporting with proper cancellation
	progressCtx, progressCancel := context.WithCancel(context.Background())
	progressTicker := time.NewTicker(1 * time.Second)
	defer progressTicker.Stop()

	var progressWg sync.WaitGroup
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		defer progressCancel() // Ensure cancellation when done
		for {
			select {
			case <-progressTicker.C:
				completed := atomic.LoadInt64(&completedOps)
				failed := atomic.LoadInt64(&failedOps)
				total := completed + failed
				if total > 0 {
					progress := float64(total) / float64(rb.config.TotalOperations) * 100
					successRate := float64(completed) / float64(total) * 100
					fmt.Printf("\rProgress: %.1f%% (%d/%d) | Success Rate: %.1f%% | Failed: %d",
						progress, total, rb.config.TotalOperations, successRate, failed)
				}
			case <-progressCtx.Done():
				return
			}
		}
	}()

	// Launch workers
	for worker := 0; worker < rb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				start := time.Now()

				var err error
				if rb.engine.GetWorkloadGenerator().rng.Float64() < 0.7 {
					err = rb.performNodeRead(ctx, workerID, i)
				} else {
					err = rb.performEdgeRead(ctx, workerID, i)
				}

				latency := time.Since(start)

				// Record metrics
				rb.metrics.RecordOperation("read", latency, err)

				if err != nil {
					atomic.AddInt64(&failedOps, 1)
				} else {
					atomic.AddInt64(&completedOps, 1)
				}
			}
		}(worker)
	}

	wg.Wait()
	progressCancel() // Signal progress goroutine to stop
	progressWg.Wait()
	fmt.Printf("\n") // New line after progress indicator

	return nil
}

// performNodeRead performs a node read operation based on access pattern.
func (rb *ReadBenchmark) performNodeRead(ctx context.Context, workerID, opIndex int) error {
	if len(rb.nodeIDs) == 0 {
		return fmt.Errorf("no nodes available for reading")
	}

	var nodeID string
	switch rb.accessPattern {
	case AccessPatternRandom:
		nodeID = rb.nodeIDs[rb.engine.GetWorkloadGenerator().rng.Intn(len(rb.nodeIDs))]
	case AccessPatternSequential:
		index := (workerID*1000 + opIndex) % len(rb.nodeIDs)
		nodeID = rb.nodeIDs[index]
	case AccessPatternHotspot:
		// 80% of reads go to 20% of data
		if rb.engine.GetWorkloadGenerator().rng.Float64() < 0.8 {
			hotspotSize := len(rb.nodeIDs) / 5
			if hotspotSize == 0 {
				hotspotSize = 1
			}
			nodeID = rb.nodeIDs[rb.engine.GetWorkloadGenerator().rng.Intn(hotspotSize)]
		} else {
			hotspotSize := len(rb.nodeIDs) / 5
			if hotspotSize == 0 {
				hotspotSize = 1
			}
			nodeID = rb.nodeIDs[hotspotSize+rb.engine.GetWorkloadGenerator().rng.Intn(len(rb.nodeIDs)-hotspotSize)]
		}
	case AccessPatternZipfian:
		nodeID = rb.zipfianSelect(rb.nodeIDs)
	}

	_, err := rb.engine.GetEngine().GetNode(ctx, nodeID)
	return err
}

// performEdgeRead performs an edge read operation based on access pattern.
func (rb *ReadBenchmark) performEdgeRead(ctx context.Context, workerID, opIndex int) error {
	if len(rb.edgeIDs) == 0 {
		return fmt.Errorf("no edges available for reading")
	}

	var edgeID string
	switch rb.accessPattern {
	case AccessPatternRandom:
		edgeID = rb.edgeIDs[rb.engine.GetWorkloadGenerator().rng.Intn(len(rb.edgeIDs))]
	case AccessPatternSequential:
		index := (workerID*1000 + opIndex) % len(rb.edgeIDs)
		edgeID = rb.edgeIDs[index]
	case AccessPatternHotspot:
		if rb.engine.GetWorkloadGenerator().rng.Float64() < 0.8 {
			hotspotSize := len(rb.edgeIDs) / 5
			if hotspotSize == 0 {
				hotspotSize = 1
			}
			edgeID = rb.edgeIDs[rb.engine.GetWorkloadGenerator().rng.Intn(hotspotSize)]
		} else {
			hotspotSize := len(rb.edgeIDs) / 5
			if hotspotSize == 0 {
				hotspotSize = 1
			}
			edgeID = rb.edgeIDs[hotspotSize+rb.engine.GetWorkloadGenerator().rng.Intn(len(rb.edgeIDs)-hotspotSize)]
		}
	case AccessPatternZipfian:
		edgeID = rb.zipfianSelect(rb.edgeIDs)
	}

	_, err := rb.engine.GetEngine().GetEdge(ctx, edgeID)
	return err
}

// zipfianSelect implements Zipfian distribution for more realistic access patterns.
func (rb *ReadBenchmark) zipfianSelect(items []string) string {
	if len(items) == 0 {
		return ""
	}

	// Simple Zipfian approximation using exponential distribution
	rng := rb.engine.GetWorkloadGenerator().rng
	theta := 1.0 // Zipfian parameter

	for {
		u := rng.Float64()
		k := int(float64(len(items)) * pow(u, -1.0/theta))
		if k < len(items) {
			return items[k]
		}
	}
}

// pow calculates x^y for Zipfian distribution.
func pow(x, y float64) float64 {
	if x == 0 && y > 0 {
		return 0
	}
	result := 1.0
	for i := 0; i < int(y); i++ {
		result *= x
	}
	return result
}

// TraversalBenchmark performs graph traversal operations.
type TraversalBenchmark struct {
	config  *BenchmarkConfig
	engine  *BenchmarkEngine
	metrics *Metrics
	nodeIDs []string
}

// NewTraversalBenchmark creates a new traversal benchmark.
func NewTraversalBenchmark(config *BenchmarkConfig) *TraversalBenchmark {
	if config == nil {
		config = DefaultBenchmarkConfig()
	}

	return &TraversalBenchmark{
		config:  config,
		metrics: NewMetrics(),
	}
}

// Run executes the traversal benchmark.
func (tb *TraversalBenchmark) Run() error {
	fmt.Printf("=== Graph Traversal Benchmark ===\n")
	fmt.Printf("Virtual Users: %d\n", tb.config.NumVirtualUsers)
	fmt.Printf("Total Operations: %d\n", tb.config.TotalOperations)

	// Initialize engine
	var err error
	tb.engine, err = NewBenchmarkEngine(tb.config)
	if err != nil {
		return fmt.Errorf("failed to create benchmark engine: %w", err)
	}

	if err := tb.engine.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer tb.engine.Stop()

	// Pre-populate with connected graph data
	fmt.Printf("\nPre-populating connected graph data...\n")
	if err := tb.prePopulateConnectedData(); err != nil {
		return fmt.Errorf("failed to pre-populate connected data: %w", err)
	}
	fmt.Printf("Pre-populated %d nodes with connections\n", len(tb.nodeIDs))

	// Run the benchmark
	fmt.Printf("\nStarting traversal benchmark...\n")
	startTime := time.Now()

	if err := tb.runTraversalTest(); err != nil {
		return fmt.Errorf("traversal test failed: %w", err)
	}

	duration := time.Since(startTime)
	engineStats := tb.engine.GetEngine().Stats()
	tb.metrics.Finalize(duration, engineStats)

	// Print results
	tb.metrics.PrintSummary()

	return nil
}

// prePopulateConnectedData creates a connected graph for traversal testing.
func (tb *TraversalBenchmark) prePopulateConnectedData() error {
	ctx := context.Background()

	// Create a more connected graph structure
	numNodes := tb.config.TotalOperations / 10 // Create fewer nodes but more connections
	if numNodes == 0 {
		numNodes = 100
	}

	tb.nodeIDs = make([]string, 0, numNodes)

	// Create nodes
	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("traversal-node-%d", i)
		node := tb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

		if err := tb.engine.GetEngine().CreateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to create node %s: %w", nodeID, err)
		}

		tb.nodeIDs = append(tb.nodeIDs, nodeID)
	}

	// Create edges to form a connected graph
	for i := 0; i < numNodes; i++ {
		// Connect to next node (forming a chain)
		if i < numNodes-1 {
			edgeID := fmt.Sprintf("traversal-edge-chain-%d", i)
			edge := tb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, tb.nodeIDs[i], tb.nodeIDs[i+1])

			if err := tb.engine.GetEngine().CreateEdge(ctx, edge); err != nil {
				return fmt.Errorf("failed to create chain edge %s: %w", edgeID, err)
			}
		}

		// Add some random connections
		if i > 0 {
			randomTarget := tb.engine.GetWorkloadGenerator().rng.Intn(i)
			edgeID := fmt.Sprintf("traversal-edge-random-%d", i)
			edge := tb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, tb.nodeIDs[i], tb.nodeIDs[randomTarget])

			if err := tb.engine.GetEngine().CreateEdge(ctx, edge); err != nil {
				return fmt.Errorf("failed to create random edge %s: %w", edgeID, err)
			}
		}
	}

	return nil
}

// runTraversalTest executes traversal operations.
func (tb *TraversalBenchmark) runTraversalTest() error {
	ctx := context.Background()

	var completedOps int64
	var failedOps int64

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, tb.config.NumVirtualUsers)

	opsPerWorker := tb.config.TotalOperations / tb.config.NumVirtualUsers
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	// Progress reporting
	progressTicker := time.NewTicker(1 * time.Second)
	defer progressTicker.Stop()

	var progressWg sync.WaitGroup
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		for {
			select {
			case <-progressTicker.C:
				completed := atomic.LoadInt64(&completedOps)
				failed := atomic.LoadInt64(&failedOps)
				total := completed + failed
				if total > 0 {
					progress := float64(total) / float64(tb.config.TotalOperations) * 100
					successRate := float64(completed) / float64(total) * 100
					fmt.Printf("\rProgress: %.1f%% (%d/%d) | Success Rate: %.1f%% | Failed: %d",
						progress, total, tb.config.TotalOperations, successRate, failed)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Launch workers
	for worker := 0; worker < tb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				start := time.Now()

				// Pick a random starting node
				nodeID := tb.nodeIDs[tb.engine.GetWorkloadGenerator().rng.Intn(len(tb.nodeIDs))]

				// Perform neighbor traversal
				_, err := tb.engine.GetEngine().GetNeighbors(ctx, nodeID, 0) // DirectionOut

				latency := time.Since(start)

				// Record metrics
				tb.metrics.RecordOperation("traversal", latency, err)

				if err != nil {
					atomic.AddInt64(&failedOps, 1)
				} else {
					atomic.AddInt64(&completedOps, 1)
				}
			}
		}(worker)
	}

	wg.Wait()
	progressWg.Wait()
	fmt.Printf("\n") // New line after progress indicator

	return nil
}
