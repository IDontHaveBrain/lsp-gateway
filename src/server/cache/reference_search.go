package cache

import (
	"context"
	"time"

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
func (m *SCIPCacheManager) SearchReferencesEnhanced(ctx context.Context, symbolName, filePattern string, options *ReferenceSearchOptions) (*ReferenceSearchResult, error) {
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

	response, err := m.searchService.ExecuteReferenceSearchEnhanced(symbolName, filePattern, searchOptions)
	if err != nil {
		return &ReferenceSearchResult{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   map[string]interface{}{"error": err.Error()},
			Timestamp:  time.Now(),
		}, err
	}

	// Convert search.ReferenceSearchResponse to ReferenceSearchResult for backward compatibility
	metadata := map[string]interface{}{
		"scip_enabled":  response.Metadata.SCIPEnabled,
		"cache_enabled": response.Metadata.CacheEnabled,
		"symbols_found": response.Metadata.SymbolsFound,
		"file_pattern":  response.Metadata.FilePattern,
	}

	// Convert search.SCIPOccurrenceInfo to SCIPOccurrenceInfo for backward compatibility
	var references []SCIPOccurrenceInfo
	for _, searchRef := range response.References {
		references = append(references, SCIPOccurrenceInfo{
			Occurrence:  searchRef.Occurrence,
			DocumentURI: searchRef.DocumentURI,
			SymbolRoles: searchRef.SymbolRoles,
			SyntaxKind:  searchRef.SyntaxKind,
			Context:     searchRef.Context,
			LineNumber:  searchRef.LineNumber,
			Score:       searchRef.Score,
		})
	}

	var definition *SCIPOccurrenceInfo
	if response.Definition != nil {
		definition = &SCIPOccurrenceInfo{
			Occurrence:  response.Definition.Occurrence,
			DocumentURI: response.Definition.DocumentURI,
			SymbolRoles: response.Definition.SymbolRoles,
			SyntaxKind:  response.Definition.SyntaxKind,
			Context:     response.Definition.Context,
			LineNumber:  response.Definition.LineNumber,
			Score:       response.Definition.Score,
		}
	}

	// Convert search.ReferenceSearchOptions back to cache.ReferenceSearchOptions for backward compatibility
	var resultOptions *ReferenceSearchOptions
	if response.Options != nil {
		resultOptions = &ReferenceSearchOptions{
			MaxResults:  response.Options.MaxResults,
			SymbolRoles: response.Options.SymbolRoles,
			SymbolKinds: response.Options.SymbolKinds,
			IncludeCode: response.Options.IncludeCode,
			SortBy:      response.Options.SortBy,
		}
	}

	return &ReferenceSearchResult{
		SymbolName: response.SymbolName,
		SymbolID:   response.SymbolID,
		References: references,
		Definition: definition,
		TotalCount: response.TotalCount,
		FileCount:  response.FileCount,
		Options:    resultOptions,
		Metadata:   metadata,
		Timestamp:  response.Timestamp,
	}, nil
}
