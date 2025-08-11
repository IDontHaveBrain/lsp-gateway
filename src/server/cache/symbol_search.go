package cache

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
)

// Symbol search operations - handles symbol queries, enhanced searching, and direct symbol access

// QueryIndex queries the SCIP storage for symbols and relationships
func (m *SCIPCacheManager) QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error) {
	return m.WithIndexResult(query.Type, func() (*IndexResult, error) {
		result := m.WithIndexReadLock(func() interface{} {
			return m.queryIndexInternal(ctx, query)
		})
		if result == nil {
			return nil, fmt.Errorf("failed to query index")
		}
		return result.(*IndexResult), nil
	})
}

// queryIndexInternal contains the actual query logic without guards or locks
func (m *SCIPCacheManager) queryIndexInternal(ctx context.Context, query *IndexQuery) *IndexResult {

	var results []interface{}
	var err error

	switch query.Type {
	case "symbol":
		sciPSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
		if err == nil {
			results = make([]interface{}, len(sciPSymbols))
			for i, sym := range sciPSymbols {
				results[i] = sym
			}
		}
	case "definition":
		if defOccs, err := m.scipStorage.GetDefinitions(ctx, query.Symbol); err == nil && len(defOccs) > 0 {
			results = []interface{}{defOccs[0]}
		}
	case "references":
		if refOccs, err := m.scipStorage.GetReferences(ctx, query.Symbol); err == nil {
			for _, occ := range refOccs {
				results = append(results, occ)
			}
		}
	case "workspace":
		sciPSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
		if err == nil {
			results = make([]interface{}, len(sciPSymbols))
			for i, sym := range sciPSymbols {
				results[i] = sym
			}
		}
	default:
		return &IndexResult{
			Type:      query.Type,
			Results:   []interface{}{},
			Metadata:  map[string]interface{}{"error": fmt.Sprintf("unsupported query type: %s", query.Type)},
			Timestamp: time.Now(),
		}
	}

	if err != nil {
		return &IndexResult{
			Type:      query.Type,
			Results:   []interface{}{},
			Metadata:  map[string]interface{}{"error": err.Error()},
			Timestamp: time.Now(),
		}
	}

	metadata := map[string]interface{}{
		"query_language": query.Language,
		"query_type":     query.Type,
	}

	stats := m.scipStorage.GetIndexStats()
	metadata["total_symbols"] = stats.TotalSymbols
	metadata["total_documents"] = stats.TotalDocuments
	metadata["total_occurrences"] = stats.TotalOccurrences

	return &IndexResult{
		Type:      query.Type,
		Results:   results,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}
}

// SearchSymbolsEnhanced performs direct SCIP symbol search with enhanced results
func (m *SCIPCacheManager) SearchSymbolsEnhanced(ctx context.Context, query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResult, error) {
	return m.WithEnhancedSymbolResult(query, func() (*EnhancedSymbolSearchResult, error) {
		result := m.WithIndexReadLock(func() interface{} {
			return m.searchSymbolsEnhancedInternal(ctx, query)
		})
		return result.(*EnhancedSymbolSearchResult), nil
	})
}

// searchSymbolsEnhancedInternal contains the actual search logic without guards or locks
func (m *SCIPCacheManager) searchSymbolsEnhancedInternal(ctx context.Context, query *EnhancedSymbolQuery) *EnhancedSymbolSearchResult {

	maxResults := 100
	if query.MaxResults > 0 {
		maxResults = query.MaxResults
	}

	searchSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Pattern, maxResults*2)
	if err != nil {
		return &EnhancedSymbolSearchResult{
			Symbols:   []EnhancedSymbolResult{},
			Total:     0,
			Truncated: false,
			Query:     query,
			Metadata:  map[string]interface{}{"error": fmt.Sprintf("failed to search symbols in SCIP storage: %v", err)},
			Timestamp: time.Now(),
		}
	}

	var enhancedResults []EnhancedSymbolResult
	for i, symbolInfo := range searchSymbols {
		if i >= maxResults {
			break
		}

		// Apply filters
		if len(query.SymbolKinds) > 0 {
			kindMatched := false
			for _, kind := range query.SymbolKinds {
				if symbolInfo.Kind == kind {
					kindMatched = true
					break
				}
			}
			if !kindMatched {
				continue
			}
		}

		occurrences, _ := m.scipStorage.GetOccurrences(ctx, symbolInfo.Symbol)

		if query.MinOccurrences > 0 && len(occurrences) < query.MinOccurrences {
			continue
		}
		if query.MaxOccurrences > 0 && len(occurrences) > query.MaxOccurrences {
			continue
		}

		enhancedResults = append(enhancedResults, m.buildEnhancedSymbolResult(&symbolInfo, occurrences, query))
	}

	if query.SortBy != "" {
		m.sortEnhancedResults(enhancedResults, query.SortBy)
	}

	return &EnhancedSymbolSearchResult{
		Symbols:   enhancedResults,
		Total:     len(enhancedResults),
		Truncated: len(searchSymbols) > maxResults,
		Query:     query,
		Metadata: map[string]interface{}{
			"scip_enabled":     true,
			"total_candidates": len(searchSymbols),
		},
		Timestamp: time.Now(),
	}
}

// SearchSymbols provides direct access to SCIP symbol search for MCP tools
func (m *SCIPCacheManager) SearchSymbols(ctx context.Context, pattern, filePattern string, maxResults int) ([]interface{}, error) {
	common.LSPLogger.Debug("[SearchSymbols] Called with pattern='%s', filePattern='%s', maxResults=%d", pattern, filePattern, maxResults)

	if !m.enabled {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = 100
	}

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
									"filePath":    m.convertURIToFilePath(uri),
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
			"filePath":    m.convertURIToFilePath(occWithDoc.DocumentURI),
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
