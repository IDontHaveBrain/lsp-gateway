package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// StoreMethodResult stores LSP method results as SCIP occurrences with proper roles
func (m *SCIPCacheManager) StoreMethodResult(method string, params interface{}, response interface{}) error {
	if !m.enabled {
		return nil
	}

	uri := m.extractURI(params)
	if uri == "" {
		return common.ParameterValidationError("could not extract URI from parameters")
	}

	switch method {
	case "textDocument/definition":
		return m.storeDefinitionResult(uri, response)
	case "textDocument/references":
		return m.storeReferencesResult(uri, response)
	case "textDocument/hover":
		return m.storeHoverResult(uri, params, response)
	case "textDocument/documentSymbol":
		return m.storeDocumentSymbolResult(uri, response)
	case "workspace/symbol":
		return m.storeWorkspaceSymbolResult(response)
	case "textDocument/completion":
		return m.storeCompletionResult(uri, params, response)
	default:
		// Method not cached in SCIP storage
		return nil
	}
}

// storeDefinitionResult stores definition results as SCIP occurrences
func (m *SCIPCacheManager) storeDefinitionResult(uri string, response interface{}) error {
	locations, ok := response.([]types.Location)
	if !ok {
		return fmt.Errorf("invalid definition response type")
	}

	for _, location := range locations {
		ctx := NewConversionContext(location.URI, m.converter.DetectLanguageFromURI(location.URI)).
			WithSymbolRoles(types.SymbolRoleDefinition)
		ctx.DefaultSize = 100

		// Create document with single occurrence
		scipDoc, err := m.converter.ConvertLocationsToSCIPDocument(ctx, []types.Location{location}, nil)
		if err != nil {
			return fmt.Errorf("failed to convert definition location: %w", err)
		}

		if err := m.converter.ValidateAndStore(m.scipStorage, scipDoc, "definition"); err != nil {
			return err
		}
	}

	return nil
}

// storeReferencesResult stores reference results as SCIP occurrences
func (m *SCIPCacheManager) storeReferencesResult(uri string, response interface{}) error {
	locations, ok := response.([]types.Location)
	if !ok {
		return fmt.Errorf("invalid references response type")
	}

	// Group locations by document URI
	docLocations := make(map[string][]types.Location)
	for _, location := range locations {
		docLocations[location.URI] = append(docLocations[location.URI], location)
	}

	// Store each document with its references
	for docURI, locationsForDoc := range docLocations {
		ctx := NewConversionContext(docURI, m.converter.DetectLanguageFromURI(docURI)).
			WithSymbolRoles(types.SymbolRoleReadAccess) // References are read access
		ctx.DefaultSize = int64(len(locationsForDoc) * 50)

		// Create custom symbol ID generator that uses the original URI
		symbolIDGenerator := func(location types.Location) string {
			return m.converter.GenerateLocationBasedSymbolID(uri, location.Range.Start.Line, location.Range.Start.Character)
		}

		scipDoc, err := m.converter.ConvertLocationsToSCIPDocument(ctx, locationsForDoc, symbolIDGenerator)
		if err != nil {
			return fmt.Errorf("failed to convert reference locations: %w", err)
		}

		if err := m.converter.ValidateAndStore(m.scipStorage, scipDoc, "references"); err != nil {
			return err
		}
	}

	return nil
}

// storeHoverResult stores hover information as symbol metadata
func (m *SCIPCacheManager) storeHoverResult(uri string, params, response interface{}) error {
	hover, ok := response.(*lsp.Hover)
	if !ok {
		return fmt.Errorf("invalid hover response type")
	}

	// Extract position from parameters
	position, err := m.extractPositionFromParams(params)
	if err != nil {
		return fmt.Errorf("failed to extract position: %w", err)
	}

	// Generate symbol ID
	symbolID := fmt.Sprintf("symbol_%s_%d_%d", uri, position.Line, position.Character)

	// Create or update symbol information with documentation
	var docText string
	if strContent, ok := hover.Contents.(string); ok {
		docText = strContent
	} else {
		docText = "hover information"
	}

	symbolInfo := scip.SCIPSymbolInformation{
		Symbol:        symbolID,
		DisplayName:   "hover_symbol", // Could be extracted from hover contents
		Documentation: []string{docText},
	}

	// Create a simple document with just the symbol information
	scipDoc := &scip.SCIPDocument{
		URI:               uri,
		Language:          m.converter.DetectLanguageFromURI(uri),
		LastModified:      time.Now(),
		Size:              100, // Rough estimate for hover info
		Occurrences:       []scip.SCIPOccurrence{},
		SymbolInformation: []scip.SCIPSymbolInformation{symbolInfo},
	}

	if err := m.scipStorage.StoreDocument(context.Background(), scipDoc); err != nil {
		return fmt.Errorf("failed to store hover information: %w", err)
	}

	return nil
}

// storeDocumentSymbolResult stores document symbols as definition occurrences
func (m *SCIPCacheManager) storeDocumentSymbolResult(uri string, response interface{}) error {
	symbols, ok := response.([]types.SymbolInformation)
	if !ok {
		return fmt.Errorf("invalid document symbol response type")
	}

	language := m.converter.DetectLanguageFromURI(uri)
	scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbols)
	if err != nil {
		return fmt.Errorf("failed to convert symbols: %w", err)
	}

	if err := m.converter.ValidateAndStore(m.scipStorage, scipDoc, "document symbols"); err != nil {
		return err
	}

	return nil
}

// storeWorkspaceSymbolResult stores workspace symbols
func (m *SCIPCacheManager) storeWorkspaceSymbolResult(response interface{}) error {
	symbols, ok := response.([]types.SymbolInformation)
	if !ok {
		return fmt.Errorf("invalid workspace symbol response type")
	}

	// Group symbols by document
	docSymbols := make(map[string][]types.SymbolInformation)
	for _, symbol := range symbols {
		docSymbols[symbol.Location.URI] = append(docSymbols[symbol.Location.URI], symbol)
	}

	// Store each document
	for uri, symbolsForDoc := range docSymbols {
		language := m.converter.DetectLanguageFromURI(uri)
		scipDoc, err := m.convertLSPSymbolsToSCIPDocument(uri, language, symbolsForDoc)
		if err != nil {
			common.LSPLogger.Warn("Failed to convert workspace symbols for %s: %v", uri, err)
			continue
		}

		if err := m.converter.StoreSCIPDocument(m.scipStorage, scipDoc); err != nil {
			common.LSPLogger.Warn("Failed to store workspace symbols for %s: %v", uri, err)
		}
	}

	return nil
}

// storeCompletionResult stores completion items (not typically cached as occurrences)
func (m *SCIPCacheManager) storeCompletionResult(uri string, params, response interface{}) error {
	// Completion items are typically not stored as occurrences
	// This is a placeholder for potential future enhancements
	// Completion items not stored in occurrence cache
	return nil
}

// extractPositionFromParams extracts position from LSP parameters
func (m *SCIPCacheManager) extractPositionFromParams(params interface{}) (types.Position, error) {
	// This is a simplified extraction - in practice you'd handle different parameter types
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return types.Position{}, common.ParameterValidationError("invalid parameters format")
	}

	positionMap, ok := paramsMap["position"].(map[string]interface{})
	if !ok {
		return types.Position{}, common.ParameterValidationError("no position in parameters")
	}

	line, ok := positionMap["line"].(float64)
	if !ok {
		return types.Position{}, common.ParameterValidationError("invalid line in position")
	}

	character, ok := positionMap["character"].(float64)
	if !ok {
		return types.Position{}, common.ParameterValidationError("invalid character in position")
	}

	return types.Position{
		Line:      int32(line),
		Character: int32(character),
	}, nil
}

// formatHoverFromSCIPSymbolInfo formats SCIP symbol information for hover display
func (m *SCIPCacheManager) formatHoverFromSCIPSymbolInfo(symbolInfo *scip.SCIPSymbolInformation) string {
	var content strings.Builder

	// Add symbol name and kind
	content.WriteString(fmt.Sprintf("**%s**\n\n", symbolInfo.DisplayName))

	// Add documentation
	if len(symbolInfo.Documentation) > 0 {
		content.WriteString(strings.Join(symbolInfo.Documentation, "\n"))
	}

	// Add signature documentation if available
	if symbolInfo.SignatureDocumentation.Text != "" {
		content.WriteString("\n\n---\n\n")
		content.WriteString(symbolInfo.SignatureDocumentation.Text)
	}

	return content.String()
}

// formatSymbolDetail formats symbol detail for completion items
func (m *SCIPCacheManager) formatSymbolDetail(symbolInfo *scip.SCIPSymbolInformation) string {
	if symbolInfo.SignatureDocumentation.Text != "" {
		return symbolInfo.SignatureDocumentation.Text
	}
	return fmt.Sprintf("%d", symbolInfo.Kind)
}