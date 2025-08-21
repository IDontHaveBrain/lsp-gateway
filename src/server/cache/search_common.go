package cache

import (
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils/filepattern"
)

// Common search utilities - shared helper functions for search operations

// extractURIFromOccurrence extracts document URI from a SCIP occurrence
func (m *SCIPCacheManager) extractURIFromOccurrence(occ *scip.SCIPOccurrence) string {
	if m.scipStorage == nil || occ == nil {
		return ""
	}

	uri, err := scip.GetDocumentURIFromOccurrence(m.scipStorage, occ)
	if err != nil {
		return ""
	}
	return uri
}

// matchFilePattern checks if a file URI matches a pattern
func (m *SCIPCacheManager) matchFilePattern(uri, pattern string) bool {
	return filepattern.Match(uri, pattern)
}

// sortEnhancedResults sorts enhanced symbol results by the specified criteria
// Note: sorting helpers are centralized in search.Sort* functions
