package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
)

// Core Cache Operations Module
// This module handles the primary cache operations including lookup, store,
// clear functionality, and supporting utility methods for key generation,
// entry validation, size management, and eviction policies.

// Lookup retrieves a cached response if available
func (m *SCIPCacheManager) Lookup(method string, params interface{}) (interface{}, bool, error) {
	return m.WithManagerGuard(func() (interface{}, bool, error) {
		resultMap := m.WithCacheReadLock(func() interface{} {
			result, found := m.lookupInternal(method, params)
			return map[string]interface{}{
				"result": result,
				"found":  found,
			}
		}).(map[string]interface{})
		return resultMap["result"], resultMap["found"].(bool), nil
	})
}

// lookupInternal contains the actual lookup logic without guards or locks
func (m *SCIPCacheManager) lookupInternal(method string, params interface{}) (interface{}, bool) {

	// Build cache key
	key, err := m.buildKey(method, params)
	if err != nil {
		m.stats.ErrorCount++
		common.LSPLogger.Error("Failed to build cache key: %v", err)
		return nil, false
	}

	// Retrieve from memory
	entry, exists := m.entries[key]
	if !exists {
		m.stats.MissCount++
		return nil, false
	}

	// Validate entry freshness (convert hours to duration)
	if !m.isValidEntry(entry, time.Duration(m.config.TTLHours)*time.Hour) {
		m.stats.MissCount++
		// Remove expired entry
		delete(m.entries, key)
		m.updateStats()
		return nil, false
	}

	// Update access time
	entry.AccessedAt = time.Now()
	m.stats.HitCount++

	return entry.Response, true
}

// Store caches an LSP response
func (m *SCIPCacheManager) Store(method string, params interface{}, response interface{}) error {
	if !m.isEnabledAndStarted() {
		return nil
	}

	m.WithCacheWriteLockVoid(func() {
		m.storeInternal(method, params, response)
	})
	return nil
}

// storeInternal contains the actual store logic without guards or locks
func (m *SCIPCacheManager) storeInternal(method string, params interface{}, response interface{}) {

	// Build cache key
	keyStr, err := m.buildKey(method, params)
	if err != nil {
		m.stats.ErrorCount++
		common.LSPLogger.Error("Failed to build cache key for storage: %v", err)
		return
	}

	// Calculate entry size
	data, err := json.Marshal(response)
	if err != nil {
		m.stats.ErrorCount++
		common.LSPLogger.Error("Failed to marshal response for caching: %v", err)
		return
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
