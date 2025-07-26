package mcp

import (
	"context"
	"fmt"
	"time"
)

const (
	LSPMethodHover = "textDocument/hover"
)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPErrorCode int

const (
	MCPErrorParseError     MCPErrorCode = -32700
	MCPErrorInvalidRequest MCPErrorCode = -32600
	MCPErrorMethodNotFound MCPErrorCode = -32601
	MCPErrorInvalidParams  MCPErrorCode = -32602
	MCPErrorInternalError  MCPErrorCode = -32603

	MCPErrorServerError        MCPErrorCode = -32000
	MCPErrorInvalidURI         MCPErrorCode = -32001
	MCPErrorUnsupportedFeature MCPErrorCode = -32002
	MCPErrorTimeout            MCPErrorCode = -32003
	MCPErrorConnectionFailed   MCPErrorCode = -32004
	MCPErrorValidationFailed   MCPErrorCode = -32005
	MCPErrorLSPServerError     MCPErrorCode = -32006
	MCPErrorResourceNotFound   MCPErrorCode = -32007
	MCPErrorTooManyRequests    MCPErrorCode = -32008
)

type LSPErrorCode int

const (
	LSPErrorUnknownErrorCode      LSPErrorCode = -32001
	LSPErrorRequestFailed         LSPErrorCode = -32803
	LSPErrorServerNotInitialized  LSPErrorCode = -32002
	LSPErrorRequestCancelled      LSPErrorCode = -32800
	LSPErrorContentModified       LSPErrorCode = -32801
	LSPErrorServerCancelled       LSPErrorCode = -32802
	LSPErrorInvalidFilePath       LSPErrorCode = -32900
	LSPErrorInvalidPosition       LSPErrorCode = -32901
	LSPErrorTextDocumentNotFound  LSPErrorCode = -32902
	LSPErrorSymbolNotFound        LSPErrorCode = -32903
	LSPErrorWorkspaceNotAvailable LSPErrorCode = -32904
)

type StructuredError struct {
	Code        MCPErrorCode           `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Retryable   bool                   `json:"retryable"`
}

type ValidationError struct {
	Field    string      `json:"field"`
	Value    interface{} `json:"value"`
	Expected string      `json:"expected"`
	Actual   string      `json:"actual"`
	Message  string      `json:"message"`
}

type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentBlock    `json:"content"`
	IsError bool              `json:"isError,omitempty"`
	Error   *StructuredError  `json:"error,omitempty"`
	Meta    *ResponseMetadata `json:"meta,omitempty"`
}

type ResponseMetadata struct {
	Timestamp   string                 `json:"timestamp"`
	Duration    string                 `json:"duration,omitempty"`
	LSPMethod   string                 `json:"lspMethod"`
	CacheHit    bool                   `json:"cacheHit,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	RequestInfo map[string]interface{} `json:"requestInfo,omitempty"`
}

type ContentBlock struct {
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

type ToolHandler struct {
	Client *LSPGatewayClient
	Tools  map[string]Tool
}

func NewToolHandler(client *LSPGatewayClient) *ToolHandler {
	handler := &ToolHandler{
		Client: client,
		Tools:  make(map[string]Tool),
	}

	handler.RegisterDefaultTools()

	return handler
}

func (h *ToolHandler) RegisterDefaultTools() {
	h.Tools["goto_definition"] = Tool{
		Name:        "goto_definition",
		Description: "Navigate to the definition of a symbol at a specific position in a file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	h.Tools["find_references"] = Tool{
		Name:        "find_references",
		Description: "Find all references to a symbol at a specific position in a file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
				"includeDeclaration": map[string]interface{}{
					"type":        "boolean",
					"description": "Include the declaration in the results",
					"default":     true,
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	h.Tools["get_hover_info"] = Tool{
		Name:        "get_hover_info",
		Description: "Get hover information for a symbol at a specific position in a file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	h.Tools["get_document_symbols"] = Tool{
		Name:        "get_document_symbols",
		Description: "Get all symbols in a document",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
			},
			"required": []string{"uri"},
		},
	}

	h.Tools["search_workspace_symbols"] = Tool{
		Name:        "search_workspace_symbols",
		Description: "Search for symbols in the workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query for symbol names",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (h *ToolHandler) ListTools() []Tool {
	tools := make([]Tool, 0, len(h.Tools))
	for _, tool := range h.Tools {
		tools = append(tools, tool)
	}
	return tools
}

func (h *ToolHandler) CallTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	tool, exists := h.Tools[call.Name]
	if !exists {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Unknown tool: %s", call.Name),
			}},
			IsError: true,
		}, nil
	}

	switch call.Name {
	case "goto_definition":
		return h.handleGotoDefinition(ctx, call.Arguments)
	case "find_references":
		return h.handleFindReferences(ctx, call.Arguments)
	case "get_hover_info":
		return h.handleGetHoverInfo(ctx, call.Arguments)
	case "get_document_symbols":
		return h.handleGetDocumentSymbols(ctx, call.Arguments)
	case "search_workspace_symbols":
		return h.handleSearchWorkspaceSymbols(ctx, call.Arguments)
	default:
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Tool %s not implemented", tool.Name),
			}},
			IsError: true,
		}, nil
	}
}

func (h *ToolHandler) handleGotoDefinition(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleGotoDefinitionWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleGotoDefinitionWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
		"position": map[string]interface{}{
			"line":      args["line"],
			"character": args["character"],
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error getting definition: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseDefinitionResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "textDocument/definition",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceDefinitionWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleFindReferences(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleFindReferencesWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleFindReferencesWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	includeDeclaration, ok := args["includeDeclaration"].(bool)
	if !ok {
		includeDeclaration = true
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
		"position": map[string]interface{}{
			"line":      args["line"],
			"character": args["character"],
		},
		"context": map[string]interface{}{
			"includeDeclaration": includeDeclaration,
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, "textDocument/references", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error finding references: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseReferencesResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "textDocument/references",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceReferencesWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleGetHoverInfo(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleGetHoverInfoWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleGetHoverInfoWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
		"position": map[string]interface{}{
			"line":      args["line"],
			"character": args["character"],
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, LSPMethodHover, params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error getting hover info: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseHoverResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: LSPMethodHover,
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceHoverWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleGetDocumentSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleGetDocumentSymbolsWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleGetDocumentSymbolsWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error getting document symbols: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseDocumentSymbolsResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "textDocument/documentSymbol",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceDocumentSymbolsWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleSearchWorkspaceSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleSearchWorkspaceSymbolsWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleSearchWorkspaceSymbolsWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	params := map[string]interface{}{
		"query": args["query"],
	}

	result, err := h.Client.SendLSPRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error searching workspace symbols: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseWorkspaceSymbolsResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "workspace/symbol",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceWorkspaceSymbolsWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}
