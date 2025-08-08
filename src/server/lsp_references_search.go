package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
)

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
		query.FilePattern = "**/*"
	}
	if query.MaxResults <= 0 {
		query.MaxResults = 100
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
				case cache.SCIPOccurrenceInfo:
					ref := ReferenceInfo{
						FilePath:      utils.URIToFilePath(v.DocumentURI),
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
										FilePath:     utils.URIToFilePath(uri),
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
				case cache.SCIPOccurrenceInfo:
					fallbackDefRef = &ReferenceInfo{
						FilePath:   utils.URIToFilePath(d.DocumentURI),
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
										FilePath:   utils.URIToFilePath(uri),
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
		scipStorage := m.getScipStorageFromCache()
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
					var symbolInfo *lsp.SymbolInformation

					// Extract symbol information from various result types
					if scipSymbol, ok := result.(*cache.SCIPSymbol); ok {
						symbolInfo = &scipSymbol.SymbolInfo
					} else if si, ok := result.(lsp.SymbolInformation); ok {
						symbolInfo = &si
					} else if resultMap, ok := result.(map[string]interface{}); ok {
						if symbolInfoData, hasSymbolInfo := resultMap["symbol_info"]; hasSymbolInfo {
							if si, ok := symbolInfoData.(lsp.SymbolInformation); ok {
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
		
		if wsResult, wsErr := m.ProcessRequest(ctx, "workspace/symbol", wsParams); wsErr == nil && wsResult != nil {
			// Parse workspace symbol results
			var wsSymbols []interface{}
			if rawBytes, ok := wsResult.(json.RawMessage); ok {
				var symbols []map[string]interface{}
				if jsonErr := json.Unmarshal(rawBytes, &symbols); jsonErr == nil {
					for _, sym := range symbols {
						wsSymbols = append(wsSymbols, sym)
					}
				}
			} else if symbols, ok := wsResult.([]interface{}); ok {
				wsSymbols = symbols
			}
			
			// For each symbol found, try to get references from its location
			// Limit the number of LSP requests to avoid overwhelming the server
			maxLSPRequests := 5
			lspRequestCount := 0
			
			for _, wsSymbol := range wsSymbols {
				// Stop making LSP requests if we've hit the limit
				if lspRequestCount >= maxLSPRequests {
					break
				}
				
				if symbolMap, ok := wsSymbol.(map[string]interface{}); ok {
					// First add the symbol itself as a reference
					if location, hasLocation := symbolMap["location"].(map[string]interface{}); hasLocation {
						if uri, hasURI := location["uri"].(string); hasURI {
							if rangeMap, hasRange := location["range"].(map[string]interface{}); hasRange {
								if start, hasStart := rangeMap["start"].(map[string]interface{}); hasStart {
									line, _ := start["line"].(float64)
									char, _ := start["character"].(float64)
									
									// Add the symbol location itself as a reference
									filePath := utils.URIToFilePath(uri)
									if filePath != "" && m.matchesFilePattern(filePath, query.FilePattern) {
										refInfo := ReferenceInfo{
											FilePath:     filePath,
											LineNumber:   int(line) + 1,
											Column:       int(char) + 1,
											IsDefinition: false,
											IsReadAccess: true,
										}
										
										// Check for duplicates
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
											references = append(references, refInfo)
										}
									}
									
									// Only try to get actual references for a limited number of symbols
									if lspRequestCount < maxLSPRequests {
										lspRequestCount++
										
										// Try to get references from this position
										refParams := map[string]interface{}{
											"textDocument": map[string]interface{}{
												"uri": uri,
											},
											"position": map[string]interface{}{
												"line":      int(line),
												"character": int(char),
											},
											"context": map[string]interface{}{
												"includeDeclaration": true,
											},
										}
										
										if refResult, refErr := m.ProcessRequest(ctx, "textDocument/references", refParams); refErr == nil && refResult != nil {
											// Parse reference results
											var parsedLocations []interface{}
											
											if rawMsg, ok := refResult.(json.RawMessage); ok {
												var locations []map[string]interface{}
												if jsonErr := json.Unmarshal(rawMsg, &locations); jsonErr == nil {
													for _, loc := range locations {
														parsedLocations = append(parsedLocations, loc)
													}
												}
											} else if locs, ok := refResult.([]interface{}); ok {
												parsedLocations = locs
											}
											
											// Process parsed locations
											for _, ref := range parsedLocations {
												if location, ok := ref.(map[string]interface{}); ok {
													refInfo := m.parseReferenceLocation(location)
													if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
														refInfo.IsReadAccess = true
														references = append(references, *refInfo)
														readAccessCount++
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
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
			if scipStorage := m.getScipStorageFromCache(); scipStorage != nil {
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
			
			wsResult, wsErr := m.ProcessRequest(ctx, "workspace/symbol", wsParams)
			if wsErr == nil && wsResult != nil {
				// Parse workspace symbol result
				var symbols []interface{}
				
				if rawMsg, ok := wsResult.(json.RawMessage); ok {
					if err := json.Unmarshal([]byte(rawMsg), &symbols); err == nil {
						for _, sym := range symbols {
							if symMap, ok := sym.(map[string]interface{}); ok {
								if name, ok := symMap["name"].(string); ok && name == query.Pattern {
									if location, ok := symMap["location"].(map[string]interface{}); ok {
										if uri, ok := location["uri"].(string); ok {
											if rng, ok := location["range"].(map[string]interface{}); ok {
												if start, ok := rng["start"].(map[string]interface{}); ok {
													if line, ok := start["line"].(float64); ok {
														if char, ok := start["character"].(float64); ok {
															fallbackDefRef = &ReferenceInfo{
																FilePath:     utils.URIToFilePath(uri),
																LineNumber:   int(line),
																Column:       int(char),
																IsDefinition: true,
															}
															common.LSPLogger.Debug("[SearchSymbolReferences] Found definition via workspace symbol: %s:%d:%d", fallbackDefRef.FilePath, fallbackDefRef.LineNumber, fallbackDefRef.Column)
															break
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		if fallbackDefRef != nil {
			// Use SCIP def position to ask LSP for references
			uri := utils.FilePathToURI(fallbackDefRef.FilePath)
			filePath := fallbackDefRef.FilePath

			// Read the file content to open it in LSP server
			content, readErr := m.readFileContent(filePath)
			if readErr == nil {
				didOpenParams := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri":        uri,
						"languageId": m.detectLanguageFromURI(uri),
						"version":    1,
						"text":       string(content),
					},
				}
				_, _ = m.ProcessRequest(ctx, "textDocument/didOpen", didOpenParams)
				defer func() {
					didCloseParams := map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": uri,
						},
					}
					_, _ = m.ProcessRequest(ctx, "textDocument/didClose", didCloseParams)
				}()
			}

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

			// Send references request
			result, err := m.ProcessRequest(ctx, types.MethodTextDocumentReferences, params)
			if err == nil && result != nil {
				// Parse the result - handle various response types
				var parsedLocations []interface{}

				if rawMsg, ok := result.(json.RawMessage); ok {
					var locations []map[string]interface{}
					if jsonErr := json.Unmarshal(rawMsg, &locations); jsonErr == nil {
						for _, loc := range locations {
							parsedLocations = append(parsedLocations, loc)
						}
					}
				} else if bytes, ok := result.([]byte); ok {
					var locations []map[string]interface{}
					if jsonErr := json.Unmarshal(bytes, &locations); jsonErr == nil {
						for _, loc := range locations {
							parsedLocations = append(parsedLocations, loc)
						}
					}
				} else if locs, ok := result.([]interface{}); ok {
					parsedLocations = locs
				}

				// Process parsed locations
				for _, ref := range parsedLocations {
					if location, ok := ref.(map[string]interface{}); ok {
						refInfo := m.parseReferenceLocation(location)
						if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
							refInfo.IsReadAccess = true
							references = append(references, *refInfo)
							readAccessCount++
						}
					} else if loc, ok := ref.(types.Location); ok {
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
func (m *LSPManager) parseReferenceLocation(location map[string]interface{}) *ReferenceInfo {
	var uri string
	var line, character int

	if uriVal, ok := location["uri"].(string); ok {
		uri = uriVal
	} else {
		return nil
	}

	if rangeMap, ok := location["range"].(map[string]interface{}); ok {
		if start, ok := rangeMap["start"].(map[string]interface{}); ok {
			if lineVal, ok := start["line"].(float64); ok {
				line = int(lineVal)
			}
			if charVal, ok := start["character"].(float64); ok {
				character = int(charVal)
			}
		}
	}

	refInfo := &ReferenceInfo{
		FilePath:   utils.URIToFilePath(uri),
		LineNumber: line,
		Column:     character,
	}

	return refInfo
}

// locationToReferenceInfo converts a types.Location to ReferenceInfo
func (m *LSPManager) locationToReferenceInfo(loc types.Location) *ReferenceInfo {
	refInfo := &ReferenceInfo{
		FilePath:   utils.URIToFilePath(loc.URI),
		LineNumber: int(loc.Range.Start.Line),
		Column:     int(loc.Range.Start.Character),
	}

	return refInfo
}

// matchesFilePattern checks if a file path matches the given pattern
func (m *LSPManager) matchesFilePattern(filePath, pattern string) bool {
	// Handle special patterns
	if pattern == "" || pattern == "**/*" || pattern == "*" {
		return true
	}

	// Convert absolute path to relative if needed for pattern matching
	// If filePath is absolute and pattern is relative, make filePath relative
	normalizedPath := filePath
	if filepath.IsAbs(filePath) && !filepath.IsAbs(pattern) {
		// Try to make the path relative to the working directory
		if wd, err := os.Getwd(); err == nil {
			if relPath, err := filepath.Rel(wd, filePath); err == nil {
				normalizedPath = relPath
			}
		}
	}

	// Convert glob pattern to filepath.Match pattern
	if strings.Contains(pattern, "**") {
		// For ** patterns, check if the path contains the pattern part
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			// Check both the normalized path and the original path
			for _, pathToCheck := range []string{normalizedPath, filePath} {
				matches := true
				if prefix != "" && !strings.Contains(pathToCheck, prefix) {
					matches = false
				}
				if matches && suffix != "" && suffix != "*" {
					matched, _ := filepath.Match(suffix, filepath.Base(pathToCheck))
					if !matched {
						matches = false
					}
				}
				if matches {
					return true
				}
			}
			return false
		}
	}

	// Check if it's a directory pattern
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(normalizedPath, pattern) || strings.HasPrefix(filePath, pattern)
	}

	// Try direct glob match on both normalized and original paths
	for _, pathToCheck := range []string{normalizedPath, filePath} {
		matched, _ := filepath.Match(pattern, pathToCheck)
		if matched {
			return true
		}
		// Try matching against the base name
		matched, _ = filepath.Match(pattern, filepath.Base(pathToCheck))
		if matched {
			return true
		}
	}

	return false
}

// readFullLine reads a complete line from a file
// readFullLine helper was removed as unused

// Helper methods for occurrence-based reference search

// getScipStorageFromCache extracts SCIP storage from cache manager
func (m *LSPManager) getScipStorageFromCache() scip.SCIPDocumentStorage {
	if m.scipCache == nil {
		return nil
	}

	// Use the GetSCIPStorage method to access the underlying storage
	return m.scipCache.GetSCIPStorage()
}

// createReferenceFromOccurrenceWithDoc creates ReferenceInfo from a SCIP occurrence with document URI
func (m *LSPManager) createReferenceFromOccurrenceWithDoc(occWithDoc scip.OccurrenceWithDocument, symbolInfo *scip.SCIPSymbolInformation) *ReferenceInfo {
	filePath := utils.URIToFilePath(occWithDoc.DocumentURI)

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
func (m *LSPManager) createReferenceFromOccurrence(ctx context.Context, scipStorage scip.SCIPDocumentStorage, occurrence scip.SCIPOccurrence, symbolInfo *scip.SCIPSymbolInformation) *ReferenceInfo {
	// NOTE: The occurrence should ideally contain document URI information
	// For now, we need to find which document contains this occurrence
	// This is inefficient but necessary with the current SCIP storage interface

	// Find the document URI that contains this occurrence
	filePath := ""
	docs, err := scipStorage.ListDocuments(ctx)
	if err == nil {
		// TODO: Optimize this by having occurrences store their document URI
		for _, docURI := range docs {
			doc, docErr := scipStorage.GetDocument(ctx, docURI)
			if docErr == nil && doc != nil {
				// Check if this document contains the occurrence
				for _, docOcc := range doc.Occurrences {
					if m.occurrencesMatch(docOcc, occurrence) {
						filePath = utils.URIToFilePath(docURI)
						break
					}
				}
				if filePath != "" {
					break
				}
			}
		}
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
func (m *LSPManager) occurrencesMatch(a, b scip.SCIPOccurrence) bool {
	return a.Symbol == b.Symbol &&
		a.Range.Start.Line == b.Range.Start.Line &&
		a.Range.Start.Character == b.Range.Start.Character &&
		a.Range.End.Line == b.Range.End.Line &&
		a.Range.End.Character == b.Range.End.Character &&
		a.SymbolRoles == b.SymbolRoles
}

// createLegacyReferenceInfo creates ReferenceInfo from LSP SymbolInformation (fallback)
func (m *LSPManager) createLegacyReferenceInfo(symbolInfo lsp.SymbolInformation) ReferenceInfo {
	refInfo := ReferenceInfo{
		FilePath:   utils.URIToFilePath(symbolInfo.Location.URI),
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

// readFileContent reads the content of a file
func (m *LSPManager) readFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// detectLanguageFromURI detects the language from a URI based on file extension
func (m *LSPManager) detectLanguageFromURI(uri string) string {
	filePath := utils.URIToFilePath(uri)
	ext := filepath.Ext(filePath)
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
		return "plaintext"
	}
}
