package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
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

// SCIPSymbol represents a symbol in the SCIP index
type SCIPSymbol struct {
	Name          string              `json:"name"`
	Kind          string              `json:"kind"`
	Language      string              `json:"language"`
	URI           string              `json:"uri"`
	Range         *Range              `json:"range,omitempty"`
	Documentation []string            `json:"documentation,omitempty"`
	Signature     string              `json:"signature,omitempty"`
	Relationships map[string][]string `json:"relationships,omitempty"`
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

// SCIPCache interface for the main cache operations with integrated SCIP indexing
type SCIPCache interface {
	Initialize(config *config.CacheConfig) error
	Start(ctx context.Context) error
	Stop() error
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	HealthCheck() (*CacheMetrics, error)
	GetMetrics() *CacheMetrics
	Clear() error // Clear all cache entries

	// SCIP indexing capabilities - integrated as core functionality
	IndexDocument(ctx context.Context, uri string, language string, content []byte) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error
}

// SimpleCacheManager implements simple in-memory cache with integrated SCIP indexing
type SimpleCacheManager struct {
	entries map[string]*CacheEntry
	config  *config.CacheConfig
	stats   *SimpleCacheStats
	mu      sync.RWMutex
	enabled bool
	started bool

	// SCIP indexing - integrated as core functionality
	scipIndex     map[string]*SCIPSymbol            // symbol name -> symbol info
	documentIndex map[string][]string               // uri -> symbol names in document
	languageIndex map[string]map[string]*SCIPSymbol // language -> symbol name -> symbol
	indexStats    *IndexStats
	indexMu       sync.RWMutex

	// File watching for real-time index updates
	fileModTimes   map[string]time.Time // file path -> last modification time
	watcherTicker  *time.Ticker         // periodic file change checker
	watcherStop    chan struct{}        // signal to stop file watcher
	projectRoot    string               // root directory to watch
	watcherRunning bool                 // whether file watcher is active
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

		// Initialize SCIP indexing
		scipIndex:     make(map[string]*SCIPSymbol),
		documentIndex: make(map[string][]string),
		languageIndex: make(map[string]map[string]*SCIPSymbol),

		// Initialize file watching
		fileModTimes: make(map[string]time.Time),
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

	// Load index from disk if disk cache is enabled
	if manager.enabled && configParam.DiskCache && configParam.StoragePath != "" {
		if err := manager.LoadIndexFromDisk(); err != nil {
			common.LSPLogger.Warn("Failed to load index from disk, starting with empty index: %v", err)
		}
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

	// Start file watcher for real-time index updates
	if m.config.BackgroundIndex {
		if err := m.startFileWatcher(); err != nil {
			common.LSPLogger.Warn("Failed to start file watcher: %v", err)
			// Continue without file watching
		}
	}

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

	// Stop file watcher if running
	if m.watcherRunning {
		m.stopFileWatcher()
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
		common.LSPLogger.Debug("Store called but cache not enabled")
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

	common.LSPLogger.Debug("Attempting to store cache entry for method=%s, key=%s", method, keyStr)

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
		return &CacheMetrics{
			HealthStatus:    "DISABLED",
			LastHealthCheck: time.Now(),
		}, nil
	}

	// Convert simple stats to CacheMetrics format
	metrics := &CacheMetrics{
		HitCount:        m.stats.HitCount,
		MissCount:       m.stats.MissCount,
		ErrorCount:      m.stats.ErrorCount,
		TotalSize:       m.stats.TotalSize,
		EntryCount:      m.stats.EntryCount,
		EvictionCount:   0, // Not tracked in simple version
		HealthStatus:    "OK",
		LastHealthCheck: time.Now(),
	}

	return metrics, nil
}

// GetMetrics returns current cache metrics
func (m *SimpleCacheManager) GetMetrics() *CacheMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return &CacheMetrics{
			HealthStatus:    "DISABLED",
			LastHealthCheck: time.Now(),
		}
	}

	// Convert simple stats to CacheMetrics format
	metrics := &CacheMetrics{
		HitCount:        m.stats.HitCount,
		MissCount:       m.stats.MissCount,
		ErrorCount:      m.stats.ErrorCount,
		TotalSize:       m.stats.TotalSize,
		EntryCount:      m.stats.EntryCount,
		EvictionCount:   0, // Not tracked in simple version
		HealthStatus:    "OK",
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

// SCIP Indexing Implementation

// IndexDocument indexes a document's symbols for SCIP queries
func (m *SimpleCacheManager) IndexDocument(ctx context.Context, uri string, language string, content []byte) error {
	if !m.enabled {
		return nil // Graceful degradation when cache is disabled
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	// Clear existing symbols for this document
	if existingSymbols, exists := m.documentIndex[uri]; exists {
		for _, symbolName := range existingSymbols {
			delete(m.scipIndex, symbolName)
		}
	}

	// Extract symbols from document content based on language
	symbols := m.extractSymbolsFromContent(uri, language, content)

	// Store symbols in index
	symbolNames := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbolKey := fmt.Sprintf("%s:%s", symbol.Language, symbol.Name)
		m.scipIndex[symbolKey] = symbol
		symbolNames = append(symbolNames, symbolKey)

		// Update language index
		if m.languageIndex[language] == nil {
			m.languageIndex[language] = make(map[string]*SCIPSymbol)
		}
		m.languageIndex[language][symbol.Name] = symbol
	}

	// Update document index
	m.documentIndex[uri] = symbolNames

	// Update statistics
	m.updateIndexStats(language, len(symbols))

	common.LSPLogger.Debug("Indexed %d symbols from %s (%s)", len(symbols), uri, language)
	return nil
}

// QueryIndex queries the SCIP index for symbols and relationships
func (m *SimpleCacheManager) QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error) {
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

	switch query.Type {
	case "symbol":
		results = m.querySymbols(query)
	case "definition":
		results = m.queryDefinitions(query)
	case "references":
		results = m.queryReferences(query)
	case "workspace":
		results = m.queryWorkspaceSymbols(query)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", query.Type)
	}

	return &IndexResult{
		Type:    query.Type,
		Results: results,
		Metadata: map[string]interface{}{
			"indexed_symbols": len(m.scipIndex),
			"query_language":  query.Language,
		},
		Timestamp: time.Now(),
	}, nil
}

// GetIndexStats returns current index statistics
func (m *SimpleCacheManager) GetIndexStats() *IndexStats {
	if !m.enabled {
		return &IndexStats{Status: "disabled"}
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *m.indexStats
	stats.SymbolCount = int64(len(m.scipIndex))
	stats.DocumentCount = int64(len(m.documentIndex))
	return &stats
}

// UpdateIndex updates the index with the given files
func (m *SimpleCacheManager) UpdateIndex(ctx context.Context, files []string) error {
	if !m.enabled {
		return nil
	}

	common.LSPLogger.Info("Indexing %d files...", len(files))

	for i, file := range files {
		// Check context cancellation
		select {
		case <-ctx.Done():
			common.LSPLogger.Warn("Index update cancelled after %d/%d files", i, len(files))
			return ctx.Err()
		default:
		}

		language := m.detectLanguageFromFile(file)
		if language == "" {
			continue
		}

		// Read actual file content
		content, err := os.ReadFile(file)
		if err != nil {
			common.LSPLogger.Warn("Failed to read file %s: %v", file, err)
			continue
		}

		// Skip empty files
		if len(content) == 0 {
			continue
		}

		if err := m.IndexDocument(ctx, file, language, content); err != nil {
			common.LSPLogger.Warn("Failed to index file %s: %v", file, err)
		}
	}

	common.LSPLogger.Info("Indexed %d documents with %d symbols", len(m.documentIndex), len(m.scipIndex))

	// Save index to disk if disk cache is enabled
	if m.config.DiskCache && m.config.StoragePath != "" {
		if err := m.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Warn("Failed to save index to disk: %v", err)
		}
	}

	return nil
}

// Helper methods for SCIP indexing

func (m *SimpleCacheManager) extractSymbolsFromContent(uri, language string, content []byte) []*SCIPSymbol {
	// Pragmatic symbol extraction based on language
	var symbols []*SCIPSymbol
	contentStr := string(content)

	switch language {
	case "go":
		symbols = m.extractGoSymbols(uri, contentStr)
	case "python":
		symbols = m.extractPythonSymbols(uri, contentStr)
	case "javascript", "typescript":
		symbols = m.extractJSSymbols(uri, contentStr)
	case "java":
		symbols = m.extractJavaSymbols(uri, contentStr)
	}

	return symbols
}

func (m *SimpleCacheManager) extractGoSymbols(uri, content string) []*SCIPSymbol {
	var symbols []*SCIPSymbol

	// Simple regex patterns for Go symbols
	patterns := map[string]string{
		"function": `func\s+(?:\([^)]*\)\s+)?(\w+)\s*\([^)]*\)`, // Handles both functions and methods with receivers
		"type":     `type\s+(\w+)\s+`,
		"const":    `const\s+(\w+)\s*=`,
		"var":      `var\s+(\w+)\s+`,
	}

	for kind, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbols = append(symbols, &SCIPSymbol{
					Name:     match[1],
					Kind:     kind,
					Language: "go",
					URI:      uri,
					Range:    nil, // Would need more sophisticated parsing for exact positions
				})
			}
		}
	}

	return symbols
}

func (m *SimpleCacheManager) extractPythonSymbols(uri, content string) []*SCIPSymbol {
	var symbols []*SCIPSymbol

	patterns := map[string]string{
		"function": `def\s+(\w+)\s*\(`,
		"class":    `class\s+(\w+)\s*[\(:]`,
	}

	for kind, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbols = append(symbols, &SCIPSymbol{
					Name:     match[1],
					Kind:     kind,
					Language: "python",
					URI:      uri,
				})
			}
		}
	}

	return symbols
}

func (m *SimpleCacheManager) extractJSSymbols(uri, content string) []*SCIPSymbol {
	var symbols []*SCIPSymbol

	patterns := map[string]string{
		"function": `function\s+(\w+)\s*\(`,
		"class":    `class\s+(\w+)\s*{`,
		"const":    `const\s+(\w+)\s*=`,
		"let":      `let\s+(\w+)\s*=`,
		"var":      `var\s+(\w+)\s*=`,
	}

	for kind, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbols = append(symbols, &SCIPSymbol{
					Name:     match[1],
					Kind:     kind,
					Language: strings.TrimSuffix(filepath.Ext(uri), "script"), // js or ts
					URI:      uri,
				})
			}
		}
	}

	return symbols
}

func (m *SimpleCacheManager) extractJavaSymbols(uri, content string) []*SCIPSymbol {
	var symbols []*SCIPSymbol

	patterns := map[string]string{
		"class":     `class\s+(\w+)\s*{`,
		"interface": `interface\s+(\w+)\s*{`,
		"method":    `(?:public|private|protected)?\s*(?:static)?\s*\w+\s+(\w+)\s*\(`,
	}

	for kind, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbols = append(symbols, &SCIPSymbol{
					Name:     match[1],
					Kind:     kind,
					Language: "java",
					URI:      uri,
				})
			}
		}
	}

	return symbols
}

func (m *SimpleCacheManager) querySymbols(query *IndexQuery) []interface{} {
	var results []interface{}

	if query.Language != "" {
		// Query specific language
		if langIndex, exists := m.languageIndex[query.Language]; exists {
			for _, symbol := range langIndex {
				if query.Symbol == "" || strings.Contains(symbol.Name, query.Symbol) {
					results = append(results, symbol)
				}
			}
		}
	} else {
		// Query all languages
		for _, symbol := range m.scipIndex {
			if query.Symbol == "" || strings.Contains(symbol.Name, query.Symbol) {
				results = append(results, symbol)
			}
		}
	}

	return results
}

func (m *SimpleCacheManager) queryDefinitions(query *IndexQuery) []interface{} {
	// For now, return symbols matching the query
	// In a full implementation, this would return definition locations
	return m.querySymbols(query)
}

func (m *SimpleCacheManager) queryReferences(query *IndexQuery) []interface{} {
	// For now, return empty - references require more sophisticated analysis
	return []interface{}{}
}

func (m *SimpleCacheManager) queryWorkspaceSymbols(query *IndexQuery) []interface{} {
	return m.querySymbols(query)
}

func (m *SimpleCacheManager) detectLanguageFromFile(file string) string {
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".java":
		return "java"
	default:
		return ""
	}
}

func (m *SimpleCacheManager) updateIndexStats(language string, symbolCount int) {
	m.indexStats.LastUpdate = time.Now()
	m.indexStats.Status = "active"

	if m.indexStats.LanguageStats[language] == 0 {
		m.indexStats.IndexedLanguages = append(m.indexStats.IndexedLanguages, language)
	}
	m.indexStats.LanguageStats[language] += int64(symbolCount)
}

// SaveIndexToDisk saves the SCIP index data to disk as JSON files
func (m *SimpleCacheManager) SaveIndexToDisk() error {
	if m.config.StoragePath == "" {
		return fmt.Errorf("storage path not configured")
	}

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(m.config.StoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %v", err)
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Save SCIP index
	scipIndexPath := filepath.Join(m.config.StoragePath, "scip_index.json")
	scipIndexData, err := json.MarshalIndent(m.scipIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SCIP index: %v", err)
	}

	if err := os.WriteFile(scipIndexPath, scipIndexData, 0644); err != nil {
		return fmt.Errorf("failed to write SCIP index file: %v", err)
	}

	// Save document index
	documentIndexPath := filepath.Join(m.config.StoragePath, "document_index.json")
	documentIndexData, err := json.MarshalIndent(m.documentIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal document index: %v", err)
	}

	if err := os.WriteFile(documentIndexPath, documentIndexData, 0644); err != nil {
		return fmt.Errorf("failed to write document index file: %v", err)
	}

	common.LSPLogger.Debug("Saved index to disk: %d symbols, %d documents", len(m.scipIndex), len(m.documentIndex))
	return nil
}

// LoadIndexFromDisk loads the SCIP index data from disk JSON files
func (m *SimpleCacheManager) LoadIndexFromDisk() error {
	if m.config.StoragePath == "" {
		return fmt.Errorf("storage path not configured")
	}

	scipIndexPath := filepath.Join(m.config.StoragePath, "scip_index.json")
	documentIndexPath := filepath.Join(m.config.StoragePath, "document_index.json")

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	// Load SCIP index if it exists
	if _, err := os.Stat(scipIndexPath); err == nil {
		scipIndexData, err := os.ReadFile(scipIndexPath)
		if err != nil {
			return fmt.Errorf("failed to read SCIP index file: %v", err)
		}

		if err := json.Unmarshal(scipIndexData, &m.scipIndex); err != nil {
			return fmt.Errorf("failed to unmarshal SCIP index: %v", err)
		}
	}

	// Load document index if it exists
	if _, err := os.Stat(documentIndexPath); err == nil {
		documentIndexData, err := os.ReadFile(documentIndexPath)
		if err != nil {
			return fmt.Errorf("failed to read document index file: %v", err)
		}

		if err := json.Unmarshal(documentIndexData, &m.documentIndex); err != nil {
			return fmt.Errorf("failed to unmarshal document index: %v", err)
		}
	}

	// Rebuild language index from loaded data
	m.languageIndex = make(map[string]map[string]*SCIPSymbol)
	for _, symbol := range m.scipIndex {
		if m.languageIndex[symbol.Language] == nil {
			m.languageIndex[symbol.Language] = make(map[string]*SCIPSymbol)
		}
		m.languageIndex[symbol.Language][symbol.Name] = symbol
	}

	// Update index stats
	m.indexStats.SymbolCount = int64(len(m.scipIndex))
	m.indexStats.DocumentCount = int64(len(m.documentIndex))
	m.indexStats.Status = "loaded"

	// Update language stats
	languageStats := make(map[string]int64)
	indexedLanguages := []string{}
	for language, symbols := range m.languageIndex {
		languageStats[language] = int64(len(symbols))
		indexedLanguages = append(indexedLanguages, language)
	}
	m.indexStats.LanguageStats = languageStats
	m.indexStats.IndexedLanguages = indexedLanguages

	common.LSPLogger.Info("Loaded index from disk: %d symbols, %d documents", len(m.scipIndex), len(m.documentIndex))
	return nil
}

// File watching methods for real-time index updates

// startFileWatcher starts periodic file change detection
func (m *SimpleCacheManager) startFileWatcher() error {
	// Get working directory as project root
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	m.projectRoot = wd

	// Initialize stop channel
	m.watcherStop = make(chan struct{})

	// Create ticker for periodic checks (every 5 seconds)
	m.watcherTicker = time.NewTicker(5 * time.Second)
	m.watcherRunning = true

	// Start goroutine for file watching
	go m.fileWatcherLoop()

	common.LSPLogger.Info("Started file watcher for project: %s", m.projectRoot)
	return nil
}

// stopFileWatcher stops the file watcher
func (m *SimpleCacheManager) stopFileWatcher() {
	if m.watcherTicker != nil {
		m.watcherTicker.Stop()
	}
	if m.watcherRunning {
		close(m.watcherStop)
		m.watcherRunning = false
	}
	common.LSPLogger.Info("Stopped file watcher")
}

// fileWatcherLoop runs the file watching loop
func (m *SimpleCacheManager) fileWatcherLoop() {
	for {
		select {
		case <-m.watcherTicker.C:
			m.scanAndIndexChangedFiles()
		case <-m.watcherStop:
			return
		}
	}
}

// scanAndIndexChangedFiles scans for changed files and re-indexes them
func (m *SimpleCacheManager) scanAndIndexChangedFiles() {
	extensions := []string{".go", ".py", ".js", ".ts", ".tsx", ".java"}
	changedFiles := []string{}

	// Walk through project directory
	err := filepath.Walk(m.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories and non-source files
		if info.IsDir() {
			name := info.Name()
			// Skip common non-source directories
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
				name == "build" || name == "dist" || name == "target" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file has relevant extension
		ext := filepath.Ext(path)
		isRelevant := false
		for _, validExt := range extensions {
			if ext == validExt {
				isRelevant = true
				break
			}
		}

		if !isRelevant {
			return nil
		}

		// Check if file has been modified
		m.mu.RLock()
		lastModTime, exists := m.fileModTimes[path]
		m.mu.RUnlock()

		if !exists || info.ModTime().After(lastModTime) {
			changedFiles = append(changedFiles, path)

			// Update modification time
			m.mu.Lock()
			m.fileModTimes[path] = info.ModTime()
			m.mu.Unlock()
		}

		return nil
	})

	if err != nil {
		common.LSPLogger.Warn("Error scanning files: %v", err)
		return
	}

	// Re-index changed files
	if len(changedFiles) > 0 {
		common.LSPLogger.Info("Detected %d file changes, re-indexing...", len(changedFiles))
		ctx := context.Background()

		for _, file := range changedFiles {
			language := m.detectLanguageFromFile(file)
			if language == "" {
				continue
			}

			// Read file content
			content, err := os.ReadFile(file)
			if err != nil {
				common.LSPLogger.Warn("Failed to read changed file %s: %v", file, err)
				continue
			}

			// Re-index the file
			if err := m.IndexDocument(ctx, file, language, content); err != nil {
				common.LSPLogger.Warn("Failed to re-index file %s: %v", file, err)
			}
		}

		// Save index to disk if disk cache is enabled
		if m.config.DiskCache && m.config.StoragePath != "" {
			if err := m.SaveIndexToDisk(); err != nil {
				common.LSPLogger.Warn("Failed to save updated index to disk: %v", err)
			}
		}

		common.LSPLogger.Info("Re-indexed %d changed files", len(changedFiles))
	}
}

// SCIPStorage interface implementation

// StorageAdapter creates an SCIPStorage adapter for SimpleCacheManager
type StorageAdapter struct {
	manager *SimpleCacheManager
}

// NewStorageAdapter creates a new storage adapter
func NewStorageAdapter(manager *SimpleCacheManager) SCIPStorage {
	return &StorageAdapter{manager: manager}
}

// Store implements SCIPStorage.Store
func (a *StorageAdapter) Store(key CacheKey, entry *CacheEntry) error {
	return a.manager.StoreEntry(key, entry)
}

// Retrieve implements SCIPStorage.Retrieve
func (a *StorageAdapter) Retrieve(key CacheKey) (*CacheEntry, error) {
	return a.manager.Retrieve(key)
}

// Delete implements SCIPStorage.Delete
func (a *StorageAdapter) Delete(key CacheKey) error {
	return a.manager.Delete(key)
}

// Clear implements SCIPStorage.Clear
func (a *StorageAdapter) Clear() error {
	return a.manager.Clear()
}

// Size implements SCIPStorage.Size
func (a *StorageAdapter) Size() int64 {
	return a.manager.Size()
}

// EntryCount implements SCIPStorage.EntryCount
func (a *StorageAdapter) EntryCount() int64 {
	return a.manager.EntryCount()
}

// StoreEntry stores a cache entry with the given key (internal method)
func (m *SimpleCacheManager) StoreEntry(key CacheKey, entry *CacheEntry) error {
	if !m.isEnabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := fmt.Sprintf("%s:%s:%s", key.Method, key.URI, key.Hash)
	m.entries[keyStr] = entry
	m.updateStats()
	return nil
}

// Retrieve retrieves a cache entry by key (SCIPStorage interface)
func (m *SimpleCacheManager) Retrieve(key CacheKey) (*CacheEntry, error) {
	if !m.isEnabled() {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	keyStr := fmt.Sprintf("%s:%s:%s", key.Method, key.URI, key.Hash)
	entry, exists := m.entries[keyStr]
	if !exists {
		return nil, nil
	}

	// Check TTL
	ttl := time.Duration(m.config.TTLHours) * time.Hour
	if !m.isValidEntry(entry, ttl) {
		return nil, nil
	}

	return entry, nil
}

// Delete removes a cache entry by key (SCIPStorage interface)
func (m *SimpleCacheManager) Delete(key CacheKey) error {
	if !m.isEnabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := fmt.Sprintf("%s:%s:%s", key.Method, key.URI, key.Hash)
	delete(m.entries, keyStr)
	m.updateStats()
	return nil
}

// Clear removes all cache entries (SCIPStorage interface)
func (m *SimpleCacheManager) Clear() error {
	if !m.isEnabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	common.LSPLogger.Info("Clearing all cache entries")
	m.entries = make(map[string]*CacheEntry)
	m.updateStats()
	return nil
}

// Size returns the total size of cached data (SCIPStorage interface)
func (m *SimpleCacheManager) Size() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getTotalSize()
}

// EntryCount returns the number of cache entries (SCIPStorage interface)
func (m *SimpleCacheManager) EntryCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.entries))
}

// MockSCIPStorageAdapter - Simple adapter for invalidation manager testing
type MockSCIPStorageAdapter struct{}

func NewMockSCIPStorageAdapter() SCIPStorage {
	return &MockSCIPStorageAdapter{}
}

func (a *MockSCIPStorageAdapter) Store(key CacheKey, entry *CacheEntry) error {
	return nil
}

func (a *MockSCIPStorageAdapter) Retrieve(key CacheKey) (*CacheEntry, error) {
	return nil, nil
}

func (a *MockSCIPStorageAdapter) Delete(key CacheKey) error {
	return nil
}

func (a *MockSCIPStorageAdapter) Clear() error {
	return nil
}

func (a *MockSCIPStorageAdapter) Size() int64 {
	return 0
}

func (a *MockSCIPStorageAdapter) EntryCount() int64 {
	return 0
}
