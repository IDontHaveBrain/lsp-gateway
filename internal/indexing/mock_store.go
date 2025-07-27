package indexing

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// MockSCIPStore is a mock implementation of SCIPStore for testing
type MockSCIPStore struct {
	config     *SCIPConfig
	cache      map[string]*SCIPCacheEntry
	stats      SCIPStoreStats
	mutex      sync.RWMutex
	cacheMutex sync.RWMutex
	queryCount int64
	hitCount   int64
	startTime  time.Time

	// Mock behavior configuration
	CacheHitProbability float64 // Probability of cache hit (0.0 to 1.0)
	ResponseLatencyMs   int     // Base response latency in milliseconds
	FailureProbability  float64 // Probability of query failure (0.0 to 1.0)
}

// SCIPCacheEntry represents a cached SCIP query response
type SCIPCacheEntry struct {
	Method     string          `json:"method"`
	Params     string          `json:"params"` // JSON-encoded parameters
	Response   json.RawMessage `json:"response"`
	CreatedAt  time.Time       `json:"created_at"`
	AccessedAt time.Time       `json:"accessed_at"`
	TTL        time.Duration   `json:"ttl"`
}

// IsExpired checks if the cache entry has expired
func (e *SCIPCacheEntry) IsExpired() bool {
	return time.Since(e.CreatedAt) > e.TTL
}

// Touch updates the accessed time for the cache entry
func (e *SCIPCacheEntry) Touch() {
	e.AccessedAt = time.Now()
}

// NewSCIPIndexStore creates a new SCIP index store with the given configuration
// This now returns the real SCIPStore implementation instead of the mock
func NewSCIPIndexStore(config *SCIPConfig) SCIPStore {
	return NewRealSCIPStore(config)
}

// NewMockSCIPStore creates a new mock SCIP store for testing
func NewMockSCIPStore(config *SCIPConfig) SCIPStore {
	if config == nil {
		// Provide default configuration for testing
		config = &SCIPConfig{
			CacheConfig: CacheConfig{
				Enabled: true,
				MaxSize: 1000,
				TTL:     10 * time.Minute,
			},
			Logging: LoggingConfig{
				LogQueries:         false,
				LogCacheOperations: false,
				LogIndexOperations: false,
			},
			Performance: PerformanceConfig{
				QueryTimeout:         5 * time.Second,
				MaxConcurrentQueries: 10,
				IndexLoadTimeout:     30 * time.Second,
			},
		}
	}

	return &MockSCIPStore{
		config:              config,
		cache:               make(map[string]*SCIPCacheEntry),
		stats:               SCIPStoreStats{},
		startTime:           time.Now(),
		CacheHitProbability: 0.8, // 80% cache hit rate for testing
		ResponseLatencyMs:   10,  // 10ms base latency
		FailureProbability:  0.1, // 10% failure rate for testing
	}
}

// LoadIndex loads a SCIP index from the specified path (mock implementation)
func (s *MockSCIPStore) LoadIndex(path string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	startTime := time.Now()

	// Simulate index loading delay
	time.Sleep(time.Duration(10+rand.Intn(40)) * time.Millisecond)

	s.stats.IndexesLoaded++

	duration := time.Since(startTime)

	if s.config.Logging.LogIndexOperations {
		fmt.Printf("SCIP: Mock loaded index from %s in %v\n", path, duration)
	}

	return nil
}

// Query performs a query against the loaded SCIP index (mock implementation)
func (s *MockSCIPStore) Query(method string, params interface{}) SCIPQueryResult {
	startTime := time.Now()
	s.mutex.Lock()
	queryID := s.queryCount
	s.queryCount++
	s.mutex.Unlock()

	// Check if method is supported by SCIP
	if !IsSupportedMethod(method) {
		return SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      fmt.Sprintf("unsupported method: %s", method),
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.0,
		}
	}

	// Simulate query processing time
	baseLatency := time.Duration(s.ResponseLatencyMs) * time.Millisecond
	jitter := time.Duration(rand.Intn(s.ResponseLatencyMs)) * time.Millisecond
	time.Sleep(baseLatency + jitter)

	// Check cache first
	cacheKey := s.generateCacheKey(method, params)
	if entry := s.getCachedResponse(cacheKey); entry != nil {
		s.mutex.Lock()
		s.hitCount++
		s.mutex.Unlock()

		entry.Touch()
		return SCIPQueryResult{
			Found:      true,
			Method:     method,
			Response:   entry.Response,
			CacheHit:   true,
			QueryTime:  time.Since(startTime),
			Confidence: 0.9, // Cache hit has high confidence
		}
	}

	// Simulate cache miss - decide if we have a result
	hasResult := rand.Float64() > s.FailureProbability && rand.Float64() < 0.7 // 70% success rate for finding results

	var result SCIPQueryResult
	if hasResult {
		// Simulate successful SCIP query result
		mockResponse := s.generateMockResponse(method, params)

		result = SCIPQueryResult{
			Found:      true,
			Method:     method,
			Response:   mockResponse,
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.8, // Mock results have reasonable confidence
			Metadata: map[string]interface{}{
				"query_id":       queryID,
				"implementation": "mock",
				"index_used":     true,
			},
		}

		// Cache the result if cache is enabled
		if s.config.CacheConfig.Enabled {
			s.CacheResponse(method, params, mockResponse)
		}
	} else {
		// Simulate cache miss with no result found
		result = SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      "no results found in SCIP index (mock)",
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.0,
			Metadata: map[string]interface{}{
				"query_id":       queryID,
				"implementation": "mock",
				"index_used":     true,
			},
		}
	}

	// Update statistics
	s.updateStats(result.QueryTime)

	if s.config.Logging.LogQueries {
		fmt.Printf("SCIP: Query %d for method %s completed in %v (found: %t, cache_hit: %t)\n",
			queryID, method, result.QueryTime, result.Found, result.CacheHit)
	}

	return result
}

// CacheResponse caches a response for future queries
func (s *MockSCIPStore) CacheResponse(method string, params interface{}, response json.RawMessage) error {
	if !s.config.CacheConfig.Enabled {
		return nil
	}

	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Check cache size limit
	if len(s.cache) >= s.config.CacheConfig.MaxSize {
		s.evictOldestEntry()
	}

	cacheKey := s.generateCacheKey(method, params)
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params for caching: %w", err)
	}

	entry := &SCIPCacheEntry{
		Method:     method,
		Params:     string(paramsJSON),
		Response:   response,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        s.config.CacheConfig.TTL,
	}

	s.cache[cacheKey] = entry

	if s.config.Logging.LogCacheOperations {
		fmt.Printf("SCIP: Cached response for method %s\n", method)
	}

	return nil
}

// InvalidateFile invalidates cached entries for a specific file
func (s *MockSCIPStore) InvalidateFile(filePath string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	keysToDelete := make([]string, 0)

	for key, entry := range s.cache {
		// Simple check if the file path is mentioned in the params
		if strings.Contains(entry.Params, filePath) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(s.cache, key)
	}

	if s.config.Logging.LogCacheOperations && len(keysToDelete) > 0 {
		fmt.Printf("SCIP: Invalidated %d cache entries for file %s\n", len(keysToDelete), filePath)
	}
}

// GetStats returns statistics about the store
func (s *MockSCIPStore) GetStats() SCIPStoreStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var hitRate float64
	if s.queryCount > 0 {
		hitRate = float64(s.hitCount) / float64(s.queryCount)
	}

	var avgQueryTime time.Duration
	if s.queryCount > 0 {
		avgQueryTime = s.stats.AverageQueryTime
	}

	return SCIPStoreStats{
		IndexesLoaded:    s.stats.IndexesLoaded,
		TotalQueries:     s.queryCount,
		CacheHitRate:     hitRate,
		AverageQueryTime: avgQueryTime,
		LastQueryTime:    s.stats.LastQueryTime,
		CacheSize:        len(s.cache),
		MemoryUsage:      s.estimateMemoryUsage(),
	}
}

// Close cleans up resources and closes the store
func (s *MockSCIPStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cacheMutex.Lock()
	s.cache = make(map[string]*SCIPCacheEntry)
	s.cacheMutex.Unlock()

	if s.config.Logging.LogIndexOperations {
		fmt.Println("SCIP: Mock store closed successfully")
	}

	return nil
}

// Helper methods

func (s *MockSCIPStore) generateCacheKey(method string, params interface{}) string {
	paramsJSON, _ := json.Marshal(params)
	return fmt.Sprintf("%s:%s", method, string(paramsJSON))
}

func (s *MockSCIPStore) getCachedResponse(key string) *SCIPCacheEntry {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	entry, exists := s.cache[key]
	if !exists {
		return nil
	}

	if entry.IsExpired() {
		// Remove expired entry
		s.cacheMutex.RUnlock()
		s.cacheMutex.Lock()
		delete(s.cache, key)
		s.cacheMutex.Unlock()
		s.cacheMutex.RLock()
		return nil
	}

	// Simulate cache hit probability for testing
	if rand.Float64() > s.CacheHitProbability {
		return nil // Simulate cache miss
	}

	return entry
}

func (s *MockSCIPStore) evictOldestEntry() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range s.cache {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(s.cache, oldestKey)
	}
}

func (s *MockSCIPStore) updateStats(queryTime time.Duration) {
	s.stats.LastQueryTime = time.Now()

	// Simple moving average for query time
	if s.stats.AverageQueryTime == 0 {
		s.stats.AverageQueryTime = queryTime
	} else {
		s.stats.AverageQueryTime = (s.stats.AverageQueryTime + queryTime) / 2
	}
}

func (s *MockSCIPStore) estimateMemoryUsage() int64 {
	// Rough estimation of memory usage
	baseSize := int64(1024) // Base struct size estimate

	// Add cache size estimate
	cacheSize := int64(len(s.cache) * 512) // Rough estimate per cache entry

	return baseSize + cacheSize
}

func (s *MockSCIPStore) generateMockResponse(method string, params interface{}) json.RawMessage {
	// Generate mock responses based on LSP method
	switch method {
	case "textDocument/definition":
		return json.RawMessage(`{"uri": "file:///mock/definition.go", "range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 15}}}`)
	case "textDocument/references":
		return json.RawMessage(`[{"uri": "file:///mock/reference1.go", "range": {"start": {"line": 5, "character": 10}, "end": {"line": 5, "character": 20}}}]`)
	case "textDocument/hover":
		return json.RawMessage(`{"contents": {"kind": "markdown", "value": "Mock hover information"}}`)
	case "textDocument/documentSymbol":
		return json.RawMessage(`[{"name": "MockSymbol", "kind": 12, "range": {"start": {"line": 1, "character": 0}, "end": {"line": 10, "character": 0}}}]`)
	case "workspace/symbol":
		return json.RawMessage(`[{"name": "MockWorkspaceSymbol", "kind": 5, "location": {"uri": "file:///mock/symbol.go", "range": {"start": {"line": 5, "character": 0}, "end": {"line": 15, "character": 0}}}}]`)
	default:
		return json.RawMessage(`{"mock": true, "method": "` + method + `"}`)
	}
}

// SupportedLSPMethods contains the LSP methods that SCIP indexing supports
var SupportedLSPMethods = []string{
	"textDocument/definition",
	"textDocument/references",
	"textDocument/hover",
	"textDocument/documentSymbol",
	"workspace/symbol",
}

// IsSupportedMethod checks if the given LSP method is supported by SCIP indexing
func IsSupportedMethod(method string) bool {
	for _, supported := range SupportedLSPMethods {
		if method == supported {
			return true
		}
	}
	return false
}

// Ensure MockSCIPStore implements SCIPStore interface at compile time
var _ SCIPStore = (*MockSCIPStore)(nil)
