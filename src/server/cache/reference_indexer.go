// reference_indexer.go - Reference-specific indexing logic
// Contains functions for processing symbol references and creating reference occurrences

package cache

import (
	"context"
	"os"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/protocol"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/lspconv"
)

func (w *WorkspaceIndexer) processReferenceBatch(ctx context.Context, symbols []indexedSymbol, scipCache *SCIPCacheManager) int {
	// Group symbols by file to optimize file open/close operations
	symbolsByFile := make(map[string][]indexedSymbol)
	for _, symbol := range symbols {
		symbolsByFile[symbol.uri] = append(symbolsByFile[symbol.uri], symbol)
	}

	referencesByDoc := make(map[string][]scip.SCIPOccurrence)

	// Process each file and its symbols together
	for fileURI, fileSymbols := range symbolsByFile {
		// Open the file once for all symbols in it
		filePath := utils.URIToFilePath(fileURI)
		content, err := os.ReadFile(filePath)
		if err != nil {
			common.LSPLogger.Debug("Skipping file %s: %v", fileURI, err)
			continue
		}

		// Open the document in LSP server
		openParams := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        fileURI,
				"languageId": w.detectLanguageFromURI(fileURI),
				"version":    1,
				"text":       string(content),
			},
		}

		// Bound open request time
		openCtx, openCancel := common.WithTimeout(ctx, 2*time.Second)
		_, openErr := w.lspFallback.ProcessRequest(openCtx, "textDocument/didOpen", openParams)
		openCancel()
		if openErr != nil {
			// If the language server doesn't support didOpen, skip this file's references
			if strings.Contains(openErr.Error(), "Unhandled method") {
				common.LSPLogger.Debug("Language server doesn't support didOpen for %s, skipping references", fileURI)
				continue
			}
			common.LSPLogger.Debug("Failed to open document %s: %v", fileURI, openErr)
		}

		// Process all symbols in this file
		for _, symbol := range fileSymbols {
			references, err := w.getReferencesForSymbolInOpenFile(ctx, symbol)
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

		// Close the document after processing all its symbols
		closeParams := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		}
		closeCtx, closeCancel := common.WithTimeout(ctx, 1*time.Second)
		_, _ = w.lspFallback.ProcessRequest(closeCtx, "textDocument/didClose", closeParams)
		closeCancel()
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

func (w *WorkspaceIndexer) getReferencesForSymbolInOpenFile(ctx context.Context, symbol indexedSymbol) ([]types.Location, error) {
	// Clamp position to file bounds to avoid LSP server line-number errors
	safePos := w.clampPositionToFile(symbol.uri, symbol.position)

	// Call textDocument/references (assumes file is already open)
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
	reqCtx, cancel := common.WithTimeout(ctx, 2*time.Second)
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
