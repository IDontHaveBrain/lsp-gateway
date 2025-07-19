package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

// MockLSPServer represents a simple mock LSP server for testing
// Note: Now uses shell script approach instead of compilation
type MockLSPServer struct {
	// Reserved for future use if process-based mocking is needed
}

// LSPMessage represents an LSP protocol message
type LSPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// TestEndToEndIntegration tests the complete HTTP → Gateway → LSP Server → Response flow
func TestEndToEndIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	// Create in-memory mock LSP server
	mockServer, cleanup := createInMemoryMockLSPServer(t)
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

	// Create test configuration
	testConfig := &config.GatewayConfig{
		Port: port,
		Servers: []config.ServerConfig{
			{
				Name:      "mock-go-lsp",
				Languages: []string{"go"},
				Command:   mockServer,
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

	// Start HTTP server with custom mux
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

	// Test the complete end-to-end flow
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	t.Run("textDocument/definition", func(t *testing.T) {
		testTextDocumentDefinition(t, baseURL)
	})

	t.Run("textDocument/references", func(t *testing.T) {
		testTextDocumentReferences(t, baseURL)
	})

	t.Run("textDocument/documentSymbol", func(t *testing.T) {
		testTextDocumentSymbol(t, baseURL)
	})

	t.Run("workspace/symbol", func(t *testing.T) {
		testWorkspaceSymbol(t, baseURL)
	})

	t.Run("textDocument/hover", func(t *testing.T) {
		testTextDocumentHover(t, baseURL)
	})

	t.Run("error_handling", func(t *testing.T) {
		testErrorHandling(t, baseURL)
	})
}

// testTextDocumentDefinition tests the textDocument/definition request flow
func testTextDocumentDefinition(t *testing.T, baseURL string) {
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	response := makeJSONRPCRequest(t, baseURL, request)

	// Validate response structure
	if response.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSON-RPC 2.0, got: %s", response.JSONRPC)
	}

	// JSON unmarshaling can result in float64 for numbers
	expectedID := float64(1)
	if response.ID == nil {
		t.Error("Expected ID in response, got nil")
	} else if id, ok := response.ID.(float64); !ok || id != expectedID {
		t.Errorf("Expected ID %.0f, got: %v", expectedID, response.ID)
	}

	if response.Error != nil {
		t.Errorf("Unexpected error in response: %v", response.Error)
	}

	if response.Result == nil {
		t.Error("Expected result in response")
	}

	// Validate that the result contains expected mock data
	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got: %T", response.Result)
	}

	if resultMap["uri"] != "file:///test.go" {
		t.Errorf("Expected URI in result, got: %v", resultMap["uri"])
	}

	if resultMap["method"] != "textDocument/definition" {
		t.Errorf("Expected method in result, got: %v", resultMap["method"])
	}
}

// testTextDocumentReferences tests the textDocument/references request flow
func testTextDocumentReferences(t *testing.T, baseURL string) {
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      2,
		Method:  "textDocument/references",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}

	response := makeJSONRPCRequest(t, baseURL, request)

	// Validate response
	if response.Error != nil {
		t.Errorf("Unexpected error in response: %v", response.Error)
	}

	if response.Result == nil {
		t.Error("Expected result in response")
	}
}

// testTextDocumentSymbol tests the textDocument/documentSymbol request flow
func testTextDocumentSymbol(t *testing.T, baseURL string) {
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      3,
		Method:  "textDocument/documentSymbol",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
		},
	}

	response := makeJSONRPCRequest(t, baseURL, request)

	// Validate response
	if response.Error != nil {
		t.Errorf("Unexpected error in response: %v", response.Error)
	}

	if response.Result == nil {
		t.Error("Expected result in response")
	}
}

// testWorkspaceSymbol tests the workspace/symbol request flow
func testWorkspaceSymbol(t *testing.T, baseURL string) {
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      4,
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "test",
		},
	}

	response := makeJSONRPCRequest(t, baseURL, request)

	// Validate response
	if response.Error != nil {
		t.Errorf("Unexpected error in response: %v", response.Error)
	}

	if response.Result == nil {
		t.Error("Expected result in response")
	}
}

// testTextDocumentHover tests the textDocument/hover request flow
func testTextDocumentHover(t *testing.T, baseURL string) {
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      5,
		Method:  "textDocument/hover",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      7,
				"character": 12,
			},
		},
	}

	response := makeJSONRPCRequest(t, baseURL, request)

	// Validate response structure
	if response.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSON-RPC 2.0, got: %s", response.JSONRPC)
	}

	// JSON unmarshaling can result in float64 for numbers
	expectedID := float64(5)
	if response.ID == nil {
		t.Error("Expected ID in response, got nil")
	} else if id, ok := response.ID.(float64); !ok || id != expectedID {
		t.Errorf("Expected ID %.0f, got: %v", expectedID, response.ID)
	}

	if response.Error != nil {
		t.Errorf("Unexpected error in response: %v", response.Error)
	}

	if response.Result == nil {
		t.Error("Expected result in response")
	}

	// Validate that the result contains expected mock data for hover
	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got: %T", response.Result)
	}

	if resultMap["uri"] != "file:///test.go" {
		t.Errorf("Expected URI in result, got: %v", resultMap["uri"])
	}

	if resultMap["method"] != "textDocument/hover" {
		t.Errorf("Expected method in result, got: %v", resultMap["method"])
	}

	// Validate hover-specific fields
	if contents, ok := resultMap["contents"]; !ok {
		t.Error("Expected contents field in hover result")
	} else if contentsMap, ok := contents.(map[string]interface{}); !ok {
		t.Error("Expected contents to be an object")
	} else {
		if kind, ok := contentsMap["kind"]; !ok {
			t.Error("Expected kind field in hover contents")
		} else if kind != "markdown" {
			t.Errorf("Expected kind to be 'markdown', got: %v", kind)
		}

		if value, ok := contentsMap["value"]; !ok {
			t.Error("Expected value field in hover contents")
		} else if valueStr, ok := value.(string); !ok {
			t.Error("Expected value to be a string")
		} else if valueStr == "" {
			t.Error("Expected non-empty value in hover contents")
		}
	}
}

// testErrorHandling tests various error scenarios
func testErrorHandling(t *testing.T, baseURL string) {
	t.Run("unsupported_file_extension", func(t *testing.T) {
		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      5,
			Method:  "textDocument/definition",
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

	t.Run("hover_unsupported_file_extension", func(t *testing.T) {
		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      6,
			Method:  "textDocument/hover",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.unknown",
				},
				"position": map[string]interface{}{
					"line":      5,
					"character": 3,
				},
			},
		}

		response := makeJSONRPCRequest(t, baseURL, request)

		// Should return an error for unsupported extension
		if response.Error == nil {
			t.Error("Expected error for hover with unsupported file extension")
		}
	})

	t.Run("invalid_json_rpc", func(t *testing.T) {
		// Send invalid JSON-RPC request
		requestBody := `{"invalid": "request"}`

		resp, err := http.Post(baseURL+"/jsonrpc", "application/json", strings.NewReader(requestBody))
		if err != nil {
			t.Fatalf("Failed to make HTTP request: %v", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("cleanup error closing response body: %v", err)
			}
		}()

		var response JSONRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should return a JSON-RPC error
		if response.Error == nil {
			t.Error("Expected error for invalid JSON-RPC request")
		}
	})
}

// makeJSONRPCRequest makes a JSON-RPC request to the gateway and returns the response
func makeJSONRPCRequest(t *testing.T, baseURL string, request JSONRPCRequest) JSONRPCResponse {
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(baseURL+"/jsonrpc", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to make HTTP request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("cleanup error closing response body: %v", err)
		}
	}()

	var response JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return response
}

var (
	// Cache for compiled mock LSP server to avoid recompilation
	cachedMockServerPath string
	cachedMockServerOnce sync.Once
)

// createInMemoryMockLSPServer creates an efficient mock LSP server with compilation caching
// This eliminates the 2-5 second compilation overhead per test by using a cached binary
func createInMemoryMockLSPServer(t *testing.T) (string, func()) {
	// Use cached binary if available, otherwise compile once
	cachedMockServerOnce.Do(func() {
		cachedMockServerPath = compileMockLSPServerOnce(t)
	})

	// Return cached binary path with no-op cleanup (binary is reused across tests)
	cleanup := func() {
		// No cleanup needed - binary is cached for reuse
	}

	return cachedMockServerPath, cleanup
}

// compileMockLSPServerOnce compiles the mock LSP server once and caches it
func compileMockLSPServerOnce(t *testing.T) string {
	// Create temporary directory for cached binary
	tempDir, err := os.MkdirTemp("", "lsp-gateway-cached-mock-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create mock LSP server source code with fixed JSONRPCVersion
	mockServerSource := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const JSONRPCVersion = "2.0"

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
				JSONRPC: JSONRPCVersion,
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
		case "textDocument/definition":
			response = LSPMessage{
				JSONRPC: JSONRPCVersion,
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
		case "textDocument/hover":
			response = LSPMessage{
				JSONRPC: JSONRPCVersion,
				ID:      msg.ID,
				Result: map[string]interface{}{
					"uri": extractURI(msg.Params),
					"method": msg.Method,
					"contents": map[string]interface{}{
						"kind":  "markdown",
						"value": "func testFunction()\n\nA test function for hover demonstration",
					},
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 7, "character": 12},
						"end":   map[string]interface{}{"line": 7, "character": 25},
					},
				},
			}
		case "textDocument/references":
			response = LSPMessage{
				JSONRPC: JSONRPCVersion,
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
				JSONRPC: JSONRPCVersion,
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
				JSONRPC: JSONRPCVersion,
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
				JSONRPC: JSONRPCVersion,
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
`

	// Write mock server source to file
	sourceFile := filepath.Join(tempDir, "mock_lsp_server.go")
	if err := os.WriteFile(sourceFile, []byte(mockServerSource), 0644); err != nil {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil {
			t.Logf("cleanup error removing temp dir: %v", rmErr)
		}
		t.Fatalf("Failed to write mock server source: %v", err)
	}

	// Compile the mock server
	binaryPath := filepath.Join(tempDir, "mock_lsp_server")
	cmd := exec.Command("go", "build", "-o", binaryPath, sourceFile)
	if err := cmd.Run(); err != nil {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil {
			t.Logf("cleanup error removing temp dir: %v", rmErr)
		}
		t.Fatalf("Failed to compile mock LSP server: %v", err)
	}

	// Binary is cached and reused across all tests
	return binaryPath
}
