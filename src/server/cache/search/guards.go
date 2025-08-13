package search

import (
    "fmt"
    "time"

    "lsp-gateway/src/server/scip"
    "lsp-gateway/src/internal/common"
)

// SearchGuard provides centralized guard logic for SearchService operations.
// It handles enabled/disabled state checks and generates appropriate default responses.
type SearchGuard struct {
	enabled bool
}

// NewSearchGuard creates a new search guard with the specified enabled state.
func NewSearchGuard(enabled bool) *SearchGuard {
	return &SearchGuard{enabled: enabled}
}

// WithEnabledGuard executes the provided function only if the service is enabled.
// Returns the function result if enabled, otherwise returns nil and no error.
func (g *SearchGuard) WithEnabledGuard(fn func() (interface{}, error)) (interface{}, error) {
	if !g.enabled {
		return nil, nil
	}
	return fn()
}

// WithEnabledGuardTyped is a generic version that provides type safety.
// Returns the zero value of type T if service is disabled.
func WithEnabledGuardTyped[T any](g *SearchGuard, fn func() (T, error)) (T, error) {
    return common.WithEnabledGuard[T](g.enabled, fn)
}

// MustBeEnabled returns an error if the service is disabled.
// Use this for operations that require the service to be enabled.
func (g *SearchGuard) MustBeEnabled() error {
	if !g.enabled {
		return fmt.Errorf("cache disabled or SCIP storage unavailable")
	}
	return nil
}

// WithSliceResult executes the function if enabled, otherwise returns an empty slice.
// Useful for methods that return []interface{} when disabled.
func (g *SearchGuard) WithSliceResult(fn func() ([]interface{}, error)) ([]interface{}, error) {
	if !g.enabled {
		return []interface{}{}, nil
	}
	return fn()
}

// WithErrorResult executes the function if enabled, otherwise returns an error.
// Use for operations that must fail when service is disabled.
func (g *SearchGuard) WithErrorResult(fn func() ([]interface{}, error)) ([]interface{}, error) {
	if err := g.MustBeEnabled(); err != nil {
		return nil, err
	}
	return fn()
}

// WithSearchResponse executes the function if enabled, otherwise returns a default SearchResponse
// with cache_disabled metadata.
func (g *SearchGuard) WithSearchResponse(searchType SearchType, fn func() (*SearchResponse, error)) (*SearchResponse, error) {
	if !g.enabled {
		return &SearchResponse{
			Type:      searchType,
			Results:   []interface{}{},
			Total:     0,
			Truncated: false,
			Metadata:  &SearchMetadata{CacheEnabled: false},
			Timestamp: time.Now(),
			Success:   true,
		}, nil
	}
	return fn()
}

// WithEnhancedSymbolResult executes the function if enabled, otherwise returns a default
// EnhancedSymbolSearchResponse with cache_disabled metadata.
func (g *SearchGuard) WithEnhancedSymbolResult(query *EnhancedSymbolQuery, fn func() (*EnhancedSymbolSearchResponse, error)) (*EnhancedSymbolSearchResponse, error) {
	if !g.enabled {
		return &EnhancedSymbolSearchResponse{
			Symbols:   []EnhancedSymbolResult{},
			Total:     0,
			Truncated: false,
			Query:     query,
			Metadata:  &SearchMetadata{CacheEnabled: false},
			Timestamp: time.Now(),
		}, nil
	}
	return fn()
}

// WithReferenceResult executes the function if enabled, otherwise returns a default
// ReferenceSearchResponse with cache_disabled metadata.
func (g *SearchGuard) WithReferenceResult(symbolName string, options *ReferenceSearchOptions, fn func() (*ReferenceSearchResponse, error)) (*ReferenceSearchResponse, error) {
	if !g.enabled {
		return &ReferenceSearchResponse{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   &SearchMetadata{CacheEnabled: false},
			Timestamp:  time.Now(),
		}, nil
	}
	return fn()
}

// WithSymbolInfoResult executes the function if enabled, otherwise returns a default
// SymbolInfoResponse with cache_disabled metadata.
func (g *SearchGuard) WithSymbolInfoResult(symbolName string, fn func() (*SymbolInfoResponse, error)) (*SymbolInfoResponse, error) {
	if !g.enabled {
		return &SymbolInfoResponse{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        &SearchMetadata{CacheEnabled: false},
			Timestamp:       time.Now(),
		}, nil
	}
	return fn()
}

// CreateDisabledSearchResponse creates a disabled SearchResponse for the given search type.
func CreateDisabledSearchResponse(searchType SearchType) *SearchResponse {
	return &SearchResponse{
		Type:      searchType,
		Results:   []interface{}{},
		Total:     0,
		Truncated: false,
		Metadata:  &SearchMetadata{CacheEnabled: false},
		Timestamp: time.Now(),
		Success:   true,
	}
}

// CreateDisabledSymbolInfoResult creates a disabled SymbolInfoResponse for the given symbol name.
func CreateDisabledSymbolInfoResult(symbolName string) *SymbolInfoResponse {
	return &SymbolInfoResponse{
		SymbolName:      symbolName,
		Kind:            scip.SCIPSymbolKindUnknown,
		Documentation:   []string{},
		Occurrences:     []SCIPOccurrenceInfo{},
		OccurrenceCount: 0,
		DefinitionCount: 0,
		ReferenceCount:  0,
		FileCount:       0,
		Metadata:        &SearchMetadata{CacheEnabled: false},
		Timestamp:       time.Now(),
	}
}

// CreateDisabledReferenceResult creates a disabled ReferenceSearchResponse for the given parameters.
func CreateDisabledReferenceResult(symbolName string, options *ReferenceSearchOptions) *ReferenceSearchResponse {
	return &ReferenceSearchResponse{
		SymbolName: symbolName,
		References: []SCIPOccurrenceInfo{},
		TotalCount: 0,
		FileCount:  0,
		Options:    options,
		Metadata:   &SearchMetadata{CacheEnabled: false},
		Timestamp:  time.Now(),
	}
}

// CreateDisabledEnhancedSymbolResult creates a disabled EnhancedSymbolSearchResponse for the given query.
func CreateDisabledEnhancedSymbolResult(query *EnhancedSymbolQuery) *EnhancedSymbolSearchResponse {
	return &EnhancedSymbolSearchResponse{
		Symbols:   []EnhancedSymbolResult{},
		Total:     0,
		Truncated: false,
		Query:     query,
		Metadata:  &SearchMetadata{CacheEnabled: false},
		Timestamp: time.Now(),
	}
}

// CreateEmptySliceResult creates an empty slice result for disabled operations.
func CreateEmptySliceResult() []interface{} {
	return []interface{}{}
}

// CreateDisabledError creates a standard error for disabled operations.
func CreateDisabledError() error {
	return fmt.Errorf("cache disabled or SCIP storage unavailable")
}
