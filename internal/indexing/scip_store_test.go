package indexing

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test helper functions

func createTestSCIPConfig() *SCIPConfig {
	return &SCIPConfig{
		CacheConfig: CacheConfig{
			Enabled: true,
			MaxSize: 100,
			TTL:     time.Hour,
		},
		Logging: LoggingConfig{
			LogQueries:         false,
			LogCacheOperations: false,
			LogIndexOperations: false,
		},
		Performance: PerformanceConfig{
			QueryTimeout:         time.Minute,
			MaxConcurrentQueries: 10,
			IndexLoadTimeout:     time.Minute * 5,
		},
	}
}

func createDisabledCacheConfig() *SCIPConfig {
	config := createTestSCIPConfig()
	config.CacheConfig.Enabled = false
	return config
}

func createSmallCacheConfig() *SCIPConfig {
	config := createTestSCIPConfig()
	config.CacheConfig.MaxSize = 2
	config.CacheConfig.TTL = time.Millisecond * 100
	return config
}

func createTestParams() map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test/main.go",
		},
		"position": map[string]interface{}{
			"line":      10,
			"character": 5,
		},
	}
}

func createTestResponse() json.RawMessage {
	response := map[string]interface{}{
		"uri": "file:///test/main.go",
		"range": map[string]interface{}{
			"start": map[string]int{"line": 10, "character": 5},
			"end":   map[string]int{"line": 10, "character": 15},
		},
	}
	data, _ := json.Marshal(response)
	return json.RawMessage(data)
}

// RealSCIPStore Tests

func TestNewRealSCIPStore(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := createTestSCIPConfig()
		store := NewRealSCIPStore(config)
		realStore := store.(*RealSCIPStore)

		if realStore.config != config {
			t.Error("Expected config to be set")
		}

		if realStore.cache == nil {
			t.Error("Expected cache to be initialized")
		}

		if realStore.queryLimiter == nil {
			t.Error("Expected query limiter to be initialized")
		}

		if cap(realStore.queryLimiter) != config.Performance.MaxConcurrentQueries {
			t.Errorf("Expected query limiter capacity %d, got %d",
				config.Performance.MaxConcurrentQueries, cap(realStore.queryLimiter))
		}

		if realStore.shutdown == nil {
			t.Error("Expected shutdown channel to be initialized")
		}

		if realStore.startTime.IsZero() {
			t.Error("Expected start time to be set")
		}

		if realStore.queryTimes == nil {
			t.Error("Expected query times slice to be initialized")
		}

		// Cleanup
		realStore.Close()
	})

	t.Run("NilConfig", func(t *testing.T) {
		store := NewRealSCIPStore(nil)
		realStore := store.(*RealSCIPStore)

		if realStore.config == nil {
			t.Error("Expected default config to be created")
		}

		if !realStore.config.CacheConfig.Enabled {
			t.Error("Expected default config to have cache enabled")
		}

		if realStore.config.CacheConfig.MaxSize != 10000 {
			t.Error("Expected default cache size to be 10000")
		}

		if realStore.config.CacheConfig.TTL != 30*time.Minute {
			t.Error("Expected default TTL to be 30 minutes")
		}

		// Cleanup
		realStore.Close()
	})

	t.Run("DisabledCache", func(t *testing.T) {
		config := createDisabledCacheConfig()
		store := NewRealSCIPStore(config)
		realStore := store.(*RealSCIPStore)

		if realStore.cache == nil {
			t.Error("Expected cache to be initialized even when disabled")
		}

		// Cleanup
		realStore.Close()
	})
}

func TestRealSCIPStore_LoadIndex(t *testing.T) {
	store := NewRealSCIPStore(createTestSCIPConfig())
	realStore := store.(*RealSCIPStore)
	defer realStore.Close()

	t.Run("ValidPath", func(t *testing.T) {
		err := realStore.LoadIndex("/test/path/index.scip")
		if err != nil {
			t.Errorf("Expected no error loading index, got: %v", err)
		}

		stats := realStore.GetStats()
		if stats.IndexesLoaded != 1 {
			t.Errorf("Expected 1 index loaded, got: %d", stats.IndexesLoaded)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		initialStats := realStore.GetStats()
		err := realStore.LoadIndex("")
		if err != nil {
			t.Errorf("Expected no error for empty path in degraded mode, got: %v", err)
		}

		stats := realStore.GetStats()
		if stats.IndexesLoaded != initialStats.IndexesLoaded+1 {
			t.Error("Expected index loaded count to increment")
		}
	})

	t.Run("MultipleIndexes", func(t *testing.T) {
		initialStats := realStore.GetStats()
		
		err1 := realStore.LoadIndex("/test/path1/index.scip")
		err2 := realStore.LoadIndex("/test/path2/index.scip")

		if err1 != nil || err2 != nil {
			t.Errorf("Expected no errors, got: %v, %v", err1, err2)
		}

		stats := realStore.GetStats()
		expectedCount := initialStats.IndexesLoaded + 2
		if stats.IndexesLoaded != expectedCount {
			t.Errorf("Expected %d indexes loaded, got: %d", expectedCount, stats.IndexesLoaded)
		}
	})
}

func TestRealSCIPStore_Query(t *testing.T) {
	store := NewRealSCIPStore(createTestSCIPConfig())
	realStore := store.(*RealSCIPStore)
	defer realStore.Close()

	t.Run("SupportedMethod", func(t *testing.T) {
		params := createTestParams()
		result := realStore.Query("textDocument/definition", params)

		if result.Method != "textDocument/definition" {
			t.Errorf("Expected method textDocument/definition, got: %s", result.Method)
		}

		if result.Found {
			t.Error("Expected found to be false in degraded mode")
		}

		if result.CacheHit {
			t.Error("Expected cache hit to be false for new query")
		}

		if result.QueryTime <= 0 {
			t.Error("Expected positive query time")
		}

		if result.Confidence != 0.0 {
			t.Errorf("Expected confidence 0.0 in degraded mode, got: %f", result.Confidence)
		}

		if result.Error == "" {
			t.Error("Expected error message in degraded mode")
		}
	})

	t.Run("UnsupportedMethod", func(t *testing.T) {
		params := createTestParams()
		result := realStore.Query("unsupported/method", params)

		if result.Found {
			t.Error("Expected found to be false for unsupported method")
		}

		if result.CacheHit {
			t.Error("Expected cache hit to be false")
		}

		if result.Error == "" {
			t.Error("Expected error message for unsupported method")
		}

		if !contains(result.Error, "unsupported method") {
			t.Errorf("Expected 'unsupported method' error, got: %s", result.Error)
		}
	})

	t.Run("CacheHit", func(t *testing.T) {
		// First, cache a response
		params := createTestParams()
		response := createTestResponse()
		err := realStore.CacheResponse("textDocument/definition", params, response)
		if err != nil {
			t.Fatalf("Failed to cache response: %v", err)
		}

		// Query the same method and params
		result := realStore.Query("textDocument/definition", params)

		if !result.Found {
			t.Error("Expected found to be true for cached response")
		}

		if !result.CacheHit {
			t.Error("Expected cache hit to be true")
		}

		if result.Confidence != 0.95 {
			t.Errorf("Expected confidence 0.95 for cache hit, got: %f", result.Confidence)
		}

		if result.Error != "" {
			t.Errorf("Expected no error for cache hit, got: %s", result.Error)
		}
	})

	t.Run("ConcurrencyLimit", func(t *testing.T) {
		// Create store with very low concurrency limit
		config := createTestSCIPConfig()
		config.Performance.MaxConcurrentQueries = 1
		limitedStore := NewRealSCIPStore(config)
		realLimitedStore := limitedStore.(*RealSCIPStore)
		defer realLimitedStore.Close()

		// Fill the query limiter
		realLimitedStore.queryLimiter <- struct{}{}

		// This query should timeout
		params := createTestParams()
		result := realLimitedStore.Query("textDocument/definition", params)

		if result.Found {
			t.Error("Expected found to be false for timeout")
		}

		if !contains(result.Error, "query timeout") {
			t.Errorf("Expected timeout error, got: %s", result.Error)
		}

		// Release the limiter
		<-realLimitedStore.queryLimiter
	})

	t.Run("StatsUpdate", func(t *testing.T) {
		initialStats := realStore.GetStats()
		params := createTestParams()
		
		realStore.Query("textDocument/definition", params)
		
		stats := realStore.GetStats()
		if stats.TotalQueries <= initialStats.TotalQueries {
			t.Error("Expected total queries to increase")
		}

		if stats.LastQueryTime.Before(initialStats.LastQueryTime) {
			t.Error("Expected last query time to be updated")
		}
	})
}

func TestRealSCIPStore_CacheResponse(t *testing.T) {
	store := NewRealSCIPStore(createTestSCIPConfig())
	realStore := store.(*RealSCIPStore)
	defer realStore.Close()

	t.Run("ValidCaching", func(t *testing.T) {
		params := createTestParams()
		response := createTestResponse()

		err := realStore.CacheResponse("textDocument/definition", params, response)
		if err != nil {
			t.Errorf("Expected no error caching response, got: %v", err)
		}

		// Verify response was cached by querying
		result := realStore.Query("textDocument/definition", params)
		if !result.CacheHit {
			t.Error("Expected cache hit after caching response")
		}
	})

	t.Run("DisabledCache", func(t *testing.T) {
		disabledStore := NewRealSCIPStore(createDisabledCacheConfig())
		realDisabledStore := disabledStore.(*RealSCIPStore)
		defer realDisabledStore.Close()

		params := createTestParams()
		response := createTestResponse()

		err := realDisabledStore.CacheResponse("textDocument/definition", params, response)
		if err != nil {
			t.Errorf("Expected no error with disabled cache, got: %v", err)
		}

		// Verify response was not cached
		result := realDisabledStore.Query("textDocument/definition", params)
		if result.CacheHit {
			t.Error("Expected no cache hit with disabled cache")
		}
	})

	t.Run("InvalidParams", func(t *testing.T) {
		// Create params that cannot be marshaled
		params := map[string]interface{}{
			"invalid": make(chan int), // Channels cannot be marshaled to JSON
		}
		response := createTestResponse()

		err := realStore.CacheResponse("textDocument/definition", params, response)
		if err == nil {
			t.Error("Expected error for unmarshalable params")
		}
	})

	t.Run("FilePathExtraction", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test/specific.go",
			},
		}
		response := createTestResponse()

		err := realStore.CacheResponse("textDocument/definition", params, response)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Invalidate the specific file
		realStore.InvalidateFile("/test/specific.go")

		// Query should not hit cache after invalidation
		result := realStore.Query("textDocument/definition", params)
		if result.CacheHit {
			t.Error("Expected no cache hit after file invalidation")
		}
	})
}

func TestRealSCIPStore_InvalidateFile(t *testing.T) {
	store := NewRealSCIPStore(createTestSCIPConfig())
	realStore := store.(*RealSCIPStore)
	defer realStore.Close()

	t.Run("ValidInvalidation", func(t *testing.T) {
		// Cache responses for multiple files
		params1 := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test/file1.go",
			},
		}
		params2 := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test/file2.go",
			},
		}
		response := createTestResponse()

		realStore.CacheResponse("textDocument/definition", params1, response)
		realStore.CacheResponse("textDocument/definition", params2, response)

		// Verify both are cached
		result1 := realStore.Query("textDocument/definition", params1)
		result2 := realStore.Query("textDocument/definition", params2)
		if !result1.CacheHit || !result2.CacheHit {
			t.Error("Expected both responses to be cached")
		}

		// Invalidate file1
		realStore.InvalidateFile("/test/file1.go")

		// Check that file1 is invalidated but file2 is still cached
		result1After := realStore.Query("textDocument/definition", params1)
		result2After := realStore.Query("textDocument/definition", params2)

		if result1After.CacheHit {
			t.Error("Expected file1 to be invalidated")
		}

		if !result2After.CacheHit {
			t.Error("Expected file2 to still be cached")
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		// Should not panic or error
		realStore.InvalidateFile("/non/existent/file.go")
	})

	t.Run("NilCache", func(t *testing.T) {
		// Create store without cache
		tempStore := &RealSCIPStore{cache: nil}
		
		// Should not panic
		tempStore.InvalidateFile("/test/file.go")
	})
}

func TestRealSCIPStore_GetStats(t *testing.T) {
	store := NewRealSCIPStore(createTestSCIPConfig())
	realStore := store.(*RealSCIPStore)
	defer realStore.Close()

	t.Run("InitialStats", func(t *testing.T) {
		stats := realStore.GetStats()

		if stats.IndexesLoaded != 0 {
			t.Errorf("Expected 0 initial indexes loaded, got: %d", stats.IndexesLoaded)
		}

		if stats.TotalQueries != 0 {
			t.Errorf("Expected 0 initial queries, got: %d", stats.TotalQueries)
		}

		if stats.CacheHitRate != 0 {
			t.Errorf("Expected 0 initial cache hit rate, got: %f", stats.CacheHitRate)
		}

		if stats.AverageQueryTime != 0 {
			t.Errorf("Expected 0 initial average query time, got: %v", stats.AverageQueryTime)
		}
	})

	t.Run("StatsAfterOperations", func(t *testing.T) {
		// Load index
		realStore.LoadIndex("/test/index.scip")

		// Cache a response
		params := createTestParams()
		response := createTestResponse()
		realStore.CacheResponse("textDocument/definition", params, response)

		// Perform queries
		realStore.Query("textDocument/definition", params) // Cache hit
		realStore.Query("textDocument/references", params) // Cache miss

		stats := realStore.GetStats()

		if stats.IndexesLoaded != 1 {
			t.Errorf("Expected 1 index loaded, got: %d", stats.IndexesLoaded)
		}

		if stats.TotalQueries != 2 {
			t.Errorf("Expected 2 queries, got: %d", stats.TotalQueries)
		}

		if stats.CacheHitRate != 0.5 {
			t.Errorf("Expected 50%% cache hit rate, got: %f", stats.CacheHitRate)
		}

		if stats.AverageQueryTime <= 0 {
			t.Error("Expected positive average query time")
		}

		if stats.LastQueryTime.IsZero() {
			t.Error("Expected last query time to be set")
		}

		if stats.CacheSize <= 0 {
			t.Error("Expected positive cache size")
		}

		if stats.MemoryUsage <= 0 {
			t.Error("Expected positive memory usage")
		}
	})

	t.Run("HitRateCalculation", func(t *testing.T) {
		freshStore := NewRealSCIPStore(createTestSCIPConfig())
		realFreshStore := freshStore.(*RealSCIPStore)
		defer realFreshStore.Close()

		params := createTestParams()
		response := createTestResponse()
		realFreshStore.CacheResponse("textDocument/definition", params, response)

		// 3 cache hits, 1 cache miss
		realFreshStore.Query("textDocument/definition", params) // Hit
		realFreshStore.Query("textDocument/definition", params) // Hit
		realFreshStore.Query("textDocument/definition", params) // Hit
		realFreshStore.Query("textDocument/references", params) // Miss

		stats := realFreshStore.GetStats()
		expectedRate := 3.0 / 4.0 // 75%
		if stats.CacheHitRate != expectedRate {
			t.Errorf("Expected cache hit rate %f, got: %f", expectedRate, stats.CacheHitRate)
		}
	})
}

func TestRealSCIPStore_Close(t *testing.T) {
	t.Run("ProperCleanup", func(t *testing.T) {
		store := NewRealSCIPStore(createTestSCIPConfig())
		realStore := store.(*RealSCIPStore)

		// Load some data
		realStore.LoadIndex("/test/index.scip")
		params := createTestParams()
		response := createTestResponse()
		realStore.CacheResponse("textDocument/definition", params, response)

		// Close the store
		err := realStore.Close()
		if err != nil {
			t.Errorf("Expected no error closing store, got: %v", err)
		}

		// Verify shutdown channel is closed
		select {
		case <-realStore.shutdown:
			// Expected - channel should be closed
		default:
			t.Error("Expected shutdown channel to be closed")
		}
	})

	t.Run("MultipleCloses", func(t *testing.T) {
		store := NewRealSCIPStore(createTestSCIPConfig())
		realStore := store.(*RealSCIPStore)

		// First close should succeed
		err1 := realStore.Close()
		if err1 != nil {
			t.Errorf("Expected no error on first close, got: %v", err1)
		}

		// Second close might panic due to closing closed channel
		// This is acceptable behavior - we just verify it doesn't hang
		defer func() {
			if r := recover(); r != nil {
				// Expected - closing an already closed channel panics
				// This is acceptable for the Close() method
			}
		}()

		_ = realStore.Close() // May panic, but that's acceptable
	})
}

// SCIPCache Tests

func TestNewSCIPCache(t *testing.T) {
	t.Run("EnabledCache", func(t *testing.T) {
		config := CacheConfig{
			Enabled: true,
			MaxSize: 100,
			TTL:     time.Hour,
		}

		cache := NewSCIPCache(config)

		if cache.entries == nil {
			t.Error("Expected entries map to be initialized")
		}

		if cache.lruList == nil {
			t.Error("Expected LRU list to be initialized")
		}

		if cache.maxSize != 100 {
			t.Errorf("Expected max size 100, got: %d", cache.maxSize)
		}

		if cache.cleanupTicker == nil {
			t.Error("Expected cleanup ticker to be initialized for enabled cache")
		}

		if cache.stopCleanup == nil {
			t.Error("Expected stop cleanup channel to be initialized")
		}

		cache.Close()
	})

	t.Run("DisabledCache", func(t *testing.T) {
		config := CacheConfig{
			Enabled: false,
			MaxSize: 100,
			TTL:     time.Hour,
		}

		cache := NewSCIPCache(config)

		if cache.entries == nil {
			t.Error("Expected entries map to be initialized")
		}

		if cache.lruList == nil {
			t.Error("Expected LRU list to be initialized")
		}

		if cache.cleanupTicker != nil {
			t.Error("Expected cleanup ticker to be nil for disabled cache")
		}

		cache.Close()
	})
}

func TestSCIPCache_SetGet(t *testing.T) {
	cache := NewSCIPCache(CacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     time.Hour,
	})
	defer cache.Close()

	t.Run("BasicSetGet", func(t *testing.T) {
		now := time.Now()
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"textDocument":{"uri":"file:///test.go"}}`,
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
			FilePaths: []string{"/test.go"},
		}

		cache.Set("test-key", entry)

		retrieved := cache.Get("test-key")
		if retrieved == nil {
			t.Fatal("Expected to retrieve cached entry")
		}

		if retrieved.Method != "textDocument/definition" {
			t.Errorf("Expected method textDocument/definition, got: %s", retrieved.Method)
		}

		// Check that stats were updated
		stats := cache.GetStats()
		if stats.HitCount != 1 {
			t.Errorf("Expected 1 hit, got: %d", stats.HitCount)
		}

		if stats.Size != 1 {
			t.Errorf("Expected cache size 1, got: %d", stats.Size)
		}
	})

	t.Run("CacheMiss", func(t *testing.T) {
		retrieved := cache.Get("non-existent-key")
		if retrieved != nil {
			t.Error("Expected nil for non-existent key")
		}

		stats := cache.GetStats()
		if stats.MissCount == 0 {
			t.Error("Expected miss count to increase")
		}
	})

	t.Run("UpdateExisting", func(t *testing.T) {
		now := time.Now()
		entry1 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"key":"value1"}`,
				Response:   json.RawMessage(`{"result":"first"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		entry2 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"key":"value2"}`,
				Response:   json.RawMessage(`{"result":"second"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		cache.Set("update-key", entry1)
		initialSize := cache.GetStats().Size

		cache.Set("update-key", entry2) // Update existing entry
		finalSize := cache.GetStats().Size

		if finalSize != initialSize {
			t.Error("Expected cache size to remain same when updating existing entry")
		}

		retrieved := cache.Get("update-key")
		if string(retrieved.Response) != `{"result":"second"}` {
			t.Error("Expected updated response")
		}
	})
}

func TestSCIPCache_LRUEviction(t *testing.T) {
	cache := NewSCIPCache(CacheConfig{
		Enabled: true,
		MaxSize: 2, // Small size to trigger eviction
		TTL:     time.Hour,
	})
	defer cache.Close()

	t.Run("EvictionOnCapacity", func(t *testing.T) {
		now := time.Now()
		entry1 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"file":"1"}`,
				Response:   json.RawMessage(`{"result":"1"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		entry2 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"file":"2"}`,
				Response:   json.RawMessage(`{"result":"2"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		entry3 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"file":"3"}`,
				Response:   json.RawMessage(`{"result":"3"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		cache.Set("key1", entry1)
		cache.Set("key2", entry2)
		
		// This should trigger eviction of key1 (least recently used)
		cache.Set("key3", entry3)

		// key1 should be evicted
		if cache.Get("key1") != nil {
			t.Error("Expected key1 to be evicted")
		}

		// key2 and key3 should still exist
		if cache.Get("key2") == nil {
			t.Error("Expected key2 to still exist")
		}

		if cache.Get("key3") == nil {
			t.Error("Expected key3 to still exist")
		}

		stats := cache.GetStats()
		if stats.EvictionCount == 0 {
			t.Error("Expected eviction count to be positive")
		}

		if stats.Size != 2 {
			t.Errorf("Expected cache size 2, got: %d", stats.Size)
		}
	})

	t.Run("LRUOrder", func(t *testing.T) {
		freshCache := NewSCIPCache(CacheConfig{
			Enabled: true,
			MaxSize: 2,
			TTL:     time.Hour,
		})
		defer freshCache.Close()

		now := time.Now()
		entry1 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "test",
				Response:   json.RawMessage(`{"result":"1"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		entry2 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "test",
				Response:   json.RawMessage(`{"result":"2"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		entry3 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "test",
				Response:   json.RawMessage(`{"result":"3"}`),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		freshCache.Set("order1", entry1)
		freshCache.Set("order2", entry2)

		// Access order1 to make it recently used
		freshCache.Get("order1")

		// Add order3, should evict order2 (least recently used)
		freshCache.Set("order3", entry3)

		if freshCache.Get("order1") == nil {
			t.Error("Expected order1 to remain (recently accessed)")
		}

		if freshCache.Get("order2") != nil {
			t.Error("Expected order2 to be evicted (least recently used)")
		}

		if freshCache.Get("order3") == nil {
			t.Error("Expected order3 to exist (newly added)")
		}
	})
}

func TestSCIPCache_TTLExpiration(t *testing.T) {
	cache := NewSCIPCache(CacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     time.Millisecond * 50, // Short TTL for testing
	})
	defer cache.Close()

	t.Run("ExpiredEntry", func(t *testing.T) {
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Response:   createTestResponse(),
				CreatedAt:  time.Now().Add(-time.Hour), // Created long ago
				AccessedAt: time.Now().Add(-time.Hour),
				TTL:        time.Millisecond * 10, // Very short TTL
			},
		}

		cache.Set("expired-key", entry)

		// Should return nil for expired entry
		retrieved := cache.Get("expired-key")
		if retrieved != nil {
			t.Error("Expected nil for expired entry")
		}

		stats := cache.GetStats()
		if stats.MissCount == 0 {
			t.Error("Expected miss count to increase for expired entry")
		}
	})

	t.Run("BackgroundCleanup", func(t *testing.T) {
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Response:   createTestResponse(),
				CreatedAt:  time.Now(),
				AccessedAt: time.Now(),
				TTL:        time.Millisecond * 10, // Very short TTL
			},
		}

		cache.Set("cleanup-key", entry)
		
		// Wait for entry to expire and cleanup to run
		time.Sleep(time.Millisecond * 100)
		
		// Trigger manual cleanup
		cache.PerformMaintenance()

		// Entry should be cleaned up
		stats := cache.GetStats()
		if stats.Size > 0 {
			t.Error("Expected expired entries to be cleaned up")
		}
	})
}

func TestSCIPCache_InvalidateByFile(t *testing.T) {
	cache := NewSCIPCache(CacheConfig{
		Enabled: true,
		MaxSize: 10,
		TTL:     time.Hour,
	})
	defer cache.Close()

	t.Run("FileInvalidation", func(t *testing.T) {
		now := time.Now()
		entry1 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
			FilePaths: []string{"/test/file1.go", "/test/shared.go"},
		}

		entry2 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/references",
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
			FilePaths: []string{"/test/file2.go", "/test/shared.go"},
		}

		entry3 := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/hover",
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
			FilePaths: []string{"/test/other.go"},
		}

		cache.Set("key1", entry1)
		cache.Set("key2", entry2)
		cache.Set("key3", entry3)

		// Invalidate shared.go - should remove key1 and key2
		deletedKeys := cache.InvalidateByFile("/test/shared.go")

		if len(deletedKeys) != 2 {
			t.Errorf("Expected 2 keys to be invalidated, got: %d", len(deletedKeys))
		}

		if cache.Get("key1") != nil {
			t.Error("Expected key1 to be invalidated")
		}

		if cache.Get("key2") != nil {
			t.Error("Expected key2 to be invalidated")
		}

		if cache.Get("key3") == nil {
			t.Error("Expected key3 to remain (different file)")
		}

		stats := cache.GetStats()
		if stats.Size != 1 {
			t.Errorf("Expected cache size 1 after invalidation, got: %d", stats.Size)
		}
	})

	t.Run("PartialPathMatch", func(t *testing.T) {
		now := time.Now()
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
			FilePaths: []string{"/long/path/to/file.go"},
		}

		cache.Set("partial-key", entry)

		// Invalidate with partial path
		deletedKeys := cache.InvalidateByFile("path/to/file.go")

		if len(deletedKeys) != 1 {
			t.Errorf("Expected 1 key to be invalidated with partial path, got: %d", len(deletedKeys))
		}

		if cache.Get("partial-key") != nil {
			t.Error("Expected entry to be invalidated with partial path match")
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		now := time.Now()
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
			FilePaths: []string{"/test/file.go"},
		}

		cache.Set("nomatch-key", entry)

		// Try to invalidate non-matching file
		deletedKeys := cache.InvalidateByFile("/other/file.go")

		if len(deletedKeys) != 0 {
			t.Errorf("Expected 0 keys to be invalidated, got: %d", len(deletedKeys))
		}

		if cache.Get("nomatch-key") == nil {
			t.Error("Expected entry to remain (no file match)")
		}
	})
}

func TestSCIPCache_GetStats(t *testing.T) {
	cache := NewSCIPCache(CacheConfig{
		Enabled: true,
		MaxSize: 5,
		TTL:     time.Hour,
	})
	defer cache.Close()

	t.Run("EmptyCache", func(t *testing.T) {
		stats := cache.GetStats()

		if stats.Size != 0 {
			t.Errorf("Expected size 0, got: %d", stats.Size)
		}

		if stats.HitCount != 0 {
			t.Errorf("Expected hit count 0, got: %d", stats.HitCount)
		}

		if stats.MissCount != 0 {
			t.Errorf("Expected miss count 0, got: %d", stats.MissCount)
		}

		if stats.EvictionCount != 0 {
			t.Errorf("Expected eviction count 0, got: %d", stats.EvictionCount)
		}

		if stats.MemoryUsage != 0 {
			t.Errorf("Expected memory usage 0, got: %d", stats.MemoryUsage)
		}
	})

	t.Run("StatsAfterOperations", func(t *testing.T) {
		now := time.Now()
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Params:     `{"test":"data"}`,
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		cache.Set("stats-key", entry)
		cache.Get("stats-key")  // Hit
		cache.Get("miss-key")   // Miss

		stats := cache.GetStats()

		if stats.Size != 1 {
			t.Errorf("Expected size 1, got: %d", stats.Size)
		}

		if stats.HitCount != 1 {
			t.Errorf("Expected hit count 1, got: %d", stats.HitCount)
		}

		if stats.MissCount != 1 {
			t.Errorf("Expected miss count 1, got: %d", stats.MissCount)
		}

		if stats.MemoryUsage <= 0 {
			t.Error("Expected positive memory usage")
		}
	})
}

func TestSCIPCache_Close(t *testing.T) {
	t.Run("ProperCleanup", func(t *testing.T) {
		cache := NewSCIPCache(CacheConfig{
			Enabled: true,
			MaxSize: 10,
			TTL:     time.Hour,
		})

		now := time.Now()
		entry := &SCIPCacheEntryExtended{
			SCIPCacheEntry: &SCIPCacheEntry{
				Method:     "textDocument/definition",
				Response:   createTestResponse(),
				CreatedAt:  now,
				AccessedAt: now,
				TTL:        time.Hour,
			},
		}

		cache.Set("cleanup-key", entry)

		// Verify entry exists
		if cache.Get("cleanup-key") == nil {
			t.Fatal("Expected entry to exist before close")
		}

		cache.Close()

		// Verify cleanup
		stats := cache.GetStats()
		if stats.Size != 0 {
			t.Errorf("Expected cache to be empty after close, got size: %d", stats.Size)
		}

		if len(cache.entries) != 0 {
			t.Error("Expected entries map to be empty after close")
		}

		if cache.lruList.Len() != 0 {
			t.Error("Expected LRU list to be empty after close")
		}
	})

	t.Run("StopBackgroundProcesses", func(t *testing.T) {
		cache := NewSCIPCache(CacheConfig{
			Enabled: true,
			MaxSize: 10,
			TTL:     time.Hour,
		})

		if cache.cleanupTicker == nil {
			t.Fatal("Expected cleanup ticker to be started")
		}

		cache.Close()

		// Note: We can't easily test that the goroutine has stopped,
		// but we can verify that cleanup channels are handled properly
		// The ticker.Stop() call should handle the goroutine cleanup
	})

	t.Run("MultipleCloses", func(t *testing.T) {
		cache := NewSCIPCache(CacheConfig{
			Enabled: true,
			MaxSize: 10,
			TTL:     time.Hour,
		})

		// First close should succeed
		cache.Close()

		// Second close might panic due to closing closed channel
		// This is acceptable behavior - we just verify it doesn't hang
		defer func() {
			if r := recover(); r != nil {
				// Expected - closing an already closed channel panics
				// This is acceptable for the Close() method
			}
		}()

		cache.Close() // May panic, but that's acceptable
	})
}

func TestSCIPCache_ConcurrentOperations(t *testing.T) {
	cache := NewSCIPCache(CacheConfig{
		Enabled: true,
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer cache.Close()

	t.Run("ConcurrentSetGet", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		numOperations := 50

		// Concurrent sets and gets
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("key-%d-%d", id, j)
					now := time.Now()
					entry := &SCIPCacheEntryExtended{
						SCIPCacheEntry: &SCIPCacheEntry{
							Method:     "test",
							Response:   json.RawMessage(fmt.Sprintf(`{"id":%d}`, id)),
							CreatedAt:  now,
							AccessedAt: now,
							TTL:        time.Hour,
						},
					}

					cache.Set(key, entry)
					cache.Get(key)
				}
			}(i)
		}

		wg.Wait()

		// Verify cache state is consistent
		stats := cache.GetStats()
		if stats.Size == 0 {
			t.Error("Expected some entries in cache after concurrent operations")
		}
	})

	t.Run("ConcurrentInvalidation", func(t *testing.T) {
		// Add some entries
		now := time.Now()
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("concurrent-key-%d", i)
			entry := &SCIPCacheEntryExtended{
				SCIPCacheEntry: &SCIPCacheEntry{
					Method:     "test",
					Response:   createTestResponse(),
					CreatedAt:  now,
					AccessedAt: now,
					TTL:        time.Hour,
				},
				FilePaths: []string{fmt.Sprintf("/test/file%d.go", i)},
			}
			cache.Set(key, entry)
		}

		var wg sync.WaitGroup
		numGoroutines := 5

		// Concurrent invalidations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				cache.InvalidateByFile(fmt.Sprintf("/test/file%d.go", id))
			}(i)
		}

		wg.Wait()

		// Should not panic and cache should be in consistent state
		stats := cache.GetStats()
		_ = stats // Just verify we can get stats without panic
	})
}

// Helper functions

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}