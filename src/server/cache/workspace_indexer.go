// workspace_indexer.go - Core workspace indexing coordination logic
// Contains primary indexing orchestration functions for workspace symbol processing

package cache

import (
	"context"
	"sync"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
)

// IndexWorkspaceFilesWithReferences performs enhanced workspace indexing that includes both
// symbol definitions AND their references. This creates a complete SCIP index suitable
// for findReferences operations.
func (w *WorkspaceIndexer) IndexWorkspaceFilesWithReferences(ctx context.Context, workspaceDir string, languages []string, maxFiles int, scipCache *SCIPCacheManager) error {
	if scipCache == nil {
		return nil
	}
	documents := scipCache.GetAllDocuments()
	symbols := w.collectUniqueDefinitions(documents)
	common.LSPLogger.Debug("Found %d unique symbols to process", len(symbols))
	symbolsByFile := make(map[string][]indexedSymbol)
	for _, s := range symbols {
		symbolsByFile[s.uri] = append(symbolsByFile[s.uri], s)
	}
	files := make([]string, 0, len(symbolsByFile))
	for k := range symbolsByFile {
		files = append(files, k)
	}
	// Process references using worker pool
	workers := computeWorkers(hasJavaInLangs(languages))
	pool := NewWorkerPool(workers)
	pool.Execute(len(files), func(idx int) error {
		fileURI := files[idx]
		fileSymbols := symbolsByFile[fileURI]
		// Rely on LSP manager to ensure didOpen via DocumentManager
		localRefs := make(map[string][]scip.SCIPOccurrence)
		for _, symbol := range fileSymbols {
			refs, err := w.getReferencesForSymbolInOpenFile(ctx, symbol)
			if err != nil {
				continue
			}
			for _, ref := range refs {
				if w.isSelfReference(ref, symbol) {
					continue
				}
				occ := w.createReferenceOccurrence(ref, symbol)
				localRefs[ref.URI] = append(localRefs[ref.URI], occ)
			}
		}
		// Flush per-doc to storage to bound memory (no global lock needed)
		for uri, occs := range localRefs {
			occs = dedupOccurrences(occs)
			_ = scipCache.AddOccurrences(ctx, uri, occs)
		}
		return nil
	})
	common.LSPLogger.Debug("Indexing complete: %d symbols (references flushed per doc)", len(symbols))
	return nil
}

func (w *WorkspaceIndexer) IndexWorkspaceFilesWithReferencesProgress(ctx context.Context, workspaceDir string, languages []string, maxFiles int, scipCache *SCIPCacheManager, progress IndexProgressFunc) error {
	if scipCache == nil {
		return nil
	}
	documents := scipCache.GetAllDocuments()
	symbols := w.collectUniqueDefinitions(documents)
	symbolsByFile := make(map[string][]indexedSymbol)
	for _, s := range symbols {
		symbolsByFile[s.uri] = append(symbolsByFile[s.uri], s)
	}
	files := make([]string, 0, len(symbolsByFile))
	for k := range symbolsByFile {
		files = append(files, k)
	}
	if progress != nil {
		progress("references_start", 0, len(symbols), "")
	}
	// Process references using worker pool with progress
	workers := computeWorkers(hasJavaInLangs(languages))
	pool := NewWorkerPool(workers)
	var mu sync.Mutex
	processed := 0
	totalSymbols := len(symbols)

	pool.Execute(len(files), func(idx int) error {
		fileURI := files[idx]
		fileSymbols := symbolsByFile[fileURI]
		// Rely on LSPManager.ensureDocumentOpen via ProcessRequest
		localRefs := make(map[string][]scip.SCIPOccurrence)
		for _, symbol := range fileSymbols {
			refs, err := w.getReferencesForSymbolInOpenFile(ctx, symbol)
			if err != nil {
				continue
			}
			for _, ref := range refs {
				if w.isSelfReference(ref, symbol) {
					continue
				}
				occ := w.createReferenceOccurrence(ref, symbol)
				localRefs[ref.URI] = append(localRefs[ref.URI], occ)
			}
		}
		// Flush per-doc to storage to bound memory (no global lock needed)
		for uri, occs := range localRefs {
			occs = dedupOccurrences(occs)
			_ = scipCache.AddOccurrences(ctx, uri, occs)
		}
		// No explicit didClose; let LSP manager track lifecycle
		mu.Lock()
		processed += len(fileSymbols)
		if progress != nil {
			progress("references", processed, totalSymbols, "")
		}
		mu.Unlock()
		return nil
	})
	if progress != nil {
		// We don't track exact added count after dedup/flush; report completion
		progress("references_complete", processed, totalSymbols, "")
	}
	return nil
}
