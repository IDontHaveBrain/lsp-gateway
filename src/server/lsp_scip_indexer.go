package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
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

	// Parse document symbols from various response formats
	symbols := m.parseSymbols(result, uri)
	common.LSPLogger.Debug("Parsed %d symbols from document %s", len(symbols), uri)

	if len(symbols) == 0 {
		common.LSPLogger.Debug("No symbols found for %s, returning early", uri)
		return
	}

	// Expand symbol ranges if they appear to be single-line
	symbols = m.expandSymbolRanges(ctx, symbols, uri, language)

	// Convert symbols to SCIP occurrences with definition roles
	occurrences := make([]scip.SCIPOccurrence, 0, len(symbols))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(symbols))

	for _, symbol := range symbols {
		// Skip builtin types that don't have actual definitions in the codebase
		if m.isBuiltinType(symbol.Name) {
			continue
		}

		// Generate SCIP symbol ID
		symbolID := m.generateSymbolID(language, uri, symbol.Name, symbol.Kind)

		// Use SelectionRange if available for more precise position, otherwise use Range
		symbolPosition := symbol.Location.Range.Start
		if symbol.SelectionRange != nil {
			symbolPosition = symbol.SelectionRange.Start
		}

		// Create SCIP occurrence with definition role
		// Use SelectionRange for the occurrence if available (more precise identifier location)
		occurrenceRange := symbol.Location.Range
		if symbol.SelectionRange != nil {
			occurrenceRange = *symbol.SelectionRange
		}

		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{
					Line:      occurrenceRange.Start.Line,
					Character: occurrenceRange.Start.Character,
				},
				End: types.Position{
					Line:      occurrenceRange.End.Line,
					Character: occurrenceRange.End.Character,
				},
			},
			Symbol:      symbolID,
			SymbolRoles: types.SymbolRoleDefinition,
			SyntaxKind:  m.mapLSPSymbolKindToSyntaxKind(symbol.Kind),
		}

		// Store the full range as SelectionRange for context
		if symbol.SelectionRange != nil && symbol.SelectionRange != &symbol.Location.Range {
			occurrence.SelectionRange = &types.Range{
				Start: types.Position{
					Line:      symbol.Location.Range.Start.Line,
					Character: symbol.Location.Range.Start.Character,
				},
				End: types.Position{
					Line:      symbol.Location.Range.End.Line,
					Character: symbol.Location.Range.End.Character,
				},
			}
		}

		occurrences = append(occurrences, occurrence)

		// Create symbol information with enhanced metadata using precise position
		symbolInfo := scip.SCIPSymbolInformation{
			Symbol:        symbolID,
			DisplayName:   symbol.Name,
			Kind:          m.mapLSPSymbolKindToSCIPKind(symbol.Kind),
			Documentation: m.getSymbolDocumentation(ctx, uri, symbolPosition),
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
		symbolName = m.getSymbolNameAtPosition(ctx, uri, position, language)
	}
	if symbolName == "" && len(locations) > 0 {
		symbolName = m.getIdentifierFromLocation(ctx, locations[0])
	}
	if symbolName == "" {
		return
	}

	// Create SCIP occurrences for each definition location
	occurrences := make([]scip.SCIPOccurrence, 0, len(locations))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(locations))

	for _, location := range locations {
		// Generate SCIP symbol ID
		symbolID := m.generateSymbolID(language, location.URI, symbolName, types.Function)

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
			Relationships: m.getSymbolRelationships(ctx, location.URI, symbolName, types.Function),
		}
		symbolInfos = append(symbolInfos, symbolInfo)
	}

	// Group and store occurrences by document URI
	documentGroups := m.groupReferencesByDocument(occurrences, locations)
	for uri, groupedOccurrences := range documentGroups {
		m.storeDocumentOccurrences(ctx, uri, language, groupedOccurrences, symbolInfos)
	}

}

// indexReferencesAsOccurrences indexes reference results as SCIP occurrences with reference roles
func (m *LSPManager) indexReferencesAsOccurrences(ctx context.Context, uri, language string, params, result interface{}) {
	locations := m.parseLocationResult(result)
	if len(locations) == 0 {
		return
	}

	// Extract position and resolve a stable symbol name via hover if possible
	position, symbolName := m.extractPositionAndSymbolFromParams(params)
	if symbolName == "" {
		symbolName = m.getSymbolNameAtPosition(ctx, uri, position, language)
	}
	if symbolName == "" && len(locations) > 0 {
		symbolName = m.getIdentifierFromLocation(ctx, locations[0])
	}
	if symbolName == "" {
		return
	}

	// Try to reuse an existing SCIP symbol ID from storage to ensure consistency with definitions
	stableSymbolID := ""
	if m.scipCache != nil {
		if scipStorage := m.scipCache.GetSCIPStorage(); scipStorage != nil {
			if infos, err := scipStorage.SearchSymbols(ctx, symbolName, 10); err == nil && len(infos) > 0 {
				// Prefer a symbol whose definition resides in the same document as 'uri'
				for _, info := range infos {
					defOccs, _ := scipStorage.GetDefinitions(ctx, info.Symbol)
					if len(defOccs) == 0 {
						continue
					}
					// Find the document URI for the first definition occurrence
					defURI := m.findDocumentURIForOccurrence(ctx, scipStorage, defOccs[0])
					if defURI == uri {
						stableSymbolID = info.Symbol
						break
					}
				}
				if stableSymbolID == "" {
					stableSymbolID = infos[0].Symbol
				}
			}
		}
	}
	if stableSymbolID == "" {
		// Fallback: generate an ID based on the definition URI (not the reference document)
		stableSymbolID = m.generateSymbolID(language, uri, symbolName, types.Function)
	}

	occurrences := make([]scip.SCIPOccurrence, 0, len(locations))
	symbolInfos := make([]scip.SCIPSymbolInformation, 0, 1)

	for _, location := range locations {
		occurrence := scip.SCIPOccurrence{
			Range: types.Range{
				Start: types.Position{Line: int32(location.Range.Start.Line), Character: int32(location.Range.Start.Character)},
				End:   types.Position{Line: int32(location.Range.End.Line), Character: int32(location.Range.End.Character)},
			},
			Symbol:      stableSymbolID,
			SymbolRoles: types.SymbolRoleReadAccess,
			SyntaxKind:  types.SyntaxKindIdentifierFunction,
		}
		occurrences = append(occurrences, occurrence)
	}

	// Create symbol info once using the document of the definition (uri)
	if len(occurrences) > 0 {
		first := locations[0]
		symbolInfos = append(symbolInfos, scip.SCIPSymbolInformation{
			Symbol:        stableSymbolID,
			DisplayName:   symbolName,
			Kind:          scip.SCIPSymbolKindFunction,
			Documentation: m.getSymbolDocumentation(ctx, uri, first.Range.Start),
			Relationships: m.getSymbolRelationships(ctx, uri, symbolName, types.Function),
		})
	}

	documentGroups := m.groupReferencesByDocument(occurrences, locations)
	for docURI, groupedOccurrences := range documentGroups {
		m.storeDocumentOccurrences(ctx, docURI, language, groupedOccurrences, symbolInfos)
	}
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
		// Expand symbol ranges to get full definitions
		uriSymbols = m.expandSymbolRanges(ctx, uriSymbols, uri, language)

		// Convert symbols to SCIP occurrences with definition roles
		occurrences := make([]scip.SCIPOccurrence, 0, len(uriSymbols))
		symbolInfos := make([]scip.SCIPSymbolInformation, 0, len(uriSymbols))

		for _, symbol := range uriSymbols {
			// Skip builtin types that don't have actual definitions in the codebase
			if m.isBuiltinType(symbol.Name) {
				continue
			}

			// Generate SCIP symbol ID
			symbolID := m.generateSymbolID(language, uri, symbol.Name, symbol.Kind)

			// Use SelectionRange if available for more precise position
			symbolPosition := symbol.Location.Range.Start
			occurrenceRange := symbol.Location.Range
			if symbol.SelectionRange != nil {
				symbolPosition = symbol.SelectionRange.Start
				occurrenceRange = *symbol.SelectionRange
			}

			// Create SCIP occurrence with definition role using precise position
			occurrence := scip.SCIPOccurrence{
				Range: types.Range{
					Start: types.Position{
						Line:      int32(occurrenceRange.Start.Line),
						Character: int32(occurrenceRange.Start.Character),
					},
					End: types.Position{
						Line:      int32(occurrenceRange.End.Line),
						Character: int32(occurrenceRange.End.Character),
					},
				},
				Symbol:      symbolID,
				SymbolRoles: types.SymbolRoleDefinition,
				SyntaxKind:  m.mapLSPSymbolKindToSyntaxKind(symbol.Kind),
			}
			occurrences = append(occurrences, occurrence)

			// Create symbol information with enhanced metadata using precise position
			symbolInfo := scip.SCIPSymbolInformation{
				Symbol:        symbolID,
				DisplayName:   symbol.Name,
				Kind:          m.mapLSPSymbolKindToSCIPKind(symbol.Kind),
				Documentation: m.getSymbolDocumentation(ctx, uri, symbolPosition),
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
	fullRangeMap := make(map[string]types.Range)
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
func (m *LSPManager) collectFullRanges(symbols []*lsp.DocumentSymbol, rangeMap map[string]types.Range) {
	for _, symbol := range symbols {
		// Store the full range for this symbol
		rangeMap[symbol.Name] = symbol.Range

		// Recursively process children
		if len(symbol.Children) > 0 {
			m.collectFullRanges(symbol.Children, rangeMap)
		}
	}
}

// parseSymbols parses document symbols from various response formats
func (m *LSPManager) parseSymbols(result interface{}, uri string) []lsp.SymbolInformation {
	var symbols []lsp.SymbolInformation

	// Handle different response types
	switch v := result.(type) {
	case nil:
		return symbols
	case []lsp.SymbolInformation:
		return v
	case []lsp.DocumentSymbol:
		for _, docSymbol := range v {
			symbolInfo := lsp.SymbolInformation{
				Name: docSymbol.Name,
				Kind: docSymbol.Kind,
				Location: types.Location{
					URI:   uri,
					Range: docSymbol.Range,
				},
			}
			// Store SelectionRange for later use
			symbolInfo.SelectionRange = &docSymbol.SelectionRange
			symbols = append(symbols, symbolInfo)
		}
		return symbols
	case json.RawMessage:
		var rawData []interface{}
		if err := json.Unmarshal(v, &rawData); err != nil {
			common.LSPLogger.Warn("Failed to unmarshal json.RawMessage for %s: %v", uri, err)
			return symbols
		}
		return m.parseSymbolsArray(rawData, uri)
	case []interface{}:
		return m.parseSymbolsArray(v, uri)
	default:
		common.LSPLogger.Warn("Unexpected response type %T for document symbols from %s", result, uri)
		return symbols
	}
}

// parseSymbolsArray parses an array of interface{} into SymbolInformation
func (m *LSPManager) parseSymbolsArray(items []interface{}, uri string) []lsp.SymbolInformation {
	var symbols []lsp.SymbolInformation

	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}

		var symbol lsp.SymbolInformation
		if err := json.Unmarshal(data, &symbol); err == nil && symbol.Name != "" {
			symbols = append(symbols, symbol)
			continue
		}

		// Try DocumentSymbol format
		var docSymbol lsp.DocumentSymbol
		if err := json.Unmarshal(data, &docSymbol); err == nil && docSymbol.Name != "" {
			symbolInfo := lsp.SymbolInformation{
				Name: docSymbol.Name,
				Kind: docSymbol.Kind,
				Location: types.Location{
					URI:   uri,
					Range: docSymbol.Range,
				},
			}
			// Store SelectionRange for later use
			symbolInfo.SelectionRange = &docSymbol.SelectionRange
			symbols = append(symbols, symbolInfo)
		}
	}

	return symbols
}

// parseLocationResult parses location results from various response formats
func (m *LSPManager) parseLocationResult(result interface{}) []types.Location {
	switch v := result.(type) {
	case nil:
		return nil
	case types.Location:
		return []types.Location{v}
	case []types.Location:
		return v
	case json.RawMessage:
		// Handle json.RawMessage type (common in LSP responses)
		var locations []types.Location
		// First try to unmarshal directly as []types.Location
		if err := json.Unmarshal(v, &locations); err == nil {
			return locations
		}
		// If that fails, try as []interface{} and parse each item
		var rawData []interface{}
		if err := json.Unmarshal(v, &rawData); err == nil {
			for _, item := range rawData {
				if data, err := json.Marshal(item); err == nil {
					var location types.Location
					if err := json.Unmarshal(data, &location); err == nil && location.URI != "" {
						locations = append(locations, location)
					}
				}
			}
		} else {
			// Failed to unmarshal json.RawMessage in parseLocationResult
		}
		return locations
	case []interface{}:
		var locations []types.Location
		for _, item := range v {
			if data, err := json.Marshal(item); err == nil {
				var location types.Location
				if err := json.Unmarshal(data, &location); err == nil && location.URI != "" {
					locations = append(locations, location)
				}
			}
		}
		return locations
	default:
		// Unexpected type in parseLocationResult
		return nil
	}
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

// generateSymbolID generates a unified SCIP symbol ID from symbol information
func (m *LSPManager) generateSymbolID(language, uri, symbolName string, symbolKind types.SymbolKind) string {
	workspaceRoot := m.findWorkspaceRoot(uri)
	filePath := utils.URIToFilePath(uri)
	relPath, _ := filepath.Rel(workspaceRoot, filePath)
	if relPath == "" || strings.HasPrefix(relPath, "..") {
		relPath = filepath.Base(filePath)
	}

	// Simple, consistent descriptor format
	descriptor := fmt.Sprintf("`%s`/%s%s", relPath, symbolName, m.getSymbolSuffix(symbolKind))

	// Use consistent package info
	packageName := "local"
	version := "0.0.0"
	if m.projectInfo != nil {
		packageName = m.projectInfo.Name
		version = m.projectInfo.Version
	}

	return fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, descriptor)
}

// getSymbolSuffix returns the appropriate suffix for a symbol kind
func (m *LSPManager) getSymbolSuffix(kind types.SymbolKind) string {
	switch kind {
	case types.Function, types.Method:
		return "()"
	case types.Class, types.Interface:
		return "#"
	default:
		return "."
	}
}

// findWorkspaceRoot finds the workspace root for a given URI
func (m *LSPManager) findWorkspaceRoot(uri string) string {
	filePath := utils.URIToFilePath(uri)
	return m.findWorkspaceRootFromPath(filePath)
}

// findWorkspaceRootFromPath finds the workspace root by searching up the directory tree for project markers
func (m *LSPManager) findWorkspaceRootFromPath(filePath string) string {
	// Get the directory containing the file
	dir := filepath.Dir(filePath)

	// Project marker files for any language
	projectMarkers := []string{
		"go.mod", "go.work",
		"package.json", "yarn.lock", "package-lock.json", "tsconfig.json",
		"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", ".python-version",
		"pom.xml", "build.gradle", "build.gradle.kts", "gradlew", "mvnw",
		".git", ".gitignore", ".vscode", ".idea",
	}

	// Search up the directory tree
	currentDir := dir
	for {
		// Check if any project marker exists in current directory
		for _, marker := range projectMarkers {
			markerPath := filepath.Join(currentDir, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return currentDir
			}
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached the root directory, no project marker found
			break
		}
		currentDir = parentDir
	}

	// Fallback to the original directory if no project marker found
	return dir
}

// mapLSPSymbolKindToSCIPKind maps LSP symbol kinds to SCIP symbol kinds
func (m *LSPManager) mapLSPSymbolKindToSCIPKind(lspKind types.SymbolKind) scip.SCIPSymbolKind {
	switch lspKind {
	case types.File:
		return scip.SCIPSymbolKindFile
	case types.Module:
		return scip.SCIPSymbolKindModule
	case types.Namespace:
		return scip.SCIPSymbolKindNamespace
	case types.Package:
		return scip.SCIPSymbolKindPackage
	case types.Class:
		return scip.SCIPSymbolKindClass
	case types.Method:
		return scip.SCIPSymbolKindMethod
	case types.Property:
		return scip.SCIPSymbolKindProperty
	case types.Field:
		return scip.SCIPSymbolKindField
	case types.Constructor:
		return scip.SCIPSymbolKindConstructor
	case types.Enum:
		return scip.SCIPSymbolKindEnum
	case types.Interface:
		return scip.SCIPSymbolKindInterface
	case types.Function:
		return scip.SCIPSymbolKindFunction
	case types.Variable:
		return scip.SCIPSymbolKindVariable
	case types.Constant:
		return scip.SCIPSymbolKindConstant
	case types.String:
		return scip.SCIPSymbolKindString
	case types.Number:
		return scip.SCIPSymbolKindNumber
	case types.Boolean:
		return scip.SCIPSymbolKindBoolean
	case types.Array:
		return scip.SCIPSymbolKindArray
	case types.Object:
		return scip.SCIPSymbolKindObject
	case types.Key:
		return scip.SCIPSymbolKindKey
	case types.Null:
		return scip.SCIPSymbolKindNull
	case types.EnumMember:
		return scip.SCIPSymbolKindEnumMember
	case types.Struct:
		return scip.SCIPSymbolKindStruct
	case types.Event:
		return scip.SCIPSymbolKindEvent
	case types.Operator:
		return scip.SCIPSymbolKindOperator
	case types.TypeParameter:
		return scip.SCIPSymbolKindTypeParameter
	default:
		return scip.SCIPSymbolKindUnknown
	}
}

// mapLSPSymbolKindToSyntaxKind maps LSP symbol kinds to SCIP syntax kinds
func (m *LSPManager) mapLSPSymbolKindToSyntaxKind(lspKind types.SymbolKind) types.SyntaxKind {
	switch lspKind {
	case types.Function:
		return types.SyntaxKindIdentifierFunctionDefinition
	case types.Method:
		return types.SyntaxKindIdentifierFunctionDefinition
	case types.Class:
		return types.SyntaxKindIdentifierType
	case types.Interface:
		return types.SyntaxKindIdentifierType
	case types.Variable:
		return types.SyntaxKindIdentifierLocal
	case types.Constant:
		return types.SyntaxKindIdentifierConstant
	case types.Field:
		return types.SyntaxKindIdentifierLocal
	case types.Property:
		return types.SyntaxKindIdentifierLocal
	case types.Namespace:
		return types.SyntaxKindIdentifierNamespace
	case types.Module:
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

	// If we only have occurrences (e.g., references), add them directly to SCIP storage
	if len(occurrences) > 0 && len(symbolInfos) == 0 {
		if mgr, ok := m.scipCache.(*cache.SCIPCacheManager); ok {
			_ = mgr.AddOccurrences(ctx, uri, occurrences)
			return
		}
	}

	// Otherwise, convert to SymbolInformation and index (keeps definitions + metadata)
	var symbols []lsp.SymbolInformation
	for i, occ := range occurrences {
		var displayName string
		var kind types.SymbolKind
		if i < len(symbolInfos) {
			displayName = symbolInfos[i].DisplayName
			kind = m.mapSCIPKindToLSPSymbolKind(symbolInfos[i].Kind)
		} else {
			displayName = occ.Symbol
			kind = types.Variable
		}

		symbols = append(symbols, lsp.SymbolInformation{
			Name: displayName,
			Kind: kind,
			Location: types.Location{
				URI: uri,
				Range: types.Range{
					Start: types.Position{Line: int32(occ.Range.Start.Line), Character: int32(occ.Range.Start.Character)},
					End:   types.Position{Line: int32(occ.Range.End.Line), Character: int32(occ.Range.End.Character)},
				},
			},
		})
	}

	if err := m.scipCache.IndexDocument(ctx, uri, language, symbols); err != nil {
		common.LSPLogger.Warn("Failed to index document symbols for %s: %v", uri, err)
	}

	for i, symbolInfo := range symbolInfos {
		if i < len(symbols) && len(symbolInfo.Documentation) > 0 {
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
			m.scipCache.Store("textDocument/hover", hoverParams, hoverResult)
		}
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

	// Use SCIPCache interface
	if m.scipCache != nil {
		return m.scipCache.GetIndexStats()
	}

	return map[string]interface{}{"status": "unknown"}
}

// RefreshIndex refreshes the SCIP index for given files
func (m *LSPManager) RefreshIndex(ctx context.Context, files []string) error {
	if m.scipCache == nil {
		return fmt.Errorf("SCIP cache not available")
	}

	if m.scipCache != nil {
		return m.scipCache.UpdateIndex(ctx, files)
	}

	return fmt.Errorf("cache does not support index updates")
}

// mapSCIPKindToLSPSymbolKind maps SCIP symbol kinds back to LSP symbol kinds
func (m *LSPManager) mapSCIPKindToLSPSymbolKind(kind scip.SCIPSymbolKind) types.SymbolKind {
	switch kind {
	case scip.SCIPSymbolKindFile:
		return types.File
	case scip.SCIPSymbolKindModule:
		return types.Module
	case scip.SCIPSymbolKindNamespace:
		return types.Namespace
	case scip.SCIPSymbolKindPackage:
		return types.Package
	case scip.SCIPSymbolKindClass:
		return types.Class
	case scip.SCIPSymbolKindMethod:
		return types.Method
	case scip.SCIPSymbolKindProperty:
		return types.Property
	case scip.SCIPSymbolKindField:
		return types.Field
	case scip.SCIPSymbolKindConstructor:
		return types.Constructor
	case scip.SCIPSymbolKindEnum:
		return types.Enum
	case scip.SCIPSymbolKindInterface:
		return types.Interface
	case scip.SCIPSymbolKindFunction:
		return types.Function
	case scip.SCIPSymbolKindVariable:
		return types.Variable
	case scip.SCIPSymbolKindConstant:
		return types.Constant
	case scip.SCIPSymbolKindString:
		return types.String
	case scip.SCIPSymbolKindNumber:
		return types.Number
	case scip.SCIPSymbolKindBoolean:
		return types.Boolean
	case scip.SCIPSymbolKindArray:
		return types.Array
	case scip.SCIPSymbolKindObject:
		return types.Object
	case scip.SCIPSymbolKindKey:
		return types.Key
	case scip.SCIPSymbolKindNull:
		return types.Null
	case scip.SCIPSymbolKindEnumMember:
		return types.EnumMember
	case scip.SCIPSymbolKindStruct:
		return types.Struct
	case scip.SCIPSymbolKindEvent:
		return types.Event
	case scip.SCIPSymbolKindOperator:
		return types.Operator
	case scip.SCIPSymbolKindTypeParameter:
		return types.TypeParameter
	default:
		return types.Variable
	}
}

// getSymbolDocumentation retrieves documentation for a symbol using hover
func (m *LSPManager) getSymbolDocumentation(ctx context.Context, uri string, position types.Position) []string {
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
func (m *LSPManager) getSymbolRelationships(ctx context.Context, uri string, symbolName string, kind types.SymbolKind) []scip.SCIPRelationship {
	var relationships []scip.SCIPRelationship

	// For classes and interfaces, try to find implementations
	if kind == types.Class || kind == types.Interface {
		implementations := m.findImplementations(ctx, uri, symbolName)
		for _, impl := range implementations {
			relationships = append(relationships, scip.SCIPRelationship{
				Symbol:           impl,
				IsImplementation: true,
			})
		}
	}

	// For methods, try to find the type they belong to
	if kind == types.Method || kind == types.Function {
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

// getSymbolNameAtPosition extracts symbol name from document content at a specific position
func (m *LSPManager) getSymbolNameAtPosition(ctx context.Context, uri string, position types.Position, language string) string {
	// Try to get document content at the position using hover
	hoverParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      position.Line,
			"character": position.Character,
		},
	}

	hoverResult, err := m.ProcessRequest(ctx, types.MethodTextDocumentHover, hoverParams)
	if err != nil || hoverResult == nil {
		common.LSPLogger.Debug("Failed to get hover information for symbol at %s:%d:%d", uri, position.Line, position.Character)
		return ""
	}

	// Extract symbol name from hover contents
	if hoverMap, ok := hoverResult.(map[string]interface{}); ok {
		if contents, exists := hoverMap["contents"]; exists {
			switch c := contents.(type) {
			case string:
				return m.extractSymbolFromHoverText(c, language)
			case map[string]interface{}:
				if value, ok := c["value"].(string); ok {
					return m.extractSymbolFromHoverText(value, language)
				}
			}
		}
	}

	return ""
}

// extractSymbolFromHoverText extracts symbol name from hover text content
func (m *LSPManager) extractSymbolFromHoverText(text, language string) string {
	if text == "" {
		return ""
	}

	switch language {
	case "go":
		// Go hover format: "func functionName(params) returnType" or "var variableName type"
		lines := strings.Split(text, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			if strings.HasPrefix(firstLine, "func ") {
				// Extract function name
				parts := strings.Fields(firstLine)
				if len(parts) >= 2 {
					funcName := parts[1]
					if parenIndex := strings.Index(funcName, "("); parenIndex != -1 {
						return funcName[:parenIndex]
					}
					return funcName
				}
			} else if strings.HasPrefix(firstLine, "var ") || strings.HasPrefix(firstLine, "const ") {
				// Extract variable/constant name
				parts := strings.Fields(firstLine)
				if len(parts) >= 2 {
					return parts[1]
				}
			} else if strings.Contains(firstLine, " ") {
				// Try to extract the symbol name (often the last part before type info)
				parts := strings.Fields(firstLine)
				if len(parts) >= 1 {
					return parts[0]
				}
			}
		}
	case "python":
		// Python hover format varies, try to extract from common patterns
		lines := strings.Split(text, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			if strings.HasPrefix(firstLine, "def ") {
				// Extract function name
				parts := strings.Fields(firstLine)
				if len(parts) >= 2 {
					funcName := parts[1]
					if parenIndex := strings.Index(funcName, "("); parenIndex != -1 {
						return funcName[:parenIndex]
					}
					return funcName
				}
			} else if strings.HasPrefix(firstLine, "class ") {
				// Extract class name
				parts := strings.Fields(firstLine)
				if len(parts) >= 2 {
					className := parts[1]
					if colonIndex := strings.Index(className, ":"); colonIndex != -1 {
						return className[:colonIndex]
					}
					return className
				}
			}
		}
	case "javascript", "typescript":
		// JS/TS hover format varies, try common patterns
		lines := strings.Split(text, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			if strings.HasPrefix(firstLine, "function ") {
				// Extract function name
				parts := strings.Fields(firstLine)
				if len(parts) >= 2 {
					funcName := parts[1]
					if parenIndex := strings.Index(funcName, "("); parenIndex != -1 {
						return funcName[:parenIndex]
					}
					return funcName
				}
			} else if strings.Contains(firstLine, ":") {
				// Try TypeScript type annotation format
				if colonIndex := strings.Index(firstLine, ":"); colonIndex > 0 {
					symbolName := strings.TrimSpace(firstLine[:colonIndex])
					return symbolName
				}
			}
		}
	}

	// Fallback: try to extract the first word that looks like an identifier
	words := strings.Fields(text)
	for _, word := range words {
		if len(word) > 0 && (word[0] >= 'a' && word[0] <= 'z' || word[0] >= 'A' && word[0] <= 'Z' || word[0] == '_') {
			// Remove non-identifier characters
			var result strings.Builder
			for _, r := range word {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
					result.WriteRune(r)
				} else {
					break
				}
			}
			if result.Len() > 0 {
				return result.String()
			}
		}
	}

	return ""
}

// groupReferencesByDocument groups occurrences by their document URIs with metadata
func (m *LSPManager) groupReferencesByDocument(occurrences []scip.SCIPOccurrence, locations []types.Location) map[string][]scip.SCIPOccurrence {
	documentGroups := make(map[string][]scip.SCIPOccurrence)

	// Group occurrences by document URI from corresponding locations
	for i, occurrence := range occurrences {
		if i < len(locations) {
			uri := locations[i].URI
			documentGroups[uri] = append(documentGroups[uri], occurrence)
		} else {
			// Fallback: try to extract URI from symbol context or use unknown
			common.LSPLogger.Warn("No corresponding location for occurrence %d, using unknown URI", i)
			documentGroups["unknown"] = append(documentGroups["unknown"], occurrence)
		}
	}

	common.LSPLogger.Debug("Grouped %d occurrences into %d documents", len(occurrences), len(documentGroups))
	return documentGroups
}

// isBuiltinType checks if a symbol name represents a builtin type
func (m *LSPManager) isBuiltinType(symbolName string) bool {
	// Common builtin types across different languages
	// Note: Some type names overlap between languages (e.g., 'int' in Go/Java/Python)
	builtinTypes := map[string]bool{
		// Go builtins
		"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true, "interface{}": true, "map": true, "chan": true,
		"complex64": true, "complex128": true, "uintptr": true,
		// TypeScript/JavaScript builtins
		"number": true, "boolean": true, "object": true, "symbol": true, "undefined": true,
		"null": true, "void": true, "never": true, "unknown": true, "bigint": true,
		"Array": true, "Object": true, "String": true, "Number": true, "Boolean": true,
		"Promise": true, "Date": true, "RegExp": true, "Error": true, "Map": true, "Set": true,
		// Python builtins (only unique ones not already in Go/JS)
		"float": true, "str": true, "list": true, "dict": true,
		"tuple": true, "set": true, "None": true, "bytes": true, "bytearray": true,
		"type": true, "complex": true,
		// Java builtins (only unique ones not already covered)
		"long": true, "short": true, "double": true, "char": true,
		"Integer": true, "Long": true, "Double": true, "Float": true,
		"Character": true, "Byte": true, "Short": true,
	}

	return builtinTypes[symbolName]
}

func (m *LSPManager) getIdentifierFromLocation(ctx context.Context, location types.Location) string {
	path := utils.URIToFilePath(location.URI)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	sl := location.Range.Start.Line
	el := location.Range.End.Line
	if sl < 0 || sl >= int32(len(lines)) {
		return ""
	}
	if el < sl {
		el = sl
	}
	if el >= int32(len(lines)) {
		el = int32(len(lines)) - 1
	}
	sc := location.Range.Start.Character
	ec := location.Range.End.Character
	if sl == el {
		line := lines[sl]
		if sc < 0 || sc > int32(len(line)) {
			sc = 0
		}
		if ec < sc || ec > int32(len(line)) {
			ec = sc
		}
		return m.sanitizeIdentifier(line[sc:ec])
	}
	var b strings.Builder
	first := lines[sl]
	if sc >= 0 && sc <= int32(len(first)) {
		b.WriteString(first[sc:])
	}
	for i := sl + 1; i < el; i++ {
		b.WriteString(lines[i])
	}
	last := lines[el]
	if ec >= 0 && ec <= int32(len(last)) {
		b.WriteString(last[:ec])
	}
	return m.sanitizeIdentifier(b.String())
}

func (m *LSPManager) sanitizeIdentifier(s string) string {
	rs := []rune(s)
	var out []rune
	for i, r := range rs {
		if i == 0 {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
				out = append(out, r)
			}
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			out = append(out, r)
		}
	}
	return strings.TrimSpace(string(out))
}

// findDocumentURIForOccurrence finds the document URI containing the given occurrence
func (m *LSPManager) findDocumentURIForOccurrence(ctx context.Context, storage scip.SCIPDocumentStorage, occ scip.SCIPOccurrence) string {
	docs, err := storage.ListDocuments(ctx)
	if err != nil {
		return ""
	}
	for _, docURI := range docs {
		doc, err := storage.GetDocument(ctx, docURI)
		if err != nil || doc == nil {
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
