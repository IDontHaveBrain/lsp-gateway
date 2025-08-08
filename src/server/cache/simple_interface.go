package cache

import (
	"context"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
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
	GetCachedDefinition(symbolID string) ([]types.Location, bool)
	GetCachedReferences(symbolID string) ([]types.Location, bool)
	GetCachedHover(symbolID string) (*lsp.Hover, bool)
	GetCachedDocumentSymbols(uri string) ([]types.SymbolInformation, bool)
	GetCachedWorkspaceSymbols(query string) ([]types.SymbolInformation, bool)
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
