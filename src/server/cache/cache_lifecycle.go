package cache

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"lsp-gateway/src/internal/common"
)

const errStorageAlreadyStarted = "storage already started"

// Lifecycle Management Module
// This module handles cache lifecycle operations including start/stop,
// health monitoring, metrics reporting, and enabled state management.

// Start begins cache operations
func (m *SCIPCacheManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentState := m.getState()

	if currentState == CacheRunning {
		return fmt.Errorf("cache manager already started")
	}

	if currentState == CacheDisabled {
		common.LSPLogger.Debug("Cache is disabled, skipping start")
		return nil
	}

	if err := m.scipStorage.Start(ctx); err != nil && err.Error() != errStorageAlreadyStarted {
		return fmt.Errorf("failed to start SCIP storage: %w", err)
	}

	// Load index from disk if available
	if m.config.StoragePath != "" {
		if err := m.LoadIndexFromDisk(); err != nil {
			common.LSPLogger.Warn("Failed to load index from disk, starting with empty cache: %v", err)
		}
		// Load file tracker metadata
		metadataPath := filepath.Join(m.config.StoragePath, "file_metadata.json")
		if err := m.fileTracker.LoadFromFile(metadataPath); err != nil {
			common.LSPLogger.Warn("Failed to load file metadata, continuing with empty tracker: %v", err)
		}
	}

	m.setState(CacheRunning)
	return nil
}

// Stop gracefully shuts down the cache manager
func (m *SCIPCacheManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.getState() != CacheRunning {
		return nil
	}

	var errs []error

	// Save file tracker metadata before stopping
	if m.config.StoragePath != "" {
		metadataPath := filepath.Join(m.config.StoragePath, "file_metadata.json")
		if err := m.fileTracker.SaveToFile(metadataPath); err != nil {
			errs = append(errs, fmt.Errorf("save metadata: %w", err))
		}
	}

	if err := m.scipStorage.Stop(context.Background()); err != nil {
		errs = append(errs, fmt.Errorf("stop storage: %w", err))
	}

	m.setState(CacheEnabled)

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// HealthCheck returns current cache health metrics
func (m *SCIPCacheManager) HealthCheck() (*CacheMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.isDisabled() {
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

	if m.isDisabled() {
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

// IsEnabled returns simple enabled/disabled status of the cache
func (m *SCIPCacheManager) IsEnabled() bool {
	return m.isRunning()
}
