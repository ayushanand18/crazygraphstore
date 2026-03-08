
// Package cache provides multi-level caching for the storage engine.
package cache

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
)

// Entry represents a cache entry with its value and metadata.
type Entry struct {
	Key     string
	Value   interface{}
	Size    int64
	Version uint64
}

// LRUCache is a thread-safe Least Recently Used cache.
type LRUCache struct {
	maxSize     int64
	currentSize atomic.Int64
	items       map[string]*list.Element
	evictList   *list.List
	mu          sync.RWMutex
	hits        atomic.Uint64
	misses      atomic.Uint64
	evictions   atomic.Uint64
}

// NewLRUCache creates a new LRU cache with the specified maximum size in bytes.
func NewLRUCache(maxSize int64) *LRUCache {
	return &LRUCache{
		maxSize:   maxSize,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return nil, false
	}

	// Move to front (most recently used)
	c.evictList.MoveToFront(elem)
	c.hits.Add(1)

	entry := elem.Value.(*Entry)
	return entry.Value, true
}

// Put adds or updates a value in the cache.
func (c *LRUCache) Put(key string, value interface{}, size int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing entry
		entry := elem.Value.(*Entry)
		oldSize := entry.Size

		entry.Value = value
		entry.Size = size
		entry.Version++

		// Update size
		c.currentSize.Add(size - oldSize)

		// Move to front
		c.evictList.MoveToFront(elem)
		return nil
	}

	// Evict entries if necessary to make room
	for c.currentSize.Load()+size > c.maxSize && c.evictList.Len() > 0 {
		c.evictOldest()
	}

	// Check if entry is too large for cache
	if size > c.maxSize {
		return fmt.Errorf("entry size %d exceeds cache max size %d", size, c.maxSize)
	}

	// Add new entry
	entry := &Entry{
		Key:     key,
		Value:   value,
		Size:    size,
		Version: 1,
	}

	elem := c.evictList.PushFront(entry)
	c.items[key] = elem
	c.currentSize.Add(size)

	return nil
}

// Invalidate removes a key from the cache.
func (c *LRUCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

// evictOldest removes the least recently used item.
func (c *LRUCache) evictOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
		c.evictions.Add(1)
	}
}

// removeElement removes an element from the cache.
func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	entry := elem.Value.(*Entry)
	delete(c.items, entry.Key)
	c.currentSize.Add(-entry.Size)
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.currentSize.Store(0)
}

// Stats returns cache statistics.
type CacheStats struct {
	Size       int64
	MaxSize    int64
	Entries    int
	Hits       uint64
	Misses     uint64
	Evictions  uint64
	HitRate    float64
	Utilization float64
}

// Stats returns current cache statistics.
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	entries := len(c.items)
	c.mu.RUnlock()

	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	size := c.currentSize.Load()
	utilization := 0.0
	if c.maxSize > 0 {
		utilization = float64(size) / float64(c.maxSize)
	}

	return CacheStats{
		Size:        size,
		MaxSize:     c.maxSize,
		Entries:     entries,
		Hits:        hits,
		Misses:      misses,
		Evictions:   c.evictions.Load(),
		HitRate:     hitRate,
		Utilization: utilization,
	}
}

// Resize changes the maximum size of the cache.
func (c *LRUCache) Resize(newSize int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxSize = newSize

	// Evict entries if necessary
	for c.currentSize.Load() > c.maxSize && c.evictList.Len() > 0 {
		c.evictOldest()
	}
}

// Contains checks if a key exists in the cache without updating LRU.
func (c *LRUCache) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.items[key]
	return ok
}

// Peek retrieves a value without updating LRU order.
func (c *LRUCache) Peek(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*Entry)
	return entry.Value, true
}

// Len returns the number of entries in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Size returns the current size of the cache in bytes.
func (c *LRUCache) Size() int64 {
	return c.currentSize.Load()
}
