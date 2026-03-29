// Package benchmarks provides utilities for performance testing the graph database.
package benchmarks

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/storage"
)

// BenchmarkConfig holds configuration for benchmark runs.
type BenchmarkConfig struct {
	NumVirtualUsers int           // Number of concurrent virtual users
	TotalOperations int           // Total operations to perform
	Duration        time.Duration // Alternative to TotalOperations - run for duration
	EngineConfig    *storage.Config
	DataDir         string // Temporary data directory for benchmarks
	WarmupOps       int    // Number of warmup operations
}

// DefaultBenchmarkConfig returns sensible defaults.
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		NumVirtualUsers: 1000,
		TotalOperations: 10000,
		Duration:        0, // Use TotalOperations
		EngineConfig:    storage.DefaultConfig(),
		DataDir:         "./benchmark-data",
		WarmupOps:       1000,
	}
}

// Metrics holds benchmark performance metrics.
type Metrics struct {
	mu sync.RWMutex

	// Timing metrics
	TotalDuration    time.Duration
	OperationsPerSec float64
	P50Latency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	P999Latency      time.Duration
	MinLatency       time.Duration
	MaxLatency       time.Duration

	// Operation counts
	TotalOps      int64
	SuccessfulOps int64
	FailedOps     int64
	WriteOps      int64
	ReadOps       int64
	UpdateOps     int64

	// Error tracking
	Errors     []string
	ErrorCount map[string]int64

	// Engine stats
	EngineStats storage.EngineStats

	// Latency distribution
	Latencies []time.Duration
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		ErrorCount: make(map[string]int64),
		Latencies:  make([]time.Duration, 0, 10000),
	}
}

// RecordOperation records a single operation's metrics.
func (m *Metrics) RecordOperation(opType string, latency time.Duration, err error) {
	atomic.AddInt64(&m.TotalOps, 1)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Latencies = append(m.Latencies, latency)

	if err != nil {
		atomic.AddInt64(&m.FailedOps, 1)
		errStr := err.Error()
		m.Errors = append(m.Errors, errStr)
		m.ErrorCount[errStr]++
	} else {
		atomic.AddInt64(&m.SuccessfulOps, 1)
		switch opType {
		case "write":
			atomic.AddInt64(&m.WriteOps, 1)
		case "read":
			atomic.AddInt64(&m.ReadOps, 1)
		case "update":
			atomic.AddInt64(&m.UpdateOps, 1)
		}
	}
}

// Finalize calculates final metrics after benchmark completion.
func (m *Metrics) Finalize(totalDuration time.Duration, engineStats storage.EngineStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalDuration = totalDuration
	m.EngineStats = engineStats

	if len(m.Latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculation
	sorted := make([]time.Duration, len(m.Latencies))
	copy(sorted, m.Latencies)

	// Simple insertion sort for small datasets
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	m.MinLatency = sorted[0]
	m.MaxLatency = sorted[len(sorted)-1]

	// Calculate percentiles
	calcPercentile := func(p float64) time.Duration {
		index := int(float64(len(sorted)) * p / 100.0)
		if index >= len(sorted) {
			index = len(sorted) - 1
		}
		return sorted[index]
	}

	m.P50Latency = calcPercentile(50)
	m.P95Latency = calcPercentile(95)
	m.P99Latency = calcPercentile(99)
	m.P999Latency = calcPercentile(99.9)

	// Calculate operations per second
	if totalDuration > 0 {
		m.OperationsPerSec = float64(m.TotalOps) / totalDuration.Seconds()
	}
}

// PrintSummary prints a formatted summary of benchmark results.
func (m *Metrics) PrintSummary() {
	fmt.Printf("\n=== Benchmark Results ===\n")
	fmt.Printf("Total Duration: %v\n", m.TotalDuration)
	fmt.Printf("Operations/Second: %.2f\n", m.OperationsPerSec)
	fmt.Printf("Total Operations: %d\n", m.TotalOps)
	fmt.Printf("Successful: %d (%.2f%%)\n", m.SuccessfulOps,
		float64(m.SuccessfulOps)/float64(m.TotalOps)*100)
	fmt.Printf("Failed: %d (%.2f%%)\n", m.FailedOps,
		float64(m.FailedOps)/float64(m.TotalOps)*100)

	fmt.Printf("\nOperation Breakdown:\n")
	fmt.Printf("  Writes: %d\n", m.WriteOps)
	fmt.Printf("  Reads: %d\n", m.ReadOps)
	fmt.Printf("  Updates: %d\n", m.UpdateOps)

	fmt.Printf("\nLatency Distribution:\n")
	fmt.Printf("  Min: %v\n", m.MinLatency)
	fmt.Printf("  P50: %v\n", m.P50Latency)
	fmt.Printf("  P95: %v\n", m.P95Latency)
	fmt.Printf("  P99: %v\n", m.P99Latency)
	fmt.Printf("  P99.9: %v\n", m.P999Latency)
	fmt.Printf("  Max: %v\n", m.MaxLatency)

	if len(m.Errors) > 0 {
		fmt.Printf("\nTop Errors:\n")
		for err, count := range m.ErrorCount {
			fmt.Printf("  %s: %d\n", err, count)
		}
	}

	fmt.Printf("\nEngine Statistics:\n")
	stats := m.EngineStats
	fmt.Printf("  Write Lanes: %d\n", stats.NumLanes)
	fmt.Printf("  Total Active Entries: %d\n", stats.TotalActiveCount)
	fmt.Printf("  Total Active Size: %.2f MB\n",
		float64(stats.TotalActiveSize)/(1024*1024))
	fmt.Printf("  Cache Hit Rate: %.2f%%\n",
		stats.CacheStats.OverallHitRate*100)
}

// WorkloadGenerator generates realistic test data.
type WorkloadGenerator struct {
	rng *rand.Rand
	mu  sync.Mutex
}

// NewWorkloadGenerator creates a new workload generator.
func NewWorkloadGenerator(seed int64) *WorkloadGenerator {
	return &WorkloadGenerator{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// Intn returns a random int in [0,n) using synchronized access to rng.
func (wg *WorkloadGenerator) Intn(n int) int {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	return wg.rng.Intn(n)
}

// Float64 returns a random float64 in [0.0,1.0) using synchronized access to rng.
func (wg *WorkloadGenerator) Float64() float64 {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	return wg.rng.Float64()
}

// GenerateNode creates a realistic test node.
func (wg *WorkloadGenerator) GenerateNode(id string) *graph.Node {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	nodeTypes := []string{"Person", "Company", "Product", "Order", "Category"}
	nodeType := nodeTypes[wg.rng.Intn(len(nodeTypes))]

	node := graph.NewNode(id, []string{nodeType})

	switch nodeType {
	case "Person":
		node.SetProperty("name", wg.generateRandomName())
		node.SetProperty("age", int64(20+wg.rng.Intn(60)))
		node.SetProperty("city", wg.generateRandomCity())
		node.SetProperty("email", fmt.Sprintf("%s@example.com", id))
	case "Company":
		node.SetProperty("name", wg.generateRandomCompanyName())
		node.SetProperty("industry", wg.generateRandomIndustry())
		node.SetProperty("founded", int64(1950+wg.rng.Intn(73)))
		node.SetProperty("employees", int64(10+wg.rng.Intn(10000)))
	case "Product":
		node.SetProperty("name", fmt.Sprintf("Product-%s", id))
		node.SetProperty("price", float64(10+wg.rng.Intn(1000)))
		node.SetProperty("category", wg.generateRandomCategory())
		node.SetProperty("in_stock", wg.rng.Intn(2) == 1)
	}

	return node
}

// GenerateEdge creates a realistic test edge.
func (wg *WorkloadGenerator) GenerateEdge(id, fromID, toID string) *graph.Edge {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	edgeTypes := []string{"KNOWS", "WORKS_AT", "BOUGHT", "LIKES", "REVIEWED", "RELATED_TO"}
	edgeType := edgeTypes[wg.rng.Intn(len(edgeTypes))]

	edge := graph.NewEdge(id, fromID, toID, edgeType)

	switch edgeType {
	case "KNOWS":
		edge.SetProperty("since", int64(2010+wg.rng.Intn(13)))
		edge.SetProperty("strength", wg.rng.Float64())
	case "WORKS_AT":
		edge.SetProperty("role", wg.generateRandomRole())
		edge.SetProperty("since", int64(2010+wg.rng.Intn(13)))
	case "BOUGHT":
		edge.SetProperty("quantity", int64(1+wg.rng.Intn(10)))
		edge.SetProperty("price", float64(10+wg.rng.Intn(1000)))
	case "LIKES":
		edge.SetProperty("rating", int64(1+wg.rng.Intn(5)))
		edge.SetProperty("timestamp_ms", time.Now().Add(-time.Duration(wg.rng.Intn(86400))*time.Second).UnixMilli())
	}

	return edge
}

// Helper methods for generating realistic data
func (wg *WorkloadGenerator) generateRandomName() string {
	firstNames := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis"}
	return fmt.Sprintf("%s %s", firstNames[wg.rng.Intn(len(firstNames))], lastNames[wg.rng.Intn(len(lastNames))])
}

func (wg *WorkloadGenerator) generateRandomCity() string {
	cities := []string{"New York", "San Francisco", "London", "Tokyo", "Paris", "Berlin", "Sydney", "Toronto"}
	return cities[wg.rng.Intn(len(cities))]
}

func (wg *WorkloadGenerator) generateRandomCompanyName() string {
	companies := []string{"TechCorp", "DataInc", "CloudSystems", "NetSolutions", "InfoTech", "DigitalLabs"}
	return companies[wg.rng.Intn(len(companies))]
}

func (wg *WorkloadGenerator) generateRandomIndustry() string {
	industries := []string{"Technology", "Finance", "Healthcare", "Retail", "Manufacturing", "Education"}
	return industries[wg.rng.Intn(len(industries))]
}

func (wg *WorkloadGenerator) generateRandomCategory() string {
	categories := []string{"Electronics", "Clothing", "Books", "Home", "Sports", "Beauty"}
	return categories[wg.rng.Intn(len(categories))]
}

func (wg *WorkloadGenerator) generateRandomRole() string {
	roles := []string{"Engineer", "Manager", "Analyst", "Designer", "Developer", "Consultant"}
	return roles[wg.rng.Intn(len(roles))]
}

// BenchmarkEngine wraps the storage engine for benchmarking.
type BenchmarkEngine struct {
	engine *storage.Engine
	config *BenchmarkConfig
	wg     *WorkloadGenerator
}

// NewBenchmarkEngine creates a new benchmark engine wrapper.
func NewBenchmarkEngine(config *BenchmarkConfig) (*BenchmarkEngine, error) {
	if config == nil {
		config = DefaultBenchmarkConfig()
	}

	// Configure engine for benchmarking
	engineConfig := config.EngineConfig
	if engineConfig == nil {
		engineConfig = storage.DefaultConfig()
	}

	// Override data directory for benchmarks
	engineConfig.DataDir = config.DataDir

	engine, err := storage.NewEngine(engineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	return &BenchmarkEngine{
		engine: engine,
		config: config,
		wg:     NewWorkloadGenerator(time.Now().UnixNano()),
	}, nil
}

// Start starts the benchmark engine.
func (be *BenchmarkEngine) Start() error {
	return be.engine.Start()
}

// Stop stops the benchmark engine.
func (be *BenchmarkEngine) Stop() error {
	return be.engine.Stop()
}

// GetEngine returns the underlying storage engine.
func (be *BenchmarkEngine) GetEngine() *storage.Engine {
	return be.engine
}

// GetWorkloadGenerator returns the workload generator.
func (be *BenchmarkEngine) GetWorkloadGenerator() *WorkloadGenerator {
	return be.wg
}
