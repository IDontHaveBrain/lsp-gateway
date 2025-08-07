package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/scip"
)

// performSCIPIndexing performs SCIP indexing based on LSP method and response using occurrence-centric approach
func (m *LSPManager) performSCIPIndexing(ctx context.Context, method, uri, language string, params, result interface{}) {
	if m.scipCache == nil {
		return
	}

	// Only index for specific methods that provide useful data
	switch method {
	case types.MethodTextDocumentDocumentSymbol:
		m.indexDocumentSymbolsAsOccurrences(ctx, uri, language, result)
	case types.MethodTextDocumentDefinition:
		m.indexDefinitionsAsOccurrences(ctx, uri, language, params, result)
	case types.MethodTextDocumentReferences:
		m.indexReferencesAsOccurrences(ctx, uri, language, params, result)
	case types.MethodWorkspaceSymbol:
		m.indexWorkspaceSymbolsAsOccurrences(ctx, language, result)
	}
}

// indexDocumentSymbolsAsOccurrences indexes document symbols as SCIP occurrences with definition roles
func (m *LSPManager) indexDocumentSymbolsAsOccurrences(ctx context.Context, uri, language string, result interface{}) {
	common.LSPLogger.Debug("indexDocumentSymbolsAsOccurrences called for uri=%s, language=%s", uri, language)

	var symbols []lsp.SymbolInformation
	var conversionFailures, jsonFailures int

	// Parse document symbols from various response formats
	symbols = m.parseDocumentSymbols(result, uri, &conversionFailures, &jsonFailures)
	common.LSPLogger.Debug("Parsed %d symbols from document %s", len(symbols), uri)

	// Report conversion issues
	if jsonFailures > 0 || conversionFailures > 0 {
		common.LSPLogger.Warn("Symbol conversion issues for %s - JSON failures: %d, conversion failures: %d, successful symbols: %d", uri, jsonFailures, conversionFailures, len(symbols))
	}

	if len(symbols) == 0 {
		common.LSPLogger.Debug("No symbols found for %s, returning early", uri)
		return
	}

	// Get package information for SCIP symbol generation
	packageInfo := m.getPackageInfoForDocument(uri, language)

	// Expand symbol ranges if they appear to be single-line
	symbols = m.expandSymbolRanges(ctx, symbols, uri, language)

	// Convert symbols to SCIP occurrences with definition roles
	occurrences := make([]scip.SCIPOccurrence, 0, len(symbols))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(symbols))

	for _, symbol := range symbols {
		// Generate SCIP symbol ID
		symbolID := m.generateSCIPSymbolID(language, packageInfo, symbol.Name, symbol.Kind)

		// Create SCIP occurrence with definition role
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      int32(symbol.Location.Range.Start.Line),
					Character: int32(symbol.Location.Range.Start.Character),
				},
				End: types.Position{
					Line:      int32(symbol.Location.Range.End.Line),
					Character: int32(symbol.Location.Range.End.Character),
				},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleDefinition,
			SyntaxKind:  m.mapLSPSymbolKindToSyntaxKind(symbol.Kind),
		}
		occurrences = append(occurrences, occurrence)

		// Create symbol information with enhanced metadata
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:        symbolID,
			DisplayName:   symbol.Name,
			Kind:          m.mapLSPSymbolKindToSCIPKind(symbol.Kind),
			Documentation: m.getSymbolDocumentation(ctx, uri, symbol.Location.Range.Start),
			Relationships: m.getSymbolRelationships(ctx, uri, symbol.Name, symbol.Kind),
		}
		symbolInfos = append(symbolInfos, symbolInfo)
	}

	// Store as SCIP document
	common.LSPLogger.Debug("Storing %d occurrences and %d symbol infos for %s", len(occurrences), len(symbolInfos), uri)
	m.storeDocumentOccurrences(ctx, uri, language, occurrences, symbolInfos)
	common.LSPLogger.Debug("Successfully stored document occurrences for %s", uri)

}

// indexDefinitionsAsOccurrences indexes definition results as SCIP occurrences with definition roles
func (m *LSPManager) indexDefinitionsAsOccurrences(ctx context.Context, uri, language string, params, result interface{}) {

	// Parse definition result - can be Location | Location[] | LocationLink[]
	locations := m.parseLocationResult(result)
	if len(locations) == 0 {
		return
	}

	// Extract position from params to determine the symbol being defined
	position, symbolName := m.extractPositionAndSymbolFromParams(params)
	if symbolName == "" {
		// Generate a placeholder symbol name based on position
		symbolName = fmt.Sprintf("symbol_at_%d_%d", position.Line, position.Character)
	}

	// Get package information for SCIP symbol generation
	packageInfo := m.getPackageInfoForDocument(uri, language)

	// Create SCIP occurrences for each definition location
	occurrences := make([]scip.SCIPOccurrence, 0, len(locations))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(locations))

	for _, location := range locations {
		// Generate SCIP symbol ID
		symbolID := m.generateSCIPSymbolIDFromName(language, packageInfo, symbolName, location.URI)

		// Create SCIP occurrence with definition role
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      int32(location.Range.Start.Line),
					Character: int32(location.Range.Start.Character),
				},
				End: types.Position{
					Line:      int32(location.Range.End.Line),
					Character: int32(location.Range.End.Character),
				},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleDefinition,
			SyntaxKind:  types.SyntaxKindIdentifierFunctionDefinition,
		}
		occurrences = append(occurrences, occurrence)

		// Create symbol information with enhanced metadata
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:        symbolID,
			DisplayName:   symbolName,
			Kind:          scip.SCIPSymbolKindFunction, // Default to function, can be refined
			Documentation: m.getSymbolDocumentation(ctx, location.URI, location.Range.Start),
			Relationships: m.getSymbolRelationships(ctx, location.URI, symbolName, lsp.Function),
		}
		symbolInfos = append(symbolInfos, symbolInfo)
	}

	// Store occurrences grouped by document URI
	m.storeOccurrencesByDocument(ctx, occurrences, symbolInfos)

}

// indexReferencesAsOccurrences indexes reference results as SCIP occurrences with reference roles
func (m *LSPManager) indexReferencesAsOccurrences(ctx context.Context, uri, language string, params, result interface{}) {

	// Parse reference result - should be Location[]
	locations := m.parseLocationResult(result)
	if len(locations) == 0 {
		return
	}

	// Extract position from params to determine the symbol being referenced
	position, symbolName := m.extractPositionAndSymbolFromParams(params)
	if symbolName == "" {
		// Generate a placeholder symbol name based on position
		symbolName = fmt.Sprintf("symbol_at_%d_%d", position.Line, position.Character)
	}

	// Get package information for SCIP symbol generation
	packageInfo := m.getPackageInfoForDocument(uri, language)

	// Create SCIP occurrences for each reference location
	occurrences := make([]scip.SCIPOccurrence, 0, len(locations))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0)

	for _, location := range locations {
		// Generate SCIP symbol ID
		symbolID := m.generateSCIPSymbolIDFromName(language, packageInfo, symbolName, location.URI)

		// Determine if this is read or write access
		role := types.SymbolRoleReadAccess // Default to read access
		// TODO: Implement context analysis to detect write access

		// Create SCIP occurrence with reference role
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      int32(location.Range.Start.Line),
					Character: int32(location.Range.Start.Character),
				},
				End: types.Position{
					Line:      int32(location.Range.End.Line),
					Character: int32(location.Range.End.Character),
				},
			},
			Symbol:      symbolID,
			SymbolRoles: role,
			SyntaxKind:  types.SyntaxKindIdentifierFunction,
		}
		occurrences = append(occurrences, occurrence)
	}

	// Create symbol information once for the referenced symbol with enhanced metadata
	if len(occurrences) > 0 && len(locations) > 0 {
		// Use the first location to get documentation
		firstLocation := locations[0]
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:        occurrences[0].Symbol,
			DisplayName:   symbolName,
			Kind:          scip.SCIPSymbolKindFunction, // Default to function, can be refined
			Documentation: m.getSymbolDocumentation(ctx, firstLocation.URI, firstLocation.Range.Start),
			Relationships: m.getSymbolRelationships(ctx, firstLocation.URI, symbolName, lsp.Function),
		}
		symbolInfos = append(symbolInfos, symbolInfo)
	}

	// Store occurrences grouped by document URI
	m.storeOccurrencesByDocument(ctx, occurrences, symbolInfos)

}

// indexWorkspaceSymbolsAsOccurrences indexes workspace symbols as SCIP occurrences with definition roles
func (m *LSPManager) indexWorkspaceSymbolsAsOccurrences(ctx context.Context, language string, result interface{}) {

	var symbols []lsp.SymbolInformation

	// Handle different response types from workspace/symbol
	switch v := result.(type) {
	case []lsp.SymbolInformation:
		symbols = v
	case []interface{}:
		for _, item := range v {
			if data, err := json.Marshal(item); err == nil {
				var symbol lsp.SymbolInformation
				if err := json.Unmarshal(data, &symbol); err == nil && symbol.Name != "" {
					symbols = append(symbols, symbol)
				}
			}
		}
	default:
	}

	if len(symbols) == 0 {
		return
	}

	// Group symbols by URI for batch processing
	symbolsByURI := make(map[string][]lsp.SymbolInformation)
	for _, symbol := range symbols {
		if symbol.Location.URI != "" {
			symbolsByURI[symbol.Location.URI] = append(symbolsByURI[symbol.Location.URI], symbol)
		}
	}

	// Convert workspace symbols to SCIP occurrences for each document
	totalIndexed := 0
	for uri, uriSymbols := range symbolsByURI {
		packageInfo := m.getPackageInfoForDocument(uri, language)

		// Expand symbol ranges to get full definitions
		uriSymbols = m.expandSymbolRanges(ctx, uriSymbols, uri, language)

		// Convert symbols to SCIP occurrences with definition roles
		occurrences := make([]scip.SCIPOccurrence, 0, len(uriSymbols))
		symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(uriSymbols))

		for _, symbol := range uriSymbols {
			// Generate SCIP symbol ID
			symbolID := m.generateSCIPSymbolID(language, packageInfo, symbol.Name, symbol.Kind)

			// Create SCIP occurrence with definition role
			occurrence := scip.SCIPOccurrence{
				Range: types.Range{
					Start: types.Position{
						Line:      int32(symbol.Location.Range.Start.Line),
						Character: int32(symbol.Location.Range.Start.Character),
					},
					End: types.Position{
						Line:      int32(symbol.Location.Range.End.Line),
						Character: int32(symbol.Location.Range.End.Character),
					},
				},
				Symbol:      symbolID,
				SymbolRoles: types.SymbolRoleDefinition,
				SyntaxKind:  m.mapLSPSymbolKindToSyntaxKind(symbol.Kind),
			}
			occurrences = append(occurrences, occurrence)

			// Create symbol information with enhanced metadata
			symbolInfo := scip.SCIPSymbolInformation{
				Symbol:        symbolID,
				DisplayName:   symbol.Name,
				Kind:          m.mapLSPSymbolKindToSCIPKind(symbol.Kind),
				Documentation: m.getSymbolDocumentation(ctx, uri, symbol.Location.Range.Start),
				Relationships: m.getSymbolRelationships(ctx, uri, symbol.Name, symbol.Kind),
			}
			symbolInfos = append(symbolInfos, symbolInfo)
		}

		// Store as SCIP document
		m.storeDocumentOccurrences(ctx, uri, language, occurrences, symbolInfos)
		totalIndexed += len(occurrences)
	}

	if totalIndexed > 0 {
	}
}

// Helper methods for SCIP symbol generation and occurrence management

// expandSymbolRanges expands single-line symbol ranges to their full extent
// This is necessary because many LSP servers return only the symbol name range
// rather than the full definition range for workspace/symbol results
func (m *LSPManager) expandSymbolRanges(ctx context.Context, symbols []lsp.SymbolInformation, uri, language string) []lsp.SymbolInformation {
	// Group symbols that need expansion
	needsExpansion := false
	for _, symbol := range symbols {
		if symbol.Location.Range.Start.Line == symbol.Location.Range.End.Line {
			needsExpansion = true
			break
		}
	}

	if !needsExpansion {
		return symbols
	}

	// Try to get full ranges via textDocument/documentSymbol
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	docSymbolResult, err := m.ProcessRequest(ctx, types.MethodTextDocumentDocumentSymbol, params)
	if err != nil || docSymbolResult == nil {
		// If we can't get document symbols, return original symbols
		return symbols
	}

	// Parse the document symbols to get full ranges
	var fullRangeSymbols []*lsp.DocumentSymbol
	switch v := docSymbolResult.(type) {
	case []lsp.DocumentSymbol:
		for i := range v {
			fullRangeSymbols = append(fullRangeSymbols, &v[i])
		}
	case []*lsp.DocumentSymbol:
		fullRangeSymbols = v
	case []interface{}:
		for _, item := range v {
			if data, err := json.Marshal(item); err == nil {
				var docSymbol lsp.DocumentSymbol
				if err := json.Unmarshal(data, &docSymbol); err == nil && docSymbol.Name != "" {
					fullRangeSymbols = append(fullRangeSymbols, &docSymbol)
				}
			}
		}
	}

	// Create a map of symbol names to their full ranges
	fullRangeMap := make(map[string]lsp.Range)
	m.collectFullRanges(fullRangeSymbols, fullRangeMap)

	// Update symbols with full ranges
	expandedSymbols := make([]lsp.SymbolInformation, len(symbols))
	for i, symbol := range symbols {
		expandedSymbols[i] = symbol
		if fullRange, exists := fullRangeMap[symbol.Name]; exists {
			// Only update if the full range is actually larger
			if fullRange.End.Line > symbol.Location.Range.End.Line ||
				(fullRange.End.Line == symbol.Location.Range.End.Line &&
					fullRange.End.Character > symbol.Location.Range.End.Character) {
				expandedSymbols[i].Location.Range = fullRange
			}
		}
	}

	return expandedSymbols
}

// collectFullRanges recursively collects full ranges from DocumentSymbol hierarchy
func (m *LSPManager) collectFullRanges(symbols []*lsp.DocumentSymbol, rangeMap map[string]lsp.Range) {
	for _, symbol := range symbols {
		// Store the full range for this symbol
		rangeMap[symbol.Name] = symbol.Range

		// Recursively process children
		if len(symbol.Children) > 0 {
			m.collectFullRanges(symbol.Children, rangeMap)
		}
	}
}

// parseDocumentSymbols parses document symbols from various response formats
func (m *LSPManager) parseDocumentSymbols(result interface{}, uri string, conversionFailures, jsonFailures *int) []lsp.SymbolInformation {
	var symbols []lsp.SymbolInformation

	// Handle different response types from textDocument/documentSymbol
	switch v := result.(type) {
	case json.RawMessage:
		var rawData []interface{}
		if err := json.Unmarshal(v, &rawData); err != nil {
			common.LSPLogger.Warn("Failed to unmarshal json.RawMessage for %s: %v", uri, err)
			return symbols
		}
		symbols = m.parseSymbolArray(rawData, uri, conversionFailures, jsonFailures)
	case []interface{}:
		symbols = m.parseSymbolArray(v, uri, conversionFailures, jsonFailures)
	case []lsp.SymbolInformation:
		symbols = v
	case []lsp.DocumentSymbol:
		for _, docSymbol := range v {
			symbols = append(symbols, lsp.SymbolInformation{
				Name: docSymbol.Name,
				Kind: docSymbol.Kind,
				Location: lsp.Location{
					URI:   uri,
					Range: docSymbol.Range,
				},
			})
		}
	case nil:
	default:
		common.LSPLogger.Warn("Unexpected response type %T for document symbols from %s", result, uri)
	}

	return symbols
}

// parseSymbolArray parses an array of interface{} into SymbolInformation
func (m *LSPManager) parseSymbolArray(items []interface{}, uri string, conversionFailures, jsonFailures *int) []lsp.SymbolInformation {
	var symbols []lsp.SymbolInformation

	for _, item := range items {
		if data, err := json.Marshal(item); err == nil {
			var symbol lsp.SymbolInformation
			if err := json.Unmarshal(data, &symbol); err == nil && symbol.Name != "" {
				symbols = append(symbols, symbol)
			} else {
				// Try DocumentSymbol format
				var docSymbol lsp.DocumentSymbol
				if err := json.Unmarshal(data, &docSymbol); err == nil && docSymbol.Name != "" {
					symbols = append(symbols, lsp.SymbolInformation{
						Name: docSymbol.Name,
						Kind: docSymbol.Kind,
						Location: lsp.Location{
							URI:   uri,
							Range: docSymbol.Range,
						},
					})
				} else {
					*conversionFailures++
				}
			}
		} else {
			*jsonFailures++
		}
	}

	return symbols
}

// parseLocationResult parses location results from various response formats
func (m *LSPManager) parseLocationResult(result interface{}) []lsp.Location {
	var locations []lsp.Location

	switch v := result.(type) {
	case nil:
		return locations
	case lsp.Location:
		locations = []lsp.Location{v}
	case []lsp.Location:
		locations = v
	case []interface{}:
		for _, item := range v {
			if data, err := json.Marshal(item); err == nil {
				var location lsp.Location
				if err := json.Unmarshal(data, &location); err == nil {
					locations = append(locations, location)
				} else {
					// Try LocationLink format
					var locationLink map[string]interface{}
					if err := json.Unmarshal(data, &locationLink); err == nil {
						if targetUri, ok := locationLink["targetUri"].(string); ok {
							var targetRange lsp.Range
							if rangeData, err := json.Marshal(locationLink["targetRange"]); err == nil {
								if err := json.Unmarshal(rangeData, &targetRange); err == nil {
									locations = append(locations, lsp.Location{
										URI:   targetUri,
										Range: targetRange,
									})
								}
							}
						}
					}
				}
			}
		}
	default:
		if data, err := json.Marshal(result); err == nil {
			var location lsp.Location
			if err := json.Unmarshal(data, &location); err == nil {
				locations = []lsp.Location{location}
			}
		}
	}

	return locations
}

// extractPositionAndSymbolFromParams extracts position and symbol name from LSP params
func (m *LSPManager) extractPositionAndSymbolFromParams(params interface{}) (types.Position, string) {
	var position types.Position
	var symbolName string

	// Try to extract position from textDocument/position params
	if paramsMap, ok := params.(map[string]interface{}); ok {
		if posMap, ok := paramsMap["position"].(map[string]interface{}); ok {
			if line, ok := posMap["line"].(float64); ok {
				position.Line = int32(line)
			}
			if char, ok := posMap["character"].(float64); ok {
				position.Character = int32(char)
			}
		}
		// TODO: Extract symbol name from context if available
	}

	return position, symbolName
}

// getPackageInfoForDocument gets package information for a document URI
func (m *LSPManager) getPackageInfoForDocument(uri, language string) *project.PackageInfo {
	// Extract directory from URI
	filePath := common.URIToFilePath(uri)
	workingDir := filepath.Dir(filePath)

	// Get package info
	packageInfo, err := project.GetPackageInfo(workingDir, language)
	if err != nil {
		return &project.PackageInfo{
			Name:     "unknown-project",
			Version:  "0.0.0",
			Language: language,
		}
	}

	return packageInfo
}

// generateSCIPSymbolID generates a SCIP symbol ID from symbol information
func (m *LSPManager) generateSCIPSymbolID(language string, packageInfo *project.PackageInfo, symbolName string, symbolKind lsp.SymbolKind) string {
	// Format: scip-<language> <package> <version> <descriptor>
	packageName := packageInfo.Name
	version := packageInfo.Version
	if version == "" {
		version = "0.0.0"
	}

	// Generate descriptor based on symbol kind
	descriptor := m.generateDescriptor(symbolName, symbolKind)

	return fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, descriptor)
}

// generateSCIPSymbolIDFromName generates a SCIP symbol ID from name and URI context
func (m *LSPManager) generateSCIPSymbolIDFromName(language string, packageInfo *project.PackageInfo, symbolName, uri string) string {
	packageName := packageInfo.Name
	version := packageInfo.Version
	if version == "" {
		version = "0.0.0"
	}

	// Extract filename for descriptor context
	filename := filepath.Base(common.URIToFilePath(uri))
	descriptor := fmt.Sprintf("`%s`/%s", filename, symbolName)

	return fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, descriptor)
}

// generateDescriptor generates a SCIP descriptor from symbol name and kind
func (m *LSPManager) generateDescriptor(symbolName string, symbolKind lsp.SymbolKind) string {
	switch symbolKind {
	case lsp.Function:
		return symbolName + "()."
	case lsp.Method:
		return symbolName + "()."
	case lsp.Class:
		return symbolName + "#"
	case lsp.Interface:
		return symbolName + "#"
	case lsp.Variable:
		return symbolName + "."
	case lsp.Constant:
		return symbolName + "."
	case lsp.Field:
		return symbolName + "."
	case lsp.Property:
		return symbolName + "."
	default:
		return symbolName + "."
	}
}

// mapLSPSymbolKindToSCIPKind maps LSP symbol kinds to SCIP symbol kinds
func (m *LSPManager) mapLSPSymbolKindToSCIPKind(lspKind lsp.SymbolKind) scip.SCIPSymbolKind {
	switch lspKind {
	case lsp.File:
		return scip.SCIPSymbolKindFile
	case lsp.Module:
		return scip.SCIPSymbolKindModule
	case lsp.Namespace:
		return scip.SCIPSymbolKindNamespace
	case lsp.Package:
		return scip.SCIPSymbolKindPackage
	case lsp.Class:
		return scip.SCIPSymbolKindClass
	case lsp.Method:
		return scip.SCIPSymbolKindMethod
	case lsp.Property:
		return scip.SCIPSymbolKindProperty
	case lsp.Field:
		return scip.SCIPSymbolKindField
	case lsp.Constructor:
		return scip.SCIPSymbolKindConstructor
	case lsp.Enum:
		return scip.SCIPSymbolKindEnum
	case lsp.Interface:
		return scip.SCIPSymbolKindInterface
	case lsp.Function:
		return scip.SCIPSymbolKindFunction
	case lsp.Variable:
		return scip.SCIPSymbolKindVariable
	case lsp.Constant:
		return scip.SCIPSymbolKindConstant
	case lsp.String:
		return scip.SCIPSymbolKindString
	case lsp.Number:
		return scip.SCIPSymbolKindNumber
	case lsp.Boolean:
		return scip.SCIPSymbolKindBoolean
	case lsp.Array:
		return scip.SCIPSymbolKindArray
	case lsp.Object:
		return scip.SCIPSymbolKindObject
	case lsp.Key:
		return scip.SCIPSymbolKindKey
	case lsp.Null:
		return scip.SCIPSymbolKindNull
	case lsp.EnumMember:
		return scip.SCIPSymbolKindEnumMember
	case lsp.Struct:
		return scip.SCIPSymbolKindStruct
	case lsp.Event:
		return scip.SCIPSymbolKindEvent
	case lsp.Operator:
		return scip.SCIPSymbolKindOperator
	case lsp.TypeParameter:
		return scip.SCIPSymbolKindTypeParameter
	default:
		return scip.SCIPSymbolKindUnknown
	}
}

// mapLSPSymbolKindToSyntaxKind maps LSP symbol kinds to SCIP syntax kinds
func (m *LSPManager) mapLSPSymbolKindToSyntaxKind(lspKind lsp.SymbolKind) types.SyntaxKind {
	switch lspKind {
	case lsp.Function:
		return types.SyntaxKindIdentifierFunctionDefinition
	case lsp.Method:
		return types.SyntaxKindIdentifierFunctionDefinition
	case lsp.Class:
		return types.SyntaxKindIdentifierType
	case lsp.Interface:
		return types.SyntaxKindIdentifierType
	case lsp.Variable:
		return types.SyntaxKindIdentifierLocal
	case lsp.Constant:
		return types.SyntaxKindIdentifierConstant
	case lsp.Field:
		return types.SyntaxKindIdentifierLocal
	case lsp.Property:
		return types.SyntaxKindIdentifierLocal
	case lsp.Namespace:
		return types.SyntaxKindIdentifierNamespace
	case lsp.Module:
		return types.SyntaxKindIdentifierModule
	default:
		return types.SyntaxKindUnspecified
	}
}

// storeDocumentOccurrences stores SCIP occurrences as a document
func (m *LSPManager) storeDocumentOccurrences(ctx context.Context, uri, language string, occurrences []scip.SCIPOccurrence, symbolInfos []scip.SCIPSymbolInformation) {
	if m.scipCache == nil {
		return
	}

	// Convert SCIP occurrences to LSP SymbolInformation for SimpleCache compatibility
	var symbols []lsp.SymbolInformation
	for i, occ := range occurrences {
		// Find corresponding symbol info
		var displayName string
		var kind lsp.SymbolKind
		if i < len(symbolInfos) {
			displayName = symbolInfos[i].DisplayName
			kind = m.mapSCIPKindToLSPSymbolKind(symbolInfos[i].Kind)
		} else {
			displayName = occ.Symbol
			kind = lsp.Variable // Default
		}

		symbol := lsp.SymbolInformation{
			Name: displayName,
			Kind: kind,
			Location: lsp.Location{
				URI: uri,
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      int(occ.Range.Start.Line),
						Character: int(occ.Range.Start.Character),
					},
					End: lsp.Position{
						Line:      int(occ.Range.End.Line),
						Character: int(occ.Range.End.Character),
					},
				},
			},
		}
		symbols = append(symbols, symbol)
	}

	// Store using SimpleCache interface
	if simpleCache, ok := m.scipCache.(cache.SimpleCache); ok {
		if err := simpleCache.IndexDocument(ctx, uri, language, symbols); err != nil {
			common.LSPLogger.Debug("Failed to index document symbols for %s: %v", uri, err)
		}

		// Store enhanced SCIP metadata separately for each symbol
		// This preserves relationships and documentation
		for i, symbolInfo := range symbolInfos {
			if i < len(symbols) {
				// Store documentation as hover result
				if len(symbolInfo.Documentation) > 0 {
					hoverParams := map[string]interface{}{
						"textDocument": map[string]interface{}{"uri": uri},
						"position": map[string]interface{}{
							"line":      symbols[i].Location.Range.Start.Line,
							"character": symbols[i].Location.Range.Start.Character,
						},
					}
					hoverResult := &lsp.Hover{
						Contents: map[string]interface{}{
							"kind":  "markdown",
							"value": strings.Join(symbolInfo.Documentation, "\n"),
						},
					}
					simpleCache.Store("textDocument/hover", hoverParams, hoverResult)
				}
			}
		}
	}
}

// storeOccurrencesByDocument stores occurrences grouped by document URI
func (m *LSPManager) storeOccurrencesByDocument(ctx context.Context, occurrences []scip.SCIPOccurrence, symbolInfos []scip.SCIPSymbolInformation) {
	// Group occurrences by document URI (extracted from locations)
	occurrencesByURI := make(map[string][]scip.SCIPOccurrence)
	for _, occ := range occurrences {
		// For now, we need to determine the document URI from occurrence context
		// This is a limitation of the current approach - we might need to track this separately
		// For now, use a placeholder approach
		docURI := "unknown"
		occurrencesByURI[docURI] = append(occurrencesByURI[docURI], occ)
	}

	// Store each document
	for uri, uriOccurrences := range occurrencesByURI {
		m.storeDocumentOccurrences(ctx, uri, "unknown", uriOccurrences, symbolInfos)
	}
}

// ProcessEnhancedQuery processes a query that combines LSP and SCIP data
func (m *LSPManager) ProcessEnhancedQuery(ctx context.Context, queryType, uri, language string, params interface{}) (interface{}, error) {
	if m.scipCache == nil {
		// Fall back to regular LSP processing
		return m.ProcessRequest(ctx, queryType, params)
	}

	// SCIP storage queries not available due to interface conflicts, fall back to regular LSP
	return m.ProcessRequest(ctx, queryType, params)
}

// GetIndexStats returns SCIP index statistics
func (m *LSPManager) GetIndexStats() interface{} {
	if m.scipCache == nil {
		return map[string]interface{}{"status": "disabled"}
	}

	// SCIP storage stats not available due to interface conflicts

	// Fallback to simple cache interface
	if simpleCache, ok := m.scipCache.(cache.SimpleCache); ok {
		return simpleCache.GetIndexStats()
	}

	return map[string]interface{}{"status": "unknown"}
}

// RefreshIndex refreshes the SCIP index for given files
func (m *LSPManager) RefreshIndex(ctx context.Context, files []string) error {
	if m.scipCache == nil {
		return fmt.Errorf("SCIP cache not available")
	}

	if simpleCache, ok := m.scipCache.(cache.SimpleCache); ok {
		return simpleCache.UpdateIndex(ctx, files)
	}

	return fmt.Errorf("cache does not support index updates")
}

// mapSCIPKindToLSPSymbolKind maps SCIP symbol kinds back to LSP symbol kinds
func (m *LSPManager) mapSCIPKindToLSPSymbolKind(kind scip.SCIPSymbolKind) lsp.SymbolKind {
	switch kind {
	case scip.SCIPSymbolKindFile:
		return lsp.File
	case scip.SCIPSymbolKindModule:
		return lsp.Module
	case scip.SCIPSymbolKindNamespace:
		return lsp.Namespace
	case scip.SCIPSymbolKindPackage:
		return lsp.Package
	case scip.SCIPSymbolKindClass:
		return lsp.Class
	case scip.SCIPSymbolKindMethod:
		return lsp.Method
	case scip.SCIPSymbolKindProperty:
		return lsp.Property
	case scip.SCIPSymbolKindField:
		return lsp.Field
	case scip.SCIPSymbolKindConstructor:
		return lsp.Constructor
	case scip.SCIPSymbolKindEnum:
		return lsp.Enum
	case scip.SCIPSymbolKindInterface:
		return lsp.Interface
	case scip.SCIPSymbolKindFunction:
		return lsp.Function
	case scip.SCIPSymbolKindVariable:
		return lsp.Variable
	case scip.SCIPSymbolKindConstant:
		return lsp.Constant
	case scip.SCIPSymbolKindString:
		return lsp.String
	case scip.SCIPSymbolKindNumber:
		return lsp.Number
	case scip.SCIPSymbolKindBoolean:
		return lsp.Boolean
	case scip.SCIPSymbolKindArray:
		return lsp.Array
	case scip.SCIPSymbolKindObject:
		return lsp.Object
	case scip.SCIPSymbolKindKey:
		return lsp.Key
	case scip.SCIPSymbolKindNull:
		return lsp.Null
	case scip.SCIPSymbolKindEnumMember:
		return lsp.EnumMember
	case scip.SCIPSymbolKindStruct:
		return lsp.Struct
	case scip.SCIPSymbolKindEvent:
		return lsp.Event
	case scip.SCIPSymbolKindOperator:
		return lsp.Operator
	case scip.SCIPSymbolKindTypeParameter:
		return lsp.TypeParameter
	default:
		return lsp.Variable
	}
}

// getSymbolDocumentation retrieves documentation for a symbol using hover
func (m *LSPManager) getSymbolDocumentation(ctx context.Context, uri string, position lsp.Position) []string {
	// Use hover to get symbol documentation
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      position.Line,
			"character": position.Character,
		},
	}

	hoverResult, err := m.ProcessRequest(ctx, types.MethodTextDocumentHover, params)
	if err != nil || hoverResult == nil {
		return nil
	}

	// Parse hover result to extract documentation
	var documentation []string
	switch v := hoverResult.(type) {
	case map[string]interface{}:
		if contents, ok := v["contents"]; ok {
			switch c := contents.(type) {
			case string:
				documentation = append(documentation, c)
			case map[string]interface{}:
				if value, ok := c["value"].(string); ok {
					documentation = append(documentation, value)
				}
			case []interface{}:
				for _, item := range c {
					if str, ok := item.(string); ok {
						documentation = append(documentation, str)
					}
				}
			}
		}
	}

	return documentation
}

// getSymbolRelationships retrieves relationships for a symbol
func (m *LSPManager) getSymbolRelationships(ctx context.Context, uri string, symbolName string, kind lsp.SymbolKind) []scip.SCIPRelationship {
	var relationships []scip.SCIPRelationship

	// For classes and interfaces, try to find implementations
	if kind == lsp.Class || kind == lsp.Interface {
		implementations := m.findImplementations(ctx, uri, symbolName)
		for _, impl := range implementations {
			relationships = append(relationships, scip.SCIPRelationship{
				Symbol:           impl,
				IsImplementation: true,
			})
		}
	}

	// For methods, try to find the type they belong to
	if kind == lsp.Method || kind == lsp.Function {
		typeDefinition := m.findTypeDefinition(ctx, uri, symbolName)
		if typeDefinition != "" {
			relationships = append(relationships, scip.SCIPRelationship{
				Symbol:           typeDefinition,
				IsTypeDefinition: true,
			})
		}
	}

	return relationships
}

// findImplementations finds implementations of an interface or class
func (m *LSPManager) findImplementations(ctx context.Context, uri string, symbolName string) []string {
	// Try to use textDocument/implementation if available
	// This is a simplified version - in reality, we'd need to find the symbol position first
	var implementations []string

	// For now, return empty - this would require more complex logic
	// to track type hierarchies across the codebase
	return implementations
}

// findTypeDefinition finds the type definition for a method or field
func (m *LSPManager) findTypeDefinition(ctx context.Context, uri string, symbolName string) string {
	// Try to use textDocument/typeDefinition if available
	// This is a simplified version - in reality, we'd need to find the symbol position first

	// For now, return empty - this would require more complex logic
	return ""
}
