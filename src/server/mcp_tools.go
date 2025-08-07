package server

import (
	"context"
	"fmt"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
)

// =============================================================================
// Main Tool Dispatcher
// =============================================================================

// delegateToolCall executes LSP tool calls with cache performance tracking
func (m *MCPServer) delegateToolCall(req *MCPRequest) *MCPResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params: expected object",
				Data:    map[string]interface{}{"received": fmt.Sprintf("%T", req.Params)},
			},
		}
	}

	name, ok := params["name"].(string)
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Missing required parameter: name",
				Data:    map[string]interface{}{"parameter": "name"},
			},
		}
	}

	// MCP server only handles enhanced tools, not basic LSP methods
	// Route to appropriate enhanced tool handler
	var result interface{}
	var err error

	switch name {
	case "findSymbols":
		result, err = m.handleFindSymbols(params)
	case "findReferences":
		result, err = m.handleFindSymbolReferences(params)
	case "findDefinitions":
		result, err = m.handleFindDefinitions(params)
	case "getSymbolInfo":
		result, err = m.handleGetSymbolInfo(params)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Tool not found: %s", name),
				Data:    map[string]interface{}{"tool": name},
			},
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32603,
				Message: err.Error(),
				Data:    map[string]interface{}{"tool": name},
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// =============================================================================
// Enhanced MCP Tools
// =============================================================================

// handleFindSymbols handles the findSymbols tool for finding code symbols
func (m *MCPServer) handleFindSymbols(params map[string]interface{}) (interface{}, error) {

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		common.LSPLogger.Error("Failed to get arguments from params: %+v", params)
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	pattern, ok := arguments["pattern"].(string)
	if !ok || pattern == "" {
		return nil, fmt.Errorf("pattern is required and must be a string")
	}

	filePattern, ok := arguments["filePattern"].(string)
	if !ok || filePattern == "" {
		return nil, fmt.Errorf("filePattern is required and must be a string")
	}

	query := types.SymbolPatternQuery{
		Pattern:     pattern,
		FilePattern: filePattern,
	}

	// Parse symbol kinds
	if kinds, ok := arguments["symbolKinds"].([]interface{}); ok {
		for _, k := range kinds {
			if kind, ok := k.(float64); ok {
				query.SymbolKinds = append(query.SymbolKinds, lsp.SymbolKind(kind))
			}
		}
	}

	// Parse max results
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Parse include code
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Parse symbol roles for enhanced filtering
	var roleFilter *types.SymbolRole
	if roles, ok := arguments["symbolRoles"].([]interface{}); ok {
		var combinedRole types.SymbolRole
		for _, r := range roles {
			if roleStr, ok := r.(string); ok {
				role := parseSymbolRole(roleStr)
				if role != 0 {
					combinedRole = combinedRole.AddRole(role)
				}
			}
		}
		if combinedRole != 0 {
			roleFilter = &combinedRole
		}
	}

	// Execute the search
	ctx := context.Background()
	result, err := m.lspManager.SearchSymbolPattern(ctx, query)
	if err != nil {
		common.LSPLogger.Error("SearchSymbolPattern failed: %v", err)
		return nil, fmt.Errorf("symbol pattern search failed: %w", err)
	}

	// Apply role filtering if specified
	filteredSymbols := result.Symbols
	if roleFilter != nil {
		filteredSymbols = m.filterSymbolsByRole(result.Symbols, *roleFilter)
	}

	// Format the result for MCP with enhanced occurrence metadata
	formattedResult := map[string]interface{}{
		"symbols":    formatEnhancedSymbolsForMCP(filteredSymbols, roleFilter != nil),
		"totalCount": len(filteredSymbols),
		"truncated":  result.Truncated,
		"searchMetadata": map[string]interface{}{
			"pattern":     pattern,
			"filePattern": filePattern,
			"roleFilter":  formatRoleFilter(roleFilter),
			"searchType":  "enhanced_pattern_search",
		},
	}

	// Wrap result in content array for MCP response
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d symbols matching pattern '%s'", len(filteredSymbols), pattern),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findSymbols"),
			},
		},
	}, nil
}

// handleFindSymbolReferences handles the findReferences tool for finding all references to a symbol
func (m *MCPServer) handleFindSymbolReferences(params map[string]interface{}) (interface{}, error) {

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		common.LSPLogger.Error("Failed to get arguments from params: %+v", params)
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	query := SymbolReferenceQuery{
		SymbolName: symbolName,
	}

	// Parse file pattern
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	} else {
		query.FilePattern = "**/*"
	}

	// Parse include definition
	if includeDefinition, ok := arguments["includeDefinition"].(bool); ok {
		query.IncludeDefinition = includeDefinition
	}

	// Parse max results
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	} else {
		query.MaxResults = 100
	}

	// Parse include code
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Parse exact match
	if exactMatch, ok := arguments["exactMatch"].(bool); ok {
		query.ExactMatch = exactMatch
	}

	// Parse include related
	if includeRelated, ok := arguments["includeRelated"].(bool); ok {
		query.IncludeRelated = includeRelated
	}

	// Parse symbol roles for enhanced filtering
	if roles, ok := arguments["symbolRoles"].([]interface{}); ok {
		var combinedRole types.SymbolRole
		for _, r := range roles {
			if roleStr, ok := r.(string); ok {
				role := parseSymbolRole(roleStr)
				if role != 0 {
					combinedRole = combinedRole.AddRole(role)
				}
			}
		}
		if combinedRole != 0 {
			query.FilterByRole = &combinedRole
		}
	}

	// Execute the search
	ctx := context.Background()
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		common.LSPLogger.Error("SearchSymbolReferences failed: %v", err)
		return nil, fmt.Errorf("symbol references search failed: %w", err)
	}

	// Format the result for MCP with enhanced metadata
	formattedResult := map[string]interface{}{
		"references":       formatEnhancedReferencesForMCP(result.References),
		"totalCount":       result.TotalCount,
		"truncated":        result.Truncated,
		"definitionCount":  result.DefinitionCount,
		"readAccessCount":  result.ReadAccessCount,
		"writeAccessCount": result.WriteAccessCount,
		"implementations":  formatEnhancedReferencesForMCP(result.Implementations),
		"relatedSymbols":   result.RelatedSymbols,
		"searchMetadata":   result.SearchMetadata,
	}

	// Wrap result in content array for MCP response
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d references to symbol '%s' (%d definitions, %d reads, %d writes)",
					result.TotalCount, symbolName, result.DefinitionCount, result.ReadAccessCount, result.WriteAccessCount),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findReferences"),
			},
		},
	}, nil
}

// handleFindDefinitions handles the findDefinitions tool for finding symbol definitions
func (m *MCPServer) handleFindDefinitions(params map[string]interface{}) (interface{}, error) {

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	// Create a reference query that only includes definitions
	query := SymbolReferenceQuery{
		SymbolName:        symbolName,
		FilePattern:       "**/*",
		IncludeDefinition: true,
		MaxResults:        100,
		ExactMatch:        false,
	}

	// Set file pattern if provided
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	}

	// Set exact match if provided
	if exactMatch, ok := arguments["exactMatch"].(bool); ok {
		query.ExactMatch = exactMatch
	}

	// Set max results if provided
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Set include code if provided
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Filter only definition roles
	definitionRole := types.SymbolRoleDefinition
	query.FilterByRole = &definitionRole

	// Execute the search
	ctx := context.Background()
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("symbol definitions search failed: %w", err)
	}

	// Filter to only definitions (extra safety)
	definitions := []ReferenceInfo{}
	for _, ref := range result.References {
		if ref.IsDefinition {
			definitions = append(definitions, ref)
		}
	}

	formattedResult := map[string]interface{}{
		"definitions": formatEnhancedReferencesForMCP(definitions),
		"totalCount":  len(definitions),
		"truncated":   result.Truncated,
		"searchMetadata": map[string]interface{}{
			"symbolName": symbolName,
			"searchType": "definitions_only",
			"exactMatch": query.ExactMatch,
		},
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d definitions for symbol '%s'", len(definitions), symbolName),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findDefinitions"),
			},
		},
	}, nil
}

// handleGetSymbolInfo handles the getSymbolInfo tool for getting detailed symbol information
func (m *MCPServer) handleGetSymbolInfo(params map[string]interface{}) (interface{}, error) {

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	// Parse options
	includeRelationships := true
	if incRel, ok := arguments["includeRelationships"].(bool); ok {
		includeRelationships = incRel
	}

	includeDocumentation := true
	if incDoc, ok := arguments["includeDocumentation"].(bool); ok {
		includeDocumentation = incDoc
	}

	filePattern := "**/*"
	if fp, ok := arguments["filePattern"].(string); ok {
		filePattern = fp
	}

	exactMatch := false
	if em, ok := arguments["exactMatch"].(bool); ok {
		exactMatch = em
	}

	ctx := context.Background()

	// First, find the symbol using SearchSymbolPattern to get detailed info
	symbolQuery := types.SymbolPatternQuery{
		Pattern:     symbolName,
		FilePattern: filePattern,
		MaxResults:  1,
		IncludeCode: true,
	}

	if exactMatch {
		symbolQuery.Pattern = "^" + symbolName + "$"
	}

	symbolResult, err := m.lspManager.SearchSymbolPattern(ctx, symbolQuery)
	if err != nil || len(symbolResult.Symbols) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbolName)
	}

	symbol := symbolResult.Symbols[0]

	// Get references if relationships are requested
	var references []ReferenceInfo
	var implementations []ReferenceInfo
	var relatedSymbols []string

	if includeRelationships {
		refQuery := SymbolReferenceQuery{
			SymbolName:        symbolName,
			FilePattern:       filePattern,
			IncludeDefinition: true,
			IncludeRelated:    true,
			MaxResults:        50, // Limit for detailed info
			ExactMatch:        exactMatch,
		}

		if refResult, err := m.lspManager.SearchSymbolReferences(ctx, refQuery); err == nil {
			references = refResult.References
			implementations = refResult.Implementations
			relatedSymbols = refResult.RelatedSymbols
		}
	}

	// Build detailed symbol info
	symbolInfo := map[string]interface{}{
		"name":       symbol.Name,
		"kind":       getSymbolKindName(symbol.Kind),
		"location":   fmt.Sprintf("%s:%d-%d", symbol.FilePath, symbol.LineNumber, symbol.EndLine),
		"container":  symbol.Container,
		"filePath":   symbol.FilePath,
		"lineNumber": symbol.LineNumber,
		"endLine":    symbol.EndLine,
	}

	if includeDocumentation {
		if symbol.Signature != "" {
			symbolInfo["signature"] = symbol.Signature
		}
		if symbol.Documentation != "" {
			symbolInfo["documentation"] = symbol.Documentation
		}
		if symbol.Code != "" {
			symbolInfo["code"] = symbol.Code
		}
	}

	if includeRelationships {
		symbolInfo["referencesCount"] = len(references)
		symbolInfo["implementationsCount"] = len(implementations)
		symbolInfo["relatedSymbols"] = relatedSymbols

		// Include sample references (up to 5)
		if len(references) > 0 {
			maxSamples := 5
			if len(references) < maxSamples {
				maxSamples = len(references)
			}
			symbolInfo["sampleReferences"] = formatEnhancedReferencesForMCP(references[:maxSamples])
		}

		// Include sample implementations (up to 3)
		if len(implementations) > 0 {
			maxImpls := 3
			if len(implementations) < maxImpls {
				maxImpls = len(implementations)
			}
			symbolInfo["sampleImplementations"] = formatEnhancedReferencesForMCP(implementations[:maxImpls])
		}
	}

	formattedResult := map[string]interface{}{
		"symbolInfo": symbolInfo,
		"searchMetadata": map[string]interface{}{
			"symbolName":           symbolName,
			"searchType":           "detailed_symbol_info",
			"includeRelationships": includeRelationships,
			"includeDocumentation": includeDocumentation,
			"exactMatch":           exactMatch,
		},
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Symbol information for '%s' (%s)", symbolName, getSymbolKindName(symbol.Kind)),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "getSymbolInfo"),
			},
		},
	}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// filterSymbolsByRole filters symbols based on role (placeholder implementation)
func (m *MCPServer) filterSymbolsByRole(symbols []types.EnhancedSymbolInfo, roleFilter types.SymbolRole) []types.EnhancedSymbolInfo {
	// For now, return all symbols since we don't have role data in EnhancedSymbolInfo
	// This would need to be enhanced with actual occurrence-based filtering
	return symbols
}
