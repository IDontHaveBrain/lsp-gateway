package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/scip"
)

// SCIP Integration Module
// This module handles SCIP-specific operations, storage integration,
// and the main cache manager construction with SCIP storage setup.

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
