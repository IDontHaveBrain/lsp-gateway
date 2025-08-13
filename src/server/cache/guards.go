package cache

import (
    "fmt"
    "time"
    "lsp-gateway/src/internal/common"
)

// WithEnabledGuard executes the provided function only if the cache is enabled.
// Returns the function result if cache is enabled, otherwise returns nil and no error.
func (m *SCIPCacheManager) WithEnabledGuard(fn func() (interface{}, error)) (interface{}, error) {
	if !m.enabled {
		return nil, nil
	}
	return fn()
}

// WithEnabledGuardTyped is a generic version of WithEnabledGuard that provides type safety.
// Returns the zero value of type T if cache is disabled.
func WithEnabledGuardTyped[T any](m *SCIPCacheManager, fn func() (T, error)) (T, error) {
    return common.WithEnabledGuard[T](m.enabled, fn)
}

// MustBeEnabled returns an error if the cache is disabled.
// Use this for operations that require the cache to be enabled.
func (m *SCIPCacheManager) MustBeEnabled() error {
	if !m.enabled {
		return fmt.Errorf("cache disabled or SCIP storage unavailable")
	}
	return nil
}

// WithIndexResult executes the function if cache is enabled, otherwise returns a default IndexResult
// with cache_disabled metadata.
func (m *SCIPCacheManager) WithIndexResult(queryType string, fn func() (*IndexResult, error)) (*IndexResult, error) {
	if !m.enabled {
		return &IndexResult{
			Type:      queryType,
			Results:   []interface{}{},
			Metadata:  map[string]interface{}{"cache_disabled": true},
			Timestamp: time.Now(),
		}, nil
	}
	return fn()
}

// WithEnhancedSymbolResult executes the function if cache is enabled, otherwise returns a default
// EnhancedSymbolSearchResult with cache_disabled metadata.
func (m *SCIPCacheManager) WithEnhancedSymbolResult(query *EnhancedSymbolQuery, fn func() (*EnhancedSymbolSearchResult, error)) (*EnhancedSymbolSearchResult, error) {
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
	return fn()
}

// WithReferenceResult executes the function if cache is enabled, otherwise returns a default
// ReferenceSearchResult with cache_disabled metadata.
func (m *SCIPCacheManager) WithReferenceResult(symbolName string, options *ReferenceSearchOptions, fn func() (*ReferenceSearchResult, error)) (*ReferenceSearchResult, error) {
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
	return fn()
}

// WithSymbolInfoResult executes the function if cache is enabled, otherwise returns a default
// SymbolInfoResult with cache_disabled metadata.
func (m *SCIPCacheManager) WithSymbolInfoResult(symbolName string, fn func() (*SymbolInfoResult, error)) (*SymbolInfoResult, error) {
	if !m.enabled {
		return &SymbolInfoResult{
			SymbolName:      symbolName,
			Kind:            0, // SCIPSymbolKindUnknown
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
	return fn()
}

// WithSliceResult executes the function if cache is enabled, otherwise returns an empty slice.
// Useful for methods that return []interface{} when cache is disabled.
func (m *SCIPCacheManager) WithSliceResult(fn func() ([]interface{}, error)) ([]interface{}, error) {
	if !m.enabled {
		return []interface{}{}, nil
	}
	return fn()
}

// isEnabledAndStarted checks if cache is enabled and properly initialized
func (m *SCIPCacheManager) isEnabledAndStarted() bool {
	return m.enabled && m.started
}

// WithManagerGuard executes the function only if the cache manager is enabled and started.
// Returns nil values if the cache is not ready.
func (m *SCIPCacheManager) WithManagerGuard(fn func() (interface{}, bool, error)) (interface{}, bool, error) {
	if !m.isEnabledAndStarted() {
		return nil, false, nil
	}
	return fn()
}
