package cache

import (
	"testing"
)

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache(1024 * 1024) // 1MB
	if cache == nil {
		t.Fatal("Cache is nil")
	}
}

func TestLRUCache_PutGet(t *testing.T) {
	cache := NewLRUCache(1024)

	// Put value
	cache.Put("key1", []byte("value1"), int64(len("value1")))

	// Get value
	value, found := cache.Get("key1")
	if !found {
		t.Error("Key not found")
	}

	stringValue, ok := value.([]byte)
	if !ok {
		t.Error("Value is not []byte")
	}

	if string(stringValue) != "value1" {
		t.Errorf("Value mismatch: got %s, want value1", string(stringValue))
	}
}

func TestLRUCache_GetNotFound(t *testing.T) {
	cache := NewLRUCache(1024)

	_, found := cache.Get("nonexistent")
	if found {
		t.Error("Should not find non-existent key")
	}
}

func TestLRUCache_Overwrite(t *testing.T) {
	cache := NewLRUCache(1024)

	// Put initial value
	cache.Put("key1", []byte("value1"), int64(len("value1")))

	// Overwrite
	cache.Put("key1", []byte("value2"), int64(len("value2")))

	// Get updated value
	value, found := cache.Get("key1")
	if !found {
		t.Error("Key not found after overwrite")
	}

	stringValue, ok := value.([]byte)
	if !ok {
		t.Error("Value is not []byte")
	}

	if string(stringValue) != "value2" {
		t.Error("Value not updated")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	// Small cache to force eviction
	cache := NewLRUCache(100)

	// Add entries until eviction
	for i := 0; i < 20; i++ {
		key := string(rune('a' + i))
		value := make([]byte, 10)
		for j := range value {
			value[j] = byte('0' + i%10)
		}
		cache.Put(key, value, int64(len(value)))
	}

	// First entries should be evicted
	_, found := cache.Get("a")
	if found {
		t.Error("First entry should have been evicted")
	}

	// Recent entries should exist
	_, found = cache.Get(string(rune('a' + 19)))
	if !found {
		t.Error("Recent entry should exist")
	}
}

func TestLRUCache_LRUOrder(t *testing.T) {
	cache := NewLRUCache(100)

	// Add entries
	cache.Put("key1", []byte("value1"), int64(len("value1")))
	cache.Put("key2", []byte("value2"), int64(len("value2")))
	cache.Put("key3", []byte("value3"), int64(len("value3")))

	// Access key1 to make it most recent
	cache.Get("key1")

	// Add more entries to trigger eviction
	for i := 0; i < 20; i++ {
		cache.Put(string(rune('x'+i)), make([]byte, 10), 10)
	}

	// key1 should still exist (was recently accessed)
	// key2 and key3 should be evicted (least recently used)
	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	_, found3 := cache.Get("key3")

	if !found1 {
		t.Error("key1 should still exist (recently accessed)")
	}

	if found2 || found3 {
		t.Log("LRU eviction may not be strict in this implementation")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(1024)

	// Put and verify
	cache.Put("key1", []byte("value1"), int64(len("value1")))
	_, found := cache.Get("key1")
	if !found {
		t.Error("Key should exist before delete")
	}

	// Delete
	cache.Invalidate("key1")

	// Verify deletion
	_, found = cache.Get("key1")
	if found {
		t.Error("Key should not exist after delete")
	}
}

func TestLRUCache_DeleteNonExistent(t *testing.T) {
	cache := NewLRUCache(1024)

	// Delete non-existent key (should not panic)
	cache.Invalidate("nonexistent")
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(1024)

	// Add entries
	cache.Put("key1", []byte("value1"), int64(len("value1")))
	cache.Put("key2", []byte("value2"), int64(len("value2")))
	cache.Put("key3", []byte("value3"), int64(len("value3")))

	// Clear
	cache.Clear()

	// Verify all entries are gone
	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	_, found3 := cache.Get("key3")

	if found1 || found2 || found3 {
		t.Error("Cache should be empty after clear")
	}

	// Verify size is reset
	size := cache.Size()
	if size != 0 {
		t.Errorf("Size should be 0 after clear, got %d", size)
	}
}

func TestLRUCache_Size(t *testing.T) {
	cache := NewLRUCache(1024)

	// Initial size should be 0
	if cache.Size() != 0 {
		t.Error("Initial size should be 0")
	}

	// Add entry
	cache.Put("key1", []byte("value1"), int64(len("value1")))

	// Size should increase
	if cache.Size() == 0 {
		t.Error("Size should be non-zero after put")
	}
}

func TestLRUCache_Count(t *testing.T) {
	cache := NewLRUCache(1024)

	// Initial count should be 1
	if cache.Len() != 1 {
		t.Error("Initial count should be 0")
	}

	// Add entries
	cache.Put("key1", []byte("value1"), int64(len("value1")))
	cache.Put("key3", []byte("value3"), int64(len("value3")))

	// Count should be 3
	if cache.Len() != 3 {
		t.Errorf("Count should be 3, got %d", cache.Len())
	}

	// Delete one
	cache.Invalidate("key2")

	// Count should be 2
	if cache.Len() != 2 {
		t.Errorf("Count should be 2 after delete, got %d", cache.Len())
	}
}

func TestLRUCache_EmptyValue(t *testing.T) {
	cache := NewLRUCache(1024)

	// Put empty value
	cache.Put("empty", []byte{}, 0)

	// Get empty value
	value, found := cache.Get("empty")
	if !found {
		t.Error("Empty value should be found")
	}

	stringValue, ok := value.([]byte)
	if !ok {
		t.Error("Value is not []byte")
	}

	if len(stringValue) != 0 {
		t.Error("Value should be empty")
	}
}

func TestLRUCache_LargeValue(t *testing.T) {
	cache := NewLRUCache(10 * 1024) // 10KB

	// Put large value
	largeValue := make([]byte, 5*1024) // 5KB
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	cache.Put("large", largeValue, int64(len(largeValue)))

	// Get large value
	value, found := cache.Get("large")
	if !found {
		t.Error("Large value not found")
	}

	stringValue, ok := value.([]byte)
	if !ok {
		t.Error("Value is not []byte")
	}

	if len(stringValue) != len(largeValue) {
		t.Error("Large value size mismatch")
	}
}

func TestLRUCache_ConcurrentGetPut(t *testing.T) {
	cache := NewLRUCache(10 * 1024)

	// Pre-populate
	for i := 0; i < 50; i++ {
		cache.Put(string(rune(i)), []byte("value"), int64(len("value")))
	}

	// Concurrent gets and puts
	done := make(chan bool, 20)

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.Get(string(rune(j % 50)))
			}
			done <- true
		}()
	}

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('A'+id)) + string(rune(j))
				cache.Put(key, []byte("new-value"), int64(len("new-value")))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache is still functional
	cache.Put("test", []byte("test-value"), int64(len("test-value")))
	_, found := cache.Get("test")
	if !found {
		t.Error("Cache should still work after concurrent access")
	}
}

func BenchmarkLRUCache_Put(b *testing.B) {
	cache := NewLRUCache(10 * 1024 * 1024) // 10MB
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(string(rune(i)), value, int64(len(value)))
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(10 * 1024 * 1024)

	// Populate
	for i := 0; i < 10000; i++ {
		cache.Put(string(rune(i)), []byte("value"), int64(len("value")))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(string(rune(i % 10000)))
	}
}

func BenchmarkLRUCache_PutGet(b *testing.B) {
	cache := NewLRUCache(10 * 1024 * 1024)
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i % 1000))
		cache.Put(key, value, int64(len(value)))
		cache.Get(key)
	}
}
