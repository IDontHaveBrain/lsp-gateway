package cache

import (
	"context"
	"fmt"
	"path/filepath"

	"lsp-gateway/src/internal/common"
)

// Lifecycle Management Module
// This module handles cache lifecycle operations including start/stop,
// health monitoring, metrics reporting, and enabled state management.

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
