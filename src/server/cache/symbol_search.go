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
func (m *SCIPCacheManager) SearchSymbolsEnhanced(ctx context.Context, query *EnhancedSymbolQuery) (*search.EnhancedSymbolSearchResponse, error) {
    return m.searchService.ExecuteEnhancedSymbolSearch(query)
}

func (m *SCIPCacheManager) trySearchServiceSymbols(ctx context.Context, pattern, filePattern string, maxResults int) ([]interface{}, bool) {
	request := &search.SearchRequest{
		Context:     ctx,
		Type:        search.SearchTypeSymbol,
		SymbolName:  pattern,
		FilePattern: filePattern,
		MaxResults:  maxResults,
	}
	response, err := m.searchService.ExecuteSymbolSearch(request)
	if err != nil || len(response.Results) == 0 {
		return nil, false
	}
	converted := make([]interface{}, 0, len(response.Results))
	for _, result := range response.Results {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if _, hasFilePath := resultMap["filePath"]; !hasFilePath {
				if docURI, hasURI := resultMap["documentURI"]; hasURI {
					if uriStr, ok := docURI.(string); ok {
						resultMap["filePath"] = utils.URIToFilePathCached(uriStr)
					}
				}
			}
			converted = append(converted, resultMap)
		} else {
			converted = append(converted, result)
		}
	}
	return converted, true
}

func (m *SCIPCacheManager) tryFallbackScan(ctx context.Context, symbolInfo scip.SCIPSymbolInformation, filePattern string, maxResults int, results []interface{}) ([]interface{}, bool, bool) {
	if uris, err := m.scipStorage.ListDocuments(ctx); err == nil {
		found := false
		for _, uri := range uris {
			if doc, de := m.scipStorage.GetDocument(ctx, uri); de == nil && doc != nil {
				for _, si := range doc.SymbolInformation {
					if si.Symbol == symbolInfo.Symbol {
						if filePattern != "" && !m.matchFilePattern(uri, filePattern) {
							found = true
							break
						}
						en := map[string]interface{}{
							"symbolInfo":  symbolInfo,
							"filePath":    utils.URIToFilePathCached(uri),
							"documentURI": uri,
							"range":       si.Range,
						}
						results = append(results, en)
						found = true
						if len(results) >= maxResults {
							return results, true, true
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
			return results, true, false
		}
	}
	return results, false, false
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

	if convertedResults, ok := m.trySearchServiceSymbols(ctx, pattern, filePattern, maxResults); ok {
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
			if updated, handled, limit := m.tryFallbackScan(ctx, symbolInfo, filePattern, maxResults, results); handled {
				results = updated
				if limit {
					common.LSPLogger.Debug("[SearchSymbols] Reached maxResults limit (%d) in fallback, stopping", maxResults)
					return results, nil
				}
				continue
			}
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
