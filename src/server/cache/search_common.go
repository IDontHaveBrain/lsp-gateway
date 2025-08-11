package cache

import (
	"context"
	"sort"
	"strings"

	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils/filepattern"
)

// Common search utilities - shared helper functions for search operations

// extractURIFromOccurrence extracts document URI from a SCIP occurrence
func (m *SCIPCacheManager) extractURIFromOccurrence(occ *scip.SCIPOccurrence) string {
	if m.scipStorage == nil || occ == nil {
		return ""
	}

	docs, err := m.scipStorage.ListDocuments(context.Background())
	if err != nil {
		return ""
	}

	for _, docURI := range docs {
		doc, docErr := m.scipStorage.GetDocument(context.Background(), docURI)
		if docErr != nil || doc == nil {
			continue
		}
		for _, docOcc := range doc.Occurrences {
			if docOcc.Symbol == occ.Symbol &&
				docOcc.Range.Start.Line == occ.Range.Start.Line &&
				docOcc.Range.Start.Character == occ.Range.Start.Character &&
				docOcc.Range.End.Line == occ.Range.End.Line &&
				docOcc.Range.End.Character == occ.Range.End.Character {
				return docURI
			}
		}
	}

	return ""
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
			return results[i].RelevanceScore > results[j].RelevanceScore
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
		// Default to final score
		sort.Slice(results, func(i, j int) bool {
			return results[i].FinalScore > results[j].FinalScore
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

// convertURIToFilePath converts a URI to a file path
func (m *SCIPCacheManager) convertURIToFilePath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	// Remove the file:// prefix
	path := strings.TrimPrefix(uri, "file://")

	// On Windows, file URIs look like file:///C:/path/to/file
	// After removing file://, we have /C:/path/to/file
	// We need to remove the leading slash for Windows absolute paths
	if strings.HasPrefix(path, "/") && len(path) > 2 && path[2] == ':' {
		path = path[1:]
	}

	return path
}
