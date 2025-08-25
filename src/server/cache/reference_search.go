package cache

import (
    "context"

    "lsp-gateway/src/server/cache/search"
)

// Reference search operations - handles reference searching with standard and enhanced results

// SearchReferences provides interface-compliant reference search
func (m *SCIPCacheManager) SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	request := &search.SearchRequest{
		Context:     ctx,
		Type:        search.SearchTypeReference,
		SymbolName:  symbolName,
		FilePattern: filePattern,
		MaxResults:  maxResults,
	}
	response, err := m.searchService.ExecuteReferenceSearch(request)
	if err != nil {
		return []interface{}{}, err
	}
	return response.Results, nil
}

// SearchReferencesEnhanced performs direct SCIP reference search with enhanced results
func (m *SCIPCacheManager) SearchReferencesEnhanced(ctx context.Context, symbolName, filePattern string, options *ReferenceSearchOptions) (*search.ReferenceSearchResponse, error) {
	// Convert cache.ReferenceSearchOptions to search.ReferenceSearchOptions
	var searchOptions *search.ReferenceSearchOptions
	if options != nil {
		searchOptions = &search.ReferenceSearchOptions{
			MaxResults:  options.MaxResults,
			SymbolRoles: options.SymbolRoles,
			SymbolKinds: options.SymbolKinds,
			IncludeCode: options.IncludeCode,
			SortBy:      options.SortBy,
		}
	}

    return m.searchService.ExecuteReferenceSearchEnhanced(symbolName, filePattern, searchOptions)
}
