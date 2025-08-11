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
	// Determine worker count based on environment and project type
	workers := computeWorkers(hasJavaInLangs(languages))
	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	for wkr := 0; wkr < workers; wkr++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
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
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
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
	// Determine worker count based on environment and project type
	workers := computeWorkers(hasJavaInLangs(languages))
	var mu sync.Mutex
	jobs := make(chan int, workers)
	processed := 0
	totalSymbols := len(symbols)
	var wg sync.WaitGroup
	for wkr := 0; wkr < workers; wkr++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
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
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if progress != nil {
		// We don't track exact added count after dedup/flush; report completion
		progress("references_complete", processed, totalSymbols, "")
	}
	return nil
}
