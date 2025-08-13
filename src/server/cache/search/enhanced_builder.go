package search

import (
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// BuildEnhancedSymbolResult constructs an EnhancedSymbolResult from SCIP data.
// signature: optional formatted signature string
// includeDocumentation: when true, copies documentation from symbolInfo
// computeScore: when true, fills basic scoring fields using occurrence counts
func BuildEnhancedSymbolResult(symbolInfo *scip.SCIPSymbolInformation, occurrences []scip.SCIPOccurrence, signature string, includeDocumentation bool, computeScore bool) EnhancedSymbolResult {
	definitionCount := 0
	referenceCount := 0
	writeAccessCount := 0
	readAccessCount := 0

	for _, occ := range occurrences {
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			definitionCount++
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			writeAccessCount++
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) {
			readAccessCount++
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			referenceCount++
		}
	}

	result := EnhancedSymbolResult{
		SymbolInfo:       symbolInfo,
		SymbolID:         symbolInfo.Symbol,
		DisplayName:      symbolInfo.DisplayName,
		Kind:             symbolInfo.Kind,
		Occurrences:      occurrences,
		OccurrenceCount:  len(occurrences),
		DefinitionCount:  definitionCount,
		ReferenceCount:   referenceCount,
		WriteAccessCount: writeAccessCount,
		ReadAccessCount:  readAccessCount,
		Documentation:    nil,
		Signature:        signature,
	}

	if includeDocumentation {
		result.Documentation = symbolInfo.Documentation
	}

	if computeScore {
		result.UsageFrequency = result.OccurrenceCount
		result.Relevance = 1.0
		result.Score = result.Relevance * (1.0 + float64(result.UsageFrequency)/100.0)
	}

	return result
}
