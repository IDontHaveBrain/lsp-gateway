package cache

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// Reference search operations - handles reference searching with standard and enhanced results

// SearchReferences provides interface-compliant reference search
func (m *SCIPCacheManager) SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	return m.WithSliceResult(func() ([]interface{}, error) {
		if maxResults <= 0 {
			maxResults = 100
		}

		result := m.WithIndexReadLock(func() interface{} {
			return m.searchReferencesInternal(ctx, symbolName, filePattern, maxResults)
		})
		return result.([]interface{}), nil
	})
}

// searchReferencesInternal contains the actual reference search logic without guards or locks
func (m *SCIPCacheManager) searchReferencesInternal(ctx context.Context, symbolName string, filePattern string, maxResults int) []interface{} {

	// First, try to find symbols by exact or partial name match
	var matchedSymbols []scip.SCIPSymbolInformation

	// Try exact match first
	if symbols, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
		symbolInfos, _ := symbols.SearchSymbols(ctx, symbolName, maxResults)
		matchedSymbols = append(matchedSymbols, symbolInfos...)
	}

	// If no exact matches, try searching with wildcards
	if len(matchedSymbols) == 0 {
		if symbols, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
			// Try searching with pattern matching
			patternSearch := ".*" + symbolName + ".*"
			symbolInfos, _ := symbols.SearchSymbols(ctx, patternSearch, maxResults)
			matchedSymbols = append(matchedSymbols, symbolInfos...)
		}
	}

	var allReferences []interface{}
	fileSet := make(map[string]bool)

	// For each matched symbol, get its references
	for _, symbolInfo := range matchedSymbols {
		// Get references with documents for better performance
		if refOccs, err := m.scipStorage.GetReferencesWithDocuments(ctx, symbolInfo.Symbol); err == nil {
			for _, occWithDoc := range refOccs {
				docURI := occWithDoc.DocumentURI
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					occInfo := m.buildOccurrenceInfo(&occWithDoc.SCIPOccurrence, docURI)
					allReferences = append(allReferences, occInfo)
					fileSet[docURI] = true

					if len(allReferences) >= maxResults {
						return allReferences
					}
				}
			}
		}

		// Also check occurrences by symbol for more complete results
		if occurrences, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
			if occs, err := occurrences.GetOccurrencesWithDocuments(ctx, symbolInfo.Symbol); err == nil {
				for _, occWithDoc := range occs {
					// Skip definitions, we want references
					if !occWithDoc.SCIPOccurrence.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
						docURI := occWithDoc.DocumentURI
						if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
							occInfo := m.buildOccurrenceInfo(&occWithDoc.SCIPOccurrence, docURI)
							allReferences = append(allReferences, occInfo)
							fileSet[docURI] = true

							if len(allReferences) >= maxResults {
								return allReferences
							}
						}
					}
				}
			}
		}
	}

	// Deduplicate results
	seen := make(map[string]bool)
	uniqueReferences := []interface{}{}
	for _, ref := range allReferences {
		if occInfo, ok := ref.(SCIPOccurrenceInfo); ok {
			key := fmt.Sprintf("%s:%d:%d", occInfo.DocumentURI, occInfo.LineNumber, occInfo.Occurrence.Range.Start.Character)
			if !seen[key] {
				seen[key] = true
				uniqueReferences = append(uniqueReferences, ref)
			}
		}
	}

	return uniqueReferences
}

// SearchReferencesEnhanced performs direct SCIP reference search with enhanced results
func (m *SCIPCacheManager) SearchReferencesEnhanced(ctx context.Context, symbolName, filePattern string, options *ReferenceSearchOptions) (*ReferenceSearchResult, error) {
	if !m.enabled {
		return &ReferenceSearchResult{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   map[string]interface{}{"cache_disabled": true},
			Timestamp:  time.Now(),
		}, nil
	}

	if options == nil {
		options = &ReferenceSearchOptions{MaxResults: 100}
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, _ := m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	if len(symbolInfos) == 0 {
		symbolInfos, _ = m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	}

	if len(symbolInfos) == 0 {
		return &ReferenceSearchResult{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   map[string]interface{}{"symbols_found": 0},
			Timestamp:  time.Now(),
		}, nil
	}

	var allReferences []SCIPOccurrenceInfo
	var definition *SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	symbolID := ""

	for _, symbolInfo := range symbolInfos {
		symbolID = symbolInfo.Symbol

		// Use GetReferencesWithDocuments for better performance
		if refOccurrences, err := m.scipStorage.GetReferencesWithDocuments(ctx, symbolInfo.Symbol); err == nil {
			for _, occWithDoc := range refOccurrences {
				docURI := occWithDoc.DocumentURI
				if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
					continue
				}

				occInfo := m.buildOccurrenceInfo(&occWithDoc.SCIPOccurrence, docURI)
				allReferences = append(allReferences, occInfo)
				fileSet[docURI] = true

				if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
					break
				}
			}
		}

		if definition == nil {
			// Use GetDefinitionsWithDocuments for better performance
			if defOccs, err := m.scipStorage.GetDefinitionsWithDocuments(ctx, symbolInfo.Symbol); err == nil && len(defOccs) > 0 {
				defOcc := &defOccs[0]
				docURI := defOcc.DocumentURI
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					defInfo := m.buildOccurrenceInfo(&defOcc.SCIPOccurrence, docURI)
					definition = &defInfo
					fileSet[docURI] = true
				}
			}
		}

		if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
			break
		}
	}

	if options.SortBy != "" {
		m.sortOccurrenceResults(allReferences, options.SortBy)
	}

	return &ReferenceSearchResult{
		SymbolName: symbolName,
		SymbolID:   symbolID,
		References: allReferences,
		Definition: definition,
		TotalCount: len(allReferences),
		FileCount:  len(fileSet),
		Options:    options,
		Metadata: map[string]interface{}{
			"scip_enabled":  true,
			"symbols_found": len(symbolInfos),
			"file_pattern":  filePattern,
		},
		Timestamp: time.Now(),
	}, nil
}
