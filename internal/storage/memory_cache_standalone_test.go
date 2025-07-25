package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// Minimal mock types to avoid dependency issues
type mockSCIPStore struct{}

func (m *mockSCIPStore) LoadIndex(path string) error                                            { return nil }
func (m *mockSCIPStore) Query(method string, params interface{}) SCIPQueryResult               { return SCIPQueryResult{} }
func (m *mockSCIPStore) CacheResponse(method string, params interface{}, response json.RawMessage) error { return nil }
func (m *mockSCIPStore) InvalidateFile(filePath string)                                         {}
func (m *mockSCIPStore) GetStats() SCIPStoreStats                                               { return SCIPStoreStats{} }
func (m *mockSCIPStore) Close() error                                                           { return nil }

type SCIPQueryResult struct {
	Found      bool            `json:"found"`
	Method     string          `json:"method"`
	Response   json.RawMessage `json:"response,omitempty"`
	Error      string          `json:"error,omitempty"`
	CacheHit   bool            `json:"cache_hit"`
	QueryTime  time.Duration   `json:"query_time"`
	IndexPath  string          `json:"index_path,omitempty"`
	Confidence float64         `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type SCIPStoreStats struct {
	IndexesLoaded   int           `json:"indexes_loaded"`
	TotalQueries    int64         `json:"total_queries"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	AverageQueryTime time.Duration `json:"average_query_time"`
	LastQueryTime   time.Time     `json:"last_query_time"`
	CacheSize       int           `json:"cache_size"`
	MemoryUsage     int64         `json:"memory_usage_bytes"`
}

func TestL1MemoryCache_StandaloneTest(t *testing.T) {
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
	
	// Test performance requirement (<10ms)
	start := time.Now()
	_, err = cache.Get(ctx, "test-key-1")
	latency := time.Since(start)
	if err != nil {
		t.Errorf("Get operation failed: %v", err)
	}
	if latency > 10*time.Millisecond {
		t.Errorf("Get operation took %v, expected <10ms", latency)
	}
	
	// Test stats
	stats := cache.GetStats()
	if stats.TierType != TierL1Memory {
		t.Errorf("Expected TierType %v, got %v", TierL1Memory, stats.TierType)
	}
	if stats.EntryCount != 1 {
		t.Errorf("Expected 1 entry, got %d", stats.EntryCount)
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
	if capacity.UsedEntries != 1 {
		t.Errorf("Expected 1 used entry, got %d", capacity.UsedEntries)
	}
	
	t.Logf("Standalone test passed - Performance: %v, Entries: %d", latency, stats.EntryCount)
}

func TestL1MemoryCache_PerformanceBenchmark(t *testing.T) {
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
		Response:   json.RawMessage(`{"contents":{"kind":"markdown","value":"## Function Documentation\nThis is comprehensive documentation."}}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}
	
	// Measure multiple operations to get average latency
	const numOperations = 1000
	var totalPutLatency, totalGetLatency time.Duration
	
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf-key-%d", i)
		
		// Measure Put latency
		start := time.Now()
		err = cache.Put(ctx, key, entry)
		totalPutLatency += time.Since(start)
		if err != nil {
			t.Errorf("Put operation %d failed: %v", i, err)
		}
		
		// Measure Get latency
		start = time.Now()
		_, err = cache.Get(ctx, key)
		totalGetLatency += time.Since(start)
		if err != nil {
			t.Errorf("Get operation %d failed: %v", i, err)
		}
	}
	
	avgPutLatency := totalPutLatency / numOperations
	avgGetLatency := totalGetLatency / numOperations
	
	// Verify performance requirements
	if avgPutLatency > 10*time.Millisecond {
		t.Errorf("Average Put latency %v exceeds 10ms requirement", avgPutLatency)
	}
	if avgGetLatency > 10*time.Millisecond {
		t.Errorf("Average Get latency %v exceeds 10ms requirement", avgGetLatency)
	}
	
	t.Logf("Performance benchmark passed:")
	t.Logf("  Average Put latency: %v", avgPutLatency)
	t.Logf("  Average Get latency: %v", avgGetLatency)
	t.Logf("  Operations completed: %d", numOperations)
	
	// Test throughput (operations per second)
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, err = cache.Get(ctx, fmt.Sprintf("perf-key-%d", i%numOperations))
		if err != nil {
			t.Errorf("Throughput test Get operation failed: %v", err)
		}
	}
	throughputDuration := time.Since(start)
	throughput := float64(1000) / throughputDuration.Seconds()
	
	t.Logf("  Throughput: %.0f operations/second", throughput)
	
	// Verify throughput requirement (1000+ ops/sec)
	if throughput < 1000 {
		t.Errorf("Throughput %.0f ops/sec is below 1000 ops/sec requirement", throughput)
	}
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
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()
	
	// Test concurrent operations
	const numGoroutines = 50
	const numOperations = 20
	
	errCh := make(chan error, numGoroutines*numOperations)
	doneCh := make(chan bool, numGoroutines)
	
	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { doneCh <- true }()
			
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
					errCh <- fmt.Errorf("put failed for %s: %v", key, err)
					return
				}
				
				if _, err := cache.Get(ctx, key); err != nil {
					errCh <- fmt.Errorf("get failed for %s: %v", key, err)
					return
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-doneCh
	}
	close(errCh)
	
	// Check for errors
	errorCount := 0
	for err := range errCh {
		t.Errorf("Concurrent operation failed: %v", err)
		errorCount++
	}
	
	if errorCount > 0 {
		t.Errorf("Had %d errors in concurrent test", errorCount)
	}
	
	// Verify final state
	stats := cache.GetStats()
	expectedEntries := int64(numGoroutines * numOperations)
	t.Logf("Concurrent test completed - Expected entries: %d, Actual entries: %d", 
		expectedEntries, stats.EntryCount)
	
	if stats.EntryCount != expectedEntries {
		t.Errorf("Expected %d entries, got %d", expectedEntries, stats.EntryCount)
	}
	
	t.Log("Concurrent access test passed")
}