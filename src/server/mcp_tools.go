package server

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lsp-gateway/src/internal/constants"
)

const (
	symbolToolTimeout     = 5 * time.Second
	referenceToolTimeout  = 8 * time.Second
	defaultReferenceQuery = "**/*"
)

// FindSymbolsInput describes the structured input accepted by the findSymbols tool.
type FindSymbolsInput struct {
	Pattern     string `json:"pattern"`
	FilePath    string `json:"filePath"`
	MaxResults  int    `json:"maxResults,omitempty"`
	IncludeCode bool   `json:"includeCode,omitempty"`
}

// FindReferencesInput describes the structured input accepted by the findReferences tool.
type FindReferencesInput struct {
	Pattern    string `json:"pattern"`
	FilePath   string `json:"filePath,omitempty"`
	MaxResults int    `json:"maxResults,omitempty"`
}

func (s *MCPServer) registerTools() error {
	if s.impl == nil {
		return fmt.Errorf("mcp implementation not initialised")
	}

	s.toolDefinitions = make(map[string]*mcp.Tool)
	s.toolOrder = nil

	findSymbols := &mcp.Tool{
		Name:        "findSymbols",
		Description: "Search for symbol definitions using the SCIP cache with optional code excerpts.",
	}
	mcp.AddTool[FindSymbolsInput, map[string]any](s.server, findSymbols, s.handleFindSymbols)
	s.toolDefinitions["findSymbols"] = findSymbols
	s.toolOrder = append(s.toolOrder, "findSymbols")

	findReferences := &mcp.Tool{
		Name:        "findReferences",
		Description: "Locate symbol references across the workspace with occurrence context.",
	}
	mcp.AddTool[FindReferencesInput, map[string]any](s.server, findReferences, s.handleFindReferences)
	s.toolDefinitions["findReferences"] = findReferences
	s.toolOrder = append(s.toolOrder, "findReferences")

	return nil
}

func (s *MCPServer) handleFindSymbols(ctx context.Context, _ *mcp.CallToolRequest, input FindSymbolsInput) (*mcp.CallToolResult, map[string]any, error) {
	if input.Pattern == "" {
		return nil, nil, fmt.Errorf("pattern is required")
	}
	if input.FilePath == "" {
		return nil, nil, fmt.Errorf("filePath is required")
	}

	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, symbolToolTimeout)
	defer cancel()

	query := SymbolSearchQuery{
		Pattern:     input.Pattern,
		FilePattern: input.FilePath,
		MaxResults:  maxResults,
		IncludeCode: input.IncludeCode,
	}

	result, err := s.symbolService.FindSymbols(timeoutCtx, query)
	if err != nil {
		return nil, nil, err
	}

	payload := formatSymbolSearchResult(result)
	return nil, payload, nil
}

func (s *MCPServer) handleFindReferences(ctx context.Context, _ *mcp.CallToolRequest, input FindReferencesInput) (*mcp.CallToolResult, map[string]any, error) {
	if input.Pattern == "" {
		return nil, nil, fmt.Errorf("pattern is required")
	}

	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	filePattern := input.FilePath
	if filePattern == "" {
		filePattern = defaultReferenceQuery
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, referenceToolTimeout)
	defer cancel()

	query := SymbolReferenceQuery{
		Pattern:     input.Pattern,
		FilePattern: filePattern,
		MaxResults:  maxResults,
	}

	result, err := s.symbolService.FindReferences(timeoutCtx, query)
	if err != nil {
		return nil, nil, err
	}
	payload := formatReferenceSearchResult(result)
	return nil, payload, nil
}

// formatSymbolSearchResult converts the internal result into a compact JSON payload.
func formatSymbolSearchResult(result *SymbolSearchResult) map[string]interface{} {
	if result == nil {
		return map[string]interface{}{
			"symbols":    []interface{}{},
			"totalCount": 0,
			"truncated":  false,
		}
	}

	formatted := make([]map[string]interface{}, 0, len(result.Symbols))
	for _, sym := range result.Symbols {
		entry := map[string]interface{}{
			"name":     sym.Name,
			"location": formatFileLocation(sym.FilePath, sym.LineNumber, sym.EndLine),
		}
		if sym.Signature != "" {
			entry["signature"] = sym.Signature
		}
		if sym.Documentation != "" {
			entry["documentation"] = sym.Documentation
		}
		if sym.Code != "" {
			entry["code"] = sym.Code
		}
		formatted = append(formatted, entry)
	}

	return map[string]interface{}{
		"symbols":    formatted,
		"totalCount": result.TotalCount,
		"truncated":  result.Truncated,
	}
}

func formatReferenceSearchResult(result *SymbolReferenceResult) map[string]interface{} {
	if result == nil {
		return map[string]interface{}{
			"references": []interface{}{},
			"totalCount": 0,
			"truncated":  false,
		}
	}

	formatted := make([]map[string]interface{}, 0, len(result.References))
	for _, ref := range result.References {
		entry := map[string]interface{}{
			"location": formatFileLocation(ref.FilePath, ref.LineNumber, ref.LineNumber),
		}
		if ref.Text != "" {
			entry["text"] = ref.Text
		} else if ref.Context != "" {
			entry["text"] = ref.Context
		} else {
			if code, err := extractCodeLines(ref.FilePath, ref.LineNumber, ref.LineNumber); err == nil && code != "" {
				entry["text"] = code
			}
		}
		formatted = append(formatted, entry)
	}

	return map[string]interface{}{
		"references": formatted,
		"totalCount": result.TotalCount,
		"truncated":  result.Truncated,
	}
}

func formatFileLocation(filePath string, startLine, endLine int) string {
	if filePath == "" {
		return "unknown:0"
	}
	if endLine < startLine {
		endLine = startLine
	}
	if endLine == startLine {
		return fmt.Sprintf("%s:%d", filePath, startLine)
	}
	return fmt.Sprintf("%s:%d-%d", filePath, startLine, endLine)
}
