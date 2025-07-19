package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

// TestHoverIntegration tests hover functionality across all configured language servers
func TestHoverIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping hover integration test in short mode")
	}

	// Create mock LSP server binary with hover support
	mockServerPath, cleanup := createMockLSPServerWithHover(t)
	defer cleanup()

	// Find available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Logf("cleanup error closing listener: %v", err)
	}

	// Create test configuration with all supported languages
	testConfig := &config.GatewayConfig{
		Port: port,
		Servers: []config.ServerConfig{
			{
				Name:      "mock-go-lsp",
				Languages: []string{"go"},
				Command:   mockServerPath,
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "mock-python-lsp",
				Languages: []string{"python"},
				Command:   mockServerPath,
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "mock-typescript-lsp",
				Languages: []string{"typescript", "javascript"},
				Command:   mockServerPath,
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "mock-java-lsp",
				Languages: []string{"java"},
				Command:   mockServerPath,
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	// Validate test configuration
	if err := testConfig.Validate(); err != nil {
		t.Fatalf("Invalid test configuration: %v", err)
	}

	// Create and start gateway
	gw, err := NewGateway(testConfig)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}
	defer func() {
		if err := gw.Stop(); err != nil {
			t.Logf("Error stopping gateway: %v", err)
		}
	}()

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Logf("cleanup error shutting down server: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test hover functionality for each language
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	t.Run("textDocument/hover_go", func(t *testing.T) {
		testHoverForLanguage(t, baseURL, "file:///test.go", "Go")
	})

	t.Run("textDocument/hover_python", func(t *testing.T) {
		testHoverForLanguage(t, baseURL, "file:///test.py", "Python")
	})

	t.Run("textDocument/hover_typescript", func(t *testing.T) {
		testHoverForLanguage(t, baseURL, "file:///test.ts", "TypeScript")
	})

	t.Run("textDocument/hover_javascript", func(t *testing.T) {
		testHoverForLanguage(t, baseURL, "file:///test.js", "JavaScript")
	})

	t.Run("textDocument/hover_java", func(t *testing.T) {
		testHoverForLanguage(t, baseURL, "file:///Test.java", "Java")
	})

	t.Run("hover_error_handling", func(t *testing.T) {
		testHoverErrorHandling(t, baseURL)
	})
}

// testHoverForLanguage tests hover functionality for a specific language
func testHoverForLanguage(t *testing.T, baseURL, fileURI, expectedLanguage string) {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "textDocument/hover",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	response := makeJSONRPCRequest(t, baseURL, request)

	// Validate response structure
	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC 2.0, got: %s", response.JSONRPC)
	}

	if response.ID == nil {
		t.Error("Expected ID in response, got nil")
	}

	if response.Error != nil {
		t.Errorf("Unexpected error in response: %v", response.Error)
	}

	if response.Result == nil {
		t.Error("Expected result in response")
	}

	// Validate hover response structure
	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got: %T", response.Result)
	}

	// Check that hover contains expected mock data
	if resultMap["uri"] != fileURI {
		t.Errorf("Expected URI %s in result, got: %v", fileURI, resultMap["uri"])
	}

	if resultMap["method"] != "textDocument/hover" {
		t.Errorf("Expected method textDocument/hover in result, got: %v", resultMap["method"])
	}

	if resultMap["language"] != expectedLanguage {
		t.Errorf("Expected language %s in result, got: %v", expectedLanguage, resultMap["language"])
	}

	// Validate hover content structure
	contents, ok := resultMap["contents"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected contents to be a map, got: %T", resultMap["contents"])
		return
	}

	if contents["kind"] != "markdown" {
		t.Errorf("Expected markdown content kind, got: %v", contents["kind"])
	}

	if contents["value"] == nil {
		t.Error("Expected hover content value")
	}

	// Validate range is present
	if resultMap["range"] == nil {
		t.Error("Expected range in hover result")
	}
}

// testHoverErrorHandling tests error scenarios for hover requests
func testHoverErrorHandling(t *testing.T, baseURL string) {
	t.Run("unsupported_file_extension", func(t *testing.T) {
		request := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      5,
			Method:  "textDocument/hover",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.unknown",
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
		}

		response := makeJSONRPCRequest(t, baseURL, request)

		// Should return an error for unsupported extension
		if response.Error == nil {
			t.Error("Expected error for unsupported file extension")
		}
	})

	t.Run("missing_position", func(t *testing.T) {
		request := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      6,
			Method:  "textDocument/hover",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				// Missing position parameter
			},
		}

		response := makeJSONRPCRequest(t, baseURL, request)

		// Should handle gracefully - our mock server should return hover info
		// even without position (some LSP servers allow this)
		if response.Error != nil {
			t.Logf("LSP server returned error for missing position (this is acceptable): %v", response.Error)
		}
	})

	t.Run("invalid_uri", func(t *testing.T) {
		request := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      7,
			Method:  "textDocument/hover",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "invalid://uri",
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
		}

		response := makeJSONRPCRequest(t, baseURL, request)

		// Should return an error for invalid URI scheme
		if response.Error == nil {
			t.Error("Expected error for invalid URI scheme")
		}
	})
}

// createMockLSPServerWithHover creates a mock LSP server that supports hover
func createMockLSPServerWithHover(t *testing.T) (string, func()) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lsp-gateway-hover-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create mock LSP server source code with hover support
	mockServerSource := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LSPMessage struct {
	JSONRPC string      ` + "`json:\"jsonrpc\"`" + `
	ID      interface{} ` + "`json:\"id,omitempty\"`" + `
	Method  string      ` + "`json:\"method,omitempty\"`" + `
	Params  interface{} ` + "`json:\"params,omitempty\"`" + `
	Result  interface{} ` + "`json:\"result,omitempty\"`" + `
	Error   interface{} ` + "`json:\"error,omitempty\"`" + `
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for {
		// Read Content-Length header
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				continue
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break // End of headers
			}

			if strings.HasPrefix(line, "Content-Length: ") {
				lengthStr := strings.TrimPrefix(line, "Content-Length: ")
				if n, err := strconv.Atoi(lengthStr); err == nil {
					contentLength = n
				}
			}
		}

		if contentLength <= 0 {
			continue
		}

		// Read message body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			continue
		}

		// Parse message
		var msg LSPMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		// Handle different LSP methods
		var response LSPMessage
		switch msg.Method {
		case "initialize":
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: map[string]interface{}{
					"capabilities": map[string]interface{}{
						"definitionProvider": true,
						"referencesProvider": true,
						"documentSymbolProvider": true,
						"workspaceSymbolProvider": true,
						"hoverProvider": true,
					},
				},
			}
		case "initialized":
			// No response needed for initialized notification
			continue
		case "textDocument/hover":
			uri := extractURI(msg.Params)
			language := getLanguageFromURI(uri)
			
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: map[string]interface{}{
					"uri": uri,
					"method": msg.Method,
					"language": language,
					"contents": map[string]interface{}{
						"kind": "markdown",
						"value": fmt.Sprintf("**%s Symbol Information**\n\nThis is hover information for a %s symbol at the requested position.\n\n` + "```" + `%s\nfunction exampleFunction() {\n    // Mock hover content\n    return \"Hello from %s LSP\";\n}\n` + "```" + `", language, language, strings.ToLower(language), language),
					},
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 10, "character": 5},
						"end":   map[string]interface{}{"line": 10, "character": 15},
					},
				},
			}
		case "textDocument/definition":
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: map[string]interface{}{
					"uri": extractURI(msg.Params),
					"method": msg.Method,
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 5, "character": 0},
						"end":   map[string]interface{}{"line": 5, "character": 10},
					},
				},
			}
		case "textDocument/references":
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: []map[string]interface{}{
					{
						"uri": extractURI(msg.Params),
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 5, "character": 0},
							"end":   map[string]interface{}{"line": 5, "character": 10},
						},
					},
				},
			}
		case "textDocument/documentSymbol":
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: []map[string]interface{}{
					{
						"name": "MockSymbol",
						"kind": 12, // Function
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 0, "character": 0},
							"end":   map[string]interface{}{"line": 10, "character": 0},
						},
						"selectionRange": map[string]interface{}{
							"start": map[string]interface{}{"line": 0, "character": 0},
							"end":   map[string]interface{}{"line": 0, "character": 10},
						},
					},
				},
			}
		case "workspace/symbol":
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: []map[string]interface{}{
					{
						"name": "MockWorkspaceSymbol",
						"kind": 12, // Function
						"location": map[string]interface{}{
							"uri": "file:///mock.go",
							"range": map[string]interface{}{
								"start": map[string]interface{}{"line": 0, "character": 0},
								"end":   map[string]interface{}{"line": 0, "character": 10},
							},
						},
					},
				},
			}
		default:
			// Return error for unsupported methods
			response = LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			}
		}

		// Send response
		responseData, _ := json.Marshal(response)
		responseContent := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(responseData), responseData)
		writer.WriteString(responseContent)
		writer.Flush()
	}
}

func extractURI(params interface{}) string {
	if params == nil {
		return ""
	}
	
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return ""
	}
	
	if textDoc, exists := paramsMap["textDocument"]; exists {
		if textDocMap, ok := textDoc.(map[string]interface{}); ok {
			if uri, exists := textDocMap["uri"]; exists {
				if uriStr, ok := uri.(string); ok {
					return uriStr
				}
			}
		}
	}
	
	return ""
}

func getLanguageFromURI(uri string) string {
	if uri == "" {
		return "Unknown"
	}
	
	ext := strings.ToLower(filepath.Ext(uri))
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".ts":
		return "TypeScript"
	case ".js":
		return "JavaScript"
	case ".java":
		return "Java"
	default:
		return "Unknown"
	}
}
`

	// Write mock server source to file
	sourceFile := filepath.Join(tempDir, "mock_lsp_server_hover.go")
	if err := os.WriteFile(sourceFile, []byte(mockServerSource), 0644); err != nil {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil {
			t.Logf("cleanup error removing temp dir: %v", rmErr)
		}
		t.Fatalf("Failed to write mock server source: %v", err)
	}

	// Compile the mock server
	binaryPath := filepath.Join(tempDir, "mock_lsp_server_hover")
	cmd := exec.Command("go", "build", "-o", binaryPath, sourceFile)
	if err := cmd.Run(); err != nil {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil {
			t.Logf("cleanup error removing temp dir: %v", rmErr)
		}
		t.Fatalf("Failed to compile mock LSP server: %v", err)
	}

	// Return binary path and cleanup function
	cleanup := func() {
		if err := os.RemoveAll(tempDir); err != nil {
			// Note: In cleanup functions, we typically log but don't fail the test
			t.Logf("cleanup error removing temp dir: %v", err)
		}
	}

	return binaryPath, cleanup
}
