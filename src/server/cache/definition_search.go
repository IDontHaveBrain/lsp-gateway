package cache

import (
	"context"
	"time"

	"lsp-gateway/src/server/cache/search"
	"lsp-gateway/src/server/scip"
)

// Definition search operations - handles definition searching and symbol info retrieval

// SearchDefinitions performs direct SCIP definition search with enhanced results
func (m *SCIPCacheManager) SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	request := &search.SearchRequest{
		Context:     ctx,
		Type:        search.SearchTypeDefinition,
		SymbolName:  symbolName,
		FilePattern: filePattern,
		MaxResults:  maxResults,
	}
	response, err := m.searchService.ExecuteDefinitionSearch(request)
	if err != nil {
		return []interface{}{}, err
	}
	return response.Results, nil
}

// GetSymbolInfo performs direct SCIP symbol information retrieval with occurrence details
func (m *SCIPCacheManager) GetSymbolInfo(ctx context.Context, symbolName, filePattern string) (interface{}, error) {
	response, err := m.searchService.ExecuteSymbolInfoSearch(symbolName, filePattern)
	if err != nil {
		return &SymbolInfoResult{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        map[string]interface{}{"error": err.Error()},
			Timestamp:       time.Now(),
		}, err
	}

	// Convert SymbolInfoResponse to SymbolInfoResult for backward compatibility
	metadata := map[string]interface{}{
		"scip_enabled":     response.Metadata.SCIPEnabled,
		"cache_enabled":    response.Metadata.CacheEnabled,
		"symbols_found":    response.Metadata.SymbolsFound,
		"filtered_results": response.Metadata.FilteredResults,
		"file_pattern":     response.Metadata.FilePattern,
	}

	// Convert search.SCIPOccurrenceInfo to SCIPOccurrenceInfo for backward compatibility
	var occurrences []SCIPOccurrenceInfo
	occurrences = append(occurrences, response.Occurrences...)

	return &SymbolInfoResult{
		SymbolName:      response.SymbolName,
		SymbolID:        response.SymbolID,
		SymbolInfo:      response.SymbolInfo,
		Kind:            response.Kind,
		Documentation:   response.Documentation,
		Signature:       response.Signature,
		Relationships:   response.Relationships,
		Occurrences:     occurrences,
		OccurrenceCount: response.OccurrenceCount,
		DefinitionCount: response.DefinitionCount,
		ReferenceCount:  response.ReferenceCount,
		FileCount:       response.FileCount,
		Metadata:        metadata,
		Timestamp:       response.Timestamp,
	}, nil
}
