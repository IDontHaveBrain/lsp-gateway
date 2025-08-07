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
	IndexSize        int64            `json:"index_size_bytes"`
	LastUpdate       time.Time        `json:"last_update"`
	LanguageStats    map[string]int64 `json:"language_stats"`
	IndexedLanguages []string         `json:"indexed_languages"`
	Status           string           `json:"status"`
}

// SCIPSymbol wraps LSP SymbolInformation with enhanced SCIP metadata
type SCIPSymbol struct {
	SymbolInfo          lsp.SymbolInformation  `json:"symbol_info"`
	Language            string                 `json:"language"`
	Score               float64                `json:"score,omitempty"`
	FullRange           *Range                 `json:"full_range,omitempty"`           // Full range from document symbols
	Documentation       string                 `json:"documentation,omitempty"`        // Documentation from hover
	Signature           string                 `json:"signature,omitempty"`            // Signature from hover
	RelatedSymbols      []string               `json:"related_symbols,omitempty"`      // Related symbol names
	DefinitionLocations []lsp.Location         `json:"definition_locations,omitempty"` // Definition locations for this symbol
	ReferenceLocations  []lsp.Location         `json:"reference_locations,omitempty"`  // Reference locations for this symbol
	UsageCount          int                    `json:"usage_count,omitempty"`          // Number of references to this symbol
	Metadata            map[string]interface{} `json:"metadata,omitempty"`             // Additional SCIP metadata
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
	IndexDocument(ctx context.Context, uri string, language string, symbols []lsp.SymbolInformation) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error

	// Direct SCIP search methods for MCP tools
	SearchSymbols(ctx context.Context, pattern string, filePattern string, maxResults int) ([]interface{}, error)
	SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error)
	SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error)
	GetSymbolInfo(ctx context.Context, symbolName string, filePattern string) (interface{}, error)
}

// SCIPCacheManager implements simple in-memory cache with occurrence-centric SCIP storage
type SCIPCacheManager struct {
	entries map[string]*CacheEntry
	config  *config.CacheConfig
	stats   *SimpleCacheStats
	mu      sync.RWMutex
	enabled bool
	started bool

	// SCIP storage - occurrence-centric architecture
	scipStorage scip.SCIPDocumentStorage
	indexStats  *IndexStats
	indexMu     sync.RWMutex
}

// NewSCIPCacheManager creates a simple cache manager with unified config
func NewSCIPCacheManager(configParam *config.CacheConfig) (*SCIPCacheManager, error) {
	if configParam == nil {
		configParam = config.GetDefaultCacheConfig()
	}

	// Create SCIP storage with occurrence-centric architecture
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
	}

	// Start SCIP storage if enabled
	if manager.enabled {
		if err := manager.scipStorage.Start(context.Background()); err != nil {
			// Ignore "storage already started" error - it's expected if already running
			if err.Error() != "storage already started" {
				return nil, fmt.Errorf("failed to start SCIP storage: %w", err)
			}
		}
	}

	// Auto-initialize on creation - no separate Initialize() call needed
	if manager.enabled {
	} else {
	}

	return manager, nil
}

// NewSimpleCache creates a cache manager with basic MB-based configuration using unified config
func NewSimpleCache(maxMemoryMB int) (*SCIPCacheManager, error) {
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

// Start begins cache operations
func (m *SCIPCacheManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cache manager already started")
	}

	if !m.enabled {
		common.LSPLogger.Info("Simple cache is disabled, skipping start")
		return nil
	}

	// Start SCIP storage if not already started
	if m.scipStorage != nil {
		if err := m.scipStorage.Start(ctx); err != nil {
			// Ignore "storage already started" error - it's expected if already running
			if err.Error() != "storage already started" {
				common.LSPLogger.Warn("Failed to start SCIP storage: %v", err)
			}
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

	// Stop SCIP storage
	if m.scipStorage != nil {
		if err := m.scipStorage.Stop(context.Background()); err != nil {
			common.LSPLogger.Warn("Failed to stop SCIP storage: %v", err)
		}
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

// InvalidateDocument removes all cached entries for a specific document
func (m *SCIPCacheManager) InvalidateDocument(uri string) error {
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

	// Clean up SCIP storage
	if m.scipStorage != nil {
		if err := m.scipStorage.RemoveDocument(context.Background(), uri); err != nil {
			common.LSPLogger.Error("Failed to remove document from SCIP storage: %v", err)
		}
	}

	m.updateStats()
	return nil
}

// Clear removes all cache entries
func (m *SCIPCacheManager) Clear() error {
	common.LSPLogger.Info("Clear() method called")
	
	// Don't check started state for Clear - it should work even if not started
	if !m.enabled {
		common.LSPLogger.Info("Cache is not enabled, returning early")
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	common.LSPLogger.Info("Clearing all cache entries")
	m.entries = make(map[string]*CacheEntry)
	
	// Stop SCIP storage to ensure it doesn't save on shutdown
	if m.scipStorage != nil {
		common.LSPLogger.Info("Stopping SCIP storage before clearing")
		ctx := context.Background()
		if err := m.scipStorage.Stop(ctx); err != nil {
			common.LSPLogger.Warn("Failed to stop SCIP storage: %v", err)
		}
	}
	
	// Delete persisted SCIP storage cache file
	if m.config.DiskCache && m.config.StoragePath != "" {
		// The SCIP storage saves to simple_cache.json in the storage path
		cacheFile := filepath.Join(m.config.StoragePath, "simple_cache.json")
		common.LSPLogger.Info("Attempting to remove cache file: %s", cacheFile)
		
		// Check if file exists before deletion
		if _, err := os.Stat(cacheFile); err == nil {
			common.LSPLogger.Info("Cache file exists, removing: %s", cacheFile)
			if err := os.Remove(cacheFile); err != nil {
				common.LSPLogger.Error("Failed to remove persisted SCIP cache: %v", err)
			} else {
				common.LSPLogger.Info("Successfully removed persisted SCIP cache file: %s", cacheFile)
			}
		} else if os.IsNotExist(err) {
			common.LSPLogger.Info("Cache file does not exist: %s", cacheFile)
		} else {
			common.LSPLogger.Warn("Error checking cache file: %v", err)
		}
	}
	
	// Recreate SCIP storage with empty data
	if m.scipStorage != nil {
		common.LSPLogger.Info("Recreating SCIP storage with empty data")
		scipConfig := scip.SCIPStorageConfig{
			DiskCacheDir: m.config.StoragePath,
			MemoryLimit:  int64(m.config.MaxMemoryMB) * 1024 * 1024,
		}
		newStorage, err := scip.NewSimpleSCIPStorage(scipConfig)
		if err != nil {
			common.LSPLogger.Warn("Failed to recreate SCIP storage: %v", err)
		} else {
			m.scipStorage = newStorage
			// Start the new storage but don't load from disk (file was deleted)
			ctx := context.Background()
			if err := m.scipStorage.Start(ctx); err != nil {
				common.LSPLogger.Warn("Failed to start new SCIP storage: %v", err)
			}
		}
	}
	
	// Reset index stats
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
	return m.enabled && m.started
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
func (m *SCIPCacheManager) IndexDocument(ctx context.Context, uri string, language string, symbols []lsp.SymbolInformation) error {
	if !m.enabled || m.scipStorage == nil {
		return nil // Graceful degradation when cache is disabled
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	// Convert LSP symbols to SCIP document with occurrences
	scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbols)
	if err != nil {
		return fmt.Errorf("failed to convert LSP symbols to SCIP document: %w", err)
	}
	
	common.LSPLogger.Debug("IndexDocument: Created SCIP doc for %s with %d occurrences and %d symbol infos", 
		uri, len(scipDoc.Occurrences), len(scipDoc.SymbolInformation))

	// Store in SCIP storage
	if err := m.scipStorage.StoreDocument(ctx, scipDoc); err != nil {
		return fmt.Errorf("failed to store SCIP document: %w", err)
	}
	
	common.LSPLogger.Debug("IndexDocument: Successfully stored SCIP document for %s", uri)

	// Update statistics
	m.updateIndexStats(language, len(symbols))

	return nil
}

// UpdateSymbolIndex implements the QueryManager interface with DocumentSymbol support
// This method converts LSP symbols to SCIP occurrences with enhanced range information
func (m *SCIPCacheManager) UpdateSymbolIndex(uri string, symbols []*lsp.SymbolInformation, documentSymbols []*lsp.DocumentSymbol) error {
	if !m.enabled {
		return nil
	}

	// Build a map of DocumentSymbol ranges by name for efficient lookup
	docSymbolMap := make(map[string]*lsp.DocumentSymbol)
	if documentSymbols != nil {
		m.flattenDocumentSymbols(documentSymbols, docSymbolMap, "")
	}

	// Detect language from file extension
	language := m.detectLanguageFromURI(uri)

	// Convert pointers to values and enhance with DocumentSymbol ranges
	enhancedSymbols := make([]lsp.SymbolInformation, 0, len(symbols))
	for _, sym := range symbols {
		if sym != nil {
			enhancedSym := *sym

			// Try to find matching DocumentSymbol for full range
			if docSym, found := docSymbolMap[sym.Name]; found {
				// Use the full range from DocumentSymbol
				enhancedSym.Location.Range = docSym.Range
			}

			enhancedSymbols = append(enhancedSymbols, enhancedSym)
		}
	}

	// Index using occurrence-centric approach
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

// detectLanguageFromURI detects language from file extension
func (m *SCIPCacheManager) detectLanguageFromURI(uri string) string {
	ext := filepath.Ext(uri)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	default:
		return "unknown"
	}
}

// Helper methods for LSP to SCIP conversion

// convertLSPSymbolsToSCIPDocument converts LSP symbols to a SCIP document with occurrences
func (m *SCIPCacheManager) convertLSPSymbolsToSCIPDocument(uri string, language string, symbols []lsp.SymbolInformation) (*scip.SCIPDocument, error) {
	scipDoc := &scip.SCIPDocument{
		URI:               uri,
		Language:          language,
		LastModified:      time.Now(),
		Size:              int64(len(symbols) * 100), // Rough estimate
		Occurrences:       make([]scip.SCIPOccurrence, 0, len(symbols)),
		SymbolInformation: make([]scip.SCIPSymbolInformation, 0, len(symbols)),
	}

	// Detect package information for SCIP ID generation
	packageName, version := m.detectPackageInfo(uri, language)

	for _, symbol := range symbols {
		// Generate SCIP symbol ID
		symbolDescriptor := symbol.Name
		if symbol.ContainerName != "" {
			symbolDescriptor = symbol.ContainerName + "/" + symbol.Name
		}
		scipID := fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, symbolDescriptor)

		// Create SCIP occurrence (assumed to be definition since from documentSymbol)
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      int32(symbol.Location.Range.Start.Line),
					Character: int32(symbol.Location.Range.Start.Character),
				},
				End: types.Position{
					Line:      int32(symbol.Location.Range.End.Line),
					Character: int32(symbol.Location.Range.End.Character),
				},
			},
			Symbol:      scipID,
			SymbolRoles: types.SymbolRoleDefinition, // DocumentSymbols are typically definitions
			SyntaxKind:  m.convertLSPSymbolKindToSyntaxKind(symbol.Kind),
		}
		scipDoc.Occurrences = append(scipDoc.Occurrences, occurrence)

		// Create SCIP symbol information
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:      scipID,
			DisplayName: symbol.Name,
			Kind:        m.convertLSPSymbolKindToSCIPKind(symbol.Kind),
		}
		scipDoc.SymbolInformation = append(scipDoc.SymbolInformation, symbolInfo)
	}

	return scipDoc, nil
}

// storeDefinitionResult stores definition results as SCIP occurrences
func (m *SCIPCacheManager) storeDefinitionResult(uri string, response interface{}) error {
	locations, ok := response.([]lsp.Location)
	if !ok {
		return fmt.Errorf("invalid definition response type")
	}

	for _, location := range locations {
		// Generate symbol ID from location (simplified)
		symbolID := fmt.Sprintf("symbol_%s_%d_%d", location.URI,
			location.Range.Start.Line, location.Range.Start.Character)

		// Create SCIP occurrence
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      int32(location.Range.Start.Line),
					Character: int32(location.Range.Start.Character),
				},
				End: types.Position{
					Line:      int32(location.Range.End.Line),
					Character: int32(location.Range.End.Character),
				},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleDefinition,
		}

		// Store as single-occurrence document
		scipDoc := &scip.SCIPDocument{
			URI:          location.URI,
			Language:     m.detectLanguageFromURI(location.URI),
			Occurrences:  []scip.SCIPOccurrence{occurrence},
			LastModified: time.Now(),
			Size:         100,
		}

		if err := m.scipStorage.StoreDocument(context.Background(), scipDoc); err != nil {
			return fmt.Errorf("failed to store definition: %w", err)
		}
	}

	return nil
}

// storeReferencesResult stores reference results as SCIP occurrences
func (m *SCIPCacheManager) storeReferencesResult(uri string, response interface{}) error {
	locations, ok := response.([]lsp.Location)
	if !ok {
		return fmt.Errorf("invalid references response type")
	}

	// Group locations by document
	docOccurrences := make(map[string][]scip.SCIPOccurrence)

	for _, location := range locations {
		// Generate symbol ID (simplified)
		symbolID := fmt.Sprintf("symbol_%s_%d_%d", uri,
			location.Range.Start.Line, location.Range.Start.Character)

		// Create SCIP occurrence
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      int32(location.Range.Start.Line),
					Character: int32(location.Range.Start.Character),
				},
				End: types.Position{
					Line:      int32(location.Range.End.Line),
					Character: int32(location.Range.End.Character),
				},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleReadAccess, // References are read access
		}

		docOccurrences[location.URI] = append(docOccurrences[location.URI], occurrence)
	}

	// Store each document
	for docURI, occurrences := range docOccurrences {
		scipDoc := &scip.SCIPDocument{
			URI:          docURI,
			Language:     m.detectLanguageFromURI(docURI),
			Occurrences:  occurrences,
			LastModified: time.Now(),
			Size:         int64(len(occurrences) * 50),
		}

		if err := m.scipStorage.StoreDocument(context.Background(), scipDoc); err != nil {
			return fmt.Errorf("failed to store references: %w", err)
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

	if err := m.scipStorage.StoreSymbolInformation(context.Background(), &symbolInfo); err != nil {
		return fmt.Errorf("failed to store hover information: %w", err)
	}

	return nil
}

// storeDocumentSymbolResult stores document symbols as definition occurrences
func (m *SCIPCacheManager) storeDocumentSymbolResult(uri string, response interface{}) error {
	symbols, ok := response.([]lsp.SymbolInformation)
	if !ok {
		return fmt.Errorf("invalid document symbol response type")
	}

	language := m.detectLanguageFromURI(uri)
	scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbols)
	if err != nil {
		return fmt.Errorf("failed to convert symbols: %w", err)
	}

	if err := m.scipStorage.StoreDocument(context.Background(), scipDoc); err != nil {
		return fmt.Errorf("failed to store document symbols: %w", err)
	}

	return nil
}

// storeWorkspaceSymbolResult stores workspace symbols
func (m *SCIPCacheManager) storeWorkspaceSymbolResult(response interface{}) error {
	symbols, ok := response.([]lsp.SymbolInformation)
	if !ok {
		return fmt.Errorf("invalid workspace symbol response type")
	}

	// Group symbols by document
	docSymbols := make(map[string][]lsp.SymbolInformation)
	for _, symbol := range symbols {
		docSymbols[symbol.Location.URI] = append(docSymbols[symbol.Location.URI], symbol)
	}

	// Store each document
	for uri, symbols := range docSymbols {
		language := m.detectLanguageFromURI(uri)
		scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbols)
		if err != nil {
			common.LSPLogger.Warn("Failed to convert workspace symbols for %s: %v", uri, err)
			continue
		}

		if err := m.scipStorage.StoreDocument(context.Background(), scipDoc); err != nil {
			common.LSPLogger.Warn("Failed to store workspace symbols for %s: %v", uri, err)
		}
	}

	return nil
}

// storeCompletionResult stores completion items (not typically cached as occurrences)
func (m *SCIPCacheManager) storeCompletionResult(uri string, params, response interface{}) error {
	// Completion items are typically not stored as occurrences
	// This is a placeholder for potential future enhancements
	common.LSPLogger.Debug("Completion result storage not implemented for occurrence-centric cache")
	return nil
}

// Helper methods for conversion between LSP and SCIP types

// convertLSPSymbolKindToSCIPKind converts LSP symbol kind to SCIP symbol kind
func (m *SCIPCacheManager) convertLSPSymbolKindToSCIPKind(kind lsp.SymbolKind) scip.SCIPSymbolKind {
	switch kind {
	case lsp.File:
		return scip.SCIPSymbolKindFile
	case lsp.Module:
		return scip.SCIPSymbolKindModule
	case lsp.Namespace:
		return scip.SCIPSymbolKindNamespace
	case lsp.Package:
		return scip.SCIPSymbolKindPackage
	case lsp.Class:
		return scip.SCIPSymbolKindClass
	case lsp.Method:
		return scip.SCIPSymbolKindMethod
	case lsp.Property:
		return scip.SCIPSymbolKindProperty
	case lsp.Field:
		return scip.SCIPSymbolKindField
	case lsp.Constructor:
		return scip.SCIPSymbolKindConstructor
	case lsp.Enum:
		return scip.SCIPSymbolKindEnum
	case lsp.Interface:
		return scip.SCIPSymbolKindInterface
	case lsp.Function:
		return scip.SCIPSymbolKindFunction
	case lsp.Variable:
		return scip.SCIPSymbolKindVariable
	case lsp.Constant:
		return scip.SCIPSymbolKindConstant
	default:
		return scip.SCIPSymbolKindUnknown
	}
}

// convertSCIPSymbolKindToLSP converts SCIP symbol kind back to LSP
func (m *SCIPCacheManager) convertSCIPSymbolKindToLSP(kind scip.SCIPSymbolKind) lsp.SymbolKind {
	switch kind {
	case scip.SCIPSymbolKindFile:
		return lsp.File
	case scip.SCIPSymbolKindModule:
		return lsp.Module
	case scip.SCIPSymbolKindNamespace:
		return lsp.Namespace
	case scip.SCIPSymbolKindPackage:
		return lsp.Package
	case scip.SCIPSymbolKindClass:
		return lsp.Class
	case scip.SCIPSymbolKindMethod:
		return lsp.Method
	case scip.SCIPSymbolKindProperty:
		return lsp.Property
	case scip.SCIPSymbolKindField:
		return lsp.Field
	case scip.SCIPSymbolKindConstructor:
		return lsp.Constructor
	case scip.SCIPSymbolKindEnum:
		return lsp.Enum
	case scip.SCIPSymbolKindInterface:
		return lsp.Interface
	case scip.SCIPSymbolKindFunction:
		return lsp.Function
	case scip.SCIPSymbolKindVariable:
		return lsp.Variable
	case scip.SCIPSymbolKindConstant:
		return lsp.Constant
	default:
		return lsp.Variable
	}
}

// convertLSPSymbolKindToSyntaxKind converts LSP symbol kind to syntax kind for highlighting
func (m *SCIPCacheManager) convertLSPSymbolKindToSyntaxKind(kind lsp.SymbolKind) types.SyntaxKind {
	switch kind {
	case lsp.Function, lsp.Method:
		return types.SyntaxKindIdentifierFunction
	case lsp.Class:
		return types.SyntaxKindIdentifierType
	case lsp.Variable:
		return types.SyntaxKindIdentifierLocal
	case lsp.Constant:
		return types.SyntaxKindIdentifierConstant
	case lsp.Module, lsp.Namespace:
		return types.SyntaxKindIdentifierNamespace
	default:
		return types.SyntaxKindUnspecified
	}
}

// convertSCIPSymbolKindToCompletionItemKind converts SCIP symbol kind to completion item kind
func (m *SCIPCacheManager) convertSCIPSymbolKindToCompletionItemKind(kind scip.SCIPSymbolKind) lsp.CompletionItemKind {
	switch kind {
	case scip.SCIPSymbolKindFunction:
		return lsp.FunctionComp
	case scip.SCIPSymbolKindMethod:
		return lsp.MethodComp
	case scip.SCIPSymbolKindClass:
		return lsp.ClassComp
	case scip.SCIPSymbolKindVariable:
		return lsp.VariableComp
	case scip.SCIPSymbolKindConstant:
		return lsp.ConstantComp
	case scip.SCIPSymbolKindModule:
		return lsp.ModuleComp
	case scip.SCIPSymbolKindInterface:
		return lsp.InterfaceComp
	default:
		return lsp.Text
	}
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
	// In occurrence-centric design, we need to find which document contains this occurrence
	// This is a simplified approach - in practice, you'd track this more efficiently
	if m.scipStorage == nil {
		return ""
	}

	docs, err := m.scipStorage.ListDocuments(context.Background())
	if err != nil {
		return ""
	}

	for _, docURI := range docs {
		docOccs, err := m.scipStorage.GetOccurrences(context.Background(), docURI)
		if err != nil {
			continue
		}

		for _, docOcc := range docOccs {
			if docOcc.Symbol == occ.Symbol &&
				docOcc.Range.Start.Line == occ.Range.Start.Line &&
				docOcc.Range.Start.Character == occ.Range.Start.Character {
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
		return types.Position{}, fmt.Errorf("invalid parameters format")
	}

	positionMap, ok := paramsMap["position"].(map[string]interface{})
	if !ok {
		return types.Position{}, fmt.Errorf("no position in parameters")
	}

	line, ok := positionMap["line"].(float64)
	if !ok {
		return types.Position{}, fmt.Errorf("invalid line in position")
	}

	character, ok := positionMap["character"].(float64)
	if !ok {
		return types.Position{}, fmt.Errorf("invalid character in position")
	}

	return types.Position{
		Line:      int32(line),
		Character: int32(character),
	}, nil
}

// Query methods for SCIP storage

// querySymbolsFromSCIP queries symbols using SCIP storage
func (m *SCIPCacheManager) querySymbolsFromSCIP(ctx context.Context, query *IndexQuery) ([]interface{}, error) {
	if query.Symbol == "" {
		return []interface{}{}, nil
	}

	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, len(symbolInfos))
	for i, info := range symbolInfos {
		results[i] = info
	}

	return results, nil
}

// queryDefinitionsFromSCIP queries definitions using SCIP storage
func (m *SCIPCacheManager) queryDefinitionsFromSCIP(ctx context.Context, query *IndexQuery) ([]interface{}, error) {
	if query.Symbol == "" {
		return []interface{}{}, nil
	}

	defOcc, err := m.scipStorage.GetDefinitionOccurrence(ctx, query.Symbol)
	if err != nil {
		return []interface{}{}, nil // No error, just no results
	}

	return []interface{}{*defOcc}, nil
}

// queryReferencesFromSCIP queries references using SCIP storage
func (m *SCIPCacheManager) queryReferencesFromSCIP(ctx context.Context, query *IndexQuery) ([]interface{}, error) {
	if query.Symbol == "" {
		return []interface{}{}, nil
	}

	refOccs, err := m.scipStorage.GetReferenceOccurrences(ctx, query.Symbol)
	if err != nil {
		return []interface{}{}, nil // No error, just no results
	}

	results := make([]interface{}, len(refOccs))
	for i, occ := range refOccs {
		results[i] = occ
	}

	return results, nil
}

// queryWorkspaceSymbolsFromSCIP queries workspace symbols using SCIP storage
func (m *SCIPCacheManager) queryWorkspaceSymbolsFromSCIP(ctx context.Context, query *IndexQuery) ([]interface{}, error) {
	searchQuery := ""
	if query.Symbol != "" {
		searchQuery = query.Symbol
	}

	symbolInfos, err := m.scipStorage.GetWorkspaceSymbols(ctx, searchQuery)
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, len(symbolInfos))
	for i, info := range symbolInfos {
		results[i] = info
	}

	return results, nil
}

// QueryIndex queries the SCIP storage for symbols and relationships
func (m *SCIPCacheManager) QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error) {
	if !m.enabled || m.scipStorage == nil {
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
		results, err = m.querySymbolsFromSCIP(ctx, query)
	case "definition":
		results, err = m.queryDefinitionsFromSCIP(ctx, query)
	case "references":
		results, err = m.queryReferencesFromSCIP(ctx, query)
	case "workspace":
		results, err = m.queryWorkspaceSymbolsFromSCIP(ctx, query)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", query.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Get metadata from SCIP storage
	metadata := map[string]interface{}{
		"query_language": query.Language,
		"query_type":     query.Type,
	}

	if stats, err := m.scipStorage.GetStats(ctx); err == nil {
		metadata["total_symbols"] = stats.TotalSymbols
		metadata["total_documents"] = stats.CachedDocuments
		metadata["total_occurrences"] = stats.TotalOccurrences
	}

	return &IndexResult{
		Type:      query.Type,
		Results:   results,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}, nil
}

// GetIndexStats returns current index statistics from SCIP storage
func (m *SCIPCacheManager) GetIndexStats() *IndexStats {
	if !m.enabled {
		return &IndexStats{Status: "disabled"}
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *m.indexStats

	// Get current stats from SCIP storage
	if m.scipStorage != nil {
		if scipStats, err := m.scipStorage.GetStats(context.Background()); err == nil {
			stats.SymbolCount = scipStats.TotalSymbols
			stats.DocumentCount = int64(scipStats.CachedDocuments)
			stats.IndexSize = scipStats.MemoryUsage
		}
	}

	return &stats
}

// UpdateIndex updates the index with the given files (SimpleCache interface)
func (m *SCIPCacheManager) UpdateIndex(ctx context.Context, files []string) error {
	if !m.enabled {
		return nil
	}

	// This method triggers reindexing of specified files in SCIP storage
	// The actual indexing is done when LSP symbols are received and converted to occurrences

	// For occurrence-centric approach, we don't proactively index files
	// Instead, indexing happens when LSP methods return symbol information
	return nil
}

// Direct SCIP Search Methods - Added after QueryIndex method for direct occurrence-based searching

// SearchSymbolsEnhanced performs direct SCIP symbol search using occurrence indexes with enhanced results
func (m *SCIPCacheManager) SearchSymbolsEnhanced(ctx context.Context, query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResult, error) {
	if !m.enabled || m.scipStorage == nil {
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

	// Set default limits
	maxResults := 100
	if query.MaxResults > 0 {
		maxResults = query.MaxResults
	}

	// Search symbols using SCIP storage with pattern matching
	searchSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Pattern, maxResults*2) // Get more for filtering
	if err != nil {
		return nil, fmt.Errorf("failed to search symbols in SCIP storage: %w", err)
	}

	var enhancedResults []EnhancedSymbolResult
	processedCount := 0

	for _, symbolInfo := range searchSymbols {
		if processedCount >= maxResults {
			break
		}

		// Filter by symbol kinds if specified
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

		// Get all occurrences for this symbol
		occurrences, err := m.scipStorage.GetOccurrencesBySymbol(ctx, symbolInfo.Symbol)
		if err != nil {
			continue
		}

		// Filter by symbol roles if specified
		if len(query.SymbolRoles) > 0 {
			filteredOccurrences := []scip.SCIPOccurrence{}
			for _, occ := range occurrences {
				for _, role := range query.SymbolRoles {
					if occ.SymbolRoles.HasRole(role) {
						filteredOccurrences = append(filteredOccurrences, occ)
						break
					}
				}
			}
			occurrences = filteredOccurrences
		}

		// Apply minimum occurrence filter
		if query.MinOccurrences > 0 && len(occurrences) < query.MinOccurrences {
			continue
		}

		// Apply maximum occurrence filter
		if query.MaxOccurrences > 0 && len(occurrences) > query.MaxOccurrences {
			continue
		}

		// Only include symbols with definition if requested
		if query.OnlyWithDefinition {
			hasDefinition := false
			for _, occ := range occurrences {
				if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
					hasDefinition = true
					break
				}
			}
			if !hasDefinition {
				continue
			}
		}

		// Build enhanced result
		enhancedResult := m.buildEnhancedSymbolResult(&symbolInfo, occurrences, query)
		enhancedResults = append(enhancedResults, enhancedResult)
		processedCount++
	}

	// Sort results if requested
	if query.SortBy != "" {
		m.sortEnhancedResults(enhancedResults, query.SortBy)
	}

	result := &EnhancedSymbolSearchResult{
		Symbols:   enhancedResults,
		Total:     len(enhancedResults),
		Truncated: len(searchSymbols) > maxResults,
		Query:     query,
		Metadata: map[string]interface{}{
			"scip_enabled":     true,
			"total_candidates": len(searchSymbols),
		},
		Timestamp: time.Now(),
	}

	return result, nil
}

// SearchReferencesEnhanced performs direct SCIP reference search using occurrence indexes with enhanced results
func (m *SCIPCacheManager) SearchReferencesEnhanced(ctx context.Context, symbolName string, filePattern string, options *ReferenceSearchOptions) (*ReferenceSearchResult, error) {
	if !m.enabled || m.scipStorage == nil {
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

	// Find symbols by name first
	symbolInfos, err := m.scipStorage.GetSymbolInformationByName(ctx, symbolName)
	if err != nil || len(symbolInfos) == 0 {
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

	// Process each matching symbol (handle overloaded symbols)
	for _, symbolInfo := range symbolInfos {
		symbolID = symbolInfo.Symbol

		// Get reference occurrences
		refOccurrences, err := m.scipStorage.GetReferenceOccurrences(ctx, symbolInfo.Symbol)
		if err == nil {
			for _, occ := range refOccurrences {
				// Filter by file pattern if specified
				docURI := m.extractURIFromOccurrence(&occ)
				if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
					continue
				}

				// Filter by symbol roles if specified
				if len(options.SymbolRoles) > 0 {
					roleMatched := false
					for _, role := range options.SymbolRoles {
						if occ.SymbolRoles.HasRole(role) {
							roleMatched = true
							break
						}
					}
					if !roleMatched {
						continue
					}
				}

				occInfo := m.buildOccurrenceInfo(&occ, docURI)
				allReferences = append(allReferences, occInfo)
				fileSet[docURI] = true

				if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
					break
				}
			}
		}

		// Get definition if requested
		if options.IncludeDefinition && definition == nil {
			if defOcc, err := m.scipStorage.GetDefinitionOccurrence(ctx, symbolInfo.Symbol); err == nil {
				docURI := m.extractURIFromOccurrence(defOcc)
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					definition = &SCIPOccurrenceInfo{}
					*definition = m.buildOccurrenceInfo(defOcc, docURI)
					fileSet[docURI] = true
				}
			}
		}

		// Break if we have enough results
		if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
			break
		}
	}

	// Sort results if requested
	if options.SortBy != "" {
		m.sortOccurrenceResults(allReferences, options.SortBy)
	}

	result := &ReferenceSearchResult{
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
	}

	return result, nil
}

// SearchDefinitionsEnhanced performs direct SCIP definition search using occurrence indexes with enhanced results
func (m *SCIPCacheManager) SearchDefinitionsEnhanced(ctx context.Context, symbolName string, filePattern string) (*DefinitionSearchResult, error) {
	if !m.enabled || m.scipStorage == nil {
		return &DefinitionSearchResult{
			SymbolName:  symbolName,
			Definitions: []SCIPOccurrenceInfo{},
			TotalCount:  0,
			FileCount:   0,
			Metadata:    map[string]interface{}{"cache_disabled": true},
			Timestamp:   time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Find symbols by name
	symbolInfos, err := m.scipStorage.GetSymbolInformationByName(ctx, symbolName)
	if err != nil || len(symbolInfos) == 0 {
		return &DefinitionSearchResult{
			SymbolName:  symbolName,
			Definitions: []SCIPOccurrenceInfo{},
			TotalCount:  0,
			FileCount:   0,
			Metadata:    map[string]interface{}{"symbols_found": 0},
			Timestamp:   time.Now(),
		}, nil
	}

	var allDefinitions []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	symbolID := ""

	// Get definitions for each matching symbol
	for _, symbolInfo := range symbolInfos {
		symbolID = symbolInfo.Symbol

		if defOcc, err := m.scipStorage.GetDefinitionOccurrence(ctx, symbolInfo.Symbol); err == nil {
			docURI := m.extractURIFromOccurrence(defOcc)

			// Filter by file pattern if specified
			if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
				continue
			}

			occInfo := m.buildOccurrenceInfo(defOcc, docURI)
			allDefinitions = append(allDefinitions, occInfo)
			fileSet[docURI] = true
		}
	}

	result := &DefinitionSearchResult{
		SymbolName:  symbolName,
		SymbolID:    symbolID,
		Definitions: allDefinitions,
		TotalCount:  len(allDefinitions),
		FileCount:   len(fileSet),
		Metadata: map[string]interface{}{
			"scip_enabled":  true,
			"symbols_found": len(symbolInfos),
			"file_pattern":  filePattern,
		},
		Timestamp: time.Now(),
	}

	return result, nil
}

// GetSymbolInfoEnhanced performs direct SCIP symbol information retrieval with occurrence details and enhanced results
func (m *SCIPCacheManager) GetSymbolInfoEnhanced(ctx context.Context, symbolName string, filePattern string) (*SymbolInfoResult, error) {
	if !m.enabled || m.scipStorage == nil {
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

	// Find symbols by name
	symbolInfos, err := m.scipStorage.GetSymbolInformationByName(ctx, symbolName)
	if err != nil || len(symbolInfos) == 0 {
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

	// Use the first matching symbol (could be enhanced to handle multiple)
	symbolInfo := symbolInfos[0]
	symbolID := symbolInfo.Symbol

	// Get all occurrences for this symbol
	allOccurrences, err := m.scipStorage.GetOccurrencesBySymbol(ctx, symbolID)
	if err != nil {
		allOccurrences = []scip.SCIPOccurrence{}
	}

	// Filter occurrences by file pattern and build occurrence info
	var filteredOccurrences []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	definitionCount := 0
	referenceCount := 0

	for _, occ := range allOccurrences {
		docURI := m.extractURIFromOccurrence(&occ)

		// Filter by file pattern if specified
		if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
			continue
		}

		occInfo := m.buildOccurrenceInfo(&occ, docURI)
		filteredOccurrences = append(filteredOccurrences, occInfo)
		fileSet[docURI] = true

		// Count role types
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			definitionCount++
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			referenceCount++
		}
	}

	// Get relationships
	relationships, err := m.scipStorage.GetSymbolRelationships(ctx, symbolID)
	if err != nil {
		relationships = []scip.SCIPRelationship{}
	}

	// Format signature from symbol information
	signature := m.formatSymbolDetail(&symbolInfo)

	result := &SymbolInfoResult{
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
	}

	return result, nil
}

func (m *SCIPCacheManager) updateIndexStats(language string, symbolCount int) {
	m.indexStats.LastUpdate = time.Now()
	m.indexStats.Status = "active"

	if m.indexStats.LanguageStats[language] == 0 {
		m.indexStats.IndexedLanguages = append(m.indexStats.IndexedLanguages, language)
	}
	m.indexStats.LanguageStats[language] += int64(symbolCount)

	// Get stats from SCIP storage if available
	if m.scipStorage != nil {
		if stats, err := m.scipStorage.GetStats(context.Background()); err == nil {
			m.indexStats.SymbolCount = stats.TotalSymbols
			m.indexStats.DocumentCount = int64(stats.CachedDocuments)
		}
	}

}

// GetCachedDefinition retrieves definition occurrences for a symbol using SCIP storage
func (m *SCIPCacheManager) GetCachedDefinition(symbolID string) ([]lsp.Location, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get definition occurrence from SCIP storage
	definitionOcc, err := m.scipStorage.GetDefinitionOccurrence(context.Background(), symbolID)
	if err != nil {
		return nil, false
	}

	// Convert SCIP occurrence to LSP location
	location := lsp.Location{
		URI: m.extractURIFromOccurrence(definitionOcc),
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      int(definitionOcc.Range.Start.Line),
				Character: int(definitionOcc.Range.Start.Character),
			},
			End: lsp.Position{
				Line:      int(definitionOcc.Range.End.Line),
				Character: int(definitionOcc.Range.End.Character),
			},
		},
	}

	return []lsp.Location{location}, true
}

// GetCachedReferences retrieves reference occurrences for a symbol using SCIP storage
func (m *SCIPCacheManager) GetCachedReferences(symbolID string) ([]lsp.Location, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get reference occurrences from SCIP storage
	referenceOccs, err := m.scipStorage.GetReferenceOccurrences(context.Background(), symbolID)
	if err != nil || len(referenceOccs) == 0 {
		return nil, false
	}

	// Convert SCIP occurrences to LSP locations
	locations := make([]lsp.Location, 0, len(referenceOccs))
	for _, occ := range referenceOccs {
		location := lsp.Location{
			URI: m.extractURIFromOccurrence(&occ),
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      int(occ.Range.Start.Line),
					Character: int(occ.Range.Start.Character),
				},
				End: lsp.Position{
					Line:      int(occ.Range.End.Line),
					Character: int(occ.Range.End.Character),
				},
			},
		}
		locations = append(locations, location)
	}

	return locations, true
}

// GetCachedHover retrieves hover information using symbol information from SCIP storage
func (m *SCIPCacheManager) GetCachedHover(symbolID string) (*lsp.Hover, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get symbol information from SCIP storage
	symbolInfo, err := m.scipStorage.GetSymbolInformation(context.Background(), symbolID)
	if err != nil {
		return nil, false
	}

	// Convert SCIP symbol information to LSP hover
	hover := &lsp.Hover{
		Contents: m.formatHoverFromSCIPSymbolInfo(symbolInfo),
	}

	return hover, true
}

// GetCachedDocumentSymbols retrieves document symbols using SCIP storage
func (m *SCIPCacheManager) GetCachedDocumentSymbols(uri string) ([]lsp.SymbolInformation, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get document symbols from SCIP storage
	symbolInfos, err := m.scipStorage.GetDocumentSymbols(context.Background(), uri)
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	// Convert SCIP symbol information to LSP symbol information
	symbols := make([]lsp.SymbolInformation, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		// Get definition occurrence for location
		defOcc, err := m.scipStorage.GetDefinitionOccurrence(context.Background(), scipSymbol.Symbol)
		if err != nil {
			continue
		}

		symbol := lsp.SymbolInformation{
			Name: scipSymbol.DisplayName,
			Kind: m.convertSCIPSymbolKindToLSP(scipSymbol.Kind),
			Location: lsp.Location{
				URI: uri,
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      int(defOcc.Range.Start.Line),
						Character: int(defOcc.Range.Start.Character),
					},
					End: lsp.Position{
						Line:      int(defOcc.Range.End.Line),
						Character: int(defOcc.Range.End.Character),
					},
				},
			},
		}
		symbols = append(symbols, symbol)
	}

	return symbols, true
}

// GetCachedWorkspaceSymbols retrieves workspace symbols using SCIP storage
func (m *SCIPCacheManager) GetCachedWorkspaceSymbols(query string) ([]lsp.SymbolInformation, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Search symbols in SCIP storage
	symbolInfos, err := m.scipStorage.GetWorkspaceSymbols(context.Background(), query)
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	// Convert SCIP symbol information to LSP symbol information
	symbols := make([]lsp.SymbolInformation, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		// Get definition occurrence for location
		defOcc, err := m.scipStorage.GetDefinitionOccurrence(context.Background(), scipSymbol.Symbol)
		if err != nil {
			continue
		}

		symbol := lsp.SymbolInformation{
			Name: scipSymbol.DisplayName,
			Kind: m.convertSCIPSymbolKindToLSP(scipSymbol.Kind),
			Location: lsp.Location{
				URI: m.extractURIFromOccurrence(defOcc),
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      int(defOcc.Range.Start.Line),
						Character: int(defOcc.Range.Start.Character),
					},
					End: lsp.Position{
						Line:      int(defOcc.Range.End.Line),
						Character: int(defOcc.Range.End.Character),
					},
				},
			},
		}
		symbols = append(symbols, symbol)
	}

	return symbols, true
}

// GetCachedCompletion retrieves completion items using symbol information from SCIP storage
func (m *SCIPCacheManager) GetCachedCompletion(uri string, position lsp.Position) ([]lsp.CompletionItem, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get document symbols for completion context
	symbolInfos, err := m.scipStorage.GetDocumentSymbols(context.Background(), uri)
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
	if !m.enabled || m.scipStorage == nil {
		return nil
	}

	// Extract URI from parameters
	uri := m.extractURI(params)
	if uri == "" {
		return fmt.Errorf("could not extract URI from parameters")
	}

	// Convert LSP response to SCIP occurrences based on method type
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
		common.LSPLogger.Debug("Method %s not supported for SCIP storage", method)
		return nil
	}
}

// =============================================================================
// Direct SCIP Search Methods for MCP Tools
// =============================================================================

// SearchSymbols provides direct access to SCIP symbol search for MCP tools
func (m *SCIPCacheManager) SearchSymbols(ctx context.Context, pattern string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled || m.scipStorage == nil {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	// Search using SCIP storage directly
	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, pattern, maxResults)
	if err != nil {
		return nil, fmt.Errorf("SCIP symbol search failed: %w", err)
	}

	// Enhance results with occurrence data for proper file locations
	results := make([]interface{}, 0, len(symbolInfos))
	for _, symbolInfo := range symbolInfos {
		// Get definition occurrence for this symbol to get file location
		defOcc, err := m.scipStorage.GetDefinitionOccurrence(ctx, symbolInfo.Symbol)
		if err != nil || defOcc == nil {
			// If no definition found, include basic symbol info
			results = append(results, symbolInfo)
			continue
		}
		
		// Find the document URI that contains this occurrence
		// We need to search through all documents to find which one has this occurrence
		documentURI := ""
		docs, _ := m.scipStorage.ListDocuments(ctx)
		for _, docURI := range docs {
			occs, _ := m.scipStorage.GetOccurrences(ctx, docURI)
			for _, occ := range occs {
				if occ.Symbol == defOcc.Symbol && occ.Range == defOcc.Range {
					documentURI = docURI
					break
				}
			}
			if documentURI != "" {
				break
			}
		}
		
		// Create enhanced result with both symbol info and occurrence data
		enhancedResult := map[string]interface{}{
			"symbolInfo": symbolInfo,
			"occurrence": defOcc,
			"filePath":   documentURI,
			"range":      defOcc.Range,
		}
		results = append(results, enhancedResult)
	}

	return results, nil
}

// SearchReferences provides direct access to SCIP reference search for MCP tools
func (m *SCIPCacheManager) SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled || m.scipStorage == nil {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	// First find symbol by name to get symbol ID
	symbolInfos, err := m.scipStorage.GetSymbolInformationByName(ctx, symbolName)
	if err != nil || len(symbolInfos) == 0 {
		return []interface{}{}, nil // No error, just empty results
	}

	var allReferences []interface{}

	// Get references for all matching symbol IDs
	for _, symbolInfo := range symbolInfos {
		// Get reference occurrences
		refOccurrences, err := m.scipStorage.GetReferenceOccurrences(ctx, symbolInfo.Symbol)
		if err != nil {
			continue
		}

		// Also include definition if requested (include by default)
		defOccurrence, err := m.scipStorage.GetDefinitionOccurrence(ctx, symbolInfo.Symbol)
		if err == nil {
			allReferences = append(allReferences, *defOccurrence)
		}

		// Add reference occurrences
		for _, refOcc := range refOccurrences {
			allReferences = append(allReferences, refOcc)
		}

		// Limit results
		if len(allReferences) >= maxResults {
			allReferences = allReferences[:maxResults]
			break
		}
	}

	return allReferences, nil
}

// SearchDefinitions provides direct access to SCIP definition search for MCP tools
func (m *SCIPCacheManager) SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled || m.scipStorage == nil {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	// First find symbol by name to get symbol ID
	symbolInfos, err := m.scipStorage.GetSymbolInformationByName(ctx, symbolName)
	if err != nil || len(symbolInfos) == 0 {
		return []interface{}{}, nil // No error, just empty results
	}

	var allDefinitions []interface{}

	// Get definitions for all matching symbol IDs
	for _, symbolInfo := range symbolInfos {
		// Get definition occurrence
		defOccurrence, err := m.scipStorage.GetDefinitionOccurrence(ctx, symbolInfo.Symbol)
		if err != nil {
			continue
		}

		allDefinitions = append(allDefinitions, *defOccurrence)

		// Limit results
		if len(allDefinitions) >= maxResults {
			allDefinitions = allDefinitions[:maxResults]
			break
		}
	}

	return allDefinitions, nil
}

// GetSymbolInfo provides direct access to SCIP symbol information for MCP tools
func (m *SCIPCacheManager) GetSymbolInfo(ctx context.Context, symbolName string, filePattern string) (interface{}, error) {
	if !m.enabled || m.scipStorage == nil {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	// First find symbol by name to get symbol ID
	symbolInfos, err := m.scipStorage.GetSymbolInformationByName(ctx, symbolName)
	if err != nil || len(symbolInfos) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbolName)
	}

	// Return the first matching symbol info for now
	// TODO: Could enhance to handle multiple matches or exact matching
	return symbolInfos[0], nil
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
			if relationships, err := m.scipStorage.GetSymbolRelationships(context.Background(), symbolInfo.Symbol); err == nil {
				result.Relationships = relationships
			}
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
