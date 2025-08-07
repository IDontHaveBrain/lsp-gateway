package server

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
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

	// Execute the search using SCIP cache directly with fallback
	ctx := context.Background()
	var result *types.SymbolPatternResult
	var err error

	// Try direct SCIP cache first for better performance
	if m.scipCache != nil {
		maxResults := query.MaxResults
		if maxResults <= 0 {
			maxResults = 100
		}

		scipResults, scipErr := m.scipCache.SearchSymbols(ctx, pattern, filePattern, maxResults)
		if scipErr == nil && len(scipResults) > 0 {
			// Convert SCIP results to SymbolPatternResult format
			symbols := make([]types.EnhancedSymbolInfo, 0, len(scipResults))
			for _, scipResult := range scipResults {
				// Handle enhanced result format with occurrence data
				if enhancedData, ok := scipResult.(map[string]interface{}); ok {
					// Extract symbol info and occurrence
					var symbolInfo scip.SCIPSymbolInformation
					var occurrence *scip.SCIPOccurrence

					if si, ok := enhancedData["symbolInfo"].(scip.SCIPSymbolInformation); ok {
						symbolInfo = si
					}
					if occ, ok := enhancedData["occurrence"].(*scip.SCIPOccurrence); ok {
						occurrence = occ
					}

					// Extract file path and range from occurrence
					filePath := ""
					lineNumber := 0
					endLine := 0

					if occurrence != nil {
						// Get file path from document URI
						if docURI, ok := enhancedData["filePath"].(string); ok {
							if strings.HasPrefix(docURI, "file://") {
								filePath = strings.TrimPrefix(docURI, "file://")
							} else {
								filePath = docURI
							}
						}

						// Extract line range from occurrence
						lineNumber = int(occurrence.Range.Start.Line)
						endLine = int(occurrence.Range.End.Line)
					}

					// Convert SCIP kind to LSP kind
					lspKind := lsp.Variable // Default
					switch symbolInfo.Kind {
					case scip.SCIPSymbolKindClass:
						lspKind = lsp.Class
					case scip.SCIPSymbolKindMethod:
						lspKind = lsp.Method
					case scip.SCIPSymbolKindFunction:
						lspKind = lsp.Function
					case scip.SCIPSymbolKindNamespace:
						lspKind = lsp.Namespace
					case scip.SCIPSymbolKindModule:
						lspKind = lsp.Module
					case scip.SCIPSymbolKindInterface:
						lspKind = lsp.Interface
					case scip.SCIPSymbolKindEnum:
						lspKind = lsp.Enum
					case scip.SCIPSymbolKindField:
						lspKind = lsp.Field
					case scip.SCIPSymbolKindProperty:
						lspKind = lsp.Property
					case scip.SCIPSymbolKindConstructor:
						lspKind = lsp.Constructor
					case scip.SCIPSymbolKindVariable:
						lspKind = lsp.Variable
					case scip.SCIPSymbolKindConstant:
						lspKind = lsp.Constant
					case scip.SCIPSymbolKindStruct:
						lspKind = lsp.Struct
					}

					// Extract documentation
					documentation := ""
					if len(symbolInfo.Documentation) > 0 {
						documentation = strings.Join(symbolInfo.Documentation, "\n")
					}

					// Extract container name from symbol ID if available
					containerName := ""
					if parts := strings.Fields(symbolInfo.Symbol); len(parts) > 3 {
						// Try to extract package/module info as container
						if len(parts) > 1 {
							containerName = parts[1] // Package name
						}
					}

					// Create enhanced symbol info
					enhanced := types.EnhancedSymbolInfo{
						SymbolInformation: lsp.SymbolInformation{
							Name: symbolInfo.DisplayName,
							Kind: lspKind,
							Location: lsp.Location{
								URI: "file://" + filePath,
								Range: lsp.Range{
									Start: lsp.Position{Line: lineNumber, Character: 0},
									End:   lsp.Position{Line: endLine, Character: 0},
								},
							},
							ContainerName: containerName,
						},
						FilePath:      filePath,
						LineNumber:    lineNumber,
						EndLine:       endLine,
						Container:     containerName,
						Documentation: documentation,
					}
					symbols = append(symbols, enhanced)
				} else if scipSymbol, ok := scipResult.(scip.SCIPSymbolInformation); ok {
					// Fallback: handle plain SCIPSymbolInformation without occurrence data
					symbolName := scipSymbol.DisplayName
					symbolID := scipSymbol.Symbol

					// Convert SCIP kind to LSP kind
					lspKind := lsp.Variable // Default
					switch scipSymbol.Kind {
					case scip.SCIPSymbolKindClass:
						lspKind = lsp.Class
					case scip.SCIPSymbolKindMethod:
						lspKind = lsp.Method
					case scip.SCIPSymbolKindFunction:
						lspKind = lsp.Function
					case scip.SCIPSymbolKindNamespace:
						lspKind = lsp.Namespace
					case scip.SCIPSymbolKindModule:
						lspKind = lsp.Module
					case scip.SCIPSymbolKindInterface:
						lspKind = lsp.Interface
					case scip.SCIPSymbolKindEnum:
						lspKind = lsp.Enum
					case scip.SCIPSymbolKindField:
						lspKind = lsp.Field
					case scip.SCIPSymbolKindProperty:
						lspKind = lsp.Property
					case scip.SCIPSymbolKindConstructor:
						lspKind = lsp.Constructor
					case scip.SCIPSymbolKindVariable:
						lspKind = lsp.Variable
					case scip.SCIPSymbolKindConstant:
						lspKind = lsp.Constant
					case scip.SCIPSymbolKindStruct:
						lspKind = lsp.Struct
					}

					// Extract documentation
					documentation := ""
					if len(scipSymbol.Documentation) > 0 {
						documentation = strings.Join(scipSymbol.Documentation, "\n")
					}

					// Fallback: use symbol ID as file path
					filePath := symbolID
					lineNumber := 0
					endLine := 0

					// Create enhanced symbol info
					enhanced := types.EnhancedSymbolInfo{
						SymbolInformation: lsp.SymbolInformation{
							Name: symbolName,
							Kind: lspKind,
							Location: lsp.Location{
								URI: "file://" + filePath,
								Range: lsp.Range{
									Start: lsp.Position{Line: lineNumber, Character: 0},
									End:   lsp.Position{Line: endLine, Character: 0},
								},
							},
						},
						FilePath:      filePath,
						LineNumber:    lineNumber,
						EndLine:       endLine,
						Documentation: documentation,
					}
					symbols = append(symbols, enhanced)
				}
			}

			result = &types.SymbolPatternResult{
				Symbols:    symbols,
				TotalCount: len(symbols),
				Truncated:  len(scipResults) >= maxResults,
			}
		} else {
			// Log cache miss and fall back to LSP manager
			common.LSPLogger.Debug("SCIP cache search failed or returned no results, falling back to LSP: %v", scipErr)
			result, err = m.lspManager.SearchSymbolPattern(ctx, query)
			if err != nil {
				common.LSPLogger.Error("SearchSymbolPattern fallback failed: %v", err)
				return nil, fmt.Errorf("symbol pattern search failed: %w", err)
			}
		}
	} else {
		// No cache available, use LSP manager directly
		result, err = m.lspManager.SearchSymbolPattern(ctx, query)
		if err != nil {
			common.LSPLogger.Error("SearchSymbolPattern failed: %v", err)
			return nil, fmt.Errorf("symbol pattern search failed: %w", err)
		}
	}

	// Apply role filtering if specified
	filteredSymbols := result.Symbols
	if roleFilter != nil {
		filteredSymbols = m.filterSymbolsByRole(result.Symbols, *roleFilter)
	}

	// Add code snippets if requested
	if query.IncludeCode {
		for i := range filteredSymbols {
			if filteredSymbols[i].FilePath != "" && filteredSymbols[i].LineNumber > 0 && filteredSymbols[i].EndLine > 0 {
				code, err := extractCodeLines(filteredSymbols[i].FilePath, filteredSymbols[i].LineNumber, filteredSymbols[i].EndLine)
				if err == nil {
					filteredSymbols[i].Code = code
				}
			}
		}
	}

	// Format the result for MCP with enhanced occurrence metadata
	formattedResult := map[string]interface{}{
		"symbols":    formatEnhancedSymbolsForMCP(filteredSymbols, roleFilter != nil),
		"totalCount": len(filteredSymbols),
		"truncated":  result.Truncated,
	}

	// Wrap result in content array for MCP response
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findSymbols"),
			},
		},
	}, nil
}

// handleFindSymbolReferences handles the findReferences tool for finding all references to symbols matching a pattern
func (m *MCPServer) handleFindSymbolReferences(params map[string]interface{}) (interface{}, error) {

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		common.LSPLogger.Error("Failed to get arguments from params: %+v", params)
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	pattern, ok := arguments["pattern"].(string)
	if !ok || pattern == "" {
		return nil, fmt.Errorf("pattern is required and must be a string")
	}

	filePattern := "**/*" // Default to all files
	if fp, ok := arguments["filePattern"].(string); ok && fp != "" {
		filePattern = fp
	}

	query := SymbolReferenceQuery{
		Pattern:     pattern,
		FilePattern: filePattern,
	}

	// Parse max results
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	} else {
		query.MaxResults = 100
	}

	// Execute the search using SCIP cache directly with fallback
	ctx := context.Background()
	var result *SymbolReferenceResult
	var err error

	// Always use LSP manager for now (SCIP cache conversion needs fixing)
	result, err = m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		common.LSPLogger.Error("SearchSymbolReferences failed: %v", err)
		return nil, fmt.Errorf("symbol references search failed: %w", err)
	}

	// Format the result for MCP with simplified response
	formattedResult := map[string]interface{}{
		"references": formatSimpleReferencesForMCP(result.References),
		"totalCount": result.TotalCount,
		"truncated":  result.Truncated,
	}

	// Wrap result in content array for MCP response
	return map[string]interface{}{
		"content": []map[string]interface{}{
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

	// Create a reference query for definitions using the symbol name as pattern
	pattern := symbolName
	// If exact match is requested, wrap in anchors
	if exactMatch, ok := arguments["exactMatch"].(bool); ok && exactMatch {
		pattern = "^" + symbolName + "$"
	}

	query := SymbolReferenceQuery{
		Pattern:     pattern,
		FilePattern: "**/*",
		MaxResults:  100,
	}

	// Set file pattern if provided
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	}

	// Set max results if provided
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Execute the search using SCIP cache directly with fallback
	ctx := context.Background()
	var result *SymbolReferenceResult
	var err error

	// Try direct SCIP cache first for better performance
	if m.scipCache != nil {
		scipResults, scipErr := m.scipCache.SearchDefinitions(ctx, symbolName, query.FilePattern, query.MaxResults)
		if scipErr == nil && len(scipResults) > 0 {
			// Convert SCIP results to SymbolReferenceResult format for definitions
			references := make([]ReferenceInfo, 0, len(scipResults))

			for _, scipResult := range scipResults {
				// Create a basic ReferenceInfo from SCIP definition result
				refInfo := ReferenceInfo{
					FilePath:     fmt.Sprintf("scip_def_%v", scipResult),
					LineNumber:   0, // Would need to extract from SCIP occurrence
					Column:       0,
					Text:         fmt.Sprintf("Definition: %v", scipResult),
					IsDefinition: true, // All results from SearchDefinitions are definitions
				}
				references = append(references, refInfo)
			}

			result = &SymbolReferenceResult{
				References:      references,
				TotalCount:      len(references),
				Truncated:       len(scipResults) >= query.MaxResults,
				DefinitionCount: len(references), // All are definitions
				SearchMetadata:  map[string]interface{}{"source": "scip_cache_definitions"},
			}
		} else {
			// Log cache miss and fall back to LSP manager
			common.LSPLogger.Debug("SCIP cache definitions search failed or returned no results, falling back to LSP: %v", scipErr)
			result, err = m.lspManager.SearchSymbolReferences(ctx, query)
			if err != nil {
				return nil, fmt.Errorf("symbol definitions search failed: %w", err)
			}
		}
	} else {
		// No cache available, use LSP manager directly
		result, err = m.lspManager.SearchSymbolReferences(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("symbol definitions search failed: %w", err)
		}
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

	// First, try to get symbol info directly from SCIP cache
	var symbol types.EnhancedSymbolInfo
	var symbolFound bool

	if m.scipCache != nil {
		scipResult, scipErr := m.scipCache.GetSymbolInfo(ctx, symbolName, filePattern)
		if scipErr == nil && scipResult != nil {
			// Convert SCIP symbol info to EnhancedSymbolInfo
			// This is a simplified conversion - could be enhanced
			symbol = types.EnhancedSymbolInfo{
				FilePath: fmt.Sprintf("scip_symbol_%v", scipResult),
			}
			symbolFound = true
		} else {
			// Log cache miss and fall back to LSP manager
			common.LSPLogger.Debug("SCIP cache GetSymbolInfo failed, falling back to LSP: %v", scipErr)
		}
	}

	if !symbolFound {
		// Fallback to LSP manager SearchSymbolPattern
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

		symbol = symbolResult.Symbols[0]
	}

	// Get references if relationships are requested
	var references []ReferenceInfo
	var implementations []ReferenceInfo
	var relatedSymbols []string

	if includeRelationships {
		// Create pattern for reference search
		pattern := symbolName
		if exactMatch {
			pattern = "^" + symbolName + "$"
		}

		refQuery := SymbolReferenceQuery{
			Pattern:     pattern,
			FilePattern: filePattern,
			MaxResults:  50, // Limit for detailed info
		}

		// Try SCIP cache first for references
		if m.scipCache != nil {
			scipReferences, scipErr := m.scipCache.SearchReferences(ctx, symbolName, filePattern, 50)
			if scipErr == nil && len(scipReferences) > 0 {
				// Convert SCIP references to ReferenceInfo
				references = make([]ReferenceInfo, 0, len(scipReferences))
				for _, scipRef := range scipReferences {
					refInfo := ReferenceInfo{
						FilePath:   fmt.Sprintf("scip_ref_%v", scipRef),
						LineNumber: 0, // Would need to extract from SCIP occurrence
						Column:     0,
						Text:       fmt.Sprintf("Reference: %v", scipRef),
					}
					references = append(references, refInfo)
				}
			} else {
				// Fallback to LSP manager for references
				common.LSPLogger.Debug("SCIP cache references search failed, falling back to LSP: %v", scipErr)
				if refResult, err := m.lspManager.SearchSymbolReferences(ctx, refQuery); err == nil {
					references = refResult.References
					implementations = refResult.Implementations
					relatedSymbols = refResult.RelatedSymbols
				}
			}
		} else {
			// No cache available, use LSP manager directly
			if refResult, err := m.lspManager.SearchSymbolReferences(ctx, refQuery); err == nil {
				references = refResult.References
				implementations = refResult.Implementations
				relatedSymbols = refResult.RelatedSymbols
			}
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

// extractCodeLines reads lines from a file between start and end line numbers (1-indexed)
func extractCodeLines(filePath string, startLine, endLine int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	currentLine := 1

	for scanner.Scan() {
		if currentLine >= startLine && currentLine <= endLine {
			lines = append(lines, scanner.Text())
		}
		if currentLine > endLine {
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
