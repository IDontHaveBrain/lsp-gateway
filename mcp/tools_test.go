package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLSPClient implements LSPClient interface for testing
type mockLSPClient struct {
	requests   []mockRequest
	responses  map[string]json.RawMessage
	errors     map[string]error
	callCounts map[string]int
}

type mockRequest struct {
	Method string
	Params interface{}
}

func newMockLSPClient() *mockLSPClient {
	return &mockLSPClient{
		requests:   make([]mockRequest, 0),
		responses:  make(map[string]json.RawMessage),
		errors:     make(map[string]error),
		callCounts: make(map[string]int),
	}
}

func (m *mockLSPClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.requests = append(m.requests, mockRequest{Method: method, Params: params})
	m.callCounts[method]++

	// Return queued error if any
	if err, ok := m.errors[method]; ok {
		delete(m.errors, method) // consume error
		return nil, err
	}

	// Return custom response if set
	if response, ok := m.responses[method]; ok {
		return response, nil
	}

	// Default responses
	switch method {
	case "textDocument/definition":
		return json.RawMessage(`[{"uri":"file://test.go","range":{"start":{"line":10,"character":0}}}]`), nil
	case "textDocument/hover":
		return json.RawMessage(`{"contents":"test hover content"}`), nil
	case "textDocument/references":
		return json.RawMessage(`[{"uri":"file://test.go","range":{"start":{"line":5,"character":0}}}]`), nil
	case "textDocument/documentSymbol":
		return json.RawMessage(`[{"name":"TestClass","kind":5}]`), nil
	case "workspace/symbol":
		return json.RawMessage(`[{"name":"WorkspaceSymbol","kind":5}]`), nil
	default:
		return json.RawMessage(`{}`), nil
	}
}

func (m *mockLSPClient) setError(method string, err error) {
	m.errors[method] = err
}

func (m *mockLSPClient) setResponse(method string, response json.RawMessage) {
	m.responses[method] = response
}

func (m *mockLSPClient) getCallCount(method string) int {
	return m.callCounts[method]
}

func (m *mockLSPClient) getRequests() []mockRequest {
	return m.requests
}

func (m *mockLSPClient) reset() {
	m.requests = m.requests[:0]
	m.responses = make(map[string]json.RawMessage)
	m.errors = make(map[string]error)
	m.callCounts = make(map[string]int)
}

func TestNewToolHandler(t *testing.T) {
	mockClient := newMockLSPClient()
	handler := NewToolHandler(mockClient)

	assert.NotNil(t, handler, "Handler should not be nil")
	assert.Equal(t, mockClient, handler.Client, "Client should be set")
	assert.NotNil(t, handler.Tools, "Tools map should be initialized")
	assert.Len(t, handler.Tools, 5, "Should register 5 default tools")
}

func TestToolHandler_RegisterDefaultTools(t *testing.T) {
	handler := &ToolHandler{
		Client: newMockLSPClient(),
		Tools:  make(map[string]Tool),
	}

	handler.RegisterDefaultTools()

	expectedTools := []string{
		"goto_definition",
		"find_references",
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	assert.Len(t, handler.Tools, 5, "Should register 5 tools")

	for _, toolName := range expectedTools {
		tool, exists := handler.Tools[toolName]
		assert.True(t, exists, "Tool %s should be registered", toolName)
		assert.Equal(t, toolName, tool.Name, "Tool name should match")
		assert.NotEmpty(t, tool.Description, "Tool should have description")
		assert.NotNil(t, tool.InputSchema, "Tool should have input schema")

		schema := tool.InputSchema
		assert.Equal(t, "object", schema["type"], "Schema should be object type")
		assert.NotNil(t, schema["properties"], "Schema should have properties")
		assert.NotNil(t, schema["required"], "Schema should have required fields")
	}
}

func TestToolHandler_SchemaValidation(t *testing.T) {
	handler := NewToolHandler(newMockLSPClient())

	tests := []struct {
		toolName       string
		requiredFields []string
	}{
		{"goto_definition", []string{"uri", "line", "character"}},
		{"find_references", []string{"uri", "line", "character"}},
		{"get_hover_info", []string{"uri", "line", "character"}},
		{"get_document_symbols", []string{"uri"}},
		{"search_workspace_symbols", []string{"query"}},
	}

	for _, test := range tests {
		t.Run(test.toolName, func(t *testing.T) {
			tool := handler.Tools[test.toolName]
			schema := tool.InputSchema
			required := schema["required"].([]string)
			assert.ElementsMatch(t, test.requiredFields, required)
		})
	}
}

func TestToolHandler_ListTools(t *testing.T) {
	handler := NewToolHandler(newMockLSPClient())
	tools := handler.ListTools()

	assert.Len(t, tools, 5, "Should return 5 tools")

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedNames := []string{
		"goto_definition", "find_references", "get_hover_info",
		"get_document_symbols", "search_workspace_symbols",
	}

	for _, name := range expectedNames {
		assert.True(t, toolNames[name], "Tool %s should be in list", name)
	}
}

func TestToolHandler_CallTool_UnknownTool(t *testing.T) {
	handler := NewToolHandler(newMockLSPClient())

	result, err := handler.CallTool(context.Background(), ToolCall{
		Name:      "unknown_tool",
		Arguments: map[string]interface{}{},
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Unknown tool: unknown_tool")
}

func TestToolHandler_CallTool_ValidTools(t *testing.T) {
	mockClient := newMockLSPClient()
	handler := &ToolHandler{
		Client: mockClient,
		Tools:  make(map[string]Tool),
	}
	handler.RegisterDefaultTools()

	tests := []struct {
		toolName     string
		arguments    map[string]interface{}
		expectedCall string
	}{
		{
			toolName: "goto_definition",
			arguments: map[string]interface{}{
				"uri": "file:///test.go", "line": 10, "character": 5,
			},
			expectedCall: "textDocument/definition",
		},
		{
			toolName: "get_hover_info",
			arguments: map[string]interface{}{
				"uri": "file:///test.go", "line": 10, "character": 5,
			},
			expectedCall: LSPMethodHover,
		},
		{
			toolName: "get_document_symbols",
			arguments: map[string]interface{}{
				"uri": "file:///test.go",
			},
			expectedCall: "textDocument/documentSymbol",
		},
	}

	for _, test := range tests {
		t.Run(test.toolName, func(t *testing.T) {
			mockClient.reset()

			result, err := handler.CallTool(context.Background(), ToolCall{
				Name:      test.toolName,
				Arguments: test.arguments,
			})

			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Equal(t, 1, mockClient.getCallCount(test.expectedCall))
			assert.NotNil(t, result.Meta)
			assert.Equal(t, test.expectedCall, result.Meta.LSPMethod)
		})
	}
}

func TestToolHandler_HandleGotoDefinition_Parameters(t *testing.T) {
	mockClient := newMockLSPClient()
	handler := &ToolHandler{Client: mockClient, Tools: make(map[string]Tool)}

	args := map[string]interface{}{
		"uri": "file:///test.go", "line": 10, "character": 5,
	}

	result, err := handler.handleGotoDefinition(context.Background(), args)

	require.NoError(t, err)
	assert.False(t, result.IsError)

	requests := mockClient.getRequests()
	require.Len(t, requests, 1)

	params := requests[0].Params.(map[string]interface{})
	textDoc := params["textDocument"].(map[string]interface{})
	position := params["position"].(map[string]interface{})

	assert.Equal(t, "file:///test.go", textDoc["uri"])
	assert.Equal(t, 10, position["line"])
	assert.Equal(t, 5, position["character"])
}

func TestToolHandler_HandleFindReferences_IncludeDeclaration(t *testing.T) {
	mockClient := newMockLSPClient()
	handler := &ToolHandler{Client: mockClient, Tools: make(map[string]Tool)}

	t.Run("default include declaration", func(t *testing.T) {
		mockClient.reset()
		args := map[string]interface{}{
			"uri": "file:///test.go", "line": 10, "character": 5,
		}

		result, err := handler.handleFindReferences(context.Background(), args)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		requests := mockClient.getRequests()
		require.Len(t, requests, 1)
		params := requests[0].Params.(map[string]interface{})
		context := params["context"].(map[string]interface{})
		assert.True(t, context["includeDeclaration"].(bool))
	})

	t.Run("explicit include declaration false", func(t *testing.T) {
		mockClient.reset()
		args := map[string]interface{}{
			"uri": "file:///test.go", "line": 10, "character": 5,
			"includeDeclaration": false,
		}

		result, err := handler.handleFindReferences(context.Background(), args)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		requests := mockClient.getRequests()
		require.Len(t, requests, 1)
		params := requests[0].Params.(map[string]interface{})
		context := params["context"].(map[string]interface{})
		assert.False(t, context["includeDeclaration"].(bool))
	})
}

func TestToolHandler_LSPClientError(t *testing.T) {
	mockClient := newMockLSPClient()
	mockClient.setError("textDocument/definition", fmt.Errorf("LSP request failed"))

	handler := &ToolHandler{Client: mockClient, Tools: make(map[string]Tool)}

	args := map[string]interface{}{
		"uri": "file:///test.go", "line": 10, "character": 5,
	}

	result, err := handler.handleGotoDefinition(context.Background(), args)

	require.NoError(t, err, "Handler should not return error")
	assert.True(t, result.IsError, "Result should indicate error")
	assert.Contains(t, result.Content[0].Text, "Error getting definition")
}

func TestToolHandler_ResultMetadata(t *testing.T) {
	mockClient := newMockLSPClient()
	handler := &ToolHandler{Client: mockClient, Tools: make(map[string]Tool)}

	args := map[string]interface{}{
		"uri": "file:///test.go", "line": 10, "character": 5,
	}

	result, err := handler.handleGotoDefinition(context.Background(), args)

	require.NoError(t, err)
	require.NotNil(t, result.Meta)

	assert.Equal(t, "textDocument/definition", result.Meta.LSPMethod)
	assert.NotEmpty(t, result.Meta.Timestamp)
	assert.NotEmpty(t, result.Meta.Duration)
}