package cache

import (
	"context"
	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// SimpleCache provides a direct, streamlined cache interface with integrated SCIP indexing
// This replaces the deprecated wrapper layers (SimpleCacheIntegration, SCIPQueryManager)
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

	// Compatibility methods for SCIPCache interface - use proper config type
	Initialize(config *config.CacheConfig) error

	// SCIP indexing capabilities - integrated as core functionality
	IndexDocument(ctx context.Context, uri string, language string, content []byte) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error
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
func ExampleOptionalCacheInjection(lspManager interface{ SetCache(cache SCIPCache) }) {
	// Create cache with graceful degradation
	cache := ExampleSimpleCache()

	// Inject cache (can be nil) - SimpleCache implements SCIPCache now
	if cache != nil {
		lspManager.SetCache(cache)
	}

	// LSP manager now works with or without cache
}

// Example 3: Direct Integration Pattern (replaces wrapper layers)
func ExampleDirectIntegration() {
	// Create cache
	cache := ExampleSimpleCache()

	// Create direct integration
	integration := NewDirectCacheIntegration(cache)

	// Use with LSP fallback
	result, err := integration.ProcessRequest(
		context.Background(),
		"textDocument/definition",
		map[string]interface{}{"uri": "file://test.go", "position": map[string]int{"line": 10, "character": 5}},
		func() (interface{}, error) {
			// LSP fallback function
			return "definition result", nil
		},
	)

	_ = result
	_ = err
}
