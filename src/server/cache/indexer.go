package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
)

// SimpleIndexStats tracks basic indexing performance
type SimpleIndexStats struct {
	DocumentsIndexed int
	LastIndexTime    time.Time
	TotalErrors      int
}

// SimpleIndexer manages direct synchronous SCIP indexing
type SimpleIndexer struct {
	queryManager *SCIPQueryManager
	lspFallback  LSPFallback
	stats        SimpleIndexStats
	mu           sync.RWMutex
}

// NewSimpleIndexer creates a new simple SCIP indexer
func NewSimpleIndexer(queryManager *SCIPQueryManager, lspFallback LSPFallback) *SimpleIndexer {
	return &SimpleIndexer{
		queryManager: queryManager,
		lspFallback:  lspFallback,
		stats:        SimpleIndexStats{},
	}
}

// IndexDocument indexes a single document synchronously
func (idx *SimpleIndexer) IndexDocument(ctx context.Context, uri, language string) error {
	start := time.Now()

	// Request document symbols from LSP server
	documentSymbols, err := idx.requestDocumentSymbols(ctx, uri)
	if err != nil {
		idx.mu.Lock()
		idx.stats.TotalErrors++
		idx.mu.Unlock()
		return fmt.Errorf("failed to get document symbols for %s: %w", uri, err)
	}

	// Request workspace symbols related to this document
	workspaceSymbols, err := idx.requestWorkspaceSymbols(ctx, filepath.Base(uri))
	if err != nil {
		common.LSPLogger.Warn("Failed to get workspace symbols for %s: %v", uri, err)
		workspaceSymbols = []*lsp.SymbolInformation{} // Continue with empty list
	}

	// Extract additional symbol information if available
	symbolInfos := idx.extractSymbolInformation(documentSymbols, workspaceSymbols, uri)

	// Update the query index
	err = idx.queryManager.UpdateIndex(uri, symbolInfos, documentSymbols)
	if err != nil {
		idx.mu.Lock()
		idx.stats.TotalErrors++
		idx.mu.Unlock()
		return fmt.Errorf("failed to update index for %s: %w", uri, err)
	}

	// Update statistics
	idx.mu.Lock()
	idx.stats.DocumentsIndexed++
	idx.stats.LastIndexTime = time.Now()
	idx.mu.Unlock()

	common.LSPLogger.Debug("Indexed document %s: %d symbols in %v",
		uri, len(symbolInfos), time.Since(start))

	return nil
}

// GetStats returns current indexing statistics
func (idx *SimpleIndexer) GetStats() SimpleIndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Return a copy to avoid race conditions
	return SimpleIndexStats{
		DocumentsIndexed: idx.stats.DocumentsIndexed,
		LastIndexTime:    idx.stats.LastIndexTime,
		TotalErrors:      idx.stats.TotalErrors,
	}
}

// requestDocumentSymbols requests document symbols from LSP server
func (idx *SimpleIndexer) requestDocumentSymbols(ctx context.Context, uri string) ([]*lsp.DocumentSymbol, error) {
	params := &lsp.DocumentSymbolParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}

	result, err := idx.lspFallback.ProcessRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	// Handle different response formats
	switch v := result.(type) {
	case []*lsp.DocumentSymbol:
		return v, nil
	case []interface{}:
		// Parse JSON response
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		var symbols []*lsp.DocumentSymbol
		err = json.Unmarshal(jsonBytes, &symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal document symbols: %w", err)
		}
		return symbols, nil
	default:
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
}

// requestWorkspaceSymbols requests workspace symbols from LSP server
func (idx *SimpleIndexer) requestWorkspaceSymbols(ctx context.Context, query string) ([]*lsp.SymbolInformation, error) {
	params := &lsp.WorkspaceSymbolParams{Query: query}

	result, err := idx.lspFallback.ProcessRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	// Handle different response formats
	switch v := result.(type) {
	case []*lsp.SymbolInformation:
		return v, nil
	case []interface{}:
		// Parse JSON response
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		var symbols []*lsp.SymbolInformation
		err = json.Unmarshal(jsonBytes, &symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal workspace symbols: %w", err)
		}
		return symbols, nil
	default:
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
}

// extractSymbolInformation creates SymbolInformation from DocumentSymbol
func (idx *SimpleIndexer) extractSymbolInformation(
	documentSymbols []*lsp.DocumentSymbol,
	workspaceSymbols []*lsp.SymbolInformation,
	uri string,
) []*lsp.SymbolInformation {
	var result []*lsp.SymbolInformation

	// Add workspace symbols first
	result = append(result, workspaceSymbols...)

	// Convert document symbols to symbol information
	for _, docSymbol := range documentSymbols {
		symbolInfo := idx.documentSymbolToSymbolInfo(docSymbol, uri, "")
		result = append(result, symbolInfo...)
	}

	return result
}

// documentSymbolToSymbolInfo recursively converts DocumentSymbol to SymbolInformation
func (idx *SimpleIndexer) documentSymbolToSymbolInfo(
	docSymbol *lsp.DocumentSymbol,
	uri string,
	containerName string,
) []*lsp.SymbolInformation {
	var result []*lsp.SymbolInformation

	// Create symbol information for this symbol
	symbolInfo := &lsp.SymbolInformation{
		Name: docSymbol.Name,
		Kind: docSymbol.Kind,
		Location: lsp.Location{
			URI:   uri,
			Range: docSymbol.Range,
		},
		ContainerName: containerName,
	}
	result = append(result, symbolInfo)

	// Process children recursively
	childContainer := docSymbol.Name
	if containerName != "" {
		childContainer = containerName + "." + docSymbol.Name
	}

	for _, child := range docSymbol.Children {
		childSymbols := idx.documentSymbolToSymbolInfo(child, uri, childContainer)
		result = append(result, childSymbols...)
	}

	return result
}
