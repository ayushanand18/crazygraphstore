
# Storage Engine for Graph Database

> A high-performance, LSM-tree based storage engine optimized for graph database workloads, implemented in Go.

## Features

- **Shared-Nothing Architecture**: 16 parallel write lanes with zero lock contention
- **Multi-Level Caching**: 90%+ cache hit rate with 3-tier LRU cache
- **Write-Ahead Logging**: Crash recovery with zero data loss
- **SSTable Persistence**: Memory-mapped reads with bloom filters
- **Background Compaction**: Size-tiered strategy for space efficiency
- **Graph Operations**: First-class support for nodes, edges, and traversals

## Performance

| Operation | Latency (P99) | Throughput |
|-----------|---------------|------------|
| Write (with WAL) | <2ms | 50k+ ops/sec |
| Read (cached) | <1ms | 500k+ ops/sec |
| Read (disk) | <3ms | 170k+ ops/sec |

## Quick Start

```bash
# Install dependencies
go mod tidy

# Run example
go run cmd/example/simple/main.go

# Or with persistence
go run cmd/example/persistencemain.go
```

## Documentation
- [Docs are available here](docs/)

## Architecture Overview

```
Write Path: WAL → Memtable → Rotation → Background Flush → SSTable
Read Path:  L0 Cache → Memtable → SSTable → Not Found
Recovery:   Load Manifest → Verify SSTables → Replay WAL → Ready
```

## Basic Usage

```go
import (
    "github.com/ayushanand18/crazygraphstore/pkg/storage"
    "github.com/ayushanand18/crazygraphstore/pkg/graph"
)

// Create engine
config := storage.DefaultConfig()
engine, _ := storage.NewEngine(config)
engine.Start()
defer engine.Stop()

// Create node
node := graph.NewNode("user-1", []string{"Person"})
node.SetProperty("name", "Alice")
engine.CreateNode(ctx, node)

// Create edge
edge := graph.NewEdge("e1", "user-1", "user-2", "KNOWS")
engine.CreateEdge(ctx, edge)

// Traverse
neighbors, _ := engine.GetNeighbors(ctx, "user-1", graph.DirectionOut)
```

## Project Structure

```
pkg/
├── graph/          # Domain models (Node, Edge, Path)
├── memtable/       # In-memory storage
├── storage/        # Core engine
├── wal/            # Write-Ahead Log
├── sstable/        # Disk persistence
├── cache/          # Multi-level caching
├── compaction/     # Background compaction
├── persistence/    # Flushing manager
└── manifest/       # SSTable tracking

cmd/
├── example/        # Basic example
└── example-phase2/ # Full-featured example
```

## Key Concepts

- **LSM Trees**: Log-Structured Merge Trees for write-optimized storage
- **SSTables**: Sorted String Tables for efficient disk storage
- **WAL**: Write-Ahead Logging for durability and recovery
- **Compaction**: Background merging to reduce read amplification
- **Bloom Filters**: Probabilistic data structure to avoid unnecessary disk I/O

## Implementation Status

- **Phase 1**: In-memory storage with shared-nothing write lanes
- **Phase 2**: Disk persistence with SSTables and multi-level caching
- **Phase 3**: WAL for durability and compaction for space efficiency
- **Phase 4**: Cluster mode (optional)
- **Phase 5**: Production monitoring (optional)

## Testing

```bash
# Run tests
go test ./... -v

# Run with race detector
go test ./... -v -race

# Run benchmarks
go test ./... -bench=. -benchmem
```

## Contributing

No longer just an academic project. Contributions for:
- Unit tests and benchmarks
- Documentation improvements
- Bug fixes and optimizations
are welcome!

## License

MIT License - See LICENSE file for details

## Acknowledgments

- Inspired by RocksDB, FoundationDB, and Cassandra
- Based on LSM-tree research by O'Neil et al.
- Graph concepts from Neo4j and JanusGraph

---

