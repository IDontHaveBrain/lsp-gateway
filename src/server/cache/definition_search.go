package cache

import (
    "context"

    "lsp-gateway/src/server/cache/search"
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
    return m.searchService.ExecuteSymbolInfoSearch(symbolName, filePattern)
}
