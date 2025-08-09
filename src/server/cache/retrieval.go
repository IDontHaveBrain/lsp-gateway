package cache

import (
	"context"
	"strings"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
)

// GetCachedDefinition retrieves definition occurrences for a symbol using SCIP storage
func (m *SCIPCacheManager) GetCachedDefinition(symbolID string) ([]types.Location, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	if defs, err := m.scipStorage.GetDefinitionsWithDocuments(context.Background(), symbolID); err == nil && len(defs) > 0 {
		def := defs[0]
		location := types.Location{
			URI: def.DocumentURI,
			Range: types.Range{
				Start: types.Position{Line: int32(def.Range.Start.Line), Character: int32(def.Range.Start.Character)},
				End:   types.Position{Line: int32(def.Range.End.Line), Character: int32(def.Range.End.Character)},
			},
		}
		return []types.Location{location}, true
	}
	return nil, false
}

// GetCachedReferences retrieves reference occurrences for a symbol using SCIP storage
func (m *SCIPCacheManager) GetCachedReferences(symbolID string) ([]types.Location, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	refs, err := m.scipStorage.GetReferencesWithDocuments(context.Background(), symbolID)
	if err != nil || len(refs) == 0 {
		return nil, false
	}
	locations := make([]types.Location, 0, len(refs))
	for _, ref := range refs {
		locations = append(locations, types.Location{
			URI: ref.DocumentURI,
			Range: types.Range{
				Start: types.Position{Line: int32(ref.Range.Start.Line), Character: int32(ref.Range.Start.Character)},
				End:   types.Position{Line: int32(ref.Range.End.Line), Character: int32(ref.Range.End.Character)},
			},
		})
	}
	return locations, true
}

// GetCachedHover retrieves hover information using symbol information from SCIP storage
func (m *SCIPCacheManager) GetCachedHover(symbolID string) (*lsp.Hover, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfo, err := m.scipStorage.GetSymbolInfo(context.Background(), symbolID)
	if err != nil {
		return nil, false
	}

	hover := &lsp.Hover{
		Contents: m.formatHoverFromSCIPSymbolInfo(symbolInfo),
	}

	return hover, true
}

// GetCachedDocumentSymbols retrieves document symbols using SCIP storage
func (m *SCIPCacheManager) GetCachedDocumentSymbols(uri string) ([]types.SymbolInformation, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// GetDocumentSymbols not available - would need to get document and extract symbols
	doc, err := m.scipStorage.GetDocument(context.Background(), uri)
	if err != nil {
		return nil, false
	}
	symbolInfos := doc.SymbolInformation
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	symbols := make([]types.SymbolInformation, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		defs, err := m.scipStorage.GetDefinitionsWithDocuments(context.Background(), scipSymbol.Symbol)
		if err != nil || len(defs) == 0 {
			continue
		}
		defOcc := defs[0]

		symbol := types.SymbolInformation{
			Name: scipSymbol.DisplayName,
			Kind: m.convertSCIPSymbolKindToLSP(scipSymbol.Kind),
			Location: types.Location{
				URI:   uri,
				Range: types.Range{Start: types.Position{Line: int32(defOcc.Range.Start.Line), Character: int32(defOcc.Range.Start.Character)}, End: types.Position{Line: int32(defOcc.Range.End.Line), Character: int32(defOcc.Range.End.Character)}},
			},
		}
		symbols = append(symbols, symbol)
	}

	return symbols, true
}

// GetCachedWorkspaceSymbols retrieves workspace symbols using SCIP storage
func (m *SCIPCacheManager) GetCachedWorkspaceSymbols(query string) ([]types.SymbolInformation, bool) {
	if !m.enabled {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, err := m.scipStorage.SearchSymbols(context.Background(), query, 100)
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	symbols := make([]types.SymbolInformation, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		defs, err := m.scipStorage.GetDefinitionsWithDocuments(context.Background(), scipSymbol.Symbol)
		if err != nil || len(defs) == 0 {
			continue
		}
		defOcc := defs[0]

		symbol := types.SymbolInformation{
			Name: scipSymbol.DisplayName,
			Kind: m.convertSCIPSymbolKindToLSP(scipSymbol.Kind),
			Location: types.Location{
				URI:   defOcc.DocumentURI,
				Range: types.Range{Start: types.Position{Line: int32(defOcc.Range.Start.Line), Character: int32(defOcc.Range.Start.Character)}, End: types.Position{Line: int32(defOcc.Range.End.Line), Character: int32(defOcc.Range.End.Character)}},
			},
		}
		symbols = append(symbols, symbol)
	}

	return symbols, true
}

// GetCachedCompletion retrieves completion items using symbol information from SCIP storage
func (m *SCIPCacheManager) GetCachedCompletion(uri string, position types.Position) ([]lsp.CompletionItem, bool) {
	if !m.enabled || m.scipStorage == nil {
		return nil, false
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Get document symbols for completion context
	// GetDocumentSymbols not available - would need to get document and extract symbols
	doc, err := m.scipStorage.GetDocument(context.Background(), uri)
	if err != nil {
		return nil, false
	}
	symbolInfos := doc.SymbolInformation
	if err != nil || len(symbolInfos) == 0 {
		return nil, false
	}

	// Convert symbol information to completion items
	items := make([]lsp.CompletionItem, 0, len(symbolInfos))
	for _, scipSymbol := range symbolInfos {
		item := lsp.CompletionItem{
			Label:  scipSymbol.DisplayName,
			Kind:   m.convertSCIPSymbolKindToCompletionItemKind(scipSymbol.Kind),
			Detail: m.formatSymbolDetail(&scipSymbol),
		}

		if len(scipSymbol.Documentation) > 0 {
			item.Documentation = strings.Join(scipSymbol.Documentation, "\n")
		}

		items = append(items, item)
	}

	return items, true
}