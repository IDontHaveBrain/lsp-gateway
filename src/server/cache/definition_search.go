package cache

import (
	"context"
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// Definition search operations - handles definition searching and symbol info retrieval

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
		// Use GetDefinitionsWithDocuments for better performance
		if defOccs, err := m.scipStorage.GetDefinitionsWithDocuments(ctx, symbolInfo.Symbol); err == nil && len(defOccs) > 0 {
			defOcc := &defOccs[0]
			docURI := defOcc.DocumentURI

			if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
				continue
			}

			occInfo := m.buildOccurrenceInfo(&defOcc.SCIPOccurrence, docURI)
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

	// Use GetOccurrencesWithDocuments for better performance
	allOccurrences, _ := m.scipStorage.GetOccurrencesWithDocuments(ctx, symbolID)

	var filteredOccurrences []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	definitionCount := 0
	referenceCount := 0

	for _, occWithDoc := range allOccurrences {
		docURI := occWithDoc.DocumentURI
		occ := &occWithDoc.SCIPOccurrence

		if filePattern != "" && !m.matchFilePattern(docURI, filePattern) {
			continue
		}

		occInfo := m.buildOccurrenceInfo(occ, docURI)
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
