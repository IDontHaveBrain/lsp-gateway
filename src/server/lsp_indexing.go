package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	cachesearch "lsp-gateway/src/server/cache/search"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/server/watcher"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/filepattern"
	"lsp-gateway/src/utils/lspconv"
)

func (m *LSPManager) scheduleIndexing(method, uri, language string, params, result interface{}) {
	go func() {
		m.indexLimiter <- struct{}{}
		defer func() { <-m.indexLimiter }()
		timeout := 10 * time.Second
		switch method {
		case types.MethodWorkspaceSymbol:
			timeout = 20 * time.Second
		case types.MethodTextDocumentReferences:
			timeout = 15 * time.Second
		case types.MethodTextDocumentDocumentSymbol:
			timeout = 12 * time.Second
		case types.MethodTextDocumentDefinition, types.MethodTextDocumentCompletion, types.MethodTextDocumentHover:
			timeout = 10 * time.Second
		}
		timeout = constants.AdjustDurationForWindows(timeout, 1.5)
		idxCtx, cancel := common.CreateContext(timeout)
		defer cancel()
		m.performSCIPIndexing(idxCtx, method, uri, language, params, result)
	}()
}

func (m *LSPManager) startFileWatcher() error {
	m.watcherMu.Lock()
	defer m.watcherMu.Unlock()
	extensions := constants.GetAllSupportedExtensions()
	fw, err := watcher.NewFileWatcher(extensions, m.handleFileChanges)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := fw.AddPath(wd); err != nil {
		return fmt.Errorf("failed to add watch path: %w", err)
	}
	fw.Start()
	m.fileWatcher = fw
	common.LSPLogger.Info("File watcher started for real-time change detection")
	return nil
}

func (m *LSPManager) handleFileChanges(events []watcher.FileChangeEvent) {
	if len(events) == 0 {
		return
	}
	if m.scipCache == nil {
		return
	}
	var modifiedFiles []string
	var deletedFiles []string
	for _, event := range events {
		switch event.Operation {
		case "write", "create":
			modifiedFiles = append(modifiedFiles, event.Path)
		case "remove":
			deletedFiles = append(deletedFiles, event.Path)
		}
	}
	common.LSPLogger.Debug("File changes detected: %d modified, %d deleted", len(modifiedFiles), len(deletedFiles))
	for _, path := range deletedFiles {
		uri := utils.FilePathToURI(path)
		if err := m.scipCache.InvalidateDocument(uri); err != nil {
			common.LSPLogger.Warn("Failed to invalidate deleted document %s: %v", uri, err)
		}
	}
	if len(modifiedFiles) > 0 {
		go m.performIncrementalReindex(modifiedFiles)
	}
}

func (m *LSPManager) performIncrementalReindex(files []string) {
	m.indexLimiter <- struct{}{}
	defer func() { <-m.indexLimiter }()
	cacheManager, ok := m.scipCache.(*cache.SCIPCacheManager)
	if !ok {
		common.LSPLogger.Warn("Cache manager doesn't support incremental indexing")
		return
	}
	ctx, cancel := common.CreateContext(2 * time.Minute)
	defer cancel()
	wd, err := os.Getwd()
	if err != nil {
		common.LSPLogger.Error("Failed to get working directory: %v", err)
		return
	}
	common.LSPLogger.Info("Performing incremental reindex for %d changed files", len(files))
	if err := cacheManager.PerformIncrementalIndexing(ctx, wd, m); err != nil {
		common.LSPLogger.Error("Incremental reindexing failed: %v", err)
	} else {
		common.LSPLogger.Info("Incremental reindex completed successfully")
		if stats := cacheManager.GetIndexStats(); stats != nil {
			common.LSPLogger.Debug("Cache stats after reindex: %d symbols, %d documents", stats.SymbolCount, stats.DocumentCount)
		}
	}
}

// SymbolReferenceQuery defines parameters for searching symbol references
type SymbolReferenceQuery struct {
	Pattern     string `json:"pattern"`
	FilePattern string `json:"filePattern"`
	MaxResults  int    `json:"maxResults"`
}

// SymbolReferenceResult contains the results of a symbol reference search with occurrence metadata
type SymbolReferenceResult struct {
	References []ReferenceInfo `json:"references"`
	TotalCount int             `json:"totalCount"`
	Truncated  bool            `json:"truncated"`

	// Enhanced metadata from occurrence search
	DefinitionCount  int                    `json:"definitionCount"`
	ReadAccessCount  int                    `json:"readAccessCount"`
	WriteAccessCount int                    `json:"writeAccessCount"`
	Implementations  []ReferenceInfo        `json:"implementations,omitempty"` // Implementation references
	RelatedSymbols   []string               `json:"relatedSymbols,omitempty"`  // Related symbol IDs
	SearchMetadata   map[string]interface{} `json:"searchMetadata,omitempty"`  // Additional search metadata
}

// ReferenceInfo contains information about a single reference with occurrence details
type ReferenceInfo struct {
	FilePath   string `json:"filePath"`
	LineNumber int    `json:"lineNumber"`
	Column     int    `json:"column"`
	Text       string `json:"text,omitempty"`
	Code       string `json:"code,omitempty"`
	Context    string `json:"context,omitempty"`

	// Occurrence-based metadata
	SymbolID       string           `json:"symbolId,omitempty"`      // SCIP symbol ID
	OccurrenceRole types.SymbolRole `json:"occurrenceRole"`          // Role flags for this occurrence
	IsDefinition   bool             `json:"isDefinition"`            // This occurrence is a definition
	IsReadAccess   bool             `json:"isReadAccess"`            // This occurrence is a read
	IsWriteAccess  bool             `json:"isWriteAccess"`           // This occurrence is a write
	IsImport       bool             `json:"isImport"`                // This occurrence is an import
	IsGenerated    bool             `json:"isGenerated"`             // This occurrence is in generated code
	IsTest         bool             `json:"isTest"`                  // This occurrence is in test code
	Documentation  []string         `json:"documentation,omitempty"` // Symbol documentation at this occurrence
	Relationships  []string         `json:"relationships,omitempty"` // Related symbols
	Range          *types.Range     `json:"range,omitempty"`         // Full range of the occurrence
}

// SearchSymbolReferences searches for all references to symbols matching a pattern
func (m *LSPManager) SearchSymbolReferences(ctx context.Context, query SymbolReferenceQuery) (*SymbolReferenceResult, error) {

	if query.Pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}
	if query.FilePattern == "" {
		query.FilePattern = defaultReferenceQuery
	}
	if query.MaxResults <= 0 {
		query.MaxResults = constants.DefaultMaxResults
	}

	var references []ReferenceInfo
	var implementations []ReferenceInfo
	var relatedSymbols []string
	// Capture a definition location (from SCIP) for potential LSP fallback
	var fallbackDefRef *ReferenceInfo

	// Counters for different types of occurrences
	var definitionCount, readAccessCount, writeAccessCount int

	// Preferred: use SCIP cache manager to retrieve references directly
	if m.scipCache != nil {
		scipRefs, err := m.scipCache.SearchReferences(ctx, query.Pattern, query.FilePattern, query.MaxResults)
		if err == nil && len(scipRefs) > 0 {
			for _, r := range scipRefs {
				switch v := r.(type) {
				case cachesearch.SCIPOccurrenceInfo:
					ref := ReferenceInfo{
						FilePath:      utils.URIToFilePathCached(v.DocumentURI),
						LineNumber:    int(v.Occurrence.Range.Start.Line),
						Column:        int(v.Occurrence.Range.Start.Character),
						SymbolID:      v.Occurrence.Symbol,
						IsDefinition:  v.SymbolRoles.HasRole(types.SymbolRoleDefinition),
						IsReadAccess:  v.SymbolRoles.HasRole(types.SymbolRoleReadAccess),
						IsWriteAccess: v.SymbolRoles.HasRole(types.SymbolRoleWriteAccess),
						IsImport:      v.SymbolRoles.HasRole(types.SymbolRoleImport),
						IsGenerated:   v.SymbolRoles.HasRole(types.SymbolRoleGenerated),
						IsTest:        v.SymbolRoles.HasRole(types.SymbolRoleTest),
						Range:         &types.Range{Start: v.Occurrence.Range.Start, End: v.Occurrence.Range.End},
					}
					references = append(references, ref)
					if ref.IsDefinition {
						definitionCount++
					}
					if ref.IsReadAccess {
						readAccessCount++
					}
					if ref.IsWriteAccess {
						writeAccessCount++
					}
				case map[string]interface{}:
					if uri, ok := v["document_uri"].(string); ok {
						if occ, ok := v["occurrence"].(map[string]interface{}); ok {
							if rmap, ok := occ["range"].(map[string]interface{}); ok {
								if start, ok := rmap["start"].(map[string]interface{}); ok {
									line, _ := start["line"].(float64)
									char, _ := start["character"].(float64)
									references = append(references, ReferenceInfo{
										FilePath:     utils.URIToFilePathCached(uri),
										LineNumber:   int(line),
										Column:       int(char),
										IsReadAccess: true,
									})
									readAccessCount++
								}
							}
						}
					}
				}
			}
		}

		// Always try to get a definition for potential LSP fallback
		if len(references) == 0 || fallbackDefRef == nil {
			if defs, derr := m.scipCache.SearchDefinitions(ctx, query.Pattern, query.FilePattern, 1); derr == nil && len(defs) > 0 {
				switch d := defs[0].(type) {
				case cachesearch.SCIPOccurrenceInfo:
					fallbackDefRef = &ReferenceInfo{
						FilePath:   utils.URIToFilePathCached(d.DocumentURI),
						LineNumber: int(d.Occurrence.Range.Start.Line),
						Column:     int(d.Occurrence.Range.Start.Character),
						SymbolID:   d.Occurrence.Symbol,
					}
				case map[string]interface{}:
					if uri, ok := d["document_uri"].(string); ok {
						if occ, ok := d["occurrence"].(map[string]interface{}); ok {
							if rmap, ok := occ["range"].(map[string]interface{}); ok {
								if start, ok := rmap["start"].(map[string]interface{}); ok {
									line, _ := start["line"].(float64)
									char, _ := start["character"].(float64)
									fallbackDefRef = &ReferenceInfo{
										FilePath:   utils.URIToFilePathCached(uri),
										LineNumber: int(line),
										Column:     int(char),
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// First try occurrence-based reference search with SCIP storage
	if m.scipCache != nil {

		// Get SCIP storage for direct occurrence queries
		scipStorage := m.getScipStorage()
		if scipStorage != nil {
			// Use the pattern directly for symbol search
			symbolPattern := query.Pattern

			// Search for symbol information to get the symbol ID
			symbolInfos, err := scipStorage.SearchSymbols(ctx, symbolPattern, 10) // Get a few candidates
			if err == nil && len(symbolInfos) > 0 {
				// Process each symbol found
				for _, symbolInfo := range symbolInfos {

					// Try to get references with documents first (more efficient)
					if simpleStorage, ok := scipStorage.(*scip.SimpleSCIPStorage); ok {
						refWithDocs, refErr := simpleStorage.GetReferencesWithDocuments(ctx, symbolInfo.Symbol)
						if refErr == nil {
							for _, occWithDoc := range refWithDocs {
								refInfo := m.createReferenceFromOccurrenceWithDoc(occWithDoc, &symbolInfo)
								if refInfo != nil {
									references = append(references, *refInfo)

									// Update counters
									if refInfo.IsDefinition {
										definitionCount++
									}
									if refInfo.IsReadAccess {
										readAccessCount++
									}
									if refInfo.IsWriteAccess {
										writeAccessCount++
									}
								}
							}
						}
					} else {
						// Fallback to regular GetReferences
						refOccurrences, refErr := scipStorage.GetReferences(ctx, symbolInfo.Symbol)
						if refErr == nil {
							// Convert SCIP occurrences to ReferenceInfo
							for _, occurrence := range refOccurrences {
								refInfo := m.createReferenceFromOccurrence(ctx, scipStorage, occurrence, &symbolInfo)
								if refInfo != nil {
									references = append(references, *refInfo)

									// Update counters
									if refInfo.IsDefinition {
										definitionCount++
									}
									if refInfo.IsReadAccess {
										readAccessCount++
									}
									if refInfo.IsWriteAccess {
										writeAccessCount++
									}
								}
							}
						}
					}

					// Capture a definition occurrence for LSP fallback (do not include as a reference result)
					defOccurrences, defErr := scipStorage.GetDefinitions(ctx, symbolInfo.Symbol)
					if defErr == nil && len(defOccurrences) > 0 && fallbackDefRef == nil {
						defOccurrence := defOccurrences[0]
						defRefInfo := m.createReferenceFromOccurrence(ctx, scipStorage, defOccurrence, &symbolInfo)
						if defRefInfo != nil {
							fallbackDefRef = defRefInfo
						}
					}
				}
			}
		} else {

			// Fallback to cache query approach
			symbolPattern := query.Pattern

			// Use workspace query to find all occurrences
			indexQuery := &cache.IndexQuery{
				Type:   "workspace",
				Symbol: symbolPattern,
				Filters: map[string]interface{}{
					"filePattern": query.FilePattern,
				},
			}

			if indexResult, err := m.scipCache.QueryIndex(ctx, indexQuery); err == nil && indexResult != nil {

				// Process each result as a potential reference with legacy format
				for _, result := range indexResult.Results {
					var symbolInfo *types.SymbolInformation

					// Extract symbol information from various result types
					if scipSymbol, ok := result.(*cache.SCIPSymbol); ok {
						symbolInfo = &scipSymbol.SymbolInfo
					} else if si, ok := result.(types.SymbolInformation); ok {
						symbolInfo = &si
					} else if resultMap, ok := result.(map[string]interface{}); ok {
						if symbolInfoData, hasSymbolInfo := resultMap["symbol_info"]; hasSymbolInfo {
							if si, ok := symbolInfoData.(types.SymbolInformation); ok {
								symbolInfo = &si
							}
						}
					}

					if symbolInfo != nil && symbolInfo.Location.URI != "" {
						// Check if name matches (for exact match filtering)
						// Pattern matching is already done by the cache query
						refInfo := m.createLegacyReferenceInfo(*symbolInfo)

						// Check file pattern
						if m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
							references = append(references, refInfo)
						}
					}
				}
			}

			// Also try to get references from the reference index directly
			// Try various key formats that might be used
			symbolKeys := []string{
				fmt.Sprintf("go:%s", query.Pattern),
				fmt.Sprintf("python:%s", query.Pattern),
				fmt.Sprintf("javascript:%s", query.Pattern),
				fmt.Sprintf("typescript:%s", query.Pattern),
				fmt.Sprintf("java:%s", query.Pattern),
			}

			for _, symbolKey := range symbolKeys {
				refQuery := &cache.IndexQuery{
					Type:     "references",
					Symbol:   query.Pattern,
					Language: strings.Split(symbolKey, ":")[0],
				}
				if refResult, err := m.scipCache.QueryIndex(ctx, refQuery); err == nil && refResult != nil {
					for _, ref := range refResult.Results {
						if loc, ok := ref.(types.Location); ok {
							refInfo := m.locationToReferenceInfo(loc)
							if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
								// Check for duplicates before adding
								isDuplicate := false
								for _, existing := range references {
									if existing.FilePath == refInfo.FilePath &&
										existing.LineNumber == refInfo.LineNumber &&
										existing.Column == refInfo.Column {
										isDuplicate = true
										break
									}
								}
								if !isDuplicate {
									references = append(references, *refInfo)
								}
							}
						}
					}
				}
			}
		}
	}

	// If no references found, use a more robust search approach
	if len(references) == 0 && m.scipCache != nil {
		// Use LSP workspace/symbol to find all symbols with this name across the workspace
		wsParams := map[string]interface{}{
			"query": query.Pattern,
		}

		wsResult, wsErr := m.ProcessRequest(ctx, types.MethodWorkspaceSymbol, wsParams)
		if wsErr == nil && wsResult != nil {
			symbolInfos := lspconv.ParseWorkspaceSymbols(wsResult)

			// For each symbol found, try to get references from its location
			// Limit the number of LSP requests to avoid overwhelming the server
			maxLSPRequests := 5
			lspRequestCount := 0

			for _, si := range symbolInfos {
				// Stop making LSP requests if we've hit the limit
				if lspRequestCount >= maxLSPRequests {
					break
				}

				if si.Location.URI != "" {
					filePath := utils.URIToFilePathCached(si.Location.URI)
					if filePath != "" && m.matchesFilePattern(filePath, query.FilePattern) {
						refInfo := ReferenceInfo{
							FilePath:     filePath,
							LineNumber:   int(si.Location.Range.Start.Line),
							Column:       int(si.Location.Range.Start.Character),
							IsDefinition: false,
							IsReadAccess: true,
						}
						isDuplicate := false
						for _, existing := range references {
							if existing.FilePath == refInfo.FilePath && existing.LineNumber == refInfo.LineNumber && existing.Column == refInfo.Column {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							references = append(references, refInfo)
						}
					}
				}

				if lspRequestCount < maxLSPRequests && si.Location.URI != "" {
					lspRequestCount++
					refParams := map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": si.Location.URI,
						},
						"position": map[string]interface{}{
							"line":      int(si.Location.Range.Start.Line),
							"character": int(si.Location.Range.Start.Character),
						},
						"context": map[string]interface{}{
							"includeDeclaration": true,
						},
					}
					if refResult, refErr := m.ProcessRequest(ctx, types.MethodTextDocumentReferences, refParams); refErr == nil && refResult != nil {
						locs := lspconv.ParseLocations(refResult)
						for _, loc := range locs {
							refInfo := m.locationToReferenceInfo(loc)
							if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
								refInfo.IsReadAccess = true
								references = append(references, *refInfo)
								readAccessCount++
							}
						}
					}
				}

				if si.Name == query.Pattern && si.Location.URI != "" && fallbackDefRef == nil {
					fallbackDefRef = &ReferenceInfo{
						FilePath:     utils.URIToFilePathCached(si.Location.URI),
						LineNumber:   int(si.Location.Range.Start.Line),
						Column:       int(si.Location.Range.Start.Character),
						IsDefinition: true,
					}
					common.LSPLogger.Debug("[SearchSymbolReferences] Found definition via workspace symbol: %s:%d:%d", fallbackDefRef.FilePath, fallbackDefRef.LineNumber, fallbackDefRef.Column)
				}

				continue
			}
		}
	}

	// If no references found OR we only have definitions (no actual references), fall back to LSP
	onlyDefinitions := true
	for _, ref := range references {
		if !ref.IsDefinition {
			onlyDefinitions = false
			break
		}
	}

	// Check if we need LSP fallback

	if len(references) == 0 || (onlyDefinitions && definitionCount > 0) {
		// Ensure we don't return definitions-only results
		references = []ReferenceInfo{}
		definitionCount = 0
		readAccessCount = 0
		writeAccessCount = 0

		// If we don't already have a def position, try to fetch one now from SCIP
		if fallbackDefRef == nil && m.scipCache != nil {
			if scipStorage := m.getScipStorage(); scipStorage != nil {
				if syms, err := scipStorage.SearchSymbols(ctx, query.Pattern, 1); err == nil && len(syms) > 0 {
					if defs, derr := scipStorage.GetDefinitions(ctx, syms[0].Symbol); derr == nil && len(defs) > 0 {
						if defRef := m.createReferenceFromOccurrence(ctx, scipStorage, defs[0], &syms[0]); defRef != nil {
							fallbackDefRef = defRef
						}
					}
				}
			}
		}

		// If no cache available, try to find definition via LSP workspace symbols
		if fallbackDefRef == nil && m.scipCache == nil {
			common.LSPLogger.Debug("[SearchSymbolReferences] No cache available, trying workspace symbol search for pattern: %s", query.Pattern)

			// Use workspace/symbol LSP request to find symbols matching the pattern
			wsParams := map[string]interface{}{
				"query": query.Pattern,
			}

			wsResult, wsErr := m.ProcessRequest(ctx, types.MethodWorkspaceSymbol, wsParams)
			if wsErr == nil && wsResult != nil {
				for _, si := range lspconv.ParseWorkspaceSymbols(wsResult) {
					if si.Name == query.Pattern && si.Location.URI != "" {
						fallbackDefRef = &ReferenceInfo{
							FilePath:     utils.URIToFilePathCached(si.Location.URI),
							LineNumber:   int(si.Location.Range.Start.Line),
							Column:       int(si.Location.Range.Start.Character),
							IsDefinition: true,
						}
						common.LSPLogger.Debug("[SearchSymbolReferences] Found definition via workspace symbol: %s:%d:%d", fallbackDefRef.FilePath, fallbackDefRef.LineNumber, fallbackDefRef.Column)
						break
					}
				}
			}
		}

		if fallbackDefRef != nil {
			// Use SCIP def position to ask LSP for references
			uri := utils.FilePathToURI(fallbackDefRef.FilePath)

			// Do not send didOpen/didClose here; LSPManager ensures document open automatically

			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
				"position": map[string]interface{}{
					"line":      fallbackDefRef.LineNumber,
					"character": fallbackDefRef.Column,
				},
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			}

			// Send references request and parse via shared converter
			result, err := m.ProcessRequest(ctx, types.MethodTextDocumentReferences, params)
			if err == nil && result != nil {
				locs := lspconv.ParseLocations(result)
				for _, loc := range locs {
					refInfo := m.locationToReferenceInfo(loc)
					if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
						refInfo.IsReadAccess = true
						references = append(references, *refInfo)
						readAccessCount++
					}
				}
			}
		}
	}

	// Remove duplicates (can happen when combining SCIP and LSP results)
	uniqueReferences := []ReferenceInfo{}
	seen := make(map[string]bool)
	for _, ref := range references {
		key := fmt.Sprintf("%s:%d:%d", ref.FilePath, ref.LineNumber, ref.Column)
		if !seen[key] {
			seen[key] = true
			uniqueReferences = append(uniqueReferences, ref)
		}
	}
	references = uniqueReferences

	// Apply max results limit
	truncated := false
	if len(references) > query.MaxResults {
		references = references[:query.MaxResults]
		truncated = true
	}

	// Sort references by relevance (definitions first, then by occurrence role)
	sort.Slice(references, func(i, j int) bool {
		return m.compareReferenceRelevance(references[i], references[j])
	})

	totalCount := len(references)

	return &SymbolReferenceResult{
		References:       references,
		TotalCount:       totalCount,
		Truncated:        truncated,
		DefinitionCount:  definitionCount,
		ReadAccessCount:  readAccessCount,
		WriteAccessCount: writeAccessCount,
		Implementations:  implementations,
		RelatedSymbols:   relatedSymbols,
	}, nil
}

// parseReferenceLocation parses a location map into ReferenceInfo
// parseReferenceLocation removed in favor of lspconv.ParseLocations and locationToReferenceInfo

// locationToReferenceInfo converts a types.Location to ReferenceInfo
func (m *LSPManager) locationToReferenceInfo(loc types.Location) *ReferenceInfo {
	refInfo := &ReferenceInfo{
		FilePath:   utils.URIToFilePathCached(loc.URI),
		LineNumber: int(loc.Range.Start.Line),
		Column:     int(loc.Range.Start.Character),
	}

	return refInfo
}

// matchesFilePattern checks if a file path matches the given pattern
func (m *LSPManager) matchesFilePattern(filePath, pattern string) bool {
	return filepattern.Match(filePath, pattern)
}

// readFullLine reads a complete line from a file
// readFullLine helper was removed as unused

// Helper methods for occurrence-based reference search

// createReferenceFromOccurrenceWithDoc creates ReferenceInfo from a SCIP occurrence with document URI
func (m *LSPManager) createReferenceFromOccurrenceWithDoc(occWithDoc scip.OccurrenceWithDocument, symbolInfo *scip.SCIPSymbolInformation) *ReferenceInfo {
	filePath := utils.URIToFilePathCached(occWithDoc.DocumentURI)

	// Convert SCIP range to LSP-compatible format
	refInfo := &ReferenceInfo{
		FilePath:       filePath,
		LineNumber:     int(occWithDoc.Range.Start.Line),
		Column:         int(occWithDoc.Range.Start.Character),
		SymbolID:       occWithDoc.Symbol,
		OccurrenceRole: occWithDoc.SymbolRoles,
		IsDefinition:   occWithDoc.SymbolRoles.HasRole(types.SymbolRoleDefinition),
		IsReadAccess:   occWithDoc.SymbolRoles.HasRole(types.SymbolRoleReadAccess),
		IsWriteAccess:  occWithDoc.SymbolRoles.HasRole(types.SymbolRoleWriteAccess),
		IsImport:       occWithDoc.SymbolRoles.HasRole(types.SymbolRoleImport),
		IsGenerated:    occWithDoc.SymbolRoles.HasRole(types.SymbolRoleGenerated),
		IsTest:         occWithDoc.SymbolRoles.HasRole(types.SymbolRoleTest),
		Documentation:  symbolInfo.Documentation,
		Range: &types.Range{
			Start: types.Position{
				Line:      occWithDoc.Range.Start.Line,
				Character: occWithDoc.Range.Start.Character,
			},
			End: types.Position{
				Line:      occWithDoc.Range.End.Line,
				Character: occWithDoc.Range.End.Character,
			},
		},
	}

	// Get related symbols from relationships
	if symbolInfo.Relationships != nil {
		for _, rel := range symbolInfo.Relationships {
			refInfo.Relationships = append(refInfo.Relationships, rel.Symbol)
		}
	}

	return refInfo
}

// createReferenceFromOccurrence creates a ReferenceInfo from a SCIP occurrence
// NOTE: This function is inefficient as it searches through all documents.
// Consider using GetReferencesWithDocuments() or GetDefinitionsWithDocuments()
// which provide occurrences with their document URIs directly.
func (m *LSPManager) createReferenceFromOccurrence(ctx context.Context, scipStorage scip.SCIPDocumentStorage, occurrence scip.SCIPOccurrence, symbolInfo *scip.SCIPSymbolInformation) *ReferenceInfo {
	// NOTE: The occurrence should ideally contain document URI information
	// For now, we need to find which document contains this occurrence
	// This is inefficient but necessary with the current SCIP storage interface

	// Find the document URI using optimized common function
	uri, err := scip.GetDocumentURIFromOccurrence(scipStorage, &occurrence)
	filePath := ""
	if err == nil && uri != "" {
		filePath = utils.URIToFilePathCached(uri)
	}

	// If we still don't have a file path, skip this occurrence
	if filePath == "" {
		// This can happen if the occurrence doesn't have associated document info
		// Log for debugging
		common.LSPLogger.Debug("Could not find document for occurrence with symbol: %s", occurrence.Symbol)
		return nil
	}

	// Convert SCIP range to LSP-compatible format
	refInfo := &ReferenceInfo{
		FilePath:       filePath,
		LineNumber:     int(occurrence.Range.Start.Line),
		Column:         int(occurrence.Range.Start.Character),
		SymbolID:       occurrence.Symbol,
		OccurrenceRole: occurrence.SymbolRoles,
		IsDefinition:   occurrence.SymbolRoles.HasRole(types.SymbolRoleDefinition),
		IsReadAccess:   occurrence.SymbolRoles.HasRole(types.SymbolRoleReadAccess),
		IsWriteAccess:  occurrence.SymbolRoles.HasRole(types.SymbolRoleWriteAccess),
		IsImport:       occurrence.SymbolRoles.HasRole(types.SymbolRoleImport),
		IsGenerated:    occurrence.SymbolRoles.HasRole(types.SymbolRoleGenerated),
		IsTest:         occurrence.SymbolRoles.HasRole(types.SymbolRoleTest),
		Documentation:  symbolInfo.Documentation,
		Range: &types.Range{
			Start: types.Position{
				Line:      occurrence.Range.Start.Line,
				Character: occurrence.Range.Start.Character,
			},
			End: types.Position{
				Line:      occurrence.Range.End.Line,
				Character: occurrence.Range.End.Character,
			},
		},
	}

	// Get related symbols from relationships
	if symbolInfo.Relationships != nil {
		for _, rel := range symbolInfo.Relationships {
			refInfo.Relationships = append(refInfo.Relationships, rel.Symbol)
		}
	}

	return refInfo
}

// occurrencesMatch checks if two occurrences are the same

// createLegacyReferenceInfo creates ReferenceInfo from LSP SymbolInformation (fallback)
func (m *LSPManager) createLegacyReferenceInfo(symbolInfo types.SymbolInformation) ReferenceInfo {
	refInfo := ReferenceInfo{
		FilePath:   utils.URIToFilePathCached(symbolInfo.Location.URI),
		LineNumber: int(symbolInfo.Location.Range.Start.Line),
		Column:     int(symbolInfo.Location.Range.Start.Character),
		SymbolID:   fmt.Sprintf("lsp:%s", symbolInfo.Name), // Legacy format
		Range: &types.Range{
			Start: types.Position{
				Line:      symbolInfo.Location.Range.Start.Line,
				Character: symbolInfo.Location.Range.Start.Character,
			},
			End: types.Position{
				Line:      symbolInfo.Location.Range.End.Line,
				Character: symbolInfo.Location.Range.End.Character,
			},
		},
		// Default role assumptions for LSP symbols
		IsReadAccess: true, // Assume references are read access by default
	}

	return refInfo
}

// compareReferenceRelevance compares two references for sorting by relevance
func (m *LSPManager) compareReferenceRelevance(a, b ReferenceInfo) bool {
	// Definitions get highest priority
	if a.IsDefinition && !b.IsDefinition {
		return true
	}
	if !a.IsDefinition && b.IsDefinition {
		return false
	}

	// Write accesses get higher priority than read accesses
	if a.IsWriteAccess && !b.IsWriteAccess {
		return true
	}
	if !a.IsWriteAccess && b.IsWriteAccess {
		return false
	}

	// Non-generated code gets priority over generated
	if !a.IsGenerated && b.IsGenerated {
		return true
	}
	if a.IsGenerated && !b.IsGenerated {
		return false
	}

	// Non-test code gets priority over test code
	if !a.IsTest && b.IsTest {
		return true
	}
	if a.IsTest && !b.IsTest {
		return false
	}

	// Sort by file path, then line number
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	return a.LineNumber < b.LineNumber
}

// performSCIPIndexing performs SCIP indexing based on LSP method and response using occurrence-centric approach
func (m *LSPManager) performSCIPIndexing(ctx context.Context, method, uri, language string, params, result interface{}) {
	if m.scipCache == nil {
		return
	}

	// Only index for specific methods that provide useful data
	switch method {
	case types.MethodTextDocumentDocumentSymbol:
		m.indexDocumentSymbolsAsOccurrences(ctx, uri, language, result)
	case types.MethodTextDocumentDefinition:
		m.indexDefinitionsAsOccurrences(ctx, uri, language, params, result)
	case types.MethodTextDocumentReferences:
		m.indexReferencesAsOccurrences(ctx, uri, language, params, result)
	case types.MethodWorkspaceSymbol:
		m.indexWorkspaceSymbolsAsOccurrences(ctx, language, result)
	}
}

// indexDocumentSymbolsAsOccurrences indexes document symbols as SCIP occurrences with definition roles
func (m *LSPManager) indexDocumentSymbolsAsOccurrences(ctx context.Context, uri, language string, result interface{}) {
	common.LSPLogger.Debug("indexDocumentSymbolsAsOccurrences called for uri=%s, language=%s", uri, language)

	// Parse document symbols from various response formats
	symbols := lspconv.ParseDocumentSymbolsToSymbolInformation(result, uri)
	common.LSPLogger.Debug("Parsed %d symbols from document %s", len(symbols), uri)

	if len(symbols) == 0 {
		common.LSPLogger.Debug("No symbols found for %s, returning early", uri)
		return
	}

	// Expand symbol ranges if they appear to be single-line
	symbols = m.expandSymbolRanges(ctx, symbols, uri, language)

	// Convert symbols to SCIP occurrences with definition roles
	occurrences := make([]scip.SCIPOccurrence, 0, len(symbols))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(symbols))

	for _, symbol := range symbols {
		// Skip builtin types that don't have actual definitions in the codebase
		if m.isBuiltinType(symbol.Name) {
			continue
		}

		// Use SelectionRange start when available to generate stable IDs
		symbolPosition := symbol.Location.Range.Start
		if symbol.SelectionRange != nil {
			symbolPosition = symbol.SelectionRange.Start
		}

		// Generate SCIP symbol ID with precise position
		symbolID := m.generateSymbolID(language, uri, symbol.Name, symbol.Kind, &symbolPosition)

		// Create SCIP occurrence with definition role
		// Range should be the full symbol range; SelectionRange holds the precise identifier span
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{Line: symbol.Location.Range.Start.Line, Character: symbol.Location.Range.Start.Character},
				End:   types.Position{Line: symbol.Location.Range.End.Line, Character: symbol.Location.Range.End.Character},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleDefinition,
			SyntaxKind:  m.mapLSPSymbolKindToSyntaxKind(symbol.Kind),
		}
		if symbol.SelectionRange != nil {
			sel := *symbol.SelectionRange
			occurrence.SelectionRange = &types.Range{
				Start: types.Position{Line: sel.Start.Line, Character: sel.Start.Character},
				End:   types.Position{Line: sel.End.Line, Character: sel.End.Character},
			}
		}

		occurrences = append(occurrences, occurrence)

		// Create symbol information with enhanced metadata using precise position
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:         symbolID,
			DisplayName:    symbol.Name,
			Kind:           m.mapLSPSymbolKindToSCIPKind(symbol.Kind),
			Documentation:  nil, // Skip documentation to avoid hover overhead during indexing
			Relationships:  m.getSymbolRelationships(ctx, uri, symbol.Name, symbol.Kind),
			Range:          symbol.Location.Range,
			SelectionRange: symbol.SelectionRange,
		}
		symbolInfos = append(symbolInfos, symbolInfo)
	}

	// Store as SCIP document
	common.LSPLogger.Debug("Storing %d occurrences and %d symbol infos for %s", len(occurrences), len(symbolInfos), uri)
	// Log first few symbols for debugging
	for i, info := range symbolInfos {
		if i < 3 {
			common.LSPLogger.Debug("  Symbol %d: DisplayName=%s, Symbol=%s, Kind=%v", i, info.DisplayName, info.Symbol, info.Kind)
		}
	}
	m.storeDocumentOccurrences(ctx, uri, language, occurrences, symbolInfos)
	common.LSPLogger.Debug("Successfully stored document occurrences for %s", uri)

}

// indexDefinitionsAsOccurrences indexes definition results as SCIP occurrences with definition roles
func (m *LSPManager) indexDefinitionsAsOccurrences(ctx context.Context, uri, language string, params, result interface{}) {

	// Parse definition result - can be Location | Location[] | LocationLink[]
	locations := m.parseLocationResult(result)
	if len(locations) == 0 {
		return
	}

	// Extract position from params to determine the symbol being defined
	_, symbolName := m.extractPositionAndSymbolFromParams(params)
	// Skip hover fallback to avoid overhead during indexing
	if symbolName == "" && len(locations) > 0 {
		symbolName = m.getIdentifierFromLocation(ctx, locations[0])
	}
	if symbolName == "" {
		return
	}

	// Create SCIP occurrences for each definition location
	occurrences := make([]scip.SCIPOccurrence, 0, len(locations))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(locations))

	for _, location := range locations {
		// Generate SCIP symbol ID (use definition start position)
		startPos := types.Position{Line: location.Range.Start.Line, Character: location.Range.Start.Character}
		symbolID := m.generateSymbolID(language, location.URI, symbolName, types.Function, &startPos)

		// Create SCIP occurrence with definition role
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      location.Range.Start.Line,
					Character: location.Range.Start.Character,
				},
				End: types.Position{
					Line:      location.Range.End.Line,
					Character: location.Range.End.Character,
				},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleDefinition,
			SyntaxKind:  types.SyntaxKindIdentifierFunctionDefinition,
		}
		occurrences = append(occurrences, occurrence)

		// Create symbol information with enhanced metadata
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:        symbolID,
			DisplayName:   symbolName,
			Kind:          scip.SCIPSymbolKindFunction, // Default to function, can be refined
			Documentation: nil,                         // Skip documentation to avoid hover overhead during indexing
			Relationships: m.getSymbolRelationships(ctx, location.URI, symbolName, types.Function),
		}
		symbolInfos = append(symbolInfos, symbolInfo)
	}

	// Group and store occurrences by document URI
	documentGroups := m.groupReferencesByDocument(occurrences, locations)
	for uri, groupedOccurrences := range documentGroups {
		m.storeDocumentOccurrences(ctx, uri, language, groupedOccurrences, symbolInfos)
	}

}

// indexReferencesAsOccurrences indexes reference results as SCIP occurrences with reference roles
func (m *LSPManager) indexReferencesAsOccurrences(ctx context.Context, uri, language string, params, result interface{}) {
	locations := m.parseLocationResult(result)
	if len(locations) == 0 {
		return
	}

	// Extract position and resolve a stable symbol name
	position, symbolName := m.extractPositionAndSymbolFromParams(params)
	// Skip hover fallback to avoid overhead during indexing
	if symbolName == "" && len(locations) > 0 {
		symbolName = m.getIdentifierFromLocation(ctx, locations[0])
	}
	if symbolName == "" {
		return
	}

	// Try to reuse an existing SCIP symbol ID from storage to ensure consistency with definitions
	stableSymbolID := ""
	if m.scipCache != nil {
		if scipStorage := m.scipCache.GetSCIPStorage(); scipStorage != nil {
			if infos, err := scipStorage.SearchSymbols(ctx, symbolName, 10); err == nil && len(infos) > 0 {
				// Prefer a symbol whose definition resides in the same document as 'uri'
				for _, info := range infos {
					defOccs, _ := scipStorage.GetDefinitions(ctx, info.Symbol)
					if len(defOccs) == 0 {
						continue
					}
					// Find the document URI for the first definition occurrence
					defURI := m.findDocumentURIForOccurrence(ctx, scipStorage, defOccs[0])
					if defURI == uri {
						stableSymbolID = info.Symbol
						break
					}
				}
				if stableSymbolID == "" {
					stableSymbolID = infos[0].Symbol
				}
			}
		}
	}
	if stableSymbolID == "" {
		// Fallback: generate an ID using position for stability
		stableSymbolID = m.generateSymbolID(language, uri, symbolName, types.Function, &position)
	}

	occurrences := make([]scip.SCIPOccurrence, 0, len(locations))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, 1)

	for _, location := range locations {
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{Line: location.Range.Start.Line, Character: location.Range.Start.Character},
				End:   types.Position{Line: location.Range.End.Line, Character: location.Range.End.Character},
			},
			Symbol:      stableSymbolID,
			SymbolRoles: types.SymbolRoleReadAccess,
			SyntaxKind:  types.SyntaxKindIdentifierFunction,
		}
		occurrences = append(occurrences, occurrence)
	}

	// Create symbol info once using the document of the definition (uri)
	if len(occurrences) > 0 {
		first := locations[0]
		si := scip.SCIPSymbolInformation{
			Symbol:        stableSymbolID,
			DisplayName:   symbolName,
			Kind:          scip.SCIPSymbolKindFunction,
			Documentation: nil, // Skip documentation to avoid hover overhead during indexing
			Relationships: m.getSymbolRelationships(ctx, uri, symbolName, types.Function),
			Range:         types.Range{Start: first.Range.Start, End: first.Range.End},
		}
		symbolInfos = append(symbolInfos, si)
	}

	documentGroups := m.groupReferencesByDocument(occurrences, locations)
	for docURI, groupedOccurrences := range documentGroups {
		m.storeDocumentOccurrences(ctx, docURI, language, groupedOccurrences, symbolInfos)
	}
}

// indexWorkspaceSymbolsAsOccurrences indexes workspace symbols as SCIP occurrences with definition roles
func (m *LSPManager) indexWorkspaceSymbolsAsOccurrences(ctx context.Context, language string, result interface{}) {

	symbols := lspconv.ParseWorkspaceSymbols(result)

	if len(symbols) == 0 {
		return
	}

	// Group symbols by URI for batch processing
	symbolsByURI := make(map[string][]types.SymbolInformation)
	for _, symbol := range symbols {
		if symbol.Location.URI != "" {
			symbolsByURI[symbol.Location.URI] = append(symbolsByURI[symbol.Location.URI], symbol)
		}
	}

	// Convert workspace symbols to SCIP occurrences for each document
	totalIndexed := 0
	for uri, uriSymbols := range symbolsByURI {
		// Expand symbol ranges to get full definitions
		uriSymbols = m.expandSymbolRanges(ctx, uriSymbols, uri, language)

		// Convert symbols to SCIP occurrences with definition roles
		occurrences := make([]scip.SCIPOccurrence, 0, len(uriSymbols))
		symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(uriSymbols))

		for _, symbol := range uriSymbols {
			// Skip builtin types that don't have actual definitions in the codebase
			if m.isBuiltinType(symbol.Name) {
				continue
			}

			// Generate SCIP symbol ID with precise position
			symbolPosition := symbol.Location.Range.Start
			if symbol.SelectionRange != nil {
				symbolPosition = symbol.SelectionRange.Start
			}
			symbolID := m.generateSymbolID(language, uri, symbol.Name, symbol.Kind, &symbolPosition)

			// Always use full symbol range for occurrence; carry SelectionRange separately
			symbolPosition = symbol.Location.Range.Start
			occurrenceRange := symbol.Location.Range
			if symbol.SelectionRange != nil {
				symbolPosition = symbol.SelectionRange.Start
			}

			// Create SCIP occurrence with definition role using full range
			occurrence := scip.SCIPOccurrence{
				Range: types.Range{
					Start: types.Position{
						Line:      occurrenceRange.Start.Line,
						Character: occurrenceRange.Start.Character,
					},
					End: types.Position{
						Line:      occurrenceRange.End.Line,
						Character: occurrenceRange.End.Character,
					},
				},
				Symbol:      symbolID,
				SymbolRoles: types.SymbolRoleDefinition,
				SyntaxKind:  m.mapLSPSymbolKindToSyntaxKind(symbol.Kind),
			}
			if symbol.SelectionRange != nil {
				sel := *symbol.SelectionRange
				occurrence.SelectionRange = &types.Range{
					Start: types.Position{Line: sel.Start.Line, Character: sel.Start.Character},
					End:   types.Position{Line: sel.End.Line, Character: sel.End.Character},
				}
			}
			occurrences = append(occurrences, occurrence)

			// Create symbol information with enhanced metadata using precise position
			symbolInfo := scip.SCIPSymbolInformation{
				Symbol:         symbolID,
				DisplayName:    symbol.Name,
				Kind:           m.mapLSPSymbolKindToSCIPKind(symbol.Kind),
				Documentation:  nil, // Skip documentation to avoid hover overhead during indexing
				Relationships:  m.getSymbolRelationships(ctx, uri, symbol.Name, symbol.Kind),
				Range:          symbol.Location.Range,
				SelectionRange: symbol.SelectionRange,
			}
			symbolInfos = append(symbolInfos, symbolInfo)
		}

		// Store as SCIP document
		m.storeDocumentOccurrences(ctx, uri, language, occurrences, symbolInfos)
		totalIndexed += len(occurrences)
	}

}

// Helper methods for SCIP symbol generation and occurrence management

// expandSymbolRanges expands single-line symbol ranges to their full extent
// This is necessary because many LSP servers return only the symbol name range
// rather than the full definition range for types.MethodWorkspaceSymbol results
func (m *LSPManager) expandSymbolRanges(ctx context.Context, symbols []types.SymbolInformation, uri, language string) []types.SymbolInformation {
	// Group symbols that need expansion
	needsExpansion := false
	for _, symbol := range symbols {
		if symbol.Location.Range.Start.Line == symbol.Location.Range.End.Line {
			needsExpansion = true
			break
		}
	}

	if !needsExpansion {
		return symbols
	}

	// Try to get full ranges via textDocument/documentSymbol
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	docSymbolResult, err := m.ProcessRequest(ctx, types.MethodTextDocumentDocumentSymbol, params)
	if err != nil || docSymbolResult == nil {
		// If we can't get document symbols, return original symbols
		return symbols
	}

	// Parse the document symbols to get full ranges
	fullRangeSymbols := lspconv.ParseDocumentSymbols(docSymbolResult)

	// Create a map of symbol names to their full ranges (multiple per name)
	fullRangeMap := make(map[string][]types.Range)
	m.collectFullRanges(fullRangeSymbols, fullRangeMap)

	// Update symbols with full ranges
	expandedSymbols := make([]types.SymbolInformation, len(symbols))
	for i, symbol := range symbols {
		expandedSymbols[i] = symbol
		if ranges, exists := fullRangeMap[symbol.Name]; exists {
			orig := symbol.Location.Range
			best := orig
			for _, fr := range ranges {
				if (fr.Start.Line < orig.Start.Line || (fr.Start.Line == orig.Start.Line && fr.Start.Character <= orig.Start.Character)) &&
					(fr.End.Line > orig.End.Line || (fr.End.Line == orig.End.Line && fr.End.Character >= orig.End.Character)) {
					if fr.End.Line > best.End.Line || (fr.End.Line == best.End.Line && fr.End.Character > best.End.Character) {
						best = fr
					}
				}
			}
			expandedSymbols[i].Location.Range = best
		}
	}

	return expandedSymbols
}

// collectFullRanges recursively collects full ranges from DocumentSymbol hierarchy
func (m *LSPManager) collectFullRanges(symbols []*lsp.DocumentSymbol, rangeMap map[string][]types.Range) {
	for _, symbol := range symbols {
		key := symbol.Name
		rangeMap[key] = append(rangeMap[key], symbol.Range)

		// Recursively process children
		if len(symbol.Children) > 0 {
			m.collectFullRanges(symbol.Children, rangeMap)
		}
	}
}

// parseSymbols parses document symbols from various response formats
// parseSymbols removed; use lspconv.ParseDocumentSymbolsToSymbolInformation

// parseLocationResult parses location results from various response formats
func (m *LSPManager) parseLocationResult(result interface{}) []types.Location {
	return lspconv.ParseLocations(result)
}

// extractPositionAndSymbolFromParams extracts position and symbol name from LSP params
func (m *LSPManager) extractPositionAndSymbolFromParams(params interface{}) (types.Position, string) {
	var position types.Position
	var symbolName string

	// Try to extract position from textDocument/position params
	if paramsMap, ok := params.(map[string]interface{}); ok {
		if posMap, ok := paramsMap["position"].(map[string]interface{}); ok {
			if line, ok := posMap["line"].(float64); ok {
				position.Line = int32(line)
			}
			if char, ok := posMap["character"].(float64); ok {
				position.Character = int32(char)
			}
		}
		// TODO: Extract symbol name from context if available
	}

	return position, symbolName
}

// generateSymbolID generates a unified SCIP symbol ID from symbol information
func (m *LSPManager) generateSymbolID(language, uri, symbolName string, symbolKind types.SymbolKind, pos *types.Position) string {
	workspaceRoot := m.findWorkspaceRoot(uri)
	filePath := utils.URIToFilePathCached(uri)
	relPath, _ := filepath.Rel(workspaceRoot, filePath)
	if relPath == "" || strings.HasPrefix(relPath, "..") {
		relPath = filepath.Base(filePath)
	}

	// Include position when available to stabilize ID for same-named symbols
	if pos != nil {
		descriptor := fmt.Sprintf("`%s`:%d:%d/%s%s", relPath, pos.Line, pos.Character, symbolName, m.getSymbolSuffix(symbolKind))
		packageName := "local"
		version := "0.0.0"
		if m.projectInfo != nil {
			packageName = m.projectInfo.Name
			version = m.projectInfo.Version
		}
		return fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, descriptor)
	}
	// Fallback: path + name only
	descriptor := fmt.Sprintf("`%s`/%s%s", relPath, symbolName, m.getSymbolSuffix(symbolKind))

	// Use consistent package info
	packageName := "local"
	version := "0.0.0"
	if m.projectInfo != nil {
		packageName = m.projectInfo.Name
		version = m.projectInfo.Version
	}

	return fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, descriptor)
}

// getSymbolSuffix returns the appropriate suffix for a symbol kind
func (m *LSPManager) getSymbolSuffix(kind types.SymbolKind) string {
	switch kind {
	case types.Function, types.Method:
		return "()"
	case types.Class, types.Interface:
		return "#"
	default:
		return "."
	}
}

// findWorkspaceRoot finds the workspace root for a given URI
func (m *LSPManager) findWorkspaceRoot(uri string) string {
	filePath := utils.URIToFilePathCached(uri)
	return m.findWorkspaceRootFromPath(filePath)
}

// findWorkspaceRootFromPath finds the workspace root by searching up the directory tree for project markers
func (m *LSPManager) findWorkspaceRootFromPath(filePath string) string {
	// Get the directory containing the file
	dir := filepath.Dir(filePath)

	// Project marker files for any language
	projectMarkers := []string{
		"go.mod", "go.work",
		"package.json", "yarn.lock", "package-lock.json", "tsconfig.json",
		"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", ".python-version",
		"pom.xml", "build.gradle", "build.gradle.kts", "gradlew", "mvnw",
		".git", ".gitignore", ".vscode", ".idea",
	}

	// Search up the directory tree
	currentDir := dir
	for {
		// Check if any project marker exists in current directory
		for _, marker := range projectMarkers {
			markerPath := filepath.Join(currentDir, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return currentDir
			}
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached the root directory, no project marker found
			break
		}
		currentDir = parentDir
	}

	// Fallback to the original directory if no project marker found
	return dir
}

// mapLSPSymbolKindToSCIPKind maps LSP symbol kinds to SCIP symbol kinds
func (m *LSPManager) mapLSPSymbolKindToSCIPKind(lspKind types.SymbolKind) scip.SCIPSymbolKind {
	switch lspKind {
	case types.File:
		return scip.SCIPSymbolKindFile
	case types.Module:
		return scip.SCIPSymbolKindModule
	case types.Namespace:
		return scip.SCIPSymbolKindNamespace
	case types.Package:
		return scip.SCIPSymbolKindPackage
	case types.Class:
		return scip.SCIPSymbolKindClass
	case types.Method:
		return scip.SCIPSymbolKindMethod
	case types.Property:
		return scip.SCIPSymbolKindProperty
	case types.Field:
		return scip.SCIPSymbolKindField
	case types.Constructor:
		return scip.SCIPSymbolKindConstructor
	case types.Enum:
		return scip.SCIPSymbolKindEnum
	case types.Interface:
		return scip.SCIPSymbolKindInterface
	case types.Function:
		return scip.SCIPSymbolKindFunction
	case types.Variable:
		return scip.SCIPSymbolKindVariable
	case types.Constant:
		return scip.SCIPSymbolKindConstant
	case types.String:
		return scip.SCIPSymbolKindString
	case types.Number:
		return scip.SCIPSymbolKindNumber
	case types.Boolean:
		return scip.SCIPSymbolKindBoolean
	case types.Array:
		return scip.SCIPSymbolKindArray
	case types.Object:
		return scip.SCIPSymbolKindObject
	case types.Key:
		return scip.SCIPSymbolKindKey
	case types.Null:
		return scip.SCIPSymbolKindNull
	case types.EnumMember:
		return scip.SCIPSymbolKindEnumMember
	case types.Struct:
		return scip.SCIPSymbolKindStruct
	case types.Event:
		return scip.SCIPSymbolKindEvent
	case types.Operator:
		return scip.SCIPSymbolKindOperator
	case types.TypeParameter:
		return scip.SCIPSymbolKindTypeParameter
	default:
		return scip.SCIPSymbolKindUnknown
	}
}

// mapLSPSymbolKindToSyntaxKind maps LSP symbol kinds to SCIP syntax kinds
func (m *LSPManager) mapLSPSymbolKindToSyntaxKind(lspKind types.SymbolKind) types.SyntaxKind {
	switch lspKind {
	case types.Function:
		return types.SyntaxKindIdentifierFunctionDefinition
	case types.Method:
		return types.SyntaxKindIdentifierFunctionDefinition
	case types.Class:
		return types.SyntaxKindIdentifierType
	case types.Interface:
		return types.SyntaxKindIdentifierType
	case types.Variable:
		return types.SyntaxKindIdentifierLocal
	case types.Constant:
		return types.SyntaxKindIdentifierConstant
	case types.Field:
		return types.SyntaxKindIdentifierLocal
	case types.Property:
		return types.SyntaxKindIdentifierLocal
	case types.Namespace:
		return types.SyntaxKindIdentifierNamespace
	case types.Module:
		return types.SyntaxKindIdentifierModule
	default:
		return types.SyntaxKindUnspecified
	}
}

// storeDocumentOccurrences stores SCIP occurrences as a document
func (m *LSPManager) storeDocumentOccurrences(ctx context.Context, uri, language string, occurrences []scip.SCIPOccurrence, symbolInfos []scip.SCIPSymbolInformation) {
	if m.scipCache == nil {
		return
	}

	// If we only have occurrences (e.g., references), add them directly to SCIP storage
	if len(occurrences) > 0 && len(symbolInfos) == 0 {
		if mgr, ok := m.scipCache.(*cache.SCIPCacheManager); ok {
			_ = mgr.AddOccurrences(ctx, uri, occurrences)
			return
		}
	}

	// Otherwise, convert to SymbolInformation and index (keeps definitions + metadata)
	var symbols []types.SymbolInformation
	for i, occ := range occurrences {
		var displayName string
		var kind types.SymbolKind
		if i < len(symbolInfos) {
			displayName = symbolInfos[i].DisplayName
			kind = m.mapSCIPKindToLSPSymbolKind(symbolInfos[i].Kind)
		} else {
			displayName = occ.Symbol
			kind = types.Variable
		}

		si := types.SymbolInformation{
			Name: displayName,
			Kind: kind,
			Location: types.Location{
				URI: uri,
				Range: types.Range{
					Start: types.Position{Line: occ.Range.Start.Line, Character: occ.Range.Start.Character},
					End:   types.Position{Line: occ.Range.End.Line, Character: occ.Range.End.Character},
				},
			},
		}
		if occ.SelectionRange != nil {
			sr := *occ.SelectionRange
			si.SelectionRange = &types.Range{Start: types.Position{Line: sr.Start.Line, Character: sr.Start.Character}, End: types.Position{Line: sr.End.Line, Character: sr.End.Character}}
		}
		symbols = append(symbols, si)
	}

	common.LSPLogger.Debug("Converting %d occurrences to symbols for IndexDocument", len(symbols))
	// Log first few symbols being indexed
	for i, sym := range symbols {
		if i < 3 {
			common.LSPLogger.Debug("  IndexDocument symbol %d: Name=%s, Kind=%v, URI=%s", i, sym.Name, sym.Kind, sym.Location.URI)
		}
	}
	if err := m.scipCache.IndexDocument(ctx, uri, language, symbols); err != nil {
		common.LSPLogger.Warn("Failed to index document symbols for %s: %v", uri, err)
	} else {
		common.LSPLogger.Debug("Successfully called IndexDocument for %s with %d symbols", uri, len(symbols))
	}

	// Skip hover result caching during indexing to avoid overhead
}

// ProcessEnhancedQuery processes a query that combines LSP and SCIP data
func (m *LSPManager) ProcessEnhancedQuery(ctx context.Context, queryType, uri, language string, params interface{}) (interface{}, error) {
	if m.scipCache == nil {
		// Fall back to regular LSP processing
		return m.ProcessRequest(ctx, queryType, params)
	}

	// SCIP storage queries not available due to interface conflicts, fall back to regular LSP
	return m.ProcessRequest(ctx, queryType, params)
}

// GetIndexStats returns SCIP index statistics
func (m *LSPManager) GetIndexStats() interface{} {
	if m.scipCache == nil {
		return map[string]interface{}{"status": "disabled"}
	}

	// SCIP storage stats not available due to interface conflicts

	// Use SCIPCache interface
	if m.scipCache != nil {
		return m.scipCache.GetIndexStats()
	}

	return map[string]interface{}{"status": "unknown"}
}

// RefreshIndex refreshes the SCIP index for given files
func (m *LSPManager) RefreshIndex(ctx context.Context, files []string) error {
	if m.scipCache == nil {
		return fmt.Errorf("SCIP cache not available")
	}

	if m.scipCache != nil {
		return m.scipCache.UpdateIndex(ctx, files)
	}

	return fmt.Errorf("cache does not support index updates")
}

// mapSCIPKindToLSPSymbolKind maps SCIP symbol kinds back to LSP symbol kinds
func (m *LSPManager) mapSCIPKindToLSPSymbolKind(kind scip.SCIPSymbolKind) types.SymbolKind {
	switch kind {
	case scip.SCIPSymbolKindFile:
		return types.File
	case scip.SCIPSymbolKindModule:
		return types.Module
	case scip.SCIPSymbolKindNamespace:
		return types.Namespace
	case scip.SCIPSymbolKindPackage:
		return types.Package
	case scip.SCIPSymbolKindClass:
		return types.Class
	case scip.SCIPSymbolKindMethod:
		return types.Method
	case scip.SCIPSymbolKindProperty:
		return types.Property
	case scip.SCIPSymbolKindField:
		return types.Field
	case scip.SCIPSymbolKindConstructor:
		return types.Constructor
	case scip.SCIPSymbolKindEnum:
		return types.Enum
	case scip.SCIPSymbolKindInterface:
		return types.Interface
	case scip.SCIPSymbolKindFunction:
		return types.Function
	case scip.SCIPSymbolKindVariable:
		return types.Variable
	case scip.SCIPSymbolKindConstant:
		return types.Constant
	case scip.SCIPSymbolKindString:
		return types.String
	case scip.SCIPSymbolKindNumber:
		return types.Number
	case scip.SCIPSymbolKindBoolean:
		return types.Boolean
	case scip.SCIPSymbolKindArray:
		return types.Array
	case scip.SCIPSymbolKindObject:
		return types.Object
	case scip.SCIPSymbolKindKey:
		return types.Key
	case scip.SCIPSymbolKindNull:
		return types.Null
	case scip.SCIPSymbolKindEnumMember:
		return types.EnumMember
	case scip.SCIPSymbolKindStruct:
		return types.Struct
	case scip.SCIPSymbolKindEvent:
		return types.Event
	case scip.SCIPSymbolKindOperator:
		return types.Operator
	case scip.SCIPSymbolKindTypeParameter:
		return types.TypeParameter
	default:
		return types.Variable
	}
}

// Removed getSymbolDocumentation - hover not needed during indexing

// getSymbolRelationships retrieves relationships for a symbol
func (m *LSPManager) getSymbolRelationships(ctx context.Context, uri string, symbolName string, kind types.SymbolKind) []scip.SCIPRelationship {
	var relationships []scip.SCIPRelationship

	// For classes and interfaces, try to find implementations
	if kind == types.Class || kind == types.Interface {
		implementations := m.findImplementations(ctx, uri, symbolName)
		for _, impl := range implementations {
			relationships = append(relationships, scip.SCIPRelationship{
				Symbol:           impl,
				IsImplementation: true,
			})
		}
	}

	// For methods, try to find the type they belong to
	if kind == types.Method || kind == types.Function {
		typeDefinition := m.findTypeDefinition(ctx, uri, symbolName)
		if typeDefinition != "" {
			relationships = append(relationships, scip.SCIPRelationship{
				Symbol:           typeDefinition,
				IsTypeDefinition: true,
			})
		}
	}

	return relationships
}

// findImplementations finds implementations of an interface or class
func (m *LSPManager) findImplementations(ctx context.Context, uri string, symbolName string) []string {
	// Try to use textDocument/implementation if available
	// This is a simplified version - in reality, we'd need to find the symbol position first
	var implementations []string

	// For now, return empty - this would require more complex logic
	// to track type hierarchies across the codebase
	return implementations
}

// findTypeDefinition finds the type definition for a method or field
func (m *LSPManager) findTypeDefinition(ctx context.Context, uri string, symbolName string) string {
	// Try to use textDocument/typeDefinition if available
	// This is a simplified version - in reality, we'd need to find the symbol position first

	// For now, return empty - this would require more complex logic
	return ""
}

// Removed getSymbolNameAtPosition and extractSymbolFromHoverText - hover not needed during indexing

// groupReferencesByDocument groups occurrences by their document URIs with metadata
func (m *LSPManager) groupReferencesByDocument(occurrences []scip.SCIPOccurrence, locations []types.Location) map[string][]scip.SCIPOccurrence {
	documentGroups := make(map[string][]scip.SCIPOccurrence)

	// Group occurrences by document URI from corresponding locations
	for i, occurrence := range occurrences {
		if i < len(locations) {
			uri := locations[i].URI
			documentGroups[uri] = append(documentGroups[uri], occurrence)
		} else {
			// Fallback: try to extract URI from symbol context or use unknown
			common.LSPLogger.Warn("No corresponding location for occurrence %d, using unknown URI", i)
			documentGroups["unknown"] = append(documentGroups["unknown"], occurrence)
		}
	}

	common.LSPLogger.Debug("Grouped %d occurrences into %d documents", len(occurrences), len(documentGroups))
	return documentGroups
}

// isBuiltinType checks if a symbol name represents a builtin type
func (m *LSPManager) isBuiltinType(symbolName string) bool {
	// Common builtin types across different languages
	// Note: Some type names overlap between languages (e.g., 'int' in Go/Java/Python)
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
		// Python builtins (only unique ones not already in Go/JS)
		"float": true, "str": true, "list": true, "dict": true,
		"tuple": true, "set": true, "None": true, "bytes": true, "bytearray": true,
		"type": true, "complex": true,
		// Java builtins (only unique ones not already covered)
		"long": true, "short": true, "double": true, "char": true,
		"Integer": true, "Long": true, "Double": true, "Float": true,
		"Character": true, "Byte": true, "Short": true,
	}

	return builtinTypes[symbolName]
}

func (m *LSPManager) getIdentifierFromLocation(ctx context.Context, location types.Location) string {
	path := utils.URIToFilePathCached(location.URI)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	sl := location.Range.Start.Line
	el := location.Range.End.Line
	if sl < 0 || sl >= int32(len(lines)) {
		return ""
	}
	if el < sl {
		el = sl
	}
	if el >= int32(len(lines)) {
		el = int32(len(lines)) - 1
	}
	sc := location.Range.Start.Character
	ec := location.Range.End.Character
	if sl == el {
		line := lines[sl]
		if sc < 0 || sc > int32(len(line)) {
			sc = 0
		}
		if ec < sc || ec > int32(len(line)) {
			ec = sc
		}
		return m.sanitizeIdentifier(line[sc:ec])
	}
	var b strings.Builder
	first := lines[sl]
	if sc >= 0 && sc <= int32(len(first)) {
		b.WriteString(first[sc:])
	}
	for i := sl + 1; i < el; i++ {
		b.WriteString(lines[i])
	}
	last := lines[el]
	if ec >= 0 && ec <= int32(len(last)) {
		b.WriteString(last[:ec])
	}
	return m.sanitizeIdentifier(b.String())
}

func (m *LSPManager) sanitizeIdentifier(s string) string {
	rs := []rune(s)
	var out []rune
	for i, r := range rs {
		if i == 0 {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
				out = append(out, r)
			}
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			out = append(out, r)
		}
	}
	return strings.TrimSpace(string(out))
}

// findDocumentURIForOccurrence finds the document URI containing the given occurrence
func (m *LSPManager) findDocumentURIForOccurrence(ctx context.Context, storage scip.SCIPDocumentStorage, occ scip.SCIPOccurrence) string {
	docs, err := storage.ListDocuments(ctx)
	if err != nil {
		return ""
	}
	for _, docURI := range docs {
		doc, err := storage.GetDocument(ctx, docURI)
		if err != nil || doc == nil {
			continue
		}
		for _, docOcc := range doc.Occurrences {
			if docOcc.Symbol == occ.Symbol &&
				docOcc.Range.Start.Line == occ.Range.Start.Line &&
				docOcc.Range.Start.Character == occ.Range.Start.Character &&
				docOcc.Range.End.Line == occ.Range.End.Line &&
				docOcc.Range.End.Character == occ.Range.End.Character {
				return docURI
			}
		}
	}
	return ""
}
