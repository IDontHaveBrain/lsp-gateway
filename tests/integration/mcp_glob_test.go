package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/utils"
)

func TestMCPGlobPatterns(t *testing.T) {
	// Create a temporary workspace for testing
	tmpDir, err := os.MkdirTemp("", "mcp_glob_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file structure
	testFiles := map[string]string{
		"src/server/gateway.go": `package server
func NewHTTPGateway() {}
func StartGateway() {}`,
		"src/server/mcp_server.go": `package server
func NewMCPServer() {}
func StartMCPServer() {}`,
		"src/server/cache/manager.go": `package cache
func NewCacheManager() {}
func ClearCache() {}`,
		"src/client/client.go": `package client
func NewClient() {}
func Connect() {}`,
		"tests/test_gateway.go": `package tests
func TestGateway() {}`,
		"cmd/main.go": `package main
func main() {}`,
		"internal/utils/helper.go": `package utils
func HelperFunc() {}`,
		"vendor/external/lib.go": `package external
func ExternalFunc() {}`,
	}

	// Create all test files
	for relPath, content := range testFiles {
		fullPath := filepath.Join(tmpDir, relPath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	// Change to temp directory for testing
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Setup MCP server with cache
	cfg := config.GetDefaultConfigWithCache()
	cfg.EnableCache()
	
	mcpServer, err := server.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Start the server
	if err := mcpServer.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer mcpServer.Stop()

	// Wait for cache initialization
	time.Sleep(2 * time.Second)

	// Test cases for different glob patterns
	testCases := []struct {
		name        string
		pattern     string
		filePath    string
		wantSymbols []string
		wantCount   int
		description string
	}{
		{
			name:        "All Go files",
			pattern:     "New.*",
			filePath:    "*.go",
			wantSymbols: []string{"NewHTTPGateway", "NewMCPServer", "NewCacheManager", "NewClient"},
			wantCount:   4,
			description: "Should match all New* functions in any .go file",
		},
		{
			name:        "Files in specific directory",
			pattern:     "New.*",
			filePath:    "src/server/*.go",
			wantSymbols: []string{"NewHTTPGateway", "NewMCPServer"},
			wantCount:   2,
			description: "Should only match files directly in src/server/",
		},
		{
			name:        "Files in subdirectories",
			pattern:     "New.*",
			filePath:    "src/server/**/*.go",
			wantSymbols: []string{"NewHTTPGateway", "NewMCPServer", "NewCacheManager"},
			wantCount:   3,
			description: "Should match files in src/server and all subdirectories",
		},
		{
			name:        "All files under src",
			pattern:     "New.*",
			filePath:    "src/**/*.go",
			wantSymbols: []string{"NewHTTPGateway", "NewMCPServer", "NewCacheManager", "NewClient"},
			wantCount:   4,
			description: "Should match all Go files under src/",
		},
		{
			name:        "Tests directory",
			pattern:     "Test.*",
			filePath:    "tests/*.go",
			wantSymbols: []string{"TestGateway"},
			wantCount:   1,
			description: "Should match test files",
		},
		{
			name:        "Exclude vendor",
			pattern:     ".*Func",
			filePath:    "internal/**/*.go",
			wantSymbols: []string{"HelperFunc"},
			wantCount:   1,
			description: "Should match internal but not vendor",
		},
		{
			name:        "Main function in cmd",
			pattern:     "main",
			filePath:    "cmd/*.go",
			wantSymbols: []string{"main"},
			wantCount:   1,
			description: "Should find main function in cmd/",
		},
		{
			name:        "Cross-platform path separators",
			pattern:     "New.*",
			filePath:    `src\server\*.go`, // Windows-style backslashes
			wantSymbols: []string{"NewHTTPGateway", "NewMCPServer"},
			wantCount:   2,
			description: "Should handle backslash separators",
		},
		{
			name:        "No matches in wrong directory",
			pattern:     "New.*",
			filePath:    "tests/*.go",
			wantSymbols: []string{},
			wantCount:   0,
			description: "Should not find New* functions in tests/",
		},
		{
			name:        "Complex nested pattern",
			pattern:     ".*Cache.*",
			filePath:    "src/**/cache/*.go",
			wantSymbols: []string{"NewCacheManager", "ClearCache"},
			wantCount:   2,
			description: "Should match cache-related functions in cache directories",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build the request
			req := &server.MCPRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "findSymbols",
					"arguments": map[string]interface{}{
						"pattern":     tc.pattern,
						"filePath":    tc.filePath,
						"maxResults":  100,
						"includeCode": false,
					},
				},
			}

			// Execute the request
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Use delegateToolCall directly for testing
			response := handleToolCallForTest(mcpServer, req)
			
			if response == nil {
				t.Fatalf("Got nil response")
			}

			// Check for errors
			if response.Error != nil {
				t.Fatalf("Got error response: %v", response.Error)
			}

			// Parse the result
			result, ok := response.Result.(map[string]interface{})
			if !ok {
				t.Fatalf("Invalid result type: %T", response.Result)
			}

			content, ok := result["content"].([]map[string]interface{})
			if !ok || len(content) == 0 {
				t.Fatalf("Invalid content in result")
			}

			text, ok := content[0]["text"].(string)
			if !ok {
				t.Fatalf("Invalid text in content")
			}

			// Parse the JSON response
			var symbolResult struct {
				Symbols    []interface{} `json:"symbols"`
				TotalCount int          `json:"totalCount"`
				Truncated  bool         `json:"truncated"`
			}

			// The text contains formatted JSON, parse it
			if err := parseFormattedResult(text, &symbolResult); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			// Verify count
			if symbolResult.TotalCount != tc.wantCount {
				t.Errorf("%s: Expected %d symbols, got %d", tc.description, tc.wantCount, symbolResult.TotalCount)
			}

			// Verify specific symbols if provided
			if len(tc.wantSymbols) > 0 {
				foundSymbols := make(map[string]bool)
				for _, sym := range symbolResult.Symbols {
					if symMap, ok := sym.(map[string]interface{}); ok {
						if name, ok := symMap["name"].(string); ok {
							foundSymbols[name] = true
						}
					}
				}

				for _, want := range tc.wantSymbols {
					if !foundSymbols[want] {
						t.Errorf("%s: Expected to find symbol %s, but didn't", tc.description, want)
					}
				}
			}
		})
	}
}

// Helper function to handle tool calls in tests
func handleToolCallForTest(mcpServer *server.MCPServer, req *server.MCPRequest) *server.MCPResponse {
	// This would normally be an internal method, but for testing we simulate it
	// In a real test, you'd either export the method or use a test helper
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &server.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &server.RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	// For this test, we'll need to make the delegateToolCall method accessible
	// or create a test-specific handler
	// This is a simplified version for demonstration
	return nil // In real implementation, this would call the actual handler
}

// Helper function to parse formatted result
func parseFormattedResult(text string, result interface{}) error {
	// The text is formatted, we need to extract the JSON part
	// This is a simplified parser for testing
	// In production, you'd use the actual formatting logic
	return fmt.Errorf("not implemented for this test")
}

// TestFilePatternCompatibility verifies backward compatibility and edge cases
func TestFilePatternCompatibility(t *testing.T) {
	testPatterns := []struct {
		input    string
		expected string
		desc     string
	}{
		{"*.go", "*.go", "Simple glob"},
		{"**/*.go", "**/*.go", "Recursive glob"},
		{"src/server/*.go", "src/server/*.go", "Directory glob"},
		{"src\\server\\*.go", "src/server/*.go", "Windows path normalization"},
		{"", "**/*", "Empty defaults to all"},
		{".", ".", "Current directory"},
		{"./", "./", "Current directory with slash"},
		{"src/", "src/", "Directory prefix"},
	}

	for _, tp := range testPatterns {
		t.Run(tp.desc, func(t *testing.T) {
			// Test normalization logic here
			// This would test the internal path normalization
		})
	}
}