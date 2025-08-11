// indexing_workers.go - Worker pool management for indexing operations
// Contains functions for coordinating parallel indexing work across worker goroutines

package cache

import (
	"context"
	"path/filepath"
	"sync"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
)

// IndexSpecificFilesWithReferences indexes references for specific files only
func (w *WorkspaceIndexer) IndexSpecificFilesWithReferences(ctx context.Context, files []string, scipCache *SCIPCacheManager, progress IndexProgressFunc) error {
	if scipCache == nil || len(files) == 0 {
		return nil
	}

	// Process symbols from specific files
	documents := make(map[string]*scip.SCIPDocument)
	for _, file := range files {
		absPath, err := filepath.Abs(file)
		if err != nil {
			continue
		}
		uri := utils.FilePathToURI(absPath)

		doc, err := scipCache.scipStorage.GetDocument(ctx, uri)
		if err == nil && doc != nil {
			documents[uri] = doc
		}
	}

	if len(documents) == 0 {
		common.LSPLogger.Debug("No documents found for reference indexing")
		return nil
	}

	symbols := w.collectUniqueDefinitions(documents)
	common.LSPLogger.Debug("Found %d unique symbols to process from %d files", len(symbols), len(files))

	if len(symbols) == 0 {
		return nil
	}

	// Group symbols by file
	symbolsByFile := make(map[string][]indexedSymbol)
	for _, s := range symbols {
		symbolsByFile[s.uri] = append(symbolsByFile[s.uri], s)
	}

	filesToProcess := make([]string, 0, len(symbolsByFile))
	for k := range symbolsByFile {
		filesToProcess = append(filesToProcess, k)
	}

	// Determine worker count - limit for Java projects to prevent LSP overload
	workers := computeWorkers(hasJavaInURIs(filesToProcess))

	// Process references
	var wg sync.WaitGroup
	fileChan := make(chan string, len(filesToProcess))
	progressChan := make(chan int, workers)

	// Progress reporter
	if progress != nil {
		go func() {
			processed := 0
			total := len(symbols)
			for range progressChan {
				processed++
				progress("references", processed, total, "")
			}
		}()
	}

	// Worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fileURI := range fileChan {
				fileSymbols := symbolsByFile[fileURI]
				// Process references for symbols in this file
				localRefs := make(map[string][]scip.SCIPOccurrence)
				for _, symbol := range fileSymbols {
					refs, err := w.getReferencesForSymbolInOpenFile(ctx, symbol)
					if err != nil {
						if progress != nil {
							progressChan <- 1
						}
						continue
					}
					for _, ref := range refs {
						if w.isSelfReference(ref, symbol) {
							continue
						}
						occ := w.createReferenceOccurrence(ref, symbol)
						localRefs[ref.URI] = append(localRefs[ref.URI], occ)
					}
					if progress != nil {
						progressChan <- 1
					}
				}
				// Flush per-doc to storage to bound memory
				for uri, occs := range localRefs {
					occs = dedupOccurrences(occs)
					_ = scipCache.AddOccurrences(ctx, uri, occs)
				}
			}
		}()
	}

	// Send work to workers
	for _, fileURI := range filesToProcess {
		fileChan <- fileURI
	}
	close(fileChan)

	wg.Wait()
	if progress != nil {
		close(progressChan)
		progress("references_complete", len(symbols), 0, "")
	}

	common.LSPLogger.Debug("Reference indexing complete for %d symbols", len(symbols))
	return nil
}
