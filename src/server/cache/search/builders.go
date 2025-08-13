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
