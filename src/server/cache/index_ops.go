package cache

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// IndexDocument indexes LSP symbols using occurrence-centric SCIP storage
func (m *SCIPCacheManager) IndexDocument(ctx context.Context, uri, language string, symbols []types.SymbolInformation) error {
	if !m.enabled {
		return nil
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	doc, err := m.createSCIPDocument(uri, language, symbols)
	if err != nil {
		return fmt.Errorf("failed to create SCIP document: %w", err)
	}

	if err := m.scipStorage.StoreDocument(ctx, doc); err != nil {
		return fmt.Errorf("failed to store SCIP document: %w", err)
	}

	m.updateIndexStats(language, len(symbols))
	return nil
}

// UpdateSymbolIndex converts LSP symbols to SCIP occurrences with enhanced range information
func (m *SCIPCacheManager) UpdateSymbolIndex(uri string, symbols []*types.SymbolInformation, documentSymbols []*lsp.DocumentSymbol) error {
	if !m.enabled {
		return nil
	}

	docSymbolMap := make(map[string]*lsp.DocumentSymbol)
	if documentSymbols != nil {
		m.flattenDocumentSymbols(documentSymbols, docSymbolMap, "")
	}

	language := m.converter.DetectLanguageFromURI(uri)

	enhancedSymbols := make([]types.SymbolInformation, 0, len(symbols))
	for _, sym := range symbols {
		if sym != nil {
			enhancedSym := *sym
			if docSym, found := docSymbolMap[sym.Name]; found {
				enhancedSym.Location.Range = docSym.Range
			}
			enhancedSymbols = append(enhancedSymbols, enhancedSym)
		}
	}

	return m.IndexDocument(context.Background(), uri, language, enhancedSymbols)
}

// flattenDocumentSymbols recursively flattens DocumentSymbols into a map
func (m *SCIPCacheManager) flattenDocumentSymbols(symbols []*lsp.DocumentSymbol, result map[string]*lsp.DocumentSymbol, parentPath string) {
	for _, sym := range symbols {
		if sym != nil {
			fullPath := sym.Name
			if parentPath != "" {
				fullPath = parentPath + "/" + sym.Name
			}

			// Store both with simple name and full path
			result[sym.Name] = sym
			result[fullPath] = sym

			if sym.Children != nil {
				m.flattenDocumentSymbols(sym.Children, result, fullPath)
			}
		}
	}
}

// createSCIPDocument creates a SCIP document from LSP symbols
func (m *SCIPCacheManager) createSCIPDocument(uri, language string, symbols []types.SymbolInformation) (*scip.SCIPDocument, error) {
	ctx := NewConversionContext(uri, language).
		WithSymbolRoles(types.SymbolRoleDefinition)

	return m.converter.ConvertLSPSymbolsToSCIPDocument(ctx, symbols)
}

// convertLSPSymbolsToSCIPDocument converts LSP symbols to a SCIP document format
func (m *SCIPCacheManager) convertLSPSymbolsToSCIPDocument(uri, language string, symbols []types.SymbolInformation) (*scip.SCIPDocument, error) {
	ctx := NewConversionContext(uri, language).
		WithSymbolRoles(types.SymbolRoleDefinition)

	return m.converter.ConvertLSPSymbolsToSCIPDocument(ctx, symbols)
}

// GetIndexStats returns current index statistics
func (m *SCIPCacheManager) GetIndexStats() *IndexStats {
	if !m.enabled {
		return &IndexStats{Status: "disabled"}
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	stats := *m.indexStats

	scipStats := m.scipStorage.GetIndexStats()
	stats.SymbolCount = scipStats.TotalSymbols
	stats.DocumentCount = int64(scipStats.TotalDocuments)
	stats.ReferenceCount = scipStats.TotalOccurrences
	stats.IndexSize = scipStats.MemoryUsage

	return &stats
}

// UpdateIndex updates the index with the given files
func (m *SCIPCacheManager) UpdateIndex(ctx context.Context, files []string) error {
	if !m.enabled {
		return nil
	}
	// Indexing happens when LSP methods return symbol information
	return nil
}

// updateIndexStats updates the index statistics with new data
func (m *SCIPCacheManager) updateIndexStats(language string, symbolCount int) {
	m.indexStats.LastUpdate = time.Now()
	m.indexStats.Status = "active"

	if m.indexStats.LanguageStats[language] == 0 {
		m.indexStats.IndexedLanguages = append(m.indexStats.IndexedLanguages, language)
	}
	m.indexStats.LanguageStats[language] += int64(symbolCount)

	stats := m.scipStorage.GetIndexStats()
	m.indexStats.SymbolCount = stats.TotalSymbols
	m.indexStats.DocumentCount = int64(stats.TotalDocuments)
}

// Helper methods for conversion between LSP and SCIP types

// convertLSPSymbolKindToSCIPKind converts LSP symbol kind to SCIP symbol kind
func (m *SCIPCacheManager) convertLSPSymbolKindToSCIPKind(kind types.SymbolKind) scip.SCIPSymbolKind {
	return m.converter.ConvertLSPSymbolKindToSCIP(kind)
}

// convertSCIPSymbolKindToLSP converts SCIP symbol kind back to LSP
func (m *SCIPCacheManager) convertSCIPSymbolKindToLSP(kind scip.SCIPSymbolKind) types.SymbolKind {
	return m.converter.ConvertSCIPSymbolKindToLSP(kind)
}

// convertLSPSymbolKindToSyntaxKind converts LSP symbol kind to syntax kind for highlighting
func (m *SCIPCacheManager) convertLSPSymbolKindToSyntaxKind(kind types.SymbolKind) types.SyntaxKind {
	return m.converter.ConvertLSPSymbolKindToSyntax(kind)
}

// convertSCIPSymbolKindToCompletionItemKind converts SCIP symbol kind to completion item kind
func (m *SCIPCacheManager) convertSCIPSymbolKindToCompletionItemKind(kind scip.SCIPSymbolKind) lsp.CompletionItemKind {
	return m.converter.ConvertSCIPSymbolKindToCompletionItem(kind)
}
