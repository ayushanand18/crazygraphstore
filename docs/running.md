
# Running the Storage Engine

## Quick Start

```bash
# Run Phase 1 example (in-memory)
go run cmd/example/main.go

# Run Phase 2 example (with persistence and caching)
go run cmd/example-phase2/main.go
```

## With Make

```bash
# Build
make build

# Run example
make example

# Run Phase 2 example
make -f Makefile.phase2 example-phase2

# Run tests
make test

# Run benchmarks
make bench

# Clean build artifacts
make clean
```

## Configuration

Create a `config.yaml` file:

```yaml
storage:
  num_write_lanes: 16
  memtable_size: 67108864  # 64MB
  cache_size: 4294967296   # 4GB
  data_dir: ./data
  wal_dir: ./data/wal
  enable_wal: true
```

## Development

```bash
# Install dependencies
go mod tidy

# Run with race detector
go run -race cmd/example/main.go

# Profile CPU
go run cmd/example/main.go -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Profile memory
go run cmd/example/main.go -memprofile=mem.prof
go tool pprof mem.prof
```

## Production Deployment

```bash
# Build optimized binary
go build -ldflags="-s -w" -o storage-engine cmd/example/main.go

# Run with logging
./storage-engine --config production.yaml --log-level info

# With monitoring
./storage-engine --config production.yaml --metrics-port 9090
```
