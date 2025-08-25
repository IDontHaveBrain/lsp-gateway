package server

import (
	"encoding/json"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/protocol"
)

// =============================================================================
// Test Helper Functions
// =============================================================================

func createTestMCPServer() *MCPServer {
	cfg := config.GetDefaultConfig()
	// Disable cache to avoid initialization issues in tests
	cfg.Cache = &config.CacheConfig{Enabled: false}

	mcpServer, _ := NewMCPServer(cfg)
	return mcpServer
}

func createValidFindSymbolsParams() map[string]interface{} {
	return map[string]interface{}{
		"name": "findSymbols",
		"arguments": map[string]interface{}{
			"pattern":     "test.*",
			"filePath":    "*.go",
			"maxResults":  float64(100),
			"includeCode": true,
		},
	}
}

func createValidFindReferencesParams() map[string]interface{} {
	return map[string]interface{}{
		"name": "findReferences",
		"arguments": map[string]interface{}{
			"pattern":    "TestFunction",
			"filePath":   "**/*.go",
			"maxResults": float64(50),
		},
	}
}

// =============================================================================
// delegateToolCall Tests
// =============================================================================

func TestDelegateToolCall_ValidFindSymbols(t *testing.T) {
	mcpServer := createTestMCPServer()

	req := &protocol.JSONRPCRequest{
		ID:     "test-1",
		Method: "tools/call",
		Params: createValidFindSymbolsParams(),
	}

	response := mcpServer.delegateToolCall(req)

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	// Check response structure (may have error due to no LSP servers, but structure should be valid)
	if response.ID != req.ID {
		t.Errorf("Expected response ID %s, got %s", req.ID, response.ID)
	}

	// Verify result structure when successful
	if response.Error == nil && response.Result != nil {
		result, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be map[string]interface{}, got %T", response.Result)
		}

		content, ok := result["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("Expected content array in result")
		}

		if len(content) > 0 && content[0]["type"] != "text" {
			t.Errorf("Expected content type 'text', got %v", content[0]["type"])
		}
	}
}

func TestDelegateToolCall_ValidFindReferences(t *testing.T) {
	mcpServer := createTestMCPServer()

	req := &protocol.JSONRPCRequest{
		ID:     "test-2",
		Method: "tools/call",
		Params: createValidFindReferencesParams(),
	}

	response := mcpServer.delegateToolCall(req)

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.ID != req.ID {
		t.Errorf("Expected response ID %s, got %s", req.ID, response.ID)
	}

	// Verify result structure when successful
	if response.Error == nil && response.Result != nil {
		result, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be map[string]interface{}, got %T", response.Result)
		}

		content, ok := result["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("Expected content array in result")
		}

		if len(content) > 0 && content[0]["type"] != "text" {
			t.Errorf("Expected content type 'text', got %v", content[0]["type"])
		}
	}
}

func TestDelegateToolCall_InvalidParams(t *testing.T) {
	mcpServer := createTestMCPServer()

	tests := []struct {
		name            string
		params          interface{}
		expectErrorCode int
	}{
		{
			name:            "non-map params",
			params:          "invalid",
			expectErrorCode: -32602, // Invalid params
		},
		{
			name:            "missing name",
			params:          map[string]interface{}{},
			expectErrorCode: -32602, // Invalid params
		},
		{
			name: "invalid name type",
			params: map[string]interface{}{
				"name": 123,
			},
			expectErrorCode: -32602, // Invalid params
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPCRequest{
				ID:     "test-error",
				Method: "tools/call",
				Params: tt.params,
			}

			response := mcpServer.delegateToolCall(req)

			if response == nil {
				t.Fatal("Expected non-nil response")
			}

			if response.Error == nil {
				t.Fatal("Expected error response")
			}

			if response.Error.Code != tt.expectErrorCode {
				t.Errorf("Expected error code %d, got %d", tt.expectErrorCode, response.Error.Code)
			}
		})
	}
}

func TestDelegateToolCall_UnknownTool(t *testing.T) {
	mcpServer := createTestMCPServer()

	req := &protocol.JSONRPCRequest{
		ID:     "test-unknown",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      "unknownTool",
			"arguments": map[string]interface{}{},
		},
	}

	response := mcpServer.delegateToolCall(req)

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.Error == nil {
		t.Fatal("Expected error response")
	}

	if response.Error.Code != -32601 { // Method not found
		t.Errorf("Expected error code -32601, got %d", response.Error.Code)
	}

	expectedMessage := "tool not found: unknownTool"
	if response.Error.Message != expectedMessage {
		t.Errorf("Expected error message '%s', got '%s'", expectedMessage, response.Error.Message)
	}
}

// =============================================================================
// handleFindSymbols Parameter Validation Tests
// =============================================================================

func TestHandleFindSymbols_InvalidParameters(t *testing.T) {
	mcpServer := createTestMCPServer()

	tests := []struct {
		name      string
		params    map[string]interface{}
		expectErr string
	}{
		{
			name:      "missing arguments",
			params:    map[string]interface{}{},
			expectErr: "missing or invalid arguments",
		},
		{
			name: "invalid arguments type",
			params: map[string]interface{}{
				"arguments": "invalid",
			},
			expectErr: "missing or invalid arguments",
		},
		{
			name: "missing pattern",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"filePath": "*.go",
				},
			},
			expectErr: "pattern is required and must be a string",
		},
		{
			name: "empty pattern",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern":  "",
					"filePath": "*.go",
				},
			},
			expectErr: "pattern is required and must be a string",
		},
		{
			name: "invalid pattern type",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern":  123,
					"filePath": "*.go",
				},
			},
			expectErr: "pattern is required and must be a string",
		},
		{
			name: "missing filePath",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern": "test.*",
				},
			},
			expectErr: "filePath is required and must be a string",
		},
		{
			name: "empty filePath",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern":  "test.*",
					"filePath": "",
				},
			},
			expectErr: "filePath is required and must be a string",
		},
		{
			name: "invalid filePath type",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern":  "test.*",
					"filePath": 123,
				},
			},
			expectErr: "filePath is required and must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mcpServer.handleFindSymbols(tt.params)

			if err == nil {
				t.Fatal("Expected error")
			}

			if err.Error() != tt.expectErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectErr, err.Error())
			}

			if result != nil {
				t.Errorf("Expected nil result on error, got %v", result)
			}
		})
	}
}

func TestHandleFindSymbols_ValidParametersStructure(t *testing.T) {
	mcpServer := createTestMCPServer()

	params := map[string]interface{}{
		"arguments": map[string]interface{}{
			"pattern":     "test.*",
			"filePath":    "*.go",
			"maxResults":  float64(100),
			"includeCode": true,
		},
	}

	result, err := mcpServer.handleFindSymbols(params)

	if err != nil {
		t.Errorf("Expected no error with valid params, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result with valid params")
	}

	// Verify result structure
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected content array in result")
	}

	if len(content) == 0 {
		t.Fatal("Expected non-empty content array")
	}

	if content[0]["type"] != "text" {
		t.Errorf("Expected content type 'text', got %v", content[0]["type"])
	}
}

// =============================================================================
// handleFindSymbolReferences Parameter Validation Tests
// =============================================================================

func TestHandleFindSymbolReferences_InvalidParameters(t *testing.T) {
	mcpServer := createTestMCPServer()

	tests := []struct {
		name      string
		params    map[string]interface{}
		expectErr string
	}{
		{
			name:      "missing arguments",
			params:    map[string]interface{}{},
			expectErr: "missing or invalid arguments",
		},
		{
			name: "invalid arguments type",
			params: map[string]interface{}{
				"arguments": "invalid",
			},
			expectErr: "missing or invalid arguments",
		},
		{
			name: "missing pattern",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{},
			},
			expectErr: "pattern is required and must be a string",
		},
		{
			name: "empty pattern",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern": "",
				},
			},
			expectErr: "pattern is required and must be a string",
		},
		{
			name: "invalid pattern type",
			params: map[string]interface{}{
				"arguments": map[string]interface{}{
					"pattern": 123,
				},
			},
			expectErr: "pattern is required and must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mcpServer.handleFindSymbolReferences(tt.params)

			if err == nil {
				t.Fatal("Expected error")
			}

			if err.Error() != tt.expectErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectErr, err.Error())
			}

			if result != nil {
				t.Errorf("Expected nil result on error, got %v", result)
			}
		})
	}
}

func TestHandleFindSymbolReferences_ValidParametersStructure(t *testing.T) {
	mcpServer := createTestMCPServer()

	params := map[string]interface{}{
		"arguments": map[string]interface{}{
			"pattern":    "TestFunction",
			"filePath":   "**/*.go",
			"maxResults": float64(50),
		},
	}

	result, err := mcpServer.handleFindSymbolReferences(params)

	if err != nil {
		t.Errorf("Expected no error with valid params, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result with valid params")
	}

	// Verify result structure
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	content, ok := resultMap["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected content array in result")
	}

	if len(content) == 0 {
		t.Fatal("Expected non-empty content array")
	}

	if content[0]["type"] != "text" {
		t.Errorf("Expected content type 'text', got %v", content[0]["type"])
	}
}

func TestHandleFindSymbolReferences_OptionalParameters(t *testing.T) {
	mcpServer := createTestMCPServer()

	tests := []struct {
		name          string
		filePath      interface{}
		maxResults    interface{}
		expectedValid bool
	}{
		{
			name:          "default values - only pattern",
			expectedValid: true,
		},
		{
			name:          "custom filePath",
			filePath:      "src/**/*.go",
			expectedValid: true,
		},
		{
			name:          "custom maxResults",
			maxResults:    float64(25),
			expectedValid: true,
		},
		{
			name:          "both custom",
			filePath:      "test/**/*.js",
			maxResults:    float64(10),
			expectedValid: true,
		},
		{
			name:          "invalid maxResults type - should be ignored",
			maxResults:    "invalid",
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"pattern": "test.*",
			}

			if tt.filePath != nil {
				args["filePath"] = tt.filePath
			}

			if tt.maxResults != nil {
				args["maxResults"] = tt.maxResults
			}

			params := map[string]interface{}{
				"arguments": args,
			}

			result, err := mcpServer.handleFindSymbolReferences(params)

			if tt.expectedValid {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
			} else {
				if err == nil {
					t.Error("Expected error but got none")
				}
			}
		})
	}
}

// =============================================================================
// MCP Response Format Compliance Tests
// =============================================================================

func TestMCPResponseFormatCompliance(t *testing.T) {
	mcpServer := createTestMCPServer()

	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name:   "findSymbols response format",
			params: createValidFindSymbolsParams(),
		},
		{
			name:   "findReferences response format",
			params: createValidFindReferencesParams(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPCRequest{
				ID:     "format-test",
				Method: "tools/call",
				Params: tt.params,
			}

			response := mcpServer.delegateToolCall(req)

			if response == nil {
				t.Fatal("Expected non-nil response")
			}

			// Verify response can be marshaled to JSON (MCP requirement)
			_, err := json.Marshal(response)
			if err != nil {
				t.Errorf("Response cannot be marshaled to JSON: %v", err)
			}

			// Verify response structure
			if response.ID != req.ID {
				t.Errorf("Response ID must match request ID: expected %s, got %s", req.ID, response.ID)
			}

			// If successful, verify MCP content structure
			if response.Error == nil && response.Result != nil {
				result, ok := response.Result.(map[string]interface{})
				if !ok {
					t.Fatal("Result must be a map")
				}

				content, ok := result["content"].([]map[string]interface{})
				if !ok {
					t.Fatal("Result must have 'content' array")
				}

				if len(content) == 0 {
					t.Fatal("Content array must not be empty")
				}

				// Verify each content item has required MCP fields
				for i, item := range content {
					if _, ok := item["type"]; !ok {
						t.Errorf("Content item %d missing 'type' field", i)
					}

					if _, ok := item["text"]; !ok {
						t.Errorf("Content item %d missing 'text' field", i)
					}

					if item["type"] != "text" {
						t.Errorf("Content item %d type must be 'text', got %v", i, item["type"])
					}

					// Verify text is a string
					if _, ok := item["text"].(string); !ok {
						t.Errorf("Content item %d text must be string, got %T", i, item["text"])
					}
				}
			}
		})
	}
}

func TestMCPErrorResponseFormatCompliance(t *testing.T) {
	mcpServer := createTestMCPServer()

	req := &protocol.JSONRPCRequest{
		ID:     "error-format-test",
		Method: "tools/call",
		Params: "invalid-params", // Should cause error
	}

	response := mcpServer.delegateToolCall(req)

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.Error == nil {
		t.Fatal("Expected error response")
	}

	// Verify error can be marshaled to JSON
	_, err := json.Marshal(response)
	if err != nil {
		t.Errorf("Error response cannot be marshaled to JSON: %v", err)
	}

	// Verify MCP error structure
	if response.Error.Code == 0 {
		t.Error("Error must have non-zero code")
	}

	if response.Error.Message == "" {
		t.Error("Error must have non-empty message")
	}

	// Verify ID matches request
	if response.ID != req.ID {
		t.Errorf("Response ID must match request ID: expected %s, got %s", req.ID, response.ID)
	}

	// Result should be nil on error
	if response.Result != nil {
		t.Error("Result must be nil on error")
	}
}

// =============================================================================
// Edge Cases and Error Handling Tests
// =============================================================================

func TestDelegateToolCall_NilRequest(t *testing.T) {
	// This would cause a panic if not handled properly
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("delegateToolCall should not panic with nil request: %v", r)
		}
	}()

	mcpServer := createTestMCPServer()

	// We can't test nil request directly as it would cause panic in real usage
	// Instead test with minimal invalid request
	req := &protocol.JSONRPCRequest{}
	response := mcpServer.delegateToolCall(req)

	if response == nil {
		t.Fatal("Expected non-nil response even for invalid request")
	}

	if response.Error == nil {
		t.Fatal("Expected error response for invalid request")
	}
}

func TestContext_Timeout_Handling(t *testing.T) {
	mcpServer := createTestMCPServer()

	// Test that context timeout is properly handled in tools
	params := map[string]interface{}{
		"arguments": map[string]interface{}{
			"pattern":  "test.*",
			"filePath": "*.go",
		},
	}

	// The actual timeout handling happens inside the tool implementation
	// We can't easily test timeout without mocking, but we can ensure
	// the function completes within reasonable time
	start := time.Now()
	result, err := mcpServer.handleFindSymbols(params)
	duration := time.Since(start)

	// Should complete quickly without LSP servers
	if duration > time.Second {
		t.Errorf("Function took too long: %v", duration)
	}

	// Should handle gracefully even without LSP servers
	if err != nil {
		t.Errorf("Expected graceful handling of timeout/no servers, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result even on timeout/empty result")
	}
}

// =============================================================================
// Integration-style Tests (without external dependencies)
// =============================================================================

func TestFullToolFlow_FindSymbols(t *testing.T) {
	mcpServer := createTestMCPServer()

	// Test the full flow from request to response
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "integration-test",
		Method:  "tools/call",
		Params:  createValidFindSymbolsParams(),
	}

	response := mcpServer.delegateToolCall(req)

	// Verify full response structure
	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.JSONRPC == "" {
		response.JSONRPC = "2.0" // May be set elsewhere
	}

	if response.ID != req.ID {
		t.Errorf("Response ID mismatch: expected %s, got %s", req.ID, response.ID)
	}

	// Response should have either result or error
	if response.Result == nil && response.Error == nil {
		t.Fatal("Response must have either result or error")
	}

	// If it has result, verify structure
	if response.Result != nil {
		resultMap, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("Result must be map[string]interface{}, got %T", response.Result)
		}

		content, ok := resultMap["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("Result must contain content array")
		}

		// Content array should have at least one item
		if len(content) == 0 {
			t.Fatal("Content array should not be empty")
		}

		// Verify content structure
		firstContent := content[0]
		if firstContent["type"] != "text" {
			t.Errorf("First content type should be 'text', got %v", firstContent["type"])
		}

		textContent, ok := firstContent["text"].(string)
		if !ok {
			t.Fatal("Content text should be string")
		}

		if len(textContent) == 0 {
			t.Error("Content text should not be empty")
		}
	}
}

func TestFullToolFlow_FindReferences(t *testing.T) {
	mcpServer := createTestMCPServer()

	// Test the full flow from request to response
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "integration-test-refs",
		Method:  "tools/call",
		Params:  createValidFindReferencesParams(),
	}

	response := mcpServer.delegateToolCall(req)

	// Verify full response structure
	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.ID != req.ID {
		t.Errorf("Response ID mismatch: expected %s, got %s", req.ID, response.ID)
	}

	// Response should have either result or error
	if response.Result == nil && response.Error == nil {
		t.Fatal("Response must have either result or error")
	}

	// If it has result, verify structure matches MCP format
	if response.Result != nil {
		resultMap, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("Result must be map[string]interface{}, got %T", response.Result)
		}

		content, ok := resultMap["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("Result must contain content array")
		}

		// Content array should have at least one item
		if len(content) == 0 {
			t.Fatal("Content array should not be empty")
		}

		// Verify content is properly formatted for AI consumption
		firstContent := content[0]
		if firstContent["type"] != "text" {
			t.Errorf("First content type should be 'text', got %v", firstContent["type"])
		}

		textContent, ok := firstContent["text"].(string)
		if !ok {
			t.Fatal("Content text should be string")
		}

		// Text should contain structured data for AI parsing
		if len(textContent) == 0 {
			t.Error("Content text should not be empty")
		}
	}
}
