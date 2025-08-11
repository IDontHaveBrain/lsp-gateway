package cache

import (
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// Result building utilities - creates enhanced symbol results and occurrence info from SCIP data

// buildEnhancedSymbolResult creates an enhanced symbol result from SCIP data
func (m *SCIPCacheManager) buildEnhancedSymbolResult(symbolInfo *scip.SCIPSymbolInformation, occurrences []scip.SCIPOccurrence, query *EnhancedSymbolQuery) EnhancedSymbolResult {
	result := EnhancedSymbolResult{
		SymbolInfo:      symbolInfo,
		SymbolID:        symbolInfo.Symbol,
		DisplayName:     symbolInfo.DisplayName,
		Kind:            symbolInfo.Kind,
		OccurrenceCount: len(occurrences),
	}

	// Include occurrences if requested
	if query != nil && (query.IncludeDocumentation || len(occurrences) < 50) { // Avoid large arrays
		result.Occurrences = occurrences
	}

	// Count roles and collect metadata
	fileSet := make(map[string]bool)
	var allRoles types.SymbolRole

	for _, occ := range occurrences {
		allRoles |= occ.SymbolRoles

		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			result.DefinitionCount++
			result.HasDefinition = true
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) {
			result.ReadAccessCount++
			result.HasReferences = true
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			result.WriteAccessCount++
		}

		// Extract document URI from occurrence
		docURI := m.extractURIFromOccurrence(&occ)
		if docURI != "" {
			fileSet[docURI] = true
		}
	}

	result.AllRoles = allRoles
	result.ReferenceCount = result.ReadAccessCount + result.WriteAccessCount
	result.FileCount = len(fileSet)
	result.DocumentURIs = make([]string, 0, len(fileSet))
	for uri := range fileSet {
		result.DocumentURIs = append(result.DocumentURIs, uri)
	}

	// Include documentation and relationships if requested
	if query != nil && query.IncludeDocumentation {
		result.Documentation = symbolInfo.Documentation
		if query.IncludeRelationships {
			// Note: Relationships not available in simplified interface
			// result.Relationships = relationships
		}
	}

	// Calculate basic scoring
	result.PopularityScore = float64(result.OccurrenceCount)
	result.RelevanceScore = 1.0 // Basic relevance - could be enhanced with pattern matching
	result.FinalScore = result.RelevanceScore * (1.0 + result.PopularityScore/100.0)

	return result
}

// buildOccurrenceInfo creates occurrence info with context
func (m *SCIPCacheManager) buildOccurrenceInfo(occ *scip.SCIPOccurrence, docURI string) SCIPOccurrenceInfo {
	return SCIPOccurrenceInfo{
		Occurrence:  *occ,
		DocumentURI: docURI,
		SymbolRoles: occ.SymbolRoles,
		SyntaxKind:  occ.SyntaxKind,
		LineNumber:  occ.Range.Start.Line,
		Score:       1.0, // Basic score
		// Context could be added by reading file content around the occurrence
	}
}
