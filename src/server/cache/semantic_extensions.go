package cache

import (
	"fmt"
	"path/filepath"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/server/scip"
)

// CallGraphNode represents a function/method and its call relationships
type CallGraphNode struct {
	Symbol   *lsp.SymbolInformation
	Calls    []string // Symbol IDs that this function calls
	CalledBy []string // Symbol IDs that call this function
}

// PathIndex organizes symbols by file path segments for hierarchical queries
type PathIndex struct {
	Path    string                              // File path segment (e.g., "src/server", "src/cli")
	Symbols []*lsp.SymbolInformation            // Top-level symbols in this path
	Files   map[string][]*lsp.SymbolInformation // symbols by file within path
}

// SignatureInfo contains rich signature information for symbols
type SignatureInfo struct {
	Symbol     *lsp.SymbolInformation
	Parameters []ParameterInfo `json:"parameters,omitempty"`
	ReturnType string          `json:"return_type,omitempty"`
	ClassName  string          `json:"class_name,omitempty"` // For methods
	Package    string          `json:"package,omitempty"`    // Package/namespace
	Generics   []string        `json:"generics,omitempty"`   // Generic type parameters
}

type ParameterInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SemanticCacheManager enhances basic cache with semantic indexing
type SemanticCacheManager struct {
	responseCache *SimpleCacheManager      // Raw LSP response cache
	scipStorage   scip.SCIPDocumentStorage // Semantic storage (from existing code)

	// New semantic indexes for LLM features
	callGraph   map[string]*CallGraphNode  // Symbol ID -> call relationships
	pathIndex   map[string]*PathIndex      // Path segment -> symbols
	signatures  map[string]*SignatureInfo  // Symbol ID -> rich signature info
	defRefLinks map[string][]*lsp.Location // Symbol ID -> all reference locations
}

// NewSemanticCacheManager creates enhanced cache with existing components
func NewSemanticCacheManager(responseCache *SimpleCacheManager, scipStorage scip.SCIPDocumentStorage) *SemanticCacheManager {
	return &SemanticCacheManager{
		responseCache: responseCache,
		scipStorage:   scipStorage,
		callGraph:     make(map[string]*CallGraphNode),
		pathIndex:     make(map[string]*PathIndex),
		signatures:    make(map[string]*SignatureInfo),
		defRefLinks:   make(map[string][]*lsp.Location),
	}
}

// StoreWithSemanticIndexing stores both raw response and semantic information
func (m *SemanticCacheManager) StoreWithSemanticIndexing(method string, params interface{}, response interface{}) error {
	// Store raw response in existing cache
	if err := m.responseCache.Store(method, params, response); err != nil {
		return err
	}

	// Extract and store semantic information based on LSP method
	switch method {
	case "textDocument/documentSymbol":
		return m.indexDocumentSymbols(params, response)
	case "textDocument/references":
		return m.indexReferences(params, response)
	case "textDocument/hover":
		return m.indexHoverSignatures(params, response)
	}

	return nil
}

// GetCallGraph returns call relationships for LLM analysis
func (m *SemanticCacheManager) GetCallGraph(symbolID string) (*CallGraphNode, error) {
	if node, exists := m.callGraph[symbolID]; exists {
		return node, nil
	}
	return nil, fmt.Errorf("call graph not found for symbol: %s", symbolID)
}

// GetSymbolsByPath returns top-level symbols under path hierarchy
func (m *SemanticCacheManager) GetSymbolsByPath(pathPattern string) ([]*lsp.SymbolInformation, error) {
	var results []*lsp.SymbolInformation

	for path, index := range m.pathIndex {
		if matched, _ := filepath.Match(pathPattern, path); matched {
			results = append(results, index.Symbols...)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no symbols found for path pattern: %s", pathPattern)
	}

	return results, nil
}

// GetSignatureInfo returns rich signature information for LLM context
func (m *SemanticCacheManager) GetSignatureInfo(symbolID string) (*SignatureInfo, error) {
	if sig, exists := m.signatures[symbolID]; exists {
		return sig, nil
	}
	return nil, fmt.Errorf("signature info not found for symbol: %s", symbolID)
}

// GetDefinitionWithReferences returns definition + all reference locations
func (m *SemanticCacheManager) GetDefinitionWithReferences(symbolID string) ([]*lsp.Location, error) {
	if refs, exists := m.defRefLinks[symbolID]; exists {
		return refs, nil
	}
	return nil, fmt.Errorf("definition-reference links not found for symbol: %s", symbolID)
}

// Private indexing methods

func (m *SemanticCacheManager) indexDocumentSymbols(params interface{}, response interface{}) error {
	// Extract URI from params
	uri, err := m.extractURIFromParams(params)
	if err != nil {
		return err
	}

	// Parse document symbols from response
	symbols, ok := response.([]*lsp.DocumentSymbol)
	if !ok {
		return fmt.Errorf("invalid document symbol response type")
	}

	// Build path index
	m.buildPathIndex(uri, symbols)

	// Extract call relationships (basic implementation)
	m.buildCallGraph(symbols)

	common.LSPLogger.Debug("Indexed semantic information for %s: %d symbols", uri, len(symbols))
	return nil
}

// extractURIFromParams extracts URI from various parameter formats
func (m *SemanticCacheManager) extractURIFromParams(params interface{}) (string, error) {
	switch p := params.(type) {
	case map[string]interface{}:
		if textDoc, ok := p["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok {
				return uri, nil
			}
		}
	}
	return "", fmt.Errorf("could not extract URI from params")
}

func (m *SemanticCacheManager) indexReferences(params interface{}, response interface{}) error {
	// Extract symbol position from params for symbol identification
	// Store reference locations for definition-reference links
	locations, ok := response.([]*lsp.Location)
	if !ok {
		return fmt.Errorf("invalid references response type")
	}

	// Use first location as definition, rest as references
	if len(locations) > 0 && locations[0] != nil {
		symbolID := m.generateSymbolID(locations[0])
		// Filter out nil locations before storing
		var validLocations []*lsp.Location
		for _, loc := range locations {
			if loc != nil {
				validLocations = append(validLocations, loc)
			}
		}
		if len(validLocations) > 0 {
			m.defRefLinks[symbolID] = validLocations
		}
	}

	return nil
}

func (m *SemanticCacheManager) indexHoverSignatures(params interface{}, response interface{}) error {
	// Extract rich signature information from hover responses
	_, ok := response.(*lsp.Hover)
	if !ok {
		return nil // Not all hovers contain signature info
	}

	// Parse signature from hover contents and store
	// This would extract parameter types, return types, etc.

	return nil
}

func (m *SemanticCacheManager) buildPathIndex(uri string, symbols []*lsp.DocumentSymbol) {
	// Extract path segments (e.g., "src/server/cache" from full file path)
	// Strip file:// scheme prefix if present
	filePath := strings.TrimPrefix(uri, "file://")
	dir := filepath.Dir(filePath)
	pathSegments := strings.Split(dir, "/")

	// Build hierarchical index by path depth
	for i := 1; i <= len(pathSegments); i++ {
		pathKey := strings.Join(pathSegments[:i], "/")

		if _, exists := m.pathIndex[pathKey]; !exists {
			m.pathIndex[pathKey] = &PathIndex{
				Path:    pathKey,
				Symbols: []*lsp.SymbolInformation{},
				Files:   make(map[string][]*lsp.SymbolInformation),
			}
		}

		// Convert DocumentSymbol to SymbolInformation for storage
		for _, docSymbol := range symbols {
			if isTopLevelSymbol(docSymbol) { // Only top-level symbols for path index
				symbolInfo := documentSymbolToSymbolInfo(docSymbol, uri)
				m.pathIndex[pathKey].Symbols = append(m.pathIndex[pathKey].Symbols, symbolInfo)
				m.pathIndex[pathKey].Files[uri] = append(m.pathIndex[pathKey].Files[uri], symbolInfo)
			}
		}
	}
}

func (m *SemanticCacheManager) buildCallGraph(symbols []*lsp.DocumentSymbol) {
	// Basic call graph extraction (can be enhanced later)
	for _, symbol := range symbols {
		if symbol.Kind == lsp.Function || symbol.Kind == lsp.Method {
			symbolID := m.generateSymbolIDFromDocSymbol(symbol)

			if _, exists := m.callGraph[symbolID]; !exists {
				m.callGraph[symbolID] = &CallGraphNode{
					Symbol:   documentSymbolToSymbolInfo(symbol, ""),
					Calls:    []string{},
					CalledBy: []string{},
				}
			}
		}
	}
}

// Helper functions

func (m *SemanticCacheManager) generateSymbolID(location *lsp.Location) string {
	if location == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", location.URI, location.Range.Start.Line, location.Range.Start.Character)
}

func (m *SemanticCacheManager) generateSymbolIDFromDocSymbol(symbol *lsp.DocumentSymbol) string {
	return fmt.Sprintf("%s:%d:%d", symbol.Name, symbol.Range.Start.Line, symbol.Range.Start.Character)
}

func isTopLevelSymbol(symbol *lsp.DocumentSymbol) bool {
	// Consider classes, functions, constants as top-level
	return symbol.Kind == lsp.Class || symbol.Kind == lsp.Function ||
		symbol.Kind == lsp.Variable || symbol.Kind == lsp.Constant ||
		symbol.Kind == lsp.Interface || symbol.Kind == lsp.Enum
}

func documentSymbolToSymbolInfo(docSymbol *lsp.DocumentSymbol, uri string) *lsp.SymbolInformation {
	return &lsp.SymbolInformation{
		Name: docSymbol.Name,
		Kind: docSymbol.Kind,
		Location: lsp.Location{
			URI:   uri,
			Range: docSymbol.Range,
		},
	}
}
