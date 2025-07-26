package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
)

// L1MemoryCache implements a high-performance in-memory cache tier
// with LRU eviction, compression support, and comprehensive monitoring
type L1MemoryCache struct {
	// Core cache storage with O(1) LRU operations
	entries map[string]*memoryCacheEntry
	lruHead *memoryCacheEntry
	lruTail *memoryCacheEntry

	// Concurrency control with RWMutex for better read performance
	mu sync.RWMutex

	// Configuration
	config *L1MemoryCacheConfig

	// Compression providers
	compressor CompressionProvider

	// Atomic counters for thread-safe metrics
	stats *l1CacheStats

	// Background maintenance
	stopCh    chan struct{}
	cleanupWg sync.WaitGroup

	// Circuit breaker for self-protection
	circuitBreaker *circuitBreaker

	// Object pool for reducing GC pressure
	entryPool  sync.Pool
	bufferPool sync.Pool

	// Health monitoring
	health *l1CacheHealth

	// Started flag
	started atomic.Bool
}

// memoryCacheEntry represents an entry in the LRU cache
type memoryCacheEntry struct {
	key        string
	value      *CacheEntry
	compressed []byte
	size       int64
	prev       *memoryCacheEntry
	next       *memoryCacheEntry
	accessTime time.Time
	createTime time.Time
}

// L1MemoryCacheConfig contains configuration for the L1 memory cache
type L1MemoryCacheConfig struct {
	MaxCapacity          int64                 `json:"max_capacity"`
	MaxEntries           int64                 `json:"max_entries"`
	EvictionThreshold    float64               `json:"eviction_threshold"`
	CompressionEnabled   bool                  `json:"compression_enabled"`
	CompressionThreshold int64                 `json:"compression_threshold"`
	CompressionType      CompressionType       `json:"compression_type"`
	TTLCheckInterval     time.Duration         `json:"ttl_check_interval"`
	MetricsInterval      time.Duration         `json:"metrics_interval"`
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker_config"`
	PoolSizes            *PoolSizes            `json:"pool_sizes"`
}

// PoolSizes configures object pool sizes for memory optimization
type PoolSizes struct {
	EntryPoolSize  int `json:"entry_pool_size"`
	BufferPoolSize int `json:"buffer_pool_size"`
}

// l1CacheStats provides atomic counters for cache statistics
type l1CacheStats struct {
	// Request metrics
	totalRequests atomic.Int64
	cacheHits     atomic.Int64
	cacheMisses   atomic.Int64

	// Performance metrics
	totalLatency atomic.Int64 // nanoseconds
	maxLatency   atomic.Int64 // nanoseconds
	minLatency   atomic.Int64 // nanoseconds

	// Capacity metrics
	currentEntries atomic.Int64
	currentSize    atomic.Int64
	peakEntries    atomic.Int64
	peakSize       atomic.Int64

	// Operation metrics
	evictions      atomic.Int64
	compressions   atomic.Int64
	decompressions atomic.Int64
	ttlExpirations atomic.Int64

	// Error metrics
	errors              atomic.Int64
	compressionErrors   atomic.Int64
	decompressionErrors atomic.Int64

	// Time tracking
	startTime time.Time
	lastReset atomic.Value // time.Time
}

// l1CacheHealth tracks health status
type l1CacheHealth struct {
	mu           sync.RWMutex
	healthy      bool
	issues       []HealthIssue
	lastCheck    time.Time
	healthScore  float64
	circuitState CircuitBreakerState
}

// circuitBreaker provides self-protection against cascading failures
type circuitBreaker struct {
	mu          sync.RWMutex
	state       CircuitBreakerState
	failures    atomic.Int64
	lastFailure atomic.Value // time.Time
	nextRetry   atomic.Value // time.Time
	config      *CircuitBreakerConfig
}

// defaultL1Config returns default configuration for L1 memory cache
func defaultL1Config() *L1MemoryCacheConfig {
	return &L1MemoryCacheConfig{
		MaxCapacity:          8 * 1024 * 1024 * 1024, // 8GB
		MaxEntries:           1000000,                // 1M entries
		EvictionThreshold:    0.8,                    // 80% utilization
		CompressionEnabled:   true,
		CompressionThreshold: 1024, // 1KB
		CompressionType:      CompressionLZ4,
		TTLCheckInterval:     30 * time.Second,
		MetricsInterval:      10 * time.Second,
		CircuitBreakerConfig: &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 10,
			RecoveryTimeout:  30 * time.Second,
			HalfOpenRequests: 5,
			MinRequestCount:  20,
		},
		PoolSizes: &PoolSizes{
			EntryPoolSize:  1000,
			BufferPoolSize: 100,
		},
	}
}

// NewL1MemoryCache creates a new L1 memory cache instance
func NewL1MemoryCache(config *L1MemoryCacheConfig) *L1MemoryCache {
	if config == nil {
		config = defaultL1Config()
	}

	cache := &L1MemoryCache{
		entries:        make(map[string]*memoryCacheEntry),
		config:         config,
		stats:          &l1CacheStats{startTime: time.Now()},
		stopCh:         make(chan struct{}),
		health:         &l1CacheHealth{healthy: true, healthScore: 1.0},
		circuitBreaker: newCircuitBreaker(config.CircuitBreakerConfig),
	}

	// Initialize compression provider
	cache.compressor = newCompressionProvider(config.CompressionType)

	// Initialize object pools
	cache.initializePools()

	// Initialize LRU sentinel nodes
	cache.initializeLRU()

	return cache
}

// initializePools sets up object pools for memory optimization
func (c *L1MemoryCache) initializePools() {
	c.entryPool = sync.Pool{
		New: func() interface{} {
			return &memoryCacheEntry{}
		},
	}

	c.bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}
}

// initializeLRU sets up the LRU doubly-linked list
func (c *L1MemoryCache) initializeLRU() {
	c.lruHead = &memoryCacheEntry{}
	c.lruTail = &memoryCacheEntry{}
	c.lruHead.next = c.lruTail
	c.lruTail.prev = c.lruHead
}

// Initialize implements StorageTier interface
func (c *L1MemoryCache) Initialize(ctx context.Context, config TierConfig) error {
	if c.started.Load() {
		return errors.New("L1 memory cache already initialized")
	}

	// Update configuration if provided
	if config.BackendConfig.Options != nil {
		if err := c.updateConfigFromTierConfig(config); err != nil {
			return fmt.Errorf("failed to update configuration: %w", err)
		}
	}

	// Start background processes
	c.cleanupWg.Add(2)
	go c.ttlCleanupWorker()
	go c.metricsWorker()

	c.started.Store(true)

	return nil
}

// Get implements StorageTier interface with <10ms performance target
func (c *L1MemoryCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		c.recordLatency(latency)
		c.stats.totalRequests.Add(1)
	}()

	// Circuit breaker check
	if !c.circuitBreaker.canExecute() {
		c.stats.errors.Add(1)
		return nil, errors.New("circuit breaker open")
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.stats.cacheMisses.Add(1)
		c.circuitBreaker.recordFailure()
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Check TTL expiration
	if entry.value.IsExpired() {
		c.mu.Lock()
		c.removeLRU(entry)
		delete(c.entries, key)
		c.mu.Unlock()

		c.stats.cacheMisses.Add(1)
		c.stats.ttlExpirations.Add(1)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	// Move to front of LRU
	c.mu.Lock()
	c.moveToFront(entry)
	c.mu.Unlock()

	// Update access metadata
	entry.value.Touch()
	entry.accessTime = time.Now()

	// Decompress if needed
	result := entry.value
	if entry.compressed != nil {
		decompressed, err := c.decompress(entry.compressed)
		if err != nil {
			c.stats.decompressionErrors.Add(1)
			c.circuitBreaker.recordFailure()
			return nil, fmt.Errorf("decompression failed: %w", err)
		}

		if err := json.Unmarshal(decompressed, result); err != nil {
			c.stats.errors.Add(1)
			return nil, fmt.Errorf("deserialization failed: %w", err)
		}

		c.stats.decompressions.Add(1)
	}

	c.stats.cacheHits.Add(1)
	c.circuitBreaker.recordSuccess()

	return result, nil
}

// Put implements StorageTier interface with automatic eviction and compression
func (c *L1MemoryCache) Put(ctx context.Context, key string, entry *CacheEntry) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		c.recordLatency(latency)
		c.stats.totalRequests.Add(1)
	}()

	// Circuit breaker check
	if !c.circuitBreaker.canExecute() {
		c.stats.errors.Add(1)
		return errors.New("circuit breaker open")
	}

	// Serialize and optionally compress the entry
	data, err := json.Marshal(entry)
	if err != nil {
		c.stats.errors.Add(1)
		return fmt.Errorf("serialization failed: %w", err)
	}

	var compressed []byte
	entrySize := int64(len(data))

	if c.config.CompressionEnabled && entrySize > c.config.CompressionThreshold {
		compressed, err = c.compress(data)
		if err != nil {
			c.stats.compressionErrors.Add(1)
			// Continue without compression
		} else {
			c.stats.compressions.Add(1)
			entrySize = int64(len(compressed))
		}
	}

	// Calculate checksum
	entry.Checksum = c.calculateChecksum(data)
	entry.Size = int64(len(data))
	entry.CompressedSize = entrySize
	entry.CurrentTier = TierL1Memory

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if eviction is needed
	if c.needsEviction(entrySize) {
		if err := c.evictLRU(); err != nil {
			c.stats.errors.Add(1)
			return fmt.Errorf("eviction failed: %w", err)
		}
	}

	// Remove existing entry if present
	if existing, exists := c.entries[key]; exists {
		c.removeLRU(existing)
		c.returnEntryToPool(existing)
	}

	// Create new entry
	newEntry := c.getEntryFromPool()
	newEntry.key = key
	newEntry.value = entry
	newEntry.compressed = compressed
	newEntry.size = entrySize
	newEntry.accessTime = time.Now()
	newEntry.createTime = time.Now()

	// Add to cache and LRU front
	c.entries[key] = newEntry
	c.addToFront(newEntry)

	// Update metrics
	c.stats.currentEntries.Store(int64(len(c.entries)))
	c.stats.currentSize.Add(entrySize)

	if currentEntries := c.stats.currentEntries.Load(); currentEntries > c.stats.peakEntries.Load() {
		c.stats.peakEntries.Store(currentEntries)
	}
	if currentSize := c.stats.currentSize.Load(); currentSize > c.stats.peakSize.Load() {
		c.stats.peakSize.Store(currentSize)
	}

	c.circuitBreaker.recordSuccess()
	return nil
}

// Delete implements StorageTier interface
func (c *L1MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	c.removeLRU(entry)
	delete(c.entries, key)

	c.stats.currentEntries.Store(int64(len(c.entries)))
	c.stats.currentSize.Add(-entry.size)

	c.returnEntryToPool(entry)

	return nil
}

// Exists implements StorageTier interface
func (c *L1MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return false, nil
	}

	// Check TTL expiration
	if entry.value.IsExpired() {
		// Remove expired entry
		c.mu.Lock()
		c.removeLRU(entry)
		delete(c.entries, key)
		c.mu.Unlock()

		c.stats.ttlExpirations.Add(1)
		return false, nil
	}

	return true, nil
}

// GetBatch implements StorageTier interface for efficient batch operations
func (c *L1MemoryCache) GetBatch(ctx context.Context, keys []string) (map[string]*CacheEntry, error) {
	result := make(map[string]*CacheEntry, len(keys))

	for _, key := range keys {
		if entry, err := c.Get(ctx, key); err == nil {
			result[key] = entry
		}
	}

	return result, nil
}

// PutBatch implements StorageTier interface for efficient batch operations
func (c *L1MemoryCache) PutBatch(ctx context.Context, entries map[string]*CacheEntry) error {
	var lastError error

	for key, entry := range entries {
		if err := c.Put(ctx, key, entry); err != nil {
			lastError = err
		}
	}

	return lastError
}

// DeleteBatch implements StorageTier interface
func (c *L1MemoryCache) DeleteBatch(ctx context.Context, keys []string) error {
	var lastError error

	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			lastError = err
		}
	}

	return lastError
}

// Invalidate implements StorageTier interface with pattern matching
func (c *L1MemoryCache) Invalidate(ctx context.Context, pattern string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toDelete []string

	for key := range c.entries {
		if matched, _ := c.matchPattern(key, pattern); matched {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		if entry, exists := c.entries[key]; exists {
			c.removeLRU(entry)
			delete(c.entries, key)
			c.returnEntryToPool(entry)
		}
	}

	c.stats.currentEntries.Store(int64(len(c.entries)))

	return len(toDelete), nil
}

// InvalidateByFile implements StorageTier interface
func (c *L1MemoryCache) InvalidateByFile(ctx context.Context, filePath string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toDelete []string

	for key, entry := range c.entries {
		for _, path := range entry.value.FilePaths {
			if path == filePath {
				toDelete = append(toDelete, key)
				break
			}
		}
	}

	for _, key := range toDelete {
		if entry, exists := c.entries[key]; exists {
			c.removeLRU(entry)
			delete(c.entries, key)
			c.returnEntryToPool(entry)
		}
	}

	c.stats.currentEntries.Store(int64(len(c.entries)))

	return len(toDelete), nil
}

// InvalidateByProject implements StorageTier interface
func (c *L1MemoryCache) InvalidateByProject(ctx context.Context, projectPath string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toDelete []string

	for key, entry := range c.entries {
		if entry.value.ProjectPath == projectPath {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		if entry, exists := c.entries[key]; exists {
			c.removeLRU(entry)
			delete(c.entries, key)
			c.returnEntryToPool(entry)
		}
	}

	c.stats.currentEntries.Store(int64(len(c.entries)))

	return len(toDelete), nil
}

// Clear implements StorageTier interface
func (c *L1MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Return all entries to pool
	for _, entry := range c.entries {
		c.returnEntryToPool(entry)
	}

	// Clear all data structures
	c.entries = make(map[string]*memoryCacheEntry)
	c.initializeLRU()

	// Reset metrics
	c.stats.currentEntries.Store(0)
	c.stats.currentSize.Store(0)

	return nil
}

// GetStats implements StorageTier interface
func (c *L1MemoryCache) GetStats() TierStats {
	totalRequests := c.stats.totalRequests.Load()
	cacheHits := c.stats.cacheHits.Load()

	var hitRate float64
	if totalRequests > 0 {
		hitRate = float64(cacheHits) / float64(totalRequests)
	}

	var avgLatency time.Duration
	if totalRequests > 0 {
		avgLatency = time.Duration(c.stats.totalLatency.Load() / totalRequests)
	}

	uptime := time.Since(c.stats.startTime)

	var requestsPerSecond float64
	if uptime.Seconds() > 0 {
		requestsPerSecond = float64(totalRequests) / uptime.Seconds()
	}

	return TierStats{
		TierType:          TierL1Memory,
		TotalCapacity:     c.config.MaxCapacity,
		UsedCapacity:      c.stats.currentSize.Load(),
		FreeCapacity:      c.config.MaxCapacity - c.stats.currentSize.Load(),
		EntryCount:        c.stats.currentEntries.Load(),
		TotalRequests:     totalRequests,
		CacheHits:         cacheHits,
		CacheMisses:       c.stats.cacheMisses.Load(),
		HitRate:           hitRate,
		AvgLatency:        avgLatency,
		P50Latency:        avgLatency, // Simplified for now
		P95Latency:        time.Duration(c.stats.maxLatency.Load()),
		P99Latency:        time.Duration(c.stats.maxLatency.Load()),
		MaxLatency:        time.Duration(c.stats.maxLatency.Load()),
		RequestsPerSecond: requestsPerSecond,
		ErrorCount:        c.stats.errors.Load(),
		ErrorRate:         c.calculateErrorRate(),
		EvictionCount:     c.stats.evictions.Load(),
		StartTime:         c.stats.startTime,
		LastUpdate:        time.Now(),
		Uptime:            uptime,
	}
}

// GetHealth implements StorageTier interface
func (c *L1MemoryCache) GetHealth() TierHealth {
	c.health.mu.RLock()
	defer c.health.mu.RUnlock()

	health := TierHealth{
		TierType:  TierL1Memory,
		Healthy:   c.health.healthy,
		LastCheck: c.health.lastCheck,
		Issues:    append([]HealthIssue{}, c.health.issues...),
		Metrics:   c.getHealthMetrics(),
	}

	health.CircuitBreaker.State = c.circuitBreaker.getState()
	health.CircuitBreaker.FailureCount = int(c.circuitBreaker.failures.Load())

	if lastFailure := c.circuitBreaker.lastFailure.Load(); lastFailure != nil {
		health.CircuitBreaker.LastFailure = lastFailure.(time.Time)
	}

	if nextRetry := c.circuitBreaker.nextRetry.Load(); nextRetry != nil {
		health.CircuitBreaker.NextRetry = nextRetry.(time.Time)
	}

	return health
}

// GetCapacity implements StorageTier interface
func (c *L1MemoryCache) GetCapacity() TierCapacity {
	currentSize := c.stats.currentSize.Load()
	currentEntries := c.stats.currentEntries.Load()

	var utilizationPct float64
	if c.config.MaxCapacity > 0 {
		utilizationPct = float64(currentSize) / float64(c.config.MaxCapacity) * 100
	}

	return TierCapacity{
		TierType:          TierL1Memory,
		MaxCapacity:       c.config.MaxCapacity,
		UsedCapacity:      currentSize,
		AvailableCapacity: c.config.MaxCapacity - currentSize,
		MaxEntries:        c.config.MaxEntries,
		UsedEntries:       currentEntries,
		UtilizationPct:    utilizationPct,
	}
}

// GetTierType implements StorageTier interface
func (c *L1MemoryCache) GetTierType() TierType {
	return TierL1Memory
}

// GetTierLevel implements StorageTier interface
func (c *L1MemoryCache) GetTierLevel() int {
	return 1
}

// Flush implements StorageTier interface
func (c *L1MemoryCache) Flush(ctx context.Context) error {
	// Memory cache doesn't need flushing
	return nil
}

// Close implements StorageTier interface
func (c *L1MemoryCache) Close() error {
	if !c.started.Load() {
		return nil
	}

	close(c.stopCh)
	c.cleanupWg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Return all entries to pool
	for _, entry := range c.entries {
		c.returnEntryToPool(entry)
	}

	c.entries = nil
	c.started.Store(false)

	return nil
}

// LRU management methods

// addToFront adds an entry to the front of the LRU list
func (c *L1MemoryCache) addToFront(entry *memoryCacheEntry) {
	entry.prev = c.lruHead
	entry.next = c.lruHead.next
	c.lruHead.next.prev = entry
	c.lruHead.next = entry
}

// removeLRU removes an entry from the LRU list
func (c *L1MemoryCache) removeLRU(entry *memoryCacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
}

// moveToFront moves an entry to the front of the LRU list
func (c *L1MemoryCache) moveToFront(entry *memoryCacheEntry) {
	c.removeLRU(entry)
	c.addToFront(entry)
}

// evictLRU evicts the least recently used entry
func (c *L1MemoryCache) evictLRU() error {
	if c.lruTail.prev == c.lruHead {
		return errors.New("no entries to evict")
	}

	toEvict := c.lruTail.prev
	c.removeLRU(toEvict)
	delete(c.entries, toEvict.key)

	c.stats.currentSize.Add(-toEvict.size)
	c.stats.evictions.Add(1)

	c.returnEntryToPool(toEvict)

	return nil
}

// needsEviction checks if eviction is needed to accommodate new entry
func (c *L1MemoryCache) needsEviction(newEntrySize int64) bool {
	currentSize := c.stats.currentSize.Load()
	currentEntries := c.stats.currentEntries.Load()

	// Check capacity limits
	if (currentSize + newEntrySize) > c.config.MaxCapacity {
		return true
	}

	if currentEntries >= c.config.MaxEntries {
		return true
	}

	// Check threshold
	utilizationPct := float64(currentSize+newEntrySize) / float64(c.config.MaxCapacity)
	if utilizationPct > c.config.EvictionThreshold {
		return true
	}

	return false
}

// Compression methods

// compress compresses data using the configured compression algorithm
func (c *L1MemoryCache) compress(data []byte) ([]byte, error) {
	if c.compressor == nil {
		return nil, errors.New("no compressor available")
	}

	return c.compressor.Compress(data)
}

// decompress decompresses data using the configured compression algorithm
func (c *L1MemoryCache) decompress(data []byte) ([]byte, error) {
	if c.compressor == nil {
		return nil, errors.New("no compressor available")
	}

	return c.compressor.Decompress(data)
}

// Helper methods

// calculateChecksum calculates SHA-256 checksum for data integrity
func (c *L1MemoryCache) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// matchPattern matches a key against a pattern (supports * wildcard)
func (c *L1MemoryCache) matchPattern(key, pattern string) (bool, error) {
	// Simple wildcard matching - can be enhanced with regex
	if pattern == "*" {
		return true, nil
	}

	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(key, parts[0]) && strings.HasSuffix(key, parts[1]), nil
		}
	}

	return key == pattern, nil
}

// getHealthMetrics returns health-related metrics
func (c *L1MemoryCache) getHealthMetrics() map[string]float64 {
	metrics := make(map[string]float64)

	totalRequests := c.stats.totalRequests.Load()
	if totalRequests > 0 {
		metrics["hit_rate"] = float64(c.stats.cacheHits.Load()) / float64(totalRequests)
		metrics["error_rate"] = float64(c.stats.errors.Load()) / float64(totalRequests)
	}

	metrics["utilization"] = float64(c.stats.currentSize.Load()) / float64(c.config.MaxCapacity)
	metrics["entry_count"] = float64(c.stats.currentEntries.Load())

	return metrics
}

// calculateErrorRate calculates the current error rate
func (c *L1MemoryCache) calculateErrorRate() float64 {
	totalRequests := c.stats.totalRequests.Load()
	if totalRequests == 0 {
		return 0.0
	}

	return float64(c.stats.errors.Load()) / float64(totalRequests)
}

// recordLatency records latency metrics atomically
func (c *L1MemoryCache) recordLatency(latency time.Duration) {
	nanos := latency.Nanoseconds()
	c.stats.totalLatency.Add(nanos)

	// Update max latency
	for {
		current := c.stats.maxLatency.Load()
		if nanos <= current || c.stats.maxLatency.CompareAndSwap(current, nanos) {
			break
		}
	}

	// Update min latency
	for {
		current := c.stats.minLatency.Load()
		if current == 0 || (nanos < current && c.stats.minLatency.CompareAndSwap(current, nanos)) {
			break
		}
	}
}

// Object pool methods

// getEntryFromPool gets an entry from the pool
func (c *L1MemoryCache) getEntryFromPool() *memoryCacheEntry {
	entry := c.entryPool.Get().(*memoryCacheEntry)
	// Reset the entry
	entry.key = ""
	entry.value = nil
	entry.compressed = nil
	entry.size = 0
	entry.prev = nil
	entry.next = nil
	entry.accessTime = time.Time{}
	entry.createTime = time.Time{}
	return entry
}

// returnEntryToPool returns an entry to the pool
func (c *L1MemoryCache) returnEntryToPool(entry *memoryCacheEntry) {
	// Clear sensitive data
	entry.value = nil
	entry.compressed = nil
	c.entryPool.Put(entry)
}

// Background workers

// ttlCleanupWorker periodically removes expired entries
func (c *L1MemoryCache) ttlCleanupWorker() {
	defer c.cleanupWg.Done()

	ticker := time.NewTicker(c.config.TTLCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanupExpiredEntries()
		}
	}
}

// metricsWorker periodically updates metrics and health status
func (c *L1MemoryCache) metricsWorker() {
	defer c.cleanupWg.Done()

	ticker := time.NewTicker(c.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.updateHealthStatus()
		}
	}
}

// cleanupExpiredEntries removes expired entries
func (c *L1MemoryCache) cleanupExpiredEntries() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toDelete []string

	for key, entry := range c.entries {
		if entry.value.IsExpired() {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		if entry, exists := c.entries[key]; exists {
			c.removeLRU(entry)
			delete(c.entries, key)
			c.returnEntryToPool(entry)
			c.stats.ttlExpirations.Add(1)
		}
	}

	if len(toDelete) > 0 {
		c.stats.currentEntries.Store(int64(len(c.entries)))
	}
}

// updateHealthStatus updates the health status based on current metrics
func (c *L1MemoryCache) updateHealthStatus() {
	c.health.mu.Lock()
	defer c.health.mu.Unlock()

	c.health.lastCheck = time.Now()
	c.health.issues = c.health.issues[:0] // Clear existing issues

	// Check error rate
	errorRate := c.calculateErrorRate()
	if errorRate > 0.05 { // 5% error threshold
		c.health.issues = append(c.health.issues, HealthIssue{
			Type:      "high_error_rate",
			Severity:  IssueSeverityWarning,
			Message:   fmt.Sprintf("Error rate is %.2f%%", errorRate*100),
			Timestamp: time.Now(),
			Component: "L1MemoryCache",
		})
	}

	// Check memory utilization
	utilization := float64(c.stats.currentSize.Load()) / float64(c.config.MaxCapacity)
	if utilization > 0.9 { // 90% utilization threshold
		c.health.issues = append(c.health.issues, HealthIssue{
			Type:      "high_memory_utilization",
			Severity:  IssueSeverityWarning,
			Message:   fmt.Sprintf("Memory utilization is %.1f%%", utilization*100),
			Timestamp: time.Now(),
			Component: "L1MemoryCache",
		})
	}

	// Update health score
	c.health.healthScore = c.calculateHealthScore()
	c.health.healthy = c.health.healthScore > 0.7 && len(c.health.issues) == 0
	c.health.circuitState = c.circuitBreaker.getState()
}

// calculateHealthScore calculates an overall health score
func (c *L1MemoryCache) calculateHealthScore() float64 {
	score := 1.0

	// Penalize high error rate
	errorRate := c.calculateErrorRate()
	score -= errorRate * 0.5

	// Penalize high utilization
	utilization := float64(c.stats.currentSize.Load()) / float64(c.config.MaxCapacity)
	if utilization > 0.8 {
		score -= (utilization - 0.8) * 0.5
	}

	// Penalize circuit breaker issues
	if c.circuitBreaker.getState() != CircuitBreakerClosed {
		score -= 0.3
	}

	if score < 0 {
		score = 0
	}

	return score
}

// updateConfigFromTierConfig updates cache configuration from tier config
func (c *L1MemoryCache) updateConfigFromTierConfig(config TierConfig) error {
	options := config.BackendConfig.Options

	if maxCapacity, ok := options["max_capacity"].(int64); ok {
		c.config.MaxCapacity = maxCapacity
	}

	if maxEntries, ok := options["max_entries"].(int64); ok {
		c.config.MaxEntries = maxEntries
	}

	if compressionEnabled, ok := options["compression_enabled"].(bool); ok {
		c.config.CompressionEnabled = compressionEnabled
	}

	return nil
}

// Circuit breaker implementation

// newCircuitBreaker creates a new circuit breaker
func newCircuitBreaker(config *CircuitBreakerConfig) *circuitBreaker {
	if config == nil || !config.Enabled {
		return &circuitBreaker{
			state:  CircuitBreakerClosed,
			config: config,
		}
	}

	return &circuitBreaker{
		state:  CircuitBreakerClosed,
		config: config,
	}
}

// canExecute checks if requests can be executed
func (cb *circuitBreaker) canExecute() bool {
	if cb.config == nil || !cb.config.Enabled {
		return true
	}

	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		if nextRetry := cb.nextRetry.Load(); nextRetry != nil {
			if time.Now().After(nextRetry.(time.Time)) {
				cb.mu.Lock()
				cb.state = CircuitBreakerHalfOpen
				cb.mu.Unlock()
				return true
			}
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// recordSuccess records a successful operation
func (cb *circuitBreaker) recordSuccess() {
	if cb.config == nil || !cb.config.Enabled {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitBreakerHalfOpen {
		cb.state = CircuitBreakerClosed
		cb.failures.Store(0)
	}
}

// recordFailure records a failed operation
func (cb *circuitBreaker) recordFailure() {
	if cb.config == nil || !cb.config.Enabled {
		return
	}

	failures := cb.failures.Add(1)
	cb.lastFailure.Store(time.Now())

	if failures >= int64(cb.config.FailureThreshold) {
		cb.mu.Lock()
		cb.state = CircuitBreakerOpen
		cb.nextRetry.Store(time.Now().Add(cb.config.RecoveryTimeout))
		cb.mu.Unlock()
	}
}

// getState returns the current circuit breaker state
func (cb *circuitBreaker) getState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Compression provider implementation

type simpleCompressionProvider struct {
	compressionType CompressionType
}

func newCompressionProvider(compressionType CompressionType) CompressionProvider {
	return &simpleCompressionProvider{
		compressionType: compressionType,
	}
}

func (cp *simpleCompressionProvider) Compress(data []byte) ([]byte, error) {
	switch cp.compressionType {
	case CompressionLZ4:
		// Use S2 as LZ4 alternative since it's available
		return cp.compressS2(data)
	case CompressionGzip:
		return cp.compressGzip(data)
	case CompressionSnappy:
		return cp.compressS2(data)
	case CompressionZstd:
		return cp.compressZstd(data)
	default:
		return data, nil
	}
}

func (cp *simpleCompressionProvider) Decompress(data []byte) ([]byte, error) {
	switch cp.compressionType {
	case CompressionLZ4:
		// Use S2 as LZ4 alternative since it's available
		return cp.decompressS2(data)
	case CompressionGzip:
		return cp.decompressGzip(data)
	case CompressionSnappy:
		return cp.decompressS2(data)
	case CompressionZstd:
		return cp.decompressZstd(data)
	default:
		return data, nil
	}
}

func (cp *simpleCompressionProvider) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (cp *simpleCompressionProvider) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (cp *simpleCompressionProvider) compressS2(data []byte) ([]byte, error) {
	return s2.Encode(nil, data), nil
}

func (cp *simpleCompressionProvider) decompressS2(data []byte) ([]byte, error) {
	return s2.Decode(nil, data)
}

func (cp *simpleCompressionProvider) compressZstd(data []byte) ([]byte, error) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}
	defer encoder.Close()
	return encoder.EncodeAll(data, nil), nil
}

func (cp *simpleCompressionProvider) decompressZstd(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return decoder.DecodeAll(data, nil)
}

func (cp *simpleCompressionProvider) EstimateCompressionRatio(data []byte) float64 {
	// Simple estimation based on entropy
	return 0.6 // 60% compression ratio estimate
}

func (cp *simpleCompressionProvider) GetCompressionType() CompressionType {
	return cp.compressionType
}

func (cp *simpleCompressionProvider) GetCompressionLevel() int {
	return 1 // Default level
}

func (cp *simpleCompressionProvider) SetCompressionLevel(level int) error {
	// Not implemented for this simple provider
	return nil
}

func (cp *simpleCompressionProvider) SetCompressionType(compressionType CompressionType) error {
	cp.compressionType = compressionType
	return nil
}
