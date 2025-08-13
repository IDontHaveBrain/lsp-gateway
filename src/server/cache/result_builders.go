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

    // Include occurrences if requested or small
    if query != nil && (query.IncludeDocumentation || len(occurrences) < 50) {
        result.Occurrences = occurrences
    }

    // Count roles
    for _, occ := range occurrences {
        if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
            result.DefinitionCount++
        }
        if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) {
            result.ReadAccessCount++
        }
        if occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
            result.WriteAccessCount++
        }
    }
    result.ReferenceCount = result.ReadAccessCount + result.WriteAccessCount

    // Basic metadata
    result.FileCount = 0 // not tracked here; could be derived from occurrences' documents
    if query != nil && query.IncludeDocumentation {
        result.Documentation = symbolInfo.Documentation
    }

    // Scoring: map to search type fields
    result.UsageFrequency = result.OccurrenceCount
    result.Relevance = 1.0
    result.Score = result.Relevance * (1.0 + float64(result.UsageFrequency)/100.0)

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
