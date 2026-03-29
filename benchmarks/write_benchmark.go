// Package benchmarks provides write performance testing for the graph database.
package benchmarks

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// WriteBenchmark performs intensive write operations.
type WriteBenchmark struct {
	config  *BenchmarkConfig
	engine  *BenchmarkEngine
	metrics *Metrics
}

// NewWriteBenchmark creates a new write benchmark.
func NewWriteBenchmark(config *BenchmarkConfig) *WriteBenchmark {
	if config == nil {
		config = DefaultBenchmarkConfig()
	}

	return &WriteBenchmark{
		config:  config,
		metrics: NewMetrics(),
	}
}

// Run executes the write benchmark.
func (wb *WriteBenchmark) Run() error {
	fmt.Printf("=== Write Benchmark ===\n")
	fmt.Printf("Virtual Users: %d\n", wb.config.NumVirtualUsers)
	fmt.Printf("Total Operations: %d\n", wb.config.TotalOperations)
	fmt.Printf("Warmup Operations: %d\n", wb.config.WarmupOps)

	// Initialize engine
	var err error
	wb.engine, err = NewBenchmarkEngine(wb.config)
	if err != nil {
		return fmt.Errorf("failed to create benchmark engine: %w", err)
	}

	if err := wb.engine.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer wb.engine.Stop()

	// Perform warmup
	if wb.config.WarmupOps > 0 {
		fmt.Printf("\nPerforming warmup (%d operations)...\n", wb.config.WarmupOps)
		if err := wb.warmup(); err != nil {
			return fmt.Errorf("warmup failed: %w", err)
		}
		fmt.Printf("Warmup completed.\n")
	}

	// Reset metrics after warmup
	wb.metrics = NewMetrics()

	// Run the actual benchmark
	fmt.Printf("\nStarting write benchmark...\n")
	startTime := time.Now()

	if err := wb.runWriteTest(); err != nil {
		return fmt.Errorf("write test failed: %w", err)
	}

	duration := time.Since(startTime)
	engineStats := wb.engine.GetEngine().Stats()
	wb.metrics.Finalize(duration, engineStats)

	// Print results
	wb.metrics.PrintSummary()

	return nil
}

// warmup performs warmup operations to stabilize the system.
func (wb *WriteBenchmark) warmup() error {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, wb.config.NumVirtualUsers)

	opsPerWorker := wb.config.WarmupOps / wb.config.NumVirtualUsers
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	var failedOps int64

	for worker := 0; worker < wb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				nodeID := fmt.Sprintf("warmup-node-%d-%d", workerID, i)
				node := wb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

				if err := wb.engine.GetEngine().CreateNode(ctx, node); err != nil {
					atomic.AddInt64(&failedOps, 1)
					log.Printf("Warmup write failed: %v", err)
				}
			}
		}(worker)
	}

	wg.Wait()

	if failedOps > 0 {
		return fmt.Errorf("warmup had %d failed operations", failedOps)
	}

	return nil
}

// runWriteTest executes the main write benchmark.
func (wb *WriteBenchmark) runWriteTest() error {
	ctx := context.Background()

	// Use atomic counters for progress tracking
	var completedOps int64
	var failedOps int64

	// Worker pool pattern
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, wb.config.NumVirtualUsers)

	// Calculate operations per worker
	opsPerWorker := wb.config.TotalOperations / wb.config.NumVirtualUsers
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
					progress := float64(total) / float64(wb.config.TotalOperations) * 100
					successRate := float64(completed) / float64(total) * 100
					fmt.Printf("\rProgress: %.1f%% (%d/%d) | Success Rate: %.1f%% | Failed: %d",
						progress, total, wb.config.TotalOperations, successRate, failed)
				}
			case <-progressCtx.Done():
				return
			}
		}
	}()

	// Launch workers
	for worker := 0; worker < wb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				start := time.Now()

				// Generate test data
				nodeID := fmt.Sprintf("bench-node-%d-%d", workerID, i)
				node := wb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

				// Perform write operation
				err := wb.engine.GetEngine().CreateNode(ctx, node)
				latency := time.Since(start)

				// Record metrics
				wb.metrics.RecordOperation("write", latency, err)

				if err != nil {
					atomic.AddInt64(&failedOps, 1)
				} else {
					atomic.AddInt64(&completedOps, 1)

					// Occasionally create edges to simulate real workload
					if i > 0 && i%10 == 0 {
						wb.createRandomEdge(ctx, workerID, i)
					}
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

// createRandomEdge creates a random edge for more realistic workload.
func (wb *WriteBenchmark) createRandomEdge(ctx context.Context, workerID, opIndex int) {
	start := time.Now()

	// Only create edges if we have existing nodes to connect to
	// This prevents race condition where edges reference non-existent nodes
	if opIndex == 0 {
		return // Skip edge creation for first operation
	}

	// Connect to a previously created node from the same worker
	fromNodeID := fmt.Sprintf("bench-node-%d-%d", workerID, opIndex)
	toNodeID := fmt.Sprintf("bench-node-%d-%d", workerID, opIndex-1)

	edgeID := fmt.Sprintf("bench-edge-%d-%d", workerID, opIndex)
	edge := wb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, fromNodeID, toNodeID)

	err := wb.engine.GetEngine().CreateEdge(ctx, edge)
	latency := time.Since(start)

	wb.metrics.RecordOperation("write", latency, err)
}

// WriteBenchmarkConfig holds specific write benchmark configuration.
type WriteBenchmarkConfig struct {
	*BenchmarkConfig
	NodeRatio float64 // Ratio of nodes vs edges (0.7 = 70% nodes, 30% edges)
	BatchSize int     // Operations per batch (0 = no batching)
}

// DefaultWriteBenchmarkConfig returns defaults for write benchmarks.
func DefaultWriteBenchmarkConfig() *WriteBenchmarkConfig {
	return &WriteBenchmarkConfig{
		BenchmarkConfig: DefaultBenchmarkConfig(),
		NodeRatio:       0.7, // 70% nodes, 30% edges
		BatchSize:       0,   // No batching by default
	}
}

// BatchWriteBenchmark performs batched write operations for higher throughput.
type BatchWriteBenchmark struct {
	config  *WriteBenchmarkConfig
	engine  *BenchmarkEngine
	metrics *Metrics
}

// NewBatchWriteBenchmark creates a new batch write benchmark.
func NewBatchWriteBenchmark(config *WriteBenchmarkConfig) *BatchWriteBenchmark {
	if config == nil {
		config = DefaultWriteBenchmarkConfig()
	}

	return &BatchWriteBenchmark{
		config:  config,
		metrics: NewMetrics(),
	}
}

// Run executes the batch write benchmark.
func (bwb *BatchWriteBenchmark) Run() error {
	fmt.Printf("=== Batch Write Benchmark ===\n")
	fmt.Printf("Virtual Users: %d\n", bwb.config.NumVirtualUsers)
	fmt.Printf("Total Operations: %d\n", bwb.config.TotalOperations)
	fmt.Printf("Batch Size: %d\n", bwb.config.BatchSize)
	fmt.Printf("Node/Edge Ratio: %.1f/%.1f\n", bwb.config.NodeRatio, 1.0-bwb.config.NodeRatio)

	// Initialize engine
	var err error
	bwb.engine, err = NewBenchmarkEngine(bwb.config.BenchmarkConfig)
	if err != nil {
		return fmt.Errorf("failed to create benchmark engine: %w", err)
	}

	if err := bwb.engine.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer bwb.engine.Stop()

	// Run the benchmark
	fmt.Printf("\nStarting batch write benchmark...\n")
	startTime := time.Now()

	if err := bwb.runBatchWriteTest(); err != nil {
		return fmt.Errorf("batch write test failed: %w", err)
	}

	duration := time.Since(startTime)
	engineStats := bwb.engine.GetEngine().Stats()
	bwb.metrics.Finalize(duration, engineStats)

	// Print results
	bwb.metrics.PrintSummary()

	return nil
}

// runBatchWriteTest executes batched write operations.
func (bwb *BatchWriteBenchmark) runBatchWriteTest() error {
	ctx := context.Background()

	var completedOps int64
	var failedOps int64

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, bwb.config.NumVirtualUsers)

	// Calculate batches per worker
	totalBatches := bwb.config.TotalOperations / bwb.config.BatchSize
	if totalBatches == 0 {
		totalBatches = 1
	}
	batchesPerWorker := totalBatches / bwb.config.NumVirtualUsers
	if batchesPerWorker == 0 {
		batchesPerWorker = 1
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
					progress := float64(total) / float64(bwb.config.TotalOperations) * 100
					successRate := float64(completed) / float64(total) * 100
					fmt.Printf("\rProgress: %.1f%% (%d/%d) | Success Rate: %.1f%% | Failed: %d",
						progress, total, bwb.config.TotalOperations, successRate, failed)
				}
			case <-progressCtx.Done():
				return
			}
		}
	}()

	// Launch batch workers
	for worker := 0; worker < bwb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for batch := 0; batch < batchesPerWorker; batch++ {
				batchStart := time.Now()

				// Process a batch of operations
				batchErrors := 0
				for i := 0; i < bwb.config.BatchSize; i++ {
					opStart := time.Now()

					var err error
					if bwb.shouldCreateNode() {
						err = bwb.createNode(ctx, workerID, batch, i)
					} else {
						err = bwb.createEdge(ctx, workerID, batch, i)
					}

					latency := time.Since(opStart)
					bwb.metrics.RecordOperation("write", latency, err)

					if err != nil {
						batchErrors++
					}
				}

				_ = time.Since(batchStart) // Track batch processing time
				atomic.AddInt64(&completedOps, int64(bwb.config.BatchSize-batchErrors))
				atomic.AddInt64(&failedOps, int64(batchErrors))
			}
		}(worker)
	}

	wg.Wait()
	progressCancel() // Signal progress goroutine to stop
	progressWg.Wait()
	fmt.Printf("\n") // New line after progress indicator

	return nil
}

// shouldCreateNode determines if the next operation should be a node creation.
func (bwb *BatchWriteBenchmark) shouldCreateNode() bool {
	return bwb.engine.GetWorkloadGenerator().rng.Float64() < bwb.config.NodeRatio
}

// createNode creates a node for the batch benchmark.
func (bwb *BatchWriteBenchmark) createNode(ctx context.Context, workerID, batch, index int) error {
	nodeID := fmt.Sprintf("batch-node-%d-%d-%d", workerID, batch, index)
	node := bwb.engine.GetWorkloadGenerator().GenerateNode(nodeID)
	return bwb.engine.GetEngine().CreateNode(ctx, node)
}

// createEdge creates an edge for the batch benchmark.
func (bwb *BatchWriteBenchmark) createEdge(ctx context.Context, workerID, batch, index int) error {
	// Find existing nodes to connect
	fromNodeID := fmt.Sprintf("batch-node-%d-%d-%d", workerID, batch, index-1)
	if index == 0 && batch > 0 {
		fromNodeID = fmt.Sprintf("batch-node-%d-%d-%d", workerID, batch-1, bwb.config.BatchSize-1)
	}

	toNodeID := fmt.Sprintf("batch-node-%d-%d-%d",
		(workerID+1)%bwb.config.NumVirtualUsers, batch, index)

	edgeID := fmt.Sprintf("batch-edge-%d-%d-%d", workerID, batch, index)
	edge := bwb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, fromNodeID, toNodeID)

	return bwb.engine.GetEngine().CreateEdge(ctx, edge)
}
