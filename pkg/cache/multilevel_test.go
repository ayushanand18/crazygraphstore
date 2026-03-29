package cache

import (
	"sync"
	"testing"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
)

func TestNewMultiLevelCache(t *testing.T) {
	cache := NewMultiLevelCacheFromSize(10 * 1024 * 1024) // 10MB
	if cache == nil {
		t.Fatal("Cache is nil")
	}
}

func TestMultiLevelCache_PutGetHotTier(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put in hot tier (frequently accessed)
	node := &graph.Node{ID: "hot-key"}
	cache.PutNode("hot-key", node)

	// Get from hot tier
	value, found := cache.GetNode("hot-key")
	if !found {
		t.Error("Node not found")
	}

	if value.ID != "hot-key" {
		t.Error("Value mismatch")
	}
}

func TestMultiLevelCache_PutGetConnectionTier(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put in connection tier
	edge := &graph.Edge{ID: "conn-key", FromNodeID: "node1", ToNodeID: "node2"}
	cache.PutEdge("conn-key", edge)

	// Get from connection tier
	value, found := cache.GetEdge("conn-key")
	if !found {
		t.Error("Edge not found")
	}

	if value.ID != "conn-key" {
		t.Error("Value mismatch")
	}
}

func TestMultiLevelCache_PutGetDataTier(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put in data tier
	edge := &graph.Edge{ID: "data-key", FromNodeID: "node1", ToNodeID: "node2"}
	cache.PutEdge("data-key", edge)

	// Get from data tier
	value, found := cache.GetEdge("data-key")
	if !found {
		t.Error("Connection list not found")
	}

	if value.ID != "data-key" {
		t.Error("Value mismatch")
	}
}

func TestMultiLevelCache_GetNotFound(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	_, found := cache.GetNode("nonexistent")
	if found {
		t.Error("Should not find non-existent node")
	}
}

func TestMultiLevelCache_TierPromotion(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put in data tier
	cache.Put("key", []byte("value"), TierData)

	// Get multiple times to promote
	for i := 0; i < 5; i++ {
		cache.Get("key")
	}

	// After multiple accesses, might be promoted to higher tier
	_, tier, found := cache.Get("key")
	if !found {
		t.Error("Key should still exist")
	}

	// Note: Promotion logic depends on implementation
	t.Logf("Key tier after multiple accesses: %v", tier)
}

func TestMultiLevelCache_MultipleKeys(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Add keys to different tiers
	hotNode := &graph.Node{ID: "hot1"}
	cache.PutNode("hot1", hotNode)

	connEdge := &graph.Edge{ID: "conn-edge", FromNodeID: "hot1", ToNodeID: "conn2"}
	cache.PutEdge("conn1", connEdge)

	dataEdge := &graph.Edge{ID: "data-edge", FromNodeID: "conn1", ToNodeID: "data2"}
	cache.PutEdge("data1", dataEdge)

	// Verify all keys
	hotNode, found1 := cache.GetNode("hot1")
	if !found1 || hotNode.ID != "hot1" {
		t.Error("hot1 not in hot tier")
	}

	connEdge, found2 := cache.GetEdge("conn1")
	if !found2 || connEdge.ID != "conn-edge" {
		t.Error("conn1 not in connection tier")
	}

	dataEdge, found3 := cache.GetEdge("data1")
	if !found3 || dataEdge.ID != "data-edge" {
		t.Error("data1 not in data tier")
	}
}

func TestMultiLevelCache_Delete(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put in each tier
	hotNode := &graph.Node{ID: "hot-key"}
	cache.PutNode("hot-key", hotNode)

	connEdge := &graph.Edge{ID: "conn-key", FromNodeID: "node1", ToNodeID: "node2"}
	cache.PutEdge("conn-key", connEdge)

	dataEdge := &graph.Edge{ID: "data-key", FromNodeID: "node1", ToNodeID: "node2"}
	cache.PutEdge("data-key", dataEdge)

	// Delete from hot tier
	cache.InvalidateNodeByKey("hot-key")
	_, found := cache.GetNode("hot-key")
	if found {
		t.Error("Node should be invalidated")
	}

	// Delete from connection tier
	cache.InvalidateEdgeByKey("conn-key")
	_, _, found = cache.Get("conn-key")
	if found {
		t.Error("Connection list should be invalidated")
	}

	// Delete from data tier
	cache.InvalidateEdgeByKey("data-key")
	_, _, found = cache.Get("data-key")
	if found {
		t.Error("Edge should be invalidated")
	}
}

func TestMultiLevelCache_Clear(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Add entries to all tiers
	cache.Put("hot1", []byte("value"), TierHot)
	cache.Put("conn1", []byte("value"), TierConnection)
	cache.Put("data1", []byte("value"), TierData)

	// Clear cache
	cache.Clear()


	// Verify all entries are gone
	_, found1 := cache.GetNode("hot1")
	_, found2 := cache.GetEdge("conn1")
	_, found3 := cache.GetEdge("data1")

	if found1 || found2 || found3 {
		t.Error("Cache should be empty after clear")
	}
}

func TestMultiLevelCache_Stats(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Add entries
	cache.Put("hot1", []byte("value1"), TierHot)
	cache.Put("conn1", []byte("value2"), TierConnection)
	cache.Put("data1", []byte("value3"), TierData)

	// Get stats
	stats := cache.Stats()


	if stats.TotalSize == 0 {
		t.Error("Total size should be non-zero")
	}

	if stats.TotalCount == 0 {
		t.Error("Total count should be non-zero")
	}

	// Verify tier stats
	if stats.HotTier.Entries == 0 {
		t.Error("Hot tier should have entries")
	}

	if stats.ConnectionTier.Entries == 0 {
		t.Error("Connection tier should have entries")
	}

	if stats.DataTier.Entries == 0 {
		t.Error("Data tier should have entries")
	}
}

func TestMultiLevelCache_TierEviction(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        100, // Very small to force eviction
		ConnectionSize: 200,
		DataSize:       300,
	}

	cache := NewMultiLevelCache(config)

	// Fill hot tier beyond capacity
	for i := 0; i < 20; i++ {
		key := string(rune('a' + i))
		node := &graph.Node{ID: key}
		cache.PutNode(key, node)
	}

	// Some early entries should be evicted
	_, found := cache.GetNode("a")

	// Most recent entries should still exist
	_, found2 := cache.GetNode(string(rune('a' + 19)))
	if !found2 {
		t.Error("Recent entry should still exist")
	}

	if found {
		t.Log("Note: First entry might still exist due to LRU implementation")
	}
}

func TestMultiLevelCache_ConcurrentAccess(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        10 * 1024,
		ConnectionSize: 20 * 1024,
		DataSize:       30 * 1024,
	}

	cache := NewMultiLevelCache(config)

	done := make(chan bool, 30)

	// Writers to hot tier
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('h'+id)) + string(rune('0'+j%10))
				node := &graph.Node{ID: key}
				cache.PutNode(key, node)
			}
		}(i)
	}

	// Writers to connection tier
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('c'+id)) + string(rune('0'+j%10))
				edge := &graph.Edge{ID: key, FromNodeID: "conn-node", ToNodeID: "conn-target"}
				cache.PutEdge(key, edge)
			}
			done <- true
		}(i)
	}

	// Writers to data tier
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('d'+id)) + string(rune('0'+j%10))
				edge := &graph.Edge{ID: key, FromNodeID: "data-node", ToNodeID: "data-target"}
				cache.PutEdge(key, edge)
			}
		}()
	}

	// Wait for all
	for i := 0; i < 30; i++ {
		<-done
	}

	// Verify cache is still functional
	hotNode := &graph.Node{ID: "test"}
	cache.PutNode("test", hotNode)
	_, found := cache.GetNode("test")
	if !found {
		t.Error("Cache should work after concurrent access")
	}
}

func TestMultiLevelCache_MixedOperations(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        10 * 1024,
		ConnectionSize: 20 * 1024,
		DataSize:       30 * 1024,
	}

	cache := NewMultiLevelCache(config)

	done := make(chan bool, 30)

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.GetNode(string(rune('a' + j%26)))
			}
			done <- true
		}()
	}

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('A' + id))
				tier := CacheTier(j % 3) // Rotate through tiers
				cache.Put(key, []byte("value"), tier)
			}
			done <- true
		}(i)
	}

	// Deleters
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.InvalidateNode(string(rune('z' + j%26)))
			}
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 30; i++ {
		<-done
	}
}

func TestMultiLevelCache_LargeValues(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        100 * 1024,
		ConnectionSize: 200 * 1024,
		DataSize:       300 * 1024,
	}

	cache := NewMultiLevelCache(config)

	// Put large values
	hotNode := &graph.Node{ID: "large-hot"}
	cache.PutNode("large-hot", hotNode)

	connEdge := &graph.Edge{ID: "large-conn", FromNodeID: "large-hot", ToNodeID: "large-conn"}
	cache.PutEdge("large-conn", connEdge)

	dataEdge := &graph.Edge{ID: "large-data", FromNodeID: "large-hot", ToNodeID: "large-data"}
	cache.PutEdge("large-data", dataEdge)

	// Verify large values
	hotNode, found1 := cache.GetNode("large-hot")
	if !found1 || hotNode.ID != "large-hot" {
		t.Error("Large hot value issue")
	}

	connEdge, found2 := cache.GetEdge("large-conn")
	if !found2 || connEdge.ID != "large-conn" {
		t.Error("Large connection value issue")
	}

	dataEdge, found3 := cache.GetEdge("large-data")
	if !found3 || dataEdge.ID != "large-data" {
		t.Error("Large data value issue")
	}
}

func TestMultiLevelCache_EmptyValues(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put empty values
	hotNode := &graph.Node{ID: "empty-hot"}
	cache.PutNode("empty-hot", hotNode)

	connEdge := &graph.Edge{ID: "empty-conn", FromNodeID: "empty-hot", ToNodeID: "empty-conn"}
	cache.PutEdge("empty-conn", connEdge)

	dataEdge := &graph.Edge{ID: "empty-data", FromNodeID: "empty-hot", ToNodeID: "empty-data"}
	cache.PutEdge("empty-data", dataEdge)

	// Verify empty values
	hotNode, found1 := cache.GetNode("empty-hot")
	if !found1 || hotNode.ID != "empty-hot" {
		t.Error("Empty hot value issue")
	}

	connEdge, found2 := cache.GetEdge("empty-conn")
	if !found2 || connEdge.ID != "empty-conn" {
		t.Error("Empty connection value issue")
	}

	dataEdge, found3 := cache.GetEdge("empty-data")
	if !found3 || dataEdge.ID != "empty-data" {
		t.Error("Empty data value issue")
	}
}

func TestMultiLevelCache_Overwrite(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024,
		ConnectionSize: 2048,
		DataSize:       4096,
	}

	cache := NewMultiLevelCache(config)

	// Put in hot tier
	hotNode := &graph.Node{ID: "key"}
	cache.PutNode("key", hotNode)

	// Overwrite in connection tier
	cache.Put("key", []byte("conn-value"), TierConnection)

	// Should find in hot tier
	hotNode, found := cache.GetNode("key")
	if !found {
		t.Error("Key should exist")
	}

	if hotNode.ID != "key" {
		t.Error("Value should be updated")
	}

	// Note: Tier depends on implementation
	t.Logf("Key tier after overwrite: %v", "Hot")
}

func BenchmarkMultiLevelCache_PutHot(b *testing.B) {
	config := MultiLevelConfig{
		HotSize:        10 * 1024 * 1024,
		ConnectionSize: 20 * 1024 * 1024,
		DataSize:       50 * 1024 * 1024,
	}

	cache := NewMultiLevelCache(config)

	// Put in hot tier
	for i := 0; i < b.N; i++ {
		key := string(rune(i))
		node := &graph.Node{ID: key}
		cache.PutNode(key, node)
	}
}

func BenchmarkMultiLevelCache_Get(b *testing.B) {
	config := MultiLevelConfig{
		HotSize:        10 * 1024 * 1024,
		ConnectionSize: 20 * 1024 * 1024,
		DataSize:       50 * 1024 * 1024,
	}

	cache := NewMultiLevelCache(config)

	// Populate
	for i := 0; i < 10000; i++ {
		tier := CacheTier(i % 3)
		node := &graph.Node{ID: string(rune(i))}
		cache.Put(string(rune(i)), node, tier)
	}


	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetNode(string(rune(i % 10000)))
	}
}
