package search

import (
	"fmt"
	"time"
)

// DefaultResultBuilder provides default implementation for building search results
type DefaultResultBuilder struct{}

// BuildSearchResponse creates a SearchResponse from raw data
func (b *DefaultResultBuilder) BuildSearchResponse(searchType SearchType, data interface{}, metadata *SearchMetadata) *SearchResponse {
	results, ok := data.([]interface{})
	if !ok {
		results = []interface{}{}
	}

	if metadata == nil {
		metadata = &SearchMetadata{
			CacheEnabled: true,
			SCIPEnabled:  true,
		}
	}

	return &SearchResponse{
		Type:      searchType,
		Results:   results,
		Total:     len(results),
		Truncated: false,
		Metadata:  metadata,
		Timestamp: time.Now(),
		Success:   true,
	}
}

// BuildErrorResponse creates an error SearchResponse
func (b *DefaultResultBuilder) BuildErrorResponse(searchType SearchType, err error) *SearchResponse {
	return &SearchResponse{
		Type:      searchType,
		Results:   []interface{}{},
		Total:     0,
		Truncated: false,
		Metadata: &SearchMetadata{
			CacheEnabled: true,
			SCIPEnabled:  true,
			Errors:       []string{err.Error()},
		},
		Timestamp: time.Now(),
		Success:   false,
		Error:     err.Error(),
	}
}

// BuildDisabledResponse creates a response for when cache is disabled
func (b *DefaultResultBuilder) BuildDisabledResponse(searchType SearchType) *SearchResponse {
	return &SearchResponse{
		Type:      searchType,
		Results:   []interface{}{},
		Total:     0,
		Truncated: false,
		Metadata: &SearchMetadata{
			CacheEnabled: false,
			SCIPEnabled:  false,
		},
		Timestamp: time.Now(),
		Success:   true,
	}
}

// SearchRequestBuilder provides a fluent interface for building search requests
type SearchRequestBuilder struct {
	request *SearchRequest
}

// NewSearchRequestBuilder creates a new search request builder
func NewSearchRequestBuilder() *SearchRequestBuilder {
	return &SearchRequestBuilder{
		request: &SearchRequest{},
	}
}

// WithType sets the search type
func (b *SearchRequestBuilder) WithType(searchType SearchType) *SearchRequestBuilder {
	b.request.Type = searchType
	return b
}

// WithSymbolName sets the symbol name
func (b *SearchRequestBuilder) WithSymbolName(symbolName string) *SearchRequestBuilder {
	b.request.SymbolName = symbolName
	return b
}

// WithFilePattern sets the file pattern
func (b *SearchRequestBuilder) WithFilePattern(filePattern string) *SearchRequestBuilder {
	b.request.FilePattern = filePattern
	return b
}

// WithMaxResults sets the maximum number of results
func (b *SearchRequestBuilder) WithMaxResults(maxResults int) *SearchRequestBuilder {
	b.request.MaxResults = maxResults
	return b
}

// WithContext sets the context
func (b *SearchRequestBuilder) WithContext(ctx interface{}) *SearchRequestBuilder {
	if context, ok := ctx.(interface{ Done() <-chan struct{} }); ok {
		b.request.Context = context.(interface {
			Deadline() (deadline time.Time, ok bool)
			Done() <-chan struct{}
			Err() error
			Value(key interface{}) interface{}
		})
	}
	return b
}

// WithOptions sets the type-specific options
func (b *SearchRequestBuilder) WithOptions(options interface{}) *SearchRequestBuilder {
	b.request.Options = options
	return b
}

// Build creates the search request
func (b *SearchRequestBuilder) Build() (*SearchRequest, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	return &SearchRequest{
		Type:        b.request.Type,
		SymbolName:  b.request.SymbolName,
		FilePattern: b.request.FilePattern,
		MaxResults:  b.request.MaxResults,
		Context:     b.request.Context,
		Options:     b.request.Options,
	}, nil
}

// validate checks that the search request is valid
func (b *SearchRequestBuilder) validate() error {
	if b.request.Type == "" {
		return fmt.Errorf("search type is required")
	}
	if b.request.SymbolName == "" {
		return fmt.Errorf("symbol name is required")
	}
	if b.request.Context == nil {
		return fmt.Errorf("context is required")
	}
	return nil
}

// Reset clears the builder
func (b *SearchRequestBuilder) Reset() *SearchRequestBuilder {
	b.request = &SearchRequest{}
	return b
}

// Clone creates a copy of the builder
func (b *SearchRequestBuilder) Clone() *SearchRequestBuilder {
	return &SearchRequestBuilder{
		request: &SearchRequest{
			Type:        b.request.Type,
			SymbolName:  b.request.SymbolName,
			FilePattern: b.request.FilePattern,
			MaxResults:  b.request.MaxResults,
			Context:     b.request.Context,
			Options:     b.request.Options,
		},
	}
}

// SearchResultProcessor provides utilities for processing search results
type SearchResultProcessor struct{}

// NewSearchResultProcessor creates a new search result processor
func NewSearchResultProcessor() *SearchResultProcessor {
	return &SearchResultProcessor{}
}

// MergeResults merges multiple search results into a single result
func (p *SearchResultProcessor) MergeResults(results ...*SearchResponse) *SearchResponse {
	if len(results) == 0 {
		return &SearchResponse{
			Results:   []interface{}{},
			Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true},
			Timestamp: time.Now(),
			Success:   true,
		}
	}

	// Use the first result as the base
	merged := &SearchResponse{
		Type:      results[0].Type,
		Results:   []interface{}{},
		Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true},
		Timestamp: time.Now(),
		Success:   true,
	}

	totalCount := 0
	truncated := false

	for _, result := range results {
		// Merge results
		merged.Results = append(merged.Results, result.Results...)
		totalCount += result.Total
		if result.Truncated {
			truncated = true
		}

		// Update metadata with aggregated information
		if result.Metadata != nil {
			if result.Metadata.SymbolsFound > merged.Metadata.SymbolsFound {
				merged.Metadata.SymbolsFound = result.Metadata.SymbolsFound
			}
			if result.Metadata.FilesMatched > merged.Metadata.FilesMatched {
				merged.Metadata.FilesMatched = result.Metadata.FilesMatched
			}
		}
	}

	merged.Total = totalCount
	merged.Truncated = truncated

	return merged
}

// FilterResults filters search results based on a predicate function
func (p *SearchResultProcessor) FilterResults(result *SearchResponse, predicate func(interface{}) bool) *SearchResponse {
	filtered := &SearchResponse{
		Type:      result.Type,
		Results:   []interface{}{},
		Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true},
		Timestamp: time.Now(),
		Success:   result.Success,
	}

	// Copy metadata values
	if result.Metadata != nil {
		*filtered.Metadata = *result.Metadata
	}

	// Filter results
	for _, item := range result.Results {
		if predicate(item) {
			filtered.Results = append(filtered.Results, item)
		}
	}

	filtered.Total = len(filtered.Results)
	filtered.Truncated = result.Truncated
	if filtered.Metadata.Debug == nil {
		filtered.Metadata.Debug = make(map[string]interface{})
	}
	filtered.Metadata.Debug["original_count"] = result.Total
	filtered.Metadata.Debug["filtered_count"] = filtered.Total

	return filtered
}

// LimitResults limits the number of results in a search result
func (p *SearchResultProcessor) LimitResults(result *SearchResponse, limit int) *SearchResponse {
	if limit <= 0 || len(result.Results) <= limit {
		return result
	}

	limited := &SearchResponse{
		Type:      result.Type,
		Results:   result.Results[:limit],
		Total:     limit,
		Truncated: true,
		Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true},
		Timestamp: time.Now(),
		Success:   result.Success,
	}

	// Copy metadata values
	if result.Metadata != nil {
		*limited.Metadata = *result.Metadata
	}

	if limited.Metadata.Debug == nil {
		limited.Metadata.Debug = make(map[string]interface{})
	}
	limited.Metadata.Debug["original_count"] = result.Total
	limited.Metadata.Debug["limited_to"] = limit

	return limited
}

// SortResults sorts search results based on a comparison function
func (p *SearchResultProcessor) SortResults(result *SearchResponse, less func(i, j int) bool) *SearchResponse {
	sorted := &SearchResponse{
		Type:      result.Type,
		Results:   make([]interface{}, len(result.Results)),
		Total:     result.Total,
		Truncated: result.Truncated,
		Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true},
		Timestamp: time.Now(),
		Success:   result.Success,
	}

	// Copy results and metadata
	copy(sorted.Results, result.Results)
	if result.Metadata != nil {
		*sorted.Metadata = *result.Metadata
	}

	// Sort using a simple bubble sort implementation
	n := len(sorted.Results)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if less(j+1, j) {
				sorted.Results[j], sorted.Results[j+1] = sorted.Results[j+1], sorted.Results[j]
			}
		}
	}

	if sorted.Metadata.Debug == nil {
		sorted.Metadata.Debug = make(map[string]interface{})
	}
	sorted.Metadata.Debug["sorted"] = true

	return sorted
}

// DeduplicateResults removes duplicate results based on a key function
func (p *SearchResultProcessor) DeduplicateResults(result *SearchResponse, keyFunc func(interface{}) string) *SearchResponse {
	seen := make(map[string]bool)
	deduplicated := &SearchResponse{
		Type:      result.Type,
		Results:   []interface{}{},
		Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true},
		Timestamp: time.Now(),
		Success:   result.Success,
	}

	// Copy metadata
	if result.Metadata != nil {
		*deduplicated.Metadata = *result.Metadata
	}

	// Deduplicate results
	for _, item := range result.Results {
		key := keyFunc(item)
		if !seen[key] {
			seen[key] = true
			deduplicated.Results = append(deduplicated.Results, item)
		}
	}

	deduplicated.Total = len(deduplicated.Results)
	deduplicated.Truncated = result.Truncated
	if deduplicated.Metadata.Debug == nil {
		deduplicated.Metadata.Debug = make(map[string]interface{})
	}
	deduplicated.Metadata.Debug["original_count"] = result.Total
	deduplicated.Metadata.Debug["deduplicated_count"] = deduplicated.Total
	deduplicated.Metadata.Debug["duplicates_removed"] = result.Total - deduplicated.Total

	return deduplicated
}
