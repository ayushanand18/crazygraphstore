// Package benchmarks provides the main entry point for running benchmarks.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ayushanand18/crazygraphstore/benchmarks"
)

func main() {
	// Command line flags
	var (
		suiteName           = flag.String("suite", "crazygraphstore-benchmark", "Benchmark suite name")
		runWriteBenchmarks  = flag.Bool("write", true, "Run write benchmarks")
		runReadBenchmarks   = flag.Bool("read", true, "Run read benchmarks")
		runMixedBenchmarks  = flag.Bool("mixed", true, "Run mixed workload benchmarks")
		runStressBenchmarks = flag.Bool("stress", true, "Run stress benchmarks")
		virtualUsers        = flag.Int("users", 1000, "Number of virtual users")
		operations          = flag.Int("ops", 10000, "Number of operations")
		outputDir           = flag.String("output", "./benchmark-results", "Output directory")
		enableJSON          = flag.Bool("json", true, "Enable JSON output")
		enableCSV           = flag.Bool("csv", true, "Enable CSV output")
		cleanup             = flag.Bool("cleanup", true, "Cleanup temporary data after run")
		benchmarkType       = flag.String("type", "all", "Benchmark type: write, read, mixed, stress, all")
		warmupOps           = flag.Int("warmup", 1000, "Number of warmup operations")
		_                   = warmupOps // TODO: implement warmup configuration
		duration            = flag.Duration("duration", 0, "Run for duration instead of operations (0 = use operations)")
	)
	flag.Parse()

	// Validate arguments
	if *virtualUsers <= 0 {
		log.Fatal("Number of virtual users must be positive")
	}
	if *operations <= 0 && *duration == 0 {
		log.Fatal("Either operations or duration must be specified")
	}

	fmt.Printf("=== CrazyGraphStore Benchmark Suite ===\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Suite Name: %s\n", *suiteName)
	fmt.Printf("  Virtual Users: %d\n", *virtualUsers)
	if *duration > 0 {
		fmt.Printf("  Duration: %v\n", *duration)
	} else {
		fmt.Printf("  Operations: %d\n", *operations)
	}
	fmt.Printf("  Output Directory: %s\n", *outputDir)
	fmt.Printf("  Benchmark Type: %s\n", *benchmarkType)
	fmt.Printf("\n")

	// Create runner configuration
	config := &benchmarks.RunnerConfig{
		SuiteName:           *suiteName,
		RunWriteBenchmarks:  *runWriteBenchmarks && (*benchmarkType == "all" || *benchmarkType == "write"),
		RunReadBenchmarks:   *runReadBenchmarks && (*benchmarkType == "all" || *benchmarkType == "read"),
		RunMixedBenchmarks:  *runMixedBenchmarks && (*benchmarkType == "all" || *benchmarkType == "mixed"),
		RunStressBenchmarks: *runStressBenchmarks && (*benchmarkType == "all" || *benchmarkType == "stress"),
		VirtualUsers:        []int{*virtualUsers},
		OperationCounts:     []int{*operations},
		Durations:           []time.Duration{*duration},
		OutputDir:           *outputDir,
		EnableJSONOutput:    *enableJSON,
		EnableCSVOutput:     *enableCSV,
		CleanupAfterRun:     *cleanup,
	}

	// Create and run benchmark runner
	runner := benchmarks.NewBenchmarkRunner(config)

	fmt.Printf("Starting benchmark suite...\n\n")
	if err := runner.RunAll(); err != nil {
		log.Fatalf("Benchmark suite failed: %v", err)
	}

	fmt.Printf("\nBenchmark suite completed successfully!\n")
	fmt.Printf("Results saved to: %s\n", *outputDir)
}

// runSingleBenchmark runs a single benchmark type with custom configuration.
func runSingleBenchmark(benchmarkType string, virtualUsers, operations int, outputDir string) {
	fmt.Printf("=== Running Single %s Benchmark ===\n", benchmarkType)

	var err error
	switch benchmarkType {
	case "write":
		config := benchmarks.DefaultBenchmarkConfig()
		config.NumVirtualUsers = virtualUsers
		config.TotalOperations = operations
		config.DataDir = outputDir + "/write-data"

		benchmark := benchmarks.NewWriteBenchmark(config)
		err = benchmark.Run()

	case "read":
		config := benchmarks.DefaultReadBenchmarkConfig()
		config.NumVirtualUsers = virtualUsers
		config.TotalOperations = operations
		config.DataDir = outputDir + "/read-data"

		benchmark := benchmarks.NewReadBenchmark(config)
		err = benchmark.Run()

	case "mixed":
		config := benchmarks.DefaultMixedWorkloadConfig()
		config.NumVirtualUsers = virtualUsers
		config.TotalOperations = operations
		config.DataDir = outputDir + "/mixed-data"

		benchmark := benchmarks.NewMixedBenchmark(config)
		err = benchmark.Run()

	case "stress":
		config := benchmarks.DefaultStressTestConfig()
		config.NumVirtualUsers = virtualUsers
		config.TotalOperations = operations
		config.DataDir = outputDir + "/stress-data"

		benchmark := benchmarks.NewConcurrencyStressBenchmark(config)
		err = benchmark.Run()

	default:
		log.Fatalf("Unknown benchmark type: %s", benchmarkType)
	}

	if err != nil {
		log.Fatalf("%s benchmark failed: %v", benchmarkType, err)
	}
}

// printUsage prints usage information.
func printUsage() {
	fmt.Printf("Usage: %s [options]\n", os.Args[0])
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  -suite string       Benchmark suite name (default \"crazygraphstore-benchmark\")\n")
	fmt.Printf("  -type string        Benchmark type: write, read, mixed, stress, all (default \"all\")\n")
	fmt.Printf("  -users int          Number of virtual users (default 1000)\n")
	fmt.Printf("  -ops int            Number of operations (default 10000)\n")
	fmt.Printf("  -duration duration  Run for duration instead of operations (e.g., 30s, 5m)\n")
	fmt.Printf("  -output string      Output directory (default \"./benchmark-results\")\n")
	fmt.Printf("  -warmup int         Number of warmup operations (default 1000)\n")
	fmt.Printf("  -write              Run write benchmarks (default true)\n")
	fmt.Printf("  -read               Run read benchmarks (default true)\n")
	fmt.Printf("  -mixed              Run mixed workload benchmarks (default true)\n")
	fmt.Printf("  -stress             Run stress benchmarks (default true)\n")
	fmt.Printf("  -json               Enable JSON output (default true)\n")
	fmt.Printf("  -csv                Enable CSV output (default true)\n")
	fmt.Printf("  -cleanup            Cleanup temporary data after run (default true)\n")
	fmt.Printf("\nExamples:\n")
	fmt.Printf("  # Run all benchmarks with default settings\n")
	fmt.Printf("  %s\n", os.Args[0])
	fmt.Printf("\n  # Run only write benchmarks with 500 users and 5000 operations\n")
	fmt.Printf("  %s -type write -users 500 -ops 5000\n", os.Args[0])
	fmt.Printf("\n  # Run mixed workload for 30 seconds\n")
	fmt.Printf("  %s -type mixed -duration 30s\n", os.Args[0])
	fmt.Printf("\n  # Run stress test with high concurrency\n")
	fmt.Printf("  %s -type stress -users 2000 -ops 50000\n", os.Args[0])
}
