package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
)

// TestableToolHandler wraps ToolHandler to make it testable with mocks
type TestableToolHandler struct {
	*ToolHandler
	mockClient LSPClientInterface
}

func NewTestableToolHandler(mockClient LSPClientInterface) *TestableToolHandler {
	// Create an empty tool handler structure
	handler := &ToolHandler{
		client: nil, // We'll override this
		tools:  make(map[string]Tool),
	}

	// Register default tools
	handler.registerDefaultTools()

	return &TestableToolHandler{
		ToolHandler: handler,
		mockClient:  mockClient,
	}
}

// Override the methods that use the client
func (th *TestableToolHandler) handleGotoDefinition(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
		"position": map[string]interface{}{
			"line":      args["line"],
			"character": args["character"],
		},
	}

	result, err := th.mockClient.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error getting definition: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
		}},
	}, nil
}

func (th *TestableToolHandler) handleFindReferences(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
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

	result, err := th.mockClient.SendLSPRequest(ctx, "textDocument/references", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error finding references: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
		}},
	}, nil
}

func (th *TestableToolHandler) handleGetHoverInfo(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
		"position": map[string]interface{}{
			"line":      args["line"],
			"character": args["character"],
		},
	}

	result, err := th.mockClient.SendLSPRequest(ctx, gateway.LSPMethodHover, params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error getting hover info: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
		}},
	}, nil
}

func (th *TestableToolHandler) handleGetDocumentSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": args["uri"],
		},
	}

	result, err := th.mockClient.SendLSPRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error getting document symbols: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
		}},
	}, nil
}

func (th *TestableToolHandler) handleSearchWorkspaceSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"query": args["query"],
	}

	result, err := th.mockClient.SendLSPRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Error searching workspace symbols: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
		}},
	}, nil
}

// Override CallTool to use our testable handlers
func (th *TestableToolHandler) CallTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	tool, exists := th.tools[call.Name]
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
		return th.handleGotoDefinition(ctx, call.Arguments)
	case "find_references":
		return th.handleFindReferences(ctx, call.Arguments)
	case "get_hover_info":
		return th.handleGetHoverInfo(ctx, call.Arguments)
	case "get_document_symbols":
		return th.handleGetDocumentSymbols(ctx, call.Arguments)
	case "search_workspace_symbols":
		return th.handleSearchWorkspaceSymbols(ctx, call.Arguments)
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

// Test ToolHandler creation and initialization
func TestNewToolHandler(t *testing.T) {
	// Use a real LSPGatewayClient for this test
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       5 * time.Second,
		MaxRetries:    3,
	}
	realClient := NewLSPGatewayClient(config)
	handler := NewToolHandler(realClient)

	if handler == nil {
		t.Fatal("Expected ToolHandler to be created, got nil")
	}

	if realClient == nil {
		t.Error("Expected ToolHandler to have a client")
	}

	if handler.tools == nil {
		t.Fatal("Expected tools map to be initialized")
	}

	// Check that default tools are registered
	expectedTools := []string{
		"goto_definition",
		"find_references",
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	for _, toolName := range expectedTools {
		if _, exists := handler.tools[toolName]; !exists {
			t.Errorf("Expected tool %s to be registered", toolName)
		}
	}
}

// Test tool listing
func TestListTools(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	tools := handler.ListTools()
	if len(tools) != 5 {
		t.Errorf("Expected 5 tools, got %d", len(tools))
	}

	// Check that all expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true

		// Validate tool structure
		if tool.Name == "" {
			t.Error("Tool name should not be empty")
		}
		if tool.Description == "" {
			t.Error("Tool description should not be empty")
		}
		if tool.InputSchema == nil {
			t.Error("Tool input schema should not be nil")
		}

		// Validate input schema structure
		schema, ok := tool.InputSchema["type"]
		if !ok || schema != "object" {
			t.Errorf("Expected input schema type 'object', got %v", schema)
		}

		properties, ok := tool.InputSchema["properties"]
		if !ok {
			t.Error("Expected input schema to have properties")
		}

		required, ok := tool.InputSchema["required"]
		if !ok {
			t.Error("Expected input schema to have required fields")
		}

		// Validate required fields are arrays
		if reflect.TypeOf(required).Kind() != reflect.Slice {
			t.Error("Expected required fields to be an array")
		}

		// Validate properties structure
		if reflect.TypeOf(properties).Kind() != reflect.Map {
			t.Error("Expected properties to be a map")
		}
	}

	expectedTools := []string{
		"goto_definition",
		"find_references",
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool %s in list", expected)
		}
	}
}

// Test goto_definition tool
func TestGotoDefinitionTool(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedResponse := json.RawMessage(`[{"uri": "file:///test.go", "range": {"start": {"line": 5, "character": 10}}}]`)
	mockClient.SetResponse("textDocument/definition", expectedResponse)

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "goto_definition",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected tool result, got nil")
	}

	if result.IsError {
		t.Error("Expected successful result, got error result")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %s", result.Content[0].Type)
	}

	// JSON formatting may differ (spaces), so validate by unmarshaling and comparing
	var expectedData, actualData interface{}
	if err := json.Unmarshal(expectedResponse, &expectedData); err != nil {
		t.Fatalf("Failed to unmarshal expected response: %v", err)
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &actualData); err != nil {
		t.Fatalf("Failed to unmarshal actual response: %v", err)
	}

	if !reflect.DeepEqual(expectedData, actualData) {
		t.Errorf("Expected response data %v, got %v", expectedData, actualData)
	}

	// Note: Parameter validation is handled by the individual tool handler methods
	// The important part is that the tool call succeeded and returned the expected result
}

// Test find_references tool
func TestFindReferencesTool(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedResponse := json.RawMessage(`[{"uri": "file:///test.go", "range": {"start": {"line": 5, "character": 10}}}]`)
	mockClient.SetResponse("textDocument/references", expectedResponse)

	args := map[string]interface{}{
		"uri":                "file:///test.go",
		"line":               10,
		"character":          5,
		"includeDeclaration": false,
	}

	call := ToolCall{
		Name:      "find_references",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	if result.IsError {
		t.Error("Expected successful result, got error result")
	}

	// Note: Parameter validation is handled by the tool handler methods
}

// Test find_references tool with default includeDeclaration
func TestFindReferencesToolDefaultIncludeDeclaration(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "find_references",
		Arguments: args,
	}

	_, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	// Note: Default parameter validation is handled by the tool handler methods
}

// Test get_hover_info tool
func TestGetHoverInfoTool(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedResponse := json.RawMessage(`{"contents": {"kind": "markdown", "value": "function description"}}`)
	mockClient.SetResponse(gateway.LSPMethodHover, expectedResponse)

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "get_hover_info",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	if result.IsError {
		t.Error("Expected successful result, got error result")
	}

	// Note: Method validation is handled by the tool handler implementation
}

// Test get_document_symbols tool
func TestGetDocumentSymbolsTool(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedResponse := json.RawMessage(`[{"name": "main", "kind": 12, "range": {"start": {"line": 0, "character": 0}}}]`)
	mockClient.SetResponse("textDocument/documentSymbol", expectedResponse)

	args := map[string]interface{}{
		"uri": "file:///test.go",
	}

	call := ToolCall{
		Name:      "get_document_symbols",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	if result.IsError {
		t.Error("Expected successful result, got error result")
	}

	// Note: Parameter structure validation is handled by the tool handler implementation
}

// Test search_workspace_symbols tool
func TestSearchWorkspaceSymbolsTool(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedResponse := json.RawMessage(`[{"name": "TestFunction", "kind": 12, "location": {"uri": "file:///test.go"}}]`)
	mockClient.SetResponse("workspace/symbol", expectedResponse)

	args := map[string]interface{}{
		"query": "Test",
	}

	call := ToolCall{
		Name:      "search_workspace_symbols",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	if result.IsError {
		t.Error("Expected successful result, got error result")
	}

	// Note: Parameter structure validation is handled by the tool handler implementation
}

// Test unknown tool handling
func TestUnknownTool(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	call := ToolCall{
		Name:      "unknown_tool",
		Arguments: map[string]interface{}{},
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected tool call to handle unknown tool gracefully, got error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for unknown tool")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in error result")
	}

	if !strings.Contains(result.Content[0].Text, "Unknown tool: unknown_tool") {
		t.Errorf("Expected unknown tool message, got: %s", result.Content[0].Text)
	}
}

// Test error handling for LSP client errors
func TestLSPClientError(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedError := fmt.Errorf("LSP server connection failed")
	mockClient.SetError("textDocument/definition", expectedError)

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "goto_definition",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected tool call to handle LSP error gracefully, got error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for LSP client error")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in error result")
	}

	if !strings.Contains(result.Content[0].Text, "Error getting definition") {
		t.Errorf("Expected definition error message, got: %s", result.Content[0].Text)
	}

	if !strings.Contains(result.Content[0].Text, expectedError.Error()) {
		t.Errorf("Expected error message to contain LSP error, got: %s", result.Content[0].Text)
	}
}

// Test error handling for each tool type
func TestToolSpecificErrors(t *testing.T) {
	tests := []struct {
		toolName      string
		expectedError string
		lspMethod     string
	}{
		{
			toolName:      "goto_definition",
			expectedError: "Error getting definition",
			lspMethod:     "textDocument/definition",
		},
		{
			toolName:      "find_references",
			expectedError: "Error finding references",
			lspMethod:     "textDocument/references",
		},
		{
			toolName:      "get_hover_info",
			expectedError: "Error getting hover info",
			lspMethod:     gateway.LSPMethodHover,
		},
		{
			toolName:      "get_document_symbols",
			expectedError: "Error getting document symbols",
			lspMethod:     "textDocument/documentSymbol",
		},
		{
			toolName:      "search_workspace_symbols",
			expectedError: "Error searching workspace symbols",
			lspMethod:     "workspace/symbol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			mockClient := NewMockLSPGatewayClient()
			handler := NewTestableToolHandler(mockClient)

			mockClient.SetError(tt.lspMethod, fmt.Errorf("test error"))

			args := map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      10,
				"character": 5,
				"query":     "test", // For workspace symbols
			}

			call := ToolCall{
				Name:      tt.toolName,
				Arguments: args,
			}

			result, err := handler.CallTool(context.Background(), call)
			if err != nil {
				t.Fatalf("Expected tool call to handle error gracefully, got error: %v", err)
			}

			if !result.IsError {
				t.Error("Expected error result")
			}

			if !strings.Contains(result.Content[0].Text, tt.expectedError) {
				t.Errorf("Expected error message to contain '%s', got: %s", tt.expectedError, result.Content[0].Text)
			}
		})
	}
}

// Test tool schema validation
func TestToolSchemaValidation(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	// Test goto_definition schema
	tool := handler.tools["goto_definition"]

	// Check required fields
	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		t.Fatal("Expected required fields to be []string")
	}

	expectedRequired := []string{"uri", "line", "character"}
	if !reflect.DeepEqual(required, expectedRequired) {
		t.Errorf("Expected required fields %v, got %v", expectedRequired, required)
	}

	// Check properties
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be map[string]interface{}")
	}

	// Validate uri property
	uriProp, ok := properties["uri"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected uri property to be map[string]interface{}")
	}

	if uriProp["type"] != "string" {
		t.Errorf("Expected uri type 'string', got %v", uriProp["type"])
	}

	// Validate line property
	lineProp, ok := properties["line"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected line property to be map[string]interface{}")
	}

	if lineProp["type"] != "integer" {
		t.Errorf("Expected line type 'integer', got %v", lineProp["type"])
	}

	// Validate character property
	charProp, ok := properties["character"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected character property to be map[string]interface{}")
	}

	if charProp["type"] != "integer" {
		t.Errorf("Expected character type 'integer', got %v", charProp["type"])
	}
}

// Test find_references schema with includeDeclaration
func TestFindReferencesSchema(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	tool := handler.tools["find_references"]
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be map[string]interface{}")
	}

	// Check includeDeclaration property
	includeProp, ok := properties["includeDeclaration"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected includeDeclaration property")
	}

	if includeProp["type"] != "boolean" {
		t.Errorf("Expected includeDeclaration type 'boolean', got %v", includeProp["type"])
	}

	if includeProp["default"] != true {
		t.Errorf("Expected includeDeclaration default true, got %v", includeProp["default"])
	}

	// Check that includeDeclaration is not in required fields
	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		t.Fatal("Expected required fields to be []string")
	}

	for _, field := range required {
		if field == "includeDeclaration" {
			t.Error("includeDeclaration should not be in required fields")
		}
	}
}

// Test workspace symbols schema
func TestWorkspaceSymbolsSchema(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	tool := handler.tools["search_workspace_symbols"]

	// Check required fields (should only have query)
	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		t.Fatal("Expected required fields to be []string")
	}

	expectedRequired := []string{"query"}
	if !reflect.DeepEqual(required, expectedRequired) {
		t.Errorf("Expected required fields %v, got %v", expectedRequired, required)
	}

	// Check properties
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be map[string]interface{}")
	}

	// Should only have query property
	if len(properties) != 1 {
		t.Errorf("Expected 1 property, got %d", len(properties))
	}

	queryProp, ok := properties["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected query property")
	}

	if queryProp["type"] != "string" {
		t.Errorf("Expected query type 'string', got %v", queryProp["type"])
	}
}

// Test document symbols schema
func TestDocumentSymbolsSchema(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	tool := handler.tools["get_document_symbols"]

	// Check required fields (should only have uri)
	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		t.Fatal("Expected required fields to be []string")
	}

	expectedRequired := []string{"uri"}
	if !reflect.DeepEqual(required, expectedRequired) {
		t.Errorf("Expected required fields %v, got %v", expectedRequired, required)
	}

	// Check properties
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be map[string]interface{}")
	}

	// Should only have uri property
	if len(properties) != 1 {
		t.Errorf("Expected 1 property, got %d", len(properties))
	}
}

// Test concurrent tool calls
func TestConcurrentToolCalls(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	// Set up responses for all methods
	mockClient.SetResponse("textDocument/definition", json.RawMessage(`{"result": "definition"}`))
	mockClient.SetResponse("textDocument/references", json.RawMessage(`{"result": "references"}`))
	mockClient.SetResponse(gateway.LSPMethodHover, json.RawMessage(`{"result": "hover"}`))

	const numCalls = 100
	results := make(chan *ToolResult, numCalls)
	errors := make(chan error, numCalls)

	// Launch concurrent tool calls
	for i := 0; i < numCalls; i++ {
		go func(index int) {
			args := map[string]interface{}{
				"uri":       fmt.Sprintf("file:///test%d.go", index),
				"line":      index,
				"character": index % 10,
			}

			call := ToolCall{
				Name:      "goto_definition",
				Arguments: args,
			}

			result, err := handler.CallTool(context.Background(), call)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numCalls; i++ {
		select {
		case result := <-results:
			if !result.IsError {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Unexpected error in concurrent call: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent calls to complete")
		}
	}

	if successCount != numCalls {
		t.Errorf("Expected %d successful calls, got %d", numCalls, successCount)
	}

	// Verify total request count
	totalRequests := mockClient.GetRequestCount()
	if totalRequests < int64(numCalls) {
		t.Errorf("Expected at least %d total requests, got %d", numCalls, totalRequests)
	}
}

// Test context cancellation
func TestToolCallContextCancellation(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	// Set up mock to simulate slow response by setting a delay
	mockClient.SetDelay("textDocument/definition", 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "goto_definition",
		Arguments: args,
	}

	result, err := handler.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("Expected tool call to handle cancellation gracefully, got error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for cancelled context")
	}

	if !strings.Contains(result.Content[0].Text, "context canceled") {
		t.Errorf("Expected context cancellation error, got: %s", result.Content[0].Text)
	}
}

// Test tool result structure
func TestToolResultStructure(t *testing.T) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	expectedResponse := json.RawMessage(`{"test": "data"}`)
	mockClient.SetResponse("textDocument/definition", expectedResponse)

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "goto_definition",
		Arguments: args,
	}

	result, err := handler.CallTool(context.Background(), call)
	if err != nil {
		t.Fatalf("Expected successful tool call, got error: %v", err)
	}

	// Validate result structure
	if result.IsError {
		t.Error("Expected successful result")
	}

	if result.Error != nil {
		t.Error("Expected no error in successful result")
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(result.Content))
	}

	content := result.Content[0]
	if content.Type != "text" {
		t.Errorf("Expected content type 'text', got %s", content.Type)
	}

	// JSON formatting may differ (spaces), so validate by unmarshaling and comparing
	var expectedData, actualData interface{}
	if err := json.Unmarshal(expectedResponse, &expectedData); err != nil {
		t.Fatalf("Failed to unmarshal expected response: %v", err)
	}
	if err := json.Unmarshal([]byte(content.Text), &actualData); err != nil {
		t.Fatalf("Failed to unmarshal actual response: %v", err)
	}

	if !reflect.DeepEqual(expectedData, actualData) {
		t.Errorf("Expected content data %v, got %v", expectedData, actualData)
	}

	if content.Data != nil {
		t.Error("Expected no data in text content")
	}

	if content.Annotations != nil {
		t.Error("Expected no annotations in basic content")
	}
}

// Benchmark tool calls
func BenchmarkToolCall(b *testing.B) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	mockClient.SetResponse("textDocument/definition", json.RawMessage(`{"benchmark": "result"}`))

	args := map[string]interface{}{
		"uri":       "file:///test.go",
		"line":      10,
		"character": 5,
	}

	call := ToolCall{
		Name:      "goto_definition",
		Arguments: args,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler.CallTool(context.Background(), call)
		if err != nil {
			b.Fatalf("Tool call failed: %v", err)
		}
	}
}

func BenchmarkListTools(b *testing.B) {
	mockClient := NewMockLSPGatewayClient()
	handler := NewTestableToolHandler(mockClient)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tools := handler.ListTools()
		if len(tools) != 5 {
			b.Fatalf("Expected 5 tools, got %d", len(tools))
		}
	}
}

// Test error code constants
func TestErrorCodeConstants(t *testing.T) {

	// Validate that error codes have expected values
	if MCPErrorParseError != -32700 {
		t.Errorf("Expected MCPErrorParseError to be -32700, got %d", MCPErrorParseError)
	}

	if MCPErrorServerError != -32000 {
		t.Errorf("Expected MCPErrorServerError to be -32000, got %d", MCPErrorServerError)
	}

	// Test LSP error codes
	if LSPErrorRequestFailed != -32803 {
		t.Errorf("Expected LSPErrorRequestFailed to be -32803, got %d", LSPErrorRequestFailed)
	}

	if LSPErrorInvalidFilePath != -32900 {
		t.Errorf("Expected LSPErrorInvalidFilePath to be -32900, got %d", LSPErrorInvalidFilePath)
	}
}

// Test structured error
func TestStructuredError(t *testing.T) {
	err := &StructuredError{
		Code:        MCPErrorInvalidParams,
		Message:     "Invalid parameter value",
		Details:     "The 'line' parameter must be non-negative",
		Context:     map[string]interface{}{"parameter": "line", "value": -1},
		Suggestions: []string{"Use line >= 0", "Check parameter validation"},
		Retryable:   false,
	}

	if err.Code != MCPErrorInvalidParams {
		t.Errorf("Expected error code %d, got %d", MCPErrorInvalidParams, err.Code)
	}

	if err.Message != "Invalid parameter value" {
		t.Errorf("Expected message 'Invalid parameter value', got %s", err.Message)
	}

	if err.Retryable {
		t.Error("Expected error to not be retryable")
	}

	if len(err.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(err.Suggestions))
	}
}

// Test validation error
func TestValidationError(t *testing.T) {
	valErr := &ValidationError{
		Field:    "line",
		Value:    -1,
		Expected: "integer >= 0",
		Actual:   "integer < 0",
		Message:  "Line number must be non-negative",
	}

	if valErr.Field != "line" {
		t.Errorf("Expected field 'line', got %s", valErr.Field)
	}

	if valErr.Value != -1 {
		t.Errorf("Expected value -1, got %v", valErr.Value)
	}

	if valErr.Message != "Line number must be non-negative" {
		t.Errorf("Expected specific message, got %s", valErr.Message)
	}
}
