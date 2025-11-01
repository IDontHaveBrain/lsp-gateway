package cache

import (
	"lsp-gateway/src/server/cache/search"
	"lsp-gateway/src/server/scip"
)

// Result building utilities - creates enhanced symbol results and occurrence info from SCIP data

// buildOccurrenceInfo creates occurrence info with context
func (m *SCIPCacheManager) buildOccurrenceInfo(occ *scip.SCIPOccurrence, docURI string) search.SCIPOccurrenceInfo {
	return search.SCIPOccurrenceInfo{
		Occurrence:  *occ,
		DocumentURI: docURI,
		SymbolRoles: occ.SymbolRoles,
		SyntaxKind:  occ.SyntaxKind,
		LineNumber:  occ.Range.Start.Line,
		Score:       1.0, // Basic score
		// Context could be added by reading file content around the occurrence
	}
}
