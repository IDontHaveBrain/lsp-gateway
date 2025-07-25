package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestL1MemoryCache_BasicFunctionality(t *testing.T) {
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
	if err != nil {
		t.Errorf("Put operation failed: %v", err)
	}
	
	// Test Get
	retrieved, err := cache.Get(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Get operation failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved entry is nil")
	}
	if retrieved.Method != entry.Method {
		t.Errorf("Expected method %s, got %s", entry.Method, retrieved.Method)
	}
	if retrieved.Params != entry.Params {
		t.Errorf("Expected params %s, got %s", entry.Params, retrieved.Params)
	}
	
	// Test Exists
	exists, err := cache.Exists(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Exists operation failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist")
	}
	
	exists, err = cache.Exists(ctx, "non-existent-key")
	if err != nil {
		t.Errorf("Exists operation failed: %v", err)
	}
	if exists {
		t.Error("Non-existent key should not exist")
	}
	
	// Test Delete
	err = cache.Delete(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Delete operation failed: %v", err)
	}
	
	exists, err = cache.Exists(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Exists operation failed after delete: %v", err)
	}
	if exists {
		t.Error("Key should not exist after deletion")
	}
	
	t.Log("Basic functionality test passed")
}

func TestL1MemoryCache_Performance(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
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
	if err != nil {
		t.Errorf("Put operation failed: %v", err)
	}
	if putLatency > 10*time.Millisecond {
		t.Errorf("Put operation took %v, expected <10ms", putLatency)
	}
	
	// Test Get performance - should be <10ms
	start = time.Now()
	retrieved, err := cache.Get(ctx, "perf-test-key")
	getLatency := time.Since(start)
	if err != nil {
		t.Errorf("Get operation failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved entry is nil")
	}
	if getLatency > 10*time.Millisecond {
		t.Errorf("Get operation took %v, expected <10ms", getLatency)
	}
	
	t.Logf("Performance test passed - Put: %v, Get: %v", putLatency, getLatency)
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
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
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
		if err != nil {
			t.Errorf("Put operation failed for key %d: %v", i, err)
		}
	}
	
	// Check that eviction occurred
	stats := cache.GetStats()
	if stats.EntryCount > config.MaxEntries {
		t.Errorf("Entry count %d exceeds max entries %d", stats.EntryCount, config.MaxEntries)
	}
	if stats.EvictionCount == 0 {
		t.Error("Expected evictions to have occurred")
	}
	
	t.Logf("LRU eviction test passed - entries: %d, evictions: %d", stats.EntryCount, stats.EvictionCount)
}

func TestL1MemoryCache_Compression(t *testing.T) {
	config := &L1MemoryCacheConfig{
		MaxCapacity:          10 * 1024 * 1024, // 10MB
		MaxEntries:           1000,
		CompressionEnabled:   true,
		CompressionThreshold: 100, // Compress anything > 100 bytes
		CompressionType:      CompressionSnappy, // Use S2/Snappy since it's available
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
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
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
	if err != nil {
		t.Errorf("Put operation failed: %v", err)
	}
	
	// Retrieve and verify
	retrieved, err := cache.Get(ctx, "compression-test-key")
	if err != nil {
		t.Errorf("Get operation failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved entry is nil")
	}
	if retrieved.Method != entry.Method {
		t.Errorf("Expected method %s, got %s", entry.Method, retrieved.Method)
	}
	if string(retrieved.Response) != string(entry.Response) {
		t.Error("Response data doesn't match after compression/decompression")
	}
	
	t.Log("Compression test passed")
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
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
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
	if err != nil {
		t.Errorf("Put operation failed: %v", err)
	}
	
	// Verify it exists initially
	exists, err := cache.Exists(ctx, "ttl-test-key")
	if err != nil {
		t.Errorf("Exists operation failed: %v", err)
	}
	if !exists {
		t.Error("Entry should exist initially")
	}
	
	// Wait for TTL expiration
	time.Sleep(150 * time.Millisecond)
	
	// Verify it's expired
	exists, err = cache.Exists(ctx, "ttl-test-key")
	if err != nil {
		t.Errorf("Exists operation failed after TTL: %v", err)
	}
	if exists {
		t.Error("Entry should have expired")
	}
	
	// Verify Get returns error for expired entry
	_, err = cache.Get(ctx, "ttl-test-key")
	if err == nil {
		t.Error("Get should return error for expired entry")
	}
	
	t.Log("TTL expiration test passed")
}

func TestL1MemoryCache_Stats(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()
	
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
	
	// Add some test data
	entry := &CacheEntry{
		Method:     "textDocument/publishDiagnostics",
		Params:     `{"uri":"file:///test.go","diagnostics":[]}`,
		Response:   json.RawMessage(`{"method":"textDocument/publishDiagnostics","params":{"uri":"file:///test.go","diagnostics":[]}}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	for i := 0; i < 5; i++ {
		err = cache.Put(ctx, fmt.Sprintf("stats-test-key-%d", i), entry)
		if err != nil {
			t.Errorf("Put operation failed for key %d: %v", i, err)
		}
	}
	
	// Perform some gets
	for i := 0; i < 3; i++ {
		_, err = cache.Get(ctx, fmt.Sprintf("stats-test-key-%d", i))
		if err != nil {
			t.Errorf("Get operation failed for key %d: %v", i, err)
		}
	}
	
	// Test stats
	stats := cache.GetStats()
	if stats.TierType != TierL1Memory {
		t.Errorf("Expected TierType %v, got %v", TierL1Memory, stats.TierType)
	}
	if stats.EntryCount != 5 {
		t.Errorf("Expected 5 entries, got %d", stats.EntryCount)
	}
	if stats.TotalRequests == 0 {
		t.Error("Expected non-zero total requests")
	}
	if stats.CacheHits == 0 {
		t.Error("Expected non-zero cache hits")
	}
	if stats.UsedCapacity == 0 {
		t.Error("Expected non-zero used capacity")
	}
	
	// Test health
	health := cache.GetHealth()
	if health.TierType != TierL1Memory {
		t.Errorf("Expected TierType %v, got %v", TierL1Memory, health.TierType)
	}
	if !health.Healthy {
		t.Error("Cache should be healthy")
	}
	
	// Test capacity
	capacity := cache.GetCapacity()
	if capacity.TierType != TierL1Memory {
		t.Errorf("Expected TierType %v, got %v", TierL1Memory, capacity.TierType)
	}
	if capacity.MaxCapacity == 0 {
		t.Error("Expected non-zero max capacity")
	}
	if capacity.UsedEntries != 5 {
		t.Errorf("Expected 5 used entries, got %d", capacity.UsedEntries)
	}
	
	t.Logf("Stats test passed - Entries: %d, Requests: %d, Hits: %d, HitRate: %.2f", 
		stats.EntryCount, stats.TotalRequests, stats.CacheHits, stats.HitRate)
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
	if err != nil {
		b.Fatalf("Failed to initialize cache: %v", err)
	}
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
	if err != nil {
		b.Fatalf("Failed to put benchmark data: %v", err)
	}
	
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
	if err != nil {
		b.Fatalf("Failed to initialize cache: %v", err)
	}
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