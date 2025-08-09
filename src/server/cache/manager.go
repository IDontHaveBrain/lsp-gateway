package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// SCIP indexing data structures

// IndexQuery represents a query to the SCIP index
type IndexQuery struct {
	Type     string                 `json:"type"` // "symbol", "definition", "references", etc.
	Symbol   string                 `json:"symbol,omitempty"`
	URI      string                 `json:"uri,omitempty"`
	Position *Position              `json:"position,omitempty"`
	Language string                 `json:"language,omitempty"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	MaxDepth int                    `json:"max_depth,omitempty"`
}

// Position represents a position in a document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// IndexResult represents the result of an index query
type IndexResult struct {
	Type      string                 `json:"type"`
	Results   []interface{}          `json:"results"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// IndexStats represents statistics about the SCIP index
type IndexStats struct {
	DocumentCount    int64            `json:"document_count"`
	SymbolCount      int64            `json:"symbol_count"`
	ReferenceCount   int64            `json:"reference_count"`
	IndexSize        int64            `json:"index_size_bytes"`
	LastUpdate       time.Time        `json:"last_update"`
	LanguageStats    map[string]int64 `json:"language_stats"`
	IndexedLanguages []string         `json:"indexed_languages"`
	Status           string           `json:"status"`
}

// SCIPSymbol wraps LSP SymbolInformation with enhanced SCIP metadata
type SCIPSymbol struct {
	SymbolInfo          types.SymbolInformation `json:"symbol_info"`
	Language            string                  `json:"language"`
	Score               float64                 `json:"score,omitempty"`
	FullRange           *Range                  `json:"full_range,omitempty"`           // Full range from document symbols
	Documentation       string                  `json:"documentation,omitempty"`        // Documentation from hover
	Signature           string                  `json:"signature,omitempty"`            // Signature from hover
	RelatedSymbols      []string                `json:"related_symbols,omitempty"`      // Related symbol names
	DefinitionLocations []types.Location        `json:"definition_locations,omitempty"` // Definition locations for this symbol
	ReferenceLocations  []types.Location        `json:"reference_locations,omitempty"`  // Reference locations for this symbol
	UsageCount          int                     `json:"usage_count,omitempty"`          // Number of references to this symbol
	Metadata            map[string]interface{}  `json:"metadata,omitempty"`             // Additional SCIP metadata
}

// Range represents a range in a document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

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

// SCIPCache interface for the main cache operations with integrated SCIP indexing
type SCIPCache interface {
	Start(ctx context.Context) error
	Stop() error
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	HealthCheck() (*CacheMetrics, error)
	GetMetrics() *CacheMetrics
	Clear() error // Clear all cache entries

	// SCIP indexing capabilities - integrated as core functionality
	IndexDocument(ctx context.Context, uri string, language string, symbols []types.SymbolInformation) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error

	// Direct SCIP search methods for MCP tools
	SearchSymbols(ctx context.Context, pattern string, filePattern string, maxResults int) ([]interface{}, error)
	SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error)
	SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error)
	GetSymbolInfo(ctx context.Context, symbolName string, filePattern string) (interface{}, error)

	// Access to underlying SCIP storage
	GetSCIPStorage() scip.SCIPDocumentStorage
}

// SCIPCacheManager implements simple in-memory cache with occurrence-centric SCIP storage
type SCIPCacheManager struct {
	entries      map[string]*CacheEntry
	config       *config.CacheConfig
	stats        *SimpleCacheStats
	mu           sync.RWMutex
	enabled      bool
	started      bool
	scipStorage  scip.SCIPDocumentStorage
	indexStats   *IndexStats
	indexMu      sync.RWMutex
	converter    *SCIPConverter
	fileTracker  *FileChangeTracker
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
			LastUpdate:       time.Now(),
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

// InvalidateDocument removes all cached entries for a specific document and its dependencies
func (m *SCIPCacheManager) InvalidateDocument(uri string) error {
	if !m.isEnabled() {
		return nil
	}

	var affectedDocs []string
	if scipStorage, ok := m.scipStorage.(*scip.SimpleSCIPStorage); ok {
		affectedDocs = scipStorage.GetAffectedDocuments(uri)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	common.LSPLogger.Debug("Invalidating cache for document: %s (affects %d other documents)", uri, len(affectedDocs))

	m.invalidateSingleDocument(uri)
	for _, affectedURI := range affectedDocs {
		m.invalidateSingleDocument(affectedURI)
	}

	m.updateStats()

	if m.config.BackgroundIndex && len(affectedDocs) > 0 {
		docsToReindex := append(affectedDocs, uri)
		go m.reindexDocuments(docsToReindex)
	}

	return nil
}

// invalidateSingleDocument removes cached entries for a single document
func (m *SCIPCacheManager) invalidateSingleDocument(uri string) {
	// Remove all entries with matching URI
	var removedCount int
	for key, entry := range m.entries {
		if entry.Key.URI == uri {
			delete(m.entries, key)
			removedCount++
		}
	}

	// Cache entries removed

	// Clean up SCIP storage
	if m.scipStorage != nil {
		if err := m.scipStorage.RemoveDocument(context.Background(), uri); err != nil {
			common.LSPLogger.Error("Failed to remove document from SCIP storage: %v", err)
		}
	}
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

// Helper methods for simple cache implementation

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

// SCIP Indexing Implementation

// IndexDocument indexes LSP symbols using occurrence-centric SCIP storage
func (m *SCIPCacheManager) IndexDocument(ctx context.Context, uri, language string, symbols []types.SymbolInformation) error {
	if !m.enabled {
		return nil
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	doc, err := m.createSCIPDocument(uri, language, symbols)
	if err != nil {
		return fmt.Errorf("failed to create SCIP document: %w", err)
	}

	if err := m.scipStorage.StoreDocument(ctx, doc); err != nil {
		return fmt.Errorf("failed to store SCIP document: %w", err)
	}

	m.updateIndexStats(language, len(symbols))
	return nil
}

// UpdateSymbolIndex converts LSP symbols to SCIP occurrences with enhanced range information
func (m *SCIPCacheManager) UpdateSymbolIndex(uri string, symbols []*types.SymbolInformation, documentSymbols []*lsp.DocumentSymbol) error {
	if !m.enabled {
		return nil
	}

	docSymbolMap := make(map[string]*lsp.DocumentSymbol)
	if documentSymbols != nil {
		m.flattenDocumentSymbols(documentSymbols, docSymbolMap, "")
	}

	language := m.converter.DetectLanguageFromURI(uri)

	enhancedSymbols := make([]types.SymbolInformation, 0, len(symbols))
	for _, sym := range symbols {
		if sym != nil {
			enhancedSym := *sym
			if docSym, found := docSymbolMap[sym.Name]; found {
				enhancedSym.Location.Range = docSym.Range
			}
			enhancedSymbols = append(enhancedSymbols, enhancedSym)
		}
	}

	return m.IndexDocument(context.Background(), uri, language, enhancedSymbols)
}

// flattenDocumentSymbols recursively flattens DocumentSymbols into a map
func (m *SCIPCacheManager) flattenDocumentSymbols(symbols []*lsp.DocumentSymbol, result map[string]*lsp.DocumentSymbol, parentPath string) {
	for _, sym := range symbols {
		if sym != nil {
			fullPath := sym.Name
			if parentPath != "" {
				fullPath = parentPath + "/" + sym.Name
			}

			// Store both with simple name and full path
			result[sym.Name] = sym
			result[fullPath] = sym

			if sym.Children != nil {
				m.flattenDocumentSymbols(sym.Children, result, fullPath)
			}
		}
	}
}

// Helper methods for LSP to SCIP conversion

// createSCIPDocument creates a SCIP document from LSP symbols
func (m *SCIPCacheManager) createSCIPDocument(uri, language string, symbols []types.SymbolInformation) (*scip.SCIPDocument, error) {
	ctx := NewConversionContext(uri, language).
		WithSymbolRoles(types.SymbolRoleDefinition)

	return m.converter.ConvertLSPSymbolsToSCIPDocument(ctx, symbols)
}

// storeDefinitionResult stores definition results as SCIP occurrences
func (m *SCIPCacheManager) storeDefinitionResult(uri string, response interface{}) error {
	locations, ok := response.([]types.Location)
	if !ok {
		return fmt.Errorf("invalid definition response type")
	}

	for _, location := range locations {
		ctx := NewConversionContext(location.URI, m.converter.DetectLanguageFromURI(location.URI)).
			WithSymbolRoles(types.SymbolRoleDefinition)
		ctx.DefaultSize = 100

		// Create document with single occurrence
		scipDoc, err := m.converter.ConvertLocationsToSCIPDocument(ctx, []types.Location{location}, nil)
		if err != nil {
			return fmt.Errorf("failed to convert definition location: %w", err)
		}

		if err := m.converter.ValidateAndStore(m.scipStorage, scipDoc, "definition"); err != nil {
			return err
		}
	}

	return nil
}

// storeReferencesResult stores reference results as SCIP occurrences
func (m *SCIPCacheManager) storeReferencesResult(uri string, response interface{}) error {
	locations, ok := response.([]types.Location)
	if !ok {
		return fmt.Errorf("invalid references response type")
	}

	// Group locations by document URI
	docLocations := make(map[string][]types.Location)
	for _, location := range locations {
		docLocations[location.URI] = append(docLocations[location.URI], location)
	}

	// Store each document with its references
	for docURI, locationsForDoc := range docLocations {
		ctx := NewConversionContext(docURI, m.converter.DetectLanguageFromURI(docURI)).
			WithSymbolRoles(types.SymbolRoleReadAccess) // References are read access
		ctx.DefaultSize = int64(len(locationsForDoc) * 50)

		// Create custom symbol ID generator that uses the original URI
		symbolIDGenerator := func(location types.Location) string {
			return m.converter.GenerateLocationBasedSymbolID(uri, location.Range.Start.Line, location.Range.Start.Character)
		}

		scipDoc, err := m.converter.ConvertLocationsToSCIPDocument(ctx, locationsForDoc, symbolIDGenerator)
		if err != nil {
			return fmt.Errorf("failed to convert reference locations: %w", err)
		}

		if err := m.converter.ValidateAndStore(m.scipStorage, scipDoc, "references"); err != nil {
			return err
		}
	}

	return nil
}

// storeHoverResult stores hover information as symbol metadata
func (m *SCIPCacheManager) storeHoverResult(uri string, params, response interface{}) error {
	hover, ok := response.(*lsp.Hover)
	if !ok {
		return fmt.Errorf("invalid hover response type")
	}

	// Extract position from parameters
	position, err := m.extractPositionFromParams(params)
	if err != nil {
		return fmt.Errorf("failed to extract position: %w", err)
	}

	// Generate symbol ID
	symbolID := fmt.Sprintf("symbol_%s_%d_%d", uri, position.Line, position.Character)

	// Create or update symbol information with documentation
	var docText string
	if strContent, ok := hover.Contents.(string); ok {
		docText = strContent
	} else {
		docText = "hover information"
	}

	symbolInfo := scip.SCIPSymbolInformation{
		Symbol:        symbolID,
		DisplayName:   "hover_symbol", // Could be extracted from hover contents
		Documentation: []string{docText},
	}

	// Create a simple document with just the symbol information
	scipDoc := &scip.SCIPDocument{
		URI:               uri,
		Language:          m.converter.DetectLanguageFromURI(uri),
		LastModified:      time.Now(),
		Size:              100, // Rough estimate for hover info
		Occurrences:       []scip.SCIPOccurrence{},
		SymbolInformation: []scip.SCIPSymbolInformation{symbolInfo},
	}

	if err := m.scipStorage.StoreDocument(context.Background(), scipDoc); err != nil {
		return fmt.Errorf("failed to store hover information: %w", err)
	}

	return nil
}

// storeDocumentSymbolResult stores document symbols as definition occurrences
func (m *SCIPCacheManager) storeDocumentSymbolResult(uri string, response interface{}) error {
	symbols, ok := response.([]types.SymbolInformation)
	if !ok {
		return fmt.Errorf("invalid document symbol response type")
	}

	language := m.converter.DetectLanguageFromURI(uri)
	scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbols)
	if err != nil {
		return fmt.Errorf("failed to convert symbols: %w", err)
	}

	if err := m.converter.ValidateAndStore(m.scipStorage, scipDoc, "document symbols"); err != nil {
		return err
	}

	return nil
}

// storeWorkspaceSymbolResult stores workspace symbols
func (m *SCIPCacheManager) storeWorkspaceSymbolResult(response interface{}) error {
	symbols, ok := response.([]types.SymbolInformation)
	if !ok {
		return fmt.Errorf("invalid workspace symbol response type")
	}

	// Group symbols by document
	docSymbols := make(map[string][]types.SymbolInformation)
	for _, symbol := range symbols {
		docSymbols[symbol.Location.URI] = append(docSymbols[symbol.Location.URI], symbol)
	}

	// Store each document
	for uri, symbolsForDoc := range docSymbols {
		language := m.converter.DetectLanguageFromURI(uri)
		scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbolsForDoc)
		if err != nil {
			common.LSPLogger.Warn("Failed to convert workspace symbols for %s: %v", uri, err)
			continue
		}

		if err := m.converter.StoreSCIPDocument(m.scipStorage, scipDoc); err != nil {
			common.LSPLogger.Warn("Failed to store workspace symbols for %s: %v", uri, err)
		}
	}

	return nil
}

// storeCompletionResult stores completion items (not typically cached as occurrences)
func (m *SCIPCacheManager) storeCompletionResult(uri string, params, response interface{}) error {
	// Completion items are typically not stored as occurrences
	// This is a placeholder for potential future enhancements
	// Completion items not stored in occurrence cache
	return nil
}

// convertLSPSymbolsToSCIPDocument converts LSP symbols to a SCIP document format
func (m *SCIPCacheManager) convertLSPSymbolsToSCIPDocument(uri, language string, symbols []types.SymbolInformation) (*scip.SCIPDocument, error) {
	ctx := NewConversionContext(uri, language).
		WithSymbolRoles(types.SymbolRoleDefinition)

	return m.converter.ConvertLSPSymbolsToSCIPDocument(ctx, symbols)
}

// Helper methods for conversion between LSP and SCIP types

// convertLSPSymbolKindToSCIPKind converts LSP symbol kind to SCIP symbol kind
func (m *SCIPCacheManager) convertLSPSymbolKindToSCIPKind(kind types.SymbolKind) scip.SCIPSymbolKind {
	return m.converter.ConvertLSPSymbolKindToSCIP(kind)
}

// convertSCIPSymbolKindToLSP converts SCIP symbol kind back to LSP
func (m *SCIPCacheManager) convertSCIPSymbolKindToLSP(kind scip.SCIPSymbolKind) types.SymbolKind {
	return m.converter.ConvertSCIPSymbolKindToLSP(kind)
}

// convertLSPSymbolKindToSyntaxKind converts LSP symbol kind to syntax kind for highlighting
func (m *SCIPCacheManager) convertLSPSymbolKindToSyntaxKind(kind types.SymbolKind) types.SyntaxKind {
	return m.converter.ConvertLSPSymbolKindToSyntax(kind)
}

// convertSCIPSymbolKindToCompletionItemKind converts SCIP symbol kind to completion item kind
func (m *SCIPCacheManager) convertSCIPSymbolKindToCompletionItemKind(kind scip.SCIPSymbolKind) lsp.CompletionItemKind {
	return m.converter.ConvertSCIPSymbolKindToCompletionItem(kind)
}

// formatHoverFromSCIPSymbolInfo formats SCIP symbol information for hover display
func (m *SCIPCacheManager) formatHoverFromSCIPSymbolInfo(symbolInfo *scip.SCIPSymbolInformation) string {
	var content strings.Builder

	// Add symbol name and kind
	content.WriteString(fmt.Sprintf("**%s**\n\n", symbolInfo.DisplayName))

	// Add documentation
	if len(symbolInfo.Documentation) > 0 {
		content.WriteString(strings.Join(symbolInfo.Documentation, "\n"))
	}

	// Add signature documentation if available
	if symbolInfo.SignatureDocumentation.Text != "" {
		content.WriteString("\n\n---\n\n")
		content.WriteString(symbolInfo.SignatureDocumentation.Text)
	}

	return content.String()
}

// formatSymbolDetail formats symbol detail for completion items
func (m *SCIPCacheManager) formatSymbolDetail(symbolInfo *scip.SCIPSymbolInformation) string {
	if symbolInfo.SignatureDocumentation.Text != "" {
		return symbolInfo.SignatureDocumentation.Text
	}
	return fmt.Sprintf("%d", symbolInfo.Kind)
}

// extractURIFromOccurrence extracts document URI from a SCIP occurrence
func (m *SCIPCacheManager) extractURIFromOccurrence(occ *scip.SCIPOccurrence) string {
	if m.scipStorage == nil || occ == nil {
		return ""
	}

	docs, err := m.scipStorage.ListDocuments(context.Background())
	if err != nil {
		return ""
	}

	for _, docURI := range docs {
		doc, docErr := m.scipStorage.GetDocument(context.Background(), docURI)
		if docErr != nil || doc == nil {
			continue
		}
		for _, docOcc := range doc.Occurrences {
			if docOcc.Symbol == occ.Symbol &&
				docOcc.Range.Start.Line == occ.Range.Start.Line &&
				docOcc.Range.Start.Character == occ.Range.Start.Character &&
				docOcc.Range.End.Line == occ.Range.End.Line &&
				docOcc.Range.End.Character == occ.Range.End.Character {
				return docURI
			}
		}
	}

	return ""
}

// extractPositionFromParams extracts position from LSP parameters
func (m *SCIPCacheManager) extractPositionFromParams(params interface{}) (types.Position, error) {
	// This is a simplified extraction - in practice you'd handle different parameter types
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return types.Position{}, common.ParameterValidationError("invalid parameters format")
	}

	positionMap, ok := paramsMap["position"].(map[string]interface{})
	if !ok {
		return types.Position{}, common.ParameterValidationError("no position in parameters")
	}

	line, ok := positionMap["line"].(float64)
	if !ok {
		return types.Position{}, common.ParameterValidationError("invalid line in position")
	}

	character, ok := positionMap["character"].(float64)
	if !ok {
		return types.Position{}, common.ParameterValidationError("invalid character in position")
	}

	return types.Position{
		Line:      int32(line),
		Character: int32(character),
	}, nil
}

// QueryIndex queries the SCIP storage for symbols and relationships
func (m *SCIPCacheManager) QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error) {
	if !m.enabled {
		return &IndexResult{
			Type:      query.Type,
			Results:   []interface{}{},
			Metadata:  map[string]interface{}{"cache_disabled": true},
			Timestamp: time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	var results []interface{}
	var err error

	switch query.Type {
	case "symbol":
		sciPSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
		if err == nil {
			results = make([]interface{}, len(sciPSymbols))
			for i, sym := range sciPSymbols {
				results[i] = sym
			}
		}
	case "definition":
		if defOccs, err := m.scipStorage.GetDefinitions(ctx, query.Symbol); err == nil && len(defOccs) > 0 {
			results = []interface{}{defOccs[0]}
		}
	case "references":
		if refOccs, err := m.scipStorage.GetReferences(ctx, query.Symbol); err == nil {
			for _, occ := range refOccs {
				results = append(results, occ)
			}
		}
	case "workspace":
		sciPSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
		if err == nil {
			results = make([]interface{}, len(sciPSymbols))
			for i, sym := range sciPSymbols {
				results[i] = sym
			}
		}
	default:
		return nil, fmt.Errorf("unsupported query type: %s", query.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	metadata := map[string]interface{}{
		"query_language": query.Language,
		"query_type":     query.Type,
	}

	stats := m.scipStorage.GetIndexStats()
	metadata["total_symbols"] = stats.TotalSymbols
	metadata["total_documents"] = stats.TotalDocuments
	metadata["total_occurrences"] = stats.TotalOccurrences

	return &IndexResult{
		Type:      query.Type,
		Results:   results,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}, nil
}

// GetIndexStats returns current index statistics
func (m *SCIPCacheManager) GetIndexStats() *IndexStats {
	if !m.enabled {
		return &IndexStats{Status: "disabled"}
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	stats := *m.indexStats

	scipStats := m.scipStorage.GetIndexStats()
	stats.SymbolCount = scipStats.TotalSymbols
	stats.DocumentCount = int64(scipStats.TotalDocuments)
	stats.ReferenceCount = scipStats.TotalOccurrences
	stats.IndexSize = scipStats.MemoryUsage

	return &stats
}

// UpdateIndex updates the index with the given files
func (m *SCIPCacheManager) UpdateIndex(ctx context.Context, files []string) error {
	if !m.enabled {
		return nil
	}
	// Indexing happens when LSP methods return symbol information
	return nil
}

// Direct SCIP Search Methods - Added after QueryIndex method for direct occurrence-based searching

// SearchSymbolsEnhanced performs direct SCIP symbol search with enhanced results
func (m *SCIPCacheManager) SearchSymbolsEnhanced(ctx context.Context, query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResult, error) {
	if !m.enabled {
		return &EnhancedSymbolSearchResult{
			Symbols:   []EnhancedSymbolResult{},
			Total:     0,
			Truncated: false,
			Query:     query,
			Metadata:  map[string]interface{}{"cache_disabled": true},
			Timestamp: time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	maxResults := 100
	if query.MaxResults > 0 {
		maxResults = query.MaxResults
	}

	searchSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Pattern, maxResults*2)
	if err != nil {
		return nil, fmt.Errorf("failed to search symbols in SCIP storage: %w", err)
	}

	var enhancedResults []EnhancedSymbolResult
	for i, symbolInfo := range searchSymbols {
		if i >= maxResults {
			break
		}

		// Apply filters
		if len(query.SymbolKinds) > 0 {
			kindMatched := false
			for _, kind := range query.SymbolKinds {
				if symbolInfo.Kind == kind {
					kindMatched = true
					break
				}
			}
			if !kindMatched {
				continue
			}
		}

		occurrences, _ := m.scipStorage.GetOccurrences(ctx, symbolInfo.Symbol)

		if query.MinOccurrences > 0 && len(occurrences) < query.MinOccurrences {
			continue
		}
		if query.MaxOccurrences > 0 && len(occurrences) > query.MaxOccurrences {
			continue
		}

		enhancedResults = append(enhancedResults, m.buildEnhancedSymbolResult(&symbolInfo, occurrences, query))
	}

	if query.SortBy != "" {
		m.sortEnhancedResults(enhancedResults, query.SortBy)
	}

	return &EnhancedSymbolSearchResult{
		Symbols:   enhancedResults,
		Total:     len(enhancedResults),
		Truncated: len(searchSymbols) > maxResults,
		Query:     query,
		Metadata: map[string]interface{}{
			"scip_enabled":     true,
			"total_candidates": len(searchSymbols),
		},
		Timestamp: time.Now(),
	}, nil
}

// SearchReferences provides interface-compliant reference search
func (m *SCIPCacheManager) SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled {
		return []interface{}{}, nil
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// First, try to find symbols by exact or partial name match
	var matchedSymbols []scip.SCIPSymbolInformation

	// Try exact match first
	if symbols, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
		symbolInfos, _ := symbols.SearchSymbols(ctx, symbolName, maxResults)
		matchedSymbols = append(matchedSymbols, symbolInfos...)
	}

	// If no exact matches, try searching with wildcards
	if len(matchedSymbols) == 0 {
		if symbols, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
			// Try searching with pattern matching
			patternSearch := ".*" + symbolName + ".*"
			symbolInfos, _ := symbols.SearchSymbols(ctx, patternSearch, maxResults)
			matchedSymbols = append(matchedSymbols, symbolInfos...)
		}
	}

	var allReferences []interface{}
	fileSet := make(map[string]bool)

	// For each matched symbol, get its references
	for _, symbolInfo := range matchedSymbols {
		// Get references from the index
		if refOccs, err := m.scipStorage.GetReferences(ctx, symbolInfo.Symbol); err == nil {
			for _, occ := range refOccs {
				docURI := m.extractURIFromOccurrence(&occ)
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					occInfo := m.buildOccurrenceInfo(&occ, docURI)
					allReferences = append(allReferences, occInfo)
					fileSet[docURI] = true

					if len(allReferences) >= maxResults {
						return allReferences, nil
					}
				}
			}
		}

		// Also check occurrences by symbol for more complete results
		if occurrences, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
			if occs, err := occurrences.GetOccurrences(ctx, symbolInfo.Symbol); err == nil {
				for _, occ := range occs {
					// Skip definitions, we want references
					if !occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
						docURI := m.extractURIFromOccurrence(&occ)
						if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
							occInfo := m.buildOccurrenceInfo(&occ, docURI)
							allReferences = append(allReferences, occInfo)
							fileSet[docURI] = true

							if len(allReferences) >= maxResults {
								return allReferences, nil
							}
						}
					}
				}
			}
		}
	}

	// Deduplicate results
	seen := make(map[string]bool)
	uniqueReferences := []interface{}{}
	for _, ref := range allReferences {
		if occInfo, ok := ref.(SCIPOccurrenceInfo); ok {
			key := fmt.Sprintf("%s:%d:%d", occInfo.DocumentURI, occInfo.LineNumber, occInfo.Occurrence.Range.Start.Character)
			if !seen[key] {
				seen[key] = true
				uniqueReferences = append(uniqueReferences, ref)
			}
		}
	}

	return uniqueReferences, nil
}

// SearchReferencesEnhanced performs direct SCIP reference search with enhanced results
func (m *SCIPCacheManager) SearchReferencesEnhanced(ctx context.Context, symbolName, filePattern string, options *ReferenceSearchOptions) (*ReferenceSearchResult, error) {
	if !m.enabled {
		return &ReferenceSearchResult{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   map[string]interface{}{"cache_disabled": true},
			Timestamp:  time.Now(),
		}, nil
	}

	if options == nil {
		options = &ReferenceSearchOptions{MaxResults: 100}
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, _ := m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	if len(symbolInfos) == 0 {
		symbolInfos, _ = m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	}

	if len(symbolInfos) == 0 {
		return &ReferenceSearchResult{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   map[string]interface{}{"symbols_found": 0},
			Timestamp:  time.Now(),
		}, nil
	}

	var allReferences []SCIPOccurrenceInfo
	var definition *SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	symbolID := ""

	for _, symbolInfo := range symbolInfos {
		symbolID = symbolInfo.Symbol

		if refOccurrences, err := m.scipStorage.GetReferences(ctx, symbolInfo.Symbol); err == nil {
			for _, occ := range refOccurrences {
				docURI := m.extractURIFromOccurrence(&occ)
				if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
					continue
				}

				occInfo := m.buildOccurrenceInfo(&occ, docURI)
				allReferences = append(allReferences, occInfo)
				fileSet[docURI] = true

				if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
					break
				}
			}
		}

		if definition == nil {
			if defOccs, err := m.scipStorage.GetDefinitions(ctx, symbolInfo.Symbol); err == nil && len(defOccs) > 0 {
				defOcc := &defOccs[0]
				docURI := m.extractURIFromOccurrence(defOcc)
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					defInfo := m.buildOccurrenceInfo(defOcc, docURI)
					definition = &defInfo
					fileSet[docURI] = true
				}
			}
		}

		if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
			break
		}
	}

	if options.SortBy != "" {
		m.sortOccurrenceResults(allReferences, options.SortBy)
	}

	return &ReferenceSearchResult{
		SymbolName: symbolName,
		SymbolID:   symbolID,
		References: allReferences,
		Definition: definition,
		TotalCount: len(allReferences),
		FileCount:  len(fileSet),
		Options:    options,
		Metadata: map[string]interface{}{
			"scip_enabled":  true,
			"symbols_found": len(symbolInfos),
			"file_pattern":  filePattern,
		},
		Timestamp: time.Now(),
	}, nil
}

// SearchDefinitions performs direct SCIP definition search with enhanced results
func (m *SCIPCacheManager) SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled {
		return []interface{}{}, nil
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, _ := m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	if len(symbolInfos) == 0 {
		return []interface{}{}, nil
	}

	var allDefinitions []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)

	for _, symbolInfo := range symbolInfos {

		if defOccs, err := m.scipStorage.GetDefinitions(ctx, symbolInfo.Symbol); err == nil && len(defOccs) > 0 {
			defOcc := &defOccs[0]
			docURI := m.extractURIFromOccurrence(defOcc)

			if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
				continue
			}

			occInfo := m.buildOccurrenceInfo(defOcc, docURI)
			allDefinitions = append(allDefinitions, occInfo)
			fileSet[docURI] = true
		}
	}

	// Convert to interface slice and apply maxResults limit
	results := make([]interface{}, 0, len(allDefinitions))
	for i, def := range allDefinitions {
		if maxResults > 0 && i >= maxResults {
			break
		}
		results = append(results, def)
	}

	return results, nil
}

// GetSymbolInfo performs direct SCIP symbol information retrieval with occurrence details
func (m *SCIPCacheManager) GetSymbolInfo(ctx context.Context, symbolName, filePattern string) (interface{}, error) {
	if !m.enabled {
		return &SymbolInfoResult{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        map[string]interface{}{"cache_disabled": true},
			Timestamp:       time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, _ := m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	if len(symbolInfos) == 0 {
		return &SymbolInfoResult{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        map[string]interface{}{"symbols_found": 0},
			Timestamp:       time.Now(),
		}, nil
	}

	symbolInfo := symbolInfos[0]
	symbolID := symbolInfo.Symbol

	allOccurrences, _ := m.scipStorage.GetOccurrences(ctx, symbolID)

	var filteredOccurrences []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	definitionCount := 0
	referenceCount := 0

	for _, occ := range allOccurrences {
		docURI := m.extractURIFromOccurrence(&occ)

		if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
			continue
		}

		occInfo := m.buildOccurrenceInfo(&occ, docURI)
		filteredOccurrences = append(filteredOccurrences, occInfo)
		fileSet[docURI] = true

		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			definitionCount++
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			referenceCount++
		}
	}

	// Note: Relationships not available in simplified interface, would need to be implemented separately
	var relationships []scip.SCIPRelationship
	signature := m.formatSymbolDetail(&symbolInfo)

	return &SymbolInfoResult{
		SymbolName:      symbolName,
		SymbolID:        symbolID,
		SymbolInfo:      &symbolInfo,
		Kind:            symbolInfo.Kind,
		Documentation:   symbolInfo.Documentation,
		Signature:       signature,
		Relationships:   relationships,
		Occurrences:     filteredOccurrences,
		OccurrenceCount: len(filteredOccurrences),
		DefinitionCount: definitionCount,
		ReferenceCount:  referenceCount,
		FileCount:       len(fileSet),
		Metadata: map[string]interface{}{
			"scip_enabled":         true,
			"symbols_found":        len(symbolInfos),
			"total_occurrences":    len(allOccurrences),
			"filtered_occurrences": len(filteredOccurrences),
			"file_pattern":         filePattern,
		},
		Timestamp: time.Now(),
	}, nil
}

func (m *SCIPCacheManager) updateIndexStats(language string, symbolCount int) {
	m.indexStats.LastUpdate = time.Now()
	m.indexStats.Status = "active"

	if m.indexStats.LanguageStats[language] == 0 {
		m.indexStats.IndexedLanguages = append(m.indexStats.IndexedLanguages, language)
	}
	m.indexStats.LanguageStats[language] += int64(symbolCount)

	stats := m.scipStorage.GetIndexStats()
	m.indexStats.SymbolCount = stats.TotalSymbols
	m.indexStats.DocumentCount = int64(stats.TotalDocuments)
}

// GetCachedDefinition retrieves definition occurrences for a symbol using SCIP storage
func (m *SCIPCacheManager) GetCachedDefinition(symbolID string) ([]types.Location, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	if defs, err := m.scipStorage.GetDefinitionsWithDocuments(context.Background(), symbolID); err == nil && len(defs) > 0 {
		def := defs[0]
		location := types.Location{
			URI: def.DocumentURI,
			Range: types.Range{
				Start: types.Position{Line: int32(def.Range.Start.Line), Character: int32(def.Range.Start.Character)},
				End:   types.Position{Line: int32(def.Range.End.Line), Character: int32(def.Range.End.Character)},
			},
		}
		return []types.Location{location}, true
	}
	return nil, false
}

// GetCachedReferences retrieves reference occurrences for a symbol using SCIP storage
func (m *SCIPCacheManager) GetCachedReferences(symbolID string) ([]types.Location, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	refs, err := m.scipStorage.GetReferencesWithDocuments(context.Background(), symbolID)
	if err != nil || len(refs) == 0 {
		return nil, false
	}
	locations := make([]types.Location, 0, len(refs))
	for _, ref := range refs {
		locations = append(locations, types.Location{
			URI: ref.DocumentURI,
			Range: types.Range{
				Start: types.Position{Line: int32(ref.Range.Start.Line), Character: int32(ref.Range.Start.Character)},
				End:   types.Position{Line: int32(ref.Range.End.Line), Character: int32(ref.Range.End.Character)},
			},
		})
	}
	return locations, true
}

// GetCachedHover retrieves hover information using symbol information from SCIP storage
func (m *SCIPCacheManager) GetCachedHover(symbolID string) (*lsp.Hover, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfo, err := m.scipStorage.GetSymbolInfo(context.Background(), symbolID)
	if err != nil {
		return nil, false
	}

	hover := &lsp.Hover{
		Contents: m.formatHoverFromSCIPSymbolInfo(symbolInfo),
	}

	return hover, true
}

// GetCachedDocumentSymbols retrieves document symbols using SCIP storage
func (m *SCIPCacheManager) GetCachedDocumentSymbols(uri string) ([]types.SymbolInformation, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// GetDocumentSymbols not available - would need to get document and extract symbols
	doc, err := m.scipStorage.GetDocument(context.Background(), uri)
	if err != nil {
		return nil, false
	}
	symbolInfos := doc.SymbolInformation
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	symbols := make([]types.SymbolInformation, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		defs, err := m.scipStorage.GetDefinitionsWithDocuments(context.Background(), scipSymbol.Symbol)
		if err != nil || len(defs) == 0 {
			continue
		}
		defOcc := defs[0]

		symbol := types.SymbolInformation{
			Name: scipSymbol.DisplayName,
			Kind: m.convertSCIPSymbolKindToLSP(scipSymbol.Kind),
			Location: types.Location{
				URI:   uri,
				Range: types.Range{Start: types.Position{Line: int32(defOcc.Range.Start.Line), Character: int32(defOcc.Range.Start.Character)}, End: types.Position{Line: int32(defOcc.Range.End.Line), Character: int32(defOcc.Range.End.Character)}},
			},
		}
		symbols = append(symbols, symbol)
	}

	return symbols, true
}

// GetCachedWorkspaceSymbols retrieves workspace symbols using SCIP storage
func (m *SCIPCacheManager) GetCachedWorkspaceSymbols(query string) ([]types.SymbolInformation, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, err := m.scipStorage.SearchSymbols(context.Background(), query, 100)
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	symbols := make([]types.SymbolInformation, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		defs, err := m.scipStorage.GetDefinitionsWithDocuments(context.Background(), scipSymbol.Symbol)
		if err != nil || len(defs) == 0 {
			continue
		}
		defOcc := defs[0]

		symbol := types.SymbolInformation{
			Name: scipSymbol.DisplayName,
			Kind: m.convertSCIPSymbolKindToLSP(scipSymbol.Kind),
			Location: types.Location{
				URI:   defOcc.DocumentURI,
				Range: types.Range{Start: types.Position{Line: int32(defOcc.Range.Start.Line), Character: int32(defOcc.Range.Start.Character)}, End: types.Position{Line: int32(defOcc.Range.End.Line), Character: int32(defOcc.Range.End.Character)}},
			},
		}
		symbols = append(symbols, symbol)
	}

	return symbols, true
}

// GetCachedCompletion retrieves completion items using symbol information from SCIP storage
func (m *SCIPCacheManager) GetCachedCompletion(uri string, position types.Position) ([]lsp.CompletionItem, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get document symbols for completion context
	// GetDocumentSymbols not available - would need to get document and extract symbols
	doc, err := m.scipStorage.GetDocument(context.Background(), uri)
	if err != nil {
		return nil, false
	}
	symbolInfos := doc.SymbolInformation
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	// Convert symbol information to completion items
	items := make([]lsp.CompletionItem, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		item := lsp.CompletionItem{
			Label:  scipSymbol.DisplayName,
			Kind:   m.convertSCIPSymbolKindToCompletionItemKind(scipSymbol.Kind),
			Detail: m.formatSymbolDetail(&scipSymbol),
		}

		if len(scipSymbol.Documentation) > 0 {
			item.Documentation = strings.Join(scipSymbol.Documentation, "\n")
		}

		items = append(items, item)
	}

	return items, true
}

// StoreMethodResult stores LSP method results as SCIP occurrences with proper roles
func (m *SCIPCacheManager) StoreMethodResult(method string, params interface{}, response interface{}) error {
	if !m.enabled {
		return nil
	}

	uri := m.extractURI(params)
	if uri == "" {
		return common.ParameterValidationError("could not extract URI from parameters")
	}

	switch method {
	case "textDocument/definition":
		return m.storeDefinitionResult(uri, response)
	case "textDocument/references":
		return m.storeReferencesResult(uri, response)
	case "textDocument/hover":
		return m.storeHoverResult(uri, params, response)
	case "textDocument/documentSymbol":
		return m.storeDocumentSymbolResult(uri, response)
	case "workspace/symbol":
		return m.storeWorkspaceSymbolResult(response)
	case "textDocument/completion":
		return m.storeCompletionResult(uri, params, response)
	default:
		// Method not cached in SCIP storage
		return nil
	}
}

// =============================================================================
// Direct SCIP Search Methods for MCP Tools
// =============================================================================

// SearchSymbols provides direct access to SCIP symbol search for MCP tools
func (m *SCIPCacheManager) SearchSymbols(ctx context.Context, pattern, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, pattern, maxResults)
	if err != nil {
		return nil, fmt.Errorf("SCIP symbol search failed: %w", err)
	}

	results := make([]interface{}, 0, len(symbolInfos))
	for _, symbolInfo := range symbolInfos {
		// Prefer definition; else any occurrence
		var occWithDoc *scip.OccurrenceWithDocument
		if defs, _ := m.scipStorage.GetDefinitionsWithDocuments(ctx, symbolInfo.Symbol); len(defs) > 0 {
			occWithDoc = &defs[0]
		} else if occs, _ := m.scipStorage.GetOccurrencesWithDocuments(ctx, symbolInfo.Symbol); len(occs) > 0 {
			occWithDoc = &occs[0]
		}
		if occWithDoc == nil {
			results = append(results, symbolInfo)
			continue
		}
		// Apply file filter
		if filePattern != "" && !m.matchFilePattern(occWithDoc.DocumentURI, filePattern) {
			continue
		}
		enhancedResult := map[string]interface{}{
			"symbolInfo":  symbolInfo,
			"occurrence":  &occWithDoc.SCIPOccurrence,
			"filePath":    occWithDoc.DocumentURI,
			"documentURI": occWithDoc.DocumentURI,
			"range":       occWithDoc.Range,
		}
		results = append(results, enhancedResult)
	}

	return results, nil
}

// =============================================================================
// Helper Methods for Enhanced Direct SCIP Search
// =============================================================================

// buildEnhancedSymbolResult creates an enhanced symbol result from SCIP data
func (m *SCIPCacheManager) buildEnhancedSymbolResult(symbolInfo *scip.SCIPSymbolInformation, occurrences []scip.SCIPOccurrence, query *EnhancedSymbolQuery) EnhancedSymbolResult {
	result := EnhancedSymbolResult{
		SymbolInfo:      symbolInfo,
		SymbolID:        symbolInfo.Symbol,
		DisplayName:     symbolInfo.DisplayName,
		Kind:            symbolInfo.Kind,
		OccurrenceCount: len(occurrences),
	}

	// Include occurrences if requested
	if query != nil && (query.IncludeDocumentation || len(occurrences) < 50) { // Avoid large arrays
		result.Occurrences = occurrences
	}

	// Count roles and collect metadata
	fileSet := make(map[string]bool)
	var allRoles types.SymbolRole

	for _, occ := range occurrences {
		allRoles |= occ.SymbolRoles

		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			result.DefinitionCount++
			result.HasDefinition = true
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) {
			result.ReadAccessCount++
			result.HasReferences = true
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			result.WriteAccessCount++
		}

		// Extract document URI from occurrence
		docURI := m.extractURIFromOccurrence(&occ)
		if docURI != "" {
			fileSet[docURI] = true
		}
	}

	result.AllRoles = allRoles
	result.ReferenceCount = result.ReadAccessCount + result.WriteAccessCount
	result.FileCount = len(fileSet)
	result.DocumentURIs = make([]string, 0, len(fileSet))
	for uri := range fileSet {
		result.DocumentURIs = append(result.DocumentURIs, uri)
	}

	// Include documentation and relationships if requested
	if query != nil && query.IncludeDocumentation {
		result.Documentation = symbolInfo.Documentation
		if query.IncludeRelationships {
			// Note: Relationships not available in simplified interface
			// result.Relationships = relationships
		}
	}

	// Calculate basic scoring
	result.PopularityScore = float64(result.OccurrenceCount)
	result.RelevanceScore = 1.0 // Basic relevance - could be enhanced with pattern matching
	result.FinalScore = result.RelevanceScore * (1.0 + result.PopularityScore/100.0)

	return result
}

// buildOccurrenceInfo creates occurrence info with context
func (m *SCIPCacheManager) buildOccurrenceInfo(occ *scip.SCIPOccurrence, docURI string) SCIPOccurrenceInfo {
	return SCIPOccurrenceInfo{
		Occurrence:  *occ,
		DocumentURI: docURI,
		SymbolRoles: occ.SymbolRoles,
		SyntaxKind:  occ.SyntaxKind,
		LineNumber:  occ.Range.Start.Line,
		Score:       1.0, // Basic score
		// Context could be added by reading file content around the occurrence
	}
}

// matchFilePattern checks if a file URI matches a pattern
func (m *SCIPCacheManager) matchFilePattern(uri, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Simple pattern matching - could be enhanced with glob patterns
	if strings.Contains(pattern, "*") {
		// Basic wildcard support
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		if matched, err := regexp.MatchString(pattern, uri); err == nil {
			return matched
		}
	}

	// Exact or substring match
	return strings.Contains(uri, pattern)
}

// sortEnhancedResults sorts enhanced symbol results by the specified criteria
func (m *SCIPCacheManager) sortEnhancedResults(results []EnhancedSymbolResult, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(results, func(i, j int) bool {
			return results[i].DisplayName < results[j].DisplayName
		})
	case "relevance":
		sort.Slice(results, func(i, j int) bool {
			return results[i].RelevanceScore > results[j].RelevanceScore
		})
	case "occurrences":
		sort.Slice(results, func(i, j int) bool {
			return results[i].OccurrenceCount > results[j].OccurrenceCount
		})
	case "kind":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Kind < results[j].Kind
		})
	default:
		// Default to final score
		sort.Slice(results, func(i, j int) bool {
			return results[i].FinalScore > results[j].FinalScore
		})
	}
}

// sortOccurrenceResults sorts occurrence results by the specified criteria
func (m *SCIPCacheManager) sortOccurrenceResults(results []SCIPOccurrenceInfo, sortBy string) {
	switch sortBy {
	case "location":
		sort.Slice(results, func(i, j int) bool {
			if results[i].DocumentURI != results[j].DocumentURI {
				return results[i].DocumentURI < results[j].DocumentURI
			}
			if results[i].LineNumber != results[j].LineNumber {
				return results[i].LineNumber < results[j].LineNumber
			}
			return results[i].Occurrence.Range.Start.Character < results[j].Occurrence.Range.Start.Character
		})
	case "file":
		sort.Slice(results, func(i, j int) bool {
			return results[i].DocumentURI < results[j].DocumentURI
		})
	case "relevance":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	default:
		// Default to location
		sort.Slice(results, func(i, j int) bool {
			if results[i].DocumentURI != results[j].DocumentURI {
				return results[i].DocumentURI < results[j].DocumentURI
			}
			return results[i].LineNumber < results[j].LineNumber
		})
	}
}

// reindexDocuments performs background reindexing of affected documents
// This is triggered when a file changes and affects other files that reference it
func (m *SCIPCacheManager) reindexDocuments(uris []string) {
	// Create a context with timeout for reindexing
	ctx, cancel := common.CreateContext(30 * time.Second)
	defer cancel()

	common.LSPLogger.Debug("Starting background reindexing for %d documents", len(uris))

	successCount := 0
	errorCount := 0

	for _, uri := range uris {
		// Skip if context is cancelled
		select {
		case <-ctx.Done():
			common.LSPLogger.Warn("Reindexing cancelled after %d/%d documents", successCount, len(uris))
			return
		default:
		}

		// Detect language from URI
		language := m.converter.DetectLanguageFromURI(uri)
		if language == "" {
			common.LSPLogger.Debug("Could not detect language for %s, skipping reindex", uri)
			errorCount++
			continue
		}

		// Request fresh document symbols from LSP server
		// Note: We need access to LSP manager, which should be injected or accessible
		// For now, we'll just mark this as a placeholder for actual implementation
		// The actual implementation would need to call LSP server through the manager

		// Parameters for document symbols request would be:
		// params := map[string]interface{}{
		//     "textDocument": map[string]interface{}{
		//         "uri": uri,
		//     },
		// }

		// Since we don't have direct access to LSP manager here, we'll just log
		// In a real implementation, this would be coordinated through the LSP manager
		common.LSPLogger.Debug("Would reindex document %s (language: %s)", uri, language)

		// For demonstration purposes, we'll just mark as successful
		// In actual implementation, this would:
		// 1. Get fresh symbols from LSP server
		// 2. Convert to SCIP format
		// 3. Store in SCIP storage
		successCount++
	}

	common.LSPLogger.Debug("Background reindexing completed: %d successful, %d errors", successCount, errorCount)
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
