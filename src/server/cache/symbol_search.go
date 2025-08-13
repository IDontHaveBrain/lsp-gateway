package cache

import (
    "context"
    "fmt"

    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/constants"
    "lsp-gateway/src/server/cache/search"
    "lsp-gateway/src/server/scip"
    "lsp-gateway/src/utils"
)

// Symbol search operations - handles symbol queries, enhanced searching, and direct symbol access

// convertEnhancedSymbolResults converts search.EnhancedSymbolResult to cache.EnhancedSymbolResult
func convertEnhancedSymbolResults(searchResults []search.EnhancedSymbolResult) []EnhancedSymbolResult {
    // Types unified; return as-is
    return searchResults
}

// QueryIndex queries the SCIP storage for symbols and relationships
func (m *SCIPCacheManager) QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error) {
	return m.WithIndexResult(query.Type, func() (*IndexResult, error) {
		request := &search.SearchRequest{
			Context:    ctx,
			Type:       search.SearchType(query.Type),
			SymbolName: query.Symbol,
			MaxResults: constants.DefaultMaxResults,
		}

		response, err := m.searchService.ExecuteSearch(request)
		if err != nil {
			return nil, err
		}

		// Convert SearchResponse to IndexResult
		metadata := map[string]interface{}{
			"query_language": query.Language,
			"query_type":     query.Type,
		}
		if response.Metadata != nil {
			if response.Metadata.IndexStats != nil {
				metadata["total_symbols"] = response.Metadata.IndexStats.TotalSymbols
				metadata["total_documents"] = response.Metadata.IndexStats.TotalDocuments
				metadata["total_occurrences"] = response.Metadata.IndexStats.TotalOccurrences
			}
		}

		return &IndexResult{
			Type:      query.Type,
			Results:   response.Results,
			Metadata:  metadata,
			Timestamp: response.Timestamp,
		}, nil
	})
}

// SearchSymbolsEnhanced performs direct SCIP symbol search with enhanced results
func (m *SCIPCacheManager) SearchSymbolsEnhanced(ctx context.Context, query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResult, error) {
	return m.WithEnhancedSymbolResult(query, func() (*EnhancedSymbolSearchResult, error) {
		// Convert cache EnhancedSymbolQuery to search EnhancedSymbolQuery
		searchQuery := &search.EnhancedSymbolQuery{
			Pattern:              query.Pattern,
			FilePattern:          query.FilePattern,
			Language:             query.Language,
			MaxResults:           query.MaxResults,
			QueryType:            search.OccurrenceQueryType(query.QueryType),
			SymbolRoles:          query.SymbolRoles,
			SymbolKinds:          query.SymbolKinds,
			ExcludeRoles:         query.ExcludeRoles,
			MinOccurrences:       query.MinOccurrences,
			MaxOccurrences:       query.MaxOccurrences,
			IncludeDocumentation: query.IncludeDocumentation,
			IncludeRelationships: query.IncludeRelationships,
			OnlyWithDefinition:   query.OnlyWithDefinition,
			SortBy:               query.SortBy,
			IncludeScore:         query.IncludeScore,
		}

		response, err := m.searchService.ExecuteEnhancedSymbolSearch(searchQuery)
		if err != nil {
			return nil, err
		}

		// Convert search response to cache response
		metadata := map[string]interface{}{
			"scip_enabled": true,
		}
		if response.Metadata != nil {
			metadata["total_candidates"] = response.Metadata.TotalCandidates
		}

        return &EnhancedSymbolSearchResult{
            Symbols:   response.Symbols,
            Total:     response.Total,
            Truncated: response.Truncated,
            Query:     query,
            Metadata:  metadata,
            Timestamp: response.Timestamp,
        }, nil
    })
}

// SearchSymbols provides direct access to SCIP symbol search for MCP tools
func (m *SCIPCacheManager) SearchSymbols(ctx context.Context, pattern, filePattern string, maxResults int) ([]interface{}, error) {
	common.LSPLogger.Debug("[SearchSymbols] Called with pattern='%s', filePattern='%s', maxResults=%d", pattern, filePattern, maxResults)

	if !m.enabled {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	// Try SearchService first
	request := &search.SearchRequest{
		Context:     ctx,
		Type:        search.SearchTypeSymbol,
		SymbolName:  pattern,
		FilePattern: filePattern,
		MaxResults:  maxResults,
	}

	response, err := m.searchService.ExecuteSymbolSearch(request)
	if err == nil && len(response.Results) > 0 {
		// Convert results to include filePath field for backward compatibility
		convertedResults := make([]interface{}, 0, len(response.Results))
		for _, result := range response.Results {
			if resultMap, ok := result.(map[string]interface{}); ok {
				// Add filePath field using utils.URIToFilePath if not present
                if _, hasFilePath := resultMap["filePath"]; !hasFilePath {
                    if docURI, hasURI := resultMap["documentURI"]; hasURI {
                        if uriStr, ok := docURI.(string); ok {
                            resultMap["filePath"] = utils.URIToFilePathCached(uriStr)
                        }
                    }
                }
				convertedResults = append(convertedResults, resultMap)
			} else {
				convertedResults = append(convertedResults, result)
			}
		}
		common.LSPLogger.Debug("[SearchSymbols] SearchService returned %d results", len(convertedResults))
		return convertedResults, nil
	}

	// Fallback to complex document scanning logic when SearchService fails or returns no results
	common.LSPLogger.Debug("[SearchSymbols] SearchService failed or returned no results, falling back to document scanning")

	// When file filter is provided, fetch more symbols to account for filtering
	searchLimit := maxResults
	if filePattern != "" {
		// Fetch 10x more symbols when filtering to ensure enough results after filtering
		searchLimit = maxResults * 10
		if searchLimit > 1000 {
			searchLimit = 1000
		}
	}

	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("SCIP symbol search failed: %w", err)
	}
	common.LSPLogger.Debug("[SearchSymbols] SCIP storage returned %d symbol infos", len(symbolInfos))

	results := make([]interface{}, 0, len(symbolInfos))
	for _, symbolInfo := range symbolInfos {
		common.LSPLogger.Debug("[SearchSymbols] Processing symbol: %s (DisplayName: %s)",
			symbolInfo.Symbol, symbolInfo.DisplayName)

		// Prefer definition; else any occurrence
		var occWithDoc *scip.OccurrenceWithDocument
		defs, defErr := m.scipStorage.GetDefinitionsWithDocuments(ctx, symbolInfo.Symbol)
		common.LSPLogger.Debug("[SearchSymbols] GetDefinitionsWithDocuments for %s: %d results, err=%v",
			symbolInfo.Symbol, len(defs), defErr)
		if len(defs) > 0 {
			occWithDoc = &defs[0]
		} else {
			occs, occErr := m.scipStorage.GetOccurrencesWithDocuments(ctx, symbolInfo.Symbol)
			common.LSPLogger.Debug("[SearchSymbols] GetOccurrencesWithDocuments for %s: %d results, err=%v",
				symbolInfo.Symbol, len(occs), occErr)
			if len(occs) > 0 {
				occWithDoc = &occs[0]
			}
		}

		if occWithDoc == nil {
			common.LSPLogger.Debug("[SearchSymbols] No occurrences found, scanning documents")
			// Fallback: scan documents' symbol information to resolve a document URI
			if uris, err := m.scipStorage.ListDocuments(ctx); err == nil {
				common.LSPLogger.Debug("[SearchSymbols] Scanning %d documents", len(uris))
				found := false
				for _, uri := range uris {
					if doc, de := m.scipStorage.GetDocument(ctx, uri); de == nil && doc != nil {
						for _, si := range doc.SymbolInformation {
							if si.Symbol == symbolInfo.Symbol {
								common.LSPLogger.Debug("[SearchSymbols] Found matching symbol in doc %s: %s == %s",
									uri, si.Symbol, symbolInfo.Symbol)
								// Apply file filter on document URI if provided
								if filePattern != "" && !m.matchFilePattern(uri, filePattern) {
									common.LSPLogger.Debug("[SearchSymbols] Symbol filtered out by filePattern: URI=%s, pattern=%s", uri, filePattern)
									found = true // resolved but filtered out; skip symbol
									break
								}
								enhanced := map[string]interface{}{
									"symbolInfo":  symbolInfo,
                "filePath":    utils.URIToFilePathCached(uri),
									"documentURI": uri,
									"range":       si.Range,
								}
								results = append(results, enhanced)
								found = true
								// Check if we have reached maxResults after filtering
								if len(results) >= maxResults {
									common.LSPLogger.Debug("[SearchSymbols] Reached maxResults limit (%d) in fallback, stopping", maxResults)
									return results, nil
								}
								break
							}
						}
					}
					if found {
						break
					}
				}
				if found {
					continue
				}
			}
			// If we cannot resolve a document URI, skip this symbol to avoid unknown locations
			continue
		}
		// Apply file filter
		if filePattern != "" {
			matches := m.matchFilePattern(occWithDoc.DocumentURI, filePattern)
			common.LSPLogger.Debug("[SearchSymbols] File filter: URI=%s, pattern=%s, matches=%v",
				occWithDoc.DocumentURI, filePattern, matches)
			if !matches {
				continue
			}
		}
		enhancedResult := map[string]interface{}{
			"symbolInfo":  symbolInfo,
			"occurrence":  &occWithDoc.SCIPOccurrence,
            "filePath":    utils.URIToFilePathCached(occWithDoc.DocumentURI),
			"documentURI": occWithDoc.DocumentURI,
			"range":       occWithDoc.Range,
		}
		results = append(results, enhancedResult)

		// Check if we have reached maxResults after filtering
		if len(results) >= maxResults {
			common.LSPLogger.Debug("[SearchSymbols] Reached maxResults limit (%d), stopping", maxResults)
			break
		}
	}

	return results, nil
}
