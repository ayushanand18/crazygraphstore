
// Package cache provides multi-level caching implementation.
package cache

import (
	"sync"
	"sync/atomic"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

// CacheTier represents the different cache tiers.
type CacheTier int

const (
	TierHot        CacheTier = iota // L0: Hot cache for frequently accessed nodes
	TierConnection                   // L1: Connection cache for adjacency lists
	TierData                         // L2: Data cache for large node data
)

// MultiLevelCache implements a 3-tier cache hierarchy.
type MultiLevelCache struct {
	hot        *LRUCache       // L0: 40% - Most frequently accessed nodes
	connection *LRUCache       // L1: 30% - Precomputed connection lists
	data       *LRUCache       // L2: 30% - Large data blobs
	versions   sync.Map        // Version tracking for cache invalidation
	totalSize  int64           // Total cache size
	metrics    *CacheMetrics
}

// CacheMetrics tracks cache performance.
type CacheMetrics struct {
	l0Hits    atomic.Uint64
	l0Misses  atomic.Uint64
	l1Hits    atomic.Uint64
	l1Misses  atomic.Uint64
	l2Hits    atomic.Uint64
	l2Misses  atomic.Uint64
	totalHits atomic.Uint64
	totalMisses atomic.Uint64
	invalidations atomic.Uint64
}

// NewMultiLevelCache creates a new multi-level cache.
// totalSize is divided: 40% hot, 30% connection, 30% data
func NewMultiLevelCache(totalSize int64) *MultiLevelCache {
	hotSize := totalSize * 40 / 100
	connSize := totalSize * 30 / 100
	dataSize := totalSize * 30 / 100

	return &MultiLevelCache{
		hot:        NewLRUCache(hotSize),
		connection: NewLRUCache(connSize),
		data:       NewLRUCache(dataSize),
		totalSize:  totalSize,
		metrics:    &CacheMetrics{},
	}
}

// GetNode retrieves a node from the hot cache.
func (mlc *MultiLevelCache) GetNode(nodeID string) (*graph.Node, bool) {
	value, ok := mlc.hot.Get(nodeID)
	if ok {
		mlc.metrics.l0Hits.Add(1)
		mlc.metrics.totalHits.Add(1)
		return value.(*graph.Node), true
	}

	mlc.metrics.l0Misses.Add(1)
	mlc.metrics.totalMisses.Add(1)
	return nil, false
}

// PutNode stores a node in the hot cache.
func (mlc *MultiLevelCache) PutNode(node *graph.Node) error {
	// Estimate node size
	size := mlc.estimateNodeSize(node)
	return mlc.hot.Put(node.ID, node, size)
}

// GetEdge retrieves an edge from the data cache.
func (mlc *MultiLevelCache) GetEdge(edgeID string) (*graph.Edge, bool) {
	value, ok := mlc.data.Get(edgeID)
	if ok {
		mlc.metrics.l2Hits.Add(1)
		mlc.metrics.totalHits.Add(1)
		return value.(*graph.Edge), true
	}

	mlc.metrics.l2Misses.Add(1)
	mlc.metrics.totalMisses.Add(1)
	return nil, false
}

// PutEdge stores an edge in the data cache.
func (mlc *MultiLevelCache) PutEdge(edge *graph.Edge) error {
	// Estimate edge size
	size := mlc.estimateEdgeSize(edge)
	return mlc.data.Put(edge.ID, edge, size)
}

// GetConnectionList retrieves a cached connection list.
func (mlc *MultiLevelCache) GetConnectionList(nodeID string, direction graph.Direction) ([]string, bool) {
	key := mlc.connectionKey(nodeID, direction)
	value, ok := mlc.connection.Get(key)
	if ok {
		mlc.metrics.l1Hits.Add(1)
		mlc.metrics.totalHits.Add(1)
		return value.([]string), true
	}

	mlc.metrics.l1Misses.Add(1)
	mlc.metrics.totalMisses.Add(1)
	return nil, false
}

// PutConnectionList stores a connection list in the connection cache.
func (mlc *MultiLevelCache) PutConnectionList(nodeID string, direction graph.Direction, edgeIDs []string) error {
	key := mlc.connectionKey(nodeID, direction)
	// Estimate size: ~8 bytes per string reference + overhead
	size := int64(len(edgeIDs) * 8)
	return mlc.connection.Put(key, edgeIDs, size)
}

// InvalidateNode removes a node from all cache tiers.
func (mlc *MultiLevelCache) InvalidateNode(nodeID string) {
	mlc.hot.Invalidate(nodeID)
	mlc.data.Invalidate(nodeID)

	// Invalidate connection caches for this node
	mlc.connection.Invalidate(mlc.connectionKey(nodeID, graph.DirectionOut))
	mlc.connection.Invalidate(mlc.connectionKey(nodeID, graph.DirectionIn))
	mlc.connection.Invalidate(mlc.connectionKey(nodeID, graph.DirectionBoth))

	// Increment version
	v, _ := mlc.versions.LoadOrStore(nodeID, &atomic.Uint64{})
	v.(*atomic.Uint64).Add(1)

	mlc.metrics.invalidations.Add(1)
}

// InvalidateEdge removes an edge from the cache.
func (mlc *MultiLevelCache) InvalidateEdge(edgeID string) {
	mlc.data.Invalidate(edgeID)
	mlc.metrics.invalidations.Add(1)
}

// Clear removes all entries from all cache tiers.
func (mlc *MultiLevelCache) Clear() {
	mlc.hot.Clear()
	mlc.connection.Clear()
	mlc.data.Clear()
}

// Stats returns comprehensive cache statistics.
type MultiLevelCacheStats struct {
	TotalSize       int64
	HotStats        CacheStats
	ConnectionStats CacheStats
	DataStats       CacheStats
	L0HitRate       float64
	L1HitRate       float64
	L2HitRate       float64
	OverallHitRate  float64
	Invalidations   uint64
}

// Stats returns current cache statistics.
func (mlc *MultiLevelCache) Stats() MultiLevelCacheStats {
	l0Hits := mlc.metrics.l0Hits.Load()
	l0Misses := mlc.metrics.l0Misses.Load()
	l1Hits := mlc.metrics.l1Hits.Load()
	l1Misses := mlc.metrics.l1Misses.Load()
	l2Hits := mlc.metrics.l2Hits.Load()
	l2Misses := mlc.metrics.l2Misses.Load()
	totalHits := mlc.metrics.totalHits.Load()
	totalMisses := mlc.metrics.totalMisses.Load()

	l0Total := l0Hits + l0Misses
	l1Total := l1Hits + l1Misses
	l2Total := l2Hits + l2Misses
	overallTotal := totalHits + totalMisses

	l0HitRate := 0.0
	if l0Total > 0 {
		l0HitRate = float64(l0Hits) / float64(l0Total)
	}

	l1HitRate := 0.0
	if l1Total > 0 {
		l1HitRate = float64(l1Hits) / float64(l1Total)
	}

	l2HitRate := 0.0
	if l2Total > 0 {
		l2HitRate = float64(l2Hits) / float64(l2Total)
	}

	overallHitRate := 0.0
	if overallTotal > 0 {
		overallHitRate = float64(totalHits) / float64(overallTotal)
	}

	return MultiLevelCacheStats{
		TotalSize:       mlc.totalSize,
		HotStats:        mlc.hot.Stats(),
		ConnectionStats: mlc.connection.Stats(),
		DataStats:       mlc.data.Stats(),
		L0HitRate:       l0HitRate,
		L1HitRate:       l1HitRate,
		L2HitRate:       l2HitRate,
		OverallHitRate:  overallHitRate,
		Invalidations:   mlc.metrics.invalidations.Load(),
	}
}

// connectionKey generates a cache key for connection lists.
func (mlc *MultiLevelCache) connectionKey(nodeID string, direction graph.Direction) string {
	return nodeID + ":" + direction.String()
}

// estimateNodeSize estimates the memory size of a node.
func (mlc *MultiLevelCache) estimateNodeSize(node *graph.Node) int64 {
	size := int64(len(node.ID))
	size += int64(len(node.Labels) * 16) // Estimate for label strings

	// Estimate properties size
	for key, value := range node.Properties {
		size += int64(len(key))
		switch v := value.(type) {
		case string:
			size += int64(len(v))
		case []byte:
			size += int64(len(v))
		case int64, float64, bool:
			size += 8
		default:
			size += 16 // Generic estimate
		}
	}

	// Add overhead for struct fields
	size += 64

	return size
}

// estimateEdgeSize estimates the memory size of an edge.
func (mlc *MultiLevelCache) estimateEdgeSize(edge *graph.Edge) int64 {
	size := int64(len(edge.ID))
	size += int64(len(edge.FromNodeID))
	size += int64(len(edge.ToNodeID))
	size += int64(len(edge.Type))

	// Estimate properties size
	for key, value := range edge.Properties {
		size += int64(len(key))
		switch v := value.(type) {
		case string:
			size += int64(len(v))
		case []byte:
			size += int64(len(v))
		case int64, float64, bool:
			size += 8
		default:
			size += 16 // Generic estimate
		}
	}

	// Add overhead for struct fields
	size += 64

	return size
}

// Resize adjusts the total cache size and redistributes among tiers.
func (mlc *MultiLevelCache) Resize(newTotalSize int64) {
	mlc.totalSize = newTotalSize

	hotSize := newTotalSize * 40 / 100
	connSize := newTotalSize * 30 / 100
	dataSize := newTotalSize * 30 / 100

	mlc.hot.Resize(hotSize)
	mlc.connection.Resize(connSize)
	mlc.data.Resize(dataSize)
}

// WarmUp preloads frequently accessed keys into the cache.
func (mlc *MultiLevelCache) WarmUp(nodes []*graph.Node) {
	for _, node := range nodes {
		mlc.PutNode(node)
	}
}
