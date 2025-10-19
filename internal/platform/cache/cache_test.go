package cache

import (
	"sync"
	"testing"
	"time"

	"aethonx/internal/testutil"
)

func TestNewMemoryCache(t *testing.T) {
	t.Run("creates cache with specified capacity", func(t *testing.T) {
		cache := NewMemoryCache(100)
		testutil.AssertEqual(t, cache.Capacity(), 100, "capacity should match")
		testutil.AssertEqual(t, cache.Size(), 0, "new cache should be empty")
	})

	t.Run("uses default capacity for invalid values", func(t *testing.T) {
		cache := NewMemoryCache(0)
		testutil.AssertEqual(t, cache.Capacity(), 100, "should use default capacity")

		cache = NewMemoryCache(-10)
		testutil.AssertEqual(t, cache.Capacity(), 100, "should use default capacity for negative")
	})
}

func TestMemoryCache_SetAndGet(t *testing.T) {
	t.Run("stores and retrieves value", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 0)

		value, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "should find stored value")
		testutil.AssertEqual(t, value, "value1", "value should match")
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		cache := NewMemoryCache(10)
		value, found := cache.Get("missing")

		testutil.AssertTrue(t, !found, "should not find missing key")
		testutil.AssertTrue(t, value == nil, "value should be nil")
	})

	t.Run("updates existing key", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 0)
		cache.Set("key1", "value2", 0)

		value, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "should find key")
		testutil.AssertEqual(t, value, "value2", "should have updated value")
		testutil.AssertEqual(t, cache.Size(), 1, "size should still be 1")
	})

	t.Run("stores different types", func(t *testing.T) {
		cache := NewMemoryCache(10)

		cache.Set("string", "value", 0)
		cache.Set("int", 42, 0)
		cache.Set("struct", struct{ Name string }{"test"}, 0)

		val1, _ := cache.Get("string")
		testutil.AssertEqual(t, val1, "value", "should store string")

		val2, _ := cache.Get("int")
		testutil.AssertEqual(t, val2, 42, "should store int")

		val3, _ := cache.Get("struct")
		testutil.AssertEqual(t, val3.(struct{ Name string }).Name, "test", "should store struct")
	})
}

func TestMemoryCache_TTL(t *testing.T) {
	t.Run("expires item after TTL", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 100*time.Millisecond)

		// Should be available immediately
		value, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "should find key before expiration")
		testutil.AssertEqual(t, value, "value1", "value should match")

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Should be expired
		value, found = cache.Get("key1")
		testutil.AssertTrue(t, !found, "should not find expired key")
		testutil.AssertTrue(t, value == nil, "value should be nil for expired key")
	})

	t.Run("zero TTL means no expiration", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 0)

		time.Sleep(50 * time.Millisecond)

		value, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "should find key with zero TTL")
		testutil.AssertEqual(t, value, "value1", "value should match")
	})

	t.Run("updates TTL on set", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 100*time.Millisecond)

		time.Sleep(60 * time.Millisecond)

		// Update with new TTL
		cache.Set("key1", "value2", 200*time.Millisecond)

		time.Sleep(60 * time.Millisecond)

		// Should still be available (original 100ms would have expired)
		value, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "should find key with updated TTL")
		testutil.AssertEqual(t, value, "value2", "should have updated value")
	})
}

func TestMemoryCache_LRUEviction(t *testing.T) {
	t.Run("evicts LRU item when at capacity", func(t *testing.T) {
		cache := NewMemoryCache(3)

		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.Set("key3", "value3", 0)

		testutil.AssertEqual(t, cache.Size(), 3, "should have 3 items")

		// Add 4th item, should evict key1 (oldest)
		cache.Set("key4", "value4", 0)

		testutil.AssertEqual(t, cache.Size(), 3, "should still have 3 items")

		_, found := cache.Get("key1")
		testutil.AssertTrue(t, !found, "oldest key should be evicted")

		_, found = cache.Get("key4")
		testutil.AssertTrue(t, found, "new key should exist")
	})

	t.Run("get marks item as recently used", func(t *testing.T) {
		cache := NewMemoryCache(3)

		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.Set("key3", "value3", 0)

		// Access key1, making it recently used
		cache.Get("key1")

		// Add 4th item, should evict key2 (now oldest)
		cache.Set("key4", "value4", 0)

		_, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "recently used key should not be evicted")

		_, found = cache.Get("key2")
		testutil.AssertTrue(t, !found, "LRU key should be evicted")
	})

	t.Run("set updates LRU order", func(t *testing.T) {
		cache := NewMemoryCache(3)

		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.Set("key3", "value3", 0)

		// Update key1, making it recently used
		cache.Set("key1", "updated", 0)

		// Add 4th item, should evict key2 (now oldest)
		cache.Set("key4", "value4", 0)

		value, found := cache.Get("key1")
		testutil.AssertTrue(t, found, "updated key should not be evicted")
		testutil.AssertEqual(t, value, "updated", "should have updated value")

		_, found = cache.Get("key2")
		testutil.AssertTrue(t, !found, "LRU key should be evicted")
	})
}

func TestMemoryCache_Delete(t *testing.T) {
	t.Run("deletes existing key", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		cache.Delete("key1")

		testutil.AssertEqual(t, cache.Size(), 1, "size should decrease")

		_, found := cache.Get("key1")
		testutil.AssertTrue(t, !found, "deleted key should not be found")

		_, found = cache.Get("key2")
		testutil.AssertTrue(t, found, "other keys should remain")
	})

	t.Run("delete non-existent key is safe", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Delete("missing")
		testutil.AssertEqual(t, cache.Size(), 0, "size should remain 0")
	})
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(10)
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)

	cache.Clear()

	testutil.AssertEqual(t, cache.Size(), 0, "cache should be empty")

	_, found := cache.Get("key1")
	testutil.AssertTrue(t, !found, "all keys should be removed")
}

func TestMemoryCache_SetCapacity(t *testing.T) {
	t.Run("increases capacity", func(t *testing.T) {
		cache := NewMemoryCache(3)
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.Set("key3", "value3", 0)

		cache.SetCapacity(5)
		testutil.AssertEqual(t, cache.Capacity(), 5, "capacity should be updated")
		testutil.AssertEqual(t, cache.Size(), 3, "items should remain")
	})

	t.Run("decreases capacity and evicts items", func(t *testing.T) {
		cache := NewMemoryCache(5)
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.Set("key3", "value3", 0)
		cache.Set("key4", "value4", 0)
		cache.Set("key5", "value5", 0)

		cache.SetCapacity(2)
		testutil.AssertEqual(t, cache.Capacity(), 2, "capacity should be updated")
		testutil.AssertEqual(t, cache.Size(), 2, "should evict excess items")
	})

	t.Run("zero or negative capacity defaults to 1", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.SetCapacity(0)
		testutil.AssertEqual(t, cache.Capacity(), 1, "zero capacity should default to 1")

		cache.SetCapacity(-5)
		testutil.AssertEqual(t, cache.Capacity(), 1, "negative capacity should default to 1")
	})
}

func TestMemoryCache_CleanExpired(t *testing.T) {
	cache := NewMemoryCache(10)

	cache.Set("key1", "value1", 50*time.Millisecond)
	cache.Set("key2", "value2", 200*time.Millisecond)
	cache.Set("key3", "value3", 0) // no expiration

	time.Sleep(100 * time.Millisecond)

	removed := cache.CleanExpired()
	testutil.AssertEqual(t, removed, 1, "should remove 1 expired item")
	testutil.AssertEqual(t, cache.Size(), 2, "should have 2 items remaining")

	_, found := cache.Get("key1")
	testutil.AssertTrue(t, !found, "expired key should be removed")

	_, found = cache.Get("key2")
	testutil.AssertTrue(t, found, "non-expired key should remain")

	_, found = cache.Get("key3")
	testutil.AssertTrue(t, found, "permanent key should remain")
}

func TestMemoryCache_Keys(t *testing.T) {
	t.Run("returns all active keys", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)
		cache.Set("key3", "value3", 0)

		keys := cache.Keys()
		testutil.AssertEqual(t, len(keys), 3, "should return all keys")
		testutil.AssertContains(t, keys, "key1", "should contain key1")
		testutil.AssertContains(t, keys, "key2", "should contain key2")
		testutil.AssertContains(t, keys, "key3", "should contain key3")
	})

	t.Run("excludes expired keys", func(t *testing.T) {
		cache := NewMemoryCache(10)
		cache.Set("key1", "value1", 50*time.Millisecond)
		cache.Set("key2", "value2", 0)

		time.Sleep(100 * time.Millisecond)

		keys := cache.Keys()
		testutil.AssertEqual(t, len(keys), 1, "should exclude expired keys")
		testutil.AssertContains(t, keys, "key2", "should contain active key")
	})

	t.Run("returns empty slice for empty cache", func(t *testing.T) {
		cache := NewMemoryCache(10)
		keys := cache.Keys()
		testutil.AssertEqual(t, len(keys), 0, "should return empty slice")
	})
}

func TestMemoryCache_StartCleanupWorker(t *testing.T) {
	cache := NewMemoryCache(10)

	cache.Set("key1", "value1", 100*time.Millisecond)
	cache.Set("key2", "value2", 100*time.Millisecond)
	cache.Set("key3", "value3", 0)

	// Start cleanup worker with short interval
	stop := cache.StartCleanupWorker(50 * time.Millisecond)
	defer stop()

	// Wait for items to expire and worker to clean them
	time.Sleep(200 * time.Millisecond)

	testutil.AssertEqual(t, cache.Size(), 1, "expired items should be cleaned")

	_, found := cache.Get("key3")
	testutil.AssertTrue(t, found, "permanent key should remain")
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache(100)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cache.Set(string(rune('A'+n)), n, 0)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cache.Get(string(rune('A' + n)))
		}(i)
	}

	wg.Wait()

	// Cache should be in consistent state
	testutil.AssertTrue(t, cache.Size() <= 100, "size should not exceed capacity")
}

func TestMemoryCache_Interface(t *testing.T) {
	var _ Cache = (*MemoryCache)(nil) // Compile-time interface check
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := NewMemoryCache(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(string(rune(i)), i, 0)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := NewMemoryCache(10000)

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		cache.Set(string(rune(i)), i, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(string(rune(i % 1000)))
	}
}

func BenchmarkMemoryCache_ConcurrentSet(b *testing.B) {
	cache := NewMemoryCache(10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(string(rune(i)), i, 0)
			i++
		}
	})
}

func BenchmarkMemoryCache_ConcurrentGet(b *testing.B) {
	cache := NewMemoryCache(10000)

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		cache.Set(string(rune(i)), i, 0)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(string(rune(i % 1000)))
			i++
		}
	})
}
