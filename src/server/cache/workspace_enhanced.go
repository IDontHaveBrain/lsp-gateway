package cache

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
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
		// Silently skip "no identifier found" errors - these are expected for some positions
		if strings.Contains(err.Error(), "no identifier found") {
			return []types.Location{}, nil
		}
		// Skip invalid line/position errors from language servers
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "bad line number") || strings.Contains(lower, "line number") {
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

// parseLocationResponse removed; use lspconv.ParseLocations directly

func (w *WorkspaceIndexer) parseRange(rangeData map[string]interface{}) types.Range {
    if r, ok := lspconv.ParseRangeFromMap(rangeData); ok {
        return r
    }
    return types.Range{}
}

// clampPositionToFile ensures the given position is within the file's line bounds
func (w *WorkspaceIndexer) clampPositionToFile(uri string, pos types.Position) types.Position {
	path := utils.URIToFilePath(uri)
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

func (w *WorkspaceIndexer) detectLanguageFromURI(uri string) string {
    path := utils.URIToFilePath(uri)
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
