// Package benchmarks provides a comprehensive benchmark runner for the graph database.
package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BenchmarkSuite represents a collection of benchmark results.
type BenchmarkSuite struct {
	Name        string                 `json:"name"`
	Timestamp   time.Time              `json:"timestamp"`
	Environment map[string]interface{} `json:"environment"`
	Results     []*BenchmarkResult     `json:"results"`
	Summary     *BenchmarkSuiteSummary `json:"summary"`
}

// BenchmarkResult represents the result of a single benchmark.
type BenchmarkResult struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Config       map[string]interface{} `json:"config"`
	Duration     time.Duration          `json:"duration"`
	Operations   int64                  `json:"operations"`
	Throughput   float64                `json:"throughput"`
	P50Latency   time.Duration          `json:"p50_latency"`
	P95Latency   time.Duration          `json:"p95_latency"`
	P99Latency   time.Duration          `json:"p99_latency"`
	P999Latency  time.Duration          `json:"p999_latency"`
	MinLatency   time.Duration          `json:"min_latency"`
	MaxLatency   time.Duration          `json:"max_latency"`
	SuccessRate  float64                `json:"success_rate"`
	ErrorCount   int64                  `json:"error_count"`
	CacheHitRate float64                `json:"cache_hit_rate"`
	EngineStats  map[string]interface{} `json:"engine_stats"`
}

// BenchmarkSuiteSummary provides aggregated statistics across all benchmarks.
type BenchmarkSuiteSummary struct {
	TotalDuration      time.Duration `json:"total_duration"`
	TotalOperations    int64         `json:"total_operations"`
	AverageThroughput  float64       `json:"average_throughput"`
	AverageP99Latency  time.Duration `json:"average_p99_latency"`
	OverallSuccessRate float64       `json:"overall_success_rate"`
	BestThroughput     float64       `json:"best_throughput"`
	BestLatency        time.Duration `json:"best_latency"`
}

// BenchmarkRunner orchestrates the execution of multiple benchmarks.
type BenchmarkRunner struct {
	config    *RunnerConfig
	suite     *BenchmarkSuite
	outputDir string
}

// RunnerConfig holds configuration for the benchmark runner.
type RunnerConfig struct {
	SuiteName           string          `json:"suite_name"`
	RunWriteBenchmarks  bool            `json:"run_write_benchmarks"`
	RunReadBenchmarks   bool            `json:"run_read_benchmarks"`
	RunMixedBenchmarks  bool            `json:"run_mixed_benchmarks"`
	RunStressBenchmarks bool            `json:"run_stress_benchmarks"`
	VirtualUsers        []int           `json:"virtual_users"`
	OperationCounts     []int           `json:"operation_counts"`
	Durations           []time.Duration `json:"durations"`
	OutputDir           string          `json:"output_dir"`
	EnableJSONOutput    bool            `json:"enable_json_output"`
	EnableCSVOutput     bool            `json:"enable_csv_output"`
	CleanupAfterRun     bool            `json:"cleanup_after_run"`
}

// DefaultRunnerConfig returns sensible defaults for the benchmark runner.
func DefaultRunnerConfig() *RunnerConfig {
	return &RunnerConfig{
		SuiteName:           "crazygraphstore-benchmark",
		RunWriteBenchmarks:  true,
		RunReadBenchmarks:   true,
		RunMixedBenchmarks:  true,
		RunStressBenchmarks: true,
		VirtualUsers:        []int{100, 500, 1000},
		OperationCounts:     []int{1000, 5000, 10000},
		Durations:           []time.Duration{}, // Use operation counts instead
		OutputDir:           "./benchmark-results",
		EnableJSONOutput:    true,
		EnableCSVOutput:     true,
		CleanupAfterRun:     true,
	}
}

// NewBenchmarkRunner creates a new benchmark runner.
func NewBenchmarkRunner(config *RunnerConfig) *BenchmarkRunner {
	if config == nil {
		config = DefaultRunnerConfig()
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create output directory %s: %v\n", config.OutputDir, err)
	}

	return &BenchmarkRunner{
		config: config,
		suite: &BenchmarkSuite{
			Name:        config.SuiteName,
			Timestamp:   time.Now(),
			Environment: getSystemInfo(),
			Results:     make([]*BenchmarkResult, 0),
		},
		outputDir: config.OutputDir,
	}
}

// RunAll executes all configured benchmarks.
func (br *BenchmarkRunner) RunAll() error {
	fmt.Printf("=== CrazyGraphStore Benchmark Suite ===\n")
	fmt.Printf("Suite: %s\n", br.suite.Name)
	fmt.Printf("Timestamp: %s\n", br.suite.Timestamp.Format(time.RFC3339))
	fmt.Printf("Output Directory: %s\n", br.outputDir)
	fmt.Printf("\n")

	// Run write benchmarks
	if br.config.RunWriteBenchmarks {
		if err := br.runWriteBenchmarks(); err != nil {
			return fmt.Errorf("write benchmarks failed: %w", err)
		}
	}

	// Run read benchmarks
	if br.config.RunReadBenchmarks {
		if err := br.runReadBenchmarks(); err != nil {
			return fmt.Errorf("read benchmarks failed: %w", err)
		}
	}

	// Run mixed benchmarks
	if br.config.RunMixedBenchmarks {
		if err := br.runMixedBenchmarks(); err != nil {
			return fmt.Errorf("mixed benchmarks failed: %w", err)
		}
	}

	// Run stress benchmarks
	if br.config.RunStressBenchmarks {
		if err := br.runStressBenchmarks(); err != nil {
			return fmt.Errorf("stress benchmarks failed: %w", err)
		}
	}

	// Generate summary
	br.generateSummary()

	// Save results
	if err := br.saveResults(); err != nil {
		return fmt.Errorf("failed to save results: %w", err)
	}

	// Print summary
	br.printSummary()

	// Cleanup if requested
	if br.config.CleanupAfterRun {
		br.cleanup()
	}

	return nil
}

// runWriteBenchmarks executes write performance tests.
func (br *BenchmarkRunner) runWriteBenchmarks() error {
	fmt.Printf("=== Running Write Benchmarks ===\n")

	for _, users := range br.config.VirtualUsers {
		for _, ops := range br.config.OperationCounts {
			fmt.Printf("\nWrite Benchmark: %d users, %d operations\n", users, ops)

			// Configure benchmark
			config := DefaultBenchmarkConfig()
			config.NumVirtualUsers = users
			config.TotalOperations = ops
			config.DataDir = filepath.Join(br.outputDir, fmt.Sprintf("write-data-%d-%d", users, ops))

			// Run benchmark
			benchmark := NewWriteBenchmark(config)
			if err := benchmark.Run(); err != nil {
				fmt.Printf("Write benchmark failed: %v\n", err)
				continue
			}

			// Record result
			result := br.createBenchmarkResult("Write", "write", config, benchmark.metrics)
			br.suite.Results = append(br.suite.Results, result)
		}
	}

	return nil
}

// runReadBenchmarks executes read performance tests.
func (br *BenchmarkRunner) runReadBenchmarks() error {
	fmt.Printf("=== Running Read Benchmarks ===\n")

	accessPatterns := []AccessPattern{
		AccessPatternRandom,
		AccessPatternSequential,
		AccessPatternHotspot,
		AccessPatternZipfian,
	}

	for _, users := range br.config.VirtualUsers {
		for _, ops := range br.config.OperationCounts {
			for _, pattern := range accessPatterns {
				fmt.Printf("\nRead Benchmark: %d users, %d operations, %s pattern\n",
					users, ops, pattern.String())

				// Configure benchmark
				config := DefaultReadBenchmarkConfig()
				config.NumVirtualUsers = users
				config.TotalOperations = ops
				config.AccessPattern = pattern
				config.DataDir = filepath.Join(br.outputDir, fmt.Sprintf("read-data-%d-%d-%s", users, ops, pattern.String()))

				// Run benchmark
				benchmark := NewReadBenchmark(config)
				if err := benchmark.Run(); err != nil {
					fmt.Printf("Read benchmark failed: %v\n", err)
					continue
				}

				// Record result
				result := br.createBenchmarkResult(fmt.Sprintf("Read-%s", pattern.String()), "read", config.BenchmarkConfig, benchmark.metrics)
				br.suite.Results = append(br.suite.Results, result)
			}
		}
	}

	return nil
}

// runMixedBenchmarks executes mixed workload tests.
func (br *BenchmarkRunner) runMixedBenchmarks() error {
	fmt.Printf("=== Running Mixed Workload Benchmarks ===\n")

	writeRatios := []float64{0.1, 0.3, 0.5}

	for _, users := range br.config.VirtualUsers {
		for _, ops := range br.config.OperationCounts {
			for _, writeRatio := range writeRatios {
				fmt.Printf("\nMixed Benchmark: %d users, %d operations, %.1f%% writes\n",
					users, ops, writeRatio*100)

				// Configure benchmark
				config := DefaultMixedWorkloadConfig()
				config.NumVirtualUsers = users
				config.TotalOperations = ops
				config.WriteRatio = writeRatio
				config.DataDir = filepath.Join(br.outputDir, fmt.Sprintf("mixed-data-%d-%d-%.1f", users, ops, writeRatio))

				// Run benchmark
				benchmark := NewMixedBenchmark(config)
				if err := benchmark.Run(); err != nil {
					fmt.Printf("Mixed benchmark failed: %v\n", err)
					continue
				}

				// Record result
				result := br.createBenchmarkResult(fmt.Sprintf("Mixed-%.0f", writeRatio*100), "mixed", config.BenchmarkConfig, benchmark.metrics)
				br.suite.Results = append(br.suite.Results, result)
			}
		}
	}

	return nil
}

// runStressBenchmarks executes stress tests.
func (br *BenchmarkRunner) runStressBenchmarks() error {
	fmt.Printf("=== Running Stress Benchmarks ===\n")

	contentionLevels := []float64{0.5, 0.8, 1.0}

	for _, users := range br.config.VirtualUsers {
		for _, ops := range br.config.OperationCounts {
			for _, contention := range contentionLevels {
				fmt.Printf("\nStress Benchmark: %d users, %d operations, %.1f contention\n",
					users, ops, contention)

				// Configure benchmark
				config := DefaultStressTestConfig()
				config.NumVirtualUsers = users
				config.TotalOperations = ops
				config.ContentionLevel = contention
				config.DataDir = filepath.Join(br.outputDir, fmt.Sprintf("stress-data-%d-%d-%.1f", users, ops, contention))

				// Run benchmark
				benchmark := NewConcurrencyStressBenchmark(config)
				if err := benchmark.Run(); err != nil {
					fmt.Printf("Stress benchmark failed: %v\n", err)
					continue
				}

				// Record result
				result := br.createBenchmarkResult(fmt.Sprintf("Stress-%.0f", contention*100), "stress", config.BenchmarkConfig, benchmark.metrics)
				br.suite.Results = append(br.suite.Results, result)
			}
		}
	}

	return nil
}

// createBenchmarkResult converts metrics to a benchmark result.
func (br *BenchmarkRunner) createBenchmarkResult(name, benchmarkType string, config *BenchmarkConfig, metrics *Metrics) *BenchmarkResult {
	result := &BenchmarkResult{
		Name: name,
		Type: benchmarkType,
		Config: map[string]interface{}{
			"virtual_users":    config.NumVirtualUsers,
			"total_operations": config.TotalOperations,
			"data_dir":         config.DataDir,
		},
		Duration:    metrics.TotalDuration,
		Operations:  metrics.TotalOps,
		Throughput:  metrics.OperationsPerSec,
		P50Latency:  metrics.P50Latency,
		P95Latency:  metrics.P95Latency,
		P99Latency:  metrics.P99Latency,
		P999Latency: metrics.P999Latency,
		MinLatency:  metrics.MinLatency,
		MaxLatency:  metrics.MaxLatency,
		SuccessRate: float64(metrics.SuccessfulOps) / float64(metrics.TotalOps) * 100,
		ErrorCount:  metrics.FailedOps,
		EngineStats: map[string]interface{}{
			"write_lanes":        metrics.EngineStats.NumLanes,
			"total_active_count": metrics.EngineStats.TotalActiveCount,
			"total_active_size":  metrics.EngineStats.TotalActiveSize,
			"cache_hit_rate":     metrics.EngineStats.CacheStats.OverallHitRate * 100,
		},
	}

	return result
}

// generateSummary creates aggregated summary statistics.
func (br *BenchmarkRunner) generateSummary() {
	if len(br.suite.Results) == 0 {
		return
	}

	var totalDuration time.Duration
	var totalOps int64
	var totalSuccessful int64
	var totalOpsForThroughput float64
	var p99Latencies []time.Duration
	var throughputs []float64

	for _, result := range br.suite.Results {
		totalDuration += result.Duration
		totalOps += result.Operations
		totalSuccessful += int64(result.SuccessRate * float64(result.Operations) / 100)
		totalOpsForThroughput += result.Throughput
		p99Latencies = append(p99Latencies, result.P99Latency)
		throughputs = append(throughputs, result.Throughput)
	}

	// Calculate averages
	avgThroughput := totalOpsForThroughput / float64(len(br.suite.Results))
	avgP99Latency := p99Latencies[0]
	if len(p99Latencies) > 1 {
		var totalP99 time.Duration
		for _, lat := range p99Latencies {
			totalP99 += lat
		}
		avgP99Latency = totalP99 / time.Duration(len(p99Latencies))
	}

	// Find best throughput and latency
	bestThroughput := throughputs[0]
	bestLatency := p99Latencies[0]
	for _, throughput := range throughputs {
		if throughput > bestThroughput {
			bestThroughput = throughput
		}
	}
	for _, latency := range p99Latencies {
		if latency < bestLatency {
			bestLatency = latency
		}
	}

	br.suite.Summary = &BenchmarkSuiteSummary{
		TotalDuration:      totalDuration,
		TotalOperations:    totalOps,
		AverageThroughput:  avgThroughput,
		AverageP99Latency:  avgP99Latency,
		OverallSuccessRate: float64(totalSuccessful) / float64(totalOps) * 100,
		BestThroughput:     bestThroughput,
		BestLatency:        bestLatency,
	}
}

// saveResults saves benchmark results to files.
func (br *BenchmarkRunner) saveResults() error {
	// Save JSON output
	if br.config.EnableJSONOutput {
		jsonFile := filepath.Join(br.outputDir, fmt.Sprintf("%s.json", br.suite.Name))
		data, err := json.MarshalIndent(br.suite, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		if err := os.WriteFile(jsonFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write JSON file: %w", err)
		}
		fmt.Printf("Results saved to: %s\n", jsonFile)
	}

	// Save CSV output
	if br.config.EnableCSVOutput {
		csvFile := filepath.Join(br.outputDir, fmt.Sprintf("%s.csv", br.suite.Name))
		if err := br.saveCSV(csvFile); err != nil {
			return fmt.Errorf("failed to save CSV: %w", err)
		}
		fmt.Printf("CSV saved to: %s\n", csvFile)
	}

	return nil
}

// saveCSV saves results in CSV format.
func (br *BenchmarkRunner) saveCSV(filename string) error {
	// Simple CSV implementation
	csvContent := "Name,Type,VirtualUsers,Operations,Duration(ms),Throughput(ops/s),P99(us),SuccessRate(%),CacheHitRate(%)\n"

	for _, result := range br.suite.Results {
		virtualUsers := result.Config["virtual_users"].(int)
		csvContent += fmt.Sprintf("%s,%s,%d,%d,%.2f,%.2f,%d,%.2f,%.2f\n",
			result.Name,
			result.Type,
			virtualUsers,
			result.Operations,
			float64(result.Duration.Nanoseconds())/1000000.0,
			result.Throughput,
			result.P99Latency.Microseconds(),
			result.SuccessRate,
			result.EngineStats["cache_hit_rate"].(float64),
		)
	}

	return os.WriteFile(filename, []byte(csvContent), 0644)
}

// printSummary prints the benchmark suite summary.
func (br *BenchmarkRunner) printSummary() {
	fmt.Printf("\n=== Benchmark Suite Summary ===\n")
	if br.suite.Summary == nil {
		fmt.Printf("No results to summarize\n")
		return
	}

	summary := br.suite.Summary
	fmt.Printf("Total Duration: %v\n", summary.TotalDuration)
	fmt.Printf("Total Operations: %d\n", summary.TotalOperations)
	fmt.Printf("Average Throughput: %.2f ops/sec\n", summary.AverageThroughput)
	fmt.Printf("Average P99 Latency: %v\n", summary.AverageP99Latency)
	fmt.Printf("Overall Success Rate: %.2f%%\n", summary.OverallSuccessRate)
	fmt.Printf("Best Throughput: %.2f ops/sec\n", summary.BestThroughput)
	fmt.Printf("Best P99 Latency: %v\n", summary.BestLatency)

	fmt.Printf("\nBenchmark Results (%d total):\n", len(br.suite.Results))
	for _, result := range br.suite.Results {
		fmt.Printf("  %s: %.2f ops/sec, %v P99, %.1f%% success\n",
			result.Name, result.Throughput, result.P99Latency, result.SuccessRate)
	}
}

// cleanup removes temporary data directories.
func (br *BenchmarkRunner) cleanup() {
	fmt.Printf("\nCleaning up temporary data...\n")

	// Remove data directories
	entries, err := os.ReadDir(br.outputDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[:4] == "data" {
			dirPath := filepath.Join(br.outputDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				fmt.Printf("Failed to remove %s: %v\n", dirPath, err)
			}
		}
	}
}

// getSystemInfo collects basic system information for the benchmark report.
func getSystemInfo() map[string]interface{} {
	// Basic system info - could be expanded with more detailed metrics
	return map[string]interface{}{
		"go_version": "go1.21", // Should be detected dynamically
		"os":         "linux",
		"arch":       "amd64",
		"cpu_count":  8, // Should be detected dynamically
	}
}
