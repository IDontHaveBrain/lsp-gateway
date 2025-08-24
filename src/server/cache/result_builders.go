package cache

import (
	"lsp-gateway/src/server/cache/search"
	"lsp-gateway/src/server/scip"
)

// Result building utilities - creates enhanced symbol results and occurrence info from SCIP data

// buildEnhancedSymbolResult creates an enhanced symbol result from SCIP data
func (m *SCIPCacheManager) buildEnhancedSymbolResult(symbolInfo *scip.SCIPSymbolInformation, occurrences []scip.SCIPOccurrence, query *EnhancedSymbolQuery) EnhancedSymbolResult {
	includeDocs := query != nil && query.IncludeDocumentation
	// Cache manager path computes basic scoring to support relevance-based sorting
	return search.BuildEnhancedSymbolResult(symbolInfo, occurrences, "", includeDocs, true)
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
