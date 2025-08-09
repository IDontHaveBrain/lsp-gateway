package server

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/protocol"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
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
				Code:    protocol.InvalidParams,
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
				Code:    protocol.InvalidParams,
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
				Code:    protocol.MethodNotFound,
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
				Code:    protocol.InternalError,
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
				query.SymbolKinds = append(query.SymbolKinds, types.SymbolKind(kind))
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

	// Execute the search using SCIP cache directly with fallback
	ctx, cancel := common.CreateContext(5 * time.Second)
	defer cancel()
	var result *types.SymbolPatternResult
	var err error

        // Try direct SCIP cache first for better performance
        if m.lspManager.scipCache != nil {
		maxResults := query.MaxResults
		if maxResults <= 0 {
			maxResults = 100
		}

		scipResults, scipErr := m.lspManager.scipCache.SearchSymbols(ctx, pattern, filePattern, maxResults)
		if scipErr == nil && len(scipResults) > 0 {
			// Convert SCIP results to SymbolPatternResult format
			symbols := make([]types.EnhancedSymbolInfo, 0, len(scipResults))
            for _, scipResult := range scipResults {
                // Handle enhanced result format with occurrence data
                if enhancedData, ok := scipResult.(map[string]interface{}); ok {
                    // Extract symbol info and occurrence
                    var symbolInfo scip.SCIPSymbolInformation
                    var occurrence *scip.SCIPOccurrence
                    var rng types.Range

                    if si, ok := enhancedData["symbolInfo"].(scip.SCIPSymbolInformation); ok {
                        symbolInfo = si
                    }
                    if occ, ok := enhancedData["occurrence"].(*scip.SCIPOccurrence); ok {
                        occurrence = occ
                    }
                    // Also try to extract a plain range if provided
                    if r, ok := enhancedData["range"].(types.Range); ok {
                        rng = r
                    } else if rmap, ok := enhancedData["range"].(map[string]interface{}); ok {
                        // Defensive: parse map form if present
                        if s, ok := rmap["start"].(map[string]interface{}); ok {
                            if v, ok := s["line"].(float64); ok {
                                rng.Start.Line = int32(v)
                            }
                            if v, ok := s["character"].(float64); ok {
                                rng.Start.Character = int32(v)
                            }
                        }
                        if e, ok := rmap["end"].(map[string]interface{}); ok {
                            if v, ok := e["line"].(float64); ok {
                                rng.End.Line = int32(v)
                            }
                            if v, ok := e["character"].(float64); ok {
                                rng.End.Character = int32(v)
                            }
                        }
                    }

                    // Extract file path and range from occurrence/range
                    filePath := ""
                    // Prefer explicit documentURI if present
                    if docURI, ok := enhancedData["documentURI"].(string); ok && docURI != "" {
                        filePath = utils.URIToFilePath(docURI)
                    } else if fp, ok := enhancedData["filePath"].(string); ok && fp != "" {
                        filePath = utils.URIToFilePath(fp)
                    }

                    // Determine line range
                    lineNumber := 0
                    endLine := 0
                    if occurrence != nil {
                        lineNumber = int(occurrence.Range.Start.Line)
                        endLine = int(occurrence.Range.End.Line)
                    } else if (rng.Start.Line != 0 || rng.End.Line != 0) || (rng.Start.Character != 0 || rng.End.Character != 0) {
                        lineNumber = int(rng.Start.Line)
                        endLine = int(rng.End.Line)
                    }
                    if endLine < lineNumber {
                        endLine = lineNumber
                    }

					// Convert SCIP kind to LSP kind
					lspKind := types.Variable // Default
					switch symbolInfo.Kind {
					case scip.SCIPSymbolKindClass:
						lspKind = types.Class
					case scip.SCIPSymbolKindMethod:
						lspKind = types.Method
					case scip.SCIPSymbolKindFunction:
						lspKind = types.Function
					case scip.SCIPSymbolKindNamespace:
						lspKind = types.Namespace
					case scip.SCIPSymbolKindModule:
						lspKind = types.Module
					case scip.SCIPSymbolKindInterface:
						lspKind = types.Interface
					case scip.SCIPSymbolKindEnum:
						lspKind = types.Enum
					case scip.SCIPSymbolKindField:
						lspKind = types.Field
					case scip.SCIPSymbolKindProperty:
						lspKind = types.Property
					case scip.SCIPSymbolKindConstructor:
						lspKind = types.Constructor
					case scip.SCIPSymbolKindVariable:
						lspKind = types.Variable
					case scip.SCIPSymbolKindConstant:
						lspKind = types.Constant
					case scip.SCIPSymbolKindStruct:
						lspKind = types.Struct
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
                        SymbolInformation: types.SymbolInformation{
                            Name: symbolInfo.DisplayName,
                            Kind: lspKind,
                            Location: types.Location{
                                URI: "file://" + filePath,
                                Range: types.Range{
                                    Start: types.Position{Line: int32(lineNumber), Character: 0},
                                    End:   types.Position{Line: int32(endLine), Character: 0},
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
					lspKind := types.Variable // Default
					switch scipSymbol.Kind {
					case scip.SCIPSymbolKindClass:
						lspKind = types.Class
					case scip.SCIPSymbolKindMethod:
						lspKind = types.Method
					case scip.SCIPSymbolKindFunction:
						lspKind = types.Function
					case scip.SCIPSymbolKindNamespace:
						lspKind = types.Namespace
					case scip.SCIPSymbolKindModule:
						lspKind = types.Module
					case scip.SCIPSymbolKindInterface:
						lspKind = types.Interface
					case scip.SCIPSymbolKindEnum:
						lspKind = types.Enum
					case scip.SCIPSymbolKindField:
						lspKind = types.Field
					case scip.SCIPSymbolKindProperty:
						lspKind = types.Property
					case scip.SCIPSymbolKindConstructor:
						lspKind = types.Constructor
					case scip.SCIPSymbolKindVariable:
						lspKind = types.Variable
					case scip.SCIPSymbolKindConstant:
						lspKind = types.Constant
					case scip.SCIPSymbolKindStruct:
						lspKind = types.Struct
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
						SymbolInformation: types.SymbolInformation{
							Name: symbolName,
							Kind: lspKind,
							Location: types.Location{
								URI: "file://" + filePath,
								Range: types.Range{
									Start: types.Position{Line: int32(lineNumber), Character: 0},
									End:   types.Position{Line: int32(endLine), Character: 0},
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
			// Cache miss, falling back to LSP manager
			result, err = m.lspManager.SearchSymbolPattern(ctx, query)
			if err != nil {
				return nil, fmt.Errorf("symbol pattern search failed: %w", err)
			}
		}
	} else {
		// No cache available, use LSP manager directly
		result, err = m.lspManager.SearchSymbolPattern(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("symbol pattern search failed: %w", err)
		}
	}

	// No role filtering (symbolRoles removed)
	filteredSymbols := result.Symbols

	// Add code snippets if requested
	if query.IncludeCode {
		for i := range filteredSymbols {
			if filteredSymbols[i].FilePath != "" {
				start := filteredSymbols[i].LineNumber + 1
				end := filteredSymbols[i].EndLine + 1
				if end < start {
					end = start
				}
				code, err := extractCodeLines(filteredSymbols[i].FilePath, start, end)
				if err == nil {
					filteredSymbols[i].Code = code
				}
			}
		}
	}

	// Format the result for MCP with enhanced occurrence metadata
	formattedResult := map[string]interface{}{
		"symbols":    formatEnhancedSymbolsForMCP(filteredSymbols),
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

	// Execute the search using LSP manager which will use SCIP cache if available
	ctx := context.Background()

	// Use LSP manager's SearchSymbolReferences which already handles SCIP cache integration
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		common.LSPLogger.Error("SearchSymbolReferences failed: %v", err)
		// Return empty result instead of error for better UX
		result = &SymbolReferenceResult{
			References: []ReferenceInfo{},
			TotalCount: 0,
			Truncated:  false,
		}
	}

	// Format the result for MCP response (metadata removed by request)
	formattedResult := map[string]interface{}{
		"references": formatEnhancedReferencesForMCP(result.References),
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

// =============================================================================
// Helper Functions
// =============================================================================

// Role-based filtering removed (symbolRoles parameter deprecated)

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
