package server

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/scip"
)

// SymbolReferenceQuery defines parameters for searching symbol references with occurrence-based filtering
type SymbolReferenceQuery struct {
	SymbolName        string `json:"symbolName"`
	FilePattern       string `json:"filePattern"`
	IncludeDefinition bool   `json:"includeDefinition"`
	MaxResults        int    `json:"maxResults"`
	IncludeCode       bool   `json:"includeCode"`
	ExactMatch        bool   `json:"exactMatch"`

	// Occurrence-based filtering options
	FilterByRole     *types.SymbolRole `json:"filterByRole,omitempty"` // Filter by specific symbol roles
	OnlyReadAccess   bool              `json:"onlyReadAccess"`         // Only show read accesses
	OnlyWriteAccess  bool              `json:"onlyWriteAccess"`        // Only show write accesses
	ExcludeGenerated bool              `json:"excludeGenerated"`       // Exclude generated code
	ExcludeTest      bool              `json:"excludeTest"`            // Exclude test code
	IncludeRelated   bool              `json:"includeRelated"`         // Include implementation relationships
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

// SearchSymbolReferences searches for all references to a symbol using occurrence-based search
func (m *LSPManager) SearchSymbolReferences(ctx context.Context, query SymbolReferenceQuery) (*SymbolReferenceResult, error) {

	if query.SymbolName == "" {
		return nil, fmt.Errorf("symbolName cannot be empty")
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
	usedOccurrenceSearch := false

	// Counters for different types of occurrences
	var definitionCount, readAccessCount, writeAccessCount int

	// First try occurrence-based reference search with SCIP storage
	if m.scipCache != nil {

		// Get SCIP storage for direct occurrence queries
		scipStorage := m.getScipStorageFromCache()
		if scipStorage != nil {
			// First find the symbol to get its SCIP ID
			symbolPattern := query.SymbolName
			if query.ExactMatch {
				symbolPattern = "^" + query.SymbolName + "$"
			}

			// Search for symbol information to get the symbol ID
			symbolInfos, err := scipStorage.SearchSymbols(ctx, symbolPattern, 10) // Get a few candidates
			if err == nil && len(symbolInfos) > 0 {
				// Process each symbol found
				for _, symbolInfo := range symbolInfos {
					if m.matchesSymbolName(symbolInfo.DisplayName, query.SymbolName, query.ExactMatch) {
						// Get all reference occurrences for this symbol
						refOccurrences, refErr := scipStorage.GetReferenceOccurrences(ctx, symbolInfo.Symbol)
						if refErr == nil {

							// Convert SCIP occurrences to ReferenceInfo with role-based filtering
							for _, occurrence := range refOccurrences {
								refInfo := m.createReferenceFromOccurrence(ctx, scipStorage, occurrence, &symbolInfo, query)
								if refInfo != nil && m.passesRoleFilter(*refInfo, query) {
									references = append(references, *refInfo)
									usedOccurrenceSearch = true

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

						// Get definition occurrence if requested
						if query.IncludeDefinition {
							defOccurrence, defErr := scipStorage.GetDefinitionOccurrence(ctx, symbolInfo.Symbol)
							if defErr == nil && defOccurrence != nil {
								defRefInfo := m.createReferenceFromOccurrence(ctx, scipStorage, *defOccurrence, &symbolInfo, query)
								if defRefInfo != nil && m.passesRoleFilter(*defRefInfo, query) {
									// Check for duplicates
									if !m.isDuplicateReference(references, *defRefInfo) {
										references = append(references, *defRefInfo)
										definitionCount++
									}
								}
							}
						}

						// Get implementation relationships if requested
						if query.IncludeRelated {
							implOccurrences, implErr := scipStorage.GetImplementations(ctx, symbolInfo.Symbol)
							if implErr == nil {
								for _, implOcc := range implOccurrences {
									implRefInfo := m.createReferenceFromOccurrence(ctx, scipStorage, implOcc, &symbolInfo, query)
									if implRefInfo != nil {
										implementations = append(implementations, *implRefInfo)
									}
								}
							}

							// Get related symbols through relationships
							relationships, relErr := scipStorage.GetSymbolRelationships(ctx, symbolInfo.Symbol)
							if relErr == nil {
								for _, rel := range relationships {
									if !m.containsString(relatedSymbols, rel.Symbol) {
										relatedSymbols = append(relatedSymbols, rel.Symbol)
									}
								}
							}
						}
					}
				}
			}
		} else {

			// Fallback to cache query approach
			symbolPattern := query.SymbolName
			if query.ExactMatch {
				symbolPattern = "^" + query.SymbolName + "$"
			}

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
						nameMatches := m.matchesSymbolName(symbolInfo.Name, query.SymbolName, query.ExactMatch)

						if nameMatches {
							refInfo := m.createLegacyReferenceInfo(*symbolInfo, query)

							// Check file pattern and role filters
							if m.matchesFilePattern(refInfo.FilePath, query.FilePattern) && m.passesRoleFilter(refInfo, query) {
								references = append(references, refInfo)
								usedOccurrenceSearch = true
							}
						}
					}
				}
			}

			// Also try to get references from the reference index directly
			// Try various key formats that might be used
			symbolKeys := []string{
				fmt.Sprintf("go:%s", query.SymbolName),
				fmt.Sprintf("python:%s", query.SymbolName),
				fmt.Sprintf("javascript:%s", query.SymbolName),
				fmt.Sprintf("typescript:%s", query.SymbolName),
				fmt.Sprintf("java:%s", query.SymbolName),
			}

			for _, symbolKey := range symbolKeys {
				refQuery := &cache.IndexQuery{
					Type:     "references",
					Symbol:   query.SymbolName,
					Language: strings.Split(symbolKey, ":")[0],
				}
				if refResult, err := m.scipCache.QueryIndex(ctx, refQuery); err == nil && refResult != nil {
					for _, ref := range refResult.Results {
						if loc, ok := ref.(lsp.Location); ok {
							refInfo := m.locationToReferenceInfo(loc, query.IncludeCode)
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
									usedOccurrenceSearch = true
								}
							}
						}
					}
				}
			}
		}
	}

	// If no SCIP occurrence results or SCIP not available, fall back to LSP
	if !usedOccurrenceSearch || len(references) == 0 {

		// Find the symbol first using SearchSymbolPattern
		symbolQuery := types.SymbolPatternQuery{
			Pattern:     query.SymbolName,
			FilePattern: query.FilePattern,
			MaxResults:  1, // We only need one matching symbol to find its references
			IncludeCode: false,
		}

		// If exact match is requested, wrap pattern in anchors
		if query.ExactMatch {
			symbolQuery.Pattern = "^" + query.SymbolName + "$"
		}

		// Find the symbol first
		symbolResult, err := m.SearchSymbolPattern(ctx, symbolQuery)
		if err != nil {
			// Don't fail completely, just return empty results
			return &SymbolReferenceResult{
				References: []ReferenceInfo{},
				TotalCount: 0,
				Truncated:  false,
			}, nil
		}

		if len(symbolResult.Symbols) > 0 {
			// Get the first matching symbol
			symbol := symbolResult.Symbols[0]

			// Prepare textDocument/references request
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": symbol.Location.URI,
				},
				"position": map[string]interface{}{
					"line":      symbol.Location.Range.Start.Line,
					"character": symbol.Location.Range.Start.Character,
				},
				"context": map[string]interface{}{
					"includeDeclaration": query.IncludeDefinition,
				},
			}

			// Send references request
			result, err := m.ProcessRequest(ctx, types.MethodTextDocumentReferences, params)
			if err == nil {
				// Parse the result
				switch refs := result.(type) {
				case []interface{}:
					for _, ref := range refs {
						if location, ok := ref.(map[string]interface{}); ok {
							refInfo := m.parseReferenceLocation(location, query.IncludeCode)
							if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
								references = append(references, *refInfo)
							}
						} else if loc, ok := ref.(lsp.Location); ok {
							refInfo := m.locationToReferenceInfo(loc, query.IncludeCode)
							if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
								references = append(references, *refInfo)
							}
						}
					}
				case []lsp.Location:
					for _, loc := range refs {
						refInfo := m.locationToReferenceInfo(loc, query.IncludeCode)
						if refInfo != nil && m.matchesFilePattern(refInfo.FilePath, query.FilePattern) {
							references = append(references, *refInfo)
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

	// Create enhanced search metadata
	searchMetadata := map[string]interface{}{
		"search_type":           "occurrence_based",
		"used_scip_storage":     usedOccurrenceSearch,
		"symbol_pattern":        query.SymbolName,
		"exact_match":           query.ExactMatch,
		"file_pattern":          query.FilePattern,
		"include_definition":    query.IncludeDefinition,
		"include_related":       query.IncludeRelated,
		"filter_read_only":      query.OnlyReadAccess,
		"filter_write_only":     query.OnlyWriteAccess,
		"exclude_generated":     query.ExcludeGenerated,
		"exclude_test":          query.ExcludeTest,
		"related_symbols_count": len(relatedSymbols),
		"implementations_count": len(implementations),
	}

	return &SymbolReferenceResult{
		References:       references,
		TotalCount:       totalCount,
		Truncated:        truncated,
		DefinitionCount:  definitionCount,
		ReadAccessCount:  readAccessCount,
		WriteAccessCount: writeAccessCount,
		Implementations:  implementations,
		RelatedSymbols:   relatedSymbols,
		SearchMetadata:   searchMetadata,
	}, nil
}

// parseReferenceLocation parses a location map into ReferenceInfo
func (m *LSPManager) parseReferenceLocation(location map[string]interface{}, includeCode bool) *ReferenceInfo {
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
		FilePath:   strings.TrimPrefix(uri, "file://"),
		LineNumber: line,
		Column:     character,
	}

	// Read code context if requested
	if includeCode && uri != "" {
		r := lsp.Range{
			Start: lsp.Position{Line: line, Character: character},
			End:   lsp.Position{Line: line, Character: character + 50}, // Read 50 chars for context
		}
		if code, err := m.readSymbolCode(refInfo.FilePath, r); err == nil {
			refInfo.Code = strings.TrimSpace(code)
			// Also get the full line as context
			if fullLine, err := m.readFullLine(refInfo.FilePath, line); err == nil {
				refInfo.Context = strings.TrimSpace(fullLine)
			}
		}
	}

	return refInfo
}

// locationToReferenceInfo converts an lsp.Location to ReferenceInfo
func (m *LSPManager) locationToReferenceInfo(loc lsp.Location, includeCode bool) *ReferenceInfo {
	refInfo := &ReferenceInfo{
		FilePath:   strings.TrimPrefix(loc.URI, "file://"),
		LineNumber: loc.Range.Start.Line,
		Column:     loc.Range.Start.Character,
	}

	// Read code context if requested
	if includeCode && loc.URI != "" {
		if code, err := m.readSymbolCode(refInfo.FilePath, loc.Range); err == nil {
			refInfo.Code = strings.TrimSpace(code)
			// Also get the full line as context
			if fullLine, err := m.readFullLine(refInfo.FilePath, loc.Range.Start.Line); err == nil {
				refInfo.Context = strings.TrimSpace(fullLine)
			}
		}
	}

	return refInfo
}

// matchesFilePattern checks if a file path matches the given pattern
func (m *LSPManager) matchesFilePattern(filePath, pattern string) bool {
	// Handle special patterns
	if pattern == "" || pattern == "**/*" || pattern == "*" {
		return true
	}

	// Convert glob pattern to filepath.Match pattern
	if strings.Contains(pattern, "**") {
		// For ** patterns, check if the path contains the pattern part
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			if prefix != "" && !strings.HasPrefix(filePath, prefix) {
				return false
			}
			if suffix != "" && suffix != "*" {
				matched, _ := filepath.Match(suffix, filepath.Base(filePath))
				return matched
			}
			return true
		}
	}

	// Check if it's a directory pattern
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(filePath, pattern)
	}

	// Try direct glob match
	matched, _ := filepath.Match(pattern, filePath)
	if matched {
		return true
	}

	// Try matching against the base name
	matched, _ = filepath.Match(pattern, filepath.Base(filePath))
	return matched
}

// readFullLine reads a complete line from a file
func (m *LSPManager) readFullLine(filePath string, lineNumber int) (string, error) {
	r := lsp.Range{
		Start: lsp.Position{Line: lineNumber, Character: 0},
		End:   lsp.Position{Line: lineNumber, Character: 1000}, // Read up to 1000 chars
	}
	return m.readSymbolCode(filePath, r)
}

// Helper methods for occurrence-based reference search

// getScipStorageFromCache extracts SCIP storage from cache manager
func (m *LSPManager) getScipStorageFromCache() scip.SCIPDocumentStorage {
	if m.scipCache == nil {
		return nil
	}

	// For now, return nil and handle gracefully - need proper accessor method
	// TODO: Add proper accessor method to SimpleCacheManager or use direct interface
	return nil
}

// createReferenceFromOccurrence creates a ReferenceInfo from a SCIP occurrence
func (m *LSPManager) createReferenceFromOccurrence(ctx context.Context, scipStorage scip.SCIPDocumentStorage, occurrence scip.SCIPOccurrence, symbolInfo *scip.SCIPSymbolInformation, query SymbolReferenceQuery) *ReferenceInfo {
	// Convert SCIP range to LSP-compatible format
	refInfo := &ReferenceInfo{
		FilePath:       "", // Need to determine from document context
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

	// Read code context if requested
	if query.IncludeCode && refInfo.FilePath != "" {
		lspRange := lsp.Range{
			Start: lsp.Position{Line: refInfo.LineNumber, Character: refInfo.Column},
			End:   lsp.Position{Line: int(occurrence.Range.End.Line), Character: int(occurrence.Range.End.Character)},
		}
		if code, err := m.readSymbolCode(refInfo.FilePath, lspRange); err == nil {
			refInfo.Code = strings.TrimSpace(code)
			// Also get the full line as context
			if fullLine, err := m.readFullLine(refInfo.FilePath, refInfo.LineNumber); err == nil {
				refInfo.Context = strings.TrimSpace(fullLine)
			}
		}
	}

	return refInfo
}

// createLegacyReferenceInfo creates ReferenceInfo from LSP SymbolInformation (fallback)
func (m *LSPManager) createLegacyReferenceInfo(symbolInfo lsp.SymbolInformation, query SymbolReferenceQuery) ReferenceInfo {
	refInfo := ReferenceInfo{
		FilePath:   strings.TrimPrefix(symbolInfo.Location.URI, "file://"),
		LineNumber: symbolInfo.Location.Range.Start.Line,
		Column:     symbolInfo.Location.Range.Start.Character,
		SymbolID:   fmt.Sprintf("lsp:%s", symbolInfo.Name), // Legacy format
		Range: &types.Range{
			Start: types.Position{
				Line:      int32(symbolInfo.Location.Range.Start.Line),
				Character: int32(symbolInfo.Location.Range.Start.Character),
			},
			End: types.Position{
				Line:      int32(symbolInfo.Location.Range.End.Line),
				Character: int32(symbolInfo.Location.Range.End.Character),
			},
		},
		// Default role assumptions for LSP symbols
		IsReadAccess: true, // Assume references are read access by default
	}

	// Read code context if requested
	if query.IncludeCode && symbolInfo.Location.URI != "" {
		if code, err := m.readSymbolCode(refInfo.FilePath, symbolInfo.Location.Range); err == nil {
			refInfo.Code = strings.TrimSpace(code)
			// Also get the full line as context
			if fullLine, err := m.readFullLine(refInfo.FilePath, symbolInfo.Location.Range.Start.Line); err == nil {
				refInfo.Context = strings.TrimSpace(fullLine)
			}
		}
	}

	return refInfo
}

// matchesSymbolName checks if a symbol name matches the query pattern
func (m *LSPManager) matchesSymbolName(symbolName string, queryName string, exactMatch bool) bool {
	if exactMatch {
		return symbolName == queryName
	}
	return strings.Contains(strings.ToLower(symbolName), strings.ToLower(queryName))
}

// passesRoleFilter checks if a reference passes the role-based filters
func (m *LSPManager) passesRoleFilter(refInfo ReferenceInfo, query SymbolReferenceQuery) bool {
	// Filter by specific role if provided
	if query.FilterByRole != nil {
		if !refInfo.OccurrenceRole.HasRole(*query.FilterByRole) {
			return false
		}
	}

	// Filter for read access only
	if query.OnlyReadAccess && !refInfo.IsReadAccess {
		return false
	}

	// Filter for write access only
	if query.OnlyWriteAccess && !refInfo.IsWriteAccess {
		return false
	}

	// Exclude generated code
	if query.ExcludeGenerated && refInfo.IsGenerated {
		return false
	}

	// Exclude test code
	if query.ExcludeTest && refInfo.IsTest {
		return false
	}

	return true
}

// isDuplicateReference checks if a reference is already in the results
func (m *LSPManager) isDuplicateReference(references []ReferenceInfo, newRef ReferenceInfo) bool {
	for _, ref := range references {
		if ref.FilePath == newRef.FilePath &&
			ref.LineNumber == newRef.LineNumber &&
			ref.Column == newRef.Column {
			return true
		}
	}
	return false
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

// containsString checks if a string slice contains a specific string
func (m *LSPManager) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
