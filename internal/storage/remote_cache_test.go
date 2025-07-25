package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lsp-gateway/internal/gateway"
)

// MockBackend implements RemoteBackend for testing
type MockBackend struct {
	data      map[string][]byte
	metadata  map[string]*StorageMetadata
	mu        sync.RWMutex
	healthy   bool
	latency   time.Duration
	errorRate float64
	calls     map[string]int
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		data:     make(map[string][]byte),
		metadata: make(map[string]*StorageMetadata),
		healthy:  true,
		calls:    make(map[string]int),
	}
}

func (m *MockBackend) GetBackendType() BackendType {
	return BackendCustom
}

func (m *MockBackend) GetEndpoint() string {
	return "mock://localhost"
}

func (m *MockBackend) Initialize(ctx context.Context, config BackendConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls["initialize"]++
	return nil
}

func (m *MockBackend) Get(ctx context.Context, key string) ([]byte, *StorageMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	m.calls["get"]++
	
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	
	if !m.healthy {
		return nil, nil, fmt.Errorf("backend unhealthy")
	}
	
	data, exists := m.data[key]
	if !exists {
		return nil, nil, ErrCacheEntryNotFound
	}
	
	metadata := m.metadata[key]
	if metadata == nil {
		metadata = &StorageMetadata{
			Key:  key,
			Size: int64(len(data)),
		}
	}
	
	return data, metadata, nil
}

func (m *MockBackend) Put(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["put"]++
	
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	
	if !m.healthy {
		return fmt.Errorf("backend unhealthy")
	}
	
	m.data[key] = data
	if metadata != nil {
		m.metadata[key] = metadata
	}
	
	return nil
}

func (m *MockBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["delete"]++
	
	if !m.healthy {
		return fmt.Errorf("backend unhealthy")
	}
	
	delete(m.data, key)
	delete(m.metadata, key)
	
	return nil
}

func (m *MockBackend) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	m.calls["exists"]++
	
	if !m.healthy {
		return false, fmt.Errorf("backend unhealthy")
	}
	
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockBackend) GetBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	m.calls["get_batch"]++
	
	if !m.healthy {
		return nil, fmt.Errorf("backend unhealthy")
	}
	
	result := make(map[string][]byte)
	for _, key := range keys {
		if data, exists := m.data[key]; exists {
			result[key] = data
		}
	}
	
	return result, nil
}

func (m *MockBackend) PutBatch(ctx context.Context, entries map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["put_batch"]++
	
	if !m.healthy {
		return fmt.Errorf("backend unhealthy")
	}
	
	for key, data := range entries {
		m.data[key] = data
	}
	
	return nil
}

func (m *MockBackend) DeleteBatch(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.calls["delete_batch"]++
	
	if !m.healthy {
		return fmt.Errorf("backend unhealthy")
	}
	
	for _, key := range keys {
		delete(m.data, key)
		delete(m.metadata, key)
	}
	
	return nil
}

func (m *MockBackend) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	m.calls["ping"]++
	
	if !m.healthy {
		return fmt.Errorf("backend unhealthy")
	}
	
	return nil
}

func (m *MockBackend) GetStats(ctx context.Context) (*BackendStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return &BackendStats{
		Endpoint:     m.GetEndpoint(),
		Healthy:      m.healthy,
		ResponseTime: m.latency,
		ErrorRate:    m.errorRate,
		RequestCount: int64(m.getTotalCalls()),
		LastCheck:    time.Now(),
	}, nil
}

func (m *MockBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls["close"]++
	return nil
}

func (m *MockBackend) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

func (m *MockBackend) SetLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency = latency
}

func (m *MockBackend) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls[operation]
}

func (m *MockBackend) getTotalCalls() int {
	total := 0
	for _, count := range m.calls {
		total += count
	}
	return total
}

// Test helper functions

func createTestRemoteCache(t *testing.T) (*RemoteCache, *MockBackend) {
	config := TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendCustom,
		BackendConfig: BackendConfig{
			Type:           BackendCustom,
			TimeoutMs:      5000,
			MaxConnections: 10,
			RetryCount:     3,
			RetryDelayMs:   100,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 5,
				RecoveryTimeout:  30 * time.Second,
				HalfOpenRequests: 3,
			},
		},
		MaxCapacity:         1024 * 1024 * 1024, // 1GB
		MaxEntries:          10000,
		TimeoutMs:           5000,
		RetryCount:          3,
		RetryDelayMs:        100,
		MaintenanceInterval: 5 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionZstd,
			Level:     3,
			MinSize:   1024,
			Threshold: 0.8,
		},
	}

	rc, err := NewRemoteCache(config)
	require.NoError(t, err)

	// Replace the backend with our mock
	mockBackend := NewMockBackend()
	rc.backends["mock"] = mockBackend
	rc.activeBackend = "mock"

	return rc, mockBackend
}

func createTestCacheEntry(key string) *CacheEntry {
	response := map[string]interface{}{
		"result": "test data",
		"symbols": []string{"function1", "class1"},
	}
	responseData, _ := json.Marshal(response)

	return &CacheEntry{
		Method:      "textDocument/documentSymbol",
		Params:      fmt.Sprintf(`{"textDocument":{"uri":"file://%s"}}`, key),
		Response:    responseData,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         time.Hour,
		Key:         key,
		Size:        int64(len(responseData)),
		Version:     1,
		CurrentTier: TierL3Remote,
		OriginTier:  TierL1Memory,
		Priority:    50,
		AccessCount: 1,
		FilePaths:   []string{key},
	}
}

// Unit Tests

func TestRemoteCacheCreation(t *testing.T) {
	config := TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendCustom,
		BackendConfig: BackendConfig{
			Type:         BackendCustom,
			TimeoutMs:    5000,
			RetryCount:   3,
			RetryDelayMs: 100,
		},
		MaxCapacity:  1024 * 1024,
		TimeoutMs:    5000,
		RetryCount:   3,
		RetryDelayMs: 100,
	}

	rc, err := NewRemoteCache(config)
	assert.NoError(t, err)
	assert.NotNil(t, rc)
	assert.Equal(t, TierL3Remote, rc.GetTierType())
	assert.Equal(t, 3, rc.GetTierLevel())

	defer rc.Close()
}

func TestInvalidTierType(t *testing.T) {
	config := TierConfig{
		TierType: TierL1Memory, // Invalid for remote cache
	}

	rc, err := NewRemoteCache(config)
	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "invalid tier type")
}

func TestRemoteCacheGetPut(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Test Put
	err := rc.Put(ctx, key, entry)
	assert.NoError(t, err)
	assert.Equal(t, 1, mockBackend.GetCallCount("put"))

	// Test Get
	retrievedEntry, err := rc.Get(ctx, key)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedEntry)
	assert.Equal(t, entry.Method, retrievedEntry.Method)
	assert.Equal(t, entry.Key, retrievedEntry.Key)
	assert.Equal(t, 1, mockBackend.GetCallCount("get"))
}

func TestRemoteCacheGetNonExistent(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	
	_, err := rc.Get(ctx, "nonexistent/file.go")
	assert.Error(t, err)
	assert.Equal(t, ErrCacheEntryNotFound, err)
}

func TestRemoteCacheDelete(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Put and then delete
	err := rc.Put(ctx, key, entry)
	assert.NoError(t, err)

	err = rc.Delete(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, 1, mockBackend.GetCallCount("delete"))

	// Verify it's gone
	_, err = rc.Get(ctx, key)
	assert.Error(t, err)
}

func TestRemoteCacheExists(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Check non-existent
	exists, err := rc.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Put and check exists
	err = rc.Put(ctx, key, entry)
	assert.NoError(t, err)

	exists, err = rc.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 2, mockBackend.GetCallCount("exists"))
}

func TestRemoteCacheBatchOperations(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()

	// Create test entries
	entries := make(map[string]*CacheEntry)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("test/file%d.go", i)
		entries[key] = createTestCacheEntry(key)
	}

	// Test PutBatch
	err := rc.PutBatch(ctx, entries)
	assert.NoError(t, err)
	assert.Equal(t, 1, mockBackend.GetCallCount("put_batch"))

	// Test GetBatch
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}

	retrievedEntries, err := rc.GetBatch(ctx, keys)
	assert.NoError(t, err)
	assert.Equal(t, len(entries), len(retrievedEntries))
	assert.Equal(t, 1, mockBackend.GetCallCount("get_batch"))

	// Test DeleteBatch
	err = rc.DeleteBatch(ctx, keys)
	assert.NoError(t, err)
	assert.Equal(t, 1, mockBackend.GetCallCount("delete_batch"))
}

func TestRemoteCacheCircuitBreaker(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Make backend unhealthy
	mockBackend.SetHealthy(false)

	// Try operations - should fail and trigger circuit breaker
	for i := 0; i < 10; i++ {
		rc.Put(ctx, key, entry)
		rc.Get(ctx, key)
	}

	// Check circuit breaker state
	health := rc.GetHealth()
	// Circuit breaker should be triggered after multiple failures
	assert.False(t, health.Healthy)
}

func TestRemoteCacheRetryLogic(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Make backend unhealthy temporarily
	mockBackend.SetHealthy(false)

	// Try operation - should retry
	err := rc.Put(ctx, key, entry)
	assert.Error(t, err)

	// Check that retries were attempted
	stats := rc.GetStats()
	assert.Greater(t, stats.ErrorCount, int64(0))
}

func TestRemoteCacheAsyncOperations(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)
	
	// Set low priority for async operation
	entry.Priority = 10
	entry.CachingHint = CachingHintPreferRemote

	// Start background workers
	err := rc.Initialize(ctx, rc.config)
	assert.NoError(t, err)

	// Put should use async path
	err = rc.putAsync(ctx, key, entry)
	assert.NoError(t, err)

	// Wait for background processing
	time.Sleep(100 * time.Millisecond)

	// Flush to complete async operations
	err = rc.Flush(ctx)
	assert.NoError(t, err)

	// Verify operation was processed
	assert.Greater(t, mockBackend.GetCallCount("put"), 0)
}

func TestRemoteCacheStats(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Perform some operations
	rc.Put(ctx, key, entry)
	rc.Get(ctx, key)
	rc.Get(ctx, "nonexistent.go") // Miss

	stats := rc.GetStats()
	assert.Equal(t, TierL3Remote, stats.TierType)
	assert.Greater(t, stats.TotalRequests, int64(0))
	assert.Greater(t, stats.CacheHits, int64(0))
	assert.Greater(t, stats.CacheMisses, int64(0))
	assert.True(t, stats.HitRate >= 0.0 && stats.HitRate <= 1.0)
}

func TestRemoteCacheHealth(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()

	// Initially healthy
	health := rc.GetHealth()
	assert.Equal(t, TierL3Remote, health.TierType)
	assert.True(t, health.Healthy)
	assert.Equal(t, TierStatusHealthy, health.Status)

	// Make backend unhealthy
	mockBackend.SetHealthy(false)
	
	// Update health (simulating maintenance routine)
	rc.updateHealthStatus(ctx)
	
	health = rc.GetHealth()
	assert.False(t, health.Healthy)
	assert.Greater(t, len(health.Issues), 0)
}

func TestRemoteCacheCapacity(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	capacity := rc.GetCapacity()
	assert.Equal(t, TierL3Remote, capacity.TierType)
	assert.Greater(t, capacity.MaxCapacity, int64(0))
	assert.Equal(t, int64(0), capacity.UsedCapacity)
	assert.True(t, capacity.UtilizationPct >= 0.0)
}

func TestRemoteCacheCompression(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	// Test compression provider
	if rc.compression != nil {
		testData := []byte("This is some test data that should be compressed for efficiency")
		
		compressed, err := rc.compression.Compress(testData)
		assert.NoError(t, err)
		assert.NotNil(t, compressed)
		
		decompressed, err := rc.compression.Decompress(compressed)
		assert.NoError(t, err)
		assert.Equal(t, testData, decompressed)
		
		ratio := rc.compression.EstimateCompressionRatio(testData)
		assert.True(t, ratio > 0.0 && ratio <= 1.0)
	}
}

func TestRemoteCacheSerialization(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	entry := createTestCacheEntry("test.go")
	
	// Test serialization
	data, err := rc.serializer.Serialize(entry)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// Test deserialization
	var retrievedEntry CacheEntry
	err = rc.serializer.Deserialize(data, &retrievedEntry)
	assert.NoError(t, err)
	assert.Equal(t, entry.Method, retrievedEntry.Method)
	assert.Equal(t, entry.Key, retrievedEntry.Key)
}

func TestRemoteCacheLatencyTracking(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	// Set some latency
	mockBackend.SetLatency(50 * time.Millisecond)

	ctx := context.Background()
	key := "test/file.go"
	entry := createTestCacheEntry(key)

	// Perform operations
	rc.Put(ctx, key, entry)
	rc.Get(ctx, key)

	stats := rc.GetStats()
	assert.Greater(t, stats.AvgLatency, time.Duration(0))
	assert.Greater(t, stats.MaxLatency, time.Duration(0))
}

func TestRemoteCacheMaintenanceRoutine(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()

	// Initialize to start maintenance routine
	err := rc.Initialize(ctx, rc.config)
	assert.NoError(t, err)

	// Let maintenance run once
	time.Sleep(100 * time.Millisecond)

	// Verify maintenance updates health
	health := rc.GetHealth()
	assert.NotZero(t, health.LastCheck)
}

func TestRemoteCacheClose(t *testing.T) {
	rc, mockBackend := createTestRemoteCache(t)

	ctx := context.Background()
	err := rc.Initialize(ctx, rc.config)
	assert.NoError(t, err)

	// Close should stop all background processes
	err = rc.Close()
	assert.NoError(t, err)

	// Verify backend was closed
	assert.Equal(t, 1, mockBackend.GetCallCount("close"))

	// Multiple closes should be safe
	err = rc.Close()
	assert.NoError(t, err)
}

// Integration Tests with HTTP Server

func TestHTTPRestBackendIntegration(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.URL.Path == "/cache/test.go" {
				w.Header().Set("Content-Type", "application/msgpack")
				w.Header().Set("ETag", "test-checksum")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("test data"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case "PUT":
			if r.URL.Path == "/cache/test.go" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		case "DELETE":
			if r.URL.Path == "/cache/test.go" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case "HEAD":
			if r.URL.Path == "/cache/test.go" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	// Create HTTP backend
	backend := &HTTPRestBackend{
		endpoint: server.URL,
		client:   http.DefaultClient,
		headers:  make(map[string]string),
		timeout:  5 * time.Second,
	}

	ctx := context.Background()
	config := BackendConfig{
		Type:           BackendCustom,
		ConnectionString: server.URL,
		TimeoutMs:        5000,
	}

	// Test Initialize
	err := backend.Initialize(ctx, config)
	assert.NoError(t, err)

	// Test Put
	testData := []byte("test data")
	metadata := &StorageMetadata{
		Key:         "test.go",
		ContentType: "application/msgpack",
		Size:        int64(len(testData)),
	}
	err = backend.Put(ctx, "test.go", testData, metadata)
	assert.NoError(t, err)

	// Test Get
	data, retrievedMetadata, err := backend.Get(ctx, "test.go")
	assert.NoError(t, err)
	assert.Equal(t, testData, data)
	assert.Equal(t, "application/msgpack", retrievedMetadata.ContentType)
	assert.Equal(t, "test-checksum", retrievedMetadata.Checksum)

	// Test Exists
	exists, err := backend.Exists(ctx, "test.go")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test Delete
	err = backend.Delete(ctx, "test.go")
	assert.NoError(t, err)

	// Test Ping
	err = backend.Ping(ctx)
	assert.NoError(t, err)

	// Test GetStats
	stats, err := backend.GetStats(ctx)
	assert.NoError(t, err)
	assert.True(t, stats.Healthy)
	assert.Equal(t, server.URL, stats.Endpoint)
}

// Performance Tests

func TestRemoteCachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	rc, mockBackend := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()

	// Set realistic latency
	mockBackend.SetLatency(10 * time.Millisecond)

	const numOperations = 1000
	entries := make(map[string]*CacheEntry)

	// Generate test data
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("test/file%d.go", i)
		entries[key] = createTestCacheEntry(key)
	}

	// Measure batch put performance
	start := time.Now()
	err := rc.PutBatch(ctx, entries)
	putDuration := time.Since(start)
	assert.NoError(t, err)

	// Measure batch get performance
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}

	start = time.Now()
	retrievedEntries, err := rc.GetBatch(ctx, keys)
	getDuration := time.Since(start)
	assert.NoError(t, err)
	assert.Equal(t, len(entries), len(retrievedEntries))

	t.Logf("Batch Put (%d entries): %v", numOperations, putDuration)
	t.Logf("Batch Get (%d entries): %v", numOperations, getDuration)
	t.Logf("Put throughput: %.2f ops/sec", float64(numOperations)/putDuration.Seconds())
	t.Logf("Get throughput: %.2f ops/sec", float64(numOperations)/getDuration.Seconds())

	// Verify performance targets
	// Target: <200ms for reasonable batch sizes
	assert.Less(t, putDuration, 200*time.Millisecond)
	assert.Less(t, getDuration, 200*time.Millisecond)
}

func TestRemoteCacheConcurrency(t *testing.T) {
	rc, _ := createTestRemoteCache(t)
	defer rc.Close()

	ctx := context.Background()
	err := rc.Initialize(ctx, rc.config)
	assert.NoError(t, err)

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("concurrent/file%d_%d.go", id, j)
				entry := createTestCacheEntry(key)

				// Put
				if err := rc.Put(ctx, key, entry); err != nil {
					errors <- err
					return
				}

				// Get
				if _, err := rc.Get(ctx, key); err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Concurrent operation error: %v", err)
		errorCount++
	}

	// Allow some errors due to circuit breaker, but not too many
	assert.Less(t, errorCount, numGoroutines*numOperations/10)

	stats := rc.GetStats()
	t.Logf("Final stats - Requests: %d, Hits: %d, Misses: %d, Errors: %d",
		stats.TotalRequests, stats.CacheHits, stats.CacheMisses, stats.ErrorCount)
}