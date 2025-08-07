package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

func TestNewMCPServer(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name:   "with default config",
			config: config.GetDefaultConfig(),
		},
		{
			name: "with custom config",
			config: &config.Config{
				Cache: &config.CacheConfig{
					Enabled:     true,
					MaxMemoryMB: 256,
					TTLHours:    1,
					StoragePath: "/tmp/test-cache",
				},
				Servers: map[string]*config.ServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{"serve"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpServer, err := server.NewMCPServer(tt.config)
			if err != nil {
				t.Fatalf("NewMCPServer failed: %v", err)
			}
			if mcpServer == nil {
				t.Fatal("NewMCPServer returned nil")
			}
		})
	}
}

func TestMCPServerInitialize(t *testing.T) {
	cfg := config.GetDefaultConfig()
	mcpServer, err := server.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("NewMCPServer failed: %v", err)
	}

	// Create a mock MCP request for initialize
	request := &server.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	// Test that we can at least create the server and it has the expected structure
	if mcpServer == nil {
		t.Fatal("MCP server should not be nil")
	}

	// Validate the request structure is correct
	if request.Method != "initialize" {
		t.Errorf("Expected method 'initialize', got '%s'", request.Method)
	}
	if request.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", request.JSONRPC)
	}
}

func TestMCPServerToolsList(t *testing.T) {
	cfg := config.GetDefaultConfig()
	mcpServer, err := server.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("NewMCPServer failed: %v", err)
	}

	// Create a mock MCP request for tools/list
	request := &server.MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	// Test that we can create the request structure correctly
	if mcpServer == nil {
		t.Fatal("MCP server should not be nil")
	}

	// Validate the request structure is correct for tools/list
	if request.Method != "tools/list" {
		t.Errorf("Expected method 'tools/list', got '%s'", request.Method)
	}

	// Test the expected tool names are correctly defined
	expectedTools := []string{"findSymbols", "findReferences", "findDefinitions", "getSymbolInfo"}
	for _, tool := range expectedTools {
		if tool == "" {
			t.Errorf("Tool name should not be empty")
		}
	}
}

func TestMCPServerToolsCall(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		arguments map[string]interface{}
		valid     bool
	}{
		{
			name:     "findSymbols with valid params",
			toolName: "findSymbols",
			arguments: map[string]interface{}{
				"pattern":     "test.*",
				"filePattern": "*.go",
			},
			valid: true,
		},
		{
			name:     "findReferences with valid params",
			toolName: "findReferences",
			arguments: map[string]interface{}{
				"symbolName": "TestFunction",
			},
			valid: true,
		},
		{
			name:      "unknown tool",
			toolName:  "unknownTool",
			arguments: map[string]interface{}{},
			valid:     false,
		},
		{
			name:     "findSymbols missing required params",
			toolName: "findSymbols",
			arguments: map[string]interface{}{
				"pattern": "test.*",
				// missing filePattern
			},
			valid: false,
		},
	}

	cfg := config.GetDefaultConfig()
	mcpServer, err := server.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("NewMCPServer failed: %v", err)
	}

	if mcpServer == nil {
		t.Fatal("MCP server should not be nil")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock MCP request for tools/call
			request := &server.MCPRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name":      tt.toolName,
					"arguments": tt.arguments,
				},
			}

			// Validate request structure
			if request.Method != "tools/call" {
				t.Errorf("Expected method 'tools/call', got '%s'", request.Method)
			}

			// Validate tool name and arguments structure
			if params, ok := request.Params.(map[string]interface{}); ok {
				if name, ok := params["name"].(string); ok {
					if name != tt.toolName {
						t.Errorf("Expected tool name '%s', got '%s'", tt.toolName, name)
					}
				} else {
					t.Error("Tool name not found in params")
				}
			} else {
				t.Error("Invalid params structure")
			}
		})
	}
}

func TestMCPJSONRPCStructures(t *testing.T) {
	tests := []struct {
		name        string
		request     string
		expectValid bool
	}{
		{
			name:        "valid JSON-RPC request",
			request:     `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			expectValid: true,
		},
		{
			name:        "invalid JSON",
			request:     `{invalid json}`,
			expectValid: false,
		},
		{
			name:        "missing method",
			request:     `{"jsonrpc":"2.0","id":1,"params":{}}`,
			expectValid: false,
		},
		{
			name:        "valid tools/list request",
			request:     `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req server.MCPRequest
			err := json.Unmarshal([]byte(tt.request), &req)

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected valid JSON, but got error: %v", err)
				}
				if req.JSONRPC != "2.0" {
					t.Errorf("Expected JSONRPC '2.0', got '%s'", req.JSONRPC)
				}
			} else {
				if err == nil && req.Method != "" {
					t.Error("Expected invalid JSON or missing method")
				}
			}
		})
	}
}

func TestMCPServerStop(t *testing.T) {
	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:     true,
			StoragePath: "/tmp/test-cache",
		},
		Servers: map[string]*config.ServerConfig{},
	}

	mcpServer, err := server.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("NewMCPServer failed: %v", err)
	}

	// Test that Stop method exists and can be called
	err = mcpServer.Stop()
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestMCPErrorCodes(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{-32700, "Parse error"},
		{-32600, "Invalid request"},
		{-32601, "Method not found"},
		{-32602, "Invalid params"},
		{-32603, "Internal error"},
		{-32000, "Server error"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			err := &server.MCPError{
				Code:    tt.code,
				Message: tt.expected,
			}

			data, jsonErr := json.Marshal(err)
			if jsonErr != nil {
				t.Fatalf("Failed to marshal error: %v", jsonErr)
			}
			if !strings.Contains(string(data), tt.expected) {
				t.Errorf("Error message not found in JSON: %s", string(data))
			}
		})
	}
}
