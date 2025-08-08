package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// indexedSymbol represents a symbol found during indexing
type indexedSymbol struct {
	uri        string
	symbolID   string
	position   types.Position
	syntaxKind types.SyntaxKind
}

// IndexWorkspaceFilesWithReferences performs enhanced workspace indexing that includes both
// symbol definitions AND their references. This creates a complete SCIP index suitable
// for findReferences operations.
func (w *WorkspaceIndexer) IndexWorkspaceFilesWithReferences(ctx context.Context, workspaceDir string, languages []string, maxFiles int, scipCache *SCIPCacheManager) error {
	// Step 1: Basic indexing
	if err := w.IndexWorkspaceFiles(ctx, workspaceDir, languages, maxFiles); err != nil {
		return fmt.Errorf("failed to index workspace files: %w", err)
	}

	if scipCache == nil {
		return nil
	}

	// Step 2: Collect unique definitions
	documents := scipCache.GetAllDocuments()
	symbols := w.collectUniqueDefinitions(documents)

	common.LSPLogger.Info("Found %d unique symbols to process", len(symbols))

	// Step 3: Process in batches
	const batchSize = 50
	totalReferences := 0

	for i := 0; i < len(symbols); i += batchSize {
		end := min(i+batchSize, len(symbols))
		batch := symbols[i:end]

		added := w.processReferenceBatch(ctx, batch, scipCache)
		totalReferences += added

		if totalReferences > 0 && totalReferences%100 == 0 {
			common.LSPLogger.Info("Added %d references so far...", totalReferences)
		}
	}

	common.LSPLogger.Info("Indexing complete: %d symbols, %d references", len(symbols), totalReferences)

	return scipCache.SaveIndexToDisk()
}

func (w *WorkspaceIndexer) collectUniqueDefinitions(documents map[string]*scip.SCIPDocument) []indexedSymbol {
	uniqueSymbols := make(map[string]indexedSymbol)

	for docURI, doc := range documents {
		if doc == nil {
			continue
		}

		for _, occ := range doc.Occurrences {
			// Only process definitions
			if !occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
				continue
			}

			// Skip builtins once
			if w.isBuiltinSymbol(occ.Symbol) {
				continue
			}

			// Skip invalid syntax kinds that are unlikely to have valid identifiers
			if !w.isValidIdentifierSyntaxKind(occ.SyntaxKind) {
				continue
			}

			// Skip positions that appear invalid (negative or suspiciously large)
			if occ.Range.Start.Line < 0 || occ.Range.Start.Character < 0 {
				continue
			}

			// Use the occurrence range start position (which should be SelectionRange if available)
			// This ensures we're using the precise identifier location, not the full symbol range
			uniqueSymbols[occ.Symbol] = indexedSymbol{
				uri:        docURI,
				symbolID:   occ.Symbol,
				position:   occ.Range.Start, // This is now the precise position from SelectionRange
				syntaxKind: occ.SyntaxKind,
			}
		}
	}

	// Convert to slice
	result := make([]indexedSymbol, 0, len(uniqueSymbols))
	for _, sym := range uniqueSymbols {
		result = append(result, sym)
	}

	return result
}

func (w *WorkspaceIndexer) processReferenceBatch(ctx context.Context, symbols []indexedSymbol, scipCache *SCIPCacheManager) int {
	referencesByDoc := make(map[string][]scip.SCIPOccurrence)

	for _, symbol := range symbols {
		references, err := w.getReferencesForSymbol(ctx, symbol)
		if err != nil {
			continue
		}

		for _, ref := range references {
			// Skip self-references
			if w.isSelfReference(ref, symbol) {
				continue
			}

			occurrence := w.createReferenceOccurrence(ref, symbol)
			referencesByDoc[ref.URI] = append(referencesByDoc[ref.URI], occurrence)
		}
	}

	// Batch update all documents
	totalAdded := 0
	for uri, occurrences := range referencesByDoc {
		if err := scipCache.AddOccurrences(ctx, uri, occurrences); err == nil {
			totalAdded += len(occurrences)
		}
	}

	return totalAdded
}

func (w *WorkspaceIndexer) getReferencesForSymbol(ctx context.Context, symbol indexedSymbol) ([]lsp.Location, error) {

	// Call textDocument/references directly
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": symbol.uri,
		},
		"position": map[string]interface{}{
			"line":      symbol.position.Line,
			"character": symbol.position.Character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	result, err := w.lspFallback.ProcessRequest(ctx, types.MethodTextDocumentReferences, params)
	if err != nil {
		// Silently skip "no identifier found" errors - these are expected for some positions
		if strings.Contains(err.Error(), "no identifier found") {
			return []lsp.Location{}, nil
		}
		return nil, err
	}

	return w.parseLocationResponse(result)
}

func (w *WorkspaceIndexer) parseLocationResponse(result interface{}) ([]lsp.Location, error) {
	locations := []lsp.Location{}

	switch refs := result.(type) {
	case json.RawMessage:
		var refArray []interface{}
		if err := json.Unmarshal(refs, &refArray); err == nil {
			for _, ref := range refArray {
				if refData, err := json.Marshal(ref); err == nil {
					var loc lsp.Location
					if err := json.Unmarshal(refData, &loc); err == nil {
						locations = append(locations, loc)
					}
				}
			}
		}
	case []interface{}:
		for _, ref := range refs {
			if refMap, ok := ref.(map[string]interface{}); ok {
				loc := lsp.Location{}
				if uri, ok := refMap["uri"].(string); ok {
					loc.URI = uri
				}
				if rangeData, ok := refMap["range"].(map[string]interface{}); ok {
					loc.Range = w.parseRange(rangeData)
				}
				locations = append(locations, loc)
			}
		}
	case []lsp.Location:
		locations = refs
	}

	return locations, nil
}

func (w *WorkspaceIndexer) parseRange(rangeData map[string]interface{}) lsp.Range {
	r := lsp.Range{}

	if start, ok := rangeData["start"].(map[string]interface{}); ok {
		if line, ok := start["line"].(float64); ok {
			r.Start.Line = int(line)
		}
		if char, ok := start["character"].(float64); ok {
			r.Start.Character = int(char)
		}
	}

	if end, ok := rangeData["end"].(map[string]interface{}); ok {
		if line, ok := end["line"].(float64); ok {
			r.End.Line = int(line)
		}
		if char, ok := end["character"].(float64); ok {
			r.End.Character = int(char)
		}
	}

	return r
}

func (w *WorkspaceIndexer) isSelfReference(ref lsp.Location, symbol indexedSymbol) bool {
	return ref.URI == symbol.uri &&
		int32(ref.Range.Start.Line) == symbol.position.Line &&
		int32(ref.Range.Start.Character) == symbol.position.Character
}

func (w *WorkspaceIndexer) createReferenceOccurrence(ref lsp.Location, symbol indexedSymbol) scip.SCIPOccurrence {
	return scip.SCIPOccurrence{
		Range: types.Range{
			Start: types.Position{
				Line:      int32(ref.Range.Start.Line),
				Character: int32(ref.Range.Start.Character),
			},
			End: types.Position{
				Line:      int32(ref.Range.End.Line),
				Character: int32(ref.Range.End.Character),
			},
		},
		Symbol:      symbol.symbolID,
		SymbolRoles: types.SymbolRoleReadAccess,
		SyntaxKind:  symbol.syntaxKind,
	}
}

func (w *WorkspaceIndexer) detectLanguageFromURI(uri string) string {
	ext := filepath.Ext(uri)
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	default:
		return "unknown"
	}
}

func (w *WorkspaceIndexer) isValidIdentifierSyntaxKind(syntaxKind types.SyntaxKind) bool {
	// Skip unspecified syntax kinds first
	if syntaxKind == types.SyntaxKindUnspecified {
		return false
	}

	// Only allow syntax kinds that represent actual identifiers
	switch syntaxKind {
	case types.SyntaxKindIdentifierFunction,
		types.SyntaxKindIdentifierFunctionDefinition,
		types.SyntaxKindIdentifierType,
		types.SyntaxKindIdentifierBuiltinType,
		types.SyntaxKindIdentifierLocal,
		types.SyntaxKindIdentifierConstant,
		types.SyntaxKindIdentifierMutableGlobal,
		types.SyntaxKindIdentifierNamespace,
		types.SyntaxKindIdentifierModule,
		types.SyntaxKindIdentifierAttribute,
		types.SyntaxKindIdentifierParameter,
		types.SyntaxKindIdentifierBuiltin,
		types.SyntaxKindIdentifierNull,
		types.SyntaxKindIdentifierShadowed,
		types.SyntaxKindIdentifierMacro,
		types.SyntaxKindIdentifierMacroDefinition:
		return true
	default:
		// Skip non-identifier syntax kinds like comments, strings, etc.
		return false
	}
}

func (w *WorkspaceIndexer) isBuiltinSymbol(symbol string) bool {
	// Consolidated builtin types from across all supported languages
	builtinTypes := map[string]bool{
		// Go builtins
		"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true, "interface{}": true, "map": true, "chan": true,
		"complex64": true, "complex128": true, "uintptr": true,
		// TypeScript/JavaScript builtins
		"number": true, "boolean": true, "object": true, "symbol": true, "undefined": true,
		"null": true, "void": true, "never": true, "unknown": true, "bigint": true,
		"Array": true, "Object": true, "String": true, "Number": true, "Boolean": true,
		"Promise": true, "Date": true, "RegExp": true, "Error": true, "Map": true, "Set": true,
		// Python builtins
		"float": true, "str": true, "list": true, "dict": true,
		"tuple": true, "set": true, "None": true, "bytes": true, "bytearray": true,
		"type": true, "complex": true,
		// Java builtins
		"long": true, "short": true, "double": true, "char": true,
		"Integer": true, "Long": true, "Double": true, "Float": true,
		"Character": true, "Byte": true, "Short": true,
	}

	// Extract the symbol name from qualified paths
	parts := strings.Split(symbol, ".")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Clean up any trailing syntax
		lastPart = strings.TrimSuffix(lastPart, "()")
		lastPart = strings.TrimSuffix(lastPart, "#")
		lastPart = strings.TrimSuffix(lastPart, ".")

		if builtinTypes[lastPart] {
			return true
		}
	}

	return builtinTypes[symbol]
}

// min is a utility function for getting the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetAllDocuments returns all documents in the cache
func (m *SCIPCacheManager) GetAllDocuments() map[string]*scip.SCIPDocument {
	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	if storage, ok := m.scipStorage.(*scip.SimpleSCIPStorage); ok {
		return storage.GetAllDocuments()
	}
	return nil
}

// AddOccurrences efficiently adds occurrences to a document using batch operations
func (m *SCIPCacheManager) AddOccurrences(ctx context.Context, uri string, occurrences []scip.SCIPOccurrence) error {
	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	if storage, ok := m.scipStorage.(*scip.SimpleSCIPStorage); ok {
		return storage.AddOccurrences(ctx, uri, occurrences)
	}

	// Fallback implementation
	doc, err := m.scipStorage.GetDocument(ctx, uri)
	if err != nil {
		doc = &scip.SCIPDocument{
			URI:         uri,
			Occurrences: occurrences,
		}
		return m.scipStorage.StoreDocument(ctx, doc)
	}

	doc.Occurrences = append(doc.Occurrences, occurrences...)
	return m.scipStorage.StoreDocument(ctx, doc)
}
