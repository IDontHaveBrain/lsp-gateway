package cache

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// Enhanced occurrence-based query types
type OccurrenceQueryType string

const (
	QueryTypeSymbols             OccurrenceQueryType = "symbols"
	QueryTypeWriteAccesses       OccurrenceQueryType = "write_accesses"
	QueryTypeImportStatements    OccurrenceQueryType = "import_statements"
	QueryTypeForwardDeclarations OccurrenceQueryType = "forward_declarations"
	QueryTypeGeneratedCode       OccurrenceQueryType = "generated_code"
	QueryTypeTestCode            OccurrenceQueryType = "test_code"
	QueryTypeImplementations     OccurrenceQueryType = "implementations"
	QueryTypeRelatedSymbols      OccurrenceQueryType = "related_symbols"
)

// EnhancedSymbolQuery represents an enhanced symbol query with occurrence-based filtering
type EnhancedSymbolQuery struct {
	// Basic query parameters
	Pattern     string `json:"pattern"`
	FilePattern string `json:"file_pattern,omitempty"`
	Language    string `json:"language,omitempty"`
	MaxResults  int    `json:"max_results,omitempty"`

	// Occurrence-based filtering
	QueryType      OccurrenceQueryType   `json:"query_type,omitempty"`
	SymbolRoles    []types.SymbolRole    `json:"symbol_roles,omitempty"`
	SymbolKinds    []scip.SCIPSymbolKind `json:"symbol_kinds,omitempty"`
	ExcludeRoles   []types.SymbolRole    `json:"exclude_roles,omitempty"`
	MinOccurrences int                   `json:"min_occurrences,omitempty"`
	MaxOccurrences int                   `json:"max_occurrences,omitempty"`

	// Advanced filtering
	IncludeDocumentation bool   `json:"include_documentation"`
	IncludeRelationships bool   `json:"include_relationships"`
	OnlyWithDefinition   bool   `json:"only_with_definition"`
	SortBy               string `json:"sort_by,omitempty"` // "relevance", "name", "occurrences", "kind"
	IncludeScore         bool   `json:"include_score"`
}

// EnhancedSymbolResult represents an enhanced symbol search result with occurrence metadata
type EnhancedSymbolResult struct {
	// Symbol information
	SymbolInfo  *scip.SCIPSymbolInformation `json:"symbol_info"`
	SymbolID    string                      `json:"symbol_id"`
	DisplayName string                      `json:"display_name"`
	Kind        scip.SCIPSymbolKind         `json:"kind"`

	// Occurrence metadata
	Occurrences      []scip.SCIPOccurrence `json:"occurrences,omitempty"`
	OccurrenceCount  int                   `json:"occurrence_count"`
	DefinitionCount  int                   `json:"definition_count"`
	ReferenceCount   int                   `json:"reference_count"`
	WriteAccessCount int                   `json:"write_access_count"`
	ReadAccessCount  int                   `json:"read_access_count"`

	// Role aggregation
	AllRoles        types.SymbolRole `json:"all_roles"`
	HasDefinition   bool             `json:"has_definition"`
	HasReferences   bool             `json:"has_references"`
	InGeneratedCode bool             `json:"in_generated_code"`
	InTestCode      bool             `json:"in_test_code"`

	// Documentation and relationships
	Documentation  []string                `json:"documentation,omitempty"`
	Signature      string                  `json:"signature,omitempty"`
	Relationships  []scip.SCIPRelationship `json:"relationships,omitempty"`
	RelatedSymbols []string                `json:"related_symbols,omitempty"`

	// Scoring and relevance
	RelevanceScore  float64 `json:"relevance_score"`
	PopularityScore float64 `json:"popularity_score"`
	FinalScore      float64 `json:"final_score"`

	// File distribution
	DocumentURIs []string `json:"document_uris,omitempty"`
	FileCount    int      `json:"file_count"`
}

// querySymbols handles symbol search queries with enhanced occurrence-based pattern matching and filtering
func (m *SimpleCacheManager) querySymbols(query *IndexQuery) []interface{} {

	// Use SCIP storage for occurrence-based queries if available
	if m.scipStorage != nil {
		return m.querySymbolsWithSCIPStorage(context.Background(), query)
	}

	// Fallback to legacy implementation
	return m.querySymbolsLegacy(query)
}

// querySymbolsWithSCIPStorage performs enhanced symbol search using SCIP storage
func (m *SimpleCacheManager) querySymbolsWithSCIPStorage(ctx context.Context, query *IndexQuery) []interface{} {
	common.LSPLogger.Debug("Performing occurrence-based symbol search with pattern: %s", query.Symbol)

	// Create enhanced query from index query
	enhancedQuery := m.convertToEnhancedQuery(query)

	// Get symbol information based on query type
	var symbolInfos []scip.SCIPSymbolInformation
	var err error

	switch enhancedQuery.QueryType {
	case QueryTypeSymbols:
		symbolInfos, err = m.scipStorage.SearchSymbols(ctx, enhancedQuery.Pattern, enhancedQuery.MaxResults*2)
	case QueryTypeWriteAccesses:
		return m.findWriteAccesses(ctx, enhancedQuery)
	case QueryTypeImportStatements:
		return m.findImportStatements(ctx, enhancedQuery)
	case QueryTypeForwardDeclarations:
		return m.findForwardDeclarations(ctx, enhancedQuery)
	case QueryTypeGeneratedCode:
		return m.findGeneratedCode(ctx, enhancedQuery)
	case QueryTypeTestCode:
		return m.findTestCode(ctx, enhancedQuery)
	case QueryTypeImplementations:
		return m.findImplementations(ctx, enhancedQuery)
	case QueryTypeRelatedSymbols:
		return m.findRelatedSymbols(ctx, enhancedQuery)
	default:
		// Default to symbol search
		symbolInfos, err = m.scipStorage.SearchSymbols(ctx, enhancedQuery.Pattern, enhancedQuery.MaxResults*2)
	}

	if err != nil {
		common.LSPLogger.Debug("SCIP symbol search failed: %v", err)
		return []interface{}{}
	}

	// Convert to enhanced results with occurrence data
	var enhancedResults []EnhancedSymbolResult
	for _, symbolInfo := range symbolInfos {
		result, err := m.createEnhancedResult(ctx, &symbolInfo, enhancedQuery)
		if err == nil && m.passesEnhancedFilters(result, enhancedQuery) {
			enhancedResults = append(enhancedResults, *result)
		}
	}

	// Sort results by relevance
	m.sortEnhancedResults(enhancedResults, enhancedQuery.SortBy)

	// Apply result limit
	if enhancedQuery.MaxResults > 0 && len(enhancedResults) > enhancedQuery.MaxResults {
		enhancedResults = enhancedResults[:enhancedQuery.MaxResults]
	}

	// Convert back to interface{} for compatibility
	results := make([]interface{}, len(enhancedResults))
	for i, result := range enhancedResults {
		results[i] = result
	}

	common.LSPLogger.Info("Occurrence-based symbol search returned %d enhanced results", len(results))
	return results
}

// querySymbolsLegacy handles symbol search queries with legacy pattern matching (fallback)
func (m *SimpleCacheManager) querySymbolsLegacy(query *IndexQuery) []interface{} {
	var results []interface{}

	// Legacy query functionality removed - return empty results

	// Return empty results when SCIP storage is not available

	return results
}

// queryDefinitions handles definition search queries
func (m *SimpleCacheManager) queryDefinitions(query *IndexQuery) []interface{} {
	var results []interface{}

	// Use SCIP storage for definition queries
	if m.scipStorage != nil {
		ctx := context.Background()

		// If a specific symbol is requested, look up its definitions using SCIP storage
		if query.Symbol != "" {
			if defOcc, err := m.scipStorage.GetDefinitionOccurrence(ctx, query.Symbol); err == nil {
				results = append(results, *defOcc)
			}
			return results
		}
	}

	// If no definitions found in symbols, fallback to general symbol query
	if len(results) == 0 {
		return m.querySymbols(query)
	}

	return results
}

// queryReferences handles reference search queries
func (m *SimpleCacheManager) queryReferences(query *IndexQuery) []interface{} {
	var results []interface{}

	// Use SCIP storage for reference queries
	if m.scipStorage != nil {
		ctx := context.Background()

		// If a specific symbol is requested, look up its references using SCIP storage
		if query.Symbol != "" {
			if refOccs, err := m.scipStorage.GetReferenceOccurrences(ctx, query.Symbol); err == nil {
				for _, occ := range refOccs {
					results = append(results, occ)
				}
			}
			return results
		}
	}

	return results
}

// queryWorkspaceSymbols handles workspace symbol queries
func (m *SimpleCacheManager) queryWorkspaceSymbols(query *IndexQuery) []interface{} {
	return m.querySymbols(query)
}

// enhanceSymbolWithIDFormats enhances symbol result with both simple and SCIP ID formats during transition
func (m *SimpleCacheManager) enhanceSymbolWithIDFormats(symbol *SCIPSymbol, queryIsSimpleID bool, queryIsSCIPID bool) map[string]interface{} {
	// Create base result from the SCIPSymbol
	result := map[string]interface{}{
		"symbol_info":          symbol.SymbolInfo,
		"language":             symbol.Language,
		"score":                symbol.Score,
		"full_range":           symbol.FullRange,
		"documentation":        symbol.Documentation,
		"signature":            symbol.Signature,
		"related_symbols":      symbol.RelatedSymbols,
		"definition_locations": symbol.DefinitionLocations,
		"reference_locations":  symbol.ReferenceLocations,
		"usage_count":          symbol.UsageCount,
		"metadata":             symbol.Metadata,
	}

	// Generate both ID formats
	simpleID, scipID := m.getSymbolIDFormats(symbol)

	// Add ID format information for transition period
	result["id_formats"] = map[string]interface{}{
		"simple": simpleID,
		"scip":   scipID,
	}

	// Indicate which format was queried
	if queryIsSimpleID {
		result["query_format"] = "simple"
		result["primary_id"] = simpleID
		result["alternative_id"] = scipID
	} else if queryIsSCIPID {
		result["query_format"] = "scip"
		result["primary_id"] = scipID
		result["alternative_id"] = simpleID
	} else {
		result["query_format"] = "pattern"
		result["primary_id"] = simpleID // Default to simple for pattern queries
		result["alternative_id"] = scipID
	}

	return result
}

// matchFilePattern checks if a file path matches a pattern (supports glob and regex)
func (m *SimpleCacheManager) matchFilePattern(filePath, pattern string) bool {
	// Handle special case for current directory
	if pattern == "." || pattern == "./" {
		return true
	}

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		return strings.Contains(filePath, pattern)
	}

	// Handle glob patterns
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
		// Convert glob to regex
		regexPattern := strings.ReplaceAll(pattern, "**", "@@DOUBLESTAR@@")
		regexPattern = strings.ReplaceAll(regexPattern, ".", "\\.")
		regexPattern = strings.ReplaceAll(regexPattern, "@@DOUBLESTAR@@", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "*", "[^/]*")
		regexPattern = strings.ReplaceAll(regexPattern, "?", ".")

		if !strings.Contains(pattern, "/") {
			// For patterns like "*.go", match at any depth
			regexPattern = "(^|.*/)" + regexPattern + "$"
		} else if !strings.HasPrefix(pattern, "**/") {
			// For patterns with path like "test/*.go"
			regexPattern = "(^|.*/)" + regexPattern + "$"
		}

		if re, err := regexp.Compile(regexPattern); err == nil {
			return re.MatchString(filePath)
		}
	}

	// Try as regex
	if re, err := regexp.Compile(pattern); err == nil {
		return re.MatchString(filePath)
	}

	// Fall back to simple contains
	return strings.Contains(filePath, pattern)
}

// isSimpleIDFormat checks if a string follows the simple ID format: "language:symbolName"
func (m *SimpleCacheManager) isSimpleIDFormat(id string) bool {
	// Simple format: "language:symbolName" (e.g., "go:TestFunction")
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return false
	}

	language := parts[0]
	symbolName := parts[1]

	// Check if language is a valid supported language
	supportedLanguages := []string{"go", "python", "javascript", "typescript", "java"}
	for _, lang := range supportedLanguages {
		if language == lang {
			return symbolName != "" // Must have non-empty symbol name
		}
	}

	return false
}

// isSCIPIDFormat checks if a string follows the SCIP ID format: "scip-language package version descriptor"
func (m *SimpleCacheManager) isSCIPIDFormat(id string) bool {
	// SCIP format: "scip-language package version descriptor"
	if !strings.HasPrefix(id, "scip-") {
		return false
	}

	// Remove "scip-" prefix and split by spaces
	remaining := strings.TrimPrefix(id, "scip-")
	parts := strings.Fields(remaining)

	// Must have at least 4 parts: language, package, version, descriptor
	if len(parts) < 4 {
		return false
	}

	language := parts[0]
	// parts[1] is package name
	// parts[2] is version
	// parts[3:] is descriptor (can be multiple words)

	// Check if language is a valid supported language
	supportedLanguages := []string{"go", "python", "javascript", "typescript", "java"}
	for _, lang := range supportedLanguages {
		if language == lang {
			return true
		}
	}

	return false
}

// convertSimpleToSCIP converts simple ID format to SCIP format
// Simple: "go:TestFunction" -> SCIP: "scip-go main v1.0.0 TestFunction"
func (m *SimpleCacheManager) convertSimpleToSCIP(simpleID string) string {
	parts := strings.Split(simpleID, ":")
	if len(parts) != 2 {
		return simpleID // Return original if not valid simple format
	}

	language := parts[0]
	symbolName := parts[1]

	// Default package and version for transition period
	defaultPackage := "main"
	defaultVersion := "v1.0.0"

	return fmt.Sprintf("scip-%s %s %s %s", language, defaultPackage, defaultVersion, symbolName)
}

// convertSCIPToSimple converts SCIP ID format to simple format
// SCIP: "scip-go main v1.0.0 TestFunction" -> Simple: "go:TestFunction"
func (m *SimpleCacheManager) convertSCIPToSimple(scipID string) string {
	if !strings.HasPrefix(scipID, "scip-") {
		return scipID // Return original if not valid SCIP format
	}

	remaining := strings.TrimPrefix(scipID, "scip-")
	parts := strings.Fields(remaining)

	if len(parts) < 4 {
		return scipID // Return original if not enough parts
	}

	language := parts[0]
	// Skip package (parts[1]) and version (parts[2])
	descriptor := strings.Join(parts[3:], " ") // Descriptor can be multiple words

	return fmt.Sprintf("%s:%s", language, descriptor)
}

// matchesIDQuery checks if a symbol matches an ID-based query (simple or SCIP format)
func (m *SimpleCacheManager) matchesIDQuery(symbol *SCIPSymbol, queryID string, isSimpleID bool, isSCIPID bool) bool {
	// Generate both simple and SCIP IDs for this symbol
	simpleID := fmt.Sprintf("%s:%s", symbol.Language, symbol.SymbolInfo.Name)
	scipID := m.convertSimpleToSCIP(simpleID)

	if isSimpleID {
		// Query is in simple format, check direct match and converted SCIP match
		return queryID == simpleID || m.convertSCIPToSimple(queryID) == simpleID
	}

	if isSCIPID {
		// Query is in SCIP format, check direct match and converted simple match
		return queryID == scipID || m.convertSimpleToSCIP(m.convertSCIPToSimple(queryID)) == scipID
	}

	return false
}

// getSymbolIDFormats returns both simple and SCIP ID formats for a symbol
func (m *SimpleCacheManager) getSymbolIDFormats(symbol *SCIPSymbol) (string, string) {
	simpleID := fmt.Sprintf("%s:%s", symbol.Language, symbol.SymbolInfo.Name)
	scipID := m.convertSimpleToSCIP(simpleID)
	return simpleID, scipID
}

// Helper methods for enhanced occurrence-based search

// convertToEnhancedQuery converts IndexQuery to EnhancedSymbolQuery
func (m *SimpleCacheManager) convertToEnhancedQuery(query *IndexQuery) *EnhancedSymbolQuery {
	enhanced := &EnhancedSymbolQuery{
		Pattern:    query.Symbol,
		Language:   query.Language,
		MaxResults: 100,              // Default
		QueryType:  QueryTypeSymbols, // Default
		SortBy:     "relevance",      // Default
	}

	// Extract enhanced parameters from filters
	if query.Filters != nil {
		if filePattern, ok := query.Filters["filePattern"].(string); ok {
			enhanced.FilePattern = filePattern
		}
		if maxResults, ok := query.Filters["maxResults"].(int); ok && maxResults > 0 {
			enhanced.MaxResults = maxResults
		}
		if queryType, ok := query.Filters["queryType"].(string); ok {
			enhanced.QueryType = OccurrenceQueryType(queryType)
		}
		if sortBy, ok := query.Filters["sortBy"].(string); ok {
			enhanced.SortBy = sortBy
		}
		if includeDoc, ok := query.Filters["includeDocumentation"].(bool); ok {
			enhanced.IncludeDocumentation = includeDoc
		}
		if includeRel, ok := query.Filters["includeRelationships"].(bool); ok {
			enhanced.IncludeRelationships = includeRel
		}
		if onlyDef, ok := query.Filters["onlyWithDefinition"].(bool); ok {
			enhanced.OnlyWithDefinition = onlyDef
		}
		if minOcc, ok := query.Filters["minOccurrences"].(int); ok {
			enhanced.MinOccurrences = minOcc
		}
		if maxOcc, ok := query.Filters["maxOccurrences"].(int); ok {
			enhanced.MaxOccurrences = maxOcc
		}
	}

	return enhanced
}

// createEnhancedResult creates an enhanced symbol result with occurrence data
func (m *SimpleCacheManager) createEnhancedResult(ctx context.Context, symbolInfo *scip.SCIPSymbolInformation, query *EnhancedSymbolQuery) (*EnhancedSymbolResult, error) {
	if symbolInfo == nil {
		return nil, fmt.Errorf("symbol info is nil")
	}

	// Get all occurrences for this symbol
	occurrences, err := m.scipStorage.GetOccurrencesBySymbol(ctx, symbolInfo.Symbol)
	if err != nil {
		// Non-fatal error, continue with empty occurrences
		occurrences = []scip.SCIPOccurrence{}
	}

	// Create enhanced result
	result := &EnhancedSymbolResult{
		SymbolInfo:      symbolInfo,
		SymbolID:        symbolInfo.Symbol,
		DisplayName:     symbolInfo.DisplayName,
		Kind:            symbolInfo.Kind,
		OccurrenceCount: len(occurrences),
		Documentation:   symbolInfo.Documentation,
		Signature:       symbolInfo.SignatureDocumentation.Text,
		Relationships:   symbolInfo.Relationships,
	}

	// Analyze occurrences to extract role metadata
	var documentURIs []string
	seenURIs := make(map[string]bool)

	for _, occurrence := range occurrences {
		// Aggregate roles\n\t\tresult.AllRoles = result.AllRoles.AddRole(occurrence.SymbolRoles)

		// Count role types
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			result.DefinitionCount++
			result.HasDefinition = true
		}
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleReadAccess) {
			result.ReadAccessCount++
		}
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			result.WriteAccessCount++
		}
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleGenerated) {
			result.InGeneratedCode = true
		}
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleTest) {
			result.InTestCode = true
		}

		// Track document URIs (would need to be extracted from context)
		// For now, use placeholder logic
		uri := fmt.Sprintf("doc-%d", len(seenURIs)) // Placeholder
		if !seenURIs[uri] {
			seenURIs[uri] = true
			documentURIs = append(documentURIs, uri)
		}
	}

	result.ReferenceCount = result.OccurrenceCount - result.DefinitionCount
	result.HasReferences = result.ReferenceCount > 0
	result.DocumentURIs = documentURIs
	result.FileCount = len(documentURIs)

	// Get related symbols
	if query.IncludeRelationships && len(symbolInfo.Relationships) > 0 {
		for _, rel := range symbolInfo.Relationships {
			if !m.containsString(result.RelatedSymbols, rel.Symbol) {
				result.RelatedSymbols = append(result.RelatedSymbols, rel.Symbol)
			}
		}
	}

	// Calculate scores
	result.RelevanceScore = m.calculateRelevanceScore(result, query)
	result.PopularityScore = m.calculatePopularityScore(result)
	result.FinalScore = result.RelevanceScore + (result.PopularityScore * 0.3)

	// Include full occurrences if requested (for detailed analysis)
	if query.MaxResults <= 10 { // Only for small result sets to avoid memory issues
		result.Occurrences = occurrences
	}

	return result, nil
}

// findWriteAccesses finds all write access occurrences
func (m *SimpleCacheManager) findWriteAccesses(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding write accesses for pattern: %s", query.Pattern)

	// Search for occurrences with write access role
	occurrences, err := m.scipStorage.SearchOccurrences(ctx, query.Pattern, query.MaxResults*2)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	for _, occurrence := range occurrences {
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			symbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, occurrence.Symbol)
			if err == nil {
				result := m.createOccurrenceResult(occurrence, symbolInfo)
				results = append(results, result)
			}
		}
	}

	return results
}

// findImportStatements finds all import statement occurrences
func (m *SimpleCacheManager) findImportStatements(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding import statements for pattern: %s", query.Pattern)

	occurrences, err := m.scipStorage.SearchOccurrences(ctx, query.Pattern, query.MaxResults*2)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	for _, occurrence := range occurrences {
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleImport) {
			symbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, occurrence.Symbol)
			if err == nil {
				result := m.createOccurrenceResult(occurrence, symbolInfo)
				results = append(results, result)
			}
		}
	}

	return results
}

// findForwardDeclarations finds all forward declaration occurrences
func (m *SimpleCacheManager) findForwardDeclarations(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding forward declarations for pattern: %s", query.Pattern)

	occurrences, err := m.scipStorage.SearchOccurrences(ctx, query.Pattern, query.MaxResults*2)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	for _, occurrence := range occurrences {
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleForwardDefinition) {
			symbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, occurrence.Symbol)
			if err == nil {
				result := m.createOccurrenceResult(occurrence, symbolInfo)
				results = append(results, result)
			}
		}
	}

	return results
}

// findGeneratedCode finds all occurrences in generated code
func (m *SimpleCacheManager) findGeneratedCode(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding generated code for pattern: %s", query.Pattern)

	occurrences, err := m.scipStorage.SearchOccurrences(ctx, query.Pattern, query.MaxResults*2)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	for _, occurrence := range occurrences {
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleGenerated) {
			symbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, occurrence.Symbol)
			if err == nil {
				result := m.createOccurrenceResult(occurrence, symbolInfo)
				results = append(results, result)
			}
		}
	}

	return results
}

// findTestCode finds all occurrences in test code
func (m *SimpleCacheManager) findTestCode(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding test code for pattern: %s", query.Pattern)

	occurrences, err := m.scipStorage.SearchOccurrences(ctx, query.Pattern, query.MaxResults*2)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	for _, occurrence := range occurrences {
		if occurrence.SymbolRoles.HasRole(types.SymbolRoleTest) {
			symbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, occurrence.Symbol)
			if err == nil {
				result := m.createOccurrenceResult(occurrence, symbolInfo)
				results = append(results, result)
			}
		}
	}

	return results
}

// findImplementations finds implementation relationships
func (m *SimpleCacheManager) findImplementations(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding implementations for pattern: %s", query.Pattern)

	// First find symbols matching the pattern
	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, query.Pattern, query.MaxResults)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	for _, symbolInfo := range symbolInfos {
		// Get implementations for this symbol
		implementations, err := m.scipStorage.GetImplementations(ctx, symbolInfo.Symbol)
		if err == nil {
			for _, implOcc := range implementations {
				implSymbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, implOcc.Symbol)
				if err == nil {
					result := m.createOccurrenceResult(implOcc, implSymbolInfo)
					results = append(results, result)
				}
			}
		}
	}

	return results
}

// findRelatedSymbols finds symbols related through relationships
func (m *SimpleCacheManager) findRelatedSymbols(ctx context.Context, query *EnhancedSymbolQuery) []interface{} {
	common.LSPLogger.Debug("Finding related symbols for pattern: %s", query.Pattern)

	// First find symbols matching the pattern
	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, query.Pattern, query.MaxResults)
	if err != nil {
		return []interface{}{}
	}

	var results []interface{}
	relatedSymbolIDs := make(map[string]bool)

	for _, symbolInfo := range symbolInfos {
		// Get relationships for this symbol
		relationships, err := m.scipStorage.GetSymbolRelationships(ctx, symbolInfo.Symbol)
		if err == nil {
			for _, rel := range relationships {
				if !relatedSymbolIDs[rel.Symbol] {
					relatedSymbolIDs[rel.Symbol] = true
					relatedSymbolInfo, err := m.scipStorage.GetSymbolInformation(ctx, rel.Symbol)
					if err == nil {
						result, err := m.createEnhancedResult(ctx, relatedSymbolInfo, query)
						if err == nil {
							results = append(results, *result)
						}
					}
				}
			}
		}
	}

	return results
}

// createOccurrenceResult creates a result from a single occurrence
func (m *SimpleCacheManager) createOccurrenceResult(occurrence scip.SCIPOccurrence, symbolInfo *scip.SCIPSymbolInformation) map[string]interface{} {
	return map[string]interface{}{
		"symbol_info":   symbolInfo,
		"occurrence":    occurrence,
		"symbol_id":     occurrence.Symbol,
		"display_name":  symbolInfo.DisplayName,
		"symbol_roles":  occurrence.SymbolRoles,
		"is_definition": occurrence.SymbolRoles.HasRole(types.SymbolRoleDefinition),
		"is_reference":  !occurrence.SymbolRoles.HasRole(types.SymbolRoleDefinition),
		"is_write":      occurrence.SymbolRoles.HasRole(types.SymbolRoleWriteAccess),
		"is_read":       occurrence.SymbolRoles.HasRole(types.SymbolRoleReadAccess),
		"is_import":     occurrence.SymbolRoles.HasRole(types.SymbolRoleImport),
		"is_generated":  occurrence.SymbolRoles.HasRole(types.SymbolRoleGenerated),
		"is_test":       occurrence.SymbolRoles.HasRole(types.SymbolRoleTest),
		"documentation": symbolInfo.Documentation,
		"relationships": symbolInfo.Relationships,
		"range":         occurrence.Range,
		"timestamp":     time.Now(),
	}
}

// passesEnhancedFilters checks if a result passes the enhanced filters
func (m *SimpleCacheManager) passesEnhancedFilters(result *EnhancedSymbolResult, query *EnhancedSymbolQuery) bool {
	// Check minimum/maximum occurrence count
	if query.MinOccurrences > 0 && result.OccurrenceCount < query.MinOccurrences {
		return false
	}
	if query.MaxOccurrences > 0 && result.OccurrenceCount > query.MaxOccurrences {
		return false
	}

	// Check if definition is required
	if query.OnlyWithDefinition && !result.HasDefinition {
		return false
	}

	// Check symbol kinds
	if len(query.SymbolKinds) > 0 {
		found := false
		for _, kind := range query.SymbolKinds {
			if result.Kind == kind {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check required roles
	if len(query.SymbolRoles) > 0 {
		for _, role := range query.SymbolRoles {
			if !result.AllRoles.HasRole(role) {
				return false
			}
		}
	}

	// Check excluded roles
	if len(query.ExcludeRoles) > 0 {
		for _, role := range query.ExcludeRoles {
			if result.AllRoles.HasRole(role) {
				return false
			}
		}
	}

	return true
}

// sortEnhancedResults sorts results based on the specified criteria
func (m *SimpleCacheManager) sortEnhancedResults(results []EnhancedSymbolResult, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(results, func(i, j int) bool {
			return results[i].DisplayName < results[j].DisplayName
		})
	case "occurrences":
		sort.Slice(results, func(i, j int) bool {
			return results[i].OccurrenceCount > results[j].OccurrenceCount
		})
	case "kind":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Kind < results[j].Kind
		})
	case "relevance":
		fallthrough
	default:
		sort.Slice(results, func(i, j int) bool {
			return results[i].FinalScore > results[j].FinalScore
		})
	}
}

// calculateRelevanceScore calculates relevance score based on query match
func (m *SimpleCacheManager) calculateRelevanceScore(result *EnhancedSymbolResult, query *EnhancedSymbolQuery) float64 {
	score := 1.0

	// Exact name match gets highest score
	if strings.EqualFold(result.DisplayName, query.Pattern) {
		score += 5.0
	} else if strings.Contains(strings.ToLower(result.DisplayName), strings.ToLower(query.Pattern)) {
		score += 2.0
	}

	// Bonus for having definition
	if result.HasDefinition {
		score += 3.0
	}

	// Bonus for having documentation
	if len(result.Documentation) > 0 {
		score += 1.0
	}

	// Bonus for having relationships
	if len(result.Relationships) > 0 {
		score += 0.5
	}

	// Penalty for generated/test code
	if result.InGeneratedCode {
		score *= 0.7
	}
	if result.InTestCode {
		score *= 0.8
	}

	return score
}

// calculatePopularityScore calculates popularity score based on usage
func (m *SimpleCacheManager) calculatePopularityScore(result *EnhancedSymbolResult) float64 {
	score := 0.0

	// More occurrences = more popular
	score += float64(result.OccurrenceCount) * 0.1

	// More files = more popular
	score += float64(result.FileCount) * 0.5

	// Write accesses indicate active usage
	score += float64(result.WriteAccessCount) * 0.3

	// Relationships indicate importance
	score += float64(len(result.RelatedSymbols)) * 0.2

	return score
}

// containsString checks if a string slice contains a specific string
func (m *SimpleCacheManager) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
