
# Storage Engine Architecture

## Overview

A high-performance storage engine for graph databases using LSM-tree architecture with shared-nothing write lanes and multi-level caching.

## Core Architecture

### Write Path

```
Request → WAL Append → Memtable Write → Size Check → Rotation?
   ↓         ↓             ↓               ↓            ↓
  <1ms      <1ms          <1ms           <1ms         <2ms
 (sync)    (sync)        (sync)         (sync)      (sync)

                    ↓ (async background)
         
Background Flush → SSTable → Compaction → Manifest Update
      ↓              ↓           ↓             ↓
    ~100ms       On Disk      1-5s        Atomic
   (async)      (durable)    (async)     (durable)
```

### Read Path

```
Request → L0 Cache → L1 Cache → L2 Cache → Active Memtable
   ↓         ↓          ↓          ↓             ↓
           <1ms       <2ms       <2ms          <3ms
           (90%)      (5%)       (3%)          (1%)
           
                    ↓ (cache miss)
                    
           Old Memtables → SSTable + Bloom → Not Found
                ↓              ↓               ↓
               <4ms           <5ms          Error
               (0.5%)         (0.5%)
```

### Recovery Path

```
Startup → Load Manifest → Verify SSTables → Replay WAL → Rebuild → Ready
   ↓          ↓                ↓               ↓            ↓         ↓
 Start    Read JSON      Check files      Per lane     Fill mem   Serve
```

## Component Details

### 1. Shared-Nothing Write Lanes

**Design**: 16 parallel write lanes, each with dedicated memtable and WAL

**Benefits**:
- Zero lock contention on writes
- Linear scalability with CPU cores
- Each thread writes at full speed

**Implementation**:
```go
type WriteLane struct {
    id      int
    wal     *WAL              // Dedicated WAL
    active  *memtable.NodeMemtable  // No locks!
    old     []*memtable.NodeMemtable // Locked only during rotation
}
```

**Key Feature**: Consistent hashing ensures same node always goes to same lane

### 2. Multi-Level Cache

**Design**: 3-tier cache architecture with different sizes

**Tiers**:
- **L0 (Hot)**: 40% - Most frequently accessed nodes
- **L1 (Connection)**: 30% - Precomputed adjacency lists
- **L2 (Data)**: 30% - Large data blobs

**Invalidation**: Version-based + write-through

**Target**: 90%+ overall cache hit rate

### 3. Write-Ahead Log (WAL)

**Design**: Per-lane WAL with group commit

**Entry Format**:
```
[Type][KeyLen][Key][ValueLen][Value][Timestamp][CRC32]
```

**Durability Levels**:
- NoSync: Fastest, relies on OS cache
- GroupCommit: Batch fsync every 10ms (default)
- SyncEveryWrite: Fsync per write (slowest)

**Recovery**: Replay all entries from last checkpoint

### 4. SSTable Format

**File Structure**:
```
┌──────────────────┐
│ Data Block 1     │ 64KB
├──────────────────┤
│ Data Block 2     │ 64KB
├──────────────────┤
│ ...              │
├──────────────────┤
│ Index Block      │ [first_key, offset] per block
├──────────────────┤
│ Bloom Filter     │ 1% false positive rate
├──────────────────┤
│ Footer           │ 48 bytes (offsets + magic)
└──────────────────┘
```

**Optimizations**:
- Memory-mapped reads (zero-copy)
- Bloom filters reduce disk I/O by 90%
- Block cache for frequently accessed blocks

### 5. Compaction

**Strategy**: Size-tiered compaction

**Trigger**: More than 4 tables at level 0

**Process**:
1. Select tables of similar size
2. K-way merge (heap-based)
3. Write new merged table
4. Update manifest
5. Delete old tables

**Target**: <2x space amplification

### 6. Graph Data Model

**Node Storage**:
```go
type Node struct {
    ID         string                 // Unique identifier
    Labels     []string               // ["Person", "Employee"]
    Properties map[string]interface{} // Arbitrary key-value
    Version    uint64                 // For optimistic locking
}
```

**Edge Storage**:
```go
type Edge struct {
    ID         string                 // Unique edge ID
    FromNodeID string                 // Source node
    ToNodeID   string                 // Target node
    Type       string                 // "KNOWS", "WORKS_AT"
    Properties map[string]interface{} // Edge properties
}
```

**Adjacency Lists**: Maintained for O(1) neighbor lookups

## Performance Characteristics

### Write Performance

- **Latency**: <2ms P99 (with WAL)
- **Throughput**: 50k+ ops/sec (single node)
- **Bottleneck**: WAL fsync (mitigated by group commit)

### Read Performance

- **Latency**: <1ms P99 (cached), <5ms P99 (disk)
- **Throughput**: 500k+ ops/sec (cached)
- **Bottleneck**: Cache misses (target 90% hit rate)

### Space Efficiency

- **Amplification**: <2x with regular compaction
- **Bloom Filter**: 1% FPR, ~117KB per 100k keys
- **Index Overhead**: ~16 bytes per block

## Scalability

### Vertical Scaling

- Write lanes scale with CPU cores
- Cache scales with available RAM
- Disk I/O scales with SSD performance

### Horizontal Scaling (Future)

- Consistent hashing for data partitioning
- Replication factor: 3 (configurable)
- Quorum: R=2, W=2

## Configuration

### Default Configuration

```yaml
storage:
  num_write_lanes: 16           # Parallel write lanes
  memtable_size: 67108864       # 64MB per lane
  cache_size: 4294967296        # 4GB total
  wal_durability: group_commit  # Balance speed/durability
  wal_group_commit_interval: 10ms
  compaction_trigger: 4         # Compact at 4+ tables
  bloom_fpr: 0.01              # 1% false positive rate
```

### Tuning for Workloads

**Write-Heavy**:
```yaml
num_write_lanes: 32           # More parallelism
memtable_size: 134217728      # 128MB (less rotation)
cache_size: 8589934592        # 8GB
```

**Read-Heavy**:
```yaml
num_write_lanes: 16           # Standard
memtable_size: 67108864       # 64MB
cache_size: 17179869184       # 16GB (aggressive caching)
bloom_fpr: 0.001             # 0.1% FPR (less disk I/O)
```

## Design Decisions

### Why Shared-Nothing?

- Eliminates lock contention
- Linear scalability
- Simpler reasoning about concurrency

### Why Multi-Level Cache?

- Different access patterns need different caching strategies
- Hot data vs connection data vs large blobs
- Flexible tier sizing based on workload

### Why Size-Tiered Compaction?

- Simpler than leveled compaction
- Good enough for most workloads
- Lower write amplification

### Why Per-Lane WAL?

- Parallel disk writes
- No serialization bottleneck
- Independent recovery

## Future Enhancements

1. **Leveled Compaction**: Better space efficiency
2. **Bloom Filter Tuning**: Adaptive FPR based on hit rate
3. **Compression**: Snappy or LZ4 for SSTables
4. **Block Cache**: Separate cache for SSTable blocks
5. **Cluster Mode**: Distributed storage with replication

---

*See [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for development roadmap*
