# Benchmark Suite

benchmark suite

## Overview

This benchmark suite provides performance testing for various workloads including:
- **Write Performance**: High-throughput write operations with concurrent virtual users
- **Read Performance**: Various access patterns (random, sequential, hotspot, Zipfian)
- **Mixed Workloads**: Realistic read/write ratios with concurrent operations
- **Stress Testing**: High-concurrency scenarios with contention testing

## Features

- **Modular Design**: Reusable components across different benchmark types
- **Realistic Workloads**: Simulates real-world graph database usage patterns
- **Comprehensive Metrics**: Detailed latency distributions, throughput, and engine statistics
- **Multiple Output Formats**: JSON and CSV export for analysis
- **Configurable Scenarios**: Flexible configuration for different testing needs

## Quick Start

### Installation

```bash
cd crazygrapstore/benchmarks
go mod tidy
```

### Running Benchmarks

#### Run All Benchmarks (Default Configuration)

```bash
go run benchmarks/cmd/benchmark/main.go
```

#### Run Specific Benchmark Types

























































































```bash
# Write benchmarks only
go run benchmarks/cmd/benchmark/main.go -type write

# Read benchmarks with custom parameters
go run benchmarks/cmd/benchmark/main.go -type read -users 500 -ops 5000

# Mixed workload for 30 seconds
go run benchmarks/cmd/benchmark/main.go -type mixed -duration 30s

# Stress test with high concurrency
go run benchmarks/cmd/benchmark/main.go -type stress -users 2000 -ops 50000
```

#### Custom Configuration

```bash
go run benchmarks/cmd/benchmark/main.go \
  -users 1000 \
  -ops 10000 \
  -output ./my-results \
  -json \
  -csv \
  -cleanup=false
```

## Benchmark Types

### 1. Write Benchmarks

Tests pure write performance with:
- **Concurrent Writes**: Multiple virtual users writing simultaneously
- **Batch Processing**: Optional batching for higher throughput
- **Node/Edge Creation**: Realistic graph data generation
- **Write-Ahead Logging**: Tests durability performance

**Configuration:**
- Virtual Users: 100, 500, 1000 (default: 1000)
- Operations: 1K, 5K, 10K (default: 10K)
- Node/Edge Ratio: 70/30 (configurable)

### 2. Read Benchmarks

Tests read performance with different access patterns:

#### Access Patterns
- **Random**: Uniform random access across all data
- **Sequential**: Ordered access for cache-friendly patterns
- **Hotspot (80/20)**: 80% of reads target 20% of data
- **Zipfian**: Natural distribution found in real workloads

**Features:**
- Cache warmup for consistent measurements
- Node vs Edge read ratios
- Traversal operations for graph-specific workloads

### 3. Mixed Workload Benchmarks

Simulates real-world usage with configurable read/write ratios:

**Operation Types:**
- CreateNode, CreateEdge (writes)
- ReadNode, ReadEdge (reads)
- UpdateNode (writes)
- Traversal (reads)

**Configurable Ratios:**
- Write/Read: 10%, 30%, 50% writes
- Create/Update: 80/20 within writes
- Node/Edge: 70/30 overall
- Traversal/Point: 10/90

### 4. Stress Benchmarks

Tests system limits under high contention:

**Features:**
- High concurrency (up to 2000+ virtual users)
- Hotspot contention (90% on 10% of data)
- Lock contention testing
- Resource exhaustion scenarios

## Configuration

### Default Settings

```go
Virtual Users:    1000
Operations:       10,000
Write Lanes:     16
Memtable Size:   64MB
Cache Size:      4GB
Data Directory:  ./benchmark-data
```

### Custom Configuration

Create custom benchmark configurations:

```go
// Write benchmark
config := &benchmarks.BenchmarkConfig{
    NumVirtualUsers: 500,
    TotalOperations: 5000,
    EngineConfig:    storage.DefaultConfig(),
    DataDir:         "./custom-data",
}

// Mixed workload
config := &benchmarks.MixedWorkloadConfig{
    BenchmarkConfig: baseConfig,
    WriteRatio:     0.3,  // 30% writes
    CreateRatio:    0.8,  // 80% creates within writes
    NodeRatio:      0.7,  // 70% node operations
}
```

## Metrics and Results

### Collected Metrics

**Performance Metrics:**
- Throughput (operations/second)
- Latency percentiles (P50, P95, P99, P99.9)
- Min/Max latencies
- Success rate
- Error counts

**Engine Statistics:**
- Write lane utilization
- Cache hit rates (L0, L1, L2)
- Memory usage
- Active entries

### Output Formats

#### JSON Output
```json
{
  "name": "crazygraphstore-benchmark",
  "timestamp": "2024-01-01T12:00:00Z",
  "results": [
    {
      "name": "Write",
      "type": "write",
      "throughput": 50000.0,
      "p99_latency": "2ms",
      "success_rate": 99.9
    }
  ]
}
```

#### CSV Output
```csv
Name,Type,VirtualUsers,Operations,Duration(ms),Throughput(ops/s),P99(us),SuccessRate(%),CacheHitRate(%)
Write,write,1000,10000,200.00,50000.0,2000,99.9,95.2
```

## Performance Targets

Based on the project specifications, the benchmark suite targets:

| Operation | Target P99 Latency | Target Throughput |
|-----------|-------------------|------------------|
| Write (with WAL) | <2ms | 50k+ ops/sec |
| Read (cached) | <1ms | 500k+ ops/sec |
| Read (disk) | <3ms | 170k+ ops/sec |

## Architecture

### Modular Components

1. **Utils** (`utils.go`): Core benchmark infrastructure
   - Configuration management
   - Metrics collection
   - Workload generation
   - Engine wrapper

2. **Write Benchmarks** (`write_benchmark.go`): 
   - Single write operations
   - Batch write operations
   - Concurrent write testing

3. **Read Benchmarks** (`read_benchmark.go`):
   - Multiple access patterns
   - Cache performance testing
   - Traversal operations

4. **Mixed Benchmarks** (`mixed_benchmark.go`):
   - Realistic workload simulation
   - Configurable operation ratios
   - Concurrency stress testing

5. **Runner** (`runner.go`):
   - Suite orchestration
   - Result aggregation
   - Report generation

### Design Principles

- **No Lock Contention**: Shared-nothing architecture testing
- **Realistic Data**: Workload generator creates meaningful graph data
- **Comprehensive Coverage**: Tests all major engine components
- **Production Ready**: Suitable for continuous integration and regression testing

## Usage Examples

### Performance Regression Testing

```bash
# Run full suite for CI/CD
go run benchmarks/cmd/benchmark/main.go \
  -suite "regression-test" \
  -output ./ci-results \
  -cleanup=true

# Compare with baseline
diff baseline.json current.json
```

### Capacity Planning

```bash
# Test scaling characteristics
for users in 100 500 1000 2000; do
  go run benchmarks/cmd/benchmark/main.go \
    -type mixed \
    -users $users \
    -ops 50000 \
    -output "./scale-test-$users"
done
```

### Production Monitoring

```bash
# Quick health check
go run benchmarks/cmd/benchmark/main.go \
  -type read \
  -users 100 \
  -ops 1000 \
  -duration 30s
```

## Troubleshooting

### Common Issues

1. **Memory Errors**: Reduce virtual users or operations
2. **Disk Space**: Ensure sufficient space for data directory
3. **Permission Errors**: Check write permissions for output directory
4. **Port Conflicts**: Ensure no other processes using data directories

### Performance Tips

1. **Warmup**: Always allow cache warmup for consistent results
2. **Isolation**: Run benchmarks on dedicated hardware
3. **Multiple Runs**: Average results across multiple executions
4. **Monitoring**: Monitor system resources during benchmark runs

## Contributing

To add new benchmark types:

1. Create new benchmark file following existing patterns
2. Implement required interface methods
3. Add configuration options
4. Update runner to include new benchmark
5. Add documentation and examples

## License

This benchmark suite is part of the CrazyGraphStore project and follows the same license terms.
