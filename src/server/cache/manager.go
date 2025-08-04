package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"os"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
)

// SCIPConfig represents configuration for the SCIP cache system
type SCIPConfig struct {
	Enabled        bool          `yaml:"enabled" json:"enabled"`
	MaxSize        int64         `yaml:"max_size" json:"max_size"`
	TTL            time.Duration `yaml:"ttl" json:"ttl"`
	EvictionPolicy string        `yaml:"eviction_policy" json:"eviction_policy"`
	BackgroundSync bool          `yaml:"background_sync" json:"background_sync"`
	HealthCheckTTL time.Duration `yaml:"health_check_ttl" json:"health_check_ttl"`
}

// DefaultSCIPConfig returns default configuration for SCIP cache
func DefaultSCIPConfig() *SCIPConfig {
	return &SCIPConfig{
		Enabled:        true,
		MaxSize:        100 * 1024 * 1024, // 100MB
		TTL:            30 * time.Minute,
		EvictionPolicy: "lru",
		BackgroundSync: true,
		HealthCheckTTL: 5 * time.Minute,
	}
}

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

// CacheMetrics represents cache performance metrics
type CacheMetrics struct {
	HitCount        int64         `json:"hit_count"`
	MissCount       int64         `json:"miss_count"`
	ErrorCount      int64         `json:"error_count"`
	EvictionCount   int64         `json:"eviction_count"`
	TotalSize       int64         `json:"total_size"`
	EntryCount      int64         `json:"entry_count"`
	AverageHitTime  time.Duration `json:"average_hit_time"`
	AverageMissTime time.Duration `json:"average_miss_time"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	HealthStatus    string        `json:"health_status"`
}

// HealthLevel represents cache system health status
type HealthLevel int

const (
	HealthOK HealthLevel = iota
	HealthDegraded
	HealthFailing
	HealthCritical
)

var healthLevelNames = map[HealthLevel]string{
	HealthOK:       "OK",
	HealthDegraded: "DEGRADED",
	HealthFailing:  "FAILING",
	HealthCritical: "CRITICAL",
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

// SCIPCache interface for the main cache operations
type SCIPCache interface {
	Initialize(config *SCIPConfig) error
	Start(ctx context.Context) error
	Stop() error
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	HealthCheck() (*CacheMetrics, error)
	GetMetrics() *CacheMetrics
}

// SCIPCacheManager implements the main cache orchestration
type SCIPCacheManager struct {
	storage       SCIPStorage
	query         SCIPQuery
	invalidation  SCIPInvalidation
	config        *SCIPConfig
	metrics       *CacheMetrics
	healthChecker *HealthChecker

	// Manager lifecycle fields
	mu      sync.RWMutex
	started bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Circuit breaker for error recovery
	circuitBreaker *CircuitBreaker
}

// CircuitBreaker implements basic circuit breaker pattern for cache operations
type CircuitBreaker struct {
	failures    int64
	lastFailure time.Time
	state       CircuitState
	threshold   int64
	timeout     time.Duration
	mu          sync.RWMutex
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// HealthChecker monitors cache system health
type HealthChecker struct {
	level     HealthLevel
	lastCheck time.Time
	errors    []error
	mu        sync.RWMutex
}

// NewSCIPCacheManager creates a new SCIP cache manager
func NewSCIPCacheManager(config *SCIPConfig) (*SCIPCacheManager, error) {
	if config == nil {
		config = DefaultSCIPConfig()
	}

	// Initialize circuit breaker
	circuitBreaker := &CircuitBreaker{
		threshold: 5,
		timeout:   60 * time.Second,
		state:     CircuitClosed,
	}

	// Initialize health checker
	healthChecker := &HealthChecker{
		level:     HealthOK,
		lastCheck: time.Now(),
		errors:    make([]error, 0),
	}

	// Initialize metrics
	metrics := &CacheMetrics{
		HealthStatus:    healthLevelNames[HealthOK],
		LastHealthCheck: time.Now(),
	}

	manager := &SCIPCacheManager{
		config:         config,
		metrics:        metrics,
		healthChecker:  healthChecker,
		circuitBreaker: circuitBreaker,
	}

	return manager, nil
}

// Initialize sets up the cache components
func (m *SCIPCacheManager) Initialize(config *SCIPConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config != nil {
		m.config = config
	}

	if !m.config.Enabled {
		common.LSPLogger.Info("SCIP cache is disabled by configuration")
		return nil
	}

	common.LSPLogger.Info("Initializing SCIP cache manager with config: max_size=%d, ttl=%v",
		m.config.MaxSize, m.config.TTL)

	// Initialize storage component with persistent SCIP storage
	storageConfig := convertToSCIPStorageConfig(m.config)
	scipStorage, err := NewSCIPStorageManager(storageConfig)
	if err != nil {
		return fmt.Errorf("failed to create SCIP storage: %w", err)
	}
	m.storage = scipStorage

	// Initialize query component
	m.query = NewSCIPQueryWrapper()

	// Initialize invalidation component - will be set via SetInvalidation method

	common.LSPLogger.Info("SCIP cache manager initialized successfully with persistent storage")
	return nil
}

// Start begins cache background processing
func (m *SCIPCacheManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cache manager already started")
	}

	if !m.config.Enabled {
		common.LSPLogger.Info("SCIP cache is disabled, skipping start")
		return nil
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true

	common.LSPLogger.Info("Starting SCIP cache manager")

	// Start persistent storage if available
	if scipStorage, ok := m.storage.(*SCIPStorageWrapper); ok {
		if err := scipStorage.storage.Start(ctx); err != nil {
			return fmt.Errorf("failed to start SCIP storage: %w", err)
		}
		common.LSPLogger.Info("SCIP persistent storage started successfully")
	}

	// Start background health monitoring
	if m.config.BackgroundSync {
		go m.backgroundHealthCheck()
	}

	// Start background cleanup tasks
	go m.backgroundCleanup()

	common.LSPLogger.Info("SCIP cache manager started successfully")
	return nil
}

// Stop gracefully shuts down the cache manager
func (m *SCIPCacheManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	if !m.config.Enabled {
		return nil
	}

	common.LSPLogger.Info("Stopping SCIP cache manager")

	// Stop persistent storage if available
	if scipStorage, ok := m.storage.(*SCIPStorageWrapper); ok {
		if err := scipStorage.storage.Stop(context.Background()); err != nil {
			common.LSPLogger.Error("Error stopping SCIP storage: %v", err)
		} else {
			common.LSPLogger.Info("SCIP persistent storage stopped successfully")
		}
	}

	if m.cancel != nil {
		m.cancel()
	}

	m.started = false

	// Perform final metrics collection
	m.updateHealthStatus()

	common.LSPLogger.Info("SCIP cache manager stopped successfully")
	return nil
}

// Lookup retrieves a cached response if available
func (m *SCIPCacheManager) Lookup(method string, params interface{}) (interface{}, bool, error) {
	if !m.isEnabled() {
		return nil, false, nil
	}

	if !m.circuitBreaker.CanExecute() {
		m.metrics.ErrorCount++
		return nil, false, fmt.Errorf("cache circuit breaker is open")
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		if m.metrics.AverageHitTime == 0 {
			m.metrics.AverageHitTime = duration
		} else {
			m.metrics.AverageHitTime = (m.metrics.AverageHitTime + duration) / 2
		}
	}()

	// Check if storage is available
	if m.storage == nil {
		m.metrics.MissCount++
		return nil, false, nil
	}

	// Build cache key
	if m.query == nil {
		m.metrics.MissCount++
		return nil, false, nil
	}

	key, err := m.query.BuildKey(method, params)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		m.metrics.ErrorCount++
		common.LSPLogger.Error("Failed to build cache key: %v", err)
		return nil, false, nil
	}

	// Retrieve from storage
	entry, err := m.storage.Retrieve(key)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		m.metrics.ErrorCount++
		m.metrics.MissCount++
		common.LSPLogger.Debug("Cache miss for method=%s: %v", method, err)
		return nil, false, nil
	}

	if entry == nil {
		m.metrics.MissCount++
		return nil, false, nil
	}

	// Validate entry freshness
	if !m.query.IsValidEntry(entry, m.config.TTL) {
		m.metrics.MissCount++
		// Schedule cleanup
		go func() {
			if err := m.storage.Delete(key); err != nil {
				common.LSPLogger.Error("Failed to delete expired cache entry: %v", err)
			}
		}()
		return nil, false, nil
	}

	// Update access time
	entry.AccessedAt = time.Now()
	if err := m.storage.Store(key, entry); err != nil {
		common.LSPLogger.Error("Failed to update cache entry access time: %v", err)
	}

	m.circuitBreaker.RecordSuccess()
	m.metrics.HitCount++
	common.LSPLogger.Debug("Cache hit for method=%s", method)

	return entry.Response, true, nil
}

// Store caches an LSP response
func (m *SCIPCacheManager) Store(method string, params interface{}, response interface{}) error {
	if !m.isEnabled() {
		return nil
	}

	if !m.circuitBreaker.CanExecute() {
		m.metrics.ErrorCount++
		return fmt.Errorf("cache circuit breaker is open")
	}

	// Check if storage is available
	if m.storage == nil || m.query == nil {
		return nil
	}

	// Build cache key
	key, err := m.query.BuildKey(method, params)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		m.metrics.ErrorCount++
		common.LSPLogger.Error("Failed to build cache key for storage: %v", err)
		return nil
	}

	// Calculate entry size
	data, err := json.Marshal(response)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		m.metrics.ErrorCount++
		common.LSPLogger.Error("Failed to marshal response for caching: %v", err)
		return nil
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:        key,
		Response:   response,
		Timestamp:  now,
		AccessedAt: now,
		Size:       int64(len(data)),
	}

	// Check if we need to evict entries
	if m.storage.Size()+entry.Size > m.config.MaxSize {
		common.LSPLogger.Debug("Cache size limit reached, eviction needed")
		// Trigger background eviction (placeholder for now)
		go m.performEviction()
	}

	// Store the entry
	if err := m.storage.Store(key, entry); err != nil {
		m.circuitBreaker.RecordFailure()
		m.metrics.ErrorCount++
		common.LSPLogger.Error("Failed to store cache entry: %v", err)
		return err
	}

	m.circuitBreaker.RecordSuccess()
	m.metrics.TotalSize = m.storage.Size()
	m.metrics.EntryCount = m.storage.EntryCount()

	common.LSPLogger.Debug("Cached response for method=%s, size=%d bytes", method, entry.Size)
	return nil
}

// InvalidateDocument removes all cached entries for a specific document
func (m *SCIPCacheManager) InvalidateDocument(uri string) error {
	if !m.isEnabled() {
		return nil
	}

	if m.invalidation == nil {
		return nil
	}

	common.LSPLogger.Info("Invalidating cache for document: %s", uri)

	if err := m.invalidation.InvalidateDocument(uri); err != nil {
		m.metrics.ErrorCount++
		common.LSPLogger.Error("Failed to invalidate document cache: %v", err)
		return err
	}

	m.metrics.TotalSize = m.storage.Size()
	m.metrics.EntryCount = m.storage.EntryCount()

	return nil
}

// HealthCheck returns current cache health metrics
func (m *SCIPCacheManager) HealthCheck() (*CacheMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		return &CacheMetrics{
			HealthStatus: "DISABLED",
		}, nil
	}

	m.updateHealthStatus()

	// Update current metrics
	if m.storage != nil {
		m.metrics.TotalSize = m.storage.Size()
		m.metrics.EntryCount = m.storage.EntryCount()
	}

	m.metrics.LastHealthCheck = time.Now()

	return m.metrics, nil
}

// GetMetrics returns current cache metrics
func (m *SCIPCacheManager) GetMetrics() *CacheMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modifications
	metricsCopy := *m.metrics
	return &metricsCopy
}

// SetInvalidation sets the invalidation component
func (m *SCIPCacheManager) SetInvalidation(invalidation SCIPInvalidation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidation = invalidation
}

// isEnabled checks if cache is enabled and properly initialized
func (m *SCIPCacheManager) isEnabled() bool {
	return m.config != nil && m.config.Enabled && m.started
}

// updateHealthStatus evaluates and updates cache health status
func (m *SCIPCacheManager) updateHealthStatus() {
	m.healthChecker.mu.Lock()
	defer m.healthChecker.mu.Unlock()

	now := time.Now()
	totalRequests := m.metrics.HitCount + m.metrics.MissCount

	var newLevel HealthLevel = HealthOK

	// Determine health level based on metrics
	if totalRequests > 0 {
		errorRate := float64(m.metrics.ErrorCount) / float64(totalRequests)

		switch {
		case errorRate > 0.5:
			newLevel = HealthCritical
		case errorRate > 0.2:
			newLevel = HealthFailing
		case errorRate > 0.1:
			newLevel = HealthDegraded
		}
	}

	// Check circuit breaker state
	if m.circuitBreaker.state == CircuitOpen {
		if newLevel < HealthFailing {
			newLevel = HealthFailing
		}
	}

	m.healthChecker.level = newLevel
	m.healthChecker.lastCheck = now
	m.metrics.HealthStatus = healthLevelNames[newLevel]

	if newLevel != HealthOK {
		common.LSPLogger.Warn("Cache health degraded to %s (error rate: %.2f%%)",
			healthLevelNames[newLevel],
			float64(m.metrics.ErrorCount)/float64(totalRequests)*100)
	}
}

// backgroundHealthCheck performs periodic health monitoring
func (m *SCIPCacheManager) backgroundHealthCheck() {
	ticker := time.NewTicker(m.config.HealthCheckTTL)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if _, err := m.HealthCheck(); err != nil {
				common.LSPLogger.Error("Background health check failed: %v", err)
			}
		}
	}
}

// backgroundCleanup performs periodic cache cleanup
func (m *SCIPCacheManager) backgroundCleanup() {
	ticker := time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performCleanup()
		}
	}
}

// performEviction removes entries to make space (placeholder implementation)
func (m *SCIPCacheManager) performEviction() {
	if m.storage == nil {
		return
	}

	common.LSPLogger.Debug("Performing cache eviction")
	m.metrics.EvictionCount++

	// Actual eviction logic would be implemented in storage component
	// For now, just log the action
}

// performCleanup removes expired entries
func (m *SCIPCacheManager) performCleanup() {
	if !m.isEnabled() {
		return
	}

	common.LSPLogger.Debug("Performing cache cleanup")

	// Actual cleanup logic would iterate through storage and remove expired entries
	// For now, just update metrics
	if m.storage != nil {
		m.metrics.TotalSize = m.storage.Size()
		m.metrics.EntryCount = m.storage.EntryCount()
	}
}

// Circuit breaker methods

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int64, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     CircuitClosed,
	}
}

// CanExecute checks if operations can be executed
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = CircuitClosed
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = CircuitOpen
	}
}

// Helper functions for SCIP storage integration

// convertToSCIPStorageConfig converts cache config to SCIP storage config
func convertToSCIPStorageConfig(cacheConfig *SCIPConfig) scip.SCIPStorageConfig {
	// Default cache directory
	cacheDir := filepath.Join(os.TempDir(), "lsp-gateway-scip-cache")
	if homeDir, err := os.UserHomeDir(); err == nil {
		cacheDir = filepath.Join(homeDir, ".lsp-gateway", "scip-cache")
	}

	return scip.SCIPStorageConfig{
		MemoryLimit:        cacheConfig.MaxSize,
		DiskCacheDir:       cacheDir,
		CompressionType:    "gzip",
		CompactionInterval: 30 * time.Minute,
		MaxDocumentAge:     24 * time.Hour,
		EnableMetrics:      true,
	}
}

// NewSCIPStorageManager creates a new SCIP storage manager wrapper
func NewSCIPStorageManager(config scip.SCIPStorageConfig) (SCIPStorage, error) {
	storage, err := scip.NewSCIPStorageManager(config)
	if err != nil {
		return nil, err
	}
	return &SCIPStorageWrapper{storage: storage}, nil
}

// NewSCIPQueryWrapper creates a new SCIP query wrapper for manager integration
func NewSCIPQueryWrapper() SCIPQuery {
	return &SCIPQueryWrapper{}
}

// SCIPStorageWrapper adapts scip.SCIPStorageManager to cache.SCIPStorage interface
type SCIPStorageWrapper struct {
	storage *scip.SCIPStorageManager
}

func (w *SCIPStorageWrapper) Store(key CacheKey, entry *CacheEntry) error {
	// Convert cache entry to SCIP document format
	doc := &scip.SCIPDocument{
		URI:          key.URI,
		Content:      []byte{}, // Empty content for cache entries
		Symbols:      []scip.SCIPSymbol{},
		References:   []scip.SCIPReference{},
		LastModified: entry.Timestamp,
		Size:         entry.Size,
	}
	
	return w.storage.StoreDocument(context.Background(), doc)
}

func (w *SCIPStorageWrapper) Retrieve(key CacheKey) (*CacheEntry, error) {
	doc, err := w.storage.GetDocument(context.Background(), key.URI)
	if err != nil {
		return nil, err
	}
	
	if doc == nil {
		return nil, nil
	}
	
	// Convert SCIP document back to cache entry
	entry := &CacheEntry{
		Key:        key,
		Response:   nil, // Response data would need separate storage
		Timestamp:  doc.LastModified,
		AccessedAt: doc.LastModified,
		Size:       doc.Size,
	}
	
	return entry, nil
}

func (w *SCIPStorageWrapper) Delete(key CacheKey) error {
	return w.storage.RemoveDocument(context.Background(), key.URI)
}

func (w *SCIPStorageWrapper) Clear() error {
	// Implementation would require iterating through all documents
	return nil // Placeholder
}

func (w *SCIPStorageWrapper) Size() int64 {
	stats, err := w.storage.GetStats(context.Background())
	if err != nil {
		return 0
	}
	return stats.MemoryUsage + stats.DiskUsage
}

func (w *SCIPStorageWrapper) EntryCount() int64 {
	stats, err := w.storage.GetStats(context.Background())
	if err != nil {
		return 0
	}
	return int64(stats.CachedDocuments)
}

// SCIPQueryWrapper implements SCIPQuery interface
type SCIPQueryWrapper struct{}

func (q *SCIPQueryWrapper) BuildKey(method string, params interface{}) (CacheKey, error) {
	// Extract URI from parameters
	uri, err := q.ExtractURI(params)
	if err != nil {
		return CacheKey{}, err
	}
	
	// Create hash of parameters
	data, err := json.Marshal(params)
	if err != nil {
		return CacheKey{}, err
	}
	
	// Simple hash - in production would use proper hash function
	hash := fmt.Sprintf("%x", len(data))
	
	return CacheKey{
		Method: method,
		URI:    uri,
		Hash:   hash,
	}, nil
}

func (q *SCIPQueryWrapper) IsValidEntry(entry *CacheEntry, ttl time.Duration) bool {
	if entry == nil {
		return false
	}
	return time.Since(entry.Timestamp) < ttl
}

func (q *SCIPQueryWrapper) ExtractURI(params interface{}) (string, error) {
	// Try to extract URI from common LSP parameter types
	if data, err := json.Marshal(params); err == nil {
		var parsed struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(data, &parsed); err == nil {
			if parsed.TextDocument.URI != "" {
				return parsed.TextDocument.URI, nil
			}
		}
	}
	
	return "", fmt.Errorf("unable to extract URI from parameters")
}
