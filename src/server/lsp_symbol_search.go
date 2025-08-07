package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// EnhancedSymbolResult contains rich symbol information with occurrence data and role-based scoring
type EnhancedSymbolResult struct {
	// Base symbol information
	lsp.SymbolInformation

	// Enhanced metadata from SCIP
	SymbolID        string                  `json:"symbol_id"`
	OccurrenceRoles types.SymbolRole        `json:"occurrence_roles"`
	Documentation   []string                `json:"documentation,omitempty"`
	Signature       string                  `json:"signature,omitempty"`
	Relationships   []scip.SCIPRelationship `json:"relationships,omitempty"`

	// Scoring and ranking
	Score          float64 `json:"-"` // Internal use for sorting
	RelevanceScore float64 `json:"relevance_score"`
	RoleBonus      float64 `json:"role_bonus"`

	// Context information
	FilePath   string `json:"file_path"`
	LineNumber int    `json:"line_number"`
	EndLine    int    `json:"end_line,omitempty"`
	Container  string `json:"container,omitempty"`
	Code       string `json:"code,omitempty"`

	// Occurrence metadata
	OccurrenceCount int  `json:"occurrence_count"`
	IsDefinition    bool `json:"is_definition"`
	IsGenerated     bool `json:"is_generated"`
	IsTest          bool `json:"is_test"`
}

// SearchSymbolPattern searches for symbols matching a pattern using occurrence-based search
func (m *LSPManager) SearchSymbolPattern(ctx context.Context, query types.SymbolPatternQuery) (*types.SymbolPatternResult, error) {

	if query.Pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}
	if query.FilePattern == "" {
		return nil, fmt.Errorf("filePattern cannot be empty")
	}

	// Set defaults
	if query.MaxResults <= 0 {
		query.MaxResults = 100
	}

	// First try occurrence-based search with SCIP storage for fast lookup
	var enrichedSymbols []EnhancedSymbolResult
	usedOccurrenceSearch := false

	if m.scipCache != nil {

		// Use direct SCIP storage methods for occurrence-based search
		scipStorage := m.getScipStorage()
		if scipStorage != nil {
			// Search symbols using SCIP storage SearchSymbols method
			symbolInfos, err := scipStorage.SearchSymbols(ctx, query.Pattern, query.MaxResults*2) // Get more for filtering
			if err == nil && len(symbolInfos) > 0 {

				// Convert SCIP symbol information to enriched results with occurrence data
				for _, symbolInfo := range symbolInfos {
					enrichedResult, err := m.createEnrichedSymbolResult(ctx, scipStorage, &symbolInfo, query)
					if err == nil && enrichedResult != nil {
						enrichedSymbols = append(enrichedSymbols, *enrichedResult)
						usedOccurrenceSearch = true
					}
				}

				// Also search occurrences directly for pattern matching
				occurrences, err := scipStorage.SearchOccurrences(ctx, query.Pattern, query.MaxResults)
				if err == nil && len(occurrences) > 0 {

					// Process occurrences to extract symbols with role-based scoring
					for _, occurrence := range occurrences {
						if enrichedResult := m.createEnrichedResultFromOccurrence(ctx, scipStorage, occurrence, query); enrichedResult != nil {
							// Check for duplicates based on symbol ID and URI
							if !m.isDuplicateEnrichedResult(enrichedSymbols, *enrichedResult) {
								enrichedSymbols = append(enrichedSymbols, *enrichedResult)
								usedOccurrenceSearch = true
							}
						}
					}
				}
			}
		}
	}

	// If no occurrence search results or SCIP not available, fall back to traditional LSP search
	if !usedOccurrenceSearch || len(enrichedSymbols) == 0 {
		// Get all active clients for aggregation
		m.mu.RLock()
		clients := make(map[string]interface{})
		for k, v := range m.clients {
			clients[k] = v
		}
		m.mu.RUnlock()

		// Use the workspace aggregator to get symbols from all language servers
		// Convert regex patterns to something LSP servers can handle
		lspQuery := query.Pattern

		// Strip (?i) prefix if present since LSP servers don't understand regex flags
		if strings.HasPrefix(lspQuery, "(?i)") {
			lspQuery = strings.TrimPrefix(lspQuery, "(?i)")
		}

		// For regex patterns, try to extract a usable part for the LSP query
		// LSP servers typically do fuzzy/substring matching, not regex
		if strings.Contains(lspQuery, "*") || strings.Contains(lspQuery, "?") || strings.Contains(lspQuery, "[") || strings.Contains(lspQuery, ".") {
			// Try to extract a literal prefix before any regex metacharacters
			// For patterns like "New.*" extract "New"
			prefixEnd := strings.IndexAny(lspQuery, ".*?[]()+|^$\\")
			if prefixEnd > 0 {
				lspQuery = lspQuery[:prefixEnd]
			} else if prefixEnd == 0 {
				// Pattern starts with metacharacter (like ".*Manager")
				// Try to extract a suffix after the metacharacters
				if strings.HasPrefix(lspQuery, ".*") {
					lspQuery = strings.TrimPrefix(lspQuery, ".*")
					// Remove any remaining regex chars from the suffix
					if idx := strings.IndexAny(lspQuery, ".*?[]()+|^$\\"); idx > 0 {
						lspQuery = lspQuery[:idx]
					}
				} else {
					// Can't extract a useful query, use a common letter
					lspQuery = "e" // Common letter to get many symbols
				}
			}
		}

		aggregatedResult, err := m.workspaceAggregator.ProcessWorkspaceSymbol(ctx, clients, map[string]interface{}{
			"query": lspQuery,
		})

		if err != nil {
			// Workspace symbol aggregation failed
			// Continue with empty results rather than failing completely
		} else if aggregatedResult != nil {
			// Convert aggregated result to enriched symbol array
			if symbols, ok := aggregatedResult.([]interface{}); ok {
				for _, sym := range symbols {
					if symbolMap, ok := sym.(map[string]interface{}); ok {
						// Convert map to SymbolInformation and create enriched result
						symbolInfo := m.mapToSymbolInfo(symbolMap)
						if symbolInfo != nil {
							// Convert to enriched format for consistency
							enrichedResult := m.convertToEnrichedResult(*symbolInfo, query)
							if enrichedResult != nil {
								enrichedSymbols = append(enrichedSymbols, *enrichedResult)
							}
						}
					} else if symbolInfo, ok := sym.(lsp.SymbolInformation); ok {
						// Convert to enriched format for consistency
						enrichedResult := m.convertToEnrichedResult(symbolInfo, query)
						if enrichedResult != nil {
							enrichedSymbols = append(enrichedSymbols, *enrichedResult)
						}
					}
				}
			}
		}
	}

	// Apply pattern matching, filtering, and role-based scoring
	var matchedSymbols []types.EnhancedSymbolInfo

	// Cache document symbols to get full ranges (only used for fallback LSP symbols)
	var docSymbolsCache map[string][]lsp.DocumentSymbol
	if !usedOccurrenceSearch {
		docSymbolsCache = make(map[string][]lsp.DocumentSymbol)
	}

	for _, enrichedSymbol := range enrichedSymbols {
		symbol := enrichedSymbol.SymbolInformation

		matched, baseScore := types.MatchSymbolPattern(symbol, query)
		if matched {
			// Calculate enhanced score based on roles and occurrence data
			finalScore := m.calculateEnhancedScore(enrichedSymbol, baseScore)

			enhancedInfo := types.EnhancedSymbolInfo{
				SymbolInformation: symbol,
				Score:             finalScore,
				FilePath:          strings.TrimPrefix(symbol.Location.URI, "file://"),
				LineNumber:        symbol.Location.Range.Start.Line, // Keep 0-indexed
				EndLine:           symbol.Location.Range.End.Line,   // Keep 0-indexed
				Signature:         enrichedSymbol.Signature,
				Documentation:     strings.Join(enrichedSymbol.Documentation, "\n"),
			}

			// Only perform runtime enhancement for symbols from fallback LSP queries
			// SCIP occurrence-based symbols should already be enhanced at search-time
			if !usedOccurrenceSearch && enhancedInfo.LineNumber == enhancedInfo.EndLine && symbol.Location.URI != "" {
				// Check cache first
				if _, cached := docSymbolsCache[symbol.Location.URI]; !cached {
					// Fetch document symbols for this file
					params := map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": symbol.Location.URI,
						},
					}
					// Get the appropriate client for this file's language
					fileLanguage := m.documentManager.DetectLanguage(symbol.Location.URI)
					if client, err := m.getClient(fileLanguage); err == nil {
						if docResult, err := client.SendRequest(ctx, types.MethodTextDocumentDocumentSymbol, params); err == nil {
							if docSymbols := m.parseDocumentSymbolsToDocumentSymbol(docResult); docSymbols != nil {
								docSymbolsCache[symbol.Location.URI] = docSymbols
							}
						}
					}
				}

				// Find matching symbol in document symbols to get full range
				if docSymbols, ok := docSymbolsCache[symbol.Location.URI]; ok {
					if fullRange := m.findSymbolRange(symbol.Name, symbol.Location.Range.Start.Line, docSymbols); fullRange != nil {
						enhancedInfo.EndLine = fullRange.End.Line
					}
				}
			}

			if symbol.ContainerName != "" {
				enhancedInfo.Container = symbol.ContainerName
			}

			// Read source code if requested
			if query.IncludeCode {
				if code, err := m.readSymbolCode(enhancedInfo.FilePath, symbol.Location.Range); err == nil {
					enhancedInfo.Code = code
				}
			}

			matchedSymbols = append(matchedSymbols, enhancedInfo)
		}
	}

	// Sort by score (highest first)
	sortSymbolsByScore(matchedSymbols)

	// Apply max results limit
	truncated := false
	if len(matchedSymbols) > query.MaxResults {
		matchedSymbols = matchedSymbols[:query.MaxResults]
		truncated = true
	}

	// Cache the results if cache is available
	if m.scipCache != nil && len(matchedSymbols) > 0 {
		cacheKey := fmt.Sprintf("pattern_search:%s", query.Pattern)
		m.scipCache.Store("pattern_search", map[string]interface{}{"query": cacheKey}, matchedSymbols)
	}

	result := &types.SymbolPatternResult{
		Symbols:    matchedSymbols,
		TotalCount: len(matchedSymbols),
		Truncated:  truncated,
	}

	return result, nil
}

// mapToSymbolInfo converts a map to SymbolInformation
func (m *LSPManager) mapToSymbolInfo(symbolMap map[string]interface{}) *lsp.SymbolInformation {
	symbol := &lsp.SymbolInformation{}

	if name, ok := symbolMap["name"].(string); ok {
		symbol.Name = name
	} else {
		return nil
	}

	if kind, ok := symbolMap["kind"].(float64); ok {
		symbol.Kind = lsp.SymbolKind(kind)
	}

	if location, ok := symbolMap["location"].(map[string]interface{}); ok {
		if uri, ok := location["uri"].(string); ok {
			symbol.Location.URI = uri
		}

		if rangeMap, ok := location["range"].(map[string]interface{}); ok {
			if start, ok := rangeMap["start"].(map[string]interface{}); ok {
				if line, ok := start["line"].(float64); ok {
					symbol.Location.Range.Start.Line = int(line)
				}
				if char, ok := start["character"].(float64); ok {
					symbol.Location.Range.Start.Character = int(char)
				}
			}
			if end, ok := rangeMap["end"].(map[string]interface{}); ok {
				if line, ok := end["line"].(float64); ok {
					symbol.Location.Range.End.Line = int(line)
				}
				if char, ok := end["character"].(float64); ok {
					symbol.Location.Range.End.Character = int(char)
				}
			}
		}
	}

	if containerName, ok := symbolMap["containerName"].(string); ok {
		symbol.ContainerName = containerName
	}

	return symbol
}

// parseDocumentSymbolsToDocumentSymbol converts document symbol response to DocumentSymbol array
func (m *LSPManager) parseDocumentSymbolsToDocumentSymbol(result interface{}) []lsp.DocumentSymbol {
	var symbols []lsp.DocumentSymbol

	switch v := result.(type) {
	case []lsp.DocumentSymbol:
		return v
	case []interface{}:
		for _, item := range v {
			if data, err := json.Marshal(item); err == nil {
				var docSymbol lsp.DocumentSymbol
				if err := json.Unmarshal(data, &docSymbol); err == nil && docSymbol.Name != "" {
					symbols = append(symbols, docSymbol)
					// Add children recursively
					symbols = append(symbols, m.flattenDocumentSymbols(docSymbol.Children)...)
				}
			}
		}
	}

	return symbols
}

// flattenDocumentSymbols flattens nested document symbols
func (m *LSPManager) flattenDocumentSymbols(symbols []*lsp.DocumentSymbol) []lsp.DocumentSymbol {
	var result []lsp.DocumentSymbol
	for _, sym := range symbols {
		if sym != nil {
			result = append(result, *sym)
			if sym.Children != nil {
				result = append(result, m.flattenDocumentSymbols(sym.Children)...)
			}
		}
	}
	return result
}

// findSymbolRange finds the full range of a symbol from document symbols
func (m *LSPManager) findSymbolRange(name string, startLine int, docSymbols []lsp.DocumentSymbol) *lsp.Range {
	for _, sym := range docSymbols {
		// Match by name and approximate start line (within 2 lines tolerance)
		if sym.Name == name &&
			sym.Range.Start.Line >= startLine-2 &&
			sym.Range.Start.Line <= startLine+2 {
			return &sym.Range
		}
	}
	return nil
}

// sortSymbolsByScore sorts symbols by score in descending order
func sortSymbolsByScore(symbols []types.EnhancedSymbolInfo) {
	for i := 0; i < len(symbols)-1; i++ {
		for j := i + 1; j < len(symbols); j++ {
			if symbols[j].Score > symbols[i].Score {
				symbols[i], symbols[j] = symbols[j], symbols[i]
			}
		}
	}
}

// readSymbolCode reads the source code for a symbol from the file with character-level precision
func (m *LSPManager) readSymbolCode(filePath string, r lsp.Range) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()

		if lineNum == r.Start.Line && lineNum == r.End.Line {
			// Single line symbol - extract from start to end character
			if r.Start.Character < len(line) {
				endChar := r.End.Character
				if endChar > len(line) {
					endChar = len(line)
				}
				lines = append(lines, line[r.Start.Character:endChar])
			}
		} else if lineNum == r.Start.Line {
			// First line - extract from start character to end of line
			if r.Start.Character < len(line) {
				lines = append(lines, line[r.Start.Character:])
			}
		} else if lineNum == r.End.Line {
			// Last line - extract from beginning to end character
			endChar := r.End.Character
			if endChar > len(line) {
				endChar = len(line)
			}
			lines = append(lines, line[:endChar])
		} else if lineNum > r.Start.Line && lineNum < r.End.Line {
			// Middle lines - include entire line
			lines = append(lines, line)
		}

		lineNum++
		if lineNum > r.End.Line {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// Helper methods for occurrence-based search

// getScipStorage extracts SCIP storage from cache manager
func (m *LSPManager) getScipStorage() scip.SCIPDocumentStorage {
	if m.scipCache == nil {
		return nil
	}

	// For now, return nil and handle gracefully - need proper accessor method
	// TODO: Add proper accessor method to SimpleCacheManager or use direct interface
	return nil
}

// createEnrichedSymbolResult creates enriched result from SCIP symbol information
func (m *LSPManager) createEnrichedSymbolResult(ctx context.Context, scipStorage scip.SCIPDocumentStorage, symbolInfo *scip.SCIPSymbolInformation, query types.SymbolPatternQuery) (*EnhancedSymbolResult, error) {
	if symbolInfo == nil {
		return nil, fmt.Errorf("symbol info is nil")
	}

	// Convert SCIP symbol information to LSP format for consistency
	lspSymbol := lsp.SymbolInformation{
		Name: symbolInfo.DisplayName,
		Kind: m.mapSCIPKindToLSP(symbolInfo.Kind),
		Location: lsp.Location{
			URI:   "",          // Need to determine URI from symbol context
			Range: lsp.Range{}, // Need to extract from occurrences
		},
	}

	// Get occurrences for this symbol to extract location information
	occurrences, err := scipStorage.GetOccurrencesBySymbol(ctx, symbolInfo.Symbol)
	if err == nil && len(occurrences) > 0 {
		// Use the first occurrence for location (definition preferred)
		firstOcc := occurrences[0]
		// Find definition occurrence if available
		for _, occ := range occurrences {
			if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
				firstOcc = occ
				break
			}
		}

		// Convert SCIP range to LSP range
		lspSymbol.Location.Range = lsp.Range{
			Start: lsp.Position{
				Line:      int(firstOcc.Range.Start.Line),
				Character: int(firstOcc.Range.Start.Character),
			},
			End: lsp.Position{
				Line:      int(firstOcc.Range.End.Line),
				Character: int(firstOcc.Range.End.Character),
			},
		}
	}

	enrichedResult := &EnhancedSymbolResult{
		SymbolInformation: lspSymbol,
		SymbolID:          symbolInfo.Symbol,
		Documentation:     symbolInfo.Documentation,
		Signature:         symbolInfo.SignatureDocumentation.Text,
		Relationships:     symbolInfo.Relationships,
		OccurrenceCount:   len(occurrences),
		FilePath:          strings.TrimPrefix(lspSymbol.Location.URI, "file://"),
		LineNumber:        lspSymbol.Location.Range.Start.Line,
		EndLine:           lspSymbol.Location.Range.End.Line,
	}

	// Set role flags based on occurrences
	for _, occ := range occurrences {
		enrichedResult.OccurrenceRoles = enrichedResult.OccurrenceRoles.AddRole(occ.SymbolRoles)
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			enrichedResult.IsDefinition = true
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleGenerated) {
			enrichedResult.IsGenerated = true
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleTest) {
			enrichedResult.IsTest = true
		}
	}

	return enrichedResult, nil
}

// createEnrichedResultFromOccurrence creates enriched result from SCIP occurrence
func (m *LSPManager) createEnrichedResultFromOccurrence(ctx context.Context, scipStorage scip.SCIPDocumentStorage, occurrence scip.SCIPOccurrence, query types.SymbolPatternQuery) *EnhancedSymbolResult {
	// Get symbol information for this occurrence
	symbolInfo, err := scipStorage.GetSymbolInformation(ctx, occurrence.Symbol)
	if err != nil {
		return nil
	}

	// Create enriched result from occurrence and symbol info
	lspSymbol := lsp.SymbolInformation{
		Name: symbolInfo.DisplayName,
		Kind: m.mapSCIPKindToLSP(symbolInfo.Kind),
		Location: lsp.Location{
			URI: "", // Need to determine from context
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      int(occurrence.Range.Start.Line),
					Character: int(occurrence.Range.Start.Character),
				},
				End: lsp.Position{
					Line:      int(occurrence.Range.End.Line),
					Character: int(occurrence.Range.End.Character),
				},
			},
		},
	}

	enrichedResult := &EnhancedSymbolResult{
		SymbolInformation: lspSymbol,
		SymbolID:          occurrence.Symbol,
		OccurrenceRoles:   occurrence.SymbolRoles,
		Documentation:     symbolInfo.Documentation,
		Signature:         symbolInfo.SignatureDocumentation.Text,
		Relationships:     symbolInfo.Relationships,
		FilePath:          strings.TrimPrefix(lspSymbol.Location.URI, "file://"),
		LineNumber:        lspSymbol.Location.Range.Start.Line,
		EndLine:           lspSymbol.Location.Range.End.Line,
		IsDefinition:      occurrence.SymbolRoles.HasRole(types.SymbolRoleDefinition),
		IsGenerated:       occurrence.SymbolRoles.HasRole(types.SymbolRoleGenerated),
		IsTest:            occurrence.SymbolRoles.HasRole(types.SymbolRoleTest),
	}

	return enrichedResult
}

// isDuplicateEnrichedResult checks if enriched result is duplicate
func (m *LSPManager) isDuplicateEnrichedResult(existing []EnhancedSymbolResult, new EnhancedSymbolResult) bool {
	for _, result := range existing {
		if result.SymbolID == new.SymbolID && result.FilePath == new.FilePath &&
			result.LineNumber == new.LineNumber {
			return true
		}
	}
	return false
}

// convertToEnrichedResult converts LSP symbol to enriched result (for fallback cases)
func (m *LSPManager) convertToEnrichedResult(symbol lsp.SymbolInformation, query types.SymbolPatternQuery) *EnhancedSymbolResult {
	return &EnhancedSymbolResult{
		SymbolInformation: symbol,
		SymbolID:          fmt.Sprintf("lsp:%s", symbol.Name), // Simple ID for LSP symbols
		FilePath:          strings.TrimPrefix(symbol.Location.URI, "file://"),
		LineNumber:        symbol.Location.Range.Start.Line,
		EndLine:           symbol.Location.Range.End.Line,
		OccurrenceCount:   1, // Single occurrence from LSP
	}
}

// calculateEnhancedScore calculates enhanced scoring based on occurrence roles
func (m *LSPManager) calculateEnhancedScore(enrichedSymbol EnhancedSymbolResult, baseScore float64) float64 {
	finalScore := baseScore

	// Role-based bonuses
	if enrichedSymbol.IsDefinition {
		finalScore += 2.0 // Definitions get highest priority
	}
	if enrichedSymbol.OccurrenceRoles.HasRole(types.SymbolRoleReadAccess) {
		finalScore += 0.5
	}
	if enrichedSymbol.OccurrenceRoles.HasRole(types.SymbolRoleWriteAccess) {
		finalScore += 1.0
	}
	if enrichedSymbol.OccurrenceRoles.HasRole(types.SymbolRoleImport) {
		finalScore += 0.3
	}

	// Penalty for generated/test code
	if enrichedSymbol.IsGenerated {
		finalScore *= 0.7
	}
	if enrichedSymbol.IsTest {
		finalScore *= 0.8
	}

	// Bonus for symbols with documentation
	if len(enrichedSymbol.Documentation) > 0 {
		finalScore += 0.2
	}

	// Bonus for symbols with relationships (likely more important)
	if len(enrichedSymbol.Relationships) > 0 {
		finalScore += 0.3
	}

	// Bonus based on occurrence count (more references = more important)
	if enrichedSymbol.OccurrenceCount > 1 {
		finalScore += float64(enrichedSymbol.OccurrenceCount) * 0.1
	}

	return finalScore
}

// mapSCIPKindToLSP maps SCIP symbol kind to LSP symbol kind
func (m *LSPManager) mapSCIPKindToLSP(scipKind scip.SCIPSymbolKind) lsp.SymbolKind {
	switch scipKind {
	case scip.SCIPSymbolKindFile:
		return lsp.File
	case scip.SCIPSymbolKindModule:
		return lsp.Module
	case scip.SCIPSymbolKindNamespace:
		return lsp.Namespace
	case scip.SCIPSymbolKindPackage:
		return lsp.Package
	case scip.SCIPSymbolKindClass:
		return lsp.Class
	case scip.SCIPSymbolKindMethod:
		return lsp.Method
	case scip.SCIPSymbolKindProperty:
		return lsp.Property
	case scip.SCIPSymbolKindField:
		return lsp.Field
	case scip.SCIPSymbolKindConstructor:
		return lsp.Constructor
	case scip.SCIPSymbolKindEnum:
		return lsp.Enum
	case scip.SCIPSymbolKindInterface:
		return lsp.Interface
	case scip.SCIPSymbolKindFunction:
		return lsp.Function
	case scip.SCIPSymbolKindVariable:
		return lsp.Variable
	case scip.SCIPSymbolKindConstant:
		return lsp.Constant
	case scip.SCIPSymbolKindString:
		return lsp.String
	case scip.SCIPSymbolKindNumber:
		return lsp.Number
	case scip.SCIPSymbolKindBoolean:
		return lsp.Boolean
	case scip.SCIPSymbolKindArray:
		return lsp.Array
	case scip.SCIPSymbolKindObject:
		return lsp.Object
	case scip.SCIPSymbolKindKey:
		return lsp.Key
	case scip.SCIPSymbolKindNull:
		return lsp.Null
	case scip.SCIPSymbolKindEnumMember:
		return lsp.EnumMember
	case scip.SCIPSymbolKindStruct:
		return lsp.Struct
	case scip.SCIPSymbolKindEvent:
		return lsp.Event
	case scip.SCIPSymbolKindOperator:
		return lsp.Operator
	case scip.SCIPSymbolKindTypeParameter:
		return lsp.TypeParameter
	default:
		return lsp.Variable // Default fallback
	}
}
