// reference_indexer.go - Reference-specific indexing logic
// Contains functions for processing symbol references and creating reference occurrences

package cache

import (
	"context"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/protocol"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils/lspconv"
)

func (w *WorkspaceIndexer) getReferencesForSymbolInOpenFile(ctx context.Context, symbol indexedSymbol) ([]types.Location, error) {
	// Clamp position to file bounds to avoid LSP server line-number errors
	safePos := w.clampPositionToFile(symbol.uri, symbol.position)

	// Call types.MethodTextDocumentReferences (assumes file is already open)
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": symbol.uri,
		},
		"position": map[string]interface{}{
			"line":      safePos.Line,
			"character": safePos.Character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	// Per-request timeout to prevent hangs on problematic positions
	// Use a reasonable timeout for indexing operations (10s should be enough for most cases)
	// This timeout will be respected by the LSP client after our fix
	reqCtx, cancel := common.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := w.lspFallback.ProcessRequest(reqCtx, types.MethodTextDocumentReferences, params)
	if err != nil {
		// Use protocol's enhanced error suppression for expected indexing errors
		if protocol.IsExpectedSuppressibleError(err) {
			return []types.Location{}, nil
		}
		return nil, err
	}

	locs := lspconv.ParseLocations(result)
	if locs == nil {
		return []types.Location{}, nil
	}
	return locs, nil
}

func (w *WorkspaceIndexer) isSelfReference(ref types.Location, symbol indexedSymbol) bool {
	return ref.URI == symbol.uri &&
		ref.Range.Start.Line == symbol.position.Line &&
		ref.Range.Start.Character == symbol.position.Character
}

func (w *WorkspaceIndexer) createReferenceOccurrence(ref types.Location, symbol indexedSymbol) scip.SCIPOccurrence {
	return scip.SCIPOccurrence{
		Range: types.Range{
			Start: types.Position{
				Line:      ref.Range.Start.Line,
				Character: ref.Range.Start.Character,
			},
			End: types.Position{
				Line:      ref.Range.End.Line,
				Character: ref.Range.End.Character,
			},
		},
		Symbol:      symbol.symbolID,
		SymbolRoles: types.SymbolRoleReadAccess,
		SyntaxKind:  symbol.syntaxKind,
	}
}
