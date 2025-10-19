// Package cache provides an in-memory caching layer with TTL and LRU eviction.
package cache

import (
	"container/list"
	"sync"
	"time"
)

// Cache defines the interface for a generic cache
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns the value and true if found, nil and false otherwise.
	Get(key string) (interface{}, bool)

	// Set stores a value in the cache with a TTL.
	// If ttl is 0, the item never expires.
	Set(key string, value interface{}, ttl time.Duration)

	// Delete removes a value from the cache.
	Delete(key string)

	// Clear removes all values from the cache.
	Clear()

	// Size returns the current number of items in the cache.
	Size() int

	// Capacity returns the maximum number of items the cache can hold.
	Capacity() int
}

// entry represents a cached item with metadata
type entry struct {
	key       string
	value     interface{}
	expiresAt time.Time
	element   *list.Element // for LRU tracking
}

// MemoryCache implements an in-memory LRU cache with TTL support
type MemoryCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*entry
	lruList  *list.List // doubly linked list for LRU tracking
}

// NewMemoryCache creates a new in-memory cache with the specified capacity.
// When the cache reaches capacity, the least recently used item is evicted.
//
// Example:
//   cache := cache.NewMemoryCache(100) // cache with capacity of 100 items
func NewMemoryCache(capacity int) *MemoryCache {
	if capacity <= 0 {
		capacity = 100 // default capacity
	}

	return &MemoryCache{
		capacity: capacity,
		items:    make(map[string]*entry),
		lruList:  list.New(),
	}
}

// Get retrieves a value from the cache.
// If the item exists and hasn't expired, it's marked as recently used.
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.deleteEntry(entry)
		return nil, false
	}

	// Mark as recently used
	c.lruList.MoveToFront(entry.element)

	return entry.value, true
}

// Set stores a value in the cache with a TTL.
// If the key already exists, its value and TTL are updated.
// If ttl is 0, the item never expires.
func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	// Update existing entry
	if existing, exists := c.items[key]; exists {
		existing.value = value
		existing.expiresAt = expiresAt
		c.lruList.MoveToFront(existing.element)
		return
	}

	// Evict LRU item if at capacity
	if len(c.items) >= c.capacity {
		c.evictLRU()
	}

	// Add new entry
	entry := &entry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	entry.element = c.lruList.PushFront(entry)
	c.items[key] = entry
}

// Delete removes a value from the cache.
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.items[key]; exists {
		c.deleteEntry(entry)
	}
}

// Clear removes all values from the cache.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*entry)
	c.lruList.Init()
}

// Size returns the current number of items in the cache.
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Capacity returns the maximum number of items the cache can hold.
func (c *MemoryCache) Capacity() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capacity
}

// SetCapacity changes the cache capacity.
// If the new capacity is smaller than the current size, LRU items are evicted.
func (c *MemoryCache) SetCapacity(capacity int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if capacity <= 0 {
		capacity = 1
	}

	c.capacity = capacity

	// Evict items if over capacity
	for len(c.items) > c.capacity {
		c.evictLRU()
	}
}

// CleanExpired removes all expired items from the cache.
// This can be called periodically to free up memory.
func (c *MemoryCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	// Iterate over all items and remove expired ones
	for _, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			c.deleteEntry(entry)
			removed++
		}
	}

	return removed
}

// Keys returns all keys currently in the cache (excluding expired items).
func (c *MemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	keys := make([]string, 0, len(c.items))

	for key, entry := range c.items {
		// Skip expired items
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			continue
		}
		keys = append(keys, key)
	}

	return keys
}

// evictLRU removes the least recently used item from the cache.
// Must be called with c.mu held.
func (c *MemoryCache) evictLRU() {
	if c.lruList.Len() == 0 {
		return
	}

	// Get LRU item (back of list)
	element := c.lruList.Back()
	if element != nil {
		entry := element.Value.(*entry)
		c.deleteEntry(entry)
	}
}

// deleteEntry removes an entry from the cache.
// Must be called with c.mu held.
func (c *MemoryCache) deleteEntry(entry *entry) {
	delete(c.items, entry.key)
	c.lruList.Remove(entry.element)
}

// StartCleanupWorker starts a background goroutine that periodically
// removes expired items from the cache.
//
// The worker runs every interval duration and calls CleanExpired().
// Returns a function that can be called to stop the worker.
//
// Example:
//   cache := cache.NewMemoryCache(100)
//   stop := cache.StartCleanupWorker(5 * time.Minute)
//   defer stop() // Stop the worker when done
func (c *MemoryCache) StartCleanupWorker(interval time.Duration) func() {
	stopChan := make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				c.CleanExpired()
			case <-stopChan:
				ticker.Stop()
				return
			}
		}
	}()

	// Return stop function
	return func() {
		close(stopChan)
	}
}
