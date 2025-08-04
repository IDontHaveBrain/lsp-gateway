package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// Removed duplicate SCIPConfig - now using config.CacheConfig directly

// CacheKey represents a unique identifier for cached LSP responses
type CacheKey struct {
	Method string `json:"method"`
	URI    string `json:"uri"`
	Hash   string `json:"hash"` // Hash of parameters for uniqueness
}

// CacheEntry represents a cached LSP response with metadata
type CacheEntry struct {
	Key        CacheKey    `json:"key"`
	Response   interface{} `json:"response"`
	Timestamp  time.Time   `json:"timestamp"`
	AccessedAt time.Time   `json:"accessed_at"`
	Size       int64       `json:"size"`
}

// SimpleCacheStats represents basic cache statistics
type SimpleCacheStats struct {
	HitCount   int64 `json:"hit_count"`
	MissCount  int64 `json:"miss_count"`
	ErrorCount int64 `json:"error_count"`
	TotalSize  int64 `json:"total_size"`
	EntryCount int64 `json:"entry_count"`
}

// CacheMetrics represents cache performance metrics (compatibility alias)
type CacheMetrics struct {
	HitCount        int64         `json:"hit_count"`
	MissCount       int64         `json:"miss_count"`
	ErrorCount      int64         `json:"error_count"`
	EvictionCount   int64         `json:"eviction_count"`
	TotalSize       int64         `json:"total_size"`
	EntryCount      int64         `json:"entry_count"`
	AverageHitTime  time.Duration `json:"average_hit_time"`
	AverageMissTime time.Duration `json:"average_miss_time"`
	HealthStatus    string        `json:"health_status"`     // Compatibility field
	LastHealthCheck time.Time     `json:"last_health_check"` // Compatibility field
}

// SCIPStorage interface for cache storage operations
type SCIPStorage interface {
	Store(key CacheKey, entry *CacheEntry) error
	Retrieve(key CacheKey) (*CacheEntry, error)
	Delete(key CacheKey) error
	Clear() error
	Size() int64
	EntryCount() int64
}

// SCIPQuery interface for cache querying operations
type SCIPQuery interface {
	BuildKey(method string, params interface{}) (CacheKey, error)
	IsValidEntry(entry *CacheEntry, ttl time.Duration) bool
	ExtractURI(params interface{}) (string, error)
}

// SCIPInvalidation interface for cache invalidation operations
type SCIPInvalidation interface {
	InvalidateDocument(uri string) error
	InvalidateSymbol(symbolID string) error
	InvalidatePattern(pattern string) error
	SetupFileWatcher(projectRoot string) error
	GetDependencies(uri string) ([]string, error)
	CascadeInvalidate(uris []string) error
}

// SCIPCache interface for the main cache operations (using unified config)
type SCIPCache interface {
	Initialize(config *config.CacheConfig) error
	Start(ctx context.Context) error
	Stop() error
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	HealthCheck() (*CacheMetrics, error)
	GetMetrics() *CacheMetrics
}

// SimpleCacheManager implements simple in-memory cache with unified config
type SimpleCacheManager struct {
	entries map[string]*CacheEntry
	config  *config.CacheConfig
	stats   *SimpleCacheStats
	mu      sync.RWMutex
	enabled bool
	started bool
}

// NewSCIPCacheManager creates a simple cache manager with unified config
func NewSCIPCacheManager(configParam *config.CacheConfig) (*SimpleCacheManager, error) {
	if configParam == nil {
		configParam = config.GetDefaultCacheConfig()
	}

	manager := &SimpleCacheManager{
		entries: make(map[string]*CacheEntry),
		config:  configParam,
		stats:   &SimpleCacheStats{},
		enabled: configParam.Enabled,
	}

	// Auto-initialize on creation - no separate Initialize() call needed
	if manager.enabled {
		common.LSPLogger.Info("Simple cache manager auto-initialized with max_memory=%d MB", configParam.MaxMemoryMB)
	} else {
		common.LSPLogger.Info("Simple cache manager created but disabled")
	}

	return manager, nil
}

// NewSimpleCache creates a cache manager with basic MB-based configuration using unified config
func NewSimpleCache(maxMemoryMB int) (*SimpleCacheManager, error) {
	if maxMemoryMB <= 0 {
		maxMemoryMB = 256 // Default 256MB
	}

	config := &config.CacheConfig{
		Enabled:            true,
		MaxMemoryMB:        maxMemoryMB,
		TTLHours:           24, // 24 hour default
		EvictionPolicy:     "simple",
		BackgroundIndex:    false,
		HealthCheckMinutes: 5,
		DiskCache:          false,
	}

	return NewSCIPCacheManager(config)
}

// Initialize - DEPRECATED: Auto-initialization happens in constructor (simplified initialization)
func (m *SimpleCacheManager) Initialize(config *config.CacheConfig) error {
	common.LSPLogger.Warn("Initialize() is deprecated - cache auto-initializes in constructor")
	return nil // No-op - initialization happens in NewSCIPCacheManager
}

// Start begins cache operations
func (m *SimpleCacheManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cache manager already started")
	}

	if !m.enabled {
		common.LSPLogger.Info("Simple cache is disabled, skipping start")
		return nil
	}

	m.started = true
	common.LSPLogger.Info("Simple cache manager started successfully")
	return nil
}

// Stop gracefully shuts down the cache manager
func (m *SimpleCacheManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	m.started = false
	common.LSPLogger.Info("Simple cache manager stopped successfully")
	return nil
}

// Lookup retrieves a cached response if available
func (m *SimpleCacheManager) Lookup(method string, params interface{}) (interface{}, bool, error) {
	if !m.isEnabled() {
		return nil, false, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build cache key
	key, err := m.buildKey(method, params)
	if err != nil {
		m.stats.ErrorCount++
		common.LSPLogger.Error("Failed to build cache key: %v", err)
		return nil, false, nil
	}

	// Retrieve from memory
	entry, exists := m.entries[key]
	if !exists {
		m.stats.MissCount++
		return nil, false, nil
	}

	// Validate entry freshness (convert hours to duration)
	if !m.isValidEntry(entry, time.Duration(m.config.TTLHours)*time.Hour) {
		m.stats.MissCount++
		// Remove expired entry
		delete(m.entries, key)
		m.updateStats()
		return nil, false, nil
	}

	// Update access time
	entry.AccessedAt = time.Now()
	m.stats.HitCount++
	common.LSPLogger.Debug("Cache hit for method=%s", method)

	return entry.Response, true, nil
}

// Store caches an LSP response
func (m *SimpleCacheManager) Store(method string, params interface{}, response interface{}) error {
	if !m.isEnabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Build cache key
	keyStr, err := m.buildKey(method, params)
	if err != nil {
		m.stats.ErrorCount++
		common.LSPLogger.Error("Failed to build cache key for storage: %v", err)
		return nil
	}

	// Calculate entry size
	data, err := json.Marshal(response)
	if err != nil {
		m.stats.ErrorCount++
		common.LSPLogger.Error("Failed to marshal response for caching: %v", err)
		return nil
	}

	now := time.Now()
	entry := &CacheEntry{
		Key: CacheKey{
			Method: method,
			URI:    m.extractURI(params),
			Hash:   keyStr,
		},
		Response:   response,
		Timestamp:  now,
		AccessedAt: now,
		Size:       int64(len(data)),
	}

	// Check if we need to evict entries (convert MB to bytes)
	maxSizeBytes := int64(m.config.MaxMemoryMB) * 1024 * 1024
	if m.getTotalSize()+entry.Size > maxSizeBytes {
		common.LSPLogger.Debug("Cache size limit reached, performing simple eviction")
		m.performSimpleEviction()
	}

	// Store the entry
	m.entries[keyStr] = entry
	m.updateStats()

	common.LSPLogger.Debug("Cached response for method=%s, size=%d bytes", method, entry.Size)
	return nil
}

// InvalidateDocument removes all cached entries for a specific document
func (m *SimpleCacheManager) InvalidateDocument(uri string) error {
	if !m.isEnabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	common.LSPLogger.Info("Invalidating cache for document: %s", uri)

	// Remove all entries with matching URI
	var removedCount int
	for key, entry := range m.entries {
		if entry.Key.URI == uri {
			delete(m.entries, key)
			removedCount++
		}
	}

	m.updateStats()
	common.LSPLogger.Debug("Invalidated %d cache entries for document: %s", removedCount, uri)
	return nil
}

// HealthCheck returns current cache health metrics
func (m *SimpleCacheManager) HealthCheck() (*CacheMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return &CacheMetrics{}, nil
	}

	// Convert simple stats to CacheMetrics format
	metrics := &CacheMetrics{
		HitCount:        m.stats.HitCount,
		MissCount:       m.stats.MissCount,
		ErrorCount:      m.stats.ErrorCount,
		TotalSize:       m.stats.TotalSize,
		EntryCount:      m.stats.EntryCount,
		EvictionCount:   0, // Not tracked in simple version
		HealthStatus:    "healthy",
		LastHealthCheck: time.Now(),
	}

	return metrics, nil
}

// GetMetrics returns current cache metrics
func (m *SimpleCacheManager) GetMetrics() *CacheMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Convert simple stats to CacheMetrics format
	metrics := &CacheMetrics{
		HitCount:        m.stats.HitCount,
		MissCount:       m.stats.MissCount,
		ErrorCount:      m.stats.ErrorCount,
		TotalSize:       m.stats.TotalSize,
		EntryCount:      m.stats.EntryCount,
		EvictionCount:   0, // Not tracked in simple version
		HealthStatus:    "healthy",
		LastHealthCheck: time.Now(),
	}

	return metrics
}

// SetInvalidation sets the invalidation component (compatibility method)
func (m *SimpleCacheManager) SetInvalidation(invalidation SCIPInvalidation) {
	// No-op for simple cache - invalidation is handled directly
}

// isEnabled checks if cache is enabled and properly initialized
func (m *SimpleCacheManager) isEnabled() bool {
	return m.enabled && m.started
}

// IsEnabled returns simple enabled/disabled status of the cache
func (m *SimpleCacheManager) IsEnabled() bool {
	return m.enabled && m.started
}

// Helper methods for simple cache implementation

// buildKey creates a cache key from method and parameters
func (m *SimpleCacheManager) buildKey(method string, params interface{}) (string, error) {
	// Create hash of method and parameters
	data, err := json.Marshal(map[string]interface{}{
		"method": method,
		"params": params,
	})
	if err != nil {
		return "", err
	}

	// Create SHA256 hash
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// extractURI extracts URI from parameters for invalidation
func (m *SimpleCacheManager) extractURI(params interface{}) string {
	uri, _ := ExtractURIFromParams("", params)
	return uri
}

// isValidEntry checks if cache entry is still valid based on TTL
func (m *SimpleCacheManager) isValidEntry(entry *CacheEntry, ttl time.Duration) bool {
	if entry == nil {
		return false
	}
	return time.Since(entry.Timestamp) < ttl
}

// getTotalSize calculates total size of all cache entries
func (m *SimpleCacheManager) getTotalSize() int64 {
	var total int64
	for _, entry := range m.entries {
		total += entry.Size
	}
	return total
}

// updateStats recalculates cache statistics
func (m *SimpleCacheManager) updateStats() {
	m.stats.TotalSize = m.getTotalSize()
	m.stats.EntryCount = int64(len(m.entries))
}

// performSimpleEviction removes oldest entries when cache is full
func (m *SimpleCacheManager) performSimpleEviction() {
	if len(m.entries) == 0 {
		return
	}

	// Find oldest entry
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range m.entries {
		if first || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
			first = false
		}
	}

	// Remove oldest entry
	if oldestKey != "" {
		delete(m.entries, oldestKey)
		common.LSPLogger.Debug("Evicted oldest cache entry: %s", oldestKey)
	}
}

// Compatibility stubs for removed enterprise features

// CacheWarmupManager provides compatibility stub for cache warmup (no-op in simple version)
type CacheWarmupManager struct{}

// NewCacheWarmupManager creates a new cache warmup manager (no-op stub)
func NewCacheWarmupManager(manager *SimpleCacheManager) *CacheWarmupManager {
	return &CacheWarmupManager{}
}

// Start is a no-op for simple cache
func (w *CacheWarmupManager) Start() error {
	return nil
}

// Stop is a no-op for simple cache
func (w *CacheWarmupManager) Stop() error {
	return nil
}

// NewCachedLSPManager creates a new cached LSP manager with unified config (compatibility stub)
func NewCachedLSPManager(lspManager interface{}) *SimpleCacheManager {
	manager, _ := NewSCIPCacheManager(config.GetDefaultCacheConfig())
	return manager
}
