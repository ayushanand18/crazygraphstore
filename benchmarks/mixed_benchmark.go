// Package benchmarks provides mixed workload performance testing for the graph database.
package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MixedBenchmark performs concurrent mixed read/write operations.
type MixedBenchmark struct {
	config  *MixedWorkloadConfig
	engine  *BenchmarkEngine
	metrics *Metrics
	nodeIDs []string
	edgeIDs []string
}

// OperationType represents different operation types in mixed workload.
type OperationType int

const (
	OpTypeCreateNode OperationType = iota
	OpTypeCreateEdge
	OpTypeReadNode
	OpTypeReadEdge
	OpTypeUpdateNode
	OpTypeTraversal
)

// String returns string representation of operation type.
func (ot OperationType) String() string {
	switch ot {
	case OpTypeCreateNode:
		return "CreateNode"
	case OpTypeCreateEdge:
		return "CreateEdge"
	case OpTypeReadNode:
		return "ReadNode"
	case OpTypeReadEdge:
		return "ReadEdge"
	case OpTypeUpdateNode:
		return "UpdateNode"
	case OpTypeTraversal:
		return "Traversal"
	default:
		return "Unknown"
	}
}

// MixedWorkloadConfig defines the distribution of operations in mixed workload.
type MixedWorkloadConfig struct {
	*BenchmarkConfig
	WriteRatio     float64 // Percentage of write operations (vs reads)
	CreateRatio    float64 // Within writes, percentage of creates (vs updates)
	NodeRatio      float64 // Within operations, percentage of node operations (vs edges)
	TraversalRatio float64 // Percentage of traversal operations
}

// DefaultMixedWorkloadConfig returns defaults for mixed workload benchmarks.
func DefaultMixedWorkloadConfig() *MixedWorkloadConfig {
	return &MixedWorkloadConfig{
		BenchmarkConfig: DefaultBenchmarkConfig(),
		WriteRatio:      0.3, // 30% writes, 70% reads
		CreateRatio:     0.8, // 80% creates, 20% updates within writes
		NodeRatio:       0.7, // 70% node operations, 30% edge operations
		TraversalRatio:  0.1, // 10% traversals, 90% point operations
	}
}

// NewMixedBenchmark creates a new mixed workload benchmark.
func NewMixedBenchmark(config *MixedWorkloadConfig) *MixedBenchmark {
	if config == nil {
		config = DefaultMixedWorkloadConfig()
	}

	return &MixedBenchmark{
		config:  config,
		metrics: NewMetrics(),
	}
}

// Run executes the mixed workload benchmark.
func (mb *MixedBenchmark) Run() error {
	fmt.Printf("=== Mixed Workload Benchmark ===\n")
	fmt.Printf("Virtual Users: %d\n", mb.config.NumVirtualUsers)
	fmt.Printf("Total Operations: %d\n", mb.config.TotalOperations)
	fmt.Printf("Write/Read Ratio: %.1f/%.1f\n", mb.config.WriteRatio, 1.0-mb.config.WriteRatio)
	fmt.Printf("Create/Update Ratio: %.1f/%.1f\n", mb.config.CreateRatio, 1.0-mb.config.CreateRatio)
	fmt.Printf("Node/Edge Ratio: %.1f/%.1f\n", mb.config.NodeRatio, 1.0-mb.config.NodeRatio)
	fmt.Printf("Traversal/Point Ratio: %.1f/%.1f\n", mb.config.TraversalRatio, 1.0-mb.config.TraversalRatio)

	// Initialize engine
	var err error
	mb.engine, err = NewBenchmarkEngine(mb.config.BenchmarkConfig)
	if err != nil {
		return fmt.Errorf("failed to create benchmark engine: %w", err)
	}

	if err := mb.engine.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer mb.engine.Stop()

	// Pre-populate with initial data
	fmt.Printf("\nPre-populating initial dataset...\n")
	if err := mb.prePopulateInitialData(); err != nil {
		return fmt.Errorf("failed to pre-populate initial data: %w", err)
	}
	fmt.Printf("Pre-populated %d nodes and %d edges\n", len(mb.nodeIDs), len(mb.edgeIDs))

	// Run the benchmark
	fmt.Printf("\nStarting mixed workload benchmark...\n")
	startTime := time.Now()

	if err := mb.runMixedWorkload(); err != nil {
		return fmt.Errorf("mixed workload test failed: %w", err)
	}

	duration := time.Since(startTime)
	engineStats := mb.engine.GetEngine().Stats()
	mb.metrics.Finalize(duration, engineStats)

	// Print results
	mb.metrics.PrintSummary()

	return nil
}

// prePopulateInitialData creates initial dataset for mixed workload.
func (mb *MixedBenchmark) prePopulateInitialData() error {
	ctx := context.Background()
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, mb.config.NumVirtualUsers)

	// Create initial dataset (25% of total operations)
	initialSize := mb.config.TotalOperations / 4
	if initialSize == 0 {
		initialSize = 100
	}

	opsPerWorker := initialSize / mb.config.NumVirtualUsers
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	mb.nodeIDs = make([]string, 0, initialSize)
	mb.edgeIDs = make([]string, 0, initialSize)

	var mu sync.Mutex
	var failedOps int64

	for worker := 0; worker < mb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				// Create nodes
				nodeID := fmt.Sprintf("mixed-node-%d-%d", workerID, i)
				node := mb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

				if err := mb.engine.GetEngine().CreateNode(ctx, node); err != nil {
					atomic.AddInt64(&failedOps, 1)
					continue
				}

				mu.Lock()
				mb.nodeIDs = append(mb.nodeIDs, nodeID)
				mu.Unlock()

				// Create edges occasionally
				if i > 0 && i%3 == 0 && len(mb.nodeIDs) > 1 {
					edgeID := fmt.Sprintf("mixed-edge-%d-%d", workerID, i)
					fromNodeID := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]

					var toNodeID string
					for {
						candidate := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]
						if candidate != fromNodeID {
							toNodeID = candidate
							break
						}
					}

					edge := mb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, fromNodeID, toNodeID)

					if err := mb.engine.GetEngine().CreateEdge(ctx, edge); err != nil {
						atomic.AddInt64(&failedOps, 1)
						continue
					}

					mu.Lock()
					mb.edgeIDs = append(mb.edgeIDs, edgeID)
					mu.Unlock()
				}
			}
		}(worker)
	}

	wg.Wait()

	if failedOps > 0 {
		return fmt.Errorf("pre-population had %d failed operations", failedOps)
	}

	return nil
}

// runMixedWorkload executes the mixed workload test.
func (mb *MixedBenchmark) runMixedWorkload() error {
	ctx := context.Background()

	var completedOps int64
	var failedOps int64
	var operationCounters [6]int64 // Count each operation type

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, mb.config.NumVirtualUsers)

	opsPerWorker := mb.config.TotalOperations / mb.config.NumVirtualUsers
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
					progress := float64(total) / float64(mb.config.TotalOperations) * 100
					successRate := float64(completed) / float64(total) * 100
					fmt.Printf("\rProgress: %.1f%% (%d/%d) | Success Rate: %.1f%% | Failed: %d",
						progress, total, mb.config.TotalOperations, successRate, failed)
				}
			case <-progressCtx.Done():
				return
			}
		}
	}()

	// Launch workers
	for worker := 0; worker < mb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				start := time.Now()

				// Determine operation type based on workload configuration
				opType := mb.selectOperationType()

				var err error
				switch opType {
				case OpTypeCreateNode:
					err = mb.performCreateNode(ctx, workerID, i)
					atomic.AddInt64(&operationCounters[OpTypeCreateNode], 1)
				case OpTypeCreateEdge:
					err = mb.performCreateEdge(ctx, workerID, i)
					atomic.AddInt64(&operationCounters[OpTypeCreateEdge], 1)
				case OpTypeReadNode:
					err = mb.performReadNode(ctx)
					atomic.AddInt64(&operationCounters[OpTypeReadNode], 1)
				case OpTypeReadEdge:
					err = mb.performReadEdge(ctx)
					atomic.AddInt64(&operationCounters[OpTypeReadEdge], 1)
				case OpTypeUpdateNode:
					err = mb.performUpdateNode(ctx)
					atomic.AddInt64(&operationCounters[OpTypeUpdateNode], 1)
				case OpTypeTraversal:
					err = mb.performTraversal(ctx)
					atomic.AddInt64(&operationCounters[OpTypeTraversal], 1)
				}

				latency := time.Since(start)

				// Record metrics
				mb.metrics.RecordOperation(opType.String(), latency, err)

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

	// Print operation breakdown
	fmt.Printf("\nOperation Breakdown:\n")
	for i, count := range operationCounters {
		if count > 0 {
			fmt.Printf("  %s: %d\n", OperationType(i).String(), count)
		}
	}

	return nil
}

// selectOperationType determines the next operation type based on workload configuration.
func (mb *MixedBenchmark) selectOperationType() OperationType {
	rng := mb.engine.GetWorkloadGenerator().rng

	// First decide if it's a traversal operation
	if rng.Float64() < mb.config.TraversalRatio {
		return OpTypeTraversal
	}

	// Then decide between read and write
	if rng.Float64() < mb.config.WriteRatio {
		// Write operation
		if rng.Float64() < mb.config.CreateRatio {
			// Create operation
			if rng.Float64() < mb.config.NodeRatio {
				return OpTypeCreateNode
			} else {
				return OpTypeCreateEdge
			}
		} else {
			// Update operation (only nodes for now)
			return OpTypeUpdateNode
		}
	} else {
		// Read operation
		if rng.Float64() < mb.config.NodeRatio {
			return OpTypeReadNode
		} else {
			return OpTypeReadEdge
		}
	}
}

// performCreateNode creates a new node.
func (mb *MixedBenchmark) performCreateNode(ctx context.Context, workerID, opIndex int) error {
	nodeID := fmt.Sprintf("runtime-node-%d-%d-%d", workerID, opIndex, time.Now().UnixNano())
	node := mb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

	err := mb.engine.GetEngine().CreateNode(ctx, node)
	if err == nil {
		// Add to node IDs for future operations
		mb.nodeIDs = append(mb.nodeIDs, nodeID)
	}
	return err
}

// performCreateEdge creates a new edge.
func (mb *MixedBenchmark) performCreateEdge(ctx context.Context, workerID, opIndex int) error {
	if len(mb.nodeIDs) < 2 {
		return fmt.Errorf("insufficient nodes for edge creation")
	}

	edgeID := fmt.Sprintf("runtime-edge-%d-%d-%d", workerID, opIndex, time.Now().UnixNano())

	// Select two random nodes
	fromNodeID := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]
	var toNodeID string
	for {
		candidate := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]
		if candidate != fromNodeID {
			toNodeID = candidate
			break
		}
	}

	edge := mb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, fromNodeID, toNodeID)

	err := mb.engine.GetEngine().CreateEdge(ctx, edge)
	if err == nil {
		// Add to edge IDs for future operations
		mb.edgeIDs = append(mb.edgeIDs, edgeID)
	}
	return err
}

// performReadNode reads an existing node.
func (mb *MixedBenchmark) performReadNode(ctx context.Context) error {
	if len(mb.nodeIDs) == 0 {
		return fmt.Errorf("no nodes available for reading")
	}

	nodeID := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]
	_, err := mb.engine.GetEngine().GetNode(ctx, nodeID)
	return err
}

// performReadEdge reads an existing edge.
func (mb *MixedBenchmark) performReadEdge(ctx context.Context) error {
	if len(mb.edgeIDs) == 0 {
		return fmt.Errorf("no edges available for reading")
	}

	edgeID := mb.edgeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.edgeIDs))]
	_, err := mb.engine.GetEngine().GetEdge(ctx, edgeID)
	return err
}

// performUpdateNode updates an existing node.
func (mb *MixedBenchmark) performUpdateNode(ctx context.Context) error {
	if len(mb.nodeIDs) == 0 {
		return fmt.Errorf("no nodes available for updating")
	}

	nodeID := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]

	// Update with random properties
	updates := make(map[string]interface{})
	rng := mb.engine.GetWorkloadGenerator().rng

	if rng.Intn(2) == 0 {
		updates["last_accessed"] = time.Now()
	}
	if rng.Intn(3) == 0 {
		updates["access_count"] = rng.Int63n(1000)
	}

	return mb.engine.GetEngine().UpdateNode(ctx, nodeID, updates)
}

// performTraversal performs a graph traversal operation.
func (mb *MixedBenchmark) performTraversal(ctx context.Context) error {
	if len(mb.nodeIDs) == 0 {
		return fmt.Errorf("no nodes available for traversal")
	}

	nodeID := mb.nodeIDs[mb.engine.GetWorkloadGenerator().Intn(len(mb.nodeIDs))]
	_, err := mb.engine.GetEngine().GetNeighbors(ctx, nodeID, 0) // DirectionOut
	return err
}

// ConcurrencyStressBenchmark performs high-concurrency stress testing.
type ConcurrencyStressBenchmark struct {
	config  *StressTestConfig
	engine  *BenchmarkEngine
	metrics *Metrics
	nodeIDs []string
	edgeIDs []string
}

// StressTestConfig holds configuration for stress testing.
type StressTestConfig struct {
	*BenchmarkConfig
	ContentionLevel float64 // 0.0 = no contention, 1.0 = maximum contention
	HotspotRatio    float64 // Ratio of operations targeting hotspots
}

// DefaultStressTestConfig returns defaults for stress testing.
func DefaultStressTestConfig() *StressTestConfig {
	return &StressTestConfig{
		BenchmarkConfig: DefaultBenchmarkConfig(),
		ContentionLevel: 0.8, // High contention
		HotspotRatio:    0.9, // 90% of operations on hotspots
	}
}

// NewConcurrencyStressBenchmark creates a new stress test benchmark.
func NewConcurrencyStressBenchmark(config *StressTestConfig) *ConcurrencyStressBenchmark {
	if config == nil {
		config = DefaultStressTestConfig()
	}

	return &ConcurrencyStressBenchmark{
		config:  config,
		metrics: NewMetrics(),
	}
}

// Run executes the concurrency stress benchmark.
func (csb *ConcurrencyStressBenchmark) Run() error {
	fmt.Printf("=== Concurrency Stress Benchmark ===\n")
	fmt.Printf("Virtual Users: %d\n", csb.config.NumVirtualUsers)
	fmt.Printf("Total Operations: %d\n", csb.config.TotalOperations)
	fmt.Printf("Contention Level: %.1f\n", csb.config.ContentionLevel)
	fmt.Printf("Hotspot Ratio: %.1f\n", csb.config.HotspotRatio)

	// Initialize engine
	var err error
	csb.engine, err = NewBenchmarkEngine(csb.config.BenchmarkConfig)
	if err != nil {
		return fmt.Errorf("failed to create benchmark engine: %w", err)
	}

	if err := csb.engine.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer csb.engine.Stop()

	// Pre-populate with data
	fmt.Printf("\nPre-populating stress test data...\n")
	if err := csb.prePopulateStressData(); err != nil {
		return fmt.Errorf("failed to pre-populate stress data: %w", err)
	}
	fmt.Printf("Pre-populated %d nodes and %d edges\n", len(csb.nodeIDs), len(csb.edgeIDs))

	// Run the stress test
	fmt.Printf("\nStarting concurrency stress test...\n")
	startTime := time.Now()

	if err := csb.runStressTest(); err != nil {
		return fmt.Errorf("stress test failed: %w", err)
	}

	duration := time.Since(startTime)
	engineStats := csb.engine.GetEngine().Stats()
	csb.metrics.Finalize(duration, engineStats)

	// Print results
	csb.metrics.PrintSummary()

	return nil
}

// prePopulateStressData creates data for stress testing.
func (csb *ConcurrencyStressBenchmark) prePopulateStressData() error {
	ctx := context.Background()

	// Create a smaller dataset to force contention
	dataSize := csb.config.TotalOperations / 20
	if dataSize == 0 {
		dataSize = 50
	}

	csb.nodeIDs = make([]string, 0, dataSize)
	csb.edgeIDs = make([]string, 0, dataSize)

	// Create nodes
	for i := 0; i < dataSize; i++ {
		nodeID := fmt.Sprintf("stress-node-%d", i)
		node := csb.engine.GetWorkloadGenerator().GenerateNode(nodeID)

		if err := csb.engine.GetEngine().CreateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to create stress node %s: %w", nodeID, err)
		}

		csb.nodeIDs = append(csb.nodeIDs, nodeID)
	}

	// Create some edges
	for i := 0; i < dataSize/2; i++ {
		edgeID := fmt.Sprintf("stress-edge-%d", i)
		fromNodeID := csb.nodeIDs[i]
		toNodeID := csb.nodeIDs[(i+1)%len(csb.nodeIDs)]

		edge := csb.engine.GetWorkloadGenerator().GenerateEdge(edgeID, fromNodeID, toNodeID)

		if err := csb.engine.GetEngine().CreateEdge(ctx, edge); err != nil {
			return fmt.Errorf("failed to create stress edge %s: %w", edgeID, err)
		}

		csb.edgeIDs = append(csb.edgeIDs, edgeID)
	}

	return nil
}

// runStressTest executes high-concurrency operations with contention.
func (csb *ConcurrencyStressBenchmark) runStressTest() error {
	ctx := context.Background()

	var completedOps int64
	var failedOps int64

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, csb.config.NumVirtualUsers)

	opsPerWorker := csb.config.TotalOperations / csb.config.NumVirtualUsers
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
					progress := float64(total) / float64(csb.config.TotalOperations) * 100
					successRate := float64(completed) / float64(total) * 100
					fmt.Printf("\rProgress: %.1f%% (%d/%d) | Success Rate: %.1f%% | Failed: %d",
						progress, total, csb.config.TotalOperations, successRate, failed)
				}
			case <-progressCtx.Done():
				return
			}
		}
	}()

	// Launch high-concurrency workers
	for worker := 0; worker < csb.config.NumVirtualUsers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for i := 0; i < opsPerWorker; i++ {
				start := time.Now()

				var err error
				if csb.engine.GetWorkloadGenerator().Float64() < 0.7 {
					// Read operation (70%)
					err = csb.performStressRead(ctx)
				} else {
					// Write operation (30%)
					err = csb.performStressWrite(ctx, workerID)
				}

				latency := time.Since(start)

				// Record metrics
				csb.metrics.RecordOperation("stress", latency, err)

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

// performStressRead performs a stress read with hotspot access.
func (csb *ConcurrencyStressBenchmark) performStressRead(ctx context.Context) error {
	var nodeID string

	if csb.engine.GetWorkloadGenerator().Float64() < csb.config.HotspotRatio {
		// Access hotspot (first 10% of nodes)
		hotspotSize := len(csb.nodeIDs) / 10
		if hotspotSize == 0 {
			hotspotSize = 1
		}
		nodeID = csb.nodeIDs[csb.engine.GetWorkloadGenerator().Intn(hotspotSize)]
	} else {
		// Access random node
		nodeID = csb.nodeIDs[csb.engine.GetWorkloadGenerator().Intn(len(csb.nodeIDs))]
	}

	_, err := csb.engine.GetEngine().GetNode(ctx, nodeID)
	return err
}

// performStressWrite performs a stress write with contention.
func (csb *ConcurrencyStressBenchmark) performStressWrite(ctx context.Context, workerID int) error {
	var nodeID string

	if csb.engine.GetWorkloadGenerator().Float64() < csb.config.HotspotRatio {
		// Update hotspot (first 10% of nodes)
		hotspotSize := len(csb.nodeIDs) / 10
		if hotspotSize == 0 {
			hotspotSize = 1
		}
		nodeID = csb.nodeIDs[csb.engine.GetWorkloadGenerator().Intn(hotspotSize)]
	} else {
		// Update random node
		nodeID = csb.nodeIDs[csb.engine.GetWorkloadGenerator().Intn(len(csb.nodeIDs))]
	}

	// Create contention by updating the same properties
	updates := map[string]interface{}{
		"stress_counter": time.Now().UnixNano(),
		"last_stress":    workerID,
	}

	return csb.engine.GetEngine().UpdateNode(ctx, nodeID, updates)
}
