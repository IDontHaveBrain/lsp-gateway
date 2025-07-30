package indexing

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RealSCIPStore is the production implementation of SCIPStore using actual SCIP indexing
type RealSCIPStore struct {
	scipClient SCIPStore // Active SCIP client adapter for querying indices
	cache      *SCIPCache
	config     *SCIPConfig
	stats      SCIPStoreStats
	mutex      sync.RWMutex

	// Performance tracking
	queryTimes []time.Duration
	startTime  time.Time
	queryCount int64
	hitCount   int64

	// Concurrency control
	queryLimiter chan struct{}
	shutdown     chan struct{}
	wg           sync.WaitGroup
}

// SCIPCache implements production-ready caching with LRU eviction and TTL
type SCIPCache struct {
	entries map[string]*SCIPCacheEntryExtended
	lruList *list.List
	maxSize int
	mutex   sync.RWMutex

	// Performance metrics
	hitCount      int64
	missCount     int64
	evictionCount int64

	// Background cleanup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
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

// SCIPCacheEntryExtended extends the existing SCIPCacheEntry with LRU support
type SCIPCacheEntryExtended struct {
	*SCIPCacheEntry

	// LRU support
	lruElement *list.Element

	// File associations for invalidation
	FilePaths []string `json:"file_paths"`
}

// NewRealSCIPStore creates a new production SCIP store with the given configuration
func NewRealSCIPStore(config *SCIPConfig) SCIPStore {
	if config == nil {
		// Provide default configuration
		config = &SCIPConfig{
			CacheConfig: CacheConfig{
				Enabled: true,
				MaxSize: 10000,
				TTL:     30 * time.Minute,
			},
			Logging: LoggingConfig{
				LogQueries:         false,
				LogCacheOperations: false,
				LogIndexOperations: true,
			},
			Performance: PerformanceConfig{
				QueryTimeout:         30 * time.Second,
				MaxConcurrentQueries: 100,
				IndexLoadTimeout:     5 * time.Minute,
			},
		}
	}

	// Create SCIP client using the working SCIPClient implementation
	var scipClient SCIPStore
	baseClient, err := NewSCIPClient(config)
	if err != nil {
		if config.Logging.LogIndexOperations {
			log.Printf("SCIP: Failed to create SCIP client: %v, using degraded mode", err)
		}
		scipClient = nil
	} else {
		// Wrap the SCIP client with the adapter to implement SCIPStore interface
		scipClient = NewSCIPClientAdapter(baseClient)
		if config.Logging.LogIndexOperations {
			log.Printf("SCIP: Client successfully initialized with indexing capabilities")
		}
	}

	// Create cache
	cache := NewSCIPCache(config.CacheConfig)

	store := &RealSCIPStore{
		scipClient:   scipClient,
		cache:        cache,
		config:       config,
		stats:        SCIPStoreStats{},
		startTime:    time.Now(),
		queryLimiter: make(chan struct{}, config.Performance.MaxConcurrentQueries),
		shutdown:     make(chan struct{}),
		queryTimes:   make([]time.Duration, 0, 1000),
	}

	// Start background processes
	store.startBackgroundProcesses()

	return store
}

// NewSCIPCache creates a new SCIP cache with the given configuration
func NewSCIPCache(config CacheConfig) *SCIPCache {
	cache := &SCIPCache{
		entries:     make(map[string]*SCIPCacheEntryExtended),
		lruList:     list.New(),
		maxSize:     config.MaxSize,
		stopCleanup: make(chan struct{}),
	}

	if config.Enabled {
		// Start background cleanup process
		cache.cleanupTicker = time.NewTicker(5 * time.Minute)
		go cache.backgroundCleanup()
	}

	return cache
}

// LoadIndex loads a SCIP index from the specified path
func (s *RealSCIPStore) LoadIndex(path string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		if s.config.Logging.LogIndexOperations {
			log.Printf("SCIP: LoadIndex completed for %s in %v", path, duration)
		}
	}()

	// Check if we have a SCIP client
	if s.scipClient == nil {
		s.stats.IndexesLoaded++
		if s.config.Logging.LogIndexOperations {
			log.Printf("SCIP: No SCIP client available, simulating index load for %s", path)
		}
		return nil
	}

	// Load through SCIP client
	if err := s.scipClient.LoadIndex(path); err != nil {
		return fmt.Errorf("failed to load SCIP index: %w", err)
	}

	s.stats.IndexesLoaded++
	return nil
}

// Query performs a query against the loaded SCIP index with caching and concurrency control
func (s *RealSCIPStore) Query(method string, params interface{}) SCIPQueryResult {
	startTime := time.Now()

	// Acquire query slot for concurrency control
	select {
	case s.queryLimiter <- struct{}{}:
		defer func() { <-s.queryLimiter }()
	case <-time.After(5 * time.Second):
		return SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      "query timeout - too many concurrent requests",
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.0,
		}
	}

	// Increment query count
	queryID := atomic.AddInt64(&s.queryCount, 1)

	// Check if method is supported
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

	// Check cache first
	cacheKey := s.generateCacheKey(method, params)
	if cachedResult := s.cache.Get(cacheKey); cachedResult != nil {
		atomic.AddInt64(&s.hitCount, 1)

		result := SCIPQueryResult{
			Found:      true,
			Method:     method,
			Response:   cachedResult.Response,
			CacheHit:   true,
			QueryTime:  time.Since(startTime),
			Confidence: 0.95, // Cache hits have high confidence
			Metadata: map[string]interface{}{
				"query_id":       queryID,
				"implementation": "real_scip",
				"cache_hit":      true,
			},
		}

		s.updateStats(result.QueryTime)
		if s.config.Logging.LogQueries {
			log.Printf("SCIP: Query %d for method %s completed from cache in %v",
				queryID, method, result.QueryTime)
		}
		return result
	}

	// Perform actual SCIP query
	result := s.performSCIPQuery(method, params, queryID, startTime)

	// Cache the result if successful and caching is enabled
	if result.Found && s.config.CacheConfig.Enabled {
		if err := s.CacheResponse(method, params, result.Response); err != nil && s.config.Logging.LogCacheOperations {
			log.Printf("SCIP: Failed to cache response for method %s: %v", method, err)
		}
	}

	s.updateStats(result.QueryTime)

	if s.config.Logging.LogQueries {
		log.Printf("SCIP: Query %d for method %s completed in %v (found: %t, confidence: %.2f)",
			queryID, method, result.QueryTime, result.Found, result.Confidence)
	}

	return result
}

// performSCIPQuery executes the actual SCIP query based on the LSP method
func (s *RealSCIPStore) performSCIPQuery(method string, params interface{}, queryID int64, startTime time.Time) SCIPQueryResult {
	// Check if SCIP client is available
	if s.scipClient == nil {
		// Fallback to degraded mode
		return SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      "SCIP client unavailable - running in degraded mode",
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.0,
			Metadata: map[string]interface{}{
				"query_id":       queryID,
				"implementation": "real_scip",
				"degraded_mode":  true,
			},
		}
	}

	// Use the actual SCIP client to perform the query
	result := s.scipClient.Query(method, params)

	// Add metadata to the result
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["query_id"] = queryID
	result.Metadata["implementation"] = "real_scip"
	result.Metadata["degraded_mode"] = false

	// Update query time to include our processing time
	result.QueryTime = time.Since(startTime)

	return result
}

// executeQuery maps LSP methods to SCIP client operations
func (s *RealSCIPStore) executeQuery(ctx context.Context, method string, params interface{}) (json.RawMessage, float64, error) {
	if s.scipClient == nil {
		return nil, 0.0, fmt.Errorf("SCIP client unavailable - method %s not implemented in degraded mode", method)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, 0.0, ctx.Err()
	default:
	}

	// Perform the query using the SCIP client
	result := s.scipClient.Query(method, params)

	if !result.Found {
		if result.Error != "" {
			return nil, 0.0, fmt.Errorf("SCIP query failed: %s", result.Error)
		}
		// Return empty response for no results based on method type
		switch method {
		case "textDocument/definition", "textDocument/references", "textDocument/documentSymbol", "workspace/symbol":
			return json.RawMessage("[]"), 0.0, nil
		case "textDocument/hover":
			return json.RawMessage("null"), 0.0, nil
		default:
			return json.RawMessage("null"), 0.0, nil
		}
	}

	return result.Response, result.Confidence, nil
}

// TODO: Query handlers will be implemented when SCIP client is available

// CacheResponse caches a response for future queries with LRU management
func (s *RealSCIPStore) CacheResponse(method string, params interface{}, response json.RawMessage) error {
	if !s.config.CacheConfig.Enabled || s.cache == nil {
		return nil
	}

	cacheKey := s.generateCacheKey(method, params)
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params for caching: %w", err)
	}

	// Extract file paths for invalidation tracking
	filePaths := s.extractFilePathsFromParams(params)

	baseEntry := &SCIPCacheEntry{
		Method:     method,
		Params:     string(paramsJSON),
		Response:   response,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        s.config.CacheConfig.TTL,
	}

	entry := &SCIPCacheEntryExtended{
		SCIPCacheEntry: baseEntry,
		FilePaths:      filePaths,
	}

	s.cache.Set(cacheKey, entry)

	if s.config.Logging.LogCacheOperations {
		log.Printf("SCIP: Cached response for method %s (files: %v)", method, filePaths)
	}

	return nil
}

// InvalidateFile invalidates cached entries for a specific file
func (s *RealSCIPStore) InvalidateFile(filePath string) {
	if s.cache == nil {
		return
	}

	keysToDelete := s.cache.InvalidateByFile(filePath)

	if s.config.Logging.LogCacheOperations && len(keysToDelete) > 0 {
		log.Printf("SCIP: Invalidated %d cache entries for file %s", len(keysToDelete), filePath)
	}
}

// GetStats returns comprehensive statistics about the store
func (s *RealSCIPStore) GetStats() SCIPStoreStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	queryCount := atomic.LoadInt64(&s.queryCount)
	hitCount := atomic.LoadInt64(&s.hitCount)

	var hitRate float64
	if queryCount > 0 {
		hitRate = float64(hitCount) / float64(queryCount)
	}

	var avgQueryTime time.Duration
	if len(s.queryTimes) > 0 {
		var total time.Duration
		for _, t := range s.queryTimes {
			total += t
		}
		avgQueryTime = total / time.Duration(len(s.queryTimes))
	}

	var cacheSize int
	var memoryUsage int64

	if s.cache != nil {
		cacheStats := s.cache.GetStats()
		cacheSize = cacheStats.Size
		memoryUsage += cacheStats.MemoryUsage
	}

	// TODO: Add SCIP client stats when client is available
	// For now, only cache memory usage is included

	return SCIPStoreStats{
		IndexesLoaded:    s.stats.IndexesLoaded,
		TotalQueries:     queryCount,
		CacheHitRate:     hitRate,
		AverageQueryTime: avgQueryTime,
		LastQueryTime:    s.stats.LastQueryTime,
		CacheSize:        cacheSize,
		MemoryUsage:      memoryUsage,
	}
}

// Close cleans up resources and closes the store
func (s *RealSCIPStore) Close() error {
	// Signal shutdown
	close(s.shutdown)

	// Wait for background processes to complete
	s.wg.Wait()

	// Close cache
	if s.cache != nil {
		s.cache.Close()
	}

	// Close SCIP client if available
	if s.scipClient != nil {
		if err := s.scipClient.Close(); err != nil && s.config.Logging.LogIndexOperations {
			log.Printf("SCIP: Error closing SCIP client: %v", err)
		}
	}

	if s.config.Logging.LogIndexOperations {
		log.Println("SCIP: Real store closed successfully")
	}

	return nil
}

// Helper methods

func (s *RealSCIPStore) generateCacheKey(method string, params interface{}) string {
	paramsJSON, _ := json.Marshal(params)
	return fmt.Sprintf("%s:%s", method, string(paramsJSON))
}

func (s *RealSCIPStore) extractFilePathsFromParams(params interface{}) []string {
	var filePaths []string

	if paramMap, ok := params.(map[string]interface{}); ok {
		if textDoc, ok := paramMap["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok {
				filePaths = append(filePaths, strings.TrimPrefix(uri, "file://"))
			}
		}
	}

	return filePaths
}

func (s *RealSCIPStore) updateStats(queryTime time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.stats.LastQueryTime = time.Now()

	// Keep last 1000 query times for rolling average
	s.queryTimes = append(s.queryTimes, queryTime)
	if len(s.queryTimes) > 1000 {
		s.queryTimes = s.queryTimes[1:]
	}
}

func (s *RealSCIPStore) startBackgroundProcesses() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.performMaintenanceTasks()
			case <-s.shutdown:
				return
			}
		}
	}()
}

func (s *RealSCIPStore) performMaintenanceTasks() {
	// Perform periodic maintenance like cache cleanup, statistics updates, etc.
	if s.cache != nil {
		s.cache.PerformMaintenance()
	}
}

// SCIPCache methods

func (c *SCIPCache) Set(key string, entry *SCIPCacheEntryExtended) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if entry already exists
	if existingEntry, exists := c.entries[key]; exists {
		// Update existing entry
		existingEntry.Response = entry.Response
		existingEntry.AccessedAt = time.Now()
		existingEntry.TTL = entry.TTL
		existingEntry.FilePaths = entry.FilePaths

		// Move to front of LRU list
		c.lruList.MoveToFront(existingEntry.lruElement)
		return
	}

	// Check if we need to evict
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	// Add new entry
	entry.lruElement = c.lruList.PushFront(key)
	c.entries[key] = entry
}

func (c *SCIPCache) Get(key string) *SCIPCacheEntry {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		atomic.AddInt64(&c.missCount, 1)
		return nil
	}

	// Check if expired
	if entry.IsExpired() {
		c.removeEntry(key)
		atomic.AddInt64(&c.missCount, 1)
		return nil
	}

	// Update access time and move to front
	entry.AccessedAt = time.Now()
	c.lruList.MoveToFront(entry.lruElement)

	atomic.AddInt64(&c.hitCount, 1)
	return entry.SCIPCacheEntry
}

func (c *SCIPCache) InvalidateByFile(filePath string) []string {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var keysToDelete []string

	for key, entry := range c.entries {
		for _, entryFilePath := range entry.FilePaths {
			if entryFilePath == filePath || strings.Contains(entryFilePath, filePath) {
				keysToDelete = append(keysToDelete, key)
				break
			}
		}
	}

	for _, key := range keysToDelete {
		c.removeEntry(key)
	}

	return keysToDelete
}

func (c *SCIPCache) evictLRU() {
	if c.lruList.Len() == 0 {
		return
	}

	// Remove least recently used entry
	backElement := c.lruList.Back()
	if backElement != nil {
		key := backElement.Value.(string)
		c.removeEntry(key)
		atomic.AddInt64(&c.evictionCount, 1)
	}
}

func (c *SCIPCache) removeEntry(key string) {
	if entry, exists := c.entries[key]; exists {
		c.lruList.Remove(entry.lruElement)
		delete(c.entries, key)
	}
}

func (c *SCIPCache) backgroundCleanup() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanupExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *SCIPCache) cleanupExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var expiredKeys []string
	for key, entry := range c.entries {
		if entry.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		c.removeEntry(key)
	}
}

func (c *SCIPCache) PerformMaintenance() {
	c.cleanupExpired()
}

func (c *SCIPCache) GetStats() struct {
	Size          int
	HitCount      int64
	MissCount     int64
	EvictionCount int64
	MemoryUsage   int64
} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var memoryUsage int64
	for key, entry := range c.entries {
		memoryUsage += int64(len(key) + len(entry.Params) + len(entry.Response) + 512) // Rough estimate
	}

	return struct {
		Size          int
		HitCount      int64
		MissCount     int64
		EvictionCount int64
		MemoryUsage   int64
	}{
		Size:          len(c.entries),
		HitCount:      atomic.LoadInt64(&c.hitCount),
		MissCount:     atomic.LoadInt64(&c.missCount),
		EvictionCount: atomic.LoadInt64(&c.evictionCount),
		MemoryUsage:   memoryUsage,
	}
}

func (c *SCIPCache) Close() {
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
	}
	if c.stopCleanup != nil {
		close(c.stopCleanup)
	}

	c.mutex.Lock()
	c.entries = make(map[string]*SCIPCacheEntryExtended)
	c.lruList = list.New()
	c.mutex.Unlock()
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

// Ensure RealSCIPStore implements SCIPStore interface at compile time
var _ SCIPStore = (*RealSCIPStore)(nil)
