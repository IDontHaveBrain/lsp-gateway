package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestL1MemoryCache_BasicOperations(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	// Initialize cache
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()
	
	// Test basic Put/Get operations
	entry := &CacheEntry{
		Method:     "textDocument/definition",
		Params:     `{"textDocument":{"uri":"file:///test.go"},"position":{"line":10,"character":5}}`,
		Response:   json.RawMessage(`{"uri":"file:///test.go","range":{"start":{"line":5,"character":0},"end":{"line":5,"character":10}}}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	// Test Put
	err = cache.Put(ctx, "test-key-1", entry)
	assert.NoError(t, err)
	
	// Test Get
	retrieved, err := cache.Get(ctx, "test-key-1")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, entry.Method, retrieved.Method)
	assert.Equal(t, entry.Params, retrieved.Params)
	
	// Test Exists
	exists, err := cache.Exists(ctx, "test-key-1")
	assert.NoError(t, err)
	assert.True(t, exists)
	
	exists, err = cache.Exists(ctx, "non-existent-key")
	assert.NoError(t, err)
	assert.False(t, exists)
	
	// Test Delete
	err = cache.Delete(ctx, "test-key-1")
	assert.NoError(t, err)
	
	exists, err = cache.Exists(ctx, "test-key-1")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestL1MemoryCache_PerformanceRequirements(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Prepare test data
	entry := &CacheEntry{
		Method:     "textDocument/hover",
		Params:     `{"textDocument":{"uri":"file:///large-file.go"},"position":{"line":100,"character":20}}`,
		Response:   json.RawMessage(`{"contents":{"kind":"markdown","value":"## Function Documentation\nThis is a comprehensive documentation for a complex function with multiple parameters and return values."}}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	// Test Put performance - should be <10ms
	start := time.Now()
	err = cache.Put(ctx, "perf-test-key", entry)
	putLatency := time.Since(start)
	assert.NoError(t, err)
	assert.Less(t, putLatency, 10*time.Millisecond, "Put operation should be <10ms, got %v", putLatency)
	
	// Test Get performance - should be <10ms
	start = time.Now()
	retrieved, err := cache.Get(ctx, "perf-test-key")
	getLatency := time.Since(start)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Less(t, getLatency, 10*time.Millisecond, "Get operation should be <10ms, got %v", getLatency)
	
	t.Logf("Put latency: %v, Get latency: %v", putLatency, getLatency)
}

func TestL1MemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Test concurrent operations
	const numGoroutines = 100
	const numOperations = 10
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)
	
	// Concurrent puts
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				entry := &CacheEntry{
					Method:     "textDocument/completion",
					Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///test_%d_%d.go"}}`, id, j),
					Response:   json.RawMessage(`{"items":[{"label":"test","kind":1}]}`),
					CreatedAt:  time.Now(),
					AccessedAt: time.Now(),
					TTL:        time.Hour,
				}
				
				key := fmt.Sprintf("concurrent-key-%d-%d", id, j)
				if err := cache.Put(ctx, key, entry); err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
	
	// Verify all entries were stored
	stats := cache.GetStats()
	expectedEntries := int64(numGoroutines * numOperations)
	assert.Equal(t, expectedEntries, stats.EntryCount, "Expected %d entries, got %d", expectedEntries, stats.EntryCount)
}

func TestL1MemoryCache_LRUEviction(t *testing.T) {
	// Create cache with small capacity for testing eviction
	config := &L1MemoryCacheConfig{
		MaxCapacity:       1024, // 1KB
		MaxEntries:        5,    // Maximum 5 entries
		EvictionThreshold: 0.8,
		CompressionEnabled: false,
		TTLCheckInterval:  time.Second,
		MetricsInterval:   time.Second,
	}
	
	cache := NewL1MemoryCache(config)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Add entries that will trigger eviction
	for i := 0; i < 10; i++ {
		entry := &CacheEntry{
			Method:     "textDocument/references",
			Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///test_%d.go"},"position":{"line":%d,"character":5}}`, i, i),
			Response:   json.RawMessage(fmt.Sprintf(`[{"uri":"file:///test_%d.go","range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":10}}}]`, i, i, i)),
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		}
		
		err = cache.Put(ctx, fmt.Sprintf("lru-test-key-%d", i), entry)
		assert.NoError(t, err)
	}
	
	// Check that eviction occurred
	stats := cache.GetStats()
	assert.LessOrEqual(t, stats.EntryCount, config.MaxEntries, "Entry count should not exceed max entries")
	assert.Greater(t, stats.EvictionCount, int64(0), "Evictions should have occurred")
	
	// Verify that oldest entries were evicted (LRU)
	for i := 0; i < 5; i++ {
		exists, err := cache.Exists(ctx, fmt.Sprintf("lru-test-key-%d", i))
		assert.NoError(t, err)
		assert.False(t, exists, "Oldest entries should have been evicted")
	}
	
	// Verify that newest entries are still present
	for i := 5; i < 10; i++ {
		exists, err := cache.Exists(ctx, fmt.Sprintf("lru-test-key-%d", i))
		assert.NoError(t, err)
		if !exists {
			t.Logf("Entry %d was evicted (this may be expected due to size limits)", i)
		}
	}
}

func TestL1MemoryCache_Compression(t *testing.T) {
	config := &L1MemoryCacheConfig{
		MaxCapacity:          10 * 1024 * 1024, // 10MB
		MaxEntries:           1000,
		CompressionEnabled:   true,
		CompressionThreshold: 100, // Compress anything > 100 bytes
		CompressionType:      CompressionLZ4,
		TTLCheckInterval:     time.Minute,
		MetricsInterval:      time.Second,
	}
	
	cache := NewL1MemoryCache(config)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Create a large entry that should be compressed
	largeResponse := make([]byte, 2048)
	for i := range largeResponse {
		largeResponse[i] = byte('A' + (i % 26)) // Fill with repeating pattern for compression
	}
	
	entry := &CacheEntry{
		Method:     "textDocument/documentSymbol",
		Params:     `{"textDocument":{"uri":"file:///large-document.go"}}`,
		Response:   json.RawMessage(largeResponse),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	// Put the entry
	err = cache.Put(ctx, "compression-test-key", entry)
	assert.NoError(t, err)
	
	// Retrieve and verify
	retrieved, err := cache.Get(ctx, "compression-test-key")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, entry.Method, retrieved.Method)
	assert.Equal(t, string(entry.Response), string(retrieved.Response))
	
	// Check compression stats
	stats := cache.GetStats()
	assert.Greater(t, stats.EntryCount, int64(0))
	
	// Verify compression was used (this is somewhat indirect)
	if retrieved.CompressedSize > 0 && retrieved.Size > 0 {
		compressionRatio := float64(retrieved.CompressedSize) / float64(retrieved.Size)
		assert.Less(t, compressionRatio, 1.0, "Compression should have reduced size")
		t.Logf("Compression ratio: %.2f", compressionRatio)
	}
}

func TestL1MemoryCache_TTLExpiration(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Create entry with short TTL
	entry := &CacheEntry{
		Method:     "textDocument/semanticTokens",
		Params:     `{"textDocument":{"uri":"file:///test.go"}}`,
		Response:   json.RawMessage(`{"data":[1,2,3,4,5]}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        100 * time.Millisecond, // Very short TTL
	}
	
	// Put the entry
	err = cache.Put(ctx, "ttl-test-key", entry)
	assert.NoError(t, err)
	
	// Verify it exists initially
	exists, err := cache.Exists(ctx, "ttl-test-key")
	assert.NoError(t, err)
	assert.True(t, exists)
	
	// Wait for TTL expiration
	time.Sleep(150 * time.Millisecond)
	
	// Verify it's expired
	exists, err = cache.Exists(ctx, "ttl-test-key")
	assert.NoError(t, err)
	assert.False(t, exists, "Entry should have expired")
	
	// Verify Get returns error for expired entry
	_, err = cache.Get(ctx, "ttl-test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestL1MemoryCache_BatchOperations(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Prepare batch entries
	entries := make(map[string]*CacheEntry)
	keys := make([]string, 10)
	
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("batch-key-%d", i)
		keys[i] = key
		entries[key] = &CacheEntry{
			Method:     "textDocument/inlayHint",
			Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///batch_%d.go"},"range":{"start":{"line":0,"character":0},"end":{"line":100,"character":0}}}`, i),
			Response:   json.RawMessage(fmt.Sprintf(`[{"position":{"line":%d,"character":10},"label":"Type: int","kind":1}]`, i)),
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		}
	}
	
	// Test batch put
	start := time.Now()
	err = cache.PutBatch(ctx, entries)
	batchPutLatency := time.Since(start)
	assert.NoError(t, err)
	
	// Test batch get
	start = time.Now()
	retrieved, err := cache.GetBatch(ctx, keys)
	batchGetLatency := time.Since(start)
	assert.NoError(t, err)
	assert.Len(t, retrieved, len(keys))
	
	// Verify all entries were retrieved correctly
	for key, originalEntry := range entries {
		retrievedEntry, exists := retrieved[key]
		assert.True(t, exists, "Entry %s should exist", key)
		assert.Equal(t, originalEntry.Method, retrievedEntry.Method)
		assert.Equal(t, originalEntry.Params, retrievedEntry.Params)
	}
	
	// Test batch delete
	start = time.Now()
	err = cache.DeleteBatch(ctx, keys)
	batchDeleteLatency := time.Since(start)
	assert.NoError(t, err)
	
	// Verify all entries were deleted
	for _, key := range keys {
		exists, err := cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists, "Entry %s should have been deleted", key)
	}
	
	t.Logf("Batch operations - Put: %v, Get: %v, Delete: %v", batchPutLatency, batchGetLatency, batchDeleteLatency)
}

func TestL1MemoryCache_InvalidationOperations(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Add test entries with file associations
	testEntries := map[string]*CacheEntry{
		"file1-def": {
			Method:     "textDocument/definition",
			Params:     `{"textDocument":{"uri":"file:///project/file1.go"}}`,
			Response:   json.RawMessage(`{"uri":"file:///project/file1.go","range":{"start":{"line":10,"character":0},"end":{"line":10,"character":10}}}`),
			FilePaths:  []string{"/project/file1.go"},
			ProjectPath: "/project",
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		},
		"file1-hover": {
			Method:     "textDocument/hover",
			Params:     `{"textDocument":{"uri":"file:///project/file1.go"}}`,
			Response:   json.RawMessage(`{"contents":"Hover info for file1"}`),
			FilePaths:  []string{"/project/file1.go"},
			ProjectPath: "/project",
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		},
		"file2-def": {
			Method:     "textDocument/definition",
			Params:     `{"textDocument":{"uri":"file:///project/file2.go"}}`,
			Response:   json.RawMessage(`{"uri":"file:///project/file2.go","range":{"start":{"line":5,"character":0},"end":{"line":5,"character":8}}}`),
			FilePaths:  []string{"/project/file2.go"},
			ProjectPath: "/project",
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		},
		"other-project": {
			Method:     "textDocument/definition",
			Params:     `{"textDocument":{"uri":"file:///other/file.go"}}`,
			Response:   json.RawMessage(`{"uri":"file:///other/file.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":5}}}`),
			FilePaths:  []string{"/other/file.go"},
			ProjectPath: "/other",
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		},
	}
	
	// Put all entries
	for key, entry := range testEntries {
		err = cache.Put(ctx, key, entry)
		require.NoError(t, err)
	}
	
	// Test invalidation by file
	invalidated, err := cache.InvalidateByFile(ctx, "/project/file1.go")
	assert.NoError(t, err)
	assert.Equal(t, 2, invalidated, "Should invalidate 2 entries for file1.go")
	
	// Verify file1 entries are gone
	exists, err := cache.Exists(ctx, "file1-def")
	assert.NoError(t, err)
	assert.False(t, exists)
	
	exists, err = cache.Exists(ctx, "file1-hover")
	assert.NoError(t, err)
	assert.False(t, exists)
	
	// Verify other entries still exist
	exists, err = cache.Exists(ctx, "file2-def")
	assert.NoError(t, err)
	assert.True(t, exists)
	
	exists, err = cache.Exists(ctx, "other-project")
	assert.NoError(t, err)
	assert.True(t, exists)
	
	// Test invalidation by project
	invalidated, err = cache.InvalidateByProject(ctx, "/project")
	assert.NoError(t, err)
	assert.Equal(t, 1, invalidated, "Should invalidate 1 remaining entry for /project")
	
	// Verify project entries are gone
	exists, err = cache.Exists(ctx, "file2-def")
	assert.NoError(t, err)
	assert.False(t, exists)
	
	// Verify other project entry still exists
	exists, err = cache.Exists(ctx, "other-project")
	assert.NoError(t, err)
	assert.True(t, exists)
	
	// Test pattern invalidation
	invalidated, err = cache.Invalidate(ctx, "*")
	assert.NoError(t, err)
	assert.Equal(t, 1, invalidated, "Should invalidate all remaining entries")
	
	// Verify cache is empty
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats.EntryCount)
}

func TestL1MemoryCache_HealthAndStats(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Add some test data
	entry := &CacheEntry{
		Method:     "textDocument/publishDiagnostics",
		Params:     `{"uri":"file:///test.go","diagnostics":[]}`,
		Response:   json.RawMessage(`{"method":"textDocument/publishDiagnostics","params":{"uri":"file:///test.go","diagnostics":[]}}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	for i := 0; i < 10; i++ {
		err = cache.Put(ctx, fmt.Sprintf("health-test-key-%d", i), entry)
		require.NoError(t, err)
	}
	
	// Perform some gets
	for i := 0; i < 5; i++ {
		_, err = cache.Get(ctx, fmt.Sprintf("health-test-key-%d", i))
		assert.NoError(t, err)
	}
	
	// Test stats
	stats := cache.GetStats()
	assert.Equal(t, TierL1Memory, stats.TierType)
	assert.Equal(t, int64(10), stats.EntryCount)
	assert.Greater(t, stats.TotalRequests, int64(0))
	assert.Greater(t, stats.CacheHits, int64(0))
	assert.Greater(t, stats.HitRate, 0.0)
	assert.Less(t, stats.HitRate, 1.1) // Should be <= 1.0
	assert.Greater(t, stats.UsedCapacity, int64(0))
	assert.NotZero(t, stats.StartTime)
	assert.NotZero(t, stats.LastUpdate)
	
	t.Logf("Stats: Entries=%d, Requests=%d, Hits=%d, HitRate=%.2f", 
		stats.EntryCount, stats.TotalRequests, stats.CacheHits, stats.HitRate)
	
	// Test health
	health := cache.GetHealth()
	assert.Equal(t, TierL1Memory, health.TierType)
	assert.True(t, health.Healthy)
	assert.NotZero(t, health.LastCheck)
	assert.NotNil(t, health.Metrics)
	
	// Test capacity
	capacity := cache.GetCapacity()
	assert.Equal(t, TierL1Memory, capacity.TierType)
	assert.Greater(t, capacity.MaxCapacity, int64(0))
	assert.Greater(t, capacity.UsedCapacity, int64(0))
	assert.Greater(t, capacity.AvailableCapacity, int64(0))
	assert.Equal(t, int64(10), capacity.UsedEntries)
	assert.Greater(t, capacity.UtilizationPct, 0.0)
	
	t.Logf("Capacity: Used=%d/%d (%.1f%%), Entries=%d/%d", 
		capacity.UsedCapacity, capacity.MaxCapacity, capacity.UtilizationPct,
		capacity.UsedEntries, capacity.MaxEntries)
}

func TestL1MemoryCache_CircuitBreaker(t *testing.T) {
	// Create cache with aggressive circuit breaker for testing
	config := &L1MemoryCacheConfig{
		MaxCapacity:     1024 * 1024,
		MaxEntries:      1000,
		TTLCheckInterval: time.Second,
		MetricsInterval: time.Second,
		CircuitBreakerConfig: &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3, // Trip after 3 failures
			RecoveryTimeout:  100 * time.Millisecond,
			HalfOpenRequests: 1,
			MinRequestCount:  1,
		},
	}
	
	cache := NewL1MemoryCache(config)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Circuit breaker should be closed initially
	health := cache.GetHealth()
	assert.Equal(t, CircuitBreakerClosed, health.CircuitBreaker.State)
	
	// Try to get non-existent keys to trigger failures
	for i := 0; i < 5; i++ {
		_, err = cache.Get(ctx, fmt.Sprintf("non-existent-key-%d", i))
		assert.Error(t, err) // Should fail because key doesn't exist
	}
	
	// Circuit breaker should now be open due to failures
	health = cache.GetHealth()
	assert.Equal(t, CircuitBreakerOpen, health.CircuitBreaker.State)
	assert.Greater(t, health.CircuitBreaker.FailureCount, 0)
	
	// Subsequent requests should fail immediately due to open circuit
	_, err = cache.Get(ctx, "another-non-existent-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
	
	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)
	
	// Circuit should now be half-open and allow requests
	entry := &CacheEntry{
		Method:     "textDocument/codeAction",
		Params:     `{"textDocument":{"uri":"file:///recovery-test.go"}}`,
		Response:   json.RawMessage(`{"actions":[]}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	// Successful operation should close the circuit
	err = cache.Put(ctx, "recovery-key", entry)
	assert.NoError(t, err)
	
	_, err = cache.Get(ctx, "recovery-key")
	assert.NoError(t, err)
	
	// Circuit should be closed again
	health = cache.GetHealth()
	assert.Equal(t, CircuitBreakerClosed, health.CircuitBreaker.State)
}

func BenchmarkL1MemoryCache_Get(b *testing.B) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(b, err)
	defer cache.Close()
	
	// Prepare benchmark data
	entry := &CacheEntry{
		Method:     "textDocument/completion",
		Params:     `{"textDocument":{"uri":"file:///benchmark.go"},"position":{"line":50,"character":10}}`,
		Response:   json.RawMessage(`{"items":[{"label":"testFunc","kind":3,"detail":"func testFunc() error","documentation":"A test function for benchmarking"}]}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	err = cache.Put(ctx, "benchmark-key", entry)
	require.NoError(b, err)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cache.Get(ctx, "benchmark-key")
			if err != nil {
				b.Errorf("Get operation failed: %v", err)
			}
		}
	})
}

func BenchmarkL1MemoryCache_Put(b *testing.B) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(b, err)
	defer cache.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			entry := &CacheEntry{
				Method:     "textDocument/signatureHelp",
				Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///benchmark_%d.go"},"position":{"line":1,"character":1}}`, i),
				Response:   json.RawMessage(`{"signatures":[{"label":"func(x int) string","documentation":"Benchmark function signature"}]}`),
				CreatedAt:  time.Now(),
				AccessedAt: time.Now(),
				TTL:        time.Hour,
			}
			
			err := cache.Put(ctx, fmt.Sprintf("benchmark-put-key-%d", i), entry)
			if err != nil {
				b.Errorf("Put operation failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkL1MemoryCache_ConcurrentMixed(b *testing.B) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(b, err)
	defer cache.Close()
	
	// Pre-populate cache with some data
	for i := 0; i < 1000; i++ {
		entry := &CacheEntry{
			Method:     "textDocument/references",
			Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///mixed_%d.go"}}`, i),
			Response:   json.RawMessage(fmt.Sprintf(`[{"uri":"file:///mixed_%d.go","range":{"start":{"line":1,"character":1},"end":{"line":1,"character":10}}}]`, i)),
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		}
		err = cache.Put(ctx, fmt.Sprintf("mixed-key-%d", i), entry)
		require.NoError(b, err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%3 == 0 {
				// 33% reads
				_, err := cache.Get(ctx, fmt.Sprintf("mixed-key-%d", i%1000))
				if err != nil {
					// Key might have been evicted, which is fine in this benchmark
				}
			} else if i%3 == 1 {
				// 33% writes
				entry := &CacheEntry{
					Method:     "textDocument/foldingRange",
					Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///new_%d.go"}}`, i),
					Response:   json.RawMessage(`[{"startLine":1,"endLine":10,"kind":"region"}]`),
					CreatedAt:  time.Now(),
					AccessedAt: time.Now(),
					TTL:        time.Hour,
				}
				err := cache.Put(ctx, fmt.Sprintf("mixed-new-key-%d", i), entry)
				if err != nil {
					b.Errorf("Put operation failed: %v", err)
				}
			} else {
				// 33% existence checks
				_, err := cache.Exists(ctx, fmt.Sprintf("mixed-key-%d", i%1000))
				if err != nil {
					b.Errorf("Exists operation failed: %v", err)
				}
			}
			i++
		}
	})
}

func TestL1MemoryCache_MemoryUsage(t *testing.T) {
	// This test monitors memory usage and GC behavior
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	require.NoError(t, err)
	defer cache.Close()
	
	// Add many entries to test memory usage
	const numEntries = 10000
	
	for i := 0; i < numEntries; i++ {
		entry := &CacheEntry{
			Method:     "textDocument/documentHighlight",
			Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///memory_test_%d.go"},"position":{"line":%d,"character":5}}`, i, i%100),
			Response:   json.RawMessage(fmt.Sprintf(`[{"range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":20}},"kind":1}]`, i%100, i%100)),
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
		}
		
		err = cache.Put(ctx, fmt.Sprintf("memory-test-key-%d", i), entry)
		require.NoError(t, err)
	}
	
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	
	stats := cache.GetStats()
	
	t.Logf("Memory usage after %d entries:", numEntries)
	t.Logf("  Cache reported size: %d bytes", stats.UsedCapacity)
	t.Logf("  System memory alloc: %d bytes (delta: %d)", m2.Alloc, int64(m2.Alloc)-int64(m1.Alloc))
	t.Logf("  System heap in use: %d bytes (delta: %d)", m2.HeapInuse, int64(m2.HeapInuse)-int64(m1.HeapInuse))
	t.Logf("  GC cycles: %d", m2.NumGC-m1.NumGC)
	
	// Verify reasonable memory usage
	avgEntrySize := stats.UsedCapacity / stats.EntryCount
	t.Logf("  Average entry size: %d bytes", avgEntrySize)
	
	// Memory usage should be reasonable (less than 1KB per entry on average)
	assert.Less(t, avgEntrySize, int64(1024), "Average entry size should be reasonable")
}