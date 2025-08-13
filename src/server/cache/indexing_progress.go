// indexing_progress.go - Progress tracking utilities and helper functions
// Contains utility functions for deduplication, position validation, and cache operations

package cache

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/lspconv"
)

// indexedSymbol represents a symbol found during indexing
type indexedSymbol struct {
	uri        string
	symbolID   string
	position   types.Position
	syntaxKind types.SyntaxKind
}

func dedupOccurrences(occs []scip.SCIPOccurrence) []scip.SCIPOccurrence {
	if len(occs) < 2 {
		return occs
	}
	seen := make(map[string]struct{}, len(occs))
	out := make([]scip.SCIPOccurrence, 0, len(occs))
	for _, o := range occs {
		key := fmt.Sprintf("%d:%d:%d:%d:%s", o.Range.Start.Line, o.Range.Start.Character, o.Range.End.Line, o.Range.End.Character, o.Symbol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, o)
	}
	return out
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

			// Use precise identifier position when available and clamp to file bounds
			pos := occ.Range.Start
			if occ.SelectionRange != nil {
				pos = occ.SelectionRange.Start
			}
			pos = w.clampPositionToFile(docURI, pos)
			uniqueSymbols[occ.Symbol] = indexedSymbol{
				uri:        docURI,
				symbolID:   occ.Symbol,
				position:   pos,
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

func (w *WorkspaceIndexer) parseRange(rangeData map[string]interface{}) types.Range {
	if r, ok := lspconv.ParseRangeFromMap(rangeData); ok {
		return r
	}
	return types.Range{}
}

// clampPositionToFile ensures the given position is within the file's line bounds
func (w *WorkspaceIndexer) clampPositionToFile(uri string, pos types.Position) types.Position {
	path := utils.URIToFilePathCached(uri)
	f, err := os.Open(path)
	if err != nil {
		return pos
	}
	defer f.Close()

	// Robust line counting using scanner; last valid 0-based index = max(0, count-1)
	var count int32 = 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	var maxLine int32 = 0
	if count > 0 {
		maxLine = count - 1
	}
	if pos.Line > maxLine {
		pos.Line = maxLine
	}
	if pos.Line < 0 {
		pos.Line = 0
	}
	if pos.Character < 0 {
		pos.Character = 0
	}
	return pos
}

func (w *WorkspaceIndexer) detectLanguageFromURI(uri string) string {
	path := utils.URIToFilePathCached(uri)
	ext := filepath.Ext(path)
	if lang, ok := registry.GetLanguageByExtension(ext); ok {
		return lang.Name
	}
	return "unknown"
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
	return m.WithIndexReadLock(func() interface{} {
		if storage, ok := m.scipStorage.(*scip.SimpleSCIPStorage); ok {
			return storage.GetAllDocuments()
		}
		return nil
	}).(map[string]*scip.SCIPDocument)
}

// AddOccurrences efficiently adds occurrences to a document using batch operations
func (m *SCIPCacheManager) AddOccurrences(ctx context.Context, uri string, occurrences []scip.SCIPOccurrence) error {
	result := m.WithIndexWriteLock(func() interface{} {
		return m.addOccurrencesInternal(ctx, uri, occurrences)
	})
	if result == nil {
		return nil
	}
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// addOccurrencesInternal contains the actual logic without locks
func (m *SCIPCacheManager) addOccurrencesInternal(ctx context.Context, uri string, occurrences []scip.SCIPOccurrence) error {

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
