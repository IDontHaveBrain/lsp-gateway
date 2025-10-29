package search

import (
	"fmt"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
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
	return common.WithEnabledGuard(g.enabled, fn)
}

// NOTE: Base guard behavior is delegated to internal/common.WithEnabledGuard for consistency.

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
	return common.WithEnabledGuardDefault(g.enabled, fn, []interface{}{})
}

// WithErrorResult executes the function if enabled, otherwise returns an error.
// Use for operations that must fail when service is disabled.
func (g *SearchGuard) WithErrorResult(fn func() ([]interface{}, error)) ([]interface{}, error) {
	return common.WithEnabledGuardOrError(g.enabled, fn, "cache disabled or SCIP storage unavailable")
}

// WithSearchResponse executes the function if enabled, otherwise returns a default SearchResponse
// with cache_disabled metadata.
func (g *SearchGuard) WithSearchResponse(searchType SearchType, fn func() (*SearchResponse, error)) (*SearchResponse, error) {
	builder := &DefaultResultBuilder{}
	defaultResponse := builder.BuildDisabledResponse(searchType)
	return common.WithEnabledGuardDefault(g.enabled, fn, defaultResponse)
}

// WithEnhancedSymbolResult executes the function if enabled, otherwise returns a default
// EnhancedSymbolSearchResponse with cache_disabled metadata.
func (g *SearchGuard) WithEnhancedSymbolResult(query *EnhancedSymbolQuery, fn func() (*EnhancedSymbolSearchResponse, error)) (*EnhancedSymbolSearchResponse, error) {
	defaultResponse := &EnhancedSymbolSearchResponse{
		Symbols:   []EnhancedSymbolResult{},
		Total:     0,
		Truncated: false,
		Query:     query,
		Metadata:  &SearchMetadata{CacheEnabled: false},
		Timestamp: time.Now(),
	}
	return common.WithEnabledGuardDefault(g.enabled, fn, defaultResponse)
}

// WithReferenceResult executes the function if enabled, otherwise returns a default
// ReferenceSearchResponse with cache_disabled metadata.
func (g *SearchGuard) WithReferenceResult(symbolName string, options *ReferenceSearchOptions, fn func() (*ReferenceSearchResponse, error)) (*ReferenceSearchResponse, error) {
	defaultResponse := &ReferenceSearchResponse{
		SymbolName: symbolName,
		References: []SCIPOccurrenceInfo{},
		TotalCount: 0,
		FileCount:  0,
		Options:    options,
		Metadata:   &SearchMetadata{CacheEnabled: false},
		Timestamp:  time.Now(),
	}
	return common.WithEnabledGuardDefault(g.enabled, fn, defaultResponse)
}

// WithSymbolInfoResult executes the function if enabled, otherwise returns a default
// SymbolInfoResponse with cache_disabled metadata.
func (g *SearchGuard) WithSymbolInfoResult(symbolName string, fn func() (*SymbolInfoResponse, error)) (*SymbolInfoResponse, error) {
	defaultResponse := &SymbolInfoResponse{
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
	return common.WithEnabledGuardDefault(g.enabled, fn, defaultResponse)
}
