package cache

import (
	"context"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"strings"
)

// LSPFallback interface for fallback to actual LSP servers
type LSPFallback interface {
	ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
}

// SimpleCache provides a direct, streamlined cache interface with integrated SCIP indexing
type SimpleCache interface {
	// Core cache operations - direct and simple
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	Clear() error // Clear all cache entries

	// Lifecycle management
	Start(ctx context.Context) error
	Stop() error

	// Status and metrics
	GetMetrics() *CacheMetrics
	HealthCheck() (*CacheMetrics, error)
	IsEnabled() bool

	// SCIP indexing capabilities - integrated as core functionality
	IndexDocument(ctx context.Context, uri string, language string, symbols []lsp.SymbolInformation) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error

	// Cached retrieval methods
	GetCachedDefinition(symbolID string) ([]lsp.Location, bool)
	GetCachedReferences(symbolID string) ([]lsp.Location, bool)
	GetCachedHover(symbolID string) (*lsp.Hover, bool)
	GetCachedDocumentSymbols(uri string) ([]lsp.SymbolInformation, bool)
	GetCachedWorkspaceSymbols(query string) ([]lsp.SymbolInformation, bool)
	StoreMethodResult(method string, params interface{}, response interface{}) error
}

// DirectCacheIntegration demonstrates the new simplified integration pattern
// Use this as an example for integrating cache directly with LSP managers
type DirectCacheIntegration struct {
	cache   SimpleCache
	enabled bool
}

// NewDirectCacheIntegration creates a direct cache integration (simplified pattern)
func NewDirectCacheIntegration(cache SimpleCache) *DirectCacheIntegration {
	return &DirectCacheIntegration{
		cache:   cache,
		enabled: cache != nil && cache.IsEnabled(),
	}
}

// ProcessRequest demonstrates the optimal cache-first, LSP-fallback pattern
func (d *DirectCacheIntegration) ProcessRequest(ctx context.Context, method string, params interface{}, lspFallback func() (interface{}, error)) (interface{}, error) {
	// Optional cache check - graceful degradation if no cache
	if d.enabled && d.isCacheableMethod(method) {
		if result, found, err := d.cache.Lookup(method, params); err == nil && found {
			common.LSPLogger.Debug("Direct cache hit for method=%s", method)
			return result, nil
		}
	}

	// LSP fallback
	result, err := lspFallback()
	if err != nil {
		return nil, err
	}

	// Optional cache store - graceful degradation if no cache
	if d.enabled && d.isCacheableMethod(method) {
		if storeErr := d.cache.Store(method, params, result); storeErr != nil {
			common.LSPLogger.Debug("Failed to cache result for method=%s: %v", method, storeErr)
		}
	}

	return result, nil
}

// isCacheableMethod checks if a method should be cached
func (d *DirectCacheIntegration) isCacheableMethod(method string) bool {
	cacheableMethods := map[string]bool{
		"textDocument/definition":     true,
		"textDocument/references":     true,
		"textDocument/hover":          true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":            true,
		"textDocument/completion":     true,
	}
	return cacheableMethods[method]
}

// USAGE EXAMPLES:

// Example 1: Ultra-Simple Cache Creation
func ExampleSimpleCache() SimpleCache {
	// Single-line cache creation with graceful degradation
	cache, err := NewSimpleCache(256) // 256MB cache
	if err != nil {
		common.LSPLogger.Warn("Cache creation failed, using nil cache: %v", err)
		return nil // Graceful degradation
	}
	return cache
}

// Example 2: Optional Cache Injection Pattern
func ExampleOptionalCacheInjection(lspManager interface{ SetCache(cache SimpleCache) }) {
	// Create cache with graceful degradation
	cache := ExampleSimpleCache()

	// Inject cache (can be nil)
	if cache != nil {
		lspManager.SetCache(cache)
	}

	// LSP manager now works with or without cache
}

// Example 3: Occurrence-Centric Direct Integration Pattern
func ExampleOccurrenceDirectIntegration() {
	// Create cache with occurrence-centric support
	cache := ExampleSimpleCache()

	// Create direct integration
	integration := NewDirectCacheIntegration(cache)

	// Use with LSP fallback - now supports occurrence-centric caching
	result, err := integration.ProcessRequest(
		context.Background(),
		"textDocument/definition",
		map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://test.go"},
			"position":     map[string]interface{}{"line": 10, "character": 5},
		},
		func() (interface{}, error) {
			// LSP fallback function
			return []lsp.Location{
				{
					URI: "file://test.go",
					Range: lsp.Range{
						Start: lsp.Position{Line: 5, Character: 10},
						End:   lsp.Position{Line: 5, Character: 20},
					},
				},
			}, nil
		},
	)

	_ = result
	_ = err
}

// Example 4: Occurrence-Based Cache Usage
func ExampleOccurrenceBasedCache() {
	// Create cache
	cache := ExampleSimpleCache()

	// Direct occurrence-centric cache usage
	symbolID := "scip-go local 0.0.1 main/TestFunction"

	// Get cached definition with symbol role filtering
	if locations, found := cache.GetCachedDefinition(symbolID); found {
		common.LSPLogger.Info("Found %d definition locations for symbol: %s", len(locations), symbolID)
	}

	// Get cached references with symbol role filtering
	if locations, found := cache.GetCachedReferences(symbolID); found {
		common.LSPLogger.Info("Found %d reference locations for symbol: %s", len(locations), symbolID)
	}

	// Get cached hover information
	if hover, found := cache.GetCachedHover(symbolID); found {
		common.LSPLogger.Info("Found hover information for symbol: %s", symbolID)
		_ = hover
	}

	// Get cached document symbols
	if symbols, found := cache.GetCachedDocumentSymbols("file://test.go"); found {
		common.LSPLogger.Info("Found %d document symbols", len(symbols))
	}

	// Get cached workspace symbols with query
	if symbols, found := cache.GetCachedWorkspaceSymbols("Test"); found {
		common.LSPLogger.Info("Found %d workspace symbols matching 'Test'", len(symbols))
	}
}

// OccurrenceBasedCacheIntegration demonstrates advanced occurrence-centric cache usage
type OccurrenceBasedCacheIntegration struct {
	cache   SimpleCache
	enabled bool
}

// NewOccurrenceBasedCacheIntegration creates an occurrence-centric cache integration
func NewOccurrenceBasedCacheIntegration(cache SimpleCache) *OccurrenceBasedCacheIntegration {
	return &OccurrenceBasedCacheIntegration{
		cache:   cache,
		enabled: cache != nil && cache.IsEnabled(),
	}
}

// ProcessDefinitionRequest handles definition requests with occurrence-centric caching
func (o *OccurrenceBasedCacheIntegration) ProcessDefinitionRequest(ctx context.Context, uri string, position lsp.Position, lspFallback func() ([]lsp.Location, error)) ([]lsp.Location, error) {
	if !o.enabled {
		return lspFallback()
	}

	// Create symbol ID from position
	symbolID := o.createSymbolID(uri, position)

	// Try occurrence-centric cache first
	if locations, found := o.cache.GetCachedDefinition(symbolID); found {
		common.LSPLogger.Debug("Definition cache hit for symbol: %s", symbolID)
		return locations, nil
	}

	// Fallback to LSP
	locations, err := lspFallback()
	if err != nil {
		return nil, err
	}

	// Store result using occurrence-centric method
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": position.Line, "character": position.Character},
	}
	if storeErr := o.cache.StoreMethodResult("textDocument/definition", params, locations); storeErr != nil {
		common.LSPLogger.Debug("Failed to store definition result: %v", storeErr)
	}

	return locations, nil
}

// ProcessReferencesRequest handles references requests with occurrence-centric caching
func (o *OccurrenceBasedCacheIntegration) ProcessReferencesRequest(ctx context.Context, uri string, position lsp.Position, includeDeclaration bool, lspFallback func() ([]lsp.Location, error)) ([]lsp.Location, error) {
	if !o.enabled {
		return lspFallback()
	}

	// Create symbol ID from position
	symbolID := o.createSymbolID(uri, position)

	// Try occurrence-centric cache first
	var locations []lsp.Location
	var found bool

	// Get references (read access occurrences)
	if refLocations, refFound := o.cache.GetCachedReferences(symbolID); refFound {
		locations = append(locations, refLocations...)
		found = true
	}

	// Include definition if requested and available
	if includeDeclaration {
		if defLocations, defFound := o.cache.GetCachedDefinition(symbolID); defFound {
			locations = append(locations, defLocations...)
			found = true
		}
	}

	if found {
		common.LSPLogger.Debug("References cache hit for symbol: %s", symbolID)
		return locations, nil
	}

	// Fallback to LSP
	locations, err := lspFallback()
	if err != nil {
		return nil, err
	}

	// Store result using occurrence-centric method
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": position.Line, "character": position.Character},
		"context":      map[string]interface{}{"includeDeclaration": includeDeclaration},
	}
	if storeErr := o.cache.StoreMethodResult("textDocument/references", params, locations); storeErr != nil {
		common.LSPLogger.Debug("Failed to store references result: %v", storeErr)
	}

	return locations, nil
}

// createSymbolID creates a symbol ID from URI and position
func (o *OccurrenceBasedCacheIntegration) createSymbolID(uri string, position lsp.Position) string {
	// Simplified symbol ID creation - in practice, you'd use proper SCIP symbol format
	uriClean := strings.ReplaceAll(uri, ":", "_")
	uriClean = strings.ReplaceAll(uriClean, "/", "_")
	return strings.ToLower(uriClean + "_" + string(rune(position.Line)) + "_" + string(rune(position.Character)))
}

// FilterOccurrencesByRole demonstrates filtering occurrences by symbol roles
func FilterOccurrencesByRole(occurrences []types.SymbolOccurrence, role types.SymbolRole) []types.SymbolOccurrence {
	var filtered []types.SymbolOccurrence
	for _, occ := range occurrences {
		if occ.SymbolRoles.HasRole(role) {
			filtered = append(filtered, occ)
		}
	}
	return filtered
}

// ConvertOccurrencesToLSPLocations converts SCIP occurrences to LSP locations
func ConvertOccurrencesToLSPLocations(occurrences []types.SymbolOccurrence, documentURI string) []lsp.Location {
	locations := make([]lsp.Location, 0, len(occurrences))
	for _, occ := range occurrences {
		location := lsp.Location{
			URI: documentURI,
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
	return locations
}
