package search

import (
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
		Type:      string(searchType),
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
		Type:      string(searchType),
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
		Type:      string(searchType),
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
