package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
)

// SCIPCacheManager implements simple in-memory cache with occurrence-centric SCIP storage
type SCIPCacheManager struct {
	entries     map[string]*CacheEntry
	config      *config.CacheConfig
	stats       *SimpleCacheStats
	mu          sync.RWMutex
	enabled     bool
	started     bool
	scipStorage scip.SCIPDocumentStorage
	indexStats  *IndexStats
	indexMu     sync.RWMutex
	converter   *SCIPConverter
	fileTracker *FileChangeTracker
}

// NewSCIPCacheManager creates a simple cache manager with unified config
func NewSCIPCacheManager(configParam *config.CacheConfig) (*SCIPCacheManager, error) {
	if configParam == nil {
		configParam = config.GetDefaultCacheConfig()
	}

	scipConfig := scip.SCIPStorageConfig{
		MemoryLimit:        int64(configParam.MaxMemoryMB) * 1024 * 1024,
		DiskCacheDir:       configParam.StoragePath,
		EnableMetrics:      true,
		MaxDocumentAge:     time.Duration(configParam.TTLHours) * time.Hour,
		CompactionInterval: 5 * time.Minute,
	}

	scipStorage, err := scip.NewSimpleSCIPStorage(scipConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SCIP storage: %w", err)
	}

	manager := &SCIPCacheManager{
		entries:     make(map[string]*CacheEntry),
		config:      configParam,
		stats:       &SimpleCacheStats{},
		enabled:     configParam.Enabled,
		scipStorage: scipStorage,
		indexStats: &IndexStats{
			DocumentCount:    0,
			SymbolCount:      0,
			IndexSize:        0,
			LastUpdate:       time.Time{},
			LanguageStats:    make(map[string]int64),
			IndexedLanguages: []string{},
			Status:           "initialized",
		},
		fileTracker: NewFileChangeTracker(),
	}

	// Initialize the generic SCIP converter with package detection
	manager.converter = NewSCIPConverter(manager.detectPackageInfo)

	if manager.enabled {
		if err := manager.scipStorage.Start(context.Background()); err != nil && err.Error() != "storage already started" {
			return nil, fmt.Errorf("failed to start SCIP storage: %w", err)
		}
	}

	return manager, nil
}

// Start begins cache operations
func (m *SCIPCacheManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cache manager already started")
	}

	if !m.enabled {
		common.LSPLogger.Debug("Cache is disabled, skipping start")
		return nil
	}

	if err := m.scipStorage.Start(ctx); err != nil && err.Error() != "storage already started" {
		common.LSPLogger.Warn("Failed to start SCIP storage: %v", err)
	}

	// Load index from disk if available
	if m.config.DiskCache && m.config.StoragePath != "" {
		if err := m.LoadIndexFromDisk(); err != nil {
			// Index will be built as needed
		}
		// Load file tracker metadata
		metadataPath := filepath.Join(m.config.StoragePath, "file_metadata.json")
		if err := m.fileTracker.LoadFromFile(metadataPath); err != nil {
			common.LSPLogger.Warn("Failed to load file metadata: %v", err)
		}
	}

	m.started = true
	return nil
}

// Stop gracefully shuts down the cache manager
func (m *SCIPCacheManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	// Save file tracker metadata before stopping
	if m.config.DiskCache && m.config.StoragePath != "" {
		metadataPath := filepath.Join(m.config.StoragePath, "file_metadata.json")
		if err := m.fileTracker.SaveToFile(metadataPath); err != nil {
			common.LSPLogger.Warn("Failed to save file metadata: %v", err)
		}
	}

	if err := m.scipStorage.Stop(context.Background()); err != nil {
		common.LSPLogger.Warn("Failed to stop SCIP storage: %v", err)
	}

	m.started = false
	return nil
}

// Lookup retrieves a cached response if available
func (m *SCIPCacheManager) Lookup(method string, params interface{}) (interface{}, bool, error) {
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

	return entry.Response, true, nil
}

// Store caches an LSP response
func (m *SCIPCacheManager) Store(method string, params interface{}, response interface{}) error {
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
		m.performSimpleEviction()
	}

	// Store the entry
	m.entries[keyStr] = entry
	m.updateStats()

	return nil
}

// Clear removes all cache entries
func (m *SCIPCacheManager) Clear() error {
	if !m.enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*CacheEntry)

	// Clear file tracker
	if m.fileTracker != nil {
		m.fileTracker.Clear()
	}

	if err := m.scipStorage.Stop(context.Background()); err != nil {
		common.LSPLogger.Debug("Failed to stop SCIP storage: %v", err)
	}

	// Delete all cache files in the storage path
	if m.config.StoragePath != "" {
		// Delete the main cache file
		cacheFile := filepath.Join(m.config.StoragePath, "simple_cache.json")
		if _, err := os.Stat(cacheFile); err == nil {
			if err := os.Remove(cacheFile); err != nil {
				common.LSPLogger.Error("Failed to remove persisted SCIP cache: %v", err)
			} else {
				common.LSPLogger.Debug("Successfully removed persisted SCIP cache file: %s", cacheFile)
			}
		}

		// Delete the entire storage directory and all its contents
		if err := os.RemoveAll(m.config.StoragePath); err != nil {
			common.LSPLogger.Error("Failed to remove SCIP cache directory: %v", err)
		} else {
			common.LSPLogger.Debug("Successfully removed SCIP cache directory: %s", m.config.StoragePath)
		}
	}

	// Create new SCIP storage but don't start it to avoid recreating directories
	// The storage will be started when the cache is used again
	scipConfig := scip.SCIPStorageConfig{
		DiskCacheDir: m.config.StoragePath,
		MemoryLimit:  int64(m.config.MaxMemoryMB) * 1024 * 1024,
	}
	if newStorage, err := scip.NewSimpleSCIPStorage(scipConfig); err == nil {
		m.scipStorage = newStorage
		// Don't start it here - this would recreate the directory
		// It will be started when needed
	} else {
		common.LSPLogger.Warn("Failed to recreate SCIP storage: %v", err)
	}

	m.indexStats = &IndexStats{
		Status:        "cleared",
		LanguageStats: make(map[string]int64),
		LastUpdate:    time.Now(),
		DocumentCount: 0,
		SymbolCount:   0,
		IndexSize:     0,
	}

	m.updateStats()
	return nil
}

// HealthCheck returns current cache health metrics
func (m *SCIPCacheManager) HealthCheck() (*CacheMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return &CacheMetrics{}, nil
	}

	// Convert simple stats to CacheMetrics format
	metrics := &CacheMetrics{
		HitCount:      m.stats.HitCount,
		MissCount:     m.stats.MissCount,
		ErrorCount:    m.stats.ErrorCount,
		TotalSize:     m.stats.TotalSize,
		EntryCount:    m.stats.EntryCount,
		EvictionCount: 0, // Not tracked in simple version
	}

	return metrics, nil
}

// GetMetrics returns current cache metrics
func (m *SCIPCacheManager) GetMetrics() *CacheMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return &CacheMetrics{}
	}

	// Convert simple stats to CacheMetrics format
	metrics := &CacheMetrics{
		HitCount:      m.stats.HitCount,
		MissCount:     m.stats.MissCount,
		ErrorCount:    m.stats.ErrorCount,
		TotalSize:     m.stats.TotalSize,
		EntryCount:    m.stats.EntryCount,
		EvictionCount: 0, // Not tracked in simple version
	}

	return metrics
}

// isEnabled checks if cache is enabled and properly initialized
func (m *SCIPCacheManager) isEnabled() bool {
	return m.enabled && m.started
}

// IsEnabled returns simple enabled/disabled status of the cache
func (m *SCIPCacheManager) IsEnabled() bool {
	return m.isEnabled()
}

// buildKey creates a cache key from method and parameters
func (m *SCIPCacheManager) buildKey(method string, params interface{}) (string, error) {
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
func (m *SCIPCacheManager) extractURI(params interface{}) string {
	uri, _ := ExtractURIFromParams("", params)
	return uri
}

// isValidEntry checks if cache entry is still valid based on TTL
func (m *SCIPCacheManager) isValidEntry(entry *CacheEntry, ttl time.Duration) bool {
	if entry == nil {
		return false
	}
	return time.Since(entry.Timestamp) < ttl
}

// getTotalSize calculates total size of all cache entries
func (m *SCIPCacheManager) getTotalSize() int64 {
	var total int64
	for _, entry := range m.entries {
		total += entry.Size
	}
	return total
}

// updateStats recalculates cache statistics
func (m *SCIPCacheManager) updateStats() {
	m.stats.TotalSize = m.getTotalSize()
	m.stats.EntryCount = int64(len(m.entries))
}

// performSimpleEviction removes oldest entries when cache is full
func (m *SCIPCacheManager) performSimpleEviction() {
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
	}
}

// GetSCIPStorage returns the underlying SCIP storage for direct access
func (m *SCIPCacheManager) GetSCIPStorage() scip.SCIPDocumentStorage {
	if m == nil || !m.enabled {
		return nil
	}
	return m.scipStorage
}

// GetTrackedFileCount returns the number of files tracked for incremental indexing
func (m *SCIPCacheManager) GetTrackedFileCount() int {
	if m == nil || m.fileTracker == nil {
		return 0
	}
	return m.fileTracker.GetIndexedFileCount()
}
