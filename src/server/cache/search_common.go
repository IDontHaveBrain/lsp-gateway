package cache

import (
	"sort"

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
func (m *SCIPCacheManager) sortEnhancedResults(results []EnhancedSymbolResult, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(results, func(i, j int) bool {
			return results[i].DisplayName < results[j].DisplayName
		})
	case "relevance":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Relevance > results[j].Relevance
		})
	case "occurrences":
		sort.Slice(results, func(i, j int) bool {
			return results[i].OccurrenceCount > results[j].OccurrenceCount
		})
	case "kind":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Kind < results[j].Kind
		})
	default:
		// Default to score
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}
}

// sortOccurrenceResults sorts occurrence results by the specified criteria
func (m *SCIPCacheManager) sortOccurrenceResults(results []SCIPOccurrenceInfo, sortBy string) {
	switch sortBy {
	case "location":
		sort.Slice(results, func(i, j int) bool {
			if results[i].DocumentURI != results[j].DocumentURI {
				return results[i].DocumentURI < results[j].DocumentURI
			}
			if results[i].LineNumber != results[j].LineNumber {
				return results[i].LineNumber < results[j].LineNumber
			}
			return results[i].Occurrence.Range.Start.Character < results[j].Occurrence.Range.Start.Character
		})
	case "file":
		sort.Slice(results, func(i, j int) bool {
			return results[i].DocumentURI < results[j].DocumentURI
		})
	case "relevance":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	default:
		// Default to location
		sort.Slice(results, func(i, j int) bool {
			if results[i].DocumentURI != results[j].DocumentURI {
				return results[i].DocumentURI < results[j].DocumentURI
			}
			return results[i].LineNumber < results[j].LineNumber
		})
	}
}
