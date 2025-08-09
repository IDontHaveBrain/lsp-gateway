package cache

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// QueryIndex queries the SCIP storage for symbols and relationships
func (m *SCIPCacheManager) QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error) {
	if !m.enabled {
		return &IndexResult{
			Type:      query.Type,
			Results:   []interface{}{},
			Metadata:  map[string]interface{}{"cache_disabled": true},
			Timestamp: time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	var results []interface{}
	var err error

	switch query.Type {
	case "symbol":
		sciPSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
		if err == nil {
			results = make([]interface{}, len(sciPSymbols))
			for i, sym := range sciPSymbols {
				results[i] = sym
			}
		}
	case "definition":
		if defOccs, err := m.scipStorage.GetDefinitions(ctx, query.Symbol); err == nil && len(defOccs) > 0 {
			results = []interface{}{defOccs[0]}
		}
	case "references":
		if refOccs, err := m.scipStorage.GetReferences(ctx, query.Symbol); err == nil {
			for _, occ := range refOccs {
				results = append(results, occ)
			}
		}
	case "workspace":
		sciPSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Symbol, 100)
		if err == nil {
			results = make([]interface{}, len(sciPSymbols))
			for i, sym := range sciPSymbols {
				results[i] = sym
			}
		}
	default:
		return nil, fmt.Errorf("unsupported query type: %s", query.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	metadata := map[string]interface{}{
		"query_language": query.Language,
		"query_type":     query.Type,
	}

	stats := m.scipStorage.GetIndexStats()
	metadata["total_symbols"] = stats.TotalSymbols
	metadata["total_documents"] = stats.TotalDocuments
	metadata["total_occurrences"] = stats.TotalOccurrences

	return &IndexResult{
		Type:      query.Type,
		Results:   results,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}, nil
}

// SearchSymbolsEnhanced performs direct SCIP symbol search with enhanced results
func (m *SCIPCacheManager) SearchSymbolsEnhanced(ctx context.Context, query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResult, error) {
	if !m.enabled {
		return &EnhancedSymbolSearchResult{
			Symbols:   []EnhancedSymbolResult{},
			Total:     0,
			Truncated: false,
			Query:     query,
			Metadata:  map[string]interface{}{"cache_disabled": true},
			Timestamp: time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	maxResults := 100
	if query.MaxResults > 0 {
		maxResults = query.MaxResults
	}

	searchSymbols, err := m.scipStorage.SearchSymbols(ctx, query.Pattern, maxResults*2)
	if err != nil {
		return nil, fmt.Errorf("failed to search symbols in SCIP storage: %w", err)
	}

	var enhancedResults []EnhancedSymbolResult
	for i, symbolInfo := range searchSymbols {
		if i >= maxResults {
			break
		}

		// Apply filters
		if len(query.SymbolKinds) > 0 {
			kindMatched := false
			for _, kind := range query.SymbolKinds {
				if symbolInfo.Kind == kind {
					kindMatched = true
					break
				}
			}
			if !kindMatched {
				continue
			}
		}

		occurrences, _ := m.scipStorage.GetOccurrences(ctx, symbolInfo.Symbol)

		if query.MinOccurrences > 0 && len(occurrences) < query.MinOccurrences {
			continue
		}
		if query.MaxOccurrences > 0 && len(occurrences) > query.MaxOccurrences {
			continue
		}

		enhancedResults = append(enhancedResults, m.buildEnhancedSymbolResult(&symbolInfo, occurrences, query))
	}

	if query.SortBy != "" {
		m.sortEnhancedResults(enhancedResults, query.SortBy)
	}

	return &EnhancedSymbolSearchResult{
		Symbols:   enhancedResults,
		Total:     len(enhancedResults),
		Truncated: len(searchSymbols) > maxResults,
		Query:     query,
		Metadata: map[string]interface{}{
			"scip_enabled":     true,
			"total_candidates": len(searchSymbols),
		},
		Timestamp: time.Now(),
	}, nil
}

// SearchReferences provides interface-compliant reference search
func (m *SCIPCacheManager) SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled {
		return []interface{}{}, nil
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

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
		// Get references from the index
		if refOccs, err := m.scipStorage.GetReferences(ctx, symbolInfo.Symbol); err == nil {
			for _, occ := range refOccs {
				docURI := m.extractURIFromOccurrence(&occ)
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					occInfo := m.buildOccurrenceInfo(&occ, docURI)
					allReferences = append(allReferences, occInfo)
					fileSet[docURI] = true

					if len(allReferences) >= maxResults {
						return allReferences, nil
					}
				}
			}
		}

		// Also check occurrences by symbol for more complete results
		if occurrences, found := m.scipStorage.(*scip.SimpleSCIPStorage); found {
			if occs, err := occurrences.GetOccurrences(ctx, symbolInfo.Symbol); err == nil {
				for _, occ := range occs {
					// Skip definitions, we want references
					if !occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
						docURI := m.extractURIFromOccurrence(&occ)
						if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
							occInfo := m.buildOccurrenceInfo(&occ, docURI)
							allReferences = append(allReferences, occInfo)
							fileSet[docURI] = true

							if len(allReferences) >= maxResults {
								return allReferences, nil
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

	return uniqueReferences, nil
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

		if refOccurrences, err := m.scipStorage.GetReferences(ctx, symbolInfo.Symbol); err == nil {
			for _, occ := range refOccurrences {
				docURI := m.extractURIFromOccurrence(&occ)
				if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
					continue
				}

				occInfo := m.buildOccurrenceInfo(&occ, docURI)
				allReferences = append(allReferences, occInfo)
				fileSet[docURI] = true

				if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
					break
				}
			}
		}

		if definition == nil {
			if defOccs, err := m.scipStorage.GetDefinitions(ctx, symbolInfo.Symbol); err == nil && len(defOccs) > 0 {
				defOcc := &defOccs[0]
				docURI := m.extractURIFromOccurrence(defOcc)
				if filePattern == "" || m.matchFilePattern(docURI, filePattern) {
					defInfo := m.buildOccurrenceInfo(defOcc, docURI)
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

// SearchDefinitions performs direct SCIP definition search with enhanced results
func (m *SCIPCacheManager) SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled {
		return []interface{}{}, nil
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, _ := m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	if len(symbolInfos) == 0 {
		return []interface{}{}, nil
	}

	var allDefinitions []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)

	for _, symbolInfo := range symbolInfos {

		if defOccs, err := m.scipStorage.GetDefinitions(ctx, symbolInfo.Symbol); err == nil && len(defOccs) > 0 {
			defOcc := &defOccs[0]
			docURI := m.extractURIFromOccurrence(defOcc)

			if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
				continue
			}

			occInfo := m.buildOccurrenceInfo(defOcc, docURI)
			allDefinitions = append(allDefinitions, occInfo)
			fileSet[docURI] = true
		}
	}

	// Convert to interface slice and apply maxResults limit
	results := make([]interface{}, 0, len(allDefinitions))
	for i, def := range allDefinitions {
		if maxResults > 0 && i >= maxResults {
			break
		}
		results = append(results, def)
	}

	return results, nil
}

// GetSymbolInfo performs direct SCIP symbol information retrieval with occurrence details
func (m *SCIPCacheManager) GetSymbolInfo(ctx context.Context, symbolName, filePattern string) (interface{}, error) {
	if !m.enabled {
		return &SymbolInfoResult{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        map[string]interface{}{"cache_disabled": true},
			Timestamp:       time.Now(),
		}, nil
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	symbolInfos, _ := m.scipStorage.SearchSymbols(ctx, symbolName, 10)
	if len(symbolInfos) == 0 {
		return &SymbolInfoResult{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        map[string]interface{}{"symbols_found": 0},
			Timestamp:       time.Now(),
		}, nil
	}

	symbolInfo := symbolInfos[0]
	symbolID := symbolInfo.Symbol

	allOccurrences, _ := m.scipStorage.GetOccurrences(ctx, symbolID)

	var filteredOccurrences []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	definitionCount := 0
	referenceCount := 0

	for _, occ := range allOccurrences {
		docURI := m.extractURIFromOccurrence(&occ)

		if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
			continue
		}

		occInfo := m.buildOccurrenceInfo(&occ, docURI)
		filteredOccurrences = append(filteredOccurrences, occInfo)
		fileSet[docURI] = true

		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			definitionCount++
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
			referenceCount++
		}
	}

	// Note: Relationships not available in simplified interface, would need to be implemented separately
	var relationships []scip.SCIPRelationship
	signature := m.formatSymbolDetail(&symbolInfo)

	return &SymbolInfoResult{
		SymbolName:      symbolName,
		SymbolID:        symbolID,
		SymbolInfo:      &symbolInfo,
		Kind:            symbolInfo.Kind,
		Documentation:   symbolInfo.Documentation,
		Signature:       signature,
		Relationships:   relationships,
		Occurrences:     filteredOccurrences,
		OccurrenceCount: len(filteredOccurrences),
		DefinitionCount: definitionCount,
		ReferenceCount:  referenceCount,
		FileCount:       len(fileSet),
		Metadata: map[string]interface{}{
			"scip_enabled":         true,
			"symbols_found":        len(symbolInfos),
			"total_occurrences":    len(allOccurrences),
			"filtered_occurrences": len(filteredOccurrences),
			"file_pattern":         filePattern,
		},
		Timestamp: time.Now(),
	}, nil
}

// SearchSymbols provides direct access to SCIP symbol search for MCP tools
func (m *SCIPCacheManager) SearchSymbols(ctx context.Context, pattern, filePattern string, maxResults int) ([]interface{}, error) {
	if !m.enabled {
		return nil, fmt.Errorf("cache disabled or SCIP storage unavailable")
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	symbolInfos, err := m.scipStorage.SearchSymbols(ctx, pattern, maxResults)
	if err != nil {
		return nil, fmt.Errorf("SCIP symbol search failed: %w", err)
	}

	results := make([]interface{}, 0, len(symbolInfos))
	for _, symbolInfo := range symbolInfos {
		// Prefer definition; else any occurrence
		var occWithDoc *scip.OccurrenceWithDocument
		if defs, _ := m.scipStorage.GetDefinitionsWithDocuments(ctx, symbolInfo.Symbol); len(defs) > 0 {
			occWithDoc = &defs[0]
		} else if occs, _ := m.scipStorage.GetOccurrencesWithDocuments(ctx, symbolInfo.Symbol); len(occs) > 0 {
			occWithDoc = &occs[0]
		}
		if occWithDoc == nil {
			results = append(results, symbolInfo)
			continue
		}
		// Apply file filter
		if filePattern != "" && !m.matchFilePattern(occWithDoc.DocumentURI, filePattern) {
			continue
		}
		enhancedResult := map[string]interface{}{
			"symbolInfo":  symbolInfo,
			"occurrence":  &occWithDoc.SCIPOccurrence,
			"filePath":    occWithDoc.DocumentURI,
			"documentURI": occWithDoc.DocumentURI,
			"range":       occWithDoc.Range,
		}
		results = append(results, enhancedResult)
	}

	return results, nil
}

// Helper methods for enhanced search operations

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
	if pattern == "" {
		return true
	}

	// Simple pattern matching - could be enhanced with glob patterns
	if strings.Contains(pattern, "*") {
		// Basic wildcard support
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		if matched, err := regexp.MatchString(pattern, uri); err == nil {
			return matched
		}
	}

	// Exact or substring match
	return strings.Contains(uri, pattern)
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