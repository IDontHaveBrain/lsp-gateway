package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache/search"
	"lsp-gateway/src/server/scip"
)

// SCIP Integration Module
// This module handles SCIP-specific operations, storage integration,
// and the main cache manager construction with SCIP storage setup.

// scipStorageAdapter adapts scip.SCIPDocumentStorage to search.StorageAccess interface
type scipStorageAdapter struct {
	storage scip.SCIPDocumentStorage
}

func (s *scipStorageAdapter) SearchSymbols(ctx context.Context, pattern string, maxResults int) ([]scip.SCIPSymbolInformation, error) {
	return s.storage.SearchSymbols(ctx, pattern, maxResults)
}

func (s *scipStorageAdapter) GetDefinitions(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error) {
	return s.storage.GetDefinitions(ctx, symbolID)
}

func (s *scipStorageAdapter) GetDefinitionsWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error) {
	return s.storage.GetDefinitionsWithDocuments(ctx, symbolID)
}

func (s *scipStorageAdapter) GetReferences(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error) {
	return s.storage.GetReferences(ctx, symbolID)
}

func (s *scipStorageAdapter) GetReferencesWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error) {
	return s.storage.GetReferencesWithDocuments(ctx, symbolID)
}

func (s *scipStorageAdapter) GetOccurrences(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error) {
	return s.storage.GetOccurrences(ctx, symbolID)
}

func (s *scipStorageAdapter) GetOccurrencesWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error) {
	return s.storage.GetOccurrencesWithDocuments(ctx, symbolID)
}

func (s *scipStorageAdapter) GetIndexStats() *scip.IndexStats {
	stats := s.storage.GetIndexStats()
	return &stats
}

func (s *scipStorageAdapter) ListDocuments(ctx context.Context) ([]string, error) {
	return s.storage.ListDocuments(ctx)
}

func (s *scipStorageAdapter) GetDocument(ctx context.Context, uri string) (*scip.SCIPDocument, error) {
	return s.storage.GetDocument(ctx, uri)
}

// SCIPCacheManager implements simple in-memory cache with occurrence-centric SCIP storage
type SCIPCacheManager struct {
	entries       map[string]*CacheEntry
	config        *config.CacheConfig
	stats         *SimpleCacheStats
	mu            sync.RWMutex
	enabled       bool
	started       bool
	scipStorage   scip.SCIPDocumentStorage
	indexStats    *IndexStats
	indexMu       sync.RWMutex
	converter     *SCIPConverter
	fileTracker   *FileChangeTracker
	searchService *search.SearchService
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

	// Initialize the search service with a storage adapter
	storageAdapter := &scipStorageAdapter{storage: scipStorage}
	searchConfig := &search.SearchServiceConfig{
		Storage:            storageAdapter,
		Enabled:            configParam.Enabled,
		IndexMutex:         &manager.indexMu,
		MatchFilePatternFn: manager.matchFilePattern,
		BuildOccurrenceInfoFn: func(occ *scip.SCIPOccurrence, docURI string) interface{} {
			return manager.buildOccurrenceInfo(occ, docURI)
		},
		FormatSymbolDetailFn: manager.formatSymbolDetail,
	}
	manager.searchService = search.NewSearchService(searchConfig)

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
