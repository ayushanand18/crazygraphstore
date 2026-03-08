
package cache

import (
	"testing"
)

func TestNewMultiLevelCache(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        1024 * 1024,      // 1MB
		ConnectionSize: 10 * 1024 * 1024, // 10MB
		DataSize:       50 * 1024 * 1024, // 50MB
	}
	
	cache := NewMultiLevelCache(config)
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
	cache.Put("hot-key", []byte("hot-value"), TierHot)
	
	// Get from hot tier
	value, tier, found := cache.Get("hot-key")
	if !found {
		t.Error("Hot key not found")
	}
	
	if tier != TierHot {
		t.Errorf("Expected TierHot, got %v", tier)
	}
	
	if string(value) != "hot-value" {
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
	cache.Put("conn-key", []byte("conn-value"), TierConnection)
	
	// Get from connection tier
	value, tier, found := cache.Get("conn-key")
	if !found {
		t.Error("Connection key not found")
	}
	
	if tier != TierConnection {
		t.Errorf("Expected TierConnection, got %v", tier)
	}
	
	if string(value) != "conn-value" {
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
	cache.Put("data-key", []byte("data-value"), TierData)
	
	// Get from data tier
	value, tier, found := cache.Get("data-key")
	if !found {
		t.Error("Data key not found")
	}
	
	if tier != TierData {
		t.Errorf("Expected TierData, got %v", tier)
	}
	
	if string(value) != "data-value" {
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
	
	_, _, found := cache.Get("nonexistent")
	if found {
		t.Error("Should not find non-existent key")
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
	cache.Put("hot1", []byte("hot-value-1"), TierHot)
	cache.Put("hot2", []byte("hot-value-2"), TierHot)
	cache.Put("conn1", []byte("conn-value-1"), TierConnection)
	cache.Put("data1", []byte("data-value-1"), TierData)
	
	// Verify all keys
	_, tier1, found1 := cache.Get("hot1")
	if !found1 || tier1 != TierHot {
		t.Error("hot1 not in hot tier")
	}
	
	_, tier2, found2 := cache.Get("conn1")
	if !found2 || tier2 != TierConnection {
		t.Error("conn1 not in connection tier")
	}
	
	_, tier3, found3 := cache.Get("data1")
	if !found3 || tier3 != TierData {
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
	cache.Put("hot-key", []byte("value"), TierHot)
	cache.Put("conn-key", []byte("value"), TierConnection)
	cache.Put("data-key", []byte("value"), TierData)
	
	// Delete from hot tier
	cache.Delete("hot-key")
	_, _, found := cache.Get("hot-key")
	if found {
		t.Error("hot-key should be deleted")
	}
	
	// Delete from connection tier
	cache.Delete("conn-key")
	_, _, found = cache.Get("conn-key")
	if found {
		t.Error("conn-key should be deleted")
	}
	
	// Delete from data tier
	cache.Delete("data-key")
	_, _, found = cache.Get("data-key")
	if found {
		t.Error("data-key should be deleted")
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
	_, _, found1 := cache.Get("hot1")
	_, _, found2 := cache.Get("conn1")
	_, _, found3 := cache.Get("data1")
	
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
	if stats.HotTier.Count == 0 {
		t.Error("Hot tier should have entries")
	}
	
	if stats.ConnectionTier.Count == 0 {
		t.Error("Connection tier should have entries")
	}
	
	if stats.DataTier.Count == 0 {
		t.Error("Data tier should have entries")
	}
}

func TestMultiLevelCache_TierEviction(t *testing.T) {
	config := MultiLevelConfig{
		HotSize:        100,  // Very small to force eviction
		ConnectionSize: 200,
		DataSize:       300,
	}
	
	cache := NewMultiLevelCache(config)
	
	// Fill hot tier beyond capacity
	for i := 0; i < 20; i++ {
		key := string(rune('a' + i))
		value := make([]byte, 10)
		cache.Put(key, value, TierHot)
	}
	
	// Some early entries should be evicted
	_, _, found := cache.Get("a")
	
	// Most recent entries should still exist
	_, _, found2 := cache.Get(string(rune('a' + 19)))
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
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('h'+id)) + string(rune('0'+j%10))
				cache.Put(key, []byte("hot-value"), TierHot)
			}
			done <- true
		}(i)
	}
	
	// Writers to connection tier
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('c'+id)) + string(rune('0'+j%10))
				cache.Put(key, []byte("conn-value"), TierConnection)
			}
			done <- true
		}(i)
	}
	
	// Writers to data tier
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('d'+id)) + string(rune('0'+j%10))
				cache.Put(key, []byte("data-value"), TierData)
			}
			done <- true
		}(i)
	}
	
	// Wait for all
	for i := 0; i < 30; i++ {
		<-done
	}
	
	// Verify cache is still functional
	cache.Put("test", []byte("test-value"), TierHot)
	_, _, found := cache.Get("test")
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
				cache.Get(string(rune('a' + j%26)))
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
				cache.Delete(string(rune('z' + j%26)))
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
	largeValue := make([]byte, 10*1024) // 10KB
	
	cache.Put("large-hot", largeValue, TierHot)
	cache.Put("large-conn", largeValue, TierConnection)
	cache.Put("large-data", largeValue, TierData)
	
	// Verify large values
	val1, _, found1 := cache.Get("large-hot")
	if !found1 || len(val1) != len(largeValue) {
		t.Error("Large hot value issue")
	}
	
	val2, _, found2 := cache.Get("large-conn")
	if !found2 || len(val2) != len(largeValue) {
		t.Error("Large connection value issue")
	}
	
	val3, _, found3 := cache.Get("large-data")
	if !found3 || len(val3) != len(largeValue) {
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
	cache.Put("empty-hot", []byte{}, TierHot)
	cache.Put("empty-conn", []byte{}, TierConnection)
	cache.Put("empty-data", []byte{}, TierData)
	
	// Verify empty values
	val1, _, found1 := cache.Get("empty-hot")
	if !found1 || len(val1) != 0 {
		t.Error("Empty hot value issue")
	}
	
	val2, _, found2 := cache.Get("empty-conn")
	if !found2 || len(val2) != 0 {
		t.Error("Empty connection value issue")
	}
	
	val3, _, found3 := cache.Get("empty-data")
	if !found3 || len(val3) != 0 {
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
	cache.Put("key", []byte("hot-value"), TierHot)
	
	// Overwrite in connection tier
	cache.Put("key", []byte("conn-value"), TierConnection)
	
	// Should find in connection tier
	value, tier, found := cache.Get("key")
	if !found {
		t.Error("Key should exist")
	}
	
	if string(value) != "conn-value" {
		t.Error("Value should be updated")
	}
	
	// Note: Tier depends on implementation
	t.Logf("Tier after overwrite: %v", tier)
}

func BenchmarkMultiLevelCache_PutHot(b *testing.B) {
	config := MultiLevelConfig{
		HotSize:        10 * 1024 * 1024,
		ConnectionSize: 20 * 1024 * 1024,
		DataSize:       50 * 1024 * 1024,
	}
	
	cache := NewMultiLevelCache(config)
	value := []byte("benchmark-value")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(string(rune(i)), value, TierHot)
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
		cache.Put(string(rune(i)), []byte("value"), tier)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(string(rune(i % 10000)))
	}
}
